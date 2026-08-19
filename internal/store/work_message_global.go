package store

import (
	"context"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) WorkMessageGlobalPage(ctx context.Context, filter dashboard.WorkMessageGlobalFilter) (dashboard.WorkMessageGlobalPage, error) {
	userFilter := workMessageGlobalUserFilter(filter)
	page, err := s.WorkMessageToUsers(ctx, userFilter)
	if err != nil {
		return dashboard.WorkMessageGlobalPage{}, err
	}
	items := make([]dashboard.WorkMessageGlobalConversation, 0, len(page.Items))
	focusAvailable, err := s.tableExists(ctx, "mochat_go_work_message_focus")
	if err != nil {
		return dashboard.WorkMessageGlobalPage{}, err
	}
	for _, item := range page.Items {
		conversationID := fmt.Sprintf("%d:%d:%d", item.WorkEmployeeID, item.ToUserType, item.ToUserID)
		messagePage, err := s.WorkMessagePage(ctx, dashboard.WorkMessageFilter{
			CorpID:              filter.CorpID,
			WorkEmployeeID:      item.WorkEmployeeID,
			ToUserType:          item.ToUserType,
			ToUserID:            item.ToUserID,
			DateTimeStart:       filter.StartAt,
			DateTimeEnd:         filter.EndAt,
			Page:                1,
			PerPage:             1,
			RestrictEmployeeIDs: filter.RestrictEmployeeIDs,
			EmployeeIDs:         filter.EmployeeIDs,
			ArchiveSource:       filter.ArchiveSource,
		})
		if err != nil {
			return dashboard.WorkMessageGlobalPage{}, err
		}
		focused := false
		if focusAvailable {
			focused, err = s.workMessageFocusExists(ctx, filter.TenantID, filter.CorpID, filter.UserID, item.WorkEmployeeID, item.ToUserType, item.ToUserID)
			if err != nil {
				return dashboard.WorkMessageGlobalPage{}, err
			}
		}
		riskCount, timeoutCount, err := s.workMessageFlagCounts(ctx, filter, conversationID)
		if err != nil {
			return dashboard.WorkMessageGlobalPage{}, err
		}
		items = append(items, dashboard.WorkMessageGlobalConversation{
			ID:              workMessageArchiveID(item.MsgID, item.Seq, item.TableIndex, item.ID),
			ConversationID:  conversationID,
			EmployeeID:      item.WorkEmployeeID,
			EmployeeName:    item.EmployeeName,
			EmployeeAvatar:  item.EmployeeAvatar,
			TargetType:      workMessageTargetType(item.ToUserType),
			TargetID:        item.ToUserID,
			TargetName:      item.Name,
			TargetAvatar:    item.Avatar,
			LastMessage:     item.Content,
			LastMessageType: item.Type,
			LastDirection:   item.Direction,
			SentAt:          item.MsgDataTime,
			MessageTotal:    messagePage.Total,
			RiskCount:       riskCount,
			TimeoutCount:    timeoutCount,
			Focused:         focused,
			ArchiveSource:   item.ArchiveSource,
			ArchiveSourceID: item.ArchiveSourceID,
		})
	}
	return dashboard.WorkMessageGlobalPage{Items: items, Total: page.Total, Page: page.Page, PageSize: page.PerPage}, nil
}

