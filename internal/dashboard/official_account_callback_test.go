package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestOfficialAccountAuthEventStoresComponentVerifyTicket(t *testing.T) {
	store := &fakeOfficialAccountAuthEventStore{}
	handler := NewOfficialAccountCallbackHandler(store, nil, "component-app", "component-secret", "component-token", "")

	body := `<xml>
<AppId><![CDATA[component-app]]></AppId>
<CreateTime>1783180000</CreateTime>
<InfoType><![CDATA[component_verify_ticket]]></InfoType>
<ComponentVerifyTicket><![CDATA[ticket-from-wechat]]></ComponentVerifyTicket>
</xml>`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/officialAccount/authEventCallback", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.AuthEventCallback(rec, req)

	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "success" {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if store.componentAppID != "component-app" || store.componentVerifyTicket != "ticket-from-wechat" || store.ticketCreateTime != 1783180000 {
		t.Fatalf("stored ticket = %#v", store)
	}
}

func TestOfficialAccountAuthEventStoresEncryptedComponentVerifyTicket(t *testing.T) {
	store := &fakeOfficialAccountAuthEventStore{}
	handler := NewOfficialAccountCallbackHandler(store, nil, "component-app", "component-secret", "component-token", testWeWorkAESKey)

	plain := `<xml>
<AppId><![CDATA[component-app]]></AppId>
<CreateTime>1783180001</CreateTime>
<InfoType><![CDATA[component_verify_ticket]]></InfoType>
<ComponentVerifyTicket><![CDATA[ticket-from-encrypted-wechat]]></ComponentVerifyTicket>
</xml>`
	encrypted := encryptWeWorkCallbackTestMessage(t, testWeWorkAESKey, []byte(plain), "component-app")
	signature := weWorkCallbackTestSignature("component-token", "1783180001", "nonce", encrypted)
	body := `<xml><AppId><![CDATA[component-app]]></AppId><Encrypt><![CDATA[` + encrypted + `]]></Encrypt></xml>`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/officialAccount/authEventCallback?encrypt_type=aes&timestamp=1783180001&nonce=nonce&msg_signature="+signature, strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.AuthEventCallback(rec, req)

	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "success" {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if store.componentAppID != "component-app" || store.componentVerifyTicket != "ticket-from-encrypted-wechat" || store.ticketCreateTime != 1783180001 {
		t.Fatalf("stored ticket = %#v", store)
	}
}

func TestOfficialAccountEncryptedEchoRequiresValidSignature(t *testing.T) {
	handler := NewOfficialAccountCallbackHandler(nil, nil, "component-app", "component-secret", "component-token", testWeWorkAESKey)

	encrypted := encryptWeWorkCallbackTestMessage(t, testWeWorkAESKey, []byte("echo-ok"), "component-app")
	signature := weWorkCallbackTestSignature("component-token", "1783180004", "nonce", encrypted)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/officialAccount/authEventCallback?encrypt_type=aes&timestamp=1783180004&nonce=nonce&echostr="+url.QueryEscape(encrypted)+"&msg_signature="+signature, nil)
	rec := httptest.NewRecorder()
	handler.AuthEventCallback(rec, req)

	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "echo-ok" {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}

	missingReq := httptest.NewRequest(http.MethodGet, "/dashboard/officialAccount/authEventCallback?encrypt_type=aes&timestamp=1783180004&nonce=nonce&echostr="+url.QueryEscape(encrypted), nil)
	missingRec := httptest.NewRecorder()
	handler.AuthEventCallback(missingRec, missingReq)

	if missingRec.Code != http.StatusBadRequest || !strings.Contains(missingRec.Body.String(), "msg_signature required") {
		t.Fatalf("missing signature status=%d body=%q", missingRec.Code, missingRec.Body.String())
	}
}

func TestOfficialAccountEncryptedAuthEventRejectsMissingSignature(t *testing.T) {
	store := &fakeOfficialAccountAuthEventStore{}
	handler := NewOfficialAccountCallbackHandler(store, nil, "component-app", "component-secret", "component-token", testWeWorkAESKey)

	plain := `<xml>
<AppId><![CDATA[component-app]]></AppId>
<CreateTime>1783180002</CreateTime>
<InfoType><![CDATA[component_verify_ticket]]></InfoType>
<ComponentVerifyTicket><![CDATA[ticket-without-signature]]></ComponentVerifyTicket>
</xml>`
	encrypted := encryptWeWorkCallbackTestMessage(t, testWeWorkAESKey, []byte(plain), "component-app")
	body := `<xml><AppId><![CDATA[component-app]]></AppId><Encrypt><![CDATA[` + encrypted + `]]></Encrypt></xml>`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/officialAccount/authEventCallback?encrypt_type=aes&timestamp=1783180002&nonce=nonce", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.AuthEventCallback(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "msg_signature required") {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if store.componentVerifyTicket != "" {
		t.Fatalf("unexpected stored ticket = %#v", store)
	}
}

func TestOfficialAccountEncryptedMessageRejectsInvalidSignature(t *testing.T) {
	client := &fakeOfficialAccountMessageClient{}
	handler := NewOfficialAccountCallbackHandler(nil, client, "component-app", "component-secret", "component-token", testWeWorkAESKey)

	plain := `<xml>
<ToUserName><![CDATA[gh_3c884a361561]]></ToUserName>
<FromUserName><![CDATA[from-user]]></FromUserName>
<CreateTime>1783180003</CreateTime>
<MsgType><![CDATA[text]]></MsgType>
<Content><![CDATA[QUERY_AUTH_CODE: query-code]]></Content>
</xml>`
	encrypted := encryptWeWorkCallbackTestMessage(t, testWeWorkAESKey, []byte(plain), "component-app")
	body := `<xml><ToUserName><![CDATA[component-app]]></ToUserName><Encrypt><![CDATA[` + encrypted + `]]></Encrypt></xml>`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/component-app/officialAccount/messageEventCallback?encrypt_type=aes&timestamp=1783180003&nonce=nonce&msg_signature=bad", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.MessageEventCallback(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid msg_signature") {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if client.authCode != "" || client.toUser != "" || client.content != "" {
		t.Fatalf("unexpected message client call = %#v", client)
	}
}

func TestOfficialAccountAuthEventStoresAuthorizedEvent(t *testing.T) {
	store := &fakeOfficialAccountAuthEventStore{}
	handler := NewOfficialAccountCallbackHandler(store, nil, "component-app", "component-secret", "component-token", "aes-key")

	body := `<xml>
<AppId><![CDATA[component-app]]></AppId>
<CreateTime>1783180100</CreateTime>
<InfoType><![CDATA[authorized]]></InfoType>
<AuthorizerAppid><![CDATA[authorizer-app]]></AuthorizerAppid>
<AuthorizationCode><![CDATA[auth-code]]></AuthorizationCode>
<PreAuthCode><![CDATA[pre-auth-code]]></PreAuthCode>
</xml>`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/officialAccount/authEventCallback", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.AuthEventCallback(rec, req)

	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "success" {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if store.authorization.AuthorizerAppID != "authorizer-app" || store.authorization.AuthorizationCode != "auth-code" {
		t.Fatalf("authorization = %#v", store.authorization)
	}
	if store.authorization.AuthorizedStatus != 1 || store.authorization.ComponentSecret != "component-secret" || store.authorization.ComponentToken != "component-token" {
		t.Fatalf("authorization metadata = %#v", store.authorization)
	}
}

func TestOfficialAccountMessageEventHandlesComponentCase(t *testing.T) {
	client := &fakeOfficialAccountMessageClient{}
	handler := NewOfficialAccountCallbackHandler(nil, client, "component-app", "component-secret", "component-token", "")

	body := `<xml>
<ToUserName><![CDATA[gh_3c884a361561]]></ToUserName>
<FromUserName><![CDATA[from-user]]></FromUserName>
<CreateTime>1783180200</CreateTime>
<MsgType><![CDATA[text]]></MsgType>
<Content><![CDATA[QUERY_AUTH_CODE: query-code]]></Content>
</xml>`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/component-app/officialAccount/messageEventCallback", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.MessageEventCallback(rec, req)

	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "" {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if client.authCode != "query-code" || client.toUser != "from-user" || client.content != "query-code_from_api" {
		t.Fatalf("message client = %#v", client)
	}
}

type fakeOfficialAccountAuthEventStore struct {
	authorization         OfficialAccountAuthorization
	componentAppID        string
	componentVerifyTicket string
	ticketCreateTime      int64
}

func (s *fakeOfficialAccountAuthEventStore) UpsertOfficialAccountAuthEvent(_ context.Context, values OfficialAccountAuthorization) (int, error) {
	s.authorization = values
	return 11, nil
}

func (s *fakeOfficialAccountAuthEventStore) UpsertWeChatComponentVerifyTicket(_ context.Context, componentAppID string, componentVerifyTicket string, createTime int64) error {
	s.componentAppID = componentAppID
	s.componentVerifyTicket = componentVerifyTicket
	s.ticketCreateTime = createTime
	return nil
}

type fakeOfficialAccountMessageClient struct {
	authCode string
	toUser   string
	content  string
}

func (c *fakeOfficialAccountMessageClient) SendCustomerTextFromAuthCode(_ context.Context, authCode string, toUser string, content string) error {
	c.authCode = authCode
	c.toUser = toUser
	c.content = content
	return nil
}
