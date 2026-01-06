# Plugin

This is the plugin repository for [NanaFS](https://github.com/basenana/nanafs), providing extensible plugins for
workflow and file system operations.

All built-in plugins are organized in subdirectories.

---

## Built-in Plugins

| Plugin | Type | Description |
|--------|------|-------------|
| `archive` | Process | Extract/create archive files (zip, tar, gzip) |
| `checksum` | Process | Compute file checksums (MD5, SHA256) |
| `docloader` | Process | Parse documents (PDF, TXT, MD, HTML, EPUB, CSV) |
| `fileop` | Process | File operations (copy, move, remove, rename) |
| `filewrite` | Process | Write content to files |
| `save` | Process | Save files to NanaFS |
| `update` | Process | Update NanaFS entries |
| `metadata` | Process | Get file metadata |
| `rss` | Source | Sync RSS/Atom feeds |
| `text` | Process | Text manipulation |
| `webpack` | Process | Archive web pages |

---

## Architecture

### Plugin Interface Hierarchy

```
Plugin (base interface)
├── Name() string
├── Type() types.PluginType (source/process)
├── Version() string
│
├── ProcessPlugin: Run(ctx, request) (*Response, error)
│   └── SourcePlugin: SourceInfo() (string, error)
```

### Request/Response API

```go
// Request fields
type Request struct {
    JobID       string              // Job identifier
    Namespace   string              // Plugin namespace
    WorkingPath string              // Working directory
    PluginName  string              // Plugin name
    Parameter   map[string]any      // Plugin parameters
    Store       PersistentStore     // Persistent storage
    FS          NanaFS              // File system interface
}

// Response helpers
api.NewResponse()                          // Success
api.NewResponseWithResult(map[string]any)  // Success with result
api.NewFailedResponse("error")             // Failure
```

### Parameter Access

Parameters come from two sources:

**1. Request Parameters** (runtime parameters from workflow YAML):
```go
// Get parameter from request (workflow execution context)
api.GetStringParameter("key", request, "default")
```

**2. PluginCall Parameters** (initialization parameters from config):
```go
// Parameters passed during plugin initialization (factory function)
func NewMyPlugin(ps types.PluginCall) types.Plugin {
    algorithm := ps.Params["algorithm"]  // From config
    // ...
}
```

| Parameter Source | When to Use | Access Method |
|------------------|-------------|---------------|
| Request | Values that change per execution | `api.GetStringParameter()` in `Run()` |
| PluginCall | Values fixed at initialization | Direct access in factory function |

**Note**: Some plugins read parameters at initialization time (e.g., `algorithm` in checksum), while others read all parameters at runtime (e.g., `fileop`). Check individual plugin documentation for details.

---

## ProcessPlugin Example

**File:** `process.go`

```go
type DelayProcessPlugin struct{}

func (d *DelayProcessPlugin) Name() string    { return "delay" }
func (d *DelayProcessPlugin) Type() types.PluginType { return types.TypeProcess }
func (d *DelayProcessPlugin) Version() string { return "1.0" }

func (d *DelayProcessPlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
    delayStr := api.GetStringParameter("delay", request, "")

    // Perform action
    // ...

    return api.NewResponse(), nil
}
```

### Key Points

1. **Name()**: Returns the unique plugin identifier
2. **Type()**: Returns `types.TypeProcess` for ProcessPlugin
3. **Version()**: Returns semantic version string
4. **Run()**: Main execution method

---

## SourcePlugin Example

**File:** `source.go`

```go
type ThreeBodyPlugin struct{}

func (d *ThreeBodyPlugin) Name() string    { return "three_body" }
func (d *ThreeBodyPlugin) Type() types.PluginType { return types.TypeSource }
func (d *ThreeBodyPlugin) Version() string { return "1.0" }

func (d *ThreeBodyPlugin) SourceInfo() (string, error) {
    return "internal.FileGenerator", nil
}

func (d *ThreeBodyPlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
    // Generate content/files
    // ...

    return api.NewResponseWithResult(map[string]any{
        "file_path": "output.txt",
        "size":      1024,
    }), nil
}
```

### Key Points

1. **SourcePlugin extends ProcessPlugin**: Inherits all ProcessPlugin methods
2. **SourceInfo()**: Returns category identifier (`category.Name`)

---

## Factory Pattern

Plugins use a factory function for per-request state initialization:

```go
type Factory func(ps types.PluginCall) types.Plugin

func NewDelayPlugin(ps types.PluginCall) types.Plugin {
    return &DelayPlugin{
        logger: logger.NewPluginLogger("delay", ps.JobID),
    }
}
```

### PluginCall Structure

```go
type PluginCall struct {
    JobID      string            // Job identifier
    Workflow   string            // Workflow name
    PluginName string            // Plugin name
    Version    string            // Plugin version
    Params     map[string]string // Parameters from config
}
```

---

## Adding a New Plugin

### 1. Create Plugin File

```go
package myplugin

import (
    "context"

    "github.com/basenana/plugin/api"
    "github.com/basenana/plugin/logger"
    "github.com/basenana/plugin/types"
    "go.uber.org/zap"
)

const (
    pluginName    = "myplugin"
    pluginVersion = "1.0"
)

var PluginSpec = types.PluginSpec{
    Name:    pluginName,
    Version: pluginVersion,
    Type:    types.TypeProcess,
}

type MyPlugin struct {
    logger *zap.SugaredLogger
}

func NewMyPlugin(ps types.PluginCall) types.Plugin {
    return &MyPlugin{
        logger: logger.NewPluginLogger(pluginName, ps.JobID),
    }
}

func (p *MyPlugin) Name() string           { return pluginName }
func (p *MyPlugin) Type() types.PluginType { return types.TypeProcess }
func (p *MyPlugin) Version() string        { return pluginVersion }

func (p *MyPlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
    param := api.GetStringParameter("param_key", request, "default")

    // Perform action
    // ...

    return api.NewResponse(), nil
}
```

### 2. Register Plugin in registry.go

```go
func New() Manager {
    m := &manager{
        plugins: map[string]*pluginInfo{},
        logger:  logger.NewLogger("registry"),
    }

    m.Register(archive.PluginSpec, archive.NewArchivePlugin)
    // Add your plugin
    m.Register(MyPluginSpec, NewMyPlugin)

    return &manager{r: m}
}
```

---

## Commands

```bash
# Build the project
go build ./...

# Run tests
go test ./...
```
