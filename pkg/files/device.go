package files

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	symLinkAnnotationKey = "nanafs.symlink"
)

type Symlink struct {
	data []byte
	obj  *types.Object
}

func (s *Symlink) GetObject() *types.Object {
	return s.obj
}

func (s *Symlink) Write(ctx context.Context, data []byte, offset int64) (n int64, err error) {
	size := offset + int64(len(data))

	if size > int64(len(s.data)) {
		newData := make([]byte, size)
		copy(newData, s.data)
		s.data = newData
	}

	n = int64(copy(s.data[offset:], data))
	s.obj.Size = size
	return
}

func (s *Symlink) Read(ctx context.Context, data []byte, offset int64) (int, error) {
	n := copy(data, s.data[offset:])
	return n, nil
}

func (s *Symlink) Fsync(ctx context.Context) error {
	return s.Flush(ctx)
}

func (s *Symlink) Flush(ctx context.Context) (err error) {
	s.obj.ExtendData.Annotation.Add(&types.AnnotationItem{
		Key:     symLinkAnnotationKey,
		Content: string(s.data),
	})
	return nil
}

func (s *Symlink) Close(ctx context.Context) (err error) {
	return s.Flush(ctx)
}

func openSymlink(obj *types.Object) (*Symlink, error) {
	if obj.Kind != types.SymLinkKind {
		return nil, fmt.Errorf("not symlink")
	}

	var raw []byte
	ann := dentry.GetInternalAnnotation(obj, symLinkAnnotationKey)
	if ann != nil {
		raw = []byte(ann.Content)
	}

	if raw == nil {
		raw = make([]byte, 0, 512)
	}

	return &Symlink{obj: obj, data: raw}, nil
}
