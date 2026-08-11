-- Identity realm and single-corp schema.
-- Every consistency and dependency check in this file is before the first DDL.
-- MariaDB/MySQL DDL implicitly commits, so an unknown dependency or dirty relation
-- must stop without attempting a type change or table creation.

SET @identity_missing_0127_table_count := (
  SELECT COUNT(*)
  FROM (
    SELECT 'mochat_go_dashboard_permissions' AS table_name
    UNION ALL SELECT 'mochat_go_dashboard_permission_resources'
    UNION ALL SELECT 'mochat_go_dashboard_user_roles'
    UNION ALL SELECT 'mochat_go_dashboard_role_permissions'
    UNION ALL SELECT 'mochat_go_dashboard_user_permissions'
    UNION ALL SELECT 'mochat_go_dashboard_permission_audits'
  ) expected
  LEFT JOIN `information_schema`.`tables` actual
    ON actual.`table_schema` = DATABASE()
   AND actual.`table_name` = expected.`table_name`
  WHERE actual.`table_name` IS NULL
);
SET @identity_missing_0127_table_guard_sql := IF(
  @identity_missing_0127_table_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 missing complete 0127 dashboard schema'''
);
PREPARE identity_missing_0127_table_guard_stmt FROM @identity_missing_0127_table_guard_sql;
EXECUTE identity_missing_0127_table_guard_stmt;
DEALLOCATE PREPARE identity_missing_0127_table_guard_stmt;

SET @identity_missing_0127_tenant_column_count := (
  SELECT COUNT(*)
  FROM (
    SELECT 'mochat_go_dashboard_user_roles' AS table_name
    UNION ALL SELECT 'mochat_go_dashboard_role_permissions'
    UNION ALL SELECT 'mochat_go_dashboard_user_permissions'
    UNION ALL SELECT 'mochat_go_dashboard_permission_audits'
  ) expected
  LEFT JOIN `information_schema`.`columns` actual
    ON actual.`table_schema` = DATABASE()
   AND actual.`table_name` = expected.`table_name`
   AND actual.`column_name` = 'tenant_id'
  WHERE actual.`column_name` IS NULL
);
SET @identity_missing_0127_tenant_column_guard_sql := IF(
  @identity_missing_0127_tenant_column_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 missing 0127 tenant column'''
);
PREPARE identity_missing_0127_tenant_column_guard_stmt FROM @identity_missing_0127_tenant_column_guard_sql;
EXECUTE identity_missing_0127_tenant_column_guard_stmt;
DEALLOCATE PREPARE identity_missing_0127_tenant_column_guard_stmt;

