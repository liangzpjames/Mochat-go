package store

import (
	"database/sql"
	"testing"

	"jiyi/mochat-go/internal/modules/providers"
)

func TestArchiveSourceStatusKeepsSimulationLifecycleTruthful(t *testing.T) {
	for _, test := range []struct {
		name string
		run  string
		code string
	}{
		{name: "queued", run: "queued", code: "archive.simulation_pending"},
		{name: "running", run: "running", code: "archive.simulation_syncing"},
		{name: "succeeded", run: "succeeded", code: "archive.simulation_ready"},
		{name: "failed", run: "failed", code: "archive.simulation_failed"},
		{name: "unknown", run: "paused", code: "archive.simulation_pending"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status := archiveSourceStatusFromRun("simulated", test.run, "", sql.NullTime{}, sql.NullTime{}, sql.NullTime{})
			if status.Source != providers.SourceSimulated || status.State != providers.StateLimited || status.Code != test.code || status.Reason == "" || status.Action == "" {
				t.Fatalf("status=%#v", status)
			}
			if test.run == "queued" && status.Code == "archive.simulation_ready" || test.run == "running" && status.Code == "archive.simulation_ready" {
				t.Fatalf("lifecycle state was promoted to ready: %#v", status)
			}
			if test.run == "failed" && status.LastErrorCode != "archive.sync_failed" {
				t.Fatalf("failed status=%#v", status)
			}
		})
	}
}

func TestArchiveSourceStatusNeverPromotesExternalRunToReady(t *testing.T) {
	status := archiveSourceStatusFromRun("external", "succeeded", "", sql.NullTime{}, sql.NullTime{}, sql.NullTime{})
	if status.Source != providers.SourceExternal || status.State == providers.StateReady || status.Code != "archive.getchatdata_unimplemented" {
		t.Fatalf("status=%#v", status)
	}
}

func TestArchiveStatusSourceKindFollowsCurrentCorpArchiveMode(t *testing.T) {
	for _, test := range []struct {
		mode workMessageArchiveMode
		want providers.Source
	}{
		{mode: workMessageArchiveReal, want: providers.SourceExternal},
		{mode: workMessageArchiveSimulation, want: providers.SourceSimulated},
	} {
		if got := archiveStatusSourceKind(test.mode); got != test.want {
			t.Fatalf("mode=%v source=%q want=%q", test.mode, got, test.want)
		}
	}
}
