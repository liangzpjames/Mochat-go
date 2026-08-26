package archive

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ArchiveMediaStatus string

const (
	ArchiveMediaPending  ArchiveMediaStatus = "pending"
	ArchiveMediaFetching ArchiveMediaStatus = "fetching"
	ArchiveMediaReady    ArchiveMediaStatus = "ready"
	ArchiveMediaFailed   ArchiveMediaStatus = "failed"
	ArchiveMediaMissing  ArchiveMediaStatus = "missing"
	ArchiveMediaCorrupt  ArchiveMediaStatus = "corrupt"
)

type ArchiveMediaObject struct {
	ID                string
	Scope             Scope
	WXCorpID          string
	MsgID             string
	SourceIdentity    string
	SDKFileID         string
	MediaType         string
	FileName          string
	MIMEType          string
	ExpectedSize      int64
	ExpectedMD5       string
	Status            ArchiveMediaStatus
	IndexBuf          string
	BytesReceived     int64
	CheckpointAttempt int
	DownloadFinished  bool
	DownloadSHA256    string
	Attempt           int
	LeaseToken        string
}

type ArchiveMediaCheckpoint struct {
	ID            string
	Attempt       int
	LeaseToken    string
	NextIndexBuf  string
	BytesReceived int64
	Finished      bool
	SHA256        string
}

type ArchiveMediaCompletion struct {
	ID            string
	Attempt       int
	LeaseToken    string
	BytesReceived int64
	SHA256        string
	StoragePath   string
}

type ArchiveMediaFailure struct {
	ID         string
	Attempt    int
	LeaseToken string
	ErrorCode  string
}

type ArchiveMediaStore interface {
	ClaimArchiveMedia(context.Context, time.Time) (ArchiveMediaObject, bool, error)
	RenewArchiveMedia(context.Context, string, int, string, time.Time) error
	CheckpointArchiveMedia(context.Context, ArchiveMediaCheckpoint, time.Time) error
	CompleteArchiveMedia(context.Context, ArchiveMediaCompletion, time.Time) error
	FailArchiveMedia(context.Context, ArchiveMediaFailure, time.Time) error
	MarkArchiveMediaMissing(context.Context, ArchiveMediaFailure, time.Time) error
	MarkArchiveMediaCorrupt(context.Context, ArchiveMediaFailure, time.Time) error
}

type ArchiveMediaAttemptReference struct {
	ID                string
	Status            ArchiveMediaStatus
	CheckpointAttempt int
	ActiveAttempt     int
}

type ArchiveMediaAttemptStore interface {
	ArchiveMediaAttemptReferences(context.Context) ([]ArchiveMediaAttemptReference, error)
}

type ArchiveMediaClient interface {
	FetchMedia(context.Context, Scope, string, string, string) (MediaChunk, error)
}

type MediaSyncService struct {
	store  ArchiveMediaStore
	client ArchiveMediaClient
	root   string
	now    func() time.Time
}

var archiveMediaAttemptNamePattern = regexp.MustCompile(`^([0-9a-fA-F-]{36})\.attempt-([1-9][0-9]*)\.part$`)

func NewMediaSyncService(store ArchiveMediaStore, client ArchiveMediaClient, root string) *MediaSyncService {
	return &MediaSyncService{store: store, client: client, root: strings.TrimSpace(root), now: time.Now}
}

func (s *MediaSyncService) WithClock(now func() time.Time) *MediaSyncService {
	if s != nil && now != nil {
		s.now = now
	}
	return s
}

