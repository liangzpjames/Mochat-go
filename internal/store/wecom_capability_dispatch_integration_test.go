package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/wecomcapability"
)

type capabilityDispatchIntegrationHarness struct {
	db        *sql.DB
	store     *MySQLStore
	principal dashboardprincipal.DashboardPrincipal
}

func newCapabilityDispatchIntegrationHarness(t *testing.T) *capabilityDispatchIntegrationHarness {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB DSN is required")
	}
	cfg, err := mysql.ParseDSN(dsn)
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
	schema := fmt.Sprintf("mochat_wecom_dispatch_%d_%d", os.Getpid(), capabilityLedgerStoreSchemaSequence.Add(1))
	if _, err := admin.Exec("CREATE DATABASE `" + schema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	cfg.DBName = schema
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + schema + "`")
		_ = admin.Close()
		t.Fatal(err)
	}
	cleanup := func() {
		_ = db.Close()
		if _, err := admin.Exec("DROP DATABASE IF EXISTS `" + schema + "`"); err != nil {
			t.Errorf("drop temporary schema: %v", err)
		}
		_ = admin.Close()
	}
	t.Cleanup(cleanup)
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	createCapabilityLedgerStoreFixture(t, db)
	root := filepath.Join("..", "..")
	runner, err := migration.NewRunner(db, []migration.Migration{{
		Version: "0139_wecom_capability_ledger", Description: "wecom capability ledger",
		Path:     filepath.Join(root, "deploy", "standalone", "migrations", "0139_wecom_capability_ledger.up.sql"),
		DownPath: filepath.Join(root, "deploy", "standalone", "migrations", "0139_wecom_capability_ledger.down.sql"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	return &capabilityDispatchIntegrationHarness{
		db: db, store: NewMySQLStore(db),
		principal: dashboardprincipal.DashboardPrincipal{TenantID: 11, CorpID: 1101, AuthVersion: 1},
	}
}

func createContactDispatchForIntegration(t *testing.T, h *capabilityDispatchIntegrationHarness, suffix string) (wecomcapability.Operation, wecomcapability.Dispatch) {
	t.Helper()
	operation, err := h.store.CreateCapabilityOperation(context.Background(), h.principal, CapabilityOperationInput{
		Capability: wecomcapability.ContactBatchSend, Action: wecomcapability.ActionSend,
		IdempotencyKey: "operation-" + suffix, TargetTotal: 1, ActorSource: "system",
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := h.store.CreateCapabilityDispatch(context.Background(), h.principal, CapabilityDispatchInput{
		OperationID: operation.ID, DispatchKind: string(wecomcapability.DispatchKindContactBatch), ChunkNo: 0,
		TargetID: "external-" + suffix, IdempotencyKey: "dispatch-" + suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	if dispatch.DispatchKind != string(wecomcapability.DispatchKindContactBatch) {
		t.Fatalf("contact dispatch used non-contact kind: %q", dispatch.DispatchKind)
	}
	return operation, dispatch
}

func createRoomDispatchForIntegration(t *testing.T, h *capabilityDispatchIntegrationHarness, suffix string) (wecomcapability.Operation, wecomcapability.Dispatch) {
	t.Helper()
	operation, err := h.store.CreateCapabilityOperation(context.Background(), h.principal, CapabilityOperationInput{
		Capability: wecomcapability.RoomBatchSend, Action: wecomcapability.ActionSend,
		IdempotencyKey: "operation-room-" + suffix, TargetTotal: 1, ActorSource: "system",
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := h.store.CreateCapabilityDispatch(context.Background(), h.principal, CapabilityDispatchInput{
		OperationID: operation.ID, DispatchKind: string(wecomcapability.DispatchKindRoomBatch), ChunkNo: 0,
		TargetID: "room-" + suffix, IdempotencyKey: "dispatch-room-" + suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	if dispatch.DispatchKind != string(wecomcapability.DispatchKindRoomBatch) {
		t.Fatalf("room dispatch used non-room kind: %q", dispatch.DispatchKind)
	}
	return operation, dispatch
}

func claimAndTransitionDispatch(t *testing.T, h *capabilityDispatchIntegrationHarness, dispatch wecomcapability.Dispatch, statuses ...string) wecomcapability.Dispatch {
	t.Helper()
	claimed, err := h.store.ClaimCapabilityDispatch(context.Background(), h.principal, dispatch.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range statuses {
		providerRequestID := claimed.ProviderRequestID
		if status == wecomcapability.DispatchSubmitted && strings.TrimSpace(providerRequestID) == "" && strings.TrimSpace(claimed.ProviderMessageID) == "" && strings.TrimSpace(claimed.ProviderObjectID) == "" {
			providerRequestID = fmt.Sprintf("provider-task-%d", claimed.ID)
		}
		claimed, err = h.store.TransitionCapabilityDispatch(context.Background(), h.principal, CapabilityDispatchTransitionInput{
			DispatchID: claimed.ID, Status: status, LeaseToken: claimed.LeaseToken, Attempt: claimed.Attempt,
			ProviderRequestID: providerRequestID,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return claimed
}

func TestMySQLStoreDispatchRetryEligibilityRealMariaDB(t *testing.T) {
	h := newCapabilityDispatchIntegrationHarness(t)
	_, nonRetryDispatch := createContactDispatchForIntegration(t, h, "nonretry")
	claimed := claimAndTransitionDispatch(t, h, nonRetryDispatch)
	if _, err := h.store.TransitionCapabilityDispatch(context.Background(), h.principal, CapabilityDispatchTransitionInput{
		DispatchID: claimed.ID, Status: wecomcapability.DispatchFailed, LeaseToken: claimed.LeaseToken, Attempt: claimed.Attempt,
		LastErrorCode: "wecom.http_401",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.ClaimCapabilityDispatch(context.Background(), h.principal, nonRetryDispatch.ID, time.Minute); err == nil {
		t.Fatal("non-retryable failed dispatch was claimable")
	}

	_, backoffDispatch := createContactDispatchForIntegration(t, h, "backoff")
	claimed = claimAndTransitionDispatch(t, h, backoffDispatch)
	if _, err := h.store.TransitionCapabilityDispatch(context.Background(), h.principal, CapabilityDispatchTransitionInput{
		DispatchID: claimed.ID, Status: wecomcapability.DispatchFailed, LeaseToken: claimed.LeaseToken, Attempt: claimed.Attempt,
		LastErrorCode: "wecom.http_503",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Exec(`UPDATE mochat_go_wecom_capability_dispatches SET next_poll_at=DATE_ADD(NOW(6), INTERVAL 1 HOUR) WHERE tenant_id=? AND corp_id=? AND id=?`, h.principal.TenantID, h.principal.CorpID, backoffDispatch.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.ClaimCapabilityDispatch(context.Background(), h.principal, backoffDispatch.ID, time.Minute); err == nil {
		t.Fatal("retryable failed dispatch was claimed before next_poll_at")
	}
	if _, err := h.db.Exec(`UPDATE mochat_go_wecom_capability_dispatches SET next_poll_at=DATE_SUB(NOW(6), INTERVAL 1 SECOND) WHERE tenant_id=? AND corp_id=? AND id=?`, h.principal.TenantID, h.principal.CorpID, backoffDispatch.ID); err != nil {
		t.Fatal(err)
	}
	reclaimed, err := h.store.ClaimCapabilityDispatch(context.Background(), h.principal, backoffDispatch.ID, time.Minute)
	if err != nil || reclaimed.Status != wecomcapability.DispatchClaimed || reclaimed.Attempt != 2 {
		t.Fatalf("due retryable dispatch was not claimable: %+v err=%v", reclaimed, err)
	}
}

func TestMySQLStoreDispatchReconcilePersistsEmptyProviderIDsRealMariaDB(t *testing.T) {
	h := newCapabilityDispatchIntegrationHarness(t)
	_, dispatch := createContactDispatchForIntegration(t, h, "empty-reconcile")
	claimed := claimAndTransitionDispatch(t, h, dispatch, wecomcapability.DispatchSubmitting)
	reconciled, err := h.store.PersistCapabilityDispatchReconcile(context.Background(), h.principal, CapabilityDispatchReconcileInput{
		DispatchID: claimed.ID, LeaseToken: claimed.LeaseToken, Attempt: claimed.Attempt, ErrorCode: "wecom.timeout",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.Status != wecomcapability.DispatchSubmitting || reconciled.LastErrorCode != wecomcapability.DispatchReconcileRequiredCode || reconciled.ProviderRequestID != "" || reconciled.ProviderMessageID != "" || reconciled.ProviderObjectID != "" || reconciled.NextPollAt == nil {
		t.Fatalf("empty-provider reconcile was not persisted safely: %+v", reconciled)
	}
	var auditErrorCode string
	if err := h.db.QueryRow(`SELECT error_code FROM mochat_go_wecom_capability_operation_audits WHERE operation_id=? AND dispatch_id=? AND action='dispatch_reconcile' ORDER BY id DESC LIMIT 1`, claimed.OperationID, claimed.ID).Scan(&auditErrorCode); err != nil {
		t.Fatal(err)
	}
	if auditErrorCode != "wecom.timeout" {
		t.Fatalf("reconcile audit lost original cause: %q", auditErrorCode)
	}
	var fromStatus, toStatus string
	if err := h.db.QueryRow(`SELECT from_status,to_status FROM mochat_go_wecom_capability_operation_audits WHERE operation_id=? AND dispatch_id=? AND action='dispatch_reconcile' ORDER BY id DESC LIMIT 1`, claimed.OperationID, claimed.ID).Scan(&fromStatus, &toStatus); err != nil {
		t.Fatal(err)
	}
	if fromStatus != wecomcapability.DispatchSubmitting || toStatus != wecomcapability.DispatchSubmitting {
		t.Fatalf("empty reconcile audit used non-dispatch status transition: %s -> %s", fromStatus, toStatus)
	}

	_, submittedDispatch := createContactDispatchForIntegration(t, h, "nonempty-reconcile")
	submittedClaim := claimAndTransitionDispatch(t, h, submittedDispatch, wecomcapability.DispatchSubmitting)
	submitted, err := h.store.PersistCapabilityDispatchReconcile(context.Background(), h.principal, CapabilityDispatchReconcileInput{
		DispatchID: submittedClaim.ID, LeaseToken: submittedClaim.LeaseToken, Attempt: submittedClaim.Attempt,
		ProviderRequestID: "provider-task-1", ErrorCode: "wecom.http_503",
	})
	if err != nil {
		t.Fatal(err)
	}
	if submitted.Status != wecomcapability.DispatchSubmitted || submitted.ProviderRequestID != "provider-task-1" || submitted.LastErrorCode != wecomcapability.DispatchReconcileRequiredCode {
		t.Fatalf("provider identity did not promote reconcile to submitted: %+v", submitted)
	}
	if err := h.db.QueryRow(`SELECT from_status,to_status FROM mochat_go_wecom_capability_operation_audits WHERE operation_id=? AND dispatch_id=? AND action='dispatch_reconcile' ORDER BY id DESC LIMIT 1`, submittedClaim.OperationID, submittedClaim.ID).Scan(&fromStatus, &toStatus); err != nil {
		t.Fatal(err)
	}
	if fromStatus != wecomcapability.DispatchSubmitting || toStatus != wecomcapability.DispatchSubmitted {
		t.Fatalf("submitted reconcile audit used non-dispatch status transition: %s -> %s", fromStatus, toStatus)
	}
}

func TestMySQLStoreDispatchSubmittedRequiresProviderIdentityRealMariaDB(t *testing.T) {
	h := newCapabilityDispatchIntegrationHarness(t)
	_, dispatch := createContactDispatchForIntegration(t, h, "submitted-no-id")
	claimed := claimAndTransitionDispatch(t, h, dispatch, wecomcapability.DispatchSubmitting)
	_, err := h.store.TransitionCapabilityDispatch(context.Background(), h.principal, CapabilityDispatchTransitionInput{
		DispatchID: claimed.ID, Status: wecomcapability.DispatchSubmitted, LeaseToken: claimed.LeaseToken, Attempt: claimed.Attempt,
	})
	if !errors.Is(err, companyprofile.ErrInvalidRequest) {
		t.Fatalf("submitted transition without provider identity was not rejected: %v", err)
	}
	var status, requestID, messageID, objectID string
	if err := h.db.QueryRow(`SELECT status,provider_request_id,provider_message_id,provider_object_id FROM mochat_go_wecom_capability_dispatches WHERE tenant_id=? AND corp_id=? AND id=?`, h.principal.TenantID, h.principal.CorpID, claimed.ID).Scan(&status, &requestID, &messageID, &objectID); err != nil {
		t.Fatal(err)
	}
	if status != wecomcapability.DispatchSubmitting || requestID != "" || messageID != "" || objectID != "" {
		t.Fatalf("invalid submitted transition changed dispatch: status=%q request=%q message=%q object=%q", status, requestID, messageID, objectID)
	}
}

func TestMySQLStoreDispatchRoomUsesIndependentExactKindRealMariaDB(t *testing.T) {
	h := newCapabilityDispatchIntegrationHarness(t)
	operation, dispatch := createRoomDispatchForIntegration(t, h, "exact-kind")
	if _, err := h.store.CreateCapabilityDispatch(context.Background(), h.principal, CapabilityDispatchInput{
		OperationID: operation.ID, DispatchKind: string(wecomcapability.DispatchKindContactBatch), ChunkNo: 1,
		TargetID: "room-wrong-kind", IdempotencyKey: "dispatch-room-wrong-kind",
	}); !errors.Is(err, companyprofile.ErrInvalidRequest) {
		t.Fatalf("room operation accepted contact dispatch kind: %v", err)
	}
	claimed := claimAndTransitionDispatch(t, h, dispatch, wecomcapability.DispatchSubmitting, wecomcapability.DispatchSubmitted)
	if claimed.DispatchKind != string(wecomcapability.DispatchKindRoomBatch) || claimed.Status != wecomcapability.DispatchSubmitted || claimed.ProviderRequestID == "" {
		t.Fatalf("room dispatch lifecycle was not independently durable: %+v", claimed)
	}
}

func TestMySQLStoreDispatchResultDuplicateAndControlledRecoveryRealMariaDB(t *testing.T) {
	h := newCapabilityDispatchIntegrationHarness(t)
	_, dispatch := createContactDispatchForIntegration(t, h, "result-idempotency")
	claimed := claimAndTransitionDispatch(t, h, dispatch, wecomcapability.DispatchSubmitting, wecomcapability.DispatchSubmitted)
	input := CapabilityDispatchResultInput{
		DispatchID: claimed.ID, LeaseToken: claimed.LeaseToken, Attempt: claimed.Attempt,
		TargetKind: "external_user", TargetID: "external-user-1", Status: wecomcapability.DispatchFailed,
		ProviderTargetID: "provider-user-1", ErrorCode: "wecom.http_401", ErrorMessageSafe: "安全失败",
	}
	if _, err := h.store.RecordCapabilityDispatchResult(context.Background(), h.principal, input); err != nil {
		t.Fatal(err)
	}
	var auditBefore, eventBefore int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_audits WHERE operation_id=? AND dispatch_id=?`, claimed.OperationID, claimed.ID).Scan(&auditBefore); err != nil {
		t.Fatal(err)
	}
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_events WHERE operation_id=? AND dispatch_id=?`, claimed.OperationID, claimed.ID).Scan(&eventBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.RecordCapabilityDispatchResult(context.Background(), h.principal, input); err != nil {
		t.Fatal(err)
	}
	var auditAfter, eventAfter int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_audits WHERE operation_id=? AND dispatch_id=?`, claimed.OperationID, claimed.ID).Scan(&auditAfter); err != nil {
		t.Fatal(err)
	}
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_events WHERE operation_id=? AND dispatch_id=?`, claimed.OperationID, claimed.ID).Scan(&eventAfter); err != nil {
		t.Fatal(err)
	}
	if auditAfter != auditBefore || eventAfter != eventBefore {
		t.Fatalf("exact duplicate appended ledger history: audit %d->%d event %d->%d", auditBefore, auditAfter, eventBefore, eventAfter)
	}
	input.Status = wecomcapability.DispatchSucceeded
	input.ErrorCode = ""
	input.ErrorMessageSafe = ""
	if _, err := h.store.RecordCapabilityDispatchResult(context.Background(), h.principal, input); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.RecordCapabilityDispatchResult(context.Background(), h.principal, input); err != nil {
		t.Fatal(err)
	}
	var auditTerminal, eventTerminal int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_audits WHERE operation_id=? AND dispatch_id=?`, claimed.OperationID, claimed.ID).Scan(&auditTerminal); err != nil {
		t.Fatal(err)
	}
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_events WHERE operation_id=? AND dispatch_id=?`, claimed.OperationID, claimed.ID).Scan(&eventTerminal); err != nil {
		t.Fatal(err)
	}
	if auditTerminal != auditBefore+1 || eventTerminal != eventBefore+1 {
		t.Fatalf("controlled failed-to-succeeded transition audit count=%d/%d want %d/%d", auditTerminal, eventTerminal, auditBefore+1, eventBefore+1)
	}
}
