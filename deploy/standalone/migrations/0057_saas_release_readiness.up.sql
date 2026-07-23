CREATE TABLE IF NOT EXISTS `mochat_go_saas_release_evidence` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `evidence_key` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `title` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `category` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `required` tinyint(1) unsigned NOT NULL DEFAULT '1',
  `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'missing' COMMENT 'missing/in_progress/passed/failed',
  `evidence_url` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `environment` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `source_fingerprint` char(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `checked_at` datetime DEFAULT NULL,
  `checked_by` int(10) unsigned NOT NULL DEFAULT '0',
  `note` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_release_evidence_key` (`evidence_key`),
  KEY `idx_mochat_go_saas_release_evidence_status` (`required`, `status`, `id`),
  KEY `idx_mochat_go_saas_release_evidence_checked` (`checked_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 生产发布证据台账';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_release_candidates` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `candidate_no` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `release_version` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `source_fingerprint` char(64) COLLATE utf8mb4_bin NOT NULL,
  `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'blocked/ready',
  `required_count` int(10) unsigned NOT NULL DEFAULT '0',
  `passed_count` int(10) unsigned NOT NULL DEFAULT '0',
  `matched_count` int(10) unsigned NOT NULL DEFAULT '0',
  `snapshot_json` json NOT NULL,
  `gate_message` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_by` int(10) unsigned NOT NULL DEFAULT '0',
  `operation_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_release_candidate_no` (`candidate_no`),
  KEY `idx_mochat_go_saas_release_candidate_status` (`status`, `created_at`, `id`),
  KEY `idx_mochat_go_saas_release_candidate_fingerprint` (`source_fingerprint`, `created_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 不可变发布候选门禁快照';

INSERT IGNORE INTO `mochat_go_saas_release_evidence`
  (`evidence_key`, `title`, `category`, `required`, `status`, `version`, `created_at`, `updated_at`)
VALUES
  ('mysql57_amd64', 'MySQL 5.7 amd64 真实容器门禁', 'database', 1, 'missing', 1, NOW(), NOW()),
  ('real_wecom', '真实企业微信账号联调', 'integration', 1, 'missing', 1, NOW(), NOW()),
  ('real_wechat_open', '真实微信开放平台联调', 'integration', 1, 'missing', 1, NOW(), NOW()),
  ('real_saas_tenants', '真实 SaaS 多租户数据回归', 'tenant', 1, 'missing', 1, NOW(), NOW()),
  ('production_frontend', '生产前端浏览器回归', 'frontend', 1, 'missing', 1, NOW(), NOW()),
  ('stability', '目标环境稳定性记录', 'reliability', 1, 'missing', 1, NOW(), NOW());

INSERT IGNORE INTO `mochat_go_saas_admin_role_permissions` (`role_id`, `permission_code`, `created_at`)
SELECT r.id, p.permission_code, NOW()
FROM `mochat_go_saas_admin_roles` r
INNER JOIN (
  SELECT 'platform_operations' AS role_code, 'platform.release.read' AS permission_code
  UNION ALL SELECT 'platform_operations', 'platform.release.manage'
  UNION ALL SELECT 'platform_approver', 'platform.release.read'
  UNION ALL SELECT 'platform_auditor', 'platform.release.read'
  UNION ALL SELECT 'platform_readonly', 'platform.release.read'
) p ON p.role_code = r.code;
