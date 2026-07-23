package wechatopencredentials

import (
	"fmt"
	"strings"
	"testing"
)

func TestManagerEncryptsTicketAndOfficialAccountCredentials(t *testing.T) {
	manager, err := NewManager(Config{EncryptionKey: weChatOpenTestKey(1), EncryptionKeyID: "wechat-open-v1", RequireEncryption: true, DedicatedConfigured: true})
	if err != nil {
		t.Fatal(err)
	}
	ticket := TicketCredential{ComponentVerifyTicket: "ticket-secret"}
	ticketCiphertext, keyID, err := manager.EncryptTicket("wx-component", ticket)
	if err != nil {
		t.Fatal(err)
	}
	if keyID != "wechat-open-v1" || strings.Contains(ticketCiphertext, ticket.ComponentVerifyTicket) {
		t.Fatalf("key=%s ciphertext=%s", keyID, ticketCiphertext)
	}
	decodedTicket, err := manager.DecryptTicket("wx-component", keyID, ticketCiphertext)
	if err != nil || decodedTicket != ticket {
		t.Fatalf("ticket=%+v err=%v", decodedTicket, err)
	}
	if _, err := manager.DecryptTicket("different-component", keyID, ticketCiphertext); err == nil {
		t.Fatal("expected ticket AAD mismatch")
	}

	account := OfficialAccountCredential{
		ComponentSecret: "component-secret", ComponentToken: "component-token", ComponentAESKey: "component-aes",
		AuthorizationCode: "auth-code", PreAuthCode: "pre-auth", AuthorizerRefreshToken: "refresh-token",
	}
	accountCiphertext, accountKeyID, err := manager.EncryptOfficialAccount(10, "authorizer-app", account)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(accountCiphertext, account.AuthorizerRefreshToken) {
		t.Fatalf("ciphertext leaked refresh token: %s", accountCiphertext)
	}
	decodedAccount, err := manager.DecryptOfficialAccount(10, "authorizer-app", accountKeyID, accountCiphertext)
	if err != nil || decodedAccount != account {
		t.Fatalf("account=%+v err=%v", decodedAccount, err)
	}
	if _, err := manager.DecryptOfficialAccount(11, "authorizer-app", accountKeyID, accountCiphertext); err == nil {
		t.Fatal("expected tenant AAD mismatch")
	}
	if _, err := manager.DecryptOfficialAccount(10, "different-authorizer", accountKeyID, accountCiphertext); err == nil {
		t.Fatal("expected authorizer AAD mismatch")
	}
}

func TestManagerReadsHistoricalKeyAndSelectsFallback(t *testing.T) {
	ring := fmt.Sprintf(`{"old":"%s","new":"%s"}`, weChatOpenTestKey(2), weChatOpenTestKey(3))
	oldManager, err := NewManager(Config{EncryptionKeys: ring, EncryptionKeyID: "old"})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, keyID, err := oldManager.EncryptTicket("component", TicketCredential{ComponentVerifyTicket: "ticket"})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(Config{EncryptionKeys: ring, EncryptionKeyID: "new"})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := manager.DecryptTicket("component", keyID, ciphertext)
	if err != nil || decoded.ComponentVerifyTicket != "ticket" {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
	selected, dedicated := SelectEncryptionSource(EncryptionSource{}, EncryptionSource{Key: weChatOpenTestKey(4), KeyID: "fallback"})
	if dedicated || selected.KeyID != "fallback" || selected.Key == "" {
		t.Fatalf("selected=%+v dedicated=%t", selected, dedicated)
	}
	if _, err := NewManager(Config{RequireEncryption: true}); err == nil {
		t.Fatal("expected required encryption validation")
	}
}

func weChatOpenTestKey(value byte) string {
	return strings.Repeat(fmt.Sprintf("%02x", value), 32)
}
