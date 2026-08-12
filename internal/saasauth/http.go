package saasauth

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
	"strings"
	"time"

	"jiyi/mochat-go/internal/authrealm"
)

var (
	ErrInvalidHTTPConfig  = errors.New("invalid SaaS authentication HTTP configuration")
	ErrMFAInvalid         = errors.New("SaaS MFA verification failed")
	ErrInvalidSaaSRequest = errors.New("invalid SaaS authentication request")
)

type HTTPConfig struct {
	Service     *Service
	Persistence SaaSAuthPersistence
	Signer      authrealm.TokenConfig
	Parser      authrealm.Parser
	MFAKey      []byte
	MFAKeyID    string
	MFARequired bool
}

type HTTPHandler struct {
	service     *Service
	persistence SaaSAuthPersistence
	signer      authrealm.TokenConfig
	parser      authrealm.Parser
	protector   *MFAProtector
	mfaRequired bool
	now         func() time.Time
}

func NewHTTPHandler(config HTTPConfig) (*HTTPHandler, error) {
	if config.Service == nil || config.Persistence == nil {
		return nil, ErrInvalidHTTPConfig
	}
	if err := validateSaaSTokenConfig(config.Signer); err != nil {
		return nil, err
	}
	if err := validateSaaSTokenConfig(config.Parser.Config); err != nil {
		return nil, err
	}
	if config.Parser.ValidateSession == nil {
		return nil, fmt.Errorf("%w: session validator is required", ErrInvalidHTTPConfig)
	}
	if !sameTokenConfig(config.Signer, config.Parser.Config) {
		return nil, fmt.Errorf("%w: signer and parser must use the same SaaS realm", ErrInvalidHTTPConfig)
	}
	protector, err := NewMFAProtector(config.MFAKey, config.MFAKeyID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidHTTPConfig, err)
	}
	return &HTTPHandler{
		service:     config.Service,
		persistence: config.Persistence,
		signer:      config.Signer,
		parser:      config.Parser,
		protector:   protector,
		mfaRequired: config.MFARequired,
		now:         time.Now,
	}, nil
}

