# Configuration

<p align="right">[ <a href="https://github.com/basenana/nanafs/blob/main/docs/configuration.md">English</a> | 简体中文 ]</p>

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

最后，您可以配置 `fuse=true` 并配置挂载点路径以开启 FUSE 服务：

```json
{
  "fuse": {
    "enable": true,
    "root_path": "/your/path/to/mount",
    "display_name": "nanafs"
  }
}
```

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
        "bucket_name": ""
      }
    }
  ]
}
```

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