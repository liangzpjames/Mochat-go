package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) RoomClockInPage(ctx context.Context, filter dashboard.RoomClockInFilter) (dashboard.RoomClockInPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := roomClockInWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_room_clock_in c "+where, args...).Scan(&total); err != nil {
		return dashboard.RoomClockInPage{}, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, roomClockInSelect()+where+`
		ORDER BY c.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomClockInPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomClockInItem, 0)
	for rows.Next() {
		item, err := scanRoomClockInRow(rows)
		if err != nil {
			return dashboard.RoomClockInPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomClockInPage{}, err
	}
	return dashboard.RoomClockInPage{Items: items, Total: total, TotalPage: pageCount(total, filter.PerPage), Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) RoomClockInByID(ctx context.Context, corpID int, id int) (dashboard.RoomClockInItem, bool, error) {
	item, err := scanRoomClockInRow(s.db.QueryRowContext(ctx, roomClockInSelect()+`
		WHERE c.corp_id = ? AND c.id = ? AND c.deleted_at IS NULL
		LIMIT 1
	`, corpID, id))
	if err == sql.ErrNoRows {
		return dashboard.RoomClockInItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomClockInItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) CreateRoomClockIn(ctx context.Context, values dashboard.RoomClockInWrite) (int, error) {
	tenantID, err := s.tenantIDByCorpID(ctx, values.CorpID)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_room_clock_in
			(official_account_id, active_name, description, type, start_time, end_time, tasks, employee_qrcode, contact_tags, corp_card_status, corp_card, status, tenant_id, corp_id, create_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, values.OfficialAccountID, values.ActiveName, values.Description, roomClockInDefaultType(values.Type), roomClockInTimeArg(values.StartTime), roomClockInTimeArg(values.EndTime), jsonOrArray(values.TasksRaw), values.EmployeeQRCode, jsonOrArray(values.ContactTagsRaw), values.CorpCardStatus, jsonOrObject(values.CorpCardRaw), roomClockInCreateStatus(values), tenantID, values.CorpID, values.CreateUserID, now, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *MySQLStore) UpdateRoomClockIn(ctx context.Context, corpID int, id int, values dashboard.RoomClockInWrite) (bool, error) {
	oldPaths, tenantID, found, err := s.roomClockInStoragePaths(ctx, corpID, id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	sets := []string{"updated_at = ?"}
	args := []any{time.Now()}
	if values.HasOfficialAccountID {
		sets = append(sets, "official_account_id = ?")
		args = append(args, values.OfficialAccountID)
	}
	if values.HasActiveName {
		sets = append(sets, "active_name = ?")
		args = append(args, values.ActiveName)
	}
	if values.HasDescription {
		sets = append(sets, "description = ?")
		args = append(args, values.Description)
	}
	if values.HasType {
		sets = append(sets, "type = ?")
		args = append(args, roomClockInDefaultType(values.Type))
	}
	if values.HasStartTime {
		sets = append(sets, "start_time = ?")
		args = append(args, roomClockInTimeArg(values.StartTime))
	}
	if values.HasEndTime {
		sets = append(sets, "end_time = ?")
		args = append(args, roomClockInTimeArg(values.EndTime))
	}
	if values.HasTasks {
		sets = append(sets, "tasks = ?")
		args = append(args, jsonOrArray(values.TasksRaw))
	}
	if values.HasEmployeeQRCode {
		sets = append(sets, "employee_qrcode = ?")
		args = append(args, values.EmployeeQRCode)
	}
	if values.HasContactTags {
		sets = append(sets, "contact_tags = ?")
		args = append(args, jsonOrArray(values.ContactTagsRaw))
	}
	if values.HasCorpCardStatus {
		sets = append(sets, "corp_card_status = ?")
		args = append(args, values.CorpCardStatus)
	}
	if values.HasCorpCard {
		sets = append(sets, "corp_card = ?")
		args = append(args, jsonOrObject(values.CorpCardRaw))
	}
	if values.HasStatus {
		sets = append(sets, "status = ?")
		args = append(args, roomClockInStatusValue(values.Status))
	}
	args = append(args, corpID, id)
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_clock_in
		SET `+strings.Join(sets, ", ")+`
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return affected > 0, err
	}
	newPaths, _, _, err := s.roomClockInStoragePaths(ctx, corpID, id)
	if err != nil {
		return true, err
	}
	reclaimPaths := storagePathsRemoved(oldPaths, newPaths)
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return true, err
	}
	return true, nil
}

func (s *MySQLStore) DeleteRoomClockIn(ctx context.Context, corpID int, id int) (bool, error) {
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	reclaimPaths, tenantID, found, err := s.roomClockInStoragePathsTx(ctx, tx, corpID, id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_room_clock_in
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
	for _, table := range []string{"mc_room_clock_in_contact", "mc_room_clock_in_record"} {
		if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET deleted_at = ?, updated_at = ? WHERE clock_in_id = ? AND deleted_at IS NULL`, now, now, id); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return true, err
	}
	return true, nil
}

