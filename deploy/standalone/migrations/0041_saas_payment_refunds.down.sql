DROP TABLE IF EXISTS `mochat_go_saas_payment_refunds`;

ALTER TABLE `mochat_go_saas_payment_webhook_events`
  DROP INDEX `idx_mochat_go_saas_payment_webhook_refund`,
  DROP COLUMN `provider_refund_no`,
  DROP COLUMN `refund_no`,
  DROP COLUMN `refund_id`;

ALTER TABLE `mochat_go_saas_payment_orders`
  DROP INDEX `idx_mochat_go_saas_payment_orders_refund`,
  DROP COLUMN `latest_refund_id`,
  DROP COLUMN `refunded_amount_cents`,
  DROP COLUMN `refund_pending_amount_cents`;
