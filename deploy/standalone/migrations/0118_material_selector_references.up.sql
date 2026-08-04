ALTER TABLE `mc_friends_circle_tasks`
  ADD COLUMN `medium_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER `content`,
  ADD KEY `idx_friends_circle_tasks_corp_medium` (`corp_id`, `medium_id`);

ALTER TABLE `mc_work_room_auto_pull`
  ADD COLUMN `medium_id` INT UNSIGNED NOT NULL DEFAULT 0 AFTER `corp_id`,
  ADD KEY `idx_work_room_auto_pull_corp_medium` (`corp_id`, `medium_id`);

ALTER TABLE `mc_contact_message_batch_send`
  ADD COLUMN `medium_id` INT UNSIGNED NOT NULL DEFAULT 0 AFTER `user_id`,
  ADD KEY `idx_contact_batch_send_corp_medium` (`corp_id`, `medium_id`);

ALTER TABLE `mc_room_message_batch_send`
  ADD COLUMN `medium_id` INT UNSIGNED NOT NULL DEFAULT 0 AFTER `user_id`,
  ADD KEY `idx_room_batch_send_corp_medium` (`corp_id`, `medium_id`);