func (s *MySQLStore) roomClockInStoragePaths(ctx context.Context, corpID int, id int) ([]string, int, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, false, err
	}
	defer tx.Rollback()
	paths, tenantID, found, err := s.roomClockInStoragePathsTx(ctx, tx, corpID, id)
	if err != nil {
		return nil, 0, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, false, err
	}
	return paths, tenantID, found, nil
}

func (s *MySQLStore) roomClockInStoragePathsTx(ctx context.Context, tx *sql.Tx, corpID int, id int) ([]string, int, bool, error) {
	tenantID := 0
	employeeQRCode := ""
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(employee_qrcode, ''), COALESCE(tenant_id, 0)
		FROM mc_room_clock_in
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
		LIMIT 1
	`, corpID, id).Scan(&employeeQRCode, &tenantID)
	if err == sql.ErrNoRows {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	return roomClockInStoragePathsFromFields(employeeQRCode), tenantID, true, nil
}

func roomClockInStoragePathsFromFields(values ...string) []string {
	paths := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if idx := strings.Index(value, "/static/"); idx >= 0 {
			value = value[idx+len("/static/"):]
		}
		value = strings.TrimPrefix(value, "static/")
		paths = append(paths, value)
	}
	return uniqueStorageRelativePaths(paths)
}

func (s *MySQLStore) RoomClockInContactPage(ctx context.Context, filter dashboard.RoomClockInContactFilter) (dashboard.RoomClockInContactPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := roomClockInContactWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_room_clock_in_contact c JOIN mc_room_clock_in a ON a.id = c.clock_in_id `+where, args...).Scan(&total); err != nil {
		return dashboard.RoomClockInContactPage{}, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, roomClockInContactSelect()+where+`
		ORDER BY c.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomClockInContactPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomClockInContactItem, 0)
	for rows.Next() {
		item, err := scanRoomClockInContactRow(rows)
		if err != nil {
			return dashboard.RoomClockInContactPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomClockInContactPage{}, err
	}
	return dashboard.RoomClockInContactPage{Items: items, Total: total, TotalPage: pageCount(total, filter.PerPage), Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) RoomClockInDayPage(ctx context.Context, corpID int, clockInID int, contactID int, unionID string, page int, perPage int) (dashboard.RoomClockInDayPage, error) {
	page = positivePage(page)
	perPage = positivePerPage(perPage, 31)
	where := `WHERE a.corp_id = ? AND a.deleted_at IS NULL AND r.deleted_at IS NULL`
	args := []any{corpID}
	if clockInID > 0 {
		where += ` AND r.clock_in_id = ?`
		args = append(args, clockInID)
	}
	if contactID > 0 {
		where += ` AND r.contact_id = ?`
		args = append(args, contactID)
	}
	if strings.TrimSpace(unionID) != "" {
		where += ` AND r.union_id = ?`
		args = append(args, strings.TrimSpace(unionID))
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_room_clock_in_record r JOIN mc_room_clock_in a ON a.id = r.clock_in_id `+where, args...).Scan(&total); err != nil {
		return dashboard.RoomClockInDayPage{}, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.clock_in_id, r.contact_id, COALESCE(r.union_id, ''), DATE_FORMAT(r.day, '%Y-%m-%d'), r.created_at, r.updated_at
		FROM mc_room_clock_in_record r
		JOIN mc_room_clock_in a ON a.id = r.clock_in_id
		`+where+`
		ORDER BY r.day DESC, r.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomClockInDayPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomClockInDayRecord, 0)
	for rows.Next() {
		var item dashboard.RoomClockInDayRecord
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.ClockInID, &item.ContactID, &item.UnionID, &item.Day, &createdAt, &updatedAt); err != nil {
			return dashboard.RoomClockInDayPage{}, err
		}
		item.CreatedAt = formatTime(createdAt)
		item.UpdatedAt = formatTime(updatedAt)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomClockInDayPage{}, err
	}
	return dashboard.RoomClockInDayPage{Items: items, Total: total, TotalPage: pageCount(total, perPage), Page: page, PerPage: perPage}, nil
}

