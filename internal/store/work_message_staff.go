package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

const staffInternalGroupUnavailableReason = "当前归档数据未提供内部群聊能力"

type staffMessageAggregate struct {
	ConversationCount  int
	LastConversationAt string
}

type staffDepartmentRow struct {
	ID       int
	ParentID int
	Name     string
	Order    int
}

func staffDirectoryWhere(filter dashboard.WorkMessageStaffDirectoryFilter) (string, []any) {
	if filter.RestrictEmployeeIDs && len(uniquePositiveInts(filter.EmployeeIDs)) == 0 {
		return "1 = 0", nil
	}
	where := []string{"e.corp_id = ?", "e.deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if filter.RestrictEmployeeIDs {
		ids := uniquePositiveInts(filter.EmployeeIDs)
		where = append(where, "e.id IN ("+placeholders(len(ids))+")")
		args = append(args, intsToAny(ids)...)
	}
	if filter.Keyword != "" {
		where = append(where, `e.name LIKE ? ESCAPE '\\'`)
		args = append(args, workMessageLikePattern(filter.Keyword))
	}
	if filter.DepartmentID > 0 {
		where = append(where, "wed.department_id = ?")
		args = append(args, filter.DepartmentID)
	}
	switch filter.Mode {
	case dashboard.WorkMessageStaffModeFocused:
		where = append(where, "focus.tenant_id = ?", "focus.corp_id = ?", "focus.user_id = ?")
		args = append(args, filter.TenantID, filter.CorpID, filter.UserID)
	case dashboard.WorkMessageStaffModeArchived:
		where = append(where, "archive.work_employee_id IS NOT NULL")
	case dashboard.WorkMessageStaffModeDeparted:
		where = append(where, "e.status = 5")
	}
	return strings.Join(where, " AND "), args
}

