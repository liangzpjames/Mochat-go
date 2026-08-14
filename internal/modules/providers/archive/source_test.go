package archive

import (
	"context"
	"errors"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/modules/providers"
)

func TestSimulationSourceRequiresNamedRunAndExposesIsolatedIdentity(t *testing.T) {
	if _, err := NewSimulationSource(""); err == nil {
		t.Fatal("empty simulation run unexpectedly accepted")
	}

	source, err := NewSimulationSource("acceptance-20260814")
	if err != nil {
		t.Fatal(err)
	}
	if source.Kind() != providers.SourceSimulated {
		t.Fatalf("source kind=%q, want simulated", source.Kind())
	}
	if source.SourceID() != "simulation:acceptance-20260814" {
		t.Fatalf("source id=%q", source.SourceID())
	}
	if source.Namespace() != "MOCHAT-SIM:acceptance-20260814" {
		t.Fatalf("namespace=%q", source.Namespace())
	}
	status := source.Status()
	if status.Source != providers.SourceSimulated || status.State != providers.StateLimited || status.Code != "archive.simulation_ready" {
		t.Fatalf("status=%#v", status)
	}
}

func TestSimulationSourceCoversArchiveTypesGroupAndRevoke(t *testing.T) {
	source, err := NewSimulationSource("coverage-20260814")
	if err != nil {
		t.Fatal(err)
	}
	page, err := source.Fetch(context.Background(), Scope{TenantID: 9, CorpID: 27}, Cursor{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) < 7 {
		t.Fatalf("messages=%d, want representative archive coverage", len(page.Messages))
	}
	seen := map[string]bool{}
	group := false
	revoke := false
	for _, message := range page.Messages {
		seen[message.MsgType] = true
		if message.RoomID != "" {
			group = true
		}
		if message.Action == "revoke" {
			revoke = true
		}
		if message.Source != providers.SourceSimulated || message.SourceID != source.SourceID() || message.Namespace != source.Namespace() {
			t.Fatalf("message source identity=%#v", message)
		}
		if message.MsgID == "" || !strings.HasPrefix(message.MsgID, source.Namespace()) {
			t.Fatalf("message id=%q is outside source namespace", message.MsgID)
		}
	}
	for _, kind := range []string{"text", "image", "file", "voice", "video"} {
		if !seen[kind] {
			t.Fatalf("missing message type %q in %#v", kind, seen)
		}
	}
	if !group || !revoke {
		t.Fatalf("group=%v revoke=%v", group, revoke)
	}
}

func TestSimulationSourcePaginatesByCursorWithoutChangingMessageIDs(t *testing.T) {
	source, err := NewSimulationSource("cursor-20260814")
	if err != nil {
		t.Fatal(err)
	}
	scope := Scope{TenantID: 9, CorpID: 27}
	first, err := source.Fetch(context.Background(), scope, Cursor{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err := source.Fetch(context.Background(), scope, first.NextCursor, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Messages) != 2 || len(second.Messages) == 0 {
		t.Fatalf("first=%d second=%d", len(first.Messages), len(second.Messages))
	}
	if first.Messages[0].MsgID == second.Messages[0].MsgID {
		t.Fatal("cursor page repeated a message")
	}
	if first.Messages[1].Seq >= second.Messages[0].Seq {
		t.Fatalf("cursor order first=%d second=%d", first.Messages[1].Seq, second.Messages[0].Seq)
	}
	replay, err := source.Fetch(context.Background(), scope, Cursor{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Messages[0].MsgID != first.Messages[0].MsgID || replay.Messages[1].MsgID != first.Messages[1].MsgID {
		t.Fatal("replaying a cursor changed deterministic message IDs")
	}
}

func TestArchiveSourcesRejectInvalidScopeAndExternalFetchFailsClosed(t *testing.T) {
	source, err := NewSimulationSource("scope-20260814")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.Fetch(context.Background(), Scope{CorpID: 27}, Cursor{}, 10); !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("invalid scope error=%v, want ErrInvalidScope", err)
	}

	external, err := NewExternalSource("wecom:27", "wecom:27", providers.Status{
		State: providers.StateLimited, Code: "archive.getchatdata_unimplemented", Source: providers.SourceExternal,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := external.Fetch(context.Background(), Scope{TenantID: 9, CorpID: 27}, Cursor{}, 10); !errors.Is(err, providers.ErrCapabilityUnavailable) {
		t.Fatalf("external fetch error=%v, want ErrCapabilityUnavailable", err)
	}
}
