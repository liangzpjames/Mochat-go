CREATE TABLE `mochat_go_scrm_idempotency_keys` (
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `action` varchar(64) NOT NULL,
  `idempotency_key` varchar(128) NOT NULL,
  `request_fingerprint` char(64) NOT NULL,
  `resource_id` varchar(64) NOT NULL,
  `created_at` datetime(6) NOT NULL,
  PRIMARY KEY (`tenant_id`,`corp_id`,`action`,`idempotency_key`),
  KEY `idx_scrm_idempotency_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