func (s *MySQLStore) StaffDirectory(ctx context.Context, filter dashboard.WorkMessageStaffDirectoryFilter) (dashboard.WorkMessageStaffDirectoryPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PageSize = 50
	departments, err := s.staffDepartments(ctx, filter.CorpID)
	if err != nil {
		return dashboard.WorkMessageStaffDirectoryPage{}, err
	}
	selectedDepartmentIDs := staffDepartmentDescendantIDs(departments, filter.DepartmentID)
	baseFilter := filter
	baseFilter.Mode = dashboard.WorkMessageStaffModeAll
	// Department selection is applied after memberships are loaded so parent
	// departments include descendants without changing directory-wide counts.
	baseFilter.DepartmentID = 0
	where, args := staffDirectoryWhere(baseFilter)
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT e.id, COALESCE(e.name,''), COALESCE(e.avatar,''), COALESCE(e.status,0)
		FROM mc_work_employee e
		LEFT JOIN mc_work_employee_department wed ON wed.employee_id=e.id AND wed.deleted_at IS NULL
		WHERE `+where+`
		ORDER BY e.id ASC
	`, args...)
	if err != nil {
		return dashboard.WorkMessageStaffDirectoryPage{}, err
	}
	employees := make([]dashboard.WorkMessageStaffEmployee, 0)
	visibleIDs := make([]int, 0)
	for rows.Next() {
		var employee dashboard.WorkMessageStaffEmployee
		if err := rows.Scan(&employee.ID, &employee.Name, &employee.Avatar, &employee.Status); err != nil {
			rows.Close()
			return dashboard.WorkMessageStaffDirectoryPage{}, err
		}
		employee.DepartmentIDs = []int{}
		employees = append(employees, employee)
		visibleIDs = append(visibleIDs, employee.ID)
	}
	if err := rows.Close(); err != nil {
		return dashboard.WorkMessageStaffDirectoryPage{}, err
	}
	memberships := map[int][]int{}
	messageAggregates := map[int]staffMessageAggregate{}
	focusedCounts := map[int]int{}
	if len(visibleIDs) > 0 {
		memberships, err = s.staffMemberships(ctx, visibleIDs)
		if err != nil {
			return dashboard.WorkMessageStaffDirectoryPage{}, err
		}
		messageAggregates, err = s.staffMessageAggregates(ctx, filter)
		if err != nil {
			return dashboard.WorkMessageStaffDirectoryPage{}, err
		}
		focusedCounts, err = s.staffFocusedCounts(ctx, filter, visibleIDs)
		if err != nil {
			return dashboard.WorkMessageStaffDirectoryPage{}, err
		}
	}

	counts := dashboard.WorkMessageStaffCounts{All: len(employees)}
	filtered := make([]dashboard.WorkMessageStaffEmployee, 0, len(employees))
	for index := range employees {
		employee := &employees[index]
		employee.DepartmentIDs = append([]int(nil), memberships[employee.ID]...)
		aggregate, archived := messageAggregates[employee.ID]
		employee.Archived = archived
		employee.ConversationCount = aggregate.ConversationCount
		employee.LastConversationAt = aggregate.LastConversationAt
		employee.FocusedConversationCount = focusedCounts[employee.ID]
		if archived {
			counts.Archived++
		}
		if employee.FocusedConversationCount > 0 {
			counts.Focused++
		}
		if employee.Status == 5 {
			counts.Departed++
		}
		departmentMatches := filter.DepartmentID == 0 || staffEmployeeMatchesDepartment(employee.DepartmentIDs, selectedDepartmentIDs)
		matches := departmentMatches && (filter.Mode == dashboard.WorkMessageStaffModeAll ||
			(filter.Mode == dashboard.WorkMessageStaffModeArchived && employee.Archived) ||
			(filter.Mode == dashboard.WorkMessageStaffModeFocused && employee.FocusedConversationCount > 0) ||
			(filter.Mode == dashboard.WorkMessageStaffModeDeparted && employee.Status == 5))
		if matches {
			filtered = append(filtered, *employee)
		}
	}

	limitations := []dashboard.WorkMessageStaffLimitation{}
	if len(departments) == 0 {
		limitations = append(limitations, dashboard.WorkMessageStaffLimitation{Key: "departments", Reason: "组织架构未同步，仅展示可访问员工"})
	}
	visibleSet := make(map[int]struct{}, len(visibleIDs))
	for _, id := range visibleIDs {
		visibleSet[id] = struct{}{}
	}
	tree := buildStaffDepartmentTree(departments, memberships, visibleSet)

	total := len(filtered)
	start := (filter.Page - 1) * filter.PageSize
	if start > total {
		start = total
	}
	end := start + filter.PageSize
	if end > total {
		end = total
	}
	return dashboard.WorkMessageStaffDirectoryPage{
		Departments: tree, Employees: filtered[start:end], Counts: counts,
		Page: filter.Page, PageSize: filter.PageSize, Total: total,
		Limitations: limitations, Capabilities: staffCapabilities(),
	}, nil
}

func staffDepartmentDescendantIDs(rows []staffDepartmentRow, rootID int) map[int]struct{} {
	result := map[int]struct{}{}
	if rootID <= 0 {
		return result
	}
	known := make(map[int]struct{}, len(rows))
	children := make(map[int][]int, len(rows))
	for _, row := range rows {
		known[row.ID] = struct{}{}
		children[row.ParentID] = append(children[row.ParentID], row.ID)
	}
	if _, ok := known[rootID]; !ok {
		return result
	}
	queue := []int{rootID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, seen := result[current]; seen {
			continue
		}
		result[current] = struct{}{}
		queue = append(queue, children[current]...)
	}
	return result
}

func staffEmployeeMatchesDepartment(departmentIDs []int, selectedDepartmentIDs map[int]struct{}) bool {
	for _, departmentID := range departmentIDs {
		if _, ok := selectedDepartmentIDs[departmentID]; ok {
			return true
		}
	}
	return false
}

func staffCapabilities() []dashboard.WorkMessageCapability {
	return []dashboard.WorkMessageCapability{
		{Key: "internalGroup", Available: false, Reason: staffInternalGroupUnavailableReason},
		{Key: "conversationDownload", Available: false, Reason: "当前系统未提供完整的单会话下载能力"},
	}
}

func (s *MySQLStore) staffMessageAggregates(ctx context.Context, filter dashboard.WorkMessageStaffDirectoryFilter) (map[int]staffMessageAggregate, error) {
	mode, err := s.workMessageArchiveMode(ctx, filter.TenantID, filter.CorpID)
	if err != nil {
		return nil, err
	}
	state, err := s.archiveSourceRegistryState(ctx)
	if err != nil {
		return nil, err
	}
	source, ok := effectiveArchiveSource(mode, "")
	if !ok {
		return map[int]staffMessageAggregate{}, nil
	}
	messageFilter := dashboard.WorkMessageUserFilter{
		CorpID: filter.CorpID, AllowAllEmployees: true, ToUserType: -1,
		RestrictEmployeeIDs: filter.RestrictEmployeeIDs, EmployeeIDs: append([]int(nil), filter.EmployeeIDs...), ArchiveSource: source,
	}
	sourceSQL, sourceArgs, ok := workMessageFilteredUnionSQLWithArchiveSourceState(messageFilter, state)
	if !ok {
		return map[int]staffMessageAggregate{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT wm.work_employee_id,
		       COUNT(DISTINCT CONCAT(wm.to_user_type, ':', wm.to_user_id)),
		       MAX(wm.msg_data_time)
		FROM (`+sourceSQL+`) wm
		GROUP BY wm.work_employee_id
	`, sourceArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[int]staffMessageAggregate{}
	for rows.Next() {
		var id, count int
		var last sql.NullTime
		if err := rows.Scan(&id, &count, &last); err != nil {
			return nil, err
		}
		result[id] = staffMessageAggregate{ConversationCount: count, LastConversationAt: formatTime(last)}
	}
	return result, rows.Err()
}

