package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"strings"
	"time"

	"jiyi/mochat-go/internal/authrealm"
	"jiyi/mochat-go/internal/dashboardauth"
	"jiyi/mochat-go/internal/dashboardprincipal"
)

type dashboardIdentityQueryRowFunc func(context.Context, string, ...any) identityRowScanner

type dashboardIdentityTx interface {
	QueryRowContext(context.Context, string, ...any) identityRowScanner
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	Commit() error
	Rollback() error
}

type dashboardIdentityBeginFunc func(context.Context) (dashboardIdentityTx, error)

type dashboardIdentityExecFunc func(context.Context, string, ...any) (sql.Result, error)

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
	exec     dashboardIdentityExecFunc
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
		store.exec = func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return db.ExecContext(ctx, query, args...)
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

func (store *DashboardIdentityStore) DashboardActivationStatus(ctx context.Context, tokenDigest [32]byte, now time.Time) (dashboardauth.DashboardActivationStatus, error) {
	invalid := dashboardauth.DashboardActivationStatus{Status: dashboardauth.ActivationStatusInvalid, PrimaryAction: dashboardauth.ActivationPrimaryActionContactAdmin}
	if store == nil || tokenDigest == ([32]byte{}) {
		return invalid, nil
	}
	row, err := store.query(ctx, `
		SELECT COALESCE(tenant.name, ''), identity_row.login_identifier,
		       activation.expires_at, activation.consumed_at, identity_row.activated_at,
		       identity_row.status, dashboard_user.status, tenant.status
		FROM mochat_go_dashboard_identity_activations activation
		INNER JOIN mochat_go_dashboard_identities identity_row ON identity_row.user_id = activation.user_id
		INNER JOIN mc_user dashboard_user ON dashboard_user.id = identity_row.user_id AND dashboard_user.deleted_at IS NULL
		INNER JOIN mc_tenant tenant ON tenant.id = dashboard_user.tenant_id AND tenant.deleted_at IS NULL
		WHERE activation.token_digest = ?
		LIMIT 1
	`, tokenDigest[:])
	if err != nil {
		return dashboardauth.DashboardActivationStatus{}, err
	}
	var tenantName, loginIdentifier string
	var expiresAt time.Time
	var consumedAt, activatedAt sql.NullTime
	var identityStatus, userStatus, tenantStatus int
	if err := row.Scan(&tenantName, &loginIdentifier, &expiresAt, &consumedAt, &activatedAt, &identityStatus, &userStatus, &tenantStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return invalid, nil
		}
		return dashboardauth.DashboardActivationStatus{}, err
	}
	result := dashboardauth.DashboardActivationStatus{TenantName: strings.TrimSpace(tenantName), AccountHint: maskDashboardAccount(loginIdentifier), ExpiresAt: expiresAt.Unix()}
	switch {
	case identityStatus != dashboardauth.DashboardIdentityStatusActive || userStatus != 1 || tenantStatus != 1:
		result.Status, result.PrimaryAction = dashboardauth.ActivationStatusRevoked, dashboardauth.ActivationPrimaryActionContactAdmin
	case consumedAt.Valid && activatedAt.Valid:
		result.Status, result.PrimaryAction = dashboardauth.ActivationStatusActivated, dashboardauth.ActivationPrimaryActionLogin
	case consumedAt.Valid:
		result.Status, result.PrimaryAction = dashboardauth.ActivationStatusRevoked, dashboardauth.ActivationPrimaryActionContactAdmin
	case !expiresAt.After(now):
		result.Status, result.PrimaryAction = dashboardauth.ActivationStatusExpired, dashboardauth.ActivationPrimaryActionContactAdmin
	default:
		result.Status, result.PrimaryAction = dashboardauth.ActivationStatusValid, dashboardauth.ActivationPrimaryActionActivate
	}
	return result, nil
}

func maskDashboardAccount(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}
	if len(runes) >= 7 {
		return string(runes[:3]) + "****" + string(runes[len(runes)-4:])
	}
	return string(runes[:1]) + strings.Repeat("*", len(runes)-2) + string(runes[len(runes)-1:])
}

