package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"jiyi/mochat-go/internal/dashboard"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

func TestEvaluateRiskMessageReadsEveryEnabledRuleAndCommitsAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"rule_id", "tenant_id", "corp_id", "name", "status", "subject", "whitelist_json", "ai_insight_enabled", "trigger_count", "strategy_id", "behavior", "pattern", "notify_type", "risk_level"})
	for id := 1; id <= 101; id++ {
		pattern := fmt.Sprintf("never-%03d", id)
		if id == 101 {
			pattern = "needle"
		}
		rows.AddRow(id, 11, 27, fmt.Sprintf("rule-%03d", id), "enabled", "both", []byte(`[]`), 0, 0, id, "sensitive_word", pattern, "none", "high")
	}
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)FROM mochat_go_risk_rules r.*JOIN mochat_go_risk_rule_strategies s.*r\.tenant_id=\?.*r\.corp_id=\?.*r\.status='enabled'.*ORDER BY r\.id,s\.id`).
		WithArgs(11, 27).
		WillReturnRows(rows)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_risk_records")).
		WillReturnResult(sqlmock.NewResult(901, 1))
	mock.ExpectExec(`UPDATE mochat_go_risk_rules SET trigger_count=trigger_count\+1.*tenant_id=\?.*corp_id=\?`).
		WithArgs(sqlmock.AnyArg(), int64(101), 11, 27).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	created, err := NewMySQLStore(db).EvaluateRiskMessage(context.Background(), dashboard.RiskMessage{
		TenantID: 11, CorpID: 27, MessageID: "msg-101", ConversationID: "chat-1", ConversationType: "single", Content: "contains NEEDLE",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created != 1 {
		t.Fatalf("created=%d want=1", created)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateRiskMessageRollsBackEveryRecordWhenLaterInsertFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)FROM mochat_go_risk_rules r.*JOIN mochat_go_risk_rule_strategies s`).
		WithArgs(11, 27).
		WillReturnRows(riskEvaluationRows(
			riskEvaluationRow{id: 1, strategyID: 10, pattern: "needle"},
			riskEvaluationRow{id: 2, strategyID: 20, pattern: "needle"},
		))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_risk_records")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE mochat_go_risk_rules SET trigger_count=trigger_count\+1`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_risk_records")).WillReturnError(errors.New("injected second record failure"))
	mock.ExpectRollback()

	created, err := NewMySQLStore(db).EvaluateRiskMessage(context.Background(), dashboard.RiskMessage{TenantID: 11, CorpID: 27, MessageID: "msg-rollback", ConversationType: "single", Content: "needle"})
	if err == nil || created != 0 {
		t.Fatalf("created=%d err=%v, want complete rollback", created, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateRiskMessageRollsBackWhenTriggerCountUpdateFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)FROM mochat_go_risk_rules r.*JOIN mochat_go_risk_rule_strategies s`).
		WithArgs(11, 27).
		WillReturnRows(riskEvaluationRows(riskEvaluationRow{id: 1, strategyID: 10, pattern: "needle"}))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_risk_records")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE mochat_go_risk_rules SET trigger_count=trigger_count\+1`).WillReturnError(errors.New("injected count failure"))
	mock.ExpectRollback()

	created, err := NewMySQLStore(db).EvaluateRiskMessage(context.Background(), dashboard.RiskMessage{TenantID: 11, CorpID: 27, MessageID: "msg-count", ConversationType: "single", Content: "needle"})
	if err == nil || created != 0 {
		t.Fatalf("created=%d err=%v, want complete rollback", created, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateRiskMessageDuplicateDoesNotIncrementTriggerCount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)FROM mochat_go_risk_rules r.*JOIN mochat_go_risk_rule_strategies s`).
		WithArgs(11, 27).
		WillReturnRows(riskEvaluationRows(riskEvaluationRow{id: 1, strategyID: 10, pattern: "needle"}))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_risk_records")).WillReturnError(&mysql.MySQLError{Number: 1062, Message: "duplicate risk record"})
	mock.ExpectCommit()

	created, err := NewMySQLStore(db).EvaluateRiskMessage(context.Background(), dashboard.RiskMessage{TenantID: 11, CorpID: 27, MessageID: "msg-duplicate", ConversationType: "single", Content: "needle"})
	if err != nil || created != 0 {
		t.Fatalf("created=%d err=%v", created, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateRiskRuleRollsBackRuleWhenStrategyInsertFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_risk_rules")).WillReturnResult(sqlmock.NewResult(77, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_risk_rule_strategies")).WillReturnError(errors.New("injected strategy failure"))
	mock.ExpectRollback()

	_, err = NewMySQLStore(db).CreateRiskRule(context.Background(), dashboard.RiskRule{
		TenantID: 11, CorpID: 27, Name: "atomic rule", Status: dashboard.RiskRuleEnabled, Subject: dashboard.RiskSubjectBoth,
		Strategies: []dashboard.RiskRuleStrategy{{Behavior: "sensitive_word", Pattern: "needle", NotifyType: "none", RiskLevel: "high"}},
	})
	if err == nil {
		t.Fatal("expected strategy failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type tenantScopedRiskRuleMutations interface {
	SetRiskRuleStatus(context.Context, int, int, int64, dashboard.RiskRuleStatus) (bool, error)
	DeleteRiskRule(context.Context, int, int, int64) (bool, error)
}

var _ tenantScopedRiskRuleMutations = (*MySQLStore)(nil)

func TestUpdateRiskRuleRequiresTenantAndCorpScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE mochat_go_risk_rules SET .* WHERE id=\? AND tenant_id=\? AND corp_id=\?`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	updated, err := NewMySQLStore(db).UpdateRiskRule(context.Background(), dashboard.RiskRule{
		ID: 77, TenantID: 11, CorpID: 27, Name: "scoped", Status: dashboard.RiskRuleEnabled, Subject: dashboard.RiskSubjectBoth,
		Strategies: []dashboard.RiskRuleStrategy{{Behavior: "sensitive_word", Pattern: "needle", NotifyType: "none", RiskLevel: "high"}},
	})
	if err != nil || updated {
		t.Fatalf("updated=%v err=%v", updated, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type riskEvaluationRow struct {
	id, strategyID int
	pattern        string
}

func riskEvaluationRows(values ...riskEvaluationRow) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"rule_id", "tenant_id", "corp_id", "name", "status", "subject", "whitelist_json", "ai_insight_enabled", "trigger_count", "strategy_id", "behavior", "pattern", "notify_type", "risk_level"})
	for _, value := range values {
		rows.AddRow(value.id, 11, 27, fmt.Sprintf("rule-%d", value.id), "enabled", "both", []byte(`[]`), 0, 0, value.strategyID, "sensitive_word", value.pattern, "none", "high")
	}
	return rows
}

