#!/usr/bin/env bash
# Installs claude-acc into ~/.local/bin.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/linhh-phv/claude-acc/main/install.sh | bash
# or, from a local clone:
#   ./install.sh

set -euo pipefail

BIN_DIR="${CLAUDE_ACC_BIN_DIR:-$HOME/.local/bin}"
RAW_URL="https://raw.githubusercontent.com/linhh-phv/claude-acc/main/bin/claude-acc"

c_b=$'\033[1m'; c_g=$'\033[32m'; c_y=$'\033[33m'; c_r=$'\033[31m'; c_0=$'\033[0m'
[[ -t 1 ]] || { c_b=; c_g=; c_y=; c_r=; c_0=; }

die() { printf '%serror:%s %s\n' "$c_r" "$c_0" "$*" >&2; exit 1; }

[[ "$(uname -s)" == "Darwin" ]] || die "claude-acc only works on macOS (it shells out to the Keychain's \`security\` command)."
command -v claude >/dev/null 2>&1 || printf '%swarning:%s `claude` not found in PATH yet — install Claude Code first: https://claude.com/claude-code\n' "$c_y" "$c_0" >&2
command -v python3 >/dev/null 2>&1 || die "python3 not found (should ship with macOS)."

mkdir -p "$BIN_DIR"
DEST="$BIN_DIR/claude-acc"

# Running from a local clone (this script sits next to bin/claude-acc)?
# Copy the local file instead of re-downloading — keeps a `git clone` +
# `./install.sh` workflow working offline and always installs exactly what
# you have checked out.
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
if [[ -f "$SELF_DIR/bin/claude-acc" ]]; then
  cp "$SELF_DIR/bin/claude-acc" "$DEST"
else
  command -v curl >/dev/null 2>&1 || die "curl not found."
  curl -fsSL "$RAW_URL" -o "$DEST" || die "download failed."
fi

chmod +x "$DEST"
printf '%s✓ installed:%s %s\n' "$c_g" "$c_0" "$DEST"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *)
    printf '\n%s%s is not on your PATH.%s Add this to ~/.zshrc (or ~/.bashrc):\n\n' "$c_y" "$BIN_DIR" "$c_0"
    printf '  export PATH="%s:$PATH"\n\n' "$BIN_DIR"
    ;;
esac

printf 'Run %sclaude-acc help%s to get started.\n' "$c_b" "$c_0"
