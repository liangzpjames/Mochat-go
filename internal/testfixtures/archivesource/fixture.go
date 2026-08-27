package archivesource

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"sync"
	"time"

	"jiyi/mochat-go/internal/wecomarchivedemo"
)

const DatasetMarker = "MOCHAT-LOCAL-ACCEPTANCE-20260827"

const fixtureRandomKey = DatasetMarker + "-DECRYPTED-RANDOM-KEY"

const fixtureMP4Base64 = "AAAAIGZ0eXBpc29tAAACAGlzb21pc28yYXZjMW1wNDEAAANGbW9vdgAAAGxtdmhkAAAAAAAAAAAAAAAAAAAD6AAAAMgAAQAAAQAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAjl0cmFrAAAAXHRraGQAAAADAAAAAAAAAAAAAAABAAAAAAAAAMgAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAABAAAAAABAAAAAQAAAAAAAkZWR0cwAAABxlbHN0AAAAAAAAAAEAAADIAAAAAAABAAAAAAGxbWRpYQAAACBtZGhkAAAAAAAAAAAAAAAAAAAoAAAACABVxAAAAAAALWhkbHIAAAAAAAAAAHZpZGUAAAAAAAAAAAAAAABWaWRlb0hhbmRsZXIAAAABXG1pbmYAAAAUdm1oZAAAAAEAAAAAAAAAAAAAACRkaW5mAAAAHGRyZWYAAAAAAAAAAQAAAAx1cmwgAAAAAQAAARxzdGJsAAAAuHN0c2QAAAAAAAAAAQAAAKhhdmMxAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAABAAEABIAAAASAAAAAAAAAABFUxhdmM2MS4xOS4xMDAgbGlieDI2NAAAAAAAAAAAAAAAGP//AAAALmF2Y0MBQsAK/+EAFmdCwArZHsBEAAADAAQAAAMAKDxImSABAAVoy4PLIAAAABBwYXNwAAAAAQAAAAEAAAAUYnRydAAAAAAAAGIgAABiIAAAABhzdHRzAAAAAAAAAAEAAAABAAAIAAAAABxzdHNjAAAAAAAAAAEAAAABAAAAAQAAAAEAAAAUc3RzegAAAAAAAAJ0AAAAAQAAABRzdGNvAAAAAAAAAAEAAAN2AAAAmXVkdGEAAACRbWV0YQAAAAAAAAAhaGRscgAAAAAAAAAAbWRpcmFwcGwAAAAAAAAAAAAAAABkaWxzdAAAACSpdG9vAAAAHGRhdGEAAAABAAAAAExhdmY2MS43LjEwMAAAADipY210AAAAMGRhdGEAAAABAAAAAE1PQ0hBVC1MT0NBTC1BQ0NFUFRBTkNFLTIwMjYwODI3AAAACGZyZWUAAAJ8bWRhdAAAAmIGBf//XtxF6b3m2Ui3lizYINkj7u94MjY0IC0gY29yZSAxNjQgLSBILjI2NC9NUEVHLTQgQVZDIGNvZGVjIC0gQ29weWxlZnQgMjAwMy0yMDIzIC0gaHR0cDovL3d3dy52aWRlb2xhbi5vcmcveDI2NC5odG1sIC0gb3B0aW9uczogY2FiYWM9MCByZWY9MyBkZWJsb2NrPTE6MDowIGFuYWx5c2U9MHgxOjB4MTExIG1lPWhleCBzdWJtZT03IHBzeT0xIHBzeV9yZD0xLjAwOjAuMDAgbWl4ZWRfcmVmPTEgbWVfcmFuZ2U9MTYgY2hyb21hX21lPTEgdHJlbGxpcz0xIDh4OGRjdD0wIGNxbT0wIGRlYWR6b25lPTIxLDExIGZhc3RfcHNraXA9MSBjaHJvbWFfcXBfb2Zmc2V0PS0yIHRocmVhZHM9MSBsb29rYWhlYWRfdGhyZWFkcz0xIHNsaWNlZF90aHJlYWRzPTAgbnI9MCBkZWNpbWF0ZT0xIGludGVybGFjZWQ9MCBibHVyYXlfY29tcGF0PTAgY29uc3RyYWluZWRfaW50cmE9MCBiZnJhbWVzPTAgd2VpZ2h0cD0wIGtleWludD0yNTAga2V5aW50X21pbj01IHNjZW5lY3V0PTQwIGludHJhX3JlZnJlc2g9MCByY19sb29rYWhlYWQ9NDAgcmM9Y3JmIG1idHJlZT0xIGNyZj0yMy4wIHFjb21wPTAuNjAgcXBtaW49MCBxcG1heD02OSBxcHN0ZXA9NCBpcF9yYXRpbz0xLjQwIGFxPTE6MS4wMACAAAAACmWIhA/yYoAAw+4="

