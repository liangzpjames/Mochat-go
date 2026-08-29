package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
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

func (r *SQLOrderRepository) CreateIdempotentContext(ctx context.Context, command domain.OrderCreateCommand) (domain.OrderCreateReceipt, error) {
	if command.Order.TenantID <= 0 || command.Order.CorpID <= 0 || command.ActorID <= 0 || command.IdempotencyKey == "" || len(command.IdempotencyKey) > 128 || len(command.RequestHash) != 64 || command.ResponseStatus < 200 || command.ResponseStatus > 599 || len(command.ResponseBody) == 0 {
		return domain.OrderCreateReceipt{}, errors.New("invalid idempotent order create command")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.OrderCreateReceipt{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_scrm_order_idempotency_receipts (tenant_id,corp_id,idempotency_key,request_hash,order_id,response_status,response_body,created_at) VALUES (?,?,?,?,?,?,?,?)`, command.Order.TenantID, command.Order.CorpID, command.IdempotencyKey, command.RequestHash, command.Order.ID, command.ResponseStatus, command.ResponseBody, now)
	if err != nil {
		if !isMySQLDuplicateKey(err) {
			return domain.OrderCreateReceipt{}, fmt.Errorf("claim order idempotency receipt: %w", err)
		}
		if err := tx.Rollback(); err != nil {
			return domain.OrderCreateReceipt{}, fmt.Errorf("release duplicate order claim: %w", err)
		}
		return replayOrderReceipt(ctx, r.db, command)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_scrm_orders (id,tenant_id,corp_id,contact_id,opportunity_id,title,note,amount_cents,currency,status,version,idempotency_key,created_by,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, command.Order.ID, command.Order.TenantID, command.Order.CorpID, command.Order.ContactID, command.Order.OpportunityID, command.Order.Title, command.Order.Note, command.Order.AmountCents, command.Order.Currency, command.Order.Status, command.Order.Version, command.IdempotencyKey, command.ActorID, now, now)
	if err != nil {
		return domain.OrderCreateReceipt{}, fmt.Errorf("insert order: %w", err)
	}
	if err := r.audit(ctx, tx, command.Order, "created", 0, command.Order.Version, command.ActorID); err != nil {
		return domain.OrderCreateReceipt{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.OrderCreateReceipt{}, err
	}
	return domain.OrderCreateReceipt{OrderID: command.Order.ID, RequestHash: command.RequestHash, ResponseStatus: command.ResponseStatus, ResponseBody: append([]byte(nil), command.ResponseBody...)}, nil
}

func replayOrderReceipt(ctx context.Context, db *sql.DB, command domain.OrderCreateCommand) (domain.OrderCreateReceipt, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return domain.OrderCreateReceipt{}, err
	}
	defer tx.Rollback()
	var (
		requestHash    string
		orderID        string
		responseStatus int
		responseBody   []byte
	)
	err = tx.QueryRowContext(ctx, `SELECT request_hash,order_id,response_status,response_body FROM mochat_go_scrm_order_idempotency_receipts WHERE tenant_id=? AND corp_id=? AND idempotency_key=?`, command.Order.TenantID, command.Order.CorpID, command.IdempotencyKey).Scan(&requestHash, &orderID, &responseStatus, &responseBody)
	if err != nil {
		return domain.OrderCreateReceipt{}, fmt.Errorf("read order idempotency receipt: %w", err)
	}
	if requestHash != command.RequestHash {
		return domain.OrderCreateReceipt{}, domain.ErrOrderIdempotencyConflict
	}
	if orderID == "" || responseStatus == 0 || len(responseBody) == 0 {
		return domain.OrderCreateReceipt{}, errors.New("order idempotency receipt is incomplete")
	}
	if err := tx.Commit(); err != nil {
		return domain.OrderCreateReceipt{}, err
	}
	return domain.OrderCreateReceipt{OrderID: orderID, RequestHash: requestHash, ResponseStatus: responseStatus, ResponseBody: append([]byte(nil), responseBody...), Replayed: true}, nil
}

func isMySQLDuplicateKey(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

func (r *SQLOrderRepository) ListContext(ctx context.Context, tenantID, corpID int64, page, pageSize int) ([]domain.Order, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_scrm_orders o WHERE o.tenant_id=? AND o.corp_id=? AND o.deleted_at IS NULL`, tenantID, corpID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT o.id,o.tenant_id,o.corp_id,o.contact_id,COALESCE(c.name,''),COALESCE(o.opportunity_id,''),o.title,o.note,o.amount_cents,o.currency,o.status,o.version FROM mochat_go_scrm_orders o LEFT JOIN mochat_go_scrm_contacts c ON c.id=o.contact_id AND c.tenant_id=o.tenant_id AND c.corp_id=o.corp_id AND c.deleted_at IS NULL WHERE o.tenant_id=? AND o.corp_id=? AND o.deleted_at IS NULL ORDER BY o.updated_at DESC,o.id DESC LIMIT ? OFFSET ?`, tenantID, corpID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []domain.Order{}
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(&o.ID, &o.TenantID, &o.CorpID, &o.ContactID, &o.ContactName, &o.OpportunityID, &o.Title, &o.Note, &o.AmountCents, &o.Currency, &o.Status, &o.Version); err != nil {
			return nil, 0, err
		}
		items = append(items, o)
	}
	return items, total, rows.Err()
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
