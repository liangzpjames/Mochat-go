-- Identity realm and single-corp schema.
-- All data and dependency checks intentionally precede the first DDL. MySQL/MariaDB
-- implicitly commit DDL, so a dirty source database must stop before any change.

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

SET @identity_dangling_user_tenant_count := (
  SELECT COUNT(*)
  FROM `mc_user` u
  LEFT JOIN `mc_tenant` t ON t.`id` = u.`tenant_id`
  WHERE u.`tenant_id` < 0 OR t.`id` IS NULL
);
SET @identity_dangling_user_tenant_guard_sql := IF(
  @identity_dangling_user_tenant_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 dangling mc_user tenant'''
);
PREPARE identity_dangling_user_tenant_guard_stmt FROM @identity_dangling_user_tenant_guard_sql;
EXECUTE identity_dangling_user_tenant_guard_stmt;
DEALLOCATE PREPARE identity_dangling_user_tenant_guard_stmt;

SET @identity_dangling_corp_tenant_count := (
  SELECT COUNT(*)
  FROM `mc_corp` c
  LEFT JOIN `mc_tenant` t ON t.`id` = c.`tenant_id`
  WHERE c.`tenant_id` < 0 OR (c.`tenant_id` <> 0 AND t.`id` IS NULL)
);
SET @identity_dangling_corp_tenant_guard_sql := IF(
  @identity_dangling_corp_tenant_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0129 dangling mc_corp tenant'''
);
PREPARE identity_dangling_corp_tenant_guard_stmt FROM @identity_dangling_corp_tenant_guard_sql;
EXECUTE identity_dangling_corp_tenant_guard_stmt;
DEALLOCATE PREPARE identity_dangling_corp_tenant_guard_stmt;

-- Enumerate every dependency before altering tenant_id. The counts are kept as
-- explicit session facts so a future dependency cannot be silently missed.
SET @identity_tenant_dependency_fk_count := (
  SELECT COUNT(*)
  FROM `information_schema`.`key_column_usage`
  WHERE `constraint_schema` = DATABASE()
    AND `referenced_table_name` IN ('mc_user', 'mc_corp', 'mc_rbac_role')
    AND `referenced_column_name` = 'tenant_id'
);
SET @identity_tenant_dependency_index_count := (
  SELECT COUNT(*)
  FROM `information_schema`.`statistics`
  WHERE `table_schema` = DATABASE()
    AND `column_name` = 'tenant_id'
    AND `table_name` IN (
      'mc_user', 'mc_corp', 'mc_rbac_role',
      'mochat_go_dashboard_user_roles',
      'mochat_go_dashboard_role_permissions',
      'mochat_go_dashboard_user_permissions',
      'mochat_go_dashboard_permission_audits'
    )
);
SET @identity_0127_tenant_dependency_count := (
  SELECT COUNT(*)
  FROM `information_schema`.`columns`
  WHERE `table_schema` = DATABASE()
    AND `table_name` IN (
      'mochat_go_dashboard_user_roles',
      'mochat_go_dashboard_role_permissions',
      'mochat_go_dashboard_user_permissions',
      'mochat_go_dashboard_permission_audits'
    )
    AND `column_name` = 'tenant_id'
);
SELECT @identity_tenant_dependency_fk_count, @identity_tenant_dependency_index_count, @identity_0127_tenant_dependency_count;

SET @identity_drop_dashboard_user_roles_user_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `constraint_name` = 'fk_dashboard_user_roles_user') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_roles` DROP FOREIGN KEY `fk_dashboard_user_roles_user`'
);
PREPARE identity_drop_dashboard_user_roles_user_stmt FROM @identity_drop_dashboard_user_roles_user_sql;
EXECUTE identity_drop_dashboard_user_roles_user_stmt;
DEALLOCATE PREPARE identity_drop_dashboard_user_roles_user_stmt;

SET @identity_drop_dashboard_user_roles_role_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `constraint_name` = 'fk_dashboard_user_roles_role') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_roles` DROP FOREIGN KEY `fk_dashboard_user_roles_role`'
);
PREPARE identity_drop_dashboard_user_roles_role_stmt FROM @identity_drop_dashboard_user_roles_role_sql;
EXECUTE identity_drop_dashboard_user_roles_role_stmt;
DEALLOCATE PREPARE identity_drop_dashboard_user_roles_role_stmt;

SET @identity_drop_dashboard_role_permissions_role_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_role_permissions' AND `constraint_name` = 'fk_dashboard_role_permissions_role') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_role_permissions` DROP FOREIGN KEY `fk_dashboard_role_permissions_role`'
);
PREPARE identity_drop_dashboard_role_permissions_role_stmt FROM @identity_drop_dashboard_role_permissions_role_sql;
EXECUTE identity_drop_dashboard_role_permissions_role_stmt;
DEALLOCATE PREPARE identity_drop_dashboard_role_permissions_role_stmt;

