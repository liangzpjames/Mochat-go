-- SaaS storage ledger for files uploaded by the standalone Go runtime.
-- The table is Go-owned metadata and does not modify upstream MoChat tables.

CREATE TABLE IF NOT EXISTS `mochat_go_saas_storage_objects` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '租户 ID',
  `user_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '上传用户 ID',
  `employee_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '侧边栏员工 ID',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业 ID',
  `source` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '上传入口',
  `original_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '原始文件名',
  `relative_path` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '相对存储路径',
  `content_type` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'MIME 类型',
  `size_bytes` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '文件大小，单位字节',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_mochat_go_saas_storage_tenant` (`tenant_id`, `deleted_at`),
  KEY `idx_mochat_go_saas_storage_path` (`relative_path`(191))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 上传文件账本';
