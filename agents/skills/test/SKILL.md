---
description: Unit, integration, and end-to-end browser testing
name: test
---

## Test-Driven Workflow

Drive features from tests. For each behavior:

1. Write a failing test that asserts the desired outcome.
2. Implement the minimum code to make it pass.
3. Refactor while the test stays green.

For end-to-end coverage, exercise the running app with rodney rather than mocking the browser.

## Interactive Browser Testing

Install rodney once:

```bash
go install github.com/simonw/rodney@latest
```

Drive a running app from the shell. Each command is short-lived and talks to the same persistent headless Chrome, so steps compose naturally as you iterate:

```bash
rodney start
rodney open http://localhost:8080
rodney click "button#login"
rodney wait ".dashboard"
rodney assert 'document.title' 'Dashboard'
rodney screenshot dashboard.png
rodney stop
```
