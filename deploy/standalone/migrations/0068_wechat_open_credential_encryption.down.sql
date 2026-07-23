ALTER TABLE `mc_official_account`
  DROP KEY `idx_mc_official_account_wechat_credential_key`,
  DROP COLUMN `wechat_credentials_key_id`,
  DROP COLUMN `wechat_credentials_ciphertext`;

ALTER TABLE `mochat_go_wechat_component_tickets`
  DROP KEY `idx_mochat_go_wechat_component_ticket_credential_key`,
  DROP COLUMN `credential_key_id`,
  DROP COLUMN `component_verify_ticket_ciphertext`;
