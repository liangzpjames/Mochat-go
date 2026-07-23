DELETE rp
FROM `mochat_go_saas_admin_role_permissions` rp
INNER JOIN `mochat_go_saas_admin_roles` r ON r.id = rp.role_id
WHERE rp.permission_code IN ('platform.identity.read', 'platform.identity.manage')
  AND r.code IN ('platform_operations', 'platform_approver', 'platform_auditor', 'platform_readonly');

DROP TABLE IF EXISTS `mochat_go_saas_identity_security_incidents`;
DROP TABLE IF EXISTS `mochat_go_saas_identity_login_events`;
DROP TABLE IF EXISTS `mochat_go_saas_identity_sessions`;
DROP TABLE IF EXISTS `mochat_go_saas_identity_auth_challenges`;
DROP TABLE IF EXISTS `mochat_go_saas_identity_mfa_credentials`;
DROP TABLE IF EXISTS `mochat_go_saas_identity_user_states`;
DROP TABLE IF EXISTS `mochat_go_saas_identity_policies`;
