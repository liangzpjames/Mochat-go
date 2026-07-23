package identitysecurity

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

type identityTestStore struct {
	Store
	policy             Policy
	credential         MFACredential
	credentialFound    bool
	challenge          Challenge
	challengeFailures  int
	challengeConsumed  bool
	validatedSession   SessionClaims
	validateSessionErr error
}

func (s *identityTestStore) IdentityPolicy(context.Context, int) (Policy, bool, error) {
	return s.policy, s.policy.TenantID > 0, nil
}

func (s *identityTestStore) IdentityMFAEnrollmentCoverage(context.Context, int) (int, int, error) {
	return 1, boolInt(s.credentialFound && s.credential.Status == MFAStatusActive), nil
}

func (s *identityTestStore) IdentityMFACredential(context.Context, int) (MFACredential, bool, error) {
	return s.credential, s.credentialFound, nil
}

func (s *identityTestStore) SavePendingIdentityMFA(_ context.Context, credential MFACredential, _ Actor) (MFACredential, error) {
	credential.ID = 1
	credential.Version++
	s.credential, s.credentialFound = credential, true
	return credential, nil
}

func (s *identityTestStore) ActivateIdentityMFA(_ context.Context, userID, expectedVersion int, hashes string, count int, step int64, _ Actor, now time.Time) (MFACredential, error) {
	if s.credential.UserID != userID || s.credential.Version != expectedVersion {
		return MFACredential{}, Conflict("version changed")
	}
	s.credential.Status = MFAStatusActive
	s.credential.RecoveryCodeHashesJSON = hashes
	s.credential.RecoveryCodesRemaining = count
	s.credential.LastTOTPStep = step
	s.credential.VerifiedAt = now.UTC().Format(time.RFC3339)
	s.credential.Version++
	return s.credential, nil
}

func (s *identityTestStore) IdentityChallenge(context.Context, string) (Challenge, error) {
	if s.challenge.Status != ChallengeStatusPending {
		return Challenge{}, ErrSessionNotFound
	}
	s.challenge.Credential = s.credential
	return s.challenge, nil
}

func (s *identityTestStore) UseIdentityMFA(_ context.Context, userID, expectedVersion int, hashes string, count int, step int64, _ time.Time) (MFACredential, error) {
	if userID != s.credential.UserID || expectedVersion != s.credential.Version {
		return MFACredential{}, Conflict("version changed")
	}
	s.credential.RecoveryCodeHashesJSON = hashes
	s.credential.RecoveryCodesRemaining = count
	if step > 0 {
		s.credential.LastTOTPStep = step
	}
	s.credential.Version++
	return s.credential, nil
}

func (s *identityTestStore) ConsumeIdentityChallenge(_ context.Context, id int64, _ time.Time) (Challenge, error) {
	if id != s.challenge.ID {
		return Challenge{}, NotFound("challenge")
	}
	s.challenge.Status = ChallengeStatusConsumed
	s.challengeConsumed = true
	return s.challenge, nil
}

func (s *identityTestStore) FailIdentityChallenge(_ context.Context, id int64, _ string, _ time.Time) (Challenge, error) {
	if id != s.challenge.ID {
		return Challenge{}, NotFound("challenge")
	}
	s.challengeFailures++
	s.challenge.Attempts++
	return s.challenge, nil
}

func (s *identityTestStore) RecordIdentityLoginEvent(context.Context, LoginEvent, string, time.Time) error {
	return nil
}

func (s *identityTestStore) ValidateIdentitySession(_ context.Context, claims SessionClaims, _ time.Time) error {
	s.validatedSession = claims
	return s.validateSessionErr
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func identityTestKey() string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x53}, 32))
}

func TestIdentitySecretEncryptionBindsKeyAndUser(t *testing.T) {
	master := bytes.Repeat([]byte{0x41}, 32)
	ciphertext, err := encryptIdentitySecret(master, "q3", 42, "JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatal(err)
	}
	if ciphertext == "" || bytes.Contains([]byte(ciphertext), []byte("JBSWY3DPEHPK3PXP")) {
		t.Fatalf("ciphertext leaked secret: %q", ciphertext)
	}
	plaintext, err := decryptIdentitySecret(master, "q3", 42, ciphertext)
	if err != nil || plaintext != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("decrypt = %q, %v", plaintext, err)
	}
	if _, err := decryptIdentitySecret(master, "q3", 43, ciphertext); err == nil {
		t.Fatal("ciphertext must not decrypt for a different user")
	}
	if _, err := decryptIdentitySecret(master, "old", 42, ciphertext); err == nil {
		t.Fatal("ciphertext must not decrypt for a different key id")
	}
}

