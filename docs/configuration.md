# Configuration

<p align="right">[ English | <a href="https://github.com/basenana/nanafs/blob/main/docs/configuration_zh.md">简体中文</a> ]</p>

## API Service

You can configure the API service to enable the REST API:

```json
{
  "api": {
    "enable": true,
    "host": "127.0.0.1",
    "port": 8080,
    "server_name": "nanafs",
    "token_secret": "your-secret-key",
    "noauth": false
  }
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `enable` | bool | Enable the API service |
| `host` | string | Bind address |
| `port` | int | Port number |
| `server_name` | string | Server name |
| `token_secret` | string | JWT token secret |
| `noauth` | bool | Disable authentication |

## FUSE

If you want to use NanaFS with the POSIX file system, you need to install the FUSE library to work. But if you only use
API or WebDAV, FUSE is optional.

For Ubuntu, you can use apt to install FUSE dependencies:

```bash
sudo apt-get install -y libfuse3-dev fuse3 libssl-dev
```

For MacOS, you can use brew and other package management to install FUSE dependencies:

```bash
brew install --cask osxfuse
```

Finally, you can configure `fuse.enable=true` and configure the mount point path to enable FUSE service:

```json
{
  "fuse": {
    "enable": true,
    "root_path": "/your/path/to/mount",
    "mount_options": ["allow_other"],
    "display_name": "nanafs",
    "verbose_log": false,
    "entry_timeout": 60,
    "attr_timeout": 60
  }
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `enable` | bool | Enable FUSE service |
| `root_path` | string | Mount point path |
| `mount_options` | []string | FUSE mount options |
| `display_name` | string | Display name |
| `verbose_log` | bool | Enable verbose logging |
| `entry_timeout` | int | Entry cache timeout (seconds) |
| `attr_timeout` | int | Attribute cache timeout (seconds) |

## WebDAV

You can configure `webdav.enable=true` and related network user configuration to enable WebDAV service:

```json
{
  "webdav": {
    "enable": true,
    "host": "127.0.0.1",
    "port": 7082,
    "overwrite_users": [
      {
        "uid": 0,
        "gid": 0,
        "username": "admin",
        "password": "changeme"
      }
    ]
  }
}
```

## Metadata Service

NanaFS relies on a metadata service to persist metadata and other structured data within the system. You can use common
databases as metadata services.

### Memory

Memory metadata is for testing purposes only. Data will be lost after restart.

```json
{
  "meta": {
    "type": "memory"
  }
}
```

### SQLite

SQLite is a common file-based relational database that supports transaction operations. You can use the following
configuration to store metadata locally in SQLite:

```json
{
  "meta": {
    "type": "sqlite",
    "path": "/your/data/path/sqlite.db"
  }
}
```

### PostgreSQL

PostgreSQL is a common relational database. You can use the following configuration to store metadata in PostgreSQL:

```json
{
  "meta": {
    "type": "postgres",
    "dsn": "postgres://user:pass@host:port/dbname"
  }
}
```

## Storage

NanaFS supports configuring multiple backend storage, each of which is distinguished by a unique `id`. At the same time,
NanaFS also supports storing different files in different backend storages. The following `id` is just an example, and
you can configure it as a custom string.

### Local

`type=local` is a local directory-based storage method. When using this configuration, the data in NanaFS will be stored
in the specified directory.

```json
{
  "storages": [
    {
      "id": "local-0",
      "type": "local",
      "local_dir": "/your/data/path/local"
    }
  ]
}
```

### AWS S3

`type=s3` is a storage method based on AWS S3. When using this configuration,
the data in NanaFS will be stored in the specified Bucket.

```json
{
  "storages": [
    {
      "id": "aws-s3-0",
      "type": "s3",
      "s3": {
        "region": "",
        "access_key_id": "",
        "secret_access_key": "",
        "bucket_name": "",
        "use_path_style": false
      }
    }
  ]
}
```

### MinIO

`type=minio` is a storage method based on MinIO. MinIO is a widely used open-source object storage that also supports
use as an object storage gateway. When using this configuration, the data in NanaFS will be stored in the specified
Bucket.

```json
{
  "storages": [
    {
      "id": "custom-minio-0",
      "type": "minio",
      "minio": {
        "endpoint": "",
        "access_key_id": "",
        "secret_access_key": "",
        "bucket_name": "",
        "location": "",
        "token": "",
        "use_ssl": false
      }
    }
  ]
}
```

**Additional Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `location` | string | MinIO location |
| `token` | string | MinIO token |
| `use_ssl` | bool | Use SSL connection |

### OSS

`type=oss` is a storage method based on OSS. OSS is a S3 compatible object storage service provided by Alibaba
Cloud. When using this configuration, the data in NanaFS will be stored in the specified Bucket.

```json
{
  "storages": [
    {
      "id": "custom-oss-0",
      "type": "oss",
      "oss": {
        "endpoint": "",
        "access_key_id": "",
        "access_key_secret": "",
        "bucket_name": ""
      }
    }
  ]
}
```

### WebDAV

`type=webdav` is a storage method based on the WebDAV protocol. WebDAV is a common storage protocol. When using this
configuration, the data in NanaFS is stored in the remote storage system via the WebDAV protocol.

```json
{
  "storages": [
    {
      "id": "custom-webdav-0",
      "type": "webdav",
      "webdav": {
        "server_url": "",
        "username": "",
        "password": "",
        "insecure": false
      }
    }
  ]
}
```

### Memory

`type=memory` is a memory-based storage method. Data is stored in memory and will be lost after restart.
This storage type is for testing purposes only.

```json
{
  "storages": [
    {
      "id": "memory-0",
      "type": "memory"
    }
  ]
}
```

## Encryption

Notice:

1. After enabling encryption, please keep the secret key safe, the loss of the key will make the data unreadable.
2. It is not supported to modify the secret key in the configuration file. If you need to modify the key, you need to
   add a new storage and migrate data by file copy.

NanaFS supports the encryption of chunk pages stored on the cloud. If the encryption option is enabled, data caching and
transmission will use encrypted pages.
Currently only AES symmetric encryption is supported, and a random encryption key can be generated by the following
command:

```bash
openssl rand -hex 16
```

Use the `encryption` in the configuration to enable global encryption:

```json
{
  "encryption": {
    "enable": true,
    "method": "AES",
    "secret_key": "<secret_key>"
  }
}
```

## Workflow

NanaFS provides a workflow engine for automating file processing tasks.

```json
{
  "workflow": {
    "enable": true,
    "job_workdir": "/tmp/nanafs-jobs",
    "integration": {
      "document_webhook": ""
    }
  }
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `enable` | bool | Enable workflow engine |
| `job_workdir` | string | Working directory for jobs |
| `integration.document_webhook` | string | Webhook URL for document integration |

## File System

Configure FUSE file system options:

```json
{
  "fs": {
    "owner": {
      "uid": 1000,
      "gid": 1000
    },
    "writeback": false,
    "page_size": 4096
  }
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `owner.uid` | int64 | File owner user ID |
| `owner.gid` | int64 | File owner group ID |
| `writeback` | bool | Enable writeback mode |
| `page_size` | int | Page size for caching |

## Cache

Configure local cache for better performance:

```json
{
  "cache_dir": "/tmp/nanafs-cache",
  "cache_size": 1024,
  "debug": false
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `cache_dir` | string | Cache directory path |
| `cache_size` | int | Cache size in MB (0 for unlimited) |
| `debug` | bool | Enable debug mode |
