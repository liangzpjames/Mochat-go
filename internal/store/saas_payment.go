package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

const saasPaymentQueryMaxLimit = 5000

type saasPaymentScanner interface {
	Scan(dest ...any) error
}

func (s *MySQLStore) SaaSAdminPaymentOrders(ctx context.Context, options dashboard.SaaSAdminPaymentOrderOptions) (dashboard.SaaSAdminPaymentOrderReport, error) {
	if options.Limit <= 0 {
		options.Limit = 100
	}
	if options.Limit > saasPaymentQueryMaxLimit {
		options.Limit = saasPaymentQueryMaxLimit
	}
	where := []string{"o.deleted_at IS NULL"}
	args := []any{}
	if options.TenantID > 0 {
		where = append(where, "o.tenant_id = ?")
		args = append(args, options.TenantID)
	}
	if options.Status != "" && options.Status != dashboard.SaaSPaymentOrderStatusAll {
		where = append(where, "o.status = ?")
		args = append(args, options.Status)
	}
	if strings.TrimSpace(options.Provider) != "" {
		where = append(where, "o.provider = ?")
		args = append(args, strings.TrimSpace(options.Provider))
	}
	if strings.TrimSpace(options.PackageCode) != "" {
		where = append(where, "o.package_code = ?")
		args = append(args, strings.TrimSpace(options.PackageCode))
	}
	if strings.TrimSpace(options.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(options.Keyword) + "%"
		where = append(where, "(o.order_no LIKE ? OR COALESCE(o.provider_order_no, '') LIKE ? OR CAST(o.tenant_id AS CHAR) LIKE ? OR COALESCE(t.name, '') LIKE ? OR o.failure_code LIKE ? OR o.failure_message LIKE ? OR o.remark LIKE ?)")
		args = append(args, keyword, keyword, keyword, keyword, keyword, keyword, keyword)
	}
	args = append(args, saasPaymentQueryMaxLimit)
	rows, err := s.db.QueryContext(ctx, saasAdminPaymentOrderSelectSQL()+`
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY
			CASE o.status WHEN 'failed' THEN 1 WHEN 'processing' THEN 2 WHEN 'pending' THEN 3 WHEN 'paid' THEN 4 WHEN 'canceled' THEN 5 ELSE 6 END,
			COALESCE(o.next_dunning_at, o.checkout_expires_at, o.created_at) ASC,
			o.id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return dashboard.SaaSAdminPaymentOrderReport{}, err
	}
	defer rows.Close()
	all := make([]dashboard.SaaSAdminPaymentOrder, 0)
	for rows.Next() {
		order, err := scanSaaSAdminPaymentOrder(rows)
		if err != nil {
			return dashboard.SaaSAdminPaymentOrderReport{}, err
		}
		all = append(all, order)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSAdminPaymentOrderReport{}, err
	}
	report := dashboard.SaaSAdminPaymentOrderReport{Options: options, Summary: summarizeSaaSPaymentOrders(all)}
	if len(all) > options.Limit {
		all = all[:options.Limit]
	}
	report.Orders = all
	return report, nil
}

func (s *MySQLStore) SaaSAdminPaymentWebhookEvents(ctx context.Context, options dashboard.SaaSAdminPaymentWebhookEventOptions) ([]dashboard.SaaSAdminPaymentWebhookEvent, error) {
	if options.Limit <= 0 {
		options.Limit = 50
	}
	if options.Limit > saasPaymentQueryMaxLimit {
		options.Limit = saasPaymentQueryMaxLimit
	}
	where := []string{"1 = 1"}
	args := []any{}
	if options.TenantID > 0 {
		where = append(where, "o.tenant_id = ?")
		args = append(args, options.TenantID)
	}
	if strings.TrimSpace(options.Provider) != "" {
		where = append(where, "e.provider = ?")
		args = append(args, strings.TrimSpace(options.Provider))
	}
	if options.Status != "" && options.Status != dashboard.SaaSPaymentWebhookEventStatusAll {
		where = append(where, "e.status = ?")
		args = append(args, options.Status)
	}
	if strings.TrimSpace(options.EventType) != "" {
		where = append(where, "e.event_type = ?")
		args = append(args, strings.TrimSpace(options.EventType))
	}
	if strings.TrimSpace(options.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(options.Keyword) + "%"
		where = append(where, "(e.event_id LIKE ? OR e.order_no LIKE ? OR e.provider_order_no LIKE ? OR e.refund_no LIKE ? OR e.provider_refund_no LIKE ? OR e.last_error LIKE ? OR CAST(e.payload_json AS CHAR) LIKE ?)")
		args = append(args, keyword, keyword, keyword, keyword, keyword, keyword, keyword)
	}
	args = append(args, options.Limit)
	rows, err := s.db.QueryContext(ctx, saasAdminPaymentWebhookEventSelectSQL()+`
		LEFT JOIN mochat_go_saas_payment_orders o ON o.id = e.order_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY e.id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]dashboard.SaaSAdminPaymentWebhookEvent, 0)
	for rows.Next() {
		event, err := scanSaaSAdminPaymentWebhookEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *MySQLStore) CreateSaaSAdminPaymentOrder(ctx context.Context, create dashboard.SaaSAdminPaymentOrderCreate) (dashboard.SaaSAdminPaymentOrderCreateResult, error) {
	if create.TenantID <= 0 || strings.TrimSpace(create.OrderNo) == "" || strings.TrimSpace(create.PackageCode) == "" {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, dashboard.NewSaaSAdminBadRequest("tenantId, orderNo and packageCode are required")
	}
	if create.AmountCents <= 0 {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, dashboard.NewSaaSAdminBadRequest("amountCents must be positive")
	}
	approvedExecution := create.ApprovalExecutionID > 0
	if approvedExecution != (create.ApprovalExecutionVersion > 0) {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, dashboard.NewSaaSAdminBadRequest("审批执行引用无效")
	}
	if approvedExecution && (create.ExpectedTenantStatus <= 0 || create.ExpectedPackageVersion <= 0) {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, dashboard.NewSaaSAdminBadRequest("审批冻结快照不完整")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
	}
	defer rollbackQuietly(tx)

	var tenantName string
	var tenantStatus int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(name, ''), COALESCE(status, 0) FROM mc_tenant WHERE id = ? AND deleted_at IS NULL LIMIT 1 FOR UPDATE`, create.TenantID).Scan(&tenantName, &tenantStatus); errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, dashboard.NewSaaSAdminNotFound("tenant not found")
	} else if err != nil {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
	}
	if approvedExecution && tenantStatus != create.ExpectedTenantStatus {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, saasPaymentConflict("tenant status changed after approval request")
	}
	if tenantStatus != 1 {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, saasPaymentConflict("only active tenant can receive payment order")
	}
	pkg, err := saasAdminPackageByCodeTx(ctx, tx, strings.TrimSpace(create.PackageCode))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, dashboard.NewSaaSAdminNotFound("package not found")
	}
	if err != nil {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
	}
	if pkg.Status != 1 {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, dashboard.NewSaaSAdminBadRequest("package disabled")
	}
	if approvedExecution && pkg.Version != create.ExpectedPackageVersion {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, saasPaymentConflict(fmt.Sprintf("package version changed after approval request: current=%d", pkg.Version))
	}
	packageLimitsJSON, err := json.Marshal(pkg.Limits)
	if err != nil {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
	}

	if strings.TrimSpace(create.IdempotencyKey) != "" {
		existing, found, err := saasPaymentOrderByIdempotencyTx(ctx, tx, create.TenantID, strings.TrimSpace(create.IdempotencyKey), true)
		if err != nil {
			return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
		}
		if found {
			if !saasPaymentOrderMatchesCreate(existing, create) {
				return dashboard.SaaSAdminPaymentOrderCreateResult{}, saasPaymentConflict("payment order idempotency key already used with different request")
			}
			if err := markSaaSAdminApprovalEffectTx(ctx, tx, create.ApprovalExecutionID, create.ApprovalExecutionVersion, create.ActorUserID, 0); err != nil {
				return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
			}
			if err := tx.Commit(); err != nil {
				return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
			}
			return dashboard.SaaSAdminPaymentOrderCreateResult{Order: existing, Idempotent: true}, nil
		}
	}

	providerOrderValue := nullableTrimmedString(create.ProviderOrderNo)
	idempotencyValue := nullableTrimmedString(create.IdempotencyKey)
	serviceExpiresValue := nullableTrimmedString(create.ServiceExpiresAt)
	checkoutExpiresValue := nullableTrimmedString(create.CheckoutExpiresAt)
	metadataValue := saasAdminJSONValue(strings.TrimSpace(create.MetadataJSON))
	result, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO mochat_go_saas_payment_orders
			(order_no, tenant_id, provider, provider_order_no, idempotency_key, status,
			 package_code, package_name, package_version, package_limits_json, billing_cycle, service_expires_at,
			 amount_cents, currency, checkout_url, checkout_expires_at,
			 attempt_count, dunning_attempts, max_dunning_attempts,
			 latest_webhook_event_id, billing_event_id, version,
			 created_by_user_id, created_by_tenant_id, remark, metadata_json,
			 created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, 'pending', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?, 0, 0, 1, ?, ?, ?, ?, NOW(), NOW(), NULL)
	`, strings.TrimSpace(create.OrderNo), create.TenantID, strings.TrimSpace(create.Provider), providerOrderValue, idempotencyValue,
		pkg.Code, pkg.Name, pkg.Version, string(packageLimitsJSON), strings.TrimSpace(create.BillingCycle), serviceExpiresValue,
		create.AmountCents, strings.TrimSpace(create.Currency), strings.TrimSpace(create.CheckoutURL), checkoutExpiresValue,
		create.MaxDunningAttempts, create.ActorUserID, create.ActorTenantID, truncateRunes(strings.TrimSpace(create.Remark), 255), metadataValue)
	if err != nil {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		existing, found, err := saasPaymentOrderByNoTx(ctx, tx, strings.TrimSpace(create.OrderNo), true)
		if err != nil {
			return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
		}
		if !found && strings.TrimSpace(create.IdempotencyKey) != "" {
			existing, found, err = saasPaymentOrderByIdempotencyTx(ctx, tx, create.TenantID, strings.TrimSpace(create.IdempotencyKey), true)
			if err != nil {
				return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
			}
		}
		if !found || !saasPaymentOrderMatchesCreate(existing, create) {
			return dashboard.SaaSAdminPaymentOrderCreateResult{}, saasPaymentConflict("payment order number or provider order number already exists")
		}
		if err := markSaaSAdminApprovalEffectTx(ctx, tx, create.ApprovalExecutionID, create.ApprovalExecutionVersion, create.ActorUserID, 0); err != nil {
			return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
		}
		return dashboard.SaaSAdminPaymentOrderCreateResult{Order: existing, Idempotent: true}, nil
	}

	order, found, err := saasPaymentOrderByNoTx(ctx, tx, strings.TrimSpace(create.OrderNo), true)
	if err != nil {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
	}
	if !found {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, errors.New("payment order insert did not return a row")
	}
	order.TenantName = tenantName
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: create.TenantID, ActorUserID: create.ActorUserID, ActorTenantID: create.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionPaymentOrderCreate, TargetType: dashboard.SaaSAdminOperationTargetPaymentOrder,
		TargetID: order.OrderNo, TargetName: tenantName,
		AfterJSON: saasAdminMarshalJSON(saasPaymentOrderStatePayload(order)), Remark: strings.TrimSpace(create.Remark),
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, create.ApprovalExecutionID, create.ApprovalExecutionVersion, create.ActorUserID, operationID); err != nil {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminPaymentOrderCreateResult{}, err
	}
	return dashboard.SaaSAdminPaymentOrderCreateResult{Order: order, OperationID: operationID}, nil
}

