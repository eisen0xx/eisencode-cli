#!/usr/bin/env bash
# eisencode installer — https://eisencode.waivy.dev
set -euo pipefail

REPO="eisen0xx/eisencode-cli"
VERSION="${EISENCODE_VERSION:-latest}"

say() { printf '\033[38;5;33m%s\033[0m\n' "$*"; }
die() { printf '\033[38;5;160merror: %s\033[0m\n' "$*" >&2; exit 1; }

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux|darwin) ;;
  *) die "unsupported OS: $OS (use install.ps1 on Windows)" ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "unsupported arch: $ARCH" ;;
esac

BIN="eisencode-${OS}-${ARCH}"
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${BIN}"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${BIN}"
fi

say "eisencode — installing ${BIN}"

if [ -n "${EISENCODE_BIN_DIR:-}" ]; then
  DEST="$EISENCODE_BIN_DIR"
elif [ -w /usr/local/bin ]; then
  DEST=/usr/local/bin
else
  DEST="$HOME/.local/bin"
fi
mkdir -p "$DEST"

TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$URL" -o "$TMP" || die "download failed: $URL"
else
  wget -qO "$TMP" "$URL" || die "download failed: $URL"
fi

chmod +x "$TMP"
mv "$TMP" "$DEST/eisencode"

say "installed → $DEST/eisencode"
"$DEST/eisencode" version || true

case ":$PATH:" in
  *":$DEST:"*) ;;
  *) say "note: add $DEST to your PATH:"
     say "  export PATH=\"$DEST:\$PATH\"" ;;
esac

echo
say "next:"
echo "  eisencode login sk_eis_your_key    # key from https://eisencode.waivy.dev/dashboard"
echo "  eisencode run \"hello\""
