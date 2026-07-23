ALTER TABLE `mochat_go_saas_service_account_keys`
  ADD COLUMN `hash_key_id` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT 'legacy-jwt' COMMENT 'API Key 摘要使用的 pepper 密钥 ID' AFTER `key_hash`,
  ADD KEY `idx_mochat_go_saas_service_account_key_hash_key` (`hash_key_id`, `status`, `id`);
