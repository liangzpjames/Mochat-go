-- 0131 rollback restores the legacy password column/password hashes and the
-- 0127 company permission/resource map.
-- Secret plaintext restoration is deliberately a separate, explicit
-- restore-legacy-credentials maintenance action using the encrypted copies.

SET @identity_0131_request_id := COALESCE(@identity_0131_request_id, '');
SET @identity_0131_platform_tenant_id := COALESCE(@identity_0131_platform_tenant_id, 0);

SET @identity_0131_down_request_guard_sql := IF(
  @identity_0131_request_id <> '' AND @identity_0131_platform_tenant_id > 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 rollback request and platform tenant are required'''
);
PREPARE identity_0131_down_request_guard_stmt FROM @identity_0131_down_request_guard_sql;
EXECUTE identity_0131_down_request_guard_stmt;
DEALLOCATE PREPARE identity_0131_down_request_guard_stmt;

SET @identity_0131_down_batch_table_count := (
  SELECT COUNT(*) FROM information_schema.tables
  WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_cutover_batches'
);
SET @identity_0131_down_journal_table_count := (
  SELECT COUNT(*) FROM information_schema.tables
  WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_cutover_journal'
);
SET @identity_0131_down_metadata_guard_sql := IF(
  @identity_0131_down_batch_table_count = 1 AND @identity_0131_down_journal_table_count = 1,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 rollback metadata is missing'''
);
PREPARE identity_0131_down_metadata_guard_stmt FROM @identity_0131_down_metadata_guard_sql;
EXECUTE identity_0131_down_metadata_guard_stmt;
DEALLOCATE PREPARE identity_0131_down_metadata_guard_stmt;

SET @identity_0131_down_permission_table_count := (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name IN ('mochat_go_dashboard_permissions', 'mochat_go_dashboard_permission_resources')
);
SET @identity_0131_down_permission_row_count := (
  SELECT COUNT(*)
  FROM mochat_go_dashboard_permissions
  WHERE code = 'dashboard.company_setting.website'
);
SET @identity_0131_down_permission_guard_sql := IF(
  @identity_0131_down_permission_table_count = 2 AND @identity_0131_down_permission_row_count = 1,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 rollback company permission catalog is missing'''
);
PREPARE identity_0131_down_permission_guard_stmt FROM @identity_0131_down_permission_guard_sql;
EXECUTE identity_0131_down_permission_guard_stmt;
DEALLOCATE PREPARE identity_0131_down_permission_guard_stmt;

SET @identity_0131_down_journal_count := (
  SELECT COUNT(*) FROM mochat_go_identity_cutover_journal WHERE request_id = @identity_0131_request_id
);
SET @identity_0131_down_missing_user_count := (
  SELECT COUNT(*)
  FROM mochat_go_identity_cutover_journal j
  LEFT JOIN mc_user u ON u.id = CAST(j.entity_id AS UNSIGNED)
  WHERE j.request_id = @identity_0131_request_id AND j.entity_type = 'legacy_password' AND u.id IS NULL
);
SET @identity_0131_down_missing_hash_count := (
  SELECT COUNT(*)
  FROM mochat_go_identity_cutover_journal j
  INNER JOIN mc_user u ON u.id = CAST(j.entity_id AS UNSIGNED)
  LEFT JOIN mochat_go_saas_admin_users s ON s.id = u.id AND u.tenant_id = @identity_0131_platform_tenant_id
  LEFT JOIN mochat_go_dashboard_identities d ON d.user_id = u.id AND u.tenant_id <> @identity_0131_platform_tenant_id
  WHERE j.request_id = @identity_0131_request_id AND j.entity_type = 'legacy_password'
    AND COALESCE(s.password_hash, d.password_hash, '') = ''
);
SET @identity_0131_down_preflight_guard_sql := IF(
  @identity_0131_down_journal_count > 0 AND @identity_0131_down_missing_user_count = 0 AND @identity_0131_down_missing_hash_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0131 rollback identity facts are incomplete'''
);
PREPARE identity_0131_down_preflight_guard_stmt FROM @identity_0131_down_preflight_guard_sql;
EXECUTE identity_0131_down_preflight_guard_stmt;
DEALLOCATE PREPARE identity_0131_down_preflight_guard_stmt;

