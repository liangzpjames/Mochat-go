ALTER TABLE `mc_corp`
  ADD COLUMN `wecom_credentials_ciphertext` text COLLATE utf8mb4_bin NULL COMMENT 'AES-256-GCM 加密后的企业微信企业凭据' AFTER `chat_secret`,
  ADD COLUMN `wecom_credentials_key_id` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '企业微信企业凭据加密密钥 ID' AFTER `wecom_credentials_ciphertext`,
  ADD KEY `idx_mc_corp_wecom_credential_key` (`wecom_credentials_key_id`, `tenant_id`, `id`);

ALTER TABLE `mc_work_agent`
  ADD COLUMN `wecom_credentials_ciphertext` text COLLATE utf8mb4_bin NULL COMMENT 'AES-256-GCM 加密后的企业微信应用凭据' AFTER `wx_secret`,
  ADD COLUMN `wecom_credentials_key_id` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '企业微信应用凭据加密密钥 ID' AFTER `wecom_credentials_ciphertext`,
  ADD KEY `idx_mc_work_agent_wecom_credential_key` (`wecom_credentials_key_id`, `corp_id`, `id`);
