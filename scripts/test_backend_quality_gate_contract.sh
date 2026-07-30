#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
WORKFLOW="$ROOT/.github/workflows/mysql57-amd64.yml"
DEV_CHECK="$ROOT/scripts/dev_check.sh"
TEST_SCRIPT="$ROOT/scripts/test.sh"
FAILED=0

require_in_file() {
  file="$1"
  literal="$2"
  description="$3"
  if ! grep -Fq -- "$literal" "$file"; then
    echo "backend quality gate contract: missing $description" >&2
    FAILED=1
  fi
}

require_workflow_literal() {
  literal="$1"
  description="$2"
  require_in_file "$WORKFLOW" "$literal" "$description"
}

migration_path_count="$(grep -Fc -- '- "migrations/**"' "$WORKFLOW" || true)"
if [ "$migration_path_count" -ne 2 ]; then
  echo "backend quality gate contract: migrations/** must trigger both push and pull_request (found $migration_path_count)" >&2
  FAILED=1
fi

if grep -Fq -- 'KEEP_STACK:' "$WORKFLOW"; then
  echo "backend quality gate contract: MySQL migration smoke must retain its default cleanup" >&2
  FAILED=1
fi

require_workflow_literal \
  'trap cleanup EXIT' \
  "failure-safe SCRM integration cleanup trap"
require_workflow_literal \
  'docker compose -p "$MOCHAT_STACK_PROJECT" -f deploy/mysql57/docker-compose.yml up -d mysql57' \
  "dedicated SCRM integration database startup"
require_workflow_literal \
  'go run ./cmd/mochat-migrate -dsn "$MOCHAT_MYSQL_DSN" -project-root . -action apply' \
  "SCRM integration migration apply"
require_workflow_literal \
  "version = '0098_scrm_lead_foundation'" \
  "migration 0098 readiness assertion"
require_workflow_literal \
  'go test -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql' \
  "uncached SCRM MySQL integration command"
require_workflow_literal \
  'docker compose -p "$MOCHAT_STACK_PROJECT" -f deploy/mysql57/docker-compose.yml down -v --remove-orphans' \
  "dedicated SCRM integration database cleanup"
require_workflow_literal \
  'sh ./scripts/test_backend_quality_gate_contract.sh' \
  "CI workflow contract self-test"
require_in_file \
  "$DEV_CHECK" \
  'sh scripts/test_backend_quality_gate_contract.sh' \
  "developer quick workflow contract self-test"
require_in_file \
  "$TEST_SCRIPT" \
  './scripts/test_backend_quality_gate_contract.sh' \
  "local test workflow contract self-test"

if [ "$FAILED" -ne 0 ]; then
  exit 1
fi

echo "backend quality gate workflow contract passed"
