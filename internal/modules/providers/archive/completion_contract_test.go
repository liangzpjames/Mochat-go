package archive

import (
	"context"
	"testing"

	"jiyi/mochat-go/internal/modules/providers"
)

// This is deliberately a bad fixture: a simulated source that emits an
// external message must fail the runtime boundary, even though its type names
// and registration-looking strings would otherwise appear valid to a text
// scanner.
func TestArchiveCompletionContractRejectsBadSourceFixture(t *testing.T) {
	store := newSyncTestStore()
	service := NewSyncService(store)
	_, err := service.Sync(context.Background(), badArchiveSourceFixture{}, SyncRequest{
		Scope: Scope{TenantID: 11, CorpID: 27}, IdempotencyKey: "bad-source-fixture", Limit: 1,
	})
	if ErrorCode(err) != "archive.source_identity_mismatch" {
		t.Fatalf("bad source fixture error code=%q err=%v", ErrorCode(err), err)
	}
}

type badArchiveSourceFixture struct{}

func (badArchiveSourceFixture) Kind() providers.Source { return providers.SourceSimulated }
func (badArchiveSourceFixture) SourceID() string       { return "simulation:bad-fixture" }
func (badArchiveSourceFixture) Namespace() string      { return "MOCHAT-SIM:bad-fixture" }
func (badArchiveSourceFixture) Status() providers.Status {
	return providers.Status{Kind: "wecom_archive", Source: providers.SourceSimulated, State: providers.StateLimited, Code: "archive.simulation_ready"}
}
func (s badArchiveSourceFixture) Fetch(context.Context, Scope, Cursor, int) (Page, error) {
	return Page{Messages: []Message{{
		Source: providers.SourceExternal, SourceID: s.SourceID(), Namespace: s.Namespace(), MsgID: "MOCHAT-SIM:bad-fixture:1", Seq: 1,
	}}}, nil
}
