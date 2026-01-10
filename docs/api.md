# NanaFS REST API Reference

## Overview

NanaFS provides a RESTful API built with [Gin](https://gin-gonic.com/) framework. All API endpoints are prefixed with `/api/v1/` unless otherwise specified.

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

All entry-related endpoints support two access patterns:
- **By URI**: Use `?uri=/path/to/entry` query parameter
- **By ID**: Use `?id=12345` query parameter

#### GET /api/v1/entries/details

Retrieve details of a specific entry.

**Query Parameters**

| Name | Type | Description |
|------|------|-------------|
| `uri` | string | Entry URI path (e.g., `/inbox/tasks/task-001`) |
| `id` | int64 | Entry ID |

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
      "permissions": ["owner_read", "owner_write", "group_read", "group_write", "others_read"]
    },
    "property": null,
    "document": {
      "title": "Document Title",
      "author": "John Doe",
      "year": "2024",
      "source": "https://example.com",
      "abstract": "...",
      "keywords": ["tag1", "tag2"],
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
    "tags": ["important", "review"],
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
    "keywords": ["tag1", "tag2"],
    "notes": "",
    "url": "https://example.com/article",
    "header_image": ""
  }
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `uri` | string | yes | Entry URI path |
| `kind` | string | no | Entry type: `raw`, `group`, `smart_group`, `fifo`, `socket`, `sym_link`, `blk_dev`, `char_dev`, `external_group` |
| `rss` | object | no | RSS subscription config |
| `filter` | object | no | Filter config for smart groups |
| `properties` | object | no | Entry properties including tags and custom properties |
| `document` | object | no | Document-specific properties (only for non-group entries) |

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

#### POST /api/v1/entries/search

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

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `cel_pattern` | string | yes | CEL expression to filter entries |
| `page` | int64 | no | Page number (default: 1) |
| `page_size` | int64 | no | Number of items per page (default: all) |
| `sort` | string | no | Sort field: `created_at`, `changed_at`, `name` (default: `name`) |
| `order` | string | no | Sort order: `asc`, `desc` (default: `asc`) |

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

#### PUT /api/v1/entries

Update entry attributes. Supports `?uri=` or `?id=` query parameters.

**Query Parameters**

| Name | Type | Description |
|------|------|-------------|
| `uri` | string | Entry URI path |
| `id` | int64 | Entry ID |

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

#### DELETE /api/v1/entries

Delete a single entry. Supports `?uri=` or `?id=` query parameters.

**Query Parameters**

| Name | Type | Description |
|------|------|-------------|
| `uri` | string | Entry URI path |
| `id` | int64 | Entry ID |

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

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `entry_uri` | string | yes | URI of the entry to move |
| `new_entry_uri` | string | yes | New parent URI |
| `replace` | bool | no | Replace existing entry if exists |
| `exchange` | bool | no | Exchange positions |

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

Update tags and custom properties. Supports `?uri=` or `?id=` query parameters.

**Query Parameters**

| Name | Type | Description |
|------|------|-------------|
| `uri` | string | Entry URI path |
| `id` | int64 | Entry ID |

**Request Body:**
```json
{
  "tags": ["important", "review"],
  "properties": {
    "priority": "high",
    "due_date": "2024-01-15"
  }
}
```

