-- 0131 one-time identity cutover.
--
-- MariaDB DDL is not transactional. Every consistency check below is run
-- before the first DDL. The durable cutover journal contains ids and state
-- only; it never contains passwords, secrets, tokens, hashes, or ciphertext.
-- The maintenance CLI verifies every undecryptable credential before this
-- script is executed; SQL never treats a key id alone as proof.

SET @identity_0131_request_id := COALESCE(@identity_0131_request_id, '');
SET @identity_0131_platform_tenant_id := COALESCE(@identity_0131_platform_tenant_id, 0);

SET @identity_0131_request_guard_sql := IF(
  @identity_0131_request_id <> '' AND @identity_0131_platform_tenant_id > 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 maintenance request and platform tenant are required'''
);
PREPARE identity_0131_request_guard_stmt FROM @identity_0131_request_guard_sql;
EXECUTE identity_0131_request_guard_stmt;
DEALLOCATE PREPARE identity_0131_request_guard_stmt;

SET @identity_0131_standard_ledger_count := (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE() AND table_name = 'mochat_go_schema_migrations'
);
SET @identity_0131_standard_ledger_guard_sql := IF(
  @identity_0131_standard_ledger_count = 1,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 standard migration ledger is missing'''
);
PREPARE identity_0131_standard_ledger_guard_stmt FROM @identity_0131_standard_ledger_guard_sql;
EXECUTE identity_0131_standard_ledger_guard_stmt;
DEALLOCATE PREPARE identity_0131_standard_ledger_guard_stmt;

SET @identity_0131_standard_version_count := (
  SELECT COUNT(*)
  FROM mochat_go_schema_migrations
  WHERE version IN ('0129_identity_realms_single_corp_schema', '0130_identity_realms_single_corp_backfill')
);
SET @identity_0131_standard_version_guard_sql := IF(
  @identity_0131_standard_version_count = 2,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 requires completed 0129 and 0130 ledger facts'''
);
PREPARE identity_0131_standard_version_guard_stmt FROM @identity_0131_standard_version_guard_sql;
EXECUTE identity_0131_standard_version_guard_stmt;
DEALLOCATE PREPARE identity_0131_standard_version_guard_stmt;

SET @identity_0131_backfill_ledger_count := (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_ledger'
);
SET @identity_0131_backfill_batch_count := (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_batches'
);
SET @identity_0131_backfill_metadata_guard_sql := IF(
  @identity_0131_backfill_ledger_count = 1 AND @identity_0131_backfill_batch_count = 1,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 backfill metadata is missing'''
);
PREPARE identity_0131_backfill_metadata_guard_stmt FROM @identity_0131_backfill_metadata_guard_sql;
EXECUTE identity_0131_backfill_metadata_guard_stmt;
DEALLOCATE PREPARE identity_0131_backfill_metadata_guard_stmt;

SET @identity_0131_backfill_success_count := (
  SELECT COUNT(*)
  FROM mochat_go_identity_migration_ledger
  WHERE migration_name = '0130_identity_realms_single_corp_backfill'
    AND phase = 'backfill' AND status = 'success'
);
SET @identity_0131_backfill_batch_success_count := (
  SELECT COUNT(*)
  FROM mochat_go_identity_migration_batches
  WHERE migration_source = '0130_identity_realms_single_corp_backfill' AND status = 'completed'
);
SET @identity_0131_backfill_success_guard_sql := IF(
  @identity_0131_backfill_success_count = 1 AND @identity_0131_backfill_batch_success_count = 1,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 requires a completed 0130 backfill'''
);
PREPARE identity_0131_backfill_success_guard_stmt FROM @identity_0131_backfill_success_guard_sql;
EXECUTE identity_0131_backfill_success_guard_stmt;
DEALLOCATE PREPARE identity_0131_backfill_success_guard_stmt;

SET @identity_0131_active_identity_missing_count := (
  SELECT COUNT(*)
  FROM mc_user u
  LEFT JOIN mochat_go_dashboard_identities d ON d.user_id = u.id
  WHERE u.tenant_id <> @identity_0131_platform_tenant_id
    AND u.status = 1 AND u.deleted_at IS NULL AND d.user_id IS NULL
);
SET @identity_0131_active_identity_guard_sql := IF(
  @identity_0131_active_identity_missing_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 active user is missing dashboard identity'''
);
PREPARE identity_0131_active_identity_guard_stmt FROM @identity_0131_active_identity_guard_sql;
EXECUTE identity_0131_active_identity_guard_stmt;
DEALLOCATE PREPARE identity_0131_active_identity_guard_stmt;

