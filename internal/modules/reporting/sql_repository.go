package reporting

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	case OverviewReport:
		return r.queryOverview(ctx, q)
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

func (r *SQLRepository) queryOverview(ctx context.Context, q ReportQuery) (ReportResult, error) {
	customer, err := r.queryEntity(ctx, q, CustomerReport)
	if err != nil {
		return ReportResult{}, err
	}
	if q.TrendStartAt != nil && q.TrendEndAt != nil {
		trendQuery := q
		trendQuery.StartAt = *q.TrendStartAt
		trendQuery.EndAt = *q.TrendEndAt
		trend, trendErr := r.queryEntity(ctx, trendQuery, CustomerReport)
		if trendErr != nil {
			return ReportResult{}, trendErr
		}
		customer.Series = trend.Series
	}
	conversion, err := r.queryConversion(ctx, q)
	if err != nil {
		return ReportResult{}, err
	}
	behavior, err := r.queryBehavior(ctx, q)
	if err != nil {
		return ReportResult{}, err
	}
	employee, err := r.queryEmployee(ctx, q)
	if err != nil {
		return ReportResult{}, err
	}
	summary := map[string]*float64{}
	for _, result := range []ReportResult{customer, conversion, behavior, employee} {
		for key, value := range result.Summary {
			summary[key] = value
		}
	}
	limitations := append([]Limitation{}, customer.Limitations...)
	limitations = append(limitations, conversion.Limitations...)
	limitations = append(limitations, behavior.Limitations...)
	limitations = append(limitations, employee.Limitations...)
	aiInsight, aiErr := r.queryAIInsight(ctx, q.CorpID)
	if aiErr != nil {
		return ReportResult{}, aiErr
	}
	conversation, conversationErr := r.queryConversationStats(ctx, q)
	if conversationErr != nil {
		return ReportResult{}, conversationErr
	}
	return ReportResult{
		Summary:     summary,
		Series:      customer.Series,
		Items:       customer.Items,
		Pagination:  customer.Pagination,
		Freshness:   Freshness{Provider: "scrm", Status: "available", DataThrough: time.Now().UTC()},
		Limitations: limitations,
		AIInsight:   aiInsight,
		Conversation: conversation,
	}, nil
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
	trendSQL := "SELECT DATE_FORMAT(CONVERT_TZ(c.created_at, @@session.time_zone, ?), '%Y-%m-%d'), COUNT(DISTINCT c.id) FROM mochat_go_scrm_contacts c WHERE " + where + " GROUP BY DATE_FORMAT(CONVERT_TZ(c.created_at, @@session.time_zone, ?), '%Y-%m-%d') ORDER BY 1"
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
	rows, err := r.db.QueryContext(ctx, "SELECT c.id, DATE_FORMAT(CONVERT_TZ(c.created_at, @@session.time_zone, ?), '%Y-%m-%d'), COALESCE((SELECT a.owner_id FROM mochat_go_scrm_assignments a WHERE a.tenant_id=c.tenant_id AND a.corp_id=c.corp_id AND a.contact_id=c.id AND a.deleted_at IS NULL ORDER BY a.updated_at DESC LIMIT 1),0), COALESCE((SELECT e.name FROM mochat_go_scrm_assignments a2 JOIN mc_work_employee e ON e.corp_id=a2.corp_id AND e.id=a2.owner_id AND e.deleted_at IS NULL WHERE a2.tenant_id=c.tenant_id AND a2.corp_id=c.corp_id AND a2.contact_id=c.id AND a2.deleted_at IS NULL ORDER BY a2.updated_at DESC LIMIT 1),'') FROM mochat_go_scrm_contacts c WHERE "+where+" ORDER BY c.created_at DESC,c.id DESC LIMIT ? OFFSET ?", itemArgs...)
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
	stages := []conversionStage{
		{key: "lead", table: "mochat_go_scrm_leads", alias: "l", id: "l.id", owner: "l.owner_id"},
		{key: "contact", table: "mochat_go_scrm_contacts", alias: "c", id: "c.id", deleted: true},
		{key: "opportunity", table: "mochat_go_scrm_opportunities", alias: "o", id: "o.id", owner: "o.owner_id", deleted: true},
		{key: "won", table: "mochat_go_scrm_opportunities", alias: "o", id: "o.id", owner: "o.owner_id", deleted: true, extra: "o.status='won'"},
		{key: "order", table: "mochat_go_scrm_orders", alias: "ord", id: "ord.id", owner: "ord.created_by", deleted: true, extra: "ord.status IN ('won','completed','paid')"},
	}
	nums := make([]float64, len(stages))
	limits := limitation(q, "scrm")
	whereByStage := make([]string, len(stages))
	argsByStage := make([][]any, len(stages))
	for i, s := range stages {
		w, a := scope(q, s.alias, "created_at")
		if s.deleted {
			w += " AND " + s.alias + ".deleted_at IS NULL"
		}
		if s.extra != "" {
			w += " AND " + s.extra
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
		whereByStage[i] = w
		argsByStage[i] = a
		query := "SELECT COUNT(DISTINCT " + s.id + ") FROM " + s.table + " " + s.alias + " WHERE " + w
		if err := r.db.QueryRowContext(ctx, query, a...).Scan(&nums[i]); err != nil {
			return ReportResult{}, err
		}
	}
	s := map[string]*float64{}
	for i, stage := range stages {
		s[stage.key] = &nums[i]
		if i > 0 {
			s[stage.key+"Rate"] = Ratio(nums[i], nums[i-1])
		}
	}
	res := ReportResult{Summary: s, Series: []SeriesPoint{}, Dimensions: []Dimension{}, Items: []map[string]any{}, Pagination: Pagination{Page: q.Page, PageSize: q.PageSize, Total: int(nums[0])}, Freshness: Freshness{Provider: "scrm", Status: "available"}, Limitations: limits}
	if q.Stage == "" {
		return res, nil
	}
	idx := -1
	for i, stage := range stages {
		if stage.key == q.Stage {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ReportResult{}, fmt.Errorf("%w: unknown conversion stage", ErrInvalidQuery)
	}
	items, err := r.conversionStageItems(ctx, stages[idx], whereByStage[idx], argsByStage[idx], q)
	if err != nil {
		return ReportResult{}, err
	}
	res.Items = items
	res.Pagination.Total = int(nums[idx])
	return res, nil
}

type conversionStage struct {
	key, table, alias, id, owner, extra string
	deleted                             bool
}

func (r *SQLRepository) conversionStageItems(ctx context.Context, stage conversionStage, where string, args []any, q ReportQuery) ([]map[string]any, error) {
	var selectSQL string
	switch stage.key {
	case "lead":
		selectSQL = "SELECT l.id,l.name,l.source,l.status,COALESCE(l.owner_id,0),COALESCE(e.name,''),DATE_FORMAT(CONVERT_TZ(l.created_at,'+00:00',?),'%Y-%m-%d') FROM mochat_go_scrm_leads l LEFT JOIN mc_work_employee e ON e.corp_id=l.corp_id AND e.id=l.owner_id AND e.deleted_at IS NULL WHERE " + where + " ORDER BY l.created_at DESC,l.id DESC LIMIT ? OFFSET ?"
	case "contact":
		selectSQL = "SELECT c.id,c.name,c.phone,COALESCE((SELECT a.owner_id FROM mochat_go_scrm_assignments a WHERE a.tenant_id=c.tenant_id AND a.corp_id=c.corp_id AND a.contact_id=c.id AND a.deleted_at IS NULL ORDER BY a.updated_at DESC LIMIT 1),0),COALESCE((SELECT e.name FROM mochat_go_scrm_assignments a2 JOIN mc_work_employee e ON e.corp_id=a2.corp_id AND e.id=a2.owner_id AND e.deleted_at IS NULL WHERE a2.tenant_id=c.tenant_id AND a2.corp_id=c.corp_id AND a2.contact_id=c.id AND a2.deleted_at IS NULL ORDER BY a2.updated_at DESC LIMIT 1),''),DATE_FORMAT(CONVERT_TZ(c.created_at,'+00:00',?),'%Y-%m-%d') FROM mochat_go_scrm_contacts c WHERE " + where + " ORDER BY c.created_at DESC,c.id DESC LIMIT ? OFFSET ?"
	case "opportunity", "won":
		selectSQL = "SELECT o.id,COALESCE(c.name,''),o.status,COALESCE(o.owner_id,0),COALESCE(e.name,''),DATE_FORMAT(CONVERT_TZ(o.created_at,'+00:00',?),'%Y-%m-%d') FROM mochat_go_scrm_opportunities o LEFT JOIN mochat_go_scrm_contacts c ON c.tenant_id=o.tenant_id AND c.corp_id=o.corp_id AND c.id=o.contact_id AND c.deleted_at IS NULL LEFT JOIN mc_work_employee e ON e.corp_id=o.corp_id AND e.id=o.owner_id AND e.deleted_at IS NULL WHERE " + where + " ORDER BY o.created_at DESC,o.id DESC LIMIT ? OFFSET ?"
	case "order":
		selectSQL = "SELECT ord.id,COALESCE(c.name,''),ord.amount_cents,ord.currency,ord.status,COALESCE(ord.created_by,0),COALESCE(e.name,''),DATE_FORMAT(CONVERT_TZ(ord.created_at,'+00:00',?),'%Y-%m-%d') FROM mochat_go_scrm_orders ord LEFT JOIN mochat_go_scrm_contacts c ON c.tenant_id=ord.tenant_id AND c.corp_id=ord.corp_id AND c.id=ord.contact_id AND c.deleted_at IS NULL LEFT JOIN mc_work_employee e ON e.corp_id=ord.corp_id AND e.id=ord.created_by AND e.deleted_at IS NULL WHERE " + where + " ORDER BY ord.created_at DESC,ord.id DESC LIMIT ? OFFSET ?"
	default:
		return nil, fmt.Errorf("%w: unknown conversion stage", ErrInvalidQuery)
	}
	queryArgs := append([]any{q.Timezone}, args...)
	queryArgs = append(queryArgs, q.PageSize, (q.Page-1)*q.PageSize)
	rows, err := r.db.QueryContext(ctx, selectSQL, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, day string
		var ownerID int64
		var ownerName string
		switch stage.key {
		case "lead":
			var source, status string
			if rows.Scan(&id, &name, &source, &status, &ownerID, &ownerName, &day) == nil {
				items = append(items, map[string]any{"id": id, "name": name, "source": source, "status": status, "ownerId": ownerID, "ownerName": ownerName, "day": day})
			}
		case "contact":
			var phone string
			if rows.Scan(&id, &name, &phone, &ownerID, &ownerName, &day) == nil {
				items = append(items, map[string]any{"id": id, "name": name, "phone": phone, "ownerId": ownerID, "ownerName": ownerName, "day": day})
			}
		case "opportunity", "won":
			var status string
			if rows.Scan(&id, &name, &status, &ownerID, &ownerName, &day) == nil {
				items = append(items, map[string]any{"id": id, "contactName": name, "status": status, "ownerId": ownerID, "ownerName": ownerName, "day": day})
			}
		case "order":
			var amountCents int64
			var currency, status string
			if rows.Scan(&id, &name, &amountCents, &currency, &status, &ownerID, &ownerName, &day) == nil {
				items = append(items, map[string]any{"id": id, "contactName": name, "amount": formatMoney(amountCents, currency), "status": status, "ownerId": ownerID, "ownerName": ownerName, "day": day})
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func formatMoney(cents int64, currency string) string {
	symbol := "¥"
	if currency != "" && currency != "CNY" {
		symbol = currency + " "
	}
	return fmt.Sprintf("%s%.2f", symbol, float64(cents)/100)
}

func (r *SQLRepository) queryEmployee(ctx context.Context, q ReportQuery) (ReportResult, error) {
	tables, err := r.archiveTables(ctx)
	if err != nil {
		return ReportResult{}, err
	}
	if len(tables) == 0 {
		return UnavailableSource("conversation_archive", "会话归档表不可用").Query(ctx, q)
	}
	parts := make([]string, 0, len(tables))
	for _, t := range tables {
		parts = append(parts, "SELECT "+t.employeeCol+" AS employee_id, created_at, corp_id FROM "+t.name)
	}
	union := strings.Join(parts, " UNION ALL ")
	w, a := scopeArchive(q, "m")
	if len(q.EmployeeIDs) > 0 {
		w += " AND m.employee_id IN (" + placeholders(len(q.EmployeeIDs)) + ")"
		for _, id := range q.EmployeeIDs {
			a = append(a, id)
		}
	}
	var n float64
	// The employee metric is documented as the number of distinct employees
	// appearing in the archived messages for the scoped range, not the
	// message count; the derived table only exists to apply the tenant/corp
	// and employee scope once across all archive partitions.
	query := "SELECT COUNT(DISTINCT m.employee_id) FROM (" + union + ") m JOIN mc_corp c ON c.id = m.corp_id AND c.deleted_at IS NULL WHERE " + w
	if err := r.db.QueryRowContext(ctx, query, a...).Scan(&n); err != nil {
		return ReportResult{}, err
	}
	return ReportResult{Summary: map[string]*float64{"employee": &n}, Items: []map[string]any{}, Series: []SeriesPoint{}, Dimensions: []Dimension{}, Pagination: Pagination{Page: q.Page, PageSize: q.PageSize, Total: int(n)}, Freshness: Freshness{Provider: "conversation_archive", Status: "available"}, Limitations: limitation(q, "conversation_archive")}, nil
}

type archiveTableShape struct {
	name        string
	employeeCol string
}

func (r *SQLRepository) archiveTables(ctx context.Context) ([]archiveTableShape, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT t.table_name,
  MAX(CASE WHEN c.column_name='employee_id' THEN 1 ELSE 0 END),
  MAX(CASE WHEN c.column_name='work_employee_id' THEN 1 ELSE 0 END)
FROM information_schema.tables t
JOIN information_schema.columns c
  ON c.table_schema=t.table_schema AND c.table_name=t.table_name
WHERE t.table_schema=DATABASE() AND t.table_name LIKE 'mc_work_message_%'
  AND c.column_name IN ('employee_id','work_employee_id','corp_id','created_at')
  AND EXISTS (SELECT 1 FROM information_schema.columns c2 WHERE c2.table_schema=t.table_schema AND c2.table_name=t.table_name AND c2.column_name='corp_id')
  AND EXISTS (SELECT 1 FROM information_schema.columns c3 WHERE c3.table_schema=t.table_schema AND c3.table_name=t.table_name AND c3.column_name='created_at')
GROUP BY t.table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []archiveTableShape
	for rows.Next() {
		var t string
		var hasEmployeeID, hasWorkEmployeeID int
		if rows.Scan(&t, &hasEmployeeID, &hasWorkEmployeeID) == nil && strings.HasPrefix(t, "mc_work_message_") {
			if hasEmployeeID == 0 && hasWorkEmployeeID == 0 {
				continue
			}
			employeeCol := "work_employee_id"
			if hasWorkEmployeeID == 0 && hasEmployeeID == 1 {
				employeeCol = "employee_id"
			}
			out = append(out, archiveTableShape{name: t, employeeCol: employeeCol})
		}
	}
	return out, nil
}

// queryAIInsight reads the most recent persisted smart-analysis result for the
// corp. Page reads never invoke the model; the once-daily job writes these rows.
func (r *SQLRepository) queryAIInsight(ctx context.Context, corpID int64) (*AIInsightSummary, error) {
	var payload string
	err := r.db.QueryRowContext(ctx, `
		SELECT payload
		FROM mochat_go_ai_analysis
		WHERE corp_id = ? AND page = 'smart-analysis' AND status = 'succeeded'
		ORDER BY id DESC
		LIMIT 1`, corpID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var stored map[string]any
	if json.Unmarshal([]byte(payload), &stored) != nil {
		return nil, nil
	}
	summary, _ := stored["summary"].(string)
	generatedAt, _ := stored["generatedAt"].(string)
	return &AIInsightSummary{Capability: "ready", Provider: "dashscope", Summary: summary, GeneratedAt: generatedAt}, nil
}

// queryConversationStats aggregates archived messages from all partitions into
// 客户会话/客户群 session and message counts for the main range, plus a
// seven-day trend ending at the queried end date.
func (r *SQLRepository) queryConversationStats(ctx context.Context, q ReportQuery) (*ConversationStats, error) {
	tables, err := r.archiveTables(ctx)
	if err != nil {
		return nil, err
	}
	if len(tables) == 0 {
		return nil, nil
	}
	start := q.StartAt.UTC()
	end := q.EndAt.UTC()
	trendEnd := end
	if q.TrendEndAt != nil {
		trendEnd = q.TrendEndAt.UTC()
	}
	trendStart := trendEnd.AddDate(0, 0, -7)
	trendDays := conversationTrendDays(trendEnd, q.Timezone)
	stats := &ConversationStats{Trend: []ConversationTrendPoint{}}

	selectParts := make([]string, 0, len(tables))
	args := make([]any, 0, len(tables)*3)
	for _, t := range tables {
		selectParts = append(selectParts, fmt.Sprintf(
			"SELECT work_employee_id, to_user_id, room_id, sender_type FROM %s WHERE corp_id = ? AND msg_data_time >= ? AND msg_data_time < ?",
			t.name))
		args = append(args, q.CorpID, start, end)
	}
	union := strings.Join(selectParts, " UNION ALL ")
	rows, err := r.db.QueryContext(ctx, `
		SELECT CASE WHEN room_id > 0 THEN 1 ELSE 0 END AS is_room,
		       COUNT(DISTINCT CASE WHEN room_id > 0 THEN room_id ELSE CONCAT(work_employee_id, ':', to_user_id) END),
		       COALESCE(SUM(CASE WHEN sender_type = 0 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN sender_type = 1 THEN 1 ELSE 0 END), 0)
		FROM (`+union+`) m
		GROUP BY is_room`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var isRoom, sessions, employeeMessages, customerMessages int
		if err := rows.Scan(&isRoom, &sessions, &employeeMessages, &customerMessages); err != nil {
			return nil, err
		}
		group := ConversationGroupStats{Sessions: sessions, EmployeeMessages: employeeMessages, CustomerMessages: customerMessages}
		if isRoom == 1 {
			stats.Room = group
		} else {
			stats.Customer = group
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	trendParts := make([]string, 0, len(tables))
	trendArgs := make([]any, 0, len(tables)*3)
	for _, t := range tables {
		trendParts = append(trendParts, fmt.Sprintf(`
			SELECT DATE(msg_data_time) AS d,
			       CASE WHEN room_id > 0 THEN 1 ELSE 0 END AS is_room,
			       COUNT(DISTINCT CASE WHEN room_id > 0 THEN room_id ELSE CONCAT(work_employee_id, ':', to_user_id) END),
			       SUM(CASE WHEN sender_type = 0 THEN 1 ELSE 0 END),
			       SUM(CASE WHEN sender_type = 1 THEN 1 ELSE 0 END)
			FROM %s
			WHERE corp_id = ? AND msg_data_time >= ? AND msg_data_time < ?
			GROUP BY DATE(msg_data_time), is_room`, t.name))
		trendArgs = append(trendArgs, q.CorpID, trendStart, trendEnd)
	}
	trendRows, err := r.db.QueryContext(ctx, strings.Join(trendParts, " UNION ALL "), trendArgs...)
	if err != nil {
		return nil, err
	}
	defer trendRows.Close()
	dayMap := map[string]ConversationTrendPoint{}
	for trendRows.Next() {
		var day string
		var isRoom, sessions, employeeMessages, customerMessages int
		if err := trendRows.Scan(&day, &isRoom, &sessions, &employeeMessages, &customerMessages); err != nil {
			return nil, err
		}
		point := dayMap[day]
		if isRoom == 1 {
			point.RoomSessions = sessions
			point.RoomEmployeeMessages = employeeMessages
			point.RoomCustomerMessages = customerMessages
		} else {
			point.CustomerSessions = sessions
			point.CustomerEmployeeMessages = employeeMessages
			point.CustomerCustomerMessages = customerMessages
		}
		dayMap[day] = point
	}
	if err := trendRows.Err(); err != nil {
		return nil, err
	}
	for index := 0; index < 7; index++ {
		day := trendDays[index]
		point := dayMap[day]
		point.Date = day
		stats.Trend = append(stats.Trend, point)
	}
	return stats, nil
}

// conversationTrendDays returns the seven local calendar days covered by the
// half-open window [trendEnd-7d, trendEnd) rendered in the query timezone.
// The stored archive timestamps are session-local wall-clock values, so the
// day labels must follow the query timezone instead of UTC.
func conversationTrendDays(trendEnd time.Time, timezone string) []string {
	loc := time.UTC
	if tz, err := time.LoadLocation(timezone); err == nil {
		loc = tz
	}
	endLocal := trendEnd.In(loc)
	days := make([]string, 0, 7)
	for offset := -7; offset < 0; offset++ {
		day := time.Date(endLocal.Year(), endLocal.Month(), endLocal.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, offset)
		days = append(days, day.Format("2006-01-02"))
	}
	return days
}

// scopeArchive scopes archive partitions by corp_id directly and resolves the
// tenant through mc_corp, because the real partition schema has no tenant_id
// column. The derived table is aliased m and mc_corp is aliased c.
func scopeArchive(q ReportQuery, alias string) (string, []any) {
	return "c.tenant_id=? AND " + alias + ".corp_id=? AND " + alias + ".created_at>=? AND " + alias + ".created_at<?", []any{q.TenantID, q.CorpID, q.StartAt.UTC(), q.EndAt.UTC()}
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
