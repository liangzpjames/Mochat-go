ALTER TABLE `mochat_go_saas_service_accounts`
  ADD COLUMN `rate_limit_per_minute` int(10) unsigned NOT NULL DEFAULT '60' COMMENT 'OpenAPI 每分钟成功请求上限' AFTER `allowed_cidrs_json`,
  ADD COLUMN `daily_request_limit` int(10) unsigned NOT NULL DEFAULT '10000' COMMENT 'OpenAPI 每日成功请求上限，0 表示不限' AFTER `rate_limit_per_minute`,
  ADD COLUMN `minute_window_started_at` datetime DEFAULT NULL COMMENT '当前分钟窗口' AFTER `daily_request_limit`,
  ADD COLUMN `minute_request_count` int(10) unsigned NOT NULL DEFAULT '0' AFTER `minute_window_started_at`,
  ADD COLUMN `minute_rejected_count` int(10) unsigned NOT NULL DEFAULT '0' AFTER `minute_request_count`,
  ADD COLUMN `daily_window_date` date DEFAULT NULL COMMENT '当前自然日窗口' AFTER `minute_rejected_count`,
  ADD COLUMN `daily_request_count` bigint(20) unsigned NOT NULL DEFAULT '0' AFTER `daily_window_date`,
  ADD COLUMN `daily_rejected_count` bigint(20) unsigned NOT NULL DEFAULT '0' AFTER `daily_request_count`,
  ADD KEY `idx_mochat_go_saas_service_account_rate_window` (`status`, `daily_window_date`, `id`);

CREATE TABLE IF NOT EXISTS `mochat_go_saas_service_account_usage_daily` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `service_account_id` bigint(20) unsigned NOT NULL,
  `usage_date` date NOT NULL,
  `route_key` varchar(128) COLLATE utf8mb4_bin NOT NULL COMMENT '规范化 METHOD path',
  `request_count` bigint(20) unsigned NOT NULL DEFAULT '0',
  `rejected_count` bigint(20) unsigned NOT NULL DEFAULT '0',
  `last_used_at` datetime DEFAULT NULL,
  `last_used_ip` varchar(45) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_service_account_usage_daily` (`service_account_id`, `usage_date`, `route_key`),
  KEY `idx_mochat_go_saas_service_account_usage_date` (`usage_date`, `service_account_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版服务账号 OpenAPI 日用量';
