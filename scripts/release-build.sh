#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
DIST_DIR="${DIST_DIR:-dist}"
PLATFORMS="${PLATFORMS:-linux/amd64 linux/arm64}"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

npm --prefix web ci
npm --prefix web run build
go test ./...

for platform in $PLATFORMS; do
  os="${platform%/*}"
  arch="${platform#*/}"
  name="gsm-panel-${VERSION}-${os}-${arch}"
  out_dir="$DIST_DIR/$name"
  mkdir -p "$out_dir"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -ldflags "-s -w" -o "$out_dir/gsm-panel" ./cmd/gsm-panel
  cp README.md "$out_dir/README.md"
  cp -r data "$out_dir/data"
  tar -C "$DIST_DIR" -czf "$DIST_DIR/$name.tar.gz" "$name"
  rm -rf "$out_dir"
done

(
  cd "$DIST_DIR"
  sha256sum *.tar.gz > SHA256SUMS
)
