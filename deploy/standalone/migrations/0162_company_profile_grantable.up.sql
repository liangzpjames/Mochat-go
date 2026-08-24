-- Make the single-company profile page grantable while keeping every API
-- authorization attached to its existing page permission.
UPDATE `mochat_go_dashboard_permissions`
SET `restriction` = 'grantable', `superadmin_only` = 0, `updated_at` = CURRENT_TIMESTAMP
WHERE `code` = 'dashboard.company_setting.website';

INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT permission.`id`, 'api', seed.`http_method`, seed.`path_pattern`, seed.`scope_required`, 1, 1
FROM `mochat_go_dashboard_permissions` permission
INNER JOIN (
  SELECT 'dashboard.company_setting.website' AS `permission_code`, 'GET' AS `http_method`,
    '/dashboard/providers/status' AS `path_pattern`, 0 AS `scope_required`
) seed ON seed.`permission_code` = permission.`code`
WHERE NOT EXISTS (
  SELECT 1
  FROM `mochat_go_dashboard_permission_resources` existing
  WHERE existing.`permission_id` = permission.`id`
    AND existing.`resource_type` = 'api'
    AND existing.`http_method` = seed.`http_method`
    AND existing.`path_pattern` = seed.`path_pattern`
);
