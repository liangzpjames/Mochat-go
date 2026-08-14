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

func TestCompanySyncQueueOnlyTreatsCurrentVersionMarkerAsAlreadyQueued(t *testing.T) {
	for _, test := range []struct {
		name           string
		marker         companySyncStateMarker
		queueTicket    string
		currentVersion uint64
		wantQueued     bool
	}{
		{name: "current queued", marker: companySyncStateMarker{Code: companySyncStateQueued, CredentialVersion: 2}, currentVersion: 2, wantQueued: true},
		{name: "current running", marker: companySyncStateMarker{Code: companySyncStateRunning, CredentialVersion: 2}, currentVersion: 2, wantQueued: true},
		{name: "different ticket", marker: companySyncStateMarker{Code: companySyncStateQueued, CredentialVersion: 2, QueueTicket: "1"}, queueTicket: "2", currentVersion: 2, wantQueued: false},
		{name: "stale queued", marker: companySyncStateMarker{Code: companySyncStateQueued, CredentialVersion: 1}, currentVersion: 2},
		{name: "stale running", marker: companySyncStateMarker{Code: companySyncStateRunning, CredentialVersion: 1}, currentVersion: 2},
		{name: "legacy queued", marker: companySyncStateMarker{Code: companySyncStateQueued}, currentVersion: 2},
		{name: "failed marker", marker: companySyncStateMarker{Code: companySyncStateFailed, CredentialVersion: 2}, currentVersion: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := companySyncMarkerAlreadyQueued(test.marker, test.currentVersion, test.queueTicket); got != test.wantQueued {
				t.Fatalf("marker=%+v current=%d alreadyQueued=%v, want %v", test.marker, test.currentVersion, got, test.wantQueued)
			}
		})
	}
}

func TestCompanySyncQueuePreservesOnlyCompletionAfterRealEnqueueTicket(t *testing.T) {
	cases := []struct {
		name     string
		marker   companySyncStateMarker
		ticket   string
		current  uint64
		wantKeep bool
	}{
		{name: "same ticket completion", marker: companySyncStateMarker{Code: companySyncStateCompleted, CredentialVersion: 2, QueueTicket: "1"}, ticket: "1", current: 2, wantKeep: true},
		{name: "different ticket completion", marker: companySyncStateMarker{Code: companySyncStateCompleted, CredentialVersion: 2, QueueTicket: "2"}, ticket: "1", current: 2},
		{name: "legacy completion has no marker", marker: companySyncStateMarker{}, ticket: "1", current: 2},
		{name: "stale completion", marker: companySyncStateMarker{Code: companySyncStateCompleted, CredentialVersion: 1, QueueTicket: "1"}, ticket: "1", current: 2},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldPreserveCompletedCompanySyncMarker(test.marker, test.ticket, test.current); got != test.wantKeep {
				t.Fatalf("marker=%+v ticket=%s current=%d keep=%v, want %v", test.marker, test.ticket, test.current, got, test.wantKeep)
			}
		})
	}
}

func TestCompanySyncQueueTicketOrderingRejectsDelayedOlderReceipts(t *testing.T) {
	cases := []struct {
		name     string
		existing string
		incoming string
		want     int
		wantErr  bool
	}{
		{name: "older incoming", existing: "2", incoming: "1", want: 1},
		{name: "newer incoming", existing: "1", incoming: "2", want: -1},
		{name: "same numeric ticket", existing: "02", incoming: "2", want: 0},
		{name: "legacy marker is older", existing: "", incoming: "1", want: -1},
		{name: "legacy receipt cannot replace current ticket", existing: "2", incoming: "", wantErr: true},
		{name: "invalid existing ticket fails closed", existing: "ticket-2", incoming: "3", wantErr: true},
		{name: "invalid incoming ticket fails closed", existing: "2", incoming: "ticket-1", wantErr: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := compareCompanySyncQueueTickets(test.existing, test.incoming)
			if test.wantErr {
				if err == nil {
					t.Fatalf("existing=%q incoming=%q returned relation=%d, want fail closed", test.existing, test.incoming, got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("existing=%q incoming=%q relation=%d err=%v, want %d", test.existing, test.incoming, got, err, test.want)
			}
		})
	}
}

func TestCompanySyncQueueTicketParserAcceptsOnlyPositiveDecimalTickets(t *testing.T) {
	for _, ticket := range []string{"", "0", "-1", "ticket-1", "1.0"} {
		if _, err := parseCompanySyncQueueTicket(ticket); err == nil {
			t.Fatalf("ticket %q unexpectedly accepted", ticket)
		}
	}
	for _, ticket := range []string{"1", "02", "18446744073709551615"} {
		if value, err := parseCompanySyncQueueTicket(ticket); err != nil || value == 0 {
			t.Fatalf("ticket %q parse value=%d err=%v", ticket, value, err)
		}
	}
}

func TestCompanySyncStatusTreatsOldLifecycleMarkerAsStale(t *testing.T) {
	for _, status := range []string{"queued", "syncing", "failed"} {
		if !companySyncStatusMarkerStale(status, 0, 2) || !companySyncStatusMarkerStale(status, 1, 2) {
			t.Fatalf("status=%q was not stale for missing/old version", status)
		}
		if companySyncStatusMarkerStale(status, 2, 2) {
			t.Fatalf("status=%q was stale for current version", status)
		}
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

func TestCompanySyncStatusFromRecordRequiresCredentialVersionForCompletedEvidence(t *testing.T) {
	finished := time.Date(2026, 8, 15, 10, 11, 12, 0, time.UTC)
	current := companySyncStatusFromRecord(
		sql.NullTime{Time: finished, Valid: true},
		sql.NullString{String: `{"code":"SYNC_COMPLETED","credentialVersion":7}`, Valid: true},
		3,
		4,
	)
	if current.Status != "completed" || current.CredentialVersion != 7 {
		t.Fatalf("current completed status=%+v", current)
	}
	legacy := companySyncStatusFromRecord(
		sql.NullTime{Time: finished, Valid: true},
		sql.NullString{String: "", Valid: false},
		3,
		4,
	)
	if legacy.Status != "completed" || legacy.CredentialVersion != 0 {
		t.Fatalf("legacy completed status=%+v", legacy)
	}
}
