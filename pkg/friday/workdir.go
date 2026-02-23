package friday

import (
	"context"
	"errors"
	"io"
	"path"
	"strings"

	"github.com/basenana/friday/core/fs"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/types"
)

const WorkdirName = "workdir"

type workdirFS struct {
	fs      *core.FileSystem
	workdir string
}

func newWorkdirFS(fs *core.FileSystem, sessionID string) fs.FileSystem {
	return &workdirFS{
		fs:      fs,
		workdir: path.Join("/", SessionsDirName, sessionID, WorkdirName),
	}
}

func (w *workdirFS) resolvePath(inputPath string) (string, error) {
	cleanedPath := path.Clean("/" + inputPath)
	if !strings.HasPrefix(cleanedPath, w.workdir) && cleanedPath != w.workdir {
		return "", errors.New("path outside workdir")
	}
	relPath := strings.TrimPrefix(cleanedPath, w.workdir)
	relPath = strings.TrimPrefix(relPath, "/")
	if relPath == "" {
		relPath = "."
	}
	return relPath, nil
}

func (w *workdirFS) fullPath(relPath string) string {
	if relPath == "" {
		return w.workdir
	}
	return path.Join(w.workdir, relPath)
}

func (w *workdirFS) Ls(dirPath string) ([]string, error) {
	ctx := context.Background()
	fullPath := w.fullPath(dirPath)

	_, entry, err := w.fs.GetEntryByPath(ctx, fullPath)
	if err != nil {
		return nil, err
	}

	if !entry.IsGroup {
		return nil, errors.New("not a directory")
	}

	dir, err := w.fs.OpenDir(ctx, entry.ID)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	var result []string
	for {
		children, err := dir.Readdir(-1)
		if err != nil {
			break
		}
		for _, ch := range children {
			result = append(result, ch.Name())
		}
	}
	return result, nil
}

func (w *workdirFS) MkdirAll(dirPath string) error {
	ctx := context.Background()
	fullPath := w.fullPath(dirPath)

	parentPath := path.Dir(fullPath)
	name := path.Base(fullPath)

	if parentPath != w.workdir {
		if err := w.MkdirAll(path.Join(w.workdir, parentPath)); err != nil {
			return err
		}
	}

	attr := types.EntryAttr{
		Name: name,
		Kind: types.GroupKind,
	}

	_, err := w.fs.CreateEntry(ctx, parentPath, attr)
	if err != nil && !errors.Is(err, types.ErrIsExist) {
		return err
	}
	return nil
}

func (w *workdirFS) Read(filePath string) (string, error) {
	ctx := context.Background()
	fullPath := w.fullPath(filePath)

	_, entry, err := w.fs.GetEntryByPath(ctx, fullPath)
	if err != nil {
		return "", err
	}

	if entry.IsGroup {
		return "", errors.New("cannot read directory")
	}

	file, err := w.fs.Open(ctx, entry.ID, types.OpenAttr{Read: true})
	if err != nil {
		return "", err
	}
	defer file.Close()

	data := make([]byte, entry.Size)
	_, err = file.Read(data)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	return string(data), nil
}

func (w *workdirFS) Write(filePath string, data string) error {
	ctx := context.Background()
	fullPath := w.fullPath(filePath)

	parentPath := path.Dir(fullPath)
	name := path.Base(fullPath)

	var entry *types.Entry

	_, existingEntry, err := w.fs.GetEntryByPath(ctx, fullPath)
	if err == nil {
		entry = existingEntry
	} else {
		if !errors.Is(err, types.ErrNotFound) {
			return err
		}
		newEntry, err := w.fs.CreateEntry(ctx, parentPath, types.EntryAttr{
			Name: name,
			Kind: types.FileKind(name, types.RawKind),
		})
		if err != nil {
			return err
		}
		entry = newEntry
	}

	if entry.IsGroup {
		return errors.New("cannot write to directory")
	}

	file, err := w.fs.Open(ctx, entry.ID, types.OpenAttr{Write: true, Trunc: true})
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write([]byte(data))
	return err
}

func (w *workdirFS) Delete(targetPath string) error {
	ctx := context.Background()
	fullPath := w.fullPath(targetPath)

	_, entry, err := w.fs.GetEntryByPath(ctx, fullPath)
	if err != nil {
		return err
	}

	if entry.IsGroup {
		return w.fs.RmGroup(ctx, fullPath, types.DestroyEntryAttr{})
	}
	return w.fs.UnlinkEntry(ctx, fullPath, types.DestroyEntryAttr{})
}
