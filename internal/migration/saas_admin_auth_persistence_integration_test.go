package migration

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"jiyi/mochat-go/internal/authrealm"
	"jiyi/mochat-go/internal/saasauth"
	"jiyi/mochat-go/internal/store"
)

func TestSaaSAdminAuthPersistenceAgainstIsolatedMariaDB(t *testing.T) {
	db := newIdentitySingleCorpMigrationDB(t)
	createIdentitySingleCorpBaseFixture(t, db)
	execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", false)

	ctx := context.Background()
	saasStore := store.NewSaaSIdentityStore(db)
	service := saasauth.NewService(saasStore)
	initialHash, err := saasauth.HashPassword("task5-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := saasStore.Bootstrap(ctx, saasauth.BootstrapSaaSAdmin{
		RequestKey: "task5-real-bootstrap-1", LoginName: "real-platform-admin",
		Phone: "13800000007", Name: "Real Platform Admin", PasswordHash: initialHash,
	}); err != nil {
		t.Fatal(err)
	}

	handler := newRealSaaSHTTPHandler(t, service, saasStore)
	countsBefore := identityRealmsCounts(t, db)
	login := postRealSaaSJSON(t, handler, "/saas/auth/login", map[string]any{
		"login": "real-platform-admin", "password": "task5-initial-password",
	})
	if login.status != http.StatusAccepted {
		t.Fatalf("initial SaaS login status=%d", login.status)
	}
	var enrollment struct {
		EnrollmentToken  string `json:"enrollmentToken"`
		EnrollmentSecret string `json:"enrollmentSecret"`
	}
	decodeRealSaaSData(t, login.body, &enrollment)
	if enrollment.EnrollmentToken == "" || enrollment.EnrollmentSecret == "" {
		t.Fatal("initial SaaS login did not return one-time enrollment data")
	}
	code, err := totp.GenerateCode(enrollment.EnrollmentSecret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	mfa := postRealSaaSJSON(t, handler, "/saas/auth/mfa", map[string]any{
		"challengeToken": enrollment.EnrollmentToken, "code": code,
	})
	if mfa.status != http.StatusPreconditionRequired {
		t.Fatalf("enrollment MFA status=%d", mfa.status)
	}
	var passwordChallenge struct {
		PasswordChangeToken string `json:"passwordChangeToken"`
	}
	decodeRealSaaSData(t, mfa.body, &passwordChallenge)
	if passwordChallenge.PasswordChangeToken == "" {
		t.Fatal("enrollment MFA did not issue a password-change challenge")
	}
	password := postRealSaaSJSON(t, handler, "/saas/auth/password", map[string]any{
		"passwordChangeToken": passwordChallenge.PasswordChangeToken, "newPassword": "task5-rotated-password",
	})
	if password.status != http.StatusOK {
		t.Fatalf("password rotation status=%d", password.status)
	}
	var firstSession struct {
		Token string `json:"token"`
	}
	decodeRealSaaSData(t, password.body, &firstSession)
	if firstSession.Token == "" {
		t.Fatal("password rotation did not issue a session")
	}

	var mfaStatus int
	if err := db.QueryRow(`SELECT status FROM mochat_go_saas_admin_mfa_credentials WHERE user_id = (SELECT id FROM mochat_go_saas_admin_users WHERE login_name = 'real-platform-admin')`).Scan(&mfaStatus); err != nil {
		t.Fatal(err)
	}
	if mfaStatus != saasauth.SaaSMFAStatusActive {
		t.Fatalf("persisted MFA status=%d, want active", mfaStatus)
	}
	var ciphertext string
	if err := db.QueryRow(`SELECT secret_ciphertext FROM mochat_go_saas_admin_mfa_credentials WHERE user_id = (SELECT id FROM mochat_go_saas_admin_users WHERE login_name = 'real-platform-admin')`).Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	if ciphertext == "" || ciphertext == enrollment.EnrollmentSecret {
		t.Fatal("MFA secret was not persisted as ciphertext")
	}
	assertIdentityRealmCountsEqual(t, db, countsBefore, "SaaS auth flow must not create Dashboard, tenant, corp, or mc_user rows")

	// Recreate the handler over the same Store to prove the challenge survives a process restart.
	restartedHandler := newRealSaaSHTTPHandler(t, service, saasStore)
	secondLogin := postRealSaaSJSON(t, restartedHandler, "/saas/auth/login", map[string]any{
		"login": "real-platform-admin", "password": "task5-rotated-password",
	})
	if secondLogin.status != http.StatusAccepted {
		t.Fatalf("post-enrollment SaaS login status=%d", secondLogin.status)
	}
	var loginChallenge struct {
		ChallengeToken string `json:"challengeToken"`
	}
	decodeRealSaaSData(t, secondLogin.body, &loginChallenge)
	if loginChallenge.ChallengeToken == "" {
		t.Fatal("post-enrollment login did not issue a challenge")
	}
	challengeDigest := sha256.Sum256([]byte(loginChallenge.ChallengeToken))
	var challengeType string
	if err := db.QueryRow(`SELECT challenge_type FROM mochat_go_saas_admin_mfa_challenges WHERE token_digest = ?`, challengeDigest[:]).Scan(&challengeType); err != nil {
		t.Fatal(err)
	}
	if challengeType != saasauth.SaaSMFAChallengeLogin {
		t.Fatalf("post-enrollment challenge type=%q, want login_mfa", challengeType)
	}
	secondCode, err := totp.GenerateCode(enrollment.EnrollmentSecret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	secondMFA := postRealSaaSJSON(t, restartedHandler, "/saas/auth/mfa", map[string]any{
		"challengeToken": loginChallenge.ChallengeToken, "code": secondCode,
	})
	if secondMFA.status != http.StatusOK {
		t.Fatalf("post-restart MFA status=%d", secondMFA.status)
	}
	var secondSession struct {
		Token string `json:"token"`
	}
	decodeRealSaaSData(t, secondMFA.body, &secondSession)
	if secondSession.Token == "" {
		t.Fatal("post-restart MFA did not issue a session")
	}

	if got := realSaaSSessionStatus(t, restartedHandler, secondSession.Token); got != http.StatusOK {
		t.Fatalf("durable SaaS session status=%d", got)
	}
	logout := httptest.NewRequest(http.MethodPost, "/saas/auth/logout", nil)
	logout.Header.Set("Authorization", "Bearer "+secondSession.Token)
	logoutResponse := httptest.NewRecorder()
	restartedHandler.ServeHTTP(logoutResponse, logout)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("SaaS logout status=%d", logoutResponse.Code)
	}
	if got := realSaaSSessionStatus(t, restartedHandler, secondSession.Token); got != http.StatusUnauthorized {
		t.Fatalf("revoked SaaS session status=%d", got)
	}
}

