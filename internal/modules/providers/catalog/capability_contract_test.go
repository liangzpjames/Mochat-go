package catalog

import (
	"reflect"
	"testing"

	"jiyi/mochat-go/internal/modules/providers"
	"jiyi/mochat-go/internal/wecomcapability"
)

func TestWeComStandardRegistrationClassifiesEveryStableCapability(t *testing.T) {
	registry, err := NewRegistry(Dependencies{
		AI:            compositionStatusProvider{status: providers.Status{State: providers.StateLimited}},
		AIEnabled:     true,
		Archive:       compositionStatusProvider{status: providers.Status{State: providers.StateLimited}},
		AudioStorage:  compositionStatusProvider{status: providers.Status{State: providers.StateLimited}},
		WeComStandard: compositionStatusProvider{status: providers.Status{State: providers.StateLimited}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range registry.Snapshot(nil) {
		if status.Kind != "wecom_standard" {
			continue
		}
		if !reflect.DeepEqual(status.Capabilities, wecomcapability.All) {
			t.Fatalf("wecom_standard capabilities=%#v, want %#v", status.Capabilities, wecomcapability.All)
		}
		return
	}
	t.Fatal("wecom_standard registration missing")
}
