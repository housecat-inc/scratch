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
