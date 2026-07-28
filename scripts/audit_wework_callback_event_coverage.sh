#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

python3 - "$(pwd)" <<'PY'
from pathlib import Path
import re
import sys

repo = Path(sys.argv[1])
worker_path = repo / "internal/dashboard/wework_callback_worker.go"
test_path = repo / "internal/dashboard/wework_callback_worker_test.go"
smoke_path = repo / "scripts/smoke_wework_callback_worker.sh"
test_entry_path = repo / "scripts/test.sh"
readme_path = repo / "README.md"
gap_path = repo / "docs/phases/phase-pre0-standalone/reports/standalone-gap.md"

worker = worker_path.read_text(encoding="utf-8")
tests = test_path.read_text(encoding="utf-8")
smoke = smoke_path.read_text(encoding="utf-8")
test_entry = test_entry_path.read_text(encoding="utf-8")
readme = readme_path.read_text(encoding="utf-8")
gap = gap_path.read_text(encoding="utf-8")

events = sorted(set(re.findall(r'"(event\.[A-Za-z0-9_.]+)"', worker)))
smoke_events = sorted(set(re.findall(r'"(event\.[A-Za-z0-9_.]+)"', smoke)))
test_events = sorted(set(re.findall(r'"(event\.[A-Za-z0-9_.]+)"', tests)))

expected_noop_events = [
    "event.change_contact.update_tag",
    "event.change_external_contact.add_half_external_contact",
    "event.change_external_contact.transfer_fail",
    "event.change_external_tag.shuffle",
]

failures = []
if not events:
    failures.append(f"{worker_path.relative_to(repo)}: no callback event paths found")

for event in events:
    if event not in smoke:
        failures.append(f"{smoke_path.relative_to(repo)}: missing smoke coverage for {event}")
    if event not in tests and event not in smoke:
        failures.append(f"{test_path.relative_to(repo)} or {smoke_path.relative_to(repo)}: missing test coverage for {event}")

for event in smoke_events:
    if event not in events:
        failures.append(f"{smoke_path.relative_to(repo)}: stale event not handled by worker Process: {event}")

for event in test_events:
    if event not in events:
        failures.append(f"{test_path.relative_to(repo)}: stale event not handled by worker Process: {event}")

for event in expected_noop_events:
    if event not in events:
        failures.append(f"{worker_path.relative_to(repo)}: expected PHP-compatible no-op event is not handled: {event}")
    if event not in smoke:
        failures.append(f"{smoke_path.relative_to(repo)}: expected no-op event is not smoked: {event}")
    if event not in tests:
        failures.append(f"{test_path.relative_to(repo)}: expected no-op event is not unit tested: {event}")

audit_name = "./scripts/audit_wework_callback_event_coverage.sh"
if audit_name not in test_entry:
    failures.append(f"{test_entry_path.relative_to(repo)}: missing {audit_name}")
if "audit_wework_callback_event_coverage.sh" not in readme:
    failures.append(f"{readme_path.relative_to(repo)}: missing audit documentation")
if "audit_wework_callback_event_coverage.sh" not in gap:
    failures.append(f"{gap_path.relative_to(repo)}: missing audit documentation")

if failures:
    print("wework callback event coverage audit failed:", file=sys.stderr)
    for failure in failures:
        print(f"- {failure}", file=sys.stderr)
    sys.exit(1)

print(f"wework callback event coverage audit passed: process_events={len(events)} smoke_events={len(smoke_events)} unit_events={len(test_events)}")
for event in events:
    print(f"  {event}")
PY