-- The real 0127 permission catalog is global: permissions and resources have no
-- tenant_id. A tenant column there is an unreviewed schema dependency, not a
-- column this migration may guess how to alter.
SET @identity_unexpected_permission_tenant_column_count := (
  SELECT COUNT(*)
  FROM `information_schema`.`columns`
  WHERE `table_schema` = DATABASE()
    AND `table_name` IN ('mochat_go_dashboard_permissions', 'mochat_go_dashboard_permission_resources')
    AND `column_name` = 'tenant_id'
);
SET @identity_unexpected_permission_tenant_column_guard_sql := IF(
  @identity_unexpected_permission_tenant_column_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 unknown 0127 permission tenant column'''
);
PREPARE identity_unexpected_permission_tenant_column_guard_stmt FROM @identity_unexpected_permission_tenant_column_guard_sql;
EXECUTE identity_unexpected_permission_tenant_column_guard_stmt;
DEALLOCATE PREPARE identity_unexpected_permission_tenant_column_guard_stmt;

-- The allowlist is grouped by constraint and ordered by ordinal_position so a
-- newly added FK cannot hide behind a familiar constraint name. Known FK absence
-- is allowed here; the relation queries below still prove the data is clean.
SET @identity_unknown_tenant_fk_count := (
  SELECT COUNT(*)
  FROM (
    SELECT k.`table_name`, k.`constraint_name`,
           GROUP_CONCAT(CONCAT(k.`column_name`, '=', k.`referenced_table_name`, '.', k.`referenced_column_name`) ORDER BY k.`ordinal_position` SEPARATOR ',') AS `signature`
    FROM `information_schema`.`key_column_usage` k
    WHERE k.`constraint_schema` = DATABASE()
      AND k.`referenced_table_name` IS NOT NULL
      AND (
        k.`column_name` = 'tenant_id'
        OR k.`referenced_column_name` = 'tenant_id'
        OR k.`table_name` IN (
          'mochat_go_dashboard_permissions',
          'mochat_go_dashboard_permission_resources',
          'mochat_go_dashboard_user_roles',
          'mochat_go_dashboard_role_permissions',
          'mochat_go_dashboard_user_permissions',
          'mochat_go_dashboard_permission_audits'
        )
      )
    GROUP BY k.`table_name`, k.`constraint_name`
  ) dependencies
  WHERE NOT (
    (`table_name` = 'mochat_go_dashboard_permission_resources' AND `constraint_name` = 'fk_dashboard_permission_resource_permission' AND `signature` = 'permission_id=mochat_go_dashboard_permissions.id')
    OR (`table_name` = 'mochat_go_dashboard_user_roles' AND `constraint_name` = 'fk_dashboard_user_roles_user' AND `signature` = 'tenant_id=mc_user.tenant_id,user_id=mc_user.id')
    OR (`table_name` = 'mochat_go_dashboard_user_roles' AND `constraint_name` = 'fk_dashboard_user_roles_role' AND `signature` = 'tenant_id=mc_rbac_role.tenant_id,role_id=mc_rbac_role.id')
    OR (`table_name` = 'mochat_go_dashboard_role_permissions' AND `constraint_name` = 'fk_dashboard_role_permissions_role' AND `signature` = 'tenant_id=mc_rbac_role.tenant_id,role_id=mc_rbac_role.id')
    OR (`table_name` = 'mochat_go_dashboard_role_permissions' AND `constraint_name` = 'fk_dashboard_role_permissions_permission' AND `signature` = 'permission_id=mochat_go_dashboard_permissions.id')
    OR (`table_name` = 'mochat_go_dashboard_user_permissions' AND `constraint_name` = 'fk_dashboard_user_permissions_user' AND `signature` = 'tenant_id=mc_user.tenant_id,user_id=mc_user.id')
    OR (`table_name` = 'mochat_go_dashboard_user_permissions' AND `constraint_name` = 'fk_dashboard_user_permissions_permission' AND `signature` = 'permission_id=mochat_go_dashboard_permissions.id')
    OR (`table_name` = 'mochat_go_dashboard_permission_audits' AND `constraint_name` = 'fk_dashboard_audit_actor' AND `signature` = 'tenant_id=mc_user.tenant_id,actor_user_id=mc_user.id')
    OR (`table_name` = 'mochat_go_dashboard_identities' AND `constraint_name` = 'fk_dashboard_identity_user' AND `signature` = 'user_id=mc_user.id')
    OR (`table_name` = 'mochat_go_dashboard_identity_activations' AND `constraint_name` = 'fk_dashboard_identity_activation_identity' AND `signature` = 'user_id=mochat_go_dashboard_identities.user_id')
    OR (`table_name` = 'mochat_go_dashboard_identity_activations' AND `constraint_name` = 'fk_dashboard_identity_activation_saas_user' AND `signature` = 'created_by_saas_user_id=mochat_go_saas_admin_users.id')
    OR (`table_name` = 'mochat_go_tenant_corp_bindings' AND `constraint_name` = 'fk_tenant_corp_binding_tenant' AND `signature` = 'tenant_id=mc_tenant.id')
    OR (`table_name` = 'mochat_go_tenant_corp_bindings' AND `constraint_name` = 'fk_tenant_corp_binding_corp' AND `signature` = 'tenant_id=mc_corp.tenant_id,corp_id=mc_corp.id')
  )
);
SET @identity_unknown_tenant_fk_guard_sql := IF(
  @identity_unknown_tenant_fk_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 unknown tenant dependency FK'''
);
PREPARE identity_unknown_tenant_fk_guard_stmt FROM @identity_unknown_tenant_fk_guard_sql;
EXECUTE identity_unknown_tenant_fk_guard_stmt;
DEALLOCATE PREPARE identity_unknown_tenant_fk_guard_stmt;

-- Index dependencies are allowlisted by table, name and ordered columns. This
-- includes the indexes created by 0127 and the target indexes used by 0129.
SET @identity_unknown_tenant_index_count := (
  SELECT COUNT(*)
  FROM (
    SELECT s.`table_name`, s.`index_name`,
           GROUP_CONCAT(s.`column_name` ORDER BY s.`seq_in_index` SEPARATOR ',') AS `signature`
    FROM `information_schema`.`statistics` s
    WHERE s.`table_schema` = DATABASE()
      AND s.`table_name` IN (
        'mc_user', 'mc_corp', 'mc_rbac_role',
        'mochat_go_dashboard_user_roles',
        'mochat_go_dashboard_role_permissions',
        'mochat_go_dashboard_user_permissions',
        'mochat_go_dashboard_permission_audits',
        'mochat_go_tenant_corp_bindings'
      )
      AND EXISTS (
        SELECT 1
        FROM `information_schema`.`statistics` tenant_index
        WHERE tenant_index.`table_schema` = s.`table_schema`
          AND tenant_index.`table_name` = s.`table_name`
          AND tenant_index.`index_name` = s.`index_name`
          AND tenant_index.`column_name` = 'tenant_id'
      )
    GROUP BY s.`table_name`, s.`index_name`
  ) dependencies
  WHERE NOT (
    (`table_name` = 'mc_user' AND `index_name` = 'uni_dashboard_user_tenant_id_id' AND `signature` = 'tenant_id,id')
    OR (`table_name` = 'mc_corp' AND `index_name` = 'uni_mc_corp_tenant_id_id' AND `signature` = 'tenant_id,id')
    OR (`table_name` = 'mc_rbac_role' AND `index_name` = 'uni_dashboard_role_tenant_id_id' AND `signature` = 'tenant_id,id')
    OR (`table_name` = 'mochat_go_dashboard_user_roles' AND `index_name` = 'uni_dashboard_user_roles' AND `signature` = 'tenant_id,user_id,role_id')
    OR (`table_name` = 'mochat_go_dashboard_user_roles' AND `index_name` = 'idx_dashboard_user_roles_role' AND `signature` = 'tenant_id,role_id,user_id')
    OR (`table_name` = 'mochat_go_dashboard_role_permissions' AND `index_name` = 'uni_dashboard_role_permissions' AND `signature` = 'tenant_id,role_id,permission_id')
    OR (`table_name` = 'mochat_go_dashboard_user_permissions' AND `index_name` = 'uni_dashboard_user_permissions' AND `signature` = 'tenant_id,user_id,permission_id')
    OR (`table_name` = 'mochat_go_dashboard_permission_audits' AND `index_name` = 'idx_dashboard_permission_audits_tenant_time' AND `signature` = 'tenant_id,created_at,id')
    OR (`table_name` = 'mochat_go_dashboard_permission_audits' AND `index_name` = 'idx_dashboard_permission_audits_target' AND `signature` = 'tenant_id,target_type,target_id,id')
    OR (`table_name` = 'mochat_go_dashboard_permission_audits' AND `index_name` = 'fk_dashboard_audit_actor' AND `signature` = 'tenant_id,actor_user_id')
    OR (`table_name` = 'mochat_go_tenant_corp_bindings' AND `index_name` = 'PRIMARY' AND `signature` = 'tenant_id')
  )
);
SET @identity_unknown_tenant_index_guard_sql := IF(
  @identity_unknown_tenant_index_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 unknown tenant dependency index'''
);
PREPARE identity_unknown_tenant_index_guard_stmt FROM @identity_unknown_tenant_index_guard_sql;
EXECUTE identity_unknown_tenant_index_guard_stmt;
DEALLOCATE PREPARE identity_unknown_tenant_index_guard_stmt;

SET @identity_duplicate_dashboard_login_count := (
  SELECT COUNT(*)
  FROM `mc_user` u
  INNER JOIN `mc_user` duplicate_user
    ON duplicate_user.`phone` = u.`phone`
   AND duplicate_user.`id` <> u.`id`
   AND duplicate_user.`deleted_at` IS NULL
   AND duplicate_user.`status` = 1
  WHERE u.`deleted_at` IS NULL
    AND u.`status` = 1
    AND TRIM(u.`phone`) <> ''
);
SET @identity_duplicate_dashboard_login_guard_sql := IF(
  @identity_duplicate_dashboard_login_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 duplicate dashboard login identifier'''
);
PREPARE identity_duplicate_dashboard_login_guard_stmt FROM @identity_duplicate_dashboard_login_guard_sql;
EXECUTE identity_duplicate_dashboard_login_guard_stmt;
DEALLOCATE PREPARE identity_duplicate_dashboard_login_guard_stmt;

