package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

type CustomerLifecycleRepository struct{ db *sql.DB }

func NewCustomerLifecycleRepository(db *sql.DB) (*CustomerLifecycleRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("customer lifecycle database is required")
	}
	return &CustomerLifecycleRepository{db: db}, nil
}

func (r *CustomerLifecycleRepository) ListPublicPool(ctx context.Context, filter ports.ListPublicPoolFilter) (ports.AssignmentPage, error) {
	offset, err := parseAssignmentCursor(filter.Cursor)
	if err != nil {
		return ports.AssignmentPage{}, err
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, contact_id, owner_id, status, version, updated_at
		FROM mochat_go_scrm_assignments WHERE tenant_id=? AND corp_id=? AND status=? AND deleted_at IS NULL
		ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?`, filter.TenantID, filter.CorpID, domain.AssignmentPublicPool, limit+1, offset)
	if err != nil {
		return ports.AssignmentPage{}, err
	}
	defer rows.Close()
	items := make([]domain.CustomerAssignment, 0, limit)
	for rows.Next() {
		var item domain.CustomerAssignment
		if err := rows.Scan(&item.ID, &item.ContactID, &item.OwnerID, &item.Status, &item.Version, &item.UpdatedAt); err != nil {
			return ports.AssignmentPage{}, err
		}
		item.TenantID, item.CorpID = filter.TenantID, filter.CorpID
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ports.AssignmentPage{}, err
	}
	page := ports.AssignmentPage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		page.NextCursor = strconv.Itoa(offset + limit)
	}
	return page, nil
}

func (r *CustomerLifecycleRepository) UpdateAssignment(ctx context.Context, command ports.UpdateAssignmentCommand) (domain.CustomerAssignment, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.CustomerAssignment{}, err
	}
	defer tx.Rollback()
	status := domain.AssignmentOwned
	if command.OwnerID == nil {
		status = domain.AssignmentPublicPool
	}
	if command.OwnerID != nil && len(command.CollaboratorIDs) > 0 {
		status = domain.AssignmentCollaborating
	}
	result, err := tx.ExecContext(ctx, `UPDATE mochat_go_scrm_assignments SET owner_id=?, status=?, version=version+1, updated_at=?
		WHERE tenant_id=? AND corp_id=? AND contact_id=? AND version=? AND deleted_at IS NULL`, command.OwnerID, status, time.Now().UTC(), command.TenantID, command.CorpID, command.ContactID, command.Version)
	if err != nil {
		return domain.CustomerAssignment{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return domain.CustomerAssignment{}, ports.ErrAssignmentConflict
	}
	assignment, err := loadAssignmentTx(ctx, tx, command.TenantID, command.CorpID, command.ContactID)
	if err != nil {
		return domain.CustomerAssignment{}, err
	}
	if err := replaceCollaborators(ctx, tx, assignment, command.CollaboratorIDs); err != nil {
		return domain.CustomerAssignment{}, err
	}
	assignment.CollaboratorIDs = append([]int64(nil), command.CollaboratorIDs...)
	if err := tx.Commit(); err != nil {
		return domain.CustomerAssignment{}, err
	}
	return assignment, nil
}

func (r *CustomerLifecycleRepository) ReleaseToPublicPool(ctx context.Context, tenantID, corpID int64, contactID string, version int64, _ string) (domain.CustomerAssignment, error) {
	return r.transition(ctx, tenantID, corpID, contactID, version, `status<>?`, domain.AssignmentPublicPool, nil)
}

func (r *CustomerLifecycleRepository) ClaimFromPublicPool(ctx context.Context, tenantID, corpID int64, contactID string, userID, version int64, _ string) (domain.CustomerAssignment, error) {
	return r.transition(ctx, tenantID, corpID, contactID, version, `status=? AND owner_id IS NULL`, domain.AssignmentOwned, userID)
}

func (r *CustomerLifecycleRepository) transition(ctx context.Context, tenantID, corpID int64, contactID string, version int64, extra, status string, owner any) (domain.CustomerAssignment, error) {
	args := []any{owner, status, time.Now().UTC(), tenantID, corpID, contactID, version}
	query := `UPDATE mochat_go_scrm_assignments SET owner_id=?, status=?, version=version+1, updated_at=? WHERE tenant_id=? AND corp_id=? AND contact_id=? AND version=? AND deleted_at IS NULL AND ` + extra
	if extra == `status=? AND owner_id IS NULL` || extra == `status<>?` {
		args = append(args, status)
	}
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return domain.CustomerAssignment{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return domain.CustomerAssignment{}, ports.ErrAssignmentConflict
	}
	return r.loadAssignment(ctx, tenantID, corpID, contactID)
}

func (r *CustomerLifecycleRepository) loadAssignment(ctx context.Context, tenantID, corpID int64, contactID string) (domain.CustomerAssignment, error) {
	return loadAssignmentTx(ctx, r.db, tenantID, corpID, contactID)
}

type txQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func loadAssignmentTx(ctx context.Context, q txQuerier, tenantID, corpID int64, contactID string) (domain.CustomerAssignment, error) {
	var item domain.CustomerAssignment
	if err := q.QueryRowContext(ctx, `SELECT id, contact_id, owner_id, status, version, updated_at FROM mochat_go_scrm_assignments WHERE tenant_id=? AND corp_id=? AND contact_id=? AND deleted_at IS NULL`, tenantID, corpID, contactID).Scan(&item.ID, &item.ContactID, &item.OwnerID, &item.Status, &item.Version, &item.UpdatedAt); err != nil {
		return item, err
	}
	item.TenantID, item.CorpID = tenantID, corpID
	rows, err := q.QueryContext(ctx, `SELECT user_id FROM mochat_go_scrm_assignment_collaborators WHERE tenant_id=? AND corp_id=? AND assignment_id=? ORDER BY user_id`, tenantID, corpID, item.ID)
	if err != nil {
		return item, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return item, err
		}
		item.CollaboratorIDs = append(item.CollaboratorIDs, id)
	}
	return item, rows.Err()
}

func replaceCollaborators(ctx context.Context, tx *sql.Tx, item domain.CustomerAssignment, ids []int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM mochat_go_scrm_assignment_collaborators WHERE tenant_id=? AND corp_id=? AND assignment_id=?`, item.TenantID, item.CorpID, item.ID); err != nil {
		return err
	}
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("invalid collaborator ID")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_scrm_assignment_collaborators (tenant_id, corp_id, assignment_id, user_id, created_at) VALUES (?,?,?,?,?)`, item.TenantID, item.CorpID, item.ID, id, time.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

func parseAssignmentCursor(cursor string) (int, error) {
	if strings.TrimSpace(cursor) == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(cursor)
	if err != nil || offset < 0 {
		return 0, ports.ErrInvalidCursor
	}
	return offset, nil
}
