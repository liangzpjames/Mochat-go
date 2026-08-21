UPDATE `mochat_go_dashboard_permission_resources` r
INNER JOIN `mochat_go_dashboard_permissions` p ON p.`id` = r.`permission_id`
SET r.`status` = 1, r.`deleted_at` = NULL
WHERE p.`code` = 'dashboard.chat.file_audio'
  AND r.`http_method` IN ('POST', 'DELETE')
  AND r.`path_pattern` IN ('/dashboard/chat/media', '/dashboard/chat/media/{id}');

ALTER TABLE `mochat_go_audio_objects`
  DROP KEY `idx_audio_objects_sync_query`,
  DROP COLUMN `synced_at`,
  DROP COLUMN `receiver_name`,
  DROP COLUMN `sender_name`,
  DROP COLUMN `message_id`,
  DROP COLUMN `source`;
