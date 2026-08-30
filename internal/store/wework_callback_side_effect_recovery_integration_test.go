package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcredentials"
)

func TestCallbackSideEffectRecoveryCursorRejectsTrailingJSON(t *testing.T) {
	raw := `{"unknownAt":"2026-08-30T00:00:00Z","eventKey":"` + strings.Repeat("a", 64) + `","actionKey":"fission.customer_push"}{}`
	if _, err := decodeCallbackSideEffectCursor(base64.RawURLEncoding.EncodeToString([]byte(raw))); err == nil {
		t.Fatal("cursor with trailing JSON was accepted")
	}
}

func TestCallbackSideEffectRecoveryCursorRejectsNonCanonicalIdentity(t *testing.T) {
	for _, raw := range []string{
		`{"unknownAt":"2026-08-30T00:00:00Z","eventKey":"` + strings.Repeat("A", 64) + `","actionKey":"fission.customer_push"}`,
		`{"unknownAt":"2026-08-30T00:00:00Z","eventKey":"` + strings.Repeat("a", 64) + `","actionKey":"future.action"}`,
	} {
		if _, err := decodeCallbackSideEffectCursor(base64.RawURLEncoding.EncodeToString([]byte(raw))); err == nil {
			t.Fatalf("non-canonical cursor was accepted: %s", raw)
		}
	}
}

