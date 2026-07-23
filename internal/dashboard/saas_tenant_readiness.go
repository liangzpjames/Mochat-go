package dashboard

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	SaaSAdminTenantReadinessStateAll       = "all"
	SaaSAdminTenantReadinessStateReady     = "ready"
	SaaSAdminTenantReadinessStateAttention = "attention"
	SaaSAdminTenantReadinessStateBlocked   = "blocked"
	SaaSAdminTenantReadinessScopeBusiness  = "business_tenants"

	SaaSAdminTenantReadinessCheckPassed  = "passed"
	SaaSAdminTenantReadinessCheckWarning = "warning"
	SaaSAdminTenantReadinessCheckFailed  = "failed"

	saasAdminTenantReadinessRequiredSeedCount = 3
)

type SaaSAdminTenantReadinessOptions struct {
	TenantID              int
	PlatformAdminTenantID int
	Keyword               string
	State                 string
	Limit                 int
}

type SaaSAdminTenantReadinessFacts struct {
	TenantID                        int
	TenantName                      string
	TenantStatus                    int
	ActiveSuperAdminCount           int
	PackageCode                     string
	PackageName                     string
	PackageStatus                   int
	PackageExpiresAt                string
	SubscriptionID                  int64
	SubscriptionStatus              string
	SubscriptionTrialEndsAt         string
	SubscriptionCurrentPeriodEndsAt string
	SubscriptionGraceEndsAt         string
	SubscriptionCancelAtPeriodEnd   bool
	CoreSeedCount                   int
	BoundCorpCount                  int
	ConfiguredCorpCredentialCount   int
	ProtectedCorpCredentialCount    int
	IdentityPolicyStatus            string
	EnabledNotificationPolicyCount  int
	SecureNotificationPolicyCount   int
	BrandingStatus                  string
	PrimaryVerifiedDomainCount      int
	ReadyPrimaryDomainCount         int
}

type SaaSAdminTenantReadinessFactPage struct {
	Facts     []SaaSAdminTenantReadinessFacts
	Truncated bool
}

type SaaSAdminTenantReadinessStore interface {
	SaaSAdminTenantReadinessFacts(ctx context.Context, options SaaSAdminTenantReadinessOptions) (SaaSAdminTenantReadinessFactPage, error)
}

type SaaSAdminTenantReadinessCheck struct {
	Code               string `json:"code"`
	Label              string `json:"label"`
	Category           string `json:"category"`
	Required           bool   `json:"required"`
	Passed             bool   `json:"passed"`
	Status             string `json:"status"`
	Detail             string `json:"detail"`
	ActionWorkspace    string `json:"actionWorkspace"`
	ActionSectionLabel string `json:"actionSectionLabel"`
}

type SaaSAdminTenantReadinessItem struct {
	TenantID                 int                             `json:"tenantId"`
	TenantName               string                          `json:"tenantName"`
	TenantStatus             int                             `json:"tenantStatus"`
	PackageCode              string                          `json:"packageCode"`
	SubscriptionStatus       string                          `json:"subscriptionStatus"`
	State                    string                          `json:"state"`
	CoreReady                bool                            `json:"coreReady"`
	CompletionPercent        int                             `json:"completionPercent"`
	PassedCheckCount         int                             `json:"passedCheckCount"`
	CheckCount               int                             `json:"checkCount"`
	RequiredPassedCheckCount int                             `json:"requiredPassedCheckCount"`
	RequiredCheckCount       int                             `json:"requiredCheckCount"`
	FailedCheckCount         int                             `json:"failedCheckCount"`
	WarningCheckCount        int                             `json:"warningCheckCount"`
	Checks                   []SaaSAdminTenantReadinessCheck `json:"checks"`
}

type SaaSAdminTenantReadinessSummary struct {
	TenantCount              int `json:"tenantCount"`
	FilteredCount            int `json:"filteredCount"`
	ReadyCount               int `json:"readyCount"`
	AttentionCount           int `json:"attentionCount"`
	BlockedCount             int `json:"blockedCount"`
	CoreReadyCount           int `json:"coreReadyCount"`
	AverageCompletionPercent int `json:"averageCompletionPercent"`
}

type SaaSAdminTenantReadinessFilters struct {
	TenantID int    `json:"tenantId"`
	Keyword  string `json:"keyword"`
	State    string `json:"state"`
	Limit    int    `json:"limit"`
}

type SaaSAdminTenantReadinessReport struct {
	GeneratedAt           string                          `json:"generatedAt"`
	Scope                 string                          `json:"scope"`
	PlatformAdminTenantID int                             `json:"platformAdminTenantId"`
	Filters               SaaSAdminTenantReadinessFilters `json:"filters"`
	Summary               SaaSAdminTenantReadinessSummary `json:"summary"`
	Tenants               []SaaSAdminTenantReadinessItem  `json:"tenants"`
	Truncated             bool                            `json:"truncated"`
}

