package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/migration"

	"github.com/go-sql-driver/mysql"
)

var weWorkCallbackInboxSchemaSequence atomic.Int64

func TestMySQLStoreWeWorkCallbackInboxConcurrentAcceptanceAndLeaseFencing(t *testing.T) {
	store, db, runner := newWeWorkCallbackInboxIntegrationStore(t)
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
	stale := first
	stale.LeaseToken = "stale-token"
	if err := store.CompleteWeWorkCallback(context.Background(), stale); !errors.Is(err, dashboard.ErrWeWorkCallbackLeaseLost) {
		t.Fatalf("stale completion error=%v", err)
	}
	if dead, err := store.FailWeWorkCallback(context.Background(), first, dashboard.ErrWeWorkCallbackDependencyUnavailable.Error()+`: dependency=redis password=callback-secret-value`, 3, 20*time.Millisecond); err != nil || dead {
		t.Fatalf("first failure dead=%t err=%v", dead, err)
	}
	var safeLastError, retryStatus string
	var hasNextAttempt bool
	if err := db.QueryRow(`SELECT status,last_error,next_attempt_at IS NOT NULL FROM mochat_go_wework_callback_inbox WHERE event_key=?`, eventKey).Scan(&retryStatus, &safeLastError, &hasNextAttempt); err != nil {
		t.Fatal(err)
	}
	if retryStatus != "pending" || !hasNextAttempt || !strings.Contains(safeLastError, dashboard.ErrWeWorkCallbackDependencyUnavailable.Error()) || strings.Contains(safeLastError, "callback-secret-value") {
		t.Fatalf("durable retry status=%q next_attempt=%t last_error=%q", retryStatus, hasNextAttempt, safeLastError)
	}
	time.Sleep(30 * time.Millisecond)
	second, found, err := store.ClaimWeWorkCallback(context.Background(), time.Minute, 3)
	if err != nil || !found || second.LeaseFence != 2 || second.Attempt != 2 || second.LeaseToken == first.LeaseToken {
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

	rolledBack, err := runner.RollbackLast(context.Background())
	if err != nil || rolledBack != "0172_wework_callback_inbox" {
		t.Fatalf("rollback=%q err=%v", rolledBack, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='mochat_go_wework_callback_inbox'`).Scan(&rowCount); err != nil || rowCount != 0 {
		t.Fatalf("down migration table count=%d err=%v", rowCount, err)
	}
}

func newWeWorkCallbackInboxIntegrationStore(t *testing.T) (*MySQLStore, *sql.DB, *migration.Runner) {
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
	t.Cleanup(func() { _ = admin.Close() })
	schema := fmt.Sprintf("mochat_callback_0172_%d_%d", os.Getpid(), weWorkCallbackInboxSchemaSequence.Add(1))
	if _, err := admin.Exec("CREATE DATABASE `" + schema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE IF EXISTS `" + schema + "`") })
	testCfg := *cfg
	testCfg.DBName = schema
	testCfg.ParseTime = true
	db, err := sql.Open("mysql", testCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE mc_corp (
		id int(10) unsigned NOT NULL,
		tenant_id int(10) unsigned NOT NULL,
		deleted_at timestamp NULL DEFAULT NULL,
		PRIMARY KEY (id),
		UNIQUE KEY uk_callback_corp_scope (tenant_id,id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mc_corp (id,tenant_id) VALUES (1101,11)`); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("..", "..")
	runner, err := migration.NewRunner(db, []migration.Migration{{
		Version: "0172_wework_callback_inbox", Description: "durable WeWork callback inbox",
		Path:     filepath.Join(root, "deploy", "standalone", "migrations", "0172_wework_callback_inbox.up.sql"),
		DownPath: filepath.Join(root, "deploy", "standalone", "migrations", "0172_wework_callback_inbox.down.sql"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	return NewMySQLStore(db), db, runner
}
