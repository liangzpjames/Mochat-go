package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) SaaSAdminTenantDomainDeliveryJobs(ctx context.Context, options dashboard.SaaSTenantDomainDeliveryJobOptions) ([]dashboard.SaaSTenantDomainDeliveryJob, error) {
	if options.Limit <= 0 || options.Limit > dashboard.SaaSTenantDomainDeliveryMaxListLimit {
		options.Limit = 100
	}
	where := []string{"1 = 1"}
	args := make([]any, 0, 4)
	if options.DomainID > 0 {
		where = append(where, "j.domain_id = ?")
		args = append(args, options.DomainID)
	}
	if options.TenantID > 0 {
		where = append(where, "j.tenant_id = ?")
		args = append(args, options.TenantID)
	}
	if options.Status != "" && options.Status != dashboard.SaaSTenantDomainDeliveryJobStatusAll {
		where = append(where, "j.status = ?")
		args = append(args, options.Status)
	}
	args = append(args, options.Limit)
	rows, err := s.db.QueryContext(ctx, saasTenantDomainDeliveryJobSelect+` WHERE `+strings.Join(where, " AND ")+` ORDER BY j.id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSTenantDomainDeliveryJob, 0)
	for rows.Next() {
		item, err := scanSaaSTenantDomainDeliveryJob(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) RequestSaaSTenantDomainDelivery(ctx context.Context, input dashboard.SaaSTenantDomainDeliveryRequest) (dashboard.SaaSTenantDomainDeliveryRequestResult, error) {
	if input.DomainID <= 0 || !tenantDomainDeliveryActionValid(input.Action) {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, dashboard.NewSaaSAdminBadRequest("域名交付请求无效")
	}
	if input.MaxAttempts <= 0 || input.MaxAttempts > 20 {
		input.MaxAttempts = dashboard.SaaSTenantDomainDeliveryDefaultMaxAttempts
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
	}
	defer tx.Rollback()
	domain, found, err := lockSaaSTenantDomainDeliveryDomain(ctx, tx, input.DomainID)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
	}
	if !found {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, dashboard.NewSaaSAdminNotFound("租户域名不存在")
	}
	if err := validateTenantDomainDeliveryAction(domain, input.Action); err != nil {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
	}
	before := domain.Delivery
	job, reused, err := enqueueSaaSTenantDomainDeliveryTx(ctx, tx, domain, input.Action, input.MaxAttempts, input.ActorUserID, input.ActorTenantID)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
	}
	after, err := saasTenantDomainDeliveryByDomainID(ctx, tx, domain.ID, false)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
	}
	operationID, err := insertSaaSTenantDomainDeliveryOperation(ctx, tx, "request", domain, before, after, job, input.ActorUserID, input.ActorTenantID, "request tenant domain route and TLS delivery")
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
	}
	if job.OperationID == 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domain_delivery_jobs SET operation_id = ?, updated_at = NOW() WHERE id = ?`, operationID, job.ID); err != nil {
			return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
		}
		job.OperationID = operationID
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
	}
	job.Delivery = after
	return dashboard.SaaSTenantDomainDeliveryRequestResult{Delivery: after, Job: job, OperationID: operationID, Reused: reused}, nil
}

