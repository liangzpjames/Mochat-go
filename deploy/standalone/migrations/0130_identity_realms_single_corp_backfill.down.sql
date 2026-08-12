-- 0130 rollback is journal/request scoped and is safe after a first-DDL or
-- mid-phase failure. Every optional table/constraint lookup is guarded by
-- information_schema and executed dynamically; no paired-column assumption is
-- made about a partially-created object.

SET @identity_0130_down_request_id := COALESCE(NULLIF(@identity_0130_requested_down_request_id, ''), '');
SET @identity_0130_down_read_ledger_sql := IF(
  @identity_0130_down_request_id = '' AND (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_ledger') = 1,
  'SELECT COALESCE((SELECT request_id FROM mochat_go_identity_migration_ledger WHERE migration_name = ''0130_identity_realms_single_corp_backfill'' AND status = ''success'' ORDER BY id DESC LIMIT 1), '''') INTO @identity_0130_down_request_id',
  'SELECT 1'
);
PREPARE identity_0130_down_read_ledger_stmt FROM @identity_0130_down_read_ledger_sql;
EXECUTE identity_0130_down_read_ledger_stmt;
DEALLOCATE PREPARE identity_0130_down_read_ledger_stmt;

SET @identity_0130_down_read_journal_sql := IF(
  @identity_0130_down_request_id = '' AND (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_journal') = 1,
  'SELECT COALESCE((SELECT request_id FROM mochat_go_identity_migration_journal ORDER BY id DESC LIMIT 1), '''') INTO @identity_0130_down_request_id',
  'SELECT 1'
);
PREPARE identity_0130_down_read_journal_stmt FROM @identity_0130_down_read_journal_sql;
EXECUTE identity_0130_down_read_journal_stmt;
DEALLOCATE PREPARE identity_0130_down_read_journal_stmt;

SET @identity_0130_down_journal_column_count := (
  SELECT COUNT(*)
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'mochat_go_identity_migration_journal'
    AND column_name IN ('request_id', 'entity_type', 'entity_id')
);
SET @identity_0130_down_ledger_column_count := (
  SELECT COUNT(*)
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'mochat_go_identity_migration_ledger'
    AND column_name IN ('migration_name', 'request_id', 'phase', 'status', 'result_json')
);
SET @identity_0130_down_schema_guard_sql := IF(
  ((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_journal') = 1 AND @identity_0130_down_journal_column_count <> 3)
  OR ((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_ledger') = 1 AND @identity_0130_down_ledger_column_count <> 5),
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 rollback metadata schema is incomplete''',
  'SELECT 1'
);
PREPARE identity_0130_down_schema_guard_stmt FROM @identity_0130_down_schema_guard_sql;
EXECUTE identity_0130_down_schema_guard_stmt;
DEALLOCATE PREPARE identity_0130_down_schema_guard_stmt;

SET @identity_0130_down_journal_total_count := 0;
SET @identity_0130_down_journal_match_count := 0;
SET @identity_0130_down_journal_count_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_journal') = 1,
  'SELECT COUNT(*), COALESCE(SUM(request_id COLLATE utf8mb4_unicode_ci = @identity_0130_down_request_id COLLATE utf8mb4_unicode_ci), 0) INTO @identity_0130_down_journal_total_count, @identity_0130_down_journal_match_count FROM mochat_go_identity_migration_journal',
  'SELECT 0, 0'
);
PREPARE identity_0130_down_journal_count_stmt FROM @identity_0130_down_journal_count_sql;
EXECUTE identity_0130_down_journal_count_stmt;
DEALLOCATE PREPARE identity_0130_down_journal_count_stmt;

SET @identity_0130_down_ledger_total_count := 0;
SET @identity_0130_down_ledger_match_count := 0;
SET @identity_0130_down_ledger_count_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_ledger') = 1,
  'SELECT COUNT(*), COALESCE(SUM(request_id COLLATE utf8mb4_unicode_ci = @identity_0130_down_request_id COLLATE utf8mb4_unicode_ci), 0) INTO @identity_0130_down_ledger_total_count, @identity_0130_down_ledger_match_count FROM mochat_go_identity_migration_ledger',
  'SELECT 0, 0'
);
PREPARE identity_0130_down_ledger_count_stmt FROM @identity_0130_down_ledger_count_sql;
EXECUTE identity_0130_down_ledger_count_stmt;
DEALLOCATE PREPARE identity_0130_down_ledger_count_stmt;

SET @identity_0130_down_request_guard_sql := IF(
  (@identity_0130_down_request_id = '' AND @identity_0130_down_journal_total_count = 0 AND @identity_0130_down_ledger_total_count = 0)
  OR (@identity_0130_down_request_id <> ''
      AND (@identity_0130_down_journal_total_count = 0 OR @identity_0130_down_journal_total_count = @identity_0130_down_journal_match_count)
      AND (@identity_0130_down_ledger_total_count = 0 OR @identity_0130_down_ledger_total_count = @identity_0130_down_ledger_match_count)),
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 rollback request does not own all metadata'''
);
PREPARE identity_0130_down_request_guard_stmt FROM @identity_0130_down_request_guard_sql;
EXECUTE identity_0130_down_request_guard_stmt;
DEALLOCATE PREPARE identity_0130_down_request_guard_stmt;

SET @identity_0130_down_corp_journal_count := 0;
SET @identity_0130_down_corp_journal_count_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_journal') = 1,
  'SELECT COUNT(*) INTO @identity_0130_down_corp_journal_count FROM mochat_go_identity_migration_journal WHERE request_id COLLATE utf8mb4_unicode_ci = @identity_0130_down_request_id COLLATE utf8mb4_unicode_ci AND entity_type COLLATE utf8mb4_unicode_ci = ''corp_credentials'' COLLATE utf8mb4_unicode_ci',
  'SELECT 0 INTO @identity_0130_down_corp_journal_count'
);
PREPARE identity_0130_down_corp_journal_count_stmt FROM @identity_0130_down_corp_journal_count_sql;
EXECUTE identity_0130_down_corp_journal_count_stmt;
DEALLOCATE PREPARE identity_0130_down_corp_journal_count_stmt;

SET @identity_0130_down_agent_journal_count := 0;
SET @identity_0130_down_agent_journal_count_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_journal') = 1,
  'SELECT COUNT(*) INTO @identity_0130_down_agent_journal_count FROM mochat_go_identity_migration_journal WHERE request_id COLLATE utf8mb4_unicode_ci = @identity_0130_down_request_id COLLATE utf8mb4_unicode_ci AND entity_type COLLATE utf8mb4_unicode_ci = ''agent_credentials'' COLLATE utf8mb4_unicode_ci',
  'SELECT 0 INTO @identity_0130_down_agent_journal_count'
);
PREPARE identity_0130_down_agent_journal_count_stmt FROM @identity_0130_down_agent_journal_count_sql;
EXECUTE identity_0130_down_agent_journal_count_stmt;
DEALLOCATE PREPARE identity_0130_down_agent_journal_count_stmt;

SET @identity_0130_down_credential_target_guard_sql := IF(
  (@identity_0130_down_corp_journal_count > 0 AND (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mc_corp') <> 1)
  OR (@identity_0130_down_agent_journal_count > 0 AND (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mc_work_agent') <> 1),
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 rollback credential target is missing''',
  'SELECT 1'
);
PREPARE identity_0130_down_credential_target_guard_stmt FROM @identity_0130_down_credential_target_guard_sql;
EXECUTE identity_0130_down_credential_target_guard_stmt;
DEALLOCATE PREPARE identity_0130_down_credential_target_guard_stmt;

START TRANSACTION;

SET @identity_0130_down_corp_missing_count := 0;
SET @identity_0130_down_corp_digest_mismatch_count := 0;
SET @identity_0130_down_corp_plaintext_missing_count := 0;
SET @identity_0130_down_corp_verify_sql := IF(
  @identity_0130_down_corp_journal_count > 0,
  'SELECT COALESCE(SUM(c.id IS NULL), 0), COALESCE(SUM(COALESCE(JSON_UNQUOTE(JSON_EXTRACT(j.before_json, ''$.afterCiphertextSha256'')), '''') <> SHA2(CAST(c.wecom_credentials_ciphertext AS CHAR), 256)), 0), COALESCE(SUM(COALESCE(c.employee_secret, '''') = '''' AND COALESCE(c.contact_secret, '''') = '''' AND COALESCE(c.token, '''') = '''' AND COALESCE(c.encoding_aes_key, '''') = '''' AND COALESCE(c.chat_secret, '''') = ''''), 0) INTO @identity_0130_down_corp_missing_count, @identity_0130_down_corp_digest_mismatch_count, @identity_0130_down_corp_plaintext_missing_count FROM mochat_go_identity_migration_journal j LEFT JOIN mc_corp c ON c.id = CAST(j.entity_id AS UNSIGNED) WHERE j.request_id COLLATE utf8mb4_unicode_ci = @identity_0130_down_request_id COLLATE utf8mb4_unicode_ci AND j.entity_type COLLATE utf8mb4_unicode_ci = ''corp_credentials'' COLLATE utf8mb4_unicode_ci FOR UPDATE',
  'SELECT 0, 0, 0 INTO @identity_0130_down_corp_missing_count, @identity_0130_down_corp_digest_mismatch_count, @identity_0130_down_corp_plaintext_missing_count'
);
PREPARE identity_0130_down_corp_verify_stmt FROM @identity_0130_down_corp_verify_sql;
EXECUTE identity_0130_down_corp_verify_stmt;
DEALLOCATE PREPARE identity_0130_down_corp_verify_stmt;

SET @identity_0130_down_corp_key_mismatch_count := 0;
SET @identity_0130_down_corp_key_verify_sql := IF(
  @identity_0130_down_corp_journal_count > 0,
  'SELECT COALESCE(SUM(COALESCE(JSON_UNQUOTE(JSON_EXTRACT(j.before_json, ''$.writtenKeyId'')), '''') COLLATE utf8mb4_bin <> COALESCE(c.wecom_credentials_key_id, '''') COLLATE utf8mb4_bin), 0) INTO @identity_0130_down_corp_key_mismatch_count FROM mochat_go_identity_migration_journal j LEFT JOIN mc_corp c ON c.id = CAST(j.entity_id AS UNSIGNED) WHERE j.request_id COLLATE utf8mb4_unicode_ci = @identity_0130_down_request_id COLLATE utf8mb4_unicode_ci AND j.entity_type COLLATE utf8mb4_unicode_ci = ''corp_credentials'' COLLATE utf8mb4_unicode_ci',
  'SELECT 0 INTO @identity_0130_down_corp_key_mismatch_count'
);
PREPARE identity_0130_down_corp_key_verify_stmt FROM @identity_0130_down_corp_key_verify_sql;
EXECUTE identity_0130_down_corp_key_verify_stmt;
DEALLOCATE PREPARE identity_0130_down_corp_key_verify_stmt;

SET @identity_0130_down_agent_missing_count := 0;
SET @identity_0130_down_agent_digest_mismatch_count := 0;
SET @identity_0130_down_agent_plaintext_missing_count := 0;
SET @identity_0130_down_agent_verify_sql := IF(
  @identity_0130_down_agent_journal_count > 0,
  'SELECT COALESCE(SUM(a.id IS NULL), 0), COALESCE(SUM(COALESCE(JSON_UNQUOTE(JSON_EXTRACT(j.before_json, ''$.afterCiphertextSha256'')), '''') <> SHA2(CAST(a.wecom_credentials_ciphertext AS CHAR), 256)), 0), COALESCE(SUM(COALESCE(a.wx_secret, '''') = ''''), 0) INTO @identity_0130_down_agent_missing_count, @identity_0130_down_agent_digest_mismatch_count, @identity_0130_down_agent_plaintext_missing_count FROM mochat_go_identity_migration_journal j LEFT JOIN mc_work_agent a ON a.id = CAST(j.entity_id AS UNSIGNED) WHERE j.request_id COLLATE utf8mb4_unicode_ci = @identity_0130_down_request_id COLLATE utf8mb4_unicode_ci AND j.entity_type COLLATE utf8mb4_unicode_ci = ''agent_credentials'' COLLATE utf8mb4_unicode_ci FOR UPDATE',
  'SELECT 0, 0, 0 INTO @identity_0130_down_agent_missing_count, @identity_0130_down_agent_digest_mismatch_count, @identity_0130_down_agent_plaintext_missing_count'
);
PREPARE identity_0130_down_agent_verify_stmt FROM @identity_0130_down_agent_verify_sql;
EXECUTE identity_0130_down_agent_verify_stmt;
DEALLOCATE PREPARE identity_0130_down_agent_verify_stmt;

SET @identity_0130_down_agent_key_mismatch_count := 0;
SET @identity_0130_down_agent_key_verify_sql := IF(
  @identity_0130_down_agent_journal_count > 0,
  'SELECT COALESCE(SUM(COALESCE(JSON_UNQUOTE(JSON_EXTRACT(j.before_json, ''$.writtenKeyId'')), '''') COLLATE utf8mb4_bin <> COALESCE(a.wecom_credentials_key_id, '''') COLLATE utf8mb4_bin), 0) INTO @identity_0130_down_agent_key_mismatch_count FROM mochat_go_identity_migration_journal j LEFT JOIN mc_work_agent a ON a.id = CAST(j.entity_id AS UNSIGNED) WHERE j.request_id COLLATE utf8mb4_unicode_ci = @identity_0130_down_request_id COLLATE utf8mb4_unicode_ci AND j.entity_type COLLATE utf8mb4_unicode_ci = ''agent_credentials'' COLLATE utf8mb4_unicode_ci',
  'SELECT 0 INTO @identity_0130_down_agent_key_mismatch_count'
);
PREPARE identity_0130_down_agent_key_verify_stmt FROM @identity_0130_down_agent_key_verify_sql;
EXECUTE identity_0130_down_agent_key_verify_stmt;
DEALLOCATE PREPARE identity_0130_down_agent_key_verify_stmt;

SET @identity_0130_down_credential_verify_guard_sql := IF(
  @identity_0130_down_corp_missing_count = 0
  AND @identity_0130_down_corp_digest_mismatch_count = 0
  AND @identity_0130_down_corp_key_mismatch_count = 0
  AND @identity_0130_down_corp_plaintext_missing_count = 0
  AND @identity_0130_down_agent_missing_count = 0
  AND @identity_0130_down_agent_digest_mismatch_count = 0
  AND @identity_0130_down_agent_key_mismatch_count = 0
  AND @identity_0130_down_agent_plaintext_missing_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 rollback credential journal verification failed'''
);
PREPARE identity_0130_down_credential_verify_guard_stmt FROM @identity_0130_down_credential_verify_guard_sql;
EXECUTE identity_0130_down_credential_verify_guard_stmt;
DEALLOCATE PREPARE identity_0130_down_credential_verify_guard_stmt;

SET @identity_0130_down_clear_corp_sql := IF(
  @identity_0130_down_corp_journal_count > 0,
  'UPDATE mc_corp c INNER JOIN mochat_go_identity_migration_journal j ON j.request_id COLLATE utf8mb4_unicode_ci = @identity_0130_down_request_id COLLATE utf8mb4_unicode_ci AND j.entity_type COLLATE utf8mb4_unicode_ci = ''corp_credentials'' COLLATE utf8mb4_unicode_ci AND CAST(j.entity_id AS UNSIGNED) = c.id SET c.wecom_credentials_ciphertext = NULL, c.wecom_credentials_key_id = '''' WHERE COALESCE(JSON_UNQUOTE(JSON_EXTRACT(j.before_json, ''$.afterCiphertextSha256'')), '''') = SHA2(CAST(c.wecom_credentials_ciphertext AS CHAR), 256) AND COALESCE(JSON_UNQUOTE(JSON_EXTRACT(j.before_json, ''$.writtenKeyId'')), '''') COLLATE utf8mb4_bin = COALESCE(c.wecom_credentials_key_id, '''') COLLATE utf8mb4_bin',
  'SELECT 1'
);
PREPARE identity_0130_down_clear_corp_stmt FROM @identity_0130_down_clear_corp_sql;
EXECUTE identity_0130_down_clear_corp_stmt;
DEALLOCATE PREPARE identity_0130_down_clear_corp_stmt;

SET @identity_0130_down_clear_agent_sql := IF(
  @identity_0130_down_agent_journal_count > 0,
  'UPDATE mc_work_agent a INNER JOIN mochat_go_identity_migration_journal j ON j.request_id COLLATE utf8mb4_unicode_ci = @identity_0130_down_request_id COLLATE utf8mb4_unicode_ci AND j.entity_type COLLATE utf8mb4_unicode_ci = ''agent_credentials'' COLLATE utf8mb4_unicode_ci AND CAST(j.entity_id AS UNSIGNED) = a.id SET a.wecom_credentials_ciphertext = NULL, a.wecom_credentials_key_id = '''' WHERE COALESCE(JSON_UNQUOTE(JSON_EXTRACT(j.before_json, ''$.afterCiphertextSha256'')), '''') = SHA2(CAST(a.wecom_credentials_ciphertext AS CHAR), 256) AND COALESCE(JSON_UNQUOTE(JSON_EXTRACT(j.before_json, ''$.writtenKeyId'')), '''') COLLATE utf8mb4_bin = COALESCE(a.wecom_credentials_key_id, '''') COLLATE utf8mb4_bin',
  'SELECT 1'
);
PREPARE identity_0130_down_clear_agent_stmt FROM @identity_0130_down_clear_agent_sql;
EXECUTE identity_0130_down_clear_agent_stmt;
DEALLOCATE PREPARE identity_0130_down_clear_agent_stmt;

SET @identity_0130_down_delete_credential_journal_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_journal') = 1,
  'DELETE FROM mochat_go_identity_migration_journal WHERE request_id COLLATE utf8mb4_unicode_ci = @identity_0130_down_request_id COLLATE utf8mb4_unicode_ci AND entity_type COLLATE utf8mb4_unicode_ci IN (''corp_credentials'' COLLATE utf8mb4_unicode_ci, ''agent_credentials'' COLLATE utf8mb4_unicode_ci)',
  'SELECT 1'
);
PREPARE identity_0130_down_delete_credential_journal_stmt FROM @identity_0130_down_delete_credential_journal_sql;
EXECUTE identity_0130_down_delete_credential_journal_stmt;
DEALLOCATE PREPARE identity_0130_down_delete_credential_journal_stmt;

COMMIT;

SET @identity_0130_down_drop_role_fk_sql := IF(
  (SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_saas_admin_user_roles' AND constraint_name = 'fk_saas_admin_user_roles_identity') = 1,
  'ALTER TABLE mochat_go_saas_admin_user_roles DROP FOREIGN KEY fk_saas_admin_user_roles_identity',
  'SELECT 1'
);
PREPARE identity_0130_down_drop_role_fk_stmt FROM @identity_0130_down_drop_role_fk_sql;
EXECUTE identity_0130_down_drop_role_fk_stmt;
DEALLOCATE PREPARE identity_0130_down_drop_role_fk_stmt;

SET @identity_0130_down_drop_access_fk_sql := IF(
  (SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_saas_admin_user_access' AND constraint_name = 'fk_saas_admin_user_access_identity') = 1,
  'ALTER TABLE mochat_go_saas_admin_user_access DROP FOREIGN KEY fk_saas_admin_user_access_identity',
  'SELECT 1'
);
PREPARE identity_0130_down_drop_access_fk_stmt FROM @identity_0130_down_drop_access_fk_sql;
EXECUTE identity_0130_down_drop_access_fk_stmt;
DEALLOCATE PREPARE identity_0130_down_drop_access_fk_stmt;

SET @identity_0130_down_delete_bindings_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('mochat_go_identity_migration_journal', 'mochat_go_tenant_corp_bindings')) = 2,
  'DELETE b FROM mochat_go_tenant_corp_bindings b INNER JOIN mochat_go_identity_migration_journal j ON j.request_id COLLATE utf8mb4_unicode_ci = @identity_0130_down_request_id COLLATE utf8mb4_unicode_ci AND j.entity_type COLLATE utf8mb4_unicode_ci = ''tenant_corp_binding'' COLLATE utf8mb4_unicode_ci AND CAST(j.entity_id AS UNSIGNED) = b.tenant_id',
  'SELECT 1'
);
PREPARE identity_0130_down_delete_bindings_stmt FROM @identity_0130_down_delete_bindings_sql;
EXECUTE identity_0130_down_delete_bindings_stmt;
DEALLOCATE PREPARE identity_0130_down_delete_bindings_stmt;

SET @identity_0130_down_delete_dashboard_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('mochat_go_identity_migration_journal', 'mochat_go_dashboard_identities')) = 2,
  'DELETE d FROM mochat_go_dashboard_identities d INNER JOIN mochat_go_identity_migration_journal j ON j.request_id COLLATE utf8mb4_unicode_ci = @identity_0130_down_request_id COLLATE utf8mb4_unicode_ci AND j.entity_type COLLATE utf8mb4_unicode_ci = ''dashboard_identity'' COLLATE utf8mb4_unicode_ci AND CAST(j.entity_id AS UNSIGNED) = d.user_id',
  'SELECT 1'
);
PREPARE identity_0130_down_delete_dashboard_stmt FROM @identity_0130_down_delete_dashboard_sql;
EXECUTE identity_0130_down_delete_dashboard_stmt;
DEALLOCATE PREPARE identity_0130_down_delete_dashboard_stmt;

SET @identity_0130_down_delete_saas_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('mochat_go_identity_migration_journal', 'mochat_go_saas_admin_users')) = 2,
  'DELETE s FROM mochat_go_saas_admin_users s INNER JOIN mochat_go_identity_migration_journal j ON j.request_id COLLATE utf8mb4_unicode_ci = @identity_0130_down_request_id COLLATE utf8mb4_unicode_ci AND j.entity_type COLLATE utf8mb4_unicode_ci = ''saas_identity'' COLLATE utf8mb4_unicode_ci AND CAST(j.entity_id AS UNSIGNED) = s.id',
  'SELECT 1'
);
PREPARE identity_0130_down_delete_saas_stmt FROM @identity_0130_down_delete_saas_sql;
EXECUTE identity_0130_down_delete_saas_stmt;
DEALLOCATE PREPARE identity_0130_down_delete_saas_stmt;

SET @identity_0130_down_delete_placeholder_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('mochat_go_identity_migration_journal', 'mc_corp')) = 2
  AND (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mc_corp' AND column_name IN ('name', 'wx_corpid')) = 2,
  'DELETE c FROM mc_corp c INNER JOIN mochat_go_identity_migration_journal j ON j.request_id COLLATE utf8mb4_unicode_ci = @identity_0130_down_request_id COLLATE utf8mb4_unicode_ci AND j.entity_type COLLATE utf8mb4_unicode_ci = ''corp_placeholder'' COLLATE utf8mb4_unicode_ci AND CAST(j.entity_id AS UNSIGNED) = c.tenant_id WHERE c.wx_corpid COLLATE utf8mb4_unicode_ci = '''' COLLATE utf8mb4_unicode_ci AND c.name COLLATE utf8mb4_unicode_ci = CONCAT(''Migration placeholder [0130:'', @identity_0130_down_request_id, ''] tenant '', c.tenant_id) COLLATE utf8mb4_unicode_ci',
  'SELECT 1'
);
PREPARE identity_0130_down_delete_placeholder_stmt FROM @identity_0130_down_delete_placeholder_sql;
EXECUTE identity_0130_down_delete_placeholder_stmt;
DEALLOCATE PREPARE identity_0130_down_delete_placeholder_stmt;

SET @identity_0130_down_drop_corp_map_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_corp_map') = 1,
  'DROP TABLE mochat_go_identity_migration_corp_map',
  'SELECT 1'
);
PREPARE identity_0130_down_drop_corp_map_stmt FROM @identity_0130_down_drop_corp_map_sql;
EXECUTE identity_0130_down_drop_corp_map_stmt;
DEALLOCATE PREPARE identity_0130_down_drop_corp_map_stmt;

SET @identity_0130_down_drop_batches_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_batches') = 1,
  'DROP TABLE mochat_go_identity_migration_batches',
  'SELECT 1'
);
PREPARE identity_0130_down_drop_batches_stmt FROM @identity_0130_down_drop_batches_sql;
EXECUTE identity_0130_down_drop_batches_stmt;
DEALLOCATE PREPARE identity_0130_down_drop_batches_stmt;

SET @identity_0130_down_drop_journal_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_journal') = 1,
  'DROP TABLE mochat_go_identity_migration_journal',
  'SELECT 1'
);
PREPARE identity_0130_down_drop_journal_stmt FROM @identity_0130_down_drop_journal_sql;
EXECUTE identity_0130_down_drop_journal_stmt;
DEALLOCATE PREPARE identity_0130_down_drop_journal_stmt;

SET @identity_0130_down_drop_ledger_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_ledger') = 1,
  'DROP TABLE mochat_go_identity_migration_ledger',
  'SELECT 1'
);
PREPARE identity_0130_down_drop_ledger_stmt FROM @identity_0130_down_drop_ledger_sql;
EXECUTE identity_0130_down_drop_ledger_stmt;
DEALLOCATE PREPARE identity_0130_down_drop_ledger_stmt;
