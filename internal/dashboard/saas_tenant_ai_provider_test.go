package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type tenantAIProviderHandlerStore struct {
	*fakeSaaSAdminStore
	provider SaaSTenantAIProvider
	found    bool
	saved    SaaSTenantAIProviderInput
	readErr  error
	saveErr  error
}

type tenantAIProviderAccessHandlerStore struct {
	*tenantAIProviderHandlerStore
	profile SaaSAdminAccessProfile
}

func (s *tenantAIProviderAccessHandlerStore) SaaSAdminAccessProfile(_ context.Context, userID int, tenantID int) (SaaSAdminAccessProfile, error) {
	profile := s.profile
	profile.UserID, profile.TenantID = userID, tenantID
	return profile, nil
}

func (*tenantAIProviderAccessHandlerStore) SaaSAdminAccessRoles(context.Context) ([]SaaSAdminAccessRole, error) {
	return nil, nil
}

func (*tenantAIProviderAccessHandlerStore) UpsertSaaSAdminAccessRole(context.Context, SaaSAdminAccessRoleUpsert) (SaaSAdminAccessRoleUpsertResult, error) {
	return SaaSAdminAccessRoleUpsertResult{}, nil
}

func (*tenantAIProviderAccessHandlerStore) SaaSAdminAccessAssignments(context.Context, int, SaaSAdminAccessAssignmentOptions) ([]SaaSAdminAccessAssignment, error) {
	return nil, nil
}

func (*tenantAIProviderAccessHandlerStore) UpdateSaaSAdminAccessAssignment(context.Context, int, SaaSAdminAccessAssignmentUpdate) (SaaSAdminAccessAssignmentUpdateResult, error) {
	return SaaSAdminAccessAssignmentUpdateResult{}, nil
}

func (s *tenantAIProviderHandlerStore) SaaSTenantAIProvider(context.Context, int) (SaaSTenantAIProvider, bool, error) {
	return s.provider, s.found, s.readErr
}
func (s *tenantAIProviderHandlerStore) SaveSaaSTenantAIProvider(_ context.Context, input SaaSTenantAIProviderInput) (SaaSTenantAIProvider, error) {
	if s.saveErr != nil {
		return SaaSTenantAIProvider{}, s.saveErr
	}
	s.saved = input
	s.provider = SaaSTenantAIProvider{TenantID: input.TenantID, ProviderCode: input.ProviderCode, BaseURL: input.BaseURL, Model: input.Model, APIKeyConfigured: true, CredentialProtection: "usable", Version: input.Version + 1}
	s.found = true
	return s.provider, nil
}

func TestTenantAIProviderHandlerMapsStoreNotFoundAndConflict(t *testing.T) {
	base := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, Status: 1, IsSuperAdmin: 1}}}
	store := &tenantAIProviderHandlerStore{fakeSaaSAdminStore: base, readErr: NewSaaSAdminNotFound("tenant not found")}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenantAIProvider?tenantId=7", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.TenantAIProvider(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("not-found status=%d", rec.Code)
	}
	store.readErr = nil
	store.saveErr = &SaaSAdminOperationError{Status: http.StatusConflict, Message: "conflict"}
	body, _ := json.Marshal(SaaSTenantAIProviderInput{TenantID: 7, ProviderCode: "openai", BaseURL: "https://api.example.test/v1", Model: "model-v1", APIKey: "fixture-key-1234", EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active"})
	req = httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/tenantAIProvider", bytes.NewReader(body))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.TenantAIProvider(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d", rec.Code)
	}
}