func (handler *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler == nil {
		writeSaaSAuthEnvelope(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication unavailable", nil)
		return
	}
	switch {
	case r.URL.Path == "/saas/auth/login" && r.Method == http.MethodPost:
		handler.login(w, r)
	case r.URL.Path == "/saas/auth/mfa" && r.Method == http.MethodPost:
		handler.mfaComplete(w, r)
	case r.URL.Path == "/saas/auth/password" && r.Method == http.MethodPost:
		handler.passwordChange(w, r)
	case r.URL.Path == "/saas/auth/session" && r.Method == http.MethodGet:
		handler.session(w, r)
	case r.URL.Path == "/saas/auth/logout" && r.Method == http.MethodPost:
		handler.logout(w, r)
	default:
		http.NotFound(w, r)
	}
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type mfaRequest struct {
	ChallengeToken string `json:"challengeToken"`
	Code           string `json:"code"`
}

type passwordChangeRequest struct {
	PasswordChangeToken string `json:"passwordChangeToken"`
	NewPassword         string `json:"newPassword"`
}

func (handler *HTTPHandler) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := decodeSaaSAuthJSON(r, &request); err != nil {
		writeSaaSAuthEnvelope(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid authentication request", nil)
		return
	}
	if strings.TrimSpace(request.Login) == "" || request.Password == "" {
		writeSaaSAuthEnvelope(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid credentials", nil)
		return
	}
	identity, err := handler.service.Authenticate(r.Context(), request.Login, request.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			writeSaaSAuthEnvelope(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid credentials", nil)
			return
		}
		writeSaaSAuthEnvelope(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication unavailable", nil)
		return
	}
	if !handler.mfaRequired || identity.MFARequired == 0 {
		if identity.MustRotatePassword != 0 {
			handler.beginPasswordChange(w, r.Context(), identity)
			return
		}
		handler.issueToken(w, r.Context(), identity)
		return
	}
	mfaStatus, err := handler.persistence.MFAStatus(r.Context(), identity.ID)
	if err != nil {
		writeSaaSAuthEnvelope(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication unavailable", nil)
		return
	}
	if mfaStatus != SaaSMFAStatusActive {
		handler.beginEnrollment(w, r.Context(), identity)
		return
	}
	handler.beginLoginChallenge(w, r.Context(), identity)
}

func (handler *HTTPHandler) beginEnrollment(w http.ResponseWriter, ctx context.Context, identity SaaSIdentity) {
	token, err := newOpaqueToken()
	if err != nil {
		writeSaaSAuthEnvelope(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication unavailable", nil)
		return
	}
	now := handler.currentTime()
	enrollment, ciphertext, err := handler.protector.GenerateEnrollment(identity, now)
	if err != nil || !enrollment.ExpiresAt.After(now) {
		writeSaaSAuthEnvelope(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication unavailable", nil)
		return
	}
	if err := handler.persistence.BeginMFAEnrollment(ctx, identity.ID, identity.AuthVersion, sha256.Sum256([]byte(token)), enrollment.ExpiresAt, ciphertext, handler.protector.keyID); err != nil {
		writeSaaSAuthEnvelope(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication unavailable", nil)
		return
	}
	writeSaaSAuthEnvelope(w, http.StatusAccepted, "MFA_ENROLLMENT_REQUIRED", "multi-factor authentication enrollment required", map[string]any{
		"enrollmentToken":       token,
		"enrollmentSecret":      enrollment.Secret,
		"otpAuthURL":            enrollment.OTPAuthURL,
		"expiresAt":             enrollment.ExpiresAt.Unix(),
		"mfaEnrollmentRequired": true,
		"mustRotatePassword":    identity.MustRotatePassword != 0,
	})
}

func (handler *HTTPHandler) beginLoginChallenge(w http.ResponseWriter, ctx context.Context, identity SaaSIdentity) {
	token, err := newOpaqueToken()
	if err != nil {
		writeSaaSAuthEnvelope(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication unavailable", nil)
		return
	}
	expiresAt := handler.currentTime().Add(5 * time.Minute)
	if err := handler.persistence.CreateMFAChallenge(ctx, identity.ID, identity.AuthVersion, SaaSMFAChallengeLogin, sha256.Sum256([]byte(token)), expiresAt); err != nil {
		writeSaaSAuthEnvelope(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication unavailable", nil)
		return
	}
	writeSaaSAuthEnvelope(w, http.StatusAccepted, "MFA_REQUIRED", "multi-factor authentication required", map[string]any{
		"challengeToken": token, "expiresAt": expiresAt.Unix(), "mfaRequired": true,
		"mustRotatePassword": identity.MustRotatePassword != 0,
	})
}

func (handler *HTTPHandler) mfaComplete(w http.ResponseWriter, r *http.Request) {
	var request mfaRequest
	if err := decodeSaaSAuthJSON(r, &request); err != nil {
		writeSaaSAuthEnvelope(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid authentication request", nil)
		return
	}
	if strings.TrimSpace(request.ChallengeToken) == "" || strings.TrimSpace(request.Code) == "" {
		writeSaaSAuthEnvelope(w, http.StatusUnauthorized, "MFA_CHALLENGE_INVALID", "multi-factor authentication challenge invalid", nil)
		return
	}
	digest := sha256.Sum256([]byte(strings.TrimSpace(request.ChallengeToken)))
	challenge, err := handler.persistence.FindMFAChallenge(r.Context(), digest)
	if err != nil || challenge.Status != 0 || challenge.ExpiresAt.Before(handler.currentTime()) || challenge.Attempts >= challenge.MaxAttempts || (challenge.ChallengeType != SaaSMFAChallengeEnrollment && challenge.ChallengeType != SaaSMFAChallengeLogin) {
		writeSaaSAuthEnvelope(w, http.StatusUnauthorized, "MFA_CHALLENGE_INVALID", "multi-factor authentication challenge invalid", nil)
		return
	}
	totpStep, err := handler.protector.Verify(challenge.SecretCiphertext, challenge.EncryptionKeyID, challenge.UserID, request.Code, handler.currentTime())
	if err != nil {
		_ = handler.persistence.RecordMFAFailure(r.Context(), digest)
		writeSaaSAuthEnvelope(w, http.StatusUnauthorized, "MFA_CHALLENGE_INVALID", "multi-factor authentication challenge invalid", nil)
		return
	}
	identity, err := handler.persistence.CompleteMFAChallenge(r.Context(), digest, challenge.UserID, challenge.AuthVersion, challenge.ChallengeType, totpStep)
	if err != nil {
		writeSaaSAuthEnvelope(w, http.StatusUnauthorized, "MFA_CHALLENGE_INVALID", "multi-factor authentication challenge invalid", nil)
		return
	}
	if identity.MustRotatePassword != 0 {
		handler.beginPasswordChange(w, r.Context(), identity)
		return
	}
	handler.issueToken(w, r.Context(), identity)
}

func (handler *HTTPHandler) beginPasswordChange(w http.ResponseWriter, ctx context.Context, identity SaaSIdentity) {
	token, err := newOpaqueToken()
	if err != nil {
		writeSaaSAuthEnvelope(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication unavailable", nil)
		return
	}
	expiresAt := handler.currentTime().Add(10 * time.Minute)
	if err := handler.persistence.CreateMFAChallenge(ctx, identity.ID, identity.AuthVersion, SaaSMFAChallengePasswordChange, sha256.Sum256([]byte(token)), expiresAt); err != nil {
		writeSaaSAuthEnvelope(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication unavailable", nil)
		return
	}
	writeSaaSAuthEnvelope(w, http.StatusPreconditionRequired, "PASSWORD_CHANGE_REQUIRED", "password change required", map[string]any{
		"passwordChangeToken": token, "mustRotatePassword": true,
	})
}

func (handler *HTTPHandler) passwordChange(w http.ResponseWriter, r *http.Request) {
	var request passwordChangeRequest
	if err := decodeSaaSAuthJSON(r, &request); err != nil {
		writeSaaSAuthEnvelope(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid authentication request", nil)
		return
	}
	if strings.TrimSpace(request.PasswordChangeToken) == "" || strings.TrimSpace(request.NewPassword) == "" {
		writeSaaSAuthEnvelope(w, http.StatusUnauthorized, "PASSWORD_CHANGE_INVALID", "password change request invalid", nil)
		return
	}
	hash, err := HashPassword(request.NewPassword)
	if err != nil {
		writeSaaSAuthEnvelope(w, http.StatusBadRequest, "PASSWORD_CHANGE_INVALID", "password change request invalid", nil)
		return
	}
	identity, err := handler.persistence.CompletePasswordChange(r.Context(), sha256.Sum256([]byte(strings.TrimSpace(request.PasswordChangeToken))), hash)
	if err != nil {
		writeSaaSAuthEnvelope(w, http.StatusConflict, "PASSWORD_CHANGE_INVALID", "password change request invalid", nil)
		return
	}
	handler.issueToken(w, r.Context(), identity)
}

func (handler *HTTPHandler) session(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeSaaSAuthEnvelope(w, http.StatusUnauthorized, authrealm.CodeSessionInvalid, "session invalid", nil)
		return
	}
	claims, err := handler.parser.Parse(r.Context(), token)
	if err != nil {
		writeSaaSAuthEnvelope(w, http.StatusUnauthorized, authErrorCode(err), "session invalid", nil)
		return
	}
	writeSaaSAuthEnvelope(w, http.StatusOK, "OK", "success", map[string]any{
		"userId": claims.UserID, "realm": claims.Realm, "authVersion": claims.AuthVersion,
	})
}

func (handler *HTTPHandler) logout(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeSaaSAuthEnvelope(w, http.StatusUnauthorized, authrealm.CodeSessionInvalid, "session invalid", nil)
		return
	}
	claims, err := handler.parser.Parse(r.Context(), token)
	if err != nil || handler.persistence.RevokeSession(r.Context(), claims) != nil {
		writeSaaSAuthEnvelope(w, http.StatusUnauthorized, authrealm.CodeSessionInvalid, "session invalid", nil)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "MOCHAT_SAAS_ADMIN_TOKEN", Value: "", Path: "/saas-admin", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (handler *HTTPHandler) issueToken(w http.ResponseWriter, ctx context.Context, identity SaaSIdentity) {
	now := handler.currentTime().UTC()
	jti, err := newOpaqueToken()
	if err != nil {
		writeSaaSAuthEnvelope(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication unavailable", nil)
		return
	}
	expiresAt := now.Add(handler.signer.TTL)
	token, err := authrealm.Sign(handler.signer, authrealm.Claims{UserID: identity.ID, AuthVersion: identity.AuthVersion, JWTID: jti}, now)
	if err != nil {
		writeSaaSAuthEnvelope(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication unavailable", nil)
		return
	}
	if err := handler.persistence.CreateSession(ctx, identity.ID, identity.AuthVersion, sha256.Sum256([]byte(jti)), now, expiresAt); err != nil {
		writeSaaSAuthEnvelope(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication unavailable", nil)
		return
	}
	writeSaaSAuthEnvelope(w, http.StatusOK, "OK", "success", map[string]any{
		"token": token, "userId": identity.ID, "userName": identity.Name,
		"expiresAt": expiresAt.Unix(), "mfaRequired": false, "mustRotatePassword": false,
	})
}

func (handler *HTTPHandler) currentTime() time.Time {
	if handler.now == nil {
		return time.Now().UTC()
	}
	return handler.now().UTC()
}

func NewLoginPageHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		http.Redirect(w, r, "/saas-admin/?login=1", http.StatusFound)
	})
}

type RequestGuard struct {
	parser authrealm.Parser
}

func NewRequestGuard(parser authrealm.Parser) (*RequestGuard, error) {
	if err := validateSaaSTokenConfig(parser.Config); err != nil {
		return nil, err
	}
	if parser.ValidateSession == nil {
		return nil, fmt.Errorf("%w: session validator is required", ErrInvalidHTTPConfig)
	}
	if parser.Config.Realm != authrealm.RealmSaaSAdmin {
		return nil, fmt.Errorf("%w: SaaS request guard requires the SaaS realm", ErrInvalidHTTPConfig)
	}
	return &RequestGuard{parser: parser}, nil
}

func (guard *RequestGuard) Authorize(w http.ResponseWriter, r *http.Request) bool {
	if guard == nil {
		writeSaaSAuthEnvelope(w, http.StatusUnauthorized, authrealm.CodeSessionInvalid, "session invalid", nil)
		return false
	}
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeSaaSAuthEnvelope(w, http.StatusUnauthorized, authrealm.CodeSessionInvalid, "session invalid", nil)
		return false
	}
	claims, err := guard.parser.Parse(r.Context(), token)
	if err != nil {
		writeSaaSAuthEnvelope(w, http.StatusUnauthorized, authErrorCode(err), "session invalid", nil)
		return false
	}
	request := r.WithContext(WithPrincipal(r.Context(), Principal{UserID: claims.UserID, AuthVersion: claims.AuthVersion}))
	for _, header := range []string{
		"X-Mochat-Go-User-ID", "X-Mochat-Go-Tenant-ID", "X-Mochat-Go-Corp-ID",
		"X-User-ID", "X-Employee-ID", "X-Actor-ID", "X-Tenant-ID", "X-Corp-ID",
	} {
		request.Header.Del(header)
	}
	// Compatibility for legacy SaaS handlers; this value is always derived from
	// the verified SaaS principal and never from the client request.
	request.Header.Set("X-Mochat-Go-User-ID", fmt.Sprint(claims.UserID))
	*r = *request
	return true
}

type Principal struct {
	UserID      int
	AuthVersion uint64
}

type principalContextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, error) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	if !ok || principal.UserID <= 0 || principal.AuthVersion == 0 {
		return Principal{}, ErrSessionInvalid
	}
	return principal, nil
}

func validateSaaSTokenConfig(config authrealm.TokenConfig) error {
	if err := config.Validate(); err != nil {
		return err
	}
	if config.Realm != authrealm.RealmSaaSAdmin || strings.TrimSpace(config.Prefix) == "" {
		return fmt.Errorf("%w: SaaS realm and session prefix are required", ErrInvalidHTTPConfig)
	}
	return nil
}

func sameTokenConfig(left, right authrealm.TokenConfig) bool {
	return string(left.Secret) == string(right.Secret) && left.Issuer == right.Issuer && left.Audience == right.Audience && left.TTL == right.TTL && left.Realm == right.Realm && left.Prefix == right.Prefix
}

func decodeSaaSAuthJSON(r *http.Request, target any) error {
	if r == nil || r.Body == nil {
		return ErrInvalidSaaSRequest
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrInvalidSaaSRequest
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrInvalidSaaSRequest
	}
	return nil
}

func bearerToken(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) < len("Bearer ") || !strings.EqualFold(value[:len("Bearer ")], "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(value[len("Bearer "):])
	return token, token != ""
}

func newOpaqueToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func authErrorCode(err error) string {
	var authErr *authrealm.AuthError
	if errors.As(err, &authErr) {
		return authErr.Code()
	}
	return authrealm.CodeSessionInvalid
}

func writeSaaSAuthEnvelope(w http.ResponseWriter, status int, code, message string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Code      int    `json:"code"`
		ErrorCode string `json:"errorCode,omitempty"`
		Msg       string `json:"msg"`
		Data      any    `json:"data"`
	}{Code: status, ErrorCode: code, Msg: message, Data: data})
}
