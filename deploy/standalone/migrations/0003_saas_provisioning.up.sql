-- SaaS provisioning metadata for the standalone Go runtime.
-- These tables intentionally use the mochat_go_ prefix so the original MoChat
-- business schema remains compatible with upstream PHP installations.

CREATE TABLE IF NOT EXISTS `mochat_go_saas_packages` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `code` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '套餐编码',
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '套餐名称',
  `description` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '套餐说明',
  `max_corps` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业数上限，0 表示不限',
  `max_users` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '子账号上限，0 表示不限',
  `max_contacts` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户数上限，0 表示不限',
  `max_rooms` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户群上限，0 表示不限',
  `max_agents` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '应用数上限，0 表示不限',
  `storage_mb` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '素材存储上限 MB，0 表示不限',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '状态：1 启用，2 禁用',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_packages_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 套餐表';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_tenant_packages` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '租户 ID',
  `package_code` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '套餐编码',
  `package_name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '套餐名称',
  `starts_at` timestamp NULL DEFAULT NULL COMMENT '生效时间',
  `expires_at` timestamp NULL DEFAULT NULL COMMENT '到期时间，NULL 表示长期有效',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '状态：1 启用，2 禁用',
  `limits_json` json DEFAULT NULL COMMENT '开通时写入的套餐额度快照',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_tenant_packages_tenant` (`tenant_id`),
  KEY `idx_mochat_go_saas_tenant_packages_package` (`package_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 租户套餐绑定表';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_usage_counters` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '租户 ID',
  `metric` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '用量指标',
  `period_key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'lifetime' COMMENT '统计周期键',
  `used_value` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '当前用量',
  `limit_value` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '额度，0 表示不限',
  `updated_by` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '更新来源',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_usage_metric` (`tenant_id`, `metric`, `period_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 用量计数表';

CREATE TABLE IF NOT EXISTS `mochat_go_seed_versions` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `scope` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'tenant' COMMENT 'seed 范围',
  `target_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '范围对象 ID',
  `seed_name` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'seed 名称',
  `seed_version` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'seed 版本',
  `checksum` char(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '迁移 checksum',
  `metadata` json DEFAULT NULL COMMENT 'seed 应用元数据',
  `applied_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_seed_versions_scope` (`scope`, `target_id`, `seed_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 seed 版本记录表';

CREATE TABLE IF NOT EXISTS `mochat_go_tenant_provision_runs` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `run_key` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '幂等开通键',
  `tenant_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '租户 ID',
  `package_code` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '套餐编码',
  `admin_phone` char(11) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '管理员手机号',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '状态：1 成功，2 失败',
  `message` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '执行说明',
  `started_at` timestamp NULL DEFAULT NULL,
  `finished_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_tenant_provision_runs_key` (`run_key`),
  KEY `idx_mochat_go_tenant_provision_runs_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版租户开通记录表';
