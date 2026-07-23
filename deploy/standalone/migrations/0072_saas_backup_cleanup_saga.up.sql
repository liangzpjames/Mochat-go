ALTER TABLE `mochat_go_saas_backup_runs`
  ADD COLUMN `cleanup_run_id` bigint(20) unsigned DEFAULT NULL
    COMMENT '已冻结的保留清理任务 ID；非空时禁止校验、复制和恢复' AFTER `operation_id`,
  ADD KEY `idx_mochat_go_saas_backup_cleanup` (`cleanup_run_id`, `id`);

CREATE TABLE IF NOT EXISTS `mochat_go_saas_backup_cleanup_runs` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `cleanup_no` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending'
    COMMENT 'pending/running/succeeded/partial/failed',
  `active_slot` tinyint(3) unsigned DEFAULT NULL COMMENT '待执行或运行中固定为 1，唯一约束阻止并发清理',
  `policy_version` int(10) unsigned NOT NULL,
  `cutoff_at` datetime NOT NULL,
  `scanned_count` int(10) unsigned NOT NULL DEFAULT '0',
  `candidate_count` int(10) unsigned NOT NULL DEFAULT '0',
  `preserved_count` int(10) unsigned NOT NULL DEFAULT '0',
  `deleted_count` int(10) unsigned NOT NULL DEFAULT '0',
  `failed_count` int(10) unsigned NOT NULL DEFAULT '0',
  `replicas_deleted_count` int(10) unsigned NOT NULL DEFAULT '0',
  `missing_files_count` int(10) unsigned NOT NULL DEFAULT '0',
  `attempts` int(10) unsigned NOT NULL DEFAULT '0',
  `approval_id` bigint(20) unsigned DEFAULT NULL,
  `lease_expires_at` datetime DEFAULT NULL,
  `started_at` datetime DEFAULT NULL,
  `finished_at` datetime DEFAULT NULL,
  `last_error` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_by` int(10) unsigned NOT NULL DEFAULT '0',
  `actor_tenant_id` int(10) unsigned NOT NULL DEFAULT '0',
  `operation_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_backup_cleanup_no` (`cleanup_no`),
  UNIQUE KEY `uni_mochat_go_saas_backup_cleanup_active` (`active_slot`),
  UNIQUE KEY `uni_mochat_go_saas_backup_cleanup_approval` (`approval_id`),
  KEY `idx_mochat_go_saas_backup_cleanup_status` (`status`, `lease_expires_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 备份保留清理任务';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_backup_cleanup_items` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `cleanup_run_id` bigint(20) unsigned NOT NULL,
  `backup_run_id` bigint(20) unsigned NOT NULL,
  `backup_run_version` int(10) unsigned NOT NULL,
  `backup_no` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `artifact_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `replica_object_key` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `replica_version_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending'
    COMMENT 'pending/running/succeeded/failed',
  `replica_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending'
    COMMENT 'pending/not_required/deleted/missing/failed',
  `local_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending'
    COMMENT 'pending/not_required/deleted/missing/failed',
  `record_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending'
    COMMENT 'pending/deleted/failed',
  `attempts` int(10) unsigned NOT NULL DEFAULT '0',
  `last_error` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `replica_deleted_at` datetime DEFAULT NULL,
  `local_deleted_at` datetime DEFAULT NULL,
  `record_deleted_at` datetime DEFAULT NULL,
  `operation_id` bigint(20) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_backup_cleanup_item` (`cleanup_run_id`, `backup_run_id`),
  KEY `idx_mochat_go_saas_backup_cleanup_item_status` (`cleanup_run_id`, `status`, `id`),
  KEY `idx_mochat_go_saas_backup_cleanup_item_backup` (`backup_run_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 备份保留清理步骤';

INSERT IGNORE INTO `mochat_go_saas_admin_approval_policies`
  (`action_type`, `enabled`, `amount_threshold_cents`, `required_approvals`, `sla_minutes`, `reminder_minutes`, `expiry_hours`, `version`, `updated_by`, `created_at`, `updated_at`)
VALUES
  ('backup.retention.cleanup', 1, 0, 2, 120, 30, 12, 1, 0, NOW(), NOW());
