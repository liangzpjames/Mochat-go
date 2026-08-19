package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

const customerDirectoryPageSize = 50
const customerConversationPageSize = 20

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

// WorkMessageCustomerConversations lists archived direct or room conversations
// associated with one customer.  Count and page always wrap the same derived
// conversation source so the result cannot drift between the two queries.
func (s *MySQLStore) WorkMessageCustomerConversations(ctx context.Context, filter dashboard.WorkMessageCustomerConversationFilter) (dashboard.WorkMessageCustomerConversationPage, error) {
	filter.Page, filter.PageSize = positivePage(filter.Page), customerConversationPageSize
	profile, visible, err := s.customerWorkspaceProfile(ctx, filter.CorpID, filter.CustomerID, filter.RestrictEmployeeIDs, filter.EmployeeIDs)
	if err != nil {
		return dashboard.WorkMessageCustomerConversationPage{}, err
	}
	if !visible {
		return dashboard.WorkMessageCustomerConversationPage{}, dashboard.ErrWorkMessageConversationNotFound
	}
	if filter.RestrictEmployeeIDs && len(uniquePositiveInts(filter.EmployeeIDs)) == 0 {
		return s.emptyCustomerConversationPageWithCapabilities(ctx, profile, filter, true)
	}
	baseSQL, args, available, err := s.customerConversationSource(ctx, filter)
	if err != nil {
		return dashboard.WorkMessageCustomerConversationPage{}, err
	}
	if !available {
		return s.emptyCustomerConversationPageWithCapabilities(ctx, profile, filter, false)
	}
	total, err := countCustomerConversations(ctx, s.db, baseSQL, args)
	if err != nil {
		return dashboard.WorkMessageCustomerConversationPage{}, err
	}
	list, err := s.customerConversationPage(ctx, baseSQL, args, filter.Page, customerConversationPageSize)
	if err != nil {
		return dashboard.WorkMessageCustomerConversationPage{}, err
	}
	capabilities, err := s.decorateCustomerConversationFlags(ctx, filter, list)
	if err != nil {
		return dashboard.WorkMessageCustomerConversationPage{}, err
	}
	return dashboard.WorkMessageCustomerConversationPage{
		Customer: profile, Mode: filter.Mode, List: list, Total: total, Page: filter.Page, PageSize: customerConversationPageSize,
		Capabilities: capabilities,
	}, nil
}

// WorkMessageCustomerDetail validates that the requested conversation belongs
// to the customer, then delegates message shaping to StaffDetail so the staff
// and customer workspaces use exactly the same pagination and direction rules.
func (s *MySQLStore) WorkMessageCustomerDetail(ctx context.Context, filter dashboard.WorkMessageCustomerDetailFilter) (dashboard.WorkMessageCustomerDetail, error) {
	if filter.RestrictEmployeeIDs && !containsPositiveInt(uniquePositiveInts(filter.EmployeeIDs), filter.EmployeeID) {
		return dashboard.WorkMessageCustomerDetail{}, dashboard.ErrWorkMessageConversationNotFound
	}

	roomMembershipExists := false
	if filter.ToUserType == 2 {
		var err error
		roomMembershipExists, err = s.customerRoomMembershipExists(ctx, filter.CorpID, filter.CustomerID, filter.ToUserID)
		if err != nil {
			return dashboard.WorkMessageCustomerDetail{}, err
		}
	}
	if err := validateCustomerConversationAssociation(filter.CustomerID, filter.ToUserType, filter.ToUserID, roomMembershipExists); err != nil {
		return dashboard.WorkMessageCustomerDetail{}, err
	}

	profile, visible, err := s.customerWorkspaceProfile(ctx, filter.CorpID, filter.CustomerID, filter.RestrictEmployeeIDs, filter.EmployeeIDs)
	if err != nil {
		return dashboard.WorkMessageCustomerDetail{}, err
	}
	if !visible {
		return dashboard.WorkMessageCustomerDetail{}, dashboard.ErrWorkMessageConversationNotFound
	}

	detail, err := s.StaffDetail(ctx, dashboard.WorkMessageStaffDetailFilter{
		TenantID: filter.TenantID, CorpID: filter.CorpID, UserID: filter.UserID,
		EmployeeID: filter.EmployeeID, ToUserType: filter.ToUserType, ToUserID: filter.ToUserID,
		Keyword: filter.Keyword, MessageTypes: filter.MessageTypes, Date: filter.Date,
		PageSize: 50, Before: filter.Before,
	})
	if err != nil {
		return dashboard.WorkMessageCustomerDetail{}, err
	}
	return customerDetailFromStaff(profile, detail), nil
}

