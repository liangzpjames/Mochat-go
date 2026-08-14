-- Archive source boundary: durable runs, auditable state transitions, and
-- explicit source identity for normalized message rows. This migration is
-- additive and does not alter the already-applied 0127-0137 migrations.
-- Each guard is immediately prepared and executed. A missing table is valid
-- for a fresh install; an existing same-named table must have the complete
-- column, unique-key and foreign-key signature or the migration fails closed.
SET @archive_runs_invalid := (
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs') = 1
  AND (
    (SELECT COUNT(DISTINCT column_name) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND column_name IN ('id','tenant_id','corp_id','source_kind','source_id','namespace','idempotency_key','status','cursor_sequence','cursor_token','fetched_count','processed_count','skipped_count','failed_count','error_code','attempt','started_at','finished_at','lease_expires_at','heartbeat_at','created_at','updated_at')) <> 22
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND ((column_name = 'id' AND column_type <> 'bigint(20) unsigned') OR (column_name IN ('tenant_id','corp_id') AND column_type <> 'int(10) unsigned') OR (column_name = 'source_kind' AND character_maximum_length <> 16) OR (column_name IN ('source_id','namespace') AND character_maximum_length <> 128) OR (column_name = 'status' AND character_maximum_length <> 16))) > 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND index_name = 'uk_archive_sync_run_idempotency' AND non_unique = 0), '') <> 'tenant_id,corp_id,source_kind,source_id,idempotency_key'
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND index_name = 'uk_archive_sync_run_scope_id' AND non_unique = 0), '') <> 'tenant_id,corp_id,id'
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND index_name = 'uk_archive_sync_run_identity' AND non_unique = 0), '') <> 'tenant_id,corp_id,id,source_kind,source_id,namespace'
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND constraint_name = 'fk_archive_sync_run_corp'), '') <> 'tenant_id=mc_corp.tenant_id,corp_id=mc_corp.id'
  )
);
SET @archive_runs_guard_sql := IF(@archive_runs_invalid = 0, 'SELECT 1', 'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0138 incompatible archive sync runs table''');
PREPARE archive_runs_guard_stmt FROM @archive_runs_guard_sql;
EXECUTE archive_runs_guard_stmt;
DEALLOCATE PREPARE archive_runs_guard_stmt;

SET @archive_audits_invalid := (
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits') = 1
  AND (
    (SELECT COUNT(DISTINCT column_name) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND column_name IN ('id','run_id','tenant_id','corp_id','source_kind','source_id','namespace','action','status','error_code','cursor_sequence','fetched_count','processed_count','skipped_count','failed_count','created_at')) <> 16
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND ((column_name IN ('id','run_id') AND column_type <> 'bigint(20) unsigned') OR (column_name IN ('tenant_id','corp_id') AND column_type <> 'int(10) unsigned') OR (column_name = 'source_kind' AND character_maximum_length <> 16) OR (column_name IN ('source_id','namespace') AND character_maximum_length <> 128) OR (column_name IN ('action','status') AND character_maximum_length <> 16))) > 0
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND constraint_name = 'fk_archive_sync_audit_run'), '') <> 'tenant_id=mochat_go_archive_sync_runs.tenant_id,corp_id=mochat_go_archive_sync_runs.corp_id,run_id=mochat_go_archive_sync_runs.id,source_kind=mochat_go_archive_sync_runs.source_kind,source_id=mochat_go_archive_sync_runs.source_id,namespace=mochat_go_archive_sync_runs.namespace'
  )
);
SET @archive_audits_guard_sql := IF(@archive_audits_invalid = 0, 'SELECT 1', 'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0138 incompatible archive sync audits table''');
PREPARE archive_audits_guard_stmt FROM @archive_audits_guard_sql;
EXECUTE archive_audits_guard_stmt;
DEALLOCATE PREPARE archive_audits_guard_stmt;

SET @archive_sources_invalid := (
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources') = 1
  AND (
    (SELECT COUNT(DISTINCT column_name) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND column_name IN ('id','tenant_id','corp_id','msgid','source_kind','source_id','namespace','run_id','created_at','updated_at')) <> 10
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND ((column_name IN ('id','run_id') AND column_type <> 'bigint(20) unsigned') OR (column_name IN ('tenant_id','corp_id') AND column_type <> 'int(10) unsigned') OR (column_name = 'source_kind' AND character_maximum_length <> 16) OR (column_name IN ('source_id','namespace') AND character_maximum_length <> 128))) > 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND index_name = 'uk_archive_message_source_scope_msg' AND non_unique = 0), '') <> 'tenant_id,corp_id,msgid'
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND constraint_name = 'fk_archive_message_source_run'), '') <> 'tenant_id=mochat_go_archive_sync_runs.tenant_id,corp_id=mochat_go_archive_sync_runs.corp_id,run_id=mochat_go_archive_sync_runs.id,source_kind=mochat_go_archive_sync_runs.source_kind,source_id=mochat_go_archive_sync_runs.source_id,namespace=mochat_go_archive_sync_runs.namespace'
  )
);
SET @archive_sources_guard_sql := IF(@archive_sources_invalid = 0, 'SELECT 1', 'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0138 incompatible archive message sources table''');
PREPARE archive_sources_guard_stmt FROM @archive_sources_guard_sql;
EXECUTE archive_sources_guard_stmt;
DEALLOCATE PREPARE archive_sources_guard_stmt;

CREATE TABLE IF NOT EXISTS `mochat_go_archive_sync_runs` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL,
  `corp_id` int(10) unsigned NOT NULL,
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
  `lease_expires_at` datetime(6) NULL,
  `heartbeat_at` datetime(6) NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_archive_sync_run_idempotency` (`tenant_id`,`corp_id`,`source_kind`,`source_id`,`idempotency_key`),
  UNIQUE KEY `uk_archive_sync_run_scope_id` (`tenant_id`,`corp_id`,`id`),
  UNIQUE KEY `uk_archive_sync_run_identity` (`tenant_id`,`corp_id`,`id`,`source_kind`,`source_id`,`namespace`),
  KEY `idx_archive_sync_run_scope_status` (`tenant_id`,`corp_id`,`status`,`updated_at`),
  KEY `idx_archive_sync_run_source` (`tenant_id`,`corp_id`,`source_kind`,`source_id`,`updated_at`),
  CONSTRAINT `fk_archive_sync_run_corp` FOREIGN KEY (`tenant_id`,`corp_id`) REFERENCES `mc_corp` (`tenant_id`,`id`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tenant-scoped archive source synchronization runs';

CREATE TABLE IF NOT EXISTS `mochat_go_archive_sync_audits` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `run_id` bigint(20) unsigned NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL,
  `corp_id` int(10) unsigned NOT NULL,
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
  CONSTRAINT `fk_archive_sync_audit_run` FOREIGN KEY (`tenant_id`,`corp_id`,`run_id`,`source_kind`,`source_id`,`namespace`)
    REFERENCES `mochat_go_archive_sync_runs` (`tenant_id`,`corp_id`,`id`,`source_kind`,`source_id`,`namespace`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Auditable archive source synchronization transitions';

CREATE TABLE IF NOT EXISTS `mochat_go_archive_message_sources` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL,
  `corp_id` int(10) unsigned NOT NULL,
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
  CONSTRAINT `fk_archive_message_source_run` FOREIGN KEY (`tenant_id`,`corp_id`,`run_id`,`source_kind`,`source_id`,`namespace`)
    REFERENCES `mochat_go_archive_sync_runs` (`tenant_id`,`corp_id`,`id`,`source_kind`,`source_id`,`namespace`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Explicit source identity for normalized archive messages';
