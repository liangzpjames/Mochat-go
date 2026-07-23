ALTER TABLE `mochat_go_saas_admin_tasks`
  ADD COLUMN `version` int(10) unsigned NOT NULL DEFAULT '1' COMMENT '运营任务乐观锁版本' AFTER `status`;

INSERT IGNORE INTO `mochat_go_saas_admin_approval_policies`
  (`action_type`, `enabled`, `amount_threshold_cents`, `required_approvals`, `sla_minutes`, `reminder_minutes`, `expiry_hours`, `version`, `updated_by`, `created_at`, `updated_at`)
VALUES
  ('tenant.provision', 1, 0, 2, 120, 30, 12, 1, 0, NOW(), NOW());

UPDATE `mochat_go_saas_admin_approval_policies`
SET `enabled` = 1,
    `amount_threshold_cents` = 0,
    `required_approvals` = GREATEST(`required_approvals`, 2),
    `version` = `version` + 1,
    `updated_by` = 0,
    `updated_at` = NOW()
WHERE `action_type` = 'tenant.provision'
  AND (`enabled` <> 1 OR `amount_threshold_cents` <> 0 OR `required_approvals` < 2);
