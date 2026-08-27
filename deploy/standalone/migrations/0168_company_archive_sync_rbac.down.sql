DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
WHERE permission.`code` IN ('dashboard.company_setting.website', 'dashboard.chat.v2_all')
  AND (
    (resource.`http_method` = 'POST' AND resource.`path_pattern` = '/dashboard/company/archive-sync')
    OR (resource.`http_method` = 'GET' AND resource.`path_pattern` = '/dashboard/company/archive-sync-status')
  );
