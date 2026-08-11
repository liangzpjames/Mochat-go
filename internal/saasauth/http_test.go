package saasauth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"jiyi/mochat-go/internal/authrealm"
)

type memorySaaSAuthPersistence struct {
	mu         sync.Mutex
	statuses   map[int]int
	challenges map[[32]byte]SaaSMFAChallenge
	identities map[int]SaaSIdentity
	lastSteps  map[int]int64
	sessions   map[[32]byte]memorySession
	revoked    map[[32]byte]bool
}

type memorySession struct {
	UserID      int
	AuthVersion uint64
	IssuedAt    int64
	ExpiresAt   int64
}

func newMemorySaaSAuthPersistence(identity SaaSIdentity) *memorySaaSAuthPersistence {
	return &memorySaaSAuthPersistence{
		statuses:   make(map[int]int),
		challenges: make(map[[32]byte]SaaSMFAChallenge),
		identities: map[int]SaaSIdentity{identity.ID: identity},
		lastSteps:  make(map[int]int64),
		sessions:   make(map[[32]byte]memorySession),
		revoked:    make(map[[32]byte]bool),
	}
}

func (p *memorySaaSAuthPersistence) MFAStatus(_ context.Context, userID int) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	status, ok := p.statuses[userID]
	if !ok {
		return SaaSMFAStatusPending, nil
	}
	return status, nil
}

func (p *memorySaaSAuthPersistence) BeginMFAEnrollment(_ context.Context, userID int, authVersion uint64, digest [32]byte, expiresAt time.Time, ciphertext, keyID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.statuses[userID] = SaaSMFAStatusPending
	p.challenges[digest] = SaaSMFAChallenge{UserID: userID, AuthVersion: authVersion, ChallengeType: SaaSMFAChallengeEnrollment, ExpiresAt: expiresAt, MaxAttempts: 5, SecretCiphertext: ciphertext, EncryptionKeyID: keyID}
	return nil
}

func (p *memorySaaSAuthPersistence) CreateMFAChallenge(_ context.Context, userID int, authVersion uint64, challengeType string, digest [32]byte, expiresAt time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var ciphertext, keyID string
	for _, challenge := range p.challenges {
		if challenge.UserID == userID && challenge.ChallengeType == SaaSMFAChallengeEnrollment {
			ciphertext, keyID = challenge.SecretCiphertext, challenge.EncryptionKeyID
		}
	}
	p.challenges[digest] = SaaSMFAChallenge{UserID: userID, AuthVersion: authVersion, ChallengeType: challengeType, ExpiresAt: expiresAt, MaxAttempts: 5, SecretCiphertext: ciphertext, EncryptionKeyID: keyID}
	return nil
}

func (p *memorySaaSAuthPersistence) FindMFAChallenge(_ context.Context, digest [32]byte) (SaaSMFAChallenge, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	challenge, ok := p.challenges[digest]
	if !ok {
		return SaaSMFAChallenge{}, ErrMFAChallengeInvalid
	}
	return challenge, nil
}

func (p *memorySaaSAuthPersistence) RecordMFAFailure(_ context.Context, digest [32]byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	challenge, ok := p.challenges[digest]
	if !ok || challenge.Status != 0 || challenge.ExpiresAt.Before(time.Now()) {
		return ErrMFAChallengeInvalid
	}
	challenge.Attempts++
	if challenge.Attempts >= challenge.MaxAttempts {
		challenge.Status = 2
	}
	p.challenges[digest] = challenge
	return nil
}

func (p *memorySaaSAuthPersistence) CompleteMFAChallenge(_ context.Context, digest [32]byte, userID int, authVersion uint64, challengeType string, totpStep int64) (SaaSIdentity, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	challenge, ok := p.challenges[digest]
	if !ok || challenge.Status != 0 || challenge.UserID != userID || challenge.AuthVersion != authVersion || challenge.ChallengeType != challengeType || challenge.ExpiresAt.Before(time.Now()) || totpStep <= p.lastSteps[userID] {
		return SaaSIdentity{}, ErrMFAChallengeInvalid
	}
	identity := p.identities[userID]
	if challengeType == SaaSMFAChallengeEnrollment {
		if p.statuses[userID] == SaaSMFAStatusActive {
			return SaaSIdentity{}, ErrMFAChallengeInvalid
		}
		p.statuses[userID] = SaaSMFAStatusActive
	}
	p.lastSteps[userID] = totpStep
	challenge.Status = 1
	p.challenges[digest] = challenge
	return identity, nil
}

