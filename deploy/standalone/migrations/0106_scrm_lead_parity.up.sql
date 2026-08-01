ALTER TABLE `mochat_go_scrm_leads`
  DROP INDEX `uk_scrm_leads_tenant_business_key`,
  DROP INDEX `idx_scrm_leads_tenant_created_id`,
  ADD COLUMN `corp_id` bigint unsigned NOT NULL DEFAULT 0 AFTER `tenant_id`,
  ADD COLUMN `phone` varchar(64) NULL AFTER `name`,
  ADD COLUMN `owner_id` bigint unsigned NULL AFTER `status`,
  ADD COLUMN `converted_contact_id` varchar(36) NOT NULL DEFAULT '' AFTER `owner_id`,
  ADD COLUMN `discard_reason` varchar(500) NOT NULL DEFAULT '' AFTER `converted_contact_id`,
  ADD UNIQUE INDEX `uk_scrm_leads_scope_business_key` (`tenant_id`,`corp_id`,`business_key`),
  ADD UNIQUE INDEX `uk_scrm_leads_scope_phone` (`tenant_id`,`corp_id`,`phone`),
  ADD INDEX `idx_scrm_leads_combined_filter` (`tenant_id`,`corp_id`,`status`,`source`,`owner_id`,`created_at`,`id`);
