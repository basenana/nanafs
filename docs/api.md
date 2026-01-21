# NanaFS REST API Reference

## Overview

NanaFS provides a RESTful API built with [Gin](https://gin-gonic.com/) framework. All API endpoints are prefixed with
`/api/v1/` unless otherwise specified.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

Currently, authentication is handled via middleware. Refer to configuration for authentication settings.

---

## Endpoints

### 1. Health Check

#### GET /_ping

Health check endpoint for load balancers and monitoring.

**Response:**

```json
{
  "status": "ok",
  "time": "2024-01-01T00:00:00Z"
}
```

---

### 2. Entries (条目管理)

Entries are the core resource in NanaFS, representing files, groups, and other entities.

All entry-related endpoints support two access patterns via request body:

- **By URI**: Use `{"uri": "/path/to/entry"}` in request body
- **By ID**: Use `{"id": 12345}` in request body

The `uri` and `id` fields are mutually exclusive - specify either one, not both.

#### POST /api/v1/entries/details

Retrieve details of a specific entry.

**Request Body**

```json
{
  "uri": "/inbox/tasks/task-001"
}
```

Or by ID:

```json
{
  "id": 1001
}
```

**Response**

```json
{
  "entry": {
    "uri": "/inbox",
    "entry": 1001,
    "name": "Inbox",
    "aliases": "",
    "kind": "group",
    "is_group": true,
    "size": 0,
    "version": 1,
    "namespace": "default",
    "storage": "local",
    "access": {
      "uid": 1000,
      "gid": 1000,
      "permissions": [
        "owner_read",
        "owner_write",
        "group_read",
        "group_write",
        "others_read"
      ]
    },
    "property": {
      "tags": [
        "important"
      ],
      "index_version": "v1",
      "summarize": "finish",
      "url": "https://example.com",
      "site_name": "Example Site",
      "properties": {
        "priority": "high"
      }
    },
    "document": {
      "title": "Document Title",
      "author": "John Doe",
      "year": "2024",
      "source": "https://example.com",
      "abstract": "...",
      "keywords": [
        "tag1",
        "tag2"
      ],
      "notes": "",
      "unread": true,
      "marked": false,
      "publish_at": "2024-01-01T00:00:00Z",
      "url": "https://example.com/article",
      "header_image": ""
    },
    "created_at": "2024-01-01T00:00:00Z",
    "changed_at": "2024-01-01T00:00:00Z",
    "modified_at": "2024-01-01T00:00:00Z",
    "access_at": "2024-01-01T00:00:00Z"
  }
}
```

#### POST /api/v1/entries

Create a new entry.

**Request Body:**

```json
{
  "uri": "/inbox/tasks/task-001",
  "kind": "raw",
  "rss": null,
  "filter": null,
  "properties": {
    "tags": [
      "important",
      "review"
    ],
    "properties": {
      "priority": "high"
    }
  },
  "document": {
    "title": "Document Title",
    "author": "John Doe",
    "year": "2024",
    "source": "https://example.com",
    "abstract": "...",
    "keywords": [
      "tag1",
      "tag2"
    ],
    "notes": "",
    "url": "https://example.com/article",
    "header_image": ""
  }
}
```

**Fields:**

| Field        | Type   | Required | Description                                                                                                                                    |
|--------------|--------|----------|------------------------------------------------------------------------------------------------------------------------------------------------|
| `uri`        | string | yes      | Entry URI path                                                                                                                                 |
| `kind`       | string | no       | Entry type: `raw`, `group`, `smart_group`, `fifo`, `socket`, `sym_link`, `blk_dev`, `char_dev`, `external_group`                               |
| `rss`        | object | no       | RSS subscription config                                                                                                                        |
| `filter`     | object | no       | Filter config for smart groups                                                                                                                 |
| `properties` | object | no       | Entry properties including tags and custom properties. Note: `index_version`, `summarize`, `url`, `site_name` are read-only and set by system. |
| `document`   | object | no       | Document-specific properties (only for non-group entries)                                                                                      |

**RSS Config:**

```json
{
  "feed": "https://example.com/feed.xml",
  "site_name": "Example Site",
  "site_url": "https://example.com",
  "file_type": "html"
}
```

**Filter Config:**

```json
{
  "cel_pattern": "entry.kind == 'raw'"
}
```

**Response:**

```json
{
  "entry": {
    "uri": "/inbox/tasks/task-001",
    "entry": 1002,
    "name": "task-001",
    "kind": "raw",
    "is_group": false,
    "size": 0,
    "created_at": "2024-01-01T00:00:00Z",
    "changed_at": "2024-01-01T00:00:00Z",
    "modified_at": "2024-01-01T00:00:00Z",
    "access_at": "2024-01-01T00:00:00Z"
  }
}
```

#### POST /api/v1/entries/filter

Filter entries using CEL (Common Expression Language) patterns.

**Request Body:**

```json
{
  "cel_pattern": "entry.kind == 'raw'",
  "page": 1,
  "page_size": 10,
  "sort": "created_at",
  "order": "desc"
}
```

**Fields:**

| Field         | Type   | Required | Description                                                      |
|---------------|--------|----------|------------------------------------------------------------------|
| `cel_pattern` | string | yes      | CEL expression to filter entries                                 |
| `page`        | int64  | no       | Page number (default: 1)                                         |
| `page_size`   | int64  | no       | Number of items per page (default: 50)                           |
| `sort`        | string | no       | Sort field: `created_at`, `changed_at`, `name` (default: `name`) |
| `order`       | string | no       | Sort order: `asc`, `desc` (default: `asc`)                       |

**Response:**

```json
{
  "entries": [
    {
      "uri": "/inbox/tasks/task-001",
      "entry": 1002,
      "name": "task-001",
      "kind": "raw",
      "is_group": false,
      "size": 0,
      "document": {
        "title": "Document Title",
        "author": "John Doe",
        "year": "2024",
        "source": "https://example.com",
        "abstract": "...",
        "keywords": [
          "tag1",
          "tag2"
        ],
        "notes": "",
        "unread": true,
        "marked": false,
        "publish_at": "2024-01-01T00:00:00Z",
        "url": "https://example.com/article",
        "header_image": ""
      },
      "created_at": "2024-01-01T00:00:00Z",
      "changed_at": "2024-01-01T00:00:00Z",
      "modified_at": "2024-01-01T00:00:00Z",
      "access_at": "2024-01-01T00:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 10
  }
}
```

#### POST /api/v1/entries/search

Search documents using full-text search.

**Request Body:**

```json
{
  "query": "golang tutorial",
  "page": 1,
  "page_size": 10
}
```

**Fields:**

| Field      | Type   | Required | Description                          |
|------------|--------|----------|--------------------------------------|
| `query`    | string | yes      | Search keywords                      |
| `page`     | int64  | no       | Page number (default: 1)             |
| `page_size`| int64  | no       | Number of items per page (default: 20)|

**Response:**

```json
{
  "documents": [
    {
      "id": 1001,
      "uri": "/inbox/docs/article-001",
      "title": "Golang Tutorial",
      "content": "This is a tutorial about Go programming language...",
      "create_at": 1704067200,
      "changed_at": 1704153600
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 10
  }
}
```

#### PUT /api/v1/entries

Update entry attributes. Supports `?uri=` or `?id=` query parameters.

**Query Parameters**

| Name  | Type   | Description    |
|-------|--------|----------------|
| `uri` | string | Entry URI path |
| `id`  | int64  | Entry ID       |

**Request Body:**

```json
{
  "aliases": "alias1,alias2"
}
```

**Response:**

```json
{
  "entry": {
    "uri": "/inbox/tasks/task-001",
    "entry": 1002,
    "name": "task-001",
    "kind": "raw",
    "is_group": false,
    "size": 0,
    "created_at": "2024-01-01T00:00:00Z",
    "changed_at": "2024-01-01T00:00:00Z",
    "modified_at": "2024-01-01T00:00:00Z",
    "access_at": "2024-01-01T00:00:00Z"
  }
}
```

#### POST /api/v1/entries/delete

Delete a single entry.

**Request Body:**

```json
{
  "uri": "/inbox/tasks/task-001"
}
```

Or by ID:

```json
{
  "id": 1002
}
```

**Response:**

```json
{
  "entry": {
    "uri": "/inbox/tasks/task-001",
    "entry": 1002
  }
}
```

#### POST /api/v1/entries/batch-delete

Delete multiple entries at once.

**Request Body:**

```json
{
  "uri_list": [
    "/inbox/tasks/task-001",
    "/inbox/tasks/task-002"
  ]
}
```

**Response:**

```json
{
  "deleted": [
    "/inbox/tasks/task-001",
    "/inbox/tasks/task-002"
  ],
  "message": "Successfully deleted 2 entries"
}
```

#### PUT /api/v1/entries/parent

Change an entry's parent group.

**Request Body:**

```json
{
  "entry_uri": "/inbox/tasks/task-001",
  "new_entry_uri": "/projects/active",
  "replace": false,
  "exchange": false
}
```

**Fields:**

| Field           | Type   | Required | Description                      |
|-----------------|--------|----------|----------------------------------|
| `entry_uri`     | string | yes      | URI of the entry to move         |
| `new_entry_uri` | string | yes      | New parent URI                   |
| `replace`       | bool   | no       | Replace existing entry if exists |
| `exchange`      | bool   | no       | Exchange positions               |

**Response:**

```json
{
  "entry": {
    "uri": "/projects/active/task-001",
    "entry": 1002,
    "name": "task-001",
    "kind": "raw",
    "is_group": false,
    "size": 0,
    "created_at": "2024-01-01T00:00:00Z",
    "changed_at": "2024-01-01T00:00:00Z",
    "modified_at": "2024-01-01T00:00:00Z",
    "access_at": "2024-01-01T00:00:00Z"
  }
}
```

#### PUT /api/v1/entries/property

Update tags and custom properties for an entry.

**Request Body:**

```json
{
  "uri": "/inbox/tasks/task-001",
  "tags": ["important", "review"],
  "properties": {
    "priority": "high",
    "due_date": "2024-01-15"
  }
}
```

Or by ID:

```json
{
  "id": 1001,
  "tags": ["important"],
  "properties": {
    "priority": "high"
  }
}
```

**Response:**

```json
{
  "properties": {
    "tags": [
      "important",
      "review"
    ],
    "index_version": "v1",
    "summarize": "finish",
    "url": "https://example.com/article",
    "site_name": "Example Site",
    "properties": {
      "priority": "high",
      "due_date": "2024-01-15"
    }
  }
}
```

**Fields:**

| Field           | Type   | Description                               |
|-----------------|--------|-------------------------------------------|
| `tags`          | array  | Entry tags                                |
| `index_version` | string | Index version (read-only, system managed) |
| `summarize`     | string | AI summary task status (read-only)        |
| `url`           | string | Associated URL (read-only)                |
| `site_name`     | string | Site name (read-only)                     |
| `properties`    | object | Custom key-value properties               |

**PUT Request Body:**

```json
{
  "tags": [
    "important",
    "review"
  ],
  "properties": {
    "priority": "high",
    "due_date": "2024-01-15"
  }
}
```

**PUT Response:**

```json
{
  "properties": {
    "tags": [
      "important",
      "review"
    ],
    "index_version": "v1",
    "summarize": "finish",
    "url": "https://example.com/article",
    "site_name": "Example Site",
    "properties": {
      "priority": "high",
      "due_date": "2024-01-15"
    }
  }
}
```

#### PUT /api/v1/entries/document

Update document-specific properties. Supports `?uri=` or `?id=` query parameters.

**Query Parameters**

| Name  | Type   | Description    |
|-------|--------|----------------|
| `uri` | string | Entry URI path |
| `id`  | int64  | Entry ID       |

**Request Body:**

```json
{
  "title": "Updated Title",
  "author": "Author Name",
  "year": "2024",
  "source": "https://example.com",
  "abstract": "Document abstract...",
  "notes": "Personal notes",
  "keywords": [
    "tag1",
    "tag2"
  ],
  "url": "https://example.com/article",
  "site_name": "Site Name",
  "site_url": "https://example.com",
  "header_image": "https://example.com/image.jpg",
  "unread": true,
  "marked": false,
  "publish_at": 1704067200
}
```

**Fields:**

| Field          | Type   | Description              |
|----------------|--------|--------------------------|
| `title`        | string | Document title           |
| `author`       | string | Author name              |
| `year`         | string | Publication year         |
| `source`       | string | Source URL               |
| `abstract`     | string | Document abstract        |
| `notes`        | string | Personal notes           |
| `keywords`     | array  | Keywords/tags            |
| `url`          | string | Article URL              |
| `site_name`    | string | Website name             |
| `site_url`     | string | Website URL              |
| `header_image` | string | Header image URL         |
| `unread`       | bool   | Mark as unread           |
| `marked`       | bool   | Mark as starred          |
| `publish_at`   | int64  | Publish timestamp (Unix) |

**Response:**

```json
{
  "properties": {
    "title": "Document Title",
    "author": "John Doe",
    "year": "2024",
    "source": "https://example.com",
    "abstract": "...",
    "keywords": [
      "tag1",
      "tag2"
    ],
    "notes": "",
    "unread": true,
    "marked": false,
    "publish_at": "2024-01-01T00:00:00Z",
    "url": "https://example.com/article",
    "header_image": ""
  }
}
```

#### POST /api/v1/entries/friday

Get Friday processing summary for an entry.

**Request Body:**

```json
{
  "uri": "/inbox/tasks/task-001"
}
```

Or by ID:

```json
{
  "id": 1001
}
```

**Response:**

```json
{
  "property": {
    "summary": "Friday AI processing summary..."
  }
}
```

**Fields:**

| Field     | Type   | Description                           |
|-----------|--------|---------------------------------------|
| `summary` | string | Friday processing summary (read-only) |

---

### 3. Groups (组管理)

#### POST /api/v1/groups/children

List all children of a group.

**Request Body:**

```json
{
  "uri": "/inbox",
  "page": 1,
  "page_size": 10,
  "sort": "name",
  "order": "asc"
}
```

Or by ID:

```json
{
  "id": 1001,
  "page": 1,
  "page_size": 10
}
```

**Response**

```json
{
  "entries": [
    {
      "uri": "/inbox/tasks/task-001",
      "entry": 1002,
      "name": "task-001",
      "kind": "raw",
      "is_group": false,
      "size": 0,
      "document": {
        "title": "Document Title",
        "author": "John Doe",
        "year": "2024",
        "source": "https://example.com",
        "abstract": "...",
        "keywords": [
          "tag1",
          "tag2"
        ],
        "notes": "",
        "unread": true,
        "marked": false,
        "publish_at": "2024-01-01T00:00:00Z",
        "url": "https://example.com/article",
        "header_image": ""
      },
      "created_at": "2024-01-01T00:00:00Z",
      "changed_at": "2024-01-01T00:00:00Z",
      "modified_at": "2024-01-01T00:00:00Z",
      "access_at": "2024-01-01T00:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 10
  }
}
```

#### GET /api/v1/groups/tree

Get the complete group tree structure.

**Response:**

```json
{
  "root": {
    "name": "root",
    "uri": "/",
    "children": [
      {
        "name": "inbox",
        "uri": "/inbox",
        "children": [
          {
            "name": "tasks",
            "uri": "/inbox/tasks",
            "children": []
          }
        ]
      }
    ]
  }
}
```

---

### 4. Files (文件操作)

#### POST /api/v1/files/content

Read file content.

**Request Body:**

```json
{
  "uri": "/inbox/docs/document.pdf"
}
```

Or by ID:

```json
{
  "id": 2001
}
```

**Response:** `application/octet-stream`

#### POST /api/v1/files/content/write

Upload/write file content.

**Content-Type:** `multipart/form-data`

**Form Fields:**

| Field  | Type   | Description              |
|--------|--------|--------------------------|
| `file` | file   | The file to upload       |
| `uri`  | string | File URI path (optional) |
| `id`   | int64  | File ID (optional)       |

Note: Either `uri` or `id` must be provided in the form data to specify the target file entry.

**Response:**

```json
{
  "len": 1024
}
```

---

### 5. Messages (消息/通知)

#### GET /api/v1/messages

List messages/notifications.

**Query Parameters**

| Name  | Type | Description                           |
|-------|------|---------------------------------------|
| `all` | bool | Include all messages (including read) |

**Response**

```json
{
  "messages": [
    {
      "id": "msg-001",
      "title": "Workflow Completed",
      "message": "File processing finished",
      "type": "info",
      "source": "workflow",
      "action": "view",
      "status": "unread",
      "time": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### POST /api/v1/messages/read

Mark messages as read.

**Request Body:**

```json
{
  "message_id_list": [
    "msg-001",
    "msg-002"
  ]
}
```

**Response:**

```json
{
  "success": true
}
```

---

### 6. Workflows (工作流)

#### GET /api/v1/workflows

List all workflows.

**Query Parameters**

| Name        | Type   | Description                                                            |
|-------------|--------|------------------------------------------------------------------------|
| `page`      | int    | Page number (default: 1)                                               |
| `page_size` | int    | Number of items per page (default: all)                                |
| `sort`      | string | Sort field: `name`, `created_at`, `updated_at` (default: `created_at`) |
| `order`     | string | Sort order: `asc`, `desc` (default: `desc`)                            |

**Response:**

```json
{
  "workflows": [
    {
      "id": "wf-001",
      "name": "Process RSS",
      "enable": true,
      "queue_name": "rss-processing",
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z",
      "last_triggered_at": "2024-01-02T00:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 10
  }
}
```

#### GET /api/v1/workflows/:id/jobs

List jobs for a specific workflow.

**Query Parameters**

| Name        | Type   | Description                                                    |
|-------------|--------|----------------------------------------------------------------|
| `status`    | string | Job status filter (comma-separated, e.g., `running,failed`)    |
| `page`      | int    | Page number (default: 1)                                       |
| `page_size` | int    | Number of items per page (default: all)                        |
| `sort`      | string | Sort field: `created_at`, `updated_at` (default: `created_at`) |
| `order`     | string | Sort order: `asc`, `desc` (default: `desc`)                    |

**Valid Status Values:** `initializing`, `running`, `paused`, `succeed`, `failed`, `error`, `canceled`

**Examples:**

```bash
# List all jobs
GET /api/v1/workflows/wf-001/jobs

# Filter by single status
GET /api/v1/workflows/wf-001/jobs?status=running

# Filter by multiple statuses
GET /api/v1/workflows/wf-001/jobs?status=running,failed
```

**Response:**

```json
{
  "jobs": [
    {
      "id": "job-001",
      "workflow": "wf-001",
      "trigger_reason": "manual",
      "status": "completed",
      "message": "",
      "queue_name": "rss-processing",
      "target": {
        "entries": [
          "/inbox/rss/feed-001"
        ]
      },
      "steps": [
        {
          "name": "fetch",
          "status": "completed",
          "message": ""
        },
        {
          "name": "parse",
          "status": "completed",
          "message": ""
        }
      ],
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z",
      "start_at": "2024-01-01T00:00:00Z",
      "finish_at": "2024-01-01T00:01:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 10
  }
}
```

#### POST /api/v1/workflows

Create a new workflow.

**Request Body:**

```json
{
  "name": "Process RSS Feed",
  "trigger": {
    "rss": {
      "feed": "https://example.com/feed.xml",
      "interval": 3600
    }
  },
  "nodes": [
    {
      "name": "fetch",
      "type": "http",
      "config": {
        "url": "{{.entry.uri}}"
      }
    },
    {
      "name": "parse",
      "type": "rss-parser"
    }
  ],
  "enable": true,
  "queue_name": "rss-processing"
}
```

**Fields:**

| Field        | Type   | Required | Description                             |
|--------------|--------|----------|-----------------------------------------|
| `name`       | string | yes      | Workflow name                           |
| `trigger`    | object | yes      | Trigger configuration                   |
| `nodes`      | array  | yes      | Workflow nodes/steps                    |
| `enable`     | bool   | no       | Enable/disable workflow (default: true) |
| `queue_name` | string | no       | Queue name for job processing           |

**Trigger Sub-fields:**

| Field              | Type   | Description                                    |
|--------------------|--------|------------------------------------------------|
| `local_file_watch` | object | Local file system watcher (optional)           |
| `rss`              | object | RSS feed watcher (optional, empty object `{}`) |
| `interval`         | int    | Interval in seconds (optional)                 |
| `input_parameters` | array  | Input parameters definition (optional)         |

**WorkflowInputParameter:**

| Field      | Type   | Description                   |
|------------|--------|-------------------------------|
| `name`     | string | Parameter name                |
| `describe` | string | Parameter description         |
| `required` | bool   | Whether parameter is required |

**WorkflowLocalFileWatch:**

| Field           | Type   | Description                                 |
|-----------------|--------|---------------------------------------------|
| `directory`     | string | Directory to watch                          |
| `event`         | string | Event type (create, modify, delete)         |
| `file_pattern`  | string | File name pattern (optional)                |
| `file_types`    | string | File types filter, e.g. `txt,md` (optional) |
| `min_file_size` | int    | Minimum file size in bytes (optional)       |
| `max_file_size` | int    | Maximum file size in bytes (optional)       |
| `cel_pattern`   | string | CEL filter expression (optional)            |

**Response:**

```json
{
  "workflows": [
    {
      "id": "423ae373-c4ee-435c-bd52-45e6b9e2bdef",
      "name": "Document Load",
      "queue_name": "file",
      "enable": true,
      "created_at": "2026-01-12T23:50:30.685288+08:00",
      "updated_at": "2026-01-15T22:04:21.577409+08:00",
      "last_triggered_at": "2026-01-15T22:04:21.563627+08:00"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 50
  }
}
```

#### GET /api/v1/workflows/:id

Retrieve a specific workflow.

**Response:**

```json
{
  "workflow": {
    "id": "cfb40ff3-3167-4cc3-b939-1bb6717ecbf9",
    "namespace": "Hypo",
    "name": "RSS Collect",
    "trigger": {
      "rss": {},
      "interval": 30,
      "input_parameters": [
        {
          "name": "feed_url",
          "describe": "RSS feed URL",
          "required": true
        },
        {
          "name": "output_path",
          "describe": "Output directory path",
          "required": false
        }
      ]
    },
    "nodes": [
      {
        "name": "fetch_rss",
        "type": "rss",
        "params": {
          "clutter_free": "true",
          "file_type": "webarchive",
          "timeout": "120"
        },
        "input": {
          "feed": "$.trigger.feed"
        },
        "next": "process_articles"
      },
      {
        "name": "process_articles",
        "type": "docloader",
        "params": null,
        "input": {
          "file_path": "$.matrix.file_path",
          "site_name": "$.matrix.site_name",
          "site_url": "$.matrix.site_url",
          "title": "$.matrix.title",
          "url": "$.matrix.url"
        },
        "next": "save_to_nanafs",
        "matrix": {
          "data": {
            "file_path": "$.fetch_rss.articles.*.file_path",
            "site_name": "$.fetch_rss.articles.*.site_name",
            "site_url": "$.fetch_rss.articles.*.site_url",
            "title": "$.fetch_rss.articles.*.title",
            "url": "$.fetch_rss.articles.*.url"
          }
        }
      },
      {
        "name": "save_to_nanafs",
        "type": "save",
        "params": null,
        "input": {
          "document": "$.matrix.document",
          "file_path": "$.matrix.file_path",
          "parent_uri": "$.trigger.parent_uri"
        },
        "matrix": {
          "data": {
            "document": "$.process_articles.matrix_results.*.document",
            "file_path": "$.process_articles.matrix_results.*.file_path"
          }
        }
      }
    ],
    "enable": true,
    "queue_name": "file",
    "created_at": "2026-01-12T23:50:30.659166+08:00",
    "updated_at": "2026-01-15T22:04:21.500807+08:00",
    "last_triggered_at": "2026-01-15T22:04:21.487893+08:00"
  }
}
```

**Response with LocalFileWatch trigger:**

```json
{
  "workflow": {
    "id": "wf-filewatch-001",
    "namespace": "default",
    "name": "File Processor",
    "trigger": {
      "local_file_watch": {
        "directory": "/watch/incoming",
        "event": "create",
        "file_pattern": "*.pdf",
        "file_types": "pdf",
        "min_file_size": 1024,
        "max_file_size": 10485760,
        "cel_pattern": "entry.size > 0"
      },
      "input_parameters": []
    },
    "nodes": [
      {
        "name": "process",
        "type": "http",
        "params": {
          "url": "http://processor:8080/process"
        },
        "input": {
          "file_path": "$.trigger.file_path"
        }
      }
    ],
    "enable": true,
    "queue_name": "file",
    "created_at": "2026-01-10T10:00:00.000000+08:00",
    "updated_at": "2026-01-15T15:30:00.000000+08:00",
    "last_triggered_at": "2026-01-15T14:00:00.000000+08:00"
  }
}
```

**Fields:**

| Field               | Type   | Description                   |
|---------------------|--------|-------------------------------|
| `id`                | string | Workflow ID                   |
| `namespace`         | string | Namespace                     |
| `name`              | string | Workflow name                 |
| `trigger`           | object | Trigger configuration         |
| `nodes`             | array  | Workflow nodes/steps          |
| `enable`            | bool   | Whether workflow is enabled   |
| `queue_name`        | string | Queue name for job processing |
| `created_at`        | string | Creation timestamp            |
| `updated_at`        | string | Last update timestamp         |
| `last_triggered_at` | string | Last trigger timestamp        |

**Node Fields:**

| Field       | Type   | Description                                      |
|-------------|--------|--------------------------------------------------|
| `name`      | string | Node name                                        |
| `type`      | string | Node type (http, rss-parser, docloader, save...) |
| `params`    | object | Node configuration parameters                    |
| `input`     | object | Input mapping for data passing between nodes     |
| `next`      | string | Next node name (optional)                        |
| `condition` | string | Condition for if nodes (CEL expression)          |
| `branches`  | object | Branch mapping for if nodes                      |
| `cases`     | array  | Case definitions for switch nodes                |
| `default`   | string | Default case for switch nodes                    |
| `matrix`    | object | Matrix configuration for loop nodes              |

**WorkflowNodeCase (for switch nodes):**

| Field   | Type   | Description         |
|---------|--------|---------------------|
| `value` | string | Case matching value |
| `next`  | string | Next node name      |

**WorkflowNodeMatrix (for loop nodes):**

| Field  | Type   | Description                                                     |
|--------|--------|-----------------------------------------------------------------|
| `data` | object | Variable mappings, e.g. `{"file_path": "$.fetch.result.paths"}` |

#### PUT /api/v1/workflows/:id

Update an existing workflow.

**Request Body:**

```json
{
  "name": "Updated Workflow Name",
  "enable": false,
  "queue_name": "new-queue-name"
}
```

**Note:** All fields are optional. Only provided fields will be updated.

**Response:**

```json
{
  "workflow": {
    "id": "wf-001",
    "name": "Updated Workflow Name",
    "enable": false,
    "queue_name": "new-queue-name",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-03T00:00:00Z",
    "last_triggered_at": "2024-01-02T00:00:00Z"
  }
}
```

#### DELETE /api/v1/workflows/:id

Delete a workflow.

**Response:**

```json
{
  "message": "workflow deleted"
}
```

#### GET /api/v1/workflows/:id/jobs

List all jobs for a specific workflow.

**Response:**

```json
{
  "jobs": [
    {
      "id": "f73ff9b0-fd0e-4dd5-938d-64e4ad2a7a9e",
      "workflow": "cfb40ff3-3167-4cc3-b939-1bb6717ecbf9",
      "trigger_reason": "sync rss group",
      "status": "succeed",
      "message": "finish",
      "queue_name": "file",
      "target": {
        "entries": [
          "/Hello/梦旭随想"
        ]
      },
      "steps": [
        {
          "name": "fetch_rss",
          "status": "succeed",
          "message": ""
        },
        {
          "name": "process_articles",
          "status": "succeed",
          "message": ""
        },
        {
          "name": "save_to_nanafs",
          "status": "succeed",
          "message": ""
        }
      ],
      "created_at": "2026-01-15T22:02:52.275473+08:00",
      "updated_at": "2026-01-15T22:03:09.201952+08:00",
      "start_at": "2026-01-15T22:02:52.333161+08:00",
      "finish_at": "2026-01-15T22:03:09.193124+08:00"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 50
  }
}
```

#### GET /api/v1/workflows/:id/jobs/:jobId

Get details of a specific job.

**Response:**

```json
{
  "job": {
    "id": "job-001",
    "workflow": "wf-001",
    "trigger_reason": "manual",
    "status": "completed",
    "message": "",
    "queue_name": "rss-processing",
    "target": {
      "entries": [
        "/inbox/rss/feed-001"
      ]
    },
    "steps": [
      {
        "name": "fetch",
        "status": "completed",
        "message": ""
      },
      {
        "name": "parse",
        "status": "completed",
        "message": ""
      }
    ],
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z",
    "start_at": "2024-01-01T00:00:00Z",
    "finish_at": "2024-01-01T00:01:00Z"
  }
}
```

#### POST /api/v1/workflows/:id/jobs/:jobId/pause

Pause a running job.

**Response:**

```json
{
  "message": "job paused"
}
```

#### POST /api/v1/workflows/:id/jobs/:jobId/resume

Resume a paused job.

**Response:**

```json
{
  "message": "job resumed"
}
```

#### POST /api/v1/workflows/:id/jobs/:jobId/cancel

Cancel a job.

**Response:**

```json
{
  "message": "job cancelled"
}
```

#### POST /api/v1/workflows/:id/trigger

Manually trigger a workflow.

**Request Body:**

```json
{
  "uri": "/inbox/rss/feed-001",
  "reason": "manual trigger",
  "timeout": 300,
  "parameters": {
    "feed_url": "https://example.com/feed.xml",
    "output_path": "/processed/rss"
  }
}
```

**Fields:**

| Field        | Type   | Required | Description                                                        |
|--------------|--------|----------|--------------------------------------------------------------------|
| `uri`        | string | no       | Entry URI to trigger workflow on                                   |
| `reason`     | string | no       | Reason for triggering                                              |
| `timeout`    | int64  | no       | Timeout in seconds (default: 600)                                  |
| `parameters` | object | no       | Input parameters matching workflow's `input_parameters` definition |

**About Parameters:**

The `parameters` field provides input values for workflow execution. These values are matched against the workflow's
`input_parameters` definition:

```json
// Workflow definition with input_parameters
{
  "name": "RSS Processor",
  "trigger": {
    "rss": {},
    "input_parameters": [
      {
        "name": "feed_url",
        "describe": "RSS feed URL",
        "required": true
      },
      {
        "name": "output_path",
        "describe": "Output path",
        "required": false
      }
    ]
  },
  "nodes": [
    ...
  ]
}

// Trigger with parameters
// POST /api/v1/workflows/wf-123/trigger
{
  "parameters": {
    "feed_url": "https://example.com/feed.xml",
    "output_path": "/processed/rss"
  }
}
```

In workflow nodes, access these parameters via `$.input.<parameter_name>`:

```json
{
  "name": "fetch",
  "type": "http",
  "params": {
    "url": "{{$.input.feed_url}}"
  }
}
```

**Response:**

```json
{
  "job_id": "job-002"
}
```

#### GET /api/v1/workflows/plugins

List all available workflow plugins with their parameters.

**Response:**

```json
{
  "plugins": [
    {
      "name": "webpack",
      "version": "1.0",
      "type": "process",
      "init_parameters": [
        {
          "name": "file_type",
          "required": false,
          "default": "webarchive",
          "description": "Output file type",
          "options": [
            "html",
            "webarchive"
          ]
        }
      ],
      "parameters": [
        {
          "name": "file_name",
          "required": true,
          "description": "Output file name"
        }
      ]
    },
    {
      "name": "archive",
      "version": "1.0",
      "type": "process",
      "init_parameters": [],
      "parameters": [
        {
          "name": "action",
          "required": false,
          "default": "extract",
          "description": "Action to perform: extract or compress",
          "options": [
            "extract",
            "compress"
          ]
        },
        {
          "name": "format",
          "required": true,
          "description": "Archive format",
          "options": [
            "zip",
            "tar",
            "gzip"
          ]
        }
      ]
    },
    {
      "name": "checksum",
      "version": "1.0",
      "type": "process",
      "init_parameters": [],
      "parameters": [
        {
          "name": "algorithm",
          "required": false,
          "default": "md5",
          "description": "Hash algorithm",
          "options": [
            "md5",
            "sha256"
          ]
        }
      ]
    }
  ]
}
```

**Fields:**

| Field             | Type   | Description                                       |
|-------------------|--------|---------------------------------------------------|
| `name`            | string | Plugin name                                       |
| `version`         | string | Plugin version                                    |
| `type`            | string | Plugin type: `process` or `source`                |
| `init_parameters` | array  | Parameters for plugin initialization (via Config) |
| `parameters`      | array  | Parameters for plugin execution (via Request)     |

**Parameter Fields:**

| Field         | Type    | Description                             |
|---------------|---------|-----------------------------------------|
| `name`        | string  | Parameter name                          |
| `required`    | boolean | Whether parameter is required           |
| `default`     | string  | Default value (optional)                |
| `description` | string  | Parameter description (optional)        |
| `options`     | array   | Valid options (optional, if restricted) |

---

### 7. Configs (配置管理)

Configs API for managing system configuration.

#### GET /api/v1/configs/:group/:name

Retrieve a specific config value.

**Response:**

```json
{
  "group": "workflow",
  "name": "max_retries",
  "value": "3"
}
```

#### PUT /api/v1/configs/:group/:name

Set a config value.

**Request Body:**

```json
{
  "value": "5"
}
```

**Response:**

```json
{
  "group": "workflow",
  "name": "max_retries",
  "value": "5"
}
```

#### GET /api/v1/configs/:group

List all configs in a group.

**Response:**

```json
{
  "items": [
    {
      "group": "workflow",
      "name": "max_retries",
      "value": "3",
      "changed_at": "2024-01-01T00:00:00Z"
    },
    {
      "group": "workflow",
      "name": "default_timeout",
      "value": "600",
      "changed_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### DELETE /api/v1/configs/:group/:name

Delete a config.

**Response:**

```json
{
  "group": "workflow",
  "name": "max_retries",
  "deleted": true
}
```

---

## Error Responses

All errors follow a consistent format:

```json
{
  "error": "Error message describing the problem"
}
```

**Common HTTP Status Codes**

| Code | Description                             |
|------|-----------------------------------------|
| 200  | OK - Request successful                 |
| 201  | Created - Resource created successfully |
| 400  | Bad Request - Invalid parameters        |
| 401  | Unauthorized - Authentication required  |
| 404  | Not Found - Resource does not exist     |
| 500  | Internal Server Error                   |

**Error Codes:**

| Code                   | Description                    |
|------------------------|--------------------------------|
| `INVALID_ARGUMENT`     | Invalid request parameters     |
| `NOT_FOUND`            | Resource not found             |
| `CONFIG_NOT_FOUND`     | Configuration not found        |
| `SET_CONFIG_FAILED`    | Failed to set configuration    |
| `LIST_CONFIG_FAILED`   | Failed to list configurations  |
| `DELETE_CONFIG_FAILED` | Failed to delete configuration |

---

## Rate Limiting

Currently, no rate limiting is enforced. Production deployments should configure rate limiting at the load balancer
level.

---

## Versioning

The API uses path versioning (`/api/v1/`). Breaking changes will be introduced in new versions (e.g., `/api/v2/`).
