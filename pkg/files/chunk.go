package files

const (
	defaultChunkSize = 1 << 22 // 4MB
)

func computeChunkIndex(off int64) (idx int64, pos int64) {
	idx = off / defaultChunkSize
	pos = off % defaultChunkSize
	return
}
