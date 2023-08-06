# Deployment

<p align="right">[ English | <a href="https://github.com/basenana/nanafs/blob/main/docs/deployment_zh.md">简体中文</a> ]</p>

## Systemd

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

## Docker

TODO

## Synology

TODO