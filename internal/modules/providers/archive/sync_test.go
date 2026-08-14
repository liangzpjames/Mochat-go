package archive

import (
	"context"
	"errors"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
)

func TestSyncServicePersistsLifecycleCountsCursorAndIdempotency(t *testing.T) {
	source := &syncTestSource{pages: []Page{{Messages: []Message{
		syncTestMessage(sourceKindSimulated, "simulation:run-1", "MOCHAT-SIM:run-1", 1, "one"),
		syncTestMessage(sourceKindSimulated, "simulation:run-1", "MOCHAT-SIM:run-1", 2, "two"),
	}, NextCursor: Cursor{Sequence: 2}}}}
	store := newSyncTestStore()
	service := NewSyncService(store)
	request := SyncRequest{Scope: Scope{TenantID: 11, CorpID: 27}, IdempotencyKey: "run-1", Limit: 2}

	run, err := service.Sync(context.Background(), source, request)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != SyncStatusSucceeded || run.Counts.Fetched != 2 || run.Counts.Processed != 2 || run.Cursor.Sequence != 2 || run.Attempt != 1 {
		t.Fatalf("run=%#v", run)
	}
	if len(store.lifecycle) != 3 || store.lifecycle[0] != SyncStatusQueued || store.lifecycle[1] != SyncStatusRunning || store.lifecycle[2] != SyncStatusSucceeded {
		t.Fatalf("lifecycle=%v", store.lifecycle)
	}
	if len(store.audits) != 1 || store.audits[0] != "succeeded" {
		t.Fatalf("audits=%v", store.audits)
	}

	replay, err := service.Sync(context.Background(), source, request)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Idempotent || source.calls != 1 || len(store.upserts) != 2 {
		t.Fatalf("replay=%#v sourceCalls=%d upserts=%d", replay, source.calls, len(store.upserts))
	}
}

func TestSyncServiceFailureIsStableAndRetryReusesScopeAndAudit(t *testing.T) {
	source := &syncTestSource{
		status: providers.Status{Source: providers.SourceExternal, Code: "archive.getchatdata_unimplemented", State: providers.StateLimited},
		errors: []error{providers.ErrCapabilityUnavailable},
		pages: []Page{{Messages: []Message{
			syncTestMessage(providers.SourceExternal, "wecom:27", "wecom", 9, "retry"),
		}, NextCursor: Cursor{Sequence: 9}}},
	}
	store := newSyncTestStore()
	service := NewSyncService(store)
	request := SyncRequest{Scope: Scope{TenantID: 11, CorpID: 27}, IdempotencyKey: "external-page-0", Limit: 10}

	failed, err := service.Sync(context.Background(), source, request)
	if err == nil || failed.Status != SyncStatusFailed || ErrorCode(err) != "archive.getchatdata_unimplemented" {
		t.Fatalf("failed run=%#v error=%v code=%q", failed, err, ErrorCode(err))
	}
	if len(store.audits) != 1 || store.audits[0] != "failed:archive.getchatdata_unimplemented" {
		t.Fatalf("failure audits=%v", store.audits)
	}

	retried, err := service.Retry(context.Background(), source, request)
	if err != nil {
		t.Fatal(err)
	}
	if retried.Status != SyncStatusSucceeded || retried.Attempt != 2 || retried.Cursor.Sequence != 9 {
		t.Fatalf("retried=%#v", retried)
	}
	if source.lastScope != request.Scope || len(store.upserts) != 1 {
		t.Fatalf("scope=%#v upserts=%d", source.lastScope, len(store.upserts))
	}
	if len(store.audits) != 2 || store.audits[1] != "succeeded" {
		t.Fatalf("retry audits=%v", store.audits)
	}
}

func TestSyncServiceRejectsInvalidScopeAndMessageSourceMismatch(t *testing.T) {
	source := &syncTestSource{pages: []Page{{Messages: []Message{
		syncTestMessage(providers.SourceExternal, "wecom:27", "wecom", 1, "wrong-source"),
	}}}}
	store := newSyncTestStore()
	service := NewSyncService(store)
	request := SyncRequest{Scope: Scope{TenantID: 11, CorpID: 27}, IdempotencyKey: "scope-check"}

	if _, err := service.Sync(context.Background(), source, SyncRequest{Scope: Scope{CorpID: 27}, IdempotencyKey: "bad"}); ErrorCode(err) != "archive.scope_invalid" {
		t.Fatalf("invalid scope code=%q err=%v", ErrorCode(err), err)
	}
	if _, err := service.Sync(context.Background(), source, request); ErrorCode(err) != "archive.source_identity_mismatch" {
		t.Fatalf("mismatch code=%q err=%v", ErrorCode(err), err)
	}
	if len(store.upserts) != 0 {
		t.Fatalf("mismatched message was persisted: %d", len(store.upserts))
	}
}

