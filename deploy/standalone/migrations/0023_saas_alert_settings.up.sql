-- Tenant-scoped SaaS alert notification settings for standalone Go runtime.
-- The table stores webhook delivery policy per tenant and keeps secrets out of environment-only config.

CREATE TABLE IF NOT EXISTS `mochat_go_saas_alert_settings` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '租户 ID',
  `channel` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'webhook' COMMENT '通知通道',
  `enabled` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '是否启用',
  `webhook_url` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'Webhook URL',
  `webhook_secret` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'Webhook HMAC 签名密钥',
  `webhook_timeout_seconds` int(10) unsigned NOT NULL DEFAULT '5' COMMENT 'Webhook 请求超时秒数',
  `webhook_retry_attempts` int(10) unsigned NOT NULL DEFAULT '1' COMMENT 'Webhook 单次投递 HTTP 重试次数',
  `webhook_retry_delay_ms` int(10) unsigned NOT NULL DEFAULT '250' COMMENT 'Webhook 单次投递 HTTP 重试间隔毫秒',
  `webhook_title_template` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'SaaS额度告警：租户 {{.TenantID}} {{.Metric}}' COMMENT 'Webhook 标题模板',
  `webhook_body_template` text COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'Webhook 正文模板',
  `notification_max_attempts` int(10) unsigned NOT NULL DEFAULT '3' COMMENT 'Outbox 最大调度次数',
  `notification_retry_delay_seconds` int(10) unsigned NOT NULL DEFAULT '300' COMMENT 'Outbox 调度失败后重试间隔秒数',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_alert_settings_tenant_channel` (`tenant_id`, `channel`),
  KEY `idx_mochat_go_saas_alert_settings_channel_enabled` (`channel`, `enabled`, `updated_at`),
  KEY `idx_mochat_go_saas_alert_settings_tenant_enabled` (`tenant_id`, `enabled`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 告警通知配置';

