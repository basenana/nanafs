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
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io"
	"os"
)

const (
	S3Storage          = config.S3Storage
	s3ReadLimitEnvKey  = "STORAGE_S3_READ_LIMIT"
	s3WriteLimitEnvKey = "STORAGE_S3_WRITE_LIMIT"
)

type s3Storage struct {
	sid       string
	cfg       *config.S3Config
	readRate  *utils.ParallelLimiter
	writeRate *utils.ParallelLimiter
	logger    *zap.SugaredLogger
}

func (s *s3Storage) ID() string {
	//TODO implement me
	panic("implement me")
}

func (s *s3Storage) initBucket(ctx context.Context) error {
	return nil
}

func (s *s3Storage) Get(ctx context.Context, key, idx int64) (io.ReadCloser, error) {
	//TODO implement me
	panic("implement me")
}

func (s *s3Storage) Put(ctx context.Context, key, idx int64, dataReader io.Reader) error {
	//TODO implement me
	panic("implement me")
}

func (s *s3Storage) Delete(ctx context.Context, key int64) error {
	//TODO implement me
	panic("implement me")
}

func (s *s3Storage) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	//TODO implement me
	panic("implement me")
}

func newS3Storage(storageID string, cfg *config.S3Config) (Storage, error) {
	s := &s3Storage{
		sid:       storageID,
		cfg:       cfg,
		readRate:  utils.NewParallelLimiter(str2Int(os.Getenv(s3ReadLimitEnvKey), 20)),
		writeRate: utils.NewParallelLimiter(str2Int(os.Getenv(s3WriteLimitEnvKey), 10)),
		logger:    logger.NewLogger("minio"),
	}
	return s, s.initBucket(context.TODO())
}
