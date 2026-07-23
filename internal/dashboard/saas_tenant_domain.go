package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	SaaSTenantDomainStatusPending  = "pending"
	SaaSTenantDomainStatusActive   = "active"
	SaaSTenantDomainStatusDisabled = "disabled"
	SaaSTenantDomainStatusDeleted  = "deleted"

	SaaSTenantDomainVerificationDNS = "dns_txt"
	SaaSTenantDomainDNSPrefix       = "_mochat."
	SaaSTenantDomainTXTValuePrefix  = "mochat-domain-verification="
	SaaSTenantDomainMaxPerTenant    = 10

	SaaSTenantDomainActionCreate              = "create"
	SaaSTenantDomainActionVerify              = "verify"
	SaaSTenantDomainActionSetPrimary          = "set_primary"
	SaaSTenantDomainActionEnable              = "enable"
	SaaSTenantDomainActionDisable             = "disable"
	SaaSTenantDomainActionRotateToken         = "rotate_token"
	SaaSTenantDomainActionDelete              = "delete"
	SaaSTenantDomainApprovalPlanSchemaVersion = 1

	SaaSAdminOperationTargetTenantDomain = "saas_tenant_domain"
)

type SaaSTenantDomain struct {
	ID                 int64                    `json:"id"`
	TenantID           int                      `json:"tenantId"`
	TenantName         string                   `json:"tenantName"`
	Hostname           string                   `json:"hostname"`
	Status             string                   `json:"status"`
	IsPrimary          bool                     `json:"isPrimary"`
	VerificationMethod string                   `json:"verificationMethod"`
	VerificationToken  string                   `json:"verificationToken"`
	VerificationError  string                   `json:"verificationError"`
	LastVerificationAt string                   `json:"lastVerificationAt"`
	VerifiedAt         string                   `json:"verifiedAt"`
	Version            int                      `json:"version"`
	CreatedBy          int                      `json:"createdBy"`
	UpdatedBy          int                      `json:"updatedBy"`
	CreatedAt          string                   `json:"createdAt"`
	UpdatedAt          string                   `json:"updatedAt"`
	DeletedAt          string                   `json:"deletedAt"`
	Delivery           SaaSTenantDomainDelivery `json:"delivery"`
}

func (domain SaaSTenantDomain) VerificationRecordName() string {
	return SaaSTenantDomainDNSPrefix + domain.Hostname
}

func (domain SaaSTenantDomain) VerificationRecordValue() string {
	return SaaSTenantDomainTXTValuePrefix + domain.VerificationToken
}

func (domain SaaSTenantDomain) RoutingActive() bool {
	return domain.DeletedAt == "" && domain.Status == SaaSTenantDomainStatusActive && domain.VerifiedAt != ""
}

type SaaSAdminTenantDomainOptions struct {
	TenantID int
	Status   string
	Keyword  string
	Limit    int
}

type SaaSAdminTenantDomainCreate struct {
	TenantID                 int
	Hostname                 string
	Token                    string
	ActorUserID              int
	ActorTenantID            int
	ApprovalExecutionID      int64
	ApprovalExecutionVersion int
}

type SaaSAdminTenantDomainCreateTarget struct {
	TenantID     int
	TenantName   string
	TenantStatus int
	Hostname     string
	DomainCount  int
}

type SaaSAdminTenantDomainVerification struct {
	ID              int64
	ExpectedVersion int
	Success         bool
	ErrorMessage    string
	ActorUserID     int
	ActorTenantID   int
}

type SaaSAdminTenantDomainCommand struct {
	ID                       int64                                `json:"id"`
	Action                   string                               `json:"action"`
	ExpectedVersion          int                                  `json:"expectedVersion"`
	Token                    string                               `json:"-"`
	ActorUserID              int                                  `json:"-"`
	ActorTenantID            int                                  `json:"-"`
	ApprovalPlan             *SaaSTenantDomainCommandApprovalPlan `json:"-"`
	ApprovalExecutionID      int64                                `json:"-"`
	ApprovalExecutionVersion int                                  `json:"-"`
}

