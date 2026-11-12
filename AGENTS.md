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

## Tests

Stdlib table-driven tests whose names read like a spec. `docs/tests.txt` is the committed tree of every test name, grouped by file; review its diff to see behavior added or removed. The `testspec` analyzer enforces:

- `Test<Subject>` names the function or feature under test (`TestAddComment`, `TestGreetWorkflow`); a file may hold several
- At most three levels: `TestAddComment` › optional context ("when the branch is missing") › case ("rejects an empty body")
- A group named like a subject (`MergeDefaults`, `foldParts`) must be its own Test instead
- Case and context names are literal lowercase sentences — no `_` or `/` — via `t.Run("...")` or a literal table's `name` field
- `t.Run` only inside a `_test.go` file's Test function — never in helpers or non-test code, so no test runner frameworks
- `github.com/stretchr/testify` with `a := assert.New(t)` and `r := require.New(t)` inside each case

```go
func TestAddComment(t *testing.T) {
	tests := []struct {
		body string
		err  string
		name string
	}{
		{name: "rejects an empty body", err: "body required"},
		{name: "stores the line and side", body: "looks off"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			...
		})
	}
}
```

Run `go run ./cmd/checks ./pkg/<dir>` to check one package.

## Build & codegen

Build and dev tasks run through [mage](https://magefile.org); Go tools are invoked with `go tool`:

- `mage build` — build `cmd/scratch`
- `mage check` — `go vet` plus custom analyzers in `cmd/checks`, and verify `docs/tests.txt` is current
- `mage cover` — write per-package coverage to `docs/coverage.txt`
- `mage dev` — hot-reload dev servers via air
- `mage generate` — run all code generation (`go generate ./...`)
- `mage tests` — write the test spec tree to `docs/tests.txt`
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