func (p *memorySaaSAuthPersistence) CompletePasswordChange(_ context.Context, digest [32]byte, passwordHash string) (SaaSIdentity, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	challenge, ok := p.challenges[digest]
	if !ok || challenge.Status != 0 || challenge.ChallengeType != SaaSMFAChallengePasswordChange || challenge.ExpiresAt.Before(time.Now()) {
		return SaaSIdentity{}, ErrPasswordChange
	}
	identity := p.identities[challenge.UserID]
	if identity.AuthVersion != challenge.AuthVersion || identity.Status != SaaSIdentityStatusActive || passwordHash == "" {
		return SaaSIdentity{}, ErrPasswordChange
	}
	identity.PasswordHash = passwordHash
	identity.MustRotatePassword = 0
	identity.AuthVersion++
	p.identities[identity.ID] = identity
	challenge.Status = 1
	p.challenges[digest] = challenge
	return identity, nil
}

func (p *memorySaaSAuthPersistence) CreateSession(_ context.Context, userID int, authVersion uint64, digest [32]byte, issuedAt, expiresAt time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sessions[digest] = memorySession{UserID: userID, AuthVersion: authVersion, IssuedAt: issuedAt.Unix(), ExpiresAt: expiresAt.Unix()}
	return nil
}

func (p *memorySaaSAuthPersistence) CheckSessionToken(_ context.Context, claims authrealm.Claims) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	digest := sha256.Sum256([]byte(claims.JWTID))
	session, ok := p.sessions[digest]
	if !ok || p.revoked[digest] || session.UserID != claims.UserID || session.AuthVersion != claims.AuthVersion || session.ExpiresAt <= time.Now().Unix() {
		return ErrSessionInvalid
	}
	return nil
}

func (p *memorySaaSAuthPersistence) RevokeSession(_ context.Context, claims authrealm.Claims) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	digest := sha256.Sum256([]byte(claims.JWTID))
	if session, ok := p.sessions[digest]; !ok || session.UserID != claims.UserID || session.AuthVersion != claims.AuthVersion {
		return ErrSessionInvalid
	}
	p.revoked[digest] = true
	return nil
}

func testSaaSHTTPConfig(identityStore SaaSIdentityStore, persistence SaaSAuthPersistence) HTTPConfig {
	service := NewService(identityStore)
	tokens := authrealm.TokenConfig{
		Secret: []byte("saas-http-test-secret-only"), Issuer: "mochat-go/saas-auth",
		Audience: "mochat-saas-admin", TTL: time.Hour, Realm: authrealm.RealmSaaSAdmin,
		Prefix: "mochat_saas_admin_",
	}
	return HTTPConfig{
		Service: service, Persistence: persistence, Signer: tokens, MFAKey: []byte("01234567890123456789012345678901"), MFAKeyID: "test-mfa",
		Parser: authrealm.Parser{Config: tokens, ValidateSession: persistence.CheckSessionToken},
	}
}

