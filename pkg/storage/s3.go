/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package storage

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/smithy-go/logging"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io"
	"os"
	"runtime/trace"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	S3Storage          = config.S3Storage
	s3ReadLimitEnvKey  = "STORAGE_S3_READ_LIMIT"
	s3WriteLimitEnvKey = "STORAGE_S3_WRITE_LIMIT"
)

type s3Storage struct {
	sid       string
	s3Client  *s3.Client
	cfg       *config.S3Config
	readRate  *utils.ParallelLimiter
	writeRate *utils.ParallelLimiter
	logger    *zap.SugaredLogger
}

var _ Storage = &s3Storage{}

func (s *s3Storage) ID() string {
	return s.sid
}

func (s *s3Storage) initBucket(ctx context.Context) error {
	_, err := s.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.cfg.BucketName)})
	if err == nil {
		s.logger.Warnw("head bucket got error, try create one", "bucket", s.cfg.BucketName, "err", err)
		return nil
	}

	_, err = s.s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket:                    aws.String(s.cfg.BucketName),
		CreateBucketConfiguration: &s3types.CreateBucketConfiguration{LocationConstraint: s3types.BucketLocationConstraint(s.cfg.Region)},
	})
	if err != nil {
		s.logger.Errorw("create bucket error", "bucket", s.cfg.BucketName, "err", err)
		return fmt.Errorf("create bucket %s error %s", s.cfg.BucketName, err)
	}
	return nil
}

func (s *s3Storage) Get(ctx context.Context, key, idx int64) (io.ReadCloser, error) {
	defer trace.StartRegion(ctx, "storage.s3.Get").End()
	if err := s.readRate.Acquire(ctx); err != nil {
		return nil, err
	}
	defer s.readRate.Release()

	output, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.BucketName),
		Key:    aws.String(s3ObjectName(key, idx)),
	})
	if err != nil {
		s.logger.Errorw("get s3 object error", "object", s3ObjectName(key, idx), "err", err)
		return nil, err
	}

	return output.Body, nil
}

func (s *s3Storage) Put(ctx context.Context, key, idx int64, dataReader io.Reader) error {
	defer trace.StartRegion(ctx, "storage.s3.Put").End()
	if err := s.writeRate.Acquire(ctx); err != nil {
		return err
	}
	defer s.writeRate.Release()

	_, err := s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.cfg.BucketName),
		Key:    aws.String(s3ObjectName(key, idx)),
		Body:   newS3SeekerWrapper(dataReader),
	})
	if err != nil {
		s.logger.Errorw("put object to s3 error", "object", s3ObjectName(key, idx), "err", err)
		return err
	}

	return nil
}

func (s *s3Storage) Delete(ctx context.Context, key int64) error {
	defer trace.StartRegion(ctx, "storage.s3.Delete").End()

	listOutput, err := s.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.cfg.BucketName),
		Prefix: aws.String(s3ObjectPrefix(key)),
	})
	if err != nil {
		s.logger.Errorw("list need s3 objects need to delete error", "prefix", s3ObjectPrefix(key), "err", err)
		return err
	}

	needDel := s3types.Delete{Objects: []s3types.ObjectIdentifier{}}
	for _, need := range listOutput.Contents {
		needDel.Objects = append(needDel.Objects, s3types.ObjectIdentifier{Key: need.Key})
	}

	_, err = s.s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(s.cfg.BucketName),
		Delete: &needDel,
	})
	if err != nil {
		s.logger.Errorw("delete s3 objects error", "prefix", s3ObjectPrefix(key), "err", err)
		return err
	}

	return nil
}

func (s *s3Storage) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	defer trace.StartRegion(ctx, "storage.s3.Head").End()

	output, err := s.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.BucketName),
		Key:    aws.String(s3ObjectName(key, idx)),
	})
	if err != nil {
		s.logger.Errorw("get s3 object attr error", "object", s3ObjectName(key, idx), "err", err)
		return Info{}, err
	}

	return Info{Key: s3ObjectName(key, idx), Size: output.ContentLength}, nil
}

func newS3Storage(storageID string, cfg *config.S3Config) (Storage, error) {
	log := logger.NewLogger("s3")

	if cfg.Region == "" {
		return nil, fmt.Errorf("region is emtry")
	}
	if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("access_key_id or secret_access_key is emtry")
	}
	if cfg.BucketName == "" {
		return nil, fmt.Errorf("bucket_name is emtry")
	}

	awsConfig, err := awscfg.LoadDefaultConfig(
		context.TODO(),
		awscfg.WithRegion(cfg.Region),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		awscfg.WithDefaultsMode(aws.DefaultsModeStandard),
		awscfg.WithLogger(s3LoggerWrapper{SugaredLogger: log}),
		awscfg.WithClientLogMode(aws.LogRetries|aws.LogRequest),
	)
	if err != nil {
		return nil, err
	}
	s := &s3Storage{
		sid:       storageID,
		s3Client:  s3.NewFromConfig(awsConfig, s3CustomConfig(cfg)),
		cfg:       cfg,
		readRate:  utils.NewParallelLimiter(str2Int(os.Getenv(s3ReadLimitEnvKey), 20)),
		writeRate: utils.NewParallelLimiter(str2Int(os.Getenv(s3WriteLimitEnvKey), 10)),
		logger:    log,
	}
	return s, s.initBucket(context.TODO())
}

type s3LoggerWrapper struct {
	*zap.SugaredLogger
}

func (log s3LoggerWrapper) Logf(classification logging.Classification, format string, v ...interface{}) {
	if classification == logging.Warn {
		log.Warnf(format, v...)
		return
	}
	log.Debugf(format, v...)
}

func s3ObjectName(key, idx int64) string {
	return fmt.Sprintf("s3/chunks/%d/%s/%d_%d", key/10, reverseString(strconv.FormatInt(key, 10)), key, idx)
}

func s3ObjectPrefix(key int64) string {
	return fmt.Sprintf("s3/chunks/%d/%s/%d_", key/10, reverseString(strconv.FormatInt(key, 10)), key)
}

func s3CustomConfig(cfg *config.S3Config) func(opt *s3.Options) {
	return func(opt *s3.Options) {
		opt.RetryMode = aws.RetryModeAdaptive
		opt.RetryMaxAttempts = 50
		opt.UsePathStyle = cfg.UsePathStyle
	}
}

// newS3SeekerWrapper Wrap the Reader as a ReadSeeker by using a memory copy.
// FIXME: There are performance issues here that need to be addressed.
// We need to pay attention to further developments https://github.com/aws/aws-sdk-go-v2/issues/2038
func newS3SeekerWrapper(r io.Reader) io.ReadSeeker {
	data, _ := io.ReadAll(r)
	rs := bytes.NewReader(data)
	return rs
}