SET @identity_invalid_user_tenant_count := (
  SELECT COUNT(*)
  FROM `mc_user` u
  LEFT JOIN `mc_tenant` t ON t.`id` = u.`tenant_id`
  WHERE u.`tenant_id` IS NULL
     OR CAST(u.`tenant_id` AS DECIMAL(20,0)) < 0
     OR CAST(u.`tenant_id` AS DECIMAL(20,0)) > 4294967295
     OR t.`id` IS NULL
);
SET @identity_invalid_user_tenant_guard_sql := IF(
  @identity_invalid_user_tenant_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 invalid mc_user tenant'''
);
PREPARE identity_invalid_user_tenant_guard_stmt FROM @identity_invalid_user_tenant_guard_sql;
EXECUTE identity_invalid_user_tenant_guard_stmt;
DEALLOCATE PREPARE identity_invalid_user_tenant_guard_stmt;

SET @identity_invalid_corp_tenant_count := (
  SELECT COUNT(*)
  FROM `mc_corp` c
  LEFT JOIN `mc_tenant` t ON t.`id` = c.`tenant_id`
  WHERE c.`tenant_id` IS NULL
     OR CAST(c.`tenant_id` AS DECIMAL(20,0)) < 0
     OR CAST(c.`tenant_id` AS DECIMAL(20,0)) > 4294967295
     OR t.`id` IS NULL
);
SET @identity_invalid_corp_tenant_guard_sql := IF(
  @identity_invalid_corp_tenant_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 invalid mc_corp tenant'''
);
PREPARE identity_invalid_corp_tenant_guard_stmt FROM @identity_invalid_corp_tenant_guard_sql;
EXECUTE identity_invalid_corp_tenant_guard_stmt;
DEALLOCATE PREPARE identity_invalid_corp_tenant_guard_stmt;

SET @identity_invalid_role_tenant_count := (
  SELECT COUNT(*)
  FROM `mc_rbac_role` r
  LEFT JOIN `mc_tenant` t ON t.`id` = r.`tenant_id`
  WHERE r.`tenant_id` IS NULL
     OR CAST(r.`tenant_id` AS DECIMAL(20,0)) < 0
     OR CAST(r.`tenant_id` AS DECIMAL(20,0)) > 4294967295
     OR t.`id` IS NULL
);
SET @identity_invalid_role_tenant_guard_sql := IF(
  @identity_invalid_role_tenant_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 invalid mc_rbac_role tenant'''
);
PREPARE identity_invalid_role_tenant_guard_stmt FROM @identity_invalid_role_tenant_guard_sql;
EXECUTE identity_invalid_role_tenant_guard_stmt;
DEALLOCATE PREPARE identity_invalid_role_tenant_guard_stmt;

SET @identity_invalid_0127_tenant_count := (
  SELECT COUNT(*) FROM `mochat_go_dashboard_user_roles` WHERE `tenant_id` IS NULL OR CAST(`tenant_id` AS DECIMAL(20,0)) < 0 OR CAST(`tenant_id` AS DECIMAL(20,0)) > 4294967295
)
+ (SELECT COUNT(*) FROM `mochat_go_dashboard_role_permissions` WHERE `tenant_id` IS NULL OR CAST(`tenant_id` AS DECIMAL(20,0)) < 0 OR CAST(`tenant_id` AS DECIMAL(20,0)) > 4294967295)
+ (SELECT COUNT(*) FROM `mochat_go_dashboard_user_permissions` WHERE `tenant_id` IS NULL OR CAST(`tenant_id` AS DECIMAL(20,0)) < 0 OR CAST(`tenant_id` AS DECIMAL(20,0)) > 4294967295)
+ (SELECT COUNT(*) FROM `mochat_go_dashboard_permission_audits` WHERE `tenant_id` IS NULL OR CAST(`tenant_id` AS DECIMAL(20,0)) < 0 OR CAST(`tenant_id` AS DECIMAL(20,0)) > 4294967295);
SET @identity_invalid_0127_tenant_guard_sql := IF(
  @identity_invalid_0127_tenant_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 invalid 0127 tenant column'''
);
PREPARE identity_invalid_0127_tenant_guard_stmt FROM @identity_invalid_0127_tenant_guard_sql;
EXECUTE identity_invalid_0127_tenant_guard_stmt;
DEALLOCATE PREPARE identity_invalid_0127_tenant_guard_stmt;

