-- Local-acceptance-only parent normalization for the official 0139 migration.
-- The standalone baseline already has tenant ownership and composite indexes;
-- it predates only the NOT NULL declaration required by 0139's fail-closed
-- preflight. Never ledger this helper as a production migration.
SET @local_0139_null_corp_tenants := (SELECT COUNT(*) FROM mc_corp WHERE tenant_id IS NULL);
SET @local_0139_guard_sql := IF(
  @local_0139_null_corp_tenants = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''local 0139 compatibility found unowned corp rows'''
);
PREPARE local_0139_guard_stmt FROM @local_0139_guard_sql;
EXECUTE local_0139_guard_stmt;
DEALLOCATE PREPARE local_0139_guard_stmt;

ALTER TABLE mc_corp MODIFY COLUMN tenant_id INT UNSIGNED NOT NULL DEFAULT 0;
