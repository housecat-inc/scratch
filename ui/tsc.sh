#!/bin/sh
# Runs tsc as Effect-TS/tsgo builds it, at the version package.json pins:
# TypeScript's native compiler with Effect's diagnostics built in.
#
#   ui/tsc.sh -p <tsconfig>
#   ui/tsc.sh --lsp --stdio
#
# The effect-lsp Claude Code plugin runs it as a language server, under
# tools/tsc_lsp.ts so that a client that never requests diagnostics still gets them;
# `-p` checks one project by hand. CI's type check is `vp lint`, through
# `make -C app lint-ts`, which installs the devDependencies this needs. A
# server's stdout carries only the protocol, so everything before the exec
# reports on stderr.
#
# @effect/tsgo's own launcher finds the compiler only beside a native
# typescript package, which nothing here needs, so this resolves the platform
# package it ships the compiler in, the way that launcher does. The compiler
# reads its lib.*.d.ts files from beside itself.
set -eu
# make -C app install-vp puts vp in Go's bin directory, which the Makefile puts
# on PATH and a shell, or the environment Claude Code starts a server in, may
# not.
PATH="$PATH:$(go env GOPATH 2>/dev/null || :)/bin"
if ! command -v vp >/dev/null 2>&1; then
  echo "ui/tsc.sh: vp is not installed; make -C app lint-ts installs it and the compiler." >&2
  exit 1
fi
# vp's update check could print into, or hold open, the output this captures.
export VP_NO_UPDATE_CHECK=1
exe=$(cd "$(dirname "$0")" && vp env exec node -e '
  const { createRequire } = require("node:module");
  const path = require("node:path");
  try {
    const tsgo = require.resolve("@effect/tsgo/package.json");
    const platform = `@effect/tsgo-${process.platform}-${process.arch}/package.json`;
    console.log(path.join(path.dirname(createRequire(tsgo).resolve(platform)), "lib", "tsc"));
  } catch (err) {
    console.error(`${err.message}\nThe compiler is a devDependency, which a production install leaves out; make -C app lint-ts installs it.`);
    process.exit(1);
  }
')
case " $* " in
*" --lsp "*) exec vp env exec node "$(dirname "$0")/tools/tsc_lsp.ts" "$exe" "$@" ;;
esac
exec "$exe" "$@"
