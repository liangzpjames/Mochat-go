ALTER TABLE `mochat_go_scrm_contact_tags`
  DROP INDEX IF EXISTS `idx_scrm_contact_tag_usage`;

ALTER TABLE `mochat_go_scrm_tags`
  DROP INDEX IF EXISTS `uk_scrm_tag_active_group_name`,
  DROP INDEX IF EXISTS `idx_scrm_tag_catalog`;

ALTER TABLE `mochat_go_scrm_tags`
  DROP COLUMN IF EXISTS `active_group_name`;

ALTER TABLE `mochat_go_scrm_tags`
  DROP COLUMN IF EXISTS `group_id`;

ALTER TABLE `mochat_go_scrm_tag_groups`
  DROP INDEX IF EXISTS `uk_scrm_tag_group_active_name`,
  DROP COLUMN IF EXISTS `active_name`;

DROP TABLE IF EXISTS `mochat_go_scrm_tag_groups`;
