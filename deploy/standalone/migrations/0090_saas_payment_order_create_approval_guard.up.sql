ALTER TABLE `mochat_go_saas_payment_orders`
  ADD COLUMN `package_version` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '创建订单时冻结的套餐定义版本，0 表示旧订单' AFTER `package_name`,
  ADD COLUMN `package_limits_json` json DEFAULT NULL COMMENT '创建订单时冻结的完整套餐额度' AFTER `package_version`;

INSERT IGNORE INTO `mochat_go_saas_admin_approval_policies`
  (`action_type`, `enabled`, `amount_threshold_cents`, `required_approvals`, `sla_minutes`, `reminder_minutes`, `expiry_hours`, `version`, `updated_by`, `created_at`, `updated_at`)
VALUES
  ('payment.order.create', 1, 0, 2, 120, 30, 12, 1, 0, NOW(), NOW());

UPDATE `mochat_go_saas_admin_approval_policies`
SET `enabled` = 1,
    `amount_threshold_cents` = 0,
    `required_approvals` = GREATEST(`required_approvals`, 2),
    `version` = `version` + 1,
    `updated_by` = 0,
    `updated_at` = NOW()
WHERE `action_type` = 'payment.order.create'
  AND (`enabled` <> 1 OR `amount_threshold_cents` <> 0 OR `required_approvals` < 2);
