DELETE FROM `mochat_go_saas_admin_approval_policies`
WHERE `action_type` = 'payment.order.create';

ALTER TABLE `mochat_go_saas_payment_orders`
  DROP COLUMN `package_limits_json`,
  DROP COLUMN `package_version`;
