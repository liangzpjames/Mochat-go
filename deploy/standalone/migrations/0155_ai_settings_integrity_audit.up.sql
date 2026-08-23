-- Record AI-settings mutations without retaining configuration payloads.
-- This table is additive and is intentionally independent of provider/runtime data.

CREATE TABLE IF NOT EXISTS `mochat_go_ai_settings_audits` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint NOT NULL,
  `corp_id` bigint NOT NULL,
  `actor_user_id` bigint NOT NULL,
  `entity_type` varchar(32) NOT NULL,
  `entity_id` varchar(64) NOT NULL,
  `action` varchar(16) NOT NULL,
  `changed_fields` json NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_ai_settings_audits_scope` (`tenant_id`,`corp_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- An agent configuration needs to resolve its selected knowledge-base names.
-- Grant this shared read dependency only; knowledge-base writes remain owned by
-- the AI knowledge-base page permission.
INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT permission.`id`, 'api', resource_seed.`http_method`, resource_seed.`path_pattern`, resource_seed.`scope_required`, 1, 1
FROM `mochat_go_dashboard_permissions` permission
INNER JOIN (
  SELECT 'dashboard.ai_setting.agent' AS `permission_code`, 'GET' AS `http_method`,
    '/dashboard/ai-settings/knowledge-bases' AS `path_pattern`, 0 AS `scope_required`
) resource_seed ON resource_seed.`permission_code` = permission.`code`
WHERE NOT EXISTS (
  SELECT 1
  FROM `mochat_go_dashboard_permission_resources` existing
  WHERE existing.`permission_id` = permission.`id`
    AND existing.`resource_type` = 'api'
    AND existing.`http_method` = resource_seed.`http_method`
    AND existing.`path_pattern` = resource_seed.`path_pattern`
);
