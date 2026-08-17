package wecomarchivedemo

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"testing"
)

const callbackTestAESKey = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

func TestVerifyAndDecryptCallbackAcceptsValidCiphertext(t *testing.T) {
	token := "callback-token"
	encrypted := encryptCallbackFixture(t, callbackTestAESKey, []byte("verified"), "ww-test")
	values := url.Values{"timestamp": {"1710000000"}, "nonce": {"nonce"}}
	values.Set("msg_signature", callbackSignatureFixture(token, values.Get("timestamp"), values.Get("nonce"), encrypted))

	got, err := VerifyAndDecryptCallback(token, callbackTestAESKey, "ww-test", values, encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Message) != "verified" || got.ReceiveID != "ww-test" {
		t.Fatalf("plaintext = %+v", got)
	}
}

func TestVerifyAndDecryptCallbackRejectsTamperingAndReceiverMismatch(t *testing.T) {
	token := "callback-token"
	encrypted := encryptCallbackFixture(t, callbackTestAESKey, []byte("verified"), "ww-test")
	values := url.Values{"timestamp": {"1710000000"}, "nonce": {"nonce"}, "msg_signature": {"bad"}}
	if _, err := VerifyAndDecryptCallback(token, callbackTestAESKey, "ww-test", values, encrypted); err == nil {
		t.Fatal("tampered signature was accepted")
	}
	values.Set("msg_signature", callbackSignatureFixture(token, values.Get("timestamp"), values.Get("nonce"), encrypted))
	if _, err := VerifyAndDecryptCallback(token, callbackTestAESKey, "ww-other", values, encrypted); err == nil {
		t.Fatal("receiver mismatch was accepted")
	}
}

func encryptCallbackFixture(t *testing.T, encodingAESKey string, message []byte, receiveID string) string {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		t.Fatal(err)
	}
	plain := bytes.NewBuffer([]byte("abcdefghijklmnop"))
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(message)))
	plain.Write(size[:])
	plain.Write(message)
	plain.WriteString(receiveID)
	padding := 32 - plain.Len()%32
	padded := append(plain.Bytes(), bytes.Repeat([]byte{byte(padding)}, padding)...)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(out, padded)
	return base64.StdEncoding.EncodeToString(out)
}

func callbackSignatureFixture(token, timestamp, nonce, encrypted string) string {
	items := []string{token, timestamp, nonce, encrypted}
	sort.Strings(items)
	sum := sha1.Sum([]byte(strings.Join(items, "")))
	return hex.EncodeToString(sum[:])
}
