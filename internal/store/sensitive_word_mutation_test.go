package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/mysqlconn"
)

func TestIntegrationSensitiveWordMutationPersistsScopeConflictIdempotencyAndAudit(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_MYSQL_DSN"))
	if dsn == "" {
		if os.Getenv("MOCHAT_REQUIRE_MYSQL_INTEGRATION") == "1" {
			t.Fatal("MOCHAT_MYSQL_DSN is required")
		}
		t.Skip("MOCHAT_MYSQL_DSN is required for MySQL integration tests")
	}
	db, err := mysqlconn.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	prefix := fmt.Sprintf("task5_sensitive_%d", time.Now().UnixNano())
	tenantID := insertSensitiveWordFixture(t, db, "INSERT INTO mc_tenant (name, status) VALUES (?, 1)", prefix+"_tenant")
	corpID := insertSensitiveWordFixture(t, db, "INSERT INTO mc_corp (name, wx_corpid, tenant_id, chat_status, created_at, updated_at) VALUES (?, ?, ?, 1, NOW(), NOW())", prefix+"_corp", prefix+"_wx", tenantID)
	otherCorpID := insertSensitiveWordFixture(t, db, "INSERT INTO mc_corp (name, wx_corpid, tenant_id, chat_status, created_at, updated_at) VALUES (?, ?, ?, 1, NOW(), NOW())", prefix+"_other", prefix+"_other_wx", tenantID)
	groupID := insertSensitiveWordFixture(t, db, "INSERT INTO mc_sensitive_word_group (corp_id, name, created_at, updated_at) VALUES (?, ?, NOW(), NOW())", corpID, prefix+"_group")
	targetGroupID := insertSensitiveWordFixture(t, db, "INSERT INTO mc_sensitive_word_group (corp_id, name, created_at, updated_at) VALUES (?, ?, NOW(), NOW())", corpID, prefix+"_target_group")
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM mochat_go_saas_admin_operation_logs WHERE tenant_id = ? AND action LIKE 'sensitive_word.%'", tenantID)
		_, _ = db.Exec("DELETE FROM mochat_go_saas_admin_audit_chains WHERE tenant_id = ?", tenantID)
		_, _ = db.Exec("DELETE FROM mc_sensitive_words_monitor WHERE corp_id IN (?, ?)", corpID, otherCorpID)
		_, _ = db.Exec("DELETE FROM mc_sensitive_word WHERE corp_id IN (?, ?)", corpID, otherCorpID)
		_, _ = db.Exec("DELETE FROM mc_sensitive_word_group WHERE corp_id IN (?, ?)", corpID, otherCorpID)
		_, _ = db.Exec("DELETE FROM mc_corp WHERE id IN (?, ?)", corpID, otherCorpID)
		_, _ = db.Exec("DELETE FROM mc_tenant WHERE id = ?", tenantID)
	})

	store := NewMySQLStore(db)
	ctx := context.Background()
	create := dashboard.SensitiveWordMutation{
		Action: dashboard.SensitiveWordMutationCreateWords, TenantID: tenantID, CorpID: corpID, ActorUserID: 900001,
		GroupID: groupID, Names: []string{prefix + "_word"}, Version: "0", IdempotencyKey: prefix + "_create",
	}
	if _, err := store.MutateSensitiveWords(ctx, create); err != nil {
		t.Fatal(err)
	}
	replayed, err := store.MutateSensitiveWords(ctx, create)
	if err != nil || !replayed.Idempotent {
		t.Fatalf("idempotent replay = %#v err=%v", replayed, err)
	}
	conflictingReplay := create
	conflictingReplay.Names = []string{prefix + "_different_word"}
	_, err = store.MutateSensitiveWords(ctx, conflictingReplay)
	var operationErr *dashboard.SensitiveWordOperationError
	if !errors.As(err, &operationErr) || operationErr.Status != http.StatusConflict {
		t.Fatalf("idempotency fingerprint conflict error=%v", err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM mc_sensitive_word WHERE corp_id = ? AND name = ? AND deleted_at IS NULL", corpID, prefix+"_word").Scan(&count); err != nil || count != 1 {
		t.Fatalf("persisted word count=%d err=%v", count, err)
	}

	page, err := store.SensitiveWordPage(ctx, dashboard.SensitiveWordFilter{CorpID: corpID, GroupID: groupID, Page: 1, PerPage: 10})
	if err != nil || len(page.Items) != 1 || page.Items[0].Version == "" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	word := page.Items[0]
	result, err := store.MutateSensitiveWords(ctx, dashboard.SensitiveWordMutation{
		Action: dashboard.SensitiveWordMutationSetStatus, TenantID: tenantID, CorpID: corpID, ActorUserID: 900001,
		WordID: word.ID, Status: 2, Version: word.Version, IdempotencyKey: prefix + "_disable",
	})
	if err != nil || result.Version == "" {
		t.Fatalf("status result=%#v err=%v", result, err)
	}
	moveResult, err := store.MutateSensitiveWords(ctx, dashboard.SensitiveWordMutation{
		Action: dashboard.SensitiveWordMutationMoveWord, TenantID: tenantID, CorpID: corpID, ActorUserID: 900001,
		WordID: word.ID, GroupID: targetGroupID, Version: result.Version, IdempotencyKey: prefix + "_move_with_returned_version",
	})
	if err != nil || moveResult.Version == "" {
		t.Fatalf("mutation using returned version result=%#v err=%v", moveResult, err)
	}
	_, err = store.MutateSensitiveWords(ctx, dashboard.SensitiveWordMutation{
		Action: dashboard.SensitiveWordMutationMoveWord, TenantID: tenantID, CorpID: corpID, ActorUserID: 900001,
		WordID: word.ID, GroupID: groupID, Version: word.Version, IdempotencyKey: prefix + "_stale",
	})
	if !errors.As(err, &operationErr) || operationErr.Status != http.StatusConflict {
		t.Fatalf("stale mutation error=%v", err)
	}
	_, err = store.MutateSensitiveWords(ctx, dashboard.SensitiveWordMutation{
		Action: dashboard.SensitiveWordMutationSetStatus, TenantID: tenantID, CorpID: otherCorpID, ActorUserID: 900001,
		WordID: word.ID, Status: 1, Version: result.Version, IdempotencyKey: prefix + "_cross_corp",
	})
	if !errors.As(err, &operationErr) || operationErr.Status != http.StatusNotFound {
		t.Fatalf("cross-corp mutation error=%v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE tenant_id = ? AND action LIKE 'sensitive_word.%' AND deleted_at IS NULL", tenantID).Scan(&count); err != nil || count != 3 {
		t.Fatalf("audit count=%d err=%v", count, err)
	}
}

func TestIntegrationSensitiveWordCreateQuotaIsAtomic(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_MYSQL_DSN"))
	if dsn == "" {
		if os.Getenv("MOCHAT_REQUIRE_MYSQL_INTEGRATION") == "1" {
			t.Fatal("MOCHAT_MYSQL_DSN is required")
		}
		t.Skip("MOCHAT_MYSQL_DSN is required for MySQL integration tests")
	}
	db, err := mysqlconn.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	prefix := fmt.Sprintf("task5_sensitive_quota_%d", time.Now().UnixNano())
	tenantID := insertSensitiveWordFixture(t, db, "INSERT INTO mc_tenant (name, status) VALUES (?, 1)", prefix+"_tenant")
	corpID := insertSensitiveWordFixture(t, db, "INSERT INTO mc_corp (name, wx_corpid, tenant_id, chat_status, created_at, updated_at) VALUES (?, ?, ?, 1, NOW(), NOW())", prefix+"_corp", prefix+"_wx", tenantID)
	groupID := insertSensitiveWordFixture(t, db, "INSERT INTO mc_sensitive_word_group (corp_id, name, created_at, updated_at) VALUES (?, ?, NOW(), NOW())", corpID, prefix+"_group")
	insertSensitiveWordFixture(t, db, `
		INSERT INTO mochat_go_saas_usage_counters
			(tenant_id, metric, period_key, used_value, limit_value, updated_by, created_at, updated_at)
		VALUES (?, ?, 'lifetime', 0, 1, 'task5-test', NOW(), NOW())
	`, tenantID, dashboard.SaaSMetricSensitiveWords)
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM mochat_go_saas_admin_operation_logs WHERE tenant_id = ? AND action LIKE 'sensitive_word.%'", tenantID)
		_, _ = db.Exec("DELETE FROM mochat_go_saas_admin_audit_chains WHERE tenant_id = ?", tenantID)
		_, _ = db.Exec("DELETE FROM mochat_go_saas_usage_counters WHERE tenant_id = ?", tenantID)
		_, _ = db.Exec("DELETE FROM mc_sensitive_word WHERE corp_id = ?", corpID)
		_, _ = db.Exec("DELETE FROM mc_sensitive_word_group WHERE corp_id = ?", corpID)
		_, _ = db.Exec("DELETE FROM mc_corp WHERE id = ?", corpID)
		_, _ = db.Exec("DELETE FROM mc_tenant WHERE id = ?", tenantID)
	})

	store := NewMySQLStore(db)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			ready.Done()
			<-start
			_, mutationErr := store.MutateSensitiveWords(context.Background(), dashboard.SensitiveWordMutation{
				Action: dashboard.SensitiveWordMutationCreateWords, TenantID: tenantID, CorpID: corpID, ActorUserID: 900010 + i,
				GroupID: groupID, Names: []string{fmt.Sprintf("%s_word_%d", prefix, i)}, Version: "0", IdempotencyKey: fmt.Sprintf("%s_create_%d", prefix, i),
			})
			errs <- mutationErr
		}()
	}
	ready.Wait()
	close(start)

	successes, quotaFailures := 0, 0
	for i := 0; i < 2; i++ {
		mutationErr := <-errs
		if mutationErr == nil {
			successes++
			continue
		}
		var quotaErr *dashboard.SaaSQuotaExceededError
		if errors.As(mutationErr, &quotaErr) {
			quotaFailures++
			continue
		}
		t.Fatalf("unexpected concurrent mutation error=%v", mutationErr)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM mc_sensitive_word WHERE corp_id = ? AND deleted_at IS NULL", corpID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if successes != 1 || quotaFailures != 1 || count != 1 {
		t.Fatalf("successes=%d quotaFailures=%d persisted=%d", successes, quotaFailures, count)
	}
}

