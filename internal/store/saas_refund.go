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

func (s *MySQLStore) SaaSAdminPaymentRefunds(ctx context.Context, options dashboard.SaaSAdminPaymentRefundOptions) (dashboard.SaaSAdminPaymentRefundReport, error) {
	if options.Limit <= 0 {
		options.Limit = 100
	}
	if options.Limit > saasPaymentQueryMaxLimit {
		options.Limit = saasPaymentQueryMaxLimit
	}
	where := []string{"r.deleted_at IS NULL"}
	args := []any{}
	if options.TenantID > 0 {
		where = append(where, "r.tenant_id = ?")
		args = append(args, options.TenantID)
	}
	if options.Status != "" && options.Status != dashboard.SaaSPaymentRefundStatusAll {
		where = append(where, "r.status = ?")
		args = append(args, options.Status)
	}
	if strings.TrimSpace(options.Provider) != "" {
		where = append(where, "r.provider = ?")
		args = append(args, strings.TrimSpace(options.Provider))
	}
	if strings.TrimSpace(options.OrderNo) != "" {
		where = append(where, "r.order_no = ?")
		args = append(args, strings.TrimSpace(options.OrderNo))
	}
	if strings.TrimSpace(options.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(options.Keyword) + "%"
		where = append(where, "(r.refund_no LIKE ? OR r.order_no LIKE ? OR COALESCE(r.provider_refund_no, '') LIKE ? OR CAST(r.tenant_id AS CHAR) LIKE ? OR COALESCE(t.name, '') LIKE ? OR r.reason LIKE ? OR r.failure_code LIKE ? OR r.failure_message LIKE ? OR r.remark LIKE ?)")
		args = append(args, keyword, keyword, keyword, keyword, keyword, keyword, keyword, keyword, keyword)
	}
	args = append(args, saasPaymentQueryMaxLimit)
	rows, err := s.db.QueryContext(ctx, saasAdminPaymentRefundSelectSQL()+`
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY
			CASE r.status WHEN 'requested' THEN 1 WHEN 'processing' THEN 2 WHEN 'failed' THEN 3 WHEN 'succeeded' THEN 4 WHEN 'canceled' THEN 5 ELSE 6 END,
			r.id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return dashboard.SaaSAdminPaymentRefundReport{}, err
	}
	defer rows.Close()
	all := make([]dashboard.SaaSAdminPaymentRefund, 0)
	for rows.Next() {
		item, err := scanSaaSAdminPaymentRefund(rows)
		if err != nil {
			return dashboard.SaaSAdminPaymentRefundReport{}, err
		}
		all = append(all, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSAdminPaymentRefundReport{}, err
	}
	report := dashboard.SaaSAdminPaymentRefundReport{Options: options, Summary: summarizeSaaSPaymentRefunds(all)}
	if len(all) > options.Limit {
		all = all[:options.Limit]
	}
	report.Refunds = all
	return report, nil
}

func (s *MySQLStore) CreateSaaSAdminPaymentRefund(ctx context.Context, create dashboard.SaaSAdminPaymentRefundCreate) (dashboard.SaaSAdminPaymentRefundCreateResult, error) {
	if strings.TrimSpace(create.RefundNo) == "" || strings.TrimSpace(create.OrderNo) == "" || create.AmountCents <= 0 {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, dashboard.NewSaaSAdminBadRequest("refundNo, orderNo and positive amountCents are required")
	}
	if !dashboard.SaaSPaymentRefundEntitlementActionValid(create.EntitlementAction) {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, dashboard.NewSaaSAdminBadRequest("refund entitlement action invalid")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
	}
	defer rollbackQuietly(tx)

	order, found, err := saasPaymentOrderByNoTx(ctx, tx, strings.TrimSpace(create.OrderNo), true)
	if err != nil {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
	}
	if !found {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, dashboard.NewSaaSAdminNotFound("payment order not found")
	}
	if order.Status != dashboard.SaaSPaymentOrderStatusPaid {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, saasPaymentConflict("only paid payment order can be refunded")
	}
	if strings.TrimSpace(create.Currency) == "" {
		create.Currency = order.Currency
	}
	if strings.TrimSpace(create.Currency) != order.Currency {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, saasPaymentUnprocessable("refund currency mismatch")
	}
	if strings.TrimSpace(create.IdempotencyKey) != "" {
		existing, found, err := saasPaymentRefundByIdempotencyTx(ctx, tx, order.ID, strings.TrimSpace(create.IdempotencyKey), true)
		if err != nil {
			return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
		}
		if found {
			if !saasPaymentRefundMatchesCreate(existing, create) {
				return dashboard.SaaSAdminPaymentRefundCreateResult{}, saasPaymentConflict("payment refund idempotency key already used with different request")
			}
			if err := markSaaSAdminApprovalEffectTx(ctx, tx, create.ApprovalExecutionID, create.ApprovalExecutionVersion, create.ActorUserID, 0); err != nil {
				return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
			}
			if err := tx.Commit(); err != nil {
				return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
			}
			return dashboard.SaaSAdminPaymentRefundCreateResult{Refund: existing, Order: order, Idempotent: true}, nil
		}
	}
	if create.AmountCents > order.RefundableAmountCents {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, saasPaymentConflict(fmt.Sprintf("refund amount exceeds refundable amount: available=%d", order.RefundableAmountCents))
	}
	projectedNetPaid := saasPaymentPositiveDifference(order.AmountCents, order.RefundedAmountCents+order.RefundPendingCents+create.AmountCents)
	availableForPendingInvoices := saasPaymentPositiveDifference(projectedNetPaid, order.NetInvoicedCents)
	if order.InvoicePendingCents > availableForPendingInvoices {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, saasPaymentConflict("cancel pending invoice requests before creating this refund")
	}
	if create.EntitlementAction != dashboard.SaaSPaymentRefundEntitlementKeep &&
		(order.RefundPendingCents != 0 || order.RefundedAmountCents+create.AmountCents != order.AmountCents) {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, saasPaymentUnprocessable("suspend or cancel entitlement requires a full refund with no other pending refund")
	}

	result, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO mochat_go_saas_payment_refunds
			(refund_no, payment_order_id, order_no, tenant_id, provider, provider_refund_no,
			 idempotency_key, status, amount_cents, currency, reason, entitlement_action,
			 latest_webhook_event_id, billing_event_id, subscription_event_id, subscription_operation_id,
			 requested_by_user_id, requested_by_tenant_id, requested_at, version, remark, metadata_json,
			 created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'requested', ?, ?, ?, ?, 0, 0, 0, 0, ?, ?, NOW(), 1, ?, ?, NOW(), NOW(), NULL)
	`, strings.TrimSpace(create.RefundNo), order.ID, order.OrderNo, order.TenantID, order.Provider,
		nullableTrimmedString(create.ProviderRefundNo), nullableTrimmedString(create.IdempotencyKey),
		create.AmountCents, order.Currency, truncateRunes(create.Reason, 255), create.EntitlementAction,
		create.ActorUserID, create.ActorTenantID, truncateRunes(create.Remark, 255), saasAdminJSONValue(create.MetadataJSON))
	if err != nil {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		existing, found, err := saasPaymentRefundByNoTx(ctx, tx, strings.TrimSpace(create.RefundNo), true)
		if err != nil {
			return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
		}
		if !found && strings.TrimSpace(create.IdempotencyKey) != "" {
			existing, found, err = saasPaymentRefundByIdempotencyTx(ctx, tx, order.ID, strings.TrimSpace(create.IdempotencyKey), true)
			if err != nil {
				return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
			}
		}
		if !found || !saasPaymentRefundMatchesCreate(existing, create) {
			return dashboard.SaaSAdminPaymentRefundCreateResult{}, saasPaymentConflict("refund number, idempotency key or provider refund number already exists")
		}
		if err := markSaaSAdminApprovalEffectTx(ctx, tx, create.ApprovalExecutionID, create.ApprovalExecutionVersion, create.ActorUserID, 0); err != nil {
			return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
		}
		return dashboard.SaaSAdminPaymentRefundCreateResult{Refund: existing, Order: order, Idempotent: true}, nil
	}
	refundID, _ := result.LastInsertId()
	reservationResult, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_orders
		SET refund_pending_amount_cents = refund_pending_amount_cents + ?, latest_refund_id = ?,
			version = version + 1, updated_at = NOW()
		WHERE id = ? AND status = 'paid' AND deleted_at IS NULL
			AND amount_cents >= refunded_amount_cents + refund_pending_amount_cents + ?
	`, create.AmountCents, refundID, order.ID, create.AmountCents)
	if err != nil {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
	}
	if affected, _ := reservationResult.RowsAffected(); affected != 1 {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, saasPaymentConflict("refund reservation changed concurrently")
	}
	refund, found, err := saasPaymentRefundByNoTx(ctx, tx, strings.TrimSpace(create.RefundNo), true)
	if err != nil || !found {
		if err == nil {
			err = errors.New("payment refund insert did not return a row")
		}
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
	}
	nextOrder, found, err := saasPaymentOrderByNoTx(ctx, tx, order.OrderNo, true)
	if err != nil || !found {
		if err == nil {
			err = errors.New("payment order missing after refund reservation")
		}
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: order.TenantID, ActorUserID: create.ActorUserID, ActorTenantID: create.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionPaymentRefundRequest, TargetType: dashboard.SaaSAdminOperationTargetPaymentRefund,
		TargetID: refund.RefundNo, TargetName: order.TenantName,
		AfterJSON: saasAdminMarshalJSON(map[string]any{"refund": saasPaymentRefundStatePayload(refund), "order": saasPaymentOrderStatePayload(nextOrder)}),
		Remark:    strings.TrimSpace(create.Reason),
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, create.ApprovalExecutionID, create.ApprovalExecutionVersion, create.ActorUserID, operationID); err != nil {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminPaymentRefundCreateResult{}, err
	}
	return dashboard.SaaSAdminPaymentRefundCreateResult{Refund: refund, Order: nextOrder, OperationID: operationID}, nil
}

func (s *MySQLStore) CancelSaaSAdminPaymentRefund(ctx context.Context, cancel dashboard.SaaSAdminPaymentRefundCancel) (dashboard.SaaSAdminPaymentRefundCancelResult, error) {
	if strings.TrimSpace(cancel.RefundNo) == "" || cancel.ExpectedVersion <= 0 {
		return dashboard.SaaSAdminPaymentRefundCancelResult{}, dashboard.NewSaaSAdminBadRequest("refundNo and expectedVersion are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminPaymentRefundCancelResult{}, err
	}
	defer rollbackQuietly(tx)
	current, found, err := saasPaymentRefundByNoTx(ctx, tx, strings.TrimSpace(cancel.RefundNo), true)
	if err != nil {
		return dashboard.SaaSAdminPaymentRefundCancelResult{}, err
	}
	if !found {
		return dashboard.SaaSAdminPaymentRefundCancelResult{}, dashboard.NewSaaSAdminNotFound("payment refund not found")
	}
	if current.Version != cancel.ExpectedVersion {
		return dashboard.SaaSAdminPaymentRefundCancelResult{}, saasPaymentConflict(fmt.Sprintf("payment refund version conflict: current=%d", current.Version))
	}
	if current.Status != dashboard.SaaSPaymentRefundStatusRequested {
		return dashboard.SaaSAdminPaymentRefundCancelResult{}, saasPaymentConflict("only requested refund can be canceled")
	}
	order, found, err := saasPaymentOrderByNoTx(ctx, tx, current.OrderNo, true)
	if err != nil || !found {
		if err == nil {
			err = dashboard.NewSaaSAdminNotFound("payment order not found")
		}
		return dashboard.SaaSAdminPaymentRefundCancelResult{}, err
	}
	refundUpdate, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_refunds
		SET status = 'canceled', canceled_at = NOW(), failure_code = '', failure_message = ?,
			version = version + 1, updated_at = NOW()
		WHERE id = ? AND version = ? AND status = 'requested' AND deleted_at IS NULL
	`, truncateRunes(cancel.Reason, 255), current.ID, current.Version)
	if err != nil {
		return dashboard.SaaSAdminPaymentRefundCancelResult{}, err
	}
	if affected, _ := refundUpdate.RowsAffected(); affected != 1 {
		return dashboard.SaaSAdminPaymentRefundCancelResult{}, saasPaymentConflict("payment refund changed concurrently")
	}
	if err := releaseSaaSPaymentRefundReservationTx(ctx, tx, order.ID, current.AmountCents, current.ID); err != nil {
		return dashboard.SaaSAdminPaymentRefundCancelResult{}, err
	}
	nextRefund, _, err := saasPaymentRefundByNoTx(ctx, tx, current.RefundNo, true)
	if err != nil {
		return dashboard.SaaSAdminPaymentRefundCancelResult{}, err
	}
	nextOrder, _, err := saasPaymentOrderByNoTx(ctx, tx, order.OrderNo, true)
	if err != nil {
		return dashboard.SaaSAdminPaymentRefundCancelResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: current.TenantID, ActorUserID: cancel.ActorUserID, ActorTenantID: cancel.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionPaymentRefundCancel, TargetType: dashboard.SaaSAdminOperationTargetPaymentRefund,
		TargetID: current.RefundNo, TargetName: current.TenantName,
		BeforeJSON: saasAdminMarshalJSON(saasPaymentRefundStatePayload(current)),
		AfterJSON:  saasAdminMarshalJSON(map[string]any{"refund": saasPaymentRefundStatePayload(nextRefund), "order": saasPaymentOrderStatePayload(nextOrder)}),
		Remark:     cancel.Reason,
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminPaymentRefundCancelResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminPaymentRefundCancelResult{}, err
	}
	return dashboard.SaaSAdminPaymentRefundCancelResult{Refund: nextRefund, Order: nextOrder, PreviousStatus: current.Status, OperationID: operationID}, nil
}

