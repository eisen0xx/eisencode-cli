#!/usr/bin/env bash
# Build the release matrix into dist/.
set -euo pipefail

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
OUT="${OUT:-dist}"
LDFLAGS="-s -w -X main.version=${VERSION}"

mkdir -p "$OUT"

build() {
  local os="$1" arch="$2" ext="${3:-}"
  local name="eisencode-${os}-${arch}${ext}"
  echo "  ${name}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "$LDFLAGS" -o "${OUT}/${name}" .
}

echo "building eisencode ${VERSION}"
build linux amd64
build linux arm64
build darwin amd64
build darwin arm64
build windows amd64 .exe
echo "done → ${OUT}/"
