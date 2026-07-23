-- Persistent SaaS alert notification outbox for standalone Go runtime.
-- This table keeps webhook delivery state outside process memory.

CREATE TABLE IF NOT EXISTS `mochat_go_saas_alert_notifications` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `notification_key` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '通知聚合键',
  `alert_key` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '关联告警聚合键',
  `tenant_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '租户 ID',
  `channel` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'webhook' COMMENT '通知通道',
  `status` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending' COMMENT '状态：pending/failed/delivered/dead',
  `attempts` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '调度尝试次数',
  `max_attempts` int(10) unsigned NOT NULL DEFAULT '3' COMMENT '最大调度尝试次数',
  `alert_json` json DEFAULT NULL COMMENT '通知时使用的告警快照',
  `last_error` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '最近失败原因',
  `next_retry_at` datetime DEFAULT NULL COMMENT '下次重试时间',
  `delivered_at` datetime DEFAULT NULL COMMENT '送达时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_alert_notifications_key` (`notification_key`),
  KEY `idx_mochat_go_saas_alert_notifications_status_retry` (`status`, `next_retry_at`),
  KEY `idx_mochat_go_saas_alert_notifications_tenant_status` (`tenant_id`, `status`, `updated_at`),
  KEY `idx_mochat_go_saas_alert_notifications_alert` (`alert_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 告警通知 outbox';
