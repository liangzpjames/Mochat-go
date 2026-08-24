-- Remove only the resource key introduced by 0162, then restore the 0131
-- superadmin-only page restriction.
DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
WHERE permission.`code` = 'dashboard.company_setting.website'
  AND resource.`resource_type` = 'api'
  AND resource.`http_method` = 'GET'
  AND resource.`path_pattern` = '/dashboard/providers/status';

UPDATE `mochat_go_dashboard_permissions`
SET `restriction` = 'superadmin_only', `superadmin_only` = 1, `updated_at` = CURRENT_TIMESTAMP
WHERE `code` = 'dashboard.company_setting.website';