func (s *MySQLStore) staffFocusedCounts(ctx context.Context, filter dashboard.WorkMessageStaffDirectoryFilter, employeeIDs []int) (map[int]int, error) {
	available, err := s.tableExists(ctx, "mochat_go_work_message_focus")
	if err != nil || !available {
		return map[int]int{}, err
	}
	ids := uniquePositiveInts(employeeIDs)
	if len(ids) == 0 {
		return map[int]int{}, nil
	}
	args := []any{filter.TenantID, filter.CorpID, filter.UserID}
	args = append(args, intsToAny(ids)...)
	rows, err := s.db.QueryContext(ctx, `
		SELECT work_employee_id, COUNT(*)
		FROM mochat_go_work_message_focus
		WHERE tenant_id=? AND corp_id=? AND user_id=? AND work_employee_id IN (`+placeholders(len(ids))+`)
		GROUP BY work_employee_id
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[int]int{}
	for rows.Next() {
		var id, count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		result[id] = count
	}
	return result, rows.Err()
}

func (s *MySQLStore) staffMemberships(ctx context.Context, employeeIDs []int) (map[int][]int, error) {
	ids := uniquePositiveInts(employeeIDs)
	result := map[int][]int{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT employee_id, department_id
		FROM mc_work_employee_department
		WHERE deleted_at IS NULL AND employee_id IN (`+placeholders(len(ids))+`)
		ORDER BY employee_id, department_id
	`, intsToAny(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var employeeID, departmentID int
		if err := rows.Scan(&employeeID, &departmentID); err != nil {
			return nil, err
		}
		result[employeeID] = append(result[employeeID], departmentID)
	}
	return result, rows.Err()
}

func (s *MySQLStore) staffDepartments(ctx context.Context, corpID int) ([]staffDepartmentRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(parent_id,0), COALESCE(name,''), COALESCE(`+"`order`"+`,0)
		FROM mc_work_department
		WHERE corp_id=? AND deleted_at IS NULL
		ORDER BY `+"`order`"+`, id
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []staffDepartmentRow{}
	for rows.Next() {
		var row staffDepartmentRow
		if err := rows.Scan(&row.ID, &row.ParentID, &row.Name, &row.Order); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func buildStaffDepartmentTree(rows []staffDepartmentRow, memberships map[int][]int, visibleEmployees map[int]struct{}) []dashboard.WorkMessageStaffDepartment {
	byID := make(map[int]staffDepartmentRow, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	relevant := map[int]struct{}{}
	counts := map[int]map[int]struct{}{}
	for employeeID, departmentIDs := range memberships {
		if _, ok := visibleEmployees[employeeID]; !ok {
			continue
		}
		for _, departmentID := range departmentIDs {
			seen := map[int]struct{}{}
			for current := departmentID; current > 0; current = byID[current].ParentID {
				if _, loop := seen[current]; loop {
					break
				}
				seen[current] = struct{}{}
				if _, exists := byID[current]; !exists {
					break
				}
				relevant[current] = struct{}{}
				if counts[current] == nil {
					counts[current] = map[int]struct{}{}
				}
				counts[current][employeeID] = struct{}{}
			}
		}
	}
	// Keep the complete organization tree visible while filtering its member list.
	// Otherwise selecting a department makes the other department controls vanish,
	// so the user cannot switch back without reloading the page.
	for _, row := range rows {
		relevant[row.ID] = struct{}{}
	}
	children := map[int][]staffDepartmentRow{}
	for id := range relevant {
		row := byID[id]
		parentID := row.ParentID
		if _, ok := relevant[parentID]; !ok {
			parentID = 0
		}
		children[parentID] = append(children[parentID], row)
	}
	for parentID := range children {
		sort.SliceStable(children[parentID], func(i, j int) bool {
			if children[parentID][i].Order == children[parentID][j].Order {
				return children[parentID][i].ID < children[parentID][j].ID
			}
			return children[parentID][i].Order < children[parentID][j].Order
		})
	}
	var build func(int) []dashboard.WorkMessageStaffDepartment
	build = func(parentID int) []dashboard.WorkMessageStaffDepartment {
		result := make([]dashboard.WorkMessageStaffDepartment, 0, len(children[parentID]))
		for _, row := range children[parentID] {
			result = append(result, dashboard.WorkMessageStaffDepartment{
				ID: row.ID, ParentID: row.ParentID, Name: row.Name, EmployeeCount: len(counts[row.ID]), Children: build(row.ID),
			})
		}
		return result
	}
	return build(0)
}

func staffMessageWhere(filter dashboard.WorkMessageStaffDetailFilter) (string, []any, error) {
	where := []string{"wm.work_employee_id = ?", "wm.to_user_type = ?", "wm.to_user_id = ?"}
	args := []any{filter.EmployeeID, filter.ToUserType, filter.ToUserID}
	if types := uniquePositiveInts(filter.MessageTypes); len(types) > 0 {
		where = append(where, "wm.msg_type IN ("+placeholders(len(types))+")")
		args = append(args, intsToAny(types)...)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		where = append(where, `wm.content_text LIKE ? ESCAPE '\\'`)
		args = append(args, workMessageLikePattern(keyword))
	}
	if filter.Date != "" {
		start, err := time.Parse("2006-01-02", filter.Date)
		if err != nil {
			return "", nil, err
		}
		where = append(where, "wm.msg_data_time >= ?", "wm.msg_data_time < ?")
		args = append(args, start.Format("2006-01-02 15:04:05"), start.AddDate(0, 0, 1).Format("2006-01-02 15:04:05"))
	}
	if filter.Before != "" {
		cursor, err := dashboard.DecodeWorkMessageStaffCursor(filter.Before)
		if err != nil {
			return "", nil, err
		}
		where = append(where, `(wm.msg_data_time < ? OR
			(wm.msg_data_time = ? AND wm.seq < ?) OR
			(wm.msg_data_time = ? AND wm.seq = ? AND wm.table_index < ?) OR
			(wm.msg_data_time = ? AND wm.seq = ? AND wm.table_index = ? AND wm.id < ?))`)
		args = append(args, cursor.SentAt, cursor.SentAt, cursor.Seq, cursor.SentAt, cursor.Seq, cursor.TableIndex, cursor.SentAt, cursor.Seq, cursor.TableIndex, cursor.ID)
	}
	return strings.Join(where, " AND "), args, nil
}

func (s *MySQLStore) StaffDetail(ctx context.Context, filter dashboard.WorkMessageStaffDetailFilter) (dashboard.WorkMessageStaffDetail, error) {
	mode, err := s.workMessageArchiveMode(ctx, filter.TenantID, filter.CorpID)
	if err != nil {
		return dashboard.WorkMessageStaffDetail{}, err
	}
	state, err := s.archiveSourceRegistryState(ctx)
	if err != nil {
		return dashboard.WorkMessageStaffDetail{}, err
	}
	source, ok := effectiveArchiveSource(mode, "")
	if !ok {
		return dashboard.WorkMessageStaffDetail{}, dashboard.ErrWorkMessageConversationNotFound
	}
	baseFilter := dashboard.WorkMessageUserFilter{
		CorpID: filter.CorpID, WorkEmployeeID: filter.EmployeeID, ToUserType: filter.ToUserType, ToUserID: filter.ToUserID,
		RestrictEmployeeIDs: true, EmployeeIDs: []int{filter.EmployeeID}, ArchiveSource: source,
	}
	sourceSQL, sourceArgs, ok := workMessageFilteredUnionSQLWithArchiveSourceState(baseFilter, state)
	if !ok {
		return dashboard.WorkMessageStaffDetail{}, dashboard.ErrWorkMessageConversationNotFound
	}
	var stats dashboard.WorkMessageStaffStats
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT DATE(wm.msg_data_time)), COUNT(*),
		       COALESCE(SUM(CASE WHEN wm.is_current_user=0 THEN 1 ELSE 0 END),0),
		       COALESCE(SUM(CASE WHEN wm.is_current_user=1 THEN 1 ELSE 0 END),0)
		FROM (`+sourceSQL+`) wm
	`, sourceArgs...).Scan(&stats.CommunicationDays, &stats.MessageTotal, &stats.InboundTotal, &stats.OutboundTotal)
	if err != nil {
		return dashboard.WorkMessageStaffDetail{}, err
	}
	if stats.MessageTotal == 0 {
		return dashboard.WorkMessageStaffDetail{}, dashboard.ErrWorkMessageConversationNotFound
	}

	where, messageArgs, err := staffMessageWhere(filter)
	if err != nil {
		return dashboard.WorkMessageStaffDetail{}, err
	}
	args := append(append([]any{}, sourceArgs...), messageArgs...)
	args = append(args, filter.PageSize+1)
	rows, err := s.db.QueryContext(ctx, `
		SELECT wm.id, wm.table_index, wm.seq, wm.msgid, wm.work_employee_id,
		       wm.employee_name, wm.employee_avatar, wm.to_user_type, wm.to_user_id,
		       wm.target_name, wm.target_avatar, wm.sender_name, wm.sender_avatar,
		       wm.is_current_user, wm.msg_type, wm.content_raw, wm.msg_data_time`+archiveSourceJoinedProjectionForState(state)+`
		FROM (`+sourceSQL+`) wm`+archiveSourceRegistryJoinForState(state)+`
		WHERE `+where+`
		ORDER BY wm.msg_data_time DESC, wm.seq DESC, wm.table_index DESC, wm.id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return dashboard.WorkMessageStaffDetail{}, err
	}
	defer rows.Close()
	type messageRow struct {
		message                  dashboard.WorkMessageStaffMessage
		cursor                   dashboard.WorkMessageStaffCursor
		employeeName, targetName string
	}
	items := []messageRow{}
	for rows.Next() {
		var row messageRow
		var id, tableIndex, employeeID, toUserType, toUserID, isCurrentUser, messageType int
		var seq int64
		var msgID string
		var employeeName, employeeAvatar, targetName, targetAvatar, senderName, senderAvatar, contentRaw, archiveSource, archiveSourceID sql.NullString
		var sentAt sql.NullTime
		dest := []any{&id, &tableIndex, &seq, &msgID, &employeeID, &employeeName, &employeeAvatar, &toUserType, &toUserID, &targetName, &targetAvatar, &senderName, &senderAvatar, &isCurrentUser, &messageType, &contentRaw, &sentAt}
		if state.metadataAvailable() {
			dest = append(dest, &archiveSource, &archiveSourceID)
		}
		if err := rows.Scan(dest...); err != nil {
			return dashboard.WorkMessageStaffDetail{}, err
		}
		sentAtText := formatTime(sentAt)
		direction := "inbound"
		if isCurrentUser == 1 {
			direction = "outbound"
		}
		row.message = dashboard.WorkMessageStaffMessage{
			ID: workMessageArchiveID(msgID, seq, tableIndex, id), SenderName: nullString(senderName), SenderAvatar: nullString(senderAvatar),
			Direction: direction, SentAt: sentAtText, Type: messageType, Content: staffMessageContent(nullString(contentRaw)),
			ArchiveSource: nullString(archiveSource), ArchiveSourceID: nullString(archiveSourceID),
		}
		if !state.metadataAvailable() {
			row.message.ArchiveSource, row.message.ArchiveSourceID = "external", "wecom"
		}
		row.cursor = dashboard.WorkMessageStaffCursor{SentAt: sentAtText, TableIndex: tableIndex, Seq: seq, ID: id}
		row.employeeName, row.targetName = nullString(employeeName), nullString(targetName)
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkMessageStaffDetail{}, err
	}
	hasMore := len(items) > filter.PageSize
	if hasMore {
		items = items[:filter.PageSize]
	}
	nextBefore := ""
	if hasMore && len(items) > 0 {
		nextBefore = dashboard.EncodeWorkMessageStaffCursor(items[len(items)-1].cursor)
	}
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	messages := make([]dashboard.WorkMessageStaffMessage, 0, len(items))
	for _, item := range items {
		messages = append(messages, item.message)
	}
	employeeName, targetName := "", ""
	if len(items) > 0 {
		employeeName, targetName = items[0].employeeName, items[0].targetName
	}
	focused, err := s.workMessageFocusExists(ctx, filter.TenantID, filter.CorpID, filter.UserID, filter.EmployeeID, filter.ToUserType, filter.ToUserID)
	if err != nil {
		return dashboard.WorkMessageStaffDetail{}, err
	}
	return dashboard.WorkMessageStaffDetail{
		ConversationID: fmt.Sprintf("%d:%d:%d", filter.EmployeeID, filter.ToUserType, filter.ToUserID),
		EmployeeID:     filter.EmployeeID, TargetID: filter.ToUserID, EmployeeName: employeeName,
		TargetType: workMessageTargetType(filter.ToUserType), TargetName: targetName, Focused: focused,
		Stats: stats, Messages: messages, NextBefore: nextBefore, HasMore: hasMore, Capabilities: staffCapabilities(),
	}, nil
}

func staffMessageContent(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}
	var content any
	if err := json.Unmarshal([]byte(raw), &content); err == nil {
		switch typed := content.(type) {
		case map[string]any:
			return typed
		case string:
			return map[string]any{"text": typed}
		case []any:
			return map[string]any{"items": typed}
		case nil:
			return map[string]any{}
		default:
			return map[string]any{"value": typed}
		}
	}
	return map[string]any{"content": raw}
}
