package wecom

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/modules/providers"
)

func TestArchiveStatusAndSync(t *testing.T) {
	archive, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if status := archive.Status(); status.State != providers.StateLimited {
		t.Fatalf("status = %#v, want limited", status)
	} else if status.Code != "archive.credentials_missing" || status.Source != providers.SourceExternal {
		t.Fatalf("status = %#v, want missing-credentials external status", status)
	}
	if _, err := archive.Sync(context.Background(), providers.SyncOptions{CorpID: 1}); !errors.Is(err, providers.ErrNotConfigured) {
		t.Fatalf("Sync error = %v, want ErrNotConfigured", err)
	}
	archive, err = New(Config{CorpID: "corp-secret-value", Secret: "archive-secret-value", PublicKey: "public-key-value", PrivateKey: "private-key-value"})
	if err != nil {
		t.Fatal(err)
	}
	status := archive.Status()
	if status.State != providers.StateLimited || status.Code != "archive.getchatdata_unimplemented" || status.Source != providers.SourceExternal {
		t.Fatalf("status = %#v, want truthful limited status", status)
	}
	for _, secret := range []string{"corp-secret-value", "archive-secret-value", "public-key-value", "private-key-value"} {
		if strings.Contains(status.Reason, secret) {
			t.Fatalf("status reason contains credential material %q: %q", secret, status.Reason)
		}
	}
	if _, err := archive.Sync(context.Background(), providers.SyncOptions{CorpID: 1}); !errors.Is(err, providers.ErrCapabilityUnavailable) {
		t.Fatalf("Sync error = %v, want ErrCapabilityUnavailable", err)
	}
}

func TestAESCBCRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("企微会话存档内容 AES 解密契约测试")
	ciphertext, err := EncryptAESCBC(key, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := DecryptAESCBC(key, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatal("AES roundtrip mismatch")
	}
}

func TestRSADecryptRoundTrip(t *testing.T) {
	publicPEM, privatePEM, err := GenerateRSAKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	archive, err := New(Config{CorpID: "c", Secret: "s", PublicKey: publicPEM, PrivateKey: privatePEM})
	if err != nil {
		t.Fatal(err)
	}
	// Encrypt with the public key via x509 + rsa.EncryptOAEP, then decrypt through the adapter helper.
	payload := []byte("session-key-fixture")
	encoded := encryptWithPublicKey(t, archive.publicKey, payload)
	decrypted, err := RSADecrypt(archive.privateKey, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decrypted, payload) {
		t.Fatal("RSA roundtrip mismatch")
	}
}

func encryptWithPublicKey(t *testing.T, publicPEM string, payload []byte) string {
	t.Helper()
	block, _ := pem.Decode([]byte(publicPEM))
	if block == nil {
		t.Fatal("invalid public key")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, ok := key.(*rsa.PublicKey)
	if !ok {
		t.Fatal("public key is not RSA")
	}
	encrypted, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, payload, nil)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(encrypted)
}
