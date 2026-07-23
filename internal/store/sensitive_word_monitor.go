package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) ActiveSensitiveWords(ctx context.Context) ([]dashboard.SensitiveWordCronWord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, corp_id, name
		FROM mc_sensitive_word
		WHERE status = 1
		  AND deleted_at IS NULL
		  AND name <> ''
		ORDER BY corp_id ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	words := make([]dashboard.SensitiveWordCronWord, 0)
	for rows.Next() {
		var word dashboard.SensitiveWordCronWord
		if err := rows.Scan(&word.ID, &word.CorpID, &word.Name); err != nil {
			return nil, err
		}
		if word.ID <= 0 || word.CorpID <= 0 || strings.TrimSpace(word.Name) == "" {
			continue
		}
		words = append(words, word)
	}
	return words, rows.Err()
}

func (s *MySQLStore) SensitiveWordMessageCursor(ctx context.Context, corpID int, tableIndex int) (int, error) {
	if corpID <= 0 {
		return 0, nil
	}
	cursorType, err := sensitiveWordCursorType(tableIndex)
	if err != nil {
		return 0, err
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(last_id, 0)
		FROM mc_work_message_id
		WHERE corp_id = ?
		  AND type = ?
		  AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, corpID, cursorType)
	var lastID int
	err = row.Scan(&lastID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return lastID, err
}

func (s *MySQLStore) PendingSensitiveWordMessages(ctx context.Context, corpID int, tableIndex int, afterID int, limit int) ([]dashboard.SensitiveWordArchivedMessage, error) {
	if corpID <= 0 {
		return []dashboard.SensitiveWordArchivedMessage{}, nil
	}
	table, err := sensitiveWordMessageTable(tableIndex)
	if err != nil {
		return nil, err
	}
	if afterID < 0 {
		afterID = 0
	}
	if limit <= 0 {
		limit = dashboard.SensitiveWordMonitorDefaultMessagesPerTick
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			wm.id,
			COALESCE(wm.corp_id, 0),
			COALESCE(wm.work_employee_id, 0),
			COALESCE(wm.to_user_type, 0),
			COALESCE(wm.to_user_id, 0),
			COALESCE(wm.sender_type, 0),
			COALESCE(wm.msg_type, wm.type, 100),
			COALESCE(CAST(wm.content AS CHAR), ''),
			COALESCE(wm.content_text, ''),
			COALESCE(wm.room_id, 0),
			wm.msg_data_time,
			COALESCE(sender.name, ''),
			COALESCE(target_employee.name, target_contact.name, target_room.name, ''),
			COALESCE(room_by_id.name, target_room.name, '')
		FROM `+table+` wm
		LEFT JOIN mc_work_employee sender
			ON sender.id = wm.work_employee_id
			AND sender.corp_id = wm.corp_id
			AND sender.deleted_at IS NULL
		LEFT JOIN mc_work_employee target_employee
			ON wm.to_user_type = 0
			AND target_employee.id = wm.to_user_id
			AND target_employee.corp_id = wm.corp_id
			AND target_employee.deleted_at IS NULL
		LEFT JOIN mc_work_contact target_contact
			ON wm.to_user_type = 1
			AND target_contact.id = wm.to_user_id
			AND target_contact.corp_id = wm.corp_id
			AND target_contact.deleted_at IS NULL
		LEFT JOIN mc_work_room target_room
			ON wm.to_user_type = 2
			AND target_room.id = wm.to_user_id
			AND target_room.corp_id = wm.corp_id
			AND target_room.deleted_at IS NULL
		LEFT JOIN mc_work_room room_by_id
			ON room_by_id.id = wm.room_id
			AND room_by_id.corp_id = wm.corp_id
			AND room_by_id.deleted_at IS NULL
		WHERE wm.corp_id = ?
		  AND wm.id > ?
		  AND wm.deleted_at IS NULL
		ORDER BY wm.id ASC
		LIMIT ?
	`, corpID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]dashboard.SensitiveWordArchivedMessage, 0)
	for rows.Next() {
		var message dashboard.SensitiveWordArchivedMessage
		var msgDataTime sql.NullTime
		message.TableIndex = tableIndex
		if err := rows.Scan(&message.ID, &message.CorpID, &message.WorkEmployeeID, &message.ToUserType, &message.ToUserID, &message.SenderType, &message.MsgType, &message.ContentRaw, &message.ContentText, &message.RoomID, &msgDataTime, &message.SenderName, &message.TargetName, &message.RoomName); err != nil {
			return nil, err
		}
		if msgDataTime.Valid {
			message.MsgDataTime = msgDataTime.Time
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (s *MySQLStore) InsertSensitiveWordMonitor(ctx context.Context, item dashboard.SensitiveWordMonitorCreate) (bool, error) {
	if item.CorpID <= 0 || item.SensitiveWordID <= 0 || strings.TrimSpace(item.SensitiveWordName) == "" || strings.TrimSpace(item.ContentRaw) == "" {
		return false, nil
	}
	if item.Source != 1 && item.Source != 2 {
		item.Source = 1
	}
	if item.MsgType <= 0 {
		item.MsgType = 100
	}
	sendTime := sql.NullTime{}
	if !item.SendTime.IsZero() {
		sendTime = sql.NullTime{Time: item.SendTime, Valid: true}
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_sensitive_words_monitor (
			corp_id, sensitive_word_id, sensitive_word_name, source,
			trigger_user_id, trigger_name, work_room_id, trigger_scenario,
			sender, msg_type, send_time, content, conversation_json,
			created_at, updated_at
		)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW()
		WHERE NOT EXISTS (
			SELECT 1
			FROM mc_sensitive_words_monitor
			WHERE corp_id = ?
			  AND sensitive_word_id = ?
			  AND source = ?
			  AND trigger_user_id = ?
			  AND work_room_id = ?
			  AND send_time <=> ?
			  AND conversation_json = ?
			  AND deleted_at IS NULL
			LIMIT 1
		)
	`, item.CorpID, item.SensitiveWordID, strings.TrimSpace(item.SensitiveWordName), item.Source,
		item.TriggerUserID, strings.TrimSpace(item.TriggerName), item.WorkRoomID, strings.TrimSpace(item.TriggerScenario),
		strings.TrimSpace(item.Sender), item.MsgType, sendTime, item.ContentRaw, item.ConversationJSON,
		item.CorpID, item.SensitiveWordID, item.Source, item.TriggerUserID, item.WorkRoomID, sendTime, item.ConversationJSON)
	return rowsAffectedBool(result, err)
}

func (s *MySQLStore) UpdateSensitiveWordMessageCursor(ctx context.Context, corpID int, tableIndex int, lastID int) error {
	if corpID <= 0 || lastID <= 0 {
		return nil
	}
	cursorType, err := sensitiveWordCursorType(tableIndex)
	if err != nil {
		return err
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
	`, lastID, corpID, cursorType)
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
	`, corpID, cursorType, lastID)
	return err
}

func sensitiveWordCursorType(tableIndex int) (int, error) {
	if tableIndex < 1 || tableIndex > dashboard.SensitiveWordMonitorMessageTableCount {
		return 0, fmt.Errorf("invalid work message table index %d", tableIndex)
	}
	return dashboard.SensitiveWordMonitorCursorTypeBase + tableIndex, nil
}

func sensitiveWordMessageTable(tableIndex int) (string, error) {
	if tableIndex < 1 || tableIndex > dashboard.SensitiveWordMonitorMessageTableCount {
		return "", fmt.Errorf("invalid work message table index %d", tableIndex)
	}
	return fmt.Sprintf("mc_work_message_%d", tableIndex), nil
}
