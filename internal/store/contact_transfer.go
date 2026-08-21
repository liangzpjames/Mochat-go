package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) ContactTransferAssignedContacts(ctx context.Context, filter dashboard.ContactTransferContactFilter) ([]dashboard.ContactTransferContactItem, error) {
	where := []string{"ce.corp_id = ?", "ce.deleted_at IS NULL", "c.deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if filter.ContactName != "" {
		where = append(where, "(ce.remark LIKE ? OR c.name LIKE ? OR c.wx_external_userid LIKE ?)")
		needle := "%" + filter.ContactName + "%"
		args = append(args, needle, needle, needle)
	}
	if filter.AddTimeStart != "" {
		where = append(where, "ce.create_time >= ?")
		args = append(args, filter.AddTimeStart)
	}
	if filter.AddTimeEnd != "" {
		where = append(where, "ce.create_time < DATE_ADD(?, INTERVAL 1 DAY)")
		args = append(args, filter.AddTimeEnd)
	}
	if len(filter.EmployeeIDs) > 0 {
		where = append(where, "ce.employee_id IN ("+placeholders(len(filter.EmployeeIDs))+")")
		for _, id := range filter.EmployeeIDs {
			args = append(args, id)
		}
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			ce.contact_id,
			ce.employee_id,
			COALESCE(c.wx_external_userid, ''),
			COALESCE(e.wx_user_id, ''),
			ce.remark,
			COALESCE(c.name, ''),
			COALESCE(c.corp_name, ''),
			COALESCE(e.name, ''),
			ce.create_time,
			ce.add_way,
			COALESCE((
				SELECT state
				FROM mc_work_transfer_log
				WHERE corp_id = ce.corp_id
				  AND contact_id = c.wx_external_userid
				  AND handover_employee_id = e.wx_user_id
				  AND state IS NOT NULL
				ORDER BY id DESC
				LIMIT 1
			), 0)
		FROM mc_work_contact_employee ce
		JOIN mc_work_contact c ON c.id = ce.contact_id
		LEFT JOIN mc_work_employee e ON e.id = ce.employee_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY ce.id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]dashboard.ContactTransferContactItem, 0)
	for rows.Next() {
		var item dashboard.ContactTransferContactItem
		var createTime sql.NullTime
		var state int
		if err := rows.Scan(&item.ContactID, &item.EmployeeID, &item.ContactWXID, &item.EmployeeWXID, &item.ContactName, &item.NickName, &item.CorpName, &item.EmployeeName, &createTime, &item.AddWay, &state); err != nil {
			return nil, err
		}
		item.AddTime = formatMinuteTime(createTime)
		item.TransferState = dashboardContactTransferStateText(state)
		tags, err := s.contactTransferTagNames(ctx, item.ContactID, item.EmployeeID)
		if err != nil {
			return nil, err
		}
		item.Tags = tags
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) ContactTransferUnassignedContacts(ctx context.Context, filter dashboard.ContactTransferUnassignedFilter) (dashboard.ContactTransferUnassignedPage, error) {
	page := dashboard.ContactTransferUnassignedPage{Items: []dashboard.ContactTransferContactItem{}, LastTime: "无数据"}
	var last sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT created_at
		FROM mc_work_unassigned
		WHERE corp_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, filter.CorpID).Scan(&last)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return page, err
	}
	if last.Valid {
		page.LastTime = formatTime(last)
	}

	where := []string{"wu.corp_id = ?", "wu.deleted_at IS NULL", "c.deleted_at IS NULL", "ce.deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if filter.ContactName != "" {
		where = append(where, "(ce.remark LIKE ? OR c.name LIKE ? OR c.wx_external_userid LIKE ?)")
		needle := "%" + filter.ContactName + "%"
		args = append(args, needle, needle, needle)
	}
	if len(filter.EmployeeIDs) > 0 {
		where = append(where, "e.id IN ("+placeholders(len(filter.EmployeeIDs))+")")
		for _, id := range filter.EmployeeIDs {
			args = append(args, id)
		}
	}
	if filter.AddTimeStart != "" {
		where = append(where, "ce.create_time >= ?")
		args = append(args, filter.AddTimeStart)
	}
	if filter.AddTimeEnd != "" {
		where = append(where, "ce.create_time < DATE_ADD(?, INTERVAL 1 DAY)")
		args = append(args, filter.AddTimeEnd)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			ce.contact_id,
			e.id,
			c.wx_external_userid,
			e.wx_user_id,
			ce.remark,
			c.name,
			c.corp_name,
			e.name,
			ce.create_time,
			ce.add_way
		FROM mc_work_unassigned wu
		JOIN mc_work_employee e ON e.corp_id = wu.corp_id AND e.wx_user_id = wu.handover_userid
		JOIN mc_work_contact c ON c.corp_id = wu.corp_id AND c.wx_external_userid = wu.external_userid
		JOIN mc_work_contact_employee ce ON ce.employee_id = e.id AND ce.contact_id = c.id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY wu.id ASC
	`, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()

	for rows.Next() {
		var item dashboard.ContactTransferContactItem
		var createTime sql.NullTime
		if err := rows.Scan(&item.ContactID, &item.EmployeeID, &item.ContactWXID, &item.EmployeeWXID, &item.ContactName, &item.NickName, &item.CorpName, &item.EmployeeName, &createTime, &item.AddWay); err != nil {
			return dashboard.ContactTransferUnassignedPage{}, err
		}
		item.AddTime = formatMinuteTime(createTime)
		tags, err := s.contactTransferTagNames(ctx, item.ContactID, item.EmployeeID)
		if err != nil {
			return dashboard.ContactTransferUnassignedPage{}, err
		}
		item.Tags = tags
		page.Items = append(page.Items, item)
	}
	return page, rows.Err()
}

func (s *MySQLStore) ContactTransferRooms(ctx context.Context, corpID int, roomName string) ([]dashboard.ContactTransferRoomItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			r.id,
			r.wx_chat_id,
			r.name,
			e.name,
			(
				SELECT COUNT(*)
				FROM mc_work_contact_room cr
				WHERE cr.room_id = r.id AND cr.contact_id > 0 AND cr.status = 1 AND cr.deleted_at IS NULL
			),
			(
				SELECT COUNT(*)
				FROM mc_work_contact_room cr
				WHERE cr.room_id = r.id AND cr.join_time > CURDATE() AND cr.join_time < DATE_ADD(CURDATE(), INTERVAL 1 DAY) AND cr.status = 1 AND cr.deleted_at IS NULL
			),
			(
				SELECT COUNT(*)
				FROM mc_work_contact_room cr
				WHERE cr.room_id = r.id AND cr.out_time > CURDATE() AND cr.out_time < DATE_ADD(CURDATE(), INTERVAL 1 DAY) AND cr.status = 2 AND cr.deleted_at IS NULL
			),
			r.created_at
		FROM (
			SELECT DISTINCT handover_userid
			FROM mc_work_unassigned
			WHERE corp_id = ? AND deleted_at IS NULL
		) wu
		JOIN mc_work_employee e ON e.corp_id = ? AND e.wx_user_id = wu.handover_userid
		JOIN mc_work_room r ON r.corp_id = ? AND r.owner_id = e.id AND r.deleted_at IS NULL
		WHERE r.name LIKE ?
		ORDER BY r.id ASC
	`, corpID, corpID, corpID, "%"+roomName+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rooms := make([]dashboard.ContactTransferRoomItem, 0)
	for rows.Next() {
		var room dashboard.ContactTransferRoomItem
		var createTime sql.NullTime
		if err := rows.Scan(&room.RoomID, &room.ChatID, &room.RoomName, &room.Owner, &room.UserNum, &room.AddNum, &room.QuitNum, &createTime); err != nil {
			return nil, err
		}
		room.CreateTime = formatTime(createTime)
		rooms = append(rooms, room)
	}
	return rooms, rows.Err()
}

