DELETE rp
FROM `mochat_go_saas_admin_role_permissions` rp
INNER JOIN `mochat_go_saas_admin_roles` r ON r.id = rp.role_id
WHERE rp.permission_code IN ('platform.integrations.read', 'platform.integrations.manage')
  AND r.code IN ('platform_operations', 'platform_auditor', 'platform_readonly');

DROP TABLE IF EXISTS `mochat_go_saas_service_account_keys`;
DROP TABLE IF EXISTS `mochat_go_saas_service_accounts`;
