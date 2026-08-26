package store

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
	archiveprovider "jiyi/mochat-go/internal/modules/providers/archive"
	"jiyi/mochat-go/internal/wecomcredentials"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestArchiveMediaClaimDecryptsLocatorAndFencesStaleWorker(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager := archiveMediaTestCipher(t)
	id := "014c1da7-1b2e-4aa1-90aa-a6a0d6f53380"
	ciphertext, keyID, err := manager.EncryptArchiveMedia(11, id, wecomcredentials.ArchiveMediaCredential{SDKFileID: "private-sdk-id"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 27, 3, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT media\\.id,media\\.tenant_id").WithArgs(now).WillReturnRows(sqlmock.NewRows([]string{
		"id", "tenant_id", "corp_id", "wx_corpid", "msgid", "source_identity", "ciphertext", "key_id",
		"media_type", "media_name", "mime_type", "expected_size_bytes", "expected_md5", "status", "index_buf", "bytes_received",
		"checkpoint_attempt", "download_finished", "download_sha256", "attempt",
	}).AddRow(id, 11, 27, "ww-local", "msg-1", "wecom:ww-local", ciphertext, keyID, "image", "", "image/png", 21, "", "fetching", "index-4", 28, 4, false, "", 4))
	mock.ExpectExec("UPDATE mochat_go_archive_media_objects").WithArgs(5, sqlmock.AnyArg(), now.Add(archiveMediaLeaseDuration), now, now, id, now).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	store := NewMySQLStore(db).WithWeComCredentialCipher(manager)
	object, found, err := store.ClaimArchiveMedia(context.Background(), now)
	if err != nil || !found || object.SDKFileID != "private-sdk-id" || object.Attempt != 5 || object.LeaseToken == "" || object.IndexBuf != "index-4" || object.BytesReceived != 28 || object.CheckpointAttempt != 4 {
		t.Fatalf("object=%#v found=%v err=%v", object, found, err)
	}
	mock.ExpectExec("UPDATE mochat_go_archive_media_objects SET heartbeat_at").
		WithArgs(now, now.Add(archiveMediaLeaseDuration), now, id, 0, "stale-token").
		WillReturnResult(sqlmock.NewResult(0, 0))
	if err := store.RenewArchiveMedia(context.Background(), id, 0, "stale-token", now); !errors.Is(err, errArchiveMediaFenceRejected) {
		t.Fatalf("stale renew error=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveMediaMutationsCarryAttemptAndTokenFence(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)
	now := time.Date(2026, 8, 27, 3, 0, 0, 0, time.UTC)
	id, token := "014c1da7-1b2e-4aa1-90aa-a6a0d6f53380", "lease-token"
	mock.ExpectExec("SET index_buf=NULLIF\\(\\?,''\\),bytes_received=\\?,checkpoint_attempt=\\?,download_finished=\\?,download_sha256=\\?").
		WithArgs("next", int64(7), 3, false, "", now, now.Add(archiveMediaLeaseDuration), now, id, 3, token, int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.CheckpointArchiveMedia(context.Background(), archiveprovider.ArchiveMediaCheckpoint{ID: id, Attempt: 3, LeaseToken: token, NextIndexBuf: "next", BytesReceived: 7}, now); err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec("SET index_buf=NULLIF\\(\\?,''\\),bytes_received=\\?,checkpoint_attempt=\\?,download_finished=\\?,download_sha256=\\?").
		WithArgs("", int64(7), 3, true, strings.Repeat("a", 64), now, now.Add(archiveMediaLeaseDuration), now, id, 3, token, int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.CheckpointArchiveMedia(context.Background(), archiveprovider.ArchiveMediaCheckpoint{ID: id, Attempt: 3, LeaseToken: token, BytesReceived: 7, Finished: true, SHA256: strings.Repeat("a", 64)}, now); err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec("SET status='ready'").WithArgs(int64(7), strings.Repeat("a", 64), "safe-path", now, now, now, id, 3, token, strings.Repeat("a", 64)).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.CompleteArchiveMedia(context.Background(), archiveprovider.ArchiveMediaCompletion{ID: id, Attempt: 3, LeaseToken: token, BytesReceived: 7, SHA256: strings.Repeat("a", 64), StoragePath: "safe-path"}, now); err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec("SET status=\\?,lease_token=''").WithArgs("corrupt", now, "archive.media_integrity_mismatch", now, id, 3, token).WillReturnResult(sqlmock.NewResult(0, 0))
	if err := store.MarkArchiveMediaCorrupt(context.Background(), archiveprovider.ArchiveMediaFailure{ID: id, Attempt: 3, LeaseToken: token, ErrorCode: "archive.media_integrity_mismatch"}, now); !errors.Is(err, errArchiveMediaFenceRejected) {
		t.Fatalf("stale complete error=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveMessageMediaOutboxUsesSameTransactionAndDuplicateCannotRegressReady(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager := archiveMediaTestCipher(t)
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	insertPattern := "(?s)INSERT INTO mochat_go_archive_media_objects.*ON DUPLICATE KEY UPDATE id = id"
	mock.ExpectExec(insertPattern).WillReturnResult(sqlmock.NewResult(1, 1))
	message := archiveprovider.Message{Source: providers.SourceExternal, SourceID: "wecom:ww-local", Namespace: "wecom:ww-local", MsgID: "msg-1", Media: []archiveprovider.MediaDescriptor{{Type: "image", SDKFileID: "private-sdk-id", ExpectedSize: 21}}}
	if err := upsertArchiveMediaTx(context.Background(), tx, manager, archiveprovider.Scope{TenantID: 11, CorpID: 27}, message); err != nil {
		t.Fatal(err)
	}
	mock.ExpectCommit()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

}

func archiveMediaTestCipher(t *testing.T) *wecomcredentials.Manager {
	t.Helper()
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	manager, err := wecomcredentials.NewManager(wecomcredentials.Config{EncryptionKey: key, EncryptionKeyID: "media-v1", RequireEncryption: true})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func TestDurableArchiveBindingsAndCursorStayTenantScoped(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)
	mock.ExpectQuery("(?s)SELECT integration\\.tenant_id,integration\\.corp_id,integration\\.verified_wx_corpid.*integration\\.status='active'.*integration\\.verified_at IS NOT NULL.*binding\\.status=2 AND binding\\.verified_at IS NOT NULL.*JSON_CONTAINS\\(integration\\.scope_json, JSON_QUOTE\\('archive\\.read'\\)\\).*JSON_LENGTH\\(integration\\.missing_capabilities_json\\) = 0").
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "corp_id", "verified_wx_corpid"}).AddRow(11, 27, "ww-local"))
	bindings, err := store.DurableArchiveBindings(context.Background())
	if err != nil || len(bindings) != 1 || bindings[0].Scope.TenantID != 11 || bindings[0].Scope.CorpID != 27 || bindings[0].WXCorpID != "ww-local" {
		t.Fatalf("bindings=%#v err=%v", bindings, err)
	}
	mock.ExpectQuery("SELECT cursor_sequence,cursor_token").WithArgs(int64(11), int64(27), "wecom:ww-local").
		WillReturnRows(sqlmock.NewRows([]string{"cursor_sequence", "cursor_token"}).AddRow(42, "opaque"))
	cursor, err := store.LatestArchiveSyncCursor(context.Background(), archiveprovider.Scope{TenantID: 11, CorpID: 27}, "wecom:ww-local")
	if err != nil || cursor.Sequence != 42 || cursor.Token != "opaque" {
		t.Fatalf("cursor=%#v err=%v", cursor, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