type MediaMode string

const (
	MediaAvailable   MediaMode = "available"
	MediaMissing     MediaMode = "missing"
	MediaCorrupt     MediaMode = "corrupt"
	MediaSDKError    MediaMode = "sdk_error"
	MediaInterrupted MediaMode = "interrupted"
)

type FixtureError struct {
	Code string
}

func (e FixtureError) Error() string {
	return "local archive fixture failure: " + e.Code
}

func (e FixtureError) MediaErrorCode() string { return e.Code }

type fixtureMessage struct {
	seq        uint64
	msgID      string
	ciphertext string
	plain      []byte
}

type ArchiveFixture struct {
	mu                 sync.Mutex
	privateKeyPEM      string
	encryptedRandomKey string
	messages           []fixtureMessage
	mediaFileIDs       map[string]string
	media              map[string][]byte
	mediaModes         map[string]MediaMode
	mediaChunkDelay    time.Duration
	closed             bool
}

func NewArchiveFixture() (*ArchiveFixture, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	encryptedRandomKey, err := rsa.EncryptPKCS1v15(rand.Reader, &privateKey.PublicKey, []byte(fixtureRandomKey))
	if err != nil {
		return nil, err
	}
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	mediaFileIDs := map[string]string{
		"image":   DatasetMarker + "-SDKFILE-IMAGE",
		"voice":   DatasetMarker + "-SDKFILE-VOICE",
		"video":   DatasetMarker + "-SDKFILE-VIDEO",
		"file":    DatasetMarker + "-SDKFILE-FILE",
		"mixed":   DatasetMarker + "-SDKFILE-MIXED-IMAGE",
		"missing": DatasetMarker + "-SDKFILE-MISSING-IMAGE",
		"corrupt": DatasetMarker + "-SDKFILE-CORRUPT-IMAGE",
	}
	videoBytes, err := base64.StdEncoding.DecodeString(fixtureMP4Base64)
	if err != nil {
		return nil, fmt.Errorf("decode deterministic acceptance MP4: %w", err)
	}
	media := map[string][]byte{
		mediaFileIDs["image"]:   fixturePNG(color.NRGBA{R: 29, G: 78, B: 216, A: 255}),
		mediaFileIDs["voice"]:   fixtureWAV(),
		mediaFileIDs["video"]:   videoBytes,
		mediaFileIDs["file"]:    fixturePDF(),
		mediaFileIDs["mixed"]:   fixturePNG(color.NRGBA{R: 20, G: 184, B: 166, A: 255}),
		mediaFileIDs["missing"]: fixturePNG(color.NRGBA{R: 245, G: 158, B: 11, A: 255}),
		mediaFileIDs["corrupt"]: fixturePNG(color.NRGBA{R: 239, G: 68, B: 68, A: 255}),
	}
	plainMessages := []map[string]any{
		baseMessage(1, "text", map[string]any{"content": DatasetMarker + " local contract text"}),
		baseMessage(2, "image", map[string]any{"sdkfileid": mediaFileIDs["image"], "md5sum": mediaMD5(media[mediaFileIDs["image"]]), "filesize": len(media[mediaFileIDs["image"]])}),
		baseMessage(3, "voice", map[string]any{"sdkfileid": mediaFileIDs["voice"], "voice_size": len(media[mediaFileIDs["voice"]]), "play_length": 2, "md5sum": mediaMD5(media[mediaFileIDs["voice"]])}),
		baseMessage(4, "video", map[string]any{"sdkfileid": mediaFileIDs["video"], "filesize": len(media[mediaFileIDs["video"]]), "play_length": 3, "md5sum": mediaMD5(media[mediaFileIDs["video"]])}),
		baseMessage(5, "file", map[string]any{"sdkfileid": mediaFileIDs["file"], "filename": DatasetMarker + "-fixture.pdf", "fileext": "pdf", "filesize": len(media[mediaFileIDs["file"]]), "md5sum": mediaMD5(media[mediaFileIDs["file"]])}),
		baseMessage(6, "link", map[string]any{"title": DatasetMarker + " local link", "description": "local contract only", "link_url": "https://example.invalid/mochat-local-acceptance", "image_url": "https://example.invalid/local-image.png"}),
		baseMessage(7, "location", map[string]any{"longitude": 121.4737, "latitude": 31.2304, "address": DatasetMarker + " local location", "title": "local contract", "zoom": 16}),
		baseMessage(8, "image", map[string]any{"sdkfileid": mediaFileIDs["missing"], "md5sum": mediaMD5(media[mediaFileIDs["missing"]]), "filesize": len(media[mediaFileIDs["missing"]])}),
		baseMessage(9, "mixed", map[string]any{"item": []map[string]any{
			{"type": "text", "content": DatasetMarker + " mixed text"},
			{"type": "image", "image": map[string]any{
				"sdkfileid": mediaFileIDs["mixed"], "md5sum": mediaMD5(media[mediaFileIDs["mixed"]]), "filesize": len(media[mediaFileIDs["mixed"]]),
			}},
			{"type": "image", "image": map[string]any{
				"sdkfileid": mediaFileIDs["corrupt"], "md5sum": mediaMD5(media[mediaFileIDs["corrupt"]]), "filesize": len(media[mediaFileIDs["corrupt"]]),
			}},
		}}),
		baseMessage(10, "future_archive_type", map[string]any{"opaque": DatasetMarker + " preserve unknown payload", "version": 1}),
	}
	messages := make([]fixtureMessage, 0, len(plainMessages))
	for index, message := range plainMessages {
		plain, err := json.Marshal(message)
		if err != nil {
			return nil, err
		}
		seq := uint64(index + 1)
		messages = append(messages, fixtureMessage{
			seq: seq, msgID: fmt.Sprintf("%s-MSG-%02d", DatasetMarker, seq),
			ciphertext: fmt.Sprintf("%s-CIPHER-%02d", DatasetMarker, seq), plain: plain,
		})
	}
	return &ArchiveFixture{
		privateKeyPEM: string(privateKeyPEM), encryptedRandomKey: base64.StdEncoding.EncodeToString(encryptedRandomKey),
		messages: messages, mediaFileIDs: mediaFileIDs, media: media, mediaModes: map[string]MediaMode{
			mediaFileIDs["missing"]: MediaMissing,
			mediaFileIDs["corrupt"]: MediaCorrupt,
		},
	}, nil
}

