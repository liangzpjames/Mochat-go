DROP TABLE IF EXISTS `mochat_go_saas_service_account_usage_daily`;

ALTER TABLE `mochat_go_saas_service_accounts`
  DROP KEY `idx_mochat_go_saas_service_account_rate_window`,
  DROP COLUMN `daily_rejected_count`,
  DROP COLUMN `daily_request_count`,
  DROP COLUMN `daily_window_date`,
  DROP COLUMN `minute_rejected_count`,
  DROP COLUMN `minute_request_count`,
  DROP COLUMN `minute_window_started_at`,
  DROP COLUMN `daily_request_limit`,
  DROP COLUMN `rate_limit_per_minute`;