func (s *MySQLStore) processSaaSPaymentRefundWebhookTx(ctx context.Context, tx *sql.Tx, event dashboard.SaaSAdminPaymentWebhookEvent, input dashboard.SaaSPaymentWebhookInput) (dashboard.SaaSPaymentWebhookResult, error) {
	refund, found, err := saasPaymentRefundByNoTx(ctx, tx, strings.TrimSpace(input.RefundNo), true)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if !found {
		return s.failSaaSPaymentWebhookTx(ctx, tx, event.ID, "payment refund not found", dashboard.NewSaaSAdminNotFound("payment refund not found"))
	}
	if refund.OrderNo != strings.TrimSpace(input.OrderNo) {
		return s.failSaaSPaymentRefundWebhookTx(ctx, tx, event.ID, refund, input, "payment refund order mismatch", saasPaymentUnprocessable("payment refund order mismatch"))
	}
	if refund.Provider != strings.TrimSpace(input.Provider) {
		return s.failSaaSPaymentRefundWebhookTx(ctx, tx, event.ID, refund, input, "payment refund provider mismatch", saasPaymentUnprocessable("payment refund provider mismatch"))
	}
	if refund.ProviderRefundNo != "" && input.ProviderRefundNo != "" && refund.ProviderRefundNo != input.ProviderRefundNo {
		return s.failSaaSPaymentRefundWebhookTx(ctx, tx, event.ID, refund, input, "provider refund number mismatch", saasPaymentUnprocessable("provider refund number mismatch"))
	}
	order, found, err := saasPaymentOrderByNoTx(ctx, tx, refund.OrderNo, true)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if !found {
		return s.failSaaSPaymentRefundWebhookTx(ctx, tx, event.ID, refund, input, "payment order not found", dashboard.NewSaaSAdminNotFound("payment order not found"))
	}
	if refund.Status == dashboard.SaaSPaymentRefundStatusSucceeded || refund.Status == dashboard.SaaSPaymentRefundStatusFailed || refund.Status == dashboard.SaaSPaymentRefundStatusCanceled {
		if err := finalizeSaaSRefundWebhookEventTx(ctx, tx, event.ID, order, refund, input, dashboard.SaaSPaymentWebhookEventStatusIgnored, "payment refund already terminal"); err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
		updatedEvent, err := saasPaymentWebhookEventByIDTx(ctx, tx, event.ID)
		if err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
		return dashboard.SaaSPaymentWebhookResult{Event: updatedEvent, Order: order, Refund: refund, Ignored: true, BillingEventID: refund.BillingEventID}, nil
	}
	if input.EventType == dashboard.SaaSPaymentWebhookTypeRefundProcessing && refund.Status == dashboard.SaaSPaymentRefundStatusProcessing {
		if err := finalizeSaaSRefundWebhookEventTx(ctx, tx, event.ID, order, refund, input, dashboard.SaaSPaymentWebhookEventStatusIgnored, "payment refund already processing"); err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
		updatedEvent, err := saasPaymentWebhookEventByIDTx(ctx, tx, event.ID)
		return dashboard.SaaSPaymentWebhookResult{Event: updatedEvent, Order: order, Refund: refund, Ignored: true}, err
	}
	switch input.EventType {
	case dashboard.SaaSPaymentWebhookTypeRefundSucceeded:
		return s.settleSaaSPaymentRefundTx(ctx, tx, order, refund, event, input)
	case dashboard.SaaSPaymentWebhookTypeRefundFailed:
		return s.updateSaaSPaymentRefundFromWebhookTx(ctx, tx, order, refund, event, input, dashboard.SaaSPaymentRefundStatusFailed)
	case dashboard.SaaSPaymentWebhookTypeRefundCanceled:
		return s.updateSaaSPaymentRefundFromWebhookTx(ctx, tx, order, refund, event, input, dashboard.SaaSPaymentRefundStatusCanceled)
	case dashboard.SaaSPaymentWebhookTypeRefundProcessing:
		return s.updateSaaSPaymentRefundFromWebhookTx(ctx, tx, order, refund, event, input, dashboard.SaaSPaymentRefundStatusProcessing)
	default:
		return dashboard.SaaSPaymentWebhookResult{}, dashboard.NewSaaSAdminBadRequest("refund webhook type invalid")
	}
}

