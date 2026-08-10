package dashboard

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

const DashboardTenantAccessDeniedCode = "TENANT_ACCESS_DENIED"

const (
	DashboardTenantAccessReasonTenantMissing       = "tenant_missing"
	DashboardTenantAccessReasonTenantDisabled      = "tenant_disabled"
	DashboardTenantAccessReasonPackageMissing      = "package_missing"
	DashboardTenantAccessReasonPackageInvalid      = "package_invalid"
	DashboardTenantAccessReasonPackageDisabled     = "package_disabled"
	DashboardTenantAccessReasonPackageNotStarted   = "package_not_started"
	DashboardTenantAccessReasonPackageExpired      = "package_expired"
	DashboardTenantAccessReasonLimitsInvalid       = "limits_invalid"
	DashboardTenantAccessReasonSubscriptionMissing = "subscription_missing"
	DashboardTenantAccessReasonSubscriptionDenied  = "subscription_denied"
)

type DashboardTenantAccess struct {
	TenantID int
	Allowed  bool
	Reason   string
}

type DashboardTenantAccessSnapshot struct {
	TenantID          int
	TenantFound       bool
	TenantStatus      int
	PackageFound      bool
	PackageCode       string
	PackageStatus     int
	PackageStartsAt   time.Time
	PackageExpiresAt  time.Time
	PackageLimitsJSON string
	SubscriptionFound bool
	Subscription      SaaSAdminSubscription
}

type DashboardTenantAccessStore interface {
	DashboardTenantAccess(ctx context.Context, tenantID int, now time.Time) (DashboardTenantAccess, error)
}

func EvaluateDashboardTenantAccess(snapshot DashboardTenantAccessSnapshot, now time.Time) DashboardTenantAccess {
	denied := func(reason string) DashboardTenantAccess {
		return DashboardTenantAccess{TenantID: snapshot.TenantID, Reason: reason}
	}
	if !snapshot.TenantFound {
		return denied(DashboardTenantAccessReasonTenantMissing)
	}
	if snapshot.TenantStatus != 1 {
		return denied(DashboardTenantAccessReasonTenantDisabled)
	}
	if !snapshot.PackageFound {
		return denied(DashboardTenantAccessReasonPackageMissing)
	}
	if strings.TrimSpace(snapshot.PackageCode) == "" {
		return denied(DashboardTenantAccessReasonPackageInvalid)
	}
	if snapshot.PackageStatus != 1 {
		return denied(DashboardTenantAccessReasonPackageDisabled)
	}
	if !snapshot.PackageStartsAt.IsZero() && snapshot.PackageStartsAt.After(now) {
		return denied(DashboardTenantAccessReasonPackageNotStarted)
	}
	if !snapshot.PackageExpiresAt.IsZero() && !snapshot.PackageExpiresAt.After(now) {
		return denied(DashboardTenantAccessReasonPackageExpired)
	}
	if !dashboardTenantPackageLimitsValid(snapshot.PackageLimitsJSON) {
		return denied(DashboardTenantAccessReasonLimitsInvalid)
	}
	if !snapshot.SubscriptionFound {
		return denied(DashboardTenantAccessReasonSubscriptionMissing)
	}
	subscription := snapshot.Subscription
	subscription.TenantStatus = snapshot.TenantStatus
	effectiveStatus := SaaSAdminEffectiveSubscriptionStatus(subscription, now)
	if !SaaSAdminSubscriptionAllowsAccess(effectiveStatus) {
		return denied(DashboardTenantAccessReasonSubscriptionDenied)
	}
	return DashboardTenantAccess{TenantID: snapshot.TenantID, Allowed: true}
}

func dashboardTenantPackageLimitsValid(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &fields) != nil || fields == nil {
		return false
	}
	var limits SaaSAdminPackageLimits
	if json.Unmarshal([]byte(raw), &limits) != nil || !saasAdminPackageLimitsValid(limits) {
		return false
	}
	for _, limit := range saasAdminPackageLimitValues(limits) {
		value, ok := fields[limit.Field]
		if !ok {
			return false
		}
		var number int64
		if json.Unmarshal(value, &number) != nil || number < 0 {
			return false
		}
	}
	return true
}
