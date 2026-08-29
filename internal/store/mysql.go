package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/aiproviderconfig"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/outboundhttp"
	"jiyi/mochat-go/internal/saasalertcredentials"
	"jiyi/mochat-go/internal/wechatopencredentials"
	"jiyi/mochat-go/internal/wecomcredentials"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type MySQLStore struct {
	db                            *sql.DB
	corpDataExecutor              corpDataQueryExecutor
	dashboardTenantAccessQueryRow dashboardTenantAccessQueryRowFunc
	dashboardAccessQueryRow       dashboardAccessQueryRowFunc
	dashboardAccessQuery          dashboardAccessQueryFunc
	dashboardAccessAdminBegin     dashboardAccessAdminBeginFunc
	saasAlertCredentialCipher     *saasalertcredentials.Manager
	weComCredentialCipher         *wecomcredentials.Manager
	weChatOpenCredentialCipher    *wechatopencredentials.Manager
	aiProviderCredentialCipher    *aiproviderconfig.Manager
	aiProviderOutboundGuard       *outboundhttp.Guard
}

type corpDataQueryExecutor interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func isMySQLDuplicateKeyError(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func isMySQLRetryableTransactionError(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && (mysqlErr.Number == 1205 || mysqlErr.Number == 1213)
}

type SaaSStorageReconcileResult struct {
	TenantID          int
	Scanned           int
	MissingMarked     int
	SizeUpdated       int
	UnsafeMarked      int
	CountersRefreshed int
	RefreshedTenants  []int
}

type SaaSUsageRefreshResult struct {
	TenantID         int
	TenantsScanned   int
	MetricsRefreshed int
	RefreshedTenants []int
}

type SaaSAlert = dashboard.SaaSAlertRecord
type SaaSAlertListOptions = dashboard.SaaSAlertListOptions
type SaaSAlertListPage = dashboard.SaaSAlertListPage
type SaaSAlertNotification = dashboard.SaaSAlertNotification
type SaaSAlertSetting = dashboard.SaaSAlertSetting

type saasUsageLimitSnapshot struct {
	MaxCorps              int64 `json:"maxCorps"`
	MaxUsers              int64 `json:"maxUsers"`
	MaxContacts           int64 `json:"maxContacts"`
	MaxRooms              int64 `json:"maxRooms"`
	MaxAgents             int64 `json:"maxAgents"`
	ChannelCodes          int64 `json:"channelCodes"`
	ShopCodes             int64 `json:"shopCodes"`
	Radars                int64 `json:"radars"`
	Lotteries             int64 `json:"lotteries"`
	RoomInfinitePulls     int64 `json:"roomInfinitePulls"`
	RoomFissions          int64 `json:"roomFissions"`
	RoomClockIns          int64 `json:"roomClockIns"`
	RoomQualities         int64 `json:"roomQualities"`
	RoomCalendars         int64 `json:"roomCalendars"`
	RoomReminds           int64 `json:"roomReminds"`
	ContactSOPs           int64 `json:"contactSops"`
	RoomSOPs              int64 `json:"roomSops"`
	SensitiveWords        int64 `json:"sensitiveWords"`
	StorageMB             int64 `json:"storageMb"`
	ContactMessageBatches int64 `json:"contactMessageBatches"`
	RoomMessageBatches    int64 `json:"roomMessageBatches"`
	RoomTagPulls          int64 `json:"roomTagPulls"`
	WorkRoomAutoPulls     int64 `json:"workRoomAutoPulls"`
	WorkFissions          int64 `json:"workFissions"`
	OfficialAccounts      int64 `json:"officialAccounts"`
	AsyncExecutions       int64 `json:"asyncExecutions"`
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	store := &MySQLStore{db: db, corpDataExecutor: db}
	store.dashboardTenantAccessQueryRow = func(ctx context.Context, query string, args ...any) dashboardTenantAccessRow {
		return db.QueryRowContext(ctx, query, args...)
	}
	store.dashboardAccessQueryRow = func(ctx context.Context, query string, args ...any) dashboardTenantAccessRow {
		return db.QueryRowContext(ctx, query, args...)
	}
	store.dashboardAccessQuery = func(ctx context.Context, query string, args ...any) (dashboardAccessRows, error) {
		return db.QueryContext(ctx, query, args...)
	}
	return store
}

func (s *MySQLStore) WithSaaSAlertCredentialCipher(cipher *saasalertcredentials.Manager) *MySQLStore {
	if s != nil {
		s.saasAlertCredentialCipher = cipher
	}
	return s
}

func (s *MySQLStore) WithWeComCredentialCipher(cipher *wecomcredentials.Manager) *MySQLStore {
	if s != nil {
		s.weComCredentialCipher = cipher
	}
	return s
}

func (s *MySQLStore) WithWeChatOpenCredentialCipher(cipher *wechatopencredentials.Manager) *MySQLStore {
	if s != nil {
		s.weChatOpenCredentialCipher = cipher
	}
	return s
}

func (s *MySQLStore) DB() *sql.DB {
	return s.db
}

func (s *MySQLStore) UserByID(ctx context.Context, userID int) (dashboard.User, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			u.id, u.phone, u.name, u.gender, u.department, u.position, u.login_time, u.status,
			u.tenant_id, COALESCE(t.status, 1) AS tenant_status, u.isSuperAdmin
		FROM mc_user u
		LEFT JOIN mc_tenant t ON t.id = u.tenant_id AND t.deleted_at IS NULL
		WHERE u.id = ? AND u.deleted_at IS NULL
	`, userID)

	var user dashboard.User
	var loginTime sql.NullTime
	err := row.Scan(&user.ID, &user.Phone, &user.Name, &user.Gender, &user.Department, &user.Position, &loginTime, &user.Status, &user.TenantID, &user.TenantStatus, &user.IsSuperAdmin)
	if err == sql.ErrNoRows {
		return dashboard.User{}, false, nil
	}
	if err != nil {
		return dashboard.User{}, false, err
	}
	if user.TenantStatus == 2 {
		return dashboard.User{}, false, nil
	}
	managed, subscriptionStatus, subscriptionAllowed, graceEndsAt, err := s.saasTenantSubscriptionAccess(ctx, user.TenantID)
	if err != nil {
		return dashboard.User{}, false, err
	}
	if managed {
		user.TenantSubscriptionManaged = true
		user.TenantSubscriptionStatus = subscriptionStatus
		user.TenantSubscriptionAccessAllowed = subscriptionAllowed
		user.TenantSubscriptionGraceEndsAt = graceEndsAt
		if !subscriptionAllowed {
			return dashboard.User{}, false, nil
		}
		user.LoginTime = formatTime(loginTime)
		return user, true, nil
	}
	expired, expiresAt, err := s.saasTenantPackageExpired(ctx, user.TenantID)
	if err != nil {
		return dashboard.User{}, false, err
	}
	user.TenantPackageExpired = expired
	user.TenantPackageExpiresAt = expiresAt
	if expired {
		return dashboard.User{}, false, nil
	}
	user.LoginTime = formatTime(loginTime)
	return user, true, nil
}

func (s *MySQLStore) UserAuthByPhone(ctx context.Context, phone string) (dashboard.AuthUser, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.name, u.phone, u.status, u.password, u.tenant_id, COALESCE(t.status, 1) AS tenant_status
		FROM mc_user u
		LEFT JOIN mc_tenant t ON t.id = u.tenant_id AND t.deleted_at IS NULL
		WHERE u.phone = ? AND u.deleted_at IS NULL
		LIMIT 1
	`, phone)

	var user dashboard.AuthUser
	err := row.Scan(&user.ID, &user.Name, &user.Phone, &user.Status, &user.Password, &user.TenantID, &user.TenantStatus)
	if err == sql.ErrNoRows {
		return dashboard.AuthUser{}, false, nil
	}
	if err != nil {
		return dashboard.AuthUser{}, false, err
	}
	managed, subscriptionStatus, subscriptionAllowed, graceEndsAt, err := s.saasTenantSubscriptionAccess(ctx, user.TenantID)
	if err != nil {
		return dashboard.AuthUser{}, false, err
	}
	if managed {
		user.TenantSubscriptionManaged = true
		user.TenantSubscriptionStatus = subscriptionStatus
		user.TenantSubscriptionAccessAllowed = subscriptionAllowed
		user.TenantSubscriptionGraceEndsAt = graceEndsAt
		return user, true, nil
	}
	expired, expiresAt, err := s.saasTenantPackageExpired(ctx, user.TenantID)
	if err != nil {
		return dashboard.AuthUser{}, false, err
	}
	user.TenantPackageExpired = expired
	user.TenantPackageExpiresAt = expiresAt
	return user, true, nil
}

func (s *MySQLStore) UserAuthByTenantPhone(ctx context.Context, tenantID int, phone string) (dashboard.AuthUser, bool, error) {
	if tenantID <= 0 {
		return dashboard.AuthUser{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.name, u.phone, u.status, u.password, u.tenant_id, COALESCE(t.status, 1) AS tenant_status
		FROM mc_user u
		LEFT JOIN mc_tenant t ON t.id = u.tenant_id AND t.deleted_at IS NULL
		WHERE u.tenant_id = ? AND u.phone = ? AND u.deleted_at IS NULL
		LIMIT 1
	`, tenantID, phone)

	var user dashboard.AuthUser
	err := row.Scan(&user.ID, &user.Name, &user.Phone, &user.Status, &user.Password, &user.TenantID, &user.TenantStatus)
	if err == sql.ErrNoRows {
		return dashboard.AuthUser{}, false, nil
	}
	if err != nil {
		return dashboard.AuthUser{}, false, err
	}
	managed, subscriptionStatus, subscriptionAllowed, graceEndsAt, err := s.saasTenantSubscriptionAccess(ctx, user.TenantID)
	if err != nil {
		return dashboard.AuthUser{}, false, err
	}
	if managed {
		user.TenantSubscriptionManaged = true
		user.TenantSubscriptionStatus = subscriptionStatus
		user.TenantSubscriptionAccessAllowed = subscriptionAllowed
		user.TenantSubscriptionGraceEndsAt = graceEndsAt
		return user, true, nil
	}
	expired, expiresAt, err := s.saasTenantPackageExpired(ctx, user.TenantID)
	if err != nil {
		return dashboard.AuthUser{}, false, err
	}
	user.TenantPackageExpired = expired
	user.TenantPackageExpiresAt = expiresAt
	return user, true, nil
}

func (s *MySQLStore) UserAuthByID(ctx context.Context, userID int) (dashboard.AuthUser, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.name, u.phone, u.status, u.password, u.tenant_id, COALESCE(t.status, 1) AS tenant_status
		FROM mc_user u
		LEFT JOIN mc_tenant t ON t.id = u.tenant_id AND t.deleted_at IS NULL
		WHERE u.id = ? AND u.deleted_at IS NULL
		LIMIT 1
	`, userID)

	var user dashboard.AuthUser
	err := row.Scan(&user.ID, &user.Name, &user.Phone, &user.Status, &user.Password, &user.TenantID, &user.TenantStatus)
	if err == sql.ErrNoRows {
		return dashboard.AuthUser{}, false, nil
	}
	if err != nil {
		return dashboard.AuthUser{}, false, err
	}
	managed, subscriptionStatus, subscriptionAllowed, graceEndsAt, err := s.saasTenantSubscriptionAccess(ctx, user.TenantID)
	if err != nil {
		return dashboard.AuthUser{}, false, err
	}
	if managed {
		user.TenantSubscriptionManaged = true
		user.TenantSubscriptionStatus = subscriptionStatus
		user.TenantSubscriptionAccessAllowed = subscriptionAllowed
		user.TenantSubscriptionGraceEndsAt = graceEndsAt
		return user, true, nil
	}
	expired, expiresAt, err := s.saasTenantPackageExpired(ctx, user.TenantID)
	if err != nil {
		return dashboard.AuthUser{}, false, err
	}
	user.TenantPackageExpired = expired
	user.TenantPackageExpiresAt = expiresAt
	return user, true, nil
}

func (s *MySQLStore) saasTenantPackageExpired(ctx context.Context, tenantID int) (bool, string, error) {
	if tenantID <= 0 {
		return false, "", nil
	}
	var expiresAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(DATE_FORMAT(expires_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_tenant_packages
		WHERE tenant_id = ?
			AND status = 1
			AND deleted_at IS NULL
			AND expires_at IS NOT NULL
			AND expires_at < NOW()
		ORDER BY expires_at ASC
		LIMIT 1
	`, tenantID).Scan(&expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		if isMissingSaaSTableError(err) {
			return false, "", nil
		}
		return false, "", err
	}
	return true, expiresAt, nil
}

func (s *MySQLStore) UserAdminIDsByTenant(ctx context.Context, tenantID int) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM mc_user
		WHERE tenant_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) UserAdminLogUserIDsByCorp(ctx context.Context, corpID int) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT log_user_id
		FROM mc_work_employee
		WHERE corp_id = ? AND log_user_id > 0 AND deleted_at IS NULL
		ORDER BY log_user_id ASC
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) UserAdminLogUserIDsByEmployees(ctx context.Context, employeeIDs []int) ([]int, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return []int{}, nil
	}
	args := make([]any, 0, len(employeeIDs))
	for _, employeeID := range employeeIDs {
		args = append(args, employeeID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT log_user_id
		FROM mc_work_employee
		WHERE id IN (`+placeholders(len(employeeIDs))+`) AND log_user_id > 0 AND deleted_at IS NULL
		ORDER BY log_user_id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) UserAdminStatusCounts(ctx context.Context, userIDs []int) (dashboard.UserAdminStatusCounts, error) {
	userIDs = uniquePositiveInts(userIDs)
	if len(userIDs) == 0 {
		return dashboard.UserAdminStatusCounts{}, nil
	}
	args := make([]any, 0, len(userIDs))
	for _, userID := range userIDs {
		args = append(args, userID)
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN status = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 2 THEN 1 ELSE 0 END), 0)
		FROM mc_user
		WHERE id IN (`+placeholders(len(userIDs))+`) AND deleted_at IS NULL
	`, args...)
	var counts dashboard.UserAdminStatusCounts
	if err := row.Scan(&counts.NotEnabled, &counts.Normal, &counts.Disable); err != nil {
		return dashboard.UserAdminStatusCounts{}, err
	}
	return counts, nil
}

func (s *MySQLStore) UserAdminPage(ctx context.Context, filter dashboard.UserAdminFilter) (dashboard.UserAdminPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10
	}
	filter.UserIDs = uniquePositiveInts(filter.UserIDs)
	if filter.RestrictUserID && len(filter.UserIDs) == 0 {
		return dashboard.UserAdminPage{Items: []dashboard.UserAdminItem{}, PerPage: filter.PerPage}, nil
	}

	where, args := userAdminWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_user WHERE "+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return dashboard.UserAdminPage{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	if total == 0 {
		return dashboard.UserAdminPage{Items: []dashboard.UserAdminItem{}, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
	}

	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, phone, name, gender, department, position, login_time, status, tenant_id, isSuperAdmin, created_at, updated_at
		FROM mc_user
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.UserAdminPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.UserAdminItem, 0)
	for rows.Next() {
		item, err := scanUserAdminItem(rows)
		if err != nil {
			return dashboard.UserAdminPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.UserAdminPage{}, err
	}
	return dashboard.UserAdminPage{Items: items, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) UserAdminByID(ctx context.Context, userID int) (dashboard.UserAdminItem, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, phone, name, gender, department, position, login_time, status, tenant_id, isSuperAdmin, created_at, updated_at
		FROM mc_user
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, userID)
	item, err := scanUserAdminItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.UserAdminItem{}, false, nil
	}
	if err != nil {
		return dashboard.UserAdminItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) UserAdminRolesByUserIDs(ctx context.Context, userIDs []int) (map[int]dashboard.UserAdminRoleInfo, error) {
	userIDs = uniquePositiveInts(userIDs)
	result := make(map[int]dashboard.UserAdminRoleInfo, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(userIDs))
	for _, userID := range userIDs {
		args = append(args, userID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT ur.user_id, r.id, r.name
		FROM mc_rbac_user_role AS ur
		INNER JOIN mc_rbac_role AS r ON r.id = ur.role_id AND r.deleted_at IS NULL
		WHERE ur.user_id IN (`+placeholders(len(userIDs))+`) AND ur.deleted_at IS NULL
		ORDER BY ur.id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var userID int
		var role dashboard.UserAdminRoleInfo
		var roleName sql.NullString
		if err := rows.Scan(&userID, &role.RoleID, &roleName); err != nil {
			return nil, err
		}
		if _, exists := result[userID]; !exists {
			role.RoleName = nullString(roleName)
			result[userID] = role
		}
	}
	return result, rows.Err()
}

func (s *MySQLStore) UserAdminDepartmentsByUserIDs(ctx context.Context, corpID int, userIDs []int) (map[int][]dashboard.UserAdminDepartment, error) {
	userIDs = uniquePositiveInts(userIDs)
	result := make(map[int][]dashboard.UserAdminDepartment, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(userIDs)+1)
	args = append(args, corpID)
	for _, userID := range userIDs {
		args = append(args, userID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.log_user_id, d.id, d.name
		FROM mc_work_employee AS e
		INNER JOIN mc_work_employee_department AS ed ON ed.employee_id = e.id AND ed.deleted_at IS NULL
		INNER JOIN mc_work_department AS d ON d.id = ed.department_id AND d.deleted_at IS NULL
		WHERE e.corp_id = ? AND e.log_user_id IN (`+placeholders(len(userIDs))+`) AND e.deleted_at IS NULL
			AND e.id = (
				SELECT MIN(e2.id)
				FROM mc_work_employee AS e2
				WHERE e2.corp_id = e.corp_id AND e2.log_user_id = e.log_user_id AND e2.deleted_at IS NULL
			)
		ORDER BY e.log_user_id ASC, d.id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var userID int
		var department dashboard.UserAdminDepartment
		var departmentName sql.NullString
		if err := rows.Scan(&userID, &department.DepartmentID, &departmentName); err != nil {
			return nil, err
		}
		department.DepartmentName = nullString(departmentName)
		result[userID] = append(result[userID], department)
	}
	return result, rows.Err()
}

func (s *MySQLStore) UserAdminIDsByPhone(ctx context.Context, phone string) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM mc_user
		WHERE phone = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`, phone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) CreateUserAdmin(ctx context.Context, values dashboard.UserAdminWrite, corpID int) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer rollbackQuietly(tx)

	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_user (phone, password, name, gender, department, status, tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, values.Phone, values.Password, values.Name, values.Gender, values.Department, values.Status, values.TenantID)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_employee
		SET log_user_id = ?, updated_at = NOW()
		WHERE corp_id = ? AND mobile = ? AND deleted_at IS NULL
	`, int(id), corpID, values.Phone); err != nil {
		return 0, err
	}
	if values.RoleID > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_rbac_user_role (user_id, role_id, created_at, updated_at)
			VALUES (?, ?, NOW(), NOW())
		`, int(id), values.RoleID); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *MySQLStore) UpdateUserAdmin(ctx context.Context, userID int, values dashboard.UserAdminWrite, corpID int) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)

	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_user
		SET phone = ?, name = ?, gender = ?, department = ?, status = ?, updated_at = NOW()
		WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
	`, values.Phone, values.Name, values.Gender, values.Department, values.Status, userID, values.TenantID); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_employee
		SET log_user_id = ?, updated_at = NOW()
		WHERE corp_id = ? AND mobile = ? AND deleted_at IS NULL
	`, userID, corpID, values.Phone); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_rbac_user_role
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE user_id = ? AND deleted_at IS NULL
	`, userID); err != nil {
		return false, err
	}
	if values.RoleID > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_rbac_user_role (user_id, role_id, created_at, updated_at)
			VALUES (?, ?, NOW(), NOW())
		`, userID, values.RoleID); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) UserAdminItemsByIDs(ctx context.Context, userIDs []int) ([]dashboard.UserAdminItem, error) {
	userIDs = uniquePositiveInts(userIDs)
	if len(userIDs) == 0 {
		return []dashboard.UserAdminItem{}, nil
	}
	args := make([]any, 0, len(userIDs))
	for _, userID := range userIDs {
		args = append(args, userID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, phone, name, gender, department, position, login_time, status, tenant_id, isSuperAdmin, created_at, updated_at
		FROM mc_user
		WHERE id IN (`+placeholders(len(userIDs))+`) AND deleted_at IS NULL
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.UserAdminItem, 0)
	for rows.Next() {
		item, err := scanUserAdminItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) UpdateUserAdminStatuses(ctx context.Context, userIDs []int, status int) error {
	userIDs = uniquePositiveInts(userIDs)
	if len(userIDs) == 0 {
		return nil
	}
	args := make([]any, 0, len(userIDs)+1)
	args = append(args, status)
	for _, userID := range userIDs {
		args = append(args, userID)
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_user
		SET status = ?, updated_at = NOW()
		WHERE id IN (`+placeholders(len(userIDs))+`) AND deleted_at IS NULL
	`, args...)
	return err
}

func (s *MySQLStore) UpdateUserAdminPassword(ctx context.Context, userID int, passwordHash string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_user
		SET password = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, passwordHash, userID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func userAdminWhere(filter dashboard.UserAdminFilter) ([]string, []any) {
	where := []string{"tenant_id = ?", "deleted_at IS NULL"}
	args := []any{filter.TenantID}
	if filter.RestrictUserID {
		where = append(where, "id IN ("+placeholders(len(filter.UserIDs))+")")
		for _, userID := range filter.UserIDs {
			args = append(args, userID)
		}
	}
	if filter.Phone != "" {
		where = append(where, "phone LIKE ?")
		args = append(args, "%"+filter.Phone+"%")
	}
	if filter.Status != nil {
		where = append(where, "status = ?")
		args = append(args, *filter.Status)
	}
	return where, args
}

func scanUserAdminItem(scanner mediumScanner) (dashboard.UserAdminItem, error) {
	var item dashboard.UserAdminItem
	var phone, name, department, position sql.NullString
	var loginTime, createdAt, updatedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&phone,
		&name,
		&item.Gender,
		&department,
		&position,
		&loginTime,
		&item.Status,
		&item.TenantID,
		&item.IsSuperAdmin,
		&createdAt,
		&updatedAt,
	); err != nil {
		return dashboard.UserAdminItem{}, err
	}
	item.Phone = nullString(phone)
	item.Name = nullString(name)
	item.Department = nullString(department)
	item.Position = nullString(position)
	item.LoginTime = formatTime(loginTime)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func (s *MySQLStore) SidebarEmployeeByID(ctx context.Context, employeeID int) (dashboard.SidebarEmployee, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, log_user_id
		FROM mc_work_employee
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, employeeID)

	var employee dashboard.SidebarEmployee
	err := row.Scan(&employee.ID, &employee.CorpID, &employee.LogUserID)
	if err == sql.ErrNoRows {
		return dashboard.SidebarEmployee{}, false, nil
	}
	if err != nil {
		return dashboard.SidebarEmployee{}, false, err
	}
	return employee, true, nil
}

func (s *MySQLStore) ContactBatchAddDetail(ctx context.Context, employeeID int, batchID int, status int) (dashboard.ContactBatchAddDetail, error) {
	employee, found, err := s.sidebarEmployeeIdentity(ctx, employeeID)
	if err != nil || !found {
		return dashboard.ContactBatchAddDetail{}, err
	}

	var employeeName sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT name
		FROM mc_work_employee
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, employee.ID, employee.CorpID).Scan(&employeeName)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.ContactBatchAddDetail{}, nil
	}
	if err != nil {
		return dashboard.ContactBatchAddDetail{}, err
	}

	query := `
		SELECT id, phone, status
		FROM mc_contact_batch_add_import
		WHERE corp_id = ?
		  AND employee_id = ?
		  AND record_id = ?
		  AND deleted_at IS NULL
	`
	args := []any{employee.CorpID, employee.ID, batchID}
	if status != 4 {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY id ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return dashboard.ContactBatchAddDetail{}, err
	}
	defer rows.Close()

	detail := dashboard.ContactBatchAddDetail{
		EmployeeName: nullString(employeeName),
		List:         []dashboard.ContactBatchAddContact{},
	}
	for rows.Next() {
		var item dashboard.ContactBatchAddContact
		var phone sql.NullString
		if err := rows.Scan(&item.ID, &phone, &item.Status); err != nil {
			return dashboard.ContactBatchAddDetail{}, err
		}
		item.Phone = nullString(phone)
		detail.List = append(detail.List, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ContactBatchAddDetail{}, err
	}
	return detail, nil
}

func (s *MySQLStore) ContactBatchAddPage(ctx context.Context, filter dashboard.ContactBatchAddFilter) (dashboard.ContactBatchAddPage, error) {
	if filter.CorpID <= 0 {
		return dashboard.ContactBatchAddPage{Items: []dashboard.ContactBatchAddItem{}, Page: filter.Page, PerPage: filter.PerPage}, nil
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 15
	}
	where, args := contactBatchAddWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_contact_batch_add_import i LEFT JOIN mc_work_employee e ON e.id = i.employee_id AND e.deleted_at IS NULL WHERE "+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return dashboard.ContactBatchAddPage{}, err
	}
	offset := (filter.Page - 1) * filter.PerPage
	query := `
		SELECT i.id, i.record_id, i.phone, i.upload_at, i.status, i.add_at, i.employee_id, i.allot_num, i.remark, i.tags
		FROM mc_contact_batch_add_import i
		LEFT JOIN mc_work_employee e ON e.id = i.employee_id AND e.deleted_at IS NULL
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY i.id DESC
		LIMIT ? OFFSET ?
	`
	queryArgs := append(append([]any{}, args...), filter.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return dashboard.ContactBatchAddPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.ContactBatchAddItem, 0)
	for rows.Next() {
		item, err := scanContactBatchAddItem(rows)
		if err != nil {
			return dashboard.ContactBatchAddPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ContactBatchAddPage{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	return dashboard.ContactBatchAddPage{Items: items, Total: total, TotalPage: totalPage, PerPage: filter.PerPage, Page: filter.Page}, nil
}

func (s *MySQLStore) ContactBatchAddImportRecordPage(ctx context.Context, corpID int, page int, perPage int) (dashboard.ContactBatchAddImportRecordPage, error) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 15
	}
	if corpID <= 0 {
		return dashboard.ContactBatchAddImportRecordPage{Items: []dashboard.ContactBatchAddImportRecord{}, Page: page, PerPage: perPage}, nil
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_contact_batch_add_import_record
		WHERE corp_id = ? AND deleted_at IS NULL
	`, corpID).Scan(&total); err != nil {
		return dashboard.ContactBatchAddImportRecordPage{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, corp_id, title, upload_at, allot_employee, tags, import_num, add_num, COALESCE(file_name, ''), COALESCE(file_url, '')
		FROM mc_contact_batch_add_import_record
		WHERE corp_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, corpID, perPage, (page-1)*perPage)
	if err != nil {
		return dashboard.ContactBatchAddImportRecordPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.ContactBatchAddImportRecord, 0)
	for rows.Next() {
		var item dashboard.ContactBatchAddImportRecord
		var uploadAt sql.NullTime
		var rawEmployees, rawTags []byte
		if err := rows.Scan(&item.ID, &item.CorpID, &item.Title, &uploadAt, &rawEmployees, &rawTags, &item.ImportNum, &item.AddNum, &item.FileName, &item.FileURL); err != nil {
			return dashboard.ContactBatchAddImportRecordPage{}, err
		}
		item.UploadAt = formatTime(uploadAt)
		item.EmployeeIDs = contactBatchAddIntSliceFromJSON(rawEmployees)
		item.TagIDs = contactBatchAddIntSliceFromJSON(rawTags)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ContactBatchAddImportRecordPage{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + perPage - 1) / perPage
	}
	return dashboard.ContactBatchAddImportRecordPage{Items: items, Total: total, TotalPage: totalPage, PerPage: perPage, Page: page}, nil
}

func (s *MySQLStore) ContactBatchAddEmployeesByIDs(ctx context.Context, employeeIDs []int) (map[int]dashboard.ContactBatchAddEmployee, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	result := make(map[int]dashboard.ContactBatchAddEmployee, len(employeeIDs))
	if len(employeeIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(employeeIDs))
	for _, id := range employeeIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(name, '')
		FROM mc_work_employee
		WHERE id IN (`+placeholders(len(employeeIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var employee dashboard.ContactBatchAddEmployee
		if err := rows.Scan(&employee.ID, &employee.Name); err != nil {
			return nil, err
		}
		result[employee.ID] = employee
	}
	return result, rows.Err()
}

func (s *MySQLStore) ContactBatchAddTagsByIDs(ctx context.Context, corpID int, tagIDs []int) (map[int]dashboard.ContactBatchAddTag, error) {
	tagIDs = uniquePositiveInts(tagIDs)
	result := make(map[int]dashboard.ContactBatchAddTag, len(tagIDs))
	if len(tagIDs) == 0 {
		return result, nil
	}
	args := []any{corpID}
	for _, id := range tagIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(name, '')
		FROM mc_work_contact_tag
		WHERE corp_id = ? AND id IN (`+placeholders(len(tagIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var tag dashboard.ContactBatchAddTag
		if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
			return nil, err
		}
		result[tag.ID] = tag
	}
	return result, rows.Err()
}

func (s *MySQLStore) ContactBatchAddConfig(ctx context.Context, corpID int) (dashboard.ContactBatchAddConfig, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT pending_status, pending_time_out, pending_reminder_time, pending_leader_id, undone_status, undone_time_out, undone_reminder_time, recycle_status, recycle_time_out
		FROM mc_contact_batch_add_config
		WHERE corp_id = ?
		LIMIT 1
	`, corpID)
	var config dashboard.ContactBatchAddConfig
	var pendingTime, undoneTime string
	err := row.Scan(&config.PendingStatus, &config.PendingTimeOut, &pendingTime, &config.PendingLeaderID, &config.UndoneStatus, &config.UndoneTimeOut, &undoneTime, &config.RecycleStatus, &config.RecycleTimeOut)
	if errors.Is(err, sql.ErrNoRows) {
		config.PendingReminderTime = "00:00:00"
		config.UndoneReminderTime = "00:00:00"
		return config, false, nil
	}
	if err != nil {
		return dashboard.ContactBatchAddConfig{}, false, err
	}
	config.PendingReminderTime = pendingTime
	config.UndoneReminderTime = undoneTime
	return config, true, nil
}

func (s *MySQLStore) UpsertContactBatchAddConfig(ctx context.Context, corpID int, values dashboard.ContactBatchAddConfig) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_contact_batch_add_config
		SET pending_status = ?,
		    pending_time_out = ?,
		    pending_reminder_time = ?,
		    pending_leader_id = ?,
		    undone_status = ?,
		    undone_time_out = ?,
		    undone_reminder_time = ?,
		    recycle_status = ?,
		    recycle_time_out = ?,
		    updated_at = NOW()
		WHERE corp_id = ?
	`, values.PendingStatus, values.PendingTimeOut, values.PendingReminderTime, values.PendingLeaderID, values.UndoneStatus, values.UndoneTimeOut, values.UndoneReminderTime, values.RecycleStatus, values.RecycleTimeOut, corpID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO mc_contact_batch_add_config
			(corp_id, pending_status, pending_time_out, pending_reminder_time, pending_leader_id, undone_status, undone_time_out, undone_reminder_time, recycle_status, recycle_time_out, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, corpID, values.PendingStatus, values.PendingTimeOut, values.PendingReminderTime, values.PendingLeaderID, values.UndoneStatus, values.UndoneTimeOut, values.UndoneReminderTime, values.RecycleStatus, values.RecycleTimeOut)
	return err
}

func (s *MySQLStore) CreateContactBatchAddImport(ctx context.Context, values dashboard.ContactBatchAddImportWrite) (int, error) {
	rows := values.Rows
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now()
	employeesJSON := mustJSONStore(uniquePositiveInts(values.EmployeeIDs))
	tagsJSON := mustJSONStore(uniquePositiveInts(values.TagIDs))
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_contact_batch_add_import_record
			(corp_id, title, upload_at, allot_employee, tags, import_num, add_num, file_name, file_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?)
	`, values.CorpID, values.Title, now, employeesJSON, tagsJSON, len(rows), values.FileName, values.FileURL, now, now)
	if err != nil {
		return 0, err
	}
	recordID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	employeeIDs := uniquePositiveInts(values.EmployeeIDs)
	for index, item := range rows {
		employeeID := 0
		status := 0
		allotNum := 0
		if len(employeeIDs) > 0 {
			employeeID = employeeIDs[index%len(employeeIDs)]
			status = 1
			allotNum = 1
		}
		insert, err := tx.ExecContext(ctx, `
			INSERT INTO mc_contact_batch_add_import
				(corp_id, record_id, phone, upload_at, status, add_at, employee_id, allot_num, remark, tags, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?)
		`, values.CorpID, recordID, item.Phone, now, status, employeeID, allotNum, item.Remark, tagsJSON, now, now)
		if err != nil {
			return 0, err
		}
		importID, _ := insert.LastInsertId()
		if employeeID > 0 {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO mc_contact_batch_add_allot (import_id, employee_id, type, operate_id, created_at)
				VALUES (?, ?, 1, ?, ?)
			`, importID, employeeID, values.OperateID, now); err != nil {
				return 0, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func (s *MySQLStore) AllotContactBatchAddImports(ctx context.Context, corpID int, importIDs []int, employeeIDs []int, operateID int) (int, error) {
	importIDs = uniquePositiveInts(importIDs)
	employeeIDs = uniquePositiveInts(employeeIDs)
	if corpID <= 0 || len(importIDs) == 0 || len(employeeIDs) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	args := []any{corpID}
	for _, id := range importIDs {
		args = append(args, id)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id
		FROM mc_contact_batch_add_import
		WHERE corp_id = ? AND id IN (`+placeholders(len(importIDs))+`) AND deleted_at IS NULL
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return 0, err
	}
	ids := make([]int, 0, len(importIDs))
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	now := time.Now()
	for index, id := range ids {
		employeeID := employeeIDs[index%len(employeeIDs)]
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_contact_batch_add_import
			SET employee_id = ?, status = 1, allot_num = allot_num + 1, updated_at = ?
			WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
		`, employeeID, now, corpID, id); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_contact_batch_add_allot (import_id, employee_id, type, operate_id, created_at)
			VALUES (?, ?, 1, ?, ?)
		`, id, employeeID, operateID, now); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(ids), nil
}

func (s *MySQLStore) DeleteContactBatchAddImports(ctx context.Context, corpID int, importIDs []int) (int, error) {
	importIDs = uniquePositiveInts(importIDs)
	if corpID <= 0 || len(importIDs) == 0 {
		return 0, nil
	}
	args := []any{time.Now(), corpID}
	for _, id := range importIDs {
		args = append(args, id)
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_contact_batch_add_import
		SET deleted_at = ?
		WHERE corp_id = ? AND id IN (`+placeholders(len(importIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

func (s *MySQLStore) DeleteContactBatchAddImportRecords(ctx context.Context, corpID int, recordIDs []int) (int, int, error) {
	recordIDs = uniquePositiveInts(recordIDs)
	if corpID <= 0 || len(recordIDs) == 0 {
		return 0, 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()
	fileURLs, err := contactBatchAddImportRecordFileURLsTx(ctx, tx, corpID, recordIDs)
	if err != nil {
		return 0, 0, err
	}
	now := time.Now()
	args := []any{now, corpID}
	for _, id := range recordIDs {
		args = append(args, id)
	}
	recordResult, err := tx.ExecContext(ctx, `
		UPDATE mc_contact_batch_add_import_record
		SET deleted_at = ?
		WHERE corp_id = ? AND id IN (`+placeholders(len(recordIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return 0, 0, err
	}
	contactArgs := []any{now, corpID}
	for _, id := range recordIDs {
		contactArgs = append(contactArgs, id)
	}
	contactResult, err := tx.ExecContext(ctx, `
		UPDATE mc_contact_batch_add_import
		SET deleted_at = ?
		WHERE corp_id = ? AND record_id IN (`+placeholders(len(recordIDs))+`) AND deleted_at IS NULL
	`, contactArgs...)
	if err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	recordAffected, err := recordResult.RowsAffected()
	if err != nil {
		return 0, 0, err
	}
	contactAffected, err := contactResult.RowsAffected()
	if err != nil {
		return 0, 0, err
	}
	reclaimPaths := contactBatchAddStoragePathsFromFileURLs(fileURLs)
	if len(reclaimPaths) > 0 {
		tenantID, err := s.tenantIDByCorpID(ctx, corpID)
		if err != nil {
			return 0, 0, err
		}
		if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
			return 0, 0, err
		}
	}
	return int(recordAffected), int(contactAffected), nil
}

func contactBatchAddImportRecordFileURLsTx(ctx context.Context, tx *sql.Tx, corpID int, recordIDs []int) ([]string, error) {
	args := []any{corpID}
	for _, id := range recordIDs {
		args = append(args, id)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT COALESCE(file_url, '')
		FROM mc_contact_batch_add_import_record
		WHERE corp_id = ? AND id IN (`+placeholders(len(recordIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func (s *MySQLStore) ContactBatchAddDashboard(ctx context.Context, corpID int, page int, perPage int) (dashboard.ContactBatchAddDashboardData, error) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 15
	}
	var summary dashboard.ContactBatchAddDashboardSummary
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN status = 0 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN status = 3 THEN 1 ELSE 0 END), 0)
		FROM mc_contact_batch_add_import
		WHERE corp_id = ? AND deleted_at IS NULL
	`, corpID).Scan(&summary.ContactNum, &summary.PendingNum, &summary.ToAddNum, &summary.PassedNum); err != nil {
		return dashboard.ContactBatchAddDashboardData{}, err
	}
	summary.Completion = contactBatchCompletion(summary.PassedNum, summary.ContactNum)
	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM (
			SELECT e.id
			FROM mc_work_employee e
			LEFT JOIN (
				SELECT employee_id, COUNT(*) AS allot_num
				FROM mc_contact_batch_add_import
				WHERE corp_id = ? AND deleted_at IS NULL
				GROUP BY employee_id
			) i ON i.employee_id = e.id
			LEFT JOIN (
				SELECT employee_id, COUNT(*) AS recycle_num
				FROM mc_contact_batch_add_allot
				WHERE type = 0
				GROUP BY employee_id
			) a ON a.employee_id = e.id
			WHERE e.corp_id = ? AND e.deleted_at IS NULL
			  AND (COALESCE(i.allot_num, 0) > 0 OR COALESCE(a.recycle_num, 0) > 0)
		) stats
	`, corpID, corpID).Scan(&total); err != nil {
		return dashboard.ContactBatchAddDashboardData{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, COALESCE(e.name, ''),
		       COALESCE(i.allot_num, 0) AS allot_num,
		       COALESCE(i.to_add_num, 0) AS to_add_num,
		       COALESCE(i.pending_num, 0) AS pending_num,
		       COALESCE(i.passed_num, 0) AS passed_num,
		       COALESCE(a.recycle_num, 0) AS recycle_num
		FROM mc_work_employee e
		LEFT JOIN (
			SELECT employee_id,
			       COUNT(*) AS allot_num,
			       SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) AS to_add_num,
			       SUM(CASE WHEN status = 2 THEN 1 ELSE 0 END) AS pending_num,
			       SUM(CASE WHEN status = 3 THEN 1 ELSE 0 END) AS passed_num
			FROM mc_contact_batch_add_import
			WHERE corp_id = ? AND deleted_at IS NULL
			GROUP BY employee_id
		) i ON i.employee_id = e.id
		LEFT JOIN (
			SELECT employee_id, COUNT(*) AS recycle_num
			FROM mc_contact_batch_add_allot
			WHERE type = 0
			GROUP BY employee_id
		) a ON a.employee_id = e.id
		WHERE e.corp_id = ? AND e.deleted_at IS NULL
		  AND (COALESCE(i.allot_num, 0) > 0 OR COALESCE(a.recycle_num, 0) > 0)
		ORDER BY e.id DESC
		LIMIT ? OFFSET ?
	`, corpID, corpID, perPage, (page-1)*perPage)
	if err != nil {
		return dashboard.ContactBatchAddDashboardData{}, err
	}
	defer rows.Close()
	items := make([]dashboard.ContactBatchAddEmployeeStat, 0)
	for rows.Next() {
		var item dashboard.ContactBatchAddEmployeeStat
		if err := rows.Scan(&item.ID, &item.Name, &item.AllotNum, &item.ToAddNum, &item.PendingNum, &item.PassedNum, &item.RecycleNum); err != nil {
			return dashboard.ContactBatchAddDashboardData{}, err
		}
		item.Completion = contactBatchCompletion(item.PassedNum, item.AllotNum)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ContactBatchAddDashboardData{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + perPage - 1) / perPage
	}
	return dashboard.ContactBatchAddDashboardData{
		Employees: dashboard.ContactBatchAddEmployeeStatPage{Items: items, Total: total, TotalPage: totalPage, PerPage: perPage, Page: page},
		Dashboard: summary,
	}, nil
}

func (s *MySQLStore) SensitiveWordPage(ctx context.Context, filter dashboard.SensitiveWordFilter) (dashboard.SensitiveWordPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	filter.PerPage = 20
	where := []string{"w.corp_id = ?", "w.deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if filter.GroupID > 0 {
		where = append(where, "w.group_id = ?")
		args = append(args, filter.GroupID)
	}
	if strings.TrimSpace(filter.KeyWords) != "" {
		where = append(where, "w.name LIKE ?")
		args = append(args, "%"+strings.TrimSpace(filter.KeyWords)+"%")
	}
	if filter.Status == 1 || filter.Status == 2 {
		where = append(where, "w.status = ?")
		args = append(args, filter.Status)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_sensitive_word w WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return dashboard.SensitiveWordPage{}, err
	}

	offset := (filter.Page - 1) * filter.PerPage
	queryArgs := []any{filter.CorpID}
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, filter.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			w.id,
			w.corp_id,
			w.group_id,
			COALESCE(g.name, ''),
			w.name,
			w.status,
			COALESCE(stats.employee_num, 0),
			COALESCE(stats.contact_num, 0),
			w.created_at,
			COALESCE(w.updated_at, w.created_at, NOW())
		FROM mc_sensitive_word w
		LEFT JOIN mc_sensitive_word_group g ON g.id = w.group_id AND g.deleted_at IS NULL
		LEFT JOIN (
			SELECT
				sensitive_word_id,
				SUM(CASE WHEN source = 2 THEN 1 ELSE 0 END) AS employee_num,
				SUM(CASE WHEN source = 1 THEN 1 ELSE 0 END) AS contact_num
			FROM mc_sensitive_words_monitor
			WHERE corp_id = ? AND deleted_at IS NULL
			GROUP BY sensitive_word_id
		) stats ON stats.sensitive_word_id = w.id
		WHERE `+whereSQL+`
		ORDER BY w.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.SensitiveWordPage{}, err
	}
	defer rows.Close()

	items := []dashboard.SensitiveWordItem{}
	for rows.Next() {
		var item dashboard.SensitiveWordItem
		var createdAt sql.NullTime
		var updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.CorpID, &item.GroupID, &item.GroupName, &item.Name, &item.Status, &item.EmployeeNum, &item.ContactNum, &createdAt, &updatedAt); err != nil {
			return dashboard.SensitiveWordPage{}, err
		}
		item.CreatedAt = formatTime(createdAt)
		item.Version = sensitiveWordVersion(sensitiveWordState{ID: item.ID, GroupID: item.GroupID, Name: item.Name, Status: item.Status, UpdatedAt: updatedAt})
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SensitiveWordPage{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	return dashboard.SensitiveWordPage{Items: items, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) CreateSensitiveWords(ctx context.Context, corpID int, groupID int, names []string) error {
	if len(names) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var groupCount int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_sensitive_word_group WHERE id = ? AND corp_id = ? AND deleted_at IS NULL", groupID, corpID).Scan(&groupCount); err != nil {
		return err
	}
	if groupCount == 0 {
		return fmt.Errorf("sensitive word group not found")
	}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_sensitive_word (corp_id, group_id, name, status, created_at, updated_at)
			VALUES (?, ?, ?, 1, NOW(), NOW())
		`, corpID, groupID, name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *MySQLStore) UpdateSensitiveWordStatus(ctx context.Context, corpID int, wordID int, status int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_sensitive_word
		SET status = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, status, wordID, corpID)
	return rowsAffectedBool(result, err)
}

func (s *MySQLStore) MoveSensitiveWord(ctx context.Context, corpID int, wordID int, groupID int) (bool, error) {
	var groupCount int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_sensitive_word_group WHERE id = ? AND corp_id = ? AND deleted_at IS NULL", groupID, corpID).Scan(&groupCount); err != nil {
		return false, err
	}
	if groupCount == 0 {
		return false, fmt.Errorf("sensitive word group not found")
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_sensitive_word
		SET group_id = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, groupID, wordID, corpID)
	return rowsAffectedBool(result, err)
}

func (s *MySQLStore) DeleteSensitiveWord(ctx context.Context, corpID int, wordID int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_sensitive_word
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, wordID, corpID)
	return rowsAffectedBool(result, err)
}

func (s *MySQLStore) SensitiveWordGroups(ctx context.Context, corpID int) ([]dashboard.SensitiveWordGroup, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.name, COALESCE(g.updated_at, g.created_at, NOW()),
		       (SELECT COUNT(*) FROM mc_sensitive_word w WHERE w.group_id = g.id AND w.corp_id = g.corp_id AND w.deleted_at IS NULL),
		       (SELECT COUNT(*) FROM mc_sensitive_word w WHERE w.group_id = g.id AND w.corp_id = g.corp_id AND w.status = 1 AND w.deleted_at IS NULL)
		FROM mc_sensitive_word_group g
		WHERE g.corp_id = ? AND g.deleted_at IS NULL
		ORDER BY g.id ASC
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := []dashboard.SensitiveWordGroup{}
	for rows.Next() {
		var group dashboard.SensitiveWordGroup
		var updatedAt time.Time
		if err := rows.Scan(&group.ID, &group.Name, &updatedAt, &group.WordCount, &group.EnabledCount); err != nil {
			return nil, err
		}
		group.Version = sensitiveWordGroupVersion(sensitiveWordState{ID: group.ID, Name: group.Name, UpdatedAt: updatedAt})
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (s *MySQLStore) CreateSensitiveWordGroups(ctx context.Context, corpID int, names []string) error {
	if len(names) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_sensitive_word_group (corp_id, name, created_at, updated_at)
			VALUES (?, ?, NOW(), NOW())
		`, corpID, name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *MySQLStore) UpdateSensitiveWordGroup(ctx context.Context, corpID int, groupID int, name string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_sensitive_word_group
		SET name = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, strings.TrimSpace(name), groupID, corpID)
	return rowsAffectedBool(result, err)
}

func (s *MySQLStore) SensitiveWordsMonitorPage(ctx context.Context, filter dashboard.SensitiveWordsMonitorFilter) (dashboard.SensitiveWordsMonitorPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	filter.PerPage = 20
	where := []string{"m.corp_id = ?", "m.deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if len(filter.EmployeeIDs) > 0 {
		placeholders := make([]string, 0, len(filter.EmployeeIDs))
		for _, id := range filter.EmployeeIDs {
			if id <= 0 {
				continue
			}
			placeholders = append(placeholders, "?")
			args = append(args, id)
		}
		if len(placeholders) > 0 {
			where = append(where, "m.trigger_user_id IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	if filter.WorkRoomID > 0 {
		where = append(where, "m.work_room_id = ?")
		args = append(args, filter.WorkRoomID)
	}
	if filter.IntelligentGroupID > 0 {
		where = append(where, "w.group_id = ?")
		args = append(args, filter.IntelligentGroupID)
	}
	if filter.SensitiveWordID > 0 {
		where = append(where, "m.sensitive_word_id = ?")
		args = append(args, filter.SensitiveWordID)
	}
	if filter.Source > 0 {
		where = append(where, "m.source = ?")
		args = append(args, filter.Source)
	}
	if strings.TrimSpace(filter.Scenario) != "" {
		where = append(where, "m.trigger_scenario = ?")
		args = append(args, strings.TrimSpace(filter.Scenario))
	}
	if strings.TrimSpace(filter.TriggerStart) != "" {
		where = append(where, "m.send_time >= ?")
		args = append(args, strings.TrimSpace(filter.TriggerStart))
	}
	if strings.TrimSpace(filter.TriggerEnd) != "" {
		where = append(where, "m.send_time <= ?")
		args = append(args, strings.TrimSpace(filter.TriggerEnd))
	}
	whereSQL := strings.Join(where, " AND ")
	fromSQL := `
		FROM mc_sensitive_words_monitor m
		LEFT JOIN mc_sensitive_word w ON w.id = m.sensitive_word_id
	`

	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) "+fromSQL+" WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return dashboard.SensitiveWordsMonitorPage{}, err
	}

	offset := (filter.Page - 1) * filter.PerPage
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			m.id,
			m.sensitive_word_id,
			COALESCE(NULLIF(m.sensitive_word_name, ''), w.name, ''),
			m.source,
			m.trigger_name,
			m.trigger_scenario,
			m.send_time,
			m.work_room_id,
			CAST(m.content AS CHAR)
		`+fromSQL+`
		WHERE `+whereSQL+`
		ORDER BY m.send_time DESC, m.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.SensitiveWordsMonitorPage{}, err
	}
	defer rows.Close()

	items := []dashboard.SensitiveWordsMonitorItem{}
	for rows.Next() {
		var item dashboard.SensitiveWordsMonitorItem
		var sendTime sql.NullTime
		var content string
		if err := rows.Scan(&item.ID, &item.SensitiveWordID, &item.SensitiveWordName, &item.Source, &item.TriggerName, &item.TriggerScenario, &sendTime, &item.WorkRoomID, &content); err != nil {
			return dashboard.SensitiveWordsMonitorPage{}, err
		}
		item.TriggerTime = formatTime(sendTime)
		item.ContentPreview = sensitiveWordContentPreview(content)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SensitiveWordsMonitorPage{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	return dashboard.SensitiveWordsMonitorPage{Items: items, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func sensitiveWordContentPreview(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var payload any
	if json.Unmarshal([]byte(raw), &payload) == nil {
		if object, ok := payload.(map[string]any); ok {
			for _, key := range []string{"content", "text", "title", "description", "desc", "name"} {
				if value, ok := object[key].(string); ok && strings.TrimSpace(value) != "" {
					raw = value
					break
				}
			}
		}
	}
	runes := []rune(raw)
	if len(runes) > 120 {
		return string(runes[:120]) + "…"
	}
	return raw
}

func (s *MySQLStore) SensitiveWordsMonitorMessages(ctx context.Context, filter dashboard.SensitiveWordsMonitorMessageFilter) ([]dashboard.SensitiveWordsMonitorMessage, bool, error) {
	where := []string{"id = ?", "corp_id = ?", "deleted_at IS NULL"}
	args := []any{filter.MonitorID, filter.CorpID}
	if filter.RestrictEmployeeIDs {
		placeholders := make([]string, 0, len(filter.AllowedEmployeeIDs))
		seen := make(map[int]struct{}, len(filter.AllowedEmployeeIDs))
		for _, employeeID := range filter.AllowedEmployeeIDs {
			if employeeID <= 0 {
				continue
			}
			if _, exists := seen[employeeID]; exists {
				continue
			}
			seen[employeeID] = struct{}{}
			placeholders = append(placeholders, "?")
			args = append(args, employeeID)
		}
		if len(placeholders) == 0 {
			return nil, false, nil
		}
		where = append(where, "trigger_user_id IN ("+strings.Join(placeholders, ",")+")")
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT sender, msg_type, send_time, content, conversation_json
		FROM mc_sensitive_words_monitor
		WHERE `+strings.Join(where, " AND ")+`
		LIMIT 1
	`, args...)
	var sender string
	var msgType int
	var sendTime sql.NullTime
	var content sql.NullString
	var conversation sql.NullString
	err := row.Scan(&sender, &msgType, &sendTime, &content, &conversation)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(conversation.String) != "" {
		var messages []dashboard.SensitiveWordsMonitorMessage
		if err := json.Unmarshal([]byte(conversation.String), &messages); err == nil && len(messages) > 0 {
			for index := range messages {
				if messages[index].SendTime == "" {
					messages[index].SendTime = formatTime(sendTime)
				}
				if messages[index].Sender == "" {
					messages[index].Sender = sender
				}
				if messages[index].MsgType == 0 {
					messages[index].MsgType = msgType
				}
			}
			return messages, true, nil
		}
	}
	message := dashboard.SensitiveWordsMonitorMessage{
		Sender:     sender,
		MsgType:    msgType,
		SendTime:   formatTime(sendTime),
		IsTrigger:  1,
		MsgContent: sensitiveMonitorContent(content.String),
	}
	return []dashboard.SensitiveWordsMonitorMessage{message}, true, nil
}

func sensitiveMonitorContent(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{"content": ""}
	}
	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err == nil {
		return payload
	}
	return map[string]any{"content": raw}
}

func rowsAffectedBool(result sql.Result, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func contactBatchAddWhere(filter dashboard.ContactBatchAddFilter) ([]string, []any) {
	where := []string{"i.corp_id = ?", "i.deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if filter.HasStatus {
		where = append(where, "i.status = ?")
		args = append(args, filter.Status)
	}
	if filter.RecordID > 0 {
		where = append(where, "i.record_id = ?")
		args = append(args, filter.RecordID)
	}
	if filter.SearchKey != "" {
		like := "%" + filter.SearchKey + "%"
		where = append(where, "(i.phone LIKE ? OR i.remark LIKE ? OR e.name LIKE ?)")
		args = append(args, like, like, like)
	}
	return where, args
}

type contactBatchAddScanner interface {
	Scan(dest ...any) error
}

func scanContactBatchAddItem(scanner contactBatchAddScanner) (dashboard.ContactBatchAddItem, error) {
	var item dashboard.ContactBatchAddItem
	var phone, remark sql.NullString
	var uploadAt, addAt sql.NullTime
	var rawTags []byte
	if err := scanner.Scan(&item.ID, &item.RecordID, &phone, &uploadAt, &item.Status, &addAt, &item.EmployeeID, &item.AllotNum, &remark, &rawTags); err != nil {
		return dashboard.ContactBatchAddItem{}, err
	}
	item.Phone = nullString(phone)
	item.UploadAt = formatTime(uploadAt)
	item.AddAt = formatTime(addAt)
	item.Remark = nullString(remark)
	item.TagIDs = contactBatchAddIntSliceFromJSON(rawTags)
	return item, nil
}

func contactBatchAddIntSliceFromJSON(raw []byte) []int {
	if len(raw) == 0 {
		return []int{}
	}
	var ints []int
	if err := json.Unmarshal(raw, &ints); err == nil {
		return uniquePositiveInts(ints)
	}
	var values []map[string]any
	if err := json.Unmarshal(raw, &values); err == nil {
		result := make([]int, 0, len(values))
		for _, value := range values {
			if id := intFromAny(value["id"]); id > 0 {
				result = append(result, id)
				continue
			}
			if id := intFromAny(value["employeeId"]); id > 0 {
				result = append(result, id)
				continue
			}
			if id := intFromAny(value["tagId"]); id > 0 {
				result = append(result, id)
			}
		}
		return uniquePositiveInts(result)
	}
	return intSliceFromJSON(raw)
}

func contactBatchCompletion(done int, total int) float64 {
	if total <= 0 {
		return 0
	}
	value := float64(done) * 100 / float64(total)
	rounded, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", value), 64)
	return rounded
}

func (s *MySQLStore) WorkAgentCredentialByID(ctx context.Context, agentID int) (dashboard.SidebarAgentCredential, bool, error) {
	agent, found, err := s.loadAgentCredentialByID(ctx, s.db, agentID, false)
	if err != nil || !found {
		return dashboard.SidebarAgentCredential{}, found, err
	}
	var closeState int
	if err := s.db.QueryRowContext(ctx, `SELECT close FROM mc_work_agent WHERE id = ? AND deleted_at IS NULL`, agentID).Scan(&closeState); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dashboard.SidebarAgentCredential{}, false, nil
		}
		return dashboard.SidebarAgentCredential{}, false, err
	}
	if closeState != 0 {
		return dashboard.SidebarAgentCredential{}, false, nil
	}
	corp, found, err := s.loadCorpCredentialByID(ctx, s.db, agent.CorpID, false)
	if err != nil || !found {
		return dashboard.SidebarAgentCredential{}, found, err
	}
	corpSecret, err := s.decodeCorpCredential(corp)
	if err != nil {
		return dashboard.SidebarAgentCredential{}, false, err
	}
	agentSecret, err := s.decodeAgentCredential(agent)
	if err != nil {
		return dashboard.SidebarAgentCredential{}, false, err
	}
	return dashboard.SidebarAgentCredential{
		ID: agent.ID, CorpID: agent.CorpID, WXCorpID: corp.WXCorpID,
		EmployeeSecret: corpSecret.EmployeeSecret, ContactSecret: corpSecret.ContactSecret,
		WXAgentID: agent.WXAgentID, WXSecret: agentSecret.WXSecret,
	}, true, nil
}

func (s *MySQLStore) SidebarAgentCorpCredentialByID(ctx context.Context, corpID int) (dashboard.SidebarAgentCredential, bool, error) {
	corp, found, err := s.loadCorpCredentialByID(ctx, s.db, corpID, false)
	if err != nil || !found {
		return dashboard.SidebarAgentCredential{}, found, err
	}
	credential, err := s.decodeCorpCredential(corp)
	if err != nil {
		return dashboard.SidebarAgentCredential{}, false, err
	}
	return dashboard.SidebarAgentCredential{
		CorpID: corp.ID, WXCorpID: corp.WXCorpID,
		EmployeeSecret: credential.EmployeeSecret, ContactSecret: credential.ContactSecret,
	}, true, nil
}

func (s *MySQLStore) SidebarEmployeeLoginByWXUserID(ctx context.Context, wxUserID string, corpID int) (dashboard.SidebarAgentEmployeeLogin, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT e.id, e.corp_id, e.log_user_id, COALESCE(u.status, 0)
		FROM mc_work_employee e
		LEFT JOIN mc_user u ON u.id = e.log_user_id AND u.deleted_at IS NULL
		WHERE e.wx_user_id = ? AND e.corp_id = ? AND e.deleted_at IS NULL
		LIMIT 1
	`, wxUserID, corpID)
	var login dashboard.SidebarAgentEmployeeLogin
	err := row.Scan(&login.EmployeeID, &login.CorpID, &login.UserID, &login.UserStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SidebarAgentEmployeeLogin{}, false, nil
	}
	if err != nil {
		return dashboard.SidebarAgentEmployeeLogin{}, false, err
	}
	return login, true, nil
}

func (s *MySQLStore) EmployeeByID(ctx context.Context, employeeID int) (dashboard.Employee, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, mobile, position, gender, email, avatar, thumb_avatar, telephone, alias, status, qr_code, external_position, address
		FROM mc_work_employee
		WHERE id = ? AND deleted_at IS NULL
	`, employeeID)

	var employee dashboard.Employee
	err := row.Scan(
		&employee.ID, &employee.Name, &employee.Mobile, &employee.Position,
		&employee.Gender, &employee.Email, &employee.Avatar, &employee.ThumbAvatar,
		&employee.Telephone, &employee.Alias, &employee.Status, &employee.QRCode,
		&employee.ExternalPosition, &employee.Address,
	)
	if err == sql.ErrNoRows {
		return dashboard.Employee{}, false, nil
	}
	if err != nil {
		return dashboard.Employee{}, false, err
	}
	return employee, true, nil
}

func (s *MySQLStore) CorpByID(ctx context.Context, corpID int) (dashboard.Corp, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name
		FROM mc_corp
		WHERE id = ? AND deleted_at IS NULL
	`, corpID)

	var corp dashboard.Corp
	err := row.Scan(&corp.ID, &corp.Name)
	if err == sql.ErrNoRows {
		return dashboard.Corp{}, false, nil
	}
	if err != nil {
		return dashboard.Corp{}, false, err
	}
	return corp, true, nil
}

func (s *MySQLStore) CorpDetailByID(ctx context.Context, corpID int) (dashboard.CorpDetail, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, wx_corpid, social_code, event_callback, tenant_id, created_at, updated_at
		FROM mc_corp
		WHERE id = ? AND deleted_at IS NULL
	`, corpID)

	var corp dashboard.CorpDetail
	var socialCode, eventCallback sql.NullString
	var createdAt, updatedAt sql.NullTime
	err := row.Scan(
		&corp.ID,
		&corp.Name,
		&corp.WxCorpID,
		&socialCode,
		&eventCallback,
		&corp.TenantID,
		&createdAt,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return dashboard.CorpDetail{}, false, nil
	}
	if err != nil {
		return dashboard.CorpDetail{}, false, err
	}
	stored, found, err := s.loadCorpCredentialByID(ctx, s.db, corp.ID, false)
	if err != nil || !found {
		return dashboard.CorpDetail{}, found, err
	}
	credential, err := s.decodeCorpCredential(stored)
	if err != nil {
		return dashboard.CorpDetail{}, false, err
	}
	corp.SocialCode = nullString(socialCode)
	corp.EmployeeSecret = credential.EmployeeSecret
	corp.EventCallback = nullString(eventCallback)
	corp.ContactSecret = credential.ContactSecret
	corp.Token = credential.CallbackToken
	corp.EncodingAESKey = credential.EncodingAESKey
	corp.CreatedAt = formatTime(createdAt)
	corp.UpdatedAt = formatTime(updatedAt)
	return corp, true, nil
}

func (s *MySQLStore) WeWorkCallbackCorpByID(ctx context.Context, corpID int) (dashboard.WeWorkCallbackCorp, bool, error) {
	item, found, err := s.loadAuthoritativeWeWorkCallbackCorpByID(ctx, corpID)
	if err != nil || !found {
		return dashboard.WeWorkCallbackCorp{}, found, err
	}
	credential, err := s.decodeCorpCredential(item)
	if err != nil {
		return dashboard.WeWorkCallbackCorp{}, false, err
	}
	return dashboard.WeWorkCallbackCorp{TenantID: item.TenantID, ID: item.ID, WxCorpID: item.WXCorpID, Token: credential.CallbackToken, EncodingAESKey: credential.EncodingAESKey}, true, nil
}

func (s *MySQLStore) WeWorkCallbackCorpByWXID(ctx context.Context, wxCorpID string) (dashboard.WeWorkCallbackCorp, bool, error) {
	item, found, err := s.loadAuthoritativeWeWorkCallbackCorpByWXID(ctx, wxCorpID)
	if err != nil || !found {
		return dashboard.WeWorkCallbackCorp{}, found, err
	}
	credential, err := s.decodeCorpCredential(item)
	if err != nil {
		return dashboard.WeWorkCallbackCorp{}, false, err
	}
	return dashboard.WeWorkCallbackCorp{TenantID: item.TenantID, ID: item.ID, WxCorpID: item.WXCorpID, Token: credential.CallbackToken, EncodingAESKey: credential.EncodingAESKey}, true, nil
}

func (s *MySQLStore) CorpList(ctx context.Context, filter dashboard.CorpListFilter) (dashboard.CorpListPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10
	}
	if !filter.SuperAdmin && len(filter.CorpIDs) == 0 {
		return dashboard.CorpListPage{Items: []dashboard.CorpDetail{}}, nil
	}

	where, args := corpListWhere(filter)
	countQuery := `SELECT COUNT(*) FROM mc_corp ` + where
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return dashboard.CorpListPage{}, err
	}
	if total == 0 {
		return dashboard.CorpListPage{Items: []dashboard.CorpDetail{}}, nil
	}

	totalPage := (total + filter.PerPage - 1) / filter.PerPage
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, wx_corpid, created_at
		FROM mc_corp
		`+where+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.CorpListPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.CorpDetail, 0)
	for rows.Next() {
		var corp dashboard.CorpDetail
		var createdAt sql.NullTime
		if err := rows.Scan(&corp.ID, &corp.Name, &corp.WxCorpID, &createdAt); err != nil {
			return dashboard.CorpListPage{}, err
		}
		corp.CreatedAt = formatTime(createdAt)
		items = append(items, corp)
	}
	if err := rows.Err(); err != nil {
		return dashboard.CorpListPage{}, err
	}

	return dashboard.CorpListPage{
		Items:     items,
		Total:     total,
		TotalPage: totalPage,
	}, nil
}

func (s *MySQLStore) CountCorps(ctx context.Context) (int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_corp
		WHERE deleted_at IS NULL
	`).Scan(&total)
	return total, err
}

func (s *MySQLStore) ActiveCorpIDs(ctx context.Context) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM mc_corp
		WHERE deleted_at IS NULL
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	corpIDs := make([]int, 0)
	for rows.Next() {
		var corpID int
		if err := rows.Scan(&corpID); err != nil {
			return nil, err
		}
		corpIDs = append(corpIDs, corpID)
	}
	return corpIDs, rows.Err()
}

func (s *MySQLStore) CorpDataSummary(ctx context.Context, scope dashboard.CorpDataScope, now time.Time) (dashboard.CorpDataSummary, error) {
	if scope.TenantID <= 0 || scope.CorpID <= 0 {
		return dashboard.CorpDataSummary{}, fmt.Errorf("tenant id and corp id must be positive")
	}

	var summary dashboard.CorpDataSummary
	var latestUpdate time.Time
	for _, spec := range corpDataSummaryQuerySpecs(scope, now) {
		var row corpDataSummaryRow
		if err := s.corpDataExecutor.QueryRowContext(ctx, spec.query, spec.args...).Scan(
			&row.WeChatContactNum, &row.WeChatRoomNum, &row.RoomMemberNum, &row.CorpMemberNum,
			&row.AddContactNum, &row.LastAddContactNum, &row.AddIntoRoomNum, &row.LastAddIntoRoomNum,
			&row.LossContactNum, &row.LastLossContactNum, &row.QuitRoomNum, &row.LastQuitRoomNum,
			&row.AddFriendsNum, &row.LastAddFriendsNum, &row.MonthAddRoomNum, &row.LastMonthAddRoomNum,
			&row.MonthAddRoomMemberNum, &row.LastMonthAddRoomMemberNum,
			&row.MonthLossContactNum, &row.LastMonthLossContactNum, &row.UpdateTime,
		); err != nil {
			return dashboard.CorpDataSummary{}, err
		}
		mergeCorpDataSummary(&summary, row)
		if row.UpdateTime.Valid && row.UpdateTime.Time.After(latestUpdate) {
			latestUpdate = row.UpdateTime.Time
		}
	}
	if !latestUpdate.IsZero() {
		summary.UpdateTime = latestUpdate.In(now.Location()).Format("2006-01-02 15:04:05")
	}
	return summary, nil
}

type corpDataSummaryQuerySpec struct {
	domain string
	query  string
	args   []any
}

const corpDataTimezoneOffset = "+08:00"

type corpDataSummaryRow struct {
	WeChatContactNum          int
	WeChatRoomNum             int
	RoomMemberNum             int
	CorpMemberNum             int
	AddContactNum             int
	LastAddContactNum         int
	AddIntoRoomNum            int
	LastAddIntoRoomNum        int
	LossContactNum            int
	LastLossContactNum        int
	QuitRoomNum               int
	LastQuitRoomNum           int
	AddFriendsNum             int
	LastAddFriendsNum         int
	MonthAddRoomNum           int
	LastMonthAddRoomNum       int
	MonthAddRoomMemberNum     int
	LastMonthAddRoomMemberNum int
	MonthLossContactNum       int
	LastMonthLossContactNum   int
	UpdateTime                sql.NullTime
}

func corpDataSummaryQuerySpecs(scope dashboard.CorpDataScope, now time.Time) []corpDataSummaryQuerySpec {
	dayBegin := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	monthBegin := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	bounds := []any{
		dayBegin.AddDate(0, 0, -1).Unix(), dayBegin.Unix(), dayBegin.AddDate(0, 0, 1).Unix(),
		monthBegin.AddDate(0, -1, 0).Unix(), monthBegin.Unix(), monthBegin.AddDate(0, 1, 0).Unix(),
	}
	return []corpDataSummaryQuerySpec{
		corpDataSummaryContactsQuery(scope, bounds),
		corpDataSummaryRoomsQuery(scope, bounds),
		corpDataSummaryRoomMembersQuery(scope, bounds),
		corpDataSummaryEmployeesQuery(scope, bounds),
	}
}

func corpDataSummaryBoundsSQL() string {
	return `FROM (
		SELECT FROM_UNIXTIME(?) AS yesterday_start,
			FROM_UNIXTIME(?) AS today_start,
			FROM_UNIXTIME(?) AS tomorrow_start,
			FROM_UNIXTIME(?) AS last_month_start,
			FROM_UNIXTIME(?) AS month_start,
			FROM_UNIXTIME(?) AS next_month_start
	) AS bounds`
}

func corpDataSummarySelect(expressions []string, updateExpression string) string {
	return "SELECT " + strings.Join(expressions, ",\n") + ",\nMAX(CONVERT_TZ(" + updateExpression + ", @@session.time_zone, '" + corpDataTimezoneOffset + "')) AS update_time\n"
}

func corpDataZeroSummaryExpressions() []string {
	expressions := make([]string, 20)
	for index := range expressions {
		expressions[index] = "0"
	}
	return expressions
}

func corpDataConditionalCount(condition string) string {
	return "COUNT(CASE WHEN " + condition + " THEN 1 END)"
}

func corpDataSummaryContactsQuery(scope dashboard.CorpDataScope, bounds []any) corpDataSummaryQuerySpec {
	expressions := corpDataZeroSummaryExpressions()
	expressions[0] = corpDataConditionalCount("contact_employee.status = 1 AND contact_employee.deleted_at IS NULL")
	expressions[4] = corpDataConditionalCount("contact_employee.deleted_at IS NULL AND contact_employee.create_time >= bounds.today_start AND contact_employee.create_time < bounds.tomorrow_start")
	expressions[5] = corpDataConditionalCount("contact_employee.deleted_at IS NULL AND contact_employee.create_time >= bounds.yesterday_start AND contact_employee.create_time < bounds.today_start")
	expressions[8] = corpDataConditionalCount("contact_employee.status IN (2, 3) AND contact_employee.deleted_at >= bounds.today_start AND contact_employee.deleted_at < bounds.tomorrow_start")
	expressions[9] = corpDataConditionalCount("contact_employee.status IN (2, 3) AND contact_employee.deleted_at >= bounds.yesterday_start AND contact_employee.deleted_at < bounds.today_start")
	expressions[12] = corpDataConditionalCount("contact_employee.deleted_at IS NULL AND contact_employee.create_time >= bounds.month_start AND contact_employee.create_time < bounds.next_month_start")
	expressions[13] = corpDataConditionalCount("contact_employee.deleted_at IS NULL AND contact_employee.create_time >= bounds.last_month_start AND contact_employee.create_time < bounds.month_start")
	expressions[18] = corpDataConditionalCount("contact_employee.status IN (2, 3) AND contact_employee.deleted_at >= bounds.month_start AND contact_employee.deleted_at < bounds.next_month_start")
	expressions[19] = corpDataConditionalCount("contact_employee.status IN (2, 3) AND contact_employee.deleted_at >= bounds.last_month_start AND contact_employee.deleted_at < bounds.month_start")
	filter, filterArgs := corpDataEmployeeScopeSQL(scope, "contact_employee.employee_id")
	query := corpDataSummarySelect(expressions, "GREATEST(contact_employee.create_time, COALESCE(contact_employee.updated_at, contact_employee.create_time), COALESCE(contact_employee.deleted_at, contact_employee.create_time))") + corpDataSummaryBoundsSQL() + `
	CROSS JOIN mc_work_contact_employee AS contact_employee
	INNER JOIN mc_corp AS scoped_corp
		ON scoped_corp.id = contact_employee.corp_id
		AND scoped_corp.tenant_id = ?
		AND scoped_corp.deleted_at IS NULL
	WHERE contact_employee.corp_id = ?` + filter
	args := append(append(append([]any{}, bounds...), scope.TenantID, scope.CorpID), filterArgs...)
	return corpDataSummaryQuerySpec{domain: "contacts", query: query, args: args}
}

func corpDataSummaryRoomsQuery(scope dashboard.CorpDataScope, bounds []any) corpDataSummaryQuerySpec {
	expressions := corpDataZeroSummaryExpressions()
	expressions[1] = "COUNT(*)"
	expressions[14] = corpDataConditionalCount("room.created_at >= bounds.month_start AND room.created_at < bounds.next_month_start")
	expressions[15] = corpDataConditionalCount("room.created_at >= bounds.last_month_start AND room.created_at < bounds.month_start")
	filter, filterArgs := corpDataEmployeeScopeSQL(scope, "room.owner_id")
	query := corpDataSummarySelect(expressions, "COALESCE(room.updated_at, room.created_at)") + corpDataSummaryBoundsSQL() + `
	CROSS JOIN mc_work_room AS room
	INNER JOIN mc_corp AS scoped_corp
		ON scoped_corp.id = room.corp_id
		AND scoped_corp.tenant_id = ?
		AND scoped_corp.deleted_at IS NULL
	WHERE room.corp_id = ? AND room.deleted_at IS NULL` + filter
	args := append(append(append([]any{}, bounds...), scope.TenantID, scope.CorpID), filterArgs...)
	return corpDataSummaryQuerySpec{domain: "rooms", query: query, args: args}
}

func corpDataSummaryRoomMembersQuery(scope dashboard.CorpDataScope, bounds []any) corpDataSummaryQuerySpec {
	expressions := corpDataZeroSummaryExpressions()
	expressions[2] = corpDataConditionalCount("contact_room.status = 1")
	expressions[6] = corpDataConditionalCount("contact_room.status = 1 AND contact_room.join_time >= bounds.today_start AND contact_room.join_time < bounds.tomorrow_start")
	expressions[7] = corpDataConditionalCount("contact_room.status = 1 AND contact_room.join_time >= bounds.yesterday_start AND contact_room.join_time < bounds.today_start")
	expressions[10] = corpDataConditionalCount("contact_room.status = 2 AND contact_room.out_time != '' AND contact_room.updated_at >= bounds.today_start AND contact_room.updated_at < bounds.tomorrow_start")
	expressions[11] = corpDataConditionalCount("contact_room.status = 2 AND contact_room.out_time != '' AND contact_room.updated_at >= bounds.yesterday_start AND contact_room.updated_at < bounds.today_start")
	expressions[16] = corpDataConditionalCount("contact_room.status = 1 AND contact_room.join_time >= bounds.month_start AND contact_room.join_time < bounds.next_month_start")
	expressions[17] = corpDataConditionalCount("contact_room.status = 1 AND contact_room.join_time >= bounds.last_month_start AND contact_room.join_time < bounds.month_start")
	filter, filterArgs := corpDataEmployeeScopeSQL(scope, "room.owner_id")
	query := corpDataSummarySelect(expressions, "GREATEST(contact_room.join_time, COALESCE(contact_room.updated_at, contact_room.join_time))") + corpDataSummaryBoundsSQL() + `
	CROSS JOIN mc_work_contact_room AS contact_room
	INNER JOIN mc_work_room AS room
		ON room.id = contact_room.room_id AND room.deleted_at IS NULL
	INNER JOIN mc_corp AS scoped_corp
		ON scoped_corp.id = room.corp_id
		AND scoped_corp.tenant_id = ?
		AND scoped_corp.deleted_at IS NULL
	WHERE room.corp_id = ? AND contact_room.deleted_at IS NULL` + filter
	args := append(append(append([]any{}, bounds...), scope.TenantID, scope.CorpID), filterArgs...)
	return corpDataSummaryQuerySpec{domain: "room_members", query: query, args: args}
}

func corpDataSummaryEmployeesQuery(scope dashboard.CorpDataScope, bounds []any) corpDataSummaryQuerySpec {
	expressions := corpDataZeroSummaryExpressions()
	expressions[3] = "COUNT(*)"
	filter, filterArgs := corpDataEmployeeScopeSQL(scope, "employee.id")
	query := corpDataSummarySelect(expressions, "COALESCE(employee.updated_at, employee.created_at)") + corpDataSummaryBoundsSQL() + `
	CROSS JOIN mc_work_employee AS employee
	INNER JOIN mc_corp AS scoped_corp
		ON scoped_corp.id = employee.corp_id
		AND scoped_corp.tenant_id = ?
		AND scoped_corp.deleted_at IS NULL
	WHERE employee.corp_id = ? AND employee.status = 1 AND employee.deleted_at IS NULL` + filter
	args := append(append(append([]any{}, bounds...), scope.TenantID, scope.CorpID), filterArgs...)
	return corpDataSummaryQuerySpec{domain: "employees", query: query, args: args}
}

func mergeCorpDataSummary(summary *dashboard.CorpDataSummary, row corpDataSummaryRow) {
	summary.WeChatContactNum += row.WeChatContactNum
	summary.WeChatRoomNum += row.WeChatRoomNum
	summary.RoomMemberNum += row.RoomMemberNum
	summary.CorpMemberNum += row.CorpMemberNum
	summary.AddContactNum += row.AddContactNum
	summary.LastAddContactNum += row.LastAddContactNum
	summary.AddIntoRoomNum += row.AddIntoRoomNum
	summary.LastAddIntoRoomNum += row.LastAddIntoRoomNum
	summary.LossContactNum += row.LossContactNum
	summary.LastLossContactNum += row.LastLossContactNum
	summary.QuitRoomNum += row.QuitRoomNum
	summary.LastQuitRoomNum += row.LastQuitRoomNum
	summary.AddFriendsNum += row.AddFriendsNum
	summary.LastAddFriendsNum += row.LastAddFriendsNum
	summary.MonthAddRoomNum += row.MonthAddRoomNum
	summary.LastMonthAddRoomNum += row.LastMonthAddRoomNum
	summary.MonthAddRoomMemberNum += row.MonthAddRoomMemberNum
	summary.LastMonthAddRoomMemberNum += row.LastMonthAddRoomMemberNum
	summary.MonthLossContactNum += row.MonthLossContactNum
	summary.LastMonthLossContactNum += row.LastMonthLossContactNum
}

func (s *MySQLStore) CorpDataLineChat(ctx context.Context, scope dashboard.CorpDataScope, from time.Time, to time.Time) ([]dashboard.CorpDataPoint, error) {
	if scope.TenantID <= 0 || scope.CorpID <= 0 {
		return nil, fmt.Errorf("tenant id and corp id must be positive")
	}
	query, args := corpDataTrendQuery(scope, from, to.AddDate(0, 0, 1))
	rows, err := s.corpDataExecutor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]dashboard.CorpDataPoint, 0)
	for rows.Next() {
		var point dashboard.CorpDataPoint
		var date string
		if err := rows.Scan(&point.ID, &point.AddContactNum, &point.AddIntoRoomNum, &point.LossContactNum, &point.QuitRoomNum, &date); err != nil {
			return nil, err
		}
		point.Date = date
		points = append(points, point)
	}
	return points, rows.Err()
}

type corpDataMetric int

const (
	corpDataMetricContacts corpDataMetric = iota
	corpDataMetricRooms
	corpDataMetricRoomMembers
	corpDataMetricEmployees
	corpDataMetricAddContacts
	corpDataMetricAddRooms
	corpDataMetricAddIntoRooms
	corpDataMetricLossContacts
	corpDataMetricQuitRooms
)

type corpDataMetricSource struct {
	fromWhere          string
	args               []any
	idExpression       string
	timeExpression     string
	employeeExpression string
}

func corpDataMetricSourceFor(metric corpDataMetric, scope dashboard.CorpDataScope) corpDataMetricSource {
	var source corpDataMetricSource
	switch metric {
	case corpDataMetricContacts, corpDataMetricAddContacts:
		source = corpDataMetricSource{
			fromWhere: `
				FROM mc_work_contact_employee AS contact_employee
				INNER JOIN mc_corp AS scoped_corp
					ON scoped_corp.id = contact_employee.corp_id
					AND scoped_corp.tenant_id = ?
					AND scoped_corp.deleted_at IS NULL
				WHERE contact_employee.corp_id = ?`,
			args: []any{scope.TenantID, scope.CorpID}, idExpression: "contact_employee.id",
			employeeExpression: "contact_employee.employee_id", timeExpression: "contact_employee.create_time",
		}
		if metric == corpDataMetricContacts {
			source.fromWhere += " AND contact_employee.status = 1 AND contact_employee.deleted_at IS NULL"
		} else {
			source.fromWhere += " AND contact_employee.deleted_at IS NULL"
		}
	case corpDataMetricLossContacts:
		source = corpDataMetricSource{
			fromWhere: `
				FROM mc_work_contact_employee AS contact_employee
				INNER JOIN mc_corp AS scoped_corp
					ON scoped_corp.id = contact_employee.corp_id
					AND scoped_corp.tenant_id = ?
					AND scoped_corp.deleted_at IS NULL
				WHERE contact_employee.corp_id = ? AND contact_employee.status IN (2, 3)`,
			args: []any{scope.TenantID, scope.CorpID}, idExpression: "contact_employee.id",
			employeeExpression: "contact_employee.employee_id", timeExpression: "contact_employee.deleted_at",
		}
	case corpDataMetricRooms, corpDataMetricAddRooms:
		source = corpDataMetricSource{
			fromWhere: `
				FROM mc_work_room AS room
				INNER JOIN mc_corp AS scoped_corp
					ON scoped_corp.id = room.corp_id
					AND scoped_corp.tenant_id = ?
					AND scoped_corp.deleted_at IS NULL
				WHERE room.corp_id = ? AND room.deleted_at IS NULL`,
			args: []any{scope.TenantID, scope.CorpID}, idExpression: "room.id",
			employeeExpression: "room.owner_id", timeExpression: "room.created_at",
		}
	case corpDataMetricRoomMembers, corpDataMetricAddIntoRooms, corpDataMetricQuitRooms:
		source = corpDataMetricSource{
			fromWhere: `
				FROM mc_work_contact_room AS contact_room
				INNER JOIN mc_work_room AS room
					ON room.id = contact_room.room_id AND room.deleted_at IS NULL
				INNER JOIN mc_corp AS scoped_corp
					ON scoped_corp.id = room.corp_id
					AND scoped_corp.tenant_id = ?
					AND scoped_corp.deleted_at IS NULL
				WHERE room.corp_id = ? AND contact_room.deleted_at IS NULL`,
			args: []any{scope.TenantID, scope.CorpID}, idExpression: "contact_room.id", employeeExpression: "room.owner_id",
		}
		switch metric {
		case corpDataMetricRoomMembers:
			source.fromWhere += " AND contact_room.status = 1"
		case corpDataMetricAddIntoRooms:
			source.fromWhere += " AND contact_room.status = 1"
			source.timeExpression = "contact_room.join_time"
		case corpDataMetricQuitRooms:
			source.fromWhere += " AND contact_room.status = 2 AND contact_room.out_time != '' AND contact_room.updated_at IS NOT NULL"
			source.timeExpression = "contact_room.updated_at"
		}
	case corpDataMetricEmployees:
		source = corpDataMetricSource{
			fromWhere: `
				FROM mc_work_employee AS employee
				INNER JOIN mc_corp AS scoped_corp
					ON scoped_corp.id = employee.corp_id
					AND scoped_corp.tenant_id = ?
					AND scoped_corp.deleted_at IS NULL
				WHERE employee.corp_id = ? AND employee.status = 1 AND employee.deleted_at IS NULL`,
			args: []any{scope.TenantID, scope.CorpID}, idExpression: "employee.id", employeeExpression: "employee.id",
		}
	}
	filter, filterArgs := corpDataEmployeeScopeSQL(scope, source.employeeExpression)
	source.fromWhere += filter
	source.args = append(source.args, filterArgs...)
	return source
}

func corpDataEmployeeScopeSQL(scope dashboard.CorpDataScope, employeeExpression string) (string, []any) {
	employeeIDs := uniquePositiveInts(scope.EmployeeIDs)
	departmentIDs := uniquePositiveInts(scope.DepartmentIDs)
	if scope.EmployeeScopeRestricted && len(employeeIDs) == 0 {
		return " AND 1 = 0", nil
	}
	if len(employeeIDs) == 0 && len(departmentIDs) == 0 {
		return "", nil
	}
	query := ` AND EXISTS (
		SELECT 1
		FROM mc_work_employee AS scoped_employee
		WHERE scoped_employee.id = ` + employeeExpression + `
		  AND scoped_employee.corp_id = ?
		  AND scoped_employee.deleted_at IS NULL`
	args := []any{scope.CorpID}
	if len(employeeIDs) > 0 {
		query += " AND scoped_employee.id IN (" + placeholders(len(employeeIDs)) + ")"
		for _, employeeID := range employeeIDs {
			args = append(args, employeeID)
		}
	}
	if len(departmentIDs) > 0 {
		query += ` AND EXISTS (
			SELECT 1
			FROM mc_work_employee_department AS employee_department
			INNER JOIN mc_work_department AS department
				ON department.id = employee_department.department_id
				AND department.corp_id = ?
				AND department.deleted_at IS NULL
			WHERE employee_department.employee_id = scoped_employee.id
			  AND employee_department.deleted_at IS NULL
			  AND department.id IN (` + placeholders(len(departmentIDs)) + `)
		)`
		args = append(args, scope.CorpID)
		for _, departmentID := range departmentIDs {
			args = append(args, departmentID)
		}
	}
	return query + ")", args
}

func corpDataMetricCountQuery(metric corpDataMetric, scope dashboard.CorpDataScope, from time.Time, to time.Time) (string, []any) {
	source := corpDataMetricSourceFor(metric, scope)
	query := "SELECT COUNT(*) " + source.fromWhere
	args := append([]any{}, source.args...)
	if !from.IsZero() && !to.IsZero() && source.timeExpression != "" {
		query += " AND " + source.timeExpression + " >= FROM_UNIXTIME(?) AND " + source.timeExpression + " < FROM_UNIXTIME(?)"
		args = append(args, from.Unix(), to.Unix())
	}
	return query, args
}

func corpDataTrendQuery(scope dashboard.CorpDataScope, from time.Time, to time.Time) (string, []any) {
	type trendMetric struct {
		metric corpDataMetric
		values [4]int
	}
	metrics := []trendMetric{
		{metric: corpDataMetricAddContacts, values: [4]int{1, 0, 0, 0}},
		{metric: corpDataMetricAddIntoRooms, values: [4]int{0, 1, 0, 0}},
		{metric: corpDataMetricLossContacts, values: [4]int{0, 0, 1, 0}},
		{metric: corpDataMetricQuitRooms, values: [4]int{0, 0, 0, 1}},
	}
	parts := make([]string, 0, len(metrics))
	args := make([]any, 0)
	for _, item := range metrics {
		source := corpDataMetricSourceFor(item.metric, scope)
		part := fmt.Sprintf(`
			SELECT %s AS event_id, %d AS add_contact_num, %d AS add_into_room_num,
				%d AS loss_contact_num, %d AS quit_room_num,
				DATE_FORMAT(CONVERT_TZ(%s, @@session.time_zone, ?), '%%Y-%%m-%%d') AS date
			%s
			AND %s >= FROM_UNIXTIME(?) AND %s < FROM_UNIXTIME(?)`,
			source.idExpression, item.values[0], item.values[1], item.values[2], item.values[3],
			source.timeExpression, source.fromWhere, source.timeExpression, source.timeExpression)
		parts = append(parts, part)
		args = append(args, corpDataTimezoneOffset)
		args = append(args, source.args...)
		args = append(args, from.Unix(), to.Unix())
	}
	return `
		SELECT MIN(event_id) AS id,
			SUM(add_contact_num) AS add_contact_num,
			SUM(add_into_room_num) AS add_into_room_num,
			SUM(loss_contact_num) AS loss_contact_num,
			SUM(quit_room_num) AS quit_room_num,
			date
		FROM (` + strings.Join(parts, " UNION ALL ") + `) AS corp_events
		GROUP BY date
		ORDER BY date ASC
		LIMIT 31
	`, args
}

func (s *MySQLStore) RefreshCorpDayData(ctx context.Context, corpID int, now time.Time) (dashboard.CorpDataCronResult, error) {
	if corpID <= 0 {
		return dashboard.CorpDataCronResult{}, fmt.Errorf("corp id must be positive")
	}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	startText := dayStart.Format("2006-01-02 15:04:05")
	endText := dayEnd.Format("2006-01-02 15:04:05")
	dateText := dayStart.Format("2006-01-02")

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.CorpDataCronResult{}, err
	}
	defer rollbackQuietly(tx)

	addContactNum, err := countScalarTx(ctx, tx, `
		SELECT COUNT(*)
		FROM mc_work_contact_employee
		WHERE deleted_at IS NULL
		  AND corp_id = ?
		  AND create_time > ?
		  AND create_time < ?
	`, corpID, startText, endText)
	if err != nil {
		return dashboard.CorpDataCronResult{}, err
	}
	addRoomNum, err := countScalarTx(ctx, tx, `
		SELECT COUNT(*)
		FROM mc_work_room
		WHERE deleted_at IS NULL
		  AND corp_id = ?
		  AND created_at > ?
		  AND created_at < ?
	`, corpID, startText, endText)
	if err != nil {
		return dashboard.CorpDataCronResult{}, err
	}
	addIntoRoomNum, err := countScalarTx(ctx, tx, `
		SELECT COUNT(*)
		FROM mc_work_contact_room AS contact_room
		INNER JOIN mc_work_room AS room ON room.id = contact_room.room_id AND room.deleted_at IS NULL
		WHERE room.corp_id = ?
		  AND contact_room.deleted_at IS NULL
		  AND contact_room.join_time > ?
		  AND contact_room.join_time < ?
		  AND contact_room.status = 1
	`, corpID, startText, endText)
	if err != nil {
		return dashboard.CorpDataCronResult{}, err
	}
	quitRoomNum, err := countScalarTx(ctx, tx, `
		SELECT COUNT(*)
		FROM mc_work_contact_room AS contact_room
		INNER JOIN mc_work_room AS room ON room.id = contact_room.room_id AND room.deleted_at IS NULL
		WHERE room.corp_id = ?
		  AND contact_room.deleted_at IS NULL
		  AND contact_room.out_time > ?
		  AND contact_room.out_time < ?
		  AND contact_room.status = 2
	`, corpID, startText, endText)
	if err != nil {
		return dashboard.CorpDataCronResult{}, err
	}
	lossContactNum, err := countScalarTx(ctx, tx, `
		SELECT COUNT(*)
		FROM mc_work_contact_employee
		WHERE corp_id = ?
		  AND deleted_at > ?
		  AND deleted_at < ?
		  AND status IN (2, 3)
	`, corpID, startText, endText)
	if err != nil {
		return dashboard.CorpDataCronResult{}, err
	}

	result := dashboard.CorpDataCronResult{
		CorpID:         corpID,
		AddContactNum:  addContactNum,
		AddRoomNum:     addRoomNum,
		AddIntoRoomNum: addIntoRoomNum,
		LossContactNum: lossContactNum,
		QuitRoomNum:    quitRoomNum,
		Date:           dateText,
	}

	var dayDataID int
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_corp_day_data
		WHERE corp_id = ? AND DATE(date) = ?
		ORDER BY id DESC
		LIMIT 1
	`, corpID, dateText).Scan(&dayDataID)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO mc_corp_day_data (
				corp_id, add_contact_num, add_room_num, add_into_room_num,
				loss_contact_num, quit_room_num, date, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		`, corpID, addContactNum, addRoomNum, addIntoRoomNum, lossContactNum, quitRoomNum, dateText)
		if err != nil {
			return dashboard.CorpDataCronResult{}, err
		}
	} else if err != nil {
		return dashboard.CorpDataCronResult{}, err
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE mc_corp_day_data
			SET add_contact_num = ?,
			    add_room_num = ?,
			    add_into_room_num = ?,
			    loss_contact_num = ?,
			    quit_room_num = ?,
			    date = ?,
			    updated_at = NOW()
			WHERE id = ?
		`, addContactNum, addRoomNum, addIntoRoomNum, lossContactNum, quitRoomNum, dateText, dayDataID)
		if err != nil {
			return dashboard.CorpDataCronResult{}, err
		}
	}

	if err := upsertWorkUpdateTimeTx(ctx, tx, corpID, 6); err != nil {
		return dashboard.CorpDataCronResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.CorpDataCronResult{}, err
	}
	return result, nil
}

func (s *MySQLStore) WorkEmployeeSyncTime(ctx context.Context, corpIDs []int) (string, error) {
	return s.workUpdateTimeByType(ctx, corpIDs, 1)
}

func (s *MySQLStore) WorkContactTagSyncTime(ctx context.Context, corpIDs []int) (string, error) {
	return s.workUpdateTimeByType(ctx, corpIDs, 3)
}

func (s *MySQLStore) WorkEmployeeSyncCredentials(ctx context.Context, corpIDs []int) ([]dashboard.WorkEmployeeSyncCredential, error) {
	corpIDs = uniquePositiveInts(corpIDs)
	if len(corpIDs) == 0 {
		return []dashboard.WorkEmployeeSyncCredential{}, nil
	}
	args := make([]any, 0, len(corpIDs))
	for _, corpID := range corpIDs {
		args = append(args, corpID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM mc_corp
		WHERE id IN (`+placeholders(len(corpIDs))+`) AND deleted_at IS NULL
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	selectedIDs := make([]int, 0, len(corpIDs))
	for rows.Next() {
		var corpID int
		if err := rows.Scan(&corpID); err != nil {
			rows.Close()
			return nil, err
		}
		selectedIDs = append(selectedIDs, corpID)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	credentials := make([]dashboard.WorkEmployeeSyncCredential, 0, len(selectedIDs))
	for _, corpID := range selectedIDs {
		item, found, err := s.loadCorpCredentialByID(ctx, s.db, corpID, false)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		secret, err := s.decodeCorpCredential(item)
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, dashboard.WorkEmployeeSyncCredential{
			CorpID: item.ID, TenantID: item.TenantID, WXCorpID: item.WXCorpID,
			EmployeeSecret: secret.EmployeeSecret, ContactSecret: secret.ContactSecret,
		})
	}
	return credentials, nil
}

func (s *MySQLStore) SyncWorkEmployees(ctx context.Context, credential dashboard.WorkEmployeeSyncCredential, departments []dashboard.WorkEmployeeSyncDepartment, employees []dashboard.WorkEmployeeSyncEmployee, followUserIDs []string, defaultPasswordHash string) (dashboard.WorkEmployeeSyncResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	defer rollbackQuietly(tx)

	departmentMap, result, err := syncWorkEmployeeDepartmentsTx(ctx, tx, credential.CorpID, departments)
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	employeeResult, err := syncWorkEmployeesTx(ctx, tx, credential, departmentMap, employees, followUserIDs, defaultPasswordHash)
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	result.EmployeesCreated = employeeResult.EmployeesCreated
	result.EmployeesUpdated = employeeResult.EmployeesUpdated
	result.UsersCreated = employeeResult.UsersCreated
	result.RelationsCreated = employeeResult.RelationsCreated
	result.RelationsUpdated = employeeResult.RelationsUpdated
	result.RelationsDeleted = employeeResult.RelationsDeleted

	if err := upsertWorkUpdateTimeTx(ctx, tx, credential.CorpID, 1); err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	return result, nil
}

func (s *MySQLStore) DeleteWorkEmployeeByWXUserID(ctx context.Context, corpID int, wxUserID string) (bool, error) {
	wxUserID = strings.TrimSpace(wxUserID)
	if corpID <= 0 || wxUserID == "" {
		return false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)

	var employeeID int
	var mobile sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT id, mobile
		FROM mc_work_employee
		WHERE corp_id = ? AND wx_user_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, corpID, wxUserID).Scan(&employeeID, &mobile)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if employeeID <= 0 {
		return false, nil
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_employee
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, employeeID, corpID); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_employee_department
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE employee_id = ? AND deleted_at IS NULL
	`, employeeID); err != nil {
		return false, err
	}
	if phone := strings.TrimSpace(nullString(mobile)); phone != "" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_user
			SET status = 2, updated_at = NOW()
			WHERE phone = ? AND deleted_at IS NULL
		`, phone); err != nil {
			return false, err
		}
	}
	if err := upsertWorkUpdateTimeTx(ctx, tx, corpID, 1); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) workUpdateTimeByType(ctx context.Context, corpIDs []int, updateType int) (string, error) {
	if len(corpIDs) == 0 {
		return "", nil
	}
	args := make([]any, 0, len(corpIDs)+1)
	for _, corpID := range corpIDs {
		args = append(args, corpID)
	}
	args = append(args, updateType)

	row := s.db.QueryRowContext(ctx, `
		SELECT last_update_time
		FROM mc_work_update_time
		WHERE corp_id IN (`+placeholders(len(corpIDs))+`) AND type = ?
		ORDER BY id DESC
		LIMIT 1
	`, args...)

	var lastUpdateTime sql.NullTime
	err := row.Scan(&lastUpdateTime)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return formatTime(lastUpdateTime), nil
}

type workEmployeeSyncDepartmentRow struct {
	ID             int
	WXDepartmentID int
	Name           string
	WXParentID     int
	Order          int
}

func (s *MySQLStore) SyncWorkDepartment(ctx context.Context, corpID int, department dashboard.WorkDepartmentEventDepartment) (dashboard.WorkEmployeeSyncResult, error) {
	if corpID <= 0 || department.WXDepartmentID <= 0 {
		return dashboard.WorkEmployeeSyncResult{}, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	defer rollbackQuietly(tx)

	existing, err := workDepartmentEventRowsByWX(ctx, tx, corpID)
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	current, found := existing[department.WXDepartmentID]
	var result dashboard.WorkEmployeeSyncResult
	if !found {
		current = workEmployeeSyncDepartmentRow{
			WXDepartmentID: department.WXDepartmentID,
			Name:           department.Name,
			WXParentID:     department.WXParentID,
			Order:          department.Order,
		}
		insert, err := tx.ExecContext(ctx, `
			INSERT INTO mc_work_department (wx_department_id, corp_id, name, parent_id, wx_parentid, `+"`order`"+`, level, path, created_at, updated_at)
			VALUES (?, ?, ?, 0, ?, ?, 0, ' ', NOW(), NOW())
		`, current.WXDepartmentID, corpID, current.Name, current.WXParentID, current.Order)
		if err != nil {
			return dashboard.WorkEmployeeSyncResult{}, err
		}
		id, err := insert.LastInsertId()
		if err != nil {
			return dashboard.WorkEmployeeSyncResult{}, err
		}
		current.ID = int(id)
		existing[current.WXDepartmentID] = current
		result.DepartmentsCreated++
	} else {
		if department.HasName {
			current.Name = department.Name
		}
		if department.HasParent {
			current.WXParentID = department.WXParentID
		}
		if department.HasOrder {
			current.Order = department.Order
		}
		existing[current.WXDepartmentID] = current
	}

	parentID, path, level := workEmployeeDepartmentRelation(current.WXDepartmentID, existing, map[int]struct{}{})
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_department
		SET name = ?, wx_parentid = ?, parent_id = ?, `+"`order`"+` = ?, level = ?, path = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, current.Name, current.WXParentID, parentID, current.Order, level, path, current.ID, corpID); err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	if found {
		result.DepartmentsUpdated++
	}
	if err := upsertWorkUpdateTimeTx(ctx, tx, corpID, 1); err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	return result, nil
}

func (s *MySQLStore) DeleteWorkDepartmentByWXDepartmentID(ctx context.Context, corpID int, wxDepartmentID int) (bool, error) {
	if corpID <= 0 || wxDepartmentID <= 0 {
		return false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)

	result, err := tx.ExecContext(ctx, `
		UPDATE mc_work_department
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE corp_id = ? AND wx_department_id = ? AND deleted_at IS NULL
	`, corpID, wxDepartmentID)
	if err != nil {
		return false, err
	}
	affected, err := rowsAffectedInt(result)
	if err != nil {
		return false, err
	}
	if affected > 0 {
		if err := upsertWorkUpdateTimeTx(ctx, tx, corpID, 1); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return affected > 0, nil
}

func workDepartmentEventRowsByWX(ctx context.Context, tx *sql.Tx, corpID int) (map[int]workEmployeeSyncDepartmentRow, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, wx_department_id, name, wx_parentid, `+"`order`"+`
		FROM mc_work_department
		WHERE corp_id = ? AND deleted_at IS NULL
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[int]workEmployeeSyncDepartmentRow{}
	for rows.Next() {
		var row workEmployeeSyncDepartmentRow
		var name sql.NullString
		if err := rows.Scan(&row.ID, &row.WXDepartmentID, &name, &row.WXParentID, &row.Order); err != nil {
			return nil, err
		}
		row.Name = nullString(name)
		if row.WXDepartmentID > 0 {
			result[row.WXDepartmentID] = row
		}
	}
	return result, rows.Err()
}

func syncWorkEmployeeDepartmentsTx(ctx context.Context, tx *sql.Tx, corpID int, departments []dashboard.WorkEmployeeSyncDepartment) (map[int]workEmployeeSyncDepartmentRow, dashboard.WorkEmployeeSyncResult, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, wx_department_id, wx_parentid
		FROM mc_work_department
		WHERE corp_id = ? AND deleted_at IS NULL
	`, corpID)
	if err != nil {
		return nil, dashboard.WorkEmployeeSyncResult{}, err
	}
	existing := map[int]workEmployeeSyncDepartmentRow{}
	for rows.Next() {
		var row workEmployeeSyncDepartmentRow
		if err := rows.Scan(&row.ID, &row.WXDepartmentID, &row.WXParentID); err != nil {
			rows.Close()
			return nil, dashboard.WorkEmployeeSyncResult{}, err
		}
		if row.WXDepartmentID > 0 {
			existing[row.WXDepartmentID] = row
		}
	}
	if err := rows.Close(); err != nil {
		return nil, dashboard.WorkEmployeeSyncResult{}, err
	}
	if err := rows.Err(); err != nil {
		return nil, dashboard.WorkEmployeeSyncResult{}, err
	}

	remoteByWX := map[int]dashboard.WorkEmployeeSyncDepartment{}
	wxOrder := make([]int, 0, len(departments))
	for _, department := range departments {
		if department.WXDepartmentID <= 0 {
			continue
		}
		if _, ok := remoteByWX[department.WXDepartmentID]; !ok {
			wxOrder = append(wxOrder, department.WXDepartmentID)
		}
		remoteByWX[department.WXDepartmentID] = department
	}

	var result dashboard.WorkEmployeeSyncResult
	for _, wxDepartmentID := range wxOrder {
		department := remoteByWX[wxDepartmentID]
		if current, ok := existing[wxDepartmentID]; ok {
			current.WXParentID = department.WXParentID
			existing[wxDepartmentID] = current
			continue
		}
		insert, err := tx.ExecContext(ctx, `
			INSERT INTO mc_work_department (wx_department_id, corp_id, name, parent_id, wx_parentid, `+"`order`"+`, level, path, created_at, updated_at)
			VALUES (?, ?, ?, 0, ?, ?, 0, ' ', NOW(), NOW())
		`, wxDepartmentID, corpID, department.Name, department.WXParentID, department.Order)
		if err != nil {
			return nil, dashboard.WorkEmployeeSyncResult{}, err
		}
		id, err := insert.LastInsertId()
		if err != nil {
			return nil, dashboard.WorkEmployeeSyncResult{}, err
		}
		existing[wxDepartmentID] = workEmployeeSyncDepartmentRow{
			ID:             int(id),
			WXDepartmentID: wxDepartmentID,
			WXParentID:     department.WXParentID,
		}
		result.DepartmentsCreated++
	}

	for _, wxDepartmentID := range wxOrder {
		department := remoteByWX[wxDepartmentID]
		current := existing[wxDepartmentID]
		parentID, path, level := workEmployeeDepartmentRelation(wxDepartmentID, existing, map[int]struct{}{})
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_work_department
			SET name = ?, wx_parentid = ?, parent_id = ?, `+"`order`"+` = ?, level = ?, path = ?, updated_at = NOW()
			WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
		`, department.Name, department.WXParentID, parentID, department.Order, level, path, current.ID, corpID); err != nil {
			return nil, dashboard.WorkEmployeeSyncResult{}, err
		}
		result.DepartmentsUpdated++
	}

	return existing, result, nil
}

func workEmployeeDepartmentRelation(wxDepartmentID int, departments map[int]workEmployeeSyncDepartmentRow, visiting map[int]struct{}) (int, string, int) {
	department, ok := departments[wxDepartmentID]
	if !ok || department.ID <= 0 {
		return 0, "", 0
	}
	path := "#" + strconv.Itoa(department.ID) + "#"
	if department.WXParentID <= 0 {
		return 0, path, 0
	}
	parent, ok := departments[department.WXParentID]
	if !ok || parent.ID <= 0 {
		return 0, path, 0
	}
	if _, cycle := visiting[wxDepartmentID]; cycle {
		return 0, path, 0
	}
	visiting[wxDepartmentID] = struct{}{}
	_, parentPath, parentLevel := workEmployeeDepartmentRelation(department.WXParentID, departments, visiting)
	if parentPath == "" {
		return parent.ID, path, 1
	}
	return parent.ID, parentPath + "-" + path, parentLevel + 1
}

func syncWorkEmployeesTx(ctx context.Context, tx *sql.Tx, credential dashboard.WorkEmployeeSyncCredential, departments map[int]workEmployeeSyncDepartmentRow, employees []dashboard.WorkEmployeeSyncEmployee, followUserIDs []string, defaultPasswordHash string) (dashboard.WorkEmployeeSyncResult, error) {
	var result dashboard.WorkEmployeeSyncResult
	employeeByWX := map[string]dashboard.WorkEmployeeSyncEmployee{}
	wxOrder := make([]string, 0, len(employees))
	for _, employee := range employees {
		wxUserID := strings.TrimSpace(employee.WXUserID)
		if wxUserID == "" {
			continue
		}
		employee.WXUserID = wxUserID
		if _, ok := employeeByWX[wxUserID]; !ok {
			wxOrder = append(wxOrder, wxUserID)
		}
		employeeByWX[wxUserID] = employee
	}
	if len(wxOrder) == 0 {
		return result, nil
	}

	followSet := map[string]struct{}{}
	for _, wxUserID := range followUserIDs {
		wxUserID = strings.TrimSpace(wxUserID)
		if wxUserID != "" {
			followSet[wxUserID] = struct{}{}
		}
	}

	existingEmployees, err := workEmployeeRowsByWX(ctx, tx, credential.CorpID)
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	userIDsByPhone, _, err := workEmployeeSyncUsersByPhone(ctx, tx, credential.TenantID, employeeByWX)
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	employeeIDsByWX := make(map[string]int, len(wxOrder))
	for _, wxUserID := range wxOrder {
		employee := employeeByWX[wxUserID]
		mainDepartmentID := syncMainDepartmentID(employee, departments)
		logUserID := 0
		if employee.Mobile != "" {
			logUserID = userIDsByPhone[strings.TrimSpace(employee.Mobile)]
		}
		contactAuth := 2
		if _, ok := followSet[wxUserID]; ok {
			contactAuth = 1
		}
		values := []any{
			wxUserID,
			credential.CorpID,
			employee.Name,
			employee.Mobile,
			employee.Position,
			employee.Gender,
			employee.Email,
			employee.Avatar,
			employee.ThumbAvatar,
			employee.Telephone,
			employee.Alias,
			jsonRawOrEmptyArray(employee.ExtAttr),
			employee.Status,
			employee.QRCode,
			jsonRawOrEmptyArray(employee.ExternalProfile),
			jsonRawOrEmptyArray(employee.ExternalPosition),
			employee.Address,
			employee.OpenUserID,
			employee.WXMainDepartmentID,
			mainDepartmentID,
			logUserID,
			contactAuth,
		}
		if employeeID, ok := existingEmployees[wxUserID]; ok {
			updateArgs := append([]any{}, values[2:]...)
			updateArgs = append(updateArgs, employeeID, credential.CorpID)
			if _, err := tx.ExecContext(ctx, `
				UPDATE mc_work_employee
				SET name = ?, mobile = ?, position = ?, gender = ?, email = ?, avatar = ?, thumb_avatar = ?, telephone = ?, alias = ?,
				    extattr = ?, status = ?, qr_code = ?, external_profile = ?, external_position = ?, address = ?, open_user_id = ?,
				    wx_main_department_id = ?, main_department_id = ?, log_user_id = ?, contact_auth = ?, updated_at = NOW()
				WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
			`, updateArgs...); err != nil {
				return dashboard.WorkEmployeeSyncResult{}, err
			}
			employeeIDsByWX[wxUserID] = employeeID
			result.EmployeesUpdated++
			if err := disableWorkEmployeeUserIfNeededTx(ctx, tx, employee); err != nil {
				return dashboard.WorkEmployeeSyncResult{}, err
			}
			continue
		}
		insert, err := tx.ExecContext(ctx, `
			INSERT INTO mc_work_employee (
				wx_user_id, corp_id, name, mobile, position, gender, email, avatar, thumb_avatar, telephone, alias,
				extattr, status, qr_code, external_profile, external_position, address, open_user_id,
				wx_main_department_id, main_department_id, log_user_id, contact_auth, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		`, values...)
		if err != nil {
			return dashboard.WorkEmployeeSyncResult{}, err
		}
		id, err := insert.LastInsertId()
		if err != nil {
			return dashboard.WorkEmployeeSyncResult{}, err
		}
		employeeIDsByWX[wxUserID] = int(id)
		result.EmployeesCreated++
		if err := disableWorkEmployeeUserIfNeededTx(ctx, tx, employee); err != nil {
			return dashboard.WorkEmployeeSyncResult{}, err
		}
	}

	relationResult, err := syncWorkEmployeeRelationsTx(ctx, tx, credential.CorpID, wxOrder, employeeByWX, employeeIDsByWX, departments)
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	result.RelationsCreated = relationResult.RelationsCreated
	result.RelationsUpdated = relationResult.RelationsUpdated
	result.RelationsDeleted = relationResult.RelationsDeleted
	return result, nil
}

func disableWorkEmployeeUserIfNeededTx(ctx context.Context, tx *sql.Tx, employee dashboard.WorkEmployeeSyncEmployee) error {
	if employee.Status != 2 {
		return nil
	}
	phone := strings.TrimSpace(employee.Mobile)
	if phone == "" {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE mc_user
		SET status = 2, updated_at = NOW()
		WHERE phone = ? AND deleted_at IS NULL
	`, phone)
	return err
}

func workEmployeeRowsByWX(ctx context.Context, tx *sql.Tx, corpID int) (map[string]int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, wx_user_id
		FROM mc_work_employee
		WHERE corp_id = ? AND deleted_at IS NULL
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]int{}
	for rows.Next() {
		var id int
		var wxUserID sql.NullString
		if err := rows.Scan(&id, &wxUserID); err != nil {
			return nil, err
		}
		if value := strings.TrimSpace(nullString(wxUserID)); value != "" {
			result[value] = id
		}
	}
	return result, rows.Err()
}

func workEmployeeSyncUsersByPhone(ctx context.Context, tx *sql.Tx, tenantID int, employees map[string]dashboard.WorkEmployeeSyncEmployee) (map[string]int, map[string]bool, error) {
	phoneSet := map[string]struct{}{}
	for _, employee := range employees {
		phone := strings.TrimSpace(employee.Mobile)
		if phone != "" {
			phoneSet[phone] = struct{}{}
		}
	}
	if len(phoneSet) == 0 {
		return map[string]int{}, map[string]bool{}, nil
	}
	phones := make([]string, 0, len(phoneSet))
	for phone := range phoneSet {
		phones = append(phones, phone)
	}
	sort.Strings(phones)
	args := make([]any, 0, len(phones))
	for _, phone := range phones {
		args = append(args, phone)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, phone, tenant_id
		FROM mc_user
		WHERE phone IN (`+placeholders(len(phones))+`) AND deleted_at IS NULL
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	userIDsByPhone := map[string]int{}
	phoneExists := map[string]bool{}
	for rows.Next() {
		var id int
		var phone sql.NullString
		var rowTenantID int
		if err := rows.Scan(&id, &phone, &rowTenantID); err != nil {
			return nil, nil, err
		}
		phoneValue := strings.TrimSpace(nullString(phone))
		if phoneValue == "" {
			continue
		}
		phoneExists[phoneValue] = true
		if rowTenantID == tenantID {
			if _, exists := userIDsByPhone[phoneValue]; !exists {
				userIDsByPhone[phoneValue] = id
			}
		}
	}
	return userIDsByPhone, phoneExists, rows.Err()
}

type workEmployeeSyncRelation struct {
	ID           int
	EmployeeID   int
	DepartmentID int
}

type workEmployeeSyncDesiredRelation struct {
	EmployeeID   int
	DepartmentID int
	Leader       int
	Order        int
}

func syncWorkEmployeeRelationsTx(ctx context.Context, tx *sql.Tx, corpID int, wxOrder []string, employees map[string]dashboard.WorkEmployeeSyncEmployee, employeeIDsByWX map[string]int, departments map[int]workEmployeeSyncDepartmentRow) (dashboard.WorkEmployeeSyncResult, error) {
	var result dashboard.WorkEmployeeSyncResult
	employeeIDs := make([]int, 0, len(employeeIDsByWX))
	seenEmployeeID := map[int]struct{}{}
	for _, employeeID := range employeeIDsByWX {
		if employeeID <= 0 {
			continue
		}
		if _, ok := seenEmployeeID[employeeID]; ok {
			continue
		}
		seenEmployeeID[employeeID] = struct{}{}
		employeeIDs = append(employeeIDs, employeeID)
	}
	sort.Ints(employeeIDs)
	if len(employeeIDs) == 0 {
		return result, nil
	}

	existing, err := workEmployeeSyncRelations(ctx, tx, corpID, employeeIDs)
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	desired := map[string]workEmployeeSyncDesiredRelation{}
	for _, wxUserID := range wxOrder {
		employeeID := employeeIDsByWX[wxUserID]
		if employeeID <= 0 {
			continue
		}
		employee := employees[wxUserID]
		for index, wxDepartmentID := range employee.DepartmentIDs {
			department, ok := departments[wxDepartmentID]
			if !ok || department.ID <= 0 {
				continue
			}
			key := workEmployeeSyncRelationKey(employeeID, department.ID)
			desired[key] = workEmployeeSyncDesiredRelation{
				EmployeeID:   employeeID,
				DepartmentID: department.ID,
				Leader:       indexedIntStore(employee.IsLeaderInDepartment, index),
				Order:        indexedIntStore(employee.DepartmentOrders, index),
			}
		}
	}

	for key, relation := range desired {
		if current, ok := existing[key]; ok {
			if _, err := tx.ExecContext(ctx, `
				UPDATE mc_work_employee_department
				SET is_leader_in_dept = ?, `+"`order`"+` = ?, updated_at = NOW()
				WHERE id = ? AND deleted_at IS NULL
			`, relation.Leader, relation.Order, current.ID); err != nil {
				return dashboard.WorkEmployeeSyncResult{}, err
			}
			result.RelationsUpdated++
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_work_employee_department (employee_id, department_id, is_leader_in_dept, `+"`order`"+`, created_at, updated_at)
			VALUES (?, ?, ?, ?, NOW(), NOW())
		`, relation.EmployeeID, relation.DepartmentID, relation.Leader, relation.Order); err != nil {
			return dashboard.WorkEmployeeSyncResult{}, err
		}
		result.RelationsCreated++
	}

	deleteIDs := make([]int, 0)
	for key, relation := range existing {
		if _, ok := desired[key]; ok {
			continue
		}
		deleteIDs = append(deleteIDs, relation.ID)
	}
	if len(deleteIDs) > 0 {
		sort.Ints(deleteIDs)
		args := make([]any, 0, len(deleteIDs))
		for _, id := range deleteIDs {
			args = append(args, id)
		}
		deleteResult, err := tx.ExecContext(ctx, `
			UPDATE mc_work_employee_department
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE id IN (`+placeholders(len(deleteIDs))+`) AND deleted_at IS NULL
		`, args...)
		if err != nil {
			return dashboard.WorkEmployeeSyncResult{}, err
		}
		affected, err := deleteResult.RowsAffected()
		if err != nil {
			return dashboard.WorkEmployeeSyncResult{}, err
		}
		result.RelationsDeleted = int(affected)
	}
	return result, nil
}

func workEmployeeSyncRelations(ctx context.Context, tx *sql.Tx, corpID int, employeeIDs []int) (map[string]workEmployeeSyncRelation, error) {
	args := make([]any, 0, len(employeeIDs)+1)
	args = append(args, corpID)
	for _, employeeID := range employeeIDs {
		args = append(args, employeeID)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT wed.id, wed.employee_id, wed.department_id
		FROM mc_work_employee_department wed
		INNER JOIN mc_work_employee e ON e.id = wed.employee_id AND e.deleted_at IS NULL
		WHERE e.corp_id = ? AND wed.employee_id IN (`+placeholders(len(employeeIDs))+`) AND wed.deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]workEmployeeSyncRelation{}
	for rows.Next() {
		var relation workEmployeeSyncRelation
		if err := rows.Scan(&relation.ID, &relation.EmployeeID, &relation.DepartmentID); err != nil {
			return nil, err
		}
		result[workEmployeeSyncRelationKey(relation.EmployeeID, relation.DepartmentID)] = relation
	}
	return result, rows.Err()
}

func syncMainDepartmentID(employee dashboard.WorkEmployeeSyncEmployee, departments map[int]workEmployeeSyncDepartmentRow) int {
	if employee.WXMainDepartmentID > 0 {
		if department, ok := departments[employee.WXMainDepartmentID]; ok {
			return department.ID
		}
	}
	for _, wxDepartmentID := range employee.DepartmentIDs {
		if department, ok := departments[wxDepartmentID]; ok {
			return department.ID
		}
	}
	return 0
}

func workEmployeeSyncRelationKey(employeeID int, departmentID int) string {
	return strconv.Itoa(employeeID) + ":" + strconv.Itoa(departmentID)
}

func indexedIntStore(values []int, index int) int {
	if index < 0 || index >= len(values) {
		return 0
	}
	return values[index]
}

func jsonRawOrEmptyArray(value json.RawMessage) string {
	text := strings.TrimSpace(string(value))
	if text == "" || text == "null" || !json.Valid([]byte(text)) {
		return "[]"
	}
	return text
}

func upsertWorkUpdateTimeTx(ctx context.Context, tx *sql.Tx, corpID int, updateType int) error {
	var updateTimeID int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_update_time
		WHERE corp_id = ? AND type = ?
		ORDER BY id DESC
		LIMIT 1
	`, corpID, updateType).Scan(&updateTimeID)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO mc_work_update_time (corp_id, type, last_update_time, created_at, updated_at)
			VALUES (?, ?, NOW(), NOW(), NOW())
		`, corpID, updateType)
		return err
	}
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE mc_work_update_time
		SET last_update_time = NOW(), updated_at = NOW()
		WHERE id = ?
	`, updateTimeID)
	return err
}

func (s *MySQLStore) WorkDepartmentsByCorp(ctx context.Context, corpID int, search string) ([]dashboard.WorkDepartment, error) {
	query := `
		SELECT id, wx_department_id, corp_id, name, parent_id, wx_parentid, ` + "`order`" + `, level, path, created_at, updated_at
		FROM mc_work_department
		WHERE corp_id = ? AND deleted_at IS NULL
	`
	args := []any{corpID}
	if search != "" {
		query += " AND name LIKE ? ORDER BY `order` DESC"
		args = append(args, "%"+search+"%")
	} else {
		query += " ORDER BY id ASC"
	}
	query += " LIMIT 1000"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var departments []dashboard.WorkDepartment
	for rows.Next() {
		var department dashboard.WorkDepartment
		var createdAt sql.NullTime
		var updatedAt sql.NullTime
		if err := rows.Scan(
			&department.ID, &department.WXDepartmentID, &department.CorpID,
			&department.Name, &department.ParentID, &department.WXParentID,
			&department.Order, &department.Level, &department.Path, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		department.CreatedAt = formatTime(createdAt)
		department.UpdatedAt = formatTime(updatedAt)
		departments = append(departments, department)
	}
	return departments, rows.Err()
}

func (s *MySQLStore) ActiveWorkEmployeesByCorp(ctx context.Context, corpID int, search string) ([]dashboard.WorkDepartmentEmployee, error) {
	query := `
		SELECT id, name, wx_user_id, avatar
		FROM mc_work_employee
		WHERE corp_id = ? AND status = 1 AND deleted_at IS NULL
	`
	args := []any{corpID}
	if search != "" {
		query += " AND name LIKE ?"
		args = append(args, "%"+search+"%")
	}
	query += " ORDER BY id ASC LIMIT 1000"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var employees []dashboard.WorkDepartmentEmployee
	for rows.Next() {
		var employee dashboard.WorkDepartmentEmployee
		if err := rows.Scan(&employee.ID, &employee.Name, &employee.WXUserID, &employee.Avatar); err != nil {
			return nil, err
		}
		employees = append(employees, employee)
	}
	return employees, rows.Err()
}

func (s *MySQLStore) WorkDepartmentMembers(ctx context.Context, corpID int, departmentIDs []int) ([]dashboard.WorkDepartmentMember, error) {
	if len(departmentIDs) == 0 {
		return []dashboard.WorkDepartmentMember{}, nil
	}
	args := make([]any, 0, len(departmentIDs)+1)
	args = append(args, corpID)
	for _, departmentID := range departmentIDs {
		args = append(args, departmentID)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT wed.employee_id, wed.department_id, COALESCE(d.name, ''), COALESCE(e.name, '')
		FROM mc_work_employee_department wed
		INNER JOIN mc_work_employee e ON e.id = wed.employee_id
		LEFT JOIN mc_work_department d ON d.id = wed.department_id AND d.deleted_at IS NULL
		WHERE e.corp_id = ? AND e.status = 1 AND e.deleted_at IS NULL
		  AND wed.deleted_at IS NULL
		  AND wed.department_id IN (`+placeholders(len(departmentIDs))+`)
		ORDER BY wed.id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]dashboard.WorkDepartmentMember, 0)
	for rows.Next() {
		var member dashboard.WorkDepartmentMember
		if err := rows.Scan(&member.EmployeeID, &member.DepartmentID, &member.DepartmentName, &member.EmployeeName); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (s *MySQLStore) WorkDepartmentsByEmployeeMobile(ctx context.Context, corpID int, phone string) ([]dashboard.WorkDepartmentPhoneOption, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT d.corp_id, d.id, d.name
		FROM mc_work_employee e
		INNER JOIN mc_work_employee_department wed ON wed.employee_id = e.id AND wed.deleted_at IS NULL
		INNER JOIN mc_work_department d ON d.id = wed.department_id AND d.deleted_at IS NULL
		WHERE e.corp_id = ? AND e.mobile = ? AND e.deleted_at IS NULL
		ORDER BY d.id ASC
	`, corpID, phone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	departments := make([]dashboard.WorkDepartmentPhoneOption, 0)
	for rows.Next() {
		var department dashboard.WorkDepartmentPhoneOption
		if err := rows.Scan(&department.CorpID, &department.WorkDepartmentID, &department.WorkDepartmentName); err != nil {
			return nil, err
		}
		departments = append(departments, department)
	}
	return departments, rows.Err()
}

func (s *MySQLStore) WorkDepartmentEmployeePage(ctx context.Context, filter dashboard.WorkDepartmentEmployeeListFilter) (dashboard.WorkDepartmentEmployeePage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10
	}

	where := "WHERE e.corp_id = ? AND e.deleted_at IS NULL"
	args := []any{filter.CorpID}
	if filter.DepartmentID > 0 {
		where += " AND e.id IN (SELECT wed.employee_id FROM mc_work_employee_department wed WHERE wed.department_id = ? AND wed.deleted_at IS NULL)"
		args = append(args, filter.DepartmentID)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_work_employee e `+where, args...).Scan(&total); err != nil {
		return dashboard.WorkDepartmentEmployeePage{}, err
	}
	if total == 0 {
		return dashboard.WorkDepartmentEmployeePage{Items: []dashboard.WorkDepartmentEmployeeListItem{}}, nil
	}

	totalPage := (total + filter.PerPage - 1) / filter.PerPage
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.name, e.mobile,
		       COALESCE((
		         SELECT rr.name
		         FROM mc_rbac_user_role rur
		         INNER JOIN mc_rbac_role rr ON rr.id = rur.role_id AND rr.deleted_at IS NULL
		         WHERE rur.user_id = e.log_user_id AND rur.deleted_at IS NULL
		         ORDER BY rur.id ASC
		         LIMIT 1
		       ), '') AS role_name
		FROM mc_work_employee e
		`+where+`
		ORDER BY e.updated_at DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.WorkDepartmentEmployeePage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.WorkDepartmentEmployeeListItem, 0)
	for rows.Next() {
		var item dashboard.WorkDepartmentEmployeeListItem
		if err := rows.Scan(&item.EmployeeID, &item.EmployeeName, &item.Phone, &item.RoleName); err != nil {
			return dashboard.WorkDepartmentEmployeePage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkDepartmentEmployeePage{}, err
	}
	return dashboard.WorkDepartmentEmployeePage{Items: items, Total: total, TotalPage: totalPage}, nil
}

func (s *MySQLStore) WorkEmployeeIndexPage(ctx context.Context, filter dashboard.WorkEmployeeIndexFilter) (dashboard.WorkEmployeeIndexPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10
	}
	if len(filter.CorpIDs) == 0 {
		return dashboard.WorkEmployeeIndexPage{Items: []dashboard.WorkEmployeeIndexItem{}}, nil
	}
	if filter.RestrictEmployeeIDs && len(filter.EmployeeIDs) == 0 {
		return dashboard.WorkEmployeeIndexPage{Items: []dashboard.WorkEmployeeIndexItem{}}, nil
	}

	where := "WHERE e.deleted_at IS NULL AND e.corp_id IN (" + placeholders(len(filter.CorpIDs)) + ")"
	args := make([]any, 0, len(filter.CorpIDs)+len(filter.EmployeeIDs)+4)
	for _, corpID := range filter.CorpIDs {
		args = append(args, corpID)
	}
	if filter.RestrictEmployeeIDs {
		where += " AND e.id IN (" + placeholders(len(filter.EmployeeIDs)) + ")"
		for _, employeeID := range filter.EmployeeIDs {
			args = append(args, employeeID)
		}
	}
	if filter.Name != "" {
		where += " AND e.name LIKE ?"
		args = append(args, "%"+filter.Name+"%")
	}
	if filter.Status > 0 {
		where += " AND e.status = ?"
		args = append(args, filter.Status)
	}
	if filter.ContactAuth != "" && filter.ContactAuth != "all" {
		where += " AND e.contact_auth = ?"
		args = append(args, filter.ContactAuth)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_work_employee e `+where, args...).Scan(&total); err != nil {
		return dashboard.WorkEmployeeIndexPage{}, err
	}
	if total == 0 {
		return dashboard.WorkEmployeeIndexPage{Items: []dashboard.WorkEmployeeIndexItem{}}, nil
	}

	totalPage := (total + filter.PerPage - 1) / filter.PerPage
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.name, e.thumb_avatar, e.status, e.contact_auth, e.wx_user_id, e.corp_id, e.gender,
		       COALESCE(stat.chat_cnt, 0),
		       COALESCE(stat.message_cnt, 0),
		       COALESCE(stat.reply_percentage, 0),
		       COALESCE(stat.new_contact_cnt, 0),
		       COALESCE(stat.new_apply_cnt, 0),
		       COALESCE(stat.negative_feedback_cnt, 0),
		       COALESCE(stat.avg_reply_time, 0)
		FROM mc_work_employee e
		LEFT JOIN (
			SELECT s1.*
			FROM mc_work_employee_statistic s1
			INNER JOIN (
				SELECT employee_id, MAX(id) AS id
				FROM mc_work_employee_statistic
				WHERE deleted_at IS NULL
				GROUP BY employee_id
			) latest ON latest.id = s1.id
		) stat ON stat.employee_id = e.id
		`+where+`
		ORDER BY e.updated_at DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.WorkEmployeeIndexPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.WorkEmployeeIndexItem, 0)
	for rows.Next() {
		var item dashboard.WorkEmployeeIndexItem
		var replyPercentage int
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.ThumbAvatar,
			&item.Status,
			&item.ContactAuth,
			&item.WXUserID,
			&item.CorpID,
			&item.Gender,
			&item.MessageNums,
			&item.SendMessageNums,
			&replyPercentage,
			&item.AddNums,
			&item.ApplyNums,
			&item.InvalidContact,
			&item.AverageReply,
		); err != nil {
			return dashboard.WorkEmployeeIndexPage{}, err
		}
		item.ReplyMessageRatio = formatPercentRatio(replyPercentage)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkEmployeeIndexPage{}, err
	}
	return dashboard.WorkEmployeeIndexPage{Items: items, Total: total, TotalPage: totalPage}, nil
}

func (s *MySQLStore) EmployeeStatisticTargets(ctx context.Context) ([]dashboard.EmployeeStatisticTarget, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.corp_id, COALESCE(e.wx_user_id, '')
		FROM mc_work_employee e
		JOIN mc_corp c ON c.id = e.corp_id AND c.deleted_at IS NULL
		WHERE e.deleted_at IS NULL
		ORDER BY e.id DESC
	`)
	if err != nil {
		return nil, err
	}
	targets := make([]dashboard.EmployeeStatisticTarget, 0)
	for rows.Next() {
		var target dashboard.EmployeeStatisticTarget
		if err := rows.Scan(&target.ID, &target.CorpID, &target.WXUserID); err != nil {
			rows.Close()
			return nil, err
		}
		targets = append(targets, target)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	credentialCache := map[int]corpCredentialRecord{}
	secretCache := map[int]wecomcredentials.CorpCredential{}
	for index := range targets {
		corpID := targets[index].CorpID
		item, found := credentialCache[corpID]
		if !found {
			loaded, exists, err := s.loadCorpCredentialByID(ctx, s.db, corpID, false)
			if err != nil {
				return nil, err
			}
			if !exists {
				continue
			}
			item = loaded
			credentialCache[corpID] = loaded
			secret, err := s.decodeCorpCredential(loaded)
			if err != nil {
				return nil, err
			}
			secretCache[corpID] = secret
		}
		targets[index].WXCorpID = item.WXCorpID
		targets[index].ContactSecret = secretCache[corpID].ContactSecret
	}
	return targets, nil
}

func (s *MySQLStore) InsertEmployeeStatistic(ctx context.Context, record dashboard.EmployeeStatisticRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_work_employee_statistic (
			corp_id, employee_id, new_apply_cnt, new_contact_cnt, chat_cnt, message_cnt,
			reply_percentage, avg_reply_time, negative_feedback_cnt, syn_time, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, record.CorpID, record.EmployeeID, record.NewApplyCnt, record.NewContactCnt, record.ChatCnt, record.MessageCnt, record.ReplyPercentage, record.AvgReplyTime, record.NegativeFeedbackCnt, record.SynTime)
	return err
}

func (s *MySQLStore) WorkContactTagGroupsByCorp(ctx context.Context, corpIDs []int) ([]dashboard.WorkContactTagGroup, error) {
	if len(corpIDs) == 0 {
		return []dashboard.WorkContactTagGroup{}, nil
	}
	args := make([]any, 0, len(corpIDs))
	for _, corpID := range corpIDs {
		args = append(args, corpID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, wx_group_id, group_name
		FROM mc_work_contact_tag_group
		WHERE deleted_at IS NULL AND corp_id IN (`+placeholders(len(corpIDs))+`)
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make([]dashboard.WorkContactTagGroup, 0)
	for rows.Next() {
		var group dashboard.WorkContactTagGroup
		var wxGroupID sql.NullString
		if err := rows.Scan(&group.ID, &wxGroupID, &group.GroupName); err != nil {
			return nil, err
		}
		group.WXGroupID = nullString(wxGroupID)
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *MySQLStore) WorkContactTagGroupByID(ctx context.Context, groupID int) (dashboard.WorkContactTagGroup, bool, error) {
	var group dashboard.WorkContactTagGroup
	var wxGroupID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, wx_group_id, group_name
		FROM mc_work_contact_tag_group
		WHERE id = ? AND deleted_at IS NULL
	`, groupID).Scan(&group.ID, &wxGroupID, &group.GroupName)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WorkContactTagGroup{}, false, nil
	}
	if err != nil {
		return dashboard.WorkContactTagGroup{}, false, err
	}
	group.WXGroupID = nullString(wxGroupID)
	return group, true, nil
}

func (s *MySQLStore) WorkContactTagPage(ctx context.Context, filter dashboard.WorkContactTagFilter) (dashboard.WorkContactTagPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 20
	}
	if len(filter.CorpIDs) == 0 {
		return dashboard.WorkContactTagPage{Items: []dashboard.WorkContactTagItem{}, PerPage: filter.PerPage}, nil
	}

	where := "WHERE t.deleted_at IS NULL AND t.corp_id IN (" + placeholders(len(filter.CorpIDs)) + ")"
	args := make([]any, 0, len(filter.CorpIDs)+2)
	for _, corpID := range filter.CorpIDs {
		args = append(args, corpID)
	}
	if filter.GroupID != nil {
		where += " AND t.contact_tag_group_id = ?"
		args = append(args, *filter.GroupID)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_work_contact_tag t `+where, args...).Scan(&total); err != nil {
		return dashboard.WorkContactTagPage{}, err
	}
	if total == 0 {
		return dashboard.WorkContactTagPage{Items: []dashboard.WorkContactTagItem{}, PerPage: filter.PerPage}, nil
	}

	totalPage := (total + filter.PerPage - 1) / filter.PerPage
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.name, COALESCE(p.contact_num, 0)
		FROM mc_work_contact_tag t
		LEFT JOIN (
			SELECT contact_tag_id, COUNT(contact_id) AS contact_num
			FROM mc_work_contact_tag_pivot
			GROUP BY contact_tag_id
		) p ON p.contact_tag_id = t.id
		`+where+`
		ORDER BY t.updated_at DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.WorkContactTagPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.WorkContactTagItem, 0)
	for rows.Next() {
		var item dashboard.WorkContactTagItem
		if err := rows.Scan(&item.ID, &item.Name, &item.ContactNum); err != nil {
			return dashboard.WorkContactTagPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkContactTagPage{}, err
	}
	return dashboard.WorkContactTagPage{Items: items, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) WorkContactTagByID(ctx context.Context, tagID int) (dashboard.WorkContactTagDetail, bool, error) {
	var tag dashboard.WorkContactTagDetail
	var wxContactTagID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, wx_contact_tag_id, name, contact_tag_group_id
		FROM mc_work_contact_tag
		WHERE id = ? AND deleted_at IS NULL
	`, tagID).Scan(&tag.TagID, &wxContactTagID, &tag.TagName, &tag.GroupID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WorkContactTagDetail{}, false, nil
	}
	if err != nil {
		return dashboard.WorkContactTagDetail{}, false, err
	}
	tag.WXContactTagID = nullString(wxContactTagID)
	return tag, true, nil
}

func (s *MySQLStore) WorkContactTags(ctx context.Context, corpIDs []int, groupID *int) ([]dashboard.WorkContactTagOption, error) {
	if len(corpIDs) == 0 {
		return []dashboard.WorkContactTagOption{}, nil
	}
	where := "WHERE deleted_at IS NULL AND corp_id IN (" + placeholders(len(corpIDs)) + ")"
	args := make([]any, 0, len(corpIDs)+1)
	for _, corpID := range corpIDs {
		args = append(args, corpID)
	}
	if groupID != nil {
		where += " AND contact_tag_group_id = ?"
		args = append(args, *groupID)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name
		FROM mc_work_contact_tag
		`+where+`
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]dashboard.WorkContactTagOption, 0)
	for rows.Next() {
		var tag dashboard.WorkContactTagOption
		if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tags, nil
}

func (s *MySQLStore) WorkContactTagList(ctx context.Context, corpIDs []int, name string) ([]dashboard.WorkContactTagListGroup, error) {
	if len(corpIDs) == 0 {
		return []dashboard.WorkContactTagListGroup{}, nil
	}

	args := make([]any, 0, len(corpIDs))
	for _, corpID := range corpIDs {
		args = append(args, corpID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, wx_group_id, group_name
		FROM mc_work_contact_tag_group
		WHERE deleted_at IS NULL AND corp_id IN (`+placeholders(len(corpIDs))+`)
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make([]dashboard.WorkContactTagListGroup, 0)
	groupIDs := make([]int, 0)
	for rows.Next() {
		var group dashboard.WorkContactTagListGroup
		var wxGroupID sql.NullString
		if err := rows.Scan(&group.ID, &wxGroupID, &group.GroupName); err != nil {
			return nil, err
		}
		group.WXGroupID = nullString(wxGroupID)
		group.Tags = []dashboard.WorkContactTagListTag{}
		groups = append(groups, group)
		groupIDs = append(groupIDs, group.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(groupIDs) == 0 {
		return groups, nil
	}

	tagWhere := "WHERE deleted_at IS NULL AND contact_tag_group_id IN (" + placeholders(len(groupIDs)) + ")"
	tagArgs := make([]any, 0, len(groupIDs)+1)
	for _, groupID := range groupIDs {
		tagArgs = append(tagArgs, groupID)
	}
	if name != "" {
		tagWhere += " AND name LIKE ?"
		tagArgs = append(tagArgs, "%"+name+"%")
	}
	tagRows, err := s.db.QueryContext(ctx, `
		SELECT id, wx_contact_tag_id, name, contact_tag_group_id
		FROM mc_work_contact_tag
		`+tagWhere+`
		ORDER BY id ASC
	`, tagArgs...)
	if err != nil {
		return nil, err
	}
	defer tagRows.Close()

	tagsByGroupID := make(map[int][]dashboard.WorkContactTagListTag, len(groupIDs))
	for tagRows.Next() {
		var tag dashboard.WorkContactTagListTag
		var wxContactTagID sql.NullString
		if err := tagRows.Scan(&tag.ID, &wxContactTagID, &tag.Name, &tag.ContactTagGroupID); err != nil {
			return nil, err
		}
		tag.WXContactTagID = nullString(wxContactTagID)
		tagsByGroupID[tag.ContactTagGroupID] = append(tagsByGroupID[tag.ContactTagGroupID], tag)
	}
	if err := tagRows.Err(); err != nil {
		return nil, err
	}

	for i := range groups {
		if tags, ok := tagsByGroupID[groups[i].ID]; ok {
			groups[i].Tags = tags
		}
	}
	return groups, nil
}

func (s *MySQLStore) WorkContactTagGroupNameExists(ctx context.Context, corpID int, groupName string, excludeGroupID int) (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM mc_work_contact_tag_group
		WHERE corp_id = ? AND group_name = ? AND deleted_at IS NULL
	`
	args := []any{corpID, groupName}
	if excludeGroupID > 0 {
		query += ` AND id <> ?`
		args = append(args, excludeGroupID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return false, err
	}
	return total > 0, nil
}

func (s *MySQLStore) CreateWorkContactTagGroup(ctx context.Context, values dashboard.WorkContactTagGroupWrite) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_work_contact_tag_group (corp_id, group_name, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())
	`, values.CorpID, values.GroupName)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *MySQLStore) UpdateWorkContactTagGroup(ctx context.Context, corpID int, groupID int, groupName string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_work_contact_tag_group
		SET group_name = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, groupName, groupID, corpID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) UpdateWorkContactTagGroupWXID(ctx context.Context, corpID int, groupID int, wxGroupID string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_work_contact_tag_group
		SET wx_group_id = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, strings.TrimSpace(wxGroupID), groupID, corpID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) DeleteWorkContactTagGroupCascade(ctx context.Context, corpID int, groupID int) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)

	tagRows, err := tx.QueryContext(ctx, `
		SELECT id
		FROM mc_work_contact_tag
		WHERE corp_id = ? AND contact_tag_group_id = ? AND deleted_at IS NULL
	`, corpID, groupID)
	if err != nil {
		return false, err
	}
	defer tagRows.Close()
	tagIDs, err := scanIntColumn(tagRows)
	if err != nil {
		return false, err
	}

	groupResult, err := tx.ExecContext(ctx, `
		UPDATE mc_work_contact_tag_group
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, groupID, corpID)
	if err != nil {
		return false, err
	}
	groupAffected, err := groupResult.RowsAffected()
	if err != nil {
		return false, err
	}

	if len(tagIDs) > 0 {
		args := make([]any, 0, len(tagIDs)+1)
		args = append(args, corpID)
		for _, tagID := range tagIDs {
			args = append(args, tagID)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_work_contact_tag
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE corp_id = ? AND id IN (`+placeholders(len(tagIDs))+`) AND deleted_at IS NULL
		`, args...); err != nil {
			return false, err
		}
		pivotArgs := make([]any, 0, len(tagIDs))
		for _, tagID := range tagIDs {
			pivotArgs = append(pivotArgs, tagID)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_work_contact_tag_pivot
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE contact_tag_id IN (`+placeholders(len(tagIDs))+`) AND deleted_at IS NULL
		`, pivotArgs...); err != nil {
			return false, err
		}
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return groupAffected > 0, nil
}

func (s *MySQLStore) WorkContactTagNamesExist(ctx context.Context, corpID int, groupID int, names []string, excludeTagIDs []int) (bool, error) {
	if len(names) == 0 {
		return false, nil
	}
	args := make([]any, 0, 2+len(names)+len(excludeTagIDs))
	args = append(args, corpID, groupID)
	for _, name := range names {
		args = append(args, name)
	}
	query := `
		SELECT COUNT(*)
		FROM mc_work_contact_tag
		WHERE corp_id = ? AND contact_tag_group_id = ? AND name IN (` + placeholders(len(names)) + `) AND deleted_at IS NULL
	`
	if len(excludeTagIDs) > 0 {
		query += ` AND id NOT IN (` + placeholders(len(excludeTagIDs)) + `)`
		for _, tagID := range excludeTagIDs {
			args = append(args, tagID)
		}
	}
	var total int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return false, err
	}
	return total > 0, nil
}

func (s *MySQLStore) CreateWorkContactTags(ctx context.Context, values dashboard.WorkContactTagWrite) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	for _, tagName := range values.TagNames {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_work_contact_tag (corp_id, contact_tag_group_id, name, created_at, updated_at)
			VALUES (?, ?, ?, NOW(), NOW())
		`, values.CorpID, values.GroupID, tagName); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *MySQLStore) UpdateWorkContactTagWXIDsByName(ctx context.Context, corpID int, groupID int, tagWXIDs map[string]string) error {
	if len(tagWXIDs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	for name, wxTagID := range tagWXIDs {
		name = strings.TrimSpace(name)
		wxTagID = strings.TrimSpace(wxTagID)
		if name == "" || wxTagID == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_work_contact_tag
			SET wx_contact_tag_id = ?, updated_at = NOW()
			WHERE corp_id = ? AND contact_tag_group_id = ? AND name = ? AND deleted_at IS NULL
		`, wxTagID, corpID, groupID, name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *MySQLStore) UpdateWorkContactTag(ctx context.Context, corpID int, tagID int, groupID int, tagName string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)

	var oldGroupID int
	err = tx.QueryRowContext(ctx, `
		SELECT contact_tag_group_id
		FROM mc_work_contact_tag
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, tagID, corpID).Scan(&oldGroupID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE mc_work_contact_tag
		SET contact_tag_group_id = ?, name = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, groupID, tagName, tagID, corpID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected > 0 && oldGroupID > 0 && oldGroupID != groupID {
		if err := pruneEmptyWorkContactTagGroupsTx(ctx, tx, corpID, []int{oldGroupID}); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *MySQLStore) DeleteWorkContactTags(ctx context.Context, corpID int, tagIDs []int) (bool, error) {
	if len(tagIDs) == 0 {
		return false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)

	groupArgs := make([]any, 0, len(tagIDs)+1)
	groupArgs = append(groupArgs, corpID)
	for _, tagID := range tagIDs {
		groupArgs = append(groupArgs, tagID)
	}
	groupRows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT contact_tag_group_id
		FROM mc_work_contact_tag
		WHERE corp_id = ? AND id IN (`+placeholders(len(tagIDs))+`) AND deleted_at IS NULL
	`, groupArgs...)
	if err != nil {
		return false, err
	}
	groupIDs, err := scanIntColumn(groupRows)
	if err != nil {
		groupRows.Close()
		return false, err
	}
	if err := groupRows.Close(); err != nil {
		return false, err
	}

	args := make([]any, 0, len(tagIDs)+1)
	args = append(args, corpID)
	for _, tagID := range tagIDs {
		args = append(args, tagID)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_work_contact_tag
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE corp_id = ? AND id IN (`+placeholders(len(tagIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	pivotArgs := make([]any, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		pivotArgs = append(pivotArgs, tagID)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_contact_tag_pivot
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE contact_tag_id IN (`+placeholders(len(tagIDs))+`) AND deleted_at IS NULL
	`, pivotArgs...); err != nil {
		return false, err
	}
	if affected > 0 {
		if err := pruneEmptyWorkContactTagGroupsTx(ctx, tx, corpID, groupIDs); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *MySQLStore) MoveWorkContactTags(ctx context.Context, corpID int, tagIDs []int, groupID int) (bool, error) {
	if len(tagIDs) == 0 {
		return false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)

	groupArgs := make([]any, 0, len(tagIDs)+1)
	groupArgs = append(groupArgs, corpID)
	for _, tagID := range tagIDs {
		groupArgs = append(groupArgs, tagID)
	}
	groupRows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT contact_tag_group_id
		FROM mc_work_contact_tag
		WHERE corp_id = ? AND id IN (`+placeholders(len(tagIDs))+`) AND deleted_at IS NULL
	`, groupArgs...)
	if err != nil {
		return false, err
	}
	oldGroupIDs, err := scanIntColumn(groupRows)
	if err != nil {
		groupRows.Close()
		return false, err
	}
	if err := groupRows.Close(); err != nil {
		return false, err
	}

	args := make([]any, 0, len(tagIDs)+2)
	args = append(args, groupID, corpID)
	for _, tagID := range tagIDs {
		args = append(args, tagID)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_work_contact_tag
		SET contact_tag_group_id = ?, updated_at = NOW()
		WHERE corp_id = ? AND id IN (`+placeholders(len(tagIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected > 0 {
		if err := pruneEmptyWorkContactTagGroupsTx(ctx, tx, corpID, oldGroupIDs); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *MySQLStore) SyncWorkContactTags(ctx context.Context, corpID int, groups []dashboard.WorkContactTagSyncGroup) (dashboard.WorkContactTagSyncResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.WorkContactTagSyncResult{}, err
	}
	defer rollbackQuietly(tx)

	type existingGroup struct {
		ID        int
		WXGroupID string
	}
	groupRows, err := tx.QueryContext(ctx, `
		SELECT id, wx_group_id
		FROM mc_work_contact_tag_group
		WHERE corp_id = ? AND deleted_at IS NULL
	`, corpID)
	if err != nil {
		return dashboard.WorkContactTagSyncResult{}, err
	}
	existingGroups := map[string]existingGroup{}
	for groupRows.Next() {
		var group existingGroup
		var wxGroupID sql.NullString
		if err := groupRows.Scan(&group.ID, &wxGroupID); err != nil {
			groupRows.Close()
			return dashboard.WorkContactTagSyncResult{}, err
		}
		group.WXGroupID = nullString(wxGroupID)
		if group.WXGroupID != "" {
			existingGroups[group.WXGroupID] = group
		}
	}
	if err := groupRows.Close(); err != nil {
		return dashboard.WorkContactTagSyncResult{}, err
	}
	if err := groupRows.Err(); err != nil {
		return dashboard.WorkContactTagSyncResult{}, err
	}

	type existingTag struct {
		ID             int
		WXContactTagID string
	}
	tagRows, err := tx.QueryContext(ctx, `
		SELECT id, wx_contact_tag_id
		FROM mc_work_contact_tag
		WHERE corp_id = ? AND deleted_at IS NULL
	`, corpID)
	if err != nil {
		return dashboard.WorkContactTagSyncResult{}, err
	}
	existingTags := map[string]existingTag{}
	for tagRows.Next() {
		var tag existingTag
		var wxContactTagID sql.NullString
		if err := tagRows.Scan(&tag.ID, &wxContactTagID); err != nil {
			tagRows.Close()
			return dashboard.WorkContactTagSyncResult{}, err
		}
		tag.WXContactTagID = nullString(wxContactTagID)
		if tag.WXContactTagID != "" {
			existingTags[tag.WXContactTagID] = tag
		}
	}
	if err := tagRows.Close(); err != nil {
		return dashboard.WorkContactTagSyncResult{}, err
	}
	if err := tagRows.Err(); err != nil {
		return dashboard.WorkContactTagSyncResult{}, err
	}

	var result dashboard.WorkContactTagSyncResult
	seenGroups := map[string]struct{}{}
	seenTags := map[string]struct{}{}
	for _, group := range groups {
		wxGroupID := strings.TrimSpace(group.WXGroupID)
		if wxGroupID == "" {
			continue
		}
		if _, ok := seenGroups[wxGroupID]; ok {
			continue
		}
		seenGroups[wxGroupID] = struct{}{}

		groupID := 0
		if current, ok := existingGroups[wxGroupID]; ok {
			groupID = current.ID
			if _, err := tx.ExecContext(ctx, `
				UPDATE mc_work_contact_tag_group
				SET wx_group_id = ?, group_name = ?, `+"`order`"+` = ?, updated_at = NOW()
				WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
			`, wxGroupID, group.GroupName, group.Order, groupID, corpID); err != nil {
				return dashboard.WorkContactTagSyncResult{}, err
			}
			result.UpdatedGroups++
			delete(existingGroups, wxGroupID)
		} else {
			insert, err := tx.ExecContext(ctx, `
				INSERT INTO mc_work_contact_tag_group (wx_group_id, corp_id, group_name, `+"`order`"+`, created_at, updated_at)
				VALUES (?, ?, ?, ?, NOW(), NOW())
			`, wxGroupID, corpID, group.GroupName, group.Order)
			if err != nil {
				return dashboard.WorkContactTagSyncResult{}, err
			}
			id, err := insert.LastInsertId()
			if err != nil {
				return dashboard.WorkContactTagSyncResult{}, err
			}
			groupID = int(id)
			result.CreatedGroups++
		}

		for _, tag := range group.Tags {
			wxTagID := strings.TrimSpace(tag.WXContactTagID)
			if wxTagID == "" {
				continue
			}
			if _, ok := seenTags[wxTagID]; ok {
				continue
			}
			seenTags[wxTagID] = struct{}{}
			if current, ok := existingTags[wxTagID]; ok {
				if _, err := tx.ExecContext(ctx, `
					UPDATE mc_work_contact_tag
					SET wx_contact_tag_id = ?, name = ?, `+"`order`"+` = ?, contact_tag_group_id = ?, updated_at = NOW()
					WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
				`, wxTagID, tag.Name, tag.Order, groupID, current.ID, corpID); err != nil {
					return dashboard.WorkContactTagSyncResult{}, err
				}
				result.UpdatedTags++
				delete(existingTags, wxTagID)
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO mc_work_contact_tag (wx_contact_tag_id, corp_id, name, `+"`order`"+`, contact_tag_group_id, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, NOW(), NOW())
			`, wxTagID, corpID, tag.Name, tag.Order, groupID); err != nil {
				return dashboard.WorkContactTagSyncResult{}, err
			}
			result.CreatedTags++
		}
	}

	if len(existingGroups) > 0 {
		groupIDs := make([]int, 0, len(existingGroups))
		for _, group := range existingGroups {
			groupIDs = append(groupIDs, group.ID)
		}
		sort.Ints(groupIDs)
		args := make([]any, 0, len(groupIDs)+1)
		args = append(args, corpID)
		for _, groupID := range groupIDs {
			args = append(args, groupID)
		}
		deleteResult, err := tx.ExecContext(ctx, `
			UPDATE mc_work_contact_tag_group
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE corp_id = ? AND id IN (`+placeholders(len(groupIDs))+`) AND deleted_at IS NULL
		`, args...)
		if err != nil {
			return dashboard.WorkContactTagSyncResult{}, err
		}
		affected, err := deleteResult.RowsAffected()
		if err != nil {
			return dashboard.WorkContactTagSyncResult{}, err
		}
		result.DeletedGroups = int(affected)
	}

	if len(existingTags) > 0 {
		tagIDs := make([]int, 0, len(existingTags))
		for _, tag := range existingTags {
			tagIDs = append(tagIDs, tag.ID)
		}
		sort.Ints(tagIDs)
		args := make([]any, 0, len(tagIDs)+1)
		args = append(args, corpID)
		for _, tagID := range tagIDs {
			args = append(args, tagID)
		}
		deleteResult, err := tx.ExecContext(ctx, `
			UPDATE mc_work_contact_tag
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE corp_id = ? AND id IN (`+placeholders(len(tagIDs))+`) AND deleted_at IS NULL
		`, args...)
		if err != nil {
			return dashboard.WorkContactTagSyncResult{}, err
		}
		affected, err := deleteResult.RowsAffected()
		if err != nil {
			return dashboard.WorkContactTagSyncResult{}, err
		}
		result.DeletedTags = int(affected)

		pivotArgs := make([]any, 0, len(tagIDs))
		for _, tagID := range tagIDs {
			pivotArgs = append(pivotArgs, tagID)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_work_contact_tag_pivot
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE contact_tag_id IN (`+placeholders(len(tagIDs))+`) AND deleted_at IS NULL
		`, pivotArgs...); err != nil {
			return dashboard.WorkContactTagSyncResult{}, err
		}
	}

	var updateTimeID int
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_update_time
		WHERE corp_id = ? AND type = 3
		ORDER BY id DESC
		LIMIT 1
	`, corpID).Scan(&updateTimeID)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_work_update_time (corp_id, type, last_update_time, created_at, updated_at)
			VALUES (?, 3, NOW(), NOW(), NOW())
		`, corpID); err != nil {
			return dashboard.WorkContactTagSyncResult{}, err
		}
	} else if err != nil {
		return dashboard.WorkContactTagSyncResult{}, err
	} else if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_update_time
		SET last_update_time = NOW(), updated_at = NOW()
		WHERE id = ?
	`, updateTimeID); err != nil {
		return dashboard.WorkContactTagSyncResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return dashboard.WorkContactTagSyncResult{}, err
	}
	return result, nil
}

func (s *MySQLStore) SyncWorkContactTagGroup(ctx context.Context, corpID int, group dashboard.WorkContactTagSyncGroup) (dashboard.WorkContactTagSyncResult, error) {
	wxGroupID := strings.TrimSpace(group.WXGroupID)
	if corpID <= 0 || wxGroupID == "" {
		return dashboard.WorkContactTagSyncResult{}, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.WorkContactTagSyncResult{}, err
	}
	defer rollbackQuietly(tx)

	var result dashboard.WorkContactTagSyncResult
	groupID, found, err := workContactTagGroupIDByWXTx(ctx, tx, corpID, wxGroupID)
	if err != nil {
		return dashboard.WorkContactTagSyncResult{}, err
	}
	if found {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_work_contact_tag_group
			SET group_name = ?, `+"`order`"+` = ?, updated_at = NOW()
			WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
		`, group.GroupName, group.Order, groupID, corpID); err != nil {
			return dashboard.WorkContactTagSyncResult{}, err
		}
		result.UpdatedGroups++
	} else {
		insert, err := tx.ExecContext(ctx, `
			INSERT INTO mc_work_contact_tag_group (wx_group_id, corp_id, group_name, `+"`order`"+`, created_at, updated_at)
			VALUES (?, ?, ?, ?, NOW(), NOW())
		`, wxGroupID, corpID, group.GroupName, group.Order)
		if err != nil {
			return dashboard.WorkContactTagSyncResult{}, err
		}
		id, err := insert.LastInsertId()
		if err != nil {
			return dashboard.WorkContactTagSyncResult{}, err
		}
		groupID = int(id)
		result.CreatedGroups++
	}

	seenTags := map[string]struct{}{}
	for _, tag := range group.Tags {
		wxTagID := strings.TrimSpace(tag.WXContactTagID)
		if wxTagID == "" {
			continue
		}
		if _, ok := seenTags[wxTagID]; ok {
			continue
		}
		seenTags[wxTagID] = struct{}{}
		tagID, found, err := workContactTagIDByWXTx(ctx, tx, corpID, wxTagID)
		if err != nil {
			return dashboard.WorkContactTagSyncResult{}, err
		}
		if found {
			if _, err := tx.ExecContext(ctx, `
				UPDATE mc_work_contact_tag
				SET name = ?, `+"`order`"+` = ?, contact_tag_group_id = ?, updated_at = NOW()
				WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
			`, tag.Name, tag.Order, groupID, tagID, corpID); err != nil {
				return dashboard.WorkContactTagSyncResult{}, err
			}
			result.UpdatedTags++
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_work_contact_tag (wx_contact_tag_id, corp_id, name, `+"`order`"+`, contact_tag_group_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, NOW(), NOW())
		`, wxTagID, corpID, tag.Name, tag.Order, groupID); err != nil {
			return dashboard.WorkContactTagSyncResult{}, err
		}
		result.CreatedTags++
	}

	if err := upsertWorkUpdateTimeTx(ctx, tx, corpID, 3); err != nil {
		return dashboard.WorkContactTagSyncResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.WorkContactTagSyncResult{}, err
	}
	return result, nil
}

func pruneEmptyWorkContactTagGroupsTx(ctx context.Context, tx *sql.Tx, corpID int, groupIDs []int) error {
	groupIDs = uniquePositiveInts(groupIDs)
	for _, groupID := range groupIDs {
		if groupID <= 0 {
			continue
		}
		var tagCount int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM mc_work_contact_tag
			WHERE corp_id = ? AND contact_tag_group_id = ? AND deleted_at IS NULL
		`, corpID, groupID).Scan(&tagCount); err != nil {
			return err
		}
		if tagCount > 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_work_contact_tag_group
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
		`, groupID, corpID); err != nil {
			return err
		}
	}
	return nil
}

func (s *MySQLStore) DeleteWorkContactTagByWXContactTagID(ctx context.Context, corpID int, wxContactTagID string) (bool, error) {
	wxContactTagID = strings.TrimSpace(wxContactTagID)
	if corpID <= 0 || wxContactTagID == "" {
		return false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)

	tagID, found, err := workContactTagIDByWXTx(ctx, tx, corpID, wxContactTagID)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_contact_tag
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, tagID, corpID); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_contact_tag_pivot
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE contact_tag_id = ? AND deleted_at IS NULL
	`, tagID); err != nil {
		return false, err
	}
	if err := upsertWorkUpdateTimeTx(ctx, tx, corpID, 3); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) DeleteWorkContactTagGroupByWXGroupID(ctx context.Context, corpID int, wxGroupID string) (bool, error) {
	wxGroupID = strings.TrimSpace(wxGroupID)
	if corpID <= 0 || wxGroupID == "" {
		return false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)

	groupID, found, err := workContactTagGroupIDByWXTx(ctx, tx, corpID, wxGroupID)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	tagIDs, err := workContactTagIDsByGroupTx(ctx, tx, corpID, groupID)
	if err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_contact_tag_group
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, groupID, corpID); err != nil {
		return false, err
	}
	if len(tagIDs) > 0 {
		args := make([]any, 0, len(tagIDs)+1)
		args = append(args, corpID)
		for _, tagID := range tagIDs {
			args = append(args, tagID)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_work_contact_tag
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE corp_id = ? AND id IN (`+placeholders(len(tagIDs))+`) AND deleted_at IS NULL
		`, args...); err != nil {
			return false, err
		}
		pivotArgs := make([]any, 0, len(tagIDs))
		for _, tagID := range tagIDs {
			pivotArgs = append(pivotArgs, tagID)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_work_contact_tag_pivot
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE contact_tag_id IN (`+placeholders(len(tagIDs))+`) AND deleted_at IS NULL
		`, pivotArgs...); err != nil {
			return false, err
		}
	}
	if err := upsertWorkUpdateTimeTx(ctx, tx, corpID, 3); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func workContactTagGroupIDByWXTx(ctx context.Context, tx *sql.Tx, corpID int, wxGroupID string) (int, bool, error) {
	var id int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_contact_tag_group
		WHERE corp_id = ? AND wx_group_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, corpID, wxGroupID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func workContactTagIDByWXTx(ctx context.Context, tx *sql.Tx, corpID int, wxContactTagID string) (int, bool, error) {
	var id int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_contact_tag
		WHERE corp_id = ? AND wx_contact_tag_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, corpID, wxContactTagID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func workContactTagIDsByGroupTx(ctx context.Context, tx *sql.Tx, corpID int, groupID int) ([]int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id
		FROM mc_work_contact_tag
		WHERE corp_id = ? AND contact_tag_group_id = ? AND deleted_at IS NULL
	`, corpID, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) FriendsCircleTaskPage(ctx context.Context, filter dashboard.FriendsCircleTaskFilter) (dashboard.FriendsCircleTaskPage, error) {
	where := " WHERE corp_id = ?"
	args := []any{filter.CorpID}
	if filter.TaskName != "" {
		where += " AND task_name LIKE ?"
		args = append(args, "%"+filter.TaskName+"%")
	}
	if filter.Status != "" {
		where += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.RestrictEmployeeIDs {
		ids := uniquePositiveInts(filter.AllowedEmployeeIDs)
		if len(ids) == 0 {
			return dashboard.FriendsCircleTaskPage{}, nil
		}
		parts := make([]string, 0, len(ids))
		for range ids {
			parts = append(parts, "JSON_CONTAINS(target_employees, JSON_ARRAY(?)) OR JSON_CONTAINS(target_employees, JSON_OBJECT('employeeId', ?))")
		}
		where += " AND (" + strings.Join(parts, " OR ") + ")"
		for _, id := range ids {
			args = append(args, id, id)
		}
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_friends_circle_tasks"+where, args...).Scan(&total); err != nil {
		return dashboard.FriendsCircleTaskPage{}, err
	}
	page, perPage := filter.Page, filter.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, task_name, send_way, content, medium_id, target_employees, status, completed_total, target_total, creator_name, DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), COALESCE(DATE_FORMAT(start_at, '%Y-%m-%d %H:%i:%s'), ''), COALESCE(DATE_FORMAT(end_at, '%Y-%m-%d %H:%i:%s'), ''), external_task_id, publish_attempts, failure_reason, COALESCE(DATE_FORMAT(last_callback_at, '%Y-%m-%d %H:%i:%s'), '') FROM mc_friends_circle_tasks`+where+" ORDER BY id DESC LIMIT ? OFFSET ?", append(args, perPage, (page-1)*perPage)...)
	if err != nil {
		return dashboard.FriendsCircleTaskPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.FriendsCircleTask, 0)
	for rows.Next() {
		var item dashboard.FriendsCircleTask
		if err := rows.Scan(&item.ID, &item.TaskName, &item.SendWay, &item.Content, &item.MediumID, &item.TargetEmployees, &item.Status, &item.CompletedTotal, &item.TargetTotal, &item.CreatorName, &item.CreatedAt, &item.StartAt, &item.EndAt, &item.ExternalTaskID, &item.PublishAttempts, &item.FailureReason, &item.LastCallbackAt); err != nil {
			return dashboard.FriendsCircleTaskPage{}, err
		}
		items = append(items, item)
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + perPage - 1) / perPage
	}
	return dashboard.FriendsCircleTaskPage{Items: items, Total: total, Page: page, PerPage: perPage, TotalPage: totalPage}, rows.Err()
}

func (s *MySQLStore) FriendsCircleMaterialPage(ctx context.Context, filter dashboard.FriendsCircleMaterialFilter) (dashboard.FriendsCircleMaterialPage, error) {
	where := " WHERE corp_id = ?"
	args := []any{filter.CorpID}
	if filter.Keyword != "" {
		where += " AND (name LIKE ? OR content LIKE ?)"
		like := "%" + filter.Keyword + "%"
		args = append(args, like, like)
	}
	if filter.Type != "" {
		where += " AND type = ?"
		args = append(args, filter.Type)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_friends_circle_materials"+where, args...).Scan(&total); err != nil {
		return dashboard.FriendsCircleMaterialPage{}, err
	}
	page, perPage := filter.Page, filter.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, type, content, status, creator_name, DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s') FROM mc_friends_circle_materials`+where+" ORDER BY id DESC LIMIT ? OFFSET ?", append(args, perPage, (page-1)*perPage)...)
	if err != nil {
		return dashboard.FriendsCircleMaterialPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.FriendsCircleMaterial, 0)
	for rows.Next() {
		var item dashboard.FriendsCircleMaterial
		if err := rows.Scan(&item.ID, &item.Name, &item.Type, &item.Content, &item.Status, &item.CreatorName, &item.CreatedAt); err != nil {
			return dashboard.FriendsCircleMaterialPage{}, err
		}
		items = append(items, item)
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + perPage - 1) / perPage
	}
	return dashboard.FriendsCircleMaterialPage{Items: items, Total: total, Page: page, PerPage: perPage, TotalPage: totalPage}, rows.Err()
}

func (s *MySQLStore) FriendsCircleTaskResultPage(ctx context.Context, filter dashboard.FriendsCircleTaskResultFilter) (dashboard.FriendsCircleTaskResultPage, error) {
	where := []string{"corp_id = ?", "task_id = ?"}
	args := []any{filter.CorpID, filter.TaskID}
	if filter.Status != "" {
		where = append(where, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.RestrictEmployeeIDs {
		ids := uniquePositiveInts(filter.AllowedEmployeeIDs)
		if len(ids) == 0 {
			return dashboard.FriendsCircleTaskResultPage{}, nil
		}
		where = append(where, "target_employee_id IN ("+placeholders(len(ids))+")")
		args = append(args, intsToAny(ids)...)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_friends_circle_task_results WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return dashboard.FriendsCircleTaskResultPage{}, err
	}
	page, perPage := filter.Page, filter.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, task_id, target_employee_id, status, failure_code, failure_reason,
		       DATE_FORMAT(occurred_at, '%Y-%m-%d %H:%i:%s')
		FROM mc_friends_circle_task_results
		WHERE `+whereSQL+` ORDER BY id ASC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return dashboard.FriendsCircleTaskResultPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.FriendsCircleTaskResult, 0)
	for rows.Next() {
		var item dashboard.FriendsCircleTaskResult
		if err := rows.Scan(&item.ID, &item.TaskID, &item.TargetEmployeeID, &item.Status, &item.FailureCode, &item.FailureReason, &item.OccurredAt); err != nil {
			return dashboard.FriendsCircleTaskResultPage{}, err
		}
		items = append(items, item)
	}
	return dashboard.FriendsCircleTaskResultPage{Items: items, Total: total, Page: page, PerPage: perPage, TotalPage: pageCount(total, perPage)}, rows.Err()
}

func phase34AcquisitionLinkWhere(filter dashboard.Phase34AcquisitionLinkFilter, alias string) ([]string, []any) {
	where := []string{alias + ".corp_id = ?", alias + ".deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if filter.Name != "" {
		where = append(where, alias+".name LIKE ?")
		args = append(args, "%"+filter.Name+"%")
	}
	if filter.Status != "" {
		where = append(where, alias+".status = ?")
		args = append(args, filter.Status)
	}
	return where, args
}

func (s *MySQLStore) Phase34AcquisitionLinkPage(ctx context.Context, filter dashboard.Phase34AcquisitionLinkFilter) (dashboard.Phase34AcquisitionLinkPage, error) {
	page, perPage := filter.Page, filter.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	where, args := phase34AcquisitionLinkWhere(filter, "l")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_phase34_acquisition_links AS l WHERE "+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return dashboard.Phase34AcquisitionLinkPage{}, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT l.id, l.corp_id, l.name, l.target_url, l.authorization_status, l.status,
		       l.visit_total, l.conversion_total, l.creator_name,
		       DATE_FORMAT(l.created_at, '%Y-%m-%d %H:%i:%s'), DATE_FORMAT(l.updated_at, '%Y-%m-%d %H:%i:%s')
		FROM mc_phase34_acquisition_links AS l
		WHERE `+strings.Join(where, " AND ")+` ORDER BY l.id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return dashboard.Phase34AcquisitionLinkPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.Phase34AcquisitionLink, 0)
	for rows.Next() {
		var item dashboard.Phase34AcquisitionLink
		if err := rows.Scan(&item.ID, &item.CorpID, &item.Name, &item.TargetURL, &item.AuthorizationStatus, &item.Status, &item.VisitTotal, &item.ConversionTotal, &item.CreatorName, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return dashboard.Phase34AcquisitionLinkPage{}, err
		}
		items = append(items, item)
	}
	return dashboard.Phase34AcquisitionLinkPage{Items: items, Total: total, Page: page, PerPage: perPage, TotalPage: pageCount(total, perPage)}, rows.Err()
}

func (s *MySQLStore) CreatePhase34AcquisitionLink(ctx context.Context, values dashboard.Phase34AcquisitionLinkWrite) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_phase34_acquisition_links (corp_id, user_id, creator_name, name, target_url, authorization_status, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, values.CorpID, values.UserID, values.CreatorName, values.Name, values.TargetURL, values.AuthorizationStatus, values.Status)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func (s *MySQLStore) Phase34AcquisitionLinkByID(ctx context.Context, corpID int, id int) (dashboard.Phase34AcquisitionLink, bool, error) {
	var item dashboard.Phase34AcquisitionLink
	err := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, name, target_url, authorization_status, status, visit_total, conversion_total, creator_name,
		       DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), DATE_FORMAT(updated_at, '%Y-%m-%d %H:%i:%s')
		FROM mc_phase34_acquisition_links
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, id, corpID).Scan(&item.ID, &item.CorpID, &item.Name, &item.TargetURL, &item.AuthorizationStatus, &item.Status, &item.VisitTotal, &item.ConversionTotal, &item.CreatorName, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.Phase34AcquisitionLink{}, false, nil
	}
	if err != nil {
		return dashboard.Phase34AcquisitionLink{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) DisablePhase34AcquisitionLink(ctx context.Context, corpID int, id int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE mc_phase34_acquisition_links SET status = 'disabled', disabled_at = NOW(), updated_at = NOW() WHERE id = ? AND corp_id = ? AND deleted_at IS NULL AND status <> 'disabled'`, id, corpID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (s *MySQLStore) UpdatePhase34AcquisitionAuthorizationState(ctx context.Context, corpID int, authorizationStatus string, state string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_phase34_acquisition_links
		SET authorization_status = ?, status = ?, updated_at = NOW()
		WHERE corp_id = ? AND deleted_at IS NULL AND status <> 'disabled'
	`, authorizationStatus, state, corpID)
	return err
}

func phase34CustomerServiceWhere(filter dashboard.Phase34CustomerServiceFilter, alias string) ([]string, []any) {
	where := []string{alias + ".corp_id = ?", alias + ".deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if filter.Name != "" {
		where = append(where, alias+".name LIKE ?")
		args = append(args, "%"+filter.Name+"%")
	}
	if filter.Status != "" {
		where = append(where, alias+".status = ?")
		args = append(args, filter.Status)
	}
	if filter.RestrictEmployeeIDs {
		if len(filter.AllowedEmployeeIDs) == 0 {
			where = append(where, "1=0")
		} else {
			parts := make([]string, 0, len(filter.AllowedEmployeeIDs))
			for _, employeeID := range filter.AllowedEmployeeIDs {
				parts = append(parts, "JSON_CONTAINS("+alias+".employee_ids, JSON_ARRAY(?))")
				args = append(args, employeeID)
			}
			where = append(where, "("+strings.Join(parts, " OR ")+")")
		}
	}
	return where, args
}

func (s *MySQLStore) Phase34CustomerServicePage(ctx context.Context, filter dashboard.Phase34CustomerServiceFilter) (dashboard.Phase34CustomerServicePage, error) {
	page, perPage := filter.Page, filter.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	where, args := phase34CustomerServiceWhere(filter, "c")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_phase34_customer_services AS c WHERE "+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return dashboard.Phase34CustomerServicePage{}, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.corp_id, c.name, c.account, c.employee_ids, c.receive_mode, c.status, c.sync_reason, c.creator_name,
		       DATE_FORMAT(c.created_at, '%Y-%m-%d %H:%i:%s'), DATE_FORMAT(c.updated_at, '%Y-%m-%d %H:%i:%s')
		FROM mc_phase34_customer_services AS c
		WHERE `+strings.Join(where, " AND ")+` ORDER BY c.id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return dashboard.Phase34CustomerServicePage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.Phase34CustomerService, 0)
	for rows.Next() {
		var item dashboard.Phase34CustomerService
		if err := rows.Scan(&item.ID, &item.CorpID, &item.Name, &item.Account, &item.EmployeeIDs, &item.ReceiveMode, &item.Status, &item.SyncReason, &item.CreatorName, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return dashboard.Phase34CustomerServicePage{}, err
		}
		items = append(items, item)
	}
	return dashboard.Phase34CustomerServicePage{Items: items, Total: total, Page: page, PerPage: perPage, TotalPage: pageCount(total, perPage)}, rows.Err()
}

func (s *MySQLStore) CreatePhase34CustomerService(ctx context.Context, values dashboard.Phase34CustomerServiceWrite) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_phase34_customer_services (corp_id, user_id, creator_name, name, account, employee_ids, receive_mode, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, values.CorpID, values.UserID, values.CreatorName, values.Name, values.Account, values.EmployeeIDs, values.ReceiveMode, values.Status)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func (s *MySQLStore) Phase34CustomerServiceByID(ctx context.Context, corpID int, id int) (dashboard.Phase34CustomerService, bool, error) {
	var item dashboard.Phase34CustomerService
	err := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, name, account, employee_ids, receive_mode, status, sync_reason, creator_name,
		       DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), DATE_FORMAT(updated_at, '%Y-%m-%d %H:%i:%s')
		FROM mc_phase34_customer_services
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, id, corpID).Scan(&item.ID, &item.CorpID, &item.Name, &item.Account, &item.EmployeeIDs, &item.ReceiveMode, &item.Status, &item.SyncReason, &item.CreatorName, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.Phase34CustomerService{}, false, nil
	}
	if err != nil {
		return dashboard.Phase34CustomerService{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) UpdatePhase34CustomerServiceSyncState(ctx context.Context, corpID int, state string, reason string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_phase34_customer_services
		SET status = ?, sync_reason = ?, updated_at = NOW()
		WHERE corp_id = ? AND deleted_at IS NULL
	`, state, reason, corpID)
	return err
}

func phase34ShortLinkWhere(filter dashboard.Phase34ShortLinkFilter, alias string) ([]string, []any) {
	where := []string{alias + ".corp_id = ?", alias + ".deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if filter.Name != "" {
		where = append(where, alias+".name LIKE ?")
		args = append(args, "%"+filter.Name+"%")
	}
	if filter.Status != "" {
		where = append(where, alias+".status = ?")
		args = append(args, filter.Status)
	}
	return where, args
}

func (s *MySQLStore) Phase34ShortLinkPage(ctx context.Context, filter dashboard.Phase34ShortLinkFilter) (dashboard.Phase34ShortLinkPage, error) {
	page, perPage := filter.Page, filter.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	where, args := phase34ShortLinkWhere(filter, "s")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_phase34_short_links AS s WHERE "+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return dashboard.Phase34ShortLinkPage{}, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.corp_id, s.name, s.token, s.target_type, s.target_url, s.status, s.visit_total, s.creator_name,
		       DATE_FORMAT(s.created_at, '%Y-%m-%d %H:%i:%s'), DATE_FORMAT(s.updated_at, '%Y-%m-%d %H:%i:%s'), COALESCE(DATE_FORMAT(s.disabled_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mc_phase34_short_links AS s
		WHERE `+strings.Join(where, " AND ")+` ORDER BY s.id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return dashboard.Phase34ShortLinkPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.Phase34ShortLink, 0)
	for rows.Next() {
		var item dashboard.Phase34ShortLink
		if err := rows.Scan(&item.ID, &item.CorpID, &item.Name, &item.Token, &item.TargetType, &item.TargetURL, &item.Status, &item.VisitTotal, &item.CreatorName, &item.CreatedAt, &item.UpdatedAt, &item.DisabledAt); err != nil {
			return dashboard.Phase34ShortLinkPage{}, err
		}
		items = append(items, item)
	}
	return dashboard.Phase34ShortLinkPage{Items: items, Total: total, Page: page, PerPage: perPage, TotalPage: pageCount(total, perPage)}, rows.Err()
}

func (s *MySQLStore) CreatePhase34ShortLink(ctx context.Context, values dashboard.Phase34ShortLinkWrite) (dashboard.Phase34ShortLink, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_phase34_short_links (corp_id, user_id, creator_name, name, token, target_type, target_url, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, values.CorpID, values.UserID, values.CreatorName, values.Name, values.Token, values.TargetType, values.TargetURL, values.Status)
	if err != nil {
		return dashboard.Phase34ShortLink{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return dashboard.Phase34ShortLink{}, err
	}
	return dashboard.Phase34ShortLink{ID: int(id), CorpID: values.CorpID, Name: values.Name, Token: values.Token, TargetType: values.TargetType, TargetURL: values.TargetURL, Status: values.Status, CreatorName: values.CreatorName}, nil
}

func (s *MySQLStore) Phase34ShortLinkByToken(ctx context.Context, token string) (dashboard.Phase34ShortLink, bool, error) {
	var item dashboard.Phase34ShortLink
	err := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, name, token, target_type, target_url, status, visit_total, creator_name,
		       DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), DATE_FORMAT(updated_at, '%Y-%m-%d %H:%i:%s'), COALESCE(DATE_FORMAT(disabled_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mc_phase34_short_links
		WHERE token = ? AND deleted_at IS NULL
	`, token).Scan(&item.ID, &item.CorpID, &item.Name, &item.Token, &item.TargetType, &item.TargetURL, &item.Status, &item.VisitTotal, &item.CreatorName, &item.CreatedAt, &item.UpdatedAt, &item.DisabledAt)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.Phase34ShortLink{}, false, nil
	}
	if err != nil {
		return dashboard.Phase34ShortLink{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) RecordPhase34ShortLinkVisit(ctx context.Context, linkID int, token string, referer string, userAgent string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var corpID int
	if err := tx.QueryRowContext(ctx, `SELECT corp_id FROM mc_phase34_short_links WHERE id = ? AND token = ? AND deleted_at IS NULL`, linkID, token).Scan(&corpID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mc_phase34_short_links SET visit_total = visit_total + 1, updated_at = NOW() WHERE id = ? AND token = ? AND status = 'active' AND deleted_at IS NULL`, linkID, token); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO mc_phase34_short_link_visits (short_link_id, corp_id, token, referer, user_agent) VALUES (?, ?, ?, ?, ?)`, linkID, corpID, token, referer, userAgent); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) DisablePhase34ShortLink(ctx context.Context, corpID int, id int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE mc_phase34_short_links SET status = 'disabled', disabled_at = NOW(), updated_at = NOW() WHERE id = ? AND corp_id = ? AND deleted_at IS NULL AND status <> 'disabled'`, id, corpID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (s *MySQLStore) CreateFriendsCircleTask(ctx context.Context, value dashboard.FriendsCircleTaskWrite) (int, error) {
	result, err := s.db.ExecContext(ctx, `INSERT INTO mc_friends_circle_tasks (corp_id, user_id, creator_name, task_name, send_way, content, medium_id, target_employees, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.CorpID, value.UserID, value.CreatorName, value.TaskName, value.SendWay, value.Content, value.MediumID, value.TargetEmployees, value.Status)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func (s *MySQLStore) FriendsCircleTaskByID(ctx context.Context, corpID int, taskID int) (dashboard.FriendsCircleTask, bool, error) {
	var item dashboard.FriendsCircleTask
	err := s.db.QueryRowContext(ctx, `SELECT id, task_name, send_way, content, medium_id, target_employees, status, completed_total, target_total, creator_name, DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), COALESCE(DATE_FORMAT(start_at, '%Y-%m-%d %H:%i:%s'), ''), COALESCE(DATE_FORMAT(end_at, '%Y-%m-%d %H:%i:%s'), ''), external_task_id, publish_attempts, failure_reason, COALESCE(DATE_FORMAT(last_callback_at, '%Y-%m-%d %H:%i:%s'), '') FROM mc_friends_circle_tasks WHERE id = ? AND corp_id = ?`, taskID, corpID).Scan(&item.ID, &item.TaskName, &item.SendWay, &item.Content, &item.MediumID, &item.TargetEmployees, &item.Status, &item.CompletedTotal, &item.TargetTotal, &item.CreatorName, &item.CreatedAt, &item.StartAt, &item.EndAt, &item.ExternalTaskID, &item.PublishAttempts, &item.FailureReason, &item.LastCallbackAt)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.FriendsCircleTask{}, false, nil
	}
	return item, err == nil, err
}

func (s *MySQLStore) ClaimFriendsCircleTaskForPublish(ctx context.Context, corpID int, taskID int) (dashboard.FriendsCircleTask, bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE mc_friends_circle_tasks SET status = 'publishing', publish_attempts = publish_attempts + 1, failure_reason = '' WHERE id = ? AND corp_id = ? AND status = 'draft'`, taskID, corpID)
	if err != nil {
		return dashboard.FriendsCircleTask{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return dashboard.FriendsCircleTask{}, false, err
	}
	task, found, err := s.FriendsCircleTaskByID(ctx, corpID, taskID)
	return task, found, err
}

func (s *MySQLStore) CreateFriendsCircleMaterial(ctx context.Context, value dashboard.FriendsCircleMaterialWrite) (int, error) {
	result, err := s.db.ExecContext(ctx, `INSERT INTO mc_friends_circle_materials (corp_id, user_id, creator_name, name, type, content, status) VALUES (?, ?, ?, ?, ?, ?, ?)`, value.CorpID, value.UserID, value.CreatorName, value.Name, value.Type, value.Content, value.Status)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func (s *MySQLStore) MarkFriendsCircleTaskPublished(ctx context.Context, corpID int, taskID int, externalID string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE mc_friends_circle_tasks SET status = 'queued', external_task_id = ?, failure_reason = '' WHERE id = ? AND corp_id = ? AND status = 'publishing' AND ? <> ''`, externalID, taskID, corpID, externalID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (s *MySQLStore) MarkFriendsCircleTaskPublishFailed(ctx context.Context, corpID int, taskID int, reason string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE mc_friends_circle_tasks SET status = 'failed', failure_reason = ?, end_at = NOW() WHERE id = ? AND corp_id = ? AND status = 'publishing'`, reason, taskID, corpID)
	return err
}

func (s *MySQLStore) ApplyFriendsCircleCallback(ctx context.Context, callback dashboard.FriendsCircleCallback) (dashboard.FriendsCircleTask, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.FriendsCircleTask{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	var task dashboard.FriendsCircleTask
	err = tx.QueryRowContext(ctx, `SELECT id, task_name, send_way, content, medium_id, target_employees, status, completed_total, target_total, creator_name, DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), COALESCE(DATE_FORMAT(start_at, '%Y-%m-%d %H:%i:%s'), ''), COALESCE(DATE_FORMAT(end_at, '%Y-%m-%d %H:%i:%s'), ''), external_task_id, publish_attempts, failure_reason, COALESCE(DATE_FORMAT(last_callback_at, '%Y-%m-%d %H:%i:%s'), '') FROM mc_friends_circle_tasks WHERE corp_id = ? AND external_task_id = ? ORDER BY id DESC LIMIT 1 FOR UPDATE`, callback.CorpID, callback.ExternalTaskID).Scan(&task.ID, &task.TaskName, &task.SendWay, &task.Content, &task.MediumID, &task.TargetEmployees, &task.Status, &task.CompletedTotal, &task.TargetTotal, &task.CreatorName, &task.CreatedAt, &task.StartAt, &task.EndAt, &task.ExternalTaskID, &task.PublishAttempts, &task.FailureReason, &task.LastCallbackAt)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.FriendsCircleTask{}, false, nil
	}
	if err != nil {
		return dashboard.FriendsCircleTask{}, false, err
	}
	if !friendsCircleCallbackTransitionAllowed(task.Status, callback.Status) {
		return dashboard.FriendsCircleTask{}, true, dashboard.ErrFriendsCircleInvalidTransition
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mc_friends_circle_tasks SET status = ?, completed_total = ?, target_total = ?, failure_reason = ?, last_callback_at = NOW(), end_at = CASE WHEN ? IN ('succeeded', 'failed', 'cancelled') THEN COALESCE(end_at, NOW()) ELSE end_at END WHERE id = ? AND corp_id = ?`, callback.Status, callback.CompletedTotal, callback.TargetTotal, callback.FailureReason, callback.Status, task.ID, callback.CorpID); err != nil {
		return dashboard.FriendsCircleTask{}, false, err
	}
	for _, result := range callback.Results {
		if _, err := tx.ExecContext(ctx, `INSERT INTO mc_friends_circle_task_results (corp_id, task_id, target_employee_id, status, failure_code, failure_reason, occurred_at) VALUES (?, ?, ?, ?, ?, ?, NOW()) ON DUPLICATE KEY UPDATE status = VALUES(status), failure_code = VALUES(failure_code), failure_reason = VALUES(failure_reason), occurred_at = VALUES(occurred_at)`, callback.CorpID, task.ID, result.TargetEmployeeID, result.Status, result.FailureCode, result.FailureReason); err != nil {
			return dashboard.FriendsCircleTask{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return dashboard.FriendsCircleTask{}, false, err
	}
	task.Status = callback.Status
	task.CompletedTotal = callback.CompletedTotal
	task.TargetTotal = callback.TargetTotal
	task.FailureReason = callback.FailureReason
	return task, true, nil
}

func friendsCircleCallbackTransitionAllowed(current, next string) bool {
	if current == next {
		return true
	}
	allowed := map[string]map[string]bool{
		"queued":              {"running": true, "partially_succeeded": true, "succeeded": true, "failed": true, "cancelled": true},
		"running":             {"partially_succeeded": true, "succeeded": true, "failed": true, "cancelled": true},
		"partially_succeeded": {"succeeded": true, "failed": true, "cancelled": true},
	}
	return allowed[current][next]
}

func (s *MySQLStore) MediumGroupsByCorpID(ctx context.Context, corpID int) ([]dashboard.MediumGroup, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name
		FROM mc_medium_group
		WHERE corp_id = ? AND deleted_at IS NULL
		ORDER BY `+"`order`"+` ASC, id ASC
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make([]dashboard.MediumGroup, 0)
	for rows.Next() {
		var group dashboard.MediumGroup
		if err := rows.Scan(&group.ID, &group.Name); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (s *MySQLStore) MediumGroupNameExists(ctx context.Context, corpID int, name string, excludeGroupID int) (bool, error) {
	args := []any{corpID, name}
	query := `
		SELECT COUNT(*)
		FROM mc_medium_group
		WHERE corp_id = ? AND name = ? AND deleted_at IS NULL
	`
	if excludeGroupID > 0 {
		query += ` AND id <> ?`
		args = append(args, excludeGroupID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return false, err
	}
	return total > 0, nil
}

func (s *MySQLStore) CreateMediumGroup(ctx context.Context, values dashboard.MediumGroupWrite) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_medium_group (corp_id, name, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())
	`, values.CorpID, values.Name)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func (s *MySQLStore) UpdateMediumGroup(ctx context.Context, groupID int, values dashboard.MediumGroupWrite) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_medium_group
		SET name = ?, corp_id = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, values.Name, values.CorpID, groupID, values.CorpID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) DeleteMediumGroupReassignMedia(ctx context.Context, corpID int, groupID int) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)

	result, err := tx.ExecContext(ctx, `
		UPDATE mc_medium_group
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, groupID, corpID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_medium
		SET medium_group_id = 0, updated_at = NOW()
		WHERE corp_id = ? AND medium_group_id = ? AND deleted_at IS NULL
	`, corpID, groupID); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *MySQLStore) ChannelCodeGroupsByCorpIDs(ctx context.Context, corpIDs []int) ([]dashboard.ChannelCodeGroup, error) {
	corpIDs = uniquePositiveInts(corpIDs)
	if len(corpIDs) == 0 {
		return []dashboard.ChannelCodeGroup{}, nil
	}
	args := make([]any, 0, len(corpIDs))
	for _, corpID := range corpIDs {
		args = append(args, corpID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name
		FROM mc_channel_code_group
		WHERE corp_id IN (`+placeholders(len(corpIDs))+`) AND deleted_at IS NULL
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := make([]dashboard.ChannelCodeGroup, 0)
	for rows.Next() {
		var group dashboard.ChannelCodeGroup
		if err := rows.Scan(&group.ID, &group.Name); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (s *MySQLStore) ChannelCodeGroupByID(ctx context.Context, groupID int) (dashboard.ChannelCodeGroup, bool, error) {
	var group dashboard.ChannelCodeGroup
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name
		FROM mc_channel_code_group
		WHERE id = ? AND deleted_at IS NULL
	`, groupID).Scan(&group.ID, &group.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.ChannelCodeGroup{}, false, nil
	}
	if err != nil {
		return dashboard.ChannelCodeGroup{}, false, err
	}
	return group, true, nil
}

func (s *MySQLStore) ChannelCodeGroupNamesExist(ctx context.Context, names []string) (bool, error) {
	cleaned := make([]string, 0, len(names))
	seen := map[string]struct{}{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		cleaned = append(cleaned, name)
	}
	if len(cleaned) == 0 {
		return false, nil
	}
	args := make([]any, 0, len(cleaned))
	for _, name := range cleaned {
		args = append(args, name)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_channel_code_group
		WHERE name IN (`+placeholders(len(cleaned))+`) AND deleted_at IS NULL
	`, args...).Scan(&total); err != nil {
		return false, err
	}
	return total > 0, nil
}

func (s *MySQLStore) CreateChannelCodeGroups(ctx context.Context, corpID int, names []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_channel_code_group (corp_id, name, created_at, updated_at)
			VALUES (?, ?, NOW(), NOW())
		`, corpID, name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *MySQLStore) UpdateChannelCodeGroupName(ctx context.Context, groupID int, name string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_channel_code_group
		SET name = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, name, groupID)
	return err
}

func (s *MySQLStore) MoveChannelCodeToGroup(ctx context.Context, channelCodeID int, groupID int) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_channel_code
		SET group_id = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, groupID, channelCodeID)
	return err
}

func (s *MySQLStore) ChannelCodesForCron(ctx context.Context) ([]dashboard.ChannelCodeCronItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, corp_id, auto_add_friend, COALESCE(drainage_employee, '{}'), COALESCE(wx_config_id, ''),
		       COALESCE(DATE_FORMAT(valid_until, '%Y-%m-%d %H:%i:%s'), ''), COALESCE(lifecycle_state, 'active')
		FROM mc_channel_code
		WHERE deleted_at IS NULL
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]dashboard.ChannelCodeCronItem, 0)
	for rows.Next() {
		var item dashboard.ChannelCodeCronItem
		var rawDrainage []byte
		if err := rows.Scan(&item.ID, &item.CorpID, &item.AutoAddFriend, &rawDrainage, &item.WXConfigID, &item.ValidUntil, &item.LifecycleState); err != nil {
			return nil, err
		}
		if len(rawDrainage) > 0 {
			if err := json.Unmarshal(rawDrainage, &item.DrainageEmployee); err != nil {
				return nil, err
			}
		}
		if item.DrainageEmployee == nil {
			item.DrainageEmployee = map[string]any{}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) CreateChannelCode(ctx context.Context, values dashboard.ChannelCodeWriteValues) (int, error) {
	tags, err := json.Marshal(values.Tags)
	if err != nil {
		return 0, err
	}
	drainage, err := json.Marshal(values.DrainageEmployee)
	if err != nil {
		return 0, err
	}
	welcome, err := json.Marshal(values.WelcomeMessage)
	if err != nil {
		return 0, err
	}
	params, err := json.Marshal(map[string]any{
		"baseInfo": map[string]any{
			"groupId":       values.GroupID,
			"name":          values.Name,
			"autoAddFriend": values.AutoAddFriend,
			"tags":          values.Tags,
		},
		"drainageEmployee": values.DrainageEmployee,
		"welcomeMessage":   values.WelcomeMessage,
	})
	if err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_channel_code
			(corp_id, group_id, name, auto_add_friend, tags, type, drainage_employee, welcome_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, values.CorpID, values.GroupID, values.Name, values.AutoAddFriend, tags, values.Type, drainage, welcome)
	if err != nil {
		return 0, err
	}
	channelCodeID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mc_business_log (business_id, params, event, operation_id, created_at, updated_at)
		VALUES (?, ?, 100, ?, NOW(), NOW())
	`, channelCodeID, params, values.OperationEmployee); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(channelCodeID), nil
}

func (s *MySQLStore) UpdateChannelCode(ctx context.Context, channelCodeID int, values dashboard.ChannelCodeWriteValues) (string, error) {
	tags, err := json.Marshal(values.Tags)
	if err != nil {
		return "", err
	}
	drainage, err := json.Marshal(values.DrainageEmployee)
	if err != nil {
		return "", err
	}
	welcome, err := json.Marshal(values.WelcomeMessage)
	if err != nil {
		return "", err
	}
	params, err := json.Marshal(map[string]any{
		"channelCodeId": channelCodeID,
		"baseInfo": map[string]any{
			"groupId":       values.GroupID,
			"name":          values.Name,
			"autoAddFriend": values.AutoAddFriend,
			"tags":          values.Tags,
		},
		"drainageEmployee": values.DrainageEmployee,
		"welcomeMessage":   values.WelcomeMessage,
	})
	if err != nil {
		return "", err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var wxConfigID string
	err = tx.QueryRowContext(ctx, `
		SELECT wx_config_id
		FROM mc_channel_code
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, channelCodeID, values.CorpID).Scan(&wxConfigID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_channel_code
		SET group_id = ?, name = ?, auto_add_friend = ?, tags = ?, type = ?, drainage_employee = ?, welcome_message = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, values.GroupID, values.Name, values.AutoAddFriend, tags, values.Type, drainage, welcome, channelCodeID, values.CorpID); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mc_business_log (business_id, params, event, operation_id, created_at, updated_at)
		VALUES (?, ?, 101, ?, NOW(), NOW())
	`, channelCodeID, params, values.OperationEmployee); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return wxConfigID, nil
}

func (s *MySQLStore) UpdateChannelCodeQRCode(ctx context.Context, channelCodeID int, qrCodeURL string, wxConfigID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_channel_code
		SET qrcode_url = ?, wx_config_id = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, qrCodeURL, wxConfigID, channelCodeID)
	return err
}

func (s *MySQLStore) DeleteChannelCode(ctx context.Context, channelCodeID int) error {
	tenantID := 0
	var welcomeRaw sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(c.tenant_id, 0), COALESCE(code.welcome_message, '')
		FROM mc_channel_code AS code
		LEFT JOIN mc_corp AS c ON c.id = code.corp_id AND c.deleted_at IS NULL
		WHERE code.id = ? AND code.deleted_at IS NULL
		LIMIT 1
	`, channelCodeID).Scan(&tenantID, &welcomeRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_channel_code
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, channelCodeID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 || tenantID <= 0 {
		return nil
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, channelCodeWelcomeStoragePathsFromContent(nullString(welcomeRaw))); err != nil {
		return err
	}
	return s.RefreshSaaSUsageCounter(ctx, tenantID, dashboard.SaaSMetricChannelCodes)
}

func (s *MySQLStore) ChannelCodeEmployeeWXUserIDs(ctx context.Context, employeeIDs []int) ([]string, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return []string{}, nil
	}
	args := make([]any, 0, len(employeeIDs))
	for _, id := range employeeIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT wx_user_id
		FROM mc_work_employee
		WHERE id IN (`+placeholders(len(employeeIDs))+`) AND deleted_at IS NULL
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var wxUserID string
		if err := rows.Scan(&wxUserID); err != nil {
			return nil, err
		}
		if strings.TrimSpace(wxUserID) != "" {
			ids = append(ids, wxUserID)
		}
	}
	return ids, rows.Err()
}

func (s *MySQLStore) ChannelCodeContactCountsByEmployee(ctx context.Context, employeeIDs []int) (map[int]int, error) {
	result := map[int]int{}
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(employeeIDs))
	for _, id := range employeeIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT employee_id, COUNT(*)
		FROM mc_work_contact_employee
		WHERE employee_id IN (`+placeholders(len(employeeIDs))+`) AND status = 1 AND deleted_at IS NULL
		GROUP BY employee_id
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var employeeID int
		var total int
		if err := rows.Scan(&employeeID, &total); err != nil {
			return nil, err
		}
		result[employeeID] = total
	}
	return result, rows.Err()
}

func (s *MySQLStore) ChannelCodePage(ctx context.Context, filter dashboard.ChannelCodeListFilter) (dashboard.ChannelCodeListPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 20
	}
	page := dashboard.ChannelCodeListPage{Items: []dashboard.ChannelCodeListItem{}, PerPage: filter.PerPage}
	filter.CorpIDs = uniquePositiveInts(filter.CorpIDs)
	if len(filter.CorpIDs) == 0 {
		return page, nil
	}
	if filter.RestrictBusinessIDs {
		filter.BusinessIDs = uniquePositiveInts(filter.BusinessIDs)
		if len(filter.BusinessIDs) == 0 {
			return page, nil
		}
	}

	where, args := channelCodeWhere(filter)
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_channel_code AS cc WHERE "+strings.Join(where, " AND "), args...).Scan(&page.Total); err != nil {
		return dashboard.ChannelCodeListPage{}, err
	}
	if page.Total == 0 {
		return page, nil
	}
	page.TotalPage = (page.Total + page.PerPage - 1) / page.PerPage
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, page.PerPage, (filter.Page-1)*page.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT cc.id, cc.group_id, cc.name, cc.qrcode_url, cc.auto_add_friend, cc.tags, cc.type, COALESCE(cg.name, '')
		FROM mc_channel_code AS cc
		LEFT JOIN mc_channel_code_group AS cg ON cg.id = cc.group_id AND cg.deleted_at IS NULL
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY cc.updated_at DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.ChannelCodeListPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.ChannelCodeListItem, 0)
	channelIDs := make([]int, 0)
	tagIDs := make([]int, 0)
	itemTagIDs := map[int][]int{}
	for rows.Next() {
		var item dashboard.ChannelCodeListItem
		var rawTags []byte
		if err := rows.Scan(&item.ID, &item.GroupID, &item.Name, &item.QRCodeURL, &item.AutoAddFriend, &rawTags, &item.Type, &item.GroupName); err != nil {
			return dashboard.ChannelCodeListPage{}, err
		}
		ids := intSliceFromJSON(rawTags)
		itemTagIDs[item.ID] = ids
		tagIDs = append(tagIDs, ids...)
		channelIDs = append(channelIDs, item.ID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ChannelCodeListPage{}, err
	}

	tagNames, err := s.channelCodeTagNames(ctx, tagIDs)
	if err != nil {
		return dashboard.ChannelCodeListPage{}, err
	}
	contactNums, err := s.channelCodeContactNums(ctx, channelIDs)
	if err != nil {
		return dashboard.ChannelCodeListPage{}, err
	}
	for i := range items {
		for _, tagID := range itemTagIDs[items[i].ID] {
			if name, ok := tagNames[tagID]; ok {
				items[i].Tags = append(items[i].Tags, name)
			}
		}
		if items[i].Tags == nil {
			items[i].Tags = []string{}
		}
		items[i].ContactNum = contactNums[items[i].ID]
	}
	page.Items = items
	return page, nil
}

func (s *MySQLStore) ChannelCodeBusinessIDsByOperators(ctx context.Context, operationIDs []int) ([]int, error) {
	operationIDs = uniquePositiveInts(operationIDs)
	if len(operationIDs) == 0 {
		return []int{}, nil
	}
	args := make([]any, 0, len(operationIDs)+1)
	for _, id := range operationIDs {
		args = append(args, id)
	}
	args = append(args, 100)
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT business_id
		FROM mc_business_log
		WHERE operation_id IN (`+placeholders(len(operationIDs))+`) AND event = ?
		ORDER BY business_id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) ChannelCodeShowByID(ctx context.Context, channelCodeID int, corpID int) (dashboard.ChannelCodeShow, bool, error) {
	var groupID int
	var name string
	var autoAddFriend int
	var rawTags []byte
	var rawDrainage []byte
	var rawWelcome []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT group_id, name, auto_add_friend, tags, drainage_employee, welcome_message
		FROM mc_channel_code
		WHERE id = ? AND deleted_at IS NULL
	`, channelCodeID).Scan(&groupID, &name, &autoAddFriend, &rawTags, &rawDrainage, &rawWelcome)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.ChannelCodeShow{}, false, nil
	}
	if err != nil {
		return dashboard.ChannelCodeShow{}, false, err
	}

	groupName := "未分组"
	if groupID != 0 {
		if group, found, err := s.ChannelCodeGroupByID(ctx, groupID); err != nil {
			return dashboard.ChannelCodeShow{}, false, err
		} else if found {
			groupName = group.Name
		}
	}
	selectedTags := intSliceFromJSON(rawTags)
	tagGroups, selected, err := s.channelCodeTagGroups(ctx, corpID, selectedTags)
	if err != nil {
		return dashboard.ChannelCodeShow{}, false, err
	}
	if selected == nil {
		selected = []int{}
	}
	drainage, err := s.channelCodeDecoratedDrainage(ctx, rawDrainage)
	if err != nil {
		return dashboard.ChannelCodeShow{}, false, err
	}
	welcome, err := s.channelCodeDecoratedWelcome(ctx, rawWelcome)
	if err != nil {
		return dashboard.ChannelCodeShow{}, false, err
	}
	return dashboard.ChannelCodeShow{
		GroupID:          groupID,
		GroupName:        groupName,
		Name:             name,
		AutoAddFriend:    autoAddFriend,
		TagGroups:        tagGroups,
		SelectedTags:     selected,
		DrainageEmployee: drainage,
		WelcomeMessage:   welcome,
	}, true, nil
}

func (s *MySQLStore) ChannelCodeWelcomeByID(ctx context.Context, channelCodeID int) (dashboard.ChannelCodeWelcome, bool, error) {
	var welcome dashboard.ChannelCodeWelcome
	var rawWelcome []byte
	var rawTags []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id, welcome_message, tags
		FROM mc_channel_code
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, channelCodeID).Scan(&welcome.ID, &rawWelcome, &rawTags)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.ChannelCodeWelcome{}, false, nil
	}
	if err != nil {
		return dashboard.ChannelCodeWelcome{}, false, err
	}
	welcome.WelcomeMessage = map[string]any{}
	if len(rawWelcome) > 0 {
		if err := json.Unmarshal(rawWelcome, &welcome.WelcomeMessage); err != nil {
			return dashboard.ChannelCodeWelcome{}, false, err
		}
	}
	welcome.TagIDs = parseJSONIntSlice(rawTags)
	return welcome, true, nil
}

func (s *MySQLStore) ChannelCodeContactPage(ctx context.Context, filter dashboard.ChannelCodeContactFilter) (dashboard.ChannelCodeContactPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 15
	}
	page := dashboard.ChannelCodeContactPage{Items: []dashboard.ChannelCodeContactItem{}, PerPage: filter.PerPage}
	state := "channelCode-" + strconv.Itoa(filter.ChannelCodeID)
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_work_contact_employee
		WHERE state = ? AND deleted_at IS NULL
	`, state).Scan(&page.Total); err != nil {
		return dashboard.ChannelCodeContactPage{}, err
	}
	if page.Total == 0 {
		return page, nil
	}
	page.TotalPage = (page.Total + page.PerPage - 1) / page.PerPage
	rows, err := s.db.QueryContext(ctx, `
		SELECT contact_id, employee_id, create_time
		FROM mc_work_contact_employee
		WHERE state = ? AND deleted_at IS NULL
		ORDER BY create_time DESC
		LIMIT ? OFFSET ?
	`, state, page.PerPage, (filter.Page-1)*page.PerPage)
	if err != nil {
		return dashboard.ChannelCodeContactPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.ChannelCodeContactItem, 0)
	contactIDs := make([]int, 0)
	employeeIDs := make([]int, 0)
	for rows.Next() {
		var item dashboard.ChannelCodeContactItem
		var createTime sql.NullTime
		if err := rows.Scan(&item.ContactID, &item.EmployeeID, &createTime); err != nil {
			return dashboard.ChannelCodeContactPage{}, err
		}
		item.CreateTime = formatTime(createTime)
		contactIDs = append(contactIDs, item.ContactID)
		employeeIDs = append(employeeIDs, item.EmployeeID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ChannelCodeContactPage{}, err
	}
	contactNames, err := s.channelCodeContactNames(ctx, contactIDs)
	if err != nil {
		return dashboard.ChannelCodeContactPage{}, err
	}
	employeeNames, err := s.channelCodeEmployeeLabels(ctx, employeeIDs)
	if err != nil {
		return dashboard.ChannelCodeContactPage{}, err
	}
	for i := range items {
		items[i].Name = contactNames[items[i].ContactID]
		items[i].Employees = employeeNames[items[i].EmployeeID]
	}
	page.Items = items
	return page, nil
}

func (s *MySQLStore) ChannelCodeStatContacts(ctx context.Context, channelCodeID int) ([]dashboard.ChannelCodeStatContact, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT status, create_time, deleted_at
		FROM mc_work_contact_employee
		WHERE state = ?
	`, "channelCode-"+strconv.Itoa(channelCodeID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	contacts := make([]dashboard.ChannelCodeStatContact, 0)
	for rows.Next() {
		var contact dashboard.ChannelCodeStatContact
		var createTime sql.NullTime
		var deletedAt sql.NullTime
		if err := rows.Scan(&contact.Status, &createTime, &deletedAt); err != nil {
			return nil, err
		}
		contact.CreateAt = formatTime(createTime)
		contact.DeletedAt = formatTime(deletedAt)
		contacts = append(contacts, contact)
	}
	return contacts, rows.Err()
}

func channelCodeWhere(filter dashboard.ChannelCodeListFilter) ([]string, []any) {
	where := []string{"cc.deleted_at IS NULL", "cc.corp_id IN (" + placeholders(len(filter.CorpIDs)) + ")"}
	args := make([]any, 0, len(filter.CorpIDs)+4)
	for _, corpID := range filter.CorpIDs {
		args = append(args, corpID)
	}
	if filter.Name != "" {
		where = append(where, "cc.name LIKE ?")
		args = append(args, "%"+filter.Name+"%")
	}
	if filter.Type != nil && *filter.Type != 0 {
		where = append(where, "cc.type = ?")
		args = append(args, *filter.Type)
	}
	if filter.GroupID != nil {
		where = append(where, "cc.group_id = ?")
		args = append(args, *filter.GroupID)
	}
	if filter.RestrictBusinessIDs {
		where = append(where, "cc.id IN ("+placeholders(len(filter.BusinessIDs))+")")
		for _, id := range filter.BusinessIDs {
			args = append(args, id)
		}
	}
	return where, args
}

func (s *MySQLStore) channelCodeTagNames(ctx context.Context, tagIDs []int) (map[int]string, error) {
	result := map[int]string{}
	tagIDs = uniquePositiveInts(tagIDs)
	if len(tagIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(tagIDs))
	for _, id := range tagIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name
		FROM mc_work_contact_tag
		WHERE id IN (`+placeholders(len(tagIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		result[id] = name
	}
	return result, rows.Err()
}

func (s *MySQLStore) channelCodeContactNums(ctx context.Context, channelIDs []int) (map[int]int, error) {
	result := map[int]int{}
	channelIDs = uniquePositiveInts(channelIDs)
	if len(channelIDs) == 0 {
		return result, nil
	}
	states := make([]string, 0, len(channelIDs))
	args := make([]any, 0, len(channelIDs))
	stateToID := map[string]int{}
	for _, id := range channelIDs {
		state := "channelCode-" + strconv.Itoa(id)
		states = append(states, state)
		args = append(args, state)
		stateToID[state] = id
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT state, COUNT(contact_id)
		FROM mc_work_contact_employee
		WHERE state IN (`+placeholders(len(states))+`) AND deleted_at IS NULL
		GROUP BY state
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var state string
		var total int
		if err := rows.Scan(&state, &total); err != nil {
			return nil, err
		}
		if id, ok := stateToID[state]; ok {
			result[id] = total
		}
	}
	return result, rows.Err()
}

func (s *MySQLStore) channelCodeTagGroups(ctx context.Context, corpID int, selectedTagIDs []int) ([]dashboard.ChannelCodeTagGroup, []int, error) {
	selectedSet := map[int]struct{}{}
	for _, id := range uniquePositiveInts(selectedTagIDs) {
		selectedSet[id] = struct{}{}
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, contact_tag_group_id
		FROM mc_work_contact_tag
		WHERE corp_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`, corpID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	type tagRow struct {
		id      int
		name    string
		groupID int
	}
	tags := make([]tagRow, 0)
	groupIDs := make([]int, 0)
	for rows.Next() {
		var tag tagRow
		if err := rows.Scan(&tag.id, &tag.name, &tag.groupID); err != nil {
			return nil, nil, err
		}
		tags = append(tags, tag)
		if tag.groupID != 0 {
			groupIDs = append(groupIDs, tag.groupID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(tags) == 0 {
		return []dashboard.ChannelCodeTagGroup{}, []int{}, nil
	}
	groupNames, err := s.channelCodeTagGroupNames(ctx, groupIDs)
	if err != nil {
		return nil, nil, err
	}
	groups := []dashboard.ChannelCodeTagGroup{{GroupID: 0, GroupName: "未分组", List: []dashboard.ChannelCodeTagItem{}}}
	groupIndex := map[int]int{0: 0}
	orderedGroupIDs := uniquePositiveInts(groupIDs)
	sort.Ints(orderedGroupIDs)
	for _, id := range orderedGroupIDs {
		name, ok := groupNames[id]
		if !ok {
			continue
		}
		groupIndex[id] = len(groups)
		groups = append(groups, dashboard.ChannelCodeTagGroup{GroupID: id, GroupName: name, List: []dashboard.ChannelCodeTagItem{}})
	}
	selected := make([]int, 0)
	for _, tag := range tags {
		index, ok := groupIndex[tag.groupID]
		if !ok {
			continue
		}
		isSelected := 2
		if _, ok := selectedSet[tag.id]; ok {
			isSelected = 1
			selected = append(selected, tag.id)
		}
		groups[index].List = append(groups[index].List, dashboard.ChannelCodeTagItem{
			TagID:      tag.id,
			TagName:    tag.name,
			IsSelected: isSelected,
		})
	}
	return groups, selected, nil
}

func (s *MySQLStore) channelCodeTagGroupNames(ctx context.Context, groupIDs []int) (map[int]string, error) {
	result := map[int]string{}
	groupIDs = uniquePositiveInts(groupIDs)
	if len(groupIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(groupIDs))
	for _, id := range groupIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, group_name
		FROM mc_work_contact_tag_group
		WHERE id IN (`+placeholders(len(groupIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		result[id] = name
	}
	return result, rows.Err()
}

func (s *MySQLStore) channelCodeContactNames(ctx context.Context, contactIDs []int) (map[int]string, error) {
	result := map[int]string{}
	contactIDs = uniquePositiveInts(contactIDs)
	if len(contactIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(contactIDs))
	for _, id := range contactIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name
		FROM mc_work_contact
		WHERE id IN (`+placeholders(len(contactIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		result[id] = name
	}
	return result, rows.Err()
}

func (s *MySQLStore) channelCodeEmployeeLabels(ctx context.Context, employeeIDs []int) (map[int]string, error) {
	result := map[int]string{}
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(employeeIDs))
	for _, id := range employeeIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.name, COALESCE(c.name, '')
		FROM mc_work_employee AS e
		LEFT JOIN mc_corp AS c ON c.id = e.corp_id AND c.deleted_at IS NULL
		WHERE e.id IN (`+placeholders(len(employeeIDs))+`) AND e.deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name string
		var corpName string
		if err := rows.Scan(&id, &name, &corpName); err != nil {
			return nil, err
		}
		if corpName != "" {
			result[id] = corpName + " " + name
		} else {
			result[id] = name
		}
	}
	return result, rows.Err()
}

func (s *MySQLStore) channelCodeDecoratedDrainage(ctx context.Context, raw []byte) (any, error) {
	value := decodeJSONAny(raw)
	root, ok := value.(map[string]any)
	if !ok {
		return value, nil
	}
	if employees, ok := root["employees"].([]any); ok {
		for _, item := range employees {
			employee, ok := item.(map[string]any)
			if !ok {
				continue
			}
			employee["weekText"] = channelCodeWeekText(intFromAny(employee["week"]))
			if slots, ok := employee["timeSlot"].([]any); ok {
				for _, rawSlot := range slots {
					slot, ok := rawSlot.(map[string]any)
					if !ok {
						continue
					}
					names, err := s.channelCodeEmployeeNames(ctx, intsFromAny(slot["employeeId"]))
					if err != nil {
						return nil, err
					}
					slot["selectMembers"] = names
					departmentIDs := intsFromAny(slot["departmentId"])
					if len(departmentIDs) > 0 {
						departments, err := s.channelCodeDepartmentNames(ctx, departmentIDs)
						if err != nil {
							return nil, err
						}
						slot["departmentName"] = departments
					}
				}
			}
		}
	}
	if special, ok := root["specialPeriod"].(map[string]any); ok {
		if details, ok := special["detail"].([]any); ok {
			for _, rawDetail := range details {
				detail, ok := rawDetail.(map[string]any)
				if !ok {
					continue
				}
				if slots, ok := detail["timeSlot"].([]any); ok {
					for _, rawSlot := range slots {
						slot, ok := rawSlot.(map[string]any)
						if !ok {
							continue
						}
						names, err := s.channelCodeEmployeeNames(ctx, intsFromAny(slot["employeeId"]))
						if err != nil {
							return nil, err
						}
						slot["selectMembers"] = names
					}
				}
			}
		}
	}
	if addMax, ok := root["addMax"].(map[string]any); ok {
		names, err := s.channelCodeEmployeeNames(ctx, intsFromAny(addMax["spareEmployeeIds"]))
		if err != nil {
			return nil, err
		}
		addMax["spareEmployeeName"] = names
		if employees, ok := addMax["employees"].([]any); ok {
			for _, rawEmployee := range employees {
				employee, ok := rawEmployee.(map[string]any)
				if !ok {
					continue
				}
				names, err := s.channelCodeEmployeeNames(ctx, []int{intFromAny(employee["employeeId"])})
				if err != nil {
					return nil, err
				}
				if len(names) > 0 {
					employee["employeeName"] = names[0]
				} else {
					employee["employeeName"] = ""
				}
			}
		}
	}
	return root, nil
}

func (s *MySQLStore) channelCodeDecoratedWelcome(ctx context.Context, raw []byte) (any, error) {
	value := decodeJSONAny(raw)
	root, ok := value.(map[string]any)
	if !ok {
		return value, nil
	}
	details, ok := root["messageDetail"].([]any)
	if !ok {
		return root, nil
	}
	for _, rawDetail := range details {
		detail, ok := rawDetail.(map[string]any)
		if !ok {
			continue
		}
		if intFromAny(detail["type"]) == 1 {
			content, err := s.channelCodeMediumContent(ctx, intFromAny(detail["mediumId"]))
			if err != nil {
				return nil, err
			}
			detail["content"] = content
			continue
		}
		if nestedDetails, ok := detail["detail"].([]any); ok {
			for _, rawNested := range nestedDetails {
				nested, ok := rawNested.(map[string]any)
				if !ok {
					continue
				}
				if slots, ok := nested["timeSlot"].([]any); ok {
					for _, rawSlot := range slots {
						slot, ok := rawSlot.(map[string]any)
						if !ok {
							continue
						}
						content, err := s.channelCodeMediumContent(ctx, intFromAny(slot["mediumId"]))
						if err != nil {
							return nil, err
						}
						slot["content"] = content
					}
				}
			}
		}
	}
	return root, nil
}

func (s *MySQLStore) channelCodeMediumContent(ctx context.Context, mediumID int) (any, error) {
	if mediumID <= 0 {
		return []any{}, nil
	}
	var raw []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT content
		FROM mc_medium
		WHERE id = ? AND deleted_at IS NULL
	`, mediumID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return []any{}, nil
	}
	if err != nil {
		return nil, err
	}
	return decodeJSONAny(raw), nil
}

func (s *MySQLStore) channelCodeEmployeeNames(ctx context.Context, employeeIDs []int) ([]string, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return []string{}, nil
	}
	args := make([]any, 0, len(employeeIDs))
	for _, id := range employeeIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT name
		FROM mc_work_employee
		WHERE id IN (`+placeholders(len(employeeIDs))+`) AND deleted_at IS NULL
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (s *MySQLStore) channelCodeDepartmentNames(ctx context.Context, departmentIDs []int) ([]string, error) {
	departmentIDs = uniquePositiveInts(departmentIDs)
	if len(departmentIDs) == 0 {
		return []string{}, nil
	}
	args := make([]any, 0, len(departmentIDs))
	for _, id := range departmentIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT name
		FROM mc_work_department
		WHERE id IN (`+placeholders(len(departmentIDs))+`) AND deleted_at IS NULL
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func intSliceFromJSON(raw []byte) []int {
	if len(raw) == 0 {
		return []int{}
	}
	var ints []int
	if err := json.Unmarshal(raw, &ints); err == nil {
		return uniquePositiveInts(ints)
	}
	var values []any
	if err := json.Unmarshal(raw, &values); err != nil {
		return []int{}
	}
	result := make([]int, 0, len(values))
	for _, value := range values {
		result = append(result, intFromAny(value))
	}
	return uniquePositiveInts(result)
}

func decodeJSONAny(raw []byte) any {
	if len(raw) == 0 {
		return []any{}
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return []any{}
	}
	return value
}

func intsFromAny(value any) []int {
	switch typed := value.(type) {
	case nil:
		return []int{}
	case []int:
		return uniquePositiveInts(typed)
	case []any:
		values := make([]int, 0, len(typed))
		for _, item := range typed {
			values = append(values, intFromAny(item))
		}
		return uniquePositiveInts(values)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return []int{}
		}
		if strings.Contains(trimmed, ",") {
			parts := strings.Split(trimmed, ",")
			values := make([]int, 0, len(parts))
			for _, part := range parts {
				values = append(values, intFromAny(part))
			}
			return uniquePositiveInts(values)
		}
		return uniquePositiveInts([]int{intFromAny(trimmed)})
	default:
		return uniquePositiveInts([]int{intFromAny(value)})
	}
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		parsed, _ := strconv.Atoi(strings.TrimSpace(strings.Trim(fmt.Sprint(value), `"`)))
		return parsed
	}
}

func channelCodeWeekText(value int) string {
	switch value {
	case 0:
		return "周日"
	case 1:
		return "周一"
	case 2:
		return "周二"
	case 3:
		return "周三"
	case 4:
		return "周四"
	case 5:
		return "周五"
	case 6:
		return "周六"
	default:
		return ""
	}
}

func (s *MySQLStore) MediumPage(ctx context.Context, filter dashboard.MediumFilter) (dashboard.MediumPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10
	}
	where, args := mediumWhere(filter, "m")
	countQuery := `SELECT COUNT(*) FROM mc_medium AS m WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return dashboard.MediumPage{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}

	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.type, m.media_id, m.content, m.corp_id, m.medium_group_id,
		       COALESCE(g.name, ''), m.user_id, m.user_name, m.scope_type, m.scope_id,
		       m.sidebar_visible, m.status, m.created_at
		FROM mc_medium AS m
		LEFT JOIN mc_medium_group AS g ON g.id = m.medium_group_id AND g.deleted_at IS NULL
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY m.created_at DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.MediumPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.MediumItem, 0)
	for rows.Next() {
		item, err := scanMediumItem(rows)
		if err != nil {
			return dashboard.MediumPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.MediumPage{}, err
	}
	return dashboard.MediumPage{Items: items, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) MediumByID(ctx context.Context, corpID int, mediumID int) (dashboard.MediumItem, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT m.id, m.type, m.media_id, m.content, m.corp_id, m.medium_group_id,
		       COALESCE(g.name, ''), m.user_id, m.user_name, m.scope_type, m.scope_id,
		       m.sidebar_visible, m.status, m.created_at
		FROM mc_medium AS m
		LEFT JOIN mc_medium_group AS g ON g.id = m.medium_group_id AND g.deleted_at IS NULL
		WHERE m.id = ? AND m.corp_id = ? AND m.deleted_at IS NULL
		LIMIT 1
	`, mediumID, corpID)
	item, err := scanMediumRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.MediumItem{}, false, nil
	}
	if err != nil {
		return dashboard.MediumItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) MediumAvailableToUser(ctx context.Context, corpID int, userID int, mediumID int) (bool, error) {
	if corpID <= 0 || userID <= 0 || mediumID <= 0 {
		return false, nil
	}
	employeeID, err := s.EmployeeIDByUserCorp(ctx, userID, corpID)
	if err != nil {
		return false, err
	}
	var foundID int
	err = s.db.QueryRowContext(ctx, `
		SELECT m.id
		FROM mc_medium AS m
		WHERE m.id = ?
		  AND m.corp_id = ?
		  AND m.is_sync = 1
		  AND m.status = 'available'
		  AND m.deleted_at IS NULL
		  AND (
			m.scope_type = 'public'
			OR (m.scope_type = 'personal' AND m.scope_id = ?)
			OR (m.scope_type = 'department' AND EXISTS (
				SELECT 1
				FROM mc_work_employee_department AS ed
				WHERE ed.employee_id = ?
				  AND ed.department_id = m.scope_id
				  AND ed.deleted_at IS NULL
			))
		  )
		LIMIT 1
	`, mediumID, corpID, userID, employeeID).Scan(&foundID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return foundID == mediumID, nil
}

func (s *MySQLStore) CreateMedium(ctx context.Context, values dashboard.MediumWrite) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_medium (type, is_sync, content, corp_id, medium_group_id, user_id, user_name, scope_type, scope_id, sidebar_visible, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, values.Type, values.IsSync, values.Content, values.CorpID, values.MediumGroupID, values.UserID, values.UserName, values.ScopeType, values.ScopeID, values.SidebarVisible, values.Status)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func (s *MySQLStore) UpdateMedium(ctx context.Context, mediumID int, values dashboard.MediumWrite) (bool, error) {
	var oldContent sql.NullString
	tenantID := 0
	err := s.db.QueryRowContext(ctx, `
		SELECT m.content, COALESCE(c.tenant_id, 0)
		FROM mc_medium AS m
		LEFT JOIN mc_corp AS c ON c.id = m.corp_id AND c.deleted_at IS NULL
		WHERE m.id = ? AND m.corp_id = ? AND m.deleted_at IS NULL
		LIMIT 1
	`, mediumID, values.CorpID).Scan(&oldContent, &tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	reclaimPaths := storagePathsRemoved(
		mediumStoragePathsFromContent(nullString(oldContent)),
		mediumStoragePathsFromContent(values.Content),
	)
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_medium
		SET type = ?, is_sync = ?, content = ?, medium_group_id = ?, user_id = ?, user_name = ?, scope_type = ?, scope_id = ?, sidebar_visible = ?, status = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, values.Type, values.IsSync, values.Content, values.MediumGroupID, values.UserID, values.UserName, values.ScopeType, values.ScopeID, values.SidebarVisible, values.Status, mediumID, values.CorpID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return affected > 0, err
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return true, err
	}
	return true, nil
}

func (s *MySQLStore) DeleteMedium(ctx context.Context, corpID int, mediumID int) (bool, error) {
	var rawContent sql.NullString
	tenantID := 0
	err := s.db.QueryRowContext(ctx, `
		SELECT m.content, COALESCE(c.tenant_id, 0)
		FROM mc_medium AS m
		LEFT JOIN mc_corp AS c ON c.id = m.corp_id AND c.deleted_at IS NULL
		WHERE m.id = ? AND m.corp_id = ? AND m.deleted_at IS NULL
		LIMIT 1
	`, mediumID, corpID).Scan(&rawContent, &tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	storagePaths := mediumStoragePathsFromContent(nullString(rawContent))
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_medium
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, mediumID, corpID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return affected > 0, err
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, storagePaths); err != nil {
		return true, err
	}
	return true, nil
}

func (s *MySQLStore) UpdateMediumGroupID(ctx context.Context, corpID int, mediumID int, groupID int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_medium
		SET medium_group_id = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, groupID, mediumID, corpID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) MaterialReferences(ctx context.Context, corpID int, ids []int) ([]dashboard.MaterialReference, error) {
	ids = uniquePositiveInts(ids)
	if corpID <= 0 || len(ids) == 0 {
		return []dashboard.MaterialReference{}, nil
	}
	args := []any{corpID}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(words, '')
		FROM mc_greeting
		WHERE corp_id = ? AND medium_id IN (`+placeholders(len(ids))+`) AND deleted_at IS NULL
		ORDER BY id
	`, args...)
	if err != nil {
		return nil, err
	}
	references := make([]dashboard.MaterialReference, 0)
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		if strings.TrimSpace(name) == "" {
			name = "欢迎语"
		}
		references = append(references, dashboard.MaterialReference{SourceType: "greeting", SourceID: id, SourceName: name})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	taskRows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(task_name, '')
		FROM mc_friends_circle_tasks
		WHERE corp_id = ? AND medium_id IN (`+placeholders(len(ids))+`)
		ORDER BY id
	`, args...)
	if err != nil {
		return nil, err
	}
	for taskRows.Next() {
		var id int
		var name string
		if err := taskRows.Scan(&id, &name); err != nil {
			return nil, err
		}
		if strings.TrimSpace(name) == "" {
			name = "朋友圈任务"
		}
		references = append(references, dashboard.MaterialReference{SourceType: "friends_circle_task", SourceID: id, SourceName: name})
	}
	if err := taskRows.Err(); err != nil {
		_ = taskRows.Close()
		return nil, err
	}
	if err := taskRows.Close(); err != nil {
		return nil, err
	}
	for _, source := range []struct {
		table        string
		nameColumn   string
		sourceType   string
		fallbackName string
	}{
		{table: "mc_work_room_auto_pull", nameColumn: "qrcode_name", sourceType: "work_room_auto_pull", fallbackName: "自动拉群模板"},
		{table: "mc_contact_message_batch_send", nameColumn: "user_name", sourceType: "contact_message_batch_send", fallbackName: "客户群发任务"},
		{table: "mc_room_message_batch_send", nameColumn: "batch_title", sourceType: "room_message_batch_send", fallbackName: "群聊群发任务"},
	} {
		sourceRows, err := s.db.QueryContext(ctx, `
			SELECT id, COALESCE(`+source.nameColumn+`, '')
			FROM `+source.table+`
			WHERE corp_id = ? AND medium_id IN (`+placeholders(len(ids))+`) AND deleted_at IS NULL
			ORDER BY id
		`, args...)
		if err != nil {
			return nil, err
		}
		for sourceRows.Next() {
			var id int
			var name string
			if err := sourceRows.Scan(&id, &name); err != nil {
				_ = sourceRows.Close()
				return nil, err
			}
			if strings.TrimSpace(name) == "" {
				name = source.fallbackName
			}
			references = append(references, dashboard.MaterialReference{SourceType: source.sourceType, SourceID: id, SourceName: name})
		}
		if err := sourceRows.Err(); err != nil {
			_ = sourceRows.Close()
			return nil, err
		}
		if err := sourceRows.Close(); err != nil {
			return nil, err
		}
	}
	return references, nil
}

func (s *MySQLStore) BatchUpdateMediumGroupID(ctx context.Context, corpID int, ids []int, groupID int) (bool, error) {
	ids = uniquePositiveInts(ids)
	if corpID <= 0 || len(ids) == 0 || groupID < 0 {
		return false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if groupID > 0 {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_medium_group WHERE id = ? AND corp_id = ? AND deleted_at IS NULL`, groupID, corpID).Scan(&count); err != nil || count != 1 {
			if err != nil {
				return false, err
			}
			return false, nil
		}
	}
	args := []any{groupID, corpID}
	for _, id := range ids {
		args = append(args, id)
	}
	countArgs := []any{corpID}
	for _, id := range ids {
		countArgs = append(countArgs, id)
	}
	var materialCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_medium WHERE corp_id = ? AND id IN (`+placeholders(len(ids))+`) AND deleted_at IS NULL`, countArgs...).Scan(&materialCount); err != nil {
		return false, err
	}
	if materialCount != len(ids) {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE mc_medium SET medium_group_id = ?, updated_at = NOW() WHERE corp_id = ? AND id IN (`+placeholders(len(ids))+`) AND deleted_at IS NULL`, args...)
	if err != nil {
		return false, err
	}
	if _, err := result.RowsAffected(); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) BatchDeleteMedium(ctx context.Context, corpID int, ids []int) (bool, error) {
	ids = uniquePositiveInts(ids)
	if corpID <= 0 || len(ids) == 0 {
		return false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	checkArgs := []any{corpID}
	for _, id := range ids {
		checkArgs = append(checkArgs, id)
	}
	var references int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_greeting WHERE corp_id = ? AND medium_id IN (`+placeholders(len(ids))+`) AND deleted_at IS NULL`, checkArgs...).Scan(&references); err != nil {
		return false, err
	}
	var taskReferences int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_friends_circle_tasks WHERE corp_id = ? AND medium_id IN (`+placeholders(len(ids))+`)`, checkArgs...).Scan(&taskReferences); err != nil {
		return false, err
	}
	references += taskReferences
	for _, table := range []string{"mc_work_room_auto_pull", "mc_contact_message_batch_send", "mc_room_message_batch_send"} {
		var linked int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE corp_id = ? AND medium_id IN (`+placeholders(len(ids))+`) AND deleted_at IS NULL`, checkArgs...).Scan(&linked); err != nil {
			return false, err
		}
		references += linked
	}
	if references > 0 {
		return false, nil
	}
	var materialCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_medium WHERE corp_id = ? AND id IN (`+placeholders(len(ids))+`) AND deleted_at IS NULL`, checkArgs...).Scan(&materialCount); err != nil {
		return false, err
	}
	if materialCount != len(ids) {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE mc_medium SET deleted_at = NOW(), updated_at = NOW() WHERE corp_id = ? AND id IN (`+placeholders(len(ids))+`) AND deleted_at IS NULL`, checkArgs...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || int(affected) != len(ids) {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) MediumMediaForUpdateByID(ctx context.Context, mediumID int) (dashboard.MediumMediaUpdateItem, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, media_id, last_upload_time, type, content
		FROM mc_medium
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, mediumID)
	item, err := scanMediumMediaUpdateItem(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dashboard.MediumMediaUpdateItem{}, false, nil
		}
		return dashboard.MediumMediaUpdateItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) SidebarMediumMediaForUpdateByID(ctx context.Context, corpID int, mediumID int) (dashboard.MediumMediaUpdateItem, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, media_id, last_upload_time, type, content
		FROM mc_medium
		WHERE id = ? AND corp_id = ? AND sidebar_visible = 1 AND status = 'available' AND deleted_at IS NULL
		LIMIT 1
	`, mediumID, corpID)
	item, err := scanMediumMediaUpdateItem(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dashboard.MediumMediaUpdateItem{}, false, nil
		}
		return dashboard.MediumMediaUpdateItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) MediumMediaForUpdateByCorp(ctx context.Context, corpID int, olderThan int64) ([]dashboard.MediumMediaUpdateItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, media_id, last_upload_time, type, content
		FROM mc_medium
		WHERE corp_id = ?
		  AND deleted_at IS NULL
		  AND last_upload_time < ?
		  AND type IN (2, 4, 5, 7)
		ORDER BY id ASC
	`, corpID, olderThan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]dashboard.MediumMediaUpdateItem, 0)
	for rows.Next() {
		item, err := scanMediumMediaUpdateItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) UpdateMediumMediaID(ctx context.Context, mediumID int, mediaID string, lastUploadTime int64) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_medium
		SET media_id = ?, last_upload_time = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, mediaID, lastUploadTime, mediumID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) UpdateSidebarMediumMediaID(ctx context.Context, corpID int, mediumID int, mediaID string, lastUploadTime int64) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_medium
		SET media_id = ?, last_upload_time = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND sidebar_visible = 1 AND status = 'available' AND deleted_at IS NULL
	`, mediaID, lastUploadTime, mediumID, corpID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) MediumCorpCredentialByID(ctx context.Context, corpID int) (dashboard.MediumCorpCredential, bool, error) {
	item, found, err := s.loadCorpCredentialByID(ctx, s.db, corpID, false)
	if err != nil || !found {
		return dashboard.MediumCorpCredential{}, found, err
	}
	secret, err := s.decodeCorpCredential(item)
	if err != nil {
		return dashboard.MediumCorpCredential{}, false, err
	}
	return dashboard.MediumCorpCredential{CorpID: item.ID, WXCorpID: item.WXCorpID, EmployeeSecret: secret.EmployeeSecret}, true, nil
}

func scanMediumMediaUpdateItem(scanner mediumScanner) (dashboard.MediumMediaUpdateItem, error) {
	var item dashboard.MediumMediaUpdateItem
	var mediaID sql.NullString
	var lastUploadTime sql.NullInt64
	var rawContent []byte
	if err := scanner.Scan(&item.ID, &mediaID, &lastUploadTime, &item.Type, &rawContent); err != nil {
		return dashboard.MediumMediaUpdateItem{}, err
	}
	item.MediaID = nullString(mediaID)
	if lastUploadTime.Valid {
		item.LastUploadTime = lastUploadTime.Int64
	}
	item.Content = map[string]any{}
	if len(rawContent) > 0 {
		_ = json.Unmarshal(rawContent, &item.Content)
	}
	return item, nil
}

func mediumWhere(filter dashboard.MediumFilter, alias string) ([]string, []any) {
	prefix := alias + "."
	where := []string{prefix + "corp_id = ?", prefix + "is_sync = 1", prefix + "deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if filter.Type > 0 {
		where = append(where, prefix+"type = ?")
		args = append(args, filter.Type)
	}
	if filter.MediumGroupID != nil {
		where = append(where, prefix+"medium_group_id = ?")
		args = append(args, *filter.MediumGroupID)
	}
	if filter.Search != "" {
		where = append(where, prefix+"content LIKE ?")
		args = append(args, "%"+filter.Search+"%")
	}
	if filter.SelectorVisible {
		where = append(where, "("+
			prefix+"scope_type = 'public' OR "+
			"("+prefix+"scope_type = 'personal' AND "+prefix+"scope_id = ?) OR "+
			"("+prefix+"scope_type = 'department' AND EXISTS ("+
			"SELECT 1 FROM mc_work_employee_department AS ed "+
			"WHERE ed.employee_id = ? AND ed.department_id = "+prefix+"scope_id AND ed.deleted_at IS NULL"+
			")))")
		args = append(args, filter.UserID, filter.EmployeeID)
	} else {
		if filter.ScopeType != "" {
			where = append(where, prefix+"scope_type = ?")
			args = append(args, filter.ScopeType)
		}
		if filter.ScopeID > 0 {
			where = append(where, prefix+"scope_id = ?")
			args = append(args, filter.ScopeID)
		}
	}
	if filter.Status != "" {
		where = append(where, prefix+"status = ?")
		args = append(args, filter.Status)
	}
	if filter.SidebarVisible != nil {
		where = append(where, prefix+"sidebar_visible = ?")
		args = append(args, *filter.SidebarVisible)
	}
	return where, args
}

type mediumScanner interface {
	Scan(dest ...any) error
}

func scanMediumRow(row mediumScanner) (dashboard.MediumItem, error) {
	return scanMedium(row)
}

func scanMediumItem(rows mediumScanner) (dashboard.MediumItem, error) {
	return scanMedium(rows)
}

func scanMedium(scanner mediumScanner) (dashboard.MediumItem, error) {
	var item dashboard.MediumItem
	var rawContent []byte
	var groupName sql.NullString
	var createdAt sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&item.Type,
		&item.MediaID,
		&rawContent,
		&item.CorpID,
		&item.MediumGroupID,
		&groupName,
		&item.UserID,
		&item.UserName,
		&item.ScopeType,
		&item.ScopeID,
		&item.SidebarVisible,
		&item.Status,
		&createdAt,
	); err != nil {
		return dashboard.MediumItem{}, err
	}
	content := map[string]any{}
	if len(rawContent) > 0 {
		_ = json.Unmarshal(rawContent, &content)
	}
	item.Content = content
	item.MediumGroupName = nullString(groupName)
	if item.MediumGroupID == 0 || item.MediumGroupName == "" {
		item.MediumGroupName = "未分组"
	}
	item.CreatedAt = formatTime(createdAt)
	return item, nil
}

func (s *MySQLStore) GreetingPage(ctx context.Context, filter dashboard.GreetingFilter) (dashboard.GreetingPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10
	}
	where, args := greetingWhere(filter)
	if filter.RestrictOperationIDs && len(filter.OperationIDs) == 0 {
		return dashboard.GreetingPage{Items: []dashboard.GreetingItem{}, PerPage: filter.PerPage}, nil
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_greeting WHERE "+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return dashboard.GreetingPage{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, corp_id, type, words, medium_id, range_type, employees, created_at
		FROM mc_greeting
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY updated_at DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.GreetingPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.GreetingItem, 0)
	for rows.Next() {
		item, err := scanGreeting(rows)
		if err != nil {
			return dashboard.GreetingPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.GreetingPage{}, err
	}
	return dashboard.GreetingPage{Items: items, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) GreetingsByCorp(ctx context.Context, corpID int) ([]dashboard.GreetingItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, corp_id, type, words, medium_id, range_type, employees, created_at
		FROM mc_greeting
		WHERE corp_id = ? AND deleted_at IS NULL
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.GreetingItem, 0)
	for rows.Next() {
		item, err := scanGreeting(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) GreetingByID(ctx context.Context, greetingID int) (dashboard.GreetingItem, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, type, words, medium_id, range_type, employees, created_at
		FROM mc_greeting
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, greetingID)
	item, err := scanGreeting(row)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.GreetingItem{}, false, nil
	}
	if err != nil {
		return dashboard.GreetingItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) GreetingEmployeesByIDs(ctx context.Context, employeeIDs []int) (map[int]dashboard.GreetingEmployee, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	result := make(map[int]dashboard.GreetingEmployee, len(employeeIDs))
	if len(employeeIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(employeeIDs))
	for _, id := range employeeIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, avatar, wx_user_id
		FROM mc_work_employee
		WHERE id IN (`+placeholders(len(employeeIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var employee dashboard.GreetingEmployee
		var name, avatar, wxUserID sql.NullString
		if err := rows.Scan(&employee.ID, &name, &avatar, &wxUserID); err != nil {
			return nil, err
		}
		employee.Name = nullString(name)
		employee.Avatar = nullString(avatar)
		employee.WXUserID = nullString(wxUserID)
		result[employee.ID] = employee
	}
	return result, rows.Err()
}

func (s *MySQLStore) GreetingMediaByIDs(ctx context.Context, mediumIDs []int) (map[int]dashboard.GreetingMedium, error) {
	mediumIDs = uniquePositiveInts(mediumIDs)
	result := make(map[int]dashboard.GreetingMedium, len(mediumIDs))
	if len(mediumIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(mediumIDs))
	for _, id := range mediumIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, type, content
		FROM mc_medium
		WHERE id IN (`+placeholders(len(mediumIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var medium dashboard.GreetingMedium
		var rawContent []byte
		if err := rows.Scan(&medium.ID, &medium.Type, &rawContent); err != nil {
			return nil, err
		}
		medium.Content = map[string]any{}
		if len(rawContent) > 0 {
			_ = json.Unmarshal(rawContent, &medium.Content)
		}
		result[medium.ID] = medium
	}
	return result, rows.Err()
}

func (s *MySQLStore) CreateGreetingWithLog(ctx context.Context, values dashboard.GreetingWrite, operationID int) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer rollbackQuietly(tx)
	employees := greetingEmployeesJSONStore(values.EmployeeIDs)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_greeting (corp_id, type, words, medium_id, range_type, employees, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, values.CorpID, values.Type, values.Words, values.MediumID, values.RangeType, employees)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := insertBusinessLog(ctx, tx, int(id), greetingBusinessLogPayload(values, employees), 300, operationID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *MySQLStore) UpdateGreetingWithLog(ctx context.Context, greetingID int, values dashboard.GreetingWrite, operationID int) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)
	employees := greetingEmployeesJSONStore(values.EmployeeIDs)
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_greeting
		SET corp_id = ?, type = ?, words = ?, medium_id = ?, range_type = ?, employees = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, values.CorpID, values.Type, values.Words, values.MediumID, values.RangeType, employees, greetingID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}
	if err := insertBusinessLog(ctx, tx, greetingID, greetingBusinessLogPayload(values, employees), 301, operationID); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) DeleteGreeting(ctx context.Context, greetingID int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_greeting
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, greetingID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) RoomWelcomePage(ctx context.Context, filter dashboard.RoomWelcomeFilter) (dashboard.RoomWelcomePage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10
	}
	where, args := roomWelcomeWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_room_welcome_template WHERE "+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return dashboard.RoomWelcomePage{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, corp_id, msg_text, complex_type, msg_complex, complex_template_id, create_user_id, created_at, updated_at
		FROM mc_room_welcome_template
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomWelcomePage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomWelcomeItem, 0)
	for rows.Next() {
		item, err := scanRoomWelcome(rows)
		if err != nil {
			return dashboard.RoomWelcomePage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomWelcomePage{}, err
	}
	return dashboard.RoomWelcomePage{Items: items, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) StatisticToday(ctx context.Context, corpID int, now time.Time) (dashboard.StatisticContactSummary, error) {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.Add(24 * time.Hour)
	total, err := s.countScalar(ctx, `
		SELECT COUNT(*)
		FROM mc_work_contact_employee
		WHERE corp_id = ? AND status = 1 AND deleted_at IS NULL
	`, corpID)
	if err != nil {
		return dashboard.StatisticContactSummary{}, err
	}
	add, err := s.countScalar(ctx, `
		SELECT COUNT(*)
		FROM mc_work_contact_employee
		WHERE corp_id = ? AND create_time > ? AND create_time < ? AND deleted_at IS NULL
	`, corpID, formatDBTime(start), formatDBTime(end))
	if err != nil {
		return dashboard.StatisticContactSummary{}, err
	}
	loss, err := s.countScalar(ctx, `
		SELECT COUNT(*)
		FROM mc_work_contact_employee
		WHERE corp_id = ? AND deleted_at > ? AND deleted_at < ? AND status IN (2, 3)
	`, corpID, formatDBTime(start), formatDBTime(end))
	if err != nil {
		return dashboard.StatisticContactSummary{}, err
	}
	return dashboard.StatisticContactSummary{Total: total, Add: add, Loss: loss, Net: add - loss}, nil
}

func (s *MySQLStore) StatisticContactTrend(ctx context.Context, corpID int, employeeIDs []int, start time.Time, end time.Time) ([]dashboard.StatisticContactDay, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	days := make([]dashboard.StatisticContactDay, 0)
	for day := start; !day.After(end); day = day.Add(24 * time.Hour) {
		next := day.Add(24 * time.Hour)
		var total, add, loss int
		var err error
		if len(employeeIDs) > 0 {
			total, err = s.statisticEmployeeContactTotalAt(ctx, employeeIDs, next)
			if err != nil {
				return nil, err
			}
			add, err = s.statisticEmployeeContactAddBetween(ctx, employeeIDs, day, next)
			if err != nil {
				return nil, err
			}
			loss, err = s.statisticEmployeeContactLossBetween(ctx, employeeIDs, day, next)
			if err != nil {
				return nil, err
			}
		} else {
			total, err = s.countScalar(ctx, `
				SELECT COUNT(*)
				FROM mc_work_contact_employee
				WHERE corp_id = ? AND create_time < ? AND status = 1 AND deleted_at IS NULL
			`, corpID, formatDBTime(next))
			if err != nil {
				return nil, err
			}
			add, err = s.countScalar(ctx, `
				SELECT COUNT(*)
				FROM mc_work_contact_employee
				WHERE corp_id = ? AND create_time > ? AND create_time < ? AND deleted_at IS NULL
			`, corpID, formatDBTime(day), formatDBTime(next))
			if err != nil {
				return nil, err
			}
			loss, err = s.countScalar(ctx, `
				SELECT COUNT(*)
				FROM mc_work_contact_employee
				WHERE corp_id = ? AND deleted_at > ? AND deleted_at < ? AND status IN (2, 3)
			`, corpID, formatDBTime(day), formatDBTime(next))
			if err != nil {
				return nil, err
			}
		}
		days = append(days, dashboard.StatisticContactDay{
			Date:  day.Format("2006/01/02"),
			Total: total,
			Add:   add,
			Loss:  loss,
			Net:   add - loss,
		})
	}
	return days, nil
}

func (s *MySQLStore) StatisticTopEmployees(ctx context.Context, corpID int, limit int) (int, []dashboard.StatisticTopEmployee, error) {
	if limit <= 0 {
		limit = 10
	}
	total, err := s.countScalar(ctx, `
		SELECT COUNT(*)
		FROM mc_work_contact_employee
		WHERE corp_id = ? AND status = 1 AND deleted_at IS NULL
	`, corpID)
	if err != nil {
		return 0, nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.name, COALESCE(COUNT(ce.contact_id), 0) AS total
		FROM mc_work_employee AS e
		LEFT JOIN mc_work_contact_employee AS ce ON ce.employee_id = e.id AND ce.deleted_at IS NULL
		WHERE e.corp_id = ? AND e.deleted_at IS NULL
		GROUP BY e.id, e.name
		ORDER BY total DESC, e.id ASC
		LIMIT ?
	`, corpID, limit)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	list := make([]dashboard.StatisticTopEmployee, 0)
	for rows.Next() {
		var item dashboard.StatisticTopEmployee
		var name sql.NullString
		if err := rows.Scan(&name, &item.Total); err != nil {
			return 0, nil, err
		}
		item.Name = nullString(name)
		list = append(list, item)
	}
	return total, list, rows.Err()
}

func (s *MySQLStore) StatisticEmployeesByCorp(ctx context.Context, corpID int) ([]dashboard.StatisticEmployee, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, wx_user_id, name, avatar
		FROM mc_work_employee
		WHERE corp_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStatisticEmployees(rows)
}

func (s *MySQLStore) StatisticEmployeesByIDs(ctx context.Context, corpID int, employeeIDs []int) ([]dashboard.StatisticEmployee, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return []dashboard.StatisticEmployee{}, nil
	}
	args := make([]any, 0, len(employeeIDs)+1)
	args = append(args, corpID)
	for _, employeeID := range employeeIDs {
		args = append(args, employeeID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, wx_user_id, name, avatar
		FROM mc_work_employee
		WHERE corp_id = ? AND id IN (`+placeholders(len(employeeIDs))+`) AND deleted_at IS NULL
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStatisticEmployees(rows)
}

func (s *MySQLStore) StatisticEmployeeLocalSummary(ctx context.Context, corpID int, employeeIDs []int, start time.Time, end time.Time) (dashboard.StatisticEmployeeMetric, error) {
	where, args := statisticEmployeeWhere(corpID, employeeIDs, start, end)
	row := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(chat_cnt), 0),
			COALESCE(SUM(message_cnt), 0),
			COALESCE(AVG(reply_percentage), 0),
			COALESCE(AVG(avg_reply_time), 0)
		FROM mc_work_employee_statistic
		WHERE `+where, args...)
	var metric dashboard.StatisticEmployeeMetric
	if err := row.Scan(&metric.ChatCnt, &metric.MessageCnt, &metric.ReplyPercentage, &metric.AvgReplyTime); err != nil {
		return dashboard.StatisticEmployeeMetric{}, err
	}
	return metric, nil
}

func (s *MySQLStore) StatisticEmployeeLocalMetricsByEmployee(ctx context.Context, corpID int, employeeIDs []int, start time.Time, end time.Time) (map[int]dashboard.StatisticEmployeeMetric, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	result := make(map[int]dashboard.StatisticEmployeeMetric, len(employeeIDs))
	if len(employeeIDs) == 0 {
		return result, nil
	}
	where, args := statisticEmployeeWhere(corpID, employeeIDs, start, end)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			employee_id,
			COALESCE(SUM(chat_cnt), 0),
			COALESCE(SUM(message_cnt), 0),
			COALESCE(AVG(reply_percentage), 0),
			COALESCE(AVG(avg_reply_time), 0)
		FROM mc_work_employee_statistic
		WHERE `+where+`
		GROUP BY employee_id
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var employeeID int
		var metric dashboard.StatisticEmployeeMetric
		if err := rows.Scan(&employeeID, &metric.ChatCnt, &metric.MessageCnt, &metric.ReplyPercentage, &metric.AvgReplyTime); err != nil {
			return nil, err
		}
		result[employeeID] = metric
	}
	return result, rows.Err()
}

func (s *MySQLStore) StatisticEmployeeLocalTrend(ctx context.Context, corpID int, employeeIDs []int, start time.Time, end time.Time) ([]dashboard.StatisticBehaviorData, error) {
	where, args := statisticEmployeeWhere(corpID, employeeIDs, start, end)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			DATE(syn_time) AS stat_date,
			COALESCE(SUM(chat_cnt), 0),
			COALESCE(SUM(message_cnt), 0),
			COALESCE(AVG(reply_percentage), 0),
			COALESCE(AVG(avg_reply_time), 0)
		FROM mc_work_employee_statistic
		WHERE `+where+`
		GROUP BY DATE(syn_time)
		ORDER BY DATE(syn_time) ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.StatisticBehaviorData, 0)
	for rows.Next() {
		var date string
		var item dashboard.StatisticBehaviorData
		if err := rows.Scan(&date, &item.ChatCnt, &item.MessageCnt, &item.ReplyPercentage, &item.AvgReplyTime); err != nil {
			return nil, err
		}
		parsed, err := time.ParseInLocation("2006-01-02", date, time.Local)
		if err == nil {
			item.StatTime = parsed.Unix()
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) statisticEmployeeContactTotalAt(ctx context.Context, employeeIDs []int, end time.Time) (int, error) {
	args := make([]any, 0, len(employeeIDs)+1)
	args = append(args, formatDBTime(end))
	for _, employeeID := range employeeIDs {
		args = append(args, employeeID)
	}
	return s.countScalar(ctx, `
		SELECT COUNT(*)
		FROM mc_work_contact_employee
		WHERE created_at < ? AND deleted_at IS NULL AND employee_id IN (`+placeholders(len(employeeIDs))+`)
	`, args...)
}

func (s *MySQLStore) statisticEmployeeContactAddBetween(ctx context.Context, employeeIDs []int, start time.Time, end time.Time) (int, error) {
	args := make([]any, 0, len(employeeIDs)+2)
	args = append(args, formatDBTime(start), formatDBTime(end))
	for _, employeeID := range employeeIDs {
		args = append(args, employeeID)
	}
	return s.countScalar(ctx, `
		SELECT COUNT(*)
		FROM mc_work_contact_employee
		WHERE created_at > ? AND created_at < ? AND deleted_at IS NULL AND status = 1 AND employee_id IN (`+placeholders(len(employeeIDs))+`)
	`, args...)
}

func (s *MySQLStore) statisticEmployeeContactLossBetween(ctx context.Context, employeeIDs []int, start time.Time, end time.Time) (int, error) {
	args := make([]any, 0, len(employeeIDs)+2)
	args = append(args, formatDBTime(start), formatDBTime(end))
	for _, employeeID := range employeeIDs {
		args = append(args, employeeID)
	}
	return s.countScalar(ctx, `
		SELECT COUNT(*)
		FROM mc_work_contact_employee
		WHERE deleted_at > ? AND deleted_at < ? AND employee_id IN (`+placeholders(len(employeeIDs))+`)
	`, args...)
}

func scanStatisticEmployees(rows *sql.Rows) ([]dashboard.StatisticEmployee, error) {
	items := make([]dashboard.StatisticEmployee, 0)
	for rows.Next() {
		var item dashboard.StatisticEmployee
		var wxUserID, name, avatar sql.NullString
		if err := rows.Scan(&item.ID, &wxUserID, &name, &avatar); err != nil {
			return nil, err
		}
		item.WXUserID = nullString(wxUserID)
		item.Name = nullString(name)
		item.Avatar = nullString(avatar)
		items = append(items, item)
	}
	return items, rows.Err()
}

func statisticEmployeeWhere(corpID int, employeeIDs []int, start time.Time, end time.Time) (string, []any) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	where := []string{"corp_id = ?", "syn_time >= ?", "syn_time <= ?", "deleted_at IS NULL"}
	args := []any{corpID, formatDBTime(start), formatDBTime(end)}
	if len(employeeIDs) > 0 {
		where = append(where, "employee_id IN ("+placeholders(len(employeeIDs))+")")
		for _, employeeID := range employeeIDs {
			args = append(args, employeeID)
		}
	}
	return strings.Join(where, " AND "), args
}

func formatDBTime(value time.Time) string {
	return value.Format("2006-01-02 15:04:05")
}

func (s *MySQLStore) RoomWelcomeByID(ctx context.Context, roomWelcomeID int) (dashboard.RoomWelcomeItem, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, msg_text, complex_type, msg_complex, complex_template_id, create_user_id, created_at, updated_at
		FROM mc_room_welcome_template
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, roomWelcomeID)
	item, err := scanRoomWelcome(row)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.RoomWelcomeItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomWelcomeItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) RoomWelcomeCreatorNamesByIDs(ctx context.Context, userIDs []int) (map[int]string, error) {
	userIDs = uniquePositiveInts(userIDs)
	result := make(map[int]string, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(userIDs))
	for _, id := range userIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name
		FROM mc_user
		WHERE id IN (`+placeholders(len(userIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name sql.NullString
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		result[id] = nullString(name)
	}
	return result, rows.Err()
}

func (s *MySQLStore) RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (dashboard.RoomWelcomeCorpCredential, bool, error) {
	item, found, err := s.loadCorpCredentialByID(ctx, s.db, corpID, false)
	if err != nil || !found {
		return dashboard.RoomWelcomeCorpCredential{}, found, err
	}
	secret, err := s.decodeCorpCredential(item)
	if err != nil {
		return dashboard.RoomWelcomeCorpCredential{}, false, err
	}
	return dashboard.RoomWelcomeCorpCredential{CorpID: item.ID, WXCorpID: item.WXCorpID, ContactSecret: secret.ContactSecret}, true, nil
}

func (s *MySQLStore) RoomWelcomeCorpCredentialByWXCorpID(ctx context.Context, wxCorpID string) (dashboard.RoomWelcomeCorpCredential, bool, error) {
	item, found, err := s.loadCorpCredentialByWXCorpID(ctx, wxCorpID)
	if err != nil || !found {
		return dashboard.RoomWelcomeCorpCredential{}, found, err
	}
	secret, err := s.decodeCorpCredential(item)
	if err != nil {
		return dashboard.RoomWelcomeCorpCredential{}, false, err
	}
	return dashboard.RoomWelcomeCorpCredential{CorpID: item.ID, WXCorpID: item.WXCorpID, ContactSecret: secret.ContactSecret}, true, nil
}

func (s *MySQLStore) RoomTagPullRemindAgentByCorpID(ctx context.Context, corpID int) (dashboard.RoomTagPullAgentCredential, bool, error) {
	var agentID int
	err := s.db.QueryRowContext(ctx, `
		SELECT a.id
		FROM mc_work_agent a
		JOIN mc_corp c ON c.id = a.corp_id AND c.deleted_at IS NULL
		WHERE a.corp_id = ? AND `+authoritativeApplicationAgentSelectionSQL()+`
		LIMIT 1
	`, corpID).Scan(&agentID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.RoomTagPullAgentCredential{}, false, nil
	}
	if err != nil {
		return dashboard.RoomTagPullAgentCredential{}, false, err
	}
	agent, found, err := s.loadAgentCredentialByID(ctx, s.db, agentID, false)
	if err != nil || !found {
		return dashboard.RoomTagPullAgentCredential{}, found, err
	}
	secret, err := s.decodeAgentCredential(agent)
	if err != nil {
		return dashboard.RoomTagPullAgentCredential{}, false, err
	}
	return dashboard.RoomTagPullAgentCredential{CorpID: agent.CorpID, WXCorpID: agent.WXCorpID, WXAgentID: agent.WXAgentID, WXSecret: secret.WXSecret}, true, nil
}

func (s *MySQLStore) CreateWorkAgent(ctx context.Context, values dashboard.WorkAgentWriteValues, detail dashboard.WorkAgentDetail) (int, error) {
	storage, err := s.encodeAgentCredential(values.CorpID, values.WXAgentID, wecomcredentials.AgentCredential{WXSecret: values.WXSecret})
	if err != nil {
		return 0, err
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_work_agent (
			corp_id, wx_agent_id, wecom_credentials_ciphertext, wecom_credentials_key_id,
			name, square_logo_url, description, close,
			redirect_domain, report_location_flag, is_reportenter, home_url, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, values.CorpID, values.WXAgentID, storage.Ciphertext, storage.KeyID,
		detail.Name, detail.SquareLogoURL, detail.Description, detail.Close, detail.RedirectDomain, detail.ReportLocationFlag, detail.IsReportEnter, detail.HomeURL)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func (s *MySQLStore) SaaSQuotaStatus(ctx context.Context, tenantID int, metric string, additional int64) (dashboard.SaaSQuotaStatus, error) {
	status := dashboard.SaaSQuotaStatus{
		Metric:     metric,
		TenantID:   tenantID,
		Additional: additional,
	}
	current, err := s.saasCurrentUsage(ctx, tenantID, metric)
	if err != nil {
		return dashboard.SaaSQuotaStatus{}, err
	}
	status.Current = current
	limit, err := s.saasUsageLimit(ctx, tenantID, metric)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return status, nil
		}
		return dashboard.SaaSQuotaStatus{}, err
	}
	status.Limit = limit
	return status, nil
}

func (s *MySQLStore) RefreshSaaSUsageCounter(ctx context.Context, tenantID int, metric string) error {
	current, err := s.saasCurrentUsage(ctx, tenantID, metric)
	if err != nil {
		return err
	}
	limit, err := s.saasConfiguredUsageLimit(ctx, tenantID, metric)
	if err != nil {
		if isMissingSaaSTableError(err) {
			limit = 0
		} else {
			return err
		}
	}
	return s.upsertSaaSUsageCounter(ctx, tenantID, metric, current, limit, "runtime")
}

func (s *MySQLStore) RefreshSaaSUsageCounters(ctx context.Context, tenantID int) (SaaSUsageRefreshResult, error) {
	result := SaaSUsageRefreshResult{TenantID: tenantID}
	tenantIDs, err := s.saasUsageTenantIDs(ctx, tenantID)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return result, nil
		}
		return result, err
	}
	for _, id := range tenantIDs {
		if id <= 0 {
			continue
		}
		result.TenantsScanned++
		for _, metric := range saasUsageMetrics() {
			current, err := s.saasCurrentUsage(ctx, id, metric)
			if err != nil {
				return result, err
			}
			limit, err := s.saasConfiguredUsageLimit(ctx, id, metric)
			if err != nil {
				return result, err
			}
			if err := s.upsertSaaSUsageCounter(ctx, id, metric, current, limit, "maintenance"); err != nil {
				return result, err
			}
			result.MetricsRefreshed++
		}
		result.RefreshedTenants = append(result.RefreshedTenants, id)
	}
	return result, nil
}

func (s *MySQLStore) SaaSAdminOverview(ctx context.Context, options dashboard.SaaSAdminOverviewOptions) (dashboard.SaaSAdminOverview, error) {
	if options.Limit <= 0 {
		options.Limit = 20
	}
	if options.ExpiringDays <= 0 {
		options.ExpiringDays = 30
	}
	overview := dashboard.SaaSAdminOverview{}
	summary, err := s.saasAdminSummary(ctx, options)
	if err != nil {
		return dashboard.SaaSAdminOverview{}, err
	}
	overview.Summary = summary
	tenants, err := s.saasAdminTenants(ctx, options)
	if err != nil {
		return dashboard.SaaSAdminOverview{}, err
	}
	metrics, err := s.saasAdminMetrics(ctx, options)
	if err != nil {
		return dashboard.SaaSAdminOverview{}, err
	}
	overview.Tenants = s.attachSaaSAdminTenantUsage(ctx, tenants)
	overview.Metrics = metrics
	return overview, nil
}

func (s *MySQLStore) SaaSAdminPackages(ctx context.Context) ([]dashboard.SaaSAdminPackage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			code,
			name,
			description,
			status,
			version,
			max_corps,
			max_users,
			max_contacts,
			max_rooms,
			max_agents,
			channel_codes,
			shop_codes,
			radars,
			lotteries,
			room_infinite_pulls,
			room_fissions,
			room_clock_ins,
			room_qualities,
			room_calendars,
			room_reminds,
			contact_sops,
			room_sops,
			sensitive_words,
			storage_mb,
			contact_message_batches,
			room_message_batches,
			room_tag_pulls,
			work_room_auto_pulls,
			work_fissions,
			official_accounts,
			async_executions
		FROM mochat_go_saas_packages
		WHERE deleted_at IS NULL
		ORDER BY status ASC, code ASC
	`)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	packages := []dashboard.SaaSAdminPackage{}
	for rows.Next() {
		var item dashboard.SaaSAdminPackage
		if err := rows.Scan(
			&item.ID,
			&item.Code,
			&item.Name,
			&item.Description,
			&item.Status,
			&item.Version,
			&item.Limits.MaxCorps,
			&item.Limits.MaxUsers,
			&item.Limits.MaxContacts,
			&item.Limits.MaxRooms,
			&item.Limits.MaxAgents,
			&item.Limits.ChannelCodes,
			&item.Limits.ShopCodes,
			&item.Limits.Radars,
			&item.Limits.Lotteries,
			&item.Limits.RoomInfinitePulls,
			&item.Limits.RoomFissions,
			&item.Limits.RoomClockIns,
			&item.Limits.RoomQualities,
			&item.Limits.RoomCalendars,
			&item.Limits.RoomReminds,
			&item.Limits.ContactSOPs,
			&item.Limits.RoomSOPs,
			&item.Limits.SensitiveWords,
			&item.Limits.StorageMB,
			&item.Limits.ContactMessageBatches,
			&item.Limits.RoomMessageBatches,
			&item.Limits.RoomTagPulls,
			&item.Limits.WorkRoomAutoPulls,
			&item.Limits.WorkFissions,
			&item.Limits.OfficialAccounts,
			&item.Limits.AsyncExecutions,
		); err != nil {
			return nil, err
		}
		packages = append(packages, item)
	}
	return packages, rows.Err()
}

func (s *MySQLStore) SaaSAdminOperationLogs(ctx context.Context, options dashboard.SaaSAdminOperationLogOptions) ([]dashboard.SaaSAdminOperationLog, error) {
	if options.Limit <= 0 {
		options.Limit = 20
	}
	if options.Limit > 5000 {
		options.Limit = 5000
	}
	where, args := saasAdminOperationLogWhere(options)
	args = append(args, options.Limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			tenant_id,
			actor_user_id,
			actor_tenant_id,
			action,
			target_type,
			target_id,
			target_name,
			COALESCE(CAST(before_json AS CHAR), ''),
			COALESCE(CAST(after_json AS CHAR), ''),
			remark,
			COALESCE(DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_admin_operation_logs
		`+where+`
		ORDER BY id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	logs := []dashboard.SaaSAdminOperationLog{}
	for rows.Next() {
		var item dashboard.SaaSAdminOperationLog
		if err := rows.Scan(
			&item.ID,
			&item.TenantID,
			&item.ActorUserID,
			&item.ActorTenantID,
			&item.Action,
			&item.TargetType,
			&item.TargetID,
			&item.TargetName,
			&item.BeforeJSON,
			&item.AfterJSON,
			&item.Remark,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, item)
	}
	return logs, rows.Err()
}

func (s *MySQLStore) SaaSAdminOperationLogSummary(ctx context.Context, options dashboard.SaaSAdminOperationLogOptions) (dashboard.SaaSAdminOperationLogSummary, error) {
	where, args := saasAdminOperationLogWhere(options)
	var summary dashboard.SaaSAdminOperationLogSummary
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COUNT(DISTINCT NULLIF(tenant_id, 0)),
			COUNT(DISTINCT NULLIF(actor_user_id, 0)),
			COUNT(DISTINCT NULLIF(action, '')),
			COUNT(DISTINCT NULLIF(target_type, ''))
		FROM mochat_go_saas_admin_operation_logs
		`+where+`
	`, args...).Scan(
		&summary.OperationCount,
		&summary.TenantCount,
		&summary.ActorUserCount,
		&summary.ActionCount,
		&summary.TargetTypeCount,
	)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminOperationLogSummary{}, nil
		}
		return dashboard.SaaSAdminOperationLogSummary{}, err
	}
	return summary, nil
}

func saasAdminOperationLogWhere(options dashboard.SaaSAdminOperationLogOptions) (string, []any) {
	args := []any{}
	where := "WHERE deleted_at IS NULL"
	if options.TenantID > 0 {
		where += " AND tenant_id = ?"
		args = append(args, options.TenantID)
	}
	if options.ExcludedTenantID > 0 {
		where += " AND tenant_id <> ?"
		args = append(args, options.ExcludedTenantID)
	}
	action := strings.TrimSpace(options.Action)
	if action != "" {
		where += " AND action = ?"
		args = append(args, action)
	}
	targetType := strings.TrimSpace(options.TargetType)
	if targetType != "" {
		where += " AND target_type = ?"
		args = append(args, targetType)
	}
	keyword := strings.TrimSpace(options.Keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		where += " AND (action LIKE ? OR target_type LIKE ? OR target_id LIKE ? OR target_name LIKE ? OR remark LIKE ? OR CAST(before_json AS CHAR) LIKE ? OR CAST(after_json AS CHAR) LIKE ?)"
		args = append(args, like, like, like, like, like, like, like)
	}
	return where, args
}

func (s *MySQLStore) RecordSaaSAdminOperationLog(ctx context.Context, item dashboard.SaaSAdminOperationLog) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer rollbackQuietly(tx)

	id, err := insertSaaSAdminOperationLogTx(ctx, tx, item)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return 0, nil
		}
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// RecordSaaSAdminOperationLogInTx lets a caller keep its domain mutation and
// the integrity-chain audit record in the same database transaction.
func (s *MySQLStore) RecordSaaSAdminOperationLogInTx(ctx context.Context, tx *sql.Tx, item dashboard.SaaSAdminOperationLog) (int64, error) {
	if s == nil || s.db == nil || tx == nil {
		return 0, errors.New("SaaS admin operation log transaction is unavailable")
	}
	return insertSaaSAdminOperationLogTx(ctx, tx, item)
}

func (s *MySQLStore) SaaSAdminBillingEvents(ctx context.Context, options dashboard.SaaSAdminBillingEventOptions) ([]dashboard.SaaSAdminBillingEvent, error) {
	if options.Limit <= 0 {
		options.Limit = 20
	}
	if options.Limit > 5000 {
		options.Limit = 5000
	}
	where, args := saasAdminBillingEventWhere(options)
	args = append(args, options.Limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			tenant_id,
			event_type,
			package_code,
			package_name,
			COALESCE(DATE_FORMAT(previous_expires_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(new_expires_at, '%Y-%m-%d %H:%i:%s'), ''),
			amount_cents,
			currency,
			COALESCE(DATE_FORMAT(paid_at, '%Y-%m-%d %H:%i:%s'), ''),
			payment_method,
			external_order_no,
			actor_user_id,
			actor_tenant_id,
			remark,
			COALESCE(CAST(metadata_json AS CHAR), ''),
			COALESCE(DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_billing_events
		`+where+`
		ORDER BY id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	events := []dashboard.SaaSAdminBillingEvent{}
	for rows.Next() {
		var item dashboard.SaaSAdminBillingEvent
		if err := rows.Scan(
			&item.ID,
			&item.TenantID,
			&item.EventType,
			&item.PackageCode,
			&item.PackageName,
			&item.PreviousExpiresAt,
			&item.NewExpiresAt,
			&item.AmountCents,
			&item.Currency,
			&item.PaidAt,
			&item.PaymentMethod,
			&item.ExternalOrderNo,
			&item.ActorUserID,
			&item.ActorTenantID,
			&item.Remark,
			&item.MetadataJSON,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, item)
	}
	return events, rows.Err()
}

func (s *MySQLStore) SaaSAdminBillingEventByID(ctx context.Context, id int64) (dashboard.SaaSAdminBillingEvent, bool, error) {
	if id <= 0 {
		return dashboard.SaaSAdminBillingEvent{}, false, nil
	}
	var item dashboard.SaaSAdminBillingEvent
	err := s.db.QueryRowContext(ctx, `
		SELECT
			id,
			tenant_id,
			event_type,
			package_code,
			package_name,
			COALESCE(DATE_FORMAT(previous_expires_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(new_expires_at, '%Y-%m-%d %H:%i:%s'), ''),
			amount_cents,
			currency,
			COALESCE(DATE_FORMAT(paid_at, '%Y-%m-%d %H:%i:%s'), ''),
			payment_method,
			external_order_no,
			actor_user_id,
			actor_tenant_id,
			remark,
			COALESCE(CAST(metadata_json AS CHAR), ''),
			COALESCE(DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_billing_events
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, id).Scan(
		&item.ID,
		&item.TenantID,
		&item.EventType,
		&item.PackageCode,
		&item.PackageName,
		&item.PreviousExpiresAt,
		&item.NewExpiresAt,
		&item.AmountCents,
		&item.Currency,
		&item.PaidAt,
		&item.PaymentMethod,
		&item.ExternalOrderNo,
		&item.ActorUserID,
		&item.ActorTenantID,
		&item.Remark,
		&item.MetadataJSON,
		&item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) || isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminBillingEvent{}, false, nil
	}
	if err != nil {
		return dashboard.SaaSAdminBillingEvent{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) SaaSAdminBillingEventSummary(ctx context.Context, options dashboard.SaaSAdminBillingEventOptions) (dashboard.SaaSAdminDailyBillingSummary, error) {
	where, args := saasAdminBillingEventWhere(options)
	var summary dashboard.SaaSAdminDailyBillingSummary
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN event_type = 'renewal' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN event_type = 'refund' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN event_type <> 'refund' THEN amount_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN event_type = 'refund' THEN amount_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN event_type = 'refund' THEN -CAST(amount_cents AS SIGNED) ELSE CAST(amount_cents AS SIGNED) END), 0)
		FROM mochat_go_saas_billing_events
		`+where+`
	`, args...).Scan(
		&summary.EventCount, &summary.RenewalCount, &summary.RefundCount,
		&summary.GrossAmountCents, &summary.RefundAmountCents, &summary.AmountCents,
	)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminDailyBillingSummary{}, nil
		}
		return dashboard.SaaSAdminDailyBillingSummary{}, err
	}
	return summary, nil
}

func (s *MySQLStore) SaaSAdminBillingReconciliation(ctx context.Context, options dashboard.SaaSAdminBillingReconciliationOptions) (dashboard.SaaSAdminBillingReconciliationReport, error) {
	if options.Limit <= 0 {
		options.Limit = 20
	}
	if options.Limit > 5000 {
		options.Limit = 5000
	}
	where, args := saasAdminBillingEventWhereForAlias(options.SaaSAdminBillingEventOptions, "b")
	if options.MismatchOnly {
		where += " AND " + saasAdminBillingReconciliationMismatchSQL()
	}
	var report dashboard.SaaSAdminBillingReconciliationReport
	summaryQuery := `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN NOT (` + saasAdminBillingReconciliationMismatchSQL() + `) THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN ` + saasAdminBillingReconciliationMismatchSQL() + ` THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN tp.tenant_id IS NULL THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN tp.tenant_id IS NOT NULL AND COALESCE(tp.status, 0) <> 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN b.package_code <> '' AND COALESCE(tp.package_code, '') <> b.package_code THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN b.new_expires_at IS NOT NULL AND tp.expires_at IS NOT NULL AND tp.expires_at < b.new_expires_at THEN 1 ELSE 0 END), 0)
		FROM mochat_go_saas_billing_events b
		LEFT JOIN mochat_go_saas_tenant_packages tp ON tp.tenant_id = b.tenant_id AND tp.deleted_at IS NULL
		LEFT JOIN mc_tenant t ON t.id = b.tenant_id AND t.deleted_at IS NULL
		` + where
	if err := s.db.QueryRowContext(ctx, summaryQuery, args...).Scan(
		&report.Summary.CheckedCount,
		&report.Summary.MatchedCount,
		&report.Summary.MismatchedCount,
		&report.Summary.MissingPackageCount,
		&report.Summary.InactivePackageCount,
		&report.Summary.PackageMismatchCount,
		&report.Summary.ExpiresMismatchCount,
	); err != nil {
		if isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminBillingReconciliationReport{}, nil
		}
		return dashboard.SaaSAdminBillingReconciliationReport{}, err
	}

	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, options.Limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			b.id,
			b.tenant_id,
			b.event_type,
			b.package_code,
			b.package_name,
			COALESCE(DATE_FORMAT(b.previous_expires_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(b.new_expires_at, '%Y-%m-%d %H:%i:%s'), ''),
			b.amount_cents,
			b.currency,
			COALESCE(DATE_FORMAT(b.paid_at, '%Y-%m-%d %H:%i:%s'), ''),
			b.payment_method,
			b.external_order_no,
			b.actor_user_id,
			b.actor_tenant_id,
			b.remark,
			COALESCE(CAST(b.metadata_json AS CHAR), ''),
			COALESCE(DATE_FORMAT(b.created_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(t.name, ''),
			CASE WHEN tp.tenant_id IS NULL THEN 0 ELSE 1 END,
			COALESCE(tp.package_code, ''),
			COALESCE(tp.package_name, ''),
			COALESCE(DATE_FORMAT(tp.expires_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(tp.status, 0)
		FROM mochat_go_saas_billing_events b
		LEFT JOIN mochat_go_saas_tenant_packages tp ON tp.tenant_id = b.tenant_id AND tp.deleted_at IS NULL
		LEFT JOIN mc_tenant t ON t.id = b.tenant_id AND t.deleted_at IS NULL
		`+where+`
		ORDER BY b.id DESC
		LIMIT ?
	`, listArgs...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminBillingReconciliationReport{}, nil
		}
		return dashboard.SaaSAdminBillingReconciliationReport{}, err
	}
	defer rows.Close()

	report.Items = []dashboard.SaaSAdminBillingReconciliationItem{}
	for rows.Next() {
		var item dashboard.SaaSAdminBillingReconciliationItem
		var packageFound int
		if err := rows.Scan(
			&item.BillingEvent.ID,
			&item.BillingEvent.TenantID,
			&item.BillingEvent.EventType,
			&item.BillingEvent.PackageCode,
			&item.BillingEvent.PackageName,
			&item.BillingEvent.PreviousExpiresAt,
			&item.BillingEvent.NewExpiresAt,
			&item.BillingEvent.AmountCents,
			&item.BillingEvent.Currency,
			&item.BillingEvent.PaidAt,
			&item.BillingEvent.PaymentMethod,
			&item.BillingEvent.ExternalOrderNo,
			&item.BillingEvent.ActorUserID,
			&item.BillingEvent.ActorTenantID,
			&item.BillingEvent.Remark,
			&item.BillingEvent.MetadataJSON,
			&item.BillingEvent.CreatedAt,
			&item.TenantName,
			&packageFound,
			&item.CurrentPackageCode,
			&item.CurrentPackageName,
			&item.CurrentExpiresAt,
			&item.CurrentPackageStatus,
		); err != nil {
			return dashboard.SaaSAdminBillingReconciliationReport{}, err
		}
		item.CurrentPackageFound = packageFound == 1
		report.Items = append(report.Items, item)
	}
	return report, rows.Err()
}

func (s *MySQLStore) SaaSAdminBillingReconciliationFollowUpSnapshots(ctx context.Context, options dashboard.SaaSAdminBillingReconciliationFollowUpOptions) ([]dashboard.SaaSAdminBillingReconciliationFollowUpSnapshot, error) {
	rows, err := s.querySaaSAdminLatestBillingReconciliationFollowUpRows(ctx, options.TenantID, options.ExcludedTenantID)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	status := strings.TrimSpace(options.Status)
	owner := strings.ToLower(strings.TrimSpace(options.Owner))
	keyword := strings.ToLower(strings.TrimSpace(options.Keyword))
	limit := options.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 5000 {
		limit = 5000
	}
	items := make([]dashboard.SaaSAdminBillingReconciliationFollowUpSnapshot, 0, limit)
	for rows.Next() {
		item, err := scanSaaSAdminBillingReconciliationFollowUpSnapshot(rows)
		if err != nil {
			return nil, err
		}
		if status != "" && item.Status != status {
			continue
		}
		if owner != "" && !strings.Contains(strings.ToLower(item.Owner), owner) {
			continue
		}
		if keyword != "" && !saasAdminBillingReconciliationFollowUpSnapshotMatchesKeyword(item, keyword) {
			continue
		}
		items = append(items, item)
		if len(items) >= limit {
			break
		}
	}
	return items, rows.Err()
}

func saasAdminBillingReconciliationMismatchSQL() string {
	return `(tp.tenant_id IS NULL OR COALESCE(tp.status, 0) <> 1 OR (b.package_code <> '' AND COALESCE(tp.package_code, '') <> b.package_code) OR (b.new_expires_at IS NOT NULL AND tp.expires_at IS NOT NULL AND tp.expires_at < b.new_expires_at))`
}

func saasAdminBillingEventWhere(options dashboard.SaaSAdminBillingEventOptions) (string, []any) {
	return saasAdminBillingEventWhereForAlias(options, "")
}

func saasAdminBillingEventWhereForAlias(options dashboard.SaaSAdminBillingEventOptions, alias string) (string, []any) {
	args := []any{}
	field := func(name string) string {
		if alias == "" {
			return name
		}
		return alias + "." + name
	}
	where := "WHERE deleted_at IS NULL"
	if alias != "" {
		where = "WHERE " + field("deleted_at") + " IS NULL"
	}
	if options.TenantID > 0 {
		where += " AND " + field("tenant_id") + " = ?"
		args = append(args, options.TenantID)
	}
	if options.ExcludedTenantID > 0 {
		where += " AND " + field("tenant_id") + " <> ?"
		args = append(args, options.ExcludedTenantID)
	}
	eventType := strings.TrimSpace(options.EventType)
	if eventType != "" {
		where += " AND " + field("event_type") + " = ?"
		args = append(args, eventType)
	}
	packageCode := strings.TrimSpace(options.PackageCode)
	if packageCode != "" {
		where += " AND " + field("package_code") + " = ?"
		args = append(args, packageCode)
	}
	keyword := strings.TrimSpace(options.Keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		where += " AND (" + field("event_type") + " LIKE ? OR " + field("package_code") + " LIKE ? OR " + field("package_name") + " LIKE ? OR " + field("payment_method") + " LIKE ? OR " + field("external_order_no") + " LIKE ? OR " + field("remark") + " LIKE ? OR CAST(" + field("metadata_json") + " AS CHAR) LIKE ?)"
		args = append(args, like, like, like, like, like, like, like)
	}
	return where, args
}

func (s *MySQLStore) SaaSAdminTaskSummary(ctx context.Context, options dashboard.SaaSAdminTaskOptions) (dashboard.SaaSAdminTaskSummary, error) {
	where, args := saasAdminTaskWhere(options)
	queryArgs := []any{
		dashboard.SaaSAdminTaskStatusPending,
		dashboard.SaaSAdminTaskStatusBlocked,
		dashboard.SaaSAdminTaskStatusFailed,
		dashboard.SaaSAdminTaskStatusApplied,
		dashboard.SaaSAdminTaskStatusCanceled,
		dashboard.SaaSAdminTaskStatusPending,
		dashboard.SaaSAdminTaskStatusBlocked,
		dashboard.SaaSAdminTaskStatusFailed,
		dashboard.SaaSAdminTaskTypePackageSync,
		dashboard.SaaSAdminTaskTypeTenantRenewal,
		dashboard.SaaSAdminTaskTypeTenantProvision,
	}
	queryArgs = append(queryArgs, args...)
	var summary dashboard.SaaSAdminTaskSummary
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status IN (?, ?, ?) THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN task_type = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN task_type = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN task_type = ? THEN 1 ELSE 0 END), 0),
			COUNT(DISTINCT NULLIF(tenant_id, 0)),
			COUNT(DISTINCT NULLIF(actor_user_id, 0))
		FROM mochat_go_saas_admin_tasks
		`+where, queryArgs...).Scan(
		&summary.TaskCount,
		&summary.PendingCount,
		&summary.BlockedCount,
		&summary.FailedCount,
		&summary.AppliedCount,
		&summary.CanceledCount,
		&summary.ActionableCount,
		&summary.PackageSyncCount,
		&summary.TenantRenewalCount,
		&summary.TenantProvisionCount,
		&summary.TenantCount,
		&summary.ActorUserCount,
	)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminTaskSummary{}, nil
		}
		return dashboard.SaaSAdminTaskSummary{}, err
	}
	return summary, nil
}

func (s *MySQLStore) SaaSAdminTasks(ctx context.Context, options dashboard.SaaSAdminTaskOptions) ([]dashboard.SaaSAdminTask, error) {
	if options.Limit <= 0 {
		options.Limit = 20
	}
	if options.Limit > 5000 {
		options.Limit = 5000
	}
	where, args := saasAdminTaskWhere(options)
	args = append(args, options.Limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			task_type,
			status,
			version,
			tenant_id,
			package_code,
			actor_user_id,
			actor_tenant_id,
			COALESCE(CAST(request_json AS CHAR), ''),
			COALESCE(CAST(preview_json AS CHAR), ''),
			COALESCE(CAST(result_json AS CHAR), ''),
			remark,
			COALESCE(last_error, ''),
			COALESCE(DATE_FORMAT(applied_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(updated_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_admin_tasks
		`+where+`
		ORDER BY id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	tasks := []dashboard.SaaSAdminTask{}
	for rows.Next() {
		var item dashboard.SaaSAdminTask
		if err := rows.Scan(
			&item.ID,
			&item.TaskType,
			&item.Status,
			&item.Version,
			&item.TenantID,
			&item.PackageCode,
			&item.ActorUserID,
			&item.ActorTenantID,
			&item.RequestJSON,
			&item.PreviewJSON,
			&item.ResultJSON,
			&item.Remark,
			&item.LastError,
			&item.AppliedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		tasks = append(tasks, item)
	}
	return tasks, rows.Err()
}

func saasAdminTaskWhere(options dashboard.SaaSAdminTaskOptions) (string, []any) {
	args := []any{}
	where := "WHERE deleted_at IS NULL"
	if options.TaskID > 0 {
		where += " AND id = ?"
		args = append(args, options.TaskID)
	}
	taskType := strings.TrimSpace(options.TaskType)
	if taskType != "" {
		where += " AND task_type = ?"
		args = append(args, taskType)
	}
	status := strings.TrimSpace(options.Status)
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if options.TenantID > 0 {
		where += " AND tenant_id = ?"
		args = append(args, options.TenantID)
	}
	if options.ExcludedTenantID > 0 {
		where += " AND tenant_id <> ?"
		args = append(args, options.ExcludedTenantID)
	}
	packageCode := strings.TrimSpace(options.PackageCode)
	if packageCode != "" {
		where += " AND package_code = ?"
		args = append(args, packageCode)
	}
	return where, args
}

func (s *MySQLStore) CreateSaaSAdminTask(ctx context.Context, task dashboard.SaaSAdminTaskCreate) (dashboard.SaaSAdminTask, error) {
	task.TaskType = strings.TrimSpace(task.TaskType)
	task.Status = strings.TrimSpace(task.Status)
	if task.TaskType == "" {
		return dashboard.SaaSAdminTask{}, dashboard.NewSaaSAdminBadRequest("taskType required")
	}
	if task.TaskType != dashboard.SaaSAdminTaskTypePackageSync && task.TaskType != dashboard.SaaSAdminTaskTypeTenantRenewal && task.TaskType != dashboard.SaaSAdminTaskTypeTenantProvision {
		return dashboard.SaaSAdminTask{}, dashboard.NewSaaSAdminBadRequest("taskType unsupported")
	}
	if task.Status == "" {
		task.Status = dashboard.SaaSAdminTaskStatusPending
	}
	if !validSaaSAdminTaskStatus(task.Status) {
		return dashboard.SaaSAdminTask{}, dashboard.NewSaaSAdminBadRequest("task status invalid")
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_admin_tasks
			(task_type, status, tenant_id, package_code, actor_user_id, actor_tenant_id, request_json, preview_json, remark, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NULL)
	`, task.TaskType,
		task.Status,
		task.TenantID,
		truncateRunes(strings.TrimSpace(task.PackageCode), 64),
		task.ActorUserID,
		task.ActorTenantID,
		saasAdminJSONValue(task.RequestJSON),
		saasAdminJSONValue(task.PreviewJSON),
		truncateRunes(strings.TrimSpace(task.Remark), 255),
	)
	if err != nil {
		return dashboard.SaaSAdminTask{}, err
	}
	taskID, _ := result.LastInsertId()
	item, err := s.saasAdminTaskByID(ctx, taskID)
	if err != nil {
		return dashboard.SaaSAdminTask{}, err
	}
	return item, nil
}

func (s *MySQLStore) UpdateSaaSAdminTaskStatus(ctx context.Context, update dashboard.SaaSAdminTaskStatusUpdate) (dashboard.SaaSAdminTask, error) {
	if update.TaskID <= 0 {
		return dashboard.SaaSAdminTask{}, dashboard.NewSaaSAdminBadRequest("taskId required")
	}
	update.Status = strings.TrimSpace(update.Status)
	if !validSaaSAdminTaskStatus(update.Status) {
		return dashboard.SaaSAdminTask{}, dashboard.NewSaaSAdminBadRequest("task status invalid")
	}
	if update.ExpectedVersion < 0 {
		return dashboard.SaaSAdminTask{}, dashboard.NewSaaSAdminBadRequest("expectedVersion must not be negative")
	}
	where := "WHERE id = ? AND deleted_at IS NULL"
	args := []any{update.Status, saasAdminJSONValue(update.ResultJSON), truncateRunes(strings.TrimSpace(update.LastError), 2000), update.TaskID}
	if update.ExpectedVersion > 0 {
		where += " AND version = ?"
		args = append(args, update.ExpectedVersion)
	}
	var result sql.Result
	var err error
	if update.Applied {
		result, err = s.db.ExecContext(ctx, `
			UPDATE mochat_go_saas_admin_tasks
			SET status = ?, result_json = ?, last_error = ?, applied_at = NOW(), version = version + 1, updated_at = NOW()
			`+where, args...)
	} else {
		result, err = s.db.ExecContext(ctx, `
			UPDATE mochat_go_saas_admin_tasks
			SET status = ?, result_json = ?, last_error = ?, version = version + 1, updated_at = NOW()
			`+where, args...)
	}
	if err != nil {
		return dashboard.SaaSAdminTask{}, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		if update.ExpectedVersion > 0 {
			return dashboard.SaaSAdminTask{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "任务版本已变化，请刷新后重试"}
		}
		return dashboard.SaaSAdminTask{}, dashboard.NewSaaSAdminNotFound("task not found")
	}
	return s.saasAdminTaskByID(ctx, update.TaskID)
}

func (s *MySQLStore) saasAdminTaskByID(ctx context.Context, taskID int64) (dashboard.SaaSAdminTask, error) {
	tasks, err := s.SaaSAdminTasks(ctx, dashboard.SaaSAdminTaskOptions{TaskID: taskID, Limit: 1})
	if err != nil {
		return dashboard.SaaSAdminTask{}, err
	}
	if len(tasks) == 0 {
		return dashboard.SaaSAdminTask{}, dashboard.NewSaaSAdminNotFound("task not found")
	}
	return tasks[0], nil
}

func validSaaSAdminTaskStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case dashboard.SaaSAdminTaskStatusPending, dashboard.SaaSAdminTaskStatusBlocked, dashboard.SaaSAdminTaskStatusApplied, dashboard.SaaSAdminTaskStatusFailed, dashboard.SaaSAdminTaskStatusCanceled:
		return true
	default:
		return false
	}
}

func (s *MySQLStore) SaaSAdminLatestRiskFollowUps(ctx context.Context, tenantIDs []int) (map[int]dashboard.SaaSAdminRiskFollowUpSnapshot, error) {
	cleaned := make([]int, 0, len(tenantIDs))
	seen := map[int]struct{}{}
	for _, tenantID := range tenantIDs {
		if tenantID <= 0 {
			continue
		}
		if _, ok := seen[tenantID]; ok {
			continue
		}
		seen[tenantID] = struct{}{}
		cleaned = append(cleaned, tenantID)
	}
	if len(cleaned) == 0 {
		return map[int]dashboard.SaaSAdminRiskFollowUpSnapshot{}, nil
	}
	args := make([]any, 0, len(cleaned))
	for _, tenantID := range cleaned {
		args = append(args, tenantID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			l.id,
			l.tenant_id,
			l.target_name,
			COALESCE(CAST(l.after_json AS CHAR), ''),
			l.remark,
			COALESCE(DATE_FORMAT(l.created_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_admin_operation_logs l
		INNER JOIN (
			SELECT tenant_id, MAX(id) AS latest_id
			FROM mochat_go_saas_admin_operation_logs
			WHERE deleted_at IS NULL
			  AND action = 'tenant.risk.follow_up'
			  AND tenant_id IN (`+placeholders(len(cleaned))+`)
			GROUP BY tenant_id
		) latest ON latest.latest_id = l.id
		ORDER BY l.id DESC
	`, args...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return map[int]dashboard.SaaSAdminRiskFollowUpSnapshot{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	items := map[int]dashboard.SaaSAdminRiskFollowUpSnapshot{}
	for rows.Next() {
		var item dashboard.SaaSAdminRiskFollowUpSnapshot
		var afterJSON string
		if err := rows.Scan(
			&item.OperationID,
			&item.TenantID,
			&item.TenantName,
			&afterJSON,
			&item.Remark,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		applySaaSAdminRiskFollowUpJSON(&item, afterJSON)
		items[item.TenantID] = item
	}
	return items, rows.Err()
}

func (s *MySQLStore) SaaSAdminRiskFollowUpSnapshots(ctx context.Context, options dashboard.SaaSAdminRiskFollowUpTaskOptions) ([]dashboard.SaaSAdminRiskFollowUpSnapshot, error) {
	rows, err := s.querySaaSAdminLatestRiskFollowUpRows(ctx, options.TenantID, options.ExcludedTenantID)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	status := strings.TrimSpace(options.Status)
	owner := strings.ToLower(strings.TrimSpace(options.Owner))
	keyword := strings.ToLower(strings.TrimSpace(options.Keyword))
	limit := options.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 5000 {
		limit = 5000
	}
	items := make([]dashboard.SaaSAdminRiskFollowUpSnapshot, 0, limit)
	for rows.Next() {
		item, err := scanSaaSAdminRiskFollowUpSnapshot(rows)
		if err != nil {
			return nil, err
		}
		if status != "" && item.Status != status {
			continue
		}
		if owner != "" && !strings.Contains(strings.ToLower(item.Owner), owner) {
			continue
		}
		if keyword != "" && !saasAdminRiskFollowUpSnapshotMatchesKeyword(item, keyword) {
			continue
		}
		items = append(items, item)
		if len(items) >= limit {
			break
		}
	}
	return items, rows.Err()
}

type saasAdminRiskFollowUpScanner interface {
	Scan(dest ...any) error
}

func (s *MySQLStore) querySaaSAdminLatestRiskFollowUpRows(ctx context.Context, tenantID int, excludedTenantID int) (*sql.Rows, error) {
	args := []any{}
	tenantWhere := ""
	if tenantID > 0 {
		tenantWhere += " AND tenant_id = ?"
		args = append(args, tenantID)
	}
	if excludedTenantID > 0 {
		tenantWhere += " AND tenant_id <> ?"
		args = append(args, excludedTenantID)
	}
	return s.db.QueryContext(ctx, `
		SELECT
			l.id,
			l.tenant_id,
			l.target_name,
			COALESCE(CAST(l.after_json AS CHAR), ''),
			l.remark,
			COALESCE(DATE_FORMAT(l.created_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_admin_operation_logs l
		INNER JOIN (
			SELECT tenant_id, MAX(id) AS latest_id
			FROM mochat_go_saas_admin_operation_logs
			WHERE deleted_at IS NULL
			  AND action = 'tenant.risk.follow_up'
			  `+tenantWhere+`
			GROUP BY tenant_id
		) latest ON latest.latest_id = l.id
		ORDER BY l.id DESC
	`, args...)
}

func scanSaaSAdminRiskFollowUpSnapshot(scanner saasAdminRiskFollowUpScanner) (dashboard.SaaSAdminRiskFollowUpSnapshot, error) {
	var item dashboard.SaaSAdminRiskFollowUpSnapshot
	var afterJSON string
	if err := scanner.Scan(
		&item.OperationID,
		&item.TenantID,
		&item.TenantName,
		&afterJSON,
		&item.Remark,
		&item.CreatedAt,
	); err != nil {
		return item, err
	}
	applySaaSAdminRiskFollowUpJSON(&item, afterJSON)
	return item, nil
}

func (s *MySQLStore) querySaaSAdminLatestBillingReconciliationFollowUpRows(ctx context.Context, tenantID int, excludedTenantID int) (*sql.Rows, error) {
	args := []any{}
	tenantWhere := ""
	if tenantID > 0 {
		tenantWhere += " AND tenant_id = ?"
		args = append(args, tenantID)
	}
	if excludedTenantID > 0 {
		tenantWhere += " AND tenant_id <> ?"
		args = append(args, excludedTenantID)
	}
	return s.db.QueryContext(ctx, `
		SELECT
			l.id,
			l.tenant_id,
			COALESCE(t.name, ''),
			l.target_id,
			l.target_name,
			COALESCE(CAST(l.before_json AS CHAR), ''),
			COALESCE(CAST(l.after_json AS CHAR), ''),
			l.remark,
			COALESCE(DATE_FORMAT(l.created_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_admin_operation_logs l
		INNER JOIN (
			SELECT target_id, MAX(id) AS latest_id
			FROM mochat_go_saas_admin_operation_logs
			WHERE deleted_at IS NULL
			  AND action = 'billing.reconciliation.follow_up'
			  AND target_type = 'billing_event'
			  `+tenantWhere+`
			GROUP BY target_id
		) latest ON latest.latest_id = l.id
		LEFT JOIN mc_tenant t ON t.id = l.tenant_id AND t.deleted_at IS NULL
		ORDER BY l.id DESC
	`, args...)
}

func scanSaaSAdminBillingReconciliationFollowUpSnapshot(scanner saasAdminRiskFollowUpScanner) (dashboard.SaaSAdminBillingReconciliationFollowUpSnapshot, error) {
	var item dashboard.SaaSAdminBillingReconciliationFollowUpSnapshot
	var targetID string
	var targetName string
	var beforeJSON string
	var afterJSON string
	if err := scanner.Scan(
		&item.OperationID,
		&item.TenantID,
		&item.TenantName,
		&targetID,
		&targetName,
		&beforeJSON,
		&afterJSON,
		&item.Remark,
		&item.CreatedAt,
	); err != nil {
		return item, err
	}
	if id, err := strconv.ParseInt(strings.TrimSpace(targetID), 10, 64); err == nil {
		item.BillingEventID = id
	}
	item.ExternalOrderNo = strings.TrimSpace(targetName)
	applySaaSAdminBillingReconciliationFollowUpJSON(&item, beforeJSON, afterJSON)
	return item, nil
}

func saasAdminRiskFollowUpSnapshotMatchesKeyword(item dashboard.SaaSAdminRiskFollowUpSnapshot, keyword string) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	fields := []string{
		strconv.Itoa(item.TenantID),
		item.TenantName,
		item.Status,
		item.Owner,
		item.NextFollowUpAt,
		item.Remark,
		item.CreatedAt,
		strconv.FormatInt(item.OperationID, 10),
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), keyword) {
			return true
		}
	}
	return false
}

func saasAdminBillingReconciliationFollowUpSnapshotMatchesKeyword(item dashboard.SaaSAdminBillingReconciliationFollowUpSnapshot, keyword string) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	fields := []string{
		strconv.FormatInt(item.BillingEventID, 10),
		strconv.Itoa(item.TenantID),
		item.TenantName,
		item.Status,
		item.Owner,
		item.NextFollowUpAt,
		item.Remark,
		item.PackageCode,
		item.PackageName,
		item.NewExpiresAt,
		item.Currency,
		item.ExternalOrderNo,
		item.CreatedAt,
		strconv.FormatInt(item.OperationID, 10),
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), keyword) {
			return true
		}
	}
	return false
}

func (s *MySQLStore) RecordSaaSAdminRiskFollowUp(ctx context.Context, followUp dashboard.SaaSAdminRiskFollowUp) (dashboard.SaaSAdminRiskFollowUpResult, error) {
	if followUp.TenantID <= 0 {
		return dashboard.SaaSAdminRiskFollowUpResult{}, dashboard.NewSaaSAdminBadRequest("tenantId required")
	}
	followUp.Status = strings.TrimSpace(followUp.Status)
	switch followUp.Status {
	case dashboard.SaaSAdminRiskFollowUpStatusPending,
		dashboard.SaaSAdminRiskFollowUpStatusContacted,
		dashboard.SaaSAdminRiskFollowUpStatusRenewalPending,
		dashboard.SaaSAdminRiskFollowUpStatusResolved,
		dashboard.SaaSAdminRiskFollowUpStatusIgnored:
	default:
		return dashboard.SaaSAdminRiskFollowUpResult{}, dashboard.NewSaaSAdminBadRequest("invalid risk follow-up status")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminRiskFollowUpResult{}, err
	}
	defer rollbackQuietly(tx)

	var tenantName string
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(name, '')
		FROM mc_tenant
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, followUp.TenantID).Scan(&tenantName)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminRiskFollowUpResult{}, dashboard.NewSaaSAdminNotFound("tenant not found")
	}
	if err != nil {
		return dashboard.SaaSAdminRiskFollowUpResult{}, err
	}

	owner := strings.TrimSpace(followUp.Owner)
	nextFollowUpAt := strings.TrimSpace(followUp.NextFollowUpAt)
	remark := strings.TrimSpace(followUp.Remark)
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID:      followUp.TenantID,
		ActorUserID:   followUp.ActorUserID,
		ActorTenantID: followUp.ActorTenantID,
		Action:        "tenant.risk.follow_up",
		TargetType:    "tenant",
		TargetID:      strconv.Itoa(followUp.TenantID),
		TargetName:    tenantName,
		AfterJSON: saasAdminMarshalJSON(map[string]any{
			"status":         followUp.Status,
			"owner":          owner,
			"nextFollowUpAt": nextFollowUpAt,
			"remark":         remark,
		}),
		Remark: remark,
	})
	if err != nil {
		return dashboard.SaaSAdminRiskFollowUpResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminRiskFollowUpResult{}, err
	}
	return dashboard.SaaSAdminRiskFollowUpResult{
		TenantID:       followUp.TenantID,
		TenantName:     tenantName,
		Status:         followUp.Status,
		Owner:          owner,
		NextFollowUpAt: nextFollowUpAt,
		Remark:         remark,
		OperationID:    operationID,
	}, nil
}

func (s *MySQLStore) UpsertSaaSAdminPackage(ctx context.Context, update dashboard.SaaSAdminPackageUpsert) (dashboard.SaaSAdminPackage, error) {
	update.Code = strings.TrimSpace(update.Code)
	update.Name = strings.TrimSpace(update.Name)
	update.Description = strings.TrimSpace(update.Description)
	if update.Code == "" {
		return dashboard.SaaSAdminPackage{}, dashboard.NewSaaSAdminBadRequest("code required")
	}
	if update.Name == "" {
		return dashboard.SaaSAdminPackage{}, dashboard.NewSaaSAdminBadRequest("name required")
	}
	if update.Status != 1 && update.Status != 2 {
		return dashboard.SaaSAdminPackage{}, dashboard.NewSaaSAdminBadRequest("status must be 1 or 2")
	}
	if update.ExpectedVersion < 0 {
		return dashboard.SaaSAdminPackage{}, dashboard.NewSaaSAdminBadRequest("expectedVersion must not be negative")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminPackage{}, err
	}
	defer rollbackQuietly(tx)

	before, err := saasAdminPackageByCodeTx(ctx, tx, update.Code)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminPackage{}, err
	}
	existing := err == nil
	if existing {
		if before.Version != update.ExpectedVersion {
			return dashboard.SaaSAdminPackage{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "套餐版本已变化，请刷新后重试"}
		}
	} else if update.ExpectedVersion != 0 {
		return dashboard.SaaSAdminPackage{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "套餐状态已变化，请刷新后重试"}
	}
	beforeJSON := ""
	if existing {
		beforeJSON = saasAdminMarshalJSON(before)
	}

	values := []any{
		update.Name,
		update.Description,
		update.Limits.MaxCorps,
		update.Limits.MaxUsers,
		update.Limits.MaxContacts,
		update.Limits.MaxRooms,
		update.Limits.MaxAgents,
		update.Limits.ChannelCodes,
		update.Limits.ShopCodes,
		update.Limits.Radars,
		update.Limits.Lotteries,
		update.Limits.RoomInfinitePulls,
		update.Limits.RoomFissions,
		update.Limits.RoomClockIns,
		update.Limits.RoomQualities,
		update.Limits.RoomCalendars,
		update.Limits.RoomReminds,
		update.Limits.ContactSOPs,
		update.Limits.RoomSOPs,
		update.Limits.SensitiveWords,
		update.Limits.StorageMB,
		update.Limits.ContactMessageBatches,
		update.Limits.RoomMessageBatches,
		update.Limits.RoomTagPulls,
		update.Limits.WorkRoomAutoPulls,
		update.Limits.WorkFissions,
		update.Limits.OfficialAccounts,
		update.Limits.AsyncExecutions,
		update.Status,
	}
	if existing {
		result, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_packages
			SET name = ?,
				description = ?,
				max_corps = ?,
				max_users = ?,
				max_contacts = ?,
				max_rooms = ?,
				max_agents = ?,
				channel_codes = ?,
				shop_codes = ?,
				radars = ?,
				lotteries = ?,
				room_infinite_pulls = ?,
				room_fissions = ?,
				room_clock_ins = ?,
				room_qualities = ?,
				room_calendars = ?,
				room_reminds = ?,
				contact_sops = ?,
				room_sops = ?,
				sensitive_words = ?,
				storage_mb = ?,
				contact_message_batches = ?,
				room_message_batches = ?,
				room_tag_pulls = ?,
				work_room_auto_pulls = ?,
				work_fissions = ?,
				official_accounts = ?,
				async_executions = ?,
				status = ?,
				version = version + 1,
				updated_at = NOW(),
				deleted_at = NULL
			WHERE code = ? AND version = ? AND deleted_at IS NULL
		`, append(values, update.Code, update.ExpectedVersion)...)
		if err != nil {
			return dashboard.SaaSAdminPackage{}, err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return dashboard.SaaSAdminPackage{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "套餐版本已变化，请刷新后重试"}
		}
	} else {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_packages
				(code, name, description, max_corps, max_users, max_contacts, max_rooms, max_agents, channel_codes, shop_codes, radars, lotteries, room_infinite_pulls, room_fissions, room_clock_ins, room_qualities, room_calendars, room_reminds, contact_sops, room_sops, sensitive_words, storage_mb, contact_message_batches, room_message_batches, room_tag_pulls, work_room_auto_pulls, work_fissions, official_accounts, async_executions, status, version, created_at, updated_at, deleted_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, NOW(), NOW(), NULL)
		`, append([]any{update.Code}, values...)...)
		if isMySQLDuplicateKeyError(err) {
			return dashboard.SaaSAdminPackage{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "套餐状态已变化，请刷新后重试"}
		}
		if err != nil {
			return dashboard.SaaSAdminPackage{}, err
		}
	}
	pkg, err := saasAdminPackageByCodeTx(ctx, tx, update.Code)
	if err != nil {
		return dashboard.SaaSAdminPackage{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		ActorUserID:   update.ActorUserID,
		ActorTenantID: update.ActorTenantID,
		Action:        "package.upsert",
		TargetType:    "package",
		TargetID:      pkg.Code,
		TargetName:    pkg.Name,
		BeforeJSON:    beforeJSON,
		AfterJSON:     saasAdminMarshalJSON(pkg),
		Remark:        "保存套餐",
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminPackage{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, update.ApprovalExecutionID, update.ApprovalExecutionVersion, update.ActorUserID, operationID); err != nil {
		return dashboard.SaaSAdminPackage{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminPackage{}, err
	}
	return pkg, nil
}

func (s *MySQLStore) UpdateSaaSAdminTenantStatus(ctx context.Context, update dashboard.SaaSAdminTenantStatusUpdate) (dashboard.SaaSAdminTenantStatusUpdateResult, error) {
	if update.TenantID <= 0 {
		return dashboard.SaaSAdminTenantStatusUpdateResult{}, dashboard.NewSaaSAdminBadRequest("tenantId required")
	}
	if update.Status != 1 && update.Status != 2 {
		return dashboard.SaaSAdminTenantStatusUpdateResult{}, dashboard.NewSaaSAdminBadRequest("status must be 1 or 2")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminTenantStatusUpdateResult{}, err
	}
	defer rollbackQuietly(tx)

	var tenantName string
	var previousStatus int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(name, ''), COALESCE(status, 0)
		FROM mc_tenant
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, update.TenantID).Scan(&tenantName, &previousStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminTenantStatusUpdateResult{}, dashboard.NewSaaSAdminNotFound("tenant not found")
	}
	if err != nil {
		return dashboard.SaaSAdminTenantStatusUpdateResult{}, err
	}
	if update.ExpectedStatus > 0 && previousStatus != update.ExpectedStatus {
		return dashboard.SaaSAdminTenantStatusUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户状态已变化，请刷新后重新申请审批"}
	}
	if plan := update.ApprovalPlan; plan != nil {
		if plan.SchemaVersion != dashboard.SaaSAdminTenantStatusApprovalPlanSchemaVersion ||
			plan.Snapshot.TenantID != update.TenantID || plan.Snapshot.TenantStatus != previousStatus ||
			plan.Snapshot.TenantName != tenantName {
			return dashboard.SaaSAdminTenantStatusUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户状态审批快照已变化，请重新申请审批"}
		}
		var subscriptionID int64
		var subscriptionStatus string
		var subscriptionVersion int
		subscriptionErr := tx.QueryRowContext(ctx, `
			SELECT id, status, version
			FROM mochat_go_saas_subscriptions
			WHERE tenant_id = ? AND deleted_at IS NULL
			LIMIT 1
			FOR UPDATE
		`, update.TenantID).Scan(&subscriptionID, &subscriptionStatus, &subscriptionVersion)
		if errors.Is(subscriptionErr, sql.ErrNoRows) || isMissingSaaSTableError(subscriptionErr) {
			if plan.Snapshot.SubscriptionPresent {
				return dashboard.SaaSAdminTenantStatusUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户订阅已变化，请重新申请审批"}
			}
		} else if subscriptionErr != nil {
			return dashboard.SaaSAdminTenantStatusUpdateResult{}, subscriptionErr
		} else if !plan.Snapshot.SubscriptionPresent || plan.Snapshot.SubscriptionID != subscriptionID ||
			plan.Snapshot.SubscriptionStatus != subscriptionStatus || plan.Snapshot.SubscriptionVersion != subscriptionVersion {
			return dashboard.SaaSAdminTenantStatusUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户订阅已变化，请重新申请审批"}
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_tenant
		SET status = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, update.Status, update.TenantID); err != nil {
		return dashboard.SaaSAdminTenantStatusUpdateResult{}, err
	}
	if err := syncSaaSSubscriptionTenantStatusTx(ctx, tx, update.TenantID, update.Status, update.ActorUserID, update.ActorTenantID, update.Remark); err != nil {
		return dashboard.SaaSAdminTenantStatusUpdateResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID:      update.TenantID,
		ActorUserID:   update.ActorUserID,
		ActorTenantID: update.ActorTenantID,
		Action:        "tenant.status",
		TargetType:    "tenant",
		TargetID:      strconv.Itoa(update.TenantID),
		TargetName:    tenantName,
		BeforeJSON:    saasAdminMarshalJSON(map[string]any{"status": previousStatus}),
		AfterJSON:     saasAdminMarshalJSON(map[string]any{"status": update.Status}),
		Remark:        strings.TrimSpace(update.Remark),
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminTenantStatusUpdateResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, update.ApprovalExecutionID, update.ApprovalExecutionVersion, update.ActorUserID, operationID); err != nil {
		return dashboard.SaaSAdminTenantStatusUpdateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminTenantStatusUpdateResult{}, err
	}
	return dashboard.SaaSAdminTenantStatusUpdateResult{
		TenantID:       update.TenantID,
		TenantName:     tenantName,
		PreviousStatus: previousStatus,
		Status:         update.Status,
		Remark:         strings.TrimSpace(update.Remark),
		OperationID:    operationID,
	}, nil
}

func (s *MySQLStore) UpdateSaaSAdminTenantPackage(ctx context.Context, update dashboard.SaaSAdminTenantPackageUpdate) (dashboard.SaaSAdminTenantPackageUpdateResult, error) {
	if update.TenantID <= 0 {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, dashboard.NewSaaSAdminBadRequest("tenantId required")
	}
	update.PackageCode = strings.TrimSpace(update.PackageCode)
	if update.PackageCode == "" {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, dashboard.NewSaaSAdminBadRequest("packageCode required")
	}
	if update.ExpectedVersion < 0 {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, dashboard.NewSaaSAdminBadRequest("expectedVersion must not be negative")
	}
	if update.ExpectedPackageVersion <= 0 {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, dashboard.NewSaaSAdminBadRequest("expectedPackageVersion required")
	}
	if update.ExpectedTenantStatus <= 0 {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, dashboard.NewSaaSAdminBadRequest("expectedTenantStatus required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, err
	}
	defer rollbackQuietly(tx)

	var tenantName string
	var tenantStatus int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(name, ''), COALESCE(status, 0)
		FROM mc_tenant
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, update.TenantID).Scan(&tenantName, &tenantStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, dashboard.NewSaaSAdminNotFound("tenant not found")
	}
	if err != nil {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, err
	}
	if tenantStatus != update.ExpectedTenantStatus {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户状态已变化，请刷新后重试"}
	}

	beforeJSON := ""
	var beforePackageCode string
	var beforePackageName string
	var beforeExpiresAt string
	var beforeStatus int
	var beforeVersion int
	var beforeLimitsJSON string
	var deletedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT
			COALESCE(package_code, ''),
			COALESCE(package_name, ''),
			COALESCE(DATE_FORMAT(expires_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(status, 0),
			COALESCE(version, 0),
			COALESCE(CAST(limits_json AS CHAR), '{}'),
			deleted_at
		FROM mochat_go_saas_tenant_packages
		WHERE tenant_id = ?
		LIMIT 1
		FOR UPDATE
	`, update.TenantID).Scan(
		&beforePackageCode,
		&beforePackageName,
		&beforeExpiresAt,
		&beforeStatus,
		&beforeVersion,
		&beforeLimitsJSON,
		&deletedAt,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, err
	}
	rowExists := err == nil
	activeAssignment := rowExists && !deletedAt.Valid
	if activeAssignment {
		if beforeVersion != update.ExpectedVersion {
			return dashboard.SaaSAdminTenantPackageUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户套餐版本已变化，请刷新后重试"}
		}
		var beforeLimits dashboard.SaaSAdminPackageLimits
		if err := json.Unmarshal([]byte(beforeLimitsJSON), &beforeLimits); err != nil {
			return dashboard.SaaSAdminTenantPackageUpdateResult{}, err
		}
		beforeJSON = saasAdminMarshalJSON(map[string]any{
			"packageCode": beforePackageCode,
			"packageName": beforePackageName,
			"expiresAt":   beforeExpiresAt,
			"status":      beforeStatus,
			"version":     beforeVersion,
			"limits":      beforeLimits,
		})
	} else if update.ExpectedVersion != 0 {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户套餐状态已变化，请刷新后重试"}
	}

	pkg, err := saasAdminPackageByCodeTx(ctx, tx, update.PackageCode)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, dashboard.NewSaaSAdminNotFound("package not found")
	}
	if err != nil {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, err
	}
	if pkg.Status != 1 {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, dashboard.NewSaaSAdminBadRequest("package disabled")
	}
	if pkg.Version != update.ExpectedPackageVersion {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "目标套餐版本已变化，请刷新后重试"}
	}
	limitsJSON, err := json.Marshal(pkg.Limits)
	if err != nil {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, err
	}
	var expiresValue any
	if strings.TrimSpace(update.ExpiresAt) != "" {
		expiresValue = strings.TrimSpace(update.ExpiresAt)
	}
	newVersion := 1
	switch {
	case activeAssignment:
		result, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_tenant_packages
			SET package_code = ?,
				package_name = ?,
				expires_at = ?,
				status = 1,
				version = version + 1,
				limits_json = ?,
				updated_at = NOW()
			WHERE tenant_id = ? AND version = ? AND deleted_at IS NULL
		`, pkg.Code, pkg.Name, expiresValue, string(limitsJSON), update.TenantID, update.ExpectedVersion)
		if err != nil {
			return dashboard.SaaSAdminTenantPackageUpdateResult{}, err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return dashboard.SaaSAdminTenantPackageUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户套餐版本已变化，请刷新后重试"}
		}
		newVersion = beforeVersion + 1
	case rowExists:
		result, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_tenant_packages
			SET package_code = ?,
				package_name = ?,
				starts_at = NOW(),
				expires_at = ?,
				status = 1,
				version = version + 1,
				limits_json = ?,
				updated_at = NOW(),
				deleted_at = NULL
			WHERE tenant_id = ? AND version = ? AND deleted_at IS NOT NULL
		`, pkg.Code, pkg.Name, expiresValue, string(limitsJSON), update.TenantID, beforeVersion)
		if err != nil {
			return dashboard.SaaSAdminTenantPackageUpdateResult{}, err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return dashboard.SaaSAdminTenantPackageUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户套餐状态已变化，请刷新后重试"}
		}
		newVersion = beforeVersion + 1
	default:
		_, err := tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_tenant_packages
				(tenant_id, package_code, package_name, starts_at, expires_at, status, version, limits_json, created_at, updated_at, deleted_at)
			VALUES (?, ?, ?, NOW(), ?, 1, 1, ?, NOW(), NOW(), NULL)
		`, update.TenantID, pkg.Code, pkg.Name, expiresValue, string(limitsJSON))
		if isMySQLDuplicateKeyError(err) {
			return dashboard.SaaSAdminTenantPackageUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户套餐状态已变化，请刷新后重试"}
		}
		if err != nil {
			return dashboard.SaaSAdminTenantPackageUpdateResult{}, err
		}
	}
	if err := syncSaaSSubscriptionPackageTx(
		ctx, tx, update.TenantID, pkg.Code, pkg.Name, strings.TrimSpace(update.ExpiresAt), "", 0,
		"package_changed", "tenant_package", "", false, update.ActorUserID, update.ActorTenantID,
		strings.TrimSpace(update.Remark),
	); err != nil {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, err
	}
	remark := strings.TrimSpace(update.Remark)
	if remark == "" {
		remark = "调整租户套餐"
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID:      update.TenantID,
		ActorUserID:   update.ActorUserID,
		ActorTenantID: update.ActorTenantID,
		Action:        dashboard.SaaSAdminApprovalActionTenantPackageUpdate,
		TargetType:    "saas_tenant_package",
		TargetID:      strconv.Itoa(update.TenantID),
		TargetName:    tenantName,
		BeforeJSON:    beforeJSON,
		AfterJSON: saasAdminMarshalJSON(map[string]any{
			"packageCode":    pkg.Code,
			"packageName":    pkg.Name,
			"expiresAt":      update.ExpiresAt,
			"status":         1,
			"version":        newVersion,
			"packageVersion": pkg.Version,
			"limits":         pkg.Limits,
		}),
		Remark: remark,
	})
	if err != nil {
		if !isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminTenantPackageUpdateResult{}, err
		}
		operationID = 0
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, update.ApprovalExecutionID, update.ApprovalExecutionVersion, update.ActorUserID, operationID); err != nil {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminTenantPackageUpdateResult{}, err
	}

	result := dashboard.SaaSAdminTenantPackageUpdateResult{
		TenantID:    update.TenantID,
		TenantName:  tenantName,
		PackageCode: pkg.Code,
		PackageName: pkg.Name,
		ExpiresAt:   update.ExpiresAt,
		Status:      1,
		Version:     newVersion,
		OperationID: operationID,
	}
	refresh, err := s.RefreshSaaSUsageCounters(ctx, update.TenantID)
	if err != nil {
		result.MetricsRefreshPending = true
		result.MetricsRefreshError = truncateRunes(err.Error(), 255)
		return result, nil
	}
	result.MetricsRefreshed = refresh.MetricsRefreshed
	return result, nil
}

func (s *MySQLStore) RenewSaaSAdminTenant(ctx context.Context, renewal dashboard.SaaSAdminTenantRenewal) (dashboard.SaaSAdminTenantRenewalResult, error) {
	if renewal.TenantID <= 0 {
		return dashboard.SaaSAdminTenantRenewalResult{}, dashboard.NewSaaSAdminBadRequest("tenantId required")
	}
	if strings.TrimSpace(renewal.ExpiresAt) == "" {
		return dashboard.SaaSAdminTenantRenewalResult{}, dashboard.NewSaaSAdminBadRequest("expiresAt required")
	}
	if renewal.AmountCents < 0 {
		return dashboard.SaaSAdminTenantRenewalResult{}, dashboard.NewSaaSAdminBadRequest("amountCents must not be negative")
	}
	renewal.Currency = strings.ToUpper(strings.TrimSpace(renewal.Currency))
	if renewal.Currency == "" {
		renewal.Currency = "CNY"
	}
	if renewal.ExpectedPackageAssignmentVersion < 0 || renewal.ExpectedPackageVersion < 0 ||
		renewal.ExpectedTenantStatus < 0 || renewal.ExpectedSubscriptionVersion < 0 ||
		renewal.ExpectedTaskVersion < 0 {
		return dashboard.SaaSAdminTenantRenewalResult{}, dashboard.NewSaaSAdminBadRequest("expected version must not be negative")
	}
	approvedExecution := renewal.ApprovalExecutionID > 0
	stateReferencesProvided := approvedExecution || renewal.ExpectedPackageVersion > 0 || renewal.ExpectedTenantStatus > 0
	if approvedExecution && renewal.ApprovalExecutionVersion <= 0 {
		return dashboard.SaaSAdminTenantRenewalResult{}, dashboard.NewSaaSAdminBadRequest("approved renewal execution reference invalid")
	}
	if stateReferencesProvided {
		if renewal.ExpectedPackageVersion <= 0 || renewal.ExpectedTenantStatus <= 0 ||
			(renewal.ExpectedSubscriptionExists && renewal.ExpectedSubscriptionVersion <= 0) ||
			(!renewal.ExpectedSubscriptionExists && renewal.ExpectedSubscriptionVersion != 0) {
			return dashboard.SaaSAdminTenantRenewalResult{}, dashboard.NewSaaSAdminBadRequest("renewal execution state reference invalid")
		}
	}
	if renewal.TaskID > 0 && (renewal.ExpectedTaskVersion <= 0 || len(strings.TrimSpace(renewal.ExpectedTaskRequestSHA256)) != 64) {
		return dashboard.SaaSAdminTenantRenewalResult{}, dashboard.NewSaaSAdminBadRequest("renewal task execution reference invalid")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminTenantRenewalResult{}, err
	}
	defer rollbackQuietly(tx)

	var tenantName string
	var tenantStatus int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(name, ''), COALESCE(status, 0)
		FROM mc_tenant
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, renewal.TenantID).Scan(&tenantName, &tenantStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminTenantRenewalResult{}, dashboard.NewSaaSAdminNotFound("tenant not found")
	}
	if err != nil {
		return dashboard.SaaSAdminTenantRenewalResult{}, err
	}
	if stateReferencesProvided && tenantStatus != renewal.ExpectedTenantStatus {
		return dashboard.SaaSAdminTenantRenewalResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户状态已变化，请刷新后重新申请审批"}
	}

	var beforePackageCode string
	var beforePackageName string
	var previousExpiresAt string
	var beforeStatus int
	var beforeVersion int
	var beforeLimitsJSON string
	var deletedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT
			COALESCE(package_code, ''),
			COALESCE(package_name, ''),
			COALESCE(DATE_FORMAT(expires_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(status, 0),
			COALESCE(version, 0),
			COALESCE(CAST(limits_json AS CHAR), '{}'),
			deleted_at
		FROM mochat_go_saas_tenant_packages
		WHERE tenant_id = ?
		LIMIT 1
		FOR UPDATE
	`, renewal.TenantID).Scan(
		&beforePackageCode,
		&beforePackageName,
		&previousExpiresAt,
		&beforeStatus,
		&beforeVersion,
		&beforeLimitsJSON,
		&deletedAt,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminTenantRenewalResult{}, err
	}
	rowExists := err == nil
	activeAssignment := rowExists && !deletedAt.Valid
	activeAssignmentVersion := 0
	if activeAssignment {
		activeAssignmentVersion = beforeVersion
	}
	if stateReferencesProvided && activeAssignmentVersion != renewal.ExpectedPackageAssignmentVersion {
		return dashboard.SaaSAdminTenantRenewalResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户套餐版本已变化，请刷新后重新申请审批"}
	}
	if strings.TrimSpace(renewal.PackageCode) == "" {
		if !activeAssignment || beforePackageCode == "" {
			return dashboard.SaaSAdminTenantRenewalResult{}, dashboard.NewSaaSAdminBadRequest("packageCode required")
		}
		renewal.PackageCode = beforePackageCode
	}

	pkg, err := saasAdminPackageByCodeTx(ctx, tx, strings.TrimSpace(renewal.PackageCode))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminTenantRenewalResult{}, dashboard.NewSaaSAdminNotFound("package not found")
	}
	if err != nil {
		return dashboard.SaaSAdminTenantRenewalResult{}, err
	}
	if pkg.Status != 1 {
		return dashboard.SaaSAdminTenantRenewalResult{}, dashboard.NewSaaSAdminBadRequest("package disabled")
	}
	if stateReferencesProvided && pkg.Version != renewal.ExpectedPackageVersion {
		return dashboard.SaaSAdminTenantRenewalResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "目标套餐版本已变化，请刷新后重新申请审批"}
	}

	currentSubscription, subscriptionErr := saasAdminSubscriptionByTenantTx(ctx, tx, renewal.TenantID)
	subscriptionExists := subscriptionErr == nil
	if subscriptionErr != nil && !errors.Is(subscriptionErr, sql.ErrNoRows) {
		return dashboard.SaaSAdminTenantRenewalResult{}, subscriptionErr
	}
	if stateReferencesProvided {
		if subscriptionExists != renewal.ExpectedSubscriptionExists {
			return dashboard.SaaSAdminTenantRenewalResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户订阅状态已变化，请刷新后重新申请审批"}
		}
		if subscriptionExists && currentSubscription.Version != renewal.ExpectedSubscriptionVersion {
			return dashboard.SaaSAdminTenantRenewalResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户订阅版本已变化，请刷新后重新申请审批"}
		}
	}

	taskType := ""
	taskStatus := ""
	taskVersion := 0
	taskRequestJSON := ""
	taskRemark := ""
	if renewal.TaskID > 0 {
		err := tx.QueryRowContext(ctx, `
			SELECT task_type, status, version, COALESCE(CAST(request_json AS CHAR), ''), remark
			FROM mochat_go_saas_admin_tasks
			WHERE id = ? AND deleted_at IS NULL
			LIMIT 1
			FOR UPDATE
		`, renewal.TaskID).Scan(&taskType, &taskStatus, &taskVersion, &taskRequestJSON, &taskRemark)
		if errors.Is(err, sql.ErrNoRows) {
			return dashboard.SaaSAdminTenantRenewalResult{}, dashboard.NewSaaSAdminNotFound("task not found")
		}
		if err != nil {
			return dashboard.SaaSAdminTenantRenewalResult{}, err
		}
		if taskType != dashboard.SaaSAdminTaskTypeTenantRenewal {
			return dashboard.SaaSAdminTenantRenewalResult{}, dashboard.NewSaaSAdminBadRequest("task type unsupported")
		}
		if taskVersion != renewal.ExpectedTaskVersion {
			return dashboard.SaaSAdminTenantRenewalResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "任务版本已变化，请刷新后重新申请审批"}
		}
		if taskStatus == dashboard.SaaSAdminTaskStatusApplied || taskStatus == dashboard.SaaSAdminTaskStatusCanceled {
			return dashboard.SaaSAdminTenantRenewalResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "任务状态不允许执行续费审批"}
		}
		taskDigest := sha256.Sum256([]byte(taskRequestJSON))
		if fmt.Sprintf("%x", taskDigest) != strings.ToLower(strings.TrimSpace(renewal.ExpectedTaskRequestSHA256)) {
			return dashboard.SaaSAdminTenantRenewalResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "任务请求快照已变化，请刷新后重新申请审批"}
		}
	}

	limitsJSON, err := json.Marshal(pkg.Limits)
	if err != nil {
		return dashboard.SaaSAdminTenantRenewalResult{}, err
	}
	newPackageAssignmentVersion := 1
	switch {
	case activeAssignment:
		updateResult, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_tenant_packages
			SET package_code = ?,
				package_name = ?,
				expires_at = ?,
				status = 1,
				version = version + 1,
				limits_json = ?,
				updated_at = NOW()
			WHERE tenant_id = ? AND version = ? AND deleted_at IS NULL
		`, pkg.Code, pkg.Name, strings.TrimSpace(renewal.ExpiresAt), string(limitsJSON), renewal.TenantID, beforeVersion)
		if err != nil {
			return dashboard.SaaSAdminTenantRenewalResult{}, err
		}
		if affected, _ := updateResult.RowsAffected(); affected != 1 {
			return dashboard.SaaSAdminTenantRenewalResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户套餐版本已变化，续费事务未提交"}
		}
		newPackageAssignmentVersion = beforeVersion + 1
	case rowExists:
		updateResult, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_tenant_packages
			SET package_code = ?,
				package_name = ?,
				starts_at = NOW(),
				expires_at = ?,
				status = 1,
				version = version + 1,
				limits_json = ?,
				updated_at = NOW(),
				deleted_at = NULL
			WHERE tenant_id = ? AND version = ? AND deleted_at IS NOT NULL
		`, pkg.Code, pkg.Name, strings.TrimSpace(renewal.ExpiresAt), string(limitsJSON), renewal.TenantID, beforeVersion)
		if err != nil {
			return dashboard.SaaSAdminTenantRenewalResult{}, err
		}
		if affected, _ := updateResult.RowsAffected(); affected != 1 {
			return dashboard.SaaSAdminTenantRenewalResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户套餐状态已变化，续费事务未提交"}
		}
		newPackageAssignmentVersion = beforeVersion + 1
	default:
		_, err := tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_tenant_packages
				(tenant_id, package_code, package_name, starts_at, expires_at, status, version, limits_json, created_at, updated_at, deleted_at)
			VALUES (?, ?, ?, NOW(), ?, 1, 1, ?, NOW(), NOW(), NULL)
		`, renewal.TenantID, pkg.Code, pkg.Name, strings.TrimSpace(renewal.ExpiresAt), string(limitsJSON))
		if isMySQLDuplicateKeyError(err) {
			return dashboard.SaaSAdminTenantRenewalResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户套餐状态已变化，续费事务未提交"}
		}
		if err != nil {
			return dashboard.SaaSAdminTenantRenewalResult{}, err
		}
	}

	var previousExpiresValue any
	if activeAssignment && previousExpiresAt != "" {
		previousExpiresValue = previousExpiresAt
	}
	var paidAtValue any
	if strings.TrimSpace(renewal.PaidAt) != "" {
		paidAtValue = strings.TrimSpace(renewal.PaidAt)
	}
	metadata := map[string]any{
		"source":              "saas_admin",
		"previousPackageCode": beforePackageCode,
		"previousPackageName": beforePackageName,
		"previousStatus":      beforeStatus,
		"previousVersion":     activeAssignmentVersion,
		"newVersion":          newPackageAssignmentVersion,
		"approvalId":          renewal.ApprovalExecutionID,
		"taskId":              renewal.TaskID,
	}
	metadataJSON := saasAdminMarshalJSON(metadata)
	billingResult, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_billing_events
			(tenant_id, event_type, package_code, package_name, previous_expires_at, new_expires_at, amount_cents, currency, paid_at, payment_method, external_order_no, actor_user_id, actor_tenant_id, remark, metadata_json, created_at, updated_at, deleted_at)
		VALUES (?, 'renewal', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NULL)
	`, renewal.TenantID,
		pkg.Code,
		pkg.Name,
		previousExpiresValue,
		strings.TrimSpace(renewal.ExpiresAt),
		renewal.AmountCents,
		renewal.Currency,
		paidAtValue,
		truncateRunes(strings.TrimSpace(renewal.PaymentMethod), 64),
		truncateRunes(strings.TrimSpace(renewal.ExternalOrderNo), 128),
		renewal.ActorUserID,
		renewal.ActorTenantID,
		truncateRunes(strings.TrimSpace(renewal.Remark), 255),
		saasAdminJSONValue(metadataJSON),
	)
	if err != nil {
		return dashboard.SaaSAdminTenantRenewalResult{}, err
	}
	billingEventID, _ := billingResult.LastInsertId()
	if err := syncSaaSSubscriptionPackageTx(
		ctx, tx, renewal.TenantID, pkg.Code, pkg.Name, strings.TrimSpace(renewal.ExpiresAt), "", billingEventID,
		"renewed", "billing", "billing:"+strconv.FormatInt(billingEventID, 10), true,
		renewal.ActorUserID, renewal.ActorTenantID, strings.TrimSpace(renewal.Remark),
	); err != nil {
		return dashboard.SaaSAdminTenantRenewalResult{}, err
	}

	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID:      renewal.TenantID,
		ActorUserID:   renewal.ActorUserID,
		ActorTenantID: renewal.ActorTenantID,
		Action:        dashboard.SaaSAdminApprovalActionTenantRenewal,
		TargetType:    "tenant",
		TargetID:      strconv.Itoa(renewal.TenantID),
		TargetName:    tenantName,
		BeforeJSON: saasAdminMarshalJSON(map[string]any{
			"packageCode": beforePackageCode,
			"packageName": beforePackageName,
			"expiresAt":   previousExpiresAt,
			"status":      beforeStatus,
			"version":     activeAssignmentVersion,
		}),
		AfterJSON: saasAdminMarshalJSON(map[string]any{
			"packageCode":     pkg.Code,
			"packageName":     pkg.Name,
			"expiresAt":       renewal.ExpiresAt,
			"version":         newPackageAssignmentVersion,
			"amountCents":     renewal.AmountCents,
			"currency":        renewal.Currency,
			"billingEventId":  billingEventID,
			"externalOrderNo": renewal.ExternalOrderNo,
			"taskId":          renewal.TaskID,
			"approvalId":      renewal.ApprovalExecutionID,
		}),
		Remark: strings.TrimSpace(renewal.Remark),
	})
	if err != nil {
		if !isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminTenantRenewalResult{}, err
		}
		operationID = 0
	}
	result := dashboard.SaaSAdminTenantRenewalResult{
		TenantID:          renewal.TenantID,
		TenantName:        tenantName,
		PackageCode:       pkg.Code,
		PackageName:       pkg.Name,
		PreviousExpiresAt: previousExpiresAt,
		ExpiresAt:         strings.TrimSpace(renewal.ExpiresAt),
		AmountCents:       renewal.AmountCents,
		Currency:          renewal.Currency,
		BillingEventID:    billingEventID,
		OperationID:       operationID,
	}
	if renewal.TaskID > 0 {
		taskResultJSON := saasAdminMarshalJSON(saasAdminTenantRenewalResultPayloadForStore(result))
		updateResult, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_admin_tasks
			SET status = ?, tenant_id = ?, package_code = ?, result_json = ?, last_error = '',
				applied_at = NOW(), version = version + 1, updated_at = NOW()
			WHERE id = ? AND version = ? AND deleted_at IS NULL
		`, dashboard.SaaSAdminTaskStatusApplied, renewal.TenantID, pkg.Code, taskResultJSON, renewal.TaskID, renewal.ExpectedTaskVersion)
		if err != nil {
			return dashboard.SaaSAdminTenantRenewalResult{}, err
		}
		if affected, _ := updateResult.RowsAffected(); affected != 1 {
			return dashboard.SaaSAdminTenantRenewalResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "任务版本已变化，续费事务未提交"}
		}
		_, err = insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
			TenantID:      renewal.TenantID,
			ActorUserID:   renewal.ActorUserID,
			ActorTenantID: renewal.ActorTenantID,
			Action:        dashboard.SaaSAdminOperationActionTaskApply,
			TargetType:    dashboard.SaaSAdminOperationTargetAdminTask,
			TargetID:      strconv.FormatInt(renewal.TaskID, 10),
			TargetName:    dashboard.SaaSAdminTaskTypeTenantRenewal,
			BeforeJSON:    saasAdminMarshalJSON(map[string]any{"status": taskStatus, "version": taskVersion}),
			AfterJSON:     saasAdminMarshalJSON(map[string]any{"status": dashboard.SaaSAdminTaskStatusApplied, "version": taskVersion + 1, "tenantId": renewal.TenantID, "operationId": operationID}),
			Remark:        strings.TrimSpace(taskRemark),
		})
		if err != nil && !isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminTenantRenewalResult{}, err
		}
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, renewal.ApprovalExecutionID, renewal.ApprovalExecutionVersion, renewal.ActorUserID, operationID); err != nil {
		return dashboard.SaaSAdminTenantRenewalResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminTenantRenewalResult{}, err
	}

	refresh, err := s.RefreshSaaSUsageCounters(ctx, renewal.TenantID)
	if err != nil {
		result.MetricsRefreshPending = true
		result.MetricsRefreshError = truncateRunes(err.Error(), 255)
		return result, nil
	}
	result.MetricsRefreshed = refresh.MetricsRefreshed
	return result, nil
}

func saasAdminTenantRenewalResultPayloadForStore(result dashboard.SaaSAdminTenantRenewalResult) map[string]any {
	return map[string]any{
		"tenantId":              result.TenantID,
		"tenantName":            result.TenantName,
		"packageCode":           result.PackageCode,
		"packageName":           result.PackageName,
		"previousExpiresAt":     result.PreviousExpiresAt,
		"expiresAt":             result.ExpiresAt,
		"amountCents":           result.AmountCents,
		"currency":              result.Currency,
		"billingEventId":        result.BillingEventID,
		"operationId":           result.OperationID,
		"metricsRefreshed":      result.MetricsRefreshed,
		"metricsRefreshPending": result.MetricsRefreshPending,
		"metricsRefreshError":   result.MetricsRefreshError,
	}
}

func (s *MySQLStore) ProvisionSaaSAdminTenant(ctx context.Context, provision dashboard.SaaSAdminTenantProvision) (dashboard.SaaSAdminTenantProvisionResult, error) {
	provision.TenantName = strings.TrimSpace(provision.TenantName)
	provision.AdminPhone = strings.TrimSpace(provision.AdminPhone)
	provision.AdminName = strings.TrimSpace(provision.AdminName)
	provision.RoleName = strings.TrimSpace(provision.RoleName)
	provision.PackageCode = strings.TrimSpace(provision.PackageCode)
	provision.ConfigCopyMode = strings.ToLower(strings.TrimSpace(provision.ConfigCopyMode))
	if provision.TenantName == "" {
		return dashboard.SaaSAdminTenantProvisionResult{}, dashboard.NewSaaSAdminBadRequest("tenantName required")
	}
	if provision.AdminPhone == "" {
		return dashboard.SaaSAdminTenantProvisionResult{}, dashboard.NewSaaSAdminBadRequest("adminPhone required")
	}
	if provision.AdminName == "" {
		provision.AdminName = "超级管理员"
	}
	if provision.RoleName == "" {
		provision.RoleName = "超级管理员"
	}
	if provision.PackageCode == "" {
		return dashboard.SaaSAdminTenantProvisionResult{}, dashboard.NewSaaSAdminBadRequest("packageCode required")
	}
	if strings.TrimSpace(provision.AdminPasswordHash) == "" {
		return dashboard.SaaSAdminTenantProvisionResult{}, dashboard.NewSaaSAdminBadRequest("admin password hash required")
	}
	if provision.ConfigCopyMode == "" {
		provision.ConfigCopyMode = "missing"
	}
	if provision.ConfigCopyMode != "missing" && provision.ConfigCopyMode != "overwrite" && provision.ConfigCopyMode != "skip" {
		return dashboard.SaaSAdminTenantProvisionResult{}, dashboard.NewSaaSAdminBadRequest("configCopyMode must be missing, overwrite, or skip")
	}
	if provision.ExpectedPackageVersion < 0 || provision.ExpectedTaskVersion < 0 {
		return dashboard.SaaSAdminTenantProvisionResult{}, dashboard.NewSaaSAdminBadRequest("expected version must not be negative")
	}
	if provision.TaskID > 0 {
		if provision.ApprovalExecutionID <= 0 || provision.ExpectedTaskVersion <= 0 || len(strings.TrimSpace(provision.ExpectedTaskRequestSHA256)) != 64 {
			return dashboard.SaaSAdminTenantProvisionResult{}, dashboard.NewSaaSAdminBadRequest("approved task execution reference invalid")
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}
	defer rollbackQuietly(tx)

	pkg, err := saasAdminPackageByCodeTx(ctx, tx, provision.PackageCode)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminTenantProvisionResult{}, dashboard.NewSaaSAdminNotFound("package not found")
	}
	if err != nil {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}
	if pkg.Status != 1 {
		return dashboard.SaaSAdminTenantProvisionResult{}, dashboard.NewSaaSAdminBadRequest("package disabled")
	}
	if provision.ExpectedPackageVersion > 0 && pkg.Version != provision.ExpectedPackageVersion {
		return dashboard.SaaSAdminTenantProvisionResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "目标套餐版本已变化，请刷新后重新申请审批"}
	}

	taskType := ""
	taskStatus := ""
	taskVersion := 0
	taskRequestJSON := ""
	taskRemark := ""
	if provision.TaskID > 0 {
		err := tx.QueryRowContext(ctx, `
			SELECT task_type, status, version, COALESCE(CAST(request_json AS CHAR), ''), remark
			FROM mochat_go_saas_admin_tasks
			WHERE id = ? AND deleted_at IS NULL
			LIMIT 1
			FOR UPDATE
		`, provision.TaskID).Scan(&taskType, &taskStatus, &taskVersion, &taskRequestJSON, &taskRemark)
		if errors.Is(err, sql.ErrNoRows) {
			return dashboard.SaaSAdminTenantProvisionResult{}, dashboard.NewSaaSAdminNotFound("task not found")
		}
		if err != nil {
			return dashboard.SaaSAdminTenantProvisionResult{}, err
		}
		if taskType != dashboard.SaaSAdminTaskTypeTenantProvision {
			return dashboard.SaaSAdminTenantProvisionResult{}, dashboard.NewSaaSAdminBadRequest("task type unsupported")
		}
		if taskVersion != provision.ExpectedTaskVersion {
			return dashboard.SaaSAdminTenantProvisionResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "任务版本已变化，请刷新后重新申请审批"}
		}
		if taskStatus == dashboard.SaaSAdminTaskStatusApplied || taskStatus == dashboard.SaaSAdminTaskStatusCanceled {
			return dashboard.SaaSAdminTenantProvisionResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "任务状态不允许执行开户审批"}
		}
		taskDigest := sha256.Sum256([]byte(taskRequestJSON))
		if fmt.Sprintf("%x", taskDigest) != strings.ToLower(strings.TrimSpace(provision.ExpectedTaskRequestSHA256)) {
			return dashboard.SaaSAdminTenantProvisionResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "任务请求快照已变化，请刷新后重新申请审批"}
		}
	}

	tenantID := provision.TenantID
	if tenantID > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_tenant (id, name, status, logo, login_background, url, copyright, created_at, updated_at, deleted_at)
			VALUES (?, ?, 1, '', '', '', '', NOW(), NOW(), NULL)
			ON DUPLICATE KEY UPDATE
				name = VALUES(name),
				status = 1,
				updated_at = NOW(),
				deleted_at = NULL
		`, tenantID, provision.TenantName); err != nil {
			return dashboard.SaaSAdminTenantProvisionResult{}, err
		}
	} else {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO mc_tenant (name, status, logo, login_background, url, copyright, created_at, updated_at, deleted_at)
			VALUES (?, 1, '', '', '', '', NOW(), NOW(), NULL)
		`, provision.TenantName)
		if err != nil {
			return dashboard.SaaSAdminTenantProvisionResult{}, err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return dashboard.SaaSAdminTenantProvisionResult{}, err
		}
		tenantID = int(id)
	}

	if err := ensureSaaSAdminTenantDefaultCorpTx(ctx, tx, tenantID, provision.TenantName); err != nil {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}
	if err := ensureSaaSAdminPhoneAvailableTx(ctx, tx, provision.AdminPhone, tenantID); err != nil {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}
	adminUserID, err := upsertSaaSAdminTenantAdminUserTx(ctx, tx, tenantID, provision)
	if err != nil {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}
	roleID, err := upsertSaaSAdminTenantAdminRoleTx(ctx, tx, tenantID, provision, adminUserID)
	if err != nil {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}
	menuCount, err := replaceSaaSAdminRoleMenusWithActiveMenusTx(ctx, tx, roleID)
	if err != nil {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}
	if err := upsertSaaSAdminUserRoleTx(ctx, tx, adminUserID, roleID); err != nil {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}
	configCopyCount, err := copySaaSAdminDefaultTenantConfigsTx(ctx, tx, tenantID, provision.ConfigCopyMode)
	if err != nil {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}

	limitsJSON, err := json.Marshal(pkg.Limits)
	if err != nil {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}
	var expiresValue any
	if strings.TrimSpace(provision.ExpiresAt) != "" {
		expiresValue = strings.TrimSpace(provision.ExpiresAt)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_tenant_packages
			(tenant_id, package_code, package_name, starts_at, expires_at, status, limits_json, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, NOW(), ?, 1, ?, NOW(), NOW(), NULL)
		ON DUPLICATE KEY UPDATE
			package_code = VALUES(package_code),
			package_name = VALUES(package_name),
			expires_at = VALUES(expires_at),
			status = 1,
			version = version + 1,
			limits_json = VALUES(limits_json),
			updated_at = NOW(),
			deleted_at = NULL
	`, tenantID, pkg.Code, pkg.Name, expiresValue, string(limitsJSON)); err != nil {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}
	if err := syncSaaSSubscriptionPackageTx(
		ctx, tx, tenantID, pkg.Code, pkg.Name, strings.TrimSpace(provision.ExpiresAt), "", 0,
		"provisioned", "provision", "", true, provision.ActorUserID, provision.ActorTenantID,
		strings.TrimSpace(provision.Remark),
	); err != nil {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}
	if _, err := upsertSaaSAdminSeedVersionsTx(ctx, tx, tenantID, roleID, menuCount); err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}
	if err := recordSaaSAdminProvisionRunTx(ctx, tx, tenantID, pkg.Code, provision.AdminPhone); err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID:      tenantID,
		ActorUserID:   provision.ActorUserID,
		ActorTenantID: provision.ActorTenantID,
		Action:        "tenant.provision",
		TargetType:    "tenant",
		TargetID:      strconv.Itoa(tenantID),
		TargetName:    provision.TenantName,
		AfterJSON: saasAdminMarshalJSON(map[string]any{
			"tenantName":     provision.TenantName,
			"adminUserId":    adminUserID,
			"adminPhone":     provision.AdminPhone,
			"adminName":      provision.AdminName,
			"roleId":         roleID,
			"roleName":       provision.RoleName,
			"packageCode":    pkg.Code,
			"packageName":    pkg.Name,
			"expiresAt":      provision.ExpiresAt,
			"configCopyMode": provision.ConfigCopyMode,
			"menuCount":      menuCount,
		}),
		Remark: strings.TrimSpace(provision.Remark),
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}
	result := dashboard.SaaSAdminTenantProvisionResult{
		TenantID:        tenantID,
		TenantName:      provision.TenantName,
		AdminUserID:     adminUserID,
		AdminPhone:      provision.AdminPhone,
		AdminName:       provision.AdminName,
		RoleID:          roleID,
		RoleName:        provision.RoleName,
		PackageCode:     pkg.Code,
		PackageName:     pkg.Name,
		ExpiresAt:       strings.TrimSpace(provision.ExpiresAt),
		MenuCount:       menuCount,
		ConfigCopyCount: configCopyCount,
		OperationID:     operationID,
	}
	if provision.TaskID > 0 {
		taskResultJSON := saasAdminMarshalJSON(map[string]any{
			"tenantId":         result.TenantID,
			"tenantName":       result.TenantName,
			"adminUserId":      result.AdminUserID,
			"adminPhone":       result.AdminPhone,
			"adminName":        result.AdminName,
			"roleId":           result.RoleID,
			"roleName":         result.RoleName,
			"packageCode":      result.PackageCode,
			"packageName":      result.PackageName,
			"expiresAt":        result.ExpiresAt,
			"menuCount":        result.MenuCount,
			"configCopyCount":  result.ConfigCopyCount,
			"metricsRefreshed": 0,
			"operationId":      result.OperationID,
		})
		updateResult, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_admin_tasks
			SET status = ?, tenant_id = ?, result_json = ?, last_error = '', applied_at = NOW(), version = version + 1, updated_at = NOW()
			WHERE id = ? AND version = ? AND deleted_at IS NULL
		`, dashboard.SaaSAdminTaskStatusApplied, tenantID, taskResultJSON, provision.TaskID, provision.ExpectedTaskVersion)
		if err != nil {
			return dashboard.SaaSAdminTenantProvisionResult{}, err
		}
		if affected, _ := updateResult.RowsAffected(); affected != 1 {
			return dashboard.SaaSAdminTenantProvisionResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "任务版本已变化，业务事务未提交"}
		}
		_, err = insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
			TenantID:      tenantID,
			ActorUserID:   provision.ActorUserID,
			ActorTenantID: provision.ActorTenantID,
			Action:        dashboard.SaaSAdminOperationActionTaskApply,
			TargetType:    dashboard.SaaSAdminOperationTargetAdminTask,
			TargetID:      strconv.FormatInt(provision.TaskID, 10),
			TargetName:    dashboard.SaaSAdminTaskTypeTenantProvision,
			BeforeJSON:    saasAdminMarshalJSON(map[string]any{"status": taskStatus, "version": taskVersion}),
			AfterJSON:     saasAdminMarshalJSON(map[string]any{"status": dashboard.SaaSAdminTaskStatusApplied, "version": taskVersion + 1, "tenantId": tenantID, "operationId": operationID}),
			Remark:        strings.TrimSpace(taskRemark),
		})
		if err != nil && !isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminTenantProvisionResult{}, err
		}
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, provision.ApprovalExecutionID, provision.ApprovalExecutionVersion, provision.ActorUserID, operationID); err != nil {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminTenantProvisionResult{}, err
	}

	refresh, err := s.RefreshSaaSUsageCounters(ctx, tenantID)
	if err != nil {
		result.MetricsRefreshPending = true
		result.MetricsRefreshError = truncateRunes(err.Error(), 255)
		return result, nil
	}
	result.MetricsRefreshed = refresh.MetricsRefreshed
	return result, nil
}

func saasTenantDefaultCorpValues(tenantID int, tenantName string) (string, string) {
	return strings.TrimSpace(tenantName), fmt.Sprintf("fake_tenant_%d", tenantID)
}

func ensureSaaSAdminTenantDefaultCorpTx(ctx context.Context, tx *sql.Tx, tenantID int, tenantName string) error {
	var corpID int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_corp
		WHERE tenant_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, tenantID).Scan(&corpID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	name, wxCorpID := saasTenantDefaultCorpValues(tenantID, tenantName)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mc_corp
			(name, wx_corpid, social_code, employee_secret, event_callback, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
		VALUES (?, ?, '', '', '', '', '', '', ?, NOW(), NOW(), NULL)
	`, name, wxCorpID, tenantID)
	return err
}

func ensureSaaSAdminPhoneAvailableTx(ctx context.Context, tx *sql.Tx, phone string, tenantID int) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, tenant_id
		FROM mc_user
		WHERE phone = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`, phone)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var userID int
		var rowTenantID int
		if err := rows.Scan(&userID, &rowTenantID); err != nil {
			return err
		}
		if rowTenantID != tenantID {
			return dashboard.NewSaaSAdminBadRequest("adminPhone already used by another tenant")
		}
	}
	return rows.Err()
}

func upsertSaaSAdminTenantAdminUserTx(ctx context.Context, tx *sql.Tx, tenantID int, provision dashboard.SaaSAdminTenantProvision) (int, error) {
	var existingID int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_user
		WHERE phone = ? AND tenant_id = ?
		ORDER BY deleted_at IS NULL DESC, id ASC
		LIMIT 1
	`, provision.AdminPhone, tenantID).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if existingID > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_user
			SET password = ?, name = ?, status = 1, tenant_id = ?, isSuperAdmin = 1, updated_at = NOW(), deleted_at = NULL
			WHERE id = ?
		`, provision.AdminPasswordHash, provision.AdminName, tenantID, existingID); err != nil {
			return 0, err
		}
		return existingID, nil
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_user
			(phone, password, name, gender, department, position, login_time, status, tenant_id, created_at, updated_at, deleted_at, isSuperAdmin)
		VALUES (?, ?, ?, 0, '', '', NULL, 1, ?, NOW(), NOW(), NULL, 1)
	`, provision.AdminPhone, provision.AdminPasswordHash, provision.AdminName, tenantID)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func upsertSaaSAdminTenantAdminRoleTx(ctx context.Context, tx *sql.Tx, tenantID int, provision dashboard.SaaSAdminTenantProvision, userID int) (int, error) {
	var roleID int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_rbac_role
		WHERE tenant_id = ? AND name = ?
		ORDER BY deleted_at IS NULL DESC, id ASC
		LIMIT 1
	`, tenantID, provision.RoleName).Scan(&roleID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if roleID > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_rbac_role
			SET status = 1, operate_id = ?, operate_name = ?, data_permission = '[]', updated_at = NOW(), deleted_at = NULL
			WHERE id = ?
		`, userID, provision.AdminName, roleID); err != nil {
			return 0, err
		}
		return roleID, nil
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_rbac_role
			(tenant_id, name, remarks, status, operate_id, operate_name, data_permission, created_at, updated_at, deleted_at)
		VALUES (?, ?, 'saas admin provision full-access role', 1, ?, ?, '[]', NOW(), NOW(), NULL)
	`, tenantID, provision.RoleName, userID, provision.AdminName)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func replaceSaaSAdminRoleMenusWithActiveMenusTx(ctx context.Context, tx *sql.Tx, roleID int) (int, error) {
	if _, err := tx.ExecContext(ctx, `DELETE FROM mc_rbac_role_menu WHERE role_id = ?`, roleID); err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_rbac_role_menu (role_id, menu_id, created_at, updated_at)
		SELECT ?, id, NOW(), NOW()
		FROM mc_rbac_menu
		WHERE deleted_at IS NULL
		ORDER BY id ASC
	`, roleID)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, errors.New("no active RBAC menus found; run mochat-migrate apply before tenant provision")
	}
	return int(count), nil
}

func upsertSaaSAdminUserRoleTx(ctx context.Context, tx *sql.Tx, userID int, roleID int) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_rbac_user_role
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE user_id = ? AND role_id <> ? AND deleted_at IS NULL
	`, userID, roleID); err != nil {
		return err
	}
	var userRoleID int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_rbac_user_role
		WHERE user_id = ? AND role_id = ?
		ORDER BY id ASC
		LIMIT 1
	`, userID, roleID).Scan(&userRoleID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if userRoleID > 0 {
		_, err = tx.ExecContext(ctx, `
			UPDATE mc_rbac_user_role
			SET updated_at = NOW(), deleted_at = NULL
			WHERE id = ?
		`, userRoleID)
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mc_rbac_user_role (user_id, role_id, created_at, updated_at, deleted_at)
		VALUES (?, ?, NOW(), NOW(), NULL)
	`, userID, roleID)
	return err
}

func copySaaSAdminDefaultTenantConfigsTx(ctx context.Context, tx *sql.Tx, tenantID int, mode string) (int, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "missing"
	}
	if mode == "skip" {
		return 0, nil
	}
	if mode == "overwrite" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_system_config_value tenant_value
			JOIN mc_system_config_value src
				ON src.config_id = tenant_value.config_id
				AND src.type = 1
				AND src.target_id = 0
				AND src.deleted_at IS NULL
			SET tenant_value.deleted_at = NOW(), tenant_value.updated_at = NOW()
			WHERE tenant_value.type = 3
				AND tenant_value.target_id = ?
				AND tenant_value.deleted_at IS NULL
		`, tenantID); err != nil {
			return 0, err
		}
	}
	query := `
		INSERT INTO mc_system_config_value
			(type, target_id, config_id, value, description, created_at, updated_at, deleted_at)
		SELECT 3, ?, src.config_id, src.value, src.description, NOW(), NOW(), NULL
		FROM mc_system_config_value src
		WHERE src.type = 1
			AND src.target_id = 0
			AND src.deleted_at IS NULL
	`
	var result sql.Result
	var err error
	if mode == "missing" {
		query += `
			AND NOT EXISTS (
				SELECT 1
				FROM mc_system_config_value tenant_value
				WHERE tenant_value.type = 3
					AND tenant_value.target_id = ?
					AND tenant_value.config_id = src.config_id
					AND tenant_value.deleted_at IS NULL
			)
		`
		result, err = tx.ExecContext(ctx, query, tenantID, tenantID)
	} else {
		result, err = tx.ExecContext(ctx, query, tenantID)
	}
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func upsertSaaSAdminSeedVersionsTx(ctx context.Context, tx *sql.Tx, tenantID int, roleID int, menuCount int) (int, error) {
	checksum := ""
	_ = tx.QueryRowContext(ctx, `
		SELECT checksum
		FROM mochat_go_schema_migrations
		WHERE version = ?
		LIMIT 1
	`, "0002_seed_core_data").Scan(&checksum)
	contactFieldCount, err := saasAdminQueryIntTx(ctx, tx, `SELECT COUNT(*) FROM mc_contact_field WHERE deleted_at IS NULL`)
	if err != nil {
		return 0, err
	}
	chatToolCount, err := saasAdminQueryIntTx(ctx, tx, `SELECT COUNT(*) FROM mc_chat_tool WHERE deleted_at IS NULL`)
	if err != nil {
		return 0, err
	}
	seeds := []struct {
		name     string
		metadata map[string]any
	}{
		{name: "rbac_menu", metadata: map[string]any{"activeMenuCount": menuCount, "roleId": roleID}},
		{name: "contact_field", metadata: map[string]any{"activeFieldCount": contactFieldCount}},
		{name: "chat_tool", metadata: map[string]any{"activeToolCount": chatToolCount}},
	}
	for _, seed := range seeds {
		metadata, err := json.Marshal(seed.metadata)
		if err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mochat_go_seed_versions
				(scope, target_id, seed_name, seed_version, checksum, metadata, applied_at, created_at, updated_at)
			VALUES ('tenant', ?, ?, '0002_seed_core_data', ?, ?, NOW(), NOW(), NOW())
			ON DUPLICATE KEY UPDATE
				seed_version = VALUES(seed_version),
				checksum = VALUES(checksum),
				metadata = VALUES(metadata),
				applied_at = NOW(),
				updated_at = NOW()
		`, tenantID, seed.name, checksum, string(metadata)); err != nil {
			return 0, err
		}
	}
	return len(seeds), nil
}

func recordSaaSAdminProvisionRunTx(ctx context.Context, tx *sql.Tx, tenantID int, packageCode string, adminPhone string) error {
	runKey := fmt.Sprintf("tenant:%d:admin:%s", tenantID, adminPhone)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_tenant_provision_runs
			(run_key, tenant_id, package_code, admin_phone, status, message, started_at, finished_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, 'provisioned by saas admin', NOW(), NOW(), NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			tenant_id = VALUES(tenant_id),
			package_code = VALUES(package_code),
			admin_phone = VALUES(admin_phone),
			status = 1,
			message = 'provisioned by saas admin',
			started_at = NOW(),
			finished_at = NOW(),
			updated_at = NOW()
	`, runKey, tenantID, packageCode, adminPhone)
	return err
}

func saasAdminQueryIntTx(ctx context.Context, tx *sql.Tx, query string, args ...any) (int, error) {
	var value int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return value, err
}

func saasAdminPackageByCodeTx(ctx context.Context, tx *sql.Tx, code string) (dashboard.SaaSAdminPackage, error) {
	var item dashboard.SaaSAdminPackage
	err := tx.QueryRowContext(ctx, `
		SELECT
			id,
			code,
			name,
			description,
			status,
			version,
			max_corps,
			max_users,
			max_contacts,
			max_rooms,
			max_agents,
			channel_codes,
			shop_codes,
			radars,
			lotteries,
			room_infinite_pulls,
			room_fissions,
			room_clock_ins,
			room_qualities,
			room_calendars,
			room_reminds,
			contact_sops,
			room_sops,
			sensitive_words,
			storage_mb,
			contact_message_batches,
			room_message_batches,
			room_tag_pulls,
			work_room_auto_pulls,
			work_fissions,
			official_accounts,
			async_executions
		FROM mochat_go_saas_packages
		WHERE code = ? AND deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, code).Scan(
		&item.ID,
		&item.Code,
		&item.Name,
		&item.Description,
		&item.Status,
		&item.Version,
		&item.Limits.MaxCorps,
		&item.Limits.MaxUsers,
		&item.Limits.MaxContacts,
		&item.Limits.MaxRooms,
		&item.Limits.MaxAgents,
		&item.Limits.ChannelCodes,
		&item.Limits.ShopCodes,
		&item.Limits.Radars,
		&item.Limits.Lotteries,
		&item.Limits.RoomInfinitePulls,
		&item.Limits.RoomFissions,
		&item.Limits.RoomClockIns,
		&item.Limits.RoomQualities,
		&item.Limits.RoomCalendars,
		&item.Limits.RoomReminds,
		&item.Limits.ContactSOPs,
		&item.Limits.RoomSOPs,
		&item.Limits.SensitiveWords,
		&item.Limits.StorageMB,
		&item.Limits.ContactMessageBatches,
		&item.Limits.RoomMessageBatches,
		&item.Limits.RoomTagPulls,
		&item.Limits.WorkRoomAutoPulls,
		&item.Limits.WorkFissions,
		&item.Limits.OfficialAccounts,
		&item.Limits.AsyncExecutions,
	)
	return item, err
}

func (s *MySQLStore) saasAdminSummary(ctx context.Context, options dashboard.SaaSAdminOverviewOptions) (dashboard.SaaSAdminSummary, error) {
	var summary dashboard.SaaSAdminSummary
	var err error
	if summary.TenantCount, err = s.saasAdminCount(ctx, "SELECT COUNT(*) FROM mc_tenant WHERE deleted_at IS NULL"+saasAdminTenantWhere("id", options.TenantID, options.ExcludedTenantID), saasAdminTenantArgs(options.TenantID, options.ExcludedTenantID)...); err != nil {
		return summary, err
	}
	if summary.UserCount, err = s.saasAdminCount(ctx, "SELECT COUNT(*) FROM mc_user WHERE deleted_at IS NULL"+saasAdminTenantWhere("tenant_id", options.TenantID, options.ExcludedTenantID), saasAdminTenantArgs(options.TenantID, options.ExcludedTenantID)...); err != nil {
		return summary, err
	}
	if summary.CorpCount, err = s.saasAdminCount(ctx, "SELECT COUNT(*) FROM mc_corp WHERE deleted_at IS NULL"+saasAdminTenantWhere("tenant_id", options.TenantID, options.ExcludedTenantID), saasAdminTenantArgs(options.TenantID, options.ExcludedTenantID)...); err != nil {
		return summary, err
	}
	if summary.EnabledPackageCount, err = s.saasAdminCountAllowMissing(ctx, "SELECT COUNT(*) FROM mochat_go_saas_packages WHERE status = 1 AND deleted_at IS NULL"); err != nil {
		return summary, err
	}
	if summary.ActiveTenantPackageCount, err = s.saasAdminCountAllowMissing(ctx, "SELECT COUNT(*) FROM mochat_go_saas_tenant_packages WHERE status = 1 AND deleted_at IS NULL"+saasAdminTenantWhere("tenant_id", options.TenantID, options.ExcludedTenantID), saasAdminTenantArgs(options.TenantID, options.ExcludedTenantID)...); err != nil {
		return summary, err
	}
	if summary.OpenAlertCount, err = s.saasAdminCountAllowMissing(ctx, "SELECT COUNT(*) FROM mochat_go_saas_alerts WHERE status = ? AND deleted_at IS NULL"+saasAdminTenantWhere("tenant_id", options.TenantID, options.ExcludedTenantID), append([]any{dashboard.SaaSAlertStatusOpen}, saasAdminTenantArgs(options.TenantID, options.ExcludedTenantID)...)...); err != nil {
		return summary, err
	}
	if summary.PendingNotificationCount, err = s.saasAdminCountAllowMissing(ctx, "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE status IN ('pending', 'failed') AND deleted_at IS NULL"+saasAdminTenantWhere("tenant_id", options.TenantID, options.ExcludedTenantID), saasAdminTenantArgs(options.TenantID, options.ExcludedTenantID)...); err != nil {
		return summary, err
	}
	if summary.ExpiringSoonTenantCount, err = s.saasAdminCountAllowMissing(ctx, "SELECT COUNT(*) FROM mochat_go_saas_tenant_packages WHERE status = 1 AND deleted_at IS NULL AND expires_at IS NOT NULL AND expires_at >= NOW() AND expires_at < DATE_ADD(NOW(), INTERVAL ? DAY)"+saasAdminTenantWhere("tenant_id", options.TenantID, options.ExcludedTenantID), append([]any{options.ExpiringDays}, saasAdminTenantArgs(options.TenantID, options.ExcludedTenantID)...)...); err != nil {
		return summary, err
	}
	if summary.ExpiredTenantCount, err = s.saasAdminCountAllowMissing(ctx, "SELECT COUNT(*) FROM mochat_go_saas_tenant_packages WHERE status = 1 AND deleted_at IS NULL AND expires_at IS NOT NULL AND expires_at < NOW()"+saasAdminTenantWhere("tenant_id", options.TenantID, options.ExcludedTenantID), saasAdminTenantArgs(options.TenantID, options.ExcludedTenantID)...); err != nil {
		return summary, err
	}
	return summary, nil
}

func (s *MySQLStore) saasAdminTenants(ctx context.Context, options dashboard.SaaSAdminOverviewOptions) ([]dashboard.SaaSAdminTenantOverview, error) {
	queryArgs := []any{options.ExpiringDays, dashboard.SaaSAlertStatusOpen}
	where := "WHERE t.deleted_at IS NULL"
	if options.TenantID > 0 {
		where += " AND t.id = ?"
		queryArgs = append(queryArgs, options.TenantID)
	}
	if options.ExcludedTenantID > 0 {
		where += " AND t.id <> ?"
		queryArgs = append(queryArgs, options.ExcludedTenantID)
	}
	keyword := strings.TrimSpace(options.Keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		where += " AND (t.name LIKE ? OR c.name LIKE ? OR CAST(t.id AS CHAR) LIKE ? OR tp.package_code LIKE ? OR tp.package_name LIKE ?)"
		queryArgs = append(queryArgs, like, like, like, like, like)
	}
	if options.TenantStatus == 1 || options.TenantStatus == 2 {
		where += " AND t.status = ?"
		queryArgs = append(queryArgs, options.TenantStatus)
	}
	packageCode := strings.TrimSpace(options.PackageCode)
	if packageCode != "" {
		where += " AND tp.package_code = ?"
		queryArgs = append(queryArgs, packageCode)
	}
	switch strings.TrimSpace(options.DueState) {
	case dashboard.SaaSAdminDueStateNormal:
		where += " AND tp.tenant_id IS NOT NULL AND (tp.expires_at IS NULL OR tp.expires_at >= DATE_ADD(NOW(), INTERVAL ? DAY))"
		queryArgs = append(queryArgs, options.ExpiringDays)
	case dashboard.SaaSAdminDueStateExpiring:
		where += " AND tp.expires_at IS NOT NULL AND tp.expires_at >= NOW() AND tp.expires_at < DATE_ADD(NOW(), INTERVAL ? DAY)"
		queryArgs = append(queryArgs, options.ExpiringDays)
	case dashboard.SaaSAdminDueStateExpired:
		where += " AND tp.expires_at IS NOT NULL AND tp.expires_at < NOW()"
	case dashboard.SaaSAdminDueStateNoPackage:
		where += " AND tp.tenant_id IS NULL"
	}
	queryArgs = append(queryArgs, options.Limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			t.id,
			COALESCE(t.name, ''),
			COALESCE(NULLIF(TRIM(c.name), ''), NULLIF(TRIM(t.name), ''), ''),
			COALESCE(t.status, 0),
			COALESCE(tp.package_code, ''),
			COALESCE(tp.package_name, ''),
			COALESCE(tp.status, 0),
			COALESCE(tp.version, 0),
			COALESCE(CAST(tp.limits_json AS CHAR), '{}'),
			COALESCE(DATE_FORMAT(tp.expires_at, '%Y-%m-%d %H:%i:%s'), ''),
			CASE WHEN tp.expires_at IS NOT NULL AND tp.expires_at < NOW() THEN 1 ELSE 0 END AS expired,
			CASE WHEN tp.expires_at IS NOT NULL AND tp.expires_at >= NOW() AND tp.expires_at < DATE_ADD(NOW(), INTERVAL ? DAY) THEN 1 ELSE 0 END AS expiring_soon,
			COALESCE(alerts.open_count, 0)
		FROM mc_tenant t
		LEFT JOIN mochat_go_tenant_corp_bindings tcb ON tcb.tenant_id = t.id
		LEFT JOIN mc_corp c ON c.tenant_id = t.id AND c.id = tcb.corp_id AND c.deleted_at IS NULL
		LEFT JOIN mochat_go_saas_tenant_packages tp ON tp.tenant_id = t.id AND tp.deleted_at IS NULL
		LEFT JOIN (
			SELECT tenant_id, COUNT(*) AS open_count
			FROM mochat_go_saas_alerts
			WHERE status = ? AND deleted_at IS NULL
			GROUP BY tenant_id
		) alerts ON alerts.tenant_id = t.id
		`+where+`
		ORDER BY alerts.open_count DESC, expired DESC, expiring_soon DESC, t.id ASC
		LIMIT ?
	`, queryArgs...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	tenants := []dashboard.SaaSAdminTenantOverview{}
	for rows.Next() {
		var item dashboard.SaaSAdminTenantOverview
		var expired, expiringSoon int
		var limitsJSON string
		if err := rows.Scan(
			&item.TenantID,
			&item.TenantName,
			&item.CompanyName,
			&item.TenantStatus,
			&item.PackageCode,
			&item.PackageName,
			&item.PackageStatus,
			&item.PackageVersion,
			&limitsJSON,
			&item.ExpiresAt,
			&expired,
			&expiringSoon,
			&item.OpenAlertCount,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(limitsJSON), &item.PackageLimits); err != nil {
			return nil, err
		}
		if strings.TrimSpace(item.CompanyName) == "" {
			item.CompanyName = item.TenantName
		}
		item.Expired = expired == 1
		item.ExpiringSoon = expiringSoon == 1
		tenants = append(tenants, item)
	}
	return tenants, rows.Err()
}

func (s *MySQLStore) attachSaaSAdminTenantUsage(ctx context.Context, tenants []dashboard.SaaSAdminTenantOverview) []dashboard.SaaSAdminTenantOverview {
	for index := range tenants {
		usage, err := s.saasAdminTenantMaxUsage(ctx, tenants[index].TenantID)
		if err != nil {
			continue
		}
		tenants[index].MaxUsageMetric = usage.Metric
		tenants[index].MaxUsageCurrent = usage.Current
		tenants[index].MaxUsageLimit = usage.Limit
		tenants[index].MaxUsageRatio = usage.UsageRatio
	}
	return tenants
}

func (s *MySQLStore) saasAdminTenantMaxUsage(ctx context.Context, tenantID int) (dashboard.SaaSAdminMetricOverview, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT metric, used_value, limit_value
		FROM mochat_go_saas_usage_counters
		WHERE tenant_id = ? AND period_key = 'lifetime' AND deleted_at IS NULL AND limit_value > 0
	`, tenantID)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminMetricOverview{}, nil
		}
		return dashboard.SaaSAdminMetricOverview{}, err
	}
	defer rows.Close()

	var max dashboard.SaaSAdminMetricOverview
	for rows.Next() {
		var item dashboard.SaaSAdminMetricOverview
		if err := rows.Scan(&item.Metric, &item.Current, &item.Limit); err != nil {
			return dashboard.SaaSAdminMetricOverview{}, err
		}
		item.UsageRatio = dashboard.SaaSAdminMetricUsageRatio(item.Current, item.Limit)
		if item.UsageRatio > max.UsageRatio {
			max = item
		}
	}
	return max, rows.Err()
}

func (s *MySQLStore) saasAdminMetrics(ctx context.Context, options dashboard.SaaSAdminOverviewOptions) ([]dashboard.SaaSAdminMetricOverview, error) {
	where := "WHERE c.period_key = 'lifetime' AND c.deleted_at IS NULL"
	alertWhere := "WHERE status = ? AND deleted_at IS NULL"
	alertScopeWhere := saasAdminTenantWhere("tenant_id", options.TenantID, options.ExcludedTenantID)
	counterScopeWhere := saasAdminTenantWhere("c.tenant_id", options.TenantID, options.ExcludedTenantID)
	alertWhere += alertScopeWhere
	where += counterScopeWhere
	queryArgs := []any{dashboard.SaaSAlertStatusOpen}
	queryArgs = append(queryArgs, saasAdminTenantArgs(options.TenantID, options.ExcludedTenantID)...)
	queryArgs = append(queryArgs, saasAdminTenantArgs(options.TenantID, options.ExcludedTenantID)...)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			c.metric,
			COALESCE(SUM(c.used_value), 0),
			COALESCE(SUM(c.limit_value), 0),
			COALESCE(alerts.open_count, 0)
		FROM mochat_go_saas_usage_counters c
		LEFT JOIN (
			SELECT metric, COUNT(*) AS open_count
			FROM mochat_go_saas_alerts
			`+alertWhere+`
			GROUP BY metric
		) alerts ON alerts.metric = c.metric
		`+where+`
		GROUP BY c.metric, alerts.open_count
		ORDER BY alerts.open_count DESC, c.metric ASC
	`, queryArgs...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	metrics := []dashboard.SaaSAdminMetricOverview{}
	for rows.Next() {
		var item dashboard.SaaSAdminMetricOverview
		if err := rows.Scan(&item.Metric, &item.Current, &item.Limit, &item.OpenAlertCount); err != nil {
			return nil, err
		}
		item.UsageRatio = dashboard.SaaSAdminMetricUsageRatio(item.Current, item.Limit)
		metrics = append(metrics, item)
	}
	return metrics, rows.Err()
}

func (s *MySQLStore) SaaSAdminTenantUsage(ctx context.Context, tenantID int) ([]dashboard.SaaSAdminUsageMetric, error) {
	if tenantID <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			c.metric,
			c.period_key,
			c.used_value,
			c.limit_value,
			COALESCE(alerts.open_count, 0),
			COALESCE(c.updated_by, ''),
			COALESCE(DATE_FORMAT(c.updated_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_usage_counters c
		LEFT JOIN (
			SELECT metric, period_key, COUNT(*) AS open_count
			FROM mochat_go_saas_alerts
			WHERE tenant_id = ? AND status = ? AND deleted_at IS NULL
			GROUP BY metric, period_key
		) alerts ON alerts.metric = c.metric AND alerts.period_key = c.period_key
		WHERE c.tenant_id = ? AND c.deleted_at IS NULL
		ORDER BY
			COALESCE(alerts.open_count, 0) DESC,
			CASE WHEN c.limit_value > 0 AND c.used_value > c.limit_value THEN 1 ELSE 0 END DESC,
			CASE WHEN c.limit_value > 0 THEN c.used_value / c.limit_value ELSE 0 END DESC,
			c.metric ASC
	`, tenantID, dashboard.SaaSAlertStatusOpen, tenantID)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	metrics := []dashboard.SaaSAdminUsageMetric{}
	for rows.Next() {
		var item dashboard.SaaSAdminUsageMetric
		if err := rows.Scan(
			&item.Metric,
			&item.PeriodKey,
			&item.Current,
			&item.Limit,
			&item.OpenAlertCount,
			&item.UpdatedBy,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.Unlimited = item.Limit <= 0
		item.Remaining = dashboard.SaaSAdminUsageMetricRemaining(item.Current, item.Limit)
		item.UsageRatio = dashboard.SaaSAdminMetricUsageRatio(item.Current, item.Limit)
		item.Status = dashboard.SaaSAdminUsageMetricStatus(item.Current, item.Limit, item.OpenAlertCount)
		metrics = append(metrics, item)
	}
	return metrics, rows.Err()
}

func (s *MySQLStore) saasAdminCount(ctx context.Context, query string, args ...any) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (s *MySQLStore) saasAdminCountAllowMissing(ctx context.Context, query string, args ...any) (int, error) {
	count, err := s.saasAdminCount(ctx, query, args...)
	if err != nil && isMissingSaaSTableError(err) {
		return 0, nil
	}
	return count, err
}

func saasAdminTenantWhere(column string, tenantID int, excludedTenantID int) string {
	where := ""
	if tenantID > 0 {
		where += " AND " + column + " = ?"
	}
	if excludedTenantID > 0 {
		where += " AND " + column + " <> ?"
	}
	return where
}

func saasAdminTenantArgs(tenantID int, excludedTenantID int) []any {
	args := []any{}
	if tenantID > 0 {
		args = append(args, tenantID)
	}
	if excludedTenantID > 0 {
		args = append(args, excludedTenantID)
	}
	return args
}

func (s *MySQLStore) upsertSaaSUsageCounter(ctx context.Context, tenantID int, metric string, current int64, limit int64, updatedBy string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_usage_counters
			(tenant_id, metric, period_key, used_value, limit_value, updated_by, created_at, updated_at, deleted_at)
		VALUES (?, ?, 'lifetime', ?, ?, ?, NOW(), NOW(), NULL)
		ON DUPLICATE KEY UPDATE
			used_value = VALUES(used_value),
			limit_value = VALUES(limit_value),
			updated_by = VALUES(updated_by),
			updated_at = NOW(),
			deleted_at = NULL
	`, tenantID, metric, current, limit, strings.TrimSpace(updatedBy))
	if err != nil && isMissingSaaSTableError(err) {
		return nil
	}
	return err
}

func (s *MySQLStore) RecordSaaSQuotaAlert(ctx context.Context, alert dashboard.SaaSQuotaAlert) error {
	status := alert.Status
	if status.TenantID <= 0 || !status.Exceeded() {
		return nil
	}
	metric := strings.TrimSpace(status.Metric)
	if metric == "" {
		return nil
	}
	alertType := strings.TrimSpace(alert.AlertType)
	if alertType == "" {
		alertType = dashboard.SaaSAlertTypeQuotaExceeded
	}
	severity := strings.TrimSpace(alert.Severity)
	if severity == "" {
		severity = dashboard.SaaSAlertSeverityWarning
	}
	periodKey := strings.TrimSpace(alert.PeriodKey)
	if periodKey == "" {
		periodKey = dashboard.SaaSAlertPeriodLifetime
	}
	source := strings.TrimSpace(alert.Source)
	if source == "" {
		source = "runtime"
	}
	message := truncateRunes(strings.TrimSpace(alert.Message), 255)
	if message == "" {
		message = truncateRunes(fmt.Sprintf("SaaS quota exceeded: %s %d/%d", metric, status.Current, status.Limit), 255)
	}
	var contextJSON any
	if len(alert.Context) > 0 {
		raw, err := json.Marshal(alert.Context)
		if err != nil {
			return fmt.Errorf("marshal SaaS alert context: %w", err)
		}
		contextJSON = string(raw)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_alerts (
			alert_key, tenant_id, alert_type, severity, status, metric, period_key,
			current_value, limit_value, additional_value, occurrence_count,
			source, message, context_json, first_seen_at, last_seen_at,
			resolved_at, created_at, updated_at, deleted_at
		)
		VALUES (
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, 1,
			?, ?, ?, NOW(), NOW(),
			NULL, NOW(), NOW(), NULL
		)
		ON DUPLICATE KEY UPDATE
			status = VALUES(status),
			severity = VALUES(severity),
			current_value = VALUES(current_value),
			limit_value = VALUES(limit_value),
			additional_value = VALUES(additional_value),
			occurrence_count = occurrence_count + 1,
			source = VALUES(source),
			message = VALUES(message),
			context_json = VALUES(context_json),
			last_seen_at = NOW(),
			resolved_at = NULL,
			updated_at = NOW(),
			deleted_at = NULL
	`, saasAlertKey(status.TenantID, metric, alertType, periodKey), status.TenantID, alertType, severity, dashboard.SaaSAlertStatusOpen, metric, periodKey,
		nonNegativeInt64(status.Current), nonNegativeInt64(status.Limit), nonNegativeInt64(status.Additional),
		truncateRunes(source, 64), message, contextJSON)
	if err != nil && isMissingSaaSTableError(err) {
		return nil
	}
	return err
}

func (s *MySQLStore) ListSaaSAlerts(ctx context.Context, options SaaSAlertListOptions) (SaaSAlertListPage, error) {
	whereSQL, args := saasAlertWhereSQL(options)
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_saas_alerts `+whereSQL, args...).Scan(&total); err != nil {
		if isMissingSaaSTableError(err) {
			return SaaSAlertListPage{}, nil
		}
		return SaaSAlertListPage{}, err
	}
	pageNumber := options.Page
	if pageNumber <= 0 {
		pageNumber = 1
	}
	perPage := options.PerPage
	if perPage <= 0 {
		perPage = 50
	}
	if perPage > 500 {
		perPage = 500
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + perPage - 1) / perPage
	}

	query := `
		SELECT id, alert_key, tenant_id, alert_type, severity, status, metric, period_key,
			current_value, limit_value, additional_value, occurrence_count,
			source, message, COALESCE(CAST(context_json AS CHAR), ''),
			first_seen_at, last_seen_at, resolved_at, created_at, updated_at
		FROM mochat_go_saas_alerts
	` + whereSQL + `
		ORDER BY status = 'open' DESC, last_seen_at DESC, id DESC
		LIMIT ? OFFSET ?
	`
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, perPage, (pageNumber-1)*perPage)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return SaaSAlertListPage{}, nil
		}
		return SaaSAlertListPage{}, err
	}
	defer rows.Close()

	alerts := []SaaSAlert{}
	for rows.Next() {
		var alert SaaSAlert
		var firstSeenAt, lastSeenAt, resolvedAt, createdAt, updatedAt sql.NullTime
		if err := rows.Scan(
			&alert.ID, &alert.AlertKey, &alert.TenantID, &alert.AlertType, &alert.Severity, &alert.Status, &alert.Metric, &alert.PeriodKey,
			&alert.CurrentValue, &alert.LimitValue, &alert.AdditionalValue, &alert.OccurrenceCount,
			&alert.Source, &alert.Message, &alert.ContextJSON,
			&firstSeenAt, &lastSeenAt, &resolvedAt, &createdAt, &updatedAt,
		); err != nil {
			return SaaSAlertListPage{}, err
		}
		alert.FirstSeenAt = formatTime(firstSeenAt)
		alert.LastSeenAt = formatTime(lastSeenAt)
		alert.ResolvedAt = formatTime(resolvedAt)
		alert.CreatedAt = formatTime(createdAt)
		alert.UpdatedAt = formatTime(updatedAt)
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return SaaSAlertListPage{}, err
	}
	return SaaSAlertListPage{Items: alerts, Total: total, TotalPage: totalPage}, nil
}

func (s *MySQLStore) SaaSAdminAlertSummary(ctx context.Context, options SaaSAlertListOptions) (dashboard.SaaSAdminAlertSummary, error) {
	whereSQL, args := saasAlertWhereSQL(options)
	queryArgs := []any{
		dashboard.SaaSAlertStatusOpen,
		dashboard.SaaSAlertStatusResolved,
		dashboard.SaaSAlertSeverityWarning,
		dashboard.SaaSAlertSeverityCritical,
	}
	queryArgs = append(queryArgs, args...)
	var summary dashboard.SaaSAdminAlertSummary
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN severity = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN severity = ? THEN 1 ELSE 0 END), 0),
			COUNT(DISTINCT NULLIF(metric, '')),
			COUNT(DISTINCT NULLIF(tenant_id, 0))
		FROM mochat_go_saas_alerts
		`+whereSQL,
		queryArgs...,
	).Scan(
		&summary.AlertCount,
		&summary.OpenCount,
		&summary.ResolvedCount,
		&summary.WarningCount,
		&summary.CriticalCount,
		&summary.MetricCount,
		&summary.TenantCount,
	)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminAlertSummary{}, nil
		}
		return dashboard.SaaSAdminAlertSummary{}, err
	}
	return summary, nil
}

func saasAlertWhereSQL(options SaaSAlertListOptions) (string, []any) {
	query := "WHERE deleted_at IS NULL"
	args := []any{}
	if options.TenantID > 0 {
		query += " AND tenant_id = ?"
		args = append(args, options.TenantID)
	}
	if options.ExcludedTenantID > 0 {
		query += " AND tenant_id <> ?"
		args = append(args, options.ExcludedTenantID)
	}
	if status := strings.TrimSpace(options.Status); status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if metric := strings.TrimSpace(options.Metric); metric != "" {
		query += " AND metric = ?"
		args = append(args, metric)
	}
	if alertType := strings.TrimSpace(options.AlertType); alertType != "" {
		query += " AND alert_type = ?"
		args = append(args, alertType)
	}
	return query, args
}

func (s *MySQLStore) ResolveSaaSAlert(ctx context.Context, tenantID int, metric string, alertType string) (bool, error) {
	if tenantID <= 0 {
		return false, nil
	}
	metric = strings.TrimSpace(metric)
	if metric == "" {
		return false, nil
	}
	alertType = strings.TrimSpace(alertType)
	if alertType == "" {
		alertType = dashboard.SaaSAlertTypeQuotaExceeded
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_saas_alerts
		SET status = ?, resolved_at = NOW(), updated_at = NOW()
		WHERE alert_key = ? AND status = ? AND deleted_at IS NULL
	`, dashboard.SaaSAlertStatusResolved, saasAlertKey(tenantID, metric, alertType, dashboard.SaaSAlertPeriodLifetime), dashboard.SaaSAlertStatusOpen)
	if err != nil && isMissingSaaSTableError(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) ResolveSaaSAdminAlert(ctx context.Context, resolve dashboard.SaaSAdminAlertResolve) (dashboard.SaaSAdminAlertResolveResult, error) {
	if resolve.TenantID <= 0 {
		return dashboard.SaaSAdminAlertResolveResult{}, dashboard.NewSaaSAdminBadRequest("tenantId required")
	}
	metric := strings.TrimSpace(resolve.Metric)
	if metric == "" {
		return dashboard.SaaSAdminAlertResolveResult{}, dashboard.NewSaaSAdminBadRequest("metric required")
	}
	alertType := strings.TrimSpace(resolve.AlertType)
	if alertType == "" {
		alertType = dashboard.SaaSAlertTypeQuotaExceeded
	}
	periodKey := strings.TrimSpace(resolve.PeriodKey)
	if periodKey == "" {
		periodKey = dashboard.SaaSAlertPeriodLifetime
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminAlertResolveResult{}, err
	}
	defer rollbackQuietly(tx)

	var alert dashboard.SaaSAlertRecord
	err = tx.QueryRowContext(ctx, `
		SELECT id, alert_key, tenant_id, alert_type, severity, status, metric, period_key,
			current_value, limit_value, additional_value, occurrence_count, source, message,
			COALESCE(CAST(context_json AS CHAR), '')
		FROM mochat_go_saas_alerts
		WHERE tenant_id = ? AND metric = ? AND alert_type = ? AND period_key = ? AND status = ? AND deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, resolve.TenantID, metric, alertType, periodKey, dashboard.SaaSAlertStatusOpen).Scan(
		&alert.ID,
		&alert.AlertKey,
		&alert.TenantID,
		&alert.AlertType,
		&alert.Severity,
		&alert.Status,
		&alert.Metric,
		&alert.PeriodKey,
		&alert.CurrentValue,
		&alert.LimitValue,
		&alert.AdditionalValue,
		&alert.OccurrenceCount,
		&alert.Source,
		&alert.Message,
		&alert.ContextJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminAlertResolveResult{
			Resolved:  false,
			TenantID:  resolve.TenantID,
			Metric:    metric,
			AlertType: alertType,
			PeriodKey: periodKey,
			Status:    dashboard.SaaSAlertStatusOpen,
			Remark:    strings.TrimSpace(resolve.Remark),
		}, nil
	}
	if err != nil && isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminAlertResolveResult{}, nil
	}
	if err != nil {
		return dashboard.SaaSAdminAlertResolveResult{}, err
	}

	var tenantName string
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(name, '')
		FROM mc_tenant
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, alert.TenantID).Scan(&tenantName); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminAlertResolveResult{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_alerts
		SET status = ?, resolved_at = NOW(), updated_at = NOW()
		WHERE id = ? AND status = ? AND deleted_at IS NULL
	`, dashboard.SaaSAlertStatusResolved, alert.ID, dashboard.SaaSAlertStatusOpen); err != nil {
		return dashboard.SaaSAdminAlertResolveResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID:      alert.TenantID,
		ActorUserID:   resolve.ActorUserID,
		ActorTenantID: resolve.ActorTenantID,
		Action:        "tenant.alert.resolve",
		TargetType:    "alert",
		TargetID:      alert.AlertKey,
		TargetName:    alert.Metric,
		BeforeJSON:    saasAdminMarshalJSON(alert),
		AfterJSON: saasAdminMarshalJSON(map[string]any{
			"status":    dashboard.SaaSAlertStatusResolved,
			"metric":    alert.Metric,
			"alertType": alert.AlertType,
			"periodKey": alert.PeriodKey,
		}),
		Remark: strings.TrimSpace(resolve.Remark),
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminAlertResolveResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminAlertResolveResult{}, err
	}
	return dashboard.SaaSAdminAlertResolveResult{
		Resolved:       true,
		TenantID:       alert.TenantID,
		TenantName:     tenantName,
		AlertID:        alert.ID,
		AlertKey:       alert.AlertKey,
		Metric:         alert.Metric,
		AlertType:      alert.AlertType,
		PeriodKey:      alert.PeriodKey,
		PreviousStatus: alert.Status,
		Status:         dashboard.SaaSAlertStatusResolved,
		Remark:         strings.TrimSpace(resolve.Remark),
		OperationID:    operationID,
	}, nil
}

func (s *MySQLStore) BulkResolveSaaSAdminAlerts(ctx context.Context, resolve dashboard.SaaSAdminAlertBulkResolve) (dashboard.SaaSAdminAlertBulkResolveResult, error) {
	if resolve.Limit <= 0 {
		resolve.Limit = 50
	}
	if resolve.Limit > 100 {
		resolve.Limit = 100
	}
	if resolve.AllowedTenantID > 0 {
		if resolve.TenantID > 0 && resolve.TenantID != resolve.AllowedTenantID {
			return dashboard.SaaSAdminAlertBulkResolveResult{}, dashboard.NewSaaSAdminNotFound("alert not found")
		}
		resolve.TenantID = resolve.AllowedTenantID
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminAlertBulkResolveResult{}, err
	}
	defer rollbackQuietly(tx)

	args := []any{dashboard.SaaSAlertStatusOpen}
	where := "WHERE deleted_at IS NULL AND status = ?"
	if resolve.TenantID > 0 {
		where += " AND tenant_id = ?"
		args = append(args, resolve.TenantID)
	}
	if metric := strings.TrimSpace(resolve.Metric); metric != "" {
		where += " AND metric = ?"
		args = append(args, metric)
	}
	if alertType := strings.TrimSpace(resolve.AlertType); alertType != "" {
		where += " AND alert_type = ?"
		args = append(args, alertType)
	}
	if periodKey := strings.TrimSpace(resolve.PeriodKey); periodKey != "" {
		where += " AND period_key = ?"
		args = append(args, periodKey)
	}
	args = append(args, resolve.Limit)
	rows, err := tx.QueryContext(ctx, `
		SELECT id, alert_key, tenant_id, alert_type, severity, status, metric, period_key,
			current_value, limit_value, additional_value, occurrence_count, source, message,
			COALESCE(CAST(context_json AS CHAR), '')
		FROM mochat_go_saas_alerts
		`+where+`
		ORDER BY last_seen_at DESC, id ASC
		LIMIT ?
		FOR UPDATE
	`, args...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminAlertBulkResolveResult{}, nil
		}
		return dashboard.SaaSAdminAlertBulkResolveResult{}, err
	}
	defer rows.Close()

	alerts := []dashboard.SaaSAlertRecord{}
	for rows.Next() {
		var alert dashboard.SaaSAlertRecord
		if err := rows.Scan(
			&alert.ID,
			&alert.AlertKey,
			&alert.TenantID,
			&alert.AlertType,
			&alert.Severity,
			&alert.Status,
			&alert.Metric,
			&alert.PeriodKey,
			&alert.CurrentValue,
			&alert.LimitValue,
			&alert.AdditionalValue,
			&alert.OccurrenceCount,
			&alert.Source,
			&alert.Message,
			&alert.ContextJSON,
		); err != nil {
			return dashboard.SaaSAdminAlertBulkResolveResult{}, err
		}
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSAdminAlertBulkResolveResult{}, err
	}
	if err := rows.Close(); err != nil {
		return dashboard.SaaSAdminAlertBulkResolveResult{}, err
	}

	remark := strings.TrimSpace(resolve.Remark)
	results := make([]dashboard.SaaSAdminAlertResolveResult, 0, len(alerts))
	for _, alert := range alerts {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_alerts
			SET status = ?, resolved_at = NOW(), updated_at = NOW()
			WHERE id = ? AND status = ? AND deleted_at IS NULL
		`, dashboard.SaaSAlertStatusResolved, alert.ID, dashboard.SaaSAlertStatusOpen); err != nil {
			return dashboard.SaaSAdminAlertBulkResolveResult{}, err
		}
		operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
			TenantID:      alert.TenantID,
			ActorUserID:   resolve.ActorUserID,
			ActorTenantID: resolve.ActorTenantID,
			Action:        "tenant.alert.resolve",
			TargetType:    "alert",
			TargetID:      alert.AlertKey,
			TargetName:    alert.Metric,
			BeforeJSON:    saasAdminMarshalJSON(alert),
			AfterJSON: saasAdminMarshalJSON(map[string]any{
				"status":      dashboard.SaaSAlertStatusResolved,
				"metric":      alert.Metric,
				"alertType":   alert.AlertType,
				"periodKey":   alert.PeriodKey,
				"bulkResolve": true,
			}),
			Remark: remark,
		})
		if err != nil && !isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminAlertBulkResolveResult{}, err
		}
		results = append(results, dashboard.SaaSAdminAlertResolveResult{
			Resolved:       true,
			TenantID:       alert.TenantID,
			AlertID:        alert.ID,
			AlertKey:       alert.AlertKey,
			Metric:         alert.Metric,
			AlertType:      alert.AlertType,
			PeriodKey:      alert.PeriodKey,
			PreviousStatus: alert.Status,
			Status:         dashboard.SaaSAlertStatusResolved,
			Remark:         remark,
			OperationID:    operationID,
		})
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminAlertBulkResolveResult{}, err
	}
	return dashboard.SaaSAdminAlertBulkResolveResult{
		ResolvedCount: len(results),
		TenantID:      resolve.TenantID,
		Metric:        strings.TrimSpace(resolve.Metric),
		AlertType:     strings.TrimSpace(resolve.AlertType),
		PeriodKey:     strings.TrimSpace(resolve.PeriodKey),
		Limit:         resolve.Limit,
		Remark:        remark,
		Alerts:        results,
	}, nil
}

func (s *MySQLStore) GetSaaSAlertSetting(ctx context.Context, tenantID int, channel string) (SaaSAlertSetting, bool, error) {
	if tenantID <= 0 {
		return SaaSAlertSetting{}, false, nil
	}
	channel = strings.TrimSpace(channel)
	if channel == "" {
		channel = dashboard.SaaSAlertNotificationChannelWebhook
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, channel, enabled, webhook_url, webhook_secret,
			COALESCE(webhook_credentials_ciphertext, ''), webhook_credentials_key_id,
			webhook_timeout_seconds, webhook_retry_attempts, webhook_retry_delay_ms,
			webhook_title_template, webhook_body_template,
			notification_max_attempts, notification_retry_delay_seconds,
			minimum_severity, COALESCE(CAST(allowed_alert_types_json AS CHAR), '[]'),
			quiet_hours_enabled, quiet_hours_start, quiet_hours_end, timezone, hourly_limit,
			created_at, updated_at
		FROM mochat_go_saas_alert_settings
		WHERE tenant_id = ? AND channel = ? AND deleted_at IS NULL
	`, tenantID, channel)
	setting, err := s.scanSaaSAlertSetting(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isMissingSaaSTableError(err) {
			return dashboard.DefaultSaaSAlertSetting(tenantID, channel), false, nil
		}
		return SaaSAlertSetting{}, false, err
	}
	return dashboard.NormalizeSaaSAlertSetting(setting), true, nil
}

func (s *MySQLStore) SaaSAdminNotificationPolicies(ctx context.Context, options dashboard.SaaSAdminNotificationPolicyOptions) (dashboard.SaaSAdminNotificationPolicyReport, error) {
	channel := strings.TrimSpace(options.Channel)
	if channel == "" {
		channel = dashboard.SaaSAlertNotificationChannelWebhook
	}
	state := strings.TrimSpace(options.State)
	if state == "" {
		state = dashboard.SaaSAdminNotificationPolicyStateAll
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	options.Channel = channel
	options.State = state
	options.Limit = limit

	baseWhere := " WHERE t.deleted_at IS NULL"
	baseArgs := []any{channel}
	if options.TenantID > 0 {
		baseWhere += " AND t.id = ?"
		baseArgs = append(baseArgs, options.TenantID)
	}
	if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		baseWhere += " AND (t.name LIKE ? OR CAST(t.id AS CHAR) LIKE ? OR tp.package_code LIKE ? OR tp.package_name LIKE ?)"
		baseArgs = append(baseArgs, like, like, like, like)
	}
	join := `
		FROM mc_tenant t
		LEFT JOIN mochat_go_saas_tenant_packages tp ON tp.tenant_id = t.id AND tp.deleted_at IS NULL
		LEFT JOIN mochat_go_saas_alert_settings s ON s.tenant_id = t.id AND s.channel = ? AND s.deleted_at IS NULL
	`

	report := dashboard.SaaSAdminNotificationPolicyReport{Options: options}
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN s.id IS NOT NULL THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN s.id IS NOT NULL AND s.enabled = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN s.id IS NOT NULL AND s.enabled = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN s.id IS NULL THEN 1 ELSE 0 END), 0)
		`+join+baseWhere,
		baseArgs...,
	).Scan(
		&report.Summary.TenantCount,
		&report.Summary.ConfiguredCount,
		&report.Summary.EnabledCount,
		&report.Summary.DisabledCount,
		&report.Summary.UnconfiguredCount,
	); err != nil {
		return dashboard.SaaSAdminNotificationPolicyReport{}, err
	}

	stateWhere := ""
	switch state {
	case dashboard.SaaSAdminNotificationPolicyStateEnabled:
		stateWhere = " AND s.id IS NOT NULL AND s.enabled = 1"
		report.Summary.MatchedCount = report.Summary.EnabledCount
	case dashboard.SaaSAdminNotificationPolicyStateDisabled:
		stateWhere = " AND s.id IS NOT NULL AND s.enabled = 0"
		report.Summary.MatchedCount = report.Summary.DisabledCount
	case dashboard.SaaSAdminNotificationPolicyStateUnconfigured:
		stateWhere = " AND s.id IS NULL"
		report.Summary.MatchedCount = report.Summary.UnconfiguredCount
	default:
		report.Summary.MatchedCount = report.Summary.TenantCount
	}
	listArgs := append(append([]any{}, baseArgs...), limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			t.id,
			COALESCE(t.name, ''),
			COALESCE(t.status, 0),
			COALESCE(tp.package_code, ''),
			COALESCE(tp.package_name, ''),
			CASE WHEN s.id IS NOT NULL THEN 1 ELSE 0 END,
			COALESCE(s.id, 0),
			CASE WHEN s.id IS NOT NULL AND s.enabled = 1 THEN 1 ELSE 0 END,
			COALESCE(s.webhook_url, ''),
			COALESCE(s.webhook_secret, ''),
			COALESCE(s.webhook_credentials_ciphertext, ''),
			COALESCE(s.webhook_credentials_key_id, ''),
			COALESCE(s.webhook_timeout_seconds, 0),
			COALESCE(s.webhook_retry_attempts, 0),
			COALESCE(s.webhook_retry_delay_ms, 0),
			COALESCE(s.webhook_title_template, ''),
			COALESCE(s.webhook_body_template, ''),
			COALESCE(s.notification_max_attempts, 0),
			COALESCE(s.notification_retry_delay_seconds, 0),
			COALESCE(s.minimum_severity, ''),
			COALESCE(CAST(s.allowed_alert_types_json AS CHAR), '[]'),
			CASE WHEN s.id IS NOT NULL AND s.quiet_hours_enabled = 1 THEN 1 ELSE 0 END,
			COALESCE(s.quiet_hours_start, ''),
			COALESCE(s.quiet_hours_end, ''),
			COALESCE(s.timezone, ''),
			COALESCE(s.hourly_limit, 0),
			COALESCE(DATE_FORMAT(s.created_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(s.updated_at, '%Y-%m-%d %H:%i:%s'), '')
		`+join+baseWhere+stateWhere+`
		ORDER BY
			CASE WHEN s.id IS NULL THEN 0 WHEN s.enabled = 0 THEN 1 ELSE 2 END ASC,
			t.id ASC
		LIMIT ?
	`, listArgs...)
	if err != nil {
		return dashboard.SaaSAdminNotificationPolicyReport{}, err
	}
	defer rows.Close()

	report.Policies = make([]dashboard.SaaSAdminNotificationPolicy, 0)
	for rows.Next() {
		var policy dashboard.SaaSAdminNotificationPolicy
		var configured, enabled, quietHoursEnabled int
		var allowedAlertTypesJSON, credentialCiphertext, credentialKeyID string
		setting := dashboard.SaaSAlertSetting{Channel: channel}
		if err := rows.Scan(
			&policy.TenantID,
			&policy.TenantName,
			&policy.TenantStatus,
			&policy.PackageCode,
			&policy.PackageName,
			&configured,
			&setting.ID,
			&enabled,
			&setting.WebhookURL,
			&setting.WebhookSecret,
			&credentialCiphertext,
			&credentialKeyID,
			&setting.WebhookTimeoutSeconds,
			&setting.WebhookRetryAttempts,
			&setting.WebhookRetryDelayMS,
			&setting.WebhookTitleTemplate,
			&setting.WebhookBodyTemplate,
			&setting.NotificationMaxAttempts,
			&setting.NotificationRetryDelaySeconds,
			&setting.MinimumSeverity,
			&allowedAlertTypesJSON,
			&quietHoursEnabled,
			&setting.QuietHoursStart,
			&setting.QuietHoursEnd,
			&setting.Timezone,
			&setting.HourlyLimit,
			&setting.CreatedAt,
			&setting.UpdatedAt,
		); err != nil {
			return dashboard.SaaSAdminNotificationPolicyReport{}, err
		}
		policy.Configured = configured == 1
		if policy.Configured {
			setting.TenantID = policy.TenantID
			setting.Enabled = enabled == 1
			setting.QuietHoursEnabled = quietHoursEnabled == 1
			setting, err = s.decodeSaaSAlertCredentials(setting, credentialCiphertext, credentialKeyID)
			if err != nil {
				return dashboard.SaaSAdminNotificationPolicyReport{}, err
			}
			if err := json.Unmarshal([]byte(allowedAlertTypesJSON), &setting.AllowedAlertTypes); err != nil {
				return dashboard.SaaSAdminNotificationPolicyReport{}, fmt.Errorf("unmarshal SaaS alert setting alert types: %w", err)
			}
			setting = dashboard.NormalizeSaaSAlertSetting(setting)
		} else {
			setting = dashboard.DefaultSaaSAlertSetting(policy.TenantID, channel)
		}
		policy.Setting = setting
		report.Policies = append(report.Policies, policy)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSAdminNotificationPolicyReport{}, err
	}
	return report, nil
}

func (s *MySQLStore) SaveSaaSAlertSetting(ctx context.Context, setting dashboard.SaaSAlertSetting) (SaaSAlertSetting, error) {
	setting = dashboard.NormalizeSaaSAlertSetting(setting)
	if err := dashboard.ValidateSaaSAlertSetting(setting); err != nil {
		return SaaSAlertSetting{}, err
	}
	allowedAlertTypesJSON, err := json.Marshal(setting.AllowedAlertTypes)
	if err != nil {
		return SaaSAlertSetting{}, fmt.Errorf("marshal SaaS alert setting alert types: %w", err)
	}
	storedWebhookURL, storedWebhookSecret, credentialCiphertext, credentialKeyID, err := s.encodeSaaSAlertCredentials(setting)
	if err != nil {
		return SaaSAlertSetting{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_alert_settings (
			tenant_id, channel, enabled, webhook_url, webhook_secret,
			webhook_credentials_ciphertext, webhook_credentials_key_id,
			webhook_timeout_seconds, webhook_retry_attempts, webhook_retry_delay_ms,
			webhook_title_template, webhook_body_template,
			notification_max_attempts, notification_retry_delay_seconds,
			minimum_severity, allowed_alert_types_json,
			quiet_hours_enabled, quiet_hours_start, quiet_hours_end, timezone, hourly_limit,
			created_at, updated_at, deleted_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NULL)
		ON DUPLICATE KEY UPDATE
			enabled = VALUES(enabled),
			webhook_url = VALUES(webhook_url),
			webhook_secret = VALUES(webhook_secret),
			webhook_credentials_ciphertext = VALUES(webhook_credentials_ciphertext),
			webhook_credentials_key_id = VALUES(webhook_credentials_key_id),
			webhook_timeout_seconds = VALUES(webhook_timeout_seconds),
			webhook_retry_attempts = VALUES(webhook_retry_attempts),
			webhook_retry_delay_ms = VALUES(webhook_retry_delay_ms),
			webhook_title_template = VALUES(webhook_title_template),
			webhook_body_template = VALUES(webhook_body_template),
			notification_max_attempts = VALUES(notification_max_attempts),
			notification_retry_delay_seconds = VALUES(notification_retry_delay_seconds),
			minimum_severity = VALUES(minimum_severity),
			allowed_alert_types_json = VALUES(allowed_alert_types_json),
			quiet_hours_enabled = VALUES(quiet_hours_enabled),
			quiet_hours_start = VALUES(quiet_hours_start),
			quiet_hours_end = VALUES(quiet_hours_end),
			timezone = VALUES(timezone),
			hourly_limit = VALUES(hourly_limit),
			updated_at = NOW(),
			deleted_at = NULL
	`, setting.TenantID, setting.Channel, boolTinyInt(setting.Enabled), storedWebhookURL, storedWebhookSecret,
		credentialCiphertext, credentialKeyID,
		setting.WebhookTimeoutSeconds, setting.WebhookRetryAttempts, setting.WebhookRetryDelayMS,
		setting.WebhookTitleTemplate, setting.WebhookBodyTemplate,
		setting.NotificationMaxAttempts, setting.NotificationRetryDelaySeconds,
		setting.MinimumSeverity, string(allowedAlertTypesJSON), boolTinyInt(setting.QuietHoursEnabled),
		setting.QuietHoursStart, setting.QuietHoursEnd, setting.Timezone, setting.HourlyLimit)
	if err != nil {
		return SaaSAlertSetting{}, err
	}
	saved, _, err := s.GetSaaSAlertSetting(ctx, setting.TenantID, setting.Channel)
	return saved, err
}

func (s *MySQLStore) encodeSaaSAlertCredentials(setting dashboard.SaaSAlertSetting) (string, string, string, string, error) {
	webhookURL := strings.TrimSpace(setting.WebhookURL)
	webhookSecret := strings.TrimSpace(setting.WebhookSecret)
	if webhookURL == "" && webhookSecret == "" {
		return "", "", "", "", nil
	}
	if s.saasAlertCredentialCipher != nil {
		status := s.saasAlertCredentialCipher.ConfigStatus()
		if status.EncryptionConfigured {
			ciphertext, keyID, err := s.saasAlertCredentialCipher.Encrypt(setting.TenantID, setting.Channel, saasalertcredentials.Credential{
				WebhookURL: webhookURL, WebhookSecret: webhookSecret,
			})
			if err != nil {
				return "", "", "", "", fmt.Errorf("encrypt SaaS alert webhook credentials: %w", err)
			}
			return "", "", ciphertext, keyID, nil
		}
		if status.RequireEncryption {
			return "", "", "", "", errors.New("SaaS alert webhook credential encryption is required")
		}
	}
	return webhookURL, webhookSecret, "", "", nil
}

func (s *MySQLStore) decodeSaaSAlertCredentials(setting dashboard.SaaSAlertSetting, ciphertext string, keyID string) (dashboard.SaaSAlertSetting, error) {
	ciphertext = strings.TrimSpace(ciphertext)
	keyID = strings.TrimSpace(keyID)
	if ciphertext != "" {
		if keyID == "" {
			return dashboard.SaaSAlertSetting{}, errors.New("SaaS alert webhook credential ciphertext has no encryption key id")
		}
		if s.saasAlertCredentialCipher == nil {
			return dashboard.SaaSAlertSetting{}, fmt.Errorf("SaaS alert webhook credential encryption key %q is unavailable", keyID)
		}
		credential, err := s.saasAlertCredentialCipher.Decrypt(setting.TenantID, setting.Channel, keyID, ciphertext)
		if err != nil {
			return dashboard.SaaSAlertSetting{}, fmt.Errorf("decrypt SaaS alert webhook credentials for tenant %d channel %s: %w", setting.TenantID, setting.Channel, err)
		}
		setting.WebhookURL = credential.WebhookURL
		setting.WebhookSecret = credential.WebhookSecret
		setting.WebhookCredentialProtection = dashboard.SaaSAlertCredentialProtectionEncrypted
		setting.WebhookCredentialKeyID = keyID
		return setting, nil
	}
	setting.WebhookURL = strings.TrimSpace(setting.WebhookURL)
	setting.WebhookSecret = strings.TrimSpace(setting.WebhookSecret)
	setting.WebhookCredentialKeyID = ""
	setting.WebhookCredentialProtection = dashboard.SaaSAlertCredentialProtectionEmpty
	if setting.WebhookURL != "" || setting.WebhookSecret != "" {
		setting.WebhookCredentialProtection = dashboard.SaaSAlertCredentialProtectionLegacyPlaintext
	}
	return setting, nil
}

func (s *MySQLStore) SaaSAlertCredentialProtection(ctx context.Context) (dashboard.SaaSAlertCredentialProtectionStatus, error) {
	status := dashboard.SaaSAlertCredentialProtectionStatus{ActiveKeyID: "primary", UnavailableKeyIDs: []string{}}
	if s.saasAlertCredentialCipher != nil {
		config := s.saasAlertCredentialCipher.ConfigStatus()
		status.EncryptionConfigured = config.EncryptionConfigured
		status.RequireEncryption = config.RequireEncryption
		status.DedicatedConfigured = config.DedicatedConfigured
		status.ActiveKeyID = config.ActiveKeyID
		status.KeyCount = config.KeyCount
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT tenant_id, channel, webhook_url, webhook_secret,
			COALESCE(webhook_credentials_ciphertext, ''), webhook_credentials_key_id
		FROM mochat_go_saas_alert_settings
		WHERE deleted_at IS NULL
			AND (TRIM(webhook_url) <> '' OR TRIM(webhook_secret) <> '' OR COALESCE(webhook_credentials_ciphertext, '') <> '')
		ORDER BY id ASC
	`)
	if err != nil {
		if isMissingSaaSTableError(err) {
			status.Healthy = !status.RequireEncryption || status.EncryptionConfigured
			return status, nil
		}
		return dashboard.SaaSAlertCredentialProtectionStatus{}, err
	}
	defer rows.Close()
	unavailable := map[string]struct{}{}
	for rows.Next() {
		var tenantID int
		var channel, webhookURL, webhookSecret, ciphertext, keyID string
		if err := rows.Scan(&tenantID, &channel, &webhookURL, &webhookSecret, &ciphertext, &keyID); err != nil {
			return dashboard.SaaSAlertCredentialProtectionStatus{}, err
		}
		status.ConfiguredCredentialCount++
		ciphertext = strings.TrimSpace(ciphertext)
		keyID = strings.TrimSpace(keyID)
		if ciphertext == "" {
			status.LegacyPlaintextCount++
			status.RotationRequiredCount++
			continue
		}
		status.EncryptedCredentialCount++
		if keyID == status.ActiveKeyID {
			status.ActiveKeyCredentialCount++
		} else {
			status.RotationRequiredCount++
		}
		if s.saasAlertCredentialCipher == nil || !s.saasAlertCredentialCipher.HasKey(keyID) {
			status.UnavailableKeyCount++
			unavailable[keyID] = struct{}{}
			continue
		}
		if _, err := s.saasAlertCredentialCipher.Decrypt(tenantID, channel, keyID, ciphertext); err != nil {
			status.DecryptFailureCount++
		}
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSAlertCredentialProtectionStatus{}, err
	}
	for keyID := range unavailable {
		if keyID == "" {
			keyID = "(missing)"
		}
		status.UnavailableKeyIDs = append(status.UnavailableKeyIDs, keyID)
	}
	sort.Strings(status.UnavailableKeyIDs)
	status.Healthy = status.LegacyPlaintextCount == 0 && status.UnavailableKeyCount == 0 && status.DecryptFailureCount == 0
	if status.RequireEncryption && !status.EncryptionConfigured {
		status.Healthy = false
	}
	if status.ConfiguredCredentialCount > 0 && !status.EncryptionConfigured {
		status.Healthy = false
	}
	return status, nil
}

func (s *MySQLStore) CheckSaaSAlertCredentialProtection(ctx context.Context) error {
	status, err := s.SaaSAlertCredentialProtection(ctx)
	if err != nil {
		return err
	}
	if status.Healthy {
		return nil
	}
	return fmt.Errorf("Webhook credentials are not fully protected: legacy=%d unavailable_keys=%d decrypt_failures=%d rotation_required=%d",
		status.LegacyPlaintextCount, status.UnavailableKeyCount, status.DecryptFailureCount, status.RotationRequiredCount)
}

func (s *MySQLStore) RotateSaaSAlertCredentials(ctx context.Context, tenantID int, limit int) (dashboard.SaaSAlertCredentialRotationResult, error) {
	result := dashboard.SaaSAlertCredentialRotationResult{TenantID: tenantID, Limit: limit}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	result.Limit = limit
	if s.saasAlertCredentialCipher == nil || !s.saasAlertCredentialCipher.ConfigStatus().EncryptionConfigured {
		return result, errors.New("SaaS alert webhook credential encryption key is not configured")
	}
	result.ActiveKeyID = s.saasAlertCredentialCipher.ConfigStatus().ActiveKeyID
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()
	query := `
		SELECT id, tenant_id, channel, webhook_url, webhook_secret,
			COALESCE(webhook_credentials_ciphertext, ''), webhook_credentials_key_id
		FROM mochat_go_saas_alert_settings
		WHERE deleted_at IS NULL
			AND (TRIM(webhook_url) <> '' OR TRIM(webhook_secret) <> '' OR COALESCE(webhook_credentials_ciphertext, '') <> '')
			AND (TRIM(webhook_url) <> '' OR TRIM(webhook_secret) <> '' OR COALESCE(webhook_credentials_key_id, '') <> ? OR COALESCE(webhook_credentials_ciphertext, '') = '')`
	args := []any{result.ActiveKeyID}
	if tenantID > 0 {
		query += " AND tenant_id = ?"
		args = append(args, tenantID)
	}
	query += " ORDER BY id ASC LIMIT ? FOR UPDATE"
	args = append(args, limit)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return result, err
	}
	type candidate struct {
		id                                 int64
		tenantID                           int
		channel, webhookURL, webhookSecret string
		ciphertext, keyID                  string
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.tenantID, &item.channel, &item.webhookURL, &item.webhookSecret, &item.ciphertext, &item.keyID); err != nil {
			rows.Close()
			return result, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	result.ScannedCount = len(candidates)
	for _, item := range candidates {
		credential := saasalertcredentials.Credential{
			WebhookURL: strings.TrimSpace(item.webhookURL), WebhookSecret: strings.TrimSpace(item.webhookSecret),
		}
		if strings.TrimSpace(item.ciphertext) != "" {
			credential, err = s.saasAlertCredentialCipher.Decrypt(item.tenantID, item.channel, item.keyID, item.ciphertext)
			if err != nil {
				return result, fmt.Errorf("decrypt SaaS alert webhook credentials for tenant %d channel %s: %w", item.tenantID, item.channel, err)
			}
			result.ReencryptedCount++
		} else {
			result.LegacyCount++
		}
		ciphertext, keyID, err := s.saasAlertCredentialCipher.Encrypt(item.tenantID, item.channel, credential)
		if err != nil {
			return result, fmt.Errorf("encrypt SaaS alert webhook credentials for tenant %d channel %s: %w", item.tenantID, item.channel, err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_alert_settings
			SET webhook_url = '', webhook_secret = '', webhook_credentials_ciphertext = ?, webhook_credentials_key_id = ?, updated_at = NOW()
			WHERE id = ? AND deleted_at IS NULL
		`, ciphertext, keyID, item.id); err != nil {
			return result, err
		}
		result.RotatedCount++
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func (s *MySQLStore) EnqueueSaaSAlertNotification(ctx context.Context, alert dashboard.SaaSQuotaAlert, channel string, maxAttempts int) (SaaSAlertNotification, error) {
	status := alert.Status
	if status.TenantID <= 0 {
		return SaaSAlertNotification{}, nil
	}
	metric := strings.TrimSpace(status.Metric)
	if metric == "" {
		return SaaSAlertNotification{}, nil
	}
	alertType := strings.TrimSpace(alert.AlertType)
	if alertType == "" {
		alertType = dashboard.SaaSAlertTypeQuotaExceeded
		alert.AlertType = alertType
	}
	periodKey := strings.TrimSpace(alert.PeriodKey)
	if periodKey == "" {
		periodKey = dashboard.SaaSAlertPeriodLifetime
		alert.PeriodKey = periodKey
	}
	channel = strings.TrimSpace(channel)
	if channel == "" {
		channel = dashboard.SaaSAlertNotificationChannelWebhook
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	alertKey := saasAlertKey(status.TenantID, metric, alertType, periodKey)
	notificationKey := dashboard.SaaSAlertNotificationKey(alert, channel)
	raw, err := json.Marshal(alert)
	if err != nil {
		return SaaSAlertNotification{}, fmt.Errorf("marshal SaaS alert notification: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_alert_notifications (
			notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts,
			alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at
		)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?, '', NOW(), NULL, NOW(), NOW(), NULL)
		ON DUPLICATE KEY UPDATE
			alert_key = VALUES(alert_key),
			tenant_id = VALUES(tenant_id),
			channel = VALUES(channel),
			status = VALUES(status),
			attempts = 0,
			max_attempts = VALUES(max_attempts),
			alert_json = VALUES(alert_json),
			last_error = '',
			next_retry_at = NOW(),
			delivered_at = NULL,
			updated_at = NOW(),
			deleted_at = NULL
	`, notificationKey, alertKey, status.TenantID, channel, dashboard.SaaSAlertNotificationStatusPending, maxAttempts, string(raw))
	if err != nil && isMissingSaaSTableError(err) {
		return SaaSAlertNotification{}, nil
	}
	if err != nil {
		return SaaSAlertNotification{}, err
	}
	return s.saaSAlertNotificationByKey(ctx, notificationKey)
}

func (s *MySQLStore) ListDueSaaSAlertNotifications(ctx context.Context, limit int) ([]SaaSAlertNotification, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts,
			COALESCE(CAST(alert_json AS CHAR), ''), last_error,
			next_retry_at, delivered_at, created_at, updated_at
		FROM mochat_go_saas_alert_notifications
		WHERE deleted_at IS NULL
			AND status IN (?, ?)
			AND attempts < max_attempts
			AND (next_retry_at IS NULL OR next_retry_at <= NOW())
		ORDER BY next_retry_at IS NULL DESC, next_retry_at ASC, id ASC
		LIMIT ?
	`, dashboard.SaaSAlertNotificationStatusPending, dashboard.SaaSAlertNotificationStatusFailed, limit)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	notifications := []SaaSAlertNotification{}
	for rows.Next() {
		notification, err := scanSaaSAlertNotification(rows)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return notifications, nil
}

func (s *MySQLStore) SaaSAdminAlertNotifications(ctx context.Context, options dashboard.SaaSAdminAlertNotificationOptions) ([]SaaSAlertNotification, error) {
	if options.Limit <= 0 {
		options.Limit = 20
	}
	if options.Limit > 5000 {
		options.Limit = 5000
	}
	where, args := saasAdminAlertNotificationWhereSQL(options)
	args = append(args, options.Limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts,
			COALESCE(CAST(alert_json AS CHAR), ''), last_error,
			next_retry_at, delivered_at, created_at, updated_at
		FROM mochat_go_saas_alert_notifications
		`+where+`
		ORDER BY status IN ('failed', 'dead') DESC, next_retry_at IS NULL ASC, next_retry_at ASC, id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	notifications := []SaaSAlertNotification{}
	for rows.Next() {
		notification, err := scanSaaSAlertNotification(rows)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return notifications, nil
}

func (s *MySQLStore) SaaSAdminAlertNotificationSummary(ctx context.Context, options dashboard.SaaSAdminAlertNotificationOptions) (dashboard.SaaSAdminAlertNotificationSummary, error) {
	where, args := saasAdminAlertNotificationWhereSQL(options)
	queryArgs := []any{
		dashboard.SaaSAlertNotificationStatusPending,
		dashboard.SaaSAlertNotificationStatusFailed,
		dashboard.SaaSAlertNotificationStatusDelivered,
		dashboard.SaaSAlertNotificationStatusDead,
		dashboard.SaaSAlertNotificationStatusClosed,
		dashboard.SaaSAlertNotificationStatusSuppressed,
		dashboard.SaaSAlertNotificationStatusFailed,
		dashboard.SaaSAlertNotificationStatusDead,
	}
	queryArgs = append(queryArgs, args...)
	var summary dashboard.SaaSAdminAlertNotificationSummary
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status IN (?, ?) THEN 1 ELSE 0 END), 0),
			COUNT(DISTINCT NULLIF(tenant_id, 0)),
			COUNT(DISTINCT NULLIF(channel, ''))
		FROM mochat_go_saas_alert_notifications
		`+where,
		queryArgs...,
	).Scan(
		&summary.NotificationCount,
		&summary.PendingCount,
		&summary.FailedCount,
		&summary.DeliveredCount,
		&summary.DeadCount,
		&summary.ClosedCount,
		&summary.SuppressedCount,
		&summary.RetryableCount,
		&summary.TenantCount,
		&summary.ChannelCount,
	)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminAlertNotificationSummary{}, nil
		}
		return dashboard.SaaSAdminAlertNotificationSummary{}, err
	}
	return summary, nil
}

func (s *MySQLStore) SaaSAdminNotificationHealth(ctx context.Context, options dashboard.SaaSAdminNotificationHealthOptions) (dashboard.SaaSAdminNotificationHealthSource, error) {
	source := dashboard.SaaSAdminNotificationHealthSource{
		Tenants:        []dashboard.SaaSAdminNotificationHealthSnapshot{},
		FailureReasons: []dashboard.SaaSAdminNotificationHealthFailureSnapshot{},
	}
	channel := strings.TrimSpace(options.Channel)
	if channel == "" {
		channel = dashboard.SaaSAlertNotificationChannelWebhook
	}
	windowHours := options.WindowHours
	if windowHours <= 0 {
		windowHours = 24
	}
	staleMinutes := options.StaleMinutes
	if staleMinutes <= 0 {
		staleMinutes = 15
	}
	where := " WHERE t.deleted_at IS NULL"
	whereArgs := []any{}
	if options.TenantID > 0 {
		where += " AND t.id = ?"
		whereArgs = append(whereArgs, options.TenantID)
	}
	if options.ExcludedTenantID > 0 {
		where += " AND t.id <> ?"
		whereArgs = append(whereArgs, options.ExcludedTenantID)
	}
	if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		where += " AND (t.name LIKE ? OR CAST(t.id AS CHAR) LIKE ? OR tp.package_code LIKE ? OR tp.package_name LIKE ?)"
		whereArgs = append(whereArgs, like, like, like, like)
	}
	queryArgs := []any{staleMinutes, channel, channel, windowHours}
	queryArgs = append(queryArgs, whereArgs...)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			t.id,
			COALESCE(t.name, ''),
			COALESCE(t.status, 0),
			COALESCE(tp.package_code, ''),
			COALESCE(tp.package_name, ''),
			CASE WHEN s.id IS NULL THEN 0 ELSE 1 END,
			CASE WHEN s.id IS NOT NULL AND s.enabled = 1 THEN 1 ELSE 0 END,
			COUNT(n.id),
			COALESCE(SUM(CASE WHEN n.status = 'pending' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'pending' AND (n.next_retry_at IS NULL OR n.next_retry_at <= NOW()) THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'pending' AND n.next_retry_at > NOW() THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'pending' AND COALESCE(n.next_retry_at, n.created_at) <= DATE_SUB(NOW(), INTERVAL ? MINUTE) THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'delivered' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'dead' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'closed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'suppressed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(n.attempts), 0),
			COALESCE(AVG(CASE WHEN n.status = 'delivered' AND n.delivered_at IS NOT NULL THEN GREATEST(0, TIMESTAMPDIFF(SECOND, n.created_at, n.delivered_at)) END), 0),
			COALESCE(MAX(CASE WHEN n.status = 'delivered' AND n.delivered_at IS NOT NULL THEN GREATEST(0, TIMESTAMPDIFF(SECOND, n.created_at, n.delivered_at)) END), 0),
			COALESCE(DATE_FORMAT(MIN(CASE WHEN n.status = 'pending' THEN n.created_at END), '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(MAX(CASE WHEN n.status = 'delivered' THEN n.delivered_at END), '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(MAX(CASE WHEN n.status IN ('failed', 'dead') THEN n.updated_at END), '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(MAX(n.created_at), '%Y-%m-%d %H:%i:%s'), '')
		FROM mc_tenant t
		LEFT JOIN mochat_go_saas_tenant_packages tp ON tp.tenant_id = t.id AND tp.deleted_at IS NULL
		LEFT JOIN mochat_go_saas_alert_settings s ON s.tenant_id = t.id AND s.channel = ? AND s.deleted_at IS NULL
		LEFT JOIN mochat_go_saas_alert_notifications n ON n.tenant_id = t.id AND n.channel = ? AND n.created_at >= DATE_SUB(NOW(), INTERVAL ? HOUR) AND n.deleted_at IS NULL
		`+where+`
		GROUP BY t.id, t.name, t.status, tp.package_code, tp.package_name, s.id, s.enabled
		ORDER BY t.id ASC
	`, queryArgs...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return source, nil
		}
		return dashboard.SaaSAdminNotificationHealthSource{}, err
	}
	for rows.Next() {
		var item dashboard.SaaSAdminNotificationHealthSnapshot
		var policyConfigured, policyEnabled int
		if err := rows.Scan(
			&item.TenantID,
			&item.TenantName,
			&item.TenantStatus,
			&item.PackageCode,
			&item.PackageName,
			&policyConfigured,
			&policyEnabled,
			&item.NotificationCount,
			&item.PendingCount,
			&item.ReadyPendingCount,
			&item.DeferredCount,
			&item.StalePendingCount,
			&item.FailedCount,
			&item.DeliveredCount,
			&item.DeadCount,
			&item.ClosedCount,
			&item.SuppressedCount,
			&item.TotalAttempts,
			&item.AverageDeliverySeconds,
			&item.MaxDeliverySeconds,
			&item.OldestPendingAt,
			&item.LastDeliveredAt,
			&item.LastFailureAt,
			&item.LatestNotificationAt,
		); err != nil {
			rows.Close()
			return dashboard.SaaSAdminNotificationHealthSource{}, err
		}
		item.PolicyConfigured = policyConfigured == 1
		item.PolicyEnabled = policyEnabled == 1
		source.Tenants = append(source.Tenants, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return dashboard.SaaSAdminNotificationHealthSource{}, err
	}
	rows.Close()

	failureRows, err := s.db.QueryContext(ctx, `
		SELECT
			n.tenant_id,
			LEFT(TRIM(n.last_error), 160),
			COUNT(*),
			COALESCE(DATE_FORMAT(MAX(n.updated_at), '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_alert_notifications n
		JOIN mc_tenant t ON t.id = n.tenant_id
		LEFT JOIN mochat_go_saas_tenant_packages tp ON tp.tenant_id = t.id AND tp.deleted_at IS NULL
		`+where+`
			AND n.deleted_at IS NULL
			AND n.channel = ?
			AND n.created_at >= DATE_SUB(NOW(), INTERVAL ? HOUR)
			AND n.status IN ('failed', 'dead')
			AND TRIM(n.last_error) <> ''
		GROUP BY n.tenant_id, LEFT(TRIM(n.last_error), 160)
		ORDER BY COUNT(*) DESC, MAX(n.updated_at) DESC
		LIMIT 1000
	`, append(append([]any{}, whereArgs...), channel, windowHours)...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return source, nil
		}
		return dashboard.SaaSAdminNotificationHealthSource{}, err
	}
	defer failureRows.Close()
	for failureRows.Next() {
		var item dashboard.SaaSAdminNotificationHealthFailureSnapshot
		if err := failureRows.Scan(&item.TenantID, &item.Reason, &item.Count, &item.LastOccurredAt); err != nil {
			return dashboard.SaaSAdminNotificationHealthSource{}, err
		}
		source.FailureReasons = append(source.FailureReasons, item)
	}
	if err := failureRows.Err(); err != nil {
		return dashboard.SaaSAdminNotificationHealthSource{}, err
	}
	return source, nil
}

func (s *MySQLStore) SaaSAdminNotificationSLO(ctx context.Context, options dashboard.SaaSAdminNotificationSLOOptions) (dashboard.SaaSAdminNotificationSLOSource, error) {
	source := dashboard.SaaSAdminNotificationSLOSource{
		Days:    []dashboard.SaaSAdminNotificationSLODaySnapshot{},
		Tenants: []dashboard.SaaSAdminNotificationSLOTenantSnapshot{},
	}
	channel := strings.TrimSpace(options.Channel)
	if channel == "" {
		channel = dashboard.SaaSAlertNotificationChannelWebhook
	}
	days := options.Days
	if days <= 0 {
		days = 7
	}
	latencySeconds := options.LatencySecondsTarget
	if latencySeconds <= 0 {
		latencySeconds = 300
	}
	windowOffsetDays := days - 1
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			DATE_FORMAT(DATE_SUB(CURDATE(), INTERVAL ? DAY), '%Y-%m-%d'),
			DATE_FORMAT(CURDATE(), '%Y-%m-%d')
	`, windowOffsetDays).Scan(&source.WindowStartDate, &source.WindowEndDate); err != nil {
		return dashboard.SaaSAdminNotificationSLOSource{}, err
	}

	dailyWhere := `
		WHERE n.deleted_at IS NULL
			AND n.channel = ?
			AND n.created_at >= DATE_SUB(CURDATE(), INTERVAL ? DAY)
			AND n.created_at < DATE_ADD(CURDATE(), INTERVAL 1 DAY)
			AND t.deleted_at IS NULL`
	dailyArgs := []any{latencySeconds, channel, windowOffsetDays}
	if options.TenantID > 0 {
		dailyWhere += " AND t.id = ?"
		dailyArgs = append(dailyArgs, options.TenantID)
	}
	if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		dailyWhere += " AND (t.name LIKE ? OR CAST(t.id AS CHAR) LIKE ? OR tp.package_code LIKE ? OR tp.package_name LIKE ?)"
		dailyArgs = append(dailyArgs, like, like, like, like)
	}
	dailyRows, err := s.db.QueryContext(ctx, `
		SELECT
			DATE_FORMAT(DATE(n.created_at), '%Y-%m-%d'),
			COUNT(DISTINCT n.tenant_id),
			COUNT(n.id),
			COALESCE(SUM(CASE WHEN n.status = 'pending' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'delivered' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'dead' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'closed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'suppressed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'delivered' AND n.delivered_at IS NOT NULL AND GREATEST(0, TIMESTAMPDIFF(SECOND, n.created_at, n.delivered_at)) <= ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(n.attempts), 0),
			COALESCE(AVG(CASE WHEN n.status = 'delivered' AND n.delivered_at IS NOT NULL THEN GREATEST(0, TIMESTAMPDIFF(SECOND, n.created_at, n.delivered_at)) END), 0),
			COALESCE(MAX(CASE WHEN n.status = 'delivered' AND n.delivered_at IS NOT NULL THEN GREATEST(0, TIMESTAMPDIFF(SECOND, n.created_at, n.delivered_at)) END), 0)
		FROM mochat_go_saas_alert_notifications n
		JOIN mc_tenant t ON t.id = n.tenant_id
		LEFT JOIN mochat_go_saas_tenant_packages tp ON tp.tenant_id = t.id AND tp.deleted_at IS NULL
		`+dailyWhere+`
		GROUP BY DATE(n.created_at)
		ORDER BY DATE(n.created_at) ASC
	`, dailyArgs...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return source, nil
		}
		return dashboard.SaaSAdminNotificationSLOSource{}, err
	}
	for dailyRows.Next() {
		var item dashboard.SaaSAdminNotificationSLODaySnapshot
		if err := dailyRows.Scan(
			&item.Day,
			&item.TenantCount,
			&item.NotificationCount,
			&item.PendingCount,
			&item.DeliveredCount,
			&item.FailedCount,
			&item.DeadCount,
			&item.ClosedCount,
			&item.SuppressedCount,
			&item.DeliveredWithinTarget,
			&item.TotalAttempts,
			&item.AverageDeliverySeconds,
			&item.MaxDeliverySeconds,
		); err != nil {
			dailyRows.Close()
			return dashboard.SaaSAdminNotificationSLOSource{}, err
		}
		source.Days = append(source.Days, item)
	}
	if err := dailyRows.Err(); err != nil {
		dailyRows.Close()
		return dashboard.SaaSAdminNotificationSLOSource{}, err
	}
	dailyRows.Close()

	tenantWhere := " WHERE t.deleted_at IS NULL"
	tenantWhereArgs := []any{}
	if options.TenantID > 0 {
		tenantWhere += " AND t.id = ?"
		tenantWhereArgs = append(tenantWhereArgs, options.TenantID)
	}
	if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		tenantWhere += " AND (t.name LIKE ? OR CAST(t.id AS CHAR) LIKE ? OR tp.package_code LIKE ? OR tp.package_name LIKE ?)"
		tenantWhereArgs = append(tenantWhereArgs, like, like, like, like)
	}
	tenantArgs := []any{latencySeconds, channel, windowOffsetDays}
	tenantArgs = append(tenantArgs, tenantWhereArgs...)
	tenantRows, err := s.db.QueryContext(ctx, `
		SELECT
			t.id,
			COALESCE(t.name, ''),
			COALESCE(t.status, 0),
			COALESCE(tp.package_code, ''),
			COALESCE(tp.package_name, ''),
			COUNT(n.id),
			COALESCE(SUM(CASE WHEN n.status = 'pending' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'delivered' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'dead' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'closed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'suppressed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN n.status = 'delivered' AND n.delivered_at IS NOT NULL AND GREATEST(0, TIMESTAMPDIFF(SECOND, n.created_at, n.delivered_at)) <= ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(n.attempts), 0),
			COALESCE(AVG(CASE WHEN n.status = 'delivered' AND n.delivered_at IS NOT NULL THEN GREATEST(0, TIMESTAMPDIFF(SECOND, n.created_at, n.delivered_at)) END), 0),
			COALESCE(MAX(CASE WHEN n.status = 'delivered' AND n.delivered_at IS NOT NULL THEN GREATEST(0, TIMESTAMPDIFF(SECOND, n.created_at, n.delivered_at)) END), 0)
		FROM mc_tenant t
		LEFT JOIN mochat_go_saas_tenant_packages tp ON tp.tenant_id = t.id AND tp.deleted_at IS NULL
		LEFT JOIN mochat_go_saas_alert_notifications n
			ON n.tenant_id = t.id
			AND n.channel = ?
			AND n.created_at >= DATE_SUB(CURDATE(), INTERVAL ? DAY)
			AND n.created_at < DATE_ADD(CURDATE(), INTERVAL 1 DAY)
			AND n.deleted_at IS NULL
		`+tenantWhere+`
		GROUP BY t.id, t.name, t.status, tp.package_code, tp.package_name
		ORDER BY t.id ASC
	`, tenantArgs...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return source, nil
		}
		return dashboard.SaaSAdminNotificationSLOSource{}, err
	}
	defer tenantRows.Close()
	for tenantRows.Next() {
		var item dashboard.SaaSAdminNotificationSLOTenantSnapshot
		if err := tenantRows.Scan(
			&item.TenantID,
			&item.TenantName,
			&item.TenantStatus,
			&item.PackageCode,
			&item.PackageName,
			&item.NotificationCount,
			&item.PendingCount,
			&item.DeliveredCount,
			&item.FailedCount,
			&item.DeadCount,
			&item.ClosedCount,
			&item.SuppressedCount,
			&item.DeliveredWithinTarget,
			&item.TotalAttempts,
			&item.AverageDeliverySeconds,
			&item.MaxDeliverySeconds,
		); err != nil {
			return dashboard.SaaSAdminNotificationSLOSource{}, err
		}
		source.Tenants = append(source.Tenants, item)
	}
	if err := tenantRows.Err(); err != nil {
		return dashboard.SaaSAdminNotificationSLOSource{}, err
	}
	return source, nil
}

func saasAdminAlertNotificationWhereSQL(options dashboard.SaaSAdminAlertNotificationOptions) (string, []any) {
	args := []any{}
	where := "WHERE deleted_at IS NULL"
	if options.TenantID > 0 {
		where += " AND tenant_id = ?"
		args = append(args, options.TenantID)
	}
	if options.ExcludedTenantID > 0 {
		where += " AND tenant_id <> ?"
		args = append(args, options.ExcludedTenantID)
	}
	if status := strings.TrimSpace(options.Status); status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if channel := strings.TrimSpace(options.Channel); channel != "" {
		where += " AND channel = ?"
		args = append(args, channel)
	}
	if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		where += " AND (notification_key LIKE ? OR alert_key LIKE ? OR channel LIKE ? OR status LIKE ? OR last_error LIKE ? OR CAST(alert_json AS CHAR) LIKE ?)"
		args = append(args, like, like, like, like, like, like)
	}
	return where, args
}

func (s *MySQLStore) RetrySaaSAdminAlertNotification(ctx context.Context, retry dashboard.SaaSAdminAlertNotificationRetry) (dashboard.SaaSAdminAlertNotificationRetryResult, error) {
	if retry.NotificationID <= 0 {
		return dashboard.SaaSAdminAlertNotificationRetryResult{}, dashboard.NewSaaSAdminBadRequest("notificationId required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminAlertNotificationRetryResult{}, err
	}
	defer rollbackQuietly(tx)

	args := []any{retry.NotificationID}
	where := "WHERE id = ? AND deleted_at IS NULL"
	if retry.AllowedTenantID > 0 {
		where += " AND tenant_id = ?"
		args = append(args, retry.AllowedTenantID)
	}
	row := tx.QueryRowContext(ctx, `
		SELECT id, notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts,
			COALESCE(CAST(alert_json AS CHAR), ''), last_error,
			next_retry_at, delivered_at, created_at, updated_at
		FROM mochat_go_saas_alert_notifications
		`+where+`
		LIMIT 1
		FOR UPDATE
	`, args...)
	notification, err := scanSaaSAlertNotification(row)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminAlertNotificationRetryResult{}, dashboard.NewSaaSAdminNotFound("notification not found")
	}
	if err != nil && isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminAlertNotificationRetryResult{}, nil
	}
	if err != nil {
		return dashboard.SaaSAdminAlertNotificationRetryResult{}, err
	}
	previousStatus := notification.Status
	if previousStatus == dashboard.SaaSAlertNotificationStatusDelivered {
		return dashboard.SaaSAdminAlertNotificationRetryResult{}, dashboard.NewSaaSAdminBadRequest("delivered notification cannot be retried")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_alert_notifications
		SET status = ?, attempts = 0, last_error = '', next_retry_at = NOW(), delivered_at = NULL, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, dashboard.SaaSAlertNotificationStatusPending, notification.ID); err != nil {
		return dashboard.SaaSAdminAlertNotificationRetryResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID:      notification.TenantID,
		ActorUserID:   retry.ActorUserID,
		ActorTenantID: retry.ActorTenantID,
		Action:        "tenant.notification.retry",
		TargetType:    "alert_notification",
		TargetID:      strconv.FormatInt(notification.ID, 10),
		TargetName:    notification.NotificationKey,
		BeforeJSON:    saasAdminMarshalJSON(notification),
		AfterJSON: saasAdminMarshalJSON(map[string]any{
			"status":      dashboard.SaaSAlertNotificationStatusPending,
			"attempts":    0,
			"nextRetryAt": "now",
		}),
		Remark: strings.TrimSpace(retry.Remark),
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminAlertNotificationRetryResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminAlertNotificationRetryResult{}, err
	}
	return dashboard.SaaSAdminAlertNotificationRetryResult{
		Retried:         true,
		NotificationID:  notification.ID,
		NotificationKey: notification.NotificationKey,
		TenantID:        notification.TenantID,
		AlertKey:        notification.AlertKey,
		Channel:         notification.Channel,
		PreviousStatus:  previousStatus,
		Status:          dashboard.SaaSAlertNotificationStatusPending,
		Attempts:        0,
		MaxAttempts:     notification.MaxAttempts,
		Remark:          strings.TrimSpace(retry.Remark),
		OperationID:     operationID,
	}, nil
}

func (s *MySQLStore) BulkRetrySaaSAdminAlertNotifications(ctx context.Context, retry dashboard.SaaSAdminAlertNotificationBulkRetry) (dashboard.SaaSAdminAlertNotificationBulkRetryResult, error) {
	if retry.Limit <= 0 {
		retry.Limit = 50
	}
	if retry.Limit > 100 {
		retry.Limit = 100
	}
	if retry.AllowedTenantID > 0 {
		if retry.TenantID > 0 && retry.TenantID != retry.AllowedTenantID {
			return dashboard.SaaSAdminAlertNotificationBulkRetryResult{}, dashboard.NewSaaSAdminNotFound("notification not found")
		}
		retry.TenantID = retry.AllowedTenantID
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminAlertNotificationBulkRetryResult{}, err
	}
	defer rollbackQuietly(tx)

	args := []any{}
	where := "WHERE deleted_at IS NULL"
	switch strings.TrimSpace(retry.Status) {
	case dashboard.SaaSAlertNotificationStatusFailed, dashboard.SaaSAlertNotificationStatusDead:
		where += " AND status = ?"
		args = append(args, strings.TrimSpace(retry.Status))
	default:
		where += " AND status IN (?, ?)"
		args = append(args, dashboard.SaaSAlertNotificationStatusFailed, dashboard.SaaSAlertNotificationStatusDead)
	}
	if retry.TenantID > 0 {
		where += " AND tenant_id = ?"
		args = append(args, retry.TenantID)
	}
	if channel := strings.TrimSpace(retry.Channel); channel != "" {
		where += " AND channel = ?"
		args = append(args, channel)
	}
	if keyword := strings.TrimSpace(retry.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		where += " AND (notification_key LIKE ? OR alert_key LIKE ? OR channel LIKE ? OR status LIKE ? OR last_error LIKE ? OR CAST(alert_json AS CHAR) LIKE ?)"
		args = append(args, like, like, like, like, like, like)
	}
	args = append(args, retry.Limit)
	rows, err := tx.QueryContext(ctx, `
		SELECT id, notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts,
			COALESCE(CAST(alert_json AS CHAR), ''), last_error,
			next_retry_at, delivered_at, created_at, updated_at
		FROM mochat_go_saas_alert_notifications
		`+where+`
		ORDER BY status = 'dead' DESC, attempts DESC, next_retry_at IS NULL ASC, next_retry_at ASC, id ASC
		LIMIT ?
		FOR UPDATE
	`, args...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminAlertNotificationBulkRetryResult{}, nil
		}
		return dashboard.SaaSAdminAlertNotificationBulkRetryResult{}, err
	}
	defer rows.Close()

	notifications := []SaaSAlertNotification{}
	for rows.Next() {
		notification, err := scanSaaSAlertNotification(rows)
		if err != nil {
			return dashboard.SaaSAdminAlertNotificationBulkRetryResult{}, err
		}
		notifications = append(notifications, notification)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSAdminAlertNotificationBulkRetryResult{}, err
	}
	if err := rows.Close(); err != nil {
		return dashboard.SaaSAdminAlertNotificationBulkRetryResult{}, err
	}

	results := make([]dashboard.SaaSAdminAlertNotificationRetryResult, 0, len(notifications))
	remark := strings.TrimSpace(retry.Remark)
	for _, notification := range notifications {
		previousStatus := notification.Status
		if _, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_alert_notifications
			SET status = ?, attempts = 0, last_error = '', next_retry_at = NOW(), delivered_at = NULL, updated_at = NOW()
			WHERE id = ? AND deleted_at IS NULL
		`, dashboard.SaaSAlertNotificationStatusPending, notification.ID); err != nil {
			return dashboard.SaaSAdminAlertNotificationBulkRetryResult{}, err
		}
		operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
			TenantID:      notification.TenantID,
			ActorUserID:   retry.ActorUserID,
			ActorTenantID: retry.ActorTenantID,
			Action:        "tenant.notification.retry",
			TargetType:    "alert_notification",
			TargetID:      strconv.FormatInt(notification.ID, 10),
			TargetName:    notification.NotificationKey,
			BeforeJSON:    saasAdminMarshalJSON(notification),
			AfterJSON: saasAdminMarshalJSON(map[string]any{
				"status":      dashboard.SaaSAlertNotificationStatusPending,
				"attempts":    0,
				"nextRetryAt": "now",
				"bulkRetry":   true,
			}),
			Remark: remark,
		})
		if err != nil && !isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminAlertNotificationBulkRetryResult{}, err
		}
		results = append(results, dashboard.SaaSAdminAlertNotificationRetryResult{
			Retried:         true,
			NotificationID:  notification.ID,
			NotificationKey: notification.NotificationKey,
			TenantID:        notification.TenantID,
			AlertKey:        notification.AlertKey,
			Channel:         notification.Channel,
			PreviousStatus:  previousStatus,
			Status:          dashboard.SaaSAlertNotificationStatusPending,
			Attempts:        0,
			MaxAttempts:     notification.MaxAttempts,
			Remark:          remark,
			OperationID:     operationID,
		})
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminAlertNotificationBulkRetryResult{}, err
	}
	return dashboard.SaaSAdminAlertNotificationBulkRetryResult{
		RetriedCount:  len(results),
		TenantID:      retry.TenantID,
		Status:        strings.TrimSpace(retry.Status),
		Channel:       strings.TrimSpace(retry.Channel),
		Keyword:       strings.TrimSpace(retry.Keyword),
		Limit:         retry.Limit,
		Remark:        remark,
		Notifications: results,
	}, nil
}

func (s *MySQLStore) CloseSaaSAdminAlertNotification(ctx context.Context, closeReq dashboard.SaaSAdminAlertNotificationClose) (dashboard.SaaSAdminAlertNotificationCloseResult, error) {
	if closeReq.NotificationID <= 0 {
		return dashboard.SaaSAdminAlertNotificationCloseResult{}, dashboard.NewSaaSAdminBadRequest("notificationId required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminAlertNotificationCloseResult{}, err
	}
	defer rollbackQuietly(tx)

	args := []any{closeReq.NotificationID}
	where := "WHERE id = ? AND deleted_at IS NULL"
	if closeReq.AllowedTenantID > 0 {
		where += " AND tenant_id = ?"
		args = append(args, closeReq.AllowedTenantID)
	}
	row := tx.QueryRowContext(ctx, `
		SELECT id, notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts,
			COALESCE(CAST(alert_json AS CHAR), ''), last_error,
			next_retry_at, delivered_at, created_at, updated_at
		FROM mochat_go_saas_alert_notifications
		`+where+`
		LIMIT 1
		FOR UPDATE
	`, args...)
	notification, err := scanSaaSAlertNotification(row)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminAlertNotificationCloseResult{}, dashboard.NewSaaSAdminNotFound("notification not found")
	}
	if err != nil && isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminAlertNotificationCloseResult{}, nil
	}
	if err != nil {
		return dashboard.SaaSAdminAlertNotificationCloseResult{}, err
	}
	previousStatus := notification.Status
	switch previousStatus {
	case dashboard.SaaSAlertNotificationStatusDelivered:
		return dashboard.SaaSAdminAlertNotificationCloseResult{}, dashboard.NewSaaSAdminBadRequest("delivered notification cannot be closed")
	case dashboard.SaaSAlertNotificationStatusDead, dashboard.SaaSAlertNotificationStatusClosed:
		return dashboard.SaaSAdminAlertNotificationCloseResult{}, dashboard.NewSaaSAdminBadRequest("notification already closed")
	}
	remark := strings.TrimSpace(closeReq.Remark)
	if remark == "" {
		remark = "关闭通知"
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_alert_notifications
		SET status = ?, last_error = ?, next_retry_at = NULL, delivered_at = NULL, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, dashboard.SaaSAlertNotificationStatusClosed, remark, notification.ID); err != nil {
		return dashboard.SaaSAdminAlertNotificationCloseResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID:      notification.TenantID,
		ActorUserID:   closeReq.ActorUserID,
		ActorTenantID: closeReq.ActorTenantID,
		Action:        dashboard.SaaSAdminOperationActionNotificationClose,
		TargetType:    dashboard.SaaSAdminOperationTargetAlertNotification,
		TargetID:      strconv.FormatInt(notification.ID, 10),
		TargetName:    notification.NotificationKey,
		BeforeJSON:    saasAdminMarshalJSON(notification),
		AfterJSON: saasAdminMarshalJSON(map[string]any{
			"status":      dashboard.SaaSAlertNotificationStatusClosed,
			"previous":    previousStatus,
			"nextRetryAt": "",
		}),
		Remark: remark,
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminAlertNotificationCloseResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminAlertNotificationCloseResult{}, err
	}
	return dashboard.SaaSAdminAlertNotificationCloseResult{
		Closed:          true,
		NotificationID:  notification.ID,
		NotificationKey: notification.NotificationKey,
		TenantID:        notification.TenantID,
		AlertKey:        notification.AlertKey,
		Channel:         notification.Channel,
		PreviousStatus:  previousStatus,
		Status:          dashboard.SaaSAlertNotificationStatusClosed,
		Attempts:        notification.Attempts,
		MaxAttempts:     notification.MaxAttempts,
		Remark:          remark,
		OperationID:     operationID,
	}, nil
}

func (s *MySQLStore) BulkCloseSaaSAdminAlertNotifications(ctx context.Context, closeReq dashboard.SaaSAdminAlertNotificationBulkClose) (dashboard.SaaSAdminAlertNotificationBulkCloseResult, error) {
	if closeReq.Limit <= 0 {
		closeReq.Limit = 50
	}
	if closeReq.Limit > 100 {
		closeReq.Limit = 100
	}
	if closeReq.AllowedTenantID > 0 {
		if closeReq.TenantID > 0 && closeReq.TenantID != closeReq.AllowedTenantID {
			return dashboard.SaaSAdminAlertNotificationBulkCloseResult{}, dashboard.NewSaaSAdminNotFound("notification not found")
		}
		closeReq.TenantID = closeReq.AllowedTenantID
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminAlertNotificationBulkCloseResult{}, err
	}
	defer rollbackQuietly(tx)

	args := []any{}
	where := "WHERE deleted_at IS NULL"
	switch strings.TrimSpace(closeReq.Status) {
	case dashboard.SaaSAlertNotificationStatusPending, dashboard.SaaSAlertNotificationStatusFailed:
		where += " AND status = ?"
		args = append(args, strings.TrimSpace(closeReq.Status))
	default:
		where += " AND status IN (?, ?)"
		args = append(args, dashboard.SaaSAlertNotificationStatusPending, dashboard.SaaSAlertNotificationStatusFailed)
	}
	if closeReq.TenantID > 0 {
		where += " AND tenant_id = ?"
		args = append(args, closeReq.TenantID)
	}
	if channel := strings.TrimSpace(closeReq.Channel); channel != "" {
		where += " AND channel = ?"
		args = append(args, channel)
	}
	if keyword := strings.TrimSpace(closeReq.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		where += " AND (notification_key LIKE ? OR alert_key LIKE ? OR channel LIKE ? OR status LIKE ? OR last_error LIKE ? OR CAST(alert_json AS CHAR) LIKE ?)"
		args = append(args, like, like, like, like, like, like)
	}
	args = append(args, closeReq.Limit)
	rows, err := tx.QueryContext(ctx, `
		SELECT id, notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts,
			COALESCE(CAST(alert_json AS CHAR), ''), last_error,
			next_retry_at, delivered_at, created_at, updated_at
		FROM mochat_go_saas_alert_notifications
		`+where+`
		ORDER BY status = 'failed' DESC, attempts DESC, next_retry_at IS NULL ASC, next_retry_at ASC, id ASC
		LIMIT ?
		FOR UPDATE
	`, args...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminAlertNotificationBulkCloseResult{}, nil
		}
		return dashboard.SaaSAdminAlertNotificationBulkCloseResult{}, err
	}
	defer rows.Close()

	notifications := []SaaSAlertNotification{}
	for rows.Next() {
		notification, err := scanSaaSAlertNotification(rows)
		if err != nil {
			return dashboard.SaaSAdminAlertNotificationBulkCloseResult{}, err
		}
		notifications = append(notifications, notification)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSAdminAlertNotificationBulkCloseResult{}, err
	}
	if err := rows.Close(); err != nil {
		return dashboard.SaaSAdminAlertNotificationBulkCloseResult{}, err
	}

	results := make([]dashboard.SaaSAdminAlertNotificationCloseResult, 0, len(notifications))
	remark := strings.TrimSpace(closeReq.Remark)
	if remark == "" {
		remark = "关闭通知"
	}
	for _, notification := range notifications {
		previousStatus := notification.Status
		if _, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_alert_notifications
			SET status = ?, last_error = ?, next_retry_at = NULL, delivered_at = NULL, updated_at = NOW()
			WHERE id = ? AND deleted_at IS NULL
		`, dashboard.SaaSAlertNotificationStatusClosed, remark, notification.ID); err != nil {
			return dashboard.SaaSAdminAlertNotificationBulkCloseResult{}, err
		}
		operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
			TenantID:      notification.TenantID,
			ActorUserID:   closeReq.ActorUserID,
			ActorTenantID: closeReq.ActorTenantID,
			Action:        dashboard.SaaSAdminOperationActionNotificationClose,
			TargetType:    dashboard.SaaSAdminOperationTargetAlertNotification,
			TargetID:      strconv.FormatInt(notification.ID, 10),
			TargetName:    notification.NotificationKey,
			BeforeJSON:    saasAdminMarshalJSON(notification),
			AfterJSON: saasAdminMarshalJSON(map[string]any{
				"status":      dashboard.SaaSAlertNotificationStatusClosed,
				"previous":    previousStatus,
				"nextRetryAt": "",
				"bulkClose":   true,
			}),
			Remark: remark,
		})
		if err != nil && !isMissingSaaSTableError(err) {
			return dashboard.SaaSAdminAlertNotificationBulkCloseResult{}, err
		}
		results = append(results, dashboard.SaaSAdminAlertNotificationCloseResult{
			Closed:          true,
			NotificationID:  notification.ID,
			NotificationKey: notification.NotificationKey,
			TenantID:        notification.TenantID,
			AlertKey:        notification.AlertKey,
			Channel:         notification.Channel,
			PreviousStatus:  previousStatus,
			Status:          dashboard.SaaSAlertNotificationStatusClosed,
			Attempts:        notification.Attempts,
			MaxAttempts:     notification.MaxAttempts,
			Remark:          remark,
			OperationID:     operationID,
		})
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminAlertNotificationBulkCloseResult{}, err
	}
	return dashboard.SaaSAdminAlertNotificationBulkCloseResult{
		ClosedCount:   len(results),
		TenantID:      closeReq.TenantID,
		Status:        strings.TrimSpace(closeReq.Status),
		Channel:       strings.TrimSpace(closeReq.Channel),
		Keyword:       strings.TrimSpace(closeReq.Keyword),
		Limit:         closeReq.Limit,
		Remark:        remark,
		Notifications: results,
	}, nil
}

func (s *MySQLStore) MarkSaaSAlertNotificationDelivered(ctx context.Context, id int64) error {
	if id <= 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_saas_alert_notifications
		SET status = ?, attempts = attempts + 1, last_error = '', next_retry_at = NULL, delivered_at = NOW(), updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, dashboard.SaaSAlertNotificationStatusDelivered, id)
	if err != nil && isMissingSaaSTableError(err) {
		return nil
	}
	return err
}

func (s *MySQLStore) MarkSaaSAlertNotificationFailed(ctx context.Context, id int64, retryDelay time.Duration, lastError string) (string, error) {
	if id <= 0 {
		return "", nil
	}
	if retryDelay < 0 {
		retryDelay = 0
	}
	nextRetryAt := time.Now().Add(retryDelay).Format("2006-01-02 15:04:05")
	_, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_saas_alert_notifications
		SET attempts = attempts + 1,
			status = CASE WHEN attempts + 1 >= max_attempts THEN ? ELSE ? END,
			last_error = ?,
			next_retry_at = CASE WHEN attempts + 1 >= max_attempts THEN NULL ELSE ? END,
			updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, dashboard.SaaSAlertNotificationStatusDead, dashboard.SaaSAlertNotificationStatusFailed, truncateRunes(lastError, 255), nextRetryAt, id)
	if err != nil && isMissingSaaSTableError(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var status string
	if err := s.db.QueryRowContext(ctx, `
		SELECT status FROM mochat_go_saas_alert_notifications WHERE id = ? AND deleted_at IS NULL
	`, id).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) || isMissingSaaSTableError(err) {
			return "", nil
		}
		return "", err
	}
	return status, nil
}

func (s *MySQLStore) DeferSaaSAlertNotification(ctx context.Context, id int64, nextRetryAt time.Time, reason string) error {
	if id <= 0 {
		return nil
	}
	if nextRetryAt.IsZero() || !nextRetryAt.After(time.Now()) {
		nextRetryAt = time.Now().Add(time.Minute)
	}
	retryAfter := time.Until(nextRetryAt)
	if retryAfter < time.Second {
		retryAfter = time.Minute
	}
	retrySeconds := int64((retryAfter + time.Second - 1) / time.Second)
	_, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_saas_alert_notifications
		SET status = ?, last_error = ?, next_retry_at = DATE_ADD(NOW(), INTERVAL ? SECOND), delivered_at = NULL, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL AND status IN (?, ?)
	`, dashboard.SaaSAlertNotificationStatusPending, truncateRunes(reason, 255), retrySeconds, id,
		dashboard.SaaSAlertNotificationStatusPending, dashboard.SaaSAlertNotificationStatusFailed)
	if err != nil && isMissingSaaSTableError(err) {
		return nil
	}
	return err
}

func (s *MySQLStore) SuppressSaaSAlertNotification(ctx context.Context, id int64, reason string) error {
	if id <= 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_saas_alert_notifications
		SET status = ?, last_error = ?, next_retry_at = NULL, delivered_at = NULL, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL AND status IN (?, ?)
	`, dashboard.SaaSAlertNotificationStatusSuppressed, truncateRunes(reason, 255), id,
		dashboard.SaaSAlertNotificationStatusPending, dashboard.SaaSAlertNotificationStatusFailed)
	if err != nil && isMissingSaaSTableError(err) {
		return nil
	}
	return err
}

func (s *MySQLStore) SaaSAlertNotificationDeliveryWindow(ctx context.Context, tenantID int, channel string, duration time.Duration) (dashboard.SaaSAlertNotificationDeliveryWindow, error) {
	deliveryWindow := dashboard.SaaSAlertNotificationDeliveryWindow{}
	if tenantID <= 0 {
		return deliveryWindow, nil
	}
	channel = strings.TrimSpace(channel)
	if channel == "" {
		channel = dashboard.SaaSAlertNotificationChannelWebhook
	}
	if duration < time.Second {
		duration = time.Hour
	}
	windowSeconds := int64((duration + time.Second - 1) / time.Second)
	var retryAfterSeconds int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COALESCE(GREATEST(0, ? - TIMESTAMPDIFF(SECOND, MIN(delivered_at), NOW())), 0)
		FROM mochat_go_saas_alert_notifications
		WHERE tenant_id = ? AND channel = ? AND status = ?
			AND delivered_at >= DATE_SUB(NOW(), INTERVAL ? SECOND)
			AND deleted_at IS NULL
			AND alert_key NOT LIKE '%:notification_policy:notification_policy_test:%'
	`, windowSeconds, tenantID, channel, dashboard.SaaSAlertNotificationStatusDelivered, windowSeconds).Scan(&deliveryWindow.DeliveredCount, &retryAfterSeconds)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return dashboard.SaaSAlertNotificationDeliveryWindow{}, nil
		}
		return dashboard.SaaSAlertNotificationDeliveryWindow{}, err
	}
	if retryAfterSeconds > 0 {
		deliveryWindow.NextAvailableAt = time.Now().Add(time.Duration(retryAfterSeconds) * time.Second)
	}
	return deliveryWindow, nil
}

func (s *MySQLStore) saaSAlertNotificationByKey(ctx context.Context, notificationKey string) (SaaSAlertNotification, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts,
			COALESCE(CAST(alert_json AS CHAR), ''), last_error,
			next_retry_at, delivered_at, created_at, updated_at
		FROM mochat_go_saas_alert_notifications
		WHERE notification_key = ? AND deleted_at IS NULL
	`, notificationKey)
	notification, err := scanSaaSAlertNotification(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isMissingSaaSTableError(err) {
			return SaaSAlertNotification{}, nil
		}
		return SaaSAlertNotification{}, err
	}
	return notification, nil
}

func (s *MySQLStore) SaaSAlertNotificationByKey(ctx context.Context, notificationKey string) (SaaSAlertNotification, error) {
	return s.saaSAlertNotificationByKey(ctx, notificationKey)
}

type sqlScanner interface {
	Scan(dest ...any) error
}

func scanSaaSAlertNotification(scanner sqlScanner) (SaaSAlertNotification, error) {
	var notification SaaSAlertNotification
	var alertJSON string
	var nextRetryAt, deliveredAt, createdAt, updatedAt sql.NullTime
	if err := scanner.Scan(
		&notification.ID,
		&notification.NotificationKey,
		&notification.AlertKey,
		&notification.TenantID,
		&notification.Channel,
		&notification.Status,
		&notification.Attempts,
		&notification.MaxAttempts,
		&alertJSON,
		&notification.LastError,
		&nextRetryAt,
		&deliveredAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return SaaSAlertNotification{}, err
	}
	if strings.TrimSpace(alertJSON) != "" {
		if err := json.Unmarshal([]byte(alertJSON), &notification.Alert); err != nil {
			return SaaSAlertNotification{}, fmt.Errorf("unmarshal SaaS alert notification: %w", err)
		}
	}
	notification.NextRetryAt = formatTime(nextRetryAt)
	notification.DeliveredAt = formatTime(deliveredAt)
	notification.CreatedAt = formatTime(createdAt)
	notification.UpdatedAt = formatTime(updatedAt)
	return notification, nil
}

func (s *MySQLStore) scanSaaSAlertSetting(scanner sqlScanner) (SaaSAlertSetting, error) {
	var setting SaaSAlertSetting
	var enabled, quietHoursEnabled int
	var allowedAlertTypesJSON, credentialCiphertext, credentialKeyID string
	var createdAt, updatedAt sql.NullTime
	if err := scanner.Scan(
		&setting.ID,
		&setting.TenantID,
		&setting.Channel,
		&enabled,
		&setting.WebhookURL,
		&setting.WebhookSecret,
		&credentialCiphertext,
		&credentialKeyID,
		&setting.WebhookTimeoutSeconds,
		&setting.WebhookRetryAttempts,
		&setting.WebhookRetryDelayMS,
		&setting.WebhookTitleTemplate,
		&setting.WebhookBodyTemplate,
		&setting.NotificationMaxAttempts,
		&setting.NotificationRetryDelaySeconds,
		&setting.MinimumSeverity,
		&allowedAlertTypesJSON,
		&quietHoursEnabled,
		&setting.QuietHoursStart,
		&setting.QuietHoursEnd,
		&setting.Timezone,
		&setting.HourlyLimit,
		&createdAt,
		&updatedAt,
	); err != nil {
		return SaaSAlertSetting{}, err
	}
	setting.Enabled = enabled != 0
	setting.QuietHoursEnabled = quietHoursEnabled != 0
	if err := json.Unmarshal([]byte(allowedAlertTypesJSON), &setting.AllowedAlertTypes); err != nil {
		return SaaSAlertSetting{}, fmt.Errorf("unmarshal SaaS alert setting alert types: %w", err)
	}
	setting.CreatedAt = formatTime(createdAt)
	setting.UpdatedAt = formatTime(updatedAt)
	setting, err := s.decodeSaaSAlertCredentials(setting, credentialCiphertext, credentialKeyID)
	if err != nil {
		return SaaSAlertSetting{}, err
	}
	return dashboard.NormalizeSaaSAlertSetting(setting), nil
}

func boolTinyInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (s *MySQLStore) tenantIDByCorpID(ctx context.Context, corpID int) (int, error) {
	var tenantID int
	err := s.db.QueryRowContext(ctx, `
		SELECT tenant_id
		FROM mc_corp
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, corpID).Scan(&tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return tenantID, err
}

func tenantIDByCorpIDTx(ctx context.Context, tx *sql.Tx, corpID int) (int, error) {
	var tenantID int
	err := tx.QueryRowContext(ctx, `
		SELECT tenant_id
		FROM mc_corp
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, corpID).Scan(&tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return tenantID, err
}

func (s *MySQLStore) TenantIDByCorpID(ctx context.Context, corpID int) (int, error) {
	return s.tenantIDByCorpID(ctx, corpID)
}

func (s *MySQLStore) enforceSaaSQuotaForTenant(ctx context.Context, tenantID int, metric string, additional int64) error {
	if tenantID <= 0 || additional <= 0 {
		return nil
	}
	status, err := s.SaaSQuotaStatus(ctx, tenantID, metric, additional)
	if err != nil {
		return err
	}
	if status.Exceeded() {
		return dashboard.NewSaaSQuotaExceededError(status)
	}
	return nil
}

func (s *MySQLStore) SaaSStorageQuotaStatus(ctx context.Context, tenantID int, additionalBytes int64) (dashboard.SaaSQuotaStatus, error) {
	status := dashboard.SaaSQuotaStatus{
		Metric:   dashboard.SaaSMetricStorage,
		TenantID: tenantID,
	}
	currentBytes, err := s.saasStorageBytes(ctx, tenantID)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return status, nil
		}
		return dashboard.SaaSQuotaStatus{}, err
	}
	currentMB := bytesToUsageMB(currentBytes)
	projectedMB := bytesToUsageMB(currentBytes + additionalBytes)
	status.Current = currentMB
	if projectedMB > currentMB {
		status.Additional = projectedMB - currentMB
	}
	limit, err := s.saasUsageLimit(ctx, tenantID, dashboard.SaaSMetricStorage)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return status, nil
		}
		return dashboard.SaaSQuotaStatus{}, err
	}
	status.Limit = limit
	return status, nil
}

func (s *MySQLStore) enforceSaaSProjectedQuotaForTenant(ctx context.Context, tenantID int, metric string, current int64, projected int64) error {
	if tenantID <= 0 || projected <= current {
		return nil
	}
	limit, err := s.saasUsageLimit(ctx, tenantID, metric)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return nil
		}
		return err
	}
	if limit > 0 && projected > limit {
		return dashboard.NewSaaSQuotaExceededError(dashboard.SaaSQuotaStatus{
			Metric:     metric,
			TenantID:   tenantID,
			Current:    current,
			Limit:      limit,
			Additional: projected - current,
		})
	}
	return nil
}

func (s *MySQLStore) RecordCommonUploadStorageObject(ctx context.Context, object dashboard.CommonUploadStorageObject) error {
	if object.TenantID <= 0 || object.SizeBytes <= 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_storage_objects
			(tenant_id, user_id, employee_id, corp_id, source, original_name, relative_path, content_type, size_bytes, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NULL)
	`, object.TenantID, object.UserID, object.EmployeeID, object.CorpID, strings.TrimSpace(object.Source), strings.TrimSpace(object.OriginalName), strings.TrimSpace(object.RelativePath), strings.TrimSpace(object.ContentType), object.SizeBytes)
	if err != nil && isMissingSaaSTableError(err) {
		return nil
	}
	return err
}

func (s *MySQLStore) MarkSaaSStorageObjectsDeleted(ctx context.Context, tenantID int, relativePaths []string) (int, error) {
	if tenantID <= 0 {
		return 0, nil
	}
	paths := uniqueStorageRelativePaths(relativePaths)
	if len(paths) == 0 {
		return 0, nil
	}
	args := make([]any, 0, len(paths)+1)
	args = append(args, tenantID)
	for _, path := range paths {
		args = append(args, path)
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_saas_storage_objects
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE tenant_id = ? AND deleted_at IS NULL AND relative_path IN (`+placeholders(len(paths))+`)
	`, args...)
	if err != nil && isMissingSaaSTableError(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

func (s *MySQLStore) reclaimSaaSStorageObjects(ctx context.Context, tenantID int, relativePaths []string) error {
	if tenantID <= 0 || len(relativePaths) == 0 {
		return nil
	}
	marked, err := s.MarkSaaSStorageObjectsDeleted(ctx, tenantID, relativePaths)
	if err != nil {
		return err
	}
	if marked == 0 {
		return nil
	}
	return s.RefreshSaaSUsageCounter(ctx, tenantID, dashboard.SaaSMetricStorage)
}

func (s *MySQLStore) ReconcileSaaSStorageObjects(ctx context.Context, storageRoot string, tenantID int) (SaaSStorageReconcileResult, error) {
	result := SaaSStorageReconcileResult{TenantID: tenantID}
	if strings.TrimSpace(storageRoot) == "" {
		storageRoot = "./storage/upload/static"
	}
	root, err := filepath.Abs(storageRoot)
	if err != nil {
		return result, err
	}
	query := `
		SELECT id, tenant_id, relative_path, size_bytes
		FROM mochat_go_saas_storage_objects
		WHERE deleted_at IS NULL
	`
	args := []any{}
	if tenantID > 0 {
		query += " AND tenant_id = ?"
		args = append(args, tenantID)
	}
	query += " ORDER BY id ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return result, nil
		}
		return result, err
	}
	defer rows.Close()

	touchedTenants := map[int]struct{}{}
	for rows.Next() {
		var id int64
		var rowTenantID int
		var relativePath string
		var sizeBytes int64
		if err := rows.Scan(&id, &rowTenantID, &relativePath, &sizeBytes); err != nil {
			return result, err
		}
		result.Scanned++
		target, ok := safeStorageObjectPath(root, relativePath)
		if !ok {
			if err := s.markStorageObjectDeleted(ctx, id); err != nil {
				return result, err
			}
			result.UnsafeMarked++
			touchedTenants[rowTenantID] = struct{}{}
			continue
		}
		info, err := os.Stat(target)
		if err != nil {
			if os.IsNotExist(err) {
				if err := s.markStorageObjectDeleted(ctx, id); err != nil {
					return result, err
				}
				result.MissingMarked++
				touchedTenants[rowTenantID] = struct{}{}
				continue
			}
			return result, err
		}
		if info.IsDir() {
			if err := s.markStorageObjectDeleted(ctx, id); err != nil {
				return result, err
			}
			result.MissingMarked++
			touchedTenants[rowTenantID] = struct{}{}
			continue
		}
		if info.Size() != sizeBytes {
			if err := s.updateStorageObjectSize(ctx, id, info.Size()); err != nil {
				return result, err
			}
			result.SizeUpdated++
			touchedTenants[rowTenantID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return result, err
	}

	tenants := make([]int, 0, len(touchedTenants))
	for id := range touchedTenants {
		if id > 0 {
			tenants = append(tenants, id)
		}
	}
	sort.Ints(tenants)
	for _, id := range tenants {
		if err := s.RefreshSaaSUsageCounter(ctx, id, dashboard.SaaSMetricStorage); err != nil {
			return result, err
		}
		result.CountersRefreshed++
	}
	result.RefreshedTenants = tenants
	return result, nil
}

func safeStorageObjectPath(root string, relativePath string) (string, bool) {
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" || strings.Contains(relativePath, "../") || strings.Contains(relativePath, `..\`) || filepath.IsAbs(relativePath) {
		return "", false
	}
	clean := filepath.Clean(filepath.FromSlash(relativePath))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false
	}
	target := filepath.Join(root, clean)
	if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
		return "", false
	}
	return target, true
}

func (s *MySQLStore) markStorageObjectDeleted(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_saas_storage_objects
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, id)
	if err != nil && isMissingSaaSTableError(err) {
		return nil
	}
	return err
}

func (s *MySQLStore) updateStorageObjectSize(ctx context.Context, id int64, sizeBytes int64) error {
	if sizeBytes < 0 {
		sizeBytes = 0
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_saas_storage_objects
		SET size_bytes = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, sizeBytes, id)
	if err != nil && isMissingSaaSTableError(err) {
		return nil
	}
	return err
}

func (s *MySQLStore) saasUsageLimit(ctx context.Context, tenantID int, metric string) (int64, error) {
	var limit int64
	err := s.db.QueryRowContext(ctx, `
		SELECT limit_value
		FROM mochat_go_saas_usage_counters
		WHERE tenant_id = ? AND metric = ? AND period_key = 'lifetime' AND deleted_at IS NULL
		LIMIT 1
	`, tenantID, metric).Scan(&limit)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return limit, err
}

func (s *MySQLStore) saasConfiguredUsageLimit(ctx context.Context, tenantID int, metric string) (int64, error) {
	limits, found, err := s.saasConfiguredUsageLimits(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	if found {
		return limits.limitFor(metric), nil
	}
	return s.saasUsageLimit(ctx, tenantID, metric)
}

func (s *MySQLStore) saasConfiguredUsageLimits(ctx context.Context, tenantID int) (saasUsageLimitSnapshot, bool, error) {
	var rawLimits sql.NullString
	var packageLimits saasUsageLimitSnapshot
	err := s.db.QueryRowContext(ctx, `
				SELECT COALESCE(CAST(tp.limits_json AS CHAR), ''), COALESCE(pkg.max_corps, 0), COALESCE(pkg.max_users, 0), COALESCE(pkg.max_contacts, 0),
					COALESCE(pkg.max_rooms, 0), COALESCE(pkg.max_agents, 0), COALESCE(pkg.channel_codes, 0), COALESCE(pkg.shop_codes, 0), COALESCE(pkg.radars, 0), COALESCE(pkg.lotteries, 0), COALESCE(pkg.room_infinite_pulls, 0), COALESCE(pkg.room_fissions, 0), COALESCE(pkg.room_clock_ins, 0),
					COALESCE(pkg.room_qualities, 0), COALESCE(pkg.room_calendars, 0), COALESCE(pkg.room_reminds, 0), COALESCE(pkg.contact_sops, 0), COALESCE(pkg.room_sops, 0), COALESCE(pkg.sensitive_words, 0), COALESCE(pkg.storage_mb, 0),
			COALESCE(pkg.contact_message_batches, 0), COALESCE(pkg.room_message_batches, 0), COALESCE(pkg.room_tag_pulls, 0),
			COALESCE(pkg.work_room_auto_pulls, 0), COALESCE(pkg.work_fissions, 0), COALESCE(pkg.official_accounts, 0), COALESCE(pkg.async_executions, 0)
		FROM mochat_go_saas_tenant_packages tp
		LEFT JOIN mochat_go_saas_packages pkg ON pkg.code = tp.package_code AND pkg.deleted_at IS NULL
		WHERE tp.tenant_id = ? AND tp.status = 1 AND tp.deleted_at IS NULL
		LIMIT 1
	`, tenantID).Scan(
		&rawLimits,
		&packageLimits.MaxCorps,
		&packageLimits.MaxUsers,
		&packageLimits.MaxContacts,
		&packageLimits.MaxRooms,
		&packageLimits.MaxAgents,
		&packageLimits.ChannelCodes,
		&packageLimits.ShopCodes,
		&packageLimits.Radars,
		&packageLimits.Lotteries,
		&packageLimits.RoomInfinitePulls,
		&packageLimits.RoomFissions,
		&packageLimits.RoomClockIns,
		&packageLimits.RoomQualities,
		&packageLimits.RoomCalendars,
		&packageLimits.RoomReminds,
		&packageLimits.ContactSOPs,
		&packageLimits.RoomSOPs,
		&packageLimits.SensitiveWords,
		&packageLimits.StorageMB,
		&packageLimits.ContactMessageBatches,
		&packageLimits.RoomMessageBatches,
		&packageLimits.RoomTagPulls,
		&packageLimits.WorkRoomAutoPulls,
		&packageLimits.WorkFissions,
		&packageLimits.OfficialAccounts,
		&packageLimits.AsyncExecutions,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return saasUsageLimitSnapshot{}, false, nil
	}
	if err != nil {
		return saasUsageLimitSnapshot{}, false, err
	}
	if rawLimits.Valid {
		raw := strings.TrimSpace(rawLimits.String)
		if raw != "" && raw != "null" {
			snapshot := packageLimits
			if err := json.Unmarshal([]byte(raw), &snapshot); err == nil {
				return snapshot, true, nil
			}
		}
	}
	return packageLimits, true, nil
}

func (l saasUsageLimitSnapshot) limitFor(metric string) int64 {
	switch metric {
	case dashboard.SaaSMetricCorps:
		return l.MaxCorps
	case dashboard.SaaSMetricUsers:
		return l.MaxUsers
	case dashboard.SaaSMetricContacts:
		return l.MaxContacts
	case dashboard.SaaSMetricRooms:
		return l.MaxRooms
	case dashboard.SaaSMetricAgents:
		return l.MaxAgents
	case dashboard.SaaSMetricChannelCodes:
		return l.ChannelCodes
	case dashboard.SaaSMetricShopCodes:
		return l.ShopCodes
	case dashboard.SaaSMetricRadars:
		return l.Radars
	case dashboard.SaaSMetricLotteries:
		return l.Lotteries
	case dashboard.SaaSMetricRoomInfinitePulls:
		return l.RoomInfinitePulls
	case dashboard.SaaSMetricRoomFissions:
		return l.RoomFissions
	case dashboard.SaaSMetricRoomClockIns:
		return l.RoomClockIns
	case dashboard.SaaSMetricRoomQualities:
		return l.RoomQualities
	case dashboard.SaaSMetricRoomCalendars:
		return l.RoomCalendars
	case dashboard.SaaSMetricRoomReminds:
		return l.RoomReminds
	case dashboard.SaaSMetricContactSOPs:
		return l.ContactSOPs
	case dashboard.SaaSMetricRoomSOPs:
		return l.RoomSOPs
	case dashboard.SaaSMetricSensitiveWords:
		return l.SensitiveWords
	case dashboard.SaaSMetricStorage:
		return l.StorageMB
	case dashboard.SaaSMetricContactMessageBatches:
		return l.ContactMessageBatches
	case dashboard.SaaSMetricRoomMessageBatches:
		return l.RoomMessageBatches
	case dashboard.SaaSMetricRoomTagPulls:
		return l.RoomTagPulls
	case dashboard.SaaSMetricWorkRoomAutoPulls:
		return l.WorkRoomAutoPulls
	case dashboard.SaaSMetricWorkFissions:
		return l.WorkFissions
	case dashboard.SaaSMetricOfficialAccounts:
		return l.OfficialAccounts
	case dashboard.SaaSMetricAsyncExecutions:
		return l.AsyncExecutions
	default:
		return 0
	}
}

func (s *MySQLStore) saasUsageTenantIDs(ctx context.Context, tenantID int) ([]int, error) {
	if tenantID > 0 {
		return []int{tenantID}, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM mc_tenant
		WHERE deleted_at IS NULL
		UNION
		SELECT tenant_id
		FROM mochat_go_saas_tenant_packages
		WHERE tenant_id > 0 AND deleted_at IS NULL
		UNION
		SELECT tenant_id
		FROM mochat_go_saas_usage_counters
		WHERE tenant_id > 0 AND deleted_at IS NULL
		ORDER BY 1 ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tenantIDs := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id > 0 {
			tenantIDs = append(tenantIDs, id)
		}
	}
	return tenantIDs, rows.Err()
}

func (s *MySQLStore) saasCurrentUsage(ctx context.Context, tenantID int, metric string) (int64, error) {
	var query string
	switch metric {
	case dashboard.SaaSMetricCorps:
		query = `
			SELECT COUNT(*)
			FROM mc_corp
			WHERE tenant_id = ? AND deleted_at IS NULL
		`
	case dashboard.SaaSMetricUsers:
		query = `
			SELECT COUNT(*)
			FROM mc_user
			WHERE tenant_id = ? AND deleted_at IS NULL
		`
	case dashboard.SaaSMetricContacts:
		query = `
			SELECT COUNT(*)
			FROM mc_work_contact contact
			JOIN mc_corp corp ON corp.id = contact.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND contact.deleted_at IS NULL
		`
	case dashboard.SaaSMetricRooms:
		query = `
			SELECT COUNT(*)
			FROM mc_work_room room
			JOIN mc_corp corp ON corp.id = room.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND room.deleted_at IS NULL
		`
	case dashboard.SaaSMetricAgents:
		query = `
			SELECT COUNT(*)
			FROM mc_work_agent agent
			JOIN mc_corp corp ON corp.id = agent.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND agent.deleted_at IS NULL
		`
	case dashboard.SaaSMetricChannelCodes:
		query = `
			SELECT COUNT(*)
			FROM mc_channel_code code
			JOIN mc_corp corp ON corp.id = code.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND code.deleted_at IS NULL
		`
	case dashboard.SaaSMetricShopCodes:
		query = `
			SELECT COUNT(*)
			FROM mc_shop_code code
			JOIN mc_corp corp ON corp.id = code.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND code.deleted_at IS NULL
		`
	case dashboard.SaaSMetricRadars:
		query = `
				SELECT COUNT(*)
				FROM mc_radar radar
				JOIN mc_corp corp ON corp.id = radar.corp_id
				WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND radar.deleted_at IS NULL
			`
	case dashboard.SaaSMetricLotteries:
		query = `
				SELECT COUNT(*)
				FROM mc_lottery lottery
				JOIN mc_corp corp ON corp.id = lottery.corp_id
				WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND lottery.deleted_at IS NULL
			`
	case dashboard.SaaSMetricRoomInfinitePulls:
		query = `
				SELECT COUNT(*)
				FROM mc_room_infinite activity
				JOIN mc_corp corp ON corp.id = activity.corp_id
				WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
			`
	case dashboard.SaaSMetricRoomFissions:
		query = `
				SELECT COUNT(*)
				FROM mc_room_fission activity
				JOIN mc_corp corp ON corp.id = activity.corp_id
				WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
			`
	case dashboard.SaaSMetricRoomClockIns:
		query = `
					SELECT COUNT(*)
					FROM mc_room_clock_in activity
					JOIN mc_corp corp ON corp.id = activity.corp_id
					WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
				`
	case dashboard.SaaSMetricRoomQualities:
		query = `
					SELECT COUNT(*)
					FROM mc_room_quality activity
					JOIN mc_corp corp ON corp.id = activity.corp_id
					WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
				`
	case dashboard.SaaSMetricRoomCalendars:
		query = `
					SELECT COUNT(*)
					FROM mc_room_calendar activity
					JOIN mc_corp corp ON corp.id = activity.corp_id
					WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
				`
	case dashboard.SaaSMetricRoomReminds:
		query = `
					SELECT COUNT(*)
					FROM mc_room_remind activity
					JOIN mc_corp corp ON corp.id = activity.corp_id
					WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
				`
	case dashboard.SaaSMetricContactSOPs:
		query = `
			SELECT COUNT(*)
			FROM mc_contact_sop sop
			JOIN mc_corp corp ON corp.id = sop.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL
		`
	case dashboard.SaaSMetricRoomSOPs:
		query = `
			SELECT COUNT(*)
			FROM mc_room_sop sop
			JOIN mc_corp corp ON corp.id = sop.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL
		`
	case dashboard.SaaSMetricSensitiveWords:
		query = `
			SELECT COUNT(*)
			FROM mc_sensitive_word word
			JOIN mc_corp corp ON corp.id = word.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND word.deleted_at IS NULL
		`
	case dashboard.SaaSMetricStorage:
		return s.saasStorageUsageMB(ctx, tenantID)
	case dashboard.SaaSMetricContactMessageBatches:
		query = `
			SELECT COUNT(*)
			FROM mc_contact_message_batch_send batch
			JOIN mc_corp corp ON corp.id = batch.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND batch.deleted_at IS NULL
		`
	case dashboard.SaaSMetricRoomMessageBatches:
		query = `
			SELECT COUNT(*)
			FROM mc_room_message_batch_send batch
			JOIN mc_corp corp ON corp.id = batch.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND batch.deleted_at IS NULL
		`
	case dashboard.SaaSMetricRoomTagPulls:
		query = `
			SELECT COUNT(*)
			FROM mc_room_tag_pull activity
			JOIN mc_corp corp ON corp.id = activity.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
		`
	case dashboard.SaaSMetricWorkRoomAutoPulls:
		query = `
			SELECT COUNT(*)
			FROM mc_work_room_auto_pull activity
			JOIN mc_corp corp ON corp.id = activity.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
		`
	case dashboard.SaaSMetricWorkFissions:
		query = `
			SELECT COUNT(*)
			FROM mc_work_fission activity
			JOIN mc_corp corp ON corp.id = activity.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
		`
	case dashboard.SaaSMetricOfficialAccounts:
		query = `
			SELECT COUNT(*)
			FROM mc_official_account account
			JOIN mc_corp corp ON corp.id = account.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND account.authorized_status <> 3 AND account.deleted_at IS NULL
		`
	case dashboard.SaaSMetricAsyncExecutions:
		return s.saasAsyncExecutionsUsage(ctx, tenantID)
	default:
		return 0, fmt.Errorf("unknown SaaS usage metric %q", metric)
	}
	var current int64
	if err := s.db.QueryRowContext(ctx, query, tenantID).Scan(&current); err != nil {
		return 0, err
	}
	return current, nil
}

func saasUsageMetrics() []string {
	return []string{
		dashboard.SaaSMetricCorps,
		dashboard.SaaSMetricUsers,
		dashboard.SaaSMetricContacts,
		dashboard.SaaSMetricRooms,
		dashboard.SaaSMetricAgents,
		dashboard.SaaSMetricChannelCodes,
		dashboard.SaaSMetricShopCodes,
		dashboard.SaaSMetricRadars,
		dashboard.SaaSMetricLotteries,
		dashboard.SaaSMetricRoomInfinitePulls,
		dashboard.SaaSMetricRoomFissions,
		dashboard.SaaSMetricRoomClockIns,
		dashboard.SaaSMetricRoomQualities,
		dashboard.SaaSMetricRoomCalendars,
		dashboard.SaaSMetricRoomReminds,
		dashboard.SaaSMetricContactSOPs,
		dashboard.SaaSMetricRoomSOPs,
		dashboard.SaaSMetricSensitiveWords,
		dashboard.SaaSMetricStorage,
		dashboard.SaaSMetricContactMessageBatches,
		dashboard.SaaSMetricRoomMessageBatches,
		dashboard.SaaSMetricRoomTagPulls,
		dashboard.SaaSMetricWorkRoomAutoPulls,
		dashboard.SaaSMetricWorkFissions,
		dashboard.SaaSMetricOfficialAccounts,
		dashboard.SaaSMetricAsyncExecutions,
	}
}

func (s *MySQLStore) saasAsyncExecutionsUsage(ctx context.Context, tenantID int) (int64, error) {
	exists, err := s.mysqlTableColumnExists(ctx, "mochat_go_background_task_executions", "tenant_id")
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, nil
	}
	var current int64
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mochat_go_background_task_executions
		WHERE tenant_id = ?
			AND kind = 'queue_item'
			AND status IN ('succeeded', 'failed', 'stopped')
	`, tenantID).Scan(&current)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return 0, nil
		}
		return 0, err
	}
	return current, nil
}

func (s *MySQLStore) mysqlTableColumnExists(ctx context.Context, table string, column string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?
	`, table, column).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *MySQLStore) saasStorageUsageMB(ctx context.Context, tenantID int) (int64, error) {
	sizeBytes, err := s.saasStorageBytes(ctx, tenantID)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return 0, nil
		}
		return 0, err
	}
	return bytesToUsageMB(sizeBytes), nil
}

func (s *MySQLStore) saasStorageBytes(ctx context.Context, tenantID int) (int64, error) {
	var sizeBytes sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(size_bytes), 0)
		FROM mochat_go_saas_storage_objects
		WHERE tenant_id = ? AND deleted_at IS NULL
	`, tenantID).Scan(&sizeBytes)
	if err != nil {
		return 0, err
	}
	if !sizeBytes.Valid {
		return 0, nil
	}
	return sizeBytes.Int64, nil
}

func bytesToUsageMB(sizeBytes int64) int64 {
	if sizeBytes <= 0 {
		return 0
	}
	const mib = int64(1024 * 1024)
	return (sizeBytes + mib - 1) / mib
}

func mediumStoragePathsFromContent(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	content := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &content); err != nil {
		return nil
	}
	return uniqueStorageRelativePaths([]string{
		stringFromAny(content["imagePath"]),
		stringFromAny(content["voicePath"]),
		stringFromAny(content["videoPath"]),
		stringFromAny(content["filePath"]),
	})
}

func batchSendStoragePathsFromContent(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	items := []dashboard.ContactMessageBatchSendContent{}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	paths := make([]string, 0, len(items))
	for _, item := range items {
		paths = append(paths, item.PicURL)
	}
	return uniqueStorageRelativePaths(paths)
}

func channelCodeWelcomeStoragePathsFromContent(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil
	}
	return uniqueStorageRelativePaths(appendChannelCodeWelcomeStoragePaths(nil, decoded, ""))
}

func appendChannelCodeWelcomeStoragePaths(paths []string, value any, key string) []string {
	switch typed := value.(type) {
	case map[string]any:
		for childKey, childValue := range typed {
			paths = appendChannelCodeWelcomeStoragePaths(paths, childValue, childKey)
		}
	case []any:
		for _, childValue := range typed {
			paths = appendChannelCodeWelcomeStoragePaths(paths, childValue, key)
		}
	case string:
		if channelCodeWelcomeStoragePathKey(key) {
			paths = appendChannelCodeWelcomeStoragePath(paths, typed)
		}
	}
	return paths
}

func channelCodeWelcomeStoragePathKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(strings.TrimSpace(key)))
	switch normalized {
	case "cover", "coverpic", "coverurl", "filepath", "image", "imagepath", "imageurl", "linkcover", "linkcoverurl", "linkpic", "logo", "mediapath", "pic", "picurl", "qrcode", "qrcodeurl", "qrcodepath", "qrpath", "qrurl", "thumb", "thumburl":
		return true
	default:
		return false
	}
}

func appendChannelCodeWelcomeStoragePath(paths []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return paths
	}
	if idx := strings.Index(value, "/static/"); idx >= 0 {
		value = value[idx+len("/static/"):]
	}
	value = strings.TrimPrefix(value, "static/")
	return append(paths, value)
}

func roomWelcomeStoragePathsFromContent(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	content := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &content); err != nil {
		return nil
	}
	return uniqueStorageRelativePaths([]string{
		stringFromAny(content["pic"]),
	})
}

func roomTagPullStoragePathsFromRooms(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var rooms []map[string]any
	if err := json.Unmarshal([]byte(raw), &rooms); err != nil {
		return nil
	}
	paths := make([]string, 0, len(rooms))
	for _, room := range rooms {
		paths = append(paths, stringFromAny(room["image"]))
	}
	return uniqueStorageRelativePaths(paths)
}

func workRoomAutoPullStoragePathsFromRooms(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var rooms []map[string]any
	if err := json.Unmarshal([]byte(raw), &rooms); err != nil {
		return nil
	}
	paths := make([]string, 0, len(rooms)*2)
	for _, room := range rooms {
		paths = append(paths,
			stringFromAny(room["roomQrcodeUrl"]),
			stringFromAny(room["longRoomQrcodeUrl"]),
			stringFromAny(room["room_qrcode_url"]),
			stringFromAny(room["long_room_qrcode_url"]),
		)
	}
	return uniqueStorageRelativePaths(paths)
}

func contactBatchAddStoragePathsFromFileURLs(values []string) []string {
	paths := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if idx := strings.Index(value, "/static/"); idx >= 0 {
			value = value[idx+len("/static/"):]
		}
		value = strings.TrimPrefix(value, "static/")
		paths = append(paths, value)
	}
	return uniqueStorageRelativePaths(paths)
}

func storagePathsRemoved(oldPaths []string, newPaths []string) []string {
	oldPaths = uniqueStorageRelativePaths(oldPaths)
	if len(oldPaths) == 0 {
		return nil
	}
	newPaths = uniqueStorageRelativePaths(newPaths)
	retained := make(map[string]struct{}, len(newPaths))
	for _, path := range newPaths {
		retained[path] = struct{}{}
	}
	removed := make([]string, 0, len(oldPaths))
	for _, path := range oldPaths {
		if _, ok := retained[path]; ok {
			continue
		}
		removed = append(removed, path)
	}
	return removed
}

func uniqueStorageRelativePaths(values []string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.Contains(value, "://") || strings.HasPrefix(value, "//") {
			continue
		}
		value = filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
		if value == "." || value == ".." || strings.HasPrefix(value, "../") || strings.HasPrefix(value, "/") {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func saasAlertKey(tenantID int, metric string, alertType string, periodKey string) string {
	return fmt.Sprintf("%d:%s:%s:%s", tenantID, strings.TrimSpace(metric), strings.TrimSpace(alertType), strings.TrimSpace(periodKey))
}

func nonNegativeInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func saasAdminMarshalJSON(value any) string {
	if value == nil {
		return ""
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

func applySaaSAdminRiskFollowUpJSON(item *dashboard.SaaSAdminRiskFollowUpSnapshot, raw string) {
	if item == nil || strings.TrimSpace(raw) == "" {
		return
	}
	var payload struct {
		Status         string `json:"status"`
		Owner          string `json:"owner"`
		NextFollowUpAt string `json:"nextFollowUpAt"`
		Remark         string `json:"remark"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return
	}
	item.Status = strings.TrimSpace(payload.Status)
	item.Owner = strings.TrimSpace(payload.Owner)
	item.NextFollowUpAt = strings.TrimSpace(payload.NextFollowUpAt)
	if strings.TrimSpace(payload.Remark) != "" {
		item.Remark = strings.TrimSpace(payload.Remark)
	}
}

func applySaaSAdminBillingReconciliationFollowUpJSON(item *dashboard.SaaSAdminBillingReconciliationFollowUpSnapshot, beforeRaw string, afterRaw string) {
	if item == nil {
		return
	}
	if strings.TrimSpace(beforeRaw) != "" {
		var before struct {
			ID              int64  `json:"id"`
			TenantID        int    `json:"tenantId"`
			PackageCode     string `json:"packageCode"`
			PackageName     string `json:"packageName"`
			NewExpiresAt    string `json:"newExpiresAt"`
			AmountCents     int64  `json:"amountCents"`
			Currency        string `json:"currency"`
			ExternalOrderNo string `json:"externalOrderNo"`
		}
		if err := json.Unmarshal([]byte(beforeRaw), &before); err == nil {
			if item.BillingEventID <= 0 {
				item.BillingEventID = before.ID
			}
			if item.TenantID <= 0 {
				item.TenantID = before.TenantID
			}
			if strings.TrimSpace(item.PackageCode) == "" {
				item.PackageCode = strings.TrimSpace(before.PackageCode)
			}
			if strings.TrimSpace(item.PackageName) == "" {
				item.PackageName = strings.TrimSpace(before.PackageName)
			}
			if strings.TrimSpace(item.NewExpiresAt) == "" {
				item.NewExpiresAt = strings.TrimSpace(before.NewExpiresAt)
			}
			if item.AmountCents == 0 {
				item.AmountCents = before.AmountCents
			}
			if strings.TrimSpace(item.Currency) == "" {
				item.Currency = strings.TrimSpace(before.Currency)
			}
			if strings.TrimSpace(item.ExternalOrderNo) == "" {
				item.ExternalOrderNo = strings.TrimSpace(before.ExternalOrderNo)
			}
		}
	}
	if strings.TrimSpace(afterRaw) == "" {
		return
	}
	var after struct {
		BillingEventID  int64  `json:"billingEventId"`
		TenantID        int    `json:"tenantId"`
		Status          string `json:"status"`
		Owner           string `json:"owner"`
		NextFollowUpAt  string `json:"nextFollowUpAt"`
		Remark          string `json:"remark"`
		PackageCode     string `json:"packageCode"`
		ExternalOrderNo string `json:"externalOrderNo"`
	}
	if err := json.Unmarshal([]byte(afterRaw), &after); err != nil {
		return
	}
	if after.BillingEventID > 0 {
		item.BillingEventID = after.BillingEventID
	}
	if after.TenantID > 0 {
		item.TenantID = after.TenantID
	}
	item.Status = strings.TrimSpace(after.Status)
	item.Owner = strings.TrimSpace(after.Owner)
	item.NextFollowUpAt = strings.TrimSpace(after.NextFollowUpAt)
	if strings.TrimSpace(after.Remark) != "" {
		item.Remark = strings.TrimSpace(after.Remark)
	}
	if strings.TrimSpace(after.PackageCode) != "" {
		item.PackageCode = strings.TrimSpace(after.PackageCode)
	}
	if strings.TrimSpace(after.ExternalOrderNo) != "" {
		item.ExternalOrderNo = strings.TrimSpace(after.ExternalOrderNo)
	}
}

func insertSaaSAdminOperationLogTx(ctx context.Context, tx *sql.Tx, item dashboard.SaaSAdminOperationLog) (int64, error) {
	id, err := insertSaaSAuditIntegrityLogTx(ctx, tx, item)
	if isMissingSaaSAuditIntegritySchemaError(err) {
		return insertLegacySaaSAdminOperationLogTx(ctx, tx, item)
	}
	return id, err
}

func saasAdminJSONValue(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return raw
}

func isMissingSaaSTableError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return (strings.Contains(message, "mochat_go_saas_usage_counters") ||
		strings.Contains(message, "mochat_go_saas_storage_objects") ||
		strings.Contains(message, "mochat_go_saas_tenant_packages") ||
		strings.Contains(message, "mochat_go_saas_packages") ||
		strings.Contains(message, "mochat_go_saas_alerts") ||
		strings.Contains(message, "mochat_go_saas_alert_settings") ||
		strings.Contains(message, "mochat_go_saas_alert_notifications") ||
		strings.Contains(message, "mochat_go_saas_admin_operation_logs") ||
		strings.Contains(message, "mochat_go_saas_billing_events") ||
		strings.Contains(message, "mochat_go_saas_admin_tasks") ||
		strings.Contains(message, "mochat_go_saas_subscriptions") ||
		strings.Contains(message, "mochat_go_saas_subscription_events") ||
		strings.Contains(message, "mochat_go_seed_versions") ||
		strings.Contains(message, "mochat_go_tenant_provision_runs") ||
		strings.Contains(message, "mochat_go_background_task_executions")) &&
		(strings.Contains(message, "doesn't exist") || strings.Contains(message, "no such table"))
}

func (s *MySQLStore) WorkAgentsForSync(ctx context.Context) ([]dashboard.WorkAgentSyncItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id
		FROM mc_work_agent a
		JOIN mc_corp c ON c.id = a.corp_id AND c.deleted_at IS NULL
		WHERE a.deleted_at IS NULL
		ORDER BY a.id ASC
	`)
	if err != nil {
		return nil, err
	}
	agentIDs := make([]int, 0)
	for rows.Next() {
		var agentID int
		if err := rows.Scan(&agentID); err != nil {
			rows.Close()
			return nil, err
		}
		agentIDs = append(agentIDs, agentID)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	agents := make([]dashboard.WorkAgentSyncItem, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		item, found, err := s.loadAgentCredentialByID(ctx, s.db, agentID, false)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		secret, err := s.decodeAgentCredential(item)
		if err != nil {
			return nil, err
		}
		agents = append(agents, dashboard.WorkAgentSyncItem{ID: item.ID, CorpID: item.CorpID, WXCorpID: item.WXCorpID, WXAgentID: item.WXAgentID, WXSecret: secret.WXSecret})
	}
	return agents, nil
}

func (s *MySQLStore) UpdateWorkAgentDetail(ctx context.Context, agentID int, detail dashboard.WorkAgentDetail) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_work_agent
		SET name = ?, square_logo_url = ?, description = ?, close = ?, redirect_domain = ?,
		    report_location_flag = ?, is_reportenter = ?, home_url = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, detail.Name, detail.SquareLogoURL, detail.Description, detail.Close, detail.RedirectDomain, detail.ReportLocationFlag, detail.IsReportEnter, detail.HomeURL, agentID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) CreateRoomWelcome(ctx context.Context, values dashboard.RoomWelcomeWrite) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_room_welcome_template (corp_id, msg_text, complex_type, msg_complex, complex_template_id, create_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, values.CorpID, values.MsgText, values.ComplexType, values.MsgComplex, values.ComplexTemplateID, values.CreateUserID)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func (s *MySQLStore) UpdateRoomWelcome(ctx context.Context, roomWelcomeID int, values dashboard.RoomWelcomeWrite) (bool, error) {
	var oldMsgComplex sql.NullString
	tenantID := 0
	err := s.db.QueryRowContext(ctx, `
		SELECT rw.msg_complex, COALESCE(c.tenant_id, 0)
		FROM mc_room_welcome_template AS rw
		LEFT JOIN mc_corp AS c ON c.id = rw.corp_id AND c.deleted_at IS NULL
		WHERE rw.id = ? AND rw.corp_id = ? AND rw.deleted_at IS NULL
		LIMIT 1
	`, roomWelcomeID, values.CorpID).Scan(&oldMsgComplex, &tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	reclaimPaths := storagePathsRemoved(
		roomWelcomeStoragePathsFromContent(nullString(oldMsgComplex)),
		roomWelcomeStoragePathsFromContent(values.MsgComplex),
	)
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_welcome_template
		SET corp_id = ?, msg_text = ?, complex_type = ?, msg_complex = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, values.CorpID, values.MsgText, values.ComplexType, values.MsgComplex, roomWelcomeID, values.CorpID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return affected > 0, err
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return true, err
	}
	return true, nil
}

func (s *MySQLStore) DeleteRoomWelcome(ctx context.Context, roomWelcomeID int) (bool, error) {
	var rawMsgComplex sql.NullString
	tenantID := 0
	err := s.db.QueryRowContext(ctx, `
		SELECT rw.msg_complex, COALESCE(c.tenant_id, 0)
		FROM mc_room_welcome_template AS rw
		LEFT JOIN mc_corp AS c ON c.id = rw.corp_id AND c.deleted_at IS NULL
		WHERE rw.id = ? AND rw.deleted_at IS NULL
		LIMIT 1
	`, roomWelcomeID).Scan(&rawMsgComplex, &tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	reclaimPaths := roomWelcomeStoragePathsFromContent(nullString(rawMsgComplex))
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_welcome_template
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, roomWelcomeID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return affected > 0, err
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return true, err
	}
	return true, nil
}

func roomWelcomeWhere(filter dashboard.RoomWelcomeFilter) ([]string, []any) {
	where := []string{"corp_id = ?", "deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if filter.Text != "" {
		where = append(where, "msg_text LIKE ?")
		args = append(args, "%"+filter.Text+"%")
	}
	if filter.RestrictCreateUserID {
		where = append(where, "create_user_id = ?")
		args = append(args, filter.CreateUserID)
	}
	return where, args
}

func scanRoomWelcome(scanner mediumScanner) (dashboard.RoomWelcomeItem, error) {
	var item dashboard.RoomWelcomeItem
	var corpID int
	var msgText, complexType, complexTemplateID sql.NullString
	var rawMsgComplex []byte
	var createdAt, updatedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&corpID,
		&msgText,
		&complexType,
		&rawMsgComplex,
		&complexTemplateID,
		&item.CreateUserID,
		&createdAt,
		&updatedAt,
	); err != nil {
		return dashboard.RoomWelcomeItem{}, err
	}
	item.CorpID = corpID
	item.MsgText = nullString(msgText)
	item.ComplexType = nullString(complexType)
	item.MsgComplex = string(rawMsgComplex)
	item.ComplexTemplateID = nullString(complexTemplateID)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func greetingWhere(filter dashboard.GreetingFilter) ([]string, []any) {
	where := []string{"corp_id = ?", "deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if filter.RestrictOperationIDs {
		where = append(where, `id IN (
			SELECT business_id
			FROM mc_business_log
			WHERE operation_id IN (`+placeholders(len(filter.OperationIDs))+`) AND event IN (300)
		)`)
		for _, id := range filter.OperationIDs {
			args = append(args, id)
		}
	}
	return where, args
}

func scanGreeting(scanner mediumScanner) (dashboard.GreetingItem, error) {
	var item dashboard.GreetingItem
	var rawEmployees []byte
	var createdAt sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&item.CorpID,
		&item.Type,
		&item.Words,
		&item.MediumID,
		&item.RangeType,
		&rawEmployees,
		&createdAt,
	); err != nil {
		return dashboard.GreetingItem{}, err
	}
	item.EmployeeIDs = parseJSONIntSlice(rawEmployees)
	item.CreatedAt = formatTime(createdAt)
	return item, nil
}

func parseJSONIntSlice(raw []byte) []int {
	if len(raw) == 0 {
		return []int{}
	}
	var ints []int
	if err := json.Unmarshal(raw, &ints); err == nil {
		return uniquePositiveInts(ints)
	}
	var stringsValue []string
	if err := json.Unmarshal(raw, &stringsValue); err != nil {
		return []int{}
	}
	ints = make([]int, 0, len(stringsValue))
	for _, value := range stringsValue {
		integer, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil {
			ints = append(ints, integer)
		}
	}
	return uniquePositiveInts(ints)
}

func greetingEmployeesJSONStore(employeeIDs []int) string {
	employeeIDs = uniquePositiveInts(employeeIDs)
	raw, _ := json.Marshal(employeeIDs)
	return string(raw)
}

func greetingBusinessLogPayload(values dashboard.GreetingWrite, employees string) string {
	payload := map[string]any{
		"corp_id":    values.CorpID,
		"type":       values.Type,
		"words":      values.Words,
		"medium_id":  values.MediumID,
		"range_type": values.RangeType,
		"employees":  employees,
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

type businessLogExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertBusinessLog(ctx context.Context, executor businessLogExecutor, businessID int, params string, event int, operationID int) error {
	_, err := executor.ExecContext(ctx, `
		INSERT INTO mc_business_log (business_id, params, event, operation_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())
	`, businessID, params, event, operationID)
	return err
}

func workContactSyncEmployeesQuery() string {
	return `
		SELECT id, wx_user_id
		FROM mc_work_employee
		WHERE corp_id = ? AND contact_auth = 1 AND wx_user_id <> '' AND deleted_at IS NULL
		ORDER BY id ASC
	`
}

func (s *MySQLStore) WorkContactSyncEmployees(ctx context.Context, corpID int) ([]dashboard.WorkContactSyncEmployee, error) {
	rows, err := s.db.QueryContext(ctx, workContactSyncEmployeesQuery(), corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	employees := make([]dashboard.WorkContactSyncEmployee, 0)
	for rows.Next() {
		var employee dashboard.WorkContactSyncEmployee
		if err := rows.Scan(&employee.ID, &employee.WXUserID); err != nil {
			return nil, err
		}
		employees = append(employees, employee)
	}
	return employees, rows.Err()
}

func (s *MySQLStore) WorkContactSyncEmployeeByWXUserID(ctx context.Context, corpID int, wxUserID string) (dashboard.WorkContactSyncEmployee, bool, error) {
	wxUserID = strings.TrimSpace(wxUserID)
	if corpID <= 0 || wxUserID == "" {
		return dashboard.WorkContactSyncEmployee{}, false, nil
	}
	var employee dashboard.WorkContactSyncEmployee
	err := s.db.QueryRowContext(ctx, `
		SELECT id, wx_user_id
		FROM mc_work_employee
		WHERE corp_id = ? AND wx_user_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, corpID, wxUserID).Scan(&employee.ID, &employee.WXUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WorkContactSyncEmployee{}, false, nil
	}
	if err != nil {
		return dashboard.WorkContactSyncEmployee{}, false, err
	}
	return employee, true, nil
}

func (s *MySQLStore) additionalWorkContactUsage(ctx context.Context, corpID int, bundles []dashboard.WorkContactSyncEmployeeContacts) (int64, error) {
	externalUserIDs := uniqueStrings(workContactExternalUserIDsFromBundles(bundles))
	if len(externalUserIDs) == 0 {
		return 0, nil
	}
	existing, err := s.activeWorkContactExternalUserIDs(ctx, corpID, externalUserIDs)
	if err != nil {
		return 0, err
	}
	return int64(len(externalUserIDs) - len(existing)), nil
}

func (s *MySQLStore) additionalWorkContactSingleUsage(ctx context.Context, corpID int, wxExternalUserID string) (int64, error) {
	wxExternalUserID = strings.TrimSpace(wxExternalUserID)
	if wxExternalUserID == "" {
		return 0, nil
	}
	existing, err := s.activeWorkContactExternalUserIDs(ctx, corpID, []string{wxExternalUserID})
	if err != nil {
		return 0, err
	}
	if _, ok := existing[wxExternalUserID]; ok {
		return 0, nil
	}
	return 1, nil
}

func (s *MySQLStore) activeWorkContactExternalUserIDs(ctx context.Context, corpID int, externalUserIDs []string) (map[string]struct{}, error) {
	externalUserIDs = uniqueStrings(externalUserIDs)
	existing := make(map[string]struct{}, len(externalUserIDs))
	if corpID <= 0 || len(externalUserIDs) == 0 {
		return existing, nil
	}
	args := make([]any, 0, 1+len(externalUserIDs))
	args = append(args, corpID)
	for _, externalUserID := range externalUserIDs {
		args = append(args, externalUserID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT wx_external_userid
		FROM mc_work_contact
		WHERE corp_id = ? AND wx_external_userid IN (`+placeholders(len(externalUserIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var externalUserID string
		if err := rows.Scan(&externalUserID); err != nil {
			return nil, err
		}
		externalUserID = strings.TrimSpace(externalUserID)
		if externalUserID != "" {
			existing[externalUserID] = struct{}{}
		}
	}
	return existing, rows.Err()
}

func workContactExternalUserIDsFromBundles(bundles []dashboard.WorkContactSyncEmployeeContacts) []string {
	externalUserIDs := make([]string, 0)
	for _, bundle := range bundles {
		for _, contact := range bundle.Contacts {
			externalUserIDs = append(externalUserIDs, contact.WXExternalUserID)
		}
	}
	return externalUserIDs
}

func (s *MySQLStore) SyncWorkContacts(ctx context.Context, corpID int, bundles []dashboard.WorkContactSyncEmployeeContacts) (dashboard.WorkContactSyncResult, error) {
	tenantID, err := s.tenantIDByCorpID(ctx, corpID)
	if err != nil {
		return dashboard.WorkContactSyncResult{}, err
	}
	additional, err := s.additionalWorkContactUsage(ctx, corpID, bundles)
	if err != nil {
		return dashboard.WorkContactSyncResult{}, err
	}
	if err := s.enforceSaaSQuotaForTenant(ctx, tenantID, dashboard.SaaSMetricContacts, additional); err != nil {
		return dashboard.WorkContactSyncResult{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.WorkContactSyncResult{}, err
	}
	defer rollbackQuietly(tx)

	var result dashboard.WorkContactSyncResult
	for _, bundle := range bundles {
		employeeID := bundle.Employee.ID
		if employeeID <= 0 {
			continue
		}
		if bundle.NoContact {
			removed, err := markWorkContactEmployeeRelationsRemovedTx(ctx, tx, corpID, employeeID)
			if err != nil {
				return dashboard.WorkContactSyncResult{}, err
			}
			result.RelationsRemoved += removed
			continue
		}
		externalUserIDs := uniqueStrings(bundle.ExternalUserIDs)
		if len(externalUserIDs) == 0 {
			continue
		}
		removed, err := removeMissingWorkContactEmployeeRelationsTx(ctx, tx, corpID, employeeID, externalUserIDs)
		if err != nil {
			return dashboard.WorkContactSyncResult{}, err
		}
		result.RelationsRemoved += removed

		for _, contact := range bundle.Contacts {
			contact.WXExternalUserID = strings.TrimSpace(contact.WXExternalUserID)
			if contact.WXExternalUserID == "" {
				continue
			}
			contactID, created, wasNew, err := upsertWorkContactSyncContactTx(ctx, tx, corpID, contact)
			if err != nil {
				return dashboard.WorkContactSyncResult{}, err
			}
			if wasNew {
				result.ContactWasNew = true
			}
			if created {
				result.ContactsCreated++
			} else {
				result.ContactsUpdated++
			}

			followUser, ok := workContactSyncFollowUserFor(contact.FollowUsers, bundle.Employee.WXUserID)
			if !ok {
				continue
			}
			createdRelation, err := upsertWorkContactEmployeeRelationTx(ctx, tx, corpID, employeeID, contactID, followUser)
			if err != nil {
				return dashboard.WorkContactSyncResult{}, err
			}
			if createdRelation {
				result.RelationsCreated++
			} else {
				result.RelationsUpdated++
			}

			tagResult, err := syncWorkContactFollowUserTagsTx(ctx, tx, corpID, contactID, employeeID, followUser.Tags)
			if err != nil {
				return dashboard.WorkContactSyncResult{}, err
			}
			result.TagsCreated += tagResult.TagsCreated
			result.TagsUpdated += tagResult.TagsUpdated
			result.PivotsCreated += tagResult.PivotsCreated
			result.PivotsDeleted += tagResult.PivotsDeleted
		}
	}
	if err := upsertWorkUpdateTimeTx(ctx, tx, corpID, 2); err != nil {
		return dashboard.WorkContactSyncResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.WorkContactSyncResult{}, err
	}
	if tenantID > 0 {
		if err := s.RefreshSaaSUsageCounter(ctx, tenantID, dashboard.SaaSMetricContacts); err != nil {
			return dashboard.WorkContactSyncResult{}, err
		}
	}
	return result, nil
}

func (s *MySQLStore) SyncWorkContactForEmployee(ctx context.Context, corpID int, employee dashboard.WorkContactSyncEmployee, contact dashboard.WorkContactSyncContact, createMissing bool) (dashboard.WorkContactSyncResult, error) {
	employee.WXUserID = strings.TrimSpace(employee.WXUserID)
	contact.WXExternalUserID = strings.TrimSpace(contact.WXExternalUserID)
	if corpID <= 0 || employee.ID <= 0 || employee.WXUserID == "" || contact.WXExternalUserID == "" {
		return dashboard.WorkContactSyncResult{}, nil
	}
	tenantID, err := s.tenantIDByCorpID(ctx, corpID)
	if err != nil {
		return dashboard.WorkContactSyncResult{}, err
	}
	if createMissing {
		additional, err := s.additionalWorkContactSingleUsage(ctx, corpID, contact.WXExternalUserID)
		if err != nil {
			return dashboard.WorkContactSyncResult{}, err
		}
		if err := s.enforceSaaSQuotaForTenant(ctx, tenantID, dashboard.SaaSMetricContacts, additional); err != nil {
			return dashboard.WorkContactSyncResult{}, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.WorkContactSyncResult{}, err
	}
	defer rollbackQuietly(tx)

	var result dashboard.WorkContactSyncResult
	var contactID int
	if createMissing {
		var created bool
		var wasNew bool
		contactID, created, wasNew, err = upsertWorkContactSyncContactTx(ctx, tx, corpID, contact)
		if err != nil {
			return dashboard.WorkContactSyncResult{}, err
		}
		result.ContactWasNew = wasNew
		if created {
			result.ContactsCreated++
		} else {
			result.ContactsUpdated++
		}
	} else {
		foundContactID, found, err := workContactIDByWXExternalUserIDTx(ctx, tx, corpID, contact.WXExternalUserID, false)
		if err != nil {
			return dashboard.WorkContactSyncResult{}, err
		}
		if !found {
			if err := tx.Commit(); err != nil {
				return dashboard.WorkContactSyncResult{}, err
			}
			return result, nil
		}
		contactID = foundContactID
		if _, _, _, err := upsertWorkContactSyncContactTx(ctx, tx, corpID, contact); err != nil {
			return dashboard.WorkContactSyncResult{}, err
		}
		result.ContactsUpdated++
	}
	result.ContactID = contactID

	followUser, ok := workContactSyncFollowUserFor(contact.FollowUsers, employee.WXUserID)
	if !ok {
		if err := tx.Commit(); err != nil {
			return dashboard.WorkContactSyncResult{}, err
		}
		return result, nil
	}
	if !createMissing {
		if _, found, err := activeWorkContactEmployeeRelationIDTx(ctx, tx, corpID, employee.ID, contactID); err != nil {
			return dashboard.WorkContactSyncResult{}, err
		} else if !found {
			if err := tx.Commit(); err != nil {
				return dashboard.WorkContactSyncResult{}, err
			}
			return result, nil
		}
	}
	createdRelation, err := upsertWorkContactEmployeeRelationTx(ctx, tx, corpID, employee.ID, contactID, followUser)
	if err != nil {
		return dashboard.WorkContactSyncResult{}, err
	}
	if createdRelation {
		result.RelationsCreated++
	} else {
		result.RelationsUpdated++
	}
	tagResult, err := syncWorkContactFollowUserTagsTx(ctx, tx, corpID, contactID, employee.ID, followUser.Tags)
	if err != nil {
		return dashboard.WorkContactSyncResult{}, err
	}
	result.TagsCreated += tagResult.TagsCreated
	result.TagsUpdated += tagResult.TagsUpdated
	result.PivotsCreated += tagResult.PivotsCreated
	result.PivotsDeleted += tagResult.PivotsDeleted
	if err := upsertWorkUpdateTimeTx(ctx, tx, corpID, 2); err != nil {
		return dashboard.WorkContactSyncResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.WorkContactSyncResult{}, err
	}
	if tenantID > 0 {
		if err := s.RefreshSaaSUsageCounter(ctx, tenantID, dashboard.SaaSMetricContacts); err != nil {
			return dashboard.WorkContactSyncResult{}, err
		}
	}
	return result, nil
}

func (s *MySQLStore) RemoveWorkContactEmployeeRelation(ctx context.Context, corpID int, wxUserID string, wxExternalUserID string, status int) (dashboard.WorkContactRemovalResult, bool, error) {
	wxUserID = strings.TrimSpace(wxUserID)
	wxExternalUserID = strings.TrimSpace(wxExternalUserID)
	if corpID <= 0 || wxUserID == "" || wxExternalUserID == "" {
		return dashboard.WorkContactRemovalResult{}, false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.WorkContactRemovalResult{}, false, err
	}
	defer rollbackQuietly(tx)

	contactID, found, err := workContactIDByWXExternalUserIDTx(ctx, tx, corpID, wxExternalUserID, false)
	if err != nil {
		return dashboard.WorkContactRemovalResult{}, false, err
	}
	if !found {
		if err := tx.Commit(); err != nil {
			return dashboard.WorkContactRemovalResult{}, false, err
		}
		return dashboard.WorkContactRemovalResult{}, false, nil
	}
	contactName, unionID, err := workContactNameUnionIDByIDTx(ctx, tx, corpID, contactID)
	if err != nil {
		return dashboard.WorkContactRemovalResult{}, false, err
	}
	if err := markWorkFissionContactLossTx(ctx, tx, unionID, wxExternalUserID); err != nil {
		return dashboard.WorkContactRemovalResult{}, false, err
	}
	employeeID, found, err := workEmployeeIDByWXUserIDTx(ctx, tx, corpID, wxUserID)
	if err != nil {
		return dashboard.WorkContactRemovalResult{}, false, err
	}
	if !found {
		if err := tx.Commit(); err != nil {
			return dashboard.WorkContactRemovalResult{}, false, err
		}
		return dashboard.WorkContactRemovalResult{}, false, nil
	}
	relationID, found, err := activeWorkContactEmployeeRelationIDTx(ctx, tx, corpID, employeeID, contactID)
	if err != nil {
		return dashboard.WorkContactRemovalResult{}, false, err
	}
	if !found {
		if err := tx.Commit(); err != nil {
			return dashboard.WorkContactRemovalResult{}, false, err
		}
		return dashboard.WorkContactRemovalResult{}, false, nil
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE mc_work_contact_employee
		SET status = ?, deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, status, relationID, corpID)
	if err != nil {
		return dashboard.WorkContactRemovalResult{}, false, err
	}
	removed, err := rowsAffectedInt(res)
	if err != nil {
		return dashboard.WorkContactRemovalResult{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_contact_tag_pivot
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE contact_id = ? AND employee_id = ? AND deleted_at IS NULL
	`, contactID, employeeID); err != nil {
		return dashboard.WorkContactRemovalResult{}, false, err
	}
	var activeRelations int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_work_contact_employee
		WHERE corp_id = ? AND contact_id = ? AND deleted_at IS NULL
	`, corpID, contactID).Scan(&activeRelations); err != nil {
		return dashboard.WorkContactRemovalResult{}, false, err
	}
	if activeRelations == 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_work_contact
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
		`, contactID, corpID); err != nil {
			return dashboard.WorkContactRemovalResult{}, false, err
		}
	}
	if err := upsertWorkUpdateTimeTx(ctx, tx, corpID, 2); err != nil {
		return dashboard.WorkContactRemovalResult{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.WorkContactRemovalResult{}, false, err
	}
	return dashboard.WorkContactRemovalResult{
		ContactID:   contactID,
		ContactName: contactName,
		WXUserID:    wxUserID,
		Status:      status,
	}, removed > 0, nil
}

func workContactNameUnionIDByIDTx(ctx context.Context, tx *sql.Tx, corpID int, contactID int) (string, string, error) {
	var name, unionID sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT name, unionid
		FROM mc_work_contact
		WHERE id = ? AND corp_id = ?
		LIMIT 1
	`, contactID, corpID).Scan(&name, &unionID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	return nullString(name), nullString(unionID), nil
}

func markWorkFissionContactLossTx(ctx context.Context, tx *sql.Tx, unionID string, wxExternalUserID string) error {
	unionID = strings.TrimSpace(unionID)
	wxExternalUserID = strings.TrimSpace(wxExternalUserID)
	if unionID == "" && wxExternalUserID == "" {
		return nil
	}
	where := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if unionID != "" {
		where = append(where, "union_id = ?")
		args = append(args, unionID)
	}
	if wxExternalUserID != "" {
		where = append(where, "external_user_id = ?")
		args = append(args, wxExternalUserID)
	}
	var id, parentID, loss int
	err := tx.QueryRowContext(ctx, `
		SELECT id, COALESCE(contact_superior_user_parent, 0), COALESCE(loss, 0)
		FROM mc_work_fission_contact
		WHERE deleted_at IS NULL AND (`+strings.Join(where, " OR ")+`)
		ORDER BY id ASC
		LIMIT 1
		FOR UPDATE
	`, args...).Scan(&id, &parentID, &loss)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_fission_contact
		SET loss = 1, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, id); err != nil {
		return err
	}
	if parentID <= 0 || loss == 1 {
		return nil
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE mc_work_fission_contact
		SET invite_count = GREATEST(invite_count - 1, 0), updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, parentID)
	return err
}

func (s *MySQLStore) additionalWorkRoomUsage(ctx context.Context, corpID int, wxChatID string) (int64, error) {
	wxChatID = strings.TrimSpace(wxChatID)
	if wxChatID == "" {
		return 0, nil
	}
	existing, err := s.activeWorkRoomWXChatIDs(ctx, corpID, []string{wxChatID})
	if err != nil {
		return 0, err
	}
	if _, ok := existing[wxChatID]; ok {
		return 0, nil
	}
	return 1, nil
}

func (s *MySQLStore) activeWorkRoomWXChatIDs(ctx context.Context, corpID int, wxChatIDs []string) (map[string]struct{}, error) {
	wxChatIDs = uniqueStrings(wxChatIDs)
	existing := make(map[string]struct{}, len(wxChatIDs))
	if corpID <= 0 || len(wxChatIDs) == 0 {
		return existing, nil
	}
	args := make([]any, 0, 1+len(wxChatIDs))
	args = append(args, corpID)
	for _, wxChatID := range wxChatIDs {
		args = append(args, wxChatID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT wx_chat_id
		FROM mc_work_room
		WHERE corp_id = ? AND wx_chat_id IN (`+placeholders(len(wxChatIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var wxChatID string
		if err := rows.Scan(&wxChatID); err != nil {
			return nil, err
		}
		wxChatID = strings.TrimSpace(wxChatID)
		if wxChatID != "" {
			existing[wxChatID] = struct{}{}
		}
	}
	return existing, rows.Err()
}

func workRoomWXChatIDsFromRooms(rooms []dashboard.WorkRoomSyncRoom) []string {
	wxChatIDs := make([]string, 0, len(rooms))
	for _, room := range rooms {
		wxChatIDs = append(wxChatIDs, room.WXChatID)
	}
	return wxChatIDs
}

func (s *MySQLStore) SyncWorkRooms(ctx context.Context, corpID int, rooms []dashboard.WorkRoomSyncRoom) (dashboard.WorkRoomSyncResult, error) {
	if corpID <= 0 || len(rooms) == 0 {
		return dashboard.WorkRoomSyncResult{}, nil
	}
	tenantID, err := s.tenantIDByCorpID(ctx, corpID)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	defer rollbackQuietly(tx)

	employees, err := workRoomSyncEmployeeIDsTx(ctx, tx, corpID)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	contacts, err := workRoomSyncContactIDsTx(ctx, tx, corpID)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	currentRooms, err := workRoomSyncCurrentRoomsTx(ctx, tx, corpID)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	projectedRooms := int64(len(uniqueStrings(workRoomWXChatIDsFromRooms(rooms))))
	if err := s.enforceSaaSProjectedQuotaForTenant(ctx, tenantID, dashboard.SaaSMetricRooms, int64(len(currentRooms)), projectedRooms); err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}

	var result dashboard.WorkRoomSyncResult
	seen := make(map[string]struct{}, len(rooms))
	for _, room := range rooms {
		wxChatID := strings.TrimSpace(room.WXChatID)
		if wxChatID == "" {
			continue
		}
		if _, exists := seen[wxChatID]; exists {
			continue
		}
		seen[wxChatID] = struct{}{}
		room.WXChatID = wxChatID
		itemResult, err := syncWorkRoomOneTx(ctx, tx, corpID, room, currentRooms[wxChatID], employees, contacts)
		if err != nil {
			return dashboard.WorkRoomSyncResult{}, err
		}
		result.RoomsCreated += itemResult.RoomsCreated
		result.RoomsUpdated += itemResult.RoomsUpdated
		result.MembersCreated += itemResult.MembersCreated
		result.MembersUpdated += itemResult.MembersUpdated
		result.MembersQuit += itemResult.MembersQuit
	}

	deleteIDs := make([]int, 0)
	for wxChatID, room := range currentRooms {
		if _, ok := seen[wxChatID]; ok {
			continue
		}
		deleteIDs = append(deleteIDs, room.ID)
	}
	if len(deleteIDs) > 0 {
		args := make([]any, 0, 1+len(deleteIDs))
		args = append(args, corpID)
		for _, id := range deleteIDs {
			args = append(args, id)
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE mc_work_room
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE corp_id = ? AND id IN (`+placeholders(len(deleteIDs))+`) AND deleted_at IS NULL
		`, args...)
		if err != nil {
			return dashboard.WorkRoomSyncResult{}, err
		}
		deleted, err := rowsAffectedInt(res)
		if err != nil {
			return dashboard.WorkRoomSyncResult{}, err
		}
		result.RoomsDeleted = deleted
	}

	if err := tx.Commit(); err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	if tenantID > 0 {
		if err := s.RefreshSaaSUsageCounter(ctx, tenantID, dashboard.SaaSMetricRooms); err != nil {
			return dashboard.WorkRoomSyncResult{}, err
		}
	}
	return result, nil
}

func (s *MySQLStore) SyncWorkRoom(ctx context.Context, corpID int, room dashboard.WorkRoomSyncRoom) (dashboard.WorkRoomSyncResult, error) {
	room.WXChatID = strings.TrimSpace(room.WXChatID)
	if corpID <= 0 || room.WXChatID == "" {
		return dashboard.WorkRoomSyncResult{}, nil
	}
	tenantID, err := s.tenantIDByCorpID(ctx, corpID)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	additional, err := s.additionalWorkRoomUsage(ctx, corpID, room.WXChatID)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	if err := s.enforceSaaSQuotaForTenant(ctx, tenantID, dashboard.SaaSMetricRooms, additional); err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	defer rollbackQuietly(tx)

	employees, err := workRoomSyncEmployeeIDsTx(ctx, tx, corpID)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	contacts, err := workRoomSyncContactIDsTx(ctx, tx, corpID)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	currentRooms, err := workRoomSyncCurrentRoomsTx(ctx, tx, corpID)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	result, err := syncWorkRoomOneTx(ctx, tx, corpID, room, currentRooms[room.WXChatID], employees, contacts)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	if err := upsertWorkUpdateTimeTx(ctx, tx, corpID, 3); err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	if tenantID > 0 {
		if err := s.RefreshSaaSUsageCounter(ctx, tenantID, dashboard.SaaSMetricRooms); err != nil {
			return dashboard.WorkRoomSyncResult{}, err
		}
	}
	return result, nil
}

func syncWorkRoomOneTx(ctx context.Context, tx *sql.Tx, corpID int, room dashboard.WorkRoomSyncRoom, current workRoomSyncRoomRow, employees map[string]int, contacts map[string]int) (dashboard.WorkRoomSyncResult, error) {
	var result dashboard.WorkRoomSyncResult
	room.WXChatID = strings.TrimSpace(room.WXChatID)
	if room.WXChatID == "" {
		return result, nil
	}
	name := room.Name
	if strings.TrimSpace(name) == "" {
		name = "群聊"
	}
	ownerID := employees[strings.TrimSpace(room.Owner)]
	createTime := workRoomSyncTime(room.CreateTime)

	if current.ID > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_work_room
			SET status = ?, name = ?, owner_id = ?, notice = ?, create_time = ?, updated_at = NOW()
			WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
		`, room.Status, name, ownerID, room.Notice, createTime, current.ID, corpID); err != nil {
			return dashboard.WorkRoomSyncResult{}, err
		}
		result.RoomsUpdated++
		memberResult, err := syncWorkRoomMembersTx(ctx, tx, current.ID, room.Members, current.Members, employees, contacts)
		if err != nil {
			return dashboard.WorkRoomSyncResult{}, err
		}
		result.MembersCreated += memberResult.MembersCreated
		result.MembersUpdated += memberResult.MembersUpdated
		result.MembersQuit += memberResult.MembersQuit
		return result, nil
	}

	insert, err := tx.ExecContext(ctx, `
		INSERT INTO mc_work_room (
			corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 200, NOW(), NOW())
	`, corpID, room.WXChatID, name, ownerID, room.Notice, room.Status, createTime)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	roomID, err := insert.LastInsertId()
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	result.RoomsCreated++
	memberResult, err := syncWorkRoomMembersTx(ctx, tx, int(roomID), room.Members, map[string]workRoomSyncMemberRow{}, employees, contacts)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	result.MembersCreated += memberResult.MembersCreated
	result.MembersUpdated += memberResult.MembersUpdated
	result.MembersQuit += memberResult.MembersQuit
	return result, nil
}

func (s *MySQLStore) DeleteWorkRoomByWXChatID(ctx context.Context, corpID int, wxChatID string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_work_room
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE corp_id = ? AND wx_chat_id = ? AND deleted_at IS NULL
	`, corpID, strings.TrimSpace(wxChatID))
	if err != nil {
		return false, err
	}
	affected, err := rowsAffectedInt(result)
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

type workRoomSyncRoomRow struct {
	ID      int
	Members map[string]workRoomSyncMemberRow
}

type workRoomSyncMemberRow struct {
	ID int
}

func workRoomSyncEmployeeIDsTx(ctx context.Context, tx *sql.Tx, corpID int) (map[string]int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT wx_user_id, id
		FROM mc_work_employee
		WHERE corp_id = ? AND wx_user_id <> '' AND deleted_at IS NULL
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	employees := map[string]int{}
	for rows.Next() {
		var wxUserID string
		var id int
		if err := rows.Scan(&wxUserID, &id); err != nil {
			return nil, err
		}
		wxUserID = strings.TrimSpace(wxUserID)
		if wxUserID != "" {
			employees[wxUserID] = id
		}
	}
	return employees, rows.Err()
}

func workRoomSyncContactIDsTx(ctx context.Context, tx *sql.Tx, corpID int) (map[string]int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT wx_external_userid, id
		FROM mc_work_contact
		WHERE corp_id = ? AND wx_external_userid <> '' AND deleted_at IS NULL
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	contacts := map[string]int{}
	for rows.Next() {
		var wxExternalUserID string
		var id int
		if err := rows.Scan(&wxExternalUserID, &id); err != nil {
			return nil, err
		}
		wxExternalUserID = strings.TrimSpace(wxExternalUserID)
		if wxExternalUserID != "" {
			contacts[wxExternalUserID] = id
		}
	}
	return contacts, rows.Err()
}

func workRoomSyncCurrentRoomsTx(ctx context.Context, tx *sql.Tx, corpID int) (map[string]workRoomSyncRoomRow, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, wx_chat_id
		FROM mc_work_room
		WHERE corp_id = ? AND wx_chat_id <> '' AND deleted_at IS NULL
		ORDER BY id ASC
	`, corpID)
	if err != nil {
		return nil, err
	}
	roomsByWX := map[string]workRoomSyncRoomRow{}
	roomIDs := make([]int, 0)
	for rows.Next() {
		var id int
		var wxChatID string
		if err := rows.Scan(&id, &wxChatID); err != nil {
			rows.Close()
			return nil, err
		}
		wxChatID = strings.TrimSpace(wxChatID)
		if wxChatID == "" {
			continue
		}
		roomsByWX[wxChatID] = workRoomSyncRoomRow{ID: id, Members: map[string]workRoomSyncMemberRow{}}
		roomIDs = append(roomIDs, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(roomIDs) == 0 {
		return roomsByWX, nil
	}

	roomByID := make(map[int]string, len(roomIDs))
	for wxChatID, room := range roomsByWX {
		roomByID[room.ID] = wxChatID
	}
	args := make([]any, 0, len(roomIDs))
	for _, id := range roomIDs {
		args = append(args, id)
	}
	memberRows, err := tx.QueryContext(ctx, `
		SELECT id, wx_user_id, room_id
		FROM mc_work_contact_room
		WHERE room_id IN (`+placeholders(len(roomIDs))+`) AND status <> 2 AND deleted_at IS NULL
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer memberRows.Close()
	for memberRows.Next() {
		var id int
		var wxUserID string
		var roomID int
		if err := memberRows.Scan(&id, &wxUserID, &roomID); err != nil {
			return nil, err
		}
		wxChatID := roomByID[roomID]
		if wxChatID == "" {
			continue
		}
		wxUserID = strings.TrimSpace(wxUserID)
		if wxUserID == "" {
			continue
		}
		room := roomsByWX[wxChatID]
		room.Members[wxUserID] = workRoomSyncMemberRow{ID: id}
		roomsByWX[wxChatID] = room
	}
	return roomsByWX, memberRows.Err()
}

func syncWorkRoomMembersTx(ctx context.Context, tx *sql.Tx, roomID int, members []dashboard.WorkRoomSyncMember, current map[string]workRoomSyncMemberRow, employees map[string]int, contacts map[string]int) (dashboard.WorkRoomSyncResult, error) {
	var result dashboard.WorkRoomSyncResult
	remaining := make(map[string]workRoomSyncMemberRow, len(current))
	for wxUserID, row := range current {
		remaining[wxUserID] = row
	}
	seen := make(map[string]struct{}, len(members))
	for _, member := range members {
		member.WXUserID = strings.TrimSpace(member.WXUserID)
		if member.WXUserID == "" {
			continue
		}
		if _, exists := seen[member.WXUserID]; exists {
			continue
		}
		seen[member.WXUserID] = struct{}{}

		contactID := 0
		employeeID := 0
		if member.Type == 1 {
			employeeID = employees[member.WXUserID]
		} else {
			contactID = contacts[member.WXUserID]
		}
		joinTime := workRoomSyncTime(member.JoinTime)
		if row, exists := current[member.WXUserID]; exists {
			if _, err := tx.ExecContext(ctx, `
				UPDATE mc_work_contact_room
				SET contact_id = ?, employee_id = ?, unionid = ?, join_scene = ?, type = ?,
					status = 1, join_time = ?, out_time = '', updated_at = NOW()
				WHERE id = ? AND room_id = ? AND deleted_at IS NULL
			`, contactID, employeeID, member.UnionID, member.JoinScene, member.Type, joinTime, row.ID, roomID); err != nil {
				return dashboard.WorkRoomSyncResult{}, err
			}
			delete(remaining, member.WXUserID)
			result.MembersUpdated++
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_work_contact_room (
				wx_user_id, contact_id, employee_id, unionid, room_id, join_scene, type,
				status, join_time, out_time, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, '', NOW(), NOW())
		`, member.WXUserID, contactID, employeeID, member.UnionID, roomID, member.JoinScene, member.Type, joinTime); err != nil {
			return dashboard.WorkRoomSyncResult{}, err
		}
		result.MembersCreated++
	}
	if len(remaining) == 0 {
		return result, nil
	}
	quitIDs := make([]int, 0, len(remaining))
	for _, row := range remaining {
		quitIDs = append(quitIDs, row.ID)
	}
	args := make([]any, 0, len(quitIDs))
	for _, id := range quitIDs {
		args = append(args, id)
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE mc_work_contact_room
		SET status = 2, out_time = DATE_FORMAT(NOW(), '%Y-%m-%d %H:%i:%s'), updated_at = NOW()
		WHERE id IN (`+placeholders(len(quitIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	quit, err := rowsAffectedInt(res)
	if err != nil {
		return dashboard.WorkRoomSyncResult{}, err
	}
	result.MembersQuit = quit
	return result, nil
}

func workRoomSyncTime(unix int64) string {
	if unix > 0 {
		return time.Unix(unix, 0).Format("2006-01-02 15:04:05")
	}
	return time.Now().Format("2006-01-02 15:04:05")
}

func markWorkContactEmployeeRelationsRemovedTx(ctx context.Context, tx *sql.Tx, corpID int, employeeID int) (int, error) {
	res, err := tx.ExecContext(ctx, `
		UPDATE mc_work_contact_employee
		SET status = 2, deleted_at = NOW(), updated_at = NOW()
		WHERE corp_id = ? AND employee_id = ? AND deleted_at IS NULL
	`, corpID, employeeID)
	if err != nil {
		return 0, err
	}
	return rowsAffectedInt(res)
}

func removeMissingWorkContactEmployeeRelationsTx(ctx context.Context, tx *sql.Tx, corpID int, employeeID int, externalUserIDs []string) (int, error) {
	externalUserIDs = uniqueStrings(externalUserIDs)
	if len(externalUserIDs) == 0 {
		return 0, nil
	}
	args := make([]any, 0, 3+len(externalUserIDs))
	args = append(args, corpID, employeeID, corpID)
	for _, id := range externalUserIDs {
		args = append(args, id)
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE mc_work_contact_employee AS rel
		JOIN mc_work_contact AS contact ON contact.id = rel.contact_id AND contact.deleted_at IS NULL
		SET rel.status = 2, rel.deleted_at = NOW(), rel.updated_at = NOW()
		WHERE rel.corp_id = ? AND rel.employee_id = ? AND rel.deleted_at IS NULL
			AND contact.corp_id = ?
			AND contact.wx_external_userid NOT IN (`+placeholders(len(externalUserIDs))+`)
	`, args...)
	if err != nil {
		return 0, err
	}
	return rowsAffectedInt(res)
}

func workContactIDByWXExternalUserIDTx(ctx context.Context, tx *sql.Tx, corpID int, wxExternalUserID string, includeDeleted bool) (int, bool, error) {
	wxExternalUserID = strings.TrimSpace(wxExternalUserID)
	if corpID <= 0 || wxExternalUserID == "" {
		return 0, false, nil
	}
	query := `
		SELECT id
		FROM mc_work_contact
		WHERE corp_id = ? AND wx_external_userid = ?
	`
	if !includeDeleted {
		query += " AND deleted_at IS NULL"
	}
	query += `
		ORDER BY id ASC
		LIMIT 1
	`
	var id int
	err := tx.QueryRowContext(ctx, query, corpID, wxExternalUserID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func upsertWorkContactSyncContactTx(ctx context.Context, tx *sql.Tx, corpID int, contact dashboard.WorkContactSyncContact) (int, bool, bool, error) {
	var id int
	var deletedAt sql.NullTime
	err := tx.QueryRowContext(ctx, `
		SELECT id, deleted_at
		FROM mc_work_contact
		WHERE corp_id = ? AND wx_external_userid = ?
		ORDER BY deleted_at IS NULL DESC, id ASC
		LIMIT 1
	`, corpID, contact.WXExternalUserID).Scan(&id, &deletedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, false, false, err
	}
	externalProfile := jsonRawOrEmptyArray(contact.ExternalProfile)
	if errors.Is(err, sql.ErrNoRows) {
		insert, err := tx.ExecContext(ctx, `
			INSERT INTO mc_work_contact (
				corp_id, wx_external_userid, name, avatar, type, gender, unionid, position,
				corp_name, corp_full_name, external_profile, business_no, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		`, corpID, contact.WXExternalUserID, contact.Name, contact.Avatar, contact.Type, contact.Gender, contact.UnionID, contact.Position, contact.CorpName, contact.CorpFullName, externalProfile, contact.BusinessNo)
		if err != nil {
			return 0, false, false, err
		}
		lastID, err := insert.LastInsertId()
		if err != nil {
			return 0, false, false, err
		}
		return int(lastID), true, true, nil
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE mc_work_contact
		SET name = ?, avatar = ?, type = ?, gender = ?, unionid = ?, position = ?,
			corp_name = ?, corp_full_name = ?, external_profile = ?, business_no = ?,
			deleted_at = NULL, updated_at = NOW()
		WHERE id = ? AND corp_id = ?
	`, contact.Name, contact.Avatar, contact.Type, contact.Gender, contact.UnionID, contact.Position, contact.CorpName, contact.CorpFullName, externalProfile, contact.BusinessNo, id, corpID)
	return id, false, deletedAt.Valid, err
}

func workContactSyncFollowUserFor(followUsers []dashboard.WorkContactSyncFollowUser, wxUserID string) (dashboard.WorkContactSyncFollowUser, bool) {
	wxUserID = strings.TrimSpace(wxUserID)
	for _, follow := range followUsers {
		if strings.TrimSpace(follow.UserID) == wxUserID {
			return follow, true
		}
	}
	return dashboard.WorkContactSyncFollowUser{}, false
}

func workEmployeeIDByWXUserIDTx(ctx context.Context, tx *sql.Tx, corpID int, wxUserID string) (int, bool, error) {
	wxUserID = strings.TrimSpace(wxUserID)
	if corpID <= 0 || wxUserID == "" {
		return 0, false, nil
	}
	var id int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_employee
		WHERE corp_id = ? AND wx_user_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, corpID, wxUserID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func activeWorkContactEmployeeRelationIDTx(ctx context.Context, tx *sql.Tx, corpID int, employeeID int, contactID int) (int, bool, error) {
	if corpID <= 0 || employeeID <= 0 || contactID <= 0 {
		return 0, false, nil
	}
	var id int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_contact_employee
		WHERE corp_id = ? AND employee_id = ? AND contact_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, corpID, employeeID, contactID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func upsertWorkContactEmployeeRelationTx(ctx context.Context, tx *sql.Tx, corpID int, employeeID int, contactID int, follow dashboard.WorkContactSyncFollowUser) (bool, error) {
	createTime := time.Now().Format("2006-01-02 15:04:05")
	if follow.CreateTime > 0 {
		createTime = time.Unix(follow.CreateTime, 0).Format("2006-01-02 15:04:05")
	}
	remarkMobiles := jsonStringSlice(follow.RemarkMobiles)

	var relationID int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_contact_employee
		WHERE corp_id = ? AND employee_id = ? AND contact_id = ?
		ORDER BY deleted_at IS NULL DESC, id ASC
		LIMIT 1
	`, corpID, employeeID, contactID).Scan(&relationID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO mc_work_contact_employee (
				employee_id, contact_id, remark, description, remark_corp_name, remark_mobiles,
				add_way, oper_userid, state, corp_id, status, create_time, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, NOW(), NOW())
		`, employeeID, contactID, follow.Remark, follow.Description, follow.RemarkCorpName, remarkMobiles, follow.AddWay, follow.OperUserID, follow.State, corpID, createTime)
		return true, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE mc_work_contact_employee
		SET remark = ?, description = ?, remark_corp_name = ?, remark_mobiles = ?,
			add_way = ?, oper_userid = ?, state = ?, status = 1, create_time = ?,
			deleted_at = NULL, updated_at = NOW()
		WHERE id = ? AND corp_id = ?
	`, follow.Remark, follow.Description, follow.RemarkCorpName, remarkMobiles, follow.AddWay, follow.OperUserID, follow.State, createTime, relationID, corpID)
	return false, err
}

func syncWorkContactFollowUserTagsTx(ctx context.Context, tx *sql.Tx, corpID int, contactID int, employeeID int, tags []dashboard.WorkContactSyncTag) (dashboard.WorkContactSyncResult, error) {
	var result dashboard.WorkContactSyncResult
	tagIDs := make([]int, 0)
	for _, tag := range tags {
		if tag.Type != 1 || strings.TrimSpace(tag.WXContactTagID) == "" {
			continue
		}
		groupID, err := ensureWorkContactSyncTagGroupTx(ctx, tx, corpID, tag.GroupName)
		if err != nil {
			return dashboard.WorkContactSyncResult{}, err
		}
		tagID, createdTag, err := ensureWorkContactSyncTagTx(ctx, tx, corpID, groupID, tag)
		if err != nil {
			return dashboard.WorkContactSyncResult{}, err
		}
		if tagID <= 0 {
			continue
		}
		if createdTag {
			result.TagsCreated++
		} else {
			result.TagsUpdated++
		}
		tagIDs = append(tagIDs, tagID)
	}
	createdPivots, deletedPivots, err := syncWorkContactTagPivotsTx(ctx, tx, contactID, employeeID, tagIDs)
	if err != nil {
		return dashboard.WorkContactSyncResult{}, err
	}
	result.PivotsCreated += createdPivots
	result.PivotsDeleted += deletedPivots
	return result, nil
}

func ensureWorkContactSyncTagGroupTx(ctx context.Context, tx *sql.Tx, corpID int, groupName string) (int, error) {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return 0, nil
	}
	var id int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_contact_tag_group
		WHERE corp_id = ? AND group_name = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, corpID, groupName).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	insert, err := tx.ExecContext(ctx, `
		INSERT INTO mc_work_contact_tag_group (corp_id, group_name, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())
	`, corpID, groupName)
	if err != nil {
		return 0, err
	}
	lastID, err := insert.LastInsertId()
	return int(lastID), err
}

func ensureWorkContactSyncTagTx(ctx context.Context, tx *sql.Tx, corpID int, groupID int, tag dashboard.WorkContactSyncTag) (int, bool, error) {
	wxTagID := strings.TrimSpace(tag.WXContactTagID)
	if wxTagID == "" {
		return 0, false, nil
	}
	var id int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_contact_tag
		WHERE corp_id = ? AND wx_contact_tag_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, corpID, wxTagID).Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		insert, err := tx.ExecContext(ctx, `
			INSERT INTO mc_work_contact_tag (wx_contact_tag_id, corp_id, name, contact_tag_group_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, NOW(), NOW())
		`, wxTagID, corpID, tag.TagName, groupID)
		if err != nil {
			return 0, false, err
		}
		lastID, err := insert.LastInsertId()
		return int(lastID), true, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE mc_work_contact_tag
		SET name = ?, contact_tag_group_id = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, tag.TagName, groupID, id, corpID)
	return id, false, err
}

func syncWorkContactTagPivotsTx(ctx context.Context, tx *sql.Tx, contactID int, employeeID int, tagIDs []int) (int, int, error) {
	tagIDs = uniquePositiveInts(tagIDs)
	rows, err := tx.QueryContext(ctx, `
		SELECT id, contact_tag_id, deleted_at
		FROM mc_work_contact_tag_pivot
		WHERE contact_id = ? AND employee_id = ?
	`, contactID, employeeID)
	if err != nil {
		return 0, 0, err
	}
	current := map[int]int{}
	deletedByTagID := map[int]int{}
	for rows.Next() {
		var id, tagID int
		var deletedAt sql.NullTime
		if err := rows.Scan(&id, &tagID, &deletedAt); err != nil {
			rows.Close()
			return 0, 0, err
		}
		if deletedAt.Valid {
			if _, ok := deletedByTagID[tagID]; !ok {
				deletedByTagID[tagID] = id
			}
			continue
		}
		if _, ok := current[tagID]; !ok {
			current[tagID] = id
		}
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	desired := make(map[int]struct{}, len(tagIDs))
	created := 0
	for _, tagID := range tagIDs {
		desired[tagID] = struct{}{}
		if _, ok := current[tagID]; ok {
			continue
		}
		if pivotID, ok := deletedByTagID[tagID]; ok {
			if _, err := tx.ExecContext(ctx, `
				UPDATE mc_work_contact_tag_pivot
				SET type = 1, deleted_at = NULL, updated_at = NOW()
				WHERE id = ?
			`, pivotID); err != nil {
				return 0, 0, err
			}
			created++
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_work_contact_tag_pivot (contact_id, employee_id, contact_tag_id, type, created_at, updated_at)
			VALUES (?, ?, ?, 1, NOW(), NOW())
		`, contactID, employeeID, tagID); err != nil {
			return 0, 0, err
		}
		created++
	}

	deleteIDs := make([]int, 0)
	for tagID, pivotID := range current {
		if _, ok := desired[tagID]; ok {
			continue
		}
		deleteIDs = append(deleteIDs, pivotID)
	}
	if len(deleteIDs) == 0 {
		return created, 0, nil
	}
	args := make([]any, 0, len(deleteIDs))
	for _, id := range deleteIDs {
		args = append(args, id)
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE mc_work_contact_tag_pivot
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id IN (`+placeholders(len(deleteIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return 0, 0, err
	}
	deletedCount, err := rowsAffectedInt(res)
	return created, deletedCount, err
}

func rowsAffectedInt(result sql.Result) (int, error) {
	affected, err := result.RowsAffected()
	return int(affected), err
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func jsonStringSlice(values []string) string {
	if values == nil {
		return "[]"
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func (s *MySQLStore) WorkContactByExternalUserID(ctx context.Context, externalUserID string) (dashboard.WorkContactDetail, bool, error) {
	var contact dashboard.WorkContactDetail
	var avatar sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, avatar, corp_id
		FROM mc_work_contact
		WHERE wx_external_userid = ? AND deleted_at IS NULL
		LIMIT 1
	`, externalUserID).Scan(&contact.ID, &contact.Name, &avatar, &contact.CorpID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WorkContactDetail{}, false, nil
	}
	if err != nil {
		return dashboard.WorkContactDetail{}, false, err
	}
	contact.Avatar = nullString(avatar)
	return contact, true, nil
}

func (s *MySQLStore) SidebarWorkContactByExternalUserID(ctx context.Context, externalUserID string, corpID int, employeeID int) (dashboard.WorkContactDetail, bool, error) {
	var contact dashboard.WorkContactDetail
	var avatar sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT contact.id, contact.name, contact.avatar, contact.corp_id
		FROM mc_work_contact AS contact
		JOIN mc_work_contact_employee AS pivot
		  ON pivot.contact_id = contact.id
		 AND pivot.employee_id = ?
		 AND pivot.corp_id = contact.corp_id
		 AND pivot.deleted_at IS NULL
		JOIN mc_work_employee AS employee
		  ON employee.id = pivot.employee_id
		 AND employee.corp_id = contact.corp_id
		 AND employee.deleted_at IS NULL
		WHERE contact.wx_external_userid = ? AND contact.corp_id = ? AND contact.deleted_at IS NULL
		LIMIT 1
	`, employeeID, externalUserID, corpID).Scan(&contact.ID, &contact.Name, &avatar, &contact.CorpID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WorkContactDetail{}, false, nil
	}
	if err != nil {
		return dashboard.WorkContactDetail{}, false, err
	}
	contact.Avatar = nullString(avatar)
	return contact, true, nil
}

func (s *MySQLStore) WorkContactShowByID(ctx context.Context, contactID int, employeeID int, corpID int) (dashboard.WorkContactShow, bool, error) {
	info := dashboard.WorkContactShow{
		Tags:         []dashboard.WorkContactShowTag{},
		RoomNames:    []string{},
		EmployeeName: []string{},
	}
	var name, avatar, businessNo, remark, description sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT contact.name, contact.avatar, contact.gender, contact.business_no, pivot.remark, pivot.description
		FROM mc_work_contact AS contact
		JOIN mc_work_contact_employee AS pivot
		  ON pivot.contact_id = contact.id
		 AND pivot.employee_id = ?
		 AND pivot.corp_id = contact.corp_id
		 AND pivot.deleted_at IS NULL
		JOIN mc_work_employee AS employee
		  ON employee.id = pivot.employee_id AND employee.corp_id = contact.corp_id AND employee.deleted_at IS NULL
		WHERE contact.id = ? AND contact.corp_id = ? AND contact.deleted_at IS NULL
		LIMIT 1
	`, employeeID, contactID, corpID).Scan(&name, &avatar, &info.Gender, &businessNo, &remark, &description)
	if errors.Is(err, sql.ErrNoRows) {
		return info, false, nil
	}
	if err != nil {
		return dashboard.WorkContactShow{}, false, err
	}
	info.Name = nullString(name)
	info.Avatar = nullString(avatar)
	info.BusinessNo = nullString(businessNo)
	info.Remark = nullString(remark)
	info.Description = nullString(description)

	tags, err := s.workContactShowTags(ctx, contactID, employeeID, corpID)
	if err != nil {
		return dashboard.WorkContactShow{}, false, err
	}
	info.Tags = tags
	roomNames, err := s.workContactShowRoomNames(ctx, contactID)
	if err != nil {
		return dashboard.WorkContactShow{}, false, err
	}
	info.RoomNames = roomNames
	employeeNames, err := s.workContactShowEmployeeNames(ctx, contactID)
	if err != nil {
		return dashboard.WorkContactShow{}, false, err
	}
	info.EmployeeName = employeeNames

	return info, true, nil
}

func (s *MySQLStore) workContactShowTags(ctx context.Context, contactID int, employeeID int, corpID int) ([]dashboard.WorkContactShowTag, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT tag.id, tag.name
		FROM mc_work_contact_tag_pivot AS pivot
		JOIN mc_work_contact AS contact
		  ON contact.id = pivot.contact_id AND contact.corp_id = ? AND contact.deleted_at IS NULL
		JOIN mc_work_contact_tag AS tag
		  ON tag.id = pivot.contact_tag_id AND tag.corp_id = ? AND tag.deleted_at IS NULL
		WHERE pivot.contact_id = ?
		  AND pivot.employee_id = ?
		  AND pivot.deleted_at IS NULL
		ORDER BY tag.id ASC
	`, corpID, corpID, contactID, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]dashboard.WorkContactShowTag, 0)
	for rows.Next() {
		var tag dashboard.WorkContactShowTag
		if err := rows.Scan(&tag.TagID, &tag.TagName); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tags, nil
}

func (s *MySQLStore) workContactShowRoomNames(ctx context.Context, contactID int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT room.name
		FROM mc_work_contact_room AS contact_room
		JOIN mc_work_room AS room ON room.id = contact_room.room_id AND room.deleted_at IS NULL
		WHERE contact_room.contact_id = ? AND contact_room.deleted_at IS NULL
		ORDER BY room.id ASC
	`, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name sql.NullString
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, nullString(name))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return names, nil
}

func (s *MySQLStore) workContactShowEmployeeNames(ctx context.Context, contactID int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT employee.name, corp.name
		FROM mc_work_contact_employee AS contact_employee
		JOIN mc_work_employee AS employee ON employee.id = contact_employee.employee_id AND employee.deleted_at IS NULL
		LEFT JOIN mc_corp AS corp ON corp.id = employee.corp_id AND corp.deleted_at IS NULL
		WHERE contact_employee.contact_id = ? AND contact_employee.deleted_at IS NULL
		ORDER BY contact_employee.id ASC
	`, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var employeeName, corpName sql.NullString
		if err := rows.Scan(&employeeName, &corpName); err != nil {
			return nil, err
		}
		names = append(names, strings.TrimSpace(nullString(corpName)+" "+nullString(employeeName)))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return names, nil
}

func (s *MySQLStore) WorkContactIndexPage(ctx context.Context, filter dashboard.WorkContactIndexFilter) (dashboard.WorkContactIndexPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 20
	}
	page := dashboard.WorkContactIndexPage{Items: []dashboard.WorkContactIndexItem{}, PerPage: filter.PerPage}
	if filter.CorpID <= 0 {
		page.FilterNoData = true
		page.PerPage = 20
		return page, nil
	}
	if filter.RestrictEmployees {
		filter.EmployeeIDs = uniquePositiveInts(filter.EmployeeIDs)
		if len(filter.EmployeeIDs) == 0 {
			page.FilterNoData = true
			page.PerPage = 20
			return page, nil
		}
	}
	contactIDs, noContactIDs, noData, err := s.workContactIndexContactIDs(ctx, filter)
	if err != nil {
		return dashboard.WorkContactIndexPage{}, err
	}
	if noData {
		page.FilterNoData = true
		page.PerPage = 20
		return page, nil
	}

	where, args := workContactIndexWhere(filter, contactIDs, noContactIDs)
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_work_contact_employee WHERE "+strings.Join(where, " AND "), args...).Scan(&page.Total); err != nil {
		return dashboard.WorkContactIndexPage{}, err
	}
	if page.Total == 0 {
		page.EmptyData = true
		return page, nil
	}
	page.TotalPage = (page.Total + page.PerPage - 1) / page.PerPage
	offset := (filter.Page - 1) * page.PerPage
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, employee_id, contact_id, remark, create_time, add_way
		FROM mc_work_contact_employee
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY create_time DESC
		LIMIT ? OFFSET ?
	`, append(append([]any{}, args...), page.PerPage, offset)...)
	if err != nil {
		return dashboard.WorkContactIndexPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.WorkContactIndexItem, 0)
	contactIDs = make([]int, 0)
	employeeIDs := make([]int, 0)
	for rows.Next() {
		var item dashboard.WorkContactIndexItem
		var remark sql.NullString
		var createTime sql.NullTime
		if err := rows.Scan(&item.ID, &item.EmployeeID, &item.ContactID, &remark, &createTime, &item.AddWay); err != nil {
			return dashboard.WorkContactIndexPage{}, err
		}
		item.Remark = nullString(remark)
		item.CreateTime = formatTime(createTime)
		if filter.CurrentEmployeeID == item.EmployeeID {
			item.IsContact = 1
		} else {
			item.IsContact = 2
		}
		items = append(items, item)
		contactIDs = append(contactIDs, item.ContactID)
		employeeIDs = append(employeeIDs, item.EmployeeID)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkContactIndexPage{}, err
	}

	contacts, err := s.workContactIndexContacts(ctx, contactIDs)
	if err != nil {
		return dashboard.WorkContactIndexPage{}, err
	}
	rooms, err := s.workContactIndexRoomNames(ctx, contactIDs)
	if err != nil {
		return dashboard.WorkContactIndexPage{}, err
	}
	employees, err := s.workContactIndexEmployeeNames(ctx, employeeIDs)
	if err != nil {
		return dashboard.WorkContactIndexPage{}, err
	}
	tags, err := s.workContactIndexTags(ctx, items)
	if err != nil {
		return dashboard.WorkContactIndexPage{}, err
	}
	for index := range items {
		if contact, ok := contacts[items[index].ContactID]; ok {
			items[index].BusinessNo = contact.BusinessNo
			items[index].Name = contact.Name
			items[index].Avatar = contact.Avatar
			items[index].Gender = contact.Gender
		}
		if roomNames, ok := rooms[items[index].ContactID]; ok {
			items[index].RoomName = roomNames
		} else {
			items[index].RoomName = []string{}
		}
		items[index].EmployeeName = employees[items[index].EmployeeID]
		items[index].Tag = tags[workContactIndexPairKey(items[index].ContactID, items[index].EmployeeID)]
		if items[index].Tag == nil {
			items[index].Tag = []string{}
		}
	}
	syncTime, err := s.workUpdateTime(ctx, filter.CorpID, 2)
	if err != nil {
		return dashboard.WorkContactIndexPage{}, err
	}
	page.Items = items
	page.SyncContactTime = syncTime
	return page, nil
}

func (s *MySQLStore) WorkContactLossPage(ctx context.Context, filter dashboard.WorkContactLossFilter) (dashboard.WorkContactLossPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 20
	}
	page := dashboard.WorkContactLossPage{Items: []dashboard.WorkContactLossItem{}, PerPage: filter.PerPage}
	if filter.CorpID <= 0 {
		page.NoData = true
		page.PerPage = 20
		return page, nil
	}
	if filter.RestrictEmployees {
		filter.EmployeeIDs = uniquePositiveInts(filter.EmployeeIDs)
		if len(filter.EmployeeIDs) == 0 {
			page.NoData = true
			page.PerPage = 20
			return page, nil
		}
	}
	where, args := workContactLossWhere(filter)
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_work_contact_employee WHERE "+strings.Join(where, " AND "), args...).Scan(&page.Total); err != nil {
		return dashboard.WorkContactLossPage{}, err
	}
	if page.Total == 0 {
		page.NoData = true
		page.PerPage = 20
		return page, nil
	}
	page.TotalPage = (page.Total + page.PerPage - 1) / page.PerPage
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, employee_id, contact_id, deleted_at
		FROM mc_work_contact_employee
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY create_time DESC
		LIMIT ? OFFSET ?
	`, append(append([]any{}, args...), page.PerPage, (filter.Page-1)*page.PerPage)...)
	if err != nil {
		return dashboard.WorkContactLossPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.WorkContactLossItem, 0)
	contactIDs := make([]int, 0)
	employeeIDs := make([]int, 0)
	for rows.Next() {
		var item dashboard.WorkContactLossItem
		var deletedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.EmployeeID, &item.ContactID, &deletedAt); err != nil {
			return dashboard.WorkContactLossPage{}, err
		}
		item.DeletedAt = formatTime(deletedAt)
		items = append(items, item)
		contactIDs = append(contactIDs, item.ContactID)
		employeeIDs = append(employeeIDs, item.EmployeeID)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkContactLossPage{}, err
	}
	contacts, err := s.workContactLossContacts(ctx, contactIDs)
	if err != nil {
		return dashboard.WorkContactLossPage{}, err
	}
	employeeNames, employeeRemarks, err := s.workContactLossEmployeeNames(ctx, employeeIDs)
	if err != nil {
		return dashboard.WorkContactLossPage{}, err
	}
	tags, err := s.workContactLossTags(ctx, items)
	if err != nil {
		return dashboard.WorkContactLossPage{}, err
	}
	for index := range items {
		if contact, ok := contacts[items[index].ContactID]; ok {
			items[index].Avatar = contact.Avatar
			items[index].Name = contact.Name
		}
		items[index].EmployeeName = employeeNames[items[index].EmployeeID]
		items[index].Remark = employeeRemarks[items[index].EmployeeID]
		items[index].Tag = tags[workContactIndexPairKey(items[index].ContactID, items[index].EmployeeID)]
		if items[index].Tag == nil {
			items[index].Tag = []string{}
		}
	}
	page.Items = items
	return page, nil
}

func (s *MySQLStore) WorkContactRoomIndex(ctx context.Context, filter dashboard.WorkContactRoomFilter) (dashboard.WorkContactRoomPage, bool, error) {
	var roomCorpID, ownerID int
	err := s.db.QueryRowContext(ctx, `
		SELECT corp_id, owner_id
		FROM mc_work_room
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, filter.WorkRoomID).Scan(&roomCorpID, &ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WorkContactRoomPage{}, false, nil
	}
	if err != nil {
		return dashboard.WorkContactRoomPage{}, false, err
	}
	page := dashboard.WorkContactRoomPage{
		Items:   []dashboard.WorkContactRoomItem{},
		PerPage: filter.PerPage,
	}
	if filter.PerPage <= 0 {
		page.PerPage = 10
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}

	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 2 THEN 1 ELSE 0 END), 0)
		FROM mc_work_contact_room
		WHERE room_id = ? AND deleted_at IS NULL
	`, filter.WorkRoomID).Scan(&page.MemberNum, &page.OutRoomNum); err != nil {
		return dashboard.WorkContactRoomPage{}, false, err
	}

	employeeIDs, contactIDs, nameFiltered, err := s.workContactRoomNameIDs(ctx, roomCorpID, filter.Name)
	if err != nil {
		return dashboard.WorkContactRoomPage{}, false, err
	}
	if nameFiltered && len(employeeIDs) == 0 && len(contactIDs) == 0 {
		return page, true, nil
	}

	where, args := workContactRoomWhere(filter, employeeIDs, contactIDs, nameFiltered)
	countQuery := "SELECT COUNT(*) FROM mc_work_contact_room WHERE " + where
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&page.Total); err != nil {
		return dashboard.WorkContactRoomPage{}, false, err
	}
	if page.Total == 0 {
		return page, true, nil
	}
	page.TotalPage = (page.Total + page.PerPage - 1) / page.PerPage
	offset := (filter.Page - 1) * page.PerPage
	query := `
		SELECT id, wx_user_id, contact_id, employee_id, room_id, join_scene, type, status, join_time, out_time
		FROM mc_work_contact_room
		WHERE ` + where + `
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`
	listArgs := append(append([]any{}, args...), page.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, query, listArgs...)
	if err != nil {
		return dashboard.WorkContactRoomPage{}, false, err
	}
	defer rows.Close()

	rawItems := make([]workContactRoomRaw, 0)
	for rows.Next() {
		var item workContactRoomRaw
		var joinTime sql.NullTime
		var outTime sql.NullString
		if err := rows.Scan(&item.ID, &item.WXUserID, &item.ContactID, &item.EmployeeID, &item.RoomID, &item.JoinScene, &item.Type, &item.Status, &joinTime, &outTime); err != nil {
			return dashboard.WorkContactRoomPage{}, false, err
		}
		item.JoinTime = formatTime(joinTime)
		item.OutTime = nullString(outTime)
		rawItems = append(rawItems, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkContactRoomPage{}, false, err
	}
	items, err := s.decorateWorkContactRoomItems(ctx, rawItems, ownerID, filter.CorpID)
	if err != nil {
		return dashboard.WorkContactRoomPage{}, false, err
	}
	page.Items = items
	return page, true, nil
}

func (s *MySQLStore) WorkRoomOptions(ctx context.Context, filter dashboard.WorkRoomOptionFilter) (dashboard.WorkRoomOptionPage, error) {
	filter.CorpIDs = uniquePositiveInts(filter.CorpIDs)
	if len(filter.CorpIDs) == 0 {
		return dashboard.WorkRoomOptionPage{Items: []dashboard.WorkRoomOption{}}, nil
	}

	totalWhere := []string{"corp_id IN (" + placeholders(len(filter.CorpIDs)) + ")", "deleted_at IS NULL"}
	totalArgs := make([]any, 0, len(filter.CorpIDs)+1)
	for _, id := range filter.CorpIDs {
		totalArgs = append(totalArgs, id)
	}
	if filter.Name != "" {
		totalWhere = append(totalWhere, "name LIKE ?")
		totalArgs = append(totalArgs, "%"+filter.Name+"%")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_work_room WHERE "+strings.Join(totalWhere, " AND "), totalArgs...).Scan(&total); err != nil {
		return dashboard.WorkRoomOptionPage{}, err
	}

	listWhere := append([]string{}, totalWhere...)
	listArgs := append([]any{}, totalArgs...)
	if filter.RoomGroupID != nil {
		listWhere = append(listWhere, "room_group_id = ?")
		listArgs = append(listArgs, *filter.RoomGroupID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, room_max
		FROM mc_work_room
		WHERE `+strings.Join(listWhere, " AND ")+`
		ORDER BY id ASC
	`, listArgs...)
	if err != nil {
		return dashboard.WorkRoomOptionPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.WorkRoomOption, 0)
	roomIDs := make([]int, 0)
	for rows.Next() {
		var item dashboard.WorkRoomOption
		var name sql.NullString
		var roomMax sql.NullInt64
		if err := rows.Scan(&item.RoomID, &name, &roomMax); err != nil {
			return dashboard.WorkRoomOptionPage{}, err
		}
		item.RoomName = nullString(name)
		item.RoomMax = int(roomMax.Int64)
		items = append(items, item)
		roomIDs = append(roomIDs, item.RoomID)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkRoomOptionPage{}, err
	}
	if len(items) == 0 {
		return dashboard.WorkRoomOptionPage{Items: []dashboard.WorkRoomOption{}}, nil
	}
	counts, err := s.activeRoomMemberCounts(ctx, roomIDs)
	if err != nil {
		return dashboard.WorkRoomOptionPage{}, err
	}
	for index := range items {
		items[index].CurrentNum = counts[items[index].RoomID]
	}
	return dashboard.WorkRoomOptionPage{Total: total, Items: items}, nil
}

func (s *MySQLStore) WorkRoomIndexPage(ctx context.Context, filter dashboard.WorkRoomIndexFilter) (dashboard.WorkRoomIndexPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10
	}
	page := dashboard.WorkRoomIndexPage{Items: []dashboard.WorkRoomIndexItem{}, PerPage: filter.PerPage}
	if filter.CorpID <= 0 {
		return page, nil
	}
	filter.OwnerIDs = uniquePositiveInts(filter.OwnerIDs)
	if filter.RestrictOwner && len(filter.OwnerIDs) == 0 {
		return page, nil
	}

	where, args := workRoomIndexWhere(filter)
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_work_room WHERE "+strings.Join(where, " AND "), args...).Scan(&page.Total); err != nil {
		return dashboard.WorkRoomIndexPage{}, err
	}
	if page.Total == 0 {
		return page, nil
	}
	page.TotalPage = (page.Total + page.PerPage - 1) / page.PerPage
	offset := (filter.Page - 1) * page.PerPage
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, owner_id, room_group_id, status, notice, create_time
		FROM mc_work_room
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY create_time DESC
		LIMIT ? OFFSET ?
	`, append(append([]any{}, args...), page.PerPage, offset)...)
	if err != nil {
		return dashboard.WorkRoomIndexPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.WorkRoomIndexItem, 0)
	roomIDs := make([]int, 0)
	ownerIDs := make([]int, 0)
	groupIDs := make([]int, 0)
	for rows.Next() {
		var item dashboard.WorkRoomIndexItem
		var name, notice sql.NullString
		var groupID int
		var createTime sql.NullTime
		if err := rows.Scan(&item.WorkRoomID, &name, &item.OwnerID, &groupID, &item.Status, &notice, &createTime); err != nil {
			return dashboard.WorkRoomIndexPage{}, err
		}
		item.RoomName = nullString(name)
		item.Notice = nullString(notice)
		item.CreateTime = formatTime(createTime)
		items = append(items, item)
		roomIDs = append(roomIDs, item.WorkRoomID)
		ownerIDs = append(ownerIDs, item.OwnerID)
		groupIDs = append(groupIDs, groupID)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkRoomIndexPage{}, err
	}

	ownerNames, err := s.workRoomOwnerNames(ctx, ownerIDs)
	if err != nil {
		return dashboard.WorkRoomIndexPage{}, err
	}
	groupNames, err := s.workRoomGroupNames(ctx, groupIDs)
	if err != nil {
		return dashboard.WorkRoomIndexPage{}, err
	}
	counts, err := s.workRoomIndexMemberCounts(ctx, roomIDs)
	if err != nil {
		return dashboard.WorkRoomIndexPage{}, err
	}
	for index := range items {
		items[index].OwnerName = ownerNames[ownerIDs[index]]
		items[index].RoomGroup = groupNames[groupIDs[index]]
		if count, ok := counts[items[index].WorkRoomID]; ok {
			items[index].MemberNum = count.MemberNum
			items[index].InRoomNum = count.InRoomNum
			items[index].OutRoomNum = count.OutRoomNum
		}
	}
	page.Items = items
	return page, nil
}

func (s *MySQLStore) WorkRoomExistsByCorpWXChatID(ctx context.Context, corpID int, wxChatID string) (bool, error) {
	if corpID <= 0 || strings.TrimSpace(wxChatID) == "" {
		return false, nil
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_work_room
		WHERE corp_id = ? AND wx_chat_id = ? AND deleted_at IS NULL
	`, corpID, wxChatID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *MySQLStore) WorkRoomAutoPullPage(ctx context.Context, filter dashboard.WorkRoomAutoPullFilter) (dashboard.WorkRoomAutoPullPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10
	}
	page := dashboard.WorkRoomAutoPullPage{Items: []dashboard.WorkRoomAutoPullItem{}, PerPage: filter.PerPage}
	filter.CorpIDs = uniquePositiveInts(filter.CorpIDs)
	if len(filter.CorpIDs) == 0 {
		return page, nil
	}
	if filter.RestrictBusinessIDs {
		filter.BusinessIDs = uniquePositiveInts(filter.BusinessIDs)
		if len(filter.BusinessIDs) == 0 {
			return page, nil
		}
	}
	where, args := workRoomAutoPullWhere(filter)
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_work_room_auto_pull WHERE "+strings.Join(where, " AND "), args...).Scan(&page.Total); err != nil {
		return dashboard.WorkRoomAutoPullPage{}, err
	}
	if page.Total == 0 {
		return page, nil
	}
	page.TotalPage = (page.Total + page.PerPage - 1) / page.PerPage
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, page.PerPage, (filter.Page-1)*page.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, group_id, medium_id, qrcode_name, qrcode_url, leading_words, tags, employees, rooms, created_at,
		       lifecycle_state, data_source, provider_kind, auto_create_room
		FROM mc_work_room_auto_pull
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.WorkRoomAutoPullPage{}, err
	}
	defer rows.Close()
	type autoPullRaw struct {
		item        dashboard.WorkRoomAutoPullItem
		tagIDs      []int
		employeeIDs []int
		rooms       []roomAutoPullRoom
	}
	raws := make([]autoPullRaw, 0)
	tagIDs := make([]int, 0)
	employeeIDs := make([]int, 0)
	roomIDs := make([]int, 0)
	for rows.Next() {
		var raw autoPullRaw
		var rawTags, rawEmployees, rawRooms []byte
		var createdAt sql.NullTime
		if err := rows.Scan(&raw.item.WorkRoomAutoPullID, &raw.item.GroupID, &raw.item.MediumID, &raw.item.QRCodeName, &raw.item.QRCodeURL, &raw.item.LeadingWords, &rawTags, &rawEmployees, &rawRooms, &createdAt, &raw.item.LifecycleState, &raw.item.DataSource, &raw.item.ProviderKind, &raw.item.AutoCreateRoom); err != nil {
			return dashboard.WorkRoomAutoPullPage{}, err
		}
		if raw.item.LifecycleState == "" {
			raw.item.LifecycleState = "active"
		}
		if raw.item.DataSource == "" {
			raw.item.DataSource = "provider"
		}
		raw.item.StatisticsAvailable = true
		raw.item.CreatedAt = formatTime(createdAt)
		raw.tagIDs = intSliceFromJSON(rawTags)
		raw.employeeIDs = intSliceFromJSON(rawEmployees)
		raw.rooms = parseRoomAutoPullRooms(rawRooms)
		tagIDs = append(tagIDs, raw.tagIDs...)
		employeeIDs = append(employeeIDs, raw.employeeIDs...)
		for _, room := range raw.rooms {
			roomIDs = append(roomIDs, room.RoomID)
		}
		raws = append(raws, raw)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkRoomAutoPullPage{}, err
	}
	tagNames, err := s.channelCodeTagNames(ctx, tagIDs)
	if err != nil {
		return dashboard.WorkRoomAutoPullPage{}, err
	}
	employeeNames, err := s.workRoomAutoPullEmployeeNames(ctx, employeeIDs)
	if err != nil {
		return dashboard.WorkRoomAutoPullPage{}, err
	}
	rooms, err := s.workRoomAutoPullRoomsByID(ctx, roomIDs)
	if err != nil {
		return dashboard.WorkRoomAutoPullPage{}, err
	}
	stats, err := s.workRoomAutoPullListRoomStats(ctx, roomIDs)
	if err != nil {
		return dashboard.WorkRoomAutoPullPage{}, err
	}
	items := make([]dashboard.WorkRoomAutoPullItem, 0, len(raws))
	for _, raw := range raws {
		item := raw.item
		item.Tags = make([]string, 0)
		for _, id := range raw.tagIDs {
			if name, ok := tagNames[id]; ok {
				item.Tags = append(item.Tags, name)
			}
		}
		item.Employees = make([]string, 0)
		for _, id := range raw.employeeIDs {
			if name, ok := employeeNames[id]; ok {
				item.Employees = append(item.Employees, name)
			}
		}
		item.Rooms = make([]dashboard.WorkRoomAutoPullListRoom, 0)
		isDrawing := false
		for _, roomPayload := range raw.rooms {
			room, ok := rooms[roomPayload.RoomID]
			if !ok {
				continue
			}
			stat := stats[roomPayload.RoomID]
			item.ContactNum += stat.ContactNum
			state := 1
			if item.ProviderKind == "join_way" {
				item.Rooms = append(item.Rooms, dashboard.WorkRoomAutoPullListRoom{RoomName: room.Name, StateText: "直接进群"})
				continue
			}
			if stat.Total < room.RoomMax && stat.Total < roomPayload.MaxNum {
				if !isDrawing {
					state = 2
					isDrawing = true
				}
			} else {
				state = 3
			}
			item.Rooms = append(item.Rooms, dashboard.WorkRoomAutoPullListRoom{
				RoomName:  room.Name,
				StateText: workRoomAutoPullDrawStateText(state),
			})
		}
		items = append(items, item)
	}
	page.Items = items
	return page, nil
}

func (s *MySQLStore) WorkRoomAutoPullBusinessIDsByOperators(ctx context.Context, operationIDs []int) ([]int, error) {
	operationIDs = uniquePositiveInts(operationIDs)
	if len(operationIDs) == 0 {
		return []int{}, nil
	}
	args := make([]any, 0, len(operationIDs)+1)
	for _, id := range operationIDs {
		args = append(args, id)
	}
	args = append(args, 200)
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT business_id
		FROM mc_business_log
		WHERE operation_id IN (`+placeholders(len(operationIDs))+`) AND event = ?
		ORDER BY business_id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) WorkRoomAutoPullShowByID(ctx context.Context, id int) (dashboard.WorkRoomAutoPullShow, bool, error) {
	var info dashboard.WorkRoomAutoPullShow
	var corpID int
	var rawTags, rawEmployees, rawRooms []byte
	var createdAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, medium_id, qrcode_name, qrcode_url, is_verified, leading_words, tags, employees, rooms, created_at,
		       provider_kind, auto_create_room, room_base_name, room_base_id
		FROM mc_work_room_auto_pull
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, id).Scan(&info.WorkRoomAutoPullID, &corpID, &info.MediumID, &info.QRCodeName, &info.QRCodeURL, &info.IsVerified, &info.LeadingWords, &rawTags, &rawEmployees, &rawRooms, &createdAt, &info.ProviderKind, &info.AutoCreateRoom, &info.RoomBaseName, &info.RoomBaseID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WorkRoomAutoPullShow{}, false, nil
	}
	if err != nil {
		return dashboard.WorkRoomAutoPullShow{}, false, err
	}
	info.CreatedAt = formatTime(createdAt)
	employeeIDs := intSliceFromJSON(rawEmployees)
	info.Employees, err = s.workRoomAutoPullEmployees(ctx, employeeIDs)
	if err != nil {
		return dashboard.WorkRoomAutoPullShow{}, false, err
	}
	tagIDs := intSliceFromJSON(rawTags)
	info.Tags, info.SelectedTags, err = s.channelCodeTagGroups(ctx, corpID, tagIDs)
	if err != nil {
		return dashboard.WorkRoomAutoPullShow{}, false, err
	}
	rooms := parseRoomAutoPullRooms(rawRooms)
	info.Rooms, err = s.workRoomAutoPullShowRooms(ctx, rooms)
	if err != nil {
		return dashboard.WorkRoomAutoPullShow{}, false, err
	}
	info.RoomNum = len(info.Rooms)
	return info, true, nil
}

func (s *MySQLStore) WorkRoomAutoPullWelcomeByID(ctx context.Context, id int) (dashboard.WorkRoomAutoPullWelcome, bool, error) {
	var welcome dashboard.WorkRoomAutoPullWelcome
	var rawRooms []byte
	var rawTags []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id, leading_words, rooms, tags
		FROM mc_work_room_auto_pull
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, id).Scan(&welcome.ID, &welcome.LeadingWords, &rawRooms, &rawTags)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WorkRoomAutoPullWelcome{}, false, nil
	}
	if err != nil {
		return dashboard.WorkRoomAutoPullWelcome{}, false, err
	}
	welcome.TagIDs = parseJSONIntSlice(rawTags)
	roomPayloads := parseRoomAutoPullRooms(rawRooms)
	roomIDs := make([]int, 0, len(roomPayloads))
	for _, room := range roomPayloads {
		roomIDs = append(roomIDs, room.RoomID)
	}
	roomInfo, err := s.workRoomAutoPullRoomsByID(ctx, roomIDs)
	if err != nil {
		return dashboard.WorkRoomAutoPullWelcome{}, false, err
	}
	counts, err := s.workRoomAutoPullActiveRoomCounts(ctx, roomIDs)
	if err != nil {
		return dashboard.WorkRoomAutoPullWelcome{}, false, err
	}
	welcome.Rooms = make([]dashboard.WorkRoomAutoPullWelcomeRoom, 0, len(roomPayloads))
	for _, payload := range roomPayloads {
		info, ok := roomInfo[payload.RoomID]
		if !ok {
			continue
		}
		welcome.Rooms = append(welcome.Rooms, dashboard.WorkRoomAutoPullWelcomeRoom{
			RoomID:        payload.RoomID,
			MaxNum:        payload.MaxNum,
			RoomMax:       info.RoomMax,
			MemberNum:     counts[payload.RoomID],
			RoomQRCodeURL: payload.RoomQRCodeURL,
		})
	}
	return welcome, true, nil
}

func (s *MySQLStore) WorkFissionContactRuleByID(ctx context.Context, id int) (dashboard.WorkFissionContactRule, bool, error) {
	var rule dashboard.WorkFissionContactRule
	var rawTags []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id, contact_tags
		FROM mc_work_fission
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, id).Scan(&rule.ID, &rawTags)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WorkFissionContactRule{}, false, nil
	}
	if err != nil {
		return dashboard.WorkFissionContactRule{}, false, err
	}
	rule.TagIDs = parseJSONIntSlice(rawTags)
	return rule, true, nil
}

func (s *MySQLStore) WorkFissionContactWelcomeByParentID(ctx context.Context, parentContactID int) (dashboard.WorkFissionContactWelcome, bool, error) {
	var welcome dashboard.WorkFissionContactWelcome
	var msgText, linkTitle, linkDesc, linkCoverURL sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT contact.id, contact.fission_id,
		       welcome.msg_text, welcome.link_title, welcome.link_desc, welcome.link_cover_url
		FROM mc_work_fission_contact AS contact
		INNER JOIN mc_work_fission AS fission
			ON fission.id = contact.fission_id AND fission.deleted_at IS NULL
		INNER JOIN mc_work_fission_welcome AS welcome
			ON welcome.fission_id = contact.fission_id AND welcome.deleted_at IS NULL
		WHERE contact.id = ? AND contact.deleted_at IS NULL
		ORDER BY welcome.id DESC
		LIMIT 1
	`, parentContactID).Scan(&welcome.ParentContactID, &welcome.FissionID, &msgText, &linkTitle, &linkDesc, &linkCoverURL)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WorkFissionContactWelcome{}, false, nil
	}
	if err != nil {
		return dashboard.WorkFissionContactWelcome{}, false, err
	}
	welcome.MsgText = nullString(msgText)
	welcome.LinkTitle = nullString(linkTitle)
	welcome.LinkDesc = nullString(linkDesc)
	welcome.LinkCoverURL = nullString(linkCoverURL)
	return welcome, true, nil
}

type workFissionAddContactParent struct {
	ID               int
	FissionID        int
	Level            int
	InviteCount      int
	ParentID         int
	EmployeeWXUserID string
	ExternalUserID   string
}

type workFissionAddContactActivity struct {
	ID                 int
	ActiveName         string
	ServiceEmployees   string
	Tasks              string
	EndTime            time.Time
	NewFriend          int
	PushEmployee       int
	PushContact        int
	PushMsgText        string
	PushMsgComplex     string
	PushMsgComplexType string
	TotalCount         int
}

type workFissionPushConfig struct {
	PushEmployee   int
	PushContact    int
	MsgText        string
	MsgComplex     string
	MsgComplexType string
}

func (s *MySQLStore) HandleWorkFissionAddContact(ctx context.Context, event dashboard.WorkFissionAddContactEvent) (dashboard.WorkFissionAddContactResult, bool, error) {
	event.EmployeeWXUserID = strings.TrimSpace(event.EmployeeWXUserID)
	event.WXExternalUserID = strings.TrimSpace(event.WXExternalUserID)
	if event.CorpID <= 0 || event.ParentContactID <= 0 || event.EmployeeWXUserID == "" || event.WXExternalUserID == "" {
		return dashboard.WorkFissionAddContactResult{}, false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.WorkFissionAddContactResult{}, false, err
	}
	defer rollbackQuietly(tx)

	parent, found, err := workFissionAddContactParentByIDTx(ctx, tx, event.CorpID, event.ParentContactID)
	if err != nil || !found {
		return dashboard.WorkFissionAddContactResult{}, found, err
	}
	activity, found, err := workFissionAddContactActivityByIDTx(ctx, tx, event.CorpID, parent.FissionID)
	if err != nil || !found {
		return dashboard.WorkFissionAddContactResult{}, found, err
	}
	parentLevel, err := workFissionAddContactParentLevelTx(ctx, tx, parent)
	if err != nil {
		return dashboard.WorkFissionAddContactResult{}, false, err
	}
	childID, childCreated, err := upsertWorkFissionChildContactTx(ctx, tx, parent, parentLevel, event)
	if err != nil {
		return dashboard.WorkFissionAddContactResult{}, false, err
	}
	inviteCount := parent.InviteCount
	if childCreated && !(activity.NewFriend == 1 && !event.IsNew) {
		inviteCount = parent.InviteCount + 1
	}
	completed := activity.TotalCount > 0 && inviteCount == activity.TotalCount
	if err := updateWorkFissionParentProgressTx(ctx, tx, parent.ID, parentLevel, inviteCount, event.EmployeeWXUserID, completed); err != nil {
		return dashboard.WorkFissionAddContactResult{}, false, err
	}
	reminder := workFissionEmployeeReminder(activity, event, completed)
	customerPush := workFissionCustomerPush(activity, parent, event, completed)
	if execution, ok := dashboard.WeWorkCallbackExecutionFromContext(ctx); ok {
		if execution.CorpID != event.CorpID {
			return dashboard.WorkFissionAddContactResult{}, false, errors.New("wework callback side effect corp scope mismatch")
		}
		if reminder != nil {
			if err := insertWeWorkCallbackSideEffectIntent(ctx, tx, execution, dashboard.WeWorkCallbackActionFissionEmployeeReminder, reminder); err != nil {
				return dashboard.WorkFissionAddContactResult{}, false, err
			}
		}
		if customerPush != nil {
			if err := insertWeWorkCallbackSideEffectIntent(ctx, tx, execution, dashboard.WeWorkCallbackActionFissionCustomerPush, customerPush); err != nil {
				return dashboard.WorkFissionAddContactResult{}, false, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return dashboard.WorkFissionAddContactResult{}, false, err
	}
	return dashboard.WorkFissionAddContactResult{
		ParentContactID:  parent.ID,
		ChildContactID:   childID,
		FissionID:        parent.FissionID,
		InviteCount:      inviteCount,
		TotalCount:       activity.TotalCount,
		Completed:        completed,
		EmployeeReminder: reminder,
		CustomerPush:     customerPush,
	}, true, nil
}

func workFissionAddContactParentByIDTx(ctx context.Context, tx *sql.Tx, corpID int, parentContactID int) (workFissionAddContactParent, bool, error) {
	var parent workFissionAddContactParent
	err := tx.QueryRowContext(ctx, `
		SELECT contact.id, contact.fission_id, COALESCE(contact.level, 0), COALESCE(contact.invite_count, 0),
		       COALESCE(contact.contact_superior_user_parent, 0), COALESCE(contact.employee, ''),
		       COALESCE(contact.external_user_id, '')
		FROM mc_work_fission_contact AS contact
		INNER JOIN mc_work_fission AS fission
			ON fission.id = contact.fission_id AND fission.corp_id = ? AND fission.deleted_at IS NULL
		WHERE contact.id = ? AND contact.deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, corpID, parentContactID).Scan(&parent.ID, &parent.FissionID, &parent.Level, &parent.InviteCount, &parent.ParentID, &parent.EmployeeWXUserID, &parent.ExternalUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return workFissionAddContactParent{}, false, nil
	}
	if err != nil {
		return workFissionAddContactParent{}, false, err
	}
	return parent, true, nil
}

func workFissionAddContactAncestorByIDTx(ctx context.Context, tx *sql.Tx, id int, fissionID int) (workFissionAddContactParent, bool, error) {
	var parent workFissionAddContactParent
	err := tx.QueryRowContext(ctx, `
		SELECT id, fission_id, COALESCE(level, 0), COALESCE(invite_count, 0), COALESCE(contact_superior_user_parent, 0)
		FROM mc_work_fission_contact
		WHERE id = ? AND fission_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, id, fissionID).Scan(&parent.ID, &parent.FissionID, &parent.Level, &parent.InviteCount, &parent.ParentID)
	if errors.Is(err, sql.ErrNoRows) {
		return workFissionAddContactParent{}, false, nil
	}
	if err != nil {
		return workFissionAddContactParent{}, false, err
	}
	return parent, true, nil
}

func workFissionAddContactActivityByIDTx(ctx context.Context, tx *sql.Tx, corpID int, fissionID int) (workFissionAddContactActivity, bool, error) {
	var activity workFissionAddContactActivity
	var activeName, serviceEmployees, tasks sql.NullString
	var endTime sql.NullTime
	var newFriend sql.NullInt64
	err := tx.QueryRowContext(ctx, `
		SELECT id, active_name, service_employees, tasks, end_time, new_friend
		FROM mc_work_fission
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, fissionID, corpID).Scan(&activity.ID, &activeName, &serviceEmployees, &tasks, &endTime, &newFriend)
	if errors.Is(err, sql.ErrNoRows) {
		return workFissionAddContactActivity{}, false, nil
	}
	if err != nil {
		return workFissionAddContactActivity{}, false, err
	}
	activity.ActiveName = nullString(activeName)
	activity.ServiceEmployees = nullString(serviceEmployees)
	activity.Tasks = nullString(tasks)
	if endTime.Valid {
		activity.EndTime = endTime.Time
	}
	activity.NewFriend = nullInt64(newFriend)
	activity.TotalCount = workFissionTasksTotal(activity.Tasks)
	push, err := workFissionPushConfigByFissionIDTx(ctx, tx, fissionID)
	if err != nil {
		return workFissionAddContactActivity{}, false, err
	}
	activity.PushEmployee = push.PushEmployee
	activity.PushContact = push.PushContact
	activity.PushMsgText = push.MsgText
	activity.PushMsgComplex = push.MsgComplex
	activity.PushMsgComplexType = push.MsgComplexType
	return activity, true, nil
}

func workFissionPushConfigByFissionIDTx(ctx context.Context, tx *sql.Tx, fissionID int) (workFissionPushConfig, error) {
	var push workFissionPushConfig
	var pushEmployee, pushContact sql.NullInt64
	var msgText, msgComplex, msgComplexType sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT push_employee, push_contact, msg_text, msg_complex, msg_complex_type
		FROM mc_work_fission_push
		WHERE fission_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, fissionID).Scan(&pushEmployee, &pushContact, &msgText, &msgComplex, &msgComplexType)
	if errors.Is(err, sql.ErrNoRows) {
		return workFissionPushConfig{}, nil
	}
	if err != nil {
		return workFissionPushConfig{}, err
	}
	push.PushEmployee = nullInt64(pushEmployee)
	push.PushContact = nullInt64(pushContact)
	push.MsgText = nullString(msgText)
	push.MsgComplex = nullString(msgComplex)
	push.MsgComplexType = nullString(msgComplexType)
	return push, nil
}

func workFissionAddContactParentLevelTx(ctx context.Context, tx *sql.Tx, parent workFissionAddContactParent) (int, error) {
	if parent.Level != 0 {
		return parent.Level, nil
	}
	if parent.ParentID == 0 {
		return 1, nil
	}
	level := 2
	grandparent, found, err := workFissionAddContactAncestorByIDTx(ctx, tx, parent.ParentID, parent.FissionID)
	if err != nil || !found {
		return level, err
	}
	if grandparent.ParentID <= 0 {
		return level, nil
	}
	level = 3
	greatGrandparent, found, err := workFissionAddContactAncestorByIDTx(ctx, tx, grandparent.ParentID, parent.FissionID)
	if err != nil || !found {
		return level, err
	}
	if greatGrandparent.ParentID > 0 {
		return 0, nil
	}
	return level, nil
}

func upsertWorkFissionChildContactTx(ctx context.Context, tx *sql.Tx, parent workFissionAddContactParent, parentLevel int, event dashboard.WorkFissionAddContactEvent) (int, bool, error) {
	var childID int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_fission_contact
		WHERE fission_id = ? AND contact_superior_user_parent = ? AND external_user_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
		FOR UPDATE
	`, parent.FissionID, parent.ID, event.WXExternalUserID).Scan(&childID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, `
			UPDATE mc_work_fission_contact
			SET loss = 0, employee = ?, updated_at = NOW()
			WHERE id = ? AND deleted_at IS NULL
		`, event.EmployeeWXUserID, childID)
		return childID, false, err
	}
	level := 0
	if parentLevel > 0 && parentLevel <= 2 {
		level = parentLevel + 1
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_work_fission_contact
			(fission_id, union_id, nickname, avatar, contact_superior_user_parent, level, employee,
			 invite_count, loss, status, receive_level, is_new, external_user_id, qrcode_id, qrcode_url,
			 created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0, 0, 0, ?, ?, '', '', NOW(), NOW())
	`, parent.FissionID, event.UnionID, event.Name, event.Avatar, parent.ID, level, event.EmployeeWXUserID, workFissionBoolInt(event.IsNew), event.WXExternalUserID)
	if err != nil {
		return 0, false, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, false, err
	}
	return int(id), true, nil
}

func updateWorkFissionParentProgressTx(ctx context.Context, tx *sql.Tx, parentID int, level int, inviteCount int, employeeWXUserID string, completed bool) error {
	statusSQL := ""
	if completed {
		statusSQL = ", status = 1"
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE mc_work_fission_contact
		SET level = ?, invite_count = ?, employee = ?, updated_at = NOW()`+statusSQL+`
		WHERE id = ? AND deleted_at IS NULL
	`, level, inviteCount, employeeWXUserID, parentID)
	return err
}

func workFissionTasksTotal(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	var tasks []map[string]any
	if err := json.Unmarshal([]byte(raw), &tasks); err != nil {
		return 0
	}
	total := 0
	for _, task := range tasks {
		total += workFissionTaskCount(task["count"])
	}
	return total
}

func workFissionTaskCount(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return 0
}

func workFissionEmployeeReminder(activity workFissionAddContactActivity, event dashboard.WorkFissionAddContactEvent, completed bool) *dashboard.WorkFissionEmployeeReminder {
	if !completed || activity.PushEmployee != 1 || activity.EndTime.IsZero() || !activity.EndTime.After(time.Now()) {
		return nil
	}
	wxUserIDs := workFissionServiceEmployeeWXUserIDs(activity.ServiceEmployees)
	if len(wxUserIDs) == 0 {
		return nil
	}
	return &dashboard.WorkFissionEmployeeReminder{
		ToUser:  strings.Join(wxUserIDs, "|"),
		Content: fmt.Sprintf("客户【%s】已完成裂变任务：%s", event.Name, activity.ActiveName),
	}
}

func workFissionCustomerPush(activity workFissionAddContactActivity, parent workFissionAddContactParent, event dashboard.WorkFissionAddContactEvent, completed bool) *dashboard.WorkFissionCustomerPush {
	if !completed || activity.PushContact != 1 || activity.EndTime.IsZero() || !activity.EndTime.After(time.Now()) {
		return nil
	}
	externalUserID := strings.TrimSpace(parent.ExternalUserID)
	if externalUserID == "" {
		return nil
	}
	sender := strings.TrimSpace(event.EmployeeWXUserID)
	if sender == "" {
		sender = strings.TrimSpace(parent.EmployeeWXUserID)
	}
	if sender == "" {
		return nil
	}
	content := workFissionCustomerPushContent(activity.PushMsgText, activity.PushMsgComplexType, activity.PushMsgComplex)
	if len(content) == 0 {
		return nil
	}
	return &dashboard.WorkFissionCustomerPush{
		Sender:         sender,
		ExternalUserID: externalUserID,
		Content:        content,
	}
}

func workFissionCustomerPushContent(msgText string, complexType string, rawComplex string) []dashboard.ContactMessageBatchSendContent {
	content := make([]dashboard.ContactMessageBatchSendContent, 0, 2)
	if text := strings.TrimSpace(msgText); text != "" {
		content = append(content, dashboard.ContactMessageBatchSendContent{
			MsgType: "text",
			Content: strings.ReplaceAll(text, "[用户昵称]", "%NICKNAME%"),
		})
	}
	complexType = strings.TrimSpace(complexType)
	if complexType == "" || strings.TrimSpace(rawComplex) == "" {
		return content
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(rawComplex), &payload); err != nil {
		return content
	}
	switch complexType {
	case "image":
		if image := roomTagPullStringFromAny(payload["image"]); image != "" {
			content = append(content, dashboard.ContactMessageBatchSendContent{MsgType: "image", PicURL: image})
		}
	case "link":
		title := roomTagPullStringFromAny(payload["title"])
		url := roomTagPullStringFromAny(payload["url"])
		if title != "" && url != "" {
			content = append(content, dashboard.ContactMessageBatchSendContent{
				MsgType: "link",
				Title:   title,
				Desc:    roomTagPullStringFromAny(payload["desc"]),
				URL:     url,
				PicURL:  roomTagPullStringFromAny(payload["pic_url"]),
			})
		}
	case "applets":
		title := roomTagPullStringFromAny(payload["title"])
		appID := roomTagPullStringFromAny(payload["appid"])
		page := roomTagPullStringFromAny(payload["path"])
		image := roomTagPullStringFromAny(payload["image"])
		if title != "" && appID != "" && page != "" && image != "" {
			content = append(content, dashboard.ContactMessageBatchSendContent{
				MsgType: "miniprogram",
				Title:   title,
				AppID:   appID,
				Page:    page,
				PicURL:  image,
			})
		}
	}
	return content
}

func workFissionServiceEmployeeWXUserIDs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var decoded any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return []string{}
	}
	items, ok := decoded.([]any)
	if !ok {
		return []string{}
	}
	result := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		wxUserID := strings.TrimSpace(fmt.Sprint(object["wxUserId"]))
		if wxUserID == "" || wxUserID == "<nil>" {
			wxUserID = strings.TrimSpace(fmt.Sprint(object["wx_user_id"]))
		}
		if wxUserID == "" || wxUserID == "<nil>" {
			continue
		}
		if _, exists := seen[wxUserID]; exists {
			continue
		}
		seen[wxUserID] = struct{}{}
		result = append(result, wxUserID)
	}
	return result
}

func workFissionBoolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (s *MySQLStore) WorkRoomAutoPullEmployeeWXUserIDs(ctx context.Context, employeeIDs []int) ([]string, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return []string{}, fmt.Errorf("empty employees")
	}
	args := make([]any, 0, len(employeeIDs))
	for _, id := range employeeIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, wx_user_id
		FROM mc_work_employee
		WHERE id IN (`+placeholders(len(employeeIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[int]string{}
	for rows.Next() {
		var id int
		var wxUserID sql.NullString
		if err := rows.Scan(&id, &wxUserID); err != nil {
			return nil, err
		}
		byID[id] = nullString(wxUserID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(byID) != len(employeeIDs) {
		return nil, fmt.Errorf("employee count mismatch")
	}
	result := make([]string, 0, len(employeeIDs))
	for _, id := range employeeIDs {
		result = append(result, byID[id])
	}
	return result, nil
}

func (s *MySQLStore) WorkRoomAutoPullRoomWXChatIDs(ctx context.Context, corpID int, roomIDs []int) ([]string, error) {
	roomIDs = uniquePositiveInts(roomIDs)
	if corpID <= 0 || len(roomIDs) == 0 || len(roomIDs) > 5 {
		return nil, fmt.Errorf("群聊选择无效")
	}
	args := make([]any, 0, len(roomIDs)+1)
	args = append(args, corpID)
	for _, id := range roomIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, wx_chat_id
		FROM mc_work_room
		WHERE corp_id = ? AND id IN (`+placeholders(len(roomIDs))+`) AND wx_chat_id <> '' AND deleted_at IS NULL`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := make(map[int]string, len(roomIDs))
	for rows.Next() {
		var id int
		var wxChatID string
		if err := rows.Scan(&id, &wxChatID); err != nil {
			return nil, err
		}
		byID[id] = strings.TrimSpace(wxChatID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(roomIDs))
	for _, id := range roomIDs {
		chatID := byID[id]
		if chatID == "" {
			return nil, fmt.Errorf("群聊 %d 未同步或不属于当前企业", id)
		}
		result = append(result, chatID)
	}
	return result, nil
}

func (s *MySQLStore) CreateWorkRoomAutoPullWithLog(ctx context.Context, values dashboard.WorkRoomAutoPullWrite, operationID int) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer rollbackQuietly(tx)
	providerKind := strings.TrimSpace(values.ProviderKind)
	if providerKind == "" {
		providerKind = "contact_way"
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_work_room_auto_pull (corp_id, medium_id, qrcode_name, qrcode_url, wx_config_id, provider_kind, auto_create_room, room_base_name, room_base_id, is_verified, leading_words, tags, employees, rooms, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, values.CorpID, values.MediumID, values.QRCodeName, values.QRCodeURL, values.WXConfigID, providerKind, values.AutoCreateRoom, values.RoomBaseName, values.RoomBaseID, values.IsVerified, values.LeadingWords, values.Tags, values.Employees, values.Rooms)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := insertBusinessLog(ctx, tx, int(id), workRoomAutoPullBusinessLogPayload(values, true), 200, operationID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *MySQLStore) WorkRoomAutoPullUpdateTargetByID(ctx context.Context, id int) (dashboard.WorkRoomAutoPullUpdateTarget, bool, error) {
	var target dashboard.WorkRoomAutoPullUpdateTarget
	var wxConfigID sql.NullString
	var providerKind sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT corp_id, wx_config_id, provider_kind
		FROM mc_work_room_auto_pull
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, id).Scan(&target.CorpID, &wxConfigID, &providerKind)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WorkRoomAutoPullUpdateTarget{}, false, nil
	}
	if err != nil {
		return dashboard.WorkRoomAutoPullUpdateTarget{}, false, err
	}
	target.WXConfigID = nullString(wxConfigID)
	target.ProviderKind = strings.TrimSpace(nullString(providerKind))
	if target.ProviderKind == "" {
		target.ProviderKind = "contact_way"
	}
	return target, true, nil
}

func (s *MySQLStore) UpdateWorkRoomAutoPullWithLog(ctx context.Context, id int, values dashboard.WorkRoomAutoPullWrite, operationID int) (dashboard.WorkRoomAutoPullUpdateTarget, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.WorkRoomAutoPullUpdateTarget{}, false, err
	}
	defer rollbackQuietly(tx)
	var target dashboard.WorkRoomAutoPullUpdateTarget
	var wxConfigID sql.NullString
	var providerKind sql.NullString
	var oldRooms sql.NullString
	tenantID := 0
	err = tx.QueryRowContext(ctx, `
		SELECT p.corp_id, p.wx_config_id, p.provider_kind, p.rooms, COALESCE(c.tenant_id, 0)
		FROM mc_work_room_auto_pull AS p
		LEFT JOIN mc_corp AS c ON c.id = p.corp_id AND c.deleted_at IS NULL
		WHERE p.id = ? AND p.deleted_at IS NULL
		LIMIT 1
	`, id).Scan(&target.CorpID, &wxConfigID, &providerKind, &oldRooms, &tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WorkRoomAutoPullUpdateTarget{}, false, nil
	}
	if err != nil {
		return dashboard.WorkRoomAutoPullUpdateTarget{}, false, err
	}
	target.WXConfigID = nullString(wxConfigID)
	target.ProviderKind = strings.TrimSpace(nullString(providerKind))
	reclaimPaths := storagePathsRemoved(
		workRoomAutoPullStoragePathsFromRooms(nullString(oldRooms)),
		workRoomAutoPullStoragePathsFromRooms(values.Rooms),
	)
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_work_room_auto_pull
		SET medium_id = ?, is_verified = ?, employees = ?, tags = ?, rooms = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, values.MediumID, values.IsVerified, values.Employees, values.Tags, values.Rooms, id)
	if err != nil {
		return dashboard.WorkRoomAutoPullUpdateTarget{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return dashboard.WorkRoomAutoPullUpdateTarget{}, false, err
	}
	if affected == 0 {
		return dashboard.WorkRoomAutoPullUpdateTarget{}, false, nil
	}
	if err := insertBusinessLog(ctx, tx, id, workRoomAutoPullBusinessLogPayload(values, false), 201, operationID); err != nil {
		return dashboard.WorkRoomAutoPullUpdateTarget{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.WorkRoomAutoPullUpdateTarget{}, false, err
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return target, true, err
	}
	return target, true, nil
}

func (s *MySQLStore) UpdateWorkRoomAutoPullQRCode(ctx context.Context, id int, qrcodeURL string, configID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_work_room_auto_pull
		SET qrcode_url = ?, wx_config_id = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, qrcodeURL, configID, id)
	return err
}

func (s *MySQLStore) DeleteWorkRoomAutoPull(ctx context.Context, id int) error {
	var roomsRaw sql.NullString
	tenantID := 0
	err := s.db.QueryRowContext(ctx, `
		SELECT p.rooms, COALESCE(c.tenant_id, 0)
		FROM mc_work_room_auto_pull AS p
		LEFT JOIN mc_corp AS c ON c.id = p.corp_id AND c.deleted_at IS NULL
		WHERE p.id = ? AND p.deleted_at IS NULL
		LIMIT 1
	`, id).Scan(&roomsRaw, &tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	reclaimPaths := workRoomAutoPullStoragePathsFromRooms(nullString(roomsRaw))
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_work_room_auto_pull
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return nil
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return err
	}
	if tenantID <= 0 {
		return nil
	}
	return s.RefreshSaaSUsageCounter(ctx, tenantID, dashboard.SaaSMetricWorkRoomAutoPulls)
}

func workRoomAutoPullWhere(filter dashboard.WorkRoomAutoPullFilter) ([]string, []any) {
	where := []string{"deleted_at IS NULL", "corp_id IN (" + placeholders(len(filter.CorpIDs)) + ")", "data_source <> 'simulation'"}
	args := make([]any, 0, len(filter.CorpIDs)+len(filter.BusinessIDs)+1)
	for _, id := range filter.CorpIDs {
		args = append(args, id)
	}
	if filter.GroupID > 0 {
		where = append(where, "group_id = ?")
		args = append(args, filter.GroupID)
	}
	if filter.RestrictBusinessIDs {
		where = append(where, "id IN ("+placeholders(len(filter.BusinessIDs))+")")
		for _, id := range filter.BusinessIDs {
			args = append(args, id)
		}
	}
	if filter.QRCodeName != "" {
		where = append(where, "qrcode_name LIKE ?")
		args = append(args, "%"+filter.QRCodeName+"%")
	}
	if state := strings.TrimSpace(filter.LifecycleState); state != "" {
		where = append(where, "lifecycle_state = ?")
		args = append(args, state)
	}
	return where, args
}

func workRoomAutoPullBusinessLogPayload(values dashboard.WorkRoomAutoPullWrite, includeCreateFields bool) string {
	payload := map[string]any{
		"medium_id":   values.MediumID,
		"is_verified": values.IsVerified,
		"employees":   values.Employees,
		"tags":        values.Tags,
		"rooms":       values.Rooms,
		"updated_at":  time.Now().Format("2006-01-02 15:04:05"),
	}
	if includeCreateFields {
		payload["corp_id"] = values.CorpID
		payload["qrcode_name"] = values.QRCodeName
		payload["leading_words"] = values.LeadingWords
		payload["provider_kind"] = values.ProviderKind
		payload["auto_create_room"] = values.AutoCreateRoom
		payload["room_base_name"] = values.RoomBaseName
		payload["room_base_id"] = values.RoomBaseID
		payload["created_at"] = time.Now().Format("2006-01-02 15:04:05")
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

type roomAutoPullRoom struct {
	RoomID        int
	MaxNum        int
	RoomQRCodeURL string
}

type workRoomAutoPullRoomInfo struct {
	Name    string
	RoomMax int
}

type workRoomAutoPullRoomStat struct {
	Total      int
	ContactNum int
}

func parseRoomAutoPullRooms(raw []byte) []roomAutoPullRoom {
	if len(raw) == 0 {
		return []roomAutoPullRoom{}
	}
	var values []map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return []roomAutoPullRoom{}
	}
	rooms := make([]roomAutoPullRoom, 0, len(values))
	for _, value := range values {
		roomID := intFromAny(value["roomId"])
		if roomID == 0 {
			roomID = intFromAny(value["id"])
		}
		if roomID == 0 {
			continue
		}
		qrcodeURL := ""
		if rawURL, ok := value["roomQrcodeUrl"]; ok && rawURL != nil {
			qrcodeURL = strings.TrimSpace(fmt.Sprint(rawURL))
		}
		rooms = append(rooms, roomAutoPullRoom{
			RoomID:        roomID,
			MaxNum:        intFromAny(value["maxNum"]),
			RoomQRCodeURL: qrcodeURL,
		})
	}
	return rooms
}

func (s *MySQLStore) workRoomAutoPullEmployeeNames(ctx context.Context, employeeIDs []int) (map[int]string, error) {
	result := map[int]string{}
	employees, err := s.workRoomAutoPullEmployees(ctx, employeeIDs)
	if err != nil {
		return nil, err
	}
	for _, employee := range employees {
		result[employee.ID] = employee.Name
	}
	return result, nil
}

func (s *MySQLStore) workRoomAutoPullEmployees(ctx context.Context, employeeIDs []int) ([]dashboard.WorkRoomAutoPullEmployee, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return []dashboard.WorkRoomAutoPullEmployee{}, nil
	}
	args := make([]any, 0, len(employeeIDs))
	for _, id := range employeeIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, avatar, wx_user_id
		FROM mc_work_employee
		WHERE id IN (`+placeholders(len(employeeIDs))+`) AND deleted_at IS NULL
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	employees := make([]dashboard.WorkRoomAutoPullEmployee, 0)
	for rows.Next() {
		var employee dashboard.WorkRoomAutoPullEmployee
		var name, avatar, wxUserID sql.NullString
		if err := rows.Scan(&employee.ID, &name, &avatar, &wxUserID); err != nil {
			return nil, err
		}
		employee.EmployeeID = employee.ID
		employee.Name = nullString(name)
		employee.Avatar = nullString(avatar)
		employee.WXUserID = nullString(wxUserID)
		employee.Select = true
		employees = append(employees, employee)
	}
	return employees, rows.Err()
}

func (s *MySQLStore) workRoomAutoPullRoomsByID(ctx context.Context, roomIDs []int) (map[int]workRoomAutoPullRoomInfo, error) {
	result := map[int]workRoomAutoPullRoomInfo{}
	roomIDs = uniquePositiveInts(roomIDs)
	if len(roomIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(roomIDs))
	for _, id := range roomIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, room_max
		FROM mc_work_room
		WHERE id IN (`+placeholders(len(roomIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name sql.NullString
		var roomMax int
		if err := rows.Scan(&id, &name, &roomMax); err != nil {
			return nil, err
		}
		result[id] = workRoomAutoPullRoomInfo{Name: nullString(name), RoomMax: roomMax}
	}
	return result, rows.Err()
}

func (s *MySQLStore) workRoomAutoPullListRoomStats(ctx context.Context, roomIDs []int) (map[int]workRoomAutoPullRoomStat, error) {
	result := map[int]workRoomAutoPullRoomStat{}
	roomIDs = uniquePositiveInts(roomIDs)
	if len(roomIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(roomIDs))
	for _, id := range roomIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT room_id, contact_id, type, status
		FROM mc_work_contact_room
		WHERE room_id IN (`+placeholders(len(roomIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var roomID int
		var contactID int
		var memberType int
		var status int
		if err := rows.Scan(&roomID, &contactID, &memberType, &status); err != nil {
			return nil, err
		}
		stat := result[roomID]
		if status == 1 {
			stat.Total++
			if memberType == 2 && contactID == 0 {
				stat.ContactNum++
			}
		}
		result[roomID] = stat
	}
	return result, rows.Err()
}

func (s *MySQLStore) workRoomAutoPullShowRooms(ctx context.Context, roomPayloads []roomAutoPullRoom) ([]dashboard.WorkRoomAutoPullShowRoom, error) {
	roomIDs := make([]int, 0, len(roomPayloads))
	for _, room := range roomPayloads {
		roomIDs = append(roomIDs, room.RoomID)
	}
	roomInfo, err := s.workRoomAutoPullRoomsByID(ctx, roomIDs)
	if err != nil {
		return nil, err
	}
	counts, err := s.workRoomAutoPullActiveRoomCounts(ctx, roomIDs)
	if err != nil {
		return nil, err
	}
	rooms := make([]dashboard.WorkRoomAutoPullShowRoom, 0)
	isDrawing := false
	for _, payload := range roomPayloads {
		info, ok := roomInfo[payload.RoomID]
		if !ok {
			continue
		}
		num, ok := counts[payload.RoomID]
		if !ok {
			continue
		}
		state := 1
		if num < info.RoomMax && num < payload.MaxNum {
			if !isDrawing {
				state = 2
				isDrawing = true
			}
		} else {
			state = 3
		}
		rooms = append(rooms, dashboard.WorkRoomAutoPullShowRoom{
			RoomID:        payload.RoomID,
			RoomName:      info.Name,
			RoomMax:       info.RoomMax,
			Num:           num,
			MaxNum:        payload.MaxNum,
			RoomQRCodeURL: payload.RoomQRCodeURL,
			State:         state,
		})
	}
	return rooms, nil
}

func (s *MySQLStore) workRoomAutoPullActiveRoomCounts(ctx context.Context, roomIDs []int) (map[int]int, error) {
	result := map[int]int{}
	roomIDs = uniquePositiveInts(roomIDs)
	if len(roomIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(roomIDs)+1)
	for _, id := range roomIDs {
		args = append(args, id)
	}
	args = append(args, 1)
	rows, err := s.db.QueryContext(ctx, `
		SELECT room_id, COUNT(*)
		FROM mc_work_contact_room
		WHERE room_id IN (`+placeholders(len(roomIDs))+`) AND status = ? AND deleted_at IS NULL
		GROUP BY room_id
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var roomID int
		var total int
		if err := rows.Scan(&roomID, &total); err != nil {
			return nil, err
		}
		result[roomID] = total
	}
	return result, rows.Err()
}

func workRoomAutoPullDrawStateText(state int) string {
	switch state {
	case 1:
		return "未开始"
	case 2:
		return "拉人中"
	case 3:
		return "已拉满"
	default:
		return ""
	}
}

func (s *MySQLStore) RoomTagPullPage(ctx context.Context, filter dashboard.RoomTagPullFilter) (dashboard.RoomTagPullPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10000
	}
	page := dashboard.RoomTagPullPage{Items: []dashboard.RoomTagPullItem{}, PerPage: filter.PerPage}
	if filter.CorpID <= 0 {
		return page, nil
	}
	where, args := roomTagPullWhere(filter)
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_room_tag_pull WHERE "+strings.Join(where, " AND "), args...).Scan(&page.Total); err != nil {
		return dashboard.RoomTagPullPage{}, err
	}
	if page.Total == 0 {
		return page, nil
	}
	page.TotalPage = (page.Total + page.PerPage - 1) / page.PerPage
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, page.PerPage, (filter.Page-1)*page.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, employees, rooms, wx_tid, created_at
		FROM mc_room_tag_pull
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomTagPullPage{}, err
	}
	defer rows.Close()

	type rawItem struct {
		item        dashboard.RoomTagPullItem
		employeeIDs []int
		rooms       []roomTagPullRoomPayload
		tids        []dashboard.RoomTagPullWXTID
	}
	raws := make([]rawItem, 0)
	employeeIDs := make([]int, 0)
	activityIDs := make([]int, 0)
	for rows.Next() {
		var raw rawItem
		var employees, rooms, wxTID sql.NullString
		var createdAt sql.NullTime
		if err := rows.Scan(&raw.item.ID, &raw.item.Name, &employees, &rooms, &wxTID, &createdAt); err != nil {
			return dashboard.RoomTagPullPage{}, err
		}
		raw.item.CreatedAt = formatTime(createdAt)
		raw.employeeIDs = roomTagPullEmployeeIDs(nullString(employees))
		raw.rooms = parseRoomTagPullRooms([]byte(nullString(rooms)))
		raw.tids = parseRoomTagPullTIDs([]byte(nullString(wxTID)))
		for _, tid := range raw.tids {
			if tid.Status == 0 {
				raw.item.NoSendNum++
			}
		}
		employeeIDs = append(employeeIDs, raw.employeeIDs...)
		activityIDs = append(activityIDs, raw.item.ID)
		raws = append(raws, raw)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomTagPullPage{}, err
	}
	employeeNames, err := s.roomTagPullEmployeeNames(ctx, employeeIDs)
	if err != nil {
		return dashboard.RoomTagPullPage{}, err
	}
	counts, err := s.roomTagPullContactCounts(ctx, activityIDs)
	if err != nil {
		return dashboard.RoomTagPullPage{}, err
	}
	items := make([]dashboard.RoomTagPullItem, 0, len(raws))
	for _, raw := range raws {
		item := raw.item
		for _, employeeID := range raw.employeeIDs {
			if name, ok := employeeNames[employeeID]; ok {
				item.Employees = append(item.Employees, name)
			}
		}
		for _, room := range raw.rooms {
			if room.Name != "" {
				item.Rooms = append(item.Rooms, room.Name)
			}
		}
		count := counts[item.ID]
		item.InviteNum = count.InviteNum
		item.JoinRoomNum = count.JoinRoomNum
		item.NoInviteNum = count.NoInviteNum
		items = append(items, item)
	}
	page.Items = items
	return page, nil
}

func (s *MySQLStore) RoomTagPullByID(ctx context.Context, id int) (dashboard.RoomTagPullShow, bool, error) {
	var employees, rooms, wxTID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT employees, rooms, wx_tid
		FROM mc_room_tag_pull
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, id).Scan(&employees, &rooms, &wxTID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.RoomTagPullShow{}, false, nil
	}
	if err != nil {
		return dashboard.RoomTagPullShow{}, false, err
	}
	info := dashboard.RoomTagPullShow{}
	employeeIDs := roomTagPullEmployeeIDs(nullString(employees))
	info.Employees, err = s.roomTagPullEmployees(ctx, employeeIDs)
	if err != nil {
		return dashboard.RoomTagPullShow{}, false, err
	}
	roomPayloads := parseRoomTagPullRooms([]byte(nullString(rooms)))
	info.Rooms, err = s.roomTagPullShowRooms(ctx, roomPayloads)
	if err != nil {
		return dashboard.RoomTagPullShow{}, false, err
	}
	for _, tid := range parseRoomTagPullTIDs([]byte(nullString(wxTID))) {
		if tid.Status == 0 {
			info.NoSendNum++
		} else {
			info.SendNum++
		}
	}
	counts, err := s.roomTagPullContactCounts(ctx, []int{id})
	if err != nil {
		return dashboard.RoomTagPullShow{}, false, err
	}
	count := counts[id]
	info.JoinRoomNum = count.JoinRoomNum
	info.NoJoinRoomNum = count.NoJoinRoomNum
	info.InviteNum = count.InviteNum
	info.NoInviteNum = count.NoInviteNum
	return info, true, nil
}

func (s *MySQLStore) RoomTagPullRemindByID(ctx context.Context, id int) (dashboard.RoomTagPullRemindActivity, bool, error) {
	var employees, chooseContact, wxTID sql.NullString
	var contactNum sql.NullInt64
	var createdAt sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT employees, choose_contact, contact_num, wx_tid, DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s')
		FROM mc_room_tag_pull
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, id).Scan(&employees, &chooseContact, &contactNum, &wxTID, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.RoomTagPullRemindActivity{}, false, nil
	}
	if err != nil {
		return dashboard.RoomTagPullRemindActivity{}, false, err
	}
	employeeIDs := roomTagPullEmployeeIDs(nullString(employees))
	activity := dashboard.RoomTagPullRemindActivity{
		EmployeeIDs:   employeeIDs,
		ChooseContact: parseRoomTagPullChooseContact(nullString(chooseContact), employeeIDs),
		ContactNum:    int(contactNum.Int64),
		WXTIDs:        parseRoomTagPullTIDs([]byte(nullString(wxTID))),
		CreatedAt:     nullString(createdAt),
	}
	return activity, true, nil
}

func (s *MySQLStore) RoomTagPullCronCorps(ctx context.Context) ([]dashboard.RoomTagPullCronCorp, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM mc_corp
		WHERE deleted_at IS NULL
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	corpIDs := make([]int, 0)
	for rows.Next() {
		var corpID int
		if err := rows.Scan(&corpID); err != nil {
			rows.Close()
			return nil, err
		}
		corpIDs = append(corpIDs, corpID)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := make([]dashboard.RoomTagPullCronCorp, 0, len(corpIDs))
	for _, corpID := range corpIDs {
		item, found, err := s.loadCorpCredentialByID(ctx, s.db, corpID, false)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		secret, err := s.decodeCorpCredential(item)
		if err != nil {
			return nil, err
		}
		items = append(items, dashboard.RoomTagPullCronCorp{CorpID: item.ID, WXCorpID: item.WXCorpID, ContactSecret: secret.ContactSecret})
	}
	return items, nil
}

func (s *MySQLStore) RoomTagPullCronActivitiesByCorp(ctx context.Context, corpID int) ([]dashboard.RoomTagPullCronActivity, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, wx_tid
		FROM mc_room_tag_pull
		WHERE corp_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomTagPullCronActivity, 0)
	for rows.Next() {
		var item dashboard.RoomTagPullCronActivity
		var wxTID sql.NullString
		if err := rows.Scan(&item.ID, &wxTID); err != nil {
			return nil, err
		}
		item.WXTIDs = parseRoomTagPullTIDs([]byte(nullString(wxTID)))
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) UpdateRoomTagPullWXTIDs(ctx context.Context, activityID int, tids []dashboard.RoomTagPullWXTID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_tag_pull
		SET wx_tid = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, mustJSONStore(tids), activityID)
	return err
}

func (s *MySQLStore) RoomTagPullHasPendingContacts(ctx context.Context, activityID int) (bool, error) {
	total, err := s.countRows(ctx, `
		SELECT COUNT(*)
		FROM mc_room_tag_pull_contact
		WHERE room_tag_pull_id = ? AND send_status = 0 AND deleted_at IS NULL
	`, activityID)
	if err != nil {
		return false, err
	}
	return total > 0, nil
}

func (s *MySQLStore) UpdateRoomTagPullContactSendStatus(ctx context.Context, activityID int, wxUserID string, externalUserID string, status int) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_tag_pull_contact
		SET send_status = ?, updated_at = NOW()
		WHERE room_tag_pull_id = ? AND wx_user_id = ? AND wx_external_userid = ? AND deleted_at IS NULL
	`, status, activityID, wxUserID, externalUserID)
	return err
}

func (s *MySQLStore) RoomTagPullContactPage(ctx context.Context, filter dashboard.RoomTagPullContactFilter) (dashboard.RoomTagPullContactPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10000
	}
	page := dashboard.RoomTagPullContactPage{Items: []dashboard.RoomTagPullContactItem{}, PerPage: filter.PerPage}
	if filter.RoomTagPullID <= 0 {
		return page, nil
	}
	var contactNum sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT contact_num
		FROM mc_room_tag_pull
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, filter.RoomTagPullID).Scan(&contactNum)
	if errors.Is(err, sql.ErrNoRows) {
		return page, nil
	}
	if err != nil {
		return dashboard.RoomTagPullContactPage{}, err
	}
	page.ContactNum = int(contactNum.Int64)
	where, args := roomTagPullContactWhere(filter)
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_room_tag_pull_contact rtc WHERE "+strings.Join(where, " AND "), args...).Scan(&page.Total); err != nil {
		return dashboard.RoomTagPullContactPage{}, err
	}
	if page.Total == 0 {
		return page, nil
	}
	page.TotalPage = (page.Total + page.PerPage - 1) / page.PerPage
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, page.PerPage, (filter.Page-1)*page.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT COALESCE(c.avatar, ''), rtc.contact_name, COALESCE(e.name, ''), rtc.send_status, COALESCE(r.name, ''), rtc.is_join_room
		FROM mc_room_tag_pull_contact rtc
		LEFT JOIN mc_work_contact c ON c.id = rtc.contact_id AND c.deleted_at IS NULL
		LEFT JOIN mc_work_employee e ON e.id = rtc.employee_id AND e.deleted_at IS NULL
		LEFT JOIN mc_work_room r ON r.id = rtc.room_id AND r.deleted_at IS NULL
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY rtc.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomTagPullContactPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item dashboard.RoomTagPullContactItem
		if err := rows.Scan(&item.Avatar, &item.ContactName, &item.EmployeeName, &item.SendStatus, &item.RoomName, &item.IsJoinRoom); err != nil {
			return dashboard.RoomTagPullContactPage{}, err
		}
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomTagPullContactPage{}, err
	}
	return page, nil
}

func (s *MySQLStore) RoomTagPullEmployeeTasks(ctx context.Context, filter dashboard.RoomTagPullEmployeeTaskFilter) ([]dashboard.RoomTagPullEmployeeTask, int, error) {
	if filter.RoomTagPullID <= 0 {
		return []dashboard.RoomTagPullEmployeeTask{}, 0, nil
	}
	var wxTID sql.NullString
	var contactNum sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT wx_tid, contact_num
		FROM mc_room_tag_pull
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, filter.RoomTagPullID).Scan(&wxTID, &contactNum)
	if errors.Is(err, sql.ErrNoRows) {
		return []dashboard.RoomTagPullEmployeeTask{}, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	tids := parseRoomTagPullTIDs([]byte(nullString(wxTID)))
	taskNums := map[string]int{}
	wxUserIDs := make([]string, 0, len(tids))
	for _, tid := range tids {
		if tid.WXUserID == "" {
			continue
		}
		taskNums[tid.WXUserID]++
		wxUserIDs = append(wxUserIDs, tid.WXUserID)
	}
	employees, err := s.roomTagPullEmployeesByWXUserID(ctx, wxUserIDs)
	if err != nil {
		return nil, 0, err
	}
	contactCounts, inviteCounts, err := s.roomTagPullContactCountsByWXUserID(ctx, filter.RoomTagPullID)
	if err != nil {
		return nil, 0, err
	}
	seen := map[string]struct{}{}
	list := make([]dashboard.RoomTagPullEmployeeTask, 0)
	for _, tid := range tids {
		if tid.WXUserID == "" {
			continue
		}
		taskNum := taskNums[tid.WXUserID]
		if filter.WXUserID != "" && tid.WXUserID != filter.WXUserID {
			continue
		}
		if filter.IsSend != nil && tid.Status != *filter.IsSend {
			continue
		}
		if filter.TaskType != nil && *filter.TaskType > 0 {
			if *filter.TaskType == 1 && taskNum > 1 {
				continue
			}
			if *filter.TaskType == 2 && taskNum == 1 {
				continue
			}
		}
		employee := employees[tid.WXUserID]
		item := dashboard.RoomTagPullEmployeeTask{
			WXUserID:   tid.WXUserID,
			Status:     tid.Status,
			TaskNum:    taskNum,
			Name:       employee.Name,
			Avatar:     employee.Avatar,
			ContactNum: contactCounts[tid.WXUserID],
			InviteNum:  inviteCounts[tid.WXUserID],
		}
		key := fmt.Sprintf("%s/%d/%d/%d/%d/%s/%s", item.WXUserID, item.Status, item.TaskNum, item.ContactNum, item.InviteNum, item.Name, item.Avatar)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		list = append(list, item)
	}
	return list, int(contactNum.Int64), nil
}

func (s *MySQLStore) RoomTagPullRooms(ctx context.Context, filter dashboard.RoomTagPullRoomFilter) ([]dashboard.RoomTagPullRoom, error) {
	if filter.CorpID <= 0 {
		return []dashboard.RoomTagPullRoom{}, nil
	}
	filter.EmployeeIDs = uniquePositiveInts(filter.EmployeeIDs)
	where := []string{"corp_id = ?", "deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if len(filter.EmployeeIDs) > 0 {
		where = append(where, "owner_id IN ("+placeholders(len(filter.EmployeeIDs))+")")
		for _, employeeID := range filter.EmployeeIDs {
			args = append(args, employeeID)
		}
	}
	if filter.Name != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+filter.Name+"%")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, wx_chat_id, name, owner_id, room_max
		FROM mc_work_room
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rooms := make([]dashboard.RoomTagPullRoom, 0)
	roomIDs := make([]int, 0)
	for rows.Next() {
		var room dashboard.RoomTagPullRoom
		if err := rows.Scan(&room.ID, &room.WXChatID, &room.Name, &room.OwnerID, &room.RoomMax); err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
		roomIDs = append(roomIDs, room.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	counts, err := s.roomTagPullRoomContactCounts(ctx, roomIDs)
	if err != nil {
		return nil, err
	}
	auditStatus := map[int]int{}
	if filter.Type == 2 {
		auditStatus, err = s.roomTagPullRoomAuditStatus(ctx, roomIDs)
		if err != nil {
			return nil, err
		}
	}
	for index := range rooms {
		rooms[index].ContactNum = counts[rooms[index].ID]
		if filter.Type == 2 {
			rooms[index].AuditStatus = auditStatus[rooms[index].ID]
		}
	}
	return rooms, nil
}

func (s *MySQLStore) CountRoomTagPullSearchContacts(ctx context.Context, corpID int, search dashboard.RoomTagPullContactSearch) (int, error) {
	search.Employees = uniquePositiveInts(search.Employees)
	if corpID <= 0 || len(search.Employees) == 0 {
		return 0, nil
	}
	where, args := roomTagPullSearchWhere(corpID, search)
	var total int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT work_contact.id)
		FROM mc_work_contact work_contact
		JOIN mc_work_contact_employee ce ON work_contact.id = ce.contact_id AND ce.deleted_at IS NULL
		JOIN mc_work_employee e ON e.id = ce.employee_id AND e.deleted_at IS NULL
		JOIN mc_work_contact_tag_pivot ctp ON work_contact.id = ctp.contact_id AND ctp.deleted_at IS NULL
		WHERE `+strings.Join(where, " AND ")+`
	`, args...).Scan(&total)
	return total, err
}

func (s *MySQLStore) CountRoomTagPullFilteredContacts(ctx context.Context, corpID int, search dashboard.RoomTagPullContactSearch, rooms []dashboard.RoomTagPullWriteRoom, filterContact int) (int, error) {
	search.Employees = uniquePositiveInts(search.Employees)
	if corpID <= 0 || len(search.Employees) == 0 {
		return 0, nil
	}
	if filterContact == 0 {
		return s.CountRoomTagPullSearchContacts(ctx, corpID, search)
	}
	contacts, err := s.roomTagPullSearchContacts(ctx, corpID, search)
	if err != nil {
		return 0, err
	}
	if len(contacts) == 0 {
		return 0, nil
	}
	count := 0
	start := 0
	for _, room := range rooms {
		if room.Num <= 0 {
			continue
		}
		if len(sliceRoomTagPullStoreContacts(contacts, start, room.Num)) == 0 {
			break
		}
		start += room.Num
		roomCount, err := s.roomTagPullRoomContactCount(ctx, room.ID)
		if err != nil {
			return 0, err
		}
		count += roomCount
	}
	return count, nil
}

func (s *MySQLStore) RoomTagPullSendTargets(ctx context.Context, corpID int, employeeIDs []int, search dashboard.RoomTagPullContactSearch) ([]dashboard.RoomTagPullSendTarget, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	if corpID <= 0 || len(employeeIDs) == 0 {
		return []dashboard.RoomTagPullSendTarget{}, nil
	}
	targets := make([]dashboard.RoomTagPullSendTarget, 0, len(employeeIDs))
	for _, employeeID := range employeeIDs {
		wxUserID, err := s.roomTagPullEmployeeWXUserID(ctx, employeeID)
		if err != nil {
			return nil, err
		}
		if wxUserID == "" {
			continue
		}
		contacts, err := s.roomTagPullSearchContactsByEmployee(ctx, corpID, employeeID, search)
		if err != nil {
			return nil, err
		}
		targets = append(targets, dashboard.RoomTagPullSendTarget{
			EmployeeID: employeeID,
			WXUserID:   wxUserID,
			Contacts:   contacts,
		})
	}
	return targets, nil
}

func (s *MySQLStore) CreateRoomTagPull(ctx context.Context, values dashboard.RoomTagPullWrite) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_room_tag_pull
			(name, employees, choose_contact, guide, rooms, filter_contact, contact_num, wx_tid, tenant_id, corp_id, create_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, values.Name, values.Employees, values.ChooseContactJSON, values.Guide, values.RoomsJSON, values.FilterContact, values.ContactNum, values.WXTIDJSON, values.TenantID, values.CorpID, values.CreateUserID)
	if err != nil {
		return 0, err
	}
	id64, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	id := int(id64)
	for _, employeeID := range values.EmployeeIDs {
		contacts, err := s.roomTagPullSearchContactsByEmployee(ctx, values.CorpID, employeeID, values.ChooseContact)
		if err != nil {
			return 0, err
		}
		start := 0
		for _, room := range values.Rooms {
			if room.Num <= 0 {
				continue
			}
			sendContacts := sliceRoomTagPullStoreContacts(contacts, start, room.Num)
			if len(sendContacts) == 0 {
				break
			}
			start += room.Num
			roomMembers, err := s.roomTagPullRoomContactIDSet(ctx, room.ID)
			if err != nil {
				return 0, err
			}
			for _, contact := range sendContacts {
				isJoinRoom := 0
				if _, ok := roomMembers[contact.ContactID]; ok {
					isJoinRoom = 1
					if values.FilterContact == 1 {
						continue
					}
				}
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO mc_room_tag_pull_contact
						(room_tag_pull_id, contact_id, wx_external_userid, contact_name, employee_id, wx_user_id, send_status, is_join_room, room_id, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, NOW(), NOW())
				`, id, contact.ContactID, contact.WXExternalUserID, contact.ContactName, contact.EmployeeID, contact.WXUserID, isJoinRoom, room.ID); err != nil {
					return 0, err
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *MySQLStore) DeleteRoomTagPull(ctx context.Context, id int) (bool, error) {
	var roomsRaw sql.NullString
	tenantID := 0
	err := s.db.QueryRowContext(ctx, `
		SELECT r.rooms, COALESCE(r.tenant_id, c.tenant_id, 0)
		FROM mc_room_tag_pull AS r
		LEFT JOIN mc_corp AS c ON c.id = r.corp_id AND c.deleted_at IS NULL
		WHERE r.id = ? AND r.deleted_at IS NULL
		LIMIT 1
	`, id).Scan(&roomsRaw, &tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	reclaimPaths := roomTagPullStoragePathsFromRooms(nullString(roomsRaw))
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_tag_pull
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return true, err
	}
	if tenantID > 0 {
		if err := s.RefreshSaaSUsageCounter(ctx, tenantID, dashboard.SaaSMetricRoomTagPulls); err != nil {
			return true, err
		}
	}
	return affected > 0, nil
}

func roomTagPullWhere(filter dashboard.RoomTagPullFilter) ([]string, []any) {
	where := []string{"corp_id = ?", "deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if filter.CreatorID != nil {
		where = append(where, "create_user_id = ?")
		args = append(args, *filter.CreatorID)
	}
	if filter.Name != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+filter.Name+"%")
	}
	return where, args
}

func roomTagPullContactWhere(filter dashboard.RoomTagPullContactFilter) ([]string, []any) {
	where := []string{"rtc.room_tag_pull_id = ?", "rtc.deleted_at IS NULL"}
	args := []any{filter.RoomTagPullID}
	if filter.ContactName != "" {
		where = append(where, "rtc.contact_name LIKE ?")
		args = append(args, "%"+filter.ContactName+"%")
	}
	if filter.WXUserID != "" {
		where = append(where, "rtc.wx_user_id = ?")
		args = append(args, filter.WXUserID)
	}
	if filter.SendStatus != nil {
		where = append(where, "rtc.send_status = ?")
		args = append(args, *filter.SendStatus)
	}
	if filter.IsJoinRoom != nil {
		where = append(where, "rtc.is_join_room = ?")
		args = append(args, *filter.IsJoinRoom)
	}
	if filter.RoomID != nil {
		where = append(where, "rtc.room_id = ?")
		args = append(args, *filter.RoomID)
	}
	return where, args
}

func roomTagPullSearchWhere(corpID int, search dashboard.RoomTagPullContactSearch) ([]string, []any) {
	where := []string{"work_contact.corp_id = ?", "work_contact.deleted_at IS NULL", "ce.employee_id IN (" + placeholders(len(search.Employees)) + ")"}
	args := []any{corpID}
	for _, employeeID := range search.Employees {
		args = append(args, employeeID)
	}
	if search.IsAll == 1 {
		if search.Gender != nil {
			where = append(where, "work_contact.gender = ?")
			args = append(args, *search.Gender)
		}
		if search.StartTime != "" {
			where = append(where, "work_contact.created_at > ?")
			args = append(args, search.StartTime)
		}
		if search.EndTime != "" {
			where = append(where, "work_contact.created_at < ?")
			args = append(args, search.EndTime)
		}
		search.TagIDs = uniquePositiveInts(search.TagIDs)
		if len(search.TagIDs) > 0 {
			where = append(where, "ctp.contact_tag_id IN ("+placeholders(len(search.TagIDs))+")")
			for _, tagID := range search.TagIDs {
				args = append(args, tagID)
			}
		}
	}
	return where, args
}

func (s *MySQLStore) roomTagPullSearchContacts(ctx context.Context, corpID int, search dashboard.RoomTagPullContactSearch) ([]dashboard.RoomTagPullSendContact, error) {
	search.Employees = uniquePositiveInts(search.Employees)
	if corpID <= 0 || len(search.Employees) == 0 {
		return []dashboard.RoomTagPullSendContact{}, nil
	}
	where, args := roomTagPullSearchWhere(corpID, search)
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT work_contact.id, work_contact.wx_external_userid, work_contact.name, ce.employee_id, e.wx_user_id
		FROM mc_work_contact work_contact
		JOIN mc_work_contact_employee ce ON work_contact.id = ce.contact_id AND ce.deleted_at IS NULL
		JOIN mc_work_employee e ON e.id = ce.employee_id AND e.deleted_at IS NULL
		JOIN mc_work_contact_tag_pivot ctp ON work_contact.id = ctp.contact_id AND ctp.deleted_at IS NULL
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY work_contact.id ASC, ce.employee_id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	contacts := make([]dashboard.RoomTagPullSendContact, 0)
	for rows.Next() {
		var contact dashboard.RoomTagPullSendContact
		var externalUserID, name, wxUserID sql.NullString
		if err := rows.Scan(&contact.ContactID, &externalUserID, &name, &contact.EmployeeID, &wxUserID); err != nil {
			return nil, err
		}
		contact.WXExternalUserID = nullString(externalUserID)
		contact.ContactName = nullString(name)
		contact.WXUserID = nullString(wxUserID)
		contacts = append(contacts, contact)
	}
	return contacts, rows.Err()
}

func (s *MySQLStore) roomTagPullSearchContactsByEmployee(ctx context.Context, corpID int, employeeID int, search dashboard.RoomTagPullContactSearch) ([]dashboard.RoomTagPullSendContact, error) {
	search.Employees = []int{employeeID}
	return s.roomTagPullSearchContacts(ctx, corpID, search)
}

func (s *MySQLStore) roomTagPullEmployeeWXUserID(ctx context.Context, employeeID int) (string, error) {
	var wxUserID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT wx_user_id
		FROM mc_work_employee
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, employeeID).Scan(&wxUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return nullString(wxUserID), nil
}

func (s *MySQLStore) roomTagPullRoomContactIDSet(ctx context.Context, roomID int) (map[int]struct{}, error) {
	result := map[int]struct{}{}
	rows, err := s.db.QueryContext(ctx, `
		SELECT contact_id
		FROM mc_work_contact_room
		WHERE room_id = ? AND contact_id > 0 AND type = 2 AND status = 1 AND deleted_at IS NULL
	`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var contactID int
		if err := rows.Scan(&contactID); err != nil {
			return nil, err
		}
		result[contactID] = struct{}{}
	}
	return result, rows.Err()
}

func (s *MySQLStore) roomTagPullRoomContactCount(ctx context.Context, roomID int) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_work_contact_room
		WHERE room_id = ? AND contact_id > 0 AND type = 2 AND status = 1 AND deleted_at IS NULL
	`, roomID).Scan(&count)
	return count, err
}

func sliceRoomTagPullStoreContacts(contacts []dashboard.RoomTagPullSendContact, start int, length int) []dashboard.RoomTagPullSendContact {
	if start >= len(contacts) || length <= 0 {
		return []dashboard.RoomTagPullSendContact{}
	}
	end := start + length
	if end > len(contacts) {
		end = len(contacts)
	}
	return contacts[start:end]
}

type roomTagPullContactCount struct {
	InviteNum     int
	NoInviteNum   int
	JoinRoomNum   int
	NoJoinRoomNum int
}

func (s *MySQLStore) roomTagPullContactCounts(ctx context.Context, activityIDs []int) (map[int]roomTagPullContactCount, error) {
	result := map[int]roomTagPullContactCount{}
	activityIDs = uniquePositiveInts(activityIDs)
	if len(activityIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(activityIDs))
	for _, id := range activityIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT room_tag_pull_id,
		       COALESCE(SUM(CASE WHEN send_status = 1 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN send_status = 0 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN is_join_room = 1 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN is_join_room = 0 THEN 1 ELSE 0 END), 0)
		FROM mc_room_tag_pull_contact
		WHERE room_tag_pull_id IN (`+placeholders(len(activityIDs))+`) AND deleted_at IS NULL
		GROUP BY room_tag_pull_id
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var count roomTagPullContactCount
		if err := rows.Scan(&id, &count.InviteNum, &count.NoInviteNum, &count.JoinRoomNum, &count.NoJoinRoomNum); err != nil {
			return nil, err
		}
		result[id] = count
	}
	return result, rows.Err()
}

type roomTagPullRoomPayload struct {
	ID   int
	Name string
}

func parseRoomTagPullTIDs(raw []byte) []dashboard.RoomTagPullWXTID {
	if len(raw) == 0 {
		return []dashboard.RoomTagPullWXTID{}
	}
	var values []map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return []dashboard.RoomTagPullWXTID{}
	}
	result := make([]dashboard.RoomTagPullWXTID, 0, len(values))
	for _, value := range values {
		wxUserID := strings.TrimSpace(fmt.Sprint(value["wxUserId"]))
		if wxUserID == "" || wxUserID == "<nil>" {
			wxUserID = strings.TrimSpace(fmt.Sprint(value["wx_user_id"]))
		}
		if wxUserID == "" || wxUserID == "<nil>" {
			continue
		}
		tid := strings.TrimSpace(fmt.Sprint(value["tid"]))
		if tid == "<nil>" {
			tid = ""
		}
		result = append(result, dashboard.RoomTagPullWXTID{
			WXUserID: wxUserID,
			TID:      tid,
			Status:   intFromAny(value["status"]),
		})
	}
	return result
}

func parseRoomTagPullRooms(raw []byte) []roomTagPullRoomPayload {
	if len(raw) == 0 {
		return []roomTagPullRoomPayload{}
	}
	var values []map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return []roomTagPullRoomPayload{}
	}
	result := make([]roomTagPullRoomPayload, 0, len(values))
	for _, value := range values {
		id := intFromAny(value["id"])
		if id == 0 {
			id = intFromAny(value["roomId"])
		}
		if id == 0 {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(value["name"]))
		if name == "<nil>" {
			name = ""
		}
		result = append(result, roomTagPullRoomPayload{ID: id, Name: name})
	}
	return result
}

func roomTagPullEmployeeIDs(raw string) []int {
	if strings.TrimSpace(raw) == "" {
		return []int{}
	}
	values := make([]int, 0)
	for _, part := range strings.Split(raw, ",") {
		id := intFromAny(part)
		if id > 0 {
			values = append(values, id)
		}
	}
	return uniquePositiveInts(values)
}

func parseRoomTagPullChooseContact(raw string, fallbackEmployeeIDs []int) dashboard.RoomTagPullContactSearch {
	search := dashboard.RoomTagPullContactSearch{Employees: uniquePositiveInts(fallbackEmployeeIDs)}
	if strings.TrimSpace(raw) == "" {
		return search
	}
	values := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return search
	}
	if value, ok := roomTagPullJSONValue(values, "employees", "Employees"); ok {
		if employees := intsFromAny(value); len(employees) > 0 {
			search.Employees = employees
		}
	}
	if value, ok := roomTagPullJSONValue(values, "is_all", "isAll", "IsAll"); ok {
		search.IsAll = intFromAny(value)
	}
	if value, ok := roomTagPullJSONValue(values, "gender", "Gender"); ok && roomTagPullStringFromAny(value) != "" {
		gender := intFromAny(value)
		search.Gender = &gender
	}
	if value, ok := roomTagPullJSONValue(values, "start_time", "startTime", "StartTime"); ok {
		search.StartTime = roomTagPullStringFromAny(value)
	}
	if value, ok := roomTagPullJSONValue(values, "end_time", "endTime", "EndTime"); ok {
		search.EndTime = roomTagPullStringFromAny(value)
	}
	if value, ok := roomTagPullJSONValue(values, "tag_ids", "tagIds", "TagIDs"); ok {
		search.TagIDs = intsFromAny(value)
	}
	return search
}

func roomTagPullJSONValue(values map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		value, ok := values[key]
		if ok {
			return value, true
		}
	}
	return nil, false
}

func roomTagPullStringFromAny(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func (s *MySQLStore) roomTagPullEmployeeNames(ctx context.Context, employeeIDs []int) (map[int]string, error) {
	result := map[int]string{}
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(employeeIDs))
	for _, id := range employeeIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name
		FROM mc_work_employee
		WHERE id IN (`+placeholders(len(employeeIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name sql.NullString
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		result[id] = nullString(name)
	}
	return result, rows.Err()
}

func (s *MySQLStore) roomTagPullEmployees(ctx context.Context, employeeIDs []int) ([]dashboard.RoomTagPullEmployee, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return []dashboard.RoomTagPullEmployee{}, nil
	}
	args := make([]any, 0, len(employeeIDs))
	for _, id := range employeeIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT name, avatar, wx_user_id
		FROM mc_work_employee
		WHERE id IN (`+placeholders(len(employeeIDs))+`) AND deleted_at IS NULL
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]dashboard.RoomTagPullEmployee, 0)
	for rows.Next() {
		var employee dashboard.RoomTagPullEmployee
		var name, avatar, wxUserID sql.NullString
		if err := rows.Scan(&name, &avatar, &wxUserID); err != nil {
			return nil, err
		}
		employee.Name = nullString(name)
		employee.Avatar = nullString(avatar)
		employee.WXUserID = nullString(wxUserID)
		result = append(result, employee)
	}
	return result, rows.Err()
}

func (s *MySQLStore) roomTagPullEmployeesByWXUserID(ctx context.Context, wxUserIDs []string) (map[string]dashboard.RoomTagPullEmployee, error) {
	result := map[string]dashboard.RoomTagPullEmployee{}
	wxUserIDs = uniqueNonEmptyStrings(wxUserIDs)
	if len(wxUserIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(wxUserIDs))
	for _, id := range wxUserIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT name, avatar, wx_user_id
		FROM mc_work_employee
		WHERE wx_user_id IN (`+placeholders(len(wxUserIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var employee dashboard.RoomTagPullEmployee
		var name, avatar, wxUserID sql.NullString
		if err := rows.Scan(&name, &avatar, &wxUserID); err != nil {
			return nil, err
		}
		employee.Name = nullString(name)
		employee.Avatar = nullString(avatar)
		employee.WXUserID = nullString(wxUserID)
		result[employee.WXUserID] = employee
	}
	return result, rows.Err()
}

func (s *MySQLStore) roomTagPullShowRooms(ctx context.Context, payloads []roomTagPullRoomPayload) ([]dashboard.RoomTagPullShowRoom, error) {
	roomIDs := make([]int, 0, len(payloads))
	for _, payload := range payloads {
		roomIDs = append(roomIDs, payload.ID)
	}
	info, err := s.roomTagPullRoomInfo(ctx, roomIDs)
	if err != nil {
		return nil, err
	}
	counts, err := s.roomTagPullRoomContactCounts(ctx, roomIDs)
	if err != nil {
		return nil, err
	}
	result := make([]dashboard.RoomTagPullShowRoom, 0, len(payloads))
	for _, payload := range payloads {
		roomInfo := info[payload.ID]
		name := payload.Name
		if name == "" {
			name = roomInfo.Name
		}
		result = append(result, dashboard.RoomTagPullShowRoom{
			ID:         payload.ID,
			Name:       name,
			RoomMax:    roomInfo.RoomMax,
			ContactNum: counts[payload.ID],
		})
	}
	return result, nil
}

type roomTagPullRoomInfo struct {
	Name    string
	RoomMax int
}

func (s *MySQLStore) roomTagPullRoomInfo(ctx context.Context, roomIDs []int) (map[int]roomTagPullRoomInfo, error) {
	result := map[int]roomTagPullRoomInfo{}
	roomIDs = uniquePositiveInts(roomIDs)
	if len(roomIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(roomIDs))
	for _, id := range roomIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, room_max
		FROM mc_work_room
		WHERE id IN (`+placeholders(len(roomIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var info roomTagPullRoomInfo
		if err := rows.Scan(&id, &info.Name, &info.RoomMax); err != nil {
			return nil, err
		}
		result[id] = info
	}
	return result, rows.Err()
}

func (s *MySQLStore) roomTagPullRoomContactCounts(ctx context.Context, roomIDs []int) (map[int]int, error) {
	result := map[int]int{}
	roomIDs = uniquePositiveInts(roomIDs)
	if len(roomIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(roomIDs))
	for _, id := range roomIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT room_id, COUNT(*)
		FROM mc_work_contact_room
		WHERE room_id IN (`+placeholders(len(roomIDs))+`) AND contact_id > 0 AND status = 1 AND deleted_at IS NULL
		GROUP BY room_id
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var roomID int
		var count int
		if err := rows.Scan(&roomID, &count); err != nil {
			return nil, err
		}
		result[roomID] = count
	}
	return result, rows.Err()
}

func (s *MySQLStore) roomTagPullRoomAuditStatus(ctx context.Context, roomIDs []int) (map[int]int, error) {
	result := map[int]int{}
	roomIDs = uniquePositiveInts(roomIDs)
	if len(roomIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(roomIDs))
	for _, id := range roomIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT cr.room_id
		FROM mc_work_contact_room cr
		JOIN mc_work_employee e ON e.id = cr.employee_id AND e.audit_status = 1 AND e.deleted_at IS NULL
		WHERE cr.room_id IN (`+placeholders(len(roomIDs))+`) AND cr.employee_id > 0 AND cr.type = 1 AND cr.status = 1 AND cr.deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var roomID int
		if err := rows.Scan(&roomID); err != nil {
			return nil, err
		}
		result[roomID] = 1
	}
	return result, rows.Err()
}

func (s *MySQLStore) roomTagPullContactCountsByWXUserID(ctx context.Context, roomTagPullID int) (map[string]int, map[string]int, error) {
	contactCounts := map[string]int{}
	inviteCounts := map[string]int{}
	rows, err := s.db.QueryContext(ctx, `
		SELECT wx_user_id, COUNT(*), COALESCE(SUM(CASE WHEN send_status = 1 THEN 1 ELSE 0 END), 0)
		FROM mc_room_tag_pull_contact
		WHERE room_tag_pull_id = ? AND deleted_at IS NULL
		GROUP BY wx_user_id
	`, roomTagPullID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var wxUserID string
		var contactCount int
		var inviteCount int
		if err := rows.Scan(&wxUserID, &contactCount, &inviteCount); err != nil {
			return nil, nil, err
		}
		contactCounts[wxUserID] = contactCount
		inviteCounts[wxUserID] = inviteCount
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return contactCounts, inviteCounts, nil
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (s *MySQLStore) WorkRoomMemberStats(ctx context.Context, workRoomID int) ([]dashboard.WorkRoomMemberStat, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT status, join_time, out_time
		FROM mc_work_contact_room
		WHERE room_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`, workRoomID)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	members := make([]dashboard.WorkRoomMemberStat, 0)
	for rows.Next() {
		var member dashboard.WorkRoomMemberStat
		var joinTime sql.NullTime
		var outTime sql.NullString
		if err := rows.Scan(&member.Status, &joinTime, &outTime); err != nil {
			return nil, false, err
		}
		member.JoinTime = formatTime(joinTime)
		member.OutTime = nullString(outTime)
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return members, len(members) > 0, nil
}

type workRoomIndexCounts struct {
	MemberNum  int
	InRoomNum  int
	OutRoomNum int
}

func workRoomIndexWhere(filter dashboard.WorkRoomIndexFilter) ([]string, []any) {
	where := []string{"corp_id = ?", "deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if filter.RestrictOwner || len(filter.OwnerIDs) > 0 {
		where = append(where, "owner_id IN ("+placeholders(len(filter.OwnerIDs))+")")
		for _, id := range filter.OwnerIDs {
			args = append(args, id)
		}
	}
	if filter.RoomGroupID != nil {
		where = append(where, "room_group_id = ?")
		args = append(args, *filter.RoomGroupID)
	}
	if filter.Name != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+filter.Name+"%")
	}
	if filter.Status != nil {
		where = append(where, "status = ?")
		args = append(args, *filter.Status)
	}
	if filter.StartTime != "" {
		where = append(where, "create_time >= ?")
		args = append(args, filter.StartTime+" 00:00:00")
	}
	if filter.EndTime != "" {
		where = append(where, "create_time <= ?")
		args = append(args, filter.EndTime+" 00:00:00")
	}
	return where, args
}

func (s *MySQLStore) workRoomOwnerNames(ctx context.Context, ownerIDs []int) (map[int]string, error) {
	ownerIDs = uniquePositiveInts(ownerIDs)
	if len(ownerIDs) == 0 {
		return map[int]string{}, nil
	}
	args := make([]any, 0, len(ownerIDs))
	for _, id := range ownerIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT employee.id, employee.name, corp.name
		FROM mc_work_employee AS employee
		LEFT JOIN mc_corp AS corp ON corp.id = employee.corp_id AND corp.deleted_at IS NULL
		WHERE employee.id IN (`+placeholders(len(ownerIDs))+`) AND employee.deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := make(map[int]string, len(ownerIDs))
	for rows.Next() {
		var id int
		var employeeName, corpName sql.NullString
		if err := rows.Scan(&id, &employeeName, &corpName); err != nil {
			return nil, err
		}
		name := nullString(employeeName)
		corp := nullString(corpName)
		if name != "" && corp != "" {
			names[id] = corp + "-" + name
		} else {
			names[id] = name
		}
	}
	return names, rows.Err()
}

func (s *MySQLStore) workRoomGroupNames(ctx context.Context, groupIDs []int) (map[int]string, error) {
	groupIDs = uniquePositiveInts(groupIDs)
	if len(groupIDs) == 0 {
		return map[int]string{}, nil
	}
	args := make([]any, 0, len(groupIDs))
	for _, id := range groupIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name
		FROM mc_work_room_group
		WHERE id IN (`+placeholders(len(groupIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := make(map[int]string, len(groupIDs))
	for rows.Next() {
		var id int
		var name sql.NullString
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		names[id] = nullString(name)
	}
	return names, rows.Err()
}

func (s *MySQLStore) workRoomIndexMemberCounts(ctx context.Context, roomIDs []int) (map[int]workRoomIndexCounts, error) {
	roomIDs = uniquePositiveInts(roomIDs)
	if len(roomIDs) == 0 {
		return map[int]workRoomIndexCounts{}, nil
	}
	today := time.Now().Format("2006-01-02") + " 00:00:00"
	args := []any{today, today}
	for _, id := range roomIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			room_id,
			COALESCE(SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN join_time >= ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN out_time >= ? THEN 1 ELSE 0 END), 0)
		FROM mc_work_contact_room
		WHERE room_id IN (`+placeholders(len(roomIDs))+`) AND deleted_at IS NULL
		GROUP BY room_id
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int]workRoomIndexCounts, len(roomIDs))
	for rows.Next() {
		var roomID int
		var count workRoomIndexCounts
		if err := rows.Scan(&roomID, &count.MemberNum, &count.InRoomNum, &count.OutRoomNum); err != nil {
			return nil, err
		}
		counts[roomID] = count
	}
	return counts, rows.Err()
}

func (s *MySQLStore) WorkRoomGroupPage(ctx context.Context, corpID int, page int, perPage int) (dashboard.WorkRoomGroupPage, error) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 10
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_work_room_group
		WHERE corp_id = ? AND deleted_at IS NULL
	`, corpID).Scan(&total); err != nil {
		return dashboard.WorkRoomGroupPage{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + perPage - 1) / perPage
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, corp_id, name, created_at
		FROM mc_work_room_group
		WHERE corp_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, corpID, perPage, (page-1)*perPage)
	if err != nil {
		return dashboard.WorkRoomGroupPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.WorkRoomGroupItem, 0)
	for rows.Next() {
		var item dashboard.WorkRoomGroupItem
		var createdAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.CorpID, &item.Name, &createdAt); err != nil {
			return dashboard.WorkRoomGroupPage{}, err
		}
		item.CreatedAt = formatTime(createdAt)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkRoomGroupPage{}, err
	}
	return dashboard.WorkRoomGroupPage{Items: items, Total: total, TotalPage: totalPage, PerPage: perPage}, nil
}

func (s *MySQLStore) WorkRoomGroupByID(ctx context.Context, groupID int) (dashboard.WorkRoomGroupItem, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, name, created_at
		FROM mc_work_room_group
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, groupID)
	var item dashboard.WorkRoomGroupItem
	var createdAt sql.NullTime
	if err := row.Scan(&item.ID, &item.CorpID, &item.Name, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dashboard.WorkRoomGroupItem{}, false, nil
		}
		return dashboard.WorkRoomGroupItem{}, false, err
	}
	item.CreatedAt = formatTime(createdAt)
	return item, true, nil
}

func (s *MySQLStore) UpdateWorkRoomsGroup(ctx context.Context, values dashboard.WorkRoomBatchUpdateValues) (int, error) {
	roomIDs := uniquePositiveInts(values.WorkRoomIDs)
	if values.CorpID <= 0 || len(roomIDs) == 0 {
		return 0, nil
	}
	args := make([]any, 0, len(roomIDs)+2)
	args = append(args, values.WorkRoomGroupID, values.CorpID)
	for _, id := range roomIDs {
		args = append(args, id)
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_work_room
		SET room_group_id = ?, updated_at = NOW()
		WHERE corp_id = ? AND deleted_at IS NULL AND id IN (`+placeholders(len(roomIDs))+`)
	`, args...)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

func (s *MySQLStore) WorkRoomGroupNameExists(ctx context.Context, corpID int, name string, excludeGroupID int) (bool, error) {
	args := []any{corpID, name}
	query := `
		SELECT COUNT(*)
		FROM mc_work_room_group
		WHERE corp_id = ? AND name = ? AND deleted_at IS NULL
	`
	if excludeGroupID > 0 {
		query += ` AND id <> ?`
		args = append(args, excludeGroupID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return false, err
	}
	return total > 0, nil
}

func (s *MySQLStore) CreateWorkRoomGroup(ctx context.Context, values dashboard.WorkRoomGroupWrite) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_work_room_group (corp_id, name, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())
	`, values.CorpID, values.Name)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func (s *MySQLStore) UpdateWorkRoomGroup(ctx context.Context, groupID int, name string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_work_room_group
		SET name = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, name, groupID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) DeleteWorkRoomGroupReassignRooms(ctx context.Context, groupID int) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)

	result, err := tx.ExecContext(ctx, `
		UPDATE mc_work_room_group
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, groupID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_room
		SET room_group_id = 0, updated_at = NOW()
		WHERE room_group_id = ? AND deleted_at IS NULL
	`, groupID); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *MySQLStore) activeRoomMemberCounts(ctx context.Context, roomIDs []int) (map[int]int, error) {
	roomIDs = uniquePositiveInts(roomIDs)
	result := make(map[int]int, len(roomIDs))
	if len(roomIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(roomIDs))
	for _, id := range roomIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT room_id, COUNT(id)
		FROM mc_work_contact_room
		WHERE status = 1 AND room_id IN (`+placeholders(len(roomIDs))+`) AND deleted_at IS NULL
		GROUP BY room_id
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var roomID, total int
		if err := rows.Scan(&roomID, &total); err != nil {
			return nil, err
		}
		result[roomID] = total
	}
	return result, rows.Err()
}

type workContactRoomRaw struct {
	ID         int
	WXUserID   string
	ContactID  int
	EmployeeID int
	RoomID     int
	JoinScene  int
	Type       int
	Status     int
	JoinTime   string
	OutTime    string
}

type workContactIndexContact struct {
	BusinessNo string
	Name       string
	Avatar     string
	Gender     int
}

func (s *MySQLStore) workContactIndexContactIDs(ctx context.Context, filter dashboard.WorkContactIndexFilter) ([]int, []int, bool, error) {
	var contactIDs []int
	hasContactFilter := false
	intersect := func(ids []int) {
		ids = uniquePositiveInts(ids)
		if !hasContactFilter {
			contactIDs = ids
			hasContactFilter = true
			return
		}
		contactIDs = intersectPositiveInts(contactIDs, ids)
	}
	if filter.KeyWords != "" {
		ids, err := s.workContactIDsByStringColumn(ctx, filter.CorpID, "name", "LIKE", "%"+filter.KeyWords+"%")
		if err != nil {
			return nil, nil, false, err
		}
		if len(ids) == 0 {
			return nil, nil, true, nil
		}
		intersect(ids)
	}
	if filter.BusinessNo != "" {
		ids, err := s.workContactIDsByStringColumn(ctx, filter.CorpID, "business_no", "=", filter.BusinessNo)
		if err != nil {
			return nil, nil, false, err
		}
		if len(ids) == 0 {
			return nil, nil, true, nil
		}
		intersect(ids)
	}
	if filter.Gender != nil {
		ids, err := s.workContactIDsByGender(ctx, filter.CorpID, *filter.Gender)
		if err != nil {
			return nil, nil, false, err
		}
		if len(ids) == 0 {
			return nil, nil, true, nil
		}
		intersect(ids)
	}
	if filter.FieldID != nil {
		ids, err := s.contactIDsByField(ctx, *filter.FieldID, filter.FieldValue)
		if err != nil {
			return nil, nil, false, err
		}
		if len(ids) == 0 {
			return nil, nil, true, nil
		}
		intersect(ids)
	}
	if len(filter.RoomIDs) > 0 {
		ids, err := s.contactIDsByRooms(ctx, filter.RoomIDs)
		if err != nil {
			return nil, nil, false, err
		}
		if len(ids) == 0 {
			return nil, nil, true, nil
		}
		intersect(ids)
	}

	var noContactIDs []int
	if filter.GroupNum != nil {
		roomCounts, err := s.contactRoomCounts(ctx)
		if err != nil {
			return nil, nil, false, err
		}
		if *filter.GroupNum == 0 {
			noContactIDs = make([]int, 0, len(roomCounts))
			for contactID := range roomCounts {
				noContactIDs = append(noContactIDs, contactID)
			}
			sort.Ints(noContactIDs)
		} else {
			ids := make([]int, 0)
			for contactID, total := range roomCounts {
				if (*filter.GroupNum == 1 && total == 1) || (*filter.GroupNum == 2 && total >= 2) {
					ids = append(ids, contactID)
				}
			}
			if len(ids) == 0 {
				return nil, nil, true, nil
			}
			intersect(ids)
		}
	}
	if hasContactFilter && len(contactIDs) == 0 {
		return nil, nil, true, nil
	}
	return contactIDs, noContactIDs, false, nil
}

func (s *MySQLStore) workContactIDsByStringColumn(ctx context.Context, corpID int, column string, operator string, value string) ([]int, error) {
	query := "SELECT id FROM mc_work_contact WHERE corp_id = ? AND " + column + " " + operator + " ? AND deleted_at IS NULL"
	rows, err := s.db.QueryContext(ctx, query, corpID, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) workContactIDsByGender(ctx context.Context, corpID int, gender int) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM mc_work_contact
		WHERE corp_id = ? AND gender = ? AND deleted_at IS NULL
	`, corpID, gender)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) contactIDsByField(ctx context.Context, fieldID int, value string) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT contact_id
		FROM mc_contact_field_pivot
		WHERE contact_field_id = ? AND value LIKE ? AND deleted_at IS NULL
	`, fieldID, "%"+value+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) contactIDsByRooms(ctx context.Context, roomIDs []int) ([]int, error) {
	roomIDs = uniquePositiveInts(roomIDs)
	if len(roomIDs) == 0 {
		return []int{}, nil
	}
	args := make([]any, 0, len(roomIDs))
	for _, roomID := range roomIDs {
		args = append(args, roomID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT contact_id
		FROM mc_work_contact_room
		WHERE room_id IN (`+placeholders(len(roomIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) contactRoomCounts(ctx context.Context) (map[int]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT contact_id, COUNT(room_id)
		FROM mc_work_contact_room
		WHERE deleted_at IS NULL
		GROUP BY contact_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make(map[int]int)
	for rows.Next() {
		var contactID, total int
		if err := rows.Scan(&contactID, &total); err != nil {
			return nil, err
		}
		counts[contactID] = total
	}
	return counts, rows.Err()
}

func workContactIndexWhere(filter dashboard.WorkContactIndexFilter, contactIDs []int, noContactIDs []int) ([]string, []any) {
	where := []string{"corp_id = ?", "status = ?", "deleted_at IS NULL"}
	args := []any{filter.CorpID, 1}
	if filter.Remark != "" {
		where = append(where, "remark LIKE ?")
		args = append(args, "%"+filter.Remark+"%")
	}
	if filter.AddWay != nil {
		where = append(where, "add_way = ?")
		args = append(args, *filter.AddWay)
	}
	if len(contactIDs) > 0 {
		where = append(where, "contact_id IN ("+placeholders(len(contactIDs))+")")
		for _, contactID := range contactIDs {
			args = append(args, contactID)
		}
	}
	if len(noContactIDs) > 0 {
		where = append(where, "contact_id NOT IN ("+placeholders(len(noContactIDs))+")")
		for _, contactID := range noContactIDs {
			args = append(args, contactID)
		}
	}
	if filter.RestrictEmployees {
		where = append(where, "employee_id IN ("+placeholders(len(filter.EmployeeIDs))+")")
		for _, employeeID := range filter.EmployeeIDs {
			args = append(args, employeeID)
		}
	}
	if filter.StartTime != "" {
		where = append(where, "create_time >= ?")
		args = append(args, filter.StartTime)
	}
	if filter.EndTime != "" {
		where = append(where, "create_time <= ?")
		args = append(args, filter.EndTime)
	}
	return where, args
}

func workContactLossWhere(filter dashboard.WorkContactLossFilter) ([]string, []any) {
	where := []string{"corp_id = ?", "status IN (2, 3)"}
	args := []any{filter.CorpID}
	if filter.RestrictEmployees {
		where = append(where, "employee_id IN ("+placeholders(len(filter.EmployeeIDs))+")")
		for _, employeeID := range filter.EmployeeIDs {
			args = append(args, employeeID)
		}
	}
	return where, args
}

func (s *MySQLStore) workContactIndexContacts(ctx context.Context, contactIDs []int) (map[int]workContactIndexContact, error) {
	contactIDs = uniquePositiveInts(contactIDs)
	if len(contactIDs) == 0 {
		return map[int]workContactIndexContact{}, nil
	}
	args := make([]any, 0, len(contactIDs))
	for _, contactID := range contactIDs {
		args = append(args, contactID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, business_no, name, avatar, gender
		FROM mc_work_contact
		WHERE id IN (`+placeholders(len(contactIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	contacts := make(map[int]workContactIndexContact, len(contactIDs))
	for rows.Next() {
		var id int
		var contact workContactIndexContact
		var businessNo, name, avatar sql.NullString
		if err := rows.Scan(&id, &businessNo, &name, &avatar, &contact.Gender); err != nil {
			return nil, err
		}
		contact.BusinessNo = nullString(businessNo)
		contact.Name = nullString(name)
		contact.Avatar = nullString(avatar)
		contacts[id] = contact
	}
	return contacts, rows.Err()
}

func (s *MySQLStore) workContactLossContacts(ctx context.Context, contactIDs []int) (map[int]workContactIndexContact, error) {
	contactIDs = uniquePositiveInts(contactIDs)
	if len(contactIDs) == 0 {
		return map[int]workContactIndexContact{}, nil
	}
	args := make([]any, 0, len(contactIDs))
	for _, contactID := range contactIDs {
		args = append(args, contactID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, business_no, name, avatar, gender
		FROM mc_work_contact
		WHERE id IN (`+placeholders(len(contactIDs))+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	contacts := make(map[int]workContactIndexContact, len(contactIDs))
	for rows.Next() {
		var id int
		var contact workContactIndexContact
		var businessNo, name, avatar sql.NullString
		if err := rows.Scan(&id, &businessNo, &name, &avatar, &contact.Gender); err != nil {
			return nil, err
		}
		contact.BusinessNo = nullString(businessNo)
		contact.Name = nullString(name)
		contact.Avatar = nullString(avatar)
		contacts[id] = contact
	}
	return contacts, rows.Err()
}

func (s *MySQLStore) workContactIndexRoomNames(ctx context.Context, contactIDs []int) (map[int][]string, error) {
	contactIDs = uniquePositiveInts(contactIDs)
	if len(contactIDs) == 0 {
		return map[int][]string{}, nil
	}
	args := make([]any, 0, len(contactIDs))
	for _, contactID := range contactIDs {
		args = append(args, contactID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT contact_room.contact_id, room.name
		FROM mc_work_contact_room AS contact_room
		LEFT JOIN mc_work_room AS room ON room.id = contact_room.room_id AND room.deleted_at IS NULL
		WHERE contact_room.contact_id IN (`+placeholders(len(contactIDs))+`) AND contact_room.deleted_at IS NULL
		ORDER BY contact_room.id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := make(map[int][]string, len(contactIDs))
	for rows.Next() {
		var contactID int
		var roomName sql.NullString
		if err := rows.Scan(&contactID, &roomName); err != nil {
			return nil, err
		}
		names[contactID] = append(names[contactID], nullString(roomName))
	}
	return names, rows.Err()
}

func (s *MySQLStore) workContactIndexEmployeeNames(ctx context.Context, employeeIDs []int) (map[int]string, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return map[int]string{}, nil
	}
	args := make([]any, 0, len(employeeIDs))
	for _, employeeID := range employeeIDs {
		args = append(args, employeeID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name
		FROM mc_work_employee
		WHERE id IN (`+placeholders(len(employeeIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := make(map[int]string, len(employeeIDs))
	for rows.Next() {
		var id int
		var name sql.NullString
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		names[id] = nullString(name)
	}
	return names, rows.Err()
}

func (s *MySQLStore) workContactLossEmployeeNames(ctx context.Context, employeeIDs []int) (map[int]string, map[int]string, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return map[int]string{}, map[int]string{}, nil
	}
	args := make([]any, 0, len(employeeIDs))
	for _, employeeID := range employeeIDs {
		args = append(args, employeeID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT employee.id, employee.name, corp.name
		FROM mc_work_employee AS employee
		LEFT JOIN mc_corp AS corp ON corp.id = employee.corp_id AND corp.deleted_at IS NULL
		WHERE employee.id IN (`+placeholders(len(employeeIDs))+`) AND employee.deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	names := make(map[int]string, len(employeeIDs))
	remarks := make(map[int]string, len(employeeIDs))
	for rows.Next() {
		var id int
		var employeeName, corpName sql.NullString
		if err := rows.Scan(&id, &employeeName, &corpName); err != nil {
			return nil, nil, err
		}
		name := nullString(employeeName)
		corp := nullString(corpName)
		names[id] = strings.TrimSpace(corp + " " + name)
		remarks[id] = name
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return names, remarks, nil
}

func (s *MySQLStore) workContactIndexTags(ctx context.Context, items []dashboard.WorkContactIndexItem) (map[string][]string, error) {
	if len(items) == 0 {
		return map[string][]string{}, nil
	}
	pairs := make([]string, 0, len(items))
	args := make([]any, 0, len(items)*2)
	seen := map[string]struct{}{}
	for _, item := range items {
		key := workContactIndexPairKey(item.ContactID, item.EmployeeID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		pairs = append(pairs, "(?, ?)")
		args = append(args, item.ContactID, item.EmployeeID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT pivot.contact_id, pivot.employee_id, tag.name
		FROM mc_work_contact_tag_pivot AS pivot
		JOIN mc_work_contact_tag AS tag ON tag.id = pivot.contact_tag_id AND tag.deleted_at IS NULL
		WHERE (pivot.contact_id, pivot.employee_id) IN (`+strings.Join(pairs, ",")+`) AND pivot.deleted_at IS NULL
		ORDER BY pivot.id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := make(map[string][]string, len(items))
	tagSeen := make(map[string]map[string]struct{})
	for rows.Next() {
		var contactID, employeeID int
		var tagName sql.NullString
		if err := rows.Scan(&contactID, &employeeID, &tagName); err != nil {
			return nil, err
		}
		key := workContactIndexPairKey(contactID, employeeID)
		name := nullString(tagName)
		if tagSeen[key] == nil {
			tagSeen[key] = map[string]struct{}{}
		}
		if _, ok := tagSeen[key][name]; ok {
			continue
		}
		tagSeen[key][name] = struct{}{}
		tags[key] = append(tags[key], name)
	}
	return tags, rows.Err()
}

func (s *MySQLStore) workContactLossTags(ctx context.Context, items []dashboard.WorkContactLossItem) (map[string][]string, error) {
	if len(items) == 0 {
		return map[string][]string{}, nil
	}
	pairs := make([]string, 0, len(items))
	args := make([]any, 0, len(items)*2)
	seen := map[string]struct{}{}
	for _, item := range items {
		key := workContactIndexPairKey(item.ContactID, item.EmployeeID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		pairs = append(pairs, "(?, ?)")
		args = append(args, item.ContactID, item.EmployeeID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT pivot.contact_id, pivot.employee_id, tag.name
		FROM mc_work_contact_tag_pivot AS pivot
		JOIN mc_work_contact_tag AS tag ON tag.id = pivot.contact_tag_id AND tag.deleted_at IS NULL
		WHERE (pivot.contact_id, pivot.employee_id) IN (`+strings.Join(pairs, ",")+`)
		ORDER BY pivot.id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := make(map[string][]string, len(items))
	tagSeen := make(map[string]map[string]struct{})
	for rows.Next() {
		var contactID, employeeID int
		var tagName sql.NullString
		if err := rows.Scan(&contactID, &employeeID, &tagName); err != nil {
			return nil, err
		}
		key := workContactIndexPairKey(contactID, employeeID)
		name := nullString(tagName)
		if tagSeen[key] == nil {
			tagSeen[key] = map[string]struct{}{}
		}
		if _, ok := tagSeen[key][name]; ok {
			continue
		}
		tagSeen[key][name] = struct{}{}
		tags[key] = append(tags[key], name)
	}
	return tags, rows.Err()
}

func (s *MySQLStore) workUpdateTime(ctx context.Context, corpID int, updateType int) (string, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT last_update_time
		FROM mc_work_update_time
		WHERE corp_id = ? AND type = ?
		ORDER BY id ASC
		LIMIT 1
	`, corpID, updateType)
	var value sql.NullTime
	err := row.Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return formatTime(value), nil
}

func intersectPositiveInts(left []int, right []int) []int {
	rightSet := make(map[int]struct{}, len(right))
	for _, value := range right {
		if value > 0 {
			rightSet[value] = struct{}{}
		}
	}
	result := make([]int, 0, len(left))
	seen := map[int]struct{}{}
	for _, value := range left {
		if value <= 0 {
			continue
		}
		if _, ok := rightSet[value]; !ok {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func workContactIndexPairKey(contactID int, employeeID int) string {
	return strconv.Itoa(contactID) + ":" + strconv.Itoa(employeeID)
}

type roomMemberBase struct {
	ID     int
	Name   string
	Avatar string
}

func (s *MySQLStore) workContactRoomNameIDs(ctx context.Context, corpID int, name string) ([]int, []int, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil, false, nil
	}
	employeeIDs, err := s.idsByName(ctx, `
		SELECT id
		FROM mc_work_employee
		WHERE corp_id = ? AND name LIKE ? AND deleted_at IS NULL
	`, corpID, name)
	if err != nil {
		return nil, nil, true, err
	}
	contactIDs, err := s.idsByName(ctx, `
		SELECT id
		FROM mc_work_contact
		WHERE corp_id = ? AND name LIKE ? AND deleted_at IS NULL
	`, corpID, name)
	if err != nil {
		return nil, nil, true, err
	}
	return employeeIDs, contactIDs, true, nil
}

func (s *MySQLStore) idsByName(ctx context.Context, query string, corpID int, name string) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, query, corpID, "%"+name+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int, 0)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func workContactRoomWhere(filter dashboard.WorkContactRoomFilter, employeeIDs []int, contactIDs []int, nameFiltered bool) (string, []any) {
	parts := []string{"room_id = ?", "deleted_at IS NULL"}
	args := []any{filter.WorkRoomID}
	if filter.Status != nil {
		parts = append(parts, "status = ?")
		args = append(args, *filter.Status)
	}
	if filter.StartTime != "" {
		parts = append(parts, "join_time >= ?")
		args = append(args, filter.StartTime+" 00:00:00")
	}
	if filter.EndTime != "" {
		parts = append(parts, "join_time <= ?")
		args = append(args, filter.EndTime+" 00:00:00")
	}
	if nameFiltered {
		nameParts := make([]string, 0, 2)
		if len(contactIDs) > 0 {
			nameParts = append(nameParts, "contact_id IN ("+placeholders(len(contactIDs))+")")
			for _, id := range contactIDs {
				args = append(args, id)
			}
		}
		if len(employeeIDs) > 0 {
			nameParts = append(nameParts, "employee_id IN ("+placeholders(len(employeeIDs))+")")
			for _, id := range employeeIDs {
				args = append(args, id)
			}
		}
		if len(nameParts) > 0 {
			parts = append(parts, "("+strings.Join(nameParts, " OR ")+")")
		}
	}
	return strings.Join(parts, " AND "), args
}

func (s *MySQLStore) decorateWorkContactRoomItems(ctx context.Context, raws []workContactRoomRaw, ownerID int, corpID int) ([]dashboard.WorkContactRoomItem, error) {
	employeeIDs := make([]int, 0)
	contactIDs := make([]int, 0)
	for _, raw := range raws {
		if raw.Type == 1 {
			employeeIDs = append(employeeIDs, raw.EmployeeID)
		}
		if raw.Type == 2 {
			contactIDs = append(contactIDs, raw.ContactID)
		}
	}
	employees, err := s.roomMemberEmployees(ctx, employeeIDs)
	if err != nil {
		return nil, err
	}
	contacts, err := s.roomMemberContacts(ctx, contactIDs)
	if err != nil {
		return nil, err
	}
	items := make([]dashboard.WorkContactRoomItem, 0, len(raws))
	for _, raw := range raws {
		base := roomMemberBase{}
		contactID := 0
		employeeID := 0
		contactEmployeeID := 0
		if raw.Type == 1 {
			base = employees[raw.EmployeeID]
			employeeID = base.ID
		}
		if raw.Type == 2 {
			base = contacts[raw.ContactID]
			contactID = base.ID
			if contactID != 0 {
				id, err := s.contactEmployeeID(ctx, corpID, contactID)
				if err != nil {
					return nil, err
				}
				contactEmployeeID = id
			}
		}
		otherRooms, err := s.otherRoomNamesByWXUserID(ctx, raw.WXUserID, raw.RoomID)
		if err != nil {
			return nil, err
		}
		isOwner := 0
		if raw.EmployeeID == ownerID {
			isOwner = 1
		}
		items = append(items, dashboard.WorkContactRoomItem{
			WorkContactRoomID: raw.ID,
			Name:              base.Name,
			Avatar:            base.Avatar,
			IsOwner:           isOwner,
			JoinTime:          raw.JoinTime,
			OutRoomTime:       raw.OutTime,
			OtherRooms:        otherRooms,
			JoinScene:         raw.JoinScene,
			Type:              raw.Type,
			ContactID:         contactID,
			EmployeeID:        employeeID,
			ContactEmployeeID: contactEmployeeID,
		})
	}
	return items, nil
}

func (s *MySQLStore) roomMemberEmployees(ctx context.Context, ids []int) (map[int]roomMemberBase, error) {
	return s.roomMemberBaseMap(ctx, "mc_work_employee", ids)
}

func (s *MySQLStore) roomMemberContacts(ctx context.Context, ids []int) (map[int]roomMemberBase, error) {
	return s.roomMemberBaseMap(ctx, "mc_work_contact", ids)
}

func (s *MySQLStore) roomMemberBaseMap(ctx context.Context, table string, ids []int) (map[int]roomMemberBase, error) {
	result := map[int]roomMemberBase{}
	ids = uniquePositiveInts(ids)
	if len(ids) == 0 {
		return result, nil
	}
	query := "SELECT id, name, avatar FROM " + table + " WHERE id IN (" + placeholders(len(ids)) + ") AND deleted_at IS NULL"
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var base roomMemberBase
		var name, avatar sql.NullString
		if err := rows.Scan(&base.ID, &name, &avatar); err != nil {
			return nil, err
		}
		base.Name = nullString(name)
		base.Avatar = nullString(avatar)
		result[base.ID] = base
	}
	return result, rows.Err()
}

func (s *MySQLStore) contactEmployeeID(ctx context.Context, corpID int, contactID int) (int, error) {
	var employeeID int
	err := s.db.QueryRowContext(ctx, `
		SELECT employee_id
		FROM mc_work_contact_employee
		WHERE contact_id = ? AND corp_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, contactID, corpID).Scan(&employeeID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return employeeID, err
}

func (s *MySQLStore) otherRoomNamesByWXUserID(ctx context.Context, wxUserID string, currentRoomID int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT room.name
		FROM mc_work_contact_room AS contact_room
		JOIN mc_work_room AS room ON room.id = contact_room.room_id AND room.deleted_at IS NULL
		WHERE contact_room.wx_user_id = ? AND contact_room.room_id <> ? AND contact_room.deleted_at IS NULL
		ORDER BY contact_room.id ASC
	`, wxUserID, currentRoomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := make([]string, 0)
	for rows.Next() {
		var name sql.NullString
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, nullString(name))
	}
	return names, rows.Err()
}

func uniquePositiveInts(values []int) []int {
	seen := map[int]struct{}{}
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (s *MySQLStore) ContactEmployeeTracksByContactID(ctx context.Context, contactID int) ([]dashboard.ContactEmployeeTrack, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, content, created_at
		FROM mc_contact_employee_track
		WHERE contact_id = ? AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tracks := make([]dashboard.ContactEmployeeTrack, 0)
	for rows.Next() {
		var track dashboard.ContactEmployeeTrack
		var createdAt sql.NullTime
		if err := rows.Scan(&track.ID, &track.Content, &createdAt); err != nil {
			return nil, err
		}
		track.CreatedAt = formatTime(createdAt)
		tracks = append(tracks, track)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tracks, nil
}

func (s *MySQLStore) SidebarContactEmployeeTracksByContactID(ctx context.Context, contactID int, employeeID int, corpID int) ([]dashboard.ContactEmployeeTrack, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT track.id, track.content, track.created_at
		FROM mc_contact_employee_track AS track
		JOIN mc_work_contact AS contact
		  ON contact.id = track.contact_id AND contact.corp_id = ? AND contact.deleted_at IS NULL
		JOIN mc_work_contact_employee AS pivot
		  ON pivot.contact_id = contact.id AND pivot.employee_id = ? AND pivot.deleted_at IS NULL
		JOIN mc_work_employee AS employee
		  ON employee.id = pivot.employee_id AND employee.corp_id = contact.corp_id AND employee.deleted_at IS NULL
		WHERE track.contact_id = ? AND track.deleted_at IS NULL
		ORDER BY track.created_at DESC
	`, corpID, employeeID, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tracks := make([]dashboard.ContactEmployeeTrack, 0)
	for rows.Next() {
		var track dashboard.ContactEmployeeTrack
		var createdAt sql.NullTime
		if err := rows.Scan(&track.ID, &track.Content, &createdAt); err != nil {
			return nil, err
		}
		track.CreatedAt = formatTime(createdAt)
		tracks = append(tracks, track)
	}
	return tracks, rows.Err()
}

func (s *MySQLStore) ContactProcessesByCorpID(ctx context.Context, corpID int) ([]dashboard.ContactProcessStatus, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name
		FROM mc_contact_process
		WHERE corp_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statuses := make([]dashboard.ContactProcessStatus, 0)
	for rows.Next() {
		var status dashboard.ContactProcessStatus
		if err := rows.Scan(&status.ID, &status.Name); err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return statuses, nil
}

func (s *MySQLStore) CreateDefaultContactProcesses(ctx context.Context, corpID int) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_contact_process (corp_id, name, `+"`order`"+`, created_at)
		VALUES (?, '新客户', 1, NOW()),
		       (?, '初步沟通', 2, NOW()),
		       (?, '意向客户', 3, NOW()),
		       (?, '付款客户', 4, NOW()),
		       (?, '无意向客户', 5, NOW())
	`, corpID, corpID, corpID, corpID, corpID)
	return err
}

func (s *MySQLStore) ContactProcessByID(ctx context.Context, statusID int) (dashboard.ContactProcessStatus, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name
		FROM mc_contact_process
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, statusID)

	var status dashboard.ContactProcessStatus
	if err := row.Scan(&status.ID, &status.Name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dashboard.ContactProcessStatus{}, false, nil
		}
		return dashboard.ContactProcessStatus{}, false, err
	}
	return status, true, nil
}

func (s *MySQLStore) UpdateContactProcessStatus(ctx context.Context, update dashboard.ContactProcessStatusUpdate) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mc_contact_employee_track (employee_id, contact_id, content, corp_id, event, created_at)
		VALUES (?, ?, ?, ?, ?, NOW())
	`, update.EmployeeID, update.ContactID, update.Content, update.CorpID, update.Event); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_contact
		SET follow_up_status = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, update.StatusID, update.ContactID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) UpdateWorkContactProfile(ctx context.Context, values dashboard.WorkContactUpdateValues) (dashboard.WorkContactUpdateResult, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.WorkContactUpdateResult{}, false, err
	}
	defer tx.Rollback()

	var result dashboard.WorkContactUpdateResult
	var currentRemark, currentDescription, currentBusinessNo string
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(employee.wx_user_id, ''), COALESCE(contact.wx_external_userid, ''),
		       COALESCE(ce.remark, ''), COALESCE(ce.description, ''), COALESCE(contact.business_no, '')
		FROM mc_work_contact_employee AS ce
		JOIN mc_work_employee AS employee ON employee.id = ce.employee_id AND employee.corp_id = ce.corp_id AND employee.deleted_at IS NULL
		JOIN mc_work_contact AS contact ON contact.id = ce.contact_id AND contact.corp_id = ce.corp_id AND contact.deleted_at IS NULL
		WHERE ce.employee_id = ? AND ce.contact_id = ? AND ce.corp_id = ? AND ce.deleted_at IS NULL
		LIMIT 1 FOR UPDATE
	`, values.EmployeeID, values.ContactID, values.CorpID).Scan(
		&result.WXUserID,
		&result.WXExternalUserID,
		&currentRemark,
		&currentDescription,
		&currentBusinessNo,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WorkContactUpdateResult{}, false, nil
	}
	if err != nil {
		return dashboard.WorkContactUpdateResult{}, false, err
	}

	remarkChanged := values.Remark != nil && *values.Remark != currentRemark
	descriptionChanged := values.Description != nil && *values.Description != currentDescription
	if remarkChanged || descriptionChanged {
		sets := []string{"updated_at = NOW()"}
		args := []any{}
		if remarkChanged {
			sets = append(sets, "remark = ?")
			args = append(args, *values.Remark)
		}
		if descriptionChanged {
			sets = append(sets, "description = ?")
			args = append(args, *values.Description)
		}
		args = append(args, values.EmployeeID, values.ContactID, values.CorpID)
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_work_contact_employee
			SET `+strings.Join(sets, ", ")+`
			WHERE employee_id = ? AND contact_id = ? AND corp_id = ? AND deleted_at IS NULL
		`, args...); err != nil {
			return dashboard.WorkContactUpdateResult{}, false, err
		}
		if remarkChanged {
			if err := insertContactTrackTx(ctx, tx, values.EmployeeID, values.ContactID, "修改用户资料：备注", values.CorpID, 3); err != nil {
				return dashboard.WorkContactUpdateResult{}, false, err
			}
		}
		if descriptionChanged {
			if err := insertContactTrackTx(ctx, tx, values.EmployeeID, values.ContactID, "修改用户资料：描述", values.CorpID, 3); err != nil {
				return dashboard.WorkContactUpdateResult{}, false, err
			}
		}
	}

	if values.BusinessNo != nil && *values.BusinessNo != currentBusinessNo {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_work_contact
			SET business_no = ?, updated_at = NOW()
			WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
		`, *values.BusinessNo, values.ContactID, values.CorpID); err != nil {
			return dashboard.WorkContactUpdateResult{}, false, err
		}
		if err := insertContactTrackTx(ctx, tx, values.EmployeeID, values.ContactID, "修改用户资料：客户编号", values.CorpID, 3); err != nil {
			return dashboard.WorkContactUpdateResult{}, false, err
		}
	}

	if values.HasTag {
		appliedTags, err := addWorkContactTagsTx(ctx, tx, values.CorpID, values.ContactID, values.EmployeeID, values.TagIDs)
		if err != nil {
			return dashboard.WorkContactUpdateResult{}, false, err
		}
		result.TagSyncRequested = len(uniquePositiveInts(values.TagIDs)) > 0
		result.AddedWXTagIDs = appliedTags.wxIDs
		result.AddedTagNames = appliedTags.names
		result.UnsyncableTagIDs = appliedTags.unsyncableIDs
		if len(appliedTags.names) > 0 {
			if err := insertContactTrackTx(ctx, tx, values.EmployeeID, values.ContactID, workContactUpdateTagContent(appliedTags.names), values.CorpID, 2); err != nil {
				return dashboard.WorkContactUpdateResult{}, false, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return dashboard.WorkContactUpdateResult{}, false, err
	}
	return result, true, nil
}

func (s *MySQLStore) ApplyWorkContactTags(ctx context.Context, values dashboard.MarkTagsApplyValues) (dashboard.MarkTagsApplyResult, bool, error) {
	tagIDs := uniquePositiveInts(values.TagIDs)
	if values.CorpID <= 0 || values.ContactID <= 0 || values.EmployeeID <= 0 || len(tagIDs) == 0 {
		return dashboard.MarkTagsApplyResult{}, false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.MarkTagsApplyResult{}, false, err
	}
	defer tx.Rollback()

	var result dashboard.MarkTagsApplyResult
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(wx_external_userid, '')
		FROM mc_work_contact
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, values.ContactID, values.CorpID).Scan(&result.WXExternalUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.MarkTagsApplyResult{}, false, nil
	}
	if err != nil {
		return dashboard.MarkTagsApplyResult{}, false, err
	}
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(wx_user_id, '')
		FROM mc_work_employee
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, values.EmployeeID, values.CorpID).Scan(&result.WXUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.MarkTagsApplyResult{}, false, nil
	}
	if err != nil {
		return dashboard.MarkTagsApplyResult{}, false, err
	}

	appliedTags, err := addWorkContactTagsTx(ctx, tx, values.CorpID, values.ContactID, values.EmployeeID, tagIDs)
	if err != nil {
		return dashboard.MarkTagsApplyResult{}, false, err
	}
	result.AddedWXTagIDs = appliedTags.wxIDs
	result.AddedTagNames = appliedTags.names
	result.TagSyncRequested = len(tagIDs) > 0
	result.UnsyncableTagIDs = appliedTags.unsyncableIDs
	if len(appliedTags.names) > 0 {
		if err := insertContactTrackTx(ctx, tx, values.EmployeeID, values.ContactID, workContactUpdateTagContent(appliedTags.names), values.CorpID, 2); err != nil {
			return dashboard.MarkTagsApplyResult{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return dashboard.MarkTagsApplyResult{}, false, err
	}
	return result, true, nil
}

var errWorkContactTagScope = errors.New("work contact tag is outside corp scope")

type appliedWorkContactTags struct {
	wxIDs         []string
	names         []string
	unsyncableIDs []int
}

func addWorkContactTagsTx(ctx context.Context, tx *sql.Tx, corpID int, contactID int, employeeID int, tagIDs []int) (appliedWorkContactTags, error) {
	tagIDs = uniquePositiveInts(tagIDs)
	if len(tagIDs) == 0 {
		return appliedWorkContactTags{wxIDs: []string{}, names: []string{}, unsyncableIDs: []int{}}, nil
	}
	type tagDetails struct {
		wxID string
		name string
	}
	tagArgs := append([]any{corpID}, intsToAny(tagIDs)...)
	rows, err := tx.QueryContext(ctx, `
		SELECT id, COALESCE(wx_contact_tag_id, ''), COALESCE(name, '')
		FROM mc_work_contact_tag
		WHERE corp_id = ? AND id IN (`+placeholders(len(tagIDs))+`) AND deleted_at IS NULL
	`, tagArgs...)
	if err != nil {
		return appliedWorkContactTags{}, err
	}
	allowed := make(map[int]tagDetails, len(tagIDs))
	for rows.Next() {
		var id int
		var details tagDetails
		if err := rows.Scan(&id, &details.wxID, &details.name); err != nil {
			rows.Close()
			return appliedWorkContactTags{}, err
		}
		allowed[id] = details
	}
	if err := rows.Close(); err != nil {
		return appliedWorkContactTags{}, err
	}
	if len(allowed) != len(tagIDs) {
		return appliedWorkContactTags{}, errWorkContactTagScope
	}
	syncWXTagIDs := make([]string, 0, len(tagIDs))
	unsyncableTagIDs := make([]int, 0)
	for _, tagID := range tagIDs {
		if wxID := allowed[tagID].wxID; wxID != "" {
			syncWXTagIDs = append(syncWXTagIDs, wxID)
		} else {
			unsyncableTagIDs = append(unsyncableTagIDs, tagID)
		}
	}

	rows, err = tx.QueryContext(ctx, `
		SELECT pivot.contact_tag_id
		FROM mc_work_contact_tag_pivot AS pivot
		JOIN mc_work_contact_tag AS tag
		  ON tag.id = pivot.contact_tag_id AND tag.corp_id = ? AND tag.deleted_at IS NULL
		WHERE pivot.contact_id = ? AND pivot.employee_id = ? AND pivot.deleted_at IS NULL
	`, corpID, contactID, employeeID)
	if err != nil {
		return appliedWorkContactTags{}, err
	}
	existing := map[int]struct{}{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return appliedWorkContactTags{}, err
		}
		existing[id] = struct{}{}
	}
	if err := rows.Close(); err != nil {
		return appliedWorkContactTags{}, err
	}

	addIDs := make([]int, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		if _, ok := allowed[tagID]; !ok {
			continue
		}
		if _, ok := existing[tagID]; !ok {
			addIDs = append(addIDs, tagID)
		}
	}
	if len(addIDs) == 0 {
		return appliedWorkContactTags{wxIDs: syncWXTagIDs, names: []string{}, unsyncableIDs: unsyncableTagIDs}, nil
	}
	for _, tagID := range addIDs {
		if _, err := tx.ExecContext(ctx, `
				INSERT INTO mc_work_contact_tag_pivot
					(contact_id, employee_id, contact_tag_id, type, created_at, updated_at)
				VALUES (?, ?, ?, 1, NOW(), NOW())
			`, contactID, employeeID, tagID); err != nil {
			return appliedWorkContactTags{}, err
		}
	}

	names := []string{}
	for _, tagID := range addIDs {
		details := allowed[tagID]
		if details.name != "" {
			names = append(names, details.name)
		}
	}
	return appliedWorkContactTags{wxIDs: syncWXTagIDs, names: names, unsyncableIDs: unsyncableTagIDs}, nil
}

func (s *MySQLStore) BatchLabelWorkContacts(ctx context.Context, contactIDs []int, tagIDs []int, employeeID int, corpID int) (int, error) {
	contactIDs = uniquePositiveInts(contactIDs)
	tagIDs = uniquePositiveInts(tagIDs)
	if len(contactIDs) == 0 || len(tagIDs) == 0 || employeeID <= 0 || corpID <= 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var employeeCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_work_employee
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, employeeID, corpID).Scan(&employeeCount); err != nil {
		return 0, err
	}
	if employeeCount != 1 {
		return 0, dashboard.ErrWorkContactBatchLabelScope
	}

	contactArgs := append([]any{employeeID, corpID}, intsToAny(contactIDs)...)
	var contactCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT contact.id)
		FROM mc_work_contact AS contact
		JOIN mc_work_contact_employee AS relation
		  ON relation.contact_id = contact.id
		 AND relation.employee_id = ?
		 AND relation.corp_id = contact.corp_id
		 AND relation.deleted_at IS NULL
		WHERE contact.corp_id = ?
		  AND contact.id IN (`+placeholders(len(contactIDs))+`)
		  AND contact.deleted_at IS NULL
	`, contactArgs...).Scan(&contactCount); err != nil {
		return 0, err
	}
	if contactCount != len(contactIDs) {
		return 0, dashboard.ErrWorkContactBatchLabelScope
	}

	tagArgs := append([]any{corpID}, intsToAny(tagIDs)...)
	var tagCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_work_contact_tag
		WHERE corp_id = ?
		  AND id IN (`+placeholders(len(tagIDs))+`)
		  AND deleted_at IS NULL
	`, tagArgs...).Scan(&tagCount); err != nil {
		return 0, err
	}
	if tagCount != len(tagIDs) {
		return 0, dashboard.ErrWorkContactBatchLabelScope
	}

	args := make([]any, 0, len(contactIDs)+len(tagIDs))
	args = append(args, intsToAny(contactIDs)...)
	args = append(args, intsToAny(tagIDs)...)
	args = append(args, employeeID)
	rows, err := tx.QueryContext(ctx, `
		SELECT contact_id, contact_tag_id
		FROM mc_work_contact_tag_pivot
		WHERE contact_id IN (`+placeholders(len(contactIDs))+`)
		  AND contact_tag_id IN (`+placeholders(len(tagIDs))+`)
		  AND employee_id = ?
		  AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return 0, err
	}
	existing := map[[2]int]struct{}{}
	for rows.Next() {
		var contactID, tagID int
		if err := rows.Scan(&contactID, &tagID); err != nil {
			rows.Close()
			return 0, err
		}
		existing[[2]int{contactID, tagID}] = struct{}{}
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	inserted := 0
	for _, contactID := range contactIDs {
		for _, tagID := range tagIDs {
			if _, ok := existing[[2]int{contactID, tagID}]; ok {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO mc_work_contact_tag_pivot
					(contact_id, employee_id, contact_tag_id, created_at, updated_at)
				VALUES (?, ?, ?, NOW(), NOW())
			`, contactID, employeeID, tagID); err != nil {
				return 0, err
			}
			inserted++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

func insertContactTrackTx(ctx context.Context, tx *sql.Tx, employeeID int, contactID int, content string, corpID int, event int) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mc_contact_employee_track
			(employee_id, contact_id, content, corp_id, event, created_at)
		VALUES (?, ?, ?, ?, ?, NOW())
	`, employeeID, contactID, content, corpID, event)
	return err
}

func workContactUpdateTagContent(names []string) string {
	var builder strings.Builder
	builder.WriteString("系统对该客户打标签")
	for i, name := range names {
		builder.WriteString("【")
		builder.WriteString(name)
		builder.WriteString("】")
		if i != len(names)-1 {
			builder.WriteString("、")
		}
	}
	return builder.String()
}

func intsToAny(values []int) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func (s *MySQLStore) ContactFieldPage(ctx context.Context, filter dashboard.ContactFieldFilter) (dashboard.ContactFieldPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10
	}

	where := "WHERE deleted_at IS NULL"
	args := []any{}
	if filter.Status != 2 {
		where += " AND status = ?"
		args = append(args, filter.Status)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_contact_field `+where, args...).Scan(&total); err != nil {
		return dashboard.ContactFieldPage{}, err
	}
	if total == 0 {
		return dashboard.ContactFieldPage{Items: []dashboard.ContactField{}, PerPage: filter.PerPage}, nil
	}

	totalPage := (total + filter.PerPage - 1) / filter.PerPage
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, label, type, options, status, `+"`order`"+`, is_sys
		FROM mc_contact_field
		`+where+`
		ORDER BY `+"`order`"+` DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.ContactFieldPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.ContactField, 0)
	for rows.Next() {
		var item dashboard.ContactField
		var options sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &item.Label, &item.Type, &options, &item.Status, &item.Order, &item.IsSys); err != nil {
			return dashboard.ContactFieldPage{}, err
		}
		item.Options = dashboard.ParseContactFieldOptions(nullString(options))
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ContactFieldPage{}, err
	}
	return dashboard.ContactFieldPage{Items: items, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) ContactFieldByID(ctx context.Context, fieldID int) (dashboard.ContactField, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, label, type, options, status, `+"`order`"+`, is_sys
		FROM mc_contact_field
		WHERE id = ? AND deleted_at IS NULL
	`, fieldID)
	var field dashboard.ContactField
	var options sql.NullString
	if err := row.Scan(&field.ID, &field.Name, &field.Label, &field.Type, &options, &field.Status, &field.Order, &field.IsSys); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dashboard.ContactField{}, false, nil
		}
		return dashboard.ContactField{}, false, err
	}
	field.Options = dashboard.ParseContactFieldOptions(nullString(options))
	return field, true, nil
}

func (s *MySQLStore) ContactFieldsByStatusOrder(ctx context.Context, status int) ([]dashboard.ContactField, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, label, type, options, status, `+"`order`"+`, is_sys
		FROM mc_contact_field
		WHERE status = ? AND deleted_at IS NULL
		ORDER BY `+"`order`"+` DESC
	`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := make([]dashboard.ContactField, 0)
	for rows.Next() {
		var field dashboard.ContactField
		var options sql.NullString
		if err := rows.Scan(&field.ID, &field.Name, &field.Label, &field.Type, &options, &field.Status, &field.Order, &field.IsSys); err != nil {
			return nil, err
		}
		field.Options = dashboard.ParseContactFieldOptions(nullString(options))
		fields = append(fields, field)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return fields, nil
}

func (s *MySQLStore) ContactFieldPivotsByContactID(ctx context.Context, contactID int, fieldIDs []int) (map[int]dashboard.ContactFieldPivot, error) {
	if len(fieldIDs) == 0 {
		return map[int]dashboard.ContactFieldPivot{}, nil
	}
	placeholders := make([]string, 0, len(fieldIDs))
	args := []any{contactID}
	for _, fieldID := range fieldIDs {
		placeholders = append(placeholders, "?")
		args = append(args, fieldID)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, contact_id, contact_field_id, value
		FROM mc_contact_field_pivot
		WHERE contact_id = ? AND contact_field_id IN (`+strings.Join(placeholders, ",")+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pivots := make(map[int]dashboard.ContactFieldPivot)
	for rows.Next() {
		var pivot dashboard.ContactFieldPivot
		var value sql.NullString
		if err := rows.Scan(&pivot.ID, &pivot.ContactID, &pivot.ContactFieldID, &value); err != nil {
			return nil, err
		}
		pivot.Value = nullString(value)
		pivots[pivot.ContactFieldID] = pivot
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pivots, nil
}

func (s *MySQLStore) ContactFieldPivotByID(ctx context.Context, pivotID int) (dashboard.ContactFieldPivot, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, contact_id, contact_field_id, value
		FROM mc_contact_field_pivot
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, pivotID)

	var pivot dashboard.ContactFieldPivot
	var value sql.NullString
	if err := row.Scan(&pivot.ID, &pivot.ContactID, &pivot.ContactFieldID, &value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dashboard.ContactFieldPivot{}, false, nil
		}
		return dashboard.ContactFieldPivot{}, false, err
	}
	pivot.Value = nullString(value)
	return pivot, true, nil
}

func (s *MySQLStore) ContactAccessibleToEmployee(ctx context.Context, contactID int, employeeID int, corpID int) (bool, error) {
	var total int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_work_contact contact
		JOIN mc_work_contact_employee relation
			ON relation.contact_id = contact.id AND relation.deleted_at IS NULL
		WHERE contact.id = ? AND contact.corp_id = ? AND relation.employee_id = ?
			AND contact.deleted_at IS NULL
	`, contactID, corpID, employeeID).Scan(&total)
	return total > 0, err
}

func (s *MySQLStore) UpdateContactFieldPivotValue(ctx context.Context, pivotID int, value string) (bool, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_contact_field_pivot
		SET value = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, value, pivotID)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) CreateContactFieldPivots(ctx context.Context, pivots []dashboard.ContactFieldPivotCreate) error {
	if len(pivots) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	for _, pivot := range pivots {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_contact_field_pivot
				(contact_id, contact_field_id, value, created_at, updated_at)
			VALUES (?, ?, ?, NOW(), NOW())
		`, pivot.ContactID, pivot.ContactFieldID, pivot.Value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *MySQLStore) CreateContactEmployeeTrack(ctx context.Context, track dashboard.ContactEmployeeTrackCreate) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_contact_employee_track
			(employee_id, contact_id, content, corp_id, event, created_at)
		VALUES (?, ?, ?, ?, ?, NOW())
	`, track.EmployeeID, track.ContactID, track.Content, track.CorpID, track.Event)
	return err
}

func (s *MySQLStore) UpdateContactFieldPivotsAtomically(ctx context.Context, write dashboard.ContactFieldPivotBatchWrite) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)

	var authorizedContactID int
	if write.RequireEmployeeAccess {
		err = tx.QueryRowContext(ctx, `
			SELECT contact.id
			FROM mc_work_contact AS contact
			JOIN mc_work_contact_employee AS relation
				ON relation.contact_id = contact.id
				AND relation.corp_id = contact.corp_id
				AND relation.deleted_at IS NULL
			JOIN mc_work_employee AS employee
				ON employee.id = relation.employee_id
				AND employee.corp_id = contact.corp_id
				AND employee.deleted_at IS NULL
			WHERE contact.id = ? AND contact.corp_id = ? AND relation.employee_id = ?
				AND contact.deleted_at IS NULL
			LIMIT 1
			FOR UPDATE
		`, write.ContactID, write.CorpID, write.EmployeeID).Scan(&authorizedContactID)
	} else {
		err = tx.QueryRowContext(ctx, `
			SELECT contact.id
			FROM mc_work_contact AS contact
			JOIN mc_work_employee AS employee
				ON employee.id = ?
				AND employee.corp_id = contact.corp_id
				AND employee.deleted_at IS NULL
			WHERE contact.id = ? AND contact.corp_id = ? AND contact.deleted_at IS NULL
			LIMIT 1
			FOR UPDATE
		`, write.EmployeeID, write.ContactID, write.CorpID).Scan(&authorizedContactID)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.ErrContactFieldPivotAccess
	}
	if err != nil {
		return err
	}
	if len(write.Items) == 0 {
		return nil
	}

	fieldIDs := make([]int, 0, len(write.Items))
	pivotIDs := make([]int, 0, len(write.Items))
	for _, item := range write.Items {
		fieldIDs = append(fieldIDs, item.ContactFieldID)
		if item.PivotID > 0 {
			pivotIDs = append(pivotIDs, item.PivotID)
		}
	}
	fieldIDs = uniquePositiveInts(fieldIDs)
	if len(fieldIDs) == 0 {
		return dashboard.ErrContactFieldPivotFieldNotFound
	}
	fieldArgs := make([]any, 0, len(fieldIDs))
	for _, fieldID := range fieldIDs {
		fieldArgs = append(fieldArgs, fieldID)
	}
	fieldRows, err := tx.QueryContext(ctx, `
		SELECT id
		FROM mc_contact_field
		WHERE id IN (`+placeholders(len(fieldIDs))+`) AND deleted_at IS NULL
		FOR UPDATE
	`, fieldArgs...)
	if err != nil {
		return err
	}
	validFieldIDs := make(map[int]struct{}, len(fieldIDs))
	for fieldRows.Next() {
		var fieldID int
		if err := fieldRows.Scan(&fieldID); err != nil {
			fieldRows.Close()
			return err
		}
		validFieldIDs[fieldID] = struct{}{}
	}
	if err := fieldRows.Err(); err != nil {
		fieldRows.Close()
		return err
	}
	fieldRows.Close()
	if len(validFieldIDs) != len(fieldIDs) {
		return dashboard.ErrContactFieldPivotFieldNotFound
	}

	pivots := make(map[int]dashboard.ContactFieldPivot, len(pivotIDs))
	pivotIDs = uniquePositiveInts(pivotIDs)
	if len(pivotIDs) > 0 {
		pivotArgs := make([]any, 0, len(pivotIDs))
		for _, pivotID := range pivotIDs {
			pivotArgs = append(pivotArgs, pivotID)
		}
		rows, err := tx.QueryContext(ctx, `
			SELECT id, contact_id, contact_field_id, value
			FROM mc_contact_field_pivot
			WHERE id IN (`+placeholders(len(pivotIDs))+`) AND deleted_at IS NULL
			FOR UPDATE
		`, pivotArgs...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var pivot dashboard.ContactFieldPivot
			var value sql.NullString
			if err := rows.Scan(&pivot.ID, &pivot.ContactID, &pivot.ContactFieldID, &value); err != nil {
				rows.Close()
				return err
			}
			pivot.Value = nullString(value)
			pivots[pivot.ID] = pivot
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
	}
	for _, item := range write.Items {
		if item.PivotID <= 0 {
			continue
		}
		pivot, ok := pivots[item.PivotID]
		if !ok || pivot.ContactID != write.ContactID || pivot.ContactFieldID != item.ContactFieldID {
			return dashboard.ErrContactFieldPivotAccess
		}
	}

	content := "编辑用户画像："
	for _, item := range write.Items {
		if item.PivotID > 0 {
			if pivots[item.PivotID].Value != item.Value {
				content += item.Name + " "
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE mc_contact_field_pivot
				SET value = ?, updated_at = NOW()
				WHERE id = ? AND deleted_at IS NULL
			`, item.Value, item.PivotID); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_contact_field_pivot
				(contact_id, contact_field_id, value, created_at, updated_at)
			VALUES (?, ?, ?, NOW(), NOW())
		`, write.ContactID, item.ContactFieldID, item.Value); err != nil {
			return err
		}
		if item.Value != "" {
			content += item.Name + " "
		}
	}
	if content != "编辑用户画像：" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_contact_employee_track
				(employee_id, contact_id, content, corp_id, event, created_at)
			VALUES (?, ?, ?, ?, ?, NOW())
		`, write.EmployeeID, write.ContactID, content, write.CorpID, 4); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *MySQLStore) ContactFieldLabelExists(ctx context.Context, label string, excludeFieldID int) (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM mc_contact_field
		WHERE label = ? AND deleted_at IS NULL
	`
	args := []any{label}
	if excludeFieldID > 0 {
		query += ` AND id <> ?`
		args = append(args, excludeFieldID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return false, err
	}
	return total > 0, nil
}

func (s *MySQLStore) CreateContactField(ctx context.Context, values dashboard.ContactFieldWriteValues) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_contact_field
			(name, label, type, options, `+"`order`"+`, status, is_sys, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, NOW(), NOW())
	`, values.Name, values.Label, values.Type, values.Options, values.Order, values.Status)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *MySQLStore) UpdateContactField(ctx context.Context, fieldID int, values dashboard.ContactFieldWriteValues) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_contact_field
		SET name = ?, label = ?, type = ?, options = ?, `+"`order`"+` = ?, status = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, values.Name, values.Label, values.Type, values.Options, values.Order, values.Status, fieldID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) UpdateContactFieldStatus(ctx context.Context, fieldID int, status int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_contact_field
		SET status = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, status, fieldID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) DeleteContactField(ctx context.Context, fieldID int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_contact_field
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, fieldID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) BatchUpdateContactFields(ctx context.Context, updates []dashboard.ContactFieldBatchUpdate, destroyIDs []int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)

	for _, update := range updates {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_contact_field
			SET name = ?, label = ?, type = ?, options = ?, `+"`order`"+` = ?, status = ?, updated_at = NOW()
			WHERE id = ? AND deleted_at IS NULL
		`, update.Values.Name, update.Values.Label, update.Values.Type, update.Values.Options, update.Values.Order, update.Values.Status, update.ID); err != nil {
			return err
		}
	}

	if len(destroyIDs) > 0 {
		args := make([]any, 0, len(destroyIDs))
		for _, id := range destroyIDs {
			args = append(args, id)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_contact_field
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE id IN (`+placeholders(len(destroyIDs))+`) AND is_sys = 0 AND deleted_at IS NULL
		`, args...); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *MySQLStore) WorkAgentsByCorpID(ctx context.Context, corpID int) ([]dashboard.WorkAgent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, corp_id, name, square_logo_url
		FROM mc_work_agent
		WHERE corp_id = ? AND close = 0 AND deleted_at IS NULL
		ORDER BY id ASC
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agents := make([]dashboard.WorkAgent, 0)
	for rows.Next() {
		var agent dashboard.WorkAgent
		if err := rows.Scan(&agent.ID, &agent.CorpID, &agent.Name, &agent.SquareLogoURL); err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, rows.Err()
}

func (s *MySQLStore) EnabledChatTools(ctx context.Context) ([]dashboard.ChatTool, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, page_name, page_flag
		FROM mc_chat_tool
		WHERE status = 1 AND deleted_at IS NULL
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tools := make([]dashboard.ChatTool, 0)
	for rows.Next() {
		var tool dashboard.ChatTool
		if err := rows.Scan(&tool.ID, &tool.PageName, &tool.PageFlag); err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, rows.Err()
}

func (s *MySQLStore) RolesByTenantID(ctx context.Context, tenantID int) ([]dashboard.RoleOption, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name
		FROM mc_rbac_role
		WHERE tenant_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]dashboard.RoleOption, 0)
	for rows.Next() {
		var role dashboard.RoleOption
		if err := rows.Scan(&role.ID, &role.Name); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (s *MySQLStore) RoleList(ctx context.Context, filter dashboard.RoleListFilter) (dashboard.RoleListPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10
	}

	where := "WHERE tenant_id = ? AND deleted_at IS NULL"
	args := []any{filter.TenantID}
	if filter.Name != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+filter.Name+"%")
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_rbac_role `+where, args...).Scan(&total); err != nil {
		return dashboard.RoleListPage{}, err
	}
	if total == 0 {
		return dashboard.RoleListPage{Items: []dashboard.RoleListItem{}}, nil
	}

	totalPage := (total + filter.PerPage - 1) / filter.PerPage
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, remarks, updated_at, status
		FROM mc_rbac_role
		`+where+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoleListPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.RoleListItem, 0)
	for rows.Next() {
		var item dashboard.RoleListItem
		var remarks sql.NullString
		var updatedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Name, &remarks, &updatedAt, &item.Status); err != nil {
			return dashboard.RoleListPage{}, err
		}
		item.Remarks = nullString(remarks)
		item.UpdatedAt = formatTime(updatedAt)
		employeeNum, err := s.roleEmployeeCount(ctx, item.ID, filter.CorpID)
		if err != nil {
			return dashboard.RoleListPage{}, err
		}
		item.EmployeeNum = employeeNum
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoleListPage{}, err
	}

	return dashboard.RoleListPage{
		Items:     items,
		Total:     total,
		TotalPage: totalPage,
	}, nil
}

func (s *MySQLStore) RoleDetailByIDTenant(ctx context.Context, roleID int, tenantID int) (dashboard.RoleDetail, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, remarks, data_permission
		FROM mc_rbac_role
		WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
	`, roleID, tenantID)

	var role dashboard.RoleDetail
	var remarks, dataPermission sql.NullString
	err := row.Scan(&role.ID, &role.Name, &remarks, &dataPermission)
	if err == sql.ErrNoRows {
		return dashboard.RoleDetail{}, false, nil
	}
	if err != nil {
		return dashboard.RoleDetail{}, false, err
	}
	role.Remarks = nullString(remarks)
	role.DataPermission = nullString(dataPermission)
	return role, true, nil
}

func (s *MySQLStore) RolePermissionMenus(ctx context.Context, roleID int) ([]dashboard.RolePermissionMenu, []int, error) {
	menuIDs, err := s.MenuIDsByRole(ctx, roleID)
	if err != nil {
		return nil, nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, parent_id, name, level, is_page_menu
		FROM mc_rbac_menu
		WHERE status = 1 AND deleted_at IS NULL
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	menus := make([]dashboard.RolePermissionMenu, 0)
	for rows.Next() {
		var menu dashboard.RolePermissionMenu
		if err := rows.Scan(&menu.ID, &menu.ParentID, &menu.Name, &menu.Level, &menu.IsPageMenu); err != nil {
			return nil, nil, err
		}
		menus = append(menus, menu)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return menus, menuIDs, nil
}

func (s *MySQLStore) RoleEmployees(ctx context.Context, filter dashboard.RoleEmployeeFilter) (dashboard.RoleEmployeePage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10
	}

	userIDs, err := s.userIDsByRole(ctx, filter.RoleID)
	if err != nil || len(userIDs) == 0 {
		return dashboard.RoleEmployeePage{Items: []dashboard.RoleEmployee{}}, err
	}

	args := make([]any, 0, len(userIDs)+1)
	args = append(args, filter.CorpID)
	for _, userID := range userIDs {
		args = append(args, userID)
	}

	where := "WHERE we.corp_id = ? AND we.log_user_id IN (" + placeholders(len(userIDs)) + ") AND we.deleted_at IS NULL"
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_work_employee we `+where, args...).Scan(&total); err != nil {
		return dashboard.RoleEmployeePage{}, err
	}
	if total == 0 {
		return dashboard.RoleEmployeePage{Items: []dashboard.RoleEmployee{}}, nil
	}

	totalPage := (total + filter.PerPage - 1) / filter.PerPage
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT we.id, we.name, we.mobile, we.email,
		       COALESCE(GROUP_CONCAT(wd.name ORDER BY wd.id SEPARATOR ','), '') AS department
		FROM mc_work_employee we
		LEFT JOIN mc_work_employee_department wed ON wed.employee_id = we.id AND wed.deleted_at IS NULL
		LEFT JOIN mc_work_department wd ON wd.id = wed.department_id AND wd.deleted_at IS NULL
		`+where+`
		GROUP BY we.id, we.name, we.mobile, we.email
		ORDER BY we.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoleEmployeePage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.RoleEmployee, 0)
	for rows.Next() {
		var employee dashboard.RoleEmployee
		var email, department sql.NullString
		if err := rows.Scan(&employee.ID, &employee.Name, &employee.Mobile, &email, &department); err != nil {
			return dashboard.RoleEmployeePage{}, err
		}
		employee.Email = nullString(email)
		employee.Department = nullString(department)
		items = append(items, employee)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoleEmployeePage{}, err
	}

	return dashboard.RoleEmployeePage{
		Items:     items,
		Total:     total,
		TotalPage: totalPage,
	}, nil
}

func (s *MySQLStore) RoleNameExists(ctx context.Context, name string, tenantID int, excludeRoleID int) (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM mc_rbac_role
		WHERE name = ? AND tenant_id = ? AND deleted_at IS NULL
	`
	args := []any{name, tenantID}
	if excludeRoleID > 0 {
		query += ` AND id <> ?`
		args = append(args, excludeRoleID)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return false, err
	}
	return total > 0, nil
}

func (s *MySQLStore) CreateRole(ctx context.Context, values dashboard.RoleCreateValues, copyFromRoleID int) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer rollbackQuietly(tx)

	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_rbac_role
			(tenant_id, name, remarks, status, operate_id, operate_name, data_permission, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?, ?, NOW(), NOW())
	`, values.TenantID, values.Name, values.Remarks, values.OperateID, values.OperateName, values.DataPermission)
	if err != nil {
		return 0, err
	}
	roleID64, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	roleID := int(roleID64)

	if copyFromRoleID > 0 {
		rows, err := tx.QueryContext(ctx, `
			SELECT menu_id
			FROM mc_rbac_role_menu
			WHERE role_id = ?
			ORDER BY id ASC
		`, copyFromRoleID)
		if err != nil {
			return 0, err
		}
		menuIDs, err := scanIntColumn(rows)
		if err != nil {
			return 0, err
		}
		if len(menuIDs) == 0 {
			return 0, errors.New("该角色还没有设置权限")
		}
		for _, menuID := range menuIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO mc_rbac_role_menu (role_id, menu_id, created_at, updated_at)
				VALUES (?, ?, NOW(), NOW())
			`, roleID, menuID); err != nil {
				return 0, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return roleID, nil
}

func (s *MySQLStore) UpdateRole(ctx context.Context, roleID int, tenantID int, values dashboard.RoleUpdateValues) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_rbac_role
		SET name = ?, remarks = ?, operate_id = ?, operate_name = ?, data_permission = ?, updated_at = NOW()
		WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
	`, values.Name, values.Remarks, values.OperateID, values.OperateName, values.DataPermission, roleID, tenantID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) UpdateRoleStatus(ctx context.Context, roleID int, tenantID int, status int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_rbac_role
		SET status = ?, updated_at = NOW()
		WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
	`, status, roleID, tenantID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) DeleteRole(ctx context.Context, roleID int, tenantID int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_rbac_role
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
	`, roleID, tenantID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) RoleEmployeeCount(ctx context.Context, roleID int, corpID int) (int, error) {
	return s.roleEmployeeCount(ctx, roleID, corpID)
}

func (s *MySQLStore) ExpandedMenuIDs(ctx context.Context, menuIDs []int) ([]int, error) {
	if len(menuIDs) == 0 {
		return []int{}, nil
	}
	args := make([]any, 0, len(menuIDs))
	for _, menuID := range menuIDs {
		args = append(args, menuID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, path
		FROM mc_rbac_menu
		WHERE id IN (`+placeholders(len(menuIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := map[int]struct{}{}
	for _, menuID := range menuIDs {
		if menuID > 0 {
			seen[menuID] = struct{}{}
		}
	}
	for rows.Next() {
		var id int
		var path sql.NullString
		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		if id > 0 {
			seen[id] = struct{}{}
		}
		for _, pathID := range idsFromMenuPath(nullString(path)) {
			seen[pathID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	expanded := make([]int, 0, len(seen))
	for id := range seen {
		expanded = append(expanded, id)
	}
	sort.Ints(expanded)
	return expanded, nil
}

func (s *MySQLStore) ReplaceRoleMenus(ctx context.Context, roleID int, menuIDs []int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)

	rows, err := tx.QueryContext(ctx, `
		SELECT id, menu_id
		FROM mc_rbac_role_menu
		WHERE role_id = ?
	`, roleID)
	if err != nil {
		return err
	}

	current := map[int]int{}
	for rows.Next() {
		var id int
		var menuID int
		if err := rows.Scan(&id, &menuID); err != nil {
			_ = rows.Close()
			return err
		}
		current[menuID] = id
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	target := map[int]struct{}{}
	for _, menuID := range menuIDs {
		if menuID <= 0 {
			continue
		}
		target[menuID] = struct{}{}
		if _, exists := current[menuID]; !exists {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO mc_rbac_role_menu (role_id, menu_id, created_at, updated_at)
				VALUES (?, ?, NOW(), NOW())
			`, roleID, menuID); err != nil {
				return err
			}
		}
	}

	for menuID, id := range current {
		if _, keep := target[menuID]; keep {
			continue
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM mc_rbac_role_menu WHERE id = ?`, id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *MySQLStore) roleEmployeeCount(ctx context.Context, roleID int, corpID int) (int, error) {
	userIDs, err := s.userIDsByRole(ctx, roleID)
	if err != nil || len(userIDs) == 0 {
		return 0, err
	}

	args := make([]any, 0, len(userIDs)+1)
	args = append(args, corpID)
	for _, userID := range userIDs {
		args = append(args, userID)
	}

	var total int
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(id)
		FROM mc_work_employee
		WHERE corp_id = ? AND log_user_id IN (`+placeholders(len(userIDs))+`) AND deleted_at IS NULL
	`, args...).Scan(&total)
	return total, err
}

func (s *MySQLStore) userIDsByRole(ctx context.Context, roleID int) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id
		FROM mc_rbac_user_role
		WHERE role_id = ?
		ORDER BY id ASC
		LIMIT 1000
	`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) MenuOptions(ctx context.Context) ([]dashboard.MenuOption, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, level, parent_id, data_permission
		FROM mc_rbac_menu
		WHERE deleted_at IS NULL
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	menus := make([]dashboard.MenuOption, 0)
	for rows.Next() {
		var menu dashboard.MenuOption
		if err := rows.Scan(&menu.ID, &menu.Name, &menu.Level, &menu.ParentID, &menu.DataPermission); err != nil {
			return nil, err
		}
		menus = append(menus, menu)
	}
	return menus, rows.Err()
}

func (s *MySQLStore) MenuIcons(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT icon
		FROM mc_rbac_menu
		WHERE deleted_at IS NULL
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	icons := make([]string, 0)
	for rows.Next() {
		var icon string
		if err := rows.Scan(&icon); err != nil {
			return nil, err
		}
		if icon != "" {
			icons = append(icons, icon)
		}
	}
	return icons, rows.Err()
}

func (s *MySQLStore) MenuList(ctx context.Context, filter dashboard.MenuListFilter) ([]dashboard.MenuListItem, error) {
	var ids []int
	name := strings.TrimSpace(filter.Name)
	if name != "" {
		var err error
		ids, err = s.menuIDsByNameScope(ctx, name)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return []dashboard.MenuListItem{}, nil
		}
	}

	where := "WHERE deleted_at IS NULL"
	args := []any{}
	if len(ids) > 0 {
		where += " AND id IN (" + placeholders(len(ids)) + ")"
		for _, id := range ids {
			args = append(args, id)
		}
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, level, parent_id, icon, status, operate_name, updated_at
		FROM mc_rbac_menu
		`+where+`
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]dashboard.MenuListItem, 0)
	for rows.Next() {
		var item dashboard.MenuListItem
		var icon, operateName sql.NullString
		var updatedAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Level,
			&item.ParentID,
			&icon,
			&item.Status,
			&operateName,
			&updatedAt,
		); err != nil {
			return nil, err
		}
		item.Icon = nullString(icon)
		item.OperateName = nullString(operateName)
		item.UpdatedAt = formatTime(updatedAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) MenuDetailByID(ctx context.Context, menuID int) (dashboard.MenuDetail, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, level, status, icon, link_url, is_page_menu, link_type, data_permission, path
		FROM mc_rbac_menu
		WHERE id = ? AND deleted_at IS NULL
	`, menuID)

	var menu dashboard.MenuDetail
	var icon, linkURL, path sql.NullString
	var linkType, dataPermission sql.NullInt64
	err := row.Scan(
		&menu.ID,
		&menu.Name,
		&menu.Level,
		&menu.Status,
		&icon,
		&linkURL,
		&menu.IsPageMenu,
		&linkType,
		&dataPermission,
		&path,
	)
	if err == sql.ErrNoRows {
		return dashboard.MenuDetail{}, false, nil
	}
	if err != nil {
		return dashboard.MenuDetail{}, false, err
	}
	menu.Icon = nullString(icon)
	menu.LinkURL = nullString(linkURL)
	if linkType.Valid {
		menu.LinkType = int(linkType.Int64)
	}
	if dataPermission.Valid {
		menu.DataPermission = int(dataPermission.Int64)
	}
	menu.Path = nullString(path)
	return menu, true, nil
}

func (s *MySQLStore) MenuLinkURLExists(ctx context.Context, linkURL string, excludeMenuID int) (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM mc_rbac_menu
		WHERE link_url = ? AND deleted_at IS NULL
	`
	args := []any{linkURL}
	if excludeMenuID > 0 {
		query += ` AND id <> ?`
		args = append(args, excludeMenuID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return false, err
	}
	return total > 0, nil
}

func (s *MySQLStore) CreateMenu(ctx context.Context, values dashboard.MenuCreateValues) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer rollbackQuietly(tx)

	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_rbac_menu
			(parent_id, name, level, path, icon, status, link_type, is_page_menu, link_url, data_permission, operate_id, operate_name, sort, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, 99, NOW(), NOW())
	`, values.ParentID, values.Name, values.Level, values.Path, values.Icon, values.LinkType, values.IsPageMenu, values.LinkURL, values.DataPermission, values.OperateID, values.OperateName)
	if err != nil {
		return 0, err
	}
	menuID64, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	menuID := int(menuID64)
	finalPath := "#" + strconv.Itoa(menuID) + "#"
	if values.Path != "" {
		finalPath = values.Path + "-" + finalPath
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_rbac_menu
		SET path = ?, updated_at = NOW()
		WHERE id = ?
	`, finalPath, menuID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return menuID, nil
}

func (s *MySQLStore) UpdateMenu(ctx context.Context, menuID int, values dashboard.MenuUpdateValues) (bool, error) {
	sets := []string{"name = ?"}
	args := []any{values.Name}
	if values.UpdateIcon {
		sets = append(sets, "icon = ?")
		args = append(args, values.Icon)
	}
	if values.UpdateLinkFields {
		sets = append(sets, "link_type = ?", "is_page_menu = ?", "link_url = ?")
		args = append(args, values.LinkType, values.IsPageMenu, values.LinkURL)
	}
	if values.UpdateDataPermission {
		sets = append(sets, "data_permission = ?")
		args = append(args, values.DataPermission)
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, menuID)

	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_rbac_menu
		SET `+strings.Join(sets, ", ")+`
		WHERE id = ? AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) UpdateMenuStatus(ctx context.Context, menuID int, status int, cascade bool) (bool, error) {
	where := "id = ?"
	args := []any{status, menuID}
	if cascade {
		where = "(id = ? OR path LIKE ?)"
		args = []any{status, menuID, "%#" + strconv.Itoa(menuID) + "#%"}
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_rbac_menu
		SET status = ?, updated_at = NOW()
		WHERE `+where+` AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) DeleteMenuCascade(ctx context.Context, menuID int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_rbac_menu
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE (id = ? OR path LIKE ?) AND deleted_at IS NULL
	`, menuID, "%#"+strconv.Itoa(menuID)+"#%")
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) menuIDsByNameScope(ctx context.Context, name string) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT path
		FROM mc_rbac_menu
		WHERE name LIKE ? AND deleted_at IS NULL
	`, "%"+name+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rootByID := map[string]struct{}{}
	roots := make([]string, 0)
	for rows.Next() {
		var path sql.NullString
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		if !path.Valid {
			continue
		}
		pathValue := path.String
		if strings.Contains(pathValue, "-") {
			pathValue = strings.SplitN(pathValue, "-", 2)[0]
		}
		rootID := strings.ReplaceAll(pathValue, "#", "")
		if rootID == "" {
			continue
		}
		if _, exists := rootByID[rootID]; exists {
			continue
		}
		rootByID[rootID] = struct{}{}
		roots = append(roots, rootID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(roots) == 0 {
		return []int{}, nil
	}

	conditions := make([]string, 0, len(roots))
	args := make([]any, 0, len(roots))
	for _, rootID := range roots {
		conditions = append(conditions, "path LIKE ?")
		args = append(args, "%#"+rootID+"#%")
	}

	rows, err = s.db.QueryContext(ctx, `
		SELECT id
		FROM mc_rbac_menu
		WHERE deleted_at IS NULL AND (`+strings.Join(conditions, " OR ")+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_employee
		WHERE log_user_id = ? AND corp_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, userID, corpID)
	var employeeID int
	err := row.Scan(&employeeID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return employeeID, err
}

func (s *MySQLStore) CorpIDsByTenant(ctx context.Context, tenantID int) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM mc_corp
		WHERE tenant_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanIntColumn(rows)
}

func (s *MySQLStore) CorpIDsByUser(ctx context.Context, userID int) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT corp_id
		FROM mc_work_employee
		WHERE log_user_id = ? AND deleted_at IS NULL
		ORDER BY corp_id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanIntColumn(rows)
}

func (s *MySQLStore) CorpsByIDsName(ctx context.Context, corpIDs []int, name string) ([]dashboard.Corp, error) {
	if len(corpIDs) == 0 {
		return []dashboard.Corp{}, nil
	}

	args := make([]any, 0, len(corpIDs)+1)
	for _, corpID := range corpIDs {
		args = append(args, corpID)
	}

	query := `
		SELECT id, name
		FROM mc_corp
		WHERE id IN (` + placeholders(len(corpIDs)) + `) AND deleted_at IS NULL
	`
	if name != "" {
		query += ` AND name LIKE ?`
		args = append(args, "%"+name+"%")
	}
	query += ` ORDER BY id ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var corps []dashboard.Corp
	for rows.Next() {
		var corp dashboard.Corp
		if err := rows.Scan(&corp.ID, &corp.Name); err != nil {
			return nil, err
		}
		corps = append(corps, corp)
	}
	return corps, rows.Err()
}

func (s *MySQLStore) FirstEmployeeByUser(ctx context.Context, userID int) (int, int, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT corp_id, id
		FROM mc_work_employee
		WHERE log_user_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, userID)
	var corpID int
	var employeeID int
	err := row.Scan(&corpID, &employeeID)
	if err == sql.ErrNoRows {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return corpID, employeeID, true, nil
}

func (s *MySQLStore) RoleIDByUserTenant(ctx context.Context, userID int, tenantID int) (int, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT r.id
		FROM mc_rbac_user_role ur
		INNER JOIN mc_rbac_role r ON r.id = ur.role_id
		WHERE ur.user_id = ? AND r.tenant_id = ? AND (ur.deleted_at IS NULL) AND r.deleted_at IS NULL
		ORDER BY ur.id ASC
		LIMIT 1
	`, userID, tenantID)

	var roleID int
	err := row.Scan(&roleID)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return roleID, true, nil
}

func (s *MySQLStore) RolesByUserTenant(ctx context.Context, userID int, tenantID int) ([]dashboard.Role, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.data_permission, r.status
		FROM mc_rbac_user_role ur
		INNER JOIN mc_rbac_role r ON r.id = ur.role_id
		WHERE ur.user_id = ? AND r.tenant_id = ? AND ur.deleted_at IS NULL AND r.deleted_at IS NULL
		ORDER BY ur.id ASC
	`, userID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []dashboard.Role
	for rows.Next() {
		var role dashboard.Role
		var dataPermission sql.NullString
		if err := rows.Scan(&role.ID, &dataPermission, &role.Status); err != nil {
			return nil, err
		}
		if dataPermission.Valid {
			role.DataPermission = dataPermission.String
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (s *MySQLStore) MenuIDsByRole(ctx context.Context, roleID int) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT menu_id
		FROM mc_rbac_role_menu
		WHERE role_id = ?
		ORDER BY id ASC
	`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var menuIDs []int
	for rows.Next() {
		var menuID int
		if err := rows.Scan(&menuID); err != nil {
			return nil, err
		}
		menuIDs = append(menuIDs, menuID)
	}
	return menuIDs, rows.Err()
}

func (s *MySQLStore) RoleMenusByRoleIDs(ctx context.Context, roleIDs []int) ([]dashboard.RoleMenu, error) {
	if len(roleIDs) == 0 {
		return []dashboard.RoleMenu{}, nil
	}

	args := make([]any, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		args = append(args, roleID)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT role_id, menu_id
		FROM mc_rbac_role_menu
		WHERE role_id IN (`+placeholders(len(roleIDs))+`)
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roleMenus []dashboard.RoleMenu
	for rows.Next() {
		var roleMenu dashboard.RoleMenu
		if err := rows.Scan(&roleMenu.RoleID, &roleMenu.MenuID); err != nil {
			return nil, err
		}
		roleMenus = append(roleMenus, roleMenu)
	}
	return roleMenus, rows.Err()
}

const pageMenusQuery = `
		SELECT id, name, level, data_permission, icon, link_type, link_url, parent_id, is_page_menu, sort
		FROM mc_rbac_menu
		WHERE is_page_menu IN (1, 2) AND deleted_at IS NULL
		ORDER BY sort ASC
	`

func (s *MySQLStore) PageMenus(ctx context.Context) ([]dashboard.Menu, error) {
	return s.queryMenus(ctx, pageMenusQuery)
}

func (s *MySQLStore) MenusByIDs(ctx context.Context, menuIDs []int) ([]dashboard.Menu, error) {
	if len(menuIDs) == 0 {
		return []dashboard.Menu{}, nil
	}

	args := make([]any, 0, len(menuIDs))
	for _, menuID := range menuIDs {
		args = append(args, menuID)
	}

	return s.queryMenus(ctx, `
		SELECT id, name, level, data_permission, icon, link_type, link_url, parent_id, is_page_menu, sort
		FROM mc_rbac_menu
		WHERE id IN (`+placeholders(len(menuIDs))+`) AND deleted_at IS NULL
		ORDER BY sort ASC
	`, args...)
}

func (s *MySQLStore) MenuByLinkURL(ctx context.Context, linkURL string) (dashboard.Menu, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, level, data_permission, icon, link_type, link_url, parent_id, is_page_menu, sort
		FROM mc_rbac_menu
		WHERE link_url = ? AND deleted_at IS NULL
		LIMIT 1
	`, linkURL)

	var menu dashboard.Menu
	err := row.Scan(
		&menu.ID,
		&menu.Name,
		&menu.Level,
		&menu.DataPermission,
		&menu.Icon,
		&menu.LinkType,
		&menu.LinkURL,
		&menu.ParentID,
		&menu.IsPageMenu,
		&menu.Sort,
	)
	if err == sql.ErrNoRows {
		return dashboard.Menu{}, false, nil
	}
	if err != nil {
		return dashboard.Menu{}, false, err
	}
	return menu, true, nil
}

func (s *MySQLStore) DepartmentEmployeeIDs(ctx context.Context, employeeID int) ([]int, error) {
	departmentIDs, err := s.departmentIDsByEmployee(ctx, employeeID)
	if err != nil || len(departmentIDs) == 0 {
		return departmentIDs, err
	}

	departmentPaths, err := s.departmentPathsByIDs(ctx, departmentIDs)
	if err != nil || len(departmentPaths) == 0 {
		return []int{}, err
	}

	childDepartmentIDs, err := s.departmentIDsByParentPaths(ctx, departmentPaths)
	if err != nil || len(childDepartmentIDs) == 0 {
		return []int{}, err
	}

	return s.employeeIDsByDepartmentIDs(ctx, childDepartmentIDs)
}

func (s *MySQLStore) departmentIDsByEmployee(ctx context.Context, employeeID int) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT department_id
		FROM mc_work_employee_department
		WHERE employee_id = ? AND deleted_at IS NULL
	`, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanIntColumn(rows)
}

func (s *MySQLStore) departmentPathsByIDs(ctx context.Context, departmentIDs []int) ([]string, error) {
	args := make([]any, 0, len(departmentIDs))
	for _, departmentID := range departmentIDs {
		args = append(args, departmentID)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT path
		FROM mc_work_department
		WHERE id IN (`+placeholders(len(departmentIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

func (s *MySQLStore) departmentIDsByParentPaths(ctx context.Context, paths []string) ([]int, error) {
	args := make([]any, 0, len(paths))
	conditions := make([]string, 0, len(paths))
	for _, path := range paths {
		conditions = append(conditions, "path LIKE ?")
		args = append(args, path+"%")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM mc_work_department
		WHERE parent_id != 0 AND deleted_at IS NULL AND (`+strings.Join(conditions, " OR ")+`)
		LIMIT 1000
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanIntColumn(rows)
}

func (s *MySQLStore) employeeIDsByDepartmentIDs(ctx context.Context, departmentIDs []int) ([]int, error) {
	args := make([]any, 0, len(departmentIDs))
	for _, departmentID := range departmentIDs {
		args = append(args, departmentID)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT employee_id
		FROM mc_work_employee_department
		WHERE department_id IN (`+placeholders(len(departmentIDs))+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanIntColumn(rows)
}

func (s *MySQLStore) queryMenus(ctx context.Context, query string, args ...any) ([]dashboard.Menu, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var menus []dashboard.Menu
	for rows.Next() {
		var menu dashboard.Menu
		if err := rows.Scan(
			&menu.ID,
			&menu.Name,
			&menu.Level,
			&menu.DataPermission,
			&menu.Icon,
			&menu.LinkType,
			&menu.LinkURL,
			&menu.ParentID,
			&menu.IsPageMenu,
			&menu.Sort,
		); err != nil {
			return nil, err
		}
		menus = append(menus, menu)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return menus, nil
}

func (s *MySQLStore) countScalar(ctx context.Context, query string, args ...any) (int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func countScalarTx(ctx context.Context, tx *sql.Tx, query string, args ...any) (int, error) {
	var total int
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *MySQLStore) ContactMessageBatchSendPage(ctx context.Context, filter dashboard.ContactMessageBatchSendFilter) (dashboard.ContactMessageBatchSendPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10
	}
	where := []string{"batch.user_id = ?", "batch.deleted_at IS NULL"}
	args := []any{filter.UserID}
	if filter.TenantID > 0 && filter.CorpID > 0 {
		// Resolve the tenant through mc_corp so reads also work before the
		// controlled 0139 batch-table tenant backfill has been executed.
		where = append(where, "corp.tenant_id = ?", "batch.corp_id = ?")
		args = append(args, filter.TenantID, filter.CorpID)
	}
	if filter.RestrictEmployeeIDs {
		ids := uniquePositiveInts(filter.AllowedEmployeeIDs)
		if len(ids) == 0 {
			return dashboard.ContactMessageBatchSendPage{}, nil
		}
		parts := make([]string, 0, len(ids))
		for _, id := range ids {
			parts = append(parts, "JSON_CONTAINS(employee_ids, JSON_ARRAY(?))")
			args = append(args, id)
		}
		where = append(where, "("+strings.Join(parts, " OR ")+")")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_contact_message_batch_send batch
		JOIN mc_corp corp ON corp.id=batch.corp_id
		WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return dashboard.ContactMessageBatchSendPage{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	if total == 0 {
		return dashboard.ContactMessageBatchSendPage{Items: []dashboard.ContactMessageBatchSendItem{}, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT batch.id, batch.corp_id, batch.user_id, batch.medium_id, batch.batch_title, batch.user_name, batch.employee_ids, batch.filter_params, batch.filter_params_detail, batch.content,
		       batch.send_way, batch.definite_time, batch.send_time, batch.send_employee_total, batch.send_contact_total, batch.send_total,
		       batch.not_send_total, batch.received_total, batch.not_received_total, batch.receive_limit_total, batch.not_friend_total,
		       batch.send_status, batch.created_at
		FROM mc_contact_message_batch_send batch
		JOIN mc_corp corp ON corp.id=batch.corp_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY batch.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.ContactMessageBatchSendPage{}, err
	}
	defer rows.Close()
	items, err := scanContactMessageBatchSendRows(rows)
	if err != nil {
		return dashboard.ContactMessageBatchSendPage{}, err
	}
	return dashboard.ContactMessageBatchSendPage{Items: items, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) ContactMessageBatchSendByID(ctx context.Context, batchID int) (dashboard.ContactMessageBatchSendItem, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, user_id, medium_id, batch_title, user_name, employee_ids, filter_params, filter_params_detail, content,
		       send_way, definite_time, send_time, send_employee_total, send_contact_total, send_total,
		       not_send_total, received_total, not_received_total, receive_limit_total, not_friend_total,
		       send_status, created_at
		FROM mc_contact_message_batch_send
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, batchID)
	item, err := scanContactMessageBatchSendRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.ContactMessageBatchSendItem{}, false, nil
	}
	if err != nil {
		return dashboard.ContactMessageBatchSendItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) CreateContactMessageBatchSend(ctx context.Context, values dashboard.ContactMessageBatchSendWrite) (int, error) {
	var definite any
	if strings.TrimSpace(values.DefiniteTime) != "" {
		definite = values.DefiniteTime
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_contact_message_batch_send (
			corp_id, user_id, medium_id, user_name, batch_title, employee_ids, filter_params, filter_params_detail, content,
			send_way, definite_time, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, values.CorpID, values.UserID, values.MediumID, values.UserName, values.BatchTitle, mustJSONStore(values.EmployeeIDs), values.FilterParamsJSON, values.FilterDetailJSON, values.ContentJSON, values.SendWay, definite)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func (s *MySQLStore) CreateContactMessageBatchSendTasks(ctx context.Context, batchID int) ([]dashboard.ContactMessageBatchSendSendTarget, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer rollbackQuietly(tx)

	batch, found, err := contactMessageBatchSendByIDTx(ctx, tx, batchID)
	if err != nil {
		return nil, err
	}
	if !found {
		return []dashboard.ContactMessageBatchSendSendTarget{}, nil
	}
	var existing int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_contact_message_batch_send_employee
		WHERE batch_id = ?
	`, batchID).Scan(&existing); err != nil {
		return nil, err
	}
	if existing > 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return s.contactMessageBatchSendTargets(ctx, batchID)
	}

	employees, err := contactMessageBatchSendEmployeesByIDsTx(ctx, tx, batch.CorpID, batch.EmployeeIDs)
	if err != nil {
		return nil, err
	}
	targets := make([]dashboard.ContactMessageBatchSendSendTarget, 0, len(employees))
	contactTotal := 0
	for _, employee := range employees {
		contacts, err := contactMessageBatchSendContactsForEmployeeTx(ctx, tx, batch.CorpID, employee.ID, batch.FilterParams)
		if err != nil {
			return nil, err
		}
		contactTotal += len(contacts)
		target := dashboard.ContactMessageBatchSendSendTarget{
			EmployeeID:      employee.ID,
			WXUserID:        employee.WXUserID,
			ExternalUserIDs: make([]string, 0, len(contacts)),
			ContactIDs:      make([]int, 0, len(contacts)),
		}
		for _, contact := range contacts {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO mc_contact_message_batch_send_result (
					batch_id, employee_id, contact_id, external_user_id, created_at, updated_at
				)
				VALUES (?, ?, ?, ?, NOW(), NOW())
			`, batchID, employee.ID, contact.ID, contact.WXExternalUserID); err != nil {
				return nil, err
			}
			if contact.WXExternalUserID != "" {
				target.ExternalUserIDs = append(target.ExternalUserIDs, contact.WXExternalUserID)
			}
			target.ContactIDs = append(target.ContactIDs, contact.ID)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_contact_message_batch_send_employee (
				batch_id, employee_id, wx_user_id, send_contact_total, created_at, updated_at, last_sync_time
			)
			VALUES (?, ?, ?, ?, NOW(), NOW(), NOW())
		`, batchID, employee.ID, employee.WXUserID, len(contacts)); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_contact_message_batch_send
		SET send_employee_total = ?, not_send_total = ?, send_contact_total = ?, not_received_total = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, len(employees), len(employees), contactTotal, contactTotal, batchID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return targets, nil
}

func (s *MySQLStore) MarkContactMessageBatchSendSubmitted(ctx context.Context, batchID int, results []dashboard.ContactMessageBatchSendMessageResult) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	for _, result := range results {
		if result.EmployeeID <= 0 {
			continue
		}
		receiveStatus := 2
		sendStatus := 2
		if result.ErrCode == 0 {
			receiveStatus = 1
			sendStatus = 1
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_contact_message_batch_send_employee
			SET err_code = ?, err_msg = ?, msg_id = ?, status = ?, receive_status = ?, send_time = NOW(), updated_at = NOW()
			WHERE batch_id = ? AND employee_id = ?
		`, strconv.Itoa(result.ErrCode), result.ErrMsg, result.MsgID, sendStatus, receiveStatus, batchID, result.EmployeeID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_contact_message_batch_send
		SET send_status = 1, send_time = NOW(), updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, batchID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) ContactMessageBatchSendEmployeePage(ctx context.Context, filter dashboard.ContactMessageBatchSendEmployeeFilter) (dashboard.ContactMessageBatchSendEmployeePage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 15
	}
	where := []string{"a.batch_id = ?"}
	args := []any{filter.BatchID}
	if filter.SendStatus != nil {
		where = append(where, "a.status = ?")
		args = append(args, *filter.SendStatus)
	}
	if filter.KeyWords != "" {
		where = append(where, "e.name LIKE ?")
		args = append(args, "%"+filter.KeyWords+"%")
	}
	from := `
		FROM mc_contact_message_batch_send_employee AS a
		INNER JOIN mc_work_employee AS e ON a.employee_id = e.id AND e.deleted_at IS NULL
		WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) `+from, args...).Scan(&total); err != nil {
		return dashboard.ContactMessageBatchSendEmployeePage{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.status, a.send_time, a.send_contact_total,
		       e.id, e.name, e.alias, e.avatar, e.thumb_avatar
		`+from+`
		ORDER BY a.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.ContactMessageBatchSendEmployeePage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.ContactMessageBatchSendEmployeeItem, 0)
	for rows.Next() {
		var item dashboard.ContactMessageBatchSendEmployeeItem
		var sendTime sql.NullTime
		var name, alias, avatar, thumbAvatar sql.NullString
		if err := rows.Scan(&item.ID, &item.Status, &sendTime, &item.SendContactTotal, &item.EmployeeID, &name, &alias, &avatar, &thumbAvatar); err != nil {
			return dashboard.ContactMessageBatchSendEmployeePage{}, err
		}
		item.SendTime = formatTime(sendTime)
		item.EmployeeName = nullString(name)
		item.EmployeeAlias = nullString(alias)
		item.EmployeeAvatar = nullString(avatar)
		item.EmployeeThumbAvatar = nullString(thumbAvatar)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ContactMessageBatchSendEmployeePage{}, err
	}
	return dashboard.ContactMessageBatchSendEmployeePage{Items: items, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) ContactMessageBatchSendReceivePage(ctx context.Context, filter dashboard.ContactMessageBatchSendReceiveFilter) (dashboard.ContactMessageBatchSendReceivePage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 15
	}
	where := []string{"a.batch_id = ?"}
	args := []any{filter.BatchID}
	if filter.SendStatus != nil {
		where = append(where, "a.status = ?")
		args = append(args, *filter.SendStatus)
	}
	if filter.KeyWords != "" {
		where = append(where, "(c.name LIKE ? OR c.nick_name LIKE ?)")
		args = append(args, "%"+filter.KeyWords+"%", "%"+filter.KeyWords+"%")
	}
	from := `
		FROM mc_contact_message_batch_send_result AS a
		INNER JOIN mc_work_contact AS c ON a.contact_id = c.id AND c.deleted_at IS NULL
		INNER JOIN mc_work_employee AS e ON a.employee_id = e.id AND e.deleted_at IS NULL
		WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) `+from, args...).Scan(&total); err != nil {
		return dashboard.ContactMessageBatchSendReceivePage{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.status, a.send_time,
		       c.id, c.name, c.nick_name, c.avatar,
		       e.id, e.name, e.alias
		`+from+`
		ORDER BY a.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.ContactMessageBatchSendReceivePage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.ContactMessageBatchSendReceiveItem, 0)
	for rows.Next() {
		var item dashboard.ContactMessageBatchSendReceiveItem
		var contactName, contactNickName, contactAvatar, employeeName, employeeAlias sql.NullString
		if err := rows.Scan(&item.ID, &item.Status, &item.SendTime, &item.ContactID, &contactName, &contactNickName, &contactAvatar, &item.EmployeeID, &employeeName, &employeeAlias); err != nil {
			return dashboard.ContactMessageBatchSendReceivePage{}, err
		}
		item.ContactName = nullString(contactName)
		item.ContactNickName = nullString(contactNickName)
		item.ContactAvatar = nullString(contactAvatar)
		item.EmployeeName = nullString(employeeName)
		item.EmployeeAlias = nullString(employeeAlias)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ContactMessageBatchSendReceivePage{}, err
	}
	return dashboard.ContactMessageBatchSendReceivePage{Items: items, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) DeleteContactMessageBatchSend(ctx context.Context, batchID int) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)
	reclaimPaths, tenantID, found, err := contactMessageBatchSendStorageForDelete(ctx, tx, batchID)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_contact_message_batch_send
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, batchID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM mc_contact_message_batch_send_employee WHERE batch_id = ?`, batchID); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM mc_contact_message_batch_send_result WHERE batch_id = ?`, batchID); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	if affected > 0 {
		if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
			return true, err
		}
		if tenantID > 0 {
			if err := s.RefreshSaaSUsageCounter(ctx, tenantID, dashboard.SaaSMetricContactMessageBatches); err != nil {
				return true, err
			}
		}
	}
	return affected > 0, nil
}

func contactMessageBatchSendStorageForDelete(ctx context.Context, tx *sql.Tx, batchID int) ([]string, int, bool, error) {
	var corpID int
	var contentRaw sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT corp_id, content
		FROM mc_contact_message_batch_send
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, batchID).Scan(&corpID, &contentRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	tenantID, err := tenantIDByCorpIDTx(ctx, tx, corpID)
	if err != nil {
		return nil, 0, false, err
	}
	return batchSendStoragePathsFromContent(nullString(contentRaw)), tenantID, true, nil
}

func (s *MySQLStore) ContactMessageBatchSendRoomInfo(ctx context.Context, roomID int) (dashboard.ContactMessageBatchSendRoomInfo, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT r.name, r.owner_id, e.name, e.avatar
		FROM mc_work_room AS r
		LEFT JOIN mc_work_employee AS e ON r.owner_id = e.id AND e.deleted_at IS NULL
		WHERE r.id = ? AND r.deleted_at IS NULL
		LIMIT 1
	`, roomID)
	var room dashboard.ContactMessageBatchSendRoomInfo
	var roomName, ownerName, ownerAvatar sql.NullString
	err := row.Scan(&roomName, &room.OwnerID, &ownerName, &ownerAvatar)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.ContactMessageBatchSendRoomInfo{}, false, nil
	}
	if err != nil {
		return dashboard.ContactMessageBatchSendRoomInfo{}, false, err
	}
	room.Name = nullString(roomName)
	room.OwnerName = nullString(ownerName)
	room.OwnerAvatar = nullString(ownerAvatar)
	count := func(query string, args ...any) (int, error) {
		var total int
		if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
			return 0, err
		}
		return total, nil
	}
	var countErr error
	if room.Total, countErr = count(`SELECT COUNT(*) FROM mc_work_contact_room WHERE room_id = ? AND deleted_at IS NULL`, roomID); countErr != nil {
		return dashboard.ContactMessageBatchSendRoomInfo{}, false, countErr
	}
	if room.TotalContact, countErr = count(`SELECT COUNT(*) FROM mc_work_contact_room WHERE room_id = ? AND type = 2 AND status = 1 AND deleted_at IS NULL`, roomID); countErr != nil {
		return dashboard.ContactMessageBatchSendRoomInfo{}, false, countErr
	}
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	if room.TodayInsert, countErr = count(`SELECT COUNT(*) FROM mc_work_contact_room WHERE room_id = ? AND join_time >= ? AND join_time < ? AND deleted_at IS NULL`, roomID, today, tomorrow); countErr != nil {
		return dashboard.ContactMessageBatchSendRoomInfo{}, false, countErr
	}
	if room.TodayLoss, countErr = count(`SELECT COUNT(*) FROM mc_work_contact_room WHERE room_id = ? AND out_time >= ? AND out_time < ? AND deleted_at IS NULL`, roomID, today, tomorrow); countErr != nil {
		return dashboard.ContactMessageBatchSendRoomInfo{}, false, countErr
	}
	return room, true, nil
}

func (s *MySQLStore) ContactMessageBatchSendEmployees(ctx context.Context, batchID int, batchEmployeeID int) ([]dashboard.ContactMessageBatchSendEmployeeRef, error) {
	where := []string{"batch_id = ?"}
	args := []any{batchID}
	if batchEmployeeID > 0 {
		where = append(where, "id = ?")
		args = append(args, batchEmployeeID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT employee_id, wx_user_id
		FROM mc_contact_message_batch_send_employee
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.ContactMessageBatchSendEmployeeRef, 0)
	for rows.Next() {
		var item dashboard.ContactMessageBatchSendEmployeeRef
		var wxUserID sql.NullString
		if err := rows.Scan(&item.ID, &wxUserID); err != nil {
			return nil, err
		}
		item.WXUserID = nullString(wxUserID)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) ContactMessageBatchSendEmployeeWXUserID(ctx context.Context, employeeID int) (string, bool, error) {
	var wxUserID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT wx_user_id
		FROM mc_work_employee
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, employeeID).Scan(&wxUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return nullString(wxUserID), true, nil
}

func (s *MySQLStore) ContactMessageBatchSendFilterDetail(ctx context.Context, params dashboard.ContactMessageBatchSendFilterParams) (dashboard.ContactMessageBatchSendFilterDetail, error) {
	detail := dashboard.ContactMessageBatchSendFilterDetail{
		Gender:          params.Gender,
		AddTimeStart:    params.AddTimeStart,
		AddTimeEnd:      params.AddTimeEnd,
		Rooms:           []dashboard.ContactMessageBatchSendNameID{},
		Tags:            []dashboard.ContactMessageBatchSendNameID{},
		ExcludeContacts: []dashboard.ContactMessageBatchSendContactBase{},
	}
	rooms, err := contactMessageBatchSendNamesByIDs(ctx, s.db, "mc_work_room", params.Rooms)
	if err != nil {
		return dashboard.ContactMessageBatchSendFilterDetail{}, err
	}
	tags, err := contactMessageBatchSendNamesByIDs(ctx, s.db, "mc_work_contact_tag", params.Tags)
	if err != nil {
		return dashboard.ContactMessageBatchSendFilterDetail{}, err
	}
	detail.Rooms = rooms
	detail.Tags = tags
	return detail, nil
}

func (s *MySQLStore) DueContactMessageBatchSendIDs(ctx context.Context, now time.Time) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT batch.id
		FROM mc_contact_message_batch_send batch
		WHERE batch.send_way = 2
		  AND batch.send_status = 0
		  AND batch.definite_time IS NOT NULL
		  AND batch.definite_time <= ?
		  AND batch.deleted_at IS NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM mochat_go_wecom_capability_operations operation
			WHERE operation.tenant_id = batch.tenant_id
			  AND operation.corp_id = batch.corp_id
			  AND operation.capability = 'contact_batch_send'
			  AND operation.request_id = CONCAT('contact-batch:', batch.id)
		  )
		ORDER BY batch.definite_time ASC, batch.id ASC
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) ContactBatchSendEmployeeIDsForSync(ctx context.Context, since time.Time) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM mc_contact_message_batch_send_employee
		WHERE receive_status = 1 AND created_at >= ?
		ORDER BY id ASC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) ContactBatchSendSyncTarget(ctx context.Context, batchEmployeeID int) (dashboard.ContactBatchSendSyncTarget, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT e.id, e.batch_id, e.status, e.msg_id,
		       b.corp_id, b.send_employee_total, b.send_contact_total
		FROM mc_contact_message_batch_send_employee AS e
		INNER JOIN mc_contact_message_batch_send AS b ON b.id = e.batch_id AND b.deleted_at IS NULL
		INNER JOIN mc_corp AS c ON c.id = b.corp_id AND c.deleted_at IS NULL
		WHERE e.id = ?
		LIMIT 1
	`, batchEmployeeID)
	var target dashboard.ContactBatchSendSyncTarget
	var msgID sql.NullString
	err := row.Scan(
		&target.ID, &target.BatchID, &target.Status, &msgID,
		&target.Credential.CorpID, &target.SendEmployeeTotal, &target.SendContactTotal,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.ContactBatchSendSyncTarget{}, false, nil
	}
	if err != nil {
		return dashboard.ContactBatchSendSyncTarget{}, false, err
	}
	target.MsgID = nullString(msgID)
	credential, found, err := s.RoomWelcomeCorpCredentialByID(ctx, target.Credential.CorpID)
	if err != nil {
		return dashboard.ContactBatchSendSyncTarget{}, false, err
	}
	if !found {
		return dashboard.ContactBatchSendSyncTarget{}, false, nil
	}
	target.Credential = credential
	return target, true, nil
}

func (s *MySQLStore) UpdateContactBatchSendEmployeeSent(ctx context.Context, batchEmployeeID int, errCode int, errMsg string, sendTime int64) error {
	var sendTimeArg any
	if sendTime > 0 {
		sendTimeArg = sendTime
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_contact_message_batch_send_employee
		SET err_code = ?, err_msg = ?, status = 1, send_time = FROM_UNIXTIME(?), updated_at = NOW()
		WHERE id = ?
	`, strconv.Itoa(errCode), errMsg, sendTimeArg, batchEmployeeID)
	return err
}

func (s *MySQLStore) UpdateContactBatchSendResult(ctx context.Context, batchID int, externalUserID string, userID string, status int, sendTime int64) error {
	if sendTime < 0 {
		sendTime = 0
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_contact_message_batch_send_result
		SET user_id = ?, status = ?, send_time = ?, updated_at = NOW()
		WHERE batch_id = ? AND external_user_id = ?
	`, userID, status, sendTime, batchID, externalUserID)
	return err
}

func (s *MySQLStore) RefreshContactBatchSendTotals(ctx context.Context, batchID int, sendEmployeeTotal int, sendContactTotal int) error {
	sendTotal, err := s.countRows(ctx, `SELECT COUNT(*) FROM mc_contact_message_batch_send_employee WHERE batch_id = ? AND status = 1`, batchID)
	if err != nil {
		return err
	}
	receivedTotal, err := s.countRows(ctx, `SELECT COUNT(*) FROM mc_contact_message_batch_send_result WHERE batch_id = ? AND status = 1`, batchID)
	if err != nil {
		return err
	}
	receiveLimitTotal, err := s.countRows(ctx, `SELECT COUNT(*) FROM mc_contact_message_batch_send_result WHERE batch_id = ? AND status = 2`, batchID)
	if err != nil {
		return err
	}
	notFriendTotal, err := s.countRows(ctx, `SELECT COUNT(*) FROM mc_contact_message_batch_send_result WHERE batch_id = ? AND status = 3`, batchID)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE mc_contact_message_batch_send
		SET send_total = ?, not_send_total = ?, received_total = ?, not_received_total = ?,
		    receive_limit_total = ?, not_friend_total = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, sendTotal, nonNegative(sendEmployeeTotal-sendTotal), receivedTotal, nonNegative(sendContactTotal-receivedTotal), receiveLimitTotal, notFriendTotal, batchID)
	return err
}

type contactMessageBatchSendScanner interface {
	Scan(dest ...any) error
}

func scanContactMessageBatchSendRows(rows *sql.Rows) ([]dashboard.ContactMessageBatchSendItem, error) {
	items := make([]dashboard.ContactMessageBatchSendItem, 0)
	for rows.Next() {
		item, err := scanContactMessageBatchSendRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanContactMessageBatchSendRow(scanner contactMessageBatchSendScanner) (dashboard.ContactMessageBatchSendItem, error) {
	var item dashboard.ContactMessageBatchSendItem
	var batchTitle, userName, employeeIDsRaw, filterRaw, detailRaw, contentRaw sql.NullString
	var definiteTime, sendTime, createdAt sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.CorpID, &item.UserID, &item.MediumID, &batchTitle, &userName, &employeeIDsRaw, &filterRaw, &detailRaw, &contentRaw,
		&item.SendWay, &definiteTime, &sendTime, &item.SendEmployeeTotal, &item.SendContactTotal, &item.SendTotal,
		&item.NotSendTotal, &item.ReceivedTotal, &item.NotReceivedTotal, &item.ReceiveLimitTotal, &item.NotFriendTotal,
		&item.SendStatus, &createdAt,
	); err != nil {
		return dashboard.ContactMessageBatchSendItem{}, err
	}
	item.BatchTitle = nullString(batchTitle)
	item.UserName = nullString(userName)
	item.EmployeeIDs = parseContactMessageBatchSendIntSlice(nullString(employeeIDsRaw))
	item.FilterParamsRaw = nullString(filterRaw)
	item.FilterParams = parseContactMessageBatchSendFilterParams(nullString(filterRaw))
	item.FilterParamsDetail = parseContactMessageBatchSendFilterDetail(nullString(detailRaw))
	item.Content = parseContactMessageBatchSendContent(nullString(contentRaw))
	item.DefiniteTime = formatTime(definiteTime)
	item.SendTime = formatTime(sendTime)
	item.CreatedAt = formatTime(createdAt)
	return item, nil
}

type contactMessageBatchSendEmployeeRow struct {
	ID       int
	WXUserID string
}

type contactMessageBatchSendContactRow struct {
	ID               int
	WXExternalUserID string
}

func contactMessageBatchSendByIDTx(ctx context.Context, tx *sql.Tx, batchID int) (dashboard.ContactMessageBatchSendItem, bool, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, corp_id, user_id, medium_id, batch_title, user_name, employee_ids, filter_params, filter_params_detail, content,
		       send_way, definite_time, send_time, send_employee_total, send_contact_total, send_total,
		       not_send_total, received_total, not_received_total, receive_limit_total, not_friend_total,
		       send_status, created_at
		FROM mc_contact_message_batch_send
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, batchID)
	item, err := scanContactMessageBatchSendRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.ContactMessageBatchSendItem{}, false, nil
	}
	if err != nil {
		return dashboard.ContactMessageBatchSendItem{}, false, err
	}
	return item, true, nil
}

func contactMessageBatchSendEmployeesByIDsTx(ctx context.Context, tx *sql.Tx, corpID int, employeeIDs []int) ([]contactMessageBatchSendEmployeeRow, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return []contactMessageBatchSendEmployeeRow{}, nil
	}
	args := []any{corpID}
	for _, id := range employeeIDs {
		args = append(args, id)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, wx_user_id
		FROM mc_work_employee
		WHERE corp_id = ? AND id IN (`+placeholders(len(employeeIDs))+`) AND status = 1 AND deleted_at IS NULL
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]contactMessageBatchSendEmployeeRow, 0)
	for rows.Next() {
		var item contactMessageBatchSendEmployeeRow
		var wxUserID sql.NullString
		if err := rows.Scan(&item.ID, &wxUserID); err != nil {
			return nil, err
		}
		item.WXUserID = nullString(wxUserID)
		items = append(items, item)
	}
	return items, rows.Err()
}

func contactMessageBatchSendContactsForEmployeeTx(ctx context.Context, tx *sql.Tx, corpID int, employeeID int, filter dashboard.ContactMessageBatchSendFilterParams) ([]contactMessageBatchSendContactRow, error) {
	where := []string{"wce.employee_id = ?", "wce.corp_id = ?", "wce.deleted_at IS NULL", "wc.deleted_at IS NULL"}
	args := []any{employeeID, corpID}
	joins := []string{"INNER JOIN mc_work_contact AS wc ON wce.contact_id = wc.id"}
	if filter.Gender != nil && *filter.Gender < 3 {
		where = append(where, "wc.gender = ?")
		args = append(args, *filter.Gender)
	}
	if filter.AddTimeStart != "" {
		where = append(where, "wc.created_at >= ?")
		args = append(args, filter.AddTimeStart+" 00:00:00")
	}
	if filter.AddTimeEnd != "" {
		where = append(where, "wc.created_at <= ?")
		args = append(args, filter.AddTimeEnd+" 23:59:59")
	}
	filter.Rooms = uniquePositiveInts(filter.Rooms)
	if len(filter.Rooms) > 0 {
		joins = append(joins, "INNER JOIN mc_work_contact_room AS wcr ON wce.contact_id = wcr.contact_id AND wcr.deleted_at IS NULL")
		where = append(where, "wcr.room_id IN ("+placeholders(len(filter.Rooms))+")")
		for _, id := range filter.Rooms {
			args = append(args, id)
		}
	}
	filter.Tags = uniquePositiveInts(filter.Tags)
	filter.ExcludeContacts = uniquePositiveInts(filter.ExcludeContacts)
	if len(filter.Tags) > 0 || len(filter.ExcludeContacts) > 0 {
		joins = append(joins, "INNER JOIN mc_work_contact_tag_pivot AS wctp ON wce.contact_id = wctp.contact_id AND wctp.deleted_at IS NULL")
	}
	if len(filter.Tags) > 0 {
		where = append(where, "wctp.contact_tag_id IN ("+placeholders(len(filter.Tags))+")")
		for _, id := range filter.Tags {
			args = append(args, id)
		}
	}
	if len(filter.ExcludeContacts) > 0 {
		where = append(where, "wctp.contact_tag_id NOT IN ("+placeholders(len(filter.ExcludeContacts))+")")
		for _, id := range filter.ExcludeContacts {
			args = append(args, id)
		}
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT wc.id, wc.wx_external_userid
		FROM mc_work_contact_employee AS wce
		`+strings.Join(joins, "\n")+`
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY wc.id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	contacts := make([]contactMessageBatchSendContactRow, 0)
	for rows.Next() {
		var contact contactMessageBatchSendContactRow
		var externalUserID sql.NullString
		if err := rows.Scan(&contact.ID, &externalUserID); err != nil {
			return nil, err
		}
		contact.WXExternalUserID = nullString(externalUserID)
		contacts = append(contacts, contact)
	}
	return contacts, rows.Err()
}

func (s *MySQLStore) contactMessageBatchSendTargets(ctx context.Context, batchID int) ([]dashboard.ContactMessageBatchSendSendTarget, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.employee_id, e.wx_user_id, r.contact_id, r.external_user_id
		FROM mc_contact_message_batch_send_employee AS e
		LEFT JOIN mc_contact_message_batch_send_result AS r ON e.batch_id = r.batch_id AND e.employee_id = r.employee_id
		WHERE e.batch_id = ?
		ORDER BY e.id ASC, r.id ASC
	`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targetByEmployee := map[int]*dashboard.ContactMessageBatchSendSendTarget{}
	order := []int{}
	for rows.Next() {
		var employeeID int
		var wxUserID, externalUserID sql.NullString
		var contactID sql.NullInt64
		if err := rows.Scan(&employeeID, &wxUserID, &contactID, &externalUserID); err != nil {
			return nil, err
		}
		target, exists := targetByEmployee[employeeID]
		if !exists {
			target = &dashboard.ContactMessageBatchSendSendTarget{EmployeeID: employeeID, WXUserID: nullString(wxUserID)}
			targetByEmployee[employeeID] = target
			order = append(order, employeeID)
		}
		if contactID.Valid {
			target.ContactIDs = append(target.ContactIDs, int(contactID.Int64))
		}
		if value := nullString(externalUserID); value != "" {
			target.ExternalUserIDs = append(target.ExternalUserIDs, value)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	targets := make([]dashboard.ContactMessageBatchSendSendTarget, 0, len(order))
	for _, employeeID := range order {
		targets = append(targets, *targetByEmployee[employeeID])
	}
	return targets, nil
}

func contactMessageBatchSendNamesByIDs(ctx context.Context, db *sql.DB, table string, ids []int) ([]dashboard.ContactMessageBatchSendNameID, error) {
	ids = uniquePositiveInts(ids)
	if len(ids) == 0 {
		return []dashboard.ContactMessageBatchSendNameID{}, nil
	}
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, name
		FROM `+table+`
		WHERE id IN (`+placeholders(len(ids))+`) AND deleted_at IS NULL
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.ContactMessageBatchSendNameID, 0)
	for rows.Next() {
		var item dashboard.ContactMessageBatchSendNameID
		var name sql.NullString
		if err := rows.Scan(&item.ID, &name); err != nil {
			return nil, err
		}
		item.Name = nullString(name)
		items = append(items, item)
	}
	return items, rows.Err()
}

func parseContactMessageBatchSendIntSlice(raw string) []int {
	if strings.TrimSpace(raw) == "" {
		return []int{}
	}
	var ints []int
	if err := json.Unmarshal([]byte(raw), &ints); err == nil {
		return uniquePositiveInts(ints)
	}
	var values []any
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return []int{}
	}
	ints = make([]int, 0, len(values))
	for _, value := range values {
		ints = append(ints, intFromAny(value))
	}
	return uniquePositiveInts(ints)
}

func parseContactMessageBatchSendFilterParams(raw string) dashboard.ContactMessageBatchSendFilterParams {
	var payload map[string]any
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &payload) != nil {
		return dashboard.ContactMessageBatchSendFilterParams{}
	}
	var params dashboard.ContactMessageBatchSendFilterParams
	if value, ok := payload["gender"]; ok {
		gender := intFromAny(value)
		params.Gender = &gender
	}
	params.AddTimeStart = roomTagPullStringFromAny(payload["addTimeStart"])
	params.AddTimeEnd = roomTagPullStringFromAny(payload["addTimeEnd"])
	params.Rooms = intsFromAny(payload["rooms"])
	params.Tags = intsFromAny(payload["tags"])
	params.ExcludeContacts = intsFromAny(payload["excludeContacts"])
	return params
}

func parseContactMessageBatchSendFilterDetail(raw string) dashboard.ContactMessageBatchSendFilterDetail {
	detail := dashboard.ContactMessageBatchSendFilterDetail{
		Rooms:           []dashboard.ContactMessageBatchSendNameID{},
		Tags:            []dashboard.ContactMessageBatchSendNameID{},
		ExcludeContacts: []dashboard.ContactMessageBatchSendContactBase{},
	}
	if strings.TrimSpace(raw) == "" {
		return detail
	}
	_ = json.Unmarshal([]byte(raw), &detail)
	if detail.Rooms == nil {
		detail.Rooms = []dashboard.ContactMessageBatchSendNameID{}
	}
	if detail.Tags == nil {
		detail.Tags = []dashboard.ContactMessageBatchSendNameID{}
	}
	if detail.ExcludeContacts == nil {
		detail.ExcludeContacts = []dashboard.ContactMessageBatchSendContactBase{}
	}
	return detail
}

func parseContactMessageBatchSendContent(raw string) []dashboard.ContactMessageBatchSendContent {
	if strings.TrimSpace(raw) == "" {
		return []dashboard.ContactMessageBatchSendContent{}
	}
	var content []dashboard.ContactMessageBatchSendContent
	if err := json.Unmarshal([]byte(raw), &content); err == nil {
		return content
	}
	var maps []map[string]any
	if err := json.Unmarshal([]byte(raw), &maps); err != nil {
		return []dashboard.ContactMessageBatchSendContent{}
	}
	content = make([]dashboard.ContactMessageBatchSendContent, 0, len(maps))
	for _, item := range maps {
		content = append(content, dashboard.ContactMessageBatchSendContent{
			MsgType:    roomTagPullStringFromAny(item["msgType"]),
			Content:    roomTagPullStringFromAny(item["content"]),
			MediaID:    roomTagPullStringFromAny(item["media_id"]),
			PicURL:     roomTagPullStringFromAny(item["pic_url"]),
			Title:      roomTagPullStringFromAny(item["title"]),
			Desc:       roomTagPullStringFromAny(item["desc"]),
			URL:        roomTagPullStringFromAny(item["url"]),
			AppID:      roomTagPullStringFromAny(item["appid"]),
			Page:       roomTagPullStringFromAny(item["page"]),
			PicMediaID: roomTagPullStringFromAny(item["pic_media_id"]),
		})
	}
	return content
}

func (s *MySQLStore) RoomMessageBatchSendPage(ctx context.Context, filter dashboard.RoomMessageBatchSendFilter) (dashboard.RoomMessageBatchSendPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10
	}
	where := []string{"user_id = ?", "deleted_at IS NULL"}
	args := []any{filter.UserID}
	if filter.RestrictEmployeeIDs {
		ids := uniquePositiveInts(filter.AllowedEmployeeIDs)
		if len(ids) == 0 {
			return dashboard.RoomMessageBatchSendPage{}, nil
		}
		parts := make([]string, 0, len(ids))
		for _, id := range ids {
			parts = append(parts, "JSON_CONTAINS(employee_ids, JSON_ARRAY(?))")
			args = append(args, id)
		}
		where = append(where, "("+strings.Join(parts, " OR ")+")")
	}
	if filter.BatchTitle != "" {
		where = append(where, "batch_title LIKE ?")
		args = append(args, "%"+filter.BatchTitle+"%")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_room_message_batch_send
		WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return dashboard.RoomMessageBatchSendPage{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, corp_id, user_id, medium_id, user_name, employee_ids, batch_title, content,
		       send_way, definite_time, send_time, send_room_total, send_employee_total, send_total,
		       not_send_total, received_total, not_received_total, send_status, created_at
		FROM mc_room_message_batch_send
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomMessageBatchSendPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomMessageBatchSendItem, 0)
	for rows.Next() {
		item, err := scanRoomMessageBatchSendRow(rows)
		if err != nil {
			return dashboard.RoomMessageBatchSendPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomMessageBatchSendPage{}, err
	}
	return dashboard.RoomMessageBatchSendPage{Items: items, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) RoomMessageBatchSendByID(ctx context.Context, batchID int) (dashboard.RoomMessageBatchSendItem, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, user_id, medium_id, user_name, employee_ids, batch_title, content,
		       send_way, definite_time, send_time, send_room_total, send_employee_total, send_total,
		       not_send_total, received_total, not_received_total, send_status, created_at
		FROM mc_room_message_batch_send
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, batchID)
	item, err := scanRoomMessageBatchSendRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.RoomMessageBatchSendItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomMessageBatchSendItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) RoomMessageBatchSendSeedRooms(ctx context.Context, batchID int, limit int) ([]dashboard.ContactMessageBatchSendNameID, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT r.id, r.name
		FROM mc_room_message_batch_send_result AS result
		INNER JOIN mc_work_room AS r ON result.room_id = r.id AND r.deleted_at IS NULL
		WHERE result.batch_id = ?
		ORDER BY result.id ASC
		LIMIT ?
	`, batchID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.ContactMessageBatchSendNameID, 0)
	for rows.Next() {
		var item dashboard.ContactMessageBatchSendNameID
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) RoomMessageBatchSendValidateEmployees(ctx context.Context, corpID int, employeeIDs []int) (bool, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return false, nil
	}
	args := make([]any, 0, len(employeeIDs)+1)
	args = append(args, corpID)
	for _, id := range employeeIDs {
		args = append(args, id)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_work_employee
		WHERE corp_id = ? AND status = 1 AND deleted_at IS NULL AND id IN (`+placeholders(len(employeeIDs))+`)
	`, args...).Scan(&total); err != nil {
		return false, err
	}
	return total == len(employeeIDs), nil
}

func (s *MySQLStore) CreateRoomMessageBatchSend(ctx context.Context, values dashboard.RoomMessageBatchSendWrite) (int, error) {
	var definite any
	if strings.TrimSpace(values.DefiniteTime) != "" {
		definite = values.DefiniteTime
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_room_message_batch_send (
			corp_id, user_id, medium_id, user_name, employee_ids, batch_title, content, send_way, definite_time, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, values.CorpID, values.UserID, values.MediumID, values.UserName, mustJSONStore(values.EmployeeIDs), values.BatchTitle, values.ContentJSON, values.SendWay, definite)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func (s *MySQLStore) CreateRoomMessageBatchSendTasks(ctx context.Context, batchID int) ([]dashboard.RoomMessageBatchSendTarget, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer rollbackQuietly(tx)

	batch, found, err := roomMessageBatchSendByIDTx(ctx, tx, batchID)
	if err != nil {
		return nil, err
	}
	if !found {
		return []dashboard.RoomMessageBatchSendTarget{}, nil
	}
	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_room_message_batch_send_employee WHERE batch_id = ?`, batchID).Scan(&existing); err != nil {
		return nil, err
	}
	if existing > 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return s.roomMessageBatchSendTargets(ctx, batchID)
	}
	employees, err := roomMessageBatchSendEmployeesByIDsTx(ctx, tx, batch.CorpID, batch.EmployeeIDs)
	if err != nil {
		return nil, err
	}
	targets := make([]dashboard.RoomMessageBatchSendTarget, 0, len(employees))
	roomTotal := 0
	for _, employee := range employees {
		rooms, err := roomMessageBatchSendRoomsByOwnerTx(ctx, tx, batch.CorpID, employee.ID)
		if err != nil {
			return nil, err
		}
		roomTotal += len(rooms)
		target := dashboard.RoomMessageBatchSendTarget{EmployeeID: employee.ID, WXUserID: employee.WXUserID, ChatIDs: make([]string, 0, len(rooms))}
		for _, room := range rooms {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO mc_room_message_batch_send_result (
					batch_id, employee_id, room_id, room_name, room_employee_num, room_create_time, chat_id, created_at, updated_at
				)
				VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
			`, batchID, employee.ID, room.ID, room.Name, room.MemberNum, nullTimeArg(room.CreateTime), room.WXChatID); err != nil {
				return nil, err
			}
			if room.WXChatID != "" {
				target.ChatIDs = append(target.ChatIDs, room.WXChatID)
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_room_message_batch_send_employee (
				batch_id, employee_id, wx_user_id, send_room_total, created_at, updated_at, last_sync_time
			)
			VALUES (?, ?, ?, ?, NOW(), NOW(), NOW())
		`, batchID, employee.ID, employee.WXUserID, len(rooms)); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_room_message_batch_send
		SET send_employee_total = ?, not_send_total = ?, send_room_total = ?, not_received_total = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, len(employees), len(employees), roomTotal, roomTotal, batchID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return targets, nil
}

func (s *MySQLStore) MarkRoomMessageBatchSendSubmitted(ctx context.Context, batchID int, results []dashboard.RoomMessageBatchSendMessageResult) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	for _, result := range results {
		if result.EmployeeID <= 0 {
			continue
		}
		receiveStatus := 2
		sendStatus := 2
		if result.ErrCode == 0 {
			receiveStatus = 1
			sendStatus = 1
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_room_message_batch_send_employee
			SET err_code = ?, err_msg = ?, msg_id = ?, status = ?, receive_status = ?, send_time = NOW(), updated_at = NOW()
			WHERE batch_id = ? AND employee_id = ?
		`, strconv.Itoa(result.ErrCode), result.ErrMsg, result.MsgID, sendStatus, receiveStatus, batchID, result.EmployeeID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_room_message_batch_send
		SET send_status = 1, send_time = NOW(), updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, batchID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) RoomMessageBatchSendOwnerPage(ctx context.Context, filter dashboard.RoomMessageBatchSendOwnerFilter) (dashboard.RoomMessageBatchSendOwnerPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 15
	}
	where := []string{"a.batch_id = ?"}
	args := []any{filter.BatchID}
	if filter.SendStatus != nil {
		where = append(where, "a.status = ?")
		args = append(args, *filter.SendStatus)
	}
	from := `
		FROM mc_room_message_batch_send_employee AS a
		INNER JOIN mc_work_employee AS e ON a.employee_id = e.id AND e.deleted_at IS NULL
		WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) `+from, args...).Scan(&total); err != nil {
		return dashboard.RoomMessageBatchSendOwnerPage{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.status, a.send_time, a.send_room_total,
		       e.id, e.name, e.alias, e.avatar, e.thumb_avatar,
		       (
		         SELECT COUNT(*)
		         FROM mc_room_message_batch_send_result AS r
		         WHERE r.batch_id = a.batch_id AND r.employee_id = a.employee_id AND r.status = 1
		       ) AS send_success_total
		`+from+`
		ORDER BY a.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomMessageBatchSendOwnerPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomMessageBatchSendOwnerItem, 0)
	for rows.Next() {
		var item dashboard.RoomMessageBatchSendOwnerItem
		var sendTime sql.NullTime
		var name, alias, avatar, thumbAvatar sql.NullString
		if err := rows.Scan(&item.ID, &item.Status, &sendTime, &item.SendRoomTotal, &item.EmployeeID, &name, &alias, &avatar, &thumbAvatar, &item.SendSuccessTotal); err != nil {
			return dashboard.RoomMessageBatchSendOwnerPage{}, err
		}
		item.SendTime = formatTime(sendTime)
		item.EmployeeName = nullString(name)
		item.EmployeeAlias = nullString(alias)
		item.EmployeeAvatar = nullString(avatar)
		item.EmployeeThumbAvatar = nullString(thumbAvatar)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomMessageBatchSendOwnerPage{}, err
	}
	return dashboard.RoomMessageBatchSendOwnerPage{Items: items, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) RoomMessageBatchSendRoomPage(ctx context.Context, filter dashboard.RoomMessageBatchSendRoomFilter) (dashboard.RoomMessageBatchSendRoomPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 15
	}
	where := []string{"a.batch_id = ?"}
	args := []any{filter.BatchID}
	if filter.SendStatus != nil {
		where = append(where, "a.status = ?")
		args = append(args, *filter.SendStatus)
	}
	if filter.KeyWords != "" {
		where = append(where, "a.room_name LIKE ?")
		args = append(args, "%"+filter.KeyWords+"%")
	}
	from := `
		FROM mc_room_message_batch_send_result AS a
		INNER JOIN mc_work_employee AS e ON a.employee_id = e.id AND e.deleted_at IS NULL
		WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) `+from, args...).Scan(&total); err != nil {
		return dashboard.RoomMessageBatchSendRoomPage{}, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.status, a.send_time, a.room_id, a.room_name, a.room_create_time, a.room_employee_num,
		       e.id, e.name, e.alias
		`+from+`
		ORDER BY a.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomMessageBatchSendRoomPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomMessageBatchSendRoomItem, 0)
	for rows.Next() {
		var item dashboard.RoomMessageBatchSendRoomItem
		var roomName, employeeName, employeeAlias sql.NullString
		var roomCreateTime sql.NullTime
		if err := rows.Scan(&item.ID, &item.Status, &item.SendTime, &item.RoomID, &roomName, &roomCreateTime, &item.RoomEmployeeNum, &item.EmployeeID, &employeeName, &employeeAlias); err != nil {
			return dashboard.RoomMessageBatchSendRoomPage{}, err
		}
		item.RoomName = nullString(roomName)
		item.RoomCreateTime = formatTime(roomCreateTime)
		item.EmployeeName = nullString(employeeName)
		item.EmployeeAlias = nullString(employeeAlias)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomMessageBatchSendRoomPage{}, err
	}
	return dashboard.RoomMessageBatchSendRoomPage{Items: items, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) DeleteRoomMessageBatchSend(ctx context.Context, batchID int) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)
	reclaimPaths, tenantID, found, err := roomMessageBatchSendStorageForDelete(ctx, tx, batchID)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_room_message_batch_send
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, batchID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM mc_room_message_batch_send_employee WHERE batch_id = ?`, batchID); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM mc_room_message_batch_send_result WHERE batch_id = ?`, batchID); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	if affected > 0 {
		if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
			return true, err
		}
		if tenantID > 0 {
			if err := s.RefreshSaaSUsageCounter(ctx, tenantID, dashboard.SaaSMetricRoomMessageBatches); err != nil {
				return true, err
			}
		}
	}
	return affected > 0, nil
}

func roomMessageBatchSendStorageForDelete(ctx context.Context, tx *sql.Tx, batchID int) ([]string, int, bool, error) {
	var corpID int
	var contentRaw sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT corp_id, content
		FROM mc_room_message_batch_send
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, batchID).Scan(&corpID, &contentRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	tenantID, err := tenantIDByCorpIDTx(ctx, tx, corpID)
	if err != nil {
		return nil, 0, false, err
	}
	return batchSendStoragePathsFromContent(nullString(contentRaw)), tenantID, true, nil
}

func (s *MySQLStore) RoomMessageBatchSendEmployees(ctx context.Context, batchID int) ([]dashboard.RoomMessageBatchSendEmployeeRef, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT employee_id, wx_user_id
		FROM mc_room_message_batch_send_employee
		WHERE batch_id = ?
		ORDER BY id ASC
	`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomMessageBatchSendEmployeeRef, 0)
	for rows.Next() {
		var item dashboard.RoomMessageBatchSendEmployeeRef
		var wxUserID sql.NullString
		if err := rows.Scan(&item.ID, &wxUserID); err != nil {
			return nil, err
		}
		item.WXUserID = nullString(wxUserID)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) RoomMessageBatchSendEmployeeWXUserID(ctx context.Context, employeeID int) (string, bool, error) {
	var wxUserID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT wx_user_id
		FROM mc_work_employee
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, employeeID).Scan(&wxUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return nullString(wxUserID), true, nil
}

func (s *MySQLStore) DueRoomMessageBatchSendIDs(ctx context.Context, now time.Time) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT batch.id
		FROM mc_room_message_batch_send batch
		WHERE batch.send_way = 2
		  AND batch.send_status = 0
		  AND batch.definite_time IS NOT NULL
		  AND batch.definite_time <= ?
		  AND batch.deleted_at IS NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM mochat_go_wecom_capability_operations operation
			WHERE operation.tenant_id = batch.tenant_id
			  AND operation.corp_id = batch.corp_id
			  AND operation.capability = 'room_batch_send'
			  AND operation.request_id = CONCAT('room-batch:', batch.id)
		  )
		ORDER BY batch.definite_time ASC, batch.id ASC
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) RoomBatchSendEmployeeIDsForSync(ctx context.Context, since time.Time) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM mc_room_message_batch_send_employee
		WHERE receive_status = 1 AND created_at >= ?
		ORDER BY id ASC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntColumn(rows)
}

func (s *MySQLStore) RoomBatchSendSyncTarget(ctx context.Context, batchEmployeeID int) (dashboard.RoomBatchSendSyncTarget, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT e.id, e.batch_id, e.status, e.msg_id,
		       b.corp_id, b.send_employee_total, b.send_room_total
		FROM mc_room_message_batch_send_employee AS e
		INNER JOIN mc_room_message_batch_send AS b ON b.id = e.batch_id AND b.deleted_at IS NULL
		INNER JOIN mc_corp AS c ON c.id = b.corp_id AND c.deleted_at IS NULL
		WHERE e.id = ?
		LIMIT 1
	`, batchEmployeeID)
	var target dashboard.RoomBatchSendSyncTarget
	var msgID sql.NullString
	err := row.Scan(
		&target.ID, &target.BatchID, &target.Status, &msgID,
		&target.Credential.CorpID, &target.SendEmployeeTotal, &target.SendRoomTotal,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.RoomBatchSendSyncTarget{}, false, nil
	}
	if err != nil {
		return dashboard.RoomBatchSendSyncTarget{}, false, err
	}
	target.MsgID = nullString(msgID)
	credential, found, err := s.RoomWelcomeCorpCredentialByID(ctx, target.Credential.CorpID)
	if err != nil {
		return dashboard.RoomBatchSendSyncTarget{}, false, err
	}
	if !found {
		return dashboard.RoomBatchSendSyncTarget{}, false, nil
	}
	target.Credential = credential
	return target, true, nil
}

func (s *MySQLStore) UpdateRoomBatchSendEmployeeSent(ctx context.Context, batchEmployeeID int, errCode int, errMsg string, sendTime int64) error {
	var sendTimeArg any
	if sendTime > 0 {
		sendTimeArg = sendTime
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_message_batch_send_employee
		SET err_code = ?, err_msg = ?, status = 1, send_time = FROM_UNIXTIME(?), updated_at = NOW()
		WHERE id = ?
	`, strconv.Itoa(errCode), errMsg, sendTimeArg, batchEmployeeID)
	return err
}

func (s *MySQLStore) UpdateRoomBatchSendResult(ctx context.Context, batchID int, chatID string, userID string, status int, sendTime int64) error {
	if sendTime < 0 {
		sendTime = 0
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_message_batch_send_result
		SET user_id = ?, status = ?, send_time = ?, updated_at = NOW()
		WHERE batch_id = ? AND chat_id = ?
	`, userID, status, sendTime, batchID, chatID)
	return err
}

func (s *MySQLStore) RefreshRoomBatchSendTotals(ctx context.Context, batchID int, sendEmployeeTotal int, sendRoomTotal int) error {
	sendTotal, err := s.countRows(ctx, `SELECT COUNT(*) FROM mc_room_message_batch_send_employee WHERE batch_id = ? AND status = 1`, batchID)
	if err != nil {
		return err
	}
	receivedTotal, err := s.countRows(ctx, `SELECT COUNT(*) FROM mc_room_message_batch_send_result WHERE batch_id = ? AND status = 1`, batchID)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE mc_room_message_batch_send
		SET send_total = ?, not_send_total = ?, received_total = ?, not_received_total = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, sendTotal, nonNegative(sendEmployeeTotal-sendTotal), receivedTotal, nonNegative(sendRoomTotal-receivedTotal), batchID)
	return err
}

type roomMessageBatchSendScanner interface {
	Scan(dest ...any) error
}

func scanRoomMessageBatchSendRow(scanner roomMessageBatchSendScanner) (dashboard.RoomMessageBatchSendItem, error) {
	var item dashboard.RoomMessageBatchSendItem
	var userName, employeeIDsRaw, batchTitle, contentRaw sql.NullString
	var definiteTime, sendTime, createdAt sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.CorpID, &item.UserID, &item.MediumID, &userName, &employeeIDsRaw, &batchTitle, &contentRaw,
		&item.SendWay, &definiteTime, &sendTime, &item.SendRoomTotal, &item.SendEmployeeTotal, &item.SendTotal,
		&item.NotSendTotal, &item.ReceivedTotal, &item.NotReceivedTotal, &item.SendStatus, &createdAt,
	); err != nil {
		return dashboard.RoomMessageBatchSendItem{}, err
	}
	item.UserName = nullString(userName)
	item.EmployeeIDs = parseContactMessageBatchSendIntSlice(nullString(employeeIDsRaw))
	item.BatchTitle = nullString(batchTitle)
	item.Content = parseContactMessageBatchSendContent(nullString(contentRaw))
	item.DefiniteTime = formatTime(definiteTime)
	item.SendTime = formatTime(sendTime)
	item.CreatedAt = formatTime(createdAt)
	return item, nil
}

func roomMessageBatchSendByIDTx(ctx context.Context, tx *sql.Tx, batchID int) (dashboard.RoomMessageBatchSendItem, bool, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, corp_id, user_id, medium_id, user_name, employee_ids, batch_title, content,
		       send_way, definite_time, send_time, send_room_total, send_employee_total, send_total,
		       not_send_total, received_total, not_received_total, send_status, created_at
		FROM mc_room_message_batch_send
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, batchID)
	item, err := scanRoomMessageBatchSendRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.RoomMessageBatchSendItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomMessageBatchSendItem{}, false, err
	}
	return item, true, nil
}

type roomMessageBatchSendEmployee struct {
	ID       int
	WXUserID string
}

func roomMessageBatchSendEmployeesByIDsTx(ctx context.Context, tx *sql.Tx, corpID int, employeeIDs []int) ([]roomMessageBatchSendEmployee, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return []roomMessageBatchSendEmployee{}, nil
	}
	args := make([]any, 0, len(employeeIDs)+1)
	args = append(args, corpID)
	for _, id := range employeeIDs {
		args = append(args, id)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, wx_user_id
		FROM mc_work_employee
		WHERE corp_id = ? AND status = 1 AND deleted_at IS NULL AND id IN (`+placeholders(len(employeeIDs))+`)
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]roomMessageBatchSendEmployee, 0)
	for rows.Next() {
		var item roomMessageBatchSendEmployee
		var wxUserID sql.NullString
		if err := rows.Scan(&item.ID, &wxUserID); err != nil {
			return nil, err
		}
		item.WXUserID = nullString(wxUserID)
		items = append(items, item)
	}
	return items, rows.Err()
}

type roomMessageBatchSendRoom struct {
	ID         int
	Name       string
	WXChatID   string
	CreateTime sql.NullTime
	MemberNum  int
}

func roomMessageBatchSendRoomsByOwnerTx(ctx context.Context, tx *sql.Tx, corpID int, employeeID int) ([]roomMessageBatchSendRoom, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT r.id, r.name, r.wx_chat_id, r.create_time,
		       (
		         SELECT COUNT(*)
		         FROM mc_work_contact_room AS member
		         WHERE member.room_id = r.id AND member.deleted_at IS NULL
		       ) AS member_num
		FROM mc_work_room AS r
		WHERE r.corp_id = ? AND r.owner_id = ? AND r.deleted_at IS NULL
		ORDER BY r.id ASC
	`, corpID, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]roomMessageBatchSendRoom, 0)
	for rows.Next() {
		var item roomMessageBatchSendRoom
		var name, wxChatID sql.NullString
		if err := rows.Scan(&item.ID, &name, &wxChatID, &item.CreateTime, &item.MemberNum); err != nil {
			return nil, err
		}
		item.Name = nullString(name)
		item.WXChatID = nullString(wxChatID)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) roomMessageBatchSendTargets(ctx context.Context, batchID int) ([]dashboard.RoomMessageBatchSendTarget, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.employee_id, e.wx_user_id, r.chat_id
		FROM mc_room_message_batch_send_employee AS e
		LEFT JOIN mc_room_message_batch_send_result AS r ON r.batch_id = e.batch_id AND r.employee_id = e.employee_id
		WHERE e.batch_id = ?
		ORDER BY e.id ASC, r.id ASC
	`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byEmployee := map[int]*dashboard.RoomMessageBatchSendTarget{}
	order := []int{}
	for rows.Next() {
		var employeeID int
		var wxUserID, chatID sql.NullString
		if err := rows.Scan(&employeeID, &wxUserID, &chatID); err != nil {
			return nil, err
		}
		target, ok := byEmployee[employeeID]
		if !ok {
			target = &dashboard.RoomMessageBatchSendTarget{EmployeeID: employeeID, WXUserID: nullString(wxUserID), ChatIDs: []string{}}
			byEmployee[employeeID] = target
			order = append(order, employeeID)
		}
		if value := nullString(chatID); value != "" {
			target.ChatIDs = append(target.ChatIDs, value)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	targets := make([]dashboard.RoomMessageBatchSendTarget, 0, len(order))
	for _, employeeID := range order {
		targets = append(targets, *byEmployee[employeeID])
	}
	return targets, nil
}

func (s *MySQLStore) WorkFissionPage(ctx context.Context, filter dashboard.WorkFissionListFilter) (dashboard.WorkFissionListPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10000
	}
	where := []string{"deleted_at IS NULL", "corp_id = ?"}
	args := []any{filter.CorpID}
	if strings.TrimSpace(filter.ActiveName) != "" {
		where = append(where, "active_name LIKE ?")
		args = append(args, "%"+strings.TrimSpace(filter.ActiveName)+"%")
	}
	if filter.RestrictCreateUser {
		where = append(where, "create_user_id = ?")
		args = append(args, filter.CreateUserID)
	}
	whereSQL := "WHERE " + strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_work_fission `+whereSQL, args...).Scan(&total); err != nil {
		return dashboard.WorkFissionListPage{}, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, active_name, service_employees, contact_tags, tasks, end_time, created_at,
		       (
		         SELECT COUNT(*)
		         FROM mc_work_fission_contact AS contact
		         WHERE contact.fission_id = mc_work_fission.id AND contact.deleted_at IS NULL
		       ) AS employee_num
		FROM mc_work_fission
		`+whereSQL+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.WorkFissionListPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.WorkFissionListItem, 0)
	for rows.Next() {
		var item dashboard.WorkFissionListItem
		var activeName, serviceEmployees, contactTags, tasks sql.NullString
		var endTime, createdAt sql.NullTime
		if err := rows.Scan(&item.ID, &activeName, &serviceEmployees, &contactTags, &tasks, &endTime, &createdAt, &item.EmployeeNum); err != nil {
			return dashboard.WorkFissionListPage{}, err
		}
		item.ActiveName = nullString(activeName)
		item.ServiceEmployees = nullString(serviceEmployees)
		item.ContactTags = nullString(contactTags)
		item.Tasks = nullString(tasks)
		item.EndTime = formatTime(endTime)
		item.CreatedAt = formatTime(createdAt)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkFissionListPage{}, err
	}
	return dashboard.WorkFissionListPage{
		Items:     items,
		Total:     total,
		TotalPage: pageCount(total, filter.PerPage),
		PerPage:   filter.PerPage,
	}, nil
}

func (s *MySQLStore) WorkFissionBundleByID(ctx context.Context, corpID int, id int) (dashboard.WorkFissionBundle, bool, error) {
	var bundle dashboard.WorkFissionBundle
	fission, found, err := s.workFissionInfoByID(ctx, corpID, id)
	if err != nil || !found {
		return dashboard.WorkFissionBundle{}, found, err
	}
	bundle.Fission = fission
	if poster, found, err := s.workFissionPosterByFissionID(ctx, id); err != nil {
		return dashboard.WorkFissionBundle{}, false, err
	} else if found {
		bundle.Poster = poster
	}
	if welcome, found, err := s.workFissionWelcomeByFissionID(ctx, id); err != nil {
		return dashboard.WorkFissionBundle{}, false, err
	} else if found {
		bundle.Welcome = welcome
	}
	if push, found, err := s.workFissionPushByFissionID(ctx, id); err != nil {
		return dashboard.WorkFissionBundle{}, false, err
	} else if found {
		bundle.Push = push
	}
	if invite, found, err := s.workFissionInviteByFissionID(ctx, id); err != nil {
		return dashboard.WorkFissionBundle{}, false, err
	} else if found {
		bundle.Invite = invite
	}
	return bundle, true, nil
}

func (s *MySQLStore) WorkFissionStatistics(ctx context.Context, corpID int, fissionIDs []int) (dashboard.WorkFissionStatistics, error) {
	stats := dashboard.WorkFissionStatistics{}
	active, err := s.workFissionNamesByCorpID(ctx, corpID)
	if err != nil {
		return dashboard.WorkFissionStatistics{}, err
	}
	stats.Active = active
	if len(fissionIDs) == 0 {
		return stats, nil
	}
	args := append([]any{corpID}, intsToArgs(fissionIDs)...)
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN contact.contact_superior_user_parent = 0 THEN 1 ELSE 0 END), 0),
			COUNT(*),
			COALESCE(SUM(CASE WHEN contact.loss = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN contact.created_at > CURDATE() THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN contact.is_new = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN contact.is_new = 1 AND contact.loss = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(contact.invite_count), 0),
			COALESCE(SUM(CASE WHEN contact.level = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN contact.level = 2 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN contact.level = 3 THEN 1 ELSE 0 END), 0)
		FROM mc_work_fission_contact AS contact
		INNER JOIN mc_work_fission AS fission
		  ON fission.id = contact.fission_id
		 AND fission.deleted_at IS NULL
		 AND fission.corp_id = ?
		WHERE contact.deleted_at IS NULL AND contact.fission_id IN (` + placeholders(len(fissionIDs)) + `)
	`
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.FirstUserCount,
		&stats.UserCount,
		&stats.LossCount,
		&stats.TodayIncreaseCount,
		&stats.NewIncreaseCount,
		&stats.NewLossCount,
		&stats.InviteCount,
		&stats.FirstLevelCount,
		&stats.SecondLevelCount,
		&stats.ThirdLevelCount,
	); err != nil {
		return dashboard.WorkFissionStatistics{}, err
	}
	serviceEmployees, err := s.workFissionServiceEmployees(ctx, corpID, fissionIDs)
	if err != nil {
		return dashboard.WorkFissionStatistics{}, err
	}
	stats.ServiceEmployees = serviceEmployees
	return stats, nil
}

func (s *MySQLStore) WorkFissionChooseContactCount(ctx context.Context, filter dashboard.WorkFissionChooseContactFilter) (int, error) {
	where := []string{"rel.corp_id = ?", "rel.deleted_at IS NULL", "contact.deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if len(filter.EmployeeIDs) > 0 {
		where = append(where, "rel.employee_id IN ("+placeholders(len(filter.EmployeeIDs))+")")
		args = append(args, intsToArgs(filter.EmployeeIDs)...)
	}
	if filter.IsAll == 1 && strings.TrimSpace(filter.StartTime) != "" {
		where = append(where, "rel.create_time > ?", "rel.create_time < ?")
		args = append(args, strings.TrimSpace(filter.StartTime), strings.TrimSpace(filter.EndTime))
	}
	if filter.Gender != nil {
		where = append(where, "contact.gender = ?")
		args = append(args, *filter.Gender)
	}
	query := `
		SELECT COUNT(DISTINCT contact.id)
		FROM mc_work_contact_employee AS rel
		INNER JOIN mc_work_contact AS contact ON contact.id = rel.contact_id
		WHERE ` + strings.Join(where, " AND ")
	var count int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *MySQLStore) WorkFissionInviteTargets(ctx context.Context, filter dashboard.WorkFissionChooseContactFilter) ([]string, error) {
	where := []string{"rel.corp_id = ?", "rel.deleted_at IS NULL", "contact.deleted_at IS NULL", "contact.wx_external_userid <> ''"}
	args := []any{filter.CorpID}
	if len(filter.EmployeeIDs) > 0 {
		where = append(where, "rel.employee_id IN ("+placeholders(len(filter.EmployeeIDs))+")")
		args = append(args, intsToArgs(filter.EmployeeIDs)...)
	}
	if filter.IsAll == 1 && strings.TrimSpace(filter.StartTime) != "" {
		where = append(where, "rel.create_time > ?", "rel.create_time < ?")
		args = append(args, strings.TrimSpace(filter.StartTime), strings.TrimSpace(filter.EndTime))
	}
	if filter.Gender != nil {
		where = append(where, "contact.gender = ?")
		args = append(args, *filter.Gender)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT contact.wx_external_userid
		FROM mc_work_contact_employee AS rel
		INNER JOIN mc_work_contact AS contact ON contact.id = rel.contact_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY contact.id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := []string{}
	for rows.Next() {
		var wxExternalUserID sql.NullString
		if err := rows.Scan(&wxExternalUserID); err != nil {
			return nil, err
		}
		if value := strings.TrimSpace(nullString(wxExternalUserID)); value != "" {
			targets = append(targets, value)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return targets, nil
}

func (s *MySQLStore) WorkFissionInviteDataPage(ctx context.Context, filter dashboard.WorkFissionInviteDataFilter) (dashboard.WorkFissionInviteDataPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10000
	}
	where := []string{"contact.deleted_at IS NULL", "contact.level > 0", "fission.deleted_at IS NULL", "fission.corp_id = ?"}
	args := []any{filter.CorpID}
	if len(filter.FissionIDs) > 0 {
		where = append(where, "contact.fission_id IN ("+placeholders(len(filter.FissionIDs))+")")
		args = append(args, intsToArgs(filter.FissionIDs)...)
	}
	if strings.TrimSpace(filter.Nickname) != "" {
		where = append(where, "contact.nickname LIKE ?")
		args = append(args, "%"+strings.TrimSpace(filter.Nickname)+"%")
	}
	if strings.TrimSpace(filter.Employee) != "" {
		where = append(where, "contact.employee = ?")
		args = append(args, strings.TrimSpace(filter.Employee))
	}
	if strings.TrimSpace(filter.StartTime) != "" {
		where = append(where, "contact.created_at > ?", "contact.created_at < ?")
		args = append(args, strings.TrimSpace(filter.StartTime), strings.TrimSpace(filter.EndTime))
	}
	if filter.Status != nil {
		where = append(where, "contact.status = ?")
		args = append(args, *filter.Status)
	}
	if filter.Loss != nil {
		where = append(where, "contact.loss = ?")
		args = append(args, *filter.Loss)
	}
	whereSQL := "WHERE " + strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_work_fission_contact AS contact
		INNER JOIN mc_work_fission AS fission ON fission.id = contact.fission_id
		`+whereSQL, args...).Scan(&total); err != nil {
		return dashboard.WorkFissionInviteDataPage{}, err
	}
	queryArgs := []any{filter.CorpID, filter.CorpID, filter.CorpID, filter.CorpID}
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			contact.id,
			COALESCE(contact.nickname, ''),
			COALESCE(contact.avatar, ''),
			COALESCE(fission.active_name, ''),
			COALESCE(contact.employee, ''),
			COALESCE((SELECT employee.name FROM mc_work_employee AS employee WHERE employee.corp_id = ? AND employee.wx_user_id = contact.employee AND employee.deleted_at IS NULL LIMIT 1), ''),
			contact.created_at,
			COALESCE(contact.loss, 0),
			COALESCE(contact.level, 0),
			COALESCE(contact.status, 0),
			COALESCE(contact.invite_count, 0),
			COALESCE((SELECT work_contact.id FROM mc_work_contact AS work_contact WHERE work_contact.corp_id = ? AND work_contact.unionid = contact.union_id AND work_contact.deleted_at IS NULL LIMIT 1), 0),
			COALESCE((
				SELECT rel.employee_id
				FROM mc_work_contact_employee AS rel
				WHERE rel.corp_id = ?
				  AND rel.contact_id = (
				    SELECT work_contact2.id
				    FROM mc_work_contact AS work_contact2
				    WHERE work_contact2.corp_id = ? AND work_contact2.unionid = contact.union_id AND work_contact2.deleted_at IS NULL
				    LIMIT 1
				  )
				  AND rel.deleted_at IS NULL
				LIMIT 1
			), 0)
		FROM mc_work_fission_contact AS contact
		INNER JOIN mc_work_fission AS fission ON fission.id = contact.fission_id
		`+whereSQL+`
		ORDER BY contact.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.WorkFissionInviteDataPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.WorkFissionInviteDataItem, 0)
	for rows.Next() {
		var item dashboard.WorkFissionInviteDataItem
		var createdAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.Nickname,
			&item.Avatar,
			&item.ActiveName,
			&item.Employee,
			&item.EmployeeName,
			&createdAt,
			&item.Loss,
			&item.Level,
			&item.Status,
			&item.InviteCount,
			&item.ContactID,
			&item.EmployeeID,
		); err != nil {
			return dashboard.WorkFissionInviteDataPage{}, err
		}
		item.CreatedAt = formatTime(createdAt)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkFissionInviteDataPage{}, err
	}
	return dashboard.WorkFissionInviteDataPage{
		Items:     items,
		Total:     total,
		TotalPage: pageCount(total, filter.PerPage),
		PerPage:   filter.PerPage,
	}, nil
}

func (s *MySQLStore) CreateWorkFission(ctx context.Context, values dashboard.WorkFissionWrite) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_work_fission
			(corp_id, active_name, service_employees, auto_pass, auto_add_tag, contact_tags, end_time,
			 qr_code_invalid, tasks, new_friend, delete_invalid, receive_prize, receive_prize_employees,
			 receive_links, receive_qrcode, create_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, values.Fission.CorpID, values.Fission.ActiveName, values.Fission.ServiceEmployees, values.Fission.AutoPass, values.Fission.AutoAddTag, values.Fission.ContactTags, workFissionTimeArg(values.Fission.EndTime),
		values.Fission.QRCodeInvalid, values.Fission.Tasks, values.Fission.NewFriend, values.Fission.DeleteInvalid, values.Fission.ReceivePrize, values.Fission.ReceivePrizeEmployees,
		values.Fission.ReceiveLinks, values.Fission.ReceiveQRCode, values.Fission.CreateUserID, now, now)
	if err != nil {
		return 0, err
	}
	id64, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	id := int(id64)
	if err := insertWorkFissionPoster(ctx, tx, id, values.Poster, now); err != nil {
		return 0, err
	}
	if err := insertWorkFissionWelcome(ctx, tx, id, values.Welcome, now); err != nil {
		return 0, err
	}
	if err := insertWorkFissionPush(ctx, tx, id, values.Push, now); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mc_work_fission_invite
			(fission_id, type, text, link_title, link_desc, link_pic, wx_link_pic, created_at, updated_at)
		VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?)
	`, id, values.Invite.Text, values.Invite.LinkTitle, values.Invite.LinkDesc, values.Invite.LinkPic, values.Invite.WXLinkPic, now, now); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *MySQLStore) UpdateWorkFission(ctx context.Context, corpID int, id int, values dashboard.WorkFissionWrite) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	now := time.Now()
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_work_fission
		SET active_name = ?, service_employees = ?, auto_pass = ?, auto_add_tag = ?, contact_tags = ?, end_time = ?,
		    qr_code_invalid = ?, tasks = ?, new_friend = ?, delete_invalid = ?, receive_prize = ?,
		    receive_prize_employees = ?, receive_links = ?, receive_qrcode = ?, updated_at = ?
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, values.Fission.ActiveName, values.Fission.ServiceEmployees, values.Fission.AutoPass, values.Fission.AutoAddTag, values.Fission.ContactTags, workFissionTimeArg(values.Fission.EndTime),
		values.Fission.QRCodeInvalid, values.Fission.Tasks, values.Fission.NewFriend, values.Fission.DeleteInvalid, values.Fission.ReceivePrize,
		values.Fission.ReceivePrizeEmployees, values.Fission.ReceiveLinks, values.Fission.ReceiveQRCode, now, id, corpID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}
	if err := upsertWorkFissionPoster(ctx, tx, id, values.Poster, now); err != nil {
		return false, err
	}
	if err := upsertWorkFissionWelcome(ctx, tx, id, values.Welcome, now); err != nil {
		return false, err
	}
	if err := upsertWorkFissionPush(ctx, tx, id, values.Push, now); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) UpsertWorkFissionInvite(ctx context.Context, values dashboard.WorkFissionInviteWrite) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now()
	var id int
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_fission_invite
		WHERE fission_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, values.FissionID).Scan(&id)
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO mc_work_fission_invite (fission_id, type, text, link_title, link_desc, link_pic, wx_link_pic, created_at, updated_at)
			VALUES (?, 1, ?, ?, ?, ?, '', ?, ?)
		`, values.FissionID, values.Text, values.LinkTitle, values.LinkDesc, values.LinkPic, now, now)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_fission_invite
		SET fission_id = ?, type = 1, text = ?, link_title = ?, link_desc = ?, link_pic = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`, values.FissionID, values.Text, values.LinkTitle, values.LinkDesc, values.LinkPic, now, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) WorkFissionInviteDetail(ctx context.Context, corpID int, id int) (dashboard.WorkFissionInviteDetail, bool, error) {
	var detail dashboard.WorkFissionInviteDetail
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(contact.invite_count, 0)
		FROM mc_work_fission_contact AS contact
		INNER JOIN mc_work_fission AS fission ON fission.id = contact.fission_id AND fission.deleted_at IS NULL
		WHERE contact.id = ? AND contact.deleted_at IS NULL AND fission.corp_id = ?
		LIMIT 1
	`, id, corpID).Scan(&detail.TotalCount)
	if err == sql.ErrNoRows {
		return dashboard.WorkFissionInviteDetail{}, false, nil
	}
	if err != nil {
		return dashboard.WorkFissionInviteDetail{}, false, err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN child.is_new = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN child.loss = 1 THEN 1 ELSE 0 END), 0)
		FROM mc_work_fission_contact AS child
		INNER JOIN mc_work_fission AS fission ON fission.id = child.fission_id AND fission.deleted_at IS NULL
		WHERE child.contact_superior_user_parent = ? AND child.deleted_at IS NULL AND fission.corp_id = ?
	`, id, corpID).Scan(&detail.NewCount, &detail.LossCount); err != nil {
		return dashboard.WorkFissionInviteDetail{}, false, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			child.id,
			COALESCE(child.nickname, ''),
			COALESCE(child.loss, 0),
			child.created_at,
			COALESCE((SELECT contact.avatar FROM mc_work_contact AS contact WHERE contact.corp_id = ? AND contact.unionid = child.union_id AND contact.deleted_at IS NULL LIMIT 1), '')
		FROM mc_work_fission_contact AS child
		INNER JOIN mc_work_fission AS fission ON fission.id = child.fission_id AND fission.deleted_at IS NULL
		WHERE child.contact_superior_user_parent = ? AND child.deleted_at IS NULL AND fission.corp_id = ?
		ORDER BY child.id DESC
	`, corpID, id, corpID)
	if err != nil {
		return dashboard.WorkFissionInviteDetail{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var child dashboard.WorkFissionInviteDetailChild
		var createdAt sql.NullTime
		if err := rows.Scan(&child.ID, &child.Nickname, &child.Loss, &createdAt, &child.Avatar); err != nil {
			return dashboard.WorkFissionInviteDetail{}, false, err
		}
		child.CreatedAt = formatTime(createdAt)
		detail.Children = append(detail.Children, child)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkFissionInviteDetail{}, false, err
	}
	return detail, true, nil
}

func (s *MySQLStore) DeleteWorkFissionCascade(ctx context.Context, corpID int, id int) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	now := time.Now()
	reclaimPaths, tenantID, err := s.workFissionStoragePathsForDelete(ctx, tx, corpID, id)
	if err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_work_fission
		SET deleted_at = ?, updated_at = ?
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, now, now, id, corpID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}
	for _, table := range []string{
		"mc_work_fission_poster",
		"mc_work_fission_welcome",
		"mc_work_fission_push",
		"mc_work_fission_invite",
	} {
		if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET deleted_at = ?, updated_at = ? WHERE fission_id = ? AND deleted_at IS NULL`, now, now, id); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return false, err
	}
	if tenantID > 0 {
		if err := s.RefreshSaaSUsageCounter(ctx, tenantID, dashboard.SaaSMetricWorkFissions); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (s *MySQLStore) workFissionStoragePathsForDelete(ctx context.Context, tx *sql.Tx, corpID int, id int) ([]string, int, error) {
	tenantID, err := s.tenantIDByCorpID(ctx, corpID)
	if err != nil {
		return nil, 0, err
	}
	paths := []string{}

	var receiveQRCode sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(receive_qrcode, '')
		FROM mc_work_fission
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, id, corpID).Scan(&receiveQRCode)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, tenantID, nil
	}
	if err != nil {
		return nil, 0, err
	}
	paths = appendWorkFissionStoragePathsFromJSON(paths, receiveQRCode.String)

	rows, err := tx.QueryContext(ctx, `
		SELECT COALESCE(cover_pic, ''), COALESCE(card_corp_logo, ''), COALESCE(qrcode_url, '')
		FROM mc_work_fission_poster
		WHERE fission_id = ? AND deleted_at IS NULL
	`, id)
	if err != nil {
		return nil, 0, err
	}
	for rows.Next() {
		var coverPic, cardCorpLogo, qrcodeURL string
		if err := rows.Scan(&coverPic, &cardCorpLogo, &qrcodeURL); err != nil {
			rows.Close()
			return nil, 0, err
		}
		paths = append(paths, coverPic, cardCorpLogo, qrcodeURL)
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	rows, err = tx.QueryContext(ctx, `
		SELECT COALESCE(link_cover_url, '')
		FROM mc_work_fission_welcome
		WHERE fission_id = ? AND deleted_at IS NULL
	`, id)
	if err != nil {
		return nil, 0, err
	}
	for rows.Next() {
		var linkCoverURL string
		if err := rows.Scan(&linkCoverURL); err != nil {
			rows.Close()
			return nil, 0, err
		}
		paths = append(paths, linkCoverURL)
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	rows, err = tx.QueryContext(ctx, `
		SELECT COALESCE(link_pic, '')
		FROM mc_work_fission_invite
		WHERE fission_id = ? AND deleted_at IS NULL
	`, id)
	if err != nil {
		return nil, 0, err
	}
	for rows.Next() {
		var linkPic string
		if err := rows.Scan(&linkPic); err != nil {
			rows.Close()
			return nil, 0, err
		}
		paths = append(paths, linkPic)
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	rows, err = tx.QueryContext(ctx, `
		SELECT COALESCE(CAST(msg_complex AS CHAR), '')
		FROM mc_work_fission_push
		WHERE fission_id = ? AND deleted_at IS NULL
	`, id)
	if err != nil {
		return nil, 0, err
	}
	for rows.Next() {
		var msgComplex string
		if err := rows.Scan(&msgComplex); err != nil {
			rows.Close()
			return nil, 0, err
		}
		paths = appendWorkFissionStoragePathsFromJSON(paths, msgComplex)
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return uniqueStorageRelativePaths(paths), tenantID, nil
}

func appendWorkFissionStoragePathsFromJSON(paths []string, raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return paths
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return paths
	}
	return appendWorkFissionStoragePathsFromAny(paths, value, "")
}

func appendWorkFissionStoragePathsFromAny(paths []string, value any, key string) []string {
	switch typed := value.(type) {
	case map[string]any:
		for childKey, childValue := range typed {
			paths = appendWorkFissionStoragePathsFromAny(paths, childValue, childKey)
		}
	case []any:
		for _, childValue := range typed {
			paths = appendWorkFissionStoragePathsFromAny(paths, childValue, key)
		}
	case string:
		switch key {
		case "image", "pic_url", "cover_pic", "card_corp_logo", "link_pic", "link_cover_url":
			paths = append(paths, typed)
		}
	}
	return paths
}

func (s *MySQLStore) workFissionInfoByID(ctx context.Context, corpID int, id int) (dashboard.WorkFissionInfo, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, active_name, service_employees, auto_pass, auto_add_tag, contact_tags, end_time,
		       qr_code_invalid, tasks, new_friend, delete_invalid, receive_prize, receive_prize_employees,
		       receive_links, created_at, create_user_id
		FROM mc_work_fission
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, id, corpID)
	var item dashboard.WorkFissionInfo
	var activeName, serviceEmployees, contactTags, tasks, receivePrizeEmployees, receiveLinks sql.NullString
	var autoPass, autoAddTag, qrCodeInvalid, newFriend, deleteInvalid, receivePrize, createUserID sql.NullInt64
	var endTime, createdAt sql.NullTime
	err := row.Scan(
		&item.ID,
		&item.CorpID,
		&activeName,
		&serviceEmployees,
		&autoPass,
		&autoAddTag,
		&contactTags,
		&endTime,
		&qrCodeInvalid,
		&tasks,
		&newFriend,
		&deleteInvalid,
		&receivePrize,
		&receivePrizeEmployees,
		&receiveLinks,
		&createdAt,
		&createUserID,
	)
	if err == sql.ErrNoRows {
		return dashboard.WorkFissionInfo{}, false, nil
	}
	if err != nil {
		return dashboard.WorkFissionInfo{}, false, err
	}
	item.ActiveName = nullString(activeName)
	item.ServiceEmployees = nullString(serviceEmployees)
	item.AutoPass = nullInt64(autoPass)
	item.AutoAddTag = nullInt64(autoAddTag)
	item.ContactTags = nullString(contactTags)
	item.EndTime = formatTime(endTime)
	item.QRCodeInvalid = nullInt64(qrCodeInvalid)
	item.Tasks = nullString(tasks)
	item.NewFriend = nullInt64(newFriend)
	item.DeleteInvalid = nullInt64(deleteInvalid)
	item.ReceivePrize = nullInt64(receivePrize)
	item.ReceivePrizeEmployees = nullString(receivePrizeEmployees)
	item.ReceiveLinks = nullString(receiveLinks)
	item.CreatedAt = formatTime(createdAt)
	item.CreateUserID = nullInt64(createUserID)
	return item, true, nil
}

func (s *MySQLStore) workFissionPosterByFissionID(ctx context.Context, fissionID int) (dashboard.WorkFissionPoster, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT poster_type, cover_pic, wx_cover_pic, foward_text, avatar_show, nickname_show, nickname_color,
		       card_corp_image_name, card_corp_name, card_corp_logo, qrcode_w, qrcode_h, qrcode_x, qrcode_y,
		       qrcode_id, qrcode_url
		FROM mc_work_fission_poster
		WHERE fission_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, fissionID)
	var item dashboard.WorkFissionPoster
	var posterType, avatarShow, nicknameShow sql.NullInt64
	var coverPic, wxCoverPic, fowardText, nicknameColor, cardCorpImageName, cardCorpName, cardCorpLogo sql.NullString
	var qrcodeW, qrcodeH, qrcodeX, qrcodeY, qrcodeID, qrcodeURL sql.NullString
	err := row.Scan(&posterType, &coverPic, &wxCoverPic, &fowardText, &avatarShow, &nicknameShow, &nicknameColor, &cardCorpImageName, &cardCorpName, &cardCorpLogo, &qrcodeW, &qrcodeH, &qrcodeX, &qrcodeY, &qrcodeID, &qrcodeURL)
	if err == sql.ErrNoRows {
		return dashboard.WorkFissionPoster{}, false, nil
	}
	if err != nil {
		return dashboard.WorkFissionPoster{}, false, err
	}
	item.PosterType = nullInt64(posterType)
	item.CoverPic = nullString(coverPic)
	item.WXCoverPic = nullString(wxCoverPic)
	item.FowardText = nullString(fowardText)
	item.AvatarShow = nullInt64(avatarShow)
	item.NicknameShow = nullInt64(nicknameShow)
	item.NicknameColor = nullString(nicknameColor)
	item.CardCorpImageName = nullString(cardCorpImageName)
	item.CardCorpName = nullString(cardCorpName)
	item.CardCorpLogo = nullString(cardCorpLogo)
	item.QRCodeW = nullString(qrcodeW)
	item.QRCodeH = nullString(qrcodeH)
	item.QRCodeX = nullString(qrcodeX)
	item.QRCodeY = nullString(qrcodeY)
	item.QRCodeID = nullString(qrcodeID)
	item.QRCodeURL = nullString(qrcodeURL)
	return item, true, nil
}

func (s *MySQLStore) workFissionWelcomeByFissionID(ctx context.Context, fissionID int) (dashboard.WorkFissionWelcome, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT msg_text, link_title, link_desc, link_cover_url
		FROM mc_work_fission_welcome
		WHERE fission_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, fissionID)
	var item dashboard.WorkFissionWelcome
	var msgText, linkTitle, linkDesc, linkCoverURL sql.NullString
	err := row.Scan(&msgText, &linkTitle, &linkDesc, &linkCoverURL)
	if err == sql.ErrNoRows {
		return dashboard.WorkFissionWelcome{}, false, nil
	}
	if err != nil {
		return dashboard.WorkFissionWelcome{}, false, err
	}
	item.MsgText = nullString(msgText)
	item.LinkTitle = nullString(linkTitle)
	item.LinkDesc = nullString(linkDesc)
	item.LinkCoverURL = nullString(linkCoverURL)
	return item, true, nil
}

func (s *MySQLStore) workFissionPushByFissionID(ctx context.Context, fissionID int) (dashboard.WorkFissionPush, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT push_employee, push_contact, msg_text, msg_complex, msg_complex_type
		FROM mc_work_fission_push
		WHERE fission_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, fissionID)
	var item dashboard.WorkFissionPush
	var pushEmployee, pushContact sql.NullInt64
	var msgText, msgComplex, msgComplexType sql.NullString
	err := row.Scan(&pushEmployee, &pushContact, &msgText, &msgComplex, &msgComplexType)
	if err == sql.ErrNoRows {
		return dashboard.WorkFissionPush{}, false, nil
	}
	if err != nil {
		return dashboard.WorkFissionPush{}, false, err
	}
	item.PushEmployee = nullInt64(pushEmployee)
	item.PushContact = nullInt64(pushContact)
	item.MsgText = nullString(msgText)
	item.MsgComplex = nullString(msgComplex)
	item.MsgComplexType = nullString(msgComplexType)
	return item, true, nil
}

func (s *MySQLStore) workFissionInviteByFissionID(ctx context.Context, fissionID int) (dashboard.WorkFissionInvite, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT text, link_title, link_desc, link_pic
		FROM mc_work_fission_invite
		WHERE fission_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, fissionID)
	var item dashboard.WorkFissionInvite
	var text, linkTitle, linkDesc, linkPic sql.NullString
	err := row.Scan(&text, &linkTitle, &linkDesc, &linkPic)
	if err == sql.ErrNoRows {
		return dashboard.WorkFissionInvite{}, false, nil
	}
	if err != nil {
		return dashboard.WorkFissionInvite{}, false, err
	}
	item.Text = nullString(text)
	item.LinkTitle = nullString(linkTitle)
	item.LinkDesc = nullString(linkDesc)
	item.LinkPic = nullString(linkPic)
	return item, true, nil
}

func (s *MySQLStore) workFissionNamesByCorpID(ctx context.Context, corpID int) ([]dashboard.WorkFissionName, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(active_name, '')
		FROM mc_work_fission
		WHERE corp_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.WorkFissionName, 0)
	for rows.Next() {
		var item dashboard.WorkFissionName
		if err := rows.Scan(&item.ID, &item.ActiveName); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) workFissionServiceEmployees(ctx context.Context, corpID int, fissionIDs []int) ([]string, error) {
	if len(fissionIDs) == 0 {
		return []string{}, nil
	}
	args := []any{corpID}
	args = append(args, intsToArgs(fissionIDs)...)
	rows, err := s.db.QueryContext(ctx, `
		SELECT service_employees
		FROM mc_work_fission
		WHERE corp_id = ? AND deleted_at IS NULL AND id IN (`+placeholders(len(fissionIDs))+`)
		ORDER BY id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]string, 0)
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		if value := nullString(raw); value != "" {
			values = append(values, value)
		}
	}
	return values, rows.Err()
}

func (s *MySQLStore) WorkFissionOperationFissionByID(ctx context.Context, id int) (dashboard.WorkFissionOperationFission, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, service_employees, auto_pass, tasks, end_time, delete_invalid,
		       receive_prize, receive_links, receive_qrcode
		FROM mc_work_fission
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, id)
	var item dashboard.WorkFissionOperationFission
	var serviceEmployees, tasks, receiveLinks, receiveQRCode sql.NullString
	var corpID, autoPass, deleteInvalid, receivePrize sql.NullInt64
	var endTime sql.NullTime
	err := row.Scan(&item.ID, &corpID, &serviceEmployees, &autoPass, &tasks, &endTime, &deleteInvalid, &receivePrize, &receiveLinks, &receiveQRCode)
	if err == sql.ErrNoRows {
		return dashboard.WorkFissionOperationFission{}, false, nil
	}
	if err != nil {
		return dashboard.WorkFissionOperationFission{}, false, err
	}
	item.CorpID = nullInt64(corpID)
	item.ServiceEmployees = nullString(serviceEmployees)
	item.AutoPass = nullInt64(autoPass)
	item.Tasks = nullString(tasks)
	item.EndTime = formatTime(endTime)
	item.DeleteInvalid = nullInt64(deleteInvalid)
	item.ReceivePrize = nullInt64(receivePrize)
	item.ReceiveLinks = nullString(receiveLinks)
	item.ReceiveQRCode = nullString(receiveQRCode)
	return item, true, nil
}

func (s *MySQLStore) WorkFissionOperationPosterByFissionID(ctx context.Context, fissionID int) (dashboard.WorkFissionPoster, bool, error) {
	return s.workFissionPosterByFissionID(ctx, fissionID)
}

func (s *MySQLStore) WorkFissionOperationContactByUnionID(ctx context.Context, unionID string) (dashboard.WorkFissionOperationContact, bool, error) {
	row := s.db.QueryRowContext(ctx, workFissionOperationContactSelect(`
		WHERE union_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`), unionID)
	return scanWorkFissionOperationContact(row)
}

func (s *MySQLStore) WorkFissionOperationContactByFissionUnionID(ctx context.Context, fissionID int, unionID string) (dashboard.WorkFissionOperationContact, bool, error) {
	row := s.db.QueryRowContext(ctx, workFissionOperationContactSelect(`
		WHERE fission_id = ? AND union_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`), fissionID, unionID)
	return scanWorkFissionOperationContact(row)
}

func (s *MySQLStore) WorkFissionOperationChildren(ctx context.Context, parentID int, fissionID int) ([]dashboard.WorkFissionOperationContact, error) {
	rows, err := s.db.QueryContext(ctx, workFissionOperationContactSelect(`
		WHERE contact_superior_user_parent = ? AND fission_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
	`), parentID, fissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.WorkFissionOperationContact, 0)
	for rows.Next() {
		item, err := scanWorkFissionOperationContactRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) WorkFissionOperationWorkContactByUnionID(ctx context.Context, unionID string) (dashboard.WorkFissionOperationWorkContact, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, wx_external_userid, avatar
		FROM mc_work_contact
		WHERE unionid = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, unionID)
	var item dashboard.WorkFissionOperationWorkContact
	var corpID sql.NullInt64
	var wxExternalUserID, avatar sql.NullString
	err := row.Scan(&item.ID, &corpID, &wxExternalUserID, &avatar)
	if err == sql.ErrNoRows {
		return dashboard.WorkFissionOperationWorkContact{}, false, nil
	}
	if err != nil {
		return dashboard.WorkFissionOperationWorkContact{}, false, err
	}
	item.CorpID = nullInt64(corpID)
	item.WXExternalUserID = nullString(wxExternalUserID)
	item.Avatar = nullString(avatar)
	return item, true, nil
}

func (s *MySQLStore) WorkFissionOperationInviteCount(ctx context.Context, parentID int, fissionID int, onlyNotLoss bool) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM mc_work_fission_contact
		WHERE contact_superior_user_parent = ? AND fission_id = ? AND deleted_at IS NULL
	`
	args := []any{parentID, fissionID}
	if onlyNotLoss {
		query += " AND loss = 0"
	}
	var count int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *MySQLStore) CreateWorkFissionOperationContact(ctx context.Context, values dashboard.WorkFissionOperationContactCreate) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_work_fission_contact
			(fission_id, union_id, nickname, avatar, contact_superior_user_parent, level, employee,
			 invite_count, loss, status, receive_level, is_new, external_user_id, qrcode_id, qrcode_url,
			 created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, ?, '', 0, 0, 0, 0, 1, ?, '', '', NOW(), NOW())
	`, values.FissionID, values.UnionID, values.Nickname, values.Avatar, values.Level, values.ExternalUserID)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *MySQLStore) UpdateWorkFissionOperationContactQRCode(ctx context.Context, id int, qrcodeID string, qrcodeURL string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_work_fission_contact
		SET qrcode_id = ?, qrcode_url = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, qrcodeID, qrcodeURL, id)
	return err
}

func (s *MySQLStore) UpdateWorkFissionOperationReceiveLevel(ctx context.Context, id int, level int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_work_fission_contact
		SET receive_level = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, level, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *MySQLStore) OfficialAccountsByCorpID(ctx context.Context, corpID int) ([]dashboard.OfficialAccountItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(nickname, ''), COALESCE(avatar, '')
		FROM mc_official_account
		WHERE corp_id = ? AND authorized_status <> 3 AND deleted_at IS NULL
		ORDER BY id ASC
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]dashboard.OfficialAccountItem, 0)
	for rows.Next() {
		var item dashboard.OfficialAccountItem
		if err := rows.Scan(&item.ID, &item.Nickname, &item.Avatar); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *MySQLStore) OfficialAccountFirstByCorpID(ctx context.Context, corpID int) (dashboard.OfficialAccountItem, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(nickname, ''), COALESCE(avatar, '')
		FROM mc_official_account
		WHERE corp_id = ? AND authorized_status <> 3 AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, corpID)
	return scanOfficialAccountItem(row)
}

func (s *MySQLStore) OfficialAccountByID(ctx context.Context, id int) (dashboard.OfficialAccountItem, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(nickname, ''), COALESCE(avatar, '')
		FROM mc_official_account
		WHERE id = ? AND authorized_status <> 3 AND deleted_at IS NULL
	`, id)
	return scanOfficialAccountItem(row)
}

func (s *MySQLStore) OfficialAccountOAuthInfoByCorpIDType(ctx context.Context, corpID int, accountType int) (dashboard.OfficialAccountOAuthInfo, bool, error) {
	set, found, err := s.OfficialAccountSetByCorpIDType(ctx, corpID, accountType)
	if err != nil {
		return dashboard.OfficialAccountOAuthInfo{}, false, err
	}
	if found && set.OfficialAccountID > 0 {
		info, found, err := s.officialAccountOAuthInfoByID(ctx, set.OfficialAccountID)
		if err != nil {
			return dashboard.OfficialAccountOAuthInfo{}, false, err
		}
		if found {
			return info, true, nil
		}
	}
	row := s.db.QueryRowContext(ctx, officialAccountCredentialSelect(`
		WHERE corp_id = ? AND authorized_status <> 3 AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`), corpID)
	return s.scanOfficialAccountOAuthInfo(row)
}

func (s *MySQLStore) OfficialAccountOAuthInfoByCorpID(ctx context.Context, corpID int) (dashboard.OfficialAccountOAuthInfo, bool, error) {
	row := s.db.QueryRowContext(ctx, officialAccountCredentialSelect(`
		WHERE corp_id = ? AND authorized_status <> 3 AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`), corpID)
	return s.scanOfficialAccountOAuthInfo(row)
}

func (s *MySQLStore) OfficialAccountOAuthInfoByAuthorizerAppID(ctx context.Context, authorizerAppID string) (dashboard.OfficialAccountOAuthInfo, bool, error) {
	row := s.db.QueryRowContext(ctx, officialAccountCredentialSelect(`
		WHERE authorizer_appid = ? AND authorized_status <> 3 AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`), authorizerAppID)
	return s.scanOfficialAccountOAuthInfo(row)
}

func (s *MySQLStore) OfficialAccountOAuthInfoByID(ctx context.Context, id int) (dashboard.OfficialAccountOAuthInfo, bool, error) {
	return s.officialAccountOAuthInfoByID(ctx, id)
}

func (s *MySQLStore) officialAccountOAuthInfoByID(ctx context.Context, id int) (dashboard.OfficialAccountOAuthInfo, bool, error) {
	row := s.db.QueryRowContext(ctx, officialAccountCredentialSelect(`
		WHERE id = ? AND authorized_status <> 3 AND deleted_at IS NULL
		LIMIT 1
	`), id)
	return s.scanOfficialAccountOAuthInfo(row)
}

func (s *MySQLStore) UpsertOfficialAccountAuthEvent(ctx context.Context, values dashboard.OfficialAccountAuthorization) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var id int
	var corpID int
	err = tx.QueryRowContext(ctx, `
		SELECT id, COALESCE(corp_id, 0)
		FROM mc_official_account
		WHERE appid = ? AND authorizer_appid = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, values.ComponentAppID, values.AuthorizerAppID).Scan(&id, &corpID)
	if err == sql.ErrNoRows {
		if values.AuthorizedStatus == 3 {
			result, err := tx.ExecContext(ctx, `
				INSERT INTO mc_official_account
					(appid, authorized_status, authorizer_appid, create_time, created_at, updated_at)
				VALUES (?, ?, ?, ?, NOW(), NOW())
			`, values.ComponentAppID, values.AuthorizedStatus, values.AuthorizerAppID, values.CreateTime)
			if err != nil {
				return 0, err
			}
			insertID, err := result.LastInsertId()
			if err != nil {
				return 0, err
			}
			if err := tx.Commit(); err != nil {
				return 0, err
			}
			return int(insertID), nil
		}
		storage, err := s.encodeOfficialAccountCredential(0, values.AuthorizerAppID, officialAccountCredentialFromAuthorization(values))
		if err != nil {
			return 0, err
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO mc_official_account
				(appid, authorized_status, authorizer_appid, wechat_credentials_ciphertext, wechat_credentials_key_id,
				 create_time, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())
		`, values.ComponentAppID, values.AuthorizedStatus, values.AuthorizerAppID, storage.Ciphertext, storage.KeyID, values.CreateTime)
		if err != nil {
			return 0, err
		}
		insertID, err := result.LastInsertId()
		if err != nil {
			return 0, err
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		return int(insertID), nil
	}
	if err != nil {
		return 0, err
	}
	if values.AuthorizedStatus == 3 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_official_account
			SET appid = ?, authorized_status = ?, authorizer_appid = ?, create_time = ?, updated_at = NOW()
			WHERE id = ?
		`, values.ComponentAppID, values.AuthorizedStatus, values.AuthorizerAppID, values.CreateTime, id); err != nil {
			return 0, err
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		if err := s.refreshOfficialAccountUsageByCorpID(ctx, corpID); err != nil {
			return 0, err
		}
		return id, nil
	}
	current, found, err := s.loadOfficialAccountCredentialByID(ctx, tx, id, true)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, sql.ErrNoRows
	}
	credential, err := s.decodeOfficialAccountCredential(current)
	if err != nil {
		return 0, err
	}
	credential = mergeOfficialAccountCredential(credential, officialAccountCredentialFromAuthorization(values))
	storage, err := s.encodeOfficialAccountCredential(current.TenantID, values.AuthorizerAppID, credential)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_official_account
		SET appid = ?, authorized_status = ?, authorizer_appid = ?, wechat_credentials_ciphertext = ?,
		    wechat_credentials_key_id = ?, create_time = ?, updated_at = NOW()
		WHERE id = ?
	`, values.ComponentAppID, values.AuthorizedStatus, values.AuthorizerAppID, storage.Ciphertext, storage.KeyID, values.CreateTime, id); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	if err := s.refreshOfficialAccountUsageByCorpID(ctx, corpID); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *MySQLStore) refreshOfficialAccountUsageByCorpID(ctx context.Context, corpID int) error {
	if corpID <= 0 {
		return nil
	}
	tenantID, err := s.tenantIDByCorpID(ctx, corpID)
	if err != nil {
		return err
	}
	if tenantID <= 0 {
		return nil
	}
	return s.RefreshSaaSUsageCounter(ctx, tenantID, dashboard.SaaSMetricOfficialAccounts)
}

func (s *MySQLStore) UpsertWeChatComponentVerifyTicket(ctx context.Context, componentAppID string, componentVerifyTicket string, createTime int64) error {
	componentAppID = strings.TrimSpace(componentAppID)
	componentVerifyTicket = strings.TrimSpace(componentVerifyTicket)
	if componentAppID == "" {
		return errors.New("component_appid is required")
	}
	if componentVerifyTicket == "" {
		return errors.New("component_verify_ticket is required")
	}
	storage, err := s.encodeWeChatComponentTicketCredential(componentAppID, wechatopencredentials.TicketCredential{ComponentVerifyTicket: componentVerifyTicket})
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO mochat_go_wechat_component_tickets
			(component_appid, component_verify_ticket, component_verify_ticket_ciphertext, credential_key_id, create_time, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			component_verify_ticket = VALUES(component_verify_ticket),
			component_verify_ticket_ciphertext = VALUES(component_verify_ticket_ciphertext),
			credential_key_id = VALUES(credential_key_id),
			create_time = VALUES(create_time),
			updated_at = NOW()
	`, componentAppID, storage.Ticket, storage.Ciphertext, storage.KeyID, createTime)
	return err
}

func (s *MySQLStore) WeChatComponentVerifyTicket(ctx context.Context, componentAppID string) (string, bool, error) {
	componentAppID = strings.TrimSpace(componentAppID)
	if componentAppID == "" {
		return "", false, nil
	}
	item, found, err := s.loadWeChatComponentTicketCredential(ctx, s.db, componentAppID, false)
	if err != nil || !found {
		return "", found, err
	}
	credential, err := s.decodeWeChatComponentTicketCredential(item)
	if err != nil {
		return "", false, err
	}
	ticket := strings.TrimSpace(credential.ComponentVerifyTicket)
	return ticket, ticket != "", nil
}

func (s *MySQLStore) UpsertOfficialAccountAuthorization(ctx context.Context, values dashboard.OfficialAccountAuthorization) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	tenantID, err := s.tenantIDByCorpID(ctx, values.CorpID)
	if err != nil {
		return 0, err
	}

	var id int
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_official_account
		WHERE appid = ? AND authorizer_appid = ? AND corp_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, values.ComponentAppID, values.AuthorizerAppID, values.CorpID).Scan(&id)
	if err == sql.ErrNoRows {
		if err := s.enforceSaaSQuotaForTenant(ctx, tenantID, dashboard.SaaSMetricOfficialAccounts, 1); err != nil {
			return 0, err
		}
		storage, err := s.encodeOfficialAccountCredential(tenantID, values.AuthorizerAppID, officialAccountCredentialFromAuthorization(values))
		if err != nil {
			return 0, err
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO mc_official_account
				(appid, authorized_status, authorizer_appid, wechat_credentials_ciphertext, wechat_credentials_key_id,
				 func_info, tenant_id, corp_id, create_time, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		`, values.ComponentAppID, values.AuthorizedStatus, values.AuthorizerAppID, storage.Ciphertext, storage.KeyID,
			nullJSONString(values.FuncInfo), tenantID, values.CorpID, values.CreateTime)
		if err != nil {
			return 0, err
		}
		insertID, err := result.LastInsertId()
		if err != nil {
			return 0, err
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		if tenantID > 0 {
			if err := s.RefreshSaaSUsageCounter(ctx, tenantID, dashboard.SaaSMetricOfficialAccounts); err != nil {
				return 0, err
			}
		}
		return int(insertID), nil
	}
	if err != nil {
		return 0, err
	}
	current, found, err := s.loadOfficialAccountCredentialByID(ctx, tx, id, true)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, sql.ErrNoRows
	}
	credential, err := s.decodeOfficialAccountCredential(current)
	if err != nil {
		return 0, err
	}
	credential = mergeOfficialAccountCredential(credential, officialAccountCredentialFromAuthorization(values))
	storage, err := s.encodeOfficialAccountCredential(tenantID, values.AuthorizerAppID, credential)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_official_account
		SET appid = ?, authorized_status = ?, authorizer_appid = ?, wechat_credentials_ciphertext = ?, wechat_credentials_key_id = ?,
		    func_info = ?, tenant_id = ?, corp_id = ?, create_time = ?, updated_at = NOW()
		WHERE id = ?
	`, values.ComponentAppID, values.AuthorizedStatus, values.AuthorizerAppID, storage.Ciphertext, storage.KeyID,
		nullJSONString(values.FuncInfo), tenantID, values.CorpID, values.CreateTime, id); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	if err := s.refreshOfficialAccountUsageByCorpID(ctx, values.CorpID); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *MySQLStore) UpdateOfficialAccountAuthorizerProfile(ctx context.Context, id int, values dashboard.OfficialAccountAuthorizerProfile) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_official_account
		SET nickname = ?, head_img = ?, avatar = ?, service_type_info = ?, verify_type_info = ?,
		    user_name = ?, principal_name = ?, alias = ?, business_info = ?, qrcode_url = ?,
		    local_qrcode_url = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, values.Nickname, values.HeadImg, values.Avatar, values.ServiceTypeInfo, values.VerifyTypeInfo,
		values.UserName, values.PrincipalName, values.Alias, nullJSONString(values.BusinessInfo), values.QRCodeURL,
		values.LocalQRCodeURL, id)
	return err
}

func (s *MySQLStore) OfficialAccountSetByCorpIDType(ctx context.Context, corpID int, accountType int) (dashboard.OfficialAccountSetItem, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(official_account_id, 0), COALESCE(type, 0), COALESCE(corp_id, 0)
		FROM mc_official_account_set
		WHERE corp_id = ? AND type = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, corpID, accountType)
	var item dashboard.OfficialAccountSetItem
	err := row.Scan(&item.ID, &item.OfficialAccountID, &item.Type, &item.CorpID)
	if err == sql.ErrNoRows {
		return dashboard.OfficialAccountSetItem{}, false, nil
	}
	if err != nil {
		return dashboard.OfficialAccountSetItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) UpsertOfficialAccountSet(ctx context.Context, corpID int, accountType int, officialAccountID int, userID int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var id int
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_official_account_set
		WHERE corp_id = ? AND type = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, corpID, accountType).Scan(&id)
	now := time.Now()
	if err == sql.ErrNoRows {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_official_account_set
				(official_account_id, type, tenant_id, corp_id, create_user_id, created_at, updated_at)
			VALUES (?, ?, 0, ?, ?, ?, ?)
		`, officialAccountID, accountType, corpID, userID, now, now); err != nil {
			return err
		}
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_official_account_set
		SET official_account_id = ?, updated_at = ?
		WHERE id = ?
	`, officialAccountID, now, id); err != nil {
		return err
	}
	return tx.Commit()
}

type officialAccountScanner interface {
	Scan(dest ...any) error
}

type workFissionOperationContactScanner interface {
	Scan(dest ...any) error
}

func workFissionOperationContactSelect(suffix string) string {
	return `
		SELECT id, fission_id, union_id, nickname, avatar, contact_superior_user_parent,
		       level, employee, invite_count, loss, status, receive_level, is_new,
		       external_user_id, qrcode_id, qrcode_url, created_at
		FROM mc_work_fission_contact
	` + suffix
}

func scanWorkFissionOperationContact(scanner workFissionOperationContactScanner) (dashboard.WorkFissionOperationContact, bool, error) {
	item, err := scanWorkFissionOperationContactRow(scanner)
	if err == sql.ErrNoRows {
		return dashboard.WorkFissionOperationContact{}, false, nil
	}
	if err != nil {
		return dashboard.WorkFissionOperationContact{}, false, err
	}
	return item, true, nil
}

func scanWorkFissionOperationContactRow(scanner workFissionOperationContactScanner) (dashboard.WorkFissionOperationContact, error) {
	var item dashboard.WorkFissionOperationContact
	var unionID, nickname, avatar, employee, externalUserID, qrcodeID, qrcodeURL sql.NullString
	var fissionID, parentID, level, inviteCount, loss, status, receiveLevel, isNew sql.NullInt64
	var createdAt sql.NullTime
	err := scanner.Scan(
		&item.ID,
		&fissionID,
		&unionID,
		&nickname,
		&avatar,
		&parentID,
		&level,
		&employee,
		&inviteCount,
		&loss,
		&status,
		&receiveLevel,
		&isNew,
		&externalUserID,
		&qrcodeID,
		&qrcodeURL,
		&createdAt,
	)
	if err != nil {
		return dashboard.WorkFissionOperationContact{}, err
	}
	item.FissionID = nullInt64(fissionID)
	item.UnionID = nullString(unionID)
	item.Nickname = nullString(nickname)
	item.Avatar = nullString(avatar)
	item.ContactSuperiorUserParent = nullInt64(parentID)
	item.Level = nullInt64(level)
	item.Employee = nullString(employee)
	item.InviteCount = nullInt64(inviteCount)
	item.Loss = nullInt64(loss)
	item.Status = nullInt64(status)
	item.ReceiveLevel = nullInt64(receiveLevel)
	item.IsNew = nullInt64(isNew)
	item.ExternalUserID = nullString(externalUserID)
	item.QRCodeID = nullString(qrcodeID)
	item.QRCodeURL = nullString(qrcodeURL)
	item.CreatedAt = formatTime(createdAt)
	return item, nil
}

func scanOfficialAccountItem(scanner officialAccountScanner) (dashboard.OfficialAccountItem, bool, error) {
	var item dashboard.OfficialAccountItem
	err := scanner.Scan(&item.ID, &item.Nickname, &item.Avatar)
	if err == sql.ErrNoRows {
		return dashboard.OfficialAccountItem{}, false, nil
	}
	if err != nil {
		return dashboard.OfficialAccountItem{}, false, err
	}
	return item, true, nil
}

func insertWorkFissionPoster(ctx context.Context, tx *sql.Tx, fissionID int, values dashboard.WorkFissionPosterWrite, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mc_work_fission_poster
			(fission_id, poster_type, cover_pic, wx_cover_pic, foward_text, avatar_show, nickname_show,
			 nickname_color, card_corp_image_name, card_corp_name, card_corp_logo, qrcode_w, qrcode_h,
			 qrcode_x, qrcode_y, qrcode_id, qrcode_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, fissionID, values.PosterType, values.CoverPic, values.WXCoverPic, values.FowardText, values.AvatarShow, values.NicknameShow,
		values.NicknameColor, values.CardCorpImageName, values.CardCorpName, values.CardCorpLogo, values.QRCodeW, values.QRCodeH,
		values.QRCodeX, values.QRCodeY, values.QRCodeID, values.QRCodeURL, now, now)
	return err
}

func insertWorkFissionWelcome(ctx context.Context, tx *sql.Tx, fissionID int, values dashboard.WorkFissionWelcomeWrite, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mc_work_fission_welcome
			(fission_id, msg_text, link_title, link_desc, link_cover_url, link_wx_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, fissionID, values.MsgText, values.LinkTitle, values.LinkDesc, values.LinkCoverURL, values.LinkWXURL, now, now)
	return err
}

func insertWorkFissionPush(ctx context.Context, tx *sql.Tx, fissionID int, values dashboard.WorkFissionPushWrite, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mc_work_fission_push
			(fission_id, push_employee, push_contact, msg_text, msg_complex, msg_complex_type, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, fissionID, values.PushEmployee, values.PushContact, values.MsgText, values.MsgComplex, values.MsgComplexType, now, now)
	return err
}

func upsertWorkFissionPoster(ctx context.Context, tx *sql.Tx, fissionID int, values dashboard.WorkFissionPosterWrite, now time.Time) error {
	var id int
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM mc_work_fission_poster
		WHERE fission_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, fissionID).Scan(&id)
	if err == sql.ErrNoRows {
		return insertWorkFissionPoster(ctx, tx, fissionID, values, now)
	}
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE mc_work_fission_poster
		SET poster_type = ?, cover_pic = ?, wx_cover_pic = ?, foward_text = ?, avatar_show = ?,
		    nickname_show = ?, nickname_color = ?, card_corp_image_name = ?, card_corp_name = ?,
		    card_corp_logo = ?, qrcode_w = ?, qrcode_h = ?, qrcode_x = ?, qrcode_y = ?,
		    qrcode_id = ?, qrcode_url = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`, values.PosterType, values.CoverPic, values.WXCoverPic, values.FowardText, values.AvatarShow,
		values.NicknameShow, values.NicknameColor, values.CardCorpImageName, values.CardCorpName,
		values.CardCorpLogo, values.QRCodeW, values.QRCodeH, values.QRCodeX, values.QRCodeY,
		values.QRCodeID, values.QRCodeURL, now, id)
	return err
}

func upsertWorkFissionWelcome(ctx context.Context, tx *sql.Tx, fissionID int, values dashboard.WorkFissionWelcomeWrite, now time.Time) error {
	var id int
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM mc_work_fission_welcome
		WHERE fission_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, fissionID).Scan(&id)
	if err == sql.ErrNoRows {
		return insertWorkFissionWelcome(ctx, tx, fissionID, values, now)
	}
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE mc_work_fission_welcome
		SET msg_text = ?, link_title = ?, link_desc = ?, link_cover_url = ?, link_wx_url = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`, values.MsgText, values.LinkTitle, values.LinkDesc, values.LinkCoverURL, values.LinkWXURL, now, id)
	return err
}

func upsertWorkFissionPush(ctx context.Context, tx *sql.Tx, fissionID int, values dashboard.WorkFissionPushWrite, now time.Time) error {
	var id int
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM mc_work_fission_push
		WHERE fission_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, fissionID).Scan(&id)
	if err == sql.ErrNoRows {
		return insertWorkFissionPush(ctx, tx, fissionID, values, now)
	}
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE mc_work_fission_push
		SET push_employee = ?, push_contact = ?, msg_text = ?, msg_complex = ?, msg_complex_type = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`, values.PushEmployee, values.PushContact, values.MsgText, values.MsgComplex, values.MsgComplexType, now, id)
	return err
}

func workFissionTimeArg(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return raw
}

func nullTimeArg(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time
}

func mustJSONStore(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func formatTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02 15:04:05")
}

func (s *MySQLStore) ContactSOPDashboardPage(ctx context.Context, filter dashboard.ContactSOPDashboardFilter) (dashboard.ContactSOPDashboardPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 15
	}
	if filter.CorpID <= 0 {
		return dashboard.ContactSOPDashboardPage{Items: []dashboard.ContactSOPDashboardItem{}, Page: filter.Page, PerPage: filter.PerPage}, nil
	}
	where := []string{"s.corp_id = ?"}
	args := []any{filter.CorpID}
	if strings.TrimSpace(filter.Name) != "" {
		where = append(where, "s.name LIKE ?")
		args = append(args, "%"+strings.TrimSpace(filter.Name)+"%")
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_contact_sop s WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return dashboard.ContactSOPDashboardPage{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			s.id,
			COALESCE(s.corp_id, 0),
			COALESCE(s.creator_id, 0),
			COALESCE(u.name, ''),
			COALESCE(s.name, ''),
			COALESCE(s.setting, ''),
			COALESCE(s.employee_ids, ''),
			COALESCE(s.contact_ids, ''),
			COALESCE(s.state, 0),
			s.created_at,
			s.updated_at
		FROM mc_contact_sop s
		LEFT JOIN mc_user u ON u.id = s.creator_id AND u.deleted_at IS NULL
		WHERE `+whereSQL+`
		ORDER BY s.id DESC
		LIMIT ? OFFSET ?
	`, append(append([]any{}, args...), filter.PerPage, (filter.Page-1)*filter.PerPage)...)
	if err != nil {
		return dashboard.ContactSOPDashboardPage{}, err
	}
	defer rows.Close()

	items := []dashboard.ContactSOPDashboardItem{}
	for rows.Next() {
		item, err := scanContactSOPDashboardItem(rows)
		if err != nil {
			return dashboard.ContactSOPDashboardPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ContactSOPDashboardPage{}, err
	}
	return dashboard.ContactSOPDashboardPage{
		Items:     items,
		Total:     total,
		TotalPage: pageCount(total, filter.PerPage),
		PerPage:   filter.PerPage,
		Page:      filter.Page,
	}, nil
}

func (s *MySQLStore) ContactSOPDashboardByID(ctx context.Context, corpID int, id int) (dashboard.ContactSOPDashboardItem, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			s.id,
			COALESCE(s.corp_id, 0),
			COALESCE(s.creator_id, 0),
			COALESCE(u.name, ''),
			COALESCE(s.name, ''),
			COALESCE(s.setting, ''),
			COALESCE(s.employee_ids, ''),
			COALESCE(s.contact_ids, ''),
			COALESCE(s.state, 0),
			s.created_at,
			s.updated_at
		FROM mc_contact_sop s
		LEFT JOIN mc_user u ON u.id = s.creator_id AND u.deleted_at IS NULL
		WHERE s.id = ? AND s.corp_id = ?
		LIMIT 1
	`, id, corpID)
	item, err := scanContactSOPDashboardItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.ContactSOPDashboardItem{}, false, nil
	}
	if err != nil {
		return dashboard.ContactSOPDashboardItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) CreateContactSOPDashboard(ctx context.Context, values dashboard.ContactSOPDashboardWrite) (int, error) {
	name := strings.TrimSpace(values.Name)
	if name == "" {
		name = "未命名个人SOP"
	}
	setting := "[]"
	if values.HasSetting {
		setting = values.SettingRaw
	}
	employeeIDs := "[]"
	if values.HasEmployeeIDs {
		employeeIDs = values.EmployeeIDsRaw
	}
	contactIDs := "[]"
	if values.HasContactIDs {
		contactIDs = values.ContactIDsRaw
	}
	state := 1
	if values.HasState {
		state = values.State
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_contact_sop (corp_id, creator_id, name, setting, employee_ids, state, contact_ids, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, values.CorpID, values.CreatorID, name, setting, employeeIDs, state, contactIDs)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func (s *MySQLStore) UpdateContactSOPDashboard(ctx context.Context, corpID int, id int, values dashboard.ContactSOPDashboardWrite) (bool, error) {
	setSQL := []string{"updated_at = NOW()"}
	args := []any{}
	if values.HasName {
		setSQL = append(setSQL, "name = ?")
		args = append(args, strings.TrimSpace(values.Name))
	}
	if values.HasSetting {
		setSQL = append(setSQL, "setting = ?")
		args = append(args, values.SettingRaw)
	}
	if values.HasEmployeeIDs {
		setSQL = append(setSQL, "employee_ids = ?")
		args = append(args, values.EmployeeIDsRaw)
	}
	if values.HasContactIDs {
		setSQL = append(setSQL, "contact_ids = ?")
		args = append(args, values.ContactIDsRaw)
	}
	if values.HasState {
		setSQL = append(setSQL, "state = ?")
		args = append(args, values.State)
	}
	args = append(args, id, corpID)
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_contact_sop
		SET `+strings.Join(setSQL, ", ")+`
		WHERE id = ? AND corp_id = ?
	`, args...)
	return rowsAffectedBool(result, err)
}

func (s *MySQLStore) UpdateContactSOPDashboardEmployees(ctx context.Context, corpID int, id int, employeeIDsRaw string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_contact_sop
		SET employee_ids = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ?
	`, employeeIDsRaw, id, corpID)
	return rowsAffectedBool(result, err)
}

func (s *MySQLStore) UpdateContactSOPDashboardState(ctx context.Context, corpID int, id int, state int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_contact_sop
		SET state = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ?
	`, state, id, corpID)
	return rowsAffectedBool(result, err)
}

func (s *MySQLStore) DeleteContactSOPDashboard(ctx context.Context, corpID int, id int) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM mc_contact_sop_log WHERE corp_id = ? AND contact_sop_id = ?", corpID, id); err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM mc_contact_sop WHERE corp_id = ? AND id = ?", corpID, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *MySQLStore) RoomSOPDashboardPage(ctx context.Context, filter dashboard.RoomSOPDashboardFilter) (dashboard.RoomSOPDashboardPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 15
	}
	if filter.CorpID <= 0 {
		return dashboard.RoomSOPDashboardPage{Items: []dashboard.RoomSOPDashboardItem{}, Page: filter.Page, PerPage: filter.PerPage}, nil
	}
	where := []string{"s.corp_id = ?"}
	args := []any{filter.CorpID}
	if strings.TrimSpace(filter.Name) != "" {
		where = append(where, "s.name LIKE ?")
		args = append(args, "%"+strings.TrimSpace(filter.Name)+"%")
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_room_sop s WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return dashboard.RoomSOPDashboardPage{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			s.id,
			COALESCE(s.corp_id, 0),
			COALESCE(s.creator_id, 0),
			COALESCE(u.name, ''),
			COALESCE(s.name, ''),
			COALESCE(s.setting, ''),
			COALESCE(s.room_ids, ''),
			COALESCE(s.state, 0),
			s.created_at,
			s.updated_at
		FROM mc_room_sop s
		LEFT JOIN mc_user u ON u.id = s.creator_id AND u.deleted_at IS NULL
		WHERE `+whereSQL+`
		ORDER BY s.id DESC
		LIMIT ? OFFSET ?
	`, append(append([]any{}, args...), filter.PerPage, (filter.Page-1)*filter.PerPage)...)
	if err != nil {
		return dashboard.RoomSOPDashboardPage{}, err
	}
	defer rows.Close()

	items := []dashboard.RoomSOPDashboardItem{}
	for rows.Next() {
		item, err := scanRoomSOPDashboardItem(rows)
		if err != nil {
			return dashboard.RoomSOPDashboardPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomSOPDashboardPage{}, err
	}
	return dashboard.RoomSOPDashboardPage{
		Items:     items,
		Total:     total,
		TotalPage: pageCount(total, filter.PerPage),
		PerPage:   filter.PerPage,
		Page:      filter.Page,
	}, nil
}

func (s *MySQLStore) RoomSOPDashboardByID(ctx context.Context, corpID int, id int) (dashboard.RoomSOPDashboardItem, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			s.id,
			COALESCE(s.corp_id, 0),
			COALESCE(s.creator_id, 0),
			COALESCE(u.name, ''),
			COALESCE(s.name, ''),
			COALESCE(s.setting, ''),
			COALESCE(s.room_ids, ''),
			COALESCE(s.state, 0),
			s.created_at,
			s.updated_at
		FROM mc_room_sop s
		LEFT JOIN mc_user u ON u.id = s.creator_id AND u.deleted_at IS NULL
		WHERE s.id = ? AND s.corp_id = ?
		LIMIT 1
	`, id, corpID)
	item, err := scanRoomSOPDashboardItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.RoomSOPDashboardItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomSOPDashboardItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) CreateRoomSOPDashboard(ctx context.Context, values dashboard.RoomSOPDashboardWrite) (int, error) {
	name := strings.TrimSpace(values.Name)
	if name == "" {
		name = "未命名群SOP"
	}
	setting := "[]"
	if values.HasSetting {
		setting = values.SettingRaw
	}
	roomIDs := "[]"
	if values.HasRoomIDs {
		roomIDs = values.RoomIDsRaw
	}
	state := 1
	if values.HasState {
		state = values.State
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_room_sop (corp_id, creator_id, name, setting, room_ids, state, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, values.CorpID, values.CreatorID, name, setting, roomIDs, state)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func (s *MySQLStore) UpdateRoomSOPDashboard(ctx context.Context, corpID int, id int, values dashboard.RoomSOPDashboardWrite) (bool, error) {
	setSQL := []string{"updated_at = NOW()"}
	args := []any{}
	if values.HasName {
		setSQL = append(setSQL, "name = ?")
		args = append(args, strings.TrimSpace(values.Name))
	}
	if values.HasSetting {
		setSQL = append(setSQL, "setting = ?")
		args = append(args, values.SettingRaw)
	}
	if values.HasRoomIDs {
		setSQL = append(setSQL, "room_ids = ?")
		args = append(args, values.RoomIDsRaw)
	}
	if values.HasState {
		setSQL = append(setSQL, "state = ?")
		args = append(args, values.State)
	}
	args = append(args, id, corpID)
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_sop
		SET `+strings.Join(setSQL, ", ")+`
		WHERE id = ? AND corp_id = ?
	`, args...)
	return rowsAffectedBool(result, err)
}

func (s *MySQLStore) UpdateRoomSOPDashboardRooms(ctx context.Context, corpID int, id int, roomIDsRaw string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_sop
		SET room_ids = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ?
	`, roomIDsRaw, id, corpID)
	return rowsAffectedBool(result, err)
}

func (s *MySQLStore) UpdateRoomSOPDashboardState(ctx context.Context, corpID int, id int, state int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_sop
		SET state = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ?
	`, state, id, corpID)
	return rowsAffectedBool(result, err)
}

func (s *MySQLStore) DeleteRoomSOPDashboard(ctx context.Context, corpID int, id int) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM mc_room_sop_log WHERE corp_id = ? AND room_sop_id = ?", corpID, id); err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM mc_room_sop WHERE corp_id = ? AND id = ?", corpID, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return affected > 0, nil
}

func scanContactSOPDashboardItem(scanner mediumScanner) (dashboard.ContactSOPDashboardItem, error) {
	var item dashboard.ContactSOPDashboardItem
	var creatorName, name, setting, employeeIDs, contactIDs sql.NullString
	var createdAt, updatedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&item.CorpID,
		&item.CreatorID,
		&creatorName,
		&name,
		&setting,
		&employeeIDs,
		&contactIDs,
		&item.State,
		&createdAt,
		&updatedAt,
	); err != nil {
		return dashboard.ContactSOPDashboardItem{}, err
	}
	item.CreatorName = nullString(creatorName)
	item.Name = nullString(name)
	item.SettingRaw = nullString(setting)
	item.EmployeeIDsRaw = nullString(employeeIDs)
	item.ContactIDsRaw = nullString(contactIDs)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanRoomSOPDashboardItem(scanner mediumScanner) (dashboard.RoomSOPDashboardItem, error) {
	var item dashboard.RoomSOPDashboardItem
	var creatorName, name, setting, roomIDs sql.NullString
	var createdAt, updatedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&item.CorpID,
		&item.CreatorID,
		&creatorName,
		&name,
		&setting,
		&roomIDs,
		&item.State,
		&createdAt,
		&updatedAt,
	); err != nil {
		return dashboard.RoomSOPDashboardItem{}, err
	}
	item.CreatorName = nullString(creatorName)
	item.Name = nullString(name)
	item.SettingRaw = nullString(setting)
	item.RoomIDsRaw = nullString(roomIDs)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func (s *MySQLStore) ContactSOPTips(ctx context.Context, employeeID int, contactID int) ([]dashboard.ContactSOPItem, error) {
	employee, contact, found, err := s.contactSOPEmployeeContact(ctx, employeeID, contactID)
	if err != nil || !found {
		return []dashboard.ContactSOPItem{}, err
	}
	return s.contactSOPItems(ctx, employeeID, `
		log.corp_id = ?
		AND log.employee = ?
		AND log.contact = ?
		AND contact.id = ?
	`, employee.CorpID, employee.WXUserID, contact.WXExternalUserID, contact.ID)
}

func (s *MySQLStore) ContactSOPInfo(ctx context.Context, employeeID int, corpID int, id int) (dashboard.ContactSOPItem, bool, error) {
	items, err := s.contactSOPItems(ctx, employeeID, "log.corp_id = ? AND log.id = ?", corpID, id)
	if err != nil {
		return dashboard.ContactSOPItem{}, false, err
	}
	if len(items) > 0 {
		return items[0], true, nil
	}
	items, err = s.contactSOPItems(ctx, employeeID, "log.corp_id = ? AND log.contact_sop_id = ?", corpID, id)
	if err != nil {
		return dashboard.ContactSOPItem{}, false, err
	}
	if len(items) == 0 {
		return dashboard.ContactSOPItem{}, false, nil
	}
	return items[0], true, nil
}

func (s *MySQLStore) RoomSOPInfo(ctx context.Context, employeeID int, id int) (dashboard.RoomSOPItem, bool, error) {
	items, err := s.roomSOPItems(ctx, employeeID, "log.id = ?", id)
	if err != nil {
		return dashboard.RoomSOPItem{}, false, err
	}
	if len(items) > 0 {
		return items[0], true, nil
	}
	items, err = s.roomSOPItems(ctx, employeeID, "log.room_sop_id = ?", id)
	if err != nil {
		return dashboard.RoomSOPItem{}, false, err
	}
	if len(items) == 0 {
		return dashboard.RoomSOPItem{}, false, nil
	}
	return items[0], true, nil
}

func (s *MySQLStore) MarkRoomSOPDone(ctx context.Context, employeeID int, id int) (bool, error) {
	employee, found, err := s.sidebarEmployeeIdentity(ctx, employeeID)
	if err != nil || !found {
		return false, err
	}
	updated, err := s.markRoomSOPDone(ctx, employee, "id = ?", id)
	if err != nil || updated {
		return updated, err
	}
	return s.markRoomSOPDone(ctx, employee, "room_sop_id = ?", id)
}

type contactSOPEmployee struct {
	ID       int
	CorpID   int
	WXUserID string
}

type contactSOPContact struct {
	ID               int
	WXExternalUserID string
}

func (s *MySQLStore) contactSOPEmployeeContact(ctx context.Context, employeeID int, contactID int) (contactSOPEmployee, contactSOPContact, bool, error) {
	employee, found, err := s.sidebarEmployeeIdentity(ctx, employeeID)
	if err != nil || !found {
		return contactSOPEmployee{}, contactSOPContact{}, false, err
	}

	var contact contactSOPContact
	var wxExternalUserID sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT id, wx_external_userid
		FROM mc_work_contact
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, contactID, employee.CorpID).Scan(&contact.ID, &wxExternalUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return contactSOPEmployee{}, contactSOPContact{}, false, nil
	}
	if err != nil {
		return contactSOPEmployee{}, contactSOPContact{}, false, err
	}
	contact.WXExternalUserID = nullString(wxExternalUserID)
	if contact.WXExternalUserID == "" {
		return contactSOPEmployee{}, contactSOPContact{}, false, nil
	}
	return employee, contact, true, nil
}

func (s *MySQLStore) sidebarEmployeeIdentity(ctx context.Context, employeeID int) (contactSOPEmployee, bool, error) {
	var employee contactSOPEmployee
	var wxUserID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, wx_user_id
		FROM mc_work_employee
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, employeeID).Scan(&employee.ID, &employee.CorpID, &wxUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return contactSOPEmployee{}, false, nil
	}
	if err != nil {
		return contactSOPEmployee{}, false, err
	}
	employee.WXUserID = nullString(wxUserID)
	if employee.WXUserID == "" {
		return contactSOPEmployee{}, false, nil
	}
	return employee, true, nil
}

func (s *MySQLStore) contactSOPItems(ctx context.Context, employeeID int, predicate string, args ...any) ([]dashboard.ContactSOPItem, error) {
	queryArgs := make([]any, 0, len(args)+1)
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, employeeID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			log.id,
			COALESCE(log.contact_sop_id, 0),
			COALESCE(user.name, ''),
			COALESCE(log.task, ''),
			log.created_at,
			contact.id,
			COALESCE(contact.name, ''),
			COALESCE(contact.avatar, ''),
			COALESCE(contact.wx_external_userid, ''),
			contact.updated_at
		FROM mc_contact_sop_log AS log
		LEFT JOIN mc_contact_sop AS sop ON sop.id = log.contact_sop_id AND sop.corp_id = log.corp_id
		LEFT JOIN mc_user AS user ON user.id = sop.creator_id AND user.deleted_at IS NULL
		LEFT JOIN mc_work_contact AS contact ON contact.wx_external_userid = log.contact AND contact.corp_id = log.corp_id AND contact.deleted_at IS NULL
		WHERE `+predicate+`
		  AND EXISTS (
			SELECT 1
			FROM mc_work_employee AS employee
			WHERE employee.id = ? AND employee.corp_id = log.corp_id AND employee.wx_user_id = log.employee AND employee.deleted_at IS NULL
		  )
		ORDER BY log.created_at DESC, log.id DESC
	`, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]dashboard.ContactSOPItem, 0)
	for rows.Next() {
		var item dashboard.ContactSOPItem
		var createdAt, contactUpdatedAt sql.NullTime
		var creator, task, contactName, contactAvatar, wxExternalUserID sql.NullString
		if err := rows.Scan(
			&item.ID,
			&item.ContactSOPID,
			&creator,
			&task,
			&createdAt,
			&item.Contact.ID,
			&contactName,
			&contactAvatar,
			&wxExternalUserID,
			&contactUpdatedAt,
		); err != nil {
			return nil, err
		}
		item.Creator = nullString(creator)
		item.TaskRaw = nullString(task)
		item.Time = formatTime(createdAt)
		item.TipTime = contactSOPTipTime(createdAt)
		item.Contact.Name = nullString(contactName)
		item.Contact.Avatar = nullString(contactAvatar)
		item.Contact.WXExternalUserID = nullString(wxExternalUserID)
		item.Contact.UpdatedAt = formatTime(contactUpdatedAt)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *MySQLStore) roomSOPItems(ctx context.Context, employeeID int, predicate string, args ...any) ([]dashboard.RoomSOPItem, error) {
	employee, found, err := s.sidebarEmployeeIdentity(ctx, employeeID)
	if err != nil || !found {
		return []dashboard.RoomSOPItem{}, err
	}

	queryArgs := make([]any, 0, len(args)+2)
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, employee.CorpID, employee.WXUserID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			log.id,
			COALESCE(log.room_sop_id, 0),
			COALESCE(user.name, ''),
			COALESCE(log.task, ''),
			log.created_at,
			COALESCE(log.state, 0),
			COALESCE(room.id, 0),
			COALESCE(room.name, ''),
			COALESCE(room.wx_chat_id, ''),
			room.create_time
		FROM mc_room_sop_log AS log
		LEFT JOIN mc_room_sop AS sop ON sop.id = log.room_sop_id
		LEFT JOIN mc_user AS user ON user.id = sop.creator_id AND user.deleted_at IS NULL
		LEFT JOIN mc_work_room AS room ON room.id = log.room_id AND room.corp_id = log.corp_id AND room.deleted_at IS NULL
		WHERE `+predicate+`
		  AND log.corp_id = ?
		  AND log.employee = ?
		ORDER BY log.created_at DESC, log.id DESC
	`, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]dashboard.RoomSOPItem, 0)
	for rows.Next() {
		var item dashboard.RoomSOPItem
		var creator, task, roomName, wxChatID sql.NullString
		var createdAt, roomCreateTime sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.RoomSOPID,
			&creator,
			&task,
			&createdAt,
			&item.State,
			&item.Room.ID,
			&roomName,
			&wxChatID,
			&roomCreateTime,
		); err != nil {
			return nil, err
		}
		item.Creator = nullString(creator)
		item.TaskRaw = nullString(task)
		item.Time = formatTime(createdAt)
		item.Room.Name = nullString(roomName)
		item.Room.WXChatID = nullString(wxChatID)
		item.Room.CreateTime = formatTime(roomCreateTime)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *MySQLStore) markRoomSOPDone(ctx context.Context, employee contactSOPEmployee, predicate string, arg any) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_sop_log
		SET state = 1, updated_at = NOW()
		WHERE `+predicate+`
		  AND corp_id = ?
		  AND employee = ?
	`, arg, employee.CorpID, employee.WXUserID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func contactSOPTipTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("15:04")
}

func nullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullJSONString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullInt64(value sql.NullInt64) int {
	if !value.Valid {
		return 0
	}
	return int(value.Int64)
}

func intsToArgs(values []int) []any {
	args := make([]any, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	return args
}

func pageCount(total int, perPage int) int {
	if total <= 0 || perPage <= 0 {
		return 0
	}
	return (total + perPage - 1) / perPage
}

func formatPercentRatio(value int) string {
	return strconv.FormatFloat(float64(value)/100, 'f', 2, 64)
}

func corpListWhere(filter dashboard.CorpListFilter) (string, []any) {
	clauses := []string{"WHERE tenant_id = ? AND deleted_at IS NULL"}
	args := []any{filter.TenantID}
	if filter.CorpName != "" {
		clauses = append(clauses, "name LIKE ?")
		args = append(args, "%"+filter.CorpName+"%")
	}
	if !filter.SuperAdmin {
		for _, corpID := range filter.CorpIDs {
			args = append(args, corpID)
		}
		clauses = append(clauses, "id IN ("+placeholders(len(filter.CorpIDs))+")")
	}
	return strings.Join(clauses, " AND "), args
}

func placeholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}

func scanIntColumn(rows *sql.Rows) ([]int, error) {
	var values []int
	for rows.Next() {
		var value int
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *MySQLStore) countRows(ctx context.Context, query string, args ...any) (int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func nonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func rollbackQuietly(tx *sql.Tx) {
	_ = tx.Rollback()
}

func idsFromMenuPath(path string) []int {
	if path == "" {
		return []int{}
	}
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r == '#' || r == '-'
	})
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}