SET @identity_dashboard_user_role_relation_count := (
  SELECT COUNT(*)
  FROM `mochat_go_dashboard_user_roles` ur
  LEFT JOIN `mc_user` u ON u.`id` = ur.`user_id`
  LEFT JOIN `mc_rbac_role` r ON r.`id` = ur.`role_id`
  WHERE u.`id` IS NULL OR r.`id` IS NULL OR u.`tenant_id` <> ur.`tenant_id` OR r.`tenant_id` <> ur.`tenant_id`
);
SET @identity_dashboard_user_role_relation_guard_sql := IF(
  @identity_dashboard_user_role_relation_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 dashboard user-role relationship'''
);
PREPARE identity_dashboard_user_role_relation_guard_stmt FROM @identity_dashboard_user_role_relation_guard_sql;
EXECUTE identity_dashboard_user_role_relation_guard_stmt;
DEALLOCATE PREPARE identity_dashboard_user_role_relation_guard_stmt;

SET @identity_dashboard_role_permission_relation_count := (
  SELECT COUNT(*)
  FROM `mochat_go_dashboard_role_permissions` rp
  LEFT JOIN `mc_rbac_role` r ON r.`id` = rp.`role_id`
  LEFT JOIN `mochat_go_dashboard_permissions` p ON p.`id` = rp.`permission_id`
  WHERE r.`id` IS NULL OR p.`id` IS NULL OR r.`tenant_id` <> rp.`tenant_id`
);
SET @identity_dashboard_role_permission_relation_guard_sql := IF(
  @identity_dashboard_role_permission_relation_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 dashboard role-permission relationship'''
);
PREPARE identity_dashboard_role_permission_relation_guard_stmt FROM @identity_dashboard_role_permission_relation_guard_sql;
EXECUTE identity_dashboard_role_permission_relation_guard_stmt;
DEALLOCATE PREPARE identity_dashboard_role_permission_relation_guard_stmt;

SET @identity_dashboard_user_permission_relation_count := (
  SELECT COUNT(*)
  FROM `mochat_go_dashboard_user_permissions` up
  LEFT JOIN `mc_user` u ON u.`id` = up.`user_id`
  LEFT JOIN `mochat_go_dashboard_permissions` p ON p.`id` = up.`permission_id`
  WHERE u.`id` IS NULL OR p.`id` IS NULL OR u.`tenant_id` <> up.`tenant_id`
);
SET @identity_dashboard_user_permission_relation_guard_sql := IF(
  @identity_dashboard_user_permission_relation_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 dashboard user-permission relationship'''
);
PREPARE identity_dashboard_user_permission_relation_guard_stmt FROM @identity_dashboard_user_permission_relation_guard_sql;
EXECUTE identity_dashboard_user_permission_relation_guard_stmt;
DEALLOCATE PREPARE identity_dashboard_user_permission_relation_guard_stmt;

SET @identity_dashboard_audit_relation_count := (
  SELECT COUNT(*)
  FROM `mochat_go_dashboard_permission_audits` a
  LEFT JOIN `mc_user` u ON u.`id` = a.`actor_user_id`
  WHERE a.`actor_user_id` IS NOT NULL AND (u.`id` IS NULL OR u.`tenant_id` <> a.`tenant_id`)
);
SET @identity_dashboard_audit_relation_guard_sql := IF(
  @identity_dashboard_audit_relation_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 dashboard audit actor relationship'''
);
PREPARE identity_dashboard_audit_relation_guard_stmt FROM @identity_dashboard_audit_relation_guard_sql;
EXECUTE identity_dashboard_audit_relation_guard_stmt;
DEALLOCATE PREPARE identity_dashboard_audit_relation_guard_stmt;

