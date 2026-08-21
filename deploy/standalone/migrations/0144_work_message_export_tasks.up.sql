CREATE TABLE IF NOT EXISTS `mochat_go_work_message_export_tasks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int unsigned NOT NULL,
  `corp_id` int unsigned NOT NULL,
  `user_id` int unsigned NOT NULL,
  `idempotency_key` varchar(96) NOT NULL,
  `export_type` varchar(16) NOT NULL,
  `selected_objects_json` json NOT NULL,
  `conversation_scopes_json` json NOT NULL,
  `employee_scope_json` json NOT NULL,
  `start_at` datetime NOT NULL,
  `end_at` datetime NOT NULL,
  `file_mode` varchar(16) NOT NULL,
  `format` varchar(16) NOT NULL,
  `status` varchar(16) NOT NULL DEFAULT 'pending',
  `estimated_message_count` int unsigned NOT NULL DEFAULT 0,
  `message_count` int unsigned NOT NULL DEFAULT 0,
  `file_count` int unsigned NOT NULL DEFAULT 0,
  `artifact_name` varchar(255) NOT NULL DEFAULT '',
  `artifact_path` varchar(512) NOT NULL DEFAULT '',
  `artifact_size` bigint unsigned NOT NULL DEFAULT 0,
  `artifact_sha256` char(64) NOT NULL DEFAULT '',
  `download_count` int unsigned NOT NULL DEFAULT 0,
  `last_downloaded_at` datetime NULL,
  `lease_owner` varchar(96) NOT NULL DEFAULT '',
  `lease_expires_at` datetime NULL,
  `error_code` varchar(64) NOT NULL DEFAULT '',
  `error_message` varchar(500) NOT NULL DEFAULT '',
  `expires_at` datetime NULL,
  `started_at` datetime NULL,
  `finished_at` datetime NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_mg_wmet_idempotency` (`tenant_id`,`corp_id`,`user_id`,`idempotency_key`),
  KEY `idx_mg_wmet_claim` (`status`,`lease_expires_at`,`id`),
  KEY `idx_mg_wmet_owner` (`tenant_id`,`corp_id`,`user_id`,`created_at`,`id`),
  KEY `idx_mg_wmet_expiry` (`status`,`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT p.`id`, 'api', resource_seed.`http_method`, resource_seed.`path_pattern`, 1, 1, 1
FROM `mochat_go_dashboard_permissions` p
INNER JOIN (
  SELECT 'GET' AS `http_method`, '/dashboard/workMessage/exportCandidates' AS `path_pattern`
  UNION ALL SELECT 'GET', '/dashboard/workMessage/exportTasks'
  UNION ALL SELECT 'POST', '/dashboard/workMessage/exportTasks'
  UNION ALL SELECT 'GET', '/dashboard/workMessage/exportDownload'
) resource_seed ON 1 = 1
WHERE p.`code` = 'dashboard.chat.export'
  AND NOT EXISTS (
    SELECT 1
    FROM `mochat_go_dashboard_permission_resources` existing
    WHERE existing.`permission_id` = p.`id`
      AND existing.`resource_type` = 'api'
      AND existing.`http_method` = resource_seed.`http_method`
      AND existing.`path_pattern` = resource_seed.`path_pattern`
  );
