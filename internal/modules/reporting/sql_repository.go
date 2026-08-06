package reporting

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SQLRepository is the production reporting source. SQL is deliberately built
// from the columns present in the standalone migrations; optional filters are
// reported as limitations instead of producing invalid SQL on older volumes.
type SQLRepository struct {
	db   *sql.DB
	kind ReportKind
}

func NewSQLRepository(db *sql.DB, kind ReportKind) *SQLRepository {
	return &SQLRepository{db: db, kind: kind}
}

func (r *SQLRepository) Query(ctx context.Context, q ReportQuery) (ReportResult, error) {
	if r == nil || r.db == nil {
		return ReportResult{}, fmt.Errorf("reporting database unavailable")
	}
	switch r.kind {
	case ConversionReport:
		return r.queryConversion(ctx, q)
	case EmployeeReport:
		return r.queryEmployee(ctx, q)
	case BehaviorReport:
		return r.queryBehavior(ctx, q)
	case DetailReport:
		return r.querySummary(ctx, q)
	default:
		return r.queryEntity(ctx, q, r.kind)
	}
}

func (r *SQLRepository) querySummary(ctx context.Context, q ReportQuery) (ReportResult, error) {
	customer, err := r.queryEntity(ctx, q, CustomerReport)
	if err != nil {
		return ReportResult{}, err
	}
	conversion, err := r.queryConversion(ctx, q)
	if err != nil {
		return ReportResult{}, err
	}
	behavior, err := r.queryBehavior(ctx, q)
	if err != nil {
		return ReportResult{}, err
	}
	summary := map[string]*float64{}
	for key, value := range customer.Summary {
		summary[key] = value
	}
	for key, value := range conversion.Summary {
		summary[key] = value
	}
	for key, value := range behavior.Summary {
		summary[key] = value
	}
	return ReportResult{Summary: summary, Series: customer.Series, Dimensions: []Dimension{}, Items: []map[string]any{}, Pagination: Pagination{Page: q.Page, PageSize: q.PageSize, Total: customer.Pagination.Total}, Freshness: Freshness{Provider: "scrm", Status: "available"}, Limitations: append(append(customer.Limitations, conversion.Limitations...), behavior.Limitations...)}, nil
}

func placeholders(n int) string {
	p := make([]string, n)
	for i := range p {
		p[i] = "?"
	}
	return strings.Join(p, ",")
}

func scope(q ReportQuery, alias, created string) (string, []any) {
	return alias + ".tenant_id=? AND " + alias + ".corp_id=? AND " + alias + "." + created + ">=? AND " + alias + "." + created + "<?", []any{q.TenantID, q.CorpID, q.StartAt.UTC(), q.EndAt.UTC()}
}

func limitation(q ReportQuery, provider string) []Limitation {
	if len(q.DepartmentIDs) == 0 {
		return nil
	}
	return []Limitation{{Provider: provider, Code: "department_scope_unavailable", Message: "当前数据表没有可验证的部门字段，已保留租户与企业隔离"}}
}

