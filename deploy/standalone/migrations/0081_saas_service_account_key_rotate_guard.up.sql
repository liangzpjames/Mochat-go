INSERT IGNORE INTO `mochat_go_saas_admin_approval_policies`
  (`action_type`, `enabled`, `amount_threshold_cents`, `required_approvals`, `sla_minutes`, `reminder_minutes`, `expiry_hours`, `version`, `updated_by`, `created_at`, `updated_at`)
VALUES
  ('service_account.key.rotate', 1, 0, 2, 120, 30, 12, 1, 0, NOW(), NOW());

UPDATE `mochat_go_saas_admin_approval_policies`
SET `enabled` = 1,
    `amount_threshold_cents` = 0,
    `required_approvals` = GREATEST(`required_approvals`, 2),
    `version` = `version` + 1,
    `updated_by` = 0,
    `updated_at` = NOW()
WHERE `action_type` = 'service_account.key.rotate'
  AND (`enabled` <> 1 OR `amount_threshold_cents` <> 0 OR `required_approvals` < 2);
