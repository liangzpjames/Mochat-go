-- DELETE resources introduced by the conversation workspace without touching page grants.
DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
WHERE (permission.`code` = 'dashboard.chat.v2_all'
    AND ((resource.`http_method` = 'GET' AND resource.`path_pattern` = '/dashboard/workMessage/globalOverview')
      OR (resource.`http_method` IN ('PUT', 'DELETE') AND resource.`path_pattern` = '/dashboard/workMessage/focus')))
   OR (permission.`code` = 'dashboard.chat.v2_staff'
    AND ((resource.`http_method` = 'GET' AND resource.`path_pattern` IN ('/dashboard/workMessage/staffDirectory', '/dashboard/workMessage/staffDetail'))
      OR (resource.`http_method` IN ('PUT', 'DELETE') AND resource.`path_pattern` = '/dashboard/workMessage/focus')));
