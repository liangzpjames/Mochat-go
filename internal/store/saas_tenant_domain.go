package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) SaaSTenantDomainByHostname(ctx context.Context, hostname string) (dashboard.SaaSTenantDomain, bool, error) {
	hostname, err := dashboard.NormalizeSaaSTenantDomainHostname(hostname)
	if err != nil {
		return dashboard.SaaSTenantDomain{}, false, nil
	}
	domain, err := scanSaaSTenantDomain(s.db.QueryRowContext(ctx, saasTenantDomainSelect+`
		WHERE d.hostname_active = ? AND d.deleted_at IS NULL LIMIT 1`, hostname))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSTenantDomain{}, false, nil
	}
	return domain, err == nil, err
}

func (s *MySQLStore) SaaSAdminTenantDomains(ctx context.Context, options dashboard.SaaSAdminTenantDomainOptions) ([]dashboard.SaaSTenantDomain, error) {
	if options.Limit <= 0 || options.Limit > 100 {
		options.Limit = 100
	}
	where := []string{"d.deleted_at IS NULL", "t.deleted_at IS NULL"}
	args := make([]any, 0, 8)
	if options.TenantID > 0 {
		where = append(where, "d.tenant_id = ?")
		args = append(args, options.TenantID)
	}
	if options.Status != "" && options.Status != "all" {
		where = append(where, "d.status = ?")
		args = append(args, options.Status)
	}
	if options.Keyword != "" {
		like := "%" + options.Keyword + "%"
		where = append(where, "(d.hostname LIKE ? OR t.name LIKE ? OR CAST(d.tenant_id AS CHAR) = ?)")
		args = append(args, like, like, options.Keyword)
	}
	args = append(args, options.Limit)
	rows, err := s.db.QueryContext(ctx, saasTenantDomainSelect+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY d.is_primary DESC, d.tenant_id ASC, d.id ASC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSTenantDomain, 0)
	for rows.Next() {
		item, err := scanSaaSTenantDomain(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) SaaSAdminTenantDomain(ctx context.Context, id int64) (dashboard.SaaSTenantDomain, error) {
	if id <= 0 {
		return dashboard.SaaSTenantDomain{}, dashboard.NewSaaSAdminBadRequest("id 无效")
	}
	item, found, err := saasTenantDomainRow(ctx, s.db, id, false, false)
	if err != nil {
		return dashboard.SaaSTenantDomain{}, err
	}
	if !found {
		return dashboard.SaaSTenantDomain{}, dashboard.NewSaaSAdminNotFound("租户域名不存在")
	}
	return item, nil
}

func (s *MySQLStore) SaaSAdminTenantDomainCreateTarget(ctx context.Context, tenantID int, hostname string) (dashboard.SaaSAdminTenantDomainCreateTarget, error) {
	if tenantID <= 0 {
		return dashboard.SaaSAdminTenantDomainCreateTarget{}, dashboard.NewSaaSAdminBadRequest("tenantId 必填")
	}
	normalized, err := dashboard.NormalizeSaaSTenantDomainHostname(hostname)
	if err != nil {
		return dashboard.SaaSAdminTenantDomainCreateTarget{}, dashboard.NewSaaSAdminBadRequest(err.Error())
	}
	target := dashboard.SaaSAdminTenantDomainCreateTarget{TenantID: tenantID, Hostname: normalized}
	err = s.db.QueryRowContext(ctx, `
		SELECT name, status
		FROM mc_tenant
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, tenantID).Scan(&target.TenantName, &target.TenantStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminTenantDomainCreateTarget{}, dashboard.NewSaaSAdminNotFound("租户不存在")
	}
	if err != nil {
		return dashboard.SaaSAdminTenantDomainCreateTarget{}, err
	}
	if target.TenantStatus != 1 {
		return dashboard.SaaSAdminTenantDomainCreateTarget{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "只能为正常租户添加域名"}
	}
	var existing int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mochat_go_saas_tenant_domains
		WHERE hostname_active = ? AND deleted_at IS NULL
	`, normalized).Scan(&existing); err != nil {
		return dashboard.SaaSAdminTenantDomainCreateTarget{}, err
	}
	if existing > 0 {
		return dashboard.SaaSAdminTenantDomainCreateTarget{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "该域名已被其他租户绑定"}
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mochat_go_saas_tenant_domains
		WHERE tenant_id = ? AND deleted_at IS NULL
	`, tenantID).Scan(&target.DomainCount); err != nil {
		return dashboard.SaaSAdminTenantDomainCreateTarget{}, err
	}
	if target.DomainCount >= dashboard.SaaSTenantDomainMaxPerTenant {
		return dashboard.SaaSAdminTenantDomainCreateTarget{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "单个租户最多绑定 10 个域名"}
	}
	return target, nil
}

func (s *MySQLStore) CreateSaaSAdminTenantDomain(ctx context.Context, input dashboard.SaaSAdminTenantDomainCreate) (dashboard.SaaSAdminTenantDomainResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	defer tx.Rollback()
	var tenantName string
	var tenantStatus int
	if err := tx.QueryRowContext(ctx, `SELECT name, status FROM mc_tenant WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, input.TenantID).Scan(&tenantName, &tenantStatus); errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminTenantDomainResult{}, dashboard.NewSaaSAdminNotFound("租户不存在")
	} else if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	if tenantStatus != 1 {
		return dashboard.SaaSAdminTenantDomainResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "只能为正常租户添加域名"}
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_saas_tenant_domains WHERE tenant_id = ? AND deleted_at IS NULL`, input.TenantID).Scan(&count); err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	if count >= dashboard.SaaSTenantDomainMaxPerTenant {
		return dashboard.SaaSAdminTenantDomainResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "单个租户最多绑定 10 个域名"}
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_tenant_domains
			(tenant_id, hostname, hostname_active, status, is_primary, primary_slot, verification_method,
			 verification_token, verification_error, version, created_by, updated_by, created_at, updated_at)
		VALUES (?, ?, ?, 'pending', 0, NULL, 'dns_txt', ?, '', 1, ?, ?, NOW(), NOW())
	`, input.TenantID, input.Hostname, input.Hostname, input.Token, input.ActorUserID, input.ActorUserID)
	if isMySQLDuplicateKeyError(err) {
		return dashboard.SaaSAdminTenantDomainResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "该域名已被其他租户绑定"}
	}
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	after, found, err := saasTenantDomainRow(ctx, tx, id, false, false)
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, fmt.Errorf("read created tenant domain %d: %w", id, err)
	}
	if !found {
		return dashboard.SaaSAdminTenantDomainResult{}, fmt.Errorf("created tenant domain %d was not persisted", id)
	}
	after.TenantName = tenantName
	if err := ensureSaaSTenantDomainDeliveryTx(ctx, tx, after); err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	after.Delivery, err = saasTenantDomainDeliveryByDomainID(ctx, tx, after.ID, false)
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	operationID, err := insertSaaSTenantDomainOperation(ctx, tx, dashboard.SaaSTenantDomainActionCreate, dashboard.SaaSTenantDomain{}, after, input.ActorUserID, input.ActorTenantID, "create tenant custom domain")
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.ActorUserID, operationID); err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	return dashboard.SaaSAdminTenantDomainResult{Domain: after, OperationID: operationID}, nil
}

