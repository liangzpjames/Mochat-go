package dashboard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const (
	SaaSAdminOperationActionTenantAIProviderSave = "saas.admin.tenant_ai_provider.save"
	SaaSAdminOperationTargetTenantAIProvider     = "saas_tenant_ai_provider"
)

type SaaSTenantAIProviderInput struct {
	TenantID      int    `json:"tenantId"`
	ProviderCode  string `json:"providerCode"`
	BaseURL       string `json:"baseUrl"`
	Model         string `json:"model"`
	APIKey        string `json:"apiKey"`
	EffectiveAt   string `json:"effectiveAt"`
	ExpiresAt     string `json:"expiresAt"`
	Status        string `json:"status"`
	Version       int    `json:"version"`
	ActorUserID   int    `json:"-"`
	ActorTenantID int    `json:"-"`
}

type SaaSTenantAIProvider struct {
	TenantID             int    `json:"tenantId"`
	ProviderCode         string `json:"providerCode"`
	BaseURL              string `json:"baseUrl"`
	Model                string `json:"model"`
	APIKeyConfigured     bool   `json:"apiKeyConfigured"`
	APIKeyHint           string `json:"apiKeyHint"`
	CredentialProtection string `json:"credentialProtection"`
	EffectiveAt          string `json:"effectiveAt"`
	ExpiresAt            string `json:"expiresAt"`
	Status               string `json:"status"`
	Version              int    `json:"version"`
	UpdatedAt            string `json:"updatedAt"`
}

type SaaSTenantAIProviderStore interface {
	SaaSTenantAIProvider(ctx context.Context, tenantID int) (SaaSTenantAIProvider, bool, error)
	SaveSaaSTenantAIProvider(ctx context.Context, input SaaSTenantAIProviderInput) (SaaSTenantAIProvider, error)
}

func ValidateSaaSTenantAIProviderCreate(input SaaSTenantAIProviderInput) error {
	if err := NormalizeAndValidateSaaSTenantAIProviderInput(&input); err != nil {
		return err
	}
	if strings.TrimSpace(input.APIKey) == "" {
		return errors.New("apiKey is required when creating a tenant AI provider")
	}
	return nil
}

func ValidateSaaSTenantAIProviderUpdate(input SaaSTenantAIProviderInput, current SaaSTenantAIProvider) error {
	if err := NormalizeAndValidateSaaSTenantAIProviderInput(&input); err != nil {
		return err
	}
	if current.TenantID > 0 && current.ProviderCode != "" && current.ProviderCode != input.ProviderCode && strings.TrimSpace(input.APIKey) == "" {
		return errors.New("apiKey is required when changing providerCode")
	}
	return nil
}

func NormalizeAndValidateSaaSTenantAIProviderInput(input *SaaSTenantAIProviderInput) error {
	if input == nil {
		return errors.New("tenant AI provider input is required")
	}
	input.ProviderCode = strings.ToLower(strings.TrimSpace(input.ProviderCode))
	input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	input.Model = strings.TrimSpace(input.Model)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.APIKey = strings.TrimSpace(input.APIKey)
	if isMaskedTenantAIProviderAPIKey(input.APIKey) {
		return errors.New("apiKey must be a replacement key, not a masked value")
	}
	if input.TenantID <= 0 || input.Version < 0 {
		return errors.New("tenantId or version is invalid")
	}
	switch input.ProviderCode {
	case "deepseek", "openai", "dashscope", "custom":
	default:
		return errors.New("providerCode is unsupported")
	}
	if err := validateTenantAIProviderBaseURL(input.BaseURL); err != nil {
		return err
	}
	if input.Model == "" || len([]rune(input.Model)) > 128 {
		return errors.New("model is required and must be at most 128 characters")
	}
	if input.Status != "active" && input.Status != "disabled" {
		return errors.New("status must be active or disabled")
	}
	effective, err := time.Parse(time.RFC3339, strings.TrimSpace(input.EffectiveAt))
	if err != nil {
		return errors.New("effectiveAt must be RFC3339")
	}
	expires, err := time.Parse(time.RFC3339, strings.TrimSpace(input.ExpiresAt))
	if err != nil {
		return errors.New("expiresAt must be RFC3339")
	}
	if !expires.After(effective) {
		return errors.New("expiresAt must be after effectiveAt")
	}
	input.EffectiveAt, input.ExpiresAt = effective.UTC().Format(time.RFC3339), expires.UTC().Format(time.RFC3339)
	return nil
}

