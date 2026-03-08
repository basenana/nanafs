package friday

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/basenana/friday/core/tools"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
	"github.com/hyponet/webpage-packer/packer"
)

func (f *Friday) publishToolEvent(sessionID, eventMessage, entryURI string) {
	event := &Event{
		Id:       uuid.New().String(),
		Event:    eventMessage,
		EntryURI: entryURI,
		Time:     time.Now(),
	}
	events.PublishFridayEvent(f.namespace, sessionID, event)
}

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

			var content string
			switch path.Ext(path.Base(pathVal)) {
			case ".webarchive":
				p := packer.NewWebArchivePacker()
				content, err = p.ReadContent(ctx, packer.Option{
					Reader:      file,
					ClutterFree: true,
				})
				if err != nil {
					return nil, fmt.Errorf("read webarchive failed: %w", err)
				}
			case ".html", ".htm", ".hts":
				p := packer.NewHtmlPacker()
				content, err = p.ReadContent(ctx, packer.Option{
					Reader:      file,
					ClutterFree: true,
				})
				if err != nil {
					return nil, fmt.Errorf("read html failed: %w", err)
				}
			}

			f.publishToolEvent(request.SessionID, "Reading file", pathVal)

			if content != "" {
				markdown, err := htmltomarkdown.ConvertString(content)
				if err != nil {
					return tools.NewToolResultText(content), nil
				}
				return tools.NewToolResultText(markdown), nil
			}

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

			f.publishToolEvent(request.SessionID, "Writing file", pathVal)

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
			tools.Required(),
			tools.Description("Directory path, default is root"),
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

			if !entry.IsGroup {
				return tools.NewToolResultError("path is not a directory"), nil
			}

			f.publishToolEvent(request.SessionID, "Listing group", pathVal)

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

			f.publishToolEvent(request.SessionID, "Creating group", entryPath)

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
		"rename_file",
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

			f.publishToolEvent(request.SessionID, "Moving file", srcEntryPath)

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
		"delete_file",
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

			f.publishToolEvent(request.SessionID, "Deleting file", entryPath)

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

const (
	searchToolDesc = `Full-text search tool that finds files by keyword queries across file content.
Returns matching files with highlighted snippets showing where keywords appear.

Query Syntax:
- Space-separated keywords: "apple banana" (finds files containing both keywords - AND semantics)
- Phrase search: "exact phrase" (finds files containing the exact phrase)

Examples:
- Query: "error exception" - Find files containing both "error" and "exception"
- Query: "api authentication" - Find files mentioning both api and authentication
- Query: "golang tutorial" - Find files about golang tutorials

Tips:
- Use more specific keywords for accurate results
- Search for technical terms or function names for code files
- Combine related terms: "database mysql" finds files mentioning both`
)

// search tool - Search files by content using multi-keyword queries
func (f *Friday) newSearchTool() *tools.Tool {
	return tools.NewTool(
		"full_text_search",
		tools.WithDescription(searchToolDesc),
		tools.WithString("query",
			tools.Required(),
			tools.Description("Search keywords (space-separated, AND semantics). Example: 'error handling'"),
		),
		tools.WithToolHandler(func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
			query, ok := request.Arguments["query"].(string)
			if !ok || query == "" {
				return tools.NewToolResultError("missing required parameter: query"), nil
			}

			f.publishToolEvent(request.SessionID, "Searching: "+query, "")

			docs, err := f.indexer.QueryLanguage(ctx, f.namespace, query)
			if err != nil {
				return tools.NewToolResultError(err.Error()), nil
			}

			var results []any
			for _, doc := range docs {
				results = append(results, map[string]string{
					"title":     doc.HighlightTitle,
					"highlight": doc.HighlightContent,
					"path":      doc.URI,
				})
			}

			data, _ := json.Marshal(results)
			return tools.NewToolResultText(string(data)), nil
		}),
	)
}

