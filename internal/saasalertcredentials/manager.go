package saasalertcredentials

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/saasbackup"
)

var keyIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

type Config struct {
	EncryptionKey       string
	EncryptionKeys      string
	EncryptionKeyID     string
	RequireEncryption   bool
	DedicatedConfigured bool
}

type Credential struct {
	WebhookURL    string `json:"webhookUrl"`
	WebhookSecret string `json:"webhookSecret"`
}

type ConfigStatus struct {
	EncryptionConfigured bool   `json:"encryptionConfigured"`
	RequireEncryption    bool   `json:"requireEncryption"`
	DedicatedConfigured  bool   `json:"dedicatedConfigured"`
	ActiveKeyID          string `json:"activeKeyId"`
	KeyCount             int    `json:"keyCount"`
}

type envelope struct {
	Version    int    `json:"v"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type Manager struct {
	keys                map[string][]byte
	activeKeyID         string
	requireEncryption   bool
	dedicatedConfigured bool
}

func NewManager(config Config) (*Manager, error) {
	activeKeyID := strings.TrimSpace(config.EncryptionKeyID)
	if activeKeyID == "" {
		activeKeyID = "primary"
	}
	if !keyIDPattern.MatchString(activeKeyID) {
		return nil, fmt.Errorf("SaaS alert credential encryption key id %q is invalid", activeKeyID)
	}
	keys, err := saasbackup.ParseEncryptionKeyRing(config.EncryptionKeys)
	if err != nil {
		return nil, fmt.Errorf("SaaS alert credential encryption key ring: %w", err)
	}
	single, err := saasbackup.ParseEncryptionKey(config.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("SaaS alert credential encryption key: %w", err)
	}
	if len(single) > 0 {
		if existing, found := keys[activeKeyID]; found && !bytes.Equal(existing, single) {
			return nil, fmt.Errorf("SaaS alert credential encryption key %q differs between single-key and key-ring configuration", activeKeyID)
		}
		keys[activeKeyID] = append([]byte{}, single...)
	}
	if len(keys) > 0 {
		if _, found := keys[activeKeyID]; !found {
			return nil, fmt.Errorf("SaaS alert credential active encryption key %q is missing from the key ring", activeKeyID)
		}
	}
	if config.RequireEncryption && len(keys) == 0 {
		return nil, errors.New("SaaS alert credential encryption is required but no encryption key is configured")
	}
	return &Manager{
		keys:                keys,
		activeKeyID:         activeKeyID,
		requireEncryption:   config.RequireEncryption,
		dedicatedConfigured: config.DedicatedConfigured,
	}, nil
}

func (m *Manager) ConfigStatus() ConfigStatus {
	if m == nil {
		return ConfigStatus{ActiveKeyID: "primary"}
	}
	return ConfigStatus{
		EncryptionConfigured: len(m.keys) > 0,
		RequireEncryption:    m.requireEncryption,
		DedicatedConfigured:  m.dedicatedConfigured,
		ActiveKeyID:          m.activeKeyID,
		KeyCount:             len(m.keys),
	}
}

func (m *Manager) HasKey(keyID string) bool {
	if m == nil {
		return false
	}
	_, found := m.keys[strings.TrimSpace(keyID)]
	return found
}

func (m *Manager) Encrypt(tenantID int, channel string, credential Credential) (string, string, error) {
	if m == nil || len(m.keys) == 0 {
		return "", "", errors.New("SaaS alert credential encryption key is not configured")
	}
	key := m.keys[m.activeKeyID]
	plaintext, err := json.Marshal(Credential{
		WebhookURL:    strings.TrimSpace(credential.WebhookURL),
		WebhookSecret: strings.TrimSpace(credential.WebhookSecret),
	})
	if err != nil {
		return "", "", err
	}
	block, err := aes.NewCipher(deriveKey(key))
	if err != nil {
		return "", "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", err
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, additionalData(m.activeKeyID, tenantID, channel))
	encoded, err := json.Marshal(envelope{
		Version:    1,
		Nonce:      base64.RawURLEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
	})
	if err != nil {
		return "", "", err
	}
	return string(encoded), m.activeKeyID, nil
}

func (m *Manager) Decrypt(tenantID int, channel string, keyID string, encoded string) (Credential, error) {
	if m == nil {
		return Credential{}, errors.New("SaaS alert credential encryption manager is not configured")
	}
	keyID = strings.TrimSpace(keyID)
	key, found := m.keys[keyID]
	if !found {
		return Credential{}, fmt.Errorf("SaaS alert credential encryption key %q is unavailable", keyID)
	}
	var value envelope
	if err := json.Unmarshal([]byte(encoded), &value); err != nil || value.Version != 1 {
		return Credential{}, errors.New("SaaS alert credential ciphertext format is invalid")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(value.Nonce)
	if err != nil {
		return Credential{}, errors.New("SaaS alert credential ciphertext nonce is invalid")
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(value.Ciphertext)
	if err != nil {
		return Credential{}, errors.New("SaaS alert credential ciphertext payload is invalid")
	}
	block, err := aes.NewCipher(deriveKey(key))
	if err != nil {
		return Credential{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return Credential{}, err
	}
	if len(nonce) != aead.NonceSize() {
		return Credential{}, errors.New("SaaS alert credential ciphertext nonce length is invalid")
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, additionalData(keyID, tenantID, channel))
	if err != nil {
		return Credential{}, errors.New("SaaS alert credential ciphertext authentication failed")
	}
	var credential Credential
	if err := json.Unmarshal(plaintext, &credential); err != nil {
		return Credential{}, errors.New("SaaS alert credential plaintext format is invalid")
	}
	credential.WebhookURL = strings.TrimSpace(credential.WebhookURL)
	credential.WebhookSecret = strings.TrimSpace(credential.WebhookSecret)
	return credential, nil
}

func deriveKey(master []byte) []byte {
	hash := hmac.New(sha256.New, master)
	_, _ = hash.Write([]byte("mochat-go/saas-alert-credentials/v1"))
	return hash.Sum(nil)
}

func additionalData(keyID string, tenantID int, channel string) []byte {
	return []byte(strings.TrimSpace(keyID) + "\x00" + strconv.Itoa(tenantID) + "\x00" + strings.TrimSpace(channel))
}
