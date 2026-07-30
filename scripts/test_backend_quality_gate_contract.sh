#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

python3 - "$ROOT" <<'PY'
from __future__ import annotations

import re
import sys
from pathlib import Path


root = Path(sys.argv[1])
workflow_path = root / ".github/workflows/mysql57-amd64.yml"
dev_check_path = root / "scripts/dev_check.sh"
test_script_path = root / "scripts/test.sh"
phase3_plan_path = root / "docs/superpowers/plans/2026-07-29-phase3-yuanhu-business-foundation.md"
lifecycle_path = root / "scripts/smoke_schema_migrate.sh"
workflow_lines = workflow_path.read_text(encoding="utf-8").splitlines()
dev_check = dev_check_path.read_text(encoding="utf-8")
test_script = test_script_path.read_text(encoding="utf-8")
phase3_plan = phase3_plan_path.read_text(encoding="utf-8")
lifecycle_script = lifecycle_path.read_text(encoding="utf-8")
failures: list[str] = []


def fail(message: str) -> None:
    failures.append(message)


def indentation(line: str) -> int:
    return len(line) - len(line.lstrip(" "))


def yaml_scalar(value: str) -> str:
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in {'"', "'"}:
        return value[1:-1]
    return value


def trigger_paths(trigger: str) -> set[str]:
    on_index = next((i for i, line in enumerate(workflow_lines) if line == "on:"), None)
    if on_index is None:
        fail("workflow has no top-level on block")
        return set()

    trigger_index = next(
        (
            i
            for i in range(on_index + 1, len(workflow_lines))
            if workflow_lines[i] == f"  {trigger}:"
        ),
        None,
    )
    if trigger_index is None:
        fail(f"workflow has no on.{trigger} block")
        return set()

    trigger_end = next(
        (
            i
            for i in range(trigger_index + 1, len(workflow_lines))
            if workflow_lines[i].strip() and indentation(workflow_lines[i]) <= 2
        ),
        len(workflow_lines),
    )
    paths_index = next(
        (
            i
            for i in range(trigger_index + 1, trigger_end)
            if workflow_lines[i] == "    paths:"
        ),
        None,
    )
    if paths_index is None:
        fail(f"workflow has no on.{trigger}.paths block")
        return set()

    result: set[str] = set()
    for line in workflow_lines[paths_index + 1 :]:
        if line.strip() and indentation(line) <= 4:
            break
        match = re.fullmatch(r'\s{6}-\s+(.+)', line)
        if match:
            result.add(yaml_scalar(match.group(1)))
    return result


def parse_steps() -> list[dict[str, object]]:
    steps_index = next((i for i, line in enumerate(workflow_lines) if line == "    steps:"), None)
    if steps_index is None:
        fail("workflow job has no steps block")
        return []

    starts = [
        i
        for i in range(steps_index + 1, len(workflow_lines))
        if re.match(r"^      - (name|uses):", workflow_lines[i])
    ]
    steps: list[dict[str, object]] = []
    for position, start in enumerate(starts):
        end = starts[position + 1] if position + 1 < len(starts) else len(workflow_lines)
        block = workflow_lines[start:end]
        first_key, first_value = block[0].strip()[2:].split(":", 1)
        step: dict[str, object] = {first_key: yaml_scalar(first_value)}
        index = 1
        while index < len(block):
            line = block[index]
            if indentation(line) != 8 or ":" not in line.strip():
                index += 1
                continue
            key, raw_value = line.strip().split(":", 1)
            raw_value = raw_value.strip()
            if key == "run" and raw_value in {"|", ">"}:
                run_lines: list[str] = []
                index += 1
                while index < len(block) and (not block[index].strip() or indentation(block[index]) > 8):
                    run_line = block[index]
                    run_lines.append(run_line[10:] if run_line.startswith("          ") else run_line.lstrip())
                    index += 1
                step["run"] = "\n".join(run_lines).strip()
                continue
            if key == "env" and raw_value == "":
                env: dict[str, str] = {}
                index += 1
                while index < len(block) and (not block[index].strip() or indentation(block[index]) > 8):
                    env_line = block[index]
                    if indentation(env_line) == 10 and ":" in env_line.strip():
                        env_key, env_value = env_line.strip().split(":", 1)
                        env[env_key] = yaml_scalar(env_value)
                    index += 1
                step["env"] = env
                continue
            step[key] = yaml_scalar(raw_value)
            index += 1
        steps.append(step)
    return steps


