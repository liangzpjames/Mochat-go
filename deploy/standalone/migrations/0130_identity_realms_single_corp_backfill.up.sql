-- 0130 historical identity and single-corp backfill.
--
-- This script is intentionally not a generic migration runner shortcut. The
-- maintenance CLI verifies the signed mapping and credentials, writes the
-- normalized facts to the durable staging tables, and only then executes this
-- script. SQL validates those durable facts; it never trusts a caller-owned
-- session flag or a mapping boolean.
--
-- MariaDB DDL implicitly commits. Every consistency check below is therefore
-- before the first DDL, and the migration ledger is written only after every
-- DML phase, FK phase, and staged-batch transition succeeds.
-- A staging mapping is accepted only for a tenant whose corp_count > 1; a
-- one-corp tenant is selected directly and must not be mapped a second time.
-- Existing identity rows pass only a field-by-field equality check.

SET @identity_0130_stage_table_count := (
  SELECT COUNT(*)
  FROM (
    SELECT 'mochat_go_identity_migration_batches' AS table_name
    UNION ALL SELECT 'mochat_go_identity_migration_corp_map'
  ) expected
  INNER JOIN information_schema.tables actual
    ON actual.table_schema = DATABASE() AND actual.table_name = expected.table_name
);
SET @identity_0130_stage_table_guard_sql := IF(
  @identity_0130_stage_table_count = 2,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 validated staging tables are required'''
);
PREPARE identity_0130_stage_table_guard_stmt FROM @identity_0130_stage_table_guard_sql;
EXECUTE identity_0130_stage_table_guard_stmt;
DEALLOCATE PREPARE identity_0130_stage_table_guard_stmt;

SET @identity_0130_stage_column_count := (
  SELECT COUNT(*)
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'mochat_go_identity_migration_batches'
    AND column_name IN ('request_id', 'platform_tenant_id', 'status', 'mapping_digest', 'script_checksum', 'preflight_status', 'credential_status', 'actor_inventory_status', 'migration_source')
)
+ (
  SELECT COUNT(*)
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'mochat_go_identity_migration_corp_map'
    AND column_name IN ('request_id', 'tenant_id', 'corp_id', 'status', 'migration_source')
);
SET @identity_0130_stage_total_column_count := (
  SELECT COUNT(*)
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name IN ('mochat_go_identity_migration_batches', 'mochat_go_identity_migration_corp_map')
);
SET @identity_0130_stage_column_guard_sql := IF(
  @identity_0130_stage_column_count = 14 AND @identity_0130_stage_total_column_count = 14,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 staging schema is not the approved contract'''
);
PREPARE identity_0130_stage_column_guard_stmt FROM @identity_0130_stage_column_guard_sql;
EXECUTE identity_0130_stage_column_guard_stmt;
DEALLOCATE PREPARE identity_0130_stage_column_guard_stmt;

SET @identity_0130_stage_type_mismatch_count := (
  SELECT COUNT(*)
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND (
      (table_name = 'mochat_go_identity_migration_batches' AND column_name = 'request_id' AND column_type <> 'varchar(128)')
      OR (table_name = 'mochat_go_identity_migration_batches' AND column_name = 'platform_tenant_id' AND column_type <> 'int(10) unsigned')
      OR (table_name = 'mochat_go_identity_migration_batches' AND column_name = 'status' AND column_type <> 'varchar(16)')
      OR (table_name = 'mochat_go_identity_migration_batches' AND column_name = 'mapping_digest' AND column_type <> 'char(64)')
      OR (table_name = 'mochat_go_identity_migration_batches' AND column_name = 'script_checksum' AND column_type <> 'char(64)')
      OR (table_name = 'mochat_go_identity_migration_batches' AND column_name = 'preflight_status' AND column_type <> 'varchar(16)')
      OR (table_name = 'mochat_go_identity_migration_batches' AND column_name = 'credential_status' AND column_type <> 'varchar(16)')
      OR (table_name = 'mochat_go_identity_migration_batches' AND column_name = 'actor_inventory_status' AND column_type <> 'varchar(16)')
      OR (table_name = 'mochat_go_identity_migration_batches' AND column_name = 'migration_source' AND column_type <> 'varchar(96)')
      OR (table_name = 'mochat_go_identity_migration_corp_map' AND column_name = 'request_id' AND column_type <> 'varchar(128)')
      OR (table_name = 'mochat_go_identity_migration_corp_map' AND column_name IN ('tenant_id', 'corp_id') AND column_type <> 'int(10) unsigned')
      OR (table_name = 'mochat_go_identity_migration_corp_map' AND column_name = 'status' AND column_type <> 'varchar(16)')
      OR (table_name = 'mochat_go_identity_migration_corp_map' AND column_name = 'migration_source' AND column_type <> 'varchar(96)')
    )
);
SET @identity_0130_stage_type_guard_sql := IF(
  @identity_0130_stage_type_mismatch_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 staging column_type contract mismatch'''
);
PREPARE identity_0130_stage_type_guard_stmt FROM @identity_0130_stage_type_guard_sql;
EXECUTE identity_0130_stage_type_guard_stmt;
DEALLOCATE PREPARE identity_0130_stage_type_guard_stmt;

SET @identity_0130_stage_index_count := (
  SELECT COUNT(*)
  FROM (
    SELECT table_name, index_name, GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') AS signature
    FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name IN ('mochat_go_identity_migration_batches', 'mochat_go_identity_migration_corp_map')
      AND index_name IN ('PRIMARY', 'uni_task8_stage_corp')
    GROUP BY table_name, index_name
  ) indexes
  WHERE (table_name = 'mochat_go_identity_migration_batches' AND index_name = 'PRIMARY' AND signature = 'request_id')
     OR (table_name = 'mochat_go_identity_migration_corp_map' AND index_name = 'PRIMARY' AND signature = 'request_id,tenant_id')
     OR (table_name = 'mochat_go_identity_migration_corp_map' AND index_name = 'uni_task8_stage_corp' AND signature = 'request_id,corp_id')
);
SET @identity_0130_stage_index_name_count := (
  SELECT COUNT(DISTINCT CONCAT(table_name, '.', index_name))
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name IN ('mochat_go_identity_migration_batches', 'mochat_go_identity_migration_corp_map')
    AND index_name IN ('PRIMARY', 'uni_task8_stage_corp')
);
SET @identity_0130_stage_index_guard_sql := IF(
  @identity_0130_stage_index_count = 3 AND @identity_0130_stage_index_name_count = 3,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 staging unique key contract mismatch'''
);
PREPARE identity_0130_stage_index_guard_stmt FROM @identity_0130_stage_index_guard_sql;
EXECUTE identity_0130_stage_index_guard_stmt;
DEALLOCATE PREPARE identity_0130_stage_index_guard_stmt;

