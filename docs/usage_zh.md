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

在运行 NanaFS 之前，需要确保配置正确。我们提供了一个工具来快速编辑和生成配置文件。使用提供的工具生成默认的本地配置文件，
但注意，生成的默认配置可能不是最优配置
（[如何配置 NanaFS](https://github.com/basenana/nanafs/blob/main/docs/configuration_zh.md)）。

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
