# NanaFS

NanaFS is a fs-style workflow engine.
It is also a warehouse for unified management of your structured and unstructured data.

In personal work, study and life, a large amount of data and materials are stored in different information islands.
Such as Office files, Email, Todo List. I've spent a lot of time writing glue workflows make they can be managed
together.
Someday, I got bored: Fuck it, why not make a FS with a built-in workflow, then NanaFS is here.

## Usage

Generate default local configuration:
```bash
nanafs config init
```

Edit local configuration:
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
nanafs serve --config=/your/config/path
```

## Architecture

TODO