type SaaSTenantDomainApprovalSnapshot struct {
	ID         int64  `json:"id"`
	TenantID   int    `json:"tenantId"`
	Hostname   string `json:"hostname"`
	Status     string `json:"status"`
	IsPrimary  bool   `json:"isPrimary"`
	VerifiedAt string `json:"verifiedAt"`
	Version    int    `json:"version"`
}

type SaaSTenantDomainCommandApprovalPlan struct {
	SchemaVersion  int                                `json:"schemaVersion"`
	Command        SaaSAdminTenantDomainCommand       `json:"command"`
	Domain         SaaSTenantDomainApprovalSnapshot   `json:"domain"`
	RoutingDomains []SaaSTenantDomainApprovalSnapshot `json:"routingDomains"`
	RoutingSHA256  string                             `json:"routingSha256"`
}

type SaaSAdminTenantDomainResult struct {
	Domain      SaaSTenantDomain
	OperationID int64
}

type SaaSAdminTenantDomainRequest struct {
	Action          string `json:"action"`
	ID              int64  `json:"id"`
	TenantID        int    `json:"tenantId"`
	Hostname        string `json:"hostname"`
	ExpectedVersion int    `json:"expectedVersion"`
}

type SaaSTenantDomainReader interface {
	SaaSTenantDomainByHostname(context.Context, string) (SaaSTenantDomain, bool, error)
}

type SaaSAdminTenantDomainStore interface {
	SaaSTenantDomainReader
	SaaSAdminTenantDomains(context.Context, SaaSAdminTenantDomainOptions) ([]SaaSTenantDomain, error)
	SaaSAdminTenantDomain(context.Context, int64) (SaaSTenantDomain, error)
	CreateSaaSAdminTenantDomain(context.Context, SaaSAdminTenantDomainCreate) (SaaSAdminTenantDomainResult, error)
	CompleteSaaSAdminTenantDomainVerification(context.Context, SaaSAdminTenantDomainVerification) (SaaSAdminTenantDomainResult, error)
	ApplySaaSAdminTenantDomainCommand(context.Context, SaaSAdminTenantDomainCommand) (SaaSAdminTenantDomainResult, error)
}

type SaaSAdminTenantDomainApprovalStore interface {
	PlanSaaSAdminTenantDomainCommand(context.Context, SaaSAdminTenantDomainRequest) (SaaSTenantDomainCommandApprovalPlan, error)
}

type SaaSAdminTenantDomainCreateApprovalStore interface {
	SaaSAdminTenantDomainCreateTarget(context.Context, int, string) (SaaSAdminTenantDomainCreateTarget, error)
}

type SaaSTenantDomainOwnershipVerifier interface {
	Verify(context.Context, SaaSTenantDomain) error
}

type SaaSTenantDomainTXTLookup interface {
	LookupTXT(context.Context, string) ([]string, error)
}

type SaaSTenantDomainDNSVerifier struct {
	lookup  SaaSTenantDomainTXTLookup
	timeout time.Duration
}

func NewSaaSTenantDomainDNSVerifier(server string, timeout time.Duration) (*SaaSTenantDomainDNSVerifier, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if timeout > 30*time.Second {
		return nil, errors.New("tenant domain DNS timeout must not exceed 30 seconds")
	}
	server = strings.TrimSpace(server)
	var lookup SaaSTenantDomainTXTLookup = net.DefaultResolver
	if server != "" {
		if _, _, err := net.SplitHostPort(server); err != nil {
			return nil, fmt.Errorf("tenant domain DNS server must use host:port: %w", err)
		}
		dialer := &net.Dialer{Timeout: timeout}
		lookup = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, server)
		}}
	}
	return &SaaSTenantDomainDNSVerifier{lookup: lookup, timeout: timeout}, nil
}