func (s *MySQLStore) RetrySaaSTenantDomainDelivery(ctx context.Context, input dashboard.SaaSTenantDomainDeliveryRetry) (dashboard.SaaSTenantDomainDeliveryRequestResult, error) {
	if input.JobID <= 0 {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, dashboard.NewSaaSAdminBadRequest("jobId 无效")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
	}
	defer tx.Rollback()
	failed, found, err := saasTenantDomainDeliveryJobByID(ctx, tx, input.JobID, true)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
	}
	if !found {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, dashboard.NewSaaSAdminNotFound("域名交付任务不存在")
	}
	if failed.Status != dashboard.SaaSTenantDomainDeliveryJobStatusFailed {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "只有失败任务可以重试"}
	}
	domain, found, err := lockSaaSTenantDomainDeliveryDomain(ctx, tx, failed.DomainID)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
	}
	if !found {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, dashboard.NewSaaSAdminNotFound("租户域名不存在")
	}
	if err := validateTenantDomainDeliveryAction(domain, failed.Action); err != nil {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
	}
	before := domain.Delivery
	job, reused, err := enqueueSaaSTenantDomainDeliveryTx(ctx, tx, domain, failed.Action, failed.MaxAttempts, input.ActorUserID, input.ActorTenantID)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
	}
	after, err := saasTenantDomainDeliveryByDomainID(ctx, tx, domain.ID, false)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
	}
	operationID, err := insertSaaSTenantDomainDeliveryOperation(ctx, tx, "retry", domain, before, after, job, input.ActorUserID, input.ActorTenantID, "retry failed tenant domain delivery job "+failed.JobNo)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
	}
	if job.OperationID == 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domain_delivery_jobs SET operation_id = ?, updated_at = NOW() WHERE id = ?`, operationID, job.ID); err != nil {
			return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
		}
		job.OperationID = operationID
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSTenantDomainDeliveryRequestResult{}, err
	}
	job.Delivery = after
	return dashboard.SaaSTenantDomainDeliveryRequestResult{Delivery: after, Job: job, OperationID: operationID, Reused: reused}, nil
}

func (s *MySQLStore) ClaimSaaSTenantDomainDeliveryJobs(ctx context.Context, options dashboard.SaaSTenantDomainDeliveryClaimOptions) ([]dashboard.SaaSTenantDomainDeliveryJob, error) {
	if options.Limit <= 0 || options.Limit > 100 {
		options.Limit = 20
	}
	if options.LeaseDuration <= 0 {
		options.LeaseDuration = 2 * time.Minute
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_tenant_domain_deliveries d
		INNER JOIN mochat_go_saas_tenant_domain_delivery_jobs j ON j.domain_id = d.domain_id
		SET d.delivery_status = 'failed', d.last_error = '等待 Bridge 回调超时且已耗尽重试次数', d.version = d.version + 1, d.updated_at = NOW()
		WHERE j.status = 'waiting' AND j.active_domain_id IS NOT NULL AND j.attempts >= j.max_attempts
			AND COALESCE(j.next_attempt_at, j.updated_at) <= NOW()
	`); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_tenant_domain_delivery_jobs
		SET status = 'failed', active_domain_id = NULL, lease_expires_at = NULL,
			last_error = '等待 Bridge 回调超时且已耗尽重试次数', finished_at = NOW(), updated_at = NOW()
		WHERE status = 'waiting' AND active_domain_id IS NOT NULL AND attempts >= max_attempts
			AND COALESCE(next_attempt_at, updated_at) <= NOW()
	`); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM mochat_go_saas_tenant_domain_delivery_jobs
		WHERE active_domain_id IS NOT NULL AND attempts < max_attempts AND (
			(status = 'pending' AND COALESCE(next_attempt_at, created_at) <= NOW())
			OR (status = 'processing' AND COALESCE(lease_expires_at, updated_at) <= NOW())
			OR (status = 'waiting' AND COALESCE(next_attempt_at, updated_at) <= NOW())
		)
		ORDER BY id ASC LIMIT ? FOR UPDATE
	`, options.Limit)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, options.Limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return []dashboard.SaaSTenantDomainDeliveryJob{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+1)
	args = append(args, domainDeliveryDurationSeconds(options.LeaseDuration))
	for _, id := range ids {
		args = append(args, id)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domain_delivery_jobs SET status = 'processing', attempts = attempts + 1, lease_expires_at = DATE_ADD(NOW(), INTERVAL ? SECOND), started_at = COALESCE(started_at, NOW()), updated_at = NOW() WHERE id IN (`+placeholders+`)`, args...); err != nil {
		return nil, err
	}
	items := make([]dashboard.SaaSTenantDomainDeliveryJob, 0, len(ids))
	for _, id := range ids {
		item, found, err := saasTenantDomainDeliveryJobByID(ctx, tx, id, false)
		if err != nil {
			return nil, err
		}
		if found {
			items = append(items, item)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *MySQLStore) CompleteSaaSTenantDomainDeliveryJob(ctx context.Context, input dashboard.SaaSTenantDomainDeliveryCompletion) (dashboard.SaaSTenantDomainDeliveryJob, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		job, err := s.completeSaaSTenantDomainDeliveryJobOnce(ctx, input)
		if err == nil || !isMySQLRetryableTransactionError(err) {
			return job, err
		}
		lastErr = err
		if attempt == 2 {
			break
		}
		if err := waitForMySQLTransactionRetry(ctx, attempt); err != nil {
			return dashboard.SaaSTenantDomainDeliveryJob{}, err
		}
	}
	return dashboard.SaaSTenantDomainDeliveryJob{}, lastErr
}

func (s *MySQLStore) completeSaaSTenantDomainDeliveryJobOnce(ctx context.Context, input dashboard.SaaSTenantDomainDeliveryCompletion) (dashboard.SaaSTenantDomainDeliveryJob, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryJob{}, err
	}
	defer tx.Rollback()
	domainID, found, err := saasTenantDomainDeliveryJobDomainID(ctx, tx, input.JobID, input.JobNo)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryJob{}, err
	}
	if !found {
		return dashboard.SaaSTenantDomainDeliveryJob{}, dashboard.NewSaaSAdminNotFound("域名交付任务不存在")
	}
	if _, found, err := lockSaaSTenantDomainDeliveryDomain(ctx, tx, domainID); err != nil {
		return dashboard.SaaSTenantDomainDeliveryJob{}, err
	} else if !found {
		return dashboard.SaaSTenantDomainDeliveryJob{}, dashboard.NewSaaSAdminNotFound("租户域名不存在")
	}
	job, found, err := saasTenantDomainDeliveryJobByID(ctx, tx, input.JobID, true)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryJob{}, err
	}
	if !found || (input.JobNo != "" && job.JobNo != input.JobNo) {
		return dashboard.SaaSTenantDomainDeliveryJob{}, dashboard.NewSaaSAdminNotFound("域名交付任务不存在")
	}
	if job.Status != dashboard.SaaSTenantDomainDeliveryJobStatusProcessing {
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSTenantDomainDeliveryJob{}, err
		}
		return job, nil
	}
	if input.ExpectedAttempts > 0 && job.Attempts != input.ExpectedAttempts {
		return dashboard.SaaSTenantDomainDeliveryJob{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "域名交付任务租约已被新的执行取代"}
	}
	if err := completeSaaSTenantDomainDeliveryJobTx(ctx, tx, job, input); err != nil {
		return dashboard.SaaSTenantDomainDeliveryJob{}, err
	}
	updated, found, err := saasTenantDomainDeliveryJobByID(ctx, tx, job.ID, false)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryJob{}, err
	}
	if !found {
		return dashboard.SaaSTenantDomainDeliveryJob{}, errors.New("completed domain delivery job was not persisted")
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSTenantDomainDeliveryJob{}, err
	}
	return updated, nil
}

func (s *MySQLStore) ApplySaaSTenantDomainDeliveryCallback(ctx context.Context, event dashboard.SaaSTenantDomainDeliveryCallbackEvent) (dashboard.SaaSTenantDomainDeliveryCallbackResult, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		result, err := s.applySaaSTenantDomainDeliveryCallbackOnce(ctx, event)
		if err == nil || !isMySQLRetryableTransactionError(err) {
			return result, err
		}
		lastErr = err
		if attempt == 2 {
			break
		}
		if err := waitForMySQLTransactionRetry(ctx, attempt); err != nil {
			return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
		}
	}
	return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, lastErr
}

func (s *MySQLStore) applySaaSTenantDomainDeliveryCallbackOnce(ctx context.Context, event dashboard.SaaSTenantDomainDeliveryCallbackEvent) (dashboard.SaaSTenantDomainDeliveryCallbackResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
	}
	defer tx.Rollback()
	jobID, domainID, found, err := saasTenantDomainDeliveryJobIdentityByNo(ctx, tx, event.JobNo)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
	}
	if !found || domainID != event.DomainID {
		return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, dashboard.NewSaaSAdminNotFound("域名交付任务与回调对象不匹配")
	}
	if _, found, err := lockSaaSTenantDomainDeliveryDomain(ctx, tx, domainID); err != nil {
		return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
	} else if !found {
		return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, dashboard.NewSaaSAdminNotFound("租户域名不存在")
	}
	job, found, err := saasTenantDomainDeliveryJobByID(ctx, tx, jobID, true)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
	}
	if !found || job.DomainID != event.DomainID || job.Domain.Hostname != event.Hostname {
		return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, dashboard.NewSaaSAdminNotFound("域名交付任务与回调对象不匹配")
	}
	occurredAt, _ := dashboardParseDomainDeliveryTime(event.OccurredAt)
	result, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO mochat_go_saas_tenant_domain_delivery_events
			(event_id, job_no, domain_id, tenant_id, payload_sha256, signature_timestamp, event_type, result, occurred_at, applied_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'applied', ?, NOW(), NOW())
	`, event.EventID, event.JobNo, event.DomainID, job.TenantID, event.PayloadSHA256, event.SignatureTimestamp, event.EventType, occurredAt)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
	}
	if affected == 0 {
		var existingHash string
		if err := tx.QueryRowContext(ctx, `SELECT payload_sha256 FROM mochat_go_saas_tenant_domain_delivery_events WHERE event_id = ? FOR UPDATE`, event.EventID).Scan(&existingHash); err != nil {
			return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
		}
		if existingHash != event.PayloadSHA256 {
			return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "同一回调 eventId 的载荷不一致"}
		}
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
		}
		return dashboard.SaaSTenantDomainDeliveryCallbackResult{Duplicate: true, Delivery: job.Delivery, Job: job}, nil
	}
	if job.Status != dashboard.SaaSTenantDomainDeliveryJobStatusWaiting && job.Status != dashboard.SaaSTenantDomainDeliveryJobStatusProcessing {
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domain_delivery_events SET result = 'ignored' WHERE event_id = ?`, event.EventID); err != nil {
			return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
		}
		return dashboard.SaaSTenantDomainDeliveryCallbackResult{Ignored: true, Delivery: job.Delivery, Job: job}, nil
	}
	if job.Delivery.LastEventAt != "" {
		lastEventAt, parseErr := dashboardParseDomainDeliveryTime(job.Delivery.LastEventAt)
		if parseErr == nil && occurredAt.Before(lastEventAt) {
			if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domain_delivery_events SET result = 'ignored' WHERE event_id = ?`, event.EventID); err != nil {
				return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
			}
			if err := tx.Commit(); err != nil {
				return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
			}
			return dashboard.SaaSTenantDomainDeliveryCallbackResult{Ignored: true, Delivery: job.Delivery, Job: job}, nil
		}
	}
	update := dashboard.SaaSTenantDomainDeliveryBridgeUpdate{
		Status: event.Status, Provider: event.Provider, ProviderRequestID: event.ProviderRequestID,
		RoutingStatus: event.RoutingStatus, CertificateStatus: event.CertificateStatus,
		CertificateID: event.CertificateID, CertificateNotBefore: event.CertificateNotBefore,
		CertificateExpiresAt: event.CertificateExpiresAt, Error: event.Error, OccurredAt: event.OccurredAt,
	}
	if err := applySaaSTenantDomainDeliveryUpdateTx(ctx, tx, job, update, true); err != nil {
		return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
	}
	updated, found, err := saasTenantDomainDeliveryJobByID(ctx, tx, job.ID, false)
	if err != nil || !found {
		if err == nil {
			err = errors.New("callback domain delivery job was not persisted")
		}
		return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
	}
	_, err = insertSaaSTenantDomainDeliveryOperation(ctx, tx, "callback", updated.Domain, job.Delivery, updated.Delivery, updated, 0, 0, "signed delivery callback "+event.EventID)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSTenantDomainDeliveryCallbackResult{}, err
	}
	return dashboard.SaaSTenantDomainDeliveryCallbackResult{Delivery: updated.Delivery, Job: updated}, nil
}

