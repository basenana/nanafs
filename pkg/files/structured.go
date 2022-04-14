package files

import (
	"context"
	"encoding/json"
	"github.com/basenana/nanafs/pkg/types"
)

type structured struct {
	*types.Object
	cType   types.Kind
	version string
	raw     []byte
	attr    Attr
	spec    interface{}
}

func (s *structured) GetObject() *types.Object {
	return s.Object
}

func (s *structured) Write(ctx context.Context, data []byte, offset int64) (n int64, err error) {
	rl := int64(len(s.raw))
	dl := int64(len(data))
	if rl > dl+offset {
		return int64(copy(s.raw[offset:], data)), nil
	}
	newBuf := make([]byte, offset+dl-rl)
	s.raw = append(s.raw, newBuf...)
	n = int64(copy(s.raw[offset:], data))
	s.Size = int64(len(s.raw))
	return
}

func (s *structured) Read(ctx context.Context, data []byte, offset int64) (int, error) {
	return copy(data, s.raw[offset:]), nil
}

func (s *structured) Fsync(ctx context.Context) error {
	return nil
}

func (s *structured) Flush(ctx context.Context) (err error) {
	return nil
}

func (s *structured) Close(ctx context.Context) (err error) {
	err = json.Unmarshal(s.raw, s.spec)
	if err != nil {
		return err
	}
	return s.attr.Meta.SaveContent(ctx, s.Object, s.cType, s.version, s.spec)
}

func openStructuredFile(ctx context.Context, obj *types.Object, spec interface{}, attr Attr) (*structured, error) {
	err := attr.Meta.LoadContent(ctx, obj, obj.Kind, obj.Labels.Get(types.VersionKey).Value, spec)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	obj.Size = int64(len(raw))
	return &structured{
		Object:  obj,
		cType:   obj.Kind,
		version: obj.Labels.Get(types.VersionKey).Value,
		raw:     raw,
		attr:    attr,
		spec:    spec,
	}, nil
}

func OpenStructured(ctx context.Context, obj *types.Object, spec interface{}, attr Attr) (File, error) {
	return openStructuredFile(ctx, obj, spec, attr)
}