func TestSaaSAuthHTTPEnrollmentMFAThenPasswordCreatesDurableSession(t *testing.T) {
	initialPassword := "saas-http-initial-password"
	passwordHash, err := HashPassword(initialPassword)
	if err != nil {
		t.Fatal(err)
	}
	identity := SaaSIdentity{ID: 7, LoginName: "platform-admin", PasswordHash: passwordHash, Name: "Platform Admin", Status: SaaSIdentityStatusActive, AuthVersion: 1, MustRotatePassword: 1, MFARequired: 1}
	persistence := newMemorySaaSAuthPersistence(identity)
	identityStore := &fakeSaaSIdentityStore{authenticate: func(_ context.Context, login string) (SaaSIdentity, error) {
		if login != identity.LoginName {
			return SaaSIdentity{}, ErrIdentityNotFound
		}
		persistence.mu.Lock()
		defer persistence.mu.Unlock()
		return persistence.identities[identity.ID], nil
	}}
	handler, err := NewHTTPHandler(testSaaSHTTPConfig(identityStore, persistence))
	if err != nil {
		t.Fatal(err)
	}
	loginResponse := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/saas/auth/login", strings.NewReader(`{"login":"platform-admin","password":"saas-http-initial-password"}`))
	handler.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusAccepted {
		t.Fatalf("login status=%d body=%s", loginResponse.Code, loginResponse.Body.String())
	}
	var enrollmentEnvelope struct {
		Data struct {
			Token  string `json:"enrollmentToken"`
			Secret string `json:"enrollmentSecret"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &enrollmentEnvelope); err != nil {
		t.Fatal(err)
	}
	if enrollmentEnvelope.Data.Token == "" || enrollmentEnvelope.Data.Secret == "" {
		t.Fatal("initial login did not return one-time MFA enrollment information")
	}
	enrollmentCodeTime := time.Now().UTC().Truncate(30 * time.Second).Add(-30 * time.Second)
	code, err := totp.GenerateCode(enrollmentEnvelope.Data.Secret, enrollmentCodeTime)
	if err != nil {
		t.Fatal(err)
	}
	mfaResponse := httptest.NewRecorder()
	mfaRequest := httptest.NewRequest(http.MethodPost, "/saas/auth/mfa", strings.NewReader(`{"challengeToken":"`+enrollmentEnvelope.Data.Token+`","code":"`+code+`"}`))
	handler.ServeHTTP(mfaResponse, mfaRequest)
	if mfaResponse.Code != http.StatusPreconditionRequired || strings.Contains(mfaResponse.Body.String(), enrollmentEnvelope.Data.Secret) {
		t.Fatalf("MFA completion did not force password change without repeating secret: status=%d body=%s", mfaResponse.Code, mfaResponse.Body.String())
	}
	var passwordEnvelope struct {
		Data struct {
			Token string `json:"passwordChangeToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(mfaResponse.Body.Bytes(), &passwordEnvelope); err != nil {
		t.Fatal(err)
	}
	newPassword := "saas-http-rotated-password"
	passwordResponse := httptest.NewRecorder()
	passwordRequest := httptest.NewRequest(http.MethodPost, "/saas/auth/password", strings.NewReader(`{"passwordChangeToken":"`+passwordEnvelope.Data.Token+`","newPassword":"`+newPassword+`"}`))
	handler.ServeHTTP(passwordResponse, passwordRequest)
	if passwordResponse.Code != http.StatusOK {
		t.Fatalf("password change status=%d body=%s", passwordResponse.Code, passwordResponse.Body.String())
	}
	var tokenEnvelope struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(passwordResponse.Body.Bytes(), &tokenEnvelope); err != nil {
		t.Fatal(err)
	}
	if tokenEnvelope.Data.Token == "" {
		t.Fatal("password change did not issue a session token")
	}
	secondLoginResponse := httptest.NewRecorder()
	secondLoginRequest := httptest.NewRequest(http.MethodPost, "/saas/auth/login", strings.NewReader(`{"login":"platform-admin","password":"saas-http-rotated-password"}`))
	handler.ServeHTTP(secondLoginResponse, secondLoginRequest)
	if secondLoginResponse.Code != http.StatusAccepted {
		t.Fatalf("post-enrollment login status=%d", secondLoginResponse.Code)
	}
	var secondLoginEnvelope struct {
		Data struct {
			ChallengeToken string `json:"challengeToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(secondLoginResponse.Body.Bytes(), &secondLoginEnvelope); err != nil {
		t.Fatal(err)
	}
	secondChallengeDigest := sha256.Sum256([]byte(secondLoginEnvelope.Data.ChallengeToken))
	persistence.mu.Lock()
	secondChallenge, ok := persistence.challenges[secondChallengeDigest]
	persistence.mu.Unlock()
	if !ok || secondChallenge.ChallengeType != SaaSMFAChallengeLogin {
		t.Fatalf("post-enrollment login challenge type = %q, want %q", secondChallenge.ChallengeType, SaaSMFAChallengeLogin)
	}
	secondCode, err := totp.GenerateCode(enrollmentEnvelope.Data.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	secondMFAResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondMFAResponse, httptest.NewRequest(http.MethodPost, "/saas/auth/mfa", strings.NewReader(`{"challengeToken":"`+secondLoginEnvelope.Data.ChallengeToken+`","code":"`+secondCode+`"}`)))
	if secondMFAResponse.Code != http.StatusOK {
		t.Fatalf("second MFA status=%d body=%s", secondMFAResponse.Code, secondMFAResponse.Body.String())
	}
	persistence.mu.Lock()
	sessionsAfterSecondMFA := len(persistence.sessions)
	persistence.mu.Unlock()
	thirdLoginResponse := httptest.NewRecorder()
	handler.ServeHTTP(thirdLoginResponse, httptest.NewRequest(http.MethodPost, "/saas/auth/login", strings.NewReader(`{"login":"platform-admin","password":"saas-http-rotated-password"}`)))
	if thirdLoginResponse.Code != http.StatusAccepted {
		t.Fatalf("third login status=%d body=%s", thirdLoginResponse.Code, thirdLoginResponse.Body.String())
	}
	var thirdLoginEnvelope struct {
		Data struct {
			ChallengeToken string `json:"challengeToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(thirdLoginResponse.Body.Bytes(), &thirdLoginEnvelope); err != nil {
		t.Fatal(err)
	}
	sameStepReplay := httptest.NewRecorder()
	handler.ServeHTTP(sameStepReplay, httptest.NewRequest(http.MethodPost, "/saas/auth/mfa", strings.NewReader(`{"challengeToken":"`+thirdLoginEnvelope.Data.ChallengeToken+`","code":"`+secondCode+`"}`)))
	if sameStepReplay.Code != http.StatusUnauthorized {
		t.Fatalf("same TOTP step replay status=%d body=%s", sameStepReplay.Code, sameStepReplay.Body.String())
	}
	persistence.mu.Lock()
	if len(persistence.sessions) != sessionsAfterSecondMFA {
		t.Fatalf("same TOTP step replay changed session count from %d to %d", sessionsAfterSecondMFA, len(persistence.sessions))
	}
	persistence.mu.Unlock()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/saas/auth/session", nil)
	sessionRequest.Header.Set("Authorization", "Bearer "+tokenEnvelope.Data.Token)
	sessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(sessionResponse, sessionRequest)
	if sessionResponse.Code != http.StatusOK {
		t.Fatalf("durable session was not accepted: status=%d body=%s", sessionResponse.Code, sessionResponse.Body.String())
	}
	enrollmentReplayResponse := httptest.NewRecorder()
	handler.ServeHTTP(enrollmentReplayResponse, httptest.NewRequest(http.MethodPost, "/saas/auth/mfa", strings.NewReader(`{"challengeToken":"`+enrollmentEnvelope.Data.Token+`","code":"`+code+`"}`)))
	if enrollmentReplayResponse.Code != http.StatusUnauthorized {
		t.Fatalf("enrollment challenge was reusable: status=%d body=%s", enrollmentReplayResponse.Code, enrollmentReplayResponse.Body.String())
	}
	logoutRequest := httptest.NewRequest(http.MethodPost, "/saas/auth/logout", nil)
	logoutRequest.Header.Set("Authorization", "Bearer "+tokenEnvelope.Data.Token)
	logoutResponse := httptest.NewRecorder()
	handler.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("logout status=%d body=%s", logoutResponse.Code, logoutResponse.Body.String())
	}
	oldSessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(oldSessionResponse, sessionRequest)
	if oldSessionResponse.Code != http.StatusUnauthorized {
		t.Fatalf("revoked bearer remained valid: status=%d body=%s", oldSessionResponse.Code, oldSessionResponse.Body.String())
	}
}

func TestSaaSAuthHTTPLoginErrorsAreUniform(t *testing.T) {
	hash, err := HashPassword("saas-http-uniform-password")
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeSaaSIdentityStore{authenticate: func(_ context.Context, login string) (SaaSIdentity, error) {
		if login == "known-admin" {
			return SaaSIdentity{ID: 10, LoginName: login, PasswordHash: hash, Status: SaaSIdentityStatusActive, AuthVersion: 1}, nil
		}
		return SaaSIdentity{}, ErrIdentityNotFound
	}}
	identity := SaaSIdentity{ID: 10, LoginName: "known-admin", Status: SaaSIdentityStatusActive, AuthVersion: 1}
	handler, err := NewHTTPHandler(testSaaSHTTPConfig(store, newMemorySaaSAuthPersistence(identity)))
	if err != nil {
		t.Fatal(err)
	}
	request := func(body string) string {
		r := httptest.NewRequest(http.MethodPost, "/saas/auth/login", strings.NewReader(body))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, r)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("login status=%d body=%s", response.Code, response.Body.String())
		}
		return response.Body.String()
	}
	if wrong, unknown := request(`{"login":"known-admin","password":"wrong"}`), request(`{"login":"missing-admin","password":"wrong"}`); wrong != unknown {
		t.Fatalf("login errors were not uniform: wrong=%s unknown=%s", wrong, unknown)
	}
}

