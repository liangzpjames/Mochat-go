DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id`=resource.`permission_id`
WHERE permission.`code`='dashboard.company_setting.staff'
  AND (resource.`http_method`,resource.`path_pattern`) IN (
    ('GET','/dashboard/access/employees'),
    ('POST','/dashboard/access/employees/{id}/account'),
    ('PUT','/dashboard/access/employees/{id}/account/status'),
    ('POST','/dashboard/access/employees/{id}/account/reset-password')
  );

DROP TABLE IF EXISTS `mochat_go_archive_simulation_entities`;
DROP TABLE IF EXISTS `mochat_go_archive_simulation_messages`;
DROP TABLE IF EXISTS `mochat_go_archive_simulation_batches`;
