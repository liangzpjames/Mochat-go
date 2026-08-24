package mysql

import (
	"context"
	"errors"
	"reflect"
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
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey, ports.SessionAnalysisRuleName, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), `["direct","group"]`, 30, 2, int64(7), int64(7), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(12, 1))
	mock.ExpectExec("INSERT IGNORE INTO mochat_go_ai_analysis_rule_versions").
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT "+sessionAgentColumns+" FROM mochat_go_ai_agents WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL")).
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnRows(sessionAgentRows().AddRow("session-1", 1, 2, ports.SessionAnalysisSystemKey, ports.SessionAnalysisAssistantName, "分析企业微信会话中的客户意向、流失风险与员工服务质量。", "[]", 1, 7, 7, now, now))
	mock.ExpectQuery("SELECT id,name,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,lookback_days,minimum_messages,current_version,updated_at FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "customer_analysis_prompt", "employee_qa_prompt", "conversation_types_json", "lookback_days", "minimum_messages", "current_version", "updated_at"}).
			AddRow(12, ports.SessionAnalysisRuleName, "分析客户需求", "检查员工回答", `["direct","group"]`, 30, 2, 1, now))
	repo, _ := NewAgentRepository(db)
	agent, err := repo.EnsureSessionAssistant(context.Background(), 1, 2, 7, "session-1")
	if err != nil {
		t.Fatal(err)
	}
	if agent.SystemKey != ports.SessionAnalysisSystemKey || agent.Name != ports.SessionAnalysisAssistantName || agent.SessionAnalysisRule == nil || agent.SmartAnalysisRule != nil {
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
	expectSystemAssistantLock(mock, "session-1", ports.SessionAnalysisSystemKey, `["kb-1"]`)
	expectKnowledgeBaseLocks(mock, "kb-1")
	mock.ExpectExec("UPDATE mochat_go_ai_agents SET name=\\?, description=\\?, knowledge_base_ids=\\?, status=\\?, updated_by=\\?, updated_at=\\? WHERE id=\\? AND tenant_id=\\? AND corp_id=\\? AND system_key=\\? AND deleted_at IS NULL").
		WithArgs(ports.SessionAnalysisAssistantName, "重点识别退款风险", `["kb-1"]`, 0, int64(7), sqlmock.AnyArg(), "session-1", int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT id,objective,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,lookback_days,minimum_messages,current_version FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "objective", "customer_analysis_prompt", "employee_qa_prompt", "conversation_types_json", "lookback_days", "minimum_messages", "current_version"}).
			AddRow(12, "固定目标", "旧客户提示", "旧员工提示", `["direct"]`, 30, 2, 1))
	mock.ExpectExec("UPDATE mochat_go_ai_analysis_rules SET customer_analysis_prompt=\\?,employee_qa_prompt=\\?,conversation_types_json=\\?,lookback_days=\\?,minimum_messages=\\?,current_version=\\?,updated_by=\\?,updated_at=\\?").
		WithArgs("识别退款和流失风险", "检查员工服务风险", `["direct","group"]`, 14, 3, 2, int64(7), sqlmock.AnyArg(), int64(1), int64(2), int64(12), ports.SessionAnalysisSystemKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO mochat_go_ai_analysis_rule_versions").
		WithArgs(int64(1), int64(2), int64(12), 2, "固定目标", "识别退款和流失风险", "检查员工服务风险", `["direct","group"]`, 14, 3, int64(7), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec("INSERT INTO mochat_go_ai_settings_audits").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO mochat_go_ai_settings_audits").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT .* FROM mochat_go_ai_agents WHERE tenant_id=").WillReturnRows(sessionAgentRows().AddRow("session-1", 1, 2, ports.SessionAnalysisSystemKey, ports.SessionAnalysisAssistantName, "重点识别退款风险", `["kb-1"]`, 0, 7, 7, now, now))
	expectAgentKnowledgeStatistics(mock, 1, 0, "kb-1")
	mock.ExpectQuery("SELECT id,name,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,lookback_days,minimum_messages,current_version,updated_at FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "customer_analysis_prompt", "employee_qa_prompt", "conversation_types_json", "lookback_days", "minimum_messages", "current_version", "updated_at"}).
			AddRow(12, ports.SessionAnalysisRuleName, "识别退款和流失风险", "检查员工服务风险", `["direct","group"]`, 14, 3, 2, now))
	repo, _ := NewAgentRepository(db)
	updated, err := repo.UpdateSessionAssistant(context.Background(), ports.Agent{ID: "session-1", TenantID: 1, CorpID: 2, Name: "尝试改名", Description: "重点识别退款风险", KnowledgeBaseIDs: []string{"kb-1"}, Status: 0, UpdatedBy: 7, SessionAnalysisRule: &ports.SessionAnalysisRule{CustomerAnalysisPrompt: "识别退款和流失风险", EmployeeQAPrompt: "检查员工服务风险", ConversationTypes: []string{"direct", "group"}, LookbackDays: 14, MinimumMessages: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != ports.SessionAnalysisAssistantName || updated.Status != 0 || updated.SessionAnalysisRule == nil || updated.SessionAnalysisRule.CurrentVersion != 2 {
		t.Fatalf("updated = %#v", updated)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRepositoryRejectsNewDisabledKnowledgeBaseOnSessionAssistantUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	expectSystemAssistantLock(mock, "session-1", ports.SessionAnalysisSystemKey, `["kb-old"]`)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,status FROM mochat_go_ai_knowledge_bases WHERE tenant_id=? AND corp_id=? AND id IN (?,?) AND deleted_at IS NULL ORDER BY id FOR UPDATE")).
		WithArgs(int64(1), int64(2), "kb-disabled", "kb-old").
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).
			AddRow("kb-disabled", 0).
			AddRow("kb-old", 0))
	mock.ExpectRollback()

	repo, _ := NewAgentRepository(db)
	_, err = repo.UpdateSessionAssistant(context.Background(), ports.Agent{
		ID: "session-1", TenantID: 1, CorpID: 2, KnowledgeBaseIDs: []string{"kb-old", "kb-disabled"}, Status: 1, UpdatedBy: 7,
		SessionAnalysisRule: &ports.SessionAnalysisRule{CustomerAnalysisPrompt: "分析客户需求", EmployeeQAPrompt: "检查员工回答", ConversationTypes: []string{"direct"}, LookbackDays: 30, MinimumMessages: 2},
	})
	if err != ports.ErrKnowledgeBaseInvalid {
		t.Fatalf("error = %v, want ErrKnowledgeBaseInvalid", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRepositoryLoadsSessionAnalysisRuleWithAssistant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT "+sessionAgentColumns+" FROM mochat_go_ai_agents WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL")).
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnRows(sessionAgentRows().AddRow("session-1", 1, 2, ports.SessionAnalysisSystemKey, ports.SessionAnalysisAssistantName, "关注客户风险", "[]", 1, 7, 7, now, now))
	mock.ExpectQuery("SELECT id,name,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,lookback_days,minimum_messages,current_version,updated_at FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "customer_analysis_prompt", "employee_qa_prompt", "conversation_types_json", "lookback_days", "minimum_messages", "current_version", "updated_at"}).
			AddRow(12, ports.SessionAnalysisRuleName, "识别复购机会", "检查员工回答", `["direct","group"]`, 14, 3, 4, now))
	repo, _ := NewAgentRepository(db)
	agent, err := repo.GetSessionAssistant(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if agent.SessionAnalysisRule == nil || agent.SessionAnalysisRule.ID != 12 || agent.SessionAnalysisRule.CustomerAnalysisPrompt != "识别复购机会" || agent.SessionAnalysisRule.CurrentVersion != 4 {
		t.Fatalf("session rule = %#v", agent.SessionAnalysisRule)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRepositoryLoadsRealAssistantKnowledgeStatistics(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT "+sessionAgentColumns+" FROM mochat_go_ai_agents WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL")).
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnRows(sessionAgentRows().AddRow("session-1", 1, 2, ports.SessionAnalysisSystemKey, ports.SessionAnalysisAssistantName, "关注客户风险", `["kb-1","kb-2"]`, 1, 7, 7, now, now))
	mock.ExpectQuery("SELECT COUNT\\(DISTINCT knowledge_base.id\\), COUNT\\(DISTINCT document.id\\) FROM mochat_go_ai_knowledge_bases knowledge_base").
		WithArgs(int64(1), int64(2), "kb-1", "kb-2").
		WillReturnRows(sqlmock.NewRows([]string{"knowledge_base_count", "ready_document_count"}).AddRow(2, 7))
	expectSessionRuleRead(mock, "分析客户需求", "检查员工回答", 4, now)

	repo, _ := NewAgentRepository(db)
	agent, err := repo.GetSessionAssistant(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	value := reflect.ValueOf(agent)
	knowledgeBaseCount := value.FieldByName("KnowledgeBaseCount")
	readyDocumentCount := value.FieldByName("ReadyDocumentCount")
	if !knowledgeBaseCount.IsValid() || !readyDocumentCount.IsValid() {
		t.Fatalf("agent does not expose knowledge statistics: %#v", agent)
	}
	if knowledgeBaseCount.Int() != 2 || readyDocumentCount.Int() != 7 {
		t.Fatalf("statistics = (%d, %d), want (2, 7)", knowledgeBaseCount.Int(), readyDocumentCount.Int())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRepositoryExcludesReadyDocumentsFromDisabledKnowledgeBases(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT "+sessionAgentColumns+" FROM mochat_go_ai_agents WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL")).
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnRows(sessionAgentRows().AddRow("session-1", 1, 2, ports.SessionAnalysisSystemKey, ports.SessionAnalysisAssistantName, "关注客户风险", `["kb-disabled"]`, 1, 7, 7, now, now))
	expectAgentKnowledgeStatistics(mock, 1, 0, "kb-disabled")
	expectSessionRuleRead(mock, "分析客户需求", "检查员工回答", 4, now)

	repo, _ := NewAgentRepository(db)
	agent, err := repo.GetSessionAssistant(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if agent.KnowledgeBaseCount != 1 || agent.ReadyDocumentCount != 0 {
		t.Fatalf("statistics = (%d, %d), want (1, 0)", agent.KnowledgeBaseCount, agent.ReadyDocumentCount)
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
	expectSystemAssistantLock(mock, "session-1", ports.SessionAnalysisSystemKey, `[]`)
	mock.ExpectExec("UPDATE mochat_go_ai_agents SET name=").
		WithArgs(ports.SessionAnalysisAssistantName, "更新助手要求", "[]", 1, int64(7), sqlmock.AnyArg(), "session-1", int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT id,objective,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,lookback_days,minimum_messages,current_version FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "objective", "customer_analysis_prompt", "employee_qa_prompt", "conversation_types_json", "lookback_days", "minimum_messages", "current_version"}).
			AddRow(12, "固定目标", "分析客户需求", "检查员工回答", `["direct"]`, 30, 2, 4))
	mock.ExpectExec("INSERT INTO mochat_go_ai_settings_audits").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT .* FROM mochat_go_ai_agents WHERE tenant_id=").
		WillReturnRows(sessionAgentRows().AddRow("session-1", 1, 2, ports.SessionAnalysisSystemKey, ports.SessionAnalysisAssistantName, "更新助手要求", "[]", 1, 7, 7, now, now))
	expectSessionRuleRead(mock, "分析客户需求", "检查员工回答", 4, now)

	repo, _ := NewAgentRepository(db)
	updated, err := repo.UpdateSessionAssistant(context.Background(), ports.Agent{
		ID: "session-1", TenantID: 1, CorpID: 2, Description: "更新助手要求", Status: 1, UpdatedBy: 7,
		SessionAnalysisRule: &ports.SessionAnalysisRule{CustomerAnalysisPrompt: "分析客户需求", EmployeeQAPrompt: "检查员工回答", ConversationTypes: []string{"direct"}, LookbackDays: 30, MinimumMessages: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.SessionAnalysisRule == nil || updated.SessionAnalysisRule.CurrentVersion != 4 {
		t.Fatalf("updated = %#v", updated)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func sessionAgentRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "tenant_id", "corp_id", "system_key", "name", "description", "knowledge_base_ids", "status", "created_by", "updated_by", "created_at", "updated_at"})
}

func TestAgentRepositoryEnsuresAndReturnsExactlyTwoSystemAssistants(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE INTO mochat_go_ai_agents").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO mochat_go_ai_settings_audits").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT IGNORE INTO mochat_go_ai_agents").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO mochat_go_ai_settings_audits").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec("INSERT IGNORE INTO mochat_go_ai_analysis_rules").WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectExec("INSERT IGNORE INTO mochat_go_ai_analysis_rule_versions").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT IGNORE INTO mochat_go_ai_analysis_rules").WillReturnResult(sqlmock.NewResult(12, 1))
	mock.ExpectExec("INSERT IGNORE INTO mochat_go_ai_analysis_rule_versions").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()
	expectSystemAssistantRead(mock, ports.SessionAnalysisSystemKey, "session-1", ports.SessionAnalysisAssistantName, now)
	mock.ExpectQuery("SELECT id,name,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,lookback_days,minimum_messages,current_version,updated_at FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "customer_analysis_prompt", "employee_qa_prompt", "conversation_types_json", "lookback_days", "minimum_messages", "current_version", "updated_at"}).
			AddRow(11, ports.SessionAnalysisRuleName, "分析客户需求", "检查员工回答", `["direct","group"]`, 30, 2, 1, now))
	expectSystemAssistantRead(mock, ports.SmartAnalysisSystemKey, "smart-1", ports.SmartAnalysisAssistantName, now)
	expectSmartRuleRead(mock, "识别客户意向", 1, now)

	repo, _ := NewAgentRepository(db)
	agents, err := repo.EnsureSystemAssistants(context.Background(), 1, 2, 7, "session-1", "smart-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 2 || agents[0].SessionAnalysisRule == nil || agents[1].SmartAnalysisRule == nil || agents[0].Name != ports.SessionAnalysisAssistantName || agents[1].Name != ports.SmartAnalysisAssistantName {
		t.Fatalf("agents = %#v", agents)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRepositoryRoutesSessionUpdateByLockedSystemKeyAndVersionsBothPromptsOnce(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	mock.ExpectBegin()
	expectSystemAssistantLock(mock, "session-1", ports.SessionAnalysisSystemKey, `[]`)
	mock.ExpectExec("UPDATE mochat_go_ai_agents SET name=\\?, description=\\?, knowledge_base_ids=\\?, status=\\?, updated_by=\\?, updated_at=\\? WHERE id=\\? AND tenant_id=\\? AND corp_id=\\? AND system_key=\\? AND deleted_at IS NULL").
		WithArgs(ports.SessionAnalysisAssistantName, "新会话说明", "[]", 1, int64(7), sqlmock.AnyArg(), "session-1", int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT id,objective,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,lookback_days,minimum_messages,current_version FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "objective", "customer_analysis_prompt", "employee_qa_prompt", "conversation_types_json", "lookback_days", "minimum_messages", "current_version"}).
			AddRow(11, "固定目标", "旧客户提示", "旧员工提示", `["direct"]`, 30, 2, 4))
	mock.ExpectExec("UPDATE mochat_go_ai_analysis_rules SET customer_analysis_prompt=\\?,employee_qa_prompt=\\?,conversation_types_json=\\?,lookback_days=\\?,minimum_messages=\\?,current_version=\\?,updated_by=\\?,updated_at=\\?").
		WithArgs("新客户提示", "新员工提示", `["direct","group"]`, 14, 3, 5, int64(7), sqlmock.AnyArg(), int64(1), int64(2), int64(11), ports.SessionAnalysisSystemKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO mochat_go_ai_analysis_rule_versions").
		WithArgs(int64(1), int64(2), int64(11), 5, "固定目标", "新客户提示", "新员工提示", `["direct","group"]`, 14, 3, int64(7), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO mochat_go_ai_settings_audits").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO mochat_go_ai_settings_audits").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()
	expectSystemAssistantRead(mock, ports.SessionAnalysisSystemKey, "session-1", ports.SessionAnalysisAssistantName, now)
	mock.ExpectQuery("SELECT id,name,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,lookback_days,minimum_messages,current_version,updated_at FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "customer_analysis_prompt", "employee_qa_prompt", "conversation_types_json", "lookback_days", "minimum_messages", "current_version", "updated_at"}).
			AddRow(11, ports.SessionAnalysisRuleName, "新客户提示", "新员工提示", `["direct","group"]`, 14, 3, 5, now))

	repo, _ := NewAgentRepository(db)
	updated, err := repo.UpdateSystemAssistant(context.Background(), ports.Agent{ID: "session-1", TenantID: 1, CorpID: 2, SystemKey: ports.SmartAnalysisSystemKey, Name: "尝试改名", Description: "新会话说明", KnowledgeBaseIDs: []string{}, Status: 1, UpdatedBy: 7, SessionAnalysisRule: &ports.SessionAnalysisRule{CustomerAnalysisPrompt: "新客户提示", EmployeeQAPrompt: "新员工提示", ConversationTypes: []string{"direct", "group"}, LookbackDays: 14, MinimumMessages: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if updated.SystemKey != ports.SessionAnalysisSystemKey || updated.Name != ports.SessionAnalysisAssistantName || updated.SessionAnalysisRule == nil || updated.SessionAnalysisRule.CurrentVersion != 5 || updated.SmartAnalysisRule != nil {
		t.Fatalf("updated = %#v", updated)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRepositoryDoesNotVersionUnchangedSmartRuleOrTouchSessionRule(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	mock.ExpectBegin()
	expectSystemAssistantLock(mock, "smart-1", ports.SmartAnalysisSystemKey, `[]`)
	mock.ExpectExec("UPDATE mochat_go_ai_agents SET name=").
		WithArgs(ports.SmartAnalysisAssistantName, "新智能说明", "[]", 0, int64(7), sqlmock.AnyArg(), "smart-1", int64(1), int64(2), ports.SmartAnalysisSystemKey).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT id,objective,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,lookback_days,minimum_messages,current_version FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.DefaultSmartAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "objective", "customer_analysis_prompt", "employee_qa_prompt", "conversation_types_json", "lookback_days", "minimum_messages", "current_version"}).
			AddRow(12, "识别客户意向", nil, nil, `["direct","group"]`, 30, 2, 4))
	mock.ExpectExec("INSERT INTO mochat_go_ai_settings_audits").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	expectSystemAssistantRead(mock, ports.SmartAnalysisSystemKey, "smart-1", ports.SmartAnalysisAssistantName, now)
	expectSmartRuleRead(mock, "识别客户意向", 4, now)

	repo, _ := NewAgentRepository(db)
	updated, err := repo.UpdateSystemAssistant(context.Background(), ports.Agent{ID: "smart-1", TenantID: 1, CorpID: 2, SystemKey: ports.SessionAnalysisSystemKey, Description: "新智能说明", KnowledgeBaseIDs: []string{}, Status: 0, UpdatedBy: 7, SmartAnalysisRule: &ports.SmartAnalysisRule{Objective: "识别客户意向", ConversationTypes: []string{"group", "direct"}, LookbackDays: 30, MinimumMessages: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if updated.SystemKey != ports.SmartAnalysisSystemKey || updated.Name != ports.SmartAnalysisAssistantName || updated.SmartAnalysisRule == nil || updated.SmartAnalysisRule.CurrentVersion != 4 || updated.SessionAnalysisRule != nil {
		t.Fatalf("updated = %#v", updated)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRepositoryRollsBackSystemAssistantUpdateFailures(t *testing.T) {
	tests := []struct {
		name      string
		expectEnd func(sqlmock.Sqlmock)
	}{
		{name: "version insert", expectEnd: func(mock sqlmock.Sqlmock) {
			mock.ExpectExec("INSERT INTO mochat_go_ai_analysis_rule_versions").WillReturnError(errors.New("version failed"))
			mock.ExpectRollback()
		}},
		{name: "agent audit", expectEnd: func(mock sqlmock.Sqlmock) {
			mock.ExpectExec("INSERT INTO mochat_go_ai_settings_audits").WillReturnError(errors.New("audit failed"))
			mock.ExpectRollback()
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			expectSystemAssistantLock(mock, "session-1", ports.SessionAnalysisSystemKey, `[]`)
			mock.ExpectExec("UPDATE mochat_go_ai_agents SET name=").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT id,objective,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,lookback_days,minimum_messages,current_version FROM mochat_go_ai_analysis_rules").
				WillReturnRows(sqlmock.NewRows([]string{"id", "objective", "customer_analysis_prompt", "employee_qa_prompt", "conversation_types_json", "lookback_days", "minimum_messages", "current_version"}).
					AddRow(11, "固定目标", "旧客户提示", "旧员工提示", `["direct"]`, 30, 2, 4))
			mock.ExpectExec("UPDATE mochat_go_ai_analysis_rules SET customer_analysis_prompt=").WillReturnResult(sqlmock.NewResult(0, 1))
			if test.name == "agent audit" {
				mock.ExpectExec("INSERT INTO mochat_go_ai_analysis_rule_versions").WillReturnResult(sqlmock.NewResult(0, 1))
			}
			test.expectEnd(mock)

			repo, _ := NewAgentRepository(db)
			_, err = repo.UpdateSystemAssistant(context.Background(), ports.Agent{ID: "session-1", TenantID: 1, CorpID: 2, Description: "说明", Status: 1, UpdatedBy: 7, SessionAnalysisRule: &ports.SessionAnalysisRule{CustomerAnalysisPrompt: "新客户提示", EmployeeQAPrompt: "新员工提示", ConversationTypes: []string{"direct"}, LookbackDays: 30, MinimumMessages: 2}})
			if err == nil {
				t.Fatal("expected failure")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAgentRepositoryReturnsNotFoundWhenSystemAssistantIDIsOutsideTenant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT system_key,knowledge_base_ids FROM mochat_go_ai_agents").
		WithArgs("session-1", int64(1), int64(2), ports.SessionAnalysisSystemKey, ports.SmartAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"system_key", "knowledge_base_ids"}))
	mock.ExpectRollback()
	repo, _ := NewAgentRepository(db)
	_, err = repo.UpdateSystemAssistant(context.Background(), ports.Agent{ID: "session-1", TenantID: 1, CorpID: 2, SessionAnalysisRule: &ports.SessionAnalysisRule{}})
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func expectSystemAssistantLock(mock sqlmock.Sqlmock, id, systemKey, knowledgeBaseIDs string) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT system_key,knowledge_base_ids FROM mochat_go_ai_agents WHERE id=? AND tenant_id=? AND corp_id=? AND system_key IN (?,?) AND deleted_at IS NULL FOR UPDATE")).
		WithArgs(id, int64(1), int64(2), ports.SessionAnalysisSystemKey, ports.SmartAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"system_key", "knowledge_base_ids"}).AddRow(systemKey, knowledgeBaseIDs))
}

func expectSystemAssistantRead(mock sqlmock.Sqlmock, systemKey, id, name string, now time.Time) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT "+sessionAgentColumns+" FROM mochat_go_ai_agents WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL")).
		WithArgs(int64(1), int64(2), systemKey).
		WillReturnRows(sessionAgentRows().AddRow(id, 1, 2, systemKey, name, "说明", "[]", 1, 7, 7, now, now))
}

func expectSmartRuleRead(mock sqlmock.Sqlmock, objective string, version int, now time.Time) {
	mock.ExpectQuery("SELECT id,name,objective,conversation_types_json,lookback_days,minimum_messages,current_version,updated_at FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.DefaultSmartAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "objective", "conversation_types_json", "lookback_days", "minimum_messages", "current_version", "updated_at"}).
			AddRow(12, ports.DefaultSmartAnalysisRuleName, objective, `["direct","group"]`, 30, 2, version, now))
}

func expectSessionRuleRead(mock sqlmock.Sqlmock, customerPrompt, employeePrompt string, version int, now time.Time) {
	mock.ExpectQuery("SELECT id,name,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,lookback_days,minimum_messages,current_version,updated_at FROM mochat_go_ai_analysis_rules").
		WithArgs(int64(1), int64(2), ports.SessionAnalysisSystemKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "customer_analysis_prompt", "employee_qa_prompt", "conversation_types_json", "lookback_days", "minimum_messages", "current_version", "updated_at"}).
			AddRow(11, ports.SessionAnalysisRuleName, customerPrompt, employeePrompt, `["direct"]`, 30, 2, version, now))
}
