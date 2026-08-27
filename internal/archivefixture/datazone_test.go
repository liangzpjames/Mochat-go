package archivefixture

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDataZoneProviderFetchesMetadataAndRequiresDecryptedSecret(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	provider, err := NewDataZoneProvider("ww-corp-a")
	if err != nil {
		t.Fatal(err)
	}
	provider.WithClock(func() time.Time { return now })
	for index, kind := range []string{"text", "image", "voice", "video", "file"} {
		_, err := provider.Append(DataZoneContent{
			Sequence: int64(index + 1), MessageID: "dz-msg-" + kind, Type: kind,
			Sender: "zhangsan", Receivers: []string{"lisi"}, Body: []byte("fixture-private-" + kind),
			FileName: kind + ".bin", MIMEType: "application/octet-stream",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	page, err := provider.Fetch("ww-corp-a", 0, 10)
	if err != nil || len(page) != 5 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	encoded, _ := json.Marshal(page)
	if bytes.Contains(encoded, []byte("fixture-private")) {
		t.Fatalf("metadata leaked content: %s", encoded)
	}
	for _, item := range page {
		secret, err := provider.DecryptSecretKey(item)
		if err != nil {
			t.Fatal(err)
		}
		content, err := provider.Render("ww-corp-a", item.MessageID, secret)
		if err != nil || !strings.HasPrefix(string(content.Body), "fixture-private-") {
			t.Fatalf("render=%+v err=%v", content, err)
		}
	}
}

func TestDataZoneProviderRejectsWrongScopeTamperExpiryAndRevocation(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	provider, err := NewDataZoneProvider("ww-corp-a")
	if err != nil {
		t.Fatal(err)
	}
	provider.WithClock(func() time.Time { return now })
	item, err := provider.Append(DataZoneContent{Sequence: 1, MessageID: "dz-secret", Type: "text", Sender: "a", Receivers: []string{"b"}, Body: []byte("never-log-this")})
	if err != nil {
		t.Fatal(err)
	}
	secret, err := provider.DecryptSecretKey(item)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		corp   string
		msgID  string
		secret []byte
		code   string
	}{
		{"ww-corp-b", item.MessageID, secret, "DATA_ZONE_SCOPE_MISMATCH"},
		{"ww-corp-a", "missing", secret, "DATA_ZONE_CONTENT_UNAVAILABLE"},
		{"ww-corp-a", item.MessageID, append([]byte(nil), secret[:len(secret)-1]...), "DATA_ZONE_SECRET_INVALID"},
	}
	for _, tc := range cases {
		_, gotErr := provider.Render(tc.corp, tc.msgID, tc.secret)
		if ErrorCode(gotErr) != tc.code || strings.Contains(gotErr.Error(), "never-log-this") {
			t.Fatalf("render error=%v want code=%s", gotErr, tc.code)
		}
	}
	tampered := item
	tampered.EncryptedSecretKey = corruptBase64(tampered.EncryptedSecretKey)
	if _, err := provider.DecryptSecretKey(tampered); ErrorCode(err) != "DATA_ZONE_SECRET_INVALID" {
		t.Fatalf("tampered secret error=%v", err)
	}
	now = now.Add(6 * time.Minute)
	if _, err := provider.Render("ww-corp-a", item.MessageID, secret); ErrorCode(err) != "DATA_ZONE_CONTENT_EXPIRED" {
		t.Fatalf("expired render error=%v", err)
	}
	provider.Revoke()
	if _, err := provider.Fetch("ww-corp-a", 0, 10); ErrorCode(err) != "DATA_ZONE_AUTHORIZATION_REVOKED" {
		t.Fatalf("revoked fetch error=%v", err)
	}
}

func TestDataZoneProviderFetchesSparseSequencesInOrder(t *testing.T) {
	provider, err := NewDataZoneProvider("ww-corp-a")
	if err != nil {
		t.Fatal(err)
	}
	for _, sequence := range []int64{7, 2, 11} {
		_, err := provider.Append(DataZoneContent{Sequence: sequence, MessageID: "dz-sparse-" + string(rune('a'+sequence)), Type: "text", Body: []byte("fixture")})
		if err != nil {
			t.Fatal(err)
		}
	}
	page, err := provider.Fetch("ww-corp-a", 2, 10)
	if err != nil || len(page) != 2 || page[0].Sequence != 7 || page[1].Sequence != 11 {
		t.Fatalf("sparse page=%+v err=%v", page, err)
	}
}

func TestDataZoneProviderStateRestoresExistingEncryptedLocator(t *testing.T) {
	provider, err := NewDataZoneProvider("ww-corp-state")
	if err != nil {
		t.Fatal(err)
	}
	provider.WithContentTTL(24 * time.Hour)
	item, err := provider.Append(DataZoneContent{Sequence: 9, MessageID: "dz-persisted", Type: "file", Body: []byte("persisted-private-body"), FileName: "fixture.txt", MIMEType: "text/plain"})
	if err != nil {
		t.Fatal(err)
	}
	state, err := provider.ExportState()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := NewDataZoneProviderFromState(state)
	if err != nil {
		t.Fatal(err)
	}
	page, err := restored.Fetch("ww-corp-state", 0, 10)
	if err != nil || len(page) != 1 || page[0].EncryptedSecretKey != item.EncryptedSecretKey {
		t.Fatalf("restored page=%+v err=%v", page, err)
	}
	secret, err := restored.DecryptSecretKey(page[0])
	if err != nil {
		t.Fatal(err)
	}
	content, err := restored.Render("ww-corp-state", item.MessageID, secret)
	if err != nil || string(content.Body) != "persisted-private-body" {
		t.Fatalf("restored content=%+v err=%v", content, err)
	}
}
