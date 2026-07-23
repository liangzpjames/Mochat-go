#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

SOURCE_ROOT="${MOCHAT_SOURCE_ROOT:-../mochat}"
EMBEDDED_MANIFEST="${MOCHAT_EMBEDDED_MANIFEST:-internal/server/compat_manifest_embedded.json}"
AUDIT_SOURCE="${MOCHAT_QUEUE_AUDIT_SOURCE:-auto}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-queue-coverage.XXXXXX")"

cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT INT TERM

MANIFEST_PATH="$EMBEDDED_MANIFEST"
SOURCE_LABEL="embedded:${EMBEDDED_MANIFEST}"

case "$AUDIT_SOURCE" in
  auto|php|embedded)
    ;;
  *)
    echo "unknown MOCHAT_QUEUE_AUDIT_SOURCE=$AUDIT_SOURCE" >&2
    echo "valid values: auto, php, embedded" >&2
    exit 2
    ;;
esac

if [ "$AUDIT_SOURCE" != "embedded" ] && [ -n "${MOCHAT_SOURCE_ROOT:-}" ] && [ ! -d "$SOURCE_ROOT/api-server" ]; then
  echo "MoChat PHP source root not found: $SOURCE_ROOT" >&2
  echo "Set MOCHAT_SOURCE_ROOT to a checkout that contains api-server/." >&2
  exit 1
fi

if [ "$AUDIT_SOURCE" = "php" ] && [ ! -d "$SOURCE_ROOT/api-server" ]; then
  echo "MoChat PHP source root not found: $SOURCE_ROOT" >&2
  echo "Set MOCHAT_SOURCE_ROOT to a checkout that contains api-server/ or use MOCHAT_QUEUE_AUDIT_SOURCE=embedded." >&2
  exit 1
fi

if [ "$AUDIT_SOURCE" != "embedded" ] && [ -d "$SOURCE_ROOT/api-server" ]; then
  env -u GOROOT go run ./cmd/mochat-inventory \
    -source-root "$SOURCE_ROOT" \
    -out "$WORK_DIR" \
    -source-revision "queue-coverage-audit" >/dev/null
  MANIFEST_PATH="$WORK_DIR/compat_manifest.json"
  SOURCE_LABEL="php:${SOURCE_ROOT}"
elif [ ! -f "$EMBEDDED_MANIFEST" ]; then
  echo "embedded manifest not found: $EMBEDDED_MANIFEST" >&2
  exit 1
fi

python3 - "$MANIFEST_PATH" "$SOURCE_LABEL" "$(pwd)" <<'PY'
from collections import Counter, defaultdict
import json
import os
import re
import sys

manifest_path, source_label, repo_root = sys.argv[1], sys.argv[2], sys.argv[3]
with open(manifest_path, encoding="utf-8") as fh:
    manifest = json.load(fh)

