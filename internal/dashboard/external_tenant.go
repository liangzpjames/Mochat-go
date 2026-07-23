package dashboard

import (
	"context"
	"net/http"
	"time"
)

type ExternalTenantStore interface {
	SaaSBrandingReader
	UserByID(context.Context, int) (User, bool, error)
}

type ExternalTenantHandler struct {
	store           ExternalTenantStore
	resolver        UserIDResolver
	defaultTenantID int
}

func NewExternalTenantHandler(store ExternalTenantStore, resolver UserIDResolver, defaultTenantID int) *ExternalTenantHandler {
	if defaultTenantID <= 0 {
		defaultTenantID = 1
	}
	return &ExternalTenantHandler{store: store, resolver: resolver, defaultTenantID: defaultTenantID}
}

func (h *ExternalTenantHandler) TenantIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	tenantID := h.defaultTenantID
	if h.store != nil {
		if h.resolver != nil {
			if userID, err := h.resolver.UserID(r); err == nil && userID > 0 {
				if user, found, err := h.store.UserByID(r.Context(), userID); err == nil && found && user.TenantID > 0 {
					tenantID = user.TenantID
				}
			}
		}
	}
	profile := DefaultSaaSBrandingProfile(tenantID, "")
	if h.store != nil {
		if stored, err := h.store.SaaSBrandingProfile(r.Context(), tenantID); err == nil {
			if stored.Status == SaaSBrandingStatusActive {
				profile = NormalizeSaaSBrandingProfile(stored)
			} else {
				profile.TenantName = stored.TenantName
			}
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"license": map[string]any{
			"licenseType": SaaSBrandingLicenseType,
			"licenseNote": SaaSBrandingLicenseNote,
		},
		"licenseContactLink": profile.SupportQRURL,
		"branding":           profile,
		"news": []map[string]any{
			{
				"id":        1,
				"title":     profile.ProductName + " 独立版",
				"createdAt": time.Now().Format("2006-01-02"),
			},
		},
		"guide": map[string]any{
			"docLink": profile.DocsURL,
			"faqLink": profile.SupportURL,
		},
	})
}
