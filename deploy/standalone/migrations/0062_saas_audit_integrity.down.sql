DELETE FROM `mochat_go_saas_admin_role_permissions`
WHERE `permission_code` = 'platform.audit.manage';

DROP TABLE IF EXISTS `mochat_go_saas_admin_audit_verifications`;
DROP TABLE IF EXISTS `mochat_go_saas_admin_audit_chains`;

ALTER TABLE `mochat_go_saas_admin_operation_logs`
  DROP KEY `idx_mochat_go_saas_admin_ops_integrity`,
  DROP COLUMN `integrity_version`,
  DROP COLUMN `integrity_hash`,
  DROP COLUMN `integrity_prev_hash`;
