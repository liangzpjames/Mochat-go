DELETE FROM `mochat_go_saas_admin_approval_policies`
WHERE `action_type` = 'compliance.export.delete';

ALTER TABLE `mochat_go_saas_data_exports`
  DROP KEY `idx_mochat_go_saas_data_export_deletion`,
  DROP KEY `uni_mochat_go_saas_data_export_deletion_approval`,
  DROP COLUMN `deletion_actor_tenant_id`,
  DROP COLUMN `deletion_requested_by`,
  DROP COLUMN `deletion_last_error`,
  DROP COLUMN `deletion_finished_at`,
  DROP COLUMN `deletion_started_at`,
  DROP COLUMN `deletion_requested_at`,
  DROP COLUMN `deletion_lease_expires_at`,
  DROP COLUMN `deletion_approval_id`,
  DROP COLUMN `deletion_attempts`,
  DROP COLUMN `deletion_record_status`,
  DROP COLUMN `deletion_artifact_status`,
  DROP COLUMN `deletion_status`;
