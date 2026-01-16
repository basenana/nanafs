package friday

import (
	"context"
	"fmt"

	"github.com/basenana/friday/core/tools"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/types"
)

type Friday struct {
	fs        *core.FileSystem
	namespace string
}

func NewFriday(fs *core.FileSystem) *Friday {
	return &Friday{
		fs:        fs,
		namespace: fs.Namespace(),
	}
}

// Tools returns all available filesystem tools
func (f *Friday) Tools() []*tools.Tool {
	return []*tools.Tool{
		f.newFileReadTool(),
		f.newFileWriteTool(),
		f.newFileListTool(),
		f.newFileStatTool(),
		f.newMkdirTool(),
		f.newRenameTool(),
		f.newDeleteTool(),
	}
}

// resolveEntry resolves a path to an entry, returns (parent, entry, error)
// Uses the Friday's default namespace since FileSystem embeds the namespace
func (f *Friday) resolveEntry(ctx context.Context, inputPath string) (*types.Entry, *types.Entry, error) {
	entryPath, err := parsePath(inputPath)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid path: %w", err)
	}

	if entryPath == "" {
		entryPath = "."
	}

	parent, entry, err := f.fs.GetEntryByPath(ctx, entryPath)
	if err != nil {
		return nil, nil, fmt.Errorf("entry not found: %w", err)
	}
	return parent, entry, nil
}
