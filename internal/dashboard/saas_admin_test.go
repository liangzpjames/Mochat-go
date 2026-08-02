package dashboard

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/authjwt"
)

func TestSaaSAdminOverviewReturnsTenantScope(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 1, OpenAlertCount: 2},
			Tenants: []SaaSAdminTenantOverview{{
				TenantID:        10,
				TenantName:      "租户A",
				PackageCode:     "pro",
				OpenAlertCount:  2,
				MaxUsageMetric:  SaaSMetricContacts,
				MaxUsageCurrent: 80,
				MaxUsageLimit:   100,
				MaxUsageRatio:   0.8,
			}},
			Metrics: []SaaSAdminMetricOverview{{
				Metric:         SaaSMetricContacts,
				Current:        80,
				Limit:          100,
				UsageRatio:     0.8,
				OpenAlertCount: 2,
			}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/overview?limit=500&expiringDays=400", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.Overview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.Scope != SaaSAdminScopeTenant || store.lastOptions.TenantID != 10 || store.lastOptions.ExcludedTenantID != 0 || store.lastOptions.Limit != 100 || store.lastOptions.ExpiringDays != 365 {
		t.Fatalf("options = %+v", store.lastOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["scope"] != SaaSAdminScopeTenant || data["tenantPopulation"] != SaaSAdminTenantPopulationSelected || data["canPlatformScope"] != false {
		t.Fatalf("scope payload = %+v", data)
	}
	summary := data["summary"].(map[string]any)
	if summary["openAlertCount"].(float64) != 2 {
		t.Fatalf("summary = %+v", summary)
	}
	tenants := data["tenants"].([]any)
	if len(tenants) != 1 || tenants[0].(map[string]any)["maxUsageLabel"] != "客户数" {
		t.Fatalf("tenants = %+v", tenants)
	}
}

func TestSaaSAdminOverviewAllowsPlatformScopeForPlatformTenant(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{Summary: SaaSAdminSummary{TenantCount: 3}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/overview?scope=platform&tenantId=12", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Overview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.Scope != SaaSAdminScopePlatform || store.lastOptions.TenantID != 12 || store.lastOptions.ExcludedTenantID != 1 {
		t.Fatalf("options = %+v", store.lastOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["scope"] != SaaSAdminScopePlatform || data["tenantPopulation"] != SaaSAdminTenantPopulationBusiness || data["canPlatformScope"] != true {
		t.Fatalf("scope payload = %+v", data)
	}
}

func TestSaaSAdminOverviewParsesTenantListFilters(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{Summary: SaaSAdminSummary{TenantCount: 3}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/overview?scope=platform&keyword=%E6%96%B0%E5%BC%80&tenantStatus=1&packageCode=growth&dueState=soon&limit=30", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Overview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.Scope != SaaSAdminScopePlatform ||
		store.lastOptions.ExcludedTenantID != 1 ||
		store.lastOptions.Keyword != "新开" ||
		store.lastOptions.TenantStatus != 1 ||
		store.lastOptions.PackageCode != "growth" ||
		store.lastOptions.DueState != SaaSAdminDueStateExpiring ||
		store.lastOptions.Limit != 30 {
		t.Fatalf("options = %+v", store.lastOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	filters := data["filters"].(map[string]any)
	if filters["keyword"] != "新开" || filters["tenantStatus"].(float64) != 1 || filters["packageCode"] != "growth" || filters["dueState"] != SaaSAdminDueStateExpiring {
		t.Fatalf("filters = %+v", filters)
	}
}

func TestSaaSAdminOverviewRejectsPlatformScopeForTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/overview?scope=platform", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.Overview(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.overviewCalls != 0 {
		t.Fatalf("overview calls = %d", store.overviewCalls)
	}
}

func TestSaaSAdminOverviewRejectsInvalidTenantListFilters(t *testing.T) {
	cases := []string{
		"/dashboard/saasAdmin/overview?tenantStatus=9",
		"/dashboard/saasAdmin/overview?dueState=bad",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			store := &fakeSaaSAdminStore{
				users: map[int]User{
					1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
				},
			}
			handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("X-Mochat-Go-User-ID", "1")
			rec := httptest.NewRecorder()

			handler.Overview(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
			if store.overviewCalls != 0 {
				t.Fatalf("overview calls = %d", store.overviewCalls)
			}
		})
	}
}

func TestSaaSAdminPackagesRequiresPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
		packages: []SaaSAdminPackage{{
			Code: "growth",
			Name: "增长版",
			Limits: SaaSAdminPackageLimits{
				MaxUsers:     10,
				ChannelCodes: 12,
			},
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/packages", nil)
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.Packages(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
	if store.packageCalls != 0 {
		t.Fatalf("package calls = %d", store.packageCalls)
	}

	platformReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/packages", nil)
	platformReq.Header.Set("X-Mochat-Go-User-ID", "1")
	platformRec := httptest.NewRecorder()
	handler.Packages(platformRec, platformReq)
	if platformRec.Code != http.StatusOK {
		t.Fatalf("platform status = %d body=%s", platformRec.Code, platformRec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, platformRec)
	packages := data["packages"].([]any)
	if len(packages) != 1 {
		t.Fatalf("packages = %+v", packages)
	}
	limits := packages[0].(map[string]any)["limits"].(map[string]any)
	if limits["maxUsers"].(float64) != 10 || limits["channelCodes"].(float64) != 12 {
		t.Fatalf("limits = %+v", limits)
	}
}

func TestSaaSAdminOperationLogsRequiresPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
		operationLogs: []SaaSAdminOperationLog{
			{
				ID:            99,
				TenantID:      12,
				ActorUserID:   1,
				ActorTenantID: 1,
				Action:        "tenant.status",
				TargetType:    "tenant",
				TargetID:      "12",
				TargetName:    "租户B",
				BeforeJSON:    `{"status":1}`,
				AfterJSON:     `{"status":2}`,
				Remark:        "欠费停用",
				CreatedAt:     "2026-07-09 15:00:00",
			},
			{
				ID:            98,
				TenantID:      12,
				ActorUserID:   2,
				ActorTenantID: 1,
				Action:        "tenant.status",
				TargetType:    "tenant",
				TargetID:      "12",
				TargetName:    "租户B",
				BeforeJSON:    `{"status":2}`,
				AfterJSON:     `{"status":1}`,
				Remark:        "欠费恢复",
				CreatedAt:     "2026-07-09 15:05:00",
			},
			{
				ID:            97,
				TenantID:      13,
				ActorUserID:   3,
				ActorTenantID: 1,
				Action:        "tenant.package",
				TargetType:    "tenant",
				TargetID:      "13",
				TargetName:    "租户C",
				BeforeJSON:    `{"packageCode":"growth"}`,
				AfterJSON:     `{"packageCode":"scale"}`,
				Remark:        "升级套餐",
				CreatedAt:     "2026-07-09 15:10:00",
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/operations", nil)
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.OperationLogs(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
	if store.operationCalls != 0 || store.operationSummaryCalls != 0 {
		t.Fatalf("operation calls = %d summary calls = %d", store.operationCalls, store.operationSummaryCalls)
	}

	platformReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/operations?tenantId=12&limit=1&action=tenant.status&targetType=tenant&keyword=%E6%AC%A0%E8%B4%B9", nil)
	platformReq.Header.Set("X-Mochat-Go-User-ID", "1")
	platformRec := httptest.NewRecorder()
	handler.OperationLogs(platformRec, platformReq)
	if platformRec.Code != http.StatusOK {
		t.Fatalf("platform status = %d body=%s", platformRec.Code, platformRec.Body.String())
	}
	if store.lastOperationOptions.TenantID != 12 ||
		store.lastOperationOptions.Limit != 1 ||
		store.lastOperationOptions.Action != "tenant.status" ||
		store.lastOperationOptions.TargetType != "tenant" ||
		store.lastOperationOptions.Keyword != "欠费" {
		t.Fatalf("operation options = %+v", store.lastOperationOptions)
	}
	if store.lastOperationSummaryOptions != store.lastOperationOptions || store.operationSummaryCalls != 1 {
		t.Fatalf("operation summary options = %+v calls=%d", store.lastOperationSummaryOptions, store.operationSummaryCalls)
	}
	data := decodeSaaSAdminResponse(t, platformRec)
	filters := data["filters"].(map[string]any)
	if filters["tenantId"].(float64) != 12 || filters["action"] != "tenant.status" || filters["targetType"] != "tenant" || filters["keyword"] != "欠费" {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["operationCount"].(float64) != 2 ||
		summary["tenantCount"].(float64) != 1 ||
		summary["actorUserCount"].(float64) != 2 ||
		summary["actionCount"].(float64) != 1 ||
		summary["targetTypeCount"].(float64) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if data["returnedCount"].(float64) != 1 {
		t.Fatalf("returnedCount = %+v", data["returnedCount"])
	}
	operations := data["operations"].([]any)
	if len(operations) != 1 || operations[0].(map[string]any)["action"] != "tenant.status" {
		t.Fatalf("operations = %+v", operations)
	}
	after := operations[0].(map[string]any)["after"].(map[string]any)
	if after["status"].(float64) != 2 {
		t.Fatalf("after = %+v", after)
	}
}

func TestSaaSAdminBillingEventsRequiresPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
		billingEvents: []SaaSAdminBillingEvent{
			{
				ID:                78,
				TenantID:          12,
				EventType:         "renewal",
				PackageCode:       "growth",
				PackageName:       "增长版",
				PreviousExpiresAt: "2027-02-01 00:00:00",
				NewExpiresAt:      "2028-02-01 00:00:00",
				AmountCents:       1000,
				Currency:          "CNY",
				PaymentMethod:     "bank",
				ExternalOrderNo:   "ORDER-2",
				MetadataJSON:      `{"source":"saas_admin"}`,
				CreatedAt:         "2026-07-09 17:00:00",
			},
			{
				ID:                77,
				TenantID:          12,
				EventType:         "renewal",
				PackageCode:       "growth",
				PackageName:       "增长版",
				PreviousExpiresAt: "2027-01-01 00:00:00",
				NewExpiresAt:      "2028-01-01 00:00:00",
				AmountCents:       99000,
				Currency:          "CNY",
				PaymentMethod:     "bank",
				ExternalOrderNo:   "ORDER-1",
				MetadataJSON:      `{"source":"saas_admin"}`,
				CreatedAt:         "2026-07-09 16:00:00",
			},
			{
				ID:              79,
				TenantID:        13,
				EventType:       "renewal",
				PackageCode:     "scale",
				PackageName:     "规模版",
				AmountCents:     5000,
				Currency:        "CNY",
				ExternalOrderNo: "ORDER-3",
				CreatedAt:       "2026-07-09 18:00:00",
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/billingEvents", nil)
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.BillingEvents(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
	if store.billingCalls != 0 || store.billingSummaryCalls != 0 {
		t.Fatalf("billing calls=%d summary=%d", store.billingCalls, store.billingSummaryCalls)
	}

	platformReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/billingEvents?tenantId=12&limit=1&eventType=renewal&packageCode=growth&keyword=ORDER-", nil)
	platformReq.Header.Set("X-Mochat-Go-User-ID", "1")
	platformRec := httptest.NewRecorder()
	handler.BillingEvents(platformRec, platformReq)
	if platformRec.Code != http.StatusOK {
		t.Fatalf("platform status = %d body=%s", platformRec.Code, platformRec.Body.String())
	}
	if store.lastBillingOptions.TenantID != 12 ||
		store.lastBillingOptions.Limit != 1 ||
		store.lastBillingOptions.EventType != "renewal" ||
		store.lastBillingOptions.PackageCode != "growth" ||
		store.lastBillingOptions.Keyword != "ORDER-" {
		t.Fatalf("billing options = %+v", store.lastBillingOptions)
	}
	if store.lastBillingSummaryOptions != store.lastBillingOptions {
		t.Fatalf("summary options = %+v list options = %+v", store.lastBillingSummaryOptions, store.lastBillingOptions)
	}
	data := decodeSaaSAdminResponse(t, platformRec)
	filters := data["filters"].(map[string]any)
	if filters["tenantId"].(float64) != 12 || filters["eventType"] != "renewal" || filters["packageCode"] != "growth" || filters["keyword"] != "ORDER-" {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["eventCount"].(float64) != 2 || summary["renewalCount"].(float64) != 2 || summary["amountCents"].(float64) != 100000 {
		t.Fatalf("summary = %+v", summary)
	}
	if data["returnedCount"].(float64) != 1 {
		t.Fatalf("returnedCount = %+v", data["returnedCount"])
	}
	events := data["billingEvents"].([]any)
	if len(events) != 1 {
		t.Fatalf("events = %+v", events)
	}
	event := events[0].(map[string]any)
	if event["eventType"] != "renewal" || event["amountCents"].(float64) != 1000 || event["externalOrderNo"] != "ORDER-2" {
		t.Fatalf("event = %+v", event)
	}
	metadata := event["metadata"].(map[string]any)
	if metadata["source"] != "saas_admin" {
		t.Fatalf("metadata = %+v", metadata)
	}
}

func TestSaaSAdminBillingReconciliationRequiresPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
		billingReconciliationItems: []SaaSAdminBillingReconciliationItem{
			{
				BillingEvent: SaaSAdminBillingEvent{
					ID:              81,
					TenantID:        12,
					EventType:       "renewal",
					PackageCode:     "growth",
					PackageName:     "增长版",
					NewExpiresAt:    "2028-02-01 00:00:00",
					AmountCents:     99000,
					Currency:        "CNY",
					PaymentMethod:   "bank",
					ExternalOrderNo: "ORDER-OK",
					CreatedAt:       "2026-07-09 17:00:00",
				},
				TenantName:           "租户B",
				CurrentPackageFound:  true,
				CurrentPackageCode:   "growth",
				CurrentPackageName:   "增长版",
				CurrentExpiresAt:     "2029-02-01 00:00:00",
				CurrentPackageStatus: 1,
			},
			{
				BillingEvent: SaaSAdminBillingEvent{
					ID:              82,
					TenantID:        12,
					EventType:       "renewal",
					PackageCode:     "growth",
					PackageName:     "增长版",
					NewExpiresAt:    "2030-02-01 00:00:00",
					AmountCents:     199000,
					Currency:        "CNY",
					PaymentMethod:   "bank",
					ExternalOrderNo: "ORDER-DRIFT",
					CreatedAt:       "2026-07-09 18:00:00",
				},
				TenantName:           "租户B",
				CurrentPackageFound:  true,
				CurrentPackageCode:   "scale",
				CurrentPackageName:   "规模版",
				CurrentExpiresAt:     "2029-02-01 00:00:00",
				CurrentPackageStatus: 1,
			},
			{
				BillingEvent: SaaSAdminBillingEvent{
					ID:              83,
					TenantID:        13,
					EventType:       "renewal",
					PackageCode:     "scale",
					PackageName:     "规模版",
					NewExpiresAt:    "2028-02-01 00:00:00",
					ExternalOrderNo: "ORDER-OTHER",
					CreatedAt:       "2026-07-09 19:00:00",
				},
				TenantName:          "租户C",
				CurrentPackageFound: false,
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/billingReconciliation", nil)
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.BillingReconciliation(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
	if store.billingReconcileCalls != 0 {
		t.Fatalf("billing reconcile calls = %d", store.billingReconcileCalls)
	}

	platformReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/billingReconciliation?tenantId=12&limit=1&eventType=renewal&packageCode=growth&keyword=ORDER-&mismatchOnly=1", nil)
	platformReq.Header.Set("X-Mochat-Go-User-ID", "1")
	platformRec := httptest.NewRecorder()
	handler.BillingReconciliation(platformRec, platformReq)
	if platformRec.Code != http.StatusOK {
		t.Fatalf("platform status = %d body=%s", platformRec.Code, platformRec.Body.String())
	}
	if store.lastBillingReconcileOptions.TenantID != 12 ||
		store.lastBillingReconcileOptions.Limit != 1 ||
		store.lastBillingReconcileOptions.EventType != "renewal" ||
		store.lastBillingReconcileOptions.PackageCode != "growth" ||
		store.lastBillingReconcileOptions.Keyword != "ORDER-" ||
		!store.lastBillingReconcileOptions.MismatchOnly {
		t.Fatalf("reconcile options = %+v", store.lastBillingReconcileOptions)
	}
	data := decodeSaaSAdminResponse(t, platformRec)
	filters := data["filters"].(map[string]any)
	if filters["tenantId"].(float64) != 12 || filters["mismatchOnly"] != true {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["checkedCount"].(float64) != 1 ||
		summary["matchedCount"].(float64) != 0 ||
		summary["mismatchedCount"].(float64) != 1 ||
		summary["packageMismatchCount"].(float64) != 1 ||
		summary["expiresMismatchCount"].(float64) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if data["returnedCount"].(float64) != 1 {
		t.Fatalf("returnedCount = %+v", data["returnedCount"])
	}
	items := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %+v", items)
	}
	item := items[0].(map[string]any)
	if item["id"].(float64) != 82 || item["reconcileStatus"] != "mismatch" || item["currentPackageCode"] != "scale" {
		t.Fatalf("item = %+v", item)
	}
	reasons := item["mismatchReasons"].([]any)
	if len(reasons) != 2 || reasons[0] != "package_mismatch" || reasons[1] != "expires_mismatch" {
		t.Fatalf("reasons = %+v", reasons)
	}
}

func TestSaaSAdminBillingReconciliationFollowUpRequiresPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
		billingEvents: []SaaSAdminBillingEvent{{
			ID:                93,
			TenantID:          12,
			EventType:         "renewal",
			PackageCode:       "growth",
			PackageName:       "增长版",
			PreviousExpiresAt: "2027-01-01 00:00:00",
			NewExpiresAt:      "2031-02-03 00:00:00",
			AmountCents:       880000,
			Currency:          "CNY",
			PaidAt:            "2026-07-09 00:00:00",
			PaymentMethod:     "manual",
			ExternalOrderNo:   "DRIFT-93",
			ActorUserID:       1,
			ActorTenantID:     1,
			Remark:            "对账异常",
			MetadataJSON:      `{"source":"smoke"}`,
			CreatedAt:         "2026-07-09 18:31:00",
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/billingReconciliationFollowUp", strings.NewReader(`{"billingEventId":93}`))
	tenantReq.Header.Set("Content-Type", "application/json")
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.BillingReconciliationFollowUp(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
	if store.billingEventByIDCalls != 0 || store.recordOperationLogCalls != 0 {
		t.Fatalf("tenant calls billing=%d operation=%d", store.billingEventByIDCalls, store.recordOperationLogCalls)
	}

	platformReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/billingReconciliationFollowUp", strings.NewReader(`{"billingEventId":93,"status":"resolved","owner":"finance","nextFollowUpAt":"2026-07-10","remark":"已修正套餐"}`))
	platformReq.Header.Set("Content-Type", "application/json")
	platformReq.Header.Set("X-Mochat-Go-User-ID", "1")
	platformRec := httptest.NewRecorder()
	handler.BillingReconciliationFollowUp(platformRec, platformReq)
	if platformRec.Code != http.StatusOK {
		t.Fatalf("platform status = %d body=%s", platformRec.Code, platformRec.Body.String())
	}
	if store.billingEventByIDCalls != 1 || store.lastBillingEventByID != 93 || store.recordOperationLogCalls != 1 {
		t.Fatalf("calls billing=%d last=%d operation=%d", store.billingEventByIDCalls, store.lastBillingEventByID, store.recordOperationLogCalls)
	}
	log := store.lastRecordedOperationLog
	if log.TenantID != 12 ||
		log.ActorUserID != 1 ||
		log.ActorTenantID != 1 ||
		log.Action != SaaSAdminOperationActionBillingReconciliationFollowUp ||
		log.TargetType != SaaSAdminOperationTargetBillingEvent ||
		log.TargetID != "93" ||
		log.TargetName != "DRIFT-93" ||
		log.Remark != "已修正套餐" {
		t.Fatalf("operation log = %+v", log)
	}
	if !strings.Contains(log.BeforeJSON, `"externalOrderNo":"DRIFT-93"`) ||
		!strings.Contains(log.AfterJSON, `"billingEventId":93`) ||
		!strings.Contains(log.AfterJSON, `"status":"resolved"`) ||
		!strings.Contains(log.AfterJSON, `"owner":"finance"`) ||
		!strings.Contains(log.AfterJSON, `"nextFollowUpAt":"2026-07-10 00:00:00"`) {
		t.Fatalf("operation json before=%s after=%s", log.BeforeJSON, log.AfterJSON)
	}
	data := decodeSaaSAdminResponse(t, platformRec)
	if data["operationId"].(float64) != 1 ||
		data["billingEventId"].(float64) != 93 ||
		data["tenantId"].(float64) != 12 ||
		data["status"] != SaaSAdminRiskFollowUpStatusResolved ||
		data["owner"] != "finance" ||
		data["nextFollowUpAt"] != "2026-07-10 00:00:00" ||
		data["remark"] != "已修正套餐" {
		t.Fatalf("payload = %+v", data)
	}
	event := data["billingEvent"].(map[string]any)
	if event["externalOrderNo"] != "DRIFT-93" || event["packageCode"] != "growth" {
		t.Fatalf("billing event = %+v", event)
	}
}

func TestSaaSAdminBillingReconciliationFollowUpsAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		billingFollowUpSnapshots: []SaaSAdminBillingReconciliationFollowUpSnapshot{
			{
				OperationID:     501,
				BillingEventID:  93,
				TenantID:        12,
				TenantName:      "租户B",
				Status:          SaaSAdminRiskFollowUpStatusContacted,
				Owner:           "finance",
				NextFollowUpAt:  "2000-01-01 00:00:00",
				Remark:          "smoke-reconciliation-follow-up",
				PackageCode:     "growth",
				PackageName:     "增长版",
				NewExpiresAt:    "2031-02-03 00:00:00",
				AmountCents:     880000,
				Currency:        "CNY",
				ExternalOrderNo: "DRIFT-93",
				CreatedAt:       "2026-07-09 18:31:00",
			},
			{
				OperationID:     502,
				BillingEventID:  94,
				TenantID:        12,
				TenantName:      "租户B",
				Status:          SaaSAdminRiskFollowUpStatusResolved,
				Owner:           "finance",
				NextFollowUpAt:  "2000-01-01 00:00:00",
				Remark:          "resolved",
				ExternalOrderNo: "DRIFT-94",
				CreatedAt:       "2026-07-09 18:40:00",
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/billingReconciliationFollowUps?tenantId=12&status=contacted&owner=fin&keyword=DRIFT&dueState=overdue&limit=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.BillingReconciliationFollowUps(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.billingFollowUpCalls != 1 ||
		store.lastBillingFollowUpOptions.TenantID != 12 ||
		store.lastBillingFollowUpOptions.Status != SaaSAdminRiskFollowUpStatusContacted ||
		store.lastBillingFollowUpOptions.Owner != "fin" ||
		store.lastBillingFollowUpOptions.Keyword != "DRIFT" ||
		store.lastBillingFollowUpOptions.DueState != SaaSAdminRiskFollowUpDueStateAll ||
		store.lastBillingFollowUpOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("options = %+v calls=%d", store.lastBillingFollowUpOptions, store.billingFollowUpCalls)
	}
	data := decodeSaaSAdminResponse(t, rec)
	filters := data["filters"].(map[string]any)
	if filters["tenantId"].(float64) != 12 || filters["status"] != SaaSAdminRiskFollowUpStatusContacted || filters["owner"] != "fin" || filters["keyword"] != "DRIFT" || filters["dueState"] != SaaSAdminRiskFollowUpDueStateOverdue || filters["limit"].(float64) != 10 {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["totalCount"].(float64) != 1 || summary["contactedCount"].(float64) != 1 || summary["overdueCount"].(float64) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	items := data["followUps"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %+v", items)
	}
	item := items[0].(map[string]any)
	if item["billingEventId"].(float64) != 93 ||
		item["operationId"].(float64) != 501 ||
		item["owner"] != "finance" ||
		item["dueState"] != SaaSAdminRiskFollowUpDueStateOverdue ||
		item["overdue"] != true ||
		item["packageCode"] != "growth" ||
		item["externalOrderNo"] != "DRIFT-93" ||
		item["amountCents"].(float64) != 880000 {
		t.Fatalf("item = %+v", item)
	}
	if item["daysUntil"].(float64) >= 0 {
		t.Fatalf("daysUntil = %+v", item["daysUntil"])
	}
}

func TestSaaSAdminBillingReconciliationFollowUpsRejectsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/billingReconciliationFollowUps", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.BillingReconciliationFollowUps(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.billingFollowUpCalls != 0 {
		t.Fatalf("calls = %d", store.billingFollowUpCalls)
	}
}

func TestSaaSAdminBillingReconciliationFollowUpOwnersAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		billingFollowUpSnapshots: []SaaSAdminBillingReconciliationFollowUpSnapshot{
			{OperationID: 501, BillingEventID: 93, TenantID: 12, TenantName: "租户B", Status: SaaSAdminRiskFollowUpStatusContacted, Owner: "finance", NextFollowUpAt: "2000-01-01 00:00:00", ExternalOrderNo: "DRIFT-93", CreatedAt: "2026-07-09 18:31:00"},
			{OperationID: 502, BillingEventID: 94, TenantID: 12, TenantName: "租户B", Status: SaaSAdminRiskFollowUpStatusRenewalPending, Owner: "finance", NextFollowUpAt: "2999-01-01 00:00:00", ExternalOrderNo: "DRIFT-94", CreatedAt: "2026-07-09 18:40:00"},
			{OperationID: 503, BillingEventID: 95, TenantID: 13, TenantName: "租户C", Status: SaaSAdminRiskFollowUpStatusPending, Remark: "未分配", ExternalOrderNo: "DRIFT-95", CreatedAt: "2026-07-09 18:50:00"},
			{OperationID: 504, BillingEventID: 96, TenantID: 13, TenantName: "租户C", Status: SaaSAdminRiskFollowUpStatusResolved, Owner: "other", NextFollowUpAt: "2000-01-01 00:00:00", ExternalOrderNo: "DRIFT-96", CreatedAt: "2026-07-09 19:00:00"},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/billingReconciliationFollowUpOwners?limit=1000", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.BillingReconciliationFollowUpOwners(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.billingFollowUpCalls != 1 || store.lastBillingFollowUpOptions.DueState != SaaSAdminRiskFollowUpDueStateAll || store.lastBillingFollowUpOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("options = %+v calls=%d", store.lastBillingFollowUpOptions, store.billingFollowUpCalls)
	}
	data := decodeSaaSAdminResponse(t, rec)
	summary := data["summary"].(map[string]any)
	if summary["totalCount"].(float64) != 4 || summary["closedCount"].(float64) != 1 || summary["overdueCount"].(float64) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	owners := data["owners"].([]any)
	if len(owners) != 3 {
		t.Fatalf("owners = %+v", owners)
	}
	finance := owners[0].(map[string]any)
	if finance["owner"] != "finance" ||
		finance["totalCount"].(float64) != 2 ||
		finance["openCount"].(float64) != 2 ||
		finance["contactedCount"].(float64) != 1 ||
		finance["renewalPendingCount"].(float64) != 1 ||
		finance["overdueCount"].(float64) != 1 ||
		finance["futureCount"].(float64) != 1 ||
		finance["latestFollowUpAt"] != "2026-07-09 18:40:00" {
		t.Fatalf("finance = %+v", finance)
	}
	var unassigned map[string]any
	var closed map[string]any
	for _, raw := range owners {
		item := raw.(map[string]any)
		switch item["owner"] {
		case "未分配":
			unassigned = item
		case "other":
			closed = item
		}
	}
	if unassigned == nil || unassigned["pendingCount"].(float64) != 1 || unassigned["noDateCount"].(float64) != 1 {
		t.Fatalf("unassigned = %+v owners=%+v", unassigned, owners)
	}
	if closed == nil || closed["openCount"].(float64) != 0 || closed["closedCount"].(float64) != 1 {
		t.Fatalf("closed = %+v owners=%+v", closed, owners)
	}
}

func TestSaaSAdminBillingReconciliationFollowUpOwnersRejectsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/billingReconciliationFollowUpOwners", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.BillingReconciliationFollowUpOwners(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.billingFollowUpCalls != 0 {
		t.Fatalf("calls = %d", store.billingFollowUpCalls)
	}
}

func TestSaaSAdminBillingReconciliationFollowUpBulkCloseAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		billingEvents: []SaaSAdminBillingEvent{
			{
				ID:              93,
				TenantID:        12,
				EventType:       "renewal",
				PackageCode:     "growth",
				PackageName:     "增长版",
				NewExpiresAt:    "2031-02-03 00:00:00",
				AmountCents:     880000,
				Currency:        "CNY",
				ExternalOrderNo: "DRIFT-93",
				CreatedAt:       "2026-07-09 18:31:00",
			},
			{
				ID:              94,
				TenantID:        12,
				EventType:       "renewal",
				PackageCode:     "scale",
				PackageName:     "规模版",
				NewExpiresAt:    "2032-02-03 00:00:00",
				AmountCents:     990000,
				Currency:        "CNY",
				ExternalOrderNo: "DRIFT-94",
				CreatedAt:       "2026-07-09 18:40:00",
			},
		},
		billingFollowUpSnapshots: []SaaSAdminBillingReconciliationFollowUpSnapshot{
			{OperationID: 501, BillingEventID: 93, TenantID: 12, TenantName: "租户B", Status: SaaSAdminRiskFollowUpStatusContacted, Owner: "finance", NextFollowUpAt: "2000-01-01 00:00:00", Remark: "待关闭", PackageCode: "growth", AmountCents: 880000, Currency: "CNY", ExternalOrderNo: "DRIFT-93", CreatedAt: "2026-07-09 18:31:00"},
			{OperationID: 502, BillingEventID: 94, TenantID: 12, TenantName: "租户B", Status: SaaSAdminRiskFollowUpStatusPending, Owner: "finance", NextFollowUpAt: "2999-01-01 00:00:00", Remark: "不匹配状态", PackageCode: "scale", AmountCents: 990000, Currency: "CNY", ExternalOrderNo: "DRIFT-94", CreatedAt: "2026-07-09 18:40:00"},
			{OperationID: 503, BillingEventID: 95, TenantID: 12, TenantName: "租户B", Status: SaaSAdminRiskFollowUpStatusResolved, Owner: "finance", ExternalOrderNo: "DRIFT-95", CreatedAt: "2026-07-09 18:50:00"},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/billingReconciliationFollowUpBulkClose", strings.NewReader(`{"filterStatus":"contacted","owner":"fin","keyword":"DRIFT-93","dueState":"overdue","limit":10,"closeStatus":"resolved","remark":"批量账单已处理"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.BillingReconciliationFollowUpBulkClose(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.billingFollowUpCalls != 1 ||
		store.lastBillingFollowUpOptions.TenantID != 0 ||
		store.lastBillingFollowUpOptions.Status != SaaSAdminRiskFollowUpStatusContacted ||
		store.lastBillingFollowUpOptions.Owner != "fin" ||
		store.lastBillingFollowUpOptions.Keyword != "DRIFT-93" ||
		store.lastBillingFollowUpOptions.DueState != SaaSAdminRiskFollowUpDueStateAll ||
		store.lastBillingFollowUpOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("options = %+v calls=%d", store.lastBillingFollowUpOptions, store.billingFollowUpCalls)
	}
	if store.billingEventByIDCalls != 1 || store.lastBillingEventByID != 93 || store.recordOperationLogCalls != 1 {
		t.Fatalf("calls billing=%d last=%d operation=%d", store.billingEventByIDCalls, store.lastBillingEventByID, store.recordOperationLogCalls)
	}
	log := store.lastRecordedOperationLog
	if log.TenantID != 12 ||
		log.ActorUserID != 1 ||
		log.ActorTenantID != 1 ||
		log.Action != SaaSAdminOperationActionBillingReconciliationFollowUp ||
		log.TargetType != SaaSAdminOperationTargetBillingEvent ||
		log.TargetID != "93" ||
		log.TargetName != "DRIFT-93" ||
		log.Remark != "批量账单已处理" {
		t.Fatalf("operation log = %+v", log)
	}
	if !strings.Contains(log.BeforeJSON, `"externalOrderNo":"DRIFT-93"`) ||
		!strings.Contains(log.AfterJSON, `"billingEventId":93`) ||
		!strings.Contains(log.AfterJSON, `"status":"resolved"`) ||
		!strings.Contains(log.AfterJSON, `"owner":"finance"`) ||
		!strings.Contains(log.AfterJSON, `"remark":"批量账单已处理"`) {
		t.Fatalf("operation json before=%s after=%s", log.BeforeJSON, log.AfterJSON)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["closedCount"].(float64) != 1 || data["status"] != SaaSAdminRiskFollowUpStatusResolved || data["remark"] != "批量账单已处理" {
		t.Fatalf("data = %+v", data)
	}
	items := data["followUps"].([]any)
	if len(items) != 1 {
		t.Fatalf("followUps = %+v", items)
	}
	item := items[0].(map[string]any)
	if item["operationId"].(float64) != 1 ||
		item["billingEventId"].(float64) != 93 ||
		item["status"] != SaaSAdminRiskFollowUpStatusResolved ||
		item["owner"] != "finance" ||
		item["remark"] != "批量账单已处理" ||
		item["billingEvent"].(map[string]any)["externalOrderNo"] != "DRIFT-93" {
		t.Fatalf("item = %+v", item)
	}
}

func TestSaaSAdminBillingReconciliationFollowUpBulkCloseRejectsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/billingReconciliationFollowUpBulkClose", strings.NewReader(`{"closeStatus":"resolved"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.BillingReconciliationFollowUpBulkClose(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.billingFollowUpCalls != 0 || store.recordOperationLogCalls != 0 {
		t.Fatalf("calls snapshots=%d operation=%d", store.billingFollowUpCalls, store.recordOperationLogCalls)
	}
}

func TestSaaSAdminBillingReconciliationFollowUpBulkCloseRejectsInvalidCloseStatus(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/billingReconciliationFollowUpBulkClose", strings.NewReader(`{"closeStatus":"contacted"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.BillingReconciliationFollowUpBulkClose(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.billingFollowUpCalls != 0 || store.recordOperationLogCalls != 0 {
		t.Fatalf("calls snapshots=%d operation=%d", store.billingFollowUpCalls, store.recordOperationLogCalls)
	}
}

func TestSaaSAdminDailyReportAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{
				TenantCount:              2,
				ActiveTenantPackageCount: 2,
				UserCount:                8,
				CorpCount:                2,
				OpenAlertCount:           2,
				PendingNotificationCount: 1,
				ExpiringSoonTenantCount:  1,
			},
			Tenants: []SaaSAdminTenantOverview{
				{TenantID: 12, TenantName: "风险租户", TenantStatus: 1, PackageCode: "growth", PackageName: "增长版", ExpiresAt: "2026-07-15 00:00:00", ExpiringSoon: true, OpenAlertCount: 1},
				{TenantID: 13, TenantName: "正常租户", TenantStatus: 1, PackageCode: "scale", PackageName: "规模版", ExpiresAt: "2027-07-15 00:00:00"},
			},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{Metric: SaaSMetricUsers, Current: 9, Limit: 10, UsageRatio: 0.9, Status: "warning", OpenAlertCount: 1}},
			13: {{Metric: SaaSMetricUsers, Current: 1, Limit: 10, UsageRatio: 0.1, Status: "normal"}},
		},
		alertPage: SaaSAlertListPage{
			Total: 2,
			Items: []SaaSAlertRecord{{
				ID:              91,
				AlertKey:        "alert-91",
				TenantID:        12,
				AlertType:       SaaSAlertTypeQuotaExceeded,
				Status:          SaaSAlertStatusOpen,
				Metric:          SaaSMetricUsers,
				PeriodKey:       SaaSAlertPeriodLifetime,
				CurrentValue:    9,
				LimitValue:      10,
				OccurrenceCount: 2,
				LastSeenAt:      "2026-07-09 10:00:00",
			}},
		},
		notifications: []SaaSAlertNotification{
			{ID: 101, NotificationKey: "failed-101", TenantID: 12, Channel: "webhook", Status: SaaSAlertNotificationStatusFailed, Attempts: 1, MaxAttempts: 3, Alert: SaaSQuotaAlert{Status: SaaSQuotaStatus{TenantID: 12, Metric: SaaSMetricUsers, Current: 9, Limit: 10}, AlertType: SaaSAlertTypeQuotaExceeded, PeriodKey: SaaSAlertPeriodLifetime}, LastError: "failed", UpdatedAt: "2026-07-09 10:05:00"},
			{ID: 102, NotificationKey: "dead-102", TenantID: 12, Channel: "webhook", Status: SaaSAlertNotificationStatusDead, Attempts: 3, MaxAttempts: 3, Alert: SaaSQuotaAlert{Status: SaaSQuotaStatus{TenantID: 12, Metric: SaaSMetricUsers, Current: 9, Limit: 10}, AlertType: SaaSAlertTypeQuotaExceeded, PeriodKey: SaaSAlertPeriodLifetime}, LastError: "dead", UpdatedAt: "2026-07-09 10:06:00"},
			{ID: 103, NotificationKey: "pending-103", TenantID: 12, Channel: "webhook", Status: SaaSAlertNotificationStatusPending, Attempts: 0, MaxAttempts: 3},
			{ID: 104, NotificationKey: "delivered-104", TenantID: 12, Channel: "webhook", Status: SaaSAlertNotificationStatusDelivered, Attempts: 1, MaxAttempts: 3},
			{ID: 105, NotificationKey: "closed-105", TenantID: 12, Channel: "webhook", Status: SaaSAlertNotificationStatusClosed, Attempts: 1, MaxAttempts: 3, LastError: "closed by admin", UpdatedAt: "2026-07-09 10:07:00"},
		},
		riskFollowUpSnapshots: []SaaSAdminRiskFollowUpSnapshot{
			{TenantID: 12, TenantName: "风险租户", Status: SaaSAdminRiskFollowUpStatusContacted, Owner: "CSM-A", NextFollowUpAt: "2000-01-01 00:00:00", Remark: "today follow", OperationID: 301, CreatedAt: "2026-07-09 10:00:00"},
			{TenantID: 13, TenantName: "正常租户", Status: SaaSAdminRiskFollowUpStatusResolved, Owner: "CSM-B", Remark: "closed", OperationID: 302, CreatedAt: "2026-07-09 09:00:00"},
		},
		tasks: []SaaSAdminTask{
			{ID: 601, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusBlocked, TenantID: 12, PackageCode: "growth", ActorUserID: 1, ActorTenantID: 1, Remark: "daily-sla", LastError: "blocked", CreatedAt: "2000-01-01 00:00:00"},
			{ID: 602, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusApplied, TenantID: 13, PackageCode: "scale", ActorUserID: 2, ActorTenantID: 1, CreatedAt: "2026-07-09 09:00:00", AppliedAt: "2026-07-09 10:00:00"},
		},
		operationLogs: []SaaSAdminOperationLog{
			{ID: 401, TenantID: 12, Action: "tenant.risk.follow_up", TargetType: "tenant", TargetID: "12", TargetName: "风险租户", Remark: "today follow", CreatedAt: "2026-07-09 10:00:00"},
			{ID: 402, TenantID: 12, Action: "tenant.alert.resolve", TargetType: "tenant", TargetID: "12", TargetName: "风险租户", Remark: "today alert", CreatedAt: "2026-07-09 11:00:00"},
			{ID: 403, TenantID: 12, Action: "tenant.notification.retry", TargetType: "tenant", TargetID: "12", TargetName: "风险租户", Remark: "today retry", CreatedAt: "2026-07-09 12:00:00"},
			{ID: 405, TenantID: 12, Action: SaaSAdminOperationActionNotificationClose, TargetType: SaaSAdminOperationTargetAlertNotification, TargetID: "105", TargetName: "closed-105", Remark: "today close", CreatedAt: "2026-07-09 12:30:00"},
			{ID: 406, TenantID: 12, ActorUserID: 1, ActorTenantID: 1, Action: SaaSAdminOperationActionOperationQueueAssign, TargetType: SaaSAdminOperationTargetAdminTask, TargetID: "601", TargetName: "运营任务 SLA：package_sync #601", Remark: "daily queue assign", CreatedAt: "2026-07-09 12:45:00", AfterJSON: saasAdminPayloadJSON(map[string]any{
				"tenantId":       12,
				"source":         SaaSAdminOperationQueueSourceTaskSLA,
				"objectType":     SaaSAdminOperationTargetAdminTask,
				"objectId":       "601",
				"targetName":     "运营任务 SLA：package_sync #601",
				"owner":          "Ops-Daily",
				"status":         SaaSAdminRiskFollowUpStatusContacted,
				"nextFollowUpAt": "2026-07-10 10:00:00",
				"remark":         "daily queue assign",
				"assignedAt":     "2026-07-09 12:45:00",
			})},
			{ID: 404, TenantID: 12, Action: "tenant.renewal", TargetType: "tenant", TargetID: "12", TargetName: "风险租户", Remark: "old", CreatedAt: "2026-07-08 12:00:00"},
		},
		billingEvents: []SaaSAdminBillingEvent{
			{ID: 501, TenantID: 12, EventType: "renewal", PackageCode: "growth", PackageName: "增长版", AmountCents: 12345, Currency: "CNY", ExternalOrderNo: "ORDER-TODAY", CreatedAt: "2026-07-09 13:00:00"},
			{ID: 502, TenantID: 12, EventType: "renewal", PackageCode: "growth", PackageName: "增长版", AmountCents: 99999, Currency: "CNY", ExternalOrderNo: "ORDER-OLD", CreatedAt: "2026-07-08 13:00:00"},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/dailyReport?date=2026-07-09&days=1&limit=2&tenantLimit=50&expiringDays=15&highUsageRatio=0.75", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.DailyReport(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.overviewCalls != 1 || store.usageCalls != 2 || store.alertCalls != 1 || store.notificationCalls != 1 || store.operationCalls != 1 || store.billingCalls != 1 || store.riskFollowUpSnapshotCalls != 1 || store.taskCalls != 1 {
		t.Fatalf("calls overview=%d usage=%d alerts=%d notifications=%d operations=%d billing=%d followups=%d tasks=%d", store.overviewCalls, store.usageCalls, store.alertCalls, store.notificationCalls, store.operationCalls, store.billingCalls, store.riskFollowUpSnapshotCalls, store.taskCalls)
	}
	if store.lastOptions.Scope != SaaSAdminScopePlatform || store.lastOptions.ExcludedTenantID != 1 || store.lastOptions.Limit != 50 || store.lastOptions.ExpiringDays != 15 || store.lastOptions.DueState != SaaSAdminDueStateAll {
		t.Fatalf("overview options = %+v", store.lastOptions)
	}
	if store.lastAlertOptions.ExcludedTenantID != 1 || store.lastAlertOptions.Status != SaaSAlertStatusOpen || store.lastAlertOptions.PerPage != 2 {
		t.Fatalf("alert options = %+v", store.lastAlertOptions)
	}
	if store.lastNotificationOptions.ExcludedTenantID != 1 || store.lastNotificationOptions.Limit != saasAdminExportMaxLimit ||
		store.lastOperationOptions.ExcludedTenantID != 1 || store.lastOperationOptions.Limit != saasAdminExportMaxLimit ||
		store.lastBillingOptions.ExcludedTenantID != 1 || store.lastBillingOptions.Limit != saasAdminExportMaxLimit ||
		store.lastRiskFollowUpTaskOptions.ExcludedTenantID != 1 || store.lastTaskOptions.ExcludedTenantID != 1 {
		t.Fatalf("list options notification=%+v operation=%+v billing=%+v", store.lastNotificationOptions, store.lastOperationOptions, store.lastBillingOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	window := data["window"].(map[string]any)
	if window["date"] != "2026-07-09" || window["days"].(float64) != 1 {
		t.Fatalf("window = %+v", window)
	}
	filters := data["filters"].(map[string]any)
	if filters["tenantLimit"].(float64) != 50 || filters["limit"].(float64) != 2 || filters["highUsageRatio"].(float64) != 0.75 {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["tenantCount"].(float64) != 2 ||
		summary["riskTenantCount"].(float64) < 1 ||
		summary["openRiskFollowUpCount"].(float64) != 1 ||
		summary["overdueRiskFollowUpCount"].(float64) != 1 ||
		summary["riskFollowUpOwnerCount"].(float64) != 2 ||
		summary["openAlertCount"].(float64) != 2 ||
		summary["taskSlaActiveCount"].(float64) != 1 ||
		summary["taskSlaOverdueCount"].(float64) != 1 ||
		summary["taskSlaOwnerCount"].(float64) != 1 ||
		summary["retryableNotificationCount"].(float64) != 2 ||
		summary["pendingNotificationCount"].(float64) != 1 ||
		summary["closedNotificationCount"].(float64) != 1 ||
		summary["windowQueueAssignmentCount"].(float64) != 1 ||
		summary["windowTaskSlaAssignCount"].(float64) != 1 ||
		summary["windowNotificationAssignCount"].(float64) != 0 ||
		summary["windowClosedNotificationAssignCount"].(float64) != 0 ||
		summary["windowOperationCount"].(float64) != 5 ||
		summary["windowRiskFollowUpCount"].(float64) != 1 ||
		summary["windowAlertResolveCount"].(float64) != 1 ||
		summary["windowNotificationRetryCount"].(float64) != 1 ||
		summary["windowNotificationCloseCount"].(float64) != 1 ||
		summary["windowBillingEventCount"].(float64) != 1 ||
		summary["windowRenewalCount"].(float64) != 1 ||
		summary["windowBillingAmountCents"].(float64) != 12345 {
		t.Fatalf("summary = %+v", summary)
	}
	notifications := data["notifications"].(map[string]any)
	notificationSummary := notifications["summary"].(map[string]any)
	if notificationSummary["failedCount"].(float64) != 1 || notificationSummary["deadCount"].(float64) != 1 || notificationSummary["closedCount"].(float64) != 1 || notificationSummary["retryableCount"].(float64) != 2 {
		t.Fatalf("notification summary = %+v", notificationSummary)
	}
	if len(notifications["items"].([]any)) != 2 {
		t.Fatalf("notifications = %+v", notifications["items"])
	}
	closedNotifications := notifications["closedItems"].([]any)
	if len(closedNotifications) != 1 || closedNotifications[0].(map[string]any)["status"] != SaaSAlertNotificationStatusClosed {
		t.Fatalf("closed notifications = %+v", closedNotifications)
	}
	taskSLA := data["taskSla"].(map[string]any)
	taskSLASummary := taskSLA["summary"].(map[string]any)
	if taskSLASummary["taskCount"].(float64) != 1 || taskSLASummary["overdueCount"].(float64) != 1 || taskSLASummary["blockedCount"].(float64) != 1 {
		t.Fatalf("task SLA summary = %+v", taskSLASummary)
	}
	taskSLATasks := taskSLA["tasks"].([]any)
	if len(taskSLATasks) != 1 || taskSLATasks[0].(map[string]any)["slaStatus"] != "overdue" {
		t.Fatalf("task SLA tasks = %+v", taskSLATasks)
	}
	assignments := data["operationQueueAssignments"].(map[string]any)
	assignmentSummary := assignments["summary"].(map[string]any)
	if assignments["assignmentCount"].(float64) != 1 ||
		assignments["returnedCount"].(float64) != 1 ||
		assignmentSummary["taskSlaCount"].(float64) != 1 {
		t.Fatalf("queue assignments = %+v", assignments)
	}
	assignmentItems := assignments["assignments"].([]any)
	if len(assignmentItems) != 1 || assignmentItems[0].(map[string]any)["owner"] != "Ops-Daily" {
		t.Fatalf("queue assignment items = %+v", assignmentItems)
	}
	billing := data["billing"].(map[string]any)
	billingSummary := billing["summary"].(map[string]any)
	if billingSummary["amountCents"].(float64) != 12345 || len(billing["billingEvents"].([]any)) != 1 {
		t.Fatalf("billing = %+v", billing)
	}
	operations := data["operations"].(map[string]any)
	if len(operations["operations"].([]any)) != 2 {
		t.Fatalf("operations limited = %+v", operations["operations"])
	}
	actions := operations["summary"].([]any)
	if len(actions) != 5 {
		t.Fatalf("actions = %+v", actions)
	}
	hasCloseAction := false
	for _, action := range actions {
		payload := action.(map[string]any)
		if payload["action"] == SaaSAdminOperationActionNotificationClose && payload["count"].(float64) == 1 {
			hasCloseAction = true
			break
		}
	}
	if !hasCloseAction {
		t.Fatalf("actions missing notification close = %+v", actions)
	}
	riskFollowUps := data["riskFollowUps"].(map[string]any)
	owners := riskFollowUps["owners"].([]any)
	if len(owners) != 2 || owners[0].(map[string]any)["owner"] != "CSM-A" {
		t.Fatalf("owners = %+v", owners)
	}
}

func TestSaaSAdminDailyReportRejectsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/dailyReport", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.DailyReport(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.overviewCalls != 0 || store.operationCalls != 0 || store.billingCalls != 0 || store.taskCalls != 0 {
		t.Fatalf("calls overview=%d operation=%d billing=%d tasks=%d", store.overviewCalls, store.operationCalls, store.billingCalls, store.taskCalls)
	}
}

func TestSaaSAdminExportCSVDailyReportAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{
				TenantCount:              1,
				ActiveTenantPackageCount: 1,
				UserCount:                5,
				CorpCount:                1,
				OpenAlertCount:           1,
			},
			Tenants: []SaaSAdminTenantOverview{{
				TenantID:       12,
				TenantName:     "风险租户",
				TenantStatus:   1,
				PackageCode:    "growth",
				PackageName:    "增长版",
				ExpiresAt:      "2026-07-15 00:00:00",
				ExpiringSoon:   true,
				OpenAlertCount: 1,
			}},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{Metric: SaaSMetricUsers, Current: 9, Limit: 10, UsageRatio: 0.9, Status: "warning", OpenAlertCount: 1}},
		},
		alertPage: SaaSAlertListPage{
			Total: 1,
			Items: []SaaSAlertRecord{{
				ID:           91,
				AlertKey:     "alert-91",
				TenantID:     12,
				AlertType:    SaaSAlertTypeQuotaExceeded,
				Status:       SaaSAlertStatusOpen,
				Metric:       SaaSMetricUsers,
				PeriodKey:    SaaSAlertPeriodLifetime,
				LastSeenAt:   "2026-07-09 10:00:00",
				CurrentValue: 9,
				LimitValue:   10,
			}},
		},
		notifications: []SaaSAlertNotification{
			{ID: 101, NotificationKey: "failed-101", TenantID: 12, Channel: "webhook", Status: SaaSAlertNotificationStatusFailed, Attempts: 1, MaxAttempts: 3, LastError: "failed", UpdatedAt: "2026-07-09 10:05:00"},
			{ID: 102, NotificationKey: "pending-102", TenantID: 12, Channel: "webhook", Status: SaaSAlertNotificationStatusPending, Attempts: 0, MaxAttempts: 3, UpdatedAt: "2026-07-09 10:06:00"},
			{ID: 103, NotificationKey: "closed-103", TenantID: 12, Channel: "webhook", Status: SaaSAlertNotificationStatusClosed, Attempts: 1, MaxAttempts: 3, LastError: "closed by admin", UpdatedAt: "2026-07-09 10:07:00"},
		},
		riskFollowUpSnapshots: []SaaSAdminRiskFollowUpSnapshot{{
			TenantID:       12,
			TenantName:     "风险租户",
			Status:         SaaSAdminRiskFollowUpStatusContacted,
			Owner:          "CSM-A",
			NextFollowUpAt: "2000-01-01 00:00:00",
			Remark:         "today follow",
			OperationID:    301,
			CreatedAt:      "2026-07-09 10:00:00",
		}},
		tasks: []SaaSAdminTask{
			{ID: 701, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusFailed, TenantID: 12, PackageCode: "growth", ActorUserID: 1, ActorTenantID: 1, Remark: "daily-export-sla", LastError: "failed", CreatedAt: "2000-01-01 00:00:00"},
		},
		operationLogs: []SaaSAdminOperationLog{
			{ID: 401, TenantID: 12, Action: "tenant.risk.follow_up", TargetType: "tenant", TargetID: "12", TargetName: "风险租户", Remark: "today follow", CreatedAt: "2026-07-09 10:00:00"},
			{ID: 403, TenantID: 12, Action: SaaSAdminOperationActionNotificationClose, TargetType: SaaSAdminOperationTargetAlertNotification, TargetID: "103", TargetName: "closed-103", Remark: "today close", CreatedAt: "2026-07-09 11:00:00"},
			{ID: 404, TenantID: 12, ActorUserID: 1, ActorTenantID: 1, Action: SaaSAdminOperationActionOperationQueueAssign, TargetType: SaaSAdminOperationTargetAdminTask, TargetID: "701", TargetName: "运营任务 SLA：tenant_renewal #701", Remark: "daily export queue assign", CreatedAt: "2026-07-09 11:30:00", AfterJSON: saasAdminPayloadJSON(map[string]any{
				"tenantId":       12,
				"source":         SaaSAdminOperationQueueSourceTaskSLA,
				"objectType":     SaaSAdminOperationTargetAdminTask,
				"objectId":       "701",
				"targetName":     "运营任务 SLA：tenant_renewal #701",
				"owner":          "Ops-Export",
				"status":         SaaSAdminRiskFollowUpStatusContacted,
				"nextFollowUpAt": "2026-07-10 11:00:00",
				"remark":         "daily export queue assign",
				"assignedAt":     "2026-07-09 11:30:00",
			})},
			{ID: 402, TenantID: 12, Action: "tenant.renewal", TargetType: "tenant", TargetID: "12", TargetName: "风险租户", Remark: "old", CreatedAt: "2026-07-08 10:00:00"},
		},
		billingEvents: []SaaSAdminBillingEvent{
			{ID: 501, TenantID: 12, EventType: "renewal", PackageCode: "growth", PackageName: "增长版", AmountCents: 12345, Currency: "CNY", ExternalOrderNo: "ORDER-TODAY", CreatedAt: "2026-07-09 13:00:00"},
			{ID: 502, TenantID: 12, EventType: "renewal", PackageCode: "growth", PackageName: "增长版", AmountCents: 99999, Currency: "CNY", ExternalOrderNo: "ORDER-OLD", CreatedAt: "2026-07-08 13:00:00"},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=dailyReport&date=2026-07-09&days=1&limit=3&tenantLimit=50&expiringDays=15&highUsageRatio=0.75", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-dailyReport") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	if store.overviewCalls != 1 || store.usageCalls != 1 || store.alertCalls != 1 || store.notificationCalls != 1 || store.operationCalls != 1 || store.billingCalls != 1 || store.riskFollowUpSnapshotCalls != 1 || store.taskCalls != 1 {
		t.Fatalf("calls overview=%d usage=%d alerts=%d notifications=%d operations=%d billing=%d followups=%d tasks=%d", store.overviewCalls, store.usageCalls, store.alertCalls, store.notificationCalls, store.operationCalls, store.billingCalls, store.riskFollowUpSnapshotCalls, store.taskCalls)
	}
	if store.lastOptions.Scope != SaaSAdminScopePlatform || store.lastOptions.Limit != 50 || store.lastOptions.ExpiringDays != 15 || store.lastOptions.DueState != SaaSAdminDueStateAll {
		t.Fatalf("overview options = %+v", store.lastOptions)
	}
	if store.lastAlertOptions.Status != SaaSAlertStatusOpen || store.lastAlertOptions.PerPage != 3 {
		t.Fatalf("alert options = %+v", store.lastAlertOptions)
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) < 10 {
		t.Fatalf("records = %+v", records)
	}
	if got := records[0]; len(got) != 4 || got[0] != "section" || got[1] != "metric" || got[2] != "value" || got[3] != "remark" {
		t.Fatalf("header = %+v", records[0])
	}
	assertSaaSAdminCSVRowPrefix(t, records, []string{"summary", "tenantCount", "1"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"summary", "windowBillingAmountCents", "12345"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"summary", "taskSlaOverdueCount", "1"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"summary", "closedNotificationCount", "1"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"summary", "windowNotificationCloseCount", "1"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"summary", "windowQueueAssignmentCount", "1"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"summary", "windowTaskSlaAssignCount", "1"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"riskOwner", "CSM-A", "1"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"taskSlaOwner", "用户 #1 / 租户 #1", "1"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"taskSlaTask", "tenant_renewal overdue", "701"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"notification", "failed failed-101", "12"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"closedNotification", "closed closed-103", "12"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"operationQueueAssignment", "task_sla contacted", "Ops-Export"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"operationAction", "tenant.risk.follow_up", "1"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"billing", "renewal growth", "12345"})
}

func TestSaaSAdminExportCSVRenewalForecastAllowsPlatformAdmin(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.Local)
	expiresAt := func(days int) string {
		return today.AddDate(0, 0, days).Format("2006-01-02 15:04:05")
	}
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 3},
			Tenants: []SaaSAdminTenantOverview{
				{TenantID: 12, TenantName: "30天内规模租户", TenantStatus: 1, PackageCode: "scale", PackageName: "规模版", PackageStatus: 1, ExpiresAt: expiresAt(20), ExpiringSoon: true},
				{TenantID: 13, TenantName: "未知价格企业租户", TenantStatus: 1, PackageCode: "enterprise", PackageName: "企业版", PackageStatus: 1, ExpiresAt: expiresAt(45), ExpiringSoon: true},
				{TenantID: 14, TenantName: "窗口外增长租户", TenantStatus: 1, PackageCode: "growth", PackageName: "增长版", PackageStatus: 1, ExpiresAt: expiresAt(120)},
			},
		},
		billingEvents: []SaaSAdminBillingEvent{
			{ID: 4002, TenantID: 92, EventType: "renewal", PackageCode: "scale", PackageName: "规模版", AmountCents: 2400000, PaidAt: today.AddDate(0, 0, -8).Format("2006-01-02 15:04:05"), CreatedAt: today.AddDate(0, 0, -8).Format("2006-01-02 15:04:05")},
			{ID: 4999, TenantID: 93, EventType: "setup", PackageCode: "enterprise", PackageName: "企业版", AmountCents: 880000, CreatedAt: today.AddDate(0, 0, -7).Format("2006-01-02 15:04:05")},
		},
		tasks: []SaaSAdminTask{
			{ID: 5003, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusPending, TenantID: 12, PackageCode: "scale", ActorUserID: 1, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
			{ID: 5004, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusApplied, TenantID: 13, PackageCode: "enterprise", ActorUserID: 2, CreatedAt: today.AddDate(0, 0, -2).Format("2006-01-02 15:04:05")},
		},
		latestRiskFollowUps: map[int]SaaSAdminRiskFollowUpSnapshot{
			12: {TenantID: 12, TenantName: "30天内规模租户", Status: SaaSAdminRiskFollowUpStatusRenewalPending, Owner: "CSM-A", NextFollowUpAt: "2026-07-20 00:00:00", OperationID: 9001, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=renewalForecast&tenantLimit=20&days=90&billingLimit=100&taskLimit=50", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-renewalForecast") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	if store.lastOptions.Scope != SaaSAdminScopePlatform || store.lastOptions.Limit != 20 || store.lastOptions.ExpiringDays != 90 || store.lastOptions.DueState != SaaSAdminDueStateAll {
		t.Fatalf("overview options = %+v", store.lastOptions)
	}
	if store.lastBillingOptions.EventType != "renewal" || store.lastBillingOptions.Limit != 100 || store.lastTaskOptions.TaskType != SaaSAdminTaskTypeTenantRenewal || store.lastTaskOptions.Limit != 50 {
		t.Fatalf("billing=%+v task=%+v", store.lastBillingOptions, store.lastTaskOptions)
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 3 {
		t.Fatalf("records = %+v", records)
	}
	header := records[0]
	if header[0] != "tenantId" || header[9] != "renewalAmountCents" || header[15] != "actionableTaskCount" || header[21] != "latestTaskStatus" {
		t.Fatalf("header = %+v", header)
	}
	if header[23] != "owner" || header[24] != "riskFollowUpStatus" || header[25] != "riskFollowUpNextAt" {
		t.Fatalf("header = %+v", header)
	}
	scale := records[1]
	if scale[0] != "12" ||
		scale[1] != "30天内规模租户" ||
		scale[3] != "scale" ||
		scale[6] != "due_0_30" ||
		scale[9] != "2400000" ||
		scale[11] != "true" ||
		scale[12] != "4002" ||
		scale[15] != "1" ||
		scale[20] != "5003" ||
		scale[21] != SaaSAdminTaskStatusPending {
		t.Fatalf("scale row = %+v", scale)
	}
	if scale[23] != "CSM-A" || scale[24] != SaaSAdminRiskFollowUpStatusRenewalPending || scale[25] != "2026-07-20 00:00:00" {
		t.Fatalf("scale row = %+v", scale)
	}
	unknown := records[2]
	if unknown[0] != "13" || unknown[6] != "due_31_60" || unknown[9] != "0" || unknown[11] != "false" || unknown[19] != "1" || unknown[21] != SaaSAdminTaskStatusApplied {
		t.Fatalf("unknown row = %+v", unknown)
	}
}

func TestSaaSAdminExportCSVRenewalForecastOwnersAllowsPlatformAdmin(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.Local)
	expiresAt := func(days int) string {
		return today.AddDate(0, 0, days).Format("2006-01-02 15:04:05")
	}
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 3},
			Tenants: []SaaSAdminTenantOverview{
				{TenantID: 12, TenantName: "30天内规模租户", TenantStatus: 1, PackageCode: "scale", PackageName: "规模版", PackageStatus: 1, ExpiresAt: expiresAt(20), ExpiringSoon: true},
				{TenantID: 13, TenantName: "未知价格企业租户", TenantStatus: 1, PackageCode: "enterprise", PackageName: "企业版", PackageStatus: 1, ExpiresAt: expiresAt(45), ExpiringSoon: true},
				{TenantID: 14, TenantName: "窗口外增长租户", TenantStatus: 1, PackageCode: "growth", PackageName: "增长版", PackageStatus: 1, ExpiresAt: expiresAt(120)},
			},
		},
		billingEvents: []SaaSAdminBillingEvent{
			{ID: 4002, TenantID: 92, EventType: "renewal", PackageCode: "scale", PackageName: "规模版", AmountCents: 2400000, PaidAt: today.AddDate(0, 0, -8).Format("2006-01-02 15:04:05"), CreatedAt: today.AddDate(0, 0, -8).Format("2006-01-02 15:04:05")},
		},
		tasks: []SaaSAdminTask{
			{ID: 5003, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusPending, TenantID: 12, PackageCode: "scale", ActorUserID: 1, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
			{ID: 5004, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusApplied, TenantID: 13, PackageCode: "enterprise", ActorUserID: 2, CreatedAt: today.AddDate(0, 0, -2).Format("2006-01-02 15:04:05")},
		},
		latestRiskFollowUps: map[int]SaaSAdminRiskFollowUpSnapshot{
			12: {TenantID: 12, TenantName: "30天内规模租户", Status: SaaSAdminRiskFollowUpStatusRenewalPending, Owner: "CSM-A", NextFollowUpAt: "2026-07-20 00:00:00", OperationID: 9001, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=renewalForecastOwners&tenantLimit=20&days=90&billingLimit=100&taskLimit=50", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-renewalForecastOwners") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 3 {
		t.Fatalf("records = %+v", records)
	}
	header := records[0]
	if header[0] != "owner" || header[4] != "renewalAmountCents" || header[7] != "dueWithin30TenantCount" || header[11] != "actionableTaskCount" || header[18] != "topTenants" {
		t.Fatalf("header = %+v", header)
	}
	csm := records[1]
	if csm[0] != "CSM-A" ||
		csm[1] != "1" ||
		csm[2] != "1" ||
		csm[3] != "0" ||
		csm[4] != "2400000" ||
		csm[7] != "1" ||
		csm[11] != "1" ||
		csm[12] != "1" ||
		csm[15] != "0" ||
		csm[17] != "2026-07-20 00:00:00" ||
		!strings.Contains(csm[18], "30天内规模租户#12(due_0_30,2400000)") {
		t.Fatalf("csm row = %+v", csm)
	}
	unassigned := records[2]
	if unassigned[0] != "未分配" ||
		unassigned[1] != "1" ||
		unassigned[2] != "0" ||
		unassigned[3] != "1" ||
		unassigned[8] != "1" ||
		unassigned[15] != "1" ||
		!strings.Contains(unassigned[18], "未知价格企业租户#13(due_31_60,unknown)") {
		t.Fatalf("unassigned row = %+v", unassigned)
	}
}

func TestSaaSAdminExportCSVTenantsAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Tenants: []SaaSAdminTenantOverview{{
				TenantID:        12,
				TenantName:      "租户B",
				TenantStatus:    1,
				PackageCode:     "growth",
				PackageName:     "增长版",
				PackageStatus:   1,
				ExpiresAt:       "2028-01-01 00:00:00",
				ExpiringSoon:    true,
				OpenAlertCount:  2,
				MaxUsageMetric:  SaaSMetricUsers,
				MaxUsageCurrent: 8,
				MaxUsageLimit:   10,
				MaxUsageRatio:   0.8,
			}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=tenants&keyword=%E7%A7%9F%E6%88%B7&tenantStatus=1&packageCode=growth&dueState=normal&limit=2000&expiringDays=44", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/csv") || !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-tenants") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	if store.lastOptions.Scope != SaaSAdminScopePlatform ||
		store.lastOptions.Limit != 2000 ||
		store.lastOptions.ExpiringDays != 44 ||
		store.lastOptions.Keyword != "租户" ||
		store.lastOptions.TenantStatus != 1 ||
		store.lastOptions.PackageCode != "growth" ||
		store.lastOptions.DueState != SaaSAdminDueStateNormal {
		t.Fatalf("options = %+v", store.lastOptions)
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 2 {
		t.Fatalf("records = %+v", records)
	}
	if records[0][0] != "tenantId" || records[0][13] != "maxUsageRatio" {
		t.Fatalf("header = %+v", records[0])
	}
	row := records[1]
	if row[0] != "12" || row[1] != "租户B" || row[3] != "growth" || row[8] != "true" || row[13] != "0.800000" {
		t.Fatalf("row = %+v", row)
	}
}

func TestSaaSAdminExportCSVTenantLifecycleAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Tenants: []SaaSAdminTenantOverview{{
				TenantID:     12,
				TenantName:   "租户B",
				TenantStatus: 1,
				PackageCode:  "growth",
				PackageName:  "增长版",
			}},
		},
		operationLogs: []SaaSAdminOperationLog{{
			ID:         91,
			TenantID:   12,
			Action:     "tenant.package",
			TargetType: "tenant",
			TargetID:   "12",
			TargetName: "租户B",
			Remark:     "升级套餐",
			CreatedAt:  "2026-07-09 15:30:00",
		}},
		notifications: []SaaSAlertNotification{{
			ID:              95,
			NotificationKey: "12:tenant_renewal:tenant_renewal_reminder:renewal_20260808:webhook",
			AlertKey:        "12:tenant_renewal:tenant_renewal_reminder:renewal_20260808",
			TenantID:        12,
			Channel:         SaaSAlertNotificationChannelWebhook,
			Status:          SaaSAlertNotificationStatusPending,
			MaxAttempts:     4,
			Alert: SaaSQuotaAlert{
				Status:    SaaSQuotaStatus{TenantID: 12, Metric: SaaSEventMetricTenantRenewal},
				AlertType: SaaSAlertTypeTenantRenewal,
				PeriodKey: "renewal_20260808",
				Message:   "租户B 即将到期，请跟进续费提醒",
			},
			UpdatedAt: "2026-07-09 18:00:00",
		}, {
			ID:              96,
			NotificationKey: "12:users:quota_exceeded:lifetime:webhook",
			AlertKey:        "12:users:quota_exceeded:lifetime",
			TenantID:        12,
			Channel:         SaaSAlertNotificationChannelWebhook,
			Status:          SaaSAlertNotificationStatusDead,
			MaxAttempts:     3,
			Alert: SaaSQuotaAlert{
				Status:    SaaSQuotaStatus{TenantID: 12, Metric: SaaSMetricUsers},
				AlertType: SaaSAlertTypeQuotaExceeded,
				PeriodKey: SaaSAlertPeriodLifetime,
				Message:   "子账号数接近上限",
			},
			LastError: "webhook failed",
			UpdatedAt: "2026-07-09 17:00:00",
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=tenantLifecycle&tenantId=12&source=notification&status=pending&eventType=tenant_renewal_reminder&keyword=续费提醒&limit=9999&expiringDays=44", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-tenantLifecycle") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	if store.lastOptions.Scope != SaaSAdminScopeTenant ||
		store.lastOptions.TenantID != 12 ||
		store.lastOptions.Limit != 1 ||
		store.lastOptions.ExpiringDays != 44 {
		t.Fatalf("overview options = %+v", store.lastOptions)
	}
	if store.lastOperationOptions.TenantID != 12 || store.lastOperationOptions.Limit != saasAdminExportMaxLimit ||
		store.lastNotificationOptions.TenantID != 12 || store.lastNotificationOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("options operation=%+v notification=%+v", store.lastOperationOptions, store.lastNotificationOptions)
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 2 || records[0][0] != "tenantId" || records[0][12] != "payload" {
		t.Fatalf("records = %+v", records)
	}
	row := records[1]
	if row[0] != "12" || row[1] != "租户B" || row[4] != "notification" || row[5] != SaaSAlertTypeTenantRenewal || row[7] != SaaSAlertNotificationStatusPending || row[9] != "95" {
		t.Fatalf("row = %+v", row)
	}
	if !strings.Contains(row[6], "tenant_renewal_reminder") || !strings.Contains(row[12], `"tenantId":12`) || !strings.Contains(row[12], `"alertType":"tenant_renewal_reminder"`) {
		t.Fatalf("row payload = %+v", row)
	}
}

func TestSaaSAdminExportCSVUsageAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Tenants: []SaaSAdminTenantOverview{{
				TenantID:      12,
				TenantName:    "租户B",
				TenantStatus:  1,
				PackageCode:   "growth",
				PackageName:   "增长版",
				PackageStatus: 1,
				ExpiresAt:     "2028-01-01 00:00:00",
			}},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{
				Metric:         SaaSMetricUsers,
				PeriodKey:      SaaSAlertPeriodLifetime,
				Current:        9,
				Limit:          10,
				Remaining:      1,
				UsageRatio:     0.9,
				Status:         "warning",
				OpenAlertCount: 2,
				UpdatedBy:      "smoke",
				UpdatedAt:      "2026-07-09 18:40:00",
			}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=usage&keyword=%E7%A7%9F%E6%88%B7&tenantStatus=1&packageCode=growth&dueState=normal&limit=2000&expiringDays=44", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-usage") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	if store.lastOptions.Scope != SaaSAdminScopePlatform ||
		store.lastOptions.Limit != 2000 ||
		store.lastOptions.ExpiringDays != 44 ||
		store.lastOptions.Keyword != "租户" ||
		store.lastOptions.TenantStatus != 1 ||
		store.lastOptions.PackageCode != "growth" ||
		store.lastOptions.DueState != SaaSAdminDueStateNormal {
		t.Fatalf("options = %+v", store.lastOptions)
	}
	if store.usageCalls != 1 || store.lastUsageTenantID != 12 {
		t.Fatalf("usage calls=%d tenant=%d", store.usageCalls, store.lastUsageTenantID)
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 2 {
		t.Fatalf("records = %+v", records)
	}
	header := records[0]
	if header[0] != "tenantId" || header[6] != "metric" || header[10] != "usageLimit" || header[17] != "updatedAt" {
		t.Fatalf("header = %+v", header)
	}
	row := records[1]
	if row[0] != "12" ||
		row[1] != "租户B" ||
		row[3] != "growth" ||
		row[6] != SaaSMetricUsers ||
		row[7] != "子账号数" ||
		row[9] != "9" ||
		row[10] != "10" ||
		row[13] != "0.900000" ||
		row[14] != "warning" ||
		row[15] != "2" ||
		row[16] != "smoke" {
		t.Fatalf("row = %+v", row)
	}
}

func TestSaaSAdminExportCSVPackagesAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		packages: []SaaSAdminPackage{{
			Code:        "growth",
			Name:        "增长版",
			Description: "增长套餐",
			Status:      1,
			Limits: SaaSAdminPackageLimits{
				MaxCorps:              2,
				MaxUsers:              10,
				MaxContacts:           1000,
				ChannelCodes:          12,
				SensitiveWords:        30,
				StorageMB:             2048,
				ContactMessageBatches: 5,
				AsyncExecutions:       1000,
			},
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=packages", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-packages") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	if store.packageCalls != 1 {
		t.Fatalf("package calls = %d", store.packageCalls)
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 2 {
		t.Fatalf("records = %+v", records)
	}
	header := records[0]
	if header[0] != "code" || header[3] != "status" || header[5] != "maxUsers" || header[22] != "storageMb" || header[29] != "asyncExecutions" {
		t.Fatalf("header = %+v", header)
	}
	row := records[1]
	if row[0] != "growth" ||
		row[1] != "增长版" ||
		row[2] != "增长套餐" ||
		row[3] != "1" ||
		row[4] != "2" ||
		row[5] != "10" ||
		row[6] != "1000" ||
		row[9] != "12" ||
		row[21] != "30" ||
		row[22] != "2048" ||
		row[23] != "5" ||
		row[29] != "1000" {
		t.Fatalf("row = %+v", row)
	}
}

func TestSaaSAdminExportCSVRiskAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 2},
			Tenants: []SaaSAdminTenantOverview{
				{
					TenantID:        12,
					TenantName:      "风险租户",
					TenantStatus:    1,
					PackageCode:     "growth",
					PackageName:     "增长版",
					PackageStatus:   1,
					ExpiresAt:       "2026-07-20 00:00:00",
					ExpiringSoon:    true,
					OpenAlertCount:  1,
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 9,
					MaxUsageLimit:   10,
					MaxUsageRatio:   0.9,
				},
				{
					TenantID:        13,
					TenantName:      "普通租户",
					TenantStatus:    1,
					PackageCode:     "growth",
					PackageName:     "增长版",
					PackageStatus:   1,
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 1,
					MaxUsageLimit:   10,
					MaxUsageRatio:   0.1,
				},
			},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{
				Metric:         SaaSMetricUsers,
				PeriodKey:      SaaSAlertPeriodLifetime,
				Current:        9,
				Limit:          10,
				Remaining:      1,
				UsageRatio:     0.9,
				Status:         "warning",
				OpenAlertCount: 1,
			}},
			13: {{
				Metric:     SaaSMetricUsers,
				PeriodKey:  SaaSAlertPeriodLifetime,
				Current:    1,
				Limit:      10,
				Remaining:  9,
				UsageRatio: 0.1,
				Status:     "normal",
			}},
		},
		latestRiskFollowUps: map[int]SaaSAdminRiskFollowUpSnapshot{
			12: {
				TenantID:       12,
				TenantName:     "风险租户",
				Status:         SaaSAdminRiskFollowUpStatusContacted,
				Owner:          "CSM-B",
				NextFollowUpAt: "2026-07-21 00:00:00",
				Remark:         "已约续费沟通",
				OperationID:    77,
				CreatedAt:      "2026-07-09 12:00:00",
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=risk&keyword=%E9%A3%8E%E9%99%A9&tenantStatus=1&packageCode=growth&dueState=expiring&limit=2000&expiringDays=20&highUsageRatio=0.85", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-risk") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	if store.lastOptions.Scope != SaaSAdminScopePlatform ||
		store.lastOptions.Limit != 2000 ||
		store.lastOptions.ExpiringDays != 20 ||
		store.lastOptions.Keyword != "风险" ||
		store.lastOptions.TenantStatus != 1 ||
		store.lastOptions.PackageCode != "growth" ||
		store.lastOptions.DueState != SaaSAdminDueStateExpiring {
		t.Fatalf("options = %+v", store.lastOptions)
	}
	if store.usageCalls != 2 {
		t.Fatalf("usage calls = %d", store.usageCalls)
	}
	if store.latestRiskFollowUpCalls != 1 {
		t.Fatalf("risk follow-up calls = %d", store.latestRiskFollowUpCalls)
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 3 {
		t.Fatalf("records = %+v", records)
	}
	if records[0][0] != "tenantId" || records[0][2] != "riskLevel" || records[0][15] != "followUpStatus" || records[0][len(records[0])-1] != "topUsageMetrics" {
		t.Fatalf("header = %+v", records[0])
	}
	row := records[1]
	if row[0] != "12" || row[1] != "风险租户" || row[2] != "high" || row[4] == "" || row[5] == "" || row[13] != SaaSMetricUsers || row[14] != "0.900000" || row[15] != SaaSAdminRiskFollowUpStatusContacted || row[16] != "CSM-B" || row[17] != "2026-07-21 00:00:00" || row[18] != "已约续费沟通" || !strings.Contains(row[len(row)-1], "users=9/10") {
		t.Fatalf("risk row = %+v", row)
	}
}

func TestSaaSAdminExportCSVRiskFollowUpsAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		riskFollowUpSnapshots: []SaaSAdminRiskFollowUpSnapshot{{
			TenantID:       12,
			TenantName:     "风险租户",
			Status:         SaaSAdminRiskFollowUpStatusContacted,
			Owner:          "smoke-csm",
			NextFollowUpAt: "2000-01-01 00:00:00",
			Remark:         "renewal follow-up export",
			OperationID:    301,
			CreatedAt:      "2026-07-09 18:30:00",
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=riskFollowUps&tenantId=12&status=contacted&owner=smoke&keyword=renewal&dueState=overdue&limit=2000", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-riskFollowUps") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	if store.riskFollowUpSnapshotCalls != 1 {
		t.Fatalf("risk follow-up snapshot calls = %d", store.riskFollowUpSnapshotCalls)
	}
	options := store.lastRiskFollowUpTaskOptions
	if options.TenantID != 12 ||
		options.Status != SaaSAdminRiskFollowUpStatusContacted ||
		options.Owner != "smoke" ||
		options.Keyword != "renewal" ||
		options.DueState != SaaSAdminRiskFollowUpDueStateAll ||
		options.Limit != saasAdminExportMaxLimit {
		t.Fatalf("store options = %+v", options)
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 2 {
		t.Fatalf("records = %+v", records)
	}
	header := records[0]
	if header[0] != "operationId" || header[1] != "tenantId" || header[6] != "dueState" || header[10] != "createdAt" {
		t.Fatalf("header = %+v", header)
	}
	row := records[1]
	if row[0] != "301" ||
		row[1] != "12" ||
		row[2] != "风险租户" ||
		row[3] != SaaSAdminRiskFollowUpStatusContacted ||
		row[4] != "smoke-csm" ||
		row[5] != "2000-01-01 00:00:00" ||
		row[6] != SaaSAdminRiskFollowUpDueStateOverdue ||
		row[7] != "true" ||
		row[9] != "renewal follow-up export" ||
		row[10] != "2026-07-09 18:30:00" {
		t.Fatalf("row = %+v", row)
	}
}

func TestSaaSAdminExportCSVRiskFollowUpOwnersAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		riskFollowUpSnapshots: []SaaSAdminRiskFollowUpSnapshot{
			{
				TenantID:       12,
				TenantName:     "风险租户",
				Status:         SaaSAdminRiskFollowUpStatusContacted,
				Owner:          "smoke-csm",
				NextFollowUpAt: "2000-01-01 00:00:00",
				Remark:         "risk owner-export open",
				OperationID:    301,
				CreatedAt:      "2026-07-09 18:30:00",
			},
			{
				TenantID:    12,
				TenantName:  "风险租户",
				Status:      SaaSAdminRiskFollowUpStatusResolved,
				Owner:       "smoke-csm",
				Remark:      "risk owner-export closed",
				OperationID: 302,
				CreatedAt:   "2026-07-10 18:30:00",
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=riskFollowUpOwners&tenantId=12&owner=smoke&keyword=owner-export&limit=2000", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-riskFollowUpOwners") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	if store.riskFollowUpSnapshotCalls != 1 {
		t.Fatalf("risk follow-up snapshot calls = %d", store.riskFollowUpSnapshotCalls)
	}
	options := store.lastRiskFollowUpTaskOptions
	if options.TenantID != 12 ||
		options.Owner != "smoke" ||
		options.Keyword != "owner-export" ||
		options.DueState != SaaSAdminRiskFollowUpDueStateAll ||
		options.Limit != saasAdminExportMaxLimit {
		t.Fatalf("store options = %+v", options)
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 2 {
		t.Fatalf("records = %+v", records)
	}
	header := records[0]
	if header[0] != "owner" || header[2] != "openCount" || header[12] != "closedCount" || header[14] != "nextFollowUpAt" {
		t.Fatalf("header = %+v", header)
	}
	row := records[1]
	if row[0] != "smoke-csm" ||
		row[1] != "2" ||
		row[2] != "1" ||
		row[4] != "1" ||
		row[6] != "1" ||
		row[8] != "1" ||
		row[12] != "1" ||
		row[13] != "2026-07-10 18:30:00" ||
		row[14] != "2000-01-01 00:00:00" {
		t.Fatalf("row = %+v", row)
	}
}

func TestSaaSAdminExportCSVBillingReconciliationFollowUpsAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		billingFollowUpSnapshots: []SaaSAdminBillingReconciliationFollowUpSnapshot{{
			OperationID:     401,
			BillingEventID:  93,
			TenantID:        12,
			TenantName:      "账单租户",
			Status:          SaaSAdminRiskFollowUpStatusContacted,
			Owner:           "finance",
			NextFollowUpAt:  "2000-01-01 00:00:00",
			Remark:          "billing follow-up export",
			PackageCode:     "growth",
			PackageName:     "增长版",
			NewExpiresAt:    "2031-02-03 00:00:00",
			AmountCents:     880000,
			Currency:        "CNY",
			ExternalOrderNo: "DRIFT-93",
			CreatedAt:       "2026-07-10 10:30:00",
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=billingReconciliationFollowUps&tenantId=12&status=contacted&owner=fin&keyword=DRIFT-93&dueState=overdue&limit=2000", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-billingReconciliationFollowUps") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	if store.billingFollowUpCalls != 1 {
		t.Fatalf("billing follow-up calls = %d", store.billingFollowUpCalls)
	}
	options := store.lastBillingFollowUpOptions
	if options.TenantID != 12 ||
		options.Status != SaaSAdminRiskFollowUpStatusContacted ||
		options.Owner != "fin" ||
		options.Keyword != "DRIFT-93" ||
		options.DueState != SaaSAdminRiskFollowUpDueStateAll ||
		options.Limit != saasAdminExportMaxLimit {
		t.Fatalf("store options = %+v", options)
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 2 {
		t.Fatalf("records = %+v", records)
	}
	header := records[0]
	if header[0] != "operationId" || header[1] != "billingEventId" || header[7] != "dueState" || header[16] != "externalOrderNo" || header[17] != "createdAt" {
		t.Fatalf("header = %+v", header)
	}
	row := records[1]
	if row[0] != "401" ||
		row[1] != "93" ||
		row[2] != "12" ||
		row[3] != "账单租户" ||
		row[4] != SaaSAdminRiskFollowUpStatusContacted ||
		row[5] != "finance" ||
		row[6] != "2000-01-01 00:00:00" ||
		row[7] != SaaSAdminRiskFollowUpDueStateOverdue ||
		row[8] != "true" ||
		row[10] != "billing follow-up export" ||
		row[11] != "growth" ||
		row[12] != "增长版" ||
		row[13] != "2031-02-03 00:00:00" ||
		row[14] != "880000" ||
		row[15] != "CNY" ||
		row[16] != "DRIFT-93" ||
		row[17] != "2026-07-10 10:30:00" {
		t.Fatalf("row = %+v", row)
	}
}

func TestSaaSAdminExportCSVBillingReconciliationFollowUpOwnersAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		billingFollowUpSnapshots: []SaaSAdminBillingReconciliationFollowUpSnapshot{
			{
				OperationID:     401,
				BillingEventID:  93,
				TenantID:        12,
				TenantName:      "账单租户",
				Status:          SaaSAdminRiskFollowUpStatusContacted,
				Owner:           "finance",
				NextFollowUpAt:  "2000-01-01 00:00:00",
				Remark:          "billing owner-export open",
				PackageCode:     "growth",
				PackageName:     "增长版",
				AmountCents:     880000,
				Currency:        "CNY",
				ExternalOrderNo: "DRIFT-93",
				CreatedAt:       "2026-07-09 10:30:00",
			},
			{
				OperationID:     402,
				BillingEventID:  94,
				TenantID:        12,
				TenantName:      "账单租户",
				Status:          SaaSAdminRiskFollowUpStatusResolved,
				Owner:           "finance",
				Remark:          "billing owner-export closed",
				PackageCode:     "growth",
				PackageName:     "增长版",
				AmountCents:     880000,
				Currency:        "CNY",
				ExternalOrderNo: "DRIFT-94",
				CreatedAt:       "2026-07-10 10:30:00",
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=billingReconciliationFollowUpOwners&tenantId=12&owner=fin&keyword=owner-export&limit=2000", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-billingReconciliationFollowUpOwners") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	if store.billingFollowUpCalls != 1 {
		t.Fatalf("billing follow-up calls = %d", store.billingFollowUpCalls)
	}
	options := store.lastBillingFollowUpOptions
	if options.TenantID != 12 ||
		options.Owner != "fin" ||
		options.Keyword != "owner-export" ||
		options.DueState != SaaSAdminRiskFollowUpDueStateAll ||
		options.Limit != saasAdminExportMaxLimit {
		t.Fatalf("store options = %+v", options)
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 2 {
		t.Fatalf("records = %+v", records)
	}
	header := records[0]
	if header[0] != "owner" || header[2] != "openCount" || header[12] != "closedCount" || header[14] != "nextFollowUpAt" {
		t.Fatalf("header = %+v", header)
	}
	row := records[1]
	if row[0] != "finance" ||
		row[1] != "2" ||
		row[2] != "1" ||
		row[4] != "1" ||
		row[6] != "1" ||
		row[8] != "1" ||
		row[12] != "1" ||
		row[13] != "2026-07-10 10:30:00" ||
		row[14] != "2000-01-01 00:00:00" {
		t.Fatalf("row = %+v", row)
	}
}

func TestSaaSAdminExportCSVTasksAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{
			{
				ID:            301,
				TaskType:      SaaSAdminTaskTypeTenantProvision,
				Status:        SaaSAdminTaskStatusApplied,
				TenantID:      44,
				PackageCode:   "growth",
				ActorUserID:   1,
				ActorTenantID: 1,
				RequestJSON:   `{"tenantName":"任务开户租户","password":"secret989","adminPasswordHash":"hash989","packageCode":"growth"}`,
				PreviewJSON:   `{"tenantName":"任务开户租户","packageCode":"growth"}`,
				ResultJSON:    `{"tenantId":44,"adminUserId":88}`,
				Remark:        "任务导出",
				AppliedAt:     "2026-07-09 18:50:00",
				CreatedAt:     "2026-07-09 18:40:00",
				UpdatedAt:     "2026-07-09 18:50:00",
			},
			{
				ID:          302,
				TaskType:    SaaSAdminTaskTypePackageSync,
				Status:      SaaSAdminTaskStatusApplied,
				TenantID:    44,
				PackageCode: "growth",
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=tasks&taskType=tenant_provision&status=applied&tenantId=44&packageCode=growth&limit=9999", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-tasks") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	if store.lastTaskOptions.TaskType != SaaSAdminTaskTypeTenantProvision ||
		store.lastTaskOptions.Status != SaaSAdminTaskStatusApplied ||
		store.lastTaskOptions.TenantID != 44 ||
		store.lastTaskOptions.PackageCode != "growth" ||
		store.lastTaskOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("task options = %+v", store.lastTaskOptions)
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 2 {
		t.Fatalf("records = %+v", records)
	}
	header := records[0]
	if header[0] != "id" || header[1] != "taskType" || header[12] != "request" || header[14] != "result" {
		t.Fatalf("header = %+v", header)
	}
	row := records[1]
	if row[0] != "301" || row[1] != SaaSAdminTaskTypeTenantProvision || row[2] != SaaSAdminTaskStatusApplied || row[3] != "44" || row[4] != "growth" || row[7] != "任务导出" {
		t.Fatalf("row = %+v", row)
	}
	if strings.Contains(row[12], "secret989") || strings.Contains(row[12], "adminPasswordHash") || strings.Contains(row[12], `"password"`) {
		t.Fatalf("request leaked secret: %s", row[12])
	}
	var requestPayload map[string]any
	if err := json.Unmarshal([]byte(row[12]), &requestPayload); err != nil {
		t.Fatalf("request json = %q err=%v", row[12], err)
	}
	if requestPayload["tenantName"] != "任务开户租户" || requestPayload["hasAdminPasswordHash"] != true {
		t.Fatalf("request payload = %+v", requestPayload)
	}
	if !strings.Contains(row[14], `"tenantId":44`) {
		t.Fatalf("result = %s", row[14])
	}
}

func TestSaaSAdminExportCSVTaskSLAAllowsPlatformAdmin(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	format := func(t time.Time) string {
		return t.Format("2006-01-02 15:04:05")
	}
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{
			{
				ID:            501,
				TaskType:      SaaSAdminTaskTypePackageSync,
				Status:        SaaSAdminTaskStatusBlocked,
				TenantID:      44,
				PackageCode:   "scale",
				ActorUserID:   1,
				ActorTenantID: 1,
				Remark:        "超时任务",
				LastError:     "额度阻断",
				CreatedAt:     format(now.Add(-30 * time.Hour)),
				UpdatedAt:     format(now.Add(-29 * time.Hour)),
			},
			{
				ID:            502,
				TaskType:      SaaSAdminTaskTypeTenantProvision,
				Status:        SaaSAdminTaskStatusPending,
				TenantID:      0,
				PackageCode:   "scale",
				ActorUserID:   2,
				ActorTenantID: 1,
				RequestJSON:   `{"tenantName":"SLA开户租户","password":"secret502","adminPasswordHash":"hash502","packageCode":"scale"}`,
				PreviewJSON:   `{"tenantName":"SLA开户租户","packageCode":"scale"}`,
				CreatedAt:     format(now.Add(-5 * time.Hour)),
			},
			{
				ID:            503,
				TaskType:      SaaSAdminTaskTypePackageSync,
				Status:        SaaSAdminTaskStatusApplied,
				TenantID:      44,
				PackageCode:   "scale",
				ActorUserID:   1,
				ActorTenantID: 1,
				CreatedAt:     format(now.Add(-50 * time.Hour)),
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=taskSla&taskType=all&status=all&packageCode=scale&warningHours=4&overdueHours=24&limit=1000", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-taskSla") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	if store.lastTaskOptions.TaskType != "" ||
		store.lastTaskOptions.Status != "" ||
		store.lastTaskOptions.PackageCode != "scale" ||
		store.lastTaskOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("task options = %+v", store.lastTaskOptions)
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 3 {
		t.Fatalf("records = %+v", records)
	}
	header := records[0]
	if header[0] != "taskId" || header[5] != "owner" || header[8] != "slaStatus" || header[16] != "request" || header[18] != "result" {
		t.Fatalf("header = %+v", header)
	}
	overdue := records[1]
	if overdue[0] != "501" || overdue[1] != SaaSAdminTaskTypePackageSync || overdue[2] != SaaSAdminTaskStatusBlocked || overdue[8] != "overdue" || overdue[11] != "超时任务" || overdue[12] != "额度阻断" {
		t.Fatalf("overdue row = %+v", overdue)
	}
	var ageHours int
	var breachHours int
	if _, err := fmt.Sscanf(overdue[9], "%d", &ageHours); err != nil || ageHours < 30 {
		t.Fatalf("ageHours = %q err=%v", overdue[9], err)
	}
	if _, err := fmt.Sscanf(overdue[10], "%d", &breachHours); err != nil || breachHours < 6 {
		t.Fatalf("breachHours = %q err=%v", overdue[10], err)
	}
	provision := records[2]
	if provision[0] != "502" || provision[1] != SaaSAdminTaskTypeTenantProvision || provision[8] != "warning" {
		t.Fatalf("provision row = %+v", provision)
	}
	if strings.Contains(provision[16], "secret502") || strings.Contains(provision[16], "adminPasswordHash") || strings.Contains(provision[16], `"password"`) {
		t.Fatalf("request leaked secret: %s", provision[16])
	}
}

func TestSaaSAdminExportCSVAlertsAndNotifications(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		alertPage: SaaSAlertListPage{
			Total:     1,
			TotalPage: 1,
			Items: []SaaSAlertRecord{{
				ID:              41,
				AlertKey:        "12:users:quota_exceeded:lifetime",
				TenantID:        12,
				AlertType:       SaaSAlertTypeQuotaExceeded,
				Severity:        SaaSAlertSeverityWarning,
				Status:          SaaSAlertStatusOpen,
				Metric:          SaaSMetricUsers,
				PeriodKey:       SaaSAlertPeriodLifetime,
				CurrentValue:    8,
				LimitValue:      10,
				AdditionalValue: 1,
				OccurrenceCount: 2,
				Source:          "smoke",
				Message:         "子账号数接近上限",
				ContextJSON:     `{"source":"smoke"}`,
				LastSeenAt:      "2026-07-09 16:30:00",
			}},
		},
		notifications: []SaaSAlertNotification{{
			ID:              77,
			NotificationKey: "12:users:quota_exceeded:lifetime:webhook",
			AlertKey:        "12:users:quota_exceeded:lifetime",
			TenantID:        12,
			Channel:         SaaSAlertNotificationChannelWebhook,
			Status:          SaaSAlertNotificationStatusDead,
			Attempts:        3,
			MaxAttempts:     3,
			Alert: SaaSQuotaAlert{
				Status: SaaSQuotaStatus{
					TenantID: 12,
					Metric:   SaaSMetricUsers,
					Current:  8,
					Limit:    10,
				},
				AlertType: SaaSAlertTypeQuotaExceeded,
				PeriodKey: SaaSAlertPeriodLifetime,
				Message:   "子账号数接近上限",
			},
			LastError:   "webhook failed",
			NextRetryAt: "2026-07-09 16:50:00",
			CreatedAt:   "2026-07-09 16:40:00",
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	alertReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=alerts&tenantId=12&status=all&metric=users&alertType=quota_exceeded&limit=9999", nil)
	alertReq.Header.Set("X-Mochat-Go-User-ID", "1")
	alertRec := httptest.NewRecorder()
	handler.ExportCSV(alertRec, alertReq)
	if alertRec.Code != http.StatusOK {
		t.Fatalf("alert status = %d body=%s", alertRec.Code, alertRec.Body.String())
	}
	if !strings.Contains(alertRec.Header().Get("Content-Disposition"), "mochat-saas-alerts") {
		t.Fatalf("alert headers = %+v", alertRec.Header())
	}
	if store.lastAlertOptions.TenantID != 12 ||
		store.lastAlertOptions.Status != "" ||
		store.lastAlertOptions.Metric != SaaSMetricUsers ||
		store.lastAlertOptions.AlertType != SaaSAlertTypeQuotaExceeded ||
		store.lastAlertOptions.Page != 1 ||
		store.lastAlertOptions.PerPage != saasAdminExportMaxLimit {
		t.Fatalf("alert options = %+v", store.lastAlertOptions)
	}
	alertRecords := readSaaSAdminCSV(t, alertRec)
	if len(alertRecords) != 2 || alertRecords[0][0] != "id" || alertRecords[0][15] != "context" {
		t.Fatalf("alert records = %+v", alertRecords)
	}
	alertRow := alertRecords[1]
	if alertRow[0] != "41" || alertRow[2] != "12" || alertRow[6] != SaaSMetricUsers || alertRow[7] != "子账号数" || alertRow[15] != `{"source":"smoke"}` {
		t.Fatalf("alert row = %+v", alertRow)
	}

	notificationReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=notifications&tenantId=12&status=dead&channel=webhook&keyword=users&limit=9999", nil)
	notificationReq.Header.Set("X-Mochat-Go-User-ID", "1")
	notificationRec := httptest.NewRecorder()
	handler.ExportCSV(notificationRec, notificationReq)
	if notificationRec.Code != http.StatusOK {
		t.Fatalf("notification status = %d body=%s", notificationRec.Code, notificationRec.Body.String())
	}
	if !strings.Contains(notificationRec.Header().Get("Content-Disposition"), "mochat-saas-notifications") {
		t.Fatalf("notification headers = %+v", notificationRec.Header())
	}
	if store.lastNotificationOptions.TenantID != 12 ||
		store.lastNotificationOptions.Status != SaaSAlertNotificationStatusDead ||
		store.lastNotificationOptions.Channel != SaaSAlertNotificationChannelWebhook ||
		store.lastNotificationOptions.Keyword != "users" ||
		store.lastNotificationOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("notification options = %+v", store.lastNotificationOptions)
	}
	notificationRecords := readSaaSAdminCSV(t, notificationRec)
	if len(notificationRecords) != 2 || notificationRecords[0][0] != "id" || notificationRecords[0][15] != "lastError" {
		t.Fatalf("notification records = %+v", notificationRecords)
	}
	notificationRow := notificationRecords[1]
	if notificationRow[0] != "77" || notificationRow[3] != "12" || notificationRow[5] != SaaSAlertNotificationStatusDead || notificationRow[8] != SaaSMetricUsers || notificationRow[9] != "子账号数" || notificationRow[15] != "webhook failed" {
		t.Fatalf("notification row = %+v", notificationRow)
	}
}

func TestSaaSAdminExportCSVOperationsAndBilling(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		operationLogs: []SaaSAdminOperationLog{{
			ID:            91,
			TenantID:      12,
			ActorUserID:   1,
			ActorTenantID: 1,
			Action:        "tenant.package",
			TargetType:    "tenant",
			TargetID:      "12",
			TargetName:    "租户B",
			BeforeJSON:    `{"packageCode":"growth"}`,
			AfterJSON:     `{"packageCode":"scale"}`,
			Remark:        "升级套餐",
			CreatedAt:     "2026-07-09 17:00:00",
		}},
		billingEvents: []SaaSAdminBillingEvent{{
			ID:                92,
			TenantID:          12,
			EventType:         "renewal",
			PackageCode:       "scale",
			PackageName:       "规模版",
			PreviousExpiresAt: "2027-01-01 00:00:00",
			NewExpiresAt:      "2028-01-01 00:00:00",
			AmountCents:       1280000,
			Currency:          "CNY",
			PaidAt:            "2026-07-09 18:00:00",
			PaymentMethod:     "bank",
			ExternalOrderNo:   "ORDER-2",
			ActorUserID:       1,
			ActorTenantID:     1,
			Remark:            "续费",
			MetadataJSON:      `{"source":"smoke"}`,
			CreatedAt:         "2026-07-09 18:01:00",
		}},
		billingReconciliationItems: []SaaSAdminBillingReconciliationItem{{
			BillingEvent: SaaSAdminBillingEvent{
				ID:                93,
				TenantID:          12,
				EventType:         "renewal",
				PackageCode:       "growth",
				PackageName:       "增长版",
				PreviousExpiresAt: "2027-01-01 00:00:00",
				NewExpiresAt:      "2028-01-01 00:00:00",
				AmountCents:       880000,
				Currency:          "CNY",
				PaidAt:            "2026-07-09 18:30:00",
				PaymentMethod:     "manual",
				ExternalOrderNo:   "DRIFT-93",
				ActorUserID:       1,
				ActorTenantID:     1,
				Remark:            "对账异常",
				MetadataJSON:      `{"source":"smoke"}`,
				CreatedAt:         "2026-07-09 18:31:00",
			},
			TenantName:           "租户B",
			CurrentPackageFound:  true,
			CurrentPackageCode:   "scale",
			CurrentPackageName:   "规模版",
			CurrentExpiresAt:     "2027-12-31 00:00:00",
			CurrentPackageStatus: 1,
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	operationReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=operation_logs&tenantId=12&limit=9999&action=tenant.package&targetType=tenant&keyword=scale", nil)
	operationReq.Header.Set("X-Mochat-Go-User-ID", "1")
	operationRec := httptest.NewRecorder()
	handler.ExportCSV(operationRec, operationReq)
	if operationRec.Code != http.StatusOK {
		t.Fatalf("operation status = %d body=%s", operationRec.Code, operationRec.Body.String())
	}
	if store.lastOperationOptions.TenantID != 12 ||
		store.lastOperationOptions.Limit != 5000 ||
		store.lastOperationOptions.Action != "tenant.package" ||
		store.lastOperationOptions.TargetType != "tenant" ||
		store.lastOperationOptions.Keyword != "scale" {
		t.Fatalf("operation options = %+v", store.lastOperationOptions)
	}
	operationRecords := readSaaSAdminCSV(t, operationRec)
	if len(operationRecords) != 2 || operationRecords[0][0] != "id" || operationRecords[1][2] != "tenant.package" || operationRecords[1][11] != `{"packageCode":"scale"}` {
		t.Fatalf("operation records = %+v", operationRecords)
	}

	billingReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=billing&tenantId=12&eventType=renewal&packageCode=scale&keyword=ORDER-2", nil)
	billingReq.Header.Set("X-Mochat-Go-User-ID", "1")
	billingRec := httptest.NewRecorder()
	handler.ExportCSV(billingRec, billingReq)
	if billingRec.Code != http.StatusOK {
		t.Fatalf("billing status = %d body=%s", billingRec.Code, billingRec.Body.String())
	}
	if store.lastBillingOptions.TenantID != 12 ||
		store.lastBillingOptions.Limit != 1000 ||
		store.lastBillingOptions.EventType != "renewal" ||
		store.lastBillingOptions.PackageCode != "scale" ||
		store.lastBillingOptions.Keyword != "ORDER-2" {
		t.Fatalf("billing options = %+v", store.lastBillingOptions)
	}
	billingRecords := readSaaSAdminCSV(t, billingRec)
	if len(billingRecords) != 2 || billingRecords[0][0] != "id" || billingRecords[1][3] != "scale" || billingRecords[1][15] != `{"source":"smoke"}` {
		t.Fatalf("billing records = %+v", billingRecords)
	}

	reconciliationReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=billingReconciliation&tenantId=12&eventType=renewal&keyword=DRIFT&mismatchOnly=1", nil)
	reconciliationReq.Header.Set("X-Mochat-Go-User-ID", "1")
	reconciliationRec := httptest.NewRecorder()
	handler.ExportCSV(reconciliationRec, reconciliationReq)
	if reconciliationRec.Code != http.StatusOK {
		t.Fatalf("reconciliation status = %d body=%s", reconciliationRec.Code, reconciliationRec.Body.String())
	}
	if store.lastBillingReconcileOptions.TenantID != 12 ||
		store.lastBillingReconcileOptions.Limit != 1000 ||
		store.lastBillingReconcileOptions.EventType != "renewal" ||
		store.lastBillingReconcileOptions.Keyword != "DRIFT" ||
		!store.lastBillingReconcileOptions.MismatchOnly {
		t.Fatalf("reconciliation options = %+v", store.lastBillingReconcileOptions)
	}
	reconciliationRecords := readSaaSAdminCSV(t, reconciliationRec)
	if len(reconciliationRecords) != 2 ||
		reconciliationRecords[0][:5][0] != "id" ||
		reconciliationRecords[0][18] != "reconcileStatus" ||
		reconciliationRecords[1][2] != "租户B" ||
		reconciliationRecords[1][4] != "growth" ||
		reconciliationRecords[1][14] != "scale" ||
		reconciliationRecords[1][18] != "mismatch" ||
		!strings.Contains(reconciliationRecords[1][19], "package_mismatch") ||
		!strings.Contains(reconciliationRecords[1][19], "expires_mismatch") {
		t.Fatalf("reconciliation records = %+v", reconciliationRecords)
	}
}

func TestSaaSAdminExportCSVRejectsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=tenants", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.overviewCalls != 0 || store.operationCalls != 0 || store.billingCalls != 0 {
		t.Fatalf("calls overview=%d operation=%d billing=%d", store.overviewCalls, store.operationCalls, store.billingCalls)
	}
}

func TestSaaSAdminLogFiltersRejectTooLongValues(t *testing.T) {
	cases := []struct {
		name string
		path string
		call func(*SaaSAdminHandler, *httptest.ResponseRecorder, *http.Request)
	}{
		{
			name: "operations action",
			path: "/dashboard/saasAdmin/operations?action=" + strings.Repeat("a", 65),
			call: func(handler *SaaSAdminHandler, rec *httptest.ResponseRecorder, req *http.Request) {
				handler.OperationLogs(rec, req)
			},
		},
		{
			name: "billing package",
			path: "/dashboard/saasAdmin/billingEvents?packageCode=" + strings.Repeat("b", 65),
			call: func(handler *SaaSAdminHandler, rec *httptest.ResponseRecorder, req *http.Request) {
				handler.BillingEvents(rec, req)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeSaaSAdminStore{
				users: map[int]User{
					1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
				},
			}
			handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("X-Mochat-Go-User-ID", "1")
			rec := httptest.NewRecorder()

			tc.call(handler, rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			if store.operationCalls != 0 || store.billingCalls != 0 {
				t.Fatalf("calls operation=%d billing=%d", store.operationCalls, store.billingCalls)
			}
		})
	}
}

func TestSaaSAdminTenantDetailAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 1, UserCount: 8, OpenAlertCount: 1},
			Tenants: []SaaSAdminTenantOverview{{
				TenantID:        12,
				TenantName:      "租户B",
				TenantStatus:    1,
				PackageCode:     "growth",
				PackageName:     "增长版",
				OpenAlertCount:  1,
				MaxUsageMetric:  SaaSMetricUsers,
				MaxUsageCurrent: 8,
				MaxUsageLimit:   10,
				MaxUsageRatio:   0.8,
			}},
			Metrics: []SaaSAdminMetricOverview{{
				Metric:         SaaSMetricUsers,
				Current:        8,
				Limit:          10,
				UsageRatio:     0.8,
				OpenAlertCount: 1,
			}},
		},
		operationLogs: []SaaSAdminOperationLog{{
			ID:         91,
			TenantID:   12,
			Action:     "tenant.package",
			TargetType: "tenant",
			TargetID:   "12",
			TargetName: "租户B",
			AfterJSON:  `{"packageCode":"growth"}`,
			CreatedAt:  "2026-07-09 15:30:00",
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenant?tenantId=12&operationLimit=99&expiringDays=400", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TenantDetail(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.Scope != SaaSAdminScopeTenant || store.lastOptions.TenantID != 12 || store.lastOptions.Limit != 1 || store.lastOptions.ExpiringDays != 365 {
		t.Fatalf("overview options = %+v", store.lastOptions)
	}
	if store.lastOperationOptions.TenantID != 12 || store.lastOperationOptions.Limit != 50 {
		t.Fatalf("operation options = %+v", store.lastOperationOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["tenantId"].(float64) != 12 || data["canPlatformScope"] != true {
		t.Fatalf("data = %+v", data)
	}
	tenant := data["tenant"].(map[string]any)
	if tenant["tenantName"] != "租户B" || tenant["maxUsageLabel"] != "子账号数" {
		t.Fatalf("tenant = %+v", tenant)
	}
	operations := data["operations"].([]any)
	if len(operations) != 1 || operations[0].(map[string]any)["action"] != "tenant.package" {
		t.Fatalf("operations = %+v", operations)
	}
}

func TestSaaSAdminTenantDetailRestrictsTenantAdminToOwnTenant(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 1},
			Tenants: []SaaSAdminTenantOverview{{TenantID: 10, TenantName: "租户A"}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	crossReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenant?tenantId=12", nil)
	crossReq.Header.Set("X-Mochat-Go-User-ID", "7")
	crossRec := httptest.NewRecorder()
	handler.TenantDetail(crossRec, crossReq)
	if crossRec.Code != http.StatusForbidden {
		t.Fatalf("cross status = %d body=%s", crossRec.Code, crossRec.Body.String())
	}
	if store.overviewCalls != 0 || store.operationCalls != 0 {
		t.Fatalf("calls overview=%d operations=%d", store.overviewCalls, store.operationCalls)
	}

	ownReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenant", nil)
	ownReq.Header.Set("X-Mochat-Go-User-ID", "7")
	ownRec := httptest.NewRecorder()
	handler.TenantDetail(ownRec, ownReq)
	if ownRec.Code != http.StatusOK {
		t.Fatalf("own status = %d body=%s", ownRec.Code, ownRec.Body.String())
	}
	if store.lastOptions.TenantID != 10 || store.lastOperationOptions.TenantID != 10 {
		t.Fatalf("options overview=%+v operations=%+v", store.lastOptions, store.lastOperationOptions)
	}
	data := decodeSaaSAdminResponse(t, ownRec)
	if data["canPlatformScope"] != false {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminTenantDetailReturnsNotFoundWhenTenantMissing(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{Summary: SaaSAdminSummary{}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenant?tenantId=404", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TenantDetail(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.operationCalls != 0 {
		t.Fatalf("operation calls = %d", store.operationCalls)
	}
}

func TestSaaSAdminTenantLifecycleAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 1, UserCount: 8, OpenAlertCount: 1},
			Tenants: []SaaSAdminTenantOverview{{
				TenantID:       12,
				TenantName:     "租户B",
				TenantStatus:   1,
				PackageCode:    "growth",
				PackageName:    "增长版",
				OpenAlertCount: 1,
			}},
		},
		operationLogs: []SaaSAdminOperationLog{{
			ID:          91,
			TenantID:    12,
			ActorUserID: 1,
			Action:      "tenant.package",
			TargetType:  "tenant",
			TargetID:    "12",
			TargetName:  "租户B",
			Remark:      "升级套餐",
			CreatedAt:   "2026-07-09 15:30:00",
		}},
		billingEvents: []SaaSAdminBillingEvent{{
			ID:              92,
			TenantID:        12,
			EventType:       "renewal",
			PackageCode:     "growth",
			PackageName:     "增长版",
			AmountCents:     1280000,
			Currency:        "CNY",
			ExternalOrderNo: "ORDER-2",
			ActorUserID:     1,
			Remark:          "续费",
			CreatedAt:       "2026-07-09 16:00:00",
		}},
		tasks: []SaaSAdminTask{{
			ID:          93,
			TaskType:    SaaSAdminTaskTypeTenantRenewal,
			Status:      SaaSAdminTaskStatusApplied,
			TenantID:    12,
			PackageCode: "growth",
			ActorUserID: 1,
			Remark:      "续费任务",
			AppliedAt:   "2026-07-09 17:00:00",
			CreatedAt:   "2026-07-09 16:30:00",
		}},
		alertPage: SaaSAlertListPage{Items: []SaaSAlertRecord{{
			ID:           94,
			AlertKey:     "12:users:quota_exceeded:lifetime",
			TenantID:     12,
			AlertType:    SaaSAlertTypeQuotaExceeded,
			Severity:     SaaSAlertSeverityWarning,
			Status:       SaaSAlertStatusOpen,
			Metric:       SaaSMetricUsers,
			PeriodKey:    SaaSAlertPeriodLifetime,
			CurrentValue: 8,
			LimitValue:   10,
			Message:      "子账号数接近上限",
			LastSeenAt:   "2026-07-09 17:30:00",
		}}},
		notifications: []SaaSAlertNotification{{
			ID:              95,
			NotificationKey: "12:users:quota_exceeded:lifetime:webhook",
			AlertKey:        "12:users:quota_exceeded:lifetime",
			TenantID:        12,
			Channel:         SaaSAlertNotificationChannelWebhook,
			Status:          SaaSAlertNotificationStatusDead,
			Attempts:        3,
			MaxAttempts:     3,
			Alert: SaaSQuotaAlert{
				Status:    SaaSQuotaStatus{TenantID: 12, Metric: SaaSMetricUsers, Current: 8, Limit: 10},
				AlertType: SaaSAlertTypeQuotaExceeded,
				PeriodKey: SaaSAlertPeriodLifetime,
				Message:   "子账号数接近上限",
			},
			LastError: "webhook failed",
			UpdatedAt: "2026-07-09 18:00:00",
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenantLifecycle?tenantId=12&limit=3", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TenantLifecycle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.Scope != SaaSAdminScopeTenant || store.lastOptions.TenantID != 12 {
		t.Fatalf("overview options = %+v", store.lastOptions)
	}
	if store.lastOperationOptions.TenantID != 12 || store.lastOperationOptions.Limit != 3 {
		t.Fatalf("operation options = %+v", store.lastOperationOptions)
	}
	if store.lastBillingOptions.TenantID != 12 || store.lastBillingOptions.Limit != 3 {
		t.Fatalf("billing options = %+v", store.lastBillingOptions)
	}
	if store.lastTaskOptions.TenantID != 12 || store.lastTaskOptions.Limit != 3 {
		t.Fatalf("task options = %+v", store.lastTaskOptions)
	}
	if store.lastAlertOptions.TenantID != 12 || store.lastAlertOptions.PerPage != 3 {
		t.Fatalf("alert options = %+v", store.lastAlertOptions)
	}
	if store.lastNotificationOptions.TenantID != 12 || store.lastNotificationOptions.Limit != 3 {
		t.Fatalf("notification options = %+v", store.lastNotificationOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	summary := data["summary"].(map[string]any)
	if summary["operationCount"].(float64) != 1 ||
		summary["billingEventCount"].(float64) != 1 ||
		summary["taskCount"].(float64) != 1 ||
		summary["alertCount"].(float64) != 1 ||
		summary["notificationCount"].(float64) != 1 ||
		summary["timelineCount"].(float64) != 5 ||
		summary["rawTimelineCount"].(float64) != 5 ||
		summary["returnedEventCount"].(float64) != 3 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary["filterActive"].(bool) {
		t.Fatalf("filterActive = %+v", summary)
	}
	timeline := data["timeline"].([]any)
	if len(timeline) != 3 {
		t.Fatalf("timeline = %+v", timeline)
	}
	first := timeline[0].(map[string]any)
	if first["source"] != "notification" || first["status"] != SaaSAlertNotificationStatusDead || first["referenceId"] != "95" {
		t.Fatalf("first event = %+v", first)
	}
	if len(data["operations"].([]any)) != 1 || len(data["billingEvents"].([]any)) != 1 || len(data["tasks"].([]any)) != 1 || len(data["alerts"].([]any)) != 1 || len(data["notifications"].([]any)) != 1 {
		t.Fatalf("data sections = %+v", data)
	}
}

func TestSaaSAdminTenantLifecycleFiltersTimeline(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Tenants: []SaaSAdminTenantOverview{{
				TenantID:     12,
				TenantName:   "租户B",
				TenantStatus: 1,
				PackageCode:  "growth",
				PackageName:  "增长版",
			}},
		},
		operationLogs: []SaaSAdminOperationLog{{
			ID:         91,
			TenantID:   12,
			Action:     "tenant.package",
			TargetType: "tenant",
			TargetID:   "12",
			TargetName: "租户B",
			Remark:     "升级套餐",
			CreatedAt:  "2026-07-09 15:30:00",
		}},
		notifications: []SaaSAlertNotification{{
			ID:              95,
			NotificationKey: "12:tenant_renewal:tenant_renewal_reminder:renewal_20260808:webhook",
			AlertKey:        "12:tenant_renewal:tenant_renewal_reminder:renewal_20260808",
			TenantID:        12,
			Channel:         SaaSAlertNotificationChannelWebhook,
			Status:          SaaSAlertNotificationStatusPending,
			MaxAttempts:     4,
			Alert: SaaSQuotaAlert{
				Status:    SaaSQuotaStatus{TenantID: 12, Metric: SaaSEventMetricTenantRenewal},
				AlertType: SaaSAlertTypeTenantRenewal,
				PeriodKey: "renewal_20260808",
				Message:   "租户B 即将到期，请跟进续费提醒",
			},
			UpdatedAt: "2026-07-09 18:00:00",
		}, {
			ID:              96,
			NotificationKey: "12:users:quota_exceeded:lifetime:webhook",
			AlertKey:        "12:users:quota_exceeded:lifetime",
			TenantID:        12,
			Channel:         SaaSAlertNotificationChannelWebhook,
			Status:          SaaSAlertNotificationStatusDead,
			MaxAttempts:     3,
			Alert: SaaSQuotaAlert{
				Status:    SaaSQuotaStatus{TenantID: 12, Metric: SaaSMetricUsers},
				AlertType: SaaSAlertTypeQuotaExceeded,
				PeriodKey: SaaSAlertPeriodLifetime,
				Message:   "子账号数接近上限",
			},
			LastError: "webhook failed",
			UpdatedAt: "2026-07-09 17:00:00",
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenantLifecycle?tenantId=12&source=notification&status=pending&eventType=tenant_renewal_reminder&keyword=续费提醒&limit=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TenantLifecycle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	filters := data["filters"].(map[string]any)
	if filters["source"] != "notification" || filters["status"] != "pending" || filters["eventType"] != "tenant_renewal_reminder" || filters["keyword"] != "续费提醒" {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["timelineCount"].(float64) != 1 || summary["rawTimelineCount"].(float64) != 3 || summary["returnedEventCount"].(float64) != 1 || !summary["filterActive"].(bool) {
		t.Fatalf("summary = %+v", summary)
	}
	timeline := data["timeline"].([]any)
	if len(timeline) != 1 {
		t.Fatalf("timeline = %+v", timeline)
	}
	event := timeline[0].(map[string]any)
	if event["source"] != "notification" || event["eventType"] != SaaSAlertTypeTenantRenewal || event["status"] != SaaSAlertNotificationStatusPending || event["referenceId"] != "95" {
		t.Fatalf("event = %+v", event)
	}
}

func TestSaaSAdminTenantLifecycleFiltersOperationQueueAssignmentStatus(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Tenants: []SaaSAdminTenantOverview{{
				TenantID:     12,
				TenantName:   "租户B",
				TenantStatus: 1,
				PackageCode:  "growth",
				PackageName:  "增长版",
			}},
		},
		operationLogs: []SaaSAdminOperationLog{{
			ID:         91,
			TenantID:   12,
			Action:     "tenant.package",
			TargetType: "tenant",
			TargetID:   "12",
			TargetName: "租户B",
			Remark:     "升级套餐",
			CreatedAt:  "2026-07-09 15:30:00",
		}, {
			ID:          101,
			TenantID:    12,
			ActorUserID: 1,
			Action:      SaaSAdminOperationActionOperationQueueAssign,
			TargetType:  SaaSAdminOperationTargetAdminTask,
			TargetID:    "701",
			TargetName:  "任务 SLA",
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"source":         SaaSAdminOperationQueueSourceTaskSLA,
				"objectType":     SaaSAdminOperationTargetAdminTask,
				"objectId":       "701",
				"targetName":     "任务 SLA",
				"owner":          "ops-task-sla",
				"status":         "contacted",
				"nextFollowUpAt": "2026-07-25 11:00:00",
				"remark":         "生命周期回看任务 SLA 认领",
			}),
			Remark:    "生命周期回看任务 SLA 认领",
			CreatedAt: "2026-07-09 18:30:00",
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenantLifecycle?tenantId=12&source=operation&status=contacted&eventType=operation_queue.assign&keyword=ops-task-sla&limit=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TenantLifecycle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	filters := data["filters"].(map[string]any)
	if filters["source"] != "operation" || filters["status"] != "contacted" || filters["eventType"] != "operation_queue.assign" || filters["keyword"] != "ops-task-sla" {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["timelineCount"].(float64) != 1 || summary["rawTimelineCount"].(float64) != 2 || summary["returnedEventCount"].(float64) != 1 || !summary["filterActive"].(bool) {
		t.Fatalf("summary = %+v", summary)
	}
	timeline := data["timeline"].([]any)
	event := timeline[0].(map[string]any)
	if event["source"] != "operation" || event["eventType"] != SaaSAdminOperationActionOperationQueueAssign || event["status"] != "contacted" || event["referenceId"] != "101" {
		t.Fatalf("event = %+v", event)
	}
	payload := event["payload"].(map[string]any)
	after := payload["after"].(map[string]any)
	if after["owner"] != "ops-task-sla" || after["source"] != SaaSAdminOperationQueueSourceTaskSLA || after["status"] != "contacted" {
		t.Fatalf("after = %+v", after)
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=tenantLifecycle&tenantId=12&source=operation&status=contacted&eventType=operation_queue.assign&keyword=ops-task-sla&limit=1000", nil)
	exportReq.Header.Set("X-Mochat-Go-User-ID", "1")
	exportRec := httptest.NewRecorder()
	handler.ExportCSV(exportRec, exportReq)
	if exportRec.Code != http.StatusOK {
		t.Fatalf("export status = %d body=%s", exportRec.Code, exportRec.Body.String())
	}
	records := readSaaSAdminCSV(t, exportRec)
	if len(records) != 2 || records[0][7] != "status" {
		t.Fatalf("records = %+v", records)
	}
	row := records[1]
	if row[4] != "operation" || row[5] != SaaSAdminOperationActionOperationQueueAssign || row[7] != "contacted" || row[11] != "生命周期回看任务 SLA 认领" || !strings.Contains(row[12], "ops-task-sla") {
		t.Fatalf("row = %+v", row)
	}
}

func TestSaaSAdminTenantLifecycleRequiresPlatformAdminAndTenantID(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	missingReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenantLifecycle", nil)
	missingReq.Header.Set("X-Mochat-Go-User-ID", "1")
	missingRec := httptest.NewRecorder()
	handler.TenantLifecycle(missingRec, missingReq)
	if missingRec.Code != http.StatusBadRequest {
		t.Fatalf("missing status = %d body=%s", missingRec.Code, missingRec.Body.String())
	}

	invalidSourceReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenantLifecycle?tenantId=10&source=unknown", nil)
	invalidSourceReq.Header.Set("X-Mochat-Go-User-ID", "1")
	invalidSourceRec := httptest.NewRecorder()
	handler.TenantLifecycle(invalidSourceRec, invalidSourceReq)
	if invalidSourceRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid source status = %d body=%s", invalidSourceRec.Code, invalidSourceRec.Body.String())
	}

	tenantReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenantLifecycle?tenantId=10", nil)
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.TenantLifecycle(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
	if store.overviewCalls != 0 || store.operationCalls != 0 || store.billingCalls != 0 || store.taskCalls != 0 || store.alertCalls != 0 || store.notificationCalls != 0 {
		t.Fatalf("calls overview=%d operation=%d billing=%d task=%d alert=%d notification=%d", store.overviewCalls, store.operationCalls, store.billingCalls, store.taskCalls, store.alertCalls, store.notificationCalls)
	}
}

func TestSaaSAdminUsageAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Tenants: []SaaSAdminTenantOverview{{
				TenantID:      12,
				TenantName:    "租户B",
				PackageCode:   "scale",
				PackageName:   "规模版",
				ExpiresAt:     "2028-01-01 00:00:00",
				PackageStatus: 1,
			}},
		},
		usageMetrics: []SaaSAdminUsageMetric{
			{
				Metric:         SaaSMetricContacts,
				PeriodKey:      SaaSAlertPeriodLifetime,
				Current:        12,
				Limit:          10,
				Remaining:      0,
				UsageRatio:     1.2,
				Status:         "exceeded",
				OpenAlertCount: 1,
				UpdatedBy:      "maintenance",
				UpdatedAt:      "2026-07-09 16:20:00",
			},
			{
				Metric:     SaaSMetricUsers,
				PeriodKey:  SaaSAlertPeriodLifetime,
				Current:    8,
				Limit:      10,
				Remaining:  2,
				UsageRatio: 0.8,
				Status:     "warning",
				UpdatedBy:  "runtime",
				UpdatedAt:  "2026-07-09 16:21:00",
			},
			{
				Metric:    SaaSMetricStorage,
				PeriodKey: SaaSAlertPeriodLifetime,
				Current:   99,
				Limit:     0,
				Unlimited: true,
				Status:    "unlimited",
				UpdatedBy: "maintenance",
				UpdatedAt: "2026-07-09 16:22:00",
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/usage?tenantId=12&expiringDays=400", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Usage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.TenantID != 12 || store.lastOptions.Limit != 1 || store.lastOptions.ExpiringDays != 365 {
		t.Fatalf("overview options = %+v", store.lastOptions)
	}
	if store.lastUsageTenantID != 12 || store.usageCalls != 1 {
		t.Fatalf("usage tenant=%d calls=%d", store.lastUsageTenantID, store.usageCalls)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["tenantId"].(float64) != 12 || data["canPlatformScope"] != true {
		t.Fatalf("data = %+v", data)
	}
	summary := data["summary"].(map[string]any)
	if summary["metricCount"].(float64) != 3 ||
		summary["limitedMetricCount"].(float64) != 2 ||
		summary["unlimitedMetricCount"].(float64) != 1 ||
		summary["openAlertMetricCount"].(float64) != 1 ||
		summary["exceededMetricCount"].(float64) != 1 ||
		summary["warningMetricCount"].(float64) != 1 ||
		summary["highestUsageMetric"] != SaaSMetricContacts {
		t.Fatalf("summary = %+v", summary)
	}
	metrics := data["usageMetrics"].([]any)
	if len(metrics) != 3 {
		t.Fatalf("metrics = %+v", metrics)
	}
	first := metrics[0].(map[string]any)
	if first["metric"] != SaaSMetricContacts ||
		first["label"] != "客户数" ||
		first["status"] != "exceeded" ||
		first["remaining"].(float64) != 0 ||
		first["openAlertCount"].(float64) != 1 ||
		first["updatedBy"] != "maintenance" {
		t.Fatalf("first metric = %+v", first)
	}
}

func TestSaaSAdminUsageRestrictsTenantAdminToOwnTenant(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Tenants: []SaaSAdminTenantOverview{{TenantID: 10, TenantName: "租户A"}},
		},
		usageMetrics: []SaaSAdminUsageMetric{{
			Metric:    SaaSMetricUsers,
			PeriodKey: SaaSAlertPeriodLifetime,
			Current:   2,
			Limit:     10,
			Remaining: 8,
			Status:    "normal",
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	crossReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/usage?tenantId=12", nil)
	crossReq.Header.Set("X-Mochat-Go-User-ID", "7")
	crossRec := httptest.NewRecorder()
	handler.Usage(crossRec, crossReq)
	if crossRec.Code != http.StatusForbidden {
		t.Fatalf("cross status = %d body=%s", crossRec.Code, crossRec.Body.String())
	}
	if store.overviewCalls != 0 || store.usageCalls != 0 {
		t.Fatalf("calls overview=%d usage=%d", store.overviewCalls, store.usageCalls)
	}

	ownReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/usage", nil)
	ownReq.Header.Set("X-Mochat-Go-User-ID", "7")
	ownRec := httptest.NewRecorder()
	handler.Usage(ownRec, ownReq)
	if ownRec.Code != http.StatusOK {
		t.Fatalf("own status = %d body=%s", ownRec.Code, ownRec.Body.String())
	}
	if store.lastOptions.TenantID != 10 || store.lastUsageTenantID != 10 {
		t.Fatalf("options overview=%+v usageTenant=%d", store.lastOptions, store.lastUsageTenantID)
	}
	data := decodeSaaSAdminResponse(t, ownRec)
	if data["canPlatformScope"] != false {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminRiskAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 3},
			Tenants: []SaaSAdminTenantOverview{
				{
					TenantID:        12,
					TenantName:      "高风险租户",
					TenantStatus:    1,
					PackageCode:     "growth",
					PackageName:     "成长版",
					ExpiresAt:       "2026-07-15 00:00:00",
					ExpiringSoon:    true,
					OpenAlertCount:  2,
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 85,
					MaxUsageLimit:   100,
					MaxUsageRatio:   0.85,
				},
				{
					TenantID:     13,
					TenantName:   "停用租户",
					TenantStatus: 2,
				},
				{
					TenantID:        14,
					TenantName:      "正常租户",
					TenantStatus:    1,
					PackageCode:     "growth",
					PackageName:     "成长版",
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 1,
					MaxUsageLimit:   100,
					MaxUsageRatio:   0.01,
				},
			},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {
				{
					Metric:         SaaSMetricContacts,
					PeriodKey:      SaaSAlertPeriodLifetime,
					Current:        11,
					Limit:          10,
					UsageRatio:     1.1,
					Status:         "exceeded",
					OpenAlertCount: 1,
				},
				{
					Metric:     SaaSMetricUsers,
					PeriodKey:  SaaSAlertPeriodLifetime,
					Current:    85,
					Limit:      100,
					UsageRatio: 0.85,
					Status:     "warning",
				},
			},
			13: {},
			14: {{
				Metric:     SaaSMetricUsers,
				PeriodKey:  SaaSAlertPeriodLifetime,
				Current:    1,
				Limit:      100,
				UsageRatio: 0.01,
				Status:     "normal",
			}},
		},
		latestRiskFollowUps: map[int]SaaSAdminRiskFollowUpSnapshot{
			12: {
				TenantID:       12,
				TenantName:     "高风险租户",
				Status:         SaaSAdminRiskFollowUpStatusRenewalPending,
				Owner:          "CSM-A",
				NextFollowUpAt: "2026-07-01 00:00:00",
				Remark:         "续费推进中",
				OperationID:    1201,
				CreatedAt:      "2026-06-30 12:00:00",
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/risk?scope=platform&keyword=%E9%A3%8E%E9%99%A9&tenantStatus=1&packageCode=growth&dueState=expiring&limit=50&expiringDays=10&highUsageRatio=0.75", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Risk(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.Scope != SaaSAdminScopePlatform ||
		store.lastOptions.Keyword != "风险" ||
		store.lastOptions.TenantStatus != 1 ||
		store.lastOptions.PackageCode != "growth" ||
		store.lastOptions.DueState != SaaSAdminDueStateExpiring ||
		store.lastOptions.Limit != 50 ||
		store.lastOptions.ExpiringDays != 10 {
		t.Fatalf("overview options = %+v", store.lastOptions)
	}
	if store.usageCalls != 3 {
		t.Fatalf("usage calls = %d", store.usageCalls)
	}
	if store.latestRiskFollowUpCalls != 1 {
		t.Fatalf("risk follow-up calls = %d", store.latestRiskFollowUpCalls)
	}
	if got := store.lastRiskFollowUpTenantIDs; len(got) != 3 || !containsInt(got, 12) || !containsInt(got, 13) || !containsInt(got, 14) {
		t.Fatalf("risk follow-up tenant ids = %+v", got)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["scope"] != SaaSAdminScopePlatform || data["canPlatformScope"] != true {
		t.Fatalf("data = %+v", data)
	}
	filters := data["filters"].(map[string]any)
	if filters["highUsageRatio"].(float64) != 0.75 {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["totalTenantCount"].(float64) != 3 ||
		summary["evaluatedTenantCount"].(float64) != 3 ||
		summary["riskTenantCount"].(float64) != 2 ||
		summary["criticalRiskTenantCount"].(float64) != 2 ||
		summary["disabledTenantCount"].(float64) != 1 ||
		summary["noPackageTenantCount"].(float64) != 1 ||
		summary["openAlertTenantCount"].(float64) != 1 ||
		summary["highUsageTenantCount"].(float64) != 1 ||
		summary["exceededUsageTenantCount"].(float64) != 1 ||
		summary["followUpTenantCount"].(float64) != 1 ||
		summary["pendingFollowUpCount"].(float64) != 1 ||
		summary["renewalPendingCount"].(float64) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	items := data["riskTenants"].([]any)
	if len(items) != 3 {
		t.Fatalf("items = %+v", items)
	}
	var tenant12 map[string]any
	for _, raw := range items {
		item := raw.(map[string]any)
		if item["tenantId"].(float64) == 12 {
			tenant12 = item
			break
		}
	}
	if tenant12 == nil {
		t.Fatalf("tenant 12 not found: %+v", items)
	}
	if tenant12["riskLevel"] != "critical" ||
		tenant12["riskScore"].(float64) < 90 ||
		tenant12["suggestedAction"] != "处理打开告警，评估扩容或清理用量" {
		t.Fatalf("tenant 12 = %+v", tenant12)
	}
	followUp := tenant12["followUp"].(map[string]any)
	if followUp["status"] != SaaSAdminRiskFollowUpStatusRenewalPending ||
		followUp["owner"] != "CSM-A" ||
		followUp["nextFollowUpAt"] != "2026-07-01 00:00:00" ||
		followUp["operationId"].(float64) != 1201 {
		t.Fatalf("followUp = %+v", followUp)
	}
	topMetrics := tenant12["topUsageMetrics"].([]any)
	if len(topMetrics) == 0 || topMetrics[0].(map[string]any)["metric"] != SaaSMetricContacts {
		t.Fatalf("top metrics = %+v", topMetrics)
	}
}

func TestSaaSAdminCustomerSuccessAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 3},
			Tenants: []SaaSAdminTenantOverview{
				{
					TenantID:        1,
					TenantName:      "平台租户",
					TenantStatus:    1,
					PackageCode:     "platform",
					PackageName:     "平台版",
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 1,
					MaxUsageLimit:   100,
					MaxUsageRatio:   0.01,
				},
				{
					TenantID:        12,
					TenantName:      "高风险租户",
					TenantStatus:    1,
					PackageCode:     "growth",
					PackageName:     "成长版",
					ExpiresAt:       "2026-07-15 00:00:00",
					ExpiringSoon:    true,
					OpenAlertCount:  2,
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 12,
					MaxUsageLimit:   10,
					MaxUsageRatio:   1.2,
				},
				{
					TenantID:        13,
					TenantName:      "正常租户",
					TenantStatus:    1,
					PackageCode:     "growth",
					PackageName:     "成长版",
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 1,
					MaxUsageLimit:   10,
					MaxUsageRatio:   0.1,
				},
			},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{
				Metric:         SaaSMetricUsers,
				PeriodKey:      SaaSAlertPeriodLifetime,
				Current:        12,
				Limit:          10,
				UsageRatio:     1.2,
				Status:         "exceeded",
				OpenAlertCount: 2,
			}},
			13: {{
				Metric:     SaaSMetricUsers,
				PeriodKey:  SaaSAlertPeriodLifetime,
				Current:    1,
				Limit:      10,
				UsageRatio: 0.1,
				Status:     "normal",
			}},
		},
		latestRiskFollowUps: map[int]SaaSAdminRiskFollowUpSnapshot{
			12: {
				TenantID:       12,
				TenantName:     "高风险租户",
				Status:         SaaSAdminRiskFollowUpStatusPending,
				Owner:          "CSM-A",
				NextFollowUpAt: "2000-01-01 00:00:00",
				Remark:         "需要续费沟通",
				OperationID:    1201,
				CreatedAt:      "2026-07-01 12:00:00",
			},
		},
		riskFollowUpSnapshots: []SaaSAdminRiskFollowUpSnapshot{{
			TenantID:       12,
			TenantName:     "高风险租户",
			Status:         SaaSAdminRiskFollowUpStatusPending,
			Owner:          "CSM-A",
			NextFollowUpAt: "2000-01-01 00:00:00",
			Remark:         "需要续费沟通",
			OperationID:    1201,
			CreatedAt:      "2026-07-01 12:00:00",
		}},
		billingFollowUpSnapshots: []SaaSAdminBillingReconciliationFollowUpSnapshot{{
			OperationID:    2201,
			BillingEventID: 88,
			TenantID:       12,
			TenantName:     "高风险租户",
			Status:         SaaSAdminRiskFollowUpStatusPending,
			Owner:          "Finance-A",
			NextFollowUpAt: "2000-01-01 00:00:00",
			Remark:         "账单异常待核对",
			PackageCode:    "growth",
			PackageName:    "成长版",
			CreatedAt:      "2026-07-01 13:00:00",
		}},
		tasks: []SaaSAdminTask{{
			ID:          3301,
			TaskType:    SaaSAdminTaskTypeTenantRenewal,
			Status:      SaaSAdminTaskStatusBlocked,
			TenantID:    12,
			PackageCode: "growth",
			ActorUserID: 1,
			Remark:      "续费任务阻断",
		}},
		notifications: []SaaSAlertNotification{
			{ID: 4401, NotificationKey: "failed", TenantID: 12, Status: SaaSAlertNotificationStatusFailed},
			{ID: 4402, NotificationKey: "dead", TenantID: 12, Status: SaaSAlertNotificationStatusDead},
			{ID: 4403, NotificationKey: "ok", TenantID: 13, Status: SaaSAlertNotificationStatusDelivered},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/customerSuccess?limit=5&tenantLimit=10&expiringDays=10&highUsageRatio=0.8&owner=CSM&priority=critical", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.CustomerSuccess(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.Scope != SaaSAdminScopePlatform ||
		store.lastOptions.ExcludedTenantID != 1 ||
		store.lastOptions.Limit != 10 ||
		store.lastOptions.ExpiringDays != 10 ||
		store.lastOptions.DueState != SaaSAdminDueStateAll {
		t.Fatalf("overview options = %+v", store.lastOptions)
	}
	if store.usageCalls != 2 ||
		store.latestRiskFollowUpCalls != 1 ||
		store.riskFollowUpSnapshotCalls != 1 ||
		store.billingFollowUpCalls != 1 ||
		store.taskCalls != 1 ||
		store.notificationCalls != 1 {
		t.Fatalf("calls usage=%d latest=%d riskTasks=%d billing=%d tasks=%d notifications=%d", store.usageCalls, store.latestRiskFollowUpCalls, store.riskFollowUpSnapshotCalls, store.billingFollowUpCalls, store.taskCalls, store.notificationCalls)
	}
	if store.lastRiskFollowUpTaskOptions.ExcludedTenantID != 1 || store.lastRiskFollowUpTaskOptions.Limit != saasAdminExportMaxLimit ||
		store.lastBillingFollowUpOptions.ExcludedTenantID != 1 || store.lastBillingFollowUpOptions.Limit != saasAdminExportMaxLimit ||
		store.lastTaskOptions.ExcludedTenantID != 1 || store.lastTaskOptions.Limit != saasAdminExportMaxLimit ||
		store.lastNotificationOptions.ExcludedTenantID != 1 || store.lastNotificationOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("limits risk=%+v billing=%+v tasks=%+v notifications=%+v", store.lastRiskFollowUpTaskOptions, store.lastBillingFollowUpOptions, store.lastTaskOptions, store.lastNotificationOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	filters := data["filters"].(map[string]any)
	if filters["owner"] != "CSM" || filters["priority"] != SaaSAdminCustomerSuccessPriorityCritical || filters["tenantLimit"].(float64) != 10 || filters["limit"].(float64) != 5 {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["queueCount"].(float64) != 1 ||
		summary["returnedCount"].(float64) != 1 ||
		summary["criticalCount"].(float64) != 1 ||
		summary["overdueCount"].(float64) != 1 ||
		summary["billingFollowUpCount"].(float64) != 1 ||
		summary["actionableTaskCount"].(float64) != 1 ||
		summary["retryableNotificationCount"].(float64) != 2 {
		t.Fatalf("summary = %+v", summary)
	}
	items := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %+v", items)
	}
	item := items[0].(map[string]any)
	if item["tenantId"].(float64) != 12 ||
		item["priority"] != SaaSAdminCustomerSuccessPriorityCritical ||
		item["owner"] != "CSM-A" ||
		item["dueState"] != SaaSAdminRiskFollowUpDueStateOverdue ||
		item["nextAction"] != "立即跟进逾期风险任务" ||
		item["billingFollowUpCount"].(float64) != 1 ||
		item["retryableNotificationCount"].(float64) != 2 ||
		item["healthScore"].(float64) <= item["riskScore"].(float64) {
		t.Fatalf("item = %+v", item)
	}
	if item["riskFollowUp"] == nil || len(item["billingFollowUps"].([]any)) != 1 {
		t.Fatalf("follow up payload = %+v", item)
	}
	taskSummary := item["adminTaskSummary"].(map[string]any)
	if taskSummary["blockedCount"].(float64) != 1 || taskSummary["actionableCount"].(float64) != 1 {
		t.Fatalf("task summary = %+v", taskSummary)
	}
}

func newFakeSaaSAdminOperationQueueStore(t *testing.T) *fakeSaaSAdminStore {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	format := func(v time.Time) string {
		return v.Format("2006-01-02 15:04:05")
	}
	return &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			2: {ID: 2, TenantID: 12, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 2},
			Tenants: []SaaSAdminTenantOverview{
				{
					TenantID:        1,
					TenantName:      "平台租户",
					TenantStatus:    1,
					PackageCode:     "platform",
					PackageName:     "平台版",
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 1,
					MaxUsageLimit:   100,
					MaxUsageRatio:   0.01,
				},
				{
					TenantID:        12,
					TenantName:      "运营待办租户",
					TenantStatus:    1,
					PackageCode:     "growth",
					PackageName:     "成长版",
					ExpiresAt:       format(now.Add(72 * time.Hour)),
					ExpiringSoon:    true,
					OpenAlertCount:  2,
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 18,
					MaxUsageLimit:   10,
					MaxUsageRatio:   1.8,
				},
			},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{
				Metric:         SaaSMetricUsers,
				PeriodKey:      SaaSAlertPeriodLifetime,
				Current:        18,
				Limit:          10,
				UsageRatio:     1.8,
				Status:         "exceeded",
				OpenAlertCount: 2,
			}},
		},
		latestRiskFollowUps: map[int]SaaSAdminRiskFollowUpSnapshot{
			12: {
				TenantID:       12,
				TenantName:     "运营待办租户",
				Status:         SaaSAdminRiskFollowUpStatusPending,
				Owner:          "CSM-A",
				NextFollowUpAt: format(now.Add(-48 * time.Hour)),
				Remark:         "客户成功待跟进",
				OperationID:    1201,
				CreatedAt:      format(now.Add(-72 * time.Hour)),
			},
		},
		riskFollowUpSnapshots: []SaaSAdminRiskFollowUpSnapshot{{
			TenantID:       12,
			TenantName:     "运营待办租户",
			Status:         SaaSAdminRiskFollowUpStatusPending,
			Owner:          "CSM-A",
			NextFollowUpAt: format(now.Add(-48 * time.Hour)),
			Remark:         "客户成功待跟进",
			OperationID:    1201,
			CreatedAt:      format(now.Add(-72 * time.Hour)),
		}},
		billingFollowUpSnapshots: []SaaSAdminBillingReconciliationFollowUpSnapshot{{
			OperationID:     2201,
			BillingEventID:  88,
			TenantID:        12,
			TenantName:      "运营待办租户",
			Status:          SaaSAdminRiskFollowUpStatusPending,
			Owner:           "Finance-A",
			NextFollowUpAt:  format(now.Add(-24 * time.Hour)),
			Remark:          "账单异常待核对",
			PackageCode:     "growth",
			PackageName:     "成长版",
			ExternalOrderNo: "ORDER-OPQ",
			CreatedAt:       format(now.Add(-96 * time.Hour)),
		}},
		billingEvents: []SaaSAdminBillingEvent{{
			ID:              88,
			TenantID:        12,
			EventType:       "renewal",
			PackageCode:     "growth",
			PackageName:     "成长版",
			AmountCents:     18800,
			Currency:        "CNY",
			ExternalOrderNo: "ORDER-OPQ",
			Remark:          "账单异常待核对",
			CreatedAt:       format(now.Add(-96 * time.Hour)),
		}},
		tasks: []SaaSAdminTask{{
			ID:            3301,
			TaskType:      SaaSAdminTaskTypePackageSync,
			Status:        SaaSAdminTaskStatusBlocked,
			TenantID:      12,
			PackageCode:   "growth",
			ActorUserID:   1,
			ActorTenantID: 1,
			Remark:        "套餐同步阻断",
			LastError:     "额度快照冲突",
			CreatedAt:     format(now.Add(-36 * time.Hour)),
			UpdatedAt:     format(now.Add(-30 * time.Hour)),
		}},
		notifications: []SaaSAlertNotification{
			{ID: 4401, NotificationKey: "opq-failed", TenantID: 12, Channel: SaaSAlertNotificationChannelWebhook, Status: SaaSAlertNotificationStatusFailed, Attempts: 1, MaxAttempts: 3, LastError: "webhook failed", CreatedAt: format(now.Add(-10 * time.Hour)), UpdatedAt: format(now.Add(-9 * time.Hour))},
			{ID: 4402, NotificationKey: "opq-dead", TenantID: 12, Channel: SaaSAlertNotificationChannelWebhook, Status: SaaSAlertNotificationStatusDead, Attempts: 3, MaxAttempts: 3, LastError: "webhook dead", CreatedAt: format(now.Add(-20 * time.Hour)), UpdatedAt: format(now.Add(-19 * time.Hour))},
			{ID: 4403, NotificationKey: "opq-closed", TenantID: 12, Channel: SaaSAlertNotificationChannelWebhook, Status: SaaSAlertNotificationStatusClosed, Attempts: 2, MaxAttempts: 3, LastError: "人工关闭", CreatedAt: format(now.Add(-22 * time.Hour)), UpdatedAt: format(now.Add(-21 * time.Hour))},
		},
	}
}

func TestSaaSAdminOperationQueueAllowsPlatformAdmin(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/operationQueue?limit=20&tenantLimit=50&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.OperationQueue(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.ExcludedTenantID != 1 ||
		store.lastRiskFollowUpTaskOptions.ExcludedTenantID != 1 ||
		store.lastBillingFollowUpOptions.ExcludedTenantID != 1 ||
		store.lastTaskOptions.ExcludedTenantID != 1 ||
		store.lastNotificationOptions.ExcludedTenantID != 1 ||
		store.lastNotificationHealthOptions.ExcludedTenantID != 1 {
		t.Fatalf("business tenant scope overview=%+v risk=%+v billing=%+v task=%+v notification=%+v health=%+v", store.lastOptions, store.lastRiskFollowUpTaskOptions, store.lastBillingFollowUpOptions, store.lastTaskOptions, store.lastNotificationOptions, store.lastNotificationHealthOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	filters := data["filters"].(map[string]any)
	if filters["tenantLimit"].(float64) != 50 ||
		filters["limit"].(float64) != 20 ||
		filters["warningHours"].(float64) != 4 ||
		filters["overdueHours"].(float64) != 24 {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["queueCount"].(float64) != 6 ||
		summary["returnedCount"].(float64) != 6 ||
		summary["sourceCount"].(float64) != 5 ||
		summary["customerSuccessCount"].(float64) != 1 ||
		summary["taskSlaCount"].(float64) != 1 ||
		summary["billingFollowUpCount"].(float64) != 1 ||
		summary["notificationCount"].(float64) != 2 ||
		summary["closedNotificationCount"].(float64) != 1 ||
		summary["criticalCount"].(float64) < 3 {
		t.Fatalf("summary = %+v", summary)
	}
	items := data["items"].([]any)
	sources := map[string]bool{}
	for _, raw := range items {
		item := raw.(map[string]any)
		sources[item["source"].(string)] = true
	}
	for _, source := range []string{
		SaaSAdminOperationQueueSourceCustomerSuccess,
		SaaSAdminOperationQueueSourceTaskSLA,
		SaaSAdminOperationQueueSourceBillingFollowUp,
		SaaSAdminOperationQueueSourceNotification,
		SaaSAdminOperationQueueSourceClosedNotification,
	} {
		if !sources[source] {
			t.Fatalf("missing source %s in %+v", source, items)
		}
	}

	closedReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/operationQueue?source=closed_notification&limit=10&tenantLimit=50", nil)
	closedReq.Header.Set("X-Mochat-Go-User-ID", "1")
	closedRec := httptest.NewRecorder()
	handler.OperationQueue(closedRec, closedReq)
	if closedRec.Code != http.StatusOK {
		t.Fatalf("closed status = %d body=%s", closedRec.Code, closedRec.Body.String())
	}
	closedData := decodeSaaSAdminResponse(t, closedRec)
	closedSummary := closedData["summary"].(map[string]any)
	if closedSummary["queueCount"].(float64) != 1 || closedSummary["closedNotificationCount"].(float64) != 1 {
		t.Fatalf("closed summary = %+v", closedSummary)
	}
	for _, raw := range closedData["items"].([]any) {
		if raw.(map[string]any)["source"] != SaaSAdminOperationQueueSourceClosedNotification {
			t.Fatalf("closed item = %+v", raw)
		}
	}
}

func TestSaaSAdminOperationQueueOwnersAllowsPlatformAdmin(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/operationQueueOwners?limit=10&tenantLimit=50&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.OperationQueueOwners(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	filters := data["filters"].(map[string]any)
	if filters["tenantLimit"].(float64) != 50 || filters["limit"].(float64) != 10 {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["queueCount"].(float64) != 6 ||
		summary["customerSuccessCount"].(float64) != 1 ||
		summary["taskSlaCount"].(float64) != 1 ||
		summary["billingFollowUpCount"].(float64) != 1 ||
		summary["notificationCount"].(float64) != 2 ||
		summary["closedNotificationCount"].(float64) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if data["ownerCount"].(float64) != 4 ||
		data["returnedCount"].(float64) != 4 ||
		data["scannedQueueCount"].(float64) != 6 {
		t.Fatalf("owner counts = %+v", data)
	}
	owners := data["owners"].([]any)
	byOwner := map[string]map[string]any{}
	for _, raw := range owners {
		owner := raw.(map[string]any)
		byOwner[owner["owner"].(string)] = owner
	}
	for _, owner := range []string{"CSM-A", "Finance-A", "用户 #1 / 租户 #1", "未分配"} {
		if byOwner[owner] == nil {
			t.Fatalf("missing owner %s in %+v", owner, owners)
		}
	}
	if owner := byOwner["未分配"]; owner["queueCount"].(float64) != 3 ||
		owner["notificationCount"].(float64) != 2 ||
		owner["closedNotificationCount"].(float64) != 1 ||
		owner["unassignedCount"].(float64) != 3 ||
		len(owner["topItems"].([]any)) == 0 {
		t.Fatalf("unassigned owner = %+v", owner)
	}
	if owner := byOwner["CSM-A"]; owner["customerSuccessCount"].(float64) != 1 || owner["criticalCount"].(float64) != 1 {
		t.Fatalf("csm owner = %+v", owner)
	}

	closedReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/operationQueueOwners?source=closed_notification&limit=10&tenantLimit=50", nil)
	closedReq.Header.Set("X-Mochat-Go-User-ID", "1")
	closedRec := httptest.NewRecorder()
	handler.OperationQueueOwners(closedRec, closedReq)
	if closedRec.Code != http.StatusOK {
		t.Fatalf("closed status = %d body=%s", closedRec.Code, closedRec.Body.String())
	}
	closedData := decodeSaaSAdminResponse(t, closedRec)
	if closedData["ownerCount"].(float64) != 1 {
		t.Fatalf("closed owners = %+v", closedData)
	}
	closedOwner := closedData["owners"].([]any)[0].(map[string]any)
	if closedOwner["owner"].(string) != "未分配" ||
		closedOwner["queueCount"].(float64) != 1 ||
		closedOwner["closedNotificationCount"].(float64) != 1 {
		t.Fatalf("closed owner = %+v", closedOwner)
	}
}

func TestSaaSAdminOperationQueueAssignAllowsPlatformAdmin(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/operationQueueAssign?limit=20&tenantLimit=50&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24", strings.NewReader(`{"owner":"Ops-A","status":"contacted","nextFollowUpAt":"2026-07-20 09:00:00","remark":"统一分派运营待办"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.OperationQueueAssign(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["matchedCount"].(float64) != 6 ||
		data["assignableCount"].(float64) != 6 ||
		data["assignedCount"].(float64) != 6 ||
		data["skippedCount"].(float64) != 0 ||
		data["unsupportedCount"].(float64) != 0 ||
		data["customerSuccessAssignedCount"].(float64) != 1 ||
		data["billingFollowUpAssignedCount"].(float64) != 1 ||
		data["queueAssignmentAssignedCount"].(float64) != 4 ||
		data["taskSlaAssignedCount"].(float64) != 1 ||
		data["notificationAssignedCount"].(float64) != 2 ||
		data["closedNotificationAssignedCount"].(float64) != 1 {
		t.Fatalf("assign data = %+v", data)
	}
	if data["owner"].(string) != "Ops-A" ||
		data["status"].(string) != SaaSAdminRiskFollowUpStatusContacted ||
		data["nextFollowUpAt"].(string) != "2026-07-20 09:00:00" ||
		data["remark"].(string) != "统一分派运营待办" {
		t.Fatalf("assign fields = %+v", data)
	}
	if store.lastRiskFollowUp.TenantID != 12 ||
		store.lastRiskFollowUp.Owner != "Ops-A" ||
		store.lastRiskFollowUp.Status != SaaSAdminRiskFollowUpStatusContacted ||
		store.lastRiskFollowUp.NextFollowUpAt != "2026-07-20 09:00:00" ||
		store.lastRiskFollowUp.Remark != "统一分派运营待办" ||
		store.lastRiskFollowUp.ActorUserID != 1 ||
		store.lastRiskFollowUp.ActorTenantID != 1 {
		t.Fatalf("risk follow-up = %+v", store.lastRiskFollowUp)
	}
	if store.billingEventByIDCalls != 1 || store.lastBillingEventByID != 88 || store.recordOperationLogCalls != 5 {
		t.Fatalf("calls billing=%d last=%d operation=%d", store.billingEventByIDCalls, store.lastBillingEventByID, store.recordOperationLogCalls)
	}
	var billingLog, queueLog bool
	for _, log := range store.operationLogs {
		if log.Action == SaaSAdminOperationActionBillingReconciliationFollowUp &&
			log.TargetType == SaaSAdminOperationTargetBillingEvent &&
			log.TargetID == "88" &&
			log.Remark == "统一分派运营待办" &&
			strings.Contains(log.AfterJSON, `"owner":"Ops-A"`) {
			billingLog = true
		}
		if log.Action == SaaSAdminOperationActionOperationQueueAssign &&
			log.TargetType == SaaSAdminOperationTargetAdminTask &&
			log.TargetID == "3301" &&
			strings.Contains(log.AfterJSON, `"source":"task_sla"`) &&
			strings.Contains(log.AfterJSON, `"owner":"Ops-A"`) {
			queueLog = true
		}
	}
	if !billingLog || !queueLog {
		t.Fatalf("operation logs billing=%v queue=%v logs=%+v", billingLog, queueLog, store.operationLogs)
	}
	items := data["items"].([]any)
	if len(items) != 6 {
		t.Fatalf("items = %+v", items)
	}
}

func TestSaaSAdminOperationQueueAssignRecordsTaskSLAOwner(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/operationQueueAssign?source=task_sla&limit=10&tenantLimit=50", strings.NewReader(`{"owner":"Ops-A"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.OperationQueueAssign(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["matchedCount"].(float64) != 1 ||
		data["assignableCount"].(float64) != 1 ||
		data["assignedCount"].(float64) != 1 ||
		data["skippedCount"].(float64) != 0 ||
		data["unsupportedCount"].(float64) != 0 ||
		data["queueAssignmentAssignedCount"].(float64) != 1 ||
		data["taskSlaAssignedCount"].(float64) != 1 {
		t.Fatalf("assign data = %+v", data)
	}
	if store.riskFollowUpCalls != 0 || store.taskUpdateCalls != 0 || store.recordOperationLogCalls != 1 {
		t.Fatalf("unexpected writes risk=%d task=%d operation=%d", store.riskFollowUpCalls, store.taskUpdateCalls, store.recordOperationLogCalls)
	}
	log := store.lastRecordedOperationLog
	if log.Action != SaaSAdminOperationActionOperationQueueAssign ||
		log.TargetType != SaaSAdminOperationTargetAdminTask ||
		log.TargetID != "3301" ||
		!strings.Contains(log.AfterJSON, `"source":"task_sla"`) ||
		!strings.Contains(log.AfterJSON, `"owner":"Ops-A"`) {
		t.Fatalf("operation log = %+v", log)
	}
	items := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %+v", items)
	}
	assignment := items[0].(map[string]any)["operationQueueAssignment"].(map[string]any)
	if assignment["owner"].(string) != "Ops-A" ||
		assignment["source"].(string) != SaaSAdminOperationQueueSourceTaskSLA ||
		assignment["objectId"].(string) != "3301" {
		t.Fatalf("assignment = %+v", assignment)
	}

	queueReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/operationQueue?source=task_sla&owner=Ops-A&limit=10&tenantLimit=50", nil)
	queueReq.Header.Set("X-Mochat-Go-User-ID", "1")
	queueRec := httptest.NewRecorder()
	handler.OperationQueue(queueRec, queueReq)
	if queueRec.Code != http.StatusOK {
		t.Fatalf("queue status = %d body=%s", queueRec.Code, queueRec.Body.String())
	}
	queueData := decodeSaaSAdminResponse(t, queueRec)
	queueItems := queueData["items"].([]any)
	if len(queueItems) != 1 {
		t.Fatalf("queue items = %+v", queueItems)
	}
	queueItem := queueItems[0].(map[string]any)
	if queueItem["owner"].(string) != "Ops-A" || queueItem["assignment"].(map[string]any)["owner"].(string) != "Ops-A" {
		t.Fatalf("queue item = %+v", queueItem)
	}

	ownersReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/operationQueueOwners?source=task_sla&owner=Ops-A&limit=10&tenantLimit=50", nil)
	ownersReq.Header.Set("X-Mochat-Go-User-ID", "1")
	ownersRec := httptest.NewRecorder()
	handler.OperationQueueOwners(ownersRec, ownersReq)
	if ownersRec.Code != http.StatusOK {
		t.Fatalf("owners status = %d body=%s", ownersRec.Code, ownersRec.Body.String())
	}
	ownersData := decodeSaaSAdminResponse(t, ownersRec)
	owners := ownersData["owners"].([]any)
	if len(owners) != 1 {
		t.Fatalf("owners = %+v", owners)
	}
	owner := owners[0].(map[string]any)
	if owner["owner"].(string) != "Ops-A" ||
		owner["taskSlaCount"].(float64) != 1 ||
		owner["unassignedCount"].(float64) != 0 {
		t.Fatalf("owner = %+v", owner)
	}
}

func TestSaaSAdminOperationQueueAssignmentsAllowsPlatformAdmin(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	store.operationLogs = []SaaSAdminOperationLog{
		{
			ID:            1001,
			TenantID:      12,
			ActorUserID:   1,
			ActorTenantID: 1,
			Action:        SaaSAdminOperationActionOperationQueueAssign,
			TargetType:    SaaSAdminOperationTargetAdminTask,
			TargetID:      "3301",
			TargetName:    "运营任务 SLA：package_sync #3301",
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"tenantId":       12,
				"source":         SaaSAdminOperationQueueSourceTaskSLA,
				"objectType":     SaaSAdminOperationTargetAdminTask,
				"objectId":       "3301",
				"targetName":     "运营任务 SLA：package_sync #3301",
				"owner":          "Ops-A",
				"status":         SaaSAdminRiskFollowUpStatusContacted,
				"nextFollowUpAt": "2999-07-20 09:00:00",
				"remark":         "任务 SLA 已认领",
				"assignedAt":     "2026-07-10 10:00:00",
			}),
			Remark:    "任务 SLA 已认领",
			CreatedAt: "2026-07-10 10:00:00",
		},
		{
			ID:            1002,
			TenantID:      12,
			ActorUserID:   1,
			ActorTenantID: 1,
			Action:        SaaSAdminOperationActionOperationQueueAssign,
			TargetType:    SaaSAdminOperationTargetAlertNotification,
			TargetID:      "4401",
			TargetName:    "通知 outbox：opq-failed",
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"tenantId":   12,
				"source":     SaaSAdminOperationQueueSourceNotification,
				"objectType": SaaSAdminOperationTargetAlertNotification,
				"objectId":   "4401",
				"targetName": "通知 outbox：opq-failed",
				"owner":      "Ops-B",
				"status":     SaaSAdminRiskFollowUpStatusPending,
				"remark":     "通知失败已认领",
				"assignedAt": "2026-07-10 10:01:00",
			}),
			Remark:    "通知失败已认领",
			CreatedAt: "2026-07-10 10:01:00",
		},
		{
			ID:            1003,
			TenantID:      13,
			ActorUserID:   1,
			ActorTenantID: 1,
			Action:        SaaSAdminOperationActionOperationQueueAssign,
			TargetType:    SaaSAdminOperationTargetAdminTask,
			TargetID:      "3302",
			TargetName:    "运营任务 SLA：package_sync #3302",
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"tenantId":   13,
				"source":     SaaSAdminOperationQueueSourceTaskSLA,
				"objectType": SaaSAdminOperationTargetAdminTask,
				"objectId":   "3302",
				"targetName": "运营任务 SLA：package_sync #3302",
				"owner":      "Ops-A",
				"status":     SaaSAdminRiskFollowUpStatusContacted,
				"remark":     "任务 SLA 跨租户认领",
				"assignedAt": "2026-07-10 10:02:00",
			}),
			Remark:    "任务 SLA 跨租户认领",
			CreatedAt: "2026-07-10 10:02:00",
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/operationQueueAssignments?tenantId=12&source=task_sla&owner=Ops-A&dueState=future&keyword=任务%20SLA&limit=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.OperationQueueAssignments(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	filters := data["filters"].(map[string]any)
	if filters["source"].(string) != SaaSAdminOperationQueueSourceTaskSLA ||
		filters["owner"].(string) != "Ops-A" ||
		filters["dueState"].(string) != SaaSAdminRiskFollowUpDueStateFuture ||
		filters["tenantId"].(float64) != 12 ||
		filters["keyword"].(string) != "任务 SLA" ||
		filters["limit"].(float64) != 10 {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if data["assignmentCount"].(float64) != 1 ||
		data["returnedCount"].(float64) != 1 ||
		summary["taskSlaCount"].(float64) != 1 ||
		summary["notificationCount"].(float64) != 0 ||
		summary["futureCount"].(float64) != 1 ||
		summary["nextFollowUpAt"].(string) != "2999-07-20 09:00:00" ||
		summary["ownerCount"].(float64) != 1 {
		t.Fatalf("summary/data = %+v %+v", summary, data)
	}
	assignments := data["assignments"].([]any)
	if len(assignments) != 1 {
		t.Fatalf("assignments = %+v", assignments)
	}
	assignment := assignments[0].(map[string]any)
	if assignment["source"].(string) != SaaSAdminOperationQueueSourceTaskSLA ||
		assignment["objectType"].(string) != SaaSAdminOperationTargetAdminTask ||
		assignment["objectId"].(string) != "3301" ||
		assignment["targetName"].(string) != "运营任务 SLA：package_sync #3301" ||
		assignment["owner"].(string) != "Ops-A" ||
		assignment["dueState"].(string) != SaaSAdminRiskFollowUpDueStateFuture ||
		assignment["tenantId"].(float64) != 12 ||
		assignment["operationId"].(float64) != 1001 {
		t.Fatalf("assignment = %+v", assignment)
	}
	if store.lastOperationOptions.Action != SaaSAdminOperationActionOperationQueueAssign ||
		store.lastOperationOptions.TenantID != 12 ||
		store.lastOperationOptions.Keyword != "" {
		t.Fatalf("operation options = %+v", store.lastOperationOptions)
	}
	report := saasAdminBuildOperationQueueAssignmentReport(SaaSAdminOperationQueueAssignmentOptions{
		TenantID: 12,
		Source:   SaaSAdminOperationQueueSourceTaskSLA,
		Owner:    "Ops-A",
		DueState: SaaSAdminRiskFollowUpDueStateFuture,
		Limit:    10,
	}, store.operationLogs)
	if report.Summary.AssignmentCount != 1 ||
		report.Summary.TenantCount != 1 ||
		len(report.Assignments) != 1 ||
		report.Assignments[0].TenantID != 12 {
		t.Fatalf("tenant filtered report = %+v", report)
	}
}

func TestSaaSAdminOperationQueueRejectsTenantAdmin(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/operationQueue", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()

	handler.OperationQueue(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSaaSAdminOperationQueueOwnersRejectsTenantAdmin(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/operationQueueOwners", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()

	handler.OperationQueueOwners(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSaaSAdminOperationQueueAssignmentsRejectsTenantAdmin(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/operationQueueAssignments", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()

	handler.OperationQueueAssignments(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSaaSAdminOperationQueueAssignmentsRejectsInvalidDueState(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/operationQueueAssignments?dueState=stale", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.OperationQueueAssignments(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.operationCalls != 0 {
		t.Fatalf("operationCalls = %d", store.operationCalls)
	}
}

func TestSaaSAdminOperationQueueAssignmentCloseClosesCurrentAssignment(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	store.operationLogs = []SaaSAdminOperationLog{{
		ID:            1,
		TenantID:      12,
		ActorUserID:   1,
		ActorTenantID: 1,
		Action:        SaaSAdminOperationActionOperationQueueAssign,
		TargetType:    SaaSAdminOperationTargetAdminTask,
		TargetID:      "3301",
		TargetName:    "运营任务 SLA：package_sync #3301",
		AfterJSON: saasAdminPayloadJSON(map[string]any{
			"tenantId":       12,
			"source":         SaaSAdminOperationQueueSourceTaskSLA,
			"objectType":     SaaSAdminOperationTargetAdminTask,
			"objectId":       "3301",
			"targetName":     "运营任务 SLA：package_sync #3301",
			"owner":          "Ops-A",
			"status":         SaaSAdminRiskFollowUpStatusContacted,
			"nextFollowUpAt": "2000-07-10 09:00:00",
			"remark":         "任务 SLA 已认领",
		}),
		Remark:    "任务 SLA 已认领",
		CreatedAt: "2026-07-10 10:00:00",
	}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/operationQueueAssignmentClose", strings.NewReader(`{"operationId":1,"closeStatus":"resolved","remark":"任务 SLA 已完成"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.OperationQueueAssignmentClose(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["closed"] != true || data["alreadyClosed"] != false {
		t.Fatalf("close data = %+v", data)
	}
	closed := data["assignment"].(map[string]any)
	if closed["operationId"].(float64) != 2 ||
		closed["status"].(string) != SaaSAdminRiskFollowUpStatusResolved ||
		closed["dueState"].(string) != SaaSAdminRiskFollowUpDueStateClosed ||
		closed["nextFollowUpAt"].(string) != "" ||
		closed["remark"].(string) != "任务 SLA 已完成" {
		t.Fatalf("closed assignment = %+v", closed)
	}
	log := store.lastRecordedOperationLog
	if log.Action != SaaSAdminOperationActionOperationQueueAssignmentClose ||
		log.TargetType != SaaSAdminOperationTargetAdminTask ||
		log.TargetID != "3301" ||
		!strings.Contains(log.BeforeJSON, `"operationId":1`) ||
		!strings.Contains(log.AfterJSON, `"previousOperationId":1`) {
		t.Fatalf("close operation = %+v", log)
	}

	assignmentsReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/operationQueueAssignments?currentOnly=true&dueState=closed&limit=10", nil)
	assignmentsReq.Header.Set("X-Mochat-Go-User-ID", "1")
	assignmentsRec := httptest.NewRecorder()
	handler.OperationQueueAssignments(assignmentsRec, assignmentsReq)
	if assignmentsRec.Code != http.StatusOK {
		t.Fatalf("assignments status = %d body=%s", assignmentsRec.Code, assignmentsRec.Body.String())
	}
	assignmentsData := decodeSaaSAdminResponse(t, assignmentsRec)
	if assignmentsData["assignmentCount"].(float64) != 1 || assignmentsData["returnedCount"].(float64) != 1 {
		t.Fatalf("assignments data = %+v logs=%+v", assignmentsData, store.operationLogs)
	}
	filters := assignmentsData["filters"].(map[string]any)
	if filters["currentOnly"] != true {
		t.Fatalf("assignment filters = %+v", filters)
	}
	current := assignmentsData["assignments"].([]any)[0].(map[string]any)
	if current["operationId"].(float64) != 2 || current["dueState"].(string) != SaaSAdminRiskFollowUpDueStateClosed {
		t.Fatalf("current assignment = %+v", current)
	}

	queueReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/operationQueue?source=task_sla&limit=10&tenantLimit=50", nil)
	queueReq.Header.Set("X-Mochat-Go-User-ID", "1")
	queueRec := httptest.NewRecorder()
	handler.OperationQueue(queueRec, queueReq)
	if queueRec.Code != http.StatusOK {
		t.Fatalf("queue status = %d body=%s", queueRec.Code, queueRec.Body.String())
	}
	queueItem := decodeSaaSAdminResponse(t, queueRec)["items"].([]any)[0].(map[string]any)
	if queueItem["owner"].(string) == "Ops-A" {
		t.Fatalf("closed assignment should release assigned owner: %+v", queueItem)
	}
	if _, exists := queueItem["assignment"]; exists {
		t.Fatalf("closed assignment should not decorate queue item: %+v", queueItem)
	}

	staleReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/operationQueueAssignmentClose", strings.NewReader(`{"operationId":1}`))
	staleReq.Header.Set("Content-Type", "application/json")
	staleReq.Header.Set("X-Mochat-Go-User-ID", "1")
	staleRec := httptest.NewRecorder()
	handler.OperationQueueAssignmentClose(staleRec, staleReq)
	if staleRec.Code != http.StatusConflict {
		t.Fatalf("stale status = %d body=%s", staleRec.Code, staleRec.Body.String())
	}
}

func TestSaaSAdminOperationQueueAssignmentCloseRejectsTenantAdmin(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/operationQueueAssignmentClose", strings.NewReader(`{"operationId":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()

	handler.OperationQueueAssignmentClose(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.recordOperationLogCalls != 0 {
		t.Fatalf("recordOperationLogCalls = %d", store.recordOperationLogCalls)
	}
}

func TestSaaSAdminOperationQueueAssignmentNotificationsUseLatestAssignmentOnly(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	store.operationLogs = []SaaSAdminOperationLog{
		{
			ID:            1,
			TenantID:      12,
			ActorUserID:   1,
			ActorTenantID: 1,
			Action:        SaaSAdminOperationActionOperationQueueAssign,
			TargetType:    SaaSAdminOperationTargetAdminTask,
			TargetID:      "3301",
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"source":         SaaSAdminOperationQueueSourceTaskSLA,
				"objectType":     SaaSAdminOperationTargetAdminTask,
				"objectId":       "3301",
				"targetName":     "旧认领",
				"owner":          "Ops-Old",
				"status":         SaaSAdminRiskFollowUpStatusPending,
				"nextFollowUpAt": "2000-07-10 09:00:00",
				"remark":         "旧认领已逾期",
			}),
			CreatedAt: "2026-07-10 09:00:00",
		},
		{
			ID:            2,
			TenantID:      12,
			ActorUserID:   1,
			ActorTenantID: 1,
			Action:        SaaSAdminOperationActionOperationQueueAssign,
			TargetType:    SaaSAdminOperationTargetAdminTask,
			TargetID:      "3301",
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"source":         SaaSAdminOperationQueueSourceTaskSLA,
				"objectType":     SaaSAdminOperationTargetAdminTask,
				"objectId":       "3301",
				"targetName":     "最新认领",
				"owner":          "Ops-New",
				"status":         SaaSAdminRiskFollowUpStatusContacted,
				"nextFollowUpAt": "2999-07-20 09:00:00",
				"remark":         "最新认领未到期",
			}),
			CreatedAt: "2026-07-10 10:00:00",
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/operationQueueAssignmentNotifications?source=task_sla&dueState=overdue&limit=20", strings.NewReader(`{"remark":"只提醒最新认领"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.OperationQueueAssignmentNotifications(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["matchedCount"].(float64) != 0 || data["eligibleCount"].(float64) != 0 || data["enqueuedCount"].(float64) != 0 {
		t.Fatalf("notification data = %+v", data)
	}
	if data["filters"].(map[string]any)["currentOnly"] != true {
		t.Fatalf("notification filters = %+v", data["filters"])
	}
	if store.notificationEnqueueCalls != 0 || store.recordOperationLogCalls != 0 {
		t.Fatalf("writes notification=%d operation=%d", store.notificationEnqueueCalls, store.recordOperationLogCalls)
	}
}

func TestSaaSAdminOperationQueueAssignmentNotificationsEnqueuesAndSkipsExisting(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	format := func(v time.Time) string {
		return v.Format("2006-01-02 15:04:05")
	}
	options := SaaSAdminOperationQueueAssignmentOptions{
		Source:   SaaSAdminOperationQueueSourceTaskSLA,
		Owner:    "Ops-A",
		DueState: SaaSAdminRiskFollowUpDueStateOverdue,
		Limit:    20,
	}
	existingAssignment := SaaSAdminOperationQueueAssignment{
		TenantID:       12,
		Source:         SaaSAdminOperationQueueSourceTaskSLA,
		ObjectType:     SaaSAdminOperationTargetAdminTask,
		ObjectID:       "3302",
		TargetName:     "运营任务 SLA：package_sync #3302",
		Owner:          "Ops-A",
		Status:         SaaSAdminRiskFollowUpStatusPending,
		DueState:       SaaSAdminRiskFollowUpDueStateOverdue,
		NextFollowUpAt: format(now.Add(-24 * time.Hour)),
		Remark:         "已存在提醒的认领",
		OperationID:    1002,
		ActorUserID:    1,
		ActorTenantID:  1,
		AssignedAt:     format(now.Add(-48 * time.Hour)),
	}
	existingNotify := SaaSAdminOperationQueueAssignmentNotifications{Options: options, Channel: SaaSAlertNotificationChannelWebhook, MaxAttempts: 3, Remark: "existing"}
	existingAlert, existingKey, err := saasAdminOperationQueueAssignmentNotificationAlert(existingAssignment, existingNotify, 1)
	if err != nil {
		t.Fatal(err)
	}
	store := newFakeSaaSAdminOperationQueueStore(t)
	store.operationLogs = []SaaSAdminOperationLog{
		{
			ID:            1001,
			TenantID:      12,
			ActorUserID:   1,
			ActorTenantID: 1,
			Action:        SaaSAdminOperationActionOperationQueueAssign,
			TargetType:    SaaSAdminOperationTargetAdminTask,
			TargetID:      "3301",
			TargetName:    "运营任务 SLA：package_sync #3301",
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"tenantId":       12,
				"source":         SaaSAdminOperationQueueSourceTaskSLA,
				"objectType":     SaaSAdminOperationTargetAdminTask,
				"objectId":       "3301",
				"targetName":     "运营任务 SLA：package_sync #3301",
				"owner":          "Ops-A",
				"status":         SaaSAdminRiskFollowUpStatusPending,
				"nextFollowUpAt": format(now.Add(-48 * time.Hour)),
				"remark":         "任务 SLA 认领已逾期",
				"assignedAt":     format(now.Add(-72 * time.Hour)),
			}),
			Remark:    "任务 SLA 认领已逾期",
			CreatedAt: format(now.Add(-72 * time.Hour)),
		},
		{
			ID:            1002,
			TenantID:      12,
			ActorUserID:   1,
			ActorTenantID: 1,
			Action:        SaaSAdminOperationActionOperationQueueAssign,
			TargetType:    SaaSAdminOperationTargetAdminTask,
			TargetID:      "3302",
			TargetName:    "运营任务 SLA：package_sync #3302",
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"tenantId":       12,
				"source":         SaaSAdminOperationQueueSourceTaskSLA,
				"objectType":     SaaSAdminOperationTargetAdminTask,
				"objectId":       "3302",
				"targetName":     "运营任务 SLA：package_sync #3302",
				"owner":          "Ops-A",
				"status":         SaaSAdminRiskFollowUpStatusPending,
				"nextFollowUpAt": existingAssignment.NextFollowUpAt,
				"remark":         "已存在提醒的认领",
				"assignedAt":     existingAssignment.AssignedAt,
			}),
			Remark:    "已存在提醒的认领",
			CreatedAt: format(now.Add(-48 * time.Hour)),
		},
		{
			ID:            1003,
			TenantID:      12,
			ActorUserID:   1,
			ActorTenantID: 1,
			Action:        SaaSAdminOperationActionOperationQueueAssign,
			TargetType:    SaaSAdminOperationTargetAdminTask,
			TargetID:      "3303",
			TargetName:    "运营任务 SLA：package_sync #3303",
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"tenantId":       12,
				"source":         SaaSAdminOperationQueueSourceTaskSLA,
				"objectType":     SaaSAdminOperationTargetAdminTask,
				"objectId":       "3303",
				"targetName":     "运营任务 SLA：package_sync #3303",
				"owner":          "Ops-A",
				"status":         SaaSAdminRiskFollowUpStatusPending,
				"nextFollowUpAt": "2999-07-20 09:00:00",
				"remark":         "未来认领",
				"assignedAt":     format(now.Add(-24 * time.Hour)),
			}),
			Remark:    "未来认领",
			CreatedAt: format(now.Add(-24 * time.Hour)),
		},
	}
	store.notifications = []SaaSAlertNotification{{
		ID:              601,
		NotificationKey: existingKey,
		AlertKey:        fmt.Sprintf("12:%s:%s:%s", SaaSEventMetricOperationQueueAssignment, SaaSAlertTypeOperationQueueAssign, existingAlert.PeriodKey),
		TenantID:        12,
		Channel:         SaaSAlertNotificationChannelWebhook,
		Status:          SaaSAlertNotificationStatusDelivered,
		MaxAttempts:     3,
		Alert:           existingAlert,
	}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/operationQueueAssignmentNotifications?source=task_sla&owner=Ops-A&dueState=overdue&limit=20", strings.NewReader(`{"maxAttempts":4,"remark":"认领提醒"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.OperationQueueAssignmentNotifications(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationEnqueueCalls != 1 || store.recordOperationLogCalls != 1 {
		t.Fatalf("enqueue calls=%d operation logs=%d", store.notificationEnqueueCalls, store.recordOperationLogCalls)
	}
	if store.lastNotificationOptions.Keyword != SaaSAlertTypeOperationQueueAssign ||
		store.lastNotificationOptions.Channel != SaaSAlertNotificationChannelWebhook ||
		store.lastNotificationOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("existing notification options = %+v", store.lastNotificationOptions)
	}
	if store.lastEnqueuedChannel != SaaSAlertNotificationChannelWebhook || store.lastEnqueuedMaxAttempts != 4 {
		t.Fatalf("enqueue channel=%q max=%d", store.lastEnqueuedChannel, store.lastEnqueuedMaxAttempts)
	}
	alert := store.lastEnqueuedAlert
	if alert.AlertType != SaaSAlertTypeOperationQueueAssign ||
		alert.Source != "saas_admin.operation_queue.assignment_notification" ||
		alert.Severity != SaaSAlertSeverityCritical ||
		alert.Status.TenantID != 12 ||
		alert.Status.Metric != SaaSEventMetricOperationQueueAssignment ||
		alert.PeriodKey != "operation_queue_assignment_1001_overdue" ||
		!strings.Contains(alert.Message, "运营待办认领 #1001") ||
		alert.Context["operationId"].(int64) != 1001 ||
		alert.Context["source"] != SaaSAdminOperationQueueSourceTaskSLA ||
		alert.Context["dueState"] != SaaSAdminRiskFollowUpDueStateOverdue ||
		alert.Context["notificationRemark"] != "认领提醒" {
		t.Fatalf("alert = %+v", alert)
	}
	operation := store.lastRecordedOperationLog
	if operation.Action != SaaSAdminOperationActionOperationQueueAssignmentNotify ||
		operation.TargetType != SaaSAdminOperationTargetAlertNotification ||
		operation.TenantID != 12 ||
		operation.Remark != "认领提醒" ||
		!strings.Contains(operation.AfterJSON, `"source":"operation_queue_assignment"`) ||
		!strings.Contains(operation.AfterJSON, `"operationId":1001`) ||
		!strings.Contains(operation.AfterJSON, SaaSAlertTypeOperationQueueAssign) {
		t.Fatalf("operation = %+v", operation)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["matchedCount"].(float64) != 2 ||
		data["eligibleCount"].(float64) != 2 ||
		data["enqueuedCount"].(float64) != 1 ||
		data["skippedExistingCount"].(float64) != 1 ||
		data["skippedStatusCount"].(float64) != 0 ||
		data["skippedInvalidCount"].(float64) != 0 ||
		data["operationQueueAssignmentNotificationKey"] != SaaSAlertTypeOperationQueueAssign {
		t.Fatalf("data = %+v", data)
	}
	notification := data["notifications"].([]any)[0].(map[string]any)
	if notification["tenantId"].(float64) != 12 ||
		notification["status"] != SaaSAlertNotificationStatusPending ||
		notification["metric"] != SaaSEventMetricOperationQueueAssignment ||
		notification["alertType"] != SaaSAlertTypeOperationQueueAssign {
		t.Fatalf("notification = %+v", notification)
	}
	skipped := data["skipped"].([]any)
	if len(skipped) != 1 || skipped[0].(map[string]any)["operationId"].(float64) != 1002 {
		t.Fatalf("skipped = %+v", skipped)
	}
}

func TestSaaSAdminOperationQueueAssignmentNotificationsRejectsTenantAdminAndInvalidDueState(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/operationQueueAssignmentNotifications", strings.NewReader(`{}`))
	tenantReq.Header.Set("Content-Type", "application/json")
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "2")
	tenantRec := httptest.NewRecorder()
	handler.OperationQueueAssignmentNotifications(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/operationQueueAssignmentNotifications?dueState=future", strings.NewReader(`{}`))
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidReq.Header.Set("X-Mochat-Go-User-ID", "1")
	invalidRec := httptest.NewRecorder()
	handler.OperationQueueAssignmentNotifications(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d body=%s", invalidRec.Code, invalidRec.Body.String())
	}
}

func TestSaaSAdminOperationQueueAssignRejectsTenantAdmin(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/operationQueueAssign", strings.NewReader(`{"owner":"Ops-A"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()

	handler.OperationQueueAssign(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSaaSAdminExportCSVOperationQueueAllowsPlatformAdmin(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=operationQueue&limit=1000&tenantLimit=50&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-operationQueue") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 7 {
		t.Fatalf("records = %+v", records)
	}
	if got := records[0][:4]; strings.Join(got, ",") != "source,priority,tenantId,tenantName" {
		t.Fatalf("header = %+v", records[0])
	}
	assertSaaSAdminCSVRowPrefix(t, records, []string{SaaSAdminOperationQueueSourceTaskSLA})
	assertSaaSAdminCSVRowPrefix(t, records, []string{SaaSAdminOperationQueueSourceNotification})
	assertSaaSAdminCSVRowPrefix(t, records, []string{SaaSAdminOperationQueueSourceClosedNotification})
}

func TestSaaSAdminExportCSVOperationQueueOwnersAllowsPlatformAdmin(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=operationQueueOwners&limit=1000&tenantLimit=50&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-operationQueueOwners") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 5 {
		t.Fatalf("records = %+v", records)
	}
	if got := records[0][:4]; strings.Join(got, ",") != "owner,queueCount,tenantCount,sourceCount" {
		t.Fatalf("header = %+v", records[0])
	}
	assertSaaSAdminCSVRowPrefix(t, records, []string{"CSM-A"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"Finance-A"})
	assertSaaSAdminCSVRowPrefix(t, records, []string{"未分配", "3"})
	for _, record := range records[1:] {
		if record[0] == "未分配" && (record[11] != "2" || record[12] != "1") {
			t.Fatalf("unassigned csv row = %+v", record)
		}
	}
}

func TestSaaSAdminExportCSVOperationQueueAssignmentsAllowsPlatformAdmin(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	store.operationLogs = []SaaSAdminOperationLog{
		{
			ID:            1001,
			TenantID:      12,
			ActorUserID:   1,
			ActorTenantID: 1,
			Action:        SaaSAdminOperationActionOperationQueueAssign,
			TargetType:    SaaSAdminOperationTargetAdminTask,
			TargetID:      "3301",
			TargetName:    "运营任务 SLA：package_sync #3301",
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"tenantId":       12,
				"source":         SaaSAdminOperationQueueSourceTaskSLA,
				"objectType":     SaaSAdminOperationTargetAdminTask,
				"objectId":       "3301",
				"targetName":     "运营任务 SLA：package_sync #3301",
				"owner":          "Ops-A",
				"status":         SaaSAdminRiskFollowUpStatusContacted,
				"nextFollowUpAt": "2999-07-20 09:00:00",
				"remark":         "任务 SLA 已认领",
				"assignedAt":     "2026-07-10 10:00:00",
			}),
			Remark:    "任务 SLA 已认领",
			CreatedAt: "2026-07-10 10:00:00",
		},
		{
			ID:            1002,
			TenantID:      13,
			ActorUserID:   1,
			ActorTenantID: 1,
			Action:        SaaSAdminOperationActionOperationQueueAssign,
			TargetType:    SaaSAdminOperationTargetAdminTask,
			TargetID:      "3302",
			TargetName:    "运营任务 SLA：package_sync #3302",
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"tenantId":   13,
				"source":     SaaSAdminOperationQueueSourceTaskSLA,
				"objectType": SaaSAdminOperationTargetAdminTask,
				"objectId":   "3302",
				"targetName": "运营任务 SLA：package_sync #3302",
				"owner":      "Ops-A",
				"status":     SaaSAdminRiskFollowUpStatusContacted,
				"remark":     "任务 SLA 跨租户认领",
				"assignedAt": "2026-07-10 10:01:00",
			}),
			Remark:    "任务 SLA 跨租户认领",
			CreatedAt: "2026-07-10 10:01:00",
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=operationQueueAssignments&tenantId=12&source=task_sla&owner=Ops-A&dueState=future&keyword=任务%20SLA&limit=1000", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-operationQueueAssignments") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 2 {
		t.Fatalf("records = %+v", records)
	}
	if got := records[0][:5]; strings.Join(got, ",") != "operationId,tenantId,source,objectType,objectId" {
		t.Fatalf("header = %+v", records[0])
	}
	row := records[1]
	if row[0] != "1001" ||
		row[1] != "12" ||
		row[2] != SaaSAdminOperationQueueSourceTaskSLA ||
		row[4] != "3301" ||
		row[5] != "运营任务 SLA：package_sync #3301" ||
		row[6] != "Ops-A" ||
		row[7] != SaaSAdminRiskFollowUpStatusContacted ||
		row[8] != SaaSAdminRiskFollowUpDueStateFuture ||
		row[10] != "任务 SLA 已认领" {
		t.Fatalf("row = %+v", row)
	}
	if store.lastOperationOptions.TenantID != 12 ||
		store.lastOperationOptions.Action != SaaSAdminOperationActionOperationQueueAssign ||
		store.lastOperationOptions.Keyword != "" ||
		store.lastOperationOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("operation options = %+v", store.lastOperationOptions)
	}
}

func TestSaaSAdminCustomerSuccessOwnersSummarizesFullQueue(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 3},
			Tenants: []SaaSAdminTenantOverview{
				{
					TenantID:        12,
					TenantName:      "逾期风险租户",
					TenantStatus:    1,
					PackageCode:     "growth",
					PackageName:     "成长版",
					OpenAlertCount:  2,
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 12,
					MaxUsageLimit:   10,
					MaxUsageRatio:   1.2,
				},
				{
					TenantID:        13,
					TenantName:      "高用量租户",
					TenantStatus:    1,
					PackageCode:     "growth",
					PackageName:     "成长版",
					OpenAlertCount:  1,
					MaxUsageMetric:  SaaSMetricContacts,
					MaxUsageCurrent: 95,
					MaxUsageLimit:   100,
					MaxUsageRatio:   0.95,
				},
				{
					TenantID:        14,
					TenantName:      "近期跟进租户",
					TenantStatus:    1,
					PackageCode:     "growth",
					PackageName:     "成长版",
					OpenAlertCount:  1,
					MaxUsageMetric:  SaaSMetricRooms,
					MaxUsageCurrent: 85,
					MaxUsageLimit:   100,
					MaxUsageRatio:   0.85,
				},
			},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{
				Metric:         SaaSMetricUsers,
				PeriodKey:      SaaSAlertPeriodLifetime,
				Current:        12,
				Limit:          10,
				UsageRatio:     1.2,
				Status:         "exceeded",
				OpenAlertCount: 2,
			}},
			13: {{
				Metric:         SaaSMetricContacts,
				PeriodKey:      SaaSAlertPeriodLifetime,
				Current:        95,
				Limit:          100,
				UsageRatio:     0.95,
				Status:         "warning",
				OpenAlertCount: 1,
			}},
			14: {{
				Metric:         SaaSMetricRooms,
				PeriodKey:      SaaSAlertPeriodLifetime,
				Current:        85,
				Limit:          100,
				UsageRatio:     0.85,
				Status:         "warning",
				OpenAlertCount: 1,
			}},
		},
		latestRiskFollowUps: map[int]SaaSAdminRiskFollowUpSnapshot{
			12: {
				TenantID:       12,
				TenantName:     "逾期风险租户",
				Status:         SaaSAdminRiskFollowUpStatusPending,
				Owner:          "CSM-A",
				NextFollowUpAt: "2000-01-01 00:00:00",
				OperationID:    1201,
				CreatedAt:      "2026-07-01 12:00:00",
			},
			13: {
				TenantID:       13,
				TenantName:     "高用量租户",
				Status:         SaaSAdminRiskFollowUpStatusContacted,
				Owner:          "CSM-B",
				NextFollowUpAt: "2099-01-10 00:00:00",
				OperationID:    1301,
				CreatedAt:      "2026-07-01 13:00:00",
			},
			14: {
				TenantID:       14,
				TenantName:     "近期跟进租户",
				Status:         SaaSAdminRiskFollowUpStatusPending,
				Owner:          "CSM-A",
				NextFollowUpAt: time.Now().AddDate(0, 0, 3).Format("2006-01-02 15:04:05"),
				OperationID:    1401,
				CreatedAt:      "2026-07-01 14:00:00",
			},
		},
		riskFollowUpSnapshots: []SaaSAdminRiskFollowUpSnapshot{
			{
				TenantID:       12,
				TenantName:     "逾期风险租户",
				Status:         SaaSAdminRiskFollowUpStatusPending,
				Owner:          "CSM-A",
				NextFollowUpAt: "2000-01-01 00:00:00",
				OperationID:    1201,
				CreatedAt:      "2026-07-01 12:00:00",
			},
			{
				TenantID:       13,
				TenantName:     "高用量租户",
				Status:         SaaSAdminRiskFollowUpStatusContacted,
				Owner:          "CSM-B",
				NextFollowUpAt: "2099-01-10 00:00:00",
				OperationID:    1301,
				CreatedAt:      "2026-07-01 13:00:00",
			},
			{
				TenantID:       14,
				TenantName:     "近期跟进租户",
				Status:         SaaSAdminRiskFollowUpStatusPending,
				Owner:          "CSM-A",
				NextFollowUpAt: time.Now().AddDate(0, 0, 3).Format("2006-01-02 15:04:05"),
				OperationID:    1401,
				CreatedAt:      "2026-07-01 14:00:00",
			},
		},
		tasks: []SaaSAdminTask{{
			ID:          3301,
			TaskType:    SaaSAdminTaskTypeTenantRenewal,
			Status:      SaaSAdminTaskStatusBlocked,
			TenantID:    12,
			PackageCode: "growth",
		}},
		notifications: []SaaSAlertNotification{
			{ID: 4401, NotificationKey: "failed", TenantID: 12, Status: SaaSAlertNotificationStatusFailed},
			{ID: 4402, NotificationKey: "dead", TenantID: 12, Status: SaaSAlertNotificationStatusDead},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/customerSuccessOwners?limit=1&tenantLimit=10&expiringDays=10&highUsageRatio=0.8", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.CustomerSuccessOwners(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.Limit != 10 || store.usageCalls != 3 {
		t.Fatalf("overview options=%+v usageCalls=%d", store.lastOptions, store.usageCalls)
	}
	if store.lastRiskFollowUpTaskOptions.Limit != saasAdminExportMaxLimit ||
		store.lastTaskOptions.Limit != saasAdminExportMaxLimit ||
		store.lastNotificationOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("limits risk=%+v tasks=%+v notifications=%+v", store.lastRiskFollowUpTaskOptions, store.lastTaskOptions, store.lastNotificationOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["ownerCount"].(float64) != 2 || data["returnedCount"].(float64) != 1 {
		t.Fatalf("owner counts = %+v", data)
	}
	filters := data["filters"].(map[string]any)
	if filters["limit"].(float64) != 1 || filters["tenantLimit"].(float64) != 10 {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["queueCount"].(float64) != 3 ||
		summary["returnedCount"].(float64) != 3 ||
		summary["criticalCount"].(float64) != 1 ||
		summary["retryableNotificationCount"].(float64) != 2 {
		t.Fatalf("summary = %+v", summary)
	}
	owners := data["owners"].([]any)
	if len(owners) != 1 {
		t.Fatalf("owners = %+v", owners)
	}
	owner := owners[0].(map[string]any)
	if owner["owner"] != "CSM-A" ||
		owner["tenantCount"].(float64) != 2 ||
		owner["criticalCount"].(float64) != 1 ||
		owner["overdueCount"].(float64) != 1 ||
		owner["dueSoonCount"].(float64) != 1 ||
		owner["actionableTaskCount"].(float64) != 1 ||
		owner["retryableNotificationCount"].(float64) != 2 ||
		owner["failedNotificationCount"].(float64) != 1 ||
		owner["deadNotificationCount"].(float64) != 1 ||
		owner["maxHealthScore"].(float64) < owner["averageHealthScore"].(float64) ||
		owner["nextFollowUpAt"] != "2000-01-01 00:00:00" {
		t.Fatalf("owner = %+v", owner)
	}
	topTenants := owner["topTenants"].([]any)
	if len(topTenants) != 2 || topTenants[0].(map[string]any)["tenantId"].(float64) != 12 {
		t.Fatalf("top tenants = %+v", topTenants)
	}
}

func TestSaaSAdminBusinessMetricsSummarizesEstimatedRevenue(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{
				TenantCount:              4,
				ActiveTenantPackageCount: 3,
			},
			Tenants: []SaaSAdminTenantOverview{
				{
					TenantID:      11,
					TenantName:    "增长稳定租户",
					TenantStatus:  1,
					PackageCode:   "growth",
					PackageName:   "增长版",
					PackageStatus: 1,
				},
				{
					TenantID:        12,
					TenantName:      "增长风险租户",
					TenantStatus:    1,
					PackageCode:     "growth",
					PackageName:     "增长版",
					PackageStatus:   1,
					ExpiringSoon:    true,
					OpenAlertCount:  1,
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 90,
					MaxUsageLimit:   100,
					MaxUsageRatio:   0.9,
				},
				{
					TenantID:      13,
					TenantName:    "规模过期租户",
					TenantStatus:  1,
					PackageCode:   "scale",
					PackageName:   "规模版",
					PackageStatus: 1,
					Expired:       true,
				},
				{
					TenantID:      14,
					TenantName:    "企业未知价格租户",
					TenantStatus:  1,
					PackageCode:   "enterprise",
					PackageName:   "企业版",
					PackageStatus: 1,
				},
			},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{
				Metric:     SaaSMetricUsers,
				PeriodKey:  SaaSAlertPeriodLifetime,
				Current:    90,
				Limit:      100,
				UsageRatio: 0.9,
				Status:     "warning",
			}},
		},
		packages: []SaaSAdminPackage{
			{Code: "growth", Name: "增长版", Status: 1},
			{Code: "scale", Name: "规模版", Status: 1},
			{Code: "enterprise", Name: "企业版", Status: 1},
		},
		billingEvents: []SaaSAdminBillingEvent{
			{ID: 1001, EventType: "renewal", PackageCode: "growth", PackageName: "增长版", AmountCents: 1200000, PaidAt: "2026-07-01 00:00:00", CreatedAt: "2026-07-01 00:00:00"},
			{ID: 1002, EventType: "renewal", PackageCode: "scale", PackageName: "规模版", AmountCents: 2400000, PaidAt: "2026-07-02 00:00:00", CreatedAt: "2026-07-02 00:00:00"},
			{ID: 1003, EventType: "setup", PackageCode: "growth", PackageName: "增长版", AmountCents: 50000, CreatedAt: "2026-07-03 00:00:00"},
			{ID: 1004, EventType: "refund", PackageCode: "growth", PackageName: "增长版", AmountCents: 200000, PaidAt: "2026-07-04 00:00:00", CreatedAt: "2026-07-04 00:00:00"},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/businessMetrics?tenantLimit=20&expiringDays=10&highUsageRatio=0.8&billingLimit=100", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.BusinessMetrics(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.Scope != SaaSAdminScopePlatform || store.lastOptions.ExcludedTenantID != 1 || store.lastOptions.Limit != 20 || store.lastOptions.ExpiringDays != 10 || store.lastOptions.DueState != SaaSAdminDueStateAll {
		t.Fatalf("overview options = %+v", store.lastOptions)
	}
	if store.usageCalls != 4 || store.packageCalls != 1 || store.billingCalls != 1 || store.lastBillingOptions.ExcludedTenantID != 1 || store.lastBillingOptions.Limit != 100 {
		t.Fatalf("calls usage=%d package=%d billing=%d billingOptions=%+v", store.usageCalls, store.packageCalls, store.billingCalls, store.lastBillingOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["estimated"] != true || data["packageCount"].(float64) != 3 {
		t.Fatalf("data = %+v", data)
	}
	filters := data["filters"].(map[string]any)
	if filters["tenantLimit"].(float64) != 20 || filters["billingLimit"].(float64) != 100 || filters["highUsageRatio"].(float64) != 0.8 {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["tenantCount"].(float64) != 4 ||
		summary["activeTenantPackageCount"].(float64) != 3 ||
		summary["pricedTenantCount"].(float64) != 2 ||
		summary["unknownPriceTenantCount"].(float64) != 1 ||
		summary["estimatedMrrCents"].(float64) != 200000 ||
		summary["estimatedArrCents"].(float64) != 2400000 ||
		summary["estimatedArpaCents"].(float64) != 100000 ||
		summary["atRiskTenantCount"].(float64) != 2 ||
		summary["atRiskMrrCents"].(float64) != 300000 ||
		summary["expiringSoonTenantCount"].(float64) != 1 ||
		summary["expiringSoonMrrCents"].(float64) != 100000 ||
		summary["expiredTenantCount"].(float64) != 1 ||
		summary["expiredMrrCents"].(float64) != 200000 ||
		summary["recentBillingEventCount"].(float64) != 4 ||
		summary["recentRenewalCount"].(float64) != 2 ||
		summary["recentRefundCount"].(float64) != 1 ||
		summary["recentGrossAmountCents"].(float64) != 3650000 ||
		summary["recentRefundAmountCents"].(float64) != 200000 ||
		summary["recentBillingAmountCents"].(float64) != 3450000 ||
		summary["billingPricePackageCount"].(float64) != 2 ||
		summary["missingBillingPackageCount"].(float64) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	packages := data["packages"].([]any)
	if len(packages) != 3 {
		t.Fatalf("packages = %+v", packages)
	}
	growth := packages[0].(map[string]any)
	if growth["packageCode"] != "growth" ||
		growth["tenantCount"].(float64) != 2 ||
		growth["pricedTenantCount"].(float64) != 2 ||
		growth["estimatedMrrCents"].(float64) != 200000 ||
		growth["atRiskTenantCount"].(float64) != 1 ||
		growth["atRiskMrrCents"].(float64) != 100000 ||
		growth["expiringSoonTenantCount"].(float64) != 1 ||
		growth["latestAmountCents"].(float64) != 1200000 ||
		growth["latestBillingEventId"].(float64) != 1001 ||
		growth["estimated"] != true {
		t.Fatalf("growth = %+v", growth)
	}
	scale := packages[1].(map[string]any)
	if scale["packageCode"] != "scale" ||
		scale["tenantCount"].(float64) != 1 ||
		scale["estimatedMrrCents"].(float64) != 0 ||
		scale["expiredTenantCount"].(float64) != 1 ||
		scale["expiredMrrCents"].(float64) != 200000 {
		t.Fatalf("scale = %+v", scale)
	}
	enterprise := packages[2].(map[string]any)
	if enterprise["packageCode"] != "enterprise" ||
		enterprise["unknownPriceTenantCount"].(float64) != 1 ||
		enterprise["estimated"] != false {
		t.Fatalf("enterprise = %+v", enterprise)
	}
	recent := data["recentBillingEvents"].([]any)
	if len(recent) != 4 {
		t.Fatalf("recent events = %+v", recent)
	}
}

func TestSaaSAdminBusinessTrendsSummarizesMonthlyBillingAndRenewalFunnel(t *testing.T) {
	now := time.Now()
	currentMonth := time.Date(now.Year(), now.Month(), 1, 10, 0, 0, 0, time.Local)
	previousMonth := currentMonth.AddDate(0, -1, 0)
	outsideWindow := currentMonth.AddDate(0, -2, 0)
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		billingEvents: []SaaSAdminBillingEvent{
			{ID: 2001, TenantID: 11, EventType: "renewal", PackageCode: "growth", PackageName: "增长版", AmountCents: 1200000, PaidAt: previousMonth.Format("2006-01-02 15:04:05"), CreatedAt: previousMonth.Format("2006-01-02 15:04:05")},
			{ID: 2002, TenantID: 12, EventType: "renewal", PackageCode: "scale", PackageName: "规模版", AmountCents: 2400000, PaidAt: currentMonth.Format("2006-01-02 15:04:05"), CreatedAt: currentMonth.Format("2006-01-02 15:04:05")},
			{ID: 2003, TenantID: 12, EventType: "setup", PackageCode: "scale", PackageName: "规模版", AmountCents: 50000, CreatedAt: currentMonth.AddDate(0, 0, 1).Format("2006-01-02 15:04:05")},
			{ID: 2005, TenantID: 12, EventType: "refund", PackageCode: "scale", PackageName: "规模版", AmountCents: 400000, PaidAt: currentMonth.AddDate(0, 0, 2).Format("2006-01-02 15:04:05"), CreatedAt: currentMonth.AddDate(0, 0, 2).Format("2006-01-02 15:04:05")},
			{ID: 2004, TenantID: 13, EventType: "renewal", PackageCode: "growth", PackageName: "增长版", AmountCents: 900000, PaidAt: outsideWindow.Format("2006-01-02 15:04:05"), CreatedAt: outsideWindow.Format("2006-01-02 15:04:05")},
		},
		tasks: []SaaSAdminTask{
			{ID: 3001, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusPending, TenantID: 11, PackageCode: "growth", ActorUserID: 1},
			{ID: 3002, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusBlocked, TenantID: 11, PackageCode: "growth", ActorUserID: 1},
			{ID: 3003, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusFailed, TenantID: 12, PackageCode: "scale", ActorUserID: 2},
			{ID: 3004, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusApplied, TenantID: 12, PackageCode: "scale", ActorUserID: 2},
			{ID: 3005, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusCanceled, TenantID: 12, PackageCode: "scale", ActorUserID: 2},
			{ID: 3999, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusPending, TenantID: 12, PackageCode: "scale", ActorUserID: 2},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/businessTrends?months=2&billingLimit=10&taskLimit=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.BusinessTrends(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.billingCalls != 1 || store.lastBillingOptions.ExcludedTenantID != 1 || store.lastBillingOptions.Limit != 10 ||
		store.taskCalls != 1 || store.lastTaskOptions.TaskType != SaaSAdminTaskTypeTenantRenewal || store.lastTaskOptions.ExcludedTenantID != 1 || store.lastTaskOptions.Limit != 20 {
		t.Fatalf("calls billing=%d billingOptions=%+v taskCalls=%d taskOptions=%+v", store.billingCalls, store.lastBillingOptions, store.taskCalls, store.lastTaskOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	filters := data["filters"].(map[string]any)
	if filters["months"].(float64) != 2 || filters["billingLimit"].(float64) != 10 || filters["taskLimit"].(float64) != 20 {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["monthCount"].(float64) != 2 ||
		summary["billingEventCount"].(float64) != 4 ||
		summary["renewalCount"].(float64) != 2 ||
		summary["refundCount"].(float64) != 1 ||
		summary["grossAmountCents"].(float64) != 3650000 ||
		summary["refundAmountCents"].(float64) != 400000 ||
		summary["billingAmountCents"].(float64) != 3250000 ||
		summary["tenantCount"].(float64) != 2 ||
		summary["packageCount"].(float64) != 2 ||
		summary["taskCount"].(float64) != 5 ||
		summary["pendingTaskCount"].(float64) != 1 ||
		summary["blockedTaskCount"].(float64) != 1 ||
		summary["failedTaskCount"].(float64) != 1 ||
		summary["appliedTaskCount"].(float64) != 1 ||
		summary["canceledTaskCount"].(float64) != 1 ||
		summary["actionableTaskCount"].(float64) != 3 {
		t.Fatalf("summary = %+v", summary)
	}
	months := data["months"].([]any)
	if len(months) != 2 {
		t.Fatalf("months = %+v", months)
	}
	previous := months[0].(map[string]any)
	current := months[1].(map[string]any)
	if previous["month"] != previousMonth.Format("2006-01") || previous["eventCount"].(float64) != 1 || previous["amountCents"].(float64) != 1200000 {
		t.Fatalf("previous = %+v", previous)
	}
	if current["month"] != currentMonth.Format("2006-01") || current["eventCount"].(float64) != 3 || current["renewalCount"].(float64) != 1 || current["refundCount"].(float64) != 1 || current["grossAmountCents"].(float64) != 2450000 || current["refundAmountCents"].(float64) != 400000 || current["amountCents"].(float64) != 2050000 || current["tenantCount"].(float64) != 1 || current["packageCount"].(float64) != 1 {
		t.Fatalf("current = %+v", current)
	}
	currentPackages := current["packages"].([]any)
	if len(currentPackages) != 1 {
		t.Fatalf("current packages = %+v", currentPackages)
	}
	scale := currentPackages[0].(map[string]any)
	if scale["packageCode"] != "scale" || scale["eventCount"].(float64) != 3 || scale["renewalCount"].(float64) != 1 || scale["refundCount"].(float64) != 1 || scale["grossAmountCents"].(float64) != 2450000 || scale["refundAmountCents"].(float64) != 400000 || scale["amountCents"].(float64) != 2050000 {
		t.Fatalf("scale = %+v", scale)
	}
	funnel := data["renewalFunnel"].(map[string]any)
	funnelSummary := funnel["summary"].(map[string]any)
	if funnelSummary["taskCount"].(float64) != 5 || funnelSummary["tenantRenewalCount"].(float64) != 5 || funnelSummary["actionableCount"].(float64) != 3 {
		t.Fatalf("funnel summary = %+v", funnelSummary)
	}
	recentTasks := funnel["recentTasks"].([]any)
	if len(recentTasks) != 5 || recentTasks[0].(map[string]any)["taskType"] != SaaSAdminTaskTypeTenantRenewal {
		t.Fatalf("recent tasks = %+v", recentTasks)
	}
}

func TestSaaSAdminExportCSVBusinessMetricsAndTrendsAllowsPlatformAdmin(t *testing.T) {
	metricsStore := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{
				TenantCount:              2,
				ActiveTenantPackageCount: 2,
			},
			Tenants: []SaaSAdminTenantOverview{
				{TenantID: 11, TenantName: "增长租户", TenantStatus: 1, PackageCode: "growth", PackageName: "增长版", PackageStatus: 1},
				{
					TenantID:        12,
					TenantName:      "规模风险租户",
					TenantStatus:    1,
					PackageCode:     "scale",
					PackageName:     "规模版",
					PackageStatus:   1,
					ExpiringSoon:    true,
					OpenAlertCount:  1,
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 90,
					MaxUsageLimit:   100,
					MaxUsageRatio:   0.9,
				},
			},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{
				Metric:     SaaSMetricUsers,
				PeriodKey:  SaaSAlertPeriodLifetime,
				Current:    90,
				Limit:      100,
				UsageRatio: 0.9,
				Status:     "warning",
			}},
		},
		packages: []SaaSAdminPackage{
			{Code: "growth", Name: "增长版", Status: 1},
			{Code: "scale", Name: "规模版", Status: 1},
		},
		billingEvents: []SaaSAdminBillingEvent{
			{ID: 1001, TenantID: 11, EventType: "renewal", PackageCode: "growth", PackageName: "增长版", AmountCents: 1200000, PaidAt: "2026-07-01 00:00:00", CreatedAt: "2026-07-01 00:00:00"},
			{ID: 1002, TenantID: 12, EventType: "renewal", PackageCode: "scale", PackageName: "规模版", AmountCents: 2400000, PaidAt: "2026-07-02 00:00:00", CreatedAt: "2026-07-02 00:00:00"},
		},
	}
	metricsHandler := NewSaaSAdminHandler(metricsStore, HeaderUserIDResolver{}, 1)
	metricsReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=businessMetrics&tenantLimit=20&expiringDays=10&highUsageRatio=0.8&billingLimit=100", nil)
	metricsReq.Header.Set("X-Mochat-Go-User-ID", "1")
	metricsRec := httptest.NewRecorder()

	metricsHandler.ExportCSV(metricsRec, metricsReq)

	if metricsRec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d body=%s", metricsRec.Code, metricsRec.Body.String())
	}
	if !strings.Contains(metricsRec.Header().Get("Content-Disposition"), "mochat-saas-businessMetrics") {
		t.Fatalf("metrics headers = %+v", metricsRec.Header())
	}
	if metricsStore.lastOptions.Scope != SaaSAdminScopePlatform ||
		metricsStore.lastOptions.Limit != 20 ||
		metricsStore.lastOptions.ExpiringDays != 10 ||
		metricsStore.lastOptions.DueState != SaaSAdminDueStateAll ||
		metricsStore.usageCalls != 2 ||
		metricsStore.packageCalls != 1 ||
		metricsStore.billingCalls != 1 ||
		metricsStore.lastBillingOptions.Limit != 100 {
		t.Fatalf("metrics calls overview=%+v usage=%d packages=%d billing=%d billingOptions=%+v", metricsStore.lastOptions, metricsStore.usageCalls, metricsStore.packageCalls, metricsStore.billingCalls, metricsStore.lastBillingOptions)
	}
	metricsRecords := readSaaSAdminCSV(t, metricsRec)
	if len(metricsRecords) < 8 || metricsRecords[0][0] != "section" || metricsRecords[0][3] != "remark" {
		t.Fatalf("metrics records = %+v", metricsRecords)
	}
	assertSaaSAdminCSVRowPrefix(t, metricsRecords, []string{"filter", "billingLimit", "100"})
	assertSaaSAdminCSVRowPrefix(t, metricsRecords, []string{"summary", "estimatedMrrCents", "300000"})
	assertSaaSAdminCSVRowPrefix(t, metricsRecords, []string{"summary", "atRiskTenantCount", "1"})
	assertSaaSAdminCSVRowPrefix(t, metricsRecords, []string{"package", "scale", "200000"})
	assertSaaSAdminCSVRowPrefix(t, metricsRecords, []string{"recentBilling", "renewal scale", "2400000"})

	now := time.Now()
	currentMonth := time.Date(now.Year(), now.Month(), 1, 10, 0, 0, 0, time.Local)
	trendStore := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		billingEvents: []SaaSAdminBillingEvent{
			{ID: 2001, TenantID: 12, EventType: "renewal", PackageCode: "scale", PackageName: "规模版", AmountCents: 2400000, PaidAt: currentMonth.Format("2006-01-02 15:04:05"), CreatedAt: currentMonth.Format("2006-01-02 15:04:05")},
		},
		tasks: []SaaSAdminTask{
			{ID: 3001, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusPending, TenantID: 12, PackageCode: "scale", ActorUserID: 1, CreatedAt: currentMonth.Format("2006-01-02 15:04:05")},
			{ID: 3999, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusPending, TenantID: 12, PackageCode: "scale", ActorUserID: 2},
		},
	}
	trendHandler := NewSaaSAdminHandler(trendStore, HeaderUserIDResolver{}, 1)
	trendReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=business_trends&months=2&billingLimit=10&taskLimit=20", nil)
	trendReq.Header.Set("X-Mochat-Go-User-ID", "1")
	trendRec := httptest.NewRecorder()

	trendHandler.ExportCSV(trendRec, trendReq)

	if trendRec.Code != http.StatusOK {
		t.Fatalf("trend status = %d body=%s", trendRec.Code, trendRec.Body.String())
	}
	if !strings.Contains(trendRec.Header().Get("Content-Disposition"), "mochat-saas-businessTrends") {
		t.Fatalf("trend headers = %+v", trendRec.Header())
	}
	if trendStore.billingCalls != 1 ||
		trendStore.lastBillingOptions.Limit != 10 ||
		trendStore.taskCalls != 1 ||
		trendStore.lastTaskOptions.TaskType != SaaSAdminTaskTypeTenantRenewal ||
		trendStore.lastTaskOptions.Limit != 20 {
		t.Fatalf("trend calls billing=%d billingOptions=%+v taskCalls=%d taskOptions=%+v", trendStore.billingCalls, trendStore.lastBillingOptions, trendStore.taskCalls, trendStore.lastTaskOptions)
	}
	trendRecords := readSaaSAdminCSV(t, trendRec)
	if len(trendRecords) < 8 || trendRecords[0][0] != "section" || trendRecords[0][3] != "remark" {
		t.Fatalf("trend records = %+v", trendRecords)
	}
	assertSaaSAdminCSVRowPrefix(t, trendRecords, []string{"filter", "months", "2"})
	assertSaaSAdminCSVRowPrefix(t, trendRecords, []string{"summary", "billingAmountCents", "2400000"})
	assertSaaSAdminCSVRowPrefix(t, trendRecords, []string{"trendPackage", currentMonth.Format("2006-01") + " scale", "2400000"})
	assertSaaSAdminCSVRowPrefix(t, trendRecords, []string{"renewalFunnel", "tenantRenewalCount", "1"})
	assertSaaSAdminCSVRowPrefix(t, trendRecords, []string{"renewalTask", "3001", SaaSAdminTaskStatusPending})
}

func TestSaaSAdminRenewalForecastSummarizesExpiringRevenueAndTasks(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.Local)
	expiresAt := func(days int) string {
		return today.AddDate(0, 0, days).Format("2006-01-02 15:04:05")
	}
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 6},
			Tenants: []SaaSAdminTenantOverview{
				{TenantID: 11, TenantName: "已过期增长租户", TenantStatus: 1, PackageCode: "growth", PackageName: "增长版", PackageStatus: 1, ExpiresAt: expiresAt(-1), Expired: true},
				{TenantID: 12, TenantName: "30天内规模租户", TenantStatus: 1, PackageCode: "scale", PackageName: "规模版", PackageStatus: 1, ExpiresAt: expiresAt(20), ExpiringSoon: true},
				{TenantID: 13, TenantName: "未知价格企业租户", TenantStatus: 1, PackageCode: "enterprise", PackageName: "企业版", PackageStatus: 1, ExpiresAt: expiresAt(45), ExpiringSoon: true},
				{TenantID: 14, TenantName: "90天内增长租户", TenantStatus: 1, PackageCode: "growth", PackageName: "增长版", PackageStatus: 1, ExpiresAt: expiresAt(80), ExpiringSoon: true},
				{TenantID: 15, TenantName: "窗口外规模租户", TenantStatus: 1, PackageCode: "scale", PackageName: "规模版", PackageStatus: 1, ExpiresAt: expiresAt(120)},
				{TenantID: 16, TenantName: "已停用租户", TenantStatus: 2, PackageCode: "growth", PackageName: "增长版", PackageStatus: 1, ExpiresAt: expiresAt(10)},
			},
		},
		billingEvents: []SaaSAdminBillingEvent{
			{ID: 4001, TenantID: 91, EventType: "renewal", PackageCode: "growth", PackageName: "增长版", AmountCents: 1200000, PaidAt: today.AddDate(0, 0, -10).Format("2006-01-02 15:04:05"), CreatedAt: today.AddDate(0, 0, -10).Format("2006-01-02 15:04:05")},
			{ID: 4002, TenantID: 92, EventType: "renewal", PackageCode: "scale", PackageName: "规模版", AmountCents: 2400000, PaidAt: today.AddDate(0, 0, -8).Format("2006-01-02 15:04:05"), CreatedAt: today.AddDate(0, 0, -8).Format("2006-01-02 15:04:05")},
			{ID: 4003, TenantID: 93, EventType: "setup", PackageCode: "enterprise", PackageName: "企业版", AmountCents: 50000, CreatedAt: today.AddDate(0, 0, -6).Format("2006-01-02 15:04:05")},
		},
		tasks: []SaaSAdminTask{
			{ID: 5001, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusFailed, TenantID: 11, PackageCode: "growth", ActorUserID: 1, CreatedAt: today.AddDate(0, 0, -4).Format("2006-01-02 15:04:05")},
			{ID: 5002, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusApplied, TenantID: 12, PackageCode: "scale", ActorUserID: 1, CreatedAt: today.AddDate(0, 0, -5).Format("2006-01-02 15:04:05")},
			{ID: 5003, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusPending, TenantID: 12, PackageCode: "scale", ActorUserID: 1, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
			{ID: 5004, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusBlocked, TenantID: 13, PackageCode: "enterprise", ActorUserID: 2, CreatedAt: today.AddDate(0, 0, -2).Format("2006-01-02 15:04:05")},
			{ID: 5005, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusApplied, TenantID: 14, PackageCode: "growth", ActorUserID: 2, CreatedAt: today.AddDate(0, 0, -3).Format("2006-01-02 15:04:05")},
			{ID: 5999, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusPending, TenantID: 12, PackageCode: "scale", ActorUserID: 2},
		},
		latestRiskFollowUps: map[int]SaaSAdminRiskFollowUpSnapshot{
			12: {TenantID: 12, TenantName: "30天内规模租户", Status: SaaSAdminRiskFollowUpStatusRenewalPending, Owner: "CSM-A", NextFollowUpAt: "2026-07-20 00:00:00", OperationID: 9001, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/renewalForecast?tenantLimit=20&days=90&billingLimit=100&taskLimit=50", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RenewalForecast(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.Scope != SaaSAdminScopePlatform || store.lastOptions.ExcludedTenantID != 1 || store.lastOptions.Limit != 20 || store.lastOptions.ExpiringDays != 90 || store.lastOptions.DueState != SaaSAdminDueStateAll {
		t.Fatalf("overview options = %+v", store.lastOptions)
	}
	if store.lastBillingOptions.EventType != "renewal" || store.lastBillingOptions.ExcludedTenantID != 1 || store.lastBillingOptions.Limit != 100 ||
		store.lastTaskOptions.TaskType != SaaSAdminTaskTypeTenantRenewal || store.lastTaskOptions.ExcludedTenantID != 1 || store.lastTaskOptions.Limit != 50 {
		t.Fatalf("billing=%+v task=%+v", store.lastBillingOptions, store.lastTaskOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["estimated"] != true {
		t.Fatalf("data = %+v", data)
	}
	filters := data["filters"].(map[string]any)
	if filters["tenantLimit"].(float64) != 20 || filters["days"].(float64) != 90 || filters["billingLimit"].(float64) != 100 || filters["taskLimit"].(float64) != 50 {
		t.Fatalf("filters = %+v", filters)
	}
	if filters["bucket"] != SaaSAdminRenewalForecastFilterAll || filters["priced"] != SaaSAdminRenewalForecastFilterAll || filters["taskStatus"] != SaaSAdminRenewalForecastFilterAll || filters["owner"] != "" || filters["packageCode"] != "" {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["tenantCount"].(float64) != 6 ||
		summary["forecastTenantCount"].(float64) != 4 ||
		summary["pricedTenantCount"].(float64) != 3 ||
		summary["unknownPriceTenantCount"].(float64) != 1 ||
		summary["renewalAmountCents"].(float64) != 4800000 ||
		summary["estimatedMrrCents"].(float64) != 400000 ||
		summary["expiredTenantCount"].(float64) != 1 ||
		summary["expiredAmountCents"].(float64) != 1200000 ||
		summary["dueWithin30TenantCount"].(float64) != 1 ||
		summary["dueWithin30AmountCents"].(float64) != 2400000 ||
		summary["due31To60TenantCount"].(float64) != 1 ||
		summary["due31To60AmountCents"].(float64) != 0 ||
		summary["due61To90TenantCount"].(float64) != 1 ||
		summary["due61To90AmountCents"].(float64) != 1200000 ||
		summary["actionableTaskCount"].(float64) != 3 ||
		summary["pendingTaskCount"].(float64) != 1 ||
		summary["blockedTaskCount"].(float64) != 1 ||
		summary["failedTaskCount"].(float64) != 1 ||
		summary["appliedTaskCount"].(float64) != 2 {
		t.Fatalf("summary = %+v", summary)
	}
	buckets := data["buckets"].([]any)
	if len(buckets) != 5 {
		t.Fatalf("buckets = %+v", buckets)
	}
	expired := buckets[0].(map[string]any)
	due30 := buckets[1].(map[string]any)
	due31 := buckets[2].(map[string]any)
	due61 := buckets[3].(map[string]any)
	if expired["bucket"] != "expired" || expired["tenantCount"].(float64) != 1 || expired["renewalAmountCents"].(float64) != 1200000 || expired["actionableTaskCount"].(float64) != 1 {
		t.Fatalf("expired bucket = %+v", expired)
	}
	if due30["bucket"] != "due_0_30" || due30["tenantCount"].(float64) != 1 || due30["renewalAmountCents"].(float64) != 2400000 || due30["actionableTaskCount"].(float64) != 1 {
		t.Fatalf("due30 bucket = %+v", due30)
	}
	if due31["bucket"] != "due_31_60" || due31["tenantCount"].(float64) != 1 || due31["unknownPriceTenantCount"].(float64) != 1 || due31["actionableTaskCount"].(float64) != 1 {
		t.Fatalf("due31 bucket = %+v", due31)
	}
	if due61["bucket"] != "due_61_90" || due61["tenantCount"].(float64) != 1 || due61["renewalAmountCents"].(float64) != 1200000 {
		t.Fatalf("due61 bucket = %+v", due61)
	}
	tenants := data["tenants"].([]any)
	if len(tenants) != 4 {
		t.Fatalf("tenants = %+v", tenants)
	}
	first := tenants[0].(map[string]any)
	if first["tenantId"].(float64) != 11 || first["bucket"] != "expired" || first["renewalAmountCents"].(float64) != 1200000 || first["priced"] != true {
		t.Fatalf("first tenant = %+v", first)
	}
	scale := tenants[1].(map[string]any)
	latestTask := scale["latestTask"].(map[string]any)
	taskSummary := scale["taskSummary"].(map[string]any)
	riskFollowUp := scale["riskFollowUp"].(map[string]any)
	if scale["tenantId"].(float64) != 12 || scale["bucket"] != "due_0_30" || scale["owner"] != "CSM-A" || scale["latestBillingEventId"].(float64) != 4002 || latestTask["id"].(float64) != 5003 || latestTask["status"] != SaaSAdminTaskStatusPending || taskSummary["taskCount"].(float64) != 2 || taskSummary["actionableCount"].(float64) != 1 || riskFollowUp["owner"] != "CSM-A" {
		t.Fatalf("scale tenant = %+v", scale)
	}
	unknown := tenants[2].(map[string]any)
	if unknown["tenantId"].(float64) != 13 || unknown["priced"] != false || unknown["renewalAmountCents"].(float64) != 0 {
		t.Fatalf("unknown tenant = %+v", unknown)
	}
	owners := data["owners"].([]any)
	if len(owners) != 2 {
		t.Fatalf("owners = %+v", owners)
	}
	ownerByName := map[string]map[string]any{}
	for _, raw := range owners {
		owner := raw.(map[string]any)
		ownerByName[owner["owner"].(string)] = owner
	}
	csmOwner := ownerByName["CSM-A"]
	if csmOwner["tenantCount"].(float64) != 1 ||
		csmOwner["renewalAmountCents"].(float64) != 2400000 ||
		csmOwner["dueWithin30TenantCount"].(float64) != 1 ||
		csmOwner["actionableTaskCount"].(float64) != 1 ||
		csmOwner["nextFollowUpAt"] != "2026-07-20 00:00:00" {
		t.Fatalf("csm owner = %+v", csmOwner)
	}
	topTenants := csmOwner["topTenants"].([]any)
	if len(topTenants) != 1 || topTenants[0].(map[string]any)["tenantId"].(float64) != 12 {
		t.Fatalf("topTenants = %+v", topTenants)
	}
	unassignedOwner := ownerByName["未分配"]
	if unassignedOwner["tenantCount"].(float64) != 3 ||
		unassignedOwner["expiredTenantCount"].(float64) != 1 ||
		unassignedOwner["unknownPriceTenantCount"].(float64) != 1 {
		t.Fatalf("unassigned owner = %+v", unassignedOwner)
	}
	renewalTasks := data["renewalTasks"].(map[string]any)
	renewalSummary := renewalTasks["summary"].(map[string]any)
	if renewalSummary["taskCount"].(float64) != 5 || renewalSummary["tenantRenewalCount"].(float64) != 5 || renewalSummary["actionableCount"].(float64) != 3 {
		t.Fatalf("renewal summary = %+v", renewalSummary)
	}
	recentBilling := data["recentBillingEvents"].([]any)
	if len(recentBilling) != 2 {
		t.Fatalf("recent billing = %+v", recentBilling)
	}
}

func TestSaaSAdminRenewalForecastFiltersBucketPriceOwnerPackageAndTaskStatus(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.Local)
	expiresAt := func(days int) string {
		return today.AddDate(0, 0, days).Format("2006-01-02 15:04:05")
	}
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 4},
			Tenants: []SaaSAdminTenantOverview{
				{TenantID: 12, TenantName: "目标规模租户", TenantStatus: 1, PackageCode: "scale", PackageName: "规模版", PackageStatus: 1, ExpiresAt: expiresAt(20), ExpiringSoon: true},
				{TenantID: 13, TenantName: "负责人不匹配租户", TenantStatus: 1, PackageCode: "scale", PackageName: "规模版", PackageStatus: 1, ExpiresAt: expiresAt(21), ExpiringSoon: true},
				{TenantID: 14, TenantName: "未知价格企业租户", TenantStatus: 1, PackageCode: "enterprise", PackageName: "企业版", PackageStatus: 1, ExpiresAt: expiresAt(20), ExpiringSoon: true},
				{TenantID: 15, TenantName: "窗口外规模租户", TenantStatus: 1, PackageCode: "scale", PackageName: "规模版", PackageStatus: 1, ExpiresAt: expiresAt(80), ExpiringSoon: true},
			},
		},
		billingEvents: []SaaSAdminBillingEvent{
			{ID: 4002, TenantID: 92, EventType: "renewal", PackageCode: "scale", PackageName: "规模版", AmountCents: 2400000, PaidAt: today.AddDate(0, 0, -8).Format("2006-01-02 15:04:05"), CreatedAt: today.AddDate(0, 0, -8).Format("2006-01-02 15:04:05")},
		},
		tasks: []SaaSAdminTask{
			{ID: 5002, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusApplied, TenantID: 12, PackageCode: "scale", ActorUserID: 1, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
			{ID: 5003, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusPending, TenantID: 13, PackageCode: "scale", ActorUserID: 1, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
		},
		latestRiskFollowUps: map[int]SaaSAdminRiskFollowUpSnapshot{
			12: {TenantID: 12, TenantName: "目标规模租户", Status: SaaSAdminRiskFollowUpStatusRenewalPending, Owner: "CSM-A", NextFollowUpAt: "2026-07-20 00:00:00", OperationID: 9001, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
			13: {TenantID: 13, TenantName: "负责人不匹配租户", Status: SaaSAdminRiskFollowUpStatusPending, Owner: "CSM-B", NextFollowUpAt: "2026-07-21 00:00:00", OperationID: 9002, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/renewalForecast?tenantLimit=20&days=90&billingLimit=100&taskLimit=50&bucket=due_0_30&priced=true&packageCode=scale&owner=CSM-A&taskStatus=applied", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RenewalForecast(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.PackageCode != "scale" {
		t.Fatalf("overview options = %+v", store.lastOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	filters := data["filters"].(map[string]any)
	if filters["bucket"] != "due_0_30" ||
		filters["priced"] != SaaSAdminRenewalForecastPriceStatePriced ||
		filters["packageCode"] != "scale" ||
		filters["owner"] != "CSM-A" ||
		filters["taskStatus"] != SaaSAdminTaskStatusApplied {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["forecastTenantCount"].(float64) != 1 || summary["renewalAmountCents"].(float64) != 2400000 || summary["appliedTaskCount"].(float64) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	tenants := data["tenants"].([]any)
	if len(tenants) != 1 {
		t.Fatalf("tenants = %+v", tenants)
	}
	tenant := tenants[0].(map[string]any)
	latestTask := tenant["latestTask"].(map[string]any)
	riskFollowUp := tenant["riskFollowUp"].(map[string]any)
	if tenant["tenantId"].(float64) != 12 || tenant["owner"] != "CSM-A" || tenant["bucket"] != "due_0_30" || tenant["priced"] != true || latestTask["status"] != SaaSAdminTaskStatusApplied || riskFollowUp["owner"] != "CSM-A" {
		t.Fatalf("tenant = %+v", tenant)
	}
	owners := data["owners"].([]any)
	if len(owners) != 1 {
		t.Fatalf("owners = %+v", owners)
	}
	owner := owners[0].(map[string]any)
	if owner["owner"] != "CSM-A" ||
		owner["tenantCount"].(float64) != 1 ||
		owner["renewalAmountCents"].(float64) != 2400000 ||
		owner["appliedTaskCount"].(float64) != 1 ||
		owner["nextFollowUpAt"] != "2026-07-20 00:00:00" {
		t.Fatalf("owner = %+v", owner)
	}
}

func TestSaaSAdminRenewalForecastTasksCreatesTasksFromForecast(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.Local)
	expiresAt := func(days int) string {
		return today.AddDate(0, 0, days).Format("2006-01-02 15:04:05")
	}
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		packages: []SaaSAdminPackage{
			{Code: "growth", Name: "增长版", Status: 1},
			{Code: "scale", Name: "规模版", Status: 1},
			{Code: "enterprise", Name: "企业版", Status: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 5},
			Tenants: []SaaSAdminTenantOverview{
				{TenantID: 12, TenantName: "30天内规模租户", TenantStatus: 1, PackageCode: "scale", PackageName: "规模版", PackageStatus: 1, ExpiresAt: expiresAt(20), ExpiringSoon: true},
				{TenantID: 13, TenantName: "已有任务企业租户", TenantStatus: 1, PackageCode: "enterprise", PackageName: "企业版", PackageStatus: 1, ExpiresAt: expiresAt(45), ExpiringSoon: true},
				{TenantID: 14, TenantName: "90天内增长租户", TenantStatus: 1, PackageCode: "growth", PackageName: "增长版", PackageStatus: 1, ExpiresAt: expiresAt(80), ExpiringSoon: true},
				{TenantID: 15, TenantName: "窗口外规模租户", TenantStatus: 1, PackageCode: "scale", PackageName: "规模版", PackageStatus: 1, ExpiresAt: expiresAt(120)},
				{TenantID: 16, TenantName: "已停用租户", TenantStatus: 2, PackageCode: "growth", PackageName: "增长版", PackageStatus: 1, ExpiresAt: expiresAt(10)},
			},
		},
		billingEvents: []SaaSAdminBillingEvent{
			{ID: 4001, TenantID: 91, EventType: "renewal", PackageCode: "growth", PackageName: "增长版", AmountCents: 1200000, PaidAt: today.AddDate(0, 0, -10).Format("2006-01-02 15:04:05"), CreatedAt: today.AddDate(0, 0, -10).Format("2006-01-02 15:04:05")},
			{ID: 4002, TenantID: 92, EventType: "renewal", PackageCode: "scale", PackageName: "规模版", AmountCents: 2400000, PaidAt: today.AddDate(0, 0, -8).Format("2006-01-02 15:04:05"), CreatedAt: today.AddDate(0, 0, -8).Format("2006-01-02 15:04:05")},
		},
		tasks: []SaaSAdminTask{
			{ID: 5001, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusPending, TenantID: 13, PackageCode: "enterprise", ActorUserID: 2, CreatedAt: today.AddDate(0, 0, -2).Format("2006-01-02 15:04:05")},
			{ID: 5002, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusApplied, TenantID: 14, PackageCode: "growth", ActorUserID: 2, CreatedAt: today.AddDate(0, 0, -3).Format("2006-01-02 15:04:05")},
			{ID: 5999, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusPending, TenantID: 12, PackageCode: "scale", ActorUserID: 2},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/renewalForecastTasks?tenantLimit=20&days=90&billingLimit=100&taskLimit=50", strings.NewReader(`{"months":12,"externalOrderNoPrefix":"RF","remark":"预测续费任务"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RenewalForecastTasks(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.Scope != SaaSAdminScopeTenant || store.lastOptions.TenantID != 14 || store.lastBillingOptions.EventType != "renewal" || store.lastTaskOptions.TaskType != SaaSAdminTaskTypeTenantRenewal {
		t.Fatalf("overview=%+v billing=%+v tasks=%+v", store.lastOptions, store.lastBillingOptions, store.lastTaskOptions)
	}
	if store.taskCreateCalls != 2 || store.recordOperationLogCalls != 2 {
		t.Fatalf("task creates=%d operation logs=%d", store.taskCreateCalls, store.recordOperationLogCalls)
	}
	if store.lastTaskCreate.TaskType != SaaSAdminTaskTypeTenantRenewal ||
		store.lastTaskCreate.Status != SaaSAdminTaskStatusPending ||
		store.lastTaskCreate.TenantID != 14 ||
		store.lastTaskCreate.PackageCode != "growth" ||
		store.lastTaskCreate.ActorUserID != 1 ||
		store.lastTaskCreate.ActorTenantID != 1 ||
		store.lastTaskCreate.Remark != "预测续费任务" {
		t.Fatalf("last task create = %+v", store.lastTaskCreate)
	}
	if !strings.Contains(store.lastTaskCreate.RequestJSON, `"amountCents":1200000`) ||
		!strings.Contains(store.lastTaskCreate.RequestJSON, `"externalOrderNo":"RF-14"`) ||
		!strings.Contains(store.lastTaskCreate.RequestJSON, `"remark":"预测续费任务"`) {
		t.Fatalf("last task request=%s preview=%s", store.lastTaskCreate.RequestJSON, store.lastTaskCreate.PreviewJSON)
	}
	operation := store.lastRecordedOperationLog
	if operation.Action != SaaSAdminOperationActionTaskCreate ||
		operation.TargetType != SaaSAdminOperationTargetAdminTask ||
		operation.TenantID != 14 ||
		operation.Remark != "预测续费任务" ||
		!strings.Contains(operation.AfterJSON, `"source":"renewal_forecast"`) ||
		!strings.Contains(operation.AfterJSON, `"days":90`) {
		t.Fatalf("operation = %+v", operation)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["matchedCount"].(float64) != 3 ||
		data["createdCount"].(float64) != 2 ||
		data["pendingCount"].(float64) != 2 ||
		data["blockedCount"].(float64) != 0 ||
		data["skippedExistingCount"].(float64) != 1 ||
		data["skippedInvalidCount"].(float64) != 0 ||
		data["months"].(float64) != 12 ||
		data["remark"] != "预测续费任务" {
		t.Fatalf("data = %+v", data)
	}
	filters := data["filters"].(map[string]any)
	if filters["days"].(float64) != 90 || filters["tenantLimit"].(float64) != 20 || filters["billingLimit"].(float64) != 100 || filters["taskLimit"].(float64) != 50 {
		t.Fatalf("filters = %+v", filters)
	}
	tasks := data["tasks"].([]any)
	if len(tasks) != 2 {
		t.Fatalf("tasks = %+v", tasks)
	}
	taskByTenant := map[int]map[string]any{}
	for _, raw := range tasks {
		task := raw.(map[string]any)
		taskByTenant[int(task["tenantId"].(float64))] = task
	}
	scaleTask, ok := taskByTenant[12]
	if !ok || scaleTask["taskType"] != SaaSAdminTaskTypeTenantRenewal || scaleTask["status"] != SaaSAdminTaskStatusPending || scaleTask["packageCode"] != "scale" {
		t.Fatalf("scale task = %+v tasks=%+v", scaleTask, tasks)
	}
	scaleRequest := scaleTask["request"].(map[string]any)
	if scaleRequest["externalOrderNo"] != "RF-12" || scaleRequest["amountCents"].(float64) != 2400000 {
		t.Fatalf("scale request = %+v", scaleRequest)
	}
	growthTask, ok := taskByTenant[14]
	if !ok || growthTask["taskType"] != SaaSAdminTaskTypeTenantRenewal || growthTask["status"] != SaaSAdminTaskStatusPending || growthTask["packageCode"] != "growth" {
		t.Fatalf("growth task = %+v tasks=%+v", growthTask, tasks)
	}
	growthRequest := growthTask["request"].(map[string]any)
	if growthRequest["externalOrderNo"] != "RF-14" || growthRequest["amountCents"].(float64) != 1200000 {
		t.Fatalf("growth request = %+v", growthRequest)
	}
	skipped := data["skipped"].([]any)
	if len(skipped) != 1 || skipped[0].(map[string]any)["tenantId"].(float64) != 13 || skipped[0].(map[string]any)["reason"] != "tenant renewal task already actionable" {
		t.Fatalf("skipped = %+v", skipped)
	}
}

func TestSaaSAdminRenewalForecastNotificationsEnqueuesAndSkipsExisting(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.Local)
	expiresAt := func(days int) string {
		return today.AddDate(0, 0, days).Format("2006-01-02 15:04:05")
	}
	existingPeriodKey := "renewal_" + today.AddDate(0, 0, 45).Format("20060102")
	existingAlert := SaaSQuotaAlert{
		Status:    SaaSQuotaStatus{TenantID: 13, Metric: SaaSEventMetricTenantRenewal},
		AlertType: SaaSAlertTypeTenantRenewal,
		PeriodKey: existingPeriodKey,
	}
	existingKey := SaaSAlertNotificationKey(existingAlert, SaaSAlertNotificationChannelWebhook)
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 2},
			Tenants: []SaaSAdminTenantOverview{
				{TenantID: 12, TenantName: "待提醒预测租户", TenantStatus: 1, PackageCode: "scale", PackageName: "规模版", PackageStatus: 1, ExpiresAt: expiresAt(20), ExpiringSoon: true},
				{TenantID: 13, TenantName: "已有提醒预测租户", TenantStatus: 1, PackageCode: "scale", PackageName: "规模版", PackageStatus: 1, ExpiresAt: expiresAt(45), ExpiringSoon: true},
			},
		},
		billingEvents: []SaaSAdminBillingEvent{
			{ID: 4002, TenantID: 92, EventType: "renewal", PackageCode: "scale", PackageName: "规模版", AmountCents: 2400000, PaidAt: today.AddDate(0, 0, -8).Format("2006-01-02 15:04:05"), CreatedAt: today.AddDate(0, 0, -8).Format("2006-01-02 15:04:05")},
		},
		latestRiskFollowUps: map[int]SaaSAdminRiskFollowUpSnapshot{
			12: {TenantID: 12, TenantName: "待提醒预测租户", Status: SaaSAdminRiskFollowUpStatusRenewalPending, Owner: "forecast-csm", NextFollowUpAt: "2026-07-22 00:00:00", OperationID: 9001, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
			13: {TenantID: 13, TenantName: "已有提醒预测租户", Status: SaaSAdminRiskFollowUpStatusRenewalPending, Owner: "forecast-csm", NextFollowUpAt: "2026-07-23 00:00:00", OperationID: 9002, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
		},
		notifications: []SaaSAlertNotification{{
			ID:              501,
			NotificationKey: existingKey,
			AlertKey:        "13:tenant_renewal:tenant_renewal_reminder:" + existingPeriodKey,
			TenantID:        13,
			Channel:         SaaSAlertNotificationChannelWebhook,
			Status:          SaaSAlertNotificationStatusDelivered,
			MaxAttempts:     3,
			Alert:           existingAlert,
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/renewalForecastNotifications?tenantLimit=20&days=90&billingLimit=100&taskLimit=50&owner=forecast-csm&priced=true", strings.NewReader(`{"reminderDays":30,"maxAttempts":4,"remark":"预测续费提醒"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RenewalForecastNotifications(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationEnqueueCalls != 1 || store.recordOperationLogCalls != 1 {
		t.Fatalf("enqueue calls=%d operation logs=%d", store.notificationEnqueueCalls, store.recordOperationLogCalls)
	}
	if store.lastEnqueuedChannel != SaaSAlertNotificationChannelWebhook || store.lastEnqueuedMaxAttempts != 4 {
		t.Fatalf("enqueue channel=%q max=%d", store.lastEnqueuedChannel, store.lastEnqueuedMaxAttempts)
	}
	alert := store.lastEnqueuedAlert
	if alert.AlertType != SaaSAlertTypeTenantRenewal ||
		alert.Source != "saas_admin.renewal_forecast.notification" ||
		alert.Status.TenantID != 12 ||
		alert.Status.Metric != SaaSEventMetricTenantRenewal ||
		alert.Status.Limit != 30 ||
		alert.PeriodKey != "renewal_"+today.AddDate(0, 0, 20).Format("20060102") ||
		!strings.Contains(alert.Message, "待提醒预测租户") ||
		alert.Context["packageCode"] != "scale" ||
		alert.Context["owner"] != "forecast-csm" ||
		alert.Context["bucket"] != "due_0_30" ||
		alert.Context["renewalAmountCents"].(int64) != 2400000 ||
		alert.Context["latestBillingEventId"].(int64) != 4002 {
		t.Fatalf("alert = %+v", alert)
	}
	operation := store.lastRecordedOperationLog
	if operation.Action != SaaSAdminOperationActionTenantRenewalNotify ||
		operation.TargetType != SaaSAdminOperationTargetAlertNotification ||
		operation.TenantID != 12 ||
		operation.Remark != "预测续费提醒" ||
		!strings.Contains(operation.AfterJSON, `"source":"renewal_forecast"`) ||
		!strings.Contains(operation.AfterJSON, `"owner":"forecast-csm"`) ||
		!strings.Contains(operation.AfterJSON, `"tenant_renewal_reminder"`) {
		t.Fatalf("operation = %+v", operation)
	}
	if store.lastNotificationOptions.Keyword != SaaSAlertTypeTenantRenewal ||
		store.lastNotificationOptions.Channel != SaaSAlertNotificationChannelWebhook ||
		store.lastNotificationOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("existing notification options = %+v", store.lastNotificationOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["matchedCount"].(float64) != 2 ||
		data["enqueuedCount"].(float64) != 1 ||
		data["skippedExistingCount"].(float64) != 1 ||
		data["skippedInvalidCount"].(float64) != 0 ||
		data["channel"] != SaaSAlertNotificationChannelWebhook ||
		data["maxAttempts"].(float64) != 4 ||
		data["reminderDays"].(float64) != 30 ||
		data["remark"] != "预测续费提醒" {
		t.Fatalf("data = %+v", data)
	}
	filters := data["filters"].(map[string]any)
	if filters["days"].(float64) != 90 ||
		filters["owner"] != "forecast-csm" ||
		filters["priced"] != SaaSAdminRenewalForecastPriceStatePriced {
		t.Fatalf("filters = %+v", filters)
	}
	notifications := data["notifications"].([]any)
	if len(notifications) != 1 {
		t.Fatalf("notifications = %+v", notifications)
	}
	notification := notifications[0].(map[string]any)
	if notification["tenantId"].(float64) != 12 ||
		notification["status"] != SaaSAlertNotificationStatusPending ||
		notification["metric"] != SaaSEventMetricTenantRenewal ||
		notification["alertType"] != SaaSAlertTypeTenantRenewal {
		t.Fatalf("notification = %+v", notification)
	}
	skipped := data["skipped"].([]any)
	if len(skipped) != 1 || skipped[0].(map[string]any)["tenantId"].(float64) != 13 {
		t.Fatalf("skipped = %+v", skipped)
	}
}

func TestSaaSAdminRenewalForecastNotificationsRejectsTenantAdminAndInvalidChannel(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/renewalForecastNotifications", strings.NewReader(`{"reminderDays":30}`))
	tenantReq.Header.Set("Content-Type", "application/json")
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.RenewalForecastNotifications(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
	if store.notificationEnqueueCalls != 0 || store.overviewCalls != 0 || store.billingCalls != 0 || store.taskCalls != 0 {
		t.Fatalf("calls enqueue=%d overview=%d billing=%d tasks=%d", store.notificationEnqueueCalls, store.overviewCalls, store.billingCalls, store.taskCalls)
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/renewalForecastNotifications", strings.NewReader(`{"channel":"sms"}`))
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidReq.Header.Set("X-Mochat-Go-User-ID", "1")
	invalidRec := httptest.NewRecorder()
	handler.RenewalForecastNotifications(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d body=%s", invalidRec.Code, invalidRec.Body.String())
	}
	if store.notificationEnqueueCalls != 0 || store.overviewCalls != 0 || store.billingCalls != 0 || store.taskCalls != 0 {
		t.Fatalf("calls after invalid enqueue=%d overview=%d billing=%d tasks=%d", store.notificationEnqueueCalls, store.overviewCalls, store.billingCalls, store.taskCalls)
	}
}

func TestSaaSAdminRenewalForecastAssignAllowsPlatformAdmin(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.Local)
	expiresAt := func(days int) string {
		return today.AddDate(0, 0, days).Format("2006-01-02 15:04:05")
	}
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 3},
			Tenants: []SaaSAdminTenantOverview{
				{TenantID: 12, TenantName: "目标续费租户", TenantStatus: 1, PackageCode: "scale", PackageName: "规模版", PackageStatus: 1, ExpiresAt: expiresAt(20), ExpiringSoon: true},
				{TenantID: 13, TenantName: "负责人不匹配租户", TenantStatus: 1, PackageCode: "scale", PackageName: "规模版", PackageStatus: 1, ExpiresAt: expiresAt(22), ExpiringSoon: true},
				{TenantID: 14, TenantName: "套餐不匹配租户", TenantStatus: 1, PackageCode: "growth", PackageName: "增长版", PackageStatus: 1, ExpiresAt: expiresAt(20), ExpiringSoon: true},
			},
		},
		billingEvents: []SaaSAdminBillingEvent{
			{ID: 4002, TenantID: 92, EventType: "renewal", PackageCode: "scale", PackageName: "规模版", AmountCents: 2400000, PaidAt: today.AddDate(0, 0, -8).Format("2006-01-02 15:04:05"), CreatedAt: today.AddDate(0, 0, -8).Format("2006-01-02 15:04:05")},
			{ID: 4003, TenantID: 93, EventType: "renewal", PackageCode: "growth", PackageName: "增长版", AmountCents: 1200000, PaidAt: today.AddDate(0, 0, -7).Format("2006-01-02 15:04:05"), CreatedAt: today.AddDate(0, 0, -7).Format("2006-01-02 15:04:05")},
		},
		tasks: []SaaSAdminTask{
			{ID: 5002, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusApplied, TenantID: 12, PackageCode: "scale", ActorUserID: 1, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
			{ID: 5003, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusPending, TenantID: 13, PackageCode: "scale", ActorUserID: 1, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
		},
		latestRiskFollowUps: map[int]SaaSAdminRiskFollowUpSnapshot{
			12: {TenantID: 12, TenantName: "目标续费租户", Status: SaaSAdminRiskFollowUpStatusPending, Owner: "CSM-A", NextFollowUpAt: "2026-07-20 00:00:00", OperationID: 9001, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
			13: {TenantID: 13, TenantName: "负责人不匹配租户", Status: SaaSAdminRiskFollowUpStatusPending, Owner: "CSM-B", NextFollowUpAt: "2026-07-21 00:00:00", OperationID: 9002, CreatedAt: today.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/renewalForecastAssign?tenantLimit=20&days=90&billingLimit=100&taskLimit=50&bucket=due_0_30&priced=true&packageCode=scale&owner=CSM-A&taskStatus=applied", strings.NewReader(`{"owner":"CSM-Renewal","nextFollowUpAt":"2026-07-25","remark":"预测分派"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RenewalForecastAssign(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.riskFollowUpCalls != 1 {
		t.Fatalf("risk follow-up calls = %d", store.riskFollowUpCalls)
	}
	if store.lastRiskFollowUp.TenantID != 12 ||
		store.lastRiskFollowUp.Status != SaaSAdminRiskFollowUpStatusRenewalPending ||
		store.lastRiskFollowUp.Owner != "CSM-Renewal" ||
		store.lastRiskFollowUp.NextFollowUpAt != "2026-07-25 00:00:00" ||
		store.lastRiskFollowUp.Remark != "预测分派" ||
		store.lastRiskFollowUp.ActorUserID != 1 ||
		store.lastRiskFollowUp.ActorTenantID != 1 {
		t.Fatalf("follow-up = %+v", store.lastRiskFollowUp)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["matchedCount"].(float64) != 1 ||
		data["assignedCount"].(float64) != 1 ||
		data["status"] != SaaSAdminRiskFollowUpStatusRenewalPending ||
		data["owner"] != "CSM-Renewal" ||
		data["nextFollowUpAt"] != "2026-07-25 00:00:00" {
		t.Fatalf("data = %+v", data)
	}
	filters := data["filters"].(map[string]any)
	if filters["bucket"] != "due_0_30" ||
		filters["priced"] != SaaSAdminRenewalForecastPriceStatePriced ||
		filters["packageCode"] != "scale" ||
		filters["owner"] != "CSM-A" ||
		filters["taskStatus"] != SaaSAdminTaskStatusApplied {
		t.Fatalf("filters = %+v", filters)
	}
	followUps := data["followUps"].([]any)
	if len(followUps) != 1 || followUps[0].(map[string]any)["tenantId"].(float64) != 12 || followUps[0].(map[string]any)["owner"] != "CSM-Renewal" {
		t.Fatalf("followUps = %+v", followUps)
	}
}

func TestSaaSAdminRenewalForecastAssignRequiresPlatformAdminAndOwner(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/renewalForecastAssign", strings.NewReader(`{"owner":"CSM"}`))
	tenantReq.Header.Set("Content-Type", "application/json")
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.RenewalForecastAssign(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
	if store.riskFollowUpCalls != 0 || store.overviewCalls != 0 || store.billingCalls != 0 || store.taskCalls != 0 {
		t.Fatalf("calls follow=%d overview=%d billing=%d tasks=%d", store.riskFollowUpCalls, store.overviewCalls, store.billingCalls, store.taskCalls)
	}

	missingOwnerReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/renewalForecastAssign", strings.NewReader(`{"remark":"缺负责人"}`))
	missingOwnerReq.Header.Set("Content-Type", "application/json")
	missingOwnerReq.Header.Set("X-Mochat-Go-User-ID", "1")
	missingOwnerRec := httptest.NewRecorder()
	handler.RenewalForecastAssign(missingOwnerRec, missingOwnerReq)
	if missingOwnerRec.Code != http.StatusBadRequest {
		t.Fatalf("missing owner status = %d body=%s", missingOwnerRec.Code, missingOwnerRec.Body.String())
	}
	if store.riskFollowUpCalls != 0 || store.overviewCalls != 0 || store.billingCalls != 0 || store.taskCalls != 0 {
		t.Fatalf("calls after invalid follow=%d overview=%d billing=%d tasks=%d", store.riskFollowUpCalls, store.overviewCalls, store.billingCalls, store.taskCalls)
	}
}

func TestSaaSAdminCustomerSuccessRequiresPlatformAdminAndValidPriority(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/customerSuccess", nil)
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.CustomerSuccess(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
	if store.overviewCalls != 0 {
		t.Fatalf("overview calls after tenant request = %d", store.overviewCalls)
	}

	invalidReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/customerSuccess?priority=urgent", nil)
	invalidReq.Header.Set("X-Mochat-Go-User-ID", "1")
	invalidRec := httptest.NewRecorder()
	handler.CustomerSuccess(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d body=%s", invalidRec.Code, invalidRec.Body.String())
	}
	if store.overviewCalls != 0 {
		t.Fatalf("overview calls after invalid request = %d", store.overviewCalls)
	}

	ownerReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/customerSuccessOwners", nil)
	ownerReq.Header.Set("X-Mochat-Go-User-ID", "7")
	ownerRec := httptest.NewRecorder()
	handler.CustomerSuccessOwners(ownerRec, ownerReq)
	if ownerRec.Code != http.StatusForbidden {
		t.Fatalf("owner status = %d body=%s", ownerRec.Code, ownerRec.Body.String())
	}
	if store.overviewCalls != 0 {
		t.Fatalf("overview calls after owner tenant request = %d", store.overviewCalls)
	}

	businessReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/businessMetrics", nil)
	businessReq.Header.Set("X-Mochat-Go-User-ID", "7")
	businessRec := httptest.NewRecorder()
	handler.BusinessMetrics(businessRec, businessReq)
	if businessRec.Code != http.StatusForbidden {
		t.Fatalf("business metrics status = %d body=%s", businessRec.Code, businessRec.Body.String())
	}
	if store.overviewCalls != 0 {
		t.Fatalf("overview calls after business tenant request = %d", store.overviewCalls)
	}

	businessTrendsReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/businessTrends", nil)
	businessTrendsReq.Header.Set("X-Mochat-Go-User-ID", "7")
	businessTrendsRec := httptest.NewRecorder()
	handler.BusinessTrends(businessTrendsRec, businessTrendsReq)
	if businessTrendsRec.Code != http.StatusForbidden {
		t.Fatalf("business trends status = %d body=%s", businessTrendsRec.Code, businessTrendsRec.Body.String())
	}
	if store.billingCalls != 0 || store.taskCalls != 0 {
		t.Fatalf("calls after business trends tenant request billing=%d tasks=%d", store.billingCalls, store.taskCalls)
	}

	renewalForecastReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/renewalForecast", nil)
	renewalForecastReq.Header.Set("X-Mochat-Go-User-ID", "7")
	renewalForecastRec := httptest.NewRecorder()
	handler.RenewalForecast(renewalForecastRec, renewalForecastReq)
	if renewalForecastRec.Code != http.StatusForbidden {
		t.Fatalf("renewal forecast status = %d body=%s", renewalForecastRec.Code, renewalForecastRec.Body.String())
	}
	if store.overviewCalls != 0 || store.billingCalls != 0 || store.taskCalls != 0 {
		t.Fatalf("calls after renewal forecast tenant request overview=%d billing=%d tasks=%d", store.overviewCalls, store.billingCalls, store.taskCalls)
	}

	renewalForecastTasksReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/renewalForecastTasks", strings.NewReader(`{"months":12}`))
	renewalForecastTasksReq.Header.Set("Content-Type", "application/json")
	renewalForecastTasksReq.Header.Set("X-Mochat-Go-User-ID", "7")
	renewalForecastTasksRec := httptest.NewRecorder()
	handler.RenewalForecastTasks(renewalForecastTasksRec, renewalForecastTasksReq)
	if renewalForecastTasksRec.Code != http.StatusForbidden {
		t.Fatalf("renewal forecast tasks status = %d body=%s", renewalForecastTasksRec.Code, renewalForecastTasksRec.Body.String())
	}
	if store.overviewCalls != 0 || store.billingCalls != 0 || store.taskCalls != 0 || store.taskCreateCalls != 0 {
		t.Fatalf("calls after renewal forecast tasks tenant request overview=%d billing=%d tasks=%d creates=%d", store.overviewCalls, store.billingCalls, store.taskCalls, store.taskCreateCalls)
	}
}

func TestSaaSAdminCustomerSuccessAssignAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 1},
			Tenants: []SaaSAdminTenantOverview{{
				TenantID:        12,
				TenantName:      "待分派租户",
				TenantStatus:    1,
				PackageCode:     "growth",
				PackageName:     "成长版",
				PackageStatus:   1,
				OpenAlertCount:  2,
				MaxUsageMetric:  SaaSMetricUsers,
				MaxUsageCurrent: 12,
				MaxUsageLimit:   10,
				MaxUsageRatio:   1.2,
			}},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{
				Metric:         SaaSMetricUsers,
				PeriodKey:      SaaSAlertPeriodLifetime,
				Current:        12,
				Limit:          10,
				UsageRatio:     1.2,
				Status:         "exceeded",
				OpenAlertCount: 2,
			}},
		},
		notifications: []SaaSAlertNotification{
			{ID: 4401, NotificationKey: "failed", TenantID: 12, Status: SaaSAlertNotificationStatusFailed},
		},
		tasks: []SaaSAdminTask{{
			ID:          3301,
			TaskType:    SaaSAdminTaskTypeTenantRenewal,
			Status:      SaaSAdminTaskStatusBlocked,
			TenantID:    12,
			PackageCode: "growth",
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/customerSuccessAssign?limit=5&tenantLimit=10&expiringDays=10&highUsageRatio=0.8&priority=critical", strings.NewReader(`{"owner":"CSM-B","nextFollowUpAt":"2026-07-20","remark":"批量分派"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.CustomerSuccessAssign(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.riskFollowUpCalls != 1 {
		t.Fatalf("risk follow-up calls = %d", store.riskFollowUpCalls)
	}
	if store.lastRiskFollowUp.TenantID != 12 ||
		store.lastRiskFollowUp.Status != SaaSAdminRiskFollowUpStatusPending ||
		store.lastRiskFollowUp.Owner != "CSM-B" ||
		store.lastRiskFollowUp.NextFollowUpAt != "2026-07-20 00:00:00" ||
		store.lastRiskFollowUp.Remark != "批量分派" ||
		store.lastRiskFollowUp.ActorUserID != 1 ||
		store.lastRiskFollowUp.ActorTenantID != 1 {
		t.Fatalf("follow-up = %+v", store.lastRiskFollowUp)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["matchedCount"].(float64) != 1 ||
		data["assignedCount"].(float64) != 1 ||
		data["owner"] != "CSM-B" ||
		data["nextFollowUpAt"] != "2026-07-20 00:00:00" {
		t.Fatalf("data = %+v", data)
	}
	filters := data["filters"].(map[string]any)
	if filters["priority"] != SaaSAdminCustomerSuccessPriorityCritical || filters["limit"].(float64) != 5 || filters["tenantLimit"].(float64) != 10 {
		t.Fatalf("filters = %+v", filters)
	}
	followUps := data["followUps"].([]any)
	if len(followUps) != 1 || followUps[0].(map[string]any)["tenantId"].(float64) != 12 {
		t.Fatalf("followUps = %+v", followUps)
	}
}

func TestSaaSAdminCustomerSuccessAssignRequiresPlatformAdminAndOwner(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/customerSuccessAssign", strings.NewReader(`{"owner":"CSM"}`))
	tenantReq.Header.Set("Content-Type", "application/json")
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.CustomerSuccessAssign(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
	if store.riskFollowUpCalls != 0 || store.overviewCalls != 0 {
		t.Fatalf("calls follow=%d overview=%d", store.riskFollowUpCalls, store.overviewCalls)
	}

	missingOwnerReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/customerSuccessAssign", strings.NewReader(`{"remark":"缺负责人"}`))
	missingOwnerReq.Header.Set("Content-Type", "application/json")
	missingOwnerReq.Header.Set("X-Mochat-Go-User-ID", "1")
	missingOwnerRec := httptest.NewRecorder()
	handler.CustomerSuccessAssign(missingOwnerRec, missingOwnerReq)
	if missingOwnerRec.Code != http.StatusBadRequest {
		t.Fatalf("missing owner status = %d body=%s", missingOwnerRec.Code, missingOwnerRec.Body.String())
	}
	if store.riskFollowUpCalls != 0 || store.overviewCalls != 0 {
		t.Fatalf("calls after invalid follow=%d overview=%d", store.riskFollowUpCalls, store.overviewCalls)
	}
}

func TestSaaSAdminCustomerSuccessRenewalTasksCreatesTasksFromQueue(t *testing.T) {
	renewalBase := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	renewalBaseText := renewalBase.Format("2006-01-02 15:04:05")
	renewalExpiresText := renewalBase.AddDate(0, 6, 0).Format("2006-01-02 15:04:05")
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		packages: []SaaSAdminPackage{{
			Code:   "growth",
			Name:   "成长版",
			Status: 1,
		}},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 2},
			Tenants: []SaaSAdminTenantOverview{
				{
					TenantID:        12,
					TenantName:      "待续费租户",
					TenantStatus:    1,
					PackageCode:     "growth",
					PackageName:     "成长版",
					PackageStatus:   1,
					ExpiresAt:       renewalBaseText,
					OpenAlertCount:  1,
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 12,
					MaxUsageLimit:   10,
					MaxUsageRatio:   1.2,
				},
				{
					TenantID:        13,
					TenantName:      "已有任务租户",
					TenantStatus:    1,
					PackageCode:     "growth",
					PackageName:     "成长版",
					PackageStatus:   1,
					ExpiresAt:       "2026-09-01 00:00:00",
					OpenAlertCount:  1,
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 11,
					MaxUsageLimit:   10,
					MaxUsageRatio:   1.1,
				},
			},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{
				Metric:         SaaSMetricUsers,
				PeriodKey:      SaaSAlertPeriodLifetime,
				Current:        12,
				Limit:          10,
				UsageRatio:     1.2,
				Status:         "exceeded",
				OpenAlertCount: 1,
			}},
			13: {{
				Metric:         SaaSMetricUsers,
				PeriodKey:      SaaSAlertPeriodLifetime,
				Current:        11,
				Limit:          10,
				UsageRatio:     1.1,
				Status:         "exceeded",
				OpenAlertCount: 1,
			}},
		},
		tasks: []SaaSAdminTask{{
			ID:          3301,
			TaskType:    SaaSAdminTaskTypeTenantRenewal,
			Status:      SaaSAdminTaskStatusPending,
			TenantID:    13,
			PackageCode: "growth",
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/customerSuccessRenewalTasks?limit=10&tenantLimit=10&highUsageRatio=0.8", strings.NewReader(`{"months":6,"amount":"199.00","paymentMethod":"manual","externalOrderNoPrefix":"CS","remark":"队列续费任务"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.CustomerSuccessRenewalTasks(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskCreateCalls != 1 || store.recordOperationLogCalls != 1 {
		t.Fatalf("task creates=%d operation logs=%d", store.taskCreateCalls, store.recordOperationLogCalls)
	}
	if store.lastTaskCreate.TaskType != SaaSAdminTaskTypeTenantRenewal ||
		store.lastTaskCreate.Status != SaaSAdminTaskStatusPending ||
		store.lastTaskCreate.TenantID != 12 ||
		store.lastTaskCreate.PackageCode != "growth" ||
		store.lastTaskCreate.ActorUserID != 1 ||
		store.lastTaskCreate.ActorTenantID != 1 ||
		store.lastTaskCreate.Remark != "队列续费任务" {
		t.Fatalf("task create = %+v", store.lastTaskCreate)
	}
	if !strings.Contains(store.lastTaskCreate.RequestJSON, `"expiresAt":"`+renewalExpiresText+`"`) ||
		!strings.Contains(store.lastTaskCreate.RequestJSON, `"amountCents":19900`) ||
		!strings.Contains(store.lastTaskCreate.RequestJSON, `"externalOrderNo":"CS-12"`) ||
		!strings.Contains(store.lastTaskCreate.PreviewJSON, `"previousExpiresAt":"`+renewalBaseText+`"`) {
		t.Fatalf("task json request=%s preview=%s", store.lastTaskCreate.RequestJSON, store.lastTaskCreate.PreviewJSON)
	}
	operation := store.lastRecordedOperationLog
	if operation.Action != SaaSAdminOperationActionTaskCreate ||
		operation.TargetType != SaaSAdminOperationTargetAdminTask ||
		operation.TenantID != 12 ||
		operation.Remark != "队列续费任务" ||
		!strings.Contains(operation.AfterJSON, `"source":"customer_success_queue"`) {
		t.Fatalf("operation = %+v", operation)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["matchedCount"].(float64) != 2 ||
		data["createdCount"].(float64) != 1 ||
		data["pendingCount"].(float64) != 1 ||
		data["skippedExistingCount"].(float64) != 1 ||
		data["months"].(float64) != 6 ||
		data["amountCents"].(float64) != 19900 {
		t.Fatalf("data = %+v", data)
	}
	tasks := data["tasks"].([]any)
	if len(tasks) != 1 || tasks[0].(map[string]any)["taskType"] != SaaSAdminTaskTypeTenantRenewal {
		t.Fatalf("tasks = %+v", tasks)
	}
	skipped := data["skipped"].([]any)
	if len(skipped) != 1 || skipped[0].(map[string]any)["tenantId"].(float64) != 13 {
		t.Fatalf("skipped = %+v", skipped)
	}
}

func TestSaaSAdminCustomerSuccessRenewalNotificationsEnqueuesAndSkipsExisting(t *testing.T) {
	existingAlert := SaaSQuotaAlert{
		Status:    SaaSQuotaStatus{TenantID: 13, Metric: SaaSEventMetricTenantRenewal},
		AlertType: SaaSAlertTypeTenantRenewal,
		PeriodKey: "renewal_20990901",
	}
	existingKey := SaaSAlertNotificationKey(existingAlert, SaaSAlertNotificationChannelWebhook)
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 2},
			Tenants: []SaaSAdminTenantOverview{
				{
					TenantID:        12,
					TenantName:      "待提醒租户",
					TenantStatus:    1,
					PackageCode:     "growth",
					PackageName:     "成长版",
					PackageStatus:   1,
					ExpiresAt:       "2099-08-01 00:00:00",
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 12,
					MaxUsageLimit:   10,
					MaxUsageRatio:   1.2,
				},
				{
					TenantID:        13,
					TenantName:      "已有提醒租户",
					TenantStatus:    1,
					PackageCode:     "growth",
					PackageName:     "成长版",
					PackageStatus:   1,
					ExpiresAt:       "2099-09-01 00:00:00",
					MaxUsageMetric:  SaaSMetricUsers,
					MaxUsageCurrent: 11,
					MaxUsageLimit:   10,
					MaxUsageRatio:   1.1,
				},
			},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{
				Metric:     SaaSMetricUsers,
				PeriodKey:  SaaSAlertPeriodLifetime,
				Current:    12,
				Limit:      10,
				UsageRatio: 1.2,
				Status:     "exceeded",
			}},
			13: {{
				Metric:     SaaSMetricUsers,
				PeriodKey:  SaaSAlertPeriodLifetime,
				Current:    11,
				Limit:      10,
				UsageRatio: 1.1,
				Status:     "exceeded",
			}},
		},
		notifications: []SaaSAlertNotification{{
			ID:              501,
			NotificationKey: existingKey,
			AlertKey:        "13:tenant_renewal:tenant_renewal_reminder:renewal_20990901",
			TenantID:        13,
			Channel:         SaaSAlertNotificationChannelWebhook,
			Status:          SaaSAlertNotificationStatusDelivered,
			MaxAttempts:     3,
			Alert:           existingAlert,
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/customerSuccessRenewalNotifications?limit=10&tenantLimit=10&highUsageRatio=0.8", strings.NewReader(`{"reminderDays":45,"maxAttempts":5,"remark":"队列续费提醒"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.CustomerSuccessRenewalNotifications(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationEnqueueCalls != 1 || store.recordOperationLogCalls != 1 {
		t.Fatalf("enqueue calls=%d operation logs=%d", store.notificationEnqueueCalls, store.recordOperationLogCalls)
	}
	if store.lastEnqueuedChannel != SaaSAlertNotificationChannelWebhook || store.lastEnqueuedMaxAttempts != 5 {
		t.Fatalf("enqueue channel=%q max=%d", store.lastEnqueuedChannel, store.lastEnqueuedMaxAttempts)
	}
	alert := store.lastEnqueuedAlert
	if alert.AlertType != SaaSAlertTypeTenantRenewal ||
		alert.Status.TenantID != 12 ||
		alert.Status.Metric != SaaSEventMetricTenantRenewal ||
		alert.Status.Limit != 45 ||
		alert.PeriodKey != "renewal_20990801" ||
		!strings.Contains(alert.Message, "待提醒租户") ||
		alert.Context["packageCode"] != "growth" ||
		alert.Context["expiresAt"] != "2099-08-01" {
		t.Fatalf("alert = %+v", alert)
	}
	operation := store.lastRecordedOperationLog
	if operation.Action != SaaSAdminOperationActionTenantRenewalNotify ||
		operation.TargetType != SaaSAdminOperationTargetAlertNotification ||
		operation.TenantID != 12 ||
		operation.Remark != "队列续费提醒" ||
		!strings.Contains(operation.AfterJSON, `"source":"customer_success_queue"`) ||
		!strings.Contains(operation.AfterJSON, `"tenant_renewal_reminder"`) {
		t.Fatalf("operation = %+v", operation)
	}
	if store.lastNotificationOptions.Keyword != SaaSAlertTypeTenantRenewal ||
		store.lastNotificationOptions.Channel != SaaSAlertNotificationChannelWebhook ||
		store.lastNotificationOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("existing notification options = %+v", store.lastNotificationOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["matchedCount"].(float64) != 2 ||
		data["enqueuedCount"].(float64) != 1 ||
		data["skippedExistingCount"].(float64) != 1 ||
		data["channel"] != SaaSAlertNotificationChannelWebhook ||
		data["maxAttempts"].(float64) != 5 ||
		data["reminderDays"].(float64) != 45 ||
		data["remark"] != "队列续费提醒" {
		t.Fatalf("data = %+v", data)
	}
	notifications := data["notifications"].([]any)
	if len(notifications) != 1 {
		t.Fatalf("notifications = %+v", notifications)
	}
	notification := notifications[0].(map[string]any)
	if notification["tenantId"].(float64) != 12 ||
		notification["status"] != SaaSAlertNotificationStatusPending ||
		notification["metric"] != SaaSEventMetricTenantRenewal ||
		notification["metricLabel"] != "租户续费" ||
		notification["alertType"] != SaaSAlertTypeTenantRenewal {
		t.Fatalf("notification = %+v", notification)
	}
	skipped := data["skipped"].([]any)
	if len(skipped) != 1 || skipped[0].(map[string]any)["tenantId"].(float64) != 13 {
		t.Fatalf("skipped = %+v", skipped)
	}
}

func TestSaaSAdminCustomerSuccessRenewalNotificationsRejectsTenantAdminAndInvalidChannel(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/customerSuccessRenewalNotifications", strings.NewReader(`{"reminderDays":30}`))
	tenantReq.Header.Set("Content-Type", "application/json")
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.CustomerSuccessRenewalNotifications(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
	if store.notificationEnqueueCalls != 0 || store.overviewCalls != 0 {
		t.Fatalf("calls enqueue=%d overview=%d", store.notificationEnqueueCalls, store.overviewCalls)
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/customerSuccessRenewalNotifications", strings.NewReader(`{"channel":"sms"}`))
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidReq.Header.Set("X-Mochat-Go-User-ID", "1")
	invalidRec := httptest.NewRecorder()
	handler.CustomerSuccessRenewalNotifications(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d body=%s", invalidRec.Code, invalidRec.Body.String())
	}
	if store.notificationEnqueueCalls != 0 || store.overviewCalls != 0 {
		t.Fatalf("calls after invalid enqueue=%d overview=%d", store.notificationEnqueueCalls, store.overviewCalls)
	}
}

func TestSaaSAdminCustomerSuccessRenewalTasksRequiresPlatformAdminAndValidMonths(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/customerSuccessRenewalTasks", strings.NewReader(`{"months":12}`))
	tenantReq.Header.Set("Content-Type", "application/json")
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.CustomerSuccessRenewalTasks(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
	if store.taskCreateCalls != 0 || store.overviewCalls != 0 {
		t.Fatalf("calls create=%d overview=%d", store.taskCreateCalls, store.overviewCalls)
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/customerSuccessRenewalTasks", strings.NewReader(`{"months":-1}`))
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidReq.Header.Set("X-Mochat-Go-User-ID", "1")
	invalidRec := httptest.NewRecorder()
	handler.CustomerSuccessRenewalTasks(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d body=%s", invalidRec.Code, invalidRec.Body.String())
	}
	if store.taskCreateCalls != 0 || store.overviewCalls != 0 {
		t.Fatalf("calls after invalid create=%d overview=%d", store.taskCreateCalls, store.overviewCalls)
	}
}

func TestSaaSAdminExportCSVCustomerSuccessAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 1},
			Tenants: []SaaSAdminTenantOverview{{
				TenantID:        12,
				TenantName:      "高风险租户",
				TenantStatus:    1,
				PackageCode:     "growth",
				PackageName:     "成长版",
				PackageStatus:   1,
				ExpiresAt:       "2026-07-12 00:00:00",
				ExpiringSoon:    true,
				OpenAlertCount:  2,
				MaxUsageMetric:  SaaSMetricUsers,
				MaxUsageCurrent: 12,
				MaxUsageLimit:   10,
				MaxUsageRatio:   1.2,
			}},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{
				Metric:         SaaSMetricUsers,
				PeriodKey:      SaaSAlertPeriodLifetime,
				Current:        12,
				Limit:          10,
				UsageRatio:     1.2,
				Status:         "exceeded",
				OpenAlertCount: 2,
			}},
		},
		latestRiskFollowUps: map[int]SaaSAdminRiskFollowUpSnapshot{
			12: {
				TenantID:       12,
				TenantName:     "高风险租户",
				Status:         SaaSAdminRiskFollowUpStatusPending,
				Owner:          "CSM-A",
				NextFollowUpAt: "2000-01-01 00:00:00",
				Remark:         "需要续费沟通",
				OperationID:    1201,
				CreatedAt:      "2026-07-01 12:00:00",
			},
		},
		riskFollowUpSnapshots: []SaaSAdminRiskFollowUpSnapshot{{
			TenantID:       12,
			TenantName:     "高风险租户",
			Status:         SaaSAdminRiskFollowUpStatusPending,
			Owner:          "CSM-A",
			NextFollowUpAt: "2000-01-01 00:00:00",
			Remark:         "需要续费沟通",
			OperationID:    1201,
			CreatedAt:      "2026-07-01 12:00:00",
		}},
		billingFollowUpSnapshots: []SaaSAdminBillingReconciliationFollowUpSnapshot{{
			OperationID:    2201,
			BillingEventID: 88,
			TenantID:       12,
			TenantName:     "高风险租户",
			Status:         SaaSAdminRiskFollowUpStatusPending,
			Owner:          "Finance-A",
			NextFollowUpAt: "2000-01-01 00:00:00",
			Remark:         "账单异常待核对",
			PackageCode:    "growth",
			PackageName:    "成长版",
			CreatedAt:      "2026-07-01 13:00:00",
		}},
		tasks: []SaaSAdminTask{{
			ID:          3301,
			TaskType:    SaaSAdminTaskTypeTenantRenewal,
			Status:      SaaSAdminTaskStatusBlocked,
			TenantID:    12,
			PackageCode: "growth",
			ActorUserID: 1,
			Remark:      "续费任务阻断",
		}},
		notifications: []SaaSAlertNotification{
			{ID: 4401, NotificationKey: "failed", TenantID: 12, Status: SaaSAlertNotificationStatusFailed},
			{ID: 4402, NotificationKey: "dead", TenantID: 12, Status: SaaSAlertNotificationStatusDead},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=customerSuccess&tenantLimit=10&limit=1000&expiringDays=10&highUsageRatio=0.8&owner=CSM-A&priority=critical", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "mochat-saas-customerSuccess") {
		t.Fatalf("headers = %+v", rec.Header())
	}
	if store.lastOptions.Scope != SaaSAdminScopePlatform ||
		store.lastOptions.Limit != 10 ||
		store.lastOptions.ExpiringDays != 10 ||
		store.lastOptions.DueState != SaaSAdminDueStateAll {
		t.Fatalf("overview options = %+v", store.lastOptions)
	}
	records := readSaaSAdminCSV(t, rec)
	if len(records) != 2 {
		t.Fatalf("records = %+v", records)
	}
	header := records[0]
	if header[0] != "tenantId" || header[2] != "priority" || header[4] != "owner" || header[24] != "billingFollowUpCount" || header[29] != "topUsageMetrics" {
		t.Fatalf("header = %+v", header)
	}
	row := records[1]
	if row[0] != "12" ||
		row[1] != "高风险租户" ||
		row[2] != SaaSAdminCustomerSuccessPriorityCritical ||
		row[4] != "CSM-A" ||
		row[5] != SaaSAdminRiskFollowUpDueStateOverdue ||
		row[8] != SaaSAdminCustomerSuccessPriorityCritical ||
		row[12] != "growth" ||
		row[20] != SaaSAdminRiskFollowUpStatusPending ||
		row[24] != "1" ||
		row[25] != "1" ||
		row[26] != "2" ||
		!strings.Contains(row[29], SaaSMetricUsers+"=12/10") {
		t.Fatalf("row = %+v", row)
	}
}

func TestSaaSAdminRiskRestrictsTenantAdminToOwnTenant(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{
			Summary: SaaSAdminSummary{TenantCount: 1},
			Tenants: []SaaSAdminTenantOverview{{
				TenantID:        10,
				TenantName:      "自租户",
				TenantStatus:    1,
				PackageCode:     "growth",
				MaxUsageMetric:  SaaSMetricUsers,
				MaxUsageCurrent: 9,
				MaxUsageLimit:   10,
				MaxUsageRatio:   0.9,
			}},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			10: {{
				Metric:     SaaSMetricUsers,
				PeriodKey:  SaaSAlertPeriodLifetime,
				Current:    9,
				Limit:      10,
				UsageRatio: 0.9,
				Status:     "warning",
			}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	crossReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/risk?tenantId=12", nil)
	crossReq.Header.Set("X-Mochat-Go-User-ID", "7")
	crossRec := httptest.NewRecorder()
	handler.Risk(crossRec, crossReq)
	if crossRec.Code != http.StatusForbidden {
		t.Fatalf("cross status = %d body=%s", crossRec.Code, crossRec.Body.String())
	}
	if store.overviewCalls != 0 || store.usageCalls != 0 {
		t.Fatalf("calls overview=%d usage=%d", store.overviewCalls, store.usageCalls)
	}

	ownReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/risk?limit=999&highUsageRatio=0.9", nil)
	ownReq.Header.Set("X-Mochat-Go-User-ID", "7")
	ownRec := httptest.NewRecorder()
	handler.Risk(ownRec, ownReq)
	if ownRec.Code != http.StatusOK {
		t.Fatalf("own status = %d body=%s", ownRec.Code, ownRec.Body.String())
	}
	if store.lastOptions.TenantID != 10 || store.lastOptions.Limit != saasAdminListMaxLimit || store.lastUsageTenantID != 10 {
		t.Fatalf("options overview=%+v usageTenant=%d", store.lastOptions, store.lastUsageTenantID)
	}
	if store.latestRiskFollowUpCalls != 0 {
		t.Fatalf("risk follow-up calls = %d", store.latestRiskFollowUpCalls)
	}
	data := decodeSaaSAdminResponse(t, ownRec)
	if data["canPlatformScope"] != false {
		t.Fatalf("data = %+v", data)
	}
	items := data["riskTenants"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["riskLevel"] != "medium" {
		t.Fatalf("items = %+v", items)
	}
	if items[0].(map[string]any)["followUp"] != nil {
		t.Fatalf("tenant risk leaked follow-up = %+v", items[0])
	}
}

func TestSaaSAdminRiskRejectsInvalidHighUsageRatio(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/risk?scope=platform&highUsageRatio=0.2", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Risk(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.overviewCalls != 0 || store.usageCalls != 0 {
		t.Fatalf("calls overview=%d usage=%d", store.overviewCalls, store.usageCalls)
	}
}

func TestSaaSAdminAlertsAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		alertPage: SaaSAlertListPage{
			Items: []SaaSAlertRecord{{
				ID:              41,
				AlertKey:        "12:users:quota_exceeded:lifetime",
				TenantID:        12,
				AlertType:       SaaSAlertTypeQuotaExceeded,
				Severity:        SaaSAlertSeverityWarning,
				Status:          SaaSAlertStatusOpen,
				Metric:          SaaSMetricUsers,
				PeriodKey:       SaaSAlertPeriodLifetime,
				CurrentValue:    8,
				LimitValue:      10,
				OccurrenceCount: 2,
				Message:         "子账号数接近上限",
				ContextJSON:     `{"source":"smoke"}`,
				LastSeenAt:      "2026-07-09 16:30:00",
			}},
			Total:     1,
			TotalPage: 1,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/alerts?tenantId=12&status=all&metric=users&alertType=quota_exceeded&page=2&perPage=5", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Alerts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.alertCalls != 1 ||
		store.lastAlertOptions.TenantID != 12 ||
		store.lastAlertOptions.Status != "" ||
		store.lastAlertOptions.Metric != SaaSMetricUsers ||
		store.lastAlertOptions.AlertType != SaaSAlertTypeQuotaExceeded ||
		store.lastAlertOptions.Page != 2 ||
		store.lastAlertOptions.PerPage != 5 {
		t.Fatalf("alert options = %+v calls=%d", store.lastAlertOptions, store.alertCalls)
	}
	if store.alertSummaryCalls != 1 || store.lastAlertSummaryOptions != store.lastAlertOptions {
		t.Fatalf("alert summary options = %+v calls=%d", store.lastAlertSummaryOptions, store.alertSummaryCalls)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["tenantId"].(float64) != 12 || data["canPlatformScope"] != true {
		t.Fatalf("data = %+v", data)
	}
	if data["returnedCount"].(float64) != 1 {
		t.Fatalf("returnedCount = %+v", data["returnedCount"])
	}
	summary := data["summary"].(map[string]any)
	if summary["alertCount"].(float64) != 1 ||
		summary["openCount"].(float64) != 1 ||
		summary["warningCount"].(float64) != 1 ||
		summary["metricCount"].(float64) != 1 ||
		summary["tenantCount"].(float64) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	page := data["page"].(map[string]any)
	if page["total"].(float64) != 1 || page["perPage"].(float64) != 5 {
		t.Fatalf("page = %+v", page)
	}
	alerts := data["alerts"].([]any)
	first := alerts[0].(map[string]any)
	if first["tenantId"].(float64) != 12 || first["metric"] != SaaSMetricUsers || first["metricLabel"] != "子账号数" {
		t.Fatalf("first alert = %+v", first)
	}
	contextPayload := first["context"].(map[string]any)
	if contextPayload["source"] != "smoke" {
		t.Fatalf("context = %+v", contextPayload)
	}
}

func TestSaaSAdminAlertsRestrictsTenantAdminToOwnTenant(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	crossReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/alerts?tenantId=12", nil)
	crossReq.Header.Set("X-Mochat-Go-User-ID", "7")
	crossRec := httptest.NewRecorder()
	handler.Alerts(crossRec, crossReq)
	if crossRec.Code != http.StatusForbidden {
		t.Fatalf("cross status = %d body=%s", crossRec.Code, crossRec.Body.String())
	}
	if store.alertCalls != 0 {
		t.Fatalf("alert calls = %d", store.alertCalls)
	}

	ownReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/alerts?status=open&perPage=999", nil)
	ownReq.Header.Set("X-Mochat-Go-User-ID", "7")
	ownRec := httptest.NewRecorder()
	handler.Alerts(ownRec, ownReq)
	if ownRec.Code != http.StatusOK {
		t.Fatalf("own status = %d body=%s", ownRec.Code, ownRec.Body.String())
	}
	if store.lastAlertOptions.TenantID != 10 || store.lastAlertOptions.Status != SaaSAlertStatusOpen || store.lastAlertOptions.PerPage != saasAdminListMaxLimit {
		t.Fatalf("alert options = %+v", store.lastAlertOptions)
	}
	data := decodeSaaSAdminResponse(t, ownRec)
	if data["canPlatformScope"] != false {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminResolveAlertAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		alertResolveResult: SaaSAdminAlertResolveResult{
			Resolved:       true,
			TenantID:       12,
			TenantName:     "租户B",
			AlertID:        41,
			AlertKey:       "12:users:quota_exceeded:lifetime",
			Metric:         SaaSMetricUsers,
			AlertType:      SaaSAlertTypeQuotaExceeded,
			PeriodKey:      SaaSAlertPeriodLifetime,
			PreviousStatus: SaaSAlertStatusOpen,
			Status:         SaaSAlertStatusResolved,
			Remark:         "已扩容",
			OperationID:    99,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/alertResolve", strings.NewReader(`{"tenantId":12,"metric":"users","remark":"已扩容"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ResolveAlert(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.alertResolveCalls != 1 ||
		store.lastAlertResolve.TenantID != 12 ||
		store.lastAlertResolve.Metric != SaaSMetricUsers ||
		store.lastAlertResolve.AlertType != SaaSAlertTypeQuotaExceeded ||
		store.lastAlertResolve.PeriodKey != SaaSAlertPeriodLifetime ||
		store.lastAlertResolve.ActorUserID != 1 ||
		store.lastAlertResolve.ActorTenantID != 1 {
		t.Fatalf("resolve = %+v calls=%d", store.lastAlertResolve, store.alertResolveCalls)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["resolved"] != true || data["operationId"].(float64) != 99 || data["metricLabel"] != "子账号数" {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminResolveAlertRestrictsTenantAdminToOwnTenant(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
		alertResolveResult: SaaSAdminAlertResolveResult{
			Resolved: true,
			TenantID: 10,
			Metric:   SaaSMetricUsers,
			Status:   SaaSAlertStatusResolved,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	crossReq := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/alertResolve", strings.NewReader(`{"tenantId":12,"metric":"users"}`))
	crossReq.Header.Set("Content-Type", "application/json")
	crossReq.Header.Set("X-Mochat-Go-User-ID", "7")
	crossRec := httptest.NewRecorder()
	handler.ResolveAlert(crossRec, crossReq)
	if crossRec.Code != http.StatusForbidden {
		t.Fatalf("cross status = %d body=%s", crossRec.Code, crossRec.Body.String())
	}
	if store.alertResolveCalls != 0 {
		t.Fatalf("resolve calls = %d", store.alertResolveCalls)
	}

	ownReq := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/alertResolve", strings.NewReader(`{"metric":"users"}`))
	ownReq.Header.Set("Content-Type", "application/json")
	ownReq.Header.Set("X-Mochat-Go-User-ID", "7")
	ownRec := httptest.NewRecorder()
	handler.ResolveAlert(ownRec, ownReq)
	if ownRec.Code != http.StatusOK {
		t.Fatalf("own status = %d body=%s", ownRec.Code, ownRec.Body.String())
	}
	if store.lastAlertResolve.TenantID != 10 || store.lastAlertResolve.ActorUserID != 7 || store.lastAlertResolve.ActorTenantID != 10 {
		t.Fatalf("resolve = %+v", store.lastAlertResolve)
	}
}

func TestSaaSAdminResolveAlertRejectsMissingMetric(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/alertResolve", strings.NewReader(`{"tenantId":12}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ResolveAlert(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.alertResolveCalls != 0 {
		t.Fatalf("resolve calls = %d", store.alertResolveCalls)
	}
}

func TestSaaSAdminBulkResolveAlertsAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		alertBulkResult: SaaSAdminAlertBulkResolveResult{
			ResolvedCount: 2,
			TenantID:      12,
			Metric:        SaaSMetricUsers,
			AlertType:     SaaSAlertTypeQuotaExceeded,
			Limit:         5,
			Remark:        "批量解决",
			Alerts: []SaaSAdminAlertResolveResult{
				{Resolved: true, TenantID: 12, AlertID: 41, Metric: SaaSMetricUsers, AlertType: SaaSAlertTypeQuotaExceeded, PreviousStatus: SaaSAlertStatusOpen, Status: SaaSAlertStatusResolved, OperationID: 91},
				{Resolved: true, TenantID: 12, AlertID: 42, Metric: SaaSMetricUsers, AlertType: SaaSAlertTypeQuotaExceeded, PreviousStatus: SaaSAlertStatusOpen, Status: SaaSAlertStatusResolved, OperationID: 92},
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/alertBulkResolve", strings.NewReader(`{"tenantId":12,"metric":"users","alertType":"quota_exceeded","limit":5,"remark":"批量解决"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.BulkResolveAlerts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.alertBulkCalls != 1 ||
		store.lastAlertBulk.TenantID != 12 ||
		store.lastAlertBulk.AllowedTenantID != 0 ||
		store.lastAlertBulk.Metric != SaaSMetricUsers ||
		store.lastAlertBulk.AlertType != SaaSAlertTypeQuotaExceeded ||
		store.lastAlertBulk.Limit != 5 ||
		store.lastAlertBulk.ActorUserID != 1 ||
		store.lastAlertBulk.ActorTenantID != 1 ||
		store.lastAlertBulk.Remark != "批量解决" {
		t.Fatalf("bulk = %+v calls=%d", store.lastAlertBulk, store.alertBulkCalls)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["resolved"] != true || data["resolvedCount"].(float64) != 2 {
		t.Fatalf("data = %+v", data)
	}
	items := data["alerts"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["status"] != SaaSAlertStatusResolved {
		t.Fatalf("items = %+v", items)
	}
}

func TestSaaSAdminBulkResolveAlertsRestrictsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	crossReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/alertBulkResolve", strings.NewReader(`{"tenantId":12,"metric":"users"}`))
	crossReq.Header.Set("Content-Type", "application/json")
	crossReq.Header.Set("X-Mochat-Go-User-ID", "7")
	crossRec := httptest.NewRecorder()
	handler.BulkResolveAlerts(crossRec, crossReq)
	if crossRec.Code != http.StatusForbidden {
		t.Fatalf("cross status = %d body=%s", crossRec.Code, crossRec.Body.String())
	}
	if store.alertBulkCalls != 0 {
		t.Fatalf("bulk calls = %d", store.alertBulkCalls)
	}

	ownReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/alertBulkResolve", strings.NewReader(`{"metric":"users","limit":999,"remark":"自租户批量解决"}`))
	ownReq.Header.Set("Content-Type", "application/json")
	ownReq.Header.Set("X-Mochat-Go-User-ID", "7")
	ownRec := httptest.NewRecorder()
	handler.BulkResolveAlerts(ownRec, ownReq)
	if ownRec.Code != http.StatusOK {
		t.Fatalf("own status = %d body=%s", ownRec.Code, ownRec.Body.String())
	}
	if store.lastAlertBulk.TenantID != 10 ||
		store.lastAlertBulk.AllowedTenantID != 10 ||
		store.lastAlertBulk.Metric != SaaSMetricUsers ||
		store.lastAlertBulk.Limit != saasAdminListMaxLimit ||
		store.lastAlertBulk.ActorUserID != 7 {
		t.Fatalf("bulk = %+v", store.lastAlertBulk)
	}
}

func TestSaaSAdminBulkResolveAlertsRejectsTooLongMetric(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/alertBulkResolve", strings.NewReader(`{"metric":"`+strings.Repeat("x", 65)+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.BulkResolveAlerts(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.alertBulkCalls != 0 {
		t.Fatalf("bulk calls = %d", store.alertBulkCalls)
	}
}

func TestSaaSAdminNotificationsAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		notifications: []SaaSAlertNotification{{
			ID:              77,
			NotificationKey: "12:users:quota_exceeded:lifetime:webhook",
			AlertKey:        "12:users:quota_exceeded:lifetime",
			TenantID:        12,
			Channel:         SaaSAlertNotificationChannelWebhook,
			Status:          SaaSAlertNotificationStatusFailed,
			Attempts:        2,
			MaxAttempts:     3,
			Alert: SaaSQuotaAlert{
				Status: SaaSQuotaStatus{
					TenantID: 12,
					Metric:   SaaSMetricUsers,
					Current:  8,
					Limit:    10,
				},
				AlertType: SaaSAlertTypeQuotaExceeded,
				PeriodKey: SaaSAlertPeriodLifetime,
				Message:   "子账号数接近上限",
			},
			LastError:   "webhook 500",
			NextRetryAt: "2026-07-09 16:50:00",
			CreatedAt:   "2026-07-09 16:40:00",
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/notifications?tenantId=12&status=failed&channel=webhook&keyword=users&limit=5", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Notifications(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationCalls != 1 ||
		store.lastNotificationOptions.TenantID != 12 ||
		store.lastNotificationOptions.Status != SaaSAlertNotificationStatusFailed ||
		store.lastNotificationOptions.Channel != SaaSAlertNotificationChannelWebhook ||
		store.lastNotificationOptions.Keyword != "users" ||
		store.lastNotificationOptions.Limit != 5 {
		t.Fatalf("notification options = %+v calls=%d", store.lastNotificationOptions, store.notificationCalls)
	}
	if store.notificationSummaryCalls != 1 || store.lastNotificationSummaryOptions != store.lastNotificationOptions {
		t.Fatalf("notification summary options = %+v calls=%d", store.lastNotificationSummaryOptions, store.notificationSummaryCalls)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["tenantId"].(float64) != 12 || data["canPlatformScope"] != true {
		t.Fatalf("data = %+v", data)
	}
	if data["returnedCount"].(float64) != 1 {
		t.Fatalf("returnedCount = %+v", data["returnedCount"])
	}
	summary := data["summary"].(map[string]any)
	if summary["notificationCount"].(float64) != 1 ||
		summary["failedCount"].(float64) != 1 ||
		summary["retryableCount"].(float64) != 1 ||
		summary["tenantCount"].(float64) != 1 ||
		summary["channelCount"].(float64) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	items := data["notifications"].([]any)
	if len(items) != 1 {
		t.Fatalf("notifications = %+v", items)
	}
	first := items[0].(map[string]any)
	if first["id"].(float64) != 77 ||
		first["metric"] != SaaSMetricUsers ||
		first["metricLabel"] != "子账号数" ||
		first["status"] != SaaSAlertNotificationStatusFailed ||
		first["attempts"].(float64) != 2 ||
		first["lastError"] != "webhook 500" {
		t.Fatalf("first = %+v", first)
	}
}

func TestSaaSAdminNotificationsRestrictsTenantAdminToOwnTenant(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	crossReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/notifications?tenantId=12", nil)
	crossReq.Header.Set("X-Mochat-Go-User-ID", "7")
	crossRec := httptest.NewRecorder()
	handler.Notifications(crossRec, crossReq)
	if crossRec.Code != http.StatusForbidden {
		t.Fatalf("cross status = %d body=%s", crossRec.Code, crossRec.Body.String())
	}
	if store.notificationCalls != 0 {
		t.Fatalf("notification calls = %d", store.notificationCalls)
	}

	ownReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/notifications?limit=999", nil)
	ownReq.Header.Set("X-Mochat-Go-User-ID", "7")
	ownRec := httptest.NewRecorder()
	handler.Notifications(ownRec, ownReq)
	if ownRec.Code != http.StatusOK {
		t.Fatalf("own status = %d body=%s", ownRec.Code, ownRec.Body.String())
	}
	if store.lastNotificationOptions.TenantID != 10 || store.lastNotificationOptions.Limit != saasAdminListMaxLimit {
		t.Fatalf("notification options = %+v", store.lastNotificationOptions)
	}
}

func TestSaaSAdminRetryNotificationPassesActorAndAllowedTenant(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
		notificationRetryResult: SaaSAdminAlertNotificationRetryResult{
			Retried:         true,
			NotificationID:  77,
			NotificationKey: "10:users:quota_exceeded:lifetime:webhook",
			TenantID:        10,
			AlertKey:        "10:users:quota_exceeded:lifetime",
			Channel:         SaaSAlertNotificationChannelWebhook,
			PreviousStatus:  SaaSAlertNotificationStatusDead,
			Status:          SaaSAlertNotificationStatusPending,
			MaxAttempts:     3,
			OperationID:     88,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationRetry", strings.NewReader(`{"notificationId":77,"remark":"人工重试"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.RetryNotification(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationRetryCalls != 1 ||
		store.lastNotificationRetry.NotificationID != 77 ||
		store.lastNotificationRetry.AllowedTenantID != 10 ||
		store.lastNotificationRetry.ActorUserID != 7 ||
		store.lastNotificationRetry.ActorTenantID != 10 ||
		store.lastNotificationRetry.Remark != "人工重试" {
		t.Fatalf("retry = %+v calls=%d", store.lastNotificationRetry, store.notificationRetryCalls)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["retried"] != true || data["status"] != SaaSAlertNotificationStatusPending || data["operationId"].(float64) != 88 {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminRetryNotificationRejectsMissingID(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationRetry", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RetryNotification(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationRetryCalls != 0 {
		t.Fatalf("retry calls = %d", store.notificationRetryCalls)
	}
}

func TestSaaSAdminBulkRetryNotificationsAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		notificationBulkResult: SaaSAdminAlertNotificationBulkRetryResult{
			RetriedCount: 2,
			TenantID:     12,
			Status:       SaaSAlertNotificationStatusFailed,
			Channel:      SaaSAlertNotificationChannelWebhook,
			Keyword:      "bulk",
			Limit:        5,
			Remark:       "批量重试",
			Notifications: []SaaSAdminAlertNotificationRetryResult{
				{Retried: true, NotificationID: 77, TenantID: 12, PreviousStatus: SaaSAlertNotificationStatusFailed, Status: SaaSAlertNotificationStatusPending, OperationID: 88},
				{Retried: true, NotificationID: 78, TenantID: 12, PreviousStatus: SaaSAlertNotificationStatusDead, Status: SaaSAlertNotificationStatusPending, OperationID: 89},
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationBulkRetry", strings.NewReader(`{"tenantId":12,"status":"failed","channel":"webhook","keyword":"bulk","limit":5,"remark":"批量重试"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.BulkRetryNotifications(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationBulkCalls != 1 ||
		store.lastNotificationBulk.TenantID != 12 ||
		store.lastNotificationBulk.AllowedTenantID != 0 ||
		store.lastNotificationBulk.Status != SaaSAlertNotificationStatusFailed ||
		store.lastNotificationBulk.Channel != SaaSAlertNotificationChannelWebhook ||
		store.lastNotificationBulk.Keyword != "bulk" ||
		store.lastNotificationBulk.Limit != 5 ||
		store.lastNotificationBulk.ActorUserID != 1 ||
		store.lastNotificationBulk.ActorTenantID != 1 ||
		store.lastNotificationBulk.Remark != "批量重试" {
		t.Fatalf("bulk = %+v calls=%d", store.lastNotificationBulk, store.notificationBulkCalls)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["retried"] != true || data["retriedCount"].(float64) != 2 {
		t.Fatalf("data = %+v", data)
	}
	items := data["notifications"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["status"] != SaaSAlertNotificationStatusPending {
		t.Fatalf("items = %+v", items)
	}
}

func TestSaaSAdminBulkRetryNotificationsRestrictsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	crossReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationBulkRetry", strings.NewReader(`{"tenantId":12,"status":"failed"}`))
	crossReq.Header.Set("Content-Type", "application/json")
	crossReq.Header.Set("X-Mochat-Go-User-ID", "7")
	crossRec := httptest.NewRecorder()
	handler.BulkRetryNotifications(crossRec, crossReq)
	if crossRec.Code != http.StatusForbidden {
		t.Fatalf("cross status = %d body=%s", crossRec.Code, crossRec.Body.String())
	}
	if store.notificationBulkCalls != 0 {
		t.Fatalf("bulk calls = %d", store.notificationBulkCalls)
	}

	ownReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationBulkRetry", strings.NewReader(`{"status":"dead","limit":999,"remark":"自租户批量重试"}`))
	ownReq.Header.Set("Content-Type", "application/json")
	ownReq.Header.Set("X-Mochat-Go-User-ID", "7")
	ownRec := httptest.NewRecorder()
	handler.BulkRetryNotifications(ownRec, ownReq)
	if ownRec.Code != http.StatusOK {
		t.Fatalf("own status = %d body=%s", ownRec.Code, ownRec.Body.String())
	}
	if store.lastNotificationBulk.TenantID != 10 ||
		store.lastNotificationBulk.AllowedTenantID != 10 ||
		store.lastNotificationBulk.Status != SaaSAlertNotificationStatusDead ||
		store.lastNotificationBulk.Limit != saasAdminListMaxLimit ||
		store.lastNotificationBulk.ActorUserID != 7 {
		t.Fatalf("bulk = %+v", store.lastNotificationBulk)
	}
}

func TestSaaSAdminBulkRetryNotificationsRejectsNonRetryableStatus(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationBulkRetry", strings.NewReader(`{"status":"pending"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.BulkRetryNotifications(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationBulkCalls != 0 {
		t.Fatalf("bulk calls = %d", store.notificationBulkCalls)
	}
}

func TestSaaSAdminCloseNotificationPassesActorAndAllowedTenant(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
		notificationCloseResult: SaaSAdminAlertNotificationCloseResult{
			Closed:          true,
			NotificationID:  77,
			NotificationKey: "smoke-close",
			TenantID:        10,
			PreviousStatus:  SaaSAlertNotificationStatusPending,
			Status:          SaaSAlertNotificationStatusClosed,
			Remark:          "关闭待发送",
			OperationID:     88,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationClose", strings.NewReader(`{"notificationId":77,"remark":"关闭待发送"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.CloseNotification(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationCloseCalls != 1 ||
		store.lastNotificationClose.NotificationID != 77 ||
		store.lastNotificationClose.AllowedTenantID != 10 ||
		store.lastNotificationClose.ActorUserID != 7 ||
		store.lastNotificationClose.ActorTenantID != 10 ||
		store.lastNotificationClose.Remark != "关闭待发送" {
		t.Fatalf("close = %+v calls=%d", store.lastNotificationClose, store.notificationCloseCalls)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["closed"] != true || data["status"] != SaaSAlertNotificationStatusClosed || data["previousStatus"] != SaaSAlertNotificationStatusPending || data["operationId"].(float64) != 88 {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminBulkCloseNotificationsAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		notificationBulkCloseResult: SaaSAdminAlertNotificationBulkCloseResult{
			ClosedCount: 2,
			TenantID:    12,
			Status:      SaaSAlertNotificationStatusFailed,
			Channel:     SaaSAlertNotificationChannelWebhook,
			Keyword:     "bulk",
			Limit:       5,
			Remark:      "批量关闭",
			Notifications: []SaaSAdminAlertNotificationCloseResult{
				{Closed: true, NotificationID: 77, TenantID: 12, PreviousStatus: SaaSAlertNotificationStatusFailed, Status: SaaSAlertNotificationStatusClosed, OperationID: 88},
				{Closed: true, NotificationID: 78, TenantID: 12, PreviousStatus: SaaSAlertNotificationStatusPending, Status: SaaSAlertNotificationStatusClosed, OperationID: 89},
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationBulkClose", strings.NewReader(`{"tenantId":12,"status":"failed","channel":"webhook","keyword":"bulk","limit":5,"remark":"批量关闭"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.BulkCloseNotifications(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationBulkCloseCalls != 1 ||
		store.lastNotificationBulkClose.TenantID != 12 ||
		store.lastNotificationBulkClose.AllowedTenantID != 0 ||
		store.lastNotificationBulkClose.Status != SaaSAlertNotificationStatusFailed ||
		store.lastNotificationBulkClose.Channel != SaaSAlertNotificationChannelWebhook ||
		store.lastNotificationBulkClose.Keyword != "bulk" ||
		store.lastNotificationBulkClose.Limit != 5 ||
		store.lastNotificationBulkClose.ActorUserID != 1 ||
		store.lastNotificationBulkClose.ActorTenantID != 1 ||
		store.lastNotificationBulkClose.Remark != "批量关闭" {
		t.Fatalf("bulk close = %+v calls=%d", store.lastNotificationBulkClose, store.notificationBulkCloseCalls)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["closed"] != true || data["closedCount"].(float64) != 2 {
		t.Fatalf("data = %+v", data)
	}
	items := data["notifications"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["status"] != SaaSAlertNotificationStatusClosed {
		t.Fatalf("items = %+v", items)
	}
}

func TestSaaSAdminBulkCloseNotificationsRestrictsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	crossReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationBulkClose", strings.NewReader(`{"tenantId":12,"status":"failed"}`))
	crossReq.Header.Set("Content-Type", "application/json")
	crossReq.Header.Set("X-Mochat-Go-User-ID", "7")
	crossRec := httptest.NewRecorder()
	handler.BulkCloseNotifications(crossRec, crossReq)
	if crossRec.Code != http.StatusForbidden {
		t.Fatalf("cross status = %d body=%s", crossRec.Code, crossRec.Body.String())
	}
	if store.notificationBulkCloseCalls != 0 {
		t.Fatalf("bulk close calls = %d", store.notificationBulkCloseCalls)
	}

	ownReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationBulkClose", strings.NewReader(`{"status":"pending","limit":999}`))
	ownReq.Header.Set("Content-Type", "application/json")
	ownReq.Header.Set("X-Mochat-Go-User-ID", "7")
	ownRec := httptest.NewRecorder()
	handler.BulkCloseNotifications(ownRec, ownReq)
	if ownRec.Code != http.StatusOK {
		t.Fatalf("own status = %d body=%s", ownRec.Code, ownRec.Body.String())
	}
	if store.lastNotificationBulkClose.TenantID != 10 ||
		store.lastNotificationBulkClose.AllowedTenantID != 10 ||
		store.lastNotificationBulkClose.Status != SaaSAlertNotificationStatusPending ||
		store.lastNotificationBulkClose.Limit != saasAdminListMaxLimit ||
		store.lastNotificationBulkClose.ActorUserID != 7 ||
		store.lastNotificationBulkClose.Remark != "关闭通知" {
		t.Fatalf("bulk close = %+v", store.lastNotificationBulkClose)
	}
}

func TestSaaSAdminBulkCloseNotificationsRejectsNonClosableStatus(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationBulkClose", strings.NewReader(`{"status":"dead"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.BulkCloseNotifications(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationBulkCloseCalls != 0 {
		t.Fatalf("bulk close calls = %d", store.notificationBulkCloseCalls)
	}
}

func TestSaaSAdminUpsertPackageAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		packages: []SaaSAdminPackage{{
			Code:   "scale",
			Name:   "规模版",
			Status: 1,
			Limits: SaaSAdminPackageLimits{
				MaxUsers:        300,
				ChannelCodes:    33,
				AsyncExecutions: 1000,
			},
		}},
		overview: SaaSAdminOverview{
			Tenants: []SaaSAdminTenantOverview{{
				TenantID:    12,
				TenantName:  "使用规模版租户",
				PackageCode: "scale",
			}},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{
				Metric:  SaaSMetricUsers,
				Current: 200,
				Limit:   300,
			}},
		},
		upsertPackageResult: SaaSAdminPackage{
			Code:   "scale",
			Name:   "规模版",
			Status: 1,
			Limits: SaaSAdminPackageLimits{
				MaxUsers:        120,
				ChannelCodes:    33,
				AsyncExecutions: 1000,
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/package", strings.NewReader(`{"code":"scale","name":"规模版","status":1,"limits":{"maxUsers":120,"channelCodes":33,"asyncExecutions":1000}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.UpsertPackage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.upsertPackageCalls != 1 {
		t.Fatalf("upsert calls = %d", store.upsertPackageCalls)
	}
	if store.packageCalls != 1 || store.overviewCalls != 1 || store.usageCalls != 1 {
		t.Fatalf("impact calls packages=%d overview=%d usage=%d", store.packageCalls, store.overviewCalls, store.usageCalls)
	}
	if store.lastOptions.PackageCode != "scale" || store.lastOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("impact overview options = %+v", store.lastOptions)
	}
	if store.lastUsageTenantID != 12 {
		t.Fatalf("last usage tenant = %d", store.lastUsageTenantID)
	}
	if store.lastPackageUpsert.Code != "scale" || store.lastPackageUpsert.Name != "规模版" || store.lastPackageUpsert.Status != 1 || store.lastPackageUpsert.Limits.MaxUsers != 120 {
		t.Fatalf("last package upsert = %+v", store.lastPackageUpsert)
	}
	data := decodeSaaSAdminResponse(t, rec)
	limits := data["limits"].(map[string]any)
	if data["code"] != "scale" || limits["maxUsers"].(float64) != 120 {
		t.Fatalf("data = %+v", data)
	}
	impact := data["impact"].(map[string]any)
	if impact["existing"] != true ||
		impact["assignedTenantCount"].(float64) != 1 ||
		impact["checkedTenantCount"].(float64) != 1 ||
		impact["changedLimitCount"].(float64) != 1 ||
		impact["decreasedLimitCount"].(float64) != 1 ||
		impact["overLimitTenantCount"].(float64) != 1 ||
		impact["tenantSnapshotsUpdated"] != false {
		t.Fatalf("impact = %+v", impact)
	}
	changes := impact["changes"].([]any)
	if len(changes) != 1 {
		t.Fatalf("changes = %+v", changes)
	}
	change := changes[0].(map[string]any)
	if change["field"] != "maxUsers" || change["metric"] != SaaSMetricUsers || change["direction"] != "decrease" || change["before"].(float64) != 300 || change["after"].(float64) != 120 {
		t.Fatalf("change = %+v", change)
	}
	overLimitTenants := impact["overLimitTenants"].([]any)
	if len(overLimitTenants) != 1 {
		t.Fatalf("overLimitTenants = %+v", overLimitTenants)
	}
	overLimitTenant := overLimitTenants[0].(map[string]any)
	if overLimitTenant["tenantId"].(float64) != 12 || overLimitTenant["metric"] != SaaSMetricUsers || overLimitTenant["current"].(float64) != 200 || overLimitTenant["limit"].(float64) != 120 {
		t.Fatalf("overLimitTenant = %+v", overLimitTenant)
	}
}

func TestSaaSAdminUpsertPackageRejectsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/package", strings.NewReader(`{"code":"scale","name":"规模版"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.UpsertPackage(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.upsertPackageCalls != 0 {
		t.Fatalf("upsert calls = %d", store.upsertPackageCalls)
	}
}

func TestSaaSAdminUpsertPackageRejectsInvalidLimits(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/package", strings.NewReader(`{"code":"bad_code","name":"坏套餐","limits":{"maxUsers":-1}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.UpsertPackage(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.upsertPackageCalls != 0 {
		t.Fatalf("upsert calls = %d", store.upsertPackageCalls)
	}
}

func TestSaaSAdminUpdateTenantStatusAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		statusResult: SaaSAdminTenantStatusUpdateResult{
			TenantID:       12,
			TenantName:     "租户B",
			PreviousStatus: 1,
			Status:         2,
			Remark:         "欠费停用",
			OperationID:    88,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantStatus", strings.NewReader(`{"tenantId":12,"status":2,"remark":"欠费停用"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.UpdateTenantStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.statusCalls != 1 {
		t.Fatalf("status calls = %d", store.statusCalls)
	}
	if store.lastStatusUpdate.TenantID != 12 || store.lastStatusUpdate.Status != 2 || store.lastStatusUpdate.Remark != "欠费停用" || store.lastStatusUpdate.ActorUserID != 1 {
		t.Fatalf("last status update = %+v", store.lastStatusUpdate)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["previousStatus"].(float64) != 1 || data["status"].(float64) != 2 || data["operationId"].(float64) != 88 {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminUpdateTenantStatusRejectsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantStatus", strings.NewReader(`{"tenantId":12,"status":2}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.UpdateTenantStatus(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.statusCalls != 0 {
		t.Fatalf("status calls = %d", store.statusCalls)
	}
}

func TestSaaSAdminUpdateTenantStatusRejectsPlatformDisable(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantStatus", strings.NewReader(`{"tenantId":1,"status":2}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.UpdateTenantStatus(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.statusCalls != 0 {
		t.Fatalf("status calls = %d", store.statusCalls)
	}
}

func TestSaaSAdminRiskFollowUpAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		riskFollowUpResult: SaaSAdminRiskFollowUpResult{
			TenantID:       12,
			TenantName:     "租户B",
			Status:         SaaSAdminRiskFollowUpStatusRenewalPending,
			Owner:          "CSM-A",
			NextFollowUpAt: "2026-07-20 00:00:00",
			Remark:         "推进续费",
			OperationID:    128,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/riskFollowUp", strings.NewReader(`{"tenantId":12,"status":"renewal_pending","owner":"CSM-A","nextFollowUpAt":"2026-07-20","remark":"推进续费"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RiskFollowUp(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.riskFollowUpCalls != 1 {
		t.Fatalf("risk follow-up calls = %d", store.riskFollowUpCalls)
	}
	got := store.lastRiskFollowUp
	if got.TenantID != 12 || got.Status != SaaSAdminRiskFollowUpStatusRenewalPending || got.Owner != "CSM-A" || got.NextFollowUpAt != "2026-07-20 00:00:00" || got.Remark != "推进续费" {
		t.Fatalf("last risk follow-up = %+v", got)
	}
	if got.ActorUserID != 1 || got.ActorTenantID != 1 {
		t.Fatalf("actor = %+v", got)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["operationId"].(float64) != 128 || data["status"] != SaaSAdminRiskFollowUpStatusRenewalPending || data["owner"] != "CSM-A" {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminRiskFollowUpRejectsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/riskFollowUp", strings.NewReader(`{"tenantId":10,"status":"contacted"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.RiskFollowUp(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.riskFollowUpCalls != 0 {
		t.Fatalf("risk follow-up calls = %d", store.riskFollowUpCalls)
	}
}

func TestSaaSAdminRiskFollowUpRejectsInvalidStatus(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/riskFollowUp", strings.NewReader(`{"tenantId":12,"status":"done"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RiskFollowUp(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.riskFollowUpCalls != 0 {
		t.Fatalf("risk follow-up calls = %d", store.riskFollowUpCalls)
	}
}

func TestSaaSAdminRiskFollowUpsAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		riskFollowUpSnapshots: []SaaSAdminRiskFollowUpSnapshot{
			{
				TenantID:       12,
				TenantName:     "风险租户",
				Status:         SaaSAdminRiskFollowUpStatusContacted,
				Owner:          "smoke-csm",
				NextFollowUpAt: "2000-01-01 00:00:00",
				Remark:         "renewal smoke-risk-follow-up",
				OperationID:    301,
				CreatedAt:      "2026-07-09 10:00:00",
			},
			{
				TenantID:       13,
				TenantName:     "已关闭租户",
				Status:         SaaSAdminRiskFollowUpStatusResolved,
				Owner:          "other",
				NextFollowUpAt: "2026-07-20 00:00:00",
				Remark:         "closed",
				OperationID:    302,
				CreatedAt:      "2026-07-09 09:00:00",
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/riskFollowUps?tenantId=12&status=contacted&owner=smoke&keyword=renewal&dueState=overdue&limit=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RiskFollowUps(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.riskFollowUpSnapshotCalls != 1 {
		t.Fatalf("risk follow-up snapshot calls = %d", store.riskFollowUpSnapshotCalls)
	}
	options := store.lastRiskFollowUpTaskOptions
	if options.TenantID != 12 ||
		options.Status != SaaSAdminRiskFollowUpStatusContacted ||
		options.Owner != "smoke" ||
		options.Keyword != "renewal" ||
		options.DueState != SaaSAdminRiskFollowUpDueStateAll ||
		options.Limit != saasAdminExportMaxLimit {
		t.Fatalf("store options = %+v", options)
	}
	data := decodeSaaSAdminResponse(t, rec)
	filters := data["filters"].(map[string]any)
	if filters["dueState"] != SaaSAdminRiskFollowUpDueStateOverdue || filters["limit"].(float64) != 10 {
		t.Fatalf("filters = %+v", filters)
	}
	summary := data["summary"].(map[string]any)
	if summary["totalCount"].(float64) != 1 ||
		summary["contactedCount"].(float64) != 1 ||
		summary["overdueCount"].(float64) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	items := data["followUps"].([]any)
	if len(items) != 1 {
		t.Fatalf("followUps = %+v", items)
	}
	item := items[0].(map[string]any)
	if item["tenantId"].(float64) != 12 ||
		item["status"] != SaaSAdminRiskFollowUpStatusContacted ||
		item["owner"] != "smoke-csm" ||
		item["dueState"] != SaaSAdminRiskFollowUpDueStateOverdue ||
		item["overdue"] != true ||
		item["operationId"].(float64) != 301 {
		t.Fatalf("item = %+v", item)
	}
	if item["daysUntil"].(float64) >= 0 {
		t.Fatalf("daysUntil = %+v", item["daysUntil"])
	}
}

func TestSaaSAdminRiskFollowUpsRejectsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/riskFollowUps", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.RiskFollowUps(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.riskFollowUpSnapshotCalls != 0 {
		t.Fatalf("risk follow-up snapshot calls = %d", store.riskFollowUpSnapshotCalls)
	}
}

func TestSaaSAdminRiskFollowUpsRejectsInvalidFilters(t *testing.T) {
	for _, path := range []string{
		"/dashboard/saasAdmin/riskFollowUps?status=done",
		"/dashboard/saasAdmin/riskFollowUps?dueState=bad",
	} {
		store := &fakeSaaSAdminStore{
			users: map[int]User{
				1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			},
		}
		handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Mochat-Go-User-ID", "1")
		rec := httptest.NewRecorder()

		handler.RiskFollowUps(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d body=%s", path, rec.Code, rec.Body.String())
		}
		if store.riskFollowUpSnapshotCalls != 0 {
			t.Fatalf("%s risk follow-up snapshot calls = %d", path, store.riskFollowUpSnapshotCalls)
		}
	}
}

func TestSaaSAdminRiskFollowUpOwnersAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		riskFollowUpSnapshots: []SaaSAdminRiskFollowUpSnapshot{
			{
				TenantID:       12,
				TenantName:     "风险租户A",
				Status:         SaaSAdminRiskFollowUpStatusContacted,
				Owner:          "CSM-A",
				NextFollowUpAt: "2000-01-01 00:00:00",
				Remark:         "overdue",
				OperationID:    301,
				CreatedAt:      "2026-07-09 10:00:00",
			},
			{
				TenantID:       13,
				TenantName:     "风险租户B",
				Status:         SaaSAdminRiskFollowUpStatusRenewalPending,
				Owner:          "CSM-A",
				NextFollowUpAt: "2099-01-01 00:00:00",
				Remark:         "future",
				OperationID:    302,
				CreatedAt:      "2026-07-09 09:00:00",
			},
			{
				TenantID:       14,
				TenantName:     "未分配租户",
				Status:         SaaSAdminRiskFollowUpStatusPending,
				Owner:          "",
				NextFollowUpAt: "",
				Remark:         "no owner",
				OperationID:    303,
				CreatedAt:      "2026-07-09 08:00:00",
			},
			{
				TenantID:       15,
				TenantName:     "已关闭租户",
				Status:         SaaSAdminRiskFollowUpStatusResolved,
				Owner:          "CSM-B",
				NextFollowUpAt: "2000-01-02 00:00:00",
				Remark:         "closed",
				OperationID:    304,
				CreatedAt:      "2026-07-09 07:00:00",
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/riskFollowUpOwners?limit=1000", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RiskFollowUpOwners(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.riskFollowUpSnapshotCalls != 1 {
		t.Fatalf("risk follow-up snapshot calls = %d", store.riskFollowUpSnapshotCalls)
	}
	options := store.lastRiskFollowUpTaskOptions
	if options.DueState != SaaSAdminRiskFollowUpDueStateAll || options.Limit != saasAdminExportMaxLimit {
		t.Fatalf("store options = %+v", options)
	}
	data := decodeSaaSAdminResponse(t, rec)
	summary := data["summary"].(map[string]any)
	if summary["totalCount"].(float64) != 4 || summary["closedCount"].(float64) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	owners := data["owners"].([]any)
	if len(owners) != 3 {
		t.Fatalf("owners = %+v", owners)
	}
	first := owners[0].(map[string]any)
	if first["owner"] != "CSM-A" ||
		first["totalCount"].(float64) != 2 ||
		first["openCount"].(float64) != 2 ||
		first["contactedCount"].(float64) != 1 ||
		first["renewalPendingCount"].(float64) != 1 ||
		first["overdueCount"].(float64) != 1 ||
		first["futureCount"].(float64) != 1 ||
		first["latestFollowUpAt"] != "2026-07-09 10:00:00" ||
		first["nextFollowUpAt"] != "2000-01-01 00:00:00" {
		t.Fatalf("first owner = %+v", first)
	}
	unassigned := owners[1].(map[string]any)
	if unassigned["owner"] != "未分配" || unassigned["pendingCount"].(float64) != 1 || unassigned["noDateCount"].(float64) != 1 {
		t.Fatalf("unassigned = %+v", unassigned)
	}
	closed := owners[2].(map[string]any)
	if closed["owner"] != "CSM-B" || closed["openCount"].(float64) != 0 || closed["closedCount"].(float64) != 1 {
		t.Fatalf("closed = %+v", closed)
	}
}

func TestSaaSAdminRiskFollowUpOwnersRejectsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/riskFollowUpOwners", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.RiskFollowUpOwners(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.riskFollowUpSnapshotCalls != 0 {
		t.Fatalf("risk follow-up snapshot calls = %d", store.riskFollowUpSnapshotCalls)
	}
}

func TestSaaSAdminRiskFollowUpBulkCloseAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		riskFollowUpSnapshots: []SaaSAdminRiskFollowUpSnapshot{
			{
				TenantID:       12,
				TenantName:     "风险租户A",
				Status:         SaaSAdminRiskFollowUpStatusContacted,
				Owner:          "smoke-csm",
				NextFollowUpAt: "2000-01-01 00:00:00",
				Remark:         "renewal smoke-risk-follow-up",
				OperationID:    301,
				CreatedAt:      "2026-07-09 10:00:00",
			},
			{
				TenantID:       13,
				TenantName:     "风险租户B",
				Status:         SaaSAdminRiskFollowUpStatusContacted,
				Owner:          "smoke-csm",
				NextFollowUpAt: "2000-01-02 00:00:00",
				Remark:         "renewal smoke-risk-follow-up",
				OperationID:    302,
				CreatedAt:      "2026-07-09 09:00:00",
			},
			{
				TenantID:       14,
				TenantName:     "已关闭租户",
				Status:         SaaSAdminRiskFollowUpStatusResolved,
				Owner:          "smoke-csm",
				NextFollowUpAt: "2000-01-03 00:00:00",
				Remark:         "renewal smoke-risk-follow-up",
				OperationID:    303,
				CreatedAt:      "2026-07-09 08:00:00",
			},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/riskFollowUpBulkClose", strings.NewReader(`{"filterStatus":"contacted","owner":"smoke","keyword":"renewal","dueState":"overdue","limit":10,"closeStatus":"resolved","remark":"批量已处理"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RiskFollowUpBulkClose(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.riskFollowUpSnapshotCalls != 1 {
		t.Fatalf("risk follow-up snapshot calls = %d", store.riskFollowUpSnapshotCalls)
	}
	options := store.lastRiskFollowUpTaskOptions
	if options.Status != SaaSAdminRiskFollowUpStatusContacted ||
		options.Owner != "smoke" ||
		options.Keyword != "renewal" ||
		options.DueState != SaaSAdminRiskFollowUpDueStateAll ||
		options.Limit != saasAdminExportMaxLimit {
		t.Fatalf("store options = %+v", options)
	}
	if store.riskFollowUpCalls != 2 || len(store.riskFollowUps) != 2 {
		t.Fatalf("risk follow-up calls = %d followUps=%+v", store.riskFollowUpCalls, store.riskFollowUps)
	}
	for _, followUp := range store.riskFollowUps {
		if followUp.Status != SaaSAdminRiskFollowUpStatusResolved ||
			followUp.Owner != "smoke-csm" ||
			followUp.NextFollowUpAt != "" ||
			followUp.Remark != "批量已处理" ||
			followUp.ActorUserID != 1 ||
			followUp.ActorTenantID != 1 {
			t.Fatalf("followUp = %+v", followUp)
		}
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["closedCount"].(float64) != 2 || data["status"] != SaaSAdminRiskFollowUpStatusResolved || data["remark"] != "批量已处理" {
		t.Fatalf("data = %+v", data)
	}
	items := data["followUps"].([]any)
	if len(items) != 2 {
		t.Fatalf("followUps payload = %+v", items)
	}
}

func TestSaaSAdminRiskFollowUpBulkCloseRejectsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/riskFollowUpBulkClose", strings.NewReader(`{"closeStatus":"resolved"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.RiskFollowUpBulkClose(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.riskFollowUpSnapshotCalls != 0 || store.riskFollowUpCalls != 0 {
		t.Fatalf("calls snapshots=%d followUps=%d", store.riskFollowUpSnapshotCalls, store.riskFollowUpCalls)
	}
}

func TestSaaSAdminRiskFollowUpBulkCloseRejectsInvalidCloseStatus(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/riskFollowUpBulkClose", strings.NewReader(`{"closeStatus":"contacted"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RiskFollowUpBulkClose(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.riskFollowUpSnapshotCalls != 0 || store.riskFollowUpCalls != 0 {
		t.Fatalf("calls snapshots=%d followUps=%d", store.riskFollowUpSnapshotCalls, store.riskFollowUpCalls)
	}
}

func TestSaaSAdminUpdateTenantPackageAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		overview: SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{
			TenantID: 12, TenantName: "租户B", TenantStatus: 1,
			PackageCode: "starter", PackageName: "入门版", PackageStatus: 1, PackageVersion: 4,
			PackageLimits: SaaSAdminPackageLimits{MaxUsers: 50},
		}}},
		packages: []SaaSAdminPackage{{
			Code: "growth", Name: "增长版", Status: 1, Version: 3,
			Limits: SaaSAdminPackageLimits{MaxUsers: 120},
		}},
		updateResult: SaaSAdminTenantPackageUpdateResult{
			TenantID:         12,
			TenantName:       "租户B",
			PackageCode:      "growth",
			PackageName:      "增长版",
			ExpiresAt:        "2027-01-02 00:00:00",
			Status:           1,
			Version:          5,
			OperationID:      81,
			MetricsRefreshed: 26,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantPackage", strings.NewReader(`{"tenantId":12,"packageCode":"growth","expiresAt":"2027-01-02","expectedVersion":4}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.UpdateTenantPackage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.updateCalls != 1 {
		t.Fatalf("update calls = %d", store.updateCalls)
	}
	if store.lastUpdate.TenantID != 12 || store.lastUpdate.PackageCode != "growth" || store.lastUpdate.ExpiresAt != "2027-01-02 00:00:00" ||
		store.lastUpdate.ExpectedVersion != 4 || store.lastUpdate.ExpectedPackageVersion != 3 || store.lastUpdate.ExpectedTenantStatus != 1 {
		t.Fatalf("last update = %+v", store.lastUpdate)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["metricsRefreshed"].(float64) != 26 || data["version"].(float64) != 5 || data["operationId"].(float64) != 81 {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminUpdateTenantPackageRejectsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantPackage", strings.NewReader(`{"tenantId":12,"packageCode":"growth"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.UpdateTenantPackage(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.updateCalls != 0 {
		t.Fatalf("update calls = %d", store.updateCalls)
	}
}

func TestSaaSAdminPackageSyncDefaultsToDryRun(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		packages: []SaaSAdminPackage{{
			Code:   "scale",
			Name:   "规模版",
			Status: 1,
			Limits: SaaSAdminPackageLimits{MaxUsers: 5},
		}},
		overview: SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{
			TenantID:    12,
			TenantName:  "租户B",
			PackageCode: "scale",
			ExpiresAt:   "2027-02-03 00:00:00",
		}}},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{Metric: SaaSMetricUsers, Current: 8, Limit: 120}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/packageSync", strings.NewReader(`{"packageCode":"scale"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.PackageSync(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.updateCalls != 0 {
		t.Fatalf("update calls = %d", store.updateCalls)
	}
	if store.lastOptions.Scope != SaaSAdminScopePlatform || store.lastOptions.PackageCode != "scale" || store.lastOptions.DueState != SaaSAdminDueStateAll || store.lastOptions.Limit != saasAdminListMaxLimit {
		t.Fatalf("overview options = %+v", store.lastOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["dryRun"] != true || data["tenantSnapshotsUpdated"] != false || data["matchedTenantCount"].(float64) != 1 || data["checkedTenantCount"].(float64) != 1 || data["overLimitTenantCount"].(float64) != 1 || data["syncedTenantCount"].(float64) != 0 {
		t.Fatalf("data = %+v", data)
	}
	overLimit := data["overLimitTenants"].([]any)[0].(map[string]any)
	if overLimit["tenantId"].(float64) != 12 || overLimit["metric"] != SaaSMetricUsers || overLimit["current"].(float64) != 8 || overLimit["limit"].(float64) != 5 {
		t.Fatalf("over limit = %+v", overLimit)
	}
}

func TestSaaSAdminPackageSyncBlocksOverLimitApply(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		packages: []SaaSAdminPackage{{
			Code:   "scale",
			Name:   "规模版",
			Status: 1,
			Limits: SaaSAdminPackageLimits{MaxUsers: 5},
		}},
		overview: SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{
			TenantID:   12,
			TenantName: "租户B",
			ExpiresAt:  "2027-02-03 00:00:00",
		}}},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{Metric: SaaSMetricUsers, Current: 8}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/packageSync", strings.NewReader(`{"packageCode":"scale","dryRun":false}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.PackageSync(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.updateCalls != 0 {
		t.Fatalf("update calls = %d", store.updateCalls)
	}
	var decoded struct {
		Code int            `json:"code"`
		Msg  string         `json:"msg"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if decoded.Code != http.StatusBadRequest || decoded.Data["blocked"] != true || decoded.Data["tenantSnapshotsUpdated"] != false || decoded.Data["overLimitTenantCount"].(float64) != 1 {
		t.Fatalf("decoded = %+v", decoded)
	}
}

func TestSaaSAdminPackageSyncAllowsOverLimitApply(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		packages: []SaaSAdminPackage{{
			Code:   "scale",
			Name:   "规模版",
			Status: 1,
			Limits: SaaSAdminPackageLimits{MaxUsers: 5},
		}},
		overview: SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{
			TenantID:   12,
			TenantName: "租户B",
			ExpiresAt:  "2027-02-03 00:00:00",
		}}},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{Metric: SaaSMetricUsers, Current: 8}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/packageSync", strings.NewReader(`{"packageCode":"scale","dryRun":false,"allowOverLimit":true,"tenantId":12,"limit":5001,"remark":"批量同步套餐快照"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.PackageSync(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.updateCalls != 1 {
		t.Fatalf("update calls = %d", store.updateCalls)
	}
	if store.lastUpdate.TenantID != 12 || store.lastUpdate.PackageCode != "scale" || store.lastUpdate.ExpiresAt != "2027-02-03 00:00:00" || store.lastUpdate.Remark != "批量同步套餐快照" || store.lastUpdate.ActorUserID != 1 || store.lastUpdate.ActorTenantID != 1 {
		t.Fatalf("last update = %+v", store.lastUpdate)
	}
	if store.lastOptions.TenantID != 12 || store.lastOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("overview options = %+v", store.lastOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["dryRun"] != false || data["allowOverLimit"] != true || data["tenantSnapshotsUpdated"] != true || data["syncedTenantCount"].(float64) != 1 || data["metricsRefreshed"].(float64) != 26 {
		t.Fatalf("data = %+v", data)
	}
	tenant := data["tenants"].([]any)[0].(map[string]any)
	if tenant["synced"] != true || tenant["metricsRefreshed"].(float64) != 26 {
		t.Fatalf("tenant = %+v", tenant)
	}
}

func TestSaaSAdminPackageSyncTaskCreatesBlockedPreview(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		packages: []SaaSAdminPackage{{
			Code:   "scale",
			Name:   "规模版",
			Status: 1,
			Limits: SaaSAdminPackageLimits{MaxUsers: 5},
		}},
		overview: SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{
			TenantID:   12,
			TenantName: "租户B",
			ExpiresAt:  "2027-02-03 00:00:00",
		}}},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{Metric: SaaSMetricUsers, Current: 8}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/packageSyncTask", strings.NewReader(`{"packageCode":"scale","remark":"先预览任务"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.PackageSyncTask(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskCreateCalls != 1 || store.updateCalls != 0 {
		t.Fatalf("task creates=%d updates=%d", store.taskCreateCalls, store.updateCalls)
	}
	if store.lastTaskCreate.Status != SaaSAdminTaskStatusBlocked || store.lastTaskCreate.TaskType != SaaSAdminTaskTypePackageSync {
		t.Fatalf("task create = %+v", store.lastTaskCreate)
	}
	if !strings.Contains(store.lastTaskCreate.PreviewJSON, `"blocked":true`) || !strings.Contains(store.lastTaskCreate.RequestJSON, `"dryRun":false`) {
		t.Fatalf("task json request=%s preview=%s", store.lastTaskCreate.RequestJSON, store.lastTaskCreate.PreviewJSON)
	}
	if store.recordOperationLogCalls != 1 {
		t.Fatalf("record operation log calls = %d", store.recordOperationLogCalls)
	}
	operation := store.lastRecordedOperationLog
	if operation.Action != SaaSAdminOperationActionTaskCreate ||
		operation.TargetType != SaaSAdminOperationTargetAdminTask ||
		operation.TargetID != "1" ||
		operation.TargetName != SaaSAdminTaskTypePackageSync ||
		operation.ActorUserID != 1 ||
		operation.Remark != "先预览任务" {
		t.Fatalf("operation = %+v", operation)
	}
	if operation.BeforeJSON != "" || !strings.Contains(operation.AfterJSON, `"status":"blocked"`) {
		t.Fatalf("operation json before=%s after=%s", operation.BeforeJSON, operation.AfterJSON)
	}
	data := decodeSaaSAdminResponse(t, rec)
	task := data["task"].(map[string]any)
	result := data["result"].(map[string]any)
	if task["status"] != SaaSAdminTaskStatusBlocked || result["blocked"] != true || result["overLimitTenantCount"].(float64) != 1 {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminTaskCancelAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{{
			ID:          31,
			TaskType:    SaaSAdminTaskTypePackageSync,
			Status:      SaaSAdminTaskStatusBlocked,
			TenantID:    12,
			PackageCode: "scale",
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/taskCancel", strings.NewReader(`{"taskId":31,"remark":"暂不处理"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TaskCancel(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskCalls != 1 || store.taskUpdateCalls != 1 {
		t.Fatalf("calls task=%d taskUpdate=%d", store.taskCalls, store.taskUpdateCalls)
	}
	if store.lastTaskStatusUpdate.TaskID != 31 || store.lastTaskStatusUpdate.Status != SaaSAdminTaskStatusCanceled || store.lastTaskStatusUpdate.Applied {
		t.Fatalf("task update = %+v", store.lastTaskStatusUpdate)
	}
	if !strings.Contains(store.lastTaskStatusUpdate.ResultJSON, `"canceled":true`) || !strings.Contains(store.lastTaskStatusUpdate.ResultJSON, `"remark":"暂不处理"`) {
		t.Fatalf("result json = %s", store.lastTaskStatusUpdate.ResultJSON)
	}
	if store.recordOperationLogCalls != 1 {
		t.Fatalf("record operation log calls = %d", store.recordOperationLogCalls)
	}
	operation := store.lastRecordedOperationLog
	if operation.Action != SaaSAdminOperationActionTaskCancel ||
		operation.TargetType != SaaSAdminOperationTargetAdminTask ||
		operation.TargetID != "31" ||
		operation.TargetName != SaaSAdminTaskTypePackageSync ||
		operation.TenantID != 12 ||
		operation.ActorUserID != 1 ||
		operation.Remark != "暂不处理" {
		t.Fatalf("operation = %+v", operation)
	}
	if !strings.Contains(operation.BeforeJSON, `"status":"blocked"`) || !strings.Contains(operation.AfterJSON, `"status":"canceled"`) {
		t.Fatalf("operation json before=%s after=%s", operation.BeforeJSON, operation.AfterJSON)
	}
	data := decodeSaaSAdminResponse(t, rec)
	task := data["task"].(map[string]any)
	result := data["result"].(map[string]any)
	if task["status"] != SaaSAdminTaskStatusCanceled || task["canApply"] != false || task["canCancel"] != false || result["canceled"] != true || result["actorUserId"].(float64) != 1 {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminTasksReturnsSummaryAndReturnedCount(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{
			{ID: 61, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusPending, TenantID: 10, PackageCode: "scale", ActorUserID: 1},
			{ID: 62, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusBlocked, TenantID: 10, PackageCode: "scale", ActorUserID: 1},
			{ID: 63, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusFailed, TenantID: 11, PackageCode: "scale", ActorUserID: 2},
			{ID: 64, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusApplied, TenantID: 11, PackageCode: "scale", ActorUserID: 2},
			{ID: 65, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusCanceled, TenantID: 0, PackageCode: "scale", ActorUserID: 0},
			{ID: 66, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusPending, TenantID: 10, PackageCode: "growth", ActorUserID: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tasks?taskType=package_sync&packageCode=scale&limit=2", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Tasks(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastTaskOptions.TaskType != SaaSAdminTaskTypePackageSync || store.lastTaskOptions.PackageCode != "scale" || store.lastTaskOptions.Limit != 2 {
		t.Fatalf("options = %+v", store.lastTaskOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["returnedCount"].(float64) != 2 {
		t.Fatalf("returnedCount = %+v", data["returnedCount"])
	}
	tasks := data["tasks"].([]any)
	if len(tasks) != 2 {
		t.Fatalf("tasks = %+v", tasks)
	}
	summary := data["summary"].(map[string]any)
	expectSummary := map[string]float64{
		"taskCount":            5,
		"pendingCount":         1,
		"blockedCount":         1,
		"failedCount":          1,
		"appliedCount":         1,
		"canceledCount":        1,
		"actionableCount":      3,
		"packageSyncCount":     5,
		"tenantRenewalCount":   0,
		"tenantProvisionCount": 0,
		"tenantCount":          2,
		"actorUserCount":       2,
	}
	for key, want := range expectSummary {
		if summary[key].(float64) != want {
			t.Fatalf("summary[%s] = %+v, want %.0f; summary=%+v", key, summary[key], want, summary)
		}
	}
}

func TestSaaSAdminTaskOwnersAggregatesByActorAndFilters(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{
			{ID: 72, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusFailed, TenantID: 10, PackageCode: "scale", ActorUserID: 1, ActorTenantID: 1, CreatedAt: "2026-07-10 10:00:00", LastError: "额度阻断"},
			{ID: 71, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusPending, TenantID: 11, PackageCode: "scale", ActorUserID: 1, ActorTenantID: 1, CreatedAt: "2026-07-10 09:00:00"},
			{ID: 74, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusBlocked, TenantID: 10, PackageCode: "scale", ActorUserID: 2, ActorTenantID: 1, CreatedAt: "2026-07-10 11:00:00"},
			{ID: 73, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusApplied, TenantID: 12, PackageCode: "scale", ActorUserID: 2, ActorTenantID: 1, AppliedAt: "2026-07-10 12:00:00", CreatedAt: "2026-07-10 08:00:00"},
			{ID: 75, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusCanceled, TenantID: 0, PackageCode: "scale", ActorUserID: 0, ActorTenantID: 0, CreatedAt: "2026-07-10 07:00:00"},
			{ID: 76, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusPending, TenantID: 10, PackageCode: "scale", ActorUserID: 3, ActorTenantID: 1, CreatedAt: "2026-07-10 06:00:00"},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/taskOwners?taskType=package_sync&packageCode=scale&limit=2", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TaskOwners(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.taskCalls != 1 {
		t.Fatalf("task calls = %d", store.taskCalls)
	}
	if store.lastTaskOptions.TaskType != SaaSAdminTaskTypePackageSync || store.lastTaskOptions.PackageCode != "scale" || store.lastTaskOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("options = %+v", store.lastTaskOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	filters := data["filters"].(map[string]any)
	if filters["taskType"] != SaaSAdminTaskTypePackageSync || filters["packageCode"] != "scale" || filters["limit"].(float64) != 2 {
		t.Fatalf("filters = %+v", filters)
	}
	if data["ownerCount"].(float64) != 3 || data["returnedCount"].(float64) != 2 || data["scannedTaskCount"].(float64) != 5 || data["partial"].(bool) {
		t.Fatalf("data = %+v", data)
	}
	summary := data["summary"].(map[string]any)
	if summary["taskCount"].(float64) != 5 || summary["packageSyncCount"].(float64) != 5 || summary["actionableCount"].(float64) != 3 {
		t.Fatalf("summary = %+v", summary)
	}
	owners := data["owners"].([]any)
	first := owners[0].(map[string]any)
	if first["actorUserId"].(float64) != 1 || first["actorTenantId"].(float64) != 1 {
		t.Fatalf("first owner = %+v", first)
	}
	firstSummary := first["summary"].(map[string]any)
	if firstSummary["taskCount"].(float64) != 2 || firstSummary["actionableCount"].(float64) != 2 || firstSummary["failedCount"].(float64) != 1 {
		t.Fatalf("first summary = %+v", firstSummary)
	}
	if first["lastError"] != "额度阻断" || first["lastTaskAt"] != "2026-07-10 10:00:00" {
		t.Fatalf("first owner = %+v", first)
	}
	recentTasks := first["recentTasks"].([]any)
	if len(recentTasks) != 2 || recentTasks[0].(map[string]any)["id"].(float64) != 72 {
		t.Fatalf("recent tasks = %+v", recentTasks)
	}
}

func TestSaaSAdminTaskOwnersRequiresPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			2: {ID: 2, TenantID: 2, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/taskOwners", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()

	handler.TaskOwners(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestSaaSAdminTaskSLAAggregatesActiveTasks(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	format := func(t time.Time) string {
		return t.Format("2006-01-02 15:04:05")
	}
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{
			{ID: 81, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusPending, TenantID: 10, PackageCode: "scale", ActorUserID: 1, ActorTenantID: 1, CreatedAt: format(now.Add(-30 * time.Hour))},
			{ID: 82, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusBlocked, TenantID: 11, PackageCode: "scale", ActorUserID: 1, ActorTenantID: 1, CreatedAt: format(now.Add(-5 * time.Hour))},
			{ID: 83, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusFailed, TenantID: 12, PackageCode: "scale", ActorUserID: 2, ActorTenantID: 1, CreatedAt: format(now.Add(-1 * time.Hour))},
			{ID: 84, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusApplied, TenantID: 12, PackageCode: "scale", ActorUserID: 2, ActorTenantID: 1, CreatedAt: format(now.Add(-100 * time.Hour))},
			{ID: 85, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusCanceled, TenantID: 0, PackageCode: "scale", ActorUserID: 0, ActorTenantID: 0, CreatedAt: format(now.Add(-100 * time.Hour))},
			{ID: 86, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusPending, TenantID: 10, PackageCode: "growth", ActorUserID: 3, ActorTenantID: 1, CreatedAt: format(now.Add(-40 * time.Hour))},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/taskSla?taskType=package_sync&packageCode=scale&warningHours=4&overdueHours=24&limit=2", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TaskSLA(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastTaskOptions.TaskType != SaaSAdminTaskTypePackageSync || store.lastTaskOptions.PackageCode != "scale" || store.lastTaskOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("options = %+v", store.lastTaskOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	filters := data["filters"].(map[string]any)
	if filters["warningHours"].(float64) != 4 || filters["overdueHours"].(float64) != 24 || filters["limit"].(float64) != 2 {
		t.Fatalf("filters = %+v", filters)
	}
	taskSummary := data["taskSummary"].(map[string]any)
	if taskSummary["taskCount"].(float64) != 5 || taskSummary["appliedCount"].(float64) != 1 || taskSummary["canceledCount"].(float64) != 1 {
		t.Fatalf("task summary = %+v", taskSummary)
	}
	summary := data["summary"].(map[string]any)
	if summary["taskCount"].(float64) != 3 || summary["overdueCount"].(float64) != 1 || summary["warningCount"].(float64) != 1 || summary["freshCount"].(float64) != 1 || summary["maxAgeHours"].(float64) < 30 {
		t.Fatalf("summary = %+v", summary)
	}
	if data["ownerCount"].(float64) != 2 || data["returnedOwners"].(float64) != 2 || data["returnedCount"].(float64) != 2 || data["scannedTaskCount"].(float64) != 5 || data["partial"].(bool) {
		t.Fatalf("data = %+v", data)
	}
	tasks := data["tasks"].([]any)
	first := tasks[0].(map[string]any)
	firstTask := first["task"].(map[string]any)
	if firstTask["id"].(float64) != 81 || first["slaStatus"] != "overdue" || first["breachHours"].(float64) < 6 {
		t.Fatalf("first sla task = %+v", first)
	}
	owners := data["owners"].([]any)
	owner := owners[0].(map[string]any)
	ownerSummary := owner["summary"].(map[string]any)
	if owner["actorUserId"].(float64) != 1 || ownerSummary["taskCount"].(float64) != 2 || ownerSummary["overdueCount"].(float64) != 1 || ownerSummary["warningCount"].(float64) != 1 {
		t.Fatalf("owner = %+v", owner)
	}
}

func TestSaaSAdminTaskSLARejectsInvalidThresholds(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/taskSla?warningHours=24&overdueHours=4", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TaskSLA(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestSaaSAdminTaskSLARequiresPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			2: {ID: 2, TenantID: 2, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/taskSla", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()

	handler.TaskSLA(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestSaaSAdminTaskSLANotificationsEnqueuesAndSkipsExisting(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	format := func(t time.Time) string {
		return t.Format("2006-01-02 15:04:05")
	}
	options := SaaSAdminTaskSLAOptions{
		SaaSAdminTaskOptions: SaaSAdminTaskOptions{TaskType: SaaSAdminTaskTypePackageSync, PackageCode: "scale", Limit: 20},
		WarningHours:         4,
		OverdueHours:         24,
	}
	existingTask := SaaSAdminTask{ID: 92, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusBlocked, TenantID: 11, PackageCode: "scale", ActorUserID: 2, ActorTenantID: 1, CreatedAt: format(now.Add(-5 * time.Hour))}
	existingItem := saasAdminTaskSLAItem(existingTask, options, now)
	existingNotify := SaaSAdminTaskSLANotifications{Options: options, Channel: SaaSAlertNotificationChannelWebhook, MaxAttempts: 3, SLAStatus: "warning", Remark: "existing"}
	existingAlert, existingKey, err := saasAdminTaskSLANotificationAlert(existingItem, existingNotify, 1)
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{
			{ID: 91, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusPending, TenantID: 10, PackageCode: "scale", ActorUserID: 1, ActorTenantID: 1, Remark: "超时待处理", CreatedAt: format(now.Add(-30 * time.Hour)), UpdatedAt: format(now.Add(-29 * time.Hour))},
			existingTask,
			{ID: 93, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusFailed, TenantID: 12, PackageCode: "scale", ActorUserID: 3, ActorTenantID: 1, CreatedAt: format(now.Add(-1 * time.Hour))},
			{ID: 94, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusApplied, TenantID: 12, PackageCode: "scale", ActorUserID: 3, ActorTenantID: 1, CreatedAt: format(now.Add(-100 * time.Hour))},
		},
		notifications: []SaaSAlertNotification{{
			ID:              601,
			NotificationKey: existingKey,
			AlertKey:        fmt.Sprintf("11:%s:%s:%s", SaaSEventMetricAdminTaskSLA, SaaSAlertTypeAdminTaskSLA, existingAlert.PeriodKey),
			TenantID:        11,
			Channel:         SaaSAlertNotificationChannelWebhook,
			Status:          SaaSAlertNotificationStatusDelivered,
			MaxAttempts:     3,
			Alert:           existingAlert,
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/taskSlaNotifications?taskType=package_sync&packageCode=scale&warningHours=4&overdueHours=24&limit=20", strings.NewReader(`{"maxAttempts":4,"remark":"SLA催办"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TaskSLANotifications(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationEnqueueCalls != 1 || store.recordOperationLogCalls != 1 {
		t.Fatalf("enqueue calls=%d operation logs=%d", store.notificationEnqueueCalls, store.recordOperationLogCalls)
	}
	if store.lastNotificationOptions.Keyword != SaaSAlertTypeAdminTaskSLA ||
		store.lastNotificationOptions.Channel != SaaSAlertNotificationChannelWebhook ||
		store.lastNotificationOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("existing notification options = %+v", store.lastNotificationOptions)
	}
	if store.lastEnqueuedChannel != SaaSAlertNotificationChannelWebhook || store.lastEnqueuedMaxAttempts != 4 {
		t.Fatalf("enqueue channel=%q max=%d", store.lastEnqueuedChannel, store.lastEnqueuedMaxAttempts)
	}
	alert := store.lastEnqueuedAlert
	if alert.AlertType != SaaSAlertTypeAdminTaskSLA ||
		alert.Source != "saas_admin.task_sla.notification" ||
		alert.Severity != SaaSAlertSeverityCritical ||
		alert.Status.TenantID != 10 ||
		alert.Status.Metric != SaaSEventMetricAdminTaskSLA ||
		alert.Status.Limit != 24 ||
		alert.PeriodKey != "admin_task_sla_91_overdue_4h_24h" ||
		!strings.Contains(alert.Message, "运营任务 #91") ||
		alert.Context["taskId"].(int64) != 91 ||
		alert.Context["slaStatus"] != "overdue" ||
		alert.Context["packageCode"] != "scale" ||
		alert.Context["notificationRemark"] != "SLA催办" {
		t.Fatalf("alert = %+v", alert)
	}
	operation := store.lastRecordedOperationLog
	if operation.Action != SaaSAdminOperationActionTaskSLANotify ||
		operation.TargetType != SaaSAdminOperationTargetAlertNotification ||
		operation.TenantID != 10 ||
		operation.Remark != "SLA催办" ||
		!strings.Contains(operation.AfterJSON, `"source":"admin_task_sla"`) ||
		!strings.Contains(operation.AfterJSON, `"id":91`) ||
		!strings.Contains(operation.AfterJSON, SaaSAlertTypeAdminTaskSLA) {
		t.Fatalf("operation = %+v", operation)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["matchedCount"].(float64) != 3 ||
		data["eligibleCount"].(float64) != 2 ||
		data["enqueuedCount"].(float64) != 1 ||
		data["skippedExistingCount"].(float64) != 1 ||
		data["skippedStatusCount"].(float64) != 1 ||
		data["skippedInvalidCount"].(float64) != 0 ||
		data["slaStatus"] != "warning" ||
		data["taskSLANotificationKey"] != SaaSAlertTypeAdminTaskSLA {
		t.Fatalf("data = %+v", data)
	}
	notification := data["notifications"].([]any)[0].(map[string]any)
	if notification["tenantId"].(float64) != 10 ||
		notification["status"] != SaaSAlertNotificationStatusPending ||
		notification["metric"] != SaaSEventMetricAdminTaskSLA ||
		notification["alertType"] != SaaSAlertTypeAdminTaskSLA {
		t.Fatalf("notification = %+v", notification)
	}
	skipped := data["skipped"].([]any)
	if len(skipped) != 2 {
		t.Fatalf("skipped = %+v", skipped)
	}
}

func TestSaaSAdminTaskSLANotificationsRejectsTenantAdminAndInvalidStatus(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			2: {ID: 2, TenantID: 2, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/taskSlaNotifications", strings.NewReader(`{"slaStatus":"warning"}`))
	tenantReq.Header.Set("Content-Type", "application/json")
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "2")
	tenantRec := httptest.NewRecorder()
	handler.TaskSLANotifications(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/taskSlaNotifications", strings.NewReader(`{"slaStatus":"bad"}`))
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidReq.Header.Set("X-Mochat-Go-User-ID", "1")
	invalidRec := httptest.NewRecorder()
	handler.TaskSLANotifications(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d body=%s", invalidRec.Code, invalidRec.Body.String())
	}
	if store.notificationEnqueueCalls != 0 || store.recordOperationLogCalls != 0 {
		t.Fatalf("calls enqueue=%d operation=%d", store.notificationEnqueueCalls, store.recordOperationLogCalls)
	}
}

func TestSaaSAdminTaskCancelRejectsAppliedTask(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{{
			ID:     32,
			Status: SaaSAdminTaskStatusApplied,
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/taskCancel", strings.NewReader(`{"taskId":32}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TaskCancel(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskUpdateCalls != 0 {
		t.Fatalf("task updates = %d", store.taskUpdateCalls)
	}
}

func TestSaaSAdminTaskResetMovesFailedTaskToPending(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{{
			ID:          33,
			TaskType:    SaaSAdminTaskTypePackageSync,
			Status:      SaaSAdminTaskStatusFailed,
			TenantID:    12,
			PackageCode: "scale",
			ResultJSON:  `{"failed":true}`,
			LastError:   "webhook 500",
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/taskReset", strings.NewReader(`{"taskId":33,"remark":"重新排队"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TaskReset(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskCalls != 1 || store.taskUpdateCalls != 1 {
		t.Fatalf("calls task=%d taskUpdate=%d", store.taskCalls, store.taskUpdateCalls)
	}
	if store.lastTaskStatusUpdate.TaskID != 33 || store.lastTaskStatusUpdate.Status != SaaSAdminTaskStatusPending || store.lastTaskStatusUpdate.ResultJSON != "" || store.lastTaskStatusUpdate.LastError != "" {
		t.Fatalf("task update = %+v", store.lastTaskStatusUpdate)
	}
	if store.recordOperationLogCalls != 1 {
		t.Fatalf("record operation log calls = %d", store.recordOperationLogCalls)
	}
	operation := store.lastRecordedOperationLog
	if operation.Action != SaaSAdminOperationActionTaskReset ||
		operation.TargetType != SaaSAdminOperationTargetAdminTask ||
		operation.TargetID != "33" ||
		operation.TargetName != SaaSAdminTaskTypePackageSync ||
		operation.TenantID != 12 ||
		operation.ActorUserID != 1 ||
		operation.Remark != "重新排队" {
		t.Fatalf("operation = %+v", operation)
	}
	if !strings.Contains(operation.BeforeJSON, `"status":"failed"`) ||
		!strings.Contains(operation.BeforeJSON, `"lastError":"webhook 500"`) ||
		!strings.Contains(operation.AfterJSON, `"status":"pending"`) ||
		!strings.Contains(operation.AfterJSON, `"reset":true`) ||
		!strings.Contains(operation.AfterJSON, `"previousStatus":"failed"`) {
		t.Fatalf("operation json before=%s after=%s", operation.BeforeJSON, operation.AfterJSON)
	}
	data := decodeSaaSAdminResponse(t, rec)
	task := data["task"].(map[string]any)
	result := data["result"].(map[string]any)
	if task["status"] != SaaSAdminTaskStatusPending || task["lastError"] != "" || task["canApply"] != true || task["canCancel"] != true || result["reset"] != true || result["previousStatus"] != SaaSAdminTaskStatusFailed {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminTaskResetRejectsPendingTask(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{{
			ID:     34,
			Status: SaaSAdminTaskStatusPending,
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/taskReset", strings.NewReader(`{"taskId":34}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TaskReset(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskUpdateCalls != 0 || store.recordOperationLogCalls != 0 {
		t.Fatalf("task updates=%d operation logs=%d", store.taskUpdateCalls, store.recordOperationLogCalls)
	}
}

func TestSaaSAdminTaskBulkCancelCancelsEligibleAndSkipsApplied(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{
			{ID: 41, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusPending, PackageCode: "scale"},
			{ID: 42, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusBlocked, PackageCode: "scale"},
			{ID: 43, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusApplied, PackageCode: "scale"},
			{ID: 44, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusCanceled, PackageCode: "scale"},
			{ID: 45, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusFailed, PackageCode: "scale"},
			{ID: 46, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusPending, PackageCode: "growth"},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/taskBulkCancel", strings.NewReader(`{"taskType":"package_sync","status":"all","packageCode":"scale","limit":10,"remark":"批量清理"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TaskBulkCancel(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskCalls != 1 || store.taskUpdateCalls != 3 {
		t.Fatalf("calls task=%d taskUpdate=%d", store.taskCalls, store.taskUpdateCalls)
	}
	if store.lastTaskOptions.TaskType != SaaSAdminTaskTypePackageSync || store.lastTaskOptions.PackageCode != "scale" || store.lastTaskOptions.Status != "" || store.lastTaskOptions.Limit != 10 {
		t.Fatalf("task options = %+v", store.lastTaskOptions)
	}
	statusByID := map[int64]string{}
	for _, item := range store.tasks {
		statusByID[item.ID] = item.Status
	}
	for _, id := range []int64{41, 42, 45} {
		if statusByID[id] != SaaSAdminTaskStatusCanceled {
			t.Fatalf("task %d status = %s", id, statusByID[id])
		}
	}
	if statusByID[43] != SaaSAdminTaskStatusApplied || statusByID[44] != SaaSAdminTaskStatusCanceled || statusByID[46] != SaaSAdminTaskStatusPending {
		t.Fatalf("status map = %+v", statusByID)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["canceled"] != true || data["matchedCount"].(float64) != 5 || data["canceledCount"].(float64) != 3 || data["skippedAppliedCount"].(float64) != 1 || data["skippedCanceledCount"].(float64) != 1 {
		t.Fatalf("data = %+v", data)
	}
	filters := data["filters"].(map[string]any)
	if filters["taskType"] != SaaSAdminTaskTypePackageSync || filters["packageCode"] != "scale" || filters["limit"].(float64) != 10 {
		t.Fatalf("filters = %+v", filters)
	}
	tasks := data["tasks"].([]any)
	if len(tasks) != 3 {
		t.Fatalf("tasks = %+v", tasks)
	}
	if !strings.Contains(store.lastTaskStatusUpdate.ResultJSON, `"bulkCancel":true`) || !strings.Contains(store.lastTaskStatusUpdate.ResultJSON, `"remark":"批量清理"`) {
		t.Fatalf("result json = %s", store.lastTaskStatusUpdate.ResultJSON)
	}
	if store.recordOperationLogCalls != 3 || len(store.operationLogs) != 3 {
		t.Fatalf("operation logs calls=%d logs=%d", store.recordOperationLogCalls, len(store.operationLogs))
	}
	for _, operation := range store.operationLogs {
		if operation.Action != SaaSAdminOperationActionTaskBulkCancel ||
			operation.TargetType != SaaSAdminOperationTargetAdminTask ||
			operation.TargetName != SaaSAdminTaskTypePackageSync ||
			operation.Remark != "批量清理" {
			t.Fatalf("operation = %+v", operation)
		}
		if !strings.Contains(operation.AfterJSON, `"status":"canceled"`) || !strings.Contains(operation.AfterJSON, `"bulkCancel":true`) {
			t.Fatalf("operation after json = %s", operation.AfterJSON)
		}
	}
}

func TestSaaSAdminTaskBulkResetResetsFailedAndBlockedOnly(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{
			{ID: 51, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusBlocked, PackageCode: "scale", ResultJSON: `{"blocked":true}`, LastError: "over limit"},
			{ID: 52, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusFailed, PackageCode: "scale", ResultJSON: `{"failed":true}`, LastError: "apply failed"},
			{ID: 53, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusPending, PackageCode: "scale"},
			{ID: 54, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusApplied, PackageCode: "scale"},
			{ID: 55, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusCanceled, PackageCode: "scale"},
			{ID: 56, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusBlocked, PackageCode: "growth"},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/taskBulkReset", strings.NewReader(`{"taskType":"package_sync","status":"all","packageCode":"scale","limit":10,"remark":"批量重置"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TaskBulkReset(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskCalls != 1 || store.taskUpdateCalls != 2 {
		t.Fatalf("calls task=%d taskUpdate=%d", store.taskCalls, store.taskUpdateCalls)
	}
	if store.lastTaskOptions.TaskType != SaaSAdminTaskTypePackageSync || store.lastTaskOptions.PackageCode != "scale" || store.lastTaskOptions.Status != "" || store.lastTaskOptions.Limit != 10 {
		t.Fatalf("task options = %+v", store.lastTaskOptions)
	}
	statusByID := map[int64]string{}
	lastErrorByID := map[int64]string{}
	resultByID := map[int64]string{}
	for _, item := range store.tasks {
		statusByID[item.ID] = item.Status
		lastErrorByID[item.ID] = item.LastError
		resultByID[item.ID] = item.ResultJSON
	}
	for _, id := range []int64{51, 52, 53} {
		if statusByID[id] != SaaSAdminTaskStatusPending {
			t.Fatalf("task %d status = %s", id, statusByID[id])
		}
	}
	if lastErrorByID[51] != "" || lastErrorByID[52] != "" || resultByID[51] != "" || resultByID[52] != "" {
		t.Fatalf("reset task error/result not cleared: lastError=%+v result=%+v", lastErrorByID, resultByID)
	}
	if statusByID[54] != SaaSAdminTaskStatusApplied || statusByID[55] != SaaSAdminTaskStatusCanceled || statusByID[56] != SaaSAdminTaskStatusBlocked {
		t.Fatalf("status map = %+v", statusByID)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["reset"] != true ||
		data["matchedCount"].(float64) != 5 ||
		data["resetCount"].(float64) != 2 ||
		data["skippedPendingCount"].(float64) != 1 ||
		data["skippedAppliedCount"].(float64) != 1 ||
		data["skippedCanceledCount"].(float64) != 1 {
		t.Fatalf("data = %+v", data)
	}
	filters := data["filters"].(map[string]any)
	if filters["taskType"] != SaaSAdminTaskTypePackageSync || filters["packageCode"] != "scale" || filters["limit"].(float64) != 10 {
		t.Fatalf("filters = %+v", filters)
	}
	tasks := data["tasks"].([]any)
	if len(tasks) != 2 {
		t.Fatalf("tasks = %+v", tasks)
	}
	if store.recordOperationLogCalls != 2 || len(store.operationLogs) != 2 {
		t.Fatalf("operation logs calls=%d logs=%d", store.recordOperationLogCalls, len(store.operationLogs))
	}
	previousStatuses := map[string]bool{}
	for _, operation := range store.operationLogs {
		if operation.Action != SaaSAdminOperationActionTaskBulkReset ||
			operation.TargetType != SaaSAdminOperationTargetAdminTask ||
			operation.TargetName != SaaSAdminTaskTypePackageSync ||
			operation.Remark != "批量重置" {
			t.Fatalf("operation = %+v", operation)
		}
		if !strings.Contains(operation.AfterJSON, `"status":"pending"`) ||
			!strings.Contains(operation.AfterJSON, `"bulkReset":true`) ||
			!strings.Contains(operation.AfterJSON, `"reset":true`) {
			t.Fatalf("operation after json = %s", operation.AfterJSON)
		}
		if strings.Contains(operation.AfterJSON, `"previousStatus":"blocked"`) {
			previousStatuses[SaaSAdminTaskStatusBlocked] = true
		}
		if strings.Contains(operation.AfterJSON, `"previousStatus":"failed"`) {
			previousStatuses[SaaSAdminTaskStatusFailed] = true
		}
	}
	if !previousStatuses[SaaSAdminTaskStatusBlocked] || !previousStatuses[SaaSAdminTaskStatusFailed] {
		t.Fatalf("previous statuses = %+v logs=%+v", previousStatuses, store.operationLogs)
	}
}

func TestSaaSAdminPackageSyncTaskApplyRejectsCanceledTask(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{{
			ID:       33,
			TaskType: SaaSAdminTaskTypePackageSync,
			Status:   SaaSAdminTaskStatusCanceled,
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/packageSyncTaskApply", strings.NewReader(`{"taskId":33}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.PackageSyncTaskApply(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskUpdateCalls != 0 || store.updateCalls != 0 {
		t.Fatalf("task updates=%d updates=%d", store.taskUpdateCalls, store.updateCalls)
	}
}

func TestSaaSAdminPackageSyncTaskApplyUsesStoredRequest(t *testing.T) {
	requestJSON := saasAdminPayloadJSON(saasAdminPackageSyncTaskRequestPayload(SaaSAdminPackageTenantSnapshotSync{
		PackageCode:    "scale",
		TenantID:       12,
		Limit:          100,
		AllowOverLimit: true,
		Remark:         "任务应用",
	}))
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{{
			ID:          9,
			TaskType:    SaaSAdminTaskTypePackageSync,
			Status:      SaaSAdminTaskStatusPending,
			TenantID:    12,
			PackageCode: "scale",
			RequestJSON: requestJSON,
			Remark:      "任务应用",
		}},
		packages: []SaaSAdminPackage{{
			Code:   "scale",
			Name:   "规模版",
			Status: 1,
			Limits: SaaSAdminPackageLimits{MaxUsers: 5},
		}},
		overview: SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{
			TenantID:   12,
			TenantName: "租户B",
			ExpiresAt:  "2027-02-03 00:00:00",
		}}},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{Metric: SaaSMetricUsers, Current: 8}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/packageSyncTaskApply", strings.NewReader(`{"taskId":9}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.PackageSyncTaskApply(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskCalls != 1 || store.taskUpdateCalls != 1 || store.updateCalls != 1 {
		t.Fatalf("calls task=%d taskUpdate=%d update=%d", store.taskCalls, store.taskUpdateCalls, store.updateCalls)
	}
	if store.lastUpdate.TenantID != 12 || store.lastUpdate.PackageCode != "scale" || store.lastUpdate.Remark != "任务应用" || store.lastUpdate.ActorUserID != 1 {
		t.Fatalf("last update = %+v", store.lastUpdate)
	}
	if store.lastTaskStatusUpdate.TaskID != 9 || store.lastTaskStatusUpdate.Status != SaaSAdminTaskStatusApplied || !store.lastTaskStatusUpdate.Applied {
		t.Fatalf("task update = %+v", store.lastTaskStatusUpdate)
	}
	if store.recordOperationLogCalls != 1 {
		t.Fatalf("record operation log calls = %d", store.recordOperationLogCalls)
	}
	operation := store.lastRecordedOperationLog
	if operation.Action != SaaSAdminOperationActionTaskApply ||
		operation.TargetType != SaaSAdminOperationTargetAdminTask ||
		operation.TargetID != "9" ||
		operation.TenantID != 12 ||
		operation.Remark != "任务应用" {
		t.Fatalf("operation = %+v", operation)
	}
	if !strings.Contains(operation.BeforeJSON, `"status":"pending"`) || !strings.Contains(operation.AfterJSON, `"status":"applied"`) {
		t.Fatalf("operation json before=%s after=%s", operation.BeforeJSON, operation.AfterJSON)
	}
	data := decodeSaaSAdminResponse(t, rec)
	task := data["task"].(map[string]any)
	result := data["result"].(map[string]any)
	if task["status"] != SaaSAdminTaskStatusApplied || result["syncedTenantCount"].(float64) != 1 || result["metricsRefreshed"].(float64) != 26 {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminPackageSyncTaskBulkApplyAppliesAndReportsOutcomes(t *testing.T) {
	validRequest := saasAdminPayloadJSON(saasAdminPackageSyncTaskRequestPayload(SaaSAdminPackageTenantSnapshotSync{
		PackageCode:    "scale",
		TenantID:       12,
		Limit:          100,
		AllowOverLimit: true,
		Remark:         "批量套餐同步",
	}))
	blockedRequest := saasAdminPayloadJSON(saasAdminPackageSyncTaskRequestPayload(SaaSAdminPackageTenantSnapshotSync{
		PackageCode:    "scale",
		TenantID:       12,
		Limit:          100,
		AllowOverLimit: false,
		Remark:         "会阻断",
	}))
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{
			{ID: 81, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusPending, TenantID: 12, PackageCode: "scale", RequestJSON: validRequest, Remark: "批量套餐同步"},
			{ID: 82, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusPending, TenantID: 12, PackageCode: "scale", RequestJSON: blockedRequest, Remark: "会阻断"},
			{ID: 83, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusPending, TenantID: 12, PackageCode: "scale", RequestJSON: "{bad", Remark: "坏请求"},
			{ID: 84, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusApplied, TenantID: 12, PackageCode: "scale", RequestJSON: validRequest},
			{ID: 85, TaskType: SaaSAdminTaskTypePackageSync, Status: SaaSAdminTaskStatusCanceled, TenantID: 12, PackageCode: "scale", RequestJSON: validRequest},
		},
		packages: []SaaSAdminPackage{{
			Code:   "scale",
			Name:   "规模版",
			Status: 1,
			Limits: SaaSAdminPackageLimits{MaxUsers: 5},
		}},
		overview: SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{
			TenantID:   12,
			TenantName: "租户B",
			ExpiresAt:  "2027-02-03 00:00:00",
		}}},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{Metric: SaaSMetricUsers, Current: 8}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/packageSyncTaskBulkApply", strings.NewReader(`{"status":"all","packageCode":"scale","limit":10,"remark":"批量应用套餐同步"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.PackageSyncTaskBulkApply(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskCalls != 1 || store.taskUpdateCalls != 3 || store.updateCalls != 1 {
		t.Fatalf("calls task=%d taskUpdate=%d update=%d", store.taskCalls, store.taskUpdateCalls, store.updateCalls)
	}
	if store.lastTaskOptions.TaskType != SaaSAdminTaskTypePackageSync ||
		store.lastTaskOptions.Status != "" ||
		store.lastTaskOptions.PackageCode != "scale" ||
		store.lastTaskOptions.Limit != 10 {
		t.Fatalf("task options = %+v", store.lastTaskOptions)
	}
	if store.lastUpdate.TenantID != 12 ||
		store.lastUpdate.PackageCode != "scale" ||
		store.lastUpdate.Remark != "批量套餐同步" ||
		store.lastUpdate.ActorUserID != 1 ||
		store.lastUpdate.ActorTenantID != 1 {
		t.Fatalf("last update = %+v", store.lastUpdate)
	}
	statusByID := map[int64]string{}
	lastErrorByID := map[int64]string{}
	for _, item := range store.tasks {
		statusByID[item.ID] = item.Status
		lastErrorByID[item.ID] = item.LastError
	}
	if statusByID[81] != SaaSAdminTaskStatusApplied ||
		statusByID[82] != SaaSAdminTaskStatusBlocked ||
		statusByID[83] != SaaSAdminTaskStatusFailed ||
		statusByID[84] != SaaSAdminTaskStatusApplied ||
		statusByID[85] != SaaSAdminTaskStatusCanceled {
		t.Fatalf("status map = %+v", statusByID)
	}
	if !strings.Contains(lastErrorByID[82], "存在超出新套餐额度的租户") || !strings.Contains(lastErrorByID[83], "task request JSON 格式错误") {
		t.Fatalf("last errors = %+v", lastErrorByID)
	}
	if store.recordOperationLogCalls != 2 || len(store.operationLogs) != 2 {
		t.Fatalf("operation logs calls=%d logs=%d", store.recordOperationLogCalls, len(store.operationLogs))
	}
	seenApply := false
	seenBlock := false
	for _, operation := range store.operationLogs {
		if operation.TargetType != SaaSAdminOperationTargetAdminTask ||
			operation.TargetName != SaaSAdminTaskTypePackageSync ||
			operation.Remark == "" {
			t.Fatalf("operation = %+v", operation)
		}
		if !strings.Contains(operation.AfterJSON, `"bulkApply":true`) {
			t.Fatalf("operation after json = %s", operation.AfterJSON)
		}
		switch operation.Action {
		case SaaSAdminOperationActionTaskApply:
			seenApply = true
			if operation.TargetID != "81" || operation.Remark != "批量应用套餐同步" {
				t.Fatalf("apply operation = %+v", operation)
			}
		case SaaSAdminOperationActionTaskBlock:
			seenBlock = true
			if operation.TargetID != "82" {
				t.Fatalf("block operation = %+v", operation)
			}
		default:
			t.Fatalf("operation action = %+v", operation)
		}
	}
	if !seenApply || !seenBlock {
		t.Fatalf("seen apply=%v block=%v logs=%+v", seenApply, seenBlock, store.operationLogs)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["applied"] != true ||
		data["matchedCount"].(float64) != 5 ||
		data["appliedCount"].(float64) != 1 ||
		data["blockedCount"].(float64) != 1 ||
		data["failedCount"].(float64) != 1 ||
		data["skippedAppliedCount"].(float64) != 1 ||
		data["skippedCanceledCount"].(float64) != 1 {
		t.Fatalf("data = %+v", data)
	}
	tasks := data["tasks"].([]any)
	if len(tasks) != 3 {
		t.Fatalf("tasks = %+v", tasks)
	}
	errors := data["errors"].([]any)
	if len(errors) != 1 || errors[0].(map[string]any)["taskId"].(float64) != 83 {
		t.Fatalf("errors = %+v", errors)
	}
}

func TestSaaSAdminPackageSyncTaskBulkApplyRejectsTenantAdminAndWrongType(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/packageSyncTaskBulkApply", strings.NewReader(`{"taskType":"package_sync"}`))
	tenantReq.Header.Set("Content-Type", "application/json")
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.PackageSyncTaskBulkApply(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
	if store.taskCalls != 0 || store.taskUpdateCalls != 0 {
		t.Fatalf("calls task=%d update=%d", store.taskCalls, store.taskUpdateCalls)
	}

	wrongTypeReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/packageSyncTaskBulkApply", strings.NewReader(`{"taskType":"tenant_renewal"}`))
	wrongTypeReq.Header.Set("Content-Type", "application/json")
	wrongTypeReq.Header.Set("X-Mochat-Go-User-ID", "1")
	wrongTypeRec := httptest.NewRecorder()
	handler.PackageSyncTaskBulkApply(wrongTypeRec, wrongTypeReq)
	if wrongTypeRec.Code != http.StatusBadRequest {
		t.Fatalf("wrong type status = %d body=%s", wrongTypeRec.Code, wrongTypeRec.Body.String())
	}
	if store.taskCalls != 0 || store.taskUpdateCalls != 0 {
		t.Fatalf("calls after wrong type task=%d update=%d", store.taskCalls, store.taskUpdateCalls)
	}
}

func TestSaaSAdminPackageSyncRejectsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/packageSync", strings.NewReader(`{"packageCode":"scale"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.PackageSync(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.packageCalls != 0 || store.overviewCalls != 0 || store.updateCalls != 0 {
		t.Fatalf("calls package=%d overview=%d update=%d", store.packageCalls, store.overviewCalls, store.updateCalls)
	}
}

func TestSaaSAdminRenewTenantAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		renewalResult: SaaSAdminTenantRenewalResult{
			TenantID:          12,
			TenantName:        "租户B",
			PackageCode:       "growth",
			PackageName:       "增长版",
			PreviousExpiresAt: "2027-01-02 00:00:00",
			ExpiresAt:         "2028-01-02 00:00:00",
			AmountCents:       12345,
			Currency:          "CNY",
			BillingEventID:    99,
			MetricsRefreshed:  26,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantRenewal", strings.NewReader(`{"tenantId":12,"packageCode":"growth","expiresAt":"2028-01-02","amount":"123.45","currency":"cny","paidAt":"2026-07-09","paymentMethod":"bank","externalOrderNo":"ORDER-2","remark":"续费一年"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RenewTenant(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.renewalCalls != 1 {
		t.Fatalf("renewal calls = %d", store.renewalCalls)
	}
	if store.lastRenewal.TenantID != 12 || store.lastRenewal.PackageCode != "growth" || store.lastRenewal.ExpiresAt != "2028-01-02 00:00:00" {
		t.Fatalf("last renewal = %+v", store.lastRenewal)
	}
	if store.lastRenewal.AmountCents != 12345 || store.lastRenewal.Currency != "CNY" || store.lastRenewal.PaidAt != "2026-07-09 00:00:00" || store.lastRenewal.ActorUserID != 1 {
		t.Fatalf("last renewal money/actor = %+v", store.lastRenewal)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["billingEventId"].(float64) != 99 || data["amountCents"].(float64) != 12345 || data["metricsRefreshed"].(float64) != 26 {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminTenantRenewalTaskCreatesPendingPreview(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		packages: []SaaSAdminPackage{{
			Code:   "growth",
			Name:   "增长版",
			Status: 1,
		}},
		overview: SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{
			TenantID:    12,
			TenantName:  "租户B",
			PackageCode: "growth",
			PackageName: "增长版",
			ExpiresAt:   "2027-01-02 00:00:00",
		}}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantRenewalTask", strings.NewReader(`{"tenantId":12,"expiresAt":"2028-01-02","amount":"123.45","externalOrderNo":"ORDER-TASK","remark":"续费任务"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TenantRenewalTask(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskCreateCalls != 1 || store.renewalCalls != 0 {
		t.Fatalf("task creates=%d renewal=%d", store.taskCreateCalls, store.renewalCalls)
	}
	if store.lastTaskCreate.TaskType != SaaSAdminTaskTypeTenantRenewal ||
		store.lastTaskCreate.Status != SaaSAdminTaskStatusPending ||
		store.lastTaskCreate.TenantID != 12 ||
		store.lastTaskCreate.PackageCode != "growth" {
		t.Fatalf("task create = %+v", store.lastTaskCreate)
	}
	if !strings.Contains(store.lastTaskCreate.RequestJSON, `"packageCode":"growth"`) || !strings.Contains(store.lastTaskCreate.PreviewJSON, `"previousExpiresAt":"2027-01-02 00:00:00"`) {
		t.Fatalf("task json request=%s preview=%s", store.lastTaskCreate.RequestJSON, store.lastTaskCreate.PreviewJSON)
	}
	data := decodeSaaSAdminResponse(t, rec)
	task := data["task"].(map[string]any)
	result := data["result"].(map[string]any)
	if task["taskType"] != SaaSAdminTaskTypeTenantRenewal || task["status"] != SaaSAdminTaskStatusPending || result["blocked"] != false || result["packageCode"] != "growth" {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminTenantRenewalTaskApplyUsesStoredRequest(t *testing.T) {
	requestJSON := saasAdminPayloadJSON(saasAdminTenantRenewalTaskRequestPayload(SaaSAdminTenantRenewal{
		TenantID:        12,
		PackageCode:     "growth",
		ExpiresAt:       "2028-01-02 00:00:00",
		AmountCents:     12345,
		Currency:        "CNY",
		PaymentMethod:   "bank",
		ExternalOrderNo: "ORDER-TASK",
		Remark:          "任务应用续费",
	}))
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{{
			ID:          10,
			TaskType:    SaaSAdminTaskTypeTenantRenewal,
			Status:      SaaSAdminTaskStatusPending,
			Version:     4,
			TenantID:    12,
			PackageCode: "growth",
			RequestJSON: requestJSON,
			Remark:      "任务应用续费",
		}},
		packages: []SaaSAdminPackage{{
			Code:   "growth",
			Name:   "增长版",
			Status: 1,
		}},
		overview: SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{
			TenantID:    12,
			TenantName:  "租户B",
			PackageCode: "growth",
			PackageName: "增长版",
			ExpiresAt:   "2027-01-02 00:00:00",
		}}},
		renewalResult: SaaSAdminTenantRenewalResult{
			TenantID:          12,
			TenantName:        "租户B",
			PackageCode:       "growth",
			PackageName:       "增长版",
			PreviousExpiresAt: "2027-01-02 00:00:00",
			ExpiresAt:         "2028-01-02 00:00:00",
			AmountCents:       12345,
			Currency:          "CNY",
			BillingEventID:    1001,
			MetricsRefreshed:  26,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantRenewalTaskApply", strings.NewReader(`{"taskId":10}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TenantRenewalTaskApply(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskCalls != 1 || store.taskUpdateCalls != 0 || store.renewalCalls != 1 {
		t.Fatalf("calls task=%d taskUpdate=%d renewal=%d", store.taskCalls, store.taskUpdateCalls, store.renewalCalls)
	}
	if store.lastRenewal.TenantID != 12 || store.lastRenewal.PackageCode != "growth" || store.lastRenewal.ExpiresAt != "2028-01-02 00:00:00" || store.lastRenewal.ActorUserID != 1 {
		t.Fatalf("last renewal = %+v", store.lastRenewal)
	}
	if store.lastRenewal.TaskID != 10 || store.lastRenewal.ExpectedTaskVersion != 4 || len(store.lastRenewal.ExpectedTaskRequestSHA256) != 64 {
		t.Fatalf("task execution references = %+v", store.lastRenewal)
	}
	data := decodeSaaSAdminResponse(t, rec)
	task := data["task"].(map[string]any)
	result := data["result"].(map[string]any)
	if task["status"] != SaaSAdminTaskStatusApplied || result["billingEventId"].(float64) != 1001 || result["metricsRefreshed"].(float64) != 26 {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminTenantRenewalTaskBulkApplyAppliesAndReportsOutcomes(t *testing.T) {
	validRequest := saasAdminPayloadJSON(saasAdminTenantRenewalTaskRequestPayload(SaaSAdminTenantRenewal{
		TenantID:        12,
		PackageCode:     "growth",
		ExpiresAt:       "2028-01-02 00:00:00",
		AmountCents:     12345,
		Currency:        "CNY",
		PaymentMethod:   "bank",
		ExternalOrderNo: "ORDER-BULK",
		Remark:          "批量应用续费",
	}))
	blockedRequest := saasAdminPayloadJSON(saasAdminTenantRenewalTaskRequestPayload(SaaSAdminTenantRenewal{
		TenantID:    12,
		PackageCode: "growth",
		ExpiresAt:   "2026-01-01 00:00:00",
		Currency:    "CNY",
		Remark:      "会阻断",
	}))
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{
			{ID: 61, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusPending, TenantID: 12, PackageCode: "growth", RequestJSON: validRequest, Remark: "批量应用续费"},
			{ID: 62, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusPending, TenantID: 12, PackageCode: "growth", RequestJSON: blockedRequest, Remark: "会阻断"},
			{ID: 63, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusPending, TenantID: 12, PackageCode: "growth", RequestJSON: "{bad", Remark: "坏请求"},
			{ID: 64, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusApplied, TenantID: 12, PackageCode: "growth", RequestJSON: validRequest},
			{ID: 65, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusCanceled, TenantID: 12, PackageCode: "growth", RequestJSON: validRequest},
		},
		packages: []SaaSAdminPackage{{
			Code:   "growth",
			Name:   "增长版",
			Status: 1,
		}},
		overview: SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{
			TenantID:    12,
			TenantName:  "租户B",
			PackageCode: "growth",
			PackageName: "增长版",
			ExpiresAt:   "2027-01-02 00:00:00",
		}}},
		renewalResult: SaaSAdminTenantRenewalResult{
			TenantID:          12,
			TenantName:        "租户B",
			PackageCode:       "growth",
			PackageName:       "增长版",
			PreviousExpiresAt: "2027-01-02 00:00:00",
			ExpiresAt:         "2028-01-02 00:00:00",
			AmountCents:       12345,
			Currency:          "CNY",
			BillingEventID:    2001,
			MetricsRefreshed:  26,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantRenewalTaskBulkApply", strings.NewReader(`{"taskType":"tenant_renewal","status":"all","tenantId":12,"packageCode":"growth","limit":10,"remark":"批量应用续费"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TenantRenewalTaskBulkApply(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskCalls != 1 || store.taskUpdateCalls != 2 || store.renewalCalls != 1 {
		t.Fatalf("calls task=%d taskUpdate=%d renewal=%d", store.taskCalls, store.taskUpdateCalls, store.renewalCalls)
	}
	if store.lastTaskOptions.TaskType != SaaSAdminTaskTypeTenantRenewal ||
		store.lastTaskOptions.Status != "" ||
		store.lastTaskOptions.TenantID != 12 ||
		store.lastTaskOptions.PackageCode != "growth" ||
		store.lastTaskOptions.Limit != 10 {
		t.Fatalf("task options = %+v", store.lastTaskOptions)
	}
	statusByID := map[int64]string{}
	lastErrorByID := map[int64]string{}
	for _, item := range store.tasks {
		statusByID[item.ID] = item.Status
		lastErrorByID[item.ID] = item.LastError
	}
	if statusByID[61] != SaaSAdminTaskStatusApplied ||
		statusByID[62] != SaaSAdminTaskStatusBlocked ||
		statusByID[63] != SaaSAdminTaskStatusFailed ||
		statusByID[64] != SaaSAdminTaskStatusApplied ||
		statusByID[65] != SaaSAdminTaskStatusCanceled {
		t.Fatalf("status map = %+v", statusByID)
	}
	if !strings.Contains(lastErrorByID[62], "新到期时间早于当前到期时间") || !strings.Contains(lastErrorByID[63], "task request JSON 格式错误") {
		t.Fatalf("last errors = %+v", lastErrorByID)
	}
	if store.recordOperationLogCalls != 1 || len(store.operationLogs) != 1 {
		t.Fatalf("operation logs calls=%d logs=%d", store.recordOperationLogCalls, len(store.operationLogs))
	}
	seenBlock := false
	for _, operation := range store.operationLogs {
		if operation.TargetType != SaaSAdminOperationTargetAdminTask ||
			operation.TargetName != SaaSAdminTaskTypeTenantRenewal {
			t.Fatalf("operation = %+v", operation)
		}
		if !strings.Contains(operation.AfterJSON, `"bulkApply":true`) {
			t.Fatalf("operation after json = %s", operation.AfterJSON)
		}
		switch operation.Action {
		case SaaSAdminOperationActionTaskBlock:
			seenBlock = true
		default:
			t.Fatalf("operation action = %+v", operation)
		}
	}
	if !seenBlock {
		t.Fatalf("seen block=%v logs=%+v", seenBlock, store.operationLogs)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["applied"] != true ||
		data["matchedCount"].(float64) != 5 ||
		data["appliedCount"].(float64) != 1 ||
		data["blockedCount"].(float64) != 1 ||
		data["failedCount"].(float64) != 1 ||
		data["skippedAppliedCount"].(float64) != 1 ||
		data["skippedCanceledCount"].(float64) != 1 {
		t.Fatalf("data = %+v", data)
	}
	tasks := data["tasks"].([]any)
	if len(tasks) != 3 {
		t.Fatalf("tasks = %+v", tasks)
	}
	errors := data["errors"].([]any)
	if len(errors) != 1 || errors[0].(map[string]any)["taskId"].(float64) != 63 {
		t.Fatalf("errors = %+v", errors)
	}
}

func TestSaaSAdminTenantRenewalTaskBulkApplyRejectsTenantAdminAndWrongType(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantRenewalTaskBulkApply", strings.NewReader(`{"taskType":"tenant_renewal"}`))
	tenantReq.Header.Set("Content-Type", "application/json")
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.TenantRenewalTaskBulkApply(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
	if store.taskCalls != 0 || store.taskUpdateCalls != 0 {
		t.Fatalf("calls task=%d update=%d", store.taskCalls, store.taskUpdateCalls)
	}

	wrongTypeReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantRenewalTaskBulkApply", strings.NewReader(`{"taskType":"package_sync"}`))
	wrongTypeReq.Header.Set("Content-Type", "application/json")
	wrongTypeReq.Header.Set("X-Mochat-Go-User-ID", "1")
	wrongTypeRec := httptest.NewRecorder()
	handler.TenantRenewalTaskBulkApply(wrongTypeRec, wrongTypeReq)
	if wrongTypeRec.Code != http.StatusBadRequest {
		t.Fatalf("wrong type status = %d body=%s", wrongTypeRec.Code, wrongTypeRec.Body.String())
	}
	if store.taskCalls != 0 || store.taskUpdateCalls != 0 {
		t.Fatalf("calls after wrong type task=%d update=%d", store.taskCalls, store.taskUpdateCalls)
	}
}

func TestSaaSAdminRenewTenantRejectsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantRenewal", strings.NewReader(`{"tenantId":12,"expiresAt":"2028-01-02"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.RenewTenant(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.renewalCalls != 0 {
		t.Fatalf("renewal calls = %d", store.renewalCalls)
	}
}

func TestSaaSAdminRenewTenantRejectsInvalidAmount(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantRenewal", strings.NewReader(`{"tenantId":12,"expiresAt":"2028-01-02","amount":"12.345"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RenewTenant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.renewalCalls != 0 {
		t.Fatalf("renewal calls = %d", store.renewalCalls)
	}
}

func TestSaaSAdminRenewTenantRejectsTimestampOverflow(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantRenewal", strings.NewReader(`{"tenantId":12,"expiresAt":"2041-01-01"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RenewTenant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), saasAdminMySQLTimestampMax) {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if store.renewalCalls != 0 {
		t.Fatalf("renewal calls = %d", store.renewalCalls)
	}
}

func TestNormalizeSaaSAdminExpiresAtAcceptsSafeTimestampBoundary(t *testing.T) {
	got, err := normalizeSaaSAdminExpiresAt(saasAdminMySQLTimestampMax)
	if err != nil {
		t.Fatalf("normalize error = %v", err)
	}
	if got != saasAdminMySQLTimestampMax {
		t.Fatalf("normalized = %q", got)
	}
}

func TestSaaSAdminProvisionTenantAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		provisionResult: SaaSAdminTenantProvisionResult{
			TenantID:         88,
			TenantName:       "新租户",
			AdminUserID:      188,
			AdminPhone:       "13800138088",
			AdminName:        "租户管理员",
			RoleID:           288,
			RoleName:         "超级管理员",
			PackageCode:      "growth",
			PackageName:      "增长版",
			ExpiresAt:        "2028-07-09 00:00:00",
			MenuCount:        31,
			ConfigCopyCount:  4,
			MetricsRefreshed: 26,
			OperationID:      388,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "secret")
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantProvision", strings.NewReader(`{"tenantName":"新租户","adminPhone":"13800138088","adminName":"租户管理员","password":"abc123","roleName":"超级管理员","packageCode":"growth","expiresAt":"2028-07-09","configCopyMode":"missing","remark":"新客户开户"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ProvisionTenant(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.provisionCalls != 1 {
		t.Fatalf("provision calls = %d", store.provisionCalls)
	}
	if store.lastProvision.TenantName != "新租户" || store.lastProvision.AdminPhone != "13800138088" || store.lastProvision.PackageCode != "growth" || store.lastProvision.ExpiresAt != "2028-07-09 00:00:00" {
		t.Fatalf("last provision = %+v", store.lastProvision)
	}
	if store.lastProvision.AdminPasswordHash == "abc123" || !authjwt.CheckPasswordHash("secret", "abc123", store.lastProvision.AdminPasswordHash) {
		t.Fatalf("password hash invalid: %q", store.lastProvision.AdminPasswordHash)
	}
	if store.lastProvision.ActorUserID != 1 || store.lastProvision.ActorTenantID != 1 {
		t.Fatalf("actor = %+v", store.lastProvision)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["tenantId"].(float64) != 88 || data["adminUserId"].(float64) != 188 || data["metricsRefreshed"].(float64) != 26 {
		t.Fatalf("data = %+v", data)
	}
	if strings.Contains(rec.Body.String(), "abc123") {
		t.Fatalf("response leaked password: %s", rec.Body.String())
	}
}

func TestSaaSAdminTenantProvisionTaskCreatesPendingPreviewWithoutPasswordLeak(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		packages: []SaaSAdminPackage{{
			Code:   "growth",
			Name:   "增长版",
			Status: 1,
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "secret")
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantProvisionTask", strings.NewReader(`{"tenantName":"任务租户","adminPhone":"13800138089","adminName":"任务管理员","password":"abc123","roleName":"超级管理员","packageCode":"growth","expiresAt":"2028-07-09","configCopyMode":"missing","remark":"任务开户"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TenantProvisionTask(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskCreateCalls != 1 || store.provisionCalls != 0 {
		t.Fatalf("task creates=%d provision=%d", store.taskCreateCalls, store.provisionCalls)
	}
	if store.lastTaskCreate.TaskType != SaaSAdminTaskTypeTenantProvision || store.lastTaskCreate.Status != SaaSAdminTaskStatusPending || store.lastTaskCreate.PackageCode != "growth" {
		t.Fatalf("task create = %+v", store.lastTaskCreate)
	}
	if strings.Contains(store.lastTaskCreate.RequestJSON, "abc123") || !strings.Contains(store.lastTaskCreate.RequestJSON, `"adminPasswordHash"`) {
		t.Fatalf("task request not sanitized correctly: %s", store.lastTaskCreate.RequestJSON)
	}
	if !strings.Contains(store.lastTaskCreate.PreviewJSON, `"tenantName":"任务租户"`) {
		t.Fatalf("preview json = %s", store.lastTaskCreate.PreviewJSON)
	}
	body := rec.Body.String()
	if strings.Contains(body, "abc123") || strings.Contains(body, "adminPasswordHash") {
		t.Fatalf("response leaked password data: %s", body)
	}
	data := decodeSaaSAdminResponse(t, rec)
	task := data["task"].(map[string]any)
	request := task["request"].(map[string]any)
	result := data["result"].(map[string]any)
	if task["taskType"] != SaaSAdminTaskTypeTenantProvision || task["status"] != SaaSAdminTaskStatusPending || request["hasAdminPasswordHash"] != true || result["packageCode"] != "growth" {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminTenantProvisionTaskApplyUsesStoredHash(t *testing.T) {
	hash, err := authjwt.GeneratePasswordHash("secret", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	requestJSON := saasAdminPayloadJSON(saasAdminTenantProvisionTaskRequestPayload(SaaSAdminTenantProvision{
		TenantName:        "任务租户",
		AdminPhone:        "13800138089",
		AdminName:         "任务管理员",
		AdminPasswordHash: hash,
		RoleName:          "超级管理员",
		PackageCode:       "growth",
		ExpiresAt:         "2028-07-09 00:00:00",
		ConfigCopyMode:    "missing",
		Remark:            "任务开户",
	}))
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{{
			ID:          11,
			TaskType:    SaaSAdminTaskTypeTenantProvision,
			Status:      SaaSAdminTaskStatusPending,
			PackageCode: "growth",
			RequestJSON: requestJSON,
			Remark:      "任务开户",
		}},
		packages: []SaaSAdminPackage{{
			Code:   "growth",
			Name:   "增长版",
			Status: 1,
		}},
		provisionResult: SaaSAdminTenantProvisionResult{
			TenantID:         88,
			TenantName:       "任务租户",
			AdminUserID:      188,
			AdminPhone:       "13800138089",
			AdminName:        "任务管理员",
			RoleID:           288,
			RoleName:         "超级管理员",
			PackageCode:      "growth",
			PackageName:      "增长版",
			ExpiresAt:        "2028-07-09 00:00:00",
			MenuCount:        31,
			ConfigCopyCount:  4,
			MetricsRefreshed: 26,
			OperationID:      388,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "secret")
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantProvisionTaskApply", strings.NewReader(`{"taskId":11}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TenantProvisionTaskApply(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskCalls != 1 || store.taskUpdateCalls != 1 || store.provisionCalls != 1 {
		t.Fatalf("calls task=%d taskUpdate=%d provision=%d", store.taskCalls, store.taskUpdateCalls, store.provisionCalls)
	}
	if store.lastProvision.TenantName != "任务租户" || store.lastProvision.AdminPhone != "13800138089" || store.lastProvision.PackageCode != "growth" || store.lastProvision.ActorUserID != 1 || store.lastProvision.ActorTenantID != 1 {
		t.Fatalf("last provision = %+v", store.lastProvision)
	}
	if store.lastProvision.AdminPasswordHash != hash || !authjwt.CheckPasswordHash("secret", "abc123", store.lastProvision.AdminPasswordHash) {
		t.Fatalf("password hash invalid: %q", store.lastProvision.AdminPasswordHash)
	}
	if store.lastTaskStatusUpdate.TaskID != 11 || store.lastTaskStatusUpdate.Status != SaaSAdminTaskStatusApplied || !store.lastTaskStatusUpdate.Applied {
		t.Fatalf("task update = %+v", store.lastTaskStatusUpdate)
	}
	if store.recordOperationLogCalls != 1 {
		t.Fatalf("record operation log calls = %d", store.recordOperationLogCalls)
	}
	operation := store.lastRecordedOperationLog
	if operation.Action != SaaSAdminOperationActionTaskApply ||
		operation.TargetType != SaaSAdminOperationTargetAdminTask ||
		operation.TargetID != "11" ||
		operation.TargetName != SaaSAdminTaskTypeTenantProvision ||
		operation.Remark != "任务开户" {
		t.Fatalf("operation = %+v", operation)
	}
	if !strings.Contains(operation.BeforeJSON, `"status":"pending"`) || !strings.Contains(operation.AfterJSON, `"status":"applied"`) {
		t.Fatalf("operation json before=%s after=%s", operation.BeforeJSON, operation.AfterJSON)
	}
	body := rec.Body.String()
	if strings.Contains(body, "abc123") || strings.Contains(body, "adminPasswordHash") {
		t.Fatalf("response leaked password data: %s", body)
	}
	data := decodeSaaSAdminResponse(t, rec)
	task := data["task"].(map[string]any)
	result := data["result"].(map[string]any)
	if task["status"] != SaaSAdminTaskStatusApplied || result["tenantId"].(float64) != 88 || result["metricsRefreshed"].(float64) != 26 {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAdminTenantProvisionTaskBulkApplyAppliesAndReportsOutcomes(t *testing.T) {
	hash, err := authjwt.GeneratePasswordHash("secret", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	validRequest := saasAdminPayloadJSON(saasAdminTenantProvisionTaskRequestPayload(SaaSAdminTenantProvision{
		TenantName:        "批量开户租户",
		AdminPhone:        "13800138089",
		AdminName:         "批量管理员",
		AdminPasswordHash: hash,
		RoleName:          "超级管理员",
		PackageCode:       "growth",
		ExpiresAt:         "2028-07-09 00:00:00",
		ConfigCopyMode:    "missing",
		Remark:            "批量平台开户",
	}))
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
		tasks: []SaaSAdminTask{
			{ID: 71, TaskType: SaaSAdminTaskTypeTenantProvision, Status: SaaSAdminTaskStatusPending, PackageCode: "growth", RequestJSON: validRequest, Remark: "批量平台开户"},
			{ID: 72, TaskType: SaaSAdminTaskTypeTenantProvision, Status: SaaSAdminTaskStatusPending, PackageCode: "growth", RequestJSON: "{bad", Remark: "坏请求"},
			{ID: 73, TaskType: SaaSAdminTaskTypeTenantProvision, Status: SaaSAdminTaskStatusApplied, PackageCode: "growth", RequestJSON: validRequest},
			{ID: 74, TaskType: SaaSAdminTaskTypeTenantProvision, Status: SaaSAdminTaskStatusCanceled, PackageCode: "growth", RequestJSON: validRequest},
		},
		packages: []SaaSAdminPackage{{
			Code:   "growth",
			Name:   "增长版",
			Status: 1,
		}},
		provisionResult: SaaSAdminTenantProvisionResult{
			TenantID:         88,
			TenantName:       "批量开户租户",
			AdminUserID:      188,
			AdminPhone:       "13800138089",
			AdminName:        "批量管理员",
			RoleID:           288,
			RoleName:         "超级管理员",
			PackageCode:      "growth",
			PackageName:      "增长版",
			ExpiresAt:        "2028-07-09 00:00:00",
			MenuCount:        31,
			ConfigCopyCount:  4,
			MetricsRefreshed: 26,
			OperationID:      388,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "secret")
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantProvisionTaskBulkApply", strings.NewReader(`{"status":"all","packageCode":"growth","limit":10,"remark":"批量应用开户"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TenantProvisionTaskBulkApply(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.taskCalls != 1 || store.taskUpdateCalls != 2 || store.provisionCalls != 1 {
		t.Fatalf("calls task=%d taskUpdate=%d provision=%d", store.taskCalls, store.taskUpdateCalls, store.provisionCalls)
	}
	if store.lastTaskOptions.TaskType != SaaSAdminTaskTypeTenantProvision ||
		store.lastTaskOptions.Status != "" ||
		store.lastTaskOptions.PackageCode != "growth" ||
		store.lastTaskOptions.Limit != 10 {
		t.Fatalf("task options = %+v", store.lastTaskOptions)
	}
	if store.lastProvision.TenantName != "批量开户租户" ||
		store.lastProvision.AdminPhone != "13800138089" ||
		store.lastProvision.AdminPasswordHash != hash ||
		store.lastProvision.ActorUserID != 1 ||
		store.lastProvision.ActorTenantID != 1 {
		t.Fatalf("last provision = %+v", store.lastProvision)
	}
	statusByID := map[int64]string{}
	lastErrorByID := map[int64]string{}
	for _, item := range store.tasks {
		statusByID[item.ID] = item.Status
		lastErrorByID[item.ID] = item.LastError
	}
	if statusByID[71] != SaaSAdminTaskStatusApplied ||
		statusByID[72] != SaaSAdminTaskStatusFailed ||
		statusByID[73] != SaaSAdminTaskStatusApplied ||
		statusByID[74] != SaaSAdminTaskStatusCanceled {
		t.Fatalf("status map = %+v", statusByID)
	}
	if !strings.Contains(lastErrorByID[72], "task request JSON 格式错误") {
		t.Fatalf("last errors = %+v", lastErrorByID)
	}
	if store.recordOperationLogCalls != 1 || len(store.operationLogs) != 1 {
		t.Fatalf("operation logs calls=%d logs=%d", store.recordOperationLogCalls, len(store.operationLogs))
	}
	operation := store.operationLogs[0]
	if operation.Action != SaaSAdminOperationActionTaskApply ||
		operation.TargetType != SaaSAdminOperationTargetAdminTask ||
		operation.TargetID != "71" ||
		operation.TargetName != SaaSAdminTaskTypeTenantProvision ||
		operation.Remark != "批量应用开户" {
		t.Fatalf("operation = %+v", operation)
	}
	if !strings.Contains(operation.AfterJSON, `"bulkApply":true`) {
		t.Fatalf("operation after json = %s", operation.AfterJSON)
	}
	body := rec.Body.String()
	if strings.Contains(body, "abc123") || strings.Contains(body, hash) || strings.Contains(body, "adminPasswordHash") {
		t.Fatalf("response leaked password data: %s", body)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["applied"] != true ||
		data["matchedCount"].(float64) != 4 ||
		data["appliedCount"].(float64) != 1 ||
		data["blockedCount"].(float64) != 0 ||
		data["failedCount"].(float64) != 1 ||
		data["skippedAppliedCount"].(float64) != 1 ||
		data["skippedCanceledCount"].(float64) != 1 {
		t.Fatalf("data = %+v", data)
	}
	tasks := data["tasks"].([]any)
	if len(tasks) != 2 {
		t.Fatalf("tasks = %+v", tasks)
	}
	errors := data["errors"].([]any)
	if len(errors) != 1 || errors[0].(map[string]any)["taskId"].(float64) != 72 {
		t.Fatalf("errors = %+v", errors)
	}
}

func TestSaaSAdminTenantProvisionTaskBulkApplyRejectsTenantAdminAndWrongType(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "secret")

	tenantReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantProvisionTaskBulkApply", strings.NewReader(`{"taskType":"tenant_provision"}`))
	tenantReq.Header.Set("Content-Type", "application/json")
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.TenantProvisionTaskBulkApply(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status = %d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
	if store.taskCalls != 0 || store.taskUpdateCalls != 0 {
		t.Fatalf("calls task=%d update=%d", store.taskCalls, store.taskUpdateCalls)
	}

	wrongTypeReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantProvisionTaskBulkApply", strings.NewReader(`{"taskType":"tenant_renewal"}`))
	wrongTypeReq.Header.Set("Content-Type", "application/json")
	wrongTypeReq.Header.Set("X-Mochat-Go-User-ID", "1")
	wrongTypeRec := httptest.NewRecorder()
	handler.TenantProvisionTaskBulkApply(wrongTypeRec, wrongTypeReq)
	if wrongTypeRec.Code != http.StatusBadRequest {
		t.Fatalf("wrong type status = %d body=%s", wrongTypeRec.Code, wrongTypeRec.Body.String())
	}
	if store.taskCalls != 0 || store.taskUpdateCalls != 0 {
		t.Fatalf("calls after wrong type task=%d update=%d", store.taskCalls, store.taskUpdateCalls)
	}
}

func TestSaaSAdminProvisionTenantRejectsTenantAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			7: {ID: 7, TenantID: 10, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "secret")
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantProvision", strings.NewReader(`{"tenantName":"新租户","adminPhone":"13800138088","password":"abc123","packageCode":"growth"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.ProvisionTenant(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.provisionCalls != 0 {
		t.Fatalf("provision calls = %d", store.provisionCalls)
	}
}

func TestSaaSAdminProvisionTenantRejectsInvalidPhone(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "secret")
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantProvision", strings.NewReader(`{"tenantName":"新租户","adminPhone":"bad","password":"abc123","packageCode":"growth"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ProvisionTenant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.provisionCalls != 0 {
		t.Fatalf("provision calls = %d", store.provisionCalls)
	}
}

type fakeSaaSAdminStore struct {
	users                          map[int]User
	overview                       SaaSAdminOverview
	packages                       []SaaSAdminPackage
	usageMetrics                   []SaaSAdminUsageMetric
	usageByTenant                  map[int][]SaaSAdminUsageMetric
	alertPage                      SaaSAlertListPage
	notifications                  []SaaSAlertNotification
	notificationHealthSource       SaaSAdminNotificationHealthSource
	notificationSLOSource          SaaSAdminNotificationSLOSource
	notificationPolicyReport       SaaSAdminNotificationPolicyReport
	alertSettings                  map[string]SaaSAlertSetting
	lastSavedAlertSetting          SaaSAlertSetting
	lastEnqueuedAlert              SaaSQuotaAlert
	lastEnqueuedChannel            string
	lastEnqueuedMaxAttempts        int
	operationLogs                  []SaaSAdminOperationLog
	billingEvents                  []SaaSAdminBillingEvent
	billingReconciliationItems     []SaaSAdminBillingReconciliationItem
	billingFollowUpSnapshots       []SaaSAdminBillingReconciliationFollowUpSnapshot
	tasks                          []SaaSAdminTask
	latestRiskFollowUps            map[int]SaaSAdminRiskFollowUpSnapshot
	riskFollowUpSnapshots          []SaaSAdminRiskFollowUpSnapshot
	alertResolveResult             SaaSAdminAlertResolveResult
	alertBulkResult                SaaSAdminAlertBulkResolveResult
	notificationRetryResult        SaaSAdminAlertNotificationRetryResult
	notificationBulkResult         SaaSAdminAlertNotificationBulkRetryResult
	notificationCloseResult        SaaSAdminAlertNotificationCloseResult
	notificationBulkCloseResult    SaaSAdminAlertNotificationBulkCloseResult
	upsertPackageResult            SaaSAdminPackage
	riskFollowUpResult             SaaSAdminRiskFollowUpResult
	statusResult                   SaaSAdminTenantStatusUpdateResult
	tenantStatusPlan               SaaSAdminTenantStatusApprovalPlan
	tenantStatusPlanErr            error
	updateResult                   SaaSAdminTenantPackageUpdateResult
	renewalResult                  SaaSAdminTenantRenewalResult
	provisionResult                SaaSAdminTenantProvisionResult
	subscriptionReport             SaaSAdminSubscriptionReport
	subscriptionEvents             []SaaSAdminSubscriptionEvent
	subscriptionTransitionResult   SaaSAdminSubscriptionTransitionResult
	subscriptionReconcileResult    SaaSAdminSubscriptionReconcileResult
	lastOptions                    SaaSAdminOverviewOptions
	lastAlertOptions               SaaSAlertListOptions
	lastAlertSummaryOptions        SaaSAlertListOptions
	lastNotificationOptions        SaaSAdminAlertNotificationOptions
	lastNotificationSummaryOptions SaaSAdminAlertNotificationOptions
	lastNotificationHealthOptions  SaaSAdminNotificationHealthOptions
	lastNotificationSLOOptions     SaaSAdminNotificationSLOOptions
	lastNotificationPolicyOptions  SaaSAdminNotificationPolicyOptions
	lastOperationOptions           SaaSAdminOperationLogOptions
	lastOperationSummaryOptions    SaaSAdminOperationLogOptions
	lastBillingOptions             SaaSAdminBillingEventOptions
	lastBillingSummaryOptions      SaaSAdminBillingEventOptions
	lastBillingReconcileOptions    SaaSAdminBillingReconciliationOptions
	lastBillingFollowUpOptions     SaaSAdminBillingReconciliationFollowUpOptions
	lastBillingEventByID           int64
	lastTaskOptions                SaaSAdminTaskOptions
	lastRiskFollowUpTaskOptions    SaaSAdminRiskFollowUpTaskOptions
	lastRiskFollowUpTenantIDs      []int
	lastAlertResolve               SaaSAdminAlertResolve
	lastAlertBulk                  SaaSAdminAlertBulkResolve
	lastNotificationRetry          SaaSAdminAlertNotificationRetry
	lastNotificationBulk           SaaSAdminAlertNotificationBulkRetry
	lastNotificationClose          SaaSAdminAlertNotificationClose
	lastNotificationBulkClose      SaaSAdminAlertNotificationBulkClose
	lastRecordedOperationLog       SaaSAdminOperationLog
	lastPackageUpsert              SaaSAdminPackageUpsert
	lastTaskCreate                 SaaSAdminTaskCreate
	lastTaskStatusUpdate           SaaSAdminTaskStatusUpdate
	lastRiskFollowUp               SaaSAdminRiskFollowUp
	riskFollowUps                  []SaaSAdminRiskFollowUp
	lastStatusUpdate               SaaSAdminTenantStatusUpdate
	lastUpdate                     SaaSAdminTenantPackageUpdate
	lastRenewal                    SaaSAdminTenantRenewal
	lastProvision                  SaaSAdminTenantProvision
	lastSubscriptionOptions        SaaSAdminSubscriptionOptions
	lastSubscriptionEventOptions   SaaSAdminSubscriptionEventOptions
	lastSubscriptionTransition     SaaSAdminSubscriptionTransition
	lastSubscriptionReconcile      SaaSAdminSubscriptionReconcile
	lastUsageTenantID              int
	overviewCalls                  int
	packageCalls                   int
	usageCalls                     int
	alertCalls                     int
	alertSummaryCalls              int
	alertResolveCalls              int
	alertBulkCalls                 int
	notificationCalls              int
	notificationSummaryCalls       int
	notificationHealthCalls        int
	notificationSLOCalls           int
	notificationPolicyCalls        int
	alertSettingSaveCalls          int
	notificationEnqueueCalls       int
	notificationRetryCalls         int
	notificationBulkCalls          int
	notificationCloseCalls         int
	notificationBulkCloseCalls     int
	operationCalls                 int
	operationSummaryCalls          int
	recordOperationLogCalls        int
	billingCalls                   int
	billingSummaryCalls            int
	billingReconcileCalls          int
	billingFollowUpCalls           int
	billingEventByIDCalls          int
	taskCalls                      int
	taskCreateCalls                int
	taskUpdateCalls                int
	latestRiskFollowUpCalls        int
	riskFollowUpSnapshotCalls      int
	upsertPackageCalls             int
	riskFollowUpCalls              int
	statusCalls                    int
	tenantStatusPlanCalls          int
	updateCalls                    int
	renewalCalls                   int
	provisionCalls                 int
	subscriptionCalls              int
	subscriptionEventCalls         int
	subscriptionTransitionCalls    int
	subscriptionReconcileCalls     int
}

func (s *fakeSaaSAdminStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminOverview(_ context.Context, options SaaSAdminOverviewOptions) (SaaSAdminOverview, error) {
	s.lastOptions = options
	s.overviewCalls++
	return s.overview, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminPackages(_ context.Context) ([]SaaSAdminPackage, error) {
	s.packageCalls++
	return s.packages, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminTenantUsage(_ context.Context, tenantID int) ([]SaaSAdminUsageMetric, error) {
	s.lastUsageTenantID = tenantID
	s.usageCalls++
	if s.usageByTenant != nil {
		return s.usageByTenant[tenantID], nil
	}
	return s.usageMetrics, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminAlertSummary(_ context.Context, options SaaSAlertListOptions) (SaaSAdminAlertSummary, error) {
	s.lastAlertSummaryOptions = options
	s.alertSummaryCalls++
	items := s.matchingAlerts(options)
	summary := SaaSAdminAlertSummary{AlertCount: len(items)}
	tenants := map[int]bool{}
	metrics := map[string]bool{}
	for _, item := range items {
		switch item.Status {
		case SaaSAlertStatusOpen:
			summary.OpenCount++
		case SaaSAlertStatusResolved:
			summary.ResolvedCount++
		}
		switch item.Severity {
		case SaaSAlertSeverityWarning:
			summary.WarningCount++
		case SaaSAlertSeverityCritical:
			summary.CriticalCount++
		}
		if item.TenantID > 0 {
			tenants[item.TenantID] = true
		}
		if item.Metric != "" {
			metrics[item.Metric] = true
		}
	}
	summary.TenantCount = len(tenants)
	summary.MetricCount = len(metrics)
	return summary, nil
}

func (s *fakeSaaSAdminStore) ListSaaSAlerts(_ context.Context, options SaaSAlertListOptions) (SaaSAlertListPage, error) {
	s.lastAlertOptions = options
	s.alertCalls++
	return s.alertPage, nil
}

func (s *fakeSaaSAdminStore) matchingAlerts(options SaaSAlertListOptions) []SaaSAlertRecord {
	items := make([]SaaSAlertRecord, 0, len(s.alertPage.Items))
	for _, item := range s.alertPage.Items {
		if options.TenantID > 0 && item.TenantID != options.TenantID {
			continue
		}
		if options.Status != "" && item.Status != options.Status {
			continue
		}
		if options.Metric != "" && item.Metric != options.Metric {
			continue
		}
		if options.AlertType != "" && item.AlertType != options.AlertType {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (s *fakeSaaSAdminStore) ResolveSaaSAdminAlert(_ context.Context, resolve SaaSAdminAlertResolve) (SaaSAdminAlertResolveResult, error) {
	s.lastAlertResolve = resolve
	s.alertResolveCalls++
	if s.alertResolveResult.Metric != "" || s.alertResolveResult.TenantID > 0 {
		return s.alertResolveResult, nil
	}
	return SaaSAdminAlertResolveResult{
		Resolved:       true,
		TenantID:       resolve.TenantID,
		Metric:         resolve.Metric,
		AlertType:      resolve.AlertType,
		PeriodKey:      resolve.PeriodKey,
		PreviousStatus: SaaSAlertStatusOpen,
		Status:         SaaSAlertStatusResolved,
		Remark:         resolve.Remark,
	}, nil
}

func (s *fakeSaaSAdminStore) BulkResolveSaaSAdminAlerts(_ context.Context, resolve SaaSAdminAlertBulkResolve) (SaaSAdminAlertBulkResolveResult, error) {
	s.lastAlertBulk = resolve
	s.alertBulkCalls++
	if s.alertBulkResult.ResolvedCount > 0 || len(s.alertBulkResult.Alerts) > 0 {
		return s.alertBulkResult, nil
	}
	return SaaSAdminAlertBulkResolveResult{
		ResolvedCount: 1,
		TenantID:      resolve.TenantID,
		Metric:        resolve.Metric,
		AlertType:     resolve.AlertType,
		PeriodKey:     resolve.PeriodKey,
		Limit:         resolve.Limit,
		Remark:        resolve.Remark,
		Alerts: []SaaSAdminAlertResolveResult{{
			Resolved:       true,
			TenantID:       resolve.TenantID,
			Metric:         resolve.Metric,
			AlertType:      resolve.AlertType,
			PeriodKey:      resolve.PeriodKey,
			PreviousStatus: SaaSAlertStatusOpen,
			Status:         SaaSAlertStatusResolved,
			Remark:         resolve.Remark,
		}},
	}, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminAlertNotifications(_ context.Context, options SaaSAdminAlertNotificationOptions) ([]SaaSAlertNotification, error) {
	s.lastNotificationOptions = options
	s.notificationCalls++
	return s.matchingNotifications(options, true), nil
}

func (s *fakeSaaSAdminStore) SaaSAdminAlertNotificationSummary(_ context.Context, options SaaSAdminAlertNotificationOptions) (SaaSAdminAlertNotificationSummary, error) {
	s.lastNotificationSummaryOptions = options
	s.notificationSummaryCalls++
	items := s.matchingNotifications(options, false)
	summary := SaaSAdminAlertNotificationSummary{NotificationCount: len(items)}
	tenants := map[int]bool{}
	channels := map[string]bool{}
	for _, item := range items {
		switch item.Status {
		case SaaSAlertNotificationStatusPending:
			summary.PendingCount++
		case SaaSAlertNotificationStatusFailed:
			summary.FailedCount++
			summary.RetryableCount++
		case SaaSAlertNotificationStatusDelivered:
			summary.DeliveredCount++
		case SaaSAlertNotificationStatusDead:
			summary.DeadCount++
			summary.RetryableCount++
		case SaaSAlertNotificationStatusClosed:
			summary.ClosedCount++
		case SaaSAlertNotificationStatusSuppressed:
			summary.SuppressedCount++
		}
		if item.TenantID > 0 {
			tenants[item.TenantID] = true
		}
		if item.Channel != "" {
			channels[item.Channel] = true
		}
	}
	summary.TenantCount = len(tenants)
	summary.ChannelCount = len(channels)
	return summary, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminNotificationHealth(_ context.Context, options SaaSAdminNotificationHealthOptions) (SaaSAdminNotificationHealthSource, error) {
	s.lastNotificationHealthOptions = options
	s.notificationHealthCalls++
	source := s.notificationHealthSource
	if source.Tenants == nil {
		source.Tenants = []SaaSAdminNotificationHealthSnapshot{}
	}
	if source.FailureReasons == nil {
		source.FailureReasons = []SaaSAdminNotificationHealthFailureSnapshot{}
	}
	return source, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminNotificationSLO(_ context.Context, options SaaSAdminNotificationSLOOptions) (SaaSAdminNotificationSLOSource, error) {
	s.lastNotificationSLOOptions = options
	s.notificationSLOCalls++
	source := s.notificationSLOSource
	if source.Days == nil {
		source.Days = []SaaSAdminNotificationSLODaySnapshot{}
	}
	if source.Tenants == nil {
		source.Tenants = []SaaSAdminNotificationSLOTenantSnapshot{}
	}
	return source, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminNotificationPolicies(_ context.Context, options SaaSAdminNotificationPolicyOptions) (SaaSAdminNotificationPolicyReport, error) {
	s.lastNotificationPolicyOptions = options
	s.notificationPolicyCalls++
	report := s.notificationPolicyReport
	report.Options = options
	if report.Policies == nil {
		report.Policies = []SaaSAdminNotificationPolicy{}
	}
	return report, nil
}

func (s *fakeSaaSAdminStore) GetSaaSAlertSetting(_ context.Context, tenantID int, channel string) (SaaSAlertSetting, bool, error) {
	if channel == "" {
		channel = SaaSAlertNotificationChannelWebhook
	}
	setting, found := s.alertSettings[fmt.Sprintf("%d:%s", tenantID, channel)]
	if !found {
		return DefaultSaaSAlertSetting(tenantID, channel), false, nil
	}
	return NormalizeSaaSAlertSetting(setting), true, nil
}

func (s *fakeSaaSAdminStore) SaveSaaSAlertSetting(_ context.Context, setting SaaSAlertSetting) (SaaSAlertSetting, error) {
	setting = NormalizeSaaSAlertSetting(setting)
	if setting.ID <= 0 {
		setting.ID = 1001
	}
	setting.UpdatedAt = "2026-07-10 13:00:00"
	if setting.CreatedAt == "" {
		setting.CreatedAt = setting.UpdatedAt
	}
	if s.alertSettings == nil {
		s.alertSettings = map[string]SaaSAlertSetting{}
	}
	s.alertSettings[fmt.Sprintf("%d:%s", setting.TenantID, setting.Channel)] = setting
	s.lastSavedAlertSetting = setting
	s.alertSettingSaveCalls++
	return setting, nil
}

func (s *fakeSaaSAdminStore) EnqueueSaaSAlertNotification(_ context.Context, alert SaaSQuotaAlert, channel string, maxAttempts int) (SaaSAlertNotification, error) {
	s.lastEnqueuedAlert = alert
	s.lastEnqueuedChannel = channel
	s.lastEnqueuedMaxAttempts = maxAttempts
	s.notificationEnqueueCalls++
	key := SaaSAlertNotificationKey(alert, channel)
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	for i, item := range s.notifications {
		if item.NotificationKey != key {
			continue
		}
		item.AlertKey = fmt.Sprintf("%d:%s:%s:%s", alert.Status.TenantID, strings.TrimSpace(alert.Status.Metric), strings.TrimSpace(alert.AlertType), strings.TrimSpace(alert.PeriodKey))
		item.TenantID = alert.Status.TenantID
		item.Channel = channel
		item.Status = SaaSAlertNotificationStatusPending
		item.Attempts = 0
		item.MaxAttempts = maxAttempts
		item.Alert = alert
		item.LastError = ""
		item.NextRetryAt = "now"
		item.DeliveredAt = ""
		s.notifications[i] = item
		return item, nil
	}
	notification := SaaSAlertNotification{
		ID:              int64(1000 + len(s.notifications) + 1),
		NotificationKey: key,
		AlertKey:        fmt.Sprintf("%d:%s:%s:%s", alert.Status.TenantID, strings.TrimSpace(alert.Status.Metric), strings.TrimSpace(alert.AlertType), strings.TrimSpace(alert.PeriodKey)),
		TenantID:        alert.Status.TenantID,
		Channel:         channel,
		Status:          SaaSAlertNotificationStatusPending,
		MaxAttempts:     maxAttempts,
		Alert:           alert,
		NextRetryAt:     "now",
		CreatedAt:       "2026-07-10 10:00:00",
		UpdatedAt:       "2026-07-10 10:00:00",
	}
	s.notifications = append(s.notifications, notification)
	return notification, nil
}

func (s *fakeSaaSAdminStore) SaaSAlertNotificationByKey(_ context.Context, notificationKey string) (SaaSAlertNotification, error) {
	for _, notification := range s.notifications {
		if notification.NotificationKey == notificationKey {
			return notification, nil
		}
	}
	return SaaSAlertNotification{}, nil
}

func (s *fakeSaaSAdminStore) matchingNotifications(options SaaSAdminAlertNotificationOptions, applyLimit bool) []SaaSAlertNotification {
	items := make([]SaaSAlertNotification, 0, len(s.notifications))
	keyword := strings.ToLower(strings.TrimSpace(options.Keyword))
	for _, item := range s.notifications {
		if options.TenantID > 0 && item.TenantID != options.TenantID {
			continue
		}
		if options.Status != "" && item.Status != options.Status {
			continue
		}
		if options.Channel != "" && item.Channel != options.Channel {
			continue
		}
		if keyword != "" {
			haystack := strings.ToLower(strings.Join([]string{
				item.NotificationKey,
				item.AlertKey,
				item.Channel,
				item.Status,
				item.LastError,
				item.Alert.Status.Metric,
				item.Alert.AlertType,
				item.Alert.Message,
			}, "\n"))
			if !strings.Contains(haystack, keyword) {
				continue
			}
		}
		items = append(items, item)
	}
	if applyLimit && options.Limit > 0 && len(items) > options.Limit {
		return items[:options.Limit]
	}
	return items
}

func (s *fakeSaaSAdminStore) RetrySaaSAdminAlertNotification(_ context.Context, retry SaaSAdminAlertNotificationRetry) (SaaSAdminAlertNotificationRetryResult, error) {
	s.lastNotificationRetry = retry
	s.notificationRetryCalls++
	if s.notificationRetryResult.NotificationID > 0 || s.notificationRetryResult.TenantID > 0 {
		return s.notificationRetryResult, nil
	}
	return SaaSAdminAlertNotificationRetryResult{
		Retried:        true,
		NotificationID: retry.NotificationID,
		TenantID:       retry.AllowedTenantID,
		PreviousStatus: SaaSAlertNotificationStatusFailed,
		Status:         SaaSAlertNotificationStatusPending,
		Remark:         retry.Remark,
	}, nil
}

func (s *fakeSaaSAdminStore) BulkRetrySaaSAdminAlertNotifications(_ context.Context, retry SaaSAdminAlertNotificationBulkRetry) (SaaSAdminAlertNotificationBulkRetryResult, error) {
	s.lastNotificationBulk = retry
	s.notificationBulkCalls++
	if s.notificationBulkResult.RetriedCount > 0 || len(s.notificationBulkResult.Notifications) > 0 {
		return s.notificationBulkResult, nil
	}
	return SaaSAdminAlertNotificationBulkRetryResult{
		RetriedCount: 1,
		TenantID:     retry.TenantID,
		Status:       retry.Status,
		Channel:      retry.Channel,
		Keyword:      retry.Keyword,
		Limit:        retry.Limit,
		Remark:       retry.Remark,
		Notifications: []SaaSAdminAlertNotificationRetryResult{{
			Retried:        true,
			NotificationID: 1,
			TenantID:       retry.TenantID,
			PreviousStatus: SaaSAlertNotificationStatusFailed,
			Status:         SaaSAlertNotificationStatusPending,
			Remark:         retry.Remark,
		}},
	}, nil
}

func (s *fakeSaaSAdminStore) CloseSaaSAdminAlertNotification(_ context.Context, closeReq SaaSAdminAlertNotificationClose) (SaaSAdminAlertNotificationCloseResult, error) {
	s.lastNotificationClose = closeReq
	s.notificationCloseCalls++
	if s.notificationCloseResult.NotificationID > 0 || s.notificationCloseResult.TenantID > 0 {
		return s.notificationCloseResult, nil
	}
	return SaaSAdminAlertNotificationCloseResult{
		Closed:         true,
		NotificationID: closeReq.NotificationID,
		TenantID:       closeReq.AllowedTenantID,
		PreviousStatus: SaaSAlertNotificationStatusPending,
		Status:         SaaSAlertNotificationStatusClosed,
		Remark:         closeReq.Remark,
	}, nil
}

func (s *fakeSaaSAdminStore) BulkCloseSaaSAdminAlertNotifications(_ context.Context, closeReq SaaSAdminAlertNotificationBulkClose) (SaaSAdminAlertNotificationBulkCloseResult, error) {
	s.lastNotificationBulkClose = closeReq
	s.notificationBulkCloseCalls++
	if s.notificationBulkCloseResult.ClosedCount > 0 || len(s.notificationBulkCloseResult.Notifications) > 0 {
		return s.notificationBulkCloseResult, nil
	}
	return SaaSAdminAlertNotificationBulkCloseResult{
		ClosedCount: 1,
		TenantID:    closeReq.TenantID,
		Status:      closeReq.Status,
		Channel:     closeReq.Channel,
		Keyword:     closeReq.Keyword,
		Limit:       closeReq.Limit,
		Remark:      closeReq.Remark,
		Notifications: []SaaSAdminAlertNotificationCloseResult{{
			Closed:         true,
			NotificationID: 1,
			TenantID:       closeReq.TenantID,
			PreviousStatus: SaaSAlertNotificationStatusPending,
			Status:         SaaSAlertNotificationStatusClosed,
			Remark:         closeReq.Remark,
		}},
	}, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminOperationLogs(_ context.Context, options SaaSAdminOperationLogOptions) ([]SaaSAdminOperationLog, error) {
	s.lastOperationOptions = options
	s.operationCalls++
	return s.matchingOperationLogs(options, true), nil
}

func (s *fakeSaaSAdminStore) SaaSAdminOperationLogSummary(_ context.Context, options SaaSAdminOperationLogOptions) (SaaSAdminOperationLogSummary, error) {
	s.lastOperationSummaryOptions = options
	s.operationSummaryCalls++
	items := s.matchingOperationLogs(options, false)
	summary := SaaSAdminOperationLogSummary{OperationCount: len(items)}
	tenantIDs := map[int]struct{}{}
	actorUserIDs := map[int]struct{}{}
	actions := map[string]struct{}{}
	targetTypes := map[string]struct{}{}
	for _, item := range items {
		if item.TenantID > 0 {
			tenantIDs[item.TenantID] = struct{}{}
		}
		if item.ActorUserID > 0 {
			actorUserIDs[item.ActorUserID] = struct{}{}
		}
		if item.Action != "" {
			actions[item.Action] = struct{}{}
		}
		if item.TargetType != "" {
			targetTypes[item.TargetType] = struct{}{}
		}
	}
	summary.TenantCount = len(tenantIDs)
	summary.ActorUserCount = len(actorUserIDs)
	summary.ActionCount = len(actions)
	summary.TargetTypeCount = len(targetTypes)
	return summary, nil
}

func (s *fakeSaaSAdminStore) matchingOperationLogs(options SaaSAdminOperationLogOptions, applyLimit bool) []SaaSAdminOperationLog {
	items := make([]SaaSAdminOperationLog, 0, len(s.operationLogs))
	keyword := strings.ToLower(strings.TrimSpace(options.Keyword))
	for _, item := range s.operationLogs {
		if options.TenantID > 0 && item.TenantID != options.TenantID {
			continue
		}
		if options.Action != "" && item.Action != options.Action {
			continue
		}
		if options.TargetType != "" && item.TargetType != options.TargetType {
			continue
		}
		if keyword != "" {
			haystack := strings.ToLower(strings.Join([]string{
				item.Action,
				item.TargetType,
				item.TargetID,
				item.TargetName,
				item.Remark,
				item.BeforeJSON,
				item.AfterJSON,
			}, "\n"))
			if !strings.Contains(haystack, keyword) {
				continue
			}
		}
		items = append(items, item)
	}
	if applyLimit && options.Limit > 0 && len(items) > options.Limit {
		return items[:options.Limit]
	}
	return items
}

func (s *fakeSaaSAdminStore) RecordSaaSAdminOperationLog(_ context.Context, item SaaSAdminOperationLog) (int64, error) {
	s.recordOperationLogCalls++
	id := int64(len(s.operationLogs) + 1)
	if item.ID > 0 {
		id = item.ID
	}
	item.ID = id
	s.lastRecordedOperationLog = item
	s.operationLogs = append(s.operationLogs, item)
	return id, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminBillingEvents(_ context.Context, options SaaSAdminBillingEventOptions) ([]SaaSAdminBillingEvent, error) {
	s.lastBillingOptions = options
	s.billingCalls++
	return s.matchingBillingEvents(options, true), nil
}

func (s *fakeSaaSAdminStore) SaaSAdminBillingEventSummary(_ context.Context, options SaaSAdminBillingEventOptions) (SaaSAdminDailyBillingSummary, error) {
	s.lastBillingSummaryOptions = options
	s.billingSummaryCalls++
	return saasAdminDailyBillingSummary(s.matchingBillingEvents(options, false)), nil
}

func (s *fakeSaaSAdminStore) SaaSAdminBillingEventByID(_ context.Context, id int64) (SaaSAdminBillingEvent, bool, error) {
	s.lastBillingEventByID = id
	s.billingEventByIDCalls++
	for _, item := range s.billingEvents {
		if item.ID == id {
			return item, true, nil
		}
	}
	for _, item := range s.billingReconciliationItems {
		if item.BillingEvent.ID == id {
			return item.BillingEvent, true, nil
		}
	}
	return SaaSAdminBillingEvent{}, false, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminBillingReconciliation(_ context.Context, options SaaSAdminBillingReconciliationOptions) (SaaSAdminBillingReconciliationReport, error) {
	s.lastBillingReconcileOptions = options
	s.billingReconcileCalls++
	items := s.matchingBillingReconciliationItems(options, false)
	report := SaaSAdminBillingReconciliationReport{
		Summary: saasAdminBillingReconciliationSummary(items),
	}
	if options.Limit > 0 && len(items) > options.Limit {
		items = items[:options.Limit]
	}
	report.Items = items
	return report, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminBillingReconciliationFollowUpSnapshots(_ context.Context, options SaaSAdminBillingReconciliationFollowUpOptions) ([]SaaSAdminBillingReconciliationFollowUpSnapshot, error) {
	s.lastBillingFollowUpOptions = options
	s.billingFollowUpCalls++
	items := make([]SaaSAdminBillingReconciliationFollowUpSnapshot, 0, len(s.billingFollowUpSnapshots))
	owner := strings.ToLower(strings.TrimSpace(options.Owner))
	keyword := strings.ToLower(strings.TrimSpace(options.Keyword))
	for _, item := range s.billingFollowUpSnapshots {
		if options.TenantID > 0 && item.TenantID != options.TenantID {
			continue
		}
		if options.Status != "" && item.Status != options.Status {
			continue
		}
		if owner != "" && !strings.Contains(strings.ToLower(item.Owner), owner) {
			continue
		}
		if keyword != "" && !saasAdminBillingReconciliationFollowUpSnapshotMatchesKeyword(item, keyword) {
			continue
		}
		items = append(items, item)
	}
	if options.Limit > 0 && len(items) > options.Limit {
		items = items[:options.Limit]
	}
	return items, nil
}

func (s *fakeSaaSAdminStore) matchingBillingReconciliationItems(options SaaSAdminBillingReconciliationOptions, applyLimit bool) []SaaSAdminBillingReconciliationItem {
	items := make([]SaaSAdminBillingReconciliationItem, 0, len(s.billingReconciliationItems))
	keyword := strings.ToLower(strings.TrimSpace(options.Keyword))
	for _, item := range s.billingReconciliationItems {
		event := item.BillingEvent
		if options.TenantID > 0 && event.TenantID != options.TenantID {
			continue
		}
		if options.EventType != "" && event.EventType != options.EventType {
			continue
		}
		if options.PackageCode != "" && event.PackageCode != options.PackageCode {
			continue
		}
		if keyword != "" {
			haystack := strings.ToLower(strings.Join([]string{
				event.EventType,
				event.PackageCode,
				event.PackageName,
				event.PaymentMethod,
				event.ExternalOrderNo,
				event.Remark,
				event.MetadataJSON,
				item.TenantName,
				item.CurrentPackageCode,
				item.CurrentPackageName,
			}, "\n"))
			if !strings.Contains(haystack, keyword) {
				continue
			}
		}
		item = saasAdminBillingReconciliationItemWithStatus(item)
		if options.MismatchOnly && item.Status != "mismatch" {
			continue
		}
		items = append(items, item)
	}
	if applyLimit && options.Limit > 0 && len(items) > options.Limit {
		return items[:options.Limit]
	}
	return items
}

func saasAdminBillingReconciliationSummary(items []SaaSAdminBillingReconciliationItem) SaaSAdminBillingReconciliationSummary {
	var summary SaaSAdminBillingReconciliationSummary
	for _, item := range items {
		item = saasAdminBillingReconciliationItemWithStatus(item)
		summary.CheckedCount++
		if item.Status == "matched" {
			summary.MatchedCount++
			continue
		}
		summary.MismatchedCount++
		for _, reason := range item.Reasons {
			switch reason {
			case "missing_package":
				summary.MissingPackageCount++
			case "inactive_package":
				summary.InactivePackageCount++
			case "package_mismatch":
				summary.PackageMismatchCount++
			case "expires_mismatch":
				summary.ExpiresMismatchCount++
			}
		}
	}
	return summary
}

func (s *fakeSaaSAdminStore) matchingBillingEvents(options SaaSAdminBillingEventOptions, applyLimit bool) []SaaSAdminBillingEvent {
	items := make([]SaaSAdminBillingEvent, 0, len(s.billingEvents))
	keyword := strings.ToLower(strings.TrimSpace(options.Keyword))
	for _, item := range s.billingEvents {
		if options.TenantID > 0 && item.TenantID != options.TenantID {
			continue
		}
		if options.EventType != "" && item.EventType != options.EventType {
			continue
		}
		if options.PackageCode != "" && item.PackageCode != options.PackageCode {
			continue
		}
		if keyword != "" {
			haystack := strings.ToLower(strings.Join([]string{
				item.EventType,
				item.PackageCode,
				item.PackageName,
				item.PaymentMethod,
				item.ExternalOrderNo,
				item.Remark,
				item.MetadataJSON,
			}, "\n"))
			if !strings.Contains(haystack, keyword) {
				continue
			}
		}
		items = append(items, item)
	}
	if applyLimit && options.Limit > 0 && len(items) > options.Limit {
		items = items[:options.Limit]
	}
	return items
}

func (s *fakeSaaSAdminStore) SaaSAdminTaskSummary(_ context.Context, options SaaSAdminTaskOptions) (SaaSAdminTaskSummary, error) {
	items := s.matchingTasks(options, false)
	summary := SaaSAdminTaskSummary{TaskCount: len(items)}
	tenants := map[int]bool{}
	actors := map[int]bool{}
	for _, item := range items {
		switch item.Status {
		case SaaSAdminTaskStatusPending:
			summary.PendingCount++
			summary.ActionableCount++
		case SaaSAdminTaskStatusBlocked:
			summary.BlockedCount++
			summary.ActionableCount++
		case SaaSAdminTaskStatusFailed:
			summary.FailedCount++
			summary.ActionableCount++
		case SaaSAdminTaskStatusApplied:
			summary.AppliedCount++
		case SaaSAdminTaskStatusCanceled:
			summary.CanceledCount++
		}
		switch item.TaskType {
		case SaaSAdminTaskTypePackageSync:
			summary.PackageSyncCount++
		case SaaSAdminTaskTypeTenantRenewal:
			summary.TenantRenewalCount++
		case SaaSAdminTaskTypeTenantProvision:
			summary.TenantProvisionCount++
		}
		if item.TenantID > 0 {
			tenants[item.TenantID] = true
		}
		if item.ActorUserID > 0 {
			actors[item.ActorUserID] = true
		}
	}
	summary.TenantCount = len(tenants)
	summary.ActorUserCount = len(actors)
	return summary, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminTasks(_ context.Context, options SaaSAdminTaskOptions) ([]SaaSAdminTask, error) {
	s.lastTaskOptions = options
	s.taskCalls++
	return s.matchingTasks(options, true), nil
}

func (s *fakeSaaSAdminStore) matchingTasks(options SaaSAdminTaskOptions, applyLimit bool) []SaaSAdminTask {
	items := make([]SaaSAdminTask, 0, len(s.tasks))
	for _, item := range s.tasks {
		if options.TaskID > 0 && item.ID != options.TaskID {
			continue
		}
		if options.TaskType != "" && item.TaskType != options.TaskType {
			continue
		}
		if options.Status != "" && item.Status != options.Status {
			continue
		}
		if options.TenantID > 0 && item.TenantID != options.TenantID {
			continue
		}
		if options.PackageCode != "" && item.PackageCode != options.PackageCode {
			continue
		}
		items = append(items, item)
	}
	if applyLimit && options.Limit > 0 && len(items) > options.Limit {
		items = items[:options.Limit]
	}
	return items
}

func (s *fakeSaaSAdminStore) CreateSaaSAdminTask(_ context.Context, task SaaSAdminTaskCreate) (SaaSAdminTask, error) {
	s.lastTaskCreate = task
	s.taskCreateCalls++
	id := int64(len(s.tasks) + 1)
	for _, item := range s.tasks {
		if item.ID >= id {
			id = item.ID + 1
		}
	}
	created := SaaSAdminTask{
		ID:            id,
		TaskType:      task.TaskType,
		Status:        task.Status,
		TenantID:      task.TenantID,
		PackageCode:   task.PackageCode,
		ActorUserID:   task.ActorUserID,
		ActorTenantID: task.ActorTenantID,
		RequestJSON:   task.RequestJSON,
		PreviewJSON:   task.PreviewJSON,
		Remark:        task.Remark,
		CreatedAt:     "2026-07-09 10:00:00",
		UpdatedAt:     "2026-07-09 10:00:00",
	}
	s.tasks = append(s.tasks, created)
	return created, nil
}

func (s *fakeSaaSAdminStore) UpdateSaaSAdminTaskStatus(_ context.Context, update SaaSAdminTaskStatusUpdate) (SaaSAdminTask, error) {
	s.lastTaskStatusUpdate = update
	s.taskUpdateCalls++
	for i := range s.tasks {
		if s.tasks[i].ID != update.TaskID {
			continue
		}
		s.tasks[i].Status = update.Status
		s.tasks[i].ResultJSON = update.ResultJSON
		s.tasks[i].LastError = update.LastError
		s.tasks[i].UpdatedAt = "2026-07-09 10:01:00"
		if update.Applied {
			s.tasks[i].AppliedAt = "2026-07-09 10:01:00"
		}
		return s.tasks[i], nil
	}
	return SaaSAdminTask{
		ID:         update.TaskID,
		TaskType:   SaaSAdminTaskTypePackageSync,
		Status:     update.Status,
		ResultJSON: update.ResultJSON,
		LastError:  update.LastError,
		AppliedAt:  "2026-07-09 10:01:00",
		UpdatedAt:  "2026-07-09 10:01:00",
	}, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminLatestRiskFollowUps(_ context.Context, tenantIDs []int) (map[int]SaaSAdminRiskFollowUpSnapshot, error) {
	s.lastRiskFollowUpTenantIDs = append([]int{}, tenantIDs...)
	s.latestRiskFollowUpCalls++
	if s.latestRiskFollowUps != nil {
		return s.latestRiskFollowUps, nil
	}
	return map[int]SaaSAdminRiskFollowUpSnapshot{}, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminRiskFollowUpSnapshots(_ context.Context, options SaaSAdminRiskFollowUpTaskOptions) ([]SaaSAdminRiskFollowUpSnapshot, error) {
	s.lastRiskFollowUpTaskOptions = options
	s.riskFollowUpSnapshotCalls++
	return s.riskFollowUpSnapshots, nil
}

func (s *fakeSaaSAdminStore) UpsertSaaSAdminPackage(_ context.Context, update SaaSAdminPackageUpsert) (SaaSAdminPackage, error) {
	s.lastPackageUpsert = update
	s.upsertPackageCalls++
	if s.upsertPackageResult.Code != "" {
		return s.upsertPackageResult, nil
	}
	return SaaSAdminPackage{
		Code:        update.Code,
		Name:        update.Name,
		Description: update.Description,
		Status:      update.Status,
		Limits:      update.Limits,
	}, nil
}

func (s *fakeSaaSAdminStore) RecordSaaSAdminRiskFollowUp(_ context.Context, followUp SaaSAdminRiskFollowUp) (SaaSAdminRiskFollowUpResult, error) {
	s.lastRiskFollowUp = followUp
	s.riskFollowUps = append(s.riskFollowUps, followUp)
	s.riskFollowUpCalls++
	if s.riskFollowUpResult.TenantID > 0 || s.riskFollowUpResult.OperationID > 0 {
		return s.riskFollowUpResult, nil
	}
	return SaaSAdminRiskFollowUpResult{
		TenantID:       followUp.TenantID,
		Status:         followUp.Status,
		Owner:          followUp.Owner,
		NextFollowUpAt: followUp.NextFollowUpAt,
		Remark:         followUp.Remark,
		OperationID:    int64(s.riskFollowUpCalls),
	}, nil
}

func (s *fakeSaaSAdminStore) UpdateSaaSAdminTenantStatus(_ context.Context, update SaaSAdminTenantStatusUpdate) (SaaSAdminTenantStatusUpdateResult, error) {
	s.lastStatusUpdate = update
	s.statusCalls++
	return s.statusResult, nil
}

func (s *fakeSaaSAdminStore) PlanSaaSAdminTenantStatusUpdate(_ context.Context, update SaaSAdminTenantStatusUpdate) (SaaSAdminTenantStatusApprovalPlan, error) {
	s.tenantStatusPlanCalls++
	if s.tenantStatusPlanErr != nil {
		return SaaSAdminTenantStatusApprovalPlan{}, s.tenantStatusPlanErr
	}
	if s.tenantStatusPlan.SchemaVersion != 0 {
		return s.tenantStatusPlan, nil
	}
	currentStatus := 1
	if update.Status == 1 {
		currentStatus = 2
	}
	update.ExpectedStatus = currentStatus
	return SaaSAdminTenantStatusApprovalPlan{
		SchemaVersion: SaaSAdminTenantStatusApprovalPlanSchemaVersion,
		Update:        update,
		Snapshot: SaaSAdminTenantStatusApprovalSnapshot{
			TenantID:     update.TenantID,
			TenantName:   fmt.Sprintf("租户 %d", update.TenantID),
			TenantStatus: currentStatus,
		},
	}, nil
}

func (s *fakeSaaSAdminStore) UpdateSaaSAdminTenantPackage(_ context.Context, update SaaSAdminTenantPackageUpdate) (SaaSAdminTenantPackageUpdateResult, error) {
	s.lastUpdate = update
	s.updateCalls++
	if s.updateResult.TenantID == 0 && s.updateResult.PackageCode == "" && s.updateResult.MetricsRefreshed == 0 {
		return SaaSAdminTenantPackageUpdateResult{
			TenantID:         update.TenantID,
			PackageCode:      update.PackageCode,
			ExpiresAt:        update.ExpiresAt,
			Status:           1,
			Version:          update.ExpectedVersion + 1,
			MetricsRefreshed: 26,
		}, nil
	}
	return s.updateResult, nil
}

func (s *fakeSaaSAdminStore) RenewSaaSAdminTenant(_ context.Context, renewal SaaSAdminTenantRenewal) (SaaSAdminTenantRenewalResult, error) {
	s.lastRenewal = renewal
	s.renewalCalls++
	if renewal.TaskID > 0 {
		for index := range s.tasks {
			if s.tasks[index].ID != renewal.TaskID {
				continue
			}
			s.tasks[index].Status = SaaSAdminTaskStatusApplied
			s.tasks[index].Version++
			s.tasks[index].LastError = ""
			s.tasks[index].ResultJSON = saasAdminPayloadJSON(saasAdminTenantRenewalPayload(s.renewalResult))
			break
		}
	}
	return s.renewalResult, nil
}

func (s *fakeSaaSAdminStore) ProvisionSaaSAdminTenant(_ context.Context, provision SaaSAdminTenantProvision) (SaaSAdminTenantProvisionResult, error) {
	s.lastProvision = provision
	s.provisionCalls++
	return s.provisionResult, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminSubscriptions(_ context.Context, options SaaSAdminSubscriptionOptions) (SaaSAdminSubscriptionReport, error) {
	s.lastSubscriptionOptions = options
	s.subscriptionCalls++
	report := s.subscriptionReport
	report.Options = options
	return report, nil
}

func (s *fakeSaaSAdminStore) SaaSAdminSubscriptionEvents(_ context.Context, options SaaSAdminSubscriptionEventOptions) ([]SaaSAdminSubscriptionEvent, error) {
	s.lastSubscriptionEventOptions = options
	s.subscriptionEventCalls++
	return s.subscriptionEvents, nil
}

func (s *fakeSaaSAdminStore) TransitionSaaSAdminSubscription(_ context.Context, transition SaaSAdminSubscriptionTransition) (SaaSAdminSubscriptionTransitionResult, error) {
	s.lastSubscriptionTransition = transition
	s.subscriptionTransitionCalls++
	if s.subscriptionTransitionResult.Subscription.TenantID > 0 {
		return s.subscriptionTransitionResult, nil
	}
	return SaaSAdminSubscriptionTransitionResult{
		Subscription:   SaaSAdminSubscription{TenantID: transition.TenantID, Status: transition.Status, EffectiveStatus: transition.Status, Version: transition.ExpectedVersion + 1},
		PreviousStatus: SaaSAdminSubscriptionStatusActive, Changed: true, EventID: 1, OperationID: 2,
	}, nil
}

func (s *fakeSaaSAdminStore) ReconcileSaaSAdminSubscriptions(_ context.Context, reconcile SaaSAdminSubscriptionReconcile) (SaaSAdminSubscriptionReconcileResult, error) {
	s.lastSubscriptionReconcile = reconcile
	s.subscriptionReconcileCalls++
	return s.subscriptionReconcileResult, nil
}

func readSaaSAdminCSV(t *testing.T, rec *httptest.ResponseRecorder) [][]string {
	t.Helper()
	body := rec.Body.String()
	if !strings.HasPrefix(body, "\ufeff") {
		t.Fatalf("missing utf-8 bom: %q", body)
	}
	records, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(body, "\ufeff"))).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v body=%q", err, body)
	}
	return records
}

func assertSaaSAdminCSVRowPrefix(t *testing.T, records [][]string, prefix []string) {
	t.Helper()
	for _, record := range records {
		if len(record) < len(prefix) {
			continue
		}
		matched := true
		for i, value := range prefix {
			if record[i] != value {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	t.Fatalf("missing CSV row prefix %v in %+v", prefix, records)
}

func decodeSaaSAdminResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var decoded struct {
		Code int            `json:"code"`
		Msg  string         `json:"msg"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if decoded.Code != 200 {
		t.Fatalf("code = %d msg=%s", decoded.Code, decoded.Msg)
	}
	return decoded.Data
}