func (s *MySQLStore) WorkMessageGlobalOverview(ctx context.Context, filter dashboard.WorkMessageGlobalFilter) (dashboard.WorkMessageGlobalOverview, error) {
	userFilter := workMessageGlobalUserFilter(filter)
	mode, err := s.workMessageArchiveMode(ctx, filter.TenantID, filter.CorpID)
	if err != nil {
		return dashboard.WorkMessageGlobalOverview{}, err
	}
	if _, available := workMessageArchivePredicate(mode); !available {
		return unavailableGlobalOverview("当前企业没有可用的会话存档数据"), nil
	}
	registryState, err := s.archiveSourceRegistryState(ctx)
	if err != nil {
		return dashboard.WorkMessageGlobalOverview{}, err
	}
	effectiveSource, sourceOK := effectiveArchiveSource(mode, userFilter.ArchiveSource)
	if !sourceOK {
		return unavailableGlobalOverview("当前归档来源不可用"), nil
	}
	userFilter.ArchiveSource = effectiveSource
	sourceSQL, sourceArgs, sourceOK := workMessageFilteredUnionSQLWithArchiveSourceState(userFilter, registryState)
	if !sourceOK {
		return unavailableGlobalOverview("当前归档来源没有可用数据"), nil
	}
	whereSQL, whereArgs := workMessageUserWhere(userFilter)
	args := append(append([]any{}, sourceArgs...), whereArgs...)
	var customerConversations, customerMessages, customerEmployees int64
	var roomConversations, roomMessages, roomEmployees int64
	var employeeConversations, employeeMessages int64
	err = s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(DISTINCT CASE WHEN wm.to_user_type = 1 THEN CONCAT(wm.work_employee_id, ':', wm.to_user_id) END),
			COALESCE(SUM(CASE WHEN wm.to_user_type = 1 THEN 1 ELSE 0 END), 0),
			COUNT(DISTINCT CASE WHEN wm.to_user_type = 1 THEN wm.work_employee_id END),
			COUNT(DISTINCT CASE WHEN wm.to_user_type = 2 THEN CONCAT(wm.work_employee_id, ':', wm.to_user_id) END),
			COALESCE(SUM(CASE WHEN wm.to_user_type = 2 THEN 1 ELSE 0 END), 0),
			COUNT(DISTINCT CASE WHEN wm.to_user_type = 2 THEN wm.work_employee_id END),
			COUNT(DISTINCT CASE WHEN wm.to_user_type = 0 THEN CONCAT(wm.work_employee_id, ':', wm.to_user_id) END),
			COALESCE(SUM(CASE WHEN wm.to_user_type = 0 THEN 1 ELSE 0 END), 0)
		FROM (`+sourceSQL+`) wm
		WHERE `+whereSQL, args...).Scan(
		&customerConversations, &customerMessages, &customerEmployees,
		&roomConversations, &roomMessages, &roomEmployees,
		&employeeConversations, &employeeMessages,
	)
	if err != nil {
		return dashboard.WorkMessageGlobalOverview{}, err
	}
	metrics := map[string]dashboard.WorkMessageMetricValue{}
	setMetric(metrics, "customerConversations", customerConversations)
	setMetric(metrics, "customerMessages", customerMessages)
	setMetric(metrics, "customerEmployees", customerEmployees)
	setMetric(metrics, "roomConversations", roomConversations)
	setMetric(metrics, "roomMessages", roomMessages)
	setMetric(metrics, "roomEmployees", roomEmployees)
	setMetric(metrics, "employeeConversations", employeeConversations)
	setMetric(metrics, "employeeMessages", employeeMessages)

	capabilities := []dashboard.WorkMessageCapability{{Key: "aiSummary", Available: false, Reason: "当前系统未接入会话级 AI 摘要数据"}, {Key: "internalGroup", Available: false, Reason: "当前归档数据未提供内部群聊能力"}}
	riskAvailable, err := s.tableExists(ctx, "mochat_go_risk_records")
	if err != nil {
		return dashboard.WorkMessageGlobalOverview{}, err
	}
	timeoutAvailable, err := s.tableExists(ctx, "mochat_go_timeout_records")
	if err != nil {
		return dashboard.WorkMessageGlobalOverview{}, err
	}
	if riskAvailable {
		value, err := s.countFlaggedConversations(ctx, "mochat_go_risk_records", filter)
		if err != nil {
			return dashboard.WorkMessageGlobalOverview{}, err
		}
		setMetric(metrics, "riskConversations", value)
		capabilities = append(capabilities, dashboard.WorkMessageCapability{Key: "riskRecords", Available: true})
	} else {
		metrics["riskConversations"] = unavailableMetric("风险记录表未接入")
		capabilities = append(capabilities, dashboard.WorkMessageCapability{Key: "riskRecords", Available: false, Reason: "系统没有风险记录数据表"})
	}
	if timeoutAvailable {
		value, err := s.countFlaggedConversations(ctx, "mochat_go_timeout_records", filter)
		if err != nil {
			return dashboard.WorkMessageGlobalOverview{}, err
		}
		setMetric(metrics, "timeoutConversations", value)
		capabilities = append(capabilities, dashboard.WorkMessageCapability{Key: "timeoutRecords", Available: true})
	} else {
		metrics["timeoutConversations"] = unavailableMetric("超时记录表未接入")
		capabilities = append(capabilities, dashboard.WorkMessageCapability{Key: "timeoutRecords", Available: false, Reason: "系统没有超时记录数据表"})
	}
	return dashboard.WorkMessageGlobalOverview{Metrics: metrics, Capabilities: capabilities}, nil
}

func (s *MySQLStore) SetWorkMessageFocus(ctx context.Context, input dashboard.WorkMessageFocusInput) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT IGNORE INTO mochat_go_work_message_focus (tenant_id, corp_id, user_id, work_employee_id, to_user_type, to_user_id)
		VALUES (?, ?, ?, ?, ?, ?)
	`, input.TenantID, input.CorpID, input.UserID, input.WorkEmployeeID, input.ToUserType, input.ToUserID)
	return err
}