**Response:**
```json
{
  "properties": {
    "tags": ["important", "review"],
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

| Name | Type | Description |
|------|------|-------------|
| `uri` | string | Entry URI path |
| `id` | int64 | Entry ID |

**Request Body:**
```json
{
  "title": "Updated Title",
  "author": "Author Name",
  "year": "2024",
  "source": "https://example.com",
  "abstract": "Document abstract...",
  "notes": "Personal notes",
  "keywords": ["tag1", "tag2"],
  "url": "https://example.com/article",
  "site_name": "Site Name",
  "site_url": "https://example.com",
  "header_image": "https://example.com/image.jpg",
  "unread": true,
  "marked": false,
  "publishAt": 1704067200
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `title` | string | Document title |
| `author` | string | Author name |
| `year` | string | Publication year |
| `source` | string | Source URL |
| `abstract` | string | Document abstract |
| `notes` | string | Personal notes |
| `keywords` | array | Keywords/tags |
| `url` | string | Article URL |
| `site_name` | string | Website name |
| `site_url` | string | Website URL |
| `header_image` | string | Header image URL |
| `unread` | bool | Mark as unread |
| `marked` | bool | Mark as starred |
| `publishAt` | int64 | Publish timestamp (Unix) |

**Response:**
```json
{
  "properties": {
    "title": "Document Title",
    "author": "John Doe",
    "year": "2024",
    "source": "https://example.com",
    "abstract": "...",
    "keywords": ["tag1", "tag2"],
    "notes": "",
    "unread": true,
    "marked": false,
    "publish_at": "2024-01-01T00:00:00Z",
    "url": "https://example.com/article",
    "header_image": ""
  }
}
```

---

### 3. Groups (组管理)

#### GET /api/v1/groups/children

List all children of a group. Supports `?uri=` or `?id=` query parameters.

**Query Parameters**

| Name | Type | Description |
|------|------|-------------|
| `uri` | string | Group URI path |
| `id` | int64 | Group ID |
| `page` | int64 | Page number (default: 1) |
| `page_size` | int64 | Number of items per page (default: all) |
| `sort` | string | Sort field: `created_at`, `changed_at`, `name` (default: `name`) |
| `order` | string | Sort order: `asc`, `desc` (default: `asc`) |

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

#### GET /api/v1/files/content

Read file content. Supports `?uri=` or `?id=` query parameters.

**Query Parameters**

| Name | Type | Description |
|------|------|-------------|
| `uri` | string | File URI path |
| `id` | int64 | File ID |

**Response:** `application/octet-stream`

#### POST /api/v1/files/content

Upload/write file content. Supports `?uri=` or `?id=` query parameters.

**Content-Type:** `multipart/form-data`

**Form Field:** `file` (the file to upload)

**Query Parameters**

| Name | Type | Description |
|------|------|-------------|
| `uri` | string | File URI path |
| `id` | int64 | File ID |

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

| Name | Type | Description |
|------|------|-------------|
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
  "message_id_list": ["msg-001", "msg-002"]
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

**Response:**
```json
{
  "workflows": [
    {
      "id": "wf-001",
      "name": "Process RSS",
      "queue_name": "rss-processing",
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z",
      "last_triggered_at": "2024-01-02T00:00:00Z"
    }
  ]
}
```

#### GET /api/v1/workflows/:id/jobs

List jobs for a specific workflow.

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
        "entries": ["/inbox/rss/feed-001"]
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
  ]
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

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Workflow name |
| `trigger` | object | yes | Trigger configuration (RSS, Interval, or LocalFileWatch) |
| `nodes` | array | yes | Workflow nodes/steps |
| `enable` | bool | no | Enable/disable workflow (default: true) |
| `queue_name` | string | no | Queue name for job processing |

**Response:**
```json
{
  "workflow": {
    "id": "wf-002",
    "name": "Process RSS Feed",
    "queue_name": "rss-processing",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z",
    "last_triggered_at": null
  }
}
```

#### GET /api/v1/workflows/:id

Retrieve a specific workflow.

**Response:**
```json
{
  "workflow": {
    "id": "wf-001",
    "name": "Process RSS",
    "queue_name": "rss-processing",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z",
    "last_triggered_at": "2024-01-02T00:00:00Z"
  }
}
```

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
      "id": "job-001",
      "workflow": "wf-001",
      "trigger_reason": "manual",
      "status": "completed",
      "message": "",
      "queue_name": "rss-processing",
      "target": {
        "entries": ["/inbox/rss/feed-001"]
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
  ]
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
      "entries": ["/inbox/rss/feed-001"]
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
  "timeout": 300
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `uri` | string | no | Entry URI to trigger workflow on |
| `reason` | string | no | Reason for triggering |
| `timeout` | int64 | no | Timeout in seconds (default: 600) |

**Response:**
```json
{
  "job_id": "job-002"
}
```

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

| Code | Description |
|------|-------------|
| 200 | OK - Request successful |
| 201 | Created - Resource created successfully |
| 400 | Bad Request - Invalid parameters |
| 401 | Unauthorized - Authentication required |
| 404 | Not Found - Resource does not exist |
| 500 | Internal Server Error |

**Error Codes:**

| Code | Description |
|------|-------------|
| `INVALID_ARGUMENT` | Invalid request parameters |
| `NOT_FOUND` | Resource not found |
| `CONFIG_NOT_FOUND` | Configuration not found |
| `SET_CONFIG_FAILED` | Failed to set configuration |
| `LIST_CONFIG_FAILED` | Failed to list configurations |
| `DELETE_CONFIG_FAILED` | Failed to delete configuration |

---

## Rate Limiting

Currently, no rate limiting is enforced. Production deployments should configure rate limiting at the load balancer level.

---

## Versioning

The API uses path versioning (`/api/v1/`). Breaking changes will be introduced in new versions (e.g., `/api/v2/`).