func (s *MySQLStore) CancelSaaSAdminPaymentOrder(ctx context.Context, cancel dashboard.SaaSAdminPaymentOrderCancel) (dashboard.SaaSAdminPaymentOrderCancelResult, error) {
	if strings.TrimSpace(cancel.OrderNo) == "" || cancel.ExpectedVersion <= 0 {
		return dashboard.SaaSAdminPaymentOrderCancelResult{}, dashboard.NewSaaSAdminBadRequest("orderNo and expectedVersion are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminPaymentOrderCancelResult{}, err
	}
	defer rollbackQuietly(tx)
	current, found, err := saasPaymentOrderByNoTx(ctx, tx, strings.TrimSpace(cancel.OrderNo), true)
	if err != nil {
		return dashboard.SaaSAdminPaymentOrderCancelResult{}, err
	}
	if !found {
		return dashboard.SaaSAdminPaymentOrderCancelResult{}, dashboard.NewSaaSAdminNotFound("payment order not found")
	}
	if current.Version != cancel.ExpectedVersion {
		return dashboard.SaaSAdminPaymentOrderCancelResult{}, saasPaymentConflict(fmt.Sprintf("payment order version conflict: current=%d", current.Version))
	}
	if current.Status == dashboard.SaaSPaymentOrderStatusPaid {
		return dashboard.SaaSAdminPaymentOrderCancelResult{}, saasPaymentConflict("paid payment order cannot be canceled")
	}
	if current.Status == dashboard.SaaSPaymentOrderStatusCanceled {
		return dashboard.SaaSAdminPaymentOrderCancelResult{}, saasPaymentConflict("payment order already canceled")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_orders
		SET status = 'canceled', canceled_at = NOW(), next_dunning_at = NULL,
			failure_code = '', failure_message = ?, version = version + 1, updated_at = NOW()
		WHERE id = ? AND version = ? AND deleted_at IS NULL
	`, truncateRunes(strings.TrimSpace(cancel.Reason), 255), current.ID, current.Version); err != nil {
		return dashboard.SaaSAdminPaymentOrderCancelResult{}, err
	}
	next, _, err := saasPaymentOrderByNoTx(ctx, tx, current.OrderNo, true)
	if err != nil {
		return dashboard.SaaSAdminPaymentOrderCancelResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: current.TenantID, ActorUserID: cancel.ActorUserID, ActorTenantID: cancel.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionPaymentOrderCancel, TargetType: dashboard.SaaSAdminOperationTargetPaymentOrder,
		TargetID: current.OrderNo, TargetName: current.TenantName,
		BeforeJSON: saasAdminMarshalJSON(saasPaymentOrderStatePayload(current)),
		AfterJSON:  saasAdminMarshalJSON(saasPaymentOrderStatePayload(next)), Remark: strings.TrimSpace(cancel.Reason),
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminPaymentOrderCancelResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminPaymentOrderCancelResult{}, err
	}
	return dashboard.SaaSAdminPaymentOrderCancelResult{Order: next, PreviousStatus: current.Status, OperationID: operationID}, nil
}

func (s *MySQLStore) ProcessSaaSPaymentWebhook(ctx context.Context, input dashboard.SaaSPaymentWebhookInput) (dashboard.SaaSPaymentWebhookResult, error) {
	if !dashboard.SaaSPaymentWebhookTypeValid(input.EventType) || strings.TrimSpace(input.Provider) == "" || strings.TrimSpace(input.EventID) == "" || strings.TrimSpace(input.OrderNo) == "" {
		return dashboard.SaaSPaymentWebhookResult{}, dashboard.NewSaaSAdminBadRequest("payment webhook fields invalid")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	defer rollbackQuietly(tx)
	event, inserted, err := ensureSaaSPaymentWebhookEventTx(ctx, tx, input)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if !inserted && event.PayloadSHA256 != input.PayloadSHA256 {
		return dashboard.SaaSPaymentWebhookResult{}, saasPaymentConflict("payment webhook event payload digest mismatch")
	}
	if event.Status == dashboard.SaaSPaymentWebhookEventStatusProcessed || event.Status == dashboard.SaaSPaymentWebhookEventStatusIgnored {
		order, _, err := saasPaymentOrderByNoTx(ctx, tx, event.OrderNo, false)
		if err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
		var refund dashboard.SaaSAdminPaymentRefund
		if event.RefundID > 0 {
			refund, _, err = saasPaymentRefundByIDTx(ctx, tx, event.RefundID)
			if err != nil {
				return dashboard.SaaSPaymentWebhookResult{}, err
			}
		}
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
		return dashboard.SaaSPaymentWebhookResult{
			Event: event, Order: order, Refund: refund, Duplicate: true,
			Ignored:        event.Status == dashboard.SaaSPaymentWebhookEventStatusIgnored,
			BillingEventID: saasPaymentWebhookBillingEventID(order, refund),
		}, nil
	}
	if !inserted {
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_payment_webhook_events SET attempts = attempts + 1, status = 'received', last_error = '', updated_at = NOW() WHERE id = ?`, event.ID); err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
	}
	if dashboard.SaaSPaymentWebhookRefundType(input.EventType) {
		result, err := s.processSaaSPaymentRefundWebhookTx(ctx, tx, event, input)
		if err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
		return result, nil
	}
	order, found, err := saasPaymentOrderByNoTx(ctx, tx, strings.TrimSpace(input.OrderNo), true)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if !found {
		return s.failSaaSPaymentWebhookTx(ctx, tx, event.ID, "payment order not found", dashboard.NewSaaSAdminNotFound("payment order not found"))
	}
	if order.Provider != strings.TrimSpace(input.Provider) {
		return s.failSaaSPaymentWebhookTx(ctx, tx, event.ID, "payment provider mismatch", saasPaymentUnprocessable("payment provider mismatch"))
	}
	if order.ProviderOrderNo != "" && input.ProviderOrderNo != "" && order.ProviderOrderNo != input.ProviderOrderNo {
		return s.failSaaSPaymentWebhookTx(ctx, tx, event.ID, "provider order number mismatch", saasPaymentUnprocessable("provider order number mismatch"))
	}
	if order.Status == dashboard.SaaSPaymentOrderStatusPaid {
		if err := finalizeSaaSPaymentWebhookEventTx(ctx, tx, event.ID, order.ID, order.OrderNo, input.ProviderOrderNo, dashboard.SaaSPaymentWebhookEventStatusIgnored, "payment order already paid"); err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
		updatedEvent, err := saasPaymentWebhookEventByIDTx(ctx, tx, event.ID)
		if err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
		return dashboard.SaaSPaymentWebhookResult{Event: updatedEvent, Order: order, Ignored: true, BillingEventID: order.BillingEventID}, nil
	}

	var result dashboard.SaaSPaymentWebhookResult
	switch input.EventType {
	case dashboard.SaaSPaymentWebhookTypeSucceeded:
		result, err = s.settleSaaSPaymentOrderTx(ctx, tx, order, event, input)
	case dashboard.SaaSPaymentWebhookTypeFailed:
		result, err = s.updateSaaSPaymentOrderFromWebhookTx(ctx, tx, order, event, input, dashboard.SaaSPaymentOrderStatusFailed)
	case dashboard.SaaSPaymentWebhookTypeCanceled:
		result, err = s.updateSaaSPaymentOrderFromWebhookTx(ctx, tx, order, event, input, dashboard.SaaSPaymentOrderStatusCanceled)
	case dashboard.SaaSPaymentWebhookTypeProcessing:
		result, err = s.updateSaaSPaymentOrderFromWebhookTx(ctx, tx, order, event, input, dashboard.SaaSPaymentOrderStatusProcessing)
	}
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if input.EventType == dashboard.SaaSPaymentWebhookTypeSucceeded {
		refresh, refreshErr := s.RefreshSaaSUsageCounters(ctx, result.Order.TenantID)
		if refreshErr != nil {
			result.Warning = "payment settled but SaaS usage refresh failed: " + refreshErr.Error()
		} else {
			result.MetricsRefreshed = refresh.MetricsRefreshed
		}
	}
	return result, nil
}

