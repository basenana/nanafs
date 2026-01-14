# Agentic Plugins

Three AI agent plugins powered by Friday core: React, Research, and Summary.

## Type
ProcessPlugin

## Version
1.0.0

## Plugins

### 1. react
ReAct (Reasoning + Action) agent with file access tools.

**Name:** `react`

### 2. research
Multi-step research agent that plans, researches, and summarizes.

**Name:** `research`

### 3. summary
Summarization agent for condensing content.

**Name:** `summary`

## Required Config

All three plugins require the following config in `PluginCall.Config`:

| Config Key | Required | Description |
|------------|----------|-------------|
| `llm_host` | Yes | LLM API endpoint (e.g., `https://api.openai.com/v1`) |
| `llm_api_key` | Yes | LLM API key |
| `llm_model` | Yes | Model name (e.g., `gpt-4o`, `gpt-4o-mini`) |

## Parameters

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `message` | Yes | string | User message to process |
| `system_prompt` | No | string | Custom system prompt |

## Output

```json
{
  "result": "<agent response content>"
}
```

## React Plugin - File Access Tools

The React plugin includes built-in file access tools:

| Tool | Description |
|------|-------------|
| `file_read` | Read file contents from working directory |
| `file_write` | Write content to a file |
| `file_list` | List files in a directory |
| `file_parse` | Parse document (PDF, HTML, Markdown, etc.) and extract text |

### Tool Parameters

#### file_read
| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `path` | Yes | string | Relative path to file |

#### file_write
| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `path` | Yes | string | Relative path to file |
| `content` | Yes | string | Content to write |

#### file_list
| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `path` | No | string | Directory path (default: `.`) |

**Returns:** JSON array of file info with fields: `name`, `size`, `modified`, `is_dir`

```json
[
  {"name": "readme.md", "size": 1024, "modified": "2024-01-15 10:30:00", "is_dir": false},
  {"name": "docs/", "size": 0, "modified": "2024-01-14 09:00:00", "is_dir": true}
]
```

#### file_parse
| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `path` | Yes | string | Relative path to document file |

**Supported formats:** PDF, HTML, Markdown, TXT, EPUB, WebArchive

## Usage Example

```yaml
# React Agent with file access
- name: react
  config:
    llm_host: "https://api.openai.com/v1"
    llm_api_key: "your-api-key"
    llm_model: "gpt-4o"
  parameters:
    message: "Read the README.md file and summarize its contents"
    system_prompt: "You are a helpful assistant with file access"

# Research Agent
- name: research
  config:
    llm_host: "https://api.openai.com/v1"
    llm_api_key: "your-api-key"
  parameters:
    message: "Research the latest developments in quantum computing"

# Summary Agent
- name: summary
  config:
    llm_host: "https://api.openai.com/v1"
    llm_api_key: "your-api-key"
  parameters:
    message: "Summarize the following article: ..."
```

## Notes

- React plugin has file access restricted to the working directory
- All plugins use blocking mode (wait for complete response)
- Custom system prompt is optional, defaults to Friday agent defaults
- Research agent performs: Planning -> Research -> Summary workflow
