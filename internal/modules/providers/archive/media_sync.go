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
	ID             string
	Scope          Scope
	WXCorpID       string
	MsgID          string
	SourceIdentity string
	SDKFileID      string
	MediaType      string
	FileName       string
	MIMEType       string
	ExpectedSize   int64
	ExpectedMD5    string
	Status         ArchiveMediaStatus
	IndexBuf       string
	BytesReceived  int64
	Attempt        int
	LeaseToken     string
}

type ArchiveMediaCheckpoint struct {
	ID            string
	Attempt       int
	LeaseToken    string
	NextIndexBuf  string
	BytesReceived int64
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

type ArchiveMediaClient interface {
	FetchMedia(context.Context, Scope, string, string, string) (MediaChunk, error)
}

type MediaSyncService struct {
	store  ArchiveMediaStore
	client ArchiveMediaClient
	root   string
	now    func() time.Time
}

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
	partPath := finalPath + ".part"
	if _, statErr := os.Stat(finalPath); statErr == nil {
		shaValue, md5Value, size, hashErr := hashMediaFile(finalPath)
		if hashErr != nil {
			return true, s.recordFailed(ctx, object, "archive.media_storage_read_failed", hashErr)
		}
		if !archiveMediaIntegrityMatches(object, size, md5Value) {
			_ = os.Remove(finalPath)
			return true, s.recordCorrupt(ctx, object, "archive.media_integrity_mismatch", partPath, errors.New("archive media integrity mismatch"))
		}
		completion := ArchiveMediaCompletion{ID: object.ID, Attempt: object.Attempt, LeaseToken: object.LeaseToken, BytesReceived: size, SHA256: shaValue, StoragePath: finalPath}
		if err := s.store.CompleteArchiveMedia(ctx, completion, s.now()); err != nil {
			return true, err
		}
		return true, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return true, s.recordFailed(ctx, object, "archive.media_storage_unavailable", statErr)
	}
	file, err := openMediaPart(partPath, object.BytesReceived)
	if err != nil {
		return true, s.recordCorrupt(ctx, object, "archive.media_checkpoint_mismatch", partPath, err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
	}()

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
		checkpoint := ArchiveMediaCheckpoint{ID: object.ID, Attempt: object.Attempt, LeaseToken: object.LeaseToken, NextIndexBuf: next, BytesReceived: bytesReceived}
		if err := s.store.CheckpointArchiveMedia(ctx, checkpoint, s.now()); err != nil {
			_ = file.Close()
			closed = true
			return true, err
		}
		if !chunk.Finished {
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
	if err := s.store.RenewArchiveMedia(ctx, object.ID, object.Attempt, object.LeaseToken, s.now()); err != nil {
		return true, err
	}
	if err := os.Rename(partPath, finalPath); err != nil {
		return true, s.recordFailed(ctx, object, "archive.media_storage_commit_failed", err)
	}
	completion := ArchiveMediaCompletion{ID: object.ID, Attempt: object.Attempt, LeaseToken: object.LeaseToken, BytesReceived: size, SHA256: shaValue, StoragePath: finalPath}
	if err := s.store.CompleteArchiveMedia(ctx, completion, s.now()); err != nil {
		return true, err
	}
	return true, nil
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
		object.Attempt <= 0 || strings.TrimSpace(object.LeaseToken) == "" || object.BytesReceived < 0 {
		return errors.New("archive media claim is incomplete")
	}
	return nil
}

func openMediaPart(path string, checkpoint int64) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if stat.Size() < checkpoint {
		_ = file.Close()
		return nil, errors.New("archive media part is shorter than checkpoint")
	}
	if stat.Size() > checkpoint {
		if err := file.Truncate(checkpoint); err != nil {
			_ = file.Close()
			return nil, err
		}
	}
	if _, err := file.Seek(checkpoint, io.SeekStart); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
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
			_ = os.Remove(partPath)
			failure := mediaFailure(object, "archive.media_missing")
			if err := s.store.MarkArchiveMediaMissing(ctx, failure, s.now()); err != nil {
				return err
			}
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
	return fmt.Errorf("%s: %w", code, sanitizeMediaCause(cause))
}

func (s *MediaSyncService) recordCorrupt(ctx context.Context, object ArchiveMediaObject, code, partPath string, cause error) error {
	if partPath != "" {
		_ = os.Remove(partPath)
	}
	if err := s.store.MarkArchiveMediaCorrupt(ctx, mediaFailure(object, code), s.now()); err != nil {
		return err
	}
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
