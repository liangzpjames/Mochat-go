package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestWorkMessageGlobalOverviewUsesPrincipalScopeAndParsesMessageTypes(t *testing.T) {
	metric := int64(8)
	store := &fakeAutoTagStore{
		user: User{ID: 42, TenantID: 1},
		globalOverview: WorkMessageGlobalOverview{Metrics: map[string]WorkMessageMetricValue{
			"customerConversations": {Value: &metric, Status: "available"},
		}},
	}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{
		CorpID: 7, WorkEmployeeID: 9, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{9, 10},
	}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/globalOverview?employeeIds=9&employeeIds=999&messageTypes=text&messageTypes=image&startAt=2026-08-01&endAt=2026-08-19", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "42")
	req = withAutoTagTestPrincipal(req, 42, 1, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageGlobalOverview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data WorkMessageGlobalOverview `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Metrics["customerConversations"].Value == nil || *envelope.Data.Metrics["customerConversations"].Value != 8 {
		t.Fatalf("data=%#v", envelope.Data)
	}
	if store.archiveTenantID != 1 || store.archiveCorpID != 7 {
		t.Fatalf("archive scope tenant=%d corp=%d", store.archiveTenantID, store.archiveCorpID)
	}
	if !reflect.DeepEqual(store.globalOverviewFilter.MessageTypes, []int{1, 100, 2}) {
		t.Fatalf("message types=%v, want text aliases and image", store.globalOverviewFilter.MessageTypes)
	}
}

func TestWorkMessageFocusRejectsOutOfScopeConversation(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 42, TenantID: 1}}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{
		CorpID: 7, WorkEmployeeID: 9, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{9},
	}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
	req := httptest.NewRequest(http.MethodPut, "/dashboard/workMessage/focus", strings.NewReader(`{"conversationId":"99:1:31"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "42")
	req = withAutoTagTestPrincipal(req, 42, 1, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageFocus(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.globalFocusCalls != 0 {
		t.Fatalf("out of scope focus calls=%d", store.globalFocusCalls)
	}
}

func TestWorkMessageGlobalOverviewRejectsInvalidMessageType(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 42, TenantID: 1}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/globalOverview?messageTypes=unknown", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "42")
	req = withAutoTagTestPrincipal(req, 42, 1, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageGlobalOverview(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
