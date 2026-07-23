package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/clientip"
	"jiyi/mochat-go/internal/serviceaccountkey"
)

type fakeSaaSServiceAccountStore struct {
	*fakeSaaSAdminStore
	accounts          []SaaSServiceAccount
	principal         SaaSServiceAccountPrincipal
	principalFound    bool
	createInput       SaaSServiceAccountCreate
	updateInput       SaaSServiceAccountUpdate
	rotateInput       SaaSServiceAccountKeyRotate
	revokeInput       SaaSServiceAccountKeyRevoke
	lastOptions       SaaSServiceAccountOptions
	usageOptions      SaaSServiceAccountUsageOptions
	usageReport       SaaSServiceAccountUsageReport
	usageAlertOptions SaaSServiceAccountUsageAlertEvaluateOptions
	usageAlertResult  SaaSServiceAccountUsageAlertEvaluateResult
	lastUseAccount    int64
	lastUseKey        int64
	lastUseIP         string
	lastUseRoute      string
	createCalls       int
	updateCalls       int
	rotateCalls       int
	revokeCalls       int
	principalCalls    int
	useCalls          int
	usageAlertCalls   int
	useErr            error
	useState          SaaSServiceAccountRateLimitState
	protectionCounts  []SaaSServiceAccountKeyProtectionCount
	protectionErr     error
}

func (s *fakeSaaSServiceAccountStore) SaaSServiceAccounts(_ context.Context, options SaaSServiceAccountOptions) ([]SaaSServiceAccount, error) {
	s.lastOptions = options
	return s.accounts, nil
}

func (s *fakeSaaSServiceAccountStore) SaaSServiceAccountUsage(_ context.Context, options SaaSServiceAccountUsageOptions) (SaaSServiceAccountUsageReport, error) {
	s.usageOptions = options
	return s.usageReport, nil
}

func (s *fakeSaaSServiceAccountStore) CreateSaaSServiceAccount(_ context.Context, input SaaSServiceAccountCreate) (SaaSServiceAccountCreateResult, error) {
	s.createCalls++
	s.createInput = input
	return SaaSServiceAccountCreateResult{
		Account: SaaSServiceAccount{
			ID: 10, TenantID: input.TenantID, TenantName: "测试租户", TenantStatus: 1,
			Code: input.Code, Name: input.Name, Description: input.Description, Status: input.Status,
			Scopes: input.Scopes, AllowedCIDRs: input.AllowedCIDRs, ExpiresAt: input.ExpiresAt, Version: 1,
			RateLimitPerMinute: *input.RateLimitPerMinute, DailyRequestLimit: int64(*input.DailyRequestLimit),
			UsageAlertEnabled: *input.UsageAlertEnabled, UsageWarningPercent: *input.UsageWarningPercent,
			RejectionWarningCount: *input.RejectionWarningCount, UsageAlertCooldownMinutes: *input.UsageAlertCooldownMinutes,
			Keys: []SaaSServiceAccountKey{{ID: 20, ServiceAccountID: 10, Name: input.KeyName, Prefix: input.Key.Prefix, HashKeyID: input.Key.HashKeyID, LastFour: input.Key.LastFour, Status: SaaSServiceAccountKeyStatusActive, ExpiresAt: input.KeyExpiresAt, Version: 1}},
		},
		OperationID: 100,
	}, nil
}

func (s *fakeSaaSServiceAccountStore) EvaluateSaaSServiceAccountUsageAlerts(_ context.Context, options SaaSServiceAccountUsageAlertEvaluateOptions) (SaaSServiceAccountUsageAlertEvaluateResult, error) {
	s.usageAlertCalls++
	s.usageAlertOptions = options
	return s.usageAlertResult, nil
}

func (s *fakeSaaSServiceAccountStore) UpdateSaaSServiceAccount(_ context.Context, input SaaSServiceAccountUpdate) (SaaSServiceAccountUpdateResult, error) {
	s.updateCalls++
	s.updateInput = input
	return SaaSServiceAccountUpdateResult{Account: SaaSServiceAccount{ID: input.ID, Name: input.Name, Status: input.Status, Scopes: input.Scopes, Version: input.ExpectedVersion + 1}, OperationID: 101}, nil
}

