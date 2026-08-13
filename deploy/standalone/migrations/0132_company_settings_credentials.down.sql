DELETE resource
FROM mochat_go_dashboard_permission_resources resource
INNER JOIN mochat_go_dashboard_permissions permission ON permission.id = resource.permission_id
WHERE permission.code = 'dashboard.company_setting.website'
  AND (
    (resource.http_method = 'PUT' AND resource.path_pattern = '/dashboard/company/application-credentials')
    OR (resource.http_method = 'GET' AND resource.path_pattern = '/dashboard/company/callback-configuration')
    OR (resource.http_method = 'POST' AND resource.path_pattern = '/dashboard/company/callback-configuration/regenerate')
  );
