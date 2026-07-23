UPDATE `mochat_go_saas_admin_approval_policies`
SET `enabled` = 1,
    `amount_threshold_cents` = 0,
    `required_approvals` = GREATEST(`required_approvals`, 2),
    `version` = `version` + 1,
    `updated_by` = 0,
    `updated_at` = NOW()
WHERE `action_type` = 'payment.settlement.close'
  AND (`enabled` <> 1 OR `amount_threshold_cents` <> 0 OR `required_approvals` < 2);
