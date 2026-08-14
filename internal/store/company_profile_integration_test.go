package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcredentials"
)

func TestCompanyProfileStoreDoesNotReadLegacyPlaintextCredentialColumns(t *testing.T) {
	for _, file := range []string{"company_profile.go", "company_employee_sync.go"} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{
			"COALESCE(c.employee_secret",
			"COALESCE(c.contact_secret",
			"COALESCE(c.token",
			"COALESCE(c.encoding_aes_key",
			"COALESCE(c.chat_secret",
			"COALESCE(a.wx_secret",
		} {
			if strings.Contains(string(source), forbidden) {
				t.Fatalf("%s reads legacy plaintext credential column with %s", file, forbidden)
			}
		}
	}
}

func TestCompanyProfileApplicationCallbackAndArchiveConfigurationIsAtomicRealMariaDB(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createDashboardAdminProvisioningFixture(t, db)
	manager := testWeComCredentialManager(t, wecomcredentials.Config{
		EncryptionKey:       testCompanyCredentialKey(19),
		EncryptionKeyID:     "company-settings-key",
		RequireEncryption:   true,
		DedicatedConfigured: true,
	})
	prepareCompanyProfileRepositoryFixture(t, db, manager)
	store := NewMySQLStore(db).WithWeComCredentialCipher(manager)
	principal := dashboardprincipal.DashboardPrincipal{
		UserID: 10, TenantID: 1, CorpID: 100, CorpStatus: dashboardprincipal.CorpBindingStatusPending,
		IsSuperAdmin: true, AuthVersion: 1,
	}

	profile, err := store.ConfigureApplication(context.Background(), principal, companyprofile.ApplicationCredentialsInput{
		WXAgentID: "1000099", Secret: "one-application-secret", ExpectedVersion: 1, RequestID: "configure-one-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.BindingVersion != 2 || profile.ApplicationAgentID != "1000099" {
		t.Fatalf("profile=%+v, want version 2 and application agent 1000099", profile)
	}
	corpCredential := loadAndDecryptCompanyCredential(t, manager, db, 100, "ww-candidate")
	if corpCredential.EmployeeSecret != "one-application-secret" || corpCredential.ContactSecret != "one-application-secret" {
		t.Fatalf("shared corp secrets not updated together: %+v", corpCredential)
	}
	if corpCredential.CallbackToken == "" || len(corpCredential.EncodingAESKey) != 43 {
		t.Fatalf("initial callback configuration not generated: %+v", corpCredential)
	}
	var wxAgentID, agentCiphertext, agentKeyID, plaintextSecret string
	if err := db.QueryRow(`SELECT wx_agent_id, COALESCE(CAST(wecom_credentials_ciphertext AS CHAR),''), COALESCE(wecom_credentials_key_id,''), COALESCE(wx_secret,'') FROM mc_work_agent WHERE id=300`).Scan(&wxAgentID, &agentCiphertext, &agentKeyID, &plaintextSecret); err != nil {
		t.Fatal(err)
	}
	if wxAgentID != "1000099" || plaintextSecret != "" {
		t.Fatalf("agent id=%q plaintext=%q", wxAgentID, plaintextSecret)
	}
	agentCredential, err := manager.DecryptAgent(100, wxAgentID, agentKeyID, agentCiphertext)
	if err != nil || agentCredential.WXSecret != "one-application-secret" {
		t.Fatalf("agent credential=%+v err=%v", agentCredential, err)
	}

	callback, err := store.GetCallbackConfiguration(context.Background(), principal)
	if err != nil {
		t.Fatal(err)
	}
	if callback.CorpID != 100 || callback.Token != corpCredential.CallbackToken || callback.EncodingAESKey != corpCredential.EncodingAESKey || !callback.Configured || callback.BindingVersion != 2 {
		t.Fatalf("callback=%+v", callback)
	}
	rotatedCallback, err := store.RegenerateCallbackConfiguration(context.Background(), principal, companyprofile.CallbackConfigurationInput{
		Token: "replacement-callback-token", EncodingAESKey: strings.Repeat("b", 43), ExpectedVersion: 2, RequestID: "rotate-callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rotatedCallback.Token != "replacement-callback-token" || rotatedCallback.BindingVersion != 3 {
		t.Fatalf("rotated callback=%+v", rotatedCallback)
	}

	chatSecret, publicKey, privateKey := "archive-secret", "public-pem", "private-pem"
	profile, err = store.RotateArchiveCredentials(context.Background(), principal, companyprofile.ArchiveCredentialsInput{
		ChatSecret: &chatSecret, RSAPublicKey: &publicKey, RSAPrivateKey: &privateKey,
		ExpectedVersion: 3, RequestID: "archive-key-pair",
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.BindingVersion != 4 {
		t.Fatalf("archive binding version=%d, want 4", profile.BindingVersion)
	}
	archiveCredential := loadAndDecryptCompanyCredential(t, manager, db, 100, "ww-candidate")
	if archiveCredential.ChatSecret != chatSecret || archiveCredential.ArchiveRSAPublicKey != publicKey || archiveCredential.ArchiveRSAPrivateKey != privateKey {
		t.Fatalf("archive credential=%+v", archiveCredential)
	}
	var auditCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id=1 AND request_id IN ('configure-one-secret','rotate-callback','archive-key-pair')`).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 3 {
		t.Fatalf("audit count=%d, want 3", auditCount)
	}
}

func TestCompanyProfileRepositoryRotateVerifyAndSyncIsBindingScopedRealMariaDB(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createDashboardAdminProvisioningFixture(t, db)
	manager := testWeComCredentialManager(t, wecomcredentials.Config{
		EncryptionKey:       testCompanyCredentialKey(17),
		EncryptionKeyID:     "task10-company-key",
		RequireEncryption:   true,
		DedicatedConfigured: true,
	})
	prepareCompanyProfileRepositoryFixture(t, db, manager)
	store := NewMySQLStore(db).WithWeComCredentialCipher(manager)
	principal := dashboardprincipal.DashboardPrincipal{
		UserID:       10,
		TenantID:     1,
		CorpID:       100,
		CorpStatus:   dashboardprincipal.CorpBindingStatusPending,
		IsSuperAdmin: true,
		AuthVersion:  1,
	}
	ctx := context.Background()

	profile, err := store.GetProfile(ctx, principal)
	if err != nil {
		t.Fatal(err)
	}
	if profile.BindingVersion != 1 || profile.WXCorpID != "" || !profile.Credentials.WeCom.Configured {
		t.Fatalf("initial profile=%+v", profile)
	}
	assertCompanyProfileJSONHasNoCredentialMaterial(t, profile)

	rotatedSecret := "employee-rotated"
	profile, err = store.RotateWeComCredentials(ctx, principal, companyprofile.WeComCredentialsInput{
		EmployeeSecret: &rotatedSecret, ExpectedVersion: 1, RequestID: "task10-rotate-wecom",
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.BindingVersion != 2 {
		t.Fatalf("rotated binding version=%d, want 2", profile.BindingVersion)
	}
	var wxCorpIDAfterRotate string
	if err := db.QueryRow(`SELECT COALESCE(wx_corpid,'') FROM mc_corp WHERE id=100`).Scan(&wxCorpIDAfterRotate); err != nil {
		t.Fatal(err)
	}
	if wxCorpIDAfterRotate != "ww-candidate" {
		t.Fatalf("credential rotation changed authoritative candidate wxCorpID=%q", wxCorpIDAfterRotate)
	}
	assertCompanyCredentialColumnsEncrypted(t, db, 100, "task10-company-key")

	var employeeSecret, contactSecret, callbackToken, encodingAESKey, chatSecret string
	if err := db.QueryRow(`SELECT employee_secret, contact_secret, token, encoding_aes_key, chat_secret FROM mc_corp WHERE id=100`).Scan(&employeeSecret, &contactSecret, &callbackToken, &encodingAESKey, &chatSecret); err != nil {
		t.Fatal(err)
	}
	if employeeSecret != "" || contactSecret != "" || callbackToken != "" || encodingAESKey != "" || chatSecret != "" {
		t.Fatalf("plaintext credential columns remain: employee=%q contact=%q token=%q aes=%q chat=%q", employeeSecret, contactSecret, callbackToken, encodingAESKey, chatSecret)
	}
	credentialRow := loadCompanyCredentialFixture(t, db, 100)
	decoded, err := manager.DecryptCorp(1, "ww-candidate", credentialRow.KeyID, credentialRow.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.EmployeeSecret != rotatedSecret || decoded.ContactSecret != "contact-fixture" || decoded.CallbackToken != "callback-fixture" || decoded.EncodingAESKey != "aes-fixture" || decoded.ChatSecret != "archive-fixture" {
		t.Fatalf("decoded credential fields do not round-trip: %+v", decoded)
	}

	wrongVersionSecret := "must-not-write"
	if _, err := store.RotateWeComCredentials(ctx, principal, companyprofile.WeComCredentialsInput{
		EmployeeSecret: &wrongVersionSecret, ExpectedVersion: 1, RequestID: "task10-stale-rotate",
	}); !errors.Is(err, companyprofile.ErrVersionConflict) {
		t.Fatalf("stale rotate error=%v, want ErrVersionConflict", err)
	}
	decodedAfterStale := loadAndDecryptCompanyCredential(t, manager, db, 100, "ww-candidate")
	if decodedAfterStale.EmployeeSecret != rotatedSecret {
		t.Fatalf("stale rotate changed employee secret=%q", decodedAfterStale.EmployeeSecret)
	}

	verifiedProfile, err := store.CommitVerification(ctx, principal, companyprofile.VerifyInput{
		ExpectedVersion: 2, RequestID: "task10-verify",
	}, companyprofile.VerificationResult{WXCorpID: "ww-authoritative", CorpName: "权威企业"})
	if err != nil {
		t.Fatal(err)
	}
	if verifiedProfile.BindingVersion != 3 || verifiedProfile.WXCorpID != "ww-authoritative" || verifiedProfile.AuthoritativeCorpName != "权威企业" {
		t.Fatalf("verified profile=%+v", verifiedProfile)
	}
	decodedAfterVerify := loadAndDecryptCompanyCredential(t, manager, db, 100, "ww-authoritative")
	if decodedAfterVerify.EmployeeSecret != rotatedSecret || decodedAfterVerify.ChatSecret != "archive-fixture" {
		t.Fatalf("verify did not rebind encrypted credential AAD: %+v", decodedAfterVerify)
	}

	agentSecret := "agent-rotated"
	verifiedPrincipal := principal
	verifiedPrincipal.CorpStatus = dashboardprincipal.CorpBindingStatusActive
	queued, err := store.QueueEmployeeSync(ctx, verifiedPrincipal)
	if err != nil {
		t.Fatal(err)
	}
	if queued.AlreadyQueued || queued.Cursor != dashboard.CompanyEmployeeSyncCursor {
		t.Fatalf("first queue result=%+v", queued)
	}
	duplicateQueue, err := store.QueueEmployeeSync(ctx, verifiedPrincipal)
	if err != nil {
		t.Fatal(err)
	}
	if !duplicateQueue.AlreadyQueued || duplicateQueue.Cursor != dashboard.CompanyEmployeeSyncCursor {
		t.Fatalf("duplicate queue result=%+v", duplicateQueue)
	}
	queuedStatus, err := store.GetSyncStatus(ctx, verifiedPrincipal)
	if err != nil {
		t.Fatal(err)
	}
	if queuedStatus.Status != "queued" || queuedStatus.Cursor != dashboard.CompanyEmployeeSyncCursor {
		t.Fatalf("queued sync status=%+v", queuedStatus)
	}
	if err := store.BeginCompanyEmployeeSync(ctx, verifiedPrincipal.TenantID); err != nil {
		t.Fatal(err)
	}
	runningStatus, err := store.GetSyncStatus(ctx, verifiedPrincipal)
	if err != nil {
		t.Fatal(err)
	}
	if runningStatus.Status != "syncing" || runningStatus.Cursor != dashboard.CompanyEmployeeSyncCursor {
		t.Fatalf("running sync status=%+v", runningStatus)
	}
	if err := store.RecordCompanyEmployeeSyncFailure(ctx, verifiedPrincipal.TenantID); err != nil {
		t.Fatal(err)
	}
	failedStatus, err := store.GetSyncStatus(ctx, verifiedPrincipal)
	if err != nil {
		t.Fatal(err)
	}
	if failedStatus.Status != "failed" || failedStatus.ErrorCode != "SYNC_FAILED" {
		t.Fatalf("failed sync status=%+v", failedStatus)
	}
	if _, err := store.RotateAgentCredentials(ctx, verifiedPrincipal, companyprofile.AgentCredentialsInput{
		AgentID: 300, WXSecret: &agentSecret, ExpectedVersion: 3, RequestID: "task10-rotate-agent",
	}); err != nil {
		t.Fatal(err)
	}
	assertCompanyAgentEncrypted(t, db, 300, "task10-company-key")
	var agentCiphertext, agentKeyID string
	if err := db.QueryRow(`SELECT wecom_credentials_ciphertext, wecom_credentials_key_id FROM mc_work_agent WHERE id=300`).Scan(&agentCiphertext, &agentKeyID); err != nil {
		t.Fatal(err)
	}
	decodedAgent, err := manager.DecryptAgent(100, "100001", agentKeyID, agentCiphertext)
	if err != nil || decodedAgent.WXSecret != agentSecret {
		t.Fatalf("decoded agent=%+v err=%v", decodedAgent, err)
	}
	preservedCiphertext, preservedKeyID := agentCiphertext, agentKeyID
	identifierOnlyProfile, err := store.RotateAgentCredentials(ctx, verifiedPrincipal, companyprofile.AgentCredentialsInput{
		AgentID: 300, ExpectedVersion: 4, RequestID: "task10-rotate-agent-identifiers-only",
	})
	if err != nil {
		t.Fatal(err)
	}
	if identifierOnlyProfile.BindingVersion != 5 {
		t.Fatalf("identifier-only profile version=%d, want 5", identifierOnlyProfile.BindingVersion)
	}
	if err := db.QueryRow(`SELECT wecom_credentials_ciphertext, wecom_credentials_key_id FROM mc_work_agent WHERE id=300`).Scan(&agentCiphertext, &agentKeyID); err != nil {
		t.Fatal(err)
	}
	if agentCiphertext != preservedCiphertext || agentKeyID != preservedKeyID {
		t.Fatalf("identifier-only rotation changed encrypted secret storage")
	}
	decodedAgent, err = manager.DecryptAgent(100, "100001", agentKeyID, agentCiphertext)
	if err != nil || decodedAgent.WXSecret != agentSecret {
		t.Fatalf("identifier-only decoded agent=%+v err=%v", decodedAgent, err)
	}
	if _, err := store.RotateAgentCredentials(ctx, verifiedPrincipal, companyprofile.AgentCredentialsInput{
		AgentID: 999, ExpectedVersion: 5, RequestID: "task10-rotate-agent-missing",
	}); !errors.Is(err, companyprofile.ErrNotFound) {
		t.Fatalf("missing agent error=%v, want ErrNotFound", err)
	}
	var identifierAuditCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id=1 AND action=? AND request_id=?`, "dashboard.company.agent_credentials.rotate", "task10-rotate-agent-identifiers-only").Scan(&identifierAuditCount); err != nil {
		t.Fatal(err)
	}
	if identifierAuditCount != 1 {
		t.Fatalf("identifier-only audit count=%d, want 1", identifierAuditCount)
	}

	countsBeforeSync := companyIdentityAndRBACCounts(t, db)
	firstSync, err := store.SyncEmployeeData(ctx, verifiedPrincipal, companyprofile.EmployeeSyncData{
		Departments: []companyprofile.SyncDepartment{{WXDepartmentID: 7, Name: "销售", WXParentID: 0, Order: 1}},
		Employees: []companyprofile.SyncEmployee{{
			WXUserID: "wecom-user-1", Name: "员工一", Mobile: "13900000001", Status: 1,
			WXMainDepartmentID: 7, DepartmentIDs: []int{7}, IsLeaderInDepartment: []int{1}, DepartmentOrders: []int{1},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if firstSync.Status != "completed" || firstSync.DepartmentsCreated != 1 || firstSync.EmployeesCreated != 1 {
		t.Fatalf("first sync=%+v", firstSync)
	}
	assertCompanySyncJSONHasNoCredentialMaterial(t, firstSync)
	countsAfterFirstSync := companyIdentityAndRBACCounts(t, db)
	if countsAfterFirstSync != countsBeforeSync {
		t.Fatalf("sync changed identity/RBAC tables: before=%v after=%v", countsBeforeSync, countsAfterFirstSync)
	}

	secondSync, err := store.SyncEmployeeData(ctx, verifiedPrincipal, companyprofile.EmployeeSyncData{
		Departments: []companyprofile.SyncDepartment{{WXDepartmentID: 7, Name: "销售二部", WXParentID: 0, Order: 2}},
		Employees: []companyprofile.SyncEmployee{{
			WXUserID: "wecom-user-1", Name: "员工一离职", Mobile: "13900000001", Status: 2,
			WXMainDepartmentID: 7, DepartmentIDs: []int{7}, IsLeaderInDepartment: []int{0}, DepartmentOrders: []int{2},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if secondSync.Status != "completed" || secondSync.EmployeesUpdated != 1 {
		t.Fatalf("second sync=%+v", secondSync)
	}
	var employeeStatus int
	if err := db.QueryRow(`SELECT status FROM mc_work_employee WHERE corp_id=100 AND wx_user_id='wecom-user-1' AND deleted_at IS NULL`).Scan(&employeeStatus); err != nil {
		t.Fatal(err)
	}
	if employeeStatus != 2 {
		t.Fatalf("employee status=%d, want 2", employeeStatus)
	}
	if companyIdentityAndRBACCounts(t, db) != countsBeforeSync {
		t.Fatal("repeat sync changed identity/RBAC tables")
	}

	beforeFailureData := companySyncDataCounts(t, db)
	if _, err := db.Exec(`CREATE TRIGGER task10_company_sync_failure BEFORE INSERT ON mc_work_employee FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'company sync fixture failure'`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SyncEmployeeData(ctx, verifiedPrincipal, companyprofile.EmployeeSyncData{
		Departments: []companyprofile.SyncDepartment{{WXDepartmentID: 8, Name: "不会提交", WXParentID: 0, Order: 1}},
		Employees: []companyprofile.SyncEmployee{{
			WXUserID: "wecom-user-rollback", Name: "回滚员工", Status: 1,
			WXMainDepartmentID: 8, DepartmentIDs: []int{8},
		}},
	}); err == nil {
		t.Fatal("sync trigger failure unexpectedly committed")
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS task10_company_sync_failure`); err != nil {
		t.Fatal(err)
	}
	if afterFailureData := companySyncDataCounts(t, db); afterFailureData != beforeFailureData {
		t.Fatalf("sync failure left business rows: before=%v after=%v", beforeFailureData, afterFailureData)
	}

	status, err := store.GetSyncStatus(ctx, verifiedPrincipal)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "failed" || status.ErrorCode != "SYNC_FAILED" || status.Departments != 1 || status.Employees != 1 {
		t.Fatalf("sync status=%+v", status)
	}
	assertCompanySyncStatusJSONHasNoCredentialMaterial(t, status)
	if _, err := store.SyncEmployeeData(ctx, verifiedPrincipal, companyprofile.EmployeeSyncData{}); err != nil {
		t.Fatal(err)
	}
	status, err = store.GetSyncStatus(ctx, verifiedPrincipal)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "completed" || status.ErrorCode != "" {
		t.Fatalf("successful retry did not clear sync failure: %+v", status)
	}

	wrongCorp := verifiedPrincipal
	wrongCorp.CorpID = 200
	if _, err := store.GetProfile(ctx, wrongCorp); !errors.Is(err, companyprofile.ErrNotFound) {
		t.Fatalf("cross-corp profile error=%v, want ErrNotFound", err)
	}
}

func TestCompanyProfileCredentialRotationScopesVerifiedBindingInvalidationRealMariaDB(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createDashboardAdminProvisioningFixture(t, db)
	manager := testWeComCredentialManager(t, wecomcredentials.Config{
		EncryptionKey: testCompanyCredentialKey(23), EncryptionKeyID: "rotation-invalidation-key",
		RequireEncryption: true, DedicatedConfigured: true,
	})
	prepareCompanyProfileRepositoryFixture(t, db, manager)
	store := NewMySQLStore(db).WithWeComCredentialCipher(manager)
	principal := dashboardprincipal.DashboardPrincipal{UserID: 10, TenantID: 1, CorpID: 100, CorpStatus: dashboardprincipal.CorpBindingStatusPending, IsSuperAdmin: true, AuthVersion: 1}
	ctx := context.Background()

	verified, err := store.CommitVerification(ctx, principal, companyprofile.VerifyInput{ExpectedVersion: 1, RequestID: "rotation-invalidation-verify"}, companyprofile.VerificationResult{WXCorpID: "ww-authoritative", CorpName: "Verified corp"})
	if err != nil {
		t.Fatal(err)
	}
	if verified.VerifiedAt == nil || verified.WXCorpID != "ww-authoritative" || verified.BindingVersion != 2 {
		t.Fatalf("verified profile=%+v", verified)
	}

	archiveSecret := "archive-after-verify"
	archivePublic := "archive-public-after-verify"
	archivePrivate := "archive-private-after-verify"
	rotatedArchive, err := store.RotateArchiveCredentials(ctx, principal, companyprofile.ArchiveCredentialsInput{
		ChatSecret: &archiveSecret, RSAPublicKey: &archivePublic, RSAPrivateKey: &archivePrivate,
		ExpectedVersion: 2, RequestID: "rotation-preserve-archive",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rotatedArchive.VerifiedAt == nil || rotatedArchive.WXCorpID != "ww-authoritative" || rotatedArchive.AuthoritativeCorpName != "Verified corp" || rotatedArchive.BindingVersion != 3 {
		t.Fatalf("archive credential rotation invalidated standard verification: %+v", rotatedArchive)
	}

	rotatedCallback, err := store.RegenerateCallbackConfiguration(ctx, principal, companyprofile.CallbackConfigurationInput{
		Token: "callback-after-verify", EncodingAESKey: strings.Repeat("c", 43), ExpectedVersion: 3, RequestID: "rotation-preserve-callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rotatedCallback.BindingVersion != 4 {
		t.Fatalf("rotated callback=%+v", rotatedCallback)
	}
	verifiedAfterCallback, err := store.GetProfile(ctx, principal)
	if err != nil {
		t.Fatal(err)
	}
	if verifiedAfterCallback.VerifiedAt == nil || verifiedAfterCallback.WXCorpID != "ww-authoritative" || verifiedAfterCallback.BindingVersion != 4 {
		t.Fatalf("callback rotation invalidated standard verification: %+v", verifiedAfterCallback)
	}

	agentSecret := "agent-after-verify"
	rotatedAgent, err := store.RotateAgentCredentials(ctx, principal, companyprofile.AgentCredentialsInput{AgentID: 300, WXSecret: &agentSecret, ExpectedVersion: 4, RequestID: "rotation-preserve-agent"})
	if err != nil {
		t.Fatal(err)
	}
	if rotatedAgent.VerifiedAt == nil || rotatedAgent.WXCorpID != "ww-authoritative" || rotatedAgent.BindingVersion != 5 {
		t.Fatalf("agent credential rotation invalidated standard verification: %+v", rotatedAgent)
	}

	chatOnlySecret := "chat-only-after-verify"
	rotatedNonStandard, err := store.RotateWeComCredentials(ctx, principal, companyprofile.WeComCredentialsInput{ChatSecret: &chatOnlySecret, ExpectedVersion: 5, RequestID: "rotation-preserve-chat-only"})
	if err != nil {
		t.Fatal(err)
	}
	if rotatedNonStandard.VerifiedAt == nil || rotatedNonStandard.WXCorpID != "ww-authoritative" || rotatedNonStandard.AuthoritativeCorpName != "Verified corp" || rotatedNonStandard.BindingVersion != 6 {
		t.Fatalf("non-standard-only WeCom rotation invalidated standard verification: %+v", rotatedNonStandard)
	}

	rotatedSecret := "rotated-after-verify"
	rotated, err := store.RotateWeComCredentials(ctx, principal, companyprofile.WeComCredentialsInput{EmployeeSecret: &rotatedSecret, ExpectedVersion: 6, RequestID: "rotation-invalidation-standard"})
	if err != nil {
		t.Fatal(err)
	}
	if rotated.VerifiedAt != nil || rotated.WXCorpID != "" || rotated.AuthoritativeCorpName != "" || rotated.BindingVersion != 7 {
		t.Fatalf("standard credential rotation retained stale verification: %+v", rotated)
	}

	verifiedAgain, err := store.CommitVerification(ctx, principal, companyprofile.VerifyInput{ExpectedVersion: 7, RequestID: "rotation-invalidation-reverify"}, companyprofile.VerificationResult{WXCorpID: "ww-authoritative", CorpName: "Verified again"})
	if err != nil {
		t.Fatal(err)
	}
	if verifiedAgain.VerifiedAt == nil || verifiedAgain.BindingVersion != 8 {
		t.Fatalf("reverified profile=%+v", verifiedAgain)
	}
	configured, err := store.ConfigureApplication(ctx, principal, companyprofile.ApplicationCredentialsInput{WXAgentID: "100001", Secret: "rotated-application-after-verify", ExpectedVersion: 8, RequestID: "rotation-invalidation-application"})
	if err != nil {
		t.Fatal(err)
	}
	if configured.VerifiedAt != nil || configured.WXCorpID != "" || configured.AuthoritativeCorpName != "" || configured.BindingVersion != 9 {
		t.Fatalf("application credential rotation retained stale verification: %+v", configured)
	}
}

func TestCompanyProfileAgentNoopFallbackRequiresFullOwnershipRealMariaDB(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createDashboardAdminProvisioningFixture(t, db)
	manager := testWeComCredentialManager(t, wecomcredentials.Config{
		EncryptionKey: testCompanyCredentialKey(19), EncryptionKeyID: "task10-agent-fallback-key", RequireEncryption: true, DedicatedConfigured: true,
	})
	prepareCompanyProfileRepositoryFixture(t, db, manager)
	var ciphertext, keyID string
	if err := db.QueryRow(`SELECT COALESCE(CAST(wecom_credentials_ciphertext AS CHAR),''), COALESCE(wecom_credentials_key_id,'') FROM mc_work_agent WHERE id=300`).Scan(&ciphertext, &keyID); err != nil {
		t.Fatal(err)
	}
	storage := agentCredentialStorage{Ciphertext: ciphertext, KeyID: keyID}
	baseBinding := companyBindingRecord{TenantID: 1, CorpID: 100, Status: 1, Version: 1}

	cases := []struct {
		name    string
		mutate  string
		binding companyBindingRecord
		result  fixedCompanySQLResult
		wantOK  bool
	}{
		{name: "exact desired state", binding: baseBinding, result: fixedCompanySQLResult{rows: 0}, wantOK: true},
		{name: "wrong tenant", binding: companyBindingRecord{TenantID: 2, CorpID: 100, Status: 1, Version: 1}, result: fixedCompanySQLResult{rows: 0}},
		{name: "wrong version", binding: companyBindingRecord{TenantID: 1, CorpID: 100, Status: 1, Version: 2}, result: fixedCompanySQLResult{rows: 0}},
		{name: "wrong binding status", binding: companyBindingRecord{TenantID: 1, CorpID: 100, Status: 3, Version: 1}, result: fixedCompanySQLResult{rows: 0}},
		{name: "deleted agent", binding: baseBinding, mutate: `UPDATE mc_work_agent SET deleted_at=NOW() WHERE id=300`, result: fixedCompanySQLResult{rows: 0}},
		{name: "wrong desired ciphertext", binding: baseBinding, mutate: `UPDATE mc_work_agent SET wecom_credentials_key_id='other-key' WHERE id=300`, result: fixedCompanySQLResult{rows: 0}},
		{name: "driver rows error", binding: baseBinding, result: fixedCompanySQLResult{rows: 0, err: errors.New("rows affected unavailable")}},
		{name: "too many rows", binding: baseBinding, result: fixedCompanySQLResult{rows: 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := db.BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if tc.mutate != "" {
				if _, err := tx.Exec(tc.mutate); err != nil {
					t.Fatal(err)
				}
			}
			err = requireCompanyAgentRowsOrMatched(context.Background(), tx, tc.result, tc.binding, 300, storage)
			if tc.wantOK && err != nil {
				t.Fatalf("fallback error=%v, want exact desired state accepted", err)
			}
			if !tc.wantOK && err == nil {
				t.Fatal("fallback accepted a row outside tenant/version/status/deleted/desired-state contract")
			}
		})
	}
}

type fixedCompanySQLResult struct {
	rows int64
	err  error
}

func (r fixedCompanySQLResult) LastInsertId() (int64, error) { return 0, nil }
func (r fixedCompanySQLResult) RowsAffected() (int64, error) { return r.rows, r.err }

type companyCredentialFixtureRow struct {
	KeyID      string
	Ciphertext string
}

func prepareCompanyProfileRepositoryFixture(t *testing.T, db *sql.DB, manager *wecomcredentials.Manager) {
	t.Helper()
	corpCiphertext, corpKeyID, err := manager.EncryptCorp(1, "ww-candidate", wecomcredentials.CorpCredential{
		EmployeeSecret: "employee-fixture", ContactSecret: "contact-fixture", CallbackToken: "callback-fixture",
		EncodingAESKey: "aes-fixture", ChatSecret: "archive-fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	agentCiphertext, agentKeyID, err := manager.EncryptAgent(100, "100001", wecomcredentials.AgentCredential{WXSecret: "agent-fixture"})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`ALTER TABLE mc_corp ADD COLUMN chat_secret varchar(255) NOT NULL DEFAULT '', ADD COLUMN wecom_credentials_ciphertext text NULL, ADD COLUMN wecom_credentials_key_id varchar(64) NOT NULL DEFAULT ''`,
		`CREATE TABLE mc_work_agent (id int(10) unsigned NOT NULL AUTO_INCREMENT, corp_id int(11) NOT NULL, wx_agent_id varchar(255) NOT NULL DEFAULT '', wx_secret varchar(255) NOT NULL DEFAULT '', name varchar(255) NOT NULL DEFAULT '', square_logo_url varchar(255) NOT NULL DEFAULT '', description varchar(255) NOT NULL DEFAULT '', close tinyint NOT NULL DEFAULT 0, redirect_domain varchar(255) NOT NULL DEFAULT '', report_location_flag tinyint NOT NULL DEFAULT 0, is_reportenter tinyint NOT NULL DEFAULT 0, home_url varchar(255) NOT NULL DEFAULT '', created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at timestamp NULL DEFAULT NULL, deleted_at timestamp NULL DEFAULT NULL, wecom_credentials_ciphertext text NULL, wecom_credentials_key_id varchar(64) NOT NULL DEFAULT '', PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_department (id int(10) unsigned NOT NULL AUTO_INCREMENT, wx_department_id int(10) unsigned NOT NULL DEFAULT 0, corp_id int(10) unsigned NOT NULL, name varchar(255) NOT NULL DEFAULT '', parent_id int(10) unsigned NOT NULL DEFAULT 0, wx_parentid int(10) unsigned NOT NULL DEFAULT 0, ` + "`order`" + ` int(10) unsigned NOT NULL DEFAULT 0, level tinyint NOT NULL DEFAULT 0, path varchar(255) NOT NULL DEFAULT '', created_at timestamp NULL, updated_at timestamp NULL, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_employee (id int(10) unsigned NOT NULL AUTO_INCREMENT, wx_user_id varchar(255) NOT NULL DEFAULT '', corp_id int(11) NOT NULL DEFAULT 0, name varchar(255) NOT NULL DEFAULT '', mobile char(11) NOT NULL DEFAULT '', position varchar(255) NOT NULL DEFAULT '', gender tinyint unsigned NOT NULL DEFAULT 0, email varchar(255) NOT NULL DEFAULT '', avatar varchar(255) NOT NULL DEFAULT '', thumb_avatar varchar(255) NOT NULL DEFAULT '', telephone varchar(255) NOT NULL DEFAULT '', alias varchar(255) NOT NULL DEFAULT '', extattr json DEFAULT NULL, status tinyint unsigned NOT NULL DEFAULT 0, qr_code varchar(255) NOT NULL DEFAULT '', external_profile json DEFAULT NULL, external_position varchar(255) DEFAULT '', address varchar(255) NOT NULL DEFAULT '', open_user_id char(100) NOT NULL DEFAULT '', wx_main_department_id int(10) unsigned NOT NULL DEFAULT 0, main_department_id int(11) NOT NULL DEFAULT 0, log_user_id int(10) unsigned NOT NULL DEFAULT 0, contact_auth tinyint NOT NULL DEFAULT 2, audit_status tinyint NOT NULL DEFAULT 0, created_at timestamp NULL, updated_at timestamp NULL, deleted_at timestamp NULL, PRIMARY KEY (id), KEY idx_company_employee_corp (corp_id, deleted_at)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_employee_department (id int(10) unsigned NOT NULL AUTO_INCREMENT, employee_id int(10) unsigned NOT NULL DEFAULT 0, department_id int(10) unsigned NOT NULL DEFAULT 0, is_leader_in_dept tinyint NOT NULL DEFAULT 0, ` + "`order`" + ` int NOT NULL DEFAULT 0, created_at timestamp NULL, updated_at timestamp NULL, deleted_at timestamp NULL, PRIMARY KEY (id), KEY idx_company_employee_department (employee_id, deleted_at, department_id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_update_time (id int(10) unsigned NOT NULL AUTO_INCREMENT, corp_id int(11) NOT NULL DEFAULT 0, type tinyint NOT NULL DEFAULT 0, last_update_time timestamp NULL, error_msg json DEFAULT NULL, created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`INSERT INTO mochat_go_dashboard_identities (user_id, login_identifier, password_hash, status, must_rotate_password, auth_version, mfa_required, activated_at) VALUES (10, '13800000001', '!task10-fixture-hash', 1, 0, 1, 0, NOW())`,
		`UPDATE mc_user SET isSuperAdmin=1 WHERE id=10`,
		`UPDATE mc_corp SET wx_corpid='ww-candidate', employee_secret='', contact_secret='', token='', encoding_aes_key='', chat_secret='' WHERE id=100`,
		`INSERT INTO mochat_go_tenant_corp_bindings (tenant_id, corp_id, status, version, verified_wx_corpid, verified_corp_name) VALUES (1, 100, 1, 1, NULL, '')`,
		`INSERT INTO mc_work_agent (id, corp_id, wx_agent_id, wx_secret, name) VALUES (300, 100, '100001', '', 'Fixture agent')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("company repository fixture: %v", err)
		}
	}
	if _, err := db.Exec(`UPDATE mc_corp SET wecom_credentials_ciphertext=?, wecom_credentials_key_id=? WHERE id=100`, corpCiphertext, corpKeyID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE mc_work_agent SET wecom_credentials_ciphertext=?, wecom_credentials_key_id=? WHERE id=300`, agentCiphertext, agentKeyID); err != nil {
		t.Fatal(err)
	}
}

func assertCompanyCredentialColumnsEncrypted(t *testing.T, db *sql.DB, corpID int, keyID string) {
	t.Helper()
	var ciphertext, storedKeyID string
	if err := db.QueryRow(`SELECT COALESCE(CAST(wecom_credentials_ciphertext AS CHAR),''), COALESCE(wecom_credentials_key_id,'') FROM mc_corp WHERE id=?`, corpID).Scan(&ciphertext, &storedKeyID); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(ciphertext) == "" || storedKeyID != keyID || strings.Contains(ciphertext, "employee-fixture") || strings.Contains(ciphertext, "archive-fixture") {
		t.Fatalf("corp credential is not protected: key=%q ciphertext=%q", storedKeyID, ciphertext)
	}
}

func assertCompanyAgentEncrypted(t *testing.T, db *sql.DB, agentID int, keyID string) {
	t.Helper()
	var ciphertext, storedKeyID, plaintext string
	if err := db.QueryRow(`SELECT COALESCE(wx_secret,''), COALESCE(CAST(wecom_credentials_ciphertext AS CHAR),''), COALESCE(wecom_credentials_key_id,'') FROM mc_work_agent WHERE id=?`, agentID).Scan(&plaintext, &ciphertext, &storedKeyID); err != nil {
		t.Fatal(err)
	}
	if plaintext != "" || strings.TrimSpace(ciphertext) == "" || storedKeyID != keyID {
		t.Fatalf("agent credential is not protected: plaintext=%q key=%q ciphertext=%q", plaintext, storedKeyID, ciphertext)
	}
}

func loadCompanyCredentialFixture(t *testing.T, db *sql.DB, corpID int) companyCredentialFixtureRow {
	t.Helper()
	var row companyCredentialFixtureRow
	if err := db.QueryRow(`SELECT COALESCE(wecom_credentials_key_id,''), COALESCE(CAST(wecom_credentials_ciphertext AS CHAR),'') FROM mc_corp WHERE id=?`, corpID).Scan(&row.KeyID, &row.Ciphertext); err != nil {
		t.Fatal(err)
	}
	return row
}

func loadAndDecryptCompanyCredential(t *testing.T, manager *wecomcredentials.Manager, db *sql.DB, corpID int, wxCorpID string) wecomcredentials.CorpCredential {
	t.Helper()
	row := loadCompanyCredentialFixture(t, db, corpID)
	credential, err := manager.DecryptCorp(1, wxCorpID, row.KeyID, row.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	return credential
}

func companyIdentityAndRBACCounts(t *testing.T, db *sql.DB) [4]int {
	t.Helper()
	var dashboardIdentities, dashboardMFA, userRoles, directPermissions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_identities`).Scan(&dashboardIdentities); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_mfa_credentials`).Scan(&dashboardMFA); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mc_rbac_user_role`).Scan(&userRoles); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_user_permissions`).Scan(&directPermissions); err != nil {
		t.Fatal(err)
	}
	return [4]int{dashboardIdentities, dashboardMFA, userRoles, directPermissions}
}

func companySyncDataCounts(t *testing.T, db *sql.DB) [4]int {
	t.Helper()
	var departments, employees, relations, updates int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mc_work_department WHERE corp_id=100 AND deleted_at IS NULL`).Scan(&departments); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mc_work_employee WHERE corp_id=100 AND deleted_at IS NULL`).Scan(&employees); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mc_work_employee_department WHERE deleted_at IS NULL`).Scan(&relations); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mc_work_update_time WHERE corp_id=100 AND type=1`).Scan(&updates); err != nil {
		t.Fatal(err)
	}
	return [4]int{departments, employees, relations, updates}
}

func assertCompanyProfileJSONHasNoCredentialMaterial(t *testing.T, profile companyprofile.Profile) {
	t.Helper()
	serialized, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(serialized))
	for _, forbidden := range []string{"secret", "ciphertext", "password", "hash", "token"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("profile JSON contains forbidden field %q", forbidden)
		}
	}
}

func assertCompanySyncJSONHasNoCredentialMaterial(t *testing.T, result companyprofile.SyncResult) {
	t.Helper()
	serialized, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(serialized))
	for _, forbidden := range []string{"secret", "ciphertext", "password", "hash", "token", "employee-fixture", "archive-fixture"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("sync result contains forbidden material %q", forbidden)
		}
	}
}

func assertCompanySyncStatusJSONHasNoCredentialMaterial(t *testing.T, status companyprofile.SyncStatus) {
	t.Helper()
	serialized, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(serialized))
	for _, forbidden := range []string{"secret", "ciphertext", "password", "hash", "token", "company sync fixture failure"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("sync status contains forbidden material %q", forbidden)
		}
	}
}

func testCompanyCredentialKey(value byte) string {
	key := make([]byte, 32)
	for index := range key {
		key[index] = value
	}
	return base64.StdEncoding.EncodeToString(key)
}
