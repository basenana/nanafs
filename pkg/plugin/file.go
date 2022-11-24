package plugin

import (
	"context"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/types"
)

type File struct {
}

var _ files.File = File{}

func (d File) GetObject() *types.Object {
	//TODO implement me
	panic("implement me")
}

func (d File) GetAttr() files.Attr {
	//TODO implement me
	panic("implement me")
}

func (d File) Write(ctx context.Context, data []byte, offset int64) (n int64, err error) {
	//TODO implement me
	panic("implement me")
}

func (d File) Read(ctx context.Context, data []byte, offset int64) (int, error) {
	//TODO implement me
	panic("implement me")
}

func (d File) Fsync(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (d File) Flush(ctx context.Context) (err error) {
	//TODO implement me
	panic("implement me")
}

func (d File) Close(ctx context.Context) (err error) {
	//TODO implement me
	panic("implement me")
}
