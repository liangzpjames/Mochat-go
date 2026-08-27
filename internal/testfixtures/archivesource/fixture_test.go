package archivesource

import (
	"bytes"
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	_ "image/png"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/wecomarchivedemo"
)

var _ wecomarchivedemo.MediaErrorCoder = FixtureError{}

func TestFixtureMediaBytesAreDecodableProductionFormats(t *testing.T) {
	fixture, err := NewArchiveFixture()
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	ids := fixture.MediaFileIDs()
	pngBytes := fixture.ExpectedMedia(ids["image"])
	if config, format, err := image.DecodeConfig(bytes.NewReader(pngBytes)); err != nil || format != "png" || config.Width < 1 || config.Height < 1 {
		t.Fatalf("PNG config=%+v format=%q err=%v", config, format, err)
	}
	wav := fixture.ExpectedMedia(ids["voice"])
	if len(wav) < 45 || string(wav[:4]) != "RIFF" || string(wav[8:12]) != "WAVE" || string(wav[12:16]) != "fmt " || string(wav[36:40]) != "data" || int(binary.LittleEndian.Uint32(wav[40:44])) != len(wav)-44 {
		t.Fatalf("WAV is not a complete PCM container: %d bytes", len(wav))
	}
	mp4 := fixture.ExpectedMedia(ids["video"])
	if len(mp4) < 128 || !bytes.Contains(mp4[:64], []byte("ftyp")) || !bytes.Contains(mp4, []byte("moov")) || !bytes.Contains(mp4, []byte("mdat")) || !bytes.Contains(mp4, []byte(DatasetMarker)) {
		t.Fatalf("MP4 does not contain playable container boxes and dataset marker: %d bytes", len(mp4))
	}
	pdf := fixture.ExpectedMedia(ids["file"])
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.")) || !bytes.Contains(pdf, []byte("xref")) || !bytes.Contains(pdf, []byte("startxref")) || !bytes.Contains(pdf, []byte(DatasetMarker)) || !bytes.HasSuffix(pdf, []byte("%%EOF\n")) {
		t.Fatalf("PDF is not a complete marked document: %q", pdf)
	}
}

func TestFixtureMissingMediaIsAnIndependentDashboardMessage(t *testing.T) {
	fixture, err := NewArchiveFixture()
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	store, err := wecomarchivedemo.NewEvidenceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := wecomarchivedemo.NewArchiveService(fixture, fixture.PrivateKeyPEM(), store, 1, 5)
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.FetchPage(context.Background(), 7, 1)
	if err != nil || len(page.Messages) != 1 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	var message map[string]any
	if err := json.Unmarshal(page.Messages[0], &message); err != nil {
		t.Fatal(err)
	}
	imagePayload, _ := message["image"].(map[string]any)
	if message["msgtype"] != "image" || imagePayload["sdkfileid"] != fixture.MediaFileIDs()["missing"] {
		t.Fatalf("sequence 8 is not independent missing image: %s", page.Messages[0])
	}
}

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
	if rawEnvelope.ErrCode != 0 || len(rawEnvelope.ChatData) != 10 {
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
	wantTypes := []string{"text", "image", "voice", "video", "file", "link", "location", "image", "mixed", "future_archive_type"}
	if page.NextSeq != 10 || len(page.Messages) != len(wantTypes) {
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
		sdkFileID := fixture.MediaFileIDs()[msgType]
		if index == 7 {
			sdkFileID = fixture.MediaFileIDs()["missing"]
		}
		if sdkFileID != "" && msgType != "mixed" {
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
			if len(payload.Item) != 3 || payload.Item[1].Type != "image" || payload.Item[1].Image.SDKFileID != mixedID || payload.Item[1].Image.MD5Sum != hex.EncodeToString(sum[:]) {
				t.Fatalf("mixed payload does not match SDK nested image shape: %+v", payload)
			}
			mediaID := fixture.MediaFileIDs()["corrupt"]
			if mediaID == "" || payload.Item[2].Type != "image" || payload.Item[2].Image.SDKFileID != mediaID {
				t.Fatalf("mixed corrupt media contract is missing: %+v", payload)
			}
		}
	}
}

func TestFixtureChatDataUsesPerMessageAuthenticatedCiphertext(t *testing.T) {
	fixture, err := NewArchiveFixture()
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	raw, err := fixture.GetChatData(0, 2, 5)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		ChatData []struct {
			EncryptedRandomKey string `json:"encrypt_random_key"`
			EncryptedChatMsg   string `json:"encrypt_chat_msg"`
		} `json:"chatdata"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.ChatData) != 2 {
		t.Fatalf("chat data=%s err=%v", raw, err)
	}
	if envelope.ChatData[0].EncryptedRandomKey == envelope.ChatData[1].EncryptedRandomKey || envelope.ChatData[0].EncryptedChatMsg == envelope.ChatData[1].EncryptedChatMsg {
		t.Fatal("fixture reused a random key or ciphertext")
	}
	for _, item := range envelope.ChatData {
		if _, err := base64.StdEncoding.DecodeString(item.EncryptedRandomKey); err != nil {
			t.Fatalf("random key is not encrypted base64: %v", err)
		}
		if decoded, err := base64.StdEncoding.DecodeString(item.EncryptedChatMsg); err != nil || bytes.Contains(decoded, []byte(DatasetMarker)) {
			t.Fatalf("chat ciphertext is not opaque base64: %v", err)
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

func TestArchiveFixtureAppendUsesEncryptedSDKAndChunkedMediaContracts(t *testing.T) {
	fixture, err := NewArchiveFixture()
	if err != nil {
		t.Fatal(err)
	}
	message, err := fixture.Append(FixtureInput{Sequence: 11, MessageID: DatasetMarker + "-DYNAMIC-11", Type: "image", Body: []byte("deterministic-image"), FileName: "dynamic.png", MIMEType: "image/png"})
	if err != nil {
		t.Fatal(err)
	}
	store, err := wecomarchivedemo.NewEvidenceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := wecomarchivedemo.NewArchiveService(fixture, fixture.PrivateKeyPEM(), store, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.FetchPage(context.Background(), 10, 10)
	if err != nil || len(page.Messages) != 1 || !bytes.Contains(page.Messages[0], []byte(message.SDKFileID)) {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	var assembled []byte
	index := ""
	for {
		chunk, err := fixture.GetMediaData(context.Background(), message.SDKFileID, index, 5)
		if err != nil {
			t.Fatal(err)
		}
		assembled = append(assembled, chunk.Data...)
		if chunk.Finished {
			break
		}
		index = chunk.NextIndexBuf
	}
	if string(assembled) != "deterministic-image" {
		t.Fatalf("media=%q", assembled)
	}
}
