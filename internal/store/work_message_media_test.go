package store

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/dashboard"
)

func TestWorkMessageMediaProjectionBatchesAndKeepsSourceIdentity(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)
	first, second := any(map[string]any{"text": "a"}), any(map[string]any{"text": "b"})
	refs := []workMessageMediaProjection{
		{MsgID: "same", SourceIdentity: "wecom:ww-a", Content: &first},
		{MsgID: "second", SourceIdentity: "wecom:ww-a", Content: &second},
	}
	mock.ExpectQuery(regexp.QuoteMeta("FROM mochat_go_archive_media_objects")).
		WithArgs(11, 27, "same", "second").
		WillReturnRows(sqlmock.NewRows([]string{"id", "msgid", "source_identity", "media_type", "media_name", "mime_type", "size_bytes", "status", "last_error_code"}).
			AddRow("8ff7bf2d-5604-43bc-a600-3ec91d575085", "same", "wecom:ww-a", "image", "safe.png", "image/png", 8, "ready", "").
			AddRow("81596770-01df-4b51-b1dc-30f2f2379679", "same", "wecom:ww-other", "image", "wrong.png", "image/png", 8, "ready", "").
			AddRow("2b91d440-a26e-482b-ad1b-8ee6f95b284f", "second", "wecom:ww-a", "voice", "voice.wav", "audio/wav", 9, "failed", "archive.media_fetch_failed"))
	if err := store.projectWorkMessageMedia(context.Background(), 11, 27, refs); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	firstContent := first.(map[string]any)
	media, ok := firstContent["media"].(map[string]any)
	if !ok || media["id"] != "8ff7bf2d-5604-43bc-a600-3ec91d575085" || media["url"] != "/dashboard/archive/media/8ff7bf2d-5604-43bc-a600-3ec91d575085/content" {
		t.Fatalf("first media = %#v", firstContent["media"])
	}
	secondContent := second.(map[string]any)
	failed, ok := secondContent["media"].(map[string]any)
	if !ok || failed["status"] != "failed" || failed["errorCode"] != "archive.media_fetch_failed" {
		t.Fatalf("second media = %#v", secondContent["media"])
	}
	if _, exists := failed["url"]; exists {
		t.Fatalf("failed media exposed URL: %#v", failed)
	}
	if _, leaked := failed["sdkfileid"]; leaked {
		t.Fatalf("media leaked sdkfileid: %#v", failed)
	}
	if firstContent["text"] != "a" || secondContent["text"] != "b" {
		t.Fatalf("message order/content changed: %#v %#v", firstContent, secondContent)
	}
}

func TestArchiveMediaContentScopesTenantCorpSourceAndEmployees(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)
	query := "(?s)FROM mochat_go_archive_media_objects media.*INNER JOIN mochat_go_archive_message_sources source.*source.source_id=media.source_identity.*media.id=\\?.*media.tenant_id=\\?.*media.corp_id=\\?.*message.to_user_type IN \\(\\?,\\?\\).*message.work_employee_id IN \\(\\?,\\?\\)"
	mock.ExpectQuery(query).
		WithArgs("8ff7bf2d-5604-43bc-a600-3ec91d575085", 11, 27, 1, 2, 32, 31).
		WillReturnRows(sqlmock.NewRows([]string{"id", "media_type", "media_name", "mime_type", "bytes_received", "status", "storage_path", "sha256"}).
			AddRow("8ff7bf2d-5604-43bc-a600-3ec91d575085", "image", "safe.png", "image/png", 8, "ready", "archive-media/8ff7bf2d-5604-43bc-a600-3ec91d575085", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	object, found, err := store.ArchiveMediaContent(context.Background(), dashboard.ArchiveMediaContentFilter{
		ID: "8ff7bf2d-5604-43bc-a600-3ec91d575085", TenantID: 11, CorpID: 27,
		AllowedConversationTypes: []int{1, 2, 2, -1}, RestrictEmployeeIDs: true, AllowedEmployeeIDs: []int{32, 31, 31, -1},
	})
	if err != nil || !found || object.ID == "" || object.Status != "ready" {
		t.Fatalf("object=%+v found=%v err=%v", object, found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveMediaContentEmptyConversationScopeFailsWithoutQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)
	_, found, err := store.ArchiveMediaContent(context.Background(), dashboard.ArchiveMediaContentFilter{
		ID: "8ff7bf2d-5604-43bc-a600-3ec91d575085", TenantID: 11, CorpID: 27,
	})
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveMediaContentEmptyEmployeeScopeFailsWithoutQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)
	_, found, err := store.ArchiveMediaContent(context.Background(), dashboard.ArchiveMediaContentFilter{
		ID: "8ff7bf2d-5604-43bc-a600-3ec91d575085", TenantID: 11, CorpID: 27, AllowedConversationTypes: []int{0}, RestrictEmployeeIDs: true,
	})
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWorkMessageMediaProjectionEmptyPageDoesNotQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)
	if err := store.ProjectWorkMessagePageMedia(context.Background(), 11, 27, &dashboard.WorkMessagePage{}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveMediaProjectionDoesNotExposePrivateLocatorFields(t *testing.T) {
	payload := archiveMediaPayload(workMessageMediaRow{
		ID: "8ff7bf2d-5604-43bc-a600-3ec91d575085", MediaType: "video", Name: "clip.mp4",
		MIMEType: "video/mp4", Size: 9, Status: "ready", ErrorCode: "should-not-contain-a-secret",
	})
	for _, forbidden := range []string{"sdkfileid", "storage", "sourceidentity", "ciphertext"} {
		for key := range payload {
			if strings.Contains(strings.ToLower(key), forbidden) {
				t.Fatalf("private projection field %q in %#v", key, payload)
			}
		}
	}
}
