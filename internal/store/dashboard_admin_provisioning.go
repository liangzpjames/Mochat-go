package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardadmin"
)

const dashboardAdminActivationLifetime = 24 * time.Hour

const dashboardResendActivationOperation = "dashboard_activation_resend"
const dashboardSuperAdminReplaceOperation = "dashboard_superadmin_replace"
const dashboardSuperAdminStatusOperation = "dashboard_superadmin_status"

var errDashboardProvisionRunRace = errors.New("dashboard tenant provisioning receipt race")
var errDashboardGovernanceWriteMismatch = errors.New("dashboard governance write affected an unexpected number of rows")

// DashboardAdminGovernance returns only tenant-scoped Dashboard identity
// facts needed to render SaaS governance controls. It deliberately selects no
// password, digest, MFA, session, or token column.
func (s *MySQLStore) DashboardAdminGovernance(ctx context.Context, actor dashboardadmin.Actor, tenantID int) (dashboardadmin.DashboardAdminGovernanceView, error) {
	if s == nil || s.db == nil {
		return dashboardadmin.DashboardAdminGovernanceView{}, dashboardadmin.ErrStoreUnavailable
	}
	if tenantID <= 0 {
		return dashboardadmin.DashboardAdminGovernanceView{}, dashboardadmin.ErrInvalidRequest
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboardadmin.DashboardAdminGovernanceView{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockSaaSActorTx(ctx, tx, actor.UserID); err != nil {
		return dashboardadmin.DashboardAdminGovernanceView{}, err
	}
	bindingVersion, err := readDashboardTenantBindingTx(ctx, tx, tenantID)
	if err != nil {
		return dashboardadmin.DashboardAdminGovernanceView{}, err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT dashboard_user.id, dashboard_user.name, dashboard_user.status,
		       identity_row.login_identifier, identity_row.status, identity_row.activated_at,
		       COALESCE(dashboard_user.isSuperAdmin, 0)
		FROM mc_user dashboard_user
		INNER JOIN mochat_go_dashboard_identities identity_row ON identity_row.user_id=dashboard_user.id
		WHERE dashboard_user.tenant_id=? AND dashboard_user.deleted_at IS NULL
		ORDER BY dashboard_user.id
	`, tenantID)
	if err != nil {
		return dashboardadmin.DashboardAdminGovernanceView{}, err
	}
	defer rows.Close()
	view := dashboardadmin.DashboardAdminGovernanceView{TenantID: tenantID, BindingVersion: bindingVersion, Identities: make([]dashboardadmin.DashboardIdentityRecord, 0)}
	for rows.Next() {
		var id, userStatus, identityStatus, isSuperAdmin int
		var name, loginIdentifier string
		var activatedAt sql.NullTime
		if err := rows.Scan(&id, &name, &userStatus, &loginIdentifier, &identityStatus, &activatedAt, &isSuperAdmin); err != nil {
			return dashboardadmin.DashboardAdminGovernanceView{}, err
		}
		view.Identities = append(view.Identities, dashboardadmin.DashboardIdentityRecord{
			ID: id, Name: name, LoginIdentifier: loginIdentifier, UserStatus: userStatus, IdentityStatus: identityStatus,
			ActivatedAt: dashboardAdminNullableTimeString(activatedAt), IsSuperAdmin: isSuperAdmin == 1,
		})
	}
	if err := rows.Err(); err != nil {
		return dashboardadmin.DashboardAdminGovernanceView{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboardadmin.DashboardAdminGovernanceView{}, err
	}
	return view, nil
}

func dashboardAdminNullableTimeString(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339)
}

// ProvisionDashboardTenant is the only SQL entry point for a SaaS-controlled
// tenant bootstrap. Every artifact, including both audit records, is created
// by one transaction. The raw activation value is generated in memory and is
// never written to SQL, an audit, or an error.
func (s *MySQLStore) ProvisionDashboardTenant(ctx context.Context, actor dashboardadmin.Actor, input dashboardadmin.ProvisionDashboardTenant) (dashboardadmin.ProvisionResult, error) {
	if s == nil || s.db == nil {
		return dashboardadmin.ProvisionResult{}, dashboardadmin.ErrStoreUnavailable
	}
	if err := validateDashboardProvisionStoreInput(input); err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		result, err := s.provisionDashboardTenantTx(ctx, actor, input)
		if errors.Is(err, errDashboardProvisionRunRace) || isMySQLRetryableTransactionError(err) {
			if ctx.Err() != nil {
				return dashboardadmin.ProvisionResult{}, ctx.Err()
			}
			continue
		}
		return result, err
	}
	return dashboardadmin.ProvisionResult{}, errDashboardProvisionRunRace
}

// ResendDashboardActivation is a separate high-risk use case. It never
// changes an already activated identity. A generic, unique operation receipt
// is inserted before any target mutation; the raw value is returned only to
// the first successful caller and is never part of the receipt or audit.
func (s *MySQLStore) ResendDashboardActivation(ctx context.Context, actor dashboardadmin.Actor, input dashboardadmin.ResendActivationInput) (dashboardadmin.ResendActivationResult, error) {
	if s == nil || s.db == nil {
		return dashboardadmin.ResendActivationResult{}, dashboardadmin.ErrStoreUnavailable
	}
	if !validDashboardApprovalExecutionReference(input.ApprovalExecutionID, input.ApprovalExecutionVersion) || input.TenantID <= 0 || input.TargetUserID <= 0 || input.ExpectedVersion == 0 || strings.TrimSpace(input.RequestID) == "" {
		return dashboardadmin.ResendActivationResult{}, dashboardadmin.ErrInvalidRequest
	}
	fingerprint, err := dashboardResendActivationFingerprint(input)
	if err != nil {
		return dashboardadmin.ResendActivationResult{}, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		result, err := s.resendDashboardActivationTx(ctx, actor, input, fingerprint)
		if !isMySQLRetryableTransactionError(err) || ctx.Err() != nil || attempt == 2 {
			return result, err
		}
	}
	return dashboardadmin.ResendActivationResult{}, errDashboardProvisionRunRace
}

type dashboardIdempotencyReceipt struct {
	ID            int64
	Fingerprint   []byte
	TenantID      int
	TargetID      int
	ResultVersion uint64
	Status        int
}

func (s *MySQLStore) resendDashboardActivationTx(ctx context.Context, actor dashboardadmin.Actor, input dashboardadmin.ResendActivationInput, fingerprint []byte) (dashboardadmin.ResendActivationResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboardadmin.ResendActivationResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockSaaSActorTx(ctx, tx, actor.UserID); err != nil {
		return dashboardadmin.ResendActivationResult{}, err
	}
	receiptID, existing, err := insertOrLoadDashboardIdempotencyReceiptTx(ctx, tx, actor.UserID, dashboardResendActivationOperation, strings.TrimSpace(input.RequestID), fingerprint, input.TenantID, input.TargetUserID)
	if err != nil {
		return dashboardadmin.ResendActivationResult{}, err
	}
	if existing != nil {
		if existing.Status != 1 || !bytes.Equal(existing.Fingerprint, fingerprint) || existing.TenantID != input.TenantID || existing.TargetID != input.TargetUserID {
			return dashboardadmin.ResendActivationResult{}, dashboardadmin.ErrIdempotencyConflict
		}
		if err := markDashboardAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, actor.UserID, 0); err != nil {
			return dashboardadmin.ResendActivationResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return dashboardadmin.ResendActivationResult{}, err
		}
		return dashboardadmin.ResendActivationResult{
			TenantID: input.TenantID, DashboardUserID: input.TargetUserID,
			Version: existing.ResultVersion, Idempotent: true,
		}, nil
	}
	bindingVersion, _, err := lockDashboardTenantBindingTx(ctx, tx, input.TenantID)
	if err != nil {
		return dashboardadmin.ResendActivationResult{}, err
	}
	if bindingVersion != input.ExpectedVersion {
		return dashboardadmin.ResendActivationResult{}, dashboardadmin.ErrVersionConflict
	}
	subject, err := lockDashboardSubjectTx(ctx, tx, input.TenantID, input.TargetUserID)
	if err != nil {
		return dashboardadmin.ResendActivationResult{}, err
	}
	if subject.IsSuperAdmin != 1 {
		return dashboardadmin.ResendActivationResult{}, dashboardadmin.ErrNotSuperAdmin
	}
	if subject.UserStatus != 1 || subject.IdentityStatus != 1 || subject.AuthVersion == 0 || subject.MustRotatePassword != 1 {
		return dashboardadmin.ResendActivationResult{}, dashboardadmin.ErrTargetNotFound
	}
	if subject.ActivatedAt.Valid {
		return dashboardadmin.ResendActivationResult{}, dashboardadmin.ErrActivationAlreadyComplete
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_dashboard_identity_activations
		SET consumed_at = NOW(), updated_at = NOW()
		WHERE user_id = ? AND consumed_at IS NULL
	`, input.TargetUserID); err != nil {
		return dashboardadmin.ResendActivationResult{}, err
	}
	randomValue := make([]byte, 32)
	if _, err := rand.Read(randomValue); err != nil {
		return dashboardadmin.ResendActivationResult{}, err
	}
	activationToken := base64.RawURLEncoding.EncodeToString(randomValue)
	activationDigest := sha256.Sum256([]byte(activationToken))
	activationExpiresAt := time.Now().Add(dashboardAdminActivationLifetime)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_dashboard_identity_activations
			(user_id, token_digest, expires_at, consumed_at, created_by_saas_user_id, request_id, created_at)
		VALUES (?, ?, ?, NULL, ?, ?, NOW())
	`, input.TargetUserID, activationDigest[:], activationExpiresAt, actor.UserID, dashboardResendAuditRequestID(input.RequestID)); err != nil {
		return dashboardadmin.ResendActivationResult{}, err
	}
	resultVersion, err := advanceDashboardBindingTx(ctx, tx, input.TenantID, input.ExpectedVersion)
	if err != nil {
		return dashboardadmin.ResendActivationResult{}, err
	}
	after := map[string]any{
		"tenantId":         input.TenantID,
		"dashboardUserId":  input.TargetUserID,
		"activationIssued": true,
	}
	operationID, err := insertDashboardGovernanceAuditsTx(ctx, tx, actor.UserID, input.TenantID, "saas.admin.dashboard_activation.resend", fmt.Sprintf("%d", input.TargetUserID), after, input.ExpectedVersion, resultVersion, dashboardResendAuditRequestID(input.RequestID))
	if err != nil {
		return dashboardadmin.ResendActivationResult{}, err
	}
	if err := markDashboardAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, actor.UserID, operationID); err != nil {
		return dashboardadmin.ResendActivationResult{}, err
	}
	updated, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_idempotency_receipts
		SET status = 1, result_version = ?, updated_at = NOW()
		WHERE id = ? AND status = 0
	`, resultVersion, receiptID)
	if err != nil {
		return dashboardadmin.ResendActivationResult{}, err
	}
	if affected, err := updated.RowsAffected(); err != nil || affected != 1 {
		return dashboardadmin.ResendActivationResult{}, errDashboardProvisionRunRace
	}
	if err := tx.Commit(); err != nil {
		return dashboardadmin.ResendActivationResult{}, err
	}
	return dashboardadmin.ResendActivationResult{
		TenantID: input.TenantID, DashboardUserID: input.TargetUserID,
		Version: resultVersion, ActivationToken: activationToken, ActivationExpiresAt: activationExpiresAt,
	}, nil
}

func dashboardResendActivationFingerprint(input dashboardadmin.ResendActivationInput) ([]byte, error) {
	canonical := struct {
		TenantID        int
		TargetUserID    int
		ExpectedVersion uint64
	}{input.TenantID, input.TargetUserID, input.ExpectedVersion}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(encoded)
	return digest[:], nil
}

func dashboardResendAuditRequestID(requestID string) string {
	return "resend:" + strings.TrimSpace(requestID)
}

func insertOrLoadDashboardIdempotencyReceiptTx(ctx context.Context, tx *sql.Tx, actorUserID int, operation, requestKey string, fingerprint []byte, tenantID, targetID int) (int64, *dashboardIdempotencyReceipt, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_idempotency_receipts
			(operation, request_key, fingerprint, tenant_id, target_id, result_version, status, created_by_saas_user_id, created_at)
		VALUES (?, ?, ?, ?, ?, 0, 0, ?, NOW())
	`, operation, requestKey, fingerprint, tenantID, targetID, actorUserID)
	if err == nil {
		receiptID, err := result.LastInsertId()
		if err != nil || receiptID <= 0 {
			return 0, nil, errDashboardProvisionRunRace
		}
		return receiptID, nil, nil
	}
	if !isMySQLDuplicateKeyError(err) {
		return 0, nil, err
	}
	receipt := &dashboardIdempotencyReceipt{}
	if err := tx.QueryRowContext(ctx, `
		SELECT id, fingerprint, tenant_id, target_id, result_version, status
		FROM mochat_go_saas_idempotency_receipts
		WHERE operation = ? AND request_key = ?
		LIMIT 1
		FOR UPDATE
	`, operation, requestKey).Scan(&receipt.ID, &receipt.Fingerprint, &receipt.TenantID, &receipt.TargetID, &receipt.ResultVersion, &receipt.Status); err != nil {
		return 0, nil, err
	}
	return receipt.ID, receipt, nil
}

func (s *MySQLStore) provisionDashboardTenantTx(ctx context.Context, actor dashboardadmin.Actor, input dashboardadmin.ProvisionDashboardTenant) (dashboardadmin.ProvisionResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// This is deliberately the first business query in the transaction. The
	// HTTP principal and its permission set are advisory; this lock is the
	// authoritative SaaS identity-domain recheck.
	if err := lockSaaSActorTx(ctx, tx, actor.UserID); err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	fingerprint, err := dashboardProvisionFingerprint(input)
	if err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	if result, found, err := loadDashboardProvisionReceiptTx(ctx, tx, input.IdempotencyKey, fingerprint); err != nil {
		return dashboardadmin.ProvisionResult{}, err
	} else if found {
		if err := markDashboardAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, actor.UserID, 0); err != nil {
			return dashboardadmin.ProvisionResult{}, err
		}
		return result, tx.Commit()
	}

	loginIdentifier := strings.TrimSpace(input.AdminLoginIdentifier)
	if err := rejectExistingDashboardLoginTx(ctx, tx, loginIdentifier); err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	pkg, err := lockDashboardPackageTx(ctx, tx, input.PackageID)
	if err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	if pkg.Version != int(input.ExpectedVersion) || !dashboardAdminPackageLimitsValid(pkg.Limits) || !dashboardAdminPackageLimitsEqual(input.Limits, pkg.Limits) {
		return dashboardadmin.ProvisionResult{}, dashboardadmin.ErrVersionConflict
	}
	if requestedCode := strings.TrimSpace(input.Subscription.PackageCode); requestedCode != "" && requestedCode != pkg.Code {
		return dashboardadmin.ProvisionResult{}, dashboardadmin.ErrInvalidRequest
	}
	startsAt, err := dashboardAdminProvisionTime(input.Subscription.StartsAt)
	if err != nil {
		return dashboardadmin.ProvisionResult{}, dashboardadmin.ErrInvalidRequest
	}
	expiresAt, err := dashboardAdminProvisionTime(input.Subscription.ExpiresAt)
	if err != nil || (startsAt != nil && expiresAt != nil && !expiresAt.(time.Time).After(startsAt.(time.Time))) {
		return dashboardadmin.ProvisionResult{}, dashboardadmin.ErrInvalidRequest
	}
	subscriptionStatus := strings.TrimSpace(input.Subscription.Status)
	billingCycle := strings.TrimSpace(input.Subscription.BillingCycle)
	if (subscriptionStatus != "trialing" && subscriptionStatus != "active") || (billingCycle != "monthly" && billingCycle != "yearly" && billingCycle != "custom" && billingCycle != "lifetime") || startsAt == nil || expiresAt == nil {
		return dashboardadmin.ProvisionResult{}, dashboardadmin.ErrInvalidRequest
	}

	result, err := insertDashboardProvisionArtifactsTx(ctx, tx, actor.UserID, input, pkg, loginIdentifier, subscriptionStatus, billingCycle, startsAt, expiresAt, fingerprint)
	if err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	return result, nil
}

type dashboardAdminPackageSnapshot struct {
	ID      int
	Code    string
	Name    string
	Status  int
	Version int
	Limits  dashboardadmin.SaaSAdminPackageLimits
}

func lockSaaSActorTx(ctx context.Context, tx *sql.Tx, actorUserID int) error {
	if actorUserID <= 0 {
		return dashboardadmin.ErrPermissionDenied
	}
	var lockedID int
	err := tx.QueryRowContext(ctx, `
		SELECT admin_user.id
		FROM mochat_go_saas_admin_users admin_user
		WHERE admin_user.id = ? AND admin_user.status = 1
		  AND (EXISTS (
			SELECT 1
			FROM mochat_go_saas_admin_user_roles user_role
			INNER JOIN mochat_go_saas_admin_roles admin_role
				ON admin_role.id = user_role.role_id AND admin_role.status = 1
			INNER JOIN mochat_go_saas_admin_role_permissions admin_permission
				ON admin_permission.role_id = admin_role.id
			WHERE user_role.user_id = admin_user.id
			  AND (admin_permission.permission_code = ? OR admin_permission.permission_code = '*')
		  ))
		LIMIT 1
		FOR UPDATE
	`, actorUserID, dashboardadmin.PermissionTenantsManage).Scan(&lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboardadmin.ErrPermissionDenied
	}
	return err
}

func dashboardProvisionFingerprint(input dashboardadmin.ProvisionDashboardTenant) (string, error) {
	canonical := struct {
		TenantName           string
		WeComIntegrationMode string
		PackageID            int
		Limits               dashboardadmin.SaaSAdminPackageLimits
		Subscription         dashboardadmin.SubscriptionInput
		AdminLoginIdentifier string
		AdminName            string
		IdempotencyKey       string
		ExpectedVersion      uint64
	}{
		TenantName:           strings.TrimSpace(input.TenantName),
		WeComIntegrationMode: strings.TrimSpace(input.WeComIntegrationMode),
		PackageID:            input.PackageID,
		Limits:               input.Limits,
		Subscription: dashboardadmin.SubscriptionInput{
			PackageCode:  strings.TrimSpace(input.Subscription.PackageCode),
			Status:       strings.TrimSpace(input.Subscription.Status),
			BillingCycle: strings.TrimSpace(input.Subscription.BillingCycle),
			StartsAt:     strings.TrimSpace(input.Subscription.StartsAt),
			ExpiresAt:    strings.TrimSpace(input.Subscription.ExpiresAt),
		},
		AdminLoginIdentifier: strings.TrimSpace(input.AdminLoginIdentifier),
		AdminName:            strings.TrimSpace(input.AdminName),
		IdempotencyKey:       strings.TrimSpace(input.IdempotencyKey),
		ExpectedVersion:      input.ExpectedVersion,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func loadDashboardProvisionReceiptTx(ctx context.Context, tx *sql.Tx, runKey, fingerprint string) (dashboardadmin.ProvisionResult, bool, error) {
	var tenantID int
	var packageCode, adminPhone, message string
	var status int
	err := tx.QueryRowContext(ctx, `
		SELECT tenant_id, package_code, admin_phone, status, message
		FROM mochat_go_tenant_provision_runs
		WHERE run_key = ?
		LIMIT 1
		FOR UPDATE
	`, strings.TrimSpace(runKey)).Scan(&tenantID, &packageCode, &adminPhone, &status, &message)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboardadmin.ProvisionResult{}, false, nil
	}
	if err != nil {
		return dashboardadmin.ProvisionResult{}, false, err
	}
	if status != 1 || message != "request_sha256:"+fingerprint {
		return dashboardadmin.ProvisionResult{}, false, dashboardadmin.ErrIdempotencyConflict
	}
	var userID, corpID int
	err = tx.QueryRowContext(ctx, `
		SELECT dashboard_user.id, binding.corp_id
		FROM mc_user dashboard_user
		INNER JOIN mochat_go_dashboard_identities identity_row ON identity_row.user_id = dashboard_user.id
		INNER JOIN mochat_go_tenant_corp_bindings binding ON binding.tenant_id = dashboard_user.tenant_id
		WHERE dashboard_user.tenant_id = ? AND dashboard_user.phone = ? AND dashboard_user.deleted_at IS NULL
		LIMIT 1
	`, tenantID, adminPhone).Scan(&userID, &corpID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboardadmin.ProvisionResult{}, false, dashboardadmin.ErrTargetNotFound
	}
	if err != nil {
		return dashboardadmin.ProvisionResult{}, false, err
	}
	return dashboardadmin.ProvisionResult{TenantID: tenantID, DashboardUserID: userID, BindingCorpID: corpID, Idempotent: true}, true, nil
}

func rejectExistingDashboardLoginTx(ctx context.Context, tx *sql.Tx, loginIdentifier string) error {
	var existingID int
	err := tx.QueryRowContext(ctx, `
		SELECT dashboard_user.id FROM mc_user dashboard_user
		WHERE dashboard_user.phone = ? AND dashboard_user.deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, loginIdentifier).Scan(&existingID)
	if err == nil {
		return dashboardadmin.ErrLoginIdentifierConflict
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	err = tx.QueryRowContext(ctx, `
		SELECT user_id FROM mochat_go_dashboard_identities
		WHERE login_identifier = ?
		LIMIT 1
		FOR UPDATE
	`, loginIdentifier).Scan(&existingID)
	if err == nil {
		return dashboardadmin.ErrLoginIdentifierConflict
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

func lockDashboardPackageTx(ctx context.Context, tx *sql.Tx, packageID int) (dashboardAdminPackageSnapshot, error) {
	var pkg dashboardAdminPackageSnapshot
	err := tx.QueryRowContext(ctx, `
		SELECT id, code, name, status, version,
			max_corps, max_users, max_contacts, max_rooms, max_agents,
			channel_codes, shop_codes, radars, lotteries, room_infinite_pulls,
			room_fissions, room_clock_ins, room_qualities, room_calendars, room_reminds,
			contact_sops, room_sops, sensitive_words, storage_mb, contact_message_batches,
			room_message_batches, room_tag_pulls, work_room_auto_pulls, work_fissions,
			official_accounts, async_executions
		FROM mochat_go_saas_packages
		WHERE id = ? AND status = 1 AND deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, packageID).Scan(
		&pkg.ID, &pkg.Code, &pkg.Name, &pkg.Status, &pkg.Version,
		&pkg.Limits.MaxCorps, &pkg.Limits.MaxUsers, &pkg.Limits.MaxContacts, &pkg.Limits.MaxRooms,
		&pkg.Limits.MaxAgents, &pkg.Limits.ChannelCodes, &pkg.Limits.ShopCodes, &pkg.Limits.Radars,
		&pkg.Limits.Lotteries, &pkg.Limits.RoomInfinitePulls, &pkg.Limits.RoomFissions,
		&pkg.Limits.RoomClockIns, &pkg.Limits.RoomQualities, &pkg.Limits.RoomCalendars,
		&pkg.Limits.RoomReminds, &pkg.Limits.ContactSOPs, &pkg.Limits.RoomSOPs,
		&pkg.Limits.SensitiveWords, &pkg.Limits.StorageMB, &pkg.Limits.ContactMessageBatches,
		&pkg.Limits.RoomMessageBatches, &pkg.Limits.RoomTagPulls, &pkg.Limits.WorkRoomAutoPulls,
		&pkg.Limits.WorkFissions, &pkg.Limits.OfficialAccounts, &pkg.Limits.AsyncExecutions,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboardAdminPackageSnapshot{}, dashboardadmin.ErrTargetNotFound
	}
	if err != nil {
		return dashboardAdminPackageSnapshot{}, err
	}
	return pkg, nil
}

func dashboardAdminPackageLimitsValid(limits dashboardadmin.SaaSAdminPackageLimits) bool {
	return limits.MaxCorps >= 0 && limits.MaxUsers >= 0 && limits.MaxContacts >= 0 &&
		limits.MaxRooms >= 0 && limits.MaxAgents >= 0 && limits.ChannelCodes >= 0 &&
		limits.ShopCodes >= 0 && limits.Radars >= 0 && limits.Lotteries >= 0 &&
		limits.RoomInfinitePulls >= 0 && limits.RoomFissions >= 0 && limits.RoomClockIns >= 0 &&
		limits.RoomQualities >= 0 && limits.RoomCalendars >= 0 && limits.RoomReminds >= 0 &&
		limits.ContactSOPs >= 0 && limits.RoomSOPs >= 0 && limits.SensitiveWords >= 0 &&
		limits.StorageMB >= 0 && limits.ContactMessageBatches >= 0 && limits.RoomMessageBatches >= 0 &&
		limits.RoomTagPulls >= 0 && limits.WorkRoomAutoPulls >= 0 && limits.WorkFissions >= 0 &&
		limits.OfficialAccounts >= 0 && limits.AsyncExecutions >= 0
}

func dashboardAdminPackageLimitsEqual(left, right dashboardadmin.SaaSAdminPackageLimits) bool {
	return left.MaxCorps == right.MaxCorps && left.MaxUsers == right.MaxUsers && left.MaxContacts == right.MaxContacts &&
		left.MaxRooms == right.MaxRooms && left.MaxAgents == right.MaxAgents && left.ChannelCodes == right.ChannelCodes &&
		left.ShopCodes == right.ShopCodes && left.Radars == right.Radars && left.Lotteries == right.Lotteries &&
		left.RoomInfinitePulls == right.RoomInfinitePulls && left.RoomFissions == right.RoomFissions && left.RoomClockIns == right.RoomClockIns &&
		left.RoomQualities == right.RoomQualities && left.RoomCalendars == right.RoomCalendars && left.RoomReminds == right.RoomReminds &&
		left.ContactSOPs == right.ContactSOPs && left.RoomSOPs == right.RoomSOPs && left.SensitiveWords == right.SensitiveWords &&
		left.StorageMB == right.StorageMB && left.ContactMessageBatches == right.ContactMessageBatches && left.RoomMessageBatches == right.RoomMessageBatches &&
		left.RoomTagPulls == right.RoomTagPulls && left.WorkRoomAutoPulls == right.WorkRoomAutoPulls && left.WorkFissions == right.WorkFissions &&
		left.OfficialAccounts == right.OfficialAccounts && left.AsyncExecutions == right.AsyncExecutions
}

func insertDashboardProvisionArtifactsTx(ctx context.Context, tx *sql.Tx, actorUserID int, input dashboardadmin.ProvisionDashboardTenant, pkg dashboardAdminPackageSnapshot, loginIdentifier, subscriptionStatus, billingCycle string, startsAt, expiresAt any, fingerprint string) (dashboardadmin.ProvisionResult, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_tenant (name, status, created_at, updated_at, deleted_at)
		VALUES (?, 1, NOW(), NOW(), NULL)
	`, strings.TrimSpace(input.TenantName))
	if err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	tenantID64, err := result.LastInsertId()
	if err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	tenantID := int(tenantID64)

	result, err = tx.ExecContext(ctx, `
		INSERT INTO mc_corp (name, tenant_id, created_at, updated_at, deleted_at)
		VALUES (?, ?, NOW(), NOW(), NULL)
	`, strings.TrimSpace(input.TenantName), tenantID)
	if err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	corpID64, err := result.LastInsertId()
	if err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	corpID := int(corpID64)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_tenant_corp_bindings (tenant_id, corp_id, status, wecom_integration_mode, version, verified_corp_name, created_at, updated_at)
		VALUES (?, ?, 1, ?, 1, '', NOW(), NOW())
	`, tenantID, corpID, strings.TrimSpace(input.WeComIntegrationMode)); err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_wecom_integrations
			(id, tenant_id, corp_id, mode, slot, status, scope_json, scope_digest, missing_capabilities_json, generation, version, last_audit_at)
		VALUES (UUID(), ?, ?, ?, 'current', 'unconfigured', JSON_ARRAY(), SHA2('', 256), JSON_ARRAY(), 1, 1, NOW(6))
	`, tenantID, corpID, strings.TrimSpace(input.WeComIntegrationMode)); err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}

	limitsJSON, err := json.Marshal(pkg.Limits)
	if err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_tenant_packages
			(tenant_id, package_code, package_name, starts_at, expires_at, status, version, limits_json, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, 1, 1, ?, NOW(), NOW(), NULL)
	`, tenantID, pkg.Code, pkg.Name, startsAt, expiresAt, string(limitsJSON)); err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	metadataJSON := `{"source":"dashboard_admin_provisioning","packageVersion":` + fmt.Sprintf("%d", pkg.Version) + `}`
	subscriptionResult, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_subscriptions
			(tenant_id, package_code, package_name, status, billing_cycle, current_period_starts_at, current_period_ends_at, version, state_reason, metadata_json, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, 'tenant provisioned', ?, NOW(), NOW(), NULL)
	`, tenantID, pkg.Code, pkg.Name, subscriptionStatus, billingCycle, startsAt, expiresAt, metadataJSON)
	if err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	subscriptionID, err := subscriptionResult.LastInsertId()
	if err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_subscription_events
			(subscription_id, tenant_id, event_type, from_status, to_status, effective_at, actor_user_id, actor_tenant_id, source, idempotency_key, reason, payload_json, created_at)
		VALUES (?, ?, 'provisioned', '', ?, NOW(), ?, 0, 'saas_admin', ?, 'initial tenant provisioning', ?, NOW())
	`, subscriptionID, tenantID, subscriptionStatus, actorUserID, input.IdempotencyKey, metadataJSON); err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}

	userResult, err := tx.ExecContext(ctx, `
		INSERT INTO mc_user (phone, name, status, tenant_id, isSuperAdmin, created_at, updated_at, deleted_at)
		VALUES (?, ?, 1, ?, 1, NOW(), NOW(), NULL)
	`, loginIdentifier, strings.TrimSpace(input.AdminName), tenantID)
	if err != nil {
		if isMySQLDuplicateKeyError(err) {
			return dashboardadmin.ProvisionResult{}, dashboardadmin.ErrLoginIdentifierConflict
		}
		return dashboardadmin.ProvisionResult{}, err
	}
	userID64, err := userResult.LastInsertId()
	if err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	userID := int(userID64)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_dashboard_identities
			(user_id, login_identifier, password_hash, status, must_rotate_password, auth_version, mfa_required, activated_at, created_at, updated_at)
		VALUES (?, ?, '!activation-pending', 1, 1, 1, 1, NULL, NOW(), NOW())
	`, userID, loginIdentifier); err != nil {
		if isMySQLDuplicateKeyError(err) {
			return dashboardadmin.ProvisionResult{}, dashboardadmin.ErrLoginIdentifierConflict
		}
		return dashboardadmin.ProvisionResult{}, err
	}

	randomValue := make([]byte, 32)
	if _, err := rand.Read(randomValue); err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	activationToken := base64.RawURLEncoding.EncodeToString(randomValue)
	activationDigest := sha256.Sum256([]byte(activationToken))
	activationExpiresAt := time.Now().Add(dashboardAdminActivationLifetime)
	requestID := strings.TrimSpace(input.RequestID)
	if requestID == "" {
		requestID = strings.TrimSpace(input.IdempotencyKey)
	}
	requestID = truncateRunes(requestID, 96)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_dashboard_identity_activations
			(user_id, token_digest, expires_at, consumed_at, created_by_saas_user_id, request_id, created_at)
		VALUES (?, ?, ?, NULL, ?, ?, NOW())
	`, userID, activationDigest[:], activationExpiresAt, actorUserID, requestID); err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_tenant_provision_runs
			(run_key, tenant_id, package_code, admin_phone, status, message, started_at, finished_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, NOW(), NOW(), NOW(), NOW())
	`, input.IdempotencyKey, tenantID, pkg.Code, loginIdentifier, "request_sha256:"+fingerprint); err != nil {
		if isMySQLDuplicateKeyError(err) {
			return dashboardadmin.ProvisionResult{}, errDashboardProvisionRunRace
		}
		return dashboardadmin.ProvisionResult{}, err
	}

	after := map[string]any{
		"tenantId":             tenantID,
		"corpId":               corpID,
		"dashboardUserId":      userID,
		"packageCode":          pkg.Code,
		"packageVersion":       pkg.Version,
		"activationIssued":     true,
		"isSuperAdmin":         true,
		"requestId":            requestID,
		"wecomIntegrationMode": strings.TrimSpace(input.WeComIntegrationMode),
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID:      tenantID,
		ActorUserID:   actorUserID,
		ActorTenantID: 0,
		Action:        "saas.admin.dashboard_tenant.provision",
		TargetType:    "tenant",
		TargetID:      fmt.Sprintf("%d", tenantID),
		TargetName:    strings.TrimSpace(input.TenantName),
		AfterJSON:     string(afterJSON),
		Remark:        "dashboard tenant provisioned",
	})
	if err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	if err := insertDashboardAdminGovernanceAuditTx(ctx, tx, tenantID, actorUserID, "dashboard.tenant.provision", "tenant", fmt.Sprintf("%d", tenantID), nil, after, input.ExpectedVersion, 1, requestID); err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	if err := markDashboardAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, actorUserID, operationID); err != nil {
		return dashboardadmin.ProvisionResult{}, err
	}
	return dashboardadmin.ProvisionResult{TenantID: tenantID, DashboardUserID: userID, BindingCorpID: corpID, ActivationToken: activationToken, ActivationExpiresAt: activationExpiresAt}, nil
}

