package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

const trajectoryInternalGroupReason = "当前归档数据未提供内部群聊能力"

type trajectoryEventRow struct {
	Type  int
	ID    int
	Hour  string
	Count int64
	First sql.NullTime
	Last  sql.NullTime
}

type trajectoryTarget struct {
	Name   string
	Avatar string
	Status string
}

func trajectoryAvailableMetric(subjects, messages int64) dashboard.WorkMessageTrajectoryMetric {
	s, m := subjects, messages
	return dashboard.WorkMessageTrajectoryMetric{Status: "available", SubjectTotal: &s, MessageTotal: &m}
}

func trajectoryUnavailableMetric() dashboard.WorkMessageTrajectoryMetric {
	return dashboard.WorkMessageTrajectoryMetric{Status: "unavailable", Reason: trajectoryInternalGroupReason}
}

func (s *MySQLStore) TrajectoryDay(ctx context.Context, filter dashboard.WorkMessageTrajectoryFilter) (dashboard.WorkMessageTrajectoryDay, error) {
	day := dashboard.WorkMessageTrajectoryDay{Date: filter.Date, Timezone: "Asia/Shanghai", Metrics: map[string]dashboard.WorkMessageTrajectoryMetric{}, Events: []dashboard.WorkMessageTrajectoryEvent{}, Limitations: []dashboard.WorkMessageStaffLimitation{}, Capabilities: []dashboard.WorkMessageCapability{{Key: "internalGroup", Available: false, Reason: trajectoryInternalGroupReason}}}
	var employeeName, employeeAvatar string
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(name,''), COALESCE(avatar,'') FROM mc_work_employee WHERE corp_id=? AND id=? AND deleted_at IS NULL`, filter.CorpID, filter.EmployeeID).Scan(&employeeName, &employeeAvatar); err != nil {
		if err == sql.ErrNoRows {
			return day, dashboard.ErrWorkMessageConversationNotFound
		}
		return day, err
	}
	day.Employee.ID, day.Employee.Name, day.Employee.Avatar = filter.EmployeeID, employeeName, employeeAvatar

	mode, err := s.workMessageArchiveMode(ctx, filter.TenantID, filter.CorpID)
	if err != nil {
		return day, err
	}
	if _, available := workMessageArchivePredicate(mode); !available {
		for _, key := range []string{"internalSingle", "externalSingle", "internalGroup", "externalGroup"} {
			day.Metrics[key] = trajectoryUnavailableMetric()
		}
		return day, nil
	}
	registryState, err := s.archiveSourceRegistryState(ctx)
	if err != nil {
		return day, err
	}
	archiveSource, ok := effectiveArchiveSource(mode, "")
	if !ok {
		return day, nil
	}
	base := dashboard.WorkMessageUserFilter{TenantID: filter.TenantID, UserID: filter.UserID, CorpID: filter.CorpID, WorkEmployeeID: filter.EmployeeID, ToUserType: -1, DateTimeStart: filter.StartAt, DateTimeEnd: filter.EndAt, AllowAllEmployees: false, RestrictEmployeeIDs: filter.RestrictEmployeeIDs, EmployeeIDs: filter.EmployeeIDs, ArchiveSource: archiveSource}
	sourceSQL, sourceArgs, ok := workMessageFilteredUnionSQLWithArchiveSourceState(base, registryState)
	if !ok {
		return day, nil
	}
	metricSQL := `SELECT
		COUNT(DISTINCT CASE WHEN wm.to_user_type=0 AND wm.to_user_id>0 THEN wm.to_user_id END),
		COALESCE(SUM(CASE WHEN wm.to_user_type=0 AND wm.to_user_id>0 THEN 1 ELSE 0 END),0),
		COUNT(DISTINCT CASE WHEN wm.to_user_type=1 AND wm.to_user_id>0 THEN wm.to_user_id END),
		COALESCE(SUM(CASE WHEN wm.to_user_type=1 AND wm.to_user_id>0 THEN 1 ELSE 0 END),0),
		COUNT(DISTINCT CASE WHEN wm.to_user_type=2 AND wm.to_user_id>0 THEN wm.to_user_id END),
		COALESCE(SUM(CASE WHEN wm.to_user_type=2 AND wm.to_user_id>0 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN wm.to_user_type=2 AND wm.to_user_id=0 THEN 1 ELSE 0 END),0)
		FROM (` + sourceSQL + `) wm`
	var employeeSubjects, employeeMessages, customerSubjects, customerMessages, roomSubjects, roomMessages, unmatched int64
	if err := s.db.QueryRowContext(ctx, metricSQL, sourceArgs...).Scan(&employeeSubjects, &employeeMessages, &customerSubjects, &customerMessages, &roomSubjects, &roomMessages, &unmatched); err != nil {
		return day, err
	}
	day.Metrics["internalSingle"] = trajectoryAvailableMetric(employeeSubjects, employeeMessages)
	day.Metrics["externalSingle"] = trajectoryAvailableMetric(customerSubjects, customerMessages)
	day.Metrics["internalGroup"] = trajectoryUnavailableMetric()
	day.Metrics["externalGroup"] = trajectoryAvailableMetric(roomSubjects, roomMessages)
	day.UnmatchedTargetMessages = unmatched
	if unmatched > 0 {
		day.Limitations = append(day.Limitations, dashboard.WorkMessageStaffLimitation{Key: "unmatchedTargetMessages", Reason: fmt.Sprintf("有 %d 条消息无法关联会话对象", unmatched)})
	}

	eventWhere := "wm.to_user_id > 0"
	switch filter.ConversationType {
	case dashboard.WorkMessageTrajectoryEmployee:
		eventWhere += " AND wm.to_user_type=0"
	case dashboard.WorkMessageTrajectoryCustomer:
		eventWhere += " AND wm.to_user_type=1"
	case dashboard.WorkMessageTrajectoryRoom:
		eventWhere += " AND wm.to_user_type=2"
	}
	eventSQL := `SELECT wm.to_user_type, wm.to_user_id, DATE_FORMAT(wm.msg_data_time,'%H'), COUNT(*), MIN(wm.msg_data_time), MAX(wm.msg_data_time) FROM (` + sourceSQL + `) wm WHERE ` + eventWhere + ` GROUP BY wm.to_user_type, wm.to_user_id, DATE_FORMAT(wm.msg_data_time,'%H') ORDER BY DATE_FORMAT(wm.msg_data_time,'%H'), MIN(wm.msg_data_time), wm.to_user_type, wm.to_user_id`
	rows, err := s.db.QueryContext(ctx, eventSQL, sourceArgs...)
	if err != nil {
		return day, err
	}
	defer rows.Close()
	events := make([]trajectoryEventRow, 0)
	for rows.Next() {
		var row trajectoryEventRow
		if err := rows.Scan(&row.Type, &row.ID, &row.Hour, &row.Count, &row.First, &row.Last); err != nil {
			return day, err
		}
		events = append(events, row)
	}
	if err := rows.Err(); err != nil {
		return day, err
	}
	targets, err := s.trajectoryTargets(ctx, filter.CorpID, events)
	if err != nil {
		return day, err
	}
	for _, row := range events {
		target := targets[trajectoryTargetKey(row.Type, row.ID)]
		if target.Status == "" {
			target.Status = "missing"
			target.Name = ""
		}
		if row.Type == 2 && target.Status == "available" && strings.TrimSpace(target.Name) == "" {
			target.Name = "未命名客户群"
		}
		conversationID := fmt.Sprintf("%d:%d:%d", filter.EmployeeID, row.Type, row.ID)
		first, last := formatSQLTime(row.First), formatSQLTime(row.Last)
		eventsOut := dashboard.WorkMessageTrajectoryEvent{ID: conversationID + "@" + filter.Date + "T" + row.Hour, ConversationID: conversationID, Hour: row.Hour, TargetType: workMessageTargetType(row.Type), TargetID: row.ID, TargetName: target.Name, TargetAvatar: target.Avatar, TargetStatus: target.Status, MessageTotal: row.Count, FirstMessageAt: first, LastMessageAt: last}
		day.Events = append(day.Events, eventsOut)
	}
	return day, nil
}

func trajectoryTargetKey(kind, id int) string { return fmt.Sprintf("%d:%d", kind, id) }

func formatSQLTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02 15:04:05")
}

func (s *MySQLStore) trajectoryTargets(ctx context.Context, corpID int, rows []trajectoryEventRow) (map[string]trajectoryTarget, error) {
	result := map[string]trajectoryTarget{}
	ids := map[int]map[int]struct{}{0: {}, 1: {}, 2: {}}
	for _, row := range rows {
		if row.ID > 0 {
			ids[row.Type][row.ID] = struct{}{}
		}
	}
	for kind, table := range map[int]string{0: "mc_work_employee", 1: "mc_work_contact", 2: "mc_work_room"} {
		list := make([]int, 0, len(ids[kind]))
		for id := range ids[kind] {
			list = append(list, id)
		}
		sort.Ints(list)
		if len(list) == 0 {
			continue
		}
		ph := placeholders(len(list))
		args := []any{corpID}
		args = append(args, intsToAny(list)...)
		avatarColumn := "COALESCE(avatar,'')"
		if kind == 2 {
			// mc_work_room has no avatar column in the production schema.
			avatarColumn = "''"
		}
		rowsDB, err := s.db.QueryContext(ctx, `SELECT id, COALESCE(name,''), `+avatarColumn+` FROM `+table+` WHERE corp_id=? AND id IN (`+ph+`) AND deleted_at IS NULL`, args...)
		if err != nil {
			return nil, err
		}
		for rowsDB.Next() {
			var id int
			var name, avatar string
			if err := rowsDB.Scan(&id, &name, &avatar); err != nil {
				rowsDB.Close()
				return nil, err
			}
			result[trajectoryTargetKey(kind, id)] = trajectoryTarget{Name: name, Avatar: avatar, Status: "available"}
		}
		if err := rowsDB.Err(); err != nil {
			rowsDB.Close()
			return nil, err
		}
		rowsDB.Close()
	}
	return result, nil
}