func NewSaaSTenantDomainDNSVerifierWithLookup(lookup SaaSTenantDomainTXTLookup, timeout time.Duration) *SaaSTenantDomainDNSVerifier {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &SaaSTenantDomainDNSVerifier{lookup: lookup, timeout: timeout}
}

func (verifier *SaaSTenantDomainDNSVerifier) Verify(ctx context.Context, domain SaaSTenantDomain) error {
	if verifier == nil || verifier.lookup == nil {
		return errors.New("DNS TXT 校验器未配置")
	}
	hostname, err := NormalizeSaaSTenantDomainHostname(domain.Hostname)
	if err != nil || strings.TrimSpace(domain.VerificationToken) == "" {
		return errors.New("域名或校验令牌无效")
	}
	lookupCtx, cancel := context.WithTimeout(ctx, verifier.timeout)
	defer cancel()
	values, err := verifier.lookup.LookupTXT(lookupCtx, SaaSTenantDomainDNSPrefix+hostname)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(lookupCtx.Err(), context.DeadlineExceeded) {
			return errors.New("DNS TXT 查询超时")
		}
		return errors.New("DNS TXT 记录暂不可用")
	}
	expected := SaaSTenantDomainTXTValuePrefix + domain.VerificationToken
	for _, value := range values {
		if strings.TrimSpace(value) == expected {
			return nil
		}
	}
	return errors.New("未找到匹配的 DNS TXT 校验记录")
}

func NormalizeSaaSTenantDomainHostname(value string) (string, error) {
	value = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
	if value == "" || len(value) > 253 || strings.Contains(value, "://") || strings.ContainsAny(value, `/\\@:_ `) {
		return "", errors.New("hostname 必须是有效的 ASCII 域名且不能包含协议、端口或路径")
	}
	if net.ParseIP(value) != nil || value == "localhost" || strings.HasSuffix(value, ".localhost") {
		return "", errors.New("hostname 不能是 IP 或 localhost")
	}
	labels := strings.Split(value, ".")
	if len(labels) < 2 {
		return "", errors.New("hostname 至少包含两级域名")
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || !domainLabelBoundary(label[0]) || !domainLabelBoundary(label[len(label)-1]) {
			return "", errors.New("hostname 标签格式无效")
		}
		for index := 0; index < len(label); index++ {
			character := label[index]
			if !domainLabelBoundary(character) && character != '-' {
				return "", errors.New("hostname 只能包含小写字母、数字、点和连字符")
			}
		}
	}
	return value, nil
}

func domainLabelBoundary(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}

func SaaSTenantDomainRequestHostname(rawHost string) (string, bool) {
	rawHost = strings.TrimSpace(rawHost)
	if rawHost == "" {
		return "", false
	}
	host := rawHost
	if parsed, _, err := net.SplitHostPort(rawHost); err == nil {
		host = parsed
	}
	host = strings.Trim(host, "[]")
	if net.ParseIP(host) != nil || strings.EqualFold(host, "localhost") {
		return "", false
	}
	normalized, err := NormalizeSaaSTenantDomainHostname(host)
	return normalized, err == nil
}

func ResolveSaaSTenantDomainRequest(ctx context.Context, reader SaaSTenantDomainReader, rawHost string) (SaaSTenantDomain, bool, error) {
	if reader == nil {
		return SaaSTenantDomain{}, false, nil
	}
	hostname, ok := SaaSTenantDomainRequestHostname(rawHost)
	if !ok {
		return SaaSTenantDomain{}, false, nil
	}
	return reader.SaaSTenantDomainByHostname(ctx, hostname)
}

func (h *SaaSAdminHandler) WithTenantDomainVerifier(verifier SaaSTenantDomainOwnershipVerifier) *SaaSAdminHandler {
	h.tenantDomainVerifier = verifier
	return h
}

