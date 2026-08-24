//go:build integration

package aiinsight

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

const aiInsightIntegrationDSNEnv = "MOCHAT_GO_AI_INSIGHT_MYSQL_INTEGRATION_DSN"

func TestSaveInsightPersistsAndUpdatesRealMariaDB(t *testing.T) {
	db := aiInsightIntegrationDB(t)
	createAIInsightIntegrationTable(t, db)
	repository := NewSQLRepository(db)
	ctx := context.Background()

	generatedAt := time.Date(2026, 8, 24, 10, 30, 0, 123000000, time.UTC)
	insight := ConversationInsight{
		TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession, RuleID: 9, RuleVersionID: 10,
		ConversationKey: "11:1:customer", EmployeeID: 11, EmployeeName: "员工", EmployeeAvatar: "employee-avatar",
		TargetType: "1", TargetID: "customer", TargetName: "客户", TargetAvatar: "customer-avatar",
		SourceStartedAt: generatedAt.Add(-time.Minute), SourceEndedAt: generatedAt,
		SourceMessageCount: 2, SourceFingerprint: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		Status: AnalysisStatusSucceeded, Summary: "初始摘要", ResultJSON: []byte(`{"outcome":"succeeded"}`),
		Provider: "provider-a", Model: "model-a", PromptVersion: "prompt-v1", GeneratedAt: &generatedAt,
	}
	if err := repository.SaveInsight(ctx, insight); err != nil {
		t.Fatalf("first SaveInsight: %v", err)
	}

	first := loadAIInsightIntegrationRow(t, db, insight)
	assertAIInsightFirstWrite(t, first, insight)

	const preservedCreatedAt = "2000-01-02 03:04:05"
	if _, err := db.ExecContext(ctx, "UPDATE mochat_go_ai_conversation_insights SET created_at=?", preservedCreatedAt); err != nil {
		t.Fatalf("set created_at sentinel: %v", err)
	}

	updatedGeneratedAt := generatedAt.Add(time.Hour)
	insight.Status = AnalysisStatusFailed
	insight.Summary = "更新摘要"
	insight.ResultJSON = []byte(`{"outcome":"failed"}`)
	insight.ErrorSummary = "provider timeout"
	insight.Provider = "provider-b"
	insight.Model = "model-b"
	insight.PromptVersion = "prompt-v2"
	insight.GeneratedAt = &updatedGeneratedAt
	if err := repository.SaveInsight(ctx, insight); err != nil {
		t.Fatalf("duplicate SaveInsight: %v", err)
	}

	updated := loadAIInsightIntegrationRow(t, db, insight)
	if updated.Status != string(AnalysisStatusFailed) || updated.Summary != insight.Summary || updated.ErrorSummary != insight.ErrorSummary || updated.Provider != insight.Provider || updated.Model != insight.Model || updated.PromptVersion != insight.PromptVersion {
		t.Fatalf("updated row = %#v", updated)
	}
	assertAIInsightJSONEqual(t, updated.ResultJSON, insight.ResultJSON)
	if !updated.GeneratedAt.Valid || !updated.GeneratedAt.Time.Equal(updatedGeneratedAt) {
		t.Fatalf("updated generated_at = %v, want %s", updated.GeneratedAt, updatedGeneratedAt)
	}
	wantCreatedAt, err := time.Parse("2006-01-02 15:04:05", preservedCreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.CreatedAt.Equal(wantCreatedAt) {
		t.Fatalf("created_at was rebuilt: got %s, want %s", updated.CreatedAt, wantCreatedAt)
	}
}

func TestEmployeeOptionsSearchesNamesOnRealMariaDB(t *testing.T) {
	db := aiInsightIntegrationDB(t)
	createAIInsightIntegrationTable(t, db)
	if _, err := db.Exec(`CREATE TEMPORARY TABLE mc_work_employee (
		id BIGINT UNSIGNED NOT NULL,
		corp_id BIGINT UNSIGNED NOT NULL,
		name VARCHAR(255) NOT NULL,
		avatar VARCHAR(512) NOT NULL,
		deleted_at DATETIME(6) NULL,
		PRIMARY KEY (id)
	)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec("DROP TEMPORARY TABLE IF EXISTS mc_work_employee") })
	if _, err := db.Exec(`INSERT INTO mc_work_employee (id,corp_id,name,avatar) VALUES (1106,8,'AI验收员工A','avatar')`); err != nil {
		t.Fatal(err)
	}
	repository := NewSQLRepository(db)
	generatedAt := time.Date(2026, 8, 24, 15, 0, 0, 0, time.UTC)
	if err := repository.SaveInsight(context.Background(), ConversationInsight{
		TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession, RuleID: 9, RuleVersionID: 10,
		ConversationKey: "employee-option", EmployeeID: 1106, EmployeeName: "归档员工名", TargetType: "1", TargetID: "customer", TargetName: "客户",
		SourceStartedAt: generatedAt.Add(-time.Minute), SourceEndedAt: generatedAt, SourceMessageCount: 2,
		SourceFingerprint: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		Status:            AnalysisStatusSucceeded, Summary: "摘要", ResultJSON: []byte(`{}`), PromptVersion: "prompt-v1", GeneratedAt: &generatedAt,
	}); err != nil {
		t.Fatal(err)
	}
	options, err := repository.EmployeeOptions(context.Background(), EmployeeOptionFilter{
		TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession, EmployeeKeyword: "AI验收员工", Limit: 20,
	})
	if err != nil {
		t.Fatalf("EmployeeOptions: %v", err)
	}
	if len(options) != 1 || options[0].ID != 1106 || options[0].Name != "AI验收员工A" {
		t.Fatalf("options = %#v", options)
	}
}

type aiInsightIntegrationRow struct {
	TenantID, CorpID, RuleVersionID int64
	AnalysisType, ConversationKey   string
	Status, Summary, ErrorSummary   string
	ResultJSON                      []byte
	Provider, Model, PromptVersion  string
	GeneratedAt                     sql.NullTime
	CreatedAt                       time.Time
}

func aiInsightIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(aiInsightIntegrationDSNEnv))
	if dsn == "" {
		if os.Getenv("MOCHAT_REQUIRE_MYSQL_INTEGRATION") == "1" {
			t.Fatalf("%s is required when MOCHAT_REQUIRE_MYSQL_INTEGRATION=1", aiInsightIntegrationDSNEnv)
		}
		t.Skipf("%s is required for MariaDB integration tests", aiInsightIntegrationDSNEnv)
	}
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid AI insight integration DSN")
	}
	if !isAIInsightIntegrationSchema(config.DBName) {
		t.Fatal("AI insight integration DSN must target a dedicated test schema")
	}
	config.ParseTime = true
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		t.Fatal("connect AI insight integration database")
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func isAIInsightIntegrationSchema(name string) bool {
	name = strings.TrimSpace(name)
	const prefix = "mochat_go_ai_insight_"
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	suffix := strings.TrimPrefix(name, prefix)
	if suffix == "test" || suffix == "integration" {
		return true
	}
	for _, allowedPrefix := range []string{"test_", "integration_"} {
		if strings.HasPrefix(suffix, allowedPrefix) {
			return isAIInsightIntegrationSchemaSuffix(strings.TrimPrefix(suffix, allowedPrefix))
		}
	}
	return false
}

func isAIInsightIntegrationSchemaSuffix(suffix string) bool {
	parts := strings.Split(suffix, "_")
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, char := range part {
			if char < 'a' || char > 'z' {
				if char < '0' || char > '9' {
					return false
				}
			}
		}
	}
	return true
}

func TestIsAIInsightIntegrationSchemaAcceptsOnlyDedicatedSchemaNames(t *testing.T) {
	for _, name := range []string{
		"mochat_go_ai_insight_test",
		"mochat_go_ai_insight_test_local",
		"mochat_go_ai_insight_integration",
		"mochat_go_ai_insight_integration_ci",
	} {
		if !isAIInsightIntegrationSchema(name) {
			t.Fatalf("dedicated integration schema %q was rejected", name)
		}
	}
	for _, name := range []string{
		"production-integration",
		"customer_test_backup",
		"mochat_go_ai_insight_tests",
		"mochat_go_ai_insight_integration-prod",
		"MOCHAT_GO_AI_INSIGHT_TEST",
	} {
		if isAIInsightIntegrationSchema(name) {
			t.Fatalf("non-dedicated integration schema %q was accepted", name)
		}
	}
}

func createAIInsightIntegrationTable(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("DROP TEMPORARY TABLE IF EXISTS mochat_go_ai_conversation_insights"); err != nil {
		t.Fatal(err)
	}
	_, err := db.Exec(`CREATE TEMPORARY TABLE mochat_go_ai_conversation_insights (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
 tenant_id BIGINT UNSIGNED NOT NULL,
 corp_id BIGINT UNSIGNED NOT NULL,
 analysis_type VARCHAR(24) NOT NULL,
 rule_id BIGINT UNSIGNED NOT NULL,
 rule_version_id BIGINT UNSIGNED NOT NULL,
 conversation_key VARCHAR(191) NOT NULL,
 employee_id BIGINT UNSIGNED NOT NULL,
 employee_name VARCHAR(120) NOT NULL,
 employee_avatar VARCHAR(512) NOT NULL,
 target_type VARCHAR(24) NOT NULL,
 target_id VARCHAR(191) NOT NULL,
 target_name VARCHAR(191) NOT NULL,
 target_avatar VARCHAR(512) NOT NULL,
 source_started_at DATETIME(6) NULL,
 source_ended_at DATETIME(6) NULL,
 source_message_count INT UNSIGNED NOT NULL,
 source_fingerprint CHAR(64) NOT NULL,
 status VARCHAR(16) NOT NULL,
 summary VARCHAR(1200) NOT NULL,
 result_json JSON NOT NULL,
 error_summary VARCHAR(500) NOT NULL,
 provider VARCHAR(64) NOT NULL,
 model VARCHAR(128) NOT NULL,
 prompt_version VARCHAR(32) NOT NULL,
 generated_at DATETIME(6) NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
 PRIMARY KEY (id),
 UNIQUE KEY uq_ai_conversation_source (tenant_id,corp_id,analysis_type,rule_version_id,conversation_key,source_fingerprint)
)`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec("DROP TEMPORARY TABLE IF EXISTS mochat_go_ai_conversation_insights"); err != nil {
			t.Errorf("drop temporary AI insight table: %v", err)
		}
	})
}

func loadAIInsightIntegrationRow(t *testing.T, db *sql.DB, insight ConversationInsight) aiInsightIntegrationRow {
	t.Helper()
	var row aiInsightIntegrationRow
	err := db.QueryRow(`SELECT tenant_id,corp_id,analysis_type,rule_version_id,conversation_key,status,summary,result_json,error_summary,provider,model,prompt_version,generated_at,created_at
 FROM mochat_go_ai_conversation_insights
 WHERE tenant_id=? AND corp_id=? AND analysis_type=? AND rule_version_id=? AND conversation_key=? AND source_fingerprint=?`,
		insight.TenantID, insight.CorpID, insight.AnalysisType, insight.RuleVersionID, insight.ConversationKey, insight.SourceFingerprint,
	).Scan(&row.TenantID, &row.CorpID, &row.AnalysisType, &row.RuleVersionID, &row.ConversationKey, &row.Status, &row.Summary, &row.ResultJSON, &row.ErrorSummary, &row.Provider, &row.Model, &row.PromptVersion, &row.GeneratedAt, &row.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func assertAIInsightFirstWrite(t *testing.T, got aiInsightIntegrationRow, want ConversationInsight) {
	t.Helper()
	if got.TenantID != want.TenantID || got.CorpID != want.CorpID || got.AnalysisType != string(want.AnalysisType) || got.RuleVersionID != want.RuleVersionID || got.ConversationKey != want.ConversationKey {
		t.Fatalf("identity row = %#v", got)
	}
	if got.Status != string(want.Status) || got.Provider != want.Provider || got.Model != want.Model || got.PromptVersion != want.PromptVersion {
		t.Fatalf("first write metadata = %#v", got)
	}
	assertAIInsightJSONEqual(t, got.ResultJSON, want.ResultJSON)
	if !got.GeneratedAt.Valid || !got.GeneratedAt.Time.Equal(*want.GeneratedAt) {
		t.Fatalf("generated_at = %v, want %s", got.GeneratedAt, *want.GeneratedAt)
	}
}

func assertAIInsightJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()
	var gotJSON, wantJSON any
	if err := json.Unmarshal(got, &gotJSON); err != nil {
		t.Fatalf("decode stored result JSON: %v", err)
	}
	if err := json.Unmarshal(want, &wantJSON); err != nil {
		t.Fatalf("decode expected result JSON: %v", err)
	}
	if !reflect.DeepEqual(gotJSON, wantJSON) {
		t.Fatalf("result JSON = %s, want %s", got, want)
	}
}

func TestProjectionFiltersUseRealMariaDBJSONAndEmployeeScope(t *testing.T) {
	db := aiInsightIntegrationDB(t)
	createAIInsightIntegrationTable(t, db)
	generatedAt := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	states := []string{"positive", "neutral", "negative", "mixed", "unknown"}
	for index, label := range states {
		keywords := []string{"常规标签"}
		if label == "positive" {
			keywords = []string{`50%_\采购`, "高意向"}
		}
		insertProjectionIntegrationInsight(t, db, 7, 8, 1001, "emotion-"+label, label, index*20, keywords, generatedAt.Add(time.Duration(index)*time.Minute))
	}
	insertProjectionIntegrationInsight(t, db, 7, 8, 1002, "other-employee", "positive", 0, []string{`50%_\采购`}, generatedAt)
	insertProjectionIntegrationInsight(t, db, 9, 8, 1001, "other-tenant", "positive", 0, []string{`50%_\采购`}, generatedAt)

	for _, label := range states {
		filter := InsightFilter{TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession, Restricted: true, AllowedEmployeeIDs: []int64{1001}}
		setProjectionFilterField(t, &filter, "View", "emotion")
		setProjectionFilterField(t, &filter, "Emotion", label)
		keys := queryProjectionIntegrationKeys(t, db, filter)
		if !reflect.DeepEqual(keys, []string{"emotion-" + label}) {
			t.Fatalf("emotion=%s keys=%#v", label, keys)
		}
	}

	zero := 0
	scoreFilter := InsightFilter{TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession, Restricted: true, AllowedEmployeeIDs: []int64{1001}}
	setProjectionFilterField(t, &scoreFilter, "View", "employee-score")
	setProjectionFilterField(t, &scoreFilter, "MinScore", &zero)
	setProjectionFilterField(t, &scoreFilter, "MaxScore", &zero)
	if keys := queryProjectionIntegrationKeys(t, db, scoreFilter); !reflect.DeepEqual(keys, []string{"emotion-positive"}) {
		t.Fatalf("score=0 keys=%#v", keys)
	}

	keywordFilter := InsightFilter{TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession, Keyword: `50%_\采购`, Restricted: true, AllowedEmployeeIDs: []int64{1001}}
	setProjectionFilterField(t, &keywordFilter, "View", "communication-keyword")
	if keys := queryProjectionIntegrationKeys(t, db, keywordFilter); !reflect.DeepEqual(keys, []string{"emotion-positive"}) {
		t.Fatalf("escaped keyword keys=%#v", keys)
	}
}

func insertProjectionIntegrationInsight(t *testing.T, db *sql.DB, tenantID, corpID, employeeID int64, key, emotion string, score int, keywords []string, sourceAt time.Time) {
	t.Helper()
	resultJSON, err := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"summary":       "真实会话投影",
		"customer": map[string]any{
			"emotion":  map[string]any{"label": emotion, "reason": "真实原因", "evidenceMessageIds": []string{"m1"}},
			"keywords": keywords,
		},
		"employeeQa": map[string]any{"score": score, "dimensions": []any{}, "strengths": []any{}, "issues": []any{}, "suggestions": []any{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := strings.Repeat(string(rune('a'+len(key)%20)), 64)
	_, err = db.Exec(`INSERT INTO mochat_go_ai_conversation_insights
 (tenant_id,corp_id,analysis_type,rule_id,rule_version_id,conversation_key,employee_id,employee_name,employee_avatar,target_type,target_id,target_name,target_avatar,source_started_at,source_ended_at,source_message_count,source_fingerprint,status,summary,result_json,error_summary,provider,model,prompt_version,generated_at,created_at,updated_at)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		tenantID, corpID, AnalysisTypeSession, 0, 0, key, employeeID, "员工", "", "1", "customer-"+key, "客户", "",
		sourceAt.Add(-time.Minute), sourceAt, 2, fingerprint, AnalysisStatusSucceeded, "真实会话投影", resultJSON, "", "provider", "model", "v2", sourceAt, sourceAt, sourceAt,
	)
	if err != nil {
		t.Fatalf("insert projection fixture %s: %v", key, err)
	}
}

func queryProjectionIntegrationKeys(t *testing.T, db *sql.DB, filter InsightFilter) []string {
	t.Helper()
	where, args := insightWhere(filter)
	rows, err := db.Query("SELECT i.conversation_key FROM mochat_go_ai_conversation_insights i WHERE "+strings.Join(where, " AND ")+" ORDER BY i.conversation_key", args...)
	if err != nil {
		t.Fatalf("query projection keys: %v; where=%s args=%#v", err, strings.Join(where, " AND "), args)
	}
	defer rows.Close()
	keys := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return keys
}
