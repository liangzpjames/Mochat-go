#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

python3 - "$(pwd)" <<'PY'
from pathlib import Path
import re
import sys

repo = Path(sys.argv[1])

expected = {
    "employee-apply": "scripts/smoke_employee_apply_worker.sh",
    "wework-callback": "scripts/smoke_wework_callback_worker.sh",
    "contact-welcome": "scripts/smoke_wework_callback_worker.sh",
    "async-file-upload": "scripts/smoke_async_file_upload_saas_usage.sh",
    "mark-tags": "scripts/smoke_mark_tags_worker.sh",
    "message-remind": "scripts/smoke_message_remind_worker.sh",
    "work-room-sync": "scripts/smoke_work_room_sync_worker.sh",
    "work-contact-sync": "scripts/smoke_work_contact_sync_worker.sh",
    "work-department-list": "scripts/smoke_work_department_list_worker.sh",
    "medium-media-id-update": "scripts/smoke_media_id_update_worker.sh",
    "employee-statistic-apply": "scripts/smoke_employee_statistic_worker.sh",
}

failures = []
for queue, script in expected.items():
    path = repo / script
    if not path.is_file():
        failures.append(f"{queue}: missing smoke script {script}")
        continue
    text = path.read_text(encoding="utf-8")
    if "mochat_go_background_task_executions" not in text or queue not in text:
        failures.append(f"{queue}: smoke does not assert queue_item execution history")
    if "mochat_go_saas_usage_counters" not in text or "async_executions" not in text:
        failures.append(f"{queue}: smoke does not assert async_executions SaaS usage counter")
    tenant_scoped = any(
        re.search(pattern, text)
        for pattern in (
            r"tenant_id\s*=\s*1",
            r"tenant_id\s*=\s*\$TENANT_ID",
            r"tenant_id\s*=\s*\$TENANT_[A-Z]+_ID",
        )
    )
    if not tenant_scoped:
        failures.append(f"{queue}: smoke does not assert tenant-scoped execution or usage")

if failures:
    for failure in failures:
        print(f"FAIL {failure}")
    raise SystemExit(1)

print("worker SaaS usage assertion audit passed")
PY
