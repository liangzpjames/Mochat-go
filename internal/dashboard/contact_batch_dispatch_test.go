package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

func TestContactBatchDurableReminderUnknownAttemptDoesNotResend(t *testing.T) {
	store := &fakeContactMessageBatchSendStore{users: map[int]User{1: {ID: 1}}}
	reminderStore := &fakeContactBatchDurableReminderStore{
		prepareErr: ErrContactBatchReminderReconcileRequired,
	}
	client := &fakeContactBatchDispatchReminderClient{}
	handler := NewContactMessageBatchSendHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", t.TempDir(), client)
	handler.durableStore = reminderStore

	request := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/contactMessageBatchSend/remind", strings.NewReader(`{"batchId":701,"batchEmployId":21}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.Remind(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "CONTACT_BATCH_REMINDER_RECONCILE_REQUIRED") {
		t.Fatalf("body=%s", recorder.Body.String())
	}
	if client.sendCalls != 0 {
		t.Fatalf("unknown attempt was resent: calls=%d", client.sendCalls)
	}
}

func TestContactBatchDurableReminderCompletedAttemptDoesNotResend(t *testing.T) {
	store := &fakeContactMessageBatchSendStore{users: map[int]User{1: {ID: 1}}}
	reminderStore := &fakeContactBatchDurableReminderStore{
		prepareReminder: ContactBatchDurableReminder{AlreadyCompleted: true},
	}
	client := &fakeContactBatchDispatchReminderClient{}
	handler := NewContactMessageBatchSendHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", t.TempDir(), client)
	handler.durableStore = reminderStore

	request := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/contactMessageBatchSend/remind", strings.NewReader(`{"batchId":701,"batchEmployId":21}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.Remind(recorder, request)

	if recorder.Code != http.StatusOK || client.sendCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, client.sendCalls, recorder.Body.String())
	}
}

type fakeContactBatchDurableReminderStore struct {
	prepareReminder ContactBatchDurableReminder
	prepareErr      error
}

func (s *fakeContactBatchDurableReminderStore) CreateContactBatchDispatch(context.Context, dashboardprincipal.DashboardPrincipal, DashboardAccessContext, ContactBatchDispatchInput) (ContactBatchDispatchResult, error) {
	return ContactBatchDispatchResult{}, nil
}

func (s *fakeContactBatchDurableReminderStore) CancelContactBatchDurable(context.Context, dashboardprincipal.DashboardPrincipal, int) error {
	return nil
}

func (s *fakeContactBatchDurableReminderStore) PrepareContactBatchDurableReminder(context.Context, dashboardprincipal.DashboardPrincipal, int, int, string) (ContactBatchDurableReminder, error) {
	return s.prepareReminder, s.prepareErr
}

func (s *fakeContactBatchDurableReminderStore) RecordContactBatchDurableReminder(context.Context, dashboardprincipal.DashboardPrincipal, ContactBatchDurableReminder, string, bool, int, int) error {
	return nil
}

type fakeContactBatchDispatchReminderClient struct {
	sendCalls int
}

func (c *fakeContactBatchDispatchReminderClient) UploadTemporaryImage(context.Context, RoomWelcomeCorpCredential, string) (string, error) {
	return "", nil
}

func (c *fakeContactBatchDispatchReminderClient) SubmitContactMessageBatchSend(context.Context, RoomWelcomeCorpCredential, ContactMessageBatchSendMessagePayload) (ContactMessageBatchSendMessageResult, error) {
	return ContactMessageBatchSendMessageResult{}, nil
}

func (c *fakeContactBatchDispatchReminderClient) SendAgentTextMessage(context.Context, RoomTagPullAgentCredential, string, string) error {
	c.sendCalls++
	return nil
}

func (c *fakeContactBatchDispatchReminderClient) SendAgentTextMessageWithDuplicateCheck(context.Context, RoomTagPullAgentCredential, string, string) error {
	c.sendCalls++
	return nil
}