func (r *SQLRepository) queryEntity(ctx context.Context, q ReportQuery, kind ReportKind) (ReportResult, error) {
	// Contacts have no external_userid/owner_id/department_id in the real
	// schema. Ownership is represented by the assignment table.
	where, args := scope(q, "c", "created_at")
	where += " AND c.deleted_at IS NULL"
	if len(q.EmployeeIDs) > 0 {
		where += " AND EXISTS (SELECT 1 FROM mochat_go_scrm_assignments a WHERE a.tenant_id=c.tenant_id AND a.corp_id=c.corp_id AND a.contact_id=c.id AND a.deleted_at IS NULL AND a.owner_id IN (" + placeholders(len(q.EmployeeIDs)) + "))"
		for _, id := range q.EmployeeIDs {
			args = append(args, id)
		}
	}
	var total float64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT c.id) FROM mochat_go_scrm_contacts c WHERE "+where, args...).Scan(&total); err != nil {
		return ReportResult{}, err
	}
	res := ReportResult{Summary: map[string]*float64{string(kind): &total}, Series: []SeriesPoint{}, Dimensions: []Dimension{}, Items: []map[string]any{}, Pagination: Pagination{Page: q.Page, PageSize: q.PageSize, Total: int(total)}, Freshness: Freshness{Provider: "scrm", Status: "available"}, Limitations: limitation(q, "scrm")}
	if kind != CustomerReport {
		return res, nil
	}
	// Trend uses tenant-local date while preserving the half-open UTC window.
	trendArgs := append([]any{q.Timezone}, args...)
	trendArgs = append(trendArgs, q.Timezone)
	trendSQL := "SELECT DATE_FORMAT(CONVERT_TZ(c.created_at,'+00:00',?), '%Y-%m-%d'), COUNT(DISTINCT c.id) FROM mochat_go_scrm_contacts c WHERE " + where + " GROUP BY DATE_FORMAT(CONVERT_TZ(c.created_at,'+00:00',?), '%Y-%m-%d') ORDER BY 1"
	if rows, err := r.db.QueryContext(ctx, trendSQL, trendArgs...); err == nil {
		defer rows.Close()
		for rows.Next() {
			var day string
			var n float64
			if rows.Scan(&day, &n) == nil {
				res.Series = append(res.Series, SeriesPoint{At: parseReportDay(day), Value: n})
			}
		}
	}
	itemArgs := append([]any{q.Timezone}, args...)
	itemArgs = append(itemArgs, q.PageSize, (q.Page-1)*q.PageSize)
	rows, err := r.db.QueryContext(ctx, "SELECT c.id, DATE_FORMAT(CONVERT_TZ(c.created_at,'+00:00',?), '%Y-%m-%d'), COALESCE((SELECT a.owner_id FROM mochat_go_scrm_assignments a WHERE a.tenant_id=c.tenant_id AND a.corp_id=c.corp_id AND a.contact_id=c.id AND a.deleted_at IS NULL ORDER BY a.updated_at DESC LIMIT 1),0), COALESCE((SELECT e.name FROM mochat_go_scrm_assignments a2 JOIN mc_work_employee e ON e.corp_id=a2.corp_id AND e.log_user_id=a2.owner_id AND e.deleted_at IS NULL WHERE a2.tenant_id=c.tenant_id AND a2.corp_id=c.corp_id AND a2.contact_id=c.id AND a2.deleted_at IS NULL ORDER BY a2.updated_at DESC LIMIT 1),'') FROM mochat_go_scrm_contacts c WHERE "+where+" ORDER BY c.created_at DESC,c.id DESC LIMIT ? OFFSET ?", itemArgs...)
	if err != nil {
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, day string
		var owner sql.NullInt64
		var ownerName sql.NullString
		if rows.Scan(&id, &day, &owner, &ownerName) == nil {
			res.Items = append(res.Items, map[string]any{"id": id, "day": day, "ownerId": owner.Int64, "ownerName": ownerName.String})
		}
	}
	return res, nil
}

func (r *SQLRepository) queryConversion(ctx context.Context, q ReportQuery) (ReportResult, error) {
	tables := []struct {
		table, alias, id, owner string
		deleted                 bool
	}{{"mochat_go_scrm_leads", "l", "l.id", "l.owner_id", false}, {"mochat_go_scrm_contacts", "c", "c.id", "", true}, {"mochat_go_scrm_opportunities", "o", "o.id", "o.owner_id", true}, {"mochat_go_scrm_opportunities", "o", "o.id", "o.owner_id", true}, {"mochat_go_scrm_orders", "ord", "ord.id", "ord.created_by", true}}
	keys := []string{"lead", "contact", "opportunity", "won", "order"}
	nums := make([]float64, 5)
	limits := limitation(q, "scrm")
	for i, s := range tables {
		w, a := scope(q, s.alias, "created_at")
		if s.deleted {
			w += " AND " + s.alias + ".deleted_at IS NULL"
		}
		if i == 3 {
			w += " AND " + s.alias + ".status='won'"
		}
		if i == 4 {
			w += " AND " + s.alias + ".status IN ('won','completed','paid')"
		}
		if len(q.EmployeeIDs) > 0 {
			if s.owner != "" {
				w += " AND " + s.owner + " IN (" + placeholders(len(q.EmployeeIDs)) + ")"
				for _, id := range q.EmployeeIDs {
					a = append(a, id)
				}
			} else {
				w += " AND EXISTS (SELECT 1 FROM mochat_go_scrm_assignments a WHERE a.tenant_id=c.tenant_id AND a.corp_id=c.corp_id AND a.contact_id=c.id AND a.deleted_at IS NULL AND a.owner_id IN (" + placeholders(len(q.EmployeeIDs)) + "))"
				for _, id := range q.EmployeeIDs {
					a = append(a, id)
				}
			}
		}
		query := "SELECT COUNT(DISTINCT " + s.id + ") FROM " + s.table + " " + s.alias + " WHERE " + w
		if err := r.db.QueryRowContext(ctx, query, a...).Scan(&nums[i]); err != nil {
			return ReportResult{}, err
		}
	}
	s := map[string]*float64{}
	for i, k := range keys {
		s[k] = &nums[i]
		if i > 0 {
			s[k+"Rate"] = Ratio(nums[i], nums[i-1])
		}
	}
	return ReportResult{Summary: s, Series: []SeriesPoint{}, Dimensions: []Dimension{}, Items: []map[string]any{}, Pagination: Pagination{Page: q.Page, PageSize: q.PageSize, Total: int(nums[0])}, Freshness: Freshness{Provider: "scrm", Status: "available"}, Limitations: limits}, nil
}

