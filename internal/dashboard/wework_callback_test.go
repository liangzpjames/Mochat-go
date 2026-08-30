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
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
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
			"wx-test": {TenantID: 3, ID: 7, WxCorpID: "wx-test", Token: "callback-token", EncodingAESKey: testWeWorkAESKey},
		},
	}
	wakeup := &fakeWeWorkCallbackWakeup{}
	handler := NewWeWorkCallbackHandler(store, wakeup)
	handler.now = func() time.Time { return time.Date(2026, 7, 4, 12, 0, 0, 0, time.Local) }
	message := []byte(`<xml><ToUserName><![CDATA[wx-test]]></ToUserName><MsgType><![CDATA[event]]></MsgType><Event><![CDATA[change_contact]]></Event><ChangeType><![CDATA[create_user]]></ChangeType><UserID><![CDATA[zhangsan]]></UserID></xml>`)
	encrypted := encryptWeWorkCallbackTestMessage(t, testWeWorkAESKey, message, "wx-test")
	signature := weWorkCallbackTestSignature("callback-token", "1710000000", "nonce", encrypted)
	body := `<xml><ToUserName><![CDATA[wx-test]]></ToUserName><Encrypt><![CDATA[` + encrypted + `]]></Encrypt></xml>`

	timestamp := strconv.FormatInt(handler.now().Unix(), 10)
	signature = weWorkCallbackTestSignature("callback-token", timestamp, "nonce", encrypted)
	req := httptest.NewRequest(http.MethodPost, "/weWork/callback?timestamp="+timestamp+"&nonce=nonce&msg_signature="+signature, strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "success" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if len(store.accepted) != 1 {
		t.Fatalf("accepted events = %d", len(store.accepted))
	}
	event := store.accepted[0].event
	if event.TenantID != 3 || event.CorpID != 7 || event.WxCorpID != "wx-test" || event.EventPath != "event.change_contact.create_user" {
		t.Fatalf("event = %+v", event)
	}
	if event.Message["UserID"] != "zhangsan" || event.ReceivedAt != "2026-07-04 12:00:00" {
		t.Fatalf("event message = %+v", event)
	}
	if event.RawXML != "" || store.accepted[0].eventKey == "" || store.accepted[0].fingerprint == "" {
		t.Fatalf("durable receipt leaks raw payload or lacks identity: %+v", store.accepted[0])
	}
	if wakeup.calls != 1 {
		t.Fatalf("wakeup calls = %d", wakeup.calls)
	}
}

func TestWeWorkCallbackAcknowledgesDurableAcceptanceWhenWakeupFails(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	store := callbackStoreFixture()
	wakeup := &fakeWeWorkCallbackWakeup{err: errors.New("redis unavailable"), waitForContext: true}
	handler := NewWeWorkCallbackHandler(store, wakeup)
	handler.now = func() time.Time { return now }

	startedAt := time.Now()
	rec := postWeWorkCallbackTestEvent(t, handler, now, callbackTestMessage("zhangsan"))
	if rec.Code != http.StatusOK || rec.Body.String() != "success" || len(store.accepted) != 1 || wakeup.calls != 1 {
		t.Fatalf("status=%d body=%q accepted=%d wakeups=%d", rec.Code, rec.Body.String(), len(store.accepted), wakeup.calls)
	}
	if elapsed := time.Since(startedAt); elapsed > 250*time.Millisecond {
		t.Fatalf("optional Redis wakeup delayed durable ACK for %s", elapsed)
	}
}

