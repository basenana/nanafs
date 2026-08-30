# pkg/cmdb

## Purpose

Configuration Management Database for NanaFS: a minimal key-value store interface for system and per-namespace configuration, with an in-memory implementation and a `SetCMDBDefaultConfigs()` bootstrap function that seeds default values.

## Key Files

| File | Responsibility |
|------|----------------|
| `cmdb.go` | `CMDB` interface, `memCmdb` in-memory implementation, `cacheConfigKey`, `Value`, `NewMemCmdb()`, `SetCMDBDefaultConfigs()`, `IsConfigNotFound()` |

## Core Capabilities

**`CMDB` interface:**
- `GetConfigValue(ctx, namespace, group, name string) (string, error)`
- `SetConfigValue(ctx, namespace, group, name, value string) error`

**Helpers:**
- `NewMemCmdb() CMDB` — mutex-protected `map[cacheConfigKey]string` implementation.
- `SetCMDBDefaultConfigs(db CMDB) error` — populates defaults only where keys are absent.
- `IsConfigNotFound(err error) bool` — not-found detection based on error message.
- Group constants: `DocConfigGroup = "document"`, `PluginConfigGroup = "plugin"`, `WorkflowConfigGroup = "workflow"`.

## Upstream (Consumers)

- `config/` (registers a CMDB behind the `Config` interface for runtime system/namespaced config).

## Downstream (Dependencies)

- `github.com/basenana/nanafs/utils/logger`
- Standard library (`context`, `sync`, ...).

## Design Notes

- Deliberately tiny interface — the persistence strategy is decided by whoever implements it; `memCmdb` is the default. In practice the REST layer wires metastore-backed config into this shape via `config.Config`.
- Not-found convention: error message contains "no record"; check with `IsConfigNotFound` rather than comparing errors directly.

## Testing

No test files in this package.
