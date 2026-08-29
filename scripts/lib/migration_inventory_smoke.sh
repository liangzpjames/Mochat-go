#!/usr/bin/env bash

build_migration_smoke_binaries() {
  env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
  env -u GOROOT go build -o "$IDENTITY_MIGRATE_BIN" ./cmd/mochat-identity-migrate
  env -u GOROOT go build -o "$AI_INSIGHT_0165_BIN" ./cmd/mochat-ai-insight-0165
}

load_migration_inventory() {
  INVENTORY_FILE="$WORK_DIR/migration-inventory.tsv"
  "$MIGRATE_BIN" -project-root "$PWD" -action inventory >"$INVENTORY_FILE"
  awk -F '\t' '
    NF != 4 { exit 10 }
    { version=substr($1,1,4); if (NR != (version + 0)) exit 11 }
    length($2) != 64 || $2 !~ /^[0-9a-f]+$/ { exit 12 }
    $3 != "automatic" && $3 != "controlled" { exit 13 }
    END { if (NR == 0) exit 14 }
  ' "$INVENTORY_FILE"
  INVENTORY_COUNT="$(wc -l <"$INVENTORY_FILE" | tr -d ' ')"
  INVENTORY_FIRST="$(awk -F '\t' 'NR==1 {print $1}' "$INVENTORY_FILE")"
  INVENTORY_LATEST="$(awk -F '\t' 'END {print $1}' "$INVENTORY_FILE")"
  INVENTORY_LATEST_CHECKSUM="$(awk -F '\t' 'END {print $2}' "$INVENTORY_FILE")"
  INVENTORY_LATEST_KIND="$(awk -F '\t' 'END {print $3}' "$INVENTORY_FILE")"
  test "$INVENTORY_FIRST" = "0001_initial_schema"
  test "$INVENTORY_LATEST_KIND" = "automatic"
}

identity_flags() {
  local request_id="$1"
  local confirmation="$WORK_DIR/maintenance-$request_id"
  printf 'MOCHAT_IDENTITY_MAINTENANCE_V1\nschema=%s\nrequest_id=%s\n' "$MYSQL_SCHEMA" "$request_id" >"$confirmation"
  printf '%s\n' "$IDENTITY_MIGRATE_BIN" "$request_id" "$confirmation"
}

run_identity_action() {
  local action="$1" request_id="$2" confirmation
  confirmation="$WORK_DIR/maintenance-$request_id"
  printf 'MOCHAT_IDENTITY_MAINTENANCE_V1\nschema=%s\nrequest_id=%s\n' "$MYSQL_SCHEMA" "$request_id" >"$confirmation"
  "$IDENTITY_MIGRATE_BIN" "$action" --execute --request-id "$request_id" --dsn-file "$DSN_FILE" \
    --schema "$MYSQL_SCHEMA" --platform-tenant-id 1 --maintenance-confirmation-file "$confirmation" \
    --credential-key-file "$CREDENTIAL_KEY_FILE" --credential-key-id migration-smoke-v1 --project-root "$PWD" --timeout 10m
}

expect_apply_blocked_by() {
  local version="$1" output="$WORK_DIR/apply-$version.out"
  if "$MIGRATE_BIN" -dsn "$MYSQL_DSN" -project-root "$PWD" -action apply >"$output" 2>&1; then
    echo "migration apply unexpectedly crossed controlled migration $version" >&2
    return 1
  fi
  grep -q "$version" "$output"
}

apply_full_inventory() {
  expect_apply_blocked_by 0130_identity_realms_single_corp_backfill
  mysql_scalar "$MYSQL_SCHEMA" "INSERT INTO mc_tenant (id,name,status) SELECT 1,'migration-smoke-platform',1 FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM mc_tenant WHERE id=1)" >/dev/null
  run_identity_action up migration-smoke-0130
  run_identity_action encrypt-credentials migration-smoke-0130
  expect_apply_blocked_by 0131_identity_realms_single_corp_cutover
  run_identity_action cutover migration-smoke-0131
  expect_apply_blocked_by 0165_ai_daily_insight_unification

  local request_id="migration-smoke-0165"
  "$AI_INSIGHT_0165_BIN" backup --dsn-file "$DSN_FILE" --project-root "$PWD" --request-id "$request_id" >"$WORK_DIR/0165-backup.json"
  "$AI_INSIGHT_0165_BIN" preflight --dsn-file "$DSN_FILE" --project-root "$PWD" --request-id "$request_id" >"$WORK_DIR/0165-preflight.json"
  local approval_token destructive_approval
  approval_token="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["approvalToken"])' "$WORK_DIR/0165-preflight.json")"
  destructive_approval="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["destructiveApproval"])' "$WORK_DIR/0165-preflight.json")"
  "$AI_INSIGHT_0165_BIN" apply --dsn-file "$DSN_FILE" --project-root "$PWD" --request-id "$request_id" \
    --approval-token "$approval_token" --approve-destructive "$destructive_approval" --confirm-traffic-stopped >"$WORK_DIR/0165-apply.json"
  "$AI_INSIGHT_0165_BIN" verify --dsn-file "$DSN_FILE" --project-root "$PWD" --request-id "$request_id" >"$WORK_DIR/0165-verify.json"
  "$MIGRATE_BIN" -dsn "$MYSQL_DSN" -project-root "$PWD" -action apply >"$WORK_DIR/apply-final.out"
}