SET @identity_dashboard_permission_resource_relation_count := (
  SELECT COUNT(*)
  FROM `mochat_go_dashboard_permission_resources` pr
  LEFT JOIN `mochat_go_dashboard_permissions` p ON p.`id` = pr.`permission_id`
  WHERE p.`id` IS NULL
);
SET @identity_dashboard_permission_resource_relation_guard_sql := IF(
  @identity_dashboard_permission_resource_relation_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 dashboard permission-resource relationship'''
);
PREPARE identity_dashboard_permission_resource_relation_guard_stmt FROM @identity_dashboard_permission_resource_relation_guard_sql;
EXECUTE identity_dashboard_permission_resource_relation_guard_stmt;
DEALLOCATE PREPARE identity_dashboard_permission_resource_relation_guard_stmt;

SET @identity_legacy_user_role_relation_count := (
  SELECT COUNT(*)
  FROM `mc_rbac_user_role` ur
  LEFT JOIN `mc_user` u ON u.`id` = CAST(ur.`user_id` AS UNSIGNED)
  LEFT JOIN `mc_rbac_role` r ON r.`id` = ur.`role_id`
  WHERE ur.`deleted_at` IS NULL
    AND (u.`id` IS NULL OR r.`id` IS NULL OR u.`tenant_id` <> r.`tenant_id`)
);
SET @identity_legacy_user_role_relation_guard_sql := IF(
  @identity_legacy_user_role_relation_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 legacy user-role relationship'''
);
PREPARE identity_legacy_user_role_relation_guard_stmt FROM @identity_legacy_user_role_relation_guard_sql;
EXECUTE identity_legacy_user_role_relation_guard_stmt;
DEALLOCATE PREPARE identity_legacy_user_role_relation_guard_stmt;

-- All eight 0127 foreign keys are dropped as one explicit allowlisted cutover
-- set. Permission/resource FKs are not tenant composites, but are restored as a
-- pair as well so a partial run can never permanently lose them.
SET @identity_drop_permission_resource_permission_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_permission_resources' AND `constraint_name` = 'fk_dashboard_permission_resource_permission') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_permission_resources` DROP FOREIGN KEY `fk_dashboard_permission_resource_permission`'
);
PREPARE identity_drop_permission_resource_permission_stmt FROM @identity_drop_permission_resource_permission_sql;
EXECUTE identity_drop_permission_resource_permission_stmt;
DEALLOCATE PREPARE identity_drop_permission_resource_permission_stmt;

SET @identity_drop_user_roles_user_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `constraint_name` = 'fk_dashboard_user_roles_user') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_roles` DROP FOREIGN KEY `fk_dashboard_user_roles_user`'
);
PREPARE identity_drop_user_roles_user_stmt FROM @identity_drop_user_roles_user_sql;
EXECUTE identity_drop_user_roles_user_stmt;
DEALLOCATE PREPARE identity_drop_user_roles_user_stmt;

SET @identity_drop_user_roles_role_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `constraint_name` = 'fk_dashboard_user_roles_role') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_roles` DROP FOREIGN KEY `fk_dashboard_user_roles_role`'
);
PREPARE identity_drop_user_roles_role_stmt FROM @identity_drop_user_roles_role_sql;
EXECUTE identity_drop_user_roles_role_stmt;
DEALLOCATE PREPARE identity_drop_user_roles_role_stmt;

SET @identity_drop_role_permissions_role_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_role_permissions' AND `constraint_name` = 'fk_dashboard_role_permissions_role') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_role_permissions` DROP FOREIGN KEY `fk_dashboard_role_permissions_role`'
);
PREPARE identity_drop_role_permissions_role_stmt FROM @identity_drop_role_permissions_role_sql;
EXECUTE identity_drop_role_permissions_role_stmt;
DEALLOCATE PREPARE identity_drop_role_permissions_role_stmt;

SET @identity_drop_role_permissions_permission_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_role_permissions' AND `constraint_name` = 'fk_dashboard_role_permissions_permission') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_role_permissions` DROP FOREIGN KEY `fk_dashboard_role_permissions_permission`'
);
PREPARE identity_drop_role_permissions_permission_stmt FROM @identity_drop_role_permissions_permission_sql;
EXECUTE identity_drop_role_permissions_permission_stmt;
DEALLOCATE PREPARE identity_drop_role_permissions_permission_stmt;

SET @identity_drop_user_permissions_user_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_permissions' AND `constraint_name` = 'fk_dashboard_user_permissions_user') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_permissions` DROP FOREIGN KEY `fk_dashboard_user_permissions_user`'
);
PREPARE identity_drop_user_permissions_user_stmt FROM @identity_drop_user_permissions_user_sql;
EXECUTE identity_drop_user_permissions_user_stmt;
DEALLOCATE PREPARE identity_drop_user_permissions_user_stmt;

SET @identity_drop_user_permissions_permission_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_permissions' AND `constraint_name` = 'fk_dashboard_user_permissions_permission') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_permissions` DROP FOREIGN KEY `fk_dashboard_user_permissions_permission`'
);
PREPARE identity_drop_user_permissions_permission_stmt FROM @identity_drop_user_permissions_permission_sql;
EXECUTE identity_drop_user_permissions_permission_stmt;
DEALLOCATE PREPARE identity_drop_user_permissions_permission_stmt;

SET @identity_drop_audit_actor_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_permission_audits' AND `constraint_name` = 'fk_dashboard_audit_actor') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_permission_audits` DROP FOREIGN KEY `fk_dashboard_audit_actor`'
);
PREPARE identity_drop_audit_actor_stmt FROM @identity_drop_audit_actor_sql;
EXECUTE identity_drop_audit_actor_stmt;
DEALLOCATE PREPARE identity_drop_audit_actor_stmt;

ALTER TABLE `mc_user`
  MODIFY COLUMN `tenant_id` int(10) unsigned NOT NULL DEFAULT 1;

ALTER TABLE `mc_corp`
  MODIFY COLUMN `tenant_id` int(10) unsigned NULL DEFAULT 0;

ALTER TABLE `mc_rbac_role`
  MODIFY COLUMN `tenant_id` int(10) unsigned NOT NULL;