func enqueueSaaSTenantDomainDeliveryTx(ctx context.Context, tx *sql.Tx, domain dashboard.SaaSTenantDomain, action string, maxAttempts, actorUserID, actorTenantID int) (dashboard.SaaSTenantDomainDeliveryJob, bool, error) {
	if maxAttempts <= 0 || maxAttempts > 20 {
		maxAttempts = dashboard.SaaSTenantDomainDeliveryDefaultMaxAttempts
	}
	if err := ensureSaaSTenantDomainDeliveryTx(ctx, tx, domain); err != nil {
		return dashboard.SaaSTenantDomainDeliveryJob{}, false, err
	}
	active, found, err := activeSaaSTenantDomainDeliveryJob(ctx, tx, domain.ID)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryJob{}, false, err
	}
	if found && active.Action == action {
		return active, true, nil
	}
	if found {
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domain_delivery_jobs SET status = 'canceled', active_domain_id = NULL, lease_expires_at = NULL, last_error = 'superseded by a newer domain lifecycle action', finished_at = NOW(), updated_at = NOW() WHERE id = ?`, active.ID); err != nil {
			return dashboard.SaaSTenantDomainDeliveryJob{}, false, err
		}
	}
	jobNo, err := newSaaSTenantDomainDeliveryJobNo()
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryJob{}, false, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_tenant_domain_delivery_jobs
			(job_no, domain_id, tenant_id, active_domain_id, action, status, attempts, max_attempts,
			 next_attempt_at, actor_user_id, actor_tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'pending', 0, ?, NOW(), ?, ?, NOW(), NOW())
	`, jobNo, domain.ID, domain.TenantID, domain.ID, action, maxAttempts, actorUserID, actorTenantID)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryJob{}, false, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryJob{}, false, err
	}
	deliveryStatus := dashboard.SaaSTenantDomainDeliveryStatusPending
	routingStatus := dashboard.SaaSTenantDomainRoutingStatusPending
	certificateStatus := dashboard.SaaSTenantDomainCertificateStatusPending
	if action == dashboard.SaaSTenantDomainDeliveryActionDisable {
		routingStatus = dashboard.SaaSTenantDomainRoutingStatusProvisioning
		certificateStatus = dashboard.SaaSTenantDomainCertificateStatusProvisioning
	}
	if action == dashboard.SaaSTenantDomainDeliveryActionDelete {
		routingStatus = dashboard.SaaSTenantDomainRoutingStatusProvisioning
		certificateStatus = dashboard.SaaSTenantDomainCertificateStatusProvisioning
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_tenant_domain_deliveries
		SET delivery_status = ?, routing_status = ?, certificate_status = ?, last_error = '', version = version + 1, updated_at = NOW()
		WHERE domain_id = ?
	`, deliveryStatus, routingStatus, certificateStatus, domain.ID); err != nil {
		return dashboard.SaaSTenantDomainDeliveryJob{}, false, err
	}
	job, found, err := saasTenantDomainDeliveryJobByID(ctx, tx, id, false)
	if err != nil {
		return dashboard.SaaSTenantDomainDeliveryJob{}, false, err
	}
	if !found {
		return dashboard.SaaSTenantDomainDeliveryJob{}, false, errors.New("created domain delivery job was not persisted")
	}
	return job, false, nil
}

func completeSaaSTenantDomainDeliveryJobTx(ctx context.Context, tx *sql.Tx, job dashboard.SaaSTenantDomainDeliveryJob, input dashboard.SaaSTenantDomainDeliveryCompletion) error {
	if input.RetryDelay <= 0 {
		input.RetryDelay = time.Minute
	}
	if input.CallbackWait <= 0 {
		input.CallbackWait = 10 * time.Minute
	}
	if input.CallError != "" {
		message := truncateDomainDeliveryStoreText(input.CallError, 500)
		if job.Attempts >= job.MaxAttempts {
			if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domain_delivery_jobs SET status = 'failed', active_domain_id = NULL, lease_expires_at = NULL, request_payload_sha256 = ?, last_error = ?, finished_at = NOW(), updated_at = NOW() WHERE id = ?`, input.RequestHash, message, job.ID); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domain_deliveries SET delivery_status = 'failed', last_error = ?, last_reconciled_at = NOW(), version = version + 1, updated_at = NOW() WHERE domain_id = ?`, message, job.DomainID)
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domain_delivery_jobs SET status = 'pending', lease_expires_at = NULL, next_attempt_at = DATE_ADD(NOW(), INTERVAL ? SECOND), request_payload_sha256 = ?, last_error = ?, updated_at = NOW() WHERE id = ?`, domainDeliveryDurationSeconds(input.RetryDelay), input.RequestHash, message, job.ID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domain_deliveries SET delivery_status = 'degraded', last_error = ?, last_reconciled_at = NOW(), version = version + 1, updated_at = NOW() WHERE domain_id = ?`, message, job.DomainID)
		return err
	}
	update, err := normalizeDomainDeliveryStoreUpdate(input.Update)
	if err != nil {
		return err
	}
	if update.Status == dashboard.SaaSTenantDomainDeliveryBridgeStatusFailed {
		return completeSaaSTenantDomainDeliveryJobTx(ctx, tx, job, dashboard.SaaSTenantDomainDeliveryCompletion{JobID: job.ID, JobNo: job.JobNo, RequestHash: input.RequestHash, CallError: update.Error, RetryDelay: input.RetryDelay, CallbackWait: input.CallbackWait})
	}
	if err := applySaaSTenantDomainDeliveryUpdateTx(ctx, tx, job, update, false); err != nil {
		return err
	}
	if update.Status == dashboard.SaaSTenantDomainDeliveryBridgeStatusReady || domainDeliveryUpdateTerminal(job.Action, update) {
		_, err = tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domain_delivery_jobs SET status = 'succeeded', active_domain_id = NULL, lease_expires_at = NULL, next_attempt_at = NULL, request_payload_sha256 = ?, provider = ?, provider_request_id = ?, last_error = '', finished_at = NOW(), updated_at = NOW() WHERE id = ?`, input.RequestHash, update.Provider, update.ProviderRequestID, job.ID)
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domain_delivery_jobs SET status = 'waiting', lease_expires_at = NULL, next_attempt_at = DATE_ADD(NOW(), INTERVAL ? SECOND), request_payload_sha256 = ?, provider = ?, provider_request_id = ?, last_error = '', updated_at = NOW() WHERE id = ?`, domainDeliveryDurationSeconds(input.CallbackWait), input.RequestHash, update.Provider, update.ProviderRequestID, job.ID)
	return err
}

func applySaaSTenantDomainDeliveryUpdateTx(ctx context.Context, tx *sql.Tx, job dashboard.SaaSTenantDomainDeliveryJob, update dashboard.SaaSTenantDomainDeliveryBridgeUpdate, callback bool) error {
	update, err := normalizeDomainDeliveryStoreUpdate(update)
	if err != nil {
		return err
	}
	deliveryStatus := domainDeliveryStoreStatus(update)
	notBefore, err := nullableDomainDeliveryTime(update.CertificateNotBefore)
	if err != nil {
		return err
	}
	expiresAt, err := nullableDomainDeliveryTime(update.CertificateExpiresAt)
	if err != nil {
		return err
	}
	eventAt := any(nil)
	if update.OccurredAt != "" {
		parsed, err := dashboardParseDomainDeliveryTime(update.OccurredAt)
		if err != nil {
			return err
		}
		eventAt = parsed
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_tenant_domain_deliveries
		SET delivery_status = ?, routing_status = ?, certificate_status = ?, provider = ?, provider_request_id = ?,
			certificate_id = ?, certificate_not_before = ?, certificate_expires_at = ?,
			last_event_at = IF(? IS NULL, last_event_at, ?), last_reconciled_at = NOW(), last_error = ?,
			version = version + 1, updated_at = NOW()
		WHERE domain_id = ?
	`, deliveryStatus, update.RoutingStatus, update.CertificateStatus, update.Provider, update.ProviderRequestID,
		update.CertificateID, notBefore, expiresAt, eventAt, eventAt, update.Error, job.DomainID)
	if err != nil {
		return err
	}
	if !callback {
		return nil
	}
	status := dashboard.SaaSTenantDomainDeliveryJobStatusWaiting
	activeDomainID := any(job.DomainID)
	finished := 0
	if update.Status == dashboard.SaaSTenantDomainDeliveryBridgeStatusReady || domainDeliveryUpdateTerminal(job.Action, update) {
		status = dashboard.SaaSTenantDomainDeliveryJobStatusSucceeded
		activeDomainID = nil
		finished = 1
	} else if update.Status == dashboard.SaaSTenantDomainDeliveryBridgeStatusFailed {
		status = dashboard.SaaSTenantDomainDeliveryJobStatusFailed
		activeDomainID = nil
		finished = 1
	}
	_, err = tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_domain_delivery_jobs SET status = ?, active_domain_id = ?, lease_expires_at = NULL, next_attempt_at = NULL, provider = ?, provider_request_id = ?, last_error = ?, finished_at = IF(? = 1, NOW(), NULL), updated_at = NOW() WHERE id = ?`, status, activeDomainID, update.Provider, update.ProviderRequestID, update.Error, finished, job.ID)
	return err
}

func ensureSaaSTenantDomainDeliveryTx(ctx context.Context, tx *sql.Tx, domain dashboard.SaaSTenantDomain) error {
	deliveryStatus := dashboard.SaaSTenantDomainDeliveryStatusUnconfigured
	routingStatus := dashboard.SaaSTenantDomainRoutingStatusPending
	certificateStatus := dashboard.SaaSTenantDomainCertificateStatusPending
	if domain.Status == dashboard.SaaSTenantDomainStatusDisabled {
		deliveryStatus = dashboard.SaaSTenantDomainDeliveryStatusDisabled
		routingStatus = dashboard.SaaSTenantDomainRoutingStatusDisabled
		certificateStatus = dashboard.SaaSTenantDomainCertificateStatusDisabled
	}
	if domain.Status == dashboard.SaaSTenantDomainStatusDeleted || domain.DeletedAt != "" {
		deliveryStatus = dashboard.SaaSTenantDomainDeliveryStatusDeleted
		routingStatus = dashboard.SaaSTenantDomainRoutingStatusDeleted
		certificateStatus = dashboard.SaaSTenantDomainCertificateStatusDeleted
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_tenant_domain_deliveries
			(domain_id, tenant_id, delivery_status, routing_status, certificate_status, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, NOW(), NOW())
		ON DUPLICATE KEY UPDATE tenant_id = VALUES(tenant_id), updated_at = NOW()
	`, domain.ID, domain.TenantID, deliveryStatus, routingStatus, certificateStatus)
	return err
}

