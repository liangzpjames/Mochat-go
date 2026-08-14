-- Archive source boundary: durable runs, auditable state transitions, and
-- explicit source identity for normalized message rows. This migration is
-- additive and does not alter the already-applied 0127-0137 migrations.
-- Each guard is immediately prepared and executed. A missing table is valid
-- for a fresh install; an existing same-named table must have the complete
-- column, unique-key and foreign-key signature or the migration fails closed.
SET @archive_runs_invalid := (
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs') = 1
  AND (
    (SELECT COUNT(DISTINCT column_name) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND column_name IN ('id','tenant_id','corp_id','source_kind','source_id','namespace','idempotency_key','status','cursor_sequence','cursor_token','fetched_count','processed_count','skipped_count','failed_count','error_code','attempt','lease_token','started_at','finished_at','lease_expires_at','heartbeat_at','created_at','updated_at')) <> 23
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND ((column_name = 'id' AND NOT (data_type = 'bigint' AND numeric_precision = 20 AND LOWER(column_type) LIKE '%unsigned')) OR (column_name IN ('tenant_id','corp_id') AND NOT (data_type = 'int' AND numeric_precision = 10 AND LOWER(column_type) LIKE '%unsigned')) OR (column_name = 'source_kind' AND character_maximum_length <> 16) OR (column_name IN ('source_id','namespace') AND character_maximum_length <> 128) OR (column_name IN ('idempotency_key','lease_token') AND character_maximum_length <> 128) OR (column_name = 'status' AND character_maximum_length <> 16))) > 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND (
      (column_name = 'id' AND data_type = 'bigint' AND numeric_precision = 20 AND LOWER(column_type) LIKE '%unsigned' AND is_nullable = 'NO' AND column_default IS NULL AND LOWER(extra) LIKE '%auto_increment%')
      OR (column_name IN ('tenant_id','corp_id') AND data_type = 'int' AND numeric_precision = 10 AND LOWER(column_type) LIKE '%unsigned' AND is_nullable = 'NO' AND column_default IS NULL AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'source_kind' AND data_type = 'varchar' AND character_maximum_length = 16 AND is_nullable = 'NO' AND column_default IS NULL AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name IN ('source_id','namespace','idempotency_key') AND data_type = 'varchar' AND character_maximum_length = 128 AND is_nullable = 'NO' AND column_default IS NULL AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'status' AND data_type = 'varchar' AND character_maximum_length = 16 AND is_nullable = 'NO' AND column_default IS NULL AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'cursor_sequence' AND data_type = 'bigint' AND numeric_precision = 19 AND LOWER(column_type) NOT LIKE '%unsigned' AND is_nullable = 'NO' AND column_default = '0' AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'cursor_token' AND data_type = 'varchar' AND character_maximum_length = 255 AND is_nullable = 'NO' AND column_default = '' AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name IN ('fetched_count','processed_count','skipped_count','failed_count') AND data_type = 'int' AND numeric_precision = 10 AND LOWER(column_type) LIKE '%unsigned' AND is_nullable = 'NO' AND column_default = '0' AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'error_code' AND data_type = 'varchar' AND character_maximum_length = 96 AND is_nullable = 'NO' AND column_default = '' AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'attempt' AND data_type = 'int' AND numeric_precision = 10 AND LOWER(column_type) LIKE '%unsigned' AND is_nullable = 'NO' AND column_default = '1' AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'lease_token' AND data_type = 'varchar' AND character_maximum_length = 128 AND is_nullable = 'NO' AND column_default = '' AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name IN ('started_at','finished_at','lease_expires_at','heartbeat_at') AND data_type = 'datetime' AND datetime_precision = 6 AND is_nullable = 'YES' AND column_default IS NULL AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'created_at' AND data_type = 'datetime' AND datetime_precision = 6 AND is_nullable = 'NO' AND LOWER(COALESCE(column_default,'')) = 'current_timestamp(6)' AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'updated_at' AND data_type = 'datetime' AND datetime_precision = 6 AND is_nullable = 'NO' AND LOWER(COALESCE(column_default,'')) = 'current_timestamp(6)' AND REPLACE(LOWER(REPLACE(COALESCE(extra,''), ' ', '')), 'default_generated', '') = 'onupdatecurrent_timestamp(6)')
    )) <> 23
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND index_name = 'uk_archive_sync_run_idempotency' AND non_unique = 0 AND sub_part IS NULL), '') <> 'tenant_id,corp_id,source_kind,source_id,idempotency_key'
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND index_name = 'uk_archive_sync_run_scope_id' AND non_unique = 0 AND sub_part IS NULL), '') <> 'tenant_id,corp_id,id'
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND index_name = 'uk_archive_sync_run_identity' AND non_unique = 0 AND sub_part IS NULL), '') <> 'tenant_id,corp_id,id,source_kind,source_id,namespace'
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND index_name = 'PRIMARY' AND non_unique = 0 AND sub_part IS NULL), '') <> 'id'
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND index_name = 'idx_archive_sync_run_scope_status' AND non_unique = 1 AND sub_part IS NULL), '') <> 'tenant_id,corp_id,status,updated_at'
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND index_name = 'idx_archive_sync_run_source' AND non_unique = 1 AND sub_part IS NULL), '') <> 'tenant_id,corp_id,source_kind,source_id,updated_at'
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND constraint_name = 'fk_archive_sync_run_corp'), '') <> 'tenant_id=mc_corp.tenant_id,corp_id=mc_corp.id'
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND constraint_name = 'fk_archive_sync_run_corp') <> 2
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND constraint_name = 'fk_archive_sync_run_corp' AND ordinal_position = 1 AND column_name = 'tenant_id' AND referenced_table_name = 'mc_corp' AND referenced_column_name = 'tenant_id') <> 1
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_runs' AND constraint_name = 'fk_archive_sync_run_corp' AND ordinal_position = 2 AND column_name = 'corp_id' AND referenced_table_name = 'mc_corp' AND referenced_column_name = 'id') <> 1
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
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND ((column_name IN ('id','run_id') AND NOT (data_type = 'bigint' AND numeric_precision = 20 AND LOWER(column_type) LIKE '%unsigned')) OR (column_name IN ('tenant_id','corp_id') AND NOT (data_type = 'int' AND numeric_precision = 10 AND LOWER(column_type) LIKE '%unsigned')) OR (column_name = 'source_kind' AND character_maximum_length <> 16) OR (column_name IN ('source_id','namespace') AND character_maximum_length <> 128) OR (column_name IN ('action','status') AND character_maximum_length <> 16))) > 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND (
      (column_name = 'id' AND data_type = 'bigint' AND numeric_precision = 20 AND LOWER(column_type) LIKE '%unsigned' AND is_nullable = 'NO' AND column_default IS NULL AND LOWER(extra) LIKE '%auto_increment%')
      OR (column_name = 'run_id' AND data_type = 'bigint' AND numeric_precision = 20 AND LOWER(column_type) LIKE '%unsigned' AND is_nullable = 'NO' AND column_default IS NULL AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name IN ('tenant_id','corp_id') AND data_type = 'int' AND numeric_precision = 10 AND LOWER(column_type) LIKE '%unsigned' AND is_nullable = 'NO' AND column_default IS NULL AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'source_kind' AND data_type = 'varchar' AND character_maximum_length = 16 AND is_nullable = 'NO' AND column_default IS NULL AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name IN ('source_id','namespace') AND data_type = 'varchar' AND character_maximum_length = 128 AND is_nullable = 'NO' AND column_default IS NULL AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name IN ('action','status') AND data_type = 'varchar' AND character_maximum_length = 16 AND is_nullable = 'NO' AND column_default IS NULL AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'error_code' AND data_type = 'varchar' AND character_maximum_length = 96 AND is_nullable = 'NO' AND column_default = '' AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'cursor_sequence' AND data_type = 'bigint' AND numeric_precision = 19 AND LOWER(column_type) NOT LIKE '%unsigned' AND is_nullable = 'NO' AND column_default = '0' AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name IN ('fetched_count','processed_count','skipped_count','failed_count') AND data_type = 'int' AND numeric_precision = 10 AND LOWER(column_type) LIKE '%unsigned' AND is_nullable = 'NO' AND column_default = '0' AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'created_at' AND data_type = 'datetime' AND datetime_precision = 6 AND is_nullable = 'NO' AND LOWER(COALESCE(column_default,'')) = 'current_timestamp(6)' AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
    )) <> 16
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND index_name = 'PRIMARY' AND non_unique = 0 AND sub_part IS NULL), '') <> 'id'
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND index_name = 'idx_archive_sync_audit_scope' AND non_unique = 1 AND sub_part IS NULL), '') <> 'tenant_id,corp_id,created_at'
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND index_name = 'idx_archive_sync_audit_run' AND non_unique = 1 AND sub_part IS NULL), '') <> 'run_id,created_at'
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND constraint_name = 'fk_archive_sync_audit_run'), '') <> 'tenant_id=mochat_go_archive_sync_runs.tenant_id,corp_id=mochat_go_archive_sync_runs.corp_id,run_id=mochat_go_archive_sync_runs.id,source_kind=mochat_go_archive_sync_runs.source_kind,source_id=mochat_go_archive_sync_runs.source_id,namespace=mochat_go_archive_sync_runs.namespace'
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND index_name = 'idx_archive_sync_audit_scope' AND non_unique = 1 AND sub_part IS NULL) <> 3
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND index_name = 'idx_archive_sync_audit_scope' AND non_unique = 1 AND sub_part IS NULL AND seq_in_index = 1 AND column_name = 'tenant_id') <> 1
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND index_name = 'idx_archive_sync_audit_scope' AND non_unique = 1 AND sub_part IS NULL AND seq_in_index = 2 AND column_name = 'corp_id') <> 1
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND index_name = 'idx_archive_sync_audit_scope' AND non_unique = 1 AND sub_part IS NULL AND seq_in_index = 3 AND column_name = 'created_at') <> 1
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND constraint_name = 'fk_archive_sync_audit_run') <> 6
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND constraint_name = 'fk_archive_sync_audit_run' AND ordinal_position = 1 AND column_name = 'tenant_id' AND referenced_table_name = 'mochat_go_archive_sync_runs' AND referenced_column_name = 'tenant_id') <> 1
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND constraint_name = 'fk_archive_sync_audit_run' AND ordinal_position = 2 AND column_name = 'corp_id' AND referenced_table_name = 'mochat_go_archive_sync_runs' AND referenced_column_name = 'corp_id') <> 1
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND constraint_name = 'fk_archive_sync_audit_run' AND ordinal_position = 3 AND column_name = 'run_id' AND referenced_table_name = 'mochat_go_archive_sync_runs' AND referenced_column_name = 'id') <> 1
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND constraint_name = 'fk_archive_sync_audit_run' AND ordinal_position = 4 AND column_name = 'source_kind' AND referenced_table_name = 'mochat_go_archive_sync_runs' AND referenced_column_name = 'source_kind') <> 1
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND constraint_name = 'fk_archive_sync_audit_run' AND ordinal_position = 5 AND column_name = 'source_id' AND referenced_table_name = 'mochat_go_archive_sync_runs' AND referenced_column_name = 'source_id') <> 1
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_sync_audits' AND constraint_name = 'fk_archive_sync_audit_run' AND ordinal_position = 6 AND column_name = 'namespace' AND referenced_table_name = 'mochat_go_archive_sync_runs' AND referenced_column_name = 'namespace') <> 1
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
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND ((column_name IN ('id','run_id') AND NOT (data_type = 'bigint' AND numeric_precision = 20 AND LOWER(column_type) LIKE '%unsigned')) OR (column_name IN ('tenant_id','corp_id') AND NOT (data_type = 'int' AND numeric_precision = 10 AND LOWER(column_type) LIKE '%unsigned')) OR (column_name = 'source_kind' AND character_maximum_length <> 16) OR (column_name IN ('source_id','namespace') AND character_maximum_length <> 128))) > 0
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND (
      (column_name = 'id' AND data_type = 'bigint' AND numeric_precision = 20 AND LOWER(column_type) LIKE '%unsigned' AND is_nullable = 'NO' AND column_default IS NULL AND LOWER(extra) LIKE '%auto_increment%')
      OR (column_name IN ('tenant_id','corp_id') AND data_type = 'int' AND numeric_precision = 10 AND LOWER(column_type) LIKE '%unsigned' AND is_nullable = 'NO' AND column_default IS NULL AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'msgid' AND data_type = 'varchar' AND character_maximum_length = 255 AND is_nullable = 'NO' AND column_default IS NULL AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'source_kind' AND data_type = 'varchar' AND character_maximum_length = 16 AND is_nullable = 'NO' AND column_default IS NULL AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name IN ('source_id','namespace') AND data_type = 'varchar' AND character_maximum_length = 128 AND is_nullable = 'NO' AND column_default IS NULL AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'run_id' AND data_type = 'bigint' AND numeric_precision = 20 AND LOWER(column_type) LIKE '%unsigned' AND is_nullable = 'NO' AND column_default IS NULL AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'created_at' AND data_type = 'datetime' AND datetime_precision = 6 AND is_nullable = 'NO' AND LOWER(COALESCE(column_default,'')) = 'current_timestamp(6)' AND REPLACE(LOWER(COALESCE(extra,'')), 'default_generated', '') = '')
      OR (column_name = 'updated_at' AND data_type = 'datetime' AND datetime_precision = 6 AND is_nullable = 'NO' AND LOWER(COALESCE(column_default,'')) = 'current_timestamp(6)' AND REPLACE(LOWER(REPLACE(COALESCE(extra,''), ' ', '')), 'default_generated', '') = 'onupdatecurrent_timestamp(6)')
    )) <> 10
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND index_name = 'PRIMARY' AND non_unique = 0 AND sub_part IS NULL), '') <> 'id'
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND index_name = 'uk_archive_message_source_scope_msg' AND non_unique = 0 AND sub_part IS NULL), '') <> 'tenant_id,corp_id,msgid'
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND index_name = 'idx_archive_message_source_filter' AND non_unique = 1 AND sub_part IS NULL), '') <> 'tenant_id,corp_id,source_kind,source_id,created_at'
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND index_name = 'idx_archive_message_source_run' AND non_unique = 1 AND sub_part IS NULL), '') <> 'run_id'
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND constraint_name = 'fk_archive_message_source_run'), '') <> 'tenant_id=mochat_go_archive_sync_runs.tenant_id,corp_id=mochat_go_archive_sync_runs.corp_id,run_id=mochat_go_archive_sync_runs.id,source_kind=mochat_go_archive_sync_runs.source_kind,source_id=mochat_go_archive_sync_runs.source_id,namespace=mochat_go_archive_sync_runs.namespace'
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND index_name = 'uk_archive_message_source_scope_msg' AND non_unique = 0 AND sub_part IS NULL) <> 3
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND index_name = 'uk_archive_message_source_scope_msg' AND non_unique = 0 AND sub_part IS NULL AND seq_in_index = 1 AND column_name = 'tenant_id') <> 1
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND index_name = 'uk_archive_message_source_scope_msg' AND non_unique = 0 AND sub_part IS NULL AND seq_in_index = 2 AND column_name = 'corp_id') <> 1
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND index_name = 'uk_archive_message_source_scope_msg' AND non_unique = 0 AND sub_part IS NULL AND seq_in_index = 3 AND column_name = 'msgid') <> 1
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND constraint_name = 'fk_archive_message_source_run') <> 6
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND constraint_name = 'fk_archive_message_source_run' AND ordinal_position = 1 AND column_name = 'tenant_id' AND referenced_table_name = 'mochat_go_archive_sync_runs' AND referenced_column_name = 'tenant_id') <> 1
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND constraint_name = 'fk_archive_message_source_run' AND ordinal_position = 2 AND column_name = 'corp_id' AND referenced_table_name = 'mochat_go_archive_sync_runs' AND referenced_column_name = 'corp_id') <> 1
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND constraint_name = 'fk_archive_message_source_run' AND ordinal_position = 3 AND column_name = 'run_id' AND referenced_table_name = 'mochat_go_archive_sync_runs' AND referenced_column_name = 'id') <> 1
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND constraint_name = 'fk_archive_message_source_run' AND ordinal_position = 4 AND column_name = 'source_kind' AND referenced_table_name = 'mochat_go_archive_sync_runs' AND referenced_column_name = 'source_kind') <> 1
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND constraint_name = 'fk_archive_message_source_run' AND ordinal_position = 5 AND column_name = 'source_id' AND referenced_table_name = 'mochat_go_archive_sync_runs' AND referenced_column_name = 'source_id') <> 1
    OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_archive_message_sources' AND constraint_name = 'fk_archive_message_source_run' AND ordinal_position = 6 AND column_name = 'namespace' AND referenced_table_name = 'mochat_go_archive_sync_runs' AND referenced_column_name = 'namespace') <> 1
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
  `lease_token` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
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

