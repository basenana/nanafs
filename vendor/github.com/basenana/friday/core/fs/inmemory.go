package fs

import (
	"errors"
	"strings"
	"sync"
)

type inMemory struct {
	mu    sync.RWMutex
	files map[string]string
	dirs  map[string][]string
}

func NewInMemory() FileSystem {
	return &inMemory{
		files: make(map[string]string),
		dirs:  make(map[string][]string),
	}
}

func (i *inMemory) Ls(dirPath string) ([]string, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	entries, ok := i.dirs[dirPath]
	if !ok {
		return nil, errors.New("directory not found")
	}
	return entries, nil
}
func (i *inMemory) MkdirAll(dirPath string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Directory already exists
	if _, ok := i.dirs[dirPath]; ok {
		return nil
	}

	i.dirs[dirPath] = []string{}

	// Update parent directory listing
	lastSlash := strings.LastIndex(dirPath, "/")
	if lastSlash > 0 {
		parentDir := dirPath[:lastSlash]
		dirName := dirPath[lastSlash+1:]
		if _, ok := i.dirs[parentDir]; !ok {
			i.dirs[parentDir] = []string{}
		}
		// Add directory if not exists
		for _, name := range i.dirs[parentDir] {
			if name == dirName {
				return nil
			}
		}
		i.dirs[parentDir] = append(i.dirs[parentDir], dirName)
	}

	return nil
}

func (i *inMemory) Read(filePath string) (string, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	content, ok := i.files[filePath]
	if !ok {
		return "", errors.New("file not found")
	}
	return content, nil
}

func (i *inMemory) Write(filePath string, data string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.files[filePath] = data

	// Update parent directory listing
	lastSlash := strings.LastIndex(filePath, "/")
	if lastSlash > 0 {
		dirPath := filePath[:lastSlash]
		fileName := filePath[lastSlash+1:]
		if _, ok := i.dirs[dirPath]; !ok {
			i.dirs[dirPath] = []string{}
		}
		// Add file if not exists
		for _, name := range i.dirs[dirPath] {
			if name == fileName {
				return nil
			}
		}
		i.dirs[dirPath] = append(i.dirs[dirPath], fileName)
	}

	return nil
}

func (i *inMemory) Delete(path string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Try to delete as file
	if _, ok := i.files[path]; ok {
		delete(i.files, path)

		// Remove from parent directory
		lastSlash := strings.LastIndex(path, "/")
		if lastSlash > 0 {
			dirPath := path[:lastSlash]
			fileName := path[lastSlash+1:]
			if entries, ok := i.dirs[dirPath]; ok {
				for j, name := range entries {
					if name == fileName {
						i.dirs[dirPath] = append(entries[:j], entries[j+1:]...)
						break
					}
				}
			}
		}
		return nil
	}

	// Try to delete as directory
	if _, ok := i.dirs[path]; ok {
		delete(i.dirs, path)

		// Remove from parent directory
		lastSlash := strings.LastIndex(path, "/")
		if lastSlash > 0 {
			dirPath := path[:lastSlash]
			dirName := path[lastSlash+1:]
			if entries, ok := i.dirs[dirPath]; ok {
				for j, name := range entries {
					if name == dirName {
						i.dirs[dirPath] = append(entries[:j], entries[j+1:]...)
						break
					}
				}
			}
		}
		return nil
	}

	return errors.New("path not found")
}