func dashboardAdminProvisionTime(raw string) (any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, nil
		}
	}
	return nil, errors.New("invalid dashboard provisioning time")
}

func validateDashboardProvisionStoreInput(input dashboardadmin.ProvisionDashboardTenant) error {
	if !validDashboardApprovalExecutionReference(input.ApprovalExecutionID, input.ApprovalExecutionVersion) || input.BodyTenantID != 0 || input.BodyActorID != 0 || input.PackageID <= 0 || input.ExpectedVersion == 0 {
		return dashboardadmin.ErrInvalidRequest
	}
	if strings.TrimSpace(input.TenantName) == "" || len([]rune(strings.TrimSpace(input.TenantName))) > 255 ||
		strings.TrimSpace(input.AdminName) == "" || len([]rune(strings.TrimSpace(input.AdminName))) > 255 ||
		!dashboardAdminValidPhone(input.AdminLoginIdentifier) || strings.TrimSpace(input.IdempotencyKey) == "" || len([]rune(strings.TrimSpace(input.IdempotencyKey))) > 128 {
		return dashboardadmin.ErrInvalidRequest
	}
	if !dashboardadmin.ValidWeComIntegrationMode(strings.TrimSpace(input.WeComIntegrationMode)) {
		return dashboardadmin.ErrInvalidRequest
	}
	if input.Subscription.Status != "trialing" && input.Subscription.Status != "active" {
		return dashboardadmin.ErrInvalidRequest
	}
	if input.Subscription.BillingCycle != "monthly" && input.Subscription.BillingCycle != "yearly" && input.Subscription.BillingCycle != "custom" && input.Subscription.BillingCycle != "lifetime" {
		return dashboardadmin.ErrInvalidRequest
	}
	startsAt, err := dashboardAdminProvisionTime(input.Subscription.StartsAt)
	if err != nil || startsAt == nil {
		return dashboardadmin.ErrInvalidRequest
	}
	expiresAt, err := dashboardAdminProvisionTime(input.Subscription.ExpiresAt)
	if err != nil || expiresAt == nil || !expiresAt.(time.Time).After(startsAt.(time.Time)) {
		return dashboardadmin.ErrInvalidRequest
	}
	if !dashboardAdminPackageLimitsValid(input.Limits) {
		return dashboardadmin.ErrInvalidRequest
	}
	return nil
}

