package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"jiyi/mochat-go/internal/modules/scrm/domain"
)

// SQLOrderRepository persists Phase 3.5 orders in migration 0119 tables.
// Scope is always part of every statement; callers must provide the authenticated tenant.
type SQLOrderRepository struct{ db *sql.DB }

func NewSQLOrderRepository(db *sql.DB) (*SQLOrderRepository, error) {
	if db == nil {
		return nil, errors.New("order database is required")
	}
	return &SQLOrderRepository{db: db}, nil
}

func (r *SQLOrderRepository) CreateContext(ctx context.Context, order domain.Order, actorID int64) (domain.Order, error) {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `INSERT INTO mochat_go_scrm_orders (id,tenant_id,corp_id,contact_id,opportunity_id,amount_cents,currency,status,version,idempotency_key,created_by,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, order.ID, order.TenantID, order.CorpID, order.ContactID, order.OpportunityID, order.AmountCents, order.Currency, order.Status, order.Version, order.ID, actorID, now, now)
	if err != nil {
		return domain.Order{}, err
	}
	r.audit(ctx, order, "created", 0, order.Version, actorID)
	return order, nil
}

func (r *SQLOrderRepository) ListContext(ctx context.Context, tenantID, corpID int64) ([]domain.Order, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,tenant_id,corp_id,contact_id,COALESCE(opportunity_id,''),amount_cents,currency,status,version FROM mochat_go_scrm_orders WHERE tenant_id=? AND corp_id=? AND deleted_at IS NULL ORDER BY updated_at DESC`, tenantID, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Order{}
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(&o.ID, &o.TenantID, &o.CorpID, &o.ContactID, &o.OpportunityID, &o.AmountCents, &o.Currency, &o.Status, &o.Version); err != nil {
			return nil, err
		}
		items = append(items, o)
	}
	return items, rows.Err()
}

func (r *SQLOrderRepository) TransitionContext(ctx context.Context, id string, tenantID, corpID int64, status domain.OrderStatus, version, actorID int64) (domain.Order, error) {
	var o domain.Order
	err := r.db.QueryRowContext(ctx, `SELECT id,tenant_id,corp_id,contact_id,COALESCE(opportunity_id,''),amount_cents,currency,status,version FROM mochat_go_scrm_orders WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL FOR UPDATE`, id, tenantID, corpID).Scan(&o.ID, &o.TenantID, &o.CorpID, &o.ContactID, &o.OpportunityID, &o.AmountCents, &o.Currency, &o.Status, &o.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Order{}, sql.ErrNoRows
	}
	if err != nil {
		return domain.Order{}, err
	}
	from := o.Version
	if err := o.Transition(status, version); err != nil {
		return domain.Order{}, err
	}
	res, err := r.db.ExecContext(ctx, `UPDATE mochat_go_scrm_orders SET status=?,version=?,updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND version=? AND deleted_at IS NULL`, o.Status, o.Version, time.Now().UTC(), id, tenantID, corpID, version)
	if err != nil {
		return domain.Order{}, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return domain.Order{}, domain.ErrOrderVersionConflict
	}
	r.audit(ctx, o, "transition", from, o.Version, actorID)
	return o, nil
}

func (r *SQLOrderRepository) audit(ctx context.Context, o domain.Order, action string, from, to, actor int64) {
	_, _ = r.db.ExecContext(ctx, `INSERT INTO mochat_go_scrm_order_audit (id,tenant_id,corp_id,order_id,action,actor_id,from_version,to_version,payload_json,created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`, uuid.NewString(), o.TenantID, o.CorpID, o.ID, action, actor, from, to, `{}`, time.Now().UTC())
}

func (r *SQLOrderRepository) Create(o domain.Order) (domain.Order, error) {
	return r.CreateContext(context.Background(), o, 0)
}
func (r *SQLOrderRepository) List(t, c int64) []domain.Order {
	v, _ := r.ListContext(context.Background(), t, c)
	return v
}
func (r *SQLOrderRepository) Transition(id string, t int64, s domain.OrderStatus, v int64) (domain.Order, error) {
	return r.TransitionContext(context.Background(), id, t, 0, s, v, 0)
}
