package mysql

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

const (
	leadColumns = "id, tenant_id, business_key, name, source, status, version, created_at, updated_at"
	leadTable   = "mochat_go_scrm_leads"
)

var _ ports.LeadRepository = (*LeadRepository)(nil)

type LeadRepository struct {
	db *sql.DB
}

func NewLeadRepository(db *sql.DB) (*LeadRepository, error) {
	if db == nil {
		return nil, errors.New("lead repository database is required")
	}
	return &LeadRepository{db: db}, nil
}

func (r *LeadRepository) CreateOrGet(ctx context.Context, lead domain.Lead) (domain.Lead, bool, error) {
	const insert = `
		INSERT INTO mochat_go_scrm_leads
			(id, tenant_id, business_key, name, source, status, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(
		ctx,
		insert,
		lead.ID,
		lead.TenantID,
		lead.BusinessKey,
		lead.Name.String(),
		lead.Source,
		lead.Status,
		lead.Version,
		lead.CreatedAt.UTC(),
		lead.UpdatedAt.UTC(),
	)
	if err == nil {
		return lead, true, nil
	}

	var mysqlError *mysqldriver.MySQLError
	if !errors.As(err, &mysqlError) || mysqlError.Number != 1062 {
		return domain.Lead{}, false, fmt.Errorf("insert lead: %w", err)
	}

	existing, findErr := r.findByBusinessKey(ctx, lead.TenantID, lead.BusinessKey)
	if findErr != nil {
		return domain.Lead{}, false, fmt.Errorf("find duplicate lead: %w", findErr)
	}
	return existing, false, nil
}

func (r *LeadRepository) List(ctx context.Context, filter ports.ListLeadsFilter) (ports.LeadPage, error) {
	if filter.TenantID <= 0 {
		return ports.LeadPage{}, errors.New("tenant ID is required")
	}
	if filter.Limit <= 0 {
		return ports.LeadPage{}, errors.New("list limit must be positive")
	}

	rows, err := r.listRows(ctx, filter)
	if err != nil {
		return ports.LeadPage{}, fmt.Errorf("list leads: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Lead, 0, filter.Limit+1)
	for rows.Next() {
		lead, err := scanLead(rows)
		if err != nil {
			return ports.LeadPage{}, fmt.Errorf("scan lead: %w", err)
		}
		items = append(items, lead)
	}
	if err := rows.Err(); err != nil {
		return ports.LeadPage{}, fmt.Errorf("iterate leads: %w", err)
	}

	page := ports.LeadPage{Items: items}
	if len(items) > filter.Limit {
		page.Items = items[:filter.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor, err = encodeLeadCursor(leadCursor{
			CreatedAt: last.CreatedAt.UTC(),
			ID:        last.ID,
		})
		if err != nil {
			return ports.LeadPage{}, err
		}
	}
	return page, nil
}

func (r *LeadRepository) listRows(ctx context.Context, filter ports.ListLeadsFilter) (*sql.Rows, error) {
	if filter.Cursor == "" {
		const firstPageQuery = `
			SELECT ` + leadColumns + `
			FROM ` + leadTable + `
			WHERE tenant_id = ?
			ORDER BY created_at DESC, id DESC
			LIMIT ?`
		return r.db.QueryContext(ctx, firstPageQuery, filter.TenantID, filter.Limit+1)
	}

	cursor, err := decodeLeadCursor(filter.Cursor)
	if err != nil {
		return nil, err
	}
	const nextPageQuery = `
		SELECT ` + leadColumns + `
		FROM ` + leadTable + `
		WHERE tenant_id = ?
		  AND (created_at < ? OR (created_at = ? AND id < ?))
		ORDER BY created_at DESC, id DESC
		LIMIT ?`
	return r.db.QueryContext(
		ctx,
		nextPageQuery,
		filter.TenantID,
		cursor.CreatedAt,
		cursor.CreatedAt,
		cursor.ID,
		filter.Limit+1,
	)
}

func (r *LeadRepository) findByBusinessKey(ctx context.Context, tenantID int64, businessKey string) (domain.Lead, error) {
	const query = `
		SELECT ` + leadColumns + `
		FROM ` + leadTable + `
		WHERE tenant_id = ? AND business_key = ?`
	return scanLead(r.db.QueryRowContext(ctx, query, tenantID, businessKey))
}

type leadScanner interface {
	Scan(...any) error
}

func scanLead(scanner leadScanner) (domain.Lead, error) {
	var (
		id          string
		tenantID    int64
		businessKey string
		name        string
		source      domain.LeadSource
		status      domain.LeadStatus
		version     int64
		createdAt   time.Time
		updatedAt   time.Time
	)
	if err := scanner.Scan(
		&id,
		&tenantID,
		&businessKey,
		&name,
		&source,
		&status,
		&version,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.Lead{}, err
	}

	lead, err := domain.NewLead(id, tenantID, businessKey, name, source, createdAt.UTC())
	if err != nil {
		return domain.Lead{}, err
	}
	lead.Status = status
	lead.Version = version
	lead.UpdatedAt = updatedAt.UTC()
	return lead, nil
}

type leadCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

func decodeLeadCursor(encoded string) (leadCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return leadCursor{}, fmt.Errorf("decode lead cursor: %w", err)
	}
	var cursor leadCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return leadCursor{}, fmt.Errorf("decode lead cursor: %w", err)
	}
	if cursor.CreatedAt.IsZero() || cursor.ID == "" {
		return leadCursor{}, errors.New("decode lead cursor: timestamp and ID are required")
	}
	cursor.CreatedAt = cursor.CreatedAt.UTC()
	return cursor, nil
}

func encodeLeadCursor(cursor leadCursor) (string, error) {
	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("encode lead cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