SET @identity_0130_validated_batch_count := (
  SELECT COUNT(*)
  FROM mochat_go_identity_migration_batches
  WHERE status = 'validated'
    AND preflight_status = 'passed'
    AND credential_status = 'verified'
    AND script_checksum <> ''
    AND CHAR_LENGTH(script_checksum) = 64
);
SET @identity_0130_validated_batch_guard_sql := IF(
  @identity_0130_validated_batch_count = 1,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 exactly one validated staging batch is required'''
);
PREPARE identity_0130_validated_batch_guard_stmt FROM @identity_0130_validated_batch_guard_sql;
EXECUTE identity_0130_validated_batch_guard_stmt;
DEALLOCATE PREPARE identity_0130_validated_batch_guard_stmt;

SET @identity_0130_request_id := (
  SELECT request_id
  FROM mochat_go_identity_migration_batches
  WHERE status = 'validated'
    AND preflight_status = 'passed'
    AND credential_status = 'verified'
  ORDER BY request_id
  LIMIT 1
);
SET @identity_0130_platform_tenant_id := (
  SELECT platform_tenant_id
  FROM mochat_go_identity_migration_batches
  WHERE request_id = @identity_0130_request_id
);
SET @identity_0130_script_checksum := (
  SELECT script_checksum
  FROM mochat_go_identity_migration_batches
  WHERE request_id = @identity_0130_request_id
);
SET @identity_0130_script_checksum_guard_sql := IF(
  @identity_0130_script_checksum IS NOT NULL
    AND CHAR_LENGTH(@identity_0130_script_checksum) = 64
    AND @identity_0130_script_checksum REGEXP '^[0-9A-Fa-f]{64}$',
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 validated script checksum is required'''
);
PREPARE identity_0130_script_checksum_guard_stmt FROM @identity_0130_script_checksum_guard_sql;
EXECUTE identity_0130_script_checksum_guard_stmt;
DEALLOCATE PREPARE identity_0130_script_checksum_guard_stmt;

SET @identity_0130_actor_inventory_count := (
  SELECT COUNT(*)
  FROM mochat_go_identity_migration_batches
  WHERE request_id = @identity_0130_request_id
    AND actor_inventory_status = 'verified'
);
SET @identity_0130_actor_inventory_guard_sql := IF(
  @identity_0130_actor_inventory_count = 1,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 identity actor inventory is not verified or contains an unknown actor column'''
);
PREPARE identity_0130_actor_inventory_guard_stmt FROM @identity_0130_actor_inventory_guard_sql;
EXECUTE identity_0130_actor_inventory_guard_stmt;
DEALLOCATE PREPARE identity_0130_actor_inventory_guard_stmt;

SET @identity_0130_platform_tenant_count := (
  SELECT COUNT(*)
  FROM mc_tenant
  WHERE id = @identity_0130_platform_tenant_id
    AND status = 1
    AND deleted_at IS NULL
);
SET @identity_0130_platform_tenant_guard_sql := IF(
  @identity_0130_platform_tenant_id > 0 AND @identity_0130_platform_tenant_count = 1,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 explicit platform tenant is invalid'''
);
PREPARE identity_0130_platform_tenant_guard_stmt FROM @identity_0130_platform_tenant_guard_sql;
EXECUTE identity_0130_platform_tenant_guard_stmt;
DEALLOCATE PREPARE identity_0130_platform_tenant_guard_stmt;

SET @identity_0130_mapping_ownership_count := (
  SELECT COUNT(*)
  FROM mochat_go_identity_migration_corp_map m
  LEFT JOIN mc_tenant t ON t.id = m.tenant_id AND t.status = 1 AND t.deleted_at IS NULL
  LEFT JOIN mc_corp selected_corp
    ON selected_corp.id = m.corp_id
   AND selected_corp.tenant_id = m.tenant_id
   AND selected_corp.deleted_at IS NULL
  LEFT JOIN (
    SELECT tenant_id, COUNT(*) AS corp_count
    FROM mc_corp
    WHERE deleted_at IS NULL
    GROUP BY tenant_id
  ) ownership ON ownership.tenant_id = m.tenant_id
  WHERE m.request_id = @identity_0130_request_id
    AND m.status = 'validated'
    AND (t.id IS NULL OR selected_corp.id IS NULL OR ownership.corp_count <= 1)
);
SET @identity_0130_mapping_ownership_guard_sql := IF(
  @identity_0130_mapping_ownership_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 signed mapping does not prove exact tenant corp ownership'''
);
PREPARE identity_0130_mapping_ownership_guard_stmt FROM @identity_0130_mapping_ownership_guard_sql;
EXECUTE identity_0130_mapping_ownership_guard_stmt;
DEALLOCATE PREPARE identity_0130_mapping_ownership_guard_stmt;

SET @identity_0130_duplicate_mapping_count := (
  SELECT COUNT(*)
  FROM (
    SELECT tenant_id, COUNT(*) AS mapping_count
    FROM mochat_go_identity_migration_corp_map
    WHERE request_id = @identity_0130_request_id AND status = 'validated'
    GROUP BY tenant_id
    HAVING COUNT(*) <> 1
  ) duplicate_mapping
);
SET @identity_0130_duplicate_mapping_guard_sql := IF(
  @identity_0130_duplicate_mapping_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 staging mapping must contain exactly one row per multi-corp tenant'''
);
PREPARE identity_0130_duplicate_mapping_guard_stmt FROM @identity_0130_duplicate_mapping_guard_sql;
EXECUTE identity_0130_duplicate_mapping_guard_stmt;
DEALLOCATE PREPARE identity_0130_duplicate_mapping_guard_stmt;

