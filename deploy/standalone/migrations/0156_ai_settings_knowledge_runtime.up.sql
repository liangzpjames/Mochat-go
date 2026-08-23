-- Add private knowledge-document persistence and one system-owned session
-- analysis assistant per tenant/corp. Uploaded content is never exposed through
-- the public /static file tree.

ALTER TABLE `mochat_go_ai_agents`
  ADD COLUMN `system_key` varchar(64) NULL AFTER `corp_id`;

CREATE UNIQUE INDEX `uq_ai_agents_system_key`
  ON `mochat_go_ai_agents` (`tenant_id`, `corp_id`, `system_key`);

CREATE TABLE IF NOT EXISTS `mochat_go_ai_knowledge_documents` (
  `id` varchar(36) NOT NULL,
  `tenant_id` bigint NOT NULL,
  `corp_id` bigint NOT NULL,
  `knowledge_base_id` varchar(36) NOT NULL,
  `filename` varchar(255) NOT NULL,
  `extension` varchar(16) NOT NULL,
  `mime_type` varchar(128) NOT NULL,
  `object_key` varchar(512) NOT NULL,
  `size_bytes` bigint NOT NULL,
  `sha256` char(64) NOT NULL,
  `status` varchar(16) NOT NULL,
  `error_summary` varchar(512) NOT NULL DEFAULT '',
  `character_count` int NOT NULL DEFAULT 0,
  `chunk_count` int NOT NULL DEFAULT 0,
  `created_by` bigint NOT NULL,
  `updated_by` bigint NOT NULL,
  `created_at` datetime(6) NOT NULL,
  `updated_at` datetime(6) NOT NULL,
  `deleted_at` datetime(6) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_ai_documents_scope` (`tenant_id`, `corp_id`, `knowledge_base_id`, `deleted_at`),
  KEY `idx_ai_documents_ready` (`tenant_id`, `corp_id`, `status`, `updated_at`),
  KEY `idx_ai_documents_checksum` (`tenant_id`, `corp_id`, `knowledge_base_id`, `sha256`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `mochat_go_ai_knowledge_chunks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint NOT NULL,
  `corp_id` bigint NOT NULL,
  `knowledge_base_id` varchar(36) NOT NULL,
  `document_id` varchar(36) NOT NULL,
  `ordinal` int NOT NULL,
  `content` text NOT NULL,
  `character_count` int NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_ai_chunk_ordinal` (`document_id`, `ordinal`),
  KEY `idx_ai_chunks_scope` (`tenant_id`, `corp_id`, `knowledge_base_id`, `document_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO `mochat_go_ai_agents`
  (`id`, `tenant_id`, `corp_id`, `system_key`, `name`, `description`, `knowledge_base_ids`, `status`, `created_by`, `updated_by`, `created_at`, `updated_at`, `deleted_at`)
SELECT UUID(), binding.`tenant_id`, binding.`corp_id`, 'session-analysis',
  '会话分析助手', '分析企业微信会话中的客户意向、流失风险与员工服务质量。',
  JSON_ARRAY(), 1, 0, 0, NOW(6), NOW(6), NULL
FROM `mochat_go_tenant_corp_bindings` binding
WHERE binding.`status` = 2
  AND NOT EXISTS (
    SELECT 1 FROM `mochat_go_ai_agents` existing
    WHERE existing.`tenant_id` = binding.`tenant_id`
      AND existing.`corp_id` = binding.`corp_id`
      AND existing.`system_key` = 'session-analysis'
  );

INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT permission.`id`, 'api', route.`http_method`, route.`path_pattern`, 0, 1, 1
FROM `mochat_go_dashboard_permissions` permission
INNER JOIN (
  SELECT 'GET' AS `http_method`, '/dashboard/ai-settings/knowledge-bases/{id}/documents' AS `path_pattern`
  UNION ALL SELECT 'POST', '/dashboard/ai-settings/knowledge-bases/{id}/documents'
  UNION ALL SELECT 'DELETE', '/dashboard/ai-settings/knowledge-bases/{id}/documents/{documentId}'
) route
WHERE permission.`code` = 'dashboard.ai_setting.ai_knowledge_base'
  AND NOT EXISTS (
    SELECT 1 FROM `mochat_go_dashboard_permission_resources` existing
    WHERE existing.`permission_id` = permission.`id`
      AND existing.`resource_type` = 'api'
      AND existing.`http_method` = route.`http_method`
      AND existing.`path_pattern` = route.`path_pattern`
  );

-- Custom agents are intentionally unavailable until another product surface
-- consumes them. Remove obsolete create/delete authorization while preserving
-- the historical records themselves.
DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
WHERE permission.`code` = 'dashboard.ai_setting.agent'
  AND resource.`resource_type` = 'api'
  AND (
    (resource.`http_method` = 'POST' AND resource.`path_pattern` = '/dashboard/ai-settings/agents')
    OR (resource.`http_method` = 'DELETE' AND resource.`path_pattern` = '/dashboard/ai-settings/agents/{id}')
  );
