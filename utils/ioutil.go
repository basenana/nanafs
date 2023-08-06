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

package utils

import (
	"context"
	"fmt"
	"io"
	"os"
)

type ContextWriterAt interface {
	WriteAt(ctx context.Context, data []byte, off int64) (int64, error)
}

type ContextReaderAt interface {
	ReadAt(ctx context.Context, dest []byte, off int64) (int64, error)
}

func Mkdir(path string) error {
	d, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err != nil && os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}

	if d.IsDir() {
		return nil
	}

	return fmt.Errorf("%s not dir", path)
}

type wrapperReader struct {
	r   io.ReaderAt
	off int64
}

func (w *wrapperReader) Read(p []byte) (n int, err error) {
	n, err = w.r.ReadAt(p, w.off)
	w.off += int64(n)
	return
}

func NewReader(reader io.ReaderAt) io.Reader {
	return &wrapperReader{r: reader}
}

func NewReaderWithOffset(reader io.ReaderAt, off int64) io.Reader {
	return &wrapperReader{r: reader, off: off}
}

type wrapperWriter struct {
	w   io.WriterAt
	off int64
}

func (w *wrapperWriter) Write(p []byte) (n int, err error) {
	n, err = w.w.WriteAt(p, w.off)
	w.off += int64(n)
	return
}

func NewWriter(writer io.WriterAt) io.Writer {
	return &wrapperWriter{w: writer}
}

func NewWriterWithOffset(writer io.WriterAt, off int64) io.Writer {
	return &wrapperWriter{w: writer, off: off}
}

type wrapperContextReader struct {
	ctx context.Context
	r   ContextReaderAt
	off int64
}

func (w *wrapperContextReader) Read(p []byte) (n int, err error) {
	var n64 int64
	n64, err = w.r.ReadAt(w.ctx, p, w.off)
	w.off += n64
	n = int(n64)
	return
}

func NewReaderWithContextReaderAt(ctx context.Context, reader ContextReaderAt) io.Reader {
	return &wrapperContextReader{ctx: ctx, r: reader}
}

type wrapperContextWriter struct {
	ctx context.Context
	w   ContextWriterAt
	off int64
}

func (w *wrapperContextWriter) Write(p []byte) (n int, err error) {
	var n64 int64
	n64, err = w.w.WriteAt(w.ctx, p, w.off)
	w.off += n64
	n = int(n64)
	return
}

func NewWriterWithContextWriter(ctx context.Context, writer ContextWriterAt) io.Writer {
	return &wrapperContextWriter{ctx: ctx, w: writer}
}

type zeroDevice struct{}

func (z zeroDevice) Read(p []byte) (n int, err error) {
	for i := 0; i < len(p); i++ {
		p[i] = 0
		n++
	}
	return
}

var ZeroDevice = zeroDevice{}