func (h *SaaSAdminHandler) TenantReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasAdminTenantReadinessStore(w)
	if !ok {
		return
	}
	options, err := saasAdminTenantReadinessOptions(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	options.PlatformAdminTenantID = h.platformAdminTenantID
	if options.TenantID == options.PlatformAdminTenantID {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "平台管理租户不参与上线准备度", nil)
		return
	}
	page, err := store.SaaSAdminTenantReadinessFacts(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	report := BuildSaaSAdminTenantReadinessReport(page, options, time.Now())
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", report)
}

func (h *SaaSAdminHandler) saasAdminTenantReadinessStore(w http.ResponseWriter) (SaaSAdminTenantReadinessStore, bool) {
	store, ok := h.store.(SaaSAdminTenantReadinessStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "SaaS tenant readiness store is not configured", nil)
		return nil, false
	}
	return store, true
}

func saasAdminTenantReadinessOptions(r *http.Request) (SaaSAdminTenantReadinessOptions, error) {
	options := SaaSAdminTenantReadinessOptions{
		Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")),
		State:   strings.ToLower(strings.TrimSpace(r.URL.Query().Get("state"))),
		Limit:   50,
	}
	if options.State == "" {
		options.State = SaaSAdminTenantReadinessStateAll
	}
	if !saasAdminTenantReadinessStateValid(options.State) {
		return SaaSAdminTenantReadinessOptions{}, errors.New("state 必须是 all/ready/attention/blocked")
	}
	if len([]rune(options.Keyword)) > 80 {
		return SaaSAdminTenantReadinessOptions{}, errors.New("keyword 最多 80 个字符")
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("tenantId")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return SaaSAdminTenantReadinessOptions{}, errors.New("tenantId 必须是正整数")
		}
		options.TenantID = value
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return SaaSAdminTenantReadinessOptions{}, errors.New("limit 必须是正整数")
		}
		options.Limit = value
	}
	if options.Limit > saasAdminListMaxLimit {
		options.Limit = saasAdminListMaxLimit
	}
	return options, nil
}

func saasAdminTenantReadinessStateValid(value string) bool {
	switch strings.TrimSpace(value) {
	case SaaSAdminTenantReadinessStateAll,
		SaaSAdminTenantReadinessStateReady,
		SaaSAdminTenantReadinessStateAttention,
		SaaSAdminTenantReadinessStateBlocked:
		return true
	default:
		return false
	}
}

func BuildSaaSAdminTenantReadinessReport(page SaaSAdminTenantReadinessFactPage, options SaaSAdminTenantReadinessOptions, now time.Time) SaaSAdminTenantReadinessReport {
	if options.State == "" {
		options.State = SaaSAdminTenantReadinessStateAll
	}
	if options.Limit <= 0 {
		options.Limit = 50
	}
	if options.Limit > saasAdminListMaxLimit {
		options.Limit = saasAdminListMaxLimit
	}
	report := SaaSAdminTenantReadinessReport{
		GeneratedAt:           now.Format("2006-01-02 15:04:05"),
		Scope:                 SaaSAdminTenantReadinessScopeBusiness,
		PlatformAdminTenantID: options.PlatformAdminTenantID,
		Filters: SaaSAdminTenantReadinessFilters{
			TenantID: options.TenantID,
			Keyword:  options.Keyword,
			State:    options.State,
			Limit:    options.Limit,
		},
		Tenants:   make([]SaaSAdminTenantReadinessItem, 0),
		Truncated: page.Truncated,
	}
	all := make([]SaaSAdminTenantReadinessItem, 0, len(page.Facts))
	completionTotal := 0
	for _, facts := range page.Facts {
		item := buildSaaSAdminTenantReadinessItem(facts, now)
		all = append(all, item)
		report.Summary.TenantCount++
		completionTotal += item.CompletionPercent
		if item.CoreReady {
			report.Summary.CoreReadyCount++
		}
		switch item.State {
		case SaaSAdminTenantReadinessStateReady:
			report.Summary.ReadyCount++
		case SaaSAdminTenantReadinessStateAttention:
			report.Summary.AttentionCount++
		case SaaSAdminTenantReadinessStateBlocked:
			report.Summary.BlockedCount++
		}
	}
	if report.Summary.TenantCount > 0 {
		report.Summary.AverageCompletionPercent = (completionTotal + report.Summary.TenantCount/2) / report.Summary.TenantCount
	}
	sort.SliceStable(all, func(i, j int) bool {
		leftPriority := saasAdminTenantReadinessStatePriority(all[i].State)
		rightPriority := saasAdminTenantReadinessStatePriority(all[j].State)
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		if all[i].CompletionPercent != all[j].CompletionPercent {
			return all[i].CompletionPercent < all[j].CompletionPercent
		}
		return all[i].TenantID < all[j].TenantID
	})
	for _, item := range all {
		if options.State != SaaSAdminTenantReadinessStateAll && item.State != options.State {
			continue
		}
		report.Summary.FilteredCount++
		if len(report.Tenants) < options.Limit {
			report.Tenants = append(report.Tenants, item)
		}
	}
	return report
}

