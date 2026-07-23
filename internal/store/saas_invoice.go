package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

const saasInvoiceQueryMaxLimit = 5000

func (s *MySQLStore) SaaSBillingProfile(ctx context.Context, tenantID int) (dashboard.SaaSBillingProfile, bool, error) {
	profile, err := scanSaaSBillingProfile(s.db.QueryRowContext(ctx, saasBillingProfileSelectSQL()+` WHERE p.tenant_id = ? AND p.deleted_at IS NULL LIMIT 1`, tenantID))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSBillingProfile{TenantID: tenantID, InvoiceType: dashboard.SaaSInvoiceTypeNormal, Status: 1}, false, nil
	}
	return profile, err == nil, err
}

func (s *MySQLStore) SaveSaaSBillingProfile(ctx context.Context, update dashboard.SaaSBillingProfileUpdate) (dashboard.SaaSBillingProfileUpdateResult, error) {
	if update.TenantID <= 0 || strings.TrimSpace(update.InvoiceTitle) == "" || strings.TrimSpace(update.TaxIdentifier) == "" || strings.TrimSpace(update.Email) == "" {
		return dashboard.SaaSBillingProfileUpdateResult{}, dashboard.NewSaaSAdminBadRequest("billing profile fields invalid")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSBillingProfileUpdateResult{}, err
	}
	defer tx.Rollback()
	var tenantName string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(name, '') FROM mc_tenant WHERE id = ? AND deleted_at IS NULL LIMIT 1 FOR UPDATE`, update.TenantID).Scan(&tenantName); errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSBillingProfileUpdateResult{}, dashboard.NewSaaSAdminNotFound("tenant not found")
	} else if err != nil {
		return dashboard.SaaSBillingProfileUpdateResult{}, err
	}
	current, found, err := saasBillingProfileByTenantTx(ctx, tx, update.TenantID, true)
	if err != nil {
		return dashboard.SaaSBillingProfileUpdateResult{}, err
	}
	created := !found
	if found {
		if update.ExpectedVersion <= 0 || update.ExpectedVersion != current.Version {
			return dashboard.SaaSBillingProfileUpdateResult{}, saasPaymentConflict(fmt.Sprintf("billing profile version conflict: current=%d", current.Version))
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_billing_profiles
			SET invoice_type = ?, invoice_title = ?, tax_identifier = ?, email = ?, phone = ?,
				registered_address = ?, bank_name = ?, bank_account = ?, recipient_name = ?,
				status = 1, version = version + 1, updated_by_user_id = ?, updated_by_tenant_id = ?,
				remark = ?, updated_at = NOW()
			WHERE id = ? AND tenant_id = ? AND version = ? AND deleted_at IS NULL
		`, strings.TrimSpace(update.InvoiceType), strings.TrimSpace(update.InvoiceTitle), strings.TrimSpace(update.TaxIdentifier),
			strings.TrimSpace(update.Email), strings.TrimSpace(update.Phone), strings.TrimSpace(update.RegisteredAddress),
			strings.TrimSpace(update.BankName), strings.TrimSpace(update.BankAccount), strings.TrimSpace(update.RecipientName),
			update.ActorUserID, update.ActorTenantID, strings.TrimSpace(update.Remark), current.ID, update.TenantID, current.Version)
		if err != nil {
			return dashboard.SaaSBillingProfileUpdateResult{}, err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return dashboard.SaaSBillingProfileUpdateResult{}, saasPaymentConflict("billing profile changed concurrently")
		}
	} else {
		if update.ExpectedVersion > 0 {
			return dashboard.SaaSBillingProfileUpdateResult{}, saasPaymentConflict("billing profile does not exist")
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_billing_profiles
				(tenant_id, invoice_type, invoice_title, tax_identifier, email, phone,
				 registered_address, bank_name, bank_account, recipient_name, status, version,
				 updated_by_user_id, updated_by_tenant_id, remark, created_at, updated_at, deleted_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 1, ?, ?, ?, NOW(), NOW(), NULL)
		`, update.TenantID, strings.TrimSpace(update.InvoiceType), strings.TrimSpace(update.InvoiceTitle),
			strings.TrimSpace(update.TaxIdentifier), strings.TrimSpace(update.Email), strings.TrimSpace(update.Phone),
			strings.TrimSpace(update.RegisteredAddress), strings.TrimSpace(update.BankName), strings.TrimSpace(update.BankAccount),
			strings.TrimSpace(update.RecipientName), update.ActorUserID, update.ActorTenantID, strings.TrimSpace(update.Remark))
		if err != nil {
			if isMySQLDuplicateKeyError(err) {
				return dashboard.SaaSBillingProfileUpdateResult{}, saasPaymentConflict("billing profile changed concurrently")
			}
			return dashboard.SaaSBillingProfileUpdateResult{}, err
		}
	}
	next, _, err := saasBillingProfileByTenantTx(ctx, tx, update.TenantID, true)
	if err != nil {
		return dashboard.SaaSBillingProfileUpdateResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: update.TenantID, ActorUserID: update.ActorUserID, ActorTenantID: update.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionBillingProfileUpdate, TargetType: dashboard.SaaSAdminOperationTargetBillingProfile,
		TargetID: fmt.Sprintf("%d", update.TenantID), TargetName: tenantName,
		BeforeJSON: saasAdminMarshalJSON(saasBillingProfileStatePayload(current)),
		AfterJSON:  saasAdminMarshalJSON(saasBillingProfileStatePayload(next)), Remark: update.Remark,
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSBillingProfileUpdateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSBillingProfileUpdateResult{}, err
	}
	return dashboard.SaaSBillingProfileUpdateResult{Profile: next, OperationID: operationID, Created: created}, nil
}

func (s *MySQLStore) SaaSInvoiceDocuments(ctx context.Context, options dashboard.SaaSInvoiceDocumentOptions) (dashboard.SaaSInvoiceDocumentReport, error) {
	where := []string{"d.deleted_at IS NULL"}
	args := []any{}
	if options.TenantID > 0 {
		where = append(where, "d.tenant_id = ?")
		args = append(args, options.TenantID)
	}
	if options.Kind != "" && options.Kind != dashboard.SaaSInvoiceKindAll {
		where = append(where, "d.kind = ?")
		args = append(args, strings.TrimSpace(options.Kind))
	}
	if options.Status != "" && options.Status != dashboard.SaaSInvoiceStatusAll {
		where = append(where, "d.status = ?")
		args = append(args, strings.TrimSpace(options.Status))
	}
	if strings.TrimSpace(options.OrderNo) != "" {
		where = append(where, "d.order_no = ?")
		args = append(args, strings.TrimSpace(options.OrderNo))
	}
	if strings.TrimSpace(options.DocumentNo) != "" {
		where = append(where, "d.document_no = ?")
		args = append(args, strings.TrimSpace(options.DocumentNo))
	}
	if strings.TrimSpace(options.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(options.Keyword) + "%"
		where = append(where, `(d.document_no LIKE ? OR d.order_no LIKE ? OR COALESCE(d.provider_document_no, '') LIKE ?
			OR d.invoice_title LIKE ? OR d.tax_identifier LIKE ? OR d.email LIKE ? OR COALESCE(t.name, '') LIKE ?
			OR COALESCE(original.document_no, '') LIKE ? OR d.remark LIKE ? OR d.failure_message LIKE ?)`)
		args = append(args, keyword, keyword, keyword, keyword, keyword, keyword, keyword, keyword, keyword, keyword)
	}
	args = append(args, saasInvoiceQueryMaxLimit)
	rows, err := s.db.QueryContext(ctx, saasInvoiceDocumentSelectSQL()+`
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY d.id DESC LIMIT ?`, args...)
	if err != nil {
		return dashboard.SaaSInvoiceDocumentReport{}, err
	}
	defer rows.Close()
	all := make([]dashboard.SaaSInvoiceDocument, 0)
	for rows.Next() {
		item, err := scanSaaSInvoiceDocument(rows)
		if err != nil {
			return dashboard.SaaSInvoiceDocumentReport{}, err
		}
		all = append(all, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSInvoiceDocumentReport{}, err
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > saasInvoiceQueryMaxLimit {
		limit = saasInvoiceQueryMaxLimit
	}
	items := all
	if len(items) > limit {
		items = items[:limit]
	}
	return dashboard.SaaSInvoiceDocumentReport{Options: options, Summary: summarizeSaaSInvoiceDocuments(all), Documents: items}, nil
}

func (s *MySQLStore) CreateSaaSInvoiceDocument(ctx context.Context, create dashboard.SaaSInvoiceDocumentCreate) (dashboard.SaaSInvoiceDocumentCreateResult, error) {
	if create.TenantID <= 0 || strings.TrimSpace(create.DocumentNo) == "" || strings.TrimSpace(create.OrderNo) == "" || create.AmountCents <= 0 {
		return dashboard.SaaSInvoiceDocumentCreateResult{}, dashboard.NewSaaSAdminBadRequest("invoice document fields invalid")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSInvoiceDocumentCreateResult{}, err
	}
	defer tx.Rollback()
	order, found, err := saasPaymentOrderByNoTx(ctx, tx, strings.TrimSpace(create.OrderNo), true)
	if err != nil {
		return dashboard.SaaSInvoiceDocumentCreateResult{}, err
	}
	if !found || order.TenantID != create.TenantID {
		return dashboard.SaaSInvoiceDocumentCreateResult{}, dashboard.NewSaaSAdminNotFound("payment order not found")
	}
	if order.Status != dashboard.SaaSPaymentOrderStatusPaid {
		return dashboard.SaaSInvoiceDocumentCreateResult{}, saasPaymentConflict("only paid payment order can be invoiced")
	}
	if strings.TrimSpace(create.Currency) == "" {
		create.Currency = order.Currency
	}
	if strings.TrimSpace(create.Currency) != order.Currency {
		return dashboard.SaaSInvoiceDocumentCreateResult{}, saasPaymentUnprocessable("invoice currency mismatch")
	}
	if strings.TrimSpace(create.IdempotencyKey) != "" {
		existing, found, err := saasInvoiceDocumentByIdempotencyTx(ctx, tx, create.TenantID, strings.TrimSpace(create.IdempotencyKey), true)
		if err != nil {
			return dashboard.SaaSInvoiceDocumentCreateResult{}, err
		}
		if found {
			if !saasInvoiceDocumentMatchesCreate(existing, create) {
				return dashboard.SaaSInvoiceDocumentCreateResult{}, saasPaymentConflict("invoice idempotency key already used with different request")
			}
			if err := tx.Commit(); err != nil {
				return dashboard.SaaSInvoiceDocumentCreateResult{}, err
			}
			return dashboard.SaaSInvoiceDocumentCreateResult{Document: existing, Order: order, Idempotent: true}, nil
		}
	}

	profile, profileFound, err := saasBillingProfileByTenantTx(ctx, tx, create.TenantID, true)
	if err != nil {
		return dashboard.SaaSInvoiceDocumentCreateResult{}, err
	}
	var original dashboard.SaaSInvoiceDocument
	if create.Kind == dashboard.SaaSInvoiceKindInvoice {
		if !profileFound || profile.Status != 1 {
			return dashboard.SaaSInvoiceDocumentCreateResult{}, saasPaymentConflict("billing profile is required before requesting an invoice")
		}
		if create.AmountCents > order.InvoiceAvailableCents {
			return dashboard.SaaSInvoiceDocumentCreateResult{}, saasPaymentConflict(fmt.Sprintf("invoice amount exceeds available amount: available=%d", order.InvoiceAvailableCents))
		}
	} else if create.Kind == dashboard.SaaSInvoiceKindCreditNote {
		original, found, err = saasInvoiceDocumentByNoTx(ctx, tx, strings.TrimSpace(create.OriginalDocumentNo), true)
		if err != nil {
			return dashboard.SaaSInvoiceDocumentCreateResult{}, err
		}
		if !found || original.TenantID != create.TenantID || original.PaymentOrderID != order.ID || original.Kind != dashboard.SaaSInvoiceKindInvoice || original.Status != dashboard.SaaSInvoiceStatusIssued {
			return dashboard.SaaSInvoiceDocumentCreateResult{}, saasPaymentConflict("issued original invoice not found for this order")
		}
		used, err := saasInvoiceOriginalCreditReservedTx(ctx, tx, original.ID)
		if err != nil {
			return dashboard.SaaSInvoiceDocumentCreateResult{}, err
		}
		originalAvailable := saasPaymentPositiveDifference(original.AmountCents, used)
		available := minInt64(order.CreditNoteDueCents, originalAvailable)
		if create.AmountCents > available {
			return dashboard.SaaSInvoiceDocumentCreateResult{}, saasPaymentConflict(fmt.Sprintf("credit note amount exceeds available amount: available=%d", available))
		}
		profile = dashboard.SaaSBillingProfile{
			TenantID: create.TenantID, InvoiceType: original.InvoiceType, InvoiceTitle: original.InvoiceTitle,
			TaxIdentifier: original.TaxIdentifier, Email: original.Email, Phone: original.Phone,
			RegisteredAddress: original.RegisteredAddress, BankName: original.BankName,
			BankAccount: original.BankAccount, RecipientName: original.RecipientName, Status: 1,
		}
	} else {
		return dashboard.SaaSInvoiceDocumentCreateResult{}, dashboard.NewSaaSAdminBadRequest("invoice kind invalid")
	}
	provider := strings.TrimSpace(create.Provider)
	if provider == "" {
		provider = "manual"
	}
	result, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO mochat_go_saas_invoice_documents
			(document_no, tenant_id, payment_order_id, order_no, kind, original_document_id, idempotency_key,
			 status, amount_cents, currency, invoice_type, invoice_title, tax_identifier, email, phone,
			 registered_address, bank_name, bank_account, recipient_name, provider, provider_document_no,
			 document_url, operation_id, requested_by_user_id, requested_by_tenant_id,
			 processed_by_user_id, processed_by_tenant_id, requested_at, version, remark, metadata_json,
			 created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'requested', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL,
			'', 0, ?, ?, 0, 0, NOW(), 1, ?, ?, NOW(), NOW(), NULL)
	`, create.DocumentNo, create.TenantID, order.ID, order.OrderNo, create.Kind, original.ID,
		nullableTrimmedString(create.IdempotencyKey), create.AmountCents, create.Currency,
		profile.InvoiceType, profile.InvoiceTitle, profile.TaxIdentifier, profile.Email, profile.Phone,
		profile.RegisteredAddress, profile.BankName, profile.BankAccount, profile.RecipientName, provider,
		create.ActorUserID, create.ActorTenantID, strings.TrimSpace(create.Remark), saasAdminJSONValue(create.MetadataJSON))
	if err != nil {
		return dashboard.SaaSInvoiceDocumentCreateResult{}, err
	}
	insertedID, _ := result.LastInsertId()
	if affected, _ := result.RowsAffected(); affected != 1 {
		existing, found, err := saasInvoiceDocumentByNoTx(ctx, tx, create.DocumentNo, true)
		if err != nil {
			return dashboard.SaaSInvoiceDocumentCreateResult{}, err
		}
		if !found && strings.TrimSpace(create.IdempotencyKey) != "" {
			existing, found, err = saasInvoiceDocumentByIdempotencyTx(ctx, tx, create.TenantID, create.IdempotencyKey, true)
			if err != nil {
				return dashboard.SaaSInvoiceDocumentCreateResult{}, err
			}
		}
		if !found || !saasInvoiceDocumentMatchesCreate(existing, create) {
			return dashboard.SaaSInvoiceDocumentCreateResult{}, saasPaymentConflict("invoice number or idempotency key already exists")
		}
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSInvoiceDocumentCreateResult{}, err
		}
		return dashboard.SaaSInvoiceDocumentCreateResult{Document: existing, Order: order, Idempotent: true}, nil
	}
	orderColumn := "invoice_pending_amount_cents"
	if create.Kind == dashboard.SaaSInvoiceKindCreditNote {
		orderColumn = "credit_pending_amount_cents"
	}
	orderUpdate, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_payment_orders SET `+orderColumn+` = `+orderColumn+` + ?, latest_invoice_document_id = ?, version = version + 1, updated_at = NOW() WHERE id = ? AND status = 'paid' AND deleted_at IS NULL`, create.AmountCents, insertedID, order.ID)
	if err != nil {
		return dashboard.SaaSInvoiceDocumentCreateResult{}, err
	}
	if affected, _ := orderUpdate.RowsAffected(); affected != 1 {
		return dashboard.SaaSInvoiceDocumentCreateResult{}, saasPaymentConflict("payment order changed concurrently")
	}
	document, _, err := saasInvoiceDocumentByNoTx(ctx, tx, create.DocumentNo, true)
	if err != nil {
		return dashboard.SaaSInvoiceDocumentCreateResult{}, err
	}
	nextOrder, _, err := saasPaymentOrderByNoTx(ctx, tx, order.OrderNo, true)
	if err != nil {
		return dashboard.SaaSInvoiceDocumentCreateResult{}, err
	}
	action := dashboard.SaaSAdminOperationActionInvoiceRequest
	if create.Kind == dashboard.SaaSInvoiceKindCreditNote {
		action = dashboard.SaaSAdminOperationActionCreditNoteRequest
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: create.TenantID, ActorUserID: create.ActorUserID, ActorTenantID: create.ActorTenantID,
		Action: action, TargetType: dashboard.SaaSAdminOperationTargetInvoiceDocument,
		TargetID: document.DocumentNo, TargetName: document.TenantName,
		BeforeJSON: saasAdminMarshalJSON(map[string]any{"order": saasPaymentOrderStatePayload(order)}),
		AfterJSON:  saasAdminMarshalJSON(map[string]any{"document": saasInvoiceDocumentStatePayload(document), "order": saasPaymentOrderStatePayload(nextOrder)}),
		Remark:     create.Remark,
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSInvoiceDocumentCreateResult{}, err
	}
	if operationID > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_invoice_documents SET operation_id = ? WHERE id = ?`, operationID, document.ID); err != nil {
			return dashboard.SaaSInvoiceDocumentCreateResult{}, err
		}
		document.OperationID = operationID
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSInvoiceDocumentCreateResult{}, err
	}
	return dashboard.SaaSInvoiceDocumentCreateResult{Document: document, Order: nextOrder, OperationID: operationID}, nil
}

