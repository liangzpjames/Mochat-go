DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
WHERE permission.`code` IN (
  'dashboard.chat.v2_all',
  'dashboard.chat.v2_staff',
  'dashboard.chat.v2_customer',
  'dashboard.chat.v2_group'
)
  AND resource.`resource_type` = 'api'
  AND (
    (resource.`http_method` = 'POST' AND resource.`path_pattern` = '/dashboard/archive/components/{id}/session')
    OR (resource.`http_method` = 'GET' AND resource.`path_pattern` = '/dashboard/archive/components/session/{token}')
  );

DROP TABLE IF EXISTS `mochat_go_archive_component_locators`;
