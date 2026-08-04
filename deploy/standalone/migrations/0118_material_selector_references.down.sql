ALTER TABLE `mc_friends_circle_tasks`
  DROP KEY `idx_friends_circle_tasks_corp_medium`,
  DROP COLUMN `medium_id`;

ALTER TABLE `mc_work_room_auto_pull`
  DROP KEY `idx_work_room_auto_pull_corp_medium`,
  DROP COLUMN `medium_id`;

ALTER TABLE `mc_contact_message_batch_send`
  DROP KEY `idx_contact_batch_send_corp_medium`,
  DROP COLUMN `medium_id`;

ALTER TABLE `mc_room_message_batch_send`
  DROP KEY `idx_room_batch_send_corp_medium`,
  DROP COLUMN `medium_id`;
