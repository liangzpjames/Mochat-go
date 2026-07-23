ALTER TABLE `mc_work_agent`
  DROP KEY `idx_mc_work_agent_wecom_credential_key`,
  DROP COLUMN `wecom_credentials_key_id`,
  DROP COLUMN `wecom_credentials_ciphertext`;

ALTER TABLE `mc_corp`
  DROP KEY `idx_mc_corp_wecom_credential_key`,
  DROP COLUMN `wecom_credentials_key_id`,
  DROP COLUMN `wecom_credentials_ciphertext`;
