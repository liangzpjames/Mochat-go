package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

const saasPaymentSettlementQueryMaxLimit = 5000

func (s *MySQLStore) SaaSAdminPaymentSettlementBatches(ctx context.Context, options dashboard.SaaSPaymentSettlementBatchOptions) (dashboard.SaaSPaymentSettlementBatchReport, error) {
	where, args := saasPaymentSettlementBatchWhere(options)
	rows, err := s.db.QueryContext(ctx, saasPaymentSettlementBatchSelectSQL()+where+` ORDER BY b.imported_at DESC, b.id DESC LIMIT ?`, append(args, saasPaymentSettlementQueryMaxLimit)...)
	if err != nil {
		return dashboard.SaaSPaymentSettlementBatchReport{}, err
	}
	defer rows.Close()
	all := make([]dashboard.SaaSPaymentSettlementBatch, 0)
	for rows.Next() {
		item, err := scanSaaSPaymentSettlementBatch(rows)
		if err != nil {
			return dashboard.SaaSPaymentSettlementBatchReport{}, err
		}
		all = append(all, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSPaymentSettlementBatchReport{}, err
	}
	limit := clampSaaSPaymentSettlementLimit(options.Limit)
	items := all
	if len(items) > limit {
		items = items[:limit]
	}
	return dashboard.SaaSPaymentSettlementBatchReport{Options: options, Summary: summarizeSaaSPaymentSettlementBatches(all), Batches: items}, nil
}

func (s *MySQLStore) SaaSAdminPaymentSettlementEntries(ctx context.Context, options dashboard.SaaSPaymentSettlementEntryOptions) (dashboard.SaaSPaymentSettlementEntryReport, error) {
	where, args := saasPaymentSettlementEntryWhere(options)
	rows, err := s.db.QueryContext(ctx, saasPaymentSettlementEntrySelectSQL()+where+` ORDER BY b.imported_at DESC, e.line_no ASC, e.id ASC LIMIT ?`, append(args, saasPaymentSettlementQueryMaxLimit)...)
	if err != nil {
		return dashboard.SaaSPaymentSettlementEntryReport{}, err
	}
	defer rows.Close()
	all := make([]dashboard.SaaSPaymentSettlementEntry, 0)
	for rows.Next() {
		item, err := scanSaaSPaymentSettlementEntry(rows)
		if err != nil {
			return dashboard.SaaSPaymentSettlementEntryReport{}, err
		}
		all = append(all, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSPaymentSettlementEntryReport{}, err
	}
	limit := clampSaaSPaymentSettlementLimit(options.Limit)
	items := all
	if len(items) > limit {
		items = items[:limit]
	}
	return dashboard.SaaSPaymentSettlementEntryReport{Options: options, Summary: summarizeSaaSPaymentSettlementEntries(all), Entries: items}, nil
}

func (s *MySQLStore) SaaSAdminPaymentSettlementResolveApprovalSnapshot(ctx context.Context, entryID int64) (dashboard.SaaSPaymentSettlementEntry, dashboard.SaaSPaymentSettlementBatch, error) {
	if entryID <= 0 {
		return dashboard.SaaSPaymentSettlementEntry{}, dashboard.SaaSPaymentSettlementBatch{}, dashboard.NewSaaSAdminBadRequest("payment settlement entry id invalid")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return dashboard.SaaSPaymentSettlementEntry{}, dashboard.SaaSPaymentSettlementBatch{}, err
	}
	defer rollbackQuietly(tx)
	entry, err := saasPaymentSettlementEntryByIDTx(ctx, tx, entryID, false)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSPaymentSettlementEntry{}, dashboard.SaaSPaymentSettlementBatch{}, dashboard.NewSaaSAdminNotFound("payment settlement entry not found")
	}
	if err != nil {
		return dashboard.SaaSPaymentSettlementEntry{}, dashboard.SaaSPaymentSettlementBatch{}, err
	}
	batch, found, err := saasPaymentSettlementBatchByIDTx(ctx, tx, entry.BatchID, false)
	if err != nil {
		return dashboard.SaaSPaymentSettlementEntry{}, dashboard.SaaSPaymentSettlementBatch{}, err
	}
	if !found {
		return dashboard.SaaSPaymentSettlementEntry{}, dashboard.SaaSPaymentSettlementBatch{}, dashboard.NewSaaSAdminNotFound("payment settlement batch not found")
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSPaymentSettlementEntry{}, dashboard.SaaSPaymentSettlementBatch{}, err
	}
	return entry, batch, nil
}

func (s *MySQLStore) ImportSaaSAdminPaymentSettlement(ctx context.Context, input dashboard.SaaSPaymentSettlementImport) (dashboard.SaaSPaymentSettlementImportResult, error) {
	if strings.TrimSpace(input.BatchNo) == "" || strings.TrimSpace(input.Provider) == "" || strings.TrimSpace(input.ProviderSettlementNo) == "" || strings.TrimSpace(input.SourceSHA256) == "" || len(input.Entries) == 0 {
		return dashboard.SaaSPaymentSettlementImportResult{}, dashboard.NewSaaSAdminBadRequest("payment settlement import fields invalid")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSPaymentSettlementImportResult{}, err
	}
	defer rollbackQuietly(tx)

	existing, found, err := saasPaymentSettlementBatchByProviderNoTx(ctx, tx, input.Provider, input.ProviderSettlementNo, true)
	if err != nil {
		return dashboard.SaaSPaymentSettlementImportResult{}, err
	}
	if !found {
		existing, found, err = saasPaymentSettlementBatchBySourceTx(ctx, tx, input.Provider, input.SourceSHA256, true)
		if err != nil {
			return dashboard.SaaSPaymentSettlementImportResult{}, err
		}
	}
	if found {
		if existing.SourceSHA256 != strings.TrimSpace(input.SourceSHA256) || existing.ProviderSettlementNo != strings.TrimSpace(input.ProviderSettlementNo) {
			return dashboard.SaaSPaymentSettlementImportResult{}, saasPaymentConflict("payment settlement batch identity already used with different source")
		}
		entries, err := saasPaymentSettlementEntriesByBatchTx(ctx, tx, existing.BatchNo, false)
		if err != nil {
			return dashboard.SaaSPaymentSettlementImportResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSPaymentSettlementImportResult{}, err
		}
		report := dashboard.SaaSPaymentSettlementEntryReport{
			Options: dashboard.SaaSPaymentSettlementEntryOptions{BatchNo: existing.BatchNo, TransactionType: dashboard.SaaSPaymentSettlementTransactionAll, ReconciliationStatus: dashboard.SaaSPaymentSettlementReconciliationAll, HandlingStatus: dashboard.SaaSPaymentSettlementHandlingAll, Limit: len(entries)},
			Summary: summarizeSaaSPaymentSettlementEntries(entries), Entries: entries,
		}
		return dashboard.SaaSPaymentSettlementImportResult{Batch: existing, Entries: report, Idempotent: true}, nil
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_payment_settlement_batches
			(batch_no, provider, provider_settlement_no, period_start, period_end, currency, status, source_sha256,
			 imported_by_user_id, imported_by_tenant_id, imported_at, reconciled_at, version, remark, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'reconciled', ?, ?, ?, NOW(), NOW(), 1, ?, NOW(), NOW())
	`, strings.TrimSpace(input.BatchNo), strings.TrimSpace(input.Provider), strings.TrimSpace(input.ProviderSettlementNo),
		nullableTrimmedString(input.PeriodStart), nullableTrimmedString(input.PeriodEnd), strings.TrimSpace(input.Currency), strings.TrimSpace(input.SourceSHA256),
		input.ActorUserID, input.ActorTenantID, truncateRunes(strings.TrimSpace(input.Remark), 255))
	if err != nil {
		if isMySQLDuplicateKeyError(err) {
			return dashboard.SaaSPaymentSettlementImportResult{}, saasPaymentConflict("payment settlement batch already exists")
		}
		return dashboard.SaaSPaymentSettlementImportResult{}, err
	}
	batchID, err := result.LastInsertId()
	if err != nil {
		return dashboard.SaaSPaymentSettlementImportResult{}, err
	}
	for _, entry := range input.Entries {
		insertResult, err := tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_payment_settlement_entries
				(batch_id, line_no, provider, provider_transaction_no, transaction_type,
				 order_no, provider_order_no, refund_no, provider_refund_no,
				 amount_cents, fee_cents, net_amount_cents, currency, occurred_at,
				 reconciliation_status, difference_amount_cents, issue_codes_json, issue_message, handling_status,
				 version, raw_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'missing_internal', ?, JSON_ARRAY('missing_internal'), '未找到内部账本', 'open', 1, ?, NOW(), NOW())
		`, batchID, entry.LineNo, strings.TrimSpace(input.Provider), strings.TrimSpace(entry.ProviderTransactionNo), strings.TrimSpace(entry.TransactionType),
			strings.TrimSpace(entry.OrderNo), strings.TrimSpace(entry.ProviderOrderNo), strings.TrimSpace(entry.RefundNo), strings.TrimSpace(entry.ProviderRefundNo),
			entry.AmountCents, entry.FeeCents, entry.NetAmountCents, strings.TrimSpace(entry.Currency), nullableTrimmedString(entry.OccurredAt),
			entry.AmountCents, saasAdminJSONValue(entry.RawJSON))
		if err != nil {
			if isMySQLDuplicateKeyError(err) {
				return dashboard.SaaSPaymentSettlementImportResult{}, saasPaymentConflict(fmt.Sprintf("settlement line or provider transaction already exists: %s", entry.ProviderTransactionNo))
			}
			return dashboard.SaaSPaymentSettlementImportResult{}, err
		}
		entryID, err := insertResult.LastInsertId()
		if err != nil {
			return dashboard.SaaSPaymentSettlementImportResult{}, err
		}
		current, err := saasPaymentSettlementEntryByIDTx(ctx, tx, entryID, true)
		if err != nil {
			return dashboard.SaaSPaymentSettlementImportResult{}, err
		}
		projected, err := evaluateSaaSPaymentSettlementEntryTx(ctx, tx, current)
		if err != nil {
			return dashboard.SaaSPaymentSettlementImportResult{}, err
		}
		if err := updateSaaSPaymentSettlementEntryMatchTx(ctx, tx, current, projected, false); err != nil {
			return dashboard.SaaSPaymentSettlementImportResult{}, err
		}
	}
	if err := refreshSaaSPaymentSettlementBatchSummaryTx(ctx, tx, batchID, false); err != nil {
		return dashboard.SaaSPaymentSettlementImportResult{}, err
	}
	batch, found, err := saasPaymentSettlementBatchByIDTx(ctx, tx, batchID, true)
	if err != nil {
		return dashboard.SaaSPaymentSettlementImportResult{}, err
	}
	if !found {
		return dashboard.SaaSPaymentSettlementImportResult{}, errors.New("payment settlement batch insert did not return a row")
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: input.ActorTenantID, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionSettlementImport, TargetType: dashboard.SaaSAdminOperationTargetSettlementBatch,
		TargetID: batch.BatchNo, TargetName: batch.ProviderSettlementNo,
		AfterJSON: saasAdminMarshalJSON(saasPaymentSettlementBatchStatePayload(batch)), Remark: strings.TrimSpace(input.Remark),
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSPaymentSettlementImportResult{}, err
	}
	entries, err := saasPaymentSettlementEntriesByBatchTx(ctx, tx, batch.BatchNo, false)
	if err != nil {
		return dashboard.SaaSPaymentSettlementImportResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSPaymentSettlementImportResult{}, err
	}
	return dashboard.SaaSPaymentSettlementImportResult{
		Batch: batch, OperationID: operationID,
		Entries: dashboard.SaaSPaymentSettlementEntryReport{
			Options: dashboard.SaaSPaymentSettlementEntryOptions{BatchNo: batch.BatchNo, TransactionType: dashboard.SaaSPaymentSettlementTransactionAll, ReconciliationStatus: dashboard.SaaSPaymentSettlementReconciliationAll, HandlingStatus: dashboard.SaaSPaymentSettlementHandlingAll, Limit: len(entries)},
			Summary: summarizeSaaSPaymentSettlementEntries(entries), Entries: entries,
		},
	}, nil
}

