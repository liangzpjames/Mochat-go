package store

import (
	"context"
	"database/sql"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

const customerDirectoryPageSize = 50

type customerDirectorySourceResult struct {
	sql        string
	args       []any
	available  bool
	limitation dashboard.WorkMessageCustomerLimitation
}

// WorkMessageCustomerDirectory aggregates both direct and group conversations by
// the external-contact ID.  All counts and the selected page derive from the
// same archive source so pagination cannot disagree with the mode counters.
func (s *MySQLStore) WorkMessageCustomerDirectory(ctx context.Context, filter dashboard.WorkMessageCustomerDirectoryFilter) (dashboard.WorkMessageCustomerDirectoryPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PageSize = customerDirectoryPageSize
	if filter.RestrictEmployeeIDs && len(uniquePositiveInts(filter.EmployeeIDs)) == 0 {
		return emptyCustomerDirectory(filter), nil
	}
	source, err := s.customerDirectorySourceResult(ctx, filter)
	if err != nil {
		return dashboard.WorkMessageCustomerDirectoryPage{}, err
	}
	if !source.available {
		return unavailableCustomerDirectoryWithLimitation(filter, source.limitation), nil
	}
	where, whereArgs := customerDirectoryOuterWhere(filter)
	counts, err := s.customerDirectoryCounts(ctx, source.sql, source.args, filter)
	if err != nil {
		return dashboard.WorkMessageCustomerDirectoryPage{}, err
	}
	total, err := countDerivedRows(ctx, s.db, source.sql, source.args, where, whereArgs)
	if err != nil {
		return dashboard.WorkMessageCustomerDirectoryPage{}, err
	}
	customers, err := s.customerDirectoryPage(ctx, source.sql, source.args, where, whereArgs, filter.Page, customerDirectoryPageSize)
	if err != nil {
		return dashboard.WorkMessageCustomerDirectoryPage{}, err
	}
	return dashboard.WorkMessageCustomerDirectoryPage{
		Customers: customers, Counts: counts, Page: filter.Page, PageSize: customerDirectoryPageSize, Total: total,
		Limitations: customerDirectoryLimitations(customers), Capabilities: dashboard.WorkMessageCustomerCapabilities(),
	}, nil
}

func customerDirectoryOuterWhere(filter dashboard.WorkMessageCustomerDirectoryFilter) (string, []any) {
	where := []string{"1 = 1"}
	args := []any{}
	switch filter.Mode {
	case dashboard.WorkMessageCustomerModeFocused:
		where = append(where, "focused_conversation_count > 0")
	case dashboard.WorkMessageCustomerModeActive:
		where = append(where, "active_relation_count > 0")
	case dashboard.WorkMessageCustomerModeLost:
		where = append(where, "active_relation_count = 0 AND lost_relation_count > 0")
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		where = append(where, `(name LIKE ? ESCAPE '\\' OR archive_name LIKE ? ESCAPE '\\' OR external_userid LIKE ? ESCAPE '\\')`)
		pattern := workMessageLikePattern(keyword)
		args = append(args, pattern, pattern, pattern)
	}
	return strings.Join(where, " AND "), args
}

func (s *MySQLStore) customerDirectorySource(ctx context.Context, filter dashboard.WorkMessageCustomerDirectoryFilter) (string, []any, bool, error) {
	source, err := s.customerDirectorySourceResult(ctx, filter)
	if err != nil {
		return "", nil, false, err
	}
	return source.sql, source.args, source.available, nil
}

func (s *MySQLStore) customerDirectorySourceResult(ctx context.Context, filter dashboard.WorkMessageCustomerDirectoryFilter) (customerDirectorySourceResult, error) {
	mode, err := s.customerDirectoryArchiveMode(ctx, filter.TenantID, filter.CorpID)
	if err != nil {
		return customerDirectorySourceResult{}, err
	}
	archiveSource, available := effectiveArchiveSource(mode, "")
	if !available {
		return customerDirectorySourceResult{limitation: dashboard.WorkMessageCustomerLimitation{Key: "archiveUnavailable", Reason: "当前企业没有可用的会话存档数据"}}, nil
	}
	state, err := s.archiveSourceRegistryState(ctx)
	if err != nil {
		return customerDirectorySourceResult{}, err
	}
	focusAvailable, err := s.tableExists(ctx, "mochat_go_work_message_focus")
	if err != nil {
		return customerDirectorySourceResult{}, err
	}
	if !focusAvailable {
		return customerDirectorySourceResult{limitation: dashboard.WorkMessageCustomerLimitation{Key: "focusUnavailable", Reason: "会话关注表不可用，无法安全计算关注客户"}}, nil
	}
	archiveFilter := dashboard.WorkMessageUserFilter{
		CorpID: filter.CorpID, AllowAllEmployees: true, ToUserType: -1, ArchiveSource: archiveSource,
		RestrictEmployeeIDs: filter.RestrictEmployeeIDs, EmployeeIDs: append([]int(nil), filter.EmployeeIDs...),
	}
	archiveSQL, archiveArgs, ok := workMessageFilteredUnionSQLWithArchiveSourceState(archiveFilter, state)
	if !ok {
		return customerDirectorySourceResult{limitation: dashboard.WorkMessageCustomerLimitation{Key: "archiveUnavailable", Reason: "当前企业没有可用的会话存档数据"}}, nil
	}

	relationWhere := ""
	relationArgs := []any{filter.CorpID}
	focusWhere := ""
	focusArgs := []any{filter.TenantID, filter.CorpID, filter.UserID}
	if filter.RestrictEmployeeIDs {
		ids := uniquePositiveInts(filter.EmployeeIDs)
		relationWhere = " AND employee_id IN (" + placeholders(len(ids)) + ")"
		relationArgs = append(relationArgs, intsToAny(ids)...)
		focusWhere = " AND work_employee_id IN (" + placeholders(len(ids)) + ")"
		focusArgs = append(focusArgs, intsToAny(ids)...)
	}
	baseSQL := `SELECT candidate.customer_id,
		COALESCE(NULLIF(contact.name,''), NULLIF(candidate.archive_name,''), '') AS name,
		COALESCE(contact.avatar,'') AS avatar,
		COALESCE(contact.wx_external_userid,'') AS external_userid,
		CASE WHEN contact.id IS NULL THEN 'missing' WHEN contact.deleted_at IS NOT NULL THEN 'deleted' ELSE 'available' END AS profile_status,
		COALESCE(relation.active_count,0) AS active_relation_count,
		COALESCE(relation.lost_count,0) AS lost_relation_count,
		candidate.direct_count, candidate.group_count, COALESCE(focus.focused_count,0) AS focused_conversation_count,
		candidate.last_conversation_at
	FROM (
		SELECT source.customer_id, MAX(source.archive_name) AS archive_name,
			COUNT(DISTINCT source.direct_key) AS direct_count, COUNT(DISTINCT source.group_key) AS group_count,
			MAX(source.last_at) AS last_conversation_at
		FROM (
			SELECT wm.to_user_id AS customer_id, MAX(wm.target_name) AS archive_name,
				CONCAT(wm.work_employee_id, ':1:', wm.to_user_id) AS direct_key, NULL AS group_key, MAX(wm.msg_data_time) AS last_at
			FROM (` + archiveSQL + `) wm
			WHERE wm.to_user_type=1
			GROUP BY wm.work_employee_id, wm.to_user_id
			UNION ALL
			SELECT membership.contact_id, MAX(wm.target_name), NULL,
				CONCAT(wm.work_employee_id, ':2:', wm.to_user_id), MAX(wm.msg_data_time)
			FROM mc_work_contact_room membership
			JOIN mc_work_room room ON room.id=membership.room_id AND room.corp_id=? AND room.deleted_at IS NULL
			JOIN (` + archiveSQL + `) wm ON wm.to_user_type=2 AND wm.to_user_id=membership.room_id
			WHERE membership.contact_id>0 AND membership.deleted_at IS NULL
			GROUP BY membership.contact_id, wm.work_employee_id, wm.to_user_id
		) source
		GROUP BY source.customer_id
	) candidate
	LEFT JOIN mc_work_contact contact ON contact.id=candidate.customer_id AND contact.corp_id=?
	LEFT JOIN (
		SELECT contact_id,
			COUNT(DISTINCT CASE WHEN status=1 AND deleted_at IS NULL THEN employee_id END) AS active_count,
			COUNT(DISTINCT CASE WHEN status IN (2,3) THEN employee_id END) AS lost_count
		FROM mc_work_contact_employee
		WHERE corp_id=?` + relationWhere + `
		GROUP BY contact_id
	) relation ON relation.contact_id=candidate.customer_id
	LEFT JOIN (
		SELECT to_user_id AS contact_id, COUNT(*) AS focused_count
		FROM mochat_go_work_message_focus
		WHERE tenant_id=? AND corp_id=? AND user_id=? AND to_user_type=1` + focusWhere + `
		GROUP BY to_user_id
	) focus ON focus.contact_id=candidate.customer_id`
	baseArgs := append([]any{}, archiveArgs...)
	baseArgs = append(baseArgs, filter.CorpID)
	baseArgs = append(baseArgs, archiveArgs...)
	baseArgs = append(baseArgs, filter.CorpID)
	baseArgs = append(baseArgs, relationArgs...)
	baseArgs = append(baseArgs, focusArgs...)
	return customerDirectorySourceResult{sql: baseSQL, args: baseArgs, available: true}, nil
}

// customerDirectoryArchiveMode is deliberately independent from
// workMessageArchiveMode: older installations can lack the simulated-archive
// tables while still having live chat archive enabled.
func (s *MySQLStore) customerDirectoryArchiveMode(ctx context.Context, tenantID, corpID int) (workMessageArchiveMode, error) {
	var chatStatus int
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(corp.chat_status),0)
		FROM mc_corp corp
		WHERE corp.id=? AND (?=0 OR corp.tenant_id=?) AND corp.deleted_at IS NULL
	`, corpID, tenantID, tenantID).Scan(&chatStatus)
	if err != nil {
		return workMessageArchiveUnavailable, err
	}
	if chatStatus == 1 {
		return workMessageArchiveReal, nil
	}
	state, err := s.archiveSourceRegistryState(ctx)
	if err != nil {
		return workMessageArchiveUnavailable, err
	}
	if !state.legacySimulation {
		return workMessageArchiveUnavailable, nil
	}
	var simulationAvailable bool
	err = s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM mochat_go_archive_simulation_batches batch
			WHERE batch.corp_id=? AND batch.status='complete' AND batch.message_count>0
		)
	`, corpID).Scan(&simulationAvailable)
	if err != nil {
		return workMessageArchiveUnavailable, err
	}
	if simulationAvailable {
		return workMessageArchiveSimulation, nil
	}
	return workMessageArchiveUnavailable, nil
}

