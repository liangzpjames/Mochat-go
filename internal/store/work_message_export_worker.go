package store

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

const workMessageExportLease = 10 * time.Minute

type workMessageExportWorkerTask struct {
	ID                 int64
	TenantID           int
	CorpID             int
	UserID             int
	ExportType         string
	ObjectIDs          []int
	ConversationScopes []string
	EmployeeIDs        []int
	StartAt            time.Time
	EndAt              time.Time
	FileMode           string
	Format             string
	LeaseOwner         string
}

type workMessageExportRow struct {
	ID            int
	TableIndex    int
	Seq           int64
	MsgID         string
	EmployeeID    int
	EmployeeName  string
	SenderName    string
	TargetName    string
	ToUserType    int
	ToUserID      int
	MessageType   int
	Content       string
	SentAt        string
	IsCurrentUser int
}

func (s *MySQLStore) RunWorkMessageExportWorker(ctx context.Context, root, owner string) (bool, error) {
	if strings.TrimSpace(owner) == "" {
		owner = fmt.Sprintf("mochat-export-%d", os.Getpid())
	}
	task, claimed, err := s.claimWorkMessageExportTask(ctx, owner)
	if err != nil || !claimed {
		return claimed, err
	}
	if err := s.processWorkMessageExportTask(ctx, root, task); err != nil {
		_, _ = s.db.ExecContext(ctx, `UPDATE mochat_go_work_message_export_tasks
			SET status='failed', error_code='EXPORT_FAILED', error_message=?, finished_at=NOW(), lease_owner='', lease_expires_at=NULL
			WHERE id=? AND lease_owner=?`, truncateExportError(err.Error()), task.ID, task.LeaseOwner)
		return true, err
	}
	return true, nil
}

func (s *MySQLStore) claimWorkMessageExportTask(ctx context.Context, owner string) (workMessageExportWorkerTask, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return workMessageExportWorkerTask{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM mochat_go_work_message_export_tasks
		WHERE (status='pending' OR (status='running' AND lease_expires_at < NOW()))
		AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY id ASC LIMIT 1 FOR UPDATE`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return workMessageExportWorkerTask{}, false, nil
	}
	if err != nil {
		return workMessageExportWorkerTask{}, false, err
	}
	leaseExpires := time.Now().Add(workMessageExportLease)
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_work_message_export_tasks
		SET status='running', lease_owner=?, lease_expires_at=?, started_at=COALESCE(started_at,NOW()), error_code='', error_message=''
		WHERE id=?`, owner, leaseExpires, id); err != nil {
		return workMessageExportWorkerTask{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return workMessageExportWorkerTask{}, false, err
	}
	return s.loadWorkMessageExportWorkerTask(ctx, id, owner)
}

func (s *MySQLStore) loadWorkMessageExportWorkerTask(ctx context.Context, id int64, owner string) (workMessageExportWorkerTask, bool, error) {
	var task workMessageExportWorkerTask
	var objectsJSON, scopesJSON, employeeScopeJSON []byte
	err := s.db.QueryRowContext(ctx, `SELECT id, tenant_id, corp_id, user_id, export_type, selected_objects_json,
		conversation_scopes_json, employee_scope_json, start_at, end_at, file_mode, format, lease_owner
		FROM mochat_go_work_message_export_tasks WHERE id=? AND lease_owner=?`, id, owner).Scan(
		&task.ID, &task.TenantID, &task.CorpID, &task.UserID, &task.ExportType, &objectsJSON, &scopesJSON, &employeeScopeJSON,
		&task.StartAt, &task.EndAt, &task.FileMode, &task.Format, &task.LeaseOwner)
	if errors.Is(err, sql.ErrNoRows) {
		return workMessageExportWorkerTask{}, false, nil
	}
	if err != nil {
		return workMessageExportWorkerTask{}, false, err
	}
	if err := json.Unmarshal(objectsJSON, &task.ObjectIDs); err != nil {
		return task, false, err
	}
	if err := json.Unmarshal(scopesJSON, &task.ConversationScopes); err != nil {
		return task, false, err
	}
	if err := json.Unmarshal(employeeScopeJSON, &task.EmployeeIDs); err != nil {
		return task, false, err
	}
	return task, true, nil
}

func (s *MySQLStore) processWorkMessageExportTask(ctx context.Context, root string, task workMessageExportWorkerTask) error {
	if strings.TrimSpace(root) == "" {
		return errors.New("导出文件根目录未配置")
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return err
	}
	rows, err := s.workMessageExportRows(ctx, task)
	if err != nil {
		return err
	}
	if len(rows) > 100000 {
		return fmt.Errorf("%w：实际读取 %d 条", dashboard.ErrWorkMessageExportMessageLimit, len(rows))
	}
	tempFile, err := os.CreateTemp(root, ".conversation-export-*.zip")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer func() { _ = os.Remove(tempPath) }()
	fileCount, err := writeWorkMessageExportZip(tempFile, task, rows)
	if closeErr := tempFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	artifactDir := filepath.Join(root, strconv.Itoa(task.TenantID), strconv.Itoa(task.CorpID))
	if err := os.MkdirAll(artifactDir, 0o750); err != nil {
		return err
	}
	artifactName := fmt.Sprintf("conversation-export-%d.zip", task.ID)
	artifactPath := filepath.Join(artifactDir, artifactName)
	if err := os.Rename(tempPath, artifactPath); err != nil {
		return err
	}
	stat, err := os.Stat(artifactPath)
	if err != nil {
		return err
	}
	hash, err := sha256File(artifactPath)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE mochat_go_work_message_export_tasks SET status='completed', message_count=?, file_count=?, artifact_name=?, artifact_path=?, artifact_size=?, artifact_sha256=?, finished_at=NOW(), lease_owner='', lease_expires_at=NULL WHERE id=? AND lease_owner=?`, len(rows), fileCount, artifactName, artifactPath, stat.Size(), hash, task.ID, task.LeaseOwner)
	return err
}

func (s *MySQLStore) workMessageExportRows(ctx context.Context, task workMessageExportWorkerTask) ([]workMessageExportRow, error) {
	state, err := s.archiveSourceRegistryState(ctx)
	if err != nil {
		return nil, err
	}
	filter := dashboard.WorkMessageUserFilter{TenantID: task.TenantID, UserID: task.UserID, CorpID: task.CorpID, ToUserType: -1,
		DateTimeStart: task.StartAt.Format("2006-01-02 15:04:05"), DateTimeEnd: task.EndAt.Format("2006-01-02 15:04:05"),
		RestrictEmployeeIDs: len(task.EmployeeIDs) > 0, EmployeeIDs: append([]int(nil), task.EmployeeIDs...), AllowAllEmployees: len(task.EmployeeIDs) == 0}
	sourceSQL, sourceArgs, ok := workMessageFilteredUnionSQLWithArchiveSourceState(filter, state)
	if !ok {
		return []workMessageExportRow{}, nil
	}
	where, whereArgs := workMessageExportObjectWhere(dashboard.WorkMessageExportCreateRequest{ExportType: task.ExportType, ConversationScopes: task.ConversationScopes}, task.ObjectIDs)
	args := append(append([]any{}, sourceArgs...), whereArgs...)
	rows, err := s.db.QueryContext(ctx, `SELECT id, table_index, seq, msgid, work_employee_id, employee_name, sender_name,
		target_name, to_user_type, to_user_id, msg_type, content_text, msg_data_time, is_current_user
		FROM (`+sourceSQL+`) export_messages WHERE `+where+`
		ORDER BY msg_data_time ASC, table_index ASC, seq ASC, id ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]workMessageExportRow, 0)
	for rows.Next() {
		var item workMessageExportRow
		var sentAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.TableIndex, &item.Seq, &item.MsgID, &item.EmployeeID, &item.EmployeeName, &item.SenderName,
			&item.TargetName, &item.ToUserType, &item.ToUserID, &item.MessageType, &item.Content, &sentAt, &item.IsCurrentUser); err != nil {
			return nil, err
		}
		item.SentAt = formatTime(sentAt)
		result = append(result, item)
		if len(result) > 100000 {
			return nil, fmt.Errorf("%w：读取过程中超过上限", dashboard.ErrWorkMessageExportMessageLimit)
		}
	}
	return result, rows.Err()
}

