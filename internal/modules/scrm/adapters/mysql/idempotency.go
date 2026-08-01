package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func requestFingerprint(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func claimIdempotency(ctx context.Context, tx *sql.Tx, tenantID, corpID int64, action, key, fingerprint, resourceID string) (bool, string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return false, "", ports.ErrAssignmentConflict
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_scrm_idempotency_keys(tenant_id,corp_id,action,idempotency_key,request_fingerprint,resource_id,created_at) VALUES(?,?,?,?,?,?,?)`, tenantID, corpID, action, key, fingerprint, resourceID, time.Now().UTC())
	if err == nil {
		return false, resourceID, nil
	}
	var duplicate *mysqldriver.MySQLError
	if !errors.As(err, &duplicate) || duplicate.Number != 1062 {
		return false, "", err
	}
	var previousFingerprint, previousResource string
	if err := tx.QueryRowContext(ctx, `SELECT request_fingerprint,resource_id FROM mochat_go_scrm_idempotency_keys WHERE tenant_id=? AND corp_id=? AND action=? AND idempotency_key=? FOR UPDATE`, tenantID, corpID, action, key).Scan(&previousFingerprint, &previousResource); err != nil {
		return false, "", err
	}
	if previousFingerprint != fingerprint {
		return false, "", ports.ErrAssignmentConflict
	}
	return true, previousResource, nil
}

func contactExistsTx(ctx context.Context, tx *sql.Tx, tenantID, corpID int64, contactID string) error {
	var found string
	err := tx.QueryRowContext(ctx, `SELECT id FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id=? AND id=? AND deleted_at IS NULL FOR UPDATE`, tenantID, corpID, contactID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.ErrContactNotFound
	}
	return err
}

func employeesInScopeTx(ctx context.Context, tx *sql.Tx, tenantID, corpID int64, ids []int64) error {
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id <= 0 {
			return ports.ErrAssignmentForbidden
		}
		seen[id] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	args := []any{tenantID, corpID}
	for id := range seen {
		args = append(args, id)
	}
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(DISTINCT e.id) FROM mc_work_employee e JOIN mc_corp c ON c.id=e.corp_id AND c.deleted_at IS NULL WHERE c.tenant_id=? AND e.corp_id=? AND e.status=1 AND e.deleted_at IS NULL AND e.id IN (`+sqlPlaceholders(len(seen))+`)`, args...).Scan(&count)
	if err != nil {
		return err
	}
	if count != len(seen) {
		return ports.ErrAssignmentForbidden
	}
	return nil
}