func (s *MySQLStore) customerDirectoryCounts(ctx context.Context, baseSQL string, baseArgs []any, _ dashboard.WorkMessageCustomerDirectoryFilter) (dashboard.WorkMessageCustomerCounts, error) {
	var counts dashboard.WorkMessageCustomerCounts
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),
		COALESCE(SUM(focused_conversation_count > 0),0),
		COALESCE(SUM(active_relation_count > 0),0),
		COALESCE(SUM(active_relation_count = 0 AND lost_relation_count > 0),0)
		FROM (`+baseSQL+`) customer_directory`, baseArgs...).Scan(&counts.All, &counts.Focused, &counts.Active, &counts.Lost)
	return counts, err
}

func countDerivedRows(ctx context.Context, db *sql.DB, baseSQL string, baseArgs []any, where string, whereArgs []any) (int, error) {
	args := append(append([]any{}, baseArgs...), whereArgs...)
	var total int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+baseSQL+`) customer_directory WHERE `+where, args...).Scan(&total)
	return total, err
}

func (s *MySQLStore) customerDirectoryPage(ctx context.Context, baseSQL string, baseArgs []any, where string, whereArgs []any, page, pageSize int) ([]dashboard.WorkMessageCustomerDirectoryItem, error) {
	args := append(append([]any{}, baseArgs...), whereArgs...)
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT customer_id, name, avatar, profile_status,
		active_relation_count, lost_relation_count, direct_count, group_count, focused_conversation_count, last_conversation_at
		FROM (`+baseSQL+`) customer_directory WHERE `+where+`
		ORDER BY last_conversation_at DESC, customer_id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []dashboard.WorkMessageCustomerDirectoryItem{}
	for rows.Next() {
		var item dashboard.WorkMessageCustomerDirectoryItem
		var lastAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Name, &item.Avatar, &item.ProfileStatus,
			&item.ActiveRelationCount, &item.LostRelationCount, &item.DirectConversationCount,
			&item.GroupConversationCount, &item.FocusedConversationCount, &lastAt); err != nil {
			return nil, err
		}
		item.LastConversationAt = formatTime(lastAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func emptyCustomerDirectory(filter dashboard.WorkMessageCustomerDirectoryFilter) dashboard.WorkMessageCustomerDirectoryPage {
	return dashboard.WorkMessageCustomerDirectoryPage{Customers: []dashboard.WorkMessageCustomerDirectoryItem{}, Limitations: []dashboard.WorkMessageCustomerLimitation{}, Page: positivePage(filter.Page), PageSize: customerDirectoryPageSize, Capabilities: dashboard.WorkMessageCustomerCapabilities()}
}

func unavailableCustomerDirectory(filter dashboard.WorkMessageCustomerDirectoryFilter, reason string) dashboard.WorkMessageCustomerDirectoryPage {
	return unavailableCustomerDirectoryWithLimitation(filter, dashboard.WorkMessageCustomerLimitation{Key: "archiveUnavailable", Reason: reason})
}

func unavailableCustomerDirectoryWithLimitation(filter dashboard.WorkMessageCustomerDirectoryFilter, limitation dashboard.WorkMessageCustomerLimitation) dashboard.WorkMessageCustomerDirectoryPage {
	page := emptyCustomerDirectory(filter)
	page.Limitations = append(page.Limitations, limitation)
	return page
}

func customerDirectoryLimitations(items []dashboard.WorkMessageCustomerDirectoryItem) []dashboard.WorkMessageCustomerLimitation {
	missing, deleted := false, false
	for _, item := range items {
		switch item.ProfileStatus {
		case "missing":
			missing = true
		case "deleted":
			deleted = true
		}
	}
	limitations := []dashboard.WorkMessageCustomerLimitation{}
	if missing {
		limitations = append(limitations, dashboard.WorkMessageCustomerLimitation{Key: "profileMissing", Reason: "部分客户资料缺失，名称仅来自会话归档"})
	}
	if deleted {
		limitations = append(limitations, dashboard.WorkMessageCustomerLimitation{Key: "profileDeleted", Reason: "部分客户资料已删除，名称仅来自会话归档"})
	}
	return limitations
}