func TestSaaSAuthRejectsUnknownAndTrailingJSONWithoutAuthentication(t *testing.T) {
	passwordHash, err := HashPassword("strict-json-password")
	if err != nil {
		t.Fatal(err)
	}
	identity := SaaSIdentity{ID: 12, LoginName: "strict-json-admin", PasswordHash: passwordHash, Name: "Strict JSON Admin", Status: SaaSIdentityStatusActive, AuthVersion: 1}
	persistence := newMemorySaaSAuthPersistence(identity)
	authenticateCalls := 0
	identityStore := &fakeSaaSIdentityStore{authenticate: func(_ context.Context, login string) (SaaSIdentity, error) {
		authenticateCalls++
		if login != identity.LoginName {
			return SaaSIdentity{}, ErrIdentityNotFound
		}
		return identity, nil
	}}
	handler, err := NewHTTPHandler(testSaaSHTTPConfig(identityStore, persistence))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		path string
		body string
	}{
		{name: "tenant field", path: "/saas/auth/login", body: `{"login":"strict-json-admin","password":"strict-json-password","tenantId":1}`},
		{name: "actor field", path: "/saas/auth/mfa", body: `{"challengeToken":"challenge","code":"000000","actorId":1}`},
		{name: "corp field", path: "/saas/auth/password", body: `{"passwordChangeToken":"challenge","newPassword":"new-password","corpId":1}`},
		{name: "trailing object", path: "/saas/auth/login", body: `{} {}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s, want 400", response.Code, response.Body.String())
			}
			var envelope struct {
				ErrorCode string `json:"errorCode"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.ErrorCode != "INVALID_REQUEST" {
				t.Fatalf("errorCode=%q, want INVALID_REQUEST", envelope.ErrorCode)
			}
		})
	}
	if authenticateCalls != 0 {
		t.Fatalf("malformed requests reached authentication %d times", authenticateCalls)
	}
	persistence.mu.Lock()
	defer persistence.mu.Unlock()
	if len(persistence.sessions) != 0 {
		t.Fatalf("malformed requests created %d sessions", len(persistence.sessions))
	}
}

