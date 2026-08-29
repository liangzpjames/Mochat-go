package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

	values := make([]riskEvaluationRow, 0, 101)
	for id := 1; id <= 101; id++ {
		pattern := fmt.Sprintf("never-%03d", id)
		if id == 101 {
			pattern = "needle"
		}
		values = append(values, riskEvaluationRow{id: id, strategyID: id, pattern: pattern})
	}
	mock.ExpectBegin()
	expectRiskHighWater(mock, 101)
	expectRiskEvaluationBatch(mock, 0, 101, values[:100]...)
	expectRiskEvaluationBatch(mock, 100, 101, values[100:]...)
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
	expectRiskHighWater(mock, 2)
	expectRiskEvaluationBatch(mock, 0, 2,
		riskEvaluationRow{id: 1, strategyID: 10, pattern: "needle"},
		riskEvaluationRow{id: 2, strategyID: 20, pattern: "needle"},
	)
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
	expectRiskHighWater(mock, 1)
	expectRiskEvaluationBatch(mock, 0, 1, riskEvaluationRow{id: 1, strategyID: 10, pattern: "needle"})
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
	expectRiskHighWater(mock, 1)
	expectRiskEvaluationBatch(mock, 0, 1, riskEvaluationRow{id: 1, strategyID: 10, pattern: "needle"})
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

