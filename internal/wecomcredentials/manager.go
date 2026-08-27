package wecomcredentials

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

type CorpCredential struct {
	EmployeeSecret       string `json:"employeeSecret"`
	ContactSecret        string `json:"contactSecret"`
	CallbackToken        string `json:"callbackToken"`
	EncodingAESKey       string `json:"encodingAesKey"`
	ChatSecret           string `json:"chatSecret"`
	ArchiveRSAPublicKey  string `json:"archiveRsaPublicKey,omitempty"`
	ArchiveRSAPrivateKey string `json:"archiveRsaPrivateKey,omitempty"`
}

type AgentCredential struct {
	WXSecret string `json:"wxSecret"`
}

// AuthorizationCredential stores the encrypted credentials for one tenant's
// selected WeCom integration. It is deliberately separate from the legacy
// corp and agent credentials so self-built and third-party credentials cannot
// be substituted for each other.
type AuthorizationCredential struct {
	Mode           string `json:"mode"`
	EmployeeSecret string `json:"employeeSecret,omitempty"`
	ContactSecret  string `json:"contactSecret,omitempty"`
	AgentSecret    string `json:"agentSecret,omitempty"`
	ChatSecret     string `json:"chatSecret,omitempty"`
	ProviderAppID  string `json:"providerAppId,omitempty"`
	PermanentCode  string `json:"permanentCode,omitempty"`
}

// ArchiveMediaCredential protects the opaque Finance SDK locator at rest.
// The media object UUID is part of the AES-GCM additional data, so ciphertext
// cannot be moved between tenants or media objects.
type ArchiveMediaCredential struct {
	SDKFileID string `json:"sdkFileId"`
}

