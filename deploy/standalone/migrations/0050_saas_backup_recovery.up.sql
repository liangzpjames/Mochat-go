CREATE TABLE IF NOT EXISTS `mochat_go_saas_backup_policies` (
  `id` bigint(20) unsigned NOT NULL,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active' COMMENT 'active 或 disabled',
  `interval_minutes` int(10) unsigned NOT NULL DEFAULT '1440',
  `retention_days` int(10) unsigned NOT NULL DEFAULT '30',
  `min_successful_backups` int(10) unsigned NOT NULL DEFAULT '7',
  `max_backup_age_minutes` int(10) unsigned NOT NULL DEFAULT '1800',
  `restore_drill_interval_days` int(10) unsigned NOT NULL DEFAULT '30',
  `require_encryption` tinyint(3) unsigned NOT NULL DEFAULT '1',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `updated_by` int(10) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 备份与恢复策略';

INSERT IGNORE INTO `mochat_go_saas_backup_policies`
  (`id`, `status`, `interval_minutes`, `retention_days`, `min_successful_backups`, `max_backup_age_minutes`,
   `restore_drill_interval_days`, `require_encryption`, `version`, `updated_by`, `created_at`, `updated_at`)
VALUES
  (1, 'active', 1440, 30, 7, 1800, 30, 1, 1, 0, NOW(), NOW());

CREATE TABLE IF NOT EXISTS `mochat_go_saas_backup_runs` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `backup_no` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `trigger_type` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'manual' COMMENT 'manual/cron/maintenance',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'running' COMMENT 'running/succeeded/failed/deleted',
  `active_slot` tinyint(3) unsigned DEFAULT NULL COMMENT '运行中固定为 1，唯一约束阻止并发备份',
  `artifact_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `artifact_format` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'sql.gz.mgbk',
  `encrypted` tinyint(3) unsigned NOT NULL DEFAULT '1',
  `encryption_key_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `sha256` char(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `size_bytes` bigint(20) unsigned NOT NULL DEFAULT '0',
  `database_name` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `migration_version` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `migration_count` int(10) unsigned NOT NULL DEFAULT '0',
  `table_count` int(10) unsigned NOT NULL DEFAULT '0',
  `verification_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending' COMMENT 'pending/passed/failed',
  `verified_at` datetime DEFAULT NULL,
  `verification_error` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `error_message` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `started_at` datetime NOT NULL,
  `finished_at` datetime DEFAULT NULL,
  `created_by` int(10) unsigned NOT NULL DEFAULT '0',
  `actor_tenant_id` int(10) unsigned NOT NULL DEFAULT '0',
  `operation_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_backup_no` (`backup_no`),
  UNIQUE KEY `uni_mochat_go_saas_backup_active` (`active_slot`),
  KEY `idx_mochat_go_saas_backup_status_time` (`status`, `finished_at`, `id`),
  KEY `idx_mochat_go_saas_backup_verify` (`verification_status`, `verified_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 数据库备份运行';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_restore_drills` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `drill_no` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `backup_run_id` bigint(20) unsigned NOT NULL,
  `trigger_type` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'manual' COMMENT 'manual/maintenance',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'running' COMMENT 'running/succeeded/failed',
  `active_slot` tinyint(3) unsigned DEFAULT NULL COMMENT '运行中固定为 1，唯一约束阻止并发恢复',
  `target_fingerprint` char(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '目标地址与库名的不可逆摘要',
  `target_database` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `expected_migration_version` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `actual_migration_version` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `expected_migration_count` int(10) unsigned NOT NULL DEFAULT '0',
  `actual_migration_count` int(10) unsigned NOT NULL DEFAULT '0',
  `expected_table_count` int(10) unsigned NOT NULL DEFAULT '0',
  `actual_table_count` int(10) unsigned NOT NULL DEFAULT '0',
  `checks_json` json DEFAULT NULL,
  `error_message` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `started_at` datetime NOT NULL,
  `finished_at` datetime DEFAULT NULL,
  `duration_ms` bigint(20) unsigned NOT NULL DEFAULT '0',
  `created_by` int(10) unsigned NOT NULL DEFAULT '0',
  `actor_tenant_id` int(10) unsigned NOT NULL DEFAULT '0',
  `operation_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_restore_drill_no` (`drill_no`),
  UNIQUE KEY `uni_mochat_go_saas_restore_drill_active` (`active_slot`),
  KEY `idx_mochat_go_saas_restore_status_time` (`status`, `finished_at`, `id`),
  KEY `idx_mochat_go_saas_restore_backup` (`backup_run_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 隔离恢复演练';

INSERT IGNORE INTO `mochat_go_saas_admin_role_permissions` (`role_id`, `permission_code`, `created_at`)
SELECT r.id, p.permission_code, NOW()
FROM `mochat_go_saas_admin_roles` r
INNER JOIN (
  SELECT 'platform_operations' AS role_code, 'platform.backups.read' AS permission_code
  UNION ALL SELECT 'platform_operations', 'platform.backups.manage'
  UNION ALL SELECT 'platform_auditor', 'platform.backups.read'
  UNION ALL SELECT 'platform_readonly', 'platform.backups.read'
) p ON p.role_code = r.code;
