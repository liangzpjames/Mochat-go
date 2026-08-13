-- Reconcile Phase 3 provider schema after historical ledger/DDL drift.
-- This migration is intentionally additive and idempotent. It never drops or
-- truncates business data, and it does not recreate legacy menu authorization.

-- Reconciled from 0006_saas_alerts.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_saas_alerts` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `alert_key` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '告警聚合键',
  `tenant_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '租户 ID',
  `alert_type` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '告警类型',
  `severity` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '告警级别',
  `status` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'open' COMMENT '状态：open/resolved',
  `metric` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'SaaS 用量指标',
  `period_key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'lifetime' COMMENT '统计周期键',
  `current_value` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '当前用量',
  `limit_value` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '额度，0 表示不限',
  `additional_value` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '本次额外用量',
  `occurrence_count` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '触发次数',
  `source` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '告警来源',
  `message` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '告警摘要',
  `context_json` json DEFAULT NULL COMMENT '告警上下文',
  `first_seen_at` datetime DEFAULT NULL COMMENT '首次触发时间',
  `last_seen_at` datetime DEFAULT NULL COMMENT '最近触发时间',
  `resolved_at` datetime DEFAULT NULL COMMENT '解决时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_alerts_key` (`alert_key`),
  KEY `idx_mochat_go_saas_alerts_tenant_status` (`tenant_id`, `status`, `last_seen_at`),
  KEY `idx_mochat_go_saas_alerts_metric_status` (`metric`, `status`, `last_seen_at`),
  KEY `idx_mochat_go_saas_alerts_type_status` (`alert_type`, `status`, `last_seen_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 告警账本';

-- Reconciled from 0049_saas_service_accounts.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_saas_service_accounts` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL COMMENT '绑定的业务租户',
  `code` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '租户内稳定编码',
  `name` varchar(120) COLLATE utf8mb4_unicode_ci NOT NULL,
  `description` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active' COMMENT 'active 或 disabled',
  `scopes_json` json NOT NULL COMMENT '允许的 OpenAPI Scope',
  `allowed_cidrs_json` json DEFAULT NULL COMMENT '可选客户端 IP/CIDR 白名单',
  `expires_at` datetime DEFAULT NULL,
  `last_used_at` datetime DEFAULT NULL,
  `last_used_ip` varchar(45) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `use_count` bigint(20) unsigned NOT NULL DEFAULT '0',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_by` int(10) unsigned NOT NULL DEFAULT '0',
  `updated_by` int(10) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_service_account_tenant_code` (`tenant_id`, `code`),
  KEY `idx_mochat_go_saas_service_account_tenant_status` (`tenant_id`, `status`, `id`),
  KEY `idx_mochat_go_saas_service_account_expiry` (`status`, `expires_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 服务账号';

-- Reconciled from 0049_saas_service_accounts.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_saas_service_account_keys` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `service_account_id` bigint(20) unsigned NOT NULL,
  `name` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `key_prefix` varchar(32) COLLATE utf8mb4_bin NOT NULL COMMENT '非敏感查找前缀',
  `key_hash` char(64) COLLATE utf8mb4_bin NOT NULL COMMENT 'HMAC-SHA256 摘要',
  `last_four` char(4) COLLATE utf8mb4_bin NOT NULL COMMENT '界面识别用后四位',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active' COMMENT 'active/retiring/revoked',
  `expires_at` datetime DEFAULT NULL,
  `retire_at` datetime DEFAULT NULL COMMENT '轮换宽限截止时间',
  `last_used_at` datetime DEFAULT NULL,
  `last_used_ip` varchar(45) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `use_count` bigint(20) unsigned NOT NULL DEFAULT '0',
  `replaced_by_key_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `revoked_at` datetime DEFAULT NULL,
  `revoked_by` int(10) unsigned NOT NULL DEFAULT '0',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_by` int(10) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_service_account_key_prefix` (`key_prefix`),
  UNIQUE KEY `uni_mochat_go_saas_service_account_key_hash` (`key_hash`),
  KEY `idx_mochat_go_saas_service_account_key_account` (`service_account_id`, `status`, `id`),
  KEY `idx_mochat_go_saas_service_account_key_expiry` (`status`, `expires_at`, `retire_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 服务账号 API Key';

-- Reconciled from 0100_scrm_customer_lifecycle.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_scrm_contacts` (
  `id` varchar(36) NOT NULL,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `name` varchar(200) NOT NULL,
  `phone` varchar(64) NOT NULL DEFAULT '',
  `version` bigint unsigned NOT NULL DEFAULT 1,
  `deleted_at` datetime(6) NULL,
  `created_at` datetime(6) NOT NULL,
  `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`), KEY `idx_scrm_contacts_scope` (`tenant_id`,`corp_id`,`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Reconciled from 0100_scrm_customer_lifecycle.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_scrm_assignments` (
  `id` varchar(36) NOT NULL, `tenant_id` bigint unsigned NOT NULL, `corp_id` bigint unsigned NOT NULL,
  `contact_id` varchar(36) NOT NULL, `owner_id` bigint unsigned NULL, `status` varchar(32) NOT NULL,
  `version` bigint unsigned NOT NULL DEFAULT 1, `deleted_at` datetime(6) NULL,
  `created_at` datetime(6) NOT NULL, `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_scrm_assignment_contact` (`tenant_id`,`corp_id`,`contact_id`,`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Reconciled from 0100_scrm_customer_lifecycle.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_scrm_stages` (
  `id` varchar(36) NOT NULL, `tenant_id` bigint unsigned NOT NULL, `corp_id` bigint unsigned NOT NULL,
  `name` varchar(100) NOT NULL, `sort_order` int NOT NULL DEFAULT 0, `version` bigint unsigned NOT NULL DEFAULT 1,
  `deleted_at` datetime(6) NULL, `created_at` datetime(6) NOT NULL, `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_scrm_stage_name` (`tenant_id`,`corp_id`,`name`,`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Reconciled from 0100_scrm_customer_lifecycle.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_scrm_opportunities` (
  `id` varchar(36) NOT NULL, `tenant_id` bigint unsigned NOT NULL, `corp_id` bigint unsigned NOT NULL,
  `contact_id` varchar(36) NOT NULL, `stage_id` varchar(36) NOT NULL, `status` varchar(32) NOT NULL,
  `lost_reason` varchar(500) NOT NULL DEFAULT '', `version` bigint unsigned NOT NULL DEFAULT 1,
  `deleted_at` datetime(6) NULL, `created_at` datetime(6) NOT NULL, `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`), KEY `idx_scrm_opportunity_stage` (`tenant_id`,`corp_id`,`stage_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Reconciled from 0100_scrm_customer_lifecycle.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_scrm_follow_ups` (
  `id` varchar(36) NOT NULL, `tenant_id` bigint unsigned NOT NULL, `corp_id` bigint unsigned NOT NULL,
  `contact_id` varchar(36) NOT NULL, `content` text NOT NULL, `created_by` bigint unsigned NOT NULL,
  `created_at` datetime(6) NOT NULL, PRIMARY KEY (`id`), KEY `idx_scrm_follow_up_contact` (`tenant_id`,`corp_id`,`contact_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Reconciled from 0100_scrm_customer_lifecycle.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_scrm_tags` (
  `id` varchar(36) NOT NULL, `tenant_id` bigint unsigned NOT NULL, `corp_id` bigint unsigned NOT NULL,
  `name` varchar(100) NOT NULL, `version` bigint unsigned NOT NULL DEFAULT 1, `deleted_at` datetime(6) NULL,
  `created_at` datetime(6) NOT NULL, `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_scrm_tag_name` (`tenant_id`,`corp_id`,`name`,`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Reconciled from 0100_scrm_customer_lifecycle.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_scrm_contact_tags` (
  `tenant_id` bigint unsigned NOT NULL, `corp_id` bigint unsigned NOT NULL, `contact_id` varchar(36) NOT NULL, `tag_id` varchar(36) NOT NULL,
  `created_at` datetime(6) NOT NULL, PRIMARY KEY (`tenant_id`,`corp_id`,`contact_id`,`tag_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Reconciled from 0109_scrm_customer_tag_parity.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_scrm_tag_groups` (
  `id` varchar(36) NOT NULL,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `name` varchar(100) NOT NULL,
  `version` bigint unsigned NOT NULL DEFAULT 1,
  `deleted_at` datetime(6) NULL,
  `active_name` varchar(100) GENERATED ALWAYS AS (CASE WHEN `deleted_at` IS NULL THEN LOWER(TRIM(`name`)) ELSE NULL END) STORED,
  `created_at` datetime(6) NOT NULL,
  `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_scrm_tag_group_scope` (`tenant_id`,`corp_id`,`name`,`deleted_at`),
  UNIQUE KEY `uk_scrm_tag_group_active_name` (`tenant_id`,`corp_id`,`active_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Reconciled from 0110_risk_behavior_provider.up.sql.
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

-- Reconciled from 0110_risk_behavior_provider.up.sql.
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

-- Reconciled from 0110_risk_behavior_provider.up.sql.
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

-- Reconciled from 0110_risk_behavior_provider.up.sql.
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

-- Reconciled from 0111_timeout_warning_provider.up.sql.
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

-- Reconciled from 0111_timeout_warning_provider.up.sql.
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

-- Reconciled from 0111_timeout_warning_provider.up.sql.
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

-- Reconciled from 0111_timeout_warning_provider.up.sql.
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

-- Reconciled from 0111_timeout_warning_provider.up.sql.
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

-- Reconciled from 0111_timeout_warning_provider.up.sql.
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

-- Reconciled from 0111_timeout_warning_provider.up.sql.
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

-- Reconciled from 0111_timeout_warning_provider.up.sql.
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

-- Reconciled from 0112_message_intercept_keyword_library_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_keyword_libraries` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `corp_id` bigint unsigned NOT NULL,
  `name` varchar(80) NOT NULL,
  `description` varchar(255) NOT NULL DEFAULT '',
  `match_mode` varchar(16) NOT NULL DEFAULT 'contains',
  `status` varchar(16) NOT NULL DEFAULT 'enabled',
  `draft_version` int unsigned NOT NULL DEFAULT 1,
  `published_version` int unsigned NOT NULL DEFAULT 0,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_keyword_library_name` (`tenant_id`,`corp_id`,`name`),
  KEY `idx_keyword_library_scope` (`tenant_id`,`corp_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0112_message_intercept_keyword_library_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_keyword_entries` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT, `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `corp_id` bigint unsigned NOT NULL, `library_id` bigint unsigned NOT NULL,
  `keyword` varchar(120) NOT NULL, `status` varchar(16) NOT NULL DEFAULT 'enabled',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP, `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_keyword_entry` (`tenant_id`,`corp_id`,`library_id`,`keyword`),
  KEY `idx_keyword_entry_library` (`tenant_id`,`corp_id`,`library_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0112_message_intercept_keyword_library_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_keyword_versions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT, `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `corp_id` bigint unsigned NOT NULL, `library_id` bigint unsigned NOT NULL, `version` int unsigned NOT NULL,
  `entry_count` int unsigned NOT NULL DEFAULT 0, `publisher_id` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_keyword_version` (`tenant_id`,`corp_id`,`library_id`,`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0112_message_intercept_keyword_library_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_keyword_version_entries` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT, `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `corp_id` bigint unsigned NOT NULL, `library_id` bigint unsigned NOT NULL, `version` int unsigned NOT NULL,
  `source_entry_id` bigint unsigned NOT NULL, `keyword` varchar(120) NOT NULL,
  PRIMARY KEY (`id`), KEY `idx_keyword_version_entry` (`tenant_id`,`corp_id`,`library_id`,`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0112_message_intercept_keyword_library_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_message_intercept_rules` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT, `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `corp_id` bigint unsigned NOT NULL, `name` varchar(80) NOT NULL, `library_id` bigint unsigned NOT NULL,
  `library_version` int unsigned NOT NULL, `conversation_scopes_json` json NOT NULL,
  `decision` varchar(24) NOT NULL DEFAULT 'blocked', `status` varchar(16) NOT NULL DEFAULT 'enabled',
  `trigger_count` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP, `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_intercept_rule_name` (`tenant_id`,`corp_id`,`name`),
  KEY `idx_intercept_rule_scope` (`tenant_id`,`corp_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0112_message_intercept_keyword_library_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_message_intercept_records` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT, `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `corp_id` bigint unsigned NOT NULL, `rule_id` bigint unsigned NOT NULL, `rule_name` varchar(80) NOT NULL,
  `library_id` bigint unsigned NOT NULL, `library_version` int unsigned NOT NULL,
  `conversation_type` varchar(16) NOT NULL, `conversation_id` varchar(128) NOT NULL DEFAULT '',
  `message_id` varchar(128) NOT NULL DEFAULT '', `sender_id` varchar(128) NOT NULL DEFAULT '',
  `sender_name` varchar(80) NOT NULL DEFAULT '', `message_content` text NOT NULL,
  `matched_keywords_json` json NOT NULL, `decision` varchar(24) NOT NULL, `explanation` varchar(255) NOT NULL,
  `audit_status` varchar(16) NOT NULL DEFAULT 'pending', `occurred_at` datetime NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`), KEY `idx_intercept_record_scope` (`tenant_id`,`corp_id`,`occurred_at`),
  KEY `idx_intercept_record_rule` (`tenant_id`,`corp_id`,`rule_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0112_message_intercept_keyword_library_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_message_intercept_audits` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT, `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `corp_id` bigint unsigned NOT NULL, `record_id` bigint unsigned NOT NULL, `action` varchar(16) NOT NULL,
  `actor_id` bigint unsigned NOT NULL DEFAULT 0, `remark` varchar(255) NOT NULL DEFAULT '',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`), KEY `idx_intercept_audit_record` (`tenant_id`,`corp_id`,`record_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0113_silent_customer_refuse_archive_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_silent_customer_rules` (
 `id` bigint unsigned NOT NULL AUTO_INCREMENT,`tenant_id` bigint unsigned NOT NULL DEFAULT 0,`corp_id` bigint unsigned NOT NULL,
 `name` varchar(80) NOT NULL,`silent_days` int unsigned NOT NULL,`status` varchar(16) NOT NULL DEFAULT 'enabled',`trigger_count` bigint unsigned NOT NULL DEFAULT 0,
 `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,`updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 PRIMARY KEY(`id`),UNIQUE KEY `uk_silent_rule_name`(`tenant_id`,`corp_id`,`name`),KEY `idx_silent_rule_scope`(`tenant_id`,`corp_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0113_silent_customer_refuse_archive_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_silent_customer_records` (
 `id` bigint unsigned NOT NULL AUTO_INCREMENT,`tenant_id` bigint unsigned NOT NULL DEFAULT 0,`corp_id` bigint unsigned NOT NULL,`rule_id` bigint unsigned NOT NULL,`rule_name` varchar(80) NOT NULL,
 `customer_id` varchar(128) NOT NULL,`customer_name` varchar(80) NOT NULL DEFAULT '',`employee_id` bigint unsigned NOT NULL DEFAULT 0,`employee_name` varchar(80) NOT NULL DEFAULT '',
 `last_interaction_at` datetime NOT NULL,`silent_days` int unsigned NOT NULL,`status` varchar(24) NOT NULL DEFAULT 'pending',`assigned_employee_id` bigint unsigned NOT NULL DEFAULT 0,
 `follow_up_note` varchar(255) NOT NULL DEFAULT '',`created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,`updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 PRIMARY KEY(`id`),UNIQUE KEY `uk_silent_record_rule_customer`(`tenant_id`,`corp_id`,`rule_id`,`customer_id`),KEY `idx_silent_record_scope`(`tenant_id`,`corp_id`,`status`,`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0113_silent_customer_refuse_archive_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_silent_customer_audits` (`id` bigint unsigned NOT NULL AUTO_INCREMENT,`tenant_id` bigint unsigned NOT NULL DEFAULT 0,`corp_id` bigint unsigned NOT NULL,`record_id` bigint unsigned NOT NULL,`action` varchar(24) NOT NULL,`actor_id` bigint unsigned NOT NULL DEFAULT 0,`remark` varchar(255) NOT NULL DEFAULT '',`created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(`id`),KEY `idx_silent_audit_record`(`tenant_id`,`corp_id`,`record_id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0113_silent_customer_refuse_archive_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_refuse_archive_records` (
 `id` bigint unsigned NOT NULL AUTO_INCREMENT,`tenant_id` bigint unsigned NOT NULL DEFAULT 0,`corp_id` bigint unsigned NOT NULL,`subject_type` varchar(16) NOT NULL,`subject_id` varchar(128) NOT NULL,`subject_name` varchar(80) NOT NULL DEFAULT '',
 `employee_id` bigint unsigned NOT NULL DEFAULT 0,`employee_name` varchar(80) NOT NULL DEFAULT '',`authorization_status` varchar(16) NOT NULL,`source` varchar(32) NOT NULL DEFAULT 'archive_provider',
 `refused_at` datetime NULL,`authorized_at` datetime NULL,`last_follow_up_at` datetime NULL,`follow_up_status` varchar(24) NOT NULL DEFAULT 'unfollowed',`follow_up_note` varchar(255) NOT NULL DEFAULT '',
 `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,`updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 PRIMARY KEY(`id`),UNIQUE KEY `uk_refuse_archive_subject`(`tenant_id`,`corp_id`,`subject_type`,`subject_id`,`employee_id`),KEY `idx_refuse_archive_scope`(`tenant_id`,`corp_id`,`authorization_status`,`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0113_silent_customer_refuse_archive_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_refuse_archive_audits` (`id` bigint unsigned NOT NULL AUTO_INCREMENT,`tenant_id` bigint unsigned NOT NULL DEFAULT 0,`corp_id` bigint unsigned NOT NULL,`record_id` bigint unsigned NOT NULL,`action` varchar(24) NOT NULL,`from_status` varchar(24) NOT NULL DEFAULT '',`to_status` varchar(24) NOT NULL DEFAULT '',`actor_id` bigint unsigned NOT NULL DEFAULT 0,`remark` varchar(255) NOT NULL DEFAULT '',`created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(`id`),KEY `idx_refuse_archive_audit_record`(`tenant_id`,`corp_id`,`record_id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0114_friends_circle_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mc_friends_circle_tasks` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `corp_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `creator_name` VARCHAR(100) NOT NULL DEFAULT '',
  `task_name` VARCHAR(100) NOT NULL,
  `send_way` VARCHAR(20) NOT NULL,
  `content` TEXT NOT NULL,
  `target_employees` TEXT NOT NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'draft',
  `completed_total` INT UNSIGNED NOT NULL DEFAULT 0,
  `target_total` INT UNSIGNED NOT NULL DEFAULT 0,
  `external_task_id` VARCHAR(128) NOT NULL DEFAULT '',
  `failure_reason` VARCHAR(500) NOT NULL DEFAULT '',
  `start_at` DATETIME NULL,
  `end_at` DATETIME NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_friends_circle_tasks_corp_created` (`corp_id`, `created_at`),
  KEY `idx_friends_circle_tasks_corp_status` (`corp_id`, `status`),
  KEY `idx_friends_circle_tasks_corp_external` (`corp_id`, `external_task_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0114_friends_circle_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mc_friends_circle_materials` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `corp_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `creator_name` VARCHAR(100) NOT NULL DEFAULT '',
  `name` VARCHAR(100) NOT NULL,
  `type` VARCHAR(20) NOT NULL,
  `content` TEXT NOT NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'available',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_friends_circle_materials_corp_created` (`corp_id`, `created_at`),
  KEY `idx_friends_circle_materials_corp_type` (`corp_id`, `type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0116_phase34_acquisition_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mc_phase34_acquisition_links` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `corp_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `creator_name` VARCHAR(100) NOT NULL DEFAULT '',
  `name` VARCHAR(100) NOT NULL,
  `target_url` VARCHAR(2048) NOT NULL,
  `authorization_status` VARCHAR(32) NOT NULL DEFAULT 'unauthorized',
  `status` VARCHAR(32) NOT NULL DEFAULT 'draft',
  `visit_total` INT UNSIGNED NOT NULL DEFAULT 0,
  `conversion_total` INT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `disabled_at` DATETIME NULL,
  `deleted_at` DATETIME NULL,
  PRIMARY KEY (`id`),
  KEY `idx_phase34_acquisition_links_corp_name` (`corp_id`, `name`, `deleted_at`),
  KEY `idx_phase34_acquisition_links_corp_status` (`corp_id`, `status`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0116_phase34_acquisition_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mc_phase34_customer_services` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `corp_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `creator_name` VARCHAR(100) NOT NULL DEFAULT '',
  `name` VARCHAR(100) NOT NULL,
  `account` VARCHAR(128) NOT NULL,
  `employee_ids` TEXT NOT NULL,
  `receive_mode` VARCHAR(32) NOT NULL DEFAULT 'round_robin',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending_sync',
  `sync_reason` VARCHAR(500) NOT NULL DEFAULT '',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `disabled_at` DATETIME NULL,
  `deleted_at` DATETIME NULL,
  PRIMARY KEY (`id`),
  KEY `idx_phase34_customer_services_corp_name` (`corp_id`, `name`, `deleted_at`),
  KEY `idx_phase34_customer_services_corp_status` (`corp_id`, `status`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0116_phase34_acquisition_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mc_phase34_short_links` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `corp_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `creator_name` VARCHAR(100) NOT NULL DEFAULT '',
  `name` VARCHAR(100) NOT NULL,
  `token` VARCHAR(64) NOT NULL,
  `target_type` VARCHAR(32) NOT NULL DEFAULT 'url',
  `target_url` VARCHAR(2048) NOT NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'active',
  `visit_total` INT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `disabled_at` DATETIME NULL,
  `deleted_at` DATETIME NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_phase34_short_links_token` (`token`),
  KEY `idx_phase34_short_links_corp_name` (`corp_id`, `name`, `deleted_at`),
  KEY `idx_phase34_short_links_corp_status` (`corp_id`, `status`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0116_phase34_acquisition_provider.up.sql.
CREATE TABLE IF NOT EXISTS `mc_phase34_short_link_visits` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `short_link_id` BIGINT UNSIGNED NOT NULL,
  `corp_id` BIGINT UNSIGNED NOT NULL,
  `token` VARCHAR(64) NOT NULL,
  `referer` VARCHAR(2048) NOT NULL DEFAULT '',
  `user_agent` VARCHAR(1000) NOT NULL DEFAULT '',
  `visited_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_phase34_short_link_visits_link` (`short_link_id`, `visited_at`),
  KEY `idx_phase34_short_link_visits_corp` (`corp_id`, `visited_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0117_friends_circle_publish_audit.up.sql.
CREATE TABLE IF NOT EXISTS `mc_friends_circle_task_results` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `corp_id` BIGINT UNSIGNED NOT NULL,
  `task_id` BIGINT UNSIGNED NOT NULL,
  `target_employee_id` BIGINT UNSIGNED NOT NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending',
  `failure_code` VARCHAR(64) NOT NULL DEFAULT '',
  `failure_reason` VARCHAR(500) NOT NULL DEFAULT '',
  `occurred_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_friends_circle_result_target` (`corp_id`, `task_id`, `target_employee_id`),
  KEY `idx_friends_circle_results_task_status` (`corp_id`, `task_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reconciled from 0126_phase3_final_providers.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_audio_objects` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT 0,
  `user_id` int(10) unsigned NOT NULL DEFAULT 0,
  `employee_id` int(10) unsigned NOT NULL DEFAULT 0,
  `corp_id` int(10) unsigned NOT NULL DEFAULT 0,
  `original_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `relative_path` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `content_type` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `size_bytes` bigint(20) unsigned NOT NULL DEFAULT 0,
  `duration_seconds` int(10) unsigned NOT NULL DEFAULT 0,
  `sha256` char(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `deleted_by` int(10) unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_audio_objects_corp_created` (`corp_id`, `deleted_at`, `created_at`),
  KEY `idx_audio_objects_path` (`relative_path`(191))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Phase 3 Final 文件录音音频元数据';

-- Reconciled from 0126_phase3_final_providers.up.sql.
CREATE TABLE IF NOT EXISTS `mochat_go_ai_analysis` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT 0,
  `corp_id` int(10) unsigned NOT NULL DEFAULT 0,
  `page` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'succeeded',
  `payload` json DEFAULT NULL,
  `error` varchar(1024) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_ai_analysis_corp_page_created` (`corp_id`, `page`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Phase 3 Final AI 洞察结果落库';

-- Existing databases can have the base table while a later ALTER was ledgered
-- without taking effect. Every change below is therefore guarded through
-- information_schema and PREPARE so it works on MySQL 5.7 and MariaDB.

-- reconcile: mc_friends_circle_tasks.publish_attempts
SET @provider_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mc_friends_circle_tasks' AND column_name='publish_attempts') = 0,
  'ALTER TABLE `mc_friends_circle_tasks` ADD COLUMN `publish_attempts` INT UNSIGNED NOT NULL DEFAULT 0 AFTER `external_task_id`',
  'SELECT 1'
);
PREPARE provider_schema_reconcile FROM @provider_schema_stmt;
EXECUTE provider_schema_reconcile;
DEALLOCATE PREPARE provider_schema_reconcile;

-- reconcile: mc_friends_circle_tasks.last_callback_at
SET @provider_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mc_friends_circle_tasks' AND column_name='last_callback_at') = 0,
  'ALTER TABLE `mc_friends_circle_tasks` ADD COLUMN `last_callback_at` DATETIME NULL AFTER `failure_reason`',
  'SELECT 1'
);
PREPARE provider_schema_reconcile FROM @provider_schema_stmt;
EXECUTE provider_schema_reconcile;
DEALLOCATE PREPARE provider_schema_reconcile;

-- reconcile: mc_friends_circle_tasks.medium_id
SET @provider_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mc_friends_circle_tasks' AND column_name='medium_id') = 0,
  'ALTER TABLE `mc_friends_circle_tasks` ADD COLUMN `medium_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER `content`',
  'SELECT 1'
);
PREPARE provider_schema_reconcile FROM @provider_schema_stmt;
EXECUTE provider_schema_reconcile;
DEALLOCATE PREPARE provider_schema_reconcile;

SET @provider_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='mc_friends_circle_tasks' AND index_name='idx_friends_circle_tasks_corp_medium') = 0,
  'ALTER TABLE `mc_friends_circle_tasks` ADD KEY `idx_friends_circle_tasks_corp_medium` (`corp_id`, `medium_id`)',
  'SELECT 1'
);
PREPARE provider_schema_reconcile FROM @provider_schema_stmt;
EXECUTE provider_schema_reconcile;
DEALLOCATE PREPARE provider_schema_reconcile;

-- reconcile: mc_work_room_auto_pull.medium_id
SET @provider_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mc_work_room_auto_pull' AND column_name='medium_id') = 0,
  'ALTER TABLE `mc_work_room_auto_pull` ADD COLUMN `medium_id` INT UNSIGNED NOT NULL DEFAULT 0 AFTER `corp_id`',
  'SELECT 1'
);
PREPARE provider_schema_reconcile FROM @provider_schema_stmt;
EXECUTE provider_schema_reconcile;
DEALLOCATE PREPARE provider_schema_reconcile;

SET @provider_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='mc_work_room_auto_pull' AND index_name='idx_work_room_auto_pull_corp_medium') = 0,
  'ALTER TABLE `mc_work_room_auto_pull` ADD KEY `idx_work_room_auto_pull_corp_medium` (`corp_id`, `medium_id`)',
  'SELECT 1'
);
PREPARE provider_schema_reconcile FROM @provider_schema_stmt;
EXECUTE provider_schema_reconcile;
DEALLOCATE PREPARE provider_schema_reconcile;

-- reconcile: mc_contact_message_batch_send.medium_id
SET @provider_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mc_contact_message_batch_send' AND column_name='medium_id') = 0,
  'ALTER TABLE `mc_contact_message_batch_send` ADD COLUMN `medium_id` INT UNSIGNED NOT NULL DEFAULT 0 AFTER `user_id`',
  'SELECT 1'
);
PREPARE provider_schema_reconcile FROM @provider_schema_stmt;
EXECUTE provider_schema_reconcile;
DEALLOCATE PREPARE provider_schema_reconcile;

SET @provider_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='mc_contact_message_batch_send' AND index_name='idx_contact_batch_send_corp_medium') = 0,
  'ALTER TABLE `mc_contact_message_batch_send` ADD KEY `idx_contact_batch_send_corp_medium` (`corp_id`, `medium_id`)',
  'SELECT 1'
);
PREPARE provider_schema_reconcile FROM @provider_schema_stmt;
EXECUTE provider_schema_reconcile;
DEALLOCATE PREPARE provider_schema_reconcile;

-- reconcile: mc_room_message_batch_send.medium_id
SET @provider_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mc_room_message_batch_send' AND column_name='medium_id') = 0,
  'ALTER TABLE `mc_room_message_batch_send` ADD COLUMN `medium_id` INT UNSIGNED NOT NULL DEFAULT 0 AFTER `user_id`',
  'SELECT 1'
);
PREPARE provider_schema_reconcile FROM @provider_schema_stmt;
EXECUTE provider_schema_reconcile;
DEALLOCATE PREPARE provider_schema_reconcile;

SET @provider_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='mc_room_message_batch_send' AND index_name='idx_room_batch_send_corp_medium') = 0,
  'ALTER TABLE `mc_room_message_batch_send` ADD KEY `idx_room_batch_send_corp_medium` (`corp_id`, `medium_id`)',
  'SELECT 1'
);
PREPARE provider_schema_reconcile FROM @provider_schema_stmt;
EXECUTE provider_schema_reconcile;
DEALLOCATE PREPARE provider_schema_reconcile;

-- reconcile: mochat_go_scrm_tags.group_id
SET @provider_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mochat_go_scrm_tags' AND column_name='group_id') = 0,
  'ALTER TABLE `mochat_go_scrm_tags` ADD COLUMN `group_id` varchar(36) NULL AFTER `corp_id`',
  'SELECT 1'
);
PREPARE provider_schema_reconcile FROM @provider_schema_stmt;
EXECUTE provider_schema_reconcile;
DEALLOCATE PREPARE provider_schema_reconcile;

-- reconcile: mochat_go_scrm_tags.active_group_name
SET @provider_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mochat_go_scrm_tags' AND column_name='active_group_name') = 0,
  'ALTER TABLE `mochat_go_scrm_tags` ADD COLUMN `active_group_name` varchar(100) GENERATED ALWAYS AS (CASE WHEN `deleted_at` IS NULL AND `group_id` IS NOT NULL THEN LOWER(TRIM(`name`)) ELSE NULL END) STORED',
  'SELECT 1'
);
PREPARE provider_schema_reconcile FROM @provider_schema_stmt;
EXECUTE provider_schema_reconcile;
DEALLOCATE PREPARE provider_schema_reconcile;

SET @provider_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='mochat_go_scrm_tags' AND index_name='idx_scrm_tag_catalog') = 0,
  'ALTER TABLE `mochat_go_scrm_tags` ADD INDEX `idx_scrm_tag_catalog` (`tenant_id`,`corp_id`,`group_id`,`name`,`deleted_at`)',
  'SELECT 1'
);
PREPARE provider_schema_reconcile FROM @provider_schema_stmt;
EXECUTE provider_schema_reconcile;
DEALLOCATE PREPARE provider_schema_reconcile;

SET @provider_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='mochat_go_scrm_tags' AND index_name='uk_scrm_tag_active_group_name') = 0,
  'ALTER TABLE `mochat_go_scrm_tags` ADD UNIQUE INDEX `uk_scrm_tag_active_group_name` (`tenant_id`,`corp_id`,`group_id`,`active_group_name`)',
  'SELECT 1'
);
PREPARE provider_schema_reconcile FROM @provider_schema_stmt;
EXECUTE provider_schema_reconcile;
DEALLOCATE PREPARE provider_schema_reconcile;

SET @provider_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='mochat_go_scrm_contact_tags' AND index_name='idx_scrm_contact_tag_usage') = 0,
  'ALTER TABLE `mochat_go_scrm_contact_tags` ADD INDEX `idx_scrm_contact_tag_usage` (`tenant_id`,`corp_id`,`tag_id`,`contact_id`)',
  'SELECT 1'
);
PREPARE provider_schema_reconcile FROM @provider_schema_stmt;
EXECUTE provider_schema_reconcile;
DEALLOCATE PREPARE provider_schema_reconcile;



