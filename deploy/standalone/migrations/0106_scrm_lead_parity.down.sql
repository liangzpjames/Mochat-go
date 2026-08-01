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
