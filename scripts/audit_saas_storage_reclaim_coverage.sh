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


def function_before(text: str, offset: int) -> str:
    prefix = text[:offset]
    matches = list(re.finditer(r"(?m)^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(", prefix))
    if not matches:
        return ""
    return matches[-1].group(1)


actual = set()
for path in sorted((repo / "internal/store").glob("*.go")):
    text = path.read_text(encoding="utf-8")
    for match in re.finditer(r"\breclaimSaaSStorageObjects\s*\(", text):
        line_start = text.rfind("\n", 0, match.start()) + 1
        line_end = text.find("\n", match.start())
        if line_end < 0:
            line_end = len(text)
        line = text[line_start:line_end].strip()
        if line.startswith("func "):
            continue
        fn = function_before(text, match.start())
        if fn:
            actual.add(fn)

expected_functions = {
    "DeleteContactBatchAddImportRecords",
    "DeleteChannelCode",
    "UpdateMedium",
    "DeleteMedium",
    "UpdateRoomWelcome",
    "DeleteRoomWelcome",
    "UpdateWorkRoomAutoPullWithLog",
    "DeleteWorkRoomAutoPull",
    "DeleteRoomTagPull",
    "DeleteContactMessageBatchSend",
    "DeleteRoomMessageBatchSend",
    "DeleteWorkFissionCascade",
    "UpdateLottery",
    "DeleteLottery",
    "UpdateRadar",
    "DeleteRadar",
    "UpdateRoomClockIn",
    "DeleteRoomClockIn",
    "UpdateRoomFission",
    "DeleteRoomFission",
    "UpsertRoomFissionInvite",
    "UpdateRoomInfinitePull",
    "DeleteRoomInfinitePull",
    "UpdateShopCode",
    "DeleteShopCode",
    "UpsertShopCodePageSetting",
}

missing_functions = sorted(expected_functions - actual)
unexpected_functions = sorted(actual - expected_functions)
for fn in missing_functions:
    fail(f"internal/store: expected SaaS storage reclaim function not found: {fn}")
for fn in unexpected_functions:
    fail(f"internal/store: new SaaS storage reclaim function needs smoke coverage mapping: {fn}")

smoke = read("scripts/smoke_saas_storage_reclaim.sh")
groups = {
    "contact batch add import": [
        "dashboard/contactBatchAdd/importDestroy",
        "relative_path = 'contactBatchAdd/import/reclaim.csv' AND deleted_at IS NOT NULL",
    ],
    "channel code rollback": [
        "dashboard/channelCode/store",
        "relative_path = 'channelCode/rollback.png' AND deleted_at IS NOT NULL",
    ],
    "medium update and delete": [
        "dashboard/medium/update",
        "dashboard/medium/destroy",
        "relative_path = 'medium/reclaim.png' AND deleted_at IS NOT NULL",
        "relative_path = 'medium/reclaim-new.png' AND deleted_at IS NOT NULL",
    ],
    "room welcome update and delete": [
        "dashboard/roomWelcome/update",
        "dashboard/roomWelcome/destroy",
        "relative_path = 'welcome/old.png' AND deleted_at IS NOT NULL",
        "relative_path = 'welcome/new.png' AND deleted_at IS NOT NULL",
    ],
    "work room auto pull update and rollback": [
        "dashboard/workRoomAutoPull/update",
        "dashboard/workRoomAutoPull/store",
        "relative_path = 'autopull/old.png' AND deleted_at IS NOT NULL",
        "relative_path = 'autopull/rollback.png' AND deleted_at IS NOT NULL",
    ],
    "room tag pull delete": [
        "dashboard/roomTagPull/destroy",
        "relative_path = 'roomtag/room.png' AND deleted_at IS NOT NULL",
    ],
    "batch send delete": [
        "dashboard/contactMessageBatchSend/destroy",
        "dashboard/roomMessageBatchSend/destroy",
        "relative_path = 'batch/contact.png' AND deleted_at IS NOT NULL",
        "relative_path = 'batch/room.png' AND deleted_at IS NOT NULL",
    ],
    "work fission delete": [
        "dashboard/workFission/destroy",
        "relative_path LIKE 'fission/%' AND deleted_at IS NOT NULL",
    ],
    "lottery delete": [
        "dashboard/lottery/destroy",
        "relative_path LIKE 'lottery/%' AND deleted_at IS NOT NULL",
    ],
    "radar delete": [
        "dashboard/radar/destroy",
        "relative_path LIKE 'radar/%' AND deleted_at IS NOT NULL",
    ],
    "room clock in delete": [
        "dashboard/roomClockIn/destroy",
        "relative_path LIKE 'clockIn/%' AND deleted_at IS NOT NULL",
    ],
    "room fission invite and delete": [
        "dashboard/roomFission/invite",
        "dashboard/roomFission/destroy",
        "relative_path = 'roomFission/invite.png' AND deleted_at IS NOT NULL",
        "relative_path LIKE 'roomFission/%' AND deleted_at IS NOT NULL",
    ],
    "room infinite pull delete": [
        "dashboard/roomInfinitePull/destroy",
        "relative_path LIKE 'roomInfinite/%' AND deleted_at IS NOT NULL",
    ],
    "shop code update page setting and delete": [
        "dashboard/shopCode/pageSet",
        "dashboard/shopCode/update",
        "dashboard/shopCode/destroy",
        "relative_path IN ('shopCode/page-logo-old.png', 'shopCode/page-poster-old.png') AND deleted_at IS NOT NULL",
        "relative_path = 'shopCode/employee-old.png' AND deleted_at IS NOT NULL",
        "relative_path LIKE 'shopCode/%' AND deleted_at IS NOT NULL",
    ],
}

for group, tokens in groups.items():
    for token in tokens:
        if token not in smoke:
            fail(f"scripts/smoke_saas_storage_reclaim.sh: {group} missing token {token!r}")

if failures:
    print("SaaS storage reclaim coverage audit failed:", file=sys.stderr)
    for failure in failures:
        print(f"- {failure}", file=sys.stderr)
    raise SystemExit(1)

print(f"SaaS storage reclaim coverage audit passed: functions={len(expected_functions)} groups={len(groups)}")
for group in sorted(groups):
    print(f"  {group}")
PY
