package files

import (
	"errors"
	"io"
	"sync/atomic"
)

type dataReader struct {
	reader io.Reader
}

func (d dataReader) Read(p []byte) (n int, err error) {
	return d.reader.Read(p)
}

func (d dataReader) Close() error {
	return nil
}

var (
	bufIsEmptyErr = errors.New("ring buffer is empty")
	bufIsFullErr  = errors.New("ring buffer is full")
)

// ringbuf: a SPSC buf queue
type ringbuf struct {
	head  uint32
	tail  uint32
	queue [bufQueueLen]cRange
	mask  uint32
}

func (b *ringbuf) put(index, offset int64, data []byte) error {
	var tail, head, next uint32
	for {
		tail = atomic.LoadUint32(&b.tail)
		head = atomic.LoadUint32(&b.head)

		next = (tail + 1) & b.mask
		if next == head&b.mask {
			return bufIsFullErr
		}

		if atomic.CompareAndSwapUint32(&b.tail, tail, next) {
			break
		}
	}

	b.queue[next].index, b.queue[next].offset = index, offset
	b.queue[next].data = data
	return nil
}

func (b *ringbuf) pop() (*cRange, bool) {
	var tail, head, next uint32
	for {
		tail = atomic.LoadUint32(&b.tail)
		head = atomic.LoadUint32(&b.head)
		if tail == head {
			return nil, false
		}

		next = (head + 1) & b.mask
		if atomic.CompareAndSwapUint32(&b.head, head, next) {
			break
		}
	}
	return &b.queue[next], true
}

func (b *ringbuf) len() int {
	return int(b.head - b.tail)
}

func newBuf() *ringbuf {
	return &ringbuf{
		queue: [bufQueueLen]cRange{},
		mask:  bufQueueLen - 1,
	}
}
