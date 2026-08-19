DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
WHERE permission.`code` = 'dashboard.chat.v2_customer'
  AND resource.`resource_type` = 'api'
  AND ((resource.`http_method` = 'GET' AND resource.`path_pattern` IN (
    '/dashboard/workMessage/customerDirectory',
    '/dashboard/workMessage/customerConversations',
    '/dashboard/workMessage/customerDetail'
  ))
    OR (resource.`http_method` IN ('PUT', 'DELETE') AND resource.`path_pattern` = '/dashboard/workMessage/focus'));
