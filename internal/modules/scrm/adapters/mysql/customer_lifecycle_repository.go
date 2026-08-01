package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
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

func (r *CustomerLifecycleRepository) ListContacts(ctx context.Context, filter ports.ListContactsFilter) (ports.ContactPage, error) {
	offset, err := parseAssignmentCursor(filter.Cursor)
	if err != nil {
		return ports.ContactPage{}, err
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	where := []string{"c.tenant_id=?", "c.corp_id=?", "c.deleted_at IS NULL"}
	args := []any{filter.TenantID, filter.CorpID}
	if filter.Keyword != "" {
		like := "%" + escapeLike(filter.Keyword) + "%"
		where = append(where, `(c.name LIKE ? ESCAPE '\\' OR c.phone LIKE ? ESCAPE '\\')`)
		args = append(args, like, like)
	}
	if len(filter.OwnerIDs) > 0 {
		where = append(where, "a.owner_id IN ("+sqlPlaceholders(len(filter.OwnerIDs))+")")
		for _, id := range filter.OwnerIDs {
			args = append(args, id)
		}
	}
	if len(filter.Statuses) > 0 {
		where = append(where, "a.status IN ("+sqlPlaceholders(len(filter.Statuses))+")")
		for _, status := range filter.Statuses {
			args = append(args, status)
		}
	}
	if len(filter.TagIDs) > 0 {
		where = append(where, `EXISTS (SELECT 1 FROM mochat_go_scrm_contact_tags wanted WHERE wanted.tenant_id=c.tenant_id AND wanted.corp_id=c.corp_id AND wanted.contact_id=c.id AND wanted.tag_id IN (`+sqlPlaceholders(len(filter.TagIDs))+`))`)
		for _, id := range filter.TagIDs {
			args = append(args, id)
		}
	}
	args = append(args, limit+1, offset)
	rows, err := r.db.QueryContext(ctx, `SELECT c.id,c.name,c.phone,c.version,c.updated_at,a.owner_id,COALESCE(a.status,''),COALESCE(a.version,0),COALESCE(GROUP_CONCAT(DISTINCT t.name ORDER BY t.name SEPARATOR '\x1f'),'')
		FROM mochat_go_scrm_contacts c
		LEFT JOIN mochat_go_scrm_assignments a ON a.tenant_id=c.tenant_id AND a.corp_id=c.corp_id AND a.contact_id=c.id AND a.deleted_at IS NULL
		LEFT JOIN mochat_go_scrm_contact_tags ct ON ct.tenant_id=c.tenant_id AND ct.corp_id=c.corp_id AND ct.contact_id=c.id
		LEFT JOIN mochat_go_scrm_tags t ON t.tenant_id=ct.tenant_id AND t.corp_id=ct.corp_id AND t.id=ct.tag_id AND t.deleted_at IS NULL
		WHERE `+strings.Join(where, " AND ")+`
		GROUP BY c.id,c.name,c.phone,c.version,c.updated_at,a.owner_id,a.status,a.version
		ORDER BY c.updated_at DESC,c.id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return ports.ContactPage{}, err
	}
	defer rows.Close()
	items := make([]ports.ContactSummary, 0, limit+1)
	for rows.Next() {
		var item ports.ContactSummary
		var tags string
		if err := rows.Scan(&item.ID, &item.Name, &item.Phone, &item.Version, &item.UpdatedAt, &item.OwnerID, &item.AssignmentStatus, &item.AssignmentVersion, &tags); err != nil {
			return ports.ContactPage{}, err
		}
		if tags != "" {
			item.TagNames = strings.Split(tags, "\x1f")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ports.ContactPage{}, err
	}
	page := ports.ContactPage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		page.NextCursor = strconv.Itoa(offset + limit)
	}
	return page, nil
}

func (r *CustomerLifecycleRepository) GetContact(ctx context.Context, tenantID, corpID int64, contactID string) (ports.ContactDetail, error) {
	var detail ports.ContactDetail
	if err := r.db.QueryRowContext(ctx, `SELECT id,name,phone,version,updated_at FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id=? AND id=? AND deleted_at IS NULL`, tenantID, corpID, contactID).Scan(&detail.ID, &detail.Name, &detail.Phone, &detail.Version, &detail.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return detail, ports.ErrContactNotFound
		}
		return detail, err
	}
	assignment, err := r.loadAssignment(ctx, tenantID, corpID, contactID)
	if err != nil {
		if err != sql.ErrNoRows {
			return detail, err
		}
	} else {
		detail.Assignment = assignment
		detail.OwnerID = assignment.OwnerID
		detail.AssignmentStatus = assignment.Status
		detail.AssignmentVersion = assignment.Version
	}
	if err := r.loadContactTags(ctx, tenantID, corpID, contactID, &detail); err != nil {
		return detail, err
	}
	// There is no durable identity relation from mochat_go_scrm_contacts to
	// mc_work_contact. A display name is not an identity key, so do not guess.
	detail.WeComFriends = []ports.WeComFriendSummary{}
	detail.WeComFriendsAvailable = false
	if err := r.loadContactOpportunities(ctx, tenantID, corpID, contactID, &detail); err != nil {
		return detail, err
	}
	if err := r.loadContactFollowUps(ctx, tenantID, corpID, contactID, &detail); err != nil {
		return detail, err
	}
	return detail, nil
}

func (r *CustomerLifecycleRepository) loadContactTags(ctx context.Context, tenantID, corpID int64, contactID string, detail *ports.ContactDetail) error {
	rows, err := r.db.QueryContext(ctx, `SELECT t.id,t.name FROM mochat_go_scrm_contact_tags ct JOIN mochat_go_scrm_tags t ON t.tenant_id=ct.tenant_id AND t.corp_id=ct.corp_id AND t.id=ct.tag_id AND t.deleted_at IS NULL WHERE ct.tenant_id=? AND ct.corp_id=? AND ct.contact_id=? ORDER BY t.name,t.id`, tenantID, corpID, contactID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item ports.ContactTagSummary
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return err
		}
		detail.Tags = append(detail.Tags, item)
		detail.TagNames = append(detail.TagNames, item.Name)
	}
	return rows.Err()
}
func (r *CustomerLifecycleRepository) loadContactOpportunities(ctx context.Context, tenantID, corpID int64, contactID string, detail *ports.ContactDetail) error {
	rows, err := r.db.QueryContext(ctx, `SELECT id,stage_id,status,amount,version FROM mochat_go_scrm_opportunities WHERE tenant_id=? AND corp_id=? AND contact_id=? AND deleted_at IS NULL ORDER BY updated_at DESC,id DESC`, tenantID, corpID, contactID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item ports.ContactOpportunitySummary
		if err := rows.Scan(&item.ID, &item.Stage, &item.Status, &item.Amount, &item.Version); err != nil {
			return err
		}
		detail.Opportunities = append(detail.Opportunities, item)
	}
	return rows.Err()
}
func (r *CustomerLifecycleRepository) loadContactFollowUps(ctx context.Context, tenantID, corpID int64, contactID string, detail *ports.ContactDetail) error {
	rows, err := r.db.QueryContext(ctx, `SELECT id,content,created_by,created_at FROM mochat_go_scrm_follow_ups WHERE tenant_id=? AND corp_id=? AND contact_id=? ORDER BY created_at,id`, tenantID, corpID, contactID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item ports.ContactFollowUpSummary
		if err := rows.Scan(&item.ID, &item.Content, &item.CreatedBy, &item.CreatedAt); err != nil {
			return err
		}
		detail.FollowUps = append(detail.FollowUps, item)
	}
	return rows.Err()
}

func sqlPlaceholders(count int) string { return strings.TrimRight(strings.Repeat("?,", count), ",") }
func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
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
	collaborators := append([]int64(nil), command.CollaboratorIDs...)
	sort.Slice(collaborators, func(i, j int) bool { return collaborators[i] < collaborators[j] })
	fingerprint := requestFingerprint(struct {
		ContactID       string
		OwnerID         *int64
		CollaboratorIDs []int64
		Version         int64
	}{command.ContactID, command.OwnerID, collaborators, command.Version})
	replayed, _, err := claimIdempotency(ctx, tx, command.TenantID, command.CorpID, "assignment.update", command.IdempotencyKey, fingerprint, command.ContactID)
	if err != nil {
		return domain.CustomerAssignment{}, err
	}
	if replayed {
		return loadAssignmentTx(ctx, tx, command.TenantID, command.CorpID, command.ContactID)
	}
	employees := append([]int64(nil), command.CollaboratorIDs...)
	if command.OwnerID != nil {
		employees = append(employees, *command.OwnerID)
	}
	if err := employeesInScopeTx(ctx, tx, command.TenantID, command.CorpID, employees); err != nil {
		return domain.CustomerAssignment{}, err
	}
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

func (r *CustomerLifecycleRepository) ReleaseToPublicPool(ctx context.Context, tenantID, corpID int64, contactID string, version int64, key string) (domain.CustomerAssignment, error) {
	return r.transition(ctx, tenantID, corpID, contactID, version, key, "assignment.release", `status<>?`, domain.AssignmentPublicPool, domain.AssignmentPublicPool, nil)
}

func (r *CustomerLifecycleRepository) ClaimFromPublicPool(ctx context.Context, tenantID, corpID int64, contactID string, userID, version int64, key string) (domain.CustomerAssignment, error) {
	return r.transition(ctx, tenantID, corpID, contactID, version, key, "assignment.claim", `status=? AND owner_id IS NULL`, domain.AssignmentPublicPool, domain.AssignmentOwned, userID)
}

func (r *CustomerLifecycleRepository) transition(ctx context.Context, tenantID, corpID int64, contactID string, version int64, key, action, extra, expectedStatus, status string, owner any) (domain.CustomerAssignment, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.CustomerAssignment{}, err
	}
	defer tx.Rollback()
	fingerprint := requestFingerprint(struct {
		ContactID string
		Version   int64
		Owner     any
	}{contactID, version, owner})
	replayed, _, err := claimIdempotency(ctx, tx, tenantID, corpID, action, key, fingerprint, contactID)
	if err != nil {
		return domain.CustomerAssignment{}, err
	}
	if replayed {
		return loadAssignmentTx(ctx, tx, tenantID, corpID, contactID)
	}
	if value, ok := owner.(int64); ok {
		if err := employeesInScopeTx(ctx, tx, tenantID, corpID, []int64{value}); err != nil {
			return domain.CustomerAssignment{}, err
		}
	}
	args := []any{owner, status, time.Now().UTC(), tenantID, corpID, contactID, version}
	query := `UPDATE mochat_go_scrm_assignments SET owner_id=?, status=?, version=version+1, updated_at=? WHERE tenant_id=? AND corp_id=? AND contact_id=? AND version=? AND deleted_at IS NULL AND ` + extra
	if extra == `status=? AND owner_id IS NULL` || extra == `status<>?` {
		args = append(args, expectedStatus)
	}
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return domain.CustomerAssignment{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return domain.CustomerAssignment{}, ports.ErrAssignmentConflict
	}
	item, err := loadAssignmentTx(ctx, tx, tenantID, corpID, contactID)
	if err != nil {
		return item, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CustomerAssignment{}, err
	}
	return item, nil
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
