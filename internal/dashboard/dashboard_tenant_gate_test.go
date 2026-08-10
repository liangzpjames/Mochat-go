package dashboard

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func validDashboardTenantLimitsJSON(t *testing.T) string {
	t.Helper()
	limits := SaaSAdminPackageLimits{
		MaxCorps: 1, MaxUsers: 100, MaxContacts: 1000, MaxRooms: 100, MaxAgents: 10,
		ChannelCodes: 10, ShopCodes: 10, Radars: 10, Lotteries: 10,
		RoomInfinitePulls: 10, RoomFissions: 10, RoomClockIns: 10, RoomQualities: 10,
		RoomCalendars: 10, RoomReminds: 10, ContactSOPs: 10, RoomSOPs: 10,
		SensitiveWords: 10, StorageMB: 1024, ContactMessageBatches: 10,
		RoomMessageBatches: 10, RoomTagPulls: 10, WorkRoomAutoPulls: 10,
		WorkFissions: 10, OfficialAccounts: 10, AsyncExecutions: 10,
	}
	raw, err := json.Marshal(limits)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestEvaluateDashboardTenantAccessFailsClosed(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.Local)
	valid := DashboardTenantAccessSnapshot{
		TenantID:          9,
		TenantFound:       true,
		TenantStatus:      1,
		PackageFound:      true,
		PackageCode:       "pro",
		PackageStatus:     1,
		PackageStartsAt:   now.Add(-time.Hour),
		PackageExpiresAt:  now.Add(time.Hour),
		PackageLimitsJSON: validDashboardTenantLimitsJSON(t),
		SubscriptionFound: true,
		Subscription: SaaSAdminSubscription{
			Status:              SaaSAdminSubscriptionStatusActive,
			CurrentPeriodEndsAt: now.Add(time.Hour).Format("2006-01-02 15:04:05"),
		},
	}

	tests := []struct {
		name   string
		mutate func(*DashboardTenantAccessSnapshot)
		reason string
	}{
		{name: "tenant missing", mutate: func(s *DashboardTenantAccessSnapshot) { s.TenantFound = false }, reason: DashboardTenantAccessReasonTenantMissing},
		{name: "tenant disabled", mutate: func(s *DashboardTenantAccessSnapshot) { s.TenantStatus = 2 }, reason: DashboardTenantAccessReasonTenantDisabled},
		{name: "tenant unknown status", mutate: func(s *DashboardTenantAccessSnapshot) { s.TenantStatus = 0 }, reason: DashboardTenantAccessReasonTenantDisabled},
		{name: "package missing", mutate: func(s *DashboardTenantAccessSnapshot) { s.PackageFound = false }, reason: DashboardTenantAccessReasonPackageMissing},
		{name: "package code empty", mutate: func(s *DashboardTenantAccessSnapshot) { s.PackageCode = " " }, reason: DashboardTenantAccessReasonPackageInvalid},
		{name: "package disabled", mutate: func(s *DashboardTenantAccessSnapshot) { s.PackageStatus = 2 }, reason: DashboardTenantAccessReasonPackageDisabled},
		{name: "package unknown status", mutate: func(s *DashboardTenantAccessSnapshot) { s.PackageStatus = 0 }, reason: DashboardTenantAccessReasonPackageDisabled},
		{name: "package not started", mutate: func(s *DashboardTenantAccessSnapshot) { s.PackageStartsAt = now.Add(time.Second) }, reason: DashboardTenantAccessReasonPackageNotStarted},
		{name: "package expired at boundary", mutate: func(s *DashboardTenantAccessSnapshot) { s.PackageExpiresAt = now }, reason: DashboardTenantAccessReasonPackageExpired},
		{name: "limits empty", mutate: func(s *DashboardTenantAccessSnapshot) { s.PackageLimitsJSON = "" }, reason: DashboardTenantAccessReasonLimitsInvalid},
		{name: "limits empty object", mutate: func(s *DashboardTenantAccessSnapshot) { s.PackageLimitsJSON = "{}" }, reason: DashboardTenantAccessReasonLimitsInvalid},
		{name: "limits missing key", mutate: func(s *DashboardTenantAccessSnapshot) { s.PackageLimitsJSON = `{"maxUsers":100}` }, reason: DashboardTenantAccessReasonLimitsInvalid},
		{name: "limits string value", mutate: func(s *DashboardTenantAccessSnapshot) {
			s.PackageLimitsJSON = strings.Replace(validDashboardTenantLimitsJSON(t), `"maxUsers":100`, `"maxUsers":"100"`, 1)
		}, reason: DashboardTenantAccessReasonLimitsInvalid},
		{name: "limits negative value", mutate: func(s *DashboardTenantAccessSnapshot) {
			s.PackageLimitsJSON = strings.Replace(validDashboardTenantLimitsJSON(t), `"maxUsers":100`, `"maxUsers":-1`, 1)
		}, reason: DashboardTenantAccessReasonLimitsInvalid},
		{name: "limits null", mutate: func(s *DashboardTenantAccessSnapshot) { s.PackageLimitsJSON = "null" }, reason: DashboardTenantAccessReasonLimitsInvalid},
		{name: "limits array", mutate: func(s *DashboardTenantAccessSnapshot) { s.PackageLimitsJSON = "[]" }, reason: DashboardTenantAccessReasonLimitsInvalid},
		{name: "limits scalar", mutate: func(s *DashboardTenantAccessSnapshot) { s.PackageLimitsJSON = "1" }, reason: DashboardTenantAccessReasonLimitsInvalid},
		{name: "limits malformed", mutate: func(s *DashboardTenantAccessSnapshot) { s.PackageLimitsJSON = "{" }, reason: DashboardTenantAccessReasonLimitsInvalid},
		{name: "subscription missing", mutate: func(s *DashboardTenantAccessSnapshot) { s.SubscriptionFound = false }, reason: DashboardTenantAccessReasonSubscriptionMissing},
		{name: "subscription suspended", mutate: func(s *DashboardTenantAccessSnapshot) { s.Subscription.Status = SaaSAdminSubscriptionStatusSuspended }, reason: DashboardTenantAccessReasonSubscriptionDenied},
		{name: "subscription canceled", mutate: func(s *DashboardTenantAccessSnapshot) { s.Subscription.Status = SaaSAdminSubscriptionStatusCanceled }, reason: DashboardTenantAccessReasonSubscriptionDenied},
		{name: "subscription unknown", mutate: func(s *DashboardTenantAccessSnapshot) { s.Subscription.Status = "unknown" }, reason: DashboardTenantAccessReasonSubscriptionDenied},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := valid
			test.mutate(&snapshot)
			access := EvaluateDashboardTenantAccess(snapshot, now)
			if access.Allowed {
				t.Fatalf("access = %+v, want denied", access)
			}
			if access.TenantID != snapshot.TenantID || access.Reason != test.reason {
				t.Fatalf("access = %+v, want tenant=%d reason=%q", access, snapshot.TenantID, test.reason)
			}
		})
	}
}