func buildSaaSAdminTenantReadinessItem(facts SaaSAdminTenantReadinessFacts, now time.Time) SaaSAdminTenantReadinessItem {
	checks := make([]SaaSAdminTenantReadinessCheck, 0, 11)
	checks = append(checks, newSaaSAdminTenantReadinessCheck(
		"tenant_active", "租户状态", "core", true, facts.TenantStatus == 1,
		saasAdminTenantStatusReadinessDetail(facts.TenantStatus), "tenant", "租户状态",
	))
	checks = append(checks, newSaaSAdminTenantReadinessCheck(
		"super_admin", "超级管理员", "core", true, facts.ActiveSuperAdminCount > 0,
		fmt.Sprintf("有效超级管理员 %d 人", facts.ActiveSuperAdminCount), "overview", "租户详情",
	))
	packageReady, packageDetail := saasAdminTenantPackageReadiness(facts, now)
	checks = append(checks, newSaaSAdminTenantReadinessCheck(
		"package", "有效套餐", "core", true, packageReady, packageDetail, "tenant", "租户套餐调整",
	))
	subscriptionReady, effectiveSubscriptionStatus, subscriptionDetail := saasAdminTenantSubscriptionReadiness(facts, now)
	checks = append(checks, newSaaSAdminTenantReadinessCheck(
		"subscription", "有效订阅", "core", true, subscriptionReady, subscriptionDetail, "tenant", "订阅生命周期",
	))
	checks = append(checks, newSaaSAdminTenantReadinessCheck(
		"baseline_seed", "基础配置", "core", true, facts.CoreSeedCount >= saasAdminTenantReadinessRequiredSeedCount,
		fmt.Sprintf("基础配置种子 %d/%d", facts.CoreSeedCount, saasAdminTenantReadinessRequiredSeedCount), "tenant", "套餐快照同步",
	))
	checks = append(checks, newSaaSAdminTenantReadinessCheck(
		"wecom_corp", "企业微信企业", "core", true, facts.BoundCorpCount > 0,
		fmt.Sprintf("已绑定企业 %d 个", facts.BoundCorpCount), "security", "企业微信凭据保护",
	))
	credentialReady := facts.BoundCorpCount > 0 &&
		facts.ConfiguredCorpCredentialCount == facts.BoundCorpCount &&
		facts.ProtectedCorpCredentialCount == facts.BoundCorpCount
	checks = append(checks, newSaaSAdminTenantReadinessCheck(
		"wecom_credentials", "企业微信凭据", "core", true, credentialReady,
		fmt.Sprintf("已配置 %d/%d，已加密 %d/%d", facts.ConfiguredCorpCredentialCount, facts.BoundCorpCount, facts.ProtectedCorpCredentialCount, facts.BoundCorpCount),
		"security", "企业微信凭据保护",
	))
	checks = append(checks, newSaaSAdminTenantReadinessCheck(
		"identity_policy", "身份安全策略", "core", true, strings.TrimSpace(facts.IdentityPolicyStatus) == "active",
		saasAdminTenantReadinessStatusDetail(facts.IdentityPolicyStatus, "未配置身份安全策略"), "security", "身份与访问安全中心",
	))
	notificationReady := facts.EnabledNotificationPolicyCount > 0 && facts.SecureNotificationPolicyCount > 0
	checks = append(checks, newSaaSAdminTenantReadinessCheck(
		"notification_policy", "通知策略", "delivery", false, notificationReady,
		fmt.Sprintf("已启用 %d 条，安全凭据 %d 条", facts.EnabledNotificationPolicyCount, facts.SecureNotificationPolicyCount), "notifications", "租户通知策略",
	))
	checks = append(checks, newSaaSAdminTenantReadinessCheck(
		"branding", "品牌档案", "delivery", false, strings.TrimSpace(facts.BrandingStatus) == "active",
		saasAdminTenantReadinessStatusDetail(facts.BrandingStatus, "未配置品牌档案"), "delivery", "品牌与白标中心",
	))
	checks = append(checks, newSaaSAdminTenantReadinessCheck(
		"primary_domain", "主域名交付", "delivery", false, facts.ReadyPrimaryDomainCount > 0,
		fmt.Sprintf("已验证主域名 %d 个，可交付 %d 个", facts.PrimaryVerifiedDomainCount, facts.ReadyPrimaryDomainCount), "delivery", "租户自定义域名中心",
	))

	item := SaaSAdminTenantReadinessItem{
		TenantID:           facts.TenantID,
		TenantName:         facts.TenantName,
		TenantStatus:       facts.TenantStatus,
		PackageCode:        facts.PackageCode,
		SubscriptionStatus: effectiveSubscriptionStatus,
		Checks:             checks,
		CheckCount:         len(checks),
	}
	for _, check := range checks {
		if check.Required {
			item.RequiredCheckCount++
		}
		if check.Passed {
			item.PassedCheckCount++
			if check.Required {
				item.RequiredPassedCheckCount++
			}
			continue
		}
		if check.Required {
			item.FailedCheckCount++
		} else {
			item.WarningCheckCount++
		}
	}
	item.CoreReady = item.RequiredPassedCheckCount == item.RequiredCheckCount
	if item.CheckCount > 0 {
		item.CompletionPercent = (item.PassedCheckCount*100 + item.CheckCount/2) / item.CheckCount
	}
	switch {
	case !item.CoreReady:
		item.State = SaaSAdminTenantReadinessStateBlocked
	case item.WarningCheckCount > 0:
		item.State = SaaSAdminTenantReadinessStateAttention
	default:
		item.State = SaaSAdminTenantReadinessStateReady
	}
	return item
}

