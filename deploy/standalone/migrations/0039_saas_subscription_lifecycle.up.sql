CREATE TABLE IF NOT EXISTS `mochat_go_saas_subscriptions` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '租户 ID',
  `package_code` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '当前套餐编码快照',
  `package_name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '当前套餐名称快照',
  `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active' COMMENT 'trialing/active/grace/past_due/suspended/canceled',
  `billing_cycle` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'custom' COMMENT 'monthly/yearly/custom/lifetime',
  `trial_starts_at` timestamp NULL DEFAULT NULL,
  `trial_ends_at` timestamp NULL DEFAULT NULL,
  `current_period_starts_at` timestamp NULL DEFAULT NULL,
  `current_period_ends_at` timestamp NULL DEFAULT NULL,
  `grace_ends_at` timestamp NULL DEFAULT NULL,
  `cancel_at_period_end` tinyint(1) NOT NULL DEFAULT '0',
  `canceled_at` timestamp NULL DEFAULT NULL,
  `suspended_at` timestamp NULL DEFAULT NULL,
  `latest_billing_event_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `version` int(10) unsigned NOT NULL DEFAULT '1' COMMENT '乐观锁版本',
  `state_reason` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `metadata_json` json DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_subscriptions_tenant` (`tenant_id`),
  KEY `idx_mochat_go_saas_subscriptions_status_period` (`status`, `current_period_ends_at`, `tenant_id`),
  KEY `idx_mochat_go_saas_subscriptions_status_grace` (`status`, `grace_ends_at`, `tenant_id`),
  KEY `idx_mochat_go_saas_subscriptions_package` (`package_code`, `status`, `tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 当前订阅状态表';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_subscription_events` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `subscription_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `tenant_id` int(10) unsigned NOT NULL DEFAULT '0',
  `event_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'transition',
  `from_status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `to_status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `effective_at` timestamp NULL DEFAULT NULL,
  `actor_user_id` int(10) unsigned NOT NULL DEFAULT '0',
  `actor_tenant_id` int(10) unsigned NOT NULL DEFAULT '0',
  `source` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'system',
  `idempotency_key` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `reason` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `payload_json` json DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_subscription_events_idempotency` (`tenant_id`, `idempotency_key`),
  KEY `idx_mochat_go_saas_subscription_events_tenant_time` (`tenant_id`, `created_at`),
  KEY `idx_mochat_go_saas_subscription_events_status_time` (`to_status`, `created_at`),
  KEY `idx_mochat_go_saas_subscription_events_subscription` (`subscription_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 订阅状态事件表';

INSERT INTO `mochat_go_saas_subscriptions`
  (`tenant_id`, `package_code`, `package_name`, `status`, `billing_cycle`, `current_period_starts_at`, `current_period_ends_at`, `grace_ends_at`, `suspended_at`, `version`, `state_reason`, `metadata_json`, `created_at`, `updated_at`, `deleted_at`)
SELECT
  p.`tenant_id`,
  p.`package_code`,
  p.`package_name`,
  CASE
    WHEN COALESCE(t.`status`, 1) = 2 OR p.`status` <> 1 THEN 'suspended'
    WHEN p.`expires_at` IS NULL OR p.`expires_at` >= NOW() THEN 'active'
    WHEN DATE_ADD(p.`expires_at`, INTERVAL 7 DAY) >= NOW() THEN 'grace'
    ELSE 'past_due'
  END,
  CASE WHEN p.`expires_at` IS NULL THEN 'lifetime' ELSE 'custom' END,
  COALESCE(p.`starts_at`, p.`created_at`, NOW()),
  p.`expires_at`,
  CASE WHEN p.`expires_at` IS NULL THEN NULL ELSE DATE_ADD(p.`expires_at`, INTERVAL 7 DAY) END,
  CASE WHEN COALESCE(t.`status`, 1) = 2 OR p.`status` <> 1 THEN NOW() ELSE NULL END,
  1,
  '0039 migration backfill',
  JSON_OBJECT('source', 'tenant_packages', 'defaultGraceDays', 7),
  COALESCE(p.`created_at`, NOW()),
  NOW(),
  NULL
FROM `mochat_go_saas_tenant_packages` p
LEFT JOIN `mc_tenant` t ON t.`id` = p.`tenant_id` AND t.`deleted_at` IS NULL
WHERE p.`deleted_at` IS NULL
ON DUPLICATE KEY UPDATE
  `package_code` = VALUES(`package_code`),
  `package_name` = VALUES(`package_name`),
  `updated_at` = NOW(),
  `deleted_at` = NULL;

INSERT INTO `mochat_go_saas_subscription_events`
  (`subscription_id`, `tenant_id`, `event_type`, `from_status`, `to_status`, `effective_at`, `actor_user_id`, `actor_tenant_id`, `source`, `idempotency_key`, `reason`, `payload_json`, `created_at`)
SELECT
  s.`id`,
  s.`tenant_id`,
  'backfill',
  '',
  s.`status`,
  NOW(),
  0,
  0,
  'migration',
  'migration:0039',
  '从租户套餐回填订阅生命周期',
  JSON_OBJECT('packageCode', s.`package_code`, 'periodEndsAt', s.`current_period_ends_at`),
  NOW()
FROM `mochat_go_saas_subscriptions` s
WHERE s.`deleted_at` IS NULL
ON DUPLICATE KEY UPDATE `subscription_id` = VALUES(`subscription_id`);
