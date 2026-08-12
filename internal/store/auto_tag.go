package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

const autoTagKeywordTaskMessagesPerTable = 500

func (s *MySQLStore) AutoTagPage(ctx context.Context, filter dashboard.AutoTagFilter) (dashboard.AutoTagPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := autoTagWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_auto_tag a "+where, args...).Scan(&total); err != nil {
		return dashboard.AutoTagPage{}, err
	}
	totalPage := 0
	if filter.PerPage > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	offset := (filter.Page - 1) * filter.PerPage
	queryArgs := append(append([]any{}, args...), filter.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, autoTagSelectPrefix()+where+`
		ORDER BY a.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.AutoTagPage{}, err
	}
	defer rows.Close()
	items, err := scanAutoTagRows(rows)
	if err != nil {
		return dashboard.AutoTagPage{}, err
	}
	return dashboard.AutoTagPage{Items: items, Total: total, TotalPage: totalPage, Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) AutoTagByID(ctx context.Context, corpID int, id int) (dashboard.AutoTagItem, bool, error) {
	row := s.db.QueryRowContext(ctx, autoTagSelectPrefix()+`
		WHERE a.corp_id = ? AND a.id = ? AND a.deleted_at IS NULL
		LIMIT 1
	`, corpID, id)
	item, err := scanAutoTag(row)
	if err == sql.ErrNoRows {
		return dashboard.AutoTagItem{}, false, nil
	}
	if err != nil {
		return dashboard.AutoTagItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) CreateAutoTag(ctx context.Context, values dashboard.AutoTagWrite) (int, error) {
	tenantID, err := s.tenantIDByCorpID(ctx, values.CorpID)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_auto_tag
			(type, name, employees, fuzzy_match_keyword, exact_match_keyword, tag_rule, tags, on_off, mark_tag_count, tenant_id, corp_id, create_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?)
	`, values.Type, values.Name, jsonOrArray(values.EmployeesRaw), jsonOrArray(values.FuzzyMatchKeywordRaw), jsonOrArray(values.ExactMatchKeywordRaw), jsonOrArray(values.TagRuleRaw), jsonOrArray(values.TagsRaw), values.OnOff, tenantID, values.CorpID, values.CreateUserID, now, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *MySQLStore) UpdateAutoTagOnOff(ctx context.Context, corpID int, id int, onOff int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_auto_tag
		SET on_off = ?, updated_at = ?
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
	`, onOff, time.Now(), corpID, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *MySQLStore) DeleteAutoTag(ctx context.Context, corpID int, id int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_auto_tag
		SET deleted_at = ?, updated_at = ?
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
	`, time.Now(), time.Now(), corpID, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *MySQLStore) AutoTagStatistics(ctx context.Context, corpID int, autoTagID int) (dashboard.AutoTagStatistics, error) {
	var stats dashboard.AutoTagStatistics
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN DATE(created_at) = CURDATE() THEN 1 ELSE 0 END), 0)
		FROM mc_auto_tag_record
		WHERE corp_id = ? AND auto_tag_id = ? AND deleted_at IS NULL
		  AND (status = 1 OR status IS NULL)
	`, corpID, autoTagID).Scan(&stats.TotalCount, &stats.TodayCount)
	return stats, err
}

func (s *MySQLStore) AutoTagRecordPage(ctx context.Context, filter dashboard.AutoTagRecordFilter) (dashboard.AutoTagRecordPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := autoTagRecordWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_auto_tag_record r "+autoTagRecordJoins()+where, args...).Scan(&total); err != nil {
		return dashboard.AutoTagRecordPage{}, err
	}
	totalPage := 0
	if filter.PerPage > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	offset := (filter.Page - 1) * filter.PerPage
	queryArgs := append(append([]any{}, args...), filter.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			r.id,
			r.auto_tag_id,
			COALESCE(r.contact_id, 0),
			COALESCE(NULLIF(c.name, ''), NULLIF(c.nick_name, ''), r.wx_external_userid, ''),
			COALESCE(c.avatar, ''),
			COALESCE(r.tag_rule_id, 0),
			COALESCE(r.wx_external_userid, ''),
			COALESCE(r.employee_id, 0),
			COALESCE(e.name, ''),
			COALESCE(r.keyword, ''),
			COALESCE(r.contact_room_id, 0),
			COALESCE(cr.room_id, 0),
			COALESCE(room.name, ''),
			COALESCE(cr.join_scene, 0),
			cr.join_time,
			COALESCE(CAST(r.tags AS CHAR), '[]'),
			COALESCE(r.corp_id, 0),
			COALESCE(r.trigger_count, 0),
			COALESCE(r.status, 0),
			ce.create_time,
			r.created_at,
			r.updated_at
		FROM mc_auto_tag_record r
		`+autoTagRecordJoins()+where+`
		ORDER BY r.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.AutoTagRecordPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.AutoTagRecordItem, 0)
	for rows.Next() {
		item, err := scanAutoTagRecordRow(rows)
		if err != nil {
			return dashboard.AutoTagRecordPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.AutoTagRecordPage{}, err
	}
	return dashboard.AutoTagRecordPage{Items: items, Total: total, TotalPage: totalPage, Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) AutoTagKeywordTask(ctx context.Context, corpID int) (dashboard.AutoTagKeywordTaskResult, error) {
	var result dashboard.AutoTagKeywordTaskResult
	if corpID <= 0 {
		return result, nil
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_auto_tag
		WHERE corp_id = ? AND type = 1 AND on_off = 1 AND deleted_at IS NULL
	`, corpID).Scan(&result.RuleCount); err != nil {
		return dashboard.AutoTagKeywordTaskResult{}, err
	}
	pending, err := s.countPendingAutoTagMessages(ctx, corpID)
	if err != nil {
		return dashboard.AutoTagKeywordTaskResult{}, err
	}
	result.PendingMessages = pending
	if result.RuleCount == 0 {
		return result, nil
	}
	enqueuedRecords := map[int]struct{}{}
	pendingEvents, err := s.pendingAutoTagRecordEvents(ctx, corpID)
	if err != nil {
		return dashboard.AutoTagKeywordTaskResult{}, err
	}
	for _, event := range pendingEvents {
		if event.AutoTagRecordID > 0 {
			enqueuedRecords[event.AutoTagRecordID] = struct{}{}
		}
		result.MarkTagsEvents = append(result.MarkTagsEvents, event)
	}
	if result.PendingMessages == 0 {
		return result, nil
	}
	rules, err := s.activeAutoTagKeywordRules(ctx, corpID)
	if err != nil {
		return dashboard.AutoTagKeywordTaskResult{}, err
	}
	if len(rules) == 0 {
		return result, nil
	}

	now := time.Now()
	groups := map[autoTagKeywordGroupKey]*autoTagKeywordMatchGroup{}
	processed := map[int]map[int]struct{}{}
	matchedMessages := map[string]struct{}{}

	for index := 1; index <= 10; index++ {
		messages, err := s.pendingAutoTagKeywordMessages(ctx, corpID, index, autoTagKeywordTaskMessagesPerTable)
		if err != nil {
			return dashboard.AutoTagKeywordTaskResult{}, err
		}
		for _, message := range messages {
			text := autoTagKeywordMessageText(message.ContentText, message.ContentRaw)
			if message.ToUserType != 1 || message.ToUserID <= 0 || message.WorkEmployeeID <= 0 || strings.TrimSpace(text) == "" {
				markAutoTagMessageProcessed(processed, message.TableIndex, message.ID)
				continue
			}
			matched := false
			for _, rule := range rules {
				if !rule.matchesEmployee(message.WorkEmployeeID, message.WorkEmployeeWXUserID) {
					continue
				}
				keyword, ok := rule.matchKeyword(text)
				if !ok {
					continue
				}
				matched = true
				matchedMessages[fmt.Sprintf("%d:%d", message.TableIndex, message.ID)] = struct{}{}
				for _, subRule := range rule.TagRules {
					periodStart, periodEnd := autoTagKeywordPeriod(message.MsgDataTime, subRule.TimeType, now)
					key := autoTagKeywordGroupKey{
						AutoTagID:     rule.ID,
						TagRuleID:     subRule.ID,
						ContactID:     message.ToUserID,
						EmployeeID:    message.WorkEmployeeID,
						ContactRoomID: 0,
						PeriodStart:   periodStart.Format(time.RFC3339),
					}
					group := groups[key]
					if group == nil {
						group = &autoTagKeywordMatchGroup{
							Key:              key,
							SubRule:          subRule,
							PeriodStart:      periodStart,
							PeriodEnd:        periodEnd,
							CorpID:           corpID,
							WXExternalUserID: message.WXExternalUserID,
							MessageRefs:      map[autoTagKeywordMessageRef]struct{}{},
						}
						groups[key] = group
					}
					if group.Keyword == "" {
						group.Keyword = keyword
					}
					if group.WXExternalUserID == "" {
						group.WXExternalUserID = message.WXExternalUserID
					}
					group.MessageRefs[autoTagKeywordMessageRef{TableIndex: message.TableIndex, ID: message.ID}] = struct{}{}
				}
			}
			if !matched {
				markAutoTagMessageProcessed(processed, message.TableIndex, message.ID)
			}
		}
	}
	result.MatchedMessages = len(matchedMessages)
	if len(groups) == 0 {
		if err := s.updateAutoTagProcessedMessages(ctx, processed); err != nil {
			return dashboard.AutoTagKeywordTaskResult{}, err
		}
		return result, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.AutoTagKeywordTaskResult{}, err
	}
	defer tx.Rollback()
	for _, group := range groups {
		if len(group.MessageRefs) >= group.SubRule.TriggerCount {
			recordID, status, found, err := autoTagRecordForPeriodTx(ctx, tx, group)
			if err != nil {
				return dashboard.AutoTagKeywordTaskResult{}, err
			}
			if !found {
				recordID, err = insertAutoTagRecordTx(ctx, tx, group)
				if err != nil {
					return dashboard.AutoTagKeywordTaskResult{}, err
				}
				status = 0
				result.CreatedRecords++
			}
			if status != 1 {
				if _, ok := enqueuedRecords[recordID]; !ok {
					result.MarkTagsEvents = append(result.MarkTagsEvents, group.markTagsEvent(recordID))
					enqueuedRecords[recordID] = struct{}{}
				}
			}
			markAutoTagGroupMessagesProcessed(processed, group)
			continue
		}
		if !group.PeriodEnd.After(now) {
			markAutoTagGroupMessagesProcessed(processed, group)
		}
	}
	if err := updateAutoTagProcessedMessagesTx(ctx, tx, processed); err != nil {
		return dashboard.AutoTagKeywordTaskResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.AutoTagKeywordTaskResult{}, err
	}
	return result, nil
}

