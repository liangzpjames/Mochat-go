package main

import "testing"

func TestAlertCredentialEncryptionDefaultsKeepDomainsIsolated(t *testing.T) {
	clearAlertCredentialEncryptionEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY", "1111111111111111111111111111111111111111111111111111111111111111")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY_ID", "alert-v1")
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS", `{"identity-v1":"2222222222222222222222222222222222222222222222222222222222222222"}`)
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID", "identity-v1")

	selected, dedicated := alertCredentialEncryptionDefaultsFromEnv()
	if !dedicated || selected.Key == "" || selected.Keys != "" || selected.KeyID != "alert-v1" {
		t.Fatalf("dedicated defaults = %+v, configured = %t", selected, dedicated)
	}

	clearAlertCredentialEncryptionEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY", "3333333333333333333333333333333333333333333333333333333333333333")
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID", "identity-v2")
	t.Setenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS", `{"compliance-v1":"4444444444444444444444444444444444444444444444444444444444444444"}`)
	t.Setenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID", "compliance-v1")

	selected, dedicated = alertCredentialEncryptionDefaultsFromEnv()
	if dedicated || selected.Key == "" || selected.Keys != "" || selected.KeyID != "identity-v2" {
		t.Fatalf("fallback defaults = %+v, configured = %t", selected, dedicated)
	}
}

func TestApplyAlertCredentialFlagSelectionClearsFallbackDomain(t *testing.T) {
	options := maintenanceOptions{
		AlertCredentialEncryptionKeys:  `{"identity-v1":"2222222222222222222222222222222222222222222222222222222222222222"}`,
		AlertCredentialEncryptionKeyID: "identity-v1",
	}
	applyAlertCredentialFlagSelection(&options, false, map[string]bool{
		"alert-credential-encryption-key": true,
	})

	if !options.AlertCredentialDedicatedConfigured || options.AlertCredentialEncryptionKeys != "" || options.AlertCredentialEncryptionKeyID != "primary" {
		t.Fatalf("explicit alert credential flags retained fallback domain = %+v", options)
	}
}

func TestWeComCredentialEncryptionDefaultsKeepDomainsIsolated(t *testing.T) {
	clearWeComCredentialEncryptionEnv(t)
	t.Setenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY", "1111111111111111111111111111111111111111111111111111111111111111")
	t.Setenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID", "wecom-v1")
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS", `{"identity-v1":"2222222222222222222222222222222222222222222222222222222222222222"}`)
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID", "identity-v1")

	selected, dedicated := weComCredentialEncryptionDefaultsFromEnv()
	if !dedicated || selected.Key == "" || selected.Keys != "" || selected.KeyID != "wecom-v1" {
		t.Fatalf("dedicated defaults = %+v, configured = %t", selected, dedicated)
	}

	clearWeComCredentialEncryptionEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY", "3333333333333333333333333333333333333333333333333333333333333333")
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID", "identity-v2")
	t.Setenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS", `{"compliance-v1":"4444444444444444444444444444444444444444444444444444444444444444"}`)
	t.Setenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID", "compliance-v1")

	selected, dedicated = weComCredentialEncryptionDefaultsFromEnv()
	if dedicated || selected.Key == "" || selected.Keys != "" || selected.KeyID != "identity-v2" {
		t.Fatalf("fallback defaults = %+v, configured = %t", selected, dedicated)
	}
}

func TestApplyWeComCredentialFlagSelectionClearsFallbackDomain(t *testing.T) {
	options := maintenanceOptions{
		WeComCredentialEncryptionKeys:  `{"identity-v1":"2222222222222222222222222222222222222222222222222222222222222222"}`,
		WeComCredentialEncryptionKeyID: "identity-v1",
	}
	applyWeComCredentialFlagSelection(&options, false, map[string]bool{
		"wecom-credential-encryption-key": true,
	})

	if !options.WeComCredentialDedicatedConfigured || options.WeComCredentialEncryptionKeys != "" || options.WeComCredentialEncryptionKeyID != "primary" {
		t.Fatalf("explicit WeCom credential flags retained fallback domain = %+v", options)
	}
}

func clearAlertCredentialEncryptionEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY",
		"MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEYS",
		"MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY_ID",
		"MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY",
		"MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS",
		"MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID",
		"MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY",
		"MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS",
		"MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID",
		"MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY",
		"MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS",
		"MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID",
	} {
		t.Setenv(name, "")
	}
}

func clearWeComCredentialEncryptionEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY",
		"MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEYS",
		"MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID",
		"MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY",
		"MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS",
		"MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID",
		"MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY",
		"MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS",
		"MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID",
		"MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY",
		"MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS",
		"MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID",
	} {
		t.Setenv(name, "")
	}
}