SET @identity_0130_multi_corp_count := (
  SELECT COUNT(*)
  FROM (
    SELECT c.tenant_id
    FROM mc_corp c
    INNER JOIN mc_tenant t ON t.id = c.tenant_id AND t.status = 1 AND t.deleted_at IS NULL
    WHERE c.deleted_at IS NULL
    GROUP BY c.tenant_id
    HAVING COUNT(*) > 1
  ) multiple_corp
  LEFT JOIN mochat_go_identity_migration_corp_map m
    ON m.request_id = @identity_0130_request_id
   AND m.tenant_id = multiple_corp.tenant_id
   AND m.status = 'validated'
  WHERE m.tenant_id IS NULL
);
SET @identity_0130_multi_corp_guard_sql := IF(
  @identity_0130_multi_corp_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 multiple valid corp requires signed mapping'''
);
PREPARE identity_0130_multi_corp_guard_stmt FROM @identity_0130_multi_corp_guard_sql;
EXECUTE identity_0130_multi_corp_guard_stmt;
DEALLOCATE PREPARE identity_0130_multi_corp_guard_stmt;

SET @identity_0130_invalid_active_phone_count := (
  SELECT COUNT(*)
  FROM mc_user u
  WHERE u.deleted_at IS NULL
    AND u.tenant_id <> @identity_0130_platform_tenant_id
    AND u.status = 1
    AND TRIM(u.phone) NOT REGEXP '^1[3-9][0-9]{9}$'
);
SET @identity_0130_invalid_active_phone_guard_sql := IF(
  @identity_0130_invalid_active_phone_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 active dashboard login identifier is invalid'''
);
PREPARE identity_0130_invalid_active_phone_guard_stmt FROM @identity_0130_invalid_active_phone_guard_sql;
EXECUTE identity_0130_invalid_active_phone_guard_stmt;
DEALLOCATE PREPARE identity_0130_invalid_active_phone_guard_stmt;

SET @identity_0130_inactive_business_tenant_count := (
  SELECT COUNT(*)
  FROM mc_user u
  LEFT JOIN mc_tenant t ON t.id = u.tenant_id
  WHERE u.deleted_at IS NULL
    AND u.tenant_id <> @identity_0130_platform_tenant_id
    AND (t.id IS NULL OR t.status <> 1 OR t.deleted_at IS NOT NULL)
);
SET @identity_0130_inactive_business_tenant_guard_sql := IF(
  @identity_0130_inactive_business_tenant_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 business tenant is missing or inactive'''
);
PREPARE identity_0130_inactive_business_tenant_guard_stmt FROM @identity_0130_inactive_business_tenant_guard_sql;
EXECUTE identity_0130_inactive_business_tenant_guard_stmt;
DEALLOCATE PREPARE identity_0130_inactive_business_tenant_guard_stmt;

SET @identity_0130_inactive_business_superadmin_count := (
  SELECT COUNT(*)
  FROM mc_user u
  LEFT JOIN mc_tenant t ON t.id = u.tenant_id
  WHERE u.tenant_id <> @identity_0130_platform_tenant_id
    AND u.isSuperAdmin = 1
    AND (u.status <> 1 OR u.deleted_at IS NOT NULL OR t.id IS NULL OR t.status <> 1 OR t.deleted_at IS NOT NULL)
);
SET @identity_0130_inactive_business_superadmin_guard_sql := IF(
  @identity_0130_inactive_business_superadmin_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 business superadmin is inactive'''
);
PREPARE identity_0130_inactive_business_superadmin_guard_stmt FROM @identity_0130_inactive_business_superadmin_guard_sql;
EXECUTE identity_0130_inactive_business_superadmin_guard_stmt;
DEALLOCATE PREPARE identity_0130_inactive_business_superadmin_guard_stmt;

SET @identity_0130_cross_tenant_relation_count := (
  SELECT COUNT(*) FROM mc_rbac_user_role ur
  INNER JOIN mc_user u ON u.id = CAST(ur.user_id AS UNSIGNED)
  INNER JOIN mc_rbac_role r ON r.id = ur.role_id
  WHERE ur.deleted_at IS NULL AND u.tenant_id <> r.tenant_id
)
+ (SELECT COUNT(*) FROM mochat_go_dashboard_user_roles dr
    INNER JOIN mc_user u ON u.id = dr.user_id
    WHERE u.tenant_id <> dr.tenant_id)
+ (SELECT COUNT(*) FROM mochat_go_dashboard_user_permissions dp
    INNER JOIN mc_user u ON u.id = dp.user_id
    WHERE u.tenant_id <> dp.tenant_id);
SET @identity_0130_cross_tenant_relation_guard_sql := IF(
  @identity_0130_cross_tenant_relation_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 cross-tenant historical relation'''
);
PREPARE identity_0130_cross_tenant_relation_guard_stmt FROM @identity_0130_cross_tenant_relation_guard_sql;
EXECUTE identity_0130_cross_tenant_relation_guard_stmt;
DEALLOCATE PREPARE identity_0130_cross_tenant_relation_guard_stmt;

SET @identity_0130_duplicate_login_count := (
  SELECT COUNT(*)
  FROM mc_user u
  INNER JOIN mc_user duplicate_user
    ON TRIM(duplicate_user.phone) = TRIM(u.phone)
   AND duplicate_user.id <> u.id
   AND duplicate_user.deleted_at IS NULL
  WHERE u.deleted_at IS NULL
    AND u.tenant_id <> @identity_0130_platform_tenant_id
    AND duplicate_user.tenant_id <> @identity_0130_platform_tenant_id
    AND TRIM(u.phone) <> ''
);
SET @identity_0130_duplicate_login_guard_sql := IF(
  @identity_0130_duplicate_login_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 duplicate dashboard login identifier'''
);
PREPARE identity_0130_duplicate_login_guard_stmt FROM @identity_0130_duplicate_login_guard_sql;
EXECUTE identity_0130_duplicate_login_guard_stmt;
DEALLOCATE PREPARE identity_0130_duplicate_login_guard_stmt;