func TestSaaSAuthExpiredConsumedAndLockedChallengesReturn401WithoutExtraSessions(t *testing.T) {
	t.Run("expired challenge", func(t *testing.T) {
		handler, persistence, _, token, secret := newMemoryEnrollmentFlow(t, 0)
		digest := sha256.Sum256([]byte(token))
		persistence.mu.Lock()
		challenge := persistence.challenges[digest]
		challenge.ExpiresAt = time.Now().Add(-time.Minute)
		persistence.challenges[digest] = challenge
		persistence.mu.Unlock()
		code, err := totp.GenerateCode(secret, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/saas/auth/mfa", strings.NewReader(`{"challengeToken":"`+token+`","code":"`+code+`"}`)))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expired challenge status=%d body=%s", response.Code, response.Body.String())
		}
		persistence.mu.Lock()
		defer persistence.mu.Unlock()
		if len(persistence.sessions) != 0 {
			t.Fatalf("expired challenge created %d sessions", len(persistence.sessions))
		}
	})

	t.Run("consumed challenge", func(t *testing.T) {
		handler, persistence, _, token, secret := newMemoryEnrollmentFlow(t, 0)
		code, err := totp.GenerateCode(secret, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		body := `{"challengeToken":"` + token + `","code":"` + code + `"}`
		first := httptest.NewRecorder()
		handler.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/saas/auth/mfa", strings.NewReader(body)))
		if first.Code != http.StatusOK {
			t.Fatalf("first MFA status=%d body=%s", first.Code, first.Body.String())
		}
		persistence.mu.Lock()
		sessionCount := len(persistence.sessions)
		persistence.mu.Unlock()
		replay := httptest.NewRecorder()
		handler.ServeHTTP(replay, httptest.NewRequest(http.MethodPost, "/saas/auth/mfa", strings.NewReader(body)))
		if replay.Code != http.StatusUnauthorized {
			t.Fatalf("consumed challenge replay status=%d body=%s", replay.Code, replay.Body.String())
		}
		persistence.mu.Lock()
		defer persistence.mu.Unlock()
		if len(persistence.sessions) != sessionCount {
			t.Fatalf("consumed replay changed session count from %d to %d", sessionCount, len(persistence.sessions))
		}
	})

	t.Run("attempt limit", func(t *testing.T) {
		handler, persistence, identity, _, secret := newMemoryEnrollmentFlow(t, 0)
		persistence.mu.Lock()
		persistence.statuses[identity.ID] = SaaSMFAStatusActive
		persistence.mu.Unlock()
		login := httptest.NewRecorder()
		handler.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/saas/auth/login", strings.NewReader(`{"login":"platform-admin","password":"flow-initial-password"}`)))
		if login.Code != http.StatusAccepted {
			t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
		}
		var loginEnvelope struct {
			Data struct {
				ChallengeToken string `json:"challengeToken"`
			} `json:"data"`
		}
		if err := json.Unmarshal(login.Body.Bytes(), &loginEnvelope); err != nil {
			t.Fatal(err)
		}
		if loginEnvelope.Data.ChallengeToken == "" {
			t.Fatal("login did not issue MFA challenge")
		}
		body := `{"challengeToken":"` + loginEnvelope.Data.ChallengeToken + `","code":"not-a-code"}`
		for attempt := 0; attempt < 5; attempt++ {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/saas/auth/mfa", strings.NewReader(body)))
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("invalid attempt %d status=%d", attempt+1, response.Code)
			}
		}
		validCode, err := totp.GenerateCode(secret, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		locked := httptest.NewRecorder()
		handler.ServeHTTP(locked, httptest.NewRequest(http.MethodPost, "/saas/auth/mfa", strings.NewReader(`{"challengeToken":"`+loginEnvelope.Data.ChallengeToken+`","code":"`+validCode+`"}`)))
		if locked.Code != http.StatusUnauthorized {
			t.Fatalf("locked challenge status=%d body=%s", locked.Code, locked.Body.String())
		}
		persistence.mu.Lock()
		defer persistence.mu.Unlock()
		if len(persistence.sessions) != 0 {
			t.Fatalf("locked challenge created %d sessions", len(persistence.sessions))
		}
	})
}

