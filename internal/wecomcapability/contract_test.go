package wecomcapability

import (
	"reflect"
	"testing"
)

func TestStableCapabilityNames(t *testing.T) {
	want := []string{
		"employee_sync", "department_sync", "external_contact_sync", "contact_tag_sync",
		"room_sync", "contact_way", "welcome_message", "contact_transfer", "agent_message",
		"contact_batch_send", "room_batch_send", "callback",
	}
	if !reflect.DeepEqual(All, want) {
		t.Fatalf("capabilities = %#v, want %#v", All, want)
	}
}
