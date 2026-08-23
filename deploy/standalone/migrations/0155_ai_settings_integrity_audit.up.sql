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
