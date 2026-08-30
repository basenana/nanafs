# pkg/types

## Purpose

Shared type definitions for the entire NanaFS system. This package defines the core data structures for entries (files/directories), access control, events, filters, workflows, properties, kinds, users, namespaces, and pagination. It is the dependency-free foundation of the codebase — nearly every other package imports it.

## Key Files

| File | Responsibility |
|------|----------------|
| `entry.go` | `Entry`, `Child`, `ChunkSeg` structs; `NewEntry()`/`InitNewEntry()` constructors |
| `access.go` | `Access`, `AccessToken`; permission constants and `HasPerm`/`AddPerm`/`RemovePerm` |
| `event.go` | `Event` (CloudEvents spec), `EventData`, `ScheduledTask` with status constants, `Notification` |
| `filter.go` | `Filter` (CEL pattern), `JobFilter`, `EventFilter` |
| `workflow.go` | `Workflow`, `WorkflowTrigger`, `WorkflowNode`, `WorkflowJob`, `WorkflowJobNode`, `WorkflowTarget`; trigger and node type constants |
| `properties.go` | `Properties`, `AttrProperties`, `SymlinkProperties`, `GroupProperties`, `DocumentProperties`, `FridayProcessProperties`, `PropertyType` constants |
| `kind.go` | `Kind` type with all kind constants (`GroupKind`, `SmartGroupKind`, `TextKind`, `PdfDocKind`, ...); `IsGroup()`, `FileKind()` |
| `user.go` | `User`, `Namespace` |
| `pagination.go` | `Pagination` with context-based `GetPagination()`/`WithPagination()` helpers and `Limit()`/`Offset()`/`SortField()`/`SortOrder()` |
| `option.go` | Operation attribute structs: `EntryAttr`, `OpenAttr`, `DestroyEntryAttr`, `ChangeParentAttr`, `UpdateEntry`, `DeleteEntry` |
| `errors.go` | Sentinel errors: `ErrNotFound`, `ErrNameTooLong`, `ErrIsExist`, `ErrNotEmpty`, `ErrNoGroup`, `ErrIsGroup`, `ErrNoAccess`, `ErrNoPerm`, `ErrNoNamespace`, `ErrConflict`, `ErrUnsupported`, `ErrNotEnable` |
| `config.go` | `ConfigItem` |
| `index.go` | `IndexDocument` |

## Core Capabilities

- Pure data structures — no interfaces or I/O.
- Constructors: `NewEntry()`, `InitNewEntry()`, `NewPagination()`, `NewPaginationWithSort()`.
- Kind-based type checking via `IsGroup()` and `FileKind()`.
- Permission helpers on `Access` for ACL manipulation.
- Context-based pagination propagation.
- Sentinel errors used across all layers for consistent error semantics.

## Upstream (Consumers)

Imported by ~100+ files across the whole system: `pkg/core`, `pkg/metastore`, `pkg/storage`, `pkg/dispatch`, `pkg/cel`, `pkg/indexer`, `pkg/events`, `pkg/bio`, `pkg/friday`, `pkg/notify`, `pkg/auth`, `workflow/`, and all API servers in `cmd/apps/apis/` (REST, WebDAV, FUSE).

## Downstream (Dependencies)

- `github.com/basenana/nanafs/utils` (only for `utils.GenerateNewID()` in `entry.go`)
- Standard library otherwise.

## Design Notes

- Keep this package free of logic and internal dependencies — it sits at the bottom of the dependency graph alongside `config/` and `utils/`.
- Sentinel errors in `errors.go` are the canonical way lower layers signal not-found / conflict / permission failures (e.g. metastore maps GORM errors onto them).
- `ScheduledTask` status constants drive the dispatch polling loop; workflow status constants live in `workflow/` (re-exported from go-flow), not here.

## Testing

No test files — types are exercised indirectly through consumer package suites (Ginkgo/Gomega elsewhere).
