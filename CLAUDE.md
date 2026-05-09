# Agent

## Style

- Write self-commenting code, don't write comments
- Sort things lexigraphically, struct fields, db columns names, etc
- Write table driven tests

## Packages

- use github.com/cockroachdb/errors for errors
- use github.com/stretchr/testify test assertions with `a := assert.New(t)`, `r := require.New(t)`

## Pull Requests

PR must be formatted as release notes with max 80 char `title`, and list of customer-facing changes in `body`. Do not add `Generated with Claude Code` to the body.
