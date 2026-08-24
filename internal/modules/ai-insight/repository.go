package aiinsight

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

type SQLRepository struct{ db *sql.DB }

var _ Repository = (*SQLRepository)(nil)

func NewSQLRepository(db *sql.DB) *SQLRepository { return &SQLRepository{db: db} }

type archiveMessageRow struct {
	ID, MsgID, Content, EmployeeName, EmployeeAvatar, TargetName, TargetAvatar string
	ConversationKey                                                            string
	EmployeeID                                                                 int64
	TargetType                                                                 int
	TargetID                                                                   string
	SenderType                                                                 int
	Sequence                                                                   int64
	TableIndex                                                                 int
	MessageTime                                                                time.Time
}

func (r *SQLRepository) ConversationCandidates(ctx context.Context, query CandidateQuery) ([]ConversationCandidate, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("AI insight repository database is unavailable")
	}
	rows, err := r.archiveMessages(ctx, query.CorpID, query.StartAt, query.EndAt, query.AllowedEmployeeIDs, query.Restricted, "")
	if err != nil {
		return nil, err
	}
	groups := make(map[string][]archiveMessageRow)
	for _, row := range rows {
		groups[row.ConversationKey] = append(groups[row.ConversationKey], row)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return latestArchiveRow(groups[keys[i]]).MessageTime.After(latestArchiveRow(groups[keys[j]]).MessageTime)
	})
	limit := query.Limit
	if limit <= 0 {
		limit = 200
	}
	if len(keys) > limit {
		keys = keys[:limit]
	}
	result := make([]ConversationCandidate, 0, len(keys))
	for _, key := range keys {
		messages := groups[key]
		sortArchiveRows(messages)
		first, last := messages[0], messages[len(messages)-1]
		result = append(result, ConversationCandidate{
			ConversationKey: key, EmployeeID: last.EmployeeID, EmployeeName: first.EmployeeName, EmployeeAvatar: first.EmployeeAvatar,
			TargetType: fmt.Sprintf("%d", first.TargetType), TargetID: first.TargetID, TargetName: first.TargetName, TargetAvatar: first.TargetAvatar,
			SourceStartedAt: last.MessageTime, SourceEndedAt: first.MessageTime, SourceMessageCount: len(messages), SourceFingerprint: fingerprintArchiveRows(messages),
		})
	}
	return result, nil
}

func (r *SQLRepository) ConversationMessages(ctx context.Context, query ConversationWindowQuery) ([]SourceMessage, error) {
	rows, err := r.archiveMessages(ctx, query.CorpID, query.StartAt, query.EndAt, query.AllowedEmployeeIDs, query.Restricted, query.ConversationKey)
	if err != nil {
		return nil, err
	}
	sortArchiveRows(rows)
	result := make([]SourceMessage, 0, len(rows))
	limit := query.Limit
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	for _, row := range rows {
		direction := "inbound"
		if row.SenderType == 0 {
			direction = "outbound"
		}
		result = append(result, SourceMessage{ID: row.ID, ConversationKey: row.ConversationKey, MessageTime: row.MessageTime, Direction: direction, SenderID: fmt.Sprint(row.EmployeeID), SenderName: row.EmployeeName, Content: row.Content, TableIndex: row.TableIndex, Sequence: row.Sequence})
	}
	return result, nil
}

func (r *SQLRepository) archiveMessages(ctx context.Context, corpID int64, startAt, endAt time.Time, allowed []int64, restricted bool, conversationKey string) ([]archiveMessageRow, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("AI insight repository database is unavailable")
	}
	if restricted && len(uniqueInt64s(allowed)) == 0 {
		return []archiveMessageRow{}, nil
	}
	all := make([]archiveMessageRow, 0)
	for tableIndex := 1; tableIndex <= 10; tableIndex++ {
		rows, err := r.queryArchiveTable(ctx, tableIndex, corpID, startAt, endAt, allowed, restricted, conversationKey)
		if err != nil {
			// Fresh installations may not have all ten shards yet. A missing
			// table is indistinguishable from an empty shard for read purposes.
			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) && mysqlErr.Number == 1146 {
				continue
			}
			return nil, err
		}
		all = append(all, rows...)
	}
	return all, nil
}

