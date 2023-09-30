package packer

import "context"

type Packer interface {
	Pack(ctx context.Context, opt Option) error
	ReadContent(ctx context.Context, opt Option) (string, error)
}
