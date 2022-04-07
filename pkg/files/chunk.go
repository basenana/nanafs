package files

const (
	defaultChunkSize = 1 << 22 // 4MB
)

var fileChunkSize int64 = defaultChunkSize

func computeChunkIndex(off, chunkSize int64) (idx int64, pos int64) {
	idx = off / chunkSize
	pos = off % chunkSize
	return
}
