# WebpackPlugin

Archives web pages from URLs into local files (webarchive or HTML format).

## Type
ProcessPlugin

## Version
1.0

## Name
`webpack`

## Parameters

| Parameter | Required | Source | Description |
|-----------|----------|--------|-------------|
| `file_name` | Yes | Request | Output filename (without extension) |
| `url` | Yes | Request | URL of the webpage to archive |
| `file_type` | No | PluginCall | Output format: `html`, `webarchive` (default: `webarchive`) |
| `clutter_free` | No | PluginCall | Remove clutter from HTML (default: `true`) |

**Note**: `file_type` and `clutter_free` are read at plugin initialization time from PluginCall.Params. `file_name` and `url` are read at runtime from Request.

## Output

```json
{
  "file_path": "<filename>.<format>",
  "size": <file-size-in-bytes>,
  "title": "<page-title>",
  "url": "<original-url>"
}
```

### Output Fields

| Field | Type | Description |
|-------|------|-------------|
| `file_path` | string | Filename of the archived page |
| `size` | int64 | File size in bytes |
| `title` | string | Page title (derived from filename) |
| `url` | string | Original URL |

## File Type Formats

| Format | Description |
|--------|-------------|
| `webarchive` | macOS Web Archive format |
| `html` | Readable HTML file with clutter removed |

## Usage Example

```yaml
# Archive webpage as webarchive
- name: webpack
  parameters:
    file_name: "example-page"
    url: "https://example.com/article"
  working_path: "/path/to/output"

# Archive as HTML
- name: webpack
  parameters:
    file_name: "example-page"
    url: "https://example.com/article"
    file_type: "html"

# Disable clutter removal
- name: webpack
  parameters:
    file_name: "example-page"
    url: "https://example.com/article"
    clutter_free: "false"
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `WebPackerEnablePrivateNet` | Set to `true` to enable access to private network resources |

## Notes
- Timeout is fixed at 60 seconds
- Uses [webpage-packer](https://github.com/hyponet/webpage-packer) for archiving
- Title is derived from the filename (extension stripped)