required_trigger_paths = {
    ".github/workflows/mysql57-amd64.yml",
    "architecture-policy.json",
    "cmd/mochat-architecture/**",
    "cmd/mochat-go/**",
    "internal/**",
    "scripts/dev_check.sh",
    "scripts/test.sh",
    "scripts/audit_architecture_boundaries.sh",
    "scripts/architecture-size-baseline.txt",
    "scripts/test_audit_architecture_boundaries.sh",
    "scripts/test_backend_quality_gate_contract.sh",
    "scripts/ci_mysql57_amd64.sh",
    "scripts/smoke_schema_migrate.sh",
    "scripts/smoke_mysql57_schema_migrate.sh",
}
for trigger in ("push", "pull_request"):
    paths = trigger_paths(trigger)
    missing = sorted(required_trigger_paths - paths)
    if missing:
        fail(f"on.{trigger}.paths missing: {', '.join(missing)}")

steps = parse_steps()
by_name = {str(step.get("name", "")): step for step in steps}
step_names = [str(step.get("name", "")) for step in steps]
required_steps = [
    ("Go architecture gate", "go run ./cmd/mochat-architecture -root ."),
    ("Go module race gate", "go test -race ./internal/modules/..."),
    ("Go tests", "go test ./..."),
    ("Go vet", "go vet ./..."),
    ("Migration 0098 lifecycle gate", "bash ./scripts/smoke_schema_migrate.sh"),
    ("SCRM MySQL integration gate", "go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql"),
]
positions: list[int] = []
for name, command in required_steps:
    if name not in by_name:
        fail(f"workflow missing named step: {name}")
        continue
    positions.append(step_names.index(name))
    run = str(by_name[name].get("run", ""))
    if command not in run:
        fail(f"{name} step does not own command: {command}")
if len(positions) == len(required_steps) and positions != sorted(positions):
    fail("workflow gate order must be architecture -> race -> full test -> vet -> lifecycle -> integration")

lifecycle = by_name.get("Migration 0098 lifecycle gate", {})
lifecycle_env = lifecycle.get("env", {}) if isinstance(lifecycle.get("env", {}), dict) else {}
if lifecycle_env.get("MOCHAT_STACK_PROJECT") != "mochat-go-schema-migrate-ci":
    fail("migration lifecycle must use the dedicated mochat-go-schema-migrate-ci project")
if lifecycle_env.get("MOCHAT_MYSQL_PORT") != "13331":
    fail("migration lifecycle must use dedicated port 13331")
if "KEEP_STACK" in lifecycle_env:
    fail("migration lifecycle must retain default cleanup")

integration = by_name.get("SCRM MySQL integration gate", {})
integration_env = integration.get("env", {}) if isinstance(integration.get("env", {}), dict) else {}
if integration_env.get("MOCHAT_REQUIRE_MYSQL_INTEGRATION") != "1":
    fail("integration step must set strict require mode")
if integration_env.get("MOCHAT_STACK_PROJECT") != "mochat-go-scrm-integration":
    fail("integration step must use its dedicated compose project")
if integration_env.get("MOCHAT_MYSQL57_PORT") != "13333":
    fail("integration step must use dedicated port 13333")

integration_run = str(integration.get("run", ""))
cleanup_marker = "cleanup() {"
down_marker = 'docker compose -p "$MOCHAT_STACK_PROJECT" -f deploy/mysql57/docker-compose.yml down -v --remove-orphans'
trap_marker = "trap cleanup EXIT"
up_marker = 'docker compose -p "$MOCHAT_STACK_PROJECT" -f deploy/mysql57/docker-compose.yml up -d mysql57'
markers = [cleanup_marker, down_marker, trap_marker, up_marker]
if any(marker not in integration_run for marker in markers):
    fail("integration step must define cleanup/down/trap and startup in one atomic run block")
elif not (
    integration_run.index(cleanup_marker)
    < integration_run.index(down_marker)
    < integration_run.index(trap_marker)
    < integration_run.index(up_marker)
):
    fail("integration cleanup and trap must be defined before database startup")

if "KEEP_STACK:" in workflow_path.read_text(encoding="utf-8"):
    fail("workflow must not disable migration cleanup")