func activeSaaSTenantDomainDeliveryJob(ctx context.Context, tx *sql.Tx, domainID int64) (dashboard.SaaSTenantDomainDeliveryJob, bool, error) {
	item, err := scanSaaSTenantDomainDeliveryJob(tx.QueryRowContext(ctx, saasTenantDomainDeliveryJobSelect+` WHERE j.active_domain_id = ? LIMIT 1 FOR UPDATE`, domainID))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSTenantDomainDeliveryJob{}, false, nil
	}
	return item, err == nil, err
}

func lockSaaSTenantDomainDeliveryDomain(ctx context.Context, tx *sql.Tx, domainID int64) (dashboard.SaaSTenantDomain, bool, error) {
	var tenantID int
	if err := tx.QueryRowContext(ctx, `SELECT tenant_id FROM mochat_go_saas_tenant_domains WHERE id = ?`, domainID).Scan(&tenantID); errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSTenantDomain{}, false, nil
	} else if err != nil {
		return dashboard.SaaSTenantDomain{}, false, err
	}
	var locked int
	if err := tx.QueryRowContext(ctx, `SELECT id FROM mc_tenant WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, tenantID).Scan(&locked); err != nil {
		return dashboard.SaaSTenantDomain{}, false, err
	}
	domain, found, err := saasTenantDomainRow(ctx, tx, domainID, true, true)
	return domain, found, err
}

func saasTenantDomainDeliveryJobDomainID(ctx context.Context, queryer saasAdminAccessQueryer, jobID int64, jobNo string) (int64, bool, error) {
	query := `SELECT domain_id FROM mochat_go_saas_tenant_domain_delivery_jobs WHERE id = ?`
	args := []any{jobID}
	if jobNo != "" {
		query += ` AND job_no = ?`
		args = append(args, jobNo)
	}
	var domainID int64
	err := queryer.QueryRowContext(ctx, query, args...).Scan(&domainID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return domainID, err == nil, err
}

func saasTenantDomainDeliveryJobIdentityByNo(ctx context.Context, queryer saasAdminAccessQueryer, jobNo string) (int64, int64, bool, error) {
	var jobID, domainID int64
	err := queryer.QueryRowContext(ctx, `SELECT id, domain_id FROM mochat_go_saas_tenant_domain_delivery_jobs WHERE job_no = ?`, jobNo).Scan(&jobID, &domainID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, false, nil
	}
	return jobID, domainID, err == nil, err
}

func waitForMySQLTransactionRetry(ctx context.Context, attempt int) error {
	timer := time.NewTimer(time.Duration(attempt+1) * 25 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func validateTenantDomainDeliveryAction(domain dashboard.SaaSTenantDomain, action string) error {
	switch action {
	case dashboard.SaaSTenantDomainDeliveryActionProvision, dashboard.SaaSTenantDomainDeliveryActionRefresh:
		if !domain.RoutingActive() {
			return &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "只有已验证且启用的域名可以下发路由和 TLS"}
		}
	case dashboard.SaaSTenantDomainDeliveryActionDisable:
		if domain.Status != dashboard.SaaSTenantDomainStatusDisabled && domain.Status != dashboard.SaaSTenantDomainStatusPending {
			return &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "域名尚未停用，不能下发停用动作"}
		}
	case dashboard.SaaSTenantDomainDeliveryActionDelete:
		if domain.DeletedAt == "" && domain.Status != dashboard.SaaSTenantDomainStatusDeleted {
			return &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "域名尚未删除，不能下发删除动作"}
		}
	default:
		return dashboard.NewSaaSAdminBadRequest("域名交付动作无效")
	}
	return nil
}

func tenantDomainDeliveryActionValid(value string) bool {
	switch value {
	case dashboard.SaaSTenantDomainDeliveryActionProvision, dashboard.SaaSTenantDomainDeliveryActionRefresh, dashboard.SaaSTenantDomainDeliveryActionDisable, dashboard.SaaSTenantDomainDeliveryActionDelete:
		return true
	default:
		return false
	}
}

func domainDeliveryUpdateTerminal(action string, update dashboard.SaaSTenantDomainDeliveryBridgeUpdate) bool {
	if action == dashboard.SaaSTenantDomainDeliveryActionDisable {
		return update.RoutingStatus == dashboard.SaaSTenantDomainRoutingStatusDisabled && (update.CertificateStatus == dashboard.SaaSTenantDomainCertificateStatusDisabled || update.CertificateStatus == dashboard.SaaSTenantDomainCertificateStatusRevoked)
	}
	if action == dashboard.SaaSTenantDomainDeliveryActionDelete {
		return update.RoutingStatus == dashboard.SaaSTenantDomainRoutingStatusDeleted && update.CertificateStatus == dashboard.SaaSTenantDomainCertificateStatusDeleted
	}
	return false
}

func saasTenantDomainDeliveryByDomainID(ctx context.Context, queryer saasAdminAccessQueryer, domainID int64, forUpdate bool) (dashboard.SaaSTenantDomainDelivery, error) {
	query := saasTenantDomainDeliverySelect + ` WHERE domain_id = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanSaaSTenantDomainDelivery(queryer.QueryRowContext(ctx, query, domainID))
}

