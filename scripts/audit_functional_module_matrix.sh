#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

python3 - "$(pwd)" <<'PY'
import json
import re
import sys
from pathlib import Path

repo = Path(sys.argv[1])
failures = []


def read(rel: str) -> str:
    return (repo / rel).read_text(encoding="utf-8")


def fail(message: str) -> None:
    failures.append(message)


def route_matches(path: str, prefix: str) -> bool:
    path = path.replace("\\\\.", "\\.")
    prefix = prefix.replace("\\\\.", "\\.")
    if prefix == "/":
        return path == "/"
    return path == prefix or path.startswith(prefix + "/")


def file_has(rel: str, token: str) -> bool:
    path = repo / rel
    if not path.exists():
        return False
    return token in path.read_text(encoding="utf-8")


manifest = json.loads(read("internal/server/compat_manifest_embedded.json"))
routes = manifest.get("routes", [])
route_paths = [route.get("path", "") for route in routes]
unique_route_paths = sorted(set(route_paths))
server_go = read("internal/server/server.go")
runtime_route_keys = sorted(
    {
        f"{match.group(1)} {match.group(2)}"
        for match in re.finditer(r'"(GET|POST|PUT|DELETE|HEAD|PATCH)\s+([^"]+)"', server_go)
    }
)
runtime_route_paths = [key.split(" ", 1)[1] for key in runtime_route_keys]
unique_runtime_route_paths = sorted(set(runtime_route_paths))
acceptance = read("scripts/standalone_acceptance.sh")

