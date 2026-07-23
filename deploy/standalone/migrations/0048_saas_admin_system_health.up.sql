CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_health_scans` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `scan_no` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '健康扫描编号',
  `trigger_type` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'manual' COMMENT 'manual 或 cron',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'completed' COMMENT 'completed 或 failed',
  `health_state` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'healthy' COMMENT 'healthy/warning/critical',
  `check_count` int(10) unsigned NOT NULL DEFAULT '0',
  `issue_count` int(10) unsigned NOT NULL DEFAULT '0',
  `critical_count` int(10) unsigned NOT NULL DEFAULT '0',
  `warning_count` int(10) unsigned NOT NULL DEFAULT '0',
  `opened_count` int(10) unsigned NOT NULL DEFAULT '0',
  `reopened_count` int(10) unsigned NOT NULL DEFAULT '0',
  `recovered_count` int(10) unsigned NOT NULL DEFAULT '0',
  `notification_count` int(10) unsigned NOT NULL DEFAULT '0',
  `failure_window_hours` smallint(5) unsigned NOT NULL DEFAULT '24',
  `notification_stale_minutes` int(10) unsigned NOT NULL DEFAULT '15',
  `actor_user_id` int(10) unsigned NOT NULL DEFAULT '0',
  `actor_tenant_id` int(10) unsigned NOT NULL DEFAULT '0',
  `started_at` datetime NOT NULL,
  `finished_at` datetime NOT NULL,
  `snapshot_json` json DEFAULT NULL,
  `error_message` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `operation_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_admin_health_scan_no` (`scan_no`),
  KEY `idx_mochat_go_saas_admin_health_scan_state` (`health_state`, `finished_at`),
  KEY `idx_mochat_go_saas_admin_health_scan_trigger` (`trigger_type`, `finished_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 总后台健康扫描';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_system_incidents` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `incident_key` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '稳定事故聚合键',
  `source` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '健康检查来源',
  `category` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '事故分类',
  `severity` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'warning 或 critical',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'open' COMMENT 'open/acknowledged/resolved',
  `title` varchar(160) COLLATE utf8mb4_unicode_ci NOT NULL,
  `detail` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `current_value` bigint(20) NOT NULL DEFAULT '0',
  `threshold_value` bigint(20) NOT NULL DEFAULT '0',
  `occurrence_count` bigint(20) unsigned NOT NULL DEFAULT '1',
  `first_detected_at` datetime NOT NULL,
  `last_detected_at` datetime NOT NULL,
  `last_scan_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `acknowledged_at` datetime DEFAULT NULL,
  `acknowledged_by` int(10) unsigned NOT NULL DEFAULT '0',
  `resolved_at` datetime DEFAULT NULL,
  `resolved_by` int(10) unsigned NOT NULL DEFAULT '0',
  `owner` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `resolution_note` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `metadata_json` json DEFAULT NULL,
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_admin_system_incident_key` (`incident_key`),
  KEY `idx_mochat_go_saas_admin_system_incident_status` (`status`, `severity`, `last_detected_at`),
  KEY `idx_mochat_go_saas_admin_system_incident_owner` (`owner`, `status`, `last_detected_at`),
  KEY `idx_mochat_go_saas_admin_system_incident_scan` (`last_scan_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 总后台系统事故';

INSERT IGNORE INTO `mochat_go_saas_admin_role_permissions` (`role_id`, `permission_code`, `created_at`)
SELECT r.id, p.permission_code, NOW()
FROM `mochat_go_saas_admin_roles` r
INNER JOIN (
  SELECT 'platform_operations' AS role_code, 'platform.system.read' AS permission_code
  UNION ALL SELECT 'platform_operations', 'platform.system.manage'
  UNION ALL SELECT 'platform_auditor', 'platform.system.read'
  UNION ALL SELECT 'platform_readonly', 'platform.system.read'
) p ON p.role_code = r.code;
