package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

type saasServiceAccountScanner interface {
	Scan(dest ...any) error
}

type saasServiceAccountQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (s *MySQLStore) SaaSServiceAccounts(ctx context.Context, options dashboard.SaaSServiceAccountOptions) ([]dashboard.SaaSServiceAccount, error) {
	if options.Limit <= 0 || options.Limit > 500 {
		options.Limit = 100
	}
	where := []string{"t.deleted_at IS NULL"}
	args := make([]any, 0, 8)
	if options.TenantID > 0 {
		where = append(where, "a.tenant_id = ?")
		args = append(args, options.TenantID)
	}
	if options.Status != "" && options.Status != dashboard.SaaSServiceAccountStatusAll {
		where = append(where, "a.status = ?")
		args = append(args, options.Status)
	}
	if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
		where = append(where, "(a.code LIKE ? OR a.name LIKE ? OR a.description LIKE ? OR t.name LIKE ? OR CAST(a.id AS CHAR) = ? OR CAST(a.tenant_id AS CHAR) = ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like, like, like, keyword, keyword)
	}
	args = append(args, options.Limit)
	rows, err := s.db.QueryContext(ctx, saasServiceAccountSelectSQL()+`
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY a.status ASC, a.id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSServiceAccount, 0)
	ids := make([]int64, 0)
	for rows.Next() {
		item, err := scanSaaSServiceAccount(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		ids = append(ids, item.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	keys, err := loadSaaSServiceAccountKeys(ctx, s.db, ids)
	if err != nil {
		return nil, err
	}
	routeUsage, err := loadSaaSServiceAccountRouteUsage(ctx, s.db, ids)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Keys = keys[items[index].ID]
		items[index].TodayRoutes = routeUsage[items[index].ID]
	}
	return items, nil
}

func (s *MySQLStore) SaaSServiceAccountKeyForApproval(ctx context.Context, serviceAccountID int64, keyID int64) (dashboard.SaaSServiceAccount, dashboard.SaaSServiceAccountKey, error) {
	account, err := saasServiceAccountByID(ctx, s.db, serviceAccountID, false)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSServiceAccount{}, dashboard.SaaSServiceAccountKey{}, dashboard.NewSaaSAdminNotFound("服务账号不存在")
	}
	if err != nil {
		return dashboard.SaaSServiceAccount{}, dashboard.SaaSServiceAccountKey{}, err
	}
	key, err := saasServiceAccountKeyByID(ctx, s.db, keyID, false)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && key.ServiceAccountID != serviceAccountID) {
		return dashboard.SaaSServiceAccount{}, dashboard.SaaSServiceAccountKey{}, dashboard.NewSaaSAdminNotFound("API Key 不存在")
	}
	if err != nil {
		return dashboard.SaaSServiceAccount{}, dashboard.SaaSServiceAccountKey{}, err
	}
	return account, key, nil
}

func (s *MySQLStore) SaaSServiceAccountUsage(ctx context.Context, options dashboard.SaaSServiceAccountUsageOptions) (dashboard.SaaSServiceAccountUsageReport, error) {
	if options.Days <= 0 || options.Days > dashboard.DefaultSaaSServiceAccountUsageRetentionDays {
		options.Days = dashboard.DefaultSaaSServiceAccountUsageHistoryDays
	}
	if options.Limit <= 0 || options.Limit > 100 {
		options.Limit = 10
	}
	report := dashboard.SaaSServiceAccountUsageReport{DateFrom: options.DateFrom, DateTo: options.DateTo}
	if err := s.db.QueryRowContext(ctx, `
		SELECT service_account_usage_retention_days
		FROM mochat_go_saas_compliance_policies WHERE id = 1
	`).Scan(&report.RetentionDays); err != nil {
		return dashboard.SaaSServiceAccountUsageReport{}, err
	}
	where, args := saasServiceAccountUsageWhere(options, true)
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(u.request_count), 0), COALESCE(SUM(u.rejected_count), 0),
			COUNT(DISTINCT u.service_account_id),
			COUNT(DISTINCT CASE WHEN u.rejected_count > 0 THEN u.service_account_id ELSE NULL END)
		FROM mochat_go_saas_service_account_usage_daily u
		INNER JOIN mochat_go_saas_service_accounts a ON a.id = u.service_account_id
		INNER JOIN mc_tenant t ON t.id = a.tenant_id AND t.deleted_at IS NULL
		WHERE `+where, args...).Scan(
		&report.RequestCount, &report.RejectedCount, &report.ActiveAccountCount, &report.LimitedAccountCount,
	); err != nil {
		return dashboard.SaaSServiceAccountUsageReport{}, err
	}
	alertWhere := []string{"deleted_at IS NULL", "status = ?", "metric = ?", "alert_type IN (?, ?)"}
	alertArgs := []any{
		dashboard.SaaSAlertStatusOpen,
		dashboard.SaaSEventMetricServiceAccountUsage,
		dashboard.SaaSAlertTypeServiceAccountUsageWarning,
		dashboard.SaaSAlertTypeServiceAccountRejectionWarning,
	}
	if options.TenantID > 0 {
		alertWhere = append(alertWhere, "tenant_id = ?")
		alertArgs = append(alertArgs, options.TenantID)
	}
	if options.ServiceAccountID > 0 {
		alertWhere = append(alertWhere, "period_key = ?")
		alertArgs = append(alertArgs, dashboard.SaaSServiceAccountUsageAlertPeriodKey(options.ServiceAccountID))
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_saas_alerts WHERE `+strings.Join(alertWhere, " AND "), alertArgs...).Scan(&report.OpenAlertCount); err != nil {
		return dashboard.SaaSServiceAccountUsageReport{}, err
	}
	metadataWhere, metadataArgs := saasServiceAccountUsageWhere(options, false)
	var oldest sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), MIN(DATE_FORMAT(u.usage_date, '%Y-%m-%d'))
		FROM mochat_go_saas_service_account_usage_daily u
		INNER JOIN mochat_go_saas_service_accounts a ON a.id = u.service_account_id
		INNER JOIN mc_tenant t ON t.id = a.tenant_id AND t.deleted_at IS NULL
		WHERE `+metadataWhere, metadataArgs...).Scan(&report.StoredRowCount, &oldest); err != nil {
		return dashboard.SaaSServiceAccountUsageReport{}, err
	}
	report.OldestUsageDate = oldest.String

	dailyRows, err := s.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(u.usage_date, '%Y-%m-%d'), SUM(u.request_count), SUM(u.rejected_count), COUNT(DISTINCT u.service_account_id)
		FROM mochat_go_saas_service_account_usage_daily u
		INNER JOIN mochat_go_saas_service_accounts a ON a.id = u.service_account_id
		INNER JOIN mc_tenant t ON t.id = a.tenant_id AND t.deleted_at IS NULL
		WHERE `+where+`
		GROUP BY u.usage_date ORDER BY u.usage_date ASC
	`, args...)
	if err != nil {
		return dashboard.SaaSServiceAccountUsageReport{}, err
	}
	for dailyRows.Next() {
		var item dashboard.SaaSServiceAccountUsageDaily
		if err := dailyRows.Scan(&item.UsageDate, &item.RequestCount, &item.RejectedCount, &item.ActiveAccountCount); err != nil {
			dailyRows.Close()
			return dashboard.SaaSServiceAccountUsageReport{}, err
		}
		report.Daily = append(report.Daily, item)
	}
	if err := dailyRows.Close(); err != nil {
		return dashboard.SaaSServiceAccountUsageReport{}, err
	}
	if err := dailyRows.Err(); err != nil {
		return dashboard.SaaSServiceAccountUsageReport{}, err
	}

	routeArgs := append(append([]any(nil), args...), options.Limit)
	routeRows, err := s.db.QueryContext(ctx, `
		SELECT u.route_key, SUM(u.request_count), SUM(u.rejected_count), COUNT(DISTINCT u.service_account_id), MAX(u.last_used_at)
		FROM mochat_go_saas_service_account_usage_daily u
		INNER JOIN mochat_go_saas_service_accounts a ON a.id = u.service_account_id
		INNER JOIN mc_tenant t ON t.id = a.tenant_id AND t.deleted_at IS NULL
		WHERE `+where+`
		GROUP BY u.route_key
		ORDER BY SUM(u.request_count) DESC, SUM(u.rejected_count) DESC, u.route_key ASC
		LIMIT ?
	`, routeArgs...)
	if err != nil {
		return dashboard.SaaSServiceAccountUsageReport{}, err
	}
	for routeRows.Next() {
		var item dashboard.SaaSServiceAccountUsageRoute
		var lastUsedAt sql.NullTime
		if err := routeRows.Scan(&item.RouteKey, &item.RequestCount, &item.RejectedCount, &item.ActiveAccountCount, &lastUsedAt); err != nil {
			routeRows.Close()
			return dashboard.SaaSServiceAccountUsageReport{}, err
		}
		item.LastUsedAt = formatTime(lastUsedAt)
		report.Routes = append(report.Routes, item)
	}
	if err := routeRows.Close(); err != nil {
		return dashboard.SaaSServiceAccountUsageReport{}, err
	}
	if err := routeRows.Err(); err != nil {
		return dashboard.SaaSServiceAccountUsageReport{}, err
	}

	accountArgs := append(append([]any(nil), args...), options.Limit)
	accountRows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.code, a.name, a.tenant_id, t.name, SUM(u.request_count), SUM(u.rejected_count),
			COUNT(DISTINCT u.route_key), MAX(u.last_used_at)
		FROM mochat_go_saas_service_account_usage_daily u
		INNER JOIN mochat_go_saas_service_accounts a ON a.id = u.service_account_id
		INNER JOIN mc_tenant t ON t.id = a.tenant_id AND t.deleted_at IS NULL
		WHERE `+where+`
		GROUP BY a.id, a.code, a.name, a.tenant_id, t.name
		ORDER BY SUM(u.request_count) DESC, SUM(u.rejected_count) DESC, a.id DESC
		LIMIT ?
	`, accountArgs...)
	if err != nil {
		return dashboard.SaaSServiceAccountUsageReport{}, err
	}
	for accountRows.Next() {
		var item dashboard.SaaSServiceAccountUsageAccount
		var lastUsedAt sql.NullTime
		if err := accountRows.Scan(
			&item.ServiceAccountID, &item.ServiceAccountCode, &item.ServiceAccountName, &item.TenantID, &item.TenantName,
			&item.RequestCount, &item.RejectedCount, &item.RouteCount, &lastUsedAt,
		); err != nil {
			accountRows.Close()
			return dashboard.SaaSServiceAccountUsageReport{}, err
		}
		item.LastUsedAt = formatTime(lastUsedAt)
		report.Accounts = append(report.Accounts, item)
	}
	if err := accountRows.Close(); err != nil {
		return dashboard.SaaSServiceAccountUsageReport{}, err
	}
	if err := accountRows.Err(); err != nil {
		return dashboard.SaaSServiceAccountUsageReport{}, err
	}
	return report, nil
}

