ALTER TABLE `mochat_go_saas_backup_policies`
  ADD COLUMN `require_offsite_replica` tinyint(3) unsigned NOT NULL DEFAULT '0'
    COMMENT '成功备份是否必须完成异地对象存储副本' AFTER `require_encryption`;

ALTER TABLE `mochat_go_saas_backup_runs`
  ADD COLUMN `replica_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'disabled'
    COMMENT 'disabled/pending/uploading/succeeded/failed/deleted' AFTER `size_bytes`,
  ADD COLUMN `replica_provider` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' AFTER `replica_status`,
  ADD COLUMN `replica_bucket` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' AFTER `replica_provider`,
  ADD COLUMN `replica_object_key` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' AFTER `replica_bucket`,
  ADD COLUMN `replica_etag` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' AFTER `replica_object_key`,
  ADD COLUMN `replica_version_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' AFTER `replica_etag`,
  ADD COLUMN `replica_sha256` char(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' AFTER `replica_version_id`,
  ADD COLUMN `replica_size_bytes` bigint(20) unsigned NOT NULL DEFAULT '0' AFTER `replica_sha256`,
  ADD COLUMN `replicated_at` datetime DEFAULT NULL AFTER `replica_size_bytes`,
  ADD COLUMN `replica_verified_at` datetime DEFAULT NULL AFTER `replicated_at`,
  ADD COLUMN `replica_error` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' AFTER `replica_verified_at`,
  ADD KEY `idx_mochat_go_saas_backup_replica` (`replica_status`, `replicated_at`, `id`);

ALTER TABLE `mochat_go_saas_restore_drills`
  ADD COLUMN `target_lifecycle` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'preconfigured'
    COMMENT 'preconfigured 或 ephemeral' AFTER `target_database`,
  ADD COLUMN `target_cleanup_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'not_required'
    COMMENT 'not_required/pending/succeeded/failed/retained' AFTER `target_lifecycle`,
  ADD COLUMN `target_cleaned_at` datetime DEFAULT NULL AFTER `target_cleanup_status`,
  ADD COLUMN `target_cleanup_error` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' AFTER `target_cleaned_at`;
