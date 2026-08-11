package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"jiyi/mochat-go/internal/saasauth"
)

type identityRowScanner interface {
	Scan(dest ...any) error
}

type saasIdentityQueryRowFunc func(context.Context, string, ...any) identityRowScanner

type saasIdentityTx interface {
	QueryRowContext(context.Context, string, ...any) identityRowScanner
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	Commit() error
	Rollback() error
}

type saasIdentityBeginFunc func(context.Context) (saasIdentityTx, error)

type sqlSaaSIdentityTx struct{ tx *sql.Tx }

func (tx sqlSaaSIdentityTx) QueryRowContext(ctx context.Context, query string, args ...any) identityRowScanner {
	return tx.tx.QueryRowContext(ctx, query, args...)
}

func (tx sqlSaaSIdentityTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.tx.ExecContext(ctx, query, args...)
}

func (tx sqlSaaSIdentityTx) Commit() error   { return tx.tx.Commit() }
func (tx sqlSaaSIdentityTx) Rollback() error { return tx.tx.Rollback() }

type SaaSIdentityStore struct {
	db       *sql.DB
	queryRow saasIdentityQueryRowFunc
	begin    saasIdentityBeginFunc
}

var _ saasauth.SaaSIdentityStore = (*SaaSIdentityStore)(nil)

func NewSaaSIdentityStore(db *sql.DB) *SaaSIdentityStore {
	store := &SaaSIdentityStore{db: db}
	if db != nil {
		store.queryRow = func(ctx context.Context, query string, args ...any) identityRowScanner {
			return db.QueryRowContext(ctx, query, args...)
		}
		store.begin = func(ctx context.Context) (saasIdentityTx, error) {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return nil, err
			}
			return sqlSaaSIdentityTx{tx: tx}, nil
		}
	}
	return store
}

func (store *SaaSIdentityStore) Authenticate(ctx context.Context, login string) (saasauth.SaaSIdentity, error) {
	if store == nil || strings.TrimSpace(login) == "" {
		return saasauth.SaaSIdentity{}, saasauth.ErrIdentityNotFound
	}
	row, err := store.query(ctx, `
		SELECT id, login_name, COALESCE(phone, ''), password_hash, name,
		       status, must_rotate_password, auth_version, mfa_required
		FROM mochat_go_saas_admin_users
		WHERE login_name = ? OR phone = ?
		LIMIT 1
	`, login, login)
	if err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	return scanSaaSIdentity(row)
}

func (store *SaaSIdentityStore) CheckSession(ctx context.Context, userID int, authVersion uint64) error {
	if store == nil || userID <= 0 || authVersion == 0 {
		return saasauth.ErrSessionInvalid
	}
	row, err := store.query(ctx, `
		SELECT status, auth_version
		FROM mochat_go_saas_admin_users
		WHERE id = ?
		LIMIT 1
	`, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return saasauth.ErrSessionInvalid
		}
		return err
	}
	var status int
	var currentVersion uint64
	if err := row.Scan(&status, &currentVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return saasauth.ErrSessionInvalid
		}
		return err
	}
	if status != saasauth.SaaSIdentityStatusActive || currentVersion != authVersion {
		return saasauth.ErrSessionInvalid
	}
	return nil
}

func (store *SaaSIdentityStore) Bootstrap(ctx context.Context, input saasauth.BootstrapSaaSAdmin) (saasauth.SaaSIdentity, error) {
	if store == nil || strings.TrimSpace(input.RequestKey) == "" || strings.TrimSpace(input.LoginName) == "" || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.PasswordHash) == "" {
		return saasauth.SaaSIdentity{}, saasauth.ErrInvalidBootstrap
	}
	if store.begin == nil {
		return saasauth.SaaSIdentity{}, errors.New("saas identity store is unavailable")
	}
	tx, err := store.begin(ctx)
	if err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT id, login_name, COALESCE(phone, ''), password_hash, name,
		       status, must_rotate_password, auth_version, mfa_required
		FROM mochat_go_saas_admin_users
		WHERE login_name = ?
		LIMIT 1
		FOR UPDATE
	`, strings.ToLower(strings.TrimSpace(input.LoginName)))
	existing, lookupErr := scanSaaSIdentity(row)
	if lookupErr == nil {
		if err := tx.Commit(); err != nil {
			return saasauth.SaaSIdentity{}, err
		}
		return existing, nil
	}
	if !errors.Is(lookupErr, saasauth.ErrIdentityNotFound) {
		return saasauth.SaaSIdentity{}, lookupErr
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_admin_users
			(login_name, phone, password_hash, name, status, must_rotate_password, auth_version, mfa_required, created_at, updated_at)
		VALUES (?, NULLIF(?, ''), ?, ?, 1, 1, 1, 1, NOW(), NOW())
	`, strings.ToLower(strings.TrimSpace(input.LoginName)), strings.TrimSpace(input.Phone), input.PasswordHash, strings.TrimSpace(input.Name))
	if err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	id, err := result.LastInsertId()
	if err != nil || id <= 0 {
		return saasauth.SaaSIdentity{}, saasauth.ErrInvalidBootstrap
	}
	identity := saasauth.SaaSIdentity{
		ID:                 int(id),
		LoginName:          strings.ToLower(strings.TrimSpace(input.LoginName)),
		Phone:              strings.TrimSpace(input.Phone),
		PasswordHash:       input.PasswordHash,
		Name:               strings.TrimSpace(input.Name),
		Status:             saasauth.SaaSIdentityStatusActive,
		MustRotatePassword: 1,
		AuthVersion:        1,
		MFARequired:        1,
	}
	if err := tx.Commit(); err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	return identity, nil
}

func (store *SaaSIdentityStore) query(ctx context.Context, query string, args ...any) (identityRowScanner, error) {
	if store == nil {
		return nil, errors.New("saas identity store is unavailable")
	}
	if store.queryRow != nil {
		return store.queryRow(ctx, query, args...), nil
	}
	if store.db == nil {
		return nil, errors.New("saas identity store is unavailable")
	}
	return store.db.QueryRowContext(ctx, query, args...), nil
}

func scanSaaSIdentity(row identityRowScanner) (saasauth.SaaSIdentity, error) {
	var identity saasauth.SaaSIdentity
	err := row.Scan(
		&identity.ID,
		&identity.LoginName,
		&identity.Phone,
		&identity.PasswordHash,
		&identity.Name,
		&identity.Status,
		&identity.MustRotatePassword,
		&identity.AuthVersion,
		&identity.MFARequired,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return saasauth.SaaSIdentity{}, saasauth.ErrIdentityNotFound
	}
	if err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	return identity, nil
}