func (s *MySQLStore) settleSaaSPaymentRefundTx(ctx context.Context, tx *sql.Tx, order dashboard.SaaSAdminPaymentOrder, refund dashboard.SaaSAdminPaymentRefund, event dashboard.SaaSAdminPaymentWebhookEvent, input dashboard.SaaSPaymentWebhookInput) (dashboard.SaaSPaymentWebhookResult, error) {
	if order.Status != dashboard.SaaSPaymentOrderStatusPaid {
		return s.failSaaSPaymentRefundWebhookTx(ctx, tx, event.ID, refund, input, "payment order is not paid", saasPaymentUnprocessable("payment order is not paid"))
	}
	if input.AmountCents != refund.AmountCents || strings.TrimSpace(input.Currency) != refund.Currency {
		return s.failSaaSPaymentRefundWebhookTx(ctx, tx, event.ID, refund, input, "refund amount or currency mismatch", saasPaymentUnprocessable("refund amount or currency mismatch"))
	}
	if order.RefundPendingCents < refund.AmountCents || order.RefundedAmountCents+refund.AmountCents > order.AmountCents {
		return s.failSaaSPaymentRefundWebhookTx(ctx, tx, event.ID, refund, input, "refund reservation invalid", saasPaymentConflict("refund reservation invalid"))
	}
	refundedAt := strings.TrimSpace(input.RefundedAt)
	if refundedAt == "" {
		refundedAt = strings.TrimSpace(input.OccurredAt)
	}
	metadata := map[string]any{
		"source": "payment_refund_webhook", "paymentOrderNo": order.OrderNo, "refundNo": refund.RefundNo,
		"provider": refund.Provider, "providerRefundNo": input.ProviderRefundNo, "webhookEventId": input.EventID,
		"originalBillingEventId": order.BillingEventID, "entitlementAction": refund.EntitlementAction,
	}
	if strings.TrimSpace(input.MetadataJSON) != "" {
		metadata["providerMetadata"] = json.RawMessage(input.MetadataJSON)
	}
	billingResult, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_billing_events
			(tenant_id, event_type, package_code, package_name, previous_expires_at, new_expires_at,
			 amount_cents, currency, paid_at, payment_method, external_order_no,
			 actor_user_id, actor_tenant_id, remark, metadata_json, created_at, updated_at, deleted_at)
		VALUES (?, 'refund', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NULL)
	`, order.TenantID, order.PackageCode, order.PackageName,
		nullableTrimmedString(order.ServiceExpiresAt), nullableTrimmedString(order.ServiceExpiresAt),
		refund.AmountCents, refund.Currency, nullableTrimmedString(refundedAt), refund.Provider, refund.RefundNo,
		refund.RequestedByUserID, refund.RequestedByTenantID, truncateRunes("refund webhook "+input.EventID, 255),
		saasAdminJSONValue(saasAdminMarshalJSON(metadata)))
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	billingEventID, _ := billingResult.LastInsertId()
	subscriptionEventID, subscriptionOperationID, err := s.applySaaSPaymentRefundEntitlementTx(ctx, tx, order, refund, refundedAt)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	providerRefundNo := strings.TrimSpace(input.ProviderRefundNo)
	refundUpdate, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_refunds
		SET status = 'succeeded', provider_refund_no = COALESCE(NULLIF(?, ''), provider_refund_no),
			latest_webhook_event_id = ?, billing_event_id = ?, subscription_event_id = ?, subscription_operation_id = ?,
			succeeded_at = ?, failed_at = NULL, canceled_at = NULL, failure_code = '', failure_message = '',
			version = version + 1, updated_at = NOW()
		WHERE id = ? AND status IN ('requested', 'processing') AND deleted_at IS NULL
	`, providerRefundNo, event.ID, billingEventID, subscriptionEventID, subscriptionOperationID,
		nullableTrimmedString(refundedAt), refund.ID)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if affected, _ := refundUpdate.RowsAffected(); affected != 1 {
		return dashboard.SaaSPaymentWebhookResult{}, saasPaymentConflict("payment refund changed concurrently")
	}
	orderUpdate, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_orders
		SET refund_pending_amount_cents = CASE
				WHEN refund_pending_amount_cents >= ? THEN refund_pending_amount_cents - ? ELSE 0 END,
			refunded_amount_cents = refunded_amount_cents + ?, latest_refund_id = ?,
			version = version + 1, updated_at = NOW()
		WHERE id = ? AND status = 'paid' AND deleted_at IS NULL
			AND refund_pending_amount_cents >= ? AND amount_cents >= refunded_amount_cents + ?
	`, refund.AmountCents, refund.AmountCents, refund.AmountCents, refund.ID, order.ID, refund.AmountCents, refund.AmountCents)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if affected, _ := orderUpdate.RowsAffected(); affected != 1 {
		return dashboard.SaaSPaymentWebhookResult{}, saasPaymentConflict("payment refund reservation changed concurrently")
	}
	nextRefund, _, err := saasPaymentRefundByNoTx(ctx, tx, refund.RefundNo, true)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	nextOrder, _, err := saasPaymentOrderByNoTx(ctx, tx, order.OrderNo, true)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if err := finalizeSaaSRefundWebhookEventTx(ctx, tx, event.ID, nextOrder, nextRefund, input, dashboard.SaaSPaymentWebhookEventStatusProcessed, ""); err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if _, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: refund.TenantID, ActorUserID: refund.RequestedByUserID, ActorTenantID: refund.RequestedByTenantID,
		Action: dashboard.SaaSAdminOperationActionPaymentRefundSucceeded, TargetType: dashboard.SaaSAdminOperationTargetPaymentRefund,
		TargetID: refund.RefundNo, TargetName: refund.TenantName,
		BeforeJSON: saasAdminMarshalJSON(saasPaymentRefundStatePayload(refund)),
		AfterJSON:  saasAdminMarshalJSON(map[string]any{"refund": saasPaymentRefundStatePayload(nextRefund), "order": saasPaymentOrderStatePayload(nextOrder), "billingEventId": billingEventID, "webhookEventId": input.EventID}),
		Remark:     "refund webhook " + input.EventID,
	}); err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	updatedEvent, err := saasPaymentWebhookEventByIDTx(ctx, tx, event.ID)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	return dashboard.SaaSPaymentWebhookResult{Event: updatedEvent, Order: nextOrder, Refund: nextRefund, BillingEventID: billingEventID}, nil
}

func (s *MySQLStore) updateSaaSPaymentRefundFromWebhookTx(ctx context.Context, tx *sql.Tx, order dashboard.SaaSAdminPaymentOrder, refund dashboard.SaaSAdminPaymentRefund, event dashboard.SaaSAdminPaymentWebhookEvent, input dashboard.SaaSPaymentWebhookInput, status string) (dashboard.SaaSPaymentWebhookResult, error) {
	providerRefundNo := strings.TrimSpace(input.ProviderRefundNo)
	failureCode := truncateRunes(strings.TrimSpace(input.FailureCode), 64)
	failureMessage := truncateRunes(strings.TrimSpace(input.FailureMessage), 255)
	if status == dashboard.SaaSPaymentRefundStatusFailed && failureMessage == "" {
		failureMessage = "refund failed"
	}
	action := dashboard.SaaSAdminOperationActionPaymentRefundProcess
	var failedAt any
	var canceledAt any
	if status == dashboard.SaaSPaymentRefundStatusFailed {
		action = dashboard.SaaSAdminOperationActionPaymentRefundFailed
		failedAt = nullableTrimmedString(input.OccurredAt)
	} else if status == dashboard.SaaSPaymentRefundStatusCanceled {
		action = dashboard.SaaSAdminOperationActionPaymentRefundCancel
		canceledAt = nullableTrimmedString(input.OccurredAt)
	}
	refundUpdate, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_refunds
		SET status = ?, provider_refund_no = COALESCE(NULLIF(?, ''), provider_refund_no),
			latest_webhook_event_id = ?, failed_at = ?, canceled_at = ?,
			failure_code = ?, failure_message = ?, version = version + 1, updated_at = NOW()
		WHERE id = ? AND status IN ('requested', 'processing') AND deleted_at IS NULL
	`, status, providerRefundNo, event.ID, failedAt, canceledAt, failureCode, failureMessage, refund.ID)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if affected, _ := refundUpdate.RowsAffected(); affected != 1 {
		return dashboard.SaaSPaymentWebhookResult{}, saasPaymentConflict("payment refund changed concurrently")
	}
	if status == dashboard.SaaSPaymentRefundStatusFailed || status == dashboard.SaaSPaymentRefundStatusCanceled {
		if err := releaseSaaSPaymentRefundReservationTx(ctx, tx, order.ID, refund.AmountCents, refund.ID); err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
	} else {
		orderUpdate, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_payment_orders SET latest_refund_id = ?, version = version + 1, updated_at = NOW() WHERE id = ? AND deleted_at IS NULL`, refund.ID, order.ID)
		if err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
		if affected, _ := orderUpdate.RowsAffected(); affected != 1 {
			return dashboard.SaaSPaymentWebhookResult{}, saasPaymentConflict("payment order changed concurrently")
		}
	}
	nextRefund, _, err := saasPaymentRefundByNoTx(ctx, tx, refund.RefundNo, true)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	nextOrder, _, err := saasPaymentOrderByNoTx(ctx, tx, order.OrderNo, true)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if err := finalizeSaaSRefundWebhookEventTx(ctx, tx, event.ID, nextOrder, nextRefund, input, dashboard.SaaSPaymentWebhookEventStatusProcessed, ""); err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if _, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: refund.TenantID, Action: action, TargetType: dashboard.SaaSAdminOperationTargetPaymentRefund,
		TargetID: refund.RefundNo, TargetName: refund.TenantName,
		BeforeJSON: saasAdminMarshalJSON(saasPaymentRefundStatePayload(refund)),
		AfterJSON:  saasAdminMarshalJSON(map[string]any{"refund": saasPaymentRefundStatePayload(nextRefund), "order": saasPaymentOrderStatePayload(nextOrder), "webhookEventId": input.EventID}),
		Remark:     "refund webhook " + input.EventID,
	}); err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	updatedEvent, err := saasPaymentWebhookEventByIDTx(ctx, tx, event.ID)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	return dashboard.SaaSPaymentWebhookResult{Event: updatedEvent, Order: nextOrder, Refund: nextRefund}, nil
}

func (s *MySQLStore) applySaaSPaymentRefundEntitlementTx(ctx context.Context, tx *sql.Tx, order dashboard.SaaSAdminPaymentOrder, refund dashboard.SaaSAdminPaymentRefund, effectiveAt string) (int64, int64, error) {
	if refund.EntitlementAction == dashboard.SaaSPaymentRefundEntitlementKeep {
		return 0, 0, nil
	}
	target := dashboard.SaaSAdminSubscriptionStatusSuspended
	if refund.EntitlementAction == dashboard.SaaSPaymentRefundEntitlementCancel {
		target = dashboard.SaaSAdminSubscriptionStatusCanceled
	}
	current, err := ensureSaaSAdminSubscriptionTx(ctx, tx, order.TenantID)
	if err != nil {
		return 0, 0, err
	}
	if current.Status == target {
		return 0, 0, nil
	}
	if !dashboard.SaaSAdminSubscriptionTransitionAllowed(current.Status, target) {
		return 0, 0, dashboard.NewSaaSAdminBadRequest("refund subscription transition not allowed: " + current.Status + " -> " + target)
	}
	now, err := saasSubscriptionDBNowTx(ctx, tx)
	if err != nil {
		return 0, 0, err
	}
	next := current
	reason := "全额退款 " + refund.RefundNo + " 成功，订阅动作为 " + refund.EntitlementAction
	if err := applySaaSAdminSubscriptionTransitionTx(ctx, tx, &next, dashboard.SaaSAdminSubscriptionTransition{Status: target, Reason: reason}, now); err != nil {
		return 0, 0, err
	}
	next.Version = current.Version + 1
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_subscriptions
		SET status = ?, cancel_at_period_end = ?, canceled_at = ?, suspended_at = ?,
			version = ?, state_reason = ?, updated_at = NOW()
		WHERE id = ? AND tenant_id = ? AND version = ? AND deleted_at IS NULL
	`, next.Status, boolToInt(next.CancelAtPeriodEnd), saasSubscriptionNullableTime(next.CanceledAt),
		saasSubscriptionNullableTime(next.SuspendedAt), next.Version, next.StateReason,
		next.ID, next.TenantID, current.Version); err != nil {
		return 0, 0, err
	}
	eventID, err := insertSaaSSubscriptionEventTx(ctx, tx, dashboard.SaaSAdminSubscriptionEvent{
		SubscriptionID: next.ID, TenantID: next.TenantID, EventType: "refund",
		FromStatus: current.Status, ToStatus: next.Status,
		EffectiveAt: saasPaymentFirstNonEmpty(effectiveAt, now.Format("2006-01-02 15:04:05")),
		ActorUserID: refund.RequestedByUserID, ActorTenantID: refund.RequestedByTenantID,
		Source: "payment_refund", IdempotencyKey: "refund:" + refund.RefundNo + ":entitlement",
		Reason: reason, PayloadJSON: saasAdminMarshalJSON(map[string]any{"before": saasSubscriptionStatePayload(current), "after": saasSubscriptionStatePayload(next), "refundNo": refund.RefundNo}),
	})
	if err != nil {
		return 0, 0, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: next.TenantID, ActorUserID: refund.RequestedByUserID, ActorTenantID: refund.RequestedByTenantID,
		Action: dashboard.SaaSAdminOperationActionSubscriptionTransition, TargetType: dashboard.SaaSAdminOperationTargetSubscription,
		TargetID: fmt.Sprintf("%d", next.ID), TargetName: next.TenantName,
		BeforeJSON: saasAdminMarshalJSON(saasSubscriptionStatePayload(current)),
		AfterJSON:  saasAdminMarshalJSON(saasSubscriptionStatePayload(next)), Remark: reason,
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return 0, 0, err
	}
	return eventID, operationID, nil
}

