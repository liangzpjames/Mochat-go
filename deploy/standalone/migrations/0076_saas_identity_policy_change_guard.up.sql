INSERT IGNORE INTO `mochat_go_saas_admin_approval_policies`
  (`action_type`, `enabled`, `amount_threshold_cents`, `required_approvals`, `sla_minutes`, `reminder_minutes`, `expiry_hours`, `version`, `updated_by`, `created_at`, `updated_at`)
VALUES
  ('identity.policy.update', 1, 0, 2, 120, 30, 12, 1, 0, NOW(), NOW());
