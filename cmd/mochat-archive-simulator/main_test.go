package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/archivebridge"

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

func TestEnsureFixtureSourceOwnershipRejectsAnotherDatasetOnSameBinding(t *testing.T) {
	for _, test := range []struct {
		name string
		rows *sqlmock.Rows
		ok   bool
	}{
		{name: "empty", rows: sqlmock.NewRows([]string{"msgid"}), ok: true},
		{name: "same", rows: sqlmock.NewRows([]string{"msgid"}).AddRow("MOCHAT-LOCAL-SIM-owned-self-0001"), ok: true},
		{name: "other", rows: sqlmock.NewRows([]string{"msgid"}).AddRow("MOCHAT-LOCAL-SIM-other-self-0001")},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			binding := archivebridge.Binding{TenantID: 11, CorpID: 27, WXCorpID: "ww-local", IntegrationMode: archivebridge.ModeSelfBuilt}
			mock.ExpectQuery("(?s)SELECT msgid.*FROM mochat_go_archive_message_sources.*source_id=\\?.*namespace=\\?").WithArgs(int64(11), int64(27), "wecom:self_built:ww-local", "wecom:self_built:ww-local").WillReturnRows(test.rows)
			err = ensureFixtureSourceOwnership(context.Background(), db, binding, "MOCHAT-LOCAL-SIM-owned")
			if (err == nil) != test.ok {
				t.Fatalf("err=%v ok=%v", err, test.ok)
			}
		})
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

func TestPrepareDelegatedFixtureAuthorizationIsNoOpWhenCredentialIsReady(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	binding := archivebridge.Binding{TenantID: 4, CorpID: 4, WXCorpID: "wwMOCHATLOCALSIM00000004", IntegrationMode: archivebridge.ModeThirdPartyDelegated}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE mc_corp SET wx_corpid").WithArgs(binding.WXCorpID, binding.CorpID, binding.TenantID).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("(?s)UPDATE mochat_go_wecom_integrations SET provider_app_id=.*credential_ciphertext=''").WithArgs(binding.TenantID, binding.CorpID).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("(?s)SELECT COUNT.*FROM mochat_go_wecom_integrations").WithArgs(binding.TenantID, binding.CorpID).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectCommit()
	if err := prepareDelegatedFixtureAuthorization(context.Background(), db, binding); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFixtureParticipantIdentitiesAreDatasetScoped(t *testing.T) {
	staff, external := fixtureParticipantIdentities("MOCHAT-LOCAL-SIM-scope")
	if staff != "MOCHAT-LOCAL-SIM-scope-STAFF-01" || external != "MOCHAT-LOCAL-SIM-scope-EXTERNAL-01" {
		t.Fatalf("staff=%q external=%q", staff, external)
	}
}

func TestFixtureActivationUsesIntegrityAwareAuditStore(t *testing.T) {
	body, err := os.ReadFile("activation.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if strings.Contains(source, "INSERT INTO mochat_go_saas_admin_operation_logs") {
		t.Fatal("fixture activation bypasses the SaaS audit integrity chain")
	}
	if !strings.Contains(source, "RecordSaaSAdminOperationLog") {
		t.Fatal("fixture activation does not record an integrity-aware audit event")
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

func TestReadFixtureFileRequiresResolvedPathInsideInputRoot(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "sample.png")
	if err := os.WriteFile(inside, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, name, err := readFixtureFile(inside, root, "file"); err != nil || name != "sample.png" {
		t.Fatalf("inside name=%q err=%v", name, err)
	}
	outsideRoot := t.TempDir()
	outside := filepath.Join(outsideRoot, "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := readFixtureFile(outside, root, "file"); err == nil {
		t.Fatal("outside fixture path unexpectedly accepted")
	}
}

func TestValidateSendArgumentsRejectsAmbiguousOrMismatchedInputs(t *testing.T) {
	for _, input := range []struct{ kind, text, file string }{
		{kind: "text", text: "hello", file: "/fixtures/input/image.png"},
		{kind: "image", text: "ignored", file: "/fixtures/input/image.png"},
		{kind: "future", text: "hello"},
	} {
		if err := validateSendArguments(input.kind, input.text, input.file); err == nil {
			t.Fatalf("input=%+v unexpectedly accepted", input)
		}
	}
	if err := validateSendArguments("text", "hello", ""); err != nil {
		t.Fatal(err)
	}
	if err := validateSendArguments("voice", "", "/fixtures/input/voice.wav"); err != nil {
		t.Fatal(err)
	}
}

func TestDispatchFixtureCallbacksPreservesOrderAndEncryptedEnvelope(t *testing.T) {
	seen := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPost || r.URL.Query().Get("msg_signature") == "" || !strings.Contains(string(body), "<Encrypt>") {
			t.Fatalf("invalid callback request url=%s body=%s", r.URL.String(), body)
		}
		seen = append(seen, r.URL.Query().Get("nonce"))
		_, _ = w.Write([]byte("success"))
	}))
	defer server.Close()
	callbacks := []archivebridge.FixtureCallback{
		{EventType: "suite_ticket", Timestamp: "1", Nonce: "ticket", Signature: "sig-1", Encrypted: "cipher-1"},
		{EventType: "create_auth", Timestamp: "2", Nonce: "auth", Signature: "sig-2", Encrypted: "cipher-2"},
	}
	if err := dispatchFixtureCallbacks(context.Background(), server.Client(), server.URL, callbacks); err != nil {
		t.Fatal(err)
	}
	if strings.Join(seen, ",") != "ticket,auth" {
		t.Fatalf("callback order=%v", seen)
	}
}

func TestFixtureSeedSummaryDoesNotExposeCallbackEnvelope(t *testing.T) {
	summary := safeFixtureBridgeSummary(0, []archivebridge.FixtureCallback{{
		EventType: "create_auth", Timestamp: "1", Nonce: "secret-nonce", Signature: "secret-signature", Encrypted: "secret-ciphertext",
	}})
	raw, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, secret := range []string{"secret-nonce", "secret-signature", "secret-ciphertext"} {
		if strings.Contains(text, secret) {
			t.Fatalf("seed summary exposed callback envelope: %s", text)
		}
	}
	if !strings.Contains(text, "create_auth") {
		t.Fatalf("seed summary omitted safe event evidence: %s", text)
	}
}

func TestFixtureMediaCleanupStagesRestoresAndPurgesRecoverably(t *testing.T) {
	root := t.TempDir()
	archiveRoot := filepath.Join(root, "archive-media")
	if err := os.MkdirAll(archiveRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	const mediaID = "014c1da7-1b2e-4aa1-90aa-a6a0d6f53380"
	paths := []string{filepath.Join(archiveRoot, mediaID), filepath.Join(archiveRoot, mediaID+".attempt-1.part")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("fixture-media"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	staged, err := stageFixtureMediaFiles(root, "MOCHAT-LOCAL-SIM-clean", []string{mediaID})
	if err != nil || len(staged.Files) != 2 {
		t.Fatalf("staged=%+v err=%v", staged, err)
	}
	for _, path := range paths {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("original still exists: %s err=%v", path, err)
		}
	}
	if err := restoreFixtureMediaFiles(staged); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("original was not restored: %s err=%v", path, err)
		}
	}
	staged, err = stageFixtureMediaFiles(root, "MOCHAT-LOCAL-SIM-clean", []string{mediaID})
	if err != nil {
		t.Fatal(err)
	}
	if removed, err := purgeFixtureMediaFiles(staged); err != nil || removed != 2 {
		t.Fatalf("removed=%d err=%v", removed, err)
	}
}
