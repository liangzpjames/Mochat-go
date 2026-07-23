ALTER TABLE `mochat_go_saas_data_exports`
  ADD COLUMN `deletion_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT ''
    COMMENT 'pending/running/succeeded/failed；空值表示未进入删除流程' AFTER `operation_id`,
  ADD COLUMN `deletion_artifact_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT ''
    COMMENT 'pending/deleted/missing/failed' AFTER `deletion_status`,
  ADD COLUMN `deletion_record_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT ''
    COMMENT 'pending/deleted/failed' AFTER `deletion_artifact_status`,
  ADD COLUMN `deletion_attempts` int(10) unsigned NOT NULL DEFAULT '0' AFTER `deletion_record_status`,
  ADD COLUMN `deletion_approval_id` bigint(20) unsigned DEFAULT NULL AFTER `deletion_attempts`,
  ADD COLUMN `deletion_lease_expires_at` datetime DEFAULT NULL AFTER `deletion_approval_id`,
  ADD COLUMN `deletion_requested_at` datetime DEFAULT NULL AFTER `deletion_lease_expires_at`,
  ADD COLUMN `deletion_started_at` datetime DEFAULT NULL AFTER `deletion_requested_at`,
  ADD COLUMN `deletion_finished_at` datetime DEFAULT NULL AFTER `deletion_started_at`,
  ADD COLUMN `deletion_last_error` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' AFTER `deletion_finished_at`,
  ADD COLUMN `deletion_requested_by` int(10) unsigned NOT NULL DEFAULT '0' AFTER `deletion_last_error`,
  ADD COLUMN `deletion_actor_tenant_id` int(10) unsigned NOT NULL DEFAULT '0' AFTER `deletion_requested_by`,
  ADD UNIQUE KEY `uni_mochat_go_saas_data_export_deletion_approval` (`deletion_approval_id`),
  ADD KEY `idx_mochat_go_saas_data_export_deletion` (`deletion_status`, `deletion_lease_expires_at`, `id`);

INSERT IGNORE INTO `mochat_go_saas_admin_approval_policies`
  (`action_type`, `enabled`, `amount_threshold_cents`, `required_approvals`, `sla_minutes`, `reminder_minutes`, `expiry_hours`, `version`, `updated_by`, `created_at`, `updated_at`)
VALUES
  ('compliance.export.delete', 1, 0, 2, 120, 30, 12, 1, 0, NOW(), NOW());
