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
	RelativePath    string     `json:"-"`
	ContentType     string     `json:"contentType"`
	SizeBytes       int64      `json:"sizeBytes"`
	DurationSeconds int64      `json:"durationSeconds"`
	SHA256          string     `json:"sha256"`
	CreatedAt       time.Time  `json:"createdAt"`
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

type MediaStore interface {
	Create(ctx context.Context, object AudioObject) (int64, error)
	GetByID(ctx context.Context, id int64) (*AudioObject, error)
	List(ctx context.Context, corpID int64, page int, perPage int, keyword string) (ListResult, error)
	SoftDelete(ctx context.Context, id int64, deletedBy int64) error
}

type SQLMediaStore struct {
	db *sql.DB
}

var _ MediaStore = (*SQLMediaStore)(nil)

func NewSQLMediaStore(db *sql.DB) *SQLMediaStore {
	return &SQLMediaStore{db: db}
}

const mediaColumns = "id, tenant_id, user_id, employee_id, corp_id, original_name, relative_path, content_type, size_bytes, duration_seconds, sha256, created_at"

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

func (s *SQLMediaStore) List(ctx context.Context, corpID int64, page int, perPage int, keyword string) (ListResult, error) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}
	where := "corp_id = ? AND deleted_at IS NULL"
	args := []any{corpID}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		where += " AND original_name LIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_audio_objects WHERE "+where, args...).Scan(&total); err != nil {
		return ListResult{}, fmt.Errorf("count audio objects: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+mediaColumns+`
		FROM mochat_go_audio_objects
		WHERE `+where+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, append(args, perPage, offset)...)
	if err != nil {
		return ListResult{}, fmt.Errorf("list audio objects: %w", err)
	}
	defer rows.Close()
	list := make([]AudioObject, 0, perPage)
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
	return ListResult{List: list, Total: total, Page: page, PerPage: perPage}, nil
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
		&object.OriginalName, &object.RelativePath, &object.ContentType,
		&object.SizeBytes, &object.DurationSeconds, &object.SHA256, &createdAt,
	); err != nil {
		return nil, err
	}
	object.CreatedAt = createdAt
	return &object, nil
}
