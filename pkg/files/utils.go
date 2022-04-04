package files

import "io"

type dataReader struct {
	reader io.Reader
}

func (d dataReader) Read(p []byte) (n int, err error) {
	return d.reader.Read(p)
}

func (d dataReader) Close() error {
	return nil
}