func (s *MySQLStore) AutoTagRoomJoinTask(ctx context.Context, corpID int, wxChatID string) (dashboard.AutoTagRoomJoinTaskResult, error) {
	var result dashboard.AutoTagRoomJoinTaskResult
	wxChatID = strings.TrimSpace(wxChatID)
	if corpID <= 0 || wxChatID == "" {
		return result, nil
	}
	roomID, found, err := s.autoTagRoomIDByWXChatID(ctx, corpID, wxChatID)
	if err != nil || !found {
		return result, err
	}
	rules, err := s.activeAutoTagRoomJoinRules(ctx, corpID)
	if err != nil {
		return dashboard.AutoTagRoomJoinTaskResult{}, err
	}
	result.RuleCount = len(rules)
	if len(rules) == 0 {
		return result, nil
	}
	members, err := s.autoTagRoomJoinMembers(ctx, corpID, roomID)
	if err != nil {
		return dashboard.AutoTagRoomJoinTaskResult{}, err
	}
	if len(members) == 0 {
		return result, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.AutoTagRoomJoinTaskResult{}, err
	}
	defer tx.Rollback()
	enqueuedRecords := map[int]struct{}{}
	for _, member := range members {
		if member.ContactID <= 0 || member.EmployeeID <= 0 || member.ContactRoomID <= 0 {
			continue
		}
		for _, rule := range rules {
			for _, subRule := range rule.SubRules {
				if !subRule.matchesRoom(roomID) {
					continue
				}
				result.MatchedMembers++
				recordID, status, found, err := autoTagRoomJoinRecordTx(ctx, tx, rule.ID, subRule.ID, member.ContactID, member.EmployeeID, member.ContactRoomID)
				if err != nil {
					return dashboard.AutoTagRoomJoinTaskResult{}, err
				}
				if !found {
					recordID, err = insertAutoTagRoomJoinRecordTx(ctx, tx, corpID, rule.ID, subRule, member)
					if err != nil {
						return dashboard.AutoTagRoomJoinTaskResult{}, err
					}
					status = 0
					result.CreatedRecords++
				}
				if status == 1 {
					continue
				}
				if _, exists := enqueuedRecords[recordID]; exists {
					continue
				}
				result.MarkTagsEvents = append(result.MarkTagsEvents, dashboard.MarkTagsEvent{
					CorpID:          corpID,
					ContactID:       member.ContactID,
					EmployeeID:      member.EmployeeID,
					TagIDs:          append([]int{}, subRule.TagIDs...),
					Source:          fmt.Sprintf("auto-tag-join-room:%d", recordID),
					AutoTagID:       rule.ID,
					AutoTagRecordID: recordID,
				})
				enqueuedRecords[recordID] = struct{}{}
				result.QueuedMarkTags++
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return dashboard.AutoTagRoomJoinTaskResult{}, err
	}
	return result, nil
}

func (s *MySQLStore) AutoTagContactTimeTask(ctx context.Context, corpID int, employeeID int, contactID int) (dashboard.AutoTagContactTimeTaskResult, error) {
	var result dashboard.AutoTagContactTimeTaskResult
	if corpID <= 0 || employeeID <= 0 || contactID <= 0 {
		return result, nil
	}
	target, found, err := s.autoTagContactTimeTarget(ctx, corpID, employeeID, contactID)
	if err != nil || !found {
		return result, err
	}
	if target.AddTime.IsZero() {
		return result, nil
	}
	rules, err := s.activeAutoTagContactTimeRules(ctx, corpID)
	if err != nil {
		return dashboard.AutoTagContactTimeTaskResult{}, err
	}
	result.RuleCount = len(rules)
	if len(rules) == 0 {
		return result, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.AutoTagContactTimeTaskResult{}, err
	}
	defer tx.Rollback()
	enqueuedRecords := map[int]struct{}{}
	for _, rule := range rules {
		if !rule.matchesEmployee(target.EmployeeID, target.EmployeeWXUserID) {
			continue
		}
		for _, subRule := range rule.SubRules {
			if !subRule.matchesAt(target.AddTime) {
				continue
			}
			result.MatchedRules++
			periodStart, periodEnd := autoTagKeywordPeriod(target.AddTime, subRule.TimeType, target.AddTime)
			recordID, status, found, err := autoTagContactTimeRecordTx(ctx, tx, rule.ID, subRule.ID, target.ContactID, target.EmployeeID, periodStart, periodEnd)
			if err != nil {
				return dashboard.AutoTagContactTimeTaskResult{}, err
			}
			if !found {
				recordID, err = insertAutoTagContactTimeRecordTx(ctx, tx, corpID, rule.ID, subRule, target)
				if err != nil {
					return dashboard.AutoTagContactTimeTaskResult{}, err
				}
				status = 0
				result.CreatedRecords++
			}
			if status == 1 {
				continue
			}
			if _, exists := enqueuedRecords[recordID]; exists {
				continue
			}
			result.MarkTagsEvents = append(result.MarkTagsEvents, dashboard.MarkTagsEvent{
				CorpID:          corpID,
				ContactID:       target.ContactID,
				EmployeeID:      target.EmployeeID,
				TagIDs:          append([]int{}, subRule.TagIDs...),
				Source:          fmt.Sprintf("auto-tag-contact-time:%d", recordID),
				AutoTagID:       rule.ID,
				AutoTagRecordID: recordID,
			})
			enqueuedRecords[recordID] = struct{}{}
			result.QueuedMarkTags++
		}
	}
	if err := tx.Commit(); err != nil {
		return dashboard.AutoTagContactTimeTaskResult{}, err
	}
	return result, nil
}

func (s *MySQLStore) autoTagContactTimeTarget(ctx context.Context, corpID int, employeeID int, contactID int) (autoTagContactTimeTarget, bool, error) {
	var target autoTagContactTimeTarget
	var addTime sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT
			rel.contact_id,
			rel.employee_id,
			COALESCE(employee.wx_user_id, ''),
			COALESCE(contact.wx_external_userid, ''),
			COALESCE(rel.create_time, rel.created_at)
		FROM mc_work_contact_employee rel
		INNER JOIN mc_work_contact contact
			ON contact.id = rel.contact_id
			AND contact.corp_id = rel.corp_id
			AND contact.deleted_at IS NULL
		INNER JOIN mc_work_employee employee
			ON employee.id = rel.employee_id
			AND employee.corp_id = rel.corp_id
			AND employee.deleted_at IS NULL
		WHERE rel.corp_id = ?
		  AND rel.employee_id = ?
		  AND rel.contact_id = ?
		  AND rel.status = 1
		  AND rel.deleted_at IS NULL
		ORDER BY rel.id ASC
		LIMIT 1
	`, corpID, employeeID, contactID).Scan(&target.ContactID, &target.EmployeeID, &target.EmployeeWXUserID, &target.WXExternalUserID, &addTime)
	if err == sql.ErrNoRows {
		return autoTagContactTimeTarget{}, false, nil
	}
	if err != nil {
		return autoTagContactTimeTarget{}, false, err
	}
	if addTime.Valid {
		target.AddTime = addTime.Time
	}
	return target, target.ContactID > 0 && target.EmployeeID > 0, nil
}

func (s *MySQLStore) activeAutoTagContactTimeRules(ctx context.Context, corpID int) ([]autoTagContactTimeRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			COALESCE(employees, '[]'),
			COALESCE(CAST(tag_rule AS CHAR), '[]')
		FROM mc_auto_tag
		WHERE corp_id = ? AND type = 3 AND on_off = 1 AND deleted_at IS NULL
		ORDER BY id ASC
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := make([]autoTagContactTimeRule, 0)
	for rows.Next() {
		var id int
		var employeesRaw, tagRuleRaw string
		if err := rows.Scan(&id, &employeesRaw, &tagRuleRaw); err != nil {
			return nil, err
		}
		rule := parseAutoTagContactTimeRule(id, employeesRaw, tagRuleRaw)
		if rule.isExecutable() {
			rules = append(rules, rule)
		}
	}
	return rules, rows.Err()
}

func (s *MySQLStore) autoTagRoomIDByWXChatID(ctx context.Context, corpID int, wxChatID string) (int, bool, error) {
	var roomID int
	err := s.db.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_room
		WHERE corp_id = ? AND wx_chat_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, corpID, wxChatID).Scan(&roomID)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return roomID, roomID > 0, nil
}

func (s *MySQLStore) activeAutoTagRoomJoinRules(ctx context.Context, corpID int) ([]autoTagRoomJoinRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(CAST(tag_rule AS CHAR), '[]')
		FROM mc_auto_tag
		WHERE corp_id = ? AND type = 2 AND on_off = 1 AND deleted_at IS NULL
		ORDER BY id ASC
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := make([]autoTagRoomJoinRule, 0)
	for rows.Next() {
		var id int
		var raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		rule := parseAutoTagRoomJoinRule(id, raw)
		if rule.isExecutable() {
			rules = append(rules, rule)
		}
	}
	return rules, rows.Err()
}

func (s *MySQLStore) autoTagRoomJoinMembers(ctx context.Context, corpID int, roomID int) ([]autoTagRoomJoinMember, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			cr.id,
			cr.contact_id,
			COALESCE(NULLIF(cr.employee_id, 0), ce.employee_id, 0),
			COALESCE(c.wx_external_userid, ''),
			cr.join_time
		FROM mc_work_contact_room cr
		INNER JOIN mc_work_contact c
			ON c.id = cr.contact_id
			AND c.corp_id = ?
			AND c.deleted_at IS NULL
		LEFT JOIN (
			SELECT contact_id, MIN(employee_id) AS employee_id
			FROM mc_work_contact_employee
			WHERE corp_id = ? AND status = 1 AND deleted_at IS NULL
			GROUP BY contact_id
		) ce ON ce.contact_id = cr.contact_id
		WHERE cr.room_id = ?
		  AND cr.type = 2
		  AND cr.status = 1
		  AND cr.contact_id > 0
		  AND cr.deleted_at IS NULL
		ORDER BY cr.id ASC
	`, corpID, corpID, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := make([]autoTagRoomJoinMember, 0)
	for rows.Next() {
		var member autoTagRoomJoinMember
		var joinTime sql.NullTime
		if err := rows.Scan(&member.ContactRoomID, &member.ContactID, &member.EmployeeID, &member.WXExternalUserID, &joinTime); err != nil {
			return nil, err
		}
		if joinTime.Valid {
			member.JoinTime = joinTime.Time
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (s *MySQLStore) countPendingAutoTagMessages(ctx context.Context, corpID int) (int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM (
			SELECT id FROM mc_work_message_1 WHERE corp_id = ? AND status = 0 AND deleted_at IS NULL
			UNION ALL SELECT id FROM mc_work_message_2 WHERE corp_id = ? AND status = 0 AND deleted_at IS NULL
			UNION ALL SELECT id FROM mc_work_message_3 WHERE corp_id = ? AND status = 0 AND deleted_at IS NULL
			UNION ALL SELECT id FROM mc_work_message_4 WHERE corp_id = ? AND status = 0 AND deleted_at IS NULL
			UNION ALL SELECT id FROM mc_work_message_5 WHERE corp_id = ? AND status = 0 AND deleted_at IS NULL
			UNION ALL SELECT id FROM mc_work_message_6 WHERE corp_id = ? AND status = 0 AND deleted_at IS NULL
			UNION ALL SELECT id FROM mc_work_message_7 WHERE corp_id = ? AND status = 0 AND deleted_at IS NULL
			UNION ALL SELECT id FROM mc_work_message_8 WHERE corp_id = ? AND status = 0 AND deleted_at IS NULL
			UNION ALL SELECT id FROM mc_work_message_9 WHERE corp_id = ? AND status = 0 AND deleted_at IS NULL
			UNION ALL SELECT id FROM mc_work_message_10 WHERE corp_id = ? AND status = 0 AND deleted_at IS NULL
		) pending
	`, corpID, corpID, corpID, corpID, corpID, corpID, corpID, corpID, corpID, corpID).Scan(&total)
	return total, err
}

func (s *MySQLStore) pendingAutoTagRecordEvents(ctx context.Context, corpID int) ([]dashboard.MarkTagsEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			r.id,
			r.auto_tag_id,
			r.contact_id,
			r.employee_id,
			COALESCE(CAST(r.tags AS CHAR), '[]')
		FROM mc_auto_tag_record r
		INNER JOIN mc_auto_tag a ON a.id = r.auto_tag_id AND a.deleted_at IS NULL
		WHERE r.corp_id = ?
			AND r.deleted_at IS NULL
			AND COALESCE(r.status, 0) = 0
			AND a.type = 1
			AND a.on_off = 1
		ORDER BY r.id ASC
		LIMIT 1000
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]dashboard.MarkTagsEvent, 0)
	for rows.Next() {
		var recordID, autoTagID, contactID, employeeID int
		var tagsRaw string
		if err := rows.Scan(&recordID, &autoTagID, &contactID, &employeeID, &tagsRaw); err != nil {
			return nil, err
		}
		tagIDs, _ := autoTagKeywordTagIDsAndRaw(autoTagKeywordRawList(tagsRaw))
		tagIDs = uniquePositiveInts(tagIDs)
		if recordID <= 0 || autoTagID <= 0 || contactID <= 0 || employeeID <= 0 || len(tagIDs) == 0 {
			continue
		}
		events = append(events, dashboard.MarkTagsEvent{
			CorpID:          corpID,
			ContactID:       contactID,
			EmployeeID:      employeeID,
			TagIDs:          tagIDs,
			Source:          fmt.Sprintf("auto-tag-keyword:%d", recordID),
			AutoTagID:       autoTagID,
			AutoTagRecordID: recordID,
		})
	}
	return events, rows.Err()
}

func (s *MySQLStore) activeAutoTagKeywordRules(ctx context.Context, corpID int) ([]autoTagKeywordRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			COALESCE(employees, '[]'),
			COALESCE(fuzzy_match_keyword, '[]'),
			COALESCE(exact_match_keyword, '[]'),
			COALESCE(CAST(tag_rule AS CHAR), '[]')
		FROM mc_auto_tag
		WHERE corp_id = ? AND type = 1 AND on_off = 1 AND deleted_at IS NULL
		ORDER BY id ASC
	`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := make([]autoTagKeywordRule, 0)
	for rows.Next() {
		var id int
		var employeesRaw, fuzzyRaw, exactRaw, tagRuleRaw string
		if err := rows.Scan(&id, &employeesRaw, &fuzzyRaw, &exactRaw, &tagRuleRaw); err != nil {
			return nil, err
		}
		rule := parseAutoTagKeywordRule(id, employeesRaw, fuzzyRaw, exactRaw, tagRuleRaw)
		if rule.isExecutable() {
			rules = append(rules, rule)
		}
	}
	return rules, rows.Err()
}

func (s *MySQLStore) pendingAutoTagKeywordMessages(ctx context.Context, corpID int, tableIndex int, limit int) ([]autoTagKeywordMessage, error) {
	table, err := autoTagKeywordMessageTable(tableIndex)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = autoTagKeywordTaskMessagesPerTable
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			wm.id,
			COALESCE(wm.corp_id, 0),
			COALESCE(wm.work_employee_id, 0),
			COALESCE(employee.wx_user_id, ''),
			COALESCE(wm.to_user_type, 0),
			COALESCE(wm.to_user_id, 0),
			COALESCE(wm.sender_type, 0),
			COALESCE(CAST(wm.content AS CHAR), ''),
			COALESCE(wm.content_text, ''),
			COALESCE(wm.room_id, 0),
			wm.msg_data_time,
			COALESCE(contact.wx_external_userid, '')
		FROM `+table+` wm
		LEFT JOIN mc_work_employee employee
			ON employee.id = wm.work_employee_id
			AND employee.corp_id = wm.corp_id
			AND employee.deleted_at IS NULL
		LEFT JOIN mc_work_contact contact
			ON wm.to_user_type = 1
			AND contact.id = wm.to_user_id
			AND contact.corp_id = wm.corp_id
			AND contact.deleted_at IS NULL
		WHERE wm.corp_id = ?
		  AND wm.status = 0
		  AND wm.deleted_at IS NULL
		ORDER BY wm.id ASC
		LIMIT ?
	`, corpID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := make([]autoTagKeywordMessage, 0)
	for rows.Next() {
		var message autoTagKeywordMessage
		var msgDataTime sql.NullTime
		message.TableIndex = tableIndex
		if err := rows.Scan(&message.ID, &message.CorpID, &message.WorkEmployeeID, &message.WorkEmployeeWXUserID, &message.ToUserType, &message.ToUserID, &message.SenderType, &message.ContentRaw, &message.ContentText, &message.RoomID, &msgDataTime, &message.WXExternalUserID); err != nil {
			return nil, err
		}
		if msgDataTime.Valid {
			message.MsgDataTime = msgDataTime.Time
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (s *MySQLStore) updateAutoTagProcessedMessages(ctx context.Context, processed map[int]map[int]struct{}) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := updateAutoTagProcessedMessagesTx(ctx, tx, processed); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) MarkAutoTagRecordApplied(ctx context.Context, recordID int, autoTagID int) error {
	if recordID <= 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if autoTagID <= 0 {
		err = tx.QueryRowContext(ctx, `
			SELECT COALESCE(auto_tag_id, 0)
			FROM mc_auto_tag_record
			WHERE id = ? AND deleted_at IS NULL
			LIMIT 1
		`, recordID).Scan(&autoTagID)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_auto_tag_record
		SET status = 1, updated_at = NOW()
		WHERE id = ?
		  AND auto_tag_id = ?
		  AND deleted_at IS NULL
		  AND COALESCE(status, 0) <> 1
	`, recordID, autoTagID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_auto_tag
			SET mark_tag_count = COALESCE(mark_tag_count, 0) + 1,
			    updated_at = NOW()
			WHERE id = ? AND deleted_at IS NULL
		`, autoTagID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type autoTagKeywordRule struct {
	ID                int
	EmployeeWXUserIDs map[string]struct{}
	EmployeeIDs       map[int]struct{}
	FuzzyKeywords     []string
	ExactKeywords     []string
	TagRules          []autoTagKeywordSubRule
}

type autoTagKeywordSubRule struct {
	ID           int
	TimeType     int
	TriggerCount int
	TagIDs       []int
	TagsRaw      string
}

type autoTagRoomJoinRule struct {
	ID       int
	SubRules []autoTagRoomJoinSubRule
}

type autoTagRoomJoinSubRule struct {
	ID      int
	RoomIDs []int
	TagIDs  []int
	TagsRaw string
}

func (r autoTagRoomJoinRule) isExecutable() bool {
	return r.ID > 0 && len(r.SubRules) > 0
}

func (r autoTagRoomJoinSubRule) matchesRoom(roomID int) bool {
	if roomID <= 0 || len(r.RoomIDs) == 0 {
		return false
	}
	for _, candidate := range r.RoomIDs {
		if candidate == roomID {
			return true
		}
	}
	return false
}

type autoTagRoomJoinMember struct {
	ContactRoomID    int
	ContactID        int
	EmployeeID       int
	WXExternalUserID string
	JoinTime         time.Time
}

type autoTagContactTimeRule struct {
	ID                int
	EmployeeWXUserIDs map[string]struct{}
	EmployeeIDs       map[int]struct{}
	SubRules          []autoTagContactTimeSubRule
}

type autoTagContactTimeSubRule struct {
	ID           int
	TimeType     int
	ScheduleDays map[int]struct{}
	StartSecond  int
	EndSecond    int
	TagIDs       []int
	TagsRaw      string
}

type autoTagContactTimeTarget struct {
	ContactID        int
	EmployeeID       int
	EmployeeWXUserID string
	WXExternalUserID string
	AddTime          time.Time
}

func (r autoTagContactTimeRule) isExecutable() bool {
	return r.ID > 0 && len(r.SubRules) > 0
}

func (r autoTagContactTimeRule) matchesEmployee(employeeID int, wxUserID string) bool {
	if len(r.EmployeeWXUserIDs) == 0 && len(r.EmployeeIDs) == 0 {
		return true
	}
	if employeeID > 0 {
		if _, ok := r.EmployeeIDs[employeeID]; ok {
			return true
		}
	}
	wxUserID = strings.TrimSpace(wxUserID)
	if wxUserID == "" {
		return false
	}
	_, ok := r.EmployeeWXUserIDs[wxUserID]
	return ok
}

func (r autoTagContactTimeSubRule) matchesAt(value time.Time) bool {
	if value.IsZero() {
		return false
	}
	local := value.In(time.Local)
	second := local.Hour()*3600 + local.Minute()*60 + local.Second()
	if r.StartSecond <= r.EndSecond {
		if second < r.StartSecond || second > r.EndSecond {
			return false
		}
	} else if second < r.StartSecond && second > r.EndSecond {
		return false
	}
	switch r.TimeType {
	case 2:
		_, ok := r.ScheduleDays[int(local.Weekday())]
		return ok
	case 3:
		if _, ok := r.ScheduleDays[local.Day()]; ok {
			return true
		}
		if _, ok := r.ScheduleDays[32]; ok {
			return local.AddDate(0, 0, 1).Day() == 1
		}
		return false
	default:
		return true
	}
}

type autoTagKeywordMessage struct {
	TableIndex           int
	ID                   int
	CorpID               int
	WorkEmployeeID       int
	WorkEmployeeWXUserID string
	ToUserType           int
	ToUserID             int
	SenderType           int
	ContentRaw           string
	ContentText          string
	RoomID               int
	MsgDataTime          time.Time
	WXExternalUserID     string
}

type autoTagKeywordGroupKey struct {
	AutoTagID     int
	TagRuleID     int
	ContactID     int
	EmployeeID    int
	ContactRoomID int
	PeriodStart   string
}

type autoTagKeywordMessageRef struct {
	TableIndex int
	ID         int
}

type autoTagKeywordMatchGroup struct {
	Key              autoTagKeywordGroupKey
	SubRule          autoTagKeywordSubRule
	PeriodStart      time.Time
	PeriodEnd        time.Time
	CorpID           int
	WXExternalUserID string
	Keyword          string
	MessageRefs      map[autoTagKeywordMessageRef]struct{}
}

func parseAutoTagKeywordRule(id int, employeesRaw string, fuzzyRaw string, exactRaw string, tagRuleRaw string) autoTagKeywordRule {
	wxUserIDs, employeeIDs := autoTagKeywordEmployeeScope(employeesRaw)
	return autoTagKeywordRule{
		ID:                id,
		EmployeeWXUserIDs: wxUserIDs,
		EmployeeIDs:       employeeIDs,
		FuzzyKeywords:     autoTagKeywordStringList(fuzzyRaw),
		ExactKeywords:     autoTagKeywordStringList(exactRaw),
		TagRules:          autoTagKeywordSubRules(tagRuleRaw),
	}
}

func parseAutoTagRoomJoinRule(id int, tagRuleRaw string) autoTagRoomJoinRule {
	values := autoTagKeywordRawList(tagRuleRaw)
	subRules := make([]autoTagRoomJoinSubRule, 0, len(values))
	for index, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		roomIDs := autoTagRoomJoinRoomIDs(firstAutoTagKeywordMapValue(item, "rooms", "roomIds", "room_ids", "room"))
		roomIDs = uniquePositiveInts(roomIDs)
		tagIDs, tagsRaw := autoTagKeywordTagIDsAndRaw(item["tags"])
		tagIDs = uniquePositiveInts(tagIDs)
		if len(roomIDs) == 0 || len(tagIDs) == 0 {
			continue
		}
		ruleID := autoTagKeywordMapInt(item, "id", "tag_rule_id", "tagRuleId")
		if ruleID <= 0 {
			ruleID = index + 1
		}
		subRules = append(subRules, autoTagRoomJoinSubRule{
			ID:      ruleID,
			RoomIDs: roomIDs,
			TagIDs:  tagIDs,
			TagsRaw: tagsRaw,
		})
	}
	return autoTagRoomJoinRule{ID: id, SubRules: subRules}
}

func parseAutoTagContactTimeRule(id int, employeesRaw string, tagRuleRaw string) autoTagContactTimeRule {
	wxUserIDs, employeeIDs := autoTagKeywordEmployeeScope(employeesRaw)
	values := autoTagKeywordRawList(tagRuleRaw)
	subRules := make([]autoTagContactTimeSubRule, 0, len(values))
	for index, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		tagIDs, tagsRaw := autoTagKeywordTagIDsAndRaw(item["tags"])
		tagIDs = uniquePositiveInts(tagIDs)
		if len(tagIDs) == 0 {
			continue
		}
		startSecond, ok := autoTagContactTimeSecond(firstAutoTagKeywordMapValue(item, "start_time", "startTime", "start"))
		if !ok {
			continue
		}
		endSecond, ok := autoTagContactTimeSecond(firstAutoTagKeywordMapValue(item, "end_time", "endTime", "end"))
		if !ok {
			continue
		}
		timeType := autoTagKeywordMapInt(item, "time_type", "timeType")
		if timeType < 1 || timeType > 3 {
			timeType = 1
		}
		scheduleDays := autoTagContactTimeSchedule(firstAutoTagKeywordMapValue(item, "schedule", "schedules", "days"))
		if (timeType == 2 || timeType == 3) && len(scheduleDays) == 0 {
			continue
		}
		ruleID := autoTagKeywordMapInt(item, "id", "tag_rule_id", "tagRuleId")
		if ruleID <= 0 {
			ruleID = index + 1
		}
		subRules = append(subRules, autoTagContactTimeSubRule{
			ID:           ruleID,
			TimeType:     timeType,
			ScheduleDays: scheduleDays,
			StartSecond:  startSecond,
			EndSecond:    endSecond,
			TagIDs:       tagIDs,
			TagsRaw:      tagsRaw,
		})
	}
	return autoTagContactTimeRule{
		ID:                id,
		EmployeeWXUserIDs: wxUserIDs,
		EmployeeIDs:       employeeIDs,
		SubRules:          subRules,
	}
}

func autoTagRoomJoinRoomIDs(raw any) []int {
	values, ok := raw.([]any)
	if !ok {
		if raw == nil {
			return nil
		}
		values = []any{raw}
	}
	result := make([]int, 0, len(values))
	for _, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			if id := autoTagKeywordMapInt(typed, "id", "room_id", "roomId", "workRoomId", "work_room_id"); id > 0 {
				result = append(result, id)
			}
		default:
			if id := autoTagKeywordIntFromAny(typed); id > 0 {
				result = append(result, id)
			}
		}
	}
	return result
}

func autoTagContactTimeSchedule(raw any) map[int]struct{} {
	values, ok := raw.([]any)
	if !ok {
		if raw == nil {
			return map[int]struct{}{}
		}
		values = []any{raw}
	}
	result := map[int]struct{}{}
	for _, value := range values {
		if day := autoTagKeywordIntFromAny(value); day >= 0 && day <= 32 {
			result[day] = struct{}{}
		}
	}
	return result
}

func autoTagContactTimeSecond(raw any) (int, bool) {
	text := autoTagKeywordStringFromAny(raw)
	if text == "" {
		return 0, false
	}
	parts := strings.Split(text, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	hour, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || hour < 0 || hour > 23 {
		return 0, false
	}
	minute, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || minute < 0 || minute > 59 {
		return 0, false
	}
	second := 0
	if len(parts) == 3 {
		second, err = strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil || second < 0 || second > 59 {
			return 0, false
		}
	}
	return hour*3600 + minute*60 + second, true
}

func (r autoTagKeywordRule) isExecutable() bool {
	return r.ID > 0 && (len(r.FuzzyKeywords) > 0 || len(r.ExactKeywords) > 0) && len(r.TagRules) > 0
}

func (r autoTagKeywordRule) matchesEmployee(employeeID int, wxUserID string) bool {
	if len(r.EmployeeWXUserIDs) == 0 && len(r.EmployeeIDs) == 0 {
		return true
	}
	if employeeID > 0 {
		if _, ok := r.EmployeeIDs[employeeID]; ok {
			return true
		}
	}
	wxUserID = strings.TrimSpace(wxUserID)
	if wxUserID == "" {
		return false
	}
	_, ok := r.EmployeeWXUserIDs[wxUserID]
	return ok
}

func (r autoTagKeywordRule) matchKeyword(text string) (string, bool) {
	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return "", false
	}
	for _, keyword := range r.ExactKeywords {
		if normalized == keyword {
			return keyword, true
		}
	}
	for _, keyword := range r.FuzzyKeywords {
		if strings.Contains(normalized, keyword) {
			return keyword, true
		}
	}
	return "", false
}

func autoTagKeywordEmployeeScope(raw string) (map[string]struct{}, map[int]struct{}) {
	values := autoTagKeywordRawList(raw)
	wxUserIDs := map[string]struct{}{}
	employeeIDs := map[int]struct{}{}
	for _, value := range values {
		switch typed := value.(type) {
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed == "" {
				continue
			}
			if id := autoTagKeywordIntFromAny(trimmed); id > 0 {
				employeeIDs[id] = struct{}{}
				continue
			}
			wxUserIDs[trimmed] = struct{}{}
		case json.Number, float64, int:
			if id := autoTagKeywordIntFromAny(typed); id > 0 {
				employeeIDs[id] = struct{}{}
			}
		case map[string]any:
			if id := autoTagKeywordMapInt(typed, "id", "employee_id", "employeeId"); id > 0 {
				employeeIDs[id] = struct{}{}
			}
			for _, key := range []string{"wxUserId", "wx_user_id", "userid", "userId", "wx_userid", "wxUserid"} {
				if text := autoTagKeywordStringFromAny(typed[key]); text != "" {
					wxUserIDs[text] = struct{}{}
				}
			}
		}
	}
	return wxUserIDs, employeeIDs
}

func autoTagKeywordStringList(raw string) []string {
	values := autoTagKeywordRawList(raw)
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		text := ""
		switch typed := value.(type) {
		case string, json.Number, float64, int:
			text = autoTagKeywordStringFromAny(typed)
		case map[string]any:
			for _, key := range []string{"keyword", "name", "value", "text", "content"} {
				text = autoTagKeywordStringFromAny(typed[key])
				if text != "" {
					break
				}
			}
		}
		if text == "" {
			continue
		}
		if _, ok := seen[text]; ok {
			continue
		}
		seen[text] = struct{}{}
		result = append(result, text)
	}
	return result
}

func autoTagKeywordSubRules(raw string) []autoTagKeywordSubRule {
	values := autoTagKeywordRawList(raw)
	result := make([]autoTagKeywordSubRule, 0, len(values))
	for index, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		tagIDs, tagsRaw := autoTagKeywordTagIDsAndRaw(item["tags"])
		tagIDs = uniquePositiveInts(tagIDs)
		if len(tagIDs) == 0 {
			continue
		}
		timeType := autoTagKeywordMapInt(item, "time_type", "timeType")
		if timeType < 1 || timeType > 3 {
			timeType = 1
		}
		triggerCount := autoTagKeywordMapInt(item, "trigger_count", "triggerCount")
		if triggerCount <= 0 {
			triggerCount = 1
		}
		ruleID := autoTagKeywordMapInt(item, "id", "tag_rule_id", "tagRuleId")
		if ruleID <= 0 {
			ruleID = index + 1
		}
		result = append(result, autoTagKeywordSubRule{
			ID:           ruleID,
			TimeType:     timeType,
			TriggerCount: triggerCount,
			TagIDs:       tagIDs,
			TagsRaw:      tagsRaw,
		})
	}
	return result
}

func autoTagKeywordRawList(raw string) []any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var values []any
	if err := decodeAutoTagKeywordJSON(raw, &values); err == nil {
		return values
	}
	var single any
	if err := decodeAutoTagKeywordJSON(raw, &single); err == nil {
		return []any{single}
	}
	return nil
}

func decodeAutoTagKeywordJSON(raw string, target any) error {
	decoder := json.NewDecoder(bytes.NewReader([]byte(raw)))
	decoder.UseNumber()
	return decoder.Decode(target)
}

func autoTagKeywordTagIDsAndRaw(raw any) ([]int, string) {
	values, ok := raw.([]any)
	if !ok {
		if raw == nil {
			return nil, "[]"
		}
		values = []any{raw}
	}
	ids := make([]int, 0, len(values))
	normalized := make([]any, 0, len(values))
	for _, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			id := autoTagKeywordMapInt(typed, "tagid", "tag_id", "tagId", "id")
			if id <= 0 {
				continue
			}
			ids = append(ids, id)
			if _, ok := typed["tagid"]; !ok {
				typed["tagid"] = id
			}
			if _, ok := typed["tagname"]; !ok {
				if name := autoTagKeywordStringFromAny(firstAutoTagKeywordMapValue(typed, "tag_name", "tagName", "name")); name != "" {
					typed["tagname"] = name
				}
			}
			normalized = append(normalized, typed)
		default:
			id := autoTagKeywordIntFromAny(typed)
			if id <= 0 {
				continue
			}
			ids = append(ids, id)
			normalized = append(normalized, map[string]any{"tagid": id})
		}
	}
	rawJSON, err := json.Marshal(normalized)
	if err != nil {
		return ids, "[]"
	}
	return ids, string(rawJSON)
}

func firstAutoTagKeywordMapValue(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func autoTagKeywordMapInt(values map[string]any, keys ...string) int {
	return autoTagKeywordIntFromAny(firstAutoTagKeywordMapValue(values, keys...))
}

func autoTagKeywordIntFromAny(value any) int {
	switch typed := value.(type) {
	case nil:
		return 0
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		if i, err := typed.Int64(); err == nil {
			return int(i)
		}
		if f, err := typed.Float64(); err == nil {
			return int(f)
		}
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return i
		}
	}
	return 0
}

func autoTagKeywordStringFromAny(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strings.TrimSpace(strconv.FormatFloat(typed, 'f', -1, 64))
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	}
	return ""
}

func autoTagKeywordMessageText(contentText string, contentRaw string) string {
	if text := strings.TrimSpace(contentText); text != "" {
		return text
	}
	contentRaw = strings.TrimSpace(contentRaw)
	if contentRaw == "" {
		return ""
	}
	var decoded any
	if err := decodeAutoTagKeywordJSON(contentRaw, &decoded); err != nil {
		return contentRaw
	}
	return strings.TrimSpace(autoTagKeywordFlattenText(decoded))
}

func autoTagKeywordFlattenText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(autoTagKeywordFlattenText(item)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " ")
	case map[string]any:
		parts := []string{}
		for _, key := range []string{"content", "text", "title", "description", "displayname", "name"} {
			if text := strings.TrimSpace(autoTagKeywordFlattenText(typed[key])); text != "" {
				parts = append(parts, text)
			}
		}
		if itemText := strings.TrimSpace(autoTagKeywordFlattenText(typed["item"])); itemText != "" {
			parts = append(parts, itemText)
		}
		if len(parts) > 0 {
			return strings.Join(parts, " ")
		}
		for _, value := range typed {
			if text := strings.TrimSpace(autoTagKeywordFlattenText(value)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " ")
	}
	return ""
}

func autoTagKeywordPeriod(value time.Time, timeType int, fallback time.Time) (time.Time, time.Time) {
	if value.IsZero() {
		value = fallback
	}
	local := value.In(time.Local)
	switch timeType {
	case 2:
		start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
		offset := (int(start.Weekday()) + 6) % 7
		start = start.AddDate(0, 0, -offset)
		return start, start.AddDate(0, 0, 7)
	case 3:
		start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, local.Location())
		return start, start.AddDate(0, 1, 0)
	default:
		start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
		return start, start.AddDate(0, 0, 1)
	}
}

func autoTagRecordForPeriodTx(ctx context.Context, tx *sql.Tx, group *autoTagKeywordMatchGroup) (int, int, bool, error) {
	var recordID, status int
	err := tx.QueryRowContext(ctx, `
		SELECT id, COALESCE(status, 0)
		FROM mc_auto_tag_record
		WHERE auto_tag_id = ?
		  AND tag_rule_id = ?
		  AND contact_id = ?
		  AND employee_id = ?
		  AND COALESCE(contact_room_id, 0) = ?
		  AND created_at >= ?
		  AND created_at < ?
		  AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, group.Key.AutoTagID, group.Key.TagRuleID, group.Key.ContactID, group.Key.EmployeeID, group.Key.ContactRoomID, group.PeriodStart, group.PeriodEnd).Scan(&recordID, &status)
	if err == sql.ErrNoRows {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return recordID, status, true, nil
}

func insertAutoTagRecordTx(ctx context.Context, tx *sql.Tx, group *autoTagKeywordMatchGroup) (int, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_auto_tag_record
			(auto_tag_id, contact_id, tag_rule_id, wx_external_userid, employee_id,
			 keyword, contact_room_id, tags, corp_id, trigger_count, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, NOW(), NOW())
	`, group.Key.AutoTagID, group.Key.ContactID, group.Key.TagRuleID, strings.TrimSpace(group.WXExternalUserID),
		group.Key.EmployeeID, strings.TrimSpace(group.Keyword), group.Key.ContactRoomID, jsonOrArray(group.SubRule.TagsRaw),
		group.CorpID, len(group.MessageRefs))
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func autoTagRoomJoinRecordTx(ctx context.Context, tx *sql.Tx, autoTagID int, tagRuleID int, contactID int, employeeID int, contactRoomID int) (int, int, bool, error) {
	var recordID, status int
	err := tx.QueryRowContext(ctx, `
		SELECT id, COALESCE(status, 0)
		FROM mc_auto_tag_record
		WHERE auto_tag_id = ?
		  AND tag_rule_id = ?
		  AND contact_id = ?
		  AND employee_id = ?
		  AND COALESCE(contact_room_id, 0) = ?
		  AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, autoTagID, tagRuleID, contactID, employeeID, contactRoomID).Scan(&recordID, &status)
	if err == sql.ErrNoRows {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return recordID, status, true, nil
}

func insertAutoTagRoomJoinRecordTx(ctx context.Context, tx *sql.Tx, corpID int, autoTagID int, subRule autoTagRoomJoinSubRule, member autoTagRoomJoinMember) (int, error) {
	createdAt := time.Now()
	if !member.JoinTime.IsZero() {
		createdAt = member.JoinTime
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_auto_tag_record
			(auto_tag_id, contact_id, tag_rule_id, wx_external_userid, employee_id,
			 keyword, contact_room_id, tags, corp_id, trigger_count, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, '', ?, ?, ?, 1, 0, ?, NOW())
	`, autoTagID, member.ContactID, subRule.ID, strings.TrimSpace(member.WXExternalUserID), member.EmployeeID,
		member.ContactRoomID, jsonOrArray(subRule.TagsRaw), corpID, createdAt)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func autoTagContactTimeRecordTx(ctx context.Context, tx *sql.Tx, autoTagID int, tagRuleID int, contactID int, employeeID int, periodStart time.Time, periodEnd time.Time) (int, int, bool, error) {
	var recordID, status int
	err := tx.QueryRowContext(ctx, `
		SELECT id, COALESCE(status, 0)
		FROM mc_auto_tag_record
		WHERE auto_tag_id = ?
		  AND tag_rule_id = ?
		  AND contact_id = ?
		  AND employee_id = ?
		  AND COALESCE(contact_room_id, 0) = 0
		  AND created_at >= ?
		  AND created_at < ?
		  AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, autoTagID, tagRuleID, contactID, employeeID, periodStart, periodEnd).Scan(&recordID, &status)
	if err == sql.ErrNoRows {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return recordID, status, true, nil
}

func insertAutoTagContactTimeRecordTx(ctx context.Context, tx *sql.Tx, corpID int, autoTagID int, subRule autoTagContactTimeSubRule, target autoTagContactTimeTarget) (int, error) {
	createdAt := time.Now()
	if !target.AddTime.IsZero() {
		createdAt = target.AddTime
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_auto_tag_record
			(auto_tag_id, contact_id, tag_rule_id, wx_external_userid, employee_id,
			 keyword, contact_room_id, tags, corp_id, trigger_count, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, '', 0, ?, ?, 1, 0, ?, NOW())
	`, autoTagID, target.ContactID, subRule.ID, strings.TrimSpace(target.WXExternalUserID), target.EmployeeID,
		jsonOrArray(subRule.TagsRaw), corpID, createdAt)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (g *autoTagKeywordMatchGroup) markTagsEvent(recordID int) dashboard.MarkTagsEvent {
	return dashboard.MarkTagsEvent{
		CorpID:          g.CorpID,
		ContactID:       g.Key.ContactID,
		EmployeeID:      g.Key.EmployeeID,
		TagIDs:          append([]int{}, g.SubRule.TagIDs...),
		Source:          fmt.Sprintf("auto-tag-keyword:%d", recordID),
		AutoTagID:       g.Key.AutoTagID,
		AutoTagRecordID: recordID,
	}
}

func markAutoTagMessageProcessed(processed map[int]map[int]struct{}, tableIndex int, id int) {
	if tableIndex <= 0 || id <= 0 {
		return
	}
	if processed[tableIndex] == nil {
		processed[tableIndex] = map[int]struct{}{}
	}
	processed[tableIndex][id] = struct{}{}
}

func markAutoTagGroupMessagesProcessed(processed map[int]map[int]struct{}, group *autoTagKeywordMatchGroup) {
	for ref := range group.MessageRefs {
		markAutoTagMessageProcessed(processed, ref.TableIndex, ref.ID)
	}
}

func updateAutoTagProcessedMessagesTx(ctx context.Context, tx *sql.Tx, processed map[int]map[int]struct{}) error {
	for tableIndex, refs := range processed {
		if len(refs) == 0 {
			continue
		}
		table, err := autoTagKeywordMessageTable(tableIndex)
		if err != nil {
			return err
		}
		ids := make([]int, 0, len(refs))
		for id := range refs {
			ids = append(ids, id)
		}
		args := intsToAny(ids)
		if _, err := tx.ExecContext(ctx, `
			UPDATE `+table+`
			SET status = 1, updated_at = NOW()
			WHERE id IN (`+placeholders(len(ids))+`)
			  AND status = 0
			  AND deleted_at IS NULL
		`, args...); err != nil {
			return err
		}
	}
	return nil
}

func autoTagKeywordMessageTable(index int) (string, error) {
	if index < 1 || index > 10 {
		return "", fmt.Errorf("invalid work message table index %d", index)
	}
	return fmt.Sprintf("mc_work_message_%d", index), nil
}

func (s *MySQLStore) WorkMessageFromUsers(ctx context.Context, filter dashboard.WorkMessageFromUserFilter) ([]dashboard.WorkMessageFromUser, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 100)
	where := "WHERE corp_id = ? AND deleted_at IS NULL"
	args := []any{filter.CorpID}
	if filter.RestrictEmployeeIDs {
		ids := uniquePositiveInts(filter.EmployeeIDs)
		if len(ids) == 0 {
			return []dashboard.WorkMessageFromUser{}, nil
		}
		where += " AND id IN (" + placeholders(len(ids)) + ")"
		args = append(args, intsToAny(ids)...)
	}
	if strings.TrimSpace(filter.Name) != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+strings.TrimSpace(filter.Name)+"%")
	}
	offset := (filter.Page - 1) * filter.PerPage
	args = append(args, filter.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(name, ''), COALESCE(avatar, '')
		FROM mc_work_employee
		`+where+`
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.WorkMessageFromUser, 0)
	for rows.Next() {
		var item dashboard.WorkMessageFromUser
		if err := rows.Scan(&item.ID, &item.Name, &item.Avatar); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) WorkMessageToUsers(ctx context.Context, filter dashboard.WorkMessageUserFilter) (dashboard.WorkMessageToUserPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	sourceSQL, sourceArgs := workMessageFilteredUnionSQL(filter)
	whereSQL, filterArgs := workMessageUserWhere(filter)
	args := append(append([]any{}, sourceArgs...), filterArgs...)
	if strings.TrimSpace(filter.Name) != "" {
		whereSQL += ` AND target_name LIKE ? ESCAPE '\\'`
		args = append(args, workMessageLikePattern(filter.Name))
	}
	var total int
	groupColumns, groupKey := workMessageConversationGrouping()
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM (
			SELECT `+groupColumns+`, MAX(id) AS last_id
			FROM (`+sourceSQL+`) wm
			WHERE `+whereSQL+`
			GROUP BY `+groupColumns+`
		) x
	`, args...).Scan(&total); err != nil {
		return dashboard.WorkMessageToUserPage{}, err
	}
	totalPage := 0
	if filter.PerPage > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	offset := (filter.Page - 1) * filter.PerPage
	queryArgs := append(append([]any{}, args...), filter.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, table_index, seq, msgid, work_employee_id, employee_name, employee_avatar, to_user_type, to_user_id, target_name, target_alias, target_avatar, content_text, msg_data_time
		FROM (
			SELECT wm.*,
			       @rn := IF(@grp = `+groupKey+`, @rn + 1, 1) AS rn,
			       @grp := `+groupKey+` AS grp
			FROM (`+sourceSQL+`) wm
			CROSS JOIN (SELECT @rn := 0, @grp := '') vars
			WHERE `+whereSQL+`
			ORDER BY wm.work_employee_id, wm.to_user_type, wm.to_user_id,
			         wm.msg_data_time DESC, wm.seq DESC, wm.table_index DESC, wm.id DESC
		) ranked
		WHERE rn = 1
		ORDER BY msg_data_time DESC, seq DESC, table_index DESC, id DESC,
		         work_employee_id DESC, to_user_type DESC, to_user_id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.WorkMessageToUserPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.WorkMessageToUser, 0)
	for rows.Next() {
		item, err := scanWorkMessageToUserRow(rows)
		if err != nil {
			return dashboard.WorkMessageToUserPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkMessageToUserPage{}, err
	}
	return dashboard.WorkMessageToUserPage{Items: items, Total: total, TotalPage: totalPage, Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) WorkMessageArchiveAuthorized(ctx context.Context, tenantID int, corpID int) (bool, error) {
	var allowed bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM mc_corp
			WHERE id = ?
			  AND tenant_id = ?
			  AND chat_status = 1
			  AND deleted_at IS NULL
		)
	`, corpID, tenantID).Scan(&allowed)
	return allowed, err
}

func (s *MySQLStore) WorkMessageByArchiveID(ctx context.Context, filter dashboard.WorkMessageArchiveFilter) (dashboard.WorkMessageItem, bool, error) {
	if filter.RestrictEmployeeIDs && len(uniquePositiveInts(filter.EmployeeIDs)) == 0 {
		return dashboard.WorkMessageItem{}, false, nil
	}
	sourceSQL, sourceArgs, idWhere, idArgs, ok := workMessageArchiveSource(filter.CorpID, filter.ArchiveMessageID)
	if !ok {
		return dashboard.WorkMessageItem{}, false, nil
	}
	where := []string{idWhere}
	args := append(append([]any{}, sourceArgs...), idArgs...)
	if filter.RestrictEmployeeIDs {
		ids := uniquePositiveInts(filter.EmployeeIDs)
		where = append(where, "work_employee_id IN ("+placeholders(len(ids))+")")
		args = append(args, intsToAny(ids)...)
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, table_index, seq, msgid, work_employee_id, employee_name, employee_avatar, to_user_type, to_user_id,
		       target_name, target_avatar, action, sender_name, sender_avatar, is_current_user,
		       msg_type, content_raw, msg_data_time
		FROM (`+sourceSQL+`) wm
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY msg_data_time DESC, seq DESC, table_index DESC, id DESC
		LIMIT 1
	`, args...)
	item, err := scanWorkMessage(row)
	if err == sql.ErrNoRows {
		return dashboard.WorkMessageItem{}, false, nil
	}
	if err != nil {
		return dashboard.WorkMessageItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) WorkMessagePage(ctx context.Context, filter dashboard.WorkMessageFilter) (dashboard.WorkMessagePage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	sourceSQL, sourceArgs := workMessageUnionSQL(filter.CorpID)
	args := append([]any{}, sourceArgs...)
	where := []string{}
	if filter.RestrictEmployeeIDs {
		ids := uniquePositiveInts(filter.EmployeeIDs)
		if len(ids) == 0 {
			return dashboard.WorkMessagePage{}, nil
		}
		where = append(where, "work_employee_id IN ("+placeholders(len(ids))+")")
		args = append(args, intsToAny(ids)...)
		if filter.WorkEmployeeID > 0 {
			where = append(where, "work_employee_id = ?")
			args = append(args, filter.WorkEmployeeID)
		}
	} else {
		where = append(where, "work_employee_id = ?")
		args = append(args, filter.WorkEmployeeID)
	}
	if filter.Type > 0 {
		where = append(where, "msg_type = ?")
		args = append(args, filter.Type)
	}
	if filter.ToUserType >= 0 {
		where = append(where, "to_user_type = ?")
		args = append(args, filter.ToUserType)
	}
	if filter.ToUserID > 0 {
		where = append(where, "to_user_id = ?")
		args = append(args, filter.ToUserID)
	}
	if strings.TrimSpace(filter.Content) != "" {
		where = append(where, "content_text LIKE ?")
		args = append(args, "%"+strings.TrimSpace(filter.Content)+"%")
	}
	if filter.DateTimeStart != "" {
		where = append(where, "msg_data_time >= ?")
		args = append(args, filter.DateTimeStart)
	}
	if filter.DateTimeEnd != "" {
		where = append(where, "msg_data_time <= ?")
		args = append(args, filter.DateTimeEnd)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ("+sourceSQL+") wm WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return dashboard.WorkMessagePage{}, err
	}
	totalPage := 0
	if filter.PerPage > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	orderSQL, offset, reverse := workMessagePageWindow(filter)
	queryArgs := append(append([]any{}, args...), filter.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, table_index, seq, msgid, work_employee_id, employee_name, employee_avatar, to_user_type, to_user_id,
		       target_name, target_avatar, action, sender_name, sender_avatar, is_current_user,
		       msg_type, content_raw, msg_data_time
		FROM (`+sourceSQL+`) wm
		WHERE `+whereSQL+`
		ORDER BY `+orderSQL+`
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.WorkMessagePage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.WorkMessageItem, 0)
	for rows.Next() {
		item, err := scanWorkMessageRow(rows)
		if err != nil {
			return dashboard.WorkMessagePage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkMessagePage{}, err
	}
	if reverse {
		reverseWorkMessageItems(items)
	}
	return dashboard.WorkMessagePage{Items: items, Total: total, TotalPage: totalPage, Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) WorkMessageConfigByCorp(ctx context.Context, corpID int) (dashboard.WorkMessageConfigItem, bool, error) {
	row := s.db.QueryRowContext(ctx, workMessageConfigSelect()+`
		WHERE c.id = ? AND c.deleted_at IS NULL
		LIMIT 1
	`, corpID)
	item, err := s.scanWorkMessageConfig(row)
	if err == sql.ErrNoRows {
		return dashboard.WorkMessageConfigItem{}, false, nil
	}
	if err != nil {
		return dashboard.WorkMessageConfigItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) WorkMessageConfigPage(ctx context.Context, corpID int, name string, page int, perPage int) (dashboard.WorkMessageConfigPage, error) {
	page = positivePage(page)
	perPage = positivePerPage(perPage, 10)
	where := "WHERE c.deleted_at IS NULL"
	args := []any{}
	if corpID > 0 {
		where += " AND c.id = ?"
		args = append(args, corpID)
	}
	if strings.TrimSpace(name) != "" {
		where += " AND c.name LIKE ?"
		args = append(args, "%"+strings.TrimSpace(name)+"%")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_corp c "+where, args...).Scan(&total); err != nil {
		return dashboard.WorkMessageConfigPage{}, err
	}
	totalPage := 0
	if perPage > 0 {
		totalPage = (total + perPage - 1) / perPage
	}
	offset := (page - 1) * perPage
	queryArgs := append(append([]any{}, args...), perPage, offset)
	rows, err := s.db.QueryContext(ctx, workMessageConfigSelect()+where+`
		ORDER BY c.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.WorkMessageConfigPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.WorkMessageConfigItem, 0)
	for rows.Next() {
		item, err := s.scanWorkMessageConfig(rows)
		if err != nil {
			return dashboard.WorkMessageConfigPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkMessageConfigPage{}, err
	}
	return dashboard.WorkMessageConfigPage{Items: items, Total: total, TotalPage: totalPage, Page: page, PerPage: perPage}, nil
}

func (s *MySQLStore) UpsertWorkMessageCorpConfig(ctx context.Context, corpID int, values dashboard.WorkMessageConfigItem) (int, error) {
	setSQL, args := workMessageConfigSets(values)
	if len(setSQL) == 0 {
		return corpID, nil
	}
	args = append(args, time.Now(), corpID)
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_corp
		SET `+strings.Join(append(setSQL, "updated_at = ?"), ", ")+`
		WHERE id = ? AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if affected == 0 {
		return 0, sql.ErrNoRows
	}
	return corpID, nil
}

func (s *MySQLStore) UpdateWorkMessageStepConfig(ctx context.Context, corpID int, values dashboard.WorkMessageConfigItem) (bool, error) {
	setSQL, args := workMessageConfigStepSets(values)
	chatSecret := strings.TrimSpace(values.ChatSecret)
	if len(setSQL) == 0 && chatSecret == "" {
		return true, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)
	if chatSecret != "" {
		current, found, err := s.loadCorpCredentialByID(ctx, tx, corpID, true)
		if err != nil {
			return false, err
		}
		if !found {
			return false, nil
		}
		credential, err := s.decodeCorpCredential(current)
		if err != nil {
			return false, err
		}
		credential.ChatSecret = chatSecret
		storage, err := s.encodeCorpCredential(current.TenantID, current.WXCorpID, credential)
		if err != nil {
			return false, err
		}
		setSQL = append(setSQL,
			"wecom_credentials_ciphertext = ?", "wecom_credentials_key_id = ?",
		)
		args = append(args, storage.Ciphertext, storage.KeyID)
	}
	args = append(args, time.Now(), corpID)
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_corp
		SET `+strings.Join(append(setSQL, "updated_at = ?"), ", ")+`
		WHERE id = ? AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func autoTagSelectPrefix() string {
	return `
		SELECT
			a.id,
			COALESCE(a.type, 0),
			COALESCE(a.name, ''),
			COALESCE(a.employees, '[]'),
			COALESCE(a.fuzzy_match_keyword, '[]'),
			COALESCE(a.exact_match_keyword, '[]'),
			COALESCE(CAST(a.tag_rule AS CHAR), '[]'),
			COALESCE(a.tags, '[]'),
			COALESCE(a.on_off, 1),
			COALESCE(a.mark_tag_count, 0),
			COALESCE(a.tenant_id, 0),
			COALESCE(a.corp_id, 0),
			COALESCE(a.create_user_id, 0),
			COALESCE(u.name, ''),
			a.created_at,
			a.updated_at
		FROM mc_auto_tag a
		LEFT JOIN mc_user u ON u.id = a.create_user_id AND u.deleted_at IS NULL
	`
}

func autoTagWhere(filter dashboard.AutoTagFilter) (string, []any) {
	where := "WHERE a.corp_id = ? AND a.deleted_at IS NULL"
	args := []any{filter.CorpID}
	if filter.Type > 0 {
		where += " AND a.type = ?"
		args = append(args, filter.Type)
	}
	if strings.TrimSpace(filter.Name) != "" {
		where += " AND a.name LIKE ?"
		args = append(args, "%"+strings.TrimSpace(filter.Name)+"%")
	}
	for _, tagID := range uniquePositiveInts(filter.Tags) {
		where += " AND a.tag_rule LIKE ?"
		args = append(args, "%"+strconvInt(tagID)+"%")
	}
	return where, args
}

func scanAutoTagRows(rows *sql.Rows) ([]dashboard.AutoTagItem, error) {
	items := make([]dashboard.AutoTagItem, 0)
	for rows.Next() {
		item, err := scanAutoTag(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAutoTag(scanner rowScanner) (dashboard.AutoTagItem, error) {
	var item dashboard.AutoTagItem
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.Type, &item.Name, &item.EmployeesRaw, &item.FuzzyMatchKeywordRaw, &item.ExactMatchKeywordRaw, &item.TagRuleRaw, &item.TagsRaw, &item.OnOff, &item.MarkTagCount, &item.TenantID, &item.CorpID, &item.CreateUserID, &item.CreateUserName, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.AutoTagItem{}, err
	}
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func autoTagRecordJoins() string {
	return `
		LEFT JOIN mc_work_contact c ON c.id = r.contact_id AND c.deleted_at IS NULL
		LEFT JOIN mc_work_employee e ON e.id = r.employee_id AND e.deleted_at IS NULL
		LEFT JOIN mc_work_contact_employee ce ON ce.contact_id = r.contact_id AND ce.employee_id = r.employee_id AND ce.deleted_at IS NULL
		LEFT JOIN mc_work_contact_room cr ON cr.id = r.contact_room_id AND cr.deleted_at IS NULL
		LEFT JOIN mc_work_room room ON room.id = cr.room_id AND room.deleted_at IS NULL
	`
}

func autoTagRecordWhere(filter dashboard.AutoTagRecordFilter) (string, []any) {
	where := "WHERE r.corp_id = ? AND r.deleted_at IS NULL AND (r.status = 1 OR r.status IS NULL)"
	args := []any{filter.CorpID}
	if filter.AutoTagID > 0 {
		where += " AND r.auto_tag_id = ?"
		args = append(args, filter.AutoTagID)
	}
	if strings.TrimSpace(filter.ContactName) != "" {
		where += " AND (c.name LIKE ? OR c.nick_name LIKE ? OR r.wx_external_userid LIKE ?)"
		like := "%" + strings.TrimSpace(filter.ContactName) + "%"
		args = append(args, like, like, like)
	}
	if strings.TrimSpace(filter.EmployeeName) != "" {
		where += " AND e.name LIKE ?"
		args = append(args, "%"+strings.TrimSpace(filter.EmployeeName)+"%")
	}
	if strings.TrimSpace(filter.RoomName) != "" {
		where += " AND room.name LIKE ?"
		args = append(args, "%"+strings.TrimSpace(filter.RoomName)+"%")
	}
	if filter.JoinScene > 0 {
		where += " AND cr.join_scene = ?"
		args = append(args, filter.JoinScene)
	}
	if filter.DateTimeStart != "" {
		where += " AND r.created_at >= ?"
		args = append(args, filter.DateTimeStart)
	}
	if filter.DateTimeEnd != "" {
		where += " AND r.created_at <= ?"
		args = append(args, filter.DateTimeEnd)
	}
	return where, args
}

func scanAutoTagRecordRow(rows *sql.Rows) (dashboard.AutoTagRecordItem, error) {
	var item dashboard.AutoTagRecordItem
	var joinTime, contactCreatedAt, createdAt, updatedAt sql.NullTime
	err := rows.Scan(&item.ID, &item.AutoTagID, &item.ContactID, &item.ContactName, &item.ContactAvatar, &item.TagRuleID, &item.WXExternalUserID, &item.EmployeeID, &item.EmployeeName, &item.Keyword, &item.ContactRoomID, &item.RoomID, &item.RoomName, &item.JoinScene, &joinTime, &item.TagsRaw, &item.CorpID, &item.TriggerCount, &item.Status, &contactCreatedAt, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.AutoTagRecordItem{}, err
	}
	item.JoinTime = formatTime(joinTime)
	item.ContactCreatedAt = formatTime(contactCreatedAt)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func workMessageUnionSQL(corpID int) (string, []any) {
	selects := make([]string, 0, 10)
	args := make([]any, 0, 10)
	for index := 1; index <= 10; index++ {
		selects = append(selects, workMessageTableSQL(corpID, index))
		args = append(args, corpID)
	}
	return strings.Join(selects, " UNION ALL "), args
}

func workMessageFilteredUnionSQL(filter dashboard.WorkMessageUserFilter) (string, []any) {
	where, filterArgs := workMessageUserBaseWhere(filter, "wm.")
	selects := make([]string, 0, dashboard.WorkMessageArchiveMessageTableCount)
	args := make([]any, 0, dashboard.WorkMessageArchiveMessageTableCount*(len(filterArgs)+1))
	for index := 1; index <= dashboard.WorkMessageArchiveMessageTableCount; index++ {
		selects = append(selects, workMessageTableSQLWithWhere(filter.CorpID, index, where))
		args = append(args, filter.CorpID)
		args = append(args, filterArgs...)
	}
	return strings.Join(selects, " UNION ALL "), args
}

func workMessageTableSQL(_ int, index int) string {
	return workMessageTableSQLWithWhere(0, index, "")
}

func workMessageTableSQLWithWhere(_ int, index int, where string) string {
	table := fmt.Sprintf("mc_work_message_%d", index)
	extraWhere := ""
	if strings.TrimSpace(where) != "" && strings.TrimSpace(where) != "1 = 1" {
		extraWhere = " AND (" + where + ")"
	}
	return `
		SELECT
			wm.id,
			` + strconv.Itoa(index) + ` AS table_index,
			COALESCE(wm.seq, 0) AS seq,
			COALESCE(wm.msgid, '') AS msgid,
			COALESCE(wm.corp_id, 0) AS corp_id,
			COALESCE(wm.work_employee_id, 0) AS work_employee_id,
			COALESCE(wm.to_user_type, 0) AS to_user_type,
			COALESCE(wm.to_user_id, 0) AS to_user_id,
			COALESCE(wm.action, 0) AS action,
			COALESCE(wm.msg_type, wm.type, 100) AS msg_type,
			COALESCE(CAST(wm.content AS CHAR), '') AS content_raw,
			COALESCE(wm.content_text, '') AS content_text,
			wm.msg_data_time,
			COALESCE(sender.name, '') AS employee_name,
			COALESCE(sender.avatar, '') AS employee_avatar,
			CASE
				WHEN COALESCE(wm.sender_type, 0) = 0 THEN COALESCE(sender.name, '')
				WHEN COALESCE(wm.to_user_type, 0) = 2 THEN '群成员'
				ELSE COALESCE(target_employee.name, target_contact.name, target_room.name, '')
			END AS sender_name,
			CASE
				WHEN COALESCE(wm.sender_type, 0) = 0 THEN COALESCE(sender.avatar, '')
				ELSE COALESCE(target_employee.avatar, target_contact.avatar, '')
			END AS sender_avatar,
			CASE WHEN COALESCE(wm.sender_type, 0) = 0 THEN 1 ELSE 0 END AS is_current_user,
			COALESCE(target_employee.name, target_contact.name, target_room.name, '') AS target_name,
			COALESCE(target_employee.alias, '', '') AS target_alias,
			COALESCE(target_employee.avatar, target_contact.avatar, '') AS target_avatar
		FROM ` + table + ` wm
		LEFT JOIN mc_work_employee sender ON sender.id = wm.work_employee_id AND sender.corp_id = wm.corp_id AND sender.deleted_at IS NULL
		LEFT JOIN mc_work_employee target_employee ON wm.to_user_type = 0 AND target_employee.id = wm.to_user_id AND target_employee.corp_id = wm.corp_id AND target_employee.deleted_at IS NULL
		LEFT JOIN mc_work_contact target_contact ON wm.to_user_type = 1 AND target_contact.id = wm.to_user_id AND target_contact.corp_id = wm.corp_id AND target_contact.deleted_at IS NULL
		LEFT JOIN mc_work_room target_room ON wm.to_user_type = 2 AND target_room.id = wm.to_user_id AND target_room.corp_id = wm.corp_id AND target_room.deleted_at IS NULL
		WHERE wm.corp_id = ? AND wm.deleted_at IS NULL` + extraWhere + `
	`
}

func scanWorkMessageToUserRow(rows *sql.Rows) (dashboard.WorkMessageToUser, error) {
	var item dashboard.WorkMessageToUser
	var msgDataTime sql.NullTime
	var employeeName, employeeAvatar, name, alias, avatar, content sql.NullString
	err := rows.Scan(&item.ID, &item.TableIndex, &item.Seq, &item.MsgID,
		&item.WorkEmployeeID, &employeeName, &employeeAvatar, &item.ToUserType, &item.ToUserID,
		&name, &alias, &avatar, &content, &msgDataTime)
	if err != nil {
		return dashboard.WorkMessageToUser{}, err
	}
	item.EmployeeName = nullString(employeeName)
	item.EmployeeAvatar = nullString(employeeAvatar)
	item.Name = nullString(name)
	item.Alias = nullString(alias)
	item.Avatar = nullString(avatar)
	item.Content = nullString(content)
	item.MsgDataTime = formatTime(msgDataTime)
	return item, nil
}

func scanWorkMessageRow(rows *sql.Rows) (dashboard.WorkMessageItem, error) {
	return scanWorkMessage(rows)
}

func scanWorkMessage(scanner rowScanner) (dashboard.WorkMessageItem, error) {
	var item dashboard.WorkMessageItem
	var msgDataTime sql.NullTime
	var employeeName, employeeAvatar, targetName, targetAvatar, name, avatar, content sql.NullString
	err := scanner.Scan(&item.ID, &item.TableIndex, &item.Seq, &item.MsgID,
		&item.WorkEmployeeID, &employeeName, &employeeAvatar,
		&item.ToUserType, &item.ToUserID, &targetName, &targetAvatar,
		&item.Action, &name, &avatar, &item.IsCurrentUser, &item.Type, &content, &msgDataTime)
	if err != nil {
		return dashboard.WorkMessageItem{}, err
	}
	item.EmployeeName = nullString(employeeName)
	item.EmployeeAvatar = nullString(employeeAvatar)
	item.TargetName = nullString(targetName)
	item.TargetAvatar = nullString(targetAvatar)
	item.Name = nullString(name)
	item.Avatar = nullString(avatar)
	item.ContentRaw = nullString(content)
	item.MsgDataTime = formatTime(msgDataTime)
	return item, nil
}

func workMessageUserWhere(filter dashboard.WorkMessageUserFilter) (string, []any) {
	baseWhere, args := workMessageUserBaseWhere(filter, "")
	if baseWhere == "1 = 0" {
		return baseWhere, args
	}
	where := make([]string, 0, 2)
	if baseWhere != "1 = 1" {
		where = append(where, baseWhere)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		where = append(where, `(employee_name LIKE ? ESCAPE '\\' OR sender_name LIKE ? ESCAPE '\\' OR target_name LIKE ? ESCAPE '\\' OR content_text LIKE ? ESCAPE '\\')`)
		like := workMessageLikePattern(keyword)
		args = append(args, like, like, like, like)
	}
	if len(where) == 0 {
		return "1 = 1", args
	}
	return strings.Join(where, " AND "), args
}

func workMessageUserBaseWhere(filter dashboard.WorkMessageUserFilter, prefix string) (string, []any) {
	if filter.RestrictEmployeeIDs && len(uniquePositiveInts(filter.EmployeeIDs)) == 0 {
		return "1 = 0", nil
	}
	column := func(name string) string { return prefix + name }
	where := make([]string, 0, 7)
	args := make([]any, 0, 8)
	if (!filter.AllowAllEmployees && !filter.RestrictEmployeeIDs) || filter.WorkEmployeeID > 0 {
		where = append(where, column("work_employee_id")+" = ?")
		args = append(args, filter.WorkEmployeeID)
	}
	if filter.ToUserType >= 0 {
		where = append(where, column("to_user_type")+" = ?")
		args = append(args, filter.ToUserType)
	}
	if filter.ToUserID > 0 {
		where = append(where, column("to_user_id")+" = ?")
		args = append(args, filter.ToUserID)
	}
	if filter.DateTimeStart != "" {
		where = append(where, column("msg_data_time")+" >= ?")
		args = append(args, filter.DateTimeStart)
	}
	if filter.DateTimeEnd != "" {
		where = append(where, column("msg_data_time")+" < ?")
		args = append(args, filter.DateTimeEnd)
	}
	if filter.RestrictEmployeeIDs {
		ids := uniquePositiveInts(filter.EmployeeIDs)
		where = append(where, column("work_employee_id")+" IN ("+placeholders(len(ids))+")")
		args = append(args, intsToAny(ids)...)
	}
	if len(where) == 0 {
		return "1 = 1", args
	}
	return strings.Join(where, " AND "), args
}

func workMessageLikePattern(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return "%" + value + "%"
}

func workMessageArchiveSource(corpID int, archiveMessageID string) (string, []any, string, []any, bool) {
	archiveMessageID = strings.TrimSpace(archiveMessageID)
	if strings.HasPrefix(archiveMessageID, "msg:") {
		msgID := strings.TrimSpace(strings.TrimPrefix(archiveMessageID, "msg:"))
		if msgID == "" {
			return "", nil, "", nil, false
		}
		selects := make([]string, 0, dashboard.WorkMessageArchiveMessageTableCount)
		args := make([]any, 0, dashboard.WorkMessageArchiveMessageTableCount*2)
		for tableIndex := 1; tableIndex <= dashboard.WorkMessageArchiveMessageTableCount; tableIndex++ {
			selects = append(selects, workMessageTableSQLWithWhere(corpID, tableIndex, "wm.msgid = ?"))
			args = append(args, corpID, msgID)
		}
		return strings.Join(selects, " UNION ALL "), args, "1 = 1", nil, true
	}
	if strings.HasPrefix(archiveMessageID, "seq:") {
		seq, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(archiveMessageID, "seq:")), 10, 64)
		if err != nil || seq <= 0 {
			return "", nil, "", nil, false
		}
		tableIndex := int((seq-1)%dashboard.WorkMessageArchiveMessageTableCount) + 1
		return workMessageTableSQLWithWhere(corpID, tableIndex, "wm.seq = ?"), []any{corpID, seq}, "1 = 1", nil, true
	}
	parts := strings.Split(archiveMessageID, ":")
	if len(parts) != 3 || parts[0] != "table" {
		return "", nil, "", nil, false
	}
	tableIndex, tableErr := strconv.Atoi(parts[1])
	id, idErr := strconv.Atoi(parts[2])
	if tableErr != nil || idErr != nil || tableIndex < 1 || tableIndex > 10 || id <= 0 {
		return "", nil, "", nil, false
	}
	return workMessageTableSQLWithWhere(corpID, tableIndex, "wm.id = ?"), []any{corpID, id}, "1 = 1", nil, true
}

func workMessageConversationGrouping() (string, string) {
	return "work_employee_id, to_user_type, to_user_id",
		"CONCAT(wm.work_employee_id, ':', wm.to_user_type, ':', wm.to_user_id)"
}

func workMessagePageWindow(filter dashboard.WorkMessageFilter) (string, int, bool) {
	if filter.Latest {
		return "msg_data_time DESC, seq DESC, table_index DESC, id DESC", 0, true
	}
	return "msg_data_time ASC, seq ASC, table_index ASC, id ASC",
		(filter.Page - 1) * filter.PerPage, false
}

func reverseWorkMessageItems(items []dashboard.WorkMessageItem) {
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
}

func workMessageConfigSelect() string {
	return `
		SELECT
			c.id,
			c.id,
			COALESCE(c.name, ''),
			COALESCE(c.wx_corpid, ''),
			COALESCE(c.social_code, ''),
			COALESCE(c.chat_admin, ''),
			COALESCE(c.chat_admin_phone, ''),
			COALESCE(c.chat_admin_idcard, ''),
			COALESCE(c.chat_apply_status, 0),
			COALESCE(c.chat_status, 0),
			COALESCE(c.tenant_id, 0),
			COALESCE(c.wecom_credentials_ciphertext, ''),
			COALESCE(c.wecom_credentials_key_id, ''),
			COALESCE(c.service_contact_url, ''),
			COALESCE(CAST(c.chat_whitelist_ip AS CHAR), '[]'),
			COALESCE(CAST(c.chat_rsa_key AS CHAR), '{}'),
			c.created_at,
			c.updated_at
		FROM mc_corp c
	`
}

func (s *MySQLStore) scanWorkMessageConfig(scanner rowScanner) (dashboard.WorkMessageConfigItem, error) {
	var item dashboard.WorkMessageConfigItem
	var createdAt, updatedAt sql.NullTime
	var ciphertext, keyID string
	var tenantID int
	err := scanner.Scan(&item.ID, &item.CorpID, &item.CorpName, &item.WXCorpID, &item.SocialCode, &item.ChatAdmin, &item.ChatAdminPhone, &item.ChatAdminIDCard, &item.ChatApplyStatus, &item.ChatStatus,
		&tenantID, &ciphertext, &keyID, &item.ServiceContactURL, &item.ChatWhitelistIPRaw, &item.ChatRSAKeyRaw, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.WorkMessageConfigItem{}, err
	}
	credential, err := s.decodeCorpCredential(corpCredentialRecord{
		ID: item.CorpID, TenantID: tenantID, WXCorpID: item.WXCorpID,
		Ciphertext: ciphertext, KeyID: keyID,
	})
	if err != nil {
		return dashboard.WorkMessageConfigItem{}, err
	}
	item.ChatSecret = credential.ChatSecret
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func workMessageConfigSets(values dashboard.WorkMessageConfigItem) ([]string, []any) {
	sets := []string{}
	args := []any{}
	if strings.TrimSpace(values.SocialCode) != "" {
		sets = append(sets, "social_code = ?")
		args = append(args, values.SocialCode)
	}
	if strings.TrimSpace(values.ChatAdmin) != "" {
		sets = append(sets, "chat_admin = ?")
		args = append(args, values.ChatAdmin)
	}
	if strings.TrimSpace(values.ChatAdminPhone) != "" {
		sets = append(sets, "chat_admin_phone = ?")
		args = append(args, values.ChatAdminPhone)
	}
	if strings.TrimSpace(values.ChatAdminIDCard) != "" {
		sets = append(sets, "chat_admin_idcard = ?")
		args = append(args, values.ChatAdminIDCard)
	}
	if values.ChatApplyStatus >= 0 {
		sets = append(sets, "chat_apply_status = ?")
		args = append(args, values.ChatApplyStatus)
	}
	if values.ChatStatus >= 0 {
		sets = append(sets, "chat_status = ?")
		args = append(args, values.ChatStatus)
	}
	return sets, args
}

func workMessageConfigStepSets(values dashboard.WorkMessageConfigItem) ([]string, []any) {
	sets, args := workMessageConfigSets(values)
	if strings.TrimSpace(values.ServiceContactURL) != "" {
		sets = append(sets, "service_contact_url = ?")
		args = append(args, values.ServiceContactURL)
	}
	if strings.TrimSpace(values.ChatWhitelistIPRaw) != "" {
		sets = append(sets, "chat_whitelist_ip = ?")
		args = append(args, jsonOrArray(values.ChatWhitelistIPRaw))
	}
	if strings.TrimSpace(values.ChatRSAKeyRaw) != "" {
		sets = append(sets, "chat_rsa_key = ?")
		args = append(args, jsonOrObject(values.ChatRSAKeyRaw))
	}
	return sets, args
}

func jsonOrArray(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "[]"
	}
	if json.Valid([]byte(raw)) {
		return raw
	}
	return "[]"
}

func jsonOrObject(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}"
	}
	if json.Valid([]byte(raw)) {
		return raw
	}
	return "{}"
}

func strconvInt(value int) string {
	return fmt.Sprintf("%d", value)
}
