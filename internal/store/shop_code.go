package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) ShopCodePage(ctx context.Context, filter dashboard.ShopCodeFilter) (dashboard.ShopCodePage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)

	where, args := shopCodeWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_shop_code "+where, args...).Scan(&total); err != nil {
		return dashboard.ShopCodePage{}, err
	}
	totalPage := 0
	if filter.PerPage > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	offset := (filter.Page - 1) * filter.PerPage
	query := shopCodeSelectPrefix() + where + " ORDER BY id DESC LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), filter.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return dashboard.ShopCodePage{}, err
	}
	defer rows.Close()
	items, err := scanShopCodeRows(rows)
	if err != nil {
		return dashboard.ShopCodePage{}, err
	}
	return dashboard.ShopCodePage{Items: items, Total: total, TotalPage: totalPage, Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) ShopCodeByID(ctx context.Context, corpID int, id int) (dashboard.ShopCodeItem, bool, error) {
	row := s.db.QueryRowContext(ctx, shopCodeSelectPrefix()+`
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
		LIMIT 1
	`, corpID, id)
	return scanShopCode(row)
}

func (s *MySQLStore) CreateShopCode(ctx context.Context, values dashboard.ShopCodeWrite) (int, error) {
	tenantID, err := s.tenantIDByCorpID(ctx, values.CorpID)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_shop_code
			(name, type, employee, employee_qrcode, qw_code, search_keyword, address, country, province, city, district, lat, lng, status, tenant_id, corp_id, create_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, values.Name, values.Type, shopCodeJSONOrEmpty(values.EmployeeRaw), shopCodeJSONOrEmpty(values.EmployeeQRCodeRaw), shopCodeJSONOrEmpty(values.QWCodeRaw),
		values.SearchKeyword, values.Address, values.Country, values.Province, values.City, values.District, values.Lat, values.Lng, shopCodeDefaultStatus(values), tenantID, values.CorpID, values.CreateUserID, now, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *MySQLStore) UpdateShopCode(ctx context.Context, corpID int, id int, values dashboard.ShopCodeWrite) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	oldPaths, tenantID, found, err := s.shopCodeStoragePathsTx(ctx, tx, corpID, id)
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
	if values.HasType {
		sets = append(sets, "type = ?")
		args = append(args, values.Type)
	}
	if values.HasEmployee {
		sets = append(sets, "employee = ?")
		args = append(args, shopCodeJSONOrEmpty(values.EmployeeRaw))
	}
	if values.HasEmployeeQRCode {
		sets = append(sets, "employee_qrcode = ?")
		args = append(args, shopCodeJSONOrEmpty(values.EmployeeQRCodeRaw))
	}
	if values.HasQWCode {
		sets = append(sets, "qw_code = ?")
		args = append(args, shopCodeJSONOrEmpty(values.QWCodeRaw))
	}
	if values.HasSearchKeyword {
		sets = append(sets, "search_keyword = ?")
		args = append(args, values.SearchKeyword)
	}
	if values.HasAddress {
		sets = append(sets, "address = ?")
		args = append(args, values.Address)
	}
	if values.HasCountry {
		sets = append(sets, "country = ?")
		args = append(args, values.Country)
	}
	if values.HasProvince {
		sets = append(sets, "province = ?")
		args = append(args, values.Province)
	}
	if values.HasCity {
		sets = append(sets, "city = ?")
		args = append(args, values.City)
	}
	if values.HasDistrict {
		sets = append(sets, "district = ?")
		args = append(args, values.District)
	}
	if values.HasLat {
		sets = append(sets, "lat = ?")
		args = append(args, values.Lat)
	}
	if values.HasLng {
		sets = append(sets, "lng = ?")
		args = append(args, values.Lng)
	}
	if values.HasStatus {
		sets = append(sets, "status = ?")
		args = append(args, values.Status)
	}
	args = append(args, corpID, id)
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_shop_code
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
	newPaths, _, _, err := s.shopCodeStoragePathsTx(ctx, tx, corpID, id)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	reclaimPaths := storagePathsRemoved(oldPaths, newPaths)
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return true, err
	}
	return affected > 0, nil
}