func saasServiceAccountUsageWhere(options dashboard.SaaSServiceAccountUsageOptions, includeDate bool) (string, []any) {
	where := []string{"1 = 1"}
	args := make([]any, 0, 4)
	if includeDate {
		where = append(where, "u.usage_date BETWEEN ? AND ?")
		args = append(args, options.DateFrom, options.DateTo)
	}
	if options.TenantID > 0 {
		where = append(where, "a.tenant_id = ?")
		args = append(args, options.TenantID)
	}
	if options.ServiceAccountID > 0 {
		where = append(where, "u.service_account_id = ?")
		args = append(args, options.ServiceAccountID)
	}
	return strings.Join(where, " AND "), args
}

func (s *MySQLStore) SaaSServiceAccountCreateTarget(ctx context.Context, tenantID int, code string) (dashboard.SaaSServiceAccountCreateTarget, error) {
	if tenantID <= 0 || strings.TrimSpace(code) == "" {
		return dashboard.SaaSServiceAccountCreateTarget{}, dashboard.NewSaaSAdminBadRequest("tenantId 和 code 必填")
	}
	var target dashboard.SaaSServiceAccountCreateTarget
	target.TenantID = tenantID
	target.Code = strings.TrimSpace(code)
	err := s.db.QueryRowContext(ctx, `
		SELECT name, status
		FROM mc_tenant
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, tenantID).Scan(&target.TenantName, &target.TenantStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSServiceAccountCreateTarget{}, dashboard.NewSaaSAdminNotFound("租户不存在")
	}
	if err != nil {
		return dashboard.SaaSServiceAccountCreateTarget{}, err
	}
	var existing int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mochat_go_saas_service_accounts
		WHERE tenant_id = ? AND code = ?
	`, tenantID, target.Code).Scan(&existing); err != nil {
		return dashboard.SaaSServiceAccountCreateTarget{}, err
	}
	if existing > 0 {
		return dashboard.SaaSServiceAccountCreateTarget{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "该租户的服务账号 code 已存在"}
	}
	return target, nil
}

