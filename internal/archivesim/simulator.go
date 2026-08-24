package archivesim

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/modules/providers"
	archiveprovider "jiyi/mochat-go/internal/modules/providers/archive"
	"jiyi/mochat-go/internal/store"
)

const messagePrefix = "MOCHAT-SIM:"

var batchKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`)

type Result struct {
	CorpID       int    `json:"corpId"`
	Batch        string `json:"batch"`
	Source       string `json:"source"`
	SourceID     string `json:"sourceId"`
	Namespace    string `json:"namespace"`
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
	if _, err := archiveprovider.NewSimulationSource(batch); err != nil {
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
	var tenantID int64
	if err := simulator.db.QueryRowContext(ctx, `SELECT tenant_id FROM mc_corp WHERE id=? AND deleted_at IS NULL LIMIT 1`, corpID).Scan(&tenantID); err != nil {
		return Result{}, err
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

	source := newSimulationBatchSource(batch, messages)
	run, syncErr := archiveprovider.NewSyncService(simulator.archive).Sync(ctx, source, archiveprovider.SyncRequest{
		Scope: archiveprovider.Scope{TenantID: tenantID, CorpID: int64(corpID)}, IdempotencyKey: "simulation:" + batch,
		Limit: len(messages),
	})
	if syncErr != nil {
		_, _ = simulator.db.ExecContext(ctx, `UPDATE mochat_go_archive_simulation_batches SET status='failed',message_count=?,updated_at=NOW() WHERE id=?`, run.Counts.Processed, batchID)
		return Result{}, syncErr
	}
	inserted := run.Counts.Processed
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
	finalResult := simulationResult(corpID, batch, "complete", inserted)
	finalResult.CursorBefore = cursorBefore
	finalResult.CursorAfter = cursorAfter
	return finalResult, nil
}

type simulationBatchSource struct {
	runID     string
	namespace string
	messages  []archiveprovider.Message
}

func newSimulationBatchSource(batch string, messages []dashboard.WorkMessageArchiveMessage) *simulationBatchSource {
	providerSource, _ := archiveprovider.NewSimulationSource(batch)
	converted := make([]archiveprovider.Message, 0, len(messages))
	for _, message := range messages {
		converted = append(converted, archiveprovider.Message{
			Source: providers.SourceSimulated, SourceID: providerSource.SourceID(), Namespace: providerSource.Namespace(),
			MsgID: message.MsgID, Seq: message.Seq, Action: message.Action, From: message.From, ToList: message.ToList,
			RoomID: message.RoomID, MsgType: message.MsgType, MsgTime: message.MsgTime, ContentRaw: message.ContentRaw,
			ContentText: message.ContentText, RawJSON: message.RawJSON,
		})
	}
	return &simulationBatchSource{runID: providerSource.SourceID(), namespace: providerSource.Namespace(), messages: converted}
}

func (s *simulationBatchSource) Kind() providers.Source { return providers.SourceSimulated }
func (s *simulationBatchSource) SourceID() string       { return s.runID }
func (s *simulationBatchSource) Namespace() string      { return s.namespace }
func (s *simulationBatchSource) Status() providers.Status {
	return providers.Status{Kind: "wecom_archive", Source: providers.SourceSimulated, State: providers.StateLimited, Code: "archive.simulation_ready"}
}

func (s *simulationBatchSource) Fetch(_ context.Context, scope archiveprovider.Scope, cursor archiveprovider.Cursor, limit int) (archiveprovider.Page, error) {
	if scope.TenantID <= 0 || scope.CorpID <= 0 {
		return archiveprovider.Page{}, archiveprovider.ErrInvalidScope
	}
	if limit <= 0 {
		limit = len(s.messages)
	}
	page := archiveprovider.Page{Messages: make([]archiveprovider.Message, 0, limit)}
	for _, message := range s.messages {
		if message.Seq <= cursor.Sequence {
			continue
		}
		page.Messages = append(page.Messages, message)
		if len(page.Messages) >= limit {
			break
		}
	}
	if len(page.Messages) > 0 {
		page.NextCursor = archiveprovider.Cursor{Sequence: page.Messages[len(page.Messages)-1].Seq}
		page.HasMore = page.NextCursor.Sequence < s.messages[len(s.messages)-1].Seq
	}
	return page, nil
}

func (simulator *Simulator) Status(ctx context.Context, corpID int, batch string) (Result, error) {
	batch, err := validate(corpID, batch)
	if err != nil {
		return Result{}, err
	}
	if _, err := archiveprovider.NewSimulationSource(batch); err != nil {
		return Result{}, err
	}
	result, found, err := simulator.status(ctx, corpID, batch)
	if err != nil {
		return Result{}, err
	}
	if !found {
		return simulationResult(corpID, batch, "absent", 0), nil
	}
	return result, nil
}

func (simulator *Simulator) status(ctx context.Context, corpID int, batch string) (Result, bool, error) {
	var result Result
	result = simulationResult(corpID, batch, "", 0)
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
	if _, err := archiveprovider.NewSimulationSource(batch); err != nil {
		return Result{}, err
	}
	tx, err := simulator.db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	var batchID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM mochat_go_archive_simulation_batches WHERE corp_id=? AND batch_key=? LIMIT 1 FOR UPDATE`, corpID, batch).Scan(&batchID)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return simulationResult(corpID, batch, "absent", 0), nil
	}
	if err != nil {
		return Result{}, err
	}
	var tenantID int64
	if err := tx.QueryRowContext(ctx, `SELECT tenant_id FROM mc_corp WHERE id=? AND deleted_at IS NULL LIMIT 1 FOR UPDATE`, corpID).Scan(&tenantID); err != nil {
		return Result{}, err
	}

	rows, err := tx.QueryContext(ctx, `SELECT msgid,table_index FROM mochat_go_archive_simulation_messages WHERE batch_id=? ORDER BY id FOR UPDATE`, batchID)
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

	sourceID := "simulation:" + batch
	namespace := messagePrefix + batch
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM mochat_go_archive_message_sources
		WHERE tenant_id=? AND corp_id=? AND source_kind='simulated' AND source_id=? AND namespace=?
	`, tenantID, corpID, sourceID, namespace); err != nil {
		return Result{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM mochat_go_archive_sync_audits
		WHERE tenant_id=? AND corp_id=? AND source_kind='simulated' AND source_id=? AND namespace=?
	`, tenantID, corpID, sourceID, namespace); err != nil {
		return Result{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM mochat_go_archive_sync_runs
		WHERE tenant_id=? AND corp_id=? AND source_kind='simulated' AND source_id=? AND namespace=?
	`, tenantID, corpID, sourceID, namespace); err != nil {
		return Result{}, err
	}
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
	return simulationResult(corpID, batch, "cleaned", len(messages)), nil
}

func simulationResult(corpID int, batch, status string, messageCount int) Result {
	return Result{
		CorpID: corpID, Batch: batch, Source: "simulated",
		SourceID: "simulation:" + batch, Namespace: messagePrefix + batch,
		Status: status, MessageCount: messageCount,
	}
}

type employee struct {
	ID   int
	WXID string
}

type simulationLabels struct {
	EmployeeA string
	EmployeeB string
	Contact   string
	Room      string
}

func simulationEntityLabels(batch string) simulationLabels {
	return simulationLabels{
		EmployeeA: "AI验收员工A-" + batch,
		EmployeeB: "AI验收员工B-" + batch,
		Contact:   "AI验收客户-" + batch,
		Room:      "AI验收客户群-" + batch,
	}
}

func (simulator *Simulator) ensureEmployees(ctx context.Context, tx *sql.Tx, batchID int64, corpID int, batch string) ([]employee, error) {
	labels := simulationEntityLabels(batch)
	names := []string{labels.EmployeeA, labels.EmployeeB}
	employees := make([]employee, 0, len(names))
	for index, name := range names {
		externalKey := fmt.Sprintf("MOCHAT_SIM_EMP_%s_%d", batch, index+1)
		result, err := tx.ExecContext(ctx, `INSERT INTO mc_work_employee (wx_user_id,corp_id,name,status,log_user_id,created_at,updated_at) VALUES (?,?,?,1,0,NOW(),NOW())`, externalKey, corpID, name)
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
	result, err := tx.ExecContext(ctx, `INSERT INTO mc_work_contact (corp_id,wx_external_userid,name,nick_name,created_at,updated_at) VALUES (?,?,?,'',NOW(),NOW())`, corpID, externalKey, simulationEntityLabels(batch).Contact)
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
	result, err := tx.ExecContext(ctx, `INSERT INTO mc_work_room (corp_id,wx_chat_id,name,owner_id,notice,status,created_at,updated_at) VALUES (?,?,?,?,?,0,NOW(),NOW())`, corpID, externalKey, simulationEntityLabels(batch).Room, ownerID, "仅用于本地验收")
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
	blueprint := archiveprovider.BuildSimulationMessages(batch, now)
	messages := make([]dashboard.WorkMessageArchiveMessage, 0, len(blueprint))
	for _, item := range blueprint {
		from := simulationParticipant(item.From, employeeA, employeeB, contact)
		to := make([]string, 0, len(item.ToList))
		for _, participant := range item.ToList {
			to = append(to, simulationParticipant(participant, employeeA, employeeB, contact))
		}
		roomID := item.RoomID
		if roomID == "room" {
			roomID = room
		}
		messages = append(messages, dashboard.WorkMessageArchiveMessage{
			Seq: item.Seq, MsgID: item.MsgID, Action: item.Action, From: from, ToList: to,
			RoomID: roomID, MsgType: item.MsgType, MsgTime: item.MsgTime,
			ContentRaw: item.ContentRaw, ContentText: item.ContentText, RawJSON: item.RawJSON,
		})
	}
	return messages
}

func simulationParticipant(value, employeeA, employeeB, contact string) string {
	switch value {
	case "employee-a":
		return employeeA
	case "employee-b":
		return employeeB
	case "contact":
		return contact
	default:
		return value
	}
}

func MessageTypes() []string {
	return archiveprovider.MessageTypes()
}
