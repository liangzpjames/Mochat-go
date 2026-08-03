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

CREATE TABLE IF NOT EXISTS `mochat_go_keyword_entries` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT, `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `corp_id` bigint unsigned NOT NULL, `library_id` bigint unsigned NOT NULL,
  `keyword` varchar(120) NOT NULL, `status` varchar(16) NOT NULL DEFAULT 'enabled',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP, `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_keyword_entry` (`tenant_id`,`corp_id`,`library_id`,`keyword`),
  KEY `idx_keyword_entry_library` (`tenant_id`,`corp_id`,`library_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `mochat_go_keyword_versions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT, `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `corp_id` bigint unsigned NOT NULL, `library_id` bigint unsigned NOT NULL, `version` int unsigned NOT NULL,
  `entry_count` int unsigned NOT NULL DEFAULT 0, `publisher_id` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_keyword_version` (`tenant_id`,`corp_id`,`library_id`,`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `mochat_go_keyword_version_entries` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT, `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `corp_id` bigint unsigned NOT NULL, `library_id` bigint unsigned NOT NULL, `version` int unsigned NOT NULL,
  `source_entry_id` bigint unsigned NOT NULL, `keyword` varchar(120) NOT NULL,
  PRIMARY KEY (`id`), KEY `idx_keyword_version_entry` (`tenant_id`,`corp_id`,`library_id`,`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

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

CREATE TABLE IF NOT EXISTS `mochat_go_message_intercept_audits` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT, `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `corp_id` bigint unsigned NOT NULL, `record_id` bigint unsigned NOT NULL, `action` varchar(16) NOT NULL,
  `actor_id` bigint unsigned NOT NULL DEFAULT 0, `remark` varchar(255) NOT NULL DEFAULT '',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`), KEY `idx_intercept_audit_record` (`tenant_id`,`corp_id`,`record_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