-- 0133 may already have retained simulation batches when 0138 is installed.
-- Backfill an explicit run and source identity for every completed legacy batch
-- so old simulation rows cannot fall through to the external default. The
-- dynamic wrapper keeps a clean install without 0133 compatible and each
-- statement remains session-local for the migration runner's pinned Conn.
SET @archive_legacy_simulation_tables := (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name IN ('mochat_go_archive_simulation_batches', 'mochat_go_archive_simulation_messages')
);
SET @archive_legacy_simulation_runs_sql := IF(
  @archive_legacy_simulation_tables = 2,
  'INSERT INTO `mochat_go_archive_sync_runs`
   (`tenant_id`,`corp_id`,`source_kind`,`source_id`,`namespace`,`idempotency_key`,`status`)
   SELECT corp.`tenant_id`, batch.`corp_id`, ''simulated'', CONCAT(''simulation:'', batch.`batch_key`),
          CONCAT(''MOCHAT-SIM:'', batch.`batch_key`), CONCAT(''legacy-simulation:'', batch.`id`), ''succeeded''
   FROM `mochat_go_archive_simulation_batches` batch
   INNER JOIN `mc_corp` corp ON corp.`id` = batch.`corp_id` AND corp.`deleted_at` IS NULL
   WHERE batch.`status` = ''complete''
   ON DUPLICATE KEY UPDATE `id` = LAST_INSERT_ID(`mochat_go_archive_sync_runs`.`id`)',
  'SELECT 1'
);
PREPARE archive_legacy_simulation_runs_stmt FROM @archive_legacy_simulation_runs_sql;
EXECUTE archive_legacy_simulation_runs_stmt;
DEALLOCATE PREPARE archive_legacy_simulation_runs_stmt;

