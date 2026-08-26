package archivesource

import (
	"bytes"
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/wecomarchivedemo"
)

var _ wecomarchivedemo.MediaErrorCoder = FixtureError{}

type recordingExecutor struct {
	statements []string
}

func TestFixtureArchiveMessagesUseProductionSDKShapesAndPreserveUnknown(t *testing.T) {
	fixture, err := NewArchiveFixture()
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	var _ wecomarchivedemo.FinanceSDK = fixture

	envelope, err := fixture.GetChatData(0, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	var rawEnvelope struct {
		ErrCode  int `json:"errcode"`
		ChatData []struct {
			Seq                uint64 `json:"seq"`
			MsgID              string `json:"msgid"`
			PublicKeyVersion   uint32 `json:"publickey_ver"`
			EncryptedRandomKey string `json:"encrypt_random_key"`
			EncryptedChatMsg   string `json:"encrypt_chat_msg"`
		} `json:"chatdata"`
	}
	if err := json.Unmarshal(envelope, &rawEnvelope); err != nil {
		t.Fatal(err)
	}
	if rawEnvelope.ErrCode != 0 || len(rawEnvelope.ChatData) != 9 {
		t.Fatalf("envelope=%s", envelope)
	}
	for _, item := range rawEnvelope.ChatData {
		if item.Seq == 0 || item.PublicKeyVersion == 0 || item.EncryptedRandomKey == "" || item.EncryptedChatMsg == "" || !strings.Contains(item.MsgID, DatasetMarker) {
			t.Fatalf("invalid SDK envelope item: %+v", item)
		}
	}

	store, err := wecomarchivedemo.NewEvidenceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := wecomarchivedemo.NewArchiveService(fixture, fixture.PrivateKeyPEM(), store, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.FetchPage(context.Background(), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	wantTypes := []string{"text", "image", "voice", "video", "file", "link", "location", "mixed", "future_archive_type"}
	if page.NextSeq != 9 || len(page.Messages) != len(wantTypes) {
		t.Fatalf("page=%+v", page)
	}
	for index, raw := range page.Messages {
		var message map[string]json.RawMessage
		if err := json.Unmarshal(raw, &message); err != nil {
			t.Fatal(err)
		}
		var msgType, msgID string
		if err := json.Unmarshal(message["msgtype"], &msgType); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(message["msgid"], &msgID); err != nil {
			t.Fatal(err)
		}
		if msgType != wantTypes[index] || !strings.Contains(msgID, DatasetMarker) {
			t.Fatalf("message[%d]=%s", index, raw)
		}
		if _, ok := message[msgType]; !ok {
			t.Fatalf("message type payload %q was not preserved: %s", msgType, raw)
		}
		if sdkFileID := fixture.MediaFileIDs()[msgType]; sdkFileID != "" && msgType != "mixed" {
			var payload struct {
				SDKFileID string `json:"sdkfileid"`
				MD5Sum    string `json:"md5sum"`
			}
			if err := json.Unmarshal(message[msgType], &payload); err != nil {
				t.Fatal(err)
			}
			sum := md5.Sum(fixture.ExpectedMedia(sdkFileID))
			if payload.SDKFileID != sdkFileID || payload.MD5Sum != hex.EncodeToString(sum[:]) {
				t.Fatalf("%s payload has inconsistent media contract: %+v", msgType, payload)
			}
		}
		if msgType == "mixed" {
			var payload struct {
				Item []struct {
					Type  string `json:"type"`
					Image struct {
						SDKFileID string `json:"sdkfileid"`
						MD5Sum    string `json:"md5sum"`
					} `json:"image"`
				} `json:"item"`
			}
			if err := json.Unmarshal(message[msgType], &payload); err != nil {
				t.Fatal(err)
			}
			mixedID := fixture.MediaFileIDs()["mixed"]
			sum := md5.Sum(fixture.ExpectedMedia(mixedID))
			if len(payload.Item) != 4 || payload.Item[1].Type != "image" || payload.Item[1].Image.SDKFileID != mixedID || payload.Item[1].Image.MD5Sum != hex.EncodeToString(sum[:]) {
				t.Fatalf("mixed payload does not match SDK nested image shape: %+v", payload)
			}
			for index, kind := range []string{"missing", "corrupt"} {
				mediaID := fixture.MediaFileIDs()[kind]
				if mediaID == "" || payload.Item[index+2].Type != "image" || payload.Item[index+2].Image.SDKFileID != mediaID {
					t.Fatalf("mixed %s media contract is missing: %+v", kind, payload)
				}
			}
		}
	}
}

func TestFixtureMixedFaultMediaDefaultsToMissingAndCorrupt(t *testing.T) {
	fixture, err := NewArchiveFixture()
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()

	ids := fixture.MediaFileIDs()
	_, err = fixture.GetMediaData(context.Background(), ids["missing"], "", 5)
	var fixtureErr FixtureError
	if !errors.As(err, &fixtureErr) || fixtureErr.Code != "MEDIA_MISSING" {
		t.Fatalf("missing media error=%v", err)
	}
	corrupt, err := fixture.GetMediaData(context.Background(), ids["corrupt"], "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(corrupt.Data, fixture.ExpectedMedia(ids["corrupt"])[:len(corrupt.Data)]) {
		t.Fatal("corrupt media returned canonical bytes")
	}
}

func TestFixtureMediaIsDeterministicReplayableAndSevenByteChunked(t *testing.T) {
	fixture, err := NewArchiveFixture()
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	sdkFileID := fixture.MediaFileIDs()["image"]
	want := fixture.ExpectedMedia(sdkFileID)
	if len(want) < 15 {
		t.Fatalf("fixture media needs at least three chunks: %d bytes", len(want))
	}
	var got []byte
	indexBuf := ""
	chunks := 0
	for {
		chunk, err := fixture.GetMediaData(context.Background(), sdkFileID, indexBuf, 5)
		if err != nil {
			t.Fatal(err)
		}
		chunks++
		if len(chunk.Data) > 7 {
			t.Fatalf("chunk length=%d", len(chunk.Data))
		}
		got = append(got, chunk.Data...)
		if chunk.Finished {
			if chunk.NextIndexBuf != "" {
				t.Fatalf("finished chunk retained index %q", chunk.NextIndexBuf)
			}
			break
		}
		if chunk.NextIndexBuf == "" || chunk.NextIndexBuf == indexBuf {
			t.Fatalf("non-progressing index %q", chunk.NextIndexBuf)
		}
		indexBuf = chunk.NextIndexBuf
	}
	if chunks < 3 || !bytes.Equal(got, want) {
		t.Fatalf("chunks=%d got=%x want=%x", chunks, got, want)
	}
	replay, err := fixture.GetMediaData(context.Background(), sdkFileID, "", 5)
	if err != nil || !bytes.Equal(replay.Data, want[:7]) {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
}

func TestFixtureMediaFaultsAreInjectableWithoutLeakingSDKFileID(t *testing.T) {
	fixture, err := NewArchiveFixture()
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	sdkFileID := fixture.MediaFileIDs()["voice"]
	for _, test := range []struct {
		mode MediaMode
		code string
	}{
		{mode: MediaMissing, code: "MEDIA_MISSING"},
		{mode: MediaSDKError, code: "MEDIA_SDK_ERROR"},
	} {
		fixture.SetMediaMode(sdkFileID, test.mode)
		_, err := fixture.GetMediaData(context.Background(), sdkFileID, "", 5)
		var fixtureErr FixtureError
		if !errors.As(err, &fixtureErr) || fixtureErr.Code != test.code || strings.Contains(err.Error(), sdkFileID) {
			t.Fatalf("mode=%s err=%v", test.mode, err)
		}
	}
	fixture.SetMediaMode(sdkFileID, MediaCorrupt)
	chunk, err := fixture.GetMediaData(context.Background(), sdkFileID, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(chunk.Data, fixture.ExpectedMedia(sdkFileID)[:len(chunk.Data)]) {
		t.Fatal("corrupt mode returned canonical media bytes")
	}
}

func TestFixtureMediaCanInjectInterruptedSDKCall(t *testing.T) {
	fixture, err := NewArchiveFixture()
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	sdkFileID := fixture.MediaFileIDs()["video"]
	fixture.SetMediaMode(sdkFileID, MediaInterrupted)
	_, err = fixture.GetMediaData(context.Background(), sdkFileID, "", 5)
	if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), sdkFileID) {
		t.Fatalf("interrupted media error=%v", err)
	}
}

func (r *recordingExecutor) ExecContext(_ context.Context, statement string, _ ...any) (sql.Result, error) {
	r.statements = append(r.statements, statement)
	return nil, nil
}

func TestPrepareDashboardPermissionDependenciesCreatesThe0127TablesAndStaffResource(t *testing.T) {
	executor := &recordingExecutor{}
	if err := PrepareDashboardPermissionDependencies(context.Background(), executor); err != nil {
		t.Fatal(err)
	}
	joined := strings.ToLower(strings.Join(executor.statements, "\n"))
	for _, fragment := range []string{
		"create table mochat_go_dashboard_permissions",
		"create table mochat_go_dashboard_permission_resources",
		"dashboard.company_setting.staff",
		"uni_dashboard_permission_resource",
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("fixture SQL missing %q: %s", fragment, joined)
		}
	}
}
