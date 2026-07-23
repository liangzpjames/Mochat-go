package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) RoomInfinitePullPage(ctx context.Context, filter dashboard.RoomInfinitePullFilter) (dashboard.RoomInfinitePullPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := roomInfinitePullWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_room_infinite i "+where, args...).Scan(&total); err != nil {
		return dashboard.RoomInfinitePullPage{}, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, roomInfinitePullSelect()+where+`
		ORDER BY i.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomInfinitePullPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomInfinitePullItem, 0)
	for rows.Next() {
		item, err := scanRoomInfinitePullRow(rows)
		if err != nil {
			return dashboard.RoomInfinitePullPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomInfinitePullPage{}, err
	}
	return dashboard.RoomInfinitePullPage{Items: items, Total: total, TotalPage: pageCount(total, filter.PerPage), Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) RoomInfinitePullByID(ctx context.Context, corpID int, id int) (dashboard.RoomInfinitePullItem, bool, error) {
	item, err := scanRoomInfinitePullRow(s.db.QueryRowContext(ctx, roomInfinitePullSelect()+`
		WHERE i.corp_id = ? AND i.id = ? AND i.deleted_at IS NULL
		LIMIT 1
	`, corpID, id))
	if err == sql.ErrNoRows {
		return dashboard.RoomInfinitePullItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomInfinitePullItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) CreateRoomInfinitePull(ctx context.Context, values dashboard.RoomInfinitePullWrite) (int, error) {
	tenantID, err := s.tenantIDByCorpID(ctx, values.CorpID)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_room_infinite
			(name, avatar, title_status, title, describe_status, `+"`describe`"+`, logo, qw_code, total_num, tenant_id, corp_id, create_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, values.Name, values.Avatar, roomInfinitePullDefaultTitleStatus(values), values.Title, roomInfinitePullDefaultDescribeStatus(values), values.Describe, values.Logo, jsonOrArray(values.QwCodeRaw), roomInfinitePullDefaultTotalNum(values), tenantID, values.CorpID, values.CreateUserID, now, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *MySQLStore) UpdateRoomInfinitePull(ctx context.Context, corpID int, id int, values dashboard.RoomInfinitePullWrite) (bool, error) {
	oldPaths, tenantID, found, err := s.roomInfinitePullStoragePaths(ctx, corpID, id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	sets := []string{"updated_at = ?"}
	args := []any{time.Now()}
	if values.HasName {
		sets = append(sets, "name = ?")
		args = append(args, values.Name)
	}
	if values.HasAvatar {
		sets = append(sets, "avatar = ?")
		args = append(args, values.Avatar)
	}
	if values.HasTitleStatus {
		sets = append(sets, "title_status = ?")
		args = append(args, values.TitleStatus)
	}
	if values.HasTitle {
		sets = append(sets, "title = ?")
		args = append(args, values.Title)
	}
	if values.HasDescribeStatus {
		sets = append(sets, "describe_status = ?")
		args = append(args, values.DescribeStatus)
	}
	if values.HasDescribe {
		sets = append(sets, "`describe` = ?")
		args = append(args, values.Describe)
	}
	if values.HasLogo {
		sets = append(sets, "logo = ?")
		args = append(args, values.Logo)
	}
	if values.HasQwCode {
		sets = append(sets, "qw_code = ?")
		args = append(args, jsonOrArray(values.QwCodeRaw))
	}
	if values.HasTotalNum {
		sets = append(sets, "total_num = ?")
		args = append(args, values.TotalNum)
	}
	args = append(args, corpID, id)
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_infinite
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
	newPaths, _, _, err := s.roomInfinitePullStoragePaths(ctx, corpID, id)
	if err != nil {
		return true, err
	}
	reclaimPaths := storagePathsRemoved(oldPaths, newPaths)
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return true, err
	}
	return true, nil
}

func (s *MySQLStore) DeleteRoomInfinitePull(ctx context.Context, corpID int, id int) (bool, error) {
	reclaimPaths, tenantID, found, err := s.roomInfinitePullStoragePaths(ctx, corpID, id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_infinite
		SET deleted_at = ?, updated_at = ?
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
	`, time.Now(), time.Now(), corpID, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return affected > 0, err
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return true, err
	}
	return true, nil
}

