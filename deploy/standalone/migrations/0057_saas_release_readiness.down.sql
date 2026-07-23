DELETE rp
FROM `mochat_go_saas_admin_role_permissions` rp
INNER JOIN `mochat_go_saas_admin_roles` r ON r.id = rp.role_id
WHERE r.code IN ('platform_operations', 'platform_approver', 'platform_auditor', 'platform_readonly')
  AND rp.permission_code IN ('platform.release.read', 'platform.release.manage');

DROP TABLE IF EXISTS `mochat_go_saas_release_candidates`;
DROP TABLE IF EXISTS `mochat_go_saas_release_evidence`;
