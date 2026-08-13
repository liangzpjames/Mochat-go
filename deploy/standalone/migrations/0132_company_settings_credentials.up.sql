-- Existing installations already applied 0127 and 0131. Register the new
-- single-company credential endpoints without changing any permission grants.
INSERT INTO mochat_go_dashboard_permission_resources
  (permission_id, resource_type, http_method, path_pattern, scope_required, status, version)
SELECT permission.id, 'api', resource_seed.http_method, resource_seed.path_pattern, 0, 1, 1
FROM mochat_go_dashboard_permissions permission
INNER JOIN (
  SELECT 'PUT' AS http_method, '/dashboard/company/application-credentials' AS path_pattern
  UNION ALL SELECT 'GET', '/dashboard/company/callback-configuration'
  UNION ALL SELECT 'POST', '/dashboard/company/callback-configuration/regenerate'
) resource_seed
WHERE permission.code = 'dashboard.company_setting.website'
  AND permission.superadmin_only = 1
  AND NOT EXISTS (
    SELECT 1
    FROM mochat_go_dashboard_permission_resources existing
    WHERE existing.permission_id = permission.id
      AND existing.http_method = resource_seed.http_method
      AND existing.path_pattern = resource_seed.path_pattern
  );
