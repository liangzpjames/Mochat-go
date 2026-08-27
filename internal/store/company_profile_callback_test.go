package store

import (
	"encoding/base64"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/wecomcredentials"
)

func TestCompanyCallbackConfigurationAcceptsOnlyAlphanumericEncodingAESKey(t *testing.T) {
	if !companyCallbackConfigurationValid("callback-token", strings.Repeat("A", 43)) {
		t.Fatal("43-character alphanumeric EncodingAESKey should be valid")
	}
	for _, invalid := range []string{
		strings.Repeat("A", 42) + "+",
		strings.Repeat("A", 42) + "/",
		strings.Repeat("A", 42) + "_",
	} {
		if companyCallbackConfigurationValid("callback-token", invalid) {
			t.Fatalf("EncodingAESKey %q should be rejected", invalid)
		}
	}
}

func TestCompanyCallbackConfigurationReadTreatsRetiredCredentialAsUnconfigured(t *testing.T) {
	manager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey:   base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")),
		EncryptionKeyID: "current-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	configuration := companyCallbackConfigurationFromCredential(
		&MySQLStore{weComCredentialCipher: manager},
		companyBindingRecord{CorpID: 17, Version: 4},
		corpCredentialRecord{ID: 17, TenantID: 9, WXCorpID: "ww-retired", Ciphertext: "retired", KeyID: "retired-key"},
	)
	if configuration.CorpID != 17 || configuration.BindingVersion != 4 || configuration.Configured || configuration.Token != "" || configuration.EncodingAESKey != "" {
		t.Fatalf("callback repair view = %+v", configuration)
	}
}
