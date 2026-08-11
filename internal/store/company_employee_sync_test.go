package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCompanySyncRejectsNonUniqueTenantBinding(t *testing.T) {
	_, err := requireSingleCompanySyncBinding([]companySyncBinding{
		{TenantID: 202, CorpID: 303, Status: 2, WXCorpID: "ww-one"},
		{TenantID: 202, CorpID: 404, Status: 2, WXCorpID: "ww-two"},
	})
	if !errors.Is(err, errCompanyBindingNotUnique) {
		t.Fatalf("error = %v, want non-unique binding error", err)
	}
}

func TestCompanySyncRejectsMissingOrUnverifiedBinding(t *testing.T) {
	if _, err := requireSingleCompanySyncBinding(nil); !errors.Is(err, errCompanyBindingNotFound) {
		t.Fatalf("missing error = %v, want not-found binding error", err)
	}
	_, err := requireSingleCompanySyncBinding([]companySyncBinding{{TenantID: 202, CorpID: 303, Status: 1}})
	if !errors.Is(err, errCompanyBindingNotVerified) {
		t.Fatalf("pending error = %v, want unverified binding error", err)
	}
}

func TestCompanySyncStatusFromRecordSanitizesFailure(t *testing.T) {
	finished := time.Date(2026, 8, 12, 10, 11, 12, 0, time.UTC)
	status := companySyncStatusFromRecord(
		sql.NullTime{Time: finished, Valid: true},
		sql.NullString{String: `{"code":"SYNC_FAILED","secret":"must-not-escape"}`, Valid: true},
		3,
		4,
	)
	if status.Status != "failed" || status.ErrorCode != "SYNC_FAILED" {
		t.Fatalf("status=%+v, want sanitized failed status", status)
	}
	raw, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "must-not-escape") || strings.Contains(string(raw), "secret") {
		t.Fatalf("status contains credential material: %s", raw)
	}
}

func TestCompanySyncStatusFromRecordDoesNotExposeUnknownErrorText(t *testing.T) {
	status := companySyncStatusFromRecord(
		sql.NullTime{Valid: false},
		sql.NullString{String: "driver detail with secret=do-not-return", Valid: true},
		0,
		0,
	)
	if status.Status != "failed" || status.ErrorCode != "SYNC_FAILED" {
		t.Fatalf("status=%+v, want stable failed status", status)
	}
	raw, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "driver detail") || strings.Contains(string(raw), "do-not-return") {
		t.Fatalf("status leaked internal error: %s", raw)
	}
}

func TestCompanySyncStatusFromRecordDistinguishesQueueLifecycle(t *testing.T) {
	queued := companySyncStatusFromRecord(sql.NullTime{}, sql.NullString{String: `{"code":"SYNC_QUEUED","cursor":"company-sync"}`, Valid: true}, 0, 0)
	if queued.Status != "queued" || queued.Cursor != "company-sync" || queued.ErrorCode != "" {
		t.Fatalf("queued status=%+v", queued)
	}
	queuedAfterRetry := companySyncStatusFromRecord(sql.NullTime{}, sql.NullString{String: `{"code":"SYNC_QUEUED","cursor":"company-sync","errorCode":"SYNC_FAILED"}`, Valid: true}, 0, 0)
	if queuedAfterRetry.Status != "queued" || queuedAfterRetry.Cursor != "company-sync" || queuedAfterRetry.ErrorCode != "SYNC_FAILED" {
		t.Fatalf("queued-after-retry status=%+v", queuedAfterRetry)
	}
	running := companySyncStatusFromRecord(sql.NullTime{}, sql.NullString{String: `{"code":"SYNC_RUNNING","cursor":"company-sync"}`, Valid: true}, 0, 0)
	if running.Status != "syncing" || running.Cursor != "company-sync" {
		t.Fatalf("running status=%+v", running)
	}
	unknown := companySyncStatusFromRecord(sql.NullTime{}, sql.NullString{String: `{"code":"SYNC_RUNNING","cursor":"secret-job-id"}`, Valid: true}, 0, 0)
	if unknown.Status != "syncing" || unknown.Cursor != "" {
		t.Fatalf("unknown cursor status=%+v", unknown)
	}
}
