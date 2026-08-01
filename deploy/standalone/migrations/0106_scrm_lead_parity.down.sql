SET @lead_rollback_conflict_count := (
  SELECT COUNT(*)
  FROM (
    SELECT `tenant_id`, `business_key`
    FROM `mochat_go_scrm_leads`
    GROUP BY `tenant_id`, `business_key`
    HAVING COUNT(DISTINCT `corp_id`) > 1
  ) cross_corp_lead_keys
);

SET @lead_rollback_guard_sql := IF(
  @lead_rollback_conflict_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0106 rollback blocked by cross-corp business_key conflict'''
);
PREPARE lead_rollback_guard_stmt FROM @lead_rollback_guard_sql;
EXECUTE lead_rollback_guard_stmt;
DEALLOCATE PREPARE lead_rollback_guard_stmt;

ALTER TABLE `mochat_go_scrm_leads`
  DROP INDEX `uk_scrm_leads_scope_business_key`,
  DROP INDEX `uk_scrm_leads_scope_phone`,
  DROP INDEX `idx_scrm_leads_combined_filter`,
  DROP COLUMN `discard_reason`,
  DROP COLUMN `converted_contact_id`,
  DROP COLUMN `owner_id`,
  DROP COLUMN `phone`,
  DROP COLUMN `corp_id`,
  ADD UNIQUE INDEX `uk_scrm_leads_tenant_business_key` (`tenant_id`,`business_key`),
  ADD INDEX `idx_scrm_leads_tenant_created_id` (`tenant_id`,`created_at`,`id`);
