package store

import (
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/saasalertcredentials"
)

func TestSaaSAlertCredentialStorageEncryptsAndDecrypts(t *testing.T) {
	manager, err := saasalertcredentials.NewManager(saasalertcredentials.Config{
		EncryptionKey:   "2121212121212121212121212121212121212121212121212121212121212121",
		EncryptionKeyID: "alert-q3", RequireEncryption: true, DedicatedConfigured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	store := &MySQLStore{saasAlertCredentialCipher: manager}
	setting := dashboard.SaaSAlertSetting{TenantID: 10, Channel: dashboard.SaaSAlertNotificationChannelWebhook, WebhookURL: "https://alerts.example.com/hook", WebhookSecret: "secret-value"}
	url, secret, ciphertext, keyID, err := store.encodeSaaSAlertCredentials(setting)
	if err != nil {
		t.Fatal(err)
	}
	if url != "" || secret != "" || keyID != "alert-q3" || ciphertext == "" || strings.Contains(ciphertext, setting.WebhookURL) || strings.Contains(ciphertext, setting.WebhookSecret) {
		t.Fatalf("stored url=%q secret=%q key=%q ciphertext=%q", url, secret, keyID, ciphertext)
	}
	decoded, err := store.decodeSaaSAlertCredentials(dashboard.SaaSAlertSetting{TenantID: 10, Channel: dashboard.SaaSAlertNotificationChannelWebhook}, ciphertext, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.WebhookURL != setting.WebhookURL || decoded.WebhookSecret != setting.WebhookSecret || decoded.WebhookCredentialProtection != dashboard.SaaSAlertCredentialProtectionEncrypted {
		t.Fatalf("decoded=%+v", decoded)
	}
}

func TestSaaSAlertCredentialStorageReadsHistoricalKeyAndLegacyPlaintext(t *testing.T) {
	oldManager, err := saasalertcredentials.NewManager(saasalertcredentials.Config{
		EncryptionKey: "2222222222222222222222222222222222222222222222222222222222222222", EncryptionKeyID: "old",
	})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, keyID, err := oldManager.Encrypt(10, dashboard.SaaSAlertNotificationChannelWebhook, saasalertcredentials.Credential{WebhookURL: "https://old.example/hook", WebhookSecret: "old-secret"})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := saasalertcredentials.NewManager(saasalertcredentials.Config{
		EncryptionKeys:  `{"old":"2222222222222222222222222222222222222222222222222222222222222222","new":"2323232323232323232323232323232323232323232323232323232323232323"}`,
		EncryptionKeyID: "new", RequireEncryption: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	store := &MySQLStore{saasAlertCredentialCipher: manager}
	decoded, err := store.decodeSaaSAlertCredentials(dashboard.SaaSAlertSetting{TenantID: 10, Channel: dashboard.SaaSAlertNotificationChannelWebhook}, ciphertext, keyID)
	if err != nil || decoded.WebhookSecret != "old-secret" || decoded.WebhookCredentialKeyID != "old" {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
	legacy, err := store.decodeSaaSAlertCredentials(dashboard.SaaSAlertSetting{TenantID: 10, Channel: dashboard.SaaSAlertNotificationChannelWebhook, WebhookURL: "https://legacy.example/hook", WebhookSecret: "legacy-secret"}, "", "")
	if err != nil || legacy.WebhookCredentialProtection != dashboard.SaaSAlertCredentialProtectionLegacyPlaintext {
		t.Fatalf("legacy=%+v err=%v", legacy, err)
	}
}

func TestSaaSAlertCredentialStorageRetainsCompatibilityWithoutKey(t *testing.T) {
	store := &MySQLStore{}
	setting := dashboard.SaaSAlertSetting{TenantID: 10, Channel: dashboard.SaaSAlertNotificationChannelWebhook, WebhookURL: "https://legacy.example/hook", WebhookSecret: "legacy-secret"}
	url, secret, ciphertext, keyID, err := store.encodeSaaSAlertCredentials(setting)
	if err != nil || url != setting.WebhookURL || secret != setting.WebhookSecret || ciphertext != "" || keyID != "" {
		t.Fatalf("url=%q secret=%q ciphertext=%q key=%q err=%v", url, secret, ciphertext, keyID, err)
	}
}