func (s *MySQLStore) UpdateShopCodeStatus(ctx context.Context, corpID int, id int, status int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_shop_code
		SET status = ?, updated_at = ?
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
	`, status, time.Now(), corpID, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *MySQLStore) DeleteShopCode(ctx context.Context, corpID int, id int) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	reclaimPaths, tenantID, found, err := s.shopCodeStoragePathsTx(ctx, tx, corpID, id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_shop_code
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
	if affected == 0 {
		return false, nil
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return true, err
	}
	return affected > 0, nil
}

func (s *MySQLStore) shopCodeStoragePathsTx(ctx context.Context, tx *sql.Tx, corpID int, id int) ([]string, int, bool, error) {
	var employeeQRCodeRaw, qwCodeRaw []byte
	tenantID := 0
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(employee_qrcode, JSON_ARRAY()), COALESCE(qw_code, JSON_ARRAY()), COALESCE(tenant_id, 0)
		FROM mc_shop_code
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
		LIMIT 1
	`, corpID, id).Scan(&employeeQRCodeRaw, &qwCodeRaw, &tenantID)
	if err == sql.ErrNoRows {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	return shopCodeStoragePathsFromFields(string(employeeQRCodeRaw), string(qwCodeRaw)), tenantID, true, nil
}

func (s *MySQLStore) ShopCodePageSetting(ctx context.Context, corpID int, codeType int) (dashboard.ShopCodePageSetting, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, type, COALESCE(title, ''), COALESCE(show_type, 1), COALESCE(`+"`default`"+`, JSON_OBJECT()), COALESCE(poster, ''), COALESCE(autoPass, 0),
		       COALESCE(tenant_id, 0), COALESCE(corp_id, 0), COALESCE(create_user_id, 0), created_at, updated_at
		FROM mc_shop_code_page
		WHERE corp_id = ? AND type = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, corpID, codeType)
	return scanShopCodePageSetting(row)
}

