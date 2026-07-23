package dashboard

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"net/url"
	pathpkg "path"
	"regexp"
	"strings"
)

const (
	SaaSBrandingStatusActive   = "active"
	SaaSBrandingStatusDisabled = "disabled"

	SaaSBrandingFilterAll          = "all"
	SaaSBrandingFilterConfigured   = "configured"
	SaaSBrandingFilterUnconfigured = "unconfigured"

	SaaSBrandingLicenseType = "GPL-3.0"
	SaaSBrandingLicenseNote = "基于 GPL-3.0 开源项目改造；对外分发须同时提供对应源码、许可证和修改说明"

	SaaSAdminOperationActionBrandingUpdate = "saas.admin.branding.update"
	SaaSAdminOperationTargetBranding       = "saas_branding_profile"
)

var saasBrandingColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type SaaSBrandingProfile struct {
	TenantID           int    `json:"tenantId"`
	TenantName         string `json:"tenantName"`
	Configured         bool   `json:"configured"`
	Status             string `json:"status"`
	ProductName        string `json:"productName"`
	ProductShortName   string `json:"productShortName"`
	ProductSubtitle    string `json:"productSubtitle"`
	LogoURL            string `json:"logoUrl"`
	FaviconURL         string `json:"faviconUrl"`
	LoginBackgroundURL string `json:"loginBackgroundUrl"`
	PrimaryColor       string `json:"primaryColor"`
	AccentColor        string `json:"accentColor"`
	WebsiteURL         string `json:"websiteUrl"`
	SupportURL         string `json:"supportUrl"`
	SupportQRURL       string `json:"supportQrUrl"`
	SupportEmail       string `json:"supportEmail"`
	DocsURL            string `json:"docsUrl"`
	FooterText         string `json:"footerText"`
	Version            int    `json:"version"`
	UpdatedBy          int    `json:"updatedBy"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

type SaaSAdminBrandingOptions struct {
	TenantID int
	Status   string
	Keyword  string
	Limit    int
}

type SaaSAdminBrandingProfileUpdate struct {
	TenantID           int    `json:"tenantId"`
	Status             string `json:"status"`
	ProductName        string `json:"productName"`
	ProductShortName   string `json:"productShortName"`
	ProductSubtitle    string `json:"productSubtitle"`
	LogoURL            string `json:"logoUrl"`
	FaviconURL         string `json:"faviconUrl"`
	LoginBackgroundURL string `json:"loginBackgroundUrl"`
	PrimaryColor       string `json:"primaryColor"`
	AccentColor        string `json:"accentColor"`
	WebsiteURL         string `json:"websiteUrl"`
	SupportURL         string `json:"supportUrl"`
	SupportQRURL       string `json:"supportQrUrl"`
	SupportEmail       string `json:"supportEmail"`
	DocsURL            string `json:"docsUrl"`
	FooterText         string `json:"footerText"`
	ExpectedVersion    int    `json:"expectedVersion"`
	ActorUserID        int    `json:"-"`
	ActorTenantID      int    `json:"-"`
}

type SaaSAdminBrandingProfileUpdateResult struct {
	Profile     SaaSBrandingProfile
	OperationID int64
}

type SaaSBrandingReader interface {
	SaaSBrandingProfile(context.Context, int) (SaaSBrandingProfile, error)
}

type SaaSAdminBrandingStore interface {
	SaaSBrandingReader
	SaaSAdminBrandingProfiles(context.Context, SaaSAdminBrandingOptions) ([]SaaSBrandingProfile, error)
	UpsertSaaSAdminBrandingProfile(context.Context, SaaSAdminBrandingProfileUpdate) (SaaSAdminBrandingProfileUpdateResult, error)
}

func DefaultSaaSBrandingProfile(tenantID int, tenantName string) SaaSBrandingProfile {
	return SaaSBrandingProfile{
		TenantID: tenantID, TenantName: tenantName, Status: SaaSBrandingStatusActive,
		ProductName: "MoChat Go", ProductShortName: "MoChat Go", ProductSubtitle: "企业微信客户运营平台",
		LogoURL: "/img/logo-no-word.c30823d0.png", FaviconURL: "/favicon.ico",
		LoginBackgroundURL: "/img/background.e06f03d5.png", PrimaryColor: "#1769AA", AccentColor: "#0F578F",
	}
}

func SaaSBrandingProfileFromTenant(tenantID int, tenantName, logoURL, backgroundURL, websiteURL, footerText string) SaaSBrandingProfile {
	profile := DefaultSaaSBrandingProfile(tenantID, tenantName)
	if value, err := validateSaaSBrandingAssetURL(logoURL); err == nil && value != "" {
		profile.LogoURL = value
	}
	if value, err := validateSaaSBrandingAssetURL(backgroundURL); err == nil && value != "" {
		profile.LoginBackgroundURL = value
	}
	if value, err := validateSaaSBrandingLinkURL(websiteURL); err == nil {
		profile.WebsiteURL = value
	}
	profile.FooterText = strings.TrimSpace(footerText)
	return profile
}

func NormalizeSaaSBrandingProfile(profile SaaSBrandingProfile) SaaSBrandingProfile {
	defaults := DefaultSaaSBrandingProfile(profile.TenantID, profile.TenantName)
	if strings.TrimSpace(profile.Status) == "" {
		profile.Status = defaults.Status
	}
	if strings.TrimSpace(profile.ProductName) == "" {
		profile.ProductName = defaults.ProductName
	}
	if strings.TrimSpace(profile.ProductShortName) == "" {
		profile.ProductShortName = profile.ProductName
	}
	if strings.TrimSpace(profile.ProductSubtitle) == "" {
		profile.ProductSubtitle = defaults.ProductSubtitle
	}
	if value, err := validateSaaSBrandingAssetURL(profile.LogoURL); err != nil || value == "" {
		profile.LogoURL = defaults.LogoURL
	} else {
		profile.LogoURL = value
	}
	if value, err := validateSaaSBrandingAssetURL(profile.FaviconURL); err != nil || value == "" {
		profile.FaviconURL = defaults.FaviconURL
	} else {
		profile.FaviconURL = value
	}
	if value, err := validateSaaSBrandingAssetURL(profile.LoginBackgroundURL); err != nil || value == "" {
		profile.LoginBackgroundURL = defaults.LoginBackgroundURL
	} else {
		profile.LoginBackgroundURL = value
	}
	if value, err := validateSaaSBrandingAssetURL(profile.SupportQRURL); err != nil {
		profile.SupportQRURL = ""
	} else {
		profile.SupportQRURL = value
	}
	profile.WebsiteURL = normalizeSaaSBrandingLinkURL(profile.WebsiteURL)
	profile.SupportURL = normalizeSaaSBrandingLinkURL(profile.SupportURL)
	profile.DocsURL = normalizeSaaSBrandingLinkURL(profile.DocsURL)
	if !saasBrandingColorPattern.MatchString(profile.PrimaryColor) {
		profile.PrimaryColor = defaults.PrimaryColor
	}
	if !saasBrandingColorPattern.MatchString(profile.AccentColor) {
		profile.AccentColor = defaults.AccentColor
	}
	return profile
}

func normalizeSaaSBrandingLinkURL(source string) string {
	value, err := validateSaaSBrandingLinkURL(source)
	if err != nil {
		return ""
	}
	return value
}

func (h *SaaSAdminHandler) BrandingProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasAdminBrandingStore(w)
	if !ok {
		return
	}
	options, err := saasAdminBrandingOptions(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	profiles, err := store.SaaSAdminBrandingProfiles(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"profiles": profiles,
		"license":  map[string]string{"licenseType": SaaSBrandingLicenseType, "licenseNote": SaaSBrandingLicenseNote},
	})
}

func (h *SaaSAdminHandler) BrandingProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasAdminBrandingStore(w)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		tenantID := positiveQueryInt(r, "tenantId", 0)
		if tenantID <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tenantId 必填", nil)
			return
		}
		profile, err := store.SaaSBrandingProfile(r.Context(), tenantID)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"profile": profile})
		return
	}
	var input SaaSAdminBrandingProfileUpdate
	if err := decodeSaaSAdminAccessJSON(r, &input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := NormalizeAndValidateSaaSBrandingUpdate(&input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	input.ActorUserID, input.ActorTenantID = user.ID, user.TenantID
	result, err := store.UpsertSaaSAdminBrandingProfile(r.Context(), input)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"profile": result.Profile, "operationId": result.OperationID})
}

func (h *SaaSAdminHandler) saasAdminBrandingStore(w http.ResponseWriter) (SaaSAdminBrandingStore, bool) {
	store, ok := h.store.(SaaSAdminBrandingStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "SaaS branding store is not configured", nil)
		return nil, false
	}
	return store, true
}

func saasAdminBrandingOptions(r *http.Request) (SaaSAdminBrandingOptions, error) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" {
		status = SaaSBrandingFilterAll
	}
	if status != SaaSBrandingFilterAll && status != SaaSBrandingFilterConfigured && status != SaaSBrandingFilterUnconfigured && status != SaaSBrandingStatusActive && status != SaaSBrandingStatusDisabled {
		return SaaSAdminBrandingOptions{}, errors.New("status 必须是 all/configured/unconfigured/active/disabled")
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len([]rune(keyword)) > 80 {
		return SaaSAdminBrandingOptions{}, errors.New("keyword 最多 80 个字符")
	}
	limit := positiveQueryInt(r, "limit", 100)
	if limit > 100 {
		limit = 100
	}
	return SaaSAdminBrandingOptions{TenantID: positiveQueryInt(r, "tenantId", 0), Status: status, Keyword: keyword, Limit: limit}, nil
}

func NormalizeAndValidateSaaSBrandingUpdate(input *SaaSAdminBrandingProfileUpdate) error {
	if input == nil {
		return errors.New("品牌配置不能为空")
	}
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.ProductName = strings.TrimSpace(input.ProductName)
	input.ProductShortName = strings.TrimSpace(input.ProductShortName)
	input.ProductSubtitle = strings.TrimSpace(input.ProductSubtitle)
	input.SupportEmail = strings.ToLower(strings.TrimSpace(input.SupportEmail))
	input.FooterText = strings.TrimSpace(input.FooterText)
	input.PrimaryColor = strings.ToUpper(strings.TrimSpace(input.PrimaryColor))
	input.AccentColor = strings.ToUpper(strings.TrimSpace(input.AccentColor))
	if input.ProductShortName == "" {
		input.ProductShortName = input.ProductName
	}
	if input.TenantID <= 0 || input.ExpectedVersion < 0 {
		return errors.New("tenantId 或 expectedVersion 无效")
	}
	if input.Status != SaaSBrandingStatusActive && input.Status != SaaSBrandingStatusDisabled {
		return errors.New("status 必须是 active 或 disabled")
	}
	if input.ProductName == "" || len([]rune(input.ProductName)) > 80 || len([]rune(input.ProductShortName)) > 32 || len([]rune(input.ProductSubtitle)) > 160 || len([]rune(input.FooterText)) > 160 {
		return errors.New("产品名称必填且最多 80 字，简称最多 32 字，副标题和页脚最多 160 字")
	}
	if !saasBrandingColorPattern.MatchString(input.PrimaryColor) || !saasBrandingColorPattern.MatchString(input.AccentColor) {
		return errors.New("品牌颜色必须是 #RRGGBB")
	}
	var err error
	if input.LogoURL, err = validateSaaSBrandingAssetURL(input.LogoURL); err != nil {
		return errors.New("logoUrl " + err.Error())
	}
	if input.FaviconURL, err = validateSaaSBrandingAssetURL(input.FaviconURL); err != nil {
		return errors.New("faviconUrl " + err.Error())
	}
	if input.LoginBackgroundURL, err = validateSaaSBrandingAssetURL(input.LoginBackgroundURL); err != nil {
		return errors.New("loginBackgroundUrl " + err.Error())
	}
	if input.SupportQRURL, err = validateSaaSBrandingAssetURL(input.SupportQRURL); err != nil {
		return errors.New("supportQrUrl " + err.Error())
	}
	if input.WebsiteURL, err = validateSaaSBrandingLinkURL(input.WebsiteURL); err != nil {
		return errors.New("websiteUrl " + err.Error())
	}
	if input.SupportURL, err = validateSaaSBrandingLinkURL(input.SupportURL); err != nil {
		return errors.New("supportUrl " + err.Error())
	}
	if input.DocsURL, err = validateSaaSBrandingLinkURL(input.DocsURL); err != nil {
		return errors.New("docsUrl " + err.Error())
	}
	if input.SupportEmail != "" {
		address, err := mail.ParseAddress(input.SupportEmail)
		if err != nil || !strings.EqualFold(address.Address, input.SupportEmail) || len(input.SupportEmail) > 254 {
			return errors.New("supportEmail 格式错误")
		}
	}
	return nil
}

func validateSaaSBrandingAssetURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil || parsed.Fragment != "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(value, "//") {
		return "", errors.New("必须是同源绝对路径")
	}
	cleaned := pathpkg.Clean(parsed.Path)
	if cleaned != parsed.Path || strings.Contains(parsed.Path, "..") {
		return "", errors.New("路径不安全")
	}
	if cleaned != "/favicon.ico" && !strings.HasPrefix(cleaned, "/img/") && !strings.HasPrefix(cleaned, "/static/") {
		return "", errors.New("只允许 /img/、/static/ 或 /favicon.ico")
	}
	return parsed.String(), nil
}

func validateSaaSBrandingLinkURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len(value) > 500 {
		return "", errors.New("最多 500 个字符")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil {
		return "", errors.New("格式错误")
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "https" || parsed.Host == "" {
			return "", errors.New("外部链接必须使用 HTTPS")
		}
		return parsed.String(), nil
	}
	if parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(value, "//") || pathpkg.Clean(parsed.Path) != parsed.Path {
		return "", errors.New("相对链接必须是安全的同源绝对路径")
	}
	return parsed.String(), nil
}
