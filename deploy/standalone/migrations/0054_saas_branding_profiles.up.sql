CREATE TABLE IF NOT EXISTS `mochat_go_saas_branding_profiles` (
  `tenant_id` int(10) unsigned NOT NULL,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active' COMMENT 'active 或 disabled',
  `product_name` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL,
  `product_short_name` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `product_subtitle` varchar(160) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `logo_url` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `favicon_url` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `login_background_url` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `primary_color` char(7) COLLATE utf8mb4_bin NOT NULL DEFAULT '#1769AA',
  `accent_color` char(7) COLLATE utf8mb4_bin NOT NULL DEFAULT '#0F578F',
  `website_url` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `support_url` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `support_qr_url` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `support_email` varchar(254) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `docs_url` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `footer_text` varchar(160) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `updated_by` int(10) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`tenant_id`),
  KEY `idx_mochat_go_saas_branding_status` (`status`, `tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 租户白标品牌档案';

INSERT IGNORE INTO `mochat_go_saas_admin_role_permissions` (`role_id`, `permission_code`, `created_at`)
SELECT r.id, p.permission_code, NOW()
FROM `mochat_go_saas_admin_roles` r
INNER JOIN (
  SELECT 'platform_operations' AS role_code, 'platform.branding.read' AS permission_code
  UNION ALL SELECT 'platform_operations', 'platform.branding.manage'
  UNION ALL SELECT 'platform_approver', 'platform.branding.read'
  UNION ALL SELECT 'platform_auditor', 'platform.branding.read'
  UNION ALL SELECT 'platform_readonly', 'platform.branding.read'
) p ON p.role_code = r.code;
