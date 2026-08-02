DROP TABLE IF EXISTS `mochat_go_scrm_assignment_history`;

ALTER TABLE `mochat_go_scrm_assignments`
  DROP INDEX `idx_scrm_assignments_public_pool`;

ALTER TABLE `mochat_go_scrm_contacts`
  DROP INDEX `idx_scrm_contacts_public_pool_filters`,
  DROP COLUMN `region`,
  DROP COLUMN `business_type`,
  DROP COLUMN `source`;
