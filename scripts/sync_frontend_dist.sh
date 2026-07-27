#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

SOURCE_ROOT="${MOCHAT_FRONTEND_SOURCE_ROOT:-../mochat}"
TARGET_ROOT="${MOCHAT_FRONTEND_TARGET_ROOT:-web}"

sync_one() {
  local name="$1"
  local source="$SOURCE_ROOT/$name/dist"
  local target="$TARGET_ROOT/$name/dist"
  if [ ! -f "$source/index.html" ]; then
    echo "$name dist not found: $source" >&2
    exit 1
  fi
  mkdir -p "$target"
  rm -rf "$target"
  mkdir -p "$(dirname "$target")"
  cp -R "$source" "$target"
  test -f "$target/index.html"
}

sync_one dashboard
sync_one sidebar
sync_one operation

echo "frontend dist synced to $TARGET_ROOT"
