SET @idempotency_already_applied := IF(
  (
    SELECT COUNT(*)
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'mochat_go_scrm_idempotency_keys'
      AND column_name IN ('tenant_id', 'corp_id', 'action', 'idempotency_key', 'request_fingerprint', 'resource_id', 'created_at')
  ) = 7
  AND (
    SELECT COUNT(DISTINCT index_name)
    FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'mochat_go_scrm_idempotency_keys'
      AND index_name IN ('PRIMARY', 'idx_scrm_idempotency_created')
  ) = 2,
  1,
  0
);

SET @idempotency_sql := IF(
  @idempotency_already_applied = 1,
  'SELECT 1',
  'CREATE TABLE `mochat_go_scrm_idempotency_keys` (
    `tenant_id` bigint unsigned NOT NULL,
    `corp_id` bigint unsigned NOT NULL,
    `action` varchar(64) NOT NULL,
    `idempotency_key` varchar(128) NOT NULL,
    `request_fingerprint` char(64) NOT NULL,
    `resource_id` varchar(64) NOT NULL,
    `created_at` datetime(6) NOT NULL,
    PRIMARY KEY (`tenant_id`,`corp_id`,`action`,`idempotency_key`),
    KEY `idx_scrm_idempotency_created` (`created_at`)
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci'
);
PREPARE idempotency_stmt FROM @idempotency_sql;
EXECUTE idempotency_stmt;
DEALLOCATE PREPARE idempotency_stmt;