SET @archive_legacy_simulation_sources_sql := IF(
  @archive_legacy_simulation_tables = 2,
  'INSERT INTO `mochat_go_archive_message_sources`
   (`tenant_id`,`corp_id`,`msgid`,`source_kind`,`source_id`,`namespace`,`run_id`)
   SELECT corp.`tenant_id`, batch.`corp_id`, message.`msgid`, ''simulated'',
          CONCAT(''simulation:'', batch.`batch_key`), CONCAT(''MOCHAT-SIM:'', batch.`batch_key`), run.`id`
   FROM `mochat_go_archive_simulation_messages` message
   INNER JOIN `mochat_go_archive_simulation_batches` batch
     ON batch.`id` = message.`batch_id` AND batch.`corp_id` = message.`corp_id` AND batch.`status` = ''complete''
   INNER JOIN `mc_corp` corp ON corp.`id` = batch.`corp_id` AND corp.`deleted_at` IS NULL
   INNER JOIN `mochat_go_archive_sync_runs` run
     ON run.`tenant_id` = corp.`tenant_id` AND run.`corp_id` = batch.`corp_id`
    AND run.`source_kind` = ''simulated''
    AND run.`source_id` = CONCAT(''simulation:'', batch.`batch_key`)
    AND run.`namespace` = CONCAT(''MOCHAT-SIM:'', batch.`batch_key`)
   WHERE NOT EXISTS (
     SELECT 1 FROM `mochat_go_archive_message_sources` existing
     WHERE existing.`tenant_id` = corp.`tenant_id` AND existing.`corp_id` = batch.`corp_id` AND existing.`msgid` = message.`msgid`
   )',
  'SELECT 1'
);
PREPARE archive_legacy_simulation_sources_stmt FROM @archive_legacy_simulation_sources_sql;
EXECUTE archive_legacy_simulation_sources_stmt;
DEALLOCATE PREPARE archive_legacy_simulation_sources_stmt;