SET @identity_0131_down_password_column_count := (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'mc_user' AND column_name = 'password'
);

-- First DDL begins only after rollback metadata and identity preflight.
SET @identity_0131_restore_password_sql := IF(
  @identity_0131_down_password_column_count = 0,
  'ALTER TABLE mc_user ADD COLUMN password varchar(255) NOT NULL DEFAULT '''' AFTER phone',
  'SELECT 1'
);
PREPARE identity_0131_restore_password_stmt FROM @identity_0131_restore_password_sql;
EXECUTE identity_0131_restore_password_stmt;
DEALLOCATE PREPARE identity_0131_restore_password_stmt;

UPDATE mc_user u
INNER JOIN mochat_go_identity_cutover_journal j
  ON j.request_id = @identity_0131_request_id AND j.entity_type = 'legacy_password' AND CAST(j.entity_id AS UNSIGNED) = u.id
LEFT JOIN mochat_go_saas_admin_users s
  ON s.id = u.id AND u.tenant_id = @identity_0131_platform_tenant_id
LEFT JOIN mochat_go_dashboard_identities d
  ON d.user_id = u.id AND u.tenant_id <> @identity_0131_platform_tenant_id
SET u.password = COALESCE(s.password_hash, d.password_hash, '');

DELETE resource
FROM mochat_go_dashboard_permission_resources resource
INNER JOIN mochat_go_dashboard_permissions permission ON permission.id = resource.permission_id
WHERE permission.code = 'dashboard.company_setting.website'
  AND resource.path_pattern IN (
    '/dashboard/company/profile',
    '/dashboard/company/wecom-credentials',
    '/dashboard/company/agent-credentials',
    '/dashboard/company/archive-credentials',
    '/dashboard/company/verify',
    '/dashboard/company/employee-sync',
    '/dashboard/company/sync-status',
    '/dashboard/company/audits'
  );

UPDATE mochat_go_dashboard_permissions
SET restriction = 'grantable', superadmin_only = 0
WHERE code = 'dashboard.company_setting.website';

INSERT INTO mochat_go_dashboard_permission_resources
  (permission_id, resource_type, http_method, path_pattern, scope_required, status, version)
SELECT permission.id, 'api', resource_seed.http_method, resource_seed.path_pattern, 0, 1, 1
FROM mochat_go_dashboard_permissions permission
INNER JOIN (
  SELECT 'GET' AS http_method, '/dashboard/corp/index' AS path_pattern
  UNION ALL SELECT 'GET', '/dashboard/corp/show'
  UNION ALL SELECT 'POST', '/dashboard/corp/store'
  UNION ALL SELECT 'PUT', '/dashboard/corp/update'
) resource_seed
WHERE permission.code = 'dashboard.company_setting.website'
  AND NOT EXISTS (
    SELECT 1
    FROM mochat_go_dashboard_permission_resources existing
    WHERE existing.permission_id = permission.id
      AND existing.http_method = resource_seed.http_method
      AND existing.path_pattern = resource_seed.path_pattern
  );

UPDATE mochat_go_identity_cutover_batches
SET status = 'rolled_back'
WHERE request_id = @identity_0131_request_id;

-- The migration ledger follows apply/down semantics. The independent cutover
-- batch and journal remain until the explicit credential-restore action has
-- completed, so rollback evidence is not confused with an applied version.
DELETE FROM mochat_go_identity_migration_ledger
WHERE migration_name = '0131_identity_realms_single_corp_cutover' AND request_id = @identity_0131_request_id;