expected = [
    ("callback", "api-server/app/core/corp/src/QueueService/WeWorkCallback.php", "wework-callback", "WeWorkCallbackQueueDescriptor", "MOCHAT_GO_ENABLE_WEWORK_CALLBACK_WORKER", "scripts/smoke_wework_callback_worker.sh", "internal/dashboard/wework_callback_worker.go", "NewWeWorkCallbackWorker"),
    ("employee", "api-server/app/core/work-employee/src/QueueService/EmployeeApply.php", "employee-apply", "EmployeeApplyQueueDescriptor", "MOCHAT_GO_ENABLE_EMPLOYEE_APPLY_WORKER", "scripts/smoke_employee_apply_worker.sh", "internal/dashboard/employee_apply_worker.go", "NewEmployeeApplyWorker"),
    ("welcome", "api-server/app/core/work-contact/src/QueueService/SendWelcome.php", "contact-welcome", "ContactWelcomeQueueDescriptor", "MOCHAT_GO_ENABLE_CONTACT_WELCOME_WORKER", "scripts/smoke_wework_callback_worker.sh", "internal/dashboard/contact_welcome_worker.go", "NewContactWelcomeWorker"),
    ("file", "api-server/app/core/common/src/QueueService/AsyncFileUpload.php", "async-file-upload", "AsyncFileUploadQueueDescriptor", "MOCHAT_GO_ENABLE_ASYNC_FILE_UPLOAD_WORKER", "scripts/smoke_async_file_upload_worker.sh", "internal/dashboard/async_file_upload_worker.go", "NewAsyncFileUploadWorker"),
    ("contact", "api-server/app/core/work-contact/src/QueueService/Tag/MarkTags.php", "mark-tags", "MarkTagsQueueDescriptor", "MOCHAT_GO_ENABLE_MARK_TAGS_WORKER", "scripts/smoke_mark_tags_worker.sh", "internal/dashboard/mark_tags_worker.go", "NewMarkTagsWorker"),
    ("remind", "api-server/app/core/work-agent/src/QueueService/MessageRemind.php", "message-remind", "MessageRemindQueueDescriptor", "MOCHAT_GO_ENABLE_MESSAGE_REMIND_WORKER", "scripts/smoke_message_remind_worker.sh", "internal/dashboard/message_remind_worker.go", "NewMessageRemindWorker"),
    ("remind", "api-server/app/core/work-agent/src/QueueService/MessageRemind.php", "message-remind", "MessageRemindQueueDescriptor", "MOCHAT_GO_ENABLE_MESSAGE_REMIND_WORKER", "scripts/smoke_message_remind_worker.sh", "internal/dashboard/message_remind_worker.go", "NewMessageRemindWorker"),
    ("remind", "api-server/app/core/work-agent/src/QueueService/MessageRemind.php", "message-remind", "MessageRemindQueueDescriptor", "MOCHAT_GO_ENABLE_MESSAGE_REMIND_WORKER", "scripts/smoke_message_remind_worker.sh", "internal/dashboard/message_remind_worker.go", "NewMessageRemindWorker"),
    ("room", "api-server/app/core/work-room/src/QueueService/UpdateApply.php", "work-room-sync", "WorkRoomSyncQueueDescriptor", "MOCHAT_GO_ENABLE_WORK_ROOM_SYNC_WORKER", "scripts/smoke_work_room_sync_worker.sh", "internal/dashboard/work_room_sync_worker.go", "NewWorkRoomSyncWorker"),
    ("room", "api-server/app/core/work-room/src/QueueService/UpdateCallback.php", "work-room-sync", "WorkRoomSyncQueueDescriptor", "MOCHAT_GO_ENABLE_WORK_ROOM_SYNC_WORKER", "scripts/smoke_work_room_sync_worker.sh", "internal/dashboard/work_room_sync_worker.go", "NewWorkRoomSyncWorker"),
    ("contact", "api-server/app/core/work-contact/src/QueueService/SyncContactApply.php", "work-contact-sync", "WorkContactSyncQueueDescriptor", "MOCHAT_GO_ENABLE_WORK_CONTACT_SYNC_WORKER", "scripts/smoke_work_contact_sync_worker.sh", "internal/dashboard/work_contact_sync_worker.go", "NewWorkContactSyncWorker"),
    ("contact", "api-server/app/core/work-contact/src/QueueService/AdminSynContactApply.php", "work-contact-sync", "WorkContactSyncQueueDescriptor", "MOCHAT_GO_ENABLE_WORK_CONTACT_SYNC_WORKER", "scripts/smoke_work_contact_sync_worker.sh", "internal/dashboard/work_contact_sync_worker.go", "NewWorkContactSyncWorker"),
    ("employee", "api-server/app/core/work-department/src/QueueService/ListApply.php", "work-department-list", "WorkDepartmentListQueueDescriptor", "MOCHAT_GO_ENABLE_WORK_DEPARTMENT_LIST_WORKER", "scripts/smoke_work_department_list_worker.sh", "internal/dashboard/work_department_list_worker.go", "NewWorkDepartmentListWorker"),
    ("file", "api-server/app/core/medium/src/Queue/MediaIdUpdateQueue.php", "medium-media-id-update", "MediumMediaIDUpdateQueueDescriptor", "MOCHAT_GO_ENABLE_MEDIA_ID_UPDATE_WORKER", "scripts/smoke_media_id_update_worker.sh", "internal/dashboard/medium_media_id_update_worker.go", "NewMediumMediaIDUpdateWorker"),
    ("employee", "api-server/app/core/work-employee/src/QueueService/EmployeeStatisticApply.php", "employee-statistic-apply", "EmployeeStatisticApplyQueueDescriptor", "MOCHAT_GO_ENABLE_EMPLOYEE_STATISTIC_WORKER", "scripts/smoke_employee_statistic_worker.sh", "internal/dashboard/employee_statistic_apply_worker.go", "NewEmployeeStatisticApplyWorker"),
]

