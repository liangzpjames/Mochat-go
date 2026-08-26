package dashboardauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/authrealm"
	"jiyi/mochat-go/internal/dashboardprincipal"
)

const (
	CodeInvalidRequest            = "INVALID_REQUEST"
	CodeInvalidCredentials        = "INVALID_CREDENTIALS"
	CodeAuthUnavailable           = "AUTH_UNAVAILABLE"
	CodeMFAEnrollmentRequired     = "MFA_ENROLLMENT_REQUIRED"
	CodeMFARequired               = "MFA_REQUIRED"
	CodeMFAChallengeInvalid       = "MFA_CHALLENGE_INVALID"
	CodePasswordChangeRequired    = "PASSWORD_CHANGE_REQUIRED"
	CodePasswordChangeInvalid     = "PASSWORD_CHANGE_INVALID"
	CodeActivationInvalid         = "ACTIVATION_INVALID"
	CodeActivationStatus          = "ACTIVATION_STATUS"
	CodeTenantAccessDenied        = "TENANT_ACCESS_DENIED"
	CodeCorpConfigurationRequired = "CORP_CONFIGURATION_REQUIRED"
)

var (
	ErrInvalidHTTPConfig = errors.New("invalid Dashboard authentication HTTP configuration")
	ErrInvalidRequest    = errors.New("invalid Dashboard authentication request")
)

type TenantAccess struct {
	TenantID int
	Allowed  bool
	Reason   string
}

type DashboardTenantGate func(context.Context, int, time.Time) (TenantAccess, error)

type HTTPConfig struct {
	Service           *Service
	Persistence       DashboardAuthPersistence
	PrincipalResolver dashboardprincipal.PrincipalResolver
	Signer            authrealm.TokenConfig
	Parser            authrealm.Parser
	MFAKey            []byte
	MFAKeyID          string
	MFARequired       bool
	TenantGate        DashboardTenantGate
	Now               func() time.Time
}

type HTTPHandler struct {
	service           *Service
	persistence       DashboardAuthPersistence
	principalResolver dashboardprincipal.PrincipalResolver
	signer            authrealm.TokenConfig
	parser            authrealm.Parser
	protector         *DashboardMFAProtector
	mfaRequired       bool
	tenantGate        DashboardTenantGate
	now               func() time.Time
}

func NewHTTPHandler(config HTTPConfig) (*HTTPHandler, error) {
	if config.Service == nil || config.Persistence == nil || config.PrincipalResolver == nil || config.TenantGate == nil {
		return nil, ErrInvalidHTTPConfig
	}
	if err := validateDashboardTokenConfig(config.Signer); err != nil {
		return nil, err
	}
	if err := validateDashboardTokenConfig(config.Parser.Config); err != nil {
		return nil, err
	}
	if config.Parser.ValidateSession == nil {
		return nil, fmt.Errorf("%w: session validator is required", ErrInvalidHTTPConfig)
	}
	if !sameDashboardTokenConfig(config.Signer, config.Parser.Config) {
		return nil, fmt.Errorf("%w: signer and parser must use the same Dashboard realm", ErrInvalidHTTPConfig)
	}
	protector, err := NewDashboardMFAProtector(config.MFAKey, config.MFAKeyID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidHTTPConfig, err)
	}
	return &HTTPHandler{
		service:           config.Service,
		persistence:       config.Persistence,
		principalResolver: config.PrincipalResolver,
		signer:            config.Signer,
		parser:            config.Parser,
		protector:         protector,
		mfaRequired:       config.MFARequired,
		tenantGate:        config.TenantGate,
		now:               config.Now,
	}, nil
}

