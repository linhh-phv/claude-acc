#!/usr/bin/env bash
# Installs claude-acc into ~/.local/bin (macOS). On Windows use install.ps1.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/linhh-phv/claude-acc/main/install.sh | bash
# or, from a local clone (builds from source, needs Go):
#   ./install.sh

set -euo pipefail

BIN_DIR="${CLAUDE_ACC_BIN_DIR:-$HOME/.local/bin}"
REPO="linhh-phv/claude-acc"

c_b=$'\033[1m'; c_g=$'\033[32m'; c_y=$'\033[33m'; c_r=$'\033[31m'; c_0=$'\033[0m'
[[ -t 1 ]] || { c_b=; c_g=; c_y=; c_r=; c_0=; }

die() { printf '%serror:%s %s\n' "$c_r" "$c_0" "$*" >&2; exit 1; }

[[ "$(uname -s)" == "Darwin" ]] || die "this installer is for macOS. On Windows run install.ps1 in PowerShell."
command -v claude >/dev/null 2>&1 || printf '%swarning:%s `claude` not found in PATH yet — install Claude Code first: https://claude.com/claude-code\n' "$c_y" "$c_0" >&2

mkdir -p "$BIN_DIR"
DEST="$BIN_DIR/claude-acc"

# Running from a local clone? Build exactly what is checked out — works
# offline and needs no published release.
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
if [[ -f "$SELF_DIR/go.mod" && -d "$SELF_DIR/cmd/claude-acc" ]]; then
  command -v go >/dev/null 2>&1 || die "building from a clone needs Go (brew install go), or use the curl one-liner to download a release."
  (cd "$SELF_DIR" && CGO_ENABLED=0 go build -trimpath -o "$DEST.new" ./cmd/claude-acc) || die "go build failed."
else
  case "$(uname -m)" in
    arm64)  arch=arm64 ;;
    x86_64) arch=amd64 ;;
    *)      die "unsupported CPU: $(uname -m)" ;;
  esac
  command -v curl >/dev/null 2>&1 || die "curl not found."
  url="https://github.com/$REPO/releases/latest/download/claude-acc-darwin-$arch"
  curl -fsSL "$url" -o "$DEST.new" || die "download failed: $url"
fi

# Replace in one step, so a running claude-acc is never left half-written.
chmod +x "$DEST.new"
mv -f "$DEST.new" "$DEST"
printf '%s✓ installed:%s %s (%s)\n' "$c_g" "$c_0" "$DEST" "$("$DEST" version)"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *)
    printf '\n%s%s is not on your PATH.%s Add this to ~/.zshrc (or ~/.bashrc):\n\n' "$c_y" "$BIN_DIR" "$c_0"
    printf '  export PATH="%s:$PATH"\n\n' "$BIN_DIR"
    ;;
esac

printf 'Run %sclaude-acc help%s to get started.\n' "$c_b" "$c_0"
