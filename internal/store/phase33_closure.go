package store

import (
	"context"
	"database/sql"
	"fmt"
	"jiyi/mochat-go/internal/dashboard"
	"strings"
	"time"
)

func closurePage(p, n int) (int, int) {
	if p < 1 {
		p = 1
	}
	if n < 1 || n > 100 {
		n = 20
	}
	return p, n
}
func closureScope(t, c int) (string, []any) {
	if t > 0 {
		return " WHERE tenant_id=? AND corp_id=?", []any{t, c}
	}
	return " WHERE corp_id=?", []any{c}
}
func (s *MySQLStore) SilentRulePage(ctx context.Context, f dashboard.SilentRuleFilter) (dashboard.SilentRulePage, error) {
	f.Page, f.PerPage = closurePage(f.Page, f.PerPage)
	w, a := closureScope(f.TenantID, f.CorpID)
	if f.Name != "" {
		w += " AND name LIKE ?"
		a = append(a, "%"+strings.TrimSpace(f.Name)+"%")
	}
	if f.Status != "" {
		w += " AND status=?"
		a = append(a, f.Status)
	}
	var total int
	if e := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_silent_customer_rules"+w, a...).Scan(&total); e != nil {
		return dashboard.SilentRulePage{}, e
	}
	rows, e := s.db.QueryContext(ctx, "SELECT id,tenant_id,corp_id,name,silent_days,status,trigger_count,created_at,updated_at FROM mochat_go_silent_customer_rules"+w+" ORDER BY updated_at DESC,id DESC LIMIT ? OFFSET ?", append(a, f.PerPage, (f.Page-1)*f.PerPage)...)
	if e != nil {
		return dashboard.SilentRulePage{}, e
	}
	defer rows.Close()
	items := []dashboard.SilentCustomerRule{}
	for rows.Next() {
		var v dashboard.SilentCustomerRule
		var c, u time.Time
		if e = rows.Scan(&v.ID, &v.TenantID, &v.CorpID, &v.Name, &v.SilentDays, &v.Status, &v.TriggerCount, &c, &u); e != nil {
			return dashboard.SilentRulePage{}, e
		}
		v.CreatedAt = c.Format(time.RFC3339)
		v.UpdatedAt = u.Format(time.RFC3339)
		items = append(items, v)
	}
	return dashboard.SilentRulePage{Items: items, Total: total, Page: f.Page, PerPage: f.PerPage}, rows.Err()
}
func (s *MySQLStore) SaveSilentRule(ctx context.Context, v dashboard.SilentCustomerRule) (int64, error) {
	if e := dashboard.ValidateSilentRule(v); e != nil {
		return 0, e
	}
	if v.ID > 0 {
		r, e := s.db.ExecContext(ctx, "UPDATE mochat_go_silent_customer_rules SET name=?,silent_days=?,status=? WHERE id=? AND tenant_id=? AND corp_id=?", v.Name, v.SilentDays, v.Status, v.ID, v.TenantID, v.CorpID)
		if e != nil {
			return 0, e
		}
		n, _ := r.RowsAffected()
		if n == 0 {
			return 0, sql.ErrNoRows
		}
		return v.ID, nil
	}
	r, e := s.db.ExecContext(ctx, "INSERT INTO mochat_go_silent_customer_rules(tenant_id,corp_id,name,silent_days,status) VALUES(?,?,?,?,?)", v.TenantID, v.CorpID, v.Name, v.SilentDays, v.Status)
	if e != nil {
		return 0, e
	}
	return r.LastInsertId()
}
func (s *MySQLStore) SetSilentRuleStatus(ctx context.Context, t, c int, id int64, status string) (bool, error) {
	if status != "enabled" && status != "disabled" {
		return false, fmt.Errorf("规则状态无效")
	}
	r, e := s.db.ExecContext(ctx, "UPDATE mochat_go_silent_customer_rules SET status=? WHERE id=? AND tenant_id=? AND corp_id=?", status, id, t, c)
	if e != nil {
		return false, e
	}
	n, _ := r.RowsAffected()
	return n > 0, nil
}
func (s *MySQLStore) DeleteSilentRule(ctx context.Context, t, c int, id int64) (bool, error) {
	var n int
	if e := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_silent_customer_records WHERE tenant_id=? AND corp_id=? AND rule_id=?", t, c, id).Scan(&n); e != nil {
		return false, e
	}
	if n > 0 {
		return false, fmt.Errorf("规则已有沉默记录，不能删除")
	}
	r, e := s.db.ExecContext(ctx, "DELETE FROM mochat_go_silent_customer_rules WHERE id=? AND tenant_id=? AND corp_id=?", id, t, c)
	if e != nil {
		return false, e
	}
	x, _ := r.RowsAffected()
	return x > 0, nil
}
func (s *MySQLStore) EvaluateSilentCustomer(ctx context.Context, t, c int, in dashboard.SilentCustomerActivity) (int64, error) {
	if e := dashboard.ValidateSilentActivity(in); e != nil {
		return 0, e
	}
	last, _ := time.Parse(time.RFC3339, in.LastInteractionAt)
	days := int(time.Since(last).Hours() / 24)
	rows, e := s.db.QueryContext(ctx, "SELECT id,name,silent_days FROM mochat_go_silent_customer_rules WHERE tenant_id=? AND corp_id=? AND status='enabled' AND silent_days<=?", t, c, days)
	if e != nil {
		return 0, e
	}
	defer rows.Close()
	var count int64
	for rows.Next() {
		var id int64
		var name string
		var threshold int
		if e = rows.Scan(&id, &name, &threshold); e != nil {
			return count, e
		}
		r, e := s.db.ExecContext(ctx, `INSERT INTO mochat_go_silent_customer_records(tenant_id,corp_id,rule_id,rule_name,customer_id,customer_name,employee_id,employee_name,last_interaction_at,silent_days) VALUES(?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE customer_name=VALUES(customer_name),employee_id=VALUES(employee_id),employee_name=VALUES(employee_name),last_interaction_at=VALUES(last_interaction_at),silent_days=VALUES(silent_days),updated_at=CURRENT_TIMESTAMP`, t, c, id, name, in.CustomerID, in.CustomerName, in.EmployeeID, in.EmployeeName, last, days)
		if e != nil {
			return count, e
		}
		n, _ := r.RowsAffected()
		if n > 0 {
			count++
			_, _ = s.db.ExecContext(ctx, "UPDATE mochat_go_silent_customer_rules SET trigger_count=trigger_count+1 WHERE id=? AND tenant_id=? AND corp_id=?", id, t, c)
		}
	}
	return count, rows.Err()
}
func (s *MySQLStore) SilentRecordPage(ctx context.Context, f dashboard.SilentRecordFilter) (dashboard.SilentRecordPage, error) {
	f.Page, f.PerPage = closurePage(f.Page, f.PerPage)
	w, a := closureScope(f.TenantID, f.CorpID)
	if f.Customer != "" {
		w += " AND (customer_name LIKE ? OR customer_id LIKE ?)"
		a = append(a, "%"+f.Customer+"%", "%"+f.Customer+"%")
	}
	if f.Status != "" {
		w += " AND status=?"
		a = append(a, f.Status)
	}
	if f.AssignedEmployeeID > 0 {
		w += " AND assigned_employee_id=?"
		a = append(a, f.AssignedEmployeeID)
	}
	if f.RuleID > 0 {
		w += " AND rule_id=?"
		a = append(a, f.RuleID)
	}
	if f.RestrictEmployeeIDs {
		if len(f.AllowedEmployeeIDs) == 0 {
			w += " AND 1=0"
		} else {
			placeholders := strings.TrimSuffix(strings.Repeat("?,", len(f.AllowedEmployeeIDs)), ",")
			w += " AND (employee_id IN (" + placeholders + ") OR assigned_employee_id IN (" + placeholders + "))"
			for _, id := range f.AllowedEmployeeIDs {
				a = append(a, id)
			}
			for _, id := range f.AllowedEmployeeIDs {
				a = append(a, id)
			}
		}
	}
	var total int
	if e := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_silent_customer_records"+w, a...).Scan(&total); e != nil {
		return dashboard.SilentRecordPage{}, e
	}
	rows, e := s.db.QueryContext(ctx, "SELECT id,rule_id,rule_name,customer_id,customer_name,employee_id,employee_name,last_interaction_at,silent_days,status,assigned_employee_id,follow_up_note,updated_at FROM mochat_go_silent_customer_records"+w+" ORDER BY updated_at DESC,id DESC LIMIT ? OFFSET ?", append(a, f.PerPage, (f.Page-1)*f.PerPage)...)
	if e != nil {
		return dashboard.SilentRecordPage{}, e
	}
	defer rows.Close()
	items := []dashboard.SilentCustomerRecord{}
	for rows.Next() {
		var v dashboard.SilentCustomerRecord
		var last, u time.Time
		if e = rows.Scan(&v.ID, &v.RuleID, &v.RuleName, &v.CustomerID, &v.CustomerName, &v.EmployeeID, &v.EmployeeName, &last, &v.SilentDays, &v.Status, &v.AssignedEmployeeID, &v.FollowUpNote, &u); e != nil {
			return dashboard.SilentRecordPage{}, e
		}
		v.LastInteractionAt = last.Format(time.RFC3339)
		v.UpdatedAt = u.Format(time.RFC3339)
		items = append(items, v)
	}
	return dashboard.SilentRecordPage{Items: items, Total: total, Page: f.Page, PerPage: f.PerPage}, rows.Err()
}
func (s *MySQLStore) ActSilentRecords(ctx context.Context, t, c int, actor int64, ids []int64, action string, assignee int64, remark string) (int64, error) {
	statuses := map[string]string{"assign": "assigned", "followed": "followed", "awakened": "awakened", "ignored": "ignored", "closed": "closed"}
	to, ok := statuses[action]
	if !ok {
		return 0, fmt.Errorf("处置动作无效")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	var count int64
	for _, id := range ids {
		q := "UPDATE mochat_go_silent_customer_records SET status=?,follow_up_note=?"
		a := []any{to, strings.TrimSpace(remark)}
		if action == "assign" {
			q += ",assigned_employee_id=?"
			a = append(a, assignee)
		}
		q += " WHERE id=? AND tenant_id=? AND corp_id=?"
		a = append(a, id, t, c)
		r, e := tx.ExecContext(ctx, q, a...)
		if e != nil {
			return 0, e
		}
		n, _ := r.RowsAffected()
		if n > 0 {
			count++
			if _, e = tx.ExecContext(ctx, "INSERT INTO mochat_go_silent_customer_audits(tenant_id,corp_id,record_id,action,actor_id,remark) VALUES(?,?,?,?,?,?)", t, c, id, action, actor, remark); e != nil {
				return 0, e
			}
		}
	}
	return count, tx.Commit()
}
func (s *MySQLStore) RefuseArchivePage(ctx context.Context, f dashboard.RefuseArchiveFilter) (dashboard.RefuseArchivePage, error) {
	f.Page, f.PerPage = closurePage(f.Page, f.PerPage)
	w, a := closureScope(f.TenantID, f.CorpID)
	if f.Subject != "" {
		w += " AND (subject_name LIKE ? OR subject_id LIKE ?)"
		a = append(a, "%"+f.Subject+"%", "%"+f.Subject+"%")
	}
	if f.SubjectType != "" {
		w += " AND subject_type=?"
		a = append(a, f.SubjectType)
	}
	if f.EmployeeID > 0 {
		w += " AND employee_id=?"
		a = append(a, f.EmployeeID)
	}
	if f.RefusedFrom != "" {
		w += " AND refused_at >= ?"
		a = append(a, f.RefusedFrom+" 00:00:00")
	}
	if f.RefusedTo != "" {
		w += " AND refused_at < DATE_ADD(?, INTERVAL 1 DAY)"
		a = append(a, f.RefusedTo+" 00:00:00")
	}
	if f.AuthorizationStatus != "" {
		w += " AND authorization_status=?"
		a = append(a, f.AuthorizationStatus)
	}
	if f.FollowUpStatus != "" {
		w += " AND follow_up_status=?"
		a = append(a, f.FollowUpStatus)
	}
	var total int
	if e := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_refuse_archive_records"+w, a...).Scan(&total); e != nil {
		return dashboard.RefuseArchivePage{}, e
	}
	rows, e := s.db.QueryContext(ctx, "SELECT id,subject_type,subject_id,subject_name,employee_id,employee_name,authorization_status,source,refused_at,authorized_at,last_follow_up_at,follow_up_status,follow_up_note,updated_at FROM mochat_go_refuse_archive_records"+w+" ORDER BY updated_at DESC,id DESC LIMIT ? OFFSET ?", append(a, f.PerPage, (f.Page-1)*f.PerPage)...)
	if e != nil {
		return dashboard.RefuseArchivePage{}, e
	}
	defer rows.Close()
	items := []dashboard.RefuseArchiveRecord{}
	for rows.Next() {
		var v dashboard.RefuseArchiveRecord
		var refused, authorized, follow sql.NullTime
		var u time.Time
		if e = rows.Scan(&v.ID, &v.SubjectType, &v.SubjectID, &v.SubjectName, &v.EmployeeID, &v.EmployeeName, &v.AuthorizationStatus, &v.Source, &refused, &authorized, &follow, &v.FollowUpStatus, &v.FollowUpNote, &u); e != nil {
			return dashboard.RefuseArchivePage{}, e
		}
		if refused.Valid {
			v.RefusedAt = refused.Time.Format(time.RFC3339)
		}
		if authorized.Valid {
			v.AuthorizedAt = authorized.Time.Format(time.RFC3339)
		}
		if follow.Valid {
			v.LastFollowUpAt = follow.Time.Format(time.RFC3339)
		}
		v.UpdatedAt = u.Format(time.RFC3339)
		items = append(items, v)
	}
	return dashboard.RefuseArchivePage{Items: items, Total: total, Page: f.Page, PerPage: f.PerPage}, rows.Err()
}
func (s *MySQLStore) UpsertRefuseArchive(ctx context.Context, t, c int, actor int64, v dashboard.RefuseArchiveRecord) (int64, error) {
	if e := dashboard.ValidateRefuseArchive(v); e != nil {
		return 0, e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	var id int64
	var old string
	e = tx.QueryRowContext(ctx, "SELECT id,authorization_status FROM mochat_go_refuse_archive_records WHERE tenant_id=? AND corp_id=? AND subject_type=? AND subject_id=? AND employee_id=? FOR UPDATE", t, c, v.SubjectType, v.SubjectID, v.EmployeeID).Scan(&id, &old)
	now := time.Now()
	if e == sql.ErrNoRows {
		r, e := tx.ExecContext(ctx, `INSERT INTO mochat_go_refuse_archive_records(tenant_id,corp_id,subject_type,subject_id,subject_name,employee_id,employee_name,authorization_status,source,refused_at,authorized_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, t, c, v.SubjectType, v.SubjectID, v.SubjectName, v.EmployeeID, v.EmployeeName, v.AuthorizationStatus, v.Source, nullableTime(v.AuthorizationStatus == "refused", now), nullableTime(v.AuthorizationStatus == "authorized", now))
		if e != nil {
			return 0, e
		}
		id, _ = r.LastInsertId()
	} else if e != nil {
		return 0, e
	} else {
		_, e = tx.ExecContext(ctx, `UPDATE mochat_go_refuse_archive_records SET subject_name=?,employee_name=?,authorization_status=?,source=?,refused_at=IF(?='refused',?,refused_at),authorized_at=IF(?='authorized',?,authorized_at) WHERE id=? AND tenant_id=? AND corp_id=?`, v.SubjectName, v.EmployeeName, v.AuthorizationStatus, v.Source, v.AuthorizationStatus, now, v.AuthorizationStatus, now, id, t, c)
		if e != nil {
			return 0, e
		}
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO mochat_go_refuse_archive_audits(tenant_id,corp_id,record_id,action,from_status,to_status,actor_id,remark) VALUES(?,?,?,?,?,?,?,?)", t, c, id, "status_sync", old, v.AuthorizationStatus, actor, "provider status sync"); e != nil {
		return 0, e
	}
	return id, tx.Commit()
}
func nullableTime(ok bool, t time.Time) any {
	if ok {
		return t
	}
	return nil
}
func (s *MySQLStore) FollowUpRefuseArchive(ctx context.Context, t, c int, actor, id int64, status, note string) (bool, error) {
	if status != "contacted" && status != "waiting" && status != "completed" {
		return false, fmt.Errorf("跟进状态无效")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	r, e := tx.ExecContext(ctx, "UPDATE mochat_go_refuse_archive_records SET follow_up_status=?,follow_up_note=?,last_follow_up_at=NOW() WHERE id=? AND tenant_id=? AND corp_id=?", status, strings.TrimSpace(note), id, t, c)
	if e != nil {
		return false, e
	}
	n, _ := r.RowsAffected()
	if n > 0 {
		if _, e = tx.ExecContext(ctx, "INSERT INTO mochat_go_refuse_archive_audits(tenant_id,corp_id,record_id,action,to_status,actor_id,remark) VALUES(?,?,?,?,?,?,?)", t, c, id, "follow_up", status, actor, note); e != nil {
			return false, e
		}
	}
	return n > 0, tx.Commit()
}