lifecycle_markers = [
    '"$MIGRATE_BIN" -dsn "$MIGRATE_DSN" -project-root "$PWD" -action apply >"$WORK_DIR/apply.out"',
    "WHERE version = '0098_scrm_lead_foundation' AND CHAR_LENGTH(checksum) = 64",
    '"$MIGRATE_BIN" -dsn "$MIGRATE_DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-0098.out"',
    "grep -q $'0098_scrm_lead_foundation\\trolled_back' \"$WORK_DIR/rollback-0098.out\"",
    "SHOW TABLES LIKE 'mochat_go_scrm_leads'\")\" = \"\"",
    '"$MIGRATE_BIN" -dsn "$MIGRATE_DSN" -project-root "$PWD" -action apply >"$WORK_DIR/reapply-latest.out"',
    "grep -q $'0098_scrm_lead_foundation\\tapplied_now' \"$WORK_DIR/reapply-latest.out\"",
]
missing_lifecycle_markers = [marker for marker in lifecycle_markers if marker not in lifecycle_script]
if missing_lifecycle_markers:
    fail("authoritative lifecycle script is missing 0098 apply/checksum/rollback/replay assertions")
elif [lifecycle_script.index(marker) for marker in lifecycle_markers] != sorted(
    lifecycle_script.index(marker) for marker in lifecycle_markers
):
    fail("authoritative lifecycle script must execute 0098 apply/checksum/rollback/replay in order")
for marker in ("trap cleanup EXIT INT TERM", "compose up -d mysql", "compose down -v --remove-orphans"):
    if marker not in lifecycle_script:
        fail(f"authoritative lifecycle cleanup contract missing: {marker}")
if all(marker in lifecycle_script for marker in ("trap cleanup EXIT INT TERM", "compose up -d mysql")):
    if lifecycle_script.index("trap cleanup EXIT INT TERM") > lifecycle_script.index("compose up -d mysql"):
        fail("authoritative lifecycle cleanup trap must be installed before startup")

for literal in (
    "sh scripts/test_backend_quality_gate_contract.sh",
    "go build ./cmd/mochat-go ./cmd/mochat-inventory ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./cmd/mochat-saas-maintenance ./cmd/mochat-architecture",
):
    if literal not in dev_check:
        fail(f"developer quick gate missing: {literal}")

for literal in (
    "./scripts/test_backend_quality_gate_contract.sh",
    "go build ./cmd/mochat-go ./cmd/mochat-inventory ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./cmd/mochat-saas-maintenance ./cmd/mochat-architecture",
):
    if literal not in test_script:
        fail(f"local test gate missing: {literal}")

headings = [match.start() for match in re.finditer(r"(?m)^### .+$", phase3_plan)]
if len(headings) < 7:
    fail("Phase 3 plan must retain all seven task sections")
    task2 = ""
    task7 = ""
else:
    task2 = phase3_plan[headings[1] : headings[2]]
    task7 = phase3_plan[headings[6] :]

for literal in (
    "`internal/modules/scrm/adapters/mysql/customer_lifecycle_repository_integration_test.go`",
    "`//go:build integration`",
    "TestSCRMCustomerLifecycleTenantIsolationIntegration",
    "TestSCRMCustomerLifecycleOptimisticLockIntegration",
    "TestSCRMCustomerLifecyclePublicPoolConcurrentClaimIntegration",
    "MOCHAT_REQUIRE_MYSQL_INTEGRATION=1 go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql -run 'TestSCRMCustomerLifecycle(TenantIsolation|OptimisticLock|PublicPoolConcurrentClaim)Integration'",
):
    if literal not in task2:
        fail(f"Phase 3 Task 2 missing integration contract: {literal}")

for literal in (
    "go test ./...",
    "go vet ./...",
    "go run ./cmd/mochat-architecture -root .",
    "go test -race ./internal/modules/...",
    "MOCHAT_REQUIRE_MYSQL_INTEGRATION=1 go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql",
    "bash ./scripts/smoke_schema_migrate.sh",
    "go build ./cmd/mochat-go ./cmd/mochat-inventory ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./cmd/mochat-saas-maintenance ./cmd/mochat-architecture",
):
    if literal not in task7:
        fail(f"Phase 3 Task 7 missing final gate: {literal}")

if failures:
    for message in failures:
        print(f"backend quality gate contract: {message}", file=sys.stderr)
    raise SystemExit(1)

print("backend quality gate workflow contract passed")
PY