// RunOne claims and finishes at most one media object. The message and its
// cursor were committed before this worker runs; media failures never mutate
// either of them.
func (s *MediaSyncService) RunOne(ctx context.Context) (bool, error) {
	if s == nil || s.store == nil || s.client == nil || s.root == "" {
		return false, errors.New("archive media service is not configured")
	}
	if ctx == nil {
		return false, errors.New("context is required")
	}
	object, found, err := s.store.ClaimArchiveMedia(ctx, s.now())
	if err != nil || !found {
		return false, err
	}
	if err := validateClaimedMedia(object); err != nil {
		return true, s.recordCorrupt(ctx, object, "archive.media_claim_invalid", "", err)
	}
	dir := filepath.Join(s.root, "archive-media")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return true, s.recordFailed(ctx, object, "archive.media_storage_unavailable", err)
	}
	finalPath := filepath.Join(dir, object.ID)
	partPath := archiveMediaAttemptPath(dir, object.ID, object.Attempt)
	if _, statErr := os.Stat(finalPath); statErr == nil {
		shaValue, md5Value, size, hashErr := hashMediaFile(finalPath)
		if hashErr != nil {
			return true, s.recordFailed(ctx, object, "archive.media_storage_read_failed", hashErr)
		}
		if !object.DownloadFinished || !strings.EqualFold(object.DownloadSHA256, shaValue) || !archiveMediaIntegrityMatches(object, size, md5Value) {
			return true, s.recordCorrupt(ctx, object, "archive.media_integrity_mismatch", partPath, errors.New("archive media integrity mismatch"))
		}
		completion := ArchiveMediaCompletion{ID: object.ID, Attempt: object.Attempt, LeaseToken: object.LeaseToken, BytesReceived: size, SHA256: shaValue, StoragePath: finalPath}
		if err := s.store.CompleteArchiveMedia(ctx, completion, s.now()); err != nil {
			return true, err
		}
		s.cleanupUUIDAttemptsBestEffort(object.ID, 0, 0)
		return true, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return true, s.recordFailed(ctx, object, "archive.media_storage_unavailable", statErr)
	}
	file, err := prepareMediaAttemptPart(dir, object)
	if err != nil {
		return true, s.recordCorrupt(ctx, object, "archive.media_checkpoint_mismatch", partPath, err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
	}()

	if object.DownloadFinished {
		if err := file.Close(); err != nil {
			closed = true
			return true, s.recordFailed(ctx, object, "archive.media_storage_close_failed", err)
		}
		closed = true
		return true, s.commitFinishedPart(ctx, object, partPath, finalPath, object.DownloadSHA256)
	}

	indexBuf := object.IndexBuf
	bytesReceived := object.BytesReceived
	for {
		if err := s.store.RenewArchiveMedia(ctx, object.ID, object.Attempt, object.LeaseToken, s.now()); err != nil {
			return true, err
		}
		chunk, fetchErr := s.client.FetchMedia(ctx, object.Scope, object.WXCorpID, object.SDKFileID, indexBuf)
		if fetchErr != nil {
			_ = file.Close()
			closed = true
			return true, s.classifyFetchFailure(ctx, object, partPath, fetchErr)
		}
		// The network call can outlive the lease. Fence the response before it
		// can touch any attempt-owned file.
		if err := s.store.RenewArchiveMedia(ctx, object.ID, object.Attempt, object.LeaseToken, s.now()); err != nil {
			_ = file.Close()
			closed = true
			return true, err
		}
		next := strings.TrimSpace(chunk.NextIndexBuf)
		if !chunk.Finished && (next == "" || next == indexBuf) {
			_ = file.Close()
			closed = true
			return true, s.recordFailed(ctx, object, "archive.media_cursor_stalled", errors.New("archive media cursor did not advance"))
		}
		if len(chunk.Data) > 0 {
			written, writeErr := file.Write(chunk.Data)
			if writeErr != nil || written != len(chunk.Data) {
				if writeErr == nil {
					writeErr = io.ErrShortWrite
				}
				_ = file.Close()
				closed = true
				return true, s.recordFailed(ctx, object, "archive.media_storage_write_failed", writeErr)
			}
			bytesReceived += int64(written)
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			closed = true
			return true, s.recordFailed(ctx, object, "archive.media_storage_sync_failed", err)
		}
		if !chunk.Finished {
			previousCheckpointAttempt := object.CheckpointAttempt
			checkpoint := ArchiveMediaCheckpoint{ID: object.ID, Attempt: object.Attempt, LeaseToken: object.LeaseToken, NextIndexBuf: next, BytesReceived: bytesReceived}
			if err := s.store.CheckpointArchiveMedia(ctx, checkpoint, s.now()); err != nil {
				_ = file.Close()
				closed = true
				return true, err
			}
			object.CheckpointAttempt = object.Attempt
			object.BytesReceived = bytesReceived
			if previousCheckpointAttempt > 0 && previousCheckpointAttempt != object.Attempt {
				_ = os.Remove(archiveMediaAttemptPath(dir, object.ID, previousCheckpointAttempt))
			}
			indexBuf = next
			continue
		}
		break
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		closed = true
		return true, s.recordFailed(ctx, object, "archive.media_storage_sync_failed", err)
	}
	if err := file.Close(); err != nil {
		closed = true
		return true, s.recordFailed(ctx, object, "archive.media_storage_close_failed", err)
	}
	closed = true
	shaValue, md5Value, size, err := hashMediaFile(partPath)
	if err != nil {
		return true, s.recordFailed(ctx, object, "archive.media_storage_read_failed", err)
	}
	if !archiveMediaIntegrityMatches(object, size, md5Value) {
		return true, s.recordCorrupt(ctx, object, "archive.media_integrity_mismatch", partPath, errors.New("archive media integrity mismatch"))
	}
	checkpoint := ArchiveMediaCheckpoint{ID: object.ID, Attempt: object.Attempt, LeaseToken: object.LeaseToken,
		BytesReceived: size, Finished: true, SHA256: shaValue}
	if err := s.store.CheckpointArchiveMedia(ctx, checkpoint, s.now()); err != nil {
		return true, err
	}
	previousCheckpointAttempt := object.CheckpointAttempt
	object.CheckpointAttempt = object.Attempt
	object.BytesReceived = size
	object.DownloadFinished = true
	object.DownloadSHA256 = shaValue
	if previousCheckpointAttempt > 0 && previousCheckpointAttempt != object.Attempt {
		_ = os.Remove(archiveMediaAttemptPath(dir, object.ID, previousCheckpointAttempt))
	}
	return true, s.commitFinishedPart(ctx, object, partPath, finalPath, shaValue)
}