SET @identity_0130_negative_tenant_count := (
  SELECT COUNT(*) FROM mc_user
  WHERE CAST(tenant_id AS DECIMAL(20,0)) < 0 OR CAST(tenant_id AS DECIMAL(20,0)) > 4294967295
)
+ (SELECT COUNT(*) FROM mc_corp
    WHERE CAST(tenant_id AS DECIMAL(20,0)) < 0 OR CAST(tenant_id AS DECIMAL(20,0)) > 4294967295)
+ (SELECT COUNT(*) FROM mc_rbac_role
    WHERE CAST(tenant_id AS DECIMAL(20,0)) < 0 OR CAST(tenant_id AS DECIMAL(20,0)) > 4294967295)
+ (SELECT COUNT(*) FROM mochat_go_identity_migration_corp_map
    WHERE request_id = @identity_0130_request_id
      AND (CAST(tenant_id AS DECIMAL(20,0)) < 0 OR CAST(tenant_id AS DECIMAL(20,0)) > 4294967295));
SET @identity_0130_negative_tenant_guard_sql := IF(
  @identity_0130_negative_tenant_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 negative tenant id'''
);
PREPARE identity_0130_negative_tenant_guard_stmt FROM @identity_0130_negative_tenant_guard_sql;
EXECUTE identity_0130_negative_tenant_guard_stmt;
DEALLOCATE PREPARE identity_0130_negative_tenant_guard_stmt;

SET @identity_0130_dangling_tenant_count := (
  SELECT COUNT(*)
  FROM mc_user u
  LEFT JOIN mc_tenant t ON t.id = u.tenant_id
  WHERE t.id IS NULL
)
+ (SELECT COUNT(*)
   FROM mc_corp c
   LEFT JOIN mc_tenant t ON t.id = c.tenant_id
   WHERE t.id IS NULL)
+ (SELECT COUNT(*)
   FROM mc_rbac_role r
   LEFT JOIN mc_tenant t ON t.id = r.tenant_id
   WHERE t.id IS NULL);
SET @identity_0130_dangling_tenant_guard_sql := IF(
  @identity_0130_dangling_tenant_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 dangling tenant reference'''
);
PREPARE identity_0130_dangling_tenant_guard_stmt FROM @identity_0130_dangling_tenant_guard_sql;
EXECUTE identity_0130_dangling_tenant_guard_stmt;
DEALLOCATE PREPARE identity_0130_dangling_tenant_guard_stmt;

SET @identity_0130_dangling_corp_count := (
  SELECT COUNT(*)
  FROM mc_corp c
  LEFT JOIN mc_tenant t ON t.id = c.tenant_id
  WHERE t.id IS NULL
);
SET @identity_0130_dangling_corp_guard_sql := IF(
  @identity_0130_dangling_corp_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 dangling corp reference'''
);
PREPARE identity_0130_dangling_corp_guard_stmt FROM @identity_0130_dangling_corp_guard_sql;
EXECUTE identity_0130_dangling_corp_guard_stmt;
DEALLOCATE PREPARE identity_0130_dangling_corp_guard_stmt;

-- A historical SaaS actor is explainable only when it is an existing SaaS
-- identity or a live legacy user in the explicitly staged platform tenant.
-- Business-tenant superadmins are Dashboard users, never SaaS actors.
SET @identity_0130_unmapped_actor_count := (
  SELECT COUNT(*)
  FROM (
    SELECT user_id AS actor_id FROM mochat_go_saas_admin_user_access WHERE user_id > 0
    UNION SELECT user_id FROM mochat_go_saas_admin_user_roles WHERE user_id > 0
    UNION SELECT assigned_by FROM mochat_go_saas_admin_user_roles WHERE assigned_by > 0
    UNION SELECT created_by FROM mochat_go_saas_admin_roles WHERE created_by > 0
    UNION SELECT updated_by FROM mochat_go_saas_admin_roles WHERE updated_by > 0
    UNION SELECT actor_user_id FROM mochat_go_saas_admin_operation_logs WHERE actor_user_id > 0
    UNION SELECT requester_user_id FROM mochat_go_saas_admin_approvals WHERE requester_user_id > 0
    UNION SELECT reviewer_user_id FROM mochat_go_saas_admin_approvals WHERE reviewer_user_id > 0
    UNION SELECT execution_user_id FROM mochat_go_saas_admin_approvals WHERE execution_user_id > 0
    UNION SELECT actor_user_id FROM mochat_go_saas_admin_approval_events WHERE actor_user_id > 0
    UNION SELECT reviewer_user_id FROM mochat_go_saas_admin_approval_decisions WHERE reviewer_user_id > 0
    UNION SELECT delegated_from_user_id FROM mochat_go_saas_admin_approval_decisions WHERE delegated_from_user_id > 0
    UNION SELECT delegator_user_id FROM mochat_go_saas_admin_approval_delegations WHERE delegator_user_id > 0
    UNION SELECT delegate_user_id FROM mochat_go_saas_admin_approval_delegations WHERE delegate_user_id > 0
    UNION SELECT created_by FROM mochat_go_saas_admin_approval_delegations WHERE created_by > 0
    UNION SELECT updated_by FROM mochat_go_saas_admin_approval_delegations WHERE updated_by > 0
    UNION SELECT updated_by FROM mochat_go_saas_admin_approval_policies WHERE updated_by > 0
    UNION SELECT created_by_saas_user_id FROM mochat_go_saas_idempotency_receipts WHERE created_by_saas_user_id > 0
    UNION SELECT created_by_saas_user_id FROM mochat_go_dashboard_identity_activations WHERE created_by_saas_user_id > 0
  ) actors
  LEFT JOIN mochat_go_saas_admin_users saas_user ON saas_user.id = actors.actor_id
  LEFT JOIN mc_user legacy_user ON legacy_user.id = actors.actor_id
  WHERE saas_user.id IS NULL
    AND (
      legacy_user.id IS NULL
      OR legacy_user.tenant_id <> @identity_0130_platform_tenant_id
      OR legacy_user.status <> 1
      OR legacy_user.deleted_at IS NOT NULL
    )
);
SET @identity_0130_unmapped_actor_guard_sql := IF(
  @identity_0130_unmapped_actor_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 unmapped SaaS actor or business tenant superadmin'''
);
PREPARE identity_0130_unmapped_actor_guard_stmt FROM @identity_0130_unmapped_actor_guard_sql;
EXECUTE identity_0130_unmapped_actor_guard_stmt;
DEALLOCATE PREPARE identity_0130_unmapped_actor_guard_stmt;