func (handler *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler == nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	switch {
	case r.URL.Path == "/dashboard/user/auth" && r.Method == http.MethodPost:
		handler.login(w, r)
	case r.URL.Path == "/dashboard/user/authMFA" && r.Method == http.MethodPost:
		handler.mfaComplete(w, r)
	case r.URL.Path == "/dashboard/auth/activate" && r.Method == http.MethodPost:
		handler.activate(w, r)
	case r.URL.Path == "/dashboard/auth/activation/status" && r.Method == http.MethodPost:
		handler.activationStatus(w, r)
	case r.URL.Path == "/dashboard/auth/password/reset-request" && r.Method == http.MethodPost:
		handler.resetRequest(w, r)
	case r.URL.Path == "/dashboard/auth/password/reset" && r.Method == http.MethodPost:
		handler.reset(w, r)
	case r.URL.Path == "/dashboard/auth/session" && r.Method == http.MethodGet:
		handler.session(w, r)
	case (r.URL.Path == "/dashboard/auth/logout" && r.Method == http.MethodPost) || (r.URL.Path == "/dashboard/user/logout" && r.Method == http.MethodPut):
		handler.logout(w, r)
	default:
		http.NotFound(w, r)
	}
}

type dashboardLoginRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type dashboardMFARequest struct {
	ChallengeToken      string `json:"challengeToken"`
	Code                string `json:"code"`
	PasswordChangeToken string `json:"passwordChangeToken"`
	NewPassword         string `json:"newPassword"`
}

type dashboardActivationRequest struct {
	ActivationToken string `json:"activationToken"`
	Password        string `json:"password"`
}

type dashboardActivationStatusRequest struct {
	ActivationToken string `json:"activationToken"`
}

func (handler *HTTPHandler) activationStatus(w http.ResponseWriter, r *http.Request) {
	var request dashboardActivationStatusRequest
	if err := decodeDashboardAuthJSON(r, &request); err != nil || strings.TrimSpace(request.ActivationToken) == "" {
		writeDashboardAuthEnvelope(w, http.StatusBadRequest, CodeInvalidRequest, "invalid activation status request", nil)
		return
	}
	store, ok := any(handler.persistence).(DashboardActivationStatusStore)
	if !ok {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	status, err := store.DashboardActivationStatus(r.Context(), sha256.Sum256([]byte(strings.TrimSpace(request.ActivationToken))), handler.currentTime())
	if err != nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	writeDashboardAuthEnvelope(w, http.StatusOK, CodeActivationStatus, "success", status)
}

type dashboardPasswordResetRequest struct {
	ResetToken  string `json:"resetToken"`
	NewPassword string `json:"newPassword"`
}

func (handler *HTTPHandler) login(w http.ResponseWriter, r *http.Request) {
	var request dashboardLoginRequest
	if err := decodeDashboardAuthJSON(r, &request); err != nil {
		writeDashboardAuthEnvelope(w, http.StatusBadRequest, CodeInvalidRequest, "invalid authentication request", nil)
		return
	}
	if strings.TrimSpace(request.Phone) == "" || request.Password == "" {
		writeDashboardAuthEnvelope(w, http.StatusUnauthorized, CodeInvalidCredentials, "invalid credentials", nil)
		return
	}
	identity, err := handler.service.Authenticate(r.Context(), request.Phone, request.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			writeDashboardAuthEnvelope(w, http.StatusUnauthorized, CodeInvalidCredentials, "invalid credentials", nil)
			return
		}
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	if !handler.authorizeTenant(w, r.Context(), identity.UserID) {
		return
	}
	if handler.mfaRequired && identity.MFARequired != 0 {
		status, err := handler.persistence.MFAStatus(r.Context(), identity.UserID)
		if err != nil {
			writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
			return
		}
		if status != DashboardMFAStatusActive {
			handler.beginEnrollment(w, r.Context(), identity)
			return
		}
		handler.beginLoginChallenge(w, r.Context(), identity)
		return
	}
	if identity.MustRotatePassword != 0 {
		handler.beginPasswordChange(w, r.Context(), identity)
		return
	}
	handler.issueToken(w, r.Context(), identity)
}

func (handler *HTTPHandler) authorizeTenant(w http.ResponseWriter, ctx context.Context, userID int) bool {
	if handler.principalResolver == nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return false
	}
	principal, err := handler.principalResolver.ResolveUser(ctx, userID, handler.currentTime())
	if err != nil {
		if errors.Is(err, dashboardprincipal.ErrTenantAccessDenied) {
			writeDashboardAuthEnvelope(w, http.StatusForbidden, CodeTenantAccessDenied, "tenant access denied", nil)
		} else {
			writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		}
		return false
	}
	if principal.UserID != userID || principal.AuthVersion == 0 {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return false
	}
	if principal.CorpStatus == dashboardprincipal.CorpBindingStatusSuspended {
		writeDashboardAuthEnvelope(w, http.StatusForbidden, CodeTenantAccessDenied, "tenant access denied", nil)
		return false
	}
	return true
}

