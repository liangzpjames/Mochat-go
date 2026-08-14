package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

var (
	ErrCapabilityOperationConflict = errors.New("capability operation conflict")
	ErrCapabilityOperationStale    = errors.New("capability operation lease or credential is stale")
	ErrCapabilityInvalidState      = errors.New("capability operation state is invalid")
)

const (
	capabilityActorUser   = "user"
	capabilityActorSystem = "system"
)

type CapabilityOperationInput struct {
	Capability     string
	Action         wecomcapability.OperationAction
	IdempotencyKey string
	RequestID      string
	TargetTotal    int
	ActorSource    string
}

type CapabilityOperationTransitionInput struct {
	OperationID       int64
	Status            string
	LeaseToken        string
	Attempt           int
	ProviderRequestID string
	ProviderObjectID  string
	ActualAgentID     string
	ExternalSuccess   bool
	CallbackEvidence  bool
	TargetTotal       int
	SuccessTotal      int
	FailureTotal      int
	ErrorCode         string
	RequestID         string
}

type CapabilityDispatchInput struct {
	OperationID    int64
	DispatchKind   string
	ChunkNo        int
	TargetID       string
	IdempotencyKey string
}

type CapabilityDispatchTransitionInput struct {
	DispatchID        int64
	Status            string
	LeaseToken        string
	Attempt           int
	ProviderRequestID string
	ProviderMessageID string
	ProviderObjectID  string
	NextPollAt        *time.Time
	LastErrorCode     string
}

type CapabilityOperationResultInput struct {
	OperationID      int64
	LeaseToken       string
	Attempt          int
	TargetKind       string
	TargetID         string
	Status           string
	ProviderTargetID string
	ErrorCode        string
	ErrorMessageSafe string
}

func capabilityOperationCanBeClaimed(status string) bool {
	switch status {
	case wecomcapability.OperationPending, wecomcapability.OperationFailed, wecomcapability.OperationPartialFailed:
		return true
	case wecomcapability.OperationClaimed, wecomcapability.OperationSubmitting, wecomcapability.OperationSubmitted, wecomcapability.OperationPolling:
		return true
	default:
		return false
	}
}

func capabilityDispatchCanBeClaimed(status string) bool {
	switch status {
	case wecomcapability.DispatchQueued, wecomcapability.DispatchFailed, wecomcapability.DispatchPartialFailed:
		return true
	case wecomcapability.DispatchClaimed, wecomcapability.DispatchSubmitting, wecomcapability.DispatchSubmitted, wecomcapability.DispatchPolling:
		return true
	default:
		return false
	}
}

func capabilityOperationAllowsDispatch(status string) bool {
	switch status {
	case wecomcapability.OperationPending, wecomcapability.OperationClaimed, wecomcapability.OperationSubmitting, wecomcapability.OperationSubmitted, wecomcapability.OperationPolling:
		return true
	default:
		return false
	}
}

func ValidateCapabilityOperationInput(input CapabilityOperationInput) error {
	if _, ok := capabilityNameSet[input.Capability]; !ok {
		return fmt.Errorf("unknown capability")
	}
	if !wecomcapability.IsValidOperationActionForCapability(input.Capability, input.Action) {
		return fmt.Errorf("invalid capability action")
	}
	if !validLedgerToken(input.IdempotencyKey, 128) || input.TargetTotal < 0 {
		return fmt.Errorf("invalid operation identity or target count")
	}
	if input.ActorSource != "" && input.ActorSource != capabilityActorUser && input.ActorSource != capabilityActorSystem {
		return fmt.Errorf("invalid actor source")
	}
	return nil
}

func ValidateCapabilityDispatchInput(input CapabilityDispatchInput) error {
	if input.OperationID <= 0 || input.ChunkNo < 0 || !validLedgerToken(input.DispatchKind, 32) || !validLedgerToken(input.TargetID, 255) || !validLedgerToken(input.IdempotencyKey, 128) {
		return fmt.Errorf("invalid dispatch identity")
	}
	return nil
}

func validLedgerToken(value string, limit int) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= limit && !strings.ContainsAny(value, "\x00\r\n")
}

