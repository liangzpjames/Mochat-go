package archivefixture

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/wecomarchivedemo"
)

const suiteTestAESKey = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

func TestSuiteProviderReceivesEncryptedTicketAndRejectsTamper(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	provider, err := NewSuiteProvider("ww-suite-local", "suite-secret-local", "callback-token-local", suiteTestAESKey)
	if err != nil {
		t.Fatal(err)
	}
	provider.WithClock(func() time.Time { return now })
	values, encrypted, err := provider.BuildTicketCallback("ticket-new", "1787832000", "nonce-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.ReceiveTicket(values, encrypted); err != nil {
		t.Fatal(err)
	}
	if provider.LatestTicketConfigured() != true {
		t.Fatal("latest suite ticket was not stored")
	}
	values.Set("msg_signature", "tampered")
	if err := provider.ReceiveTicket(values, encrypted); err == nil {
		t.Fatal("tampered suite callback accepted")
	} else if strings.Contains(err.Error(), "ticket-new") || strings.Contains(err.Error(), encrypted) {
		t.Fatalf("callback error leaked sensitive material: %v", err)
	}
}

func TestSuiteProviderRejectsExpiredAndReplayedCallbacks(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	provider, err := NewSuiteProvider("ww-suite-local", "suite-secret-local", "callback-token-local", suiteTestAESKey)
	if err != nil {
		t.Fatal(err)
	}
	provider.WithClock(func() time.Time { return now })
	values, encrypted, err := provider.BuildTicketCallback("ticket-new", "1787832000", "nonce-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.ReceiveTicket(values, encrypted); err != nil {
		t.Fatal(err)
	}
	if err := provider.ReceiveTicket(values, encrypted); ErrorCode(err) != "SUITE_CALLBACK_REPLAYED" {
		t.Fatalf("replay error=%v", err)
	}
	expiredValues, expired, _ := provider.BuildTicketCallback("ticket-old", "1787831000", "nonce-old")
	if err := provider.ReceiveTicket(expiredValues, expired); ErrorCode(err) != "SUITE_CALLBACK_EXPIRED" {
		t.Fatalf("expired error=%v", err)
	}
}

func TestSuiteProviderAuthorizationLifecycleIsTenantAndCorpIsolated(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	provider, err := NewSuiteProvider("ww-suite-local", "suite-secret-local", "callback-token-local", suiteTestAESKey)
	if err != nil {
		t.Fatal(err)
	}
	provider.WithClock(func() time.Time { return now })
	values, encrypted, err := provider.BuildTicketCallback("ticket-new", "1787832000", "nonce-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.ReceiveTicket(values, encrypted); err != nil {
		t.Fatal(err)
	}
	suiteToken, err := provider.ExchangeSuiteToken("ww-suite-local", "suite-secret-local", "ticket-new")
	if err != nil || suiteToken.Value == "" || !suiteToken.ExpiresAt.Equal(now.Add(2*time.Hour)) {
		t.Fatalf("suite token=%+v err=%v", suiteToken, err)
	}
	preAuth, err := provider.CreatePreAuthCode(suiteToken.Value)
	if err != nil || preAuth == "" {
		t.Fatalf("pre-auth=%q err=%v", preAuth, err)
	}
	authCode, err := provider.Authorize(preAuth, 201, "ww-corp-a")
	if err != nil {
		t.Fatal(err)
	}
	authorization, err := provider.ExchangePermanentCode(suiteToken.Value, authCode)
	if err != nil || authorization.TenantID != 201 || authorization.CorpID != "ww-corp-a" || authorization.PermanentCode == "" {
		t.Fatalf("authorization=%+v err=%v", authorization, err)
	}
	if _, err := provider.ExchangePermanentCode(suiteToken.Value, authCode); ErrorCode(err) != "AUTH_CODE_CONSUMED" {
		t.Fatalf("reused auth code error=%v", err)
	}
	corpToken, err := provider.CorpToken(suiteToken.Value, 201, "ww-corp-a", authorization.PermanentCode)
	if err != nil || corpToken.Value == "" {
		t.Fatalf("corp token=%+v err=%v", corpToken, err)
	}
	if _, err := provider.CorpToken(suiteToken.Value, 202, "ww-corp-a", authorization.PermanentCode); ErrorCode(err) != "CORP_SCOPE_MISMATCH" {
		t.Fatalf("cross-tenant token error=%v", err)
	}
	provider.RevokeAuthorization(201, "ww-corp-a")
	if _, err := provider.CorpToken(suiteToken.Value, 201, "ww-corp-a", authorization.PermanentCode); ErrorCode(err) != "AUTHORIZATION_REVOKED" {
		t.Fatalf("revoked token error=%v", err)
	}
}

func TestSuiteProviderExpiresTokensAndNeverLeaksSecrets(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	provider, err := NewSuiteProvider("ww-suite-local", "suite-secret-local", "callback-token-local", suiteTestAESKey)
	if err != nil {
		t.Fatal(err)
	}
	provider.WithClock(func() time.Time { return now })
	values, encrypted, _ := provider.BuildTicketCallback("ticket-new", "1787832000", "nonce-a")
	if err := provider.ReceiveTicket(values, encrypted); err != nil {
		t.Fatal(err)
	}
	token, err := provider.ExchangeSuiteToken("ww-suite-local", "suite-secret-local", "ticket-new")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2*time.Hour + time.Second)
	_, err = provider.CreatePreAuthCode(token.Value)
	if ErrorCode(err) != "SUITE_TOKEN_EXPIRED" {
		t.Fatalf("expired token error=%v", err)
	}
	for _, secret := range []string{"suite-secret-local", "ticket-new", token.Value, encrypted} {
		if err != nil && strings.Contains(err.Error(), secret) {
			t.Fatalf("error leaked secret %q: %v", secret, err)
		}
	}
}

func TestSuiteAuthorizationKeyUsesStableDecimalTenantIdentity(t *testing.T) {
	if got, want := suiteAuthorizationKey(201, "ww-corp-a"), "201\x00ww-corp-a"; got != want {
		t.Fatalf("suite authorization key=%q want=%q", got, want)
	}
}

func TestSuiteProviderBuildsEncryptedCreateAuthCallback(t *testing.T) {
	provider, err := NewSuiteProvider("ww-suite-local", "suite-secret-local", "callback-token-local", suiteTestAESKey)
	if err != nil {
		t.Fatal(err)
	}
	values, encrypted, err := provider.BuildAuthorizationCallback("auth-code-local", "1787832000", "nonce-auth")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := wecomarchivedemo.VerifyAndDecryptCallback("callback-token-local", suiteTestAESKey, "ww-suite-local", values, encrypted)
	if err != nil {
		t.Fatal(err)
	}
	var event struct {
		SuiteID  string `xml:"SuiteId"`
		InfoType string `xml:"InfoType"`
		AuthCode string `xml:"AuthCode"`
	}
	if err := xml.Unmarshal(plain.Message, &event); err != nil || event.SuiteID != "ww-suite-local" || event.InfoType != "create_auth" || event.AuthCode != "auth-code-local" {
		t.Fatalf("event=%+v err=%v", event, err)
	}
}