func (s *MediaSyncService) commitFinishedPart(ctx context.Context, object ArchiveMediaObject, partPath, finalPath, checkpointSHA string) error {
	shaValue, md5Value, size, err := hashMediaFile(partPath)
	if err != nil {
		return s.recordFailed(ctx, object, "archive.media_storage_read_failed", err)
	}
	if !strings.EqualFold(strings.TrimSpace(checkpointSHA), shaValue) || !archiveMediaIntegrityMatches(object, size, md5Value) {
		return s.recordCorrupt(ctx, object, "archive.media_integrity_mismatch", partPath, errors.New("archive media integrity mismatch"))
	}
	if err := s.store.RenewArchiveMedia(ctx, object.ID, object.Attempt, object.LeaseToken, s.now()); err != nil {
		return err
	}
	if err := publishArchiveMedia(partPath, finalPath, shaValue); err != nil {
		return s.recordFailed(ctx, object, "archive.media_storage_commit_failed", err)
	}
	completion := ArchiveMediaCompletion{ID: object.ID, Attempt: object.Attempt, LeaseToken: object.LeaseToken, BytesReceived: size, SHA256: shaValue, StoragePath: finalPath}
	if err := s.store.CompleteArchiveMedia(ctx, completion, s.now()); err != nil {
		return err
	}
	s.cleanupUUIDAttemptsBestEffort(object.ID, 0, 0)
	return nil
}

