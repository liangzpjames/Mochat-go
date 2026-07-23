-- Extend tenant notification settings with delivery policy controls.
-- Defaults preserve the pre-migration behavior: all alert types, warning and above,
-- no quiet hours, and no hourly delivery limit.

ALTER TABLE `mochat_go_saas_alert_settings`
  ADD COLUMN `minimum_severity` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'warning' COMMENT '最低通知严重级别' AFTER `notification_retry_delay_seconds`,
  ADD COLUMN `allowed_alert_types_json` text COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '允许投递的告警类型 JSON，空表示全部' AFTER `minimum_severity`,
  ADD COLUMN `quiet_hours_enabled` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '是否启用免打扰时段' AFTER `allowed_alert_types_json`,
  ADD COLUMN `quiet_hours_start` char(5) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '22:00' COMMENT '免打扰开始 HH:MM' AFTER `quiet_hours_enabled`,
  ADD COLUMN `quiet_hours_end` char(5) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '08:00' COMMENT '免打扰结束 HH:MM' AFTER `quiet_hours_start`,
  ADD COLUMN `timezone` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'Asia/Shanghai' COMMENT '免打扰时区' AFTER `quiet_hours_end`,
  ADD COLUMN `hourly_limit` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '每小时最大业务通知数，0 为不限' AFTER `timezone`;

ALTER TABLE `mochat_go_saas_alert_notifications`
  ADD KEY `idx_mochat_go_saas_alert_notifications_delivery_window` (`tenant_id`, `channel`, `status`, `delivered_at`);
