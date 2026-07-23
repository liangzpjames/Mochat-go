CREATE TABLE IF NOT EXISTS `mochat_go_saas_release_evidence_actions` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `evidence_key` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `owner_user_id` int(10) unsigned NOT NULL DEFAULT '0',
  `due_at` datetime DEFAULT NULL,
  `next_action` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `note` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_by` int(10) unsigned NOT NULL DEFAULT '0',
  `updated_by` int(10) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_release_evidence_action_key` (`evidence_key`),
  KEY `idx_mochat_go_saas_release_evidence_action_owner_due` (`owner_user_id`, `due_at`, `id`),
  KEY `idx_mochat_go_saas_release_evidence_action_due` (`due_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='生产发布补证行动跟踪';

INSERT IGNORE INTO `mochat_go_saas_release_evidence_actions`
  (`evidence_key`, `next_action`, `version`, `created_by`, `updated_by`, `created_at`, `updated_at`)
VALUES
  ('mysql57_amd64', '在 amd64 Linux 目标机运行 MySQL 5.7 真实容器门禁并上传报告工件', 1, 0, 0, NOW(), NOW()),
  ('real_wecom', '使用真实企业微信账号完成授权、回调和消息链路联调并上传报告', 1, 0, 0, NOW(), NOW()),
  ('real_wechat_open', '使用真实微信开放平台账号完成授权、回调和票据刷新联调并上传报告', 1, 0, 0, NOW(), NOW()),
  ('real_saas_tenants', '使用至少两个真实业务租户完成隔离、订阅和核心流程回归并上传报告', 1, 0, 0, NOW(), NOW()),
  ('production_frontend', '在生产域名完成桌面与移动端浏览器回归并上传报告', 1, 0, 0, NOW(), NOW()),
  ('stability', '在目标环境完成短时健康与外部监控记录并上传报告', 1, 0, 0, NOW(), NOW());
