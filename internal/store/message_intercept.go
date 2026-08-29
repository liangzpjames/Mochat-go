package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"jiyi/mochat-go/internal/dashboard"
	"strings"
	"time"
)

func normalizeInterceptPage(p, n int) (int, int) {
	if p < 1 {
		p = 1
	}
	if n < 1 || n > 100 {
		n = 20
	}
	return p, n
}
func interceptScope(t, c int) (string, []any) {
	if t > 0 {
		return " WHERE tenant_id=? AND corp_id=?", []any{t, c}
	}
	return " WHERE corp_id=?", []any{c}
}

func (s *MySQLStore) KeywordLibraryPage(ctx context.Context, f dashboard.KeywordLibraryFilter) (dashboard.KeywordLibraryPage, error) {
	f.Page, f.PerPage = normalizeInterceptPage(f.Page, f.PerPage)
	w, a := interceptScope(f.TenantID, f.CorpID)
	if strings.TrimSpace(f.Name) != "" {
		w += " AND l.name LIKE ?"
		a = append(a, "%"+strings.TrimSpace(f.Name)+"%")
	}
	if f.Status != "" {
		w += " AND l.status=?"
		a = append(a, f.Status)
	}
	w = strings.Replace(w, "tenant_id", "l.tenant_id", 1)
	w = strings.Replace(w, "corp_id", "l.corp_id", 1)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_keyword_libraries l"+w, a...).Scan(&total); err != nil {
		return dashboard.KeywordLibraryPage{}, err
	}
	q := `SELECT l.id,l.tenant_id,l.corp_id,l.name,l.description,l.match_mode,l.status,l.draft_version,l.published_version,COUNT(e.id),l.created_at,l.updated_at FROM mochat_go_keyword_libraries l LEFT JOIN mochat_go_keyword_entries e ON e.tenant_id=l.tenant_id AND e.corp_id=l.corp_id AND e.library_id=l.id` + w + ` GROUP BY l.id ORDER BY l.updated_at DESC,l.id DESC LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, q, append(a, f.PerPage, (f.Page-1)*f.PerPage)...)
	if err != nil {
		return dashboard.KeywordLibraryPage{}, err
	}
	defer rows.Close()
	items := []dashboard.KeywordLibrary{}
	for rows.Next() {
		var v dashboard.KeywordLibrary
		var c, u time.Time
		if err = rows.Scan(&v.ID, &v.TenantID, &v.CorpID, &v.Name, &v.Description, &v.MatchMode, &v.Status, &v.DraftVersion, &v.PublishedVersion, &v.EntryCount, &c, &u); err != nil {
			return dashboard.KeywordLibraryPage{}, err
		}
		v.CreatedAt = c.Format(time.RFC3339)
		v.UpdatedAt = u.Format(time.RFC3339)
		items = append(items, v)
	}
	return dashboard.KeywordLibraryPage{Items: items, Total: total, Page: f.Page, PerPage: f.PerPage}, rows.Err()
}
func (s *MySQLStore) KeywordEntryPage(ctx context.Context, f dashboard.KeywordEntryFilter) (dashboard.KeywordEntryPage, error) {
	f.Page, f.PerPage = normalizeInterceptPage(f.Page, f.PerPage)
	w, a := interceptScope(f.TenantID, f.CorpID)
	w += " AND library_id=?"
	a = append(a, f.LibraryID)
	if f.Keyword != "" {
		w += " AND keyword LIKE ?"
		a = append(a, "%"+strings.TrimSpace(f.Keyword)+"%")
	}
	if f.Status != "" {
		w += " AND status=?"
		a = append(a, f.Status)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_keyword_entries"+w, a...).Scan(&total); err != nil {
		return dashboard.KeywordEntryPage{}, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id,library_id,keyword,status,created_at,updated_at FROM mochat_go_keyword_entries"+w+" ORDER BY id DESC LIMIT ? OFFSET ?", append(a, f.PerPage, (f.Page-1)*f.PerPage)...)
	if err != nil {
		return dashboard.KeywordEntryPage{}, err
	}
	defer rows.Close()
	items := []dashboard.KeywordEntry{}
	for rows.Next() {
		var v dashboard.KeywordEntry
		var c, u time.Time
		if err = rows.Scan(&v.ID, &v.LibraryID, &v.Keyword, &v.Status, &c, &u); err != nil {
			return dashboard.KeywordEntryPage{}, err
		}
		v.CreatedAt = c.Format(time.RFC3339)
		v.UpdatedAt = u.Format(time.RFC3339)
		items = append(items, v)
	}
	return dashboard.KeywordEntryPage{Items: items, Total: total, Page: f.Page, PerPage: f.PerPage}, rows.Err()
}
func (s *MySQLStore) SaveKeywordLibrary(ctx context.Context, v dashboard.KeywordLibrary) (int64, error) {
	if err := dashboard.ValidateKeywordLibrary(v); err != nil {
		return 0, err
	}
	if v.ID > 0 {
		r, e := s.db.ExecContext(ctx, `UPDATE mochat_go_keyword_libraries SET name=?,description=?,match_mode=?,status=?,draft_version=draft_version+1 WHERE id=? AND tenant_id=? AND corp_id=?`, strings.TrimSpace(v.Name), strings.TrimSpace(v.Description), v.MatchMode, v.Status, v.ID, v.TenantID, v.CorpID)
		if e != nil {
			return 0, e
		}
		n, _ := r.RowsAffected()
		if n == 0 {
			return 0, sql.ErrNoRows
		}
		return v.ID, nil
	}
	r, e := s.db.ExecContext(ctx, `INSERT INTO mochat_go_keyword_libraries(tenant_id,corp_id,name,description,match_mode,status) VALUES(?,?,?,?,?,?)`, v.TenantID, v.CorpID, strings.TrimSpace(v.Name), strings.TrimSpace(v.Description), v.MatchMode, v.Status)
	if e != nil {
		return 0, e
	}
	return r.LastInsertId()
}
func (s *MySQLStore) SetKeywordLibraryStatus(ctx context.Context, t, c int, id int64, status string) (bool, error) {
	if status != "enabled" && status != "disabled" {
		return false, fmt.Errorf("词库状态无效")
	}
	r, e := s.db.ExecContext(ctx, "UPDATE mochat_go_keyword_libraries SET status=? WHERE id=? AND tenant_id=? AND corp_id=?", status, id, t, c)
	if e != nil {
		return false, e
	}
	n, _ := r.RowsAffected()
	return n > 0, nil
}
func (s *MySQLStore) DeleteKeywordLibrary(ctx context.Context, t, c int, id int64) (bool, error) {
	var n int
	if e := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_message_intercept_rules WHERE tenant_id=? AND corp_id=? AND library_id=?", t, c, id).Scan(&n); e != nil {
		return false, e
	}
	if n > 0 {
		return false, fmt.Errorf("词库已被拦截规则引用，不能删除")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	for _, q := range []string{"DELETE FROM mochat_go_keyword_version_entries WHERE library_id=? AND tenant_id=? AND corp_id=?", "DELETE FROM mochat_go_keyword_versions WHERE library_id=? AND tenant_id=? AND corp_id=?", "DELETE FROM mochat_go_keyword_entries WHERE library_id=? AND tenant_id=? AND corp_id=?"} {
		if _, e = tx.ExecContext(ctx, q, id, t, c); e != nil {
			return false, e
		}
	}
	r, e := tx.ExecContext(ctx, "DELETE FROM mochat_go_keyword_libraries WHERE id=? AND tenant_id=? AND corp_id=?", id, t, c)
	if e != nil {
		return false, e
	}
	n64, _ := r.RowsAffected()
	if e = tx.Commit(); e != nil {
		return false, e
	}
	return n64 > 0, nil
}
func (s *MySQLStore) SaveKeywordEntry(ctx context.Context, t, c int, v dashboard.KeywordEntry) (int64, error) {
	if err := dashboard.ValidateKeywordEntry(v); err != nil {
		return 0, err
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	var lockedLibraryID int64
	if e = tx.QueryRowContext(ctx, `SELECT id FROM mochat_go_keyword_libraries WHERE id=? AND tenant_id=? AND corp_id=? FOR UPDATE`, v.LibraryID, t, c).Scan(&lockedLibraryID); e != nil {
		return 0, e
	}
	var id int64
	if v.ID > 0 {
		r, e := tx.ExecContext(ctx, `UPDATE mochat_go_keyword_entries SET keyword=?,status=? WHERE id=? AND tenant_id=? AND corp_id=? AND library_id=?`, strings.TrimSpace(v.Keyword), v.Status, v.ID, t, c, v.LibraryID)
		if e != nil {
			return 0, e
		}
		n, e := r.RowsAffected()
		if e != nil {
			return 0, e
		}
		if n == 0 {
			return 0, sql.ErrNoRows
		}
		id = v.ID
	} else {
		r, e := tx.ExecContext(ctx, `INSERT INTO mochat_go_keyword_entries(tenant_id,corp_id,library_id,keyword,status) VALUES(?,?,?,?,?)`, t, c, v.LibraryID, strings.TrimSpace(v.Keyword), v.Status)
		if e != nil {
			return 0, e
		}
		n, e := r.RowsAffected()
		if e != nil {
			return 0, e
		}
		if n != 1 {
			return 0, fmt.Errorf("关键词保存失败")
		}
		id, e = r.LastInsertId()
		if e != nil {
			return 0, e
		}
	}
	versionResult, e := tx.ExecContext(ctx, "UPDATE mochat_go_keyword_libraries SET draft_version=draft_version+1 WHERE id=? AND tenant_id=? AND corp_id=?", v.LibraryID, t, c)
	if e != nil {
		return 0, e
	}
	versionRows, e := versionResult.RowsAffected()
	if e != nil {
		return 0, e
	}
	if versionRows != 1 {
		return 0, fmt.Errorf("关键词库版本更新失败")
	}
	if e = tx.Commit(); e != nil {
		return 0, e
	}
	return id, nil
}
func (s *MySQLStore) SetKeywordEntryStatus(ctx context.Context, t, c int, id int64, status string) (bool, error) {
	if status != "enabled" && status != "disabled" {
		return false, fmt.Errorf("关键词状态无效")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	libraryID, found, e := lockKeywordEntryForMutation(ctx, tx, t, c, id)
	if e != nil || !found {
		return false, e
	}
	r, e := tx.ExecContext(ctx, "UPDATE mochat_go_keyword_entries SET status=? WHERE id=? AND tenant_id=? AND corp_id=? AND library_id=?", status, id, t, c, libraryID)
	if e != nil {
		return false, e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return false, e
	}
	if n == 0 {
		return false, nil
	}
	if n != 1 {
		return false, fmt.Errorf("关键词状态更新影响了意外的行数")
	}
	if e := incrementKeywordLibraryDraftVersion(ctx, tx, t, c, libraryID); e != nil {
		return false, e
	}
	if e := tx.Commit(); e != nil {
		return false, e
	}
	return true, nil
}
func (s *MySQLStore) DeleteKeywordEntry(ctx context.Context, t, c int, id int64) (bool, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	libraryID, found, e := lockKeywordEntryForMutation(ctx, tx, t, c, id)
	if e != nil || !found {
		return false, e
	}
	r, e := tx.ExecContext(ctx, "DELETE FROM mochat_go_keyword_entries WHERE id=? AND tenant_id=? AND corp_id=? AND library_id=?", id, t, c, libraryID)
	if e != nil {
		return false, e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return false, e
	}
	if n != 1 {
		return false, fmt.Errorf("关键词删除影响了意外的行数")
	}
	if e := incrementKeywordLibraryDraftVersion(ctx, tx, t, c, libraryID); e != nil {
		return false, e
	}
	if e = tx.Commit(); e != nil {
		return false, e
	}
	return true, nil
}

func lockKeywordEntryForMutation(ctx context.Context, tx *sql.Tx, tenantID, corpID int, entryID int64) (int64, bool, error) {
	var libraryID int64
	if err := tx.QueryRowContext(ctx, "SELECT library_id FROM mochat_go_keyword_entries WHERE id=? AND tenant_id=? AND corp_id=?", entryID, tenantID, corpID).Scan(&libraryID); err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	var lockedLibraryID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM mochat_go_keyword_libraries WHERE id=? AND tenant_id=? AND corp_id=? FOR UPDATE", libraryID, tenantID, corpID).Scan(&lockedLibraryID); err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	var lockedEntryID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM mochat_go_keyword_entries WHERE id=? AND tenant_id=? AND corp_id=? AND library_id=? FOR UPDATE", entryID, tenantID, corpID, libraryID).Scan(&lockedEntryID); err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	return libraryID, true, nil
}

func incrementKeywordLibraryDraftVersion(ctx context.Context, tx *sql.Tx, tenantID, corpID int, libraryID int64) error {
	result, err := tx.ExecContext(ctx, "UPDATE mochat_go_keyword_libraries SET draft_version=draft_version+1 WHERE id=? AND tenant_id=? AND corp_id=?", libraryID, tenantID, corpID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("关键词库版本更新影响了意外的行数")
	}
	return nil
}
func (s *MySQLStore) PublishKeywordLibrary(ctx context.Context, t, c int, id, actor int64) (int, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	var version int
	if e = tx.QueryRowContext(ctx, "SELECT published_version+1 FROM mochat_go_keyword_libraries WHERE id=? AND tenant_id=? AND corp_id=? FOR UPDATE", id, t, c).Scan(&version); e != nil {
		return 0, e
	}
	var count int
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_keyword_entries WHERE tenant_id=? AND corp_id=? AND library_id=? AND status='enabled'", t, c, id).Scan(&count); e != nil {
		return 0, e
	}
	if count == 0 {
		return 0, fmt.Errorf("至少需要一个启用关键词才能发布")
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO mochat_go_keyword_versions(tenant_id,corp_id,library_id,version,entry_count,publisher_id) VALUES(?,?,?,?,?,?)", t, c, id, version, count, actor); e != nil {
		return 0, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO mochat_go_keyword_version_entries(tenant_id,corp_id,library_id,version,source_entry_id,keyword) SELECT tenant_id,corp_id,library_id,?,id,keyword FROM mochat_go_keyword_entries WHERE tenant_id=? AND corp_id=? AND library_id=? AND status='enabled'`, version, t, c, id); e != nil {
		return 0, e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE mochat_go_keyword_libraries SET published_version=? WHERE id=? AND tenant_id=? AND corp_id=?", version, id, t, c); e != nil {
		return 0, e
	}
	return version, tx.Commit()
}