func (s *MySQLStore) CreateSaaSServiceAccount(ctx context.Context, input dashboard.SaaSServiceAccountCreate) (dashboard.SaaSServiceAccountCreateResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSServiceAccountCreateResult{}, err
	}
	defer tx.Rollback()

	var tenantName string
	var tenantStatus int
	if err := tx.QueryRowContext(ctx, `SELECT name, status FROM mc_tenant WHERE id = ? AND deleted_at IS NULL LIMIT 1 FOR UPDATE`, input.TenantID).Scan(&tenantName, &tenantStatus); errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSServiceAccountCreateResult{}, dashboard.NewSaaSAdminNotFound("租户不存在")
	} else if err != nil {
		return dashboard.SaaSServiceAccountCreateResult{}, err
	}
	if tenantStatus != 1 {
		return dashboard.SaaSServiceAccountCreateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "只能为正常租户创建服务账号"}
	}
	scopesJSON, _ := json.Marshal(input.Scopes)
	allowedCIDRsJSON, _ := json.Marshal(input.AllowedCIDRs)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_service_accounts
			(tenant_id, code, name, description, status, scopes_json, allowed_cidrs_json, rate_limit_per_minute, daily_request_limit,
			 usage_alert_enabled, usage_warning_percent, rejection_warning_count, usage_alert_cooldown_minutes, expires_at,
			 version, created_by, updated_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, NOW(), NOW())
	`, input.TenantID, input.Code, input.Name, input.Description, input.Status, string(scopesJSON), nullableSaaSServiceAccountJSON(input.AllowedCIDRs, allowedCIDRsJSON),
		serviceAccountIntValue(input.RateLimitPerMinute, dashboard.DefaultSaaSServiceAccountRateLimitPerMinute), serviceAccountIntValue(input.DailyRequestLimit, dashboard.DefaultSaaSServiceAccountDailyRequestLimit),
		boolTinyInt(serviceAccountBoolValue(input.UsageAlertEnabled, true)), serviceAccountIntValue(input.UsageWarningPercent, dashboard.DefaultSaaSServiceAccountUsageWarningPercent),
		serviceAccountIntValue(input.RejectionWarningCount, dashboard.DefaultSaaSServiceAccountRejectionWarningCount), serviceAccountIntValue(input.UsageAlertCooldownMinutes, dashboard.DefaultSaaSServiceAccountUsageAlertCooldownMinutes),
		nullableSaaSServiceAccountTime(input.ExpiresAt), input.ActorUserID, input.ActorUserID)
	if isMySQLDuplicateKeyError(err) {
		return dashboard.SaaSServiceAccountCreateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "该租户的服务账号 code 已存在"}
	}
	if err != nil {
		return dashboard.SaaSServiceAccountCreateResult{}, err
	}
	accountID, _ := result.LastInsertId()
	keyID, err := insertSaaSServiceAccountKeyTx(ctx, tx, accountID, input.KeyName, input.KeyExpiresAt, input.Key, input.ActorUserID)
	if err != nil {
		if isMySQLDuplicateKeyError(err) {
			return dashboard.SaaSServiceAccountCreateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "API Key 生成冲突，请重试"}
		}
		return dashboard.SaaSServiceAccountCreateResult{}, err
	}
	after, err := saasServiceAccountByID(ctx, tx, accountID, false)
	if err != nil {
		return dashboard.SaaSServiceAccountCreateResult{}, err
	}
	after.TenantName = tenantName
	after.TenantStatus = tenantStatus
	keys, err := loadSaaSServiceAccountKeys(ctx, tx, []int64{accountID})
	if err != nil {
		return dashboard.SaaSServiceAccountCreateResult{}, err
	}
	after.Keys = keys[accountID]
	var initialKey dashboard.SaaSServiceAccountKey
	for _, item := range after.Keys {
		if item.ID == keyID {
			initialKey = item
			break
		}
	}
	afterJSON, _ := json.Marshal(saasServiceAccountAuditPayload(after))
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: input.TenantID, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionServiceAccountCreate, TargetType: dashboard.SaaSAdminOperationTargetServiceAccount,
		TargetID: strconv.FormatInt(accountID, 10), TargetName: after.Name, AfterJSON: string(afterJSON), Remark: "create SaaS service account and initial API key",
	})
	if err != nil {
		return dashboard.SaaSServiceAccountCreateResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.ActorUserID, operationID); err != nil {
		return dashboard.SaaSServiceAccountCreateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSServiceAccountCreateResult{}, err
	}
	return dashboard.SaaSServiceAccountCreateResult{Account: after, Key: initialKey, OperationID: operationID}, nil
}

func (s *MySQLStore) UpdateSaaSServiceAccount(ctx context.Context, input dashboard.SaaSServiceAccountUpdate) (dashboard.SaaSServiceAccountUpdateResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSServiceAccountUpdateResult{}, err
	}
	defer tx.Rollback()
	before, err := saasServiceAccountByID(ctx, tx, input.ID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSServiceAccountUpdateResult{}, dashboard.NewSaaSAdminNotFound("服务账号不存在")
	}
	if err != nil {
		return dashboard.SaaSServiceAccountUpdateResult{}, err
	}
	if before.Version != input.ExpectedVersion {
		return dashboard.SaaSServiceAccountUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "服务账号版本已变化，请刷新后重试"}
	}
	beforeKeys, err := loadSaaSServiceAccountKeys(ctx, tx, []int64{input.ID})
	if err != nil {
		return dashboard.SaaSServiceAccountUpdateResult{}, err
	}
	before.Keys = beforeKeys[input.ID]
	scopesJSON, _ := json.Marshal(input.Scopes)
	allowedCIDRsJSON, _ := json.Marshal(input.AllowedCIDRs)
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_service_accounts
		SET usage_alert_last_evaluated_at = CASE
				WHEN status <> ?
					OR NOT (expires_at <=> ?)
					OR daily_request_limit <> COALESCE(?, daily_request_limit)
					OR usage_alert_enabled <> COALESCE(?, usage_alert_enabled)
					OR usage_warning_percent <> COALESCE(?, usage_warning_percent)
					OR rejection_warning_count <> COALESCE(?, rejection_warning_count)
					OR usage_alert_cooldown_minutes <> COALESCE(?, usage_alert_cooldown_minutes)
					THEN NULL ELSE usage_alert_last_evaluated_at END,
			usage_alert_last_notified_at = CASE
				WHEN daily_request_limit <> COALESCE(?, daily_request_limit)
					OR usage_alert_enabled <> COALESCE(?, usage_alert_enabled)
					OR usage_warning_percent <> COALESCE(?, usage_warning_percent)
					OR usage_alert_cooldown_minutes <> COALESCE(?, usage_alert_cooldown_minutes)
				THEN NULL ELSE usage_alert_last_notified_at END,
			rejection_alert_last_notified_at = CASE
				WHEN usage_alert_enabled <> COALESCE(?, usage_alert_enabled)
					OR rejection_warning_count <> COALESCE(?, rejection_warning_count)
					OR usage_alert_cooldown_minutes <> COALESCE(?, usage_alert_cooldown_minutes)
				THEN NULL ELSE rejection_alert_last_notified_at END,
			name = ?, description = ?, status = ?, scopes_json = ?, allowed_cidrs_json = ?,
			rate_limit_per_minute = COALESCE(?, rate_limit_per_minute), daily_request_limit = COALESCE(?, daily_request_limit), expires_at = ?,
			usage_alert_enabled = COALESCE(?, usage_alert_enabled), usage_warning_percent = COALESCE(?, usage_warning_percent),
			rejection_warning_count = COALESCE(?, rejection_warning_count), usage_alert_cooldown_minutes = COALESCE(?, usage_alert_cooldown_minutes),
			version = version + 1, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND version = ?
	`, input.Status, nullableSaaSServiceAccountTime(input.ExpiresAt), nullableServiceAccountInt(input.DailyRequestLimit), nullableServiceAccountBool(input.UsageAlertEnabled),
		nullableServiceAccountInt(input.UsageWarningPercent), nullableServiceAccountInt(input.RejectionWarningCount), nullableServiceAccountInt(input.UsageAlertCooldownMinutes),
		nullableServiceAccountInt(input.DailyRequestLimit), nullableServiceAccountBool(input.UsageAlertEnabled), nullableServiceAccountInt(input.UsageWarningPercent), nullableServiceAccountInt(input.UsageAlertCooldownMinutes),
		nullableServiceAccountBool(input.UsageAlertEnabled), nullableServiceAccountInt(input.RejectionWarningCount), nullableServiceAccountInt(input.UsageAlertCooldownMinutes),
		input.Name, input.Description, input.Status, string(scopesJSON), nullableSaaSServiceAccountJSON(input.AllowedCIDRs, allowedCIDRsJSON),
		nullableServiceAccountInt(input.RateLimitPerMinute), nullableServiceAccountInt(input.DailyRequestLimit), nullableSaaSServiceAccountTime(input.ExpiresAt),
		nullableServiceAccountBool(input.UsageAlertEnabled), nullableServiceAccountInt(input.UsageWarningPercent), nullableServiceAccountInt(input.RejectionWarningCount), nullableServiceAccountInt(input.UsageAlertCooldownMinutes),
		input.ActorUserID, input.ID, input.ExpectedVersion)
	if err != nil {
		return dashboard.SaaSServiceAccountUpdateResult{}, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return dashboard.SaaSServiceAccountUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "服务账号版本已变化，请刷新后重试"}
	}
	after, err := saasServiceAccountByID(ctx, tx, input.ID, false)
	if err != nil {
		return dashboard.SaaSServiceAccountUpdateResult{}, err
	}
	keys, err := loadSaaSServiceAccountKeys(ctx, tx, []int64{input.ID})
	if err != nil {
		return dashboard.SaaSServiceAccountUpdateResult{}, err
	}
	after.Keys = keys[input.ID]
	beforeJSON, _ := json.Marshal(saasServiceAccountAuditPayload(before))
	afterJSON, _ := json.Marshal(saasServiceAccountAuditPayload(after))
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: after.TenantID, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionServiceAccountUpdate, TargetType: dashboard.SaaSAdminOperationTargetServiceAccount,
		TargetID: strconv.FormatInt(input.ID, 10), TargetName: after.Name, BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON), Remark: "update SaaS service account",
	})
	if err != nil {
		return dashboard.SaaSServiceAccountUpdateResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.ActorUserID, operationID); err != nil {
		return dashboard.SaaSServiceAccountUpdateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSServiceAccountUpdateResult{}, err
	}
	return dashboard.SaaSServiceAccountUpdateResult{Account: after, OperationID: operationID}, nil
}

