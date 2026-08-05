package reporting

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SQLRepository is the production reporting source. All predicates are scoped by
// tenant/corp and by the half-open UTC window supplied by the service.
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
	default:
		return r.queryEntity(ctx, q, r.kind)
	}
}

func bounds(q ReportQuery) (string, []any) {
	return "tenant_id=? AND corp_id=? AND created_at>=? AND created_at<?", []any{q.TenantID, q.CorpID, q.StartAt.UTC(), q.EndAt.UTC()}
}

func scopedWhere(q ReportQuery, owner string) (string, []any) {
	w, a := bounds(q)
	if len(q.DepartmentIDs) > 0 {
		w += " AND department_id IN (" + placeholders(len(q.DepartmentIDs)) + ")"
		for _, id := range q.DepartmentIDs {
			a = append(a, id)
		}
	}
	if len(q.EmployeeIDs) > 0 {
		w += " AND " + owner + " IN (" + placeholders(len(q.EmployeeIDs)) + ")"
		for _, id := range q.EmployeeIDs {
			a = append(a, id)
		}
	}
	return w, a
}
func placeholders(n int) string {
	p := make([]string, n)
	for i := range p {
		p[i] = "?"
	}
	return strings.Join(p, ",")
}

func (r *SQLRepository) queryEntity(ctx context.Context, q ReportQuery, kind ReportKind) (ReportResult, error) {
	table, provider, owner := "mochat_go_scrm_contacts", "scrm", "owner_id"
	key := "id"
	if kind == CustomerReport {
		key = "COALESCE(NULLIF(external_userid,''),id)"
	}
	where, args := scopedWhere(q, owner)
	var total float64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT "+key+") FROM "+table+" WHERE "+where, args...).Scan(&total); err != nil {
		return ReportResult{}, err
	}
	result := ReportResult{Summary: map[string]*float64{string(kind): &total}, Series: []SeriesPoint{}, Dimensions: []Dimension{}, Items: []map[string]any{}, Pagination: Pagination{Page: q.Page, PageSize: q.PageSize, Total: int(total)}, Freshness: Freshness{Provider: provider, Status: "available"}}
	if kind != CustomerReport {
		return result, nil
	}
	// Daily trend is evaluated in the tenant's requested timezone and retains
	// the half-open window in the inner predicate.
	trendSQL := "SELECT DATE(CONVERT_TZ(created_at,'+00:00',?)), COUNT(DISTINCT " + key + ") FROM " + table + " WHERE " + where + " GROUP BY DATE(CONVERT_TZ(created_at,'+00:00',?)) ORDER BY 1"
	trendArgs := append([]any{q.Timezone}, args...)
	trendArgs = append(trendArgs, q.Timezone)
	if rows, err := r.db.QueryContext(ctx, trendSQL, trendArgs...); err == nil {
		defer rows.Close()
		for rows.Next() {
			var day string
			var n float64
			if rows.Scan(&day, &n) == nil {
				t, _ := time.ParseInLocation("2006-01-02", day, time.UTC)
				result.Series = append(result.Series, SeriesPoint{At: t, Value: n})
			}
		}
	}
	rows, err := r.db.QueryContext(ctx, "SELECT "+key+", DATE(CONVERT_TZ(created_at,'+00:00',?)), COALESCE(owner_id,0) FROM "+table+" WHERE "+where+" ORDER BY created_at DESC LIMIT ? OFFSET ?", append([]any{q.Timezone}, append(args, q.PageSize, (q.Page-1)*q.PageSize)...)...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, day string
		var owner int64
		if rows.Scan(&id, &day, &owner) == nil {
			result.Items = append(result.Items, map[string]any{"id": id, "day": day, "ownerId": owner})
		}
	}
	dimRows, dimErr := r.db.QueryContext(ctx, "SELECT COALESCE(owner_id,0), COUNT(DISTINCT "+key+") FROM "+table+" WHERE "+where+" GROUP BY owner_id ORDER BY 2 DESC", args...)
	if dimErr == nil {
		defer dimRows.Close()
		for dimRows.Next() {
			var owner int64
			var n float64
			if dimRows.Scan(&owner, &n) == nil {
				result.Dimensions = append(result.Dimensions, Dimension{Key: fmt.Sprint(owner), Label: "负责人", Value: n})
			}
		}
	}
	return result, nil
}