func TestEvaluateDashboardTenantAccessAllowsEffectiveSubscriptionStates(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.Local)
	tests := []struct {
		name         string
		subscription SaaSAdminSubscription
	}{
		{name: "active", subscription: SaaSAdminSubscription{Status: SaaSAdminSubscriptionStatusActive, CurrentPeriodEndsAt: now.Add(time.Hour).Format("2006-01-02 15:04:05")}},
		{name: "trialing", subscription: SaaSAdminSubscription{Status: SaaSAdminSubscriptionStatusTrialing, TrialEndsAt: now.Add(time.Hour).Format("2006-01-02 15:04:05")}},
		{name: "grace", subscription: SaaSAdminSubscription{Status: SaaSAdminSubscriptionStatusGrace, GraceEndsAt: now.Add(time.Hour).Format("2006-01-02 15:04:05")}},
		{name: "expired active becomes grace", subscription: SaaSAdminSubscription{Status: SaaSAdminSubscriptionStatusActive, CurrentPeriodEndsAt: now.Add(-time.Minute).Format("2006-01-02 15:04:05"), GraceEndsAt: now.Add(time.Hour).Format("2006-01-02 15:04:05")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := DashboardTenantAccessSnapshot{
				TenantID: 9, TenantFound: true, TenantStatus: 1,
				PackageFound: true, PackageCode: "pro", PackageStatus: 1,
				PackageStartsAt: now.Add(-time.Hour), PackageExpiresAt: now.Add(time.Hour),
				PackageLimitsJSON: validDashboardTenantLimitsJSON(t), SubscriptionFound: true,
				Subscription: test.subscription,
			}
			access := EvaluateDashboardTenantAccess(snapshot, now)
			if !access.Allowed || access.Reason != "" {
				t.Fatalf("access = %+v, want allowed", access)
			}
		})
	}
}

func TestEvaluateDashboardTenantAccessAllowsOpenPackageDates(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.Local)
	snapshot := DashboardTenantAccessSnapshot{
		TenantID: 9, TenantFound: true, TenantStatus: 1,
		PackageFound: true, PackageCode: "lifetime", PackageStatus: 1,
		PackageLimitsJSON: validDashboardTenantLimitsJSON(t), SubscriptionFound: true,
		Subscription: SaaSAdminSubscription{Status: SaaSAdminSubscriptionStatusActive},
	}
	if access := EvaluateDashboardTenantAccess(snapshot, now); !access.Allowed {
		t.Fatalf("access = %+v, want allowed", access)
	}
}