// ResolveIdentity returns only the authenticated Dashboard identity facts
// needed by the principal resolver. Corp ownership is deliberately resolved
// by TenantCorpBindingStore after the SaaS tenant gate, not by this query.
func (store *DashboardIdentityStore) ResolveIdentity(ctx context.Context, userID int) (dashboardprincipal.AuthenticatedIdentity, error) {
	if store == nil || userID <= 0 {
		return dashboardprincipal.AuthenticatedIdentity{}, dashboardprincipal.ErrPrincipalUnavailable
	}
	row, err := store.query(ctx, `
		SELECT d.user_id, u.tenant_id, COALESCE(u.isSuperAdmin, 0),
		       u.status, d.auth_version
		FROM mochat_go_dashboard_identities d
		INNER JOIN mc_user u ON u.id = d.user_id AND u.deleted_at IS NULL
		WHERE d.user_id = ? AND d.status = 1
		LIMIT 1
	`, userID)
	if err != nil {
		return dashboardprincipal.AuthenticatedIdentity{}, dashboardprincipal.ErrPrincipalUnavailable
	}
	var identity dashboardprincipal.AuthenticatedIdentity
	var isSuperAdmin, userStatus int
	if err := row.Scan(&identity.UserID, &identity.TenantID, &isSuperAdmin, &userStatus, &identity.AuthVersion); err != nil {
		return dashboardprincipal.AuthenticatedIdentity{}, dashboardprincipal.ErrPrincipalUnavailable
	}
	identity.IsSuperAdmin = isSuperAdmin == 1
	identity.Active = userStatus == 1 && identity.UserID == userID && identity.TenantID > 0 && identity.AuthVersion > 0
	if !identity.Active {
		return dashboardprincipal.AuthenticatedIdentity{}, dashboardprincipal.ErrPrincipalUnavailable
	}
	return identity, nil
}

func (store *DashboardIdentityStore) MFAStatus(ctx context.Context, userID int) (int, error) {
	if store == nil || userID <= 0 {
		return dashboardauth.DashboardMFAStatusPending, dashboardauth.ErrMFAChallengeInvalid
	}
	row, err := store.query(ctx, `
		SELECT status
		FROM mochat_go_dashboard_mfa_credentials
		WHERE user_id = ?
		LIMIT 1
	`, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dashboardauth.DashboardMFAStatusPending, nil
		}
		return dashboardauth.DashboardMFAStatusPending, err
	}
	var status int
	if err := row.Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dashboardauth.DashboardMFAStatusPending, nil
		}
		return dashboardauth.DashboardMFAStatusPending, err
	}
	return status, nil
}

