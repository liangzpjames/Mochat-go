CREATE TABLE IF NOT EXISTS `mochat_go_scrm_tag_groups` (
  `id` varchar(36) NOT NULL,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `name` varchar(100) NOT NULL,
  `version` bigint unsigned NOT NULL DEFAULT 1,
  `deleted_at` datetime(6) NULL,
  `created_at` datetime(6) NOT NULL,
  `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_scrm_tag_group_scope` (`tenant_id`,`corp_id`,`name`,`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE `mochat_go_scrm_tags`
  ADD COLUMN IF NOT EXISTS `group_id` varchar(36) NULL AFTER `corp_id`;

INSERT INTO `mochat_go_scrm_tag_groups` (`id`,`tenant_id`,`corp_id`,`name`,`version`,`created_at`,`updated_at`)
SELECT CONCAT('g-', LEFT(MD5(CONCAT(`tenant_id`, ':', `corp_id`)), 32)), `tenant_id`, `corp_id`, '默认分组', 1, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)
FROM `mochat_go_scrm_tags`
GROUP BY `tenant_id`,`corp_id`
ON DUPLICATE KEY UPDATE `updated_at`=`updated_at`;

UPDATE `mochat_go_scrm_tags` AS t
JOIN `mochat_go_scrm_tag_groups` AS g
  ON g.`tenant_id`=t.`tenant_id` AND g.`corp_id`=t.`corp_id` AND g.`name`='默认分组' AND g.`deleted_at` IS NULL
SET t.`group_id`=g.`id`
WHERE t.`group_id` IS NULL;

ALTER TABLE `mochat_go_scrm_tags`
  ADD INDEX IF NOT EXISTS `idx_scrm_tag_catalog` (`tenant_id`,`corp_id`,`group_id`,`name`,`deleted_at`);

ALTER TABLE `mochat_go_scrm_contact_tags`
  ADD INDEX IF NOT EXISTS `idx_scrm_contact_tag_usage` (`tenant_id`,`corp_id`,`tag_id`,`contact_id`);
