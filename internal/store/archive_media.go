package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	archiveprovider "jiyi/mochat-go/internal/modules/providers/archive"
	"jiyi/mochat-go/internal/wecomcredentials"

	"github.com/google/uuid"
)

const archiveMediaLeaseDuration = 5 * time.Minute

var errArchiveMediaFenceRejected = errors.New("archive media lease fence rejected")

func upsertArchiveMediaTx(ctx context.Context, tx *sql.Tx, cipher *wecomcredentials.Manager, scope archiveprovider.Scope, message archiveprovider.Message) error {
	if len(message.Media) == 0 {
		return nil
	}
	if cipher == nil || !cipher.ConfigStatus().EncryptionConfigured {
		return errors.New("archive media credential encryption is not configured")
	}
	for _, descriptor := range message.Media {
		sdkFileID := strings.TrimSpace(descriptor.SDKFileID)
		if sdkFileID == "" {
			return errors.New("archive media SDK file id is missing")
		}
		id := uuid.NewString()
		ciphertext, keyID, err := cipher.EncryptArchiveMedia(int(scope.TenantID), id, wecomcredentials.ArchiveMediaCredential{SDKFileID: sdkFileID})
		if err != nil {
			return err
		}
		hash := sha256.Sum256([]byte(sdkFileID))
		expectedMD5 := strings.ToLower(strings.TrimSpace(descriptor.ExpectedMD5))
		if len(expectedMD5) != 32 {
			expectedMD5 = ""
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO mochat_go_archive_media_objects
			(id,tenant_id,corp_id,msgid,source_identity,sdk_file_id_hash,sdk_file_id_ciphertext,sdk_file_id_key_id,
			 media_type,media_name,mime_type,expected_size_bytes,expected_md5,status)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,'pending')
			ON DUPLICATE KEY UPDATE id = id
		`, id, scope.TenantID, scope.CorpID, message.MsgID, strings.TrimSpace(message.SourceID), hex.EncodeToString(hash[:]), ciphertext, keyID,
			strings.TrimSpace(descriptor.Type), strings.TrimSpace(descriptor.FileName), strings.TrimSpace(descriptor.MIMEType), archiveMediaNonNegative(descriptor.ExpectedSize), expectedMD5)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *MySQLStore) ClaimArchiveMedia(ctx context.Context, at time.Time) (archiveprovider.ArchiveMediaObject, bool, error) {
	if s == nil || s.db == nil || s.weComCredentialCipher == nil {
		return archiveprovider.ArchiveMediaObject{}, false, errors.New("archive media store unavailable")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return archiveprovider.ArchiveMediaObject{}, false, err
	}
	defer rollbackQuietly(tx)
	var object archiveprovider.ArchiveMediaObject
	var ciphertext, keyID string
	var indexBuf sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT media.id,media.tenant_id,media.corp_id,COALESCE(NULLIF(binding.verified_wx_corpid,''),corp.wx_corpid),
		       media.msgid,media.source_identity,media.sdk_file_id_ciphertext,media.sdk_file_id_key_id,
		       media.media_type,media.media_name,media.mime_type,media.expected_size_bytes,media.expected_md5,
		       media.status,media.index_buf,media.bytes_received,media.checkpoint_attempt,
		       media.download_finished,media.download_sha256,media.attempt
		FROM mochat_go_archive_media_objects media
		INNER JOIN mc_corp corp ON corp.tenant_id=media.tenant_id AND corp.id=media.corp_id AND corp.deleted_at IS NULL
		LEFT JOIN mochat_go_tenant_corp_bindings binding ON binding.tenant_id=media.tenant_id AND binding.corp_id=media.corp_id
		WHERE media.status IN ('pending','failed') OR (media.status='fetching' AND media.lease_expires_at <= ?)
		ORDER BY media.updated_at ASC,media.id ASC
		LIMIT 1 FOR UPDATE
	`, at).Scan(&object.ID, &object.Scope.TenantID, &object.Scope.CorpID, &object.WXCorpID,
		&object.MsgID, &object.SourceIdentity, &ciphertext, &keyID, &object.MediaType, &object.FileName,
		&object.MIMEType, &object.ExpectedSize, &object.ExpectedMD5, &object.Status, &indexBuf, &object.BytesReceived,
		&object.CheckpointAttempt, &object.DownloadFinished, &object.DownloadSHA256, &object.Attempt)
	if errors.Is(err, sql.ErrNoRows) {
		return archiveprovider.ArchiveMediaObject{}, false, nil
	}
	if err != nil {
		return archiveprovider.ArchiveMediaObject{}, false, err
	}
	credential, err := s.weComCredentialCipher.DecryptArchiveMedia(int(object.Scope.TenantID), object.ID, keyID, ciphertext)
	if err != nil {
		return archiveprovider.ArchiveMediaObject{}, false, err
	}
	object.SDKFileID = credential.SDKFileID
	object.IndexBuf = indexBuf.String
	token, err := newArchiveLeaseToken()
	if err != nil {
		return archiveprovider.ArchiveMediaObject{}, false, err
	}
	object.Attempt++
	object.LeaseToken = token
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_archive_media_objects
		SET status='fetching',attempt=?,lease_token=?,lease_expires_at=?,heartbeat_at=?,last_error_code='',last_error='',updated_at=?
		WHERE id=? AND (status IN ('pending','failed') OR (status='fetching' AND lease_expires_at <= ?))
	`, object.Attempt, token, at.Add(archiveMediaLeaseDuration), at, at, object.ID, at)
	if err != nil {
		return archiveprovider.ArchiveMediaObject{}, false, err
	}
	if err := requireArchiveMediaRows(result); err != nil {
		return archiveprovider.ArchiveMediaObject{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return archiveprovider.ArchiveMediaObject{}, false, err
	}
	object.Status = archiveprovider.ArchiveMediaFetching
	return object, true, nil
}

func (s *MySQLStore) RenewArchiveMedia(ctx context.Context, id string, attempt int, token string, at time.Time) error {
	return s.archiveMediaLeaseMutation(ctx, `
		UPDATE mochat_go_archive_media_objects SET heartbeat_at=?,lease_expires_at=?,updated_at=?
		WHERE id=? AND status='fetching' AND attempt=? AND lease_token=?
	`, at, at.Add(archiveMediaLeaseDuration), at, id, attempt, strings.TrimSpace(token))
}

func (s *MySQLStore) CheckpointArchiveMedia(ctx context.Context, value archiveprovider.ArchiveMediaCheckpoint, at time.Time) error {
	if value.BytesReceived < 0 {
		return errors.New("archive media checkpoint bytes invalid")
	}
	shaValue := strings.ToLower(strings.TrimSpace(value.SHA256))
	if value.Finished {
		if value.BytesReceived <= 0 || strings.TrimSpace(value.NextIndexBuf) != "" || len(shaValue) != 64 {
			return errors.New("archive media finished checkpoint invalid")
		}
	} else if shaValue != "" {
		return errors.New("archive media unfinished checkpoint hash invalid")
	}
	return s.archiveMediaLeaseMutation(ctx, `
		UPDATE mochat_go_archive_media_objects
		SET index_buf=NULLIF(?,''),bytes_received=?,checkpoint_attempt=?,download_finished=?,download_sha256=?,
		    heartbeat_at=?,lease_expires_at=?,updated_at=?
		WHERE id=? AND status='fetching' AND attempt=? AND lease_token=? AND bytes_received <= ?
	`, strings.TrimSpace(value.NextIndexBuf), value.BytesReceived, value.Attempt, value.Finished, shaValue,
		at, at.Add(archiveMediaLeaseDuration), at,
		value.ID, value.Attempt, strings.TrimSpace(value.LeaseToken), value.BytesReceived)
}

func (s *MySQLStore) CompleteArchiveMedia(ctx context.Context, value archiveprovider.ArchiveMediaCompletion, at time.Time) error {
	if value.BytesReceived <= 0 || len(strings.TrimSpace(value.SHA256)) != 64 || strings.TrimSpace(value.StoragePath) == "" {
		return errors.New("archive media completion invalid")
	}
	return s.archiveMediaLeaseMutation(ctx, `
		UPDATE mochat_go_archive_media_objects
		SET status='ready',index_buf=NULL,bytes_received=?,sha256=?,storage_path=?,lease_token='',lease_expires_at=NULL,
		    heartbeat_at=?,last_error_code='',last_error='',completed_at=?,updated_at=?
		WHERE id=? AND status='fetching' AND attempt=? AND lease_token=?
		  AND download_finished=1 AND download_sha256=?
	`, value.BytesReceived, strings.ToLower(strings.TrimSpace(value.SHA256)), value.StoragePath, at, at, at,
		value.ID, value.Attempt, strings.TrimSpace(value.LeaseToken), strings.ToLower(strings.TrimSpace(value.SHA256)))
}

func (s *MySQLStore) FailArchiveMedia(ctx context.Context, value archiveprovider.ArchiveMediaFailure, at time.Time) error {
	return s.finishArchiveMedia(ctx, value, archiveprovider.ArchiveMediaFailed, at)
}

func (s *MySQLStore) MarkArchiveMediaMissing(ctx context.Context, value archiveprovider.ArchiveMediaFailure, at time.Time) error {
	return s.finishArchiveMedia(ctx, value, archiveprovider.ArchiveMediaMissing, at)
}

func (s *MySQLStore) MarkArchiveMediaCorrupt(ctx context.Context, value archiveprovider.ArchiveMediaFailure, at time.Time) error {
	return s.finishArchiveMedia(ctx, value, archiveprovider.ArchiveMediaCorrupt, at)
}

func (s *MySQLStore) ArchiveMediaAttemptReferences(ctx context.Context) ([]archiveprovider.ArchiveMediaAttemptReference, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("archive media store unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,status,checkpoint_attempt,attempt AS snapshot_attempt,
		       CASE WHEN status='fetching' THEN attempt ELSE 0 END AS active_attempt
		FROM mochat_go_archive_media_objects
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]archiveprovider.ArchiveMediaAttemptReference, 0)
	for rows.Next() {
		var item archiveprovider.ArchiveMediaAttemptReference
		if err := rows.Scan(&item.ID, &item.Status, &item.CheckpointAttempt, &item.SnapshotAttempt, &item.ActiveAttempt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *MySQLStore) finishArchiveMedia(ctx context.Context, value archiveprovider.ArchiveMediaFailure, status archiveprovider.ArchiveMediaStatus, at time.Time) error {
	code := strings.TrimSpace(value.ErrorCode)
	if code == "" {
		code = "archive.media_failed"
	}
	return s.archiveMediaLeaseMutation(ctx, `
		UPDATE mochat_go_archive_media_objects
		SET status=?,lease_token='',lease_expires_at=NULL,heartbeat_at=?,last_error_code=?,last_error='',updated_at=?
		WHERE id=? AND status='fetching' AND attempt=? AND lease_token=?
	`, string(status), at, code, at, value.ID, value.Attempt, strings.TrimSpace(value.LeaseToken))
}

func (s *MySQLStore) archiveMediaLeaseMutation(ctx context.Context, query string, args ...any) error {
	if s == nil || s.db == nil {
		return errors.New("archive media store unavailable")
	}
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	return requireArchiveMediaRows(result)
}

func requireArchiveMediaRows(result sql.Result) error {
	if result == nil {
		return errArchiveMediaFenceRejected
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return errArchiveMediaFenceRejected
	}
	return nil
}

func archiveMediaNonNegative(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}
