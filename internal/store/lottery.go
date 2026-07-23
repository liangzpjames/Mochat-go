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

func (s *MySQLStore) LotteryPage(ctx context.Context, filter dashboard.LotteryFilter) (dashboard.LotteryPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := lotteryFilterWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_lottery l "+where, args...).Scan(&total); err != nil {
		return dashboard.LotteryPage{}, err
	}
	totalPage := 0
	if filter.PerPage > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	offset := (filter.Page - 1) * filter.PerPage
	queryArgs := append(append([]any{}, args...), filter.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, lotterySelectPrefix()+where+`
		GROUP BY l.id
		ORDER BY l.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.LotteryPage{}, err
	}
	defer rows.Close()
	items, err := scanLotteryRows(rows)
	if err != nil {
		return dashboard.LotteryPage{}, err
	}
	return dashboard.LotteryPage{Items: items, Total: total, TotalPage: totalPage, Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) LotteryByID(ctx context.Context, corpID int, id int) (dashboard.LotteryItem, dashboard.LotteryPrizeItem, bool, error) {
	item, ok, err := scanLottery(s.db.QueryRowContext(ctx, lotterySelectPrefix()+`
		WHERE l.corp_id = ? AND l.id = ? AND l.deleted_at IS NULL
		LIMIT 1
	`, corpID, id))
	if err != nil || !ok {
		return item, dashboard.LotteryPrizeItem{}, ok, err
	}
	prize, _, err := scanLotteryPrize(s.db.QueryRowContext(ctx, lotteryPrizeSelect()+`
		WHERE p.lottery_id = ? AND p.deleted_at IS NULL
		ORDER BY p.id DESC
		LIMIT 1
	`, id))
	if err != nil {
		return dashboard.LotteryItem{}, dashboard.LotteryPrizeItem{}, false, err
	}
	return item, prize, true, nil
}

func (s *MySQLStore) CreateLottery(ctx context.Context, values dashboard.LotteryWrite) (int, error) {
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
		INSERT INTO mc_lottery
			(name, description, type, time_type, start_time, end_time, contact_tags, tenant_id, corp_id, create_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, values.Name, values.Description, values.Type, values.TimeType, values.StartTime, values.EndTime, jsonOrArray(values.ContactTagsRaw), tenantID, values.CorpID, values.CreateUserID, now, now)
	if err != nil {
		return 0, err
	}
	id64, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	id := int(id64)
	if err := upsertLotteryPrize(ctx, tx, id, values, now); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *MySQLStore) UpdateLottery(ctx context.Context, corpID int, id int, values dashboard.LotteryWrite) (bool, error) {
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	oldPaths, tenantID, found, err := s.lotteryStoragePathsTx(ctx, tx, corpID, id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	sets := []string{"updated_at = ?"}
	args := []any{now}
	if values.HasName {
		sets = append(sets, "name = ?")
		args = append(args, values.Name)
	}
	if values.HasDescription {
		sets = append(sets, "description = ?")
		args = append(args, values.Description)
	}
	if values.HasType {
		sets = append(sets, "type = ?")
		args = append(args, values.Type)
	}
	if values.HasTimeType {
		sets = append(sets, "time_type = ?")
		args = append(args, values.TimeType)
	}
	if values.HasStartTime {
		sets = append(sets, "start_time = ?")
		args = append(args, values.StartTime)
	}
	if values.HasEndTime {
		sets = append(sets, "end_time = ?")
		args = append(args, values.EndTime)
	}
	if values.HasContactTags {
		sets = append(sets, "contact_tags = ?")
		args = append(args, jsonOrArray(values.ContactTagsRaw))
	}
	args = append(args, corpID, id)
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_lottery
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
	if lotteryPrizeTouched(values) {
		if err := upsertLotteryPrize(ctx, tx, id, values, now); err != nil {
			return false, err
		}
	}
	newPaths, _, _, err := s.lotteryStoragePathsTx(ctx, tx, corpID, id)
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
	return true, nil
}

func (s *MySQLStore) DeleteLottery(ctx context.Context, corpID int, id int) (bool, error) {
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	reclaimPaths, tenantID, found, err := s.lotteryStoragePathsTx(ctx, tx, corpID, id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_lottery
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
	if _, err := tx.ExecContext(ctx, `UPDATE mc_lottery_prize SET deleted_at = ?, updated_at = ? WHERE lottery_id = ? AND deleted_at IS NULL`, now, now, id); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mc_lottery_contact SET deleted_at = ?, updated_at = ? WHERE lottery_id = ? AND deleted_at IS NULL`, now, now, id); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mc_lottery_contact_record SET deleted_at = ?, updated_at = ? WHERE lottery_id = ? AND deleted_at IS NULL`, now, now, id); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return true, err
	}
	return true, nil
}

func (s *MySQLStore) lotteryStoragePathsTx(ctx context.Context, tx *sql.Tx, corpID int, id int) ([]string, int, bool, error) {
	tenantID := 0
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(tenant_id, 0)
		FROM mc_lottery
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
		LIMIT 1
	`, corpID, id).Scan(&tenantID)
	if err == sql.ErrNoRows {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	paths := []string{}
	rows, err := tx.QueryContext(ctx, `
		SELECT
			COALESCE(prize_set, JSON_ARRAY()),
			COALESCE(exchange_set, JSON_OBJECT()),
			COALESCE(draw_set, JSON_OBJECT()),
			COALESCE(win_set, JSON_OBJECT()),
			COALESCE(corp_card, JSON_OBJECT())
		FROM mc_lottery_prize
		WHERE lottery_id = ? AND deleted_at IS NULL
	`, id)
	if err != nil {
		return nil, 0, false, err
	}
	for rows.Next() {
		var prizeSet, exchangeSet, drawSet, winSet, corpCard []byte
		if err := rows.Scan(&prizeSet, &exchangeSet, &drawSet, &winSet, &corpCard); err != nil {
			rows.Close()
			return nil, 0, false, err
		}
		paths = appendLotteryStoragePathsFromJSON(paths, string(prizeSet))
		paths = appendLotteryStoragePathsFromJSON(paths, string(exchangeSet))
		paths = appendLotteryStoragePathsFromJSON(paths, string(drawSet))
		paths = appendLotteryStoragePathsFromJSON(paths, string(winSet))
		paths = appendLotteryStoragePathsFromJSON(paths, string(corpCard))
	}
	if err := rows.Close(); err != nil {
		return nil, 0, false, err
	}
	if err := rows.Err(); err != nil {
		return nil, 0, false, err
	}
	rows, err = tx.QueryContext(ctx, `
		SELECT COALESCE(receive_qr, '')
		FROM mc_lottery_contact_record
		WHERE lottery_id = ? AND deleted_at IS NULL
	`, id)
	if err != nil {
		return nil, 0, false, err
	}
	for rows.Next() {
		receiveQR := ""
		if err := rows.Scan(&receiveQR); err != nil {
			rows.Close()
			return nil, 0, false, err
		}
		paths = appendLotteryStoragePath(paths, receiveQR)
	}
	if err := rows.Close(); err != nil {
		return nil, 0, false, err
	}
	if err := rows.Err(); err != nil {
		return nil, 0, false, err
	}
	return uniqueStorageRelativePaths(paths), tenantID, true, nil
}

