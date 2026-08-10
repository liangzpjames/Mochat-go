package dashboard

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"jiyi/mochat-go/internal/authjwt"
	"jiyi/mochat-go/internal/identitysecurity"
)

type AuthUser struct {
	ID                              int
	Name                            string
	Phone                           string
	Status                          int
	Password                        string
	TenantID                        int
	TenantStatus                    int
	TenantPackageExpired            bool
	TenantPackageExpiresAt          string
	TenantSubscriptionManaged       bool
	TenantSubscriptionStatus        string
	TenantSubscriptionAccessAllowed bool
	TenantSubscriptionGraceEndsAt   string
}

type AuthStore interface {
	UserAuthByPhone(ctx context.Context, phone string) (AuthUser, bool, error)
}

type authTenantStore interface {
	UserAuthByTenantPhone(ctx context.Context, tenantID int, phone string) (AuthUser, bool, error)
}

type authIdentitySecurity interface {
	RecordUnknownPasswordFailure(context.Context, string, identitysecurity.RequestMeta) error
	ClientMeta(*http.Request) identitysecurity.RequestMeta
	CheckPasswordLogin(context.Context, identitysecurity.Principal, identitysecurity.RequestMeta) (identitysecurity.Policy, error)
	RecordPasswordFailure(context.Context, identitysecurity.Principal, string, identitysecurity.RequestMeta, identitysecurity.Policy) (identitysecurity.UserState, error)
	PasswordAccepted(context.Context, identitysecurity.Principal, string, identitysecurity.RequestMeta, identitysecurity.Policy) (identitysecurity.AuthDecision, error)
	SessionTTL(identitysecurity.Policy, time.Duration) time.Duration
	RegisterSession(context.Context, identitysecurity.Principal, string, map[string]any, string, identitysecurity.RequestMeta, identitysecurity.Policy) (identitysecurity.Session, error)
	CompleteMFA(context.Context, string, string, identitysecurity.RequestMeta) (identitysecurity.MFACompletion, error)
	CompleteMFAForTenant(context.Context, string, string, identitysecurity.RequestMeta, int) (identitysecurity.MFACompletion, error)
}

type AuthHandler struct {
	store      AuthStore
	secret     string
	ttl        time.Duration
	now        func() time.Time
	security   authIdentitySecurity
	domains    SaaSTenantDomainReader
	tenantGate DashboardTenantAccessStore
}

type authRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

func NewAuthHandler(store AuthStore, secret string, ttl time.Duration) *AuthHandler {
	return &AuthHandler{store: store, secret: secret, ttl: ttl}
}

func (h *AuthHandler) WithIdentitySecurity(manager *identitysecurity.Manager) *AuthHandler {
	h.security = manager
	return h
}

func (h *AuthHandler) WithTenantDomains(reader SaaSTenantDomainReader) *AuthHandler {
	h.domains = reader
	return h
}

func (h *AuthHandler) WithDashboardTenantGate(gate DashboardTenantAccessStore) *AuthHandler {
	h.tenantGate = gate
	return h
}

func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}

	params, err := parseAuthRequest(r)
	if err != nil || !validPhone(params.Phone) || params.Password == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "手机号或密码错误", nil)
		return
	}

	domain, domainFound, err := ResolveSaaSTenantDomainRequest(r.Context(), h.domains, r.Host)
	if err != nil {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "租户域名服务暂不可用", nil)
		return
	}
	if domainFound && !domain.RoutingActive() {
		writeEnvelope(w, http.StatusMisdirectedRequest, http.StatusMisdirectedRequest, "租户域名尚未启用", nil)
		return
	}
	var user AuthUser
	var ok bool
	if domainFound {
		store, supported := h.store.(authTenantStore)
		if !supported {
			writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "租户域名认证暂不可用", nil)
			return
		}
		user, ok, err = store.UserAuthByTenantPhone(r.Context(), domain.TenantID, params.Phone)
	} else {
		user, ok, err = h.store.UserAuthByPhone(r.Context(), params.Phone)
	}
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !ok {
		if h.security != nil {
			_ = h.security.RecordUnknownPasswordFailure(r.Context(), params.Phone, h.security.ClientMeta(r))
		}
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "手机号或密码错误", nil)
		return
	}
	principal := identitysecurity.Principal{UserID: user.ID, TenantID: user.TenantID, Phone: params.Phone, Name: user.Name}
	var policy identitysecurity.Policy
	if h.security != nil {
		policy, err = h.security.CheckPasswordLogin(r.Context(), principal, h.security.ClientMeta(r))
		if err != nil {
			writeIdentityAuthError(w, err)
			return
		}
	}
	if user.Status != 1 {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "账户无法登录", nil)
		return
	}
	if h.tenantGate == nil {
		if user.TenantStatus == 2 {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "租户已停用", nil)
			return
		}
		if user.TenantSubscriptionManaged && !user.TenantSubscriptionAccessAllowed {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, SaaSAdminSubscriptionAccessReason(user.TenantSubscriptionStatus), nil)
			return
		}
		if user.TenantPackageExpired {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "租户套餐已到期", nil)
			return
		}
	}
	if !authjwt.CheckPasswordHash(h.secret, params.Password, user.Password) {
		if h.security != nil {
			_, _ = h.security.RecordPasswordFailure(r.Context(), principal, params.Phone, h.security.ClientMeta(r), policy)
		}
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "手机号或密码错误", nil)
		return
	}
	if h.tenantGate != nil {
		access, err := h.dashboardTenantAccess(r.Context(), user.TenantID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "tenant access service unavailable", nil)
			return
		}
		if !access.Allowed {
			writeMachineEnvelope(w, http.StatusForbidden, DashboardTenantAccessDeniedCode, "tenant access denied", nil)
			return
		}
	}
	method := identitysecurity.AuthMethodPassword
	if h.security != nil {
		decision, err := h.security.PasswordAccepted(r.Context(), principal, params.Phone, h.security.ClientMeta(r), policy)
		if err != nil {
			writeIdentityAuthError(w, err)
			return
		}
		if decision.MFARequired {
			writeEnvelope(w, http.StatusAccepted, http.StatusAccepted, "mfa required", map[string]any{
				"mfaRequired": true, "challengeToken": decision.ChallengeToken,
				"expiresIn": int64(time.Until(decision.ChallengeExpiry).Seconds()), "methods": []string{"totp", "recovery"},
			})
			return
		}
		policy = decision.Policy
		method = decision.AuthMethod
	}
	h.issueToken(w, r, user, principal, policy, method)
}