func TestMFAEnrollmentVerificationAndTOTPReplayProtection(t *testing.T) {
	now := time.Date(2026, 7, 11, 10, 30, 0, 0, time.UTC)
	store := &identityTestStore{policy: DefaultPolicy(9)}
	manager, err := NewManager(store, Config{EncryptionKey: identityTestKey(), EncryptionKeyID: "q3", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	principal := Principal{UserID: 42, TenantID: 9, Phone: "13800138000", Name: "测试用户"}
	enrollment, err := manager.BeginMFA(context.Background(), MFABegin{Principal: principal, Actor: Actor{UserID: 42, TenantID: 9}})
	if err != nil {
		t.Fatal(err)
	}
	if enrollment.Secret == "" || len(enrollment.RecoveryCodes) != 8 || enrollment.Version != 1 {
		t.Fatalf("unexpected enrollment: %+v", enrollment)
	}
	if bytes.Contains([]byte(store.credential.SecretCiphertext), []byte(enrollment.Secret)) {
		t.Fatal("stored MFA credential contains plaintext secret")
	}
	code, err := totp.GenerateCode(enrollment.Secret, now)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := manager.VerifyMFA(context.Background(), MFAVerify{
		Principal: principal, Code: code, ExpectedVersion: enrollment.Version, Actor: Actor{UserID: 42, TenantID: 9},
	})
	if err != nil {
		t.Fatal(err)
	}
	if credential.Status != MFAStatusActive || credential.LastTOTPStep <= 0 || credential.RecoveryCodesRemaining != 8 {
		t.Fatalf("unexpected active credential: %+v", credential)
	}

	now = now.Add(30 * time.Second)
	loginCode, err := totp.GenerateCode(enrollment.Secret, now)
	if err != nil {
		t.Fatal(err)
	}
	store.challenge = Challenge{ID: 7, UserID: 42, TenantID: 9, Status: ChallengeStatusPending, MaxAttempts: 5, IP: "127.0.0.1", ExpiresAt: now.Add(5 * time.Minute)}
	completion, err := manager.CompleteMFA(context.Background(), "challenge-one", loginCode, RequestMeta{IP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if completion.AuthMethod != AuthMethodTOTP || !store.challengeConsumed {
		t.Fatalf("unexpected completion: %+v consumed=%v", completion, store.challengeConsumed)
	}

	store.challenge = Challenge{ID: 8, UserID: 42, TenantID: 9, Status: ChallengeStatusPending, MaxAttempts: 5, IP: "127.0.0.1", ExpiresAt: now.Add(5 * time.Minute)}
	store.challengeConsumed = false
	_, err = manager.CompleteMFA(context.Background(), "challenge-two", loginCode, RequestMeta{IP: "127.0.0.1"})
	if err == nil || StatusCode(err) != 401 {
		t.Fatalf("replayed TOTP error = %v", err)
	}
	if store.challengeFailures != 1 || store.challengeConsumed {
		t.Fatalf("replayed TOTP failures=%d consumed=%v", store.challengeFailures, store.challengeConsumed)
	}
}

func TestRecoveryCodeIsSingleUse(t *testing.T) {
	now := time.Date(2026, 7, 11, 11, 0, 0, 0, time.UTC)
	store := &identityTestStore{policy: DefaultPolicy(9)}
	manager, err := NewManager(store, Config{EncryptionKey: identityTestKey(), EncryptionKeyID: "q3", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	principal := Principal{UserID: 42, TenantID: 9, Phone: "13800138000"}
	enrollment, err := manager.BeginMFA(context.Background(), MFABegin{Principal: principal})
	if err != nil {
		t.Fatal(err)
	}
	code, _ := totp.GenerateCode(enrollment.Secret, now)
	if _, err := manager.VerifyMFA(context.Background(), MFAVerify{Principal: principal, Code: code, ExpectedVersion: enrollment.Version}); err != nil {
		t.Fatal(err)
	}
	recovery := enrollment.RecoveryCodes[0]
	store.challenge = Challenge{ID: 10, UserID: 42, TenantID: 9, Status: ChallengeStatusPending, MaxAttempts: 5, ExpiresAt: now.Add(5 * time.Minute)}
	completion, err := manager.CompleteMFA(context.Background(), "recovery-one", recovery, RequestMeta{})
	if err != nil || completion.AuthMethod != AuthMethodRecovery {
		t.Fatalf("recovery completion = %+v, %v", completion, err)
	}
	if store.credential.RecoveryCodesRemaining != 7 {
		t.Fatalf("recovery codes remaining = %d", store.credential.RecoveryCodesRemaining)
	}
	store.challenge = Challenge{ID: 11, UserID: 42, TenantID: 9, Status: ChallengeStatusPending, MaxAttempts: 5, ExpiresAt: now.Add(5 * time.Minute)}
	if _, err := manager.CompleteMFA(context.Background(), "recovery-two", recovery, RequestMeta{}); err == nil {
		t.Fatal("used recovery code must not be accepted again")
	}
}

func TestCompleteMFAForTenantRejectsChallengeFromAnotherTenant(t *testing.T) {
	now := time.Date(2026, 7, 11, 11, 15, 0, 0, time.UTC)
	store := &identityTestStore{
		policy:    DefaultPolicy(9),
		challenge: Challenge{ID: 12, UserID: 42, TenantID: 9, Status: ChallengeStatusPending, MaxAttempts: 5, ExpiresAt: now.Add(5 * time.Minute)},
	}
	manager, err := NewManager(store, Config{EncryptionKey: identityTestKey(), EncryptionKeyID: "q3", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.CompleteMFAForTenant(context.Background(), "cross-tenant-challenge", "123456", RequestMeta{}, 10); err == nil || StatusCode(err) != 401 {
		t.Fatalf("cross-tenant MFA error = %v", err)
	}
	if store.challengeFailures != 0 || store.challengeConsumed {
		t.Fatalf("cross-tenant challenge was mutated: failures=%d consumed=%v", store.challengeFailures, store.challengeConsumed)
	}
}

func TestIPAllowlistAndPersistentSessionEnforcement(t *testing.T) {
	if !ipAllowed("10.8.1.9", []string{"10.8.0.0/16"}) || ipAllowed("10.9.1.9", []string{"10.8.0.0/16"}) {
		t.Fatal("CIDR allowlist decision is incorrect")
	}
	claims := SessionClaims{JTI: "session-1", UserID: 42, IssuedAt: time.Now().Add(-time.Minute), ExpiresAt: time.Now().Add(time.Hour)}
	store := &identityTestStore{validateSessionErr: ErrSessionRevoked}
	manager, err := NewManager(store, Config{EncryptionKey: identityTestKey(), EnforceSessions: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.ValidateSession(context.Background(), claims); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("ValidateSession error = %v", err)
	}
	if store.validatedSession.JTI != claims.JTI {
		t.Fatal("persistent session store was not consulted")
	}
	store.validatedSession = SessionClaims{}
	manager, err = NewManager(store, Config{EncryptionKey: identityTestKey(), EnforceSessions: false})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.ValidateSession(context.Background(), claims); err != nil || store.validatedSession.JTI != "" {
		t.Fatalf("disabled session enforcement consulted store: err=%v claims=%+v", err, store.validatedSession)
	}
}

func TestNilManagerSkipsPersistentSessionValidation(t *testing.T) {
	var manager *Manager
	if err := manager.ValidateJWTSession(context.Background(), "session-jti", 1, time.Now(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("ValidateJWTSession() error = %v", err)
	}
}

func TestManagerClientMetaUsesOnlyTrustedProxyHeaders(t *testing.T) {
	manager, err := NewManager(&identityTestStore{}, Config{
		EncryptionKey: identityTestKey(), TrustProxyHeaders: true, TrustedProxyCIDRs: []string{"10.0.0.0/8"},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "http://example.test/security/login", nil)
	request.RemoteAddr = "10.0.0.8:4321"
	request.Header.Set("X-Forwarded-For", "198.51.100.20, 192.0.2.10")
	request.Header.Set("User-Agent", "identity-test")
	meta := manager.ClientMeta(request)
	if meta.IP != "192.0.2.10" || meta.UserAgent != "identity-test" {
		t.Fatalf("ClientMeta() = %+v", meta)
	}
	status := manager.ConfigStatus()
	if !status.TrustedProxyHeaders || status.TrustedProxyCIDRCount != 1 || len(status.TrustedProxyCIDRs) != 1 {
		t.Fatalf("ConfigStatus() = %+v", status)
	}

	request.RemoteAddr = "203.0.113.8:4321"
	request.Header.Set("X-Forwarded-For", "198.51.100.99")
	if meta = manager.ClientMeta(request); meta.IP != "203.0.113.8" {
		t.Fatalf("untrusted peer ClientMeta() = %+v", meta)
	}
}

func TestManagerRejectsProxyHeadersWithoutTrustedCIDRs(t *testing.T) {
	_, err := NewManager(&identityTestStore{}, Config{EncryptionKey: identityTestKey(), TrustProxyHeaders: true})
	if err == nil {
		t.Fatal("expected trusted proxy configuration error")
	}
}
