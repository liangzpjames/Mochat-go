package store

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/mysqlconn"
)

func TestSensitiveWordsMonitorMessagesEnforcesEmployeeScope(t *testing.T) {
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

	prefix := fmt.Sprintf("monitor_scope_%d", time.Now().UnixNano())
	corpID := insertSensitiveWordFixture(t, db,
		"INSERT INTO mc_corp (name, wx_corpid, tenant_id, chat_status, created_at, updated_at) VALUES (?, ?, 0, 1, NOW(), NOW())",
		prefix+"_corp", prefix+"_wx")
	monitorID := insertSensitiveWordFixture(t, db, `
		INSERT INTO mc_sensitive_words_monitor
			(corp_id, sensitive_word_id, sensitive_word_name, source, trigger_user_id, trigger_name, sender, msg_type, content, created_at, updated_at)
		VALUES (?, 1, ?, 2, 81001, ?, ?, 1, ?, NOW(), NOW())
	`, corpID, prefix+"_word", prefix+"_employee", prefix+"_sender", `{"content":"scoped"}`)
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM mc_sensitive_words_monitor WHERE id = ?", monitorID)
		_, _ = db.Exec("DELETE FROM mc_corp WHERE id = ?", corpID)
	})

	store := NewMySQLStore(db)
	ctx := context.Background()
	allowed := dashboard.SensitiveWordsMonitorMessageFilter{
		CorpID: corpID, MonitorID: monitorID, RestrictEmployeeIDs: true, AllowedEmployeeIDs: []int{81001},
	}
	if messages, found, err := store.SensitiveWordsMonitorMessages(ctx, allowed); err != nil || !found || len(messages) != 1 {
		t.Fatalf("allowed lookup found=%t messages=%d err=%v", found, len(messages), err)
	}

	disallowed := allowed
	disallowed.AllowedEmployeeIDs = []int{81002}
	if messages, found, err := store.SensitiveWordsMonitorMessages(ctx, disallowed); err != nil || found || len(messages) != 0 {
		t.Fatalf("same-corp different employee lookup found=%t messages=%d err=%v", found, len(messages), err)
	}

	disallowed.AllowedEmployeeIDs = nil
	if messages, found, err := store.SensitiveWordsMonitorMessages(ctx, disallowed); err != nil || found || len(messages) != 0 {
		t.Fatalf("empty employee scope lookup found=%t messages=%d err=%v", found, len(messages), err)
	}

	tenantScope := allowed
	tenantScope.RestrictEmployeeIDs = false
	tenantScope.AllowedEmployeeIDs = nil
	if messages, found, err := store.SensitiveWordsMonitorMessages(ctx, tenantScope); err != nil || !found || len(messages) != 1 {
		t.Fatalf("tenant lookup found=%t messages=%d err=%v", found, len(messages), err)
	}
}
