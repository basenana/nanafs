# ArchivePlugin

Extracts and creates archive files (zip, tar, gzip).

## Type
ProcessPlugin

## Version
1.0

## Name
`archive`

## Parameters

| Parameter | Required | Type | Default | Description |
|-----------|----------|------|---------|-------------|
| `action` | No | string | `extract` | Action: `extract` or `compress` |
| `file_path` | Extract | string | - | Path to the archive file (for extraction) |
| `source_path` | Compress | string | - | Path to file/directory to compress |
| `format` | Yes | string | - | Archive format: `zip`, `tar`, or `gzip` |
| `dest_path` | No | string | `.` | Destination directory |
| `archive_name` | No | string | auto-generated | Output archive filename |

## Output

### Extract
```json
{
  "success": true
}
```

### Compress
```json
{
  "success": true,
  "file_path": "/path/to/archive.zip",
  "size": 1024
}
```

On failure, returns an error message.

## Usage Example

### Extract

```yaml
# Extract a zip file
- name: archive
  parameters:
    file_path: "/path/to/archive.zip"
    format: "zip"
    dest_path: "/path/to/output"

# Extract a tar.gz file
- name: archive
  parameters:
    file_path: "/path/to/archive.tar.gz"
    format: "tar"
    dest_path: "/path/to/output"

# Extract a gzip file
- name: archive
  parameters:
    file_path: "/path/to/file.gz"
    format: "gzip"
```

### Compress

```yaml
# Compress a single file to zip
- name: archive
  parameters:
    action: "compress"
    source_path: "/path/to/file.txt"
    format: "zip"
    dest_path: "/path/to/output"
    archive_name: "archive.zip"

# Compress a directory to tar.gz
- name: archive
  parameters:
    action: "compress"
    source_path: "/path/to/mydir"
    format: "tar"
    dest_path: "/path/to/output"

# Compress a single file to gzip
- name: archive
  parameters:
    action: "compress"
    source_path: "/path/to/file.txt"
    format: "gzip"
    dest_path: "/path/to/output"
```

## Notes

- For `extract` action: `file_path` is required
- For `compress` action: `source_path` is required
- `tar` format uses gzip compression (`.tar.gz` or `.tgz`)
- `gzip` format only supports single files, not directories
- When `archive_name` is not provided, it's auto-generated based on source name and format
