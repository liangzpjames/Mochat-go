DELETE rp
FROM `mochat_go_saas_admin_role_permissions` rp
WHERE rp.permission_code = 'platform.approvals.manage';

ALTER TABLE `mochat_go_saas_admin_approvals`
  DROP INDEX `idx_mochat_go_saas_admin_approval_reminder`,
  DROP INDEX `idx_mochat_go_saas_admin_approval_sla`,
  DROP COLUMN `reminder_count`,
  DROP COLUMN `last_reminded_at`,
  DROP COLUMN `next_reminder_at`,
  DROP COLUMN `sla_due_at`,
  DROP COLUMN `reminder_minutes`,
  DROP COLUMN `approval_count`,
  DROP COLUMN `required_approvals`,
  DROP COLUMN `policy_version`;

DROP TABLE IF EXISTS `mochat_go_saas_admin_approval_delegations`;
DROP TABLE IF EXISTS `mochat_go_saas_admin_approval_decisions`;
DROP TABLE IF EXISTS `mochat_go_saas_admin_approval_policies`;
