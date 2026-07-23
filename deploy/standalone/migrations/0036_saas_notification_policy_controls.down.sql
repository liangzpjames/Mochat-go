ALTER TABLE `mochat_go_saas_alert_notifications`
  DROP INDEX `idx_mochat_go_saas_alert_notifications_delivery_window`;

ALTER TABLE `mochat_go_saas_alert_settings`
  DROP COLUMN `hourly_limit`,
  DROP COLUMN `timezone`,
  DROP COLUMN `quiet_hours_end`,
  DROP COLUMN `quiet_hours_start`,
  DROP COLUMN `quiet_hours_enabled`,
  DROP COLUMN `allowed_alert_types_json`,
  DROP COLUMN `minimum_severity`;
