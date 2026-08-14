package archive

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
)

type SyncStatus string

const (
	SyncStatusQueued    SyncStatus = "queued"
	SyncStatusRunning   SyncStatus = "running"
	SyncStatusSucceeded SyncStatus = "succeeded"
	SyncStatusFailed    SyncStatus = "failed"
)

type SyncCounts struct {
	Fetched   int `json:"fetched"`
	Processed int `json:"processed"`
	Skipped   int `json:"skipped"`
	Failed    int `json:"failed"`
}

type SyncRequest struct {
	Scope          Scope
	IdempotencyKey string
	Limit          int
	RetryFailed    bool
}

type SyncRun struct {
	ID             string
	Scope          Scope
	Source         providers.Source
	SourceID       string
	Namespace      string
	IdempotencyKey string
	Status         SyncStatus
	Cursor         Cursor
	Counts         SyncCounts
	ErrorCode      string
	Attempt        int
	StartedAt      *time.Time
	FinishedAt     *time.Time
	Idempotent     bool
}

type UpsertResult struct {
	Inserted bool
	Skipped  bool
}

// SyncStore owns persistence, cursor storage, idempotency and audit writes.
// SyncService only coordinates source fetches and state transitions.
type SyncStore interface {
	EnqueueArchiveSync(context.Context, SyncRun, bool) (SyncRun, error)
	MarkArchiveSyncRunning(context.Context, string, time.Time) error
	UpsertArchiveMessage(context.Context, string, Scope, Message) (UpsertResult, error)
	SaveArchiveSyncCursor(context.Context, string, Cursor, time.Time) error
	CompleteArchiveSync(context.Context, string, SyncCounts, Cursor, time.Time) (SyncRun, error)
	FailArchiveSync(context.Context, string, SyncCounts, Cursor, string, time.Time) (SyncRun, error)
}

type SyncService struct {
	store SyncStore
	now   func() time.Time
}

func NewSyncService(store SyncStore) *SyncService {
	return &SyncService{store: store, now: time.Now}
}

func (s *SyncService) WithClock(now func() time.Time) *SyncService {
	if s != nil && now != nil {
		s.now = now
	}
	return s
}

func (s *SyncService) Sync(ctx context.Context, source ArchiveSource, request SyncRequest) (SyncRun, error) {
	if s == nil || s.store == nil {
		return SyncRun{}, newSyncError("archive.persistence_unavailable", nil)
	}
	if ctx == nil {
		return SyncRun{}, newSyncError("archive.context_required", nil)
	}
	template, err := syncRunTemplate(source, request)
	if err != nil {
		return SyncRun{}, err
	}
	run, err := s.store.EnqueueArchiveSync(ctx, template, request.RetryFailed)
	if err != nil {
		return run, newSyncError("archive.persistence_failed", err)
	}
	if run.Status == SyncStatusSucceeded {
		run.Idempotent = true
		return run, nil
	}
	if run.Status == SyncStatusFailed && !request.RetryFailed {
		return run, newSyncError("archive.retry_required", nil)
	}
	if run.Status == SyncStatusRunning {
		return run, newSyncError("archive.sync_in_progress", nil)
	}
	if run.Status != SyncStatusQueued {
		return run, newSyncError("archive.invalid_run_state", nil)
	}

	now := s.now()
	if err := s.store.MarkArchiveSyncRunning(ctx, run.ID, now); err != nil {
		return run, newSyncError("archive.persistence_failed", err)
	}
	run.Status = SyncStatusRunning
	run.StartedAt = &now
	counts := SyncCounts{}
	cursor := run.Cursor
	limit := request.Limit
	if limit <= 0 {
		limit = DefaultFetchLimit
	}
	for {
		page, fetchErr := source.Fetch(ctx, request.Scope, cursor, limit)
		if fetchErr != nil {
			counts.Failed++
			return s.fail(ctx, run, counts, cursor, sourceErrorCode(source, fetchErr), fetchErr)
		}
		if len(page.Messages) == 0 {
			if page.HasMore {
				counts.Failed++
				return s.fail(ctx, run, counts, cursor, "archive.cursor_stalled", nil)
			}
			break
		}
		counts.Fetched += len(page.Messages)
		previousCursor := cursor
		pageCursor := previousCursor
		if page.NextCursor.Sequence > pageCursor.Sequence {
			pageCursor.Sequence = page.NextCursor.Sequence
		}
		if strings.TrimSpace(page.NextCursor.Token) != "" {
			pageCursor.Token = strings.TrimSpace(page.NextCursor.Token)
		}
		for _, message := range page.Messages {
			if !messageIdentityMatches(message, source) {
				counts.Failed++
				return s.fail(ctx, run, counts, cursor, "archive.source_identity_mismatch", nil)
			}
			if message.Seq <= cursor.Sequence || message.Seq <= 0 || strings.TrimSpace(message.MsgID) == "" {
				counts.Skipped++
				continue
			}
			upsert, upsertErr := s.store.UpsertArchiveMessage(ctx, run.ID, request.Scope, message)
			if upsertErr != nil {
				counts.Failed++
				return s.fail(ctx, run, counts, cursor, "archive.persistence_failed", upsertErr)
			}
			if upsert.Inserted {
				counts.Processed++
			} else {
				counts.Skipped++
			}
			if message.Seq > pageCursor.Sequence {
				pageCursor.Sequence = message.Seq
			}
		}
		progressed := pageCursor.Sequence > previousCursor.Sequence || pageCursor.Token != previousCursor.Token
		if progressed {
			if err := s.store.SaveArchiveSyncCursor(ctx, run.ID, pageCursor, s.now()); err != nil {
				counts.Failed++
				return s.fail(ctx, run, counts, previousCursor, "archive.persistence_failed", err)
			}
			cursor = pageCursor
		}
		if !page.HasMore {
			break
		}
		if !progressed {
			counts.Failed++
			return s.fail(ctx, run, counts, cursor, "archive.cursor_stalled", nil)
		}
	}

	completed, err := s.store.CompleteArchiveSync(ctx, run.ID, counts, cursor, s.now())
	if err != nil {
		return run, newSyncError("archive.persistence_failed", err)
	}
	completed.Status = SyncStatusSucceeded
	completed.Counts = counts
	completed.Cursor = cursor
	return completed, nil
}

