package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

type workMessageArchiveParticipant struct {
	ID   int
	Kind int
}

type workMessageArchiveRoom struct {
	ID      int
	OwnerID int
}

// archiveDBTX is implemented by both *sql.DB and *sql.Tx. Keeping the
// participant resolution and normalized message insert on this boundary lets
// the archive source identity and the legacy message row share one transaction.
type archiveDBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *MySQLStore) WorkMessageArchiveEnabledCorps(ctx context.Context) ([]dashboard.WorkMessageArchiveCorp, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM mc_corp
		WHERE chat_status = 1
		  AND COALESCE(wecom_credentials_ciphertext, '') <> ''
		  AND wx_corpid <> ''
		  AND deleted_at IS NULL
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	corpIDs := make([]int, 0)
	for rows.Next() {
		var corpID int
		if err := rows.Scan(&corpID); err != nil {
			rows.Close()
			return nil, err
		}
		corpIDs = append(corpIDs, corpID)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	corps := make([]dashboard.WorkMessageArchiveCorp, 0, len(corpIDs))
	for _, corpID := range corpIDs {
		item, found, err := s.loadCorpCredentialByID(ctx, s.db, corpID, false)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		credential, err := s.decodeCorpCredential(item)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(credential.ChatSecret) == "" || strings.TrimSpace(credential.ArchiveRSAPublicKey) == "" ||
			strings.TrimSpace(credential.ArchiveRSAPrivateKey) == "" {
			continue
		}
		corps = append(corps, dashboard.WorkMessageArchiveCorp{
			CorpID: item.ID, WXCorpID: item.WXCorpID, ChatSecret: credential.ChatSecret,
			RSAPublicKey: credential.ArchiveRSAPublicKey, RSAPrivateKey: credential.ArchiveRSAPrivateKey,
		})
	}
	return corps, nil
}

func (s *MySQLStore) WorkMessageArchiveCursor(ctx context.Context, corpID int) (int64, error) {
	if corpID <= 0 {
		return 0, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(last_id, 0)
		FROM mc_work_message_id
		WHERE corp_id = ?
		  AND type = ?
		  AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, corpID, dashboard.WorkMessageArchiveCursorType)
	var lastSeq int64
	err := row.Scan(&lastSeq)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return lastSeq, err
}

func (s *MySQLStore) UpsertWorkMessageArchive(ctx context.Context, corpID int, message dashboard.WorkMessageArchiveMessage) (dashboard.WorkMessageArchiveUpsertResult, error) {
	return s.upsertWorkMessageArchiveWithExecutor(ctx, s.db, corpID, message)
}

func (s *MySQLStore) upsertWorkMessageArchiveWithExecutor(ctx context.Context, executor archiveDBTX, corpID int, message dashboard.WorkMessageArchiveMessage) (dashboard.WorkMessageArchiveUpsertResult, error) {
	if corpID <= 0 || message.Seq <= 0 || strings.TrimSpace(message.MsgID) == "" {
		return dashboard.WorkMessageArchiveUpsertResult{Skipped: true}, nil
	}
	if executor == nil {
		return dashboard.WorkMessageArchiveUpsertResult{}, errors.New("archive message executor unavailable")
	}
	item, resolved, err := s.workMessageArchiveCreateWithExecutor(ctx, executor, corpID, message)
	if err != nil {
		return dashboard.WorkMessageArchiveUpsertResult{}, err
	}
	table, err := workMessageArchiveTable(message.Seq)
	if err != nil {
		return dashboard.WorkMessageArchiveUpsertResult{}, err
	}
	sendTime := sql.NullTime{Time: item.MsgDataTime, Valid: !item.MsgDataTime.IsZero()}
	result, err := executor.ExecContext(ctx, `
		INSERT INTO `+table+` (
			corp_id, msgid, seq, work_employee_id, to_user_type, to_user_id,
			sender_type, action, type, msg_type, content, content_text, room_id,
			status, msg_data_time, created_at, updated_at
		)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, NOW(), NOW()
		WHERE NOT EXISTS (
			SELECT 1
			FROM `+table+`
			WHERE corp_id = ?
			  AND msgid = ?
			  AND deleted_at IS NULL
			LIMIT 1
		)
	`, corpID, item.MsgID, item.Seq, item.WorkEmployeeID, item.ToUserType, item.ToUserID,
		item.SenderType, item.Action, item.Type, item.MsgType, item.ContentRaw, item.ContentText, item.RoomID,
		sendTime, corpID, item.MsgID)
	inserted, err := rowsAffectedBool(result, err)
	if err != nil {
		return dashboard.WorkMessageArchiveUpsertResult{}, err
	}
	return dashboard.WorkMessageArchiveUpsertResult{Inserted: inserted, Skipped: !inserted, Resolved: resolved}, nil
}

func (s *MySQLStore) UpdateWorkMessageArchiveCursor(ctx context.Context, corpID int, lastSeq int64) error {
	if corpID <= 0 || lastSeq <= 0 {
		return nil
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_work_message_id
		SET last_id = ?, deleted_at = NULL, updated_at = NOW()
		WHERE id = (
			SELECT id
			FROM (
				SELECT id
				FROM mc_work_message_id
				WHERE corp_id = ?
				  AND type = ?
				ORDER BY id DESC
				LIMIT 1
			) cursor_row
		)
	`, lastSeq, corpID, dashboard.WorkMessageArchiveCursorType)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO mc_work_message_id (corp_id, type, last_id, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
	`, corpID, dashboard.WorkMessageArchiveCursorType, lastSeq)
	return err
}

type workMessageArchiveCreate struct {
	MsgID          string
	Seq            int64
	WorkEmployeeID int
	ToUserType     int
	ToUserID       int
	SenderType     int
	Action         int
	Type           int
	MsgType        int
	ContentRaw     string
	ContentText    string
	RoomID         int
	MsgDataTime    time.Time
}

func (s *MySQLStore) workMessageArchiveCreate(ctx context.Context, corpID int, message dashboard.WorkMessageArchiveMessage) (workMessageArchiveCreate, bool, error) {
	return s.workMessageArchiveCreateWithExecutor(ctx, s.db, corpID, message)
}

func (s *MySQLStore) workMessageArchiveCreateWithExecutor(ctx context.Context, executor archiveDBTX, corpID int, message dashboard.WorkMessageArchiveMessage) (workMessageArchiveCreate, bool, error) {
	item := workMessageArchiveCreate{
		MsgID:       strings.TrimSpace(message.MsgID),
		Seq:         message.Seq,
		Action:      workMessageArchiveActionCode(message.Action),
		Type:        workMessageArchiveMsgTypeCode(message.MsgType),
		MsgType:     workMessageArchiveMsgTypeCode(message.MsgType),
		ContentRaw:  workMessageArchiveJSON(message.ContentRaw, message.RawJSON),
		ContentText: strings.TrimSpace(message.ContentText),
		MsgDataTime: message.MsgTime,
	}
	if item.MsgDataTime.IsZero() {
		item.MsgDataTime = time.Now()
	}
	sender, senderFound, err := lookupWorkMessageArchiveParticipantWithExecutor(ctx, executor, corpID, message.From)
	if err != nil {
		return item, false, err
	}
	toEmployees, toContacts, err := lookupWorkMessageArchiveParticipantsWithExecutor(ctx, executor, corpID, message.ToList)
	if err != nil {
		return item, false, err
	}
	if strings.TrimSpace(message.RoomID) != "" {
		room, found, err := lookupWorkMessageArchiveRoomWithExecutor(ctx, executor, corpID, message.RoomID)
		if err != nil {
			return item, false, err
		}
		if found {
			item.ToUserType = 2
			item.ToUserID = room.ID
			item.RoomID = room.ID
			if senderFound && sender.Kind == 0 {
				item.WorkEmployeeID = sender.ID
				item.SenderType = 0
			} else if len(toEmployees) > 0 {
				item.WorkEmployeeID = toEmployees[0].ID
				item.SenderType = 1
			} else if room.OwnerID > 0 {
				item.WorkEmployeeID = room.OwnerID
				item.SenderType = 1
			}
			return item, item.WorkEmployeeID > 0 && item.ToUserID > 0, nil
		}
	}
	if senderFound && sender.Kind == 0 {
		item.WorkEmployeeID = sender.ID
		item.SenderType = 0
		if len(toContacts) > 0 {
			item.ToUserType = 1
			item.ToUserID = toContacts[0].ID
		} else if len(toEmployees) > 0 {
			item.ToUserType = 0
			item.ToUserID = toEmployees[0].ID
		}
		return item, item.WorkEmployeeID > 0 && item.ToUserID > 0, nil
	}
	if senderFound && sender.Kind == 1 {
		item.SenderType = 1
		item.ToUserType = 1
		item.ToUserID = sender.ID
		if len(toEmployees) > 0 {
			item.WorkEmployeeID = toEmployees[0].ID
		}
		return item, item.WorkEmployeeID > 0 && item.ToUserID > 0, nil
	}
	if len(toEmployees) > 0 {
		item.WorkEmployeeID = toEmployees[0].ID
		item.SenderType = 1
		if len(toContacts) > 0 {
			item.ToUserType = 1
			item.ToUserID = toContacts[0].ID
		} else {
			item.ToUserType = 0
			item.ToUserID = toEmployees[0].ID
		}
	}
	return item, item.WorkEmployeeID > 0 && item.ToUserID > 0, nil
}

func (s *MySQLStore) lookupWorkMessageArchiveParticipants(ctx context.Context, corpID int, wxIDs []string) ([]workMessageArchiveParticipant, []workMessageArchiveParticipant, error) {
	return lookupWorkMessageArchiveParticipantsWithExecutor(ctx, s.db, corpID, wxIDs)
}

func lookupWorkMessageArchiveParticipantsWithExecutor(ctx context.Context, executor archiveDBTX, corpID int, wxIDs []string) ([]workMessageArchiveParticipant, []workMessageArchiveParticipant, error) {
	employees := []workMessageArchiveParticipant{}
	contacts := []workMessageArchiveParticipant{}
	for _, wxID := range wxIDs {
		participant, found, err := lookupWorkMessageArchiveParticipantWithExecutor(ctx, executor, corpID, wxID)
		if err != nil {
			return nil, nil, err
		}
		if !found {
			continue
		}
		if participant.Kind == 0 {
			employees = append(employees, participant)
		} else {
			contacts = append(contacts, participant)
		}
	}
	return employees, contacts, nil
}

func (s *MySQLStore) lookupWorkMessageArchiveParticipant(ctx context.Context, corpID int, wxID string) (workMessageArchiveParticipant, bool, error) {
	return lookupWorkMessageArchiveParticipantWithExecutor(ctx, s.db, corpID, wxID)
}

func lookupWorkMessageArchiveParticipantWithExecutor(ctx context.Context, executor archiveDBTX, corpID int, wxID string) (workMessageArchiveParticipant, bool, error) {
	wxID = strings.TrimSpace(wxID)
	if executor == nil || corpID <= 0 || wxID == "" {
		return workMessageArchiveParticipant{}, false, nil
	}
	row := executor.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_employee
		WHERE corp_id = ?
		  AND wx_user_id = ?
		  AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, corpID, wxID)
	var id int
	err := row.Scan(&id)
	if err == nil {
		return workMessageArchiveParticipant{ID: id, Kind: 0}, true, nil
	}
	if err != sql.ErrNoRows {
		return workMessageArchiveParticipant{}, false, err
	}
	row = executor.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_contact
		WHERE corp_id = ?
		  AND wx_external_userid = ?
		  AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, corpID, wxID)
	err = row.Scan(&id)
	if err == nil {
		return workMessageArchiveParticipant{ID: id, Kind: 1}, true, nil
	}
	if err == sql.ErrNoRows {
		return workMessageArchiveParticipant{}, false, nil
	}
	return workMessageArchiveParticipant{}, false, err
}

