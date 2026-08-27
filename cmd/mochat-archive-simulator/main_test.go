package main

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSimulationCLIRequiresExplicitEnableSwitch(t *testing.T) {
	if err := requireSimulationEnabled(false); err == nil {
		t.Fatal("simulation command unexpectedly enabled without explicit switch")
	}
	if err := requireSimulationEnabled(true); err != nil {
		t.Fatal(err)
	}
}

func TestResolveBindingUsesAuthoritativeActiveCurrentSlot(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta("FROM mochat_go_wecom_integrations integration") + ".*" + regexp.QuoteMeta("integration.slot='current'") + ".*" + regexp.QuoteMeta("integration.status='active'") + ".*" + regexp.QuoteMeta("integration.mode=binding.wecom_integration_mode")).
		WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "corp_id", "verified_wx_corpid", "wecom_integration_mode"}).AddRow(11, 27, "ww-safe", "self_built"))
	binding, err := resolveBinding(context.Background(), db, 11, "self_built")
	if err != nil || binding.TenantID != 11 || binding.CorpID != 27 || binding.WXCorpID != "ww-safe" {
		t.Fatalf("binding=%+v err=%v", binding, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFixtureCleanupRejectsAnyMessageOutsideConfirmedDataset(t *testing.T) {
	if err := validateFixtureCleanupMessageIDs("MOCHAT-LOCAL-SIM-clean", []string{"MOCHAT-LOCAL-SIM-clean-MSG-01", "MOCHAT-LOCAL-SIM-clean-DELEGATED-02"}); err != nil {
		t.Fatal(err)
	}
	if err := validateFixtureCleanupMessageIDs("MOCHAT-LOCAL-SIM-clean", []string{"MOCHAT-LOCAL-SIM-clean-MSG-01", "production-message"}); err == nil {
		t.Fatal("cleanup accepted a production message in the same source identity")
	}
}

func TestLocalFixtureWXCorpIDIsDeterministicAndRejectsProductionIdentity(t *testing.T) {
	if got := localFixtureWXCorpID(42); got != "wwMOCHATLOCALSIM00000042" {
		t.Fatalf("local fixture corp id=%q", got)
	}
	if !fixtureWXCorpIDAllowed(42, "") || !fixtureWXCorpIDAllowed(42, "wwSIM00000000000001") || !fixtureWXCorpIDAllowed(42, "wwMOCHATLOCALSIM00000042") {
		t.Fatal("fixture identity should allow empty and visibly local corp ids")
	}
	if fixtureWXCorpIDAllowed(42, "wwProductionCorp123") {
		t.Fatal("fixture identity accepted a production-looking corp id")
	}
}

func TestFixtureIntegrationAlreadyActiveRequiresCompleteLocalContractState(t *testing.T) {
	if !fixtureIntegrationAlreadyActive(
		"active",
		"wwMOCHATLOCALSIM00000042",
		"local_contract",
		[]string{"contacts.read", "archive.read"},
		2,
		"wwMOCHATLOCALSIM00000042",
		"wwMOCHATLOCALSIM00000042",
		1,
	) {
		t.Fatal("complete local-contract state should be idempotent")
	}
	if fixtureIntegrationAlreadyActive("active", "wwMOCHATLOCALSIM00000042", "local_contract", []string{"contacts.read"}, 2, "wwMOCHATLOCALSIM00000042", "wwMOCHATLOCALSIM00000042", 1) {
		t.Fatal("state without archive.read must be repaired")
	}
	if fixtureIntegrationAlreadyActive("active", "wwMOCHATLOCALSIM00000042", "local_contract", []string{"archive.read"}, 1, "wwMOCHATLOCALSIM00000042", "wwMOCHATLOCALSIM00000042", 1) {
		t.Fatal("unverified binding must be repaired")
	}
}

func TestFixtureParticipantIdentitiesAreDatasetScoped(t *testing.T) {
	staff, external := fixtureParticipantIdentities("MOCHAT-LOCAL-SIM-scope")
	if staff != "MOCHAT-LOCAL-SIM-scope-STAFF-01" || external != "MOCHAT-LOCAL-SIM-scope-EXTERNAL-01" {
		t.Fatalf("staff=%q external=%q", staff, external)
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