func (r *SQLRepository) queryConversion(ctx context.Context, q ReportQuery) (ReportResult, error) {
	tables := []string{"mochat_go_scrm_leads", "mochat_go_scrm_contacts", "mochat_go_scrm_opportunities", "mochat_go_scrm_opportunities", "mochat_go_scrm_orders"}
	keys := []string{"lead", "contact", "opportunity", "won", "order"}
	nums := make([]float64, 5)
	for i, t := range tables {
		w, a := bounds(q)
		if len(q.DepartmentIDs) > 0 {
			w += " AND department_id IN (" + placeholders(len(q.DepartmentIDs)) + ")"
			for _, id := range q.DepartmentIDs {
				a = append(a, id)
			}
		}
		if len(q.EmployeeIDs) > 0 {
			w += " AND owner_id IN (" + placeholders(len(q.EmployeeIDs)) + ")"
			for _, id := range q.EmployeeIDs {
				a = append(a, id)
			}
		}
		if i == 3 {
			w += " AND status='won'"
		}
		if i == 4 {
			w += " AND status IN ('won','completed','paid')"
		}
		if err := r.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT id) FROM "+t+" WHERE "+w, a...).Scan(&nums[i]); err != nil {
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
	return ReportResult{Summary: s, Series: []SeriesPoint{}, Dimensions: []Dimension{}, Items: []map[string]any{}, Pagination: Pagination{Page: q.Page, PageSize: q.PageSize, Total: int(nums[0])}, Freshness: Freshness{Provider: "scrm", Status: "available"}}, nil
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
		parts[i] = "SELECT employee_id,department_id,created_at,tenant_id,corp_id FROM " + t
	}
	union := strings.Join(parts, " UNION ALL ")
	w, a := bounds(q)
	if len(q.DepartmentIDs) > 0 {
		w += " AND m.department_id IN (" + placeholders(len(q.DepartmentIDs)) + ")"
		for _, id := range q.DepartmentIDs {
			a = append(a, id)
		}
	}
	if len(q.EmployeeIDs) > 0 {
		w += " AND m.employee_id IN (" + placeholders(len(q.EmployeeIDs)) + ")"
		for _, id := range q.EmployeeIDs {
			a = append(a, id)
		}
	}
	query := "SELECT COUNT(*) FROM (SELECT employee_id FROM (" + union + ") m WHERE " + strings.ReplaceAll(w, "created_at", "m.created_at") + ") x"
	var n float64
	if err := r.db.QueryRowContext(ctx, query, a...).Scan(&n); err != nil {
		return ReportResult{}, err
	}
	return ReportResult{Summary: map[string]*float64{"employee": &n}, Items: []map[string]any{}, Series: []SeriesPoint{}, Dimensions: []Dimension{}, Pagination: Pagination{Page: q.Page, PageSize: q.PageSize, Total: int(n)}, Freshness: Freshness{Provider: "conversation_archive", Status: "available"}}, nil
}
func (r *SQLRepository) archiveTables(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name LIKE 'mc_work_message_%'")
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
	w, a := bounds(q)
	// The durable audit schema records action (created/transition), not a
	// synthetic event_type. Keep the whitelist tied to those persisted values.
	w += " AND action IN ('created','transition')"
	if len(q.EmployeeIDs) > 0 {
		w += " AND actor_id IN (" + placeholders(len(q.EmployeeIDs)) + ")"
		for _, id := range q.EmployeeIDs {
			a = append(a, id)
		}
	}
	var n float64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_scrm_order_audit WHERE "+w, a...).Scan(&n); err != nil {
		return ReportResult{}, err
	}
	rows, err := r.db.QueryContext(ctx, "SELECT action,created_at,id FROM mochat_go_scrm_order_audit WHERE "+w+" ORDER BY created_at DESC LIMIT ? OFFSET ?", append(a, q.PageSize, (q.Page-1)*q.PageSize)...)
	if err != nil {
		return ReportResult{}, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var typ string
		var at time.Time
		var id string
		if rows.Scan(&typ, &at, &id) == nil {
			items = append(items, map[string]any{"eventType": typ, "id": id, "occurredAt": at})
		}
	}
	limitations := []Limitation{}
	if len(q.DepartmentIDs) > 0 {
		limitations = append(limitations, Limitation{Provider: "business_audit", Code: "department_scope_unavailable", Message: "业务审计表未记录部门字段，已保留租户与企业隔离"})
	}
	return ReportResult{Summary: map[string]*float64{"behavior": &n}, Items: items, Series: []SeriesPoint{}, Dimensions: []Dimension{}, Pagination: Pagination{Page: q.Page, PageSize: q.PageSize, Total: int(n)}, Freshness: Freshness{Provider: "business_audit", Status: "available"}, Limitations: limitations}, nil
}
