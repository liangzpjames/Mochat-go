package store

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardadmin"
	"jiyi/mochat-go/internal/wecomcredentials"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestWeComIntegrationGenerationIsStrictlyMonotonic(t *testing.T) {
	if got := nextWeComIntegrationGeneration(9, 12); got != 13 {
		t.Fatalf("generation=%d, want 13", got)
	}
	if got := nextWeComIntegrationGeneration(15, 2); got != 16 {
		t.Fatalf("generation=%d, want 16", got)
	}
}

func TestWeComIntegrationSwapFailsClosedBeforeGenerationChange(t *testing.T) {
	binding := weComBinding{CorpID: 63, WXCorpID: "ww-authoritative"}
	current := dashboardadmin.WeComIntegration{ID: "current", Slot: "current", Status: "active", Generation: 9, Version: 3}
	verified := dashboardadmin.WeComIntegration{ID: "candidate", Slot: "candidate", Status: "active", VerifiedWXCorpID: "ww-authoritative", VerifiedAt: "2026-08-27T00:00:00Z", VerificationLevel: dashboardadmin.WeComVerificationLocalContract, Generation: 12, Version: 3}
	tests := []struct {
		name       string
		candidate  dashboardadmin.WeComIntegration
		version    uint64
		leases     int
		decryptErr error
		want       error
	}{
		{"version conflict", verified, 4, 0, nil, dashboardadmin.ErrVersionConflict},
		{"not verified", func() dashboardadmin.WeComIntegration { v := verified; v.Status = "failed"; return v }(), 3, 0, nil, dashboardadmin.ErrWeComCandidateNotVerified},
		{"empty verification level", func() dashboardadmin.WeComIntegration { v := verified; v.VerificationLevel = ""; return v }(), 3, 0, nil, dashboardadmin.ErrWeComCandidateNotVerified},
		{"online verification level", func() dashboardadmin.WeComIntegration { v := verified; v.VerificationLevel = "online"; return v }(), 3, 0, nil, dashboardadmin.ErrWeComCandidateNotVerified},
		{"unknown verification level", func() dashboardadmin.WeComIntegration { v := verified; v.VerificationLevel = "fixture"; return v }(), 3, 0, nil, dashboardadmin.ErrWeComCandidateNotVerified},
		{"corp mismatch", func() dashboardadmin.WeComIntegration { v := verified; v.VerifiedWXCorpID = "ww-other"; return v }(), 3, 0, nil, dashboardadmin.ErrWeComCorpMismatch},
		{"missing scope", func() dashboardadmin.WeComIntegration {
			v := verified
			v.MissingCapabilities = []string{"archive.read"}
			return v
		}(), 3, 0, nil, dashboardadmin.ErrWeComMissingCapabilities},
		{"decrypt failure", verified, 3, 0, errors.New("cipher detail must not escape"), dashboardadmin.ErrWeComCredentialDecrypt},
		{"active lease", verified, 3, 1, nil, dashboardadmin.ErrWeComActiveMediaLease},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			generation, err := validateWeComIntegrationSwap(binding, current, tc.candidate, tc.version, tc.leases, tc.decryptErr)
			if !errors.Is(err, tc.want) || generation != 0 {
				t.Fatalf("generation=%d err=%v want=%v", generation, err, tc.want)
			}
		})
	}
	generation, err := validateWeComIntegrationSwap(binding, current, verified, 3, 0, nil)
	if err != nil || generation != 13 {
		t.Fatalf("generation=%d err=%v", generation, err)
	}
}

func TestWeComIntegrationCandidateVersionGuardLocksPromotedTarget(t *testing.T) {
	binding := weComBinding{CorpID: 63, WXCorpID: "ww-authoritative"}
	current := dashboardadmin.WeComIntegration{ID: "current", Slot: "current", Status: "active", Generation: 1, Version: 1}
	verifiedCandidate := dashboardadmin.WeComIntegration{ID: "candidate", Slot: "candidate", Status: "active", VerifiedWXCorpID: "ww-authoritative", VerifiedAt: "2026-08-27T00:00:00Z", VerificationLevel: dashboardadmin.WeComVerificationLocalContract, Generation: 1, Version: 2}
	if generation, err := validateWeComIntegrationSwap(binding, current, verifiedCandidate, 2, 0, nil); err != nil || generation != 2 {
		t.Fatalf("save candidate v1 -> verify v2 -> switch v2: generation=%d err=%v", generation, err)
	}
	if _, err := validateWeComIntegrationSwap(binding, current, verifiedCandidate, 1, 0, nil); !errors.Is(err, dashboardadmin.ErrVersionConflict) {
		t.Fatalf("stale candidate version accepted: %v", err)
	}
}