func (handler *HTTPHandler) beginEnrollment(w http.ResponseWriter, ctx context.Context, identity DashboardIdentity) {
	token, err := newDashboardOpaqueToken()
	if err != nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	enrollment, ciphertext, err := handler.protector.GenerateEnrollment(identity, handler.currentTime())
	if err != nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	if err := handler.persistence.BeginMFAEnrollment(ctx, identity.UserID, identity.AuthVersion, sha256.Sum256([]byte(token)), enrollment.ExpiresAt, ciphertext, handler.protector.keyID); err != nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	writeDashboardAuthEnvelope(w, http.StatusAccepted, CodeMFAEnrollmentRequired, "multi-factor authentication enrollment required", map[string]any{
		"enrollmentToken":  token,
		"enrollmentSecret": enrollment.Secret,
		"otpAuthURL":       enrollment.OTPAuthURL,
		"expiresAt":        enrollment.ExpiresAt.Unix(),
	})
}

func (handler *HTTPHandler) beginLoginChallenge(w http.ResponseWriter, ctx context.Context, identity DashboardIdentity) {
	token, err := newDashboardOpaqueToken()
	if err != nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	expiresAt := handler.currentTime().Add(5 * time.Minute)
	if err := handler.persistence.CreateMFAChallenge(ctx, identity.UserID, identity.AuthVersion, DashboardMFAChallengeLogin, sha256.Sum256([]byte(token)), expiresAt); err != nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	writeDashboardAuthEnvelope(w, http.StatusAccepted, CodeMFARequired, "multi-factor authentication required", map[string]any{
		"challengeToken": token,
		"expiresAt":      expiresAt.Unix(),
	})
}

