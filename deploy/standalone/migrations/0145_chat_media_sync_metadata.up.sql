ALTER TABLE `mochat_go_audio_objects`
  ADD COLUMN `source` varchar(32) NOT NULL DEFAULT 'legacy_manual' AFTER `original_name`,
  ADD COLUMN `message_id` varchar(255) NOT NULL DEFAULT '' AFTER `source`,
  ADD COLUMN `sender_name` varchar(255) NOT NULL DEFAULT '' AFTER `message_id`,
  ADD COLUMN `receiver_name` varchar(255) NOT NULL DEFAULT '' AFTER `sender_name`,
  ADD COLUMN `synced_at` datetime NULL AFTER `updated_at`,
  ADD KEY `idx_audio_objects_sync_query` (`corp_id`, `source`, `synced_at`, `sender_name`, `receiver_name`, `id`);

UPDATE `mochat_go_dashboard_permission_resources` r
INNER JOIN `mochat_go_dashboard_permissions` p ON p.`id` = r.`permission_id`
SET r.`status` = 0, r.`deleted_at` = NOW()
WHERE p.`code` = 'dashboard.chat.file_audio'
  AND r.`http_method` IN ('POST', 'DELETE')
  AND r.`path_pattern` IN ('/dashboard/chat/media', '/dashboard/chat/media/{id}');
