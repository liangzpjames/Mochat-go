package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardadmin"
	"jiyi/mochat-go/internal/saasauth"

	mysqldriver "github.com/go-sql-driver/mysql"
)

var dashboardAdminSchemaSequence atomic.Int64

func TestDashboardAdminApprovalExecutionRealMariaDB(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
	createDashboardAdminProvisioningFixture(t, db)
	rootUserID := seedDashboardAdminProvisioningPackageAndActor(t, db)

	ctx := context.Background()
	store := NewMySQLStore(db)
	actor := dashboardadmin.NewSaaSApprovalExecutionActor(rootUserID)

	provisionRaw, err := json.Marshal(dashboardAdminProvisioningInput("approval-real-provision", "13800000101", "Approval real provision"))
	if err != nil {
		t.Fatal(err)
	}
	provisionResult := executeRealDashboardAdminApproval(t, ctx, db, store, actor, dashboardadmin.ApprovalActionTenantProvision, provisionRaw, "dashboard_tenant", "new")
	if !realApprovalResultHasPositiveID(provisionResult, "tenantId") || !realApprovalResultHasPositiveID(provisionResult, "dashboardUserId") {
		t.Fatal("provision approval result omitted business identifiers")
	}
	assertRealApprovalEffectAndAudits(t, db, provisionResult, "saas.admin.dashboard_tenant.provision", "dashboard.tenant.provision")

	resendSeed := dashboardAdminProvisioningInput("approval-real-resend-seed", "13800000102", "Approval real resend")
	resendSeedResult, err := store.ProvisionDashboardTenant(ctx, actor, resendSeed)
	if err != nil {
		t.Fatalf("seed resend target: %v", err)
	}
	resendRaw, err := json.Marshal(map[string]any{
		"tenantId":        resendSeedResult.TenantID,
		"targetUserId":    resendSeedResult.DashboardUserID,
		"expectedVersion": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	resendResult := executeRealDashboardAdminApproval(t, ctx, db, store, actor, dashboardadmin.ApprovalActionActivationResend, resendRaw, "dashboard_identity_activation", fmt.Sprintf("%d", resendSeedResult.DashboardUserID))
	if !realApprovalResultHasPositiveID(resendResult, "dashboardUserId") || !realApprovalResultHasPositiveVersion(resendResult, 2) {
		t.Fatalf("resend approval result omitted result version: %#v", resendResult)
	}
	assertRealApprovalEffectAndAudits(t, db, resendResult, "saas.admin.dashboard_activation.resend", "saas.admin.dashboard_activation.resend")

	replaceSeed := dashboardAdminProvisioningInput("approval-real-replace-seed", "13800000103", "Approval real replace")
	replaceSeedResult, err := store.ProvisionDashboardTenant(ctx, actor, replaceSeed)
	if err != nil {
		t.Fatalf("seed replacement target: %v", err)
	}
	activateProvisionedSubjectForGovernance(t, db, replaceSeedResult.DashboardUserID)
	replacementCandidateID := insertActivatedDashboardUser(t, db, replaceSeedResult.TenantID, "13800000104", "Approval replacement candidate", false)
	replaceRaw, err := json.Marshal(map[string]any{
		"tenantId":        replaceSeedResult.TenantID,
		"currentAdminId":  replaceSeedResult.DashboardUserID,
		"newAdminId":      replacementCandidateID,
		"expectedVersion": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	replaceResult := executeRealDashboardAdminApproval(t, ctx, db, store, actor, dashboardadmin.ApprovalActionSuperAdminReplace, replaceRaw, "dashboard_superadmin", fmt.Sprintf("%d", replacementCandidateID))
	if !realApprovalResultHasPositiveID(replaceResult, "dashboardUserId") || !realApprovalResultHasPositiveVersion(replaceResult, 2) {
		t.Fatal("replace approval result omitted result version")
	}
	assertRealApprovalEffectAndAudits(t, db, replaceResult, "saas.admin.dashboard_superadmin.replace", "saas.admin.dashboard_superadmin.replace")

	statusSeed := dashboardAdminProvisioningInput("approval-real-status-seed", "13800000105", "Approval real status")
	statusSeedResult, err := store.ProvisionDashboardTenant(ctx, actor, statusSeed)
	if err != nil {
		t.Fatalf("seed status target: %v", err)
	}
	activateProvisionedSubjectForGovernance(t, db, statusSeedResult.DashboardUserID)
	insertActivatedDashboardUser(t, db, statusSeedResult.TenantID, "13800000107", "Approval status backup", true)
	statusRaw, err := json.Marshal(map[string]any{
		"tenantId":        statusSeedResult.TenantID,
		"targetUserId":    statusSeedResult.DashboardUserID,
		"enabled":         false,
		"expectedVersion": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	statusResult := executeRealDashboardAdminApproval(t, ctx, db, store, actor, dashboardadmin.ApprovalActionSuperAdminStatus, statusRaw, "dashboard_superadmin", fmt.Sprintf("%d", statusSeedResult.DashboardUserID))
	if !realApprovalResultHasPositiveID(statusResult, "dashboardUserId") || !realApprovalResultHasPositiveVersion(statusResult, 2) {
		t.Fatal("status approval result omitted result version")
	}
	assertRealApprovalEffectAndAudits(t, db, statusResult, "saas.admin.dashboard_superadmin.status", "saas.admin.dashboard_superadmin.status")
}

func TestDashboardAdminApprovalEffectUpdateFailureRollsBackBusinessTransactionRealMariaDB(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
	createDashboardAdminProvisioningFixture(t, db)
	rootUserID := seedDashboardAdminProvisioningPackageAndActor(t, db)

	ctx := context.Background()
	store := NewMySQLStore(db)
	actor := dashboardadmin.NewSaaSApprovalExecutionActor(rootUserID)
	seedInput := dashboardAdminProvisioningInput("approval-real-rollback-seed", "13800000106", "Approval rollback")
	seed, err := store.ProvisionDashboardTenant(ctx, actor, seedInput)
	if err != nil {
		t.Fatalf("seed rollback target: %v", err)
	}
	activateProvisionedSubjectForGovernance(t, db, seed.DashboardUserID)
	raw, err := json.Marshal(map[string]any{
		"tenantId":        seed.TenantID,
		"targetUserId":    seed.DashboardUserID,
		"enabled":         false,
		"expectedVersion": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	started := createAndBeginRealDashboardAdminApproval(t, ctx, store, actor.UserID, dashboardadmin.ApprovalActionSuperAdminStatus, raw, "dashboard_superadmin", fmt.Sprintf("%d", seed.DashboardUserID))

	beforeCounts := dashboardAdminCounts(t, db)
	var beforeVersion uint64
	if err := db.QueryRow(`SELECT version FROM mochat_go_tenant_corp_bindings WHERE tenant_id=?`, seed.TenantID).Scan(&beforeVersion); err != nil {
		t.Fatal(err)
	}
	var beforeUserStatus, beforeIdentityStatus, beforeSuperAdmin int
	if err := db.QueryRow(`SELECT dashboard_user.status, identity_row.status, dashboard_user.isSuperAdmin FROM mc_user dashboard_user INNER JOIN mochat_go_dashboard_identities identity_row ON identity_row.user_id=dashboard_user.id WHERE dashboard_user.id=?`, seed.DashboardUserID).Scan(&beforeUserStatus, &beforeIdentityStatus, &beforeSuperAdmin); err != nil {
		t.Fatal(err)
	}
	var beforeOperationCount, beforeAuditCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE tenant_id=? AND action=?`, seed.TenantID, "saas.admin.dashboard_superadmin.status").Scan(&beforeOperationCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id=? AND action=?`, seed.TenantID, "saas.admin.dashboard_superadmin.status").Scan(&beforeAuditCount); err != nil {
		t.Fatal(err)
	}

	triggerName := "task7_approval_effect_failure"
	if _, err := db.Exec("CREATE TRIGGER `" + triggerName + "` AFTER INSERT ON mochat_go_dashboard_permission_audits FOR EACH ROW UPDATE mochat_go_saas_admin_approvals SET version = version + 1 WHERE id = " + fmt.Sprintf("%d", started.ID) + " AND status = 'executing'"); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = db.Exec("DROP TRIGGER IF EXISTS `" + triggerName + "`") }()

	if _, err := dashboardadmin.NewService(store).ExecuteApproval(ctx, actor, dashboardadmin.ApprovalActionSuperAdminStatus, raw, started.ID, started.Version); err == nil {
		t.Fatal("effect update failure unexpectedly committed")
	}

	afterCounts := dashboardAdminCounts(t, db)
	if !sameDashboardAdminCounts(afterCounts, beforeCounts) {
		t.Fatal("effect update failure left business artifacts behind")
	}
	var afterVersion uint64
	if err := db.QueryRow(`SELECT version FROM mochat_go_tenant_corp_bindings WHERE tenant_id=?`, seed.TenantID).Scan(&afterVersion); err != nil {
		t.Fatal(err)
	}
	if afterVersion != beforeVersion {
		t.Fatalf("binding version=%d, want unchanged %d", afterVersion, beforeVersion)
	}
	var afterUserStatus, afterIdentityStatus, afterSuperAdmin int
	if err := db.QueryRow(`SELECT dashboard_user.status, identity_row.status, dashboard_user.isSuperAdmin FROM mc_user dashboard_user INNER JOIN mochat_go_dashboard_identities identity_row ON identity_row.user_id=dashboard_user.id WHERE dashboard_user.id=?`, seed.DashboardUserID).Scan(&afterUserStatus, &afterIdentityStatus, &afterSuperAdmin); err != nil {
		t.Fatal(err)
	}
	if afterUserStatus != beforeUserStatus || afterIdentityStatus != beforeIdentityStatus || afterSuperAdmin != beforeSuperAdmin {
		t.Fatal("effect update failure left status mutation behind")
	}
	var afterOperationCount, afterAuditCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE tenant_id=? AND action=?`, seed.TenantID, "saas.admin.dashboard_superadmin.status").Scan(&afterOperationCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id=? AND action=?`, seed.TenantID, "saas.admin.dashboard_superadmin.status").Scan(&afterAuditCount); err != nil {
		t.Fatal(err)
	}
	if afterOperationCount != beforeOperationCount || afterAuditCount != beforeAuditCount {
		t.Fatal("effect update failure left audit mutation behind")
	}
	var status string
	var version int
	var effectApplied sql.NullTime
	if err := db.QueryRow(`SELECT status, version, effect_applied_at FROM mochat_go_saas_admin_approvals WHERE id=?`, started.ID).Scan(&status, &version, &effectApplied); err != nil {
		t.Fatal(err)
	}
	if status != dashboard.SaaSAdminApprovalStatusExecuting || version != started.Version || effectApplied.Valid {
		t.Fatal("failed effect did not leave approval lease unchanged")
	}
}

func createAndBeginRealDashboardAdminApproval(t *testing.T, ctx context.Context, store *MySQLStore, executionUserID int, action string, raw []byte, targetType, targetID string) dashboard.SaaSAdminApproval {
	t.Helper()
	digest := sha256.Sum256(raw)
	requestKey := fmt.Sprintf("task7-real-%d", time.Now().UnixNano())
	now := time.Now()
	created, err := store.CreateSaaSAdminApproval(ctx, dashboard.SaaSAdminApprovalCreate{
		RequestNo:          requestKey,
		ActionType:         action,
		RiskLevel:          "critical",
		RequiredPermission: dashboardadmin.PermissionTenantsManage,
		TargetType:         targetType,
		TargetID:           targetID,
		TargetName:         targetID,
		RequesterUserID:    10,
		RequesterTenantID:  1,
		IdempotencyKey:     requestKey,
		RequestSHA256:      fmt.Sprintf("%x", digest[:]),
		RequestJSON:        string(raw),
		Reason:             "Task7 real transaction effect test",
		ExpiresAt:          now.Add(time.Hour),
		PolicyVersion:      1,
		RequiredApprovals:  1,
		ReminderMinutes:    30,
		SLADueAt:           now.Add(30 * time.Minute),
		NextReminderAt:     now.Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create real approval action=%s: %v", action, err)
	}
	decided, err := store.DecideSaaSAdminApproval(ctx, dashboard.SaaSAdminApprovalDecision{
		ApprovalID:      created.Approval.ID,
		Decision:        dashboard.SaaSAdminApprovalDecisionApprove,
		Reason:          "independent reviewer approval",
		ExpectedVersion: created.Approval.Version,
		ActorUserID:     700,
		ActorTenantID:   1,
	})
	if err != nil {
		t.Fatalf("decide real approval action=%s: %v", action, err)
	}
	started, err := store.BeginSaaSAdminApprovalExecution(ctx, dashboard.SaaSAdminApprovalExecutionStart{
		ApprovalID:      decided.ID,
		ExpectedVersion: decided.Version,
		ActorUserID:     executionUserID,
		ActorTenantID:   1,
	})
	if err != nil {
		t.Fatalf("begin real approval action=%s: %v", action, err)
	}
	if started.Status != dashboard.SaaSAdminApprovalStatusExecuting || started.ExecutionUserID != executionUserID {
		t.Fatalf("started approval action=%s status=%s executor=%d", action, started.Status, started.ExecutionUserID)
	}
	return started
}

func executeRealDashboardAdminApproval(t *testing.T, ctx context.Context, db *sql.DB, store *MySQLStore, actor dashboardadmin.Actor, action string, raw []byte, targetType, targetID string) map[string]any {
	t.Helper()
	started := createAndBeginRealDashboardAdminApproval(t, ctx, store, actor.UserID, action, raw, targetType, targetID)
	result, err := dashboardadmin.NewService(store).ExecuteApproval(ctx, actor, action, raw, started.ID, started.Version)
	if err != nil {
		t.Fatalf("execute real approval action=%s: %v", action, err)
	}
	persistent := make(map[string]any, len(result))
	for key, value := range result {
		if key == "activationToken" {
			if token, ok := value.(string); ok && strings.TrimSpace(token) != "" {
				persistent["activationTokenDelivered"] = true
			}
			continue
		}
		persistent[key] = value
	}
	persistentJSON, err := json.Marshal(persistent)
	if err != nil {
		t.Fatalf("marshal persistent approval result action=%s: %v", action, err)
	}
	finished, err := store.FinishSaaSAdminApprovalExecution(ctx, dashboard.SaaSAdminApprovalExecutionFinish{
		ApprovalID:      started.ID,
		ExpectedVersion: started.Version,
		ActorUserID:     actor.UserID,
		ActorTenantID:   1,
		Success:         true,
		ResultJSON:      string(persistentJSON),
	})
	if err != nil {
		t.Fatalf("finish real approval action=%s: %v", action, err)
	}
	if finished.Status != dashboard.SaaSAdminApprovalStatusExecuted || finished.EffectOperationID <= 0 || finished.EffectAppliedAtValue.IsZero() {
		t.Fatalf("finished approval action=%s status=%s effectOperation=%d effectApplied=%t", action, finished.Status, finished.EffectOperationID, !finished.EffectAppliedAtValue.IsZero())
	}
	var persisted string
	if err := db.QueryRow(`SELECT COALESCE(CAST(result_json AS CHAR), '') FROM mochat_go_saas_admin_approvals WHERE id=?`, started.ID).Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(persisted, "activationToken") && !strings.Contains(persisted, "activationTokenDelivered") {
		t.Fatal("persistent approval result contains a raw activation field")
	}
	result["_approvalID"] = started.ID
	return result
}

func realApprovalResultHasPositiveID(result map[string]any, key string) bool {
	value, ok := result[key]
	if !ok {
		return false
	}
	switch number := value.(type) {
	case int:
		return number > 0
	case int64:
		return number > 0
	case float64:
		return number > 0
	default:
		return false
	}
}

func realApprovalResultHasPositiveVersion(result map[string]any, expected int) bool {
	value, ok := result["version"]
	if !ok {
		return false
	}
	switch number := value.(type) {
	case int:
		return number == expected
	case int64:
		return number == int64(expected)
	case uint:
		return number == uint(expected)
	case uint64:
		return number == uint64(expected)
	case float64:
		return int(number) == expected
	default:
		return false
	}
}

func assertRealApprovalEffectAndAudits(t *testing.T, db *sql.DB, result map[string]any, operationAction, auditAction string) {
	t.Helper()
	approvalID, ok := result["_approvalID"].(int64)
	if !ok || approvalID <= 0 {
		t.Fatal("real approval result missing test approval id")
	}
	tenantID, ok := realApprovalResultInt(result, "tenantId")
	if !ok || tenantID <= 0 {
		t.Fatal("real approval result missing tenant id")
	}
	var effectOperationID int64
	var effectApplied sql.NullTime
	var status string
	if err := db.QueryRow(`SELECT status, effect_applied_at, effect_operation_id FROM mochat_go_saas_admin_approvals WHERE id=?`, approvalID).Scan(&status, &effectApplied, &effectOperationID); err != nil {
		t.Fatal(err)
	}
	if status != dashboard.SaaSAdminApprovalStatusExecuted || !effectApplied.Valid || effectOperationID <= 0 {
		t.Fatalf("approval effect status=%s applied=%t operation=%d", status, effectApplied.Valid, effectOperationID)
	}
	var operationCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id=? AND tenant_id=? AND action=?`, effectOperationID, tenantID, operationAction).Scan(&operationCount); err != nil {
		t.Fatal(err)
	}
	if operationCount != 1 {
		t.Fatalf("effect operation count=%d, want 1", operationCount)
	}
	var auditCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id=? AND action=?`, tenantID, auditAction).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("dashboard audit count=%d, want 1", auditCount)
	}
	var dashboardRealmActorCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id=? AND action=? AND actor_user_id IS NOT NULL`, tenantID, auditAction).Scan(&dashboardRealmActorCount); err != nil {
		t.Fatal(err)
	}
	if dashboardRealmActorCount != 0 {
		t.Fatal("SaaS actor was written into the Dashboard mc_user audit foreign-key column")
	}
}

func realApprovalResultInt(result map[string]any, key string) (int, bool) {
	value, ok := result[key]
	if !ok {
		return 0, false
	}
	switch number := value.(type) {
	case int:
		return number, true
	case int64:
		return int(number), true
	case float64:
		return int(number), true
	default:
		return 0, false
	}
}

func TestDashboardAdminProvisioningRealMariaDB(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
	createDashboardAdminProvisioningFixture(t, db)
	rootUserID := seedDashboardAdminProvisioningPackageAndActor(t, db)

	ctx := context.Background()
	store := NewMySQLStore(db)
	actor := dashboardadmin.Actor{UserID: rootUserID, Active: true, Permissions: []string{dashboardadmin.PermissionTenantsManage}}
	input := dashboardAdminProvisioningInput("provision-task7-1", "13800000099", "Task 7 Tenant")

	before := dashboardAdminCounts(t, db)
	first, err := store.ProvisionDashboardTenant(ctx, actor, input)
	if err != nil {
		t.Fatalf("real provisioning: %v", err)
	}
	if first.ActivationToken == "" || first.Idempotent {
		t.Fatal("first provisioning did not return exactly one activation value")
	}
	after := dashboardAdminCounts(t, db)
	for table, wantDelta := range map[string]int{"mc_tenant": 1, "mc_corp": 1, "mochat_go_tenant_corp_bindings": 1, "mochat_go_saas_tenant_packages": 1, "mochat_go_saas_subscriptions": 1, "mochat_go_saas_subscription_events": 1, "mc_user": 1, "mochat_go_dashboard_identities": 1, "mochat_go_dashboard_identity_activations": 1, "mochat_go_tenant_provision_runs": 1, "mochat_go_saas_admin_operation_logs": 1, "mochat_go_dashboard_permission_audits": 1} {
		if delta := after[table] - before[table]; delta != wantDelta {
			t.Fatalf("%s delta=%d, want %d", table, delta, wantDelta)
		}
	}
	var digest []byte
	if err := db.QueryRow(`SELECT token_digest FROM mochat_go_dashboard_identity_activations WHERE user_id=?`, first.DashboardUserID).Scan(&digest); err != nil {
		t.Fatal(err)
	}
	expectedDigest := sha256.Sum256([]byte(first.ActivationToken))
	if string(digest) != string(expectedDigest[:]) {
		t.Fatal("activation persistence is not the SHA-256 digest of the one-time value")
	}
	var bindingStatus int
	var verifiedCorpName string
	if err := db.QueryRow(`SELECT status, verified_corp_name FROM mochat_go_tenant_corp_bindings WHERE tenant_id=?`, first.TenantID).Scan(&bindingStatus, &verifiedCorpName); err != nil {
		t.Fatal(err)
	}
	if bindingStatus != 1 || verifiedCorpName != "" {
		t.Fatalf("pending binding status=%d verifiedCorpName=%q, want status=1 and blank authority", bindingStatus, verifiedCorpName)
	}
	var limitCount int
	if err := db.QueryRow(`SELECT JSON_LENGTH(limits_json) FROM mochat_go_saas_tenant_packages WHERE tenant_id=?`, first.TenantID).Scan(&limitCount); err != nil {
		t.Fatal(err)
	}
	if limitCount != len(packageLimitJSONKeysForIntegration) {
		t.Fatalf("quota snapshot fields=%d, want %d", limitCount, len(packageLimitJSONKeysForIntegration))
	}
	var auditText string
	if err := db.QueryRow(`SELECT CONCAT(COALESCE(before_json,''), COALESCE(after_json,''), remark) FROM mochat_go_saas_admin_operation_logs WHERE tenant_id=? ORDER BY id DESC LIMIT 1`, first.TenantID).Scan(&auditText); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(auditText, first.ActivationToken) {
		t.Fatal("raw activation value entered the SaaS audit")
	}

	second, err := store.ProvisionDashboardTenant(ctx, actor, input)
	if err != nil {
		t.Fatalf("idempotent provisioning: %v", err)
	}
	if !second.Idempotent || second.ActivationToken != "" {
		t.Fatal("idempotent provisioning replayed or regenerated the original activation value")
	}
	if got := dashboardAdminCounts(t, db); !sameDashboardAdminCounts(got, after) {
		t.Fatal("idempotent provisioning changed durable artifact counts")
	}

	conflictInput := input
	conflictInput.TenantName = "different tenant input"
	if _, err := store.ProvisionDashboardTenant(ctx, actor, conflictInput); !errorsIsDashboardAdmin(err, dashboardadmin.ErrIdempotencyConflict) {
		t.Fatalf("same key with different business input error=%v, want ErrIdempotencyConflict", err)
	}
	duplicatePhone := input
	duplicatePhone.IdempotencyKey = "provision-task7-duplicate-phone"
	if _, err := store.ProvisionDashboardTenant(ctx, actor, duplicatePhone); !errorsIsDashboardAdmin(err, dashboardadmin.ErrLoginIdentifierConflict) {
		t.Fatalf("duplicate phone error=%v, want ErrLoginIdentifierConflict", err)
	}
	if got := dashboardAdminCounts(t, db); !sameDashboardAdminCounts(got, after) {
		t.Fatal("conflict paths wrote durable artifacts")
	}

	resendInput := dashboardadmin.ResendActivationInput{TenantID: first.TenantID, TargetUserID: first.DashboardUserID, ExpectedVersion: 1, RequestID: "resend-task7-concurrent"}
	var resendResults [2]dashboardadmin.ResendActivationResult
	var resendErrors [2]error
	var resendWait sync.WaitGroup
	for index := range resendResults {
		resendWait.Add(1)
		go func(index int) {
			defer resendWait.Done()
			resendResults[index], resendErrors[index] = store.ResendDashboardActivation(ctx, actor, resendInput)
		}(index)
	}
	resendWait.Wait()
	for index, resendErr := range resendErrors {
		if resendErr != nil {
			t.Fatalf("concurrent resend[%d]: %v", index, resendErr)
		}
	}
	if (resendResults[0].ActivationToken == "") == (resendResults[1].ActivationToken == "") {
		t.Fatalf("concurrent resend raw-token presence=%t/%t, want exactly one first response", resendResults[0].ActivationToken != "", resendResults[1].ActivationToken != "")
	}
	for index, result := range resendResults {
		if result.TenantID != first.TenantID || result.DashboardUserID != first.DashboardUserID || result.Version != 2 {
			t.Fatalf("concurrent resend[%d] tenant=%d user=%d version=%d idempotent=%t, want resultVersion=2", index, result.TenantID, result.DashboardUserID, result.Version, result.Idempotent)
		}
	}
	var activationCount, pendingActivationCount int
	if err := db.QueryRow(`SELECT COUNT(*), SUM(consumed_at IS NULL) FROM mochat_go_dashboard_identity_activations WHERE user_id=?`, first.DashboardUserID).Scan(&activationCount, &pendingActivationCount); err != nil {
		t.Fatal(err)
	}
	if activationCount != 2 || pendingActivationCount != 1 {
		t.Fatalf("activation rows=%d pending=%d, want one old consumed and one new pending", activationCount, pendingActivationCount)
	}
	var bindingVersion uint64
	if err := db.QueryRow(`SELECT version FROM mochat_go_tenant_corp_bindings WHERE tenant_id=?`, first.TenantID).Scan(&bindingVersion); err != nil {
		t.Fatal(err)
	}
	if bindingVersion != 2 {
		t.Fatalf("concurrent resend binding version=%d, want one increment", bindingVersion)
	}
	var receiptCount, receiptStatus int
	var receiptVersion uint64
	if err := db.QueryRow(`SELECT COUNT(*), MAX(status), MAX(result_version) FROM mochat_go_saas_idempotency_receipts WHERE operation=? AND request_key=?`, "dashboard_activation_resend", resendInput.RequestID).Scan(&receiptCount, &receiptStatus, &receiptVersion); err != nil {
		t.Fatal(err)
	}
	if receiptCount != 1 || receiptStatus != 1 || receiptVersion != 2 {
		t.Fatalf("resend receipt count=%d status=%d version=%d, want one committed receipt at version 2", receiptCount, receiptStatus, receiptVersion)
	}
	var resendOperationCount, resendAuditCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE tenant_id=? AND action=?`, first.TenantID, "saas.admin.dashboard_activation.resend").Scan(&resendOperationCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id=? AND action=?`, first.TenantID, "saas.admin.dashboard_activation.resend").Scan(&resendAuditCount); err != nil {
		t.Fatal(err)
	}
	if resendOperationCount != 1 || resendAuditCount != 1 {
		t.Fatalf("resend audit counts SaaS=%d Dashboard=%d, want one each", resendOperationCount, resendAuditCount)
	}
	if _, err := store.ResendDashboardActivation(ctx, actor, dashboardadmin.ResendActivationInput{TenantID: first.TenantID, TargetUserID: first.DashboardUserID, ExpectedVersion: 99, RequestID: resendInput.RequestID}); !errorsIsDashboardAdmin(err, dashboardadmin.ErrIdempotencyConflict) {
		t.Fatalf("same resend key with changed fingerprint error=%v, want ErrIdempotencyConflict", err)
	}

	if _, err := db.Exec(`INSERT INTO mc_user (id, phone, name, status, tenant_id, isSuperAdmin, created_at, updated_at, deleted_at) VALUES (701, '13800000095', 'Legacy-only superadmin', 1, 1, 1, NOW(), NOW(), NULL)`); err != nil {
		t.Fatal(err)
	}
	legacyBefore := dashboardAdminCounts(t, db)
	legacyInput := dashboardAdminProvisioningInput("provision-task7-legacy-id", "13800000094", "Legacy actor must fail")
	if _, err := store.ProvisionDashboardTenant(ctx, dashboardadmin.Actor{UserID: 701, Active: true, Permissions: []string{dashboardadmin.PermissionTenantsManage}}, legacyInput); !errorsIsDashboardAdmin(err, dashboardadmin.ErrPermissionDenied) {
		t.Fatalf("same-ID legacy mc_user superadmin error=%v, want ErrPermissionDenied", err)
	}
	if got := dashboardAdminCounts(t, db); !sameDashboardAdminCounts(got, legacyBefore) {
		t.Fatal("legacy mc_user superadmin fallback wrote provisioning artifacts")
	}

	activateProvisionedSubjectForGovernance(t, db, first.DashboardUserID)
	secondAdminID := insertActivatedDashboardUser(t, db, first.TenantID, "13800000098", "Second admin", false)
	replacementInput := dashboardadmin.ReplaceSuperAdminInput{TenantID: first.TenantID, CurrentAdminID: first.DashboardUserID, NewAdminID: secondAdminID, ExpectedVersion: 2, RequestID: "task7-replace"}
	var replacementResults [2]dashboardadmin.GovernanceResult
	var replacementErrors [2]error
	var replacementWait sync.WaitGroup
	for index := range replacementResults {
		replacementWait.Add(1)
		go func(index int) {
			defer replacementWait.Done()
			replacementResults[index], replacementErrors[index] = store.ReplaceDashboardSuperAdmin(ctx, actor, replacementInput)
		}(index)
	}
	replacementWait.Wait()
	for index, replaceErr := range replacementErrors {
		if replaceErr != nil {
			t.Fatalf("concurrent replacement[%d]: %v", index, replaceErr)
		}
		if replacementResults[index].Version != 3 {
			t.Fatalf("concurrent replacement[%d] version=%d, want one resultVersion=3", index, replacementResults[index].Version)
		}
	}
	replacementRetry, err := store.ReplaceDashboardSuperAdmin(ctx, actor, replacementInput)
	if err != nil || !replacementRetry.Idempotent || replacementRetry.Version != 3 {
		t.Fatalf("replacement retry tenant=%d user=%d version=%d idempotent=%t error=%v, want idempotent original version 3", replacementRetry.TenantID, replacementRetry.DashboardUserID, replacementRetry.Version, replacementRetry.Idempotent, err)
	}
	var replacementReceiptCount, replacementAuditCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_saas_idempotency_receipts WHERE operation=? AND request_key=?`, "dashboard_superadmin_replace", replacementInput.RequestID).Scan(&replacementReceiptCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id=? AND action=?`, first.TenantID, "saas.admin.dashboard_superadmin.replace").Scan(&replacementAuditCount); err != nil {
		t.Fatal(err)
	}
	if replacementReceiptCount != 1 || replacementAuditCount != 1 {
		t.Fatalf("replacement receipt/audit count=%d/%d, want one each", replacementReceiptCount, replacementAuditCount)
	}
	assertSuperAdminFlags(t, db, first.TenantID, first.DashboardUserID, 0, 1)

	thirdAdminID := insertActivatedDashboardUser(t, db, first.TenantID, "13800000097", "Third admin", true)
	if _, err := store.ReplaceDashboardSuperAdmin(ctx, actor, dashboardadmin.ReplaceSuperAdminInput{TenantID: first.TenantID, CurrentAdminID: first.DashboardUserID, NewAdminID: thirdAdminID, ExpectedVersion: 2, RequestID: replacementInput.RequestID}); !errorsIsDashboardAdmin(err, dashboardadmin.ErrIdempotencyConflict) {
		t.Fatalf("replacement request-key fingerprint error=%v, want ErrIdempotencyConflict", err)
	}
	statusInput := dashboardadmin.SuperAdminStatusInput{TenantID: first.TenantID, TargetUserID: secondAdminID, Enabled: false, ExpectedVersion: 3, RequestID: "task7-disable"}
	var statusResults [2]dashboardadmin.GovernanceResult
	var statusErrors [2]error
	var statusWait sync.WaitGroup
	for index := range statusResults {
		statusWait.Add(1)
		go func(index int) {
			defer statusWait.Done()
			statusResults[index], statusErrors[index] = store.SetDashboardSuperAdminStatus(ctx, actor, statusInput)
		}(index)
	}
	statusWait.Wait()
	for index, statusErr := range statusErrors {
		if statusErr != nil {
			t.Fatalf("concurrent status[%d]: %v", index, statusErr)
		}
		if statusResults[index].Version != 4 {
			t.Fatalf("concurrent status[%d] version=%d, want one resultVersion=4", index, statusResults[index].Version)
		}
	}
	statusRetry, err := store.SetDashboardSuperAdminStatus(ctx, actor, statusInput)
	if err != nil || !statusRetry.Idempotent || statusRetry.Version != 4 {
		t.Fatalf("status retry tenant=%d user=%d version=%d idempotent=%t error=%v, want idempotent original version 4", statusRetry.TenantID, statusRetry.DashboardUserID, statusRetry.Version, statusRetry.Idempotent, err)
	}
	var statusReceiptCount, statusAuditCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_saas_idempotency_receipts WHERE operation=? AND request_key=?`, "dashboard_superadmin_status", statusInput.RequestID).Scan(&statusReceiptCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id=? AND action=?`, first.TenantID, "saas.admin.dashboard_superadmin.status").Scan(&statusAuditCount); err != nil {
		t.Fatal(err)
	}
	if statusReceiptCount != 1 || statusAuditCount != 1 {
		t.Fatalf("status receipt/audit count=%d/%d, want one each", statusReceiptCount, statusAuditCount)
	}
	if _, err := store.SetDashboardSuperAdminStatus(ctx, actor, dashboardadmin.SuperAdminStatusInput{TenantID: first.TenantID, TargetUserID: secondAdminID, Enabled: true, ExpectedVersion: 3, RequestID: statusInput.RequestID}); !errorsIsDashboardAdmin(err, dashboardadmin.ErrIdempotencyConflict) {
		t.Fatalf("status request-key fingerprint error=%v, want ErrIdempotencyConflict", err)
	}
	assertSuperAdminFlags(t, db, first.TenantID, secondAdminID, 1, 2)
	statusNoop, err := store.SetDashboardSuperAdminStatus(ctx, actor, dashboardadmin.SuperAdminStatusInput{TenantID: first.TenantID, TargetUserID: secondAdminID, Enabled: false, ExpectedVersion: 4, RequestID: "task7-disable-noop"})
	if err != nil || statusNoop.Idempotent || statusNoop.Version != 4 {
		t.Fatalf("disabled-state no-op tenant=%d user=%d version=%d idempotent=%t error=%v, want original version 4 without mutation", statusNoop.TenantID, statusNoop.DashboardUserID, statusNoop.Version, statusNoop.Idempotent, err)
	}
	restored, err := store.SetDashboardSuperAdminStatus(ctx, actor, dashboardadmin.SuperAdminStatusInput{TenantID: first.TenantID, TargetUserID: secondAdminID, Enabled: true, ExpectedVersion: 4, RequestID: "task7-restore"})
	if err != nil {
		t.Fatalf("restore original super administrator: %v", err)
	}
	if restored.Version != 5 {
		t.Fatalf("restore binding version=%d, want 5", restored.Version)
	}
	assertSuperAdminFlags(t, db, first.TenantID, secondAdminID, 1, 1)
	activeNoop, err := store.SetDashboardSuperAdminStatus(ctx, actor, dashboardadmin.SuperAdminStatusInput{TenantID: first.TenantID, TargetUserID: secondAdminID, Enabled: true, ExpectedVersion: 5, RequestID: "task7-restore-noop"})
	if err != nil || activeNoop.Idempotent || activeNoop.Version != 5 {
		t.Fatalf("active-state no-op tenant=%d user=%d version=%d idempotent=%t error=%v, want original version 5 without mutation", activeNoop.TenantID, activeNoop.DashboardUserID, activeNoop.Version, activeNoop.Idempotent, err)
	}
	if _, err := store.SetDashboardSuperAdminStatus(ctx, actor, dashboardadmin.SuperAdminStatusInput{TenantID: first.TenantID, TargetUserID: thirdAdminID, Enabled: false, ExpectedVersion: 5, RequestID: "task7-last-disable"}); err != nil {
		t.Fatalf("disable third while two valid administrators exist: %v", err)
	}
	if _, err := store.SetDashboardSuperAdminStatus(ctx, actor, dashboardadmin.SuperAdminStatusInput{TenantID: first.TenantID, TargetUserID: secondAdminID, Enabled: false, ExpectedVersion: 6, RequestID: "task7-last-conflict"}); !errorsIsDashboardAdmin(err, dashboardadmin.ErrLastSuperAdmin) {
		t.Fatalf("last super administrator error=%v, want ErrLastSuperAdmin", err)
	}

	governanceView, err := store.DashboardAdminGovernance(ctx, actor, first.TenantID)
	if err != nil {
		t.Fatalf("durable dashboard governance read: %v", err)
	}
	if governanceView.TenantID != first.TenantID || governanceView.BindingVersion != 6 {
		t.Fatalf("governance view tenant/version=%d/%d, want %d/6", governanceView.TenantID, governanceView.BindingVersion, first.TenantID)
	}
	var firstIdentity, secondIdentity, thirdIdentity *dashboardadmin.DashboardIdentityRecord
	for index := range governanceView.Identities {
		identity := &governanceView.Identities[index]
		switch identity.ID {
		case first.DashboardUserID:
			firstIdentity = identity
		case secondAdminID:
			secondIdentity = identity
		case thirdAdminID:
			thirdIdentity = identity
		}
	}
	if firstIdentity == nil || firstIdentity.IsSuperAdmin || firstIdentity.ActivatedAt == "" {
		t.Fatalf("replaced ordinary activated identity facts=%+v, want non-superadmin with activation", firstIdentity)
	}
	if secondIdentity == nil || !secondIdentity.IsSuperAdmin || secondIdentity.UserStatus != 1 || secondIdentity.IdentityStatus != 1 {
		t.Fatalf("active superadmin facts=%+v, want active superadmin", secondIdentity)
	}
	if thirdIdentity == nil || !thirdIdentity.IsSuperAdmin || thirdIdentity.UserStatus == 1 || thirdIdentity.IdentityStatus == 1 {
		t.Fatalf("disabled superadmin facts=%+v, want retained superadmin marker and disabled status", thirdIdentity)
	}
	if _, err := store.DashboardAdminGovernance(ctx, actor, 2); !errorsIsDashboardAdmin(err, dashboardadmin.ErrTargetNotFound) {
		t.Fatalf("unbound cross-tenant governance read error=%v, want ErrTargetNotFound", err)
	}

	if _, err := db.Exec(`DROP TABLE mochat_go_dashboard_permission_audits`); err != nil {
		t.Fatal(err)
	}
	rollbackBefore := dashboardAdminCountsWithoutDashboardAudit(t, db)
	rollbackInput := dashboardAdminProvisioningInput("provision-task7-rollback", "13800000096", "Rollback tenant")
	if _, err := store.ProvisionDashboardTenant(ctx, actor, rollbackInput); err == nil {
		t.Fatal("missing second audit table unexpectedly allowed a partial provisioning commit")
	}
	countsAfterRollback := dashboardAdminCountsWithoutDashboardAudit(t, db)
	if !sameDashboardAdminCounts(countsAfterRollback, rollbackBefore) {
		t.Fatalf("failed transaction changed durable artifacts: before=%v after=%v", rollbackBefore, countsAfterRollback)
	}
}

func dashboardAdminProvisioningInput(key, phone, tenantName string) dashboardadmin.ProvisionDashboardTenant {
	return dashboardadmin.ProvisionDashboardTenant{
		TenantName:           tenantName,
		WeComIntegrationMode: dashboardadmin.WeComIntegrationModeSelfBuilt,
		PackageID:            11,
		Limits:               dashboardAdminIntegrationLimits(),
		Subscription:         dashboardadmin.SubscriptionInput{PackageCode: "pro", Status: "trialing", BillingCycle: "custom", StartsAt: "2026-08-11T00:00:00Z", ExpiresAt: "2026-09-11T00:00:00Z"},
		AdminLoginIdentifier: phone,
		AdminName:            "Provisioned administrator",
		IdempotencyKey:       key,
		ExpectedVersion:      1,
		RequestID:            key,
	}
}

func dashboardAdminIntegrationLimits() dashboardadmin.SaaSAdminPackageLimits {
	return dashboardadmin.SaaSAdminPackageLimits{MaxCorps: 5, MaxUsers: 5, MaxContacts: 5, MaxRooms: 5, MaxAgents: 5, ChannelCodes: 5, ShopCodes: 5, Radars: 5, Lotteries: 5, RoomInfinitePulls: 5, RoomFissions: 5, RoomClockIns: 5, RoomQualities: 5, RoomCalendars: 5, RoomReminds: 5, ContactSOPs: 5, RoomSOPs: 5, SensitiveWords: 5, StorageMB: 5, ContactMessageBatches: 5, RoomMessageBatches: 5, RoomTagPulls: 5, WorkRoomAutoPulls: 5, WorkFissions: 5, OfficialAccounts: 5, AsyncExecutions: 5}
}

var packageLimitJSONKeysForIntegration = []string{"maxCorps", "maxUsers", "maxContacts", "maxRooms", "maxAgents", "channelCodes", "shopCodes", "radars", "lotteries", "roomInfinitePulls", "roomFissions", "roomClockIns", "roomQualities", "roomCalendars", "roomReminds", "contactSops", "roomSops", "sensitiveWords", "storageMb", "contactMessageBatches", "roomMessageBatches", "roomTagPulls", "workRoomAutoPulls", "workFissions", "officialAccounts", "asyncExecutions"}

func seedDashboardAdminProvisioningPackageAndActor(t *testing.T, db *sql.DB) int {
	t.Helper()
	limits := dashboardAdminIntegrationLimits()
	if _, err := db.Exec(`INSERT INTO mochat_go_saas_packages (id, code, name, status, version, max_corps, max_users, max_contacts, max_rooms, max_agents, channel_codes, shop_codes, radars, lotteries, room_infinite_pulls, room_fissions, room_clock_ins, room_qualities, room_calendars, room_reminds, contact_sops, room_sops, sensitive_words, storage_mb, contact_message_batches, room_message_batches, room_tag_pulls, work_room_auto_pulls, work_fissions, official_accounts, async_executions) VALUES (11, 'pro', 'Pro', 1, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, limits.MaxCorps, limits.MaxUsers, limits.MaxContacts, limits.MaxRooms, limits.MaxAgents, limits.ChannelCodes, limits.ShopCodes, limits.Radars, limits.Lotteries, limits.RoomInfinitePulls, limits.RoomFissions, limits.RoomClockIns, limits.RoomQualities, limits.RoomCalendars, limits.RoomReminds, limits.ContactSOPs, limits.RoomSOPs, limits.SensitiveWords, limits.StorageMB, limits.ContactMessageBatches, limits.RoomMessageBatches, limits.RoomTagPulls, limits.WorkRoomAutoPulls, limits.WorkFissions, limits.OfficialAccounts, limits.AsyncExecutions); err != nil {
		t.Fatal(err)
	}
	identity, err := NewSaaSIdentityStore(db).Bootstrap(context.Background(), saasauth.BootstrapSaaSAdmin{
		RequestKey:   "task7-fixture-root",
		LoginName:    "fixture-platform",
		Name:         "Platform fixture",
		PasswordHash: "!fixture-disabled",
	})
	if err != nil {
		t.Fatalf("create SaaS root fixture: %v", err)
	}
	if identity.ID <= 0 {
		t.Fatalf("fixture SaaS root id=%d, want a persisted auto-increment identity", identity.ID)
	}
	return identity.ID
}

func activateProvisionedSubjectForGovernance(t *testing.T, db *sql.DB, userID int) {
	t.Helper()
	if _, err := db.Exec(`UPDATE mochat_go_dashboard_identities SET password_hash='!fixture-disabled', must_rotate_password=0, activated_at=NOW(), status=1 WHERE user_id=?`, userID); err != nil {
		t.Fatal(err)
	}
}

func insertActivatedDashboardUser(t *testing.T, db *sql.DB, tenantID int, phone, name string, superAdmin bool) int {
	t.Helper()
	flag := 0
	if superAdmin {
		flag = 1
	}
	result, err := db.Exec(`INSERT INTO mc_user (phone, name, status, tenant_id, isSuperAdmin, created_at, updated_at, deleted_at) VALUES (?, ?, 1, ?, ?, NOW(), NOW(), NULL)`, phone, name, tenantID, flag)
	if err != nil {
		t.Fatal(err)
	}
	userID64, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	userID := int(userID64)
	if _, err := db.Exec(`INSERT INTO mochat_go_dashboard_identities (user_id, login_identifier, password_hash, status, must_rotate_password, auth_version, mfa_required, activated_at) VALUES (?, ?, '!fixture-disabled', 1, 0, 1, 0, NOW())`, userID, phone); err != nil {
		t.Fatal(err)
	}
	return userID
}

func assertSuperAdminFlags(t *testing.T, db *sql.DB, tenantID, userID, superAdmin, identityStatus int) {
	t.Helper()
	var gotSuperAdmin, gotStatus int
	if err := db.QueryRow(`SELECT dashboard_user.isSuperAdmin, identity_row.status FROM mc_user dashboard_user INNER JOIN mochat_go_dashboard_identities identity_row ON identity_row.user_id=dashboard_user.id WHERE dashboard_user.tenant_id=? AND dashboard_user.id=?`, tenantID, userID).Scan(&gotSuperAdmin, &gotStatus); err != nil {
		t.Fatal(err)
	}
	if gotSuperAdmin != superAdmin || gotStatus != identityStatus {
		t.Fatalf("user=%d superAdmin=%d identityStatus=%d, want %d/%d", userID, gotSuperAdmin, gotStatus, superAdmin, identityStatus)
	}
}

func errorsIsDashboardAdmin(err, target error) bool {
	return err != nil && (err == target || strings.Contains(err.Error(), target.Error()))
}

func dashboardAdminCounts(t *testing.T, db *sql.DB) map[string]int {
	t.Helper()
	return dashboardAdminCountsForTables(t, db, []string{"mc_tenant", "mc_corp", "mochat_go_tenant_corp_bindings", "mochat_go_saas_tenant_packages", "mochat_go_saas_subscriptions", "mochat_go_saas_subscription_events", "mc_user", "mochat_go_dashboard_identities", "mochat_go_dashboard_identity_activations", "mochat_go_saas_idempotency_receipts", "mochat_go_tenant_provision_runs", "mochat_go_saas_admin_operation_logs", "mochat_go_dashboard_permission_audits"})
}

func dashboardAdminCountsWithoutDashboardAudit(t *testing.T, db *sql.DB) map[string]int {
	t.Helper()
	return dashboardAdminCountsForTables(t, db, []string{"mc_tenant", "mc_corp", "mochat_go_tenant_corp_bindings", "mochat_go_saas_tenant_packages", "mochat_go_saas_subscriptions", "mochat_go_saas_subscription_events", "mc_user", "mochat_go_dashboard_identities", "mochat_go_dashboard_identity_activations", "mochat_go_saas_idempotency_receipts", "mochat_go_tenant_provision_runs", "mochat_go_saas_admin_operation_logs"})
}

func dashboardAdminCountsForTables(t *testing.T, db *sql.DB, tables []string) map[string]int {
	t.Helper()
	counts := make(map[string]int, len(tables))
	for _, table := range tables {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		counts[table] = count
	}
	return counts
}

func sameDashboardAdminCounts(left, right map[string]int) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func newDashboardAdminProvisioningDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB DSN is required")
	}
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	adminCfg := *cfg
	adminCfg.DBName = ""
	admin, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.PingContext(context.Background()); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	schema := fmt.Sprintf("mochat_identity_single_corp_task7_%d_%d", os.Getpid(), dashboardAdminSchemaSequence.Add(1))
	var schemaExists int
	if err := admin.QueryRow(`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name=?`, schema).Scan(&schemaExists); err != nil {
		_ = admin.Close()
		t.Fatalf("check isolated schema collision: %v", err)
	}
	if schemaExists != 0 {
		_ = admin.Close()
		t.Fatalf("isolated schema already exists: %s", schema)
	}
	if _, err := admin.Exec("CREATE DATABASE `" + schema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + schema + "`")
		var leftovers int
		if err := admin.QueryRow(`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name=?`, schema).Scan(&leftovers); err != nil {
			t.Errorf("check isolated schema cleanup: %v", err)
		} else if leftovers != 0 {
			t.Errorf("isolated schema %s still exists after cleanup", schema)
		}
		t.Logf("isolated schema cleanup=0 (%s)", schema)
		_ = admin.Close()
	})
	testCfg := *cfg
	testCfg.DBName = schema
	db, err := sql.Open("mysql", testCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func dashboardAdminSchemaLeftovers(t *testing.T) int {
	t.Helper()
	dsn := os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN")
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	adminCfg := *cfg
	adminCfg.DBName = ""
	admin, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	return dashboardAdminSchemaLeftoversWithDB(t, admin)
}

func dashboardAdminSchemaLeftoversWithDB(t *testing.T, admin *sql.DB) int {
	t.Helper()
	var count int
	if err := admin.QueryRow(`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name LIKE 'mochat_identity_single_corp_task7_%'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func createDashboardAdminProvisioningFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		`INSERT INTO mc_tenant (id, name, status) VALUES (1, 'Fixture tenant 1', 1), (2, 'Fixture tenant 2', 1)`,
		`INSERT INTO mc_corp (id, tenant_id, name) VALUES (100, 1, 'Fixture corp 1'), (200, 2, 'Fixture corp 2')`,
		`INSERT INTO mc_user (id, tenant_id, phone, status, deleted_at) VALUES (10, 1, '13800000001', 1, NULL)`,
		`INSERT INTO mc_rbac_role (id, tenant_id, operate_id, operate_name) VALUES (20, 1, 10, 'Fixture actor')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("base fixture: %v", err)
		}
	}
}
