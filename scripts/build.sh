#!/usr/bin/env bash
# Cross-compiles claude-acc for every supported platform into dist/, with
# checksums — the files a GitHub release carries.
set -euo pipefail
cd "$(dirname "$0")/.."

rm -rf dist
mkdir -p dist
for target in darwin/arm64 darwin/amd64 windows/amd64 windows/arm64; do
  os="${target%/*}" arch="${target#*/}"
  out="dist/claude-acc-$os-$arch"
  [[ "$os" == windows ]] && out="$out.exe"
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$out" ./cmd/claude-acc
  printf '%s\n' "$out"
done
(cd dist && shasum -a 256 claude-acc-* > SHA256SUMS)
