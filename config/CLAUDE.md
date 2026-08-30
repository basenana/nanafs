# config

## Purpose

Application configuration management for NanaFS. Loads the JSON-based `Bootstrap` configuration, applies defaults, validates it through a verifier chain, and exposes both static bootstrap values and dynamic CMDB-backed runtime config (system-wide and per-namespace) with a 15-minute cache.

## Key Files

| File | Responsibility |
|------|----------------|
| `boot.go` | `Bootstrap` struct and sub-configs: `FsApi`, `Webdav`, `FUSE`, `Encryption`, `Workflow`, `JWT`, `GoogleOAuth`, `Integration`, `Friday`, `LLM` |
| `config.go` | `Config` interface, `configWrapper` implementation, `Value` wrapper type, `FindConfigFile()`, `NewConfig()`, `NewMockConfig()` |
| `verify.go` | Verifier chain: `verifiers` slice of `verifier` funcs and `Verify()` entry point |
| `default.go` | `DefaultConfig()` factory and `generateSecretKey()` |
| `fs.go` | `FS`, `FSOwner`, `Meta`, `Storage` structs; storage backend configs (`S3Config`, `MinIOConfig`, `OSSConfig`, `WebdavStorageConfig`); storage/meta type constants; encryption constants |
| `version.go` | `Version` struct, `Version()`, `VersionInfo()` — build info injected via ldflags |

## Core Capabilities

**`Config` interface:**
- `GetBootstrapConfig() Bootstrap`
- `RegisterCMDB(cmdb cmdb.CMDB) error`
- `SetSystemConfig(ctx, group, name string, value any) error`
- `GetSystemConfig(ctx, group, name string) Value`
- `GetNamespacedConfig(ctx, namespace, group, name string) Value`
- `SetNamespacedConfig(ctx, namespace, group, name string, value any) error`

**`Value` type:** wraps a config value with `Int()`, `Int64()`, `Bool()`, `String()`, `Unmarshal()` accessors.

**Verifier chain (in order):** `setDefaultValue`, `checkFuseConfig`, `checkMetaConfig`, `checkStorageConfigs`, `checkGlobalEncryptionConfig`, `checkLocalCache`, `checkWorkflow`.

**Constants:** meta types `MemoryMeta`, `SqliteMeta`, `PostgresMeta`; storage types `S3Storage`, `OSSStorage`, `MinioStorage`, `WebdavStorage`, `LocalStorage`, `MemoryStorage`; encryption `AESEncryption`, `ChaCha20Encryption`.

## Upstream (Consumers)

Imported by ~42 files: `pkg/metastore`, `pkg/storage`, `pkg/indexer`, `pkg/friday`, `pkg/core`, `pkg/bio`, `pkg/dispatch`, `workflow/`, and all `cmd/apps/*` entry points.

## Downstream (Dependencies)

- `github.com/basenana/nanafs/pkg/cmdb` (runtime config backend)
- `github.com/basenana/nanafs/utils`
- Standard library (`encoding/json`, `crypto/rand`, `regexp`, ...)

## Design Notes

- Two-level config model: static `Bootstrap` from JSON file + dynamic CMDB overrides (system/namespaced).
- `configWrapper` caches CMDB reads in memory with a 15-minute TTL.
- Factory pattern via `NewConfig()`; `NewMockConfig(Bootstrap)` supports tests without file I/O.
- `gitTag`/`gitCommit` in `version.go` are set at build time via `-X` ldflags (see `.goreleaser.yaml`).

## Testing

No test files in this package.