func (s *MySQLStore) lookupWorkMessageArchiveRoom(ctx context.Context, corpID int, wxChatID string) (workMessageArchiveRoom, bool, error) {
	return lookupWorkMessageArchiveRoomWithExecutor(ctx, s.db, corpID, wxChatID)
}

func lookupWorkMessageArchiveRoomWithExecutor(ctx context.Context, executor archiveDBTX, corpID int, wxChatID string) (workMessageArchiveRoom, bool, error) {
	wxChatID = strings.TrimSpace(wxChatID)
	if executor == nil || corpID <= 0 || wxChatID == "" {
		return workMessageArchiveRoom{}, false, nil
	}
	row := executor.QueryRowContext(ctx, `
		SELECT id, COALESCE(owner_id, 0)
		FROM mc_work_room
		WHERE corp_id = ?
		  AND wx_chat_id = ?
		  AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`, corpID, wxChatID)
	var room workMessageArchiveRoom
	err := row.Scan(&room.ID, &room.OwnerID)
	if err == sql.ErrNoRows {
		return workMessageArchiveRoom{}, false, nil
	}
	if err != nil {
		return workMessageArchiveRoom{}, false, err
	}
	return room, true, nil
}

func workMessageArchiveTable(seq int64) (string, error) {
	if seq <= 0 {
		return "", fmt.Errorf("invalid work message archive seq %d", seq)
	}
	index := int((seq-1)%dashboard.WorkMessageArchiveMessageTableCount) + 1
	return fmt.Sprintf("mc_work_message_%d", index), nil
}

