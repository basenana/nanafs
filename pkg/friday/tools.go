package friday

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/basenana/friday/core/tools"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/types"
)

// file_read tool - Read file contents from NanaFS
func (f *Friday) newFileReadTool() *tools.Tool {
	return tools.NewTool(
		"file_read",
		tools.WithDescription("Read file contents from NanaFS"),
		tools.WithString("path",
			tools.Required(),
			tools.Description("File path"),
		),
		tools.WithToolHandler(func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
			pathVal, ok := request.Arguments["path"].(string)
			if !ok || pathVal == "" {
				return tools.NewToolResultError("missing required parameter: path"), nil
			}

			_, entry, err := f.resolveEntry(ctx, pathVal)
			if err != nil {
				return tools.NewToolResultError(err.Error()), nil
			}

			if entry.IsGroup {
				return tools.NewToolResultError("path is a directory"), nil
			}

			file, err := f.fs.Open(ctx, entry.ID, types.OpenAttr{Read: true})
			if err != nil {
				return tools.NewToolResultError("open file failed"), nil
			}
			defer file.Close()

			data := make([]byte, entry.Size)
			_, err = file.Read(data)
			if err != nil && !errors.Is(err, io.EOF) {
				return tools.NewToolResultError("read file failed"), nil
			}

			return tools.NewToolResultText(string(data)), nil
		}),
	)
}

// file_write tool - Write content to a file in NanaFS
func (f *Friday) newFileWriteTool() *tools.Tool {
	return tools.NewTool(
		"file_write",
		tools.WithDescription("Write content to a file in NanaFS"),
		tools.WithString("path",
			tools.Required(),
			tools.Description("File path"),
		),
		tools.WithString("content",
			tools.Required(),
			tools.Description("Content to write"),
		),
		tools.WithToolHandler(func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
			pathVal, ok := request.Arguments["path"].(string)
			if !ok || pathVal == "" {
				return tools.NewToolResultError("missing required parameter: path"), nil
			}

			content, ok := request.Arguments["content"].(string)
			if !ok {
				content = ""
			}

			entryPath, err := parsePath(pathVal)
			if err != nil {
				return tools.NewToolResultError("invalid path"), nil
			}

			parentURI, name := splitParentAndName(entryPath)
			if name == "" {
				return tools.NewToolResultError("invalid file name"), nil
			}

			_, entry, err := f.resolveEntry(ctx, pathVal)
			if err != nil && !isNotFoundError(err) {
				return tools.NewToolResultError(err.Error()), nil
			}

			var file core.File
			if isNotFoundError(err) {
				// File doesn't exist, create it
				attr := types.EntryAttr{
					Name: name,
					Kind: types.FileKind(name, types.RawKind),
				}
				newEntry, err := f.fs.CreateEntry(ctx, parentURI, attr)
				if err != nil {
					return tools.NewToolResultError("create entry failed"), nil
				}
				entry = newEntry
			}

			if entry.IsGroup {
				return tools.NewToolResultError("path is a directory"), nil
			}

			file, err = f.fs.Open(ctx, entry.ID, types.OpenAttr{Write: true, Trunc: true})
			if err != nil {
				return tools.NewToolResultError("open file failed"), nil
			}
			defer file.Close()

			_, err = file.Write([]byte(content))
			if err != nil {
				return tools.NewToolResultError("write file failed"), nil
			}

			return tools.NewToolResultText("file written successfully"), nil
		}),
	)
}

// file_list tool - List files and directories in a path
func (f *Friday) newFileListTool() *tools.Tool {
	return tools.NewTool(
		"file_list",
		tools.WithDescription("List files and directories in a path"),
		tools.WithString("path",
			tools.Description("Directory path, default is root"),
		),
		tools.WithToolHandler(func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
			pathVal := "."
			if p, ok := request.Arguments["path"].(string); ok && p != "" {
				pathVal = p
			}

			_, entry, err := f.resolveEntry(ctx, pathVal)
			if err != nil {
				return tools.NewToolResultError(err.Error()), nil
			}

			if !entry.IsGroup {
				return tools.NewToolResultError("path is not a directory"), nil
			}

			dir, err := f.fs.OpenDir(ctx, entry.ID)
			if err != nil {
				return tools.NewToolResultError("open directory failed"), nil
			}
			defer dir.Close()

			var list []fileInfo
			for {
				children, err := dir.Readdir(-1)
				if err != nil {
					break
				}
				for _, ch := range children {
					list = append(list, fileInfo{
						Name:     ch.Name(),
						Size:     formatSize(ch.Size()),
						Modified: ch.ModTime().Format("2006-01-02 15:04:05"),
						IsDir:    ch.IsDir(),
					})
				}
			}

			data, _ := json.Marshal(list)
			return tools.NewToolResultText(string(data)), nil
		}),
	)
}