func (r *SQLRepository) queryArchiveTable(ctx context.Context, tableIndex int, corpID int64, startAt, endAt time.Time, allowed []int64, restricted bool, conversationKey string) ([]archiveMessageRow, error) {
	table := fmt.Sprintf("mc_work_message_%d", tableIndex)
	where := []string{"wm.corp_id = ?", "wm.deleted_at IS NULL", "wm.content_text IS NOT NULL", "TRIM(wm.content_text) <> ''"}
	args := []any{corpID}
	if !startAt.IsZero() {
		where = append(where, "wm.msg_data_time >= ?")
		args = append(args, startAt)
	}
	if !endAt.IsZero() {
		where = append(where, "wm.msg_data_time <= ?")
		args = append(args, endAt)
	}
	if restricted {
		ids := uniqueInt64s(allowed)
		where = append(where, "wm.work_employee_id IN ("+placeholders(len(ids))+")")
		for _, id := range ids {
			args = append(args, id)
		}
	}
	if conversationKey != "" {
		where = append(where, "CONCAT(wm.work_employee_id, ':', wm.to_user_type, ':', COALESCE(NULLIF(wm.to_user_id, 0), NULLIF(wm.room_id, 0), 0)) = ?")
		args = append(args, conversationKey)
	}
	query := `SELECT wm.id, COALESCE(wm.msgid,''), COALESCE(wm.content_text,''), COALESCE(wm.work_employee_id,0), COALESCE(wm.to_user_type,0),
 COALESCE(NULLIF(wm.to_user_id,0), NULLIF(wm.room_id,0), 0), COALESCE(wm.sender_type,0), COALESCE(wm.seq,0), wm.msg_data_time,
 COALESCE(sender.name,''), COALESCE(sender.avatar,''), COALESCE(target_employee.name,target_contact.name,target_room.name,''), COALESCE(target_employee.avatar,target_contact.avatar,'')
 FROM ` + table + ` wm
 LEFT JOIN mc_work_employee sender ON sender.id=wm.work_employee_id AND sender.corp_id=wm.corp_id AND sender.deleted_at IS NULL
 LEFT JOIN mc_work_employee target_employee ON wm.to_user_type=0 AND target_employee.id=wm.to_user_id AND target_employee.corp_id=wm.corp_id AND target_employee.deleted_at IS NULL
 LEFT JOIN mc_work_contact target_contact ON wm.to_user_type=1 AND target_contact.id=wm.to_user_id AND target_contact.corp_id=wm.corp_id AND target_contact.deleted_at IS NULL
 LEFT JOIN mc_work_room target_room ON wm.to_user_type=2 AND target_room.id=COALESCE(NULLIF(wm.to_user_id,0), NULLIF(wm.room_id,0)) AND target_room.corp_id=wm.corp_id AND target_room.deleted_at IS NULL
 WHERE ` + strings.Join(where, " AND ") + ` ORDER BY wm.msg_data_time DESC, wm.seq DESC, wm.id DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]archiveMessageRow, 0)
	for rows.Next() {
		var row archiveMessageRow
		var msgTime sql.NullTime
		if err := rows.Scan(&row.ID, &row.MsgID, &row.Content, &row.EmployeeID, &row.TargetType, &row.TargetID, &row.SenderType, &row.Sequence, &msgTime, &row.EmployeeName, &row.EmployeeAvatar, &row.TargetName, &row.TargetAvatar); err != nil {
			return nil, err
		}
		if !msgTime.Valid {
			continue
		}
		row.TableIndex = tableIndex
		row.MessageTime = msgTime.Time
		if row.TargetID == "" || row.TargetID == "0" {
			row.TargetID = "0"
		}
		row.ConversationKey = fmt.Sprintf("%d:%d:%s", row.EmployeeID, row.TargetType, row.TargetID)
		if row.TargetName == "" {
			row.TargetName = row.TargetID
		}
		if row.EmployeeName == "" {
			row.EmployeeName = fmt.Sprintf("员工 %d", row.EmployeeID)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *SQLRepository) LatestSucceededFingerprint(ctx context.Context, tenantID, corpID int64, analysisType AnalysisType, ruleVersionID int64, conversationKey string) (string, error) {
	if r == nil || r.db == nil {
		return "", errors.New("AI insight repository database is unavailable")
	}
	var fingerprint string
	err := r.db.QueryRowContext(ctx, `SELECT source_fingerprint FROM mochat_go_ai_conversation_insights WHERE tenant_id=? AND corp_id=? AND analysis_type=? AND rule_version_id=? AND conversation_key=? AND status='succeeded' ORDER BY generated_at DESC, id DESC LIMIT 1`, tenantID, corpID, analysisType, ruleVersionID, conversationKey).Scan(&fingerprint)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return fingerprint, err
}

func (r *SQLRepository) SaveInsight(ctx context.Context, insight ConversationInsight) error {
	if r == nil || r.db == nil {
		return errors.New("AI insight repository database is unavailable")
	}
	resultJSON := insight.ResultJSON
	if len(resultJSON) == 0 {
		resultJSON = []byte(`{}`)
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO mochat_go_ai_conversation_insights
 (tenant_id,corp_id,analysis_type,rule_id,rule_version_id,conversation_key,employee_id,employee_name,employee_avatar,target_type,target_id,target_name,target_avatar,source_started_at,source_ended_at,source_message_count,source_fingerprint,status,summary,result_json,error_summary,provider,model,prompt_version,generated_at,created_at,updated_at)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,NOW(),NOW())
 ON DUPLICATE KEY UPDATE status=VALUES(status),summary=VALUES(summary),result_json=VALUES(result_json),error_summary=VALUES(error_summary),provider=VALUES(provider),model=VALUES(model),prompt_version=VALUES(prompt_version),generated_at=VALUES(generated_at),updated_at=NOW()`,
		insight.TenantID, insight.CorpID, insight.AnalysisType, insight.RuleID, insight.RuleVersionID, insight.ConversationKey, insight.EmployeeID, insight.EmployeeName, insight.EmployeeAvatar, insight.TargetType, insight.TargetID, insight.TargetName, insight.TargetAvatar, nullTime(insight.SourceStartedAt), nullTime(insight.SourceEndedAt), insight.SourceMessageCount, insight.SourceFingerprint, insight.Status, insight.Summary, string(resultJSON), insight.ErrorSummary, insight.Provider, insight.Model, insight.PromptVersion, nullTimePtr(insight.GeneratedAt))
	return err
}

