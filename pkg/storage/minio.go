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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"net/http"
	"runtime/trace"
	"time"
)

const (
	MinioStorage = "minio"
)

type minioStorage struct {
	sid    string
	bucket string
	cli    *minio.Client
	cfg    *config.MinIOConfig
	logger *zap.SugaredLogger
}

var _ Storage = &minioStorage{}

func (m *minioStorage) ID() string {
	return m.sid
}

func (m *minioStorage) Get(ctx context.Context, key, idx int64) (io.ReadCloser, error) {
	defer trace.StartRegion(ctx, "storage.minio.Get").End()
	obj, err := m.cli.GetObject(ctx, m.bucket, minioObjectName(key, idx), minio.GetObjectOptions{})
	if err != nil {
		m.logger.Errorw("get object failed", "object", minioObjectName(key, idx), "err", err)
		return nil, err
	}
	return obj, nil
}

func (m *minioStorage) Put(ctx context.Context, key, idx int64, dataReader io.Reader) error {
	defer trace.StartRegion(ctx, "storage.minio.Put").End()
	maxConcurrentUploads <- struct{}{}
	defer func() {
		<-maxConcurrentUploads
	}()
	data, err := ioutil.ReadAll(dataReader)
	if err != nil {
		return err
	}

	_, err = m.cli.PutObject(ctx, m.bucket, minioObjectName(key, idx), bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType:      "application/octet-stream",
		DisableMultipart: true,
	})
	if err != nil {
		m.logger.Errorw("put object failed", "object", minioObjectName(key, idx), "err", err)
		return err
	}
	return nil
}

func (m *minioStorage) Delete(ctx context.Context, key int64) error {
	defer trace.StartRegion(ctx, "storage.minio.Delete").End()
	needDeleteCh := make(chan minio.ObjectInfo, 10)
	errCh := m.cli.RemoveObjects(ctx, m.bucket, needDeleteCh, minio.RemoveObjectsOptions{})

	objectCh := m.cli.ListObjects(context.Background(), m.bucket, minio.ListObjectsOptions{Prefix: minioObjectPrefix(key), Recursive: true})
	var listErr error
	for object := range objectCh {
		if object.Err != nil {
			m.logger.Errorw("list object to delete failed", "object", object.Key, "err", object.Err)
			listErr = object.Err
			continue
		}
		needDeleteCh <- object
	}
	close(needDeleteCh)
	if listErr != nil {
		return listErr
	}

	var err error
	for gotErr := range errCh {
		if gotErr.Err != nil {
			m.logger.Errorw("delete object failed", "object", gotErr.ObjectName, "err", gotErr.Err)
			err = gotErr.Err
		}
	}
	return err
}

func (m *minioStorage) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	defer trace.StartRegion(ctx, "storage.minio.Head").End()
	info, err := m.cli.StatObject(ctx, m.bucket, minioObjectName(key, idx), minio.StatObjectOptions{})
	if err != nil {
		m.logger.Errorw("head object failed", "object", minioObjectName(key, idx), "err", err)
		return Info{}, err
	}
	return Info{Key: info.Key, Size: info.Size}, nil
}

func (m *minioStorage) initBucket(ctx context.Context) error {
	defer trace.StartRegion(ctx, "storage.minio.initBucket").End()
	ctx, canF := context.WithTimeout(ctx, time.Minute)
	defer canF()

	exists, errBucketExists := m.cli.BucketExists(ctx, m.bucket)
	if errBucketExists == nil && exists {
		return nil
	}

	m.logger.Infof("init bucket: %s", m.bucket)
	err := m.cli.MakeBucket(ctx, m.bucket, minio.MakeBucketOptions{Region: m.cfg.Location})
	if err != nil {
		return err
	}
	return nil
}

func newMinioStorage(storageID string, cfg *config.MinIOConfig) (Storage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("minio is nil")
	}

	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("minio config endpoint is empty")
	}
	if cfg.AccessKeyID == "" {
		return nil, fmt.Errorf("minio config access_key_id is empty")
	}
	if cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("minio config secret_access_key is empty")
	}

	if cfg.BucketName == "" {
		cfg.BucketName = fmt.Sprintf("nanafs-%s", storageID)
	}

	minioClient, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.Token),
		Secure:    cfg.UseSSL,
		Transport: http.DefaultTransport,
	})
	if err != nil {
		return nil, err
	}
	s := &minioStorage{
		sid:    storageID,
		bucket: cfg.BucketName,
		cli:    minioClient,
		cfg:    cfg,
		logger: logger.NewLogger("minio"),
	}
	return s, s.initBucket(context.TODO())
}

func minioObjectName(key, idx int64) string {
	return fmt.Sprintf("minio/chunks/%d/%d/%d_%d", key/10000, key/100, key, idx)
}

func minioObjectPrefix(key int64) string {
	return fmt.Sprintf("/minio/chunks/%d/%d/%d_", key/10000, key/100, key)
}