func (s *MySQLStore) ReconcileSaaSAdminPaymentSettlement(ctx context.Context, input dashboard.SaaSPaymentSettlementReconcile) (dashboard.SaaSPaymentSettlementReconcileResult, error) {
	if strings.TrimSpace(input.BatchNo) == "" || input.ExpectedVersion <= 0 {
		return dashboard.SaaSPaymentSettlementReconcileResult{}, dashboard.NewSaaSAdminBadRequest("batchNo and expectedVersion are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSPaymentSettlementReconcileResult{}, err
	}
	defer rollbackQuietly(tx)
	batch, found, err := saasPaymentSettlementBatchByNoTx(ctx, tx, input.BatchNo, true)
	if err != nil {
		return dashboard.SaaSPaymentSettlementReconcileResult{}, err
	}
	if !found {
		return dashboard.SaaSPaymentSettlementReconcileResult{}, dashboard.NewSaaSAdminNotFound("payment settlement batch not found")
	}
	if batch.Version != input.ExpectedVersion {
		return dashboard.SaaSPaymentSettlementReconcileResult{}, saasPaymentConflict(fmt.Sprintf("payment settlement batch version conflict: current=%d", batch.Version))
	}
	if batch.Status == dashboard.SaaSPaymentSettlementBatchStatusClosed {
		return dashboard.SaaSPaymentSettlementReconcileResult{}, saasPaymentConflict("closed payment settlement batch must be reopened before reconciliation")
	}
	entries, err := saasPaymentSettlementEntriesByBatchTx(ctx, tx, batch.BatchNo, true)
	if err != nil {
		return dashboard.SaaSPaymentSettlementReconcileResult{}, err
	}
	projected := make([]dashboard.SaaSPaymentSettlementEntry, 0, len(entries))
	for _, current := range entries {
		next, err := evaluateSaaSPaymentSettlementEntryTx(ctx, tx, current)
		if err != nil {
			return dashboard.SaaSPaymentSettlementReconcileResult{}, err
		}
		projected = append(projected, next)
		if !input.DryRun {
			if err := updateSaaSPaymentSettlementEntryMatchTx(ctx, tx, current, next, true); err != nil {
				return dashboard.SaaSPaymentSettlementReconcileResult{}, err
			}
		}
	}
	projectedSummary := summarizeSaaSPaymentSettlementEntries(projected)
	if input.DryRun {
		projectedBatch := applySaaSPaymentSettlementEntrySummary(batch, projectedSummary)
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSPaymentSettlementReconcileResult{}, err
		}
		return dashboard.SaaSPaymentSettlementReconcileResult{
			Batch: projectedBatch, DryRun: true,
			Entries: dashboard.SaaSPaymentSettlementEntryReport{Options: dashboard.SaaSPaymentSettlementEntryOptions{BatchNo: batch.BatchNo, Limit: len(projected)}, Summary: projectedSummary, Entries: projected},
		}, nil
	}
	if err := refreshSaaSPaymentSettlementBatchSummaryTx(ctx, tx, batch.ID, true); err != nil {
		return dashboard.SaaSPaymentSettlementReconcileResult{}, err
	}
	nextBatch, _, err := saasPaymentSettlementBatchByIDTx(ctx, tx, batch.ID, true)
	if err != nil {
		return dashboard.SaaSPaymentSettlementReconcileResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: input.ActorTenantID, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionSettlementReconcile, TargetType: dashboard.SaaSAdminOperationTargetSettlementBatch,
		TargetID: batch.BatchNo, TargetName: batch.ProviderSettlementNo,
		BeforeJSON: saasAdminMarshalJSON(saasPaymentSettlementBatchStatePayload(batch)), AfterJSON: saasAdminMarshalJSON(saasPaymentSettlementBatchStatePayload(nextBatch)),
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSPaymentSettlementReconcileResult{}, err
	}
	nextEntries, err := saasPaymentSettlementEntriesByBatchTx(ctx, tx, batch.BatchNo, false)
	if err != nil {
		return dashboard.SaaSPaymentSettlementReconcileResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSPaymentSettlementReconcileResult{}, err
	}
	return dashboard.SaaSPaymentSettlementReconcileResult{
		Batch: nextBatch, OperationID: operationID,
		Entries: dashboard.SaaSPaymentSettlementEntryReport{Options: dashboard.SaaSPaymentSettlementEntryOptions{BatchNo: batch.BatchNo, Limit: len(nextEntries)}, Summary: summarizeSaaSPaymentSettlementEntries(nextEntries), Entries: nextEntries},
	}, nil
}

