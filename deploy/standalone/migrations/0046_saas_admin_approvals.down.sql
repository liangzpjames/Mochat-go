DELETE rp
FROM `mochat_go_saas_admin_role_permissions` rp
INNER JOIN `mochat_go_saas_admin_roles` r ON r.id = rp.role_id
WHERE rp.permission_code IN ('platform.approvals.read', 'platform.approvals.review', 'platform.approvals.execute');

DELETE FROM `mochat_go_saas_admin_roles` WHERE `code` = 'platform_approver' AND `is_system` = 1;

DROP TABLE IF EXISTS `mochat_go_saas_admin_approval_events`;
DROP TABLE IF EXISTS `mochat_go_saas_admin_approvals`;
