package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type AnalysisRow struct {
	Payload   string
	CreatedAt time.Time
}

type AnalysisStore interface {
	Latest(ctx context.Context, corpID int64, page string) (*AnalysisRow, error)
	Save(ctx context.Context, corpID int64, page string, status string, payload any, message string) error
}

type SQLAnalysisStore struct {
	db *sql.DB
}

var _ AnalysisStore = (*SQLAnalysisStore)(nil)

func NewSQLAnalysisStore(db *sql.DB) *SQLAnalysisStore {
	return &SQLAnalysisStore{db: db}
}

func (s *SQLAnalysisStore) Latest(ctx context.Context, corpID int64, page string) (*AnalysisRow, error) {
	var row AnalysisRow
	err := s.db.QueryRowContext(ctx, `
		SELECT payload, created_at
		FROM mochat_go_ai_analysis
		WHERE corp_id = ? AND page = ? AND status = 'succeeded'
		ORDER BY id DESC
		LIMIT 1
	`, corpID, page).Scan(&row.Payload, &row.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *SQLAnalysisStore) Save(ctx context.Context, corpID int64, page string, status string, payload any, message string) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO mochat_go_ai_analysis
			(tenant_id, corp_id, page, status, payload, error, created_at, updated_at)
		VALUES (0, ?, ?, ?, ?, ?, NOW(), NOW())
	`, corpID, page, status, string(encoded), message)
	if err != nil {
		return fmt.Errorf("save AI analysis: %w", err)
	}
	return nil
}

func FetchArchiveTexts(ctx context.Context, db *sql.DB, corpID int64, limit int, allowedEmployeeIDs []int64, restricted bool) ([]string, error) {
	if db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	texts := make([]string, 0, limit)
	for table := 1; table <= 10 && len(texts) < limit; table++ {
		query := fmt.Sprintf(`
			SELECT content_text
			FROM mc_work_message_%d
			WHERE corp_id = ? AND content_text IS NOT NULL AND deleted_at IS NULL`, table)
		args := []any{corpID}
		if restricted {
			if len(allowedEmployeeIDs) == 0 {
				query += " AND 1=0"
			} else {
				query += " AND work_employee_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(allowedEmployeeIDs)), ",") + ")"
				for _, id := range allowedEmployeeIDs {
					args = append(args, id)
				}
			}
		}
		query += `
			ORDER BY msg_data_time DESC
			LIMIT ?`
		args = append(args, limit-len(texts))
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			// The shard may not exist in fresh deployments; skip it.
			continue
		}
		for rows.Next() {
			var text string
			if err := rows.Scan(&text); err != nil {
				rows.Close()
				return texts, err
			}
			if text = trimSpace(text); text != "" {
				texts = append(texts, text)
				if len(texts) >= limit {
					break
				}
			}
		}
		_ = rows.Close()
		_ = rows.Err()
	}
	return texts, nil
}

func trimSpace(value string) string {
	start := 0
	for start < len(value) && (value[start] == ' ' || value[start] == '\t' || value[start] == '\n' || value[start] == '\r') {
		start++
	}
	end := len(value)
	for end > start && (value[end-1] == ' ' || value[end-1] == '\t' || value[end-1] == '\n' || value[end-1] == '\r') {
		end--
	}
	return value[start:end]
}
