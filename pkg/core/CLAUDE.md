# pkg/core

## Purpose

Central filesystem engine for NanaFS: namespace and entry lifecycle management, entry caching, raw file I/O over chunk storage, and event publishing. The `Core` interface is the main programmatic API for all file operations; `FileSystem` provides a namespace-scoped facade used by FUSE, WebDAV, and Friday.

## Key Files

| File | Responsibility |
|------|----------------|
| `core.go` | `Core` interface + `core` implementation; entry lifecycle methods; event publishing |
| `fs.go` | `FileSystem` namespace-scoped wrapper; `File`, `fsFile`, `fsDIR` implementing io interfaces; `ProbableEntryPath` |
| `group.go` | `Group` interface with `stdGroup`, `dynamicGroup`, `emptyGroup` implementations (smart groups) |
| `rawfile.go` | `RawFile` interface; `rawFile` (chunk-based) and `symlink` implementations; open-file tracking (`IsFileOpened`) |
| `cache.go` | LFU cache for entries, LRU cache for child lookups (`bluele/gcache`) |
| `access.go` | Permission checking (`IsAccess`) and mode utilities |
| `instrumental.go` | Prometheus metrics for file operations |
| `utils.go` | Root/namespace entry initialization; `publicEntryActionEvent`/`publicFileActionEvent`; `entryActionEventHandler` eventbus bridge |

## Core Capabilities

**`Core` interface (grouped by concern):**
- Namespace: `FSRoot`, `NamespaceRoot`, `CreateNamespace`
- Entry lifecycle: `GetEntry`, `GetEntryByPath`, `CreateEntry`, `UpdateEntry`, `RemoveEntry`, `DestroyEntry`, `CleanEntryData`
- Entry relationships: `FindEntry`, `ListChildren`, `ListParents`, `MirrorEntry`, `ChangeEntryParent`
- Groups: `OpenGroup` → `Group` (`FindEntry`, `ListChildren`)
- File I/O: `Open` → `RawFile` (`GetAttr`, `WriteAt`, `ReadAt`, `Fsync`, `Flush`, `Close`)
- Chunks/segments: `NextSegmentID`, `ListSegments`, `AppendSegments`, `DeleteSegment`, `ChunkCompact`

**`FileSystem`** (namespace-scoped facade): `NewFileSystem`, `Namespace`, `FsInfo`, `Root`, `LookUpEntry`, `LinkEntry`, `UnlinkEntry`, `RmGroup`, `Rename`, `OpenDir`, `GetXAttr`, `SetXAttr`, `RemoveXAttr`.

## Upstream (Consumers)

- `pkg/dispatch` — maintenance executors operate on entries and chunks
- `pkg/friday` — `FileSystem` for agent workdirs, sessions, and tools
- `workflow/jobrun/adaptor.go` — `namespacedFS` exposes core to workflow plugins
- Test suites in `cmd/apps/apis/*`

## Downstream (Dependencies)

- `pkg/metastore` (`Meta`, `EntryStore`) — metadata persistence
- `pkg/storage` (`Storage`) — object storage for entry data
- `pkg/bio` — block I/O (`InitPageCache`, `NewChunkReader`, `NewChunkWriter`)
- `pkg/events` — publishes `action.entry.*` / `action.file.*` topics on `hyponet/eventbus`
- `pkg/types`
- External: `go.uber.org/zap`, `bluele/gcache`

## Design Notes

- Event flow: core publishes entry/file action events (via `publicEntryActionEvent`/`publicFileActionEvent` in utils.go) to the eventbus; consumed by `workflow/trigger.go` (CEL rule matching → `TriggerWorkflow`) and `pkg/dispatch` (event → `ScheduledTask` bridge for compaction, cleanup, reindexing).
- Caching: LFU cache keyed by `namespace+id` for entries; LRU cache keyed by `namespace+parentID+name` for child lookups. Cache invalidation hooks tie into bio (`InvalidCacheHook`).
- `FileSystem` is the composition root used by FUSE/WebDAV/REST/Friday — everything below it goes through `Core`.
- Smart groups (`dynamicGroup`) compute children on demand via metastore filters rather than stored children links.

## Testing

- `suite_test.go` — Ginkgo/Gomega suite (`TestCore`); `BeforeSuite` builds an in-memory metastore and calls `New`.
- `core_test.go`, `group_test.go`, `rawfile_test.go` — spec tests.
