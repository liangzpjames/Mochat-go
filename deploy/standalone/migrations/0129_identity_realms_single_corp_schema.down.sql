-- Revert only the objects and type changes introduced by 0129.
-- Every conditional statement is safe when an earlier DDL stage failed.

SET @identity_down_drop_saas_access_fk_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_saas_admin_user_access' AND `constraint_name` = 'fk_saas_admin_user_access_identity') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_saas_admin_user_access` DROP FOREIGN KEY `fk_saas_admin_user_access_identity`'
);
PREPARE identity_down_drop_saas_access_fk_stmt FROM @identity_down_drop_saas_access_fk_sql;
EXECUTE identity_down_drop_saas_access_fk_stmt;
DEALLOCATE PREPARE identity_down_drop_saas_access_fk_stmt;

SET @identity_down_drop_dashboard_user_roles_user_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `constraint_name` = 'fk_dashboard_user_roles_user') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_roles` DROP FOREIGN KEY `fk_dashboard_user_roles_user`'
);
PREPARE identity_down_drop_dashboard_user_roles_user_stmt FROM @identity_down_drop_dashboard_user_roles_user_sql;
EXECUTE identity_down_drop_dashboard_user_roles_user_stmt;
DEALLOCATE PREPARE identity_down_drop_dashboard_user_roles_user_stmt;

SET @identity_down_drop_dashboard_user_roles_role_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `constraint_name` = 'fk_dashboard_user_roles_role') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_roles` DROP FOREIGN KEY `fk_dashboard_user_roles_role`'
);
PREPARE identity_down_drop_dashboard_user_roles_role_stmt FROM @identity_down_drop_dashboard_user_roles_role_sql;
EXECUTE identity_down_drop_dashboard_user_roles_role_stmt;
DEALLOCATE PREPARE identity_down_drop_dashboard_user_roles_role_stmt;

SET @identity_down_drop_dashboard_role_permissions_role_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_role_permissions' AND `constraint_name` = 'fk_dashboard_role_permissions_role') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_role_permissions` DROP FOREIGN KEY `fk_dashboard_role_permissions_role`'
);
PREPARE identity_down_drop_dashboard_role_permissions_role_stmt FROM @identity_down_drop_dashboard_role_permissions_role_sql;
EXECUTE identity_down_drop_dashboard_role_permissions_role_stmt;
DEALLOCATE PREPARE identity_down_drop_dashboard_role_permissions_role_stmt;

SET @identity_down_drop_dashboard_user_permissions_user_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_permissions' AND `constraint_name` = 'fk_dashboard_user_permissions_user') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_permissions` DROP FOREIGN KEY `fk_dashboard_user_permissions_user`'
);
PREPARE identity_down_drop_dashboard_user_permissions_user_stmt FROM @identity_down_drop_dashboard_user_permissions_user_sql;
EXECUTE identity_down_drop_dashboard_user_permissions_user_stmt;
DEALLOCATE PREPARE identity_down_drop_dashboard_user_permissions_user_stmt;

SET @identity_down_drop_dashboard_audit_actor_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_permission_audits' AND `constraint_name` = 'fk_dashboard_audit_actor') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_permission_audits` DROP FOREIGN KEY `fk_dashboard_audit_actor`'
);
PREPARE identity_down_drop_dashboard_audit_actor_stmt FROM @identity_down_drop_dashboard_audit_actor_sql;
EXECUTE identity_down_drop_dashboard_audit_actor_stmt;
DEALLOCATE PREPARE identity_down_drop_dashboard_audit_actor_stmt;

DROP TABLE IF EXISTS `mochat_go_tenant_corp_bindings`;
DROP TABLE IF EXISTS `mochat_go_dashboard_identity_activations`;
DROP TABLE IF EXISTS `mochat_go_dashboard_identities`;
DROP TABLE IF EXISTS `mochat_go_saas_admin_users`;

SET @identity_down_restore_dashboard_audits_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_permission_audits' AND `column_name` = 'tenant_id' AND `column_type` LIKE '%unsigned%') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_permission_audits` MODIFY COLUMN `tenant_id` int(11) NOT NULL'
);
PREPARE identity_down_restore_dashboard_audits_stmt FROM @identity_down_restore_dashboard_audits_sql;
EXECUTE identity_down_restore_dashboard_audits_stmt;
DEALLOCATE PREPARE identity_down_restore_dashboard_audits_stmt;