func (s *MySQLStore) settleSaaSPaymentOrderTx(ctx context.Context, tx *sql.Tx, order dashboard.SaaSAdminPaymentOrder, event dashboard.SaaSAdminPaymentWebhookEvent, input dashboard.SaaSPaymentWebhookInput) (dashboard.SaaSPaymentWebhookResult, error) {
	if input.AmountCents != order.AmountCents || strings.TrimSpace(input.Currency) != order.Currency {
		return s.failSaaSPaymentWebhookTx(ctx, tx, event.ID, "payment amount or currency mismatch", saasPaymentUnprocessable("payment amount or currency mismatch"))
	}
	pkg := dashboard.SaaSAdminPackage{
		Code: order.PackageCode, Name: order.PackageName, Status: 1, Version: order.PackageVersion, Limits: order.PackageLimits,
	}
	var err error
	if order.PackageVersion <= 0 {
		pkg, err = saasAdminPackageByCodeTx(ctx, tx, order.PackageCode)
		if errors.Is(err, sql.ErrNoRows) {
			return s.failSaaSPaymentWebhookTx(ctx, tx, event.ID, "package not found", dashboard.NewSaaSAdminNotFound("package not found"))
		}
		if err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
		if pkg.Status != 1 {
			return s.failSaaSPaymentWebhookTx(ctx, tx, event.ID, "package disabled", saasPaymentUnprocessable("package disabled"))
		}
	}
	var tenantName string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(name, '') FROM mc_tenant WHERE id = ? AND deleted_at IS NULL LIMIT 1 FOR UPDATE`, order.TenantID).Scan(&tenantName); errors.Is(err, sql.ErrNoRows) {
		return s.failSaaSPaymentWebhookTx(ctx, tx, event.ID, "tenant not found", dashboard.NewSaaSAdminNotFound("tenant not found"))
	} else if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}

	var previousPackageCode string
	var previousPackageName string
	var previousExpiresAt string
	var previousStatus int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(package_code, ''), COALESCE(package_name, ''),
			COALESCE(DATE_FORMAT(expires_at, '%Y-%m-%d %H:%i:%s'), ''), COALESCE(status, 0)
		FROM mochat_go_saas_tenant_packages
		WHERE tenant_id = ? AND deleted_at IS NULL
		LIMIT 1 FOR UPDATE
	`, order.TenantID).Scan(&previousPackageCode, &previousPackageName, &previousExpiresAt, &previousStatus)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	newExpiresAt := strings.TrimSpace(order.ServiceExpiresAt)
	if order.BillingCycle != "lifetime" && newExpiresAt == "" {
		return s.failSaaSPaymentWebhookTx(ctx, tx, event.ID, "service expiration missing", saasPaymentUnprocessable("service expiration missing"))
	}
	if order.BillingCycle != "lifetime" && previousExpiresAt != "" && saasPaymentTimeAfter(previousExpiresAt, newExpiresAt) {
		newExpiresAt = previousExpiresAt
	}
	limitsJSON, err := json.Marshal(pkg.Limits)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_tenant_packages
			(tenant_id, package_code, package_name, starts_at, expires_at, status, limits_json, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, NOW(), ?, 1, ?, NOW(), NOW(), NULL)
		ON DUPLICATE KEY UPDATE package_code = VALUES(package_code), package_name = VALUES(package_name),
			expires_at = VALUES(expires_at), status = 1, limits_json = VALUES(limits_json), updated_at = NOW(), deleted_at = NULL
	`, order.TenantID, pkg.Code, pkg.Name, nullableTrimmedString(newExpiresAt), string(limitsJSON)); err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	paidAt := strings.TrimSpace(input.PaidAt)
	if paidAt == "" {
		paidAt = strings.TrimSpace(input.OccurredAt)
	}
	metadata := map[string]any{
		"source": "payment_webhook", "paymentOrderNo": order.OrderNo, "provider": order.Provider,
		"providerOrderNo": input.ProviderOrderNo, "webhookEventId": input.EventID,
		"previousPackageCode": previousPackageCode, "previousPackageName": previousPackageName, "previousStatus": previousStatus,
	}
	if strings.TrimSpace(input.MetadataJSON) != "" {
		metadata["providerMetadata"] = json.RawMessage(input.MetadataJSON)
	}
	billingResult, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_billing_events
			(tenant_id, event_type, package_code, package_name, previous_expires_at, new_expires_at,
			 amount_cents, currency, paid_at, payment_method, external_order_no,
			 actor_user_id, actor_tenant_id, remark, metadata_json, created_at, updated_at, deleted_at)
		VALUES (?, 'renewal', ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?, NOW(), NOW(), NULL)
	`, order.TenantID, pkg.Code, pkg.Name, nullableTrimmedString(previousExpiresAt), nullableTrimmedString(newExpiresAt),
		order.AmountCents, order.Currency, nullableTrimmedString(paidAt), order.Provider, order.OrderNo,
		truncateRunes("payment webhook "+input.EventID, 255), saasAdminJSONValue(saasAdminMarshalJSON(metadata)))
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	billingEventID, _ := billingResult.LastInsertId()
	if err := syncSaaSSubscriptionPackageTx(
		ctx, tx, order.TenantID, pkg.Code, pkg.Name, newExpiresAt, order.BillingCycle, billingEventID,
		"payment_succeeded", "payment", "payment:"+order.Provider+":"+input.EventID, true,
		0, 0, "payment succeeded: "+order.OrderNo,
	); err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_orders
		SET status = 'paid', provider_order_no = COALESCE(NULLIF(?, ''), provider_order_no),
			attempt_count = attempt_count + 1, latest_webhook_event_id = ?, billing_event_id = ?, paid_at = ?,
			failed_at = NULL, canceled_at = NULL, failure_code = '', failure_message = '', next_dunning_at = NULL,
			version = version + 1, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, strings.TrimSpace(input.ProviderOrderNo), event.ID, billingEventID, nullableTrimmedString(paidAt), order.ID); err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if err := finalizeSaaSPaymentWebhookEventTx(ctx, tx, event.ID, order.ID, order.OrderNo, input.ProviderOrderNo, dashboard.SaaSPaymentWebhookEventStatusProcessed, ""); err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	next, _, err := saasPaymentOrderByNoTx(ctx, tx, order.OrderNo, true)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	next.TenantName = tenantName
	if _, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: order.TenantID, Action: dashboard.SaaSAdminOperationActionPaymentOrderPaid,
		TargetType: dashboard.SaaSAdminOperationTargetPaymentOrder, TargetID: order.OrderNo, TargetName: tenantName,
		BeforeJSON: saasAdminMarshalJSON(saasPaymentOrderStatePayload(order)),
		AfterJSON:  saasAdminMarshalJSON(map[string]any{"order": saasPaymentOrderStatePayload(next), "billingEventId": billingEventID, "expiresAt": newExpiresAt, "webhookEventId": input.EventID}),
		Remark:     "payment webhook " + input.EventID,
	}); err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	updatedEvent, err := saasPaymentWebhookEventByIDTx(ctx, tx, event.ID)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	return dashboard.SaaSPaymentWebhookResult{Event: updatedEvent, Order: next, BillingEventID: billingEventID}, nil
}

