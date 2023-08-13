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
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io"
	"os"
)

const (
	UnofficialAliyunDriverStorage = config.UnofficialAliyunDriverStorage
	aliyunDriverReadLimitEnvKey   = "STORAGE_ALIYUN_DRIVER_READ_LIMIT"
	aliyunDriverWriteLimitEnvKey  = "STORAGE_ALIYUN_DRIVER_WRITE_LIMIT"
)

// unofficialAliyunDriverStorage
// This storage uses a third-party SDK to operate resources on Aliyun Driver,
// which may pose availability risks. It can be used in backup scenarios.
type unofficialAliyunDriverStorage struct {
	sid       string
	cfg       *config.AliyunDriverConfig
	readRate  *utils.ParallelLimiter
	writeRate *utils.ParallelLimiter
	logger    *zap.SugaredLogger
}

func (u *unofficialAliyunDriverStorage) ID() string {
	return u.sid
}

func (u *unofficialAliyunDriverStorage) Get(ctx context.Context, key, idx int64) (io.ReadCloser, error) {
	//TODO implement me
	panic("implement me")
}

func (u *unofficialAliyunDriverStorage) Put(ctx context.Context, key, idx int64, dataReader io.Reader) error {
	//TODO implement me
	panic("implement me")
}

func (u *unofficialAliyunDriverStorage) Delete(ctx context.Context, key int64) error {
	//TODO implement me
	panic("implement me")
}

func (u *unofficialAliyunDriverStorage) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	//TODO implement me
	panic("implement me")
}

func newUnofficialAliyunDriverStorage(storageID string, cfg *config.AliyunDriverConfig) (Storage, error) {
	s := &unofficialAliyunDriverStorage{
		sid:       storageID,
		cfg:       cfg,
		readRate:  utils.NewParallelLimiter(str2Int(os.Getenv(aliyunDriverReadLimitEnvKey), 10)),
		writeRate: utils.NewParallelLimiter(str2Int(os.Getenv(aliyunDriverWriteLimitEnvKey), 5)),
		logger:    logger.NewLogger("UnofficialAliyunDriver"),
	}
	return s, nil
}

func aliyunDriverObjectPath(key, idx int64) string {
	return fmt.Sprintf("/aliyundriver/chunks/%d/%d/%d_%d", key/100, key, key, idx)
}

func aliyunDriverObjectDir(key int64) string {
	return fmt.Sprintf("/aliyundriver/chunks/%d/%d", key/100, key)
}