SET @identity_0131_invalid_binding_count := (
  SELECT COUNT(*)
  FROM (
    SELECT t.id
    FROM mc_tenant t
    LEFT JOIN mochat_go_tenant_corp_bindings b ON b.tenant_id = t.id
    WHERE t.id <> @identity_0131_platform_tenant_id AND t.status = 1 AND t.deleted_at IS NULL
    GROUP BY t.id
    HAVING COUNT(b.tenant_id) <> 1
  ) invalid_bindings
);
SET @identity_0131_binding_guard_sql := IF(
  @identity_0131_invalid_binding_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 active tenant does not have exactly one binding'''
);
PREPARE identity_0131_binding_guard_stmt FROM @identity_0131_binding_guard_sql;
EXECUTE identity_0131_binding_guard_stmt;
DEALLOCATE PREPARE identity_0131_binding_guard_stmt;

SET @identity_0131_dangling_binding_count := (
  SELECT COUNT(*)
  FROM mochat_go_tenant_corp_bindings b
  LEFT JOIN mc_corp c ON c.id = b.corp_id
  WHERE c.id IS NULL OR c.tenant_id <> b.tenant_id
);
SET @identity_0131_dangling_binding_guard_sql := IF(
  @identity_0131_dangling_binding_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 dangling or cross-tenant binding'''
);
PREPARE identity_0131_dangling_binding_guard_stmt FROM @identity_0131_dangling_binding_guard_sql;
EXECUTE identity_0131_dangling_binding_guard_stmt;
DEALLOCATE PREPARE identity_0131_dangling_binding_guard_stmt;

SET @identity_0131_missing_corp_ciphertext_count := (
  SELECT COUNT(*)
  FROM mc_corp c
  WHERE (
    COALESCE(c.employee_secret, '') <> '' OR COALESCE(c.contact_secret, '') <> ''
    OR COALESCE(c.token, '') <> '' OR COALESCE(c.encoding_aes_key, '') <> ''
    OR COALESCE(c.chat_secret, '') <> ''
  ) AND (COALESCE(CAST(c.wecom_credentials_ciphertext AS CHAR), '') = '' OR COALESCE(c.wecom_credentials_key_id, '') = '')
);
SET @identity_0131_missing_agent_ciphertext_count := (
  SELECT COUNT(*)
  FROM mc_work_agent a
  WHERE COALESCE(a.wx_secret, '') <> ''
    AND (COALESCE(CAST(a.wecom_credentials_ciphertext AS CHAR), '') = '' OR COALESCE(a.wecom_credentials_key_id, '') = '')
);
SET @identity_0131_credential_guard_sql := IF(
  @identity_0131_missing_corp_ciphertext_count = 0 AND @identity_0131_missing_agent_ciphertext_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 credential ciphertext copy is missing'''
);
PREPARE identity_0131_credential_guard_stmt FROM @identity_0131_credential_guard_sql;
EXECUTE identity_0131_credential_guard_stmt;
DEALLOCATE PREPARE identity_0131_credential_guard_stmt;

SET @identity_0131_password_column_count := (
  SELECT COUNT(*)
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'mc_user' AND column_name = 'password'
);
SET @identity_0131_cutover_batch_table_count := (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_cutover_batches'
);
SET @identity_0131_resume_batch_status := '';
SET @identity_0131_resume_batch_sql := IF(
  @identity_0131_cutover_batch_table_count = 1,
  'SELECT COALESCE((SELECT status FROM mochat_go_identity_cutover_batches WHERE request_id = @identity_0131_request_id), '''') INTO @identity_0131_resume_batch_status',
  'SELECT '''' INTO @identity_0131_resume_batch_status'
);
PREPARE identity_0131_resume_batch_stmt FROM @identity_0131_resume_batch_sql;
EXECUTE identity_0131_resume_batch_stmt;
DEALLOCATE PREPARE identity_0131_resume_batch_stmt;
SET @identity_0131_password_guard_sql := IF(
  @identity_0131_password_column_count = 1 OR @identity_0131_resume_batch_status IN ('started', 'completed', 'rolled_back', 'restored'),
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 legacy password cutover state is ambiguous'''
);
PREPARE identity_0131_password_guard_stmt FROM @identity_0131_password_guard_sql;
EXECUTE identity_0131_password_guard_stmt;
DEALLOCATE PREPARE identity_0131_password_guard_stmt;