func TestTenantAIProviderHandlerRejectsBadInputWithoutEchoingBody(t *testing.T) {
	store := &tenantAIProviderHandlerStore{fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, Status: 1, IsSuperAdmin: 1}}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	valid := SaaSTenantAIProviderInput{TenantID: 7, ProviderCode: "openai", BaseURL: "https://api.example.test/v1", Model: "model-v1", APIKey: "fixture-key-1234", EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active"}
	cases := []struct {
		name   string
		mutate func(*SaaSTenantAIProviderInput)
		raw    string
	}{
		{"provider", func(v *SaaSTenantAIProviderInput) { v.ProviderCode = "unsupported" }, ""}, {"model-empty", func(v *SaaSTenantAIProviderInput) { v.Model = "" }, ""}, {"model-long", func(v *SaaSTenantAIProviderInput) { v.Model = strings.Repeat("x", 129) }, ""}, {"status", func(v *SaaSTenantAIProviderInput) { v.Status = "bad" }, ""}, {"effective", func(v *SaaSTenantAIProviderInput) { v.EffectiveAt = "bad" }, ""}, {"expiry", func(v *SaaSTenantAIProviderInput) { v.ExpiresAt = v.EffectiveAt }, ""}, {"tenant", func(v *SaaSTenantAIProviderInput) { v.TenantID = 0 }, ""}, {"malformed", nil, "{"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := valid
			raw := tc.raw
			if tc.mutate != nil {
				tc.mutate(&input)
				body, _ := json.Marshal(input)
				raw = string(body)
			}
			req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/tenantAIProvider", strings.NewReader(raw))
			req.Header.Set("X-Mochat-Go-User-ID", "1")
			rec := httptest.NewRecorder()
			handler.TenantAIProvider(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s status=%d", tc.name, rec.Code)
			}
			if strings.Contains(rec.Body.String(), "fixture-key-1234") {
				t.Fatal("error response leaked submitted credential")
			}
		})
	}
}

