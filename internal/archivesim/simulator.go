package archivesim

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/store"
)

const messagePrefix = "MOCHAT-SIM:"

var batchKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`)

type Result struct {
	CorpID       int    `json:"corpId"`
	Batch        string `json:"batch"`
	Status       string `json:"status"`
	MessageCount int    `json:"messageCount"`
	Idempotent   bool   `json:"idempotent"`
	CursorBefore int64  `json:"cursorBefore,omitempty"`
	CursorAfter  int64  `json:"cursorAfter,omitempty"`
}

type Simulator struct {
	db      *sql.DB
	archive *store.MySQLStore
	now     func() time.Time
}

func New(db *sql.DB) *Simulator {
	return &Simulator{db: db, archive: store.NewMySQLStore(db), now: time.Now}
}

func (simulator *Simulator) Apply(ctx context.Context, corpID int, batch string) (Result, error) {
	batch, err := validate(corpID, batch)
	if err != nil {
		return Result{}, err
	}
	if simulator == nil || simulator.db == nil || simulator.archive == nil {
		return Result{}, errors.New("archive simulator unavailable")
	}
	cursorBefore, err := simulator.archive.WorkMessageArchiveCursor(ctx, corpID)
	if err != nil {
		return Result{}, fmt.Errorf("read archive cursor before simulation: %w", err)
	}
	current, found, err := simulator.status(ctx, corpID, batch)
	if err != nil {
		return Result{}, err
	}
	if found && current.Status == "complete" {
		current.Idempotent = true
		return current, nil
	}
	if found {
		if _, err := simulator.Cleanup(ctx, corpID, batch); err != nil {
			return Result{}, fmt.Errorf("clean incomplete batch: %w", err)
		}
	}

	tx, err := simulator.db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	var corpExists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_corp WHERE id=? AND deleted_at IS NULL`, corpID).Scan(&corpExists); err != nil {
		return Result{}, err
	}
	if corpExists != 1 {
		return Result{}, fmt.Errorf("corp %d not found", corpID)
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_archive_simulation_batches (corp_id,batch_key,status,message_count,created_at,updated_at) VALUES (?,?,'applying',0,NOW(),NOW())`, corpID, batch)
	if err != nil {
		return Result{}, err
	}
	batchID, err := result.LastInsertId()
	if err != nil {
		return Result{}, err
	}
	employees, err := simulator.ensureEmployees(ctx, tx, batchID, corpID, batch)
	if err != nil {
		return Result{}, err
	}
	contactWXID, err := simulator.createContact(ctx, tx, batchID, corpID, batch)
	if err != nil {
		return Result{}, err
	}
	roomWXID, err := simulator.createRoom(ctx, tx, batchID, corpID, batch, employees[0].ID)
	if err != nil {
		return Result{}, err
	}
	messages := buildMessages(batch, simulator.now(), employees[0].WXID, employees[1].WXID, contactWXID, roomWXID)
	for _, message := range messages {
		tableIndex := int((message.Seq-1)%dashboard.WorkMessageArchiveMessageTableCount) + 1
		if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_archive_simulation_messages (batch_id,corp_id,msgid,table_index,created_at) VALUES (?,?,?,?,NOW())`, batchID, corpID, message.MsgID, tableIndex); err != nil {
			return Result{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Result{}, err
	}

	inserted := 0
	for _, message := range messages {
		upsert, err := simulator.archive.UpsertWorkMessageArchive(ctx, corpID, message)
		if err != nil {
			_, _ = simulator.db.ExecContext(ctx, `UPDATE mochat_go_archive_simulation_batches SET status='failed',message_count=?,updated_at=NOW() WHERE id=?`, inserted, batchID)
			return Result{}, err
		}
		if !upsert.Resolved {
			_, _ = simulator.db.ExecContext(ctx, `UPDATE mochat_go_archive_simulation_batches SET status='failed',message_count=?,updated_at=NOW() WHERE id=?`, inserted, batchID)
			return Result{}, fmt.Errorf("simulated message %s participants were not resolved", message.MsgID)
		}
		if upsert.Inserted {
			inserted++
		}
	}
	if _, err := simulator.db.ExecContext(ctx, `UPDATE mochat_go_archive_simulation_batches SET status='complete',message_count=?,updated_at=NOW() WHERE id=?`, inserted, batchID); err != nil {
		return Result{}, err
	}
	cursorAfter, err := simulator.archive.WorkMessageArchiveCursor(ctx, corpID)
	if err != nil {
		return Result{}, fmt.Errorf("read archive cursor after simulation: %w", err)
	}
	if cursorAfter != cursorBefore {
		return Result{}, fmt.Errorf("real archive cursor changed during simulation: before=%d after=%d", cursorBefore, cursorAfter)
	}
	return Result{CorpID: corpID, Batch: batch, Status: "complete", MessageCount: inserted, CursorBefore: cursorBefore, CursorAfter: cursorAfter}, nil
}

