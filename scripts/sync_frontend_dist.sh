#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

SOURCE_ROOT="${MOCHAT_FRONTEND_SOURCE_ROOT:-../mochat}"
TARGET_ROOT="${MOCHAT_FRONTEND_TARGET_ROOT:-web}"

sync_one() {
  local source_name="$1"
  local target_name="$2"
  local source="$SOURCE_ROOT/$source_name/dist"
  local target="$TARGET_ROOT/$target_name/dist"
  if [ ! -f "$source/index.html" ]; then
    echo "$source_name dist not found: $source" >&2
    exit 1
  fi
  mkdir -p "$target"
  rm -rf "$target"
  mkdir -p "$(dirname "$target")"
  cp -R "$source" "$target"
  test -f "$target/index.html"
}

sync_one dashboard dashboard
sync_one sidebar sidebar
sync_one operation operation
sync_one saas-admin apps/saas-admin

echo "frontend dist synced to $TARGET_ROOT"