func (r *SQLRepository) CreateRun(ctx context.Context, run InsightRun) (int64, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO mochat_go_ai_insight_runs (tenant_id,corp_id,analysis_type,rule_version_id,status,planned_at,started_at,candidate_count,success_count,failure_count,backlog_count,error_summary,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,NOW(),NOW())`, run.TenantID, run.CorpID, run.AnalysisType, run.RuleVersionID, run.Status, nullTimePtr(run.PlannedAt), nullTimePtr(run.StartedAt), run.CandidateCount, run.SuccessCount, run.FailureCount, run.BacklogCount, run.ErrorSummary)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *SQLRepository) FinishRun(ctx context.Context, id int64, result InsightRunResult) error {
	_, err := r.db.ExecContext(ctx, `UPDATE mochat_go_ai_insight_runs SET status=?,candidate_count=?,success_count=?,failure_count=?,backlog_count=?,error_summary=?,finished_at=?,updated_at=NOW() WHERE id=?`, result.Status, result.CandidateCount, result.SuccessCount, result.FailureCount, result.BacklogCount, result.ErrorSummary, result.FinishedAt, id)
	return err
}

func (r *SQLRepository) InsightPage(ctx context.Context, filter InsightFilter) (InsightPage, error) {
	if r == nil || r.db == nil {
		return InsightPage{}, errors.New("AI insight repository database is unavailable")
	}
	where, args := insightWhere(filter)
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_ai_conversation_insights i WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return InsightPage{}, err
	}
	page, size := normalizeInsightPage(filter)
	offset := (page - 1) * size
	query := `SELECT i.id,i.tenant_id,i.corp_id,i.analysis_type,i.rule_id,i.rule_version_id,COALESCE(rule.name,''),COALESCE(version.version,0),i.conversation_key,i.employee_id,i.employee_name,i.employee_avatar,i.target_type,i.target_id,i.target_name,i.target_avatar,i.source_started_at,i.source_ended_at,i.source_message_count,i.source_fingerprint,i.status,i.summary,i.result_json,i.error_summary,i.provider,i.model,i.prompt_version,i.generated_at,i.created_at FROM mochat_go_ai_conversation_insights i LEFT JOIN mochat_go_ai_analysis_rules rule ON rule.id=i.rule_id AND rule.tenant_id=i.tenant_id AND rule.corp_id=i.corp_id LEFT JOIN mochat_go_ai_analysis_rule_versions version ON version.id=i.rule_version_id AND version.tenant_id=i.tenant_id AND version.corp_id=i.corp_id WHERE ` + whereSQL + ` ORDER BY i.generated_at DESC, i.id DESC LIMIT ? OFFSET ?`
	listArgs := append(append([]any(nil), args...), size, offset)
	rows, err := r.db.QueryContext(ctx, query, listArgs...)
	if err != nil {
		return InsightPage{}, err
	}
	defer rows.Close()
	items := make([]ConversationInsight, 0)
	for rows.Next() {
		item, err := scanInsight(rows)
		if err != nil {
			return InsightPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return InsightPage{}, err
	}
	return InsightPage{Items: items, Page: page, PageSize: size, Total: total}, nil
}

func (r *SQLRepository) EmployeeOptions(ctx context.Context, filter EmployeeOptionFilter) ([]EmployeeOption, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("AI insight repository database is unavailable")
	}
	where := []string{"i.tenant_id=?", "i.corp_id=?", "i.analysis_type=?"}
	args := []any{filter.TenantID, filter.CorpID, filter.AnalysisType}
	if filter.Restricted {
		ids := uniqueInt64s(filter.AllowedEmployeeIDs)
		if len(ids) == 0 {
			return []EmployeeOption{}, nil
		}
		where = append(where, "i.employee_id IN ("+placeholders(len(ids))+")")
		for _, id := range ids {
			args = append(args, id)
		}
	}
	if keyword := strings.TrimSpace(filter.EmployeeKeyword); keyword != "" {
		where = append(where, "COALESCE(NULLIF(e.name,''),i.employee_name) LIKE ? ESCAPE '\\\\'")
		args = append(args, escapedLike(keyword))
	}
	limit := filter.Limit
	if limit < 1 || limit > 100 {
		limit = 50
	}
	args = append(args, limit)
	query := `SELECT i.employee_id,COALESCE(NULLIF(MAX(e.name),''),NULLIF(MAX(i.employee_name),''),'') AS employee_name,COALESCE(NULLIF(MAX(e.avatar),''),NULLIF(MAX(i.employee_avatar),''),'') AS employee_avatar FROM mochat_go_ai_conversation_insights i LEFT JOIN mc_work_employee e ON e.id=i.employee_id AND e.corp_id=i.corp_id AND e.deleted_at IS NULL WHERE ` + strings.Join(where, " AND ") + ` GROUP BY i.employee_id ORDER BY employee_name ASC, i.employee_id ASC LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	options := make([]EmployeeOption, 0)
	for rows.Next() {
		var option EmployeeOption
		if err := rows.Scan(&option.ID, &option.Name, &option.Avatar); err != nil {
			return nil, err
		}
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return options, nil
}

func (r *SQLRepository) InsightDetail(ctx context.Context, filter InsightDetailFilter) (ConversationInsight, error) {
	where := []string{"i.tenant_id=?", "i.corp_id=?", "i.analysis_type=?", "i.id=?"}
	args := []any{filter.TenantID, filter.CorpID, filter.AnalysisType, filter.ID}
	if filter.Restricted {
		ids := uniqueInt64s(filter.AllowedEmployeeIDs)
		if len(ids) == 0 {
			return ConversationInsight{}, sql.ErrNoRows
		}
		where = append(where, "i.employee_id IN ("+placeholders(len(ids))+")")
		for _, id := range ids {
			args = append(args, id)
		}
	}
	query := `SELECT i.id,i.tenant_id,i.corp_id,i.analysis_type,i.rule_id,i.rule_version_id,COALESCE(rule.name,''),COALESCE(version.version,0),i.conversation_key,i.employee_id,i.employee_name,i.employee_avatar,i.target_type,i.target_id,i.target_name,i.target_avatar,i.source_started_at,i.source_ended_at,i.source_message_count,i.source_fingerprint,i.status,i.summary,i.result_json,i.error_summary,i.provider,i.model,i.prompt_version,i.generated_at,i.created_at FROM mochat_go_ai_conversation_insights i LEFT JOIN mochat_go_ai_analysis_rules rule ON rule.id=i.rule_id AND rule.tenant_id=i.tenant_id AND rule.corp_id=i.corp_id LEFT JOIN mochat_go_ai_analysis_rule_versions version ON version.id=i.rule_version_id AND version.tenant_id=i.tenant_id AND version.corp_id=i.corp_id WHERE ` + strings.Join(where, " AND ")
	var row ConversationInsight
	var err error
	row, err = scanInsight(r.db.QueryRowContext(ctx, query, args...))
	return row, err
}

func (r *SQLRepository) LatestRun(ctx context.Context, tenantID, corpID int64, analysisType AnalysisType) (*InsightRun, error) {
	var run InsightRun
	var planned, started, finished sql.NullTime
	err := r.db.QueryRowContext(ctx, `SELECT id,tenant_id,corp_id,analysis_type,rule_version_id,status,planned_at,started_at,finished_at,candidate_count,success_count,failure_count,backlog_count,error_summary,created_at FROM mochat_go_ai_insight_runs WHERE tenant_id=? AND corp_id=? AND analysis_type=? ORDER BY created_at DESC,id DESC LIMIT 1`, tenantID, corpID, analysisType).Scan(&run.ID, &run.TenantID, &run.CorpID, &run.AnalysisType, &run.RuleVersionID, &run.Status, &planned, &started, &finished, &run.CandidateCount, &run.SuccessCount, &run.FailureCount, &run.BacklogCount, &run.ErrorSummary, &run.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	run.PlannedAt, run.StartedAt, run.FinishedAt = nullTimePtrValue(planned), nullTimePtrValue(started), nullTimePtrValue(finished)
	return &run, err
}

func (r *SQLRepository) RulePage(ctx context.Context, filter RuleFilter) (RulePage, error) {
	where := []string{"tenant_id=?", "corp_id=?", "deleted_at IS NULL"}
	args := []any{filter.TenantID, filter.CorpID}
	if filter.Status != "" {
		where = append(where, "status=?")
		args = append(args, filter.Status)
	}
	if filter.Keyword != "" {
		where = append(where, "(name LIKE ? OR objective LIKE ?)")
		like := "%" + filter.Keyword + "%"
		args = append(args, like, like)
	}
	query := `SELECT id,tenant_id,corp_id,name,objective,conversation_types_json,target_scope,target_ids_json,lookback_days,minimum_messages,status,current_version,created_at,updated_at,deleted_at FROM mochat_go_ai_analysis_rules WHERE ` + strings.Join(where, " AND ") + ` ORDER BY updated_at DESC,id DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return RulePage{}, err
	}
	defer rows.Close()
	items := []AnalysisRule{}
	for rows.Next() {
		item, err := scanRule(rows)
		if err != nil {
			return RulePage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return RulePage{}, err
	}
	page, size := normalizePage(filter.Page, filter.PageSize)
	total := len(items)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	return RulePage{Items: items[start:end], Page: page, PageSize: size, Total: total}, nil
}

func (r *SQLRepository) RuleByID(ctx context.Context, tenantID, corpID, id int64) (AnalysisRule, error) {
	var rule AnalysisRule
	row := r.db.QueryRowContext(ctx, `SELECT id,tenant_id,corp_id,name,objective,conversation_types_json,target_scope,target_ids_json,lookback_days,minimum_messages,status,current_version,created_at,updated_at,deleted_at FROM mochat_go_ai_analysis_rules WHERE tenant_id=? AND corp_id=? AND id=? AND deleted_at IS NULL`, tenantID, corpID, id)
	return rule, scanRuleInto(row, &rule)
}

func (r *SQLRepository) CreateRule(ctx context.Context, write RuleWrite) (AnalysisRule, error) {
	if err := validateRuleWrite(write); err != nil {
		return AnalysisRule{}, err
	}
	types, _ := json.Marshal(write.ConversationTypes)
	ids, _ := json.Marshal(write.TargetIDs)
	result, err := r.db.ExecContext(ctx, `INSERT INTO mochat_go_ai_analysis_rules (tenant_id,corp_id,name,objective,conversation_types_json,target_scope,target_ids_json,lookback_days,minimum_messages,status,current_version,created_by,updated_by,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,1,?,?,NOW(),NOW())`, write.TenantID, write.CorpID, write.Name, write.Objective, string(types), write.TargetScope, string(ids), write.LookbackDays, write.MinimumMessages, write.Status, write.ActorID, write.ActorID)
	if err != nil {
		return AnalysisRule{}, err
	}
	id, _ := result.LastInsertId()
	_, err = r.db.ExecContext(ctx, `INSERT INTO mochat_go_ai_analysis_rule_versions (tenant_id,corp_id,rule_id,version,objective,conversation_types_json,target_scope,target_ids_json,lookback_days,minimum_messages,created_by,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,NOW())`, write.TenantID, write.CorpID, id, 1, write.Objective, string(types), write.TargetScope, string(ids), write.LookbackDays, write.MinimumMessages, write.ActorID)
	if err != nil {
		return AnalysisRule{}, err
	}
	return r.RuleByID(ctx, write.TenantID, write.CorpID, id)
}

func (r *SQLRepository) UpdateRule(ctx context.Context, write RuleWrite) (AnalysisRule, error) {
	if write.ID <= 0 {
		return AnalysisRule{}, errors.New("rule id is required")
	}
	if err := validateRuleWrite(write); err != nil {
		return AnalysisRule{}, err
	}
	current, err := r.RuleByID(ctx, write.TenantID, write.CorpID, write.ID)
	if err != nil {
		return AnalysisRule{}, err
	}
	types, _ := json.Marshal(write.ConversationTypes)
	ids, _ := json.Marshal(write.TargetIDs)
	next := current.CurrentVersion + 1
	_, err = r.db.ExecContext(ctx, `UPDATE mochat_go_ai_analysis_rules SET name=?,objective=?,conversation_types_json=?,target_scope=?,target_ids_json=?,lookback_days=?,minimum_messages=?,status=?,current_version=?,updated_by=?,updated_at=NOW() WHERE tenant_id=? AND corp_id=? AND id=? AND deleted_at IS NULL`, write.Name, write.Objective, string(types), write.TargetScope, string(ids), write.LookbackDays, write.MinimumMessages, write.Status, next, write.ActorID, write.TenantID, write.CorpID, write.ID)
	if err != nil {
		return AnalysisRule{}, err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO mochat_go_ai_analysis_rule_versions (tenant_id,corp_id,rule_id,version,objective,conversation_types_json,target_scope,target_ids_json,lookback_days,minimum_messages,created_by,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,NOW())`, write.TenantID, write.CorpID, write.ID, next, write.Objective, string(types), write.TargetScope, string(ids), write.LookbackDays, write.MinimumMessages, write.ActorID)
	if err != nil {
		return AnalysisRule{}, err
	}
	return r.RuleByID(ctx, write.TenantID, write.CorpID, write.ID)
}

func (r *SQLRepository) SetRuleStatus(ctx context.Context, write RuleStatusWrite) error {
	_, err := r.db.ExecContext(ctx, `UPDATE mochat_go_ai_analysis_rules SET status=?,updated_by=?,updated_at=NOW() WHERE tenant_id=? AND corp_id=? AND id=? AND deleted_at IS NULL`, write.Status, write.ActorID, write.TenantID, write.CorpID, write.ID)
	return err
}
func (r *SQLRepository) DeleteRule(ctx context.Context, write RuleDelete) error {
	_, err := r.db.ExecContext(ctx, `UPDATE mochat_go_ai_analysis_rules SET status='disabled',deleted_by=?,deleted_at=NOW(),updated_at=NOW() WHERE tenant_id=? AND corp_id=? AND id=? AND deleted_at IS NULL`, write.ActorID, write.TenantID, write.CorpID, write.ID)
	return err
}
func (r *SQLRepository) EnabledRuleVersions(ctx context.Context, tenantID, corpID int64) ([]AnalysisRuleVersion, error) {
	query, args := enabledRuleVersionsQuery(tenantID, corpID)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []AnalysisRuleVersion{}
	for rows.Next() {
		var v AnalysisRuleVersion
		var types, ids []byte
		if err := rows.Scan(&v.ID, &v.TenantID, &v.CorpID, &v.RuleID, &v.Version, &v.Name, &v.Objective, &v.CustomerAnalysisPrompt, &v.EmployeeQAPrompt, &types, &v.TargetScope, &ids, &v.LookbackDays, &v.MinimumMessages, &v.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(types, &v.ConversationTypes)
		_ = json.Unmarshal(ids, &v.TargetIDs)
		result = append(result, v)
	}
	return result, rows.Err()
}

func enabledRuleVersionsQuery(tenantID, corpID int64) (string, []any) {
	return `SELECT v.id,v.tenant_id,v.corp_id,v.rule_id,v.version,r.name,v.objective,COALESCE(v.customer_analysis_prompt,''),COALESCE(v.employee_qa_prompt,''),v.conversation_types_json,v.target_scope,v.target_ids_json,v.lookback_days,v.minimum_messages,v.created_at
        FROM mochat_go_ai_analysis_rule_versions v
        JOIN mochat_go_ai_analysis_rules r ON r.id=v.rule_id AND r.tenant_id=v.tenant_id AND r.corp_id=v.corp_id
          AND r.status='enabled' AND r.deleted_at IS NULL AND v.version=r.current_version
        WHERE v.tenant_id=? AND v.corp_id=? AND r.system_key=?
        ORDER BY v.rule_id`, []any{tenantID, corpID, DefaultSmartAnalysisRuleSystemKey}
}

func (r *SQLRepository) CurrentEnabledRuleVersion(ctx context.Context, tenantID, corpID int64, systemKey string) (*AnalysisRuleVersion, error) {
	query, args := currentEnabledRuleVersionQuery(tenantID, corpID, systemKey)
	var version AnalysisRuleVersion
	var types, ids []byte
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&version.ID, &version.TenantID, &version.CorpID, &version.RuleID, &version.Version, &version.Name,
		&version.Objective, &version.CustomerAnalysisPrompt, &version.EmployeeQAPrompt, &types, &version.TargetScope,
		&ids, &version.LookbackDays, &version.MinimumMessages, &version.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(types, &version.ConversationTypes)
	_ = json.Unmarshal(ids, &version.TargetIDs)
	return &version, nil
}

func currentEnabledRuleVersionQuery(tenantID, corpID int64, systemKey string) (string, []any) {
	return `SELECT v.id,v.tenant_id,v.corp_id,v.rule_id,v.version,r.name,v.objective,COALESCE(v.customer_analysis_prompt,''),COALESCE(v.employee_qa_prompt,''),v.conversation_types_json,v.target_scope,v.target_ids_json,v.lookback_days,v.minimum_messages,v.created_at
        FROM mochat_go_ai_analysis_rule_versions v
        JOIN mochat_go_ai_analysis_rules r ON r.id=v.rule_id AND r.tenant_id=v.tenant_id AND r.corp_id=v.corp_id
          AND r.status='enabled' AND r.deleted_at IS NULL AND v.version=r.current_version
        WHERE v.tenant_id=? AND v.corp_id=? AND r.system_key=?
        LIMIT 1`, []any{tenantID, corpID, systemKey}
}

func (r *SQLRepository) queryInsightRow(ctx context.Context, query string, args ...any) (ConversationInsight, error) {
	return scanInsight(r.db.QueryRowContext(ctx, query, args...))
}

type rowScanner interface{ Scan(...any) error }

func scanInsight(scanner rowScanner) (ConversationInsight, error) {
	var item ConversationInsight
	var analysisType string
	var status string
	var start, end, generated, created sql.NullTime
	var result []byte
	err := scanner.Scan(&item.ID, &item.TenantID, &item.CorpID, &analysisType, &item.RuleID, &item.RuleVersionID, &item.RuleNameSnapshot, &item.RuleVersion, &item.ConversationKey, &item.EmployeeID, &item.EmployeeName, &item.EmployeeAvatar, &item.TargetType, &item.TargetID, &item.TargetName, &item.TargetAvatar, &start, &end, &item.SourceMessageCount, &item.SourceFingerprint, &status, &item.Summary, &result, &item.ErrorSummary, &item.Provider, &item.Model, &item.PromptVersion, &generated, &created)
	if err != nil {
		return item, err
	}
	item.AnalysisType = AnalysisType(analysisType)
	item.Status = AnalysisStatus(status)
	item.SourceStartedAt = start.Time
	item.SourceEndedAt = end.Time
	item.GeneratedAt = nullTimePtrValue(generated)
	item.CreatedAt = created.Time
	item.ResultJSON = append([]byte(nil), result...)
	if item.AnalysisType == AnalysisTypeSession {
		var parsed SessionAnalysisResult
		if json.Unmarshal(result, &parsed) == nil {
			item.SessionResult = &parsed
		}
	} else if item.AnalysisType == AnalysisTypeSmart {
		var parsed SmartAnalysisResult
		if json.Unmarshal(result, &parsed) == nil {
			item.SmartResult = &parsed
		}
	}
	return item, nil
}

func scanRule(scanner rowScanner) (AnalysisRule, error) {
	var rule AnalysisRule
	return rule, scanRuleInto(scanner, &rule)
}
func scanRuleInto(scanner rowScanner, rule *AnalysisRule) error {
	var types, ids []byte
	var deleted sql.NullTime
	err := scanner.Scan(&rule.ID, &rule.TenantID, &rule.CorpID, &rule.Name, &rule.Objective, &types, &rule.TargetScope, &ids, &rule.LookbackDays, &rule.MinimumMessages, &rule.Status, &rule.CurrentVersion, &rule.CreatedAt, &rule.UpdatedAt, &deleted)
	if err != nil {
		return err
	}
	_ = json.Unmarshal(types, &rule.ConversationTypes)
	_ = json.Unmarshal(ids, &rule.TargetIDs)
	rule.DeletedAt = nullTimePtrValue(deleted)
	return nil
}

func insightWhere(filter InsightFilter) ([]string, []any) {
	where := []string{"i.tenant_id=?", "i.corp_id=?", "i.analysis_type=?"}
	args := []any{filter.TenantID, filter.CorpID, filter.AnalysisType}
	if filter.EmployeeID > 0 {
		where = append(where, "i.employee_id=?")
		args = append(args, filter.EmployeeID)
	}
	if filter.ConversationType != "" {
		where = append(where, "i.target_type=?")
		args = append(args, filter.ConversationType)
	}
	if filter.TargetID != "" {
		where = append(where, "i.target_id=?")
		args = append(args, filter.TargetID)
	}
	if filter.RuleVersionID > 0 {
		where = append(where, "i.rule_version_id=?")
		args = append(args, filter.RuleVersionID)
	}
	if filter.Status != "" {
		where = append(where, "i.status=?")
		args = append(args, filter.Status)
	}
	if filter.StartAt != nil {
		where = append(where, "i.source_ended_at>=?")
		args = append(args, *filter.StartAt)
	}
	if filter.EndAt != nil {
		where = append(where, "i.source_started_at<?")
		args = append(args, *filter.EndAt)
	}
	switch filter.View {
	case "emotion":
		if emotion := strings.TrimSpace(filter.Emotion); emotion != "" {
			where = append(where, "JSON_UNQUOTE(JSON_EXTRACT(i.result_json,'$.customer.emotion.label'))=?")
			args = append(args, emotion)
		}
	case "employee-score":
		if filter.MinScore != nil || filter.MaxScore != nil {
			where = append(where, "JSON_TYPE(JSON_EXTRACT(i.result_json,'$.employeeQa.score'))='INTEGER'")
		}
		if filter.MinScore != nil {
			where = append(where, "CAST(JSON_UNQUOTE(JSON_EXTRACT(i.result_json,'$.employeeQa.score')) AS SIGNED)>=?")
			args = append(args, *filter.MinScore)
		}
		if filter.MaxScore != nil {
			where = append(where, "CAST(JSON_UNQUOTE(JSON_EXTRACT(i.result_json,'$.employeeQa.score')) AS SIGNED)<=?")
			args = append(args, *filter.MaxScore)
		}
	case "communication-keyword":
		if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
			where = append(where, "JSON_SEARCH(JSON_EXTRACT(i.result_json,'$.customer.keywords'),'one',?,'\\\\','$[*]') IS NOT NULL")
			args = append(args, escapedLike(keyword))
		}
	default:
		if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
			value := escapedLike(keyword)
			where = append(where, "(i.summary LIKE ? ESCAPE '\\\\' OR i.target_name LIKE ? ESCAPE '\\\\' OR i.employee_name LIKE ? ESCAPE '\\\\')")
			args = append(args, value, value, value)
		}
	}
	if customerName := strings.TrimSpace(filter.CustomerName); customerName != "" {
		where = append(where, "(i.target_type='1' AND i.target_name LIKE ? ESCAPE '\\\\')")
		args = append(args, escapedLike(customerName))
	}
	if filter.Restricted {
		ids := uniqueInt64s(filter.AllowedEmployeeIDs)
		if len(ids) == 0 {
			return append(where, "1=0"), args
		}
		where = append(where, "i.employee_id IN ("+placeholders(len(ids))+")")
		for _, id := range ids {
			args = append(args, id)
		}
	}
	return where, args
}