func (simulator *Simulator) Status(ctx context.Context, corpID int, batch string) (Result, error) {
	batch, err := validate(corpID, batch)
	if err != nil {
		return Result{}, err
	}
	result, found, err := simulator.status(ctx, corpID, batch)
	if err != nil {
		return Result{}, err
	}
	if !found {
		return Result{CorpID: corpID, Batch: batch, Status: "absent"}, nil
	}
	return result, nil
}

func (simulator *Simulator) status(ctx context.Context, corpID int, batch string) (Result, bool, error) {
	var result Result
	result.CorpID, result.Batch = corpID, batch
	err := simulator.db.QueryRowContext(ctx, `SELECT status,message_count FROM mochat_go_archive_simulation_batches WHERE corp_id=? AND batch_key=?`, corpID, batch).Scan(&result.Status, &result.MessageCount)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, false, nil
	}
	return result, err == nil, err
}

func (simulator *Simulator) Cleanup(ctx context.Context, corpID int, batch string) (Result, error) {
	batch, err := validate(corpID, batch)
	if err != nil {
		return Result{}, err
	}
	var batchID int64
	err = simulator.db.QueryRowContext(ctx, `SELECT id FROM mochat_go_archive_simulation_batches WHERE corp_id=? AND batch_key=?`, corpID, batch).Scan(&batchID)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{CorpID: corpID, Batch: batch, Status: "absent"}, nil
	}
	if err != nil {
		return Result{}, err
	}

	rows, err := simulator.db.QueryContext(ctx, `SELECT msgid,table_index FROM mochat_go_archive_simulation_messages WHERE batch_id=? ORDER BY id`, batchID)
	if err != nil {
		return Result{}, err
	}
	type registeredMessage struct {
		msgID      string
		tableIndex int
	}
	messages := []registeredMessage{}
	for rows.Next() {
		var item registeredMessage
		if err := rows.Scan(&item.msgID, &item.tableIndex); err != nil {
			rows.Close()
			return Result{}, err
		}
		messages = append(messages, item)
	}
	if err := rows.Close(); err != nil {
		return Result{}, err
	}

	tx, err := simulator.db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	for _, message := range messages {
		if message.tableIndex < 1 || message.tableIndex > dashboard.WorkMessageArchiveMessageTableCount || !strings.HasPrefix(message.msgID, messagePrefix+batch+":") {
			return Result{}, errors.New("simulation registry contains an unsafe message target")
		}
		query := fmt.Sprintf("DELETE FROM mc_work_message_%d WHERE corp_id=? AND msgid=?", message.tableIndex)
		if _, err := tx.ExecContext(ctx, query, corpID, message.msgID); err != nil {
			return Result{}, err
		}
	}
	entities, err := registeredEntities(ctx, tx, batchID)
	if err != nil {
		return Result{}, err
	}
	for _, entity := range entities {
		var query string
		switch entity.kind {
		case "room":
			query = `DELETE FROM mc_work_room WHERE id=? AND corp_id=? AND wx_chat_id=?`
		case "contact":
			query = `DELETE FROM mc_work_contact WHERE id=? AND corp_id=? AND wx_external_userid=?`
		case "employee":
			query = `DELETE FROM mc_work_employee WHERE id=? AND corp_id=? AND wx_user_id=? AND log_user_id=0`
		default:
			return Result{}, errors.New("simulation registry contains an unsafe entity target")
		}
		if _, err := tx.ExecContext(ctx, query, entity.id, corpID, entity.externalKey); err != nil {
			return Result{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM mochat_go_archive_simulation_batches WHERE id=? AND corp_id=? AND batch_key=?`, batchID, corpID, batch); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(); err != nil {
		return Result{}, err
	}
	return Result{CorpID: corpID, Batch: batch, Status: "cleaned", MessageCount: len(messages)}, nil
}

type employee struct {
	ID   int
	WXID string
}

func (simulator *Simulator) ensureEmployees(ctx context.Context, tx *sql.Tx, batchID int64, corpID int, batch string) ([]employee, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,wx_user_id FROM mc_work_employee WHERE corp_id=? AND deleted_at IS NULL ORDER BY id LIMIT 2`, corpID)
	if err != nil {
		return nil, err
	}
	employees := []employee{}
	for rows.Next() {
		var item employee
		if err := rows.Scan(&item.ID, &item.WXID); err != nil {
			rows.Close()
			return nil, err
		}
		if strings.TrimSpace(item.WXID) != "" {
			employees = append(employees, item)
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for len(employees) < 2 {
		externalKey := fmt.Sprintf("MOCHAT_SIM_EMP_%s_%d", batch, len(employees)+1)
		result, err := tx.ExecContext(ctx, `INSERT INTO mc_work_employee (wx_user_id,corp_id,name,status,log_user_id,created_at,updated_at) VALUES (?,?,?,1,0,NOW(),NOW())`, externalKey, corpID, fmt.Sprintf("模拟员工%d", len(employees)+1))
		if err != nil {
			return nil, err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return nil, err
		}
		if err := registerEntity(ctx, tx, batchID, corpID, "employee", int(id), externalKey); err != nil {
			return nil, err
		}
		employees = append(employees, employee{ID: int(id), WXID: externalKey})
	}
	return employees, nil
}

func (simulator *Simulator) createContact(ctx context.Context, tx *sql.Tx, batchID int64, corpID int, batch string) (string, error) {
	externalKey := "MOCHAT_SIM_CONTACT_" + batch
	result, err := tx.ExecContext(ctx, `INSERT INTO mc_work_contact (corp_id,wx_external_userid,name,nick_name,created_at,updated_at) VALUES (?,?,?,'',NOW(),NOW())`, corpID, externalKey, "模拟客户")
	if err != nil {
		return "", err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	return externalKey, registerEntity(ctx, tx, batchID, corpID, "contact", int(id), externalKey)
}

func (simulator *Simulator) createRoom(ctx context.Context, tx *sql.Tx, batchID int64, corpID int, batch string, ownerID int) (string, error) {
	externalKey := "MOCHAT_SIM_ROOM_" + batch
	result, err := tx.ExecContext(ctx, `INSERT INTO mc_work_room (corp_id,wx_chat_id,name,owner_id,notice,status,created_at,updated_at) VALUES (?,?,?,?,?,0,NOW(),NOW())`, corpID, externalKey, "模拟客户群", ownerID, "仅用于本地验收")
	if err != nil {
		return "", err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	return externalKey, registerEntity(ctx, tx, batchID, corpID, "room", int(id), externalKey)
}

func registerEntity(ctx context.Context, tx *sql.Tx, batchID int64, corpID int, kind string, id int, externalKey string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_archive_simulation_entities (batch_id,corp_id,entity_type,entity_id,external_key,created_at) VALUES (?,?,?,?,?,NOW())`, batchID, corpID, kind, id, externalKey)
	return err
}

type registeredEntity struct {
	kind        string
	id          int
	externalKey string
}

func registeredEntities(ctx context.Context, tx *sql.Tx, batchID int64) ([]registeredEntity, error) {
	rows, err := tx.QueryContext(ctx, `SELECT entity_type,entity_id,external_key FROM mochat_go_archive_simulation_entities WHERE batch_id=? ORDER BY FIELD(entity_type,'room','contact','employee'),id`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []registeredEntity{}
	for rows.Next() {
		var item registeredEntity
		if err := rows.Scan(&item.kind, &item.id, &item.externalKey); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func validate(corpID int, batch string) (string, error) {
	batch = strings.TrimSpace(batch)
	if corpID <= 0 || !batchKeyPattern.MatchString(batch) {
		return "", errors.New("corp-id must be positive and batch must contain only letters, digits, underscore or dash")
	}
	return batch, nil
}

func buildMessages(batch string, now time.Time, employeeA, employeeB, contact, room string) []dashboard.WorkMessageArchiveMessage {
	types := []struct{ kind, text string }{
		{"text", "模拟验收关键词：客户咨询产品方案"},
		{"text", "模拟客户回复：请发送报价"},
		{"image", "模拟图片：产品截图"},
		{"file", "模拟文件：产品报价单.pdf"},
		{"voice", "模拟语音：三十秒需求说明"},
		{"video", "模拟视频：产品演示"},
		{"location", "模拟位置：上海市浦东新区"},
		{"card", "模拟名片：客户联系人"},
		{"link", "模拟链接：https://example.invalid/mochat-simulation"},
		{"text", "模拟内部会话：员工协作跟进"},
		{"text", "模拟群聊：欢迎加入验收群"},
		{"emotion", "模拟表情消息"},
	}
	hash := sha256.Sum256([]byte(batch))
	base := int64(4_000_000_000_000_000 + (binary.BigEndian.Uint64(hash[:8]) & ((1 << 50) - 1)))
	start := now.UTC().Add(-time.Duration(len(types)) * time.Minute)
	messages := make([]dashboard.WorkMessageArchiveMessage, 0, len(types))
	for index, item := range types {
		from, to := employeeA, []string{contact}
		roomID := ""
		switch index {
		case 1:
			from, to = contact, []string{employeeA}
		case 9:
			from, to = employeeA, []string{employeeB}
		case 10, 11:
			from, to, roomID = employeeA, []string{employeeB, contact}, room
		}
		content, _ := json.Marshal(map[string]any{"msgtype": item.kind, "content": item.text, "simulation": true})
		messages = append(messages, dashboard.WorkMessageArchiveMessage{
			Seq: base + int64(index+1), MsgID: fmt.Sprintf("%s%s:%03d", messagePrefix, batch, index+1),
			Action: "send", From: from, ToList: to, RoomID: roomID, MsgType: item.kind,
			MsgTime: start.Add(time.Duration(index) * time.Minute), ContentRaw: string(content), ContentText: item.text,
		})
	}
	return messages
}

func MessageTypes() []string {
	messages := buildMessages("coverage", time.Unix(1, 0), "employee-a", "employee-b", "contact", "room")
	set := map[string]bool{}
	for _, message := range messages {
		set[message.MsgType] = true
	}
	result := make([]string, 0, len(set))
	for kind := range set {
		result = append(result, kind)
	}
	sort.Strings(result)
	return result
}