modules = [
    {
        "name": "基础运行与静态入口",
        "prefixes": [
            "/",
            "/healthz",
            "/readyz",
            "/compat/status",
            "/compat/routes",
            "/favicon.ico",
            "/{wxVerifyTxt:WW_verify_[0-9a-zA-Z]{16}\\.txt$}",
        ],
        "sources": ["internal/server/server.go", "internal/frontend/static.go"],
        "smokes": ["smoke_standalone.sh", "standalone_stack_check.sh", "smoke_admin_core_dashboard.sh"],
    },
    {
        "name": "租户、账号、角色与菜单",
        "prefixes": [
            "/dashboard/agent",
            "/dashboard/chatTool",
            "/dashboard/common",
            "/dashboard/corp",
            "/dashboard/corpData",
            "/dashboard/external",
            "/dashboard/menu",
            "/dashboard/role",
            "/dashboard/statistic",
            "/dashboard/user",
            "/dashboard/workDepartment",
            "/dashboard/workEmployee",
            "/dashboard/workEmployeeDepartment",
        ],
        "sources": [
            "internal/dashboard/auth.go",
            "internal/dashboard/corp_admin.go",
            "internal/dashboard/menu_admin.go",
            "internal/dashboard/role_admin.go",
            "internal/dashboard/user_admin.go",
        ],
        "smokes": [
            "smoke_bootstrap_standalone.sh",
            "smoke_saas_tenant_isolation.sh",
            "smoke_admin_core_dashboard.sh",
            "smoke_standalone_compose_app.sh",
        ],
        "tokens": [
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'corps'"),
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'users'"),
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'agents'"),
        ],
    },
    {
        "name": "客户、客户群、标签与画像",
        "prefixes": [
            "/dashboard/contactField",
            "/dashboard/contactFieldPivot",
            "/dashboard/workContact",
            "/dashboard/workContactRoom",
            "/dashboard/workContactTag",
            "/dashboard/workContactTagGroup",
            "/dashboard/workRoom",
            "/dashboard/workRoomGroup",
        ],
        "sources": [
            "internal/dashboard/contact_field.go",
            "internal/dashboard/work_contact_update.go",
            "internal/dashboard/work_contact_tag_sync.go",
            "internal/dashboard/work_room_group.go",
        ],
        "smokes": [
            "smoke_admin_core_dashboard.sh",
            "smoke_work_contact_tag_remote_write.sh",
            "smoke_work_contact_sync_worker.sh",
            "smoke_work_room_sync_worker.sh",
            "smoke_sidebar_frontend_contact.sh",
        ],
        "tokens": [
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'contacts'"),
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'rooms'"),
        ],
    },
    {
        "name": "素材库与上传存储",
        "prefixes": ["/dashboard/medium", "/dashboard/mediumGroup", "/sidebar/common", "/sidebar/medium", "/sidebar/mediumGroup"],
        "sources": [
            "internal/dashboard/common_upload.go",
            "internal/dashboard/medium.go",
            "internal/dashboard/medium_group.go",
            "internal/dashboard/medium_media_id_update_worker.go",
        ],
        "smokes": [
            "smoke_admin_core_dashboard.sh",
            "smoke_sidebar_frontend_contact.sh",
            "smoke_media_id_update_worker.sh",
            "smoke_media_id_update_cron.sh",
            "smoke_saas_storage_reclaim.sh",
        ],
        "tokens": [
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'storage_mb'"),
            ("scripts/audit_saas_storage_reclaim_coverage.sh", "medium update and delete"),
        ],
    },
    {
        "name": "企微回调、通讯录同步与异步队列",
        "prefixes": ["/weWork/callback"],
        "sources": [
            "internal/dashboard/wework_callback.go",
            "internal/dashboard/wework_callback_worker.go",
            "internal/dashboard/work_contact_sync_worker.go",
            "internal/dashboard/work_department_list_worker.go",
        ],
        "smokes": [
            "smoke_wework_callback_worker.sh",
            "smoke_work_contact_sync_worker.sh",
            "smoke_work_department_list_worker.sh",
            "smoke_employee_apply_worker.sh",
        ],
        "tokens": [
            ("scripts/audit_queue_annotation_coverage.sh", "work-contact-sync"),
            ("scripts/audit_wework_callback_event_coverage.sh", "process_events"),
        ],
    },
    {
        "name": "微信开放平台与公众号",
        "prefixes": ["/dashboard/officialAccount", "/dashboard/{appId}", "/load/{params?}"],
        "sources": [
            "internal/dashboard/official_account.go",
            "internal/dashboard/official_account_authorization.go",
            "internal/dashboard/official_account_callback.go",
            "internal/dashboard/operation_load_oauth.go",
        ],
        "smokes": ["smoke_official_account_ticket.sh", "smoke_operation_frontend_work_fission.sh"],
        "tokens": [("scripts/smoke_saas_quota_enforcement.sh", "metric = 'official_accounts'")],
    },
    {
        "name": "侧边栏工作台",
        "prefixes": [
            "/sidebar/agent",
            "/sidebar/contactFieldPivot",
            "/sidebar/contactProcessStatus",
            "/sidebar/contactBatchAdd",
            "/sidebar/contactSop",
            "/sidebar/roomSop",
            "/sidebar/workContact",
            "/sidebar/workContactTag",
            "/sidebar/workContactTagGroup",
            "/sidebar/workRoom",
            "/sidebar/wxJsSdk",
        ],
        "sources": ["internal/dashboard/sidebar_agent.go", "internal/dashboard/session_bridge.go"],
        "smokes": ["smoke_sidebar_frontend_contact.sh", "smoke_frontend_static_browser.sh"],
    },
    {
        "name": "渠道活码",
        "prefixes": ["/dashboard/channelCode", "/dashboard/channelCodeGroup"],
        "sources": ["internal/dashboard/channel_code.go", "internal/dashboard/channel_code_routes.go", "internal/dashboard/channel_code_cron.go"],
        "smokes": ["smoke_channel_code_dashboard.sh", "smoke_channel_code_cron.sh", "smoke_saas_storage_reclaim.sh"],
        "tokens": [
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'channel_codes'"),
            ("scripts/audit_saas_storage_reclaim_coverage.sh", "channel code rollback"),
        ],
    },
    {
        "name": "客户转接",
        "prefixes": ["/dashboard/contactTransfer"],
        "sources": ["internal/dashboard/contact_transfer.go", "internal/dashboard/contact_transfer_cron.go"],
        "smokes": ["smoke_contact_transfer_dashboard.sh", "smoke_transfer_state_refresh_cron.sh"],
    },
    {
        "name": "好友欢迎语与入群欢迎语",
        "prefixes": ["/dashboard/greeting", "/dashboard/roomWelcome"],
        "sources": ["internal/dashboard/greeting.go", "internal/dashboard/room_welcome.go", "internal/dashboard/room_welcome_wecom.go"],
        "smokes": ["smoke_greeting_dashboard.sh", "smoke_room_welcome_dashboard.sh", "smoke_saas_storage_reclaim.sh"],
        "tokens": [("scripts/audit_saas_storage_reclaim_coverage.sh", "room welcome update and delete")],
    },
    {
        "name": "客户群发与客户群群发",
        "prefixes": ["/dashboard/contactMessageBatchSend", "/dashboard/roomMessageBatchSend"],
        "sources": ["internal/dashboard/contact_message_batch_send.go", "internal/dashboard/room_message_batch_send.go", "internal/dashboard/batch_send_schedule_cron.go"],
        "smokes": [
            "smoke_contact_message_batch_send_dashboard.sh",
            "smoke_room_message_batch_send_dashboard.sh",
            "smoke_contact_batch_send_cron.sh",
            "smoke_room_batch_send_cron.sh",
            "smoke_saas_storage_reclaim.sh",
        ],
        "tokens": [
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'contact_message_batches'"),
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'room_message_batches'"),
            ("scripts/audit_saas_storage_reclaim_coverage.sh", "batch send delete"),
        ],
    },
    {
        "name": "任务宝裂变与 operation 前端",
        "prefixes": ["/dashboard/workFission", "/operation/auth", "/operation/openUserInfo", "/operation/workFission"],
        "sources": [
            "internal/dashboard/work_fission.go",
            "internal/dashboard/work_fission_oauth.go",
            "internal/dashboard/work_fission_operation.go",
        ],
        "smokes": ["smoke_work_fission_dashboard.sh", "smoke_operation_frontend_work_fission.sh", "smoke_saas_storage_reclaim.sh"],
        "tokens": [
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'work_fissions'"),
            ("scripts/audit_saas_storage_reclaim_coverage.sh", "work fission delete"),
        ],
    },
    {
        "name": "标签建群",
        "prefixes": ["/dashboard/roomTagPull", "/roomTagPull/contactDetail", "/roomTagPull/clientDetails"],
        "sources": ["internal/dashboard/room_tag_pull.go", "internal/dashboard/room_tag_pull_cron.go", "internal/dashboard/room_tag_pull_page.go"],
        "smokes": ["smoke_room_tag_pull_dashboard.sh", "smoke_room_tag_pull_cron.sh", "smoke_saas_storage_reclaim.sh"],
        "tokens": [
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'room_tag_pulls'"),
            ("scripts/audit_saas_storage_reclaim_coverage.sh", "room tag pull delete"),
        ],
    },
    {
        "name": "自动拉群",
        "prefixes": ["/dashboard/workRoomAutoPull"],
        "sources": ["internal/dashboard/work_room_auto_pull.go", "internal/dashboard/work_room_auto_pull_wecom.go"],
        "smokes": ["smoke_work_room_auto_pull_dashboard.sh", "smoke_saas_storage_reclaim.sh"],
        "tokens": [
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'work_room_auto_pulls'"),
            ("scripts/audit_saas_storage_reclaim_coverage.sh", "work room auto pull update and rollback"),
        ],
    },
    {
        "name": "批量加好友",
        "prefixes": ["/dashboard/contactBatchAdd"],
        "sources": ["internal/dashboard/contact_batch_add.go", "internal/dashboard/contact_batch_add_dashboard.go"],
        "smokes": ["smoke_contact_batch_add_dashboard.sh", "smoke_saas_storage_reclaim.sh"],
        "tokens": [("scripts/audit_saas_storage_reclaim_coverage.sh", "contact batch add import")],
    },
    {
        "name": "敏感词与会话监控",
        "prefixes": [
            "/dashboard/sensitiveWords",
            "/dashboard/sensitiveWord",
            "/dashboard/sensitiveWordGroup",
            "/dashboard/sensitiveWordsMonitor",
        ],
        "sources": ["internal/dashboard/sensitive_word.go", "internal/dashboard/sensitive_word_page.go", "internal/dashboard/sensitive_word_cron.go"],
        "smokes": ["smoke_sensitive_word_dashboard.sh", "smoke_sensitive_word_monitor_cron.sh", "smoke_work_message_archive_sync_cron.sh"],
        "tokens": [("scripts/smoke_saas_quota_enforcement.sh", "metric = 'sensitive_words'")],
    },
    {
        "name": "个人和群 SOP",
        "prefixes": ["/dashboard/contactSop", "/dashboard/roomSop"],
        "sources": ["internal/dashboard/contact_sop.go", "internal/dashboard/room_sop.go", "internal/dashboard/sop_dashboard_page.go", "internal/dashboard/sop_log_cron.go"],
        "smokes": ["smoke_sop_dashboard.sh", "smoke_sidebar_frontend_contact.sh"],
        "tokens": [
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'contact_sops'"),
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'room_sops'"),
        ],
    },
    {
        "name": "门店活码",
        "prefixes": ["/dashboard/shopCode", "/operation/shopCode"],
        "sources": ["internal/dashboard/shop_code_dashboard.go", "internal/dashboard/shop_code_page.go", "internal/dashboard/operation_h5.go"],
        "smokes": ["smoke_shop_code_dashboard.sh", "smoke_saas_storage_reclaim.sh"],
        "tokens": [
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'shop_codes'"),
            ("scripts/audit_saas_storage_reclaim_coverage.sh", "shop code update page setting and delete"),
        ],
    },
    {
        "name": "互动雷达",
        "prefixes": ["/dashboard/radar"],
        "sources": ["internal/dashboard/radar_dashboard.go", "internal/dashboard/radar_page.go"],
        "smokes": ["smoke_radar_dashboard.sh", "smoke_saas_storage_reclaim.sh"],
        "tokens": [
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'radars'"),
            ("scripts/audit_saas_storage_reclaim_coverage.sh", "radar delete"),
        ],
    },
    {
        "name": "自动标签与会话存档",
        "prefixes": ["/dashboard/autoTag", "/Task/AutoTag", "/dashboard/Task", "/dashboard/workMessage", "/dashboard/workMessageConfig"],
        "sources": ["internal/dashboard/auto_tag_dashboard.go", "internal/dashboard/work_message_archive_sync_cron.go", "internal/dashboard/mark_tags_worker.go"],
        "smokes": ["smoke_auto_tag_dashboard.sh", "smoke_auto_tag_keyword_task.sh", "smoke_work_message_archive_sync_cron.sh", "smoke_mark_tags_worker.sh"],
        "tokens": [("scripts/audit_queue_annotation_coverage.sh", "mark-tags")],
    },
    {
        "name": "抽奖活动",
        "prefixes": ["/dashboard/lottery", "/operation/lottery"],
        "sources": ["internal/dashboard/lottery_dashboard.go", "internal/dashboard/lottery_page.go", "internal/dashboard/operation_h5.go"],
        "smokes": ["smoke_lottery_dashboard.sh", "smoke_saas_storage_reclaim.sh"],
        "tokens": [
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'lotteries'"),
            ("scripts/audit_saas_storage_reclaim_coverage.sh", "lottery delete"),
        ],
    },
    {
        "name": "群裂变",
        "prefixes": ["/dashboard/roomFission", "/operation/roomFission"],
        "sources": ["internal/dashboard/room_fission_dashboard.go", "internal/dashboard/room_fission_page.go", "internal/dashboard/operation_h5.go"],
        "smokes": ["smoke_room_fission_dashboard.sh", "smoke_dashboard_frontend_login.sh", "smoke_saas_storage_reclaim.sh"],
        "tokens": [
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'room_fissions'"),
            ("scripts/audit_saas_storage_reclaim_coverage.sh", "room fission invite and delete"),
        ],
    },
    {
        "name": "群打卡",
        "prefixes": ["/dashboard/roomClockIn", "/dashboard/clockIn", "/operation/roomClockIn"],
        "sources": ["internal/dashboard/room_clock_in_dashboard.go", "internal/dashboard/room_clock_in_page.go", "internal/dashboard/operation_h5.go"],
        "smokes": ["smoke_room_clock_in_dashboard.sh", "smoke_saas_storage_reclaim.sh"],
        "tokens": [
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'room_clock_ins'"),
            ("scripts/audit_saas_storage_reclaim_coverage.sh", "room clock in delete"),
        ],
    },
    {
        "name": "群质检",
        "prefixes": ["/dashboard/roomQuality"],
        "sources": ["internal/dashboard/room_quality_dashboard.go", "internal/dashboard/room_quality_page.go"],
        "smokes": ["smoke_room_quality_dashboard.sh"],
        "tokens": [("scripts/smoke_saas_quota_enforcement.sh", "metric = 'room_qualities'")],
    },
    {
        "name": "群日历",
        "prefixes": ["/dashboard/roomCalendar"],
        "sources": ["internal/dashboard/room_calendar_dashboard.go", "internal/dashboard/room_calendar_page.go"],
        "smokes": ["smoke_room_calendar_dashboard.sh"],
        "tokens": [("scripts/smoke_saas_quota_enforcement.sh", "metric = 'room_calendars'")],
    },
    {
        "name": "客户群提醒",
        "prefixes": ["/dashboard/roomRemind", "/dashboard/task/roomRemind"],
        "sources": ["internal/dashboard/room_remind_dashboard.go", "internal/dashboard/room_remind_page.go"],
        "smokes": ["smoke_room_remind_dashboard.sh", "smoke_message_remind_worker.sh"],
        "tokens": [("scripts/smoke_saas_quota_enforcement.sh", "metric = 'room_reminds'")],
    },
    {
        "name": "无限拉群",
        "prefixes": ["/dashboard/roomInfinitePull", "/operation/roomInfinitePull"],
        "sources": ["internal/dashboard/room_infinite_pull_dashboard.go", "internal/dashboard/room_infinite_pull_page.go", "internal/dashboard/operation_h5.go"],
        "smokes": ["smoke_room_infinite_pull_dashboard.sh", "smoke_saas_storage_reclaim.sh"],
        "tokens": [
            ("scripts/smoke_saas_quota_enforcement.sh", "metric = 'room_infinite_pulls'"),
            ("scripts/audit_saas_storage_reclaim_coverage.sh", "room infinite pull delete"),
        ],
    },
    {
        "name": "SaaS 告警管理",
        "prefixes": ["/dashboard/saasAlert"],
        "sources": ["internal/dashboard/saas_alert.go", "internal/dashboard/saas_alert_page.go", "internal/dashboard/saas_alert_setting.go"],
        "smokes": ["smoke_employee_apply_worker.sh", "smoke_saas_alert_notification_cron.sh", "smoke_saas_alert_setting_dispatch.sh"],
        "tokens": [("scripts/smoke_employee_apply_worker.sh", "dashboard/saasAlert/index")],
    },
    {
        "name": "SaaS 总后台与平台运营",
		"prefixes": ["/dashboard/saasAdmin", "/dashboard/saasBilling", "/api/saas/v1", "/webhooks/saas/payment", "/webhooks/saas/domain-delivery", "/security"],
        "sources": [
            "internal/dashboard/saas_admin.go",
            "internal/dashboard/saas_admin_access.go",
            "internal/dashboard/saas_admin_approval.go",
			"internal/dashboard/saas_admin_approval_governance.go",
			"internal/dashboard/saas_admin_approval_reminder_cron.go",
			"internal/dashboard/saas_admin_system_health.go",
			"internal/dashboard/saas_branding.go",
			"internal/dashboard/external_tenant.go",
			"internal/dashboard/identity_login_page.go",
			"internal/dashboard/saas_admin_system_health_cron.go",
			"internal/dashboard/saas_service_account.go",
			"internal/dashboard/saas_backup_recovery.go",
            "internal/dashboard/saas_admin_page.go",
            "internal/dashboard/saas_admin_notification_health.go",
            "internal/dashboard/saas_admin_notification_slo.go",
            "internal/dashboard/saas_admin_notification_health_recovery.go",
            "internal/dashboard/saas_admin_notification_health_recovery_cron.go",
            "internal/dashboard/saas_admin_subscription.go",
            "internal/dashboard/saas_admin_subscription_reconcile_cron.go",
            "internal/dashboard/saas_admin_payment.go",
            "internal/dashboard/saas_admin_refund.go",
            "internal/dashboard/saas_admin_invoice.go",
            "internal/dashboard/saas_admin_settlement.go",
            "internal/dashboard/saas_admin_settlement_sync.go",
            "internal/dashboard/saas_payment_settlement_sync.go",
            "internal/dashboard/saas_billing.go",
            "internal/dashboard/saas_billing_page.go",
            "internal/dashboard/saas_invoice.go",
            "internal/dashboard/saas_payment_dunning_cron.go",
            "internal/store/saas_payment.go",
            "internal/store/saas_refund.go",
            "internal/store/saas_invoice.go",
            "internal/store/saas_settlement.go",
            "internal/store/saas_settlement_sync.go",
            "internal/store/saas_admin_access.go",
            "internal/store/saas_admin_approval.go",
			"internal/store/saas_admin_approval_governance.go",
			"internal/store/saas_admin_system_health.go",
			"internal/dashboard/saas_admin_audit_integrity.go",
			"internal/dashboard/saas_admin_audit_integrity_cron.go",
			"internal/store/saas_audit_integrity.go",
			"internal/dashboard/saas_admin_audit_anchor.go",
			"internal/store/saas_audit_anchor.go",
			"internal/saasauditanchor/manager.go",
			"internal/saasauditanchor/cron.go",
			"internal/store/saas_branding.go",
			"internal/store/saas_service_account.go",
			"internal/store/saas_backup_recovery.go",
			"internal/saasbackup/manager.go",
			"internal/saasbackup/artifact.go",
			"internal/saasbackup/cron.go",
			"internal/saascompliance/manager.go",
			"internal/saascompliance/artifact.go",
			"internal/saascompliance/inventory.go",
			"internal/saascompliance/cron.go",
			"internal/store/saas_compliance.go",
			"internal/identitysecurity/manager.go",
			"internal/clientip/resolver.go",
			"internal/identitysecurity/cron.go",
			"internal/dashboard/saas_identity_security.go",
			"internal/store/saas_identity_security.go",
			"internal/dashboard/saas_tenant_domain.go",
			"internal/dashboard/saas_tenant_domain_delivery.go",
			"internal/dashboard/saas_tenant_domain_delivery_http.go",
			"internal/store/saas_tenant_domain.go",
			"internal/store/saas_tenant_domain_delivery.go",
			"internal/dashboard/saas_release_readiness.go",
			"internal/store/saas_release_readiness.go",
			"internal/dashboard/saas_admin_wecom_credential.go",
			"internal/store/wecom_credentials.go",
			"internal/wecomcredentials/manager.go",
        ],
        "smokes": [
            "smoke_saas_admin_dashboard.sh",
            "smoke_saas_admin_access_rbac.sh",
            "smoke_saas_admin_approvals.sh",
			"smoke_saas_admin_approval_governance.sh",
			"smoke_saas_package_definition_approval.sh",
			"smoke_saas_tenant_package_assignment_approval.sh",
			"smoke_saas_tenant_provision_approval.sh",
			"smoke_saas_tenant_renewal_approval.sh",
			"smoke_saas_subscription_transition_approval.sh",
			"smoke_saas_invoice_issue_approval.sh",
			"smoke_saas_payment_order_create_approval.sh",
			"smoke_saas_payment_settlement_close_approval.sh",
			"smoke_saas_payment_settlement_reopen_approval.sh",
			"smoke_saas_payment_settlement_resolve_approval.sh",
			"smoke_saas_admin_system_health.sh",
			"smoke_saas_audit_integrity.sh",
			"smoke_saas_service_accounts.sh",
			"smoke_saas_backup_recovery.sh",
			"smoke_saas_compliance_lifecycle.sh",
			"smoke_saas_identity_security.sh",
			"smoke_saas_branding.sh",
			"smoke_saas_tenant_domains.sh",
			"smoke_saas_release_readiness.sh",
			"smoke_wecom_credential_encryption.sh",
            "smoke_saas_notification_slo.sh",
            "smoke_saas_notification_health_recovery_cron.sh",
            "smoke_saas_subscription_lifecycle.sh",
            "smoke_saas_payment_collection.sh",
            "smoke_saas_payment_refunds.sh",
            "smoke_saas_billing_invoices.sh",
            "smoke_saas_payment_settlements.sh",
            "smoke_saas_payment_settlement_sync.sh",
            "smoke_standalone_compose_app.sh",
        ],
        "tokens": [
            ("scripts/smoke_saas_admin_dashboard.sh", "notificationHealthRecovery"),
            ("scripts/smoke_saas_notification_slo.sh", "notificationSlo"),
            ("scripts/smoke_saas_notification_health_recovery_cron.sh", "cron-saas-notification-health-recovery"),
            ("scripts/smoke_saas_subscription_lifecycle.sh", "subscriptionTransition"),
            ("scripts/smoke_saas_payment_collection.sh", "/webhooks/saas/payment"),
            ("scripts/smoke_saas_payment_collection.sh", "payment_failed_reminder"),
            ("deploy/standalone/migrations/0040_saas_payment_collection.up.sql", "mochat_go_saas_payment_orders"),
            ("scripts/smoke_saas_payment_refunds.sh", "refund.succeeded"),
            ("scripts/smoke_saas_payment_refunds.sh", "netPaidAmountCents"),
            ("deploy/standalone/migrations/0041_saas_payment_refunds.up.sql", "mochat_go_saas_payment_refunds"),
            ("scripts/smoke_saas_billing_invoices.sh", "netInvoicedCents"),
            ("scripts/smoke_saas_billing_invoices.sh", "/dashboard/saasBilling/summary"),
            ("deploy/standalone/migrations/0042_saas_billing_invoices.up.sql", "mochat_go_saas_invoice_documents"),
            ("scripts/smoke_saas_payment_settlements.sh", "paymentSettlementReconcile"),
            ("scripts/smoke_saas_payment_settlements.sh", "identifierConflictCount"),
            ("deploy/standalone/migrations/0043_saas_payment_settlements.up.sql", "mochat_go_saas_payment_settlement_entries"),
            ("scripts/smoke_saas_payment_settlement_sync.sh", "paymentSettlementSyncRuns"),
            ("scripts/smoke_saas_payment_settlement_sync.sh", "cron-saas-payment-settlement-sync"),
            ("deploy/standalone/migrations/0044_saas_payment_settlement_sync.up.sql", "mochat_go_saas_payment_settlement_sync_runs"),
            ("scripts/smoke_saas_admin_access_rbac.sh", "platform.access.manage"),
            ("scripts/smoke_saas_admin_access_rbac.sh", "assignment-version-conflict"),
            ("deploy/standalone/migrations/0045_saas_admin_rbac.up.sql", "mochat_go_saas_admin_user_roles"),
            ("scripts/smoke_saas_admin_approvals.sh", "approvalExecute"),
            ("scripts/smoke_saas_admin_approvals.sh", "requester-review-forbidden"),
            ("deploy/standalone/migrations/0046_saas_admin_approvals.up.sql", "mochat_go_saas_admin_approval_events"),
			("deploy/standalone/migrations/0047_saas_admin_approval_governance.up.sql", "mochat_go_saas_admin_approval_delegations"),
			("deploy/standalone/migrations/0048_saas_admin_system_health.up.sql", "mochat_go_saas_admin_system_incidents"),
			("deploy/standalone/migrations/0049_saas_service_accounts.up.sql", "mochat_go_saas_service_account_keys"),
			("deploy/standalone/migrations/0050_saas_backup_recovery.up.sql", "mochat_go_saas_restore_drills"),
			("deploy/standalone/migrations/0051_saas_backup_resilience.up.sql", "require_offsite_replica"),
			("deploy/standalone/migrations/0052_saas_compliance_lifecycle.up.sql", "mochat_go_saas_erasure_requests"),
			("deploy/standalone/migrations/0053_saas_identity_security.up.sql", "mochat_go_saas_identity_sessions"),
			("deploy/standalone/migrations/0054_saas_branding_profiles.up.sql", "mochat_go_saas_branding_profiles"),
			("deploy/standalone/migrations/0055_saas_tenant_domains.up.sql", "mochat_go_saas_tenant_domains"),
			("deploy/standalone/migrations/0056_saas_domain_delivery.up.sql", "mochat_go_saas_tenant_domain_delivery_jobs"),
			("deploy/standalone/migrations/0057_saas_release_readiness.up.sql", "mochat_go_saas_release_candidates"),
			("deploy/standalone/migrations/0058_saas_release_evidence_integrity.up.sql", "artifact_sha256"),
			("deploy/standalone/migrations/0059_saas_service_account_rate_limits.up.sql", "mochat_go_saas_service_account_usage_daily"),
			("deploy/standalone/migrations/0062_saas_audit_integrity.up.sql", "mochat_go_saas_admin_audit_verifications"),
			("deploy/standalone/migrations/0063_saas_audit_anchor_signatures.up.sql", "mochat_go_saas_admin_audit_anchor_checkpoints"),
			("deploy/standalone/migrations/0064_saas_audit_anchor_remote_immutability.up.sql", "remote_object_key"),
			("deploy/standalone/migrations/0065_saas_service_account_key_pepper_ring.up.sql", "hash_key_id"),
			("deploy/standalone/migrations/0066_saas_alert_credential_encryption.up.sql", "webhook_credentials_ciphertext"),
			("deploy/standalone/migrations/0067_wecom_credential_encryption.up.sql", "wecom_credentials_ciphertext"),
			("deploy/standalone/migrations/0068_wechat_open_credential_encryption.up.sql", "wechat_credentials_ciphertext"),
			("deploy/standalone/migrations/0069_saas_release_candidate_approval.up.sql", "release.candidate.gate"),
			("deploy/standalone/migrations/0070_saas_approval_policy_change_guard.up.sql", "approval.policy.update"),
			("deploy/standalone/migrations/0071_saas_backup_policy_change_guard.up.sql", "backup.policy.update"),
			("deploy/standalone/migrations/0072_saas_backup_cleanup_saga.up.sql", "backup.retention.cleanup"),
			("deploy/standalone/migrations/0073_saas_compliance_export_deletion_saga.up.sql", "compliance.export.delete"),
			("deploy/standalone/migrations/0074_saas_compliance_legal_hold_release_guard.up.sql", "compliance.legal_hold.release"),
			("deploy/standalone/migrations/0075_saas_compliance_policy_change_guard.up.sql", "compliance.policy.update"),
			("deploy/standalone/migrations/0076_saas_identity_policy_change_guard.up.sql", "identity.policy.update"),
			("deploy/standalone/migrations/0077_saas_tenant_disable_approval_guard.up.sql", "tenant.disable"),
			("deploy/standalone/migrations/0078_saas_critical_approval_policy_guard.up.sql", "payment.refund.create"),
			("deploy/standalone/migrations/0079_saas_service_account_key_revoke_guard.up.sql", "service_account.key.revoke"),
			("deploy/standalone/migrations/0080_saas_service_account_update_guard.up.sql", "service_account.update"),
				("deploy/standalone/migrations/0081_saas_service_account_key_rotate_guard.up.sql", "service_account.key.rotate"),
				("deploy/standalone/migrations/0082_saas_service_account_create_guard.up.sql", "service_account.create"),
				("deploy/standalone/migrations/0083_saas_identity_mfa_reset_guard.up.sql", "identity.mfa.reset"),
				("deploy/standalone/migrations/0084_saas_package_definition_guard.up.sql", "package.upsert"),
				("scripts/smoke_saas_package_definition_approval.sh", "package-growth-stale-v1"),
				("scripts/smoke_saas_package_definition_approval.sh", "effect_applied_at IS NULL"),
				("deploy/standalone/migrations/0085_saas_tenant_package_assignment_guard.up.sql", "tenant.package.update"),
				("deploy/standalone/migrations/0086_saas_tenant_provision_approval_guard.up.sql", "tenant.provision"),
				("deploy/standalone/migrations/0087_saas_tenant_renewal_approval_guard.up.sql", "tenant.renewal"),
				("deploy/standalone/migrations/0088_saas_subscription_transition_approval_guard.up.sql", "tenant.subscription.transition"),
				("deploy/standalone/migrations/0089_saas_invoice_issue_approval_guard.up.sql", "billing.invoice.issue"),
				("deploy/standalone/migrations/0090_saas_payment_order_create_approval_guard.up.sql", "payment.order.create"),
					("deploy/standalone/migrations/0091_saas_payment_settlement_close_guard.up.sql", "payment.settlement.close"),
					("deploy/standalone/migrations/0092_saas_payment_settlement_reopen_guard.up.sql", "payment.settlement.reopen"),
				("scripts/smoke_saas_tenant_package_assignment_approval.sh", "tenant-package-991-stale"),
				("scripts/smoke_saas_tenant_package_assignment_approval.sh", "effect_applied_at IS NULL"),
				("scripts/smoke_saas_tenant_provision_approval.sh", "tenant-provision-task-stale-0086"),
				("scripts/smoke_saas_tenant_provision_approval.sh", "hasAdminPasswordHash"),
				("scripts/smoke_saas_tenant_provision_approval.sh", "saas.admin.task.apply"),
				("scripts/smoke_saas_tenant_renewal_approval.sh", "tenant-renewal-task-stale-0087"),
				("scripts/smoke_saas_tenant_renewal_approval.sh", "tenant-renewal-direct-stale-subscription-0087"),
				("scripts/smoke_saas_tenant_renewal_approval.sh", "saas.admin.task.apply"),
				("scripts/smoke_saas_subscription_transition_approval.sh", "subscription-transition-stale-version-0088"),
				("scripts/smoke_saas_subscription_transition_approval.sh", "subscription-transition-tenant-drift-0088"),
				("scripts/smoke_saas_subscription_transition_approval.sh", "tenant.subscription.transition"),
				("scripts/smoke_saas_subscription_transition_approval.sh", "effect_applied_at IS NULL"),
				("scripts/smoke_saas_invoice_issue_approval.sh", "invoice-issue-document-drift-0089"),
				("scripts/smoke_saas_invoice_issue_approval.sh", "invoice-issue-order-drift-0089"),
				("scripts/smoke_saas_invoice_issue_approval.sh", "billing.invoice.issue"),
				("scripts/smoke_saas_invoice_issue_approval.sh", "effect_applied_at IS NULL"),
				("scripts/smoke_saas_payment_order_create_approval.sh", "payment-order-approval-drift-0090"),
				("scripts/smoke_saas_payment_order_create_approval.sh", "package_limits_json"),
				("scripts/smoke_saas_payment_order_create_approval.sh", "effect_applied_at IS NULL"),
				("scripts/smoke_saas_payment_settlement_close_approval.sh", "settlement-close-drift-0091"),
				("scripts/smoke_saas_payment_settlement_close_approval.sh", "total_net_cents = total_net_cents + 1"),
			("scripts/smoke_saas_payment_settlement_close_approval.sh", "effect_applied_at IS NULL"),
			("scripts/smoke_saas_payment_settlement_reopen_approval.sh", "settlement-reopen-drift-0092"),
			("scripts/smoke_saas_payment_settlement_reopen_approval.sh", "close_reason = '审批外篡改关账原因'"),
			("scripts/smoke_saas_payment_settlement_reopen_approval.sh", "effect_applied_at IS NULL"),
			("scripts/smoke_saas_payment_settlement_resolve_approval.sh", "settlement-resolve-drift-0093"),
			("scripts/smoke_saas_payment_settlement_resolve_approval.sh", "审批外篡改差异说明"),
			("scripts/smoke_saas_payment_settlement_resolve_approval.sh", "effect_applied_at IS NULL"),
			("scripts/smoke_wecom_credential_encryption.sh", "/dashboard/saasAdmin/wecomCredentialProtection"),
			("scripts/smoke_official_account_ticket.sh", "/dashboard/saasAdmin/wechatOpenCredentialProtection"),
			("scripts/smoke_official_account_ticket.sh", "/dashboard/saasAdmin/wechatOpenCredentialRotation"),
			("scripts/smoke_official_account_ticket.sh", "wechat_open_credential_probe"),
			("scripts/smoke_wecom_credential_encryption.sh", "/dashboard/saasAdmin/wecomCredentialRotation"),
			("scripts/smoke_wecom_credential_encryption.sh", "wecom_credential_protection"),
			("scripts/smoke_saas_service_accounts.sh", "legacy-disabled-protection"),
			("scripts/smoke_saas_release_readiness.sh", "platform.release.manage"),
			("scripts/smoke_saas_release_readiness.sh", "sourceFingerprint"),
			("scripts/smoke_saas_release_readiness.sh", "artifactSha256"),
			("scripts/smoke_saas_release_readiness.sh", "status == \"ready\""),
			("scripts/smoke_saas_release_readiness.sh", "release-approval-executed"),
			("scripts/smoke_saas_tenant_domains.sh", "mochat-domain-verification="),
			("scripts/smoke_saas_tenant_domains.sh", "platform.domains.manage"),
			("scripts/smoke_saas_tenant_domains.sh", "/webhooks/saas/domain-delivery"),
			("scripts/smoke_saas_tenant_domains.sh", "domain_delivery_queue"),
			("scripts/smoke_saas_tenant_domains.sh", "sensitive-upstream-body-must-not-persist"),
			("scripts/smoke_saas_admin_system_health.sh", "cron-saas-admin-system-health"),
			("scripts/smoke_saas_admin_system_health.sh", "platform.system.manage"),
			("scripts/smoke_saas_audit_integrity.sh", "platform.audit.manage"),
			("scripts/smoke_saas_audit_integrity.sh", "tampered.audit.action"),
			("scripts/smoke_saas_audit_integrity.sh", "verify-audit-integrity"),
			("scripts/smoke_saas_audit_integrity.sh", "create-audit-anchor"),
			("scripts/smoke_saas_audit_integrity.sh", "AAN-ORPHAN-ROLLBACK-EVIDENCE"),
			("scripts/smoke_saas_service_accounts.sh", "/api/saas/v1/whoami"),
			("scripts/smoke_saas_service_accounts.sh", "platform.integrations.manage"),
			("scripts/smoke_saas_service_accounts.sh", "X-RateLimit-Remaining"),
			("scripts/smoke_saas_service_accounts.sh", "ip-forwarded-allowed"),
			("scripts/smoke_saas_identity_security.sh", "MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS"),
			("scripts/smoke_saas_backup_recovery.sh", "platform.backups.manage"),
			("scripts/smoke_saas_backup_recovery.sh", "/dashboard/saasAdmin/restoreDrill"),
			("scripts/smoke_saas_backup_recovery.sh", "tampered-offsite-replica"),
			("scripts/smoke_saas_compliance_lifecycle.sh", "tenant.data.erase"),
			("scripts/smoke_saas_compliance_lifecycle.sh", "COMPLIANCE-PII-982-SECRET"),
			("scripts/smoke_saas_compliance_lifecycle.sh", "complianceErasureSteps"),
			("scripts/smoke_saas_identity_security.sh", "platform.identity.manage"),
			("scripts/smoke_saas_identity_security.sh", "/dashboard/user/authMFA"),
			("scripts/smoke_saas_identity_security.sh", "recovery-login"),
			("scripts/smoke_saas_branding.sh", "platform.branding.manage"),
			("scripts/smoke_saas_branding.sh", "baseURL:\"/dashboard\""),
            ("scripts/smoke_standalone_compose_app.sh", "0037_saas_notification_health_index"),
        ],
    },
]