func fixturePNG(fill color.NRGBA) []byte {
	canvas := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			canvas.SetNRGBA(x, y, fill)
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, canvas); err != nil {
		panic(err)
	}
	return append(encoded.Bytes(), []byte(DatasetMarker)...)
}

func fixtureWAV() []byte {
	const sampleRate = 8000
	pcm := make([]byte, 160)
	for index := range pcm {
		if (index/10)%2 == 0 {
			pcm[index] = 64
		} else {
			pcm[index] = 192
		}
	}
	result := make([]byte, 44+len(pcm))
	copy(result[0:4], "RIFF")
	binary.LittleEndian.PutUint32(result[4:8], uint32(len(result)-8))
	copy(result[8:12], "WAVE")
	copy(result[12:16], "fmt ")
	binary.LittleEndian.PutUint32(result[16:20], 16)
	binary.LittleEndian.PutUint16(result[20:22], 1)
	binary.LittleEndian.PutUint16(result[22:24], 1)
	binary.LittleEndian.PutUint32(result[24:28], sampleRate)
	binary.LittleEndian.PutUint32(result[28:32], sampleRate)
	binary.LittleEndian.PutUint16(result[32:34], 1)
	binary.LittleEndian.PutUint16(result[34:36], 8)
	copy(result[36:40], "data")
	binary.LittleEndian.PutUint32(result[40:44], uint32(len(pcm)))
	copy(result[44:], pcm)
	return result
}

