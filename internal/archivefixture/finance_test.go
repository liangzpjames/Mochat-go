package archivefixture

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestFinanceCipherRoundTripUsesDistinctAuthenticatedKeys(t *testing.T) {
	cipher, err := NewFinanceCipher("ww-local-a")
	if err != nil {
		t.Fatal(err)
	}
	first, err := cipher.Encrypt(7, "msg-7", []byte(`{"msgtype":"text","text":{"content":"one"}}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := cipher.Encrypt(8, "msg-8", []byte(`{"msgtype":"text","text":{"content":"two"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if first.EncryptedRandomKey == second.EncryptedRandomKey || first.EncryptedMessage == second.EncryptedMessage {
		t.Fatal("fixture reused encrypted key or ciphertext across messages")
	}
	plain, err := cipher.Decrypt(first)
	if err != nil || !strings.Contains(string(plain), `"content":"one"`) {
		t.Fatalf("round trip plain=%q err=%v", plain, err)
	}
	if _, err := base64.StdEncoding.DecodeString(first.EncryptedRandomKey); err != nil {
		t.Fatalf("encrypted random key is not base64: %v", err)
	}
	if _, err := base64.StdEncoding.DecodeString(first.EncryptedMessage); err != nil {
		t.Fatalf("encrypted message is not base64: %v", err)
	}
}

func TestFinanceCipherRejectsTamperAndBindingMismatchWithoutLeakingMaterial(t *testing.T) {
	cipher, err := NewFinanceCipher("ww-local-a")
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := cipher.Encrypt(7, "msg-7", []byte("fixture-secret-plaintext"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []EncryptedChatData{
		func() EncryptedChatData { value := envelope; value.Sequence++; return value }(),
		func() EncryptedChatData { value := envelope; value.MessageID = "msg-other"; return value }(),
		func() EncryptedChatData { value := envelope; value.PublicKeyVersion++; return value }(),
		func() EncryptedChatData {
			value := envelope
			value.EncryptedMessage = corruptBase64(value.EncryptedMessage)
			return value
		}(),
		func() EncryptedChatData {
			value := envelope
			value.EncryptedRandomKey = corruptBase64(value.EncryptedRandomKey)
			return value
		}(),
	}
	for _, candidate := range cases {
		_, decryptErr := cipher.Decrypt(candidate)
		if decryptErr == nil {
			t.Fatalf("tampered envelope accepted: %+v", candidate)
		}
		if strings.Contains(decryptErr.Error(), "fixture-secret-plaintext") || strings.Contains(decryptErr.Error(), envelope.EncryptedRandomKey) || strings.Contains(decryptErr.Error(), envelope.EncryptedMessage) {
			t.Fatalf("error leaked fixture material: %v", decryptErr)
		}
	}
}

func TestFinanceCipherRejectsCrossCorpAndExportsPrivateKeyForSDKBoundary(t *testing.T) {
	first, err := NewFinanceCipher("ww-local-a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewFinanceCipher("ww-local-b")
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := first.Encrypt(1, "msg-1", []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.Decrypt(envelope); err == nil {
		t.Fatal("cross-corp ciphertext accepted")
	}
	if !strings.Contains(first.PrivateKeyPEM(), "BEGIN RSA PRIVATE KEY") || first.PublicKeyVersion() != 1 {
		t.Fatal("SDK boundary key material is unavailable")
	}
}

func corruptBase64(value string) string {
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(raw) == 0 {
		return value + "!"
	}
	raw[len(raw)-1] ^= 0x01
	return base64.StdEncoding.EncodeToString(raw)
}
