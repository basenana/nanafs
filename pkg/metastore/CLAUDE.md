# pkg/metastore

## Purpose

Central metadata store for NanaFS. Provides PostgreSQL and SQLite persistence (via GORM) for entries, children links, properties, chunk segments, scheduled tasks, workflows, workflow jobs, notifications, users, and namespaces. Also implements CEL-based entry filtering translated to SQL WHERE clauses, and full-text document search (SQLite FTS5 / PostgreSQL tsvector).

## Key Files

| File | Responsibility |
|------|----------------|
| `interface.go` | `Meta` interface composed of sub-interfaces: `SysConfig`, `EntryStore`, `ScheduledTaskRecorder`, `NotificationRecorder`, `DocumentSearcher`, `UserStore`, `NamespaceStore`; `EntryIterator` |
| `meta.go` | Factory `NewMetaStorage(metaType string, meta config.Meta) (Meta, error)` dispatching to `MemoryMeta`/`SqliteMeta`/`PostgresMeta` |
| `sql.go` | `sqlMetaStore` (embeds `*gorm.DB`); implementations of all `Meta` methods |
| `filter.go` | `FilterEntries` — CEL pattern → SQL via `filters.Convert`, table joins via `filters.Join`, pagination |
| `iterator.go` | `EntryIterator` (`HasNext()`, `Next()`) and `simpleIterator` |
| `instrumental.go` | Prometheus latency/error metrics, `DisableMetrics()` |

**Subpackage `pkg/metastore/db/`:**

| File | Responsibility |
|------|----------------|
| `model.go` | GORM models (`Entry`, `Children`, `EntryProperty`, `EntryChunk`, `ScheduledTask`, `Workflow`, `WorkflowJob`, `User`, `Namespace`, ...) with `From*`/`To*` converters |
| `migrate.go` | `gormigrate` migrations (IDs `2025062100`–`2026021600`), FTS `documents` table creation, `Migrate(db)` |
| `json.go` | Custom GORM `JSON` type (`JSON`/`JSONB` per dialect) |
| `utils.go` | `SqlError2Error` (maps GORM errors to `types.ErrNotFound`/`ErrIsExist`), zap GORM logger, unix permission mapping |

**Subpackage `pkg/metastore/filters/`:** `convert.go` (`Convert`/`Join` dispatching by dialect), `sqlite.go` and `posgres.go` CEL→SQL converters (AND/OR/NOT, comparisons, `in`, `contains`, `size()`, timestamps, bools, JSON lists).

**Subpackage `pkg/metastore/search/`:** `search.go` (dialect dispatcher), `sqlite.go` (FTS5 `MATCH` with `highlight()`), `postgres.go` (tsquery, `ts_headline`, `ts_rank`, weighted tsvector).

## Core Capabilities

**`Meta` interface (grouped by sub-interface):**

- **`SysConfig`:** `SystemInfo`, `GetConfigValue`/`SetConfigValue`/`ListConfigValues`/`DeleteConfigValue`
- **`EntryStore`:** entry CRUD (`GetEntry`, `CreateEntry`, `UpdateEntry`, `RemoveEntry`, `DeleteRemovedEntry`), relationships (`FindEntry`, `GetChild`, `ListChildren`, `ListNamespaceGroups`, `ListParents`, `MirrorEntry`, `ChangeEntryParent`), `FilterEntries` (CEL→SQL), lifecycle (`Open`, `Flush`, `ScanOrphanEntries`), properties (`GetEntryProperties`, `UpdateEntryProperties`), segments (`NextSegmentID`, `ListSegments`, `AppendSegments`, `DeleteSegment`)
- **`ScheduledTaskRecorder`:** `ListTask`/`SaveTask`/`DeleteFinishedTask`, workflow CRUD (`GetWorkflow`, `ListWorkflows`, `SaveWorkflow`, `DeleteWorkflow`, `ListAllNamespaceWorkflows`), job management (`GetWorkflowJob`, `ListWorkflowJobs`, `SaveWorkflowJob`, `DeleteWorkflowJobs`, `ListAllNamespaceWorkflowJobs`), queue ops (`GetPendingNamespaces`, `ClaimNextJob`), job data (`LoadJobData`, `SaveJobData`)
- **`NotificationRecorder`:** `ListNotifications`, `RecordNotification`, `UpdateNotificationStatus`
- **`DocumentSearcher`:** `IndexDocument` (with pluggable tokenizer), `QueryDocuments`, `DeleteDocument`, `UpdateDocumentURI`
- **`UserStore`:** `CreateUser`, `GetUserByGoogleID`/`GetUserByEmail`/`GetUserByID`/`GetUserByNamespace`, `UpdateUser`
- **`NamespaceStore`:** `CreateNamespace`, `GetNamespace`, `ListNamespaces`, `DeleteNamespace`, `NamespaceExists`

## Upstream (Consumers)

- `pkg/core` — entry/segment persistence
- `pkg/auth` — users and namespaces
- `pkg/dispatch` — scheduled tasks, workflows
- `pkg/notify` — notifications
- `pkg/indexer` — document search delegation
- `pkg/friday` — metadata access
- `workflow/` and `workflow/jobrun/` — workflows, jobs, queues
- Test suites across `cmd/apps/apis/*`

## Downstream (Dependencies)

- `github.com/basenana/nanafs/pkg/cel` (parsing + SQL templates for filters)
- `github.com/basenana/nanafs/pkg/types`
- External: `gorm.io/gorm` (+ postgres driver, `glebarez/sqlite`), `go-gormigrate/gormigrate/v2`, `prometheus/client_golang`, `google/uuid`, `go.uber.org/zap`

## Design Notes

- CEL→SQL filters: `filters/` parses the CEL AST (`cel.Parse`) and emits dialect-specific WHERE clauses — SQLite uses `?` placeholders and backtick identifiers; PostgreSQL uses `$N` positional params and JSONB extraction.
- Full-text search: SQLite FTS5 virtual table (`unicode61` tokenizer) vs. PostgreSQL `TSVECTOR` + GIN index with `ts_headline`/`ts_rank`. The `tokenizer func(string) []string` parameter allows pluggable tokenization (jieba for Chinese — see `pkg/indexer`).
- Schema migrations are ordered gormigrate IDs; FTS tables added in later migrations.
- A global `bigLock` mutex serializes writes (SQLite single-connection constraint).
- GORM errors are normalized to `types` sentinel errors via `SqlError2Error`.

## Testing

- `suite_test.go` — Ginkgo/Gomega suite (`TestMetaStore`).
- `sql_test.go`, `filter_test.go`, `search_test.go` — standard Go tests over in-memory SQLite.
- `filters/sqlite_test.go`, `filters/posgres_test.go` — CEL→SQL conversion tests.