func (s *fakeSaaSServiceAccountStore) RotateSaaSServiceAccountKey(_ context.Context, input SaaSServiceAccountKeyRotate) (SaaSServiceAccountKeyRotateResult, error) {
	s.rotateCalls++
	s.rotateInput = input
	key := SaaSServiceAccountKey{ID: 21, ServiceAccountID: input.ServiceAccountID, Name: input.Name, Prefix: input.Key.Prefix, HashKeyID: input.Key.HashKeyID, LastFour: input.Key.LastFour, Status: SaaSServiceAccountKeyStatusActive, ExpiresAt: input.ExpiresAt, Version: 1}
	return SaaSServiceAccountKeyRotateResult{Account: SaaSServiceAccount{ID: input.ServiceAccountID, Version: input.ExpectedVersion + 1, Keys: []SaaSServiceAccountKey{key}}, Key: key, RetiringKeys: 1, OperationID: 102}, nil
}

func (s *fakeSaaSServiceAccountStore) RevokeSaaSServiceAccountKey(_ context.Context, input SaaSServiceAccountKeyRevoke) (SaaSServiceAccountKeyRevokeResult, error) {
	s.revokeCalls++
	s.revokeInput = input
	key := SaaSServiceAccountKey{ID: input.KeyID, ServiceAccountID: input.ServiceAccountID, Status: SaaSServiceAccountKeyStatusRevoked, Version: input.ExpectedVersion + 1}
	return SaaSServiceAccountKeyRevokeResult{Account: SaaSServiceAccount{ID: input.ServiceAccountID, Version: 2, Keys: []SaaSServiceAccountKey{key}}, Key: key, OperationID: 103}, nil
}

func (s *fakeSaaSServiceAccountStore) SaaSServiceAccountPrincipalByPrefix(_ context.Context, _ string) (SaaSServiceAccountPrincipal, bool, error) {
	s.principalCalls++
	return s.principal, s.principalFound, nil
}

func (s *fakeSaaSServiceAccountStore) ConsumeSaaSServiceAccountRequest(_ context.Context, serviceAccountID int64, keyID int64, clientIP string, routeKey string) (SaaSServiceAccountRateLimitState, error) {
	s.useCalls++
	s.lastUseAccount = serviceAccountID
	s.lastUseKey = keyID
	s.lastUseIP = clientIP
	s.lastUseRoute = routeKey
	return s.useState, s.useErr
}

func (s *fakeSaaSServiceAccountStore) SaaSServiceAccountKeyProtectionCounts(context.Context) ([]SaaSServiceAccountKeyProtectionCount, error) {
	return s.protectionCounts, s.protectionErr
}

func TestSaaSServiceAccountKeyGenerationAndParsing(t *testing.T) {
	handler := NewSaaSAdminHandler(&fakeSaaSAdminStore{}, HeaderUserIDResolver{}, 1).WithServiceAccountKeyPepper("test-service-account-pepper")
	key, err := handler.newSaaSServiceAccountKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key.PlainText, "mch_live_") || len(key.Prefix) != 12 || len(key.Hash) != 64 || len(key.LastFour) != 4 || key.HashKeyID != serviceaccountkey.LegacyJWTKeyID {
		t.Fatalf("key = %+v", key)
	}
	if key.Hash != hashSaaSServiceAccountKey("test-service-account-pepper", key.PlainText) || strings.Contains(key.Hash, key.LastFour) {
		t.Fatal("key hash mismatch or leaks key suffix")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/saas/v1/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+key.PlainText)
	token, prefix, ok := presentedSaaSServiceAccountKey(req)
	if !ok || token != key.PlainText || prefix != key.Prefix {
		t.Fatalf("token=%q prefix=%q ok=%t", token, prefix, ok)
	}
}