verify_full_inventory_ledger() {
  "$MIGRATE_BIN" -dsn "$MYSQL_DSN" -project-root "$PWD" -action status >"$WORK_DIR/status.out"
  while IFS=$'\t' read -r version checksum kind description; do
    grep -q "^${version}"$'\t''applied$' "$WORK_DIR/status.out"
  done <"$INVENTORY_FILE"
  test "$(mysql_scalar "$MYSQL_SCHEMA" "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "$INVENTORY_COUNT"
  test "$(mysql_scalar "$MYSQL_SCHEMA" "SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version='$INVENTORY_FIRST')" = "1"
  test "$(mysql_scalar "$MYSQL_SCHEMA" "SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version='$INVENTORY_LATEST' AND checksum='$INVENTORY_LATEST_CHECKSUM')" = "1"
}

verify_latest_rollback_reapply() {
  "$MIGRATE_BIN" -dsn "$MYSQL_DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-latest.out"
  grep -q "^${INVENTORY_LATEST}"$'\t''rolled_back$' "$WORK_DIR/rollback-latest.out"
  test "$(mysql_scalar "$MYSQL_SCHEMA" "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "$((INVENTORY_COUNT - 1))"
  "$MIGRATE_BIN" -dsn "$MYSQL_DSN" -project-root "$PWD" -action apply >"$WORK_DIR/reapply-latest.out"
  grep -q "^${INVENTORY_LATEST}"$'\t''applied_now$' "$WORK_DIR/reapply-latest.out"
}

verify_checksum_drift_rejected() {
  local drift_root="$WORK_DIR/checksum-drift"
  mkdir -p "$drift_root/deploy/standalone"
  cp -R deploy/standalone/migrations "$drift_root/deploy/standalone/"
  printf '\n-- controlled checksum drift probe\n' >>"$drift_root/deploy/standalone/migrations/${INVENTORY_LATEST}.up.sql"
  if "$MIGRATE_BIN" -dsn "$MYSQL_DSN" -project-root "$drift_root" -action status >"$WORK_DIR/checksum-drift.out" 2>&1; then
    echo "checksum drift was accepted" >&2
    return 1
  fi
  grep -qi 'checksum mismatch' "$WORK_DIR/checksum-drift.out"
}

verify_baseline_from_full_schema() {
  mysql_scalar "$MYSQL_SCHEMA" "DROP TABLE mochat_go_schema_migrations" >/dev/null
  "$MIGRATE_BIN" -dsn "$MYSQL_DSN" -project-root "$PWD" -action baseline >"$WORK_DIR/baseline.out"
  test "$(mysql_scalar "$MYSQL_SCHEMA" "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "$INVENTORY_COUNT"
}

run_migration_inventory_smoke() {
  build_migration_smoke_binaries
  load_migration_inventory
  DSN_FILE="$WORK_DIR/mysql.dsn"
  CREDENTIAL_KEY_FILE="$WORK_DIR/wecom-credential.key"
  printf '%s' "$MYSQL_DSN" >"$DSN_FILE"
  printf '%064d' 1 >"$CREDENTIAL_KEY_FILE"
  apply_full_inventory
  verify_full_inventory_ledger
  verify_latest_rollback_reapply
  verify_checksum_drift_rejected
  verify_baseline_from_full_schema
  echo "migration inventory smoke passed: count=$INVENTORY_COUNT first=$INVENTORY_FIRST latest=$INVENTORY_LATEST checksum=$INVENTORY_LATEST_CHECKSUM kind=$INVENTORY_LATEST_KIND schema=$MYSQL_SCHEMA"
}
