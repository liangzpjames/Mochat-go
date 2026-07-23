DROP TABLE IF EXISTS `mochat_go_saas_invoice_documents`;
DROP TABLE IF EXISTS `mochat_go_saas_billing_profiles`;

ALTER TABLE `mochat_go_saas_payment_orders`
  DROP INDEX `idx_mochat_go_saas_payment_orders_invoice`,
  DROP COLUMN `latest_invoice_document_id`,
  DROP COLUMN `credited_amount_cents`,
  DROP COLUMN `credit_pending_amount_cents`,
  DROP COLUMN `invoiced_amount_cents`,
  DROP COLUMN `invoice_pending_amount_cents`;
