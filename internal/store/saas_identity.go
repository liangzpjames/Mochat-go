package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	mysqldriver "github.com/go-sql-driver/mysql"

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

type saasBootstrapRow struct {
	identity   saasauth.SaaSIdentity
	requestKey string
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
	requestKey := strings.TrimSpace(input.RequestKey)
	loginName := strings.ToLower(strings.TrimSpace(input.LoginName))
	phone := strings.TrimSpace(input.Phone)
	name := strings.TrimSpace(input.Name)
	if store == nil || requestKey == "" || loginName == "" || name == "" || strings.TrimSpace(input.PasswordHash) == "" {
		return saasauth.SaaSIdentity{}, saasauth.ErrInvalidBootstrap
	}
	normalized := input
	normalized.RequestKey = requestKey
	normalized.LoginName = loginName
	normalized.Phone = phone
	normalized.Name = name
	for attempt := 0; attempt < 3; attempt++ {
		identity, err := store.bootstrapOnce(ctx, normalized)
		if err == nil || !isRetryableBootstrapError(err) || ctx.Err() != nil || attempt == 2 {
			return identity, err
		}
	}
	return saasauth.SaaSIdentity{}, saasauth.ErrIdentityUnavailable
}

func (store *SaaSIdentityStore) bootstrapOnce(ctx context.Context, input saasauth.BootstrapSaaSAdmin) (saasauth.SaaSIdentity, error) {
	requestKey := input.RequestKey
	loginName := input.LoginName
	phone := input.Phone
	name := input.Name
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
		       status, must_rotate_password, auth_version, mfa_required,
		       COALESCE(bootstrap_request_key, '')
		FROM mochat_go_saas_admin_users
		WHERE bootstrap_request_key = ?
		LIMIT 1
		FOR UPDATE
	`, requestKey)
	existing, lookupErr := scanSaaSBootstrapRow(row)
	if lookupErr == nil {
		if existing.requestKey != requestKey || !sameBootstrapBusinessInput(existing.identity, loginName, phone, name) {
			return saasauth.SaaSIdentity{}, saasauth.ErrBootstrapConflict
		}
		if err := tx.Commit(); err != nil {
			return saasauth.SaaSIdentity{}, err
		}
		return existing.identity, nil
	}
	if !errors.Is(lookupErr, saasauth.ErrIdentityNotFound) {
		return saasauth.SaaSIdentity{}, lookupErr
	}

	row = tx.QueryRowContext(ctx, `
		SELECT id, login_name, COALESCE(phone, ''), password_hash, name,
		       status, must_rotate_password, auth_version, mfa_required,
		       COALESCE(bootstrap_request_key, '')
		FROM mochat_go_saas_admin_users
		ORDER BY id
		LIMIT 1
		FOR UPDATE
	`)
	existing, lookupErr = scanSaaSBootstrapRow(row)
	if lookupErr == nil {
		if existing.requestKey != requestKey || !sameBootstrapBusinessInput(existing.identity, loginName, phone, name) {
			return saasauth.SaaSIdentity{}, saasauth.ErrBootstrapConflict
		}
		if err := tx.Commit(); err != nil {
			return saasauth.SaaSIdentity{}, err
		}
		return existing.identity, nil
	}
	if !errors.Is(lookupErr, saasauth.ErrIdentityNotFound) {
		return saasauth.SaaSIdentity{}, lookupErr
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_admin_users
			(login_name, phone, password_hash, name, status, must_rotate_password, auth_version, mfa_required, bootstrap_request_key, created_at, updated_at)
		VALUES (?, NULLIF(?, ''), ?, ?, 1, 1, 1, 1, ?, NOW(), NOW())
	`, loginName, phone, input.PasswordHash, name, requestKey)
	if err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	id, err := result.LastInsertId()
	if err != nil || id <= 0 {
		return saasauth.SaaSIdentity{}, saasauth.ErrInvalidBootstrap
	}
	identity := saasauth.SaaSIdentity{
		ID:                 int(id),
		LoginName:          loginName,
		Phone:              phone,
		PasswordHash:       input.PasswordHash,
		Name:               name,
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

func isRetryableBootstrapError(err error) bool {
	var mysqlErr *mysqldriver.MySQLError
	if !errors.As(err, &mysqlErr) {
		return false
	}
	return mysqlErr.Number == 1062 || mysqlErr.Number == 1205 || mysqlErr.Number == 1213
}

func sameBootstrapBusinessInput(identity saasauth.SaaSIdentity, loginName, phone, name string) bool {
	return strings.ToLower(strings.TrimSpace(identity.LoginName)) == loginName &&
		strings.TrimSpace(identity.Phone) == phone &&
		strings.TrimSpace(identity.Name) == name
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

func scanSaaSBootstrapRow(row identityRowScanner) (saasBootstrapRow, error) {
	var result saasBootstrapRow
	err := row.Scan(
		&result.identity.ID,
		&result.identity.LoginName,
		&result.identity.Phone,
		&result.identity.PasswordHash,
		&result.identity.Name,
		&result.identity.Status,
		&result.identity.MustRotatePassword,
		&result.identity.AuthVersion,
		&result.identity.MFARequired,
		&result.requestKey,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return saasBootstrapRow{}, saasauth.ErrIdentityNotFound
	}
	if err != nil {
		return saasBootstrapRow{}, err
	}
	return result, nil
}
