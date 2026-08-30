# pkg/friday

## Purpose

LLM-powered AI assistant on top of NanaFS. `Manager` lazily instantiates per-namespace `Friday` agents that expose the filesystem to an LLM through tools (read, list, search, filter, write, rename, mkdir, delete), persist chat sessions into NanaFS itself under `/.friday/sessions/`, and stream tool events over SSE.

## Key Files

| File | Responsibility |
|------|----------------|
| `friday.go` | `Friday` struct; `NewFriday`, `OpenSession`, `NewSession`, `Chat` (streaming LLM chat); `Tools()`, `resolveEntry` |
| `manager.go` | `Manager`; `NewFridayManager`; `GetFriday` (lazy per-namespace instantiation); `Factory` type |
| `tools.go` | LLM tool definitions: `file_read`, `file_list`, `file_stat`, `file_write`, `delete_file`, `rename_file`, `mkdir`, `full_text_search`, `filter_entries`; publishes SSE events via `events.PublishFridayEvent` |
| `session.go` | `SessionStore` interface; `fileSessionStore` (NanaFS-backed); session CRUD, `AppendMessage`, `GetMessages`; `SessionMeta`, `SessionMessage` |
| `workdir.go` | Per-session `workdirFS` for LLM working files |
| `prompt.go` | System prompts and agent descriptions |
| `research.go` | Research agent hook |
| `types.go` | Local types: `Event`, `fileInfo`, `filterResult` |
| `utils.go` | `parsePath`, `splitParentAndName`, `entryToStatInfo`, `formatSize`, `isNotFoundError` |

## Core Capabilities

**`Manager`:** `GetFriday(namespace string) (*Friday, error)` — caches per-namespace instances behind a `sync.RWMutex`; built from a `Factory func(namespace string) (*core.FileSystem, core.Core, metastore.Meta, indexer.Indexer, error)`.

**`Friday` methods:** `NewSession`, `OpenSession`, `Chat(ctx, sess, message) *api.Response` (streaming), `Namespace`, `GetStore`, `GetWorkdirPath`, `Tools`.

**`SessionStore` interface:** `CreateSession`, `GetSession`, `ListSessions`, `DeleteSession`, `RenameSession`, `AppendMessage`, `GetMessages`.

**LLM tools:**
- `file_read` — reads content; handles `.webarchive`/`.html` via webpage-packer, converts to markdown
- `file_list` — directory listing with name/size/mtime/isDir
- `file_stat` — entry metadata as JSON
- `file_write`, `delete_file`, `rename_file`, `mkdir` — filesystem mutations
- `full_text_search` — indexer query with highlighted snippets
- `filter_entries` — CEL-based entry filtering with pagination (kind, tags, timestamps, document properties)

## Upstream (Consumers)

- `cmd/apps/apis/rest/v1/friday.go` (REST + SSE endpoints, wired via `rest/common/depends.go`)

## Downstream (Dependencies)

- `pkg/core` (`core.Core`, `core.FileSystem`)
- `pkg/metastore` (`Meta` — entry filtering)
- `pkg/indexer` (`Indexer` — full-text search)
- `pkg/events` (`PublishFridayEvent` for SSE)
- External: `basenana/friday/core/*` (openai provider, tools, agents, session, planning, subagents), `JohannesKaufmann/html-to-markdown/v2`, `hyponet/webpage-packer`, `google/uuid`

## Design Notes

- Friday SSE events: every tool invocation in `tools.go` publishes via `events.PublishFridayEvent(namespace, sessionID, event)` on topic `friday.sessions.{session}.events`; the REST layer streams these to clients subscribed via `events.SubscribeFridayEvents`.
- Session persistence: `fileSessionStore` stores `meta.json` and `history.jsonl` under `/.friday/sessions/{sessionID}/` inside NanaFS itself, so sessions survive restarts.
- Per-namespace isolation: one `Friday` instance per namespace, each with its own workdir for LLM scratch files.
- The agent is built on `basenana/friday` core with a max loop of 20 tool iterations.

## Testing

- `session_test.go` — standard Go `testing` table-driven tests for message converters. No Ginkgo suite in this package.
