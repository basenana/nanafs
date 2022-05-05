# NanaFS

NanaFS is a fs-style workflow engine.
It is also a warehouse for unified management of your structured and unstructured data.

In personal work, study and life, a large amount of data and materials are stored in different information islands.
Such as Office files, Email, Todo List. I've spent a lot of time writing glue workflows make they can be managed
together.
Someday, I got bored: fuck it, why not make an FS with a built-in workflow, then NanaFS is here.

## Usage

### Requirements

1. In POSIX-FS use case, FUSE lib is required (other case, such as using api or webdav only, FUSE is optional)
2. If you need to build the binary yourself, Docker is needed.

Luckily, installing FUSE or Docker is pretty straightforward.

For Ubuntu, FUSE can be installed using `apt`:

```bash
sudo apt-get install -y libfuse3-dev fuse3 libssl-dev
```

For macOS, same operation like linux, `brew` will make things done:

```bash
brew install --cask osxfuse
```

### Build

If you need to build the binary yourself, just `make` it:

```bash
make build
```

### Run

Before trigger nanafs run, you should make sure the configuration is correct, 
We provide a tool to quickly edit and generate configuration files.

Generate default local configuration with the following command, 
and you will get a default config file in your home path:

```bash
nanafs config init
```

Edit local configuration file obtained in the previous step:

```json
{
  "debug": false,
  "api": {
    "enable": true,
    "port": 8080,
    "pprof": true
  },
  "fs": {
    "enable": true,
    "root_path": "/your/nanafs/mountpoint"
  },
  "meta": {
    "type": "sqlite",
    "path": "/your/data/path/sqlite.db"
  },
  "storages": [
    {
      "id": "local",
      "local_dir": "/your/data/path"
    }
  ]
}
```

Run nanafs:

```bash
nanafs serve
```

Now, you successfully got a slowly and buggy FS.

## Architecture

TODO