func (s *MySQLStore) MessageInterceptRulePage(ctx context.Context, f dashboard.MessageInterceptRuleFilter) (dashboard.MessageInterceptRulePage, error) {
	f.Page, f.PerPage = normalizeInterceptPage(f.Page, f.PerPage)
	w, a := interceptScope(f.TenantID, f.CorpID)
	w = strings.Replace(w, "tenant_id", "r.tenant_id", 1)
	w = strings.Replace(w, "corp_id", "r.corp_id", 1)
	if f.Name != "" {
		w += " AND r.name LIKE ?"
		a = append(a, "%"+strings.TrimSpace(f.Name)+"%")
	}
	if f.Status != "" {
		w += " AND r.status=?"
		a = append(a, f.Status)
	}
	var total int
	if e := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_message_intercept_rules r"+w, a...).Scan(&total); e != nil {
		return dashboard.MessageInterceptRulePage{}, e
	}
	rows, e := s.db.QueryContext(ctx, `SELECT r.id,r.tenant_id,r.corp_id,r.name,r.library_id,l.name,r.library_version,r.conversation_scopes_json,r.decision,r.status,r.trigger_count,r.created_at,r.updated_at FROM mochat_go_message_intercept_rules r JOIN mochat_go_keyword_libraries l ON l.id=r.library_id AND l.tenant_id=r.tenant_id AND l.corp_id=r.corp_id`+w+` ORDER BY r.updated_at DESC,r.id DESC LIMIT ? OFFSET ?`, append(a, f.PerPage, (f.Page-1)*f.PerPage)...)
	if e != nil {
		return dashboard.MessageInterceptRulePage{}, e
	}
	defer rows.Close()
	items := []dashboard.MessageInterceptRule{}
	for rows.Next() {
		var v dashboard.MessageInterceptRule
		var scopes []byte
		var c, u time.Time
		if e = rows.Scan(&v.ID, &v.TenantID, &v.CorpID, &v.Name, &v.LibraryID, &v.LibraryName, &v.LibraryVersion, &scopes, &v.Decision, &v.Status, &v.TriggerCount, &c, &u); e != nil {
			return dashboard.MessageInterceptRulePage{}, e
		}
		_ = json.Unmarshal(scopes, &v.ConversationScopes)
		v.CreatedAt = c.Format(time.RFC3339)
		v.UpdatedAt = u.Format(time.RFC3339)
		items = append(items, v)
	}
	return dashboard.MessageInterceptRulePage{Items: items, Total: total, Page: f.Page, PerPage: f.PerPage}, rows.Err()
}
func (s *MySQLStore) SaveMessageInterceptRule(ctx context.Context, v dashboard.MessageInterceptRule) (int64, error) {
	if err := dashboard.ValidateMessageInterceptRule(v); err != nil {
		return 0, err
	}
	var exists int
	if e := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_keyword_versions WHERE tenant_id=? AND corp_id=? AND library_id=? AND version=?", v.TenantID, v.CorpID, v.LibraryID, v.LibraryVersion).Scan(&exists); e != nil {
		return 0, e
	}
	if exists == 0 {
		return 0, fmt.Errorf("关联的词库版本不存在")
	}
	scopes, _ := json.Marshal(v.ConversationScopes)
	if v.ID > 0 {
		r, e := s.db.ExecContext(ctx, "UPDATE mochat_go_message_intercept_rules SET name=?,library_id=?,library_version=?,conversation_scopes_json=?,decision=?,status=? WHERE id=? AND tenant_id=? AND corp_id=?", v.Name, v.LibraryID, v.LibraryVersion, scopes, v.Decision, v.Status, v.ID, v.TenantID, v.CorpID)
		if e != nil {
			return 0, e
		}
		n, _ := r.RowsAffected()
		if n == 0 {
			return 0, sql.ErrNoRows
		}
		return v.ID, nil
	}
	r, e := s.db.ExecContext(ctx, "INSERT INTO mochat_go_message_intercept_rules(tenant_id,corp_id,name,library_id,library_version,conversation_scopes_json,decision,status) VALUES(?,?,?,?,?,?,?,?)", v.TenantID, v.CorpID, v.Name, v.LibraryID, v.LibraryVersion, scopes, v.Decision, v.Status)
	if e != nil {
		return 0, e
	}
	return r.LastInsertId()
}
func (s *MySQLStore) SetMessageInterceptRuleStatus(ctx context.Context, t, c int, id int64, status string) (bool, error) {
	if status != "enabled" && status != "disabled" {
		return false, fmt.Errorf("规则状态无效")
	}
	r, e := s.db.ExecContext(ctx, "UPDATE mochat_go_message_intercept_rules SET status=? WHERE id=? AND tenant_id=? AND corp_id=?", status, id, t, c)
	if e != nil {
		return false, e
	}
	n, _ := r.RowsAffected()
	return n > 0, nil
}
func (s *MySQLStore) DeleteMessageInterceptRule(ctx context.Context, t, c int, id int64) (bool, error) {
	r, e := s.db.ExecContext(ctx, "DELETE FROM mochat_go_message_intercept_rules WHERE id=? AND tenant_id=? AND corp_id=?", id, t, c)
	if e != nil {
		return false, e
	}
	n, _ := r.RowsAffected()
	return n > 0, nil
}
func (s *MySQLStore) MessageInterceptRecordPage(ctx context.Context, f dashboard.MessageInterceptRecordFilter) (dashboard.MessageInterceptRecordPage, error) {
	f.Page, f.PerPage = normalizeInterceptPage(f.Page, f.PerPage)
	w, a := interceptScope(f.TenantID, f.CorpID)
	if f.RuleID > 0 {
		w += " AND rule_id=?"
		a = append(a, f.RuleID)
	}
	if f.Decision != "" {
		w += " AND decision=?"
		a = append(a, f.Decision)
	}
	if f.AuditStatus != "" {
		w += " AND audit_status=?"
		a = append(a, f.AuditStatus)
	}
	if f.ConversationType != "" {
		conversationType := f.ConversationType
		if conversationType == "group" {
			conversationType = "room"
		}
		w += " AND conversation_type=?"
		a = append(a, conversationType)
	}
	if f.Keyword != "" {
		w += " AND (message_content LIKE ? OR JSON_SEARCH(matched_keywords_json,'one',?) IS NOT NULL)"
		a = append(a, "%"+f.Keyword+"%", f.Keyword)
	}
	var total int
	if e := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_message_intercept_records"+w, a...).Scan(&total); e != nil {
		return dashboard.MessageInterceptRecordPage{}, e
	}
	rows, e := s.db.QueryContext(ctx, "SELECT id,rule_id,rule_name,library_id,library_version,conversation_type,conversation_id,message_id,sender_id,sender_name,message_content,matched_keywords_json,decision,explanation,audit_status,occurred_at FROM mochat_go_message_intercept_records"+w+" ORDER BY occurred_at DESC,id DESC LIMIT ? OFFSET ?", append(a, f.PerPage, (f.Page-1)*f.PerPage)...)
	if e != nil {
		return dashboard.MessageInterceptRecordPage{}, e
	}
	defer rows.Close()
	items := []dashboard.MessageInterceptRecord{}
	for rows.Next() {
		var v dashboard.MessageInterceptRecord
		var keys []byte
		var at time.Time
		if e = rows.Scan(&v.ID, &v.RuleID, &v.RuleName, &v.LibraryID, &v.LibraryVersion, &v.ConversationType, &v.ConversationID, &v.MessageID, &v.SenderID, &v.SenderName, &v.MessageContent, &keys, &v.Decision, &v.Explanation, &v.AuditStatus, &at); e != nil {
			return dashboard.MessageInterceptRecordPage{}, e
		}
		_ = json.Unmarshal(keys, &v.MatchedKeywords)
		v.OccurredAt = at.Format(time.RFC3339)
		items = append(items, v)
	}
	return dashboard.MessageInterceptRecordPage{Items: items, Total: total, Page: f.Page, PerPage: f.PerPage}, rows.Err()
}
func (s *MySQLStore) EvaluateMessageIntercept(ctx context.Context, t, c int, in dashboard.MessageInterceptEvaluation) (dashboard.MessageInterceptDecision, error) {
	if strings.TrimSpace(in.Content) == "" {
		return dashboard.MessageInterceptDecision{}, fmt.Errorf("消息内容不能为空")
	}
	rows, e := s.db.QueryContext(ctx, "SELECT id,name,library_id,library_version,conversation_scopes_json,decision FROM mochat_go_message_intercept_rules WHERE tenant_id=? AND corp_id=? AND status='enabled' ORDER BY id", t, c)
	if e != nil {
		return dashboard.MessageInterceptDecision{}, e
	}
	defer rows.Close()
	result := dashboard.MessageInterceptDecision{Decision: "allowed", Explanation: "未命中启用的拦截规则"}
	for rows.Next() {
		var id, lib int64
		var name, decision string
		var ver int
		var scopesJSON []byte
		if e = rows.Scan(&id, &name, &lib, &ver, &scopesJSON, &decision); e != nil {
			return result, e
		}
		var scopes []string
		_ = json.Unmarshal(scopesJSON, &scopes)
		scopeOK := false
		for _, v := range scopes {
			if v == in.ConversationType {
				scopeOK = true
			}
		}
		if !scopeOK {
			continue
		}
		var mode string
		if e = s.db.QueryRowContext(ctx, "SELECT match_mode FROM mochat_go_keyword_libraries WHERE id=? AND tenant_id=? AND corp_id=?", lib, t, c).Scan(&mode); e != nil {
			return result, e
		}
		kr, e := s.db.QueryContext(ctx, "SELECT keyword FROM mochat_go_keyword_version_entries WHERE tenant_id=? AND corp_id=? AND library_id=? AND version=?", t, c, lib, ver)
		if e != nil {
			return result, e
		}
		keys := []string{}
		for kr.Next() {
			var k string
			_ = kr.Scan(&k)
			keys = append(keys, k)
		}
		_ = kr.Close()
		matched := dashboard.MatchKeywords(in.Content, mode, keys)
		if len(matched) == 0 {
			continue
		}
		at := time.Now()
		if in.OccurredAt != "" {
			if parsed, pe := time.Parse(time.RFC3339, in.OccurredAt); pe == nil {
				at = parsed
			}
		}
		mk, _ := json.Marshal(matched)
		explain := fmt.Sprintf("命中规则“%s”的词库版本 v%d：%s", name, ver, strings.Join(matched, "、"))
		r, e := s.db.ExecContext(ctx, `INSERT INTO mochat_go_message_intercept_records(tenant_id,corp_id,rule_id,rule_name,library_id,library_version,conversation_type,conversation_id,message_id,sender_id,sender_name,message_content,matched_keywords_json,decision,explanation,occurred_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, t, c, id, name, lib, ver, in.ConversationType, in.ConversationID, in.MessageID, in.SenderID, in.SenderName, in.Content, mk, decision, explain, at)
		if e != nil {
			return result, e
		}
		rid, _ := r.LastInsertId()
		_, _ = s.db.ExecContext(ctx, "UPDATE mochat_go_message_intercept_rules SET trigger_count=trigger_count+1 WHERE id=? AND tenant_id=? AND corp_id=?", id, t, c)
		result.Matched = true
		result.RecordIDs = append(result.RecordIDs, rid)
		result.MatchedKeywords = append(result.MatchedKeywords, matched...)
		if result.Decision == "allowed" || decision == "blocked" {
			result.Decision = decision
		}
		result.Explanation = explain
	}
	return result, rows.Err()
}
func (s *MySQLStore) AuditMessageInterceptRecords(ctx context.Context, t, c int, actor int64, ids []int64, action, remark string) (int64, error) {
	if action != "confirmed" && action != "ignored" {
		return 0, fmt.Errorf("审核动作无效")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	var count int64
	for _, id := range ids {
		r, e := tx.ExecContext(ctx, "UPDATE mochat_go_message_intercept_records SET audit_status=? WHERE id=? AND tenant_id=? AND corp_id=?", action, id, t, c)
		if e != nil {
			return 0, e
		}
		n, _ := r.RowsAffected()
		if n > 0 {
			count++
			if _, e = tx.ExecContext(ctx, "INSERT INTO mochat_go_message_intercept_audits(tenant_id,corp_id,record_id,action,actor_id,remark) VALUES(?,?,?,?,?,?)", t, c, id, action, actor, strings.TrimSpace(remark)); e != nil {
				return 0, e
			}
		}
	}
	return count, tx.Commit()
}