-- Existing rows are accepted only if every authentication fact is identical.
-- The legacy platform actor keeps its original numeric id; no mapping table or
-- historical-reference rewrite is introduced.
SET @identity_0130_identity_conflict_count := (
  SELECT COUNT(*)
  FROM mc_user u
  LEFT JOIN mochat_go_saas_admin_users s ON s.id = u.id
  LEFT JOIN mochat_go_saas_admin_users login_conflict ON login_conflict.login_name = CONCAT('legacy-mc-user-', u.id)
  LEFT JOIN mochat_go_saas_admin_users phone_conflict
    ON phone_conflict.phone = NULLIF(TRIM(u.phone), '')
   AND phone_conflict.id <> u.id
  WHERE u.tenant_id = @identity_0130_platform_tenant_id
    AND u.deleted_at IS NULL
    AND (u.isSuperAdmin = 1 OR EXISTS (SELECT 1 FROM mochat_go_saas_admin_user_access a WHERE a.user_id = u.id) OR EXISTS (SELECT 1 FROM mochat_go_saas_admin_user_roles r WHERE r.user_id = u.id))
    AND (
      (s.id IS NOT NULL AND (
        s.login_name <> CONCAT('legacy-mc-user-', u.id)
        OR COALESCE(s.phone, '') <> COALESCE(NULLIF(TRIM(u.phone), ''), '')
        OR s.password_hash <> u.password
        OR s.name <> u.name
        OR s.status <> IF(u.status = 1, 1, 2)
        OR s.must_rotate_password <> 1
        OR s.auth_version <> 1
        OR s.mfa_required <> 1
      ))
      OR (login_conflict.id IS NOT NULL AND login_conflict.id <> u.id)
      OR phone_conflict.id IS NOT NULL
    )
);
SET @identity_0130_identity_conflict_guard_sql := IF(
  @identity_0130_identity_conflict_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 SaaS platform identity phone/login conflict'''
);
PREPARE identity_0130_identity_conflict_guard_stmt FROM @identity_0130_identity_conflict_guard_sql;
EXECUTE identity_0130_identity_conflict_guard_stmt;
DEALLOCATE PREPARE identity_0130_identity_conflict_guard_stmt;

SET @identity_0130_dashboard_identity_conflict_count := (
  SELECT COUNT(*)
  FROM mc_user u
  LEFT JOIN mochat_go_dashboard_identities same_user ON same_user.user_id = u.id
  LEFT JOIN mochat_go_dashboard_identities login_conflict ON login_conflict.login_identifier = TRIM(u.phone)
  WHERE u.deleted_at IS NULL
    AND u.tenant_id <> @identity_0130_platform_tenant_id
    AND TRIM(u.phone) <> ''
    AND (
      (same_user.user_id IS NOT NULL AND (
        same_user.login_identifier <> TRIM(u.phone)
        OR same_user.password_hash <> u.password
        OR same_user.status <> IF(u.status = 1, 1, 2)
      ))
      OR (login_conflict.user_id IS NOT NULL AND login_conflict.user_id <> u.id)
    )
);
SET @identity_0130_dashboard_identity_conflict_guard_sql := IF(
  @identity_0130_dashboard_identity_conflict_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 Dashboard identity conflict'''
);
PREPARE identity_0130_dashboard_identity_conflict_guard_stmt FROM @identity_0130_dashboard_identity_conflict_guard_sql;
EXECUTE identity_0130_dashboard_identity_conflict_guard_stmt;
DEALLOCATE PREPARE identity_0130_dashboard_identity_conflict_guard_stmt;

SET @identity_0130_platform_duplicate_phone_count := (
  SELECT COUNT(*)
  FROM mc_user u
  INNER JOIN mc_user duplicate_user
    ON duplicate_user.tenant_id = u.tenant_id
   AND duplicate_user.id <> u.id
   AND duplicate_user.deleted_at IS NULL
   AND TRIM(duplicate_user.phone) = TRIM(u.phone)
  WHERE u.tenant_id = @identity_0130_platform_tenant_id
    AND u.deleted_at IS NULL
    AND TRIM(u.phone) <> ''
   AND (
     u.isSuperAdmin = 1
     OR duplicate_user.isSuperAdmin = 1
     OR EXISTS (SELECT 1 FROM mochat_go_saas_admin_user_access a WHERE a.user_id = u.id)
     OR EXISTS (SELECT 1 FROM mochat_go_saas_admin_user_access a WHERE a.user_id = duplicate_user.id)
     OR EXISTS (SELECT 1 FROM mochat_go_saas_admin_user_roles r WHERE r.user_id = u.id)
     OR EXISTS (SELECT 1 FROM mochat_go_saas_admin_user_roles r WHERE r.user_id = duplicate_user.id)
   )
);
SET @identity_0130_platform_duplicate_phone_guard_sql := IF(
  @identity_0130_platform_duplicate_phone_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 SaaS platform identity phone/login conflict'''
);
PREPARE identity_0130_platform_duplicate_phone_guard_stmt FROM @identity_0130_platform_duplicate_phone_guard_sql;
EXECUTE identity_0130_platform_duplicate_phone_guard_stmt;
DEALLOCATE PREPARE identity_0130_platform_duplicate_phone_guard_stmt;

-- Existing bindings must match the one authoritative selection exactly. A
-- tenant with zero corp has no pre-existing binding; a multi-corp tenant must
-- be present in the validated staging map. No minimum-id guess is allowed.
SET @identity_0130_binding_conflict_count := (
  SELECT COUNT(*)
  FROM mochat_go_tenant_corp_bindings b
  LEFT JOIN (
    SELECT c.tenant_id, MAX(c.id) AS corp_id
    FROM mc_corp c
    INNER JOIN mc_tenant t ON t.id = c.tenant_id AND t.status = 1 AND t.deleted_at IS NULL
    WHERE c.deleted_at IS NULL
    GROUP BY c.tenant_id
    HAVING COUNT(*) = 1
    UNION ALL
    SELECT tenant_id, corp_id
    FROM mochat_go_identity_migration_corp_map
    WHERE request_id = @identity_0130_request_id AND status = 'validated'
  ) selected ON selected.tenant_id = b.tenant_id
  WHERE selected.tenant_id IS NULL
     OR selected.corp_id <> b.corp_id
     OR b.status <> 1
     OR b.version <> 1
     OR COALESCE(b.verified_wx_corpid, '') <> ''
     OR b.verified_corp_name <> ''
);
SET @identity_0130_binding_conflict_guard_sql := IF(
  @identity_0130_binding_conflict_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 tenant corp binding conflict'''
);
PREPARE identity_0130_binding_conflict_guard_stmt FROM @identity_0130_binding_conflict_guard_sql;
EXECUTE identity_0130_binding_conflict_guard_stmt;
DEALLOCATE PREPARE identity_0130_binding_conflict_guard_stmt;