func (handler *HTTPHandler) mfaComplete(w http.ResponseWriter, r *http.Request) {
	var request dashboardMFARequest
	if err := decodeDashboardAuthJSON(r, &request); err != nil {
		writeDashboardAuthEnvelope(w, http.StatusBadRequest, CodeInvalidRequest, "invalid authentication request", nil)
		return
	}
	if request.PasswordChangeToken != "" || request.NewPassword != "" {
		if strings.TrimSpace(request.PasswordChangeToken) == "" || strings.TrimSpace(request.NewPassword) == "" || request.ChallengeToken != "" || request.Code != "" {
			writeDashboardAuthEnvelope(w, http.StatusUnauthorized, CodePasswordChangeInvalid, "password change invalid", nil)
			return
		}
		hash, err := HashPassword(request.NewPassword)
		if err != nil {
			writeDashboardAuthEnvelope(w, http.StatusBadRequest, CodePasswordChangeInvalid, "password change invalid", nil)
			return
		}
		identity, err := handler.persistence.CompletePasswordChange(r.Context(), sha256.Sum256([]byte(strings.TrimSpace(request.PasswordChangeToken))), hash)
		if err != nil {
			writeDashboardAuthEnvelope(w, http.StatusUnauthorized, CodePasswordChangeInvalid, "password change invalid", nil)
			return
		}
		if !handler.authorizeTenant(w, r.Context(), identity.UserID) {
			return
		}
		handler.issueToken(w, r.Context(), identity)
		return
	}
	if strings.TrimSpace(request.ChallengeToken) == "" || strings.TrimSpace(request.Code) == "" {
		writeDashboardAuthEnvelope(w, http.StatusUnauthorized, CodeMFAChallengeInvalid, "multi-factor authentication challenge invalid", nil)
		return
	}
	digest := sha256.Sum256([]byte(strings.TrimSpace(request.ChallengeToken)))
	challenge, err := handler.persistence.FindMFAChallenge(r.Context(), digest)
	if err != nil || challenge.Status != DashboardMFAStatusPending || !challenge.ExpiresAt.After(handler.currentTime()) || challenge.Attempts >= challenge.MaxAttempts || (challenge.ChallengeType != DashboardMFAChallengeEnrollment && challenge.ChallengeType != DashboardMFAChallengeLogin) {
		writeDashboardAuthEnvelope(w, http.StatusUnauthorized, CodeMFAChallengeInvalid, "multi-factor authentication challenge invalid", nil)
		return
	}
	totpStep, err := handler.protector.Verify(challenge.SecretCiphertext, challenge.EncryptionKeyID, challenge.UserID, request.Code, handler.currentTime())
	if err != nil {
		_ = handler.persistence.RecordMFAFailure(r.Context(), digest)
		writeDashboardAuthEnvelope(w, http.StatusUnauthorized, CodeMFAChallengeInvalid, "multi-factor authentication challenge invalid", nil)
		return
	}
	identity, err := handler.persistence.CompleteMFAChallenge(r.Context(), digest, challenge.UserID, challenge.AuthVersion, challenge.ChallengeType, totpStep)
	if err != nil {
		writeDashboardAuthEnvelope(w, http.StatusUnauthorized, CodeMFAChallengeInvalid, "multi-factor authentication challenge invalid", nil)
		return
	}
	if !handler.authorizeTenant(w, r.Context(), identity.UserID) {
		return
	}
	if identity.MustRotatePassword != 0 {
		handler.beginPasswordChange(w, r.Context(), identity)
		return
	}
	handler.issueToken(w, r.Context(), identity)
}

func (handler *HTTPHandler) beginPasswordChange(w http.ResponseWriter, ctx context.Context, identity DashboardIdentity) {
	token, err := newDashboardOpaqueToken()
	if err != nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	expiresAt := handler.currentTime().Add(10 * time.Minute)
	if err := handler.persistence.CreateMFAChallenge(ctx, identity.UserID, identity.AuthVersion, DashboardMFAChallengePasswordChange, sha256.Sum256([]byte(token)), expiresAt); err != nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	writeDashboardAuthEnvelope(w, http.StatusAccepted, CodePasswordChangeRequired, "password change required", map[string]any{
		"passwordChangeToken": token,
		"expiresAt":           expiresAt.Unix(),
	})
}

