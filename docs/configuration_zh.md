# Configuration

<p align="right">[ <a href="https://github.com/basenana/nanafs/blob/main/docs/configuration.md">English</a> | 简体中文 ]</p>

## API 服务

您可以配置 API 服务来启用 REST API：

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

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `enable` | bool | 启用 API 服务 |
| `host` | string | 绑定地址 |
| `port` | int | 端口号 |
| `server_name` | string | 服务器名称 |
| `token_secret` | string | JWT 令牌密钥 |
| `noauth` | bool | 禁用认证 |

## FUSE

如果想要使用 POSIX 文件系统的方式使用 NanaFS，你需要安装 FUSE 库来处理。但如果你仅使用 API 或 WebDAV，FUSE 配置是可选项。

对于 Ubuntu，可以使用 apt 安装 FUSE 依赖：

```bash
sudo apt-get install -y libfuse3-dev fuse3 libssl-dev
```

对于 MacOS，可以使用 brew 等包管理器安装 FUSE 依赖：

```bash
brew install --cask osxfuse
```

最后，您可以配置 `fuse.enable=true` 并配置挂载点路径以开启 FUSE 服务：

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

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `enable` | bool | 启用 FUSE 服务 |
| `root_path` | string | 挂载点路径 |
| `mount_options` | []string | FUSE 挂载选项 |
| `display_name` | string | 显示名称 |
| `verbose_log` | bool | 启用详细日志 |
| `entry_timeout` | int | 条目缓存超时（秒） |
| `attr_timeout` | int | 属性缓存超时（秒） |

## WebDAV

您可以配置 `webdav.enable=true` 以及相关网络、用户配置以开启 WebDAV 服务：

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

## 元数据服务

NanaFS 依赖于一个元数据服务来持久化系统内的元数据和其他结构化数据。您可以使用常见的数据库作为元数据服务。

### Memory

Memory 元数据仅用于测试用途。数据将在重启后丢失。

```json
{
  "meta": {
    "type": "memory"
  }
}
```

### SQLite

SQLite 是一款常见的基于本地文件的关系型数据库，支持事务操作。您可以使用以下配置将元数据存储在本地的 SQLite 中：

```json
{
  "meta": {
    "type": "sqlite",
    "path": "/your/data/path/sqlite.db"
  }
}
```

### PostgreSQL

PostgreSQL 是一款常见的关系型数据库。您可以使用以下配置将元数据存储在 PostgreSQL 中：

```json
{
  "meta": {
    "type": "postgres",
    "dsn": "postgres://user:pass@host:port/dbname"
  }
}
```

## 存储

NanaFS 支持配置多个后端存储，每个后端存储由唯一的 `id` 区分。同时，NanaFS
也支持将不同的文件存储在不同的后端存储中。以下 `id` 仅作为示例，您可以配置为自定义字符串。

### Local

`type=local` 是基于本地目录的存储方式。使用该配置时，NanaFS 中的数据会被存储在指定的目录中。

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

`type=s3` 是基于 AWS S3 的存储方式。使用该配置时，NanaFS 中的数据会被存储在指定的 Bucket 中。

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

`type=minio` 是基于 MinIO 的存储方式，MinIO 是一个使用广泛的开源对象存储，也支持作为对象存储网关使用。使用该配置时，NanaFS
中的数据会被存储在指定的 Bucket 中。

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

**补充字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `location` | string | MinIO 区域 |
| `token` | string | MinIO 令牌 |
| `use_ssl` | bool | 使用 SSL 连接 |

### OSS

`type=oss` 是基于 OSS 的存储方式，OSS 是阿里云提供的 S3 兼容的对象存储服务。使用该配置时，NanaFS 中的数据会被存储在指定的
Bucket 中。

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

`type=webdav` 是基于 WebDAV 协议的存储方式，WebDAV 是一种常见的存储协议，使用该配置时，NanaFS 中的数据通过 WebDAV
协议存储到远端存储系统中。

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

`type=memory` 是基于内存的存储方式。数据存储在内存中，重启后会丢失。
此存储类型仅用于测试用途。

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

## 数据块加密

注意事项：

1. 开启加密后，请妥善保管密钥，密钥遗失会导致数据不可读
2. 不支持在配置文件修改密钥，如果需要修改密钥，需要新增一个新的 storage，并通过文件拷贝的方式迁移

NanaFS 支持将存储到云上的数据块加密，如果开启了加密选项，数据缓存和传输均是使用加密后的数据块。
当前仅支持 AES 对称加密，可以通过下述命令生成一个随机的加密密钥：

```bash
openssl rand -hex 16
```

开启数据块加密，在配置中的 `encryption` 项可以进行如下配置：

```json
{
  "encryption": {
    "enable": true,
    "method": "AES",
    "secret_key": "<secret_key>"
  }
}
```

## 工作流

NanaFS 提供了工作流引擎来自动化文件处理任务。

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

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `enable` | bool | 启用工作流引擎 |
| `job_workdir` | string | 任务工作目录 |
| `integration.document_webhook` | string | 文档集成 Webhook URL |

## 文件系统

配置 FUSE 文件系统选项：

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

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `owner.uid` | int64 | 文件所有者用户 ID |
| `owner.gid` | int64 | 文件所有者组 ID |
| `writeback` | bool | 启用写回模式 |
| `page_size` | int | 缓存页面大小 |

## 缓存

配置本地缓存以提升性能：

```json
{
  "cache_dir": "/tmp/nanafs-cache",
  "cache_size": 1024,
  "debug": false
}
```

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `cache_dir` | string | 缓存目录路径 |
| `cache_size` | int | 缓存大小（MB，0 表示无限制） |
| `debug` | bool | 启用调试模式 |