SET @identity_drop_dashboard_user_permissions_user_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_permissions' AND `constraint_name` = 'fk_dashboard_user_permissions_user') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_permissions` DROP FOREIGN KEY `fk_dashboard_user_permissions_user`'
);
PREPARE identity_drop_dashboard_user_permissions_user_stmt FROM @identity_drop_dashboard_user_permissions_user_sql;
EXECUTE identity_drop_dashboard_user_permissions_user_stmt;
DEALLOCATE PREPARE identity_drop_dashboard_user_permissions_user_stmt;

SET @identity_drop_dashboard_audit_actor_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_permission_audits' AND `constraint_name` = 'fk_dashboard_audit_actor') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_permission_audits` DROP FOREIGN KEY `fk_dashboard_audit_actor`'
);
PREPARE identity_drop_dashboard_audit_actor_stmt FROM @identity_drop_dashboard_audit_actor_sql;
EXECUTE identity_drop_dashboard_audit_actor_stmt;
DEALLOCATE PREPARE identity_drop_dashboard_audit_actor_stmt;

SET @identity_drop_dashboard_role_permission_permission_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_role_permissions' AND `constraint_name` = 'fk_dashboard_role_permissions_permission') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_role_permissions` DROP FOREIGN KEY `fk_dashboard_role_permissions_permission`'
);
PREPARE identity_drop_dashboard_role_permission_permission_stmt FROM @identity_drop_dashboard_role_permission_permission_sql;
EXECUTE identity_drop_dashboard_role_permission_permission_stmt;
DEALLOCATE PREPARE identity_drop_dashboard_role_permission_permission_stmt;

ALTER TABLE `mc_user`
  MODIFY COLUMN `tenant_id` int(10) unsigned NOT NULL DEFAULT 1;

ALTER TABLE `mc_corp`
  MODIFY COLUMN `tenant_id` int(10) unsigned NULL DEFAULT 0;

ALTER TABLE `mc_rbac_role`
  MODIFY COLUMN `tenant_id` int(10) unsigned NOT NULL;

SET @identity_alter_dashboard_user_roles_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `column_name` = 'tenant_id') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_roles` MODIFY COLUMN `tenant_id` int(10) unsigned NOT NULL'
);
PREPARE identity_alter_dashboard_user_roles_stmt FROM @identity_alter_dashboard_user_roles_sql;
EXECUTE identity_alter_dashboard_user_roles_stmt;
DEALLOCATE PREPARE identity_alter_dashboard_user_roles_stmt;

SET @identity_alter_dashboard_role_permissions_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_role_permissions' AND `column_name` = 'tenant_id') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_role_permissions` MODIFY COLUMN `tenant_id` int(10) unsigned NOT NULL'
);
PREPARE identity_alter_dashboard_role_permissions_stmt FROM @identity_alter_dashboard_role_permissions_sql;
EXECUTE identity_alter_dashboard_role_permissions_stmt;
DEALLOCATE PREPARE identity_alter_dashboard_role_permissions_stmt;

SET @identity_alter_dashboard_user_permissions_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_permissions' AND `column_name` = 'tenant_id') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_permissions` MODIFY COLUMN `tenant_id` int(10) unsigned NOT NULL'
);
PREPARE identity_alter_dashboard_user_permissions_stmt FROM @identity_alter_dashboard_user_permissions_sql;
EXECUTE identity_alter_dashboard_user_permissions_stmt;
DEALLOCATE PREPARE identity_alter_dashboard_user_permissions_stmt;

SET @identity_alter_dashboard_audits_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_permission_audits' AND `column_name` = 'tenant_id') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_permission_audits` MODIFY COLUMN `tenant_id` int(10) unsigned NOT NULL'
);
PREPARE identity_alter_dashboard_audits_stmt FROM @identity_alter_dashboard_audits_sql;
EXECUTE identity_alter_dashboard_audits_stmt;
DEALLOCATE PREPARE identity_alter_dashboard_audits_stmt;

