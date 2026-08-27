package archivefixture

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const financeFixturePublicKeyVersion uint32 = 1

var errFinanceFixtureAuthentication = errors.New("finance fixture authentication failed")

// EncryptedChatData mirrors the cryptographic fields returned by the WeCom
// Finance SDK GetChatData boundary while retaining the authenticated message
// identity required by the local fixture implementation.
type EncryptedChatData struct {
	Sequence           uint64
	MessageID          string
	PublicKeyVersion   uint32
	EncryptedRandomKey string
	EncryptedMessage   string
}

type FinanceCipher struct {
	corpID     string
	privateKey *rsa.PrivateKey
	version    uint32
}

func NewFinanceCipher(corpID string) (*FinanceCipher, error) {
	corpID = strings.TrimSpace(corpID)
	if corpID == "" {
		return nil, errors.New("finance fixture corp id is required")
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, errors.New("finance fixture key generation failed")
	}
	return &FinanceCipher{corpID: corpID, privateKey: privateKey, version: financeFixturePublicKeyVersion}, nil
}

func (c *FinanceCipher) PublicKeyVersion() uint32 {
	if c == nil {
		return 0
	}
	return c.version
}

func (c *FinanceCipher) PrivateKeyPEM() string {
	if c == nil || c.privateKey == nil {
		return ""
	}
	raw := x509.MarshalPKCS1PrivateKey(c.privateKey)
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: raw}))
}

func (c *FinanceCipher) Encrypt(sequence uint64, messageID string, plaintext []byte) (EncryptedChatData, error) {
	messageID = strings.TrimSpace(messageID)
	if c == nil || c.privateKey == nil || sequence == 0 || messageID == "" {
		return EncryptedChatData{}, errors.New("finance fixture message identity is invalid")
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return EncryptedChatData{}, errors.New("finance fixture key generation failed")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return EncryptedChatData{}, errFinanceFixtureAuthentication
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return EncryptedChatData{}, errFinanceFixtureAuthentication
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return EncryptedChatData{}, errors.New("finance fixture nonce generation failed")
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, c.additionalData(sequence, messageID, c.version))
	wrapped, err := rsa.EncryptPKCS1v15(rand.Reader, &c.privateKey.PublicKey, key)
	if err != nil {
		return EncryptedChatData{}, errors.New("finance fixture key wrapping failed")
	}
	return EncryptedChatData{
		Sequence: sequence, MessageID: messageID, PublicKeyVersion: c.version,
		EncryptedRandomKey: base64.StdEncoding.EncodeToString(wrapped),
		EncryptedMessage:   base64.StdEncoding.EncodeToString(append(nonce, ciphertext...)),
	}, nil
}

func (c *FinanceCipher) Decrypt(value EncryptedChatData) ([]byte, error) {
	if c == nil || c.privateKey == nil || value.PublicKeyVersion != c.version {
		return nil, errFinanceFixtureAuthentication
	}
	wrapped, err := base64.StdEncoding.DecodeString(value.EncryptedRandomKey)
	if err != nil {
		return nil, errFinanceFixtureAuthentication
	}
	key, err := rsa.DecryptPKCS1v15(rand.Reader, c.privateKey, wrapped)
	if err != nil {
		return nil, errFinanceFixtureAuthentication
	}
	return c.DecryptWithRandomKey(key, value)
}

// DecryptWithRandomKey is the local equivalent of the SDK DecryptData call:
// the caller already unwrapped encrypt_random_key with the configured RSA
// private key and supplies only that short-lived key plus the ciphertext.
func (c *FinanceCipher) DecryptWithRandomKey(randomKey []byte, value EncryptedChatData) ([]byte, error) {
	if c == nil || value.Sequence == 0 || strings.TrimSpace(value.MessageID) == "" || value.PublicKeyVersion != c.version || len(randomKey) != 32 {
		return nil, errFinanceFixtureAuthentication
	}
	raw, err := base64.StdEncoding.DecodeString(value.EncryptedMessage)
	if err != nil {
		return nil, errFinanceFixtureAuthentication
	}
	block, err := aes.NewCipher(randomKey)
	if err != nil {
		return nil, errFinanceFixtureAuthentication
	}
	aead, err := cipher.NewGCM(block)
	if err != nil || len(raw) < aead.NonceSize()+aead.Overhead() {
		return nil, errFinanceFixtureAuthentication
	}
	plaintext, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], c.additionalData(value.Sequence, value.MessageID, value.PublicKeyVersion))
	if err != nil {
		return nil, errFinanceFixtureAuthentication
	}
	return plaintext, nil
}

func (c *FinanceCipher) additionalData(sequence uint64, messageID string, version uint32) []byte {
	return []byte(strings.Join([]string{c.corpID, strconv.FormatUint(sequence, 10), strings.TrimSpace(messageID), fmt.Sprint(version)}, "\x00"))
}