func validCapabilityMachineCode(value string, limit int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > limit {
		return value == ""
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '.' && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

var capabilityNameSet = func() map[string]struct{} {
	result := make(map[string]struct{}, len(wecomcapability.All))
	for _, capability := range wecomcapability.All {
		result[capability] = struct{}{}
	}
	return result
}()

func (s *MySQLStore) CreateCapabilityOperation(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input CapabilityOperationInput) (wecomcapability.Operation, error) {
	if s == nil || s.db == nil {
		return wecomcapability.Operation{}, companyprofile.ErrStoreUnavailable
	}
	if err := ValidateCapabilityOperationInput(input); err != nil {
		return wecomcapability.Operation{}, companyprofile.ErrInvalidRequest
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wecomcapability.Operation{}, companyprofile.ErrStoreUnavailable
	}
	defer rollbackQuietly(tx)
	actorUserID, actorSource, err := s.capabilityActor(ctx, tx, principal, input.ActorSource, true)
	if err != nil {
		return wecomcapability.Operation{}, err
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return wecomcapability.Operation{}, err
	}
	generation := credentialGenerationForCapability(binding, input.Capability)
	if generation == 0 {
		return wecomcapability.Operation{}, companyprofile.ErrStoreUnavailable
	}
	var operationID int64
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_wecom_capability_operations
		(tenant_id, corp_id, capability, action, credential_group, credential_generation,
		 idempotency_key, status, target_total, actor_user_id, actor_source, request_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id)`,
		binding.TenantID, binding.CorpID, input.Capability, input.Action,
		wecomcapability.CredentialGroupForCapability(input.Capability), generation,
		strings.TrimSpace(input.IdempotencyKey), wecomcapability.OperationPending,
		input.TargetTotal, actorUserID, actorSource, strings.TrimSpace(input.RequestID))
	if err != nil {
		return wecomcapability.Operation{}, err
	}
	operationID, err = result.LastInsertId()
	if err != nil || operationID <= 0 {
		return wecomcapability.Operation{}, ErrCapabilityOperationConflict
	}
	rowsAffected, rowsErr := result.RowsAffected()
	created := rowsErr == nil && rowsAffected == 1
	operation, err := queryCapabilityOperationTx(ctx, tx, binding.TenantID, binding.CorpID, operationID, false)
	if err != nil {
		return wecomcapability.Operation{}, err
	}
	if operation.CredentialVersion != generation || operation.Capability != input.Capability || operation.IdempotencyKey != strings.TrimSpace(input.IdempotencyKey) {
		return wecomcapability.Operation{}, ErrCapabilityOperationConflict
	}
	if created {
		if err := appendCapabilityLedgerTransitionTx(ctx, tx, operation, "", "create", actorUserID, actorSource, input.RequestID, nil); err != nil {
			return wecomcapability.Operation{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return wecomcapability.Operation{}, err
	}
	return operation, nil
}

func (s *MySQLStore) LatestCapabilityOperations(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, capabilities []string) (map[string]wecomcapability.Operation, error) {
	result := make(map[string]wecomcapability.Operation)
	if s == nil || s.db == nil {
		return nil, companyprofile.ErrStoreUnavailable
	}
	if principal.UserID > 0 {
		if err := s.checkCompanyActor(ctx, s.db, principal, false); err != nil {
			return nil, err
		}
	} else if _, err := s.loadCompanyBinding(ctx, s.db, principal, false); err != nil {
		return nil, err
	}
	allowed := make([]string, 0, len(capabilities))
	seen := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		if _, ok := capabilityNameSet[capability]; !ok {
			continue
		}
		if _, ok := seen[capability]; ok {
			continue
		}
		seen[capability] = struct{}{}
		allowed = append(allowed, capability)
	}
	if len(allowed) == 0 {
		return result, nil
	}
	placeholders := make([]string, len(allowed))
	args := make([]any, 0, len(allowed)+2)
	for index, capability := range allowed {
		placeholders[index] = "?"
		args = append(args, capability)
	}
	args = append(args, principal.TenantID, principal.CorpID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT o.id, o.tenant_id, o.corp_id, o.capability, o.action, o.credential_group,
		       o.credential_generation, o.idempotency_key, o.status, o.provider_request_id,
		       o.provider_object_id, o.actual_agent_id, o.external_success, o.callback_evidence,
		       o.target_total, o.success_total, o.failure_total, o.error_code, o.actor_user_id,
		       o.actor_source, o.request_id, o.lease_token, o.lease_expires_at, o.attempt,
		       o.requested_at, o.started_at, o.finished_at, o.created_at, o.updated_at
		FROM mochat_go_wecom_capability_operations o
		WHERE o.capability IN (`+strings.Join(placeholders, ",")+`) AND o.tenant_id=? AND o.corp_id=?
		ORDER BY o.capability ASC, o.id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		operation, err := scanCapabilityOperation(rows)
		if err != nil {
			return nil, err
		}
		if _, exists := result[operation.Capability]; !exists {
			result[operation.Capability] = operation
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *MySQLStore) ClaimCapabilityOperation(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, operationID int64, leaseDuration time.Duration) (wecomcapability.Operation, error) {
	if s == nil || s.db == nil || operationID <= 0 || leaseDuration <= 0 {
		return wecomcapability.Operation{}, companyprofile.ErrInvalidRequest
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wecomcapability.Operation{}, companyprofile.ErrStoreUnavailable
	}
	defer rollbackQuietly(tx)
	claimActorSource := capabilityActorUser
	if principal.UserID <= 0 {
		claimActorSource = capabilityActorSystem
	}
	actorUserID, actorSource, err := s.capabilityActor(ctx, tx, principal, claimActorSource, true)
	if err != nil {
		return wecomcapability.Operation{}, err
	}
	operation, err := queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, operationID, true)
	if err != nil {
		return wecomcapability.Operation{}, err
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return wecomcapability.Operation{}, err
	}
	if operation.CredentialVersion != credentialGenerationForCapability(binding, operation.Capability) {
		return wecomcapability.Operation{}, ErrCapabilityOperationStale
	}
	if !capabilityOperationCanBeClaimed(operation.Status) {
		return wecomcapability.Operation{}, ErrCapabilityOperationConflict
	}
	previousStatus := operation.Status
	leaseToken, err := newCapabilityLeaseToken()
	if err != nil {
		return wecomcapability.Operation{}, err
	}
	updated, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_wecom_capability_operations
		SET status=?, lease_token=?, lease_expires_at=DATE_ADD(NOW(6), INTERVAL ? MICROSECOND), attempt=attempt+1,
		    started_at=COALESCE(started_at,NOW(6)), updated_at=NOW(6)
		WHERE tenant_id=? AND corp_id=? AND id=? AND status=?
		  AND (status IN (?, ?, ?) OR lease_expires_at IS NULL OR lease_expires_at <= NOW(6))`,
		wecomcapability.OperationClaimed, leaseToken, leaseDuration.Microseconds(),
		principal.TenantID, principal.CorpID, operationID, operation.Status,
		wecomcapability.OperationPending, wecomcapability.OperationFailed, wecomcapability.OperationPartialFailed)
	if err != nil {
		return wecomcapability.Operation{}, err
	}
	if err := requireCompanyRows(updated, 1); err != nil {
		return wecomcapability.Operation{}, ErrCapabilityOperationStale
	}
	operation, err = queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, operationID, true)
	if err != nil {
		return wecomcapability.Operation{}, err
	}
	if err := appendCapabilityLedgerTransitionTx(ctx, tx, operation, previousStatus, "claim", actorUserID, actorSource, "", nil); err != nil {
		return wecomcapability.Operation{}, err
	}
	if err := tx.Commit(); err != nil {
		return wecomcapability.Operation{}, err
	}
	return operation, nil
}