func (h *AuthHandler) issueToken(w http.ResponseWriter, r *http.Request, user AuthUser, principal identitysecurity.Principal, policy identitysecurity.Policy, method string) {

	now := time.Now()
	if h.now != nil {
		now = h.now()
	}
	ttl := h.ttl
	if h.security != nil {
		ttl = h.security.SessionTTL(policy, ttl)
	}
	token, payload, err := authjwt.MakeToken(authjwt.TokenOptions{
		Secret: h.secret,
		TTL:    ttl,
		Now:    now,
		UID:    user.ID,
		Issuer: requestIssuer(r),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	var session any
	if h.security != nil {
		created, err := h.security.RegisterSession(r.Context(), principal, token, payload, method, h.security.ClientMeta(r), policy)
		if err != nil {
			writeIdentityAuthError(w, err)
			return
		}
		session = created
	}

	expire := int64(ttl / time.Second)
	if expire <= 0 {
		expire = int64((7 * 24 * time.Hour) / time.Second)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"token": token, "expire": expire, "mfaRequired": false, "authMethod": method, "session": session,
	})
}

type authMFARequest struct {
	ChallengeToken string `json:"challengeToken"`
	Code           string `json:"code"`
}

type authUserByIDStore interface {
	UserAuthByID(ctx context.Context, userID int) (AuthUser, bool, error)
}

func (h *AuthHandler) MFA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if h.security == nil {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "identity security is disabled", nil)
		return
	}
	var request authMFARequest
	if err := decodeAuthJSONBody(r, &request); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "challengeToken 和 code 必填", nil)
		return
	}
	domain, domainFound, err := ResolveSaaSTenantDomainRequest(r.Context(), h.domains, r.Host)
	if err != nil {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "租户域名服务暂不可用", nil)
		return
	}
	if domainFound && !domain.RoutingActive() {
		writeEnvelope(w, http.StatusMisdirectedRequest, http.StatusMisdirectedRequest, "租户域名尚未启用", nil)
		return
	}
	var completion identitysecurity.MFACompletion
	if domainFound {
		completion, err = h.security.CompleteMFAForTenant(r.Context(), request.ChallengeToken, request.Code, h.security.ClientMeta(r), domain.TenantID)
	} else {
		completion, err = h.security.CompleteMFA(r.Context(), request.ChallengeToken, request.Code, h.security.ClientMeta(r))
	}
	if err != nil {
		writeIdentityAuthError(w, err)
		return
	}
	store, ok := h.store.(authUserByIDStore)
	if !ok {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "identity user lookup is unavailable", nil)
		return
	}
	user, found, err := store.UserAuthByID(r.Context(), completion.Principal.UserID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found || user.Status != 1 {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "账户无法登录", nil)
		return
	}
	if h.tenantGate == nil {
		if user.TenantStatus == 2 || user.TenantPackageExpired || (user.TenantSubscriptionManaged && !user.TenantSubscriptionAccessAllowed) {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "账户无法登录", nil)
			return
		}
	} else {
		access, err := h.dashboardTenantAccess(r.Context(), user.TenantID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "tenant access service unavailable", nil)
			return
		}
		if !access.Allowed {
			writeMachineEnvelope(w, http.StatusForbidden, DashboardTenantAccessDeniedCode, "tenant access denied", nil)
			return
		}
	}
	principal := completion.Principal
	principal.Name, principal.Phone = user.Name, user.Phone
	h.issueToken(w, r, user, principal, completion.Policy, completion.AuthMethod)
}

func (h *AuthHandler) dashboardTenantAccess(ctx context.Context, tenantID int) (DashboardTenantAccess, error) {
	now := time.Now()
	if h.now != nil {
		now = h.now()
	}
	return h.tenantGate.DashboardTenantAccess(ctx, tenantID, now)
}

func writeIdentityAuthError(w http.ResponseWriter, err error) {
	status := identitysecurity.StatusCode(err)
	message := err.Error()
	if status >= http.StatusInternalServerError {
		message = "身份安全服务暂不可用"
	}
	writeEnvelope(w, status, status, message, nil)
}

func decodeAuthJSONBody(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func parseAuthRequest(r *http.Request) (authRequest, error) {
	var params authRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return params, err
		}
		params.Phone = r.FormValue("phone")
		params.Password = r.FormValue("password")
		return params, nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return params, err
	}
	if err := json.Unmarshal(body, &params); err == nil {
		return params, nil
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		return params, err
	}
	params.Phone = values.Get("phone")
	params.Password = values.Get("password")
	return params, nil
}

var phonePattern = regexp.MustCompile(`^[0-9]+$`)

func validPhone(phone string) bool {
	return phone != "" && phonePattern.MatchString(phone)
}

func requestIssuer(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = ":"
	}
	return scheme + "://" + host + r.URL.RequestURI()
}
