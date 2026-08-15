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

func TestRoomBatchDispatchStoreRejectsBodyTenantAndActorAndUsesDurableOperation(t *testing.T) {
	store := &fakeRoomBatchDispatchStore{
		fakeRoomMessageBatchSendStore: &fakeRoomMessageBatchSendStore{users: map[int]User{1: {ID: 1, TenantID: 1, Name: "admin"}}},
		result:                        RoomBatchDispatchResult{OperationID: 92, BatchID: 810701, Status: "pending"},
	}
	handler := NewRoomMessageBatchSendHandlerWithDispatch(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", t.TempDir(), nil)

	request := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomMessageBatchSend/store", strings.NewReader(`{"tenantId":99,"actorId":42,"idempotencyKey":"room-701","employeeIds":[11],"roomTargets":[{"ownerEmployeeId":11,"roomId":21}],"content":[{"msgType":"text","content":"hello"}]}`))
	request = request.WithContext(WithDashboardAccessContext(request.Context(), DashboardAccessContext{
		UserID: 1, TenantID: 1, CorpID: 7, PermissionCode: roomBatchDispatchPageCode,
		PermissionCodes: []string{roomBatchDispatchPageCode}, Scope: DataScopeTenant,
		IsSuperAdmin: true,
	}))
	request.Header.Set("X-Mochat-Go-User-ID", "1")
	response := httptest.NewRecorder()
	handler.Store(response, request)
	if response.Code != http.StatusBadRequest || store.calls != 0 {
		t.Fatalf("body tenant/actor status=%d calls=%d body=%s", response.Code, store.calls, response.Body.String())
	}

	request = authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomMessageBatchSend/store", strings.NewReader(`{"idempotencyKey":"room-701","employeeIds":[11],"roomTargets":[{"ownerEmployeeId":11,"roomId":21}],"content":[{"msgType":"text","content":"hello"}],"sendWay":1}`))
	request = request.WithContext(WithDashboardAccessContext(request.Context(), DashboardAccessContext{
		UserID: 1, TenantID: 1, CorpID: 7, PermissionCode: roomBatchDispatchPageCode,
		PermissionCodes: []string{roomBatchDispatchPageCode}, Scope: DataScopeTenant,
		IsSuperAdmin: true,
	}))
	request.Header.Set("X-Mochat-Go-User-ID", "1")
	response = httptest.NewRecorder()
	handler.Store(response, request)
	if response.Code != http.StatusOK || store.calls != 1 || store.lastPrincipal.TenantID != 1 || store.lastPrincipal.CorpID != 7 || store.lastPrincipal.UserID != 1 {
		t.Fatalf("durable create status=%d calls=%d principal=%#v body=%s", response.Code, store.calls, store.lastPrincipal, response.Body.String())
	}
}

func TestRoomBatchDurableConstructorFailsClosedWhenStoreIsNotDurable(t *testing.T) {
	store := &fakeRoomMessageBatchSendStore{users: map[int]User{1: {ID: 1, TenantID: 1, Name: "admin"}}}
	handler := NewRoomMessageBatchSendHandlerWithDispatch(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", t.TempDir(), nil)
	request := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomMessageBatchSend/store", strings.NewReader(`{"idempotencyKey":"room-701","employeeIds":[11],"roomTargets":[{"ownerEmployeeId":11,"roomId":21}],"content":[{"msgType":"text","content":"hello"}]}`))
	request = request.WithContext(WithDashboardAccessContext(request.Context(), DashboardAccessContext{
		UserID: 1, TenantID: 1, CorpID: 7, PermissionCodes: []string{roomBatchDispatchPageCode}, Scope: DataScopeTenant, IsSuperAdmin: true,
	}))
	request.Header.Set("X-Mochat-Go-User-ID", "1")
	response := httptest.NewRecorder()
	handler.Store(response, request)
	if response.Code != http.StatusServiceUnavailable || store.createCalls != 0 {
		t.Fatalf("missing durable dependency status=%d createCalls=%d body=%s", response.Code, store.createCalls, response.Body.String())
	}
}

func TestRoomBatchDurableBodyRejectsUnknownField(t *testing.T) {
	store := &fakeRoomBatchDispatchStore{
		fakeRoomMessageBatchSendStore: &fakeRoomMessageBatchSendStore{users: map[int]User{1: {ID: 1, TenantID: 1, Name: "admin"}}},
		result:                        RoomBatchDispatchResult{OperationID: 92, BatchID: 810701, Status: "pending"},
	}
	handler := NewRoomMessageBatchSendHandlerWithDispatch(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", t.TempDir(), nil)
	request := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomMessageBatchSend/store", strings.NewReader(`{"idempotencyKey":"room-701","employeeIds":[11],"roomTargets":[{"ownerEmployeeId":11,"roomId":21}],"content":[{"msgType":"text","content":"hello"}],"tenatId":1}`))
	request = request.WithContext(WithDashboardAccessContext(request.Context(), DashboardAccessContext{
		UserID: 1, TenantID: 1, CorpID: 7, PermissionCodes: []string{roomBatchDispatchPageCode}, Scope: DataScopeTenant, IsSuperAdmin: true,
	}))
	request.Header.Set("X-Mochat-Go-User-ID", "1")
	response := httptest.NewRecorder()
	handler.Store(response, request)
	if response.Code != http.StatusBadRequest || store.calls != 0 {
		t.Fatalf("unknown field status=%d calls=%d body=%s", response.Code, store.calls, response.Body.String())
	}
}

func TestRoomBatchDispatchScopeDenialRejectsTargetsOutsideEmployeeScope(t *testing.T) {
	store := &fakeRoomBatchDispatchStore{
		fakeRoomMessageBatchSendStore: &fakeRoomMessageBatchSendStore{users: map[int]User{1: {ID: 1, TenantID: 1, Name: "admin"}}},
		result:                        RoomBatchDispatchResult{OperationID: 92, BatchID: 810701, Status: "pending"},
	}
	handler := NewRoomMessageBatchSendHandlerWithDispatch(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", t.TempDir(), nil)
	// Owner 12 is not among the declared employee ids 11, so the target must be
	// rejected before the store is ever called.
	request := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomMessageBatchSend/store", strings.NewReader(`{"idempotencyKey":"room-701","employeeIds":[11],"roomTargets":[{"ownerEmployeeId":12,"roomId":21}],"content":[{"msgType":"text","content":"hello"}],"sendWay":1}`))
	request = request.WithContext(WithDashboardAccessContext(request.Context(), DashboardAccessContext{
		UserID: 1, TenantID: 1, CorpID: 7, PermissionCodes: []string{roomBatchDispatchPageCode}, Scope: DataScopeTenant, IsSuperAdmin: true,
	}))
	request.Header.Set("X-Mochat-Go-User-ID", "1")
	response := httptest.NewRecorder()
	handler.Store(response, request)
	if response.Code != http.StatusForbidden || store.calls != 0 || !strings.Contains(response.Body.String(), "ROOM_BATCH_SCOPE_DENIED") {
		t.Fatalf("scope denial status=%d calls=%d body=%s", response.Code, store.calls, response.Body.String())
	}
}

func TestRoomBatchDurableBodyRejectsUnsupportedFilterContentAndInvalidSchedule(t *testing.T) {
	h := &RoomMessageBatchSendHandler{}
	base := func() roomBatchDispatchBody {
		return roomBatchDispatchBody{
			BatchTitle: "room batch", EmployeeIDs: []int{11},
			RoomTargets:    []RoomBatchTarget{{OwnerEmployeeID: 11, RoomID: 21}},
			Content:        []ContactMessageBatchSendContent{{MsgType: "text", Content: "hello"}},
			IdempotencyKey: "room-701", SendWay: 1,
		}
	}
	for name, mutate := range map[string]func(*roomBatchDispatchBody){
		"non-empty filter": func(body *roomBatchDispatchBody) { body.FilterParams = json.RawMessage(`{"gender":1}`) },
		"unknown content type": func(body *roomBatchDispatchBody) {
			body.Content = []ContactMessageBatchSendContent{{MsgType: "unsupported", Content: "hello"}}
		},
		"incompatible content fields": func(body *roomBatchDispatchBody) {
			body.Content = []ContactMessageBatchSendContent{{MsgType: "text", Content: "hello", URL: "https://example.test"}}
		},
		"immediate with schedule": func(body *roomBatchDispatchBody) { body.DefiniteTime = "2026-08-15 12:00:00" },
		"scheduled invalid time":  func(body *roomBatchDispatchBody) { body.SendWay = 2; body.DefiniteTime = "2026-99-99 12:00:00" },
	} {
		t.Run(name, func(t *testing.T) {
			_, err := h.roomBatchDispatchInputFromBody(func() roomBatchDispatchBody { body := base(); mutate(&body); return body }(), User{ID: 1, Name: "actor"}, 7, DashboardAccessContext{Scope: DataScopeTenant})
			if err == nil {
				t.Fatal("invalid durable body was accepted")
			}
		})
	}
}

func TestRoomBatchErrorCodesKeepTenantSessionAndScopeSemanticsDistinct(t *testing.T) {
	cases := []struct {
		err    error
		code   string
		status int
	}{
		{ErrRoomBatchTenantDenied, "TENANT_ACCESS_DENIED", http.StatusForbidden},
		{ErrRoomBatchPermissionDenied, "DASHBOARD_PERMISSION_DENIED", http.StatusForbidden},
		{ErrRoomBatchTargetNotOwned, "ROOM_BATCH_SCOPE_DENIED", http.StatusForbidden},
		{ErrRoomBatchQuotaExceeded, "ROOM_BATCH_QUOTA_EXCEEDED", http.StatusConflict},
		{ErrRoomBatchCapabilityLimited, "ROOM_BATCH_CAPABILITY_LIMITED", http.StatusConflict},
		{ErrRoomBatchNotFound, "ROOM_BATCH_NOT_FOUND", http.StatusNotFound},
	}
	for _, item := range cases {
		code, status := roomBatchDispatchHTTPError(item.err)
		if code != item.code || status != item.status {
			t.Errorf("error=%v got code=%s status=%d, want code=%s status=%d", item.err, code, status, item.code, item.status)
		}
	}
}

func TestRoomBatchPollUsesOwnerRoomCompositeTargetIdentity(t *testing.T) {
	poller := &RoomBatchDispatchPoller{
		store: fakeRoomBatchPollRuntimeStore{},
		client: &fakeContactBatchPollClient{
			taskPages:   map[string]BatchSendGroupTaskPage{"": {TaskList: []BatchSendGroupTask{{UserID: "sender", Status: batchSendStatusSent}}}},
			resultPages: map[string]BatchSendGroupResultPage{"sender|": {SendList: []BatchSendGroupResult{{UserID: "sender", ChatID: "chat-shared", Status: batchSendMessageDelivered}}}},
		},
	}
	base := wecomcapability.DispatchPollRequest{Principal: wecomcapability.DispatchPrincipal{TenantID: 1, CorpID: 7, UserID: 9}, Capability: wecomcapability.RoomBatchSend, Dispatch: wecomcapability.Dispatch{DispatchKind: string(wecomcapability.DispatchKindRoomBatch), ProviderMessageID: "msg-1"}}
	first := base
	first.Dispatch.TargetID = "room_batch:701:owner:101:chunk:1"
	second := base
	second.Dispatch.TargetID = "room_batch:701:owner:202:chunk:1"
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
	if firstResult.Results[0].TargetKind != "employee_room_chat_id" || firstResult.Results[0].TargetID != "101:chat-shared" {
		t.Fatalf("first identity=%#v", firstResult.Results[0])
	}
	if secondResult.Results[0].TargetID != "202:chat-shared" || secondResult.Results[0].TargetID == firstResult.Results[0].TargetID {
		t.Fatalf("shared chat identities collapsed: first=%#v second=%#v", firstResult.Results[0], secondResult.Results[0])
	}
}

func TestRoomBatchPollDoesNotTerminateWhileAnyTaskIsPending(t *testing.T) {
	poller := &RoomBatchDispatchPoller{
		store: fakeRoomBatchPollRuntimeStore{},
		client: &fakeContactBatchPollClient{
			taskPages: map[string]BatchSendGroupTaskPage{
				"":       {TaskList: []BatchSendGroupTask{{UserID: "sender", Status: batchSendStatusSent}}, NextCursor: "page-2"},
				"page-2": {TaskList: []BatchSendGroupTask{{UserID: "sender", Status: batchSendMessageNotSent}}},
			},
			resultPages: map[string]BatchSendGroupResultPage{"sender|": {SendList: []BatchSendGroupResult{{UserID: "sender", ChatID: "chat-1", Status: batchSendMessageDelivered}}}},
		},
	}
	result, err := poller.Poll(context.Background(), wecomcapability.DispatchPollRequest{
		Principal: wecomcapability.DispatchPrincipal{TenantID: 1, CorpID: 7, UserID: 9}, Capability: wecomcapability.RoomBatchSend,
		Dispatch: wecomcapability.Dispatch{DispatchKind: string(wecomcapability.DispatchKindRoomBatch), TargetID: "room_batch:701:owner:101:chunk:1", ProviderMessageID: "msg-1"},
	})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Terminal || len(result.Results) != 1 {
		t.Fatalf("pending task prematurely terminal: %#v", result)
	}
}

type fakeRoomBatchDispatchStore struct {
	*fakeRoomMessageBatchSendStore
	result        RoomBatchDispatchResult
	calls         int
	createCalls   int
	lastPrincipal dashboardprincipal.DashboardPrincipal
}

func (s *fakeRoomBatchDispatchStore) CreateRoomBatchDispatch(_ context.Context, principal dashboardprincipal.DashboardPrincipal, _ DashboardAccessContext, _ RoomBatchDispatchInput) (RoomBatchDispatchResult, error) {
	s.calls++
	s.createCalls++
	s.lastPrincipal = principal
	return s.result, nil
}

type fakeRoomBatchPollRuntimeStore struct{}

func (fakeRoomBatchPollRuntimeStore) RoomBatchDispatchPayload(context.Context, dashboardprincipal.DashboardPrincipal, wecomcapability.Dispatch) (RoomMessageBatchSendMessagePayload, error) {
	return RoomMessageBatchSendMessagePayload{}, nil
}

func (fakeRoomBatchPollRuntimeStore) RoomBatchDispatchCredential(context.Context, dashboardprincipal.DashboardPrincipal) (RoomWelcomeCorpCredential, bool, error) {
	return RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "contact-secret"}, true, nil
}
