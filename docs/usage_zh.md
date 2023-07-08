# Instructions For Usage

<p align="right">[ <a href="https://github.com/basenana/nanafs/blob/main/docs/usage.md">English</a> | 简体中文 ]</p>

## 快速开始

📢 注意：

1. NanaFS 还在快速迭代中，配置和使用信息会随着版本变化
2. Windows 未进行过测试，但在计划中

### 下载二进制

当前和历史版本的二进制文件可以在 [release 页面](https://github.com/basenana/nanafs/releases) 上下载。

### 构建自定义版本

NanaFS 的二进制执行文件编译依赖了 Docker。如果您有构建自定义版本的需求，可以在安装 Docker 后，执行 make 命令：

```bash
make build
```

### 使用默认配置运行

在运行 NanaFS 之前，需要确保配置正确。我们提供了一个工具来快速编辑和生成配置文件。使用提供的工具生成默认的本地配置文件，但注意，生成的默认配置可能不是最优配置。

生成默认配置，生成的配置文件可在 `~/.nana` 目录中找到：

```bash
nanafs config init
```

默认配置使用 SQLite 作为元数据库，以本地磁盘作为后端存储。如果您需要修改为其他选项，请参考下文的配置部分。

完成 NanaFS 的配置后，可以执行下面的命令启动 NanaFS：

```bash
nanafs serve
```

该命令将使用 `~/.nana` 目录中的配置文件启动，如果您需要特定的配置文件，请使用 `--config` 指定其绝对路径。

## 配置

### FUSE

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

### WebDAV

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

### 元数据服务

NanaFS 依赖于一个元数据服务来持久化系统内的元数据和其他结构化数据。您可以使用常见的数据库作为元数据服务。

#### SQLite

SQLite 是一款常见的基于本地文件的关系型数据库，支持事务操作。您可以使用以下配置将元数据存储在本地的 SQLite 中：

```json
{
  "meta": {
    "type": "sqlite",
    "path": "/your/data/path/sqlite.db"
  }
}
```

#### PostgreSQL

PostgreSQL 是一款常见的关系型数据库。您可以使用以下配置将元数据存储在 PostgreSQL 中：

```json
{
  "meta": {
    "type": "postgres",
    "dsn": "postgres://user:pass@host:port/dbname"
  }
}
```

### 存储

NanaFS 支持配置多个后端存储，每个后端存储由唯一的 `id` 区分。同时，NanaFS
也支持将不同的文件存储在不同的后端存储中。以下 `id` 仅作为示例，您可以配置为自定义字符串。

#### Local

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

#### AWS S3

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

#### MinIO

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

#### OSS

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

#### WebDAV

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

### 数据块加密

注意事项：

1. 开启加密后，请妥善保管密钥，密钥遗失会导致数据不可读
2. 不支持在配置文件修改密钥，如果需要修改密钥，需要新增一个新的 storage，并通过文件拷贝的方式迁移

NanaFS 支持将存储到云上的数据块加密，如果开启了加密选项，数据缓存和传输均是使用加密后的数据块。
当前仅支持 AES 对称加密，可以通过下述命令生成一个随机的加密密钥：

```bash
openssl rand -hex 16
```

开启数据块加密有两种方式，第一种是全局的加密开启，在配置中的 `global_encryption` 项可以进行如下配置：

```json
{
  "global_encryption": {
    "enable": true,
    "method": "AES",
    "secret_key": "<secret_key>"
  }
}
```

第二种方式是仅对某个 storage 存储的数据进行加密，当同时配置了 `global_encryption` 和 `storages.encryption` 时，会优先使用
storage 自己的加密配置：

```json
{
  "storages": [
    {
      "encryption": {
        "enable": true,
        "method": "AES",
        "secret_key": "<secret_key>"
      }
    }
  ]
}
```

## Deployment

### Systemd

在 Linux 中，可以使用 Systemd 部署 NanaFS。首先，将 NanaFS 的配置文件保存在 `/etc/nanafs/nanafs.conf` 中。

然后新增 Systemd 配置文件：

```bash
$ cat /etc/systemd/system/nanafs.service

[Unit]
Description=NanaFS is FS-style workflow engine for unified data management.
Requires=network-online.target
After=network-online.target

[Service]
User=root
ExecStart=/usr/local/bin/nanafs serve --config /etc/nanafs/nanafs.conf
Restart=always
TimeoutSec=900

[Install]
WantedBy=multi-user.target
```

最后，重新加载 Systemd 配置，并启动 NanaFS：

```bash
systemctl daemon-reload
systemctl start nanafs
```

如需配置开机自动启动，可以执行：

```bash
systemctl enable nanafs
```

如需查看 NanaFS 日志可使用：

```bash
journalctl -u nanafs
```

### Docker

TODO