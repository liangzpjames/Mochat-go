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

DROP TABLE IF EXISTS `mochat_go_ai_knowledge_chunks`;
DROP TABLE IF EXISTS `mochat_go_ai_knowledge_documents`;

DROP INDEX `uq_ai_agents_system_key` ON `mochat_go_ai_agents`;
ALTER TABLE `mochat_go_ai_agents` DROP COLUMN `system_key`;