func (s *MySQLStore) CompleteSaaSAdminTenantDomainVerification(ctx context.Context, input dashboard.SaaSAdminTenantDomainVerification) (dashboard.SaaSAdminTenantDomainResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	defer tx.Rollback()
	if _, err := lockSaaSTenantDomainTenant(ctx, tx, input.ID); err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	before, found, err := saasTenantDomainRow(ctx, tx, input.ID, false, true)
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	if !found {
		return dashboard.SaaSAdminTenantDomainResult{}, dashboard.NewSaaSAdminNotFound("租户域名不存在")
	}
	if before.Version != input.ExpectedVersion {
		return dashboard.SaaSAdminTenantDomainResult{}, tenantDomainVersionConflict()
	}
	if before.Status != dashboard.SaaSTenantDomainStatusPending {
		return dashboard.SaaSAdminTenantDomainResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "只有待验证域名可以执行 DNS TXT 校验"}
	}
	if err := lockSaaSTenantDomains(ctx, tx, before.TenantID); err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	action := dashboard.SaaSTenantDomainActionVerify
	if input.Success {
		primary := before.IsPrimary
		if !primary {
			hasPrimary, err := tenantHasPrimaryDomain(ctx, tx, before.TenantID, before.ID)
			if err != nil {
				return dashboard.SaaSAdminTenantDomainResult{}, err
			}
			primary = !hasPrimary
		}
		primarySlot := any(nil)
		if primary {
			primarySlot = before.TenantID
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_tenant_domains
			SET status = 'active', is_primary = ?, primary_slot = ?, verification_error = '',
				last_verification_at = NOW(), verified_at = NOW(), version = version + 1,
				updated_by = ?, updated_at = NOW()
			WHERE id = ? AND version = ? AND deleted_at IS NULL
		`, tenantDomainBoolInt(primary), primarySlot, input.ActorUserID, before.ID, before.Version)
	} else {
		action = "verify_failed"
		_, err = tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_tenant_domains
			SET status = 'pending', is_primary = 0, primary_slot = NULL, verification_error = ?,
				last_verification_at = NOW(), verified_at = NULL, version = version + 1,
				updated_by = ?, updated_at = NOW()
			WHERE id = ? AND version = ? AND deleted_at IS NULL
		`, truncateSaaSTenantDomainError(input.ErrorMessage), input.ActorUserID, before.ID, before.Version)
		if err == nil && before.IsPrimary {
			err = assignFallbackSaaSTenantDomain(ctx, tx, before.TenantID, before.ID, input.ActorUserID)
		}
	}
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	after, found, err := saasTenantDomainRow(ctx, tx, before.ID, false, false)
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, fmt.Errorf("read verified tenant domain %d: %w", before.ID, err)
	}
	if !found {
		return dashboard.SaaSAdminTenantDomainResult{}, fmt.Errorf("verified tenant domain %d was not persisted", before.ID)
	}
	var deliveryJob dashboard.SaaSTenantDomainDeliveryJob
	if input.Success {
		deliveryJob, _, err = enqueueSaaSTenantDomainDeliveryTx(ctx, tx, after, dashboard.SaaSTenantDomainDeliveryActionProvision, dashboard.SaaSTenantDomainDeliveryDefaultMaxAttempts, input.ActorUserID, input.ActorTenantID)
		if err != nil {
			return dashboard.SaaSAdminTenantDomainResult{}, err
		}
		after.Delivery, err = saasTenantDomainDeliveryByDomainID(ctx, tx, after.ID, false)
		if err != nil {
			return dashboard.SaaSAdminTenantDomainResult{}, err
		}
	}
	operationID, err := insertSaaSTenantDomainOperation(ctx, tx, action, before, after, input.ActorUserID, input.ActorTenantID, "verify tenant domain ownership with DNS TXT")
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	if deliveryJob.ID > 0 && deliveryJob.OperationID == 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domain_delivery_jobs SET operation_id = ?, updated_at = NOW() WHERE id = ?`, operationID, deliveryJob.ID); err != nil {
			return dashboard.SaaSAdminTenantDomainResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	return dashboard.SaaSAdminTenantDomainResult{Domain: after, OperationID: operationID}, nil
}

func (s *MySQLStore) ApplySaaSAdminTenantDomainCommand(ctx context.Context, input dashboard.SaaSAdminTenantDomainCommand) (dashboard.SaaSAdminTenantDomainResult, error) {
	approvedExecution := input.ApprovalExecutionID > 0
	if approvedExecution != (input.ApprovalPlan != nil) {
		return dashboard.SaaSAdminTenantDomainResult{}, dashboard.NewSaaSAdminBadRequest("域名审批执行引用不完整")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	defer tx.Rollback()
	if _, err := lockSaaSTenantDomainTenant(ctx, tx, input.ID); err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	before, found, err := saasTenantDomainRow(ctx, tx, input.ID, false, true)
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	if !found {
		return dashboard.SaaSAdminTenantDomainResult{}, dashboard.NewSaaSAdminNotFound("租户域名不存在")
	}
	if before.Version != input.ExpectedVersion {
		return dashboard.SaaSAdminTenantDomainResult{}, tenantDomainVersionConflict()
	}
	if err := lockSaaSTenantDomains(ctx, tx, before.TenantID); err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	if approvedExecution {
		plan := input.ApprovalPlan
		if plan.SchemaVersion != dashboard.SaaSTenantDomainApprovalPlanSchemaVersion ||
			plan.Command.ID != input.ID || plan.Command.Action != input.Action || plan.Command.ExpectedVersion != input.ExpectedVersion ||
			plan.Domain.ID != input.ID || plan.Domain.TenantID != before.TenantID || len(plan.RoutingSHA256) != sha256.Size*2 {
			return dashboard.SaaSAdminTenantDomainResult{}, dashboard.NewSaaSAdminBadRequest("域名审批请求载荷损坏")
		}
		expectedDigest, err := saasTenantDomainRoutingSHA256(plan.RoutingDomains)
		if err != nil || expectedDigest != plan.RoutingSHA256 {
			return dashboard.SaaSAdminTenantDomainResult{}, dashboard.NewSaaSAdminBadRequest("域名审批路由快照损坏")
		}
		domains, err := saasTenantDomainsForTenantTx(ctx, tx, before.TenantID)
		if err != nil {
			return dashboard.SaaSAdminTenantDomainResult{}, err
		}
		currentPlan, err := newSaaSTenantDomainCommandApprovalPlan(before, domains, input.Action)
		if err != nil {
			return dashboard.SaaSAdminTenantDomainResult{}, err
		}
		if plan.Domain != currentPlan.Domain || plan.RoutingSHA256 != currentPlan.RoutingSHA256 {
			return dashboard.SaaSAdminTenantDomainResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户域名路由快照已变化，请重新发起审批"}
		}
	}
	if err := validateSaaSTenantDomainCommandState(before, input.Action); err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	switch input.Action {
	case dashboard.SaaSTenantDomainActionSetPrimary:
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domains SET is_primary = 0, primary_slot = NULL, version = version + 1, updated_by = ?, updated_at = NOW() WHERE tenant_id = ? AND id <> ? AND is_primary = 1 AND deleted_at IS NULL`, input.ActorUserID, before.TenantID, before.ID); err != nil {
			return dashboard.SaaSAdminTenantDomainResult{}, err
		}
		if !before.IsPrimary {
			_, err = tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domains SET is_primary = 1, primary_slot = tenant_id, version = version + 1, updated_by = ?, updated_at = NOW() WHERE id = ? AND version = ? AND deleted_at IS NULL`, input.ActorUserID, before.ID, before.Version)
		}
	case dashboard.SaaSTenantDomainActionDisable:
		_, err = tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domains SET status = 'disabled', is_primary = 0, primary_slot = NULL, version = version + 1, updated_by = ?, updated_at = NOW() WHERE id = ? AND version = ? AND deleted_at IS NULL`, input.ActorUserID, before.ID, before.Version)
		if err == nil && before.IsPrimary {
			err = assignFallbackSaaSTenantDomain(ctx, tx, before.TenantID, before.ID, input.ActorUserID)
		}
	case dashboard.SaaSTenantDomainActionEnable:
		hasPrimary, checkErr := tenantHasPrimaryDomain(ctx, tx, before.TenantID, before.ID)
		if checkErr != nil {
			return dashboard.SaaSAdminTenantDomainResult{}, checkErr
		}
		primary := before.IsPrimary || !hasPrimary
		primarySlot := any(nil)
		if primary {
			primarySlot = before.TenantID
		}
		_, err = tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domains SET status = 'active', is_primary = ?, primary_slot = ?, version = version + 1, updated_by = ?, updated_at = NOW() WHERE id = ? AND version = ? AND deleted_at IS NULL`, tenantDomainBoolInt(primary), primarySlot, input.ActorUserID, before.ID, before.Version)
	case dashboard.SaaSTenantDomainActionRotateToken:
		if strings.TrimSpace(input.Token) == "" {
			return dashboard.SaaSAdminTenantDomainResult{}, dashboard.NewSaaSAdminBadRequest("新的 DNS 校验令牌不能为空")
		}
		_, err = tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domains SET status = 'pending', is_primary = 0, primary_slot = NULL, verification_token = ?, verification_error = '', last_verification_at = NULL, verified_at = NULL, version = version + 1, updated_by = ?, updated_at = NOW() WHERE id = ? AND version = ? AND deleted_at IS NULL`, input.Token, input.ActorUserID, before.ID, before.Version)
		if err == nil && before.IsPrimary {
			err = assignFallbackSaaSTenantDomain(ctx, tx, before.TenantID, before.ID, input.ActorUserID)
		}
	case dashboard.SaaSTenantDomainActionDelete:
		_, err = tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domains SET hostname_active = NULL, is_primary = 0, primary_slot = NULL, deleted_at = NOW(), version = version + 1, updated_by = ?, updated_at = NOW() WHERE id = ? AND version = ? AND deleted_at IS NULL`, input.ActorUserID, before.ID, before.Version)
	default:
		return dashboard.SaaSAdminTenantDomainResult{}, dashboard.NewSaaSAdminBadRequest("域名操作无效")
	}
	if isMySQLDuplicateKeyError(err) {
		return dashboard.SaaSAdminTenantDomainResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "域名主记录已变化，请刷新后重试"}
	}
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	after, found, err := saasTenantDomainRow(ctx, tx, before.ID, input.Action == dashboard.SaaSTenantDomainActionDelete, false)
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, fmt.Errorf("read updated tenant domain %d: %w", before.ID, err)
	}
	if !found {
		return dashboard.SaaSAdminTenantDomainResult{}, fmt.Errorf("updated tenant domain %d was not persisted", before.ID)
	}
	if input.Action == dashboard.SaaSTenantDomainActionDelete {
		after.Status = dashboard.SaaSTenantDomainStatusDeleted
	}
	deliveryAction := ""
	switch input.Action {
	case dashboard.SaaSTenantDomainActionSetPrimary:
		deliveryAction = dashboard.SaaSTenantDomainDeliveryActionRefresh
	case dashboard.SaaSTenantDomainActionDisable, dashboard.SaaSTenantDomainActionRotateToken:
		deliveryAction = dashboard.SaaSTenantDomainDeliveryActionDisable
	case dashboard.SaaSTenantDomainActionEnable:
		deliveryAction = dashboard.SaaSTenantDomainDeliveryActionProvision
	case dashboard.SaaSTenantDomainActionDelete:
		deliveryAction = dashboard.SaaSTenantDomainDeliveryActionDelete
	}
	var deliveryJob dashboard.SaaSTenantDomainDeliveryJob
	if deliveryAction != "" {
		deliveryJob, _, err = enqueueSaaSTenantDomainDeliveryTx(ctx, tx, after, deliveryAction, dashboard.SaaSTenantDomainDeliveryDefaultMaxAttempts, input.ActorUserID, input.ActorTenantID)
		if err != nil {
			return dashboard.SaaSAdminTenantDomainResult{}, err
		}
		after.Delivery, err = saasTenantDomainDeliveryByDomainID(ctx, tx, after.ID, false)
		if err != nil {
			return dashboard.SaaSAdminTenantDomainResult{}, err
		}
	}
	operationID, err := insertSaaSTenantDomainOperation(ctx, tx, input.Action, before, after, input.ActorUserID, input.ActorTenantID, "apply tenant domain command")
	if err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	if deliveryJob.ID > 0 && deliveryJob.OperationID == 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domain_delivery_jobs SET operation_id = ?, updated_at = NOW() WHERE id = ?`, operationID, deliveryJob.ID); err != nil {
			return dashboard.SaaSAdminTenantDomainResult{}, err
		}
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.ActorUserID, operationID); err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminTenantDomainResult{}, err
	}
	return dashboard.SaaSAdminTenantDomainResult{Domain: after, OperationID: operationID}, nil
}