func releaseSaaSPaymentRefundReservationTx(ctx context.Context, tx *sql.Tx, orderID int64, amountCents int64, refundID int64) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_orders
		SET refund_pending_amount_cents = CASE
				WHEN refund_pending_amount_cents >= ? THEN refund_pending_amount_cents - ? ELSE 0 END,
			latest_refund_id = ?, version = version + 1, updated_at = NOW()
		WHERE id = ? AND refund_pending_amount_cents >= ? AND deleted_at IS NULL
	`, amountCents, amountCents, refundID, orderID, amountCents)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return saasPaymentConflict("payment refund reservation changed concurrently")
	}
	return nil
}

func finalizeSaaSRefundWebhookEventTx(ctx context.Context, tx *sql.Tx, eventID int64, order dashboard.SaaSAdminPaymentOrder, refund dashboard.SaaSAdminPaymentRefund, input dashboard.SaaSPaymentWebhookInput, status string, lastError string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_webhook_events
		SET order_id = ?, refund_id = ?, order_no = ?, refund_no = ?,
			provider_order_no = ?, provider_refund_no = ?, status = ?, processed_at = NOW(),
			last_error = ?, updated_at = NOW()
		WHERE id = ?
	`, order.ID, refund.ID, order.OrderNo, refund.RefundNo,
		saasPaymentFirstNonEmpty(input.ProviderOrderNo, order.ProviderOrderNo),
		saasPaymentFirstNonEmpty(input.ProviderRefundNo, refund.ProviderRefundNo),
		status, truncateRunes(lastError, 255), eventID)
	return err
}

