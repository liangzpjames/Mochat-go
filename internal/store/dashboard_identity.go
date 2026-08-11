package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"jiyi/mochat-go/internal/dashboardauth"
)

type dashboardIdentityQueryRowFunc func(context.Context, string, ...any) identityRowScanner

type dashboardIdentityTx interface {
	QueryRowContext(context.Context, string, ...any) identityRowScanner
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	Commit() error
	Rollback() error
}

type dashboardIdentityBeginFunc func(context.Context) (dashboardIdentityTx, error)

type sqlDashboardIdentityTx struct{ tx *sql.Tx }

func (tx sqlDashboardIdentityTx) QueryRowContext(ctx context.Context, query string, args ...any) identityRowScanner {
	return tx.tx.QueryRowContext(ctx, query, args...)
}

func (tx sqlDashboardIdentityTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.tx.ExecContext(ctx, query, args...)
}

func (tx sqlDashboardIdentityTx) Commit() error   { return tx.tx.Commit() }
func (tx sqlDashboardIdentityTx) Rollback() error { return tx.tx.Rollback() }

type DashboardIdentityStore struct {
	db       *sql.DB
	queryRow dashboardIdentityQueryRowFunc
	begin    dashboardIdentityBeginFunc
}

var _ dashboardauth.DashboardIdentityStore = (*DashboardIdentityStore)(nil)

func NewDashboardIdentityStore(db *sql.DB) *DashboardIdentityStore {
	store := &DashboardIdentityStore{db: db}
	if db != nil {
		store.queryRow = func(ctx context.Context, query string, args ...any) identityRowScanner {
			return db.QueryRowContext(ctx, query, args...)
		}
		store.begin = func(ctx context.Context) (dashboardIdentityTx, error) {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return nil, err
			}
			return sqlDashboardIdentityTx{tx: tx}, nil
		}
	}
	return store
}

func (store *DashboardIdentityStore) Authenticate(ctx context.Context, loginIdentifier string) (dashboardauth.DashboardIdentity, error) {
	if store == nil || strings.TrimSpace(loginIdentifier) == "" {
		return dashboardauth.DashboardIdentity{}, dashboardauth.ErrIdentityNotFound
	}
	row, err := store.query(ctx, `
		SELECT user_id, login_identifier, password_hash, status,
		       must_rotate_password, auth_version, mfa_required
		FROM mochat_go_dashboard_identities
		WHERE login_identifier = ?
		LIMIT 1
	`, strings.TrimSpace(loginIdentifier))
	if err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	return scanDashboardIdentity(row)
}

func (store *DashboardIdentityStore) CheckSession(ctx context.Context, userID int, authVersion uint64) error {
	if store == nil || userID <= 0 || authVersion == 0 {
		return dashboardauth.ErrSessionInvalid
	}
	row, err := store.query(ctx, `
		SELECT status, auth_version
		FROM mochat_go_dashboard_identities
		WHERE user_id = ?
		LIMIT 1
	`, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dashboardauth.ErrSessionInvalid
		}
		return err
	}
	var status int
	var currentVersion uint64
	if err := row.Scan(&status, &currentVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dashboardauth.ErrSessionInvalid
		}
		return err
	}
	if status != dashboardauth.DashboardIdentityStatusActive || currentVersion != authVersion {
		return dashboardauth.ErrSessionInvalid
	}
	return nil
}

func (store *DashboardIdentityStore) Activate(ctx context.Context, tokenDigest [32]byte, passwordHash string) error {
	if store == nil || tokenDigest == ([32]byte{}) || strings.TrimSpace(passwordHash) == "" || store.begin == nil {
		return dashboardauth.ErrActivationInvalid
	}
	tx, err := store.begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT user_id
		FROM mochat_go_dashboard_identity_activations
		WHERE token_digest = ? AND consumed_at IS NULL AND expires_at > NOW()
		LIMIT 1
		FOR UPDATE
	`, tokenDigest[:])
	var userID int
	if err := row.Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dashboardauth.ErrActivationInvalid
		}
		return err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_dashboard_identities
		SET password_hash = ?, must_rotate_password = 0,
			auth_version = auth_version + 1,
			activated_at = COALESCE(activated_at, NOW()), updated_at = NOW()
		WHERE user_id = ? AND status = 1
	`, passwordHash, userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return dashboardauth.ErrActivationInvalid
	}
	result, err = tx.ExecContext(ctx, `
		UPDATE mochat_go_dashboard_identity_activations
		SET consumed_at = NOW()
		WHERE token_digest = ? AND consumed_at IS NULL
	`, tokenDigest[:])
	if err != nil {
		return err
	}
	affected, err = result.RowsAffected()
	if err != nil || affected != 1 {
		return dashboardauth.ErrActivationInvalid
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (store *DashboardIdentityStore) query(ctx context.Context, query string, args ...any) (identityRowScanner, error) {
	if store == nil {
		return nil, errors.New("dashboard identity store is unavailable")
	}
	if store.queryRow != nil {
		return store.queryRow(ctx, query, args...), nil
	}
	if store.db == nil {
		return nil, errors.New("dashboard identity store is unavailable")
	}
	return store.db.QueryRowContext(ctx, query, args...), nil
}

func scanDashboardIdentity(row identityRowScanner) (dashboardauth.DashboardIdentity, error) {
	var identity dashboardauth.DashboardIdentity
	err := row.Scan(
		&identity.UserID,
		&identity.LoginIdentifier,
		&identity.PasswordHash,
		&identity.Status,
		&identity.MustRotatePassword,
		&identity.AuthVersion,
		&identity.MFARequired,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboardauth.DashboardIdentity{}, dashboardauth.ErrIdentityNotFound
	}
	if err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	return identity, nil
}
