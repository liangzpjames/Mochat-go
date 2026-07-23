#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

python3 - "$(pwd)" <<'PY'
from pathlib import Path
import re
import sys

repo = Path(sys.argv[1])
failures = []


def read(rel: str) -> str:
    return (repo / rel).read_text(encoding="utf-8")


def fail(message: str) -> None:
    failures.append(message)


quota_go = read("internal/dashboard/saas_quota.go")
metrics = [
    (name, value)
    for name, value in re.findall(r'\b(SaaSMetric[A-Za-z0-9]+)\s*=\s*"([^"]+)"', quota_go)
]

if not metrics:
    fail("internal/dashboard/saas_quota.go: no SaaS metrics found")

metric_names = [name for name, _ in metrics]
metric_values = [value for _, value in metrics]

label_cases = set(re.findall(r'case\s+(SaaSMetric[A-Za-z0-9]+)\s*:', quota_go))
for name in metric_names:
    if name not in label_cases:
        fail(f"internal/dashboard/saas_quota.go: saasMetricLabel missing {name}")

store_go = read("internal/store/mysql.go")


def section(text: str, marker: str) -> str:
    start = text.find(marker)
    if start < 0:
        fail(f"internal/store/mysql.go: missing section {marker}")
        return ""
    next_func = text.find("\nfunc ", start + len(marker))
    if next_func < 0:
        return text[start:]
    return text[start:next_func]


limit_for = section(store_go, "func (l saasUsageLimitSnapshot) limitFor")
current_usage = section(store_go, "func (s *MySQLStore) saasCurrentUsage")
usage_metrics = section(store_go, "func saasUsageMetrics() []string")

for name, _ in metrics:
    ref = "dashboard." + name
    if ref not in limit_for:
        fail(f"internal/store/mysql.go: limitFor missing {ref}")
    if ref not in current_usage:
        fail(f"internal/store/mysql.go: saasCurrentUsage missing {ref}")
    if ref not in usage_metrics:
        fail(f"internal/store/mysql.go: saasUsageMetrics missing {ref}")

bootstrap = read("cmd/mochat-bootstrap/main.go")
for _, value in metrics:
    if f'name: "{value}"' not in bootstrap:
        fail(f"cmd/mochat-bootstrap/main.go: upsertUsageCounters missing metric {value}")

quota_smoke = read("scripts/smoke_saas_quota_enforcement.sh")
usage_smoke = read("scripts/smoke_saas_usage_refresh.sh")
for _, value in metrics:
    if f"metric = '{value}'" not in quota_smoke and f'"{value}"' not in quota_smoke:
        fail(f"scripts/smoke_saas_quota_enforcement.sh: missing metric {value}")
    if f'assert_metric "{value}"' not in usage_smoke:
        fail(f"scripts/smoke_saas_usage_refresh.sh: missing assert_metric for {value}")

expected_count = len(metrics)
for rel in [
    "scripts/smoke_saas_quota_enforcement.sh",
    "scripts/smoke_saas_usage_refresh.sh",
]:
    text = read(rel)
    if f"usage_metric_count\\t{expected_count}" not in text and f"metrics_refreshed\\t{expected_count}" not in text:
        fail(f"{rel}: missing expected SaaS metric count {expected_count}")

if failures:
    print("SaaS metric coverage audit failed:", file=sys.stderr)
    for failure in failures:
        print(f"- {failure}", file=sys.stderr)
    raise SystemExit(1)

print(f"SaaS metric coverage audit passed: metrics={expected_count}")
for _, value in metrics:
    print(f"  {value}")
PY