func (s *MySQLStore) DeleteWorkMessageFocus(ctx context.Context, input dashboard.WorkMessageFocusInput) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM mochat_go_work_message_focus
		WHERE tenant_id=? AND corp_id=? AND user_id=? AND work_employee_id=? AND to_user_type=? AND to_user_id=?
	`, input.TenantID, input.CorpID, input.UserID, input.WorkEmployeeID, input.ToUserType, input.ToUserID)
	return err
}

func (s *MySQLStore) WorkMessageGlobalConversationExists(ctx context.Context, filter dashboard.WorkMessageGlobalFilter) (bool, error) {
	page, err := s.WorkMessageToUsers(ctx, workMessageGlobalUserFilter(filter))
	return len(page.Items) > 0, err
}

func workMessageGlobalUserFilter(filter dashboard.WorkMessageGlobalFilter) dashboard.WorkMessageUserFilter {
	toUserType := -1
	switch filter.ConversationType {
	case "employee":
		toUserType = 0
	case "customer":
		toUserType = 1
	case "room":
		toUserType = 2
	}
	return dashboard.WorkMessageUserFilter{
		TenantID:            filter.TenantID,
		UserID:              filter.UserID,
		CorpID:              filter.CorpID,
		WorkEmployeeID:      0,
		ToUserType:          toUserType,
		ToUserID:            filter.ToUserID,
		Keyword:             filter.Keyword,
		DateTimeStart:       filter.StartAt,
		DateTimeEnd:         filter.EndAt,
		AllowAllEmployees:   true,
		RestrictEmployeeIDs: filter.RestrictEmployeeIDs,
		EmployeeIDs:         append([]int(nil), filter.EmployeeIDs...),
		ArchiveSource:       filter.ArchiveSource,
		MessageTypes:        append([]int(nil), filter.MessageTypes...),
		GlobalBucket:        string(filter.Bucket),
		Page:                positivePage(filter.Page),
		PerPage:             positivePerPage(filter.PageSize, 20),
	}
}

func (s *MySQLStore) workMessageFocusExists(ctx context.Context, tenantID, corpID, userID, employeeID, toUserType, toUserID int) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_work_message_focus WHERE tenant_id=? AND corp_id=? AND user_id=? AND work_employee_id=? AND to_user_type=? AND to_user_id=?`, tenantID, corpID, userID, employeeID, toUserType, toUserID).Scan(&count)
	return count > 0, err
}

func (s *MySQLStore) workMessageFlagCounts(ctx context.Context, filter dashboard.WorkMessageGlobalFilter, conversationID string) (int, int, error) {
	risk, err := s.countFlagRows(ctx, "mochat_go_risk_records", filter, conversationID)
	if err != nil {
		return 0, 0, err
	}
	timeout, err := s.countFlagRows(ctx, "mochat_go_timeout_records", filter, conversationID)
	return risk, timeout, err
}