// CleanupStaleAttempts is the retryable GC boundary called by the periodic
// worker. It removes only files for database-known UUIDs and never removes an
// active fetching attempt or the checkpoint referenced by a resumable row.
func (s *MediaSyncService) CleanupStaleAttempts(ctx context.Context) (int, error) {
	if s == nil || s.store == nil || s.root == "" {
		return 0, errors.New("archive media service is not configured")
	}
	if ctx == nil {
		return 0, errors.New("context is required")
	}
	referenceStore, ok := s.store.(ArchiveMediaAttemptStore)
	if !ok {
		return 0, errors.New("archive media attempt cleanup store unavailable")
	}
	references, err := referenceStore.ArchiveMediaAttemptReferences(ctx)
	if err != nil {
		return 0, err
	}
	byID := make(map[string]ArchiveMediaAttemptReference, len(references))
	for _, reference := range references {
		parsed, parseErr := uuid.Parse(reference.ID)
		if parseErr != nil || !strings.EqualFold(parsed.String(), strings.TrimSpace(reference.ID)) {
			continue
		}
		byID[strings.ToLower(parsed.String())] = reference
	}
	dir := filepath.Join(s.root, "archive-media")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	removed := 0
	var cleanupErr error
	for _, entry := range entries {
		match := archiveMediaAttemptNamePattern.FindStringSubmatch(entry.Name())
		if len(match) != 3 || (entry.Type()&os.ModeSymlink == 0 && !entry.Type().IsRegular()) {
			continue
		}
		parsed, parseErr := uuid.Parse(match[1])
		attempt, attemptErr := strconv.Atoi(match[2])
		if parseErr != nil || attemptErr != nil || attempt <= 0 || !strings.EqualFold(parsed.String(), match[1]) {
			continue
		}
		reference, exists := byID[strings.ToLower(parsed.String())]
		if !exists || archiveMediaAttemptReferenced(reference, attempt) {
			continue
		}
		if removeErr := os.Remove(filepath.Join(dir, entry.Name())); removeErr != nil {
			cleanupErr = errors.Join(cleanupErr, errors.New("remove stale archive media attempt failed"))
			continue
		}
		removed++
	}
	return removed, cleanupErr
}

func archiveMediaAttemptReferenced(reference ArchiveMediaAttemptReference, attempt int) bool {
	switch reference.Status {
	case ArchiveMediaFetching:
		return attempt == reference.ActiveAttempt || (reference.CheckpointAttempt > 0 && attempt == reference.CheckpointAttempt)
	case ArchiveMediaPending, ArchiveMediaFailed:
		return reference.CheckpointAttempt > 0 && attempt == reference.CheckpointAttempt
	case ArchiveMediaReady, ArchiveMediaMissing, ArchiveMediaCorrupt:
		return false
	default:
		return true
	}
}

