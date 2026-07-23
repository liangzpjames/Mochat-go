package store

import (
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/wechatopencredentials"
)

func TestWeChatOpenCredentialStorageEncryptsAndDecrypts(t *testing.T) {
	manager := testWeChatOpenCredentialManager(t, wechatopencredentials.Config{
		EncryptionKey: strings.Repeat("41", 32), EncryptionKeyID: "wechat-open-q3", RequireEncryption: true, DedicatedConfigured: true,
	})
	store := &MySQLStore{weChatOpenCredentialCipher: manager}
	ticketStorage, err := store.encodeWeChatComponentTicketCredential("component-app", wechatopencredentials.TicketCredential{ComponentVerifyTicket: "ticket-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if ticketStorage.Ticket != "" || ticketStorage.Ciphertext == "" || ticketStorage.KeyID != "wechat-open-q3" || strings.Contains(ticketStorage.Ciphertext, "ticket-secret") {
		t.Fatalf("ticket storage=%+v", ticketStorage)
	}
	ticket, err := store.decodeWeChatComponentTicketCredential(weChatComponentTicketCredentialRecord{ComponentAppID: "component-app", Ciphertext: ticketStorage.Ciphertext, KeyID: ticketStorage.KeyID})
	if err != nil || ticket.ComponentVerifyTicket != "ticket-secret" {
		t.Fatalf("ticket=%+v err=%v", ticket, err)
	}

	credential := wechatopencredentials.OfficialAccountCredential{
		ComponentSecret: "secret", ComponentToken: "token", ComponentAESKey: "aes", AuthorizationCode: "auth",
		PreAuthCode: "pre", AuthorizerRefreshToken: "refresh",
	}
	storage, err := store.encodeOfficialAccountCredential(10, "authorizer", credential)
	if err != nil {
		t.Fatal(err)
	}
	if storage.ComponentSecret != "" || storage.ComponentToken != "" || storage.ComponentAESKey != "" || storage.AuthorizationCode != "" || storage.PreAuthCode != "" || storage.Ciphertext == "" || strings.Contains(storage.Ciphertext, "refresh") {
		t.Fatalf("official account storage=%+v", storage)
	}
	decoded, err := store.decodeOfficialAccountCredential(officialAccountCredentialRecord{TenantID: 10, AuthorizerAppID: "authorizer", Ciphertext: storage.Ciphertext, KeyID: storage.KeyID})
	if err != nil || decoded != credential {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
}

func TestWeChatOpenCredentialStorageLegacyAndMerge(t *testing.T) {
	store := &MySQLStore{}
	legacyTicket, err := store.decodeWeChatComponentTicketCredential(weChatComponentTicketCredentialRecord{Ticket: " legacy-ticket "})
	if err != nil || legacyTicket.ComponentVerifyTicket != "legacy-ticket" {
		t.Fatalf("ticket=%+v err=%v", legacyTicket, err)
	}
	legacyAccount, err := store.decodeOfficialAccountCredential(officialAccountCredentialRecord{
		ComponentSecret: "secret", ComponentToken: "token", ComponentAESKey: "aes", AuthorizationCode: "auth", PreAuthCode: "pre",
	})
	if err != nil || legacyAccount.ComponentSecret != "secret" || legacyAccount.ComponentToken != "token" {
		t.Fatalf("account=%+v err=%v", legacyAccount, err)
	}
	merged := mergeOfficialAccountCredential(
		wechatopencredentials.OfficialAccountCredential{ComponentSecret: "old-secret", AuthorizerRefreshToken: "long-lived-refresh"},
		officialAccountCredentialFromAuthorization(dashboard.OfficialAccountAuthorization{ComponentSecret: "new-secret", AuthorizationCode: "new-auth"}),
	)
	if merged.ComponentSecret != "new-secret" || merged.AuthorizationCode != "new-auth" || merged.AuthorizerRefreshToken != "long-lived-refresh" {
		t.Fatalf("merged=%+v", merged)
	}
	plaintext, err := store.encodeOfficialAccountCredential(10, "authorizer", merged)
	if err != nil || plaintext.ComponentSecret != "new-secret" || plaintext.Ciphertext != "" {
		t.Fatalf("plaintext=%+v err=%v", plaintext, err)
	}
}

func TestWeChatOpenCredentialRotationClassification(t *testing.T) {
	if !weChatComponentTicketCredentialNeedsRotation(weChatComponentTicketCredentialRecord{Ticket: "legacy"}, "active") ||
		weChatComponentTicketCredentialNeedsRotation(weChatComponentTicketCredentialRecord{Ciphertext: "cipher", KeyID: "active"}, "active") {
		t.Fatal("ticket rotation classification mismatch")
	}
	if !officialAccountCredentialNeedsRotation(officialAccountCredentialRecord{ComponentSecret: "legacy"}, "active") ||
		officialAccountCredentialNeedsRotation(officialAccountCredentialRecord{Ciphertext: "cipher", KeyID: "active"}, "active") {
		t.Fatal("official account rotation classification mismatch")
	}
	status := dashboard.SaaSWeChatOpenCredentialProtectionStatus{ActiveKeyID: "active", KeyCount: 1}
	unavailable := map[string]struct{}{}
	updateWeChatOpenProtectionStatus(&status, unavailable, "cipher", "", false, nil)
	if _, found := unavailable["(missing)"]; !found || status.EncryptedCredentialCount != 1 || status.RotationRequiredCount != 1 {
		t.Fatalf("status=%+v unavailable=%+v", status, unavailable)
	}
}

func testWeChatOpenCredentialManager(t *testing.T, config wechatopencredentials.Config) *wechatopencredentials.Manager {
	t.Helper()
	manager, err := wechatopencredentials.NewManager(config)
	if err != nil {
		t.Fatal(err)
	}
	return manager
}