func TestCallbackSideEffectRecoveryIsScopedIdempotentAndRevivesOnlyAfterLastUnknownRealMySQL(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
	createDashboardAdminProvisioningFixture(t, db)
	manager := testWeComCredentialManager(t, wecomcredentials.Config{EncryptionKey: testCompanyCredentialKey(47), EncryptionKeyID: "callback-recovery", RequireEncryption: true, DedicatedConfigured: true})
	prepareCompanyProfileRepositoryFixture(t, db, manager)
	activateCallbackRecoveryBinding(t, db)
	store := NewMySQLStore(db)
	principal := dashboardprincipal.DashboardPrincipal{UserID: 10, TenantID: 1, CorpID: 100, CorpStatus: dashboardprincipal.CorpBindingStatusActive, IsSuperAdmin: true, AuthVersion: 1}
	eventKey := strings.Repeat("e", 64)
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_inbox
		(tenant_id,corp_id,event_key,payload_fingerprint,event_path,event_json,status,attempt,lease_fence,last_error,received_at)
		VALUES (1,100,?,?,'event.test','{}','dead',3,7,'provider result unknown',UTC_TIMESTAMP(6))`, eventKey, strings.Repeat("f", 64)); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"fission.customer_push", "fission.employee_reminder"} {
		if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_side_effects
			(tenant_id,corp_id,event_key,action_key,payload_hash,status,version,unknown_at,reconcile_after)
			VALUES (1,100,?,?,?,'unknown',3,DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 20 MINUTE),DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 5 MINUTE))`, eventKey, action, strings.Repeat("a", 64)); err != nil {
			t.Fatal(err)
		}
	}
	firstPage, err := store.ListCallbackSideEffects(context.Background(), principal, companyprofile.CallbackSideEffectListInput{Status: "unknown", Limit: 1})
	if err != nil || len(firstPage.Items) != 1 || firstPage.NextCursor == "" {
		t.Fatalf("first page=%+v err=%v", firstPage, err)
	}
	secondPage, err := store.ListCallbackSideEffects(context.Background(), principal, companyprofile.CallbackSideEffectListInput{Status: "unknown", Limit: 1, Cursor: firstPage.NextCursor})
	if err != nil || len(secondPage.Items) != 1 || secondPage.Items[0].ActionKey == firstPage.Items[0].ActionKey {
		t.Fatalf("second page=%+v first=%+v err=%v", secondPage, firstPage, err)
	}
	firstInput := companyprofile.CallbackSideEffectReconcileInput{Decision: companyprofile.CallbackSideEffectDecisionConfirmSent, ExpectedVersion: 3, ExpectedInboxLeaseFence: 7, Reason: "企微管理后台回执确认已发送", EvidenceKind: "provider_message_id", EvidenceRef: "receipt-first"}
	firstRequestID := "r" + strings.Repeat("x", 127)
	first, err := store.ReconcileCallbackSideEffect(context.Background(), principal, eventKey, "fission.employee_reminder", firstRequestID, firstInput)
	if err != nil || first.Status != "sent" || first.InboxReplayScheduled || first.RemainingUnknownActions != 1 || first.InboxLeaseFence != 7 {
		t.Fatalf("first reconciliation=%+v err=%v", first, err)
	}
	replayed, err := store.ReconcileCallbackSideEffect(context.Background(), principal, eventKey, "fission.employee_reminder", firstRequestID, firstInput)
	if err != nil || replayed != (companyprofile.CallbackSideEffectReconcileResult{EventKey: eventKey, ActionKey: "fission.employee_reminder", Status: "sent", Version: 4, InboxLeaseFence: 7, RemainingUnknownActions: 1, Idempotent: true}) {
		t.Fatalf("idempotent replay=%+v err=%v", replayed, err)
	}
	conflicting := firstInput
	conflicting.EvidenceRef = "different-receipt"
	if _, err := store.ReconcileCallbackSideEffect(context.Background(), principal, eventKey, "fission.employee_reminder", firstRequestID, conflicting); !errors.Is(err, companyprofile.ErrIdempotencyConflict) {
		t.Fatalf("idempotency conflict error=%v", err)
	}
	secondInput := companyprofile.CallbackSideEffectReconcileInput{Decision: companyprofile.CallbackSideEffectDecisionConfirmNotSentAndRetry, ExpectedVersion: 3, ExpectedInboxLeaseFence: 7, Reason: "企微管理后台确认没有发送记录", EvidenceKind: "provider_delivery_query_absent", EvidenceRef: "search-second"}
	second, err := store.ReconcileCallbackSideEffect(context.Background(), principal, eventKey, "fission.customer_push", "request-recovery-second", secondInput)
	if err != nil || second.Status != "pending" || !second.InboxReplayScheduled || second.RemainingUnknownActions != 0 || second.InboxLeaseFence != 8 {
		t.Fatalf("second reconciliation=%+v err=%v", second, err)
	}
	var inboxStatus, firstStatus, secondStatus string
	var attempt, audits, commands int
	var fence uint64
	if err := db.QueryRow(`SELECT status,attempt,lease_fence FROM mochat_go_wework_callback_inbox WHERE tenant_id=1 AND corp_id=100 AND event_key=?`, eventKey).Scan(&inboxStatus, &attempt, &fence); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status FROM mochat_go_wework_callback_side_effects WHERE tenant_id=1 AND corp_id=100 AND event_key=? AND action_key='fission.employee_reminder'`, eventKey).Scan(&firstStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status FROM mochat_go_wework_callback_side_effects WHERE tenant_id=1 AND corp_id=100 AND event_key=? AND action_key='fission.customer_push'`, eventKey).Scan(&secondStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id=1 AND target_type='callback_side_effect' AND target_id=?`, eventKey).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wework_callback_side_effect_commands WHERE tenant_id=1 AND corp_id=100 AND event_key=?`, eventKey).Scan(&commands); err != nil {
		t.Fatal(err)
	}
	var storedAuditRequestID string
	var storedAuditAfterJSON string
	if err := db.QueryRow(`SELECT request_id,after_json FROM mochat_go_dashboard_permission_audits WHERE tenant_id=1 AND target_type='callback_side_effect' AND target_id=? AND expected_version=3 ORDER BY id LIMIT 1`, eventKey).Scan(&storedAuditRequestID, &storedAuditAfterJSON); err != nil {
		t.Fatal(err)
	}
	if inboxStatus != "pending" || attempt != 0 || fence != 8 || firstStatus != "sent" || secondStatus != "pending" || audits != 2 || commands != 2 {
		t.Fatalf("state inbox=%s attempt=%d fence=%d actions=%s/%s audits=%d commands=%d", inboxStatus, attempt, fence, firstStatus, secondStatus, audits, commands)
	}
	if storedAuditRequestID != firstRequestID {
		t.Fatalf("audit request id length=%d want=%d", len(storedAuditRequestID), len(firstRequestID))
	}
	var storedAuditAfter map[string]any
	if err := json.Unmarshal([]byte(storedAuditAfterJSON), &storedAuditAfter); err != nil || storedAuditAfter["actionKey"] != "fission.employee_reminder" {
		t.Fatalf("audit does not identify the reconciled action: %s", storedAuditAfterJSON)
	}
	detail, err := store.GetCallbackSideEffect(context.Background(), principal, eventKey, "fission.customer_push")
	if err != nil || len(detail.Actions) != 2 || detail.LastErrorCode != "" {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}
}