func (s *MySQLStore) TransitionCapabilityOperation(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input CapabilityOperationTransitionInput) (wecomcapability.Operation, error) {
	if s == nil || s.db == nil || input.OperationID <= 0 || input.Attempt <= 0 || strings.TrimSpace(input.LeaseToken) == "" || !wecomcapability.IsValidOperationStatus(input.Status) {
		return wecomcapability.Operation{}, companyprofile.ErrInvalidRequest
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wecomcapability.Operation{}, companyprofile.ErrStoreUnavailable
	}
	defer rollbackQuietly(tx)
	transitionActorSource := capabilityActorUser
	if principal.UserID <= 0 {
		transitionActorSource = capabilityActorSystem
	}
	actorUserID, actorSource, err := s.capabilityActor(ctx, tx, principal, transitionActorSource, true)
	if err != nil {
		return wecomcapability.Operation{}, err
	}
	operation, err := queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, input.OperationID, true)
	if err != nil {
		return wecomcapability.Operation{}, err
	}
	if !wecomcapability.CanTransitionOperation(operation.Status, input.Status) {
		return wecomcapability.Operation{}, ErrCapabilityInvalidState
	}
	previousStatus := operation.Status
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return wecomcapability.Operation{}, err
	}
	if operation.CredentialVersion != credentialGenerationForCapability(binding, operation.Capability) {
		return wecomcapability.Operation{}, ErrCapabilityOperationStale
	}
	if operation.LeaseToken != input.LeaseToken || operation.Attempt != input.Attempt {
		return wecomcapability.Operation{}, ErrCapabilityOperationStale
	}
	if input.TargetTotal < 0 || input.SuccessTotal < 0 || input.FailureTotal < 0 || input.SuccessTotal+input.FailureTotal > input.TargetTotal {
		return wecomcapability.Operation{}, companyprofile.ErrInvalidRequest
	}
	finished := input.Status == wecomcapability.OperationSucceeded || input.Status == wecomcapability.OperationPartialFailed || input.Status == wecomcapability.OperationFailed || input.Status == wecomcapability.OperationCancelled
	errorCode := strings.TrimSpace(input.ErrorCode)
	if !validCapabilityMachineCode(errorCode, 96) {
		return wecomcapability.Operation{}, companyprofile.ErrInvalidRequest
	}
	if (input.Status == wecomcapability.OperationFailed || input.Status == wecomcapability.OperationPartialFailed || input.Status == wecomcapability.OperationCancelled) && errorCode == "" {
		return wecomcapability.Operation{}, companyprofile.ErrInvalidRequest
	}
	updated, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_wecom_capability_operations
		SET status=?, provider_request_id=?, provider_object_id=?, actual_agent_id=?,
		    external_success=?, callback_evidence=?, target_total=?, success_total=?, failure_total=?,
		    error_code=?, request_id=?, finished_at=CASE WHEN ? THEN NOW(6) ELSE finished_at END,
		    updated_at=NOW(6)
		WHERE tenant_id=? AND corp_id=? AND id=? AND lease_token=? AND attempt=?
		  AND lease_expires_at IS NOT NULL AND lease_expires_at > NOW(6)`,
		input.Status, strings.TrimSpace(input.ProviderRequestID), strings.TrimSpace(input.ProviderObjectID), strings.TrimSpace(input.ActualAgentID),
		input.ExternalSuccess, input.CallbackEvidence, input.TargetTotal, input.SuccessTotal, input.FailureTotal,
		errorCode, strings.TrimSpace(input.RequestID), finished,
		principal.TenantID, principal.CorpID, input.OperationID, input.LeaseToken, input.Attempt)
	if err != nil {
		return wecomcapability.Operation{}, err
	}
	if err := requireCompanyRows(updated, 1); err != nil {
		return wecomcapability.Operation{}, ErrCapabilityOperationStale
	}
	operation, err = queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, input.OperationID, true)
	if err != nil {
		return wecomcapability.Operation{}, err
	}
	if input.Status == wecomcapability.OperationSucceeded && !wecomcapability.IsCurrentOperationEvidence(operation, principal.TenantID, principal.CorpID, operation.Capability, operation.CredentialVersion) {
		return wecomcapability.Operation{}, companyprofile.ErrInvalidRequest
	}
	if err := appendCapabilityLedgerTransitionTx(ctx, tx, operation, previousStatus, "transition", actorUserID, actorSource, input.RequestID, nil); err != nil {
		return wecomcapability.Operation{}, err
	}
	if err := tx.Commit(); err != nil {
		return wecomcapability.Operation{}, err
	}
	return operation, nil
}

func (s *MySQLStore) CreateCapabilityDispatch(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input CapabilityDispatchInput) (wecomcapability.Dispatch, error) {
	if s == nil || s.db == nil {
		return wecomcapability.Dispatch{}, companyprofile.ErrStoreUnavailable
	}
	if err := ValidateCapabilityDispatchInput(input); err != nil {
		return wecomcapability.Dispatch{}, companyprofile.ErrInvalidRequest
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wecomcapability.Dispatch{}, companyprofile.ErrStoreUnavailable
	}
	defer rollbackQuietly(tx)
	dispatchActorSource := capabilityActorUser
	if principal.UserID <= 0 {
		dispatchActorSource = capabilityActorSystem
	}
	actorUserID, actorSource, err := s.capabilityActor(ctx, tx, principal, dispatchActorSource, true)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	operation, err := queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, input.OperationID, true)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	generation := credentialGenerationForCapability(binding, operation.Capability)
	if generation == 0 || operation.CredentialVersion != generation {
		return wecomcapability.Dispatch{}, ErrCapabilityOperationStale
	}
	if !capabilityOperationAllowsDispatch(operation.Status) {
		return wecomcapability.Dispatch{}, ErrCapabilityInvalidState
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_wecom_capability_dispatches
		(tenant_id,corp_id,operation_id,dispatch_kind,chunk_no,target_id,idempotency_key,status,credential_generation)
		VALUES (?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)`,
		principal.TenantID, principal.CorpID, input.OperationID, input.DispatchKind, input.ChunkNo,
		input.TargetID, input.IdempotencyKey, wecomcapability.DispatchQueued, generation)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	dispatchID, err := result.LastInsertId()
	if err != nil || dispatchID <= 0 {
		return wecomcapability.Dispatch{}, ErrCapabilityOperationConflict
	}
	dispatch, err := queryCapabilityDispatchTx(ctx, tx, principal.TenantID, principal.CorpID, dispatchID, true)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	if dispatch.OperationID != input.OperationID || dispatch.TargetID != strings.TrimSpace(input.TargetID) || dispatch.IdempotencyKey != strings.TrimSpace(input.IdempotencyKey) || dispatch.CredentialVersion != generation {
		return wecomcapability.Dispatch{}, ErrCapabilityOperationConflict
	}
	rowsAffected, rowsErr := result.RowsAffected()
	if rowsErr == nil && rowsAffected == 1 {
		if err := appendCapabilityLedgerTransitionTx(ctx, tx, operation, operation.Status, "dispatch_enqueue", actorUserID, actorSource, "", &dispatchID); err != nil {
			return wecomcapability.Dispatch{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return wecomcapability.Dispatch{}, err
	}
	return dispatch, nil
}

func (s *MySQLStore) ClaimCapabilityDispatch(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, dispatchID int64, leaseDuration time.Duration) (wecomcapability.Dispatch, error) {
	if s == nil || s.db == nil || dispatchID <= 0 || leaseDuration <= 0 {
		return wecomcapability.Dispatch{}, companyprofile.ErrInvalidRequest
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wecomcapability.Dispatch{}, companyprofile.ErrStoreUnavailable
	}
	defer rollbackQuietly(tx)
	actorSource := capabilityActorUser
	if principal.UserID <= 0 {
		actorSource = capabilityActorSystem
	}
	actorUserID, actorSource, err := s.capabilityActor(ctx, tx, principal, actorSource, true)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	// Resolve the parent through the child row under the same lock. This keeps
	// the tenant/corp scope and generation fence in one transaction.
	var operationID int64
	if err := tx.QueryRowContext(ctx, `SELECT operation_id FROM mochat_go_wecom_capability_dispatches WHERE tenant_id=? AND corp_id=? AND id=? FOR UPDATE`, principal.TenantID, principal.CorpID, dispatchID).Scan(&operationID); err != nil {
		return wecomcapability.Dispatch{}, err
	}
	operation, err := queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, operationID, true)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	if !capabilityOperationAllowsDispatch(operation.Status) {
		return wecomcapability.Dispatch{}, ErrCapabilityInvalidState
	}
	dispatch, err := queryCapabilityDispatchTx(ctx, tx, principal.TenantID, principal.CorpID, dispatchID, true)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	generation := credentialGenerationForCapability(binding, operation.Capability)
	if generation == 0 || operation.CredentialVersion != generation || dispatch.CredentialVersion != generation {
		return wecomcapability.Dispatch{}, ErrCapabilityOperationStale
	}
	if !capabilityDispatchCanBeClaimed(dispatch.Status) {
		return wecomcapability.Dispatch{}, ErrCapabilityInvalidState
	}
	leaseToken, err := newCapabilityLeaseToken()
	if err != nil {
		return wecomcapability.Dispatch{}, companyprofile.ErrStoreUnavailable
	}
	previousStatus := dispatch.Status
	updated, err := tx.ExecContext(ctx, `UPDATE mochat_go_wecom_capability_dispatches SET status=?, lease_token=?, lease_expires_at=DATE_ADD(NOW(6), INTERVAL ? MICROSECOND), attempt=attempt+1, updated_at=NOW(6) WHERE tenant_id=? AND corp_id=? AND id=? AND credential_generation=? AND status=? AND (status IN (?, ?, ?) OR lease_expires_at IS NULL OR lease_expires_at <= NOW(6))`, wecomcapability.DispatchClaimed, leaseToken, leaseDuration.Microseconds(), principal.TenantID, principal.CorpID, dispatchID, generation, dispatch.Status, wecomcapability.DispatchQueued, wecomcapability.DispatchFailed, wecomcapability.DispatchPartialFailed)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	if err := requireCompanyRows(updated, 1); err != nil {
		return wecomcapability.Dispatch{}, ErrCapabilityOperationStale
	}
	dispatch, err = queryCapabilityDispatchTx(ctx, tx, principal.TenantID, principal.CorpID, dispatchID, true)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	auditOperation := operation
	auditOperation.Status = dispatch.Status
	if err := appendCapabilityLedgerTransitionTx(ctx, tx, auditOperation, previousStatus, "dispatch_claim", actorUserID, actorSource, "", &dispatchID); err != nil {
		return wecomcapability.Dispatch{}, err
	}
	if err := tx.Commit(); err != nil {
		return wecomcapability.Dispatch{}, err
	}
	return dispatch, nil
}