func dashboardAdminValidPhone(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 11 {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

type dashboardAdminSubject struct {
	UserID             int
	UserStatus         int
	IsSuperAdmin       int
	IdentityStatus     int
	MustRotatePassword int
	AuthVersion        uint64
	ActivatedAt        sql.NullTime
}

func (s *MySQLStore) ReplaceDashboardSuperAdmin(ctx context.Context, actor dashboardadmin.Actor, input dashboardadmin.ReplaceSuperAdminInput) (dashboardadmin.GovernanceResult, error) {
	if s == nil || s.db == nil {
		return dashboardadmin.GovernanceResult{}, dashboardadmin.ErrStoreUnavailable
	}
	if !validDashboardApprovalExecutionReference(input.ApprovalExecutionID, input.ApprovalExecutionVersion) || input.TenantID <= 0 || input.CurrentAdminID <= 0 || input.NewAdminID <= 0 || input.CurrentAdminID == input.NewAdminID || input.ExpectedVersion == 0 || !validDashboardGovernanceRequestKey(input.RequestID) {
		return dashboardadmin.GovernanceResult{}, dashboardadmin.ErrInvalidRequest
	}
	for attempt := 0; attempt < 3; attempt++ {
		result, err := s.replaceDashboardSuperAdminTx(ctx, actor, input)
		if !isMySQLRetryableTransactionError(err) || ctx.Err() != nil || attempt == 2 {
			return result, err
		}
	}
	return dashboardadmin.GovernanceResult{}, errDashboardProvisionRunRace
}

func (s *MySQLStore) replaceDashboardSuperAdminTx(ctx context.Context, actor dashboardadmin.Actor, input dashboardadmin.ReplaceSuperAdminInput) (dashboardadmin.GovernanceResult, error) {
	if s == nil || s.db == nil {
		return dashboardadmin.GovernanceResult{}, dashboardadmin.ErrStoreUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockSaaSActorTx(ctx, tx, actor.UserID); err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	fingerprint, err := dashboardGovernanceFingerprint(dashboardSuperAdminReplaceOperation, input.TenantID, input.CurrentAdminID, input.NewAdminID, 0, false, input.ExpectedVersion)
	if err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	receiptID, existing, err := insertOrLoadDashboardIdempotencyReceiptTx(ctx, tx, actor.UserID, dashboardSuperAdminReplaceOperation, strings.TrimSpace(input.RequestID), fingerprint, input.TenantID, input.NewAdminID)
	if err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	if existing != nil {
		if !dashboardGovernanceReceiptMatches(existing, fingerprint, input.TenantID, input.NewAdminID) {
			return dashboardadmin.GovernanceResult{}, dashboardadmin.ErrIdempotencyConflict
		}
		if err := markDashboardAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, actor.UserID, 0); err != nil {
			return dashboardadmin.GovernanceResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return dashboardadmin.GovernanceResult{}, err
		}
		return dashboardadmin.GovernanceResult{TenantID: input.TenantID, DashboardUserID: input.NewAdminID, Version: existing.ResultVersion, Idempotent: true}, nil
	}
	bindingVersion, _, err := lockDashboardTenantBindingTx(ctx, tx, input.TenantID)
	if err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	if bindingVersion != input.ExpectedVersion {
		return dashboardadmin.GovernanceResult{}, dashboardadmin.ErrVersionConflict
	}
	subjects, err := lockDashboardSubjectsTx(ctx, tx, input.TenantID, input.CurrentAdminID, input.NewAdminID)
	if err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	current := subjects[input.CurrentAdminID]
	newSubject := subjects[input.NewAdminID]
	if !dashboardAdminSubjectIsActiveSuperAdmin(current) || newSubject.AuthVersion == 0 || newSubject.UserStatus != 1 || newSubject.IdentityStatus != 1 || !newSubject.ActivatedAt.Valid {
		return dashboardadmin.GovernanceResult{}, dashboardadmin.ErrReplacementRequiresActivation
	}
	if err := execDashboardGovernanceUpdateTx(ctx, tx, 1, `UPDATE mc_user SET isSuperAdmin=0, dashboard_access_version=dashboard_access_version+1, updated_at=NOW() WHERE tenant_id=? AND id=?`, input.TenantID, input.CurrentAdminID); err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	if err := execDashboardGovernanceUpdateTx(ctx, tx, 1, `UPDATE mc_user SET isSuperAdmin=1, dashboard_access_version=dashboard_access_version+1, updated_at=NOW() WHERE tenant_id=? AND id=?`, input.TenantID, input.NewAdminID); err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	if err := execDashboardGovernanceUpdateTx(ctx, tx, 2, `UPDATE mochat_go_dashboard_identities SET auth_version=auth_version+1, updated_at=NOW() WHERE user_id IN (?, ?)`, input.CurrentAdminID, input.NewAdminID); err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	resultVersion, err := advanceDashboardBindingTx(ctx, tx, input.TenantID, input.ExpectedVersion)
	if err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	requestID := dashboardGovernanceRequestID(input.RequestID, "replace", input.TenantID, input.NewAdminID)
	after := map[string]any{"tenantId": input.TenantID, "oldAdminId": input.CurrentAdminID, "newAdminId": input.NewAdminID, "isSuperAdmin": true}
	operationID, err := insertDashboardGovernanceAuditsTx(ctx, tx, actor.UserID, input.TenantID, "saas.admin.dashboard_superadmin.replace", fmt.Sprintf("%d", input.NewAdminID), after, input.ExpectedVersion, resultVersion, requestID)
	if err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	if err := markDashboardAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, actor.UserID, operationID); err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	if err := completeDashboardIdempotencyReceiptTx(ctx, tx, receiptID, resultVersion); err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	return dashboardadmin.GovernanceResult{TenantID: input.TenantID, DashboardUserID: input.NewAdminID, Version: resultVersion}, nil
}

func (s *MySQLStore) SetDashboardSuperAdminStatus(ctx context.Context, actor dashboardadmin.Actor, input dashboardadmin.SuperAdminStatusInput) (dashboardadmin.GovernanceResult, error) {
	if s == nil || s.db == nil {
		return dashboardadmin.GovernanceResult{}, dashboardadmin.ErrStoreUnavailable
	}
	if !validDashboardApprovalExecutionReference(input.ApprovalExecutionID, input.ApprovalExecutionVersion) || input.TenantID <= 0 || input.TargetUserID <= 0 || input.ExpectedVersion == 0 || !validDashboardGovernanceRequestKey(input.RequestID) {
		return dashboardadmin.GovernanceResult{}, dashboardadmin.ErrInvalidRequest
	}
	for attempt := 0; attempt < 3; attempt++ {
		result, err := s.setDashboardSuperAdminStatusTx(ctx, actor, input)
		if !isMySQLRetryableTransactionError(err) || ctx.Err() != nil || attempt == 2 {
			return result, err
		}
	}
	return dashboardadmin.GovernanceResult{}, errDashboardProvisionRunRace
}

func (s *MySQLStore) setDashboardSuperAdminStatusTx(ctx context.Context, actor dashboardadmin.Actor, input dashboardadmin.SuperAdminStatusInput) (dashboardadmin.GovernanceResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockSaaSActorTx(ctx, tx, actor.UserID); err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	fingerprint, err := dashboardGovernanceFingerprint(dashboardSuperAdminStatusOperation, input.TenantID, 0, 0, input.TargetUserID, input.Enabled, input.ExpectedVersion)
	if err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	receiptID, existing, err := insertOrLoadDashboardIdempotencyReceiptTx(ctx, tx, actor.UserID, dashboardSuperAdminStatusOperation, strings.TrimSpace(input.RequestID), fingerprint, input.TenantID, input.TargetUserID)
	if err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	if existing != nil {
		if !dashboardGovernanceReceiptMatches(existing, fingerprint, input.TenantID, input.TargetUserID) {
			return dashboardadmin.GovernanceResult{}, dashboardadmin.ErrIdempotencyConflict
		}
		if err := markDashboardAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, actor.UserID, 0); err != nil {
			return dashboardadmin.GovernanceResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return dashboardadmin.GovernanceResult{}, err
		}
		return dashboardadmin.GovernanceResult{TenantID: input.TenantID, DashboardUserID: input.TargetUserID, Version: existing.ResultVersion, Idempotent: true}, nil
	}
	bindingVersion, _, err := lockDashboardTenantBindingTx(ctx, tx, input.TenantID)
	if err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	if bindingVersion != input.ExpectedVersion {
		return dashboardadmin.GovernanceResult{}, dashboardadmin.ErrVersionConflict
	}
	subject, activeIDs, err := lockDashboardStatusSubjectsTx(ctx, tx, input.TenantID, input.TargetUserID)
	if err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	if subject.IsSuperAdmin != 1 {
		return dashboardadmin.GovernanceResult{}, dashboardadmin.ErrNotSuperAdmin
	}
	if subject.AuthVersion == 0 || !subject.ActivatedAt.Valid {
		return dashboardadmin.GovernanceResult{}, dashboardadmin.ErrReplacementRequiresActivation
	}
	currentlySuperAdmin := dashboardAdminSubjectIsActiveSuperAdmin(subject)
	if input.Enabled == currentlySuperAdmin {
		if err := markDashboardAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, actor.UserID, 0); err != nil {
			return dashboardadmin.GovernanceResult{}, err
		}
		if err := completeDashboardIdempotencyReceiptTx(ctx, tx, receiptID, input.ExpectedVersion); err != nil {
			return dashboardadmin.GovernanceResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return dashboardadmin.GovernanceResult{}, err
		}
		return dashboardadmin.GovernanceResult{TenantID: input.TenantID, DashboardUserID: input.TargetUserID, Version: input.ExpectedVersion}, nil
	}
	if !input.Enabled {
		if len(activeIDs) <= 1 {
			return dashboardadmin.GovernanceResult{}, dashboardadmin.ErrLastSuperAdmin
		}
		if err := execDashboardGovernanceUpdateTx(ctx, tx, 1, `UPDATE mc_user SET status=2, isSuperAdmin=1, dashboard_access_version=dashboard_access_version+1, updated_at=NOW() WHERE tenant_id=? AND id=?`, input.TenantID, input.TargetUserID); err != nil {
			return dashboardadmin.GovernanceResult{}, err
		}
		if err := execDashboardGovernanceUpdateTx(ctx, tx, 1, `UPDATE mochat_go_dashboard_identities SET status=2, auth_version=auth_version+1, updated_at=NOW() WHERE user_id=?`, input.TargetUserID); err != nil {
			return dashboardadmin.GovernanceResult{}, err
		}
	} else {
		if err := execDashboardGovernanceUpdateTx(ctx, tx, 1, `UPDATE mc_user SET status=1, isSuperAdmin=1, dashboard_access_version=dashboard_access_version+1, updated_at=NOW() WHERE tenant_id=? AND id=?`, input.TenantID, input.TargetUserID); err != nil {
			return dashboardadmin.GovernanceResult{}, err
		}
		if err := execDashboardGovernanceUpdateTx(ctx, tx, 1, `UPDATE mochat_go_dashboard_identities SET status=1, auth_version=auth_version+1, updated_at=NOW() WHERE user_id=?`, input.TargetUserID); err != nil {
			return dashboardadmin.GovernanceResult{}, err
		}
	}
	resultVersion, err := advanceDashboardBindingTx(ctx, tx, input.TenantID, input.ExpectedVersion)
	if err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	requestID := dashboardGovernanceRequestID(input.RequestID, "status", input.TenantID, input.TargetUserID)
	after := map[string]any{"tenantId": input.TenantID, "targetUserId": input.TargetUserID, "enabled": input.Enabled, "isSuperAdmin": true}
	operationID, err := insertDashboardGovernanceAuditsTx(ctx, tx, actor.UserID, input.TenantID, "saas.admin.dashboard_superadmin.status", fmt.Sprintf("%d", input.TargetUserID), after, input.ExpectedVersion, resultVersion, requestID)
	if err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	if err := markDashboardAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, actor.UserID, operationID); err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	if err := completeDashboardIdempotencyReceiptTx(ctx, tx, receiptID, resultVersion); err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboardadmin.GovernanceResult{}, err
	}
	return dashboardadmin.GovernanceResult{TenantID: input.TenantID, DashboardUserID: input.TargetUserID, Version: resultVersion}, nil
}