func (s *MediaSyncService) cleanupUUIDAttemptsBestEffort(id string, keepCheckpoint, keepActive int) {
	dir := filepath.Join(s.root, "archive-media")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		match := archiveMediaAttemptNamePattern.FindStringSubmatch(entry.Name())
		if len(match) != 3 || !strings.EqualFold(match[1], id) {
			continue
		}
		attempt, parseErr := strconv.Atoi(match[2])
		if parseErr != nil || attempt == keepCheckpoint || attempt == keepActive {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
}

func archiveMediaIntegrityMatches(object ArchiveMediaObject, size int64, md5Value string) bool {
	return size > 0 && (object.ExpectedSize <= 0 || size == object.ExpectedSize) &&
		(object.ExpectedMD5 == "" || strings.EqualFold(object.ExpectedMD5, md5Value))
}

func validateClaimedMedia(object ArchiveMediaObject) error {
	if _, err := uuid.Parse(object.ID); err != nil {
		return errors.New("archive media object ID is invalid")
	}
	if !object.Scope.valid() || strings.TrimSpace(object.WXCorpID) == "" || strings.TrimSpace(object.SDKFileID) == "" ||
		object.Attempt <= 0 || strings.TrimSpace(object.LeaseToken) == "" || object.BytesReceived < 0 || object.CheckpointAttempt < 0 {
		return errors.New("archive media claim is incomplete")
	}
	if object.BytesReceived > 0 && (object.CheckpointAttempt <= 0 || object.CheckpointAttempt >= object.Attempt) {
		return errors.New("archive media checkpoint owner is missing")
	}
	if object.DownloadFinished && (object.BytesReceived <= 0 || len(strings.TrimSpace(object.DownloadSHA256)) != 64) {
		return errors.New("archive media finished checkpoint is incomplete")
	}
	return nil
}

func archiveMediaAttemptPath(dir, id string, attempt int) string {
	return filepath.Join(dir, fmt.Sprintf("%s.attempt-%d.part", id, attempt))
}

func prepareMediaAttemptPart(dir string, object ArchiveMediaObject) (*os.File, error) {
	path := archiveMediaAttemptPath(dir, object.ID, object.Attempt)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	if object.BytesReceived > 0 {
		sourcePath := archiveMediaAttemptPath(dir, object.ID, object.CheckpointAttempt)
		source, openErr := os.Open(sourcePath)
		if openErr != nil {
			_ = file.Close()
			return nil, openErr
		}
		copied, copyErr := io.CopyN(file, source, object.BytesReceived)
		closeErr := source.Close()
		if copyErr != nil || copied != object.BytesReceived || closeErr != nil {
			_ = file.Close()
			if copyErr != nil {
				return nil, errors.New("archive media part is shorter than checkpoint")
			}
			return nil, errors.New("archive media checkpoint copy failed")
		}
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func publishArchiveMedia(partPath, finalPath, expectedSHA string) error {
	err := os.Link(partPath, finalPath)
	if err == nil {
		return os.Remove(partPath)
	}
	if !errors.Is(err, os.ErrExist) {
		return err
	}
	shaValue, _, _, hashErr := hashMediaFile(finalPath)
	if hashErr != nil || !strings.EqualFold(shaValue, expectedSHA) {
		return errors.New("archive media final object conflicts with checkpoint")
	}
	return os.Remove(partPath)
}

func hashMediaFile(path string) (shaValue, md5Value string, size int64, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", 0, err
	}
	defer file.Close()
	shaHash, md5Hash := sha256.New(), md5.New()
	size, err = io.Copy(io.MultiWriter(shaHash, md5Hash), file)
	if err != nil {
		return "", "", 0, err
	}
	return hex.EncodeToString(shaHash.Sum(nil)), hex.EncodeToString(md5Hash.Sum(nil)), size, nil
}

func (s *MediaSyncService) classifyFetchFailure(ctx context.Context, object ArchiveMediaObject, partPath string, cause error) error {
	var fetchErr *MediaFetchError
	if errors.As(cause, &fetchErr) {
		switch fetchErr.Code {
		case "ARCHIVE_MEDIA_NOT_FOUND", "ARCHIVE_MEDIA_MISSING":
			failure := mediaFailure(object, "archive.media_missing")
			if err := s.store.MarkArchiveMediaMissing(ctx, failure, s.now()); err != nil {
				return err
			}
			s.cleanupUUIDAttemptsBestEffort(object.ID, 0, 0)
			return errors.New("archive.media_missing")
		case "ARCHIVE_MEDIA_CORRUPT":
			return s.recordCorrupt(ctx, object, "archive.media_corrupt", partPath, cause)
		}
	}
	return s.recordFailed(ctx, object, "archive.media_fetch_failed", cause)
}

func (s *MediaSyncService) recordFailed(ctx context.Context, object ArchiveMediaObject, code string, cause error) error {
	if err := s.store.FailArchiveMedia(ctx, mediaFailure(object, code), s.now()); err != nil {
		return err
	}
	s.cleanupUUIDAttemptsBestEffort(object.ID, object.CheckpointAttempt, 0)
	return fmt.Errorf("%s: %w", code, sanitizeMediaCause(cause))
}

func (s *MediaSyncService) recordCorrupt(ctx context.Context, object ArchiveMediaObject, code, partPath string, cause error) error {
	if err := s.store.MarkArchiveMediaCorrupt(ctx, mediaFailure(object, code), s.now()); err != nil {
		return err
	}
	s.cleanupUUIDAttemptsBestEffort(object.ID, 0, 0)
	return fmt.Errorf("%s: %w", code, sanitizeMediaCause(cause))
}

func mediaFailure(object ArchiveMediaObject, code string) ArchiveMediaFailure {
	return ArchiveMediaFailure{ID: object.ID, Attempt: object.Attempt, LeaseToken: object.LeaseToken, ErrorCode: code}
}

func sanitizeMediaCause(cause error) error {
	if cause == nil {
		return errors.New("archive media operation failed")
	}
	var fetchErr *MediaFetchError
	if errors.As(cause, &fetchErr) {
		return errors.New(fetchErr.Error())
	}
	return errors.New("archive media operation failed")
}
