package fs

type FileSystem interface {
	Ls(dirPath string) ([]string, error)
	MkdirAll(dirPath string) error
	Read(filePath string) (string, error)
	Write(filePath string, data string) error
	Delete(path string) error
}