func TestTenantAIProviderHandlerSuccessPayloadsAreRedacted(t *testing.T) {
	base := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, Status: 1, IsSuperAdmin: 1}}}
	store := &tenantAIProviderHandlerStore{fakeSaaSAdminStore: base}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	request := func(method, raw string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/dashboard/saasAdmin/tenantAIProvider?tenantId=7", strings.NewReader(raw))
		req.Header.Set("X-Mochat-Go-User-ID", "1")
		rec := httptest.NewRecorder()
		handler.TenantAIProvider(rec, req)
		return rec
	}
	for _, tc := range []struct {
		name       string
		configured bool
		method     string
		body       string
	}{
		{"unconfigured", false, http.MethodGet, ""}, {"configured", true, http.MethodGet, ""}, {"put", false, http.MethodPut, `{"tenantId":7,"providerCode":"openai","baseUrl":"https://api.example.test/v1","model":"model-v1","apiKey":"fixture-key-1234","effectiveAt":"2026-08-25T00:00:00Z","expiresAt":"2026-08-26T00:00:00Z","status":"active","version":0}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store.found = tc.configured
			store.provider = SaaSTenantAIProvider{TenantID: 7, ProviderCode: "openai", BaseURL: "https://api.example.test/v1", Model: "model-v1", APIKeyConfigured: true, APIKeyHint: "1234", CredentialProtection: "usable", Version: 1}
			rec := request(tc.method, tc.body)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s status=%d", tc.name, rec.Code)
			}
			var decoded any
			if json.Unmarshal(rec.Body.Bytes(), &decoded) != nil {
				t.Fatal("invalid response")
			}
			assertTenantAIProviderResponseRedacted(t, decoded)
		})
	}
}

func TestTenantAIProviderHandlerStoreFailuresDoNotLeakCredentialData(t *testing.T) {
	base := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, Status: 1, IsSuperAdmin: 1}}}
	store := &tenantAIProviderHandlerStore{fakeSaaSAdminStore: base}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	for _, tc := range []struct {
		name   string
		method string
	}{
		{"get", http.MethodGet}, {"put", http.MethodPut},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store.readErr, store.saveErr = nil, nil
			if tc.method == http.MethodGet {
				store.readErr = errors.New("ordinary storage failure")
			} else {
				store.saveErr = errors.New("ordinary storage failure")
			}
			body := ""
			path := "/dashboard/saasAdmin/tenantAIProvider?tenantId=7"
			if tc.method == http.MethodPut {
				body = `{"tenantId":7,"providerCode":"openai","baseUrl":"https://provider.example.test/private","model":"model-v1","apiKey":"fixture-key-1234","effectiveAt":"2026-08-25T00:00:00Z","expiresAt":"2026-08-26T00:00:00Z","status":"active"}`
				path = "/dashboard/saasAdmin/tenantAIProvider"
			}
			req := httptest.NewRequest(tc.method, path, strings.NewReader(body))
			req.Header.Set("X-Mochat-Go-User-ID", "1")
			rec := httptest.NewRecorder()
			handler.TenantAIProvider(rec, req)
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("%s status=%d", tc.name, rec.Code)
			}
			for _, prohibited := range []string{"fixture-key-1234", "1234", "ciphertext", "https://provider.example.test/private"} {
				if strings.Contains(rec.Body.String(), prohibited) {
					t.Fatal("ordinary store error response exposed protected provider data")
				}
			}
		})
	}
}

func TestTenantAIProviderHandlerUsesActualIntegrationsPermissions(t *testing.T) {
	base := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, Status: 1, IsSuperAdmin: 0}}}
	store := &tenantAIProviderAccessHandlerStore{tenantAIProviderHandlerStore: &tenantAIProviderHandlerStore{fakeSaaSAdminStore: base}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	request := func(method string) *httptest.ResponseRecorder {
		path, body := "/dashboard/saasAdmin/tenantAIProvider?tenantId=7", ""
		if method == http.MethodPut {
			path = "/dashboard/saasAdmin/tenantAIProvider"
			body = `{"tenantId":7,"providerCode":"openai","baseUrl":"https://provider.example.test/v1","model":"model-v1","apiKey":"fixture-key-1234","effectiveAt":"2026-08-25T00:00:00Z","expiresAt":"2026-08-26T00:00:00Z","status":"active"}`
		}
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("X-Mochat-Go-User-ID", "1")
		rec := httptest.NewRecorder()
		handler.TenantAIProvider(rec, req)
		return rec
	}
	store.profile.Permissions = []string{SaaSAdminPermissionIntegrationsRead}
	if rec := request(http.MethodGet); rec.Code != http.StatusOK {
		t.Fatalf("read-only GET status=%d", rec.Code)
	}
	if rec := request(http.MethodPut); rec.Code != http.StatusForbidden {
		t.Fatalf("read-only PUT status=%d", rec.Code)
	}
	store.profile.Permissions = []string{SaaSAdminPermissionIntegrationsManage}
	if rec := request(http.MethodGet); rec.Code != http.StatusForbidden {
		t.Fatalf("manage-only GET status=%d", rec.Code)
	}
	if rec := request(http.MethodPut); rec.Code != http.StatusOK {
		t.Fatalf("manage-only PUT status=%d", rec.Code)
	}
	store.profile.Permissions = []string{SaaSAdminPermissionIntegrationsRead, SaaSAdminPermissionIntegrationsManage}
	if rec := request(http.MethodPut); rec.Code != http.StatusOK {
		t.Fatalf("read-manage PUT status=%d", rec.Code)
	}
}

func assertTenantAIProviderResponseRedacted(t *testing.T, value any) {
	t.Helper()
	switch item := value.(type) {
	case map[string]any:
		for key, nested := range item {
			lowered := strings.ToLower(key)
			if lowered == "apikey" || strings.Contains(lowered, "ciphertext") || strings.Contains(lowered, "authorization") {
				t.Fatal("response contains protected field")
			}
			if key == "baseUrl" {
				if text, ok := nested.(string); ok && strings.Contains(text, "/private/") {
					t.Fatal("response contains full private URL path")
				}
			}
			assertTenantAIProviderResponseRedacted(t, nested)
		}
	case []any:
		for _, nested := range item {
			assertTenantAIProviderResponseRedacted(t, nested)
		}
	}
}

func TestTenantAIProviderHandlerRejectsTenantAdminAndMaskedKey(t *testing.T) {
	store := &tenantAIProviderHandlerStore{fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, Status: 1, IsSuperAdmin: 1}, 2: {ID: 2, TenantID: 2, Status: 1, IsSuperAdmin: 1}}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	body, _ := json.Marshal(SaaSTenantAIProviderInput{TenantID: 7, ProviderCode: "openai", BaseURL: "https://api.example.test/v1", Model: "model-v1", APIKey: "••••1234", EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active"})
	req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/tenantAIProvider", bytes.NewReader(body))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.TenantAIProvider(rec, req)
	if rec.Code != http.StatusBadRequest || store.saved.APIKey != "" {
		t.Fatalf("masked-key status=%d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenantAIProvider?tenantId=7", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec = httptest.NewRecorder()
	handler.TenantAIProvider(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("tenant admin status=%d, want 403", rec.Code)
	}
}

func TestTenantAIProviderPermissionBoundary(t *testing.T) {
	read := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenantAIProvider?tenantId=7", nil)
	if got := SaaSAdminRequiredPermission(read); got != SaaSAdminPermissionIntegrationsRead {
		t.Fatalf("GET permission = %q, want %q", got, SaaSAdminPermissionIntegrationsRead)
	}
	manage := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/tenantAIProvider", nil)
	if got := SaaSAdminRequiredPermission(manage); got != SaaSAdminPermissionIntegrationsManage {
		t.Fatalf("PUT permission = %q, want %q", got, SaaSAdminPermissionIntegrationsManage)
	}
}

func TestTenantAIProviderAuditPayloadRedactsCredentialAndBaseURLPath(t *testing.T) {
	payload := SaaSTenantAIProviderAuditPayload(SaaSTenantAIProvider{
		ProviderCode: "openai", Model: "model-v1", BaseURL: "https://api.example.test/private/path", APIKeyHint: "1234", Version: 2,
	}, true)
	serialized := saasAdminPayloadJSON(payload)
	for _, prohibited := range []string{"apiKey", "1234", "private/path"} {
		if strings.Contains(serialized, prohibited) {
			t.Fatal("audit payload leaked protected value")
		}
	}
	if !strings.Contains(serialized, "api.example.test") {
		t.Fatal("audit payload omitted base URL host")
	}
}

func TestTenantAIProviderInputRejectsUnsafeBaseURL(t *testing.T) {
	input := SaaSTenantAIProviderInput{
		TenantID: 7, ProviderCode: "deepseek", BaseURL: "http://127.0.0.1", Model: "model-v1",
		APIKey: "test-fixture-key", EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active",
	}
	if err := NormalizeAndValidateSaaSTenantAIProviderInput(&input); err == nil {
		t.Fatal("unsafe base URL was accepted")
	}
}

func TestTenantAIProviderInputRejectsMaskedAPIKey(t *testing.T) {
	for _, key := range []string{"********", "••••1234", "....1234"} {
		input := SaaSTenantAIProviderInput{TenantID: 7, ProviderCode: "openai", BaseURL: "https://api.example.test/v1", Model: "model-v1", APIKey: key, EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active"}
		if err := NormalizeAndValidateSaaSTenantAIProviderInput(&input); err == nil {
			t.Fatal("masked-key case was accepted")
		}
	}
}

func TestTenantAIProviderProviderChangeRequiresReplacementKey(t *testing.T) {
	input := SaaSTenantAIProviderInput{TenantID: 7, ProviderCode: "openai", BaseURL: "https://api.example.test/v1", Model: "model-v1", EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active", Version: 1}
	if err := ValidateSaaSTenantAIProviderUpdate(input, SaaSTenantAIProvider{TenantID: 7, ProviderCode: "deepseek", Version: 1}); err == nil {
		t.Fatal("provider change without replacement key was accepted")
	}
}

func TestTenantAIProviderPublicPayloadNeverContainsCredential(t *testing.T) {
	payload := SaaSTenantAIProviderPublicPayload(SaaSTenantAIProvider{
		TenantID: 7, ProviderCode: "openai", BaseURL: "https://api.example.test/v1", Model: "model-v1",
		APIKeyConfigured: true, APIKeyHint: "1234", CredentialProtection: "configured", Version: 1,
	})
	if _, found := payload["apiKey"]; found {
		t.Fatal("payload exposed apiKey")
	}
	if _, found := payload["credentialCiphertext"]; found {
		t.Fatal("payload exposed ciphertext")
	}
}
