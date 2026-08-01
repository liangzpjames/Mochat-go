package mysql

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

const (
	leadColumns = "id, tenant_id, corp_id, business_key, name, phone, source, status, owner_id, converted_contact_id, discard_reason, version, created_at, updated_at"
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
			(id, tenant_id, corp_id, business_key, name, phone, source, status, owner_id, converted_contact_id, discard_reason, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	phone := nullableLeadPhone(lead.Phone)
	_, err := r.db.ExecContext(
		ctx,
		insert,
		lead.ID,
		lead.TenantID,
		lead.CorpID,
		lead.BusinessKey,
		lead.Name.String(),
		phone,
		lead.Source,
		lead.Status,
		lead.OwnerID,
		lead.ConvertedContactID,
		lead.DiscardReason,
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

	existing, findErr := r.findByBusinessKey(ctx, lead.TenantID, lead.CorpID, lead.BusinessKey)
	if findErr == nil {
		return existing, false, nil
	}
	if !errors.Is(findErr, sql.ErrNoRows) {
		return domain.Lead{}, false, fmt.Errorf("find duplicate lead: %w", findErr)
	}
	return domain.Lead{}, false, ports.ErrDuplicateLead
}

func (r *LeadRepository) List(ctx context.Context, filter ports.ListLeadsFilter) (ports.LeadPage, error) {
	if filter.TenantID <= 0 {
		return ports.LeadPage{}, errors.New("tenant ID is required")
	}
	if filter.CorpID <= 0 || filter.Limit <= 0 {
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
	query := `SELECT ` + leadColumns + ` FROM ` + leadTable + ` WHERE tenant_id = ? AND corp_id = ?`
	args := []any{filter.TenantID, filter.CorpID}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		pattern := "%" + escapeLeadLike(keyword) + "%"
		query += ` AND (name LIKE ? ESCAPE '\\' OR phone LIKE ? ESCAPE '\\' OR business_key LIKE ? ESCAPE '\\')`
		args = append(args, pattern, pattern, pattern)
	}
	query, args = appendLeadInFilter(query, args, "status", leadStatuses(filter.Statuses))
	query, args = appendLeadInFilter(query, args, "source", leadSources(filter.Sources))
	if len(filter.OwnerIDs) > 0 {
		values := make([]string, 0, len(filter.OwnerIDs))
		for _, id := range filter.OwnerIDs {
			if id > 0 {
				values = append(values, fmt.Sprint(id))
			}
		}
		query, args = appendLeadInFilter(query, args, "owner_id", values)
	}
	if !filter.CreatedFrom.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, filter.CreatedFrom.UTC())
	}
	if !filter.CreatedTo.IsZero() {
		query += " AND created_at < ?"
		args = append(args, filter.CreatedTo.UTC())
	}
	if filter.Cursor != "" {
		cursor, err := decodeLeadCursor(filter.Cursor)
		if err != nil {
			return nil, err
		}
		query += " AND (created_at < ? OR (created_at = ? AND id < ?))"
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	query += " ORDER BY created_at DESC, id DESC LIMIT ?"
	args = append(args, filter.Limit+1)
	return r.db.QueryContext(ctx, query, args...)
}

func (r *LeadRepository) findByBusinessKey(ctx context.Context, tenantID, corpID int64, businessKey string) (domain.Lead, error) {
	const query = `
		SELECT ` + leadColumns + `
		FROM ` + leadTable + `
		WHERE tenant_id = ? AND corp_id = ? AND business_key = ?`
	return scanLead(r.db.QueryRowContext(ctx, query, tenantID, corpID, businessKey))
}

func (r *LeadRepository) FindDuplicates(ctx context.Context, filter ports.DuplicateLeadFilter) ([]domain.Lead, error) {
	if filter.TenantID <= 0 || filter.CorpID <= 0 || (strings.TrimSpace(filter.BusinessKey) == "" && strings.TrimSpace(filter.Phone) == "") {
		return nil, errors.New("duplicate filter is required")
	}
	query := `SELECT ` + leadColumns + ` FROM ` + leadTable + ` WHERE tenant_id=? AND corp_id=? AND (`
	args := []any{filter.TenantID, filter.CorpID}
	clauses := []string{}
	if key := strings.TrimSpace(filter.BusinessKey); key != "" {
		clauses = append(clauses, "business_key=?")
		args = append(args, key)
	}
	if phone := strings.TrimSpace(filter.Phone); phone != "" {
		clauses = append(clauses, "phone=?")
		args = append(args, phone)
	}
	query += strings.Join(clauses, " OR ") + `) ORDER BY created_at DESC, id DESC LIMIT 20`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Lead{}
	for rows.Next() {
		item, err := scanLead(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *LeadRepository) Assign(ctx context.Context, command ports.AssignLeadCommand) (domain.Lead, error) {
	return r.mutateLead(ctx, command.TenantID, command.CorpID, command.LeadID, command.Version, func(tx *sql.Tx) error {
		var employeeID int64
		err := tx.QueryRowContext(ctx, `
			SELECT e.id
			FROM mc_work_employee e
			INNER JOIN mc_corp c ON c.id = e.corp_id
			WHERE e.id = ? AND e.corp_id = ? AND e.status = 1 AND e.deleted_at IS NULL
			  AND c.tenant_id = ? AND c.deleted_at IS NULL`, command.OwnerID, command.CorpID, command.TenantID).Scan(&employeeID)
		if errors.Is(err, sql.ErrNoRows) {
			return ports.ErrLeadOwnerOutOfScope
		}
		return err
	}, func(lead *domain.Lead) error {
		if lead.Status == domain.LeadStatusNew {
			if err := lead.TransitionTo(domain.LeadStatusQualified, command.UpdatedAt); err != nil {
				return err
			}
		} else if lead.Status == domain.LeadStatusQualified {
			lead.Version++
			lead.UpdatedAt = command.UpdatedAt.UTC()
		} else {
			return domain.ErrInvalidLeadTransition
		}
		lead.OwnerID = &command.OwnerID
		return nil
	})
}

func (r *LeadRepository) Transition(ctx context.Context, command ports.TransitionLeadCommand) (domain.Lead, error) {
	return r.mutateLead(ctx, command.TenantID, command.CorpID, command.LeadID, command.Version, nil, func(lead *domain.Lead) error {
		if err := lead.TransitionTo(command.ToStatus, command.UpdatedAt); err != nil {
			return err
		}
		if command.ToStatus == domain.LeadStatusDiscarded {
			lead.DiscardReason = strings.TrimSpace(command.DiscardReason)
		}
		if command.ToStatus == domain.LeadStatusConverted {
			lead.ConvertedContactID = lead.ID
		}
		return nil
	})
}

func (r *LeadRepository) mutateLead(ctx context.Context, tenantID, corpID int64, leadID string, version int64, before func(*sql.Tx) error, apply func(*domain.Lead) error) (domain.Lead, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Lead{}, err
	}
	defer tx.Rollback()
	if before != nil {
		if err := before(tx); err != nil {
			return domain.Lead{}, err
		}
	}
	lead, err := scanLead(tx.QueryRowContext(ctx, `SELECT `+leadColumns+` FROM `+leadTable+` WHERE tenant_id=? AND corp_id=? AND id=? FOR UPDATE`, tenantID, corpID, leadID))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Lead{}, ports.ErrLeadNotFound
	}
	if err != nil {
		return domain.Lead{}, err
	}
	if lead.Version != version {
		return domain.Lead{}, ports.ErrLeadConflict
	}
	if err := apply(&lead); err != nil {
		return domain.Lead{}, err
	}
	if lead.Status == domain.LeadStatusConverted {
		if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_scrm_contacts (id,tenant_id,corp_id,name,phone,version,created_at,updated_at) VALUES (?,?,?,?,?,1,?,?)`, lead.ID, lead.TenantID, lead.CorpID, lead.Name.String(), lead.Phone, lead.UpdatedAt, lead.UpdatedAt); err != nil {
			return domain.Lead{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_scrm_assignments (id,tenant_id,corp_id,contact_id,owner_id,status,version,created_at,updated_at) VALUES (?,?,?,?,?,?,1,?,?)`, lead.ID, lead.TenantID, lead.CorpID, lead.ID, lead.OwnerID, assignmentStatus(lead.OwnerID), lead.UpdatedAt, lead.UpdatedAt); err != nil {
			return domain.Lead{}, err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE `+leadTable+` SET status=?,owner_id=?,converted_contact_id=?,discard_reason=?,version=?,updated_at=? WHERE tenant_id=? AND corp_id=? AND id=? AND version=?`, lead.Status, lead.OwnerID, lead.ConvertedContactID, lead.DiscardReason, lead.Version, lead.UpdatedAt, tenantID, corpID, leadID, version)
	if err != nil {
		return domain.Lead{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return domain.Lead{}, ports.ErrLeadConflict
	}
	if err := tx.Commit(); err != nil {
		return domain.Lead{}, err
	}
	return lead, nil
}

func assignmentStatus(ownerID *int64) string {
	if ownerID == nil {
		return domain.AssignmentPublicPool
	}
	return domain.AssignmentOwned
}
func nullableLeadPhone(phone string) any {
	if strings.TrimSpace(phone) == "" {
		return nil
	}
	return strings.TrimSpace(phone)
}
func escapeLeadLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
func appendLeadInFilter(query string, args []any, column string, values []string) (string, []any) {
	if len(values) == 0 {
		return query, args
	}
	placeholders := make([]string, len(values))
	for i, value := range values {
		placeholders[i] = "?"
		args = append(args, value)
	}
	return query + " AND " + column + " IN (" + strings.Join(placeholders, ",") + ")", args
}
func leadStatuses(values []domain.LeadStatus) []string {
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = string(v)
	}
	return result
}
func leadSources(values []domain.LeadSource) []string {
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = string(v)
	}
	return result
}

type leadScanner interface {
	Scan(...any) error
}

func scanLead(scanner leadScanner) (domain.Lead, error) {
	var (
		id                 string
		tenantID           int64
		corpID             int64
		businessKey        string
		name               string
		phone              sql.NullString
		source             domain.LeadSource
		status             domain.LeadStatus
		ownerID            sql.NullInt64
		convertedContactID string
		discardReason      string
		version            int64
		createdAt          time.Time
		updatedAt          time.Time
	)
	if err := scanner.Scan(
		&id,
		&tenantID,
		&corpID,
		&businessKey,
		&name,
		&phone,
		&source,
		&status,
		&ownerID,
		&convertedContactID,
		&discardReason,
		&version,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.Lead{}, err
	}

	lead, err := domain.NewCorpLead(id, tenantID, corpID, businessKey, name, phone.String, source, createdAt.UTC())
	if err != nil {
		return domain.Lead{}, err
	}
	lead.Status = status
	if ownerID.Valid {
		value := ownerID.Int64
		lead.OwnerID = &value
	}
	lead.ConvertedContactID, lead.DiscardReason = convertedContactID, discardReason
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
		return leadCursor{}, fmt.Errorf("%w: decode base64: %v", ports.ErrInvalidCursor, err)
	}
	var cursor leadCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return leadCursor{}, fmt.Errorf("%w: decode JSON: %v", ports.ErrInvalidCursor, err)
	}
	if cursor.CreatedAt.IsZero() || cursor.ID == "" {
		return leadCursor{}, fmt.Errorf("%w: timestamp and ID are required", ports.ErrInvalidCursor)
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
