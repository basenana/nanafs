# Agentic Plugins

Three AI agent plugins powered by Friday core: React, and Summary.

## Type

ProcessPlugin

## Version

1.0.0

## Plugins

### 1. react

ReAct (Reasoning + Action) agent with file access tools.

**Name:** `react`

### 2. summary

Summarization agent that reads content from a file and generates a summary.

**Name:** `summary`

**Supported file formats:** PDF, HTML, Markdown, TXT, EPUB, WebArchive

## Required Config

| Config Key           | Required | Description                                          |
|----------------------|----------|------------------------------------------------------|
| `friday_llm_host`    | Yes      | LLM API endpoint (e.g., `https://api.openai.com/v1`) |
| `friday_llm_api_key` | Yes      | LLM API key                                          |
| `friday_llm_model`   | Yes      | Model name (e.g., `gpt-4o`, `gpt-4o-mini`)           |

## Parameters

| Parameter       | Required | Plugin  | Type   | Description               |
|-----------------|----------|---------|--------|---------------------------|
| `message`       | Yes      | react   | string | User message to process   |
| `file_path`     | Yes      | summary | string | Path to file to summarize |
| `system_prompt` | No       | all     | string | Custom system prompt      |

## Output

### react

```json
{
  "result": "<agent response content>"
}
```

### summary

```json
{
  "file_path": "path/to/input file",
  "result": "<summary content>"
}
```

## Tools

### File Access Tools

| Tool         | Description                                                 |
|--------------|-------------------------------------------------------------|
| `file_read`  | Read file contents from working directory                   |
| `file_write` | Write content to a file                                     |
| `file_list`  | List files in a directory                                   |
| `file_parse` | Parse document (PDF, HTML, Markdown, etc.) and extract text |

#### file_read

| Parameter | Required | Type   | Description           |
|-----------|----------|--------|-----------------------|
| `path`    | Yes      | string | Relative path to file |

#### file_write

| Parameter | Required | Type   | Description           |
|-----------|----------|--------|-----------------------|
| `path`    | Yes      | string | Relative path to file |
| `content` | Yes      | string | Content to write      |

#### file_list

| Parameter | Required | Type   | Description                   |
|-----------|----------|--------|-------------------------------|
| `path`    | No       | string | Directory path (default: `.`) |

**Returns:** JSON array of file info with fields: `name`, `size`, `modified`, `is_dir`

```json
[
  {
    "name": "readme.md",
    "size": 1024,
    "modified": "2024-01-15 10:30:00",
    "is_dir": false
  },
  {
    "name": "docs/",
    "size": 0,
    "modified": "2024-01-14 09:00:00",
    "is_dir": true
  }
]
```

#### file_parse

| Parameter | Required | Type   | Description                    |
|-----------|----------|--------|--------------------------------|
| `path`    | Yes      | string | Relative path to document file |

**Supported formats:** PDF, HTML, Markdown, TXT, EPUB, WebArchive

#### web_search

| Parameter    | Required | Type   | Description                                           |
|--------------|----------|--------|-------------------------------------------------------|
| `query`      | Yes      | string | Search query                                          |
| `time_range` | Yes      | string | Time range: `day`, `week`, `month`, `year`, `anytime` |

**Returns:** JSON array of search results with fields: `title`, `content`, `site`, `url`

```json
[
  {
    "title": "Result Title",
    "content": "Result snippet...",
    "site": "example.com",
    "url": "https://..."
  }
]
```

#### crawl_webpages

| Parameter  | Required | Type  | Description           |
|------------|----------|-------|-----------------------|
| `url_list` | Yes      | array | List of URLs to crawl |

**Returns:** JSON array of page content with fields: `url`, `file_path`, `error`

- Downloaded pages are saved to working directory
- `file_path` contains the relative path to saved HTML file
- `error` contains error message if crawling failed

## Usage Example

```yaml
# React Agent with file access
- name: react
  config:
    friday_llm_host: "https://api.openai.com/v1"
    friday_llm_api_key: "your-api-key"
    friday_llm_model: "gpt-4o"
  parameters:
    message: "Read the README.md file and summarize its contents"
    system_prompt: "You are a helpful assistant with file access"

# Summary Agent
- name: summary
  config:
    friday_llm_host: "https://api.openai.com/v1"
    friday_llm_api_key: "your-api-key"
    friday_llm_model: "gpt-4o-mini"
  parameters:
    file_path: "article.pdf"
```

## Notes

- File access tools are restricted to the working directory
- All plugins use blocking mode (wait for complete response)
- Custom system prompt is optional, defaults to Friday agent defaults
