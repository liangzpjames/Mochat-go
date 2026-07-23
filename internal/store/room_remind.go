package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) RoomRemindPage(ctx context.Context, filter dashboard.RoomRemindFilter) (dashboard.RoomRemindPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := roomRemindWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_room_remind r "+where, args...).Scan(&total); err != nil {
		return dashboard.RoomRemindPage{}, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, roomRemindSelect()+where+`
		ORDER BY r.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomRemindPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomRemindItem, 0)
	for rows.Next() {
		item, err := scanRoomRemindRow(rows)
		if err != nil {
			return dashboard.RoomRemindPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomRemindPage{}, err
	}
	return dashboard.RoomRemindPage{Items: items, Total: total, TotalPage: pageCount(total, filter.PerPage), Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) RoomRemindByID(ctx context.Context, corpID int, id int) (dashboard.RoomRemindItem, bool, error) {
	item, err := scanRoomRemindRow(s.db.QueryRowContext(ctx, roomRemindSelect()+`
		WHERE r.corp_id = ? AND r.id = ? AND r.deleted_at IS NULL
		LIMIT 1
	`, corpID, id))
	if err == sql.ErrNoRows {
		return dashboard.RoomRemindItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomRemindItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) CreateRoomRemind(ctx context.Context, values dashboard.RoomRemindWrite) (int, error) {
	tenantID, err := s.tenantIDByCorpID(ctx, values.CorpID)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_room_remind
			(name, rooms, is_qrcode, is_link, is_miniprogram, is_card, is_keyword, keyword, status, tenant_id, corp_id, create_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, values.Name, jsonOrArray(values.RoomsRaw), roomRemindFlag(values.IsQrcode), roomRemindFlag(values.IsLink), roomRemindFlag(values.IsMiniprogram), roomRemindFlag(values.IsCard), roomRemindKeywordFlag(values), values.Keyword, roomRemindDefaultStatus(values), tenantID, values.CorpID, values.CreateUserID, now, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *MySQLStore) UpdateRoomRemind(ctx context.Context, corpID int, id int, values dashboard.RoomRemindWrite) (bool, error) {
	sets := []string{"updated_at = ?"}
	args := []any{time.Now()}
	if values.HasName {
		sets = append(sets, "name = ?")
		args = append(args, values.Name)
	}
	if values.HasRooms {
		sets = append(sets, "rooms = ?")
		args = append(args, jsonOrArray(values.RoomsRaw))
	}
	if values.HasIsQrcode {
		sets = append(sets, "is_qrcode = ?")
		args = append(args, roomRemindFlag(values.IsQrcode))
	}
	if values.HasIsLink {
		sets = append(sets, "is_link = ?")
		args = append(args, roomRemindFlag(values.IsLink))
	}
	if values.HasIsMiniprogram {
		sets = append(sets, "is_miniprogram = ?")
		args = append(args, roomRemindFlag(values.IsMiniprogram))
	}
	if values.HasIsCard {
		sets = append(sets, "is_card = ?")
		args = append(args, roomRemindFlag(values.IsCard))
	}
	if values.HasIsKeyword {
		sets = append(sets, "is_keyword = ?")
		args = append(args, roomRemindFlag(values.IsKeyword))
	}
	if values.HasKeyword {
		sets = append(sets, "keyword = ?")
		args = append(args, values.Keyword)
	}
	if values.HasStatus {
		sets = append(sets, "status = ?")
		args = append(args, values.Status)
	}
	args = append(args, corpID, id)
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_remind
		SET `+strings.Join(sets, ", ")+`
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) UpdateRoomRemindStatus(ctx context.Context, corpID int, id int, status int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_remind
		SET status = ?, updated_at = ?
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
	`, status, time.Now(), corpID, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) DeleteRoomRemind(ctx context.Context, corpID int, id int) (bool, error) {
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_room_remind
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
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_room_remind_record
		SET deleted_at = ?, updated_at = ?
		WHERE remind_id = ? AND deleted_at IS NULL
	`, now, now, id); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func roomRemindSelect() string {
	return `
		SELECT
			r.id,
			COALESCE(r.name, ''),
			COALESCE(r.rooms, JSON_ARRAY()),
			COALESCE(r.is_qrcode, 0),
			COALESCE(r.is_link, 0),
			COALESCE(r.is_miniprogram, 0),
			COALESCE(r.is_card, 0),
			COALESCE(r.is_keyword, 0),
			COALESCE(r.keyword, ''),
			COALESCE(r.status, 1),
			COALESCE(r.tenant_id, 0),
			COALESCE(r.corp_id, 0),
			COALESCE(r.create_user_id, 0),
			COALESCE(u.name, ''),
			COALESCE(JSON_LENGTH(r.rooms), 0),
			(SELECT COUNT(*) FROM mc_room_remind_record rr WHERE rr.remind_id = r.id AND rr.deleted_at IS NULL),
			r.created_at,
			r.updated_at
		FROM mc_room_remind r
		LEFT JOIN mc_user u ON u.id = r.create_user_id AND u.deleted_at IS NULL
	`
}

func roomRemindWhere(filter dashboard.RoomRemindFilter) (string, []any) {
	where := "WHERE r.corp_id = ? AND r.deleted_at IS NULL"
	args := []any{filter.CorpID}
	if strings.TrimSpace(filter.Name) != "" {
		where += " AND r.name LIKE ?"
		args = append(args, "%"+strings.TrimSpace(filter.Name)+"%")
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		where += " AND r.keyword LIKE ?"
		args = append(args, "%"+strings.TrimSpace(filter.Keyword)+"%")
	}
	if filter.Status >= 0 {
		where += " AND r.status = ?"
		args = append(args, filter.Status)
	}
	return where, args
}

type roomRemindScanner interface {
	Scan(dest ...any) error
}

func scanRoomRemindRow(scanner roomRemindScanner) (dashboard.RoomRemindItem, error) {
	var item dashboard.RoomRemindItem
	var roomsRaw []byte
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.Name, &roomsRaw, &item.IsQrcode, &item.IsLink, &item.IsMiniprogram, &item.IsCard, &item.IsKeyword, &item.Keyword, &item.Status, &item.TenantID, &item.CorpID, &item.CreateUserID, &item.CreateUserName, &item.RoomNum, &item.RecordNum, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RoomRemindItem{}, err
	}
	item.RoomsRaw = string(roomsRaw)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func roomRemindFlag(value int) int {
	if value == 1 {
		return 1
	}
	return 0
}

func roomRemindKeywordFlag(values dashboard.RoomRemindWrite) int {
	if values.HasIsKeyword {
		return roomRemindFlag(values.IsKeyword)
	}
	if strings.TrimSpace(values.Keyword) != "" {
		return 1
	}
	return 0
}

func roomRemindDefaultStatus(values dashboard.RoomRemindWrite) int {
	if !values.HasStatus || values.Status < 0 {
		return 1
	}
	return values.Status
}
