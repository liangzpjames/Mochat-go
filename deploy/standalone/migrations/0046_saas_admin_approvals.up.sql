CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_approvals` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `request_no` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '审批单号',
  `action_type` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '受保护动作',
  `risk_level` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'high' COMMENT 'high 或 critical',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending' COMMENT 'pending/approved/rejected/canceled/expired/executing/executed',
  `required_permission` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '发起人所需业务权限',
  `target_type` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `target_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `target_name` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `requester_user_id` int(10) unsigned NOT NULL,
  `requester_tenant_id` int(10) unsigned NOT NULL,
  `idempotency_key` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `request_sha256` char(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `request_json` json DEFAULT NULL,
  `reason` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `reviewer_user_id` int(10) unsigned NOT NULL DEFAULT '0',
  `reviewed_at` timestamp NULL DEFAULT NULL,
  `decision_reason` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `execution_user_id` int(10) unsigned NOT NULL DEFAULT '0',
  `execution_started_at` timestamp NULL DEFAULT NULL,
  `effect_applied_at` timestamp NULL DEFAULT NULL COMMENT '业务副作用事务已提交时间',
  `effect_operation_id` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '业务副作用操作日志 ID',
  `executed_at` timestamp NULL DEFAULT NULL,
  `execution_attempts` int(10) unsigned NOT NULL DEFAULT '0',
  `result_json` json DEFAULT NULL,
  `last_error` text COLLATE utf8mb4_unicode_ci,
  `expires_at` timestamp NOT NULL,
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_admin_approval_no` (`request_no`),
  UNIQUE KEY `uni_mochat_go_saas_admin_approval_idempotency` (`requester_user_id`, `idempotency_key`),
  KEY `idx_mochat_go_saas_admin_approval_status_time` (`status`, `created_at`),
  KEY `idx_mochat_go_saas_admin_approval_action_time` (`action_type`, `created_at`),
  KEY `idx_mochat_go_saas_admin_approval_requester_time` (`requester_user_id`, `created_at`),
  KEY `idx_mochat_go_saas_admin_approval_expiry` (`status`, `expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 总后台高风险审批';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_approval_events` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `approval_id` bigint(20) unsigned NOT NULL,
  `event_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `from_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `to_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `actor_user_id` int(10) unsigned NOT NULL DEFAULT '0',
  `actor_tenant_id` int(10) unsigned NOT NULL DEFAULT '0',
  `reason` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `context_json` json DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mochat_go_saas_admin_approval_event_approval` (`approval_id`, `id`),
  KEY `idx_mochat_go_saas_admin_approval_event_actor` (`actor_user_id`, `created_at`),
  KEY `idx_mochat_go_saas_admin_approval_event_type` (`event_type`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 总后台审批事件';

INSERT IGNORE INTO `mochat_go_saas_admin_roles`
  (`code`, `name`, `description`, `status`, `is_system`, `version`, `created_by`, `updated_by`, `created_at`, `updated_at`)
VALUES
  ('platform_approver', '平台审批', '复核并执行总后台高风险操作，不直接发起业务变更', 1, 1, 1, 0, 0, NOW(), NOW());

INSERT IGNORE INTO `mochat_go_saas_admin_role_permissions` (`role_id`, `permission_code`, `created_at`)
SELECT r.id, p.permission_code, NOW()
FROM `mochat_go_saas_admin_roles` r
INNER JOIN (
  SELECT 'platform_operations' AS role_code, 'platform.approvals.read' AS permission_code
  UNION ALL SELECT 'platform_finance', 'platform.approvals.read'
  UNION ALL SELECT 'platform_customer_success', 'platform.approvals.read'
  UNION ALL SELECT 'platform_auditor', 'platform.approvals.read'
  UNION ALL SELECT 'platform_readonly', 'platform.approvals.read'
  UNION ALL SELECT 'platform_approver', 'platform.overview.read'
  UNION ALL SELECT 'platform_approver', 'platform.audit.read'
  UNION ALL SELECT 'platform_approver', 'platform.approvals.read'
  UNION ALL SELECT 'platform_approver', 'platform.approvals.review'
  UNION ALL SELECT 'platform_approver', 'platform.approvals.execute'
) p ON p.role_code = r.code;
