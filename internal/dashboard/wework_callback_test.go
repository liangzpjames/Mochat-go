package dashboard

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"
)

const testWeWorkAESKey = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

func TestWeWorkCallbackVerifyURLReturnsEchoString(t *testing.T) {
	store := &fakeWeWorkCallbackStore{
		byID: map[int]WeWorkCallbackCorp{
			7: {ID: 7, WxCorpID: "wx-test", Token: "callback-token", EncodingAESKey: testWeWorkAESKey},
		},
	}
	handler := NewWeWorkCallbackHandler(store, nil)
	encrypted := encryptWeWorkCallbackTestMessage(t, testWeWorkAESKey, []byte("verified"), "wx-test")
	signature := weWorkCallbackTestSignature("callback-token", "1710000000", "nonce", encrypted)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/corp/weWorkCallback?cid=7&timestamp=1710000000&nonce=nonce&echostr="+encrypted+"&msg_signature="+signature, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "verified" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestWeWorkCallbackPostDecryptsAndQueuesEvent(t *testing.T) {
	store := &fakeWeWorkCallbackStore{
		byWXID: map[string]WeWorkCallbackCorp{
			"wx-test": {ID: 7, WxCorpID: "wx-test", Token: "callback-token", EncodingAESKey: testWeWorkAESKey},
		},
	}
	queue := &fakeWeWorkCallbackQueue{}
	handler := NewWeWorkCallbackHandler(store, queue)
	handler.now = func() time.Time { return time.Date(2026, 7, 4, 12, 0, 0, 0, time.Local) }
	message := []byte(`<xml><ToUserName><![CDATA[wx-test]]></ToUserName><MsgType><![CDATA[event]]></MsgType><Event><![CDATA[change_contact]]></Event><ChangeType><![CDATA[create_user]]></ChangeType><UserID><![CDATA[zhangsan]]></UserID></xml>`)
	encrypted := encryptWeWorkCallbackTestMessage(t, testWeWorkAESKey, message, "wx-test")
	signature := weWorkCallbackTestSignature("callback-token", "1710000000", "nonce", encrypted)
	body := `<xml><ToUserName><![CDATA[wx-test]]></ToUserName><Encrypt><![CDATA[` + encrypted + `]]></Encrypt></xml>`

	req := httptest.NewRequest(http.MethodPost, "/weWork/callback?timestamp=1710000000&nonce=nonce&msg_signature="+signature, strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "success" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if len(queue.events) != 1 {
		t.Fatalf("queued events = %d", len(queue.events))
	}
	event := queue.events[0]
	if event.CorpID != 7 || event.WxCorpID != "wx-test" || event.EventPath != "event.change_contact.create_user" {
		t.Fatalf("event = %+v", event)
	}
	if event.Message["UserID"] != "zhangsan" || event.ReceivedAt != "2026-07-04 12:00:00" {
		t.Fatalf("event message = %+v", event)
	}
}

func TestWeWorkCallbackRejectsInvalidSignature(t *testing.T) {
	store := &fakeWeWorkCallbackStore{
		byWXID: map[string]WeWorkCallbackCorp{
			"wx-test": {ID: 7, WxCorpID: "wx-test", Token: "callback-token", EncodingAESKey: testWeWorkAESKey},
		},
	}
	queue := &fakeWeWorkCallbackQueue{}
	handler := NewWeWorkCallbackHandler(store, queue)
	encrypted := encryptWeWorkCallbackTestMessage(t, testWeWorkAESKey, []byte(`<xml><MsgType><![CDATA[event]]></MsgType></xml>`), "wx-test")
	body := `<xml><ToUserName><![CDATA[wx-test]]></ToUserName><Encrypt><![CDATA[` + encrypted + `]]></Encrypt></xml>`

	req := httptest.NewRequest(http.MethodPost, "/weWork/callback?timestamp=1710000000&nonce=nonce&msg_signature=bad", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(queue.events) != 0 {
		t.Fatalf("queued events = %d", len(queue.events))
	}
}

type fakeWeWorkCallbackStore struct {
	byID   map[int]WeWorkCallbackCorp
	byWXID map[string]WeWorkCallbackCorp
}

func (s *fakeWeWorkCallbackStore) WeWorkCallbackCorpByID(_ context.Context, corpID int) (WeWorkCallbackCorp, bool, error) {
	corp, ok := s.byID[corpID]
	return corp, ok, nil
}

func (s *fakeWeWorkCallbackStore) WeWorkCallbackCorpByWXID(_ context.Context, wxCorpID string) (WeWorkCallbackCorp, bool, error) {
	corp, ok := s.byWXID[wxCorpID]
	return corp, ok, nil
}

type fakeWeWorkCallbackQueue struct {
	events []WeWorkCallbackEvent
}

func (q *fakeWeWorkCallbackQueue) EnqueueWeWorkCallback(_ context.Context, event WeWorkCallbackEvent) error {
	q.events = append(q.events, event)
	return nil
}

func encryptWeWorkCallbackTestMessage(t *testing.T, encodingAESKey string, message []byte, receiveID string) string {
	t.Helper()
	key, err := decodeWeWorkAESKey(encodingAESKey)
	if err != nil {
		t.Fatal(err)
	}
	var plain bytes.Buffer
	plain.Write([]byte("abcdefghijklmnop"))
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(message)))
	plain.Write(size[:])
	plain.Write(message)
	plain.WriteString(receiveID)
	padded := pkcs7PadForTest(plain.Bytes(), aes.BlockSize)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(out, padded)
	return base64.StdEncoding.EncodeToString(out)
}

func pkcs7PadForTest(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	if padding == 0 {
		padding = blockSize
	}
	return append(append([]byte{}, data...), bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func weWorkCallbackTestSignature(token string, timestamp string, nonce string, encrypted string) string {
	items := []string{token, timestamp, nonce, encrypted}
	sort.Strings(items)
	sum := sha1.Sum([]byte(strings.Join(items, "")))
	return hex.EncodeToString(sum[:])
}