func (s *MySQLStore) SaaSServiceAccountForApproval(ctx context.Context, serviceAccountID int64) (dashboard.SaaSServiceAccount, error) {
	account, err := saasServiceAccountByID(ctx, s.db, serviceAccountID, false)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSServiceAccount{}, dashboard.NewSaaSAdminNotFound("服务账号不存在")
	}
	return account, err
}

func (s *MySQLStore) RotateSaaSServiceAccountKey(ctx context.Context, input dashboard.SaaSServiceAccountKeyRotate) (dashboard.SaaSServiceAccountKeyRotateResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, err
	}
	defer tx.Rollback()
	before, err := saasServiceAccountByID(ctx, tx, input.ServiceAccountID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, dashboard.NewSaaSAdminNotFound("服务账号不存在")
	}
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, err
	}
	if before.Version != input.ExpectedVersion {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "服务账号版本已变化，请刷新后重试"}
	}
	beforeKeys, err := loadSaaSServiceAccountKeys(ctx, tx, []int64{input.ServiceAccountID})
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, err
	}
	before.Keys = beforeKeys[input.ServiceAccountID]
	if before.Status != dashboard.SaaSServiceAccountStatusActive || before.TenantStatus != 1 {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "只能为启用且租户正常的服务账号轮换密钥"}
	}
	now := time.Now()
	if accountExpiry, ok := parseStoreDateTime(before.ExpiresAt); ok && !now.Before(accountExpiry) {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "服务账号已过期"}
	}
	if accountExpiry, ok := parseStoreDateTime(before.ExpiresAt); ok {
		keyExpiry, _ := parseStoreDateTime(input.ExpiresAt)
		if keyExpiry.After(accountExpiry) {
			return dashboard.SaaSServiceAccountKeyRotateResult{}, dashboard.NewSaaSAdminBadRequest("新密钥过期时间不能晚于服务账号过期时间")
		}
	}
	var usableKeyCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mochat_go_saas_service_account_keys
		WHERE service_account_id = ? AND status IN ('active', 'retiring')
			AND (expires_at IS NULL OR expires_at > NOW())
			AND (status = 'active' OR retire_at > NOW())
	`, input.ServiceAccountID).Scan(&usableKeyCount); err != nil {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, err
	}
	if usableKeyCount >= 10 {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "当前可用或宽限密钥已达 10 个，请先吊销旧密钥"}
	}
	newKeyID, err := insertSaaSServiceAccountKeyTx(ctx, tx, input.ServiceAccountID, input.Name, input.ExpiresAt, input.Key, input.ActorUserID)
	if isMySQLDuplicateKeyError(err) {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "API Key 生成冲突，请重试"}
	}
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, err
	}
	retireAt := now.Add(time.Duration(input.GraceMinutes) * time.Minute)
	retiringResult, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_service_account_keys
		SET status = 'retiring', retire_at = ?, replaced_by_key_id = ?, version = version + 1, updated_at = NOW()
		WHERE service_account_id = ? AND id <> ? AND status = 'active'
	`, retireAt, newKeyID, input.ServiceAccountID, newKeyID)
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, err
	}
	retiringCount, _ := retiringResult.RowsAffected()
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_service_accounts
		SET version = version + 1, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND version = ?
	`, input.ActorUserID, input.ServiceAccountID, input.ExpectedVersion)
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "服务账号版本已变化，请刷新后重试"}
	}
	after, err := saasServiceAccountByID(ctx, tx, input.ServiceAccountID, false)
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, err
	}
	keys, err := loadSaaSServiceAccountKeys(ctx, tx, []int64{input.ServiceAccountID})
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, err
	}
	after.Keys = keys[input.ServiceAccountID]
	var newKey dashboard.SaaSServiceAccountKey
	for _, item := range after.Keys {
		if item.ID == newKeyID {
			newKey = item
			break
		}
	}
	beforeJSON, _ := json.Marshal(saasServiceAccountAuditPayload(before))
	afterJSON, _ := json.Marshal(map[string]any{"account": saasServiceAccountAuditPayload(after), "newKey": saasServiceAccountKeyAuditPayload(newKey), "retiringKeys": retiringCount, "graceMinutes": input.GraceMinutes})
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: after.TenantID, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionServiceAccountRotate, TargetType: dashboard.SaaSAdminOperationTargetServiceAccountKey,
		TargetID: strconv.FormatInt(newKeyID, 10), TargetName: newKey.Name, BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON), Remark: "rotate SaaS service account API key",
	})
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.ActorUserID, operationID); err != nil {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSServiceAccountKeyRotateResult{}, err
	}
	return dashboard.SaaSServiceAccountKeyRotateResult{Account: after, Key: newKey, RetiringKeys: int(retiringCount), OperationID: operationID}, nil
}

func (s *MySQLStore) RevokeSaaSServiceAccountKey(ctx context.Context, input dashboard.SaaSServiceAccountKeyRevoke) (dashboard.SaaSServiceAccountKeyRevokeResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, err
	}
	defer tx.Rollback()
	account, err := saasServiceAccountByID(ctx, tx, input.ServiceAccountID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, dashboard.NewSaaSAdminNotFound("服务账号不存在")
	}
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, err
	}
	before, err := saasServiceAccountKeyByID(ctx, tx, input.KeyID, true)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && before.ServiceAccountID != input.ServiceAccountID) {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, dashboard.NewSaaSAdminNotFound("API Key 不存在")
	}
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, err
	}
	if before.Version != input.ExpectedVersion {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "API Key 版本已变化，请刷新后重试"}
	}
	if before.Status == dashboard.SaaSServiceAccountKeyStatusRevoked {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "API Key 已吊销"}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_service_account_keys
		SET status = 'revoked', retire_at = NULL, revoked_at = NOW(), revoked_by = ?, version = version + 1, updated_at = NOW()
		WHERE id = ? AND service_account_id = ? AND version = ? AND status <> 'revoked'
	`, input.ActorUserID, input.KeyID, input.ServiceAccountID, input.ExpectedVersion)
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "API Key 版本已变化，请刷新后重试"}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_service_accounts SET version = version + 1, updated_by = ?, updated_at = NOW() WHERE id = ?
	`, input.ActorUserID, input.ServiceAccountID); err != nil {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, err
	}
	afterKey, err := saasServiceAccountKeyByID(ctx, tx, input.KeyID, false)
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, err
	}
	afterAccount, err := saasServiceAccountByID(ctx, tx, input.ServiceAccountID, false)
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, err
	}
	keys, err := loadSaaSServiceAccountKeys(ctx, tx, []int64{input.ServiceAccountID})
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, err
	}
	afterAccount.Keys = keys[input.ServiceAccountID]
	beforeJSON, _ := json.Marshal(saasServiceAccountKeyAuditPayload(before))
	afterJSON, _ := json.Marshal(saasServiceAccountKeyAuditPayload(afterKey))
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: account.TenantID, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionServiceAccountRevoke, TargetType: dashboard.SaaSAdminOperationTargetServiceAccountKey,
		TargetID: strconv.FormatInt(input.KeyID, 10), TargetName: afterKey.Name, BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON), Remark: "revoke SaaS service account API key",
	})
	if err != nil {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.ActorUserID, operationID); err != nil {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSServiceAccountKeyRevokeResult{}, err
	}
	return dashboard.SaaSServiceAccountKeyRevokeResult{Account: afterAccount, Key: afterKey, OperationID: operationID}, nil
}

func (s *MySQLStore) SaaSServiceAccountPrincipalByPrefix(ctx context.Context, prefix string) (dashboard.SaaSServiceAccountPrincipal, bool, error) {
	var item dashboard.SaaSServiceAccountPrincipal
	var scopesRaw, allowedCIDRsRaw []byte
	var accountExpiresAt, keyExpiresAt, keyRetireAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT a.id, a.code, a.name, a.status, a.tenant_id, t.name, t.status, a.scopes_json, a.allowed_cidrs_json, a.expires_at,
			k.id, k.name, k.key_prefix, k.key_hash, k.hash_key_id, k.status, k.expires_at, k.retire_at,
			a.rate_limit_per_minute, a.daily_request_limit
		FROM mochat_go_saas_service_account_keys k
		INNER JOIN mochat_go_saas_service_accounts a ON a.id = k.service_account_id
		INNER JOIN mc_tenant t ON t.id = a.tenant_id AND t.deleted_at IS NULL
		WHERE k.key_prefix = ?
		LIMIT 1
	`, prefix).Scan(
		&item.ServiceAccountID, &item.ServiceAccountCode, &item.ServiceAccountName, &item.ServiceAccountStatus,
		&item.TenantID, &item.TenantName, &item.TenantStatus, &scopesRaw, &allowedCIDRsRaw, &accountExpiresAt,
		&item.KeyID, &item.KeyName, &item.KeyPrefix, &item.KeyHash, &item.KeyHashKeyID, &item.KeyStatus, &keyExpiresAt, &keyRetireAt,
		&item.RateLimitPerMinute, &item.DailyRequestLimit,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSServiceAccountPrincipal{}, false, nil
	}
	if err != nil {
		return dashboard.SaaSServiceAccountPrincipal{}, false, err
	}
	item.Scopes = decodeSaaSServiceAccountStrings(scopesRaw)
	item.AllowedCIDRs = decodeSaaSServiceAccountStrings(allowedCIDRsRaw)
	item.AccountExpiresAt = formatTime(accountExpiresAt)
	item.KeyExpiresAt = formatTime(keyExpiresAt)
	item.KeyRetireAt = formatTime(keyRetireAt)
	return item, true, nil
}

