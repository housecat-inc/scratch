---
description: exe.dev / Shelley VM specifics — internal git mirror, gh CLI host, running scratch as a service
name: exe-dev
---

## exe.dev / Shelley VMs

This repo is mirrored on the internal exe.dev host. On an exe.dev VM the environment sets `EXEUNTU=1`.

### Git and gh

Clone over HTTPS from the internal mirror:

```bash
git clone https://housecat-inc-scratch.int.exe.xyz/housecat-inc/scratch.git
```

Point the `gh` CLI at the internal host:

```bash
export GH_HOST=housecat-inc-scratch.int.exe.xyz
gh pr list -R housecat-inc/scratch
```

### Run scratch as a service

Deploy under systemd (builds, installs the unit, restarts, shows status):

```bash
mage deploy
```

Or run it directly on port 8888:

```bash
systemd-run --user --unit=scratch --collect ~/.local/bin/scratch --port 8888
```

Then continue at the private URL `https://<vm-hostname>.exe.xyz:8888/`.
