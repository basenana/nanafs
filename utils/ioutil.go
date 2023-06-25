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
	"fmt"
	"io"
	"os"
)

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

type dataReader struct {
	reader io.Reader
}

func (d dataReader) Read(p []byte) (n int, err error) {
	return d.reader.Read(p)
}

func (d dataReader) Close() error {
	return nil
}

func NewDateReader(reader io.Reader) io.ReadCloser {
	return dataReader{reader: reader}
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

type zeroDevice struct{}

func (z zeroDevice) Read(p []byte) (n int, err error) {
	for i := 0; i < len(p); i++ {
		p[i] = 0
		n++
	}
	return
}

var ZeroDevice = zeroDevice{}