type realSaaSResponse struct {
	status int
	body   []byte
}

func newRealSaaSHTTPHandler(t *testing.T, service *saasauth.Service, persistence *store.SaaSIdentityStore) http.Handler {
	t.Helper()
	tokens := authrealm.TokenConfig{
		Secret: []byte("task5-real-saas-jwt-secret"), Issuer: "mochat-go/saas-auth",
		Audience: "mochat-saas-admin", TTL: time.Hour, Realm: authrealm.RealmSaaSAdmin,
		Prefix: "mochat_saas_admin_",
	}
	parser := authrealm.Parser{Config: tokens, ValidateSession: service.CheckTokenSession}
	handler, err := saasauth.NewHTTPHandler(saasauth.HTTPConfig{
		Service: service, Persistence: persistence, Signer: tokens, Parser: parser,
		MFAKey: make([]byte, 32), MFAKeyID: "task5-real-mfa",
	})
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func postRealSaaSJSON(t *testing.T, handler http.Handler, path string, payload map[string]any) realSaaSResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	record := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
	handler.ServeHTTP(record, request)
	return realSaaSResponse{status: record.Code, body: record.Body.Bytes()}
}

func decodeRealSaaSData(t *testing.T, body []byte, target any) {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		t.Fatal("SaaS auth response did not contain data")
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		t.Fatal(err)
	}
}

func realSaaSSessionStatus(t *testing.T, handler http.Handler, token string) int {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/saas/auth/session", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	record := httptest.NewRecorder()
	handler.ServeHTTP(record, request)
	return record.Code
}