func (h *SaaSAdminHandler) TenantDomains(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasAdminTenantDomainStore(w)
	if !ok {
		return
	}
	options, err := saasAdminTenantDomainOptions(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	items, err := store.SaaSAdminTenantDomains(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"domains": saasTenantDomainPayloads(items),
		"verification": map[string]any{
			"method": SaaSTenantDomainVerificationDNS, "recordPrefix": SaaSTenantDomainDNSPrefix,
			"valuePrefix": SaaSTenantDomainTXTValuePrefix, "maxDomainsPerTenant": SaaSTenantDomainMaxPerTenant,
		},
	})
}

func (h *SaaSAdminHandler) TenantDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasAdminTenantDomainStore(w)
	if !ok {
		return
	}
	var request SaaSAdminTenantDomainRequest
	if err := decodeSaaSAdminAccessJSON(r, &request); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := normalizeAndValidateSaaSAdminTenantDomainRequest(&request); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	actorUserID, actorTenantID := user.ID, user.TenantID
	if request.Action == SaaSTenantDomainActionCreate {
		if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionTenantDomainCreate, 0) {
			return
		}
		token, err := generateSaaSTenantDomainToken()
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "生成域名校验令牌失败", nil)
			return
		}
		result, err := store.CreateSaaSAdminTenantDomain(r.Context(), SaaSAdminTenantDomainCreate{
			TenantID: request.TenantID, Hostname: request.Hostname, Token: token,
			ActorUserID: actorUserID, ActorTenantID: actorTenantID,
		})
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeEnvelope(w, http.StatusCreated, http.StatusCreated, "success", saasTenantDomainResultPayload(result))
		return
	}
	if request.Action == SaaSTenantDomainActionVerify {
		if h.tenantDomainVerifier == nil {
			writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "DNS TXT 校验器未配置", nil)
			return
		}
		domain, err := store.SaaSAdminTenantDomain(r.Context(), request.ID)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		if domain.Version != request.ExpectedVersion {
			writeEnvelope(w, http.StatusConflict, http.StatusConflict, "域名版本已变化，请刷新后重试", nil)
			return
		}
		verifyErr := h.tenantDomainVerifier.Verify(r.Context(), domain)
		result, err := store.CompleteSaaSAdminTenantDomainVerification(r.Context(), SaaSAdminTenantDomainVerification{
			ID: request.ID, ExpectedVersion: request.ExpectedVersion, Success: verifyErr == nil,
			ErrorMessage: tenantDomainVerificationError(verifyErr), ActorUserID: actorUserID, ActorTenantID: actorTenantID,
		})
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		if verifyErr != nil {
			writeEnvelope(w, http.StatusUnprocessableEntity, http.StatusUnprocessableEntity, verifyErr.Error(), saasTenantDomainResultPayload(result))
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasTenantDomainResultPayload(result))
		return
	}
	if SaaSTenantDomainCommandRequiresApproval(request.Action) && h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionTenantDomainCommand, 0) {
		return
	}
	command := SaaSAdminTenantDomainCommand{
		ID: request.ID, Action: request.Action, ExpectedVersion: request.ExpectedVersion,
		ActorUserID: actorUserID, ActorTenantID: actorTenantID,
	}
	if request.Action == SaaSTenantDomainActionRotateToken {
		token, err := generateSaaSTenantDomainToken()
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "生成域名校验令牌失败", nil)
			return
		}
		command.Token = token
	}
	result, err := store.ApplySaaSAdminTenantDomainCommand(r.Context(), command)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasTenantDomainResultPayload(result))
}

func SaaSTenantDomainCommandRequiresApproval(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case SaaSTenantDomainActionSetPrimary, SaaSTenantDomainActionEnable, SaaSTenantDomainActionDisable,
		SaaSTenantDomainActionRotateToken, SaaSTenantDomainActionDelete:
		return true
	default:
		return false
	}
}

func (h *SaaSAdminHandler) saasAdminTenantDomainStore(w http.ResponseWriter) (SaaSAdminTenantDomainStore, bool) {
	store, ok := h.store.(SaaSAdminTenantDomainStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "SaaS tenant domain store is not configured", nil)
		return nil, false
	}
	return store, true
}

