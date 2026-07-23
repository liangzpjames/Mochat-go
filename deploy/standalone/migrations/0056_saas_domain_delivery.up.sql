CREATE TABLE IF NOT EXISTS `mochat_go_saas_tenant_domain_deliveries` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `domain_id` bigint(20) unsigned NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL,
  `delivery_status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'unconfigured' COMMENT 'unconfigured/pending/provisioning/ready/degraded/failed/disabled/deleted',
  `routing_status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending' COMMENT 'pending/provisioning/ready/failed/disabled/deleted',
  `certificate_status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending' COMMENT 'pending/provisioning/active/expiring/failed/revoked/disabled/deleted',
  `provider` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `provider_request_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `certificate_id` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '非敏感证书引用，不保存证书或私钥',
  `certificate_not_before` datetime DEFAULT NULL,
  `certificate_expires_at` datetime DEFAULT NULL,
  `last_event_at` datetime DEFAULT NULL,
  `last_reconciled_at` datetime DEFAULT NULL,
  `last_error` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_domain_delivery_domain` (`domain_id`),
  KEY `idx_mochat_go_saas_domain_delivery_tenant` (`tenant_id`, `delivery_status`, `id`),
  KEY `idx_mochat_go_saas_domain_delivery_expiry` (`certificate_status`, `certificate_expires_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 自定义域名路由与 TLS 当前状态';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_tenant_domain_delivery_jobs` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `job_no` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `domain_id` bigint(20) unsigned NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL,
  `active_domain_id` bigint(20) unsigned DEFAULT NULL COMMENT '活动任务时等于 domain_id，保证单域名串行交付',
  `action` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'provision/refresh/disable/delete',
  `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending' COMMENT 'pending/processing/waiting/succeeded/failed/canceled',
  `attempts` int(10) unsigned NOT NULL DEFAULT '0',
  `max_attempts` int(10) unsigned NOT NULL DEFAULT '5',
  `next_attempt_at` datetime DEFAULT NULL,
  `lease_expires_at` datetime DEFAULT NULL,
  `provider` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `provider_request_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `request_payload_sha256` char(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `last_error` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `actor_user_id` int(10) unsigned NOT NULL DEFAULT '0',
  `actor_tenant_id` int(10) unsigned NOT NULL DEFAULT '0',
  `operation_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `started_at` datetime DEFAULT NULL,
  `finished_at` datetime DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_domain_delivery_job_no` (`job_no`),
  UNIQUE KEY `uni_mochat_go_saas_domain_delivery_active` (`active_domain_id`),
  KEY `idx_mochat_go_saas_domain_delivery_due` (`status`, `next_attempt_at`, `lease_expires_at`, `id`),
  KEY `idx_mochat_go_saas_domain_delivery_domain` (`domain_id`, `id`),
  KEY `idx_mochat_go_saas_domain_delivery_tenant_job` (`tenant_id`, `status`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 自定义域名路由与 TLS 交付任务';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_tenant_domain_delivery_events` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `event_id` varchar(128) COLLATE utf8mb4_bin NOT NULL,
  `job_no` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `domain_id` bigint(20) unsigned NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL,
  `payload_sha256` char(64) COLLATE utf8mb4_bin NOT NULL,
  `signature_timestamp` bigint(20) NOT NULL DEFAULT '0',
  `event_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'delivery.status',
  `result` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'applied' COMMENT 'applied/ignored',
  `occurred_at` datetime NOT NULL,
  `applied_at` datetime NOT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_domain_delivery_event` (`event_id`),
  KEY `idx_mochat_go_saas_domain_delivery_event_domain` (`domain_id`, `occurred_at`, `id`),
  KEY `idx_mochat_go_saas_domain_delivery_event_tenant` (`tenant_id`, `occurred_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 自定义域名交付签名回调幂等事件';

INSERT INTO `mochat_go_saas_tenant_domain_deliveries`
  (`domain_id`, `tenant_id`, `delivery_status`, `routing_status`, `certificate_status`, `version`, `created_at`, `updated_at`)
SELECT d.id, d.tenant_id,
  CASE WHEN d.deleted_at IS NOT NULL THEN 'deleted' WHEN d.status = 'disabled' THEN 'disabled' ELSE 'unconfigured' END,
  CASE WHEN d.deleted_at IS NOT NULL THEN 'deleted' WHEN d.status = 'disabled' THEN 'disabled' ELSE 'pending' END,
  CASE WHEN d.deleted_at IS NOT NULL THEN 'deleted' WHEN d.status = 'disabled' THEN 'disabled' ELSE 'pending' END,
  1, NOW(), NOW()
FROM `mochat_go_saas_tenant_domains` d
ON DUPLICATE KEY UPDATE `tenant_id` = VALUES(`tenant_id`), `updated_at` = NOW();