func TestCallbackSideEffectRecoveryFailsClosedForLeaseQuarantineUnknownActionAndCrossScopeRealMySQL(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
	createDashboardAdminProvisioningFixture(t, db)
	manager := testWeComCredentialManager(t, wecomcredentials.Config{EncryptionKey: testCompanyCredentialKey(49), EncryptionKeyID: "callback-recovery-guards", RequireEncryption: true, DedicatedConfigured: true})
	prepareCompanyProfileRepositoryFixture(t, db, manager)
	activateCallbackRecoveryBinding(t, db)
	store := NewMySQLStore(db)
	principal := dashboardprincipal.DashboardPrincipal{UserID: 10, TenantID: 1, CorpID: 100, CorpStatus: dashboardprincipal.CorpBindingStatusActive, IsSuperAdmin: true, AuthVersion: 1}
	input := companyprofile.CallbackSideEffectReconcileInput{Decision: companyprofile.CallbackSideEffectDecisionConfirmSent, ExpectedVersion: 2, ExpectedInboxLeaseFence: 4, Reason: "受控状态机保护测试", EvidenceKind: "provider_message_id", EvidenceRef: "guard-evidence"}

	activeKey := strings.Repeat("1", 64)
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_inbox (tenant_id,corp_id,event_key,payload_fingerprint,event_path,event_json,status,attempt,lease_token,lease_fence,lease_expires_at,received_at) VALUES (1,100,?,?,'event.test','{}','processing',1,?,4,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 MINUTE),UTC_TIMESTAMP(6))`, activeKey, strings.Repeat("2", 64), strings.Repeat("3", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_side_effects (tenant_id,corp_id,event_key,action_key,payload_hash,status,version,unknown_at,reconcile_after) VALUES (1,100,?,'fission.customer_push',?,'unknown',2,DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 20 MINUTE),DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 5 MINUTE))`, activeKey, strings.Repeat("4", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReconcileCallbackSideEffect(context.Background(), principal, activeKey, "fission.customer_push", "request-active-lease", input); !errors.Is(err, companyprofile.ErrCallbackLeaseActive) {
		t.Fatalf("active lease error=%v", err)
	}
	for _, tc := range []struct {
		name       string
		key        string
		status     string
		leaseToken string
		expiresAt  *time.Time
		want       error
	}{
		{name: "future processing without token", key: strings.Repeat("0", 64), status: "processing", expiresAt: timePointer(time.Now().Add(5 * time.Minute)), want: companyprofile.ErrCallbackLeaseActive},
		{name: "expired processing without token", key: strings.Repeat("2", 64), status: "processing", expiresAt: timePointer(time.Now().Add(-5 * time.Minute)), want: companyprofile.ErrInboxStateConflict},
		{name: "completed", key: strings.Repeat("3", 64), status: "completed", want: companyprofile.ErrInboxStateConflict},
	} {
		seedCallbackRecoveryGuard(t, db, tc.key, tc.status, tc.leaseToken, tc.expiresAt)
		if _, err := store.ReconcileCallbackSideEffect(context.Background(), principal, tc.key, "fission.customer_push", "request-guard-"+tc.name, input); !errors.Is(err, tc.want) {
			t.Fatalf("%s error=%v, want %v", tc.name, err, tc.want)
		}
	}
	expiredKey := strings.Repeat("4", 64)
	seedCallbackRecoveryGuard(t, db, expiredKey, "processing", strings.Repeat("f", 64), timePointer(time.Now().Add(-5*time.Minute)))
	if result, err := store.ReconcileCallbackSideEffect(context.Background(), principal, expiredKey, "fission.customer_push", "request-expired-processing", input); err != nil || !result.InboxReplayScheduled {
		t.Fatalf("structurally complete expired processing result=%+v err=%v", result, err)
	}

	quarantineKey := strings.Repeat("5", 64)
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_inbox (tenant_id,corp_id,event_key,payload_fingerprint,event_path,event_json,status,attempt,lease_fence,received_at) VALUES (1,100,?,?,'event.test','{}','dead',3,4,UTC_TIMESTAMP(6))`, quarantineKey, strings.Repeat("6", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_side_effects (tenant_id,corp_id,event_key,action_key,payload_hash,status,version,unknown_at,reconcile_after) VALUES (1,100,?,'fission.customer_push',?,'unknown',2,UTC_TIMESTAMP(6),DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 MINUTE))`, quarantineKey, strings.Repeat("7", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReconcileCallbackSideEffect(context.Background(), principal, quarantineKey, "fission.customer_push", "request-quarantine", input); !errors.Is(err, companyprofile.ErrQuarantineActive) {
		t.Fatalf("quarantine error=%v", err)
	}

	unknownKey := strings.Repeat("8", 64)
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_inbox (tenant_id,corp_id,event_key,payload_fingerprint,event_path,event_json,status,attempt,lease_fence,received_at) VALUES (1,100,?,?,'event.test','{}','dead',3,4,UTC_TIMESTAMP(6))`, unknownKey, strings.Repeat("9", 64)); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"fission.customer_push", "future.action"} {
		if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_side_effects (tenant_id,corp_id,event_key,action_key,payload_hash,status,version,unknown_at,reconcile_after) VALUES (1,100,?,?,?,'unknown',2,DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 20 MINUTE),DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 5 MINUTE))`, unknownKey, action, strings.Repeat("a", 64)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.ReconcileCallbackSideEffect(context.Background(), principal, unknownKey, "fission.customer_push", "request-unknown-action", input); !errors.Is(err, companyprofile.ErrUnsupportedAction) {
		t.Fatalf("unknown action error=%v", err)
	}

	if _, err := db.Exec(`INSERT INTO mc_tenant (id,name,status) VALUES (999,'Other tenant',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mc_corp (id,tenant_id,name) VALUES (9900,999,'Other corp')`); err != nil {
		t.Fatal(err)
	}
	otherKey := strings.Repeat("b", 64)
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_inbox (tenant_id,corp_id,event_key,payload_fingerprint,event_path,event_json,status,attempt,lease_fence,received_at) VALUES (999,9900,?,?,'event.test','{}','dead',3,4,UTC_TIMESTAMP(6))`, otherKey, strings.Repeat("c", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_side_effects (tenant_id,corp_id,event_key,action_key,payload_hash,status,version,unknown_at,reconcile_after) VALUES (999,9900,?,'fission.customer_push',?,'unknown',2,DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 20 MINUTE),DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 5 MINUTE))`, otherKey, strings.Repeat("d", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetCallbackSideEffect(context.Background(), principal, otherKey, "fission.customer_push"); !errors.Is(err, companyprofile.ErrRecoveryTargetNotFound) {
		t.Fatalf("cross-scope detail error=%v", err)
	}
	bindingKey := strings.Repeat("c", 64)
	seedCallbackRecoveryGuard(t, db, bindingKey, "dead", "", nil)
	if _, err := db.Exec(`UPDATE mochat_go_tenant_corp_bindings SET status=3 WHERE tenant_id=1 AND corp_id=100`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReconcileCallbackSideEffect(context.Background(), principal, bindingKey, "fission.customer_push", "request-suspended-binding", input); !errors.Is(err, companyprofile.ErrTenantAccessDenied) {
		t.Fatalf("suspended binding error=%v", err)
	}
	if _, err := db.Exec(`UPDATE mochat_go_tenant_corp_bindings SET status=2 WHERE tenant_id=1 AND corp_id=100`); err != nil {
		t.Fatal(err)
	}
	actorKey := strings.Repeat("d", 64)
	seedCallbackRecoveryGuard(t, db, actorKey, "dead", "", nil)
	if _, err := db.Exec(`UPDATE mochat_go_dashboard_identities SET auth_version=auth_version+1 WHERE user_id=10`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReconcileCallbackSideEffect(context.Background(), principal, actorKey, "fission.customer_push", "request-stale-principal", input); !errors.Is(err, companyprofile.ErrPermissionDenied) {
		t.Fatalf("stale principal error=%v", err)
	}
}

func TestCallbackSideEffectRecoveryConcurrentSameCommandMutatesOnceRealMySQL(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
	createDashboardAdminProvisioningFixture(t, db)
	manager := testWeComCredentialManager(t, wecomcredentials.Config{EncryptionKey: testCompanyCredentialKey(48), EncryptionKeyID: "callback-recovery-concurrent", RequireEncryption: true, DedicatedConfigured: true})
	prepareCompanyProfileRepositoryFixture(t, db, manager)
	activateCallbackRecoveryBinding(t, db)
	store := NewMySQLStore(db)
	principal := dashboardprincipal.DashboardPrincipal{UserID: 10, TenantID: 1, CorpID: 100, CorpStatus: dashboardprincipal.CorpBindingStatusActive, IsSuperAdmin: true, AuthVersion: 1}
	eventKey := strings.Repeat("d", 64)
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_inbox (tenant_id,corp_id,event_key,payload_fingerprint,event_path,event_json,status,attempt,lease_fence,received_at) VALUES (1,100,?,?,'event.test','{}','dead',3,12,UTC_TIMESTAMP(6))`, eventKey, strings.Repeat("c", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_side_effects (tenant_id,corp_id,event_key,action_key,payload_hash,status,version,unknown_at,reconcile_after) VALUES (1,100,?,'fission.customer_push',?,'unknown',5,DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 20 MINUTE),DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 5 MINUTE))`, eventKey, strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	input := companyprofile.CallbackSideEffectReconcileInput{Decision: companyprofile.CallbackSideEffectDecisionConfirmSent, ExpectedVersion: 5, ExpectedInboxLeaseFence: 12, Reason: "并发确认回执已发送", EvidenceKind: "provider_message_id", EvidenceRef: "concurrent-one"}
	var wg sync.WaitGroup
	type outcome struct {
		result companyprofile.CallbackSideEffectReconcileResult
		err    error
	}
	outcomes := make(chan outcome, 32)
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			result, err := store.ReconcileCallbackSideEffect(ctx, principal, eventKey, "fission.customer_push", "request-recovery-concurrent", input)
			outcomes <- outcome{result: result, err: err}
		}()
	}
	wg.Wait()
	close(outcomes)
	idempotent := 0
	for item := range outcomes {
		if item.err != nil {
			t.Fatal(item.err)
		}
		if item.result.Idempotent {
			idempotent++
		}
	}
	if idempotent != 31 {
		t.Fatalf("idempotent responses=%d, want 31", idempotent)
	}
	var version, audits, commands int
	if err := db.QueryRow(`SELECT version FROM mochat_go_wework_callback_side_effects WHERE tenant_id=1 AND corp_id=100 AND event_key=? AND action_key='fission.customer_push'`, eventKey).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id=1 AND target_type='callback_side_effect' AND target_id=?`, eventKey).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wework_callback_side_effect_commands WHERE tenant_id=1 AND corp_id=100 AND request_id='request-recovery-concurrent'`).Scan(&commands); err != nil {
		t.Fatal(err)
	}
	if version != 6 || audits != 1 || commands != 1 {
		t.Fatalf("version=%d audits=%d commands=%d", version, audits, commands)
	}
	firstCrossEvent := strings.Repeat("6", 64)
	secondCrossEvent := strings.Repeat("7", 64)
	seedCallbackRecoveryGuard(t, db, firstCrossEvent, "dead", "", nil)
	seedCallbackRecoveryGuard(t, db, secondCrossEvent, "dead", "", nil)
	crossOutcomes := make(chan error, 2)
	for _, key := range []string{firstCrossEvent, secondCrossEvent} {
		wg.Add(1)
		go func(eventKey string) {
			defer wg.Done()
			_, err := store.ReconcileCallbackSideEffect(context.Background(), principal, eventKey, "fission.customer_push", "request-cross-event-concurrent", companyprofile.CallbackSideEffectReconcileInput{
				Decision: companyprofile.CallbackSideEffectDecisionConfirmSent, ExpectedVersion: 2, ExpectedInboxLeaseFence: 4,
				Reason: "跨事件同请求键并发串行化", EvidenceKind: "provider_message_id", EvidenceRef: "cross-event",
			})
			crossOutcomes <- err
		}(key)
	}
	wg.Wait()
	close(crossOutcomes)
	successes, conflicts := 0, 0
	for err := range crossOutcomes {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, companyprofile.ErrIdempotencyConflict):
			conflicts++
		default:
			t.Fatalf("cross-event duplicate race returned %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("cross-event outcomes success=%d conflict=%d", successes, conflicts)
	}
	var sentActions, unknownActions, crossVersionSum, crossAudits, crossCommands int
	if err := db.QueryRow(`SELECT COALESCE(SUM(status='sent'),0),COALESCE(SUM(status='unknown'),0),COALESCE(SUM(version),0)
		FROM mochat_go_wework_callback_side_effects WHERE tenant_id=1 AND corp_id=100 AND event_key IN (?,?)`, firstCrossEvent, secondCrossEvent).
		Scan(&sentActions, &unknownActions, &crossVersionSum); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id=1 AND request_id='request-cross-event-concurrent'`).Scan(&crossAudits); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wework_callback_side_effect_commands WHERE tenant_id=1 AND corp_id=100 AND request_id='request-cross-event-concurrent'`).Scan(&crossCommands); err != nil {
		t.Fatal(err)
	}
	if sentActions != 1 || unknownActions != 1 || crossVersionSum != 5 || crossAudits != 1 || crossCommands != 1 {
		t.Fatalf("cross-event durable state sent=%d unknown=%d versionSum=%d audits=%d commands=%d", sentActions, unknownActions, crossVersionSum, crossAudits, crossCommands)
	}
}

