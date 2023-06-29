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
	"crypto/tls"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/studio-b12/gowebdav"
	"go.uber.org/zap"
	"io"
	"net/http"
	"runtime/trace"
	"sync"
	"time"
)

const (
	WebdavStorage = config.WebdavStorage
)

type webdavStorage struct {
	sid       string
	basePath  string
	cli       *gowebdav.Client
	readRate  *utils.ParallelLimiter
	writeRate *utils.ParallelLimiter
	dirLock   sync.Mutex
	logger    *zap.SugaredLogger
}

var _ Storage = &webdavStorage{}

func (w *webdavStorage) ID() string {
	return w.sid
}

func (w *webdavStorage) Get(ctx context.Context, key, idx int64) (io.ReadCloser, error) {
	defer trace.StartRegion(ctx, "storage.webdav.Get").End()
	if err := w.readRate.Acquire(ctx); err != nil {
		return nil, err
	}
	defer w.readRate.Release()
	fileReader, err := w.cli.ReadStream(webdavObjectPath(key, idx))
	if err != nil {
		w.logger.Errorw("get file from server failed", "path", webdavObjectPath(key, idx), "err", err)
		return nil, err
	}
	return fileReader, nil
}

func (w *webdavStorage) Put(ctx context.Context, key, idx int64, dataReader io.Reader) error {
	defer trace.StartRegion(ctx, "storage.webdav.Put").End()
	if err := w.writeRate.Acquire(ctx); err != nil {
		return err
	}
	defer w.writeRate.Release()
	err := func() error {
		// concurrent creation will result in a 403 error.
		w.dirLock.Lock()
		defer w.dirLock.Unlock()

		var innerErr error
		for i := 0; i < 5; i++ {
			innerErr = w.cli.MkdirAll(webdavObjectDir(key), 0655)
			if innerErr == nil {
				return nil
			}
			time.Sleep(time.Millisecond * 10)
		}
		return innerErr
	}()
	if err != nil {
		w.logger.Errorw("put file to server failed: mkdir error", "path", webdavObjectDir(key), "err", err)
		return err
	}

	err = w.cli.WriteStream(webdavObjectPath(key, idx), dataReader, 0655)
	if err != nil {
		w.logger.Errorw("put file to server failed", "path", webdavObjectPath(key, idx), "err", err)
		return err
	}
	return nil
}

func (w *webdavStorage) Delete(ctx context.Context, key int64) error {
	defer trace.StartRegion(ctx, "storage.webdav.Delete").End()
	w.dirLock.Lock()
	defer w.dirLock.Unlock()
	err := w.cli.RemoveAll(webdavObjectDir(key))
	if err != nil {
		w.logger.Errorw("delete dir failed", "path", webdavObjectDir(key), "err", err)
		return err
	}
	return nil
}

func (w *webdavStorage) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	defer trace.StartRegion(ctx, "storage.webdav.Head").End()
	info, err := w.cli.Stat(webdavObjectPath(key, idx))
	if err != nil {
		w.logger.Errorw("stat file from server failed", "path", webdavObjectPath(key, idx), "err", err)
		return Info{}, err
	}
	return Info{
		Key:  info.Name(),
		Size: info.Size(),
	}, nil
}

func newWebdavStorage(storageID string, cfg *config.WebdavStorageConfig) (Storage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("webdav is nil")
	}
	if storageID == "" {
		return nil, fmt.Errorf("storage id is empty")
	}

	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("webdav config server_url is empty")
	}
	if cfg.Username == "" {
		return nil, fmt.Errorf("webdav config user is empty")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("webdav config password is empty")
	}

	t := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   60 * time.Second,
		ExpectContinueTimeout: 10 * time.Second,
	}
	if cfg.Insecure {
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	cli := gowebdav.NewClient(cfg.ServerURL, cfg.Username, cfg.Password)
	cli.SetTransport(t)

	s := &webdavStorage{
		sid:       storageID,
		cli:       cli,
		readRate:  utils.NewParallelLimiter(10),
		writeRate: utils.NewParallelLimiter(5),
		logger:    logger.NewLogger("webdav"),
	}
	return s, nil
}

func webdavObjectPath(key, idx int64) string {
	return fmt.Sprintf("/webdav/chunks/%d/%d/%d_%d", key/100, key, key, idx)
}

func webdavObjectDir(key int64) string {
	return fmt.Sprintf("/webdav/chunks/%d/%d", key/100, key)
}