func (s *MySQLStore) failSaaSPaymentRefundWebhookTx(ctx context.Context, tx *sql.Tx, eventID int64, refund dashboard.SaaSAdminPaymentRefund, input dashboard.SaaSPaymentWebhookInput, message string, returnErr error) (dashboard.SaaSPaymentWebhookResult, error) {
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_webhook_events
		SET order_id = ?, refund_id = ?, order_no = ?, refund_no = ?,
			provider_order_no = COALESCE(NULLIF(?, ''), provider_order_no),
			provider_refund_no = COALESCE(NULLIF(?, ''), provider_refund_no),
			status = 'failed', processed_at = NOW(), last_error = ?, updated_at = NOW()
		WHERE id = ?
	`, refund.PaymentOrderID, refund.ID, refund.OrderNo, refund.RefundNo,
		strings.TrimSpace(input.ProviderOrderNo), saasPaymentFirstNonEmpty(input.ProviderRefundNo, refund.ProviderRefundNo),
		truncateRunes(message, 255), eventID); err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	return dashboard.SaaSPaymentWebhookResult{}, returnErr
}

func saasPaymentRefundByNoTx(ctx context.Context, tx *sql.Tx, refundNo string, forUpdate bool) (dashboard.SaaSAdminPaymentRefund, bool, error) {
	query := saasAdminPaymentRefundSelectSQL() + ` WHERE r.refund_no = ? AND r.deleted_at IS NULL LIMIT 1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	item, err := scanSaaSAdminPaymentRefund(tx.QueryRowContext(ctx, query, refundNo))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminPaymentRefund{}, false, nil
	}
	return item, err == nil, err
}

