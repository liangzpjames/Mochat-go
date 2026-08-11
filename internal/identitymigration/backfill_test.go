package identitymigration

import "testing"

func TestCredentialRowsRequireLegacyPlaintextBeforeWritingCiphertext(t *testing.T) {
	if hasCorpCredentialPlaintext("", "", "", "", "") {
		t.Fatal("empty corp credential row must remain unencrypted")
	}
	if !hasCorpCredentialPlaintext("employee", "", "", "", "") {
		t.Fatal("non-empty employee secret must be encrypted")
	}
	if !hasCorpCredentialPlaintext("", "", "", "", "chat") {
		t.Fatal("non-empty chat secret must be encrypted")
	}
	if hasAgentCredentialPlaintext("") {
		t.Fatal("empty agent secret must remain unencrypted")
	}
	if !hasAgentCredentialPlaintext("agent") {
		t.Fatal("non-empty agent secret must be encrypted")
	}
}
