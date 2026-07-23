ALTER TABLE `mochat_go_wechat_component_tickets`
  ADD COLUMN `component_verify_ticket_ciphertext` text COLLATE utf8mb4_bin NULL COMMENT 'AES-256-GCM 加密后的微信开放平台 component_verify_ticket' AFTER `component_verify_ticket`,
  ADD COLUMN `credential_key_id` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '微信开放平台 Ticket 加密密钥 ID' AFTER `component_verify_ticket_ciphertext`,
  ADD KEY `idx_mochat_go_wechat_component_ticket_credential_key` (`credential_key_id`, `component_appid`);

ALTER TABLE `mc_official_account`
  ADD COLUMN `wechat_credentials_ciphertext` text COLLATE utf8mb4_bin NULL COMMENT 'AES-256-GCM 加密后的微信开放平台与公众号授权凭据' AFTER `token`,
  ADD COLUMN `wechat_credentials_key_id` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '微信开放平台与公众号授权凭据加密密钥 ID' AFTER `wechat_credentials_ciphertext`,
  ADD KEY `idx_mc_official_account_wechat_credential_key` (`wechat_credentials_key_id`, `tenant_id`, `id`);