ALTER TABLE `mochat_go_dashboard_user_roles`
  MODIFY COLUMN `tenant_id` int(10) unsigned NOT NULL;

ALTER TABLE `mochat_go_dashboard_role_permissions`
  MODIFY COLUMN `tenant_id` int(10) unsigned NOT NULL;

ALTER TABLE `mochat_go_dashboard_user_permissions`
  MODIFY COLUMN `tenant_id` int(10) unsigned NOT NULL;

ALTER TABLE `mochat_go_dashboard_permission_audits`
  MODIFY COLUMN `tenant_id` int(10) unsigned NOT NULL;

SET @identity_add_corp_composite_index_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`statistics` WHERE `table_schema` = DATABASE() AND `table_name` = 'mc_corp' AND `index_name` = 'uni_mc_corp_tenant_id_id') > 0,
  'SELECT 1',
  'ALTER TABLE `mc_corp` ADD UNIQUE KEY `uni_mc_corp_tenant_id_id` (`tenant_id`, `id`)'
);
PREPARE identity_add_corp_composite_index_stmt FROM @identity_add_corp_composite_index_sql;
EXECUTE identity_add_corp_composite_index_stmt;
DEALLOCATE PREPARE identity_add_corp_composite_index_stmt;

SET @identity_add_permission_resource_permission_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_permission_resources' AND `constraint_name` = 'fk_dashboard_permission_resource_permission') > 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_permission_resources` ADD CONSTRAINT `fk_dashboard_permission_resource_permission` FOREIGN KEY (`permission_id`) REFERENCES `mochat_go_dashboard_permissions` (`id`)'
);
PREPARE identity_add_permission_resource_permission_stmt FROM @identity_add_permission_resource_permission_sql;
EXECUTE identity_add_permission_resource_permission_stmt;
DEALLOCATE PREPARE identity_add_permission_resource_permission_stmt;

SET @identity_add_user_roles_user_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `constraint_name` = 'fk_dashboard_user_roles_user') > 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_roles` ADD CONSTRAINT `fk_dashboard_user_roles_user` FOREIGN KEY (`tenant_id`, `user_id`) REFERENCES `mc_user` (`tenant_id`, `id`)'
);
PREPARE identity_add_user_roles_user_stmt FROM @identity_add_user_roles_user_sql;
EXECUTE identity_add_user_roles_user_stmt;
DEALLOCATE PREPARE identity_add_user_roles_user_stmt;

SET @identity_add_user_roles_role_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `constraint_name` = 'fk_dashboard_user_roles_role') > 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_roles` ADD CONSTRAINT `fk_dashboard_user_roles_role` FOREIGN KEY (`tenant_id`, `role_id`) REFERENCES `mc_rbac_role` (`tenant_id`, `id`)'
);
PREPARE identity_add_user_roles_role_stmt FROM @identity_add_user_roles_role_sql;
EXECUTE identity_add_user_roles_role_stmt;
DEALLOCATE PREPARE identity_add_user_roles_role_stmt;

SET @identity_add_role_permissions_role_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_role_permissions' AND `constraint_name` = 'fk_dashboard_role_permissions_role') > 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_role_permissions` ADD CONSTRAINT `fk_dashboard_role_permissions_role` FOREIGN KEY (`tenant_id`, `role_id`) REFERENCES `mc_rbac_role` (`tenant_id`, `id`)'
);
PREPARE identity_add_role_permissions_role_stmt FROM @identity_add_role_permissions_role_sql;
EXECUTE identity_add_role_permissions_role_stmt;
DEALLOCATE PREPARE identity_add_role_permissions_role_stmt;

SET @identity_add_role_permissions_permission_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_role_permissions' AND `constraint_name` = 'fk_dashboard_role_permissions_permission') > 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_role_permissions` ADD CONSTRAINT `fk_dashboard_role_permissions_permission` FOREIGN KEY (`permission_id`) REFERENCES `mochat_go_dashboard_permissions` (`id`)'
);
PREPARE identity_add_role_permissions_permission_stmt FROM @identity_add_role_permissions_permission_sql;
EXECUTE identity_add_role_permissions_permission_stmt;
DEALLOCATE PREPARE identity_add_role_permissions_permission_stmt;

SET @identity_add_user_permissions_user_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_permissions' AND `constraint_name` = 'fk_dashboard_user_permissions_user') > 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_permissions` ADD CONSTRAINT `fk_dashboard_user_permissions_user` FOREIGN KEY (`tenant_id`, `user_id`) REFERENCES `mc_user` (`tenant_id`, `id`)'
);
PREPARE identity_add_user_permissions_user_stmt FROM @identity_add_user_permissions_user_sql;
EXECUTE identity_add_user_permissions_user_stmt;
DEALLOCATE PREPARE identity_add_user_permissions_user_stmt;

SET @identity_add_user_permissions_permission_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_permissions' AND `constraint_name` = 'fk_dashboard_user_permissions_permission') > 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_permissions` ADD CONSTRAINT `fk_dashboard_user_permissions_permission` FOREIGN KEY (`permission_id`) REFERENCES `mochat_go_dashboard_permissions` (`id`)'
);
PREPARE identity_add_user_permissions_permission_stmt FROM @identity_add_user_permissions_permission_sql;
EXECUTE identity_add_user_permissions_permission_stmt;
DEALLOCATE PREPARE identity_add_user_permissions_permission_stmt;