func TestSensitiveWordMutationFingerprintBindsCanonicalPayload(t *testing.T) {
	base := dashboard.SensitiveWordMutation{
		Action: dashboard.SensitiveWordMutationSetStatus, CorpID: 10, WordID: 20, GroupID: 30,
		Status: 2, Name: "name", Names: []string{" beta ", "alpha", "alpha"}, Version: " version ",
	}
	canonical := sensitiveWordMutationFingerprint(normalizeSensitiveWordMutation(base))
	equivalent := base
	equivalent.Names = []string{"alpha", "beta"}
	if got := sensitiveWordMutationFingerprint(normalizeSensitiveWordMutation(equivalent)); got != canonical {
		t.Fatalf("canonical equivalent fingerprint=%s want=%s", got, canonical)
	}
	mutations := []dashboard.SensitiveWordMutation{base, base, base, base, base, base}
	mutations[0].WordID++
	mutations[1].GroupID++
	mutations[2].Status = 1
	mutations[3].Name = "different"
	mutations[4].Names = []string{"different"}
	mutations[5].Version = "different"
	for i, mutation := range mutations {
		if got := sensitiveWordMutationFingerprint(normalizeSensitiveWordMutation(mutation)); got == canonical {
			t.Fatalf("payload mutation %d did not change fingerprint", i)
		}
	}
}

func insertSensitiveWordFixture(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	result, err := db.Exec(query, args...)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return int(id)
}