func TestSaaSServiceAccountDedicatedPepperStatusAndLegacyCutover(t *testing.T) {
	base := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}
	store := &fakeSaaSServiceAccountStore{
		fakeSaaSAdminStore: base,
		protectionCounts: []SaaSServiceAccountKeyProtectionCount{
			{HashKeyID: serviceaccountkey.LegacyJWTKeyID, KeyCount: 2, UsableKeyCount: 1},
			{HashKeyID: "2026-q3", KeyCount: 1, UsableKeyCount: 1},
		},
	}
	manager, err := serviceaccountkey.NewManager(serviceaccountkey.Config{
		ActiveKeyID: "2026-q3", ActiveKey: strings.Repeat("07", 32),
		LegacyJWTSecret: "legacy-dashboard-secret", AllowLegacyJWT: true, RequireDedicated: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithServiceAccountKeyManager(manager)
	key, err := handler.newSaaSServiceAccountKey()
	if err != nil || key.HashKeyID != "2026-q3" {
		t.Fatalf("key=%+v err=%v", key, err)
	}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/serviceAccounts", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServiceAccounts(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data struct {
			KeyProtection      map[string]any `json:"keyProtection"`
			ClientIPResolution map[string]any `json:"clientIPResolution"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.KeyProtection["activeKeyId"] != "2026-q3" || payload.Data.KeyProtection["legacyUsableKeyCount"] != float64(1) || payload.Data.KeyProtection["healthy"] != true {
		t.Fatalf("key protection = %+v", payload.Data.KeyProtection)
	}
	if payload.Data.ClientIPResolution["trustProxyHeaders"] != false || payload.Data.ClientIPResolution["trustedProxyCidrCount"] != float64(0) {
		t.Fatalf("client IP resolution = %+v", payload.Data.ClientIPResolution)
	}
	if err := handler.CheckServiceAccountKeyProtection(context.Background()); err != nil {
		t.Fatal(err)
	}

	strictManager, err := serviceaccountkey.NewManager(serviceaccountkey.Config{
		ActiveKeyID: "2026-q3", ActiveKey: strings.Repeat("07", 32), AllowLegacyJWT: false, RequireDedicated: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	handler.WithServiceAccountKeyManager(strictManager)
	if err := handler.CheckServiceAccountKeyProtection(context.Background()); err == nil || !strings.Contains(err.Error(), serviceaccountkey.LegacyJWTKeyID) {
		t.Fatalf("expected missing legacy pepper error, got %v", err)
	}
}

func TestNormalizeSaaSServiceAccountCreate(t *testing.T) {
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.Local)
	input := SaaSServiceAccountCreate{
		TenantID: 8, Code: " Data_Sync ", Name: " 数据同步 ", Status: "ACTIVE",
		Scopes:       []string{SaaSServiceAccountScopeUsageRead, SaaSServiceAccountScopeProfileRead, SaaSServiceAccountScopeUsageRead},
		AllowedCIDRs: []string{"127.0.0.1", "10.0.0.9/24", "10.0.0.0/24"},
	}
	if err := normalizeSaaSServiceAccountCreate(&input, now); err != nil {
		t.Fatal(err)
	}
	if input.Code != "data_sync" || input.Name != "数据同步" || input.Status != SaaSServiceAccountStatusActive || input.KeyName != "初始密钥" {
		t.Fatalf("input = %+v", input)
	}
	if len(input.Scopes) != 2 || strings.Join(input.AllowedCIDRs, ",") != "10.0.0.0/24,127.0.0.1/32" {
		t.Fatalf("scopes=%v cidrs=%v", input.Scopes, input.AllowedCIDRs)
	}
	if input.RateLimitPerMinute == nil || *input.RateLimitPerMinute != DefaultSaaSServiceAccountRateLimitPerMinute || input.DailyRequestLimit == nil || *input.DailyRequestLimit != DefaultSaaSServiceAccountDailyRequestLimit {
		t.Fatalf("request limits = minute %v daily %v", input.RateLimitPerMinute, input.DailyRequestLimit)
	}
	if input.UsageAlertEnabled == nil || !*input.UsageAlertEnabled || input.UsageWarningPercent == nil || *input.UsageWarningPercent != 80 || input.RejectionWarningCount == nil || *input.RejectionWarningCount != 1 || input.UsageAlertCooldownMinutes == nil || *input.UsageAlertCooldownMinutes != 60 {
		t.Fatalf("usage alert policy = enabled %v usage %v rejection %v cooldown %v", input.UsageAlertEnabled, input.UsageWarningPercent, input.RejectionWarningCount, input.UsageAlertCooldownMinutes)
	}
	wantExpiry := now.Add(saasServiceAccountDefaultKeyTTL).Format("2006-01-02 15:04:05")
	if input.KeyExpiresAt != wantExpiry {
		t.Fatalf("key expiry = %q, want %q", input.KeyExpiresAt, wantExpiry)
	}
	input.Scopes = []string{"tenant.unknown.read"}
	if err := normalizeSaaSServiceAccountCreate(&input, now); err == nil {
		t.Fatal("unknown scope accepted")
	}
}

func TestSaaSServiceAccountUsageAlertEvaluateHandler(t *testing.T) {
	base := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}
	store := &fakeSaaSServiceAccountStore{fakeSaaSAdminStore: base, usageAlertResult: SaaSServiceAccountUsageAlertEvaluateResult{
		ScannedAccounts: 2, EligibleAccounts: 2, UsageWarningAccounts: 1, RejectionWarningAccounts: 1,
		NotificationsQueued: 2, NotificationsClosed: 1, OperationID: 88, EvaluatedAt: "2026-07-12 16:30:00",
	}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/serviceAccountUsageAlertEvaluate", strings.NewReader(`{"tenantId":8,"serviceAccountId":10,"limit":20}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServiceAccountUsageAlertEvaluate(rec, req)
	if rec.Code != http.StatusOK || store.usageAlertCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.usageAlertCalls, rec.Body.String())
	}
	if store.usageAlertOptions.TenantID != 8 || store.usageAlertOptions.ServiceAccountID != 10 || store.usageAlertOptions.Limit != 20 || store.usageAlertOptions.Source != "admin_manual" || store.usageAlertOptions.ActorUserID != 1 {
		t.Fatalf("options=%+v", store.usageAlertOptions)
	}
	var payload struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil || payload.Data["notificationsQueued"] != float64(2) || payload.Data["notificationsClosed"] != float64(1) || payload.Data["operationId"] != float64(88) {
		t.Fatalf("payload=%+v err=%v", payload, err)
	}

	bad := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/serviceAccountUsageAlertEvaluate", strings.NewReader(`{"limit":501}`))
	bad.Header.Set("X-Mochat-Go-User-ID", "1")
	badRec := httptest.NewRecorder()
	handler.ServiceAccountUsageAlertEvaluate(badRec, bad)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("bad status=%d body=%s", badRec.Code, badRec.Body.String())
	}
}