func (s *MySQLStore) PlanSaaSAdminTenantDomainCommand(ctx context.Context, request dashboard.SaaSAdminTenantDomainRequest) (dashboard.SaaSTenantDomainCommandApprovalPlan, error) {
	if !dashboard.SaaSTenantDomainCommandRequiresApproval(request.Action) || request.ID <= 0 || request.ExpectedVersion <= 0 {
		return dashboard.SaaSTenantDomainCommandApprovalPlan{}, dashboard.NewSaaSAdminBadRequest("域名审批动作无效")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSTenantDomainCommandApprovalPlan{}, err
	}
	defer tx.Rollback()
	if _, err := lockSaaSTenantDomainTenant(ctx, tx, request.ID); err != nil {
		return dashboard.SaaSTenantDomainCommandApprovalPlan{}, err
	}
	before, found, err := saasTenantDomainRow(ctx, tx, request.ID, false, true)
	if err != nil {
		return dashboard.SaaSTenantDomainCommandApprovalPlan{}, err
	}
	if !found {
		return dashboard.SaaSTenantDomainCommandApprovalPlan{}, dashboard.NewSaaSAdminNotFound("租户域名不存在")
	}
	if before.Version != request.ExpectedVersion {
		return dashboard.SaaSTenantDomainCommandApprovalPlan{}, tenantDomainVersionConflict()
	}
	if err := lockSaaSTenantDomains(ctx, tx, before.TenantID); err != nil {
		return dashboard.SaaSTenantDomainCommandApprovalPlan{}, err
	}
	if err := validateSaaSTenantDomainCommandState(before, request.Action); err != nil {
		return dashboard.SaaSTenantDomainCommandApprovalPlan{}, err
	}
	domains, err := saasTenantDomainsForTenantTx(ctx, tx, before.TenantID)
	if err != nil {
		return dashboard.SaaSTenantDomainCommandApprovalPlan{}, err
	}
	plan, err := newSaaSTenantDomainCommandApprovalPlan(before, domains, request.Action)
	if err != nil {
		return dashboard.SaaSTenantDomainCommandApprovalPlan{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSTenantDomainCommandApprovalPlan{}, err
	}
	return plan, nil
}

func validateSaaSTenantDomainCommandState(domain dashboard.SaaSTenantDomain, action string) error {
	switch action {
	case dashboard.SaaSTenantDomainActionSetPrimary:
		if !domain.RoutingActive() {
			return &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "只有已验证且启用的域名可以设为主域名"}
		}
		if domain.IsPrimary {
			return &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "该域名已经是主域名"}
		}
	case dashboard.SaaSTenantDomainActionDisable:
		if domain.Status != dashboard.SaaSTenantDomainStatusActive {
			return &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "只有启用中的域名可以停用"}
		}
	case dashboard.SaaSTenantDomainActionEnable:
		if domain.Status != dashboard.SaaSTenantDomainStatusDisabled {
			return &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "只有已停用域名可以重新启用"}
		}
		if domain.VerifiedAt == "" {
			return &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "域名尚未通过 DNS TXT 校验"}
		}
	case dashboard.SaaSTenantDomainActionRotateToken:
	case dashboard.SaaSTenantDomainActionDelete:
		if domain.Status == dashboard.SaaSTenantDomainStatusActive || domain.IsPrimary {
			return &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "启用中的域名必须先停用才能删除"}
		}
	default:
		return dashboard.NewSaaSAdminBadRequest("域名操作无效")
	}
	return nil
}

