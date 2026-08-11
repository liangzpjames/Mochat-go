package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"jiyi/mochat-go/internal/dashboardadmin"
	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/saasauth"

	mysqldriver "github.com/go-sql-driver/mysql"
)

var dashboardAdminSchemaSequence atomic.Int64

func TestDashboardAdminProvisioningRealMariaDB(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createDashboardAdminProvisioningFixture(t, db)
	seedDashboardAdminProvisioningPackageAndActor(t, db)

	ctx := context.Background()
	store := NewMySQLStore(db)
	actor := dashboardadmin.Actor{UserID: 700, Active: true, Permissions: []string{dashboardadmin.PermissionTenantsManage}}
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
	rollbackInput := dashboardAdminProvisioningInput("provision-task7-rollback", "13800000096", "Rollback tenant")
	if _, err := store.ProvisionDashboardTenant(ctx, actor, rollbackInput); err == nil {
		t.Fatal("missing second audit table unexpectedly allowed a partial provisioning commit")
	}
	countsAfterRollback := dashboardAdminCountsWithoutDashboardAudit(t, db)
	if countsAfterRollback["mc_tenant"] != after["mc_tenant"] || countsAfterRollback["mc_user"] != after["mc_user"]+2 {
		t.Fatal("failed transaction left tenant or user artifacts behind")
	}
}

func dashboardAdminProvisioningInput(key, phone, tenantName string) dashboardadmin.ProvisionDashboardTenant {
	return dashboardadmin.ProvisionDashboardTenant{
		TenantName:           tenantName,
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

func seedDashboardAdminProvisioningPackageAndActor(t *testing.T, db *sql.DB) {
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
	if identity.ID != 700 {
		t.Fatalf("fixture SaaS root id=%d, want 700", identity.ID)
	}
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
	baseline := dashboardAdminSchemaLeftoversWithDB(t, admin)
	schema := fmt.Sprintf("mochat_identity_single_corp_task7_%d_%d", os.Getpid(), dashboardAdminSchemaSequence.Add(1))
	if _, err := admin.Exec("CREATE DATABASE `" + schema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + schema + "`")
		leftovers := dashboardAdminSchemaLeftoversWithDB(t, admin)
		if leftovers != baseline {
			t.Errorf("schema leftovers changed from baseline=%d to %d", baseline, leftovers)
		}
		t.Logf("schema leftovers=0 (task schema dropped; baseline=%d)", baseline)
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
		`CREATE TABLE mc_tenant (id int(10) unsigned NOT NULL AUTO_INCREMENT, name varchar(255) NOT NULL DEFAULT '', status tinyint NOT NULL DEFAULT 1, created_at timestamp NULL, updated_at timestamp NULL, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_corp (id int(10) unsigned NOT NULL AUTO_INCREMENT, tenant_id int(11) DEFAULT 0, name varchar(255) NOT NULL DEFAULT '', wx_corpid varchar(255) NOT NULL DEFAULT '', employee_secret varchar(255) NOT NULL DEFAULT '', contact_secret varchar(255) NOT NULL DEFAULT '', token varchar(255) NOT NULL DEFAULT '', encoding_aes_key varchar(255) NOT NULL DEFAULT '', created_at timestamp NULL, updated_at timestamp NULL, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_user (id int(10) unsigned NOT NULL AUTO_INCREMENT, tenant_id int(11) NOT NULL DEFAULT 1, phone char(11) NOT NULL DEFAULT '', password varchar(255) NOT NULL DEFAULT '', name varchar(255) NOT NULL DEFAULT '', status tinyint unsigned NOT NULL DEFAULT 1, created_at timestamp NULL, updated_at timestamp NULL, deleted_at timestamp NULL, isSuperAdmin tinyint NOT NULL DEFAULT 0, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_role (id int(11) NOT NULL AUTO_INCREMENT, tenant_id int(11) NOT NULL, data_permission json DEFAULT NULL, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_user_role (id int(11) NOT NULL AUTO_INCREMENT, user_id int(11) NOT NULL, role_id int(11) NOT NULL, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`INSERT INTO mc_tenant (id, name, status) VALUES (1, 'Fixture tenant 1', 1), (2, 'Fixture tenant 2', 1)`,
		`INSERT INTO mc_corp (id, tenant_id, name) VALUES (100, 1, 'Fixture corp 1'), (200, 2, 'Fixture corp 2')`,
		`INSERT INTO mc_user (id, tenant_id, phone, status, deleted_at) VALUES (10, 1, '13800000001', 1, NULL)`,
		`INSERT INTO mc_rbac_role (id, tenant_id) VALUES (20, 1)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("base fixture: %v", err)
		}
	}
	root := filepath.Join("..", "..")
	pageRBAC, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0127_dashboard_page_rbac.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	pageScript := string(pageRBAC)
	start := strings.Index(pageScript, "ALTER TABLE `mc_user`")
	end := strings.Index(pageScript, "INSERT INTO `mochat_go_dashboard_permissions`")
	if start < 0 || end <= start {
		t.Fatal("0127 DDL boundaries not found")
	}
	applyDashboardAdminSQL(t, db, pageScript[start:end])
	for _, migrationName := range []string{"0003_saas_provisioning.up.sql", "0024_saas_package_extended_limits.up.sql", "0025_saas_radar_limit.up.sql", "0026_saas_lottery_limit.up.sql", "0027_saas_room_infinite_pull_limit.up.sql", "0028_saas_room_fission_limit.up.sql", "0029_saas_room_clock_in_limit.up.sql", "0030_saas_room_operation_limits.up.sql", "0031_saas_sop_limits.up.sql", "0032_saas_sensitive_word_limit.up.sql", "0033_saas_admin_operation_logs.up.sql", "0039_saas_subscription_lifecycle.up.sql", "0045_saas_admin_rbac.up.sql", "0084_saas_package_definition_guard.up.sql", "0085_saas_tenant_package_assignment_guard.up.sql"} {
		body, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", migrationName))
		if err != nil {
			t.Fatal(err)
		}
		applyDashboardAdminSQL(t, db, string(body))
	}
	body, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0129_identity_realms_single_corp_schema.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	applyDashboardAdminSQL(t, db, string(body))
}

func applyDashboardAdminSQL(t *testing.T, db *sql.DB, script string) {
	t.Helper()
	statements, err := migration.SplitSQLStatements(script)
	if err != nil {
		t.Fatalf("split migration SQL: %v", err)
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("migration fixture statement: %v", err)
		}
	}
}
