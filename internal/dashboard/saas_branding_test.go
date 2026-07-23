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

type fakeSaaSAdminBrandingStore struct {
	*fakeSaaSAdminStore
	profiles        []SaaSBrandingProfile
	profile         SaaSBrandingProfile
	updateResult    SaaSAdminBrandingProfileUpdateResult
	lastOptions     SaaSAdminBrandingOptions
	lastUpdate      SaaSAdminBrandingProfileUpdate
	listCalls       int
	profileCalls    int
	updateCalls     int
	profileByTenant map[int]SaaSBrandingProfile
}

func (s *fakeSaaSAdminBrandingStore) SaaSBrandingProfile(_ context.Context, tenantID int) (SaaSBrandingProfile, error) {
	s.profileCalls++
	if s.profileByTenant != nil {
		if profile, ok := s.profileByTenant[tenantID]; ok {
			return profile, nil
		}
	}
	return s.profile, nil
}

func (s *fakeSaaSAdminBrandingStore) SaaSAdminBrandingProfiles(_ context.Context, options SaaSAdminBrandingOptions) ([]SaaSBrandingProfile, error) {
	s.lastOptions = options
	s.listCalls++
	return s.profiles, nil
}

func (s *fakeSaaSAdminBrandingStore) UpsertSaaSAdminBrandingProfile(_ context.Context, input SaaSAdminBrandingProfileUpdate) (SaaSAdminBrandingProfileUpdateResult, error) {
	s.lastUpdate = input
	s.updateCalls++
	return s.updateResult, nil
}

func TestNormalizeSaaSBrandingProfileNormalizesEveryEmptyLink(t *testing.T) {
	profile := NormalizeSaaSBrandingProfile(SaaSBrandingProfile{
		TenantID: 3, WebsiteURL: "", SupportURL: "", DocsURL: "",
		PrimaryColor: "bad", AccentColor: "bad",
	})
	if profile.WebsiteURL != "" || profile.SupportURL != "" || profile.DocsURL != "" {
		t.Fatalf("links = website:%q support:%q docs:%q", profile.WebsiteURL, profile.SupportURL, profile.DocsURL)
	}
	if profile.PrimaryColor != "#1769AA" || profile.AccentColor != "#0F578F" {
		t.Fatalf("colors = %q %q", profile.PrimaryColor, profile.AccentColor)
	}
}

func TestNormalizeAndValidateSaaSBrandingUpdate(t *testing.T) {
	input := SaaSAdminBrandingProfileUpdate{
		TenantID: 8, Status: " ACTIVE ", ProductName: " 产品云 ", ProductSubtitle: " 私域运营 ",
		LogoURL: "/img/brand.png", FaviconURL: "/favicon.ico", LoginBackgroundURL: "/static/login.png",
		PrimaryColor: "#1a2b3c", AccentColor: "#4d5e6f", WebsiteURL: "https://example.com/product",
		SupportURL: "/support", SupportQRURL: "/img/support.png", SupportEmail: "HELP@example.com",
		DocsURL: "/docs", FooterText: " 版权所有 ", ExpectedVersion: 2,
	}
	if err := NormalizeAndValidateSaaSBrandingUpdate(&input); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if input.Status != SaaSBrandingStatusActive || input.ProductName != "产品云" || input.ProductShortName != "产品云" {
		t.Fatalf("normalized identity = %+v", input)
	}
	if input.PrimaryColor != "#1A2B3C" || input.AccentColor != "#4D5E6F" || input.SupportEmail != "help@example.com" {
		t.Fatalf("normalized style/contact = %+v", input)
	}
}

