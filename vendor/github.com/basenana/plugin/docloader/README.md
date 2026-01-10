# DocLoader

Loads and parses document files (PDF, TXT, MD, HTML, EPUB, webarchive).

## Type
ProcessPlugin

## Version
1.0

## Name
`docloader`

## Parameters

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `file_path` | Yes | string | Path to document file |
| `title` | No | string | Override document title |
| `url` | No | string | Document source URL |
| `site_name` | No | string | Site name (for web content) |
| `site_url` | No | string | Site URL (for web content) |

## Supported Formats

| Extension | Format |
|-----------|--------|
| `.pdf` | PDF Document |
| `.txt` | Plain Text |
| `.md`, `.markdown` | Markdown |
| `.html`, `.htm` | HTML |
| `.webarchive` | Web Archive |
| `.epub` | EPUB |

## Output

Returns a map with `file_path` and `document` object containing:

```json
{
  "file_path": "document.pdf",
  "document": {
    "content": "<extracted-text>",
    "properties": {
      "title": "<document-title>",
      "author": "<author>",
      "year": "2024",
      "source": "<source>",
      "abstract": "<summary>",
      "keywords": ["tag1", "tag2"],
      "url": "<source-url>",
      "site_name": "<site-name>",
      "site_url": "<site-url>",
      "header_image": "<url>",
      "publish_at": 1704067200,
      "unread": false,
      "marked": false
    }
  }
}
```

### Document Properties

| Field | Type | Description |
|-------|------|-------------|
| `content` | string | Document text content |
| `properties.title` | string | Document title |
| `properties.author` | string | Author name |
| `properties.year` | string | Publication year (from filename or metadata) |
| `properties.source` | string | Source/publisher |
| `properties.abstract` | string | Document abstract/summary |
| `properties.notes` | string | Personal notes |
| `properties.keywords` | []string | Keywords (array of strings) |
| `properties.url` | string | Source URL |
| `properties.site_name` | string | Site name (for web content) |
| `properties.site_url` | string | Site URL (for web content) |
| `properties.header_image` | string | Header image URL (HTML only) |
| `properties.unread` | bool | Marked as unread |
| `properties.marked` | bool | Marked as starred |
| `properties.publish_at` | int64 | Publish timestamp (Unix) |

## Architecture

```
docloader.go
├── DocLoader (main plugin)
├── Parser interface (Load returns types.Document)
│
├── filename.go
│   └── extractFileNameMetadata() // Parse filename patterns for author/title/year
│
├── pdf.go
│   └── PDF parser (extracts PDF metadata, supports password)
│
├── html.go
│   ├── HTML parser
│   └── extractHTMLMetadata() // Meta tags, OG tags, Dublin Core
│
├── epub.go
│   └── EPUB parser (extracts Dublin Core from OPF)
│
└── plaintext.go
    ├── Text parser (TXT/MD/Markdown)
    └── extractTextContentMetadata() // Title from # heading, abstract from paragraphs
```

## Metadata Extraction by Format

### PDF
- Extracts info dict metadata (author, title, subject, creator, producer)
- Supports password-protected PDFs
- Falls back to file modification time for `publish_at`

### Text (TXT, MD, Markdown)
- Extracts title from first `#` heading or first non-empty line
- Extracts first paragraph as abstract
- Parses filename patterns for author/title/year:
  - `Author_Title_2024.txt`
  - `Author - Title (2024).md`

### HTML
- Extracts meta tags: `author`, `description`, `keywords`
- Extracts Open Graph tags: `og:title`, `og:description`, `og:image`, `og:site_name`
- Extracts Dublin Core tags: `dc.title`, `dc.creator`, `dc.description`, etc.
- Falls back to HTML `<title>` tag

### EPUB
- Extracts Dublin Core metadata from OPF container
- Supports: title, creator, description, subject, publisher, date

## Usage Example

```yaml
# Load a PDF document
- name: docloader
  parameters:
    file_path: "/path/to/document.pdf"

# Load an HTML file
- name: docloader
  parameters:
    file_path: "/path/to/page.html"

# Load a markdown file
- name: docloader
  parameters:
    file_path: "/path/to/readme.md"
```

## Notes

- The `file_path` is relative to the working path
- If no title is found, filename (without extension) is used
- Fields not found in document will be empty/default values
- `header_image` only available for HTML with OG meta tags
- `year` is extracted from filename patterns or document metadata
- `keywords` is returned as an array, not comma-separated string
- `publish_at` is Unix timestamp (int64), not string