func (s *MySQLStore) updateSaaSPaymentOrderFromWebhookTx(ctx context.Context, tx *sql.Tx, order dashboard.SaaSAdminPaymentOrder, event dashboard.SaaSAdminPaymentWebhookEvent, input dashboard.SaaSPaymentWebhookInput, status string) (dashboard.SaaSPaymentWebhookResult, error) {
	if status == dashboard.SaaSPaymentOrderStatusCanceled && order.Status == dashboard.SaaSPaymentOrderStatusCanceled {
		if err := finalizeSaaSPaymentWebhookEventTx(ctx, tx, event.ID, order.ID, order.OrderNo, input.ProviderOrderNo, dashboard.SaaSPaymentWebhookEventStatusIgnored, "payment order already canceled"); err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
		updatedEvent, err := saasPaymentWebhookEventByIDTx(ctx, tx, event.ID)
		if err != nil {
			return dashboard.SaaSPaymentWebhookResult{}, err
		}
		return dashboard.SaaSPaymentWebhookResult{Event: updatedEvent, Order: order, Ignored: true}, nil
	}
	providerOrderNo := strings.TrimSpace(input.ProviderOrderNo)
	failureCode := truncateRunes(strings.TrimSpace(input.FailureCode), 64)
	failureMessage := truncateRunes(strings.TrimSpace(input.FailureMessage), 255)
	if status == dashboard.SaaSPaymentOrderStatusFailed && failureMessage == "" {
		failureMessage = "payment failed"
	}
	var action string
	var failedAt any
	var canceledAt any
	switch status {
	case dashboard.SaaSPaymentOrderStatusFailed:
		action = dashboard.SaaSAdminOperationActionPaymentOrderFailed
		failedAt = nullableTrimmedString(input.OccurredAt)
	case dashboard.SaaSPaymentOrderStatusCanceled:
		action = dashboard.SaaSAdminOperationActionPaymentOrderCancel
		canceledAt = nullableTrimmedString(input.OccurredAt)
	case dashboard.SaaSPaymentOrderStatusProcessing:
		action = dashboard.SaaSAdminOperationActionPaymentOrderProcess
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_orders
		SET status = ?, provider_order_no = COALESCE(NULLIF(?, ''), provider_order_no),
			attempt_count = attempt_count + 1, latest_webhook_event_id = ?,
			failed_at = ?, canceled_at = ?, failure_code = ?, failure_message = ?,
			next_dunning_at = CASE WHEN ? = 'failed' THEN NOW() ELSE NULL END,
			version = version + 1, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, status, providerOrderNo, event.ID, failedAt, canceledAt, failureCode, failureMessage, status, order.ID); err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if err := finalizeSaaSPaymentWebhookEventTx(ctx, tx, event.ID, order.ID, order.OrderNo, providerOrderNo, dashboard.SaaSPaymentWebhookEventStatusProcessed, ""); err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	next, _, err := saasPaymentOrderByNoTx(ctx, tx, order.OrderNo, true)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if _, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: order.TenantID, Action: action, TargetType: dashboard.SaaSAdminOperationTargetPaymentOrder,
		TargetID: order.OrderNo, TargetName: order.TenantName,
		BeforeJSON: saasAdminMarshalJSON(saasPaymentOrderStatePayload(order)),
		AfterJSON:  saasAdminMarshalJSON(map[string]any{"order": saasPaymentOrderStatePayload(next), "webhookEventId": input.EventID, "failureCode": failureCode, "failureMessage": failureMessage}),
		Remark:     "payment webhook " + input.EventID,
	}); err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	updatedEvent, err := saasPaymentWebhookEventByIDTx(ctx, tx, event.ID)
	if err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	return dashboard.SaaSPaymentWebhookResult{Event: updatedEvent, Order: next}, nil
}