SET @identity_0130_unknown_actor_fk_count := (
  SELECT COUNT(*)
  FROM information_schema.key_column_usage
  WHERE constraint_schema = DATABASE()
    AND table_name IN ('mochat_go_saas_admin_user_access', 'mochat_go_saas_admin_user_roles')
    AND referenced_table_name IS NOT NULL
    AND constraint_name NOT IN ('fk_saas_admin_user_access_identity', 'fk_saas_admin_user_roles_identity')
);
SET @identity_0130_unknown_actor_fk_guard_sql := IF(
  @identity_0130_unknown_actor_fk_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 unknown SaaS actor dependency'''
);
PREPARE identity_0130_unknown_actor_fk_guard_stmt FROM @identity_0130_unknown_actor_fk_guard_sql;
EXECUTE identity_0130_unknown_actor_fk_guard_stmt;
DEALLOCATE PREPARE identity_0130_unknown_actor_fk_guard_stmt;

-- Actual decryption is performed by the Go credential manager before staging.
-- The SQL side only accepts the durable preflight/credential facts above and
-- never treats a key id alone as proof of decryptability.
-- An unreadable credential therefore stops the preflight before this first DDL.

CREATE TABLE IF NOT EXISTS mochat_go_identity_migration_ledger (
  id bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  migration_name varchar(96) NOT NULL,
  request_id varchar(128) NOT NULL,
  phase varchar(32) NOT NULL,
  status varchar(16) NOT NULL,
  result_json json DEFAULT NULL,
  created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uni_identity_migration_ledger_request (migration_name, request_id),
  KEY idx_identity_migration_ledger_status (migration_name, phase, status, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='0130 migration ledger success only';

CREATE TABLE IF NOT EXISTS mochat_go_identity_migration_journal (
  id bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  request_id varchar(128) NOT NULL,
  entity_type varchar(48) NOT NULL,
  entity_id varchar(128) NOT NULL,
  before_json json DEFAULT NULL,
  created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uni_identity_migration_journal_entity (request_id, entity_type, entity_id),
  KEY idx_identity_migration_journal_request (request_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='0130 append-only rollback journal';

START TRANSACTION;

-- Preserve the legacy platform actor's numeric id. Existing identical rows are
-- left untouched; only absent rows are inserted, and no conflict is swallowed.
-- Every legacy actor id remains the SaaS identity id; historical references are
-- checked against that same numeric id and are never rewritten.
INSERT INTO mochat_go_identity_migration_journal (request_id, entity_type, entity_id, before_json)
SELECT @identity_0130_request_id, 'saas_identity', CAST(u.id AS CHAR), NULL
FROM mc_user u
WHERE u.tenant_id = @identity_0130_platform_tenant_id
  AND u.deleted_at IS NULL
  AND (u.isSuperAdmin = 1 OR EXISTS (SELECT 1 FROM mochat_go_saas_admin_user_access a WHERE a.user_id = u.id) OR EXISTS (SELECT 1 FROM mochat_go_saas_admin_user_roles r WHERE r.user_id = u.id))
  AND NOT EXISTS (SELECT 1 FROM mochat_go_saas_admin_users existing WHERE existing.id = u.id)
  AND NOT EXISTS (
    SELECT 1 FROM mochat_go_identity_migration_journal j
    WHERE j.request_id = @identity_0130_request_id AND j.entity_type = 'saas_identity' AND j.entity_id = CAST(u.id AS CHAR)
  );

INSERT INTO mochat_go_saas_admin_users
  (id, login_name, phone, password_hash, name, status, must_rotate_password, auth_version, mfa_required)
SELECT u.id,
       CONCAT('legacy-mc-user-', u.id),
       NULLIF(TRIM(u.phone), ''),
       u.password,
       u.name,
       IF(u.status = 1 AND u.deleted_at IS NULL, 1, 2),
       1, 1, 1
FROM mc_user u
WHERE u.tenant_id = @identity_0130_platform_tenant_id
  AND u.deleted_at IS NULL
  AND (u.isSuperAdmin = 1 OR EXISTS (SELECT 1 FROM mochat_go_saas_admin_user_access a WHERE a.user_id = u.id) OR EXISTS (SELECT 1 FROM mochat_go_saas_admin_user_roles r WHERE r.user_id = u.id))
  AND NOT EXISTS (SELECT 1 FROM mochat_go_saas_admin_users existing WHERE existing.id = u.id);

SET @identity_0130_saas_actor_verify_count := (
  SELECT COUNT(*)
  FROM mc_user u
  INNER JOIN mochat_go_saas_admin_users s ON s.id = u.id
  WHERE u.tenant_id = @identity_0130_platform_tenant_id
    AND u.deleted_at IS NULL
    AND (u.isSuperAdmin = 1 OR EXISTS (SELECT 1 FROM mochat_go_saas_admin_user_access a WHERE a.user_id = u.id) OR EXISTS (SELECT 1 FROM mochat_go_saas_admin_user_roles r WHERE r.user_id = u.id))
    AND (s.login_name <> CONCAT('legacy-mc-user-', u.id) OR COALESCE(s.phone, '') <> COALESCE(NULLIF(TRIM(u.phone), ''), '') OR s.password_hash <> u.password OR s.name <> u.name OR s.status <> IF(u.status = 1, 1, 2) OR s.must_rotate_password <> 1 OR s.auth_version <> 1 OR s.mfa_required <> 1)
);
SET @identity_0130_saas_actor_verify_guard_sql := IF(
  @identity_0130_saas_actor_verify_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 SaaS identity post-write conflict'''
);
PREPARE identity_0130_saas_actor_verify_guard_stmt FROM @identity_0130_saas_actor_verify_guard_sql;
EXECUTE identity_0130_saas_actor_verify_guard_stmt;
DEALLOCATE PREPARE identity_0130_saas_actor_verify_guard_stmt;

