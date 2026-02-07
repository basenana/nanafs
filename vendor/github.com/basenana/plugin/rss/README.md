# RssSourcePlugin

Fetches RSS/Atom feeds and archives articles in specified format (url, html, rawhtml, webarchive).

## Type

SourcePlugin

## Version

1.0

## Name

`rss`

## Parameters

| Parameter      | Required | Source     | Description                                                                   |
|----------------|----------|------------|-------------------------------------------------------------------------------|
| `feed`         | Yes      | Request    | RSS feed URL                                                                  |
| `file_type`    | No       | PluginCall | Output format: `url`, `html`, `rawhtml`, `webarchive` (default: `webarchive`) |
| `timeout`      | No       | PluginCall | Download timeout in seconds (default: 120)                                    |
| `clutter_free` | No       | PluginCall | Remove clutter from HTML (default: `true`)                                    |
| `parent_uri`   | Yes      | Request    | Parent entry URI for NanaFS deduplication                                     |
| `header_*`     | No       | PluginCall | Custom HTTP headers (prefix with `header_`)                                   |

**Note**: `file_type`, `timeout`, `clutter_free`, and `header_*` are read at plugin initialization time from
PluginCall.Params. `feed` and `parent_uri` are read at runtime from Request. `FS` (NanaFS interface) must also be
provided via Request when using `parent_uri`.

## Output

```json
{
  "articles": [
    {
      "file_path": "<filename>",
      "size": <file-size-in-bytes>,
      "title": "<article-title>",
      "url": "<article-url>",
      "site_url": "<site-url>",
      "site_name": "<site-name>",
      "updated_at": "<RFC3339-timestamp>"
    },
    ...
  ]
}
```

### Article Structure

| Field        | Type   | Description                               |
|--------------|--------|-------------------------------------------|
| `file_path`  | string | Filename of the archived article          |
| `size`       | int64  | File size in bytes                        |
| `title`      | string | Article title                             |
| `url`        | string | Original article URL                      |
| `site_url`   | string | Site URL of the feed                      |
| `site_name`  | string | Site name of the feed                     |
| `updated_at` | string | Publication/update time in RFC3339 format |

## File Type Formats

| Format       | Description                      |
|--------------|----------------------------------|
| `url`        | Internet Shortcut file (.url)    |
| `html`       | Readable HTML file               |
| `rawhtml`    | Full HTML with clutter removal   |
| `webarchive` | Web Archive format (.webarchive) |

## Usage Example

Parameters are passed from two sources:

- `feed` is passed via Request parameters (runtime)
- Other parameters (`file_type`, `timeout`, `header_*`) are passed via PluginCall parameters (initialization)

```yaml
# Fetch RSS feed with default settings
- name: rss
  parameters:
    feed: "https://example.com/feed.xml"
  working_path: "/path/to/output"

# Fetch with custom timeout (via PluginCall params)
- name: rss
  parameters:
    feed: "https://example.com/feed.xml"
  with:
    timeout: 60
    file_type: "html"

# Fetch with custom headers (via PluginCall params)
- name: rss
  parameters:
    feed: "https://example.com/feed.xml"
  with:
    header_User-Agent: "MyBot/1.0"
```

**Note**: The `with` section (or equivalent) passes initialization parameters to the plugin factory function.

## Notes

- Uses NanaFS to track already-processed articles to avoid duplicates
- Maximum 50 articles processed per feed
- For RSSHub feeds, automatically uses `html` format
- Custom headers are passed to the web packer
