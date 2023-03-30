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

import "io"

type readerAtWrapper struct {
	io.ReaderAt
	off int64
}

func (r *readerAtWrapper) Read(p []byte) (n int, err error) {
	n, err = r.ReadAt(p, r.off)
	r.off += int64(n)
	return n, err
}

func NewReaderAtWrapper(at io.ReaderAt) io.Reader {
	return &readerAtWrapper{ReaderAt: at}
}

type writeAtWrapper struct {
	io.WriterAt
	off int64
}

func (w *writeAtWrapper) Write(p []byte) (n int, err error) {
	n, err = w.WriteAt(p, w.off)
	w.off += int64(n)
	return n, err
}

func NewWriteAtWrapper(at io.WriterAt) io.Writer {
	return &writeAtWrapper{WriterAt: at}
}