func (s *MySQLStore) TransitionSaaSInvoiceDocument(ctx context.Context, transition dashboard.SaaSInvoiceDocumentTransition) (dashboard.SaaSInvoiceDocumentTransitionResult, error) {
	approvedExecution := transition.ApprovalExecutionID > 0
	if approvedExecution != (transition.ApprovalExecutionVersion > 0) {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, dashboard.NewSaaSAdminBadRequest("审批执行引用无效")
	}
	if approvedExecution && (transition.Status != dashboard.SaaSInvoiceStatusIssued || transition.ApprovalPlan == nil) {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, dashboard.NewSaaSAdminBadRequest("发票开具审批冻结快照无效")
	}
	if !approvedExecution && transition.ApprovalPlan != nil {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, dashboard.NewSaaSAdminBadRequest("发票开具审批执行引用缺失")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, err
	}
	defer tx.Rollback()
	current, found, err := saasInvoiceDocumentByNoTx(ctx, tx, strings.TrimSpace(transition.DocumentNo), true)
	if err != nil {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, err
	}
	if !found || (transition.TenantID > 0 && current.TenantID != transition.TenantID) {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, dashboard.NewSaaSAdminNotFound("invoice document not found")
	}
	if approvedExecution {
		plan := transition.ApprovalPlan
		document := plan.Document
		if plan.Transition.DocumentNo != transition.DocumentNo ||
			plan.Transition.ExpectedVersion != transition.ExpectedVersion ||
			plan.Transition.Status != transition.Status ||
			document.ID != current.ID ||
			document.DocumentNo != current.DocumentNo ||
			document.TenantID != current.TenantID ||
			document.PaymentOrderID != current.PaymentOrderID ||
			document.OrderNo != current.OrderNo ||
			document.Kind != current.Kind ||
			document.OriginalDocumentID != current.OriginalDocumentID ||
			document.OriginalDocumentNo != current.OriginalDocumentNo ||
			document.Status != current.Status ||
			document.AmountCents != current.AmountCents ||
			document.Currency != current.Currency ||
			document.InvoiceType != current.InvoiceType ||
			document.InvoiceTitle != current.InvoiceTitle ||
			document.TaxIdentifier != current.TaxIdentifier ||
			document.Provider != current.Provider ||
			document.Version != current.Version {
			return dashboard.SaaSInvoiceDocumentTransitionResult{}, &dashboard.SaaSAdminOperationError{
				Status: 409, Message: "发票单据已变化，请刷新后重新申请审批",
			}
		}
	}
	if transition.ExpectedVersion != current.Version {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, saasPaymentConflict(fmt.Sprintf("invoice version conflict: current=%d", current.Version))
	}
	if !dashboard.SaaSInvoiceTransitionAllowed(current.Status, transition.Status) {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, saasPaymentConflict("invoice status transition not allowed: " + current.Status + " -> " + transition.Status)
	}
	if transition.TenantID > 0 && (current.Status != dashboard.SaaSInvoiceStatusRequested || transition.Status != dashboard.SaaSInvoiceStatusCanceled) {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, saasPaymentConflict("tenant can only cancel a requested invoice")
	}
	order, found, err := saasPaymentOrderByNoTx(ctx, tx, current.OrderNo, true)
	if err != nil {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, err
	}
	if !found {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, dashboard.NewSaaSAdminNotFound("payment order not found")
	}
	if approvedExecution {
		frozen := transition.ApprovalPlan.Order
		if frozen.ID != order.ID ||
			frozen.OrderNo != order.OrderNo ||
			frozen.TenantID != order.TenantID ||
			frozen.Status != order.Status ||
			frozen.AmountCents != order.AmountCents ||
			frozen.RefundPendingCents != order.RefundPendingCents ||
			frozen.RefundedAmountCents != order.RefundedAmountCents ||
			frozen.InvoicePendingCents != order.InvoicePendingCents ||
			frozen.InvoicedAmountCents != order.InvoicedAmountCents ||
			frozen.CreditPendingCents != order.CreditPendingCents ||
			frozen.CreditedAmountCents != order.CreditedAmountCents ||
			frozen.Version != order.Version {
			return dashboard.SaaSInvoiceDocumentTransitionResult{}, &dashboard.SaaSAdminOperationError{
				Status: 409, Message: "支付订单或开票金额台账已变化，请刷新后重新申请审批",
			}
		}
	}
	if current.Status == transition.Status {
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSInvoiceDocumentTransitionResult{}, err
		}
		return dashboard.SaaSInvoiceDocumentTransitionResult{Document: current, Order: order, PreviousStatus: current.Status}, nil
	}
	if transition.Status == dashboard.SaaSInvoiceStatusIssued {
		if err := applySaaSInvoiceIssuedOrderTx(ctx, tx, order, current); err != nil {
			return dashboard.SaaSInvoiceDocumentTransitionResult{}, err
		}
	} else if transition.Status == dashboard.SaaSInvoiceStatusFailed || transition.Status == dashboard.SaaSInvoiceStatusCanceled {
		if err := releaseSaaSInvoiceReservationTx(ctx, tx, order.ID, current); err != nil {
			return dashboard.SaaSInvoiceDocumentTransitionResult{}, err
		}
	}
	provider := strings.TrimSpace(transition.Provider)
	if provider == "" {
		provider = current.Provider
	}
	issuedAt := strings.TrimSpace(transition.IssuedAt)
	if issuedAt == "" && transition.Status == dashboard.SaaSInvoiceStatusIssued {
		issuedAt, err = saasPaymentDBNowStringTx(ctx, tx)
		if err != nil {
			return dashboard.SaaSInvoiceDocumentTransitionResult{}, err
		}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_invoice_documents
		SET status = ?, provider = ?, provider_document_no = COALESCE(NULLIF(?, ''), provider_document_no),
			document_url = CASE WHEN ? <> '' THEN ? ELSE document_url END,
			processed_by_user_id = ?, processed_by_tenant_id = ?,
			processing_at = CASE WHEN ? = 'processing' THEN NOW() ELSE processing_at END,
			issued_at = CASE WHEN ? = 'issued' THEN ? ELSE issued_at END,
			failed_at = CASE WHEN ? = 'failed' THEN NOW() ELSE failed_at END,
			canceled_at = CASE WHEN ? = 'canceled' THEN NOW() ELSE canceled_at END,
			failure_code = ?, failure_message = ?, version = version + 1,
			remark = CASE WHEN ? <> '' THEN ? ELSE remark END, updated_at = NOW()
		WHERE id = ? AND version = ? AND status = ? AND deleted_at IS NULL
	`, transition.Status, provider, strings.TrimSpace(transition.ProviderDocumentNo),
		strings.TrimSpace(transition.DocumentURL), strings.TrimSpace(transition.DocumentURL),
		transition.ActorUserID, transition.ActorTenantID,
		transition.Status, transition.Status, nullableTrimmedString(issuedAt), transition.Status, transition.Status,
		strings.TrimSpace(transition.FailureCode), strings.TrimSpace(transition.FailureMessage),
		strings.TrimSpace(transition.Remark), strings.TrimSpace(transition.Remark), current.ID, current.Version, current.Status)
	if err != nil {
		if isMySQLDuplicateKeyError(err) {
			return dashboard.SaaSInvoiceDocumentTransitionResult{}, saasPaymentConflict("invoice provider document number already exists")
		}
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, saasPaymentConflict("invoice document changed concurrently")
	}
	next, _, err := saasInvoiceDocumentByNoTx(ctx, tx, current.DocumentNo, true)
	if err != nil {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, err
	}
	nextOrder, _, err := saasPaymentOrderByNoTx(ctx, tx, order.OrderNo, true)
	if err != nil {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: current.TenantID, ActorUserID: transition.ActorUserID, ActorTenantID: transition.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionInvoiceTransition, TargetType: dashboard.SaaSAdminOperationTargetInvoiceDocument,
		TargetID: current.DocumentNo, TargetName: current.TenantName,
		BeforeJSON: saasAdminMarshalJSON(map[string]any{"document": saasInvoiceDocumentStatePayload(current), "order": saasPaymentOrderStatePayload(order)}),
		AfterJSON:  saasAdminMarshalJSON(map[string]any{"document": saasInvoiceDocumentStatePayload(next), "order": saasPaymentOrderStatePayload(nextOrder)}),
		Remark:     transition.Remark,
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, err
	}
	if operationID > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_invoice_documents SET operation_id = ? WHERE id = ?`, operationID, next.ID); err != nil {
			return dashboard.SaaSInvoiceDocumentTransitionResult{}, err
		}
		next.OperationID = operationID
	}
	if err := markSaaSAdminApprovalEffectTx(
		ctx, tx, transition.ApprovalExecutionID, transition.ApprovalExecutionVersion, transition.ActorUserID, operationID,
	); err != nil {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSInvoiceDocumentTransitionResult{}, err
	}
	return dashboard.SaaSInvoiceDocumentTransitionResult{Document: next, Order: nextOrder, PreviousStatus: current.Status, OperationID: operationID}, nil
}

