package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

type OpportunityRepository struct{ db *sql.DB }

func NewOpportunityRepository(db *sql.DB) (*OpportunityRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("opportunity database is required")
	}
	return &OpportunityRepository{db: db}, nil
}

func (r *OpportunityRepository) ListOpportunities(ctx context.Context, filter ports.OpportunityFilter) ([]ports.Opportunity, error) {
	query := `SELECT id,contact_id,stage_id,status,lost_reason,version,amount,start_date,end_date FROM mochat_go_scrm_opportunities WHERE tenant_id=? AND corp_id=? AND deleted_at IS NULL`
	args := []any{filter.TenantID, filter.CorpID}
	if filter.Stage != "" {
		query += " AND stage_id=?"
		args = append(args, filter.Stage)
	}
	if filter.OwnerID != nil {
		query += " AND owner_id=?"
		args = append(args, *filter.OwnerID)
	}
	query += " ORDER BY updated_at DESC, id DESC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ports.Opportunity{}
	for rows.Next() {
		var item ports.Opportunity
		if err := rows.Scan(&item.ID, &item.ContactID, &item.Stage, &item.Status, &item.LostReason, &item.Version, &item.Amount, &item.StartDate, &item.EndDate); err != nil {
			return nil, err
		}
		item.TenantID, item.CorpID = filter.TenantID, filter.CorpID
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *OpportunityRepository) CreateOpportunity(ctx context.Context, c ports.CreateOpportunityCommand) (ports.Opportunity, error) {
	start, err := time.Parse("2006-01-02", c.StartDate)
	if err != nil {
		return ports.Opportunity{}, err
	}
	end, err := time.Parse("2006-01-02", c.EndDate)
	if err != nil {
		return ports.Opportunity{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.Opportunity{}, err
	}
	defer tx.Rollback()
	if err := contactExistsTx(ctx, tx, c.TenantID, c.CorpID, c.ContactID); err != nil {
		return ports.Opportunity{}, err
	}
	if c.OwnerID > 0 {
		if err := employeesInScopeTx(ctx, tx, c.TenantID, c.CorpID, []int64{c.OwnerID}); err != nil {
			return ports.Opportunity{}, err
		}
	}
	id := uuid.NewString()
	fingerprint := requestFingerprint(struct {
		ContactID, Stage, StartDate, EndDate string
		OwnerID                              int64
		Amount                               float64
	}{strings.TrimSpace(c.ContactID), strings.TrimSpace(c.Stage), c.StartDate, c.EndDate, c.OwnerID, c.Amount})
	replayed, resourceID, err := claimIdempotency(ctx, tx, c.TenantID, c.CorpID, "opportunity.create", c.IdempotencyKey, fingerprint, id)
	if err != nil {
		return ports.Opportunity{}, err
	}
	if replayed {
		item, err := getOpportunityWith(ctx, tx, c.TenantID, c.CorpID, resourceID, false)
		if err != nil {
			return ports.Opportunity{}, err
		}
		if err := tx.Commit(); err != nil {
			return ports.Opportunity{}, err
		}
		return item, nil
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_scrm_opportunities (id,tenant_id,corp_id,contact_id,stage_id,status,owner_id,version,amount,start_date,end_date,created_at,updated_at) VALUES (?,?,?,?,?,'open',?,1,?,?,?,?,?)`, id, c.TenantID, c.CorpID, c.ContactID, c.Stage, nullablePositiveID(c.OwnerID), c.Amount, start, end, now, now); err != nil {
		return ports.Opportunity{}, err
	}
	item, err := getOpportunityWith(ctx, tx, c.TenantID, c.CorpID, id, false)
	if err != nil {
		return ports.Opportunity{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.Opportunity{}, err
	}
	return item, nil
}

func (r *OpportunityRepository) ChangeOpportunityStage(ctx context.Context, c ports.ChangeOpportunityStageCommand) (ports.Opportunity, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.Opportunity{}, err
	}
	defer tx.Rollback()
	if _, err := getOpportunityWith(ctx, tx, c.TenantID, c.CorpID, c.OpportunityID, true); err != nil {
		return ports.Opportunity{}, err
	}
	fingerprint := requestFingerprint(struct {
		OpportunityID, ToStage, Reason string
		Version                        int64
	}{c.OpportunityID, c.ToStage, strings.TrimSpace(c.Reason), c.Version})
	replayed, resourceID, err := claimIdempotency(ctx, tx, c.TenantID, c.CorpID, "opportunity.stage", c.IdempotencyKey, fingerprint, c.OpportunityID)
	if err != nil {
		return ports.Opportunity{}, err
	}
	if !replayed {
		status := "open"
		if c.ToStage == domain.OpportunityStatusWon || c.ToStage == domain.OpportunityStatusLost {
			status = c.ToStage
		}
		result, err := tx.ExecContext(ctx, `UPDATE mochat_go_scrm_opportunities SET stage_id=?,status=?,lost_reason=?,version=version+1,updated_at=? WHERE tenant_id=? AND corp_id=? AND id=? AND version=? AND deleted_at IS NULL AND status NOT IN ('won','lost')`, c.ToStage, status, strings.TrimSpace(c.Reason), time.Now().UTC(), c.TenantID, c.CorpID, c.OpportunityID, c.Version)
		if err != nil {
			return ports.Opportunity{}, err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return ports.Opportunity{}, ports.ErrAssignmentConflict
		}
	}
	item, err := getOpportunityWith(ctx, tx, c.TenantID, c.CorpID, resourceID, false)
	if err != nil {
		return ports.Opportunity{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.Opportunity{}, err
	}
	return item, nil
}

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getOpportunityWith(ctx context.Context, query rowQuerier, tenant, corp int64, id string, lock bool) (ports.Opportunity, error) {
	statement := `SELECT id,contact_id,stage_id,status,lost_reason,version,amount,start_date,end_date FROM mochat_go_scrm_opportunities WHERE tenant_id=? AND corp_id=? AND id=? AND deleted_at IS NULL`
	if lock {
		statement += " FOR UPDATE"
	}
	var item ports.Opportunity
	err := query.QueryRowContext(ctx, statement, tenant, corp, id).Scan(&item.ID, &item.ContactID, &item.Stage, &item.Status, &item.LostReason, &item.Version, &item.Amount, &item.StartDate, &item.EndDate)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ports.ErrOpportunityNotFound
	}
	item.TenantID, item.CorpID = tenant, corp
	return item, err
}

func (r *OpportunityRepository) ListFollowUps(ctx context.Context, tenant, corp int64, contact string) ([]ports.FollowUpRecord, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,content,created_by,created_at FROM mochat_go_scrm_follow_ups WHERE tenant_id=? AND corp_id=? AND contact_id=? ORDER BY created_at,id`, tenant, corp, contact)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ports.FollowUpRecord{}
	for rows.Next() {
		var item ports.FollowUpRecord
		if err := rows.Scan(&item.ID, &item.Content, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.TenantID, item.CorpID, item.ContactID = tenant, corp, contact
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *OpportunityRepository) AppendFollowUp(ctx context.Context, c ports.AppendFollowUpCommand) (ports.FollowUpRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.FollowUpRecord{}, err
	}
	defer tx.Rollback()
	if err := contactExistsTx(ctx, tx, c.TenantID, c.CorpID, c.ContactID); err != nil {
		return ports.FollowUpRecord{}, err
	}
	id := uuid.NewString()
	fingerprint := requestFingerprint(struct {
		ContactID, Content string
		CreatedBy          int64
	}{c.ContactID, strings.TrimSpace(c.Content), c.CreatedBy})
	replayed, resourceID, err := claimIdempotency(ctx, tx, c.TenantID, c.CorpID, "follow-up.append", c.IdempotencyKey, fingerprint, id)
	if err != nil {
		return ports.FollowUpRecord{}, err
	}
	if replayed {
		item, err := getFollowUpWith(ctx, tx, c.TenantID, c.CorpID, resourceID)
		if err != nil {
			return ports.FollowUpRecord{}, err
		}
		if err := tx.Commit(); err != nil {
			return ports.FollowUpRecord{}, err
		}
		return item, nil
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_scrm_follow_ups (id,tenant_id,corp_id,contact_id,content,created_by,created_at) VALUES (?,?,?,?,?,?,?)`, id, c.TenantID, c.CorpID, c.ContactID, strings.TrimSpace(c.Content), c.CreatedBy, now); err != nil {
		return ports.FollowUpRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.FollowUpRecord{}, err
	}
	return ports.FollowUpRecord{ID: id, TenantID: c.TenantID, CorpID: c.CorpID, ContactID: c.ContactID, Content: strings.TrimSpace(c.Content), CreatedBy: c.CreatedBy, CreatedAt: now}, nil
}

func getFollowUpWith(ctx context.Context, query rowQuerier, tenant, corp int64, id string) (ports.FollowUpRecord, error) {
	var item ports.FollowUpRecord
	err := query.QueryRowContext(ctx, `SELECT id,contact_id,content,created_by,created_at FROM mochat_go_scrm_follow_ups WHERE tenant_id=? AND corp_id=? AND id=?`, tenant, corp, id).Scan(&item.ID, &item.ContactID, &item.Content, &item.CreatedBy, &item.CreatedAt)
	item.TenantID, item.CorpID = tenant, corp
	return item, err
}

type TagRepository struct{ db *sql.DB }

func NewTagRepository(db *sql.DB) (*TagRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("tag database is required")
	}
	return &TagRepository{db: db}, nil
}

func (r *TagRepository) ListTags(ctx context.Context, tenant, corp int64) ([]ports.Tag, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,version FROM mochat_go_scrm_tags WHERE tenant_id=? AND corp_id=? AND deleted_at IS NULL ORDER BY name`, tenant, corp)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ports.Tag{}
	for rows.Next() {
		var tag ports.Tag
		tag.TenantID, tag.CorpID = tenant, corp
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Version); err != nil {
			return nil, err
		}
		items = append(items, tag)
	}
	return items, rows.Err()
}

func (r *TagRepository) CreateTag(ctx context.Context, tenant, corp int64, name, key string) (ports.Tag, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.Tag{}, err
	}
	defer tx.Rollback()
	if err := corpExistsTx(ctx, tx, tenant, corp); err != nil {
		return ports.Tag{}, err
	}
	id := uuid.NewString()
	name = strings.TrimSpace(name)
	replayed, resourceID, err := claimIdempotency(ctx, tx, tenant, corp, "tag.create", key, requestFingerprint(struct{ Name string }{name}), id)
	if err != nil {
		return ports.Tag{}, err
	}
	if !replayed {
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_scrm_tags(id,tenant_id,corp_id,name,version,created_at,updated_at) VALUES(?,?,?,?,1,?,?)`, id, tenant, corp, name, now, now); err != nil {
			return ports.Tag{}, err
		}
	}
	item, err := getTagWith(ctx, tx, tenant, corp, resourceID, false)
	if err != nil {
		return ports.Tag{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.Tag{}, err
	}
	return item, nil
}

func (r *TagRepository) RenameTag(ctx context.Context, tenant, corp int64, id, name string, version int64, key string) (ports.Tag, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.Tag{}, err
	}
	defer tx.Rollback()
	if _, err := getTagWith(ctx, tx, tenant, corp, id, true); err != nil {
		return ports.Tag{}, err
	}
	name = strings.TrimSpace(name)
	fingerprint := requestFingerprint(struct {
		ID, Name string
		Version  int64
	}{id, name, version})
	replayed, resourceID, err := claimIdempotency(ctx, tx, tenant, corp, "tag.rename", key, fingerprint, id)
	if err != nil {
		return ports.Tag{}, err
	}
	if !replayed {
		result, err := tx.ExecContext(ctx, `UPDATE mochat_go_scrm_tags SET name=?,version=version+1,updated_at=? WHERE tenant_id=? AND corp_id=? AND id=? AND version=? AND deleted_at IS NULL`, name, time.Now().UTC(), tenant, corp, id, version)
		if err != nil {
			return ports.Tag{}, err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return ports.Tag{}, ports.ErrAssignmentConflict
		}
	}
	item, err := getTagWith(ctx, tx, tenant, corp, resourceID, false)
	if err != nil {
		return ports.Tag{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.Tag{}, err
	}
	return item, nil
}

func (r *TagRepository) BindTags(ctx context.Context, tenant, corp int64, id string, contacts []string, key string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := getTagWith(ctx, tx, tenant, corp, id, true); err != nil {
		return err
	}
	contacts = normalizedContactIDs(contacts)
	for _, contactID := range contacts {
		if err := contactExistsTx(ctx, tx, tenant, corp, contactID); err != nil {
			return err
		}
	}
	fingerprint := requestFingerprint(struct {
		TagID      string
		ContactIDs []string
	}{id, contacts})
	replayed, _, err := claimIdempotency(ctx, tx, tenant, corp, "tag.bind", key, fingerprint, id)
	if err != nil {
		return err
	}
	if !replayed {
		for _, contactID := range contacts {
			if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO mochat_go_scrm_contact_tags(tenant_id,corp_id,contact_id,tag_id,created_at) VALUES(?,?,?,?,?)`, tenant, corp, contactID, id, time.Now().UTC()); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func getTagWith(ctx context.Context, query rowQuerier, tenant, corp int64, id string, lock bool) (ports.Tag, error) {
	statement := `SELECT id,name,version FROM mochat_go_scrm_tags WHERE tenant_id=? AND corp_id=? AND id=? AND deleted_at IS NULL`
	if lock {
		statement += " FOR UPDATE"
	}
	item := ports.Tag{TenantID: tenant, CorpID: corp}
	err := query.QueryRowContext(ctx, statement, tenant, corp, id).Scan(&item.ID, &item.Name, &item.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.Tag{}, ports.ErrTagNotFound
	}
	return item, err
}

func corpExistsTx(ctx context.Context, tx *sql.Tx, tenant, corp int64) error {
	var found int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM mc_corp WHERE tenant_id=? AND id=? AND deleted_at IS NULL FOR UPDATE`, tenant, corp).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.ErrContactNotFound
	}
	return err
}

func normalizedContactIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			seen[id] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func nullablePositiveID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}
