package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FileAccess struct {
	workdir string
}

func NewFileAccess(workdir string) *FileAccess {
	if workdir == "" {
		workdir = os.TempDir()
	}
	return &FileAccess{
		workdir: filepath.Clean(workdir),
	}
}

func (fa *FileAccess) ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	if strings.Contains(path, "\x00") {
		return fmt.Errorf("null character in path is not allowed")
	}

	path = filepath.Clean(path)

	if filepath.IsAbs(path) {
		return fmt.Errorf("absolute paths are not allowed: %s", path)
	}

	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal is not allowed: %s", path)
	}

	absPath := filepath.Join(fa.workdir, path)
	if !strings.HasPrefix(absPath, fa.workdir) {
		return fmt.Errorf("path is outside workdir: %s", path)
	}

	return nil
}

func (fa *FileAccess) GetAbsPath(path string) (string, error) {
	// Empty path check (must be before filepath.Clean as it returns ".")
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Handle null characters
	if strings.Contains(path, "\x00") {
		return "", fmt.Errorf("null character in path is not allowed")
	}

	// Clean the path
	path = filepath.Clean(path)

	// Check for path traversal
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path traversal is not allowed: %s", path)
	}

	// If it's an absolute path, check if it's within workdir
	if filepath.IsAbs(path) {
		// Check if the absolute path is within workdir
		if !strings.HasPrefix(path, fa.workdir) {
			return "", fmt.Errorf("path is outside workdir: %s", path)
		}
		return path, nil
	}

	return filepath.Join(fa.workdir, path), nil
}

func (fa *FileAccess) Read(path string) ([]byte, error) {
	absPath, err := fa.GetAbsPath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(absPath)
}

func (fa *FileAccess) Write(path string, data []byte, perm os.FileMode) error {
	absPath, err := fa.GetAbsPath(path)
	if err != nil {
		return err
	}
	return os.WriteFile(absPath, data, perm)
}

func (fa *FileAccess) Create(path string, perm os.FileMode) (*os.File, error) {
	absPath, err := fa.GetAbsPath(path)
	if err != nil {
		return nil, err
	}
	return os.Create(absPath)
}

func (fa *FileAccess) Open(path string) (*os.File, error) {
	absPath, err := fa.GetAbsPath(path)
	if err != nil {
		return nil, err
	}
	return os.Open(absPath)
}

func (fa *FileAccess) MkdirAll(path string, perm os.FileMode) error {
	absPath, err := fa.GetAbsPath(path)
	if err != nil {
		return err
	}
	return os.MkdirAll(absPath, perm)
}

func (fa *FileAccess) Stat(path string) (os.FileInfo, error) {
	absPath, err := fa.GetAbsPath(path)
	if err != nil {
		return nil, err
	}
	return os.Stat(absPath)
}

func (fa *FileAccess) Rename(oldPath, newPath string) error {
	oldAbsPath, err := fa.GetAbsPath(oldPath)
	if err != nil {
		return err
	}
	newAbsPath, err := fa.GetAbsPath(newPath)
	if err != nil {
		return err
	}
	return os.Rename(oldAbsPath, newAbsPath)
}

func (fa *FileAccess) Remove(path string) error {
	absPath, err := fa.GetAbsPath(path)
	if err != nil {
		return err
	}
	return os.Remove(absPath)
}

func (fa *FileAccess) Exists(path string) bool {
	absPath, err := fa.GetAbsPath(path)
	if err != nil {
		return false
	}
	_, err = os.Stat(absPath)
	return err == nil
}

func (fa *FileAccess) Copy(dstPath, srcPath string, perm os.FileMode) error {
	srcAbsPath, err := fa.GetAbsPath(srcPath)
	if err != nil {
		return err
	}
	dstAbsPath, err := fa.GetAbsPath(dstPath)
	if err != nil {
		return err
	}

	srcFile, err := os.Open(srcAbsPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstAbsPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}
	return dstFile.Chmod(perm)
}

func (fa *FileAccess) Workdir() string {
	return fa.workdir
}
