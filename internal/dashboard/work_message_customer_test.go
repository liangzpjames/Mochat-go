package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type fakeWorkMessageCustomerStore struct {
	*fakeAutoTagStore
	directory          WorkMessageCustomerDirectoryPage
	directoryFilter    WorkMessageCustomerDirectoryFilter
	conversations      WorkMessageCustomerConversationPage
	conversationFilter WorkMessageCustomerConversationFilter
	detail             WorkMessageCustomerDetail
	detailFilter       WorkMessageCustomerDetailFilter
}

func (s *fakeWorkMessageCustomerStore) WorkMessageCustomerDirectory(_ context.Context, filter WorkMessageCustomerDirectoryFilter) (WorkMessageCustomerDirectoryPage, error) {
	s.directoryFilter = filter
	return s.directory, nil
}

func (s *fakeWorkMessageCustomerStore) WorkMessageCustomerConversations(_ context.Context, filter WorkMessageCustomerConversationFilter) (WorkMessageCustomerConversationPage, error) {
	s.conversationFilter = filter
	return s.conversations, nil
}

func (s *fakeWorkMessageCustomerStore) WorkMessageCustomerDetail(_ context.Context, filter WorkMessageCustomerDetailFilter) (WorkMessageCustomerDetail, error) {
	s.detailFilter = filter
	return s.detail, nil
}

func newWorkMessageCustomerStore() *fakeWorkMessageCustomerStore {
	return &fakeWorkMessageCustomerStore{
		fakeAutoTagStore: &fakeAutoTagStore{user: User{ID: 42, TenantID: 1}},
	}
}

func customerDashboardRequest(target string) *http.Request {
	return authenticatedDashboardRequestForTestAs(http.MethodGet, target, nil, 42, 1, 7, 9)
}

