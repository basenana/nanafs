# fs

File system plugins for NanaFS operations.

## Plugins

### save (Process)

Saves a local file to the NanaFS file system with metadata.

| Parameter           | Required | Default  | Description                                       |
|---------------------|----------|----------|---------------------------------------------------|
| `file_path`         | Yes      | -        | Path to the local file                            |
| `name`              | No       | filename | Entry name in NanaFS                              |
| `parent_uri`        | Yes      | -        | Parent entry URI                                  |
| `subgroup`          | No       | -        | Sub group name (creates nested group if provided) |
| `subgroup_overview` | No       | -        | Sub group overview document file path             |
| `properties`        | No       | -        | Properties map (flat structure)                   |
| `document`          | No       | -        | Document struct from docloader                    |

**Properties fields**:

- `title` - Entry title
- `author` - Author name
- `year` - Publication year
- `source` - Source/publisher
- `abstract` - Abstract/summary
- `notes` - Personal notes (not populated by docloader)
- `keywords` - Keywords (comma-separated)
- `url` - Source URL
- `site_name` - Site name (for web content)
- `site_url` - Site URL (for web content)
- `header_image` - Header image URL
- `unread` - Mark as unread (default: false)
- `marked` - Mark as starred (default: false)
- `publish_at` - Publish timestamp (Unix)

**Properties structure** (flat, not nested):

```json
{
  "file_path": "/path/to/document.pdf",
  "name": "My Document",
  "parent_uri": "123",
  "properties": {
    "title": "Document Title",
    "author": "Author Name",
    "marked": true
  }
}
```

**Or use document from docloader**:

```json
{
  "file_path": "/path/to/document.pdf",
  "document": {
    "content": "...",
    "properties": {
      "title": "Document Title",
      "author": "Author Name"
    }
  }
}
```

### update (Process)

Updates an existing entry in NanaFS.

| Parameter    | Required | Default | Description                     |
|--------------|----------|---------|---------------------------------|
| `entry_uri`  | Yes      | -       | Entry URI (numeric ID)          |
| `properties` | No       | -       | Properties map (flat structure) |
| `document`   | No       | -       | Document struct from docloader  |

**Properties structure** (flat, not nested):

```json
{
  "entry_uri": "123",
  "properties": {
    "title": "Updated Title",
    "marked": true
  }
}
```

**Or use document from docloader**:

```json
{
  "entry_uri": "123",
  "document": {
    "properties": {
      "title": "Updated Title"
    }
  }
}
```
