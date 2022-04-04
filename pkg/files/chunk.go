package files

const (
	defaultChunkSize = 1 << 22 // 4MB
)

func computeChunkIndex(off, chunkSize int64) (idx int64, pos int64) {
	idx = off / chunkSize
	pos = off % chunkSize
	return
}