func TestNormalizeAndValidateSaaSBrandingUpdateRejectsUnsafeValues(t *testing.T) {
	base := SaaSAdminBrandingProfileUpdate{
		TenantID: 8, Status: SaaSBrandingStatusActive, ProductName: "产品云",
		LogoURL: "/img/brand.png", FaviconURL: "/favicon.ico", LoginBackgroundURL: "/static/login.png",
		PrimaryColor: "#1A2B3C", AccentColor: "#4D5E6F",
	}
	tests := []struct {
		name   string
		mutate func(*SaaSAdminBrandingProfileUpdate)
	}{
		{name: "external asset", mutate: func(input *SaaSAdminBrandingProfileUpdate) { input.LogoURL = "https://cdn.example.com/logo.png" }},
		{name: "path traversal", mutate: func(input *SaaSAdminBrandingProfileUpdate) { input.LogoURL = "/img/../secret" }},
		{name: "insecure link", mutate: func(input *SaaSAdminBrandingProfileUpdate) { input.WebsiteURL = "http://example.com" }},
		{name: "scheme relative link", mutate: func(input *SaaSAdminBrandingProfileUpdate) { input.SupportURL = "//example.com/help" }},
		{name: "bad color", mutate: func(input *SaaSAdminBrandingProfileUpdate) { input.PrimaryColor = "red" }},
		{name: "bad email", mutate: func(input *SaaSAdminBrandingProfileUpdate) { input.SupportEmail = "broken@" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := base
			test.mutate(&input)
			if err := NormalizeAndValidateSaaSBrandingUpdate(&input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestSaaSAdminBrandingHandlersRequirePlatformAndPreserveActor(t *testing.T) {
	store := &fakeSaaSAdminBrandingStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			7: {ID: 7, TenantID: 9, IsSuperAdmin: 1},
		}},
		profiles: []SaaSBrandingProfile{{TenantID: 9, ProductName: "客户云"}},
		updateResult: SaaSAdminBrandingProfileUpdateResult{
			Profile: SaaSBrandingProfile{TenantID: 9, ProductName: "客户云", Version: 3}, OperationID: 88,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	listReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/brandingProfiles?tenantId=9&status=configured&keyword=%E5%AE%A2%E6%88%B7&limit=20", nil)
	listReq.Header.Set("X-Mochat-Go-User-ID", "1")
	listRec := httptest.NewRecorder()
	handler.BrandingProfiles(listRec, listReq)
	if listRec.Code != http.StatusOK || store.listCalls != 1 {
		t.Fatalf("list status=%d calls=%d body=%s", listRec.Code, store.listCalls, listRec.Body.String())
	}
	if store.lastOptions.TenantID != 9 || store.lastOptions.Status != SaaSBrandingFilterConfigured || store.lastOptions.Keyword != "客户" || store.lastOptions.Limit != 20 {
		t.Fatalf("options = %+v", store.lastOptions)
	}
	data := decodeSaaSAdminResponse(t, listRec)
	license := data["license"].(map[string]any)
	if license["licenseType"] != SaaSBrandingLicenseType {
		t.Fatalf("license = %+v", license)
	}

	payload, _ := json.Marshal(SaaSAdminBrandingProfileUpdate{
		TenantID: 9, Status: SaaSBrandingStatusActive, ProductName: "客户云", ProductShortName: "客户云",
		ProductSubtitle: "客户运营平台", LogoURL: "/img/logo.png", FaviconURL: "/favicon.ico",
		LoginBackgroundURL: "/static/login.png", PrimaryColor: "#123456", AccentColor: "#234567",
		SupportURL: "/support", DocsURL: "https://docs.example.com", ExpectedVersion: 2,
	})
	updateReq := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/brandingProfile", bytes.NewReader(payload))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("X-Mochat-Go-User-ID", "1")
	updateRec := httptest.NewRecorder()
	handler.BrandingProfile(updateRec, updateReq)
	if updateRec.Code != http.StatusOK || store.updateCalls != 1 {
		t.Fatalf("update status=%d calls=%d body=%s", updateRec.Code, store.updateCalls, updateRec.Body.String())
	}
	if store.lastUpdate.ActorUserID != 1 || store.lastUpdate.ActorTenantID != 1 || store.lastUpdate.ExpectedVersion != 2 {
		t.Fatalf("update = %+v", store.lastUpdate)
	}

	tenantReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/brandingProfiles", nil)
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "7")
	tenantRec := httptest.NewRecorder()
	handler.BrandingProfiles(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden || store.listCalls != 1 {
		t.Fatalf("tenant status=%d listCalls=%d body=%s", tenantRec.Code, store.listCalls, tenantRec.Body.String())
	}
}

func TestSaaSAdminBrandingProfileRejectsExternalAssetBeforeStore(t *testing.T) {
	store := &fakeSaaSAdminBrandingStore{fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	payload := `{"tenantId":9,"status":"active","productName":"客户云","logoUrl":"https://cdn.example.com/logo.png","primaryColor":"#123456","accentColor":"#234567"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/brandingProfile", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.BrandingProfile(rec, req)
	if rec.Code != http.StatusBadRequest || store.updateCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.updateCalls, rec.Body.String())
	}
}

func TestExternalTenantUsesAuthenticatedTenantBrandingWithoutRemoteDependency(t *testing.T) {
	store := &fakeSaaSAdminBrandingStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, TenantID: 9}}},
		profileByTenant: map[int]SaaSBrandingProfile{9: {
			TenantID: 9, TenantName: "租户九", Status: SaaSBrandingStatusActive, ProductName: "客户云",
			ProductShortName: "客户云", ProductSubtitle: "客户运营", LogoURL: "/img/logo.png", FaviconURL: "/favicon.ico",
			LoginBackgroundURL: "/static/login.png", PrimaryColor: "#123456", AccentColor: "#234567",
			SupportQRURL: "/img/support.png", SupportURL: "/support", DocsURL: "https://docs.example.com",
		}},
	}
	handler := NewExternalTenantHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/external/tenantIndex", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.TenantIndex(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	branding := data["branding"].(map[string]any)
	if branding["tenantId"].(float64) != 9 || branding["productName"] != "客户云" || data["licenseContactLink"] != "/img/support.png" {
		t.Fatalf("data = %+v", data)
	}
	license := data["license"].(map[string]any)
	if license["licenseType"] != "GPL-3.0" {
		t.Fatalf("license = %+v", license)
	}
	body := rec.Body.String()
	if strings.Contains(body, "api.mo.chat") || strings.Contains(body, "oss.mo.chat") {
		t.Fatalf("remote dependency leaked: %s", body)
	}
}

func TestExternalTenantDisabledBrandingFallsBackWithinAuthenticatedTenant(t *testing.T) {
	store := &fakeSaaSAdminBrandingStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, TenantID: 9}}},
		profileByTenant: map[int]SaaSBrandingProfile{9: {
			TenantID: 9, TenantName: "租户九", Status: SaaSBrandingStatusDisabled, ProductName: "停用品牌",
		}},
	}
	handler := NewExternalTenantHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/external/tenantIndex", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.TenantIndex(rec, req)
	data := decodeSaaSAdminResponse(t, rec)
	branding := data["branding"].(map[string]any)
	if branding["tenantId"].(float64) != 9 || branding["tenantName"] != "租户九" || branding["productName"] != "MoChat Go" {
		t.Fatalf("branding = %+v", branding)
	}
}
