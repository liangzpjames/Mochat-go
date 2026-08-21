DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission
  ON permission.`id` = resource.`permission_id`
WHERE permission.`code` = 'dashboard.chat.export'
  AND resource.`resource_type` = 'api'
  AND (
    resource.`path_pattern` IN (
      '/dashboard/workMessage/exportCandidates',
      '/dashboard/workMessage/exportTasks',
      '/dashboard/workMessage/exportDownload'
    )
  );

DROP TABLE IF EXISTS `mochat_go_work_message_export_tasks`;
