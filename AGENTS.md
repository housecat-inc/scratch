# AGENTS

You are an experienced software engineer and architect. Communicate with brevity. Be persistent and creative.

Subdirectory `AGENTS.md` / `CLAUDE.md` files take precedence within their subtree; user instructions override everything.

## Stack

- Idiomatic Go
- HTML via templ + templui components
- HTMX for interactivity
- Tailwind CSS
- Vanilla JS sparingly

Static assets (JS, CSS, icons) live in `pkg/ui/static` and are served from an `//go:embed` filesystem.

## Workflow implementation

- Implement application jobs and their business logic in idiomatic Go inside Scratch. Do not add Python or shell job implementations, or wrap an entire external script in a DBOS step, unless the user explicitly requests that approach. Existing script wrappers are migration debt, not examples to copy.
- Use DBOS for durable execution, not just scheduling. Split multi-operation jobs into meaningful steps at recovery boundaries: fetching inputs, transforming or validating data, and publishing or persisting results. Use per-item steps or child workflows when items need independent recovery. A single step is appropriate only for a genuinely single operation.
- Keep workflow orchestration deterministic. Put network calls, database operations, model calls, and other side effects or nondeterministic work inside DBOS steps using the supported durable APIs. Persist serializable step outputs so recovery can reuse completed work without fetching or computing it again. Do not rely on process memory or temporary files as the only handoff between steps.
- Configure bounded retries, backoff, and timeouts appropriate to each operation. Distinguish transient failures from permanent errors. Make writes, notifications, and external mutations safe to retry using stable idempotency keys, uniqueness constraints, or reconciliation. A DBOS checkpoint alone does not guarantee an external side effect happens exactly once; handle a crash after the effect but before its completion is recorded.
- Show actual executable steps and their recorded status, results, and errors in Scratch's Workflows UI. Descriptive labels for operations inside one opaque step do not count as durable steps.
- When refactoring existing workflows, preserve matching schedules, timezone, pause state, run history, and user-visible behavior. Account for in-flight runs and replay compatibility when changing step order or outputs. Retire obsolete script execution paths and duplicate schedules once migration is verified.
- Validate recovery as well as the happy path: fail after a completed step, resume the run, and verify that completed work is reused and side effects are not duplicated. Use isolated fixtures or test services for notifications and external writes unless live execution is explicitly authorized.

## Style

- Write self-commenting code; do not write comments
- Sort things lexicographically — struct fields, DB columns, map keys, switch cases
- Errors: `github.com/cockroachdb/errors`
- Tests: table-driven, `github.com/stretchr/testify` with `a := assert.New(t)` and `r := require.New(t)`

## Build & codegen

Build and dev tasks run through [mage](https://magefile.org); Go tools are invoked with `go tool`:

- `mage build` — build `cmd/scratch`
- `mage dev` — hot-reload dev servers via air
- `mage generate` — run all code generation (`go generate ./...`)
- `go test ./...` — run tests

Never hand-edit generated code. `mage build` regenerates before compiling; edit the source instead:

- `*.templ` → templ (`pkg/ui/*_templ.go`) — gitignored
- `pkg/db/queries/*.sql`, `pkg/db/schema/*.sql` → sqlc (`pkg/db/internal/sqlite`) — gitignored
- API route definitions → OpenAPI (`docs/openapi.json`) — committed as the API contract

## Activate changes

After implementing Scratch changes, run `go tool mage build` to stage the updated executable. Leave the running app alive so chats can finish. The sidebar shows "Pending changes, restart" when the installed executable differs from the running one; the user chooses when to apply it. Do not run `mage deploy`, restart Scratch, or schedule a deferred restart unless explicitly requested. Report that the build is pending activation. Scratch's systemd unit must set `SCRATCH_SERVICE` to its user service name for the restart button to be available.

## Database

- SQLite via sqlc — queries in `pkg/db/queries`, schema in `pkg/db/schema`
- Migrations are goose files named `NNNNN_description.sql`, numbered sequentially
- Timestamp columns map to `ts.Timestamp` from `pkg/ts` (RFC3339)

## Version control

- Start every change on a new branch off `main`; never commit to `main` directly
- Commit after every turn so progress is reviewable — small commits are fine

## Pull requests

Validated by the `lint-pr` CI job. Format as customer-facing release notes:

- Title under 80 characters
- Body is a markdown list — every non-empty line starts with `-`, `*`, `+`, or `N.`
- Summarize user-facing capability, not implementation; no low-level code details
- No "Generated with Claude Code" footer

## Review comments

Review feedback lives in `$HOME/.config/scratch/scratch.db` (SQLite). Find open comments on the current branch:

```bash
BRANCH=$(git rev-parse --abbrev-ref HEAD)
sqlite3 -separator $'\t' ~/.config/scratch/scratch.db \
  "SELECT id, path, line, side, body FROM comments WHERE slug = 'housecat-inc/scratch' AND branch = '$BRANCH' AND resolved = 0 ORDER BY created_at"
```

After addressing a comment, mark it resolved with a one-line reason:

```bash
sqlite3 ~/.config/scratch/scratch.db \
  "UPDATE comments SET resolved = 1, resolved_body = 'short reason here', updated_at = CURRENT_TIMESTAMP WHERE id = '<id>'"
```