func (s *MySQLStore) roomInfinitePullStoragePaths(ctx context.Context, corpID int, id int) ([]string, int, bool, error) {
	tenantID := 0
	avatar := ""
	logo := ""
	var qwCodeRaw []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(avatar, ''), COALESCE(logo, ''), COALESCE(qw_code, JSON_ARRAY()), COALESCE(tenant_id, 0)
		FROM mc_room_infinite
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
		LIMIT 1
	`, corpID, id).Scan(&avatar, &logo, &qwCodeRaw, &tenantID)
	if err == sql.ErrNoRows {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	return roomInfinitePullStoragePathsFromFields(avatar, logo, string(qwCodeRaw)), tenantID, true, nil
}

func roomInfinitePullStoragePathsFromFields(avatar string, logo string, qwCodeRaw string) []string {
	paths := make([]string, 0, 4)
	for _, value := range []string{avatar, logo} {
		paths = appendRoomInfinitePullStoragePath(paths, value)
	}
	paths = appendRoomInfinitePullStoragePathsFromJSON(paths, qwCodeRaw)
	return uniqueStorageRelativePaths(paths)
}

func appendRoomInfinitePullStoragePathsFromJSON(paths []string, raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return paths
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return paths
	}
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			for _, key := range []string{"qrcode", "qrCode", "qr_code", "qrcodeUrl", "qrcode_url", "roomQrcodeUrl", "room_qrcode_url", "image", "pic", "pic_url", "url"} {
				paths = appendRoomInfinitePullStoragePath(paths, stringFromAny(typed[key]))
			}
		case string:
			paths = appendRoomInfinitePullStoragePath(paths, typed)
		}
	}
	walk(decoded)
	return paths
}

func appendRoomInfinitePullStoragePath(paths []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return paths
	}
	if idx := strings.Index(value, "/static/"); idx >= 0 {
		value = value[idx+len("/static/"):]
	}
	value = strings.TrimPrefix(value, "static/")
	return append(paths, value)
}

func roomInfinitePullSelect() string {
	return `
		SELECT
			i.id,
			COALESCE(i.name, ''),
			COALESCE(i.avatar, ''),
			COALESCE(i.title_status, 1),
			COALESCE(i.title, ''),
			COALESCE(i.describe_status, 1),
			COALESCE(i.` + "`describe`" + `, ''),
			COALESCE(i.logo, ''),
			COALESCE(i.qw_code, JSON_ARRAY()),
			COALESCE(i.total_num, 0),
			COALESCE(i.tenant_id, 0),
			COALESCE(i.corp_id, 0),
			COALESCE(i.create_user_id, 0),
			COALESCE(u.name, ''),
			i.created_at,
			i.updated_at
		FROM mc_room_infinite i
		LEFT JOIN mc_user u ON u.id = i.create_user_id AND u.deleted_at IS NULL
	`
}

func roomInfinitePullWhere(filter dashboard.RoomInfinitePullFilter) (string, []any) {
	where := "WHERE i.corp_id = ? AND i.deleted_at IS NULL"
	args := []any{filter.CorpID}
	if strings.TrimSpace(filter.Name) != "" {
		where += " AND i.name LIKE ?"
		args = append(args, "%"+strings.TrimSpace(filter.Name)+"%")
	}
	return where, args
}

type roomInfinitePullScanner interface {
	Scan(dest ...any) error
}

func scanRoomInfinitePullRow(scanner roomInfinitePullScanner) (dashboard.RoomInfinitePullItem, error) {
	var item dashboard.RoomInfinitePullItem
	var qwCodeRaw []byte
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.Name, &item.Avatar, &item.TitleStatus, &item.Title, &item.DescribeStatus, &item.Describe, &item.Logo, &qwCodeRaw, &item.TotalNum, &item.TenantID, &item.CorpID, &item.CreateUserID, &item.CreateUserName, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RoomInfinitePullItem{}, err
	}
	item.QwCodeRaw = string(qwCodeRaw)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func roomInfinitePullDefaultTitleStatus(values dashboard.RoomInfinitePullWrite) int {
	if !values.HasTitleStatus {
		return 1
	}
	return values.TitleStatus
}

func roomInfinitePullDefaultDescribeStatus(values dashboard.RoomInfinitePullWrite) int {
	if !values.HasDescribeStatus {
		return 1
	}
	return values.DescribeStatus
}

func roomInfinitePullDefaultTotalNum(values dashboard.RoomInfinitePullWrite) int {
	if !values.HasTotalNum || values.TotalNum < 0 {
		return 0
	}
	return values.TotalNum
}