func TestWeWorkCallbackReturnsServiceUnavailableWhenLookupOrAcceptanceFails(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name      string
		lookupErr error
		acceptErr error
	}{
		{name: "corp lookup", lookupErr: errors.New("database unavailable")},
		{name: "durable acceptance", acceptErr: errors.New("database unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := callbackStoreFixture()
			store.lookupErr = tc.lookupErr
			store.acceptErr = tc.acceptErr
			handler := NewWeWorkCallbackHandler(store, &fakeWeWorkCallbackWakeup{})
			handler.now = func() time.Time { return now }

			rec := postWeWorkCallbackTestEvent(t, handler, now, callbackTestMessage("zhangsan"))
			if rec.Code != http.StatusServiceUnavailable || rec.Body.String() == "success" {
				t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestWeWorkCallbackDuplicateSucceedsButPayloadConflictDoesNot(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name       string
		replayed   bool
		acceptErr  error
		wantStatus int
	}{
		{name: "duplicate", replayed: true, wantStatus: http.StatusOK},
		{name: "same key different payload", acceptErr: ErrWeWorkCallbackConflict, wantStatus: http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := callbackStoreFixture()
			store.replayed = tc.replayed
			store.acceptErr = tc.acceptErr
			handler := NewWeWorkCallbackHandler(store, &fakeWeWorkCallbackWakeup{})
			handler.now = func() time.Time { return now }

			rec := postWeWorkCallbackTestEvent(t, handler, now, callbackTestMessage("zhangsan"))
			if rec.Code != tc.wantStatus {
				t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestWeWorkCallbackRejectsStaleTimestampBeforeDurableAcceptance(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	store := callbackStoreFixture()
	handler := NewWeWorkCallbackHandler(store, &fakeWeWorkCallbackWakeup{})
	handler.now = func() time.Time { return now }

	rec := postWeWorkCallbackTestEvent(t, handler, now.Add(-11*time.Minute), callbackTestMessage("zhangsan"))
	if rec.Code != http.StatusBadRequest || len(store.accepted) != 0 {
		t.Fatalf("status=%d accepted=%d body=%q", rec.Code, len(store.accepted), rec.Body.String())
	}
}

func TestWeWorkCallbackProviderEventIDIsStableAndFingerprintDetectsPayloadConflict(t *testing.T) {
	base := WeWorkCallbackEvent{TenantID: 3, CorpID: 7, WxCorpID: "wx-test", EventPath: "event.change_contact.create_user", Message: map[string]string{
		"ToUserName": "wx-test", "CreateTime": "1783159200", "MsgId": "provider-event-1", "UserID": "zhangsan", "Name": "张三",
	}}
	changed := base
	changed.Message = map[string]string{"ToUserName": "wx-test", "CreateTime": "1783159200", "MsgId": "provider-event-1", "UserID": "zhangsan", "Name": "李四"}

	if WeWorkCallbackEventKey(base) == "" || WeWorkCallbackEventKey(base) != WeWorkCallbackEventKey(changed) {
		t.Fatalf("business event key was not stable")
	}
	if WeWorkCallbackPayloadFingerprint(base) == WeWorkCallbackPayloadFingerprint(changed) {
		t.Fatalf("payload fingerprint did not detect conflict")
	}
}

func TestWeWorkCallbackWithoutProviderEventIDUsesNormalizedPayloadToAvoidSameSecondCollision(t *testing.T) {
	base := WeWorkCallbackEvent{TenantID: 3, CorpID: 7, WxCorpID: "wx-test", EventPath: "event.change_contact.create_user", Message: map[string]string{
		"ToUserName": "wx-test", "CreateTime": "1783159200", "UserID": "zhangsan", "Name": "张三",
	}}
	changed := base
	changed.Message = map[string]string{"Name": "李四", "UserID": "zhangsan", "CreateTime": "1783159200", "ToUserName": "wx-test"}
	retry := base
	retry.Message = map[string]string{"UserID": " zhangsan ", "ToUserName": "wx-test", "Name": " 张三 ", "CreateTime": "1783159200"}

	if WeWorkCallbackEventKey(base) == WeWorkCallbackEventKey(changed) {
		t.Fatal("distinct same-second callbacks without provider ids shared one event key")
	}
	if WeWorkCallbackEventKey(base) != WeWorkCallbackEventKey(retry) {
		t.Fatal("exact retry did not retain the same normalized event key")
	}
}

func TestWeWorkCallbackAcceptanceDeadlineReturnsServiceUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name       string
		blockStore func(*fakeWeWorkCallbackStore)
	}{
		{name: "corp lookup", blockStore: func(store *fakeWeWorkCallbackStore) { store.lookupWaitForContext = true }},
		{name: "durable acceptance", blockStore: func(store *fakeWeWorkCallbackStore) { store.acceptWaitForContext = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
			store := callbackStoreFixture()
			tc.blockStore(store)
			handler := NewWeWorkCallbackHandler(store, nil).WithAcceptanceTimeout(20 * time.Millisecond)
			handler.now = func() time.Time { return now }

			started := time.Now()
			rec := postWeWorkCallbackTestEvent(t, handler, now, callbackTestMessage("zhangsan"))
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
			}
			if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
				t.Fatalf("callback ACK deadline took %s", elapsed)
			}
		})
	}
}

func TestWeWorkCallbackRejectsInvalidSignature(t *testing.T) {
	store := &fakeWeWorkCallbackStore{
		byWXID: map[string]WeWorkCallbackCorp{
			"wx-test": {ID: 7, WxCorpID: "wx-test", Token: "callback-token", EncodingAESKey: testWeWorkAESKey},
		},
	}
	wakeup := &fakeWeWorkCallbackWakeup{}
	handler := NewWeWorkCallbackHandler(store, wakeup)
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	handler.now = func() time.Time { return now }
	encrypted := encryptWeWorkCallbackTestMessage(t, testWeWorkAESKey, []byte(`<xml><MsgType><![CDATA[event]]></MsgType></xml>`), "wx-test")
	body := `<xml><ToUserName><![CDATA[wx-test]]></ToUserName><Encrypt><![CDATA[` + encrypted + `]]></Encrypt></xml>`

	req := httptest.NewRequest(http.MethodPost, "/weWork/callback?timestamp="+strconv.FormatInt(now.Unix(), 10)+"&nonce=nonce&msg_signature=bad", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(store.accepted) != 0 {
		t.Fatalf("accepted events = %d", len(store.accepted))
	}
}

type fakeWeWorkCallbackStore struct {
	byID                 map[int]WeWorkCallbackCorp
	byWXID               map[string]WeWorkCallbackCorp
	lookupErr            error
	acceptErr            error
	replayed             bool
	accepted             []fakeWeWorkCallbackAcceptance
	lookupWaitForContext bool
	acceptWaitForContext bool
}

func (s *fakeWeWorkCallbackStore) WeWorkCallbackCorpByID(ctx context.Context, corpID int) (WeWorkCallbackCorp, bool, error) {
	if s.lookupWaitForContext {
		<-ctx.Done()
		return WeWorkCallbackCorp{}, false, ctx.Err()
	}
	if s.lookupErr != nil {
		return WeWorkCallbackCorp{}, false, s.lookupErr
	}
	corp, ok := s.byID[corpID]
	return corp, ok, nil
}

func (s *fakeWeWorkCallbackStore) WeWorkCallbackCorpByWXID(ctx context.Context, wxCorpID string) (WeWorkCallbackCorp, bool, error) {
	if s.lookupWaitForContext {
		<-ctx.Done()
		return WeWorkCallbackCorp{}, false, ctx.Err()
	}
	if s.lookupErr != nil {
		return WeWorkCallbackCorp{}, false, s.lookupErr
	}
	corp, ok := s.byWXID[wxCorpID]
	return corp, ok, nil
}

type fakeWeWorkCallbackAcceptance struct {
	event       WeWorkCallbackEvent
	eventKey    string
	fingerprint string
}

func (s *fakeWeWorkCallbackStore) AcceptWeWorkCallback(ctx context.Context, event WeWorkCallbackEvent, eventKey string, fingerprint string) (bool, error) {
	s.accepted = append(s.accepted, fakeWeWorkCallbackAcceptance{event: event, eventKey: eventKey, fingerprint: fingerprint})
	if s.acceptWaitForContext {
		<-ctx.Done()
		return false, ctx.Err()
	}
	return s.replayed, s.acceptErr
}

type fakeWeWorkCallbackWakeup struct {
	calls          int
	err            error
	waitForContext bool
}

func (w *fakeWeWorkCallbackWakeup) WakeWeWorkCallback(ctx context.Context) error {
	w.calls++
	if w.waitForContext {
		<-ctx.Done()
	}
	return w.err
}

func callbackStoreFixture() *fakeWeWorkCallbackStore {
	corp := WeWorkCallbackCorp{TenantID: 3, ID: 7, WxCorpID: "wx-test", Token: "callback-token", EncodingAESKey: testWeWorkAESKey}
	return &fakeWeWorkCallbackStore{byID: map[int]WeWorkCallbackCorp{7: corp}, byWXID: map[string]WeWorkCallbackCorp{"wx-test": corp}}
}

func callbackTestMessage(userID string) []byte {
	return []byte(`<xml><ToUserName><![CDATA[wx-test]]></ToUserName><CreateTime>1783159200</CreateTime><MsgType><![CDATA[event]]></MsgType><Event><![CDATA[change_contact]]></Event><ChangeType><![CDATA[create_user]]></ChangeType><UserID><![CDATA[` + userID + `]]></UserID></xml>`)
}

func postWeWorkCallbackTestEvent(t *testing.T, handler *WeWorkCallbackHandler, signedAt time.Time, message []byte) *httptest.ResponseRecorder {
	t.Helper()
	encrypted := encryptWeWorkCallbackTestMessage(t, testWeWorkAESKey, message, "wx-test")
	timestamp := strconv.FormatInt(signedAt.Unix(), 10)
	signature := weWorkCallbackTestSignature("callback-token", timestamp, "nonce", encrypted)
	body := `<xml><ToUserName><![CDATA[wx-test]]></ToUserName><Encrypt><![CDATA[` + encrypted + `]]></Encrypt></xml>`
	req := httptest.NewRequest(http.MethodPost, "/weWork/callback?timestamp="+timestamp+"&nonce=nonce&msg_signature="+signature, strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
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
