package main

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func TestSimulationCLIRequiresExplicitEnableSwitch(t *testing.T) {
	if err := requireSimulationEnabled(false); err == nil {
		t.Fatal("simulation command unexpectedly enabled without explicit switch")
	}
	if err := requireSimulationEnabled(true); err != nil {
		t.Fatal(err)
	}
}

func TestSimulationCLIRealRunRejectsBeforeSentinelDSN(t *testing.T) {
	t.Setenv("MOCHAT_MYSQL_DSN", "sentinel://must-not-be-opened")
	err := run([]string{"status", "--corp-id", "1"})
	if err == nil || !strings.Contains(err.Error(), "simulation is disabled") {
		t.Fatalf("run() error = %v, want explicit simulation-disabled error", err)
	}
}

func TestSimulationCLIInjectedRunRejectsBeforeReadingDSNOrOpeningDatabase(t *testing.T) {
	readDSN := false
	opened := false
	err := runWith([]string{"status", "--corp-id", "1"}, func(string) string {
		readDSN = true
		return "sentinel://must-not-be-read"
	}, func(string, string) (*sql.DB, error) {
		opened = true
		return nil, errors.New("database must not be opened")
	})
	if err == nil || !strings.Contains(err.Error(), "simulation is disabled") {
		t.Fatalf("runWith() error = %v, want explicit simulation-disabled error", err)
	}
	if readDSN || opened {
		t.Fatalf("disabled simulation touched database dependencies: readDSN=%v opened=%v", readDSN, opened)
	}
}
