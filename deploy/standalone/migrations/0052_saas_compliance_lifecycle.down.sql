DELETE FROM `mochat_go_saas_admin_approval_policies`
WHERE `action_type` = 'tenant.data.erase';

DELETE rp
FROM `mochat_go_saas_admin_role_permissions` rp
INNER JOIN `mochat_go_saas_admin_roles` r ON r.id = rp.role_id
WHERE rp.permission_code IN ('platform.compliance.read', 'platform.compliance.manage')
  AND r.code IN ('platform_operations', 'platform_approver', 'platform_auditor', 'platform_readonly');

DROP TABLE IF EXISTS `mochat_go_saas_tenant_tombstones`;
DROP TABLE IF EXISTS `mochat_go_saas_erasure_steps`;
DROP TABLE IF EXISTS `mochat_go_saas_erasure_requests`;
DROP TABLE IF EXISTS `mochat_go_saas_data_exports`;
DROP TABLE IF EXISTS `mochat_go_saas_legal_holds`;
DROP TABLE IF EXISTS `mochat_go_saas_compliance_policies`;