func TestCallbackSideEffectRecoveryReservationInsertFailureLeavesStateUntouchedRealMySQL(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
	createDashboardAdminProvisioningFixture(t, db)
	manager := testWeComCredentialManager(t, wecomcredentials.Config{EncryptionKey: testCompanyCredentialKey(50), EncryptionKeyID: "callback-recovery-rollback", RequireEncryption: true, DedicatedConfigured: true})
	prepareCompanyProfileRepositoryFixture(t, db, manager)
	activateCallbackRecoveryBinding(t, db)
	store := NewMySQLStore(db)
	principal := dashboardprincipal.DashboardPrincipal{UserID: 10, TenantID: 1, CorpID: 100, CorpStatus: dashboardprincipal.CorpBindingStatusActive, IsSuperAdmin: true, AuthVersion: 1}
	eventKey := strings.Repeat("f", 64)
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_inbox (tenant_id,corp_id,event_key,payload_fingerprint,event_path,event_json,status,attempt,lease_fence,last_error,received_at) VALUES (1,100,?,?,'event.test','{}','dead',3,21,'provider unknown',UTC_TIMESTAMP(6))`, eventKey, strings.Repeat("e", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_side_effects (tenant_id,corp_id,event_key,action_key,payload_hash,status,version,unknown_at,reconcile_after) VALUES (1,100,?,'fission.customer_push',?,'unknown',8,DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 20 MINUTE),DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 5 MINUTE))`, eventKey, strings.Repeat("d", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER callback_recovery_receipt_failure BEFORE INSERT ON mochat_go_wework_callback_side_effect_commands FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected receipt failure'`); err != nil {
		t.Fatal(err)
	}
	input := companyprofile.CallbackSideEffectReconcileInput{Decision: companyprofile.CallbackSideEffectDecisionConfirmSent, ExpectedVersion: 8, ExpectedInboxLeaseFence: 21, Reason: "故障注入必须完整回滚", EvidenceKind: "provider_message_id", EvidenceRef: "rollback-evidence"}
	if _, err := store.ReconcileCallbackSideEffect(context.Background(), principal, eventKey, "fission.customer_push", "request-recovery-rollback", input); err == nil || !strings.Contains(err.Error(), "injected receipt failure") {
		t.Fatalf("receipt failure error=%v", err)
	}
	var actionStatus, inboxStatus string
	var version, fence, audits, commands int
	if err := db.QueryRow(`SELECT status,version FROM mochat_go_wework_callback_side_effects WHERE tenant_id=1 AND corp_id=100 AND event_key=? AND action_key='fission.customer_push'`, eventKey).Scan(&actionStatus, &version); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status,lease_fence FROM mochat_go_wework_callback_inbox WHERE tenant_id=1 AND corp_id=100 AND event_key=?`, eventKey).Scan(&inboxStatus, &fence); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id=1 AND target_type='callback_side_effect' AND target_id=?`, eventKey).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wework_callback_side_effect_commands WHERE tenant_id=1 AND corp_id=100 AND event_key=?`, eventKey).Scan(&commands); err != nil {
		t.Fatal(err)
	}
	if actionStatus != "unknown" || version != 8 || inboxStatus != "dead" || fence != 21 || audits != 0 || commands != 0 {
		t.Fatalf("rollback action=%s/%d inbox=%s/%d audits=%d commands=%d", actionStatus, version, inboxStatus, fence, audits, commands)
	}
}

func TestCallbackSideEffectRecoveryFaultInjectionRollsBackEveryTransactionalStageRealMySQL(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
	createDashboardAdminProvisioningFixture(t, db)
	manager := testWeComCredentialManager(t, wecomcredentials.Config{EncryptionKey: testCompanyCredentialKey(51), EncryptionKeyID: "callback-recovery-stage-failures", RequireEncryption: true, DedicatedConfigured: true})
	prepareCompanyProfileRepositoryFixture(t, db, manager)
	activateCallbackRecoveryBinding(t, db)
	store := NewMySQLStore(db)
	principal := dashboardprincipal.DashboardPrincipal{UserID: 10, TenantID: 1, CorpID: 100, CorpStatus: dashboardprincipal.CorpBindingStatusActive, IsSuperAdmin: true, AuthVersion: 1}
	stages := []struct {
		name       string
		keyRune    string
		triggerSQL string
		commit     bool
	}{
		{name: "action", keyRune: "a", triggerSQL: `CREATE TRIGGER callback_recovery_action_failure BEFORE UPDATE ON mochat_go_wework_callback_side_effects FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected action failure'`},
		{name: "inbox", keyRune: "b", triggerSQL: `CREATE TRIGGER callback_recovery_inbox_failure BEFORE UPDATE ON mochat_go_wework_callback_inbox FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected inbox failure'`},
		{name: "audit", keyRune: "c", triggerSQL: `CREATE TRIGGER callback_recovery_audit_failure BEFORE INSERT ON mochat_go_dashboard_permission_audits FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected audit failure'`},
		{name: "receipt", keyRune: "d", triggerSQL: `CREATE TRIGGER callback_recovery_receipt_update_failure BEFORE UPDATE ON mochat_go_wework_callback_side_effect_commands FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected receipt update failure'`},
		{name: "commit", keyRune: "e", commit: true},
	}
	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			eventKey := strings.Repeat(stage.keyRune, 64)
			seedCallbackRecoveryRollback(t, db, eventKey)
			triggerName := ""
			if stage.triggerSQL != "" {
				if _, err := db.Exec(stage.triggerSQL); err != nil {
					t.Fatal(err)
				}
				triggerName = "callback_recovery_" + stage.name + "_failure"
				if stage.name == "receipt" {
					triggerName = "callback_recovery_receipt_update_failure"
				}
				defer db.Exec(`DROP TRIGGER ` + triggerName)
			}
			if stage.commit {
				store.callbackRecoveryCommit = func(*sql.Tx) error { return errors.New("injected commit failure") }
				defer func() { store.callbackRecoveryCommit = nil }()
			}
			input := companyprofile.CallbackSideEffectReconcileInput{Decision: companyprofile.CallbackSideEffectDecisionConfirmSent, ExpectedVersion: 8, ExpectedInboxLeaseFence: 21, Reason: "分阶段故障必须完整回滚", EvidenceKind: "provider_message_id", EvidenceRef: "failure-" + stage.name}
			if _, err := store.ReconcileCallbackSideEffect(context.Background(), principal, eventKey, "fission.customer_push", "request-stage-failure-"+stage.name, input); err == nil || !strings.Contains(err.Error(), "injected "+stage.name) {
				t.Fatalf("%s failure error=%v", stage.name, err)
			}
			assertCallbackRecoveryRolledBack(t, db, eventKey, 8, 21)
		})
	}
}