covered_manifest = set()
covered_runtime = set()
for module in modules:
    module_manifest_routes = []
    module_runtime_routes = []
    for prefix in module["prefixes"]:
        matched_manifest = [path for path in route_paths if route_matches(path, prefix)]
        matched_runtime = [path for path in runtime_route_paths if route_matches(path, prefix)]
        if not matched_manifest and not matched_runtime:
            fail(f"{module['name']}: manifest 和 Go 运行时迁移路由中都没有匹配前缀 {prefix}")
        module_manifest_routes.extend(matched_manifest)
        module_runtime_routes.extend(matched_runtime)
    covered_manifest.update(module_manifest_routes)
    covered_runtime.update(module_runtime_routes)

    for rel in module["sources"]:
        if not (repo / rel).exists():
            fail(f"{module['name']}: 缺少 Go 源码证据 {rel}")

    for smoke in module["smokes"]:
        rel = f"scripts/{smoke}" if not smoke.startswith("scripts/") else smoke
        if not (repo / rel).exists():
            fail(f"{module['name']}: 缺少 smoke 证据 {rel}")
        if smoke not in acceptance and rel not in acceptance:
            fail(f"{module['name']}: {rel} 未纳入 scripts/standalone_acceptance.sh")

    for rel, token in module.get("tokens", []):
        if not file_has(rel, token):
            fail(f"{module['name']}: {rel} 缺少证据 token {token!r}")

