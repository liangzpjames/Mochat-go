CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_roles` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `code` varchar(48) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '平台岗位编码',
  `name` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '平台岗位名称',
  `description` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` tinyint(3) unsigned NOT NULL DEFAULT '1' COMMENT '1 启用，2 停用',
  `is_system` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '是否为内置岗位',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_by` int(10) unsigned NOT NULL DEFAULT '0',
  `updated_by` int(10) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_admin_role_code` (`code`),
  KEY `idx_mochat_go_saas_admin_role_status` (`status`, `is_system`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 平台岗位';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_role_permissions` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `role_id` bigint(20) unsigned NOT NULL,
  `permission_code` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_admin_role_permission` (`role_id`, `permission_code`),
  KEY `idx_mochat_go_saas_admin_permission_code` (`permission_code`, `role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 平台岗位权限';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_user_access` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` int(10) unsigned NOT NULL,
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `updated_by` int(10) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_admin_user_access_user` (`user_id`),
  KEY `idx_mochat_go_saas_admin_user_access_updated` (`updated_at`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 平台人员授权版本';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_user_roles` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` int(10) unsigned NOT NULL,
  `role_id` bigint(20) unsigned NOT NULL,
  `assigned_by` int(10) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_admin_user_role` (`user_id`, `role_id`),
  KEY `idx_mochat_go_saas_admin_user_role_role` (`role_id`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 平台人员岗位';

INSERT IGNORE INTO `mochat_go_saas_admin_roles`
  (`code`, `name`, `description`, `status`, `is_system`, `version`, `created_by`, `updated_by`, `created_at`, `updated_at`)
VALUES
  ('platform_operations', '平台运营', '租户运营、任务、客户成功和通知处置', 1, 1, 1, 0, 0, NOW(), NOW()),
  ('platform_finance', '平台财务', '订阅、收款、退款、发票和结算管理', 1, 1, 1, 0, 0, NOW(), NOW()),
  ('platform_customer_success', '客户成功', '租户、续费、风险和运营待办管理', 1, 1, 1, 0, 0, NOW(), NOW()),
  ('platform_auditor', '审计只读', '全平台只读和操作审计', 1, 1, 1, 0, 0, NOW(), NOW()),
  ('platform_readonly', '平台只读', '除权限治理外的全平台只读访问', 1, 1, 1, 0, 0, NOW(), NOW());

INSERT IGNORE INTO `mochat_go_saas_admin_role_permissions` (`role_id`, `permission_code`, `created_at`)
SELECT r.id, p.permission_code, NOW()
FROM `mochat_go_saas_admin_roles` r
INNER JOIN (
  SELECT 'platform_operations' AS role_code, 'platform.overview.read' AS permission_code
  UNION ALL SELECT 'platform_operations', 'platform.tenants.read'
  UNION ALL SELECT 'platform_operations', 'platform.tenants.manage'
  UNION ALL SELECT 'platform_operations', 'platform.operations.read'
  UNION ALL SELECT 'platform_operations', 'platform.operations.manage'
  UNION ALL SELECT 'platform_operations', 'platform.notifications.read'
  UNION ALL SELECT 'platform_operations', 'platform.notifications.manage'
  UNION ALL SELECT 'platform_operations', 'platform.finance.read'
  UNION ALL SELECT 'platform_operations', 'platform.audit.read'
  UNION ALL SELECT 'platform_finance', 'platform.overview.read'
  UNION ALL SELECT 'platform_finance', 'platform.tenants.read'
  UNION ALL SELECT 'platform_finance', 'platform.finance.read'
  UNION ALL SELECT 'platform_finance', 'platform.finance.manage'
  UNION ALL SELECT 'platform_finance', 'platform.audit.read'
  UNION ALL SELECT 'platform_customer_success', 'platform.overview.read'
  UNION ALL SELECT 'platform_customer_success', 'platform.tenants.read'
  UNION ALL SELECT 'platform_customer_success', 'platform.tenants.manage'
  UNION ALL SELECT 'platform_customer_success', 'platform.operations.read'
  UNION ALL SELECT 'platform_customer_success', 'platform.operations.manage'
  UNION ALL SELECT 'platform_customer_success', 'platform.notifications.read'
  UNION ALL SELECT 'platform_customer_success', 'platform.finance.read'
  UNION ALL SELECT 'platform_auditor', 'platform.overview.read'
  UNION ALL SELECT 'platform_auditor', 'platform.tenants.read'
  UNION ALL SELECT 'platform_auditor', 'platform.operations.read'
  UNION ALL SELECT 'platform_auditor', 'platform.notifications.read'
  UNION ALL SELECT 'platform_auditor', 'platform.finance.read'
  UNION ALL SELECT 'platform_auditor', 'platform.audit.read'
  UNION ALL SELECT 'platform_readonly', 'platform.overview.read'
  UNION ALL SELECT 'platform_readonly', 'platform.tenants.read'
  UNION ALL SELECT 'platform_readonly', 'platform.operations.read'
  UNION ALL SELECT 'platform_readonly', 'platform.notifications.read'
  UNION ALL SELECT 'platform_readonly', 'platform.finance.read'
  UNION ALL SELECT 'platform_readonly', 'platform.audit.read'
) p ON p.role_code = r.code;