// ArchiveComponentCredential protects the data-zone display locator. It is
// decrypted only by the authenticated component-session endpoint.
type ArchiveComponentCredential struct {
	MessageID          string `json:"messageId"`
	PublicKeyVersion   uint32 `json:"publicKeyVersion"`
	EncryptedSecretKey string `json:"encryptedSecretKey"`
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
		return nil, fmt.Errorf("WeCom credential encryption key id %q is invalid", activeKeyID)
	}
	keys, err := saasbackup.ParseEncryptionKeyRing(config.EncryptionKeys)
	if err != nil {
		return nil, fmt.Errorf("WeCom credential encryption key ring: %w", err)
	}
	single, err := saasbackup.ParseEncryptionKey(config.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("WeCom credential encryption key: %w", err)
	}
	if len(single) > 0 {
		if existing, found := keys[activeKeyID]; found && !bytes.Equal(existing, single) {
			return nil, fmt.Errorf("WeCom credential encryption key %q differs between single-key and key-ring configuration", activeKeyID)
		}
		keys[activeKeyID] = append([]byte{}, single...)
	}
	if len(keys) > 0 {
		if _, found := keys[activeKeyID]; !found {
			return nil, fmt.Errorf("WeCom credential active encryption key %q is missing from the key ring", activeKeyID)
		}
	}
	if config.RequireEncryption && len(keys) == 0 {
		return nil, errors.New("WeCom credential encryption is required but no encryption key is configured")
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

func (m *Manager) EncryptCorp(tenantID int, wxCorpID string, credential CorpCredential) (string, string, error) {
	credential = normalizeCorpCredential(credential)
	return m.encrypt("corp", tenantID, wxCorpID, credential)
}

func (m *Manager) DecryptCorp(tenantID int, wxCorpID string, keyID string, encoded string) (CorpCredential, error) {
	var credential CorpCredential
	if err := m.decrypt("corp", tenantID, wxCorpID, keyID, encoded, &credential); err != nil {
		return CorpCredential{}, err
	}
	return normalizeCorpCredential(credential), nil
}

func (m *Manager) EncryptAgent(corpID int, wxAgentID string, credential AgentCredential) (string, string, error) {
	credential.WXSecret = strings.TrimSpace(credential.WXSecret)
	return m.encrypt("agent", corpID, wxAgentID, credential)
}

func (m *Manager) DecryptAgent(corpID int, wxAgentID string, keyID string, encoded string) (AgentCredential, error) {
	var credential AgentCredential
	if err := m.decrypt("agent", corpID, wxAgentID, keyID, encoded, &credential); err != nil {
		return AgentCredential{}, err
	}
	credential.WXSecret = strings.TrimSpace(credential.WXSecret)
	return credential, nil
}

// EncryptAuthorization encrypts third-party or self-built integration
// credentials. The integration resource type and both tenant and integration
// IDs are authenticated as AES-GCM additional data.
func (m *Manager) EncryptAuthorization(tenantID int, integrationID string, value AuthorizationCredential) (ciphertext, keyID string, err error) {
	return m.encrypt("integration", tenantID, integrationID, value)
}

// DecryptAuthorization decrypts credentials only when the tenant and
// integration IDs match those used when the encrypted value was created.
func (m *Manager) DecryptAuthorization(tenantID int, integrationID, keyID, ciphertext string) (AuthorizationCredential, error) {
	var credential AuthorizationCredential
	if err := m.decrypt("integration", tenantID, integrationID, keyID, ciphertext, &credential); err != nil {
		return AuthorizationCredential{}, err
	}
	return credential, nil
}

func (m *Manager) EncryptArchiveMedia(tenantID int, mediaObjectID string, value ArchiveMediaCredential) (ciphertext, keyID string, err error) {
	value.SDKFileID = strings.TrimSpace(value.SDKFileID)
	if value.SDKFileID == "" {
		return "", "", errors.New("archive media SDK file id is required")
	}
	return m.encrypt("archive_media", tenantID, mediaObjectID, value)
}

func (m *Manager) DecryptArchiveMedia(tenantID int, mediaObjectID, keyID, ciphertext string) (ArchiveMediaCredential, error) {
	var credential ArchiveMediaCredential
	if err := m.decrypt("archive_media", tenantID, mediaObjectID, keyID, ciphertext, &credential); err != nil {
		return ArchiveMediaCredential{}, err
	}
	credential.SDKFileID = strings.TrimSpace(credential.SDKFileID)
	if credential.SDKFileID == "" {
		return ArchiveMediaCredential{}, errors.New("archive media SDK file id is missing")
	}
	return credential, nil
}

func (m *Manager) EncryptArchiveComponent(tenantID int, objectID string, value ArchiveComponentCredential) (ciphertext, keyID string, err error) {
	value.MessageID = strings.TrimSpace(value.MessageID)
	value.EncryptedSecretKey = strings.TrimSpace(value.EncryptedSecretKey)
	if value.MessageID == "" || value.PublicKeyVersion == 0 || value.EncryptedSecretKey == "" {
		return "", "", errors.New("archive component locator is invalid")
	}
	return m.encrypt("archive_component", tenantID, objectID, value)
}

func (m *Manager) DecryptArchiveComponent(tenantID int, objectID, keyID, ciphertext string) (ArchiveComponentCredential, error) {
	var credential ArchiveComponentCredential
	if err := m.decrypt("archive_component", tenantID, objectID, keyID, ciphertext, &credential); err != nil {
		return ArchiveComponentCredential{}, err
	}
	credential.MessageID = strings.TrimSpace(credential.MessageID)
	credential.EncryptedSecretKey = strings.TrimSpace(credential.EncryptedSecretKey)
	if credential.MessageID == "" || credential.PublicKeyVersion == 0 || credential.EncryptedSecretKey == "" {
		return ArchiveComponentCredential{}, errors.New("archive component locator is missing")
	}
	return credential, nil
}

func (m *Manager) encrypt(resource string, ownerID int, identifier string, value any) (string, string, error) {
	if m == nil || len(m.keys) == 0 {
		return "", "", errors.New("WeCom credential encryption key is not configured")
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

func (m *Manager) decrypt(resource string, ownerID int, identifier string, keyID string, encoded string, target any) error {
	if m == nil {
		return errors.New("WeCom credential encryption manager is not configured")
	}
	keyID = strings.TrimSpace(keyID)
	key, found := m.keys[keyID]
	if !found {
		return fmt.Errorf("WeCom credential encryption key %q is unavailable", keyID)
	}
	var value envelope
	if err := json.Unmarshal([]byte(encoded), &value); err != nil || value.Version != 1 {
		return errors.New("WeCom credential ciphertext format is invalid")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(value.Nonce)
	if err != nil {
		return errors.New("WeCom credential ciphertext nonce is invalid")
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(value.Ciphertext)
	if err != nil {
		return errors.New("WeCom credential ciphertext payload is invalid")
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
		return errors.New("WeCom credential ciphertext nonce length is invalid")
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, additionalData(keyID, resource, ownerID, identifier))
	if err != nil {
		return errors.New("WeCom credential ciphertext authentication failed")
	}
	if err := json.Unmarshal(plaintext, target); err != nil {
		return errors.New("WeCom credential plaintext format is invalid")
	}
	return nil
}

func normalizeCorpCredential(value CorpCredential) CorpCredential {
	value.EmployeeSecret = strings.TrimSpace(value.EmployeeSecret)
	value.ContactSecret = strings.TrimSpace(value.ContactSecret)
	value.CallbackToken = strings.TrimSpace(value.CallbackToken)
	value.EncodingAESKey = strings.TrimSpace(value.EncodingAESKey)
	value.ChatSecret = strings.TrimSpace(value.ChatSecret)
	value.ArchiveRSAPublicKey = strings.TrimSpace(value.ArchiveRSAPublicKey)
	value.ArchiveRSAPrivateKey = strings.TrimSpace(value.ArchiveRSAPrivateKey)
	return value
}

func deriveKey(master []byte) []byte {
	hash := hmac.New(sha256.New, master)
	_, _ = hash.Write([]byte("mochat-go/wecom-credentials/v1"))
	return hash.Sum(nil)
}

func additionalData(keyID string, resource string, ownerID int, identifier string) []byte {
	return []byte(strings.TrimSpace(keyID) + "\x00" + strings.TrimSpace(resource) + "\x00" + strconv.Itoa(ownerID) + "\x00" + strings.TrimSpace(identifier))
}