func (s *MySQLStore) TransitionCapabilityDispatch(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input CapabilityDispatchTransitionInput) (wecomcapability.Dispatch, error) {
	if s == nil || s.db == nil || input.DispatchID <= 0 || !wecomcapability.IsValidDispatchStatus(input.Status) || input.Attempt <= 0 || !validLedgerToken(input.LeaseToken, 128) {
		return wecomcapability.Dispatch{}, companyprofile.ErrInvalidRequest
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wecomcapability.Dispatch{}, companyprofile.ErrStoreUnavailable
	}
	defer rollbackQuietly(tx)
	actorSource := capabilityActorUser
	if principal.UserID <= 0 {
		actorSource = capabilityActorSystem
	}
	actorUserID, actorSource, err := s.capabilityActor(ctx, tx, principal, actorSource, true)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	dispatch, err := queryCapabilityDispatchTx(ctx, tx, principal.TenantID, principal.CorpID, input.DispatchID, true)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	if dispatch.LeaseToken != input.LeaseToken || dispatch.Attempt != input.Attempt || !wecomcapability.CanTransitionDispatch(dispatch.Status, input.Status) {
		return wecomcapability.Dispatch{}, ErrCapabilityOperationStale
	}
	var operationID int64
	if err := tx.QueryRowContext(ctx, `SELECT operation_id FROM mochat_go_wecom_capability_dispatches WHERE tenant_id=? AND corp_id=? AND id=? FOR UPDATE`, principal.TenantID, principal.CorpID, input.DispatchID).Scan(&operationID); err != nil {
		return wecomcapability.Dispatch{}, err
	}
	operation, err := queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, operationID, true)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	if !capabilityOperationAllowsDispatch(operation.Status) {
		return wecomcapability.Dispatch{}, ErrCapabilityInvalidState
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	generation := credentialGenerationForCapability(binding, operation.Capability)
	if generation == 0 || dispatch.CredentialVersion != generation || operation.CredentialVersion != generation {
		return wecomcapability.Dispatch{}, ErrCapabilityOperationStale
	}
	errorCode := strings.TrimSpace(input.LastErrorCode)
	if !validCapabilityMachineCode(errorCode, 96) {
		return wecomcapability.Dispatch{}, companyprofile.ErrInvalidRequest
	}
	if (input.Status == wecomcapability.DispatchFailed || input.Status == wecomcapability.DispatchPartialFailed) && errorCode == "" {
		return wecomcapability.Dispatch{}, companyprofile.ErrInvalidRequest
	}
	if input.Status == wecomcapability.DispatchSucceeded && strings.TrimSpace(input.ProviderRequestID) == "" && strings.TrimSpace(input.ProviderMessageID) == "" && strings.TrimSpace(input.ProviderObjectID) == "" {
		return wecomcapability.Dispatch{}, companyprofile.ErrInvalidRequest
	}
	previousStatus := dispatch.Status
	finished := input.Status == wecomcapability.DispatchSucceeded || input.Status == wecomcapability.DispatchPartialFailed || input.Status == wecomcapability.DispatchFailed
	updated, err := tx.ExecContext(ctx, `UPDATE mochat_go_wecom_capability_dispatches SET status=?, provider_request_id=?, provider_message_id=?, provider_object_id=?, next_poll_at=?, last_error_code=?, lease_expires_at=CASE WHEN ? THEN NULL ELSE lease_expires_at END, updated_at=NOW(6) WHERE tenant_id=? AND corp_id=? AND id=? AND lease_token=? AND attempt=? AND lease_expires_at IS NOT NULL AND lease_expires_at > NOW(6)`, input.Status, strings.TrimSpace(input.ProviderRequestID), strings.TrimSpace(input.ProviderMessageID), strings.TrimSpace(input.ProviderObjectID), input.NextPollAt, errorCode, finished, principal.TenantID, principal.CorpID, input.DispatchID, input.LeaseToken, input.Attempt)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	if err := requireCompanyRows(updated, 1); err != nil {
		return wecomcapability.Dispatch{}, ErrCapabilityOperationStale
	}
	dispatch, err = queryCapabilityDispatchTx(ctx, tx, principal.TenantID, principal.CorpID, input.DispatchID, true)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	auditOperation := operation
	auditOperation.Status = dispatch.Status
	auditOperation.ErrorCode = errorCode
	if err := appendCapabilityLedgerTransitionTx(ctx, tx, auditOperation, previousStatus, "dispatch_transition", actorUserID, actorSource, "", &input.DispatchID); err != nil {
		return wecomcapability.Dispatch{}, err
	}
	if err := tx.Commit(); err != nil {
		return wecomcapability.Dispatch{}, err
	}
	return dispatch, nil
}

