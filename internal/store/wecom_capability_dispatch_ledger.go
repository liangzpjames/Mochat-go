package store

import (
	"context"
	"database/sql"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

// ClaimDispatch/TransitionDispatch/RecordDispatchResult expose the durable
// boundary used by wecomcapability.DispatchRunner. They intentionally wrap
// the already-approved 0139 ledger methods rather than creating a parallel
// persistence model.
func (s *MySQLStore) ClaimDispatch(ctx context.Context, request wecomcapability.DispatchClaimRequest) (wecomcapability.Dispatch, error) {
	if request.ExpectedCredentialVersion == 0 {
		return wecomcapability.Dispatch{}, ErrCapabilityOperationStale
	}
	dispatch, err := s.ClaimCapabilityDispatchAtGeneration(ctx, capabilityDashboardPrincipal(request.Principal), request.DispatchID, request.LeaseDuration, request.ExpectedCredentialVersion)
	if err != nil {
		return wecomcapability.Dispatch{}, err
	}
	if dispatch.CredentialVersion != request.ExpectedCredentialVersion {
		return wecomcapability.Dispatch{}, ErrCapabilityOperationStale
	}
	return dispatch, nil
}

func (s *MySQLStore) TransitionDispatch(ctx context.Context, request wecomcapability.DispatchTransitionRequest) (wecomcapability.Dispatch, error) {
	return s.TransitionCapabilityDispatch(ctx, capabilityDashboardPrincipal(request.Principal), CapabilityDispatchTransitionInput{
		DispatchID: request.DispatchID, Status: request.Status, LeaseToken: request.LeaseToken, Attempt: request.Attempt,
		ProviderRequestID: request.ProviderRequestID, ProviderMessageID: request.ProviderMessageID,
		ProviderObjectID: request.ProviderObjectID, NextPollAt: request.NextPollAt, LastErrorCode: request.LastErrorCode,
	})
}

func (s *MySQLStore) RecordDispatchResult(ctx context.Context, request wecomcapability.DispatchResultRequest) (wecomcapability.OperationResult, error) {
	return s.RecordCapabilityDispatchResult(ctx, capabilityDashboardPrincipal(request.Principal), CapabilityDispatchResultInput{
		DispatchID: request.DispatchID, LeaseToken: request.LeaseToken, Attempt: request.Attempt,
		TargetKind: request.TargetKind, TargetID: request.TargetID, Status: request.Status,
		ProviderTargetID: request.ProviderTargetID, ErrorCode: request.ErrorCode, ErrorMessageSafe: request.ErrorMessageSafe,
	})
}

func (s *MySQLStore) AggregateOperation(ctx context.Context, request wecomcapability.OperationAggregateRequest) (wecomcapability.OperationAggregate, error) {
	if s == nil || s.db == nil || request.OperationID <= 0 || request.Principal.TenantID <= 0 || request.Principal.CorpID <= 0 || request.ExpectedCredentialVersion == 0 {
		return wecomcapability.OperationAggregate{}, companyprofile.ErrInvalidRequest
	}
	if request.Principal.UserID > 0 {
		if err := s.checkCompanyActor(ctx, s.db, capabilityDashboardPrincipal(request.Principal), false); err != nil {
			return wecomcapability.OperationAggregate{}, err
		}
	} else if _, err := s.loadCompanyBinding(ctx, s.db, capabilityDashboardPrincipal(request.Principal), false); err != nil {
		return wecomcapability.OperationAggregate{}, err
	}
	operation, err := queryCapabilityOperationDB(ctx, s.db, request.Principal.TenantID, request.Principal.CorpID, request.OperationID)
	if err != nil {
		return wecomcapability.OperationAggregate{}, err
	}
	if operation.Capability != request.Capability || operation.CredentialVersion != request.ExpectedCredentialVersion {
		return wecomcapability.OperationAggregate{}, ErrCapabilityOperationStale
	}
	dispatches, err := queryCapabilityDispatchesDB(ctx, s.db, request.Principal.TenantID, request.Principal.CorpID, request.OperationID)
	if err != nil {
		return wecomcapability.OperationAggregate{}, err
	}
	results, err := queryCapabilityResultsDB(ctx, s.db, request.Principal.TenantID, request.Principal.CorpID, request.OperationID)
	if err != nil {
		return wecomcapability.OperationAggregate{}, err
	}
	return wecomcapability.OperationAggregate{
		OperationID: request.OperationID,
		Capability:  operation.Capability,
		Status:      wecomcapability.AggregateOperationStatusWithResults(operation.Capability, dispatches, results),
		Dispatches:  dispatches,
		Results:     results,
	}, nil
}

func (s *MySQLStore) RecordCapabilityDispatchResult(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input CapabilityDispatchResultInput) (wecomcapability.OperationResult, error) {
	if s == nil || s.db == nil || input.DispatchID <= 0 || input.Attempt <= 0 || !validLedgerToken(input.LeaseToken, 128) || !validLedgerToken(input.TargetKind, 32) || !validLedgerToken(input.TargetID, 255) || !wecomcapability.IsValidOperationResultStatus(input.Status) || !validCapabilityMachineCode(input.ErrorCode, 96) || !validSafeCapabilityText(input.ErrorMessageSafe, 255) {
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
	dispatch, err := queryCapabilityDispatchTx(ctx, tx, principal.TenantID, principal.CorpID, input.DispatchID, true)
	if err != nil {
		return wecomcapability.OperationResult{}, err
	}
	if !dispatchResultStateAllowsWrite(dispatch.Status) || dispatch.LeaseToken != input.LeaseToken || dispatch.Attempt != input.Attempt {
		return wecomcapability.OperationResult{}, ErrCapabilityOperationStale
	}
	var activeLeaseCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mochat_go_wecom_capability_dispatches
		WHERE tenant_id=? AND corp_id=? AND id=? AND lease_token=? AND attempt=?
		  AND lease_expires_at IS NOT NULL AND lease_expires_at > NOW(6)`,
		principal.TenantID, principal.CorpID, input.DispatchID, input.LeaseToken, input.Attempt).Scan(&activeLeaseCount); err != nil {
		return wecomcapability.OperationResult{}, err
	}
	if activeLeaseCount != 1 {
		return wecomcapability.OperationResult{}, ErrCapabilityOperationStale
	}
	var operationID int64
	if err := tx.QueryRowContext(ctx, `SELECT operation_id FROM mochat_go_wecom_capability_dispatches WHERE tenant_id=? AND corp_id=? AND id=? FOR UPDATE`, principal.TenantID, principal.CorpID, input.DispatchID).Scan(&operationID); err != nil {
		return wecomcapability.OperationResult{}, err
	}
	operation, err := queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, operationID, true)
	if err != nil {
		return wecomcapability.OperationResult{}, err
	}
	if !capabilityOperationAllowsDispatch(operation.Status) {
		return wecomcapability.OperationResult{}, ErrCapabilityInvalidState
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return wecomcapability.OperationResult{}, err
	}
	generation := credentialGenerationForCapability(binding, operation.Capability)
	if generation == 0 || dispatch.CredentialVersion != generation || operation.CredentialVersion != generation {
		return wecomcapability.OperationResult{}, ErrCapabilityOperationStale
	}

	var existingStatus string
	existingErr := tx.QueryRowContext(ctx, `SELECT status FROM mochat_go_wecom_capability_operation_results WHERE tenant_id=? AND corp_id=? AND operation_id=? AND target_kind=? AND target_id=? FOR UPDATE`, principal.TenantID, principal.CorpID, operationID, input.TargetKind, input.TargetID).Scan(&existingStatus)
	if existingErr != nil && existingErr != sql.ErrNoRows {
		return wecomcapability.OperationResult{}, existingErr
	}
	if existingErr == nil && existingStatus == wecomcapability.DispatchSucceeded {
		result, err := queryCapabilityResultTx(ctx, tx, principal.TenantID, principal.CorpID, operationID, input.TargetKind, input.TargetID)
		if err != nil {
			return wecomcapability.OperationResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return wecomcapability.OperationResult{}, err
		}
		return result, nil
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_wecom_capability_operation_results
		(tenant_id,corp_id,operation_id,target_kind,target_id,status,provider_target_id,error_code,error_message_safe)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE status=VALUES(status), provider_target_id=VALUES(provider_target_id), error_code=VALUES(error_code), error_message_safe=VALUES(error_message_safe), updated_at=NOW(6)`,
		principal.TenantID, principal.CorpID, operationID, input.TargetKind, input.TargetID, input.Status, input.ProviderTargetID, input.ErrorCode, input.ErrorMessageSafe); err != nil {
		return wecomcapability.OperationResult{}, err
	}
	if err := appendCapabilityLedgerTransitionTx(ctx, tx, operation, operation.Status, "dispatch_result", actorUserID, actorSource, "", &input.DispatchID); err != nil {
		return wecomcapability.OperationResult{}, err
	}
	result, err := queryCapabilityResultTx(ctx, tx, principal.TenantID, principal.CorpID, operationID, input.TargetKind, input.TargetID)
	if err != nil {
		return wecomcapability.OperationResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return wecomcapability.OperationResult{}, err
	}
	return result, nil
}

func dispatchResultStateAllowsWrite(status string) bool {
	switch status {
	case wecomcapability.DispatchClaimed, wecomcapability.DispatchSubmitting, wecomcapability.DispatchSubmitted, wecomcapability.DispatchPolling:
		return true
	default:
		return false
	}
}

func capabilityDashboardPrincipal(principal wecomcapability.DispatchPrincipal) dashboardprincipal.DashboardPrincipal {
	return dashboardprincipal.DashboardPrincipal{
		UserID: principal.UserID, TenantID: principal.TenantID, CorpID: principal.CorpID,
		CorpStatus: dashboardprincipal.CorpBindingStatusActive, IsSuperAdmin: principal.IsSuperAdmin,
		AuthVersion: principal.AuthVersion,
	}
}

func queryCapabilityOperationDB(ctx context.Context, db *sql.DB, tenantID, corpID int, operationID int64) (wecomcapability.Operation, error) {
	return scanCapabilityOperation(db.QueryRowContext(ctx, `
		SELECT id,tenant_id,corp_id,capability,action,credential_group,credential_generation,idempotency_key,status,
		       provider_request_id,provider_object_id,actual_agent_id,external_success,callback_evidence,
		       target_total,success_total,failure_total,error_code,actor_user_id,actor_source,request_id,lease_token,
		       lease_expires_at,attempt,requested_at,started_at,finished_at,created_at,updated_at
		FROM mochat_go_wecom_capability_operations WHERE tenant_id=? AND corp_id=? AND id=?`, tenantID, corpID, operationID))
}

func queryCapabilityDispatchesDB(ctx context.Context, db *sql.DB, tenantID, corpID int, operationID int64) ([]wecomcapability.Dispatch, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id,tenant_id,corp_id,operation_id,dispatch_kind,chunk_no,target_id,idempotency_key,status,
		       provider_request_id,provider_message_id,provider_object_id,credential_generation,lease_token,
		       lease_expires_at,attempt,next_poll_at,last_error_code,created_at,updated_at
		FROM mochat_go_wecom_capability_dispatches WHERE tenant_id=? AND corp_id=? AND operation_id=? ORDER BY chunk_no ASC,id ASC`, tenantID, corpID, operationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]wecomcapability.Dispatch, 0)
	for rows.Next() {
		item, err := scanCapabilityDispatch(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func scanCapabilityDispatch(scanner capabilityOperationScanner) (wecomcapability.Dispatch, error) {
	var item wecomcapability.Dispatch
	var leaseExpires, nextPoll, created, updated sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.TenantID, &item.CorpID, &item.OperationID, &item.DispatchKind, &item.ChunkNo, &item.TargetID,
		&item.IdempotencyKey, &item.Status, &item.ProviderRequestID, &item.ProviderMessageID, &item.ProviderObjectID,
		&item.CredentialVersion, &item.LeaseToken, &leaseExpires, &item.Attempt, &nextPoll, &item.LastErrorCode, &created, &updated,
	); err != nil {
		return wecomcapability.Dispatch{}, err
	}
	item.LeaseExpiresAt, item.NextPollAt = nullableTimePtr(leaseExpires), nullableTimePtr(nextPoll)
	item.CreatedAt, item.UpdatedAt = nullableTimePtr(created), nullableTimePtr(updated)
	return item, nil
}

func queryCapabilityResultTx(ctx context.Context, tx *sql.Tx, tenantID, corpID int, operationID int64, targetKind, targetID string) (wecomcapability.OperationResult, error) {
	return scanCapabilityResult(tx.QueryRowContext(ctx, `
		SELECT id,tenant_id,corp_id,operation_id,target_kind,target_id,status,provider_target_id,error_code,error_message_safe,created_at,updated_at
		FROM mochat_go_wecom_capability_operation_results WHERE tenant_id=? AND corp_id=? AND operation_id=? AND target_kind=? AND target_id=?`, tenantID, corpID, operationID, targetKind, targetID))
}

func queryCapabilityResultsDB(ctx context.Context, db *sql.DB, tenantID, corpID int, operationID int64) ([]wecomcapability.OperationResult, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id,tenant_id,corp_id,operation_id,target_kind,target_id,status,provider_target_id,error_code,error_message_safe,created_at,updated_at
		FROM mochat_go_wecom_capability_operation_results WHERE tenant_id=? AND corp_id=? AND operation_id=? ORDER BY id ASC`, tenantID, corpID, operationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]wecomcapability.OperationResult, 0)
	for rows.Next() {
		item, err := scanCapabilityResult(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func scanCapabilityResult(scanner capabilityOperationScanner) (wecomcapability.OperationResult, error) {
	var result wecomcapability.OperationResult
	var created, updated sql.NullTime
	if err := scanner.Scan(&result.ID, &result.TenantID, &result.CorpID, &result.OperationID, &result.TargetKind, &result.TargetID, &result.Status, &result.ProviderTargetID, &result.ErrorCode, &result.ErrorMessageSafe, &created, &updated); err != nil {
		return wecomcapability.OperationResult{}, err
	}
	result.CreatedAt, result.UpdatedAt = nullableTimePtr(created), nullableTimePtr(updated)
	return result, nil
}

var _ wecomcapability.DispatchLedger = (*MySQLStore)(nil)
