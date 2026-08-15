package archivesource

import (
	"context"
	"database/sql"
)

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// PrepareDashboardPermissionDependencies creates the smallest real 0127
// permission catalog needed by 0133's staff-resource upsert. It deliberately
// keeps the production table names, keys, and foreign key so archive
// integration schemas exercise the same dependency instead of bypassing it.
func PrepareDashboardPermissionDependencies(ctx context.Context, db execer) error {
	for _, statement := range []string{
		`CREATE TABLE mochat_go_dashboard_permissions (
  id bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  code varchar(96) NOT NULL,
  permission_type varchar(16) NOT NULL DEFAULT 'page',
  path varchar(191) NOT NULL,
  name varchar(100) NOT NULL,
  group_code varchar(64) DEFAULT NULL,
  sort int NOT NULL DEFAULT 0,
  restriction varchar(32) NOT NULL DEFAULT 'grantable',
  superadmin_only tinyint(1) NOT NULL DEFAULT 0,
  status tinyint NOT NULL DEFAULT 1,
  version bigint(20) unsigned NOT NULL DEFAULT 1,
  created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted_at timestamp NULL DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uni_dashboard_permissions_code (code),
  UNIQUE KEY uni_dashboard_permissions_path (path)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE mochat_go_dashboard_permission_resources (
  id bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  permission_id bigint(20) unsigned NOT NULL,
  resource_type varchar(16) NOT NULL DEFAULT 'api',
  http_method varchar(10) NOT NULL,
  path_pattern varchar(191) NOT NULL,
  scope_required tinyint(1) NOT NULL DEFAULT 0,
  status tinyint NOT NULL DEFAULT 1,
  version bigint(20) unsigned NOT NULL DEFAULT 1,
  created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted_at timestamp NULL DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uni_dashboard_permission_resource (http_method, path_pattern, permission_id),
  KEY idx_dashboard_permission_resource_match (http_method, path_pattern, status),
  CONSTRAINT fk_dashboard_permission_resource_permission FOREIGN KEY (permission_id) REFERENCES mochat_go_dashboard_permissions (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`INSERT INTO mochat_go_dashboard_permissions (id,code,permission_type,path,name,group_code,sort,restriction,superadmin_only,status,version) VALUES (1,'dashboard.company_setting.staff','page','/company-setting/staff','Staff permissions','company-settings',50,'superadmin_only',1,1,1)`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