func applySaaSInvoiceIssuedOrderTx(ctx context.Context, tx *sql.Tx, order dashboard.SaaSAdminPaymentOrder, document dashboard.SaaSInvoiceDocument) error {
	if document.Kind == dashboard.SaaSInvoiceKindInvoice {
		result, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_payment_orders
			SET invoice_pending_amount_cents = invoice_pending_amount_cents - ?,
				invoiced_amount_cents = invoiced_amount_cents + ?, latest_invoice_document_id = ?,
				version = version + 1, updated_at = NOW()
			WHERE id = ? AND status = 'paid' AND invoice_pending_amount_cents >= ? AND deleted_at IS NULL
		`, document.AmountCents, document.AmountCents, document.ID, order.ID, document.AmountCents)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return saasPaymentConflict("invoice reservation changed concurrently")
		}
		return nil
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_orders
		SET credit_pending_amount_cents = credit_pending_amount_cents - ?,
			credited_amount_cents = credited_amount_cents + ?, latest_invoice_document_id = ?,
			version = version + 1, updated_at = NOW()
		WHERE id = ? AND status = 'paid' AND credit_pending_amount_cents >= ?
			AND refunded_amount_cents >= credited_amount_cents + ?
			AND invoiced_amount_cents >= credited_amount_cents + ? AND deleted_at IS NULL
	`, document.AmountCents, document.AmountCents, document.ID, order.ID, document.AmountCents, document.AmountCents, document.AmountCents)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return saasPaymentConflict("credit note reservation changed concurrently")
	}
	return nil
}

