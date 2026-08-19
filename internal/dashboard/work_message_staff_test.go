package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestWorkMessageStaffDirectoryUsesPrincipalAndEmployeeScope(t *testing.T) {
	store := &fakeAutoTagStore{
		user: User{ID: 42, TenantID: 1},
		staffDirectory: WorkMessageStaffDirectoryPage{
			Employees: []WorkMessageStaffEmployee{{ID: 9, Name: "张伟", Status: 1, Archived: true}},
			Counts:    WorkMessageStaffCounts{All: 1, Archived: 1}, Page: 1, PageSize: 50, Total: 1,
		},
	}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{
		CorpID: 7, WorkEmployeeID: 9, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{9, 10},
	}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/staffDirectory?mode=focused&keyword=张&departmentId=10&page=2&pageSize=50", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "42")
	req = withAutoTagTestPrincipal(req, 42, 1, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageStaffDirectory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := store.staffDirectoryFilter; got.TenantID != 1 || got.CorpID != 7 || got.UserID != 42 ||
		got.Mode != WorkMessageStaffModeFocused || got.DepartmentID != 10 || got.Page != 2 || got.PageSize != 50 ||
		!got.RestrictEmployeeIDs || !reflect.DeepEqual(got.EmployeeIDs, []int{9, 10}) {
		t.Fatalf("filter=%#v", got)
	}
	var envelope struct {
		Data WorkMessageStaffDirectoryPage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Employees) != 1 || envelope.Data.Employees[0].Name != "张伟" {
		t.Fatalf("data=%#v", envelope.Data)
	}
	if envelope.Data.Employees[0].DepartmentIDs == nil {
		t.Fatal("departmentIds must be encoded as an empty array instead of null")
	}
}

func TestWorkMessageStaffDirectoryRejectsInvalidFilters(t *testing.T) {
	for _, rawQuery := range []string{"mode=unknown", "departmentId=nope", "pageSize=20"} {
		t.Run(rawQuery, func(t *testing.T) {
			store := &fakeAutoTagStore{user: User{ID: 42, TenantID: 1}}
			handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
			req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/staffDirectory?"+rawQuery, nil)
			req = withAutoTagTestPrincipal(req, 42, 1, 7)
			rec := httptest.NewRecorder()
			handler.WorkMessageStaffDirectory(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestWorkMessageStaffDetailParsesConversationAndFilters(t *testing.T) {
	store := &fakeAutoTagStore{
		user: User{ID: 42, TenantID: 1},
		staffDetail: WorkMessageStaffDetail{
			ConversationID: "9:1:31", EmployeeID: 9, TargetID: 31, TargetType: "customer",
			Stats:   WorkMessageStaffStats{CommunicationDays: 2, MessageTotal: 15, InboundTotal: 5, OutboundTotal: 10},
			HasMore: true,
		},
	}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{
		CorpID: 7, WorkEmployeeID: 9, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{9, 10},
	}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/staffDetail?conversationId=9:1:31&keyword=报价&messageTypes=image&messageTypes=file&date=2026-08-16&pageSize=50", nil)
	req = withAutoTagTestPrincipal(req, 42, 1, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageStaffDetail(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got := store.staffDetailFilter
	if got.EmployeeID != 9 || got.ToUserType != 1 || got.ToUserID != 31 || got.Date != "2026-08-16" || got.PageSize != 50 ||
		got.Keyword != "报价" || !reflect.DeepEqual(got.MessageTypes, []int{2, 5}) || !got.RestrictEmployeeIDs {
		t.Fatalf("filter=%#v", got)
	}
}

func TestParseWorkMessageConversationIDAllowsUnmappedRoomTarget(t *testing.T) {
	employeeID, targetType, targetID, ok := parseWorkMessageConversationID("1005:2:0")
	if !ok || employeeID != 1005 || targetType != 2 || targetID != 0 {
		t.Fatalf("room conversation should be accepted: %d:%d:%d ok=%v", employeeID, targetType, targetID, ok)
	}
	if _, _, _, ok := parseWorkMessageConversationID("1005:1:0"); ok {
		t.Fatal("zero customer id must remain invalid")
	}
}

func TestWorkMessageStaffDetailHidesEmployeeOutsideScope(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 42, TenantID: 1}}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{
		CorpID: 7, WorkEmployeeID: 10, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{10},
	}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-10"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/staffDetail?conversationId=9:1:31&pageSize=50", nil)
	req = withAutoTagTestPrincipal(req, 42, 1, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageStaffDetail(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func (s *fakeAutoTagStore) StaffDirectory(_ context.Context, filter WorkMessageStaffDirectoryFilter) (WorkMessageStaffDirectoryPage, error) {
	s.staffDirectoryFilter = filter
	return s.staffDirectory, nil
}

func (s *fakeAutoTagStore) StaffDetail(_ context.Context, filter WorkMessageStaffDetailFilter) (WorkMessageStaffDetail, error) {
	s.staffDetailFilter = filter
	return s.staffDetail, nil
}
