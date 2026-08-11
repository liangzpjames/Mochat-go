package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"strings"
	"time"

	"jiyi/mochat-go/internal/authrealm"
	"jiyi/mochat-go/internal/saasauth"
)

func (store *SaaSIdentityStore) MFAStatus(ctx context.Context, userID int) (int, error) {
	if store == nil || store.db == nil || userID <= 0 {
		return saasauth.SaaSMFAStatusPending, saasauth.ErrMFAChallengeInvalid
	}
	var status int
	err := store.db.QueryRowContext(ctx, `
		SELECT status
		FROM mochat_go_saas_admin_mfa_credentials
		WHERE user_id = ?
		LIMIT 1
	`, userID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return saasauth.SaaSMFAStatusPending, nil
	}
	if err != nil {
		return saasauth.SaaSMFAStatusPending, err
	}
	return status, nil
}

func (store *SaaSIdentityStore) BeginMFAEnrollment(ctx context.Context, userID int, authVersion uint64, tokenDigest [32]byte, expiresAt time.Time, secretCiphertext, encryptionKeyID string) error {
	if store == nil || store.db == nil || userID <= 0 || authVersion == 0 || expiresAt.IsZero() || strings.TrimSpace(secretCiphertext) == "" || strings.TrimSpace(encryptionKeyID) == "" {
		return saasauth.ErrMFAChallengeInvalid
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status int
	err = tx.QueryRowContext(ctx, `
		SELECT status
		FROM mochat_go_saas_admin_mfa_credentials
		WHERE user_id = ?
		FOR UPDATE
	`, userID).Scan(&status)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && status == saasauth.SaaSMFAStatusActive {
		return saasauth.ErrMFAChallengeInvalid
	}
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_admin_mfa_credentials
				(user_id, status, secret_ciphertext, encryption_key_id, last_totp_step, created_at, updated_at)
			VALUES (?, ?, ?, ?, 0, NOW(), NOW())
		`, userID, saasauth.SaaSMFAStatusPending, secretCiphertext, encryptionKeyID)
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_admin_mfa_credentials
			SET status = ?, secret_ciphertext = ?, encryption_key_id = ?, last_totp_step = 0,
				verified_at = NULL, updated_at = NOW()
			WHERE user_id = ? AND status <> ?
		`, saasauth.SaaSMFAStatusPending, secretCiphertext, encryptionKeyID, userID, saasauth.SaaSMFAStatusActive)
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_admin_mfa_challenges
			(token_digest, user_id, auth_version, challenge_type, status, attempts, max_attempts, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, 0, 5, ?, NOW(), NOW())
	`, tokenDigest[:], userID, authVersion, saasauth.SaaSMFAChallengeEnrollment, expiresAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (store *SaaSIdentityStore) CreateMFAChallenge(ctx context.Context, userID int, authVersion uint64, challengeType string, tokenDigest [32]byte, expiresAt time.Time) error {
	if store == nil || store.db == nil || userID <= 0 || authVersion == 0 || expiresAt.IsZero() || !validSaaSMFAChallengeType(challengeType) {
		return saasauth.ErrMFAChallengeInvalid
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status int
	var currentVersion uint64
	if err := tx.QueryRowContext(ctx, `
		SELECT status, auth_version
		FROM mochat_go_saas_admin_users
		WHERE id = ?
		FOR UPDATE
	`, userID).Scan(&status, &currentVersion); err != nil {
		return err
	}
	if status != saasauth.SaaSIdentityStatusActive || currentVersion != authVersion {
		return saasauth.ErrSessionInvalid
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_admin_mfa_challenges
			(token_digest, user_id, auth_version, challenge_type, status, attempts, max_attempts, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, 0, 5, ?, NOW(), NOW())
	`, tokenDigest[:], userID, authVersion, challengeType, expiresAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (store *SaaSIdentityStore) FindMFAChallenge(ctx context.Context, tokenDigest [32]byte) (saasauth.SaaSMFAChallenge, error) {
	if store == nil || store.db == nil {
		return saasauth.SaaSMFAChallenge{}, saasauth.ErrMFAChallengeInvalid
	}
	var challenge saasauth.SaaSMFAChallenge
	var credentialStatus sql.NullInt64
	err := store.db.QueryRowContext(ctx, `
		SELECT c.user_id, c.auth_version, c.challenge_type, c.status, c.attempts,
		       c.max_attempts, c.expires_at, COALESCE(m.status, 0),
		       COALESCE(m.secret_ciphertext, ''), COALESCE(m.encryption_key_id, '')
		FROM mochat_go_saas_admin_mfa_challenges c
		LEFT JOIN mochat_go_saas_admin_mfa_credentials m ON m.user_id = c.user_id
		WHERE c.token_digest = ?
		LIMIT 1
	`, tokenDigest[:]).Scan(
		&challenge.UserID, &challenge.AuthVersion, &challenge.ChallengeType, &challenge.Status,
		&challenge.Attempts, &challenge.MaxAttempts, &challenge.ExpiresAt, &credentialStatus,
		&challenge.SecretCiphertext, &challenge.EncryptionKeyID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return saasauth.SaaSMFAChallenge{}, saasauth.ErrMFAChallengeInvalid
	}
	if err != nil {
		return saasauth.SaaSMFAChallenge{}, err
	}
	return challenge, nil
}

func (store *SaaSIdentityStore) RecordMFAFailure(ctx context.Context, tokenDigest [32]byte) error {
	if store == nil || store.db == nil {
		return saasauth.ErrMFAChallengeInvalid
	}
	result, err := store.db.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_mfa_challenges
		SET status = IF(attempts + 1 >= max_attempts, 2, 0),
			attempts = attempts + 1, updated_at = NOW()
		WHERE token_digest = ? AND status = 0 AND expires_at > NOW()
	`, tokenDigest[:])
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return saasauth.ErrMFAChallengeInvalid
	}
	return nil
}

func (store *SaaSIdentityStore) CompleteMFAChallenge(ctx context.Context, tokenDigest [32]byte, userID int, authVersion uint64, challengeType string, totpStep int64) (saasauth.SaaSIdentity, error) {
	if store == nil || store.db == nil || userID <= 0 || authVersion == 0 || totpStep <= 0 || !validSaaSMFAChallengeType(challengeType) {
		return saasauth.SaaSIdentity{}, saasauth.ErrMFAChallengeInvalid
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var challenge saasauth.SaaSMFAChallenge
	if err := tx.QueryRowContext(ctx, `
		SELECT user_id, auth_version, challenge_type, status, attempts, max_attempts, expires_at
		FROM mochat_go_saas_admin_mfa_challenges
		WHERE token_digest = ? AND user_id = ? AND auth_version = ?
			AND challenge_type = ? AND status = 0 AND expires_at > NOW()
		FOR UPDATE
	`, tokenDigest[:], userID, authVersion, challengeType).Scan(
		&challenge.UserID, &challenge.AuthVersion, &challenge.ChallengeType, &challenge.Status,
		&challenge.Attempts, &challenge.MaxAttempts, &challenge.ExpiresAt,
	); err != nil {
		return saasauth.SaaSIdentity{}, saasauth.ErrMFAChallengeInvalid
	}
	var credentialStatus int
	var lastTOTPStep int64
	if err := tx.QueryRowContext(ctx, `
		SELECT status, last_totp_step
		FROM mochat_go_saas_admin_mfa_credentials
		WHERE user_id = ?
		FOR UPDATE
	`, userID).Scan(&credentialStatus, &lastTOTPStep); err != nil {
		return saasauth.SaaSIdentity{}, saasauth.ErrMFAChallengeInvalid
	}
	nextCredentialStatus, allowed := saasMFAStatusTransition(credentialStatus, challengeType)
	if !allowed || lastTOTPStep >= totpStep {
		return saasauth.SaaSIdentity{}, saasauth.ErrMFAChallengeInvalid
	}
	credentialUpdate, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_mfa_credentials
		SET status = ?, last_totp_step = ?, verified_at = IF(? = ?, NOW(), verified_at), updated_at = NOW()
		WHERE user_id = ? AND status = ? AND last_totp_step < ?
	`, nextCredentialStatus, totpStep, challengeType, saasauth.SaaSMFAChallengeEnrollment, userID, credentialStatus, totpStep)
	if err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	if affected, err := credentialUpdate.RowsAffected(); err != nil || affected != 1 {
		return saasauth.SaaSIdentity{}, saasauth.ErrMFAChallengeInvalid
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_mfa_challenges
		SET status = 1, consumed_at = NOW(), updated_at = NOW()
		WHERE token_digest = ? AND status = 0
	`, tokenDigest[:])
	if err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return saasauth.SaaSIdentity{}, saasauth.ErrMFAChallengeInvalid
	}
	identity, err := scanSaaSIdentity(tx.QueryRowContext(ctx, `
		SELECT id, login_name, COALESCE(phone, ''), password_hash, name,
		       status, must_rotate_password, auth_version, mfa_required
		FROM mochat_go_saas_admin_users
		WHERE id = ? AND status = 1 AND auth_version = ?
		LIMIT 1
	`, userID, authVersion))
	if err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	if err := tx.Commit(); err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	return identity, nil
}

func (store *SaaSIdentityStore) CompletePasswordChange(ctx context.Context, tokenDigest [32]byte, passwordHash string) (saasauth.SaaSIdentity, error) {
	if store == nil || store.db == nil || strings.TrimSpace(passwordHash) == "" {
		return saasauth.SaaSIdentity{}, saasauth.ErrPasswordChange
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var userID int
	var authVersion uint64
	if err := tx.QueryRowContext(ctx, `
		SELECT user_id, auth_version
		FROM mochat_go_saas_admin_mfa_challenges
		WHERE token_digest = ? AND challenge_type = ? AND status = 0 AND expires_at > NOW()
		FOR UPDATE
	`, tokenDigest[:], saasauth.SaaSMFAChallengePasswordChange).Scan(&userID, &authVersion); err != nil {
		return saasauth.SaaSIdentity{}, saasauth.ErrPasswordChange
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_users
		SET password_hash = ?, must_rotate_password = 0,
			auth_version = auth_version + 1, updated_at = NOW()
		WHERE id = ? AND auth_version = ? AND status = 1
	`, passwordHash, userID, authVersion)
	if err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return saasauth.SaaSIdentity{}, saasauth.ErrPasswordChange
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_mfa_challenges
		SET status = 1, consumed_at = NOW(), updated_at = NOW()
		WHERE token_digest = ? AND status = 0
	`, tokenDigest[:]); err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	identity, err := scanSaaSIdentity(tx.QueryRowContext(ctx, `
		SELECT id, login_name, COALESCE(phone, ''), password_hash, name,
		       status, must_rotate_password, auth_version, mfa_required
		FROM mochat_go_saas_admin_users
		WHERE id = ? AND status = 1
		LIMIT 1
	`, userID))
	if err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	if err := tx.Commit(); err != nil {
		return saasauth.SaaSIdentity{}, err
	}
	return identity, nil
}

func (store *SaaSIdentityStore) CreateSession(ctx context.Context, userID int, authVersion uint64, jtiDigest [32]byte, issuedAt, expiresAt time.Time) error {
	if store == nil || store.db == nil || userID <= 0 || authVersion == 0 || issuedAt.IsZero() || expiresAt.IsZero() || !expiresAt.After(issuedAt) {
		return saasauth.ErrSessionInvalid
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status int
	var currentVersion uint64
	if err := tx.QueryRowContext(ctx, `
		SELECT status, auth_version
		FROM mochat_go_saas_admin_users
		WHERE id = ?
		FOR UPDATE
	`, userID).Scan(&status, &currentVersion); err != nil {
		return err
	}
	if status != saasauth.SaaSIdentityStatusActive || currentVersion != authVersion {
		return saasauth.ErrSessionInvalid
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_admin_sessions
			(jti_digest, user_id, auth_version, status, issued_at, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?, NOW(), NOW())
	`, jtiDigest[:], userID, authVersion, issuedAt, expiresAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (store *SaaSIdentityStore) CheckSessionToken(ctx context.Context, claims authrealm.Claims) error {
	if store == nil || store.db == nil || claims.UserID <= 0 || claims.AuthVersion == 0 || strings.TrimSpace(claims.JWTID) == "" {
		return saasauth.ErrSessionInvalid
	}
	digest := sha256Digest(claims.JWTID)
	var sessionStatus, identityStatus int
	var sessionVersion, identityVersion uint64
	err := store.db.QueryRowContext(ctx, `
		SELECT s.status, s.auth_version, u.status, u.auth_version
		FROM mochat_go_saas_admin_sessions s
		JOIN mochat_go_saas_admin_users u ON u.id = s.user_id
		WHERE s.jti_digest = ? AND s.user_id = ? AND s.auth_version = ?
			AND s.status = 1 AND s.expires_at > NOW()
		LIMIT 1
	`, digest[:], claims.UserID, claims.AuthVersion).Scan(&sessionStatus, &sessionVersion, &identityStatus, &identityVersion)
	if err != nil || sessionStatus != 1 || identityStatus != saasauth.SaaSIdentityStatusActive || sessionVersion != claims.AuthVersion || identityVersion != claims.AuthVersion {
		return saasauth.ErrSessionInvalid
	}
	return nil
}

func (store *SaaSIdentityStore) RevokeSession(ctx context.Context, claims authrealm.Claims) error {
	if store == nil || store.db == nil || claims.UserID <= 0 || claims.AuthVersion == 0 || strings.TrimSpace(claims.JWTID) == "" {
		return saasauth.ErrSessionInvalid
	}
	digest := sha256Digest(claims.JWTID)
	result, err := store.db.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_sessions
		SET status = 2, revoked_at = NOW(), updated_at = NOW()
		WHERE jti_digest = ? AND user_id = ? AND auth_version = ? AND status = 1
	`, digest[:], claims.UserID, claims.AuthVersion)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return saasauth.ErrSessionRevoked
	}
	return nil
}

func validSaaSMFAChallengeType(value string) bool {
	switch value {
	case saasauth.SaaSMFAChallengeEnrollment, saasauth.SaaSMFAChallengeLogin, saasauth.SaaSMFAChallengePasswordChange:
		return true
	default:
		return false
	}
}

func saasMFAStatusTransition(currentStatus int, challengeType string) (int, bool) {
	switch challengeType {
	case saasauth.SaaSMFAChallengeEnrollment:
		if currentStatus != saasauth.SaaSMFAStatusPending {
			return 0, false
		}
		return saasauth.SaaSMFAStatusActive, true
	case saasauth.SaaSMFAChallengeLogin:
		if currentStatus != saasauth.SaaSMFAStatusActive {
			return 0, false
		}
		return saasauth.SaaSMFAStatusActive, true
	default:
		return 0, false
	}
}

func sha256Digest(value string) [32]byte {
	return sha256.Sum256([]byte(value))
}

var _ saasauth.SaaSAuthPersistence = (*SaaSIdentityStore)(nil)