func (s *MySQLStore) UpsertShopCodePageSetting(ctx context.Context, values dashboard.ShopCodePageSettingWrite) (int, error) {
	tenantID, err := s.tenantIDByCorpID(ctx, values.CorpID)
	if err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var existingID int
	var oldDefaultRaw []byte
	var oldPoster string
	var existingTenantID int
	err = tx.QueryRowContext(ctx, `
		SELECT id, COALESCE(`+"`default`"+`, JSON_OBJECT()), COALESCE(poster, ''), COALESCE(tenant_id, 0)
		FROM mc_shop_code_page
		WHERE corp_id = ? AND type = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, values.CorpID, values.Type).Scan(&existingID, &oldDefaultRaw, &oldPoster, &existingTenantID)
	now := time.Now()
	if err == sql.ErrNoRows {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO mc_shop_code_page
				(type, title, show_type, `+"`default`"+`, poster, autoPass, tenant_id, corp_id, create_user_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, values.Type, values.Title, values.ShowType, shopCodeJSONOrObject(values.DefaultRaw), values.Poster, values.AutoPass, tenantID, values.CorpID, values.CreateUserID, now, now)
		if err != nil {
			return 0, err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return 0, err
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		return int(id), nil
	}
	if err != nil {
		return 0, err
	}
	oldPaths := shopCodePageSettingStoragePathsFromFields(string(oldDefaultRaw), oldPoster)
	_, err = tx.ExecContext(ctx, `
		UPDATE mc_shop_code_page
		SET title = ?, show_type = ?, `+"`default`"+` = ?, poster = ?, autoPass = ?, updated_at = ?
		WHERE id = ?
	`, values.Title, values.ShowType, shopCodeJSONOrObject(values.DefaultRaw), values.Poster, values.AutoPass, now, existingID)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	if existingTenantID > 0 {
		tenantID = existingTenantID
	}
	newPaths := shopCodePageSettingStoragePathsFromFields(values.DefaultRaw, values.Poster)
	reclaimPaths := storagePathsRemoved(oldPaths, newPaths)
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return existingID, err
	}
	return existingID, nil
}

func (s *MySQLStore) ShopCodeAddressSuggestions(ctx context.Context, corpID int, keyword string, city string) ([]dashboard.ShopCodeAddressSuggestion, error) {
	query := `
		SELECT id, COALESCE(name, ''), COALESCE(address, ''), COALESCE(search_keyword, ''),
		       COALESCE(country, ''), COALESCE(province, ''), COALESCE(city, ''), COALESCE(district, ''),
		       COALESCE(lat, ''), COALESCE(lng, '')
		FROM mc_shop_code
		WHERE corp_id = ? AND deleted_at IS NULL
	`
	args := []any{corpID}
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query += " AND (name LIKE ? OR address LIKE ? OR search_keyword LIKE ? OR province LIKE ? OR city LIKE ? OR district LIKE ?)"
		args = append(args, like, like, like, like, like, like)
	}
	if strings.TrimSpace(city) != "" {
		like := "%" + strings.TrimSpace(city) + "%"
		query += " AND (city LIKE ? OR district LIKE ? OR province LIKE ?)"
		args = append(args, like, like, like)
	}
	query += " ORDER BY id DESC LIMIT 20"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.ShopCodeAddressSuggestion, 0)
	for rows.Next() {
		var item dashboard.ShopCodeAddressSuggestion
		if err := rows.Scan(&item.ID, &item.Name, &item.Address, &item.SearchKeyword, &item.Country, &item.Province, &item.City, &item.District, &item.Lat, &item.Lng); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) ShopCodeCities(ctx context.Context, corpID int, keyword string) ([]dashboard.ShopCodeCity, error) {
	query := `
		SELECT COALESCE(country, ''), COALESCE(province, ''), COALESCE(city, ''), COALESCE(district, '')
		FROM mc_shop_code
		WHERE corp_id = ? AND deleted_at IS NULL
	`
	args := []any{corpID}
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query += " AND (country LIKE ? OR province LIKE ? OR city LIKE ? OR district LIKE ?)"
		args = append(args, like, like, like, like)
	}
	query += " GROUP BY country, province, city, district ORDER BY province ASC, city ASC, district ASC LIMIT 100"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.ShopCodeCity, 0)
	for rows.Next() {
		var item dashboard.ShopCodeCity
		if err := rows.Scan(&item.Country, &item.Province, &item.City, &item.District); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) ShopCodeOverview(ctx context.Context, corpID int, codeType int) (dashboard.ShopCodeOverview, error) {
	query := `
		SELECT COUNT(*), SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END), SUM(CASE WHEN status = 0 THEN 1 ELSE 0 END)
		FROM mc_shop_code
		WHERE corp_id = ? AND deleted_at IS NULL
	`
	args := []any{corpID}
	if codeType > 0 {
		query += " AND type = ?"
		args = append(args, codeType)
	}
	var overview dashboard.ShopCodeOverview
	var openTotal, closeTotal sql.NullInt64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&overview.ShopTotal, &openTotal, &closeTotal); err != nil {
		return dashboard.ShopCodeOverview{}, err
	}
	overview.OpenTotal = nullInt64(openTotal)
	overview.CloseTotal = nullInt64(closeTotal)
	recordQuery := `
		SELECT COUNT(*)
		FROM mc_shop_code_record
		WHERE corp_id = ? AND deleted_at IS NULL
	`
	recordArgs := []any{corpID}
	if codeType > 0 {
		recordQuery += " AND type = ?"
		recordArgs = append(recordArgs, codeType)
	}
	if err := s.db.QueryRowContext(ctx, recordQuery, recordArgs...).Scan(&overview.RecordTotal); err != nil {
		return dashboard.ShopCodeOverview{}, err
	}
	return overview, nil
}

func (s *MySQLStore) ShopCodeRecordPage(ctx context.Context, filter dashboard.ShopCodeRecordFilter) (dashboard.ShopCodeRecordPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where := " WHERE r.corp_id = ? AND r.deleted_at IS NULL"
	args := []any{filter.CorpID}
	if filter.Type > 0 {
		where += " AND r.type = ?"
		args = append(args, filter.Type)
	}
	if filter.ShopID > 0 {
		where += " AND r.shop_id = ?"
		args = append(args, filter.ShopID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_shop_code_record r "+where, args...).Scan(&total); err != nil {
		return dashboard.ShopCodeRecordPage{}, err
	}
	totalPage := 0
	if filter.PerPage > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	offset := (filter.Page - 1) * filter.PerPage
	query := `
		SELECT r.id, COALESCE(r.type, 0), COALESCE(r.corp_id, 0), COALESCE(r.shop_id, 0), COALESCE(s.name, ''), r.created_at
		FROM mc_shop_code_record r
		LEFT JOIN mc_shop_code s ON s.id = r.shop_id AND s.deleted_at IS NULL
	` + where + " ORDER BY r.id DESC LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), filter.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return dashboard.ShopCodeRecordPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.ShopCodeRecordItem, 0)
	for rows.Next() {
		var item dashboard.ShopCodeRecordItem
		var createdAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Type, &item.CorpID, &item.ShopID, &item.ShopName, &createdAt); err != nil {
			return dashboard.ShopCodeRecordPage{}, err
		}
		item.CreatedAt = formatTime(createdAt)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ShopCodeRecordPage{}, err
	}
	return dashboard.ShopCodeRecordPage{Items: items, Total: total, TotalPage: totalPage, Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) ShopCodeShopStatPage(ctx context.Context, filter dashboard.ShopCodeFilter) (dashboard.ShopCodeShopStatPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := shopCodeWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_shop_code "+where, args...).Scan(&total); err != nil {
		return dashboard.ShopCodeShopStatPage{}, err
	}
	totalPage := 0
	if filter.PerPage > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	offset := (filter.Page - 1) * filter.PerPage
	aliasWhere, aliasArgs := shopCodeWhereWithAlias(filter, "s.")
	query := `
		SELECT s.id, COALESCE(s.name, ''), COALESCE(s.type, 0), COALESCE(s.employee, JSON_ARRAY()), COALESCE(s.employee_qrcode, JSON_ARRAY()), COALESCE(s.qw_code, JSON_ARRAY()),
		       COALESCE(s.search_keyword, ''), COALESCE(s.address, ''), COALESCE(s.country, ''), COALESCE(s.province, ''), COALESCE(s.city, ''), COALESCE(s.district, ''),
		       COALESCE(s.lat, ''), COALESCE(s.lng, ''), COALESCE(s.status, 0), COALESCE(s.tenant_id, 0), COALESCE(s.corp_id, 0), COALESCE(s.create_user_id, 0),
		       s.created_at, s.updated_at, COUNT(r.id)
		FROM mc_shop_code s
		LEFT JOIN mc_shop_code_record r ON r.shop_id = s.id AND r.deleted_at IS NULL
	` + aliasWhere + `
		GROUP BY s.id
		ORDER BY s.id DESC
		LIMIT ? OFFSET ?
	`
	queryArgs := append(append([]any{}, aliasArgs...), filter.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return dashboard.ShopCodeShopStatPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.ShopCodeShopStat, 0)
	for rows.Next() {
		item, err := scanShopCodeStatRow(rows)
		if err != nil {
			return dashboard.ShopCodeShopStatPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ShopCodeShopStatPage{}, err
	}
	return dashboard.ShopCodeShopStatPage{Items: items, Total: total, TotalPage: totalPage, Page: filter.Page, PerPage: filter.PerPage}, nil
}

func shopCodeWhere(filter dashboard.ShopCodeFilter) (string, []any) {
	return shopCodeWhereWithAlias(filter, "")
}

func shopCodeWhereWithAlias(filter dashboard.ShopCodeFilter, alias string) (string, []any) {
	where := "WHERE corp_id = ? AND deleted_at IS NULL"
	args := []any{filter.CorpID}
	if filter.Type > 0 {
		where += " AND " + alias + "type = ?"
		args = append(args, filter.Type)
	}
	if filter.Status >= 0 {
		where += " AND " + alias + "status = ?"
		args = append(args, filter.Status)
	}
	if strings.TrimSpace(filter.Name) != "" {
		like := "%" + strings.TrimSpace(filter.Name) + "%"
		where += " AND (" + alias + "name LIKE ? OR " + alias + "search_keyword LIKE ? OR " + alias + "address LIKE ?)"
		args = append(args, like, like, like)
	}
	if strings.TrimSpace(filter.City) != "" {
		like := "%" + strings.TrimSpace(filter.City) + "%"
		where += " AND (" + alias + "province LIKE ? OR " + alias + "city LIKE ? OR " + alias + "district LIKE ?)"
		args = append(args, like, like, like)
	}
	if alias != "" {
		where = strings.Replace(where, "WHERE corp_id", "WHERE "+alias+"corp_id", 1)
		where = strings.Replace(where, "AND deleted_at", "AND "+alias+"deleted_at", 1)
	}
	return " " + where, args
}

func shopCodeSelectPrefix() string {
	return `
		SELECT id, COALESCE(name, ''), COALESCE(type, 0), COALESCE(employee, JSON_ARRAY()), COALESCE(employee_qrcode, JSON_ARRAY()), COALESCE(qw_code, JSON_ARRAY()),
		       COALESCE(search_keyword, ''), COALESCE(address, ''), COALESCE(country, ''), COALESCE(province, ''), COALESCE(city, ''), COALESCE(district, ''),
		       COALESCE(lat, ''), COALESCE(lng, ''), COALESCE(status, 0), COALESCE(tenant_id, 0), COALESCE(corp_id, 0), COALESCE(create_user_id, 0),
		       created_at, updated_at
		FROM mc_shop_code
	`
}

type shopCodeScanner interface {
	Scan(dest ...any) error
}

func scanShopCode(scanner shopCodeScanner) (dashboard.ShopCodeItem, bool, error) {
	item, err := scanShopCodeRow(scanner)
	if err == sql.ErrNoRows {
		return dashboard.ShopCodeItem{}, false, nil
	}
	if err != nil {
		return dashboard.ShopCodeItem{}, false, err
	}
	return item, true, nil
}

func scanShopCodeRows(rows *sql.Rows) ([]dashboard.ShopCodeItem, error) {
	items := make([]dashboard.ShopCodeItem, 0)
	for rows.Next() {
		item, err := scanShopCodeRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanShopCodeRow(scanner shopCodeScanner) (dashboard.ShopCodeItem, error) {
	var item dashboard.ShopCodeItem
	var employee, employeeQRCode, qwCode []byte
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.Type,
		&employee,
		&employeeQRCode,
		&qwCode,
		&item.SearchKeyword,
		&item.Address,
		&item.Country,
		&item.Province,
		&item.City,
		&item.District,
		&item.Lat,
		&item.Lng,
		&item.Status,
		&item.TenantID,
		&item.CorpID,
		&item.CreateUserID,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return dashboard.ShopCodeItem{}, err
	}
	item.EmployeeRaw = string(employee)
	item.EmployeeQRCodeRaw = string(employeeQRCode)
	item.QWCodeRaw = string(qwCode)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanShopCodeStatRow(scanner shopCodeScanner) (dashboard.ShopCodeShopStat, error) {
	var stat dashboard.ShopCodeShopStat
	var employee, employeeQRCode, qwCode []byte
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(
		&stat.ID,
		&stat.Name,
		&stat.Type,
		&employee,
		&employeeQRCode,
		&qwCode,
		&stat.SearchKeyword,
		&stat.Address,
		&stat.Country,
		&stat.Province,
		&stat.City,
		&stat.District,
		&stat.Lat,
		&stat.Lng,
		&stat.Status,
		&stat.TenantID,
		&stat.CorpID,
		&stat.CreateUserID,
		&createdAt,
		&updatedAt,
		&stat.RecordTotal,
	)
	if err != nil {
		return dashboard.ShopCodeShopStat{}, err
	}
	stat.EmployeeRaw = string(employee)
	stat.EmployeeQRCodeRaw = string(employeeQRCode)
	stat.QWCodeRaw = string(qwCode)
	stat.CreatedAt = formatTime(createdAt)
	stat.UpdatedAt = formatTime(updatedAt)
	return stat, nil
}

func scanShopCodePageSetting(scanner shopCodeScanner) (dashboard.ShopCodePageSetting, bool, error) {
	var item dashboard.ShopCodePageSetting
	var defaultRaw []byte
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(
		&item.ID,
		&item.Type,
		&item.Title,
		&item.ShowType,
		&defaultRaw,
		&item.Poster,
		&item.AutoPass,
		&item.TenantID,
		&item.CorpID,
		&item.CreateUserID,
		&createdAt,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return dashboard.ShopCodePageSetting{}, false, nil
	}
	if err != nil {
		return dashboard.ShopCodePageSetting{}, false, err
	}
	item.DefaultRaw = string(defaultRaw)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, true, nil
}

func positivePage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

func positivePerPage(perPage int, fallback int) int {
	if perPage <= 0 {
		return fallback
	}
	return perPage
}

func shopCodeStoragePathsFromFields(values ...string) []string {
	paths := []string{}
	for _, value := range values {
		paths = appendShopCodeStoragePathsFromJSON(paths, value)
	}
	return uniqueStorageRelativePaths(paths)
}

func shopCodePageSettingStoragePathsFromFields(defaultRaw string, poster string) []string {
	paths := appendShopCodeStoragePathsFromJSON(nil, defaultRaw)
	paths = appendShopCodeStoragePath(paths, poster)
	return uniqueStorageRelativePaths(paths)
}

func appendShopCodeStoragePathsFromJSON(paths []string, raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return paths
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return paths
	}
	return appendShopCodeStoragePathsFromAny(paths, decoded, "")
}

func appendShopCodeStoragePathsFromAny(paths []string, value any, key string) []string {
	switch typed := value.(type) {
	case map[string]any:
		for childKey, childValue := range typed {
			paths = appendShopCodeStoragePathsFromAny(paths, childValue, childKey)
		}
	case []any:
		for _, childValue := range typed {
			paths = appendShopCodeStoragePathsFromAny(paths, childValue, key)
		}
	case string:
		if key == "" || shopCodeStoragePathKey(key) {
			paths = appendShopCodeStoragePath(paths, typed)
		}
	}
	return paths
}

func shopCodeStoragePathKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(strings.TrimSpace(key)))
	switch normalized {
	case "avatar", "cover", "coverpic", "coverurl", "employeeqrcode", "image", "imagepath", "imageurl", "logo", "path", "pic", "picurl", "picture", "poster", "qr", "qrcode", "qrcodepath", "qrcodeurl", "qrpath", "qrurl", "roomqrcode", "roomqrcodeurl", "url":
		return true
	default:
		return false
	}
}

func appendShopCodeStoragePath(paths []string, value string) []string {
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

func shopCodeJSONOrEmpty(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "[]"
	}
	return raw
}

func shopCodeJSONOrObject(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}"
	}
	return raw
}

func shopCodeDefaultStatus(values dashboard.ShopCodeWrite) int {
	if values.HasStatus {
		return values.Status
	}
	return 1
}
