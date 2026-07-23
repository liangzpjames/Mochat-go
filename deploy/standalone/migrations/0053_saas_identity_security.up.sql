CREATE TABLE IF NOT EXISTS `mochat_go_saas_identity_policies` (
  `tenant_id` int(10) unsigned NOT NULL,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active' COMMENT 'active 或 disabled',
  `max_failed_attempts` int(10) unsigned NOT NULL DEFAULT '5',
  `lockout_minutes` int(10) unsigned NOT NULL DEFAULT '30',
  `session_ttl_minutes` int(10) unsigned NOT NULL DEFAULT '10080',
  `idle_timeout_minutes` int(10) unsigned NOT NULL DEFAULT '1440',
  `max_concurrent_sessions` int(10) unsigned NOT NULL DEFAULT '5',
  `require_mfa` tinyint(3) unsigned NOT NULL DEFAULT '0',
  `allowed_ip_cidrs` json DEFAULT NULL,
  `login_event_retention_days` int(10) unsigned NOT NULL DEFAULT '180',
  `session_retention_days` int(10) unsigned NOT NULL DEFAULT '90',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `updated_by` int(10) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 租户身份安全策略';

INSERT IGNORE INTO `mochat_go_saas_identity_policies`
  (`tenant_id`, `status`, `max_failed_attempts`, `lockout_minutes`, `session_ttl_minutes`,
   `idle_timeout_minutes`, `max_concurrent_sessions`, `require_mfa`, `allowed_ip_cidrs`,
   `login_event_retention_days`, `session_retention_days`, `version`, `updated_by`, `created_at`, `updated_at`)
VALUES
  (1, 'active', 5, 30, 10080, 1440, 5, 0, JSON_ARRAY(), 180, 90, 1, 0, NOW(), NOW());

CREATE TABLE IF NOT EXISTS `mochat_go_saas_identity_user_states` (
  `user_id` int(10) unsigned NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL,
  `failed_attempts` int(10) unsigned NOT NULL DEFAULT '0',
  `locked_until` datetime DEFAULT NULL,
  `last_failed_at` datetime DEFAULT NULL,
  `last_failed_ip` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `last_success_at` datetime DEFAULT NULL,
  `last_success_ip` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `password_changed_at` datetime DEFAULT NULL,
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`user_id`),
  KEY `idx_mochat_go_saas_identity_user_state_tenant` (`tenant_id`, `locked_until`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 用户登录安全状态';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_identity_mfa_credentials` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` int(10) unsigned NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending' COMMENT 'pending/active/disabled',
  `secret_ciphertext` longtext COLLATE utf8mb4_bin NOT NULL,
  `encryption_key_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `recovery_code_hashes` json DEFAULT NULL,
  `recovery_codes_remaining` int(10) unsigned NOT NULL DEFAULT '0',
  `verified_at` datetime DEFAULT NULL,
  `last_used_at` datetime DEFAULT NULL,
  `last_totp_step` bigint(20) unsigned NOT NULL DEFAULT '0',
  `disabled_at` datetime DEFAULT NULL,
  `disabled_by` int(10) unsigned NOT NULL DEFAULT '0',
  `disabled_reason` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_identity_mfa_user` (`user_id`),
  KEY `idx_mochat_go_saas_identity_mfa_tenant_status` (`tenant_id`, `status`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS TOTP 与恢复码凭据';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_identity_auth_challenges` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `challenge_hash` char(64) COLLATE utf8mb4_bin NOT NULL,
  `user_id` int(10) unsigned NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL,
  `challenge_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'mfa_login',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending' COMMENT 'pending/consumed/expired/locked',
  `active_slot` int(10) unsigned DEFAULT NULL COMMENT 'pending 时等于 user_id，唯一约束阻止重复活动挑战',
  `attempts` int(10) unsigned NOT NULL DEFAULT '0',
  `max_attempts` int(10) unsigned NOT NULL DEFAULT '5',
  `ip_address` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `user_agent` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `expires_at` datetime NOT NULL,
  `consumed_at` datetime DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_identity_challenge_hash` (`challenge_hash`),
  UNIQUE KEY `uni_mochat_go_saas_identity_challenge_active` (`active_slot`),
  KEY `idx_mochat_go_saas_identity_challenge_expiry` (`status`, `expires_at`, `id`),
  KEY `idx_mochat_go_saas_identity_challenge_user` (`user_id`, `status`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 登录二次认证挑战';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_identity_sessions` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `session_jti` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `token_sha256` char(64) COLLATE utf8mb4_bin NOT NULL,
  `user_id` int(10) unsigned NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active' COMMENT 'active/revoked/expired',
  `auth_method` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'password',
  `ip_address` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `user_agent` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `issued_at` datetime NOT NULL,
  `expires_at` datetime NOT NULL,
  `idle_expires_at` datetime NOT NULL,
  `last_seen_at` datetime NOT NULL,
  `revoked_at` datetime DEFAULT NULL,
  `revoked_by` int(10) unsigned NOT NULL DEFAULT '0',
  `revocation_reason` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_identity_session_jti` (`session_jti`),
  UNIQUE KEY `uni_mochat_go_saas_identity_session_token` (`token_sha256`),
  KEY `idx_mochat_go_saas_identity_session_user_status` (`user_id`, `status`, `last_seen_at`, `id`),
  KEY `idx_mochat_go_saas_identity_session_tenant_status` (`tenant_id`, `status`, `last_seen_at`, `id`),
  KEY `idx_mochat_go_saas_identity_session_expiry` (`status`, `expires_at`, `idle_expires_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 可撤销登录会话';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_identity_login_events` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` int(10) unsigned NOT NULL DEFAULT '0',
  `tenant_id` int(10) unsigned NOT NULL DEFAULT '0',
  `phone_sha256` char(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `event_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `result` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL,
  `risk_level` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'normal',
  `reason_code` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `ip_address` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `user_agent` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `metadata_json` json DEFAULT NULL,
  `occurred_at` datetime NOT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mochat_go_saas_identity_event_tenant_time` (`tenant_id`, `occurred_at`, `id`),
  KEY `idx_mochat_go_saas_identity_event_user_time` (`user_id`, `occurred_at`, `id`),
  KEY `idx_mochat_go_saas_identity_event_risk_time` (`risk_level`, `occurred_at`, `id`),
  KEY `idx_mochat_go_saas_identity_event_ip_time` (`ip_address`, `occurred_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 不可变登录事件';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_identity_security_incidents` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `incident_no` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `stable_key` varchar(191) COLLATE utf8mb4_bin NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT '0',
  `user_id` int(10) unsigned NOT NULL DEFAULT '0',
  `incident_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `severity` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'warning',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'open' COMMENT 'open/acknowledged/resolved',
  `title` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `latest_detail` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `occurrence_count` int(10) unsigned NOT NULL DEFAULT '1',
  `first_occurred_at` datetime NOT NULL,
  `last_occurred_at` datetime NOT NULL,
  `assigned_to` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `acknowledged_at` datetime DEFAULT NULL,
  `acknowledged_by` int(10) unsigned NOT NULL DEFAULT '0',
  `resolved_at` datetime DEFAULT NULL,
  `resolved_by` int(10) unsigned NOT NULL DEFAULT '0',
  `resolution` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_identity_incident_no` (`incident_no`),
  UNIQUE KEY `uni_mochat_go_saas_identity_incident_stable` (`stable_key`),
  KEY `idx_mochat_go_saas_identity_incident_status` (`status`, `severity`, `last_occurred_at`, `id`),
  KEY `idx_mochat_go_saas_identity_incident_tenant` (`tenant_id`, `status`, `last_occurred_at`, `id`),
  KEY `idx_mochat_go_saas_identity_incident_user` (`user_id`, `status`, `last_occurred_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 登录安全事故';

INSERT IGNORE INTO `mochat_go_saas_admin_role_permissions` (`role_id`, `permission_code`, `created_at`)
SELECT r.id, p.permission_code, NOW()
FROM `mochat_go_saas_admin_roles` r
INNER JOIN (
  SELECT 'platform_operations' AS role_code, 'platform.identity.read' AS permission_code
  UNION ALL SELECT 'platform_operations', 'platform.identity.manage'
  UNION ALL SELECT 'platform_approver', 'platform.identity.read'
  UNION ALL SELECT 'platform_auditor', 'platform.identity.read'
  UNION ALL SELECT 'platform_readonly', 'platform.identity.read'
) p ON p.role_code = r.code;