func (s *MySQLStore) BatchTagRoomClockInContacts(ctx context.Context, corpID int, clockInID int, contactIDs []int, tagIDs []int) (int, error) {
	contactIDs = dedupePositive(contactIDs)
	tagIDs = dedupePositive(tagIDs)
	if len(contactIDs) == 0 || len(tagIDs) == 0 {
		return 0, nil
	}
	raw, _ := json.Marshal(tagIDs)
	placeholders := strings.TrimRight(strings.Repeat("?,", len(contactIDs)), ",")
	args := []any{string(raw), time.Now(), corpID, clockInID}
	args = append(args, intsToArgs(contactIDs)...)
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_clock_in_contact c
		JOIN mc_room_clock_in a ON a.id = c.clock_in_id
		SET c.contact_tags = ?, c.updated_at = ?
		WHERE a.corp_id = ? AND c.clock_in_id = ? AND c.id IN (`+placeholders+`) AND c.deleted_at IS NULL AND a.deleted_at IS NULL
	`, args...)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

func roomClockInSelect() string {
	return `
		SELECT
			c.id,
			COALESCE(c.official_account_id, 0),
			COALESCE(c.active_name, ''),
			COALESCE(c.description, ''),
			COALESCE(c.type, 1),
			c.start_time,
			c.end_time,
			COALESCE(c.tasks, JSON_ARRAY()),
			COALESCE(c.employee_qrcode, ''),
			COALESCE(c.contact_tags, JSON_ARRAY()),
			COALESCE(c.corp_card_status, 0),
			COALESCE(c.corp_card, JSON_OBJECT()),
			COALESCE(c.status, 1),
			COALESCE(c.tenant_id, 0),
			COALESCE(c.corp_id, 0),
			COALESCE(c.create_user_id, 0),
			COALESCE(u.name, ''),
			(SELECT COUNT(*) FROM mc_room_clock_in_contact cc WHERE cc.clock_in_id = c.id AND cc.deleted_at IS NULL),
			(SELECT COALESCE(SUM(cc.day_count), 0) FROM mc_room_clock_in_contact cc WHERE cc.clock_in_id = c.id AND cc.deleted_at IS NULL),
			(SELECT COUNT(*) FROM mc_room_clock_in_contact cc WHERE cc.clock_in_id = c.id AND cc.deleted_at IS NULL AND cc.receive_level > 0),
			c.created_at,
			c.updated_at
		FROM mc_room_clock_in c
		LEFT JOIN mc_user u ON u.id = c.create_user_id AND u.deleted_at IS NULL
	`
}

func roomClockInWhere(filter dashboard.RoomClockInFilter) (string, []any) {
	where := "WHERE c.corp_id = ? AND c.deleted_at IS NULL"
	args := []any{filter.CorpID}
	if filter.RestrictCreateUser {
		where += " AND c.create_user_id = ?"
		args = append(args, filter.CreateUserID)
	}
	if strings.TrimSpace(filter.ActiveName) != "" {
		where += " AND c.active_name LIKE ?"
		args = append(args, "%"+strings.TrimSpace(filter.ActiveName)+"%")
	}
	if filter.Status >= 0 {
		where += " AND c.status = ?"
		args = append(args, filter.Status)
	}
	return where, args
}

func roomClockInContactSelect() string {
	return `
		SELECT
			c.id,
			c.clock_in_id,
			COALESCE(c.union_id, ''),
			COALESCE(c.openid, ''),
			COALESCE(c.nickname, ''),
			COALESCE(c.avatar, ''),
			COALESCE(c.city, ''),
			COALESCE(c.contact_id, 0),
			COALESCE(c.employee_ids, JSON_ARRAY()),
			COALESCE(c.contact_tags, JSON_ARRAY()),
			COALESCE(c.day_count, 0),
			COALESCE(c.status, 0),
			COALESCE(c.receive_level, 0),
			COALESCE(c.write_off, 0),
			c.first_clock_at,
			c.last_clock_at,
			c.created_at,
			c.updated_at
		FROM mc_room_clock_in_contact c
		JOIN mc_room_clock_in a ON a.id = c.clock_in_id
	`
}

func roomClockInContactWhere(filter dashboard.RoomClockInContactFilter) (string, []any) {
	where := "WHERE a.corp_id = ? AND a.deleted_at IS NULL AND c.deleted_at IS NULL"
	args := []any{filter.CorpID}
	if filter.ClockInID > 0 {
		where += " AND c.clock_in_id = ?"
		args = append(args, filter.ClockInID)
	}
	if strings.TrimSpace(filter.Nickname) != "" {
		where += " AND c.nickname LIKE ?"
		args = append(args, "%"+strings.TrimSpace(filter.Nickname)+"%")
	}
	if filter.Status >= 0 {
		where += " AND c.status = ?"
		args = append(args, filter.Status)
	}
	if filter.WriteOff >= 0 {
		where += " AND c.write_off = ?"
		args = append(args, filter.WriteOff)
	}
	return where, args
}

type roomClockInScanner interface {
	Scan(dest ...any) error
}

func scanRoomClockInRow(scanner roomClockInScanner) (dashboard.RoomClockInItem, error) {
	var item dashboard.RoomClockInItem
	var startTime, endTime, createdAt, updatedAt sql.NullTime
	var tasks, contactTags, corpCard []byte
	err := scanner.Scan(&item.ID, &item.OfficialAccountID, &item.ActiveName, &item.Description, &item.Type, &startTime, &endTime, &tasks, &item.EmployeeQRCode, &contactTags, &item.CorpCardStatus, &corpCard, &item.Status, &item.TenantID, &item.CorpID, &item.CreateUserID, &item.CreateUserName, &item.ContactNum, &item.ClockInNum, &item.ReceiveNum, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RoomClockInItem{}, err
	}
	item.StartTime = formatTime(startTime)
	item.EndTime = formatTime(endTime)
	item.TasksRaw = string(tasks)
	item.ContactTagsRaw = string(contactTags)
	item.CorpCardRaw = string(corpCard)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanRoomClockInContactRow(scanner roomClockInScanner) (dashboard.RoomClockInContactItem, error) {
	var item dashboard.RoomClockInContactItem
	var employeeIDs, contactTags []byte
	var firstClockAt, lastClockAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.ClockInID, &item.UnionID, &item.OpenID, &item.Nickname, &item.Avatar, &item.City, &item.ContactID, &employeeIDs, &contactTags, &item.DayCount, &item.Status, &item.ReceiveLevel, &item.WriteOff, &firstClockAt, &lastClockAt, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RoomClockInContactItem{}, err
	}
	item.EmployeeIDsRaw = string(employeeIDs)
	item.ContactTagsRaw = string(contactTags)
	item.FirstClockAt = formatTime(firstClockAt)
	item.LastClockAt = formatTime(lastClockAt)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func roomClockInDefaultType(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func roomClockInCreateStatus(values dashboard.RoomClockInWrite) int {
	if !values.HasStatus {
		return 1
	}
	return roomClockInStatusValue(values.Status)
}

func roomClockInStatusValue(value int) int {
	if value < 0 {
		return 1
	}
	return value
}

func roomClockInTimeArg(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