func saasAdminTenantDomainOptions(r *http.Request) (SaaSAdminTenantDomainOptions, error) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" {
		status = "all"
	}
	if status != "all" && status != SaaSTenantDomainStatusPending && status != SaaSTenantDomainStatusActive && status != SaaSTenantDomainStatusDisabled {
		return SaaSAdminTenantDomainOptions{}, errors.New("status 必须是 all/pending/active/disabled")
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len([]rune(keyword)) > 80 {
		return SaaSAdminTenantDomainOptions{}, errors.New("keyword 最多 80 个字符")
	}
	limit := positiveQueryInt(r, "limit", 100)
	if limit > 100 {
		limit = 100
	}
	return SaaSAdminTenantDomainOptions{TenantID: positiveQueryInt(r, "tenantId", 0), Status: status, Keyword: keyword, Limit: limit}, nil
}

func normalizeAndValidateSaaSAdminTenantDomainRequest(request *SaaSAdminTenantDomainRequest) error {
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))
	if request.Action == SaaSTenantDomainActionCreate {
		hostname, err := NormalizeSaaSTenantDomainHostname(request.Hostname)
		if err != nil {
			return err
		}
		request.Hostname = hostname
		if request.TenantID <= 0 || request.ID != 0 || request.ExpectedVersion != 0 {
			return errors.New("创建域名时 tenantId 必填，id 和 expectedVersion 必须为 0")
		}
		return nil
	}
	valid := request.Action == SaaSTenantDomainActionVerify || request.Action == SaaSTenantDomainActionSetPrimary ||
		request.Action == SaaSTenantDomainActionEnable || request.Action == SaaSTenantDomainActionDisable ||
		request.Action == SaaSTenantDomainActionRotateToken || request.Action == SaaSTenantDomainActionDelete
	if !valid {
		return errors.New("action 必须是 create/verify/set_primary/enable/disable/rotate_token/delete")
	}
	if request.ID <= 0 || request.ExpectedVersion <= 0 || request.TenantID != 0 || strings.TrimSpace(request.Hostname) != "" {
		return errors.New("域名操作需要 id 和 expectedVersion，不能同时提交 tenantId 或 hostname")
	}
	return nil
}

func generateSaaSTenantDomainToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func tenantDomainVerificationError(err error) string {
	if err == nil {
		return ""
	}
	value := strings.TrimSpace(err.Error())
	runes := []rune(value)
	if len(runes) > 500 {
		value = string(runes[:500])
	}
	return value
}

func saasTenantDomainPayload(domain SaaSTenantDomain) map[string]any {
	return map[string]any{
		"id": domain.ID, "tenantId": domain.TenantID, "tenantName": domain.TenantName, "hostname": domain.Hostname,
		"status": domain.Status, "isPrimary": domain.IsPrimary, "routingActive": domain.RoutingActive(),
		"verificationMethod": domain.VerificationMethod, "verificationToken": domain.VerificationToken,
		"verificationRecordName": domain.VerificationRecordName(), "verificationRecordValue": domain.VerificationRecordValue(),
		"verificationError": domain.VerificationError, "lastVerificationAt": domain.LastVerificationAt,
		"verifiedAt": domain.VerifiedAt, "version": domain.Version, "createdBy": domain.CreatedBy,
		"updatedBy": domain.UpdatedBy, "createdAt": domain.CreatedAt, "updatedAt": domain.UpdatedAt,
		"delivery": domain.Delivery,
	}
}

func saasTenantDomainPayloads(domains []SaaSTenantDomain) []map[string]any {
	result := make([]map[string]any, 0, len(domains))
	for _, domain := range domains {
		result = append(result, saasTenantDomainPayload(domain))
	}
	return result
}

func saasTenantDomainResultPayload(result SaaSAdminTenantDomainResult) map[string]any {
	return map[string]any{"domain": saasTenantDomainPayload(result.Domain), "operationId": result.OperationID}
}