func (s *MySQLStore) countFlagRows(ctx context.Context, table string, filter dashboard.WorkMessageGlobalFilter, conversationID string) (int, error) {
	available, err := s.tableExists(ctx, table)
	if err != nil || !available {
		return 0, err
	}
	where := []string{"tenant_id=?", "corp_id=?", "conversation_id=?"}
	args := []any{filter.TenantID, filter.CorpID, conversationID}
	if filter.StartAt != "" {
		where = append(where, "occurred_at >= ?")
		args = append(args, filter.StartAt)
	}
	if filter.EndAt != "" {
		where = append(where, "occurred_at < ?")
		args = append(args, filter.EndAt)
	}
	var count int
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE "+strings.Join(where, " AND "), args...).Scan(&count)
	return count, err
}

func (s *MySQLStore) countFlaggedConversations(ctx context.Context, table string, filter dashboard.WorkMessageGlobalFilter) (int64, error) {
	where := []string{"tenant_id=?", "corp_id=?"}
	args := []any{filter.TenantID, filter.CorpID}
	if filter.StartAt != "" {
		where = append(where, "occurred_at >= ?")
		args = append(args, filter.StartAt)
	}
	if filter.EndAt != "" {
		where = append(where, "occurred_at < ?")
		args = append(args, filter.EndAt)
	}
	if filter.RestrictEmployeeIDs && len(filter.EmployeeIDs) > 0 {
		placeholders := make([]string, len(filter.EmployeeIDs))
		for i, id := range filter.EmployeeIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		if table == "mochat_go_timeout_records" {
			where = append(where, "employee_id IN ("+strings.Join(placeholders, ",")+")")
		} else {
			where = append(where, "CAST(SUBSTRING_INDEX(conversation_id, ':', 1) AS UNSIGNED) IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT conversation_id) FROM "+table+" WHERE "+strings.Join(where, " AND "), args...).Scan(&count)
	return count, err
}

func (s *MySQLStore) tableExists(ctx context.Context, name string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`, name).Scan(&count)
	return count > 0, err
}

func setMetric(metrics map[string]dashboard.WorkMessageMetricValue, key string, value int64) {
	metrics[key] = dashboard.WorkMessageMetricValue{Value: int64Ptr(value), Status: "available"}
}

func unavailableMetric(reason string) dashboard.WorkMessageMetricValue {
	return dashboard.WorkMessageMetricValue{Value: nil, Status: "unavailable", Reason: reason}
}

func unavailableGlobalOverview(reason string) dashboard.WorkMessageGlobalOverview {
	return dashboard.WorkMessageGlobalOverview{Metrics: map[string]dashboard.WorkMessageMetricValue{"customerConversations": unavailableMetric(reason), "customerMessages": unavailableMetric(reason), "customerEmployees": unavailableMetric(reason), "roomConversations": unavailableMetric(reason), "roomMessages": unavailableMetric(reason), "roomEmployees": unavailableMetric(reason), "employeeConversations": unavailableMetric(reason), "employeeMessages": unavailableMetric(reason), "riskConversations": unavailableMetric(reason), "timeoutConversations": unavailableMetric(reason)}, Capabilities: []dashboard.WorkMessageCapability{{Key: "archive", Available: false, Reason: reason}}}
}

func int64Ptr(value int64) *int64 { return &value }

func workMessageArchiveID(msgID string, seq int64, tableIndex, id int) string {
	if strings.TrimSpace(msgID) != "" {
		return "msg:" + msgID
	}
	if seq > 0 {
		return fmt.Sprintf("seq:%d", seq)
	}
	return fmt.Sprintf("table:%d:%d", tableIndex, id)
}

func workMessageTargetType(value int) string {
	switch value {
	case 0:
		return "employee"
	case 1:
		return "customer"
	case 2:
		return "room"
	default:
		return "unknown"
	}
}