func (s *MySQLStore) failSaaSPaymentWebhookTx(ctx context.Context, tx *sql.Tx, eventID int64, message string, returnErr error) (dashboard.SaaSPaymentWebhookResult, error) {
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_payment_webhook_events SET status = 'failed', last_error = ?, updated_at = NOW() WHERE id = ?`, truncateRunes(message, 255), eventID); err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSPaymentWebhookResult{}, err
	}
	return dashboard.SaaSPaymentWebhookResult{}, returnErr
}

func (s *MySQLStore) ProcessSaaSPaymentDunning(ctx context.Context, options dashboard.SaaSPaymentDunningOptions) (dashboard.SaaSPaymentDunningResult, error) {
	if options.Limit <= 0 {
		options.Limit = 100
	}
	if options.Limit > saasPaymentQueryMaxLimit {
		options.Limit = saasPaymentQueryMaxLimit
	}
	if options.RetryDelaySeconds < 60 {
		options.RetryDelaySeconds = 86400
	}
	if options.NotificationMaxAttempts <= 0 {
		options.NotificationMaxAttempts = 3
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT order_no
		FROM mochat_go_saas_payment_orders
		WHERE deleted_at IS NULL AND status = 'failed'
			AND dunning_attempts < max_dunning_attempts
			AND next_dunning_at IS NOT NULL AND next_dunning_at <= NOW()
		ORDER BY next_dunning_at ASC, id ASC
		LIMIT ?
	`, options.Limit)
	if err != nil {
		return dashboard.SaaSPaymentDunningResult{}, err
	}
	orderNos := make([]string, 0)
	for rows.Next() {
		var orderNo string
		if err := rows.Scan(&orderNo); err != nil {
			_ = rows.Close()
			return dashboard.SaaSPaymentDunningResult{}, err
		}
		orderNos = append(orderNos, orderNo)
	}
	if err := rows.Close(); err != nil {
		return dashboard.SaaSPaymentDunningResult{}, err
	}
	result := dashboard.SaaSPaymentDunningResult{MatchedCount: len(orderNos), DryRun: options.DryRun, OrderNos: orderNos}
	if options.DryRun {
		return result, nil
	}
	for _, orderNo := range orderNos {
		enqueued, exhausted, skipped, err := s.processSaaSPaymentDunningOrder(ctx, orderNo, options)
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, dashboard.SaaSPaymentDunningError{OrderNo: orderNo, Message: err.Error()})
			continue
		}
		if enqueued {
			result.EnqueuedCount++
		}
		if exhausted {
			result.ExhaustedCount++
		}
		if skipped {
			result.SkippedCount++
		}
	}
	return result, nil
}

