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
	}
	if _, err := archive.Sync(context.Background(), providers.SyncOptions{CorpID: 1}); err == nil {
		t.Fatal("Sync error = nil, want ErrNotConfigured")
	}
	archive, err = New(Config{CorpID: "c", Secret: "s", PublicKey: "p", PrivateKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if status := archive.Status(); status.State != providers.StateReady {
		t.Fatalf("status = %#v, want ready", status)
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
