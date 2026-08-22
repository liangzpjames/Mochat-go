DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
WHERE permission.`code` IN ('dashboard.acquisition.v2_channel_code', 'dashboard.acquisition.group_code')
  AND resource.`path_pattern` IN (
    '/dashboard/channelCode/index',
    '/dashboard/channelCode/store',
    '/dashboard/channelCode/update',
    '/dashboard/channelCode/workspaceStatistics',
    '/dashboard/channelCode/workspaceStatisticsIndex',
    '/dashboard/channelCode/export',
    '/dashboard/channelCode/batchInvalidate'
  );

DROP TABLE IF EXISTS `mc_live_code_event`;
DROP TABLE IF EXISTS `mc_group_code_group`;

ALTER TABLE `mc_room_infinite`
  DROP KEY `idx_mc_room_infinite_group`,
  DROP COLUMN `data_source`,
  DROP COLUMN `lifecycle_state`,
  DROP COLUMN `code_type`,
  DROP COLUMN `group_id`;

ALTER TABLE `mc_work_room_auto_pull`
  DROP KEY `idx_mc_work_room_auto_pull_group`,
  DROP COLUMN `data_source`,
  DROP COLUMN `lifecycle_state`,
  DROP COLUMN `group_id`;

ALTER TABLE `mc_channel_code`
  DROP COLUMN `data_source`,
  DROP COLUMN `provider_error`,
  DROP COLUMN `provider_state`,
  DROP COLUMN `lifecycle_state`,
  DROP COLUMN `valid_until`,
  DROP COLUMN `valid_from`,
  DROP COLUMN `validity_kind`;
