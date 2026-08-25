package aiproviderconfig

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
	"time"

	"jiyi/mochat-go/internal/saasbackup"
)

const keyDerivationContext = "mochat-go/ai-provider-credentials/v1"

var keyIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

type envelope struct {
	Version    int    `json:"v"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type Manager struct {
	keys              map[string][]byte
	activeKeyID       string
	requireEncryption bool
}

func NewManager(config Config) (*Manager, error) {
	activeKeyID := strings.TrimSpace(config.EncryptionKeyID)
	if activeKeyID == "" {
		activeKeyID = "primary"
	}
	if !keyIDPattern.MatchString(activeKeyID) {
		return nil, fmt.Errorf("AI provider credential encryption key id %q is invalid", activeKeyID)
	}
	keys, err := saasbackup.ParseEncryptionKeyRing(config.EncryptionKeys)
	if err != nil {
		return nil, fmt.Errorf("AI provider credential encryption key ring: %w", err)
	}
	single, err := saasbackup.ParseEncryptionKey(config.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("AI provider credential encryption key: %w", err)
	}
	if len(single) > 0 {
		if existing, found := keys[activeKeyID]; found && !bytes.Equal(existing, single) {
			return nil, fmt.Errorf("AI provider credential encryption key %q differs between single-key and key-ring configuration", activeKeyID)
		}
		keys[activeKeyID] = append([]byte{}, single...)
	}
	if len(keys) > 0 {
		if _, found := keys[activeKeyID]; !found {
			return nil, fmt.Errorf("AI provider credential active encryption key %q is missing from the key ring", activeKeyID)
		}
	}
	if config.RequireEncryption && len(keys) == 0 {
		return nil, errors.New("AI provider credential encryption is required but no encryption key is configured")
	}
	return &Manager{keys: keys, activeKeyID: activeKeyID, requireEncryption: config.RequireEncryption}, nil
}

func (m *Manager) Encrypt(tenantID int, provider, apiKey string) (string, string, string, error) {
	if m == nil || len(m.keys) == 0 {
		return "", "", "", errors.New("AI provider credential encryption key is not configured")
	}
	provider, err := validateContext(tenantID, provider)
	if err != nil {
		return "", "", "", err
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", "", "", errors.New("AI provider API key is required")
	}
	block, err := aes.NewCipher(deriveKey(m.keys[m.activeKeyID]))
	if err != nil {
		return "", "", "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", "", err
	}
	ciphertext := aead.Seal(nil, nonce, []byte(apiKey), additionalData(m.activeKeyID, tenantID, provider))
	encoded, err := json.Marshal(envelope{
		Version:    1,
		Nonce:      base64.RawURLEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
	})
	if err != nil {
		return "", "", "", err
	}
	return string(encoded), m.activeKeyID, keyHint(apiKey), nil
}

func (m *Manager) Decrypt(tenantID int, provider, keyID, encoded string) (string, error) {
	if m == nil {
		return "", errors.New("AI provider credential encryption manager is not configured")
	}
	provider, err := validateContext(tenantID, provider)
	if err != nil {
		return "", err
	}
	keyID = strings.TrimSpace(keyID)
	key, found := m.keys[keyID]
	if !found {
		return "", fmt.Errorf("AI provider credential encryption key %q is unavailable", keyID)
	}
	var value envelope
	if err := json.Unmarshal([]byte(encoded), &value); err != nil || value.Version != 1 {
		return "", errors.New("AI provider credential ciphertext format is invalid")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(value.Nonce)
	if err != nil {
		return "", errors.New("AI provider credential ciphertext nonce is invalid")
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(value.Ciphertext)
	if err != nil {
		return "", errors.New("AI provider credential ciphertext payload is invalid")
	}
	block, err := aes.NewCipher(deriveKey(key))
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(nonce) != aead.NonceSize() {
		return "", errors.New("AI provider credential ciphertext nonce length is invalid")
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, additionalData(keyID, tenantID, provider))
	if err != nil {
		return "", errors.New("AI provider credential ciphertext authentication failed")
	}
	apiKey := strings.TrimSpace(string(plaintext))
	if apiKey == "" {
		return "", errors.New("AI provider credential plaintext is invalid")
	}
	return apiKey, nil
}

// DecryptStored validates whether a stored tenant configuration is active at
// the supplied time before attempting authenticated decryption. It never
// falls back to a global provider or to plaintext credentials.
func (m *Manager) DecryptStored(config StoredConfig, now time.Time) (string, error) {
	if err := config.validateForUse(now); err != nil {
		return "", err
	}
	return m.Decrypt(config.TenantID, config.Provider, config.KeyID, config.CredentialCiphertext)
}

func validateContext(tenantID int, provider string) (string, error) {
	if tenantID <= 0 {
		return "", errors.New("AI provider tenant id is invalid")
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return "", errors.New("AI provider is required")
	}
	return provider, nil
}

func (config StoredConfig) validateForUse(now time.Time) error {
	if _, err := validateContext(config.TenantID, config.Provider); err != nil {
		return errors.New("AI provider configuration is invalid")
	}
	if strings.TrimSpace(config.Model) == "" || strings.TrimSpace(config.CredentialCiphertext) == "" || !keyIDPattern.MatchString(strings.TrimSpace(config.KeyID)) || config.Version <= 0 {
		return errors.New("AI provider configuration is invalid")
	}
	if config.Status != StatusActive {
		return errors.New("AI provider configuration is inactive")
	}
	if config.EffectiveAt != nil && now.Before(*config.EffectiveAt) {
		return errors.New("AI provider configuration is not yet effective")
	}
	if config.ExpiresAt != nil && !now.Before(*config.ExpiresAt) {
		return errors.New("AI provider configuration has expired")
	}
	return nil
}

func deriveKey(master []byte) []byte {
	hash := hmac.New(sha256.New, master)
	_, _ = hash.Write([]byte(keyDerivationContext))
	return hash.Sum(nil)
}

func additionalData(keyID string, tenantID int, provider string) []byte {
	return []byte(strings.TrimSpace(keyID) + "\x00" + strconv.Itoa(tenantID) + "\x00" + strings.TrimSpace(provider))
}

func keyHint(apiKey string) string {
	runes := []rune(apiKey)
	if len(runes) < 8 {
		return ""
	}
	return string(runes[len(runes)-4:])
}