func (s *SyncService) Retry(ctx context.Context, source ArchiveSource, request SyncRequest) (SyncRun, error) {
	request.RetryFailed = true
	return s.Sync(ctx, source, request)
}

func (s *SyncService) fail(ctx context.Context, run SyncRun, counts SyncCounts, cursor Cursor, code string, cause error) (SyncRun, error) {
	failed, err := s.store.FailArchiveSync(ctx, run.ID, counts, cursor, code, s.now())
	if err != nil {
		return run, newSyncError("archive.persistence_failed", err)
	}
	failed.Status = SyncStatusFailed
	failed.Counts = counts
	failed.Cursor = cursor
	failed.ErrorCode = code
	return failed, newSyncError(code, cause)
}

func syncRunTemplate(source ArchiveSource, request SyncRequest) (SyncRun, error) {
	if source == nil {
		return SyncRun{}, newSyncError("archive.source_unavailable", nil)
	}
	if !request.Scope.valid() {
		return SyncRun{}, newSyncError("archive.scope_invalid", ErrInvalidScope)
	}
	if strings.TrimSpace(request.IdempotencyKey) == "" {
		return SyncRun{}, newSyncError("archive.idempotency_key_missing", nil)
	}
	sourceID := strings.TrimSpace(source.SourceID())
	namespace := strings.TrimSpace(source.Namespace())
	if sourceID == "" || namespace == "" || (source.Kind() != providers.SourceExternal && source.Kind() != providers.SourceSimulated) {
		return SyncRun{}, newSyncError("archive.source_invalid", nil)
	}
	return SyncRun{
		Scope: request.Scope, Source: source.Kind(), SourceID: sourceID,
		Namespace: namespace, IdempotencyKey: strings.TrimSpace(request.IdempotencyKey),
		Status: SyncStatusQueued,
	}, nil
}

func messageIdentityMatches(message Message, source ArchiveSource) bool {
	return message.Source == source.Kind() &&
		strings.TrimSpace(message.SourceID) == strings.TrimSpace(source.SourceID()) &&
		strings.TrimSpace(message.Namespace) == strings.TrimSpace(source.Namespace())
}

func sourceErrorCode(source ArchiveSource, err error) string {
	if errors.Is(err, ErrInvalidScope) {
		return "archive.scope_invalid"
	}
	if errors.Is(err, providers.ErrNotConfigured) {
		return "archive.credentials_missing"
	}
	if errors.Is(err, providers.ErrCapabilityUnavailable) {
		status := source.Status()
		if strings.HasPrefix(strings.TrimSpace(status.Code), "archive.") {
			return status.Code
		}
		return "archive.source_unavailable"
	}
	return "archive.sync_failed"
}

type SyncError struct {
	Code  string
	Cause error
}

func (e *SyncError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code
}

func (e *SyncError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func newSyncError(code string, cause error) error {
	if strings.TrimSpace(code) == "" {
		code = "archive.sync_failed"
	}
	return &SyncError{Code: code, Cause: cause}
}

func ErrorCode(err error) string {
	var syncErr *SyncError
	if errors.As(err, &syncErr) {
		return syncErr.Code
	}
	return ""
}

func (r SyncRun) String() string {
	return fmt.Sprintf("archive-sync(%s,%s,%s)", r.Source, r.SourceID, r.Status)
}
