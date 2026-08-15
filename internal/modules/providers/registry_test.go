package providers

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type registryTestProvider struct {
	status Status
}

func (p registryTestProvider) Status() Status { return p.status }

func TestRegistryRejectsUnclassifiedSource(t *testing.T) {
	registry := NewRegistry()
	err := registry.Register(Registration{
		Kind:         "wecom_standard",
		Source:       Source("fixture"),
		Capabilities: []string{"employee_sync"},
		Provider:     registryTestProvider{status: Status{State: StateLimited}},
	})
	if !errors.Is(err, ErrInvalidRegistration) {
		t.Fatalf("Register error = %v, want ErrInvalidRegistration", err)
	}
}

func TestRegistryRejectsDuplicateKind(t *testing.T) {
	registry := NewRegistry()
	registration := Registration{
		Kind:         "wecom_standard",
		Source:       SourceExternal,
		Capabilities: []string{"employee_sync"},
		Provider:     registryTestProvider{status: Status{State: StateLimited}},
	}
	if err := registry.Register(registration); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(registration); !errors.Is(err, ErrInvalidRegistration) {
		t.Fatalf("duplicate Register error = %v, want ErrInvalidRegistration", err)
	}
}

func TestRegistrySnapshotIsSortedAndDefensive(t *testing.T) {
	registry := NewRegistry()
	for _, registration := range []Registration{
		{Kind: "z-last", Source: SourceLocal, Capabilities: []string{"storage"}, Provider: registryTestProvider{status: Status{State: StateReady}}},
		{Kind: "a-first", Source: SourceCodeOnly, Capabilities: []string{"chat"}, Provider: registryTestProvider{status: Status{State: StateLimited, Code: "ai.key_missing"}}},
	} {
		if err := registry.Register(registration); err != nil {
			t.Fatal(err)
		}
	}

	first := registry.Snapshot(context.Background())
	if got := []string{first[0].Kind, first[1].Kind}; !reflect.DeepEqual(got, []string{"a-first", "z-last"}) {
		t.Fatalf("snapshot kinds = %#v, want sorted kinds", got)
	}
	first[0].Capabilities[0] = "mutated"
	first[0].State = StateUnavailable
	second := registry.Snapshot(context.Background())
	if second[0].Capabilities[0] != "chat" || second[0].State != StateLimited {
		t.Fatalf("registry snapshot was not defensive: %#v", second[0])
	}
}

func TestStatusJSONDoesNotExposeSecretMaterial(t *testing.T) {
	status := Status{
		Kind:          "wecom_standard",
		State:         StateLimited,
		Code:          "wecom.credentials_missing",
		Source:        SourceExternal,
		Reason:        "缺少企业微信凭据",
		Action:        "请在企业设置中配置凭据",
		Capabilities:  []string{"employee_sync"},
		Missing:       []string{"employee_secret"},
		LastErrorCode: "wecom.external_failed",
	}
	payload, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(payload)
	for _, forbidden := range []string{"plaintext-secret", "access_token", "private_key"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("status JSON contains forbidden material %q: %s", forbidden, encoded)
		}
	}
}
