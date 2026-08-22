DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
WHERE permission.`code` IN ('dashboard.acquisition.v2_channel_code', 'dashboard.acquisition.group_code')
  AND NOT EXISTS (
    SELECT 1
    FROM `mochat_go_schema_migrations` legacy
    WHERE legacy.`version` = '0150_live_code_workspace'
  )
  AND resource.`path_pattern` IN (
    '/dashboard/channelCode/index',
    '/dashboard/channelCode/store',
    '/dashboard/channelCode/update',
    '/dashboard/channelCode/workspaceStatistics',
    '/dashboard/channelCode/workspaceStatisticsIndex',
    '/dashboard/channelCode/export',
    '/dashboard/channelCode/batchInvalidate'
  );

-- A development build briefly shipped the same schema under the colliding
-- 0150_live_code_workspace version. When that ledger fact exists, 0153 only
-- reconciles ownership and its rollback must not remove the legacy schema.
SET @mochat_live_code_legacy = (
  SELECT COUNT(*)
  FROM `mochat_go_schema_migrations`
  WHERE `version` = '0150_live_code_workspace'
);

SET @mochat_live_code_down_sql = IF(
  @mochat_live_code_legacy = 0,
  'DROP TABLE IF EXISTS `mc_live_code_event`',
  'SELECT 1'
);
PREPARE mochat_live_code_down FROM @mochat_live_code_down_sql;
EXECUTE mochat_live_code_down;
DEALLOCATE PREPARE mochat_live_code_down;

SET @mochat_live_code_down_sql = IF(
  @mochat_live_code_legacy = 0,
  'DROP TABLE IF EXISTS `mc_group_code_group`',
  'SELECT 1'
);
PREPARE mochat_live_code_down FROM @mochat_live_code_down_sql;
EXECUTE mochat_live_code_down;
DEALLOCATE PREPARE mochat_live_code_down;

SET @mochat_live_code_down_sql = IF(
  @mochat_live_code_legacy = 0,
  'ALTER TABLE `mc_room_infinite` DROP KEY `idx_mc_room_infinite_group`, DROP COLUMN `data_source`, DROP COLUMN `lifecycle_state`, DROP COLUMN `code_type`, DROP COLUMN `group_id`',
  'SELECT 1'
);
PREPARE mochat_live_code_down FROM @mochat_live_code_down_sql;
EXECUTE mochat_live_code_down;
DEALLOCATE PREPARE mochat_live_code_down;

SET @mochat_live_code_down_sql = IF(
  @mochat_live_code_legacy = 0,
  'ALTER TABLE `mc_work_room_auto_pull` DROP KEY `idx_mc_work_room_auto_pull_group`, DROP COLUMN `data_source`, DROP COLUMN `lifecycle_state`, DROP COLUMN `group_id`',
  'SELECT 1'
);
PREPARE mochat_live_code_down FROM @mochat_live_code_down_sql;
EXECUTE mochat_live_code_down;
DEALLOCATE PREPARE mochat_live_code_down;

SET @mochat_live_code_down_sql = IF(
  @mochat_live_code_legacy = 0,
  'ALTER TABLE `mc_channel_code` DROP COLUMN `data_source`, DROP COLUMN `provider_error`, DROP COLUMN `provider_state`, DROP COLUMN `lifecycle_state`, DROP COLUMN `valid_until`, DROP COLUMN `valid_from`, DROP COLUMN `validity_kind`',
  'SELECT 1'
);
PREPARE mochat_live_code_down FROM @mochat_live_code_down_sql;
EXECUTE mochat_live_code_down;
DEALLOCATE PREPARE mochat_live_code_down;
