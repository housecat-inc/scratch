# scratch

Setup for Agent Computers

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/housecat-inc/scratch/main/install.sh | sh
```

## claude-control

Web UI on `:8888` that walks through installing `claude`, removing interactive defaults, and connecting a subscription.

```sh
claude-control --port 8888
```

Drive the whole thing from your phone over [Claude Code on iOS](docs/screenshots/claude-code-mobile.jpeg).

| Setup | Sessions | Diffs |
| --- | --- | --- |
| ![Setup](docs/screenshots/setup.png) | ![Sessions](docs/screenshots/sessions.png) | ![Diffs](docs/screenshots/diff.png) |
| **Comments** | **Files** | **Claude Code** |
| ![Comments](docs/screenshots/comments.png) | ![Files](docs/screenshots/files.png) | ![Claude Code mobile](docs/screenshots/claude-code-mobile.jpeg) |