func (s *MySQLStore) ResolveSaaSAdminPaymentSettlementEntry(ctx context.Context, input dashboard.SaaSPaymentSettlementEntryResolve) (dashboard.SaaSPaymentSettlementEntryResolveResult, error) {
	if input.EntryID <= 0 || input.ExpectedVersion <= 0 || strings.TrimSpace(input.Reason) == "" {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, dashboard.NewSaaSAdminBadRequest("payment settlement resolution fields invalid")
	}
	if input.HandlingStatus != dashboard.SaaSPaymentSettlementHandlingOpen && input.HandlingStatus != dashboard.SaaSPaymentSettlementHandlingResolved && input.HandlingStatus != dashboard.SaaSPaymentSettlementHandlingIgnored {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, dashboard.NewSaaSAdminBadRequest("payment settlement handling status invalid")
	}
	approvedExecution := input.ApprovalExecutionID > 0
	if approvedExecution != (input.ApprovalExecutionVersion > 0) {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, dashboard.NewSaaSAdminBadRequest("审批执行引用无效")
	}
	if approvedExecution && input.ApprovalPlan == nil {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, dashboard.NewSaaSAdminBadRequest("结算差异处理审批冻结快照无效")
	}
	if !approvedExecution && input.ApprovalPlan != nil {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, dashboard.NewSaaSAdminBadRequest("结算差异处理审批执行引用缺失")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, err
	}
	defer rollbackQuietly(tx)
	var batchID int64
	// Lock the batch before the entry to match reconcile and transition ordering.
	err = tx.QueryRowContext(ctx, `
		SELECT batch_id
		FROM mochat_go_saas_payment_settlement_entries
		WHERE id = ?
		LIMIT 1
	`, input.EntryID).Scan(&batchID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, dashboard.NewSaaSAdminNotFound("payment settlement entry not found")
	}
	if err != nil {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, err
	}
	batch, found, err := saasPaymentSettlementBatchByIDTx(ctx, tx, batchID, true)
	if err != nil {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, err
	}
	if !found {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, dashboard.NewSaaSAdminNotFound("payment settlement batch not found")
	}
	current, err := saasPaymentSettlementEntryByIDTx(ctx, tx, input.EntryID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, dashboard.NewSaaSAdminNotFound("payment settlement entry not found")
	}
	if err != nil {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, err
	}
	if current.BatchID != batch.ID {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, saasPaymentConflict("payment settlement entry batch changed")
	}
	if approvedExecution {
		plan := input.ApprovalPlan
		if plan.SchemaVersion != dashboard.SaaSPaymentSettlementResolveApprovalPlanSchemaVersion ||
			plan.Resolve.EntryID != input.EntryID || plan.Resolve.ExpectedVersion != input.ExpectedVersion ||
			plan.Resolve.HandlingStatus != input.HandlingStatus || plan.Resolve.Reason != input.Reason ||
			!saasPaymentSettlementResolveApprovalSnapshotMatchesEntry(plan.Entry, current) ||
			!saasPaymentSettlementApprovalSnapshotMatchesBatch(dashboard.SaaSPaymentSettlementApprovalPlanSchemaVersion, plan.Batch, batch) {
			return dashboard.SaaSPaymentSettlementEntryResolveResult{}, &dashboard.SaaSAdminOperationError{
				Status: 409, Message: "结算差异明细、所属批次账务或处理审计已变化，请刷新后重新申请审批",
			}
		}
	}
	if batch.Status != dashboard.SaaSPaymentSettlementBatchStatusReconciled {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, saasPaymentConflict("payment settlement batch is not reconciled")
	}
	if current.Version != input.ExpectedVersion {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, saasPaymentConflict(fmt.Sprintf("payment settlement entry version conflict: current=%d", current.Version))
	}
	if current.ReconciliationStatus == dashboard.SaaSPaymentSettlementReconciliationMatched {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, saasPaymentConflict("matched payment settlement entry does not require resolution")
	}
	if !dashboardSaaSPaymentSettlementResolveTransitionValid(current.HandlingStatus, input.HandlingStatus) {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, saasPaymentConflict("payment settlement handling status transition invalid")
	}
	result, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_payment_settlement_entries
		SET handling_status = ?, handled_by_user_id = ?, handled_by_tenant_id = ?, handled_at = NOW(),
			handling_reason = ?, version = version + 1, updated_at = NOW()
		WHERE id = ? AND version = ?
		`, strings.TrimSpace(input.HandlingStatus), input.ActorUserID, input.ActorTenantID,
		truncateRunes(strings.TrimSpace(input.Reason), 255), current.ID, current.Version)
	if err != nil {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, saasPaymentConflict("payment settlement entry version changed")
	}
	if err := refreshSaaSPaymentSettlementBatchSummaryTx(ctx, tx, batch.ID, true); err != nil {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, err
	}
	next, err := saasPaymentSettlementEntryByIDTx(ctx, tx, current.ID, true)
	if err != nil {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, err
	}
	nextBatch, _, err := saasPaymentSettlementBatchByIDTx(ctx, tx, batch.ID, true)
	if err != nil {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: input.ActorTenantID, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionSettlementResolve, TargetType: dashboard.SaaSAdminOperationTargetSettlementEntry,
		TargetID: strconvFormatInt64(current.ID), TargetName: current.ProviderTransactionNo,
		BeforeJSON: saasAdminMarshalJSON(saasPaymentSettlementEntryStatePayload(current)), AfterJSON: saasAdminMarshalJSON(saasPaymentSettlementEntryStatePayload(next)),
		Remark: strings.TrimSpace(input.Reason),
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.ActorUserID, operationID); err != nil {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSPaymentSettlementEntryResolveResult{}, err
	}
	return dashboard.SaaSPaymentSettlementEntryResolveResult{Entry: next, Batch: nextBatch, OperationID: operationID}, nil
}

func saasPaymentSettlementResolveApprovalSnapshotMatchesEntry(frozen dashboard.SaaSPaymentSettlementEntryApprovalSnapshot, current dashboard.SaaSPaymentSettlementEntry) bool {
	return frozen.ID == current.ID && frozen.BatchID == current.BatchID && frozen.BatchNo == current.BatchNo &&
		frozen.BatchStatus == current.BatchStatus && frozen.LineNo == current.LineNo && frozen.Provider == current.Provider &&
		frozen.ProviderTransactionNo == current.ProviderTransactionNo && frozen.TransactionType == current.TransactionType &&
		frozen.OrderNo == current.OrderNo && frozen.ProviderOrderNo == current.ProviderOrderNo && frozen.RefundNo == current.RefundNo &&
		frozen.ProviderRefundNo == current.ProviderRefundNo && frozen.AmountCents == current.AmountCents &&
		frozen.FeeCents == current.FeeCents && frozen.NetAmountCents == current.NetAmountCents && frozen.Currency == current.Currency &&
		frozen.OccurredAt == current.OccurredAt && frozen.MatchedPaymentOrderID == current.MatchedPaymentOrderID &&
		frozen.MatchedRefundID == current.MatchedRefundID && frozen.MatchedTenantID == current.MatchedTenantID &&
		frozen.MatchedTenantName == current.MatchedTenantName && frozen.MatchedInternalNo == current.MatchedInternalNo &&
		frozen.ExpectedAmountCents == current.ExpectedAmountCents && frozen.ExpectedCurrency == current.ExpectedCurrency &&
		frozen.ExpectedStatus == current.ExpectedStatus && frozen.ReconciliationStatus == current.ReconciliationStatus &&
		frozen.DifferenceAmountCents == current.DifferenceAmountCents && frozen.IssueCodesJSON == current.IssueCodesJSON &&
		frozen.IssueMessage == current.IssueMessage && frozen.HandlingStatus == current.HandlingStatus &&
		frozen.HandledByUserID == current.HandledByUserID && frozen.HandledByTenantID == current.HandledByTenantID &&
		frozen.HandledAt == current.HandledAt && frozen.HandlingReason == current.HandlingReason && frozen.Version == current.Version &&
		frozen.RawJSON == current.RawJSON && frozen.CreatedAt == current.CreatedAt && frozen.UpdatedAt == current.UpdatedAt
}

func dashboardSaaSPaymentSettlementResolveTransitionValid(current, target string) bool {
	if current == dashboard.SaaSPaymentSettlementHandlingOpen {
		return target == dashboard.SaaSPaymentSettlementHandlingResolved || target == dashboard.SaaSPaymentSettlementHandlingIgnored
	}
	if current == dashboard.SaaSPaymentSettlementHandlingResolved || current == dashboard.SaaSPaymentSettlementHandlingIgnored {
		return target == dashboard.SaaSPaymentSettlementHandlingOpen
	}
	return false
}

func (s *MySQLStore) TransitionSaaSAdminPaymentSettlement(ctx context.Context, input dashboard.SaaSPaymentSettlementTransition) (dashboard.SaaSPaymentSettlementTransitionResult, error) {
	if strings.TrimSpace(input.BatchNo) == "" || input.ExpectedVersion <= 0 || strings.TrimSpace(input.Reason) == "" {
		return dashboard.SaaSPaymentSettlementTransitionResult{}, dashboard.NewSaaSAdminBadRequest("payment settlement transition fields invalid")
	}
	approvedExecution := input.ApprovalExecutionID > 0
	if approvedExecution != (input.ApprovalExecutionVersion > 0) {
		return dashboard.SaaSPaymentSettlementTransitionResult{}, dashboard.NewSaaSAdminBadRequest("审批执行引用无效")
	}
	approvalActionValid := input.Action == dashboard.SaaSPaymentSettlementTransitionClose || input.Action == dashboard.SaaSPaymentSettlementTransitionReopen
	if approvedExecution && (!approvalActionValid || input.ApprovalPlan == nil) {
		return dashboard.SaaSPaymentSettlementTransitionResult{}, dashboard.NewSaaSAdminBadRequest("结算状态变更审批冻结快照无效")
	}
	if !approvedExecution && input.ApprovalPlan != nil {
		return dashboard.SaaSPaymentSettlementTransitionResult{}, dashboard.NewSaaSAdminBadRequest("结算状态变更审批执行引用缺失")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSPaymentSettlementTransitionResult{}, err
	}
	defer rollbackQuietly(tx)
	current, found, err := saasPaymentSettlementBatchByNoTx(ctx, tx, input.BatchNo, true)
	if err != nil {
		return dashboard.SaaSPaymentSettlementTransitionResult{}, err
	}
	if !found {
		return dashboard.SaaSPaymentSettlementTransitionResult{}, dashboard.NewSaaSAdminNotFound("payment settlement batch not found")
	}
	if approvedExecution {
		plan := input.ApprovalPlan
		if plan.Transition.BatchNo != input.BatchNo || plan.Transition.ExpectedVersion != input.ExpectedVersion ||
			plan.Transition.Action != input.Action ||
			!saasPaymentSettlementApprovalSnapshotMatchesBatch(plan.SchemaVersion, plan.Batch, current) {
			return dashboard.SaaSPaymentSettlementTransitionResult{}, &dashboard.SaaSAdminOperationError{
				Status: 409, Message: "结算批次账务、关账审计或状态已变化，请刷新后重新申请审批",
			}
		}
		if input.Action == dashboard.SaaSPaymentSettlementTransitionReopen && plan.SchemaVersion != dashboard.SaaSPaymentSettlementApprovalPlanSchemaVersion {
			return dashboard.SaaSPaymentSettlementTransitionResult{}, dashboard.NewSaaSAdminBadRequest("结算重开审批冻结快照版本无效")
		}
	}
	if current.Version != input.ExpectedVersion {
		return dashboard.SaaSPaymentSettlementTransitionResult{}, saasPaymentConflict(fmt.Sprintf("payment settlement batch version conflict: current=%d", current.Version))
	}
	switch strings.TrimSpace(input.Action) {
	case dashboard.SaaSPaymentSettlementTransitionClose:
		if current.Status == dashboard.SaaSPaymentSettlementBatchStatusClosed {
			return dashboard.SaaSPaymentSettlementTransitionResult{}, saasPaymentConflict("payment settlement batch already closed")
		}
		if current.OpenIssueCount > 0 {
			return dashboard.SaaSPaymentSettlementTransitionResult{}, saasPaymentConflict("payment settlement batch still has open issues")
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_payment_settlement_batches
			SET status = 'closed', closed_by_user_id = ?, closed_at = NOW(), close_reason = ?, version = version + 1, updated_at = NOW()
			WHERE id = ? AND version = ?
		`, input.ActorUserID, truncateRunes(strings.TrimSpace(input.Reason), 255), current.ID, current.Version); err != nil {
			return dashboard.SaaSPaymentSettlementTransitionResult{}, err
		}
	case dashboard.SaaSPaymentSettlementTransitionReopen:
		if current.Status != dashboard.SaaSPaymentSettlementBatchStatusClosed {
			return dashboard.SaaSPaymentSettlementTransitionResult{}, saasPaymentConflict("payment settlement batch is not closed")
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_payment_settlement_batches
			SET status = 'reconciled', closed_by_user_id = 0, closed_at = NULL, close_reason = '', version = version + 1, updated_at = NOW()
			WHERE id = ? AND version = ?
		`, current.ID, current.Version); err != nil {
			return dashboard.SaaSPaymentSettlementTransitionResult{}, err
		}
	default:
		return dashboard.SaaSPaymentSettlementTransitionResult{}, dashboard.NewSaaSAdminBadRequest("payment settlement transition action invalid")
	}
	next, _, err := saasPaymentSettlementBatchByIDTx(ctx, tx, current.ID, true)
	if err != nil {
		return dashboard.SaaSPaymentSettlementTransitionResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: input.ActorTenantID, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionSettlementTransition, TargetType: dashboard.SaaSAdminOperationTargetSettlementBatch,
		TargetID: current.BatchNo, TargetName: current.ProviderSettlementNo,
		BeforeJSON: saasAdminMarshalJSON(saasPaymentSettlementBatchStatePayload(current)), AfterJSON: saasAdminMarshalJSON(saasPaymentSettlementBatchStatePayload(next)),
		Remark: strings.TrimSpace(input.Reason),
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSPaymentSettlementTransitionResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.ActorUserID, operationID); err != nil {
		return dashboard.SaaSPaymentSettlementTransitionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSPaymentSettlementTransitionResult{}, err
	}
	return dashboard.SaaSPaymentSettlementTransitionResult{Batch: next, PreviousStatus: current.Status, OperationID: operationID}, nil
}

func saasPaymentSettlementApprovalSnapshotMatchesBatch(schemaVersion int, frozen dashboard.SaaSPaymentSettlementBatchApprovalSnapshot, current dashboard.SaaSPaymentSettlementBatch) bool {
	baseMatches := frozen.ID == current.ID && frozen.BatchNo == current.BatchNo && frozen.Provider == current.Provider &&
		frozen.ProviderSettlementNo == current.ProviderSettlementNo && frozen.PeriodStart == current.PeriodStart &&
		frozen.PeriodEnd == current.PeriodEnd && frozen.Currency == current.Currency && frozen.Status == current.Status &&
		frozen.SourceSHA256 == current.SourceSHA256 && frozen.EntryCount == current.EntryCount &&
		frozen.PaymentCount == current.PaymentCount && frozen.RefundCount == current.RefundCount &&
		frozen.MatchedCount == current.MatchedCount && frozen.IssueCount == current.IssueCount &&
		frozen.OpenIssueCount == current.OpenIssueCount && frozen.ResolvedIssueCount == current.ResolvedIssueCount &&
		frozen.IgnoredIssueCount == current.IgnoredIssueCount && frozen.TotalAmountCents == current.TotalAmountCents &&
		frozen.TotalFeeCents == current.TotalFeeCents && frozen.TotalNetCents == current.TotalNetCents &&
		frozen.DifferenceAmountCents == current.DifferenceAmountCents && frozen.Version == current.Version &&
		frozen.Remark == current.Remark
	if !baseMatches {
		return false
	}
	if schemaVersion == 0 {
		return true
	}
	if schemaVersion != dashboard.SaaSPaymentSettlementApprovalPlanSchemaVersion {
		return false
	}
	return frozen.ImportedByUserID == current.ImportedByUserID && frozen.ImportedByTenantID == current.ImportedByTenantID &&
		frozen.ImportedAt == current.ImportedAt && frozen.ReconciledAt == current.ReconciledAt &&
		frozen.ClosedByUserID == current.ClosedByUserID && frozen.ClosedAt == current.ClosedAt &&
		frozen.CloseReason == current.CloseReason && frozen.CreatedAt == current.CreatedAt && frozen.UpdatedAt == current.UpdatedAt
}

func evaluateSaaSPaymentSettlementEntryTx(ctx context.Context, tx *sql.Tx, current dashboard.SaaSPaymentSettlementEntry) (dashboard.SaaSPaymentSettlementEntry, error) {
	next := current
	next.MatchedPaymentOrderID = 0
	next.MatchedRefundID = 0
	next.MatchedTenantID = 0
	next.MatchedTenantName = ""
	next.MatchedInternalNo = ""
	next.ExpectedAmountCents = 0
	next.ExpectedCurrency = ""
	next.ExpectedStatus = ""
	next.DifferenceAmountCents = current.AmountCents
	issues := make([]string, 0, 4)
	messages := make([]string, 0, 4)
	identifierConflict := false

	if current.TransactionType == dashboard.SaaSPaymentSettlementTransactionPayment {
		order, found, conflict, err := findSaaSPaymentSettlementOrderTx(ctx, tx, current)
		if err != nil {
			return next, err
		}
		identifierConflict = conflict
		if found {
			next.MatchedTenantID = order.TenantID
			next.MatchedTenantName = order.TenantName
			next.MatchedInternalNo = order.OrderNo
			next.ExpectedAmountCents = order.AmountCents
			next.ExpectedCurrency = order.Currency
			next.ExpectedStatus = order.Status
			next.DifferenceAmountCents = current.AmountCents - order.AmountCents
			if !identifierConflict {
				used, err := saasPaymentSettlementPaymentMatchUsedTx(ctx, tx, current.ID, order.ID)
				if err != nil {
					return next, err
				}
				if used {
					identifierConflict = true
					messages = append(messages, "内部支付订单已被其他结算明细匹配")
				} else {
					next.MatchedPaymentOrderID = order.ID
				}
			}
			if order.Status != dashboard.SaaSPaymentOrderStatusPaid {
				issues = append(issues, dashboard.SaaSPaymentSettlementReconciliationStatusMismatch)
				messages = append(messages, "内部支付订单状态不是 paid")
			}
			if order.Currency != current.Currency {
				issues = append(issues, dashboard.SaaSPaymentSettlementReconciliationCurrencyMismatch)
				messages = append(messages, "结算币种与内部支付订单不一致")
			}
			if order.AmountCents != current.AmountCents {
				issues = append(issues, dashboard.SaaSPaymentSettlementReconciliationAmountMismatch)
				messages = append(messages, "结算金额与内部支付订单不一致")
			}
		} else {
			issues = append(issues, dashboard.SaaSPaymentSettlementReconciliationMissingInternal)
			messages = append(messages, "未找到内部支付订单")
		}
	} else {
		refund, found, conflict, err := findSaaSPaymentSettlementRefundTx(ctx, tx, current)
		if err != nil {
			return next, err
		}
		identifierConflict = conflict
		if found {
			expectedAmount := -refund.AmountCents
			next.MatchedTenantID = refund.TenantID
			next.MatchedTenantName = refund.TenantName
			next.MatchedInternalNo = refund.RefundNo
			next.ExpectedAmountCents = expectedAmount
			next.ExpectedCurrency = refund.Currency
			next.ExpectedStatus = refund.Status
			next.DifferenceAmountCents = current.AmountCents - expectedAmount
			if !identifierConflict {
				used, err := saasPaymentSettlementRefundMatchUsedTx(ctx, tx, current.ID, refund.ID)
				if err != nil {
					return next, err
				}
				if used {
					identifierConflict = true
					messages = append(messages, "内部退款单已被其他结算明细匹配")
				} else {
					next.MatchedRefundID = refund.ID
				}
			}
			if refund.Status != dashboard.SaaSPaymentRefundStatusSucceeded {
				issues = append(issues, dashboard.SaaSPaymentSettlementReconciliationStatusMismatch)
				messages = append(messages, "内部退款单状态不是 succeeded")
			}
			if refund.Currency != current.Currency {
				issues = append(issues, dashboard.SaaSPaymentSettlementReconciliationCurrencyMismatch)
				messages = append(messages, "结算币种与内部退款单不一致")
			}
			if expectedAmount != current.AmountCents {
				issues = append(issues, dashboard.SaaSPaymentSettlementReconciliationAmountMismatch)
				messages = append(messages, "结算金额与内部退款单不一致")
			}
		} else {
			issues = append(issues, dashboard.SaaSPaymentSettlementReconciliationMissingInternal)
			messages = append(messages, "未找到内部退款单")
		}
	}
	if identifierConflict {
		issues = append([]string{dashboard.SaaSPaymentSettlementReconciliationIdentifierConflict}, issues...)
		if len(messages) == 0 || !strings.Contains(messages[0], "标识") {
			messages = append([]string{"平台单号与渠道单号未指向同一内部记录"}, messages...)
		}
		next.MatchedPaymentOrderID = 0
		next.MatchedRefundID = 0
	}
	issues = uniqueStringsInOrder(issues)
	if len(issues) == 0 {
		next.ReconciliationStatus = dashboard.SaaSPaymentSettlementReconciliationMatched
		next.IssueCodesJSON = "[]"
		next.IssueMessage = ""
		next.HandlingStatus = dashboard.SaaSPaymentSettlementHandlingNone
		next.HandledByUserID = 0
		next.HandledByTenantID = 0
		next.HandledAt = ""
		next.HandlingReason = ""
		return next, nil
	}
	next.ReconciliationStatus = issues[0]
	encoded, _ := json.Marshal(issues)
	next.IssueCodesJSON = string(encoded)
	next.IssueMessage = truncateRunes(strings.Join(uniqueStringsInOrder(messages), "；"), 255)
	if current.ReconciliationStatus != next.ReconciliationStatus || (current.HandlingStatus != dashboard.SaaSPaymentSettlementHandlingResolved && current.HandlingStatus != dashboard.SaaSPaymentSettlementHandlingIgnored) {
		next.HandlingStatus = dashboard.SaaSPaymentSettlementHandlingOpen
		next.HandledByUserID = 0
		next.HandledByTenantID = 0
		next.HandledAt = ""
		next.HandlingReason = ""
	}
	return next, nil
}

func findSaaSPaymentSettlementOrderTx(ctx context.Context, tx *sql.Tx, entry dashboard.SaaSPaymentSettlementEntry) (dashboard.SaaSAdminPaymentOrder, bool, bool, error) {
	var byOrder, byProvider dashboard.SaaSAdminPaymentOrder
	var orderFound, providerFound bool
	var err error
	if strings.TrimSpace(entry.OrderNo) != "" {
		byOrder, orderFound, err = saasPaymentOrderByNoTx(ctx, tx, entry.OrderNo, false)
		if err != nil {
			return dashboard.SaaSAdminPaymentOrder{}, false, false, err
		}
	}
	if strings.TrimSpace(entry.ProviderOrderNo) != "" {
		byProvider, providerFound, err = saasPaymentOrderByProviderNoTx(ctx, tx, entry.Provider, entry.ProviderOrderNo)
		if err != nil {
			return dashboard.SaaSAdminPaymentOrder{}, false, false, err
		}
	}
	if orderFound && providerFound {
		return byOrder, true, byOrder.ID != byProvider.ID, nil
	}
	if orderFound {
		conflict := entry.ProviderOrderNo != "" && (byOrder.Provider != entry.Provider || byOrder.ProviderOrderNo != entry.ProviderOrderNo)
		return byOrder, true, conflict, nil
	}
	if providerFound {
		conflict := entry.OrderNo != "" && byProvider.OrderNo != entry.OrderNo
		return byProvider, true, conflict, nil
	}
	return dashboard.SaaSAdminPaymentOrder{}, false, false, nil
}

func findSaaSPaymentSettlementRefundTx(ctx context.Context, tx *sql.Tx, entry dashboard.SaaSPaymentSettlementEntry) (dashboard.SaaSAdminPaymentRefund, bool, bool, error) {
	var byRefund, byProvider dashboard.SaaSAdminPaymentRefund
	var refundFound, providerFound bool
	var err error
	if strings.TrimSpace(entry.RefundNo) != "" {
		byRefund, refundFound, err = saasPaymentRefundByNoTx(ctx, tx, entry.RefundNo, false)
		if err != nil {
			return dashboard.SaaSAdminPaymentRefund{}, false, false, err
		}
	}
	if strings.TrimSpace(entry.ProviderRefundNo) != "" {
		byProvider, providerFound, err = saasPaymentRefundByProviderNoTx(ctx, tx, entry.Provider, entry.ProviderRefundNo)
		if err != nil {
			return dashboard.SaaSAdminPaymentRefund{}, false, false, err
		}
	}
	if refundFound && providerFound {
		return byRefund, true, byRefund.ID != byProvider.ID, nil
	}
	if refundFound {
		conflict := entry.ProviderRefundNo != "" && (byRefund.Provider != entry.Provider || byRefund.ProviderRefundNo != entry.ProviderRefundNo)
		return byRefund, true, conflict, nil
	}
	if providerFound {
		conflict := entry.RefundNo != "" && byProvider.RefundNo != entry.RefundNo
		return byProvider, true, conflict, nil
	}
	return dashboard.SaaSAdminPaymentRefund{}, false, false, nil
}

func saasPaymentOrderByProviderNoTx(ctx context.Context, tx *sql.Tx, provider string, providerOrderNo string) (dashboard.SaaSAdminPaymentOrder, bool, error) {
	item, err := scanSaaSAdminPaymentOrder(tx.QueryRowContext(ctx, saasAdminPaymentOrderSelectSQL()+` WHERE o.provider = ? AND o.provider_order_no = ? AND o.deleted_at IS NULL LIMIT 1`, provider, providerOrderNo))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminPaymentOrder{}, false, nil
	}
	return item, err == nil, err
}

func saasPaymentRefundByProviderNoTx(ctx context.Context, tx *sql.Tx, provider string, providerRefundNo string) (dashboard.SaaSAdminPaymentRefund, bool, error) {
	item, err := scanSaaSAdminPaymentRefund(tx.QueryRowContext(ctx, saasAdminPaymentRefundSelectSQL()+` WHERE r.provider = ? AND r.provider_refund_no = ? AND r.deleted_at IS NULL LIMIT 1`, provider, providerRefundNo))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminPaymentRefund{}, false, nil
	}
	return item, err == nil, err
}

func saasPaymentSettlementPaymentMatchUsedTx(ctx context.Context, tx *sql.Tx, entryID int64, orderID int64) (bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM mochat_go_saas_payment_settlement_entries WHERE matched_payment_order_id = ? AND id <> ? LIMIT 1`, orderID, entryID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func saasPaymentSettlementRefundMatchUsedTx(ctx context.Context, tx *sql.Tx, entryID int64, refundID int64) (bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM mochat_go_saas_payment_settlement_entries WHERE matched_refund_id = ? AND id <> ? LIMIT 1`, refundID, entryID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func updateSaaSPaymentSettlementEntryMatchTx(ctx context.Context, tx *sql.Tx, current dashboard.SaaSPaymentSettlementEntry, next dashboard.SaaSPaymentSettlementEntry, incrementVersion bool) error {
	versionSQL := "version"
	if incrementVersion {
		versionSQL = "version + 1"
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_settlement_entries
		SET matched_payment_order_id = ?, matched_refund_id = ?, matched_tenant_id = ?, matched_internal_no = ?,
			expected_amount_cents = ?, expected_currency = ?, expected_status = ?, reconciliation_status = ?,
			difference_amount_cents = ?, issue_codes_json = ?, issue_message = ?, handling_status = ?,
			handled_by_user_id = ?, handled_by_tenant_id = ?, handled_at = ?, handling_reason = ?,
			version = `+versionSQL+`, updated_at = NOW()
		WHERE id = ?
	`, nullablePositiveInt64(next.MatchedPaymentOrderID), nullablePositiveInt64(next.MatchedRefundID), next.MatchedTenantID, next.MatchedInternalNo,
		next.ExpectedAmountCents, next.ExpectedCurrency, next.ExpectedStatus, next.ReconciliationStatus,
		next.DifferenceAmountCents, saasAdminJSONValue(next.IssueCodesJSON), next.IssueMessage, next.HandlingStatus,
		next.HandledByUserID, next.HandledByTenantID, nullableTrimmedString(next.HandledAt), next.HandlingReason, current.ID)
	if isMySQLDuplicateKeyError(err) {
		return saasPaymentConflict("internal payment or refund is already matched by another settlement entry")
	}
	return err
}

