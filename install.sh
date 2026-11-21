#!/usr/bin/env bash
# eisencode installer — https://eisencode.waivy.dev
#
# Installs the standalone eisencode CLI. It does not need or modify opencode.
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

# Shell completion — printed only, never edits your dotfiles silently.
SHELL_NAME="$(basename "${SHELL:-}")"
case "$SHELL_NAME" in
  bash) COMPLETION="eisencode completion bash >> ~/.bashrc" ;;
  zsh)  COMPLETION="eisencode completion zsh > \"\${fpath[1]}/_eisencode\"" ;;
  fish) COMPLETION="mkdir -p ~/.config/fish/completions && eisencode completion fish > ~/.config/fish/completions/eisencode.fish" ;;
  *)    COMPLETION="eisencode completion bash   # or zsh | fish | powershell" ;;
esac

echo
say "next:"
echo "  eisencode login                    # prompts for your key"
echo "  eisencode run \"hello\""
echo "  eisencode chat                     # isolated sessions, /help inside"
echo
say "shell completion ($SHELL_NAME):"
echo "  $COMPLETION"
echo
say "optional: hand a session to opencode (only if installed):"
echo "  eisencode sessions export <id> --import"
