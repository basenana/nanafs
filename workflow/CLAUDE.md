# workflow

## Purpose

File-centric workflow engine for NanaFS. Manages workflow definitions and job lifecycle, activates workflows from triggers (file-watch events, interval timers, RSS polling), and executes jobs through the `jobrun` controller/scheduler/executor stack built on `basenana/go-flow` and `basenana/plugin`.

## Key Files

| File | Responsibility |
|------|----------------|
| `workflow.go` | `Workflow` interface + `manager` implementation; workflow/job CRUD; `TriggerWorkflow`, `PauseWorkflowJob`, `ResumeWorkflowJob`, `CancelWorkflowJob`, `ListPlugins` |
| `trigger.go` | `triggers` struct; subscribes to `TopicNamespaceEntry`/`ActionTypeCreate`; CEL matching via `cel.EntryMatch`; RSS workflow trigger; interval timer management |
| `defaults.go` | Built-in workflows (`NamespaceDefaultsWorkflow`): RSS collect and document load; `BuildInWorkflowID` |
| `utils.go` | `assembleWorkflowJob`, `initWorkflow`, `validateWorkflowSpec`, `isHideEntry` |

**Subpackage `workflow/jobrun/`:**

| File | Responsibility |
|------|----------------|
| `controller.go` | `Controller`; job recovery on startup; `flow.Observer` status persistence; `PauseJob`/`ResumeJob`/`CancelJob`; `runner` wrapping `flow.Runner` |
| `scheduler.go` | `Scheduler` with worker pool; 5-second polling loop (`defaultInterval`); claims jobs via `ClaimNextJob` |
| `executor.go` | `defaultExecutor` implementing `flow.Executor`; `Setup`/`Exec`/`Teardown`; dispatches to plugins or handles condition/switch/matrix nodes |
| `adaptor.go` | `Task`/`coordinator` flow adaptors; `workflowJob2Flow`; `namespacedStore` and `namespacedFS` plugin API adaptors |
| `results.go` | `JSONFileResults` — file-based workflow step results |
| `workdir.go` | Per-job workdir initialization and cleanup |
| `utils.go` | Param/matrix rendering and validation |

## Core Capabilities

**`Workflow` interface:**
- Workflow CRUD: `ListWorkflows`, `GetWorkflow`, `CreateWorkflow`, `UpdateWorkflow`, `DeleteWorkflow`
- Jobs: `ListJobs`, `GetJob`, `TriggerWorkflow`, `PauseWorkflowJob`, `ResumeWorkflowJob`, `CancelWorkflowJob`
- Plugins: `ListPlugins`
- Lifecycle: `Start`

**`JobAttr`:** `Reason`, `Queue`, `Parameters`, `Timeout`.

Job status constants re-exported from go-flow: `InitializingStatus`, `RunningStatus`, `PausingStatus`, `PausedStatus`, `SucceedStatus`, `FailedStatus`, `ErrorStatus`, `CanceledStatus`.

**Node types:** `condition` (CEL via `EvalCELWithVars`), `switch` (field-based routing), `matrix` (iteration over arrays), plus plugin-executed process nodes.

## Upstream (Consumers)

- `cmd/apps/root.go` (serve command wiring)
- `cmd/apps/apis/rest/common/depends.go`, `cmd/apps/apis/rest/v1` (base.go, workflows.go)
- `pkg/dispatch` (index_task.go triggers the built-in `docloader` workflow)

## Downstream (Dependencies)

- `pkg/core` (`core.Core` — job file access via `namespacedFS`)
- `pkg/metastore` (workflows, jobs, queues, job data)
- `pkg/indexer`, `pkg/notify`, `pkg/cel`, `pkg/events`
- External: `basenana/go-flow`, `basenana/plugin`, `hyponet/eventbus`, `google/cel-go`, `go.uber.org/zap`

## Design Notes

- Trigger flow: `trigger.go` subscribes to `TopicNamespaceEntry` for `ActionTypeCreate`, filters workflows whose `LocalFileWatch` matches, evaluates `cel.EntryMatch`, then calls `TriggerWorkflow`.
- Job execution: `Scheduler` polls every 5s, claims jobs per namespace queue (`ClaimNextJob`), converts `WorkflowJob` → `flow.Flow` (`workflowJob2Flow`), and runs it; `Controller` observes flow status and persists it.
- Indexing integration: entry events → dispatch `indexExecutor` → `TriggerWorkflow` (`docloader`) → `namespacedFS` → `indexer.Index`.
- Built-in workflows ship per namespace: `rss` (feed polling with `docloader`+`save` nodes) and `docloader` (file indexing). RSS entries older than 30 days are archived into year subdirectories.

## Testing

- `workflow/suite_test.go` — Ginkgo/Gomega suite with in-memory core/metastore/indexer and a test `delay` plugin; plus `trigger_test.go`, `workflow_test.go`.
- `workflow/jobrun/suite_test.go` — Ginkgo/Gomega; plus `controller_test.go`, `executor_test.go`, `scheduler_test.go`, `results_test.go`, `adaptor_test.go`, `utils_test.go`, `workdir_test.go`.
