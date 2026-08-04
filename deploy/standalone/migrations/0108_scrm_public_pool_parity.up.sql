SET @public_pool_source_exists := IF(
  (
    SELECT COUNT(*)
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'mochat_go_scrm_contacts'
      AND column_name = 'source'
  ) = 1,
  1,
  0
);
SET @public_pool_source_sql := IF(
  @public_pool_source_exists = 1,
  'SELECT 1',
  'ALTER TABLE `mochat_go_scrm_contacts`
    ADD COLUMN `source` varchar(32) NOT NULL DEFAULT '''' AFTER `phone`'
);
PREPARE public_pool_source_stmt FROM @public_pool_source_sql;
EXECUTE public_pool_source_stmt;
DEALLOCATE PREPARE public_pool_source_stmt;

SET @public_pool_business_type_exists := IF(
  (
    SELECT COUNT(*)
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'mochat_go_scrm_contacts'
      AND column_name = 'business_type'
  ) = 1,
  1,
  0
);
SET @public_pool_business_type_sql := IF(
  @public_pool_business_type_exists = 1,
  'SELECT 1',
  'ALTER TABLE `mochat_go_scrm_contacts`
    ADD COLUMN `business_type` varchar(64) NOT NULL DEFAULT '''' AFTER `source`'
);
PREPARE public_pool_business_type_stmt FROM @public_pool_business_type_sql;
EXECUTE public_pool_business_type_stmt;
DEALLOCATE PREPARE public_pool_business_type_stmt;

SET @public_pool_region_exists := IF(
  (
    SELECT COUNT(*)
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'mochat_go_scrm_contacts'
      AND column_name = 'region'
  ) = 1,
  1,
  0
);
SET @public_pool_region_sql := IF(
  @public_pool_region_exists = 1,
  'SELECT 1',
  'ALTER TABLE `mochat_go_scrm_contacts`
    ADD COLUMN `region` varchar(128) NOT NULL DEFAULT '''' AFTER `business_type`'
);
PREPARE public_pool_region_stmt FROM @public_pool_region_sql;
EXECUTE public_pool_region_stmt;
DEALLOCATE PREPARE public_pool_region_stmt;

UPDATE `mochat_go_scrm_contacts` c
INNER JOIN `mochat_go_scrm_leads` l
  ON l.`tenant_id`=c.`tenant_id`
 AND l.`corp_id`=c.`corp_id`
 AND l.`converted_contact_id`=c.`id`
SET c.`source`=l.`source`
WHERE c.`source`='';

SET @public_pool_contacts_index_exists := IF(
  (
    SELECT COUNT(*)
    FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'mochat_go_scrm_contacts'
      AND index_name = 'idx_scrm_contacts_public_pool_filters'
  ) > 0,
  1,
  0
);
SET @public_pool_contacts_index_sql := IF(
  @public_pool_contacts_index_exists = 1,
  'SELECT 1',
  'ALTER TABLE `mochat_go_scrm_contacts`
    ADD INDEX `idx_scrm_contacts_public_pool_filters` (`tenant_id`,`corp_id`,`source`,`business_type`,`region`,`deleted_at`)'
);
PREPARE public_pool_contacts_index_stmt FROM @public_pool_contacts_index_sql;
EXECUTE public_pool_contacts_index_stmt;
DEALLOCATE PREPARE public_pool_contacts_index_stmt;

SET @public_pool_assignments_index_exists := IF(
  (
    SELECT COUNT(*)
    FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'mochat_go_scrm_assignments'
      AND index_name = 'idx_scrm_assignments_public_pool'
  ) > 0,
  1,
  0
);
SET @public_pool_assignments_index_sql := IF(
  @public_pool_assignments_index_exists = 1,
  'SELECT 1',
  'ALTER TABLE `mochat_go_scrm_assignments`
    ADD INDEX `idx_scrm_assignments_public_pool` (`tenant_id`,`corp_id`,`status`,`owner_id`,`updated_at`,`id`)'
);
PREPARE public_pool_assignments_index_stmt FROM @public_pool_assignments_index_sql;
EXECUTE public_pool_assignments_index_stmt;
DEALLOCATE PREPARE public_pool_assignments_index_stmt;

SET @public_pool_history_exists := IF(
  (
    SELECT COUNT(*)
    FROM information_schema.tables
    WHERE table_schema = DATABASE()
      AND table_name = 'mochat_go_scrm_assignment_history'
  ) = 1
  AND (
    SELECT COUNT(*)
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'mochat_go_scrm_assignment_history'
      AND column_name IN ('id', 'tenant_id', 'corp_id', 'contact_id', 'action', 'previous_owner_id', 'new_owner_id', 'actor_id', 'reason', 'assignment_version', 'created_at')
  ) = 11,
  1,
  0
);
SET @public_pool_history_sql := IF(
  @public_pool_history_exists = 1,
  'SELECT 1',
  'CREATE TABLE `mochat_go_scrm_assignment_history` (
    `id` varchar(64) NOT NULL,
    `tenant_id` bigint unsigned NOT NULL,
    `corp_id` bigint unsigned NOT NULL,
    `contact_id` varchar(36) NOT NULL,
    `action` varchar(32) NOT NULL,
    `previous_owner_id` bigint unsigned NULL,
    `new_owner_id` bigint unsigned NULL,
    `actor_id` bigint unsigned NOT NULL,
    `reason` varchar(500) NOT NULL DEFAULT '''',
    `assignment_version` bigint unsigned NOT NULL,
    `created_at` datetime(6) NOT NULL,
    PRIMARY KEY (`id`)
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci'
);
PREPARE public_pool_history_stmt FROM @public_pool_history_sql;
EXECUTE public_pool_history_stmt;
DEALLOCATE PREPARE public_pool_history_stmt;

SET @public_pool_history_contact_index_exists := IF(
  (
    SELECT COUNT(*)
    FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'mochat_go_scrm_assignment_history'
      AND index_name = 'idx_scrm_pool_history_contact'
  ) > 0,
  1,
  0
);
SET @public_pool_history_contact_index_sql := IF(
  @public_pool_history_contact_index_exists = 1,
  'SELECT 1',
  'ALTER TABLE `mochat_go_scrm_assignment_history`
    ADD INDEX `idx_scrm_pool_history_contact` (`tenant_id`,`corp_id`,`contact_id`,`created_at`,`id`)'
);
PREPARE public_pool_history_contact_index_stmt FROM @public_pool_history_contact_index_sql;
EXECUTE public_pool_history_contact_index_stmt;
DEALLOCATE PREPARE public_pool_history_contact_index_stmt;

SET @public_pool_history_filter_index_exists := IF(
  (
    SELECT COUNT(*)
    FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'mochat_go_scrm_assignment_history'
      AND index_name = 'idx_scrm_pool_history_filter'
  ) > 0,
  1,
  0
);
SET @public_pool_history_filter_index_sql := IF(
  @public_pool_history_filter_index_exists = 1,
  'SELECT 1',
  'ALTER TABLE `mochat_go_scrm_assignment_history`
    ADD INDEX `idx_scrm_pool_history_filter` (`tenant_id`,`corp_id`,`action`,`previous_owner_id`,`created_at`)'
);
PREPARE public_pool_history_filter_index_stmt FROM @public_pool_history_filter_index_sql;
EXECUTE public_pool_history_filter_index_stmt;
DEALLOCATE PREPARE public_pool_history_filter_index_stmt;
