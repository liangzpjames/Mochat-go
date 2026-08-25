package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
	payload := saasTenantAIProviderAuditPayload(SaaSTenantAIProvider{
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