SET @identity_down_restore_dashboard_user_permissions_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_permissions' AND `column_name` = 'tenant_id' AND `column_type` LIKE '%unsigned%') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_permissions` MODIFY COLUMN `tenant_id` int(11) NOT NULL'
);
PREPARE identity_down_restore_dashboard_user_permissions_stmt FROM @identity_down_restore_dashboard_user_permissions_sql;
EXECUTE identity_down_restore_dashboard_user_permissions_stmt;
DEALLOCATE PREPARE identity_down_restore_dashboard_user_permissions_stmt;

SET @identity_down_restore_dashboard_role_permissions_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_role_permissions' AND `column_name` = 'tenant_id' AND `column_type` LIKE '%unsigned%') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_role_permissions` MODIFY COLUMN `tenant_id` int(11) NOT NULL'
);
PREPARE identity_down_restore_dashboard_role_permissions_stmt FROM @identity_down_restore_dashboard_role_permissions_sql;
EXECUTE identity_down_restore_dashboard_role_permissions_stmt;
DEALLOCATE PREPARE identity_down_restore_dashboard_role_permissions_stmt;

SET @identity_down_restore_dashboard_user_roles_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `column_name` = 'tenant_id' AND `column_type` LIKE '%unsigned%') = 0,
  'SELECT 1',
  'ALTER TABLE `mochat_go_dashboard_user_roles` MODIFY COLUMN `tenant_id` int(11) NOT NULL'
);
PREPARE identity_down_restore_dashboard_user_roles_stmt FROM @identity_down_restore_dashboard_user_roles_sql;
EXECUTE identity_down_restore_dashboard_user_roles_stmt;
DEALLOCATE PREPARE identity_down_restore_dashboard_user_roles_stmt;

SET @identity_down_restore_role_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mc_rbac_role' AND `column_name` = 'tenant_id' AND `column_type` LIKE '%unsigned%') = 0,
  'SELECT 1',
  'ALTER TABLE `mc_rbac_role` MODIFY COLUMN `tenant_id` int(11) NOT NULL'
);
PREPARE identity_down_restore_role_stmt FROM @identity_down_restore_role_sql;
EXECUTE identity_down_restore_role_stmt;
DEALLOCATE PREPARE identity_down_restore_role_stmt;

SET @identity_down_restore_user_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mc_user' AND `column_name` = 'tenant_id' AND `column_type` LIKE '%unsigned%') = 0,
  'SELECT 1',
  'ALTER TABLE `mc_user` MODIFY COLUMN `tenant_id` int(11) NOT NULL DEFAULT 1'
);
PREPARE identity_down_restore_user_stmt FROM @identity_down_restore_user_sql;
EXECUTE identity_down_restore_user_stmt;
DEALLOCATE PREPARE identity_down_restore_user_stmt;

SET @identity_down_restore_corp_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mc_corp' AND `column_name` = 'tenant_id' AND `column_type` LIKE '%unsigned%') = 0,
  'SELECT 1',
  'ALTER TABLE `mc_corp` MODIFY COLUMN `tenant_id` int(11) NULL DEFAULT 0'
);
PREPARE identity_down_restore_corp_stmt FROM @identity_down_restore_corp_sql;
EXECUTE identity_down_restore_corp_stmt;
DEALLOCATE PREPARE identity_down_restore_corp_stmt;

SET @identity_down_drop_corp_composite_index_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`statistics` WHERE `table_schema` = DATABASE() AND `table_name` = 'mc_corp' AND `index_name` = 'uni_mc_corp_tenant_id_id') = 0,
  'SELECT 1',
  'ALTER TABLE `mc_corp` DROP INDEX `uni_mc_corp_tenant_id_id`'
);
PREPARE identity_down_drop_corp_composite_index_stmt FROM @identity_down_drop_corp_composite_index_sql;
EXECUTE identity_down_drop_corp_composite_index_stmt;
DEALLOCATE PREPARE identity_down_drop_corp_composite_index_stmt;

