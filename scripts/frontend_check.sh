#!/usr/bin/env sh
set -eu

mode="${1:-}"
case "$mode" in
  quick|build|e2e) ;;
  *) echo "usage: frontend_check.sh quick|build|e2e" >&2; exit 2 ;;
esac

cd "$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
corepack_command="${MOCHAT_COREPACK_COMMAND:-corepack}"

node_version="${MOCHAT_TEST_NODE_VERSION:-$(node --version)}"
node_version="${node_version#v}"
case "$node_version" in
  *.*.*) ;;
  *) echo "invalid Node version: $node_version" >&2; exit 1 ;;
esac
node_major="${node_version%%.*}"
node_rest="${node_version#*.}"
node_minor="${node_rest%%.*}"
node_patch="${node_rest#*.}"
case "$node_major:$node_minor:$node_patch" in
  *[!0-9:]*|*::*|:*|*:) echo "invalid Node version: $node_version" >&2; exit 1 ;;
esac
if [ "$node_major" -lt 22 ] || { [ "$node_major" -eq 22 ] && [ "$node_minor" -lt 12 ]; } || [ "$node_major" -ge 25 ]; then
  echo "Node 22.12 through 24.x is required; found $node_version" >&2
  exit 1
fi

expected_pnpm="$(node -p "require('./package.json').packageManager.replace(/^pnpm@/, '')")"
actual_pnpm="${MOCHAT_TEST_PNPM_VERSION:-$("$corepack_command" pnpm --version)}"
if [ "$actual_pnpm" != "$expected_pnpm" ]; then
  echo "pnpm $expected_pnpm is required; found $actual_pnpm" >&2
  exit 1
fi

case "$mode" in
  quick)
    "$corepack_command" pnpm check:audit
    "$corepack_command" pnpm check:deps
    "$corepack_command" pnpm lint
    "$corepack_command" pnpm typecheck
    "$corepack_command" pnpm test
    ;;
  build)
    "$corepack_command" pnpm install --frozen-lockfile
    "$corepack_command" pnpm build
    for dist in "${MOCHAT_TEST_DASHBOARD_DIST:-web/apps/dashboard/dist/index.html}" "${MOCHAT_TEST_SAAS_ADMIN_DIST:-web/apps/saas-admin/dist/index.html}"; do
      test -f "$dist" || { echo "frontend build output is missing: $dist" >&2; exit 1; }
    done
    ;;
  e2e)
    "$corepack_command" pnpm --filter @mochat/e2e test:e2e
    ;;
esac
