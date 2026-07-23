DELETE FROM `mc_rbac_menu`
WHERE `id` IN (75, 76, 88, 89, 90, 91, 92, 93, 139, 140, 141, 142, 299, 302, 303, 304, 305, 306, 307, 308, 309, 337, 451, 452, 453, 454, 455, 456, 457, 458, 493);

ALTER TABLE `mc_corp`
  DROP COLUMN `chat_rsa_key`,
  DROP COLUMN `chat_whitelist_ip`,
  DROP COLUMN `service_contact_url`,
  DROP COLUMN `chat_secret`,
  DROP COLUMN `chat_status`,
  DROP COLUMN `chat_apply_status`,
  DROP COLUMN `chat_admin_idcard`,
  DROP COLUMN `chat_admin_phone`,
  DROP COLUMN `chat_admin`;

DROP TABLE IF EXISTS `mc_work_message_10`;
DROP TABLE IF EXISTS `mc_work_message_9`;
DROP TABLE IF EXISTS `mc_work_message_8`;
DROP TABLE IF EXISTS `mc_work_message_7`;
DROP TABLE IF EXISTS `mc_work_message_6`;
DROP TABLE IF EXISTS `mc_work_message_5`;
DROP TABLE IF EXISTS `mc_work_message_4`;
DROP TABLE IF EXISTS `mc_work_message_3`;
DROP TABLE IF EXISTS `mc_work_message_2`;
DROP TABLE IF EXISTS `mc_work_message_1`;
DROP TABLE IF EXISTS `mc_work_message_id`;
DROP TABLE IF EXISTS `mc_auto_tag_record`;
DROP TABLE IF EXISTS `mc_auto_tag`;