func isMaskedTenantAIProviderAPIKey(value string) bool {
	if value == "" {
		return false
	}
	runes := []rune(value)
	if len(runes) >= 4 {
		allMask := true
		for _, r := range runes {
			if r != '*' && r != '•' && r != '.' {
				allMask = false
				break
			}
		}
		if allMask {
			return true
		}
		if len(runes) >= 8 {
			prefixMask := true
			for _, r := range runes[:4] {
				if r != '*' && r != '•' && r != '.' {
					prefixMask = false
					break
				}
			}
			if prefixMask {
				return true
			}
		}
	}
	return false
}

func validateTenantAIProviderBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("baseUrl must be an HTTPS URL without userinfo, query, or fragment")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errors.New("baseUrl host is not allowed")
	}
	if ip, err := netip.ParseAddr(host); err == nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
		return errors.New("baseUrl host is not allowed")
	}
	if parsed := net.ParseIP(host); parsed != nil && (parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsLinkLocalUnicast() || parsed.IsUnspecified()) {
		return errors.New("baseUrl host is not allowed")
	}
	return nil
}

func SaaSTenantAIProviderPublicPayload(provider SaaSTenantAIProvider) map[string]any {
	return map[string]any{
		"tenantId": provider.TenantID, "providerCode": provider.ProviderCode, "baseUrl": provider.BaseURL, "model": provider.Model,
		"apiKeyConfigured": provider.APIKeyConfigured, "apiKeyHint": provider.APIKeyHint, "credentialProtection": provider.CredentialProtection,
		"effectiveAt": provider.EffectiveAt, "expiresAt": provider.ExpiresAt, "status": provider.Status, "version": provider.Version, "updatedAt": provider.UpdatedAt,
	}
}

func (h *SaaSAdminHandler) TenantAIProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasTenantAIProviderStore(w)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		tenantID := positiveQueryInt(r, "tenantId", 0)
		if tenantID <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tenantId is required", nil)
			return
		}
		provider, found, err := store.SaaSTenantAIProvider(r.Context(), tenantID)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		if !found {
			writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"configured": false, "provider": SaaSTenantAIProviderPublicPayload(SaaSTenantAIProvider{TenantID: tenantID, CredentialProtection: "unconfigured"})})
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"configured": true, "provider": SaaSTenantAIProviderPublicPayload(provider)})
		return
	}
	var input SaaSTenantAIProviderInput
	if err := decodeSaaSAdminAccessJSON(r, &input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := NormalizeAndValidateSaaSTenantAIProviderInput(&input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	input.ActorUserID, input.ActorTenantID = user.ID, user.TenantID
	before, found, err := store.SaaSTenantAIProvider(r.Context(), input.TenantID)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if !found && strings.TrimSpace(input.APIKey) == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "apiKey is required when creating a tenant AI provider", nil)
		return
	}
	if found && before.ProviderCode != input.ProviderCode && strings.TrimSpace(input.APIKey) == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "apiKey is required when changing providerCode", nil)
		return
	}
	provider, err := store.SaveSaaSTenantAIProvider(r.Context(), input)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"provider": SaaSTenantAIProviderPublicPayload(provider)})
}

func SaaSTenantAIProviderAuditPayload(provider SaaSTenantAIProvider, keyChanged bool) map[string]any {
	host := ""
	if parsed, err := url.Parse(provider.BaseURL); err == nil && parsed != nil {
		host = parsed.Hostname()
	}
	return map[string]any{
		"providerCode": provider.ProviderCode, "model": provider.Model, "baseURLHost": host,
		"effectiveAt": provider.EffectiveAt, "expiresAt": provider.ExpiresAt, "status": provider.Status,
		"keyChanged": keyChanged, "version": provider.Version,
	}
}

func (h *SaaSAdminHandler) saasTenantAIProviderStore(w http.ResponseWriter) (SaaSTenantAIProviderStore, bool) {
	store, ok := h.store.(SaaSTenantAIProviderStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "tenant AI provider store is not configured", nil)
		return nil, false
	}
	return store, true
}