func refreshSaaSPaymentSettlementBatchSummaryTx(ctx context.Context, tx *sql.Tx, batchID int64, incrementVersion bool) error {
	versionSQL := "version"
	if incrementVersion {
		versionSQL = "version + 1"
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_settlement_batches b
		SET b.entry_count = (SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_entries e WHERE e.batch_id = b.id),
			b.payment_count = (SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_entries e WHERE e.batch_id = b.id AND e.transaction_type = 'payment'),
			b.refund_count = (SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_entries e WHERE e.batch_id = b.id AND e.transaction_type = 'refund'),
			b.matched_count = (SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_entries e WHERE e.batch_id = b.id AND e.reconciliation_status = 'matched'),
			b.issue_count = (SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_entries e WHERE e.batch_id = b.id AND e.reconciliation_status <> 'matched'),
			b.open_issue_count = (SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_entries e WHERE e.batch_id = b.id AND e.reconciliation_status <> 'matched' AND e.handling_status = 'open'),
			b.resolved_issue_count = (SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_entries e WHERE e.batch_id = b.id AND e.reconciliation_status <> 'matched' AND e.handling_status = 'resolved'),
			b.ignored_issue_count = (SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_entries e WHERE e.batch_id = b.id AND e.reconciliation_status <> 'matched' AND e.handling_status = 'ignored'),
			b.total_amount_cents = (SELECT COALESCE(SUM(e.amount_cents), 0) FROM mochat_go_saas_payment_settlement_entries e WHERE e.batch_id = b.id),
			b.total_fee_cents = (SELECT COALESCE(SUM(e.fee_cents), 0) FROM mochat_go_saas_payment_settlement_entries e WHERE e.batch_id = b.id),
			b.total_net_cents = (SELECT COALESCE(SUM(e.net_amount_cents), 0) FROM mochat_go_saas_payment_settlement_entries e WHERE e.batch_id = b.id),
			b.difference_amount_cents = (SELECT COALESCE(SUM(e.difference_amount_cents), 0) FROM mochat_go_saas_payment_settlement_entries e WHERE e.batch_id = b.id),
			b.reconciled_at = NOW(), b.version = `+versionSQL+`, b.updated_at = NOW()
		WHERE b.id = ?
	`, batchID)
	return err
}

func saasPaymentSettlementBatchByNoTx(ctx context.Context, tx *sql.Tx, batchNo string, forUpdate bool) (dashboard.SaaSPaymentSettlementBatch, bool, error) {
	query := saasPaymentSettlementBatchSelectSQL() + ` WHERE b.batch_no = ? LIMIT 1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	item, err := scanSaaSPaymentSettlementBatch(tx.QueryRowContext(ctx, query, strings.TrimSpace(batchNo)))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSPaymentSettlementBatch{}, false, nil
	}
	return item, err == nil, err
}