func (store *DashboardIdentityStore) BeginMFAEnrollment(ctx context.Context, userID int, authVersion uint64, tokenDigest [32]byte, expiresAt time.Time, secretCiphertext, keyID string) error {
	if store == nil || userID <= 0 || authVersion == 0 || tokenDigest == ([32]byte{}) || expiresAt.IsZero() || strings.TrimSpace(secretCiphertext) == "" || strings.TrimSpace(keyID) == "" || store.begin == nil {
		return dashboardauth.ErrMFAChallengeInvalid
	}
	tx, err := store.begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status int
	var currentVersion uint64
	if err := tx.QueryRowContext(ctx, `
		SELECT status, auth_version
		FROM mochat_go_dashboard_identities
		WHERE user_id = ?
		FOR UPDATE
	`, userID).Scan(&status, &currentVersion); err != nil || status != dashboardauth.DashboardIdentityStatusActive || currentVersion != authVersion {
		return dashboardauth.ErrMFAChallengeInvalid
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_dashboard_mfa_credentials
			(user_id, status, secret_ciphertext, encryption_key_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			status = IF(status = ?, status, ?),
			secret_ciphertext = IF(status = ?, secret_ciphertext, VALUES(secret_ciphertext)),
			encryption_key_id = IF(status = ?, encryption_key_id, VALUES(encryption_key_id)),
			last_totp_step = IF(status = ?, last_totp_step, NULL), updated_at = NOW()
	`, userID, dashboardauth.DashboardMFAStatusPending, secretCiphertext, keyID,
		dashboardauth.DashboardMFAStatusActive, dashboardauth.DashboardMFAStatusPending,
		dashboardauth.DashboardMFAStatusActive, dashboardauth.DashboardMFAStatusActive,
		dashboardauth.DashboardMFAStatusActive); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_dashboard_mfa_challenges
			(token_digest, user_id, auth_version, challenge_type, status, attempts, max_attempts, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, 0, 5, ?, NOW(), NOW())
	`, tokenDigest[:], userID, authVersion, dashboardauth.DashboardMFAChallengeEnrollment, expiresAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (store *DashboardIdentityStore) CreateMFAChallenge(ctx context.Context, userID int, authVersion uint64, challengeType string, tokenDigest [32]byte, expiresAt time.Time) error {
	if store == nil || userID <= 0 || authVersion == 0 || tokenDigest == ([32]byte{}) || expiresAt.IsZero() || !validDashboardMFAChallengeType(challengeType) || store.begin == nil {
		return dashboardauth.ErrMFAChallengeInvalid
	}
	tx, err := store.begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status int
	var currentVersion uint64
	if err := tx.QueryRowContext(ctx, `
		SELECT status, auth_version
		FROM mochat_go_dashboard_identities
		WHERE user_id = ?
		FOR UPDATE
	`, userID).Scan(&status, &currentVersion); err != nil || status != dashboardauth.DashboardIdentityStatusActive || currentVersion != authVersion {
		return dashboardauth.ErrMFAChallengeInvalid
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_dashboard_mfa_challenges
			(token_digest, user_id, auth_version, challenge_type, status, attempts, max_attempts, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, 0, 5, ?, NOW(), NOW())
	`, tokenDigest[:], userID, authVersion, challengeType, expiresAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (store *DashboardIdentityStore) FindMFAChallenge(ctx context.Context, tokenDigest [32]byte) (dashboardauth.DashboardMFAChallenge, error) {
	if store == nil || tokenDigest == ([32]byte{}) {
		return dashboardauth.DashboardMFAChallenge{}, dashboardauth.ErrMFAChallengeInvalid
	}
	row, err := store.query(ctx, `
		SELECT c.user_id, c.auth_version, c.challenge_type, c.status, c.attempts, c.max_attempts,
		       c.expires_at, COALESCE(m.secret_ciphertext, ''), COALESCE(m.encryption_key_id, '')
		FROM mochat_go_dashboard_mfa_challenges c
		LEFT JOIN mochat_go_dashboard_mfa_credentials m ON m.user_id = c.user_id
		WHERE c.token_digest = ?
		LIMIT 1
	`, tokenDigest[:])
	if err != nil {
		return dashboardauth.DashboardMFAChallenge{}, dashboardauth.ErrMFAChallengeInvalid
	}
	return scanDashboardMFAChallenge(row)
}

func (store *DashboardIdentityStore) RecordMFAFailure(ctx context.Context, tokenDigest [32]byte) error {
	if store == nil || tokenDigest == ([32]byte{}) {
		return dashboardauth.ErrMFAChallengeInvalid
	}
	exec := store.exec
	if exec == nil && store.db != nil {
		exec = func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return store.db.ExecContext(ctx, query, args...)
		}
	}
	if exec == nil {
		return dashboardauth.ErrMFAChallengeInvalid
	}
	_, err := exec(ctx, `
		UPDATE mochat_go_dashboard_mfa_challenges
		SET status = IF(attempts + 1 >= max_attempts, 2, status),
			attempts = attempts + 1, updated_at = NOW()
		WHERE token_digest = ? AND status = 0 AND expires_at > NOW() AND attempts < max_attempts
	`, tokenDigest[:])
	return err
}

func (store *DashboardIdentityStore) CompleteMFAChallenge(ctx context.Context, tokenDigest [32]byte, userID int, authVersion uint64, challengeType string, totpStep int64) (dashboardauth.DashboardIdentity, error) {
	if store == nil || userID <= 0 || authVersion == 0 || tokenDigest == ([32]byte{}) || totpStep <= 0 || !validDashboardMFAChallengeType(challengeType) || challengeType == dashboardauth.DashboardMFAChallengePasswordChange || store.begin == nil {
		return dashboardauth.DashboardIdentity{}, dashboardauth.ErrMFAChallengeInvalid
	}
	tx, err := store.begin(ctx)
	if err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	defer func() { _ = tx.Rollback() }()
	challenge, err := scanDashboardMFAChallenge(tx.QueryRowContext(ctx, `
		SELECT user_id, auth_version, challenge_type, status, attempts, max_attempts, expires_at, '', ''
		FROM mochat_go_dashboard_mfa_challenges
		WHERE token_digest = ? AND user_id = ? AND auth_version = ?
			AND challenge_type = ? AND status = 0 AND expires_at > NOW()
		FOR UPDATE
	`, tokenDigest[:], userID, authVersion, challengeType))
	if err != nil || challenge.Status != dashboardauth.DashboardMFAStatusPending || challenge.Attempts >= challenge.MaxAttempts {
		return dashboardauth.DashboardIdentity{}, dashboardauth.ErrMFAChallengeInvalid
	}
	credentialStatusPredicate := "status = 1"
	if challengeType == dashboardauth.DashboardMFAChallengeEnrollment {
		credentialStatusPredicate = "status = 0"
	}
	credentialUpdateQuery := `
		UPDATE mochat_go_dashboard_mfa_credentials
		SET status = 1, last_totp_step = ?, verified_at = IF(? = ?, NOW(), verified_at), updated_at = NOW()
		WHERE user_id = ? AND ` + credentialStatusPredicate + ` AND (last_totp_step IS NULL OR last_totp_step < ?)
	`
	credentialUpdate, err := tx.ExecContext(ctx, credentialUpdateQuery, totpStep, challengeType, dashboardauth.DashboardMFAChallengeEnrollment, userID, totpStep)
	if err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	if affected, err := credentialUpdate.RowsAffected(); err != nil || affected != 1 {
		return dashboardauth.DashboardIdentity{}, dashboardauth.ErrMFAChallengeInvalid
	}
	challengeUpdate, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_dashboard_mfa_challenges
		SET status = 1, consumed_at = NOW(), updated_at = NOW()
		WHERE token_digest = ? AND status = 0
	`, tokenDigest[:])
	if err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	if affected, err := challengeUpdate.RowsAffected(); err != nil || affected != 1 {
		return dashboardauth.DashboardIdentity{}, dashboardauth.ErrMFAChallengeInvalid
	}
	identity, err := scanDashboardIdentity(tx.QueryRowContext(ctx, `
		SELECT user_id, login_identifier, password_hash, status,
		       must_rotate_password, auth_version, mfa_required
		FROM mochat_go_dashboard_identities
		WHERE user_id = ? AND status = 1 AND auth_version = ?
		LIMIT 1
	`, userID, authVersion))
	if err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	return identity, nil
}

func (store *DashboardIdentityStore) CompletePasswordChange(ctx context.Context, tokenDigest [32]byte, passwordHash string) (dashboardauth.DashboardIdentity, error) {
	if store == nil || tokenDigest == ([32]byte{}) || strings.TrimSpace(passwordHash) == "" || store.begin == nil {
		return dashboardauth.DashboardIdentity{}, dashboardauth.ErrInvalidPassword
	}
	tx, err := store.begin(ctx)
	if err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var userID int
	var authVersion uint64
	if err := tx.QueryRowContext(ctx, `
		SELECT user_id, auth_version
		FROM mochat_go_dashboard_mfa_challenges
		WHERE token_digest = ? AND challenge_type = ? AND status = 0 AND expires_at > NOW()
		FOR UPDATE
	`, tokenDigest[:], dashboardauth.DashboardMFAChallengePasswordChange).Scan(&userID, &authVersion); err != nil {
		return dashboardauth.DashboardIdentity{}, dashboardauth.ErrInvalidPassword
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_dashboard_identities
		SET password_hash = ?, must_rotate_password = 0,
			auth_version = auth_version + 1, updated_at = NOW()
		WHERE user_id = ? AND auth_version = ? AND status = 1
	`, passwordHash, userID, authVersion)
	if err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return dashboardauth.DashboardIdentity{}, dashboardauth.ErrInvalidPassword
	}
	challengeUpdate, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_dashboard_mfa_challenges
		SET status = 1, consumed_at = NOW(), updated_at = NOW()
		WHERE token_digest = ? AND status = 0
	`, tokenDigest[:])
	if err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	if affected, err := challengeUpdate.RowsAffected(); err != nil || affected != 1 {
		return dashboardauth.DashboardIdentity{}, dashboardauth.ErrInvalidPassword
	}
	identity, err := scanDashboardIdentity(tx.QueryRowContext(ctx, `
		SELECT user_id, login_identifier, password_hash, status,
		       must_rotate_password, auth_version, mfa_required
		FROM mochat_go_dashboard_identities
		WHERE user_id = ? AND status = 1
		LIMIT 1
	`, userID))
	if err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	return identity, nil
}

func (store *DashboardIdentityStore) CreateSession(ctx context.Context, userID int, authVersion uint64, jtiDigest [32]byte, issuedAt, expiresAt time.Time) error {
	if store == nil || userID <= 0 || authVersion == 0 || jtiDigest == ([32]byte{}) || issuedAt.IsZero() || expiresAt.IsZero() || !expiresAt.After(issuedAt) || store.begin == nil {
		return dashboardauth.ErrSessionInvalid
	}
	tx, err := store.begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status int
	var currentVersion uint64
	if err := tx.QueryRowContext(ctx, `
		SELECT status, auth_version
		FROM mochat_go_dashboard_identities
		WHERE user_id = ?
		FOR UPDATE
	`, userID).Scan(&status, &currentVersion); err != nil || status != dashboardauth.DashboardIdentityStatusActive || currentVersion != authVersion {
		return dashboardauth.ErrSessionInvalid
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_dashboard_sessions
			(jti_digest, user_id, auth_version, status, issued_at, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?, NOW(), NOW())
	`, jtiDigest[:], userID, authVersion, issuedAt, expiresAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (store *DashboardIdentityStore) CheckSessionToken(ctx context.Context, claims authrealm.Claims) error {
	if store == nil || claims.UserID <= 0 || claims.AuthVersion == 0 || strings.TrimSpace(claims.JWTID) == "" {
		return dashboardauth.ErrSessionInvalid
	}
	digest := sha256.Sum256([]byte(claims.JWTID))
	row, err := store.query(ctx, `
		SELECT s.status, s.auth_version, d.status, d.auth_version
		FROM mochat_go_dashboard_sessions s
		INNER JOIN mochat_go_dashboard_identities d ON d.user_id = s.user_id
		WHERE s.jti_digest = ? AND s.user_id = ? AND s.auth_version = ?
			AND s.status = 1 AND s.expires_at > NOW()
		LIMIT 1
	`, digest[:], claims.UserID, claims.AuthVersion)
	if err != nil {
		return dashboardauth.ErrSessionInvalid
	}
	var sessionStatus, identityStatus int
	var sessionVersion, identityVersion uint64
	if err := row.Scan(&sessionStatus, &sessionVersion, &identityStatus, &identityVersion); err != nil || sessionStatus != 1 || identityStatus != dashboardauth.DashboardIdentityStatusActive || sessionVersion != claims.AuthVersion || identityVersion != claims.AuthVersion {
		return dashboardauth.ErrSessionInvalid
	}
	return nil
}

func (store *DashboardIdentityStore) RevokeSession(ctx context.Context, claims authrealm.Claims) error {
	if store == nil || claims.UserID <= 0 || claims.AuthVersion == 0 || strings.TrimSpace(claims.JWTID) == "" {
		return dashboardauth.ErrSessionInvalid
	}
	digest := sha256.Sum256([]byte(claims.JWTID))
	exec := store.exec
	if exec == nil && store.db != nil {
		exec = func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return store.db.ExecContext(ctx, query, args...)
		}
	}
	if exec == nil {
		return dashboardauth.ErrSessionInvalid
	}
	result, err := exec(ctx, `
		UPDATE mochat_go_dashboard_sessions
		SET status = 2, revoked_at = NOW(), updated_at = NOW()
		WHERE jti_digest = ? AND user_id = ? AND auth_version = ? AND status = 1
	`, digest[:], claims.UserID, claims.AuthVersion)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return dashboardauth.ErrSessionInvalid
	}
	return nil
}

func (store *DashboardIdentityStore) CreatePasswordReset(ctx context.Context, userID int, authVersion uint64, tokenDigest [32]byte, expiresAt time.Time) error {
	if store == nil || userID <= 0 || authVersion == 0 || tokenDigest == ([32]byte{}) || expiresAt.IsZero() || store.begin == nil {
		return dashboardauth.ErrSessionInvalid
	}
	tx, err := store.begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status int
	var currentVersion uint64
	if err := tx.QueryRowContext(ctx, `
		SELECT status, auth_version
		FROM mochat_go_dashboard_identities
		WHERE user_id = ?
		FOR UPDATE
	`, userID).Scan(&status, &currentVersion); err != nil || status != dashboardauth.DashboardIdentityStatusActive || currentVersion != authVersion {
		return dashboardauth.ErrSessionInvalid
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mochat_go_dashboard_password_resets
			(token_digest, user_id, auth_version, status, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, 0, ?, NOW(), NOW())
	`, tokenDigest[:], userID, authVersion, expiresAt)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (store *DashboardIdentityStore) CompletePasswordReset(ctx context.Context, tokenDigest [32]byte, passwordHash string) (dashboardauth.DashboardIdentity, error) {
	if store == nil || tokenDigest == ([32]byte{}) || strings.TrimSpace(passwordHash) == "" || store.begin == nil {
		return dashboardauth.DashboardIdentity{}, dashboardauth.ErrInvalidPassword
	}
	tx, err := store.begin(ctx)
	if err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var userID int
	var authVersion uint64
	if err := tx.QueryRowContext(ctx, `
		SELECT user_id, auth_version
		FROM mochat_go_dashboard_password_resets
		WHERE token_digest = ? AND status = 0 AND consumed_at IS NULL AND expires_at > NOW()
		FOR UPDATE
	`, tokenDigest[:]).Scan(&userID, &authVersion); err != nil {
		return dashboardauth.DashboardIdentity{}, dashboardauth.ErrInvalidPassword
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_dashboard_identities
		SET password_hash = ?, auth_version = auth_version + 1, updated_at = NOW()
		WHERE user_id = ? AND auth_version = ? AND status = 1
	`, passwordHash, userID, authVersion)
	if err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return dashboardauth.DashboardIdentity{}, dashboardauth.ErrInvalidPassword
	}
	resetUpdate, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_dashboard_password_resets
		SET status = 1, consumed_at = NOW(), updated_at = NOW()
		WHERE token_digest = ? AND status = 0 AND consumed_at IS NULL
	`, tokenDigest[:])
	if err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	if affected, err := resetUpdate.RowsAffected(); err != nil || affected != 1 {
		return dashboardauth.DashboardIdentity{}, dashboardauth.ErrInvalidPassword
	}
	identity, err := scanDashboardIdentity(tx.QueryRowContext(ctx, `
		SELECT user_id, login_identifier, password_hash, status,
		       must_rotate_password, auth_version, mfa_required
		FROM mochat_go_dashboard_identities
		WHERE user_id = ? AND status = 1
		LIMIT 1
	`, userID))
	if err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboardauth.DashboardIdentity{}, err
	}
	return identity, nil
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

func scanDashboardMFAChallenge(row identityRowScanner) (dashboardauth.DashboardMFAChallenge, error) {
	var challenge dashboardauth.DashboardMFAChallenge
	err := row.Scan(
		&challenge.UserID, &challenge.AuthVersion, &challenge.ChallengeType,
		&challenge.Status, &challenge.Attempts, &challenge.MaxAttempts,
		&challenge.ExpiresAt, &challenge.SecretCiphertext, &challenge.EncryptionKeyID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboardauth.DashboardMFAChallenge{}, dashboardauth.ErrMFAChallengeInvalid
	}
	if err != nil {
		return dashboardauth.DashboardMFAChallenge{}, err
	}
	return challenge, nil
}

func validDashboardMFAChallengeType(value string) bool {
	switch value {
	case dashboardauth.DashboardMFAChallengeEnrollment,
		dashboardauth.DashboardMFAChallengeLogin,
		dashboardauth.DashboardMFAChallengePasswordChange:
		return true
	default:
		return false
	}
}
