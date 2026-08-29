ALTER TABLE `mochat_go_scrm_orders`
  MODIFY COLUMN `idempotency_key` varbinary(128) NOT NULL;

CREATE TABLE IF NOT EXISTS `mochat_go_scrm_order_idempotency_receipts` (
  `tenant_id` bigint(20) NOT NULL,
  `corp_id` bigint(20) NOT NULL,
  `idempotency_key` varbinary(128) NOT NULL,
  `request_hash` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `order_id` varchar(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `response_status` smallint(5) unsigned NOT NULL,
  `response_body` mediumtext COLLATE utf8mb4_bin NOT NULL,
  `created_at` datetime(6) NOT NULL,
  PRIMARY KEY (`tenant_id`,`corp_id`,`idempotency_key`),
  KEY `idx_scrm_order_receipt_order` (`tenant_id`,`corp_id`,`order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Committed HTTP receipts for idempotent SCRM order creation';
