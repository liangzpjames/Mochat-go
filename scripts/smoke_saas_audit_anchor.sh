#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."

MOCHAT_STACK_PROJECT="${MOCHAT_STACK_PROJECT:-mochat-go-saas-audit-anchor-check}" \
MOCHAT_MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13413}" \
MOCHAT_REDIS_PORT="${MOCHAT_REDIS_PORT:-26463}" \
MOCHAT_GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18187}" \
scripts/smoke_saas_audit_integrity.sh