func fixturePDF() []byte {
	content := "BT /F1 12 Tf 36 72 Td (" + DatasetMarker + " local acceptance PDF) Tj ET\n"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 120] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content),
	}
	var document bytes.Buffer
	document.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = document.Len()
		fmt.Fprintf(&document, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xref := document.Len()
	fmt.Fprintf(&document, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for index := 1; index < len(offsets); index++ {
		fmt.Fprintf(&document, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(&document, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return document.Bytes()
}

func mediaMD5(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

func baseMessage(seq int, msgType string, payload any) map[string]any {
	msgID := fmt.Sprintf("%s-MSG-%02d", DatasetMarker, seq)
	return map[string]any{
		"msgid": msgID, "action": "send", "from": DatasetMarker + "-STAFF-01",
		"tolist": []string{DatasetMarker + "-EXTERNAL-01"}, "roomid": "",
		"msgtime": int64(1787760000000 + seq), "msgtype": msgType, msgType: payload,
	}
}

func (f *ArchiveFixture) PrivateKeyPEM() string {
	return f.privateKeyPEM
}

func (f *ArchiveFixture) MediaFileIDs() map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make(map[string]string, len(f.mediaFileIDs))
	for mediaType, sdkFileID := range f.mediaFileIDs {
		result[mediaType] = sdkFileID
	}
	return result
}

func (f *ArchiveFixture) ExpectedMedia(sdkFileID string) []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]byte(nil), f.media[sdkFileID]...)
}

func (f *ArchiveFixture) SetMediaMode(sdkFileID string, mode MediaMode) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mediaModes[sdkFileID] = mode
}

func (f *ArchiveFixture) SetMediaChunkDelay(delay time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if delay < 0 {
		delay = 0
	}
	f.mediaChunkDelay = delay
}

func (f *ArchiveFixture) GetChatData(seq uint64, limit uint32, timeoutSeconds int) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil, errors.New("local archive fixture is closed")
	}
	if timeoutSeconds <= 0 || limit == 0 {
		return nil, errors.New("invalid local archive fixture request")
	}
	chatData := make([]map[string]any, 0, limit)
	for _, message := range f.messages {
		if message.seq <= seq {
			continue
		}
		chatData = append(chatData, map[string]any{
			"seq": message.seq, "msgid": message.msgID, "publickey_ver": 1,
			"encrypt_random_key": f.encryptedRandomKey, "encrypt_chat_msg": message.ciphertext,
		})
		if len(chatData) == int(limit) {
			break
		}
	}
	return json.Marshal(map[string]any{"errcode": 0, "errmsg": "ok", "chatdata": chatData})
}

func (f *ArchiveFixture) DecryptData(randomKey, encryptedMessage string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil, errors.New("local archive fixture is closed")
	}
	if randomKey != fixtureRandomKey {
		return nil, errors.New("local archive fixture random key mismatch")
	}
	for _, message := range f.messages {
		if message.ciphertext == encryptedMessage {
			return append([]byte(nil), message.plain...), nil
		}
	}
	return nil, errors.New("local archive fixture ciphertext not found")
}

