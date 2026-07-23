package store

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) PlanSaaSAdminTenantStatusUpdate(ctx context.Context, input dashboard.SaaSAdminTenantStatusUpdate) (dashboard.SaaSAdminTenantStatusApprovalPlan, error) {
	if input.TenantID <= 0 || (input.Status != 1 && input.Status != 2) {
		return dashboard.SaaSAdminTenantStatusApprovalPlan{}, dashboard.NewSaaSAdminBadRequest("租户状态审批输入无效")
	}
	expectedStatus := 1
	if input.Status == 1 {
		expectedStatus = 2
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return dashboard.SaaSAdminTenantStatusApprovalPlan{}, err
	}
	defer rollbackQuietly(tx)

	snapshot := dashboard.SaaSAdminTenantStatusApprovalSnapshot{TenantID: input.TenantID}
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(name, ''), COALESCE(status, 0)
		FROM mc_tenant
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, input.TenantID).Scan(&snapshot.TenantName, &snapshot.TenantStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminTenantStatusApprovalPlan{}, dashboard.NewSaaSAdminNotFound("tenant not found")
	}
	if err != nil {
		return dashboard.SaaSAdminTenantStatusApprovalPlan{}, err
	}
	if snapshot.TenantStatus != expectedStatus {
		message := "只能停用正常租户"
		if input.Status == 1 {
			message = "只能启用已停用租户"
		}
		return dashboard.SaaSAdminTenantStatusApprovalPlan{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: message}
	}
	err = tx.QueryRowContext(ctx, `
		SELECT id, status, version
		FROM mochat_go_saas_subscriptions
		WHERE tenant_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, input.TenantID).Scan(&snapshot.SubscriptionID, &snapshot.SubscriptionStatus, &snapshot.SubscriptionVersion)
	if err == nil {
		snapshot.SubscriptionPresent = true
	} else if !errors.Is(err, sql.ErrNoRows) && !isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminTenantStatusApprovalPlan{}, err
	}
	input.ExpectedStatus = expectedStatus
	plan := dashboard.SaaSAdminTenantStatusApprovalPlan{
		SchemaVersion: dashboard.SaaSAdminTenantStatusApprovalPlanSchemaVersion,
		Update:        input,
		Snapshot:      snapshot,
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminTenantStatusApprovalPlan{}, err
	}
	return plan, nil
}