func (s *MySQLStore) SaaSServiceAccountKeyProtectionCounts(ctx context.Context) ([]dashboard.SaaSServiceAccountKeyProtectionCount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT COALESCE(NULLIF(k.hash_key_id, ''), 'legacy-jwt'), COUNT(*),
			SUM(CASE WHEN a.status = 'active' AND t.status = 1
				AND (a.expires_at IS NULL OR a.expires_at > NOW())
				AND (k.expires_at IS NULL OR k.expires_at > NOW())
				AND (k.status = 'active' OR (k.status = 'retiring' AND k.retire_at IS NOT NULL AND k.retire_at > NOW()))
				THEN 1 ELSE 0 END)
		FROM mochat_go_saas_service_account_keys k
		INNER JOIN mochat_go_saas_service_accounts a ON a.id = k.service_account_id
		INNER JOIN mc_tenant t ON t.id = a.tenant_id AND t.deleted_at IS NULL
		GROUP BY COALESCE(NULLIF(k.hash_key_id, ''), 'legacy-jwt')
		ORDER BY COALESCE(NULLIF(k.hash_key_id, ''), 'legacy-jwt') ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSServiceAccountKeyProtectionCount, 0)
	for rows.Next() {
		var item dashboard.SaaSServiceAccountKeyProtectionCount
		if err := rows.Scan(&item.HashKeyID, &item.KeyCount, &item.UsableKeyCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) ConsumeSaaSServiceAccountRequest(ctx context.Context, serviceAccountID int64, keyID int64, clientIP string, routeKey string) (dashboard.SaaSServiceAccountRateLimitState, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSServiceAccountRateLimitState{}, err
	}
	defer tx.Rollback()

	now := time.Now()
	minuteStart := now.Truncate(time.Minute)
	dailyStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	minuteReset := minuteStart.Add(time.Minute)
	dailyReset := dailyStart.AddDate(0, 0, 1)
	var accountStatus string
	var tenantStatus int
	var accountExpiresAt, minuteWindow, dailyWindow sql.NullTime
	var rateLimitPerMinute int
	var dailyRequestLimit, minuteRequestCount, minuteRejectedCount, dailyRequestCount, dailyRejectedCount int64
	err = tx.QueryRowContext(ctx, `
		SELECT a.status, t.status, a.expires_at, a.rate_limit_per_minute, a.daily_request_limit,
			a.minute_window_started_at, a.minute_request_count, a.minute_rejected_count,
			a.daily_window_date, a.daily_request_count, a.daily_rejected_count
		FROM mochat_go_saas_service_accounts a
		INNER JOIN mc_tenant t ON t.id = a.tenant_id AND t.deleted_at IS NULL
		WHERE a.id = ? LIMIT 1 FOR UPDATE
	`, serviceAccountID).Scan(
		&accountStatus, &tenantStatus, &accountExpiresAt, &rateLimitPerMinute, &dailyRequestLimit,
		&minuteWindow, &minuteRequestCount, &minuteRejectedCount,
		&dailyWindow, &dailyRequestCount, &dailyRejectedCount,
	)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (accountStatus != dashboard.SaaSServiceAccountStatusActive || tenantStatus != 1 || (accountExpiresAt.Valid && !now.Before(accountExpiresAt.Time)))) {
		return dashboard.SaaSServiceAccountRateLimitState{}, &dashboard.SaaSServiceAccountAuthError{Status: 401, Message: "invalid API key"}
	}
	if err != nil {
		return dashboard.SaaSServiceAccountRateLimitState{}, err
	}

	var keyStatus string
	var keyExpiresAt, keyRetireAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT status, expires_at, retire_at
		FROM mochat_go_saas_service_account_keys
		WHERE id = ? AND service_account_id = ? LIMIT 1 FOR UPDATE
	`, keyID, serviceAccountID).Scan(&keyStatus, &keyExpiresAt, &keyRetireAt)
	keyUsable := err == nil && (keyStatus == dashboard.SaaSServiceAccountKeyStatusActive || (keyStatus == dashboard.SaaSServiceAccountKeyStatusRetiring && keyRetireAt.Valid && now.Before(keyRetireAt.Time))) && (!keyExpiresAt.Valid || now.Before(keyExpiresAt.Time))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !keyUsable) {
		return dashboard.SaaSServiceAccountRateLimitState{}, &dashboard.SaaSServiceAccountAuthError{Status: 401, Message: "invalid API key"}
	}
	if err != nil {
		return dashboard.SaaSServiceAccountRateLimitState{}, err
	}

	if rateLimitPerMinute <= 0 {
		rateLimitPerMinute = dashboard.DefaultSaaSServiceAccountRateLimitPerMinute
	}
	if !minuteWindow.Valid || !minuteWindow.Time.Equal(minuteStart) {
		minuteRequestCount, minuteRejectedCount = 0, 0
	}
	if !dailyWindow.Valid || dailyWindow.Time.Format("2006-01-02") != dailyStart.Format("2006-01-02") {
		dailyRequestCount, dailyRejectedCount = 0, 0
	}
	limitedBy := ""
	if minuteRequestCount >= int64(rateLimitPerMinute) {
		limitedBy = "minute"
	} else if dailyRequestLimit > 0 && dailyRequestCount >= dailyRequestLimit {
		limitedBy = "daily"
	}
	allowed := limitedBy == ""
	if allowed {
		minuteRequestCount++
		dailyRequestCount++
	} else {
		minuteRejectedCount++
		dailyRejectedCount++
	}
	state := dashboard.SaaSServiceAccountRateLimitState{
		RateLimitPerMinute: rateLimitPerMinute, MinuteRequestCount: minuteRequestCount,
		MinuteRejectedCount: minuteRejectedCount, MinuteResetAt: minuteReset,
		DailyRequestLimit: dailyRequestLimit, DailyRequestCount: dailyRequestCount,
		DailyRejectedCount: dailyRejectedCount, DailyResetAt: dailyReset, LimitedBy: limitedBy,
	}

	if allowed {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_service_accounts
			SET minute_window_started_at = ?, minute_request_count = ?, minute_rejected_count = ?,
				daily_window_date = ?, daily_request_count = ?, daily_rejected_count = ?,
				last_used_at = NOW(), last_used_ip = ?, use_count = use_count + 1
			WHERE id = ?
		`, minuteStart, minuteRequestCount, minuteRejectedCount, dailyStart, dailyRequestCount, dailyRejectedCount, clientIP, serviceAccountID); err != nil {
			return dashboard.SaaSServiceAccountRateLimitState{}, err
		}
		keyResult, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_service_account_keys
			SET last_used_at = NOW(), last_used_ip = ?, use_count = use_count + 1
			WHERE id = ? AND service_account_id = ?
		`, clientIP, keyID, serviceAccountID)
		if err != nil {
			return dashboard.SaaSServiceAccountRateLimitState{}, err
		}
		if affected, _ := keyResult.RowsAffected(); affected != 1 {
			return dashboard.SaaSServiceAccountRateLimitState{}, &dashboard.SaaSServiceAccountAuthError{Status: 401, Message: "invalid API key"}
		}
	} else if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_service_accounts
		SET minute_window_started_at = ?, minute_request_count = ?, minute_rejected_count = ?,
			daily_window_date = ?, daily_request_count = ?, daily_rejected_count = ?
		WHERE id = ?
	`, minuteStart, minuteRequestCount, minuteRejectedCount, dailyStart, dailyRequestCount, dailyRejectedCount, serviceAccountID); err != nil {
		return dashboard.SaaSServiceAccountRateLimitState{}, err
	}
	if err := recordSaaSServiceAccountRouteUsageTx(ctx, tx, serviceAccountID, dailyStart, routeKey, clientIP, allowed); err != nil {
		return dashboard.SaaSServiceAccountRateLimitState{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSServiceAccountRateLimitState{}, err
	}
	if !allowed {
		return state, &dashboard.SaaSServiceAccountRateLimitError{State: state}
	}
	return state, nil
}

