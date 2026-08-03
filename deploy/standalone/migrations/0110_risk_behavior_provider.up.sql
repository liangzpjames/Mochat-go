CREATE TABLE IF NOT EXISTS `mochat_go_risk_rules` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `name` varchar(80) NOT NULL,
  `status` varchar(16) NOT NULL DEFAULT 'enabled',
  `subject` varchar(16) NOT NULL DEFAULT 'employee',
  `whitelist_json` json NOT NULL,
  `ai_insight_enabled` tinyint(1) NOT NULL DEFAULT 1,
  `trigger_count` bigint unsigned NOT NULL DEFAULT 0,
  `created_by` bigint unsigned NOT NULL DEFAULT 0,
  `updated_by` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` datetime(6) NOT NULL,
  `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`), KEY `idx_risk_rule_scope` (`tenant_id`,`corp_id`,`status`,`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `mochat_go_risk_rule_strategies` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `rule_id` bigint unsigned NOT NULL,
  `behavior` varchar(32) NOT NULL,
  `pattern` varchar(255) NOT NULL,
  `notify_type` varchar(16) NOT NULL DEFAULT 'none',
  `risk_level` varchar(16) NOT NULL DEFAULT 'low',
  `created_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_risk_rule_behavior` (`rule_id`,`behavior`), KEY `idx_risk_strategy_behavior` (`behavior`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `mochat_go_risk_records` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `rule_id` bigint unsigned NOT NULL,
  `strategy_id` bigint unsigned NOT NULL,
  `behavior` varchar(32) NOT NULL,
  `risk_level` varchar(16) NOT NULL,
  `conversation_type` varchar(16) NOT NULL,
  `conversation_id` varchar(128) NOT NULL,
  `message_id` varchar(128) NOT NULL,
  `trigger_message` text NOT NULL,
  `related_user_json` json NOT NULL,
  `ai_summary` text NULL,
  `audit_status` varchar(16) NOT NULL DEFAULT 'pending',
  `occurred_at` datetime(6) NOT NULL,
  `created_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_risk_record_message_strategy` (`tenant_id`,`corp_id`,`message_id`,`strategy_id`), KEY `idx_risk_record_query` (`tenant_id`,`corp_id`,`risk_level`,`behavior`,`occurred_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `mochat_go_risk_record_audits` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `record_id` bigint unsigned NOT NULL,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `actor_id` bigint unsigned NOT NULL,
  `action` varchar(32) NOT NULL,
  `remark` varchar(255) NOT NULL DEFAULT '',
  `created_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`), KEY `idx_risk_audit_scope` (`tenant_id`,`corp_id`,`record_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
