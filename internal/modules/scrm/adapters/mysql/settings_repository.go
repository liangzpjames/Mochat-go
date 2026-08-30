package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"jiyi/mochat-go/internal/modules/scrm/ports"
	"time"
)

type SCRMSetting = ports.SCRMSetting
type SQLSettingsRepository struct{ db *sql.DB }

func NewSQLSettingsRepository(db *sql.DB) (*SQLSettingsRepository, error) {
	if db == nil {
		return nil, errors.New("settings database is required")
	}
	return &SQLSettingsRepository{db: db}, nil
}
func (r *SQLSettingsRepository) List(ctx context.Context, t, c int64, typ string) ([]ports.SCRMSetting, error) {
	q := `SELECT id,tenant_id,corp_id,setting_type,setting_key,label,value_json,enabled,version,updated_by,updated_at FROM mochat_go_scrm_settings WHERE tenant_id=? AND corp_id=? AND deleted_at IS NULL`
	a := []any{t, c}
	if typ != "" {
		q += " AND setting_type=?"
		a = append(a, typ)
	}
	rows, e := r.db.QueryContext(ctx, q, a...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []ports.SCRMSetting{}
	for rows.Next() {
		var s ports.SCRMSetting
		var raw []byte
		var en int
		if e := rows.Scan(&s.ID, &s.TenantID, &s.CorpID, &s.Type, &s.Key, &s.Label, &raw, &en, &s.Version, &s.UpdatedBy, &s.UpdatedAt); e != nil {
			return nil, e
		}
		s.Enabled = en != 0
		_ = json.Unmarshal(raw, &s.Value)
		out = append(out, s)
	}
	return out, rows.Err()
}
func (r *SQLSettingsRepository) Upsert(ctx context.Context, s ports.SCRMSetting, actor int64) (ports.SCRMSetting, error) {
	raw, e := json.Marshal(s.Value)
	if e != nil {
		return s, e
	}
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	if s.Version <= 0 {
		s.Version = 1
	}
	now := time.Now().UTC()
	s.UpdatedBy = actor
	s.UpdatedAt = now
	_, e = r.db.ExecContext(ctx, `INSERT INTO mochat_go_scrm_settings (id,tenant_id,corp_id,setting_type,setting_key,label,value_json,enabled,version,created_by,updated_by,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE value_json=VALUES(value_json),label=VALUES(label),enabled=VALUES(enabled),version=version+1,updated_by=VALUES(updated_by),updated_at=VALUES(updated_at)`, s.ID, s.TenantID, s.CorpID, s.Type, s.Key, s.Label, raw, s.Enabled, s.Version, actor, actor, now, now)
	if e == nil {
		// 0119 provides the auditable SCRM event stream; retain setting changes there
		// until a dedicated settings-audit table is introduced.
		_, _ = r.db.ExecContext(ctx, `INSERT INTO mochat_go_scrm_order_audit (id,tenant_id,corp_id,order_id,action,actor_id,from_version,to_version,payload_json,created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`, uuid.NewString(), s.TenantID, s.CorpID, s.ID, "setting.updated", actor, s.Version-1, s.Version, raw, now)
	}
	return s, e
}
