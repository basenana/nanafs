# pkg/indexer

## Purpose

Full-text indexing facade for NanaFS. Wraps `metastore.Meta`'s `DocumentSearcher` behind a uniform `Indexer` interface with pluggable tokenizer strategies (jieba for Chinese, whitespace for English), plus cascading delete/update of child documents when parent entries change.

## Key Files

| File | Responsibility |
|------|----------------|
| `interface.go` | `Indexer` interface: `Index`, `QueryLanguage`, `Delete`, `UpdateURI`, `DeleteChildren`, `UpdateChildrenURI` |
| `metadb.go` | `metaDB` implementation wrapping `metastore.Meta` + `Tokenizer`; `NewMetaDB(meta, tokenizer)`; recursive child traversal |
| `tokenizer.go` | `Tokenizer` interface, `jiebaTokenizer` (hyponet/jiebago), `spaceTokenizer`, constructors |
| `mem.go` | `memIndexer` no-op stub, `NewMem()` (for tests) |

## Core Capabilities

**`Indexer` interface:**
- `Index(ctx, namespace string, doc *types.IndexDocument) error`
- `QueryLanguage(ctx, namespace, query string) ([]*types.IndexDocument, error)`
- `Delete(ctx, namespace string, id int64) error`
- `UpdateURI(ctx, namespace string, id int64, uri string) error`
- `DeleteChildren(ctx, namespace string, parentID int64) error`
- `UpdateChildrenURI(ctx, namespace string, parentID int64, newParentURI string) error`

**`Tokenizer` interface:** `Tokenize(content string) []string`, with `NewJiebaTokenizer(dictPath)` and `NewSpaceTokenizer()`.

## Upstream (Consumers)

- `pkg/dispatch` (index_task.go, mainttask.go, dispatcher.go) — index/URI-update task executors
- `pkg/friday` (friday.go, manager.go) — `full_text_search` LLM tool
- `workflow/workflow.go`, `workflow/jobrun/controller.go` — document loading workflows
- `cmd/apps/apis/rest/v1` (base.go, common/depends.go) — search endpoints

## Downstream (Dependencies)

- `github.com/basenana/nanafs/pkg/metastore` (`Meta` / `DocumentSearcher`)
- `github.com/basenana/nanafs/pkg/types` (`IndexDocument`)
- External: `github.com/hyponet/jiebago` (Chinese segmentation), `go.uber.org/zap`

## Design Notes

- Strategy pattern for tokenization: the tokenizer choice determines how `IndexDocument` splits content before it reaches the dialect-specific FTS backend in `pkg/metastore/search/` (SQLite FTS5 vs. PostgreSQL tsvector).
- Indexing flow: entry events → `pkg/dispatch` `indexExecutor` → `workflow.TriggerWorkflow` (`docloader`) → `workflow/jobrun` `namespacedFS` → `indexer.Index`.
- `DeleteChildren`/`UpdateChildrenURI` recursively walk child entries so renames/moves keep document URIs consistent.

## Testing

- `suite_test.go` — Ginkgo/Gomega suite (entry function is `TestDispatch`); each test builds a SQLite metastore via `metastore.NewMetaStorage`.
- `metadb_test.go` — standard Go tests over SQLite.
