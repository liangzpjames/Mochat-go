DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission
  ON permission.`id` = resource.`permission_id`
WHERE permission.`code` = 'dashboard.chat.v2_group'
  AND resource.`resource_type` = 'api'
  AND resource.`http_method` = 'GET'
  AND resource.`path_pattern` IN (
    '/dashboard/workMessage/roomDirectory',
    '/dashboard/workMessage/roomProfile',
    '/dashboard/workMessage/roomMessages',
    '/dashboard/workMessage/roomMembers',
    '/dashboard/workMessage/roomFilterOptions'
  );

DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission
  ON permission.`id` = resource.`permission_id`
WHERE permission.`code` = 'dashboard.chat.trajectory'
  AND resource.`resource_type` = 'api'
  AND resource.`http_method` = 'GET'
  AND resource.`path_pattern` = '/dashboard/workMessage/trajectoryDay';

DROP TABLE IF EXISTS `mochat_go_work_message_participant_identity`;