func activateCallbackRecoveryBinding(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`UPDATE mochat_go_tenant_corp_bindings SET status=2 WHERE tenant_id=1 AND corp_id=100`); err != nil {
		t.Fatal(err)
	}
}

func seedCallbackRecoveryGuard(t *testing.T, db *sql.DB, eventKey, status, leaseToken string, leaseExpiresAt *time.Time) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_inbox
		(tenant_id,corp_id,event_key,payload_fingerprint,event_path,event_json,status,attempt,lease_token,lease_fence,lease_expires_at,received_at)
		VALUES (1,100,?,?,'event.test','{}',?,3,?,4,?,UTC_TIMESTAMP(6))`, eventKey, strings.Repeat("c", 64), status, leaseToken, leaseExpiresAt); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_side_effects
		(tenant_id,corp_id,event_key,action_key,payload_hash,status,version,unknown_at,reconcile_after)
		VALUES (1,100,?,'fission.customer_push',?,'unknown',2,DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 20 MINUTE),DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 5 MINUTE))`, eventKey, strings.Repeat("d", 64)); err != nil {
		t.Fatal(err)
	}
}

func seedCallbackRecoveryRollback(t *testing.T, db *sql.DB, eventKey string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_inbox
		(tenant_id,corp_id,event_key,payload_fingerprint,event_path,event_json,status,attempt,lease_fence,last_error,received_at)
		VALUES (1,100,?,?,'event.test','{}','dead',3,21,'provider unknown',UTC_TIMESTAMP(6))`, eventKey, strings.Repeat("e", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_wework_callback_side_effects
		(tenant_id,corp_id,event_key,action_key,payload_hash,status,version,unknown_at,reconcile_after)
		VALUES (1,100,?,'fission.customer_push',?,'unknown',8,DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 20 MINUTE),DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 5 MINUTE))`, eventKey, strings.Repeat("d", 64)); err != nil {
		t.Fatal(err)
	}
}