func (s *MySQLStore) ContactTransferLogs(ctx context.Context, filter dashboard.ContactTransferLogFilter) ([]dashboard.ContactTransferLogItem, error) {
	status, transferType := contactTransferLogMode(filter.Mode)
	if status == 0 || transferType == 0 {
		return []dashboard.ContactTransferLogItem{}, nil
	}
	where := []string{"log.corp_id = ?", "log.status = ?", "log.type = ?"}
	args := []any{filter.CorpID, status, transferType}
	if filter.Name != "" {
		where = append(where, "log.name LIKE ?")
		args = append(args, "%"+filter.Name+"%")
	}
	if filter.EmployeeWXID != "" {
		where = append(where, "log.takeover_employee_id = ?")
		args = append(args, filter.EmployeeWXID)
	}
	if filter.CreateTimeStart != "" {
		where = append(where, "log.created_at >= ?")
		args = append(args, filter.CreateTimeStart)
	}
	if filter.CreateTimeEnd != "" {
		where = append(where, "log.created_at < DATE_ADD(?, INTERVAL 1 DAY)")
		args = append(args, filter.CreateTimeEnd)
	}
	if filter.Mode == 2 {
		rows, err := s.db.QueryContext(ctx, `
			SELECT
				COALESCE(room.id, 0),
				COALESCE(log.name, ''),
				COALESCE(employee.name, ''),
				(
					SELECT COUNT(*)
					FROM mc_work_contact_room cr
					WHERE cr.room_id = room.id AND cr.contact_id > 0 AND cr.status = 1 AND cr.deleted_at IS NULL
				),
				log.created_at
			FROM mc_work_transfer_log log
			LEFT JOIN mc_work_employee employee ON employee.corp_id = log.corp_id AND employee.wx_user_id = log.takeover_employee_id
			LEFT JOIN mc_work_room room ON room.corp_id = log.corp_id AND room.wx_chat_id = log.contact_id AND room.deleted_at IS NULL
			WHERE `+strings.Join(where, " AND ")+`
			ORDER BY log.id ASC
		`, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := make([]dashboard.ContactTransferLogItem, 0)
		for rows.Next() {
			item := dashboard.ContactTransferLogItem{Mode: filter.Mode}
			var createTime sql.NullTime
			if err := rows.Scan(&item.RoomID, &item.Name, &item.Employee, &item.RoomNum, &createTime); err != nil {
				return nil, err
			}
			item.CreateTime = formatTime(createTime)
			items = append(items, item)
		}
		return items, rows.Err()
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			COALESCE(contact.id, 0),
			COALESCE(contact.name, ''),
			COALESCE(contact.corp_name, ''),
			COALESCE(employee.name, ''),
			COALESCE(log.state, 0),
			log.created_at
		FROM mc_work_transfer_log log
		LEFT JOIN mc_work_employee employee ON employee.corp_id = log.corp_id AND employee.wx_user_id = log.takeover_employee_id
		LEFT JOIN mc_work_contact contact ON contact.corp_id = log.corp_id AND contact.wx_external_userid = log.contact_id AND contact.deleted_at IS NULL
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY log.id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.ContactTransferLogItem, 0)
	for rows.Next() {
		item := dashboard.ContactTransferLogItem{Mode: filter.Mode}
		var state int
		var createTime sql.NullTime
		if err := rows.Scan(&item.ContactID, &item.Name, &item.CorpName, &item.Employee, &state, &createTime); err != nil {
			return nil, err
		}
		item.State = dashboardContactTransferStateText(state)
		item.CreateTime = formatTime(createTime)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) ReplaceContactTransferUnassigned(ctx context.Context, corpID int, items []dashboard.ContactTransferUnassignedSeed) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, `UPDATE mc_work_unassigned SET deleted_at = NOW(), updated_at = NOW() WHERE corp_id = ? AND deleted_at IS NULL`, corpID); err != nil {
		return err
	}
	for _, item := range items {
		if strings.TrimSpace(item.HandoverUserID) == "" || strings.TrimSpace(item.ExternalUserID) == "" {
			continue
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO mc_work_unassigned (corp_id, handover_userid, external_userid, dimission_time, created_at, updated_at)
			VALUES (?, ?, ?, ?, NOW(), NOW())
		`, corpID, item.HandoverUserID, item.ExternalUserID, item.DimissionTime); err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}

func (s *MySQLStore) CreateContactTransferLog(ctx context.Context, values dashboard.ContactTransferLogWrite) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_work_transfer_log (corp_id, status, type, name, contact_id, handover_employee_id, takeover_employee_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, values.CorpID, values.Status, values.Type, values.Name, values.ContactID, values.HandoverEmployeeID, values.TakeoverEmployeeID)
	return err
}

func (s *MySQLStore) NextContactTransferStateLog(ctx context.Context, afterID int) (dashboard.ContactTransferStateLog, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, COALESCE(contact_id, ''), COALESCE(handover_employee_id, ''), COALESCE(takeover_employee_id, ''), COALESCE(state, 0)
		FROM mc_work_transfer_log
		WHERE id > ? AND type = 1
		ORDER BY id ASC
		LIMIT 1
	`, afterID)
	var item dashboard.ContactTransferStateLog
	if err := row.Scan(&item.ID, &item.CorpID, &item.ContactID, &item.HandoverEmployeeID, &item.TakeoverEmployeeID, &item.State); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dashboard.ContactTransferStateLog{}, false, nil
		}
		return dashboard.ContactTransferStateLog{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) UpdateContactTransferLogState(ctx context.Context, logID int, state int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_work_transfer_log
		SET state = ?, updated_at = NOW()
		WHERE id = ?
	`, state, logID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) WorkContactNameByExternalUserID(ctx context.Context, corpID int, externalUserID string) (string, bool, error) {
	var name sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT name
		FROM mc_work_contact
		WHERE corp_id = ? AND wx_external_userid = ? AND deleted_at IS NULL
		LIMIT 1
	`, corpID, externalUserID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return nullString(name), true, nil
}

func (s *MySQLStore) WorkRoomNameByWXChatID(ctx context.Context, corpID int, wxChatID string) (string, bool, error) {
	var name sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT name
		FROM mc_work_room
		WHERE corp_id = ? AND wx_chat_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, corpID, wxChatID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return nullString(name), true, nil
}

func (s *MySQLStore) contactTransferTagNames(ctx context.Context, contactID int, employeeID int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tag.name
		FROM mc_work_contact_tag_pivot pivot
		JOIN mc_work_contact_tag tag ON tag.id = pivot.contact_tag_id AND tag.deleted_at IS NULL
		WHERE pivot.contact_id = ? AND pivot.employee_id = ? AND pivot.deleted_at IS NULL
		ORDER BY pivot.id ASC
	`, contactID, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := make([]string, 0)
	for rows.Next() {
		var name sql.NullString
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tags = append(tags, nullString(name))
	}
	return tags, rows.Err()
}

func contactTransferLogMode(mode int) (status int, transferType int) {
	switch mode {
	case 1:
		return 1, 1
	case 2:
		return 1, 2
	case 3:
		return 2, 1
	default:
		return 0, 0
	}
}

func dashboardContactTransferStateText(state int) string {
	switch state {
	case 1:
		return "接替完毕"
	case 2:
		return "等待接替"
	case 3:
		return "客户拒绝"
	case 4:
		return "接替成员客户达到上限"
	case 5:
		return "无接替记录"
	default:
		return ""
	}
}

func formatMinuteTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02 15:04")
}

func contactTransferDayBounds(now time.Time) (string, string) {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.Add(24 * time.Hour)
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}
