package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) RoomFissionPage(ctx context.Context, filter dashboard.RoomFissionFilter) (dashboard.RoomFissionPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := roomFissionWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_room_fission f "+where, args...).Scan(&total); err != nil {
		return dashboard.RoomFissionPage{}, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, roomFissionListSelect()+where+`
		ORDER BY f.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomFissionPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomFissionListItem, 0)
	for rows.Next() {
		item, err := scanRoomFissionListRow(rows)
		if err != nil {
			return dashboard.RoomFissionPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomFissionPage{}, err
	}
	return dashboard.RoomFissionPage{Items: items, Total: total, TotalPage: pageCount(total, filter.PerPage), Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) RoomFissionBundleByID(ctx context.Context, corpID int, id int) (dashboard.RoomFissionBundle, bool, error) {
	fission, found, err := s.roomFissionInfoByID(ctx, corpID, id)
	if err != nil || !found {
		return dashboard.RoomFissionBundle{}, found, err
	}
	poster, _, err := s.roomFissionPosterByFissionID(ctx, id)
	if err != nil {
		return dashboard.RoomFissionBundle{}, false, err
	}
	rooms, err := s.roomFissionRoomsByFissionID(ctx, corpID, id)
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
	stats, err := s.RoomFissionOverview(ctx, corpID, id)
	if err != nil {
		return dashboard.RoomFissionBundle{}, false, err
	}
	return dashboard.RoomFissionBundle{Fission: fission, Poster: poster, Rooms: rooms, Welcome: welcome, Invite: invite, Stats: stats}, true, nil
}

func (s *MySQLStore) CreateRoomFission(ctx context.Context, values dashboard.RoomFissionWrite) (int, error) {
	tenantID, err := s.tenantIDByCorpID(ctx, values.Fission.CorpID)
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
		INSERT INTO mc_room_fission
			(official_account_id, active_name, end_time, target_count, new_friend, delete_invalid, receive_employees, auto_pass, status, tenant_id, corp_id, create_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, values.Fission.OfficialAccountID, values.Fission.ActiveName, roomFissionTimeArg(values.Fission.EndTime), values.Fission.TargetCount, values.Fission.NewFriend, values.Fission.DeleteInvalid, jsonOrArray(values.Fission.ReceiveEmployeesRaw), values.Fission.AutoPass, roomFissionDefaultStatus(values.Fission.Status), tenantID, values.Fission.CorpID, values.Fission.CreateUserID, now, now)
	if err != nil {
		return 0, err
	}
	id64, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	id := int(id64)
	if err := replaceRoomFissionPoster(ctx, tx, id, values.Poster, now); err != nil {
		return 0, err
	}
	if err := replaceRoomFissionRooms(ctx, tx, id, values.Rooms, values.RoomsTouched, now); err != nil {
		return 0, err
	}
	if err := replaceRoomFissionWelcome(ctx, tx, id, values.Welcome, now); err != nil {
		return 0, err
	}
	if values.Invite.Touched {
		values.Invite.FissionID = id
		if err := upsertRoomFissionInviteTx(ctx, tx, id, values.Invite, now); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *MySQLStore) UpdateRoomFission(ctx context.Context, corpID int, id int, values dashboard.RoomFissionWrite) (bool, error) {
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	oldPaths, tenantID, found, err := s.roomFissionStoragePathsTx(ctx, tx, corpID, id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	sets := []string{"updated_at = ?"}
	args := []any{now}
	if values.Fission.HasOfficialAccountID {
		sets = append(sets, "official_account_id = ?")
		args = append(args, values.Fission.OfficialAccountID)
	}
	if values.Fission.HasActiveName {
		sets = append(sets, "active_name = ?")
		args = append(args, values.Fission.ActiveName)
	}
	if values.Fission.HasEndTime {
		sets = append(sets, "end_time = ?")
		args = append(args, roomFissionTimeArg(values.Fission.EndTime))
	}
	if values.Fission.HasTargetCount {
		sets = append(sets, "target_count = ?")
		args = append(args, values.Fission.TargetCount)
	}
	if values.Fission.HasNewFriend {
		sets = append(sets, "new_friend = ?")
		args = append(args, values.Fission.NewFriend)
	}
	if values.Fission.HasDeleteInvalid {
		sets = append(sets, "delete_invalid = ?")
		args = append(args, values.Fission.DeleteInvalid)
	}
	if values.Fission.HasReceiveEmployees {
		sets = append(sets, "receive_employees = ?")
		args = append(args, jsonOrArray(values.Fission.ReceiveEmployeesRaw))
	}
	if values.Fission.HasAutoPass {
		sets = append(sets, "auto_pass = ?")
		args = append(args, values.Fission.AutoPass)
	}
	if values.Fission.HasStatus {
		sets = append(sets, "status = ?")
		args = append(args, roomFissionDefaultStatus(values.Fission.Status))
	}
	args = append(args, corpID, id)
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_room_fission
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
	if err := replaceRoomFissionPoster(ctx, tx, id, values.Poster, now); err != nil {
		return false, err
	}
	if err := replaceRoomFissionRooms(ctx, tx, id, values.Rooms, values.RoomsTouched, now); err != nil {
		return false, err
	}
	if err := replaceRoomFissionWelcome(ctx, tx, id, values.Welcome, now); err != nil {
		return false, err
	}
	if values.Invite.Touched {
		values.Invite.FissionID = id
		if err := upsertRoomFissionInviteTx(ctx, tx, id, values.Invite, now); err != nil {
			return false, err
		}
	}
	newPaths, _, _, err := s.roomFissionStoragePathsTx(ctx, tx, corpID, id)
	if err != nil {
		return false, err
	}
	reclaimPaths := storagePathsRemoved(oldPaths, newPaths)
	if err := tx.Commit(); err != nil {
		return false, err
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return true, err
	}
	return true, nil
}

func (s *MySQLStore) DeleteRoomFission(ctx context.Context, corpID int, id int) (bool, error) {
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	reclaimPaths, tenantID, found, err := s.roomFissionStoragePathsTx(ctx, tx, corpID, id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_room_fission
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
	for _, table := range []string{"mc_room_fission_poster", "mc_room_fission_room", "mc_room_fission_welcome", "mc_room_fission_invite", "mc_room_fission_contact"} {
		if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET deleted_at = ?, updated_at = ? WHERE fission_id = ? AND deleted_at IS NULL`, now, now, id); err != nil {
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

func (s *MySQLStore) UpsertRoomFissionInvite(ctx context.Context, corpID int, id int, values dashboard.RoomFissionInviteWrite) (bool, error) {
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	oldPaths, tenantID, found, err := s.roomFissionStoragePathsTx(ctx, tx, corpID, id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	values.FissionID = id
	if err := upsertRoomFissionInviteTx(ctx, tx, id, values, now); err != nil {
		return false, err
	}
	newPaths, _, _, err := s.roomFissionStoragePathsTx(ctx, tx, corpID, id)
	if err != nil {
		return false, err
	}
	reclaimPaths := storagePathsRemoved(oldPaths, newPaths)
	if err := tx.Commit(); err != nil {
		return false, err
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return true, err
	}
	return true, nil
}

func (s *MySQLStore) RoomFissionContactPage(ctx context.Context, filter dashboard.RoomFissionContactFilter) (dashboard.RoomFissionContactPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := roomFissionContactWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_room_fission_contact c JOIN mc_room_fission f ON f.id = c.fission_id `+where, args...).Scan(&total); err != nil {
		return dashboard.RoomFissionContactPage{}, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, roomFissionContactSelect()+where+`
		ORDER BY c.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomFissionContactPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomFissionContactItem, 0)
	for rows.Next() {
		item, err := scanRoomFissionContactRow(rows)
		if err != nil {
			return dashboard.RoomFissionContactPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomFissionContactPage{}, err
	}
	return dashboard.RoomFissionContactPage{Items: items, Total: total, TotalPage: pageCount(total, filter.PerPage), Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) RoomFissionRoomPage(ctx context.Context, filter dashboard.RoomFissionRoomFilter) (dashboard.RoomFissionRoomPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := roomFissionRoomWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_room_fission_room r JOIN mc_room_fission f ON f.id = r.fission_id `+where, args...).Scan(&total); err != nil {
		return dashboard.RoomFissionRoomPage{}, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, roomFissionRoomSelect()+where+`
		GROUP BY r.id
		ORDER BY r.id ASC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RoomFissionRoomPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomFissionRoom, 0)
	for rows.Next() {
		item, err := scanRoomFissionRoomRow(rows)
		if err != nil {
			return dashboard.RoomFissionRoomPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomFissionRoomPage{}, err
	}
	return dashboard.RoomFissionRoomPage{Items: items, Total: total, TotalPage: pageCount(total, filter.PerPage), Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) RoomFissionOverview(ctx context.Context, corpID int, id int) (dashboard.RoomFissionOverview, error) {
	stats := dashboard.RoomFissionOverview{}
	if id <= 0 {
		return stats, nil
	}
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(c.id),
			COALESCE(SUM(CASE WHEN c.status = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN c.write_off = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN c.join_status = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(c.invite_count), 0),
			COALESCE(SUM(CASE WHEN c.loss = 1 THEN 1 ELSE 0 END), 0)
		FROM mc_room_fission f
		LEFT JOIN mc_room_fission_contact c ON c.fission_id = f.id AND c.deleted_at IS NULL
		WHERE f.corp_id = ? AND f.id = ? AND f.deleted_at IS NULL
	`, corpID, id).Scan(&stats.ContactNum, &stats.CompleteNum, &stats.WriteOffNum, &stats.JoinRoomNum, &stats.InviteCount, &stats.LossNum)
	if err != nil {
		return dashboard.RoomFissionOverview{}, err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_room_fission_room
		WHERE fission_id = ? AND deleted_at IS NULL
	`, id).Scan(&stats.RoomNum); err != nil {
		return dashboard.RoomFissionOverview{}, err
	}
	return stats, nil
}

func (s *MySQLStore) WriteOffRoomFissionContact(ctx context.Context, corpID int, fissionID int, contactID int) (bool, error) {
	now := time.Now()
	where := []string{"f.corp_id = ?", "c.id = ?", "c.deleted_at IS NULL", "f.deleted_at IS NULL"}
	args := []any{corpID, contactID}
	if fissionID > 0 {
		where = append(where, "c.fission_id = ?")
		args = append(args, fissionID)
	}
	args = append([]any{now}, args...)
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_room_fission_contact c
		JOIN mc_room_fission f ON f.id = c.fission_id
		SET c.write_off = 1, c.receive_status = 1, c.updated_at = ?
		WHERE `+strings.Join(where, " AND "), args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *MySQLStore) roomFissionInfoByID(ctx context.Context, corpID int, id int) (dashboard.RoomFissionInfo, bool, error) {
	item, err := scanRoomFissionInfo(s.db.QueryRowContext(ctx, roomFissionInfoSelect()+`
		WHERE f.corp_id = ? AND f.id = ? AND f.deleted_at IS NULL
		LIMIT 1
	`, corpID, id))
	if err == sql.ErrNoRows {
		return dashboard.RoomFissionInfo{}, false, nil
	}
	if err != nil {
		return dashboard.RoomFissionInfo{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) roomFissionPosterByFissionID(ctx context.Context, id int) (dashboard.RoomFissionPoster, bool, error) {
	item, err := scanRoomFissionPoster(s.db.QueryRowContext(ctx, roomFissionPosterSelect()+`
		WHERE p.fission_id = ? AND p.deleted_at IS NULL
		ORDER BY p.id DESC
		LIMIT 1
	`, id))
	if err == sql.ErrNoRows {
		return dashboard.RoomFissionPoster{}, false, nil
	}
	if err != nil {
		return dashboard.RoomFissionPoster{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) roomFissionRoomsByFissionID(ctx context.Context, corpID int, id int) ([]dashboard.RoomFissionRoom, error) {
	rows, err := s.db.QueryContext(ctx, roomFissionRoomSelect()+`
		WHERE f.corp_id = ? AND r.fission_id = ? AND r.deleted_at IS NULL AND f.deleted_at IS NULL
		GROUP BY r.id
		ORDER BY r.id ASC
	`, corpID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomFissionRoom, 0)
	for rows.Next() {
		item, err := scanRoomFissionRoomRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) roomFissionWelcomeByFissionID(ctx context.Context, id int) (dashboard.RoomFissionWelcome, bool, error) {
	item, err := scanRoomFissionWelcome(s.db.QueryRowContext(ctx, roomFissionWelcomeSelect()+`
		WHERE w.fission_id = ? AND w.deleted_at IS NULL
		ORDER BY w.id DESC
		LIMIT 1
	`, id))
	if err == sql.ErrNoRows {
		return dashboard.RoomFissionWelcome{}, false, nil
	}
	if err != nil {
		return dashboard.RoomFissionWelcome{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) roomFissionInviteByFissionID(ctx context.Context, id int) (dashboard.RoomFissionInvite, bool, error) {
	item, err := scanRoomFissionInvite(s.db.QueryRowContext(ctx, roomFissionInviteSelect()+`
		WHERE i.fission_id = ? AND i.deleted_at IS NULL
		ORDER BY i.id DESC
		LIMIT 1
	`, id))
	if err == sql.ErrNoRows {
		return dashboard.RoomFissionInvite{}, false, nil
	}
	if err != nil {
		return dashboard.RoomFissionInvite{}, false, err
	}
	return item, true, nil
}

func replaceRoomFissionPoster(ctx context.Context, tx *sql.Tx, id int, values dashboard.RoomFissionPosterWrite, now time.Time) error {
	if !values.Touched {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mc_room_fission_poster SET deleted_at = ?, updated_at = ? WHERE fission_id = ? AND deleted_at IS NULL`, now, now, id); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mc_room_fission_poster
			(fission_id, cover_pic, avatar_show, nickname_show, nickname_color, qrcode_w, qrcode_h, qrcode_x, qrcode_y, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, values.CoverPic, values.AvatarShow, values.NicknameShow, values.NicknameColor, values.QRCodeW, values.QRCodeH, values.QRCodeX, values.QRCodeY, now, now)
	return err
}

func replaceRoomFissionRooms(ctx context.Context, tx *sql.Tx, id int, values []dashboard.RoomFissionRoomWrite, touched bool, now time.Time) error {
	if !touched {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mc_room_fission_room SET deleted_at = ?, updated_at = ? WHERE fission_id = ? AND deleted_at IS NULL`, now, now, id); err != nil {
		return err
	}
	for _, room := range values {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_room_fission_room
				(fission_id, room_qrcode, room_wx_qrcode, room, room_max, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, id, room.RoomQRCode, room.RoomWXQRCode, jsonOrObject(room.RoomRaw), room.RoomMax, now, now); err != nil {
			return err
		}
	}
	return nil
}

func replaceRoomFissionWelcome(ctx context.Context, tx *sql.Tx, id int, values dashboard.RoomFissionWelcomeWrite, now time.Time) error {
	if !values.Touched {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mc_room_fission_welcome SET deleted_at = ?, updated_at = ? WHERE fission_id = ? AND deleted_at IS NULL`, now, now, id); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mc_room_fission_welcome
			(fission_id, text, link_title, link_desc, link_pic, link_wx_url, template_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, values.Text, values.LinkTitle, values.LinkDesc, values.LinkPic, values.LinkWXURL, values.TemplateID, now, now)
	return err
}

func upsertRoomFissionInviteTx(ctx context.Context, tx *sql.Tx, id int, values dashboard.RoomFissionInviteWrite, now time.Time) error {
	var inviteID int
	err := tx.QueryRowContext(ctx, `SELECT id FROM mc_room_fission_invite WHERE fission_id = ? AND deleted_at IS NULL ORDER BY id DESC LIMIT 1`, id).Scan(&inviteID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO mc_room_fission_invite
				(fission_id, type, employees, choose_contact, text, link_title, link_desc, link_pic, wx_link_pic, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, roomFissionInviteType(values.Type), jsonOrArray(values.EmployeesRaw), jsonOrObject(values.ChooseContactRaw), values.Text, values.LinkTitle, values.LinkDesc, values.LinkPic, values.WXLinkPic, now, now)
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE mc_room_fission_invite
		SET type = ?, employees = ?, choose_contact = ?, text = ?, link_title = ?, link_desc = ?, link_pic = ?, wx_link_pic = ?, updated_at = ?
		WHERE id = ?
	`, roomFissionInviteType(values.Type), jsonOrArray(values.EmployeesRaw), jsonOrObject(values.ChooseContactRaw), values.Text, values.LinkTitle, values.LinkDesc, values.LinkPic, values.WXLinkPic, now, inviteID)
	return err
}

func (s *MySQLStore) roomFissionStoragePathsTx(ctx context.Context, tx *sql.Tx, corpID int, id int) ([]string, int, bool, error) {
	tenantID := 0
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(tenant_id, 0)
		FROM mc_room_fission
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
	var queryErr error
	paths, queryErr = appendRoomFissionStoragePathsFromQuery(ctx, tx, paths, `
		SELECT COALESCE(cover_pic, '')
		FROM mc_room_fission_poster
		WHERE fission_id = ? AND deleted_at IS NULL
	`, id)
	if queryErr != nil {
		return nil, 0, false, queryErr
	}
	paths, queryErr = appendRoomFissionStoragePathsFromQuery(ctx, tx, paths, `
		SELECT COALESCE(room_qrcode, ''), COALESCE(room_wx_qrcode, '')
		FROM mc_room_fission_room
		WHERE fission_id = ? AND deleted_at IS NULL
	`, id)
	if queryErr != nil {
		return nil, 0, false, queryErr
	}
	paths, queryErr = appendRoomFissionStoragePathsFromQuery(ctx, tx, paths, `
		SELECT COALESCE(link_pic, ''), COALESCE(link_wx_url, '')
		FROM mc_room_fission_welcome
		WHERE fission_id = ? AND deleted_at IS NULL
	`, id)
	if queryErr != nil {
		return nil, 0, false, queryErr
	}
	paths, queryErr = appendRoomFissionStoragePathsFromQuery(ctx, tx, paths, `
		SELECT COALESCE(link_pic, ''), COALESCE(wx_link_pic, '')
		FROM mc_room_fission_invite
		WHERE fission_id = ? AND deleted_at IS NULL
	`, id)
	if queryErr != nil {
		return nil, 0, false, queryErr
	}
	return roomFissionStoragePathsFromFields(paths...), tenantID, true, nil
}

func appendRoomFissionStoragePathsFromQuery(ctx context.Context, tx *sql.Tx, paths []string, query string, args ...any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	values := make([]sql.NullString, len(columns))
	dest := make([]any, len(columns))
	for i := range values {
		dest[i] = &values[i]
	}
	for rows.Next() {
		for i := range values {
			values[i] = sql.NullString{}
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		for _, value := range values {
			if value.Valid {
				paths = append(paths, value.String)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return paths, nil
}

func roomFissionStoragePathsFromFields(values ...string) []string {
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

func roomFissionWhere(filter dashboard.RoomFissionFilter) (string, []any) {
	where := []string{"f.corp_id = ?", "f.deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if strings.TrimSpace(filter.ActiveName) != "" {
		where = append(where, "f.active_name LIKE ?")
		args = append(args, "%"+strings.TrimSpace(filter.ActiveName)+"%")
	}
	if filter.RestrictCreateUser {
		where = append(where, "f.create_user_id = ?")
		args = append(args, filter.CreateUserID)
	}
	return "WHERE " + strings.Join(where, " AND "), args
}

func roomFissionContactWhere(filter dashboard.RoomFissionContactFilter) (string, []any) {
	where := []string{"f.corp_id = ?", "c.deleted_at IS NULL", "f.deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if filter.FissionID > 0 {
		where = append(where, "c.fission_id = ?")
		args = append(args, filter.FissionID)
	}
	if strings.TrimSpace(filter.Nickname) != "" {
		where = append(where, "c.nickname LIKE ?")
		args = append(args, "%"+strings.TrimSpace(filter.Nickname)+"%")
	}
	if filter.Status >= 0 {
		where = append(where, "c.status = ?")
		args = append(args, filter.Status)
	}
	if filter.WriteOff >= 0 {
		where = append(where, "c.write_off = ?")
		args = append(args, filter.WriteOff)
	}
	if filter.JoinStatus >= 0 {
		where = append(where, "c.join_status = ?")
		args = append(args, filter.JoinStatus)
	}
	if filter.ReceiveStatus >= 0 {
		where = append(where, "c.receive_status = ?")
		args = append(args, filter.ReceiveStatus)
	}
	return "WHERE " + strings.Join(where, " AND "), args
}

func roomFissionRoomWhere(filter dashboard.RoomFissionRoomFilter) (string, []any) {
	where := []string{"f.corp_id = ?", "r.deleted_at IS NULL", "f.deleted_at IS NULL"}
	args := []any{filter.CorpID}
	if filter.FissionID > 0 {
		where = append(where, "r.fission_id = ?")
		args = append(args, filter.FissionID)
	}
	return "WHERE " + strings.Join(where, " AND "), args
}

func roomFissionListSelect() string {
	return `
		SELECT
			f.id,
			COALESCE(f.official_account_id, 0),
			COALESCE(f.active_name, ''),
			f.end_time,
			COALESCE(f.target_count, 0),
			COALESCE(f.new_friend, 0),
			COALESCE(f.delete_invalid, 0),
			COALESCE(f.receive_employees, JSON_ARRAY()),
			COALESCE(f.auto_pass, 0),
			COALESCE(f.status, 0),
			COALESCE(f.tenant_id, 0),
			COALESCE(f.corp_id, 0),
			COALESCE(f.create_user_id, 0),
			COALESCE(u.name, ''),
			(SELECT COUNT(*) FROM mc_room_fission_contact c WHERE c.fission_id = f.id AND c.deleted_at IS NULL),
			(SELECT COUNT(*) FROM mc_room_fission_contact c WHERE c.fission_id = f.id AND c.deleted_at IS NULL AND c.status = 1),
			(SELECT COUNT(*) FROM mc_room_fission_room r WHERE r.fission_id = f.id AND r.deleted_at IS NULL),
			f.created_at,
			f.updated_at
		FROM mc_room_fission f
		LEFT JOIN mc_user u ON u.id = f.create_user_id AND u.deleted_at IS NULL
	`
}

func roomFissionInfoSelect() string {
	return `
		SELECT
			f.id,
			COALESCE(f.official_account_id, 0),
			COALESCE(f.active_name, ''),
			f.end_time,
			COALESCE(f.target_count, 0),
			COALESCE(f.new_friend, 0),
			COALESCE(f.delete_invalid, 0),
			COALESCE(f.receive_employees, JSON_ARRAY()),
			COALESCE(f.auto_pass, 0),
			COALESCE(f.status, 0),
			COALESCE(f.tenant_id, 0),
			COALESCE(f.corp_id, 0),
			COALESCE(f.create_user_id, 0),
			f.created_at,
			f.updated_at
		FROM mc_room_fission f
	`
}

func roomFissionPosterSelect() string {
	return `
		SELECT
			p.id,
			p.fission_id,
			COALESCE(p.cover_pic, ''),
			COALESCE(p.avatar_show, 0),
			COALESCE(p.nickname_show, 0),
			COALESCE(p.nickname_color, ''),
			COALESCE(p.qrcode_w, ''),
			COALESCE(p.qrcode_h, ''),
			COALESCE(p.qrcode_x, ''),
			COALESCE(p.qrcode_y, ''),
			p.created_at,
			p.updated_at
		FROM mc_room_fission_poster p
	`
}

func roomFissionRoomSelect() string {
	return `
		SELECT
			r.id,
			r.fission_id,
			COALESCE(r.room_qrcode, ''),
			COALESCE(r.room_wx_qrcode, ''),
			COALESCE(r.room, JSON_OBJECT()),
			COALESCE(r.room_max, 0),
			COUNT(DISTINCT c.id),
			COALESCE(SUM(CASE WHEN c.join_status = 1 THEN 1 ELSE 0 END), 0),
			r.created_at,
			r.updated_at
		FROM mc_room_fission_room r
		JOIN mc_room_fission f ON f.id = r.fission_id
		LEFT JOIN mc_room_fission_contact c
			ON c.fission_id = r.fission_id
			AND c.deleted_at IS NULL
			AND c.room_id = CAST(JSON_UNQUOTE(JSON_EXTRACT(r.room, '$.id')) AS UNSIGNED)
	`
}

func roomFissionWelcomeSelect() string {
	return `
		SELECT
			w.id,
			w.fission_id,
			COALESCE(w.text, ''),
			COALESCE(w.link_title, ''),
			COALESCE(w.link_desc, ''),
			COALESCE(w.link_pic, ''),
			COALESCE(w.link_wx_url, ''),
			COALESCE(w.template_id, ''),
			w.created_at,
			w.updated_at
		FROM mc_room_fission_welcome w
	`
}

func roomFissionInviteSelect() string {
	return `
		SELECT
			i.id,
			i.fission_id,
			COALESCE(i.type, 2),
			COALESCE(i.employees, JSON_ARRAY()),
			COALESCE(i.choose_contact, JSON_OBJECT()),
			COALESCE(i.text, ''),
			COALESCE(i.link_title, ''),
			COALESCE(i.link_desc, ''),
			COALESCE(i.link_pic, ''),
			COALESCE(i.wx_link_pic, ''),
			i.created_at,
			i.updated_at
		FROM mc_room_fission_invite i
	`
}

func roomFissionContactSelect() string {
	return `
		SELECT
			c.id,
			c.fission_id,
			COALESCE(c.union_id, ''),
			COALESCE(c.nickname, ''),
			COALESCE(c.avatar, ''),
			COALESCE(c.parent_union_id, ''),
			COALESCE(c.level, 0),
			COALESCE(c.contact_id, 0),
			COALESCE(c.employee, ''),
			COALESCE(c.invite_count, 0),
			COALESCE(c.loss, 0),
			COALESCE(c.status, 0),
			COALESCE(c.receive_status, 0),
			COALESCE(c.is_new, 0),
			COALESCE(c.external_user_id, ''),
			COALESCE(c.room_id, 0),
			COALESCE(c.join_status, 0),
			COALESCE(c.write_off, 0),
			c.created_at,
			c.updated_at
		FROM mc_room_fission_contact c
		JOIN mc_room_fission f ON f.id = c.fission_id
	`
}

type roomFissionScanner interface {
	Scan(dest ...any) error
}

func scanRoomFissionListRow(scanner roomFissionScanner) (dashboard.RoomFissionListItem, error) {
	var item dashboard.RoomFissionListItem
	var endTime, createdAt, updatedAt sql.NullTime
	var receiveEmployees []byte
	err := scanner.Scan(&item.ID, &item.OfficialAccountID, &item.ActiveName, &endTime, &item.TargetCount, &item.NewFriend, &item.DeleteInvalid, &receiveEmployees, &item.AutoPass, &item.Status, &item.TenantID, &item.CorpID, &item.CreateUserID, &item.CreateUserName, &item.ContactNum, &item.CompleteNum, &item.RoomNum, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RoomFissionListItem{}, err
	}
	item.ReceiveEmployeesRaw = string(receiveEmployees)
	item.EndTime = formatTime(endTime)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanRoomFissionInfo(scanner roomFissionScanner) (dashboard.RoomFissionInfo, error) {
	var item dashboard.RoomFissionInfo
	var endTime, createdAt, updatedAt sql.NullTime
	var receiveEmployees []byte
	err := scanner.Scan(&item.ID, &item.OfficialAccountID, &item.ActiveName, &endTime, &item.TargetCount, &item.NewFriend, &item.DeleteInvalid, &receiveEmployees, &item.AutoPass, &item.Status, &item.TenantID, &item.CorpID, &item.CreateUserID, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RoomFissionInfo{}, err
	}
	item.ReceiveEmployeesRaw = string(receiveEmployees)
	item.EndTime = formatTime(endTime)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanRoomFissionPoster(scanner roomFissionScanner) (dashboard.RoomFissionPoster, error) {
	var item dashboard.RoomFissionPoster
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.FissionID, &item.CoverPic, &item.AvatarShow, &item.NicknameShow, &item.NicknameColor, &item.QRCodeW, &item.QRCodeH, &item.QRCodeX, &item.QRCodeY, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RoomFissionPoster{}, err
	}
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanRoomFissionRoomRow(scanner roomFissionScanner) (dashboard.RoomFissionRoom, error) {
	var item dashboard.RoomFissionRoom
	var room []byte
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.FissionID, &item.RoomQRCode, &item.RoomWXQRCode, &room, &item.RoomMax, &item.ContactNum, &item.JoinNum, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RoomFissionRoom{}, err
	}
	item.RoomRaw = string(room)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanRoomFissionWelcome(scanner roomFissionScanner) (dashboard.RoomFissionWelcome, error) {
	var item dashboard.RoomFissionWelcome
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.FissionID, &item.Text, &item.LinkTitle, &item.LinkDesc, &item.LinkPic, &item.LinkWXURL, &item.TemplateID, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RoomFissionWelcome{}, err
	}
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanRoomFissionInvite(scanner roomFissionScanner) (dashboard.RoomFissionInvite, error) {
	var item dashboard.RoomFissionInvite
	var employees, chooseContact []byte
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.FissionID, &item.Type, &employees, &chooseContact, &item.Text, &item.LinkTitle, &item.LinkDesc, &item.LinkPic, &item.WXLinkPic, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RoomFissionInvite{}, err
	}
	item.EmployeesRaw = string(employees)
	item.ChooseContactRaw = string(chooseContact)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanRoomFissionContactRow(scanner roomFissionScanner) (dashboard.RoomFissionContactItem, error) {
	var item dashboard.RoomFissionContactItem
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.FissionID, &item.UnionID, &item.Nickname, &item.Avatar, &item.ParentUnionID, &item.Level, &item.ContactID, &item.Employee, &item.InviteCount, &item.Loss, &item.Status, &item.ReceiveStatus, &item.IsNew, &item.ExternalUserID, &item.RoomID, &item.JoinStatus, &item.WriteOff, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RoomFissionContactItem{}, err
	}
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func roomFissionTimeArg(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func roomFissionDefaultStatus(status int) int {
	if status <= 0 {
		return 1
	}
	return status
}

func roomFissionInviteType(value int) int {
	if value != 1 {
		return 2
	}
	return 1
}