func saasPaymentSettlementBatchByIDTx(ctx context.Context, tx *sql.Tx, batchID int64, forUpdate bool) (dashboard.SaaSPaymentSettlementBatch, bool, error) {
	query := saasPaymentSettlementBatchSelectSQL() + ` WHERE b.id = ? LIMIT 1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	item, err := scanSaaSPaymentSettlementBatch(tx.QueryRowContext(ctx, query, batchID))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSPaymentSettlementBatch{}, false, nil
	}
	return item, err == nil, err
}

func saasPaymentSettlementBatchByProviderNoTx(ctx context.Context, tx *sql.Tx, provider string, providerSettlementNo string, forUpdate bool) (dashboard.SaaSPaymentSettlementBatch, bool, error) {
	query := saasPaymentSettlementBatchSelectSQL() + ` WHERE b.provider = ? AND b.provider_settlement_no = ? LIMIT 1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	item, err := scanSaaSPaymentSettlementBatch(tx.QueryRowContext(ctx, query, strings.TrimSpace(provider), strings.TrimSpace(providerSettlementNo)))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSPaymentSettlementBatch{}, false, nil
	}
	return item, err == nil, err
}

func saasPaymentSettlementBatchBySourceTx(ctx context.Context, tx *sql.Tx, provider string, sourceSHA string, forUpdate bool) (dashboard.SaaSPaymentSettlementBatch, bool, error) {
	query := saasPaymentSettlementBatchSelectSQL() + ` WHERE b.provider = ? AND b.source_sha256 = ? LIMIT 1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	item, err := scanSaaSPaymentSettlementBatch(tx.QueryRowContext(ctx, query, strings.TrimSpace(provider), strings.TrimSpace(sourceSHA)))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSPaymentSettlementBatch{}, false, nil
	}
	return item, err == nil, err
}

func saasPaymentSettlementEntriesByBatchTx(ctx context.Context, tx *sql.Tx, batchNo string, forUpdate bool) ([]dashboard.SaaSPaymentSettlementEntry, error) {
	query := saasPaymentSettlementEntrySelectSQL() + ` WHERE b.batch_no = ? ORDER BY e.line_no ASC, e.id ASC`
	if forUpdate {
		query += " FOR UPDATE"
	}
	rows, err := tx.QueryContext(ctx, query, strings.TrimSpace(batchNo))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSPaymentSettlementEntry, 0)
	for rows.Next() {
		item, err := scanSaaSPaymentSettlementEntry(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func saasPaymentSettlementEntryByIDTx(ctx context.Context, tx *sql.Tx, entryID int64, forUpdate bool) (dashboard.SaaSPaymentSettlementEntry, error) {
	query := saasPaymentSettlementEntrySelectSQL() + ` WHERE e.id = ? LIMIT 1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	return scanSaaSPaymentSettlementEntry(tx.QueryRowContext(ctx, query, entryID))
}