SET @identity_down_restore_dashboard_user_roles_user_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `column_name` = 'tenant_id') = 0,
  'SELECT 1',
  IF(
    (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `constraint_name` = 'fk_dashboard_user_roles_user') > 0,
    'SELECT 1',
    'ALTER TABLE `mochat_go_dashboard_user_roles` ADD CONSTRAINT `fk_dashboard_user_roles_user` FOREIGN KEY (`tenant_id`, `user_id`) REFERENCES `mc_user` (`tenant_id`, `id`)'
  )
);
PREPARE identity_down_restore_dashboard_user_roles_user_stmt FROM @identity_down_restore_dashboard_user_roles_user_sql;
EXECUTE identity_down_restore_dashboard_user_roles_user_stmt;
DEALLOCATE PREPARE identity_down_restore_dashboard_user_roles_user_stmt;

SET @identity_down_restore_dashboard_user_roles_role_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `column_name` = 'tenant_id') = 0,
  'SELECT 1',
  IF(
    (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_roles' AND `constraint_name` = 'fk_dashboard_user_roles_role') > 0,
    'SELECT 1',
    'ALTER TABLE `mochat_go_dashboard_user_roles` ADD CONSTRAINT `fk_dashboard_user_roles_role` FOREIGN KEY (`tenant_id`, `role_id`) REFERENCES `mc_rbac_role` (`tenant_id`, `id`)'
  )
);
PREPARE identity_down_restore_dashboard_user_roles_role_stmt FROM @identity_down_restore_dashboard_user_roles_role_sql;
EXECUTE identity_down_restore_dashboard_user_roles_role_stmt;
DEALLOCATE PREPARE identity_down_restore_dashboard_user_roles_role_stmt;

SET @identity_down_restore_dashboard_role_permissions_role_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_role_permissions' AND `column_name` = 'tenant_id') = 0,
  'SELECT 1',
  IF(
    (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_role_permissions' AND `constraint_name` = 'fk_dashboard_role_permissions_role') > 0,
    'SELECT 1',
    'ALTER TABLE `mochat_go_dashboard_role_permissions` ADD CONSTRAINT `fk_dashboard_role_permissions_role` FOREIGN KEY (`tenant_id`, `role_id`) REFERENCES `mc_rbac_role` (`tenant_id`, `id`)'
  )
);
PREPARE identity_down_restore_dashboard_role_permissions_role_stmt FROM @identity_down_restore_dashboard_role_permissions_role_sql;
EXECUTE identity_down_restore_dashboard_role_permissions_role_stmt;
DEALLOCATE PREPARE identity_down_restore_dashboard_role_permissions_role_stmt;

SET @identity_down_restore_dashboard_user_permissions_user_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_permissions' AND `column_name` = 'tenant_id') = 0,
  'SELECT 1',
  IF(
    (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_user_permissions' AND `constraint_name` = 'fk_dashboard_user_permissions_user') > 0,
    'SELECT 1',
    'ALTER TABLE `mochat_go_dashboard_user_permissions` ADD CONSTRAINT `fk_dashboard_user_permissions_user` FOREIGN KEY (`tenant_id`, `user_id`) REFERENCES `mc_user` (`tenant_id`, `id`)'
  )
);
PREPARE identity_down_restore_dashboard_user_permissions_user_stmt FROM @identity_down_restore_dashboard_user_permissions_user_sql;
EXECUTE identity_down_restore_dashboard_user_permissions_user_stmt;
DEALLOCATE PREPARE identity_down_restore_dashboard_user_permissions_user_stmt;

SET @identity_down_restore_dashboard_audit_actor_sql := IF(
  (SELECT COUNT(*) FROM `information_schema`.`columns` WHERE `table_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_permission_audits' AND `column_name` = 'tenant_id') = 0,
  'SELECT 1',
  IF(
    (SELECT COUNT(*) FROM `information_schema`.`table_constraints` WHERE `constraint_schema` = DATABASE() AND `table_name` = 'mochat_go_dashboard_permission_audits' AND `constraint_name` = 'fk_dashboard_audit_actor') > 0,
    'SELECT 1',
    'ALTER TABLE `mochat_go_dashboard_permission_audits` ADD CONSTRAINT `fk_dashboard_audit_actor` FOREIGN KEY (`tenant_id`, `actor_user_id`) REFERENCES `mc_user` (`tenant_id`, `id`)'
  )
);
PREPARE identity_down_restore_dashboard_audit_actor_stmt FROM @identity_down_restore_dashboard_audit_actor_sql;
EXECUTE identity_down_restore_dashboard_audit_actor_stmt;
DEALLOCATE PREPARE identity_down_restore_dashboard_audit_actor_stmt;