INSERT INTO mochat_go_identity_migration_journal (request_id, entity_type, entity_id, before_json)
SELECT @identity_0130_request_id, 'dashboard_identity', CAST(u.id AS CHAR), NULL
FROM mc_user u
WHERE u.deleted_at IS NULL
  AND u.tenant_id <> @identity_0130_platform_tenant_id
  AND TRIM(u.phone) <> ''
  AND NOT EXISTS (SELECT 1 FROM mochat_go_dashboard_identities existing WHERE existing.user_id = u.id)
  AND NOT EXISTS (
    SELECT 1 FROM mochat_go_identity_migration_journal j
    WHERE j.request_id = @identity_0130_request_id AND j.entity_type = 'dashboard_identity' AND j.entity_id = CAST(u.id AS CHAR)
  );

INSERT INTO mochat_go_dashboard_identities
  (user_id, login_identifier, password_hash, status, must_rotate_password, auth_version, mfa_required, activated_at)
SELECT u.id, TRIM(u.phone), u.password, IF(u.status = 1, 1, 2), 0, 1, 0,
       IF(u.status = 1, NOW(), NULL)
FROM mc_user u
WHERE u.deleted_at IS NULL
  AND u.tenant_id <> @identity_0130_platform_tenant_id
  AND TRIM(u.phone) <> ''
  AND NOT EXISTS (SELECT 1 FROM mochat_go_dashboard_identities existing WHERE existing.user_id = u.id);

SET @identity_0130_dashboard_identity_verify_count := (
  SELECT COUNT(*)
  FROM mc_user u
  INNER JOIN mochat_go_dashboard_identities d ON d.user_id = u.id
  WHERE u.deleted_at IS NULL AND u.tenant_id <> @identity_0130_platform_tenant_id AND TRIM(u.phone) <> ''
    AND (d.login_identifier <> TRIM(u.phone) OR d.password_hash <> u.password OR d.status <> IF(u.status = 1, 1, 2))
);
SET @identity_0130_dashboard_identity_verify_guard_sql := IF(
  @identity_0130_dashboard_identity_verify_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 Dashboard identity post-write conflict'''
);
PREPARE identity_0130_dashboard_identity_verify_guard_stmt FROM @identity_0130_dashboard_identity_verify_guard_sql;
EXECUTE identity_0130_dashboard_identity_verify_guard_stmt;
DEALLOCATE PREPARE identity_0130_dashboard_identity_verify_guard_stmt;

SET @identity_0130_active_dashboard_identity_missing_count := (
  SELECT COUNT(*)
  FROM mc_user u
  LEFT JOIN mochat_go_dashboard_identities d ON d.user_id = u.id
  WHERE u.deleted_at IS NULL
    AND u.status = 1
    AND u.tenant_id <> @identity_0130_platform_tenant_id
    AND d.user_id IS NULL
);
SET @identity_0130_active_dashboard_identity_missing_guard_sql := IF(
  @identity_0130_active_dashboard_identity_missing_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 active dashboard user is missing identity'''
);
PREPARE identity_0130_active_dashboard_identity_missing_guard_stmt FROM @identity_0130_active_dashboard_identity_missing_guard_sql;
EXECUTE identity_0130_active_dashboard_identity_missing_guard_stmt;
DEALLOCATE PREPARE identity_0130_active_dashboard_identity_missing_guard_stmt;

-- A zero-corp placeholder carries a unique audit marker and request id. Its
-- blank wx_corpid is never confused with an ordinary business corp row.
INSERT INTO mochat_go_identity_migration_journal (request_id, entity_type, entity_id, before_json)
SELECT @identity_0130_request_id, 'corp_placeholder', CAST(t.id AS CHAR), NULL
FROM mc_tenant t
LEFT JOIN mc_corp c ON c.tenant_id = t.id AND c.deleted_at IS NULL
WHERE t.status = 1 AND t.deleted_at IS NULL AND c.id IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM mochat_go_identity_migration_journal j
    WHERE j.request_id = @identity_0130_request_id AND j.entity_type = 'corp_placeholder' AND j.entity_id = CAST(t.id AS CHAR)
  );

INSERT INTO mc_corp
  (tenant_id, name, wx_corpid, created_at, updated_at)
SELECT t.id,
       CONCAT('Migration placeholder [0130:', @identity_0130_request_id, '] tenant ', t.id),
       '', NOW(), NOW()
FROM mc_tenant t
LEFT JOIN mc_corp c ON c.tenant_id = t.id AND c.deleted_at IS NULL
WHERE t.status = 1 AND t.deleted_at IS NULL AND c.id IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM mc_corp existing
    WHERE existing.tenant_id = t.id
      AND existing.name = CONCAT('Migration placeholder [0130:', @identity_0130_request_id, '] tenant ', t.id)
      AND existing.wx_corpid = ''
  );

