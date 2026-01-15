package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	fridaytools "github.com/basenana/friday/core/tools"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/utils"
	"go.uber.org/zap"
)

func FileAccessTools(workdir string, toolLogger *zap.SugaredLogger) []*fridaytools.Tool {
	fileAccess := utils.NewFileAccess(workdir)
	return []*fridaytools.Tool{
		NewFileReadTool(fileAccess, toolLogger),
		NewFileWriteTool(fileAccess, toolLogger),
		NewFileListTool(fileAccess, toolLogger),
		NewFileParseTool(fileAccess, toolLogger),
	}
}

func NewFileReadTool(fileAccess *utils.FileAccess, toolLogger *zap.SugaredLogger) *fridaytools.Tool {
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
				toolLogger.Warnw("missing required parameter: path")
				return fridaytools.NewToolResultError("missing required parameter: path"), nil
			}

			toolLogger.Infow("file_read started", "path", path)

			data, err := fileAccess.Read(path)
			if err != nil {
				toolLogger.Warnw("file_read failed", "path", path, "error", err)
				return fridaytools.NewToolResultError(err.Error()), nil
			}

			toolLogger.Infow("file_read completed", "path", path, "size", len(data))
			return fridaytools.NewToolResultText(string(data)), nil
		}),
	)
}

func NewFileWriteTool(fileAccess *utils.FileAccess, toolLogger *zap.SugaredLogger) *fridaytools.Tool {
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
				toolLogger.Warnw("missing required parameter: path")
				return fridaytools.NewToolResultError("missing required parameter: path"), nil
			}

			content, ok := request.Arguments["content"].(string)
			if !ok || content == "" {
				toolLogger.Warnw("missing required parameter: content")
				return fridaytools.NewToolResultError("missing required parameter: content"), nil
			}

			toolLogger.Infow("file_write started", "path", path, "content_len", len(content))

			err := fileAccess.Write(path, []byte(content), 0644)
			if err != nil {
				toolLogger.Warnw("file_write failed", "path", path, "error", err)
				return fridaytools.NewToolResultError(err.Error()), nil
			}

			toolLogger.Infow("file_write completed", "path", path)
			return fridaytools.NewToolResultText(fmt.Sprintf("file written: %s", path)), nil
		}),
	)
}

func NewFileListTool(fileAccess *utils.FileAccess, toolLogger *zap.SugaredLogger) *fridaytools.Tool {
	type fileInfo struct {
		Name     string `json:"name"`
		Size     string `json:"size"`
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

			toolLogger.Infow("file_list started", "path", path)

			absPath, err := fileAccess.GetAbsPath(path)
			if err != nil {
				toolLogger.Warnw("invalid path", "path", path, "error", err)
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
					Size:     formatSize(info.Size()),
					Modified: info.ModTime().Format("2006-01-02 15:04:05"),
					IsDir:    info.IsDir(),
				})
				return nil
			})
			if err != nil {
				toolLogger.Warnw("file_list failed", "path", path, "error", err)
				return fridaytools.NewToolResultError(err.Error()), nil
			}

			toolLogger.Infow("file_list completed", "path", path, "count", len(list))
			data, _ := json.Marshal(list)
			return fridaytools.NewToolResultText(string(data)), nil
		}),
	)
}

func NewFileParseTool(fileAccess *utils.FileAccess, toolLogger *zap.SugaredLogger) *fridaytools.Tool {
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
				toolLogger.Warnw("missing required parameter: path")
				return fridaytools.NewToolResultError("missing required parameter: path"), nil
			}

			toolLogger.Infow("file_parse started", "path", path)

			absPath, err := fileAccess.GetAbsPath(path)
			if err != nil {
				toolLogger.Warnw("invalid file path", "path", path, "error", err)
				return fridaytools.NewToolResultError(fmt.Sprintf("invalid file path: %s", err.Error())), nil
			}

			parser := newParser(absPath)
			if parser == nil {
				toolLogger.Warnw("unsupported file format", "path", path, "ext", filepath.Ext(path))
				return fridaytools.NewToolResultError(fmt.Sprintf("unsupported file format: %s", filepath.Ext(path))), nil
			}

			doc, err := parser.Load(logger.IntoContext(ctx, toolLogger))
			if err != nil {
				toolLogger.Warnw("file_parse failed", "path", path, "error", err)
				return fridaytools.NewToolResultError(err.Error()), nil
			}

			toolLogger.Infow("file_parse completed", "path", path, "content_len", len(doc.Content))
			return fridaytools.NewToolResultText(doc.Content), nil
		}),
	)
}