func saasTenantDomainDeliveryJobByID(ctx context.Context, queryer saasAdminAccessQueryer, id int64, forUpdate bool) (dashboard.SaaSTenantDomainDeliveryJob, bool, error) {
	query := saasTenantDomainDeliveryJobSelect + ` WHERE j.id = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	item, err := scanSaaSTenantDomainDeliveryJob(queryer.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSTenantDomainDeliveryJob{}, false, nil
	}
	return item, err == nil, err
}

func saasTenantDomainDeliveryJobByNo(ctx context.Context, queryer saasAdminAccessQueryer, jobNo string, forUpdate bool) (dashboard.SaaSTenantDomainDeliveryJob, bool, error) {
	query := saasTenantDomainDeliveryJobSelect + ` WHERE j.job_no = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	item, err := scanSaaSTenantDomainDeliveryJob(queryer.QueryRowContext(ctx, query, jobNo))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSTenantDomainDeliveryJob{}, false, nil
	}
	return item, err == nil, err
}

const saasTenantDomainDeliverySelect = `
	SELECT id, domain_id, tenant_id, delivery_status, routing_status, certificate_status,
		provider, provider_request_id, certificate_id, certificate_not_before, certificate_expires_at,
		last_event_at, last_reconciled_at, last_error, version, created_at, updated_at
	FROM mochat_go_saas_tenant_domain_deliveries`