func escapedLike(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
	return "%" + escaped + "%"
}

func fingerprintArchiveRows(rows []archiveMessageRow) string {
	h := sha256.New()
	for _, row := range rows {
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00%d\x00%d\n", row.ID, row.MsgID, row.MessageTime.UTC().Format(time.RFC3339Nano), row.Sequence, row.TableIndex)
		h.Write([]byte(row.Content))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
func sortArchiveRows(rows []archiveMessageRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].MessageTime.Equal(rows[j].MessageTime) {
			if rows[i].Sequence == rows[j].Sequence {
				return rows[i].TableIndex > rows[j].TableIndex
			}
			return rows[i].Sequence > rows[j].Sequence
		}
		return rows[i].MessageTime.After(rows[j].MessageTime)
	})
}
func latestArchiveRow(rows []archiveMessageRow) archiveMessageRow {
	if len(rows) == 0 {
		return archiveMessageRow{}
	}
	latest := rows[0]
	for _, row := range rows[1:] {
		if row.MessageTime.After(latest.MessageTime) {
			latest = row
		}
	}
	return latest
}
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
func uniqueInt64s(values []int64) []int64 {
	seen := map[int64]struct{}{}
	result := []int64{}
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
func normalizePage(page, size int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	return page, size
}

func normalizeInsightPage(filter InsightFilter) (int, int) {
	if filter.Export {
		return 1, 10000
	}
	return normalizePage(filter.Page, filter.PageSize)
}
func validateRuleWrite(write RuleWrite) error {
	if strings.TrimSpace(write.Name) == "" || len([]rune(write.Name)) > 80 {
		return errors.New("rule name must be 1-80 characters")
	}
	if strings.TrimSpace(write.Objective) == "" || len([]rune(write.Objective)) > 500 {
		return errors.New("rule objective must be 1-500 characters")
	}
	if len(write.ConversationTypes) == 0 {
		return errors.New("at least one conversation type is required")
	}
	if write.TargetScope != "all" && write.TargetScope != "department" && write.TargetScope != "employee" {
		return errors.New("invalid target scope")
	}
	if write.LookbackDays < 1 || write.LookbackDays > 30 {
		return errors.New("lookback days must be between 1 and 30")
	}
	if write.MinimumMessages < 2 || write.MinimumMessages > 50 {
		return errors.New("minimum messages must be between 2 and 50")
	}
	if write.Status != "enabled" && write.Status != "disabled" {
		return errors.New("invalid rule status")
	}
	return nil
}
func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
func nullTimePtr(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}
func nullTimePtrValue(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	v := value.Time
	return &v
}
