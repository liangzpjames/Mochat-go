package store

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) ChannelCodeWorkspaceEnabled() bool { return true }

func (s *MySQLStore) ChannelCodeProviderConfig(ctx context.Context, channelCodeID int, corpID int) (dashboard.RoomWelcomeCorpCredential, string, bool, error) {
	var configID string
	err := s.db.QueryRowContext(ctx, `
		SELECT wx_config_id
		FROM mc_channel_code
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, channelCodeID, corpID).Scan(&configID)
	if err == sql.ErrNoRows {
		return dashboard.RoomWelcomeCorpCredential{}, "", false, nil
	}
	if err != nil {
		return dashboard.RoomWelcomeCorpCredential{}, "", false, err
	}
	credential, found, err := s.RoomWelcomeCorpCredentialByID(ctx, corpID)
	if err != nil || !found {
		return dashboard.RoomWelcomeCorpCredential{}, "", false, err
	}
	return credential, configID, true, nil
}

func (s *MySQLStore) SetChannelCodeLifecycle(ctx context.Context, channelCodeID int, corpID int, state string, providerState string, providerError string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mc_channel_code
		SET lifecycle_state = ?, provider_state = ?, provider_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, state, providerState, providerError, channelCodeID, corpID)
	return err
}

func (s *MySQLStore) ChannelCodeWorkspacePage(ctx context.Context, filter dashboard.ChannelCodeWorkspaceFilter) (dashboard.ChannelCodeListPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 20)
	filter.CorpIDs = uniquePositiveInts(filter.CorpIDs)
	page := dashboard.ChannelCodeListPage{Items: []dashboard.ChannelCodeListItem{}, PerPage: filter.PerPage}
	if len(filter.CorpIDs) == 0 {
		return page, nil
	}
	where, args := channelCodeWorkspaceWhere(filter)
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_channel_code AS cc WHERE "+strings.Join(where, " AND "), args...).Scan(&page.Total); err != nil {
		return dashboard.ChannelCodeListPage{}, err
	}
	if page.Total == 0 {
		return page, nil
	}
	page.TotalPage = pageCount(page.Total, filter.PerPage)
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT cc.id, cc.group_id, cc.name, cc.qrcode_url, cc.auto_add_friend, cc.tags, cc.type,
		       COALESCE(cg.name, ''), cc.created_at,
		       COALESCE(creator.id, 0), COALESCE(creator.name, ''),
		       cc.validity_kind, cc.valid_from, cc.valid_until,
		       cc.lifecycle_state, cc.drainage_employee
		FROM mc_channel_code AS cc
		LEFT JOIN mc_channel_code_group AS cg ON cg.id = cc.group_id AND cg.deleted_at IS NULL
		LEFT JOIN (
			SELECT business_id, MAX(id) AS latest_log_id
			FROM mc_business_log
			WHERE event = 100
			GROUP BY business_id
		) AS latest_log ON latest_log.business_id = cc.id
		LEFT JOIN mc_business_log AS created_log ON created_log.id = latest_log.latest_log_id
		LEFT JOIN mc_work_employee AS creator ON creator.id = created_log.operation_id
			AND creator.corp_id = cc.corp_id AND creator.deleted_at IS NULL
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY cc.updated_at DESC, cc.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.ChannelCodeListPage{}, err
	}
	defer rows.Close()

	items := make([]dashboard.ChannelCodeListItem, 0)
	channelIDs := make([]int, 0)
	tagIDs := make([]int, 0)
	itemTagIDs := map[int][]int{}
	itemEmployees := map[int][]int{}
	for rows.Next() {
		var item dashboard.ChannelCodeListItem
		var rawTags, rawDrainage []byte
		var createdAt, validFrom, validUntil sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.GroupID, &item.Name, &item.QRCodeURL, &item.AutoAddFriend, &rawTags, &item.Type,
			&item.GroupName, &createdAt, &item.Creator.ID, &item.Creator.Name,
			&item.Validity.Kind, &validFrom, &validUntil, &item.State, &rawDrainage,
		); err != nil {
			return dashboard.ChannelCodeListPage{}, err
		}
		item.CreatedAt = formatTime(createdAt)
		item.Validity.From = formatTime(validFrom)
		item.Validity.Until = formatTime(validUntil)
		if item.Validity.Kind == "" {
			item.Validity.Kind = "permanent"
		}
		if item.State == "" {
			item.State = "active"
		}
		item.StatisticsAvailable = true
		itemTagIDs[item.ID] = intSliceFromJSON(rawTags)
		tagIDs = append(tagIDs, itemTagIDs[item.ID]...)
		itemEmployees[item.ID] = channelCodeDrainageEmployeeIDs(decodeJSONAny(rawDrainage))
		channelIDs = append(channelIDs, item.ID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ChannelCodeListPage{}, err
	}

	tagNames, err := s.channelCodeTagNames(ctx, tagIDs)
	if err != nil {
		return dashboard.ChannelCodeListPage{}, err
	}
	contactNums, err := s.channelCodeContactNums(ctx, channelIDs)
	if err != nil {
		return dashboard.ChannelCodeListPage{}, err
	}
	allEmployeeIDs := make([]int, 0)
	for _, ids := range itemEmployees {
		allEmployeeIDs = append(allEmployeeIDs, ids...)
	}
	employees, err := s.channelCodeWorkspaceEmployees(ctx, allEmployeeIDs)
	if err != nil {
		return dashboard.ChannelCodeListPage{}, err
	}
	for i := range items {
		for _, tagID := range itemTagIDs[items[i].ID] {
			if name, ok := tagNames[tagID]; ok {
				items[i].Tags = append(items[i].Tags, name)
			}
		}
		if items[i].Tags == nil {
			items[i].Tags = []string{}
		}
		items[i].ContactNum = contactNums[items[i].ID]
		count := items[i].ContactNum
		items[i].AddedFriendCount = &count
		for _, employeeID := range itemEmployees[items[i].ID] {
			if employee, ok := employees[employeeID]; ok {
				items[i].Employees = append(items[i].Employees, employee)
			}
		}
		if items[i].Employees == nil {
			items[i].Employees = []dashboard.ChannelCodeEmployee{}
		}
	}
	page.Items = items
	return page, nil
}

func (s *MySQLStore) ChannelCodeWorkspaceStatistics(ctx context.Context, filter dashboard.ChannelCodeStatisticsFilter) (dashboard.ChannelCodeStatisticsPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 20)
	filter.CorpIDs = uniquePositiveInts(filter.CorpIDs)
	page := dashboard.ChannelCodeStatisticsPage{
		Rows:    []dashboard.ChannelCodeStatisticsRow{},
		PerPage: filter.PerPage,
	}
	if len(filter.CorpIDs) == 0 {
		page.Summary = dashboard.ChannelCodeStatisticsSummary{
			Available:  true,
			AsOf:       time.Now().Format(time.RFC3339),
			Timezone:   "Asia/Shanghai",
			Definition: "按渠道码归因的客户关联状态变化",
		}
		return page, nil
	}
	where, whereArgs := channelCodeWorkspaceWhere(dashboard.ChannelCodeWorkspaceFilter{
		CorpIDs: filter.CorpIDs,
		GroupID: filter.GroupID,
		Name:    filter.Name,
	})
	whereSQL := strings.Join(where, " AND ")
	startAt := strings.TrimSpace(filter.StartDate) + " 00:00:00"
	endDate := strings.TrimSpace(filter.EndDate)
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}
	endAt := endDate + " 00:00:00"

	args := []any{startAt, endAt, startAt, endAt, startAt, endAt, endAt, endAt}
	args = append(args, whereArgs...)
	var summary dashboard.ChannelCodeStatisticsSummary
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN rel.create_time >= ? AND rel.create_time < DATE_ADD(?, INTERVAL 1 DAY) THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN rel.deleted_at >= ? AND rel.deleted_at < DATE_ADD(?, INTERVAL 1 DAY) THEN 1 ELSE 0 END), 0),
			COUNT(DISTINCT CASE WHEN rel.create_time >= ? AND rel.create_time < DATE_ADD(?, INTERVAL 1 DAY) THEN rel.contact_id END),
			COUNT(DISTINCT CASE WHEN rel.create_time < DATE_ADD(?, INTERVAL 1 DAY) AND (rel.deleted_at IS NULL OR rel.deleted_at >= DATE_ADD(?, INTERVAL 1 DAY)) THEN rel.contact_id END),
			COUNT(DISTINCT cc.id)
		FROM mc_channel_code AS cc
		LEFT JOIN mc_work_contact_employee AS rel ON rel.state = CONCAT('channelCode-', cc.id)
		WHERE `+whereSQL, args...).Scan(
		&summary.AddedAttempts,
		&summary.LostAttempts,
		&summary.AddedCustomers,
		&summary.RetainedCustomers,
		&summary.CodeCount,
	); err != nil {
		return dashboard.ChannelCodeStatisticsPage{}, err
	}
	summary.Available = true
	summary.AsOf = time.Now().Format(time.RFC3339)
	summary.Timezone = "Asia/Shanghai"
	summary.Definition = "按渠道码归因的客户关联状态变化"
	page.Summary = summary
	page.Total = summary.CodeCount
	page.TotalPage = pageCount(page.Total, filter.PerPage)
	if page.Total == 0 {
		return page, nil
	}

	rowArgs := []any{startAt, endAt, startAt, endAt, startAt, endAt, endAt, endAt}
	rowArgs = append(rowArgs, whereArgs...)
	rowArgs = append(rowArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			cc.id,
			cc.name,
			COALESCE(SUM(CASE WHEN rel.create_time >= ? AND rel.create_time < DATE_ADD(?, INTERVAL 1 DAY) THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN rel.deleted_at >= ? AND rel.deleted_at < DATE_ADD(?, INTERVAL 1 DAY) THEN 1 ELSE 0 END), 0),
			COUNT(DISTINCT CASE WHEN rel.create_time >= ? AND rel.create_time < DATE_ADD(?, INTERVAL 1 DAY) THEN rel.contact_id END),
			COUNT(DISTINCT CASE WHEN rel.create_time < DATE_ADD(?, INTERVAL 1 DAY) AND (rel.deleted_at IS NULL OR rel.deleted_at >= DATE_ADD(?, INTERVAL 1 DAY)) THEN rel.contact_id END)
		FROM mc_channel_code AS cc
		LEFT JOIN mc_work_contact_employee AS rel ON rel.state = CONCAT('channelCode-', cc.id)
		WHERE `+whereSQL+`
		GROUP BY cc.id, cc.name, cc.updated_at
		ORDER BY cc.updated_at DESC, cc.id DESC
		LIMIT ? OFFSET ?
	`, rowArgs...)
	if err != nil {
		return dashboard.ChannelCodeStatisticsPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item dashboard.ChannelCodeStatisticsRow
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.AddedAttempts,
			&item.LostAttempts,
			&item.AddedCustomers,
			&item.RetainedCustomers,
		); err != nil {
			return dashboard.ChannelCodeStatisticsPage{}, err
		}
		item.Available = true
		page.Rows = append(page.Rows, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ChannelCodeStatisticsPage{}, err
	}
	return page, nil
}

func channelCodeWorkspaceWhere(filter dashboard.ChannelCodeWorkspaceFilter) ([]string, []any) {
	where := []string{
		"cc.corp_id IN (" + placeholders(len(filter.CorpIDs)) + ")",
		"cc.deleted_at IS NULL",
		"cc.data_source <> 'simulation'",
	}
	args := make([]any, 0, len(filter.CorpIDs)+5)
	for _, corpID := range filter.CorpIDs {
		args = append(args, corpID)
	}
	if filter.GroupID > 0 {
		where = append(where, "cc.group_id = ?")
		args = append(args, filter.GroupID)
	}
	if name := strings.TrimSpace(filter.Name); name != "" {
		where = append(where, "cc.name LIKE ?")
		args = append(args, "%"+name+"%")
	}
	if creator := strings.TrimSpace(filter.Creator); creator != "" {
		where = append(where, `EXISTS (
			SELECT 1
			FROM mc_business_log AS creator_log
			INNER JOIN mc_work_employee AS creator_filter ON creator_filter.id = creator_log.operation_id
			WHERE creator_log.business_id = cc.id
			  AND creator_log.event = 100
			  AND creator_filter.corp_id = cc.corp_id
			  AND creator_filter.deleted_at IS NULL
			  AND creator_filter.name LIKE ?
		)`)
		args = append(args, "%"+creator+"%")
	}
	if filter.EmployeeID > 0 {
		where = append(where, "JSON_SEARCH(cc.drainage_employee, 'one', ?, NULL, '$**.employeeId') IS NOT NULL")
		args = append(args, strconv.Itoa(filter.EmployeeID))
	}
	if state := strings.TrimSpace(filter.State); state != "" {
		where = append(where, "cc.lifecycle_state = ?")
		args = append(args, state)
	}
	return where, args
}

func channelCodeDrainageEmployeeIDs(value any) []int {
	ids := make([]int, 0)
	var visit func(any)
	visit = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			if employeeID, ok := typed["employeeId"]; ok {
				ids = append(ids, intsFromAny(employeeID)...)
			}
			for key, nested := range typed {
				if key != "employeeId" {
					visit(nested)
				}
			}
		case []any:
			for _, nested := range typed {
				visit(nested)
			}
		}
	}
	visit(value)
	return uniquePositiveInts(ids)
}

func (s *MySQLStore) channelCodeWorkspaceEmployees(ctx context.Context, employeeIDs []int) (map[int]dashboard.ChannelCodeEmployee, error) {
	result := map[int]dashboard.ChannelCodeEmployee{}
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(employeeIDs))
	for _, employeeID := range employeeIDs {
		args = append(args, employeeID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.name, COALESCE(d.name, '')
		FROM mc_work_employee AS e
		LEFT JOIN mc_work_employee_department AS ed ON ed.employee_id = e.id AND ed.deleted_at IS NULL
		LEFT JOIN mc_work_department AS d ON d.id = ed.department_id AND d.deleted_at IS NULL
		WHERE e.id IN (`+placeholders(len(employeeIDs))+`) AND e.deleted_at IS NULL
		ORDER BY e.id ASC, d.id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var employeeID int
		var name, department string
		if err := rows.Scan(&employeeID, &name, &department); err != nil {
			return nil, err
		}
		item := result[employeeID]
		item.ID = employeeID
		item.Name = name
		if department != "" && !containsString(item.Departments, department) {
			item.Departments = append(item.Departments, department)
		}
		if item.Departments == nil {
			item.Departments = []string{}
		}
		result[employeeID] = item
	}
	return result, rows.Err()
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
