-- Live code workspace fields and verifiable event ledger.
ALTER TABLE `mc_channel_code`
  ADD COLUMN `validity_kind` varchar(16) NOT NULL DEFAULT 'permanent',
  ADD COLUMN `valid_from` datetime NULL,
  ADD COLUMN `valid_until` datetime NULL,
  ADD COLUMN `lifecycle_state` varchar(16) NOT NULL DEFAULT 'active',
  ADD COLUMN `provider_state` varchar(16) NOT NULL DEFAULT 'synced',
  ADD COLUMN `provider_error` varchar(512) NOT NULL DEFAULT '',
  ADD COLUMN `data_source` varchar(16) NOT NULL DEFAULT 'business';

ALTER TABLE `mc_work_room_auto_pull`
  ADD COLUMN `group_id` int unsigned NOT NULL DEFAULT '0',
  ADD COLUMN `lifecycle_state` varchar(16) NOT NULL DEFAULT 'active',
  ADD COLUMN `data_source` varchar(16) NOT NULL DEFAULT 'business',
  ADD KEY `idx_mc_work_room_auto_pull_group` (`corp_id`, `group_id`, `deleted_at`);

ALTER TABLE `mc_room_infinite`
  ADD COLUMN `group_id` bigint unsigned NOT NULL DEFAULT '0',
  ADD COLUMN `code_type` varchar(24) NOT NULL DEFAULT 'uploadedGroup',
  ADD COLUMN `lifecycle_state` varchar(16) NOT NULL DEFAULT 'active',
  ADD COLUMN `data_source` varchar(16) NOT NULL DEFAULT 'business',
  ADD KEY `idx_mc_room_infinite_group` (`corp_id`, `group_id`, `deleted_at`);

UPDATE `mc_channel_code`
SET `data_source` = 'simulation'
WHERE `qrcode_url` LIKE 'https://example.invalid/%' OR `name` LIKE 'sim-%';

UPDATE `mc_work_room_auto_pull`
SET `data_source` = 'simulation'
WHERE `qrcode_url` LIKE 'https://example.invalid/%' OR `qrcode_name` LIKE 'sim-%';

UPDATE `mc_room_infinite`
SET `data_source` = 'simulation'
WHERE `name` LIKE 'sim-%';

CREATE TABLE `mc_group_code_group` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int unsigned NOT NULL,
  `name` varchar(30) NOT NULL,
  `active_name` varchar(30) GENERATED ALWAYS AS (CASE WHEN `deleted_at` IS NULL THEN `name` ELSE NULL END) STORED,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_group_code_group_corp_name` (`corp_id`, `active_name`),
  KEY `idx_group_code_group_corp_deleted` (`corp_id`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `mc_live_code_event` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int unsigned NOT NULL,
  `mode` varchar(20) NOT NULL,
  `code_id` bigint unsigned NOT NULL,
  `contact_id` bigint unsigned NULL,
  `room_id` bigint unsigned NULL,
  `event_type` varchar(24) NOT NULL,
  `source_event_id` varchar(128) NOT NULL,
  `occurred_at` datetime NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_live_code_event_source` (`corp_id`, `mode`, `source_event_id`),
  KEY `idx_live_code_event_daily` (`corp_id`, `mode`, `code_id`, `event_type`, `occurred_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT p.`id`, 'api', resource_seed.`http_method`, resource_seed.`path_pattern`, resource_seed.`scope_required`, 1, 1
FROM `mochat_go_dashboard_permissions` p
INNER JOIN (
  SELECT 'dashboard.acquisition.v2_channel_code' AS `permission_code`, 'GET' AS `http_method`, '/dashboard/channelCode/index' AS `path_pattern`, 1 AS `scope_required`
  UNION ALL SELECT 'dashboard.acquisition.v2_channel_code', 'POST', '/dashboard/channelCode/store', 1
  UNION ALL SELECT 'dashboard.acquisition.v2_channel_code', 'PUT', '/dashboard/channelCode/update', 1
  UNION ALL SELECT 'dashboard.acquisition.v2_channel_code', 'GET', '/dashboard/channelCode/workspaceStatistics', 1
  UNION ALL SELECT 'dashboard.acquisition.v2_channel_code', 'GET', '/dashboard/channelCode/workspaceStatisticsIndex', 1
  UNION ALL SELECT 'dashboard.acquisition.v2_channel_code', 'GET', '/dashboard/channelCode/export', 1
  UNION ALL SELECT 'dashboard.acquisition.v2_channel_code', 'POST', '/dashboard/channelCode/batchInvalidate', 1
  UNION ALL SELECT 'dashboard.acquisition.group_code', 'GET', '/dashboard/groupCode/index', 1
  UNION ALL SELECT 'dashboard.acquisition.group_code', 'GET', '/dashboard/groupCode/show', 1
  UNION ALL SELECT 'dashboard.acquisition.group_code', 'POST', '/dashboard/groupCode/store', 1
  UNION ALL SELECT 'dashboard.acquisition.group_code', 'PUT', '/dashboard/groupCode/update', 1
  UNION ALL SELECT 'dashboard.acquisition.group_code', 'GET', '/dashboard/groupCode/export', 1
  UNION ALL SELECT 'dashboard.acquisition.group_code', 'GET', '/dashboard/groupCode/download', 1
  UNION ALL SELECT 'dashboard.acquisition.group_code', 'POST', '/dashboard/groupCode/batchInvalidate', 1
  UNION ALL SELECT 'dashboard.acquisition.group_code', 'GET', '/dashboard/groupCodeGroup/index', 1
  UNION ALL SELECT 'dashboard.acquisition.group_code', 'POST', '/dashboard/groupCodeGroup/store', 1
  UNION ALL SELECT 'dashboard.acquisition.group_code', 'PUT', '/dashboard/groupCodeGroup/update', 1
  UNION ALL SELECT 'dashboard.acquisition.group_code', 'POST', '/dashboard/groupCodeGroup/move', 1
  UNION ALL SELECT 'dashboard.acquisition.group_code', 'DELETE', '/dashboard/groupCodeGroup/destroy', 1
) resource_seed ON resource_seed.`permission_code` = p.`code`
WHERE NOT EXISTS (
  SELECT 1
  FROM `mochat_go_dashboard_permission_resources` existing
  WHERE existing.`permission_id` = p.`id`
    AND existing.`resource_type` = 'api'
    AND existing.`http_method` = resource_seed.`http_method`
    AND existing.`path_pattern` = resource_seed.`path_pattern`
);
