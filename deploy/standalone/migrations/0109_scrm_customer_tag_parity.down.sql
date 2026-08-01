ALTER TABLE `mochat_go_scrm_contact_tags`
  DROP INDEX IF EXISTS `idx_scrm_contact_tag_usage`;

ALTER TABLE `mochat_go_scrm_tags`
  DROP INDEX IF EXISTS `idx_scrm_tag_catalog`,
  DROP COLUMN IF EXISTS `group_id`;

DROP TABLE IF EXISTS `mochat_go_scrm_tag_groups`;