SET @identity_0131_dashboard_permission_tables_count := (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name IN ('mochat_go_dashboard_permissions', 'mochat_go_dashboard_permission_resources')
);
SET @identity_0131_dashboard_permission_tables_guard_sql := IF(
  @identity_0131_dashboard_permission_tables_count = 2,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 dashboard permission catalog is missing'''
);
PREPARE identity_0131_dashboard_permission_tables_guard_stmt FROM @identity_0131_dashboard_permission_tables_guard_sql;
EXECUTE identity_0131_dashboard_permission_tables_guard_stmt;
DEALLOCATE PREPARE identity_0131_dashboard_permission_tables_guard_stmt;

SET @identity_0131_company_permission_count := (
  SELECT COUNT(*)
  FROM mochat_go_dashboard_permissions
  WHERE code = 'dashboard.company_setting.website'
);
SET @identity_0131_company_permission_guard_sql := IF(
  @identity_0131_company_permission_count = 1,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 company permission is missing'''
);
PREPARE identity_0131_company_permission_guard_stmt FROM @identity_0131_company_permission_guard_sql;
EXECUTE identity_0131_company_permission_guard_stmt;
DEALLOCATE PREPARE identity_0131_company_permission_guard_stmt;

-- First DDL begins only after all consistency guards above.
CREATE TABLE IF NOT EXISTS mochat_go_identity_cutover_batches (
  request_id varchar(128) NOT NULL,
  platform_tenant_id int(10) unsigned NOT NULL,
  status varchar(16) NOT NULL,
  script_checksum char(64) NOT NULL,
  preflight_status varchar(16) NOT NULL DEFAULT 'passed',
  created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (request_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='0131 cutover state';

CREATE TABLE IF NOT EXISTS mochat_go_identity_cutover_journal (
  id bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  request_id varchar(128) NOT NULL,
  entity_type varchar(48) NOT NULL,
  entity_id varchar(128) NOT NULL,
  before_json json DEFAULT NULL,
  created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uni_identity_cutover_journal_entity (request_id, entity_type, entity_id),
  KEY idx_identity_cutover_journal_request (request_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='0131 cutover ids only';

-- 0127 is already deployed and its old company resource rows are immutable.
-- Cutover remaps the existing permission to the single-company API and makes
-- the page superadmin-only without recreating the historical migration.
UPDATE mochat_go_dashboard_permissions
SET restriction = 'superadmin_only', superadmin_only = 1, updated_at = CURRENT_TIMESTAMP
WHERE code = 'dashboard.company_setting.website';

DELETE resource
FROM mochat_go_dashboard_permission_resources resource
INNER JOIN mochat_go_dashboard_permissions permission ON permission.id = resource.permission_id
WHERE permission.code = 'dashboard.company_setting.website'
  AND resource.path_pattern IN (
    '/dashboard/corp/select', '/dashboard/corp/bind', '/dashboard/corp/index',
    '/dashboard/corp/show', '/dashboard/corp/store', '/dashboard/corp/update'
  );

INSERT INTO mochat_go_dashboard_permission_resources
  (permission_id, resource_type, http_method, path_pattern, scope_required, status, version)
SELECT permission.id, 'api', resource_seed.http_method, resource_seed.path_pattern, 0, 1, 1
FROM mochat_go_dashboard_permissions permission
INNER JOIN (
  SELECT 'GET' AS http_method, '/dashboard/company/profile' AS path_pattern
  UNION ALL SELECT 'PUT', '/dashboard/company/profile'
  UNION ALL SELECT 'PUT', '/dashboard/company/wecom-credentials'
  UNION ALL SELECT 'PUT', '/dashboard/company/agent-credentials'
  UNION ALL SELECT 'PUT', '/dashboard/company/archive-credentials'
  UNION ALL SELECT 'POST', '/dashboard/company/verify'
  UNION ALL SELECT 'POST', '/dashboard/company/employee-sync'
  UNION ALL SELECT 'GET', '/dashboard/company/sync-status'
  UNION ALL SELECT 'GET', '/dashboard/company/audits'
) resource_seed
WHERE permission.code = 'dashboard.company_setting.website'
  AND NOT EXISTS (
    SELECT 1
    FROM mochat_go_dashboard_permission_resources existing
    WHERE existing.permission_id = permission.id
      AND existing.http_method = resource_seed.http_method
      AND existing.path_pattern = resource_seed.path_pattern
  );

INSERT INTO mochat_go_identity_cutover_batches (request_id, platform_tenant_id, status, script_checksum, preflight_status)
VALUES (@identity_0131_request_id, @identity_0131_platform_tenant_id, 'started', COALESCE(@identity_0131_script_checksum, ''), 'passed')
ON DUPLICATE KEY UPDATE
  platform_tenant_id = VALUES(platform_tenant_id),
  status = 'started',
  script_checksum = VALUES(script_checksum),
  preflight_status = 'passed',
  updated_at = CURRENT_TIMESTAMP;

INSERT INTO mochat_go_identity_cutover_journal (request_id, entity_type, entity_id, before_json)
SELECT @identity_0131_request_id, 'legacy_password', CAST(u.id AS CHAR), JSON_OBJECT('state', 'cutover')
FROM mc_user u
WHERE u.status = 1 AND u.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM mochat_go_identity_cutover_journal j
    WHERE j.request_id COLLATE utf8mb4_unicode_ci = @identity_0131_request_id COLLATE utf8mb4_unicode_ci
      AND j.entity_type COLLATE utf8mb4_unicode_ci = 'legacy_password' COLLATE utf8mb4_unicode_ci
      AND j.entity_id COLLATE utf8mb4_unicode_ci = CAST(u.id AS CHAR) COLLATE utf8mb4_unicode_ci
  );

INSERT INTO mochat_go_identity_cutover_journal (request_id, entity_type, entity_id, before_json)
SELECT @identity_0131_request_id, 'corp_credentials', CAST(c.id AS CHAR), JSON_OBJECT('state', 'cutover')
FROM mc_corp c
WHERE (
    COALESCE(c.employee_secret, '') <> '' OR COALESCE(c.contact_secret, '') <> ''
    OR COALESCE(c.token, '') <> '' OR COALESCE(c.encoding_aes_key, '') <> ''
    OR COALESCE(c.chat_secret, '') <> ''
  ) AND COALESCE(CAST(c.wecom_credentials_ciphertext AS CHAR), '') <> ''
  AND NOT EXISTS (
    SELECT 1 FROM mochat_go_identity_cutover_journal j
    WHERE j.request_id COLLATE utf8mb4_unicode_ci = @identity_0131_request_id COLLATE utf8mb4_unicode_ci
      AND j.entity_type COLLATE utf8mb4_unicode_ci = 'corp_credentials' COLLATE utf8mb4_unicode_ci
      AND j.entity_id COLLATE utf8mb4_unicode_ci = CAST(c.id AS CHAR) COLLATE utf8mb4_unicode_ci
  );

INSERT INTO mochat_go_identity_cutover_journal (request_id, entity_type, entity_id, before_json)
SELECT @identity_0131_request_id, 'agent_credentials', CAST(a.id AS CHAR), JSON_OBJECT('state', 'cutover')
FROM mc_work_agent a
WHERE COALESCE(a.wx_secret, '') <> '' AND COALESCE(CAST(a.wecom_credentials_ciphertext AS CHAR), '') <> ''
  AND NOT EXISTS (
    SELECT 1 FROM mochat_go_identity_cutover_journal j
    WHERE j.request_id COLLATE utf8mb4_unicode_ci = @identity_0131_request_id COLLATE utf8mb4_unicode_ci
      AND j.entity_type COLLATE utf8mb4_unicode_ci = 'agent_credentials' COLLATE utf8mb4_unicode_ci
      AND j.entity_id COLLATE utf8mb4_unicode_ci = CAST(a.id AS CHAR) COLLATE utf8mb4_unicode_ci
  );

UPDATE mc_corp c
INNER JOIN mochat_go_identity_cutover_journal j
  ON j.request_id COLLATE utf8mb4_unicode_ci = @identity_0131_request_id COLLATE utf8mb4_unicode_ci
 AND j.entity_type COLLATE utf8mb4_unicode_ci = 'corp_credentials' COLLATE utf8mb4_unicode_ci
 AND CAST(j.entity_id AS UNSIGNED) = c.id
SET c.employee_secret = '', c.contact_secret = '', c.token = '', c.encoding_aes_key = '', c.chat_secret = '';

UPDATE mc_work_agent a
INNER JOIN mochat_go_identity_cutover_journal j
  ON j.request_id COLLATE utf8mb4_unicode_ci = @identity_0131_request_id COLLATE utf8mb4_unicode_ci
 AND j.entity_type COLLATE utf8mb4_unicode_ci = 'agent_credentials' COLLATE utf8mb4_unicode_ci
 AND CAST(j.entity_id AS UNSIGNED) = a.id
SET a.wx_secret = '';

SET @identity_0131_drop_password_sql := IF(
  @identity_0131_password_column_count = 1,
  'ALTER TABLE mc_user DROP COLUMN password',
  'SELECT 1'
);
PREPARE identity_0131_drop_password_stmt FROM @identity_0131_drop_password_sql;
EXECUTE identity_0131_drop_password_stmt;
DEALLOCATE PREPARE identity_0131_drop_password_stmt;

UPDATE mochat_go_identity_cutover_batches
SET status = 'completed', preflight_status = 'passed'
WHERE request_id COLLATE utf8mb4_unicode_ci = @identity_0131_request_id COLLATE utf8mb4_unicode_ci;

INSERT INTO mochat_go_identity_migration_ledger (migration_name, request_id, phase, status, result_json)
SELECT '0131_identity_realms_single_corp_cutover', @identity_0131_request_id, 'cutover', 'success', JSON_OBJECT('scriptChecksum', COALESCE(@identity_0131_script_checksum, ''))
WHERE NOT EXISTS (
  SELECT 1 FROM mochat_go_identity_migration_ledger
  WHERE migration_name COLLATE utf8mb4_unicode_ci = '0131_identity_realms_single_corp_cutover' COLLATE utf8mb4_unicode_ci
    AND request_id COLLATE utf8mb4_unicode_ci = @identity_0131_request_id COLLATE utf8mb4_unicode_ci
);
