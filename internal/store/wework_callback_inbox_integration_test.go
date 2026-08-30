package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/migration"
)

func TestMySQLStoreWeWorkCallbackSideEffectIntentIsTransactionalAndConcurrentReplayExecutesOnce(t *testing.T) {
	store, db, runner := newWeWorkCallbackSideEffectIntegrationStore(t)
	event := dashboard.WeWorkCallbackEvent{
		TenantID: 11, CorpID: 1101, WxCorpID: "wx-corp-1101", EventPath: "event.change_external_contact.add_external_contact",
		Message: map[string]string{"MsgId": "side-effect-intent"}, ReceivedAt: "2026-08-30 00:00:00",
	}
	eventKey := dashboard.WeWorkCallbackEventKey(event)
	if _, err := store.AcceptWeWorkCallback(context.Background(), event, eventKey, dashboard.WeWorkCallbackPayloadFingerprint(event)); err != nil {
		t.Fatal(err)
	}
	payload := dashboard.WorkFissionCustomerPush{Sender: "go-user", ExternalUserID: "external-parent", Content: []dashboard.ContactMessageBatchSendContent{{MsgType: "text", Content: "完成任务"}}}
	payloadHash, err := dashboard.WeWorkCallbackSideEffectPayloadHash(dashboard.WeWorkCallbackActionFissionCustomerPush, &payload)
	if err != nil {
		t.Fatal(err)
	}
	execution := dashboard.WeWorkCallbackExecution{TenantID: 11, CorpID: 1101, EventKey: eventKey, LeaseFence: 1}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := insertWeWorkCallbackSideEffectIntent(context.Background(), tx, execution, dashboard.WeWorkCallbackActionFissionCustomerPush, &payload); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wework_callback_side_effects WHERE tenant_id=11 AND corp_id=1101 AND event_key=?`, eventKey).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rolled-back intent count=%d err=%v", count, err)
	}
	tx, err = db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := insertWeWorkCallbackSideEffectIntent(context.Background(), tx, execution, dashboard.WeWorkCallbackActionFissionCustomerPush, &payload); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	claim, found, err := store.ClaimWeWorkCallback(context.Background(), time.Minute, 3)
	if err != nil || !found {
		t.Fatalf("claim side-effect callback found=%t err=%v", found, err)
	}
	execution = dashboard.WeWorkCallbackExecution{TenantID: 11, CorpID: 1101, EventKey: eventKey, LeaseToken: claim.LeaseToken, LeaseFence: claim.LeaseFence}

	const concurrency = 32
	var executeCount atomic.Int64
	errCh := make(chan error, concurrency)
	var wg sync.WaitGroup
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			execute, status, err := store.BeginWeWorkCallbackSideEffect(context.Background(), execution, dashboard.WeWorkCallbackActionFissionCustomerPush, payloadHash)
			if err != nil {
				errCh <- err
				return
			}
			if execute {
				executeCount.Add(1)
			} else if status != dashboard.WeWorkCallbackSideEffectUnknown {
				errCh <- fmt.Errorf("concurrent replay status=%q", status)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	if executeCount.Load() != 1 {
		t.Fatalf("external execution owners=%d, want 1", executeCount.Load())
	}
	if err := store.CompleteWeWorkCallbackSideEffect(context.Background(), execution, dashboard.WeWorkCallbackActionFissionCustomerPush, payloadHash); err != nil {
		t.Fatal(err)
	}
	if execute, status, err := store.BeginWeWorkCallbackSideEffect(context.Background(), execution, dashboard.WeWorkCallbackActionFissionCustomerPush, payloadHash); err != nil || execute || status != dashboard.WeWorkCallbackSideEffectSent {
		t.Fatalf("sent replay execute=%t status=%q err=%v", execute, status, err)
	}
	rolledBack, err := runner.RollbackLast(context.Background())
	if err != nil || rolledBack != "0176_wework_callback_side_effect_reconciliation" {
		t.Fatalf("rollback=%q err=%v", rolledBack, err)
	}
}

func TestMySQLStoreWeWorkCallbackInboxConcurrentAcceptanceAndLeaseFencing(t *testing.T) {
	store, db := newWeWorkCallbackInboxIntegrationStore(t)
	state, err := store.WeWorkCallbackLegacyCutover(context.Background())
	if err != nil || state.Status == "completed" || state.SourceFingerprint != "" {
		t.Fatalf("initial cutover state=%+v err=%v", state, err)
	}
	sourceA := strings.Repeat("a", 64)
	sourceB := strings.Repeat("b", 64)
	ownerA := strings.Repeat("1", 64)
	ownerB := strings.Repeat("2", 64)
	ownerC := strings.Repeat("3", 64)
	if err := store.BeginWeWorkCallbackLegacyCutover(context.Background(), sourceA, ownerA); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginWeWorkCallbackLegacyCutover(context.Background(), sourceA, ownerB); !errors.Is(err, dashboard.ErrLegacyWeWorkCallbackAlreadyRunning) {
		t.Fatalf("same source concurrent begin error=%v", err)
	}
	if err := store.BeginWeWorkCallbackLegacyCutover(context.Background(), sourceB, ownerB); !errors.Is(err, dashboard.ErrLegacyWeWorkCallbackSourceMismatch) {
		t.Fatalf("different source begin error=%v", err)
	}
	if err := store.FailWeWorkCallbackLegacyCutover(context.Background(), sourceA, ownerB, 2, "wrong owner"); !errors.Is(err, dashboard.ErrLegacyWeWorkCallbackOwnerMismatch) {
		t.Fatalf("different owner failure error=%v", err)
	}
	if err := store.CompleteWeWorkCallbackLegacyCutover(context.Background(), sourceA, ownerB, 2); !errors.Is(err, dashboard.ErrLegacyWeWorkCallbackOwnerMismatch) {
		t.Fatalf("different owner completion error=%v", err)
	}
	if err := store.FailWeWorkCallbackLegacyCutover(context.Background(), sourceA, ownerA, 2, "Authorization: Bearer callback-secret-value"); err != nil {
		t.Fatal(err)
	}
	var cutoverStatus, cutoverSource, cutoverOwner, cutoverError string
	var importedCount int
	if err := db.QueryRow(`SELECT status,source_fingerprint,owner_token,imported_count,last_error FROM mochat_go_wework_callback_cutovers WHERE name=?`, dashboard.LegacyWeWorkCallbackCutoverName).Scan(&cutoverStatus, &cutoverSource, &cutoverOwner, &importedCount, &cutoverError); err != nil {
		t.Fatal(err)
	}
	if cutoverStatus != "failed" || cutoverSource != sourceA || cutoverOwner != "" || importedCount != 2 || strings.Contains(cutoverError, "callback-secret-value") {
		t.Fatalf("failed cutover status=%q source=%q owner=%q imported=%d error=%q", cutoverStatus, cutoverSource, cutoverOwner, importedCount, cutoverError)
	}
	if err := store.BeginWeWorkCallbackLegacyCutover(context.Background(), sourceA, ownerC); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteWeWorkCallbackLegacyCutover(context.Background(), sourceA, ownerA, 3); !errors.Is(err, dashboard.ErrLegacyWeWorkCallbackOwnerMismatch) {
		t.Fatalf("stale owner completion error=%v", err)
	}
	if err := store.CompleteWeWorkCallbackLegacyCutover(context.Background(), sourceA, ownerC, 3); err != nil {
		t.Fatal(err)
	}
	state, err = store.WeWorkCallbackLegacyCutover(context.Background())
	if err != nil || state.Status != "completed" || state.SourceFingerprint != sourceA || state.OwnerToken != ownerC || state.ImportedCount != 5 {
		t.Fatalf("completed cutover state=%+v err=%v", state, err)
	}
	event := dashboard.WeWorkCallbackEvent{
		TenantID: 11, CorpID: 1101, WxCorpID: "wx-corp-1101", EventPath: "event.change_contact.create_user",
		Message:    map[string]string{"ToUserName": "wx-corp-1101", "CreateTime": "1783159200", "UserID": "go-user", "Name": "Go User"},
		ReceivedAt: "2026-07-04 12:00:00",
	}
	eventKey := dashboard.WeWorkCallbackEventKey(event)
	fingerprint := dashboard.WeWorkCallbackPayloadFingerprint(event)

	const concurrency = 32
	var wg sync.WaitGroup
	var accepted atomic.Int64
	var replayed atomic.Int64
	errCh := make(chan error, concurrency)
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wasReplay, err := store.AcceptWeWorkCallback(context.Background(), event, eventKey, fingerprint)
			if err != nil {
				errCh <- err
				return
			}
			if wasReplay {
				replayed.Add(1)
			} else {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	if accepted.Load() != 1 || replayed.Load() != concurrency-1 {
		t.Fatalf("accepted=%d replayed=%d", accepted.Load(), replayed.Load())
	}
	var rowCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_wework_callback_inbox WHERE tenant_id=11 AND corp_id=1101`).Scan(&rowCount); err != nil {
		t.Fatal(err)
	}
	if rowCount != 1 {
		t.Fatalf("inbox rows=%d", rowCount)
	}
	var storedEvent string
	if err := db.QueryRow(`SELECT event_json FROM mochat_go_wework_callback_inbox WHERE tenant_id=11 AND corp_id=1101 AND event_key=?`, eventKey).Scan(&storedEvent); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"rawXml", "msg_signature", "nonce", "callback-token", "encodingAESKey"} {
		if strings.Contains(storedEvent, forbidden) {
			t.Fatalf("stored callback event contains forbidden field %q: %s", forbidden, storedEvent)
		}
	}

	conflicting := event
	conflicting.Message = map[string]string{"ToUserName": "wx-corp-1101", "CreateTime": "1783159200", "UserID": "go-user", "Name": "Changed"}
	if _, err := store.AcceptWeWorkCallback(context.Background(), conflicting, eventKey, dashboard.WeWorkCallbackPayloadFingerprint(conflicting)); !errors.Is(err, dashboard.ErrWeWorkCallbackConflict) {
		t.Fatalf("conflict error=%v", err)
	}

	first, found, err := store.ClaimWeWorkCallback(context.Background(), time.Minute, 3)
	if err != nil || !found {
		t.Fatalf("first claim found=%t err=%v", found, err)
	}
	if first.EventKey != eventKey || first.LeaseToken == "" || first.LeaseFence != 1 || first.Attempt != 1 || first.Event.Message["UserID"] != "go-user" {
		t.Fatalf("first claim=%+v", first)
	}
	if err := store.ValidateWeWorkCallbackClaim(context.Background(), first); err != nil {
		t.Fatalf("validate current claim: %v", err)
	}
	if _, found, err := store.ClaimWeWorkCallback(context.Background(), time.Minute, 3); err != nil || found {
		t.Fatalf("leased row reclaimed found=%t err=%v", found, err)
	}
	claim := first
	for deferIndex := 0; deferIndex < 5; deferIndex++ {
		stale := claim
		stale.LeaseFence++
		if err := store.DeferWeWorkCallbackDependency(context.Background(), stale, "stale dependency", time.Millisecond); !errors.Is(err, dashboard.ErrWeWorkCallbackLeaseLost) {
			t.Fatalf("stale dependency defer[%d] error=%v", deferIndex, err)
		}
		if err := store.DeferWeWorkCallbackDependency(context.Background(), claim, dashboard.ErrWeWorkCallbackDependencyUnavailable.Error()+`: dependency=redis password=callback-secret-value`, time.Millisecond); err != nil {
			t.Fatalf("dependency defer[%d] error=%v", deferIndex, err)
		}
		time.Sleep(2 * time.Millisecond)
		if deferIndex == 4 {
			break
		}
		claim, found, err = store.ClaimWeWorkCallback(context.Background(), time.Minute, 3)
		if err != nil || !found || claim.Attempt != 1 || claim.DependencyDeferCount != deferIndex+1 || claim.LeaseFence != uint64(deferIndex+2) {
			t.Fatalf("dependency reclaim[%d]=%+v found=%t err=%v", deferIndex, claim, found, err)
		}
	}
	var safeLastError, retryStatus string
	var businessAttempts, dependencyDefers int
	var hasNextAttempt bool
	if err := db.QueryRow(`SELECT status,attempt,dependency_defer_count,last_error,next_attempt_at IS NOT NULL FROM mochat_go_wework_callback_inbox WHERE event_key=?`, eventKey).Scan(&retryStatus, &businessAttempts, &dependencyDefers, &safeLastError, &hasNextAttempt); err != nil {
		t.Fatal(err)
	}
	if retryStatus != "pending" || businessAttempts != 0 || dependencyDefers != 5 || !hasNextAttempt || !strings.Contains(safeLastError, dashboard.ErrWeWorkCallbackDependencyUnavailable.Error()) || strings.Contains(safeLastError, "callback-secret-value") {
		t.Fatalf("durable defer status=%q attempts=%d defers=%d next_attempt=%t last_error=%q", retryStatus, businessAttempts, dependencyDefers, hasNextAttempt, safeLastError)
	}
	time.Sleep(2 * time.Millisecond)
	second, found, err := store.ClaimWeWorkCallback(context.Background(), time.Minute, 3)
	if err != nil || !found || second.LeaseFence != 6 || second.Attempt != 1 || second.DependencyDeferCount != 5 || second.LeaseToken == first.LeaseToken {
		t.Fatalf("second claim=%+v found=%t err=%v", second, found, err)
	}
	if err := store.CompleteWeWorkCallback(context.Background(), first); !errors.Is(err, dashboard.ErrWeWorkCallbackLeaseLost) {
		t.Fatalf("old fence completion error=%v", err)
	}
	if err := store.ValidateWeWorkCallbackClaim(context.Background(), first); !errors.Is(err, dashboard.ErrWeWorkCallbackLeaseLost) {
		t.Fatalf("old fence validation error=%v", err)
	}
	if err := store.ValidateWeWorkCallbackClaim(context.Background(), second); err != nil {
		t.Fatalf("validate current fence: %v", err)
	}
	if err := store.CompleteWeWorkCallback(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.ClaimWeWorkCallback(context.Background(), time.Minute, 3); err != nil || found {
		t.Fatalf("completed row claimed found=%t err=%v", found, err)
	}

	businessFailureEvent := event
	businessFailureEvent.Message = map[string]string{"MsgId": "business-failure-three-attempts"}
	businessFailureKey := dashboard.WeWorkCallbackEventKey(businessFailureEvent)
	if _, err := store.AcceptWeWorkCallback(context.Background(), businessFailureEvent, businessFailureKey, dashboard.WeWorkCallbackPayloadFingerprint(businessFailureEvent)); err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		businessClaim, found, err := store.ClaimWeWorkCallback(context.Background(), time.Minute, 3)
		if err != nil || !found || businessClaim.EventKey != businessFailureKey || businessClaim.Attempt != attempt || businessClaim.DependencyDeferCount != 0 {
			t.Fatalf("business failure claim[%d]=%+v found=%t err=%v", attempt, businessClaim, found, err)
		}
		dead, err := store.FailWeWorkCallback(context.Background(), businessClaim, "business validation failed", 3, time.Millisecond)
		if err != nil || dead != (attempt == 3) {
			t.Fatalf("business failure transition[%d] dead=%t err=%v", attempt, dead, err)
		}
		if !dead {
			time.Sleep(2 * time.Millisecond)
		}
	}
	var businessStatus string
	if err := db.QueryRow(`SELECT status,attempt,dependency_defer_count FROM mochat_go_wework_callback_inbox WHERE event_key=?`, businessFailureKey).Scan(&businessStatus, &businessAttempts, &dependencyDefers); err != nil {
		t.Fatal(err)
	}
	if businessStatus != "dead" || businessAttempts != 3 || dependencyDefers != 0 {
		t.Fatalf("business failure status=%q attempts=%d dependency_defers=%d", businessStatus, businessAttempts, dependencyDefers)
	}

	expiringEvent := event
	expiringEvent.Message = map[string]string{"ToUserName": "wx-corp-1101", "CreateTime": "1783159201", "UserID": "lease-expiry-user"}
	expiringKey := dashboard.WeWorkCallbackEventKey(expiringEvent)
	if _, err := store.AcceptWeWorkCallback(context.Background(), expiringEvent, expiringKey, dashboard.WeWorkCallbackPayloadFingerprint(expiringEvent)); err != nil {
		t.Fatal(err)
	}
	expired, found, err := store.ClaimWeWorkCallback(context.Background(), 5*time.Millisecond, 3)
	if err != nil || !found {
		t.Fatalf("expiring claim found=%t err=%v", found, err)
	}
	time.Sleep(20 * time.Millisecond)
	reclaimed, found, err := store.ClaimWeWorkCallback(context.Background(), time.Minute, 3)
	if err != nil || !found || reclaimed.EventKey != expiringKey || reclaimed.LeaseFence != expired.LeaseFence+1 || reclaimed.Attempt != expired.Attempt+1 {
		t.Fatalf("reclaimed=%+v found=%t err=%v", reclaimed, found, err)
	}
	if err := store.CompleteWeWorkCallback(context.Background(), expired); !errors.Is(err, dashboard.ErrWeWorkCallbackLeaseLost) {
		t.Fatalf("expired claim completion error=%v", err)
	}
	if err := store.CompleteWeWorkCallback(context.Background(), reclaimed); err != nil {
		t.Fatal(err)
	}

	maxEvent := event
	maxEvent.Message = map[string]string{"MsgId": "lease-max-attempts"}
	maxKey := dashboard.WeWorkCallbackEventKey(maxEvent)
	if _, err := store.AcceptWeWorkCallback(context.Background(), maxEvent, maxKey, dashboard.WeWorkCallbackPayloadFingerprint(maxEvent)); err != nil {
		t.Fatal(err)
	}
	firstMax, found, err := store.ClaimWeWorkCallback(context.Background(), 5*time.Millisecond, 2)
	if err != nil || !found || firstMax.Attempt != 1 {
		t.Fatalf("first max claim=%+v found=%v err=%v", firstMax, found, err)
	}
	time.Sleep(20 * time.Millisecond)
	secondMax, found, err := store.ClaimWeWorkCallback(context.Background(), 5*time.Millisecond, 2)
	if err != nil || !found || secondMax.Attempt != 2 {
		t.Fatalf("second max claim=%+v found=%v err=%v", secondMax, found, err)
	}
	time.Sleep(20 * time.Millisecond)
	if _, found, err := store.ClaimWeWorkCallback(context.Background(), time.Minute, 2); err != nil || found {
		t.Fatalf("max-attempt callback reclaimed found=%v err=%v", found, err)
	}
	var maxStatus string
	var maxAttempts int
	if err := db.QueryRow(`SELECT status,attempt FROM mochat_go_wework_callback_inbox WHERE event_key=?`, maxKey).Scan(&maxStatus, &maxAttempts); err != nil {
		t.Fatal(err)
	}
	if maxStatus != "dead" || maxAttempts != 2 {
		t.Fatalf("max-attempt row status=%q attempts=%d", maxStatus, maxAttempts)
	}

}

func newWeWorkCallbackInboxIntegrationStore(t *testing.T) (*MySQLStore, *sql.DB) {
	t.Helper()
	db := newCurrentStoreIntegrationDB(t)
	seedWeWorkCallbackIntegrationStore(t, db)
	return NewMySQLStore(db), db
}

func newWeWorkCallbackSideEffectIntegrationStore(t *testing.T) (*MySQLStore, *sql.DB, *migration.Runner) {
	t.Helper()
	db := newStoreIntegrationDBThrough(t, "0175_contact_batch_title")
	seedWeWorkCallbackIntegrationStore(t, db)
	runner := newStoreMigrationRunnerThrough(t, db, "0176_wework_callback_side_effect_reconciliation")
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	return NewMySQLStore(db), db, runner
}

func seedWeWorkCallbackIntegrationStore(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		`INSERT INTO mc_tenant (id,name,status) VALUES (11,'Callback tenant',1)`,
		`INSERT INTO mc_corp (id,tenant_id,name) VALUES (1101,11,'Callback corp')`,
		`INSERT INTO mochat_go_tenant_corp_bindings (tenant_id,corp_id,status,version) VALUES (11,1101,1,1)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}
