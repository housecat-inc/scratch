#!/bin/sh
set -eu

REPO="housecat-inc/scratch"
BIN="${BIN:-${1:-scratch}}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux|darwin) ;;
  *) echo "unsupported OS: $OS" >&2; exit 1 ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "unsupported arch: $ARCH" >&2; exit 1 ;;
esac

URL="https://github.com/${REPO}/releases/latest/download/${BIN}_${OS}_${ARCH}.tar.gz"

mkdir -p "$INSTALL_DIR"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "downloading $URL"
curl -fsSL "$URL" | tar -xz -C "$TMP"
mv "$TMP/$BIN" "$INSTALL_DIR/$BIN"
chmod +x "$INSTALL_DIR/$BIN"

echo "installed $INSTALL_DIR/$BIN"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "note: $INSTALL_DIR is not in PATH" ;;
esac

if command -v shelley >/dev/null 2>&1; then
  VM=$(hostname -s 2>/dev/null || hostname)
  echo
  echo "detected exe.dev / Shelley"
  echo "run as a systemd service on port 8888:"
  echo "  systemd-run --user --unit=$BIN --collect $INSTALL_DIR/$BIN --port 8888"
  echo "then continue at the private URL https://$VM.exe.xyz:8888/"
fi