func appendLotteryStoragePathsFromJSON(paths []string, raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return paths
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return paths
	}
	return appendLotteryStoragePathsFromAny(paths, decoded, "")
}

func appendLotteryStoragePathsFromAny(paths []string, value any, key string) []string {
	switch typed := value.(type) {
	case map[string]any:
		for childKey, childValue := range typed {
			paths = appendLotteryStoragePathsFromAny(paths, childValue, childKey)
		}
	case []any:
		for _, childValue := range typed {
			paths = appendLotteryStoragePathsFromAny(paths, childValue, key)
		}
	case string:
		if lotteryStoragePathKey(key) {
			paths = appendLotteryStoragePath(paths, typed)
		}
	}
	return paths
}

func lotteryStoragePathKey(key string) bool {
	switch key {
	case "avatar", "cover", "coverPic", "cover_pic", "coverUrl", "cover_url", "image", "imagePath", "image_path", "imageUrl", "image_url", "logo", "pic", "picUrl", "pic_url", "picture", "qrCode", "qr_code", "qrcode", "qrcodeUrl", "qrcode_url", "receiveQr", "receive_qr", "url":
		return true
	default:
		return false
	}
}

func appendLotteryStoragePath(paths []string, value string) []string {
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

func (s *MySQLStore) LotteryContactPage(ctx context.Context, filter dashboard.LotteryContactFilter) (dashboard.LotteryContactPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := lotteryContactWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_lottery_contact c JOIN mc_lottery l ON l.id = c.lottery_id "+where, args...).Scan(&total); err != nil {
		return dashboard.LotteryContactPage{}, err
	}
	totalPage := 0
	if filter.PerPage > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	offset := (filter.Page - 1) * filter.PerPage
	queryArgs := append(append([]any{}, args...), filter.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, lotteryContactSelect()+where+`
		ORDER BY c.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.LotteryContactPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.LotteryContactItem, 0)
	for rows.Next() {
		item, err := scanLotteryContactRow(rows)
		if err != nil {
			return dashboard.LotteryContactPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.LotteryContactPage{}, err
	}
	return dashboard.LotteryContactPage{Items: items, Total: total, TotalPage: totalPage, Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) WriteOffLotteryContact(ctx context.Context, corpID int, lotteryID int, contactID int) (bool, error) {
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_lottery_contact c
		JOIN mc_lottery l ON l.id = c.lottery_id
		SET c.write_off = 1, c.updated_at = ?
		WHERE l.corp_id = ? AND c.lottery_id = ? AND c.id = ? AND c.deleted_at IS NULL AND l.deleted_at IS NULL
	`, now, corpID, lotteryID, contactID)
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
		UPDATE mc_lottery_contact_record
		SET write_off = 1, updated_at = ?
		WHERE lottery_id = ? AND contact_id = ? AND deleted_at IS NULL
	`, now, lotteryID, contactID); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) BatchTagLotteryContacts(ctx context.Context, corpID int, lotteryID int, contactIDs []int, tagIDs []int) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now()
	affected := 0
	for _, contactRecordID := range dedupePositive(contactIDs) {
		var workContactID int
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(c.contact_id, 0)
			FROM mc_lottery_contact c
			JOIN mc_lottery l ON l.id = c.lottery_id
			WHERE l.corp_id = ? AND c.lottery_id = ? AND c.id = ? AND c.deleted_at IS NULL AND l.deleted_at IS NULL
			LIMIT 1
		`, corpID, lotteryID, contactRecordID).Scan(&workContactID); err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return 0, err
		}
		rawTags := intSliceJSON(dedupePositive(tagIDs))
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_lottery_contact
			SET contact_tags = ?, updated_at = ?
			WHERE lottery_id = ? AND id = ? AND deleted_at IS NULL
		`, rawTags, now, lotteryID, contactRecordID); err != nil {
			return 0, err
		}
		if workContactID <= 0 {
			continue
		}
		for _, tagID := range dedupePositive(tagIDs) {
			var exists int
			if err := tx.QueryRowContext(ctx, `
				SELECT COUNT(*)
				FROM mc_work_contact_tag_pivot
				WHERE contact_id = ? AND contact_tag_id = ? AND deleted_at IS NULL
			`, workContactID, tagID).Scan(&exists); err != nil {
				return 0, err
			}
			if exists > 0 {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO mc_work_contact_tag_pivot (contact_id, employee_id, contact_tag_id, type, created_at, updated_at)
				VALUES (?, 0, ?, 1, ?, ?)
			`, workContactID, tagID, now, now); err != nil {
				return 0, err
			}
			affected++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return affected, nil
}

