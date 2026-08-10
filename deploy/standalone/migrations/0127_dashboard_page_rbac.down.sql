DROP TABLE IF EXISTS `mochat_go_dashboard_permission_audits`;
DROP TABLE IF EXISTS `mochat_go_dashboard_user_permissions`;
DROP TABLE IF EXISTS `mochat_go_dashboard_role_permissions`;
DROP TABLE IF EXISTS `mochat_go_dashboard_user_roles`;
DROP TABLE IF EXISTS `mochat_go_dashboard_permission_resources`;
DROP TABLE IF EXISTS `mochat_go_dashboard_permissions`;

SET @dashboard_down_role_index_exists := (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = DATABASE() AND table_name = 'mc_rbac_role'
    AND index_name = 'uni_dashboard_role_tenant_id_id'
);
SET @dashboard_down_role_index_sql := IF(
  @dashboard_down_role_index_exists = 0,
  'SELECT 1',
  'ALTER TABLE `mc_rbac_role` DROP INDEX `uni_dashboard_role_tenant_id_id`'
);
PREPARE dashboard_down_role_index_stmt FROM @dashboard_down_role_index_sql;
EXECUTE dashboard_down_role_index_stmt;
DEALLOCATE PREPARE dashboard_down_role_index_stmt;

SET @dashboard_down_role_column_exists := (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'mc_rbac_role'
    AND column_name = 'dashboard_access_version'
);
SET @dashboard_down_role_column_sql := IF(
  @dashboard_down_role_column_exists = 0,
  'SELECT 1',
  'ALTER TABLE `mc_rbac_role` DROP COLUMN `dashboard_access_version`'
);
PREPARE dashboard_down_role_column_stmt FROM @dashboard_down_role_column_sql;
EXECUTE dashboard_down_role_column_stmt;
DEALLOCATE PREPARE dashboard_down_role_column_stmt;

SET @dashboard_down_user_index_exists := (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = DATABASE() AND table_name = 'mc_user'
    AND index_name = 'uni_dashboard_user_tenant_id_id'
);
SET @dashboard_down_user_index_sql := IF(
  @dashboard_down_user_index_exists = 0,
  'SELECT 1',
  'ALTER TABLE `mc_user` DROP INDEX `uni_dashboard_user_tenant_id_id`'
);
PREPARE dashboard_down_user_index_stmt FROM @dashboard_down_user_index_sql;
EXECUTE dashboard_down_user_index_stmt;
DEALLOCATE PREPARE dashboard_down_user_index_stmt;

SET @dashboard_down_user_column_exists := (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'mc_user'
    AND column_name = 'dashboard_access_version'
);
SET @dashboard_down_user_column_sql := IF(
  @dashboard_down_user_column_exists = 0,
  'SELECT 1',
  'ALTER TABLE `mc_user` DROP COLUMN `dashboard_access_version`'
);
PREPARE dashboard_down_user_column_stmt FROM @dashboard_down_user_column_sql;
EXECUTE dashboard_down_user_column_stmt;
DEALLOCATE PREPARE dashboard_down_user_column_stmt;