func workMessageArchiveActionCode(action string) int {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "", "send":
		return 0
	case "recall", "revoke":
		return 1
	case "switch":
		return 2
	default:
		return 0
	}
}

func workMessageArchiveMsgTypeCode(msgType string) int {
	switch strings.ToLower(strings.TrimSpace(msgType)) {
	case "text":
		return 1
	case "image":
		return 2
	case "voice":
		return 3
	case "video":
		return 4
	case "file":
		return 5
	case "link":
		return 6
	case "location":
		return 7
	case "emotion":
		return 8
	case "mixed":
		return 9
	case "markdown":
		return 10
	case "meeting_voice_call":
		return 11
	case "voip_doc_share":
		return 12
	case "docmsg":
		return 13
	case "calendar":
		return 14
	case "vote":
		return 15
	case "collect":
		return 16
	case "redpacket":
		return 17
	case "card":
		return 18
	default:
		return 100
	}
}

func workMessageArchiveJSON(contentRaw string, fallback string) string {
	contentRaw = strings.TrimSpace(contentRaw)
	if contentRaw != "" && json.Valid([]byte(contentRaw)) {
		return contentRaw
	}
	fallback = strings.TrimSpace(fallback)
	if fallback != "" && json.Valid([]byte(fallback)) {
		return fallback
	}
	payload := map[string]string{"content": workMessageArchiveFirstNonBlank(contentRaw, fallback)}
	raw, err := json.Marshal(payload)
	if err != nil {
		return `{"content":""}`
	}
	return string(raw)
}

func workMessageArchiveFirstNonBlank(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
