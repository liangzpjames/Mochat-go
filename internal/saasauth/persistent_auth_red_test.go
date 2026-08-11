package saasauth

import "testing"

func TestSaaSHTTPRequiresPersistentAuthStoreAndEncryptionKey(t *testing.T) {
	config := HTTPConfig{}
	config.Persistence = nil
	config.MFAKey = []byte("01234567890123456789012345678901")
	if _, err := NewHTTPHandler(config); err == nil {
		t.Fatal("SaaS HTTP handler accepted a configuration without durable MFA/session persistence")
	}
}
