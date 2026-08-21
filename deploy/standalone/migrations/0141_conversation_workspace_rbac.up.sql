INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT p.`id`, 'api', resource_seed.`http_method`, resource_seed.`path_pattern`, resource_seed.`scope_required`, 1, 1
FROM `mochat_go_dashboard_permissions` p
INNER JOIN (
  SELECT 'dashboard.chat.v2_all' AS `permission_code`, 'GET' AS `http_method`, '/dashboard/workMessage/globalOverview' AS `path_pattern`, 0 AS `scope_required`
  UNION ALL SELECT 'dashboard.chat.v2_all', 'PUT', '/dashboard/workMessage/focus', 0
  UNION ALL SELECT 'dashboard.chat.v2_all', 'DELETE', '/dashboard/workMessage/focus', 0
  UNION ALL SELECT 'dashboard.chat.v2_staff', 'GET', '/dashboard/workMessage/staffDirectory', 1
  UNION ALL SELECT 'dashboard.chat.v2_staff', 'GET', '/dashboard/workMessage/staffDetail', 1
  UNION ALL SELECT 'dashboard.chat.v2_staff', 'PUT', '/dashboard/workMessage/focus', 1
  UNION ALL SELECT 'dashboard.chat.v2_staff', 'DELETE', '/dashboard/workMessage/focus', 1
) resource_seed ON resource_seed.`permission_code` = p.`code`
WHERE NOT EXISTS (
  SELECT 1
  FROM `mochat_go_dashboard_permission_resources` existing
  WHERE existing.`permission_id` = p.`id`
    AND existing.`resource_type` = 'api'
    AND existing.`http_method` = resource_seed.`http_method`
    AND existing.`path_pattern` = resource_seed.`path_pattern`
);
