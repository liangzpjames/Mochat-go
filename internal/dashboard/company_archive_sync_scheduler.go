package dashboard

import (
	"context"
	"errors"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboardprincipal"
	archiveprovider "jiyi/mochat-go/internal/modules/providers/archive"
)

type companyArchiveSyncRunner interface {
	EnqueueScope(context.Context, archiveprovider.Scope, string) (archiveprovider.SyncRun, error)
}

type CompanyArchiveSyncScheduler struct {
	runner companyArchiveSyncRunner
}

func NewCompanyArchiveSyncScheduler(runner companyArchiveSyncRunner) *CompanyArchiveSyncScheduler {
	return &CompanyArchiveSyncScheduler{runner: runner}
}

func (s *CompanyArchiveSyncScheduler) EnqueueArchiveSync(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, requestID string) (companyprofile.ArchiveSyncStatus, error) {
	if s == nil || s.runner == nil {
		return companyprofile.ArchiveSyncStatus{}, companyprofile.ErrStoreUnavailable
	}
	run, err := s.runner.EnqueueScope(ctx, archiveprovider.Scope{TenantID: int64(principal.TenantID), CorpID: int64(principal.CorpID)}, requestID)
	if errors.Is(err, archiveprovider.ErrDurableArchiveBindingUnavailable) {
		return companyprofile.ArchiveSyncStatus{Status: "idle", Available: false, UnavailableReason: "需要管理员先完成会话存档授权"}, companyprofile.ErrTenantAccessDenied
	}
	if err != nil {
		return companyprofile.ArchiveSyncStatus{}, companyprofile.ErrStoreUnavailable
	}
	return companyprofile.ArchiveSyncStatus{
		Status: operatorArchiveRunStatus(run.Status), Available: true, RunID: run.ID,
		Fetched: run.Counts.Fetched, Processed: run.Counts.Processed, Skipped: run.Counts.Skipped, Failed: run.Counts.Failed,
		StartedAt: run.StartedAt, FinishedAt: run.FinishedAt,
	}, nil
}

func operatorArchiveRunStatus(status archiveprovider.SyncStatus) string {
	switch status {
	case archiveprovider.SyncStatusQueued:
		return "queued"
	case archiveprovider.SyncStatusRunning:
		return "syncing"
	case archiveprovider.SyncStatusSucceeded:
		return "completed"
	case archiveprovider.SyncStatusFailed:
		return "failed"
	default:
		return "idle"
	}
}
