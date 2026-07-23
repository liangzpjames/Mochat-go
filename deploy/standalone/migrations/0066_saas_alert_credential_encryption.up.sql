ALTER TABLE `mochat_go_saas_alert_settings`
  ADD COLUMN `webhook_credentials_ciphertext` text COLLATE utf8mb4_bin NULL COMMENT 'AES-256-GCM 加密后的 Webhook URL 与签名密钥' AFTER `webhook_secret`,
  ADD COLUMN `webhook_credentials_key_id` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT 'Webhook 凭据加密密钥 ID' AFTER `webhook_credentials_ciphertext`,
  ADD KEY `idx_mochat_go_saas_alert_settings_credential_key` (`webhook_credentials_key_id`, `tenant_id`, `id`);
