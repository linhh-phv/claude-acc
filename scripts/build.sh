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
# The Windows self-test: run it on a Windows machine next to claude-acc.exe.
for arch in amd64 arm64; do
  out="dist/claude-acc-selftest-windows-$arch.exe"
  GOOS=windows GOARCH="$arch" CGO_ENABLED=0 go test -c -trimpath -o "$out" ./internal/wintest
  printf '%s\n' "$out"
done
# What to hand a Windows user: unzip, double-click install.cmd.
for arch in amd64 arm64; do
  stage="$(mktemp -d)"
  cp "dist/claude-acc-windows-$arch.exe" install.ps1 install.cmd "$stage/"
  cp docs/windows-vi.md "$stage/HUONG-DAN.md"
  (cd "$stage" && zip -q -X "$OLDPWD/dist/claude-acc-windows-$arch.zip" ./*)
  rm -rf "$stage"
  printf '%s\n' "dist/claude-acc-windows-$arch.zip"
done
(cd dist && shasum -a 256 claude-acc-* > SHA256SUMS)
