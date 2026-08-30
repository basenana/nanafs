# pkg/notify

## Purpose

In-app notification persistence for NanaFS. A thin facade over `metastore.NotificationRecorder` that records info/warn notifications and marks them read. This package handles storage only — there are no delivery channels (email, push, etc.).

## Key Files

| File | Responsibility |
|------|----------------|
| `notify.go` | `Notify` struct with `RecordInfo()`, `RecordWarn()`, `MarkRead()`, `ListNotifications()`, `NewNotify()` |
| `events.go` | Placeholder (empty) |

## Core Capabilities

**`Notify` methods:**
- `ListNotifications(ctx, namespace string) ([]types.Notification, error)`
- `RecordInfo(ctx, namespace, title, message, source string) error`
- `RecordWarn(ctx, namespace, title, message, source string) error`
- `MarkRead(ctx, namespace, nid string) error`

Constructor: `NewNotify(s metastore.NotificationRecorder) *Notify`.

The backing `metastore.NotificationRecorder` interface: `ListNotifications`, `RecordNotification`, `UpdateNotificationStatus`.

## Upstream (Consumers)

- `cmd/apps/apis/rest/v1/base.go`, `cmd/apps/apis/rest/common/depends.go` (REST endpoints)
- `workflow/workflow.go`, `workflow/jobrun/controller.go` (job failure notifications)
- `pkg/dispatch/dispatcher.go` (task failure notifications)

## Downstream (Dependencies)

- `github.com/basenana/nanafs/pkg/metastore` (`NotificationRecorder` interface)
- `github.com/basenana/nanafs/pkg/types` (`Notification`)
- `github.com/basenana/nanafs/utils` (`MustRandString` for IDs)

## Design Notes

- Notification IDs are timestamp-based: `{unixnano}-{info|warn}-{rand8}`.
- Direct DB persistence via metastore — no event bus involvement.
- All operations are namespace-scoped.

## Testing

No test files in this package; covered indirectly by `workflow/` and `pkg/dispatch` suites.