func (s *MySQLStore) RecordCapabilityOperationResult(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input CapabilityOperationResultInput) (wecomcapability.OperationResult, error) {
	if s == nil || s.db == nil || input.OperationID <= 0 || input.Attempt <= 0 || !validLedgerToken(input.LeaseToken, 128) || !validLedgerToken(input.TargetKind, 32) || !validLedgerToken(input.TargetID, 255) || !wecomcapability.IsValidOperationResultStatus(input.Status) || !validCapabilityMachineCode(input.ErrorCode, 96) || !validCapabilityMachineCode(input.ErrorMessageSafe, 255) {
		return wecomcapability.OperationResult{}, companyprofile.ErrInvalidRequest
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wecomcapability.OperationResult{}, companyprofile.ErrStoreUnavailable
	}
	defer rollbackQuietly(tx)
	resultActorSource := capabilityActorUser
	if principal.UserID <= 0 {
		resultActorSource = capabilityActorSystem
	}
	actorUserID, actorSource, err := s.capabilityActor(ctx, tx, principal, resultActorSource, true)
	if err != nil {
		return wecomcapability.OperationResult{}, err
	}
	operation, err := queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, input.OperationID, true)
	if err != nil {
		return wecomcapability.OperationResult{}, err
	}
	if operation.LeaseToken != input.LeaseToken || operation.Attempt != input.Attempt {
		return wecomcapability.OperationResult{}, ErrCapabilityOperationStale
	}
	var activeLeaseCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mochat_go_wecom_capability_operations
		WHERE tenant_id=? AND corp_id=? AND id=? AND lease_token=? AND attempt=?
		  AND lease_expires_at IS NOT NULL AND lease_expires_at > NOW(6)`,
		principal.TenantID, principal.CorpID, input.OperationID, input.LeaseToken, input.Attempt).Scan(&activeLeaseCount); err != nil {
		return wecomcapability.OperationResult{}, err
	}
	if activeLeaseCount != 1 {
		return wecomcapability.OperationResult{}, ErrCapabilityOperationStale
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return wecomcapability.OperationResult{}, err
	}
	if operation.CredentialVersion == 0 || operation.CredentialVersion != credentialGenerationForCapability(binding, operation.Capability) {
		return wecomcapability.OperationResult{}, ErrCapabilityOperationStale
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_wecom_capability_operation_results
		(tenant_id,corp_id,operation_id,target_kind,target_id,status,provider_target_id,error_code,error_message_safe)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE status=VALUES(status), provider_target_id=VALUES(provider_target_id), error_code=VALUES(error_code), error_message_safe=VALUES(error_message_safe), updated_at=NOW(6)`,
		principal.TenantID, principal.CorpID, input.OperationID, input.TargetKind, input.TargetID, input.Status,
		input.ProviderTargetID, input.ErrorCode, input.ErrorMessageSafe); err != nil {
		return wecomcapability.OperationResult{}, err
	}
	if err := appendCapabilityLedgerTransitionTx(ctx, tx, operation, operation.Status, "result", actorUserID, actorSource, "", nil); err != nil {
		return wecomcapability.OperationResult{}, err
	}
	var result wecomcapability.OperationResult
	var createdAt, updatedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT id,tenant_id,corp_id,operation_id,target_kind,target_id,status,provider_target_id,error_code,error_message_safe,created_at,updated_at
		FROM mochat_go_wecom_capability_operation_results WHERE tenant_id=? AND corp_id=? AND operation_id=? AND target_kind=? AND target_id=?`,
		principal.TenantID, principal.CorpID, input.OperationID, input.TargetKind, input.TargetID).Scan(
		&result.ID, &result.TenantID, &result.CorpID, &result.OperationID, &result.TargetKind, &result.TargetID,
		&result.Status, &result.ProviderTargetID, &result.ErrorCode, &result.ErrorMessageSafe, &createdAt, &updatedAt); err != nil {
		return wecomcapability.OperationResult{}, err
	}
	result.CreatedAt, result.UpdatedAt = nullableTimePtr(createdAt), nullableTimePtr(updatedAt)
	if err := tx.Commit(); err != nil {
		return wecomcapability.OperationResult{}, err
	}
	return result, nil
}

func (s *MySQLStore) capabilityActor(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal, requestedSource string, forUpdate bool) (*int, string, error) {
	source, err := capabilityActorSourceForPrincipal(principal.UserID, requestedSource)
	if err != nil {
		return nil, "", err
	}
	if source == capabilityActorSystem {
		if _, err := s.loadCompanyBinding(ctx, tx, principal, forUpdate); err != nil {
			return nil, "", err
		}
		return nil, source, nil
	}
	if source != capabilityActorUser {
		return nil, "", companyprofile.ErrInvalidRequest
	}
	if err := s.checkCompanyActor(ctx, tx, principal, forUpdate); err != nil {
		return nil, "", err
	}
	actor := principal.UserID
	if actor <= 0 {
		return nil, "", companyprofile.ErrPermissionDenied
	}
	return &actor, source, nil
}

func capabilityActorSourceForPrincipal(userID int, requestedSource string) (string, error) {
	source := strings.TrimSpace(requestedSource)
	if source == "" {
		if userID <= 0 {
			source = capabilityActorSystem
		} else {
			source = capabilityActorUser
		}
	}
	if userID <= 0 && source != capabilityActorSystem {
		return "", companyprofile.ErrPermissionDenied
	}
	if userID > 0 && source != capabilityActorUser {
		return "", companyprofile.ErrPermissionDenied
	}
	return source, nil
}

func credentialGenerationForCapability(binding companyBindingRecord, capability string) uint64 {
	switch wecomcapability.CredentialGroupForCapability(capability) {
	case wecomcapability.CredentialGroupEmployee:
		return binding.EmployeeGeneration
	case wecomcapability.CredentialGroupContact:
		return binding.ContactGeneration
	case wecomcapability.CredentialGroupAgent:
		return binding.AgentGeneration
	case wecomcapability.CredentialGroupCallback:
		return binding.CallbackGeneration
	default:
		return 0
	}
}

func newCapabilityLeaseToken() (string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func appendCapabilityLedgerTransitionTx(ctx context.Context, tx *sql.Tx, operation wecomcapability.Operation, fromStatus, action string, actorUserID *int, actorSource, requestID string, dispatchID *int64) error {
	var actor any
	if actorUserID != nil {
		actor = *actorUserID
	}
	var dispatch any
	if dispatchID != nil {
		dispatch = *dispatchID
	}
	args := []any{operation.TenantID, operation.CorpID, operation.ID, dispatch, fromStatus, operation.Status, action, actor, actorSource, strings.TrimSpace(requestID), operation.ErrorCode, operation.TargetTotal, operation.SuccessTotal, operation.FailureTotal}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_wecom_capability_operation_audits
		(tenant_id,corp_id,operation_id,dispatch_id,from_status,to_status,action,actor_user_id,actor_source,request_id,error_code,target_total,success_total,failure_total)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, args...); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_wecom_capability_operation_events
		(tenant_id,corp_id,operation_id,dispatch_id,from_status,to_status,action,actor_user_id,actor_source,request_id,error_code,target_total,success_total,failure_total)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, args...)
	return err
}

type capabilityOperationScanner interface {
	Scan(...any) error
}

func queryCapabilityOperationTx(ctx context.Context, tx *sql.Tx, tenantID, corpID int, operationID int64, forUpdate bool) (wecomcapability.Operation, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	return scanCapabilityOperationRow(tx.QueryRowContext(ctx, `
		SELECT id,tenant_id,corp_id,capability,action,credential_group,credential_generation,idempotency_key,status,
		       provider_request_id,provider_object_id,actual_agent_id,external_success,callback_evidence,
		       target_total,success_total,failure_total,error_code,actor_user_id,actor_source,request_id,lease_token,
		       lease_expires_at,attempt,requested_at,started_at,finished_at,created_at,updated_at
		FROM mochat_go_wecom_capability_operations WHERE tenant_id=? AND corp_id=? AND id=?`+suffix, tenantID, corpID, operationID))
}

func scanCapabilityOperation(scanner capabilityOperationScanner) (wecomcapability.Operation, error) {
	return scanCapabilityOperationRow(scanner)
}

func scanCapabilityOperationRow(scanner capabilityOperationScanner) (wecomcapability.Operation, error) {
	var operation wecomcapability.Operation
	var action, group string
	var actor sql.NullInt64
	var leaseExpires, requested, started, finished, created, updated sql.NullTime
	if err := scanner.Scan(&operation.ID, &operation.TenantID, &operation.CorpID, &operation.Capability, &action, &group,
		&operation.CredentialVersion, &operation.IdempotencyKey, &operation.Status, &operation.ProviderRequestID,
		&operation.ProviderObjectID, &operation.ActualAgentID, &operation.ExternalSuccess, &operation.CallbackEvidence,
		&operation.TargetTotal, &operation.SuccessTotal, &operation.FailureTotal, &operation.ErrorCode, &actor,
		&operation.ActorSource, &operation.RequestID, &operation.LeaseToken, &leaseExpires, &operation.Attempt,
		&requested, &started, &finished, &created, &updated); err != nil {
		return wecomcapability.Operation{}, err
	}
	operation.Action = wecomcapability.OperationAction(action)
	operation.CredentialGroup = wecomcapability.CredentialGroup(group)
	if actor.Valid {
		operation.ActorUserID = int(actor.Int64)
	}
	operation.LeaseExpiresAt = nullableTimePtr(leaseExpires)
	operation.RequestedAt, operation.StartedAt = nullableTimePtr(requested), nullableTimePtr(started)
	operation.FinishedAt, operation.CreatedAt, operation.UpdatedAt = nullableTimePtr(finished), nullableTimePtr(created), nullableTimePtr(updated)
	return operation, nil
}

func queryCapabilityDispatchTx(ctx context.Context, tx *sql.Tx, tenantID, corpID int, dispatchID int64, forUpdate bool) (wecomcapability.Dispatch, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	var item wecomcapability.Dispatch
	var leaseExpires, nextPoll, created, updated sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT id,tenant_id,corp_id,operation_id,dispatch_kind,chunk_no,target_id,idempotency_key,status,
		       provider_request_id,provider_message_id,provider_object_id,credential_generation,lease_token,
		       lease_expires_at,attempt,next_poll_at,last_error_code,created_at,updated_at
		FROM mochat_go_wecom_capability_dispatches WHERE tenant_id=? AND corp_id=? AND id=?`+suffix, tenantID, corpID, dispatchID).Scan(
		&item.ID, &item.TenantID, &item.CorpID, &item.OperationID, &item.DispatchKind, &item.ChunkNo, &item.TargetID,
		&item.IdempotencyKey, &item.Status, &item.ProviderRequestID, &item.ProviderMessageID, &item.ProviderObjectID,
		&item.CredentialVersion, &item.LeaseToken, &leaseExpires, &item.Attempt, &nextPoll, &item.LastErrorCode, &created, &updated); err != nil {
		return wecomcapability.Dispatch{}, err
	}
	item.LeaseExpiresAt, item.NextPollAt = nullableTimePtr(leaseExpires), nullableTimePtr(nextPoll)
	item.CreatedAt, item.UpdatedAt = nullableTimePtr(created), nullableTimePtr(updated)
	return item, nil
}

func nullableTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}