func TestSaaSServiceAccountUsageHandlerBuildsHistoryAndValidatesRange(t *testing.T) {
	base := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}
	today := time.Now().Format("2006-01-02")
	dateFrom := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	store := &fakeSaaSServiceAccountStore{fakeSaaSAdminStore: base, usageReport: SaaSServiceAccountUsageReport{
		DateFrom: dateFrom, DateTo: today, RetentionDays: 90, OldestUsageDate: dateFrom, StoredRowCount: 1,
		RequestCount: 12, RejectedCount: 2, ActiveAccountCount: 1, LimitedAccountCount: 1,
		Daily:    []SaaSServiceAccountUsageDaily{{UsageDate: today, RequestCount: 12, RejectedCount: 2, ActiveAccountCount: 1}},
		Routes:   []SaaSServiceAccountUsageRoute{{RouteKey: "GET /api/saas/v1/whoami", RequestCount: 12, RejectedCount: 2, ActiveAccountCount: 1}},
		Accounts: []SaaSServiceAccountUsageAccount{{ServiceAccountID: 10, ServiceAccountCode: "data_sync", TenantID: 8, TenantName: "测试租户", RequestCount: 12}},
	}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/serviceAccountUsage?days=3&tenantId=8&serviceAccountId=10&limit=5", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServiceAccountUsage(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data struct {
			RetentionDays int                            `json:"retentionDays"`
			RequestCount  int64                          `json:"requestCount"`
			Daily         []SaaSServiceAccountUsageDaily `json:"daily"`
			Routes        []map[string]any               `json:"routes"`
			Accounts      []map[string]any               `json:"accounts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if store.usageOptions.Days != 3 || store.usageOptions.TenantID != 8 || store.usageOptions.ServiceAccountID != 10 || store.usageOptions.Limit != 5 {
		t.Fatalf("options=%+v", store.usageOptions)
	}
	if payload.Data.RetentionDays != 90 || payload.Data.RequestCount != 12 || len(payload.Data.Daily) != 3 || len(payload.Data.Routes) != 1 || len(payload.Data.Accounts) != 1 {
		t.Fatalf("payload=%+v", payload.Data)
	}

	bad := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/serviceAccountUsage?days=91", nil)
	bad.Header.Set("X-Mochat-Go-User-ID", "1")
	badRec := httptest.NewRecorder()
	handler.ServiceAccountUsage(badRec, bad)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid range status=%d body=%s", badRec.Code, badRec.Body.String())
	}
}

func TestSaaSServiceAccountManagementHandlersNeverPassPlainTextToStore(t *testing.T) {
	base := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}
	store := &fakeSaaSServiceAccountStore{fakeSaaSAdminStore: base}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "test-service-account-pepper")
	body := `{"tenantId":8,"code":"data_sync","name":"数据同步","status":"active","scopes":["tenant.profile.read","tenant.usage.read"],"allowedCidrs":["127.0.0.1"],"keyExpiresAt":"2099-01-01 00:00:00"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/serviceAccount", strings.NewReader(body))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServiceAccount(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.createCalls != 1 || store.createInput.Key.Hash == "" || store.createInput.Key.HashKeyID != serviceaccountkey.LegacyJWTKeyID || store.createInput.Key.PlainText != "" {
		t.Fatalf("store input = %+v", store.createInput.Key)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache-control = %q", rec.Header().Get("Cache-Control"))
	}
	var created struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.Code != http.StatusCreated {
		t.Fatalf("decode create response: err=%v body=%s", err, rec.Body.String())
	}
	plainText, _ := created.Data["plainTextKey"].(string)
	if !strings.HasPrefix(plainText, "mch_live_") || strings.Contains(rec.Body.String(), store.createInput.Key.Hash) {
		t.Fatalf("response = %s", rec.Body.String())
	}
	rotateBody := `{"serviceAccountId":10,"expectedVersion":1,"name":"2026-Q3","expiresAt":"2099-01-01 00:00:00","graceMinutes":30}`
	rotateReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/serviceAccountKeyRotate", strings.NewReader(rotateBody))
	rotateReq.Header.Set("X-Mochat-Go-User-ID", "1")
	rotateRec := httptest.NewRecorder()
	handler.ServiceAccountKeyRotate(rotateRec, rotateReq)
	if rotateRec.Code != http.StatusOK || store.rotateCalls != 1 || store.rotateInput.Key.Hash == "" || store.rotateInput.Key.PlainText != "" {
		t.Fatalf("status=%d body=%s input=%+v", rotateRec.Code, rotateRec.Body.String(), store.rotateInput.Key)
	}
}