func lockDashboardTenantBindingTx(ctx context.Context, tx *sql.Tx, tenantID int) (uint64, int, error) {
	var version uint64
	var corpID int
	var status int
	err := tx.QueryRowContext(ctx, `
		SELECT binding.version, binding.corp_id, binding.status
		FROM mc_tenant tenant
		INNER JOIN mochat_go_tenant_corp_bindings binding ON binding.tenant_id=tenant.id
		WHERE tenant.id=? AND tenant.status=1 AND tenant.deleted_at IS NULL
		  AND binding.status IN (1, 2)
		LIMIT 1
		FOR UPDATE
	`, tenantID).Scan(&version, &corpID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, dashboardadmin.ErrTargetNotFound
	}
	return version, corpID, err
}

func readDashboardTenantBindingTx(ctx context.Context, tx *sql.Tx, tenantID int) (uint64, error) {
	var version uint64
	err := tx.QueryRowContext(ctx, `
		SELECT binding.version
		FROM mc_tenant tenant
		INNER JOIN mochat_go_tenant_corp_bindings binding ON binding.tenant_id=tenant.id
		WHERE tenant.id=? AND tenant.status=1 AND tenant.deleted_at IS NULL
		  AND binding.status IN (1, 2)
		LIMIT 1
	`, tenantID).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, dashboardadmin.ErrTargetNotFound
	}
	return version, err
}

