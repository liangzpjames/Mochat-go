package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) RoomCalendarPage(ctx context.Context, filter dashboard.RoomCalendarFilter) (dashboard.RoomCalendarPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := roomCalendarWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_room_calendar c "+where, args...).Scan(&total); err != nil {
		return dashboard.RoomCalendarPage{}, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, roomCalendarSelect()+where+`
		ORDER BY c.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomCalendarPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomCalendarItem, 0)
	for rows.Next() {
		item, err := scanRoomCalendarRow(rows)
		if err != nil {
			return dashboard.RoomCalendarPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomCalendarPage{}, err
	}
	return dashboard.RoomCalendarPage{Items: items, Total: total, TotalPage: pageCount(total, filter.PerPage), Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) RoomCalendarByID(ctx context.Context, corpID int, id int) (dashboard.RoomCalendarItem, bool, error) {
	item, err := scanRoomCalendarRow(s.db.QueryRowContext(ctx, roomCalendarSelect()+`
		WHERE c.corp_id = ? AND c.id = ? AND c.deleted_at IS NULL
		LIMIT 1
	`, corpID, id))
	if err == sql.ErrNoRows {
		return dashboard.RoomCalendarItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomCalendarItem{}, false, err
	}
	pushes, err := s.roomCalendarPushes(ctx, id)
	if err != nil {
		return dashboard.RoomCalendarItem{}, false, err
	}
	item.Pushes = pushes
	return item, true, nil
}

func (s *MySQLStore) CreateRoomCalendar(ctx context.Context, values dashboard.RoomCalendarWrite) (int, error) {
	tenantID, err := s.tenantIDByCorpID(ctx, values.CorpID)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_room_calendar
			(name, rooms, on_off, tenant_id, corp_id, create_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, values.Name, jsonOrArray(values.RoomsRaw), roomCalendarDefaultOnOff(values), tenantID, values.CorpID, values.CreateUserID, now, now)
	if err != nil {
		return 0, err
	}
	id64, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	id := int(id64)
	if values.HasPushes {
		if err := insertRoomCalendarPushes(ctx, tx, id, values.Pushes, now); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *MySQLStore) UpdateRoomCalendar(ctx context.Context, corpID int, id int, values dashboard.RoomCalendarWrite) (bool, error) {
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	sets := []string{"updated_at = ?"}
	args := []any{now}
	if values.HasName {
		sets = append(sets, "name = ?")
		args = append(args, values.Name)
	}
	if values.HasRooms {
		sets = append(sets, "rooms = ?")
		args = append(args, jsonOrArray(values.RoomsRaw))
	}
	if values.HasOnOff {
		sets = append(sets, "on_off = ?")
		args = append(args, values.OnOff)
	}
	args = append(args, corpID, id)
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_room_calendar
		SET `+strings.Join(sets, ", ")+`
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
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
	if values.HasPushes {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_room_calendar_push
			SET deleted_at = ?, updated_at = ?
			WHERE room_calendar_id = ? AND deleted_at IS NULL
		`, now, now, strconv.Itoa(id)); err != nil {
			return false, err
		}
		if err := insertRoomCalendarPushes(ctx, tx, id, values.Pushes, now); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) DeleteRoomCalendar(ctx context.Context, corpID int, id int) (bool, error) {
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_room_calendar
		SET deleted_at = ?, updated_at = ?
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
	`, now, now, corpID, id)
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
	if _, err := tx.ExecContext(ctx, `UPDATE mc_room_calendar_push SET deleted_at = ?, updated_at = ? WHERE room_calendar_id = ? AND deleted_at IS NULL`, now, now, strconv.Itoa(id)); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mc_room_calendar_record SET deleted_at = ?, updated_at = ? WHERE room_calendar_id = ? AND deleted_at IS NULL`, now, now, id); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) AddRoomCalendarRooms(ctx context.Context, corpID int, id int, roomsRaw string) (bool, error) {
	current, found, err := s.roomCalendarRoomsRaw(ctx, corpID, id)
	if err != nil || !found {
		return false, err
	}
	merged, err := mergeRoomCalendarRooms(current, roomsRaw)
	if err != nil {
		return false, err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_calendar
		SET rooms = ?, updated_at = ?
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
	`, merged, time.Now(), corpID, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) RemoveRoomCalendarRoom(ctx context.Context, corpID int, id int, roomKey string) (bool, error) {
	current, found, err := s.roomCalendarRoomsRaw(ctx, corpID, id)
	if err != nil || !found {
		return false, err
	}
	next, err := removeRoomCalendarRoom(current, roomKey)
	if err != nil {
		return false, err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_calendar
		SET rooms = ?, updated_at = ?
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
	`, next, time.Now(), corpID, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) roomCalendarRoomsRaw(ctx context.Context, corpID int, id int) (string, bool, error) {
	var raw []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(rooms, JSON_ARRAY())
		FROM mc_room_calendar
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
		LIMIT 1
	`, corpID, id).Scan(&raw)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(raw), true, nil
}

func (s *MySQLStore) roomCalendarPushes(ctx context.Context, calendarID int) ([]dashboard.RoomCalendarPushItem, error) {
	rows, err := s.db.QueryContext(ctx, roomCalendarPushSelect()+`
		WHERE p.room_calendar_id = ? AND p.deleted_at IS NULL
		ORDER BY p.day ASC, p.id ASC
	`, strconv.Itoa(calendarID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomCalendarPushItem, 0)
	for rows.Next() {
		item, err := scanRoomCalendarPushRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func insertRoomCalendarPushes(ctx context.Context, tx *sql.Tx, calendarID int, pushes []dashboard.RoomCalendarPushWrite, now time.Time) error {
	for _, push := range pushes {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_room_calendar_push
				(room_calendar_id, name, day, push_content, on_off, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, strconv.Itoa(calendarID), push.Name, push.Day, jsonOrArray(push.PushContentRaw), roomCalendarPushDefaultOnOff(push), roomCalendarPushDefaultStatus(push), now, now); err != nil {
			return err
		}
	}
	return nil
}

func roomCalendarSelect() string {
	return `
		SELECT
			c.id,
			COALESCE(c.name, ''),
			COALESCE(c.rooms, JSON_ARRAY()),
			COALESCE(c.on_off, 1),
			COALESCE(c.tenant_id, 0),
			COALESCE(c.corp_id, 0),
			COALESCE(c.create_user_id, 0),
			COALESCE(u.name, ''),
			COALESCE(JSON_LENGTH(c.rooms), 0),
			(SELECT COUNT(*) FROM mc_room_calendar_push p WHERE CAST(p.room_calendar_id AS UNSIGNED) = c.id AND p.deleted_at IS NULL),
			c.created_at,
			c.updated_at
		FROM mc_room_calendar c
		LEFT JOIN mc_user u ON u.id = c.create_user_id AND u.deleted_at IS NULL
	`
}

func roomCalendarWhere(filter dashboard.RoomCalendarFilter) (string, []any) {
	where := "WHERE c.corp_id = ? AND c.deleted_at IS NULL"
	args := []any{filter.CorpID}
	if strings.TrimSpace(filter.Name) != "" {
		where += " AND c.name LIKE ?"
		args = append(args, "%"+strings.TrimSpace(filter.Name)+"%")
	}
	if filter.OnOff >= 0 {
		where += " AND c.on_off = ?"
		args = append(args, filter.OnOff)
	}
	return where, args
}

func roomCalendarPushSelect() string {
	return `
		SELECT
			p.id,
			COALESCE(p.room_calendar_id, ''),
			COALESCE(p.name, ''),
			COALESCE(p.day, ''),
			COALESCE(p.push_content, JSON_ARRAY()),
			COALESCE(p.on_off, 1),
			COALESCE(p.status, 1),
			p.created_at,
			p.updated_at
		FROM mc_room_calendar_push p
	`
}

type roomCalendarScanner interface {
	Scan(dest ...any) error
}

func scanRoomCalendarRow(scanner roomCalendarScanner) (dashboard.RoomCalendarItem, error) {
	var item dashboard.RoomCalendarItem
	var roomsRaw []byte
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.Name, &roomsRaw, &item.OnOff, &item.TenantID, &item.CorpID, &item.CreateUserID, &item.CreateUserName, &item.RoomNum, &item.PushNum, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RoomCalendarItem{}, err
	}
	item.RoomsRaw = string(roomsRaw)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanRoomCalendarPushRow(scanner roomCalendarScanner) (dashboard.RoomCalendarPushItem, error) {
	var item dashboard.RoomCalendarPushItem
	var calendarIDText string
	var pushContent []byte
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &calendarIDText, &item.Name, &item.Day, &pushContent, &item.OnOff, &item.Status, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RoomCalendarPushItem{}, err
	}
	item.RoomCalendarID, _ = strconv.Atoi(calendarIDText)
	item.PushContentRaw = string(pushContent)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func roomCalendarDefaultOnOff(values dashboard.RoomCalendarWrite) int {
	if !values.HasOnOff || values.OnOff <= 0 {
		return 1
	}
	return values.OnOff
}

func roomCalendarPushDefaultOnOff(values dashboard.RoomCalendarPushWrite) int {
	if !values.HasOnOff || values.OnOff <= 0 {
		return 1
	}
	return values.OnOff
}

func roomCalendarPushDefaultStatus(values dashboard.RoomCalendarPushWrite) int {
	if !values.HasStatus || values.Status <= 0 {
		return 1
	}
	return values.Status
}

func mergeRoomCalendarRooms(existingRaw string, nextRaw string) (string, error) {
	existing, err := decodeRoomCalendarRooms(existingRaw)
	if err != nil {
		return "", err
	}
	next, err := decodeRoomCalendarRooms(nextRaw)
	if err != nil {
		return "", err
	}
	seen := map[string]bool{}
	merged := make([]any, 0, len(existing)+len(next))
	for _, item := range append(existing, next...) {
		key := roomCalendarRoomKey(item)
		if key == "" {
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, item)
	}
	raw, err := json.Marshal(merged)
	return string(raw), err
}

func removeRoomCalendarRoom(existingRaw string, roomKey string) (string, error) {
	existing, err := decodeRoomCalendarRooms(existingRaw)
	if err != nil {
		return "", err
	}
	roomKey = strings.TrimSpace(roomKey)
	next := make([]any, 0, len(existing))
	for _, item := range existing {
		if roomCalendarRoomKey(item) == roomKey {
			continue
		}
		next = append(next, item)
	}
	raw, err := json.Marshal(next)
	return string(raw), err
}

func decodeRoomCalendarRooms(raw string) ([]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []any{}, nil
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, err
	}
	switch typed := decoded.(type) {
	case []any:
		return typed, nil
	case nil:
		return []any{}, nil
	default:
		return []any{typed}, nil
	}
}

func roomCalendarRoomKey(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case map[string]any:
		for _, key := range []string{"wxChatId", "chatid", "roomId", "room_id", "id"} {
			if value, ok := typed[key]; ok {
				if text := roomCalendarRoomKey(value); text != "" {
					return text
				}
			}
		}
	}
	raw, _ := json.Marshal(value)
	return string(raw)
}
