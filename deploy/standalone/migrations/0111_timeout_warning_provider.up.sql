CREATE TABLE IF NOT EXISTS `mochat_go_timeout_rules` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `name` varchar(80) NOT NULL,
  `status` varchar(16) NOT NULL DEFAULT 'enabled',
  `monitor_target` varchar(16) NOT NULL DEFAULT 'all',
  `monitor_target_ids_json` json NOT NULL,
  `conversation_scopes_json` json NOT NULL,
  `ai_insight_enabled` tinyint(1) NOT NULL DEFAULT 1,
  `trigger_count` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` datetime(6) NOT NULL,
  `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_timeout_rule_scope` (`tenant_id`,`corp_id`,`status`,`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `mochat_go_timeout_rule_strategies` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `rule_id` bigint unsigned NOT NULL,
  `timeout_minutes` int unsigned NOT NULL,
  `notify_type` varchar(16) NOT NULL DEFAULT 'none',
  `risk_level` varchar(16) NOT NULL DEFAULT 'low',
  `sort_order` int unsigned NOT NULL DEFAULT 0,
  `created_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_timeout_rule_minutes` (`rule_id`,`timeout_minutes`),
  KEY `idx_timeout_strategy_scope` (`tenant_id`,`corp_id`,`rule_id`,`sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `mochat_go_timeout_rule_quiet_periods` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `rule_id` bigint unsigned NOT NULL,
  `weekday` tinyint unsigned NOT NULL,
  `start_time` time NOT NULL,
  `end_time` time NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_timeout_quiet_rule` (`tenant_id`,`corp_id`,`rule_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `mochat_go_timeout_rule_notify_targets` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `rule_id` bigint unsigned NOT NULL,
  `target_type` varchar(16) NOT NULL,
  `target_id` bigint unsigned NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_timeout_notify_target` (`rule_id`,`target_type`,`target_id`),
  KEY `idx_timeout_notify_scope` (`tenant_id`,`corp_id`,`rule_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `mochat_go_timeout_settings` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `closing_phrases_json` json NOT NULL,
  `whitelist_message_types_json` json NOT NULL,
  `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_timeout_settings_scope` (`tenant_id`,`corp_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `mochat_go_timeout_records` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `rule_id` bigint unsigned NOT NULL,
  `strategy_id` bigint unsigned NOT NULL,
  `conversation_type` varchar(16) NOT NULL,
  `conversation_id` varchar(128) NOT NULL,
  `customer_id` varchar(128) NOT NULL DEFAULT '',
  `customer_name` varchar(128) NOT NULL DEFAULT '',
  `employee_id` bigint unsigned NOT NULL DEFAULT 0,
  `employee_name` varchar(128) NOT NULL DEFAULT '',
  `trigger_message_id` varchar(128) NOT NULL,
  `trigger_message` text NOT NULL,
  `message_type` varchar(32) NOT NULL DEFAULT 'text',
  `timeout_seconds` bigint unsigned NOT NULL,
  `risk_level` varchar(16) NOT NULL,
  `ai_summary` text NULL,
  `audit_status` varchar(16) NOT NULL DEFAULT 'pending',
  `assigned_employee_id` bigint unsigned NOT NULL DEFAULT 0,
  `occurred_at` datetime(6) NOT NULL,
  `created_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_timeout_record_strategy_message` (`tenant_id`,`corp_id`,`rule_id`,`strategy_id`,`trigger_message_id`),
  KEY `idx_timeout_record_query` (`tenant_id`,`corp_id`,`risk_level`,`conversation_type`,`audit_status`,`occurred_at`),
  KEY `idx_timeout_record_customer` (`tenant_id`,`corp_id`,`customer_id`,`occurred_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `mochat_go_timeout_record_audits` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `record_id` bigint unsigned NOT NULL,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `actor_id` bigint unsigned NOT NULL,
  `action` varchar(32) NOT NULL,
  `assigned_employee_id` bigint unsigned NOT NULL DEFAULT 0,
  `remark` varchar(255) NOT NULL DEFAULT '',
  `created_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_timeout_audit_scope` (`tenant_id`,`corp_id`,`record_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `mochat_go_timeout_notification_intents` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `record_id` bigint unsigned NOT NULL,
  `strategy_id` bigint unsigned NOT NULL,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `notify_type` varchar(16) NOT NULL,
  `target_type` varchar(16) NOT NULL,
  `target_id` bigint unsigned NOT NULL DEFAULT 0,
  `status` varchar(16) NOT NULL DEFAULT 'pending',
  `created_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_timeout_notification_target` (`record_id`,`target_type`,`target_id`),
  KEY `idx_timeout_notification_pending` (`tenant_id`,`corp_id`,`status`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
