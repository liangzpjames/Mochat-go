CREATE TABLE IF NOT EXISTS `mochat_go_saas_tenant_domains` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL,
  `hostname` varchar(253) COLLATE utf8mb4_bin NOT NULL,
  `hostname_active` varchar(253) COLLATE utf8mb4_bin DEFAULT NULL COMMENT '未删除时等于 hostname，用于允许软删除后重新绑定',
  `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending' COMMENT 'pending/active/disabled',
  `is_primary` tinyint(3) unsigned NOT NULL DEFAULT '0',
  `primary_slot` int(10) unsigned DEFAULT NULL COMMENT '主域名时等于 tenant_id，唯一约束保证单租户只有一个主域名',
  `verification_method` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'dns_txt',
  `verification_token` varchar(86) COLLATE utf8mb4_bin NOT NULL,
  `verification_error` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `last_verification_at` datetime DEFAULT NULL,
  `verified_at` datetime DEFAULT NULL,
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_by` int(10) unsigned NOT NULL DEFAULT '0',
  `updated_by` int(10) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_tenant_domain_hostname` (`hostname_active`),
  UNIQUE KEY `uni_mochat_go_saas_tenant_domain_primary` (`primary_slot`),
  KEY `idx_mochat_go_saas_tenant_domain_tenant` (`tenant_id`, `status`, `id`),
  KEY `idx_mochat_go_saas_tenant_domain_verify` (`status`, `last_verification_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 租户自定义域名与所有权校验';

INSERT IGNORE INTO `mochat_go_saas_admin_role_permissions` (`role_id`, `permission_code`, `created_at`)
SELECT r.id, p.permission_code, NOW()
FROM `mochat_go_saas_admin_roles` r
INNER JOIN (
  SELECT 'platform_operations' AS role_code, 'platform.domains.read' AS permission_code
  UNION ALL SELECT 'platform_operations', 'platform.domains.manage'
  UNION ALL SELECT 'platform_approver', 'platform.domains.read'
  UNION ALL SELECT 'platform_auditor', 'platform.domains.read'
  UNION ALL SELECT 'platform_readonly', 'platform.domains.read'
) p ON p.role_code = r.code;
