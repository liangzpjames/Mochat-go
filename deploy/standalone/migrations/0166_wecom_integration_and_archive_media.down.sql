-- Remove only 0166-owned permission resources and structures. Existing
-- corp, tenant, and authoritative tenant-corp binding tables are retained.
DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
WHERE permission.`code` = 'dashboard.chat.v2_all'
  AND resource.`resource_type` = 'api'
  AND resource.`http_method` IN ('GET','HEAD')
  AND resource.`path_pattern` = '/dashboard/archive/media/{id}/content';

DELETE FROM `mochat_go_saas_admin_role_permissions`
WHERE `permission_code` IN ('platform.wecom_integrations.read','platform.wecom_integrations.manage');

DROP TABLE IF EXISTS `mochat_go_archive_media_objects`;
DROP TABLE IF EXISTS `mochat_go_wecom_integrations`;
