CREATE TABLE IF NOT EXISTS `mochat_go_saas_compliance_policies` (
  `id` bigint(20) unsigned NOT NULL,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active' COMMENT 'active 或 disabled',
  `export_retention_days` int(10) unsigned NOT NULL DEFAULT '30',
  `erasure_grace_days` int(10) unsigned NOT NULL DEFAULT '30',
  `require_recent_export` tinyint(3) unsigned NOT NULL DEFAULT '1',
  `recent_export_max_age_days` int(10) unsigned NOT NULL DEFAULT '7',
  `billing_retention_days` int(10) unsigned NOT NULL DEFAULT '2555',
  `audit_retention_days` int(10) unsigned NOT NULL DEFAULT '365',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `updated_by` int(10) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 租户数据合规策略';

INSERT IGNORE INTO `mochat_go_saas_compliance_policies`
  (`id`, `status`, `export_retention_days`, `erasure_grace_days`, `require_recent_export`,
   `recent_export_max_age_days`, `billing_retention_days`, `audit_retention_days`, `version`, `updated_by`, `created_at`, `updated_at`)
VALUES
  (1, 'active', 30, 30, 1, 7, 2555, 365, 1, 0, NOW(), NOW());

CREATE TABLE IF NOT EXISTS `mochat_go_saas_legal_holds` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `hold_no` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active' COMMENT 'active/released/expired',
  `active_slot` int(10) unsigned DEFAULT NULL COMMENT '活动保留时等于 tenant_id，唯一约束阻止重复活动保留',
  `reason` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL,
  `starts_at` datetime NOT NULL,
  `expires_at` datetime DEFAULT NULL,
  `released_at` datetime DEFAULT NULL,
  `released_by` int(10) unsigned NOT NULL DEFAULT '0',
  `release_reason` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_by` int(10) unsigned NOT NULL DEFAULT '0',
  `updated_by` int(10) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_legal_hold_no` (`hold_no`),
  UNIQUE KEY `uni_mochat_go_saas_legal_hold_active` (`active_slot`),
  KEY `idx_mochat_go_saas_legal_hold_tenant_status` (`tenant_id`, `status`, `id`),
  KEY `idx_mochat_go_saas_legal_hold_expiry` (`status`, `expires_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 租户法律保留';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_data_exports` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `export_no` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL,
  `tenant_name_snapshot` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending' COMMENT 'pending/running/succeeded/failed/expired/deleted',
  `active_slot` int(10) unsigned DEFAULT NULL COMMENT 'pending/running 时等于 tenant_id',
  `artifact_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `artifact_format` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'tar.gz.mgce',
  `encrypted` tinyint(3) unsigned NOT NULL DEFAULT '1',
  `encryption_key_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `sha256` char(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `size_bytes` bigint(20) unsigned NOT NULL DEFAULT '0',
  `manifest_sha256` char(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `inventory_version` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'v1',
  `table_count` int(10) unsigned NOT NULL DEFAULT '0',
  `row_count` bigint(20) unsigned NOT NULL DEFAULT '0',
  `file_count` int(10) unsigned NOT NULL DEFAULT '0',
  `file_size_bytes` bigint(20) unsigned NOT NULL DEFAULT '0',
  `request_reason` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL,
  `requested_by` int(10) unsigned NOT NULL DEFAULT '0',
  `actor_tenant_id` int(10) unsigned NOT NULL DEFAULT '0',
  `started_at` datetime DEFAULT NULL,
  `finished_at` datetime DEFAULT NULL,
  `expires_at` datetime DEFAULT NULL,
  `error_message` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `download_count` int(10) unsigned NOT NULL DEFAULT '0',
  `last_downloaded_at` datetime DEFAULT NULL,
  `last_downloaded_by` int(10) unsigned NOT NULL DEFAULT '0',
  `operation_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_data_export_no` (`export_no`),
  UNIQUE KEY `uni_mochat_go_saas_data_export_active` (`active_slot`),
  KEY `idx_mochat_go_saas_data_export_tenant_status` (`tenant_id`, `status`, `id`),
  KEY `idx_mochat_go_saas_data_export_expiry` (`status`, `expires_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 租户数据导出';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_erasure_requests` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `request_no` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL,
  `tenant_name_snapshot` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending_approval' COMMENT 'pending_approval/approved/waiting/running/succeeded/failed/blocked/canceled',
  `active_slot` int(10) unsigned DEFAULT NULL COMMENT '未终结时等于 tenant_id',
  `reason` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL,
  `confirmation_sha256` char(64) COLLATE utf8mb4_bin NOT NULL,
  `eligible_at` datetime NOT NULL,
  `latest_export_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `approval_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `approved_at` datetime DEFAULT NULL,
  `approved_by` int(10) unsigned NOT NULL DEFAULT '0',
  `inventory_version` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'v1',
  `total_steps` int(10) unsigned NOT NULL DEFAULT '0',
  `completed_steps` int(10) unsigned NOT NULL DEFAULT '0',
  `deleted_rows` bigint(20) unsigned NOT NULL DEFAULT '0',
  `redacted_rows` bigint(20) unsigned NOT NULL DEFAULT '0',
  `verification_sha256` char(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `report_json` json DEFAULT NULL,
  `last_error` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `requested_by` int(10) unsigned NOT NULL DEFAULT '0',
  `actor_tenant_id` int(10) unsigned NOT NULL DEFAULT '0',
  `started_at` datetime DEFAULT NULL,
  `finished_at` datetime DEFAULT NULL,
  `operation_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_erasure_request_no` (`request_no`),
  UNIQUE KEY `uni_mochat_go_saas_erasure_active` (`active_slot`),
  KEY `idx_mochat_go_saas_erasure_tenant_status` (`tenant_id`, `status`, `id`),
  KEY `idx_mochat_go_saas_erasure_due` (`status`, `eligible_at`, `id`),
  KEY `idx_mochat_go_saas_erasure_approval` (`approval_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 租户数据擦除请求';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_erasure_steps` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `request_id` bigint(20) unsigned NOT NULL,
  `step_order` int(10) unsigned NOT NULL,
  `step_key` varchar(128) COLLATE utf8mb4_bin NOT NULL,
  `table_name` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `action` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'delete/redact/verify/tombstone',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending' COMMENT 'pending/running/succeeded/failed',
  `affected_rows` bigint(20) unsigned NOT NULL DEFAULT '0',
  `started_at` datetime DEFAULT NULL,
  `finished_at` datetime DEFAULT NULL,
  `error_message` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_erasure_step` (`request_id`, `step_key`),
  KEY `idx_mochat_go_saas_erasure_step_status` (`request_id`, `status`, `step_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 租户数据擦除断点步骤';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_tenant_tombstones` (
  `tenant_id` int(10) unsigned NOT NULL,
  `anonymous_ref` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `original_name_sha256` char(64) COLLATE utf8mb4_bin NOT NULL,
  `erasure_request_id` bigint(20) unsigned NOT NULL,
  `verification_sha256` char(64) COLLATE utf8mb4_bin NOT NULL,
  `erased_at` datetime NOT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`tenant_id`),
  UNIQUE KEY `uni_mochat_go_saas_tenant_tombstone_ref` (`anonymous_ref`),
  UNIQUE KEY `uni_mochat_go_saas_tenant_tombstone_request` (`erasure_request_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 已擦除租户不可逆墓碑';

INSERT IGNORE INTO `mochat_go_saas_admin_approval_policies`
  (`action_type`, `enabled`, `amount_threshold_cents`, `required_approvals`, `sla_minutes`, `reminder_minutes`,
   `expiry_hours`, `version`, `updated_by`, `created_at`, `updated_at`)
VALUES
  ('tenant.data.erase', 1, 0, 2, 1440, 240, 72, 1, 0, NOW(), NOW());

INSERT IGNORE INTO `mochat_go_saas_admin_role_permissions` (`role_id`, `permission_code`, `created_at`)
SELECT r.id, p.permission_code, NOW()
FROM `mochat_go_saas_admin_roles` r
INNER JOIN (
  SELECT 'platform_operations' AS role_code, 'platform.compliance.read' AS permission_code
  UNION ALL SELECT 'platform_operations', 'platform.compliance.manage'
  UNION ALL SELECT 'platform_approver', 'platform.compliance.read'
  UNION ALL SELECT 'platform_auditor', 'platform.compliance.read'
  UNION ALL SELECT 'platform_readonly', 'platform.compliance.read'
) p ON p.role_code = r.code;