func saasTenantDomainsForTenantTx(ctx context.Context, tx *sql.Tx, tenantID int) ([]dashboard.SaaSTenantDomain, error) {
	rows, err := tx.QueryContext(ctx, saasTenantDomainSelect+` WHERE d.tenant_id = ? AND d.deleted_at IS NULL ORDER BY d.id ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSTenantDomain, 0, dashboard.SaaSTenantDomainMaxPerTenant)
	for rows.Next() {
		item, err := scanSaaSTenantDomain(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func newSaaSTenantDomainCommandApprovalPlan(domain dashboard.SaaSTenantDomain, domains []dashboard.SaaSTenantDomain, action string) (dashboard.SaaSTenantDomainCommandApprovalPlan, error) {
	routing := make([]dashboard.SaaSTenantDomainApprovalSnapshot, 0, len(domains))
	for _, item := range domains {
		routing = append(routing, saasTenantDomainApprovalSnapshot(item))
	}
	digest, err := saasTenantDomainRoutingSHA256(routing)
	if err != nil {
		return dashboard.SaaSTenantDomainCommandApprovalPlan{}, err
	}
	return dashboard.SaaSTenantDomainCommandApprovalPlan{
		SchemaVersion: dashboard.SaaSTenantDomainApprovalPlanSchemaVersion,
		Command: dashboard.SaaSAdminTenantDomainCommand{
			ID: domain.ID, Action: action, ExpectedVersion: domain.Version,
		},
		Domain:         saasTenantDomainApprovalSnapshot(domain),
		RoutingDomains: routing,
		RoutingSHA256:  digest,
	}, nil
}

func saasTenantDomainApprovalSnapshot(domain dashboard.SaaSTenantDomain) dashboard.SaaSTenantDomainApprovalSnapshot {
	return dashboard.SaaSTenantDomainApprovalSnapshot{
		ID: domain.ID, TenantID: domain.TenantID, Hostname: domain.Hostname, Status: domain.Status,
		IsPrimary: domain.IsPrimary, VerifiedAt: domain.VerifiedAt, Version: domain.Version,
	}
}

func saasTenantDomainRoutingSHA256(items []dashboard.SaaSTenantDomainApprovalSnapshot) (string, error) {
	encoded, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

const saasTenantDomainSelect = `
	SELECT d.id, d.tenant_id, COALESCE(t.name, ''), d.hostname, d.status, d.is_primary,
		d.verification_method, d.verification_token, d.verification_error,
		d.last_verification_at, d.verified_at, d.version, d.created_by, d.updated_by,
		d.created_at, d.updated_at, d.deleted_at,
		v.id, v.domain_id, v.tenant_id, v.delivery_status, v.routing_status, v.certificate_status,
		v.provider, v.provider_request_id, v.certificate_id, v.certificate_not_before, v.certificate_expires_at,
		v.last_event_at, v.last_reconciled_at, v.last_error, v.version, v.created_at, v.updated_at
	FROM mochat_go_saas_tenant_domains d
	LEFT JOIN mc_tenant t ON t.id = d.tenant_id
	LEFT JOIN mochat_go_saas_tenant_domain_deliveries v ON v.domain_id = d.id`

type saasTenantDomainScanner interface {
	Scan(...any) error
}

func scanSaaSTenantDomain(scanner saasTenantDomainScanner) (dashboard.SaaSTenantDomain, error) {
	var item dashboard.SaaSTenantDomain
	var primary int
	var lastVerificationAt, verifiedAt, createdAt, updatedAt, deletedAt sql.NullTime
	var deliveryID, deliveryDomainID sql.NullInt64
	var deliveryTenantID, deliveryVersion sql.NullInt64
	var deliveryStatus, routingStatus, certificateStatus sql.NullString
	var deliveryProvider, deliveryRequestID, certificateID, deliveryError sql.NullString
	var certificateNotBefore, certificateExpiresAt, lastEventAt, lastReconciledAt, deliveryCreatedAt, deliveryUpdatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.TenantID, &item.TenantName, &item.Hostname, &item.Status, &primary,
		&item.VerificationMethod, &item.VerificationToken, &item.VerificationError,
		&lastVerificationAt, &verifiedAt, &item.Version, &item.CreatedBy, &item.UpdatedBy,
		&createdAt, &updatedAt, &deletedAt,
		&deliveryID, &deliveryDomainID, &deliveryTenantID, &deliveryStatus, &routingStatus, &certificateStatus,
		&deliveryProvider, &deliveryRequestID, &certificateID, &certificateNotBefore, &certificateExpiresAt,
		&lastEventAt, &lastReconciledAt, &deliveryError, &deliveryVersion, &deliveryCreatedAt, &deliveryUpdatedAt)
	item.IsPrimary = primary == 1
	item.LastVerificationAt, item.VerifiedAt = formatTime(lastVerificationAt), formatTime(verifiedAt)
	item.CreatedAt, item.UpdatedAt, item.DeletedAt = formatTime(createdAt), formatTime(updatedAt), formatTime(deletedAt)
	if deliveryID.Valid {
		item.Delivery = dashboard.SaaSTenantDomainDelivery{
			ID: deliveryID.Int64, DomainID: deliveryDomainID.Int64, TenantID: int(deliveryTenantID.Int64),
			DeliveryStatus: deliveryStatus.String, RoutingStatus: routingStatus.String, CertificateStatus: certificateStatus.String,
			Provider: deliveryProvider.String, ProviderRequestID: deliveryRequestID.String, CertificateID: certificateID.String,
			CertificateNotBefore: formatTime(certificateNotBefore), CertificateExpiresAt: formatTime(certificateExpiresAt),
			LastEventAt: formatTime(lastEventAt), LastReconciledAt: formatTime(lastReconciledAt), LastError: deliveryError.String,
			Version: int(deliveryVersion.Int64), CreatedAt: formatTime(deliveryCreatedAt), UpdatedAt: formatTime(deliveryUpdatedAt),
		}
	}
	return item, err
}

func saasTenantDomainRow(ctx context.Context, queryer saasAdminAccessQueryer, id int64, includeDeleted bool, forUpdate bool) (dashboard.SaaSTenantDomain, bool, error) {
	query := saasTenantDomainSelect + ` WHERE d.id = ?`
	if !includeDeleted {
		query += ` AND d.deleted_at IS NULL`
	}
	if forUpdate {
		query += ` FOR UPDATE`
	}
	item, err := scanSaaSTenantDomain(queryer.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSTenantDomain{}, false, nil
	}
	return item, err == nil, err
}

func lockSaaSTenantDomains(ctx context.Context, tx *sql.Tx, tenantID int) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM mochat_go_saas_tenant_domains WHERE tenant_id = ? AND deleted_at IS NULL ORDER BY id FOR UPDATE`, tenantID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
	}
	return rows.Err()
}

