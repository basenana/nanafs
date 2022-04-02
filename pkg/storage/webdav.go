package storage

import (
	"context"
	"io"
)

type webdav struct{}

var _ Storage = &webdav{}

func (w webdav) ID() string {
	//TODO implement me
	panic("implement me")
}

func (w webdav) Get(ctx context.Context, key string, idx, off, limit int64) (io.ReadCloser, error) {
	//TODO implement me
	panic("implement me")
}

func (w webdav) Put(ctx context.Context, key string, in io.Reader, idx, off int64) error {
	//TODO implement me
	panic("implement me")
}

func (w webdav) Delete(ctx context.Context, key string) error {
	//TODO implement me
	panic("implement me")
}

func (w webdav) Head(ctx context.Context, key string) (Info, error) {
	//TODO implement me
	panic("implement me")
}

func (w webdav) Fsync(ctx context.Context, key string) error {
	//TODO implement me
	panic("implement me")
}
