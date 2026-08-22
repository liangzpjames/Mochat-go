-- Remove only mappings first introduced by 0154. Other additions are owned by
-- their original 0141-0151 migrations and must survive a rollback of 0154.
DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
INNER JOIN (
  SELECT 'dashboard.customer.inheritance' AS `permission_code`, 'GET' AS `http_method`, '/dashboard/contactTransfer/info' AS `path_pattern`
  UNION ALL SELECT 'dashboard.customer.inheritance', 'GET', '/dashboard/contactTransfer/room'
  UNION ALL SELECT 'dashboard.customer.inheritance', 'GET', '/dashboard/contactTransfer/log'
  UNION ALL SELECT 'dashboard.customer.inheritance', 'GET', '/dashboard/workEmployee/index'
  UNION ALL SELECT 'dashboard.customer.inheritance', 'POST', '/dashboard/contactTransfer/sync'
  UNION ALL SELECT 'dashboard.customer.inheritance', 'POST', '/dashboard/contactTransfer/index'
  UNION ALL SELECT 'dashboard.customer.inheritance', 'POST', '/dashboard/contactTransfer/room'
) resource_seed ON resource_seed.`permission_code` = permission.`code`
  AND resource_seed.`http_method` = resource.`http_method`
  AND resource_seed.`path_pattern` = resource.`path_pattern`
WHERE resource.`resource_type` = 'api';

-- Restore the two legacy export reads for a functional rollback. Media writes
-- and channel-code update stay disabled because the route policy classifies
-- them as deny-only contracts rather than ordinary page grants.
UPDATE `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
INNER JOIN (
  SELECT 'dashboard.chat.export' AS `permission_code`, 'GET' AS `http_method`, '/dashboard/workMessage/toUsers' AS `path_pattern`
  UNION ALL SELECT 'dashboard.chat.export', 'GET', '/dashboard/workMessage/detail'
) restoration_seed ON restoration_seed.`permission_code` = permission.`code`
  AND restoration_seed.`http_method` = resource.`http_method`
  AND restoration_seed.`path_pattern` = resource.`path_pattern`
SET resource.`status` = 1,
    resource.`deleted_at` = NULL,
    resource.`version` = resource.`version` + 1
WHERE resource.`resource_type` = 'api';
