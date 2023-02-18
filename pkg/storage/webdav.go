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

func (w webdav) Get(ctx context.Context, key int64, idx, offset int64) (io.ReadCloser, error) {
	//TODO implement me
	panic("implement me")
}

func (w webdav) Put(ctx context.Context, key int64, idx, offset int64, in io.Reader) error {
	//TODO implement me
	panic("implement me")
}

func (w webdav) Delete(ctx context.Context, key int64) error {
	//TODO implement me
	panic("implement me")
}

func (w webdav) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	//TODO implement me
	panic("implement me")
}
