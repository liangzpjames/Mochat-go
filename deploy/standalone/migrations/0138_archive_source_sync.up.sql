-- Archive source boundary: durable runs, auditable state transitions, and
-- explicit source identity for normalized message rows. This migration is
-- additive and does not alter the already-applied 0127-0137 migrations.
CREATE TABLE IF NOT EXISTS `mochat_go_archive_sync_runs` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint(20) unsigned NOT NULL,
  `corp_id` bigint(20) unsigned NOT NULL,
  `source_kind` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL,
  `source_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `namespace` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `idempotency_key` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL,
  `cursor_sequence` bigint(20) NOT NULL DEFAULT 0,
  `cursor_token` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `fetched_count` int(10) unsigned NOT NULL DEFAULT 0,
  `processed_count` int(10) unsigned NOT NULL DEFAULT 0,
  `skipped_count` int(10) unsigned NOT NULL DEFAULT 0,
  `failed_count` int(10) unsigned NOT NULL DEFAULT 0,
  `error_code` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `attempt` int(10) unsigned NOT NULL DEFAULT 1,
  `started_at` datetime(6) NULL,
  `finished_at` datetime(6) NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_archive_sync_run_idempotency` (`tenant_id`,`corp_id`,`source_kind`,`source_id`,`idempotency_key`),
  KEY `idx_archive_sync_run_scope_status` (`tenant_id`,`corp_id`,`status`,`updated_at`),
  KEY `idx_archive_sync_run_source` (`tenant_id`,`corp_id`,`source_kind`,`source_id`,`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tenant-scoped archive source synchronization runs';

CREATE TABLE IF NOT EXISTS `mochat_go_archive_sync_audits` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `run_id` bigint(20) unsigned NOT NULL,
  `tenant_id` bigint(20) unsigned NOT NULL,
  `corp_id` bigint(20) unsigned NOT NULL,
  `source_kind` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL,
  `source_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `namespace` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `action` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL,
  `error_code` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `cursor_sequence` bigint(20) NOT NULL DEFAULT 0,
  `fetched_count` int(10) unsigned NOT NULL DEFAULT 0,
  `processed_count` int(10) unsigned NOT NULL DEFAULT 0,
  `skipped_count` int(10) unsigned NOT NULL DEFAULT 0,
  `failed_count` int(10) unsigned NOT NULL DEFAULT 0,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  KEY `idx_archive_sync_audit_scope` (`tenant_id`,`corp_id`,`created_at`),
  KEY `idx_archive_sync_audit_run` (`run_id`,`created_at`),
  CONSTRAINT `fk_archive_sync_audit_run` FOREIGN KEY (`run_id`) REFERENCES `mochat_go_archive_sync_runs` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Auditable archive source synchronization transitions';

CREATE TABLE IF NOT EXISTS `mochat_go_archive_message_sources` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint(20) unsigned NOT NULL,
  `corp_id` bigint(20) unsigned NOT NULL,
  `msgid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `source_kind` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL,
  `source_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `namespace` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `run_id` bigint(20) unsigned NOT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_archive_message_source_scope_msg` (`tenant_id`,`corp_id`,`msgid`),
  KEY `idx_archive_message_source_filter` (`tenant_id`,`corp_id`,`source_kind`,`source_id`,`created_at`),
  KEY `idx_archive_message_source_run` (`run_id`),
  CONSTRAINT `fk_archive_message_source_run` FOREIGN KEY (`run_id`) REFERENCES `mochat_go_archive_sync_runs` (`id`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Explicit source identity for normalized archive messages';