func (s *MySQLStore) processSaaSPaymentDunningOrder(ctx context.Context, orderNo string, options dashboard.SaaSPaymentDunningOptions) (enqueued bool, exhausted bool, skipped bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, false, false, err
	}
	defer rollbackQuietly(tx)
	order, found, err := saasPaymentOrderByNoTx(ctx, tx, orderNo, true)
	if err != nil {
		return false, false, false, err
	}
	if !found || order.Status != dashboard.SaaSPaymentOrderStatusFailed || !order.DunningDue || order.DunningAttempts >= order.MaxDunningAttempts {
		return false, false, true, tx.Commit()
	}
	attempt := order.DunningAttempts + 1
	exhausted = attempt >= order.MaxDunningAttempts
	periodKey := fmt.Sprintf("payment:%s:dunning:%d", order.OrderNo, attempt)
	severity := dashboard.SaaSAlertSeverityWarning
	if exhausted {
		severity = dashboard.SaaSAlertSeverityCritical
	}
	alert := dashboard.SaaSQuotaAlert{
		Status:    dashboard.SaaSQuotaStatus{TenantID: order.TenantID, Metric: dashboard.SaaSEventMetricPaymentCollection, Current: int64(attempt), Limit: int64(order.MaxDunningAttempts)},
		AlertType: dashboard.SaaSAlertTypePaymentFailed, Severity: severity, PeriodKey: periodKey,
		Source: dashboard.SaaSPaymentDunningCronTaskName,
		Message: fmt.Sprintf("租户 %d 支付订单 %s 收款失败，催收 %d/%d，金额 %.2f %s",
			order.TenantID, order.OrderNo, attempt, order.MaxDunningAttempts, float64(order.AmountCents)/100, order.Currency),
		Context: map[string]any{
			"orderNo": order.OrderNo, "provider": order.Provider, "packageCode": order.PackageCode,
			"amountCents": order.AmountCents, "currency": order.Currency,
			"failureCode": order.FailureCode, "failureMessage": order.FailureMessage,
			"dunningAttempt": attempt, "maxDunningAttempts": order.MaxDunningAttempts,
		},
	}
	if err := enqueueSaaSPaymentDunningNotificationTx(ctx, tx, alert, options.NotificationMaxAttempts); err != nil {
		return false, false, false, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_orders
		SET dunning_attempts = ?, last_dunning_at = NOW(),
			next_dunning_at = IF(?, NULL, TIMESTAMPADD(SECOND, ?, NOW())),
			version = version + 1, updated_at = NOW()
		WHERE id = ? AND status = 'failed' AND dunning_attempts = ? AND deleted_at IS NULL
	`, attempt, exhausted, options.RetryDelaySeconds, order.ID, order.DunningAttempts); err != nil {
		return false, false, false, err
	}
	nextOrder, found, err := saasPaymentOrderByNoTx(ctx, tx, order.OrderNo, true)
	if err != nil {
		return false, false, false, err
	}
	if !found {
		return false, false, false, errors.New("payment order missing after dunning update")
	}
	if _, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: order.TenantID, ActorUserID: options.ActorUserID, ActorTenantID: options.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionPaymentDunning, TargetType: dashboard.SaaSAdminOperationTargetPaymentOrder,
		TargetID: order.OrderNo, TargetName: order.TenantName,
		BeforeJSON: saasAdminMarshalJSON(map[string]any{"dunningAttempts": order.DunningAttempts, "nextDunningAt": order.NextDunningAt}),
		AfterJSON:  saasAdminMarshalJSON(map[string]any{"dunningAttempts": attempt, "exhausted": exhausted, "periodKey": periodKey, "nextDunningAt": nextOrder.NextDunningAt}),
		Remark:     "payment failed dunning",
	}); err != nil && !isMissingSaaSTableError(err) {
		return false, false, false, err
	}
	if err := tx.Commit(); err != nil {
		return false, false, false, err
	}
	return true, exhausted, false, nil
}

func enqueueSaaSPaymentDunningNotificationTx(ctx context.Context, tx *sql.Tx, alert dashboard.SaaSQuotaAlert, maxAttempts int) error {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	alertKey := saasAlertKey(alert.Status.TenantID, alert.Status.Metric, alert.AlertType, alert.PeriodKey)
	notificationKey := dashboard.SaaSAlertNotificationKey(alert, dashboard.SaaSAlertNotificationChannelWebhook)
	raw, err := json.Marshal(alert)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_alert_notifications
			(notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts,
			 alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, 'webhook', 'pending', 0, ?, ?, '', NOW(), NULL, NOW(), NOW(), NULL)
		ON DUPLICATE KEY UPDATE alert_json = VALUES(alert_json), max_attempts = VALUES(max_attempts),
			status = IF(status = 'delivered', status, 'pending'), attempts = IF(status = 'delivered', attempts, 0),
			last_error = IF(status = 'delivered', last_error, ''), next_retry_at = IF(status = 'delivered', next_retry_at, NOW()),
			updated_at = NOW(), deleted_at = NULL
	`, notificationKey, alertKey, alert.Status.TenantID, maxAttempts, string(raw))
	return err
}

func ensureSaaSPaymentWebhookEventTx(ctx context.Context, tx *sql.Tx, input dashboard.SaaSPaymentWebhookInput) (dashboard.SaaSAdminPaymentWebhookEvent, bool, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO mochat_go_saas_payment_webhook_events
			(provider, event_id, event_type, order_id, refund_id, order_no, refund_no, provider_order_no, provider_refund_no, status,
			 payload_sha256, signature_timestamp, attempts, occurred_at, processed_at,
			 last_error, payload_json, created_at, updated_at)
		VALUES (?, ?, ?, 0, 0, ?, ?, ?, ?, 'received', ?, ?, 1, ?, NULL, '', ?, NOW(), NOW())
	`, input.Provider, input.EventID, input.EventType, input.OrderNo, input.RefundNo, input.ProviderOrderNo, input.ProviderRefundNo,
		input.PayloadSHA256, input.SignatureTimestamp, nullableTrimmedString(input.OccurredAt), saasAdminJSONValue(input.PayloadJSON))
	if err != nil {
		return dashboard.SaaSAdminPaymentWebhookEvent{}, false, err
	}
	rows, _ := result.RowsAffected()
	event, err := saasPaymentWebhookEventByProviderIDTx(ctx, tx, input.Provider, input.EventID, true)
	return event, rows > 0, err
}

func finalizeSaaSPaymentWebhookEventTx(ctx context.Context, tx *sql.Tx, eventID int64, orderID int64, orderNo string, providerOrderNo string, status string, lastError string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_webhook_events
		SET order_id = ?, order_no = ?, provider_order_no = ?, status = ?, processed_at = NOW(), last_error = ?, updated_at = NOW()
		WHERE id = ?
	`, orderID, orderNo, strings.TrimSpace(providerOrderNo), status, truncateRunes(lastError, 255), eventID)
	return err
}

func saasPaymentWebhookEventByProviderIDTx(ctx context.Context, tx *sql.Tx, provider string, eventID string, forUpdate bool) (dashboard.SaaSAdminPaymentWebhookEvent, error) {
	query := saasAdminPaymentWebhookEventSelectSQL() + ` WHERE e.provider = ? AND e.event_id = ? LIMIT 1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	return scanSaaSAdminPaymentWebhookEvent(tx.QueryRowContext(ctx, query, provider, eventID))
}

func saasPaymentWebhookEventByIDTx(ctx context.Context, tx *sql.Tx, eventID int64) (dashboard.SaaSAdminPaymentWebhookEvent, error) {
	return scanSaaSAdminPaymentWebhookEvent(tx.QueryRowContext(ctx, saasAdminPaymentWebhookEventSelectSQL()+` WHERE e.id = ? LIMIT 1`, eventID))
}

func saasPaymentOrderByNoTx(ctx context.Context, tx *sql.Tx, orderNo string, forUpdate bool) (dashboard.SaaSAdminPaymentOrder, bool, error) {
	query := saasAdminPaymentOrderSelectSQL() + ` WHERE o.order_no = ? AND o.deleted_at IS NULL LIMIT 1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	order, err := scanSaaSAdminPaymentOrder(tx.QueryRowContext(ctx, query, orderNo))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminPaymentOrder{}, false, nil
	}
	return order, err == nil, err
}