func (s *MySQLStore) CleanupSaaSServiceAccountUsage(ctx context.Context, limit int) (dashboard.SaaSServiceAccountUsageCleanupResult, error) {
	if limit <= 0 || limit > 1000000 {
		return dashboard.SaaSServiceAccountUsageCleanupResult{}, errors.New("服务账号用量清理批次必须在 1 到 1000000 之间")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSServiceAccountUsageCleanupResult{}, err
	}
	defer tx.Rollback()
	var result dashboard.SaaSServiceAccountUsageCleanupResult
	if err := tx.QueryRowContext(ctx, `
		SELECT service_account_usage_retention_days
		FROM mochat_go_saas_compliance_policies WHERE id = 1
	`).Scan(&result.RetentionDays); err != nil {
		return dashboard.SaaSServiceAccountUsageCleanupResult{}, err
	}
	if result.RetentionDays < 1 || result.RetentionDays > 3650 {
		return dashboard.SaaSServiceAccountUsageCleanupResult{}, errors.New("服务账号用量保留策略必须在 1 到 3650 天之间")
	}
	if err := tx.QueryRowContext(ctx, `SELECT DATE_FORMAT(DATE_SUB(CURDATE(), INTERVAL ? DAY), '%Y-%m-%d')`, result.RetentionDays-1).Scan(&result.CutoffDate); err != nil {
		return dashboard.SaaSServiceAccountUsageCleanupResult{}, err
	}
	activeHold := `
		SELECT a.id
		FROM mochat_go_saas_service_accounts a
		INNER JOIN mochat_go_saas_legal_holds h ON h.tenant_id = a.tenant_id
		WHERE h.status = 'active' AND h.starts_at <= NOW() AND (h.expires_at IS NULL OR h.expires_at > NOW())
	`
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mochat_go_saas_service_account_usage_daily
		WHERE usage_date < ? AND service_account_id NOT IN (`+activeHold+`)
	`, result.CutoffDate).Scan(&result.EligibleRows); err != nil {
		return dashboard.SaaSServiceAccountUsageCleanupResult{}, err
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mochat_go_saas_service_account_usage_daily
		WHERE usage_date < ? AND service_account_id IN (`+activeHold+`)
	`, result.CutoffDate).Scan(&result.ProtectedRows); err != nil {
		return dashboard.SaaSServiceAccountUsageCleanupResult{}, err
	}
	deleteResult, err := tx.ExecContext(ctx, `
		DELETE FROM mochat_go_saas_service_account_usage_daily
		WHERE usage_date < ? AND service_account_id NOT IN (`+activeHold+`)
		ORDER BY usage_date ASC, id ASC LIMIT ?
	`, result.CutoffDate, limit)
	if err != nil {
		return dashboard.SaaSServiceAccountUsageCleanupResult{}, err
	}
	result.DeletedRows, _ = deleteResult.RowsAffected()
	result.RemainingRows = result.EligibleRows - result.DeletedRows
	if result.RemainingRows < 0 {
		result.RemainingRows = 0
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSServiceAccountUsageCleanupResult{}, err
	}
	return result, nil
}

