ALTER TABLE `mochat_go_tenant_corp_bindings`
  ADD COLUMN `wecom_integration_mode` enum('self_built','third_party_delegated')
    COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'self_built'
    AFTER `status`,
  ADD COLUMN IF NOT EXISTS `employee_credential_generation` bigint unsigned NOT NULL DEFAULT 1 AFTER `version`,
  ADD COLUMN IF NOT EXISTS `contact_credential_generation` bigint unsigned NOT NULL DEFAULT 1 AFTER `employee_credential_generation`,
  ADD COLUMN IF NOT EXISTS `agent_credential_generation` bigint unsigned NOT NULL DEFAULT 1 AFTER `contact_credential_generation`,
  ADD COLUMN IF NOT EXISTS `callback_credential_generation` bigint unsigned NOT NULL DEFAULT 1 AFTER `agent_credential_generation`;

UPDATE `mochat_go_tenant_corp_bindings` binding
INNER JOIN (
  SELECT `tenant_id`, `corp_id`, `mode`
  FROM `mochat_go_wecom_integrations`
  WHERE `slot` = 'current'
) integration
  ON integration.`tenant_id` = binding.`tenant_id`
 AND integration.`corp_id` = binding.`corp_id`
SET binding.`wecom_integration_mode` = integration.`mode`;

INSERT INTO `mochat_go_wecom_integrations`
  (`id`,`tenant_id`,`corp_id`,`mode`,`slot`,`status`,`scope_json`,`scope_digest`,`missing_capabilities_json`,`generation`,`version`,`last_audit_at`)
SELECT UUID(), binding.`tenant_id`, binding.`corp_id`, binding.`wecom_integration_mode`, 'current', 'unconfigured',
       JSON_ARRAY(), SHA2('', 256), JSON_ARRAY(), 1, 1, NOW(6)
FROM `mochat_go_tenant_corp_bindings` binding
WHERE NOT EXISTS (
  SELECT 1
  FROM `mochat_go_wecom_integrations` existing
  WHERE existing.`tenant_id` = binding.`tenant_id`
    AND existing.`corp_id` = binding.`corp_id`
    AND existing.`slot` = 'current'
);
