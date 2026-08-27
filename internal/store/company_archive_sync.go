package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboardprincipal"
)

// GetArchiveSyncStatus projects the durable archive ledger into a small,
// operator-facing status. It deliberately omits source identity, cursors,
// leases, idempotency keys and raw provider error codes.
func (s *MySQLStore) GetArchiveSyncStatus(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) (companyprofile.ArchiveSyncStatus, error) {
	if s == nil || s.db == nil || principal.TenantID <= 0 || principal.CorpID <= 0 {
		return companyprofile.ArchiveSyncStatus{}, errors.New("archive sync status scope invalid")
	}
	var available int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mochat_go_wecom_integrations integration
		INNER JOIN mc_tenant tenant ON tenant.id=integration.tenant_id AND tenant.status=1 AND tenant.deleted_at IS NULL
		INNER JOIN mc_corp corp ON corp.tenant_id=integration.tenant_id AND corp.id=integration.corp_id AND corp.deleted_at IS NULL
		INNER JOIN mochat_go_tenant_corp_bindings binding ON binding.tenant_id=integration.tenant_id AND binding.corp_id=integration.corp_id
		WHERE `+durableArchiveEligibilityPredicate+`
		  AND integration.tenant_id=? AND integration.corp_id=?
	`, principal.TenantID, principal.CorpID).Scan(&available)
	if err != nil {
		return companyprofile.ArchiveSyncStatus{}, err
	}
	if available == 0 {
		return companyprofile.ArchiveSyncStatus{
			Status: "idle", Available: false,
			UnavailableReason: "需要管理员先完成会话存档授权",
		}, nil
	}

	var status companyprofile.ArchiveSyncStatus
	var rawStatus string
	var startedAt, finishedAt sql.NullTime
	err = s.db.QueryRowContext(ctx, `
		SELECT run.id,run.status,run.fetched_count,run.processed_count,run.skipped_count,run.failed_count,
		       run.started_at,run.finished_at
		FROM mochat_go_archive_sync_runs run
		INNER JOIN mochat_go_wecom_integrations integration
		  ON integration.tenant_id=run.tenant_id AND integration.corp_id=run.corp_id
		WHERE run.tenant_id=? AND run.corp_id=? AND run.source_kind='external'
		  AND run.source_id=CONCAT('wecom:',integration.verified_wx_corpid)
		ORDER BY run.updated_at DESC,run.id DESC
		LIMIT 1
	`, principal.TenantID, principal.CorpID).Scan(
		&status.RunID, &rawStatus, &status.Fetched, &status.Processed, &status.Skipped, &status.Failed,
		&startedAt, &finishedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return companyprofile.ArchiveSyncStatus{Status: "idle", Available: true}, nil
	}
	if err != nil {
		return companyprofile.ArchiveSyncStatus{}, err
	}
	status.Available = true
	status.Status = operatorArchiveSyncStatus(rawStatus)
	status.StartedAt = nullTimePointer(startedAt)
	status.FinishedAt = nullTimePointer(finishedAt)
	return status, nil
}

func operatorArchiveSyncStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "queued":
		return "queued"
	case "running", "syncing":
		return "syncing"
	case "succeeded", "completed":
		return "completed"
	case "failed":
		return "failed"
	default:
		return "idle"
	}
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
