# Instructions For Usage

<p align="right">[ English | <a href="https://github.com/basenana/nanafs/blob/main/docs/usage_zh.md">简体中文</a> ]</p>

## Quick Start

📢 Note:

1. NanaFS is still rapidly iterating. Configuration and usage information may change with version changes.
2. Windows has not been tested, but it is planned.

### Download Binary

Current and historical versions of binary files for NanaFS can be downloaded on
the [release page](https://github.com/basenana/nanafs/releases).

### Build Custom Version

The binary executable file compilation of NanaFS depends on Docker. If you need to build a custom version, you can
execute the `make` command after installing Docker:

```bash
make build
```

### Run with Default Configuration

Before running NanaFS, make sure the configuration is correct. We provide a tool to quickly edit and generate
configuration files. Use the provided tool to generate the default local configuration file, but note that the generated
default configuration may not be the optimal configuration.

Generate the default configuration, and the generated configuration file can be found in the `~/.nana` directory:

```bash
nanafs config init
```

The default configuration uses SQLite as the metadata database and local disk as the backend storage. If you need to
modify it to other options, please refer to the configuration section below.

After completing the configuration of NanaFS, you can execute the following command to start NanaFS:

``` bash
nanafs serve
```

This command will start using the configuration file in the `~/.nana` directory. If you need a specific configuration
file, please use `--config` to specify its absolute path.

## Configuration

### FUSE

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

Finally, you can configure `fuse=true` and configure the mount point path to enable FUSE service:

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

### Metadata Service

NanaFS relies on a metadata service to persist metadata and other structured data within the system. You can use common
databases as metadata services.

#### SQLite

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

#### PostgreSQL

PostgreSQL is a common relational database. You can use the following configuration to store metadata in PostgreSQL:

```json
{
  "meta": {
    "type": "postgres",
    "dsn": "postgres://user:pass@host:port/dbname"
  }
}
```

### Storage

NanaFS supports configuring multiple backend storage, each of which is distinguished by a unique `id`. At the same time,
NanaFS also supports storing different files in different backend storages. The following `id` is just an example, and
you can configure it as a custom string.

#### Local

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

#### MinIO

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
        "bucket_name": ""
      }
    }
  ]
}
```

#### OSS

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

#### WebDAV

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

## Deployment

### Systemd

In Linux, Systemd can be used to deploy NanaFS. First, save NanaFS configuration file to /etc/nanafs/nanafs.conf.

Next, add Systemd configuration file:

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

Finally, reload the Systemd configuration and start NanaFS:

```bash
systemctl daemon-reload
systemctl start nanafs
```

If you want to configure automatic startup at boot, you can execute:

```bash
systemctl enable nanafs
```

If you need to view NanaFS logs, use:

```bash
journalctl -u nanafs
```

### Docker

TODO