func TestEvaluateRiskMessageRollsBackFirstBatchWhenRule101Fails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	values := make([]riskEvaluationRow, 0, 101)
	for id := 1; id <= 101; id++ {
		values = append(values, riskEvaluationRow{id: id, strategyID: id, pattern: "needle"})
	}
	mock.ExpectBegin()
	expectRiskHighWater(mock, 101)
	expectRiskEvaluationBatch(mock, 0, 101, values[:100]...)
	for id := 1; id <= 100; id++ {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_risk_records")).WillReturnResult(sqlmock.NewResult(int64(id), 1))
		mock.ExpectExec(`UPDATE mochat_go_risk_rules SET trigger_count=trigger_count\+1`).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	expectRiskEvaluationBatch(mock, 100, 101, values[100:]...)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_risk_records")).WillReturnError(errors.New("injected rule 101 failure"))
	mock.ExpectRollback()

	created, err := NewMySQLStore(db).EvaluateRiskMessage(context.Background(), dashboard.RiskMessage{TenantID: 11, CorpID: 27, MessageID: "cross-batch-rollback", ConversationType: "single", Content: "needle"})
	if err == nil || created != 0 {
		t.Fatalf("created=%d err=%v", created, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRiskRuleBatchForEvaluationUsesBoundedHighWaterKeyset(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)FROM mochat_go_risk_rules.*tenant_id=\?.*corp_id=\?.*status='enabled'.*id>\?.*id<=\?.*ORDER BY id LIMIT \? FOR UPDATE`).
		WithArgs(11, 27, int64(100), int64(250), 100).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "corp_id", "name", "status", "subject", "whitelist_json", "ai_insight_enabled", "trigger_count"}).
			AddRow(101, 11, 27, "rule-101", "enabled", "both", []byte(`[]`), 0, 0).
			AddRow(102, 11, 27, "rule-102", "enabled", "both", []byte(`[]`), 0, 0))
	mock.ExpectQuery(`(?s)FROM mochat_go_risk_rule_strategies.*rule_id IN \(\?,\?\).*ORDER BY rule_id,id FOR UPDATE`).
		WithArgs(int64(101), int64(102)).
		WillReturnRows(sqlmock.NewRows([]string{"rule_id", "id", "behavior", "pattern", "notify_type", "risk_level"}).
			AddRow(101, 1001, "sensitive_word", "first", "none", "high").
			AddRow(102, 1002, "sensitive_word", "second", "none", "high"))

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	rules, cursor, err := riskRuleBatchForEvaluation(context.Background(), tx, 11, 27, 100, 250, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 || cursor != 102 {
		t.Fatalf("rules=%d cursor=%d", len(rules), cursor)
	}
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateRiskMessageRejectsInvalidOccurredAtBeforeWriting(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	created, err := NewMySQLStore(db).EvaluateRiskMessage(context.Background(), dashboard.RiskMessage{
		TenantID: 11, CorpID: 27, MessageID: "invalid-time", Content: "needle", OccurredAt: "not-rfc3339",
	})
	if err == nil || created != 0 || !strings.Contains(err.Error(), "发生时间") {
		t.Fatalf("created=%d err=%v", created, err)
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
	mock.ExpectQuery(`SELECT id FROM mochat_go_risk_rules.*FOR UPDATE`).
		WithArgs(int64(77), int64(11), int64(27)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(77))
	mock.ExpectExec(`UPDATE mochat_go_risk_rules SET name=\?,status=\?,subject=\?.* WHERE id=\? AND tenant_id=\? AND corp_id=\?`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT id,behavior FROM mochat_go_risk_rule_strategies.*FOR UPDATE`).
		WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "behavior"}).AddRow(9001, "sensitive_word"))
	mock.ExpectExec(`UPDATE mochat_go_risk_rule_strategies SET pattern=\?,notify_type=\?,risk_level=\? WHERE id=\? AND rule_id=\?`).
		WithArgs("needle", "none", "high", int64(9001), int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := NewMySQLStore(db).UpdateRiskRule(context.Background(), dashboard.RiskRule{
		ID: 77, TenantID: 11, CorpID: 27, Name: "scoped", Status: dashboard.RiskRuleEnabled, Subject: dashboard.RiskSubjectBoth,
		Strategies: []dashboard.RiskRuleStrategy{{Behavior: "sensitive_word", Pattern: "needle", NotifyType: "none", RiskLevel: "high"}},
	})
	if err != nil || !updated {
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

func expectRiskHighWater(mock sqlmock.Sqlmock, highWater int64) {
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(id\),0\) FROM mochat_go_risk_rules`).
		WithArgs(11, 27).
		WillReturnRows(sqlmock.NewRows([]string{"high_water"}).AddRow(highWater))
}

func expectRiskEvaluationBatch(mock sqlmock.Sqlmock, cursor, highWater int64, values ...riskEvaluationRow) {
	ruleRows := sqlmock.NewRows([]string{"id", "tenant_id", "corp_id", "name", "status", "subject", "whitelist_json", "ai_insight_enabled", "trigger_count"})
	strategyRows := sqlmock.NewRows([]string{"rule_id", "id", "behavior", "pattern", "notify_type", "risk_level"})
	strategyArgs := make([]driver.Value, 0, len(values))
	for _, value := range values {
		ruleRows.AddRow(value.id, 11, 27, fmt.Sprintf("rule-%d", value.id), "enabled", "both", []byte(`[]`), 0, 0)
		strategyRows.AddRow(value.id, value.strategyID, "sensitive_word", value.pattern, "none", "high")
		strategyArgs = append(strategyArgs, int64(value.id))
	}
	mock.ExpectQuery(`(?s)FROM mochat_go_risk_rules.*id>\?.*id<=\?.*LIMIT \? FOR UPDATE`).
		WithArgs(11, 27, cursor, highWater, 100).
		WillReturnRows(ruleRows)
	mock.ExpectQuery(`(?s)FROM mochat_go_risk_rule_strategies.*ORDER BY rule_id,id FOR UPDATE`).
		WithArgs(strategyArgs...).
		WillReturnRows(strategyRows)
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
	var strategyIDBefore, strategyIDAfter int64
	if err := db.QueryRow(`SELECT id FROM mochat_go_risk_rule_strategies WHERE rule_id=? AND behavior='sensitive_word'`, matchedRule).Scan(&strategyIDBefore); err != nil {
		t.Fatal(err)
	}
	updated, err := store.UpdateRiskRule(ctx, dashboard.RiskRule{ID: matchedRule, TenantID: 11, CorpID: 27, Name: "rule-101-updated", Status: dashboard.RiskRuleEnabled, Subject: dashboard.RiskSubjectBoth, Strategies: []dashboard.RiskRuleStrategy{{Behavior: "sensitive_word", Pattern: "needle", NotifyType: "none", RiskLevel: "medium"}}})
	if err != nil || !updated {
		t.Fatalf("update after evaluation updated=%v err=%v", updated, err)
	}
	if err := db.QueryRow(`SELECT id FROM mochat_go_risk_rule_strategies WHERE rule_id=? AND behavior='sensitive_word'`, matchedRule).Scan(&strategyIDAfter); err != nil || strategyIDAfter != strategyIDBefore {
		t.Fatalf("strategy id before=%d after=%d err=%v", strategyIDBefore, strategyIDAfter, err)
	}
	created, err = store.EvaluateRiskMessage(ctx, dashboard.RiskMessage{TenantID: 11, CorpID: 27, MessageID: "real-101", ConversationType: "single", Content: "needle"})
	if err != nil || created != 0 {
		t.Fatalf("retry after update created=%d err=%v", created, err)
	}
	var replayRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_risk_records WHERE tenant_id=11 AND corp_id=27 AND message_id='real-101'`).Scan(&replayRows); err != nil || replayRows != 1 {
		t.Fatalf("retry rows=%d err=%v", replayRows, err)
	}
	if err := db.QueryRow(`SELECT trigger_count FROM mochat_go_risk_rules WHERE id=?`, matchedRule).Scan(&triggerCount); err != nil || triggerCount != 1 {
		t.Fatalf("retry trigger_count=%d err=%v", triggerCount, err)
	}

	var rule100 int64
	if err := db.QueryRow(`SELECT id FROM mochat_go_risk_rules WHERE tenant_id=11 AND corp_id=27 ORDER BY id LIMIT 1 OFFSET 99`).Scan(&rule100); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE mochat_go_risk_rule_strategies SET pattern='needle' WHERE rule_id=?`, rule100); err != nil {
		t.Fatal(err)
	}
	triggerSQL := fmt.Sprintf(`CREATE TRIGGER task6_fail_risk_count BEFORE UPDATE ON mochat_go_risk_rules FOR EACH ROW BEGIN IF OLD.id=%d THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='task6 count failure'; END IF; END`, matchedRule)
	if _, err := db.Exec(triggerSQL); err != nil {
		t.Fatal(err)
	}
	created, err = store.EvaluateRiskMessage(ctx, dashboard.RiskMessage{TenantID: 11, CorpID: 27, MessageID: "real-cross-batch-rollback", ConversationType: "single", Content: "needle"})
	if err == nil || created != 0 {
		t.Fatalf("created=%d err=%v, want rollback", created, err)
	}
	if _, err := db.Exec(`DROP TRIGGER task6_fail_risk_count`); err != nil {
		t.Fatal(err)
	}
	var rollbackRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_risk_records WHERE message_id='real-cross-batch-rollback'`).Scan(&rollbackRows); err != nil || rollbackRows != 0 {
		t.Fatalf("rollback rows=%d err=%v", rollbackRows, err)
	}
	var rule100Count int64
	if err := db.QueryRow(`SELECT trigger_count FROM mochat_go_risk_rules WHERE id=?`, rule100).Scan(&rule100Count); err != nil || rule100Count != 0 {
		t.Fatalf("rule100 trigger_count=%d err=%v", rule100Count, err)
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

	disabledRuleID, _ := insertTask6RiskRule(t, db, "disable-race", "disable-before-evaluate")
	disableTx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var lockedRuleID int64
	if err := disableTx.QueryRowContext(ctx, `SELECT id FROM mochat_go_risk_rules WHERE id=? FOR UPDATE`, disabledRuleID).Scan(&lockedRuleID); err != nil {
		t.Fatal(err)
	}
	type evaluationResult struct {
		created int
		err     error
	}
	disableResult := make(chan evaluationResult, 1)
	go func() {
		created, err := store.EvaluateRiskMessage(ctx, dashboard.RiskMessage{TenantID: 11, CorpID: 27, MessageID: "disabled-linearized-first", ConversationType: "single", Content: "disable-before-evaluate"})
		disableResult <- evaluationResult{created: created, err: err}
	}()
	waitForTask6RiskRuleLock(t, db)
	if _, err := disableTx.ExecContext(ctx, `UPDATE mochat_go_risk_rules SET status='disabled' WHERE id=?`, disabledRuleID); err != nil {
		t.Fatal(err)
	}
	if err := disableTx.Commit(); err != nil {
		t.Fatal(err)
	}
	resultAfterDisable := <-disableResult
	if resultAfterDisable.err != nil || resultAfterDisable.created != 0 {
		t.Fatalf("disabled rule created=%d err=%v", resultAfterDisable.created, resultAfterDisable.err)
	}
	assertTask6RiskRuleNotTriggered(t, db, disabledRuleID, "disabled-linearized-first")

	replacedRuleID, replacedStrategyID := insertTask6RiskRule(t, db, "replace-race", "old-strategy-token")
	replaceTx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := replaceTx.QueryRowContext(ctx, `SELECT id FROM mochat_go_risk_rules WHERE id=? FOR UPDATE`, replacedRuleID).Scan(&lockedRuleID); err != nil {
		t.Fatal(err)
	}
	replaceResult := make(chan evaluationResult, 1)
	go func() {
		created, err := store.EvaluateRiskMessage(ctx, dashboard.RiskMessage{TenantID: 11, CorpID: 27, MessageID: "strategy-linearized-first", ConversationType: "single", Content: "old-strategy-token"})
		replaceResult <- evaluationResult{created: created, err: err}
	}()
	waitForTask6RiskRuleLock(t, db)
	if _, err := replaceTx.ExecContext(ctx, `UPDATE mochat_go_risk_rule_strategies SET pattern='new-strategy-token' WHERE id=? AND rule_id=?`, replacedStrategyID, replacedRuleID); err != nil {
		t.Fatal(err)
	}
	if err := replaceTx.Commit(); err != nil {
		t.Fatal(err)
	}
	resultAfterReplace := <-replaceResult
	if resultAfterReplace.err != nil || resultAfterReplace.created != 0 {
		t.Fatalf("replaced strategy created=%d err=%v", resultAfterReplace.created, resultAfterReplace.err)
	}
	assertTask6RiskRuleNotTriggered(t, db, replacedRuleID, "strategy-linearized-first")
}