// file_stat tool - Get file metadata
func (f *Friday) newFileStatTool() *tools.Tool {
	return tools.NewTool(
		"file_stat",
		tools.WithDescription("Get file metadata"),
		tools.WithString("path",
			tools.Required(),
			tools.Description("File or directory path"),
		),
		tools.WithToolHandler(func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
			pathVal, ok := request.Arguments["path"].(string)
			if !ok || pathVal == "" {
				return tools.NewToolResultError("missing required parameter: path"), nil
			}

			_, entry, err := f.resolveEntry(ctx, pathVal)
			if err != nil {
				return tools.NewToolResultError(err.Error()), nil
			}

			info := entryToStatInfo(entry)
			data, _ := json.Marshal(info)
			return tools.NewToolResultText(string(data)), nil
		}),
	)
}

// mkdir tool - Create a new directory
func (f *Friday) newMkdirTool() *tools.Tool {
	return tools.NewTool(
		"mkdir",
		tools.WithDescription("Create a new directory"),
		tools.WithString("path",
			tools.Required(),
			tools.Description("New directory path"),
		),
		tools.WithToolHandler(func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
			pathVal, ok := request.Arguments["path"].(string)
			if !ok || pathVal == "" {
				return tools.NewToolResultError("missing required parameter: path"), nil
			}

			entryPath, err := parsePath(pathVal)
			if err != nil {
				return tools.NewToolResultError("invalid path"), nil
			}

			parentURI, name := splitParentAndName(entryPath)
			if name == "" {
				return tools.NewToolResultError("invalid directory name"), nil
			}

			// Check if already exists
			_, existingEntry, err := f.resolveEntry(ctx, pathVal)
			if err == nil {
				if existingEntry.IsGroup {
					return tools.NewToolResultError("directory already exists"), nil
				}
				return tools.NewToolResultError("a file with this name already exists"), nil
			}

			attr := types.EntryAttr{
				Name: name,
				Kind: types.GroupKind,
			}
			_, err = f.fs.CreateEntry(ctx, parentURI, attr)
			if err != nil {
				return tools.NewToolResultError("create directory failed"), nil
			}

			return tools.NewToolResultText("directory created successfully"), nil
		}),
	)
}

// rename tool - Rename a file or directory
func (f *Friday) newRenameTool() *tools.Tool {
	return tools.NewTool(
		"rename",
		tools.WithDescription("Rename a file or directory"),
		tools.WithString("src",
			tools.Required(),
			tools.Description("Source path"),
		),
		tools.WithString("dest",
			tools.Required(),
			tools.Description("Destination path"),
		),
		tools.WithToolHandler(func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
			srcVal, ok := request.Arguments["src"].(string)
			if !ok || srcVal == "" {
				return tools.NewToolResultError("missing required parameter: src"), nil
			}

			destVal, ok := request.Arguments["dest"].(string)
			if !ok || destVal == "" {
				return tools.NewToolResultError("missing required parameter: dest"), nil
			}

			_, _, err := f.resolveEntry(ctx, srcVal)
			if err != nil {
				return tools.NewToolResultError(err.Error()), nil
			}

			srcEntryPath, _ := parsePath(srcVal)
			destEntryPath, err := parsePath(destVal)
			if err != nil {
				return tools.NewToolResultError("invalid dest path"), nil
			}

			destParentURI, destName := splitParentAndName(destEntryPath)
			if destName == "" {
				return tools.NewToolResultError("invalid destination name"), nil
			}

			err = f.fs.Rename(ctx, srcEntryPath, destParentURI, destName, types.ChangeParentAttr{})
			if err != nil {
				return tools.NewToolResultError("rename failed"), nil
			}

			return tools.NewToolResultText("renamed successfully"), nil
		}),
	)
}

// delete tool - Delete a file or directory
func (f *Friday) newDeleteTool() *tools.Tool {
	return tools.NewTool(
		"delete",
		tools.WithDescription("Delete a file or directory"),
		tools.WithString("path",
			tools.Required(),
			tools.Description("File or directory path to delete"),
		),
		tools.WithToolHandler(func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
			pathVal, ok := request.Arguments["path"].(string)
			if !ok || pathVal == "" {
				return tools.NewToolResultError("missing required parameter: path"), nil
			}

			entryPath, err := parsePath(pathVal)
			if err != nil {
				return tools.NewToolResultError("invalid path"), nil
			}

			_, entry, err := f.resolveEntry(ctx, pathVal)
			if err != nil {
				return tools.NewToolResultError(err.Error()), nil
			}

			if entry.IsGroup {
				err = f.fs.RmGroup(ctx, entryPath, types.DestroyEntryAttr{})
			} else {
				err = f.fs.UnlinkEntry(ctx, entryPath, types.DestroyEntryAttr{})
			}
			if err != nil {
				return tools.NewToolResultError("delete failed"), nil
			}

			return tools.NewToolResultText("deleted successfully"), nil
		}),
	)
}
