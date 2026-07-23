UPDATE `mochat_go_saas_admin_approval_policies`
SET `required_approvals` = 1,
    `version` = `version` + 1,
    `updated_by` = 0,
    `updated_at` = NOW()
WHERE `action_type` = 'payment.refund.create'
  AND `enabled` = 1
  AND `amount_threshold_cents` = 0
  AND `required_approvals` = 2
  AND `version` = 2
  AND `updated_by` = 0;