SET @identity_add_audit_actor_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_permission_audits' AND `constraint_name` = 'fk_dashboard_audit_actor') > 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_permission_audits` ADD CONSTRAINT `fk_dashboard_audit_actor` FOREIGN KEY (`tenant_id`, `actor_user_id`) REFERENCES `mc_user` (`tenant_id`, `id`)'
);
PREPARE identity_add_audit_actor_stmt FROM @identity_add_audit_actor_sql;
EXECUTE identity_add_audit_actor_stmt;
DEALLOCATE PREPARE identity_add_audit_actor_stmt;

CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_users` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `login_name` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `phone` varchar(32) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `password_hash` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` tinyint(3) unsigned NOT NULL DEFAULT 1,
  `must_rotate_password` tinyint(3) unsigned NOT NULL DEFAULT 1,
  `auth_version` bigint(20) unsigned NOT NULL DEFAULT 1,
  `mfa_required` tinyint(3) unsigned NOT NULL DEFAULT 1,
  `bootstrap_request_key` varchar(96) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_saas_admin_user_login_name` (`login_name`),
  UNIQUE KEY `uni_saas_admin_user_phone` (`phone`),
  UNIQUE KEY `uni_saas_admin_user_bootstrap_request_key` (`bootstrap_request_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='SaaS platform identity';

-- Generic SaaS mutation receipt. It intentionally stores no token, digest, or
-- other credential material; 0130 may add the historical actor FK after
-- backfill, but request uniqueness and result replay safety exist in 0129.
CREATE TABLE IF NOT EXISTS `mochat_go_saas_idempotency_receipts` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `operation` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL,
  `request_key` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `fingerprint` binary(32) NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT '0',
  `target_id` int(10) unsigned NOT NULL DEFAULT '0',
  `result_version` bigint(20) unsigned NOT NULL DEFAULT '0',
  `status` tinyint(3) unsigned NOT NULL DEFAULT '0' COMMENT '0=pending,1=committed',
  `created_by_saas_user_id` int(10) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_saas_idempotency_operation_request` (`operation`, `request_key`),
  KEY `idx_saas_idempotency_target` (`operation`, `tenant_id`, `target_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='SaaS mutation idempotency receipt without secret material';

CREATE TABLE IF NOT EXISTS `mochat_go_dashboard_identities` (
  `user_id` int(10) unsigned NOT NULL,
  `login_identifier` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `password_hash` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` tinyint(3) unsigned NOT NULL DEFAULT 1,
  `must_rotate_password` tinyint(3) unsigned NOT NULL DEFAULT 1,
  `auth_version` bigint(20) unsigned NOT NULL DEFAULT 1,
  `mfa_required` tinyint(3) unsigned NOT NULL DEFAULT 1,
  `activated_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`user_id`),
  UNIQUE KEY `uni_dashboard_identity_login_identifier` (`login_identifier`),
  CONSTRAINT `fk_dashboard_identity_user` FOREIGN KEY (`user_id`) REFERENCES `mc_user` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Dashboard tenant identity';

CREATE TABLE IF NOT EXISTS `mochat_go_dashboard_identity_activations` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` int(10) unsigned NOT NULL,
  `token_digest` binary(32) NOT NULL,
  `expires_at` timestamp NOT NULL,
  `consumed_at` timestamp NULL DEFAULT NULL,
  `created_by_saas_user_id` int(10) unsigned NOT NULL,
  `request_id` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_dashboard_identity_activation_digest` (`token_digest`),
  KEY `idx_dashboard_identity_activation_user` (`user_id`, `consumed_at`, `expires_at`),
  CONSTRAINT `fk_dashboard_identity_activation_identity` FOREIGN KEY (`user_id`) REFERENCES `mochat_go_dashboard_identities` (`user_id`),
  CONSTRAINT `fk_dashboard_identity_activation_saas_user` FOREIGN KEY (`created_by_saas_user_id`) REFERENCES `mochat_go_saas_admin_users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='One-time Dashboard identity activation digest';

CREATE TABLE IF NOT EXISTS `mochat_go_tenant_corp_bindings` (
  `tenant_id` int(10) unsigned NOT NULL,
  `corp_id` int(10) unsigned NOT NULL,
  `status` tinyint(3) unsigned NOT NULL DEFAULT 1,
  `version` bigint(20) unsigned NOT NULL DEFAULT 1,
  `verified_wx_corpid` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `verified_corp_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `verified_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`tenant_id`),
  UNIQUE KEY `uni_tenant_corp_binding_corp` (`corp_id`),
  UNIQUE KEY `uni_tenant_corp_binding_verified_wx_corpid` (`verified_wx_corpid`),
  CONSTRAINT `fk_tenant_corp_binding_tenant` FOREIGN KEY (`tenant_id`) REFERENCES `mc_tenant` (`id`),
  CONSTRAINT `fk_tenant_corp_binding_corp` FOREIGN KEY (`tenant_id`, `corp_id`) REFERENCES `mc_corp` (`tenant_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Authoritative one-tenant one-corp binding';

