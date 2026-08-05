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
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE tenant_id=? AND corp_id=?", q.TenantID, q.CorpID).Scan(&count)
	if err != nil {
		return ReportResult{}, err
	}
	return ReportResult{Summary: map[string]*float64{string(r.kind): &count}, Pagination: Pagination{Page: q.Page, PageSize: q.PageSize}, Freshness: Freshness{Provider: provider, Status: "available"}}, nil
}
