package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) ActiveContactSOPLogSources(ctx context.Context) ([]dashboard.ContactSOPLogSource, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(corp_id, 0), COALESCE(setting, ''), COALESCE(employee_ids, ''), COALESCE(contact_ids, '')
		FROM mc_contact_sop
		WHERE COALESCE(state, 0) = 1
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := make([]dashboard.ContactSOPLogSource, 0)
	for rows.Next() {
		var source dashboard.ContactSOPLogSource
		if err := rows.Scan(&source.ID, &source.CorpID, &source.SettingRaw, &source.EmployeeIDsRaw, &source.ContactIDsRaw); err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (s *MySQLStore) ActiveRoomSOPLogSources(ctx context.Context) ([]dashboard.RoomSOPLogSource, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(corp_id, 0), COALESCE(setting, ''), COALESCE(room_ids, '')
		FROM mc_room_sop
		WHERE COALESCE(state, 0) = 1
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := make([]dashboard.RoomSOPLogSource, 0)
	for rows.Next() {
		var source dashboard.RoomSOPLogSource
		if err := rows.Scan(&source.ID, &source.CorpID, &source.SettingRaw, &source.RoomIDsRaw); err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (s *MySQLStore) ContactSOPLogTargets(ctx context.Context, corpID int, employeeIDs []int, contactIDs []int) ([]dashboard.ContactSOPLogTarget, error) {
	employeeIDs = positiveIntList(employeeIDs)
	contactIDs = positiveIntList(contactIDs)
	if corpID <= 0 || len(employeeIDs) == 0 || len(contactIDs) == 0 {
		return []dashboard.ContactSOPLogTarget{}, nil
	}
	args := []any{corpID}
	args = append(args, intsAsAny(employeeIDs)...)
	args = append(args, intsAsAny(contactIDs)...)
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT
			COALESCE(e.wx_user_id, ''),
			COALESCE(c.wx_external_userid, ''),
			COALESCE(ce.create_time, ce.created_at)
		FROM mc_work_contact_employee ce
		INNER JOIN mc_work_employee e
			ON e.id = ce.employee_id
			AND e.corp_id = ce.corp_id
			AND e.deleted_at IS NULL
		INNER JOIN mc_work_contact c
			ON c.id = ce.contact_id
			AND c.corp_id = ce.corp_id
			AND c.deleted_at IS NULL
		WHERE ce.corp_id = ?
		  AND ce.employee_id IN (`+placeholders(len(employeeIDs))+`)
		  AND ce.contact_id IN (`+placeholders(len(contactIDs))+`)
		  AND ce.status = 1
		  AND ce.deleted_at IS NULL
		ORDER BY ce.employee_id ASC, ce.contact_id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targets := make([]dashboard.ContactSOPLogTarget, 0)
	for rows.Next() {
		var target dashboard.ContactSOPLogTarget
		var anchor sql.NullTime
		if err := rows.Scan(&target.EmployeeWXUserID, &target.ContactWXExternalUser, &anchor); err != nil {
			return nil, err
		}
		if strings.TrimSpace(target.EmployeeWXUserID) == "" || strings.TrimSpace(target.ContactWXExternalUser) == "" {
			continue
		}
		if anchor.Valid {
			target.AnchorTime = anchor.Time
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (s *MySQLStore) RoomSOPLogTargets(ctx context.Context, corpID int, roomIDs []int, targetAnchor string) ([]dashboard.RoomSOPLogTarget, error) {
	roomIDs = positiveIntList(roomIDs)
	if corpID <= 0 || len(roomIDs) == 0 {
		return []dashboard.RoomSOPLogTarget{}, nil
	}
	if strings.TrimSpace(targetAnchor) == dashboard.SOPLogTargetAnchorRoomJoin {
		return s.roomSOPLogJoinTargets(ctx, corpID, roomIDs)
	}
	args := []any{corpID}
	args = append(args, intsAsAny(roomIDs)...)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			r.id,
			COALESCE(e.wx_user_id, ''),
			COALESCE(r.create_time, r.created_at)
		FROM mc_work_room r
		INNER JOIN mc_work_employee e
			ON e.id = r.owner_id
			AND e.corp_id = r.corp_id
			AND e.deleted_at IS NULL
		WHERE r.corp_id = ?
		  AND r.id IN (`+placeholders(len(roomIDs))+`)
		  AND r.deleted_at IS NULL
		ORDER BY r.id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targets := make([]dashboard.RoomSOPLogTarget, 0)
	for rows.Next() {
		var target dashboard.RoomSOPLogTarget
		var anchor sql.NullTime
		if err := rows.Scan(&target.RoomID, &target.EmployeeWXUserID, &anchor); err != nil {
			return nil, err
		}
		if target.RoomID <= 0 || strings.TrimSpace(target.EmployeeWXUserID) == "" {
			continue
		}
		if anchor.Valid {
			target.AnchorTime = anchor.Time
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (s *MySQLStore) roomSOPLogJoinTargets(ctx context.Context, corpID int, roomIDs []int) ([]dashboard.RoomSOPLogTarget, error) {
	args := []any{corpID}
	args = append(args, intsAsAny(roomIDs)...)
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT
			r.id,
			COALESCE(e.wx_user_id, ''),
			COALESCE(NULLIF(c.wx_external_userid, ''), NULLIF(cr.wx_user_id, ''), ''),
			COALESCE(cr.join_time, cr.created_at, r.create_time, r.created_at)
		FROM mc_work_room r
		INNER JOIN mc_work_employee e
			ON e.id = r.owner_id
			AND e.corp_id = r.corp_id
			AND e.deleted_at IS NULL
		INNER JOIN mc_work_contact_room cr
			ON cr.room_id = r.id
			AND cr.deleted_at IS NULL
			AND cr.status = 1
			AND cr.type = 2
		LEFT JOIN mc_work_contact c
			ON c.id = cr.contact_id
			AND c.corp_id = r.corp_id
			AND c.deleted_at IS NULL
		WHERE r.corp_id = ?
		  AND r.id IN (`+placeholders(len(roomIDs))+`)
		  AND r.deleted_at IS NULL
		ORDER BY r.id ASC, cr.id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targets := make([]dashboard.RoomSOPLogTarget, 0)
	for rows.Next() {
		var target dashboard.RoomSOPLogTarget
		var anchor sql.NullTime
		if err := rows.Scan(&target.RoomID, &target.EmployeeWXUserID, &target.ContactWXExternalUser, &anchor); err != nil {
			return nil, err
		}
		if target.RoomID <= 0 || strings.TrimSpace(target.EmployeeWXUserID) == "" || strings.TrimSpace(target.ContactWXExternalUser) == "" {
			continue
		}
		if anchor.Valid {
			target.AnchorTime = anchor.Time
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (s *MySQLStore) RoomIDByWXChatID(ctx context.Context, corpID int, wxChatID string) (int, bool, error) {
	wxChatID = strings.TrimSpace(wxChatID)
	if corpID <= 0 || wxChatID == "" {
		return 0, false, nil
	}
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

func (s *MySQLStore) InsertContactSOPLog(ctx context.Context, item dashboard.ContactSOPLogCreate) (bool, error) {
	if item.CorpID <= 0 || item.ContactSOPID <= 0 || strings.TrimSpace(item.EmployeeWXUserID) == "" || strings.TrimSpace(item.ContactWXExternalUser) == "" || strings.TrimSpace(item.TaskRaw) == "" {
		return false, nil
	}
	createdAt := nullableSOPLogTime(item.CreatedAt)
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_contact_sop_log (corp_id, contact_sop_id, employee, contact, task, created_at)
		SELECT ?, ?, ?, ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1
			FROM mc_contact_sop_log
			WHERE corp_id = ?
			  AND contact_sop_id = ?
			  AND employee = ?
			  AND contact = ?
			  AND task = ?
			LIMIT 1
		)
	`, item.CorpID, item.ContactSOPID, strings.TrimSpace(item.EmployeeWXUserID), strings.TrimSpace(item.ContactWXExternalUser), item.TaskRaw, createdAt,
		item.CorpID, item.ContactSOPID, strings.TrimSpace(item.EmployeeWXUserID), strings.TrimSpace(item.ContactWXExternalUser), item.TaskRaw)
	return rowsAffectedBool(result, err)
}

func (s *MySQLStore) InsertRoomSOPLog(ctx context.Context, item dashboard.RoomSOPLogCreate) (bool, error) {
	if item.CorpID <= 0 || item.RoomSOPID <= 0 || item.RoomID <= 0 || strings.TrimSpace(item.EmployeeWXUserID) == "" || strings.TrimSpace(item.TaskRaw) == "" {
		return false, nil
	}
	createdAt := nullableSOPLogTime(item.CreatedAt)
	contact := strings.TrimSpace(item.ContactWXExternalUser)
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_room_sop_log (corp_id, room_sop_id, room_id, state, employee, contact, task, created_at, updated_at)
		SELECT ?, ?, ?, 0, ?, ?, ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1
			FROM mc_room_sop_log
			WHERE corp_id = ?
			  AND room_sop_id = ?
			  AND room_id = ?
			  AND employee = ?
			  AND COALESCE(contact, '') = ?
			  AND task = ?
			LIMIT 1
		)
	`, item.CorpID, item.RoomSOPID, item.RoomID, strings.TrimSpace(item.EmployeeWXUserID), contact, item.TaskRaw, createdAt, createdAt,
		item.CorpID, item.RoomSOPID, item.RoomID, strings.TrimSpace(item.EmployeeWXUserID), contact, item.TaskRaw)
	return rowsAffectedBool(result, err)
}

func nullableSOPLogTime(value time.Time) sql.NullTime {
	if value.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value, Valid: true}
}

func positiveIntList(values []int) []int {
	seen := map[int]struct{}{}
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func intsAsAny(values []int) []any {
	args := make([]any, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	return args
}
