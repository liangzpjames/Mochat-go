ALTER TABLE `mochat_go_saas_service_account_keys`
  DROP KEY `idx_mochat_go_saas_service_account_key_hash_key`,
  DROP COLUMN `hash_key_id`;