func assertCallbackRecoveryRolledBack(t *testing.T, db *sql.DB, eventKey string, wantVersion, wantFence int) {
	t.Helper()
	var actionStatus, inboxStatus string
	var version, fence, audits, commands int
	if err := db.QueryRow(`SELECT status,version FROM mochat_go_wework_callback_side_effects WHERE tenant_id=1 AND corp_id=100 AND event_key=? AND action_key='fission.customer_push'`, eventKey).Scan(&actionStatus, &version); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status,lease_fence FROM mochat_go_wework_callback_inbox WHERE tenant_id=1 AND corp_id=100 AND event_key=?`, eventKey).Scan(&inboxStatus, &fence); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id=1 AND target_type='callback_side_effect' AND target_id=?`, eventKey).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wework_callback_side_effect_commands WHERE tenant_id=1 AND corp_id=100 AND event_key=?`, eventKey).Scan(&commands); err != nil {
		t.Fatal(err)
	}
	if actionStatus != "unknown" || version != wantVersion || inboxStatus != "dead" || fence != wantFence || audits != 0 || commands != 0 {
		t.Fatalf("rollback action=%s/%d inbox=%s/%d audits=%d commands=%d", actionStatus, version, inboxStatus, fence, audits, commands)
	}
}

func timePointer(value time.Time) *time.Time { return &value }