const (
	filterToolDesc = `Advanced file filtering tool using CEL (Common Expression Language) for complex queries.
Returns matching file entries with pagination support.

## Available Fields

### Entry Metadata
- id (int): Unique entry identifier
- kind (string): Entry type (e.g., "file", "group", "smart_group")
- is_group (bool): Whether entry is a directory/group
- size (int): File size in bytes
- name (string): Entry name
- aliases (string): Entry aliases

### Timestamps (int, Unix timestamp)
- created_at: Creation time
- changed_at: Last change time
- modified_at: Last modification time
- access_at: Last access time

### Document Properties
- title (string): Document title
- abstract (string): Document summary
- notes (string): Document notes
- keyword (string): Single keyword
- keywords (list): Multiple keywords
- unread (bool): Whether document is unread
- marked (bool): Whether document is marked/favorited

### Tags & Properties
- tag (string): Single tag
- tags (list): Multiple tags
- url (string): Associated URL
- site (string): Website source

### Group Properties
- group.source (string): Group source type (e.g., "rss")

## Operators

### Comparison
- == : Equal
- != : Not equal
- < : Less than
- > : Greater than
- <= : Less than or equal
- >= : Greater than or equal

### String Operations
- string.contains(substring): Check if string contains substring
- string.startsWith(prefix): Check if string starts with prefix
- string.endsWith(suffix): Check if string ends with suffix

### List Operations
- in : Check if element is in list (e.g., "tag1 in tags")
- list.contains(element): Check if list contains element

### Logical
- && : AND
- || : OR
- ! : NOT

### Special
- now(): Current Unix timestamp (for time-based queries)

## Examples

- Filter unread documents: unread
- Filter marked documents: marked
- Filter all files: kind == "file"
- Filter all directories: is_group == true
- Filter by name pattern: name.startsWith("report")
- Filter by tag: "important" in tags
- Filter by size: size > 1000000
- Filter by URL containing: url.contains("github.com")
- Filter by modified time: modified_at > now() - 86400 * 7
- Combine conditions: kind == "file" && size > 1000000

Tips:
- Use single quotes for string values in CEL expressions
- For time comparisons, convert to Unix timestamp: now() returns current timestamp
- For 7 days ago: now() - 86400 * 7
- For 1 hour ago: now() - 3600
- Use parentheses to group complex conditions`
)

// filter tool - Filter entries using CEL pattern
func (f *Friday) newFilterTool() *tools.Tool {
	return tools.NewTool(
		"filter_entries",
		tools.WithDescription(filterToolDesc),
		tools.WithString("cel_pattern",
			tools.Required(),
			tools.Description("CEL filter pattern. Example: kind == 'file'"),
		),
		tools.WithNumber("page",
			tools.DefaultNumber(1),
			tools.Description("Page number (starts from 1)"),
		),
		tools.WithNumber("page_size",
			tools.DefaultNumber(20),
			tools.Description("Number of results per page (max 100)"),
		),
		tools.WithToolHandler(func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
			celPattern, ok := request.Arguments["cel_pattern"].(string)
			if !ok || celPattern == "" {
				return tools.NewToolResultError("missing required parameter: cel_pattern"), nil
			}

			page := int64(1)
			if pageVal, ok := request.Arguments["page"].(float64); ok {
				page = int64(pageVal)
			}

			pageSize := int64(20)
			if psVal, ok := request.Arguments["page_size"].(float64); ok {
				pageSize = int64(psVal)
			}
			if pageSize > 100 {
				pageSize = 100
			}

			f.publishToolEvent(request.SessionID, "Filtering entries: "+celPattern, "")

			pg := types.NewPagination(page, pageSize)
			pctx := types.WithPagination(ctx, pg)

			it, err := f.store.FilterEntries(pctx, f.namespace, types.Filter{CELPattern: celPattern})
			if err != nil {
				return tools.NewToolResultError(err.Error()), nil
			}

			var results []filterResult
			for it.HasNext() {
				entry, err := it.Next()
				if err != nil {
					continue
				}

				uri, err := core.ProbableEntryPath(ctx, f.core, entry)
				if err != nil {
					continue
				}

				var docProps *types.DocumentProperties
				if !entry.IsGroup {
					doc := &types.DocumentProperties{}
					if err := f.store.GetEntryProperties(ctx, f.namespace, types.PropertyTypeDocument, entry.ID, doc); err == nil {
						docProps = doc
					}
				}

				results = append(results, filterResult{
					URI:        uri,
					Name:       path.Base(uri),
					Kind:       string(entry.Kind),
					IsGroup:    entry.IsGroup,
					Size:       formatSize(entry.Size),
					ModifiedAt: entry.ModifiedAt.Format("2006-01-02 15:04:05"),
					Title:      docProps.Title,
					Abstract:   docProps.Abstract,
					URL:        docProps.URL,
					SiteName:   docProps.SiteName,
					Unread:     docProps.Unread,
					Marked:     docProps.Marked,
					Keywords:   docProps.Keywords,
				})
			}

			data, _ := json.Marshal(results)
			return tools.NewToolResultText(string(data)), nil
		}),
	)
}

type filterResult struct {
	URI        string   `json:"uri"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	IsGroup    bool     `json:"is_group"`
	Size       string   `json:"size"`
	ModifiedAt string   `json:"modified_at"`
	Title      string   `json:"title,omitempty"`
	Abstract   string   `json:"abstract,omitempty"`
	URL        string   `json:"url,omitempty"`
	SiteName   string   `json:"site_name,omitempty"`
	Unread     bool     `json:"unread"`
	Marked     bool     `json:"marked"`
	Keywords   []string `json:"keywords,omitempty"`
}