func TestSaaSServiceAccountOpenAPIAuthenticationAndScopes(t *testing.T) {
	base := &fakeSaaSAdminStore{
		overview:     SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{TenantID: 8, TenantName: "客户 A", TenantStatus: 1}}},
		usageMetrics: []SaaSAdminUsageMetric{{Metric: SaaSMetricContacts, Current: 5, Limit: 10}},
		alertPage:    SaaSAlertListPage{Items: []SaaSAlertRecord{{ID: 1, TenantID: 8, Metric: SaaSMetricContacts, Status: SaaSAlertStatusOpen}}, Total: 1, TotalPage: 1},
	}
	resetAt := time.Now().Add(time.Minute)
	store := &fakeSaaSServiceAccountStore{fakeSaaSAdminStore: base, principalFound: true, useState: SaaSServiceAccountRateLimitState{
		RateLimitPerMinute: 60, MinuteRequestCount: 1, MinuteResetAt: resetAt,
		DailyRequestLimit: 10000, DailyRequestCount: 1, DailyResetAt: resetAt.Add(24 * time.Hour),
	}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "test-service-account-pepper")
	key, err := handler.newSaaSServiceAccountKey()
	if err != nil {
		t.Fatal(err)
	}
	store.principal = SaaSServiceAccountPrincipal{
		ServiceAccountID: 10, ServiceAccountCode: "data_sync", ServiceAccountName: "数据同步", ServiceAccountStatus: SaaSServiceAccountStatusActive,
		TenantID: 8, TenantName: "客户 A", TenantStatus: 1,
		Scopes:       []string{SaaSServiceAccountScopeProfileRead, SaaSServiceAccountScopeUsageRead, SaaSServiceAccountScopeAlertsRead},
		AllowedCIDRs: []string{"127.0.0.1/32"}, KeyID: 20, KeyName: "初始密钥", KeyPrefix: key.Prefix, KeyHash: key.Hash, KeyHashKeyID: key.HashKeyID,
		KeyStatus: SaaSServiceAccountKeyStatusActive, KeyExpiresAt: "2099-01-01 00:00:00",
	}
	call := func(path string, serve func(http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:4321"
		req.Header.Set("Authorization", "Bearer "+key.PlainText)
		rec := httptest.NewRecorder()
		serve(rec, req)
		return rec
	}
	if rec := call("/api/saas/v1/whoami", handler.SaaSServiceAccountWhoAmI); rec.Code != http.StatusOK {
		t.Fatalf("whoami status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := call("/api/saas/v1/usage", handler.SaaSServiceAccountUsage); rec.Code != http.StatusOK || base.lastUsageTenantID != 8 || base.lastOptions.TenantID != 8 {
		t.Fatalf("usage status=%d body=%s tenant=%d options=%+v", rec.Code, rec.Body.String(), base.lastUsageTenantID, base.lastOptions)
	}
	if rec := call("/api/saas/v1/alerts?status=open", handler.SaaSServiceAccountAlerts); rec.Code != http.StatusOK || base.lastAlertOptions.TenantID != 8 {
		t.Fatalf("alerts status=%d body=%s options=%+v", rec.Code, rec.Body.String(), base.lastAlertOptions)
	}
	if store.useCalls != 3 || store.lastUseAccount != 10 || store.lastUseKey != 20 || store.lastUseIP != "127.0.0.1" || store.lastUseRoute != "GET /api/saas/v1/alerts" {
		t.Fatalf("use calls=%d account=%d key=%d ip=%q route=%q", store.useCalls, store.lastUseAccount, store.lastUseKey, store.lastUseIP, store.lastUseRoute)
	}

	wrongReq := httptest.NewRequest(http.MethodGet, "/api/saas/v1/whoami", nil)
	wrongReq.RemoteAddr = "127.0.0.1:4321"
	wrongReq.Header.Set("Authorization", "Bearer "+key.PlainText[:len(key.PlainText)-1]+"x")
	wrongRec := httptest.NewRecorder()
	handler.SaaSServiceAccountWhoAmI(wrongRec, wrongReq)
	if wrongRec.Code != http.StatusUnauthorized || strings.Contains(wrongRec.Body.String(), key.Prefix) {
		t.Fatalf("wrong key status=%d body=%s", wrongRec.Code, wrongRec.Body.String())
	}

	store.principal.Scopes = []string{SaaSServiceAccountScopeProfileRead}
	if rec := call("/api/saas/v1/usage", handler.SaaSServiceAccountUsage); rec.Code != http.StatusForbidden {
		t.Fatalf("missing scope status=%d body=%s", rec.Code, rec.Body.String())
	}
	store.principal.Scopes = []string{SaaSServiceAccountScopeProfileRead}
	store.principal.AllowedCIDRs = []string{"10.0.0.0/8"}
	if rec := call("/api/saas/v1/whoami", handler.SaaSServiceAccountWhoAmI); rec.Code != http.StatusForbidden {
		t.Fatalf("IP allowlist status=%d body=%s", rec.Code, rec.Body.String())
	}
	store.principal.AllowedCIDRs = []string{"127.0.0.1/32"}
	store.useErr = &SaaSServiceAccountAuthError{Status: http.StatusUnauthorized, Message: "account changed during use recording"}
	if rec := call("/api/saas/v1/whoami", handler.SaaSServiceAccountWhoAmI); rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "invalid API key") || strings.Contains(rec.Body.String(), "account changed") {
		t.Fatalf("concurrent auth change status=%d body=%s", rec.Code, rec.Body.String())
	}
	limitState := SaaSServiceAccountRateLimitState{
		RateLimitPerMinute: 2, MinuteRequestCount: 2, MinuteRejectedCount: 1, MinuteResetAt: time.Now().Add(time.Minute),
		DailyRequestLimit: 10, DailyRequestCount: 2, DailyRejectedCount: 1, DailyResetAt: time.Now().Add(24 * time.Hour), LimitedBy: "minute",
	}
	store.useErr = &SaaSServiceAccountRateLimitError{State: limitState}
	store.useState = limitState
	if rec := call("/api/saas/v1/whoami", handler.SaaSServiceAccountWhoAmI); rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" || rec.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("rate limit status=%d headers=%v body=%s", rec.Code, rec.Header(), rec.Body.String())
	}
}

