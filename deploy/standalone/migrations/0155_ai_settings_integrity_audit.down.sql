-- Remove only the shared Agent -> knowledge-base read dependency introduced by
-- 0155. Knowledge-base page resources and AI configuration records remain intact.
DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
WHERE permission.`code` = 'dashboard.ai_setting.agent'
  AND resource.`resource_type` = 'api'
  AND resource.`http_method` = 'GET'
  AND resource.`path_pattern` = '/dashboard/ai-settings/knowledge-bases';

DROP TABLE IF EXISTS `mochat_go_ai_settings_audits`;
