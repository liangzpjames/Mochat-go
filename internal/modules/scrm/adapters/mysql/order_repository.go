package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"jiyi/mochat-go/internal/modules/scrm/domain"
)

// SQLOrderRepository persists Phase 3.5 orders in the 0119 and 0121 schema.
// Scope is always part of every statement; callers must provide the authenticated tenant.
type SQLOrderRepository struct {
	db           *sql.DB
	auditFailure error
}

type orderExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (r *SQLOrderRepository) GetContext(ctx context.Context, id string, tenantID, corpID int64) (domain.Order, error) {
	var o domain.Order
	err := r.db.QueryRowContext(ctx, `SELECT o.id,o.tenant_id,o.corp_id,o.contact_id,COALESCE(c.name,''),COALESCE(o.opportunity_id,''),o.title,o.note,o.amount_cents,o.currency,o.status,o.version FROM mochat_go_scrm_orders o LEFT JOIN mochat_go_scrm_contacts c ON c.id=o.contact_id AND c.tenant_id=o.tenant_id AND c.corp_id=o.corp_id AND c.deleted_at IS NULL WHERE o.id=? AND o.tenant_id=? AND o.corp_id=? AND o.deleted_at IS NULL`, id, tenantID, corpID).Scan(&o.ID, &o.TenantID, &o.CorpID, &o.ContactID, &o.ContactName, &o.OpportunityID, &o.Title, &o.Note, &o.AmountCents, &o.Currency, &o.Status, &o.Version)
	return o, err
}
func (r *SQLOrderRepository) AuditContext(ctx context.Context, id string, tenantID, corpID int64) ([]map[string]any, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT action,actor_id,from_version,to_version,created_at FROM mochat_go_scrm_order_audit WHERE order_id=? AND tenant_id=? AND corp_id=? ORDER BY created_at ASC`, id, tenantID, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var action string
		var actor, from, to int64
		var at time.Time
		if err := rows.Scan(&action, &actor, &from, &to, &at); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"action": action, "actorId": actor, "fromVersion": from, "toVersion": to, "createdAt": at})
	}
	return out, rows.Err()
}

func NewSQLOrderRepository(db *sql.DB) (*SQLOrderRepository, error) {
	if db == nil {
		return nil, errors.New("order database is required")
	}
	return &SQLOrderRepository{db: db}, nil
}

func (r *SQLOrderRepository) CreateContext(ctx context.Context, order domain.Order, actorID int64) (domain.Order, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Order{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_scrm_orders (id,tenant_id,corp_id,contact_id,opportunity_id,title,note,amount_cents,currency,status,version,idempotency_key,created_by,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, order.ID, order.TenantID, order.CorpID, order.ContactID, order.OpportunityID, order.Title, order.Note, order.AmountCents, order.Currency, order.Status, order.Version, order.ID, actorID, now, now)
	if err != nil {
		return domain.Order{}, err
	}
	if err := r.audit(ctx, tx, order, "created", 0, order.Version, actorID); err != nil {
		return domain.Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Order{}, err
	}
	return order, nil
}

func (r *SQLOrderRepository) ListContext(ctx context.Context, tenantID, corpID int64) ([]domain.Order, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT o.id,o.tenant_id,o.corp_id,o.contact_id,COALESCE(c.name,''),COALESCE(o.opportunity_id,''),o.title,o.note,o.amount_cents,o.currency,o.status,o.version FROM mochat_go_scrm_orders o LEFT JOIN mochat_go_scrm_contacts c ON c.id=o.contact_id AND c.tenant_id=o.tenant_id AND c.corp_id=o.corp_id AND c.deleted_at IS NULL WHERE o.tenant_id=? AND o.corp_id=? AND o.deleted_at IS NULL ORDER BY o.updated_at DESC`, tenantID, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Order{}
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(&o.ID, &o.TenantID, &o.CorpID, &o.ContactID, &o.ContactName, &o.OpportunityID, &o.Title, &o.Note, &o.AmountCents, &o.Currency, &o.Status, &o.Version); err != nil {
			return nil, err
		}
		items = append(items, o)
	}
	return items, rows.Err()
}

func (r *SQLOrderRepository) TransitionContext(ctx context.Context, id string, tenantID, corpID int64, status domain.OrderStatus, version, actorID int64) (domain.Order, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Order{}, err
	}
	defer tx.Rollback()
	var o domain.Order
	err = tx.QueryRowContext(ctx, `SELECT o.id,o.tenant_id,o.corp_id,o.contact_id,COALESCE(c.name,''),COALESCE(o.opportunity_id,''),o.title,o.note,o.amount_cents,o.currency,o.status,o.version FROM mochat_go_scrm_orders o LEFT JOIN mochat_go_scrm_contacts c ON c.id=o.contact_id AND c.tenant_id=o.tenant_id AND c.corp_id=o.corp_id AND c.deleted_at IS NULL WHERE o.id=? AND o.tenant_id=? AND o.corp_id=? AND o.deleted_at IS NULL FOR UPDATE`, id, tenantID, corpID).Scan(&o.ID, &o.TenantID, &o.CorpID, &o.ContactID, &o.ContactName, &o.OpportunityID, &o.Title, &o.Note, &o.AmountCents, &o.Currency, &o.Status, &o.Version)
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
	res, err := tx.ExecContext(ctx, `UPDATE mochat_go_scrm_orders SET status=?,version=?,updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND version=? AND deleted_at IS NULL`, o.Status, o.Version, time.Now().UTC(), id, tenantID, corpID, version)
	if err != nil {
		return domain.Order{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.Order{}, err
	}
	if n != 1 {
		return domain.Order{}, domain.ErrOrderVersionConflict
	}
	if err := r.audit(ctx, tx, o, "transition", from, o.Version, actorID); err != nil {
		return domain.Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Order{}, err
	}
	return o, nil
}

func (r *SQLOrderRepository) audit(ctx context.Context, executor orderExecutor, o domain.Order, action string, from, to, actor int64) error {
	if r.auditFailure != nil {
		return r.auditFailure
	}
	_, err := executor.ExecContext(ctx, `INSERT INTO mochat_go_scrm_order_audit (id,tenant_id,corp_id,order_id,action,actor_id,from_version,to_version,payload_json,created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`, uuid.NewString(), o.TenantID, o.CorpID, o.ID, action, actor, from, to, `{}`, time.Now().UTC())
	return err
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
