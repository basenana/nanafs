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
default configuration may not be the optimal configuration
([How to config NanaFS](https://github.com/basenana/nanafs/blob/main/docs/configuration.md)).

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