expected_counter = Counter((pool, path) for pool, path, *_ in expected)
actual_counter = Counter((str(item.get("pool", "")), str(item.get("file", ""))) for item in manifest.get("async_queues", []))

failures = []
if actual_counter != expected_counter:
    missing = expected_counter - actual_counter
    extra = actual_counter - expected_counter
    for key, count in sorted(missing.items()):
        failures.append(f"missing async queue annotation {key} x{count}")
    for key, count in sorted(extra.items()):
        failures.append(f"unexpected async queue annotation {key} x{count}")

def read(rel):
    path = os.path.join(repo_root, rel)
    with open(path, encoding="utf-8") as fh:
        return fh.read()

registry = read("internal/dashboard/queue_registry.go")
config = read("internal/config/config.go")
config_test = read("internal/config/config_test.go")
store = read("internal/store/redis.go")
main = read("cmd/mochat-go/main.go")
docs = read("docs/standalone-gap.md")

registry_names = set(re.findall(r'QueueName\w+\s*=\s*"([^"]+)"', registry))
registry_body = re.search(r"func QueuePayloadRegistry\(\) \[\]QueuePayloadDescriptor \{(.*?)\n\}", registry, re.S)
registry_body_text = registry_body.group(1) if registry_body else ""

by_queue = {}
for pool, php_file, queue, descriptor, env, smoke, worker, start_symbol in expected:
    by_queue.setdefault(queue, {
        "descriptor": descriptor,
        "env": env,
        "smoke": smoke,
        "worker": worker,
        "start_symbol": start_symbol,
        "php_files": set(),
        "annotation_count": 0,
    })
    by_queue[queue]["php_files"].add(php_file)
    by_queue[queue]["annotation_count"] += 1

for queue, meta in sorted(by_queue.items()):
    if queue not in registry_names:
        failures.append(f"Go queue constant missing: {queue}")
    if meta["descriptor"] not in registry:
        failures.append(f"Go queue descriptor missing: {meta['descriptor']}")
    if meta["descriptor"] not in registry_body_text:
        failures.append(f"Go queue descriptor not registered: {meta['descriptor']}")
    if f'"mochat-go:{queue}"' not in registry:
        failures.append(f"Go queue source key missing: mochat-go:{queue}")
    if meta["descriptor"] not in store:
        failures.append(f"Redis store does not reference descriptor: {meta['descriptor']}")
    if meta["env"] != "MOCHAT_GO_ENABLE_CONTACT_WELCOME_WORKER":
        if meta["env"] not in config:
            failures.append(f"config env missing: {meta['env']}")
        if meta["env"] not in config_test:
            failures.append(f"config test env missing: {meta['env']}")
    if meta["start_symbol"] not in main:
        failures.append(f"cmd/mochat-go startup path does not construct worker: {meta['start_symbol']}")
    worker_path = os.path.join(repo_root, meta["worker"])
    if not os.path.isfile(worker_path):
        failures.append(f"worker file missing: {meta['worker']}")
    smoke_path = os.path.join(repo_root, meta["smoke"])
    if not os.path.isfile(smoke_path):
        failures.append(f"smoke script missing: {meta['smoke']}")
    else:
        smoke_text = read(meta["smoke"])
        if queue not in smoke_text and meta["descriptor"] not in smoke_text:
            failures.append(f"smoke script does not mention queue identity: {meta['smoke']} -> {queue}")
        if meta["env"] != "MOCHAT_GO_ENABLE_CONTACT_WELCOME_WORKER" and meta["env"] not in smoke_text:
            failures.append(f"smoke script does not enable env: {meta['smoke']} -> {meta['env']}")
    if meta["smoke"] not in docs:
        failures.append(f"standalone gap doc missing smoke script: {meta['smoke']}")

annotation_counts = defaultdict(int)
for _, _, queue, *_ in expected:
    annotation_counts[queue] += 1

if failures:
    print(f"queue annotation coverage source={source_label}")
    for failure in failures:
        print(f"FAIL {failure}")
    raise SystemExit(1)

print(f"queue annotation coverage source={source_label}")
print(f"php_async_annotations={sum(actual_counter.values())}")
print(f"go_semantic_queues={len(by_queue)}")
for queue in sorted(by_queue):
    files = ", ".join(sorted(by_queue[queue]["php_files"]))
    print(f"  {queue}: annotations={annotation_counts[queue]} php={files} smoke={by_queue[queue]['smoke']}")
print("queue annotation coverage passed")
PY