func lockSaaSTenantDomainTenant(ctx context.Context, tx *sql.Tx, domainID int64) (int, error) {
	var tenantID int
	if err := tx.QueryRowContext(ctx, `SELECT tenant_id FROM mochat_go_saas_tenant_domains WHERE id = ? AND deleted_at IS NULL`, domainID).Scan(&tenantID); errors.Is(err, sql.ErrNoRows) {
		return 0, dashboard.NewSaaSAdminNotFound("租户域名不存在")
	} else if err != nil {
		return 0, err
	}
	var lockedTenantID int
	if err := tx.QueryRowContext(ctx, `SELECT id FROM mc_tenant WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, tenantID).Scan(&lockedTenantID); errors.Is(err, sql.ErrNoRows) {
		return 0, dashboard.NewSaaSAdminNotFound("租户不存在")
	} else if err != nil {
		return 0, err
	}
	return lockedTenantID, nil
}

func tenantHasPrimaryDomain(ctx context.Context, tx *sql.Tx, tenantID int, excludeID int64) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_saas_tenant_domains WHERE tenant_id = ? AND id <> ? AND is_primary = 1 AND status = 'active' AND verified_at IS NOT NULL AND deleted_at IS NULL`, tenantID, excludeID).Scan(&count)
	return count > 0, err
}

