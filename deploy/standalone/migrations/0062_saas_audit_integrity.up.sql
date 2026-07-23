ALTER TABLE `mochat_go_saas_admin_operation_logs`
  ADD COLUMN `integrity_prev_hash` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '前一条审计日志结构摘要' AFTER `remark`,
  ADD COLUMN `integrity_hash` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '当前审计日志结构摘要' AFTER `integrity_prev_hash`,
  ADD COLUMN `integrity_version` tinyint(3) unsigned NOT NULL DEFAULT '0' COMMENT '审计完整性算法版本，0 表示 legacy' AFTER `integrity_hash`,
  ADD KEY `idx_mochat_go_saas_admin_ops_integrity` (`tenant_id`, `integrity_version`, `id`);

CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_audit_chains` (
  `tenant_id` int(10) unsigned NOT NULL COMMENT '受影响租户 ID，平台级操作为 0',
  `anchor_log_id` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT 'legacy 锚点覆盖的最后日志 ID',
  `anchor_hash` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT 'legacy 日志集合锚点摘要',
  `legacy_log_count` bigint(20) unsigned NOT NULL DEFAULT '0',
  `last_log_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `last_hash` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '当前链头摘要',
  `signed_log_count` bigint(20) unsigned NOT NULL DEFAULT '0',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'unsealed' COMMENT 'unsealed/healthy/failed',
  `sealed_at` datetime DEFAULT NULL,
  `last_verified_at` datetime DEFAULT NULL,
  `last_verification_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `last_failed_log_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `last_verification_error` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`tenant_id`),
  KEY `idx_mochat_go_saas_admin_audit_chain_status` (`status`, `last_verified_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='SaaS 总后台审计结构摘要链状态';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_audit_verifications` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT '0',
  `source` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'manual',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'healthy' COMMENT 'healthy/failed',
  `anchor_log_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `anchor_hash` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  `legacy_log_count` bigint(20) unsigned NOT NULL DEFAULT '0',
  `signed_log_count` bigint(20) unsigned NOT NULL DEFAULT '0',
  `verified_log_count` bigint(20) unsigned NOT NULL DEFAULT '0',
  `chain_head_log_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `chain_head_hash` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  `failed_log_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `error_message` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `actor_user_id` int(10) unsigned NOT NULL DEFAULT '0',
  `actor_tenant_id` int(10) unsigned NOT NULL DEFAULT '0',
  `started_at` datetime NOT NULL,
  `finished_at` datetime NOT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mochat_go_saas_admin_audit_verify_tenant` (`tenant_id`, `id`),
  KEY `idx_mochat_go_saas_admin_audit_verify_status` (`status`, `finished_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='SaaS 总后台审计完整性校验记录';

INSERT INTO `mochat_go_saas_admin_role_permissions` (`role_id`, `permission_code`, `created_at`)
SELECT r.id, p.permission_code, NOW()
FROM `mochat_go_saas_admin_roles` r
JOIN (
  SELECT 'platform.audit.manage' AS permission_code
) p
WHERE r.code = 'platform_operations'
  AND NOT EXISTS (
    SELECT 1
    FROM `mochat_go_saas_admin_role_permissions` rp
    WHERE rp.role_id = r.id AND rp.permission_code = p.permission_code
  );
