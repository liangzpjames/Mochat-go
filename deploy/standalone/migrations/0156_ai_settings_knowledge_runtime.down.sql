-- Rollback removes the document runtime introduced by 0156. Original agent
-- rows and knowledge-base metadata remain intact; uploaded document metadata
-- must be exported before running this destructive rollback.

DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
WHERE permission.`code` = 'dashboard.ai_setting.ai_knowledge_base'
  AND resource.`resource_type` = 'api'
  AND resource.`path_pattern` IN (
    '/dashboard/ai-settings/knowledge-bases/{id}/documents',
    '/dashboard/ai-settings/knowledge-bases/{id}/documents/{documentId}'
  );

INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT permission.`id`, 'api', route.`http_method`, route.`path_pattern`, 0, 1, 1
FROM `mochat_go_dashboard_permissions` permission
INNER JOIN (
  SELECT 'POST' AS `http_method`, '/dashboard/ai-settings/agents' AS `path_pattern`
  UNION ALL SELECT 'DELETE', '/dashboard/ai-settings/agents/{id}'
) route
WHERE permission.`code` = 'dashboard.ai_setting.agent'
  AND NOT EXISTS (
    SELECT 1 FROM `mochat_go_dashboard_permission_resources` existing
    WHERE existing.`permission_id` = permission.`id`
      AND existing.`resource_type` = 'api'
      AND existing.`http_method` = route.`http_method`
      AND existing.`path_pattern` = route.`path_pattern`
  );

DROP TABLE IF EXISTS `mochat_go_ai_knowledge_chunks`;
DROP TABLE IF EXISTS `mochat_go_ai_knowledge_documents`;

DROP INDEX `uq_ai_agents_system_key` ON `mochat_go_ai_agents`;
ALTER TABLE `mochat_go_ai_agents` DROP COLUMN `system_key`;
