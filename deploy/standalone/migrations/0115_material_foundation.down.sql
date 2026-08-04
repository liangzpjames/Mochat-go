DELETE FROM `mc_rbac_menu` WHERE `id` IN (115001, 115002, 115003, 115004);

ALTER TABLE `mc_medium`
  DROP KEY `idx_medium_selector`,
  DROP KEY `idx_medium_scope`,
  DROP COLUMN `status`,
  DROP COLUMN `sidebar_visible`,
  DROP COLUMN `scope_id`,
  DROP COLUMN `scope_type`;
