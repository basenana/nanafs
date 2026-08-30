# pkg/cel

## Purpose

CEL (Common Expression Language) evaluation for NanaFS. Provides two paths: runtime evaluation of CEL patterns against in-memory entries (workflow triggers, condition nodes), and CEL→SQL translation support used by `pkg/metastore/filters/` to push filters down to the database (PostgreSQL/SQLite).

## Key Files

| File | Responsibility |
|------|----------------|
| `matcher.go` | `EntryMatch()`, `EvalCEL()`, `EvalCELWithVars()`, `BuildCELFilterFromMatch()`, `globToSQLLike()` |
| `filter.go` | `Parse()` — builds the CEL env with the custom `now()` function and column variables |
| `expr.go` | CEL AST traversal helpers: `GetConstValue()`, `GetIdentExprName()`, `GetFunctionValue()`, `GetExprValue()` |
| `templates.go` | `SQLTemplate`/`SQLTemplates` map, `GetSQL()`, `GetParameterPlaceholder()`, `GetParameterValue()`, `FormatPlaceholders()` for cross-database SQL generation |
| `columns.go` | `identify2Columns` map (CEL identifier → DB column), `CheckValueType()`, exported column classification lists |

## Core Capabilities

**Runtime evaluation:**
- `EntryMatch(ctx, entry *types.Entry, match *types.WorkflowLocalFileWatch) (bool, error)` — match an entry against a workflow file-watch spec
- `EvalCEL(ctx, entry *types.Entry, pattern string) (bool, error)` — evaluate a CEL pattern against entry fields
- `EvalCELWithVars(vars map[string]any, pattern string) (bool, error)` — evaluate with a custom variable map

**Parsing and AST helpers:**
- `Parse(filter string, opts ...cel.EnvOption) (*exprv1.ParsedExpr, error)`
- `GetConstValue`, `GetIdentExprName`, `GetFunctionValue` (handles `now()`, arithmetic ops), `GetExprValue`

**SQL generation (dialect-aware):**
- `GetSQL(templateName string, dbType TemplateDBType, identifier string, args ...any) string`
- `GetParameterPlaceholder`, `GetParameterValue`, `FormatPlaceholders`
- `TemplateDBType` constants: `SQLiteTemplate`, `MySQLTemplate`, `PostgreSQLTemplate`

**Column utilities:** `CheckValueType(identifier, value)`, `ColumnsBool`, `ColumnsSizeable`, `ColumnsComparable`, `ColumnsList`, `ColumnsTime`.

**Entry variable bindings:** `id` (int), `kind` (string), `is_group` (bool), `size` (int), `name` (string), `created_at`/`changed_at`/`modified_at`/`access_at` (unix timestamps), plus a built-in `now()` function.

## Upstream (Consumers)

- `workflow/trigger.go` — `cel.EntryMatch()` for rule-based workflow triggering
- `workflow/jobrun/executor.go` — `cel.EvalCELWithVars()` for condition nodes
- `pkg/metastore/filters/` (convert.go, sqlite.go, posgres.go) — `cel.Parse()` + `GetSQL()` + AST helpers for CEL→SQL

## Downstream (Dependencies)

- `github.com/basenana/nanafs/pkg/types`
- External: `github.com/google/cel-go`, `google.golang.org/genproto/.../expr/v1alpha1` (CEL AST)

## Design Notes

- Dual-path design: runtime eval (`EvalCEL*`) vs. SQL translation (`Parse` + templates). Both share the same variable/identifier vocabulary defined in `columns.go`.
- `identify2Columns` maps CEL identifiers to table/column paths (with optional JSON key extraction for JSONB columns).
- `SQLTemplates` are keyed by operation (e.g. `content_compare`, `json_content_like`, `boolean_check`, `timestamp_field`, `json_array_length`) with `{table}`/`{column}`/`{jsonkey}` placeholders per dialect.
- `defaultCELAttributes` is built at `init()` from `identify2Columns`.

## Testing

- `matcher_test.go` — standard Go `testing` table-driven tests for `EntryMatch`.