func writeWorkMessageExportZip(file *os.File, task workMessageExportWorkerTask, rows []workMessageExportRow) (int, error) {
	archive := zip.NewWriter(file)
	defer func() { _ = archive.Close() }()
	if task.FileMode == "merge" {
		entry, err := archive.Create("conversation-messages.csv")
		if err != nil {
			return 0, err
		}
		if err := writeWorkMessageExportCSV(entry, rows); err != nil {
			return 0, err
		}
		return 1, archive.Close()
	}
	grouped := map[string][]workMessageExportRow{}
	order := make([]string, 0)
	for _, row := range rows {
		key := strconv.Itoa(row.ToUserID)
		if _, ok := grouped[key]; !ok {
			order = append(order, key)
		}
		grouped[key] = append(grouped[key], row)
	}
	if len(order) == 0 {
		order = append(order, "empty")
	}
	for _, key := range order {
		entry, err := archive.Create("conversation-" + key + ".csv")
		if err != nil {
			return 0, err
		}
		if err := writeWorkMessageExportCSV(entry, grouped[key]); err != nil {
			return 0, err
		}
	}
	return len(order), archive.Close()
}

func writeWorkMessageExportCSV(writer io.Writer, rows []workMessageExportRow) error {
	if _, err := writer.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{"对象类型", "对象ID", "对象名称", "会话ID", "员工ID", "员工名称", "发送者", "方向", "消息类型", "消息内容", "发送时间", "归档来源", "归档来源ID"}); err != nil {
		return err
	}
	for _, row := range rows {
		objectType := "customer"
		if row.ToUserType == 2 {
			objectType = "room"
		}
		archiveID := "seq:" + strconv.FormatInt(row.Seq, 10)
		if strings.TrimSpace(row.MsgID) != "" {
			archiveID = "msg:" + row.MsgID
		}
		direction := "inbound"
		if row.IsCurrentUser == 1 {
			direction = "outbound"
		}
		if err := csvWriter.Write([]string{
			dashboard.SafeWorkMessageExportCSVCell(objectType), dashboard.SafeWorkMessageExportCSVCell(strconv.Itoa(row.ToUserID)),
			dashboard.SafeWorkMessageExportCSVCell(row.TargetName), dashboard.SafeWorkMessageExportCSVCell(fmt.Sprintf("%d:%d:%d", row.EmployeeID, row.ToUserType, row.ToUserID)),
			dashboard.SafeWorkMessageExportCSVCell(strconv.Itoa(row.EmployeeID)), dashboard.SafeWorkMessageExportCSVCell(row.EmployeeName), dashboard.SafeWorkMessageExportCSVCell(row.SenderName),
			dashboard.SafeWorkMessageExportCSVCell(direction), dashboard.SafeWorkMessageExportCSVCell(strconv.Itoa(row.MessageType)), dashboard.SafeWorkMessageExportCSVCell(row.Content),
			dashboard.SafeWorkMessageExportCSVCell(row.SentAt), "external", dashboard.SafeWorkMessageExportCSVCell(archiveID),
		}); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func truncateExportError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 500 {
		return value[:500]
	}
	return value
}
