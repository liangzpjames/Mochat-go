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

func TestArchiveSourceStatusReflectsExternalBridgeLifecycle(t *testing.T) {
	for _, test := range []struct {
		run   string
		state providers.State
		code  string
	}{
		{run: "succeeded", state: providers.StateReady, code: "archive.bridge_ready"},
		{run: "queued", state: providers.StateLimited, code: "archive.bridge_pending"},
		{run: "running", state: providers.StateLimited, code: "archive.bridge_syncing"},
		{run: "failed", state: providers.StateUnavailable, code: "archive.bridge_failed"},
	} {
		status := archiveSourceStatusFromRun("external", test.run, "archive.upstream_failed", sql.NullTime{}, sql.NullTime{}, sql.NullTime{})
		if status.Source != providers.SourceExternal || status.State != test.state || status.Code != test.code || status.Reason == "" || status.Action == "" {
			t.Fatalf("run=%s status=%#v", test.run, status)
		}
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