func advanceDashboardBindingTx(ctx context.Context, tx *sql.Tx, tenantID int, expected uint64) (uint64, error) {
	result, err := tx.ExecContext(ctx, `UPDATE mochat_go_tenant_corp_bindings SET version=version+1, updated_at=NOW() WHERE tenant_id=? AND version=? AND status IN (1, 2)`, tenantID, expected)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if affected != 1 {
		return 0, dashboardadmin.ErrVersionConflict
	}
	return expected + 1, nil
}

func lockDashboardSubjectTx(ctx context.Context, tx *sql.Tx, tenantID, userID int) (dashboardAdminSubject, error) {
	var subject dashboardAdminSubject
	err := tx.QueryRowContext(ctx, `
		SELECT dashboard_user.id, dashboard_user.status, COALESCE(dashboard_user.isSuperAdmin,0), identity_row.status, identity_row.must_rotate_password, identity_row.auth_version, identity_row.activated_at
		FROM mc_user dashboard_user
		INNER JOIN mochat_go_dashboard_identities identity_row ON identity_row.user_id=dashboard_user.id
		WHERE dashboard_user.tenant_id=? AND dashboard_user.id=? AND dashboard_user.deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, tenantID, userID).Scan(&subject.UserID, &subject.UserStatus, &subject.IsSuperAdmin, &subject.IdentityStatus, &subject.MustRotatePassword, &subject.AuthVersion, &subject.ActivatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboardAdminSubject{}, dashboardadmin.ErrTargetNotFound
	}
	return subject, err
}

func lockDashboardSubjectsTx(ctx context.Context, tx *sql.Tx, tenantID, firstUserID, secondUserID int) (map[int]dashboardAdminSubject, error) {
	lowID, highID := firstUserID, secondUserID
	if lowID > highID {
		lowID, highID = highID, lowID
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT dashboard_user.id, dashboard_user.status, COALESCE(dashboard_user.isSuperAdmin,0), identity_row.status, identity_row.must_rotate_password, identity_row.auth_version, identity_row.activated_at
		FROM mc_user dashboard_user
		INNER JOIN mochat_go_dashboard_identities identity_row ON identity_row.user_id=dashboard_user.id
		WHERE dashboard_user.tenant_id=? AND dashboard_user.id IN (?, ?) AND dashboard_user.deleted_at IS NULL
		ORDER BY dashboard_user.id
		FOR UPDATE
	`, tenantID, lowID, highID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int]dashboardAdminSubject, 2)
	for rows.Next() {
		var subject dashboardAdminSubject
		if err := rows.Scan(&subject.UserID, &subject.UserStatus, &subject.IsSuperAdmin, &subject.IdentityStatus, &subject.MustRotatePassword, &subject.AuthVersion, &subject.ActivatedAt); err != nil {
			return nil, err
		}
		result[subject.UserID] = subject
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result) != 2 {
		return nil, dashboardadmin.ErrTargetNotFound
	}
	return result, nil
}