func saasPaymentSettlementBatchSelectSQL() string {
	return `
		SELECT b.id, b.batch_no, b.provider, b.provider_settlement_no,
			COALESCE(DATE_FORMAT(b.period_start, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(b.period_end, '%Y-%m-%d %H:%i:%s'), ''),
			b.currency, b.status, b.source_sha256,
			b.entry_count, b.payment_count, b.refund_count, b.matched_count, b.issue_count,
			b.open_issue_count, b.resolved_issue_count, b.ignored_issue_count,
			b.total_amount_cents, b.total_fee_cents, b.total_net_cents, b.difference_amount_cents,
			b.imported_by_user_id, b.imported_by_tenant_id,
			COALESCE(DATE_FORMAT(b.imported_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(b.reconciled_at, '%Y-%m-%d %H:%i:%s'), ''),
			b.closed_by_user_id, COALESCE(DATE_FORMAT(b.closed_at, '%Y-%m-%d %H:%i:%s'), ''),
			b.close_reason, b.version, b.remark,
			COALESCE(DATE_FORMAT(b.created_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(b.updated_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_payment_settlement_batches b
	`
}

func scanSaaSPaymentSettlementBatch(scanner saasPaymentScanner) (dashboard.SaaSPaymentSettlementBatch, error) {
	var item dashboard.SaaSPaymentSettlementBatch
	err := scanner.Scan(
		&item.ID, &item.BatchNo, &item.Provider, &item.ProviderSettlementNo,
		&item.PeriodStart, &item.PeriodEnd, &item.Currency, &item.Status, &item.SourceSHA256,
		&item.EntryCount, &item.PaymentCount, &item.RefundCount, &item.MatchedCount, &item.IssueCount,
		&item.OpenIssueCount, &item.ResolvedIssueCount, &item.IgnoredIssueCount,
		&item.TotalAmountCents, &item.TotalFeeCents, &item.TotalNetCents, &item.DifferenceAmountCents,
		&item.ImportedByUserID, &item.ImportedByTenantID, &item.ImportedAt, &item.ReconciledAt,
		&item.ClosedByUserID, &item.ClosedAt, &item.CloseReason, &item.Version, &item.Remark,
		&item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

func saasPaymentSettlementEntrySelectSQL() string {
	return `
		SELECT e.id, e.batch_id, b.batch_no, b.status, e.line_no, e.provider, e.provider_transaction_no, e.transaction_type,
			e.order_no, e.provider_order_no, e.refund_no, e.provider_refund_no,
			e.amount_cents, e.fee_cents, e.net_amount_cents, e.currency,
			COALESCE(DATE_FORMAT(e.occurred_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(e.matched_payment_order_id, 0), COALESCE(e.matched_refund_id, 0), e.matched_tenant_id,
			COALESCE(t.name, ''), e.matched_internal_no, e.expected_amount_cents, e.expected_currency, e.expected_status,
			e.reconciliation_status, e.difference_amount_cents, COALESCE(CAST(e.issue_codes_json AS CHAR), ''), e.issue_message,
			e.handling_status, e.handled_by_user_id, e.handled_by_tenant_id,
			COALESCE(DATE_FORMAT(e.handled_at, '%Y-%m-%d %H:%i:%s'), ''), e.handling_reason, e.version,
			COALESCE(CAST(e.raw_json AS CHAR), ''),
			COALESCE(DATE_FORMAT(e.created_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(e.updated_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_payment_settlement_entries e
		JOIN mochat_go_saas_payment_settlement_batches b ON b.id = e.batch_id
		LEFT JOIN mc_tenant t ON t.id = e.matched_tenant_id AND t.deleted_at IS NULL
	`
}

func scanSaaSPaymentSettlementEntry(scanner saasPaymentScanner) (dashboard.SaaSPaymentSettlementEntry, error) {
	var item dashboard.SaaSPaymentSettlementEntry
	err := scanner.Scan(
		&item.ID, &item.BatchID, &item.BatchNo, &item.BatchStatus, &item.LineNo, &item.Provider, &item.ProviderTransactionNo, &item.TransactionType,
		&item.OrderNo, &item.ProviderOrderNo, &item.RefundNo, &item.ProviderRefundNo,
		&item.AmountCents, &item.FeeCents, &item.NetAmountCents, &item.Currency, &item.OccurredAt,
		&item.MatchedPaymentOrderID, &item.MatchedRefundID, &item.MatchedTenantID, &item.MatchedTenantName,
		&item.MatchedInternalNo, &item.ExpectedAmountCents, &item.ExpectedCurrency, &item.ExpectedStatus,
		&item.ReconciliationStatus, &item.DifferenceAmountCents, &item.IssueCodesJSON, &item.IssueMessage,
		&item.HandlingStatus, &item.HandledByUserID, &item.HandledByTenantID, &item.HandledAt,
		&item.HandlingReason, &item.Version, &item.RawJSON, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

func saasPaymentSettlementBatchWhere(options dashboard.SaaSPaymentSettlementBatchOptions) (string, []any) {
	where := []string{"1 = 1"}
	args := make([]any, 0)
	if options.BatchNo != "" {
		where = append(where, "b.batch_no = ?")
		args = append(args, options.BatchNo)
	}
	if options.Provider != "" {
		where = append(where, "b.provider = ?")
		args = append(args, options.Provider)
	}
	if options.Status != "" && options.Status != dashboard.SaaSPaymentSettlementBatchStatusAll {
		where = append(where, "b.status = ?")
		args = append(args, options.Status)
	}
	if options.Currency != "" {
		where = append(where, "b.currency = ?")
		args = append(args, options.Currency)
	}
	if options.Keyword != "" {
		like := "%" + options.Keyword + "%"
		where = append(where, "(b.batch_no LIKE ? OR b.provider_settlement_no LIKE ? OR b.remark LIKE ?)")
		args = append(args, like, like, like)
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

func saasPaymentSettlementEntryWhere(options dashboard.SaaSPaymentSettlementEntryOptions) (string, []any) {
	where := []string{"1 = 1"}
	args := make([]any, 0)
	if options.BatchNo != "" {
		where = append(where, "b.batch_no = ?")
		args = append(args, options.BatchNo)
	}
	if options.Provider != "" {
		where = append(where, "e.provider = ?")
		args = append(args, options.Provider)
	}
	if options.TransactionType != "" && options.TransactionType != dashboard.SaaSPaymentSettlementTransactionAll {
		where = append(where, "e.transaction_type = ?")
		args = append(args, options.TransactionType)
	}
	if options.ReconciliationStatus != "" && options.ReconciliationStatus != dashboard.SaaSPaymentSettlementReconciliationAll {
		where = append(where, "e.reconciliation_status = ?")
		args = append(args, options.ReconciliationStatus)
	}
	if options.HandlingStatus != "" && options.HandlingStatus != dashboard.SaaSPaymentSettlementHandlingAll {
		where = append(where, "e.handling_status = ?")
		args = append(args, options.HandlingStatus)
	}
	if options.Keyword != "" {
		like := "%" + options.Keyword + "%"
		where = append(where, `(e.provider_transaction_no LIKE ? OR e.order_no LIKE ? OR e.provider_order_no LIKE ?
			OR e.refund_no LIKE ? OR e.provider_refund_no LIKE ? OR e.matched_internal_no LIKE ?
			OR e.issue_message LIKE ? OR e.handling_reason LIKE ? OR COALESCE(t.name, '') LIKE ?)`)
		args = append(args, like, like, like, like, like, like, like, like, like)
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

func summarizeSaaSPaymentSettlementBatches(items []dashboard.SaaSPaymentSettlementBatch) dashboard.SaaSPaymentSettlementBatchSummary {
	var summary dashboard.SaaSPaymentSettlementBatchSummary
	providers := map[string]struct{}{}
	currencies := map[string]struct{}{}
	for _, item := range items {
		summary.BatchCount++
		if item.Status == dashboard.SaaSPaymentSettlementBatchStatusClosed {
			summary.ClosedCount++
		} else {
			summary.ReconciledCount++
		}
		summary.EntryCount += item.EntryCount
		summary.PaymentCount += item.PaymentCount
		summary.RefundCount += item.RefundCount
		summary.MatchedCount += item.MatchedCount
		summary.IssueCount += item.IssueCount
		summary.OpenIssueCount += item.OpenIssueCount
		summary.ResolvedIssueCount += item.ResolvedIssueCount
		summary.IgnoredIssueCount += item.IgnoredIssueCount
		summary.TotalAmountCents += item.TotalAmountCents
		summary.TotalFeeCents += item.TotalFeeCents
		summary.TotalNetCents += item.TotalNetCents
		summary.DifferenceAmountCents += item.DifferenceAmountCents
		providers[item.Provider] = struct{}{}
		currencies[item.Currency] = struct{}{}
	}
	summary.ProviderCount = len(providers)
	summary.CurrencyCount = len(currencies)
	return summary
}

func summarizeSaaSPaymentSettlementEntries(items []dashboard.SaaSPaymentSettlementEntry) dashboard.SaaSPaymentSettlementEntrySummary {
	var summary dashboard.SaaSPaymentSettlementEntrySummary
	tenants := map[int]struct{}{}
	for _, item := range items {
		summary.EntryCount++
		if item.TransactionType == dashboard.SaaSPaymentSettlementTransactionPayment {
			summary.PaymentCount++
		} else if item.TransactionType == dashboard.SaaSPaymentSettlementTransactionRefund {
			summary.RefundCount++
		}
		summary.TotalAmountCents += item.AmountCents
		summary.TotalFeeCents += item.FeeCents
		summary.TotalNetCents += item.NetAmountCents
		summary.ExpectedAmountCents += item.ExpectedAmountCents
		summary.DifferenceAmountCents += item.DifferenceAmountCents
		if item.MatchedTenantID > 0 {
			tenants[item.MatchedTenantID] = struct{}{}
		}
		switch item.ReconciliationStatus {
		case dashboard.SaaSPaymentSettlementReconciliationMatched:
			summary.MatchedCount++
		case dashboard.SaaSPaymentSettlementReconciliationMissingInternal:
			summary.MissingInternalCount++
		case dashboard.SaaSPaymentSettlementReconciliationIdentifierConflict:
			summary.IdentifierConflictCount++
		case dashboard.SaaSPaymentSettlementReconciliationStatusMismatch:
			summary.StatusMismatchCount++
		case dashboard.SaaSPaymentSettlementReconciliationCurrencyMismatch:
			summary.CurrencyMismatchCount++
		case dashboard.SaaSPaymentSettlementReconciliationAmountMismatch:
			summary.AmountMismatchCount++
		}
		switch item.HandlingStatus {
		case dashboard.SaaSPaymentSettlementHandlingOpen:
			summary.OpenIssueCount++
		case dashboard.SaaSPaymentSettlementHandlingResolved:
			summary.ResolvedIssueCount++
		case dashboard.SaaSPaymentSettlementHandlingIgnored:
			summary.IgnoredIssueCount++
		}
	}
	summary.TenantCount = len(tenants)
	return summary
}

func applySaaSPaymentSettlementEntrySummary(batch dashboard.SaaSPaymentSettlementBatch, summary dashboard.SaaSPaymentSettlementEntrySummary) dashboard.SaaSPaymentSettlementBatch {
	batch.EntryCount = summary.EntryCount
	batch.PaymentCount = summary.PaymentCount
	batch.RefundCount = summary.RefundCount
	batch.MatchedCount = summary.MatchedCount
	batch.IssueCount = summary.EntryCount - summary.MatchedCount
	batch.OpenIssueCount = summary.OpenIssueCount
	batch.ResolvedIssueCount = summary.ResolvedIssueCount
	batch.IgnoredIssueCount = summary.IgnoredIssueCount
	batch.TotalAmountCents = summary.TotalAmountCents
	batch.TotalFeeCents = summary.TotalFeeCents
	batch.TotalNetCents = summary.TotalNetCents
	batch.DifferenceAmountCents = summary.DifferenceAmountCents
	return batch
}

func saasPaymentSettlementBatchStatePayload(item dashboard.SaaSPaymentSettlementBatch) map[string]any {
	return map[string]any{
		"batchNo": item.BatchNo, "provider": item.Provider, "providerSettlementNo": item.ProviderSettlementNo,
		"status": item.Status, "entryCount": item.EntryCount, "matchedCount": item.MatchedCount,
		"issueCount": item.IssueCount, "openIssueCount": item.OpenIssueCount,
		"totalAmountCents": item.TotalAmountCents, "totalFeeCents": item.TotalFeeCents,
		"totalNetCents": item.TotalNetCents, "differenceAmountCents": item.DifferenceAmountCents, "version": item.Version,
	}
}

func saasPaymentSettlementEntryStatePayload(item dashboard.SaaSPaymentSettlementEntry) map[string]any {
	return map[string]any{
		"entryId": item.ID, "batchNo": item.BatchNo, "providerTransactionNo": item.ProviderTransactionNo,
		"reconciliationStatus": item.ReconciliationStatus, "handlingStatus": item.HandlingStatus,
		"handlingReason": item.HandlingReason, "differenceAmountCents": item.DifferenceAmountCents, "version": item.Version,
	}
}

func clampSaaSPaymentSettlementLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > saasPaymentSettlementQueryMaxLimit {
		return saasPaymentSettlementQueryMaxLimit
	}
	return limit
}

func nullablePositiveInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func uniqueStringsInOrder(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func strconvFormatInt64(value int64) string {
	return fmt.Sprintf("%d", value)
}
