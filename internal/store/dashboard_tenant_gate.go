package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

type dashboardTenantAccessRow interface {
	Scan(dest ...any) error
}

type dashboardTenantAccessQueryRowFunc func(ctx context.Context, query string, args ...any) dashboardTenantAccessRow

func (s *MySQLStore) DashboardTenantAccess(ctx context.Context, tenantID int, now time.Time) (dashboard.DashboardTenantAccess, error) {
	if tenantID <= 0 {
		return dashboard.DashboardTenantAccess{TenantID: tenantID, Reason: dashboard.DashboardTenantAccessReasonTenantMissing}, nil
	}
	queryRow := s.dashboardTenantAccessQueryRow
	if queryRow == nil && s.db != nil {
		queryRow = func(ctx context.Context, query string, args ...any) dashboardTenantAccessRow {
			return s.db.QueryRowContext(ctx, query, args...)
		}
	}
	if queryRow == nil {
		return dashboard.DashboardTenantAccess{}, errors.New("dashboard tenant access store is unavailable")
	}
	return evaluateDashboardTenantAccessWithQueryRow(ctx, tenantID, now, queryRow)
}

func (s *MySQLStore) dashboardTenantAccessTx(ctx context.Context, tx *sql.Tx, tenantID int, now time.Time) (dashboard.DashboardTenantAccess, error) {
	if tx == nil {
		return dashboard.DashboardTenantAccess{}, errors.New("dashboard tenant access transaction is unavailable")
	}
	return evaluateDashboardTenantAccessWithQueryRow(ctx, tenantID, now, func(ctx context.Context, query string, args ...any) dashboardTenantAccessRow {
		return tx.QueryRowContext(ctx, query, args...)
	})
}

func evaluateDashboardTenantAccessWithQueryRow(ctx context.Context, tenantID int, now time.Time, queryRow dashboardTenantAccessQueryRowFunc) (dashboard.DashboardTenantAccess, error) {
	if tenantID <= 0 {
		return dashboard.DashboardTenantAccess{TenantID: tenantID, Reason: dashboard.DashboardTenantAccessReasonTenantMissing}, nil
	}

	var (
		actualTenantID, tenantStatus                           int
		packageID, subscriptionID                              int64
		packageCode, packageStartsAt, packageExpiresAt         string
		packageLimitsJSON                                      string
		packageStatus, subscriptionCancelAtPeriodEnd           int
		subscriptionStatus, trialEndsAt, periodEndsAt, graceAt string
	)
	err := queryRow(ctx, `
		SELECT
			t.id, COALESCE(t.status, 0),
			COALESCE(p.id, 0), COALESCE(p.package_code, ''), COALESCE(p.status, 0),
			COALESCE(DATE_FORMAT(p.starts_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(p.expires_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(CAST(p.limits_json AS CHAR), ''),
			COALESCE(sub.id, 0), COALESCE(sub.status, ''),
			COALESCE(DATE_FORMAT(sub.trial_ends_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(sub.current_period_ends_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(sub.grace_ends_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(sub.cancel_at_period_end, 0)
		FROM mc_tenant t
		LEFT JOIN mochat_go_saas_tenant_packages p
			ON p.tenant_id = t.id AND p.deleted_at IS NULL
		LEFT JOIN mochat_go_saas_subscriptions sub
			ON sub.tenant_id = t.id AND sub.deleted_at IS NULL
		WHERE t.id = ? AND t.deleted_at IS NULL
		LIMIT 1
	`, tenantID).Scan(
		&actualTenantID, &tenantStatus,
		&packageID, &packageCode, &packageStatus, &packageStartsAt, &packageExpiresAt, &packageLimitsJSON,
		&subscriptionID, &subscriptionStatus, &trialEndsAt, &periodEndsAt, &graceAt, &subscriptionCancelAtPeriodEnd,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.DashboardTenantAccess{TenantID: tenantID, Reason: dashboard.DashboardTenantAccessReasonTenantMissing}, nil
	}
	if err != nil {
		return dashboard.DashboardTenantAccess{}, err
	}
	startsAt, startsOK := parseDashboardTenantAccessTime(packageStartsAt)
	expiresAt, expiresOK := parseDashboardTenantAccessTime(packageExpiresAt)
	if !startsOK || !expiresOK {
		return dashboard.DashboardTenantAccess{TenantID: actualTenantID, Reason: dashboard.DashboardTenantAccessReasonPackageInvalid}, nil
	}
	snapshot := dashboard.DashboardTenantAccessSnapshot{
		TenantID:          actualTenantID,
		TenantFound:       true,
		TenantStatus:      tenantStatus,
		PackageFound:      packageID > 0,
		PackageCode:       packageCode,
		PackageStatus:     packageStatus,
		PackageStartsAt:   startsAt,
		PackageExpiresAt:  expiresAt,
		PackageLimitsJSON: packageLimitsJSON,
		SubscriptionFound: subscriptionID > 0,
		Subscription: dashboard.SaaSAdminSubscription{
			Status:              subscriptionStatus,
			TrialEndsAt:         trialEndsAt,
			CurrentPeriodEndsAt: periodEndsAt,
			GraceEndsAt:         graceAt,
			CancelAtPeriodEnd:   subscriptionCancelAtPeriodEnd == 1,
		},
	}
	return dashboard.EvaluateDashboardTenantAccess(snapshot, now), nil
}

func parseDashboardTenantAccessTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, true
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", raw, time.Local)
	return parsed, err == nil
}