func lockDashboardStatusSubjectsTx(ctx context.Context, tx *sql.Tx, tenantID, targetUserID int) (dashboardAdminSubject, []int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT dashboard_user.id, dashboard_user.status, COALESCE(dashboard_user.isSuperAdmin,0), identity_row.status, identity_row.must_rotate_password, identity_row.auth_version, identity_row.activated_at
		FROM mc_user dashboard_user
		INNER JOIN mochat_go_dashboard_identities identity_row ON identity_row.user_id=dashboard_user.id
		WHERE dashboard_user.tenant_id=? AND dashboard_user.deleted_at IS NULL
		  AND (dashboard_user.id=? OR (dashboard_user.status=1 AND dashboard_user.isSuperAdmin=1 AND identity_row.status=1 AND identity_row.auth_version>0 AND identity_row.activated_at IS NOT NULL))
		ORDER BY dashboard_user.id
		FOR UPDATE
	`, tenantID, targetUserID)
	if err != nil {
		return dashboardAdminSubject{}, nil, err
	}
	defer rows.Close()
	var target dashboardAdminSubject
	foundTarget := false
	activeIDs := make([]int, 0)
	for rows.Next() {
		var subject dashboardAdminSubject
		if err := rows.Scan(&subject.UserID, &subject.UserStatus, &subject.IsSuperAdmin, &subject.IdentityStatus, &subject.MustRotatePassword, &subject.AuthVersion, &subject.ActivatedAt); err != nil {
			return dashboardAdminSubject{}, nil, err
		}
		if subject.UserID == targetUserID {
			target = subject
			foundTarget = true
		}
		if dashboardAdminSubjectIsActiveSuperAdmin(subject) {
			activeIDs = append(activeIDs, subject.UserID)
		}
	}
	if err := rows.Err(); err != nil {
		return dashboardAdminSubject{}, nil, err
	}
	if !foundTarget {
		return dashboardAdminSubject{}, nil, dashboardadmin.ErrTargetNotFound
	}
	return target, activeIDs, nil
}

func execDashboardGovernanceUpdateTx(ctx context.Context, tx *sql.Tx, expected int64, query string, args ...any) error {
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != expected {
		return errDashboardGovernanceWriteMismatch
	}
	return nil
}

func dashboardAdminSubjectIsActiveSuperAdmin(subject dashboardAdminSubject) bool {
	return subject.UserStatus == 1 && subject.IsSuperAdmin == 1 && subject.IdentityStatus == 1 && subject.AuthVersion > 0 && subject.ActivatedAt.Valid
}

func lockActiveDashboardSuperAdminsTx(ctx context.Context, tx *sql.Tx, tenantID int) ([]int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT dashboard_user.id
		FROM mc_user dashboard_user
		INNER JOIN mochat_go_dashboard_identities identity_row ON identity_row.user_id=dashboard_user.id
		WHERE dashboard_user.tenant_id=? AND dashboard_user.status=1 AND dashboard_user.isSuperAdmin=1 AND dashboard_user.deleted_at IS NULL
		  AND identity_row.status=1 AND identity_row.auth_version>0 AND identity_row.activated_at IS NOT NULL
		ORDER BY dashboard_user.id
		FOR UPDATE
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int, 0)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func insertDashboardGovernanceAuditsTx(ctx context.Context, tx *sql.Tx, actorUserID, tenantID int, action, targetID string, after any, expected, resultVersion uint64, requestID string) (int64, error) {
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return 0, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID:      tenantID,
		ActorUserID:   actorUserID,
		ActorTenantID: 0,
		Action:        action,
		TargetType:    "dashboard_superadmin",
		TargetID:      targetID,
		AfterJSON:     string(afterJSON),
		Remark:        "dashboard super administrator governance",
	})
	if err != nil {
		return 0, err
	}
	if err := insertDashboardAdminGovernanceAuditTx(ctx, tx, tenantID, actorUserID, action, "dashboard_superadmin", targetID, nil, after, expected, resultVersion, requestID); err != nil {
		return 0, err
	}
	return operationID, nil
}

func markDashboardAdminApprovalEffectTx(ctx context.Context, tx *sql.Tx, approvalID int64, approvalVersion int, actorUserID int, operationID int64) error {
	if approvalID == 0 && approvalVersion == 0 {
		return nil
	}
	if approvalID <= 0 || approvalVersion <= 0 {
		return dashboardadmin.ErrInvalidRequest
	}
	return markSaaSAdminApprovalEffectTx(ctx, tx, approvalID, approvalVersion, actorUserID, operationID)
}

func insertDashboardAdminGovernanceAuditTx(ctx context.Context, tx *sql.Tx, tenantID, actorUserID int, action, targetType, targetID string, before, after any, expected, resultVersion uint64, requestID string) error {
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return err
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mochat_go_dashboard_permission_audits
			(tenant_id, actor_user_id, action, target_type, target_id, before_json, after_json, expected_version, result_version, request_id, created_at)
		VALUES (?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
	`, tenantID, action, targetType, targetID, string(beforeJSON), string(afterJSON), expected, resultVersion, requestID)
	return err
}

