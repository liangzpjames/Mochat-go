DELETE FROM `mochat_go_saas_admin_approval_policies`
WHERE `action_type` = 'tenant.package.update';

ALTER TABLE `mochat_go_saas_tenant_packages`
  DROP COLUMN `version`;
