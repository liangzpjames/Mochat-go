DELETE FROM `mc_rbac_role_menu` WHERE `menu_id` IN (117001, 117002, 117003);
DELETE FROM `mc_rbac_menu` WHERE `id` IN (117001, 117002, 117003);
DROP TABLE IF EXISTS `mc_friends_circle_task_results`;
ALTER TABLE `mc_friends_circle_tasks`
  DROP COLUMN `publish_attempts`,
  DROP COLUMN `last_callback_at`;