func dashboardGovernanceRequestID(requestID, operation string, tenantID, targetID int) string {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = fmt.Sprintf("dashboard-admin-%s-%d-%d", operation, tenantID, targetID)
	}
	return truncateRunes(requestID, 96)
}

func validDashboardGovernanceRequestKey(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len([]rune(value)) <= 80
}

func validDashboardApprovalExecutionReference(id int64, version int) bool {
	return (id == 0 && version == 0) || (id > 0 && version > 0)
}

func dashboardGovernanceFingerprint(operation string, tenantID, currentAdminID, newAdminID, targetUserID int, enabled bool, expectedVersion uint64) ([]byte, error) {
	canonical := struct {
		Operation       string
		TenantID        int
		CurrentAdminID  int
		NewAdminID      int
		TargetUserID    int
		Enabled         bool
		ExpectedVersion uint64
	}{operation, tenantID, currentAdminID, newAdminID, targetUserID, enabled, expectedVersion}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(encoded)
	return digest[:], nil
}

func dashboardGovernanceReceiptMatches(receipt *dashboardIdempotencyReceipt, fingerprint []byte, tenantID, targetID int) bool {
	return receipt != nil && receipt.Status == 1 && bytes.Equal(receipt.Fingerprint, fingerprint) && receipt.TenantID == tenantID && receipt.TargetID == targetID
}

func completeDashboardIdempotencyReceiptTx(ctx context.Context, tx *sql.Tx, receiptID int64, resultVersion uint64) error {
	updated, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_idempotency_receipts
		SET status = 1, result_version = ?, updated_at = NOW()
		WHERE id = ? AND status = 0
	`, resultVersion, receiptID)
	if err != nil {
		return err
	}
	affected, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errDashboardProvisionRunRace
	}
	return nil
}
