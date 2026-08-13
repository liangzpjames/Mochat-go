package store

import (
	"strings"
	"testing"
)

func TestCompanyCallbackConfigurationAcceptsOnlyAlphanumericEncodingAESKey(t *testing.T) {
	if !companyCallbackConfigurationValid("callback-token", strings.Repeat("A", 43)) {
		t.Fatal("43-character alphanumeric EncodingAESKey should be valid")
	}
	for _, invalid := range []string{
		strings.Repeat("A", 42) + "+",
		strings.Repeat("A", 42) + "/",
		strings.Repeat("A", 42) + "_",
	} {
		if companyCallbackConfigurationValid("callback-token", invalid) {
			t.Fatalf("EncodingAESKey %q should be rejected", invalid)
		}
	}
}
