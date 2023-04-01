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
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io"
	"os"
	"path"
)

const (
	LocalStorage         = "local"
	defaultLocalDirMode  = 0755
	defaultLocalFileMode = 0644

	cacheNodeSize = 1 << 21 // 2M
)

type local struct {
	sid    string
	dir    string
	logger *zap.SugaredLogger
}

var _ Storage = &local{}

func (l *local) ID() string {
	return l.sid
}

func (l *local) Get(ctx context.Context, key int64, idx, offset int64, dest []byte) (int64, error) {
	defer utils.TraceRegion(ctx, "local.get")()
	file, err := l.openLocalFile(l.key2LocalPath(key, idx), os.O_RDWR)
	if err != nil {
		return 0, err
	}
	count, err := file.ReadAt(dest, offset)
	return int64(count), err
}

func (l *local) Put(ctx context.Context, key int64, idx, offset int64, data []byte) error {
	defer utils.TraceRegion(ctx, "local.put")()
	file, err := l.openLocalFile(l.key2LocalPath(key, idx), os.O_CREATE|os.O_RDWR)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		l.logger.Errorw("seek file failed", "key", key, "offset", offset, "err", err.Error())
		_ = file.Close()
		return err
	}
	_, err = io.Copy(file, bytes.NewBuffer(data))
	return err
}

func (l *local) Delete(ctx context.Context, key int64) error {
	defer utils.TraceRegion(ctx, "local.delete")()
	p := path.Join(l.dir, fmt.Sprintf("%d", key))
	_, err := os.Stat(p)
	if err != nil && !os.IsNotExist(err) {
		l.logger.Errorw("delete file failed", "key", key, "err", err.Error())
		return err
	}

	return os.RemoveAll(p)
}

func (l *local) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	defer utils.TraceRegion(ctx, "local.head")()
	info, err := os.Stat(l.key2LocalPath(key, idx))
	if err != nil && !os.IsNotExist(err) {
		return Info{}, err
	}
	if os.IsNotExist(err) {
		return Info{}, types.ErrNotFound
	}
	return Info{
		Key:  info.Name(),
		Size: info.Size(),
	}, nil
}

func (l *local) openLocalFile(path string, flag int) (*os.File, error) {
	info, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if os.IsNotExist(err) && flag&os.O_CREATE == 0 {
		return nil, types.ErrNotFound
	}

	if info != nil && info.IsDir() {
		return nil, types.ErrIsGroup
	}

	f, err := os.OpenFile(path, flag, defaultLocalFileMode)
	if err != nil {
		l.logger.Errorw("open file failed", "path", path, "err", err.Error())
	}
	return f, err
}

func (l *local) key2LocalPath(key int64, idx int64) string {
	dataPath := path.Join(l.dir, fmt.Sprintf("%d", key))
	if _, err := os.Stat(dataPath); err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(dataPath, defaultLocalDirMode)
			if err != nil {
				l.logger.Errorw("data path mkdir failed", "path", dataPath)
			}
		}
	}
	return path.Join(l.dir, fmt.Sprintf("%d/%d", key, idx))
}

func newLocalStorage(sid, dir string) Storage {
	return &local{
		sid:    sid,
		dir:    dir,
		logger: logger.NewLogger("localStorage"),
	}
}
