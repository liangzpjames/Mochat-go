UPDATE `mochat_go_saas_admin_approval_policies`
SET `required_approvals` = IF(`required_approvals` = 2, 1, `required_approvals`),
    `version` = `version` + 1,
    `updated_by` = 0,
    `updated_at` = NOW()
WHERE `action_type` = 'tenant.disable';
