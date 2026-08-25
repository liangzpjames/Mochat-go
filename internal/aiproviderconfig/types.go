package aiproviderconfig

import "time"

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// StoredConfig is the persistence representation of one tenant's AI provider.
// API keys are deliberately excluded; only authenticated ciphertext and its
// non-sensitive last-four-character hint may be stored here.
type StoredConfig struct {
	TenantID             int
	Provider             string
	BaseURL              string
	Model                string
	CredentialCiphertext string
	KeyID                string
	Hint                 string
	EffectiveAt          *time.Time
	ExpiresAt            *time.Time
	Status               string
	Version              int
	CreatedBy            int
	UpdatedBy            int
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Config struct {
	EncryptionKey     string
	EncryptionKeys    string
	EncryptionKeyID   string
	RequireEncryption bool
}
