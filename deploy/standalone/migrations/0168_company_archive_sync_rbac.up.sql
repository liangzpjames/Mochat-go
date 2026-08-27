-- Register operator-triggered conversation archive synchronization on the
-- existing unique-company permission without changing any grants.
INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT permission.`id`, 'api', seed.`http_method`, seed.`path_pattern`, 0, 1, 1
FROM `mochat_go_dashboard_permissions` permission
INNER JOIN (
  SELECT 'dashboard.company_setting.website' AS `permission_code`, 'POST' AS `http_method`, '/dashboard/company/archive-sync' AS `path_pattern`, 0 AS `scope_required`
  UNION ALL SELECT 'dashboard.company_setting.website', 'GET', '/dashboard/company/archive-sync-status', 0
  UNION ALL SELECT 'dashboard.chat.v2_all', 'POST', '/dashboard/company/archive-sync', 0
  UNION ALL SELECT 'dashboard.chat.v2_all', 'GET', '/dashboard/company/archive-sync-status', 0
) seed ON seed.`permission_code` = permission.`code`
WHERE permission.`status` = 1
  AND NOT EXISTS (
    SELECT 1
    FROM `mochat_go_dashboard_permission_resources` existing
    WHERE existing.`permission_id` = permission.`id`
      AND existing.`http_method` = seed.`http_method`
      AND existing.`path_pattern` = seed.`path_pattern`
  );
