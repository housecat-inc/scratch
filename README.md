# scratch

Setup for Agent Computers

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/housecat-inc/scratch/main/install.sh | sh
```

Override the binary or destination:

```sh
INSTALL_DIR=/usr/local/bin BIN=claude-control \
  curl -fsSL https://raw.githubusercontent.com/housecat-inc/scratch/main/install.sh | sh
```

## claude-control

Web UI on `:8888` that walks through installing claude, unleashing it
(non-interactive settings defaults), and connecting a Claude subscription.

```sh
claude-control --port 8888
```
