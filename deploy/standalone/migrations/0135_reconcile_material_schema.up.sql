-- Reconcile material columns after historical 0115 ledger/DDL drift.
-- Every statement is additive and guarded for MySQL 5.7 and MariaDB.

-- reconcile: mc_medium.scope_type
SET @material_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mc_medium' AND column_name='scope_type') = 0,
  'ALTER TABLE `mc_medium` ADD COLUMN `scope_type` varchar(16) NOT NULL DEFAULT ''public'' COMMENT ''material scope public department personal'' AFTER `user_name`',
  'SELECT 1'
);
PREPARE material_schema_reconcile FROM @material_schema_stmt;
EXECUTE material_schema_reconcile;
DEALLOCATE PREPARE material_schema_reconcile;

-- reconcile: mc_medium.scope_id
SET @material_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mc_medium' AND column_name='scope_id') = 0,
  'ALTER TABLE `mc_medium` ADD COLUMN `scope_id` int(10) unsigned NOT NULL DEFAULT 0 COMMENT ''department or personal user id'' AFTER `scope_type`',
  'SELECT 1'
);
PREPARE material_schema_reconcile FROM @material_schema_stmt;
EXECUTE material_schema_reconcile;
DEALLOCATE PREPARE material_schema_reconcile;

-- reconcile: mc_medium.sidebar_visible
SET @material_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mc_medium' AND column_name='sidebar_visible') = 0,
  'ALTER TABLE `mc_medium` ADD COLUMN `sidebar_visible` tinyint(1) NOT NULL DEFAULT 1 COMMENT ''visible in chat sidebar'' AFTER `scope_id`',
  'SELECT 1'
);
PREPARE material_schema_reconcile FROM @material_schema_stmt;
EXECUTE material_schema_reconcile;
DEALLOCATE PREPARE material_schema_reconcile;

-- reconcile: mc_medium.status
SET @material_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mc_medium' AND column_name='status') = 0,
  'ALTER TABLE `mc_medium` ADD COLUMN `status` varchar(16) NOT NULL DEFAULT ''available'' COMMENT ''processing available failed disabled'' AFTER `sidebar_visible`',
  'SELECT 1'
);
PREPARE material_schema_reconcile FROM @material_schema_stmt;
EXECUTE material_schema_reconcile;
DEALLOCATE PREPARE material_schema_reconcile;

-- reconcile: mc_medium.idx_medium_scope
SET @material_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='mc_medium' AND index_name='idx_medium_scope') = 0,
  'ALTER TABLE `mc_medium` ADD KEY `idx_medium_scope` (`corp_id`, `scope_type`, `scope_id`, `deleted_at`)',
  'SELECT 1'
);
PREPARE material_schema_reconcile FROM @material_schema_stmt;
EXECUTE material_schema_reconcile;
DEALLOCATE PREPARE material_schema_reconcile;

-- reconcile: mc_medium.idx_medium_selector
SET @material_schema_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='mc_medium' AND index_name='idx_medium_selector') = 0,
  'ALTER TABLE `mc_medium` ADD KEY `idx_medium_selector` (`corp_id`, `status`, `sidebar_visible`, `deleted_at`)',
  'SELECT 1'
);
PREPARE material_schema_reconcile FROM @material_schema_stmt;
EXECUTE material_schema_reconcile;
DEALLOCATE PREPARE material_schema_reconcile;
