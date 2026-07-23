package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) RoomQualityPage(ctx context.Context, filter dashboard.RoomQualityFilter) (dashboard.RoomQualityPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := roomQualityWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_room_quality q "+where, args...).Scan(&total); err != nil {
		return dashboard.RoomQualityPage{}, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, roomQualitySelect()+where+`
		ORDER BY q.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomQualityPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomQualityItem, 0)
	for rows.Next() {
		item, err := scanRoomQualityRow(rows)
		if err != nil {
			return dashboard.RoomQualityPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomQualityPage{}, err
	}
	return dashboard.RoomQualityPage{Items: items, Total: total, TotalPage: pageCount(total, filter.PerPage), Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) RoomQualityByID(ctx context.Context, corpID int, id int) (dashboard.RoomQualityItem, bool, error) {
	item, err := scanRoomQualityRow(s.db.QueryRowContext(ctx, roomQualitySelect()+`
		WHERE q.corp_id = ? AND q.id = ? AND q.deleted_at IS NULL
		LIMIT 1
	`, corpID, id))
	if err == sql.ErrNoRows {
		return dashboard.RoomQualityItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomQualityItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) CreateRoomQuality(ctx context.Context, values dashboard.RoomQualityWrite) (int, error) {
	tenantID, err := s.tenantIDByCorpID(ctx, values.CorpID)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_room_quality
			(name, description, `+"`rule`"+`, rooms, status, tenant_id, corp_id, create_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, values.Name, values.Description, jsonOrArray(values.RuleRaw), jsonOrArray(values.RoomsRaw), roomQualityDefaultStatus(values), tenantID, values.CorpID, values.CreateUserID, now, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *MySQLStore) UpdateRoomQuality(ctx context.Context, corpID int, id int, values dashboard.RoomQualityWrite) (bool, error) {
	sets := []string{"updated_at = ?"}
	args := []any{time.Now()}
	if values.HasName {
		sets = append(sets, "name = ?")
		args = append(args, values.Name)
	}
	if values.HasDescription {
		sets = append(sets, "description = ?")
		args = append(args, values.Description)
	}
	if values.HasRule {
		sets = append(sets, "`rule` = ?")
		args = append(args, jsonOrArray(values.RuleRaw))
	}
	if values.HasRooms {
		sets = append(sets, "rooms = ?")
		args = append(args, jsonOrArray(values.RoomsRaw))
	}
	if values.HasStatus {
		sets = append(sets, "status = ?")
		args = append(args, values.Status)
	}
	args = append(args, corpID, id)
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_quality
		SET `+strings.Join(sets, ", ")+`
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) UpdateRoomQualityStatus(ctx context.Context, corpID int, id int, status int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_quality
		SET status = ?, updated_at = ?
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
	`, status, time.Now(), corpID, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) DeleteRoomQuality(ctx context.Context, corpID int, id int) (bool, error) {
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_room_quality
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
		UPDATE mc_room_quality_contact
		SET deleted_at = ?, updated_at = ?
		WHERE quality_id = ? AND deleted_at IS NULL
	`, now, now, id); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) RoomQualityContactPage(ctx context.Context, filter dashboard.RoomQualityContactFilter) (dashboard.RoomQualityContactPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := roomQualityContactWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_room_quality_contact c JOIN mc_room_quality q ON q.id = c.quality_id `+where, args...).Scan(&total); err != nil {
		return dashboard.RoomQualityContactPage{}, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, roomQualityContactSelect()+where+`
		ORDER BY c.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomQualityContactPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomQualityContactItem, 0)
	for rows.Next() {
		item, err := scanRoomQualityContactRow(rows)
		if err != nil {
			return dashboard.RoomQualityContactPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomQualityContactPage{}, err
	}
	return dashboard.RoomQualityContactPage{Items: items, Total: total, TotalPage: pageCount(total, filter.PerPage), Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) RoomQualityContactByID(ctx context.Context, corpID int, id int) (dashboard.RoomQualityContactItem, bool, error) {
	item, err := scanRoomQualityContactRow(s.db.QueryRowContext(ctx, roomQualityContactSelect()+`
		WHERE q.corp_id = ? AND c.id = ? AND c.deleted_at IS NULL AND q.deleted_at IS NULL
		LIMIT 1
	`, corpID, id))
	if err == sql.ErrNoRows {
		return dashboard.RoomQualityContactItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomQualityContactItem{}, false, err
	}
	return item, true, nil
}

func roomQualitySelect() string {
	return `
		SELECT
			q.id,
			COALESCE(q.name, ''),
			COALESCE(q.description, ''),
			COALESCE(q.` + "`rule`" + `, JSON_ARRAY()),
			COALESCE(q.rooms, JSON_ARRAY()),
			COALESCE(q.status, 1),
			COALESCE(q.tenant_id, 0),
			COALESCE(q.corp_id, 0),
			COALESCE(q.create_user_id, 0),
			COALESCE(u.name, ''),
			COALESCE(JSON_LENGTH(q.rooms), 0),
			(SELECT COUNT(*) FROM mc_room_quality_contact cc WHERE cc.quality_id = q.id AND cc.deleted_at IS NULL),
			q.created_at,
			q.updated_at
		FROM mc_room_quality q
		LEFT JOIN mc_user u ON u.id = q.create_user_id AND u.deleted_at IS NULL
	`
}

func roomQualityWhere(filter dashboard.RoomQualityFilter) (string, []any) {
	where := "WHERE q.corp_id = ? AND q.deleted_at IS NULL"
	args := []any{filter.CorpID}
	if strings.TrimSpace(filter.Name) != "" {
		where += " AND q.name LIKE ?"
		args = append(args, "%"+strings.TrimSpace(filter.Name)+"%")
	}
	if filter.Status >= 0 {
		where += " AND q.status = ?"
		args = append(args, filter.Status)
	}
	return where, args
}

func roomQualityContactSelect() string {
	return `
		SELECT
			c.id,
			c.quality_id,
			COALESCE(c.room_id, 0),
			COALESCE(c.room_name, ''),
			COALESCE(c.contact_id, 0),
			COALESCE(c.external_user_id, ''),
			COALESCE(c.nickname, ''),
			COALESCE(c.avatar, ''),
			COALESCE(c.employee_ids, JSON_ARRAY()),
			COALESCE(c.content, ''),
			COALESCE(c.msg_type, ''),
			c.trigger_at,
			COALESCE(c.status, 0),
			c.created_at,
			c.updated_at
		FROM mc_room_quality_contact c
		JOIN mc_room_quality q ON q.id = c.quality_id
	`
}

func roomQualityContactWhere(filter dashboard.RoomQualityContactFilter) (string, []any) {
	where := "WHERE q.corp_id = ? AND q.deleted_at IS NULL AND c.deleted_at IS NULL"
	args := []any{filter.CorpID}
	if filter.QualityID > 0 {
		where += " AND c.quality_id = ?"
		args = append(args, filter.QualityID)
	}
	if filter.RoomID > 0 {
		where += " AND c.room_id = ?"
		args = append(args, filter.RoomID)
	}
	if strings.TrimSpace(filter.Nickname) != "" {
		where += " AND c.nickname LIKE ?"
		args = append(args, "%"+strings.TrimSpace(filter.Nickname)+"%")
	}
	if filter.Status >= 0 {
		where += " AND c.status = ?"
		args = append(args, filter.Status)
	}
	return where, args
}

type roomQualityScanner interface {
	Scan(dest ...any) error
}

func scanRoomQualityRow(scanner roomQualityScanner) (dashboard.RoomQualityItem, error) {
	var item dashboard.RoomQualityItem
	var ruleRaw, roomsRaw []byte
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.Name, &item.Description, &ruleRaw, &roomsRaw, &item.Status, &item.TenantID, &item.CorpID, &item.CreateUserID, &item.CreateUserName, &item.RoomNum, &item.ContactNum, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RoomQualityItem{}, err
	}
	item.RuleRaw = string(ruleRaw)
	item.RoomsRaw = string(roomsRaw)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanRoomQualityContactRow(scanner roomQualityScanner) (dashboard.RoomQualityContactItem, error) {
	var item dashboard.RoomQualityContactItem
	var employeeIDs []byte
	var triggerAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.QualityID, &item.RoomID, &item.RoomName, &item.ContactID, &item.ExternalUserID, &item.Nickname, &item.Avatar, &employeeIDs, &item.Content, &item.MsgType, &triggerAt, &item.Status, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RoomQualityContactItem{}, err
	}
	item.EmployeeIDsRaw = string(employeeIDs)
	item.TriggerAt = formatTime(triggerAt)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func roomQualityDefaultStatus(values dashboard.RoomQualityWrite) int {
	if !values.HasStatus {
		return 1
	}
	return values.Status
}