func insertTask6RiskRule(t *testing.T, db *sql.DB, name, pattern string) (int64, int64) {
	t.Helper()
	result, err := db.Exec(`INSERT INTO mochat_go_risk_rules(tenant_id,corp_id,name,status,subject,whitelist_json,ai_insight_enabled,created_at,updated_at) VALUES(11,27,?,'enabled','both','[]',0,NOW(6),NOW(6))`, name)
	if err != nil {
		t.Fatal(err)
	}
	ruleID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	result, err = db.Exec(`INSERT INTO mochat_go_risk_rule_strategies(rule_id,behavior,pattern,notify_type,risk_level,created_at) VALUES(?,'sensitive_word',?,'none','high',NOW(6))`, ruleID, pattern)
	if err != nil {
		t.Fatal(err)
	}
	strategyID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return ruleID, strategyID
}

func waitForTask6RiskRuleLock(t *testing.T, db *sql.DB) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE ID<>CONNECTION_ID() AND DB=DATABASE() AND COMMAND<>'Sleep' AND INFO LIKE '%FROM mochat_go_risk_rules%' AND INFO LIKE '%FOR UPDATE%'`).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("risk evaluation did not block on the expected rule lock")
}

func assertTask6RiskRuleNotTriggered(t *testing.T, db *sql.DB, ruleID int64, messageID string) {
	t.Helper()
	var records, triggerCount int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_risk_records WHERE tenant_id=11 AND corp_id=27 AND rule_id=? AND message_id=?`, ruleID, messageID).Scan(&records); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT trigger_count FROM mochat_go_risk_rules WHERE id=?`, ruleID).Scan(&triggerCount); err != nil {
		t.Fatal(err)
	}
	if records != 0 || triggerCount != 0 {
		t.Fatalf("rule=%d records=%d trigger_count=%d", ruleID, records, triggerCount)
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
