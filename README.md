# Setup for Agent Computers

<p align="center">
  <img src="docs/screenshots/setup.png" width="130" alt="Setup"/>
  <img src="docs/screenshots/sessions.png" width="130" alt="Sessions"/>
  <img src="docs/screenshots/diff.png" width="130" alt="Diffs"/>
  <img src="docs/screenshots/comments.png" width="130" alt="Comments"/>
  <img src="docs/screenshots/files.png" width="130" alt="Files"/>
  <img src="docs/screenshots/claude-code-mobile.jpeg" width="130" alt="Claude Code mobile"/>
</p>

## Features

- Claude Code Remote Control setup
- Agent skills management
- Code review and comments
- Claude Desktop and Mobile integration

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/housecat-inc/scratch/main/install.sh | sh
```

## claude-control

Web UI on `:8888` that walks through installing `claude`, removing interactive defaults, and connecting a subscription.

```sh
claude-control --port 8888
```