func validateCustomerConversationAssociation(customerID, toUserType, toUserID int, roomMembershipExists bool) error {
	switch toUserType {
	case 1:
		if toUserID == customerID {
			return nil
		}
	case 2:
		if roomMembershipExists {
			return nil
		}
	}
	return dashboard.ErrWorkMessageConversationNotFound
}

// customerRoomMembershipExists deliberately does not filter membership.deleted_at:
// an external contact who has left a group may still have archived messages.
func (s *MySQLStore) customerRoomMembershipExists(ctx context.Context, corpID, customerID, roomID int) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM mc_work_contact_room membership
		JOIN mc_work_room room ON room.id=membership.room_id AND room.corp_id=? AND room.deleted_at IS NULL
		JOIN mc_work_contact contact_scope ON contact_scope.id=membership.contact_id AND contact_scope.corp_id=?
		WHERE membership.contact_id=? AND membership.room_id=?
	)`, corpID, corpID, customerID, roomID).Scan(&exists)
	return exists, err
}

func customerDetailFromStaff(profile dashboard.WorkMessageCustomerProfile, detail dashboard.WorkMessageStaffDetail) dashboard.WorkMessageCustomerDetail {
	return dashboard.WorkMessageCustomerDetail{
		WorkMessageStaffDetail: detail,
		CustomerID:             profile.ID,
		CustomerName:           profile.Name,
	}
}

// customerWorkspaceProfile permits an archived-only customer (whose profile
// has since been removed), while still rejecting scoped callers that have no
// employee relationship and no same-corp room membership to the customer.
func (s *MySQLStore) customerWorkspaceProfile(ctx context.Context, corpID, customerID int, restrictEmployeeIDs bool, employeeIDs []int) (dashboard.WorkMessageCustomerProfile, bool, error) {
	profile := dashboard.WorkMessageCustomerProfile{ID: customerID, ProfileStatus: "missing"}
	if corpID <= 0 || customerID <= 0 {
		return profile, false, nil
	}
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(name,''), COALESCE(avatar,''),
			CASE WHEN deleted_at IS NULL THEN 'available' ELSE 'deleted' END
		FROM mc_work_contact WHERE id=? AND corp_id=? LIMIT 1
	`, customerID, corpID).Scan(&profile.Name, &profile.Avatar, &profile.ProfileStatus)
	if err != nil && err != sql.ErrNoRows {
		return dashboard.WorkMessageCustomerProfile{}, false, err
	}
	if !restrictEmployeeIDs {
		return profile, true, nil
	}
	ids := uniquePositiveInts(employeeIDs)
	if len(ids) == 0 {
		return profile, false, nil
	}
	var visible bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM mc_work_contact_employee relation
		WHERE relation.corp_id=? AND relation.contact_id=? AND relation.employee_id IN (`+placeholders(len(ids))+`)
	)`, append([]any{corpID, customerID}, intsToAny(ids)...)...).Scan(&visible); err != nil {
		return dashboard.WorkMessageCustomerProfile{}, false, err
	}
	if visible {
		return profile, true, nil
	}
	err = s.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM mc_work_contact_room membership
		JOIN mc_work_room room ON room.id=membership.room_id AND room.corp_id=? AND room.deleted_at IS NULL
		JOIN mc_work_contact contact_scope ON contact_scope.id=membership.contact_id AND contact_scope.corp_id=?
		WHERE membership.contact_id=? AND membership.employee_id IN (`+placeholders(len(ids))+`)
	)`, append([]any{corpID, corpID, customerID}, intsToAny(ids)...)...).Scan(&visible)
	return profile, visible, err
}

func (s *MySQLStore) customerConversationSource(ctx context.Context, filter dashboard.WorkMessageCustomerConversationFilter) (string, []any, bool, error) {
	mode, err := s.customerDirectoryArchiveMode(ctx, filter.TenantID, filter.CorpID)
	if err != nil {
		return "", nil, false, err
	}
	archiveSource, available := effectiveArchiveSource(mode, "")
	if !available {
		return "", nil, false, nil
	}
	state, err := s.archiveSourceRegistryState(ctx)
	if err != nil {
		return "", nil, false, err
	}
	archiveFilter := dashboard.WorkMessageUserFilter{
		CorpID: filter.CorpID, AllowAllEmployees: true, ToUserType: -1, ArchiveSource: archiveSource,
		RestrictEmployeeIDs: filter.RestrictEmployeeIDs, EmployeeIDs: append([]int(nil), filter.EmployeeIDs...),
	}
	archiveSQL, archiveArgs, ok := workMessageFilteredUnionSQLWithArchiveSourceState(archiveFilter, state)
	if !ok {
		return "", nil, false, nil
	}
	baseSQL, args, err := customerConversationBaseSQLWithSource(filter, archiveSQL, archiveArgs, archiveSource)
	if err != nil {
		return "", nil, false, err
	}
	return baseSQL, args, true, nil
}

func customerConversationBaseSQL(filter dashboard.WorkMessageCustomerConversationFilter, archiveSQL string, archiveArgs []any) (string, []any, error) {
	return customerConversationBaseSQLWithSource(filter, archiveSQL, archiveArgs, "")
}

func customerConversationBaseSQLWithSource(filter dashboard.WorkMessageCustomerConversationFilter, archiveSQL string, archiveArgs []any, archiveSource string) (string, []any, error) {
	if filter.CorpID < 0 || filter.CustomerID <= 0 || strings.TrimSpace(archiveSQL) == "" {
		return "", nil, fmt.Errorf("invalid customer conversation source")
	}
	projections := `
		grouped.conversation_id,
		latest.work_employee_id,
		COALESCE(NULLIF(employee.name,''), NULLIF(latest.employee_name,''), '') AS employee_name,
		COALESCE(NULLIF(employee.avatar,''), NULLIF(latest.employee_avatar,''), '') AS employee_avatar,
		latest.to_user_type, latest.to_user_id,
		CASE WHEN latest.to_user_type=1 THEN COALESCE(NULLIF(contact.name,''), NULLIF(latest.target_name,''), '') ELSE COALESCE(NULLIF(room.name,''), NULLIF(latest.target_name,''), '') END AS target_name,
		CASE WHEN latest.to_user_type=1 THEN COALESCE(NULLIF(contact.avatar,''), NULLIF(latest.target_avatar,''), '') ELSE COALESCE(NULLIF(room.avatar,''), NULLIF(latest.target_avatar,''), '') END AS target_avatar,
		COALESCE(latest.content_text,'') AS content_text, COALESCE(latest.msg_type,100) AS msg_type,
		CASE WHEN COALESCE(latest.is_current_user,0)=1 THEN 'outbound' ELSE 'inbound' END AS direction,
		latest.msg_data_time AS last_at, grouped.message_total,
		'ARCHIVE_SOURCE' AS archive_source,
		CASE WHEN NULLIF(latest.msgid,'') IS NOT NULL THEN CONCAT('msg:',latest.msgid) WHEN latest.seq>0 THEN CONCAT('seq:',latest.seq) ELSE CONCAT('table:',latest.table_index,':',latest.id) END AS archive_source_id,
		CASE WHEN latest.to_user_type=1 THEN COALESCE(relation.relation_status,'none') ELSE '' END AS relation_status,
		CASE WHEN latest.to_user_type=2 THEN CASE WHEN latest.current_member=1 THEN 'active' ELSE 'left' END ELSE '' END AS membership_status`
	joins := `
		LEFT JOIN mc_work_employee employee ON employee.id=latest.work_employee_id AND employee.corp_id=? AND employee.deleted_at IS NULL
		LEFT JOIN mc_work_contact contact ON contact.id=latest.to_user_id AND contact.corp_id=?
		LEFT JOIN mc_work_room room ON room.id=latest.to_user_id AND room.corp_id=? AND room.deleted_at IS NULL
		LEFT JOIN (
			SELECT contact_id, employee_id,
				CASE WHEN MAX(CASE WHEN status=1 AND deleted_at IS NULL THEN 1 ELSE 0 END)=1 THEN 'active'
					WHEN MAX(CASE WHEN status IN (2,3) THEN 1 ELSE 0 END)=1 THEN 'lost' ELSE 'none' END AS relation_status
			FROM mc_work_contact_employee WHERE corp_id=? GROUP BY contact_id, employee_id
		) relation ON relation.contact_id=latest.to_user_id AND relation.employee_id=latest.work_employee_id`
	archiveSource = strings.ReplaceAll(archiveSource, "'", "")
	projections = strings.ReplaceAll(projections, "ARCHIVE_SOURCE", archiveSource)
	joinArgs := []any{filter.CorpID, filter.CorpID, filter.CorpID, filter.CorpID}
	switch filter.Mode {
	case dashboard.WorkMessageCustomerConversationModeDirect:
		baseSQL := `SELECT ` + projections + `
			FROM (
				SELECT wm.*, ROW_NUMBER() OVER (PARTITION BY wm.work_employee_id, wm.to_user_id ORDER BY wm.msg_data_time DESC, wm.seq DESC, wm.table_index DESC, wm.id DESC) AS rn
				FROM (` + archiveSQL + `) wm
				WHERE wm.to_user_type = 1 AND wm.to_user_id = ?
			) latest
			JOIN (
				SELECT wm.work_employee_id, wm.to_user_id, CONCAT(wm.work_employee_id, ':1:', wm.to_user_id) AS conversation_id, COUNT(*) AS message_total
				FROM (` + archiveSQL + `) wm
				WHERE wm.to_user_type = 1 AND wm.to_user_id = ?
				GROUP BY wm.work_employee_id, wm.to_user_id
			) grouped ON grouped.work_employee_id=latest.work_employee_id AND grouped.to_user_id=latest.to_user_id
			` + joins + `
			WHERE latest.rn=1`
		args := append([]any{}, archiveArgs...)
		args = append(args, filter.CustomerID)
		args = append(args, archiveArgs...)
		args = append(args, filter.CustomerID)
		args = append(args, joinArgs...)
		return baseSQL, args, nil
	case dashboard.WorkMessageCustomerConversationModeGroup:
		customerRooms := `(SELECT DISTINCT membership.room_id,
			CASE WHEN EXISTS (SELECT 1 FROM mc_work_contact_room membership_current WHERE membership_current.room_id=membership.room_id AND membership_current.contact_id=membership.contact_id AND membership_current.deleted_at IS NULL) THEN 1 ELSE 0 END AS current_member
			FROM mc_work_contact_room membership
			JOIN mc_work_room room ON room.id=membership.room_id AND room.corp_id=? AND room.deleted_at IS NULL
			JOIN mc_work_contact contact_scope ON contact_scope.id=membership.contact_id AND contact_scope.corp_id=?
			WHERE membership.contact_id = ?)`
		baseSQL := `SELECT ` + projections + `
			FROM (
				SELECT wm.*, customer_room.current_member,
					ROW_NUMBER() OVER (PARTITION BY wm.work_employee_id, wm.to_user_id ORDER BY wm.msg_data_time DESC, wm.seq DESC, wm.table_index DESC, wm.id DESC) AS rn
				FROM (` + archiveSQL + `) wm
				JOIN ` + customerRooms + ` customer_room ON customer_room.room_id=wm.to_user_id
				WHERE wm.to_user_type = 2
			) latest
			JOIN (
				SELECT wm.work_employee_id, wm.to_user_id, CONCAT(wm.work_employee_id, ':2:', wm.to_user_id) AS conversation_id, COUNT(*) AS message_total
				FROM (` + archiveSQL + `) wm
				JOIN ` + customerRooms + ` customer_room ON customer_room.room_id=wm.to_user_id
				WHERE wm.to_user_type = 2
				GROUP BY wm.work_employee_id, wm.to_user_id
			) grouped ON grouped.work_employee_id=latest.work_employee_id AND grouped.to_user_id=latest.to_user_id
			` + joins + `
			WHERE latest.rn=1`
		args := append([]any{}, archiveArgs...)
		args = append(args, filter.CorpID, filter.CorpID, filter.CustomerID)
		args = append(args, archiveArgs...)
		args = append(args, filter.CorpID, filter.CorpID, filter.CustomerID)
		args = append(args, joinArgs...)
		return baseSQL, args, nil
	default:
		return "", nil, fmt.Errorf("invalid customer conversation mode %q", filter.Mode)
	}
}

