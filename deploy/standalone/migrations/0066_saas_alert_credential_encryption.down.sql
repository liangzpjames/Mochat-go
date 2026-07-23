ALTER TABLE `mochat_go_saas_alert_settings`
  DROP KEY `idx_mochat_go_saas_alert_settings_credential_key`,
  DROP COLUMN `webhook_credentials_key_id`,
  DROP COLUMN `webhook_credentials_ciphertext`;