func recordSaaSServiceAccountRouteUsageTx(ctx context.Context, tx *sql.Tx, serviceAccountID int64, usageDate time.Time, routeKey, clientIP string, allowed bool) error {
	routeKey = strings.TrimSpace(routeKey)
	if routeKey == "" {
		routeKey = "UNKNOWN /"
	}
	if len(routeKey) > 128 {
		routeKey = routeKey[:128]
	}
	requestIncrement, rejectedIncrement := 0, 1
	if allowed {
		requestIncrement, rejectedIncrement = 1, 0
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_service_account_usage_daily
			(service_account_id, usage_date, route_key, request_count, rejected_count, last_used_at, last_used_ip, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE request_count = request_count + VALUES(request_count),
			rejected_count = rejected_count + VALUES(rejected_count), last_used_at = NOW(), last_used_ip = VALUES(last_used_ip), updated_at = NOW()
	`, serviceAccountID, usageDate, routeKey, requestIncrement, rejectedIncrement, clientIP)
	return err
}

func insertSaaSServiceAccountKeyTx(ctx context.Context, tx *sql.Tx, accountID int64, name, expiresAt string, key dashboard.SaaSServiceAccountKeyMaterial, actorUserID int) (int64, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_service_account_keys
			(service_account_id, name, key_prefix, key_hash, hash_key_id, last_four, status, expires_at, version, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'active', ?, 1, ?, NOW(), NOW())
	`, accountID, name, key.Prefix, key.Hash, key.HashKeyID, key.LastFour, nullableSaaSServiceAccountTime(expiresAt), actorUserID)
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	return id, nil
}

func saasServiceAccountByID(ctx context.Context, queryer saasServiceAccountQueryer, id int64, forUpdate bool) (dashboard.SaaSServiceAccount, error) {
	query := saasServiceAccountSelectSQL() + ` WHERE a.id = ? LIMIT 1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanSaaSServiceAccount(queryer.QueryRowContext(ctx, query, id))
}

func saasServiceAccountSelectSQL() string {
	return `
		SELECT a.id, a.tenant_id, t.name, t.status, a.code, a.name, a.description, a.status,
			a.scopes_json, a.allowed_cidrs_json, a.rate_limit_per_minute, a.daily_request_limit,
			a.usage_alert_enabled, a.usage_warning_percent, a.rejection_warning_count, a.usage_alert_cooldown_minutes,
			a.usage_alert_last_evaluated_at, a.usage_alert_last_notified_at, a.rejection_alert_last_notified_at,
			IF(a.minute_window_started_at = TIMESTAMP(DATE_FORMAT(NOW(), '%Y-%m-%d %H:%i:00')), a.minute_request_count, 0),
			IF(a.minute_window_started_at = TIMESTAMP(DATE_FORMAT(NOW(), '%Y-%m-%d %H:%i:00')), a.minute_rejected_count, 0),
			IF(a.daily_window_date = CURDATE(), a.daily_request_count, 0),
			IF(a.daily_window_date = CURDATE(), a.daily_rejected_count, 0),
			a.expires_at, a.last_used_at, a.last_used_ip, a.use_count,
			a.version, a.created_by, a.updated_by, a.created_at, a.updated_at
		FROM mochat_go_saas_service_accounts a
		INNER JOIN mc_tenant t ON t.id = a.tenant_id AND t.deleted_at IS NULL
	`
}

func scanSaaSServiceAccount(scanner saasServiceAccountScanner) (dashboard.SaaSServiceAccount, error) {
	var item dashboard.SaaSServiceAccount
	var scopesRaw, allowedCIDRsRaw []byte
	var usageAlertLastEvaluatedAt, usageAlertLastNotifiedAt, rejectionAlertLastNotifiedAt sql.NullTime
	var expiresAt, lastUsedAt, createdAt, updatedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.TenantID, &item.TenantName, &item.TenantStatus, &item.Code, &item.Name, &item.Description, &item.Status,
		&scopesRaw, &allowedCIDRsRaw, &item.RateLimitPerMinute, &item.DailyRequestLimit,
		&item.UsageAlertEnabled, &item.UsageWarningPercent, &item.RejectionWarningCount, &item.UsageAlertCooldownMinutes,
		&usageAlertLastEvaluatedAt, &usageAlertLastNotifiedAt, &rejectionAlertLastNotifiedAt,
		&item.MinuteRequestCount, &item.MinuteRejectedCount, &item.DailyRequestCount, &item.DailyRejectedCount,
		&expiresAt, &lastUsedAt, &item.LastUsedIP, &item.UseCount,
		&item.Version, &item.CreatedBy, &item.UpdatedBy, &createdAt, &updatedAt,
	); err != nil {
		return dashboard.SaaSServiceAccount{}, err
	}
	item.Scopes = decodeSaaSServiceAccountStrings(scopesRaw)
	item.AllowedCIDRs = decodeSaaSServiceAccountStrings(allowedCIDRsRaw)
	item.UsageAlertLastEvaluatedAt = formatTime(usageAlertLastEvaluatedAt)
	item.UsageAlertLastNotifiedAt = formatTime(usageAlertLastNotifiedAt)
	item.RejectionAlertLastNotifiedAt = formatTime(rejectionAlertLastNotifiedAt)
	item.ExpiresAt = formatTime(expiresAt)
	item.LastUsedAt = formatTime(lastUsedAt)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func loadSaaSServiceAccountRouteUsage(ctx context.Context, queryer saasServiceAccountQueryer, accountIDs []int64) (map[int64][]dashboard.SaaSServiceAccountRouteUsage, error) {
	result := make(map[int64][]dashboard.SaaSServiceAccountRouteUsage, len(accountIDs))
	if len(accountIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(accountIDs))
	for _, id := range accountIDs {
		args = append(args, id)
	}
	rows, err := queryer.QueryContext(ctx, `
		SELECT service_account_id, route_key, request_count, rejected_count, last_used_at, last_used_ip
		FROM mochat_go_saas_service_account_usage_daily
		WHERE usage_date = CURDATE() AND service_account_id IN (`+placeholders(len(accountIDs))+`)
		ORDER BY service_account_id ASC, request_count DESC, rejected_count DESC, route_key ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var accountID int64
		var item dashboard.SaaSServiceAccountRouteUsage
		var lastUsedAt sql.NullTime
		if err := rows.Scan(&accountID, &item.RouteKey, &item.RequestCount, &item.RejectedCount, &lastUsedAt, &item.LastUsedIP); err != nil {
			return nil, err
		}
		item.LastUsedAt = formatTime(lastUsedAt)
		result[accountID] = append(result[accountID], item)
	}
	return result, rows.Err()
}

