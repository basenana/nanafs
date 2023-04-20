# NanaFS

NanaFS is a workflow engine that simplifies data management
by allowing users to manage structured and unstructured data in one place,
rather than across multiple sources. It's like a filing cabinet
that combines all your documents, emails, and to-do lists,
making it easier to manage everything at once.

NanaFS is also customizable through plugin support,
meaning users can tailor the workflow engine to their specific needs.
This makes NanaFS a versatile and valuable tool for personal, academic, and professional use.

## Usage

### Requirements

1. NanaFS requires the FUSE library for POSIX-FS use cases, but it is optional if you are using the API or WebDAV
   exclusively.
2. In order to build the NanaFS binary, Docker is needed. So if you plan to build it yourself, you will need to have
   Docker installed first.

Fortunately, installing FUSE or Docker is a straightforward process.

For Ubuntu, FUSE can be installed using `apt`:

```bash
sudo apt-get install -y libfuse3-dev fuse3 libssl-dev
```

For macOS, same operation like linux, `brew` will make things done:

```bash
brew install --cask osxfuse
```

### Deploy using the binary file

The current and historical versions of the binary files can be downloaded on
the [release page](https://github.com/basenana/nanafs/releases).

### Build your own version

If your own version needs to be built, just `make` it:

```bash
make build
```

### Run

Before running a trigger in NanaFS, ensure the configuration is correct.

We provide a tool to quickly edit and generate configuration files.
Generate a default local configuration file using the provided tool,
but keep in mind that the generated configuration may not be optimal.

```bash
nanafs config init
```

The generated configuration file can be found in the `~/.nana` directory:

```json
{
  "api": {
    "enable": true,
    "host": "127.0.0.1",
    "port": 7081,
    "pprof": false
  },
  "fuse": {
    "enable": false,
    "root_path": "/your/path/to/mount",
    "display_name": "nanafs"
  },
  "webdav": {
    "enable": false,
    "host": "127.0.0.1",
    "port": 7082,
    "users": [
      {
        "uid": 0,
        "gid": 0,
        "username": "admin",
        "password": "changeme"
      }
    ]
  },
  "meta": {
    "type": "sqlite",
    "path": "/your/data/path/sqlite.db"
  },
  "storages": [
    {
      "id": "local-0",
      "type": "local",
      "local_dir": "/your/data/path/local"
    }
  ],
  "cache_dir": "/your/data/path/cache",
  "cache_size": 10
}
```

Run nanafs:

```bash
nanafs serve
```

## Architecture

TODO
