package store

import (
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

func TestTenantAIProviderStoreInputRejectsMissingInitialKey(t *testing.T) {
	input := dashboard.SaaSTenantAIProviderInput{
		TenantID: 9, ProviderCode: "openai", BaseURL: "https://api.example.test/v1", Model: "model-v1",
		EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active", Version: 0,
	}
	if err := dashboard.ValidateSaaSTenantAIProviderCreate(input); err == nil {
		t.Fatal("first configuration without API key was accepted")
	}
}
