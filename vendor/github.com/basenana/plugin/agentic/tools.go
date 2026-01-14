package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	fridaytools "github.com/basenana/friday/core/tools"
	"github.com/basenana/plugin/utils"
)

func FileAccessTools(workdir string) []*fridaytools.Tool {
	fileAccess := utils.NewFileAccess(workdir)
	return []*fridaytools.Tool{
		NewFileReadTool(fileAccess),
		NewFileWriteTool(fileAccess),
		NewFileListTool(fileAccess),
		NewFileParseTool(fileAccess),
	}
}

func NewFileReadTool(fileAccess *utils.FileAccess) *fridaytools.Tool {
	return fridaytools.NewTool(
		"file_read",
		fridaytools.WithDescription("Read file contents from working directory"),
		fridaytools.WithString("path",
			fridaytools.Required(),
			fridaytools.Description("Relative path to file within working directory"),
		),
		fridaytools.WithToolHandler(func(ctx context.Context, request *fridaytools.Request) (*fridaytools.Result, error) {
			path, ok := request.Arguments["path"].(string)
			if !ok || path == "" {
				return fridaytools.NewToolResultError("missing required parameter: path"), nil
			}

			data, err := fileAccess.Read(path)
			if err != nil {
				return fridaytools.NewToolResultError(err.Error()), nil
			}

			return fridaytools.NewToolResultText(string(data)), nil
		}),
	)
}

func NewFileWriteTool(fileAccess *utils.FileAccess) *fridaytools.Tool {
	return fridaytools.NewTool(
		"file_write",
		fridaytools.WithDescription("Write content to a file in working directory"),
		fridaytools.WithString("path",
			fridaytools.Required(),
			fridaytools.Description("Relative path to file within working directory"),
		),
		fridaytools.WithString("content",
			fridaytools.Required(),
			fridaytools.Description("Content to write to the file"),
		),
		fridaytools.WithToolHandler(func(ctx context.Context, request *fridaytools.Request) (*fridaytools.Result, error) {
			path, ok := request.Arguments["path"].(string)
			if !ok || path == "" {
				return fridaytools.NewToolResultError("missing required parameter: path"), nil
			}

			content, ok := request.Arguments["content"].(string)
			if !ok || content == "" {
				return fridaytools.NewToolResultError("missing required parameter: content"), nil
			}

			err := fileAccess.Write(path, []byte(content), 0644)
			if err != nil {
				return fridaytools.NewToolResultError(err.Error()), nil
			}

			return fridaytools.NewToolResultText(fmt.Sprintf("file written: %s", path)), nil
		}),
	)
}

func NewFileListTool(fileAccess *utils.FileAccess) *fridaytools.Tool {
	type fileInfo struct {
		Name     string `json:"name"`
		Size     int64  `json:"size"`
		Modified string `json:"modified"`
		IsDir    bool   `json:"is_dir"`
	}

	return fridaytools.NewTool(
		"file_list",
		fridaytools.WithDescription("List files and directories in a path"),
		fridaytools.WithString("path",
			fridaytools.Description("Relative path to directory within working directory, default is root"),
		),
		fridaytools.WithToolHandler(func(ctx context.Context, request *fridaytools.Request) (*fridaytools.Result, error) {
			path := "."
			if p, ok := request.Arguments["path"].(string); ok && p != "" {
				path = p
			}

			absPath, err := fileAccess.GetAbsPath(path)
			if err != nil {
				return fridaytools.NewToolResultError(fmt.Sprintf("invalid path: %s", err.Error())), nil
			}

			var list []fileInfo
			err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				relPath, _ := filepath.Rel(absPath, path)
				if relPath == "." {
					return nil
				}
				list = append(list, fileInfo{
					Name:     relPath,
					Size:     info.Size(),
					Modified: info.ModTime().Format("2006-01-02 15:04:05"),
					IsDir:    info.IsDir(),
				})
				return nil
			})
			if err != nil {
				return fridaytools.NewToolResultError(err.Error()), nil
			}

			data, _ := json.Marshal(list)
			return fridaytools.NewToolResultText(string(data)), nil
		}),
	)
}

func NewFileParseTool(fileAccess *utils.FileAccess) *fridaytools.Tool {
	return fridaytools.NewTool(
		"file_parse",
		fridaytools.WithDescription("Parse document file (PDF, HTML, Webarchive, etc.) and extract text content"),
		fridaytools.WithString("path",
			fridaytools.Required(),
			fridaytools.Description("Relative path to file within working directory"),
		),
		fridaytools.WithToolHandler(func(ctx context.Context, request *fridaytools.Request) (*fridaytools.Result, error) {
			path, ok := request.Arguments["path"].(string)
			if !ok || path == "" {
				return fridaytools.NewToolResultError("missing required parameter: path"), nil
			}

			absPath, err := fileAccess.GetAbsPath(path)
			if err != nil {
				return fridaytools.NewToolResultError(fmt.Sprintf("invalid file path: %s", err.Error())), nil
			}

			parser := newParser(absPath)
			if parser == nil {
				return fridaytools.NewToolResultError(fmt.Sprintf("unsupported file format: %s", filepath.Ext(path))), nil
			}

			doc, err := parser.Load(ctx)
			if err != nil {
				return fridaytools.NewToolResultError(err.Error()), nil
			}

			return fridaytools.NewToolResultText(doc.Content), nil
		}),
	)
}
