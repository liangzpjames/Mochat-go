-- Conversation archive simulation registry. These tables never participate in
-- the real WeCom archive cursor and only identify explicitly simulated rows.
CREATE TABLE IF NOT EXISTS `mochat_go_archive_simulation_batches` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(10) unsigned NOT NULL,
  `batch_key` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'applying',
  `message_count` int(10) unsigned NOT NULL DEFAULT 0,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_archive_simulation_corp_batch` (`corp_id`, `batch_key`),
  KEY `idx_archive_simulation_batch_status` (`corp_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Explicitly isolated conversation archive simulation batches';

CREATE TABLE IF NOT EXISTS `mochat_go_archive_simulation_messages` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `batch_id` bigint(20) unsigned NOT NULL,
  `corp_id` int(10) unsigned NOT NULL,
  `msgid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `table_index` tinyint(3) unsigned NOT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_archive_simulation_message` (`corp_id`, `msgid`),
  KEY `idx_archive_simulation_message_batch` (`batch_id`, `table_index`),
  CONSTRAINT `fk_archive_simulation_message_batch` FOREIGN KEY (`batch_id`) REFERENCES `mochat_go_archive_simulation_batches` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Exact simulated archive message deletion registry';

CREATE TABLE IF NOT EXISTS `mochat_go_archive_simulation_entities` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `batch_id` bigint(20) unsigned NOT NULL,
  `corp_id` int(10) unsigned NOT NULL,
  `entity_type` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL,
  `entity_id` int(10) unsigned NOT NULL,
  `external_key` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_archive_simulation_entity` (`batch_id`, `entity_type`, `entity_id`),
  KEY `idx_archive_simulation_entity_batch` (`batch_id`, `entity_type`),
  CONSTRAINT `fk_archive_simulation_entity_batch` FOREIGN KEY (`batch_id`) REFERENCES `mochat_go_archive_simulation_batches` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Synthetic participant and room cleanup registry';

INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`,`resource_type`,`http_method`,`path_pattern`,`scope_required`,`status`,`version`,`created_at`,`updated_at`)
SELECT permission.`id`,'api',resource.`http_method`,resource.`path_pattern`,0,1,1,NOW(),NOW()
FROM `mochat_go_dashboard_permissions` permission
INNER JOIN (
  SELECT 'GET' AS `http_method`, '/dashboard/access/employees' AS `path_pattern`
  UNION ALL SELECT 'POST', '/dashboard/access/employees/{id}/account'
  UNION ALL SELECT 'PUT', '/dashboard/access/employees/{id}/account/status'
  UNION ALL SELECT 'POST', '/dashboard/access/employees/{id}/account/reset-password'
) resource
WHERE permission.`code`='dashboard.company_setting.staff'
ON DUPLICATE KEY UPDATE `scope_required`=VALUES(`scope_required`),`status`=1,`deleted_at`=NULL,`updated_at`=NOW();
