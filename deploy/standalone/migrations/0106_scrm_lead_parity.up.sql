SET @lead_scope_error_count := (
  SELECT COUNT(*)
  FROM (
    SELECT l.`tenant_id`
    FROM (SELECT DISTINCT `tenant_id` FROM `mochat_go_scrm_leads`) l
    LEFT JOIN `mc_corp` c
      ON c.`tenant_id` = l.`tenant_id`
     AND c.`deleted_at` IS NULL
    GROUP BY l.`tenant_id`
    HAVING COUNT(c.`id`) <> 1
  ) ambiguous_lead_tenants
);

SET @lead_scope_guard_sql := IF(
  @lead_scope_error_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0106 cannot uniquely map historical leads to an active corp'''
);
PREPARE lead_scope_guard_stmt FROM @lead_scope_guard_sql;
EXECUTE lead_scope_guard_stmt;
DEALLOCATE PREPARE lead_scope_guard_stmt;

ALTER TABLE `mochat_go_scrm_leads`
  DROP INDEX `uk_scrm_leads_tenant_business_key`,
  DROP INDEX `idx_scrm_leads_tenant_created_id`,
  ADD COLUMN `corp_id` bigint unsigned NULL AFTER `tenant_id`,
  ADD COLUMN `phone` varchar(64) NULL AFTER `name`,
  ADD COLUMN `owner_id` bigint unsigned NULL AFTER `status`,
  ADD COLUMN `converted_contact_id` varchar(36) NOT NULL DEFAULT '' AFTER `owner_id`,
  ADD COLUMN `discard_reason` varchar(500) NOT NULL DEFAULT '' AFTER `converted_contact_id`;

UPDATE `mochat_go_scrm_leads` l
INNER JOIN (
  SELECT `tenant_id`, MIN(`id`) AS `corp_id`
  FROM `mc_corp`
  WHERE `deleted_at` IS NULL
  GROUP BY `tenant_id`
  HAVING COUNT(*) = 1
) c ON c.`tenant_id` = l.`tenant_id`
SET l.`corp_id` = c.`corp_id`;

ALTER TABLE `mochat_go_scrm_leads`
  MODIFY COLUMN `corp_id` bigint unsigned NOT NULL,
  ADD UNIQUE INDEX `uk_scrm_leads_scope_business_key` (`tenant_id`,`corp_id`,`business_key`),
  ADD UNIQUE INDEX `uk_scrm_leads_scope_phone` (`tenant_id`,`corp_id`,`phone`),
  ADD INDEX `idx_scrm_leads_combined_filter` (`tenant_id`,`corp_id`,`status`,`source`,`owner_id`,`created_at`,`id`);