func newMemoryEnrollmentFlow(t *testing.T, mustRotate int) (*HTTPHandler, *memorySaaSAuthPersistence, SaaSIdentity, string, string) {
	t.Helper()
	const initialPassword = "flow-initial-password"
	passwordHash, err := HashPassword(initialPassword)
	if err != nil {
		t.Fatal(err)
	}
	identity := SaaSIdentity{ID: 13, LoginName: "platform-admin", PasswordHash: passwordHash, Name: "Platform Admin", Status: SaaSIdentityStatusActive, AuthVersion: 1, MustRotatePassword: mustRotate, MFARequired: 1}
	persistence := newMemorySaaSAuthPersistence(identity)
	identityStore := &fakeSaaSIdentityStore{authenticate: func(_ context.Context, login string) (SaaSIdentity, error) {
		if login != identity.LoginName {
			return SaaSIdentity{}, ErrIdentityNotFound
		}
		persistence.mu.Lock()
		defer persistence.mu.Unlock()
		return persistence.identities[identity.ID], nil
	}}
	handler, err := NewHTTPHandler(testSaaSHTTPConfig(identityStore, persistence))
	if err != nil {
		t.Fatal(err)
	}
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/saas/auth/login", strings.NewReader(`{"login":"platform-admin","password":"flow-initial-password"}`)))
	if login.Code != http.StatusAccepted {
		t.Fatalf("enrollment login status=%d body=%s", login.Code, login.Body.String())
	}
	var envelope struct {
		Data struct {
			Token  string `json:"enrollmentToken"`
			Secret string `json:"enrollmentSecret"`
		} `json:"data"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Token == "" || envelope.Data.Secret == "" {
		t.Fatal("enrollment login did not issue challenge")
	}
	return handler, persistence, identity, envelope.Data.Token, envelope.Data.Secret
}

func TestSaaSAuthHTTPLogoutRequiresBearer(t *testing.T) {
	identity := SaaSIdentity{ID: 11, LoginName: "admin", Status: SaaSIdentityStatusActive, AuthVersion: 1}
	store := &fakeSaaSIdentityStore{}
	handler, err := NewHTTPHandler(testSaaSHTTPConfig(store, newMemorySaaSAuthPersistence(identity)))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/saas/auth/logout", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("logout without bearer status=%d body=%s", response.Code, response.Body.String())
	}
}