func newSaaSAdminTenantReadinessCheck(code, label, category string, required, passed bool, detail, workspace, sectionLabel string) SaaSAdminTenantReadinessCheck {
	status := SaaSAdminTenantReadinessCheckPassed
	if !passed && required {
		status = SaaSAdminTenantReadinessCheckFailed
	} else if !passed {
		status = SaaSAdminTenantReadinessCheckWarning
	}
	return SaaSAdminTenantReadinessCheck{
		Code:               code,
		Label:              label,
		Category:           category,
		Required:           required,
		Passed:             passed,
		Status:             status,
		Detail:             detail,
		ActionWorkspace:    workspace,
		ActionSectionLabel: sectionLabel,
	}
}

func saasAdminTenantStatusReadinessDetail(status int) string {
	if status == 1 {
		return "租户状态正常"
	}
	if status == 2 {
		return "租户已停用"
	}
	return fmt.Sprintf("租户状态异常：%d", status)
}

func saasAdminTenantPackageReadiness(facts SaaSAdminTenantReadinessFacts, now time.Time) (bool, string) {
	if strings.TrimSpace(facts.PackageCode) == "" {
		return false, "未分配套餐"
	}
	if facts.PackageStatus != 1 {
		return false, fmt.Sprintf("套餐 %s 已停用", facts.PackageCode)
	}
	if expiresAt, ok := parseSaaSAdminNormalizedDateTime(facts.PackageExpiresAt); ok && !expiresAt.After(now) {
		return false, fmt.Sprintf("套餐 %s 已于 %s 到期", facts.PackageCode, facts.PackageExpiresAt)
	}
	if strings.TrimSpace(facts.PackageExpiresAt) == "" {
		return true, fmt.Sprintf("套餐 %s 长期有效", facts.PackageCode)
	}
	return true, fmt.Sprintf("套餐 %s 有效至 %s", facts.PackageCode, facts.PackageExpiresAt)
}

func saasAdminTenantSubscriptionReadiness(facts SaaSAdminTenantReadinessFacts, now time.Time) (bool, string, string) {
	if facts.SubscriptionID <= 0 {
		return false, "", "未创建订阅记录"
	}
	item := SaaSAdminSubscription{
		TenantStatus:        facts.TenantStatus,
		Status:              facts.SubscriptionStatus,
		TrialEndsAt:         facts.SubscriptionTrialEndsAt,
		CurrentPeriodEndsAt: facts.SubscriptionCurrentPeriodEndsAt,
		GraceEndsAt:         facts.SubscriptionGraceEndsAt,
		CancelAtPeriodEnd:   facts.SubscriptionCancelAtPeriodEnd,
	}
	effectiveStatus := SaaSAdminEffectiveSubscriptionStatus(item, now)
	return SaaSAdminSubscriptionAllowsAccess(effectiveStatus), effectiveStatus, SaaSAdminSubscriptionAccessReason(effectiveStatus)
}

func saasAdminTenantReadinessStatusDetail(status, empty string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return empty
	}
	return "当前状态：" + status
}

func saasAdminTenantReadinessStatePriority(state string) int {
	switch state {
	case SaaSAdminTenantReadinessStateBlocked:
		return 0
	case SaaSAdminTenantReadinessStateAttention:
		return 1
	case SaaSAdminTenantReadinessStateReady:
		return 2
	default:
		return 3
	}
}
