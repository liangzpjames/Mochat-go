package reporting

import (
	"context"
	"database/sql"
	"fmt"
)

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
	if r.kind == ConversionReport {
		return r.queryConversion(ctx, q)
	}
	table := "mochat_go_scrm_contacts"
	provider := "scrm"
	switch r.kind {
	case ConversionReport:
		table = "mochat_go_scrm_leads"
	case EmployeeReport:
		table = "mc_work_message_1"
		provider = "conversation_archive"
	case BehaviorReport:
		table = "mochat_go_scrm_order_audit"
		provider = "business_audit"
	case DetailReport:
		table = "mochat_go_scrm_contacts"
	}
	var count float64
	key := "COALESCE(NULLIF(external_userid,''),id)"
	if r.kind != CustomerReport {
		key = "id"
	}
	query := "SELECT COUNT(DISTINCT " + key + ") FROM " + table + " WHERE tenant_id=? AND corp_id=?"
	err := r.db.QueryRowContext(ctx, query, q.TenantID, q.CorpID).Scan(&count)
	if err != nil {
		return ReportResult{}, err
	}
	result := ReportResult{Summary: map[string]*float64{string(r.kind): &count}, Pagination: Pagination{Page: q.Page, PageSize: q.PageSize}, Freshness: Freshness{Provider: provider, Status: "available"}}
	if r.kind == CustomerReport {
		result.Items = []map[string]any{}
		rows, queryErr := r.db.QueryContext(ctx, "SELECT "+key+", DATE(created_at) FROM "+table+" WHERE tenant_id=? AND corp_id=? ORDER BY created_at DESC LIMIT ? OFFSET ?", q.TenantID, q.CorpID, q.PageSize, (q.Page-1)*q.PageSize)
		if queryErr == nil {
			defer rows.Close()
			for rows.Next() {
				var id, day string
				if rows.Scan(&id, &day) == nil {
					result.Items = append(result.Items, map[string]any{"id": id, "day": day})
				}
			}
		}
	}
	if r.kind == BehaviorReport {
		var exists int
		if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='mochat_go_scrm_order_audit'").Scan(&exists); err != nil {
			return ReportResult{}, err
		}
		if exists == 0 {
			return UnavailableSource("business_audit", "没有可用的业务审计事件表").Query(ctx, q)
		}
	}
	if r.kind == EmployeeReport {
		var tables int
		if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name LIKE 'mc_work_message_%'").Scan(&tables); err != nil {
			return ReportResult{}, err
		}
		if tables == 0 {
			return UnavailableSource("conversation_archive", "会话归档表不可用").Query(ctx, q)
		}
	}
	return result, nil
}

func (r *SQLRepository) queryConversion(ctx context.Context, q ReportQuery) (ReportResult, error) {
	tables := []string{"mochat_go_scrm_leads", "mochat_go_scrm_contacts", "mochat_go_scrm_opportunities", "mochat_go_scrm_opportunities", "mochat_go_scrm_orders"}
	keys := []string{"lead", "contact", "opportunity", "won", "order"}
	summary := map[string]*float64{}
	for i, t := range tables {
		where := "tenant_id=? AND corp_id=?"
		if i == 3 {
			where += " AND status='won'"
		}
		var n float64
		if err := r.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT id) FROM "+t+" WHERE "+where, q.TenantID, q.CorpID).Scan(&n); err != nil {
			return ReportResult{}, err
		}
		summary[keys[i]] = &n
	}
	return ReportResult{Summary: summary, Pagination: Pagination{Page: q.Page, PageSize: q.PageSize}, Freshness: Freshness{Provider: "scrm", Status: "available"}}, nil
}