func TestSaaSServiceAccountRetiringKeyWindow(t *testing.T) {
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.Local)
	principal := SaaSServiceAccountPrincipal{
		ServiceAccountStatus: SaaSServiceAccountStatusActive, TenantStatus: 1,
		KeyStatus: SaaSServiceAccountKeyStatusRetiring, KeyExpiresAt: "2099-01-01 00:00:00", KeyRetireAt: "2026-07-11 10:30:00",
	}
	if !saasServiceAccountPrincipalEnabled(principal, now) {
		t.Fatal("retiring key should be accepted during grace period")
	}
	if saasServiceAccountPrincipalEnabled(principal, now.Add(31*time.Minute)) {
		t.Fatal("retiring key accepted after grace period")
	}
}

func TestSaaSServiceAccountTrustedProxyClientIP(t *testing.T) {
	store := &fakeSaaSServiceAccountStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{overview: SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{TenantID: 8, TenantName: "客户 A", TenantStatus: 1}}}},
		principalFound:     true,
		useState: SaaSServiceAccountRateLimitState{
			RateLimitPerMinute: 60, MinuteResetAt: time.Now().Add(time.Minute), DailyRequestLimit: 1000, DailyResetAt: time.Now().Add(24 * time.Hour),
		},
	}
	resolver, err := clientip.NewResolver(clientip.Config{TrustProxyHeaders: true, TrustedProxyCIDRs: []string{"127.0.0.1/32"}})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "test-service-account-pepper").WithServiceAccountClientIPResolver(resolver)
	key, err := handler.newSaaSServiceAccountKey()
	if err != nil {
		t.Fatal(err)
	}
	store.principal = SaaSServiceAccountPrincipal{
		ServiceAccountID: 10, ServiceAccountCode: "data_sync", ServiceAccountName: "数据同步", ServiceAccountStatus: SaaSServiceAccountStatusActive,
		TenantID: 8, TenantName: "客户 A", TenantStatus: 1, Scopes: []string{SaaSServiceAccountScopeProfileRead},
		AllowedCIDRs: []string{"203.0.113.0/24"}, KeyID: 20, KeyPrefix: key.Prefix, KeyHash: key.Hash, KeyHashKeyID: key.HashKeyID,
		KeyStatus: SaaSServiceAccountKeyStatusActive, KeyExpiresAt: "2099-01-01 00:00:00",
	}
	request := httptest.NewRequest(http.MethodGet, "/api/saas/v1/whoami", nil)
	request.RemoteAddr = "127.0.0.1:4321"
	request.Header.Set("X-Forwarded-For", "198.51.100.9, 203.0.113.25")
	request.Header.Set("Authorization", "Bearer "+key.PlainText)
	recorder := httptest.NewRecorder()
	handler.SaaSServiceAccountWhoAmI(recorder, request)
	if recorder.Code != http.StatusOK || store.lastUseIP != "203.0.113.25" {
		t.Fatalf("trusted proxy status=%d ip=%q body=%s", recorder.Code, store.lastUseIP, recorder.Body.String())
	}

	untrustedResolver, err := clientip.NewResolver(clientip.Config{TrustProxyHeaders: true, TrustedProxyCIDRs: []string{"10.0.0.0/8"}})
	if err != nil {
		t.Fatal(err)
	}
	handler.WithServiceAccountClientIPResolver(untrustedResolver)
	recorder = httptest.NewRecorder()
	handler.SaaSServiceAccountWhoAmI(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("spoofed proxy header status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSaaSServiceAccountPayloadOmitsSecretMaterial(t *testing.T) {
	payload := saasServiceAccountPayload(SaaSServiceAccount{ID: 1, Keys: []SaaSServiceAccountKey{{ID: 2, Prefix: "abcdefghijkl", LastFour: "wxyz"}}})
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "keyHash") || strings.Contains(text, "plainText") || !strings.Contains(text, "...wxyz") {
		t.Fatalf("payload = %s", text)
	}
}
