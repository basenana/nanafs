package files

import "context"

type reader struct {
	f *File
}

func (r *reader) read(ctx context.Context, data []byte, offset int64) (int, error) {
	return 0, nil
}

func (r *reader) close(ctx context.Context) error {
	return nil
}

type writer struct {
	f *File
}

func (w *writer) write(ctx context.Context, data []byte, offset int64) (int64, error) {
	return 0, nil
}

func (w *writer) fsync(ctx context.Context) error {
	return nil
}

func (w *writer) close(ctx context.Context) error {
	return nil
}
