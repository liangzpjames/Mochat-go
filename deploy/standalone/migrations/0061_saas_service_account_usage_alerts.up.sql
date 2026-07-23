ALTER TABLE `mochat_go_saas_service_accounts`
  ADD COLUMN `usage_alert_enabled` tinyint(1) unsigned NOT NULL DEFAULT '1' COMMENT '是否启用 OpenAPI 用量预警' AFTER `daily_request_limit`,
  ADD COLUMN `usage_warning_percent` int(10) unsigned NOT NULL DEFAULT '80' COMMENT '每日成功请求达到限额百分比时预警' AFTER `usage_alert_enabled`,
  ADD COLUMN `rejection_warning_count` int(10) unsigned NOT NULL DEFAULT '1' COMMENT '每日限流拒绝达到该值时预警，0 表示关闭' AFTER `usage_warning_percent`,
  ADD COLUMN `usage_alert_cooldown_minutes` int(10) unsigned NOT NULL DEFAULT '60' COMMENT '同类预警重复通知冷却分钟' AFTER `rejection_warning_count`,
  ADD COLUMN `usage_alert_last_evaluated_at` datetime DEFAULT NULL COMMENT '最近一次用量预警评估时间' AFTER `usage_alert_cooldown_minutes`,
  ADD COLUMN `usage_alert_last_notified_at` datetime DEFAULT NULL COMMENT '最近一次日用量预警通知时间' AFTER `usage_alert_last_evaluated_at`,
  ADD COLUMN `rejection_alert_last_notified_at` datetime DEFAULT NULL COMMENT '最近一次限流拒绝预警通知时间' AFTER `usage_alert_last_notified_at`,
  ADD KEY `idx_mochat_go_saas_service_account_usage_alert_eval` (`usage_alert_last_evaluated_at`, `id`);
