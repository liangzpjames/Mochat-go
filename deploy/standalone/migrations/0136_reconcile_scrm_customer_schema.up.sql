-- Reconcile SCRM customer lifecycle schema after 0107/0108 were ledgered
-- before the contacts table existed. This migration is additive and idempotent.

CREATE TABLE IF NOT EXISTS `mochat_go_scrm_idempotency_keys` (
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

CREATE TABLE IF NOT EXISTS `mochat_go_scrm_assignment_history` (
  `id` varchar(64) NOT NULL,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `contact_id` varchar(36) NOT NULL,
  `action` varchar(32) NOT NULL,
  `previous_owner_id` bigint unsigned NULL,
  `new_owner_id` bigint unsigned NULL,
  `actor_id` bigint unsigned NOT NULL,
  `reason` varchar(500) NOT NULL DEFAULT '',
  `assignment_version` bigint unsigned NOT NULL,
  `created_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_scrm_pool_history_contact` (`tenant_id`,`corp_id`,`contact_id`,`created_at`,`id`),
  KEY `idx_scrm_pool_history_filter` (`tenant_id`,`corp_id`,`action`,`previous_owner_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- reconcile: mochat_go_scrm_contacts.source
SET @scrm_customer_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mochat_go_scrm_contacts' AND column_name='source') = 0,
  'ALTER TABLE `mochat_go_scrm_contacts` ADD COLUMN `source` varchar(32) NOT NULL DEFAULT '''' AFTER `phone`',
  'SELECT 1'
);
PREPARE scrm_customer_reconcile FROM @scrm_customer_stmt;
EXECUTE scrm_customer_reconcile;
DEALLOCATE PREPARE scrm_customer_reconcile;

-- reconcile: mochat_go_scrm_contacts.business_type
SET @scrm_customer_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mochat_go_scrm_contacts' AND column_name='business_type') = 0,
  'ALTER TABLE `mochat_go_scrm_contacts` ADD COLUMN `business_type` varchar(64) NOT NULL DEFAULT '''' AFTER `source`',
  'SELECT 1'
);
PREPARE scrm_customer_reconcile FROM @scrm_customer_stmt;
EXECUTE scrm_customer_reconcile;
DEALLOCATE PREPARE scrm_customer_reconcile;

-- reconcile: mochat_go_scrm_contacts.region
SET @scrm_customer_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mochat_go_scrm_contacts' AND column_name='region') = 0,
  'ALTER TABLE `mochat_go_scrm_contacts` ADD COLUMN `region` varchar(128) NOT NULL DEFAULT '''' AFTER `business_type`',
  'SELECT 1'
);
PREPARE scrm_customer_reconcile FROM @scrm_customer_stmt;
EXECUTE scrm_customer_reconcile;
DEALLOCATE PREPARE scrm_customer_reconcile;

-- reconcile: mochat_go_scrm_contacts.idx_scrm_contacts_public_pool_filters
SET @scrm_customer_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='mochat_go_scrm_contacts' AND index_name='idx_scrm_contacts_public_pool_filters') = 0,
  'ALTER TABLE `mochat_go_scrm_contacts` ADD INDEX `idx_scrm_contacts_public_pool_filters` (`tenant_id`,`corp_id`,`source`,`business_type`,`region`,`deleted_at`)',
  'SELECT 1'
);
PREPARE scrm_customer_reconcile FROM @scrm_customer_stmt;
EXECUTE scrm_customer_reconcile;
DEALLOCATE PREPARE scrm_customer_reconcile;

-- reconcile: mochat_go_scrm_assignments.idx_scrm_assignments_public_pool
SET @scrm_customer_stmt = IF(
  (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='mochat_go_scrm_assignments' AND index_name='idx_scrm_assignments_public_pool') = 0,
  'ALTER TABLE `mochat_go_scrm_assignments` ADD INDEX `idx_scrm_assignments_public_pool` (`tenant_id`,`corp_id`,`status`,`owner_id`,`updated_at`,`id`)',
  'SELECT 1'
);
PREPARE scrm_customer_reconcile FROM @scrm_customer_stmt;
EXECUTE scrm_customer_reconcile;
DEALLOCATE PREPARE scrm_customer_reconcile;

UPDATE `mochat_go_scrm_contacts` c
INNER JOIN `mochat_go_scrm_leads` l
  ON l.`tenant_id`=c.`tenant_id`
 AND l.`corp_id`=c.`corp_id`
 AND l.`converted_contact_id`=c.`id`
SET c.`source`=l.`source`
WHERE c.`source`='';
