package mysql

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

func TestAgentRepositoryEnsuresSystemSessionAssistantIdempotently(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE INTO mochat_go_ai_agents").
		WithArgs("session-1", int64(1), int64(2), ports.SessionAnalysisSystemKey, ports.SessionAnalysisAssistantName, sqlmock.AnyArg(), "[]", 1, int64(7), int64(7), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO mochat_go_ai_settings_audits").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT IGNORE INTO mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.DefaultSmartAnalysisSystemKey, ports.DefaultSmartAnalysisRuleName, sqlmock.AnyArg(), `["direct","group"]`, 30, 2, int64(7), int64(7), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(12, 1))
	mock.ExpectExec("INSERT IGNORE INTO mochat_go_ai_analysis_rule_versions").
		WithArgs(int64(1), int64(2), ports.DefaultSmartAnalysisSystemKey).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT "+sessionAgentColumns+" FROM mochat_go_ai_agents WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL")).
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnRows(sessionAgentRows().AddRow("session-1", 1, 2, ports.SessionAnalysisSystemKey, ports.SessionAnalysisAssistantName, "分析企业微信会话中的客户意向、流失风险与员工服务质量。", "[]", 1, 7, 7, now, now))
	mock.ExpectQuery("SELECT id,name,objective,conversation_types_json,lookback_days,minimum_messages,current_version,updated_at FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.DefaultSmartAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "objective", "conversation_types_json", "lookback_days", "minimum_messages", "current_version", "updated_at"}).
			AddRow(12, ports.DefaultSmartAnalysisRuleName, "识别客户意向、沟通质量、风险信号和建议跟进动作", `["direct","group"]`, 30, 2, 1, now))
	repo, _ := NewAgentRepository(db)
	agent, err := repo.EnsureSessionAssistant(context.Background(), 1, 2, 7, "session-1")
	if err != nil {
		t.Fatal(err)
	}
	if agent.SystemKey != ports.SessionAnalysisSystemKey || agent.Name != ports.SessionAnalysisAssistantName || agent.SmartAnalysisRule == nil {
		t.Fatalf("agent = %#v", agent)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRepositoryUpdatesOnlySystemSessionAssistant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	mock.ExpectBegin()
	expectKnowledgeBaseLocks(mock, "kb-1")
	mock.ExpectExec("UPDATE mochat_go_ai_agents SET name=\\?, description=\\?, knowledge_base_ids=\\?, status=\\?, updated_by=\\?, updated_at=\\? WHERE id=\\? AND tenant_id=\\? AND corp_id=\\? AND system_key=\\? AND deleted_at IS NULL").
		WithArgs(ports.SessionAnalysisAssistantName, "重点识别退款风险", `["kb-1"]`, 0, int64(7), sqlmock.AnyArg(), "session-1", int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT id,objective,conversation_types_json,lookback_days,minimum_messages,current_version FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.DefaultSmartAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "objective", "conversation_types_json", "lookback_days", "minimum_messages", "current_version"}).
			AddRow(12, "旧目标", `["direct"]`, 30, 2, 1))
	mock.ExpectExec("UPDATE mochat_go_ai_analysis_rules SET objective=\\?,conversation_types_json=\\?,lookback_days=\\?,minimum_messages=\\?,current_version=\\?,updated_by=\\?,updated_at=\\?").
		WithArgs("识别退款和流失风险", `["direct","group"]`, 14, 3, 2, int64(7), sqlmock.AnyArg(), int64(1), int64(2), int64(12), ports.DefaultSmartAnalysisSystemKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO mochat_go_ai_analysis_rule_versions").
		WithArgs(int64(1), int64(2), int64(12), 2, "识别退款和流失风险", `["direct","group"]`, 14, 3, int64(7), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec("INSERT INTO mochat_go_ai_settings_audits").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO mochat_go_ai_settings_audits").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT .* FROM mochat_go_ai_agents WHERE tenant_id=").WillReturnRows(sessionAgentRows().AddRow("session-1", 1, 2, ports.SessionAnalysisSystemKey, ports.SessionAnalysisAssistantName, "重点识别退款风险", `["kb-1"]`, 0, 7, 7, now, now))
	mock.ExpectQuery("SELECT id,name,objective,conversation_types_json,lookback_days,minimum_messages,current_version,updated_at FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.DefaultSmartAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "objective", "conversation_types_json", "lookback_days", "minimum_messages", "current_version", "updated_at"}).
			AddRow(12, ports.DefaultSmartAnalysisRuleName, "识别退款和流失风险", `["direct","group"]`, 14, 3, 2, now))
	repo, _ := NewAgentRepository(db)
	updated, err := repo.UpdateSessionAssistant(context.Background(), ports.Agent{ID: "session-1", TenantID: 1, CorpID: 2, Name: "尝试改名", Description: "重点识别退款风险", KnowledgeBaseIDs: []string{"kb-1"}, Status: 0, UpdatedBy: 7, SmartAnalysisRule: &ports.SmartAnalysisRule{Objective: "识别退款和流失风险", ConversationTypes: []string{"direct", "group"}, LookbackDays: 14, MinimumMessages: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != ports.SessionAnalysisAssistantName || updated.Status != 0 || updated.SmartAnalysisRule == nil || updated.SmartAnalysisRule.CurrentVersion != 2 {
		t.Fatalf("updated = %#v", updated)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRepositoryLoadsDefaultSmartAnalysisRuleWithAssistant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT "+sessionAgentColumns+" FROM mochat_go_ai_agents WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL")).
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnRows(sessionAgentRows().AddRow("session-1", 1, 2, ports.SessionAnalysisSystemKey, ports.SessionAnalysisAssistantName, "关注客户风险", "[]", 1, 7, 7, now, now))
	mock.ExpectQuery("SELECT id,name,objective,conversation_types_json,lookback_days,minimum_messages,current_version,updated_at FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.DefaultSmartAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "objective", "conversation_types_json", "lookback_days", "minimum_messages", "current_version", "updated_at"}).
			AddRow(12, ports.DefaultSmartAnalysisRuleName, "识别复购机会", `["direct","group"]`, 14, 3, 4, now))
	repo, _ := NewAgentRepository(db)
	agent, err := repo.GetSessionAssistant(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if agent.SmartAnalysisRule == nil || agent.SmartAnalysisRule.ID != 12 || agent.SmartAnalysisRule.Objective != "识别复购机会" || agent.SmartAnalysisRule.CurrentVersion != 4 {
		t.Fatalf("smart rule = %#v", agent.SmartAnalysisRule)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRepositoryDoesNotCreateRuleVersionWhenDefaultRuleIsUnchanged(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE mochat_go_ai_agents SET name=").
		WithArgs(ports.SessionAnalysisAssistantName, "更新助手要求", "[]", 1, int64(7), sqlmock.AnyArg(), "session-1", int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT id,objective,conversation_types_json,lookback_days,minimum_messages,current_version FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.DefaultSmartAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "objective", "conversation_types_json", "lookback_days", "minimum_messages", "current_version"}).
			AddRow(12, "识别客户意向", `["direct"]`, 30, 2, 4))
	mock.ExpectExec("INSERT INTO mochat_go_ai_settings_audits").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT .* FROM mochat_go_ai_agents WHERE tenant_id=").
		WillReturnRows(sessionAgentRows().AddRow("session-1", 1, 2, ports.SessionAnalysisSystemKey, ports.SessionAnalysisAssistantName, "更新助手要求", "[]", 1, 7, 7, now, now))
	mock.ExpectQuery("SELECT id,name,objective,conversation_types_json,lookback_days,minimum_messages,current_version,updated_at FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.DefaultSmartAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "objective", "conversation_types_json", "lookback_days", "minimum_messages", "current_version", "updated_at"}).
			AddRow(12, ports.DefaultSmartAnalysisRuleName, "识别客户意向", `["direct"]`, 30, 2, 4, now))

	repo, _ := NewAgentRepository(db)
	updated, err := repo.UpdateSessionAssistant(context.Background(), ports.Agent{
		ID: "session-1", TenantID: 1, CorpID: 2, Description: "更新助手要求", Status: 1, UpdatedBy: 7,
		SmartAnalysisRule: &ports.SmartAnalysisRule{Objective: "识别客户意向", ConversationTypes: []string{"direct"}, LookbackDays: 30, MinimumMessages: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.SmartAnalysisRule == nil || updated.SmartAnalysisRule.CurrentVersion != 4 {
		t.Fatalf("updated = %#v", updated)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func sessionAgentRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "tenant_id", "corp_id", "system_key", "name", "description", "knowledge_base_ids", "status", "created_by", "updated_by", "created_at", "updated_at"})
}