func (r *SQLRepository) queryEmployee(ctx context.Context, q ReportQuery) (ReportResult, error) {
	tables, err := r.archiveTables(ctx)
	if err != nil {
		return ReportResult{}, err
	}
	if len(tables) == 0 {
		return UnavailableSource("conversation_archive", "会话归档表不可用").Query(ctx, q)
	}
	parts := make([]string, len(tables))
	for i, t := range tables {
		parts[i] = "SELECT employee_id,created_at,tenant_id,corp_id FROM " + t
	}
	union := strings.Join(parts, " UNION ALL ")
	w, a := scope(q, "m", "created_at")
	if len(q.EmployeeIDs) > 0 {
		w += " AND m.employee_id IN (" + placeholders(len(q.EmployeeIDs)) + ")"
		for _, id := range q.EmployeeIDs {
			a = append(a, id)
		}
	}
	var n float64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM (SELECT employee_id FROM ("+union+") m WHERE "+w+") x", a...).Scan(&n); err != nil {
		return ReportResult{}, err
	}
	return ReportResult{Summary: map[string]*float64{"employee": &n}, Items: []map[string]any{}, Series: []SeriesPoint{}, Dimensions: []Dimension{}, Pagination: Pagination{Page: q.Page, PageSize: q.PageSize, Total: int(n)}, Freshness: Freshness{Provider: "conversation_archive", Status: "available"}, Limitations: limitation(q, "conversation_archive")}, nil
}
func (r *SQLRepository) archiveTables(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT t.table_name
FROM information_schema.tables t
WHERE t.table_schema=DATABASE() AND t.table_name LIKE 'mc_work_message_%'
  AND EXISTS (SELECT 1 FROM information_schema.columns c WHERE c.table_schema=t.table_schema AND c.table_name=t.table_name AND c.column_name='employee_id')
  AND EXISTS (SELECT 1 FROM information_schema.columns c WHERE c.table_schema=t.table_schema AND c.table_name=t.table_name AND c.column_name='corp_id')
  AND EXISTS (SELECT 1 FROM information_schema.columns c WHERE c.table_schema=t.table_schema AND c.table_name=t.table_name AND c.column_name='tenant_id')
  AND EXISTS (SELECT 1 FROM information_schema.columns c WHERE c.table_schema=t.table_schema AND c.table_name=t.table_name AND c.column_name='created_at')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil && strings.HasPrefix(t, "mc_work_message_") {
			out = append(out, t)
		}
	}
	return out, nil
}

func (r *SQLRepository) queryBehavior(ctx context.Context, q ReportQuery) (ReportResult, error) {
	var exists int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='mochat_go_scrm_order_audit'").Scan(&exists); err != nil {
		return ReportResult{}, err
	}
	if exists == 0 {
		return UnavailableSource("business_audit", "没有可用的业务审计事件表").Query(ctx, q)
	}
	w, a := scope(q, "oa", "created_at")
	w += " AND oa.action IN ('created','transition','setting.updated')"
	if len(q.EmployeeIDs) > 0 {
		w += " AND oa.actor_id IN (" + placeholders(len(q.EmployeeIDs)) + ")"
		for _, id := range q.EmployeeIDs {
			a = append(a, id)
		}
	}
	var n float64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_scrm_order_audit oa WHERE "+w, a...).Scan(&n); err != nil {
		return ReportResult{}, err
	}
	rows, err := r.db.QueryContext(ctx, "SELECT oa.action,oa.actor_id,oa.order_id,oa.created_at,oa.id FROM mochat_go_scrm_order_audit oa WHERE "+w+" ORDER BY oa.created_at DESC LIMIT ? OFFSET ?", append(a, q.PageSize, (q.Page-1)*q.PageSize)...)
	if err != nil {
		return ReportResult{}, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var typ, objectID, id string
		var actorID int64
		var at time.Time
		if rows.Scan(&typ, &actorID, &objectID, &at, &id) == nil {
			detail := "订单业务操作"
			if typ == "setting.updated" {
				detail = "客户设置已更新"
			}
			items = append(items, map[string]any{"eventType": typ, "actorId": actorID, "objectId": objectID, "id": id, "occurredAt": at, "detail": detail})
		}
	}
	return ReportResult{Summary: map[string]*float64{"behavior": &n}, Items: items, Series: []SeriesPoint{}, Dimensions: []Dimension{}, Pagination: Pagination{Page: q.Page, PageSize: q.PageSize, Total: int(n)}, Freshness: Freshness{Provider: "business_audit", Status: "available"}, Limitations: limitation(q, "business_audit")}, nil
}
