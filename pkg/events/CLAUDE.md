# pkg/events

## Purpose

Event system helpers for NanaFS. Publishes CloudEvents for filesystem actions (entry create/remove/mirror, file open/close/trunc) and Friday session events, all backed by the `hyponet/eventbus` pub/sub library. This package defines topic naming and event construction — the actual publishing/subscribing goes through eventbus.

## Key Files

| File | Responsibility |
|------|----------------|
| `topic.go` | Topic/action-type constants and `NamespacedTopic()` |
| `event.go` | `BuildEntryEvent()`, `BuildFileEvent()` constructing `*types.Event` |
| `friday.go` | `PublishFridayEvent()`, `SubscribeFridayEvents()` for Friday session pub/sub |

## Core Capabilities

**Topics and actions:**
- `TopicNamespaceEntry = "action.entry."`, `TopicNamespaceFile = "action.file."`
- Entry actions: `ActionTypeCreate`, `ActionTypeRemove`, `ActionTypeMirror`, `ActionTypeChangeParent`, `ActionTypeIndex`
- File actions: `ActionTypeTrunc`, `ActionTypeOpen`, `ActionTypeClose`, `ActionTypeCompact`

**Helpers:**
- `BuildEntryEvent(actionType, source, uri string, entry *types.Entry) *types.Event`
- `BuildFileEvent(actionType string, source string, entry *types.Entry) *types.Event`
- `NamespacedTopic(topicNamespace, actionType string) string` — e.g. `action.entry.create`

**Friday session events:**
- `PublishFridayEvent(namespace, session string, event any)` — publishes to `friday.sessions.{session}.events`
- `SubscribeFridayEvents(namespace, session string) (chan any, func())` — returns event channel + unsubscribe func

## Upstream (Consumers)

- `pkg/core` (core.go, utils.go, rawfile.go) — publishes entry/file action events
- `workflow/trigger.go`, `workflow/defaults.go` — subscribes to entry events for CEL rule matching
- `pkg/dispatch` (dispatcher.go, index_task.go, mainttask.go) — subscribes to create compact/cleanup/reindex tasks
- `pkg/friday/tools.go` — publishes tool invocation events
- `cmd/apps/apis/rest/v1` (base.go, friday.go) — SSE handlers subscribe to Friday events

## Downstream (Dependencies)

- `github.com/hyponet/eventbus` (external pub/sub)
- `github.com/basenana/nanafs/pkg/types`

## Design Notes

- Central event flow of the system: `pkg/core` publishes `action.entry.*` / `action.file.*` → consumed by `pkg/dispatch` (event → `ScheduledTask` bridge) and `workflow/trigger.go` (CEL rule matching → `TriggerWorkflow`).
- Friday tool events flow: `pkg/friday/tools.go` → `events.PublishFridayEvent` → REST SSE handlers streaming to the frontend.
- Events follow CloudEvents spec v1.0 with `RefType` of `"entry"` or `"file"`.
- Purely functional helpers — no interface abstraction.

## Testing

No test files in this package; behavior is covered indirectly by `workflow/` and `pkg/dispatch` suites.