INSERT INTO mochat_go_identity_migration_journal (request_id, entity_type, entity_id, before_json)
SELECT @identity_0130_request_id, 'tenant_corp_binding', CAST(selected.tenant_id AS CHAR), NULL
FROM (
  SELECT c.tenant_id, MAX(c.id) AS corp_id
  FROM mc_corp c
  INNER JOIN mc_tenant t ON t.id = c.tenant_id AND t.status = 1 AND t.deleted_at IS NULL
  WHERE c.deleted_at IS NULL
  GROUP BY c.tenant_id
  HAVING COUNT(*) = 1
    AND NOT EXISTS (
      SELECT 1 FROM mochat_go_identity_migration_corp_map one_corp_map
      WHERE one_corp_map.request_id = @identity_0130_request_id
        AND one_corp_map.tenant_id = c.tenant_id
        AND one_corp_map.status = 'validated'
    )
  UNION ALL
  SELECT m.tenant_id, m.corp_id
  FROM mochat_go_identity_migration_corp_map m
  WHERE m.request_id = @identity_0130_request_id AND m.status = 'validated'
) selected
WHERE NOT EXISTS (SELECT 1 FROM mochat_go_tenant_corp_bindings existing WHERE existing.tenant_id = selected.tenant_id)
  AND NOT EXISTS (SELECT 1 FROM mochat_go_identity_migration_journal j WHERE j.request_id = @identity_0130_request_id AND j.entity_type = 'tenant_corp_binding' AND j.entity_id = CAST(selected.tenant_id AS CHAR));

INSERT INTO mochat_go_tenant_corp_bindings
  (tenant_id, corp_id, status, version, verified_wx_corpid, verified_corp_name)
SELECT selected.tenant_id, selected.corp_id, 1, 1, NULL, ''
FROM (
  SELECT c.tenant_id, MAX(c.id) AS corp_id
  FROM mc_corp c
  INNER JOIN mc_tenant t ON t.id = c.tenant_id AND t.status = 1 AND t.deleted_at IS NULL
  WHERE c.deleted_at IS NULL
  GROUP BY c.tenant_id
  HAVING COUNT(*) = 1
    AND NOT EXISTS (
      SELECT 1 FROM mochat_go_identity_migration_corp_map one_corp_map
      WHERE one_corp_map.request_id = @identity_0130_request_id AND one_corp_map.tenant_id = c.tenant_id AND one_corp_map.status = 'validated'
    )
  UNION ALL
  SELECT m.tenant_id, m.corp_id
  FROM mochat_go_identity_migration_corp_map m
  WHERE m.request_id = @identity_0130_request_id AND m.status = 'validated'
) selected
WHERE NOT EXISTS (SELECT 1 FROM mochat_go_tenant_corp_bindings existing WHERE existing.tenant_id = selected.tenant_id);

SET @identity_0130_binding_verify_count := (
  SELECT COUNT(*)
  FROM (
    SELECT t.id AS tenant_id, COUNT(b.corp_id) AS binding_count
    FROM mc_tenant t
    LEFT JOIN mochat_go_tenant_corp_bindings b ON b.tenant_id = t.id
    WHERE t.status = 1 AND t.deleted_at IS NULL
    GROUP BY t.id
  ) binding_counts
  WHERE binding_counts.binding_count <> 1
);
SET @identity_0130_binding_verify_guard_sql := IF(
  @identity_0130_binding_verify_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0130 tenant binding post-write conflict'''
);
PREPARE identity_0130_binding_verify_guard_stmt FROM @identity_0130_binding_verify_guard_sql;
EXECUTE identity_0130_binding_verify_guard_stmt;
DEALLOCATE PREPARE identity_0130_binding_verify_guard_stmt;

-- Mark the completed batch before writing the success ledger; the standard
-- migration record may only be written after both facts are durable.
UPDATE mochat_go_identity_migration_batches
SET status = 'completed'
WHERE request_id = @identity_0130_request_id AND status = 'validated';
COMMIT;

-- The actor ids are preserved, so these FKs are added only after all identities
-- and all actor references have been proven. Unknown dependencies failed above.
SET @identity_0130_add_access_fk_sql := IF(
  (SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_saas_admin_user_access' AND constraint_name = 'fk_saas_admin_user_access_identity') > 0,
  'SELECT 1',
  'ALTER TABLE mochat_go_saas_admin_user_access ADD CONSTRAINT fk_saas_admin_user_access_identity FOREIGN KEY (user_id) REFERENCES mochat_go_saas_admin_users (id)'
);
PREPARE identity_0130_add_access_fk_stmt FROM @identity_0130_add_access_fk_sql;
EXECUTE identity_0130_add_access_fk_stmt;
DEALLOCATE PREPARE identity_0130_add_access_fk_stmt;

SET @identity_0130_add_role_fk_sql := IF(
  (SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_saas_admin_user_roles' AND constraint_name = 'fk_saas_admin_user_roles_identity') > 0,
  'SELECT 1',
  'ALTER TABLE mochat_go_saas_admin_user_roles ADD CONSTRAINT fk_saas_admin_user_roles_identity FOREIGN KEY (user_id) REFERENCES mochat_go_saas_admin_users (id)'
);
PREPARE identity_0130_add_role_fk_stmt FROM @identity_0130_add_role_fk_sql;
EXECUTE identity_0130_add_role_fk_stmt;
DEALLOCATE PREPARE identity_0130_add_role_fk_stmt;

INSERT INTO mochat_go_identity_migration_ledger
  (migration_name, request_id, phase, status, result_json)
VALUES
  ('0130_identity_realms_single_corp_backfill', @identity_0130_request_id, 'backfill', 'success',
   JSON_OBJECT('platformTenantId', @identity_0130_platform_tenant_id, 'status', 'completed', 'scriptChecksum', @identity_0130_script_checksum, 'migrationSource', '0130_identity_realms_single_corp_backfill'));