CREATE TABLE IF NOT EXISTS `mochat_go_dashboard_mfa_credentials` (
  `user_id` int(10) unsigned NOT NULL,
  `status` tinyint(3) unsigned NOT NULL DEFAULT 0 COMMENT '0=pending,1=active,2=disabled',
  `secret_ciphertext` longtext COLLATE utf8mb4_bin NOT NULL,
  `encryption_key_id` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL,
  `last_totp_step` bigint(20) unsigned DEFAULT NULL,
  `verified_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`user_id`),
  CONSTRAINT `fk_dashboard_mfa_user` FOREIGN KEY (`user_id`) REFERENCES `mochat_go_dashboard_identities` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Dashboard MFA encrypted credential';

CREATE TABLE IF NOT EXISTS `mochat_go_dashboard_mfa_challenges` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `token_digest` binary(32) NOT NULL,
  `user_id` int(10) unsigned NOT NULL,
  `auth_version` bigint(20) unsigned NOT NULL,
  `challenge_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` tinyint(3) unsigned NOT NULL DEFAULT 0 COMMENT '0=pending,1=consumed,2=locked',
  `attempts` tinyint(3) unsigned NOT NULL DEFAULT 0,
  `max_attempts` tinyint(3) unsigned NOT NULL DEFAULT 5,
  `expires_at` timestamp NOT NULL,
  `consumed_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_dashboard_mfa_challenge_digest` (`token_digest`),
  KEY `idx_dashboard_mfa_challenge_user` (`user_id`, `status`, `expires_at`),
  CONSTRAINT `fk_dashboard_mfa_challenge_user` FOREIGN KEY (`user_id`) REFERENCES `mochat_go_dashboard_identities` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Dashboard MFA challenge';

CREATE TABLE IF NOT EXISTS `mochat_go_dashboard_sessions` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `jti_digest` binary(32) NOT NULL,
  `user_id` int(10) unsigned NOT NULL,
  `auth_version` bigint(20) unsigned NOT NULL,
  `status` tinyint(3) unsigned NOT NULL DEFAULT 1 COMMENT '1=active,2=revoked',
  `issued_at` timestamp NOT NULL,
  `expires_at` timestamp NOT NULL,
  `revoked_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_dashboard_session_jti_digest` (`jti_digest`),
  KEY `idx_dashboard_session_user` (`user_id`, `auth_version`, `status`, `expires_at`),
  CONSTRAINT `fk_dashboard_session_user` FOREIGN KEY (`user_id`) REFERENCES `mochat_go_dashboard_identities` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Dashboard session';

CREATE TABLE IF NOT EXISTS `mochat_go_dashboard_password_resets` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `token_digest` binary(32) NOT NULL,
  `user_id` int(10) unsigned NOT NULL,
  `auth_version` bigint(20) unsigned NOT NULL,
  `status` tinyint(3) unsigned NOT NULL DEFAULT 0 COMMENT '0=pending,1=consumed',
  `expires_at` timestamp NOT NULL,
  `consumed_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_dashboard_password_reset_digest` (`token_digest`),
  KEY `idx_dashboard_password_reset_user` (`user_id`, `status`, `expires_at`),
  CONSTRAINT `fk_dashboard_password_reset_user` FOREIGN KEY (`user_id`) REFERENCES `mochat_go_dashboard_identities` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Dashboard password reset';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_mfa_credentials` (
  `user_id` int(10) unsigned NOT NULL,
  `status` tinyint(3) unsigned NOT NULL DEFAULT 0 COMMENT '0=pending,1=active,2=disabled',
  `secret_ciphertext` longtext COLLATE utf8mb4_bin NOT NULL,
  `encryption_key_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `last_totp_step` bigint(20) unsigned NOT NULL DEFAULT 0,
  `verified_at` datetime DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`user_id`),
  CONSTRAINT `fk_saas_admin_mfa_user` FOREIGN KEY (`user_id`) REFERENCES `mochat_go_saas_admin_users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='SaaS Admin TOTP credential ciphertext';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_mfa_challenges` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `token_digest` binary(32) NOT NULL,
  `user_id` int(10) unsigned NOT NULL,
  `auth_version` bigint(20) unsigned NOT NULL,
  `challenge_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` tinyint(3) unsigned NOT NULL DEFAULT 0 COMMENT '0=pending,1=consumed,2=locked',
  `attempts` int(10) unsigned NOT NULL DEFAULT 0,
  `max_attempts` int(10) unsigned NOT NULL DEFAULT 5,
  `expires_at` datetime NOT NULL,
  `consumed_at` datetime DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_saas_admin_mfa_challenge_digest` (`token_digest`),
  KEY `idx_saas_admin_mfa_challenge_user_status` (`user_id`, `status`, `expires_at`),
  CONSTRAINT `fk_saas_admin_mfa_challenge_user` FOREIGN KEY (`user_id`) REFERENCES `mochat_go_saas_admin_users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='SaaS Admin one-time MFA challenge digests';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_sessions` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `jti_digest` binary(32) NOT NULL,
  `user_id` int(10) unsigned NOT NULL,
  `auth_version` bigint(20) unsigned NOT NULL,
  `status` tinyint(3) unsigned NOT NULL DEFAULT 1 COMMENT '1=active,2=revoked',
  `issued_at` datetime NOT NULL,
  `expires_at` datetime NOT NULL,
  `revoked_at` datetime DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_saas_admin_session_jti_digest` (`jti_digest`),
  KEY `idx_saas_admin_session_user_status` (`user_id`, `status`, `expires_at`),
  CONSTRAINT `fk_saas_admin_session_user` FOREIGN KEY (`user_id`) REFERENCES `mochat_go_saas_admin_users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='SaaS Admin durable JWT sessions';

-- Intentionally no FK is added to mochat_go_saas_admin_user_access here.
-- The SaaS actor FK is deferred to 0130, which owns actor backfill and must
-- preflight/resolve every actor before adding it.
