DELETE rp
FROM `mochat_go_saas_admin_role_permissions` rp
INNER JOIN `mochat_go_saas_admin_roles` r ON r.id = rp.role_id
WHERE rp.permission_code IN ('platform.domains.read', 'platform.domains.manage')
  AND r.code IN ('platform_operations', 'platform_approver', 'platform_auditor', 'platform_readonly');

DROP TABLE IF EXISTS `mochat_go_saas_tenant_domains`;