func assignFallbackSaaSTenantDomain(ctx context.Context, tx *sql.Tx, tenantID int, excludeID int64, actorUserID int) error {
	var id int64
	var version int
	err := tx.QueryRowContext(ctx, `SELECT id, version FROM mochat_go_saas_tenant_domains WHERE tenant_id = ? AND id <> ? AND status = 'active' AND verified_at IS NOT NULL AND deleted_at IS NULL ORDER BY verified_at ASC, id ASC LIMIT 1`, tenantID, excludeID).Scan(&id, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domains SET is_primary = 1, primary_slot = tenant_id, version = version + 1, updated_by = ?, updated_at = NOW() WHERE id = ? AND version = ? AND deleted_at IS NULL`, actorUserID, id, version)
	return err
}

func insertSaaSTenantDomainOperation(ctx context.Context, tx *sql.Tx, action string, before, after dashboard.SaaSTenantDomain, actorUserID, actorTenantID int, remark string) (int64, error) {
	tenantID := after.TenantID
	if tenantID == 0 {
		tenantID = before.TenantID
	}
	targetID := after.ID
	if targetID == 0 {
		targetID = before.ID
	}
	targetName := after.Hostname
	if targetName == "" {
		targetName = before.Hostname
	}
	beforeJSON, _ := json.Marshal(saasTenantDomainAuditPayload(before))
	afterJSON, _ := json.Marshal(saasTenantDomainAuditPayload(after))
	return insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: tenantID, ActorUserID: actorUserID, ActorTenantID: actorTenantID,
		Action: "saas.admin.tenant_domain." + action, TargetType: dashboard.SaaSAdminOperationTargetTenantDomain,
		TargetID: strconv.FormatInt(targetID, 10), TargetName: targetName,
		BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON), Remark: remark,
	})
}

func saasTenantDomainAuditPayload(item dashboard.SaaSTenantDomain) map[string]any {
	if item.ID == 0 {
		return map[string]any{}
	}
	return map[string]any{
		"id": item.ID, "tenantId": item.TenantID, "hostname": item.Hostname, "status": item.Status,
		"isPrimary": item.IsPrimary, "verificationMethod": item.VerificationMethod,
		"verificationRecordName": item.VerificationRecordName(), "verificationError": item.VerificationError,
		"lastVerificationAt": item.LastVerificationAt, "verifiedAt": item.VerifiedAt, "version": item.Version,
		"delivery": item.Delivery,
	}
}

func tenantDomainVersionConflict() error {
	return &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "域名版本已变化，请刷新后重试"}
}

func truncateSaaSTenantDomainError(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > 500 {
		value = string(runes[:500])
	}
	return value
}

func tenantDomainBoolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

var _ dashboard.SaaSAdminTenantDomainStore = (*MySQLStore)(nil)
