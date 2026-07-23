package dashboard

import "context"

type SaaSWeChatOpenCredentialProtectionStatus struct {
	EncryptionConfigured      bool     `json:"encryptionConfigured"`
	RequireEncryption         bool     `json:"requireEncryption"`
	DedicatedConfigured       bool     `json:"dedicatedConfigured"`
	ActiveKeyID               string   `json:"activeKeyId"`
	KeyCount                  int      `json:"keyCount"`
	ConfiguredCredentialCount int      `json:"configuredCredentialCount"`
	ComponentTicketCount      int      `json:"componentTicketCount"`
	OfficialAccountCount      int      `json:"officialAccountCount"`
	EncryptedCredentialCount  int      `json:"encryptedCredentialCount"`
	LegacyPlaintextCount      int      `json:"legacyPlaintextCount"`
	ActiveKeyCredentialCount  int      `json:"activeKeyCredentialCount"`
	RotationRequiredCount     int      `json:"rotationRequiredCount"`
	UnavailableKeyCount       int      `json:"unavailableKeyCount"`
	DecryptFailureCount       int      `json:"decryptFailureCount"`
	UnavailableKeyIDs         []string `json:"unavailableKeyIds"`
	Healthy                   bool     `json:"healthy"`
}

type SaaSWeChatOpenCredentialRotationResult struct {
	TenantID                    int    `json:"tenantId"`
	Limit                       int    `json:"limit"`
	ScannedCount                int    `json:"scannedCount"`
	RotatedCount                int    `json:"rotatedCount"`
	ComponentTicketRotatedCount int    `json:"componentTicketRotatedCount"`
	OfficialAccountRotatedCount int    `json:"officialAccountRotatedCount"`
	LegacyCount                 int    `json:"legacyCount"`
	ReencryptedCount            int    `json:"reencryptedCount"`
	ActiveKeyID                 string `json:"activeKeyId"`
}

type SaaSWeChatOpenCredentialProtectionStore interface {
	SaaSWeChatOpenCredentialProtection(ctx context.Context) (SaaSWeChatOpenCredentialProtectionStatus, error)
	RotateSaaSWeChatOpenCredentials(ctx context.Context, tenantID int, limit int) (SaaSWeChatOpenCredentialRotationResult, error)
}
