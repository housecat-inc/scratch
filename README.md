# scratch

Setup for Agent Computers

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/housecat-inc/scratch/main/install.sh | sh
```

Override the binary or destination:

```sh
INSTALL_DIR=/usr/local/bin BIN=claude-remote \
  curl -fsSL https://raw.githubusercontent.com/housecat-inc/scratch/main/install.sh | sh
```

## claude-remote

Web UI on `:8888` that walks through installing the claude CLI, writing
non-interactive settings defaults, and completing OAuth login.

```sh
claude-remote --port 8888
```
