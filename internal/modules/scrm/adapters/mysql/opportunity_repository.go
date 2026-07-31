package mysql

import (
	"context"
	"database/sql"
	"fmt"
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
	query := `SELECT id,contact_id,stage,status,lost_reason,owner_id,version,amount,start_date,end_date FROM mochat_go_scrm_opportunities WHERE tenant_id=? AND corp_id=? AND deleted_at IS NULL`
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
		if err := rows.Scan(&item.ID, &item.ContactID, &item.Stage, &item.Status, &item.LostReason, &item.OwnerID, &item.Version, &item.Amount, &item.StartDate, &item.EndDate); err != nil {
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
	id := uuid.NewString()
	_, err = r.db.ExecContext(ctx, `INSERT INTO mochat_go_scrm_opportunities (id,tenant_id,corp_id,contact_id,stage_id,status,version,amount,start_date,end_date,created_at,updated_at) VALUES (?,?,?,?,?,'open',1,?,?,?,?,?)`, id, c.TenantID, c.CorpID, c.ContactID, c.Stage, c.Amount, start, end, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		return ports.Opportunity{}, err
	}
	return r.getOpportunity(ctx, c.TenantID, c.CorpID, id)
}

func (r *OpportunityRepository) ChangeOpportunityStage(ctx context.Context, c ports.ChangeOpportunityStageCommand) (ports.Opportunity, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE mochat_go_scrm_opportunities SET stage_id=?,status=?,lost_reason=?,version=version+1,updated_at=? WHERE tenant_id=? AND corp_id=? AND id=? AND version=? AND deleted_at IS NULL AND status NOT IN ('won','lost')`, c.ToStage, c.ToStage, c.Reason, time.Now().UTC(), c.TenantID, c.CorpID, c.OpportunityID, c.Version)
	if err != nil {
		return ports.Opportunity{}, err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return ports.Opportunity{}, ports.ErrAssignmentConflict
	}
	return r.getOpportunity(ctx, c.TenantID, c.CorpID, c.OpportunityID)
}

func (r *OpportunityRepository) getOpportunity(ctx context.Context, tenant, corp int64, id string) (ports.Opportunity, error) {
	var i ports.Opportunity
	err := r.db.QueryRowContext(ctx, `SELECT id,contact_id,stage_id,status,lost_reason,owner_id,version,amount,start_date,end_date FROM mochat_go_scrm_opportunities WHERE tenant_id=? AND corp_id=? AND id=?`, tenant, corp, id).Scan(&i.ID, &i.ContactID, &i.Stage, &i.Status, &i.LostReason, &i.OwnerID, &i.Version, &i.Amount, &i.StartDate, &i.EndDate)
	i.TenantID, i.CorpID = tenant, corp
	return i, err
}

func (r *OpportunityRepository) ListFollowUps(ctx context.Context, tenant, corp int64, contact string) ([]ports.FollowUpRecord, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,content,created_by,created_at FROM mochat_go_scrm_follow_ups WHERE tenant_id=? AND corp_id=? AND contact_id=? ORDER BY created_at,id`, tenant, corp, contact)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ports.FollowUpRecord{}
	for rows.Next() {
		var i ports.FollowUpRecord
		if err := rows.Scan(&i.ID, &i.Content, &i.CreatedBy, &i.CreatedAt); err != nil {
			return nil, err
		}
		i.TenantID, i.CorpID, i.ContactID = tenant, corp, contact
		items = append(items, i)
	}
	return items, rows.Err()
}
func (r *OpportunityRepository) AppendFollowUp(ctx context.Context, c ports.AppendFollowUpCommand) (ports.FollowUpRecord, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `INSERT INTO mochat_go_scrm_follow_ups (id,tenant_id,corp_id,contact_id,content,created_by,created_at) VALUES (?,?,?,?,?,?,?)`, id, c.TenantID, c.CorpID, c.ContactID, c.Content, c.CreatedBy, now)
	if err != nil {
		return ports.FollowUpRecord{}, err
	}
	return ports.FollowUpRecord{ID: id, TenantID: c.TenantID, CorpID: c.CorpID, ContactID: c.ContactID, Content: c.Content, CreatedBy: c.CreatedBy, CreatedAt: now}, nil
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
		var t ports.Tag
		t.TenantID, t.CorpID = tenant, corp
		if err := rows.Scan(&t.ID, &t.Name, &t.Version); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}
func (r *TagRepository) CreateTag(ctx context.Context, tenant, corp int64, name, key string) (ports.Tag, error) {
	id := uuid.NewString()
	_, err := r.db.ExecContext(ctx, `INSERT INTO mochat_go_scrm_tags(id,tenant_id,corp_id,name,version,created_at,updated_at) VALUES(?,?,?,?,1,?,?)`, id, tenant, corp, name, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		return ports.Tag{}, err
	}
	return ports.Tag{ID: id, TenantID: tenant, CorpID: corp, Name: name, Version: 1}, nil
}
func (r *TagRepository) RenameTag(ctx context.Context, tenant, corp int64, id, name string, version int64, key string) (ports.Tag, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE mochat_go_scrm_tags SET name=?,version=version+1,updated_at=? WHERE tenant_id=? AND corp_id=? AND id=? AND version=? AND deleted_at IS NULL`, name, time.Now().UTC(), tenant, corp, id, version)
	if err != nil {
		return ports.Tag{}, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return ports.Tag{}, ports.ErrAssignmentConflict
	}
	return ports.Tag{ID: id, TenantID: tenant, CorpID: corp, Name: name, Version: version + 1}, nil
}
func (r *TagRepository) BindTags(ctx context.Context, tenant, corp int64, id string, contacts []string, key string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range contacts {
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO mochat_go_scrm_contact_tags(tenant_id,corp_id,contact_id,tag_id,created_at) VALUES(?,?,?,?,?)`, tenant, corp, c, id, time.Now().UTC()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

var _ = domain.OpportunityStageProposal
