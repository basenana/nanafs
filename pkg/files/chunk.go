package files

import (
	"io"
	"sync/atomic"
)

const (
	defaultChunkSize = 1 << 22 // 4MB
	bufQueueSize     = 256
)

var fileChunkSize int64 = defaultChunkSize

func computeChunkIndex(off, chunkSize int64) (idx int64, pos int64) {
	idx = off / chunkSize
	pos = off % chunkSize
	return
}

type buf struct {
	head  uint32
	tail  uint32
	queue [bufQueueSize]cRange
	mask  uint32
}

func (b *buf) put(key string, index int64, offset int64, limit int64, data io.ReadCloser) bool {
	var tail, head, next uint32
	for {
		tail = atomic.LoadUint32(&b.tail)
		head = atomic.LoadUint32(&b.head)

		next = (tail + 1) & b.mask
		if next == head&b.mask {
			return false
		}

		if atomic.CompareAndSwapUint32(&b.tail, tail, next) {
			break
		}
	}
	b.queue[next].key = key
	b.queue[next].index = index
	b.queue[next].offset = offset
	b.queue[next].limit = limit
	b.queue[next].data = data
	return true
}

func (b *buf) pop() *cRange {
	var tail, head, next uint32
	for {
		tail = atomic.LoadUint32(&b.tail)
		head = atomic.LoadUint32(&b.head)
		if tail == head {
			return nil
		}

		next = (head - 1) & b.mask
		if atomic.CompareAndSwapUint32(&b.head, head, next) {
			break
		}
	}
	return &b.queue[next]
}

func (b *buf) len() int {
	return int(b.head - b.tail)
}

func newBuf() *buf {
	return &buf{
		queue: [bufQueueSize]cRange{},
		mask:  bufQueueSize - 1,
	}
}
