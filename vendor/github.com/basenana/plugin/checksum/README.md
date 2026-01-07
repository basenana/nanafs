# ChecksumPlugin

Computes file checksums (MD5 or SHA256).

## Type
ProcessPlugin

## Version
1.0

## Name
`checksum`

## Parameters

| Parameter | Required | Source | Description |
|-----------|----------|--------|-------------|
| `file_path` | Yes | Request | Path to file to hash |
| `algorithm` | No | PluginCall | Hash algorithm: `md5` or `sha256` (default: `md5`) |

**Note**: `algorithm` is read at plugin initialization time from PluginCall.Params. If not specified, defaults to `md5`.

## Output

```json
{
  "hash": "<hex-encoded-hash>"
}
```

## Usage Example

```yaml
# Compute MD5 checksum (default)
- name: checksum
  parameters:
    file_path: "/path/to/file.txt"

# Compute SHA256 checksum (algorithm via PluginCall params)
- name: checksum
  parameters:
    file_path: "/path/to/file.txt"
  with:
    algorithm: "sha256"
```

## Output Example

```json
{
  "hash": "d41d8cd98f00b204e9800998ecf8427e"
}
```

## Notes
- Returns a 32-character hex string for MD5
- Returns a 64-character hex string for SHA256
