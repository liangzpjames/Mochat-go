-- Register the employee-name filter endpoints with the same read permissions
-- and employee data scope as their owning AI insight pages.

INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT permission.`id`, 'api', 'GET', seed.`path_pattern`, 1, 1, 1
FROM `mochat_go_dashboard_permissions` permission
INNER JOIN (
  SELECT 'dashboard.ai_insight.session_analysis' AS `permission_code`,
    '/dashboard/ai-insight/session-analysis/filter-options' AS `path_pattern`
  UNION ALL
  SELECT 'dashboard.ai_insight.smart_analysis',
    '/dashboard/ai-insight/smart-analysis/filter-options'
) seed ON seed.`permission_code` = permission.`code`
WHERE NOT EXISTS (
  SELECT 1
  FROM `mochat_go_dashboard_permission_resources` existing
  WHERE existing.`permission_id` = permission.`id`
    AND existing.`resource_type` = 'api'
    AND existing.`http_method` = 'GET'
    AND existing.`path_pattern` = seed.`path_pattern`
);