func saasPaymentOrderByIdempotencyTx(ctx context.Context, tx *sql.Tx, tenantID int, key string, forUpdate bool) (dashboard.SaaSAdminPaymentOrder, bool, error) {
	query := saasAdminPaymentOrderSelectSQL() + ` WHERE o.tenant_id = ? AND o.idempotency_key = ? AND o.deleted_at IS NULL LIMIT 1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	order, err := scanSaaSAdminPaymentOrder(tx.QueryRowContext(ctx, query, tenantID, key))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminPaymentOrder{}, false, nil
	}
	return order, err == nil, err
}

func saasAdminPaymentOrderSelectSQL() string {
	return `
		SELECT o.id, o.order_no, o.tenant_id, COALESCE(t.name, ''), o.provider,
			COALESCE(o.provider_order_no, ''), COALESCE(o.idempotency_key, ''), o.status,
			o.package_code, o.package_name, o.package_version, COALESCE(CAST(o.package_limits_json AS CHAR), '{}'), o.billing_cycle,
			COALESCE(DATE_FORMAT(o.service_expires_at, '%Y-%m-%d %H:%i:%s'), ''),
			o.amount_cents, o.refund_pending_amount_cents, o.refunded_amount_cents,
			GREATEST(CAST(o.amount_cents AS SIGNED) - CAST(o.refund_pending_amount_cents AS SIGNED) - CAST(o.refunded_amount_cents AS SIGNED), 0),
			CASE
				WHEN o.refunded_amount_cents >= o.amount_cents THEN 'full'
				WHEN o.refunded_amount_cents > 0 THEN 'partial'
				WHEN o.refund_pending_amount_cents > 0 THEN 'pending'
				ELSE 'none'
			END,
			o.invoice_pending_amount_cents, o.invoiced_amount_cents,
			o.credit_pending_amount_cents, o.credited_amount_cents,
			o.currency, o.checkout_url,
			COALESCE(DATE_FORMAT(o.checkout_expires_at, '%Y-%m-%d %H:%i:%s'), ''),
			o.attempt_count, o.dunning_attempts, o.max_dunning_attempts,
			COALESCE(DATE_FORMAT(o.next_dunning_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(o.last_dunning_at, '%Y-%m-%d %H:%i:%s'), ''),
			o.latest_webhook_event_id, o.billing_event_id, o.latest_refund_id, o.latest_invoice_document_id,
			COALESCE(DATE_FORMAT(o.paid_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(o.failed_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(o.canceled_at, '%Y-%m-%d %H:%i:%s'), ''),
			o.failure_code, o.failure_message, o.version,
			o.created_by_user_id, o.created_by_tenant_id, o.remark,
			COALESCE(CAST(o.metadata_json AS CHAR), ''),
			COALESCE(DATE_FORMAT(o.created_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(o.updated_at, '%Y-%m-%d %H:%i:%s'), ''),
			CASE WHEN o.status = 'failed' AND o.next_dunning_at IS NOT NULL AND o.next_dunning_at <= NOW()
				AND o.dunning_attempts < o.max_dunning_attempts THEN 1 ELSE 0 END
		FROM mochat_go_saas_payment_orders o
		LEFT JOIN mc_tenant t ON t.id = o.tenant_id AND t.deleted_at IS NULL
	`
}

func scanSaaSAdminPaymentOrder(scanner saasPaymentScanner) (dashboard.SaaSAdminPaymentOrder, error) {
	var order dashboard.SaaSAdminPaymentOrder
	var dunningDue int
	var packageLimitsJSON string
	err := scanner.Scan(
		&order.ID, &order.OrderNo, &order.TenantID, &order.TenantName, &order.Provider,
		&order.ProviderOrderNo, &order.IdempotencyKey, &order.Status,
		&order.PackageCode, &order.PackageName, &order.PackageVersion, &packageLimitsJSON, &order.BillingCycle, &order.ServiceExpiresAt,
		&order.AmountCents, &order.RefundPendingCents, &order.RefundedAmountCents,
		&order.RefundableAmountCents, &order.RefundStatus,
		&order.InvoicePendingCents, &order.InvoicedAmountCents, &order.CreditPendingCents, &order.CreditedAmountCents,
		&order.Currency, &order.CheckoutURL, &order.CheckoutExpiresAt,
		&order.AttemptCount, &order.DunningAttempts, &order.MaxDunningAttempts,
		&order.NextDunningAt, &order.LastDunningAt, &order.LatestWebhookEventID, &order.BillingEventID, &order.LatestRefundID, &order.LatestInvoiceID,
		&order.PaidAt, &order.FailedAt, &order.CanceledAt, &order.FailureCode, &order.FailureMessage,
		&order.Version, &order.CreatedByUserID, &order.CreatedByTenantID, &order.Remark, &order.MetadataJSON,
		&order.CreatedAt, &order.UpdatedAt, &dunningDue,
	)
	if err == nil && strings.TrimSpace(packageLimitsJSON) != "" {
		if unmarshalErr := json.Unmarshal([]byte(packageLimitsJSON), &order.PackageLimits); unmarshalErr != nil {
			return dashboard.SaaSAdminPaymentOrder{}, fmt.Errorf("decode payment order package limits: %w", unmarshalErr)
		}
	}
	order.DunningDue = dunningDue == 1
	order.NetInvoicedCents = saasPaymentPositiveDifference(order.InvoicedAmountCents, order.CreditedAmountCents)
	netPaidAfterPendingRefund := saasPaymentPositiveDifference(order.AmountCents, order.RefundedAmountCents+order.RefundPendingCents)
	order.InvoiceAvailableCents = saasPaymentPositiveDifference(netPaidAfterPendingRefund, order.NetInvoicedCents+order.InvoicePendingCents)
	refundCreditAvailable := saasPaymentPositiveDifference(order.RefundedAmountCents, order.CreditedAmountCents+order.CreditPendingCents)
	invoiceCreditAvailable := saasPaymentPositiveDifference(order.InvoicedAmountCents, order.CreditedAmountCents+order.CreditPendingCents)
	order.CreditNoteDueCents = minInt64(refundCreditAvailable, invoiceCreditAvailable)
	order.InvoiceStatus = saasPaymentInvoiceStatus(order)
	return order, err
}

func saasAdminPaymentWebhookEventSelectSQL() string {
	return `
		SELECT e.id, e.provider, e.event_id, e.event_type, e.order_id, e.order_no,
			e.provider_order_no, e.refund_id, e.refund_no, e.provider_refund_no,
			e.status, e.payload_sha256, e.signature_timestamp, e.attempts,
			COALESCE(DATE_FORMAT(e.occurred_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(e.processed_at, '%Y-%m-%d %H:%i:%s'), ''),
			e.last_error, COALESCE(CAST(e.payload_json AS CHAR), ''),
			COALESCE(DATE_FORMAT(e.created_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(e.updated_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_payment_webhook_events e
	`
}

func scanSaaSAdminPaymentWebhookEvent(scanner saasPaymentScanner) (dashboard.SaaSAdminPaymentWebhookEvent, error) {
	var event dashboard.SaaSAdminPaymentWebhookEvent
	err := scanner.Scan(
		&event.ID, &event.Provider, &event.EventID, &event.EventType, &event.OrderID, &event.OrderNo,
		&event.ProviderOrderNo, &event.RefundID, &event.RefundNo, &event.ProviderRefundNo,
		&event.Status, &event.PayloadSHA256, &event.SignatureTimestamp,
		&event.Attempts, &event.OccurredAt, &event.ProcessedAt, &event.LastError,
		&event.PayloadJSON, &event.CreatedAt, &event.UpdatedAt,
	)
	return event, err
}

func summarizeSaaSPaymentOrders(orders []dashboard.SaaSAdminPaymentOrder) dashboard.SaaSAdminPaymentOrderSummary {
	var summary dashboard.SaaSAdminPaymentOrderSummary
	tenants := map[int]struct{}{}
	providers := map[string]struct{}{}
	packages := map[string]struct{}{}
	for _, order := range orders {
		summary.OrderCount++
		summary.OrderAmountCents += order.AmountCents
		if order.TenantID > 0 {
			tenants[order.TenantID] = struct{}{}
		}
		if order.Provider != "" {
			providers[order.Provider] = struct{}{}
		}
		if order.PackageCode != "" {
			packages[order.PackageCode] = struct{}{}
		}
		switch order.Status {
		case dashboard.SaaSPaymentOrderStatusPending:
			summary.PendingCount++
			summary.CollectibleCount++
			summary.OutstandingCents += order.AmountCents
		case dashboard.SaaSPaymentOrderStatusProcessing:
			summary.ProcessingCount++
			summary.CollectibleCount++
			summary.OutstandingCents += order.AmountCents
		case dashboard.SaaSPaymentOrderStatusPaid:
			summary.PaidCount++
			summary.PaidAmountCents += order.AmountCents
			summary.RefundPendingCents += order.RefundPendingCents
			summary.RefundedAmountCents += order.RefundedAmountCents
			summary.NetPaidAmountCents += order.AmountCents - order.RefundedAmountCents
			summary.InvoicePendingCents += order.InvoicePendingCents
			summary.InvoicedAmountCents += order.InvoicedAmountCents
			summary.CreditPendingCents += order.CreditPendingCents
			summary.CreditedAmountCents += order.CreditedAmountCents
			summary.NetInvoicedCents += order.NetInvoicedCents
			summary.InvoiceAvailable += order.InvoiceAvailableCents
			summary.CreditNoteDueCents += order.CreditNoteDueCents
		case dashboard.SaaSPaymentOrderStatusFailed:
			summary.FailedCount++
			summary.CollectibleCount++
			summary.OutstandingCents += order.AmountCents
		case dashboard.SaaSPaymentOrderStatusCanceled:
			summary.CanceledCount++
		}
		if order.DunningDue {
			summary.DunningDueCount++
		}
	}
	summary.TenantCount = len(tenants)
	summary.ProviderCount = len(providers)
	summary.PackageCount = len(packages)
	return summary
}

func saasPaymentOrderMatchesCreate(order dashboard.SaaSAdminPaymentOrder, create dashboard.SaaSAdminPaymentOrderCreate) bool {
	packageVersionMatches := create.ExpectedPackageVersion <= 0 || order.PackageVersion == create.ExpectedPackageVersion
	return packageVersionMatches && order.TenantID == create.TenantID && order.Provider == strings.TrimSpace(create.Provider) &&
		order.PackageCode == strings.TrimSpace(create.PackageCode) && order.BillingCycle == strings.TrimSpace(create.BillingCycle) &&
		order.ServiceExpiresAt == strings.TrimSpace(create.ServiceExpiresAt) && order.AmountCents == create.AmountCents &&
		order.Currency == strings.TrimSpace(create.Currency)
}

func saasPaymentOrderStatePayload(order dashboard.SaaSAdminPaymentOrder) map[string]any {
	return map[string]any{
		"orderNo": order.OrderNo, "tenantId": order.TenantID, "provider": order.Provider,
		"providerOrderNo": order.ProviderOrderNo, "status": order.Status,
		"packageCode": order.PackageCode, "packageName": order.PackageName, "packageVersion": order.PackageVersion,
		"packageLimits": order.PackageLimits, "billingCycle": order.BillingCycle,
		"serviceExpiresAt": order.ServiceExpiresAt, "amountCents": order.AmountCents, "currency": order.Currency,
		"refundPendingCents": order.RefundPendingCents, "refundedAmountCents": order.RefundedAmountCents,
		"refundableAmountCents": order.RefundableAmountCents, "refundStatus": order.RefundStatus,
		"invoicePendingCents": order.InvoicePendingCents, "invoicedAmountCents": order.InvoicedAmountCents,
		"creditPendingCents": order.CreditPendingCents, "creditedAmountCents": order.CreditedAmountCents,
		"netInvoicedCents": order.NetInvoicedCents, "invoiceAvailableCents": order.InvoiceAvailableCents,
		"creditNoteDueCents": order.CreditNoteDueCents, "invoiceStatus": order.InvoiceStatus,
		"attemptCount": order.AttemptCount, "dunningAttempts": order.DunningAttempts,
		"latestWebhookEventId": order.LatestWebhookEventID, "billingEventId": order.BillingEventID, "latestRefundId": order.LatestRefundID,
		"latestInvoiceDocumentId": order.LatestInvoiceID,
		"failureCode":             order.FailureCode, "failureMessage": order.FailureMessage, "version": order.Version,
	}
}

func saasPaymentPositiveDifference(left int64, right int64) int64 {
	if left <= right {
		return 0
	}
	return left - right
}

func minInt64(left int64, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func saasPaymentInvoiceStatus(order dashboard.SaaSAdminPaymentOrder) string {
	if order.CreditNoteDueCents > 0 {
		return "credit_due"
	}
	if order.CreditPendingCents > 0 {
		return "credit_pending"
	}
	if order.InvoicePendingCents > 0 {
		return "invoice_pending"
	}
	if order.NetInvoicedCents > 0 && order.InvoiceAvailableCents == 0 {
		return "invoiced"
	}
	if order.NetInvoicedCents > 0 {
		return "partial"
	}
	return "none"
}

func saasPaymentTimeAfter(left string, right string) bool {
	leftTime, leftErr := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(left), time.Local)
	rightTime, rightErr := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(right), time.Local)
	return leftErr == nil && rightErr == nil && leftTime.After(rightTime)
}

func nullableTrimmedString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func saasPaymentConflict(message string) error {
	return &dashboard.SaaSAdminOperationError{Status: 409, Message: message}
}

func saasPaymentUnprocessable(message string) error {
	return &dashboard.SaaSAdminOperationError{Status: 422, Message: message}
}