func TestWeComIntegrationCandidateVersionFlowSaveV1VerifyV2SwitchV2(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey:   base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)),
		EncryptionKeyID: "test-v1", RequireEncryption: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, keyID, err := manager.EncryptAuthorization(41, "candidate", wecomcredentials.AuthorizationCredential{Mode: "self_built", EmployeeSecret: "employee-secret"})
	if err != nil {
		t.Fatal(err)
	}
	store := NewMySQLStore(db).WithWeComCredentialCipher(manager)
	actor := dashboardadmin.Actor{UserID: 700, Active: true}
	bindingRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"corp_id", "wx_corpid"}).AddRow(63, "ww-authoritative")
	}
	integrationRows := func(slot, status, verificationLevel string, version uint64, cipher, key string) *sqlmock.Rows {
		verifiedAt := any(nil)
		if verificationLevel != "" {
			verifiedAt = time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
		}
		return sqlmock.NewRows([]string{"id", "mode", "slot", "status", "verified_wx_corpid", "agent_id", "provider_app_id", "credential_ciphertext", "credential_key_id", "credential_hint", "scope_json", "scope_digest", "missing_capabilities_json", "generation", "version", "verification_level", "verified_at", "last_error_code", "updated_at"}).AddRow(map[bool]string{true: "candidate", false: "current"}[slot == "candidate"], "self_built", slot, status, map[bool]string{true: "ww-authoritative", false: ""}[verificationLevel != ""], "100001", "", cipher, key, "employee", `[]`, dashboardadmin.WeComScopeDigest(nil), `[]`, 1, version, verificationLevel, verifiedAt, "", time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC))
	}
	expectAuthBinding := func(permission string) {
		mock.ExpectQuery("SELECT u.id FROM mochat_go_saas_admin_users").WithArgs(700, permission).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(700))
		mock.ExpectQuery("SELECT b.corp_id").WithArgs(41).WillReturnRows(bindingRows())
	}
	expectLegacyAudit := func() {
		mock.ExpectExec("INSERT IGNORE INTO mochat_go_saas_admin_audit_chains").WillReturnError(&mysqlDriver.MySQLError{Number: 1146, Message: "Table 'fixture.mochat_go_saas_admin_audit_chains' doesn't exist"})
		mock.ExpectExec("INSERT INTO mochat_go_saas_admin_operation_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	}

	// Save candidate against current v1. The inserted candidate starts at v1.
	mock.ExpectBegin()
	expectAuthBinding(dashboard.SaaSAdminPermissionIntegrationsManage)
	mock.ExpectQuery("SELECT id,mode,slot,status").WithArgs(41, 63).WillReturnRows(integrationRows("current", "active", dashboardadmin.WeComVerificationLocalContract, 1, "", ""))
	mock.ExpectExec("INSERT INTO mochat_go_wecom_integrations").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT id,mode,slot,status").WithArgs(41, 63, "candidate").WillReturnRows(integrationRows("candidate", "pending_verification", "", 1, ciphertext, keyID))
	expectLegacyAudit()
	mock.ExpectCommit()
	saved, err := store.SaveWeComIntegrationCandidate(context.Background(), actor, 41, dashboardadmin.WeComIntegrationCandidateInput{Mode: "self_built", AgentID: "100001", EmployeeSecret: "employee-secret", Version: 1})
	if err != nil || saved.Version != 1 {
		t.Fatalf("save candidate: version=%d err=%v", saved.Version, err)
	}

	// Verify candidate v1, which atomically persists local_contract as v2.
	mock.ExpectBegin()
	expectAuthBinding(dashboard.SaaSAdminPermissionIntegrationsManage)
	mock.ExpectQuery("SELECT id,mode,slot,status").WithArgs(41, 63, "candidate").WillReturnRows(integrationRows("candidate", "pending_verification", "", 1, ciphertext, keyID))
	mock.ExpectCommit()
	mock.ExpectBegin()
	expectAuthBinding(dashboard.SaaSAdminPermissionIntegrationsManage)
	mock.ExpectQuery("SELECT id,mode,slot,status").WithArgs(41, 63, "candidate").WillReturnRows(integrationRows("candidate", "pending_verification", "", 1, ciphertext, keyID))
	mock.ExpectExec("UPDATE mochat_go_wecom_integrations SET status=").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT id,mode,slot,status").WithArgs(41, 63, "candidate").WillReturnRows(integrationRows("candidate", "active", dashboardadmin.WeComVerificationLocalContract, 2, ciphertext, keyID))
	expectLegacyAudit()
	mock.ExpectCommit()
	service := dashboardadmin.NewWeComIntegrationService(store, dashboardadmin.WeComIntegrationVerifierFunc(func(context.Context, dashboardadmin.WeComVerificationRequest) (dashboardadmin.WeComVerificationResult, error) {
		return dashboardadmin.WeComVerificationResult{VerifiedWXCorpID: "ww-authoritative", VerificationLevel: dashboardadmin.WeComVerificationLocalContract}, nil
	}))
	verified, err := service.VerifyCandidate(context.Background(), actor, 41, 1)
	if err != nil || verified.Version != 2 {
		t.Fatalf("verify candidate: version=%d err=%v", verified.Version, err)
	}

	// Switch locks the promoted candidate's v2, not current v1.
	mock.ExpectBegin()
	expectAuthBinding(dashboard.SaaSAdminPermissionIntegrationsManage)
	mock.ExpectQuery("SELECT id,mode,slot,status").WithArgs(41, 63, "current").WillReturnRows(integrationRows("current", "active", dashboardadmin.WeComVerificationLocalContract, 1, "", ""))
	mock.ExpectQuery("SELECT id,mode,slot,status").WithArgs(41, 63, "candidate").WillReturnRows(integrationRows("candidate", "active", dashboardadmin.WeComVerificationLocalContract, 2, ciphertext, keyID))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM mochat_go_archive_media_objects").WithArgs(41, 63).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("DELETE FROM mochat_go_wecom_integrations").WithArgs(41, 63).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO mochat_go_wecom_integrations").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO mochat_go_wecom_integrations").WillReturnResult(sqlmock.NewResult(0, 1))
	expectLegacyAudit()
	mock.ExpectCommit()
	view, err := service.Switch(context.Background(), actor, 41, 2)
	if err != nil || view.Current == nil || view.Current.ID != "candidate" || view.Current.Version != 3 {
		t.Fatalf("switch candidate v2: view=%+v err=%v", view, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWeComIntegrationVerificationActivationRequiresLocalContract(t *testing.T) {
	for _, level := range []string{"", "online", "fixture", "unknown", " local_contract "} {
		t.Run(level, func(t *testing.T) {
			if _, err := weComIntegrationVerificationStatus(dashboardadmin.WeComVerificationResult{VerificationLevel: level}, ""); !errors.Is(err, dashboardadmin.ErrInvalidRequest) {
				t.Fatalf("level=%q err=%v", level, err)
			}
		})
	}
	if status, err := weComIntegrationVerificationStatus(dashboardadmin.WeComVerificationResult{VerificationLevel: dashboardadmin.WeComVerificationLocalContract}, ""); err != nil || status != "active" {
		t.Fatalf("status=%q err=%v", status, err)
	}
}

func TestWeComIntegrationCredentialHintContainsNamesNotValues(t *testing.T) {
	hint := weComCredentialHint(wecomcredentials.AuthorizationCredential{EmployeeSecret: "employee-secret-value", PermanentCode: "permanent-secret-value"})
	if hint != "employee,permanent_code" {
		t.Fatalf("hint=%q", hint)
	}
}
