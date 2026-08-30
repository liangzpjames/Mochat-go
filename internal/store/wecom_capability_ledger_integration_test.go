package store

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

func TestMySQLStoreCapabilityLedgerPersistsScopedOperationAndStringTargets(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
	createCapabilityLedgerStoreFixture(t, db)

	store := NewMySQLStore(db)
	principal := dashboardprincipal.DashboardPrincipal{UserID: 0, TenantID: 11, CorpID: 1101}
	input := CapabilityOperationInput{
		Capability:     wecomcapability.EmployeeSync,
		Action:         wecomcapability.ActionSync,
		IdempotencyKey: "employee-sync-1",
		TargetTotal:    0,
		ActorSource:    "system",
	}
	created, err := store.CreateCapabilityOperation(context.Background(), principal, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.CredentialVersion != 1 || created.ActorUserID != 0 || created.ActorSource != "system" {
		t.Fatalf("created operation=%+v", created)
	}
	duplicate, err := store.CreateCapabilityOperation(context.Background(), principal, input)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.ID != created.ID {
		t.Fatalf("duplicate operation id=%d want %d", duplicate.ID, created.ID)
	}
	var eventCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_events WHERE tenant_id=11 AND corp_id=1101 AND operation_id=?`, created.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 {
		t.Fatalf("duplicate idempotency appended event count=%d want 1", eventCount)
	}
	differentAction := input
	differentAction.Action = wecomcapability.ActionPull
	differentActionOperation, err := store.CreateCapabilityOperation(context.Background(), principal, differentAction)
	if err != nil {
		t.Fatal(err)
	}
	if differentActionOperation.ID == created.ID || differentActionOperation.Action != wecomcapability.ActionPull {
		t.Fatalf("action was not part of idempotency scope: %+v vs %+v", differentActionOperation, created)
	}
	dispatch, err := store.CreateCapabilityDispatch(context.Background(), principal, CapabilityDispatchInput{
		OperationID: created.ID, DispatchKind: "contact", ChunkNo: 1, TargetID: "zhangsan@example", IdempotencyKey: "dispatch-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dispatch.ID == 0 || dispatch.TargetID != "zhangsan@example" || dispatch.CredentialVersion != 1 {
		t.Fatalf("created dispatch=%+v", dispatch)
	}
	duplicateDispatch, err := store.CreateCapabilityDispatch(context.Background(), principal, CapabilityDispatchInput{
		OperationID: created.ID, DispatchKind: "contact", ChunkNo: 1, TargetID: "zhangsan@example", IdempotencyKey: "dispatch-1",
	})
	if err != nil || duplicateDispatch.ID != dispatch.ID {
		t.Fatalf("duplicate dispatch=%+v err=%v", duplicateDispatch, err)
	}
	var dispatchAuditCount, dispatchAuditIDCount int
	if err := db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(dispatch_id IS NOT NULL),0) FROM mochat_go_wecom_capability_operation_audits WHERE operation_id=?`, created.ID).Scan(&dispatchAuditCount, &dispatchAuditIDCount); err != nil {
		t.Fatal(err)
	}
	if dispatchAuditCount < 2 || dispatchAuditIDCount < 1 {
		t.Fatalf("dispatch audit missing dispatch id: count=%d withID=%d", dispatchAuditCount, dispatchAuditIDCount)
	}
	claimedDispatch, err := store.ClaimCapabilityDispatch(context.Background(), principal, dispatch.ID, time.Minute)
	if err != nil || claimedDispatch.Status != wecomcapability.DispatchClaimed || claimedDispatch.Attempt != 1 || claimedDispatch.LeaseToken == "" {
		t.Fatalf("claimed dispatch=%+v err=%v", claimedDispatch, err)
	}
	for _, status := range []string{wecomcapability.DispatchSubmitting, wecomcapability.DispatchSubmitted, wecomcapability.DispatchPolling} {
		transition := CapabilityDispatchTransitionInput{
			DispatchID: dispatch.ID, Status: status, LeaseToken: claimedDispatch.LeaseToken, Attempt: claimedDispatch.Attempt,
		}
		if status == wecomcapability.DispatchSubmitted {
			transition.ProviderRequestID = "provider-request-1"
		}
		claimedDispatch, err = store.TransitionCapabilityDispatch(context.Background(), principal, transition)
		if err != nil {
			t.Fatalf("dispatch transition %s: %v", status, err)
		}
	}
	completedDispatch, err := store.TransitionCapabilityDispatch(context.Background(), principal, CapabilityDispatchTransitionInput{
		DispatchID: dispatch.ID, Status: wecomcapability.DispatchSucceeded, LeaseToken: claimedDispatch.LeaseToken, Attempt: claimedDispatch.Attempt,
		ProviderMessageID: "msg-abc-123",
	})
	if err != nil || completedDispatch.Status != wecomcapability.DispatchSucceeded || completedDispatch.ProviderMessageID != "msg-abc-123" {
		t.Fatalf("completed dispatch=%+v err=%v", completedDispatch, err)
	}
	if _, err := store.TransitionCapabilityDispatch(context.Background(), principal, CapabilityDispatchTransitionInput{
		DispatchID: dispatch.ID, Status: wecomcapability.DispatchFailed, LeaseToken: "stale-lease", Attempt: 1, LastErrorCode: "wecom.timeout",
	}); err == nil {
		t.Fatal("stale dispatch lease unexpectedly transitioned")
	}
	latest, err := store.LatestCapabilityOperations(context.Background(), principal, []string{wecomcapability.EmployeeSync})
	if err != nil {
		t.Fatal(err)
	}
	if latest[wecomcapability.EmployeeSync].ID != differentActionOperation.ID || latest[wecomcapability.EmployeeSync].Action != wecomcapability.ActionPull {
		t.Fatalf("latest=%+v", latest)
	}
	claimed, err := store.ClaimCapabilityOperation(context.Background(), principal, created.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != wecomcapability.OperationClaimed || claimed.Attempt != 1 || claimed.LeaseToken == "" {
		t.Fatalf("claimed operation=%+v", claimed)
	}
	for _, status := range []string{wecomcapability.OperationSubmitting, wecomcapability.OperationSubmitted, wecomcapability.OperationPolling} {
		claimed, err = store.TransitionCapabilityOperation(context.Background(), principal, CapabilityOperationTransitionInput{
			OperationID: created.ID, Status: status, LeaseToken: claimed.LeaseToken, Attempt: claimed.Attempt,
			TargetTotal: 0, SuccessTotal: 0, FailureTotal: 0,
		})
		if err != nil {
			t.Fatalf("transition %s: %v", status, err)
		}
	}
	terminalLeaseToken, terminalAttempt := claimed.LeaseToken, claimed.Attempt
	claimed, err = store.TransitionCapabilityOperation(context.Background(), principal, CapabilityOperationTransitionInput{
		OperationID: created.ID, Status: wecomcapability.OperationSucceeded, LeaseToken: claimed.LeaseToken,
		Attempt: claimed.Attempt, TargetTotal: 0, SuccessTotal: 0, FailureTotal: 0, ExternalSuccess: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != wecomcapability.OperationSucceeded || claimed.FinishedAt == nil || claimed.LeaseToken != "" || claimed.LeaseExpiresAt != nil {
		t.Fatalf("completed operation=%+v", claimed)
	}
	if _, err := store.RecordCapabilityOperationResult(context.Background(), principal, CapabilityOperationResultInput{
		OperationID: created.ID, LeaseToken: terminalLeaseToken, Attempt: terminalAttempt,
		TargetKind: "employee", TargetID: "terminal-result", Status: wecomcapability.DispatchSucceeded,
	}); err == nil {
		t.Fatal("terminal operation unexpectedly accepted a result")
	}
	var terminalResultCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_results WHERE operation_id=? AND target_id='terminal-result'`, created.ID).Scan(&terminalResultCount); err != nil {
		t.Fatal(err)
	}
	if terminalResultCount != 0 {
		t.Fatalf("terminal operation wrote result rows=%d", terminalResultCount)
	}
	if _, err := store.CreateCapabilityDispatch(context.Background(), principal, CapabilityDispatchInput{
		OperationID: created.ID, DispatchKind: "contact", ChunkNo: 2, TargetID: "terminal-target", IdempotencyKey: "dispatch-after-terminal",
	}); err == nil {
		t.Fatal("dispatch creation for terminal parent operation unexpectedly succeeded")
	}

	staleOperation, err := store.CreateCapabilityOperation(context.Background(), principal, CapabilityOperationInput{
		Capability: wecomcapability.EmployeeSync, Action: wecomcapability.ActionPull,
		IdempotencyKey: "stale-operation", ActorSource: "system",
	})
	if err != nil {
		t.Fatal(err)
	}
	firstClaim, err := store.ClaimCapabilityOperation(context.Background(), principal, staleOperation.ID, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	secondClaim, err := store.ClaimCapabilityOperation(context.Background(), principal, staleOperation.ID, time.Minute)
	if err != nil || secondClaim.Attempt != firstClaim.Attempt+1 || secondClaim.LeaseToken == firstClaim.LeaseToken {
		t.Fatalf("stale operation takeover=%+v first=%+v err=%v", secondClaim, firstClaim, err)
	}
	if _, err := store.TransitionCapabilityOperation(context.Background(), principal, CapabilityOperationTransitionInput{
		OperationID: staleOperation.ID, Status: wecomcapability.OperationSubmitting,
		LeaseToken: firstClaim.LeaseToken, Attempt: firstClaim.Attempt,
	}); err == nil {
		t.Fatal("expired operation lease unexpectedly transitioned")
	}
	if _, err := store.RecordCapabilityOperationResult(context.Background(), principal, CapabilityOperationResultInput{
		OperationID: staleOperation.ID, LeaseToken: firstClaim.LeaseToken, Attempt: firstClaim.Attempt,
		TargetKind: "employee", TargetID: "old-worker", Status: wecomcapability.DispatchSucceeded,
	}); err == nil {
		t.Fatal("old operation worker unexpectedly wrote a result")
	}
	var staleResultCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_results WHERE operation_id=? AND target_id='old-worker'`, staleOperation.ID).Scan(&staleResultCount); err != nil {
		t.Fatal(err)
	}
	if staleResultCount != 0 {
		t.Fatalf("old operation worker wrote result rows=%d", staleResultCount)
	}
	for index, phase := range []string{wecomcapability.OperationSubmitting, wecomcapability.OperationSubmitted, wecomcapability.OperationPolling} {
		phaseOperation, err := store.CreateCapabilityOperation(context.Background(), principal, CapabilityOperationInput{
			Capability: wecomcapability.EmployeeSync, Action: wecomcapability.ActionSync,
			IdempotencyKey: fmt.Sprintf("stale-phase-%d", index), ActorSource: "system",
		})
		if err != nil {
			t.Fatal(err)
		}
		phaseClaim, err := store.ClaimCapabilityOperation(context.Background(), principal, phaseOperation.ID, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		for _, next := range []string{wecomcapability.OperationSubmitting, wecomcapability.OperationSubmitted, wecomcapability.OperationPolling} {
			if phaseClaim.Status == phase {
				break
			}
			phaseClaim, err = store.TransitionCapabilityOperation(context.Background(), principal, CapabilityOperationTransitionInput{
				OperationID: phaseOperation.ID, Status: next, LeaseToken: phaseClaim.LeaseToken, Attempt: phaseClaim.Attempt,
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Exec(`UPDATE mochat_go_wecom_capability_operations SET lease_expires_at=DATE_SUB(NOW(6), INTERVAL 1 SECOND) WHERE tenant_id=? AND corp_id=? AND id=?`, principal.TenantID, principal.CorpID, phaseOperation.ID); err != nil {
			t.Fatal(err)
		}
		reclaimed, err := store.ClaimCapabilityOperation(context.Background(), principal, phaseOperation.ID, time.Minute)
		if err != nil || reclaimed.Status != phase || reclaimed.Attempt != phaseClaim.Attempt+1 {
			t.Fatalf("stale %s takeover=%+v prior=%+v err=%v", phase, reclaimed, phaseClaim, err)
		}
		phaseDispatch, err := store.CreateCapabilityDispatch(context.Background(), principal, CapabilityDispatchInput{
			OperationID: phaseOperation.ID, DispatchKind: "employee", ChunkNo: index + 10,
			TargetID: fmt.Sprintf("phase-target-%d", index), IdempotencyKey: fmt.Sprintf("stale-dispatch-phase-%d", index),
		})
		if err != nil {
			t.Fatal(err)
		}
		phaseDispatchClaim, err := store.ClaimCapabilityDispatch(context.Background(), principal, phaseDispatch.ID, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		for _, next := range []string{wecomcapability.DispatchSubmitting, wecomcapability.DispatchSubmitted, wecomcapability.DispatchPolling} {
			if phaseDispatchClaim.Status == next && next == phase {
				break
			}
			if phaseDispatchClaim.Status == phase {
				break
			}
			transition := CapabilityDispatchTransitionInput{
				DispatchID: phaseDispatch.ID, Status: next, LeaseToken: phaseDispatchClaim.LeaseToken, Attempt: phaseDispatchClaim.Attempt,
			}
			if next == wecomcapability.DispatchSubmitted {
				transition.ProviderRequestID = fmt.Sprintf("provider-phase-%d", index)
			}
			phaseDispatchClaim, err = store.TransitionCapabilityDispatch(context.Background(), principal, transition)
			if err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Exec(`UPDATE mochat_go_wecom_capability_dispatches SET lease_expires_at=DATE_SUB(NOW(6), INTERVAL 1 SECOND) WHERE tenant_id=? AND corp_id=? AND id=?`, principal.TenantID, principal.CorpID, phaseDispatch.ID); err != nil {
			t.Fatal(err)
		}
		reclaimedDispatch, err := store.ClaimCapabilityDispatch(context.Background(), principal, phaseDispatch.ID, time.Minute)
		if err != nil || reclaimedDispatch.Status != phase || reclaimedDispatch.Attempt != phaseDispatchClaim.Attempt+1 {
			t.Fatalf("stale dispatch %s takeover=%+v prior=%+v err=%v", phase, reclaimedDispatch, phaseDispatchClaim, err)
		}
	}

	staleDispatch, err := store.CreateCapabilityDispatch(context.Background(), principal, CapabilityDispatchInput{
		OperationID: staleOperation.ID, DispatchKind: "employee", ChunkNo: 1, TargetID: "employee-1", IdempotencyKey: "stale-dispatch",
	})
	if err != nil {
		t.Fatal(err)
	}
	firstDispatchClaim, err := store.ClaimCapabilityDispatch(context.Background(), principal, staleDispatch.ID, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	secondDispatchClaim, err := store.ClaimCapabilityDispatch(context.Background(), principal, staleDispatch.ID, time.Minute)
	if err != nil || secondDispatchClaim.Attempt != firstDispatchClaim.Attempt+1 || secondDispatchClaim.LeaseToken == firstDispatchClaim.LeaseToken {
		t.Fatalf("stale dispatch takeover=%+v first=%+v err=%v", secondDispatchClaim, firstDispatchClaim, err)
	}
	if _, err := store.TransitionCapabilityDispatch(context.Background(), principal, CapabilityDispatchTransitionInput{
		DispatchID: staleDispatch.ID, Status: wecomcapability.DispatchSubmitting,
		LeaseToken: firstDispatchClaim.LeaseToken, Attempt: firstDispatchClaim.Attempt,
	}); err == nil {
		t.Fatal("expired dispatch lease unexpectedly transitioned")
	}
	if _, err := store.CreateCapabilityOperation(context.Background(), dashboardprincipal.DashboardPrincipal{UserID: 11001, TenantID: 11, CorpID: 1101}, CapabilityOperationInput{
		Capability: wecomcapability.EmployeeSync, Action: wecomcapability.ActionSync,
		IdempotencyKey: "user-pretends-system", ActorSource: "system",
	}); err == nil {
		t.Fatal("user principal unexpectedly impersonated system actor")
	}
	var auditBeforeDelete, eventBeforeDelete int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_audits WHERE operation_id=?`, created.ID).Scan(&auditBeforeDelete); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_events WHERE operation_id=?`, created.ID).Scan(&eventBeforeDelete); err != nil {
		t.Fatal(err)
	}
	var auditDeleteErr error
	_, auditDeleteErr = db.Exec(`DELETE FROM mochat_go_wecom_capability_operations WHERE id=?`, created.ID)
	if auditDeleteErr == nil {
		t.Fatal("operation deletion unexpectedly removed append-only audit/event history")
	}
	var auditAfterDelete, eventAfterDelete int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_audits WHERE operation_id=?`, created.ID).Scan(&auditAfterDelete); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_events WHERE operation_id=?`, created.ID).Scan(&eventAfterDelete); err != nil {
		t.Fatal(err)
	}
	if auditAfterDelete != auditBeforeDelete || eventAfterDelete != eventBeforeDelete {
		t.Fatalf("append-only history changed after rejected delete: audits %d->%d events %d->%d", auditBeforeDelete, auditAfterDelete, eventBeforeDelete, eventAfterDelete)
	}
	if _, err := db.Exec(`UPDATE mochat_go_tenant_corp_bindings SET contact_credential_generation=contact_credential_generation+1 WHERE tenant_id=11 AND corp_id=1101`); err != nil {
		t.Fatal(err)
	}
	var employeeGeneration, contactGeneration, agentGeneration, callbackGeneration uint64
	if err := db.QueryRow(`SELECT employee_credential_generation,contact_credential_generation,agent_credential_generation,callback_credential_generation FROM mochat_go_tenant_corp_bindings WHERE tenant_id=11 AND corp_id=1101`).Scan(&employeeGeneration, &contactGeneration, &agentGeneration, &callbackGeneration); err != nil {
		t.Fatal(err)
	}
	if employeeGeneration != 1 || contactGeneration != 2 || agentGeneration != 1 || callbackGeneration != 1 {
		t.Fatalf("credential generations cross-contaminated: employee=%d contact=%d agent=%d callback=%d", employeeGeneration, contactGeneration, agentGeneration, callbackGeneration)
	}
	contactOperation, err := store.CreateCapabilityOperation(context.Background(), principal, CapabilityOperationInput{
		Capability: wecomcapability.ExternalContactSync, Action: wecomcapability.ActionSync, IdempotencyKey: "contact-sync-1", ActorSource: "system",
	})
	if err != nil || contactOperation.CredentialVersion != 2 {
		t.Fatalf("contact operation generation=%d err=%v", contactOperation.CredentialVersion, err)
	}
	var beforeInvalid int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wecom_capability_operations`).Scan(&beforeInvalid); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateCapabilityOperation(context.Background(), dashboardprincipal.DashboardPrincipal{TenantID: 22, CorpID: 1101}, input); err == nil {
		t.Fatal("cross-tenant corp operation unexpectedly succeeded")
	}
	var afterInvalid int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wecom_capability_operations`).Scan(&afterInvalid); err != nil {
		t.Fatal(err)
	}
	if afterInvalid != beforeInvalid {
		t.Fatalf("cross-tenant operation wrote rows: before=%d after=%d", beforeInvalid, afterInvalid)
	}
	if _, err := store.CreateCapabilityOperation(context.Background(), dashboardprincipal.DashboardPrincipal{UserID: 0, TenantID: 11, CorpID: 1101}, CapabilityOperationInput{Capability: wecomcapability.EmployeeSync, Action: wecomcapability.ActionSync, IdempotencyKey: "actor-zero", ActorSource: "user"}); err == nil {
		t.Fatal("actor user id zero unexpectedly accepted")
	}
	other, err := store.CreateCapabilityOperation(context.Background(), dashboardprincipal.DashboardPrincipal{TenantID: 22, CorpID: 2201}, input)
	if err != nil {
		t.Fatal(err)
	}
	if other.ID == created.ID || other.TenantID != 22 || other.CorpID != 2201 {
		t.Fatalf("cross-scope operation=%+v", other)
	}
	if _, err := store.CreateCapabilityOperation(context.Background(), dashboardprincipal.DashboardPrincipal{TenantID: 11, CorpID: 2201}, input); err == nil {
		t.Fatal("cross-tenant/corp binding unexpectedly accepted")
	}
}

func createCapabilityLedgerStoreFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO mc_tenant (id,name,status) VALUES (11,'tenant-11',1),(22,'tenant-22',1)`,
		`INSERT INTO mc_user(id,tenant_id,phone,name,status) VALUES (11001,11,'13800001101','user-11',1),(22001,22,'13800002201','user-22',1)`,
		`INSERT INTO mc_corp(id,tenant_id,name,wx_corpid) VALUES (1101,11,'corp-11','wx-corp-11'),(2201,22,'corp-22','wx-corp-22')`,
		`INSERT INTO mochat_go_tenant_corp_bindings(tenant_id,corp_id,status,verified_wx_corpid,verified_corp_name,verified_at) VALUES (11,1101,2,'wx-corp-11','corp-11',NOW()),(22,2201,2,'wx-corp-22','corp-22',NOW())`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("fixture statement %s: %v", statement, err)
		}
	}
}