const saasTenantDomainDeliveryJobSelect = `
	SELECT j.id, j.job_no, j.domain_id, j.tenant_id, j.action, j.status, j.attempts, j.max_attempts,
		j.next_attempt_at, j.lease_expires_at, j.provider, j.provider_request_id, j.request_payload_sha256,
		j.last_error, j.actor_user_id, j.actor_tenant_id, j.operation_id, j.started_at, j.finished_at,
		j.created_at, j.updated_at,
		d.hostname, d.status, d.is_primary, d.verified_at, d.version, d.deleted_at, COALESCE(t.name, ''),
		v.id, v.domain_id, v.tenant_id, v.delivery_status, v.routing_status, v.certificate_status,
		v.provider, v.provider_request_id, v.certificate_id, v.certificate_not_before, v.certificate_expires_at,
		v.last_event_at, v.last_reconciled_at, v.last_error, v.version, v.created_at, v.updated_at
	FROM mochat_go_saas_tenant_domain_delivery_jobs j
	INNER JOIN mochat_go_saas_tenant_domains d ON d.id = j.domain_id
	LEFT JOIN mc_tenant t ON t.id = j.tenant_id
	INNER JOIN mochat_go_saas_tenant_domain_deliveries v ON v.domain_id = j.domain_id`

type tenantDomainDeliveryScanner interface {
	Scan(...any) error
}