func TestWorkMessageCustomerDirectoryUsesPrincipalScope(t *testing.T) {
	store := newWorkMessageCustomerStore()
	store.directory = WorkMessageCustomerDirectoryPage{
		Customers: []WorkMessageCustomerDirectoryItem{}, Counts: WorkMessageCustomerCounts{}, Page: 1, PageSize: 50,
		Limitations: []WorkMessageCustomerLimitation{}, Capabilities: WorkMessageCustomerCapabilities(),
	}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{CorpID: 7, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{9, 10}}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
	rec := httptest.NewRecorder()

	handler.WorkMessageCustomerDirectory(rec, customerDashboardRequest("/dashboard/workMessage/customerDirectory?mode=focused&keyword=%E9%99%88&page=1&pageSize=50&corpId=999"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got := store.directoryFilter
	if got.TenantID != 1 || got.CorpID != 7 || got.UserID != 42 || got.Mode != WorkMessageCustomerModeFocused || got.Keyword != "陈" || got.Page != 1 || got.PageSize != 50 || !got.RestrictEmployeeIDs || !reflect.DeepEqual(got.EmployeeIDs, []int{9, 10}) {
		t.Fatalf("filter=%#v", got)
	}
}

func TestWorkMessageCustomerConversationsUsesPrincipalScope(t *testing.T) {
	store := newWorkMessageCustomerStore()
	store.conversations = WorkMessageCustomerConversationPage{Customer: WorkMessageCustomerProfile{ID: 31}, Mode: WorkMessageCustomerConversationModeGroup, List: []WorkMessageCustomerConversation{}, Page: 2, PageSize: 20, Capabilities: WorkMessageCustomerCapabilities()}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{CorpID: 7, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{9, 10}}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{}, authorizer)
	rec := httptest.NewRecorder()

	handler.WorkMessageCustomerConversations(rec, customerDashboardRequest("/dashboard/workMessage/customerConversations?customerId=31&mode=group&page=2&pageSize=20&corpId=999"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got := store.conversationFilter
	if got.TenantID != 1 || got.CorpID != 7 || got.UserID != 42 || got.CustomerID != 31 || got.Mode != WorkMessageCustomerConversationModeGroup || got.Page != 2 || got.PageSize != 20 || !got.RestrictEmployeeIDs || !reflect.DeepEqual(got.EmployeeIDs, []int{9, 10}) {
		t.Fatalf("filter=%#v", got)
	}
}

func TestWorkMessageCustomerDetailParsesCanonicalFilters(t *testing.T) {
	store := newWorkMessageCustomerStore()
	store.detail = WorkMessageCustomerDetail{WorkMessageStaffDetail: WorkMessageStaffDetail{ConversationID: "9:1:31", Messages: []WorkMessageStaffMessage{}, Capabilities: WorkMessageCustomerCapabilities()}, CustomerID: 31, CustomerName: "陈晓明"}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{CorpID: 7, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{9, 10}}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{}, authorizer)
	rec := httptest.NewRecorder()

	handler.WorkMessageCustomerDetail(rec, customerDashboardRequest("/dashboard/workMessage/customerDetail?customerId=31&conversationId=9:1:31&keyword=%E6%8A%A5%E4%BB%B7&messageTypes=image&messageTypes=file&date=2026-08-16&pageSize=50"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got := store.detailFilter
	if got.TenantID != 1 || got.CorpID != 7 || got.UserID != 42 || got.CustomerID != 31 || got.EmployeeID != 9 || got.ToUserType != 1 || got.ToUserID != 31 || got.Keyword != "报价" || got.Date != "2026-08-16" || got.PageSize != 50 || !got.RestrictEmployeeIDs || !reflect.DeepEqual(got.EmployeeIDs, []int{9, 10}) || !reflect.DeepEqual(got.MessageTypes, []int{2, 5}) {
		t.Fatalf("filter=%#v", got)
	}
}

func TestWorkMessageCustomerRejectsInvalidFilters(t *testing.T) {
	tests := []struct {
		name   string
		target string
		call   func(*AutoTagHandler, http.ResponseWriter, *http.Request)
	}{
		{"directory mode", "/dashboard/workMessage/customerDirectory?mode=unknown&pageSize=50", (*AutoTagHandler).WorkMessageCustomerDirectory},
		{"directory pageSize", "/dashboard/workMessage/customerDirectory?pageSize=20", (*AutoTagHandler).WorkMessageCustomerDirectory},
		{"conversations customerId", "/dashboard/workMessage/customerConversations?customerId=nope&pageSize=20", (*AutoTagHandler).WorkMessageCustomerConversations},
		{"conversations mode", "/dashboard/workMessage/customerConversations?customerId=31&mode=unknown&pageSize=20", (*AutoTagHandler).WorkMessageCustomerConversations},
		{"conversations pageSize", "/dashboard/workMessage/customerConversations?customerId=31&pageSize=50", (*AutoTagHandler).WorkMessageCustomerConversations},
		{"detail customerId", "/dashboard/workMessage/customerDetail?customerId=0&conversationId=9:1:31&pageSize=50", (*AutoTagHandler).WorkMessageCustomerDetail},
		{"detail conversationId", "/dashboard/workMessage/customerDetail?customerId=31&conversationId=bad&pageSize=50", (*AutoTagHandler).WorkMessageCustomerDetail},
		{"detail date", "/dashboard/workMessage/customerDetail?customerId=31&conversationId=9:1:31&date=bad&pageSize=50", (*AutoTagHandler).WorkMessageCustomerDetail},
		{"detail messageTypes", "/dashboard/workMessage/customerDetail?customerId=31&conversationId=9:1:31&messageTypes=invalid&pageSize=50", (*AutoTagHandler).WorkMessageCustomerDetail},
		{"detail pageSize", "/dashboard/workMessage/customerDetail?customerId=31&conversationId=9:1:31&pageSize=20", (*AutoTagHandler).WorkMessageCustomerDetail},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newWorkMessageCustomerStore()
			handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{}, nil)
			rec := httptest.NewRecorder()

			test.call(handler, rec, customerDashboardRequest(test.target))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestWorkMessageCustomerArchiveAndRBACFailures(t *testing.T) {
	t.Run("archive unauthorized uses capability code", func(t *testing.T) {
		store := newWorkMessageCustomerStore()
		store.archiveAuthorizationSet = true
		store.archiveAuthorized = false
		handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{}, nil)
		rec := httptest.NewRecorder()

		handler.WorkMessageCustomerDirectory(rec, customerDashboardRequest("/dashboard/workMessage/customerDirectory?pageSize=50"))

		body := decodeBody(t, rec.Body.Bytes())
		if rec.Code != http.StatusForbidden || body["code"] != float64(40301) {
			t.Fatalf("status=%d body=%#v", rec.Code, body)
		}
	})

	t.Run("rbac denied", func(t *testing.T) {
		store := newWorkMessageCustomerStore()
		handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{}, &recordingAuthorizer{err: ErrPermissionDenied})
		rec := httptest.NewRecorder()

		handler.WorkMessageCustomerDirectory(rec, customerDashboardRequest("/dashboard/workMessage/customerDirectory?pageSize=50"))

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestWorkMessageCustomerDetailHidesEmployeeOutsideScope(t *testing.T) {
	store := newWorkMessageCustomerStore()
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{CorpID: 7, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{10}}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-10"), HeaderUserIDResolver{}, authorizer)
	rec := httptest.NewRecorder()

	handler.WorkMessageCustomerDetail(rec, customerDashboardRequest("/dashboard/workMessage/customerDetail?customerId=31&conversationId=9:1:31&pageSize=50"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkMessageCustomerUnimplementedStoreReturnsCapabilityError(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 42, TenantID: 1}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{}, nil)
	rec := httptest.NewRecorder()

	handler.WorkMessageCustomerDirectory(rec, customerDashboardRequest("/dashboard/workMessage/customerDirectory?pageSize=50"))

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