func upsertLotteryPrize(ctx context.Context, tx *sql.Tx, lotteryID int, values dashboard.LotteryWrite, now time.Time) error {
	var id int
	err := tx.QueryRowContext(ctx, `SELECT id FROM mc_lottery_prize WHERE lottery_id = ? AND deleted_at IS NULL ORDER BY id DESC LIMIT 1`, lotteryID).Scan(&id)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO mc_lottery_prize (lottery_id, prize_set, is_show, exchange_set, draw_set, win_set, corp_card, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, lotteryID, jsonOrArray(values.PrizeSetRaw), values.IsShow, jsonOrObject(values.ExchangeSetRaw), jsonOrObject(values.DrawSetRaw), jsonOrObject(values.WinSetRaw), jsonOrObject(values.CorpCardRaw), now, now)
		return err
	}
	sets := []string{"updated_at = ?"}
	args := []any{now}
	if values.HasPrizeSet {
		sets = append(sets, "prize_set = ?")
		args = append(args, jsonOrArray(values.PrizeSetRaw))
	}
	if values.HasIsShow {
		sets = append(sets, "is_show = ?")
		args = append(args, values.IsShow)
	}
	if values.HasExchangeSet {
		sets = append(sets, "exchange_set = ?")
		args = append(args, jsonOrObject(values.ExchangeSetRaw))
	}
	if values.HasDrawSet {
		sets = append(sets, "draw_set = ?")
		args = append(args, jsonOrObject(values.DrawSetRaw))
	}
	if values.HasWinSet {
		sets = append(sets, "win_set = ?")
		args = append(args, jsonOrObject(values.WinSetRaw))
	}
	if values.HasCorpCard {
		sets = append(sets, "corp_card = ?")
		args = append(args, jsonOrObject(values.CorpCardRaw))
	}
	args = append(args, id)
	_, err = tx.ExecContext(ctx, `UPDATE mc_lottery_prize SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	return err
}

func lotteryPrizeTouched(values dashboard.LotteryWrite) bool {
	return values.HasPrizeSet || values.HasIsShow || values.HasExchangeSet || values.HasDrawSet || values.HasWinSet || values.HasCorpCard
}

func lotterySelectPrefix() string {
	return `
		SELECT
			l.id,
			COALESCE(l.name, ''),
			COALESCE(l.description, ''),
			COALESCE(l.type, ''),
			COALESCE(l.time_type, 0),
			COALESCE(l.start_time, ''),
			COALESCE(l.end_time, ''),
			COALESCE(l.contact_tags, JSON_ARRAY()),
			COALESCE(l.tenant_id, 0),
			COALESCE(l.corp_id, 0),
			COALESCE(l.create_user_id, 0),
			COALESCE(u.name, ''),
			COUNT(DISTINCT lc.id) AS contact_num,
			COALESCE(SUM(CASE WHEN lc.win_num > 0 THEN 1 ELSE 0 END), 0) AS win_num,
			l.created_at,
			l.updated_at
		FROM mc_lottery l
		LEFT JOIN mc_user u ON u.id = l.create_user_id AND u.deleted_at IS NULL
		LEFT JOIN mc_lottery_contact lc ON lc.lottery_id = l.id AND lc.deleted_at IS NULL
	`
}

func lotteryFilterWhere(filter dashboard.LotteryFilter) (string, []any) {
	where := "WHERE l.corp_id = ? AND l.deleted_at IS NULL"
	args := []any{filter.CorpID}
	if strings.TrimSpace(filter.Name) != "" {
		where += " AND l.name LIKE ?"
		args = append(args, "%"+strings.TrimSpace(filter.Name)+"%")
	}
	return where, args
}

func lotteryPrizeSelect() string {
	return `
		SELECT
			p.id,
			p.lottery_id,
			COALESCE(p.prize_set, JSON_ARRAY()),
			COALESCE(p.is_show, 0),
			COALESCE(p.exchange_set, JSON_OBJECT()),
			COALESCE(p.draw_set, JSON_OBJECT()),
			COALESCE(p.win_set, JSON_OBJECT()),
			COALESCE(p.corp_card, JSON_OBJECT()),
			p.created_at,
			p.updated_at
		FROM mc_lottery_prize p
	`
}

func lotteryContactSelect() string {
	return `
		SELECT
			c.id,
			c.lottery_id,
			COALESCE(c.union_id, ''),
			COALESCE(c.contact_id, 0),
			COALESCE(c.nickname, ''),
			COALESCE(c.avatar, ''),
			COALESCE(c.employee_ids, ''),
			COALESCE(c.city, ''),
			COALESCE(c.source, ''),
			COALESCE(c.grade, 0),
			COALESCE(c.contact_tags, JSON_ARRAY()),
			COALESCE(c.draw_num, 0),
			COALESCE(c.win_num, 0),
			COALESCE(c.status, 0),
			COALESCE(c.write_off, 0),
			COALESCE(r.prize_name, ''),
			COALESCE(r.receive_status, 0),
			COALESCE(r.receive_type, 0),
			COALESCE(r.receive_code, ''),
			COALESCE(r.receive_qr, ''),
			c.created_at,
			c.updated_at
		FROM mc_lottery_contact c
		JOIN mc_lottery l ON l.id = c.lottery_id
		LEFT JOIN (
			SELECT rr.*
			FROM mc_lottery_contact_record rr
			JOIN (
				SELECT contact_id, MAX(id) AS id
				FROM mc_lottery_contact_record
				WHERE deleted_at IS NULL
				GROUP BY contact_id
			) latest ON latest.id = rr.id
		) r ON r.contact_id = c.id
	`
}

func lotteryContactWhere(filter dashboard.LotteryContactFilter) (string, []any) {
	where := "WHERE l.corp_id = ? AND c.deleted_at IS NULL AND l.deleted_at IS NULL"
	args := []any{filter.CorpID}
	if filter.LotteryID > 0 {
		where += " AND c.lottery_id = ?"
		args = append(args, filter.LotteryID)
	}
	if strings.TrimSpace(filter.Name) != "" {
		where += " AND c.nickname LIKE ?"
		args = append(args, "%"+strings.TrimSpace(filter.Name)+"%")
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

type lotteryScanner interface {
	Scan(dest ...any) error
}

func scanLottery(scanner lotteryScanner) (dashboard.LotteryItem, bool, error) {
	item, err := scanLotteryRow(scanner)
	if err == sql.ErrNoRows {
		return dashboard.LotteryItem{}, false, nil
	}
	if err != nil {
		return dashboard.LotteryItem{}, false, err
	}
	return item, true, nil
}

func scanLotteryRows(rows *sql.Rows) ([]dashboard.LotteryItem, error) {
	items := make([]dashboard.LotteryItem, 0)
	for rows.Next() {
		item, err := scanLotteryRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanLotteryRow(scanner lotteryScanner) (dashboard.LotteryItem, error) {
	var item dashboard.LotteryItem
	var tags []byte
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.Name, &item.Description, &item.Type, &item.TimeType, &item.StartTime, &item.EndTime, &tags, &item.TenantID, &item.CorpID, &item.CreateUserID, &item.CreateUserName, &item.ContactNum, &item.WinNum, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.LotteryItem{}, err
	}
	item.ContactTagsRaw = string(tags)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanLotteryPrize(scanner lotteryScanner) (dashboard.LotteryPrizeItem, bool, error) {
	var item dashboard.LotteryPrizeItem
	var prizeSet, exchangeSet, drawSet, winSet, corpCard []byte
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.LotteryID, &prizeSet, &item.IsShow, &exchangeSet, &drawSet, &winSet, &corpCard, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return dashboard.LotteryPrizeItem{}, false, nil
	}
	if err != nil {
		return dashboard.LotteryPrizeItem{}, false, err
	}
	item.PrizeSetRaw = string(prizeSet)
	item.ExchangeRaw = string(exchangeSet)
	item.DrawSetRaw = string(drawSet)
	item.WinSetRaw = string(winSet)
	item.CorpCardRaw = string(corpCard)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, true, nil
}

func scanLotteryContactRow(scanner lotteryScanner) (dashboard.LotteryContactItem, error) {
	var item dashboard.LotteryContactItem
	var tags []byte
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.LotteryID, &item.UnionID, &item.ContactID, &item.Nickname, &item.Avatar, &item.EmployeeIDsRaw, &item.City, &item.Source, &item.Grade, &tags, &item.DrawNum, &item.WinNum, &item.Status, &item.WriteOff, &item.PrizeName, &item.ReceiveStatus, &item.ReceiveType, &item.ReceiveCode, &item.ReceiveQR, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.LotteryContactItem{}, err
	}
	item.ContactTagsRaw = string(tags)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func dedupePositive(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func intSliceJSON(values []int) string {
	if len(values) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value > 0 {
			parts = append(parts, strconv.Itoa(value))
		}
	}
	return "[" + strings.Join(parts, ",") + "]"
}