func scanSaaSTenantDomainDelivery(scanner tenantDomainDeliveryScanner) (dashboard.SaaSTenantDomainDelivery, error) {
	var item dashboard.SaaSTenantDomainDelivery
	var notBefore, expiresAt, lastEventAt, lastReconciledAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.DomainID, &item.TenantID, &item.DeliveryStatus, &item.RoutingStatus, &item.CertificateStatus,
		&item.Provider, &item.ProviderRequestID, &item.CertificateID, &notBefore, &expiresAt,
		&lastEventAt, &lastReconciledAt, &item.LastError, &item.Version, &createdAt, &updatedAt)
	item.CertificateNotBefore, item.CertificateExpiresAt = formatTime(notBefore), formatTime(expiresAt)
	item.LastEventAt, item.LastReconciledAt = formatTime(lastEventAt), formatTime(lastReconciledAt)
	item.CreatedAt, item.UpdatedAt = formatTime(createdAt), formatTime(updatedAt)
	return item, err
}

func scanSaaSTenantDomainDeliveryJob(scanner tenantDomainDeliveryScanner) (dashboard.SaaSTenantDomainDeliveryJob, error) {
	var item dashboard.SaaSTenantDomainDeliveryJob
	var nextAttemptAt, leaseExpiresAt, startedAt, finishedAt, createdAt, updatedAt sql.NullTime
	var domainPrimary int
	var domainVerifiedAt, domainDeletedAt sql.NullTime
	var deliveryNotBefore, deliveryExpiresAt, deliveryLastEventAt, deliveryLastReconciledAt, deliveryCreatedAt, deliveryUpdatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.JobNo, &item.DomainID, &item.TenantID, &item.Action, &item.Status, &item.Attempts, &item.MaxAttempts,
		&nextAttemptAt, &leaseExpiresAt, &item.Provider, &item.ProviderRequestID, &item.RequestPayloadSHA256,
		&item.LastError, &item.ActorUserID, &item.ActorTenantID, &item.OperationID, &startedAt, &finishedAt,
		&createdAt, &updatedAt,
		&item.Domain.Hostname, &item.Domain.Status, &domainPrimary, &domainVerifiedAt, &item.Domain.Version, &domainDeletedAt, &item.Domain.TenantName,
		&item.Delivery.ID, &item.Delivery.DomainID, &item.Delivery.TenantID, &item.Delivery.DeliveryStatus, &item.Delivery.RoutingStatus, &item.Delivery.CertificateStatus,
		&item.Delivery.Provider, &item.Delivery.ProviderRequestID, &item.Delivery.CertificateID, &deliveryNotBefore, &deliveryExpiresAt,
		&deliveryLastEventAt, &deliveryLastReconciledAt, &item.Delivery.LastError, &item.Delivery.Version, &deliveryCreatedAt, &deliveryUpdatedAt)
	item.NextAttemptAt, item.LeaseExpiresAt = formatTime(nextAttemptAt), formatTime(leaseExpiresAt)
	item.StartedAt, item.FinishedAt = formatTime(startedAt), formatTime(finishedAt)
	item.CreatedAt, item.UpdatedAt = formatTime(createdAt), formatTime(updatedAt)
	item.Domain.ID, item.Domain.TenantID, item.Domain.IsPrimary = item.DomainID, item.TenantID, domainPrimary == 1
	item.Domain.VerifiedAt, item.Domain.DeletedAt = formatTime(domainVerifiedAt), formatTime(domainDeletedAt)
	item.Delivery.CertificateNotBefore, item.Delivery.CertificateExpiresAt = formatTime(deliveryNotBefore), formatTime(deliveryExpiresAt)
	item.Delivery.LastEventAt, item.Delivery.LastReconciledAt = formatTime(deliveryLastEventAt), formatTime(deliveryLastReconciledAt)
	item.Delivery.CreatedAt, item.Delivery.UpdatedAt = formatTime(deliveryCreatedAt), formatTime(deliveryUpdatedAt)
	item.Domain.Delivery = item.Delivery
	return item, err
}

