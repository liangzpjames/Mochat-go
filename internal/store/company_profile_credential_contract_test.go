package store

import (
	"strings"
	"testing"

	"jiyi/mochat-go/internal/wecomcredentials"
)

func TestAuthoritativeApplicationAgentSelectionIsActiveAndShared(t *testing.T) {
	selection := authoritativeApplicationAgentSelectionSQL()
	for _, required := range []string{
		"a.close = 0", "a.deleted_at IS NULL", "(a.is_reportenter = 1) DESC",
		"a.updated_at DESC", "a.id ASC",
	} {
		if !strings.Contains(selection, required) {
			t.Fatalf("selection SQL=%q, missing %q", selection, required)
		}
	}
}

func TestCorpCredentialFactsAreCapabilitySpecificAndSecretFree(t *testing.T) {
	employee, contact, callbackToken, callbackAES := deriveCorpCredentialFacts(wecomcredentials.CorpCredential{
		EmployeeSecret: "employee-secret",
	})
	if !employee || contact || callbackToken || callbackAES {
		t.Fatalf("facts employee=%v contact=%v callbackToken=%v callbackAES=%v", employee, contact, callbackToken, callbackAES)
	}
	employee, contact, callbackToken, callbackAES = deriveCorpCredentialFacts(wecomcredentials.CorpCredential{
		ContactSecret: "contact-secret", CallbackToken: "callback-token",
	})
	if employee || !contact || !callbackToken || callbackAES {
		t.Fatalf("facts employee=%v contact=%v callbackToken=%v callbackAES=%v", employee, contact, callbackToken, callbackAES)
	}
}

func TestAgentCredentialFactsFailClosedForIncompleteOrEmptyDecryptedRows(t *testing.T) {
	cases := []struct {
		name       string
		agentID    string
		ciphertext string
		keyID      string
		secret     string
		wantID     bool
		wantSecret bool
	}{
		{name: "id without envelope", agentID: "agent-1", wantID: true},
		{name: "ciphertext without key", agentID: "agent-1", ciphertext: "cipher", wantID: true},
		{name: "key without ciphertext", agentID: "agent-1", keyID: "key", wantID: true},
		{name: "decrypt failure", agentID: "agent-1", ciphertext: "cipher", keyID: "key", wantID: true},
		{name: "empty decrypted secret", agentID: "agent-1", ciphertext: "cipher", keyID: "key", secret: "", wantID: true},
		{name: "complete", agentID: "agent-1", ciphertext: "cipher", keyID: "key", secret: "secret", wantID: true, wantSecret: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			gotID, gotSecret := deriveAgentCredentialFacts(test.agentID, test.ciphertext, test.keyID, test.secret)
			if gotID != test.wantID || gotSecret != test.wantSecret {
				t.Fatalf("facts id=%v secret=%v, want id=%v secret=%v", gotID, gotSecret, test.wantID, test.wantSecret)
			}
		})
	}
}
