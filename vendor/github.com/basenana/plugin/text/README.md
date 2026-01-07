# TextPlugin

Performs text processing operations (search, replace, regex, split, join).

## Type
ProcessPlugin

## Version
1.0

## Name
`text`

## Parameters

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `action` | Yes | string | Operation type: `search`, `replace`, `regex`, `split`, `join` |
| `content` | Yes* | string | Text content (not required for `join`) |
| `result_key` | No | string | Key name for result (default: `result`) |

*Required for `search`, `replace`, `regex`, and `split` actions. Not required for `join`.

### Action-specific Parameters

#### search
| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `pattern` | Yes | string | String to search for |

Returns a boolean indicating if the pattern was found.

#### replace
| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `pattern` | Yes | string | String to find |
| `replacement` | Yes | string | String to replace with |
| `count` | No | integer | Number of replacements (-1 for all, default: -1) |

Returns the modified content.

#### regex
| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `pattern` | Yes | string | Regex pattern to match |

Returns the first match found in the content.

#### split
| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `delimiter` | Yes* | string | Split delimiter |
| `pattern` | Yes* | string | Regex pattern to split on |

*One of `delimiter` or `pattern` is required.

Returns an array of strings (empty strings are trimmed).

#### join
| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `delimiter` | Yes | string | Delimiter to join with |
| `items` | Yes | string | Comma-separated items to join |

## Output

```json
{
  "<result_key>": <result>
}
```

## Usage Example

```yaml
# Search for a pattern
- name: text
  parameters:
    action: "search"
    content: "Hello, World!"
    pattern: "World"

# Replace text
- name: text
  parameters:
    action: "replace"
    content: "Hello, World!"
    pattern: "World"
    replacement: "Go"
    count: 1

# Regex match
- name: text
  parameters:
    action: "regex"
    content: "Email: test@example.com"
    pattern: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"

# Split text
- name: text
  parameters:
    action: "split"
    content: "apple, banana, orange"
    delimiter: ", "

# Join items
- name: text
  parameters:
    action: "join"
    delimiter: "-"
    items: "apple, banana, orange"
```

## Notes
- `search` returns a boolean (true/false)
- `replace` with `count=1` replaces only the first occurrence
- `split` trims whitespace from each resulting item
- `join` expects items as a comma-separated string