func saasPaymentRefundByIDTx(ctx context.Context, tx *sql.Tx, refundID int64) (dashboard.SaaSAdminPaymentRefund, bool, error) {
	item, err := scanSaaSAdminPaymentRefund(tx.QueryRowContext(ctx, saasAdminPaymentRefundSelectSQL()+` WHERE r.id = ? AND r.deleted_at IS NULL LIMIT 1`, refundID))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminPaymentRefund{}, false, nil
	}
	return item, err == nil, err
}

func saasPaymentRefundByIdempotencyTx(ctx context.Context, tx *sql.Tx, orderID int64, key string, forUpdate bool) (dashboard.SaaSAdminPaymentRefund, bool, error) {
	query := saasAdminPaymentRefundSelectSQL() + ` WHERE r.payment_order_id = ? AND r.idempotency_key = ? AND r.deleted_at IS NULL LIMIT 1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	item, err := scanSaaSAdminPaymentRefund(tx.QueryRowContext(ctx, query, orderID, key))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminPaymentRefund{}, false, nil
	}
	return item, err == nil, err
}

func saasAdminPaymentRefundSelectSQL() string {
	return `
		SELECT r.id, r.refund_no, r.payment_order_id, r.order_no, r.tenant_id, COALESCE(t.name, ''),
			r.provider, COALESCE(r.provider_refund_no, ''), COALESCE(r.idempotency_key, ''), r.status,
			r.amount_cents, r.currency, r.reason, r.entitlement_action,
			r.latest_webhook_event_id, r.billing_event_id, r.subscription_event_id, r.subscription_operation_id,
			r.requested_by_user_id, r.requested_by_tenant_id,
			COALESCE(DATE_FORMAT(r.requested_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(r.succeeded_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(r.failed_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(r.canceled_at, '%Y-%m-%d %H:%i:%s'), ''),
			r.failure_code, r.failure_message, r.version, r.remark,
			COALESCE(CAST(r.metadata_json AS CHAR), ''),
			COALESCE(DATE_FORMAT(r.created_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(r.updated_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(o.amount_cents, 0), COALESCE(o.refunded_amount_cents, 0),
			COALESCE(o.refund_pending_amount_cents, 0),
			GREATEST(CAST(COALESCE(o.amount_cents, 0) AS SIGNED) - CAST(COALESCE(o.refunded_amount_cents, 0) AS SIGNED) - CAST(COALESCE(o.refund_pending_amount_cents, 0) AS SIGNED), 0),
			CASE
				WHEN COALESCE(o.refunded_amount_cents, 0) >= COALESCE(o.amount_cents, 0) AND COALESCE(o.amount_cents, 0) > 0 THEN 'full'
				WHEN COALESCE(o.refunded_amount_cents, 0) > 0 THEN 'partial'
				WHEN COALESCE(o.refund_pending_amount_cents, 0) > 0 THEN 'pending'
				ELSE 'none'
			END,
			COALESCE(o.status, '')
		FROM mochat_go_saas_payment_refunds r
		LEFT JOIN mochat_go_saas_payment_orders o ON o.id = r.payment_order_id AND o.deleted_at IS NULL
		LEFT JOIN mc_tenant t ON t.id = r.tenant_id AND t.deleted_at IS NULL
	`
}

func scanSaaSAdminPaymentRefund(scanner saasPaymentScanner) (dashboard.SaaSAdminPaymentRefund, error) {
	var item dashboard.SaaSAdminPaymentRefund
	err := scanner.Scan(
		&item.ID, &item.RefundNo, &item.PaymentOrderID, &item.OrderNo, &item.TenantID, &item.TenantName,
		&item.Provider, &item.ProviderRefundNo, &item.IdempotencyKey, &item.Status,
		&item.AmountCents, &item.Currency, &item.Reason, &item.EntitlementAction,
		&item.LatestWebhookEventID, &item.BillingEventID, &item.SubscriptionEventID, &item.SubscriptionOperation,
		&item.RequestedByUserID, &item.RequestedByTenantID, &item.RequestedAt,
		&item.SucceededAt, &item.FailedAt, &item.CanceledAt, &item.FailureCode, &item.FailureMessage,
		&item.Version, &item.Remark, &item.MetadataJSON, &item.CreatedAt, &item.UpdatedAt,
		&item.OrderAmountCents, &item.OrderRefundedCents, &item.OrderRefundPending,
		&item.OrderRefundableCents, &item.OrderRefundStatus, &item.OrderPaymentStatus,
	)
	return item, err
}

func summarizeSaaSPaymentRefunds(items []dashboard.SaaSAdminPaymentRefund) dashboard.SaaSAdminPaymentRefundSummary {
	var summary dashboard.SaaSAdminPaymentRefundSummary
	tenants := map[int]struct{}{}
	orders := map[int64]struct{}{}
	providers := map[string]struct{}{}
	for _, item := range items {
		summary.RefundCount++
		summary.RefundAmountCents += item.AmountCents
		if item.TenantID > 0 {
			tenants[item.TenantID] = struct{}{}
		}
		if item.PaymentOrderID > 0 {
			orders[item.PaymentOrderID] = struct{}{}
		}
		if item.Provider != "" {
			providers[item.Provider] = struct{}{}
		}
		switch item.Status {
		case dashboard.SaaSPaymentRefundStatusRequested:
			summary.RequestedCount++
			summary.PendingAmountCents += item.AmountCents
		case dashboard.SaaSPaymentRefundStatusProcessing:
			summary.ProcessingCount++
			summary.PendingAmountCents += item.AmountCents
		case dashboard.SaaSPaymentRefundStatusSucceeded:
			summary.SucceededCount++
			summary.SucceededAmountCents += item.AmountCents
		case dashboard.SaaSPaymentRefundStatusFailed:
			summary.FailedCount++
			summary.FailedAmountCents += item.AmountCents
		case dashboard.SaaSPaymentRefundStatusCanceled:
			summary.CanceledCount++
		}
	}
	summary.TenantCount = len(tenants)
	summary.OrderCount = len(orders)
	summary.ProviderCount = len(providers)
	return summary
}

func saasPaymentRefundMatchesCreate(item dashboard.SaaSAdminPaymentRefund, create dashboard.SaaSAdminPaymentRefundCreate) bool {
	providerMatches := strings.TrimSpace(create.ProviderRefundNo) == "" || item.ProviderRefundNo == strings.TrimSpace(create.ProviderRefundNo)
	return item.OrderNo == strings.TrimSpace(create.OrderNo) && item.AmountCents == create.AmountCents &&
		item.Currency == strings.TrimSpace(create.Currency) && item.Reason == strings.TrimSpace(create.Reason) &&
		item.EntitlementAction == strings.TrimSpace(create.EntitlementAction) && providerMatches
}

func saasPaymentRefundStatePayload(item dashboard.SaaSAdminPaymentRefund) map[string]any {
	return map[string]any{
		"refundNo": item.RefundNo, "orderNo": item.OrderNo, "tenantId": item.TenantID,
		"provider": item.Provider, "providerRefundNo": item.ProviderRefundNo,
		"status": item.Status, "amountCents": item.AmountCents, "currency": item.Currency,
		"reason": item.Reason, "entitlementAction": item.EntitlementAction,
		"latestWebhookEventId": item.LatestWebhookEventID, "billingEventId": item.BillingEventID,
		"subscriptionEventId": item.SubscriptionEventID, "subscriptionOperationId": item.SubscriptionOperation,
		"failureCode": item.FailureCode, "failureMessage": item.FailureMessage, "version": item.Version,
	}
}

func saasPaymentWebhookBillingEventID(order dashboard.SaaSAdminPaymentOrder, refund dashboard.SaaSAdminPaymentRefund) int64 {
	if refund.BillingEventID > 0 {
		return refund.BillingEventID
	}
	return order.BillingEventID
}

func saasPaymentFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