var task6SchemaSequence atomic.Int64

func TestRiskAndKeywordAtomicityAgainstIsolatedMySQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB/MySQL DSN is required")
	}
	db := task6IntegrationDB(t, dsn)
	store := NewMySQLStore(db)
	ctx := context.Background()

	for id := 1; id <= 101; id++ {
		pattern := fmt.Sprintf("never-%03d", id)
		if id == 101 {
			pattern = "needle"
		}
		result, err := db.Exec(`INSERT INTO mochat_go_risk_rules(tenant_id,corp_id,name,status,subject,whitelist_json,ai_insight_enabled,created_at,updated_at) VALUES(11,27,?,'enabled','both','[]',0,NOW(6),NOW(6))`, fmt.Sprintf("rule-%03d", id))
		if err != nil {
			t.Fatal(err)
		}
		ruleID, _ := result.LastInsertId()
		if _, err := db.Exec(`INSERT INTO mochat_go_risk_rule_strategies(rule_id,behavior,pattern,notify_type,risk_level,created_at) VALUES(?,'sensitive_word',?,'none','high',NOW(6))`, ruleID, pattern); err != nil {
			t.Fatal(err)
		}
	}

	page, err := store.RiskRulePage(ctx, dashboard.RiskRuleFilter{TenantID: 11, CorpID: 27, Page: 1, PerPage: 100})
	if err != nil || len(page.Items) != 100 || page.Total != 101 {
		t.Fatalf("UI page=%+v err=%v", page, err)
	}
	created, err := store.EvaluateRiskMessage(ctx, dashboard.RiskMessage{TenantID: 11, CorpID: 27, MessageID: "real-101", ConversationType: "single", Content: "needle"})
	if err != nil || created != 1 {
		t.Fatalf("created=%d err=%v", created, err)
	}
	var matchedRule, triggerCount int64
	if err := db.QueryRow(`SELECT rule_id FROM mochat_go_risk_records WHERE tenant_id=11 AND corp_id=27 AND message_id='real-101'`).Scan(&matchedRule); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT trigger_count FROM mochat_go_risk_rules WHERE id=?`, matchedRule).Scan(&triggerCount); err != nil || triggerCount != 1 {
		t.Fatalf("trigger_count=%d err=%v", triggerCount, err)
	}

	if _, err := db.Exec(`CREATE TRIGGER task6_fail_risk_count BEFORE UPDATE ON mochat_go_risk_rules FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='task6 count failure'`); err != nil {
		t.Fatal(err)
	}
	created, err = store.EvaluateRiskMessage(ctx, dashboard.RiskMessage{TenantID: 11, CorpID: 27, MessageID: "real-rollback", ConversationType: "single", Content: "needle"})
	if err == nil || created != 0 {
		t.Fatalf("created=%d err=%v, want rollback", created, err)
	}
	if _, err := db.Exec(`DROP TRIGGER task6_fail_risk_count`); err != nil {
		t.Fatal(err)
	}
	var rollbackRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_risk_records WHERE message_id='real-rollback'`).Scan(&rollbackRows); err != nil || rollbackRows != 0 {
		t.Fatalf("rollback rows=%d err=%v", rollbackRows, err)
	}

	const workers = 16
	var failures atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.EvaluateRiskMessage(ctx, dashboard.RiskMessage{TenantID: 11, CorpID: 27, MessageID: "real-concurrent", ConversationType: "single", Content: "needle"}); err != nil {
				failures.Add(1)
			}
		}()
	}
	wg.Wait()
	if failures.Load() != 0 {
		t.Fatalf("concurrent failures=%d", failures.Load())
	}
	var concurrentRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_risk_records WHERE message_id='real-concurrent'`).Scan(&concurrentRows); err != nil || concurrentRows != 1 {
		t.Fatalf("concurrent rows=%d err=%v", concurrentRows, err)
	}
	if err := db.QueryRow(`SELECT trigger_count FROM mochat_go_risk_rules WHERE id=?`, matchedRule).Scan(&triggerCount); err != nil || triggerCount != 2 {
		t.Fatalf("concurrent trigger_count=%d err=%v", triggerCount, err)
	}
}

func task6IntegrationDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
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
	if err := admin.Ping(); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	schema := fmt.Sprintf("mochat_task6_%d_%d", os.Getpid(), task6SchemaSequence.Add(1))
	if _, err := admin.Exec("CREATE DATABASE `" + schema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + schema + "`")
		_ = admin.Close()
	})
	testCfg := *cfg
	testCfg.DBName = schema
	testCfg.MultiStatements = true
	db, err := sql.Open("mysql", testCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, name := range []string{"0110_risk_behavior_provider.up.sql", "0112_message_intercept_keyword_library_provider.up.sql"} {
		contents, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(contents)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	return db
}
