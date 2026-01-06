# FileOpPlugin

Performs file operations (copy, move, rename, remove).

## Type
ProcessPlugin

## Version
1.0

## Name
`fileop`

## Parameters

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `action` | Yes | string | Operation type: `cp`, `mv`, `rm`, `rename` |
| `src` | Yes | string | Source file path |
| `dest` | Yes* | string | Destination path (required for `cp`, `mv`, `rename`) |

*Required for `cp`, `mv`, and `rename` actions. Not required for `rm`.

## Output

```json
{
  "success": true
}
```

On failure, returns an error message.

## Usage Example

```yaml
# Copy a file
- name: fileop
  parameters:
    action: "cp"
    src: "/path/to/source.txt"
    dest: "/path/to/destination.txt"

# Move (rename) a file
- name: fileop
  parameters:
    action: "mv"
    src: "/path/to/oldname.txt"
    dest: "/path/to/newname.txt"

# Rename a file
- name: fileop
  parameters:
    action: "rename"
    src: "/path/to/file.txt"
    dest: "/path/to/newfile.txt"

# Remove a file
- name: fileop
  parameters:
    action: "rm"
    src: "/path/to/file.txt"
```

## Notes
- `mv` and `rename` are functionally identical (both use `os.Rename`)
- The `cp` action preserves the source file's permissions
- Use `mv` or `rename` for atomic file renaming
