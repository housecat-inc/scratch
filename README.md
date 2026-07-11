# Setup for Agent Computers

scratch is scaffolding for a new app built with agents. 

First bootstrap Claude Code or OpenAI Codex tools and subscriptions if needed.

Next build CRUD apps with "batteries included" for database, migrations, design system, UI / API / CLI, unit and headless browser integration tests.

Finally iterate with in-app developer tools like a DOM and web inspector agent chat and code review.

## Features

- Claude Code Remote Control setup
- Agent skills management
- Code review and comments
- Claude Desktop and Mobile integration

<p align="center">
  <img src="docs/screenshots/setup.png" width="130" alt="Setup"/>
  <img src="docs/screenshots/sessions.png" width="130" alt="Sessions"/>
  <img src="docs/screenshots/diff.png" width="130" alt="Diffs"/>
  <img src="docs/screenshots/comments.png" width="130" alt="Comments"/>
  <img src="docs/screenshots/files.png" width="130" alt="Files"/>
  <img src="docs/screenshots/claude-code-mobile.jpeg" width="130" alt="Claude Code mobile"/>
</p>

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/housecat-inc/scratch/main/install.sh | sh
```

## scratch

Web UI on `:8888` that walks through installing `claude`, removing interactive defaults, and connecting a subscription.

```sh
scratch --port 8888
```
