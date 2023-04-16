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
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io"
	"runtime/trace"
	"strconv"
)

const (
	OSSStorage = "oss"
)

type aliyunOSSStorage struct {
	sid        string
	cli        *oss.Client
	bucket     *oss.Bucket
	cfg        *config.OSSConfig
	readLimit  chan struct{}
	writeLimit chan struct{}
	logger     *zap.SugaredLogger
}

var _ Storage = &aliyunOSSStorage{}

func (a *aliyunOSSStorage) ID() string {
	return a.sid
}

func (a *aliyunOSSStorage) Get(ctx context.Context, key, idx int64) (io.ReadCloser, error) {
	defer trace.StartRegion(ctx, "storage.oss.Get").End()
	a.readLimit <- struct{}{}
	defer func() {
		<-a.readLimit
	}()
	r, err := a.bucket.GetObject(ossObjectName(key, idx))
	if err != nil {
		a.logger.Errorw("get oss object error", "object", ossObjectName(key, idx), "err", err)
		return nil, err
	}
	return r, nil
}

func (a *aliyunOSSStorage) Put(ctx context.Context, key, idx int64, dataReader io.Reader) error {
	defer trace.StartRegion(ctx, "storage.oss.Put").End()
	a.writeLimit <- struct{}{}
	defer func() {
		<-a.writeLimit
	}()
	err := a.bucket.PutObject(ossObjectName(key, idx), dataReader)
	if err != nil {
		a.logger.Errorw("put object to oss error", "object", ossObjectName(key, idx), "err", err)
		return err
	}
	return nil
}

func (a *aliyunOSSStorage) Delete(ctx context.Context, key int64) error {
	defer trace.StartRegion(ctx, "storage.oss.Delete").End()
	marker := oss.Marker("")
	prefix := oss.Prefix(ossObjectPrefix(key))
	count := 0
	for {
		lor, err := a.bucket.ListObjects(marker, prefix)
		if err != nil {
			a.logger.Errorw("list objects to delete error", "prefix", ossObjectPrefix(key), "err", err)
			return err
		}

		var objects []string
		for _, object := range lor.Objects {
			objects = append(objects, object.Key)
		}
		delRes, err := a.bucket.DeleteObjects(objects, oss.DeleteObjectsQuiet(true))
		if err != nil {
			a.logger.Errorw("delete objects error", "prefix", ossObjectPrefix(key), "err", err)
			return err
		}

		if len(delRes.DeletedObjects) > 0 {
			for _, obj := range delRes.DeletedObjects {
				a.logger.Warnw("object deleted failure", "object", obj)
			}
		}

		count += len(objects) - len(delRes.DeletedObjects)

		prefix = oss.Prefix(lor.Prefix)
		marker = oss.Marker(lor.NextMarker)
		if !lor.IsTruncated {
			break
		}
	}
	a.logger.Infof("delete objects(key=%d) finish, total delete object count: %d", key, count)
	return nil
}

func (a *aliyunOSSStorage) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	defer trace.StartRegion(ctx, "storage.oss.Head").End()
	header, err := a.bucket.GetObjectMeta(ossObjectName(key, idx))
	if err != nil {
		a.logger.Errorw("head oss object error", "object", ossObjectName(key, idx), "err", err)
		return Info{}, err
	}
	info := Info{
		Key: ossObjectName(key, idx),
	}
	info.Size, _ = strconv.ParseInt(header.Get("Content-Length"), 10, 64)
	return info, nil
}

func (a *aliyunOSSStorage) initOSSBucket(ctx context.Context) error {
	defer trace.StartRegion(ctx, "storage.oss.initOSSBucket").End()
	a.logger.Infof("OSS SDK Version: %s", oss.Version)

	isExist, err := a.cli.IsBucketExist(a.cfg.BucketName)
	if err != nil {
		a.logger.Errorw("check bucket error", "bucket", a.cfg.BucketName, "err", err)
		return err
	}

	if !isExist {
		err = a.cli.CreateBucket(a.cfg.BucketName)
		if err != nil {
			a.logger.Errorw("create bucket error", "bucket", a.cfg.BucketName, "err", err)
			return err
		}
	}

	a.bucket, err = a.cli.Bucket(a.cfg.BucketName)
	if err != nil {
		a.logger.Errorw("build bucket error", "bucket", a.cfg.BucketName, "err", err)
		return err
	}
	return err
}

func newOSSStorage(storageID string, cfg *config.OSSConfig) (Storage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("OSS config is nil")
	}
	if storageID == "" {
		return nil, fmt.Errorf("storage id is empty")
	}

	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("OSS endpoint is empty")
	}
	if cfg.AccessKeyID == "" {
		return nil, fmt.Errorf("OSS access_key_id is empty")
	}
	if cfg.AccessKeySecret == "" {
		return nil, fmt.Errorf("OSS access_key_secret is empty")
	}
	if cfg.BucketName == "" {
		cfg.BucketName = fmt.Sprintf("nanafs-%s", storageID)
	}
	cli, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return nil, err
	}

	cli.Config.RetryTimes = 20
	cli.Config.Timeout = 60 * 10
	cli.Config.HTTPTimeout = oss.HTTPTimeout{
		ConnectTimeout:   60,
		ReadWriteTimeout: 60 * 10,
		HeaderTimeout:    60,
		LongTimeout:      60 * 10,
		IdleConnTimeout:  60,
	}
	cli.Config.HTTPMaxConns = oss.HTTPMaxConns{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 64,
		MaxConnsPerHost:     64,
	}

	s := &aliyunOSSStorage{
		sid:        storageID,
		cli:        cli,
		cfg:        cfg,
		readLimit:  make(chan struct{}, 50),
		writeLimit: make(chan struct{}, 20),
		logger:     logger.NewLogger("OSS"),
	}
	return s, s.initOSSBucket(context.Background())
}

func ossObjectName(key, idx int64) string {
	return fmt.Sprintf("oss/chunks/%d/%d/%d_%d", key/1000, key/10, key, idx)
}

func ossObjectPrefix(key int64) string {
	return fmt.Sprintf("oss/chunks/%d/%d/%d_", key/1000, key/10, key)
}