func releaseSaaSInvoiceReservationTx(ctx context.Context, tx *sql.Tx, orderID int64, document dashboard.SaaSInvoiceDocument) error {
	column := "invoice_pending_amount_cents"
	if document.Kind == dashboard.SaaSInvoiceKindCreditNote {
		column = "credit_pending_amount_cents"
	}
	result, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_payment_orders SET `+column+` = `+column+` - ?, latest_invoice_document_id = ?, version = version + 1, updated_at = NOW() WHERE id = ? AND `+column+` >= ? AND deleted_at IS NULL`, document.AmountCents, document.ID, orderID, document.AmountCents)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return saasPaymentConflict("invoice reservation changed concurrently")
	}
	return nil
}

func saasPaymentDBNowStringTx(ctx context.Context, tx *sql.Tx) (string, error) {
	var value string
	if err := tx.QueryRowContext(ctx, `SELECT DATE_FORMAT(NOW(), '%Y-%m-%d %H:%i:%s')`).Scan(&value); err != nil {
		return "", err
	}
	return value, nil
}

func saasBillingProfileByTenantTx(ctx context.Context, tx *sql.Tx, tenantID int, forUpdate bool) (dashboard.SaaSBillingProfile, bool, error) {
	query := saasBillingProfileSelectSQL() + ` WHERE p.tenant_id = ? AND p.deleted_at IS NULL LIMIT 1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	profile, err := scanSaaSBillingProfile(tx.QueryRowContext(ctx, query, tenantID))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSBillingProfile{TenantID: tenantID, InvoiceType: dashboard.SaaSInvoiceTypeNormal, Status: 1}, false, nil
	}
	return profile, err == nil, err
}

