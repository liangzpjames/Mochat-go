package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

const (
	operationOfficialAccountTypeRoomClockIn = 1
	operationOfficialAccountTypeLottery     = 2
	operationOfficialAccountTypeRoomFission = 8
)

func (s *MySQLStore) LotteryByIDNoCorp(ctx context.Context, id int) (dashboard.LotteryItem, dashboard.LotteryPrizeItem, bool, error) {
	item, ok, err := scanLottery(s.db.QueryRowContext(ctx, lotterySelectPrefix()+`
		WHERE l.id = ? AND l.deleted_at IS NULL
		GROUP BY l.id
		LIMIT 1
	`, id))
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

func (s *MySQLStore) EnsureLotteryOperationContact(ctx context.Context, lotteryID int, user dashboard.OperationWechatUser) (dashboard.LotteryContactItem, error) {
	unionID := strings.TrimSpace(user.UnionID)
	if unionID == "" {
		return dashboard.LotteryContactItem{}, fmt.Errorf("union_id必填")
	}
	item, found, err := s.lotteryOperationContactByUnion(ctx, lotteryID, unionID)
	if err != nil || found {
		return item, err
	}
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_lottery_contact
			(lottery_id, union_id, contact_id, nickname, avatar, employee_ids, city, source, grade, contact_tags, draw_num, win_num, status, write_off, created_at, updated_at)
		VALUES (?, ?, 0, ?, ?, JSON_ARRAY(), ?, ?, 0, JSON_ARRAY(), 0, 0, 0, 0, ?, ?)
	`, lotteryID, unionID, strings.TrimSpace(user.Nickname), strings.TrimSpace(user.Avatar), strings.TrimSpace(user.City), strings.TrimSpace(user.Source), now, now)
	if err != nil {
		return dashboard.LotteryContactItem{}, err
	}
	id64, err := result.LastInsertId()
	if err != nil {
		return dashboard.LotteryContactItem{}, err
	}
	item, found, err = s.lotteryOperationContactByID(ctx, lotteryID, int(id64))
	if err != nil {
		return dashboard.LotteryContactItem{}, err
	}
	if !found {
		return dashboard.LotteryContactItem{}, fmt.Errorf("抽奖客户创建失败")
	}
	return item, nil
}

func (s *MySQLStore) LotteryOperationRecords(ctx context.Context, lotteryID int, contactID int) ([]dashboard.OperationLotteryRecord, error) {
	rows, err := s.db.QueryContext(ctx, operationLotteryRecordSelect()+`
		WHERE lottery_id = ? AND contact_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
	`, lotteryID, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.OperationLotteryRecord, 0)
	for rows.Next() {
		item, err := scanOperationLotteryRecord(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) CreateLotteryOperationRecord(ctx context.Context, lotteryID int, contactID int, prizeID int, prizeName string, receiveType int, receiveQR string, receiveCode string) (dashboard.OperationLotteryRecord, error) {
	if receiveType <= 0 {
		receiveType = 2
	}
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.OperationLotteryRecord{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_lottery_contact_record
			(lottery_id, contact_id, prize_id, prize_name, receive_status, receive_qr, receive_type, receive_code, write_off, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, ?, ?, ?, 0, ?, ?)
	`, lotteryID, contactID, prizeID, strings.TrimSpace(prizeName), strings.TrimSpace(receiveQR), receiveType, strings.TrimSpace(receiveCode), now, now)
	if err != nil {
		return dashboard.OperationLotteryRecord{}, err
	}
	id64, err := result.LastInsertId()
	if err != nil {
		return dashboard.OperationLotteryRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_lottery_contact
		SET draw_num = COALESCE(draw_num, 0) + 1,
		    win_num = COALESCE(win_num, 0) + 1,
		    status = 1,
		    updated_at = ?
		WHERE id = ? AND lottery_id = ? AND deleted_at IS NULL
	`, now, contactID, lotteryID); err != nil {
		return dashboard.OperationLotteryRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.OperationLotteryRecord{}, err
	}
	return s.operationLotteryRecordByID(ctx, int(id64))
}

func (s *MySQLStore) MarkLotteryOperationRecordReceived(ctx context.Context, id int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_lottery_contact_record
		SET receive_status = 1, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`, time.Now(), id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) RoomClockInByIDNoCorp(ctx context.Context, id int) (dashboard.RoomClockInItem, bool, error) {
	item, err := scanRoomClockInRow(s.db.QueryRowContext(ctx, roomClockInSelect()+`
		WHERE c.id = ? AND c.deleted_at IS NULL
		LIMIT 1
	`, id))
	if err == sql.ErrNoRows {
		return dashboard.RoomClockInItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomClockInItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) EnsureRoomClockInOperationContact(ctx context.Context, clockInID int, user dashboard.OperationWechatUser) (dashboard.RoomClockInContactItem, error) {
	unionID := strings.TrimSpace(user.UnionID)
	if unionID == "" {
		return dashboard.RoomClockInContactItem{}, fmt.Errorf("union_id必填")
	}
	item, found, err := s.roomClockInOperationContactByUnion(ctx, clockInID, unionID)
	if err != nil || found {
		return item, err
	}
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_room_clock_in_contact
			(clock_in_id, union_id, openid, nickname, avatar, city, contact_id, employee_ids, contact_tags, day_count, status, receive_level, write_off, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, JSON_ARRAY(), JSON_ARRAY(), 0, 0, 0, 0, ?, ?)
	`, clockInID, unionID, strings.TrimSpace(user.OpenID), strings.TrimSpace(user.Nickname), strings.TrimSpace(user.Avatar), strings.TrimSpace(user.City), now, now)
	if err != nil {
		return dashboard.RoomClockInContactItem{}, err
	}
	id64, err := result.LastInsertId()
	if err != nil {
		return dashboard.RoomClockInContactItem{}, err
	}
	item, found, err = s.roomClockInOperationContactByID(ctx, clockInID, int(id64))
	if err != nil {
		return dashboard.RoomClockInContactItem{}, err
	}
	if !found {
		return dashboard.RoomClockInContactItem{}, fmt.Errorf("群打卡客户创建失败")
	}
	return item, nil
}

func (s *MySQLStore) RoomClockInOperationRecords(ctx context.Context, clockInID int, contactID int) ([]dashboard.RoomClockInDayRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, clock_in_id, contact_id, COALESCE(union_id, ''), DATE_FORMAT(day, '%Y-%m-%d'), created_at, updated_at
		FROM mc_room_clock_in_record
		WHERE clock_in_id = ? AND contact_id = ? AND deleted_at IS NULL
		ORDER BY day ASC
	`, clockInID, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomClockInDayRecord, 0)
	for rows.Next() {
		item, err := scanOperationRoomClockInDay(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) RecordRoomClockInOperationDay(ctx context.Context, clockInID int, user dashboard.OperationWechatUser) (dashboard.RoomClockInContactItem, bool, error) {
	contact, err := s.EnsureRoomClockInOperationContact(ctx, clockInID, user)
	if err != nil {
		return dashboard.RoomClockInContactItem{}, false, err
	}
	today := time.Now().Format("2006-01-02")
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT IGNORE INTO mc_room_clock_in_record
			(clock_in_id, contact_id, union_id, day, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, clockInID, contact.ID, contact.UnionID, today, now, now)
	if err != nil {
		return dashboard.RoomClockInContactItem{}, false, err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return dashboard.RoomClockInContactItem{}, false, err
	}
	if inserted > 0 {
		if _, err := s.db.ExecContext(ctx, `
			UPDATE mc_room_clock_in_contact
			SET day_count = COALESCE(day_count, 0) + 1,
			    first_clock_at = COALESCE(first_clock_at, ?),
			    last_clock_at = ?,
			    updated_at = ?
			WHERE id = ? AND clock_in_id = ? AND deleted_at IS NULL
		`, now, now, now, contact.ID, clockInID); err != nil {
			return dashboard.RoomClockInContactItem{}, false, err
		}
	}
	contact, _, err = s.roomClockInOperationContactByID(ctx, clockInID, contact.ID)
	return contact, inserted > 0, err
}

func (s *MySQLStore) UpdateRoomClockInOperationReceiveLevel(ctx context.Context, clockInID int, unionID string, level int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_clock_in_contact
		SET receive_level = GREATEST(COALESCE(receive_level, 0), ?), updated_at = ?
		WHERE clock_in_id = ? AND union_id = ? AND deleted_at IS NULL
	`, level, time.Now(), clockInID, strings.TrimSpace(unionID))
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) RoomClockInOperationRanking(ctx context.Context, clockInID int, unionID string) (int, []dashboard.RoomClockInContactItem, error) {
	rows, err := s.db.QueryContext(ctx, roomClockInContactSelect()+`
		WHERE c.clock_in_id = ? AND a.deleted_at IS NULL AND c.deleted_at IS NULL
		ORDER BY c.day_count DESC, c.id ASC
		LIMIT 100
	`, clockInID)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomClockInContactItem, 0)
	ranking := 0
	for rows.Next() {
		item, err := scanRoomClockInContactRow(rows)
		if err != nil {
			return 0, nil, err
		}
		items = append(items, item)
		if strings.TrimSpace(unionID) != "" && item.UnionID == strings.TrimSpace(unionID) {
			ranking = len(items)
		}
	}
	return ranking, items, rows.Err()
}

func (s *MySQLStore) RoomFissionBundleByIDNoCorp(ctx context.Context, id int) (dashboard.RoomFissionBundle, bool, error) {
	fission, err := scanRoomFissionInfo(s.db.QueryRowContext(ctx, roomFissionInfoSelect()+`
		WHERE f.id = ? AND f.deleted_at IS NULL
		LIMIT 1
	`, id))
	if err == sql.ErrNoRows {
		return dashboard.RoomFissionBundle{}, false, nil
	}
	if err != nil {
		return dashboard.RoomFissionBundle{}, false, err
	}
	poster, _, err := s.roomFissionPosterByFissionID(ctx, id)
	if err != nil {
		return dashboard.RoomFissionBundle{}, false, err
	}
	rooms, err := s.roomFissionRoomsByFissionID(ctx, fission.CorpID, id)
	if err != nil {
		return dashboard.RoomFissionBundle{}, false, err
	}
	welcome, _, err := s.roomFissionWelcomeByFissionID(ctx, id)
	if err != nil {
		return dashboard.RoomFissionBundle{}, false, err
	}
	invite, _, err := s.roomFissionInviteByFissionID(ctx, id)
	if err != nil {
		return dashboard.RoomFissionBundle{}, false, err
	}
	stats, err := s.RoomFissionOverview(ctx, fission.CorpID, id)
	if err != nil {
		return dashboard.RoomFissionBundle{}, false, err
	}
	return dashboard.RoomFissionBundle{Fission: fission, Poster: poster, Rooms: rooms, Welcome: welcome, Invite: invite, Stats: stats}, true, nil
}

func (s *MySQLStore) EnsureRoomFissionOperationContact(ctx context.Context, fissionID int, user dashboard.OperationWechatUser, parentUnionID string, roomID int) (dashboard.RoomFissionContactItem, error) {
	unionID := strings.TrimSpace(user.UnionID)
	if unionID == "" {
		return dashboard.RoomFissionContactItem{}, fmt.Errorf("union_id必填")
	}
	item, found, err := s.roomFissionOperationContactByUnion(ctx, fissionID, unionID)
	if err != nil || found {
		return item, err
	}
	parentUnionID = strings.TrimSpace(parentUnionID)
	if parentUnionID == "" {
		parentUnionID = "0"
	}
	level := 0
	if parentUnionID != "0" {
		level = 1
	}
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_room_fission_contact
			(fission_id, union_id, nickname, avatar, parent_union_id, level, contact_id, employee, invite_count, loss, status, receive_status, is_new, external_user_id, room_id, join_status, write_off, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, '', 0, 0, 0, 0, 0, '', ?, 0, 0, ?, ?)
	`, fissionID, unionID, strings.TrimSpace(user.Nickname), strings.TrimSpace(user.Avatar), parentUnionID, level, roomID, now, now)
	if err != nil {
		return dashboard.RoomFissionContactItem{}, err
	}
	id64, err := result.LastInsertId()
	if err != nil {
		return dashboard.RoomFissionContactItem{}, err
	}
	if parentUnionID != "0" {
		if _, err := s.db.ExecContext(ctx, `
			UPDATE mc_room_fission_contact
			SET invite_count = COALESCE(invite_count, 0) + 1, updated_at = ?
			WHERE fission_id = ? AND union_id = ? AND deleted_at IS NULL
		`, now, fissionID, parentUnionID); err != nil {
			return dashboard.RoomFissionContactItem{}, err
		}
	}
	item, found, err = s.roomFissionOperationContactByID(ctx, fissionID, int(id64))
	if err != nil {
		return dashboard.RoomFissionContactItem{}, err
	}
	if !found {
		return dashboard.RoomFissionContactItem{}, fmt.Errorf("群裂变客户创建失败")
	}
	return item, nil
}

func (s *MySQLStore) RoomFissionOperationChildren(ctx context.Context, fissionID int, parentUnionID string) ([]dashboard.RoomFissionContactItem, error) {
	rows, err := s.db.QueryContext(ctx, roomFissionContactSelect()+`
		WHERE c.fission_id = ? AND c.parent_union_id = ? AND f.deleted_at IS NULL AND c.deleted_at IS NULL
		ORDER BY c.id DESC
	`, fissionID, strings.TrimSpace(parentUnionID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomFissionContactItem, 0)
	for rows.Next() {
		item, err := scanRoomFissionContactRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) UpdateRoomFissionOperationReceiveStatus(ctx context.Context, fissionID int, unionID string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_fission_contact
		SET receive_status = 1, updated_at = ?
		WHERE fission_id = ? AND union_id = ? AND deleted_at IS NULL
	`, time.Now(), fissionID, strings.TrimSpace(unionID))
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *MySQLStore) RoomInfinitePullByIDNoCorp(ctx context.Context, id int) (dashboard.RoomInfinitePullItem, bool, error) {
	item, err := scanRoomInfinitePullRow(s.db.QueryRowContext(ctx, roomInfinitePullSelect()+`
		WHERE i.id = ? AND i.deleted_at IS NULL
		LIMIT 1
	`, id))
	if err == sql.ErrNoRows {
		return dashboard.RoomInfinitePullItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomInfinitePullItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) CreateShopCodeOperationRecord(ctx context.Context, corpID int, codeType int, shopID int) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_shop_code_record (type, corp_id, shop_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, codeType, corpID, shopID, time.Now(), time.Now())
	return err
}

func (s *MySQLStore) OperationOfficialAccountInfo(ctx context.Context, module string, id int, params map[string]any) (dashboard.OfficialAccountOAuthInfo, bool, error) {
	switch module {
	case "lottery":
		corpID, found, err := s.operationCorpID(ctx, `SELECT COALESCE(corp_id, 0) FROM mc_lottery WHERE id = ? AND deleted_at IS NULL LIMIT 1`, id)
		if err != nil || !found {
			return dashboard.OfficialAccountOAuthInfo{}, false, err
		}
		return s.OfficialAccountOAuthInfoByCorpIDType(ctx, corpID, operationOfficialAccountTypeLottery)
	case "roomClockIn":
		accountID, corpID, found, err := s.operationAccountAndCorpID(ctx, `SELECT COALESCE(official_account_id, 0), COALESCE(corp_id, 0) FROM mc_room_clock_in WHERE id = ? AND deleted_at IS NULL LIMIT 1`, id)
		if err != nil || !found {
			return dashboard.OfficialAccountOAuthInfo{}, false, err
		}
		if accountID > 0 {
			if info, found, err := s.OfficialAccountOAuthInfoByID(ctx, accountID); err != nil || found {
				return info, found, err
			}
		}
		return s.OfficialAccountOAuthInfoByCorpIDType(ctx, corpID, operationOfficialAccountTypeRoomClockIn)
	case "roomFission":
		accountID, corpID, found, err := s.operationAccountAndCorpID(ctx, `SELECT COALESCE(official_account_id, 0), COALESCE(corp_id, 0) FROM mc_room_fission WHERE id = ? AND deleted_at IS NULL LIMIT 1`, id)
		if err != nil || !found {
			return dashboard.OfficialAccountOAuthInfo{}, false, err
		}
		if accountID > 0 {
			if info, found, err := s.OfficialAccountOAuthInfoByID(ctx, accountID); err != nil || found {
				return info, found, err
			}
		}
		return s.OfficialAccountOAuthInfoByCorpIDType(ctx, corpID, operationOfficialAccountTypeRoomFission)
	case "shopCode":
		return s.OfficialAccountOAuthInfoByCorpID(ctx, id)
	default:
		return dashboard.OfficialAccountOAuthInfo{}, false, nil
	}
}

func (s *MySQLStore) lotteryOperationContactByUnion(ctx context.Context, lotteryID int, unionID string) (dashboard.LotteryContactItem, bool, error) {
	item, err := scanLotteryContactRow(s.db.QueryRowContext(ctx, lotteryContactSelect()+`
		WHERE c.lottery_id = ? AND c.union_id = ? AND c.deleted_at IS NULL AND l.deleted_at IS NULL
		LIMIT 1
	`, lotteryID, strings.TrimSpace(unionID)))
	if err == sql.ErrNoRows {
		return dashboard.LotteryContactItem{}, false, nil
	}
	if err != nil {
		return dashboard.LotteryContactItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) lotteryOperationContactByID(ctx context.Context, lotteryID int, id int) (dashboard.LotteryContactItem, bool, error) {
	item, err := scanLotteryContactRow(s.db.QueryRowContext(ctx, lotteryContactSelect()+`
		WHERE c.lottery_id = ? AND c.id = ? AND c.deleted_at IS NULL AND l.deleted_at IS NULL
		LIMIT 1
	`, lotteryID, id))
	if err == sql.ErrNoRows {
		return dashboard.LotteryContactItem{}, false, nil
	}
	if err != nil {
		return dashboard.LotteryContactItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) operationLotteryRecordByID(ctx context.Context, id int) (dashboard.OperationLotteryRecord, error) {
	return scanOperationLotteryRecord(s.db.QueryRowContext(ctx, operationLotteryRecordSelect()+`
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, id))
}

func operationLotteryRecordSelect() string {
	return `
		SELECT id, lottery_id, contact_id, prize_id, COALESCE(prize_name, ''), COALESCE(receive_status, 0), COALESCE(receive_qr, ''), COALESCE(receive_type, 0), COALESCE(receive_code, ''), COALESCE(write_off, 0), created_at, updated_at
		FROM mc_lottery_contact_record
	`
}

func scanOperationLotteryRecord(scanner interface{ Scan(dest ...any) error }) (dashboard.OperationLotteryRecord, error) {
	var item dashboard.OperationLotteryRecord
	var createdAt, updatedAt sql.NullTime
	if err := scanner.Scan(&item.ID, &item.LotteryID, &item.ContactID, &item.PrizeID, &item.PrizeName, &item.ReceiveStatus, &item.ReceiveQR, &item.ReceiveType, &item.ReceiveCode, &item.WriteOff, &createdAt, &updatedAt); err != nil {
		return dashboard.OperationLotteryRecord{}, err
	}
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func (s *MySQLStore) roomClockInOperationContactByUnion(ctx context.Context, clockInID int, unionID string) (dashboard.RoomClockInContactItem, bool, error) {
	item, err := scanRoomClockInContactRow(s.db.QueryRowContext(ctx, roomClockInContactSelect()+`
		WHERE c.clock_in_id = ? AND c.union_id = ? AND a.deleted_at IS NULL AND c.deleted_at IS NULL
		LIMIT 1
	`, clockInID, strings.TrimSpace(unionID)))
	if err == sql.ErrNoRows {
		return dashboard.RoomClockInContactItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomClockInContactItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) roomClockInOperationContactByID(ctx context.Context, clockInID int, id int) (dashboard.RoomClockInContactItem, bool, error) {
	item, err := scanRoomClockInContactRow(s.db.QueryRowContext(ctx, roomClockInContactSelect()+`
		WHERE c.clock_in_id = ? AND c.id = ? AND a.deleted_at IS NULL AND c.deleted_at IS NULL
		LIMIT 1
	`, clockInID, id))
	if err == sql.ErrNoRows {
		return dashboard.RoomClockInContactItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomClockInContactItem{}, false, err
	}
	return item, true, nil
}

func scanOperationRoomClockInDay(scanner interface{ Scan(dest ...any) error }) (dashboard.RoomClockInDayRecord, error) {
	var item dashboard.RoomClockInDayRecord
	var day string
	var createdAt, updatedAt sql.NullTime
	if err := scanner.Scan(&item.ID, &item.ClockInID, &item.ContactID, &item.UnionID, &day, &createdAt, &updatedAt); err != nil {
		return dashboard.RoomClockInDayRecord{}, err
	}
	item.Day = day
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func (s *MySQLStore) roomFissionOperationContactByUnion(ctx context.Context, fissionID int, unionID string) (dashboard.RoomFissionContactItem, bool, error) {
	item, err := scanRoomFissionContactRow(s.db.QueryRowContext(ctx, roomFissionContactSelect()+`
		WHERE c.fission_id = ? AND c.union_id = ? AND f.deleted_at IS NULL AND c.deleted_at IS NULL
		LIMIT 1
	`, fissionID, strings.TrimSpace(unionID)))
	if err == sql.ErrNoRows {
		return dashboard.RoomFissionContactItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomFissionContactItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) roomFissionOperationContactByID(ctx context.Context, fissionID int, id int) (dashboard.RoomFissionContactItem, bool, error) {
	item, err := scanRoomFissionContactRow(s.db.QueryRowContext(ctx, roomFissionContactSelect()+`
		WHERE c.fission_id = ? AND c.id = ? AND f.deleted_at IS NULL AND c.deleted_at IS NULL
		LIMIT 1
	`, fissionID, id))
	if err == sql.ErrNoRows {
		return dashboard.RoomFissionContactItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomFissionContactItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) operationCorpID(ctx context.Context, query string, id int) (int, bool, error) {
	var corpID int
	err := s.db.QueryRowContext(ctx, query, id).Scan(&corpID)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return corpID, corpID > 0, nil
}

func (s *MySQLStore) operationAccountAndCorpID(ctx context.Context, query string, id int) (int, int, bool, error) {
	var accountID int
	var corpID int
	err := s.db.QueryRowContext(ctx, query, id).Scan(&accountID, &corpID)
	if err == sql.ErrNoRows {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return accountID, corpID, corpID > 0, nil
}
