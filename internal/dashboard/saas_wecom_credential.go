package dashboard

import "context"

type SaaSWeComCredentialProtectionStatus struct {
	EncryptionConfigured      bool     `json:"encryptionConfigured"`
	RequireEncryption         bool     `json:"requireEncryption"`
	DedicatedConfigured       bool     `json:"dedicatedConfigured"`
	ActiveKeyID               string   `json:"activeKeyId"`
	KeyCount                  int      `json:"keyCount"`
	ConfiguredCredentialCount int      `json:"configuredCredentialCount"`
	CorpCredentialCount       int      `json:"corpCredentialCount"`
	AgentCredentialCount      int      `json:"agentCredentialCount"`
	EncryptedCredentialCount  int      `json:"encryptedCredentialCount"`
	LegacyPlaintextCount      int      `json:"legacyPlaintextCount"`
	ActiveKeyCredentialCount  int      `json:"activeKeyCredentialCount"`
	RotationRequiredCount     int      `json:"rotationRequiredCount"`
	UnavailableKeyCount       int      `json:"unavailableKeyCount"`
	DecryptFailureCount       int      `json:"decryptFailureCount"`
	UnavailableKeyIDs         []string `json:"unavailableKeyIds"`
	Healthy                   bool     `json:"healthy"`
}

type SaaSWeComCredentialRotationResult struct {
	TenantID          int    `json:"tenantId"`
	Limit             int    `json:"limit"`
	ScannedCount      int    `json:"scannedCount"`
	RotatedCount      int    `json:"rotatedCount"`
	CorpRotatedCount  int    `json:"corpRotatedCount"`
	AgentRotatedCount int    `json:"agentRotatedCount"`
	LegacyCount       int    `json:"legacyCount"`
	ReencryptedCount  int    `json:"reencryptedCount"`
	ActiveKeyID       string `json:"activeKeyId"`
}

type SaaSWeComCredentialProtectionStore interface {
	SaaSWeComCredentialProtection(ctx context.Context) (SaaSWeComCredentialProtectionStatus, error)
	RotateSaaSWeComCredentials(ctx context.Context, tenantID int, limit int) (SaaSWeComCredentialRotationResult, error)
}
