# pkg/dispatch

## Purpose

Scheduled-task engine for NanaFS. Runs a 5-minute polling loop that picks up due `ScheduledTask` records and hands them to registered executors, and bridges filesystem events (compact, remove, index, change-parent) into scheduled tasks for maintenance: chunk compaction, orphan cleanup, reindexing, document URI updates, and finished-job cleanup.

## Key Files

| File | Responsibility |
|------|----------------|
| `dispatcher.go` | `Dispatcher`; `Run` with 5-minute `taskExecutionInterval` polling loop; `findRunnableTasks`, `dispatch`, `runRoutineTask`; `registerExecutor`/`registerRoutineTask`; `Init` factory |
| `executor.go` | `executor` interface (`execute(ctx, task) error`); `taskExecutor` stub |
| `mainttask.go` | `maintainExecutor`, `compactExecutor`, `entryCleanExecutor`; subscribes to `TopicNamespaceFile`/`ActionTypeCompact` and `TopicNamespaceEntry`/`ActionTypeRemove`; orphan scanner (every 6h) |
| `index_task.go` | `indexExecutor` (subscribes to `ActionTypeIndex`), `documentURIUpdateExecutor` (subscribes to `ActionTypeChangeParent`); triggers the `docloader` workflow for reindexing |
| `workflow.go` | `workflowExecutor`; `cleanUpFinishJobs` routine (every 6h) deleting jobs older than 24h (succeed) / 7d (failed), capped at 100 per workflow |

## Core Capabilities

**`executor` interface:** `execute(ctx context.Context, task *types.ScheduledTask) error`.

**Executors:**
- `compactExecutor` — chunk data compaction (bio `CompactChunksData`)
- `entryCleanExecutor` — orphan entry cleanup and data removal
- `indexExecutor` — re-index entries whose content mismatches, via the built-in `docloader` workflow
- `documentURIUpdateExecutor` — update indexed document URIs after moves/renames
- `workflowExecutor` — workflow-driven tasks

**Routine tasks:** orphan scan (6h), finished-job cleanup (6h).

## Upstream (Consumers)

- `cmd/apps/apis/rest/common/depends.go` (REST API wire-up)

## Downstream (Dependencies)

- `pkg/core` (`core.Core`)
- `pkg/metastore` (`Meta`, `ScheduledTaskRecorder`)
- `pkg/indexer`, `pkg/notify`
- `workflow` (`workflow.Workflow` — triggers reindex workflows)
- `pkg/events` (subscribes to eventbus topics)
- External: `go.uber.org/zap`

## Design Notes

- Event → task bridging: executors subscribe to `action.file.compact`, `action.entry.remove`, `action.entry.index`, and `action.entry.change_parent` events published by `pkg/core` and persist `ScheduledTask` records; the polling loop later executes them. Tasks can request retry by returning `ErrNeedRetry`.
- Polling interval: 5 minutes by default, overridable via `SCHED_TASK_EXEC_INTERVAL_SECONDS`.
- The dispatcher marks stale executing tasks as failed (timeout) during `findRunnableTasks`.
- Task statuses come from `pkg/types` (`ScheduledTaskWait` → `ScheduledTaskExecuting` → `ScheduledTaskSucceed`/`ScheduledTaskFailed`).
- Indexing flow: entry events → `indexExecutor` → `workflow.TriggerWorkflow` → jobrun `namespacedFS` → `indexer.Index`.

## Testing

- `suite_test.go` — Ginkgo/Gomega suite; each test builds fresh in-memory meta, core, notify, and `workflow.New`.
- `dispatcher_test.go`, `mainttask_test.go`, `workflow_test.go`.