func (handler *HTTPHandler) activate(w http.ResponseWriter, r *http.Request) {
	var request dashboardActivationRequest
	if err := decodeDashboardAuthJSON(r, &request); err != nil {
		writeDashboardAuthEnvelope(w, http.StatusBadRequest, CodeInvalidRequest, "invalid activation request", nil)
		return
	}
	if strings.TrimSpace(request.ActivationToken) == "" || strings.TrimSpace(request.Password) == "" {
		writeDashboardAuthEnvelope(w, http.StatusUnauthorized, CodeActivationInvalid, "activation invalid", nil)
		return
	}
	if err := handler.service.Activate(r.Context(), sha256.Sum256([]byte(strings.TrimSpace(request.ActivationToken))), request.Password); err != nil {
		writeDashboardAuthEnvelope(w, http.StatusConflict, CodeActivationInvalid, "activation invalid", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *HTTPHandler) resetRequest(w http.ResponseWriter, r *http.Request) {
	claims, ok := handler.parseBearer(w, r)
	if !ok {
		return
	}
	if err := decodeEmptyDashboardJSON(r); err != nil {
		writeDashboardAuthEnvelope(w, http.StatusBadRequest, CodeInvalidRequest, "invalid password reset request", nil)
		return
	}
	token, err := newDashboardOpaqueToken()
	if err != nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	expiresAt := handler.currentTime().Add(10 * time.Minute)
	if err := handler.persistence.CreatePasswordReset(r.Context(), claims.UserID, claims.AuthVersion, sha256.Sum256([]byte(token)), expiresAt); err != nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	writeDashboardAuthEnvelope(w, http.StatusAccepted, "PASSWORD_RESET_CREATED", "password reset created", map[string]any{"resetToken": token, "expiresAt": expiresAt.Unix()})
}

func (handler *HTTPHandler) reset(w http.ResponseWriter, r *http.Request) {
	var request dashboardPasswordResetRequest
	if err := decodeDashboardAuthJSON(r, &request); err != nil {
		writeDashboardAuthEnvelope(w, http.StatusBadRequest, CodeInvalidRequest, "invalid password reset request", nil)
		return
	}
	if strings.TrimSpace(request.ResetToken) == "" || strings.TrimSpace(request.NewPassword) == "" {
		writeDashboardAuthEnvelope(w, http.StatusUnauthorized, CodePasswordChangeInvalid, "password reset invalid", nil)
		return
	}
	hash, err := HashPassword(request.NewPassword)
	if err != nil {
		writeDashboardAuthEnvelope(w, http.StatusBadRequest, CodePasswordChangeInvalid, "password reset invalid", nil)
		return
	}
	if _, err := handler.persistence.CompletePasswordReset(r.Context(), sha256.Sum256([]byte(strings.TrimSpace(request.ResetToken))), hash); err != nil {
		writeDashboardAuthEnvelope(w, http.StatusConflict, CodePasswordChangeInvalid, "password reset invalid", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *HTTPHandler) session(w http.ResponseWriter, r *http.Request) {
	claims, ok := handler.parseBearer(w, r)
	if !ok {
		return
	}
	writeDashboardAuthEnvelope(w, http.StatusOK, "OK", "success", map[string]any{"userId": claims.UserID, "realm": claims.Realm, "authVersion": claims.AuthVersion})
}

func (handler *HTTPHandler) logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := handler.parseBearer(w, r)
	if !ok {
		return
	}
	if err := handler.persistence.RevokeSession(r.Context(), claims); err != nil {
		writeDashboardAuthEnvelope(w, http.StatusUnauthorized, authrealm.CodeSessionInvalid, "session invalid", nil)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "MOCHAT_DASHBOARD_TOKEN", Value: "", Path: "/dashboard", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (handler *HTTPHandler) issueToken(w http.ResponseWriter, ctx context.Context, identity DashboardIdentity) {
	now := handler.currentTime()
	jti, err := newDashboardOpaqueToken()
	if err != nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	expiresAt := now.Add(handler.signer.TTL)
	token, err := authrealm.Sign(handler.signer, authrealm.Claims{UserID: identity.UserID, AuthVersion: identity.AuthVersion, JWTID: jti}, now)
	if err != nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	if err := handler.persistence.CreateSession(ctx, identity.UserID, identity.AuthVersion, sha256.Sum256([]byte(jti)), now, expiresAt); err != nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return
	}
	writeDashboardAuthEnvelope(w, http.StatusOK, "OK", "success", map[string]any{"token": token, "userId": identity.UserID, "expiresAt": expiresAt.Unix()})
}

func (handler *HTTPHandler) parseBearer(w http.ResponseWriter, r *http.Request) (authrealm.Claims, bool) {
	token, ok := dashboardBearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeDashboardAuthEnvelope(w, http.StatusUnauthorized, authrealm.CodeSessionInvalid, "session invalid", nil)
		return authrealm.Claims{}, false
	}
	claims, err := handler.parser.Parse(r.Context(), token)
	if err != nil {
		writeDashboardAuthEnvelope(w, http.StatusUnauthorized, dashboardAuthErrorCode(err), "session invalid", nil)
		return authrealm.Claims{}, false
	}
	return claims, true
}

func (handler *HTTPHandler) currentTime() time.Time {
	if handler == nil || handler.now == nil {
		return time.Now().UTC()
	}
	return handler.now().UTC()
}

type RequestGuard struct {
	parser               authrealm.Parser
	persistence          DashboardAuthPersistence
	principalResolver    dashboardprincipal.PrincipalResolver
	tenantGate           DashboardTenantGate
	publicRouteContracts map[string]struct{}
	next                 interface {
		Authorize(http.ResponseWriter, *http.Request) bool
	}
}

var dashboardIdentityPublicRouteContracts = []string{
	"POST /dashboard/user/auth",
	"POST /dashboard/user/authMFA",
	"POST /dashboard/auth/activate",
	"POST /dashboard/auth/activation/status",
	"POST /dashboard/auth/password/reset",
}

func NewRequestGuard(parser authrealm.Parser, persistence DashboardAuthPersistence, tenantGate DashboardTenantGate) (*RequestGuard, error) {
	if persistence == nil || tenantGate == nil {
		return nil, ErrInvalidHTTPConfig
	}
	if err := validateDashboardTokenConfig(parser.Config); err != nil {
		return nil, err
	}
	if parser.ValidateSession == nil {
		return nil, fmt.Errorf("%w: session validator is required", ErrInvalidHTTPConfig)
	}
	return &RequestGuard{
		parser:               parser,
		persistence:          persistence,
		tenantGate:           tenantGate,
		publicRouteContracts: publicRouteContractSet(dashboardIdentityPublicRouteContracts),
	}, nil
}

// WithPublicRouteContracts replaces the identity-public allowlist with exact
// method+path contracts supplied by the composition root. It intentionally
// does not interpret prefixes, page-RBAC exemptions, or route families.
func (guard *RequestGuard) WithPublicRouteContracts(contracts []string) *RequestGuard {
	if guard != nil {
		guard.publicRouteContracts = publicRouteContractSet(contracts)
	}
	return guard
}

func (guard *RequestGuard) WithNext(next interface {
	Authorize(http.ResponseWriter, *http.Request) bool
}) *RequestGuard {
	if guard != nil {
		guard.next = next
	}
	return guard
}

func (guard *RequestGuard) WithPrincipalResolver(resolver dashboardprincipal.PrincipalResolver) *RequestGuard {
	if guard != nil {
		guard.principalResolver = resolver
	}
	return guard
}

func (guard *RequestGuard) Authorize(w http.ResponseWriter, r *http.Request) bool {
	if guard == nil {
		return false
	}
	if guard.isPublicRoute(r) {
		return true
	}
	if guard.principalResolver == nil {
		writeDashboardAuthEnvelope(w, http.StatusServiceUnavailable, CodeAuthUnavailable, "authentication unavailable", nil)
		return false
	}
	token, ok := dashboardBearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeDashboardAuthEnvelope(w, http.StatusUnauthorized, authrealm.CodeSessionInvalid, "session invalid", nil)
		return false
	}
	claims, err := guard.parser.Parse(r.Context(), token)
	if err != nil {
		writeDashboardAuthEnvelope(w, http.StatusUnauthorized, dashboardAuthErrorCode(err), "session invalid", nil)
		return false
	}
	principal, err := guard.principalResolver.ResolveUser(r.Context(), claims.UserID, time.Now().UTC())
	if err != nil {
		if errors.Is(err, dashboardprincipal.ErrTenantAccessDenied) {
			writeDashboardAuthEnvelope(w, http.StatusForbidden, CodeTenantAccessDenied, "tenant access denied", nil)
		} else {
			writeDashboardAuthEnvelope(w, http.StatusUnauthorized, authrealm.CodeSessionInvalid, "session invalid", nil)
		}
		return false
	}
	if principal.UserID != claims.UserID || principal.AuthVersion != claims.AuthVersion {
		writeDashboardAuthEnvelope(w, http.StatusUnauthorized, authrealm.CodeSessionInvalid, "session invalid", nil)
		return false
	}
	if principal.CorpStatus == dashboardprincipal.CorpBindingStatusSuspended {
		writeDashboardAuthEnvelope(w, http.StatusForbidden, CodeTenantAccessDenied, "tenant access denied", nil)
		return false
	}
	request := r.WithContext(dashboardprincipal.WithPrincipal(r.Context(), principal))
	for _, header := range []string{"X-Mochat-Go-User-ID", "X-Mochat-Go-Tenant-ID", "X-Mochat-Go-Corp-ID", "X-Mochat-Go-Actor-ID", "X-User-ID", "X-Employee-ID", "X-Actor-ID", "X-Tenant-ID", "X-Corp-ID"} {
		request.Header.Del(header)
	}
	request.Header.Set("X-Mochat-Go-User-ID", strconv.Itoa(principal.UserID))
	*r = *request
	if guard.next != nil {
		return guard.next.Authorize(w, r)
	}
	return true
}

func (guard *RequestGuard) isPublicRoute(r *http.Request) bool {
	if guard == nil {
		return false
	}
	return dashboardRouteContractMatches(guard.publicRouteContracts, r)
}

func publicRouteContractSet(contracts []string) map[string]struct{} {
	set := make(map[string]struct{}, len(contracts))
	for _, contract := range contracts {
		method, path, ok := strings.Cut(strings.TrimSpace(contract), " ")
		if !ok || strings.TrimSpace(method) == "" || strings.TrimSpace(path) == "" {
			continue
		}
		set[strings.TrimSpace(method)+" "+strings.TrimSpace(path)] = struct{}{}
	}
	return set
}

func dashboardRouteContractMatches(contracts map[string]struct{}, r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}
	_, ok := contracts[r.Method+" "+r.URL.Path]
	return ok
}

func decodeDashboardAuthJSON(r *http.Request, target any) error {
	if r == nil || r.Body == nil {
		return ErrInvalidRequest
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrInvalidRequest
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrInvalidRequest
	}
	return nil
}

func decodeEmptyDashboardJSON(r *http.Request) error {
	if r == nil || r.Body == nil {
		return nil
	}
	return decodeDashboardAuthJSON(r, &struct{}{})
}

func dashboardBearerToken(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) < len("Bearer ") || !strings.EqualFold(value[:len("Bearer ")], "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(value[len("Bearer "):])
	return token, token != ""
}

func newDashboardOpaqueToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func validateDashboardTokenConfig(config authrealm.TokenConfig) error {
	if err := config.Validate(); err != nil {
		return err
	}
	if config.Realm != authrealm.RealmDashboard || strings.TrimSpace(config.Prefix) == "" {
		return fmt.Errorf("%w: Dashboard realm and session prefix are required", ErrInvalidHTTPConfig)
	}
	return nil
}

func sameDashboardTokenConfig(left, right authrealm.TokenConfig) bool {
	return string(left.Secret) == string(right.Secret) && left.Issuer == right.Issuer && left.Audience == right.Audience && left.TTL == right.TTL && left.Realm == right.Realm && left.Prefix == right.Prefix
}

func dashboardAuthErrorCode(err error) string {
	var authErr *authrealm.AuthError
	if errors.As(err, &authErr) {
		return authErr.Code()
	}
	return authrealm.CodeSessionInvalid
}

func writeDashboardAuthEnvelope(w http.ResponseWriter, status int, errorCode, message string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Code      int    `json:"code"`
		ErrorCode string `json:"errorCode,omitempty"`
		Msg       string `json:"msg"`
		Data      any    `json:"data"`
	}{Code: status, ErrorCode: errorCode, Msg: message, Data: data})
}