func newSaaSTenantDomainDeliveryJobNo() (string, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return "DDJ-" + time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(random), nil
}

func insertSaaSTenantDomainDeliveryOperation(ctx context.Context, tx *sql.Tx, action string, domain dashboard.SaaSTenantDomain, before, after dashboard.SaaSTenantDomainDelivery, job dashboard.SaaSTenantDomainDeliveryJob, actorUserID, actorTenantID int, remark string) (int64, error) {
	beforeJSON, _ := json.Marshal(map[string]any{"delivery": before})
	afterJSON, _ := json.Marshal(map[string]any{"delivery": after, "jobNo": job.JobNo, "jobAction": job.Action, "jobStatus": job.Status})
	return insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: domain.TenantID, ActorUserID: actorUserID, ActorTenantID: actorTenantID,
		Action: "saas.admin.tenant_domain_delivery." + action, TargetType: dashboard.SaaSAdminOperationTargetTenantDomainDelivery,
		TargetID: strconv.FormatInt(domain.ID, 10), TargetName: domain.Hostname,
		BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON), Remark: truncateDomainDeliveryStoreText(remark, 500),
	})
}

func normalizeDomainDeliveryStoreUpdate(update dashboard.SaaSTenantDomainDeliveryBridgeUpdate) (dashboard.SaaSTenantDomainDeliveryBridgeUpdate, error) {
	// Keep store validation independent from the HTTP adapter and callbacks.
	update.Status = strings.ToLower(strings.TrimSpace(update.Status))
	update.Provider = truncateDomainDeliveryStoreText(update.Provider, 64)
	update.ProviderRequestID = truncateDomainDeliveryStoreText(update.ProviderRequestID, 128)
	update.RoutingStatus = strings.ToLower(strings.TrimSpace(update.RoutingStatus))
	update.CertificateStatus = strings.ToLower(strings.TrimSpace(update.CertificateStatus))
	update.CertificateID = truncateDomainDeliveryStoreText(update.CertificateID, 191)
	update.Error = truncateDomainDeliveryStoreText(update.Error, 500)
	if update.Status == "" {
		update.Status = dashboard.SaaSTenantDomainDeliveryBridgeStatusAccepted
	}
	if update.Status != dashboard.SaaSTenantDomainDeliveryBridgeStatusAccepted && update.Status != dashboard.SaaSTenantDomainDeliveryBridgeStatusReady && update.Status != dashboard.SaaSTenantDomainDeliveryBridgeStatusFailed {
		return update, errors.New("invalid domain delivery bridge status")
	}
	if update.RoutingStatus == "" {
		update.RoutingStatus = dashboard.SaaSTenantDomainRoutingStatusProvisioning
	}
	if update.CertificateStatus == "" {
		update.CertificateStatus = dashboard.SaaSTenantDomainCertificateStatusProvisioning
	}
	return update, nil
}

func domainDeliveryStoreStatus(update dashboard.SaaSTenantDomainDeliveryBridgeUpdate) string {
	if update.RoutingStatus == dashboard.SaaSTenantDomainRoutingStatusDeleted || update.CertificateStatus == dashboard.SaaSTenantDomainCertificateStatusDeleted {
		return dashboard.SaaSTenantDomainDeliveryStatusDeleted
	}
	if update.RoutingStatus == dashboard.SaaSTenantDomainRoutingStatusDisabled || update.CertificateStatus == dashboard.SaaSTenantDomainCertificateStatusDisabled || update.CertificateStatus == dashboard.SaaSTenantDomainCertificateStatusRevoked {
		return dashboard.SaaSTenantDomainDeliveryStatusDisabled
	}
	if update.Status == dashboard.SaaSTenantDomainDeliveryBridgeStatusFailed || update.RoutingStatus == dashboard.SaaSTenantDomainRoutingStatusFailed || update.CertificateStatus == dashboard.SaaSTenantDomainCertificateStatusFailed {
		return dashboard.SaaSTenantDomainDeliveryStatusFailed
	}
	if update.RoutingStatus == dashboard.SaaSTenantDomainRoutingStatusReady && update.CertificateStatus == dashboard.SaaSTenantDomainCertificateStatusActive {
		return dashboard.SaaSTenantDomainDeliveryStatusReady
	}
	if update.RoutingStatus == dashboard.SaaSTenantDomainRoutingStatusReady && update.CertificateStatus == dashboard.SaaSTenantDomainCertificateStatusExpiring {
		return dashboard.SaaSTenantDomainDeliveryStatusDegraded
	}
	return dashboard.SaaSTenantDomainDeliveryStatusProvisioning
}

func nullableDomainDeliveryTime(value string) (any, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	return dashboardParseDomainDeliveryTime(value)
}

func dashboardParseDomainDeliveryTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("invalid domain delivery timestamp")
}

func domainDeliveryDurationSeconds(value time.Duration) int64 {
	seconds := int64((value + time.Second - 1) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}

func truncateDomainDeliveryStoreText(value string, max int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > max {
		value = string(runes[:max])
	}
	return value
}

var _ dashboard.SaaSTenantDomainDeliveryStore = (*MySQLStore)(nil)