const sourceKindSimulated = providers.SourceSimulated

func syncTestMessage(source providers.Source, sourceID, namespace string, seq int64, text string) Message {
	return Message{Source: source, SourceID: sourceID, Namespace: namespace, MsgID: namespace + ":" + text, Seq: seq, MsgType: "text", ContentText: text}
}

type syncTestSource struct {
	status    providers.Status
	pages     []Page
	errors    []error
	calls     int
	lastScope Scope
}

func (s *syncTestSource) Kind() providers.Source {
	if s.status.Source != "" {
		return s.status.Source
	}
	return providers.SourceSimulated
}

func (s *syncTestSource) SourceID() string {
	if s.Kind() == providers.SourceExternal {
		return "wecom:27"
	}
	return "simulation:run-1"
}

func (s *syncTestSource) Namespace() string {
	if s.Kind() == providers.SourceExternal {
		return "wecom"
	}
	return "MOCHAT-SIM:run-1"
}

func (s *syncTestSource) Status() providers.Status { return s.status }

func (s *syncTestSource) Fetch(_ context.Context, scope Scope, _ Cursor, _ int) (Page, error) {
	s.calls++
	s.lastScope = scope
	if len(s.errors) > 0 {
		err := s.errors[0]
		s.errors = s.errors[1:]
		return Page{}, err
	}
	if len(s.pages) == 0 {
		return Page{}, nil
	}
	page := s.pages[0]
	s.pages = s.pages[1:]
	return page, nil
}

type syncTestStore struct {
	runs      map[string]SyncRun
	lifecycle []SyncStatus
	audits    []string
	upserts   []Message
	nextID    int
}

func newSyncTestStore() *syncTestStore { return &syncTestStore{runs: make(map[string]SyncRun)} }

func (s *syncTestStore) EnqueueArchiveSync(_ context.Context, template SyncRun, retryFailed bool) (SyncRun, error) {
	key := template.SourceID + ":" + template.IdempotencyKey
	if existing, ok := s.runs[key]; ok {
		if existing.Status == SyncStatusFailed && retryFailed {
			existing.Status = SyncStatusQueued
			existing.ErrorCode = ""
			existing.Attempt++
			s.runs[key] = existing
			s.lifecycle = append(s.lifecycle, SyncStatusQueued)
			return existing, nil
		}
		return existing, nil
	}
	s.nextID++
	template.ID = "run-" + string(rune('0'+s.nextID))
	template.Status = SyncStatusQueued
	template.Attempt = 1
	s.runs[key] = template
	s.lifecycle = append(s.lifecycle, SyncStatusQueued)
	return template, nil
}

func (s *syncTestStore) MarkArchiveSyncRunning(_ context.Context, id string, _ time.Time) error {
	for key, run := range s.runs {
		if run.ID == id {
			run.Status = SyncStatusRunning
			s.runs[key] = run
		}
	}
	s.lifecycle = append(s.lifecycle, SyncStatusRunning)
	return nil
}

func (s *syncTestStore) UpsertArchiveMessage(_ context.Context, _ string, _ Scope, message Message) (UpsertResult, error) {
	s.upserts = append(s.upserts, message)
	return UpsertResult{Inserted: true}, nil
}

func (s *syncTestStore) SaveArchiveSyncCursor(_ context.Context, _ string, _ Cursor, _ time.Time) error {
	return nil
}

func (s *syncTestStore) CompleteArchiveSync(_ context.Context, id string, counts SyncCounts, cursor Cursor, at time.Time) (SyncRun, error) {
	return s.finish(id, SyncStatusSucceeded, counts, cursor, "", at)
}

func (s *syncTestStore) FailArchiveSync(_ context.Context, id string, counts SyncCounts, cursor Cursor, code string, at time.Time) (SyncRun, error) {
	run, err := s.finish(id, SyncStatusFailed, counts, cursor, code, at)
	if err == nil {
		s.audits = append(s.audits, "failed:"+code)
	}
	return run, err
}

func (s *syncTestStore) finish(id string, status SyncStatus, counts SyncCounts, cursor Cursor, code string, at time.Time) (SyncRun, error) {
	for key, run := range s.runs {
		if run.ID != id {
			continue
		}
		run.Status, run.Counts, run.Cursor, run.ErrorCode, run.FinishedAt = status, counts, cursor, code, &at
		s.runs[key] = run
		s.lifecycle = append(s.lifecycle, status)
		if status == SyncStatusSucceeded {
			s.audits = append(s.audits, "succeeded")
		}
		return run, nil
	}
	return SyncRun{}, errors.New("run not found")
}
