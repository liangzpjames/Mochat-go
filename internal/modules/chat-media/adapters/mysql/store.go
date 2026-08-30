package http

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type AudioObject struct {
	ID              int64      `json:"id"`
	TenantID        int64      `json:"tenantId"`
	UserID          int64      `json:"userId"`
	EmployeeID      int64      `json:"employeeId"`
	CorpID          int64      `json:"corpId"`
	OriginalName    string     `json:"originalName"`
	Source          string     `json:"source"`
	MessageID       string     `json:"messageId"`
	SenderName      string     `json:"senderName"`
	ReceiverName    string     `json:"receiverName"`
	RelativePath    string     `json:"-"`
	ContentType     string     `json:"contentType"`
	SizeBytes       int64      `json:"sizeBytes"`
	DurationSeconds int64      `json:"durationSeconds"`
	SHA256          string     `json:"sha256"`
	CreatedAt       time.Time  `json:"createdAt"`
	SyncedAt        *time.Time `json:"syncedAt,omitempty"`
	PlayURL         string     `json:"playUrl"`
	DeletedAt       *time.Time `json:"-"`
	DeletedBy       int64      `json:"-"`
}

type ListResult struct {
	List    []AudioObject `json:"list"`
	Total   int64         `json:"total"`
	Page    int           `json:"page"`
	PerPage int           `json:"perPage"`
}

type MediaListFilter struct {
	CorpID     int64
	Page       int
	PerPage    int
	Sender     string
	Receiver   string
	SyncedFrom string
	SyncedTo   string
}

type MediaStore interface {
	GetByID(ctx context.Context, id int64) (*AudioObject, error)
	List(ctx context.Context, filter MediaListFilter) (ListResult, error)
}

type DurationUpdater interface {
	UpdateDuration(ctx context.Context, id int64, durationSeconds int64) error
}

type SQLMediaStore struct {
	db *sql.DB
}

var _ MediaStore = (*SQLMediaStore)(nil)

func NewSQLMediaStore(db *sql.DB) *SQLMediaStore {
	return &SQLMediaStore{db: db}
}

const mediaColumns = "id, tenant_id, user_id, employee_id, corp_id, original_name, source, message_id, sender_name, receiver_name, relative_path, content_type, size_bytes, duration_seconds, sha256, created_at, synced_at"

func (s *SQLMediaStore) Create(ctx context.Context, object AudioObject) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mochat_go_audio_objects
			(tenant_id, user_id, employee_id, corp_id, original_name, relative_path, content_type, size_bytes, duration_seconds, sha256, created_at, updated_at, deleted_at, deleted_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NULL, 0)
	`, object.TenantID, object.UserID, object.EmployeeID, object.CorpID, object.OriginalName, object.RelativePath, object.ContentType, object.SizeBytes, object.DurationSeconds, object.SHA256)
	if err != nil {
		return 0, fmt.Errorf("insert audio object: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read audio object id: %w", err)
	}
	return id, nil
}

func (s *SQLMediaStore) GetByID(ctx context.Context, id int64) (*AudioObject, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+mediaColumns+`
		FROM mochat_go_audio_objects
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, id)
	object, err := scanAudioObject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return object, nil
}

func (s *SQLMediaStore) List(ctx context.Context, filter MediaListFilter) (ListResult, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 || filter.PerPage > 100 {
		filter.PerPage = 20
	}
	where := "corp_id = ? AND deleted_at IS NULL AND source = 'wecom_sync'"
	args := []any{filter.CorpID}
	if filter.Sender = strings.TrimSpace(filter.Sender); filter.Sender != "" {
		where += " AND sender_name LIKE ?"
		args = append(args, "%"+filter.Sender+"%")
	}
	if filter.Receiver = strings.TrimSpace(filter.Receiver); filter.Receiver != "" {
		where += " AND receiver_name LIKE ?"
		args = append(args, "%"+filter.Receiver+"%")
	}
	if filter.SyncedFrom != "" {
		where += " AND synced_at >= ?"
		args = append(args, filter.SyncedFrom+" 00:00:00")
	}
	if filter.SyncedTo != "" {
		where += " AND synced_at < DATE_ADD(?, INTERVAL 1 DAY)"
		args = append(args, filter.SyncedTo+" 00:00:00")
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_audio_objects WHERE "+where, args...).Scan(&total); err != nil {
		return ListResult{}, fmt.Errorf("count audio objects: %w", err)
	}
	offset := (filter.Page - 1) * filter.PerPage
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+mediaColumns+`
		FROM mochat_go_audio_objects
		WHERE `+where+`
		ORDER BY synced_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, append(args, filter.PerPage, offset)...)
	if err != nil {
		return ListResult{}, fmt.Errorf("list audio objects: %w", err)
	}
	defer rows.Close()
	list := make([]AudioObject, 0, filter.PerPage)
	for rows.Next() {
		object, err := scanAudioObject(rows)
		if err != nil {
			return ListResult{}, err
		}
		list = append(list, *object)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}
	return ListResult{List: list, Total: total, Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *SQLMediaStore) UpdateDuration(ctx context.Context, id int64, durationSeconds int64) error {
	if durationSeconds <= 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_audio_objects
		SET duration_seconds = ?, updated_at = NOW()
		WHERE id = ? AND source = 'wecom_sync' AND deleted_at IS NULL
	`, durationSeconds, id)
	return err
}

func (s *SQLMediaStore) SoftDelete(ctx context.Context, id int64, deletedBy int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_audio_objects
		SET deleted_at = NOW(), deleted_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, deletedBy, id)
	if err != nil {
		return fmt.Errorf("soft delete audio object: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAudioObject(row scanner) (*AudioObject, error) {
	var object AudioObject
	var createdAt time.Time
	if err := row.Scan(
		&object.ID, &object.TenantID, &object.UserID, &object.EmployeeID, &object.CorpID,
		&object.OriginalName, &object.Source, &object.MessageID, &object.SenderName, &object.ReceiverName,
		&object.RelativePath, &object.ContentType,
		&object.SizeBytes, &object.DurationSeconds, &object.SHA256, &createdAt, &object.SyncedAt,
	); err != nil {
		return nil, err
	}
	object.CreatedAt = createdAt
	return &object, nil
}
