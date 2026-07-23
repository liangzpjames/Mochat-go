package wechatopencredentials

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

type TicketCredential struct {
	ComponentVerifyTicket string `json:"componentVerifyTicket"`
}

type OfficialAccountCredential struct {
	ComponentSecret        string `json:"componentSecret"`
	ComponentToken         string `json:"componentToken"`
	ComponentAESKey        string `json:"componentAesKey"`
	AuthorizationCode      string `json:"authorizationCode"`
	PreAuthCode            string `json:"preAuthCode"`
	AuthorizerRefreshToken string `json:"authorizerRefreshToken"`
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
		return nil, fmt.Errorf("WeChat Open credential encryption key id %q is invalid", activeKeyID)
	}
	keys, err := saasbackup.ParseEncryptionKeyRing(config.EncryptionKeys)
	if err != nil {
		return nil, fmt.Errorf("WeChat Open credential encryption key ring: %w", err)
	}
	single, err := saasbackup.ParseEncryptionKey(config.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("WeChat Open credential encryption key: %w", err)
	}
	if len(single) > 0 {
		if existing, found := keys[activeKeyID]; found && !bytes.Equal(existing, single) {
			return nil, fmt.Errorf("WeChat Open credential encryption key %q differs between single-key and key-ring configuration", activeKeyID)
		}
		keys[activeKeyID] = append([]byte{}, single...)
	}
	if len(keys) > 0 {
		if _, found := keys[activeKeyID]; !found {
			return nil, fmt.Errorf("WeChat Open credential active encryption key %q is missing from the key ring", activeKeyID)
		}
	}
	if config.RequireEncryption && len(keys) == 0 {
		return nil, errors.New("WeChat Open credential encryption is required but no encryption key is configured")
	}
	return &Manager{
		keys: keys, activeKeyID: activeKeyID, requireEncryption: config.RequireEncryption,
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

func (m *Manager) EncryptTicket(componentAppID string, credential TicketCredential) (string, string, error) {
	credential.ComponentVerifyTicket = strings.TrimSpace(credential.ComponentVerifyTicket)
	return m.encrypt("component-ticket", 0, componentAppID, credential)
}

func (m *Manager) DecryptTicket(componentAppID string, keyID string, encoded string) (TicketCredential, error) {
	var credential TicketCredential
	if err := m.decrypt("component-ticket", 0, componentAppID, keyID, encoded, &credential); err != nil {
		return TicketCredential{}, err
	}
	credential.ComponentVerifyTicket = strings.TrimSpace(credential.ComponentVerifyTicket)
	return credential, nil
}

func (m *Manager) EncryptOfficialAccount(tenantID int, authorizerAppID string, credential OfficialAccountCredential) (string, string, error) {
	credential = normalizeOfficialAccountCredential(credential)
	return m.encrypt("official-account", tenantID, authorizerAppID, credential)
}

func (m *Manager) DecryptOfficialAccount(tenantID int, authorizerAppID string, keyID string, encoded string) (OfficialAccountCredential, error) {
	var credential OfficialAccountCredential
	if err := m.decrypt("official-account", tenantID, authorizerAppID, keyID, encoded, &credential); err != nil {
		return OfficialAccountCredential{}, err
	}
	return normalizeOfficialAccountCredential(credential), nil
}

func (m *Manager) encrypt(resource string, ownerID int, identifier string, value any) (string, string, error) {
	if m == nil || len(m.keys) == 0 {
		return "", "", errors.New("WeChat Open credential encryption key is not configured")
	}
	plaintext, err := json.Marshal(value)
	if err != nil {
		return "", "", err
	}
	block, err := aes.NewCipher(deriveKey(m.keys[m.activeKeyID]))
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
	ciphertext := aead.Seal(nil, nonce, plaintext, additionalData(m.activeKeyID, resource, ownerID, identifier))
	encoded, err := json.Marshal(envelope{Version: 1, Nonce: base64.RawURLEncoding.EncodeToString(nonce), Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext)})
	if err != nil {
		return "", "", err
	}
	return string(encoded), m.activeKeyID, nil
}

func (m *Manager) decrypt(resource string, ownerID int, identifier string, keyID string, encoded string, target any) error {
	if m == nil {
		return errors.New("WeChat Open credential encryption manager is not configured")
	}
	keyID = strings.TrimSpace(keyID)
	key, found := m.keys[keyID]
	if !found {
		return fmt.Errorf("WeChat Open credential encryption key %q is unavailable", keyID)
	}
	var value envelope
	if err := json.Unmarshal([]byte(encoded), &value); err != nil || value.Version != 1 {
		return errors.New("WeChat Open credential ciphertext format is invalid")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(value.Nonce)
	if err != nil {
		return errors.New("WeChat Open credential ciphertext nonce is invalid")
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(value.Ciphertext)
	if err != nil {
		return errors.New("WeChat Open credential ciphertext payload is invalid")
	}
	block, err := aes.NewCipher(deriveKey(key))
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if len(nonce) != aead.NonceSize() {
		return errors.New("WeChat Open credential ciphertext nonce length is invalid")
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, additionalData(keyID, resource, ownerID, identifier))
	if err != nil {
		return errors.New("WeChat Open credential ciphertext authentication failed")
	}
	if err := json.Unmarshal(plaintext, target); err != nil {
		return errors.New("WeChat Open credential plaintext format is invalid")
	}
	return nil
}

func normalizeOfficialAccountCredential(value OfficialAccountCredential) OfficialAccountCredential {
	value.ComponentSecret = strings.TrimSpace(value.ComponentSecret)
	value.ComponentToken = strings.TrimSpace(value.ComponentToken)
	value.ComponentAESKey = strings.TrimSpace(value.ComponentAESKey)
	value.AuthorizationCode = strings.TrimSpace(value.AuthorizationCode)
	value.PreAuthCode = strings.TrimSpace(value.PreAuthCode)
	value.AuthorizerRefreshToken = strings.TrimSpace(value.AuthorizerRefreshToken)
	return value
}

func deriveKey(master []byte) []byte {
	hash := hmac.New(sha256.New, master)
	_, _ = hash.Write([]byte("mochat-go/wechat-open-credentials/v1"))
	return hash.Sum(nil)
}

func additionalData(keyID string, resource string, ownerID int, identifier string) []byte {
	return []byte(strings.TrimSpace(keyID) + "\x00" + strings.TrimSpace(resource) + "\x00" + strconv.Itoa(ownerID) + "\x00" + strings.TrimSpace(identifier))
}