func loadSaaSServiceAccountKeys(ctx context.Context, queryer saasServiceAccountQueryer, accountIDs []int64) (map[int64][]dashboard.SaaSServiceAccountKey, error) {
	result := make(map[int64][]dashboard.SaaSServiceAccountKey, len(accountIDs))
	if len(accountIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(accountIDs))
	for _, id := range accountIDs {
		args = append(args, id)
	}
	rows, err := queryer.QueryContext(ctx, `
		SELECT id, service_account_id, name, key_prefix, hash_key_id, last_four, status, expires_at, retire_at,
			last_used_at, last_used_ip, use_count, replaced_by_key_id, revoked_at, revoked_by,
			version, created_by, created_at, updated_at
		FROM mochat_go_saas_service_account_keys
		WHERE service_account_id IN (`+placeholders(len(accountIDs))+`)
		ORDER BY service_account_id ASC, id DESC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanSaaSServiceAccountKey(rows)
		if err != nil {
			return nil, err
		}
		result[item.ServiceAccountID] = append(result[item.ServiceAccountID], item)
	}
	return result, rows.Err()
}

func saasServiceAccountKeyByID(ctx context.Context, queryer saasServiceAccountQueryer, id int64, forUpdate bool) (dashboard.SaaSServiceAccountKey, error) {
	query := `
		SELECT id, service_account_id, name, key_prefix, hash_key_id, last_four, status, expires_at, retire_at,
			last_used_at, last_used_ip, use_count, replaced_by_key_id, revoked_at, revoked_by,
			version, created_by, created_at, updated_at
		FROM mochat_go_saas_service_account_keys WHERE id = ? LIMIT 1
	`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanSaaSServiceAccountKey(queryer.QueryRowContext(ctx, query, id))
}

func scanSaaSServiceAccountKey(scanner saasServiceAccountScanner) (dashboard.SaaSServiceAccountKey, error) {
	var item dashboard.SaaSServiceAccountKey
	var expiresAt, retireAt, lastUsedAt, revokedAt, createdAt, updatedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.ServiceAccountID, &item.Name, &item.Prefix, &item.HashKeyID, &item.LastFour, &item.Status,
		&expiresAt, &retireAt, &lastUsedAt, &item.LastUsedIP, &item.UseCount, &item.ReplacedByKeyID,
		&revokedAt, &item.RevokedBy, &item.Version, &item.CreatedBy, &createdAt, &updatedAt,
	); err != nil {
		return dashboard.SaaSServiceAccountKey{}, err
	}
	item.ExpiresAt = formatTime(expiresAt)
	item.RetireAt = formatTime(retireAt)
	item.LastUsedAt = formatTime(lastUsedAt)
	item.RevokedAt = formatTime(revokedAt)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func decodeSaaSServiceAccountStrings(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return []string{}
	}
	return values
}

func nullableSaaSServiceAccountTime(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullableSaaSServiceAccountJSON(values []string, encoded []byte) any {
	if len(values) == 0 {
		return nil
	}
	return string(encoded)
}

func serviceAccountIntValue(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}

func nullableServiceAccountInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func serviceAccountBoolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func nullableServiceAccountBool(value *bool) any {
	if value == nil {
		return nil
	}
	return boolTinyInt(*value)
}

func parseStoreDateTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local)
	return parsed, err == nil
}

func saasServiceAccountAuditPayload(item dashboard.SaaSServiceAccount) map[string]any {
	keys := make([]map[string]any, 0, len(item.Keys))
	for _, key := range item.Keys {
		keys = append(keys, saasServiceAccountKeyAuditPayload(key))
	}
	return map[string]any{
		"id": item.ID, "tenantId": item.TenantID, "code": item.Code, "name": item.Name, "description": item.Description,
		"status": item.Status, "scopes": item.Scopes, "allowedCidrs": item.AllowedCIDRs, "expiresAt": item.ExpiresAt,
		"rateLimitPerMinute": item.RateLimitPerMinute, "dailyRequestLimit": item.DailyRequestLimit,
		"usageAlertEnabled": item.UsageAlertEnabled, "usageWarningPercent": item.UsageWarningPercent,
		"rejectionWarningCount": item.RejectionWarningCount, "usageAlertCooldownMinutes": item.UsageAlertCooldownMinutes,
		"version": item.Version, "keys": keys,
	}
}

func saasServiceAccountKeyAuditPayload(item dashboard.SaaSServiceAccountKey) map[string]any {
	return map[string]any{
		"id": item.ID, "serviceAccountId": item.ServiceAccountID, "name": item.Name, "prefix": item.Prefix,
		"hashKeyId": item.HashKeyID,
		"lastFour":  item.LastFour, "status": item.Status, "expiresAt": item.ExpiresAt, "retireAt": item.RetireAt,
		"replacedByKeyId": item.ReplacedByKeyID, "revokedAt": item.RevokedAt, "version": item.Version,
	}
}
