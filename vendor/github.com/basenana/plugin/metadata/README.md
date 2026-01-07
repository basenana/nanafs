# MetadataPlugin

Retrieves file metadata (size, modification time, permissions, is_directory).

## Type
ProcessPlugin

## Version
1.0

## Name
`metadata`

## Parameters

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `file_path` | Yes | string | Path to file or directory |

## Output

```json
{
  "size": <file_size_bytes>,
  "modified": "<RFC3339-timestamp>",
  "mode": "<permissions-string>",
  "is_dir": <boolean>
}
```

## Usage Example

```yaml
# Get file metadata
- name: metadata
  parameters:
    file_path: "/path/to/file.txt"

# Get directory metadata
- name: metadata
  parameters:
    file_path: "/path/to/directory"
```

## Output Example

```json
{
  "size": 1024,
  "modified": "2024-01-15T10:30:00Z",
  "mode": "-rw-r--r--",
  "is_dir": false
}
```

## Notes
- `size` is in bytes
- `modified` is in RFC3339 format (e.g., "2024-01-15T10:30:00Z")
- `mode` follows Unix-style permission notation (e.g., -rw-r--r--, drwxr-xr-x)
- `is_dir` is true if the path is a directory
