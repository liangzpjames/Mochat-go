CREATE TABLE IF NOT EXISTS `mochat_go_risk_scan_states` (
  `tenant_id` int unsigned NOT NULL,
  `corp_id` int unsigned NOT NULL,
  `state` varchar(24) NOT NULL DEFAULT 'never_run',
  `last_attempt_at` datetime NULL,
  `last_success_at` datetime NULL,
  `last_failure_at` datetime NULL,
  `last_error` varchar(500) NOT NULL DEFAULT '',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`tenant_id`, `corp_id`),
  KEY `idx_mg_risk_scan_state_updated` (`state`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `mochat_go_sensitive_word_scan_states` (
  `corp_id` int unsigned NOT NULL,
  `state` varchar(24) NOT NULL DEFAULT 'never_run',
  `last_attempt_at` datetime NULL,
  `last_success_at` datetime NULL,
  `last_failure_at` datetime NULL,
  `last_error` varchar(500) NOT NULL DEFAULT '',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`corp_id`),
  KEY `idx_mg_sensitive_scan_state_updated` (`state`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
