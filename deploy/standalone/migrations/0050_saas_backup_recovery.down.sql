DELETE rp
FROM `mochat_go_saas_admin_role_permissions` rp
INNER JOIN `mochat_go_saas_admin_roles` r ON r.id = rp.role_id
WHERE rp.permission_code IN ('platform.backups.read', 'platform.backups.manage')
  AND r.code IN ('platform_operations', 'platform_auditor', 'platform_readonly');

DROP TABLE IF EXISTS `mochat_go_saas_restore_drills`;
DROP TABLE IF EXISTS `mochat_go_saas_backup_runs`;
DROP TABLE IF EXISTS `mochat_go_saas_backup_policies`;
