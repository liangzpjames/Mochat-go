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

INSERT IGNORE INTO `mochat_go_saas_admin_role_permissions` (`role_id`, `permission_code`, `created_at`)
SELECT r.id, p.permission_code, NOW()
FROM `mochat_go_saas_admin_roles` r
INNER JOIN (
  SELECT 'platform_operations' AS role_code, 'platform.integrations.read' AS permission_code
  UNION ALL SELECT 'platform_operations', 'platform.integrations.manage'
  UNION ALL SELECT 'platform_auditor', 'platform.integrations.read'
  UNION ALL SELECT 'platform_readonly', 'platform.integrations.read'
) p ON p.role_code = r.code;