SET @identity_add_corp_composite_index_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`statistics` WHERE `table_schema` = DATABASE() AND `table_name` = 'mc_corp' AND `index_name` = 'uni_mc_corp_tenant_id_id') > 0,
  'SELECT 1',
  'ALTER TABLE `mc_corp` ADD UNIQUE KEY `uni_mc_corp_tenant_id_id` (`tenant_id`, `id`)'
);
PREPARE identity_add_corp_composite_index_stmt FROM @identity_add_corp_composite_index_sql;
EXECUTE identity_add_corp_composite_index_stmt;
DEALLOCATE PREPARE identity_add_corp_composite_index_stmt;

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
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_saas_admin_user_login_name` (`login_name`),
  UNIQUE KEY `uni_saas_admin_user_phone` (`phone`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='SaaS platform identity';

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

SET @identity_add_saas_access_fk_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`tables` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_saas_admin_user_access') = 0,
  'SELECT 1',
  IF(
    (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_saas_admin_user_access' AND `constraint_name` = 'fk_saas_admin_user_access_identity') > 0,
    'SELECT 1',
    IF(
      (SELECT COUNT(*) FROM `mochat_go_saas_admin_user_access` a LEFT JOIN `mochat_go_saas_admin_users` u ON u.`id` = a.`user_id` WHERE u.`id` IS NULL) > 0,
      'SELECT 1',
      'ALTER TABLE `mochat_go_saas_admin_user_access` ADD CONSTRAINT `fk_saas_admin_user_access_identity` FOREIGN KEY (`user_id`) REFERENCES `mochat_go_saas_admin_users` (`id`)'
    )
  )
);
PREPARE identity_add_saas_access_fk_stmt FROM @identity_add_saas_access_fk_sql;
EXECUTE identity_add_saas_access_fk_stmt;
DEALLOCATE PREPARE identity_add_saas_access_fk_stmt;

SET @identity_add_dashboard_user_roles_user_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `column_name` = 'tenant_id') = 0,
  'SELECT 1',
  IF(
    (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `constraint_name` = 'fk_dashboard_user_roles_user') > 0,
    'SELECT 1',
    'ALTER TABLE `mochat_go_dashboard_user_roles` ADD CONSTRAINT `fk_dashboard_user_roles_user` FOREIGN KEY (`tenant_id`, `user_id`) REFERENCES `mc_user` (`tenant_id`, `id`)'
  )
);
PREPARE identity_add_dashboard_user_roles_user_stmt FROM @identity_add_dashboard_user_roles_user_sql;
EXECUTE identity_add_dashboard_user_roles_user_stmt;
DEALLOCATE PREPARE identity_add_dashboard_user_roles_user_stmt;

SET @identity_add_dashboard_user_roles_role_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `column_name` = 'tenant_id') = 0,
  'SELECT 1',
  IF(
    (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `constraint_name` = 'fk_dashboard_user_roles_role') > 0,
    'SELECT 1',
    'ALTER TABLE `mochat_go_dashboard_user_roles` ADD CONSTRAINT `fk_dashboard_user_roles_role` FOREIGN KEY (`tenant_id`, `role_id`) REFERENCES `mc_rbac_role` (`tenant_id`, `id`)'
  )
);
PREPARE identity_add_dashboard_user_roles_role_stmt FROM @identity_add_dashboard_user_roles_role_sql;
EXECUTE identity_add_dashboard_user_roles_role_stmt;
DEALLOCATE PREPARE identity_add_dashboard_user_roles_role_stmt;

SET @identity_add_dashboard_role_permissions_role_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_role_permissions' AND `column_name` = 'tenant_id') = 0,
  'SELECT 1',
  IF(
    (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_role_permissions' AND `constraint_name` = 'fk_dashboard_role_permissions_role') > 0,
    'SELECT 1',
    'ALTER TABLE `mochat_go_dashboard_role_permissions` ADD CONSTRAINT `fk_dashboard_role_permissions_role` FOREIGN KEY (`tenant_id`, `role_id`) REFERENCES `mc_rbac_role` (`tenant_id`, `id`)'
  )
);
PREPARE identity_add_dashboard_role_permissions_role_stmt FROM @identity_add_dashboard_role_permissions_role_sql;
EXECUTE identity_add_dashboard_role_permissions_role_stmt;
DEALLOCATE PREPARE identity_add_dashboard_role_permissions_role_stmt;

SET @identity_add_dashboard_user_permissions_user_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_permissions' AND `column_name` = 'tenant_id') = 0,
  'SELECT 1',
  IF(
    (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_permissions' AND `constraint_name` = 'fk_dashboard_user_permissions_user') > 0,
    'SELECT 1',
    'ALTER TABLE `mochat_go_dashboard_user_permissions` ADD CONSTRAINT `fk_dashboard_user_permissions_user` FOREIGN KEY (`tenant_id`, `user_id`) REFERENCES `mc_user` (`tenant_id`, `id`)'
  )
);
PREPARE identity_add_dashboard_user_permissions_user_stmt FROM @identity_add_dashboard_user_permissions_user_sql;
EXECUTE identity_add_dashboard_user_permissions_user_stmt;
DEALLOCATE PREPARE identity_add_dashboard_user_permissions_user_stmt;

SET @identity_add_dashboard_audit_actor_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_permission_audits' AND `column_name` = 'tenant_id') = 0,
  'SELECT 1',
  IF(
    (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_permission_audits' AND `constraint_name` = 'fk_dashboard_audit_actor') > 0,
    'SELECT 1',
    'ALTER TABLE `mochat_go_dashboard_permission_audits` ADD CONSTRAINT `fk_dashboard_audit_actor` FOREIGN KEY (`tenant_id`, `actor_user_id`) REFERENCES `mc_user` (`tenant_id`, `id`)'
  )
);
PREPARE identity_add_dashboard_audit_actor_stmt FROM @identity_add_dashboard_audit_actor_sql;
EXECUTE identity_add_dashboard_audit_actor_stmt;
DEALLOCATE PREPARE identity_add_dashboard_audit_actor_stmt;
