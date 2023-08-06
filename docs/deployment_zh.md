# Deployment

<p align="right">[ <a href="https://github.com/basenana/nanafs/blob/main/docs/deployment.md">English</a> | 简体中文 ]</p>

## Systemd

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

## Docker

TODO

## 群晖

TODO