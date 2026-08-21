package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

const roomDirectoryPageSize = 50

func (s *MySQLStore) workMessageRoomArchiveSource(ctx context.Context, filter dashboard.WorkMessageRoomDirectoryFilter) (string, []any, string, bool, error) {
	mode, err := s.workMessageArchiveMode(ctx, filter.TenantID, filter.CorpID)
	if err != nil {
		return "", nil, "", false, err
	}
	source, available := effectiveArchiveSource(mode, "")
	if !available {
		return "", nil, "", false, nil
	}
	state, err := s.archiveSourceRegistryState(ctx)
	if err != nil {
		return "", nil, "", false, err
	}
	archiveFilter := dashboard.WorkMessageUserFilter{
		TenantID: filter.TenantID, UserID: filter.UserID, CorpID: filter.CorpID,
		AllowAllEmployees: true, ToUserType: 2, ArchiveSource: source,
		RestrictEmployeeIDs: filter.RestrictEmployeeIDs, EmployeeIDs: append([]int(nil), filter.EmployeeIDs...),
	}
	sqlText, args, ok := workMessageFilteredUnionSQLWithArchiveSourceState(archiveFilter, state)
	return sqlText, args, source, ok, nil
}

func workMessageRoomDirectoryBaseSQL() string {
	return roomDirectoryAggregateSQL(`SELECT to_user_id, to_user_type, msgid, seq, content_text, msg_data_time, corp_id, 0 AS work_employee_id
		FROM mc_work_message_1`, "room.deleted_at IS NULL")
}

func roomDirectoryAggregateSQL(sourceSQL, where string) string {
	return `
		SELECT room.id, COALESCE(room.wx_chat_id,''), COALESCE(room.name,''),
		       COALESCE(room.owner_id,0), COALESCE(owner.name,''),
		       COUNT(DISTINCT membership.id),
		       COUNT(DISTINCT CASE WHEN membership.type=1 THEN membership.id END),
		       COUNT(DISTINCT CASE WHEN membership.type=2 THEN membership.id END),
		       COUNT(*),
		       SUBSTRING_INDEX(GROUP_CONCAT(COALESCE(wm.content_text,'') ORDER BY wm.msg_data_time DESC, wm.seq DESC, wm.id DESC SEPARATOR '|||MOCHAT|||'),'|||MOCHAT|||',1),
		       MAX(wm.msg_data_time),
		       CASE WHEN room.deleted_at IS NULL THEN 0 ELSE 1 END
		FROM (` + sourceSQL + `) wm
		INNER JOIN mc_work_room room ON room.id=wm.to_user_id AND room.corp_id=wm.corp_id
		LEFT JOIN mc_work_employee owner ON owner.id=room.owner_id AND owner.corp_id=room.corp_id AND owner.deleted_at IS NULL
		LEFT JOIN mc_work_contact_room membership ON membership.room_id=room.id AND membership.deleted_at IS NULL
		WHERE wm.to_user_type=2 AND wm.to_user_id>0 AND ` + where + `
		GROUP BY room.id, room.wx_chat_id, room.name, room.owner_id, owner.name, room.deleted_at`
}

func workMessageRoomDirectoryWhere(filter dashboard.WorkMessageRoomDirectoryFilter) (string, []any) {
	where := []string{}
	args := []any{}
	if filter.RoomMode == dashboard.WorkMessageRoomModeDissolved {
		where = append(where, "room.deleted_at IS NOT NULL")
	} else {
		where = append(where, "room.deleted_at IS NULL")
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		where = append(where, `(room.name LIKE ? ESCAPE '\\' OR room.wx_chat_id LIKE ? ESCAPE '\\' OR owner.name LIKE ? ESCAPE '\\')`)
		pattern := workMessageLikePattern(keyword)
		args = append(args, pattern, pattern, pattern)
	}
	if employeeIDs := uniquePositiveInts(filter.EmployeeIDs); len(employeeIDs) > 0 {
		where = append(where, "(room.owner_id IN ("+placeholders(len(employeeIDs)+0)+") OR membership.employee_id IN ("+placeholders(len(employeeIDs))+") OR wm.work_employee_id IN ("+placeholders(len(employeeIDs))+"))")
		args = append(args, intsToAny(employeeIDs)...)
		args = append(args, intsToAny(employeeIDs)...)
		args = append(args, intsToAny(employeeIDs)...)
	}
	if customerIDs := uniquePositiveInts(filter.CustomerIDs); len(customerIDs) > 0 {
		where = append(where, "membership.contact_id IN ("+placeholders(len(customerIDs))+")")
		args = append(args, intsToAny(customerIDs)...)
	}
	if roomGroupIDs := uniquePositiveInts(filter.RoomGroupIDs); len(roomGroupIDs) > 0 {
		where = append(where, "room.room_group_id IN ("+placeholders(len(roomGroupIDs))+")")
		args = append(args, intsToAny(roomGroupIDs)...)
	}
	if len(where) == 0 {
		return "1 = 1", args
	}
	return strings.Join(where, " AND "), args
}

func (s *MySQLStore) RoomDirectory(ctx context.Context, filter dashboard.WorkMessageRoomDirectoryFilter) (dashboard.WorkMessageRoomDirectoryPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PageSize = roomDirectoryPageSize
	if filter.RestrictEmployeeIDs && len(uniquePositiveInts(filter.EmployeeIDs)) == 0 {
		return s.emptyWorkMessageRoomDirectory(ctx, filter, true)
	}
	sourceSQL, sourceArgs, source, available, err := s.workMessageRoomArchiveSource(ctx, filter)
	if err != nil {
		return dashboard.WorkMessageRoomDirectoryPage{}, err
	}
	if !available {
		return s.emptyWorkMessageRoomDirectory(ctx, filter, false)
	}
	where, whereArgs := workMessageRoomDirectoryWhere(filter)
	baseSQL := roomDirectoryAggregateSQL(sourceSQL, where)
	baseArgs := append(append([]any{}, sourceArgs...), whereArgs...)
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+baseSQL+`) room_directory`, baseArgs...).Scan(&total); err != nil {
		return dashboard.WorkMessageRoomDirectoryPage{}, err
	}
	queryArgs := append(append([]any{}, baseArgs...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT id, external_id, name, owner_id, owner_name, member_count,
		employee_count, customer_count, message_count, last_message, last_message_at, dissolved
		FROM (
			SELECT room.id, COALESCE(room.wx_chat_id,'') AS external_id, COALESCE(room.name,'') AS name,
			       COALESCE(room.owner_id,0) AS owner_id, COALESCE(owner.name,'') AS owner_name,
			       COUNT(DISTINCT membership.id) AS member_count,
			       COUNT(DISTINCT CASE WHEN membership.type=1 THEN membership.id END) AS employee_count,
			       COUNT(DISTINCT CASE WHEN membership.type=2 THEN membership.id END) AS customer_count,
			       COUNT(*) AS message_count,
			       SUBSTRING_INDEX(GROUP_CONCAT(COALESCE(wm.content_text,'') ORDER BY wm.msg_data_time DESC, wm.seq DESC, wm.id DESC SEPARATOR '|||MOCHAT|||'),'|||MOCHAT|||',1) AS last_message,
			       MAX(wm.msg_data_time) AS last_message_at,
			       CASE WHEN room.deleted_at IS NULL THEN 0 ELSE 1 END AS dissolved
			FROM (`+sourceSQL+`) wm
			INNER JOIN mc_work_room room ON room.id=wm.to_user_id AND room.corp_id=wm.corp_id
			LEFT JOIN mc_work_employee owner ON owner.id=room.owner_id AND owner.corp_id=room.corp_id AND owner.deleted_at IS NULL
			LEFT JOIN mc_work_contact_room membership ON membership.room_id=room.id AND membership.deleted_at IS NULL
			WHERE wm.to_user_type=2 AND wm.to_user_id>0 AND `+where+`
			GROUP BY room.id, room.wx_chat_id, room.name, room.owner_id, owner.name, room.deleted_at
		) room_directory
		ORDER BY last_message_at DESC, id DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return dashboard.WorkMessageRoomDirectoryPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.WorkMessageRoomDirectoryItem, 0)
	for rows.Next() {
		var item dashboard.WorkMessageRoomDirectoryItem
		var lastAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.ExternalID, &item.Name, &item.OwnerID, &item.OwnerName, &item.MemberCount,
			&item.EmployeeCount, &item.CustomerCount, &item.MessageCount, &item.LastMessage, &lastAt, &item.Dissolved); err != nil {
			return dashboard.WorkMessageRoomDirectoryPage{}, err
		}
		item.LastMessageAt = formatTime(lastAt)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkMessageRoomDirectoryPage{}, err
	}
	capabilities, err := s.decorateWorkMessageRoomFlags(ctx, filter.TenantID, filter.CorpID, filter.UserID, items, source, true)
	if err != nil {
		return dashboard.WorkMessageRoomDirectoryPage{}, err
	}
	return dashboard.WorkMessageRoomDirectoryPage{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize, Capabilities: capabilities}, nil
}

func (s *MySQLStore) emptyWorkMessageRoomDirectory(ctx context.Context, filter dashboard.WorkMessageRoomDirectoryFilter, archiveAvailable bool) (dashboard.WorkMessageRoomDirectoryPage, error) {
	capabilities, err := s.roomCapabilities(ctx, archiveAvailable)
	if err != nil {
		return dashboard.WorkMessageRoomDirectoryPage{}, err
	}
	page := dashboard.WorkMessageRoomDirectoryPage{Items: []dashboard.WorkMessageRoomDirectoryItem{}, Page: positivePage(filter.Page), PageSize: roomDirectoryPageSize, Capabilities: capabilities}
	if !archiveAvailable {
		page.Limitations = []dashboard.WorkMessageStaffLimitation{{Key: "archiveUnavailable", Reason: "当前企业没有可用的会话存档数据"}}
	} else {
		page.Limitations = []dashboard.WorkMessageStaffLimitation{}
	}
	return page, nil
}

func (s *MySQLStore) roomCapabilities(ctx context.Context, archiveAvailable bool) ([]dashboard.WorkMessageCapability, error) {
	capabilities := []dashboard.WorkMessageCapability{{Key: "archive", Available: archiveAvailable}}
	checks := []struct {
		table, key, reason string
	}{
		{"mochat_go_work_message_focus", "focus", "会话关注表不可用"},
		{"mochat_go_risk_records", "riskRecords", "系统没有风险记录数据表"},
		{"mochat_go_timeout_records", "timeoutRecords", "系统没有超时记录数据表"},
	}
	for _, check := range checks {
		available, err := s.tableExists(ctx, check.table)
		if err != nil {
			return nil, err
		}
		capability := dashboard.WorkMessageCapability{Key: check.key, Available: available}
		if !available {
			capability.Reason = check.reason
		}
		capabilities = append(capabilities, capability)
	}
	return capabilities, nil
}

func (s *MySQLStore) workMessageRoomHasVisibleArchive(ctx context.Context, tenantID, corpID, roomID int, employeeIDs []int) (bool, error) {
	sourceSQL, sourceArgs, _, available, err := s.workMessageRoomArchiveSource(ctx, dashboard.WorkMessageRoomDirectoryFilter{
		TenantID: tenantID, CorpID: corpID, RoomMode: dashboard.WorkMessageRoomModeActive,
		RestrictEmployeeIDs: true, EmployeeIDs: employeeIDs,
	})
	if err != nil || !available {
		return false, err
	}
	args := append(append([]any{}, sourceArgs...), roomID)
	var visible bool
	err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM (`+sourceSQL+`) room_messages
		WHERE to_user_type=2 AND to_user_id=? LIMIT 1)`, args...).Scan(&visible)
	return visible, err
}

func capabilityAvailable(capabilities []dashboard.WorkMessageCapability, key string) bool {
	for _, capability := range capabilities {
		if capability.Key == key {
			return capability.Available
		}
	}
	return false
}

func (s *MySQLStore) decorateWorkMessageRoomFlags(ctx context.Context, tenantID, corpID, userID int, items []dashboard.WorkMessageRoomDirectoryItem, source string, archiveAvailable bool) ([]dashboard.WorkMessageCapability, error) {
	capabilities, err := s.roomCapabilities(ctx, archiveAvailable)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return capabilities, nil
	}
	ids := make([]int, 0, len(items))
	for index := range items {
		ids = append(ids, items[index].ID)
	}
	if capabilityAvailable(capabilities, "focus") {
		args := []any{tenantID, corpID, userID}
		args = append(args, intsToAny(ids)...)
		rows, err := s.db.QueryContext(ctx, `SELECT to_user_id FROM mochat_go_work_message_focus
			WHERE tenant_id=? AND corp_id=? AND user_id=? AND to_user_type=2 AND to_user_id IN (`+placeholders(len(ids))+`)`, args...)
		if err != nil {
			return nil, err
		}
		focused := map[int]struct{}{}
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			focused[id] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
		for index := range items {
			_, items[index].Focused = focused[items[index].ID]
		}
	}
	for _, flag := range []struct {
		key   string
		apply func(*dashboard.WorkMessageRoomDirectoryItem, int)
	}{
		{"riskRecords", func(item *dashboard.WorkMessageRoomDirectoryItem, count int) { item.RiskCount = count }},
		{"timeoutRecords", func(item *dashboard.WorkMessageRoomDirectoryItem, count int) { item.TimeoutCount = count }},
	} {
		if !capabilityAvailable(capabilities, flag.key) {
			continue
		}
		counts, err := s.roomFlagCounts(ctx, flag.key, tenantID, corpID, ids)
		if err != nil {
			return nil, err
		}
		for index := range items {
			flag.apply(&items[index], counts[items[index].ID])
		}
	}
	_ = source
	return capabilities, nil
}

func (s *MySQLStore) roomFlagCounts(ctx context.Context, capability string, tenantID, corpID int, ids []int) (map[int]int, error) {
	table := ""
	switch capability {
	case "riskRecords":
		table = "mochat_go_risk_records"
	case "timeoutRecords":
		table = "mochat_go_timeout_records"
	default:
		return nil, fmt.Errorf("invalid room flag capability %s", capability)
	}
	args := []any{tenantID, corpID}
	args = append(args, intsToAny(ids)...)
	rows, err := s.db.QueryContext(ctx, `SELECT CAST(SUBSTRING_INDEX(conversation_id, ':', -1) AS UNSIGNED), COUNT(*)
		FROM `+table+` WHERE tenant_id=? AND corp_id=?
		  AND SUBSTRING_INDEX(SUBSTRING_INDEX(conversation_id, ':', 2), ':', -1)='2'
		  AND CAST(SUBSTRING_INDEX(conversation_id, ':', -1) AS UNSIGNED) IN (`+placeholders(len(ids))+`)
		GROUP BY CAST(SUBSTRING_INDEX(conversation_id, ':', -1) AS UNSIGNED)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[int]int{}
	for rows.Next() {
		var id, count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		counts[id] = count
	}
	return counts, rows.Err()
}

func (s *MySQLStore) RoomFilterOptions(ctx context.Context, filter dashboard.WorkMessageRoomFilterOptionsFilter) (dashboard.WorkMessageRoomFilterOptions, error) {
	options := dashboard.WorkMessageRoomFilterOptions{Employees: []dashboard.WorkMessageRoomOption{}, Customers: []dashboard.WorkMessageRoomOption{}, Groups: []dashboard.WorkMessageRoomOption{}}
	if filter.RestrictEmployeeIDs && len(uniquePositiveInts(filter.EmployeeIDs)) == 0 {
		options.Capabilities = []dashboard.WorkMessageCapability{}
		return options, nil
	}
	queries := workMessageRoomFilterOptionSQL()
	if filter.Kind == "" || filter.Kind == "employee" {
		items, err := s.roomFilterOptionsQuery(ctx, queries[0], filter.CorpID, filter.EmployeeIDs, true)
		if err != nil {
			return dashboard.WorkMessageRoomFilterOptions{}, err
		}
		options.Employees = items
	}
	if filter.Kind == "" || filter.Kind == "customer" {
		items, err := s.roomFilterOptionsQuery(ctx, queries[1], filter.CorpID, filter.EmployeeIDs, false)
		if err != nil {
			return dashboard.WorkMessageRoomFilterOptions{}, err
		}
		options.Customers = items
	}
	if filter.Kind == "" || filter.Kind == "group" {
		rows, err := s.db.QueryContext(ctx, queries[2], filter.CorpID)
		if err != nil {
			return dashboard.WorkMessageRoomFilterOptions{}, err
		}
		for rows.Next() {
			var id, count int
			var name string
			if err := rows.Scan(&id, &name, &count); err != nil {
				rows.Close()
				return dashboard.WorkMessageRoomFilterOptions{}, err
			}
			options.Groups = append(options.Groups, dashboard.WorkMessageRoomOption{Value: strconv.Itoa(id), Label: name, Count: count})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return dashboard.WorkMessageRoomFilterOptions{}, err
		}
		rows.Close()
	}
	capabilities, err := s.roomCapabilities(ctx, true)
	if err != nil {
		return dashboard.WorkMessageRoomFilterOptions{}, err
	}
	options.Capabilities = capabilities
	return options, nil
}

func workMessageRoomFilterOptionSQL() []string {
	return []string{
		`SELECT e.id, COALESCE(e.name,''), COUNT(DISTINCT membership.room_id)
		 FROM mc_work_employee e
		 LEFT JOIN mc_work_contact_room membership ON membership.employee_id=e.id AND membership.type=1 AND membership.deleted_at IS NULL
		 LEFT JOIN mc_work_room room ON room.id=membership.room_id AND room.corp_id=e.corp_id AND room.deleted_at IS NULL
		 WHERE e.corp_id = ? AND e.deleted_at IS NULL
		 GROUP BY e.id, e.name ORDER BY e.id`,
		`SELECT c.id, COALESCE(c.name,''), COUNT(DISTINCT membership.room_id)
		 FROM mc_work_contact c
		 LEFT JOIN mc_work_contact_room membership ON membership.contact_id=c.id AND membership.type=2 AND membership.deleted_at IS NULL
		 LEFT JOIN mc_work_room room ON room.id=membership.room_id AND room.corp_id=c.corp_id AND room.deleted_at IS NULL
		 WHERE c.corp_id = ? AND c.deleted_at IS NULL
		 GROUP BY c.id, c.name ORDER BY c.id`,
		`SELECT group_row.id, COALESCE(group_row.name,''), COUNT(DISTINCT room.id)
		 FROM mc_work_room_group group_row
		 LEFT JOIN mc_work_room room ON room.room_group_id=group_row.id AND room.corp_id=group_row.corp_id AND room.deleted_at IS NULL
		 WHERE group_row.corp_id = ? AND group_row.deleted_at IS NULL
		 GROUP BY group_row.id, group_row.name ORDER BY group_row.id`,
	}
}

func (s *MySQLStore) roomFilterOptionsQuery(ctx context.Context, query string, corpID int, employeeIDs []int, employee bool) ([]dashboard.WorkMessageRoomOption, error) {
	args := []any{corpID}
	if employee && len(uniquePositiveInts(employeeIDs)) > 0 {
		ids := uniquePositiveInts(employeeIDs)
		query = strings.Replace(query, "WHERE e.corp_id = ?", "WHERE e.corp_id = ? AND e.id IN ("+placeholders(len(ids))+")", 1)
		args = append(args, intsToAny(ids)...)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.WorkMessageRoomOption, 0)
	for rows.Next() {
		var id, count int
		var label string
		if err := rows.Scan(&id, &label, &count); err != nil {
			return nil, err
		}
		items = append(items, dashboard.WorkMessageRoomOption{Value: strconv.Itoa(id), Label: label, Count: count})
	}
	return items, rows.Err()
}

func workMessageRoomMembersWhere(filter dashboard.WorkMessageRoomMembersFilter) (string, []any) {
	where := []string{"membership.room_id = ?", "membership.deleted_at IS NULL"}
	args := []any{filter.RoomID}
	switch filter.Mode {
	case dashboard.WorkMessageRoomMemberModeEmployee:
		where = append(where, "membership.type = 1", "membership.status = 1")
	case dashboard.WorkMessageRoomMemberModeCustomer:
		where = append(where, "membership.type = 2", "membership.status = 1")
	case dashboard.WorkMessageRoomMemberModeLeft:
		where = append(where, "membership.status = 2")
	default:
		where = append(where, "membership.status = 1")
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		where = append(where, `(employee.name LIKE ? ESCAPE '\\' OR contact.name LIKE ? ESCAPE '\\' OR membership.wx_user_id LIKE ? ESCAPE '\\')`)
		pattern := workMessageLikePattern(keyword)
		args = append(args, pattern, pattern, pattern)
	}
	return strings.Join(where, " AND "), args
}

func (s *MySQLStore) RoomMembers(ctx context.Context, filter dashboard.WorkMessageRoomMembersFilter) (dashboard.WorkMessageRoomMemberPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PageSize = roomDirectoryPageSize
	if filter.RestrictEmployeeIDs && len(uniquePositiveInts(filter.EmployeeIDs)) == 0 {
		return dashboard.WorkMessageRoomMemberPage{Items: []dashboard.WorkMessageRoomMember{}, Page: filter.Page, PageSize: filter.PageSize, Capabilities: []dashboard.WorkMessageCapability{}}, nil
	}
	if filter.RestrictEmployeeIDs {
		visible, err := s.workMessageRoomHasVisibleArchive(ctx, filter.TenantID, filter.CorpID, filter.RoomID, filter.EmployeeIDs)
		if err != nil {
			return dashboard.WorkMessageRoomMemberPage{}, err
		}
		if !visible {
			return dashboard.WorkMessageRoomMemberPage{}, dashboard.ErrWorkMessageConversationNotFound
		}
	}
	where, args := workMessageRoomMembersWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_work_contact_room membership
		LEFT JOIN mc_work_employee employee ON employee.id=membership.employee_id AND employee.corp_id=?
		LEFT JOIN mc_work_contact contact ON contact.id=membership.contact_id AND contact.corp_id=?
		WHERE `+where, append([]any{filter.CorpID, filter.CorpID}, args...)...).Scan(&total); err != nil {
		return dashboard.WorkMessageRoomMemberPage{}, err
	}
	queryArgs := append([]any{filter.CorpID, filter.CorpID}, args...)
	queryArgs = append(queryArgs, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT membership.id,
		CASE WHEN membership.type=1 THEN COALESCE(employee.wx_user_id,'') ELSE COALESCE(contact.wx_external_userid,'') END,
		CASE WHEN membership.type=1 THEN 'employee' ELSE 'customer' END,
		CASE WHEN membership.type=1 THEN COALESCE(employee.name,'') ELSE COALESCE(contact.name,'') END,
		CASE WHEN membership.type=1 THEN COALESCE(employee.avatar,'') ELSE COALESCE(contact.avatar,'') END,
		membership.join_time, COALESCE(membership.out_time,''),
		CASE WHEN membership.status=2 THEN 'left' ELSE 'active' END,
		CASE WHEN membership.type=1 THEN membership.employee_id ELSE 0 END,
		CASE WHEN membership.type=2 THEN membership.contact_id ELSE 0 END
		FROM mc_work_contact_room membership
		LEFT JOIN mc_work_employee employee ON employee.id=membership.employee_id AND employee.corp_id=?
		LEFT JOIN mc_work_contact contact ON contact.id=membership.contact_id AND contact.corp_id=?
		WHERE `+where+` ORDER BY membership.status ASC, membership.join_time ASC, membership.id ASC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return dashboard.WorkMessageRoomMemberPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.WorkMessageRoomMember, 0)
	for rows.Next() {
		var item dashboard.WorkMessageRoomMember
		var joined sql.NullTime
		if err := rows.Scan(&item.ID, &item.ExternalID, &item.Kind, &item.Name, &item.Avatar, &joined, &item.LeftAt, &item.Status, &item.EmployeeID, &item.CustomerID); err != nil {
			return dashboard.WorkMessageRoomMemberPage{}, err
		}
		item.JoinedAt = formatTime(joined)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkMessageRoomMemberPage{}, err
	}
	capabilities, err := s.roomCapabilities(ctx, true)
	if err != nil {
		return dashboard.WorkMessageRoomMemberPage{}, err
	}
	return dashboard.WorkMessageRoomMemberPage{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize, Capabilities: capabilities}, nil
}

func (s *MySQLStore) RoomProfile(ctx context.Context, filter dashboard.WorkMessageRoomProfileFilter) (dashboard.WorkMessageRoomProfile, error) {
	if filter.RestrictEmployeeIDs {
		if len(uniquePositiveInts(filter.EmployeeIDs)) == 0 {
			return dashboard.WorkMessageRoomProfile{}, dashboard.ErrWorkMessageConversationNotFound
		}
		visible, err := s.workMessageRoomHasVisibleArchive(ctx, filter.TenantID, filter.CorpID, filter.RoomID, filter.EmployeeIDs)
		if err != nil {
			return dashboard.WorkMessageRoomProfile{}, err
		}
		if !visible {
			return dashboard.WorkMessageRoomProfile{}, dashboard.ErrWorkMessageConversationNotFound
		}
	}
	var profile dashboard.WorkMessageRoomProfile
	var createdAt sql.NullTime
	var dissolved int
	err := s.db.QueryRowContext(ctx, `SELECT room.id, COALESCE(room.wx_chat_id,''), COALESCE(room.name,''),
		COALESCE(room.owner_id,0), COALESCE(owner.name,''), COALESCE(room.created_at,room.create_time),
		COALESCE(room.status,0), CASE WHEN room.deleted_at IS NULL THEN 0 ELSE 1 END
		FROM mc_work_room room
		LEFT JOIN mc_work_employee owner ON owner.id=room.owner_id AND owner.corp_id=room.corp_id AND owner.deleted_at IS NULL
		WHERE room.id=? AND room.corp_id=?`, filter.RoomID, filter.CorpID).
		Scan(&profile.ID, &profile.ExternalID, &profile.Name, &profile.OwnerID, &profile.OwnerName, &createdAt, &profile.Status, &dissolved)
	if err == sql.ErrNoRows {
		return dashboard.WorkMessageRoomProfile{}, dashboard.ErrWorkMessageConversationNotFound
	}
	if err != nil {
		return dashboard.WorkMessageRoomProfile{}, err
	}
	profile.CreatedAt = formatTime(createdAt)
	profile.Dissolved = dissolved == 1
	if profile.Dissolved {
		profile.Status = "dissolved"
	} else {
		profile.Status = "active"
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),
		COALESCE(SUM(type=1),0), COALESCE(SUM(type=2),0)
		FROM mc_work_contact_room WHERE room_id=? AND deleted_at IS NULL`, filter.RoomID).
		Scan(&profile.MemberCount, &profile.EmployeeCount, &profile.CustomerCount); err != nil {
		return dashboard.WorkMessageRoomProfile{}, err
	}
	items := []dashboard.WorkMessageRoomDirectoryItem{{ID: profile.ID}}
	capabilities, err := s.decorateWorkMessageRoomFlags(ctx, filter.TenantID, filter.CorpID, filter.UserID, items, "", true)
	if err != nil {
		return dashboard.WorkMessageRoomProfile{}, err
	}
	profile.Focused = items[0].Focused
	profile.RiskCount = items[0].RiskCount
	profile.TimeoutCount = items[0].TimeoutCount
	profile.Capabilities = capabilities
	profile.Limitations = []dashboard.WorkMessageStaffLimitation{}
	return profile, nil
}

func (s *MySQLStore) workMessageRoomMessagesArchiveSource(ctx context.Context, filter dashboard.WorkMessageRoomMessagesFilter) (string, []any, string, bool, error) {
	mode, err := s.workMessageArchiveMode(ctx, filter.TenantID, filter.CorpID)
	if err != nil {
		return "", nil, "", false, err
	}
	source, available := effectiveArchiveSource(mode, "")
	if !available {
		return "", nil, "", false, nil
	}
	state, err := s.archiveSourceRegistryState(ctx)
	if err != nil {
		return "", nil, "", false, err
	}
	archiveFilter := dashboard.WorkMessageUserFilter{
		TenantID: filter.TenantID, UserID: filter.UserID, CorpID: filter.CorpID,
		AllowAllEmployees: true, ToUserType: 2, ToUserID: filter.RoomID,
		RestrictEmployeeIDs: filter.RestrictEmployeeIDs, EmployeeIDs: append([]int(nil), filter.EmployeeIDs...),
		MessageTypes: append([]int(nil), filter.MessageTypes...), ArchiveSource: source,
	}
	if filter.Date != "" {
		date, _ := time.Parse("2006-01-02", filter.Date)
		archiveFilter.DateTimeStart = date.Format("2006-01-02 00:00:00")
		archiveFilter.DateTimeEnd = date.AddDate(0, 0, 1).Format("2006-01-02 00:00:00")
	}
	sqlText, args, ok := workMessageFilteredUnionSQLWithArchiveSourceState(archiveFilter, state)
	return sqlText, args, source, ok, nil
}

func workMessageRoomMessagesBaseSQL(sourceSQL string) string {
	return `SELECT COALESCE(NULLIF(wm.msgid,''), CONCAT('seq:',wm.seq)) AS message_id,
		wm.id AS row_id, wm.table_index, wm.seq, wm.msg_data_time AS sent_at,
		CASE WHEN participant_employee.id IS NOT NULL THEN participant_employee.id
		     WHEN participant_contact.id IS NOT NULL THEN participant_contact.id
		     WHEN COALESCE(wm.is_current_user,0)=1 THEN wm.work_employee_id ELSE 0 END AS sender_id,
		CASE WHEN participant_employee.id IS NOT NULL OR COALESCE(wm.is_current_user,0)=1 THEN 'employee'
		     WHEN participant_contact.id IS NOT NULL THEN 'customer' ELSE 'member' END AS sender_kind,
		CASE WHEN participant_employee.id IS NOT NULL THEN COALESCE(participant_employee.name,'')
		     WHEN participant_contact.id IS NOT NULL THEN COALESCE(participant_contact.name,'')
		     WHEN COALESCE(wm.is_current_user,0)=1 THEN COALESCE(wm.employee_name,'')
		     ELSE '群成员' END AS sender_name,
		CASE WHEN participant_employee.id IS NOT NULL THEN COALESCE(participant_employee.avatar,'')
		     WHEN participant_contact.id IS NOT NULL THEN COALESCE(participant_contact.avatar,'')
		     WHEN COALESCE(wm.is_current_user,0)=1 THEN COALESCE(wm.employee_avatar,'') ELSE '' END AS sender_avatar,
		CASE WHEN COALESCE(wm.is_current_user,0)=1 THEN 'outbound' ELSE 'inbound' END AS direction,
		COALESCE(wm.msg_type,100) AS message_type, COALESCE(wm.content_raw,'') AS content_raw,
		COALESCE(wm.content_text,'') AS content_text
	FROM (` + sourceSQL + `) wm
	LEFT JOIN mochat_go_work_message_participant_identity identity_row
		ON identity_row.corp_id=wm.corp_id AND identity_row.msgid=wm.msgid AND identity_row.seq=wm.seq
	LEFT JOIN mc_work_employee participant_employee
		ON participant_employee.corp_id=wm.corp_id AND participant_employee.wx_user_id=identity_row.sender_wx_id AND participant_employee.deleted_at IS NULL
	LEFT JOIN mc_work_contact participant_contact
		ON participant_contact.corp_id=wm.corp_id AND participant_contact.wx_external_userid=identity_row.sender_wx_id AND participant_contact.deleted_at IS NULL
	WHERE wm.to_user_type=2 AND wm.to_user_id=?`
}

func workMessageRoomMessagesWhere(filter dashboard.WorkMessageRoomMessagesFilter, includeBefore bool) (string, []any, error) {
	where := []string{}
	args := []any{}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		where = append(where, `(message.sender_name LIKE ? ESCAPE '\\' OR message.content_text LIKE ? ESCAPE '\\')`)
		pattern := workMessageLikePattern(keyword)
		args = append(args, pattern, pattern)
	}
	if includeBefore && strings.TrimSpace(filter.Before) != "" {
		cursor, err := dashboard.DecodeWorkMessageStaffCursor(filter.Before)
		if err != nil {
			return "", nil, err
		}
		where = append(where, `(message.sent_at < ? OR (message.sent_at = ? AND (message.seq < ? OR (message.seq = ? AND (message.table_index < ? OR (message.table_index = ? AND message.row_id < ?))))))`)
		args = append(args, cursor.SentAt, cursor.SentAt, cursor.Seq, cursor.Seq, cursor.TableIndex, cursor.TableIndex, cursor.ID)
	}
	if len(where) == 0 {
		return "1 = 1", args, nil
	}
	return strings.Join(where, " AND "), args, nil
}

type workMessageRoomScanRow struct {
	item       dashboard.WorkMessageRoomMessage
	rowID      int
	tableIndex int
	seq        int64
	sentAt     string
}

func (s *MySQLStore) RoomMessages(ctx context.Context, filter dashboard.WorkMessageRoomMessagesFilter) (dashboard.WorkMessageRoomMessages, error) {
	filter.PageSize = roomDirectoryPageSize
	result := dashboard.WorkMessageRoomMessages{RoomID: filter.RoomID, Messages: []dashboard.WorkMessageRoomMessage{}}
	sourceSQL, sourceArgs, source, available, err := s.workMessageRoomMessagesArchiveSource(ctx, filter)
	if err != nil {
		return dashboard.WorkMessageRoomMessages{}, err
	}
	capabilities, err := s.roomCapabilities(ctx, available)
	if err != nil {
		return dashboard.WorkMessageRoomMessages{}, err
	}
	result.Capabilities = capabilities
	if !available {
		return result, nil
	}
	baseSQL := workMessageRoomMessagesBaseSQL(sourceSQL)
	baseArgs := append(append([]any{}, sourceArgs...), filter.RoomID)
	statsWhere, statsArgs, err := workMessageRoomMessagesWhere(filter, false)
	if err != nil {
		return dashboard.WorkMessageRoomMessages{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),
		COUNT(DISTINCT CASE WHEN message.sender_kind='employee' THEN message.sender_id END),
		COUNT(DISTINCT CASE WHEN message.sender_kind IN ('customer','member') THEN message.sender_id END)
		FROM (`+baseSQL+`) message WHERE `+statsWhere, append(baseArgs, statsArgs...)...).
		Scan(&result.Stats.MessageTotal, &result.Stats.EmployeeTotal, &result.Stats.CustomerTotal); err != nil {
		return dashboard.WorkMessageRoomMessages{}, err
	}
	queryWhere, queryArgs, err := workMessageRoomMessagesWhere(filter, true)
	if err != nil {
		return dashboard.WorkMessageRoomMessages{}, err
	}
	pageArgs := append(append(append([]any{}, baseArgs...), queryArgs...), filter.PageSize+1)
	rows, err := s.db.QueryContext(ctx, `SELECT message_id, row_id, table_index, seq, sent_at, sender_id, sender_kind,
		sender_name, sender_avatar, direction, message_type, content_raw
		FROM (`+baseSQL+`) message WHERE `+queryWhere+`
		ORDER BY message.sent_at DESC, message.seq DESC, message.table_index DESC, message.row_id DESC LIMIT ?`, pageArgs...)
	if err != nil {
		return dashboard.WorkMessageRoomMessages{}, err
	}
	defer rows.Close()
	rowsData := make([]workMessageRoomScanRow, 0, filter.PageSize+1)
	for rows.Next() {
		var row workMessageRoomScanRow
		var sentAt sql.NullTime
		var content sql.NullString
		if err := rows.Scan(&row.item.ID, &row.rowID, &row.tableIndex, &row.seq, &sentAt, &row.item.SenderID, &row.item.SenderKind,
			&row.item.SenderName, &row.item.SenderAvatar, &row.item.Direction, &row.item.Type, &content); err != nil {
			return dashboard.WorkMessageRoomMessages{}, err
		}
		row.sentAt = formatTime(sentAt)
		row.item.SentAt = row.sentAt
		row.item.Content = staffMessageContent(content.String)
		row.item.ArchiveSource = source
		if source == "external" {
			row.item.ArchiveSourceID = "wecom"
		} else {
			row.item.ArchiveSourceID = "simulation"
		}
		rowsData = append(rowsData, row)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkMessageRoomMessages{}, err
	}
	if len(rowsData) > filter.PageSize {
		result.HasMore = true
		last := rowsData[filter.PageSize]
		result.NextBefore = dashboard.EncodeWorkMessageStaffCursor(dashboard.WorkMessageStaffCursor{SentAt: last.sentAt, TableIndex: last.tableIndex, Seq: last.seq, ID: last.rowID})
		rowsData = rowsData[:filter.PageSize]
	}
	for left, right := 0, len(rowsData)-1; left < right; left, right = left+1, right-1 {
		rowsData[left], rowsData[right] = rowsData[right], rowsData[left]
	}
	for _, row := range rowsData {
		result.Messages = append(result.Messages, row.item)
	}
	return result, nil
}