func TestContactBatchDispatchStoreRejectsBodyTenantAndActorAndUsesDurableOperation(t *testing.T) {
	store := &fakeContactBatchDispatchStore{
		fakeContactMessageBatchSendStore: &fakeContactMessageBatchSendStore{users: map[int]User{1: {ID: 1, TenantID: 1, Name: "admin"}}},
		result:                           ContactBatchDispatchResult{OperationID: 91, BatchID: 701, Status: "pending"},
	}
	handler := NewContactMessageBatchSendHandlerWithDispatch(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", t.TempDir(), nil)

	request := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/contactMessageBatchSend/store", strings.NewReader(`{"tenantId":99,"actorId":42,"idempotencyKey":"contact-701","employeeIds":[11],"contactTargets":[{"employeeId":11,"contactId":21}],"content":[{"msgType":"text","content":"hello"}]}`))
	request = request.WithContext(WithDashboardAccessContext(request.Context(), DashboardAccessContext{
		UserID: 1, TenantID: 1, CorpID: 7, PermissionCode: "dashboard.acquisition.precise_group_send",
		PermissionCodes: []string{"dashboard.acquisition.precise_group_send"}, Scope: DataScopeTenant,
		IsSuperAdmin: true,
	}))
	request.Header.Set("X-Mochat-Go-User-ID", "1")
	response := httptest.NewRecorder()
	handler.Store(response, request)
	if response.Code != http.StatusBadRequest || store.calls != 0 {
		t.Fatalf("body tenant/actor status=%d calls=%d body=%s", response.Code, store.calls, response.Body.String())
	}

	request = authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/contactMessageBatchSend/store", strings.NewReader(`{"idempotencyKey":"contact-701","employeeIds":[11],"contactTargets":[{"employeeId":11,"contactId":21}],"content":[{"msgType":"text","content":"hello"}],"sendWay":1}`))
	request = request.WithContext(WithDashboardAccessContext(request.Context(), DashboardAccessContext{
		UserID: 1, TenantID: 1, CorpID: 7, PermissionCode: "dashboard.acquisition.precise_group_send",
		PermissionCodes: []string{"dashboard.acquisition.precise_group_send"}, Scope: DataScopeTenant,
		IsSuperAdmin: true,
	}))
	request.Header.Set("X-Mochat-Go-User-ID", "1")
	response = httptest.NewRecorder()
	handler.Store(response, request)
	if response.Code != http.StatusOK || store.calls != 1 || store.lastPrincipal.TenantID != 1 || store.lastPrincipal.CorpID != 7 || store.lastPrincipal.UserID != 1 {
		t.Fatalf("durable create status=%d calls=%d principal=%#v body=%s", response.Code, store.calls, store.lastPrincipal, response.Body.String())
	}
}

func TestContactBatchDurableConstructorFailsClosedWhenStoreIsNotDurable(t *testing.T) {
	store := &fakeContactMessageBatchSendStore{users: map[int]User{1: {ID: 1, TenantID: 1, Name: "admin"}}}
	handler := NewContactMessageBatchSendHandlerWithDispatch(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", t.TempDir(), nil)
	request := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/contactMessageBatchSend/store", strings.NewReader(`{"idempotencyKey":"contact-701","employeeIds":[11],"contactTargets":[{"employeeId":11,"contactId":21}],"content":[{"msgType":"text","content":"hello"}]}`))
	request = request.WithContext(WithDashboardAccessContext(request.Context(), DashboardAccessContext{
		UserID: 1, TenantID: 1, CorpID: 7, PermissionCodes: []string{"dashboard.acquisition.precise_group_send"}, Scope: DataScopeTenant, IsSuperAdmin: true,
	}))
	request.Header.Set("X-Mochat-Go-User-ID", "1")
	response := httptest.NewRecorder()
	handler.Store(response, request)
	if response.Code != http.StatusServiceUnavailable || store.createCalls != 0 {
		t.Fatalf("missing durable dependency status=%d createCalls=%d body=%s", response.Code, store.createCalls, response.Body.String())
	}
}

func TestContactBatchDurableBodyRejectsUnknownField(t *testing.T) {
	store := &fakeContactBatchDispatchStore{
		fakeContactMessageBatchSendStore: &fakeContactMessageBatchSendStore{users: map[int]User{1: {ID: 1, TenantID: 1, Name: "admin"}}},
		result:                           ContactBatchDispatchResult{OperationID: 91, BatchID: 701, Status: "pending"},
	}
	handler := NewContactMessageBatchSendHandlerWithDispatch(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", t.TempDir(), nil)
	request := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/contactMessageBatchSend/store", strings.NewReader(`{"idempotencyKey":"contact-701","employeeIds":[11],"contactTargets":[{"employeeId":11,"contactId":21}],"content":[{"msgType":"text","content":"hello"}],"tenatId":1}`))
	request = request.WithContext(WithDashboardAccessContext(request.Context(), DashboardAccessContext{
		UserID: 1, TenantID: 1, CorpID: 7, PermissionCodes: []string{"dashboard.acquisition.precise_group_send"}, Scope: DataScopeTenant, IsSuperAdmin: true,
	}))
	request.Header.Set("X-Mochat-Go-User-ID", "1")
	response := httptest.NewRecorder()
	handler.Store(response, request)
	if response.Code != http.StatusBadRequest || store.calls != 0 {
		t.Fatalf("unknown field status=%d calls=%d body=%s", response.Code, store.calls, response.Body.String())
	}
}

func TestContactBatchDurableBodyRejectsUnsupportedFilterContentAndInvalidSchedule(t *testing.T) {
	h := &ContactMessageBatchSendHandler{}
	base := func() contactBatchDispatchBody {
		return contactBatchDispatchBody{
			BatchTitle: "contact batch", EmployeeIDs: []int{11},
			ContactTargets: []ContactBatchTarget{{EmployeeID: 11, ContactID: 21}},
			Content:        []ContactMessageBatchSendContent{{MsgType: "text", Content: "hello"}},
			IdempotencyKey: "contact-701", SendWay: 1,
		}
	}
	for name, mutate := range map[string]func(*contactBatchDispatchBody){
		"non-empty filter": func(body *contactBatchDispatchBody) { body.FilterParams = json.RawMessage(`{"gender":1}`) },
		"unknown content type": func(body *contactBatchDispatchBody) {
			body.Content = []ContactMessageBatchSendContent{{MsgType: "unsupported", Content: "hello"}}
		},
		"incompatible content fields": func(body *contactBatchDispatchBody) {
			body.Content = []ContactMessageBatchSendContent{{MsgType: "text", Content: "hello", URL: "https://example.test"}}
		},
		"immediate with schedule": func(body *contactBatchDispatchBody) { body.DefiniteTime = "2026-08-15 12:00:00" },
		"scheduled invalid time":  func(body *contactBatchDispatchBody) { body.SendWay = 2; body.DefiniteTime = "2026-99-99 12:00:00" },
	} {
		t.Run(name, func(t *testing.T) {
			_, err := h.contactBatchDispatchInputFromBody(func() contactBatchDispatchBody { body := base(); mutate(&body); return body }(), User{ID: 1, Name: "actor"}, 7, DashboardAccessContext{Scope: DataScopeTenant})
			if err == nil {
				t.Fatal("invalid durable body was accepted")
			}
		})
	}
}

func TestContactBatchErrorCodesKeepTenantSessionAndScopeSemanticsDistinct(t *testing.T) {
	cases := []struct {
		err    error
		code   string
		status int
	}{
		{ErrContactBatchTenantDenied, "TENANT_ACCESS_DENIED", http.StatusForbidden},
		{ErrContactBatchPermissionDenied, "DASHBOARD_PERMISSION_DENIED", http.StatusForbidden},
		{ErrContactBatchTargetNotOwned, "CONTACT_BATCH_SCOPE_DENIED", http.StatusForbidden},
		{ErrContactBatchQuotaExceeded, "CONTACT_BATCH_QUOTA_EXCEEDED", http.StatusConflict},
		{ErrContactBatchCapabilityLimited, "CONTACT_BATCH_CAPABILITY_LIMITED", http.StatusConflict},
		{ErrContactBatchNotFound, "CONTACT_BATCH_NOT_FOUND", http.StatusNotFound},
	}
	for _, item := range cases {
		code, status := contactBatchDispatchHTTPError(item.err)
		if code != item.code || status != item.status {
			t.Errorf("error=%v got code=%s status=%d, want code=%s status=%d", item.err, code, status, item.code, item.status)
		}
	}
}

func TestContactBatchDispatchFakeHTTPLocksContactPayloadAndProviderMessageID(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/gettoken" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"token-1","expires_in":7200}`))
			return
		}
		if r.URL.Path != "/cgi-bin/externalcontact/add_msg_template" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		got = map[string]any{}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","msgid":"contact-msg-1"}`))
	}))
	defer server.Close()
	client := NewRoomWelcomeWeComClient(server.URL)
	result, err := client.SubmitContactMessageBatchSend(context.Background(), RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "contact-secret"}, ContactMessageBatchSendMessagePayload{
		Sender: "employee-1", ExternalUserID: []string{"external-1"}, Content: []ContactMessageBatchSendContent{{MsgType: "text", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result.MsgID != "contact-msg-1" || got["chat_type"] != "single" || got["sender"] != "employee-1" {
		t.Fatalf("result=%#v payload=%#v", result, got)
	}
	if _, ok := got["chat_id_list"]; ok {
		t.Fatal("contact payload must not contain room chat_id_list")
	}
}

type fakeContactBatchDispatchStore struct {
	*fakeContactMessageBatchSendStore
	result        ContactBatchDispatchResult
	calls         int
	createCalls   int
	lastPrincipal dashboardprincipal.DashboardPrincipal
}

func (s *fakeContactBatchDispatchStore) CreateContactBatchDispatch(_ context.Context, principal dashboardprincipal.DashboardPrincipal, _ DashboardAccessContext, _ ContactBatchDispatchInput) (ContactBatchDispatchResult, error) {
	s.calls++
	s.createCalls++
	s.lastPrincipal = principal
	return s.result, nil
}

type fakeContactBatchPollRuntimeStore struct{}

func (fakeContactBatchPollRuntimeStore) ContactBatchDispatchPayload(context.Context, dashboardprincipal.DashboardPrincipal, wecomcapability.Dispatch) (ContactMessageBatchSendMessagePayload, error) {
	return ContactMessageBatchSendMessagePayload{}, nil
}

func (fakeContactBatchPollRuntimeStore) ContactBatchDispatchCredential(context.Context, dashboardprincipal.DashboardPrincipal) (RoomWelcomeCorpCredential, bool, error) {
	return RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "contact-secret"}, true, nil
}

type fakeContactBatchPollClient struct {
	taskPages   map[string]BatchSendGroupTaskPage
	resultPages map[string]BatchSendGroupResultPage
}

func (f *fakeContactBatchPollClient) GroupMessageTasks(_ context.Context, _ RoomWelcomeCorpCredential, _ string, _ int, cursor string) (BatchSendGroupTaskPage, error) {
	return f.taskPages[cursor], nil
}

func (f *fakeContactBatchPollClient) GroupMessageSendResults(_ context.Context, _ RoomWelcomeCorpCredential, _ string, userID string, _ int, cursor string) (BatchSendGroupResultPage, error) {
	return f.resultPages[userID+"|"+cursor], nil
}

func TestContactBatchPollUsesEmployeeCompositeTargetIdentity(t *testing.T) {
	poller := &ContactBatchDispatchPoller{
		store:  fakeContactBatchPollRuntimeStore{},
		client: &fakeContactBatchPollClient{taskPages: map[string]BatchSendGroupTaskPage{"": {TaskList: []BatchSendGroupTask{{UserID: "sender", Status: batchSendStatusSent}}}}, resultPages: map[string]BatchSendGroupResultPage{"sender|": {SendList: []BatchSendGroupResult{{UserID: "sender", ExternalUserID: "external-shared", Status: batchSendMessageDelivered}}}}},
	}
	base := wecomcapability.DispatchPollRequest{Principal: wecomcapability.DispatchPrincipal{TenantID: 1, CorpID: 7, UserID: 9}, Capability: wecomcapability.ContactBatchSend, Dispatch: wecomcapability.Dispatch{DispatchKind: string(wecomcapability.DispatchKindContactBatch), ProviderMessageID: "msg-1"}}
	first := base
	first.Dispatch.TargetID = "contact_batch:701:employee:101:chunk:1"
	second := base
	second.Dispatch.TargetID = "contact_batch:701:employee:202:chunk:1"
	firstResult, err := poller.Poll(context.Background(), first)
	if err != nil {
		t.Fatalf("first poll: %v", err)
	}
	secondResult, err := poller.Poll(context.Background(), second)
	if err != nil {
		t.Fatalf("second poll: %v", err)
	}
	if len(firstResult.Results) != 1 || len(secondResult.Results) != 1 {
		t.Fatalf("results first=%#v second=%#v", firstResult.Results, secondResult.Results)
	}
	if firstResult.Results[0].TargetKind != "employee_external_userid" || firstResult.Results[0].TargetID != "101:external-shared" {
		t.Fatalf("first identity=%#v", firstResult.Results[0])
	}
	if secondResult.Results[0].TargetID != "202:external-shared" || secondResult.Results[0].TargetID == firstResult.Results[0].TargetID {
		t.Fatalf("shared contact identities collapsed: first=%#v second=%#v", firstResult.Results[0], secondResult.Results[0])
	}
}

func TestContactBatchPollDoesNotTerminateWhileAnyTaskIsPending(t *testing.T) {
	poller := &ContactBatchDispatchPoller{
		store: fakeContactBatchPollRuntimeStore{},
		client: &fakeContactBatchPollClient{
			taskPages: map[string]BatchSendGroupTaskPage{
				"":       {TaskList: []BatchSendGroupTask{{UserID: "sender", Status: batchSendStatusSent}}, NextCursor: "page-2"},
				"page-2": {TaskList: []BatchSendGroupTask{{UserID: "sender", Status: batchSendMessageNotSent}}},
			},
			resultPages: map[string]BatchSendGroupResultPage{"sender|": {SendList: []BatchSendGroupResult{{UserID: "sender", ExternalUserID: "external-1", Status: batchSendMessageDelivered}}}},
		},
	}
	result, err := poller.Poll(context.Background(), wecomcapability.DispatchPollRequest{Principal: wecomcapability.DispatchPrincipal{TenantID: 1, CorpID: 7, UserID: 9}, Capability: wecomcapability.ContactBatchSend, Dispatch: wecomcapability.Dispatch{DispatchKind: string(wecomcapability.DispatchKindContactBatch), TargetID: "contact_batch:701:employee:101:chunk:1", ProviderMessageID: "msg-1"}})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Terminal || len(result.Results) != 1 {
		t.Fatalf("pending task prematurely terminal: %#v", result)
	}
}
