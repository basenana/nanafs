package files

import (
	"bytes"
	"context"
	"io"
)

type reader struct {
	f *File
	p *pool

	chunkSize int64
}

func (r *reader) read(ctx context.Context, data []byte, offset int64) (int, error) {
	if offset > r.f.Size {
		return 0, io.EOF
	}

	limit := offset + int64(len(data))
	if limit > r.f.Size {
		limit = r.f.Size
	}

	off := offset
	readTask := make([]int, 0, 1)
	for {
		cID, pos := computeChunkIndex(off, r.chunkSize)

		cr := &cRange{key: r.f.ID, index: cID, offset: pos, limit: r.chunkSize - pos}
		if off+cr.limit > limit {
			cr.limit = limit - off
		}

		if cr.limit <= cr.offset {
			break
		}

		tID, err := r.p.dispatch(ctx, cr)
		if err != nil {
			return 0, err
		}
		readTask = append(readTask, tID)

		off += cr.limit - cr.offset
		if off >= limit {
			break
		}
	}

	var (
		readTotal = 0
		buf       = make([]byte, r.chunkSize)
	)
	for tID := range readTask {
		cr, err := r.p.wait(ctx, tID)
		if err != nil {
			return 0, err
		}

		n, err := cr.data.Read(buf)
		if err != nil && err != io.EOF {
			return readTotal, err
		}
		readTotal += copy(data[readTotal:], buf[:n])
	}
	if limit == r.f.Size {
		return readTotal, io.EOF
	}
	return readTotal, nil
}

func (r *reader) close(ctx context.Context) error {
	return r.p.close(ctx)
}

func initFileReader(f *File) *reader {
	return &reader{
		f:         f,
		p:         newReadWorkerPool(f.attr.Storage),
		chunkSize: f.chunkSize,
	}
}

type writer struct {
	f *File
	p *pool

	chunkSize int64
}

func (w *writer) write(ctx context.Context, data []byte, offset int64) (int64, error) {
	limit := int64(len(data)) + offset
	off := offset
	readTask := make([]int, 0, 1)

	for {
		cID, pos := computeChunkIndex(off, w.chunkSize)
		cr := &cRange{key: w.f.ID, index: cID, offset: pos, limit: w.chunkSize - pos}
		if int(cr.limit-cr.offset) > len(data) {
			cr.limit = int64(len(data)) + pos
		}

		cr.data = dataReader{reader: bytes.NewReader(data[:cr.limit-cr.offset])}
		data = data[cr.limit-cr.offset:]

		tID, err := w.p.dispatch(ctx, cr)
		if err != nil {
			return 0, err
		}
		readTask = append(readTask, tID)

		off += cr.limit - cr.offset
		if off >= limit {
			break
		}
	}

	var totalCount int64
	for tID := range readTask {
		cr, err := w.p.wait(ctx, tID)
		if err != nil {
			return 0, err
		}
		totalCount += cr.limit - cr.offset
	}
	return totalCount, nil
}

func (w *writer) fsync(ctx context.Context) error {
	return nil
}

func (w *writer) close(ctx context.Context) error {
	return w.p.close(ctx)
}

func initFileWriter(f *File) *writer {
	return &writer{
		f:         f,
		p:         newWriteWorkerPool(f.attr.Storage),
		chunkSize: f.chunkSize,
	}
}
