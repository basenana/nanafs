package files

const (
	fileChunkSize  = 1 << 22 // 4MB
	pageSize       = 1 << 12 // 4k
	pageCacheLimit = 1 << 24 // 16MB
	bufQueueLen    = 256
)

func computeChunkIndex(off, chunkSize int64) (idx int64, pos int64) {
	idx = off / chunkSize
	pos = off % chunkSize
	return
}

func computePageIndex(off int64) (idx int64, pos int64) {
	idx = off / pageSize
	pos = off % pageSize
	return
}

type cRange struct {
	index  int64
	offset int64
	data   []byte
}