uncovered_manifest = sorted(set(route_paths) - covered_manifest)
if uncovered_manifest:
    fail("manifest 路由未归入任何功能模块: " + ", ".join(uncovered_manifest))

uncovered_runtime = sorted(set(runtime_route_paths) - covered_runtime)
if uncovered_runtime:
    fail("Go 运行时迁移路由未归入任何功能模块: " + ", ".join(uncovered_runtime))

duplicate_names = [name for name in {item["name"] for item in modules} if sum(1 for item in modules if item["name"] == name) > 1]
for name in sorted(duplicate_names):
    fail(f"重复的功能模块名称: {name}")

if failures:
    print("functional module matrix audit failed:", file=sys.stderr)
    for item in failures:
        print(f"- {item}", file=sys.stderr)
    raise SystemExit(1)

print(
    "functional module matrix audit passed: "
    f"modules={len(modules)} "
    f"manifest_unique_paths={len(covered_manifest)}/{len(unique_route_paths)} "
    f"manifest_route_entries={len(routes)} "
    f"runtime_unique_paths={len(covered_runtime)}/{len(unique_runtime_route_paths)} "
    f"runtime_route_entries={len(runtime_route_keys)}"
)
for module in modules:
    manifest_count = sum(
        1
        for path in unique_route_paths
        if any(route_matches(path, prefix) for prefix in module["prefixes"])
    )
    runtime_count = sum(
        1
        for path in unique_runtime_route_paths
        if any(route_matches(path, prefix) for prefix in module["prefixes"])
    )
    smokes = ", ".join(module["smokes"])
    print(f"  {module['name']}: manifest_paths={manifest_count} runtime_paths={runtime_count} smokes={smokes}")
PY
