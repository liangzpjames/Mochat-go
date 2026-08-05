package http

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"strings"
	"time"
)

type SQLAcceptanceStore struct{ db *sql.DB }

func NewSQLAcceptanceStore(db *sql.DB) (*SQLAcceptanceStore, error) {
	if db == nil {
		return nil, fmt.Errorf("acceptance database is required")
	}
	return &SQLAcceptanceStore{db: db}, nil
}
func acceptanceID() string {
	return AcceptancePrefix + strings.ReplaceAll(uuid.NewString(), "-", "")[:23]
}

func (s *SQLAcceptanceStore) Create(ctx context.Context, scope AcceptanceScope) (result AcceptanceResult, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	var existing int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_phase35_acceptance_resources WHERE tenant_id=? AND corp_id=? AND environment_id=? AND deleted_at IS NULL`, scope.TenantID, scope.CorpID, scope.EnvironmentID).Scan(&existing); err != nil {
		return result, err
	}
	if existing > 0 {
		return result, fmt.Errorf("acceptance resources already exist")
	}
	now := time.Now().UTC()
	contactID, orderID, settingID := acceptanceID(), acceptanceID(), acceptanceID()
	if _, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_scrm_contacts (id,tenant_id,corp_id,name,phone,version,created_at,updated_at) VALUES (?,?,?,?,?,1,?,?)`, contactID, scope.TenantID, scope.CorpID, scope.EnvironmentID+"-CONTACT", "", now, now); err != nil {
		return result, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_scrm_orders (id,tenant_id,corp_id,contact_id,amount_cents,currency,status,version,idempotency_key,created_by,created_at,updated_at) VALUES (?,?,?,?,?,'CNY','pending',1,?,?,?,?)`, orderID, scope.TenantID, scope.CorpID, contactID, 3500, orderID, scope.ActorID, now, now); err != nil {
		return result, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_scrm_settings (id,tenant_id,corp_id,setting_type,setting_key,label,value_json,enabled,version,created_by,updated_by,created_at,updated_at) VALUES (?,?,?,'customer_source',?,?,?,1,1,?,?,?,?)`, settingID, scope.TenantID, scope.CorpID, scope.EnvironmentID+"-SETTING", scope.EnvironmentID+" 验收来源", `{"acceptance":true}`, scope.ActorID, scope.ActorID, now, now); err != nil {
		return result, err
	}
	resources := []struct{ kind, id string }{{"contact", contactID}, {"order", orderID}, {"setting", settingID}}
	for _, resource := range resources {
		if _, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_phase35_acceptance_resources (id,tenant_id,corp_id,environment_id,resource_type,resource_id,created_by,created_at) VALUES (?,?,?,?,?,?,?,?)`, uuid.NewString(), scope.TenantID, scope.CorpID, scope.EnvironmentID, resource.kind, resource.id, scope.ActorID, now); err != nil {
			return result, err
		}
	}
	if err = s.audit(ctx, tx, scope, "create", len(resources), now); err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	ids := []string{contactID, orderID, settingID}
	return AcceptanceResult{Prefix: AcceptancePrefix, EnvironmentID: scope.EnvironmentID, ResourceIDs: ids, Count: len(ids)}, nil
}

func (s *SQLAcceptanceStore) Verify(ctx context.Context, scope AcceptanceScope) (AcceptanceResult, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT resource_id FROM mochat_go_phase35_acceptance_resources WHERE tenant_id=? AND corp_id=? AND environment_id=? AND deleted_at IS NULL ORDER BY created_at,id`, scope.TenantID, scope.CorpID, scope.EnvironmentID)
	if err != nil {
		return AcceptanceResult{}, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return AcceptanceResult{}, err
		}
		if !strings.HasPrefix(id, AcceptancePrefix) {
			return AcceptanceResult{}, fmt.Errorf("unsafe acceptance resource")
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return AcceptanceResult{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO mochat_go_phase35_acceptance_audit (id,tenant_id,corp_id,environment_id,action,resource_count,actor_id,created_at) VALUES (?,?,?,?,?,?,?,?)`, uuid.NewString(), scope.TenantID, scope.CorpID, scope.EnvironmentID, "verify", len(ids), scope.ActorID, time.Now().UTC())
	if err != nil {
		return AcceptanceResult{}, err
	}
	return AcceptanceResult{Prefix: AcceptancePrefix, EnvironmentID: scope.EnvironmentID, ResourceIDs: ids, Count: len(ids)}, nil
}

func (s *SQLAcceptanceStore) Cleanup(ctx context.Context, scope AcceptanceScope) (result AcceptanceResult, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	rows, err := tx.QueryContext(ctx, `SELECT resource_type,resource_id FROM mochat_go_phase35_acceptance_resources WHERE tenant_id=? AND corp_id=? AND environment_id=? AND deleted_at IS NULL FOR UPDATE`, scope.TenantID, scope.CorpID, scope.EnvironmentID)
	if err != nil {
		return result, err
	}
	type resource struct{ kind, id string }
	resources := []resource{}
	for rows.Next() {
		var r resource
		if err = rows.Scan(&r.kind, &r.id); err != nil {
			_ = rows.Close()
			return result, err
		}
		if !strings.HasPrefix(r.id, AcceptancePrefix) {
			_ = rows.Close()
			return result, fmt.Errorf("unsafe cleanup resource")
		}
		resources = append(resources, r)
	}
	if err = rows.Close(); err != nil {
		return result, err
	}
	now := time.Now().UTC()
	for _, r := range resources {
		var query string
		switch r.kind {
		case "order":
			query = `UPDATE mochat_go_scrm_orders SET deleted_at=?,updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL`
		case "setting":
			query = `UPDATE mochat_go_scrm_settings SET deleted_at=?,updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL`
		case "contact":
			query = `UPDATE mochat_go_scrm_contacts SET deleted_at=?,updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL`
		default:
			return result, fmt.Errorf("unsafe resource type")
		}
		if _, err = tx.ExecContext(ctx, query, now, now, r.id, scope.TenantID, scope.CorpID); err != nil {
			return result, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE mochat_go_phase35_acceptance_resources SET deleted_at=?,deleted_by=? WHERE tenant_id=? AND corp_id=? AND environment_id=? AND deleted_at IS NULL`, now, scope.ActorID, scope.TenantID, scope.CorpID, scope.EnvironmentID); err != nil {
		return result, err
	}
	if err = s.audit(ctx, tx, scope, "cleanup", len(resources), now); err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	return AcceptanceResult{Prefix: AcceptancePrefix, EnvironmentID: scope.EnvironmentID, Count: len(resources)}, nil
}
func (s *SQLAcceptanceStore) audit(ctx context.Context, tx *sql.Tx, scope AcceptanceScope, action string, count int, now time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_phase35_acceptance_audit (id,tenant_id,corp_id,environment_id,action,resource_count,actor_id,created_at) VALUES (?,?,?,?,?,?,?,?)`, uuid.NewString(), scope.TenantID, scope.CorpID, scope.EnvironmentID, action, count, scope.ActorID, now)
	return err
}