func (f *ArchiveFixture) GetMediaData(ctx context.Context, sdkFileID, indexBuf string, timeoutSeconds int) (wecomarchivedemo.MediaChunk, error) {
	if ctx == nil {
		return wecomarchivedemo.MediaChunk{}, errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return wecomarchivedemo.MediaChunk{}, err
	}
	f.mu.Lock()
	delay := f.mediaChunkDelay
	f.mu.Unlock()
	// The acceptance delay holds the second request only. This leaves a
	// durable first-chunk checkpoint observable without making every remaining
	// deterministic fixture chunk artificially slow.
	if delay > 0 && indexBuf == "7" {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return wecomarchivedemo.MediaChunk{}, ctx.Err()
		case <-timer.C:
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return wecomarchivedemo.MediaChunk{}, errors.New("local archive fixture is closed")
	}
	if timeoutSeconds <= 0 {
		return wecomarchivedemo.MediaChunk{}, errors.New("invalid local archive fixture timeout")
	}
	data, ok := f.media[sdkFileID]
	if !ok || f.mediaModes[sdkFileID] == MediaMissing {
		return wecomarchivedemo.MediaChunk{}, FixtureError{Code: "MEDIA_MISSING"}
	}
	if f.mediaModes[sdkFileID] == MediaSDKError {
		return wecomarchivedemo.MediaChunk{}, FixtureError{Code: "MEDIA_SDK_ERROR"}
	}
	if f.mediaModes[sdkFileID] == MediaInterrupted {
		return wecomarchivedemo.MediaChunk{}, context.Canceled
	}
	if f.mediaModes[sdkFileID] == MediaCorrupt {
		data = append([]byte(nil), data...)
		if len(data) > 0 {
			data[0] ^= 0xff
		}
	}
	offset := 0
	if indexBuf != "" {
		var err error
		offset, err = strconv.Atoi(indexBuf)
		if err != nil || offset < 0 || offset >= len(data) {
			return wecomarchivedemo.MediaChunk{}, FixtureError{Code: "MEDIA_INDEX_INVALID"}
		}
	}
	end := offset + 7
	if end > len(data) {
		end = len(data)
	}
	finished := end == len(data)
	nextIndexBuf := ""
	if !finished {
		nextIndexBuf = strconv.Itoa(end)
	}
	return wecomarchivedemo.MediaChunk{Data: append([]byte(nil), data[offset:end]...), NextIndexBuf: nextIndexBuf, Finished: finished}, nil
}

func (f *ArchiveFixture) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// PrepareDashboardPermissionDependencies creates the smallest real 0127
// permission catalog needed by 0133's staff-resource upsert. It deliberately
// keeps the production table names, keys, and foreign key so archive
// integration schemas exercise the same dependency instead of bypassing it.
func PrepareDashboardPermissionDependencies(ctx context.Context, db execer) error {
	for _, statement := range []string{
		`CREATE TABLE mochat_go_dashboard_permissions (
  id bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  code varchar(96) NOT NULL,
  permission_type varchar(16) NOT NULL DEFAULT 'page',
  path varchar(191) NOT NULL,
  name varchar(100) NOT NULL,
  group_code varchar(64) DEFAULT NULL,
  sort int NOT NULL DEFAULT 0,
  restriction varchar(32) NOT NULL DEFAULT 'grantable',
  superadmin_only tinyint(1) NOT NULL DEFAULT 0,
  status tinyint NOT NULL DEFAULT 1,
  version bigint(20) unsigned NOT NULL DEFAULT 1,
  created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted_at timestamp NULL DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uni_dashboard_permissions_code (code),
  UNIQUE KEY uni_dashboard_permissions_path (path)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE mochat_go_dashboard_permission_resources (
  id bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  permission_id bigint(20) unsigned NOT NULL,
  resource_type varchar(16) NOT NULL DEFAULT 'api',
  http_method varchar(10) NOT NULL,
  path_pattern varchar(191) NOT NULL,
  scope_required tinyint(1) NOT NULL DEFAULT 0,
  status tinyint NOT NULL DEFAULT 1,
  version bigint(20) unsigned NOT NULL DEFAULT 1,
  created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted_at timestamp NULL DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uni_dashboard_permission_resource (http_method, path_pattern, permission_id),
  KEY idx_dashboard_permission_resource_match (http_method, path_pattern, status),
  CONSTRAINT fk_dashboard_permission_resource_permission FOREIGN KEY (permission_id) REFERENCES mochat_go_dashboard_permissions (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`INSERT INTO mochat_go_dashboard_permissions (id,code,permission_type,path,name,group_code,sort,restriction,superadmin_only,status,version) VALUES (1,'dashboard.company_setting.staff','page','/company-setting/staff','Staff permissions','company-settings',50,'superadmin_only',1,1,1)`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
