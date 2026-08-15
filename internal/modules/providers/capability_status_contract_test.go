package providers

import (
	"encoding/json"
	"testing"
)

func TestCapabilityStatusIsStructuredAndSecretFree(t *testing.T) {
	status := Status{
		Kind: "wecom_standard", State: StateLimited, Source: SourceExternal,
		CapabilityStatuses: []CapabilityStatus{{
			Capability: "contact_batch_send", State: StateLimited, Code: "wecom.capability_operation_pending",
			Source: SourceExternal, Missing: []string{"contact_secret"}, Reason: "missing contact credential",
			LastErrorCode: "wecom.safe_error",
		}},
	}
	payload, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) == "" || len(status.CapabilityStatuses) != 1 {
		t.Fatalf("capability status not serialized: %s", payload)
	}
	for _, forbidden := range []string{"plaintext-secret-sentinel", "access_token", "private_key"} {
		if len(payload) > 0 && string(payload) == forbidden {
			t.Fatalf("payload contains forbidden material %q: %s", forbidden, payload)
		}
	}
}