func saasBillingProfileSelectSQL() string {
	return `
		SELECT p.id, p.tenant_id, COALESCE(t.name, ''), p.invoice_type, p.invoice_title,
			p.tax_identifier, p.email, p.phone, p.registered_address, p.bank_name,
			p.bank_account, p.recipient_name, p.status, p.version,
			p.updated_by_user_id, p.updated_by_tenant_id, p.remark,
			COALESCE(DATE_FORMAT(p.created_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(p.updated_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_billing_profiles p
		LEFT JOIN mc_tenant t ON t.id = p.tenant_id AND t.deleted_at IS NULL
	`
}

func scanSaaSBillingProfile(scanner saasPaymentScanner) (dashboard.SaaSBillingProfile, error) {
	var item dashboard.SaaSBillingProfile
	err := scanner.Scan(&item.ID, &item.TenantID, &item.TenantName, &item.InvoiceType, &item.InvoiceTitle,
		&item.TaxIdentifier, &item.Email, &item.Phone, &item.RegisteredAddress, &item.BankName,
		&item.BankAccount, &item.RecipientName, &item.Status, &item.Version,
		&item.UpdatedByUserID, &item.UpdatedByTenantID, &item.Remark, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func saasInvoiceDocumentByNoTx(ctx context.Context, tx *sql.Tx, documentNo string, forUpdate bool) (dashboard.SaaSInvoiceDocument, bool, error) {
	query := saasInvoiceDocumentSelectSQL() + ` WHERE d.document_no = ? AND d.deleted_at IS NULL LIMIT 1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	item, err := scanSaaSInvoiceDocument(tx.QueryRowContext(ctx, query, strings.TrimSpace(documentNo)))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSInvoiceDocument{}, false, nil
	}
	return item, err == nil, err
}

func saasInvoiceDocumentByIdempotencyTx(ctx context.Context, tx *sql.Tx, tenantID int, key string, forUpdate bool) (dashboard.SaaSInvoiceDocument, bool, error) {
	query := saasInvoiceDocumentSelectSQL() + ` WHERE d.tenant_id = ? AND d.idempotency_key = ? AND d.deleted_at IS NULL LIMIT 1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	item, err := scanSaaSInvoiceDocument(tx.QueryRowContext(ctx, query, tenantID, strings.TrimSpace(key)))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSInvoiceDocument{}, false, nil
	}
	return item, err == nil, err
}

func saasInvoiceOriginalCreditReservedTx(ctx context.Context, tx *sql.Tx, originalID int64) (int64, error) {
	var amount int64
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM mochat_go_saas_invoice_documents
		WHERE original_document_id = ? AND kind = 'credit_note'
			AND status IN ('requested', 'processing', 'issued') AND deleted_at IS NULL
	`, originalID).Scan(&amount)
	return amount, err
}

func saasInvoiceDocumentSelectSQL() string {
	return `
			SELECT d.id, d.document_no, d.tenant_id, COALESCE(t.name, ''),
				d.payment_order_id, d.order_no, COALESCE(o.status, ''), COALESCE(o.version, 0),
				d.kind, d.original_document_id,
			COALESCE(original.document_no, ''), COALESCE(d.idempotency_key, ''), d.status,
			d.amount_cents, d.currency, d.invoice_type, d.invoice_title, d.tax_identifier,
			d.email, d.phone, d.registered_address, d.bank_name, d.bank_account, d.recipient_name,
			d.provider, COALESCE(d.provider_document_no, ''), d.document_url, d.operation_id,
			d.requested_by_user_id, d.requested_by_tenant_id, d.processed_by_user_id, d.processed_by_tenant_id,
			COALESCE(DATE_FORMAT(d.requested_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(d.processing_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(d.issued_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(d.failed_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(d.canceled_at, '%Y-%m-%d %H:%i:%s'), ''),
			d.failure_code, d.failure_message, d.version, d.remark,
			COALESCE(CAST(d.metadata_json AS CHAR), ''),
			COALESCE(DATE_FORMAT(d.created_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(d.updated_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(o.amount_cents, 0), COALESCE(o.refund_pending_amount_cents, 0),
			COALESCE(o.refunded_amount_cents, 0), COALESCE(o.invoice_pending_amount_cents, 0),
			COALESCE(o.invoiced_amount_cents, 0), COALESCE(o.credit_pending_amount_cents, 0),
			COALESCE(o.credited_amount_cents, 0)
		FROM mochat_go_saas_invoice_documents d
		LEFT JOIN mc_tenant t ON t.id = d.tenant_id AND t.deleted_at IS NULL
		LEFT JOIN mochat_go_saas_invoice_documents original ON original.id = d.original_document_id AND original.deleted_at IS NULL
		LEFT JOIN mochat_go_saas_payment_orders o ON o.id = d.payment_order_id AND o.deleted_at IS NULL
	`
}

func scanSaaSInvoiceDocument(scanner saasPaymentScanner) (dashboard.SaaSInvoiceDocument, error) {
	var item dashboard.SaaSInvoiceDocument
	err := scanner.Scan(
		&item.ID, &item.DocumentNo, &item.TenantID, &item.TenantName,
		&item.PaymentOrderID, &item.OrderNo, &item.OrderStatus, &item.OrderVersion,
		&item.Kind, &item.OriginalDocumentID,
		&item.OriginalDocumentNo, &item.IdempotencyKey, &item.Status,
		&item.AmountCents, &item.Currency, &item.InvoiceType, &item.InvoiceTitle, &item.TaxIdentifier,
		&item.Email, &item.Phone, &item.RegisteredAddress, &item.BankName, &item.BankAccount, &item.RecipientName,
		&item.Provider, &item.ProviderDocumentNo, &item.DocumentURL, &item.OperationID,
		&item.RequestedByUserID, &item.RequestedByTenantID, &item.ProcessedByUserID, &item.ProcessedByTenantID,
		&item.RequestedAt, &item.ProcessingAt, &item.IssuedAt, &item.FailedAt, &item.CanceledAt,
		&item.FailureCode, &item.FailureMessage, &item.Version, &item.Remark, &item.MetadataJSON,
		&item.CreatedAt, &item.UpdatedAt, &item.OrderAmountCents, &item.OrderRefundPending,
		&item.OrderRefundedCents, &item.OrderInvoicePending, &item.OrderInvoicedCents,
		&item.OrderCreditPending, &item.OrderCreditedCents,
	)
	item.OrderNetPaidCents = saasPaymentPositiveDifference(item.OrderAmountCents, item.OrderRefundedCents)
	item.OrderNetInvoicedCents = saasPaymentPositiveDifference(item.OrderInvoicedCents, item.OrderCreditedCents)
	netPaidAfterPendingRefund := saasPaymentPositiveDifference(item.OrderAmountCents, item.OrderRefundedCents+item.OrderRefundPending)
	item.OrderInvoiceAvailable = saasPaymentPositiveDifference(netPaidAfterPendingRefund, item.OrderNetInvoicedCents+item.OrderInvoicePending)
	refundCreditAvailable := saasPaymentPositiveDifference(item.OrderRefundedCents, item.OrderCreditedCents+item.OrderCreditPending)
	invoiceCreditAvailable := saasPaymentPositiveDifference(item.OrderInvoicedCents, item.OrderCreditedCents+item.OrderCreditPending)
	item.OrderCreditNoteDue = minInt64(refundCreditAvailable, invoiceCreditAvailable)
	return item, err
}

func summarizeSaaSInvoiceDocuments(items []dashboard.SaaSInvoiceDocument) dashboard.SaaSInvoiceDocumentSummary {
	var summary dashboard.SaaSInvoiceDocumentSummary
	tenants := map[int]struct{}{}
	orders := map[int64]struct{}{}
	for _, item := range items {
		summary.DocumentCount++
		if item.Kind == dashboard.SaaSInvoiceKindInvoice {
			summary.InvoiceCount++
		} else if item.Kind == dashboard.SaaSInvoiceKindCreditNote {
			summary.CreditNoteCount++
		}
		switch item.Status {
		case dashboard.SaaSInvoiceStatusRequested:
			summary.RequestedCount++
			summary.RequestedAmount += item.AmountCents
		case dashboard.SaaSInvoiceStatusProcessing:
			summary.ProcessingCount++
			summary.RequestedAmount += item.AmountCents
		case dashboard.SaaSInvoiceStatusIssued:
			summary.IssuedCount++
			if item.Kind == dashboard.SaaSInvoiceKindCreditNote {
				summary.IssuedCreditAmount += item.AmountCents
			} else {
				summary.IssuedInvoiceAmount += item.AmountCents
			}
		case dashboard.SaaSInvoiceStatusFailed:
			summary.FailedCount++
		case dashboard.SaaSInvoiceStatusCanceled:
			summary.CanceledCount++
		}
		if item.TenantID > 0 {
			tenants[item.TenantID] = struct{}{}
		}
		if item.PaymentOrderID > 0 {
			orders[item.PaymentOrderID] = struct{}{}
		}
	}
	summary.NetIssuedAmount = summary.IssuedInvoiceAmount - summary.IssuedCreditAmount
	summary.TenantCount = len(tenants)
	summary.OrderCount = len(orders)
	return summary
}

func saasInvoiceDocumentMatchesCreate(item dashboard.SaaSInvoiceDocument, create dashboard.SaaSInvoiceDocumentCreate) bool {
	return item.TenantID == create.TenantID && item.OrderNo == strings.TrimSpace(create.OrderNo) &&
		item.Kind == strings.TrimSpace(create.Kind) && item.OriginalDocumentNo == strings.TrimSpace(create.OriginalDocumentNo) &&
		item.AmountCents == create.AmountCents && item.Currency == strings.TrimSpace(create.Currency)
}

func saasBillingProfileStatePayload(item dashboard.SaaSBillingProfile) map[string]any {
	return map[string]any{
		"tenantId": item.TenantID, "invoiceType": item.InvoiceType, "invoiceTitle": item.InvoiceTitle,
		"taxIdentifier": item.TaxIdentifier, "email": item.Email, "phone": item.Phone,
		"registeredAddress": item.RegisteredAddress, "bankName": item.BankName,
		"bankAccount": item.BankAccount, "recipientName": item.RecipientName,
		"status": item.Status, "version": item.Version,
	}
}

func saasInvoiceDocumentStatePayload(item dashboard.SaaSInvoiceDocument) map[string]any {
	return map[string]any{
		"documentNo": item.DocumentNo, "tenantId": item.TenantID, "orderNo": item.OrderNo,
		"kind": item.Kind, "originalDocumentNo": item.OriginalDocumentNo, "status": item.Status,
		"amountCents": item.AmountCents, "currency": item.Currency, "invoiceType": item.InvoiceType,
		"invoiceTitle": item.InvoiceTitle, "taxIdentifier": item.TaxIdentifier,
		"provider": item.Provider, "providerDocumentNo": item.ProviderDocumentNo,
		"documentUrl": item.DocumentURL, "version": item.Version,
	}
}