func countCustomerConversations(ctx context.Context, db *sql.DB, baseSQL string, args []any) (int, error) {
	var total int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+baseSQL+`) customer_conversations`, args...).Scan(&total)
	return total, err
}

func (s *MySQLStore) customerConversationPage(ctx context.Context, baseSQL string, args []any, page, pageSize int) ([]dashboard.WorkMessageCustomerConversation, error) {
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, pageSize, (positivePage(page)-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT conversation_id, work_employee_id, employee_name, employee_avatar,
		to_user_type, to_user_id, target_name, target_avatar, content_text, msg_type, direction, last_at, message_total,
		archive_source, archive_source_id, relation_status, membership_status
		FROM (`+baseSQL+`) customer_conversations
		ORDER BY last_at DESC, conversation_id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []dashboard.WorkMessageCustomerConversation{}
	for rows.Next() {
		var item dashboard.WorkMessageCustomerConversation
		var lastAt sql.NullTime
		var toUserType int
		if err := rows.Scan(&item.ConversationID, &item.EmployeeID, &item.EmployeeName, &item.EmployeeAvatar,
			&toUserType, &item.TargetID, &item.TargetName, &item.TargetAvatar, &item.LastMessage, &item.LastMessageType,
			&item.LastDirection, &lastAt, &item.MessageTotal, &item.ArchiveSource, &item.ArchiveSourceID,
			&item.RelationStatus, &item.MembershipStatus); err != nil {
			return nil, err
		}
		item.SentAt = formatTime(lastAt)
		item.ID = item.ArchiveSourceID
		item.TargetType = workMessageTargetType(toUserType)
		list = append(list, item)
	}
	return list, rows.Err()
}

func (s *MySQLStore) decorateCustomerConversationFlags(ctx context.Context, filter dashboard.WorkMessageCustomerConversationFilter, list []dashboard.WorkMessageCustomerConversation) ([]dashboard.WorkMessageCapability, error) {
	capabilities := append([]dashboard.WorkMessageCapability{}, dashboard.WorkMessageCustomerCapabilities()...)
	capabilities = append(capabilities, dashboard.WorkMessageCapability{Key: "archive", Available: true})
	flags := []struct {
		table, key, reason string
		apply              func(*dashboard.WorkMessageCustomerConversation, int)
	}{
		{"mochat_go_work_message_focus", "focus", "会话关注表不可用", func(item *dashboard.WorkMessageCustomerConversation, count int) { item.Focused = count > 0 }},
		{"mochat_go_risk_records", "riskRecords", "系统没有风险记录数据表", func(item *dashboard.WorkMessageCustomerConversation, count int) { item.RiskCount = count }},
		{"mochat_go_timeout_records", "timeoutRecords", "系统没有超时记录数据表", func(item *dashboard.WorkMessageCustomerConversation, count int) { item.TimeoutCount = count }},
	}
	available := make([]bool, len(flags))
	for index, flag := range flags {
		var err error
		available[index], err = s.tableExists(ctx, flag.table)
		if err != nil {
			return nil, err
		}
		if available[index] {
			capabilities = append(capabilities, dashboard.WorkMessageCapability{Key: flag.key, Available: true})
		} else {
			capabilities = append(capabilities, dashboard.WorkMessageCapability{Key: flag.key, Available: false, Reason: flag.reason})
		}
	}
	if len(list) == 0 {
		return capabilities, nil
	}
	ids := make([]string, 0, len(list))
	byID := make(map[string]*dashboard.WorkMessageCustomerConversation, len(list))
	for index := range list {
		ids = append(ids, list[index].ConversationID)
		byID[list[index].ConversationID] = &list[index]
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")

	for index, flag := range flags {
		if !available[index] {
			continue
		}
		if flag.key == "focus" {
			queryArgs := []any{filter.TenantID, filter.CorpID, filter.UserID}
			for _, id := range ids {
				queryArgs = append(queryArgs, id)
			}
			rows, err := s.db.QueryContext(ctx, `SELECT CONCAT(work_employee_id, ':', to_user_type, ':', to_user_id)
			FROM mochat_go_work_message_focus WHERE tenant_id=? AND corp_id=? AND user_id=?
			AND CONCAT(work_employee_id, ':', to_user_type, ':', to_user_id) IN (`+marks+`)`, queryArgs...)
			if err != nil {
				return nil, err
			}
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					rows.Close()
					return nil, err
				}
				if item := byID[id]; item != nil {
					flag.apply(item, 1)
				}
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return nil, err
			}
			rows.Close()
			continue
		}
		queryArgs := []any{filter.TenantID, filter.CorpID}
		for _, id := range ids {
			queryArgs = append(queryArgs, id)
		}
		rows, err := s.db.QueryContext(ctx, `SELECT conversation_id, COUNT(*) FROM `+flag.table+`
			WHERE tenant_id=? AND corp_id=? AND conversation_id IN (`+marks+`) GROUP BY conversation_id`, queryArgs...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var count int
			if err := rows.Scan(&id, &count); err != nil {
				rows.Close()
				return nil, err
			}
			if item := byID[id]; item != nil {
				flag.apply(item, count)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return capabilities, nil
}

func (s *MySQLStore) emptyCustomerConversationPageWithCapabilities(ctx context.Context, profile dashboard.WorkMessageCustomerProfile, filter dashboard.WorkMessageCustomerConversationFilter, archiveAvailable bool) (dashboard.WorkMessageCustomerConversationPage, error) {
	page := emptyCustomerConversationPage(profile, filter)
	capabilities, err := s.decorateCustomerConversationFlags(ctx, filter, nil)
	if err != nil {
		return dashboard.WorkMessageCustomerConversationPage{}, err
	}
	for index := range capabilities {
		if capabilities[index].Key == "archive" && !archiveAvailable {
			capabilities[index] = dashboard.WorkMessageCapability{Key: "archive", Available: false, Reason: "当前企业没有可用的会话存档数据"}
		}
	}
	page.Capabilities = capabilities
	return page, nil
}

func emptyCustomerConversationPage(profile dashboard.WorkMessageCustomerProfile, filter dashboard.WorkMessageCustomerConversationFilter) dashboard.WorkMessageCustomerConversationPage {
	capabilities := append([]dashboard.WorkMessageCapability{}, dashboard.WorkMessageCustomerCapabilities()...)
	capabilities = append(capabilities, dashboard.WorkMessageCapability{Key: "archive", Available: false, Reason: "当前企业没有可用的会话存档数据"})
	return dashboard.WorkMessageCustomerConversationPage{
		Customer: profile, Mode: filter.Mode, List: []dashboard.WorkMessageCustomerConversation{}, Page: positivePage(filter.Page), PageSize: customerConversationPageSize,
		Capabilities: capabilities,
	}
}
