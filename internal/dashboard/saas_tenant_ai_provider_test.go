package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
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
}

func (s *tenantAIProviderHandlerStore) SaaSTenantAIProvider(context.Context, int) (SaaSTenantAIProvider, bool, error) {
	return s.provider, s.found, nil
}
func (s *tenantAIProviderHandlerStore) SaveSaaSTenantAIProvider(_ context.Context, input SaaSTenantAIProviderInput) (SaaSTenantAIProvider, error) {
	s.saved = input
	s.provider = SaaSTenantAIProvider{TenantID: input.TenantID, ProviderCode: input.ProviderCode, BaseURL: input.BaseURL, Model: input.Model, APIKeyConfigured: true, CredentialProtection: "usable", Version: input.Version + 1}
	s.found = true
	return s.provider, nil
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
		t.Fatalf("masked key status=%d saved=%q", rec.Code, store.saved.APIKey)
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
			t.Fatalf("audit payload leaked %q: %s", prohibited, serialized)
		}
	}
	if !strings.Contains(serialized, "api.example.test") {
		t.Fatalf("audit payload omitted base URL host: %s", serialized)
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
			t.Fatalf("masked API key %q was accepted", key)
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
