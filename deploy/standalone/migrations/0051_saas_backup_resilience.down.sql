ALTER TABLE `mochat_go_saas_restore_drills`
  DROP COLUMN `target_cleanup_error`,
  DROP COLUMN `target_cleaned_at`,
  DROP COLUMN `target_cleanup_status`,
  DROP COLUMN `target_lifecycle`;

ALTER TABLE `mochat_go_saas_backup_runs`
  DROP INDEX `idx_mochat_go_saas_backup_replica`,
  DROP COLUMN `replica_error`,
  DROP COLUMN `replica_verified_at`,
  DROP COLUMN `replicated_at`,
  DROP COLUMN `replica_size_bytes`,
  DROP COLUMN `replica_sha256`,
  DROP COLUMN `replica_version_id`,
  DROP COLUMN `replica_etag`,
  DROP COLUMN `replica_object_key`,
  DROP COLUMN `replica_bucket`,
  DROP COLUMN `replica_provider`,
  DROP COLUMN `replica_status`;

ALTER TABLE `mochat_go_saas_backup_policies`
  DROP COLUMN `require_offsite_replica`;
