package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSidebarWorkbenchSummaryUsesResolvedEmployee(t *testing.T) {
	store := &fakeSidebarWorkbenchStore{
		employee: SidebarEmployee{ID: 7, CorpID: 9, LogUserID: 3},
		summary: SidebarWorkbenchSummary{
			Employee:  SidebarEmployeeProfile{ID: 7, Name: "员工甲", DepartmentNames: []string{"销售部"}, CorpName: "示例企业"},
			Customers: SidebarCustomerMetrics{Total: 12, AddedToday: 2, TaggedTotal: 4, OwnedRoomTotal: 1},
			Tasks:     SidebarTaskMetrics{ContactSOPRecords: 3, RoomSOPPending: 2, BatchAddPending: 1},
		},
	}
	handler := NewSidebarWorkbenchHandler(store, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})
	req := httptest.NewRequest(http.MethodGet, "/sidebar/workbench/summary", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "7")
	rec := httptest.NewRecorder()

	handler.Summary(rec, req)

	if rec.Code != http.StatusOK || store.summaryEmployee.ID != 7 || store.summaryEmployee.CorpID != 9 {
		t.Fatalf("code=%d employee=%+v body=%s", rec.Code, store.summaryEmployee, rec.Body.String())
	}
	var envelope struct {
		Code int                     `json:"code"`
		Data SidebarWorkbenchSummary `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 200 || envelope.Data.Employee.Name != "员工甲" || envelope.Data.Customers.Total != 12 {
		t.Fatalf("envelope=%+v", envelope)
	}
}

func TestSidebarWorkbenchRequiresEmployeeIdentity(t *testing.T) {
	store := &fakeSidebarWorkbenchStore{employee: SidebarEmployee{ID: 7, CorpID: 9}}
	handler := NewSidebarWorkbenchHandler(store, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "summary", path: "/sidebar/workbench/summary"},
		{name: "contacts", path: "/sidebar/workContact/index"},
		{name: "tasks", path: "/sidebar/workbench/tasks?kind=contactSop&state=recorded"},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			rec := httptest.NewRecorder()
			switch test.name {
			case "summary":
				handler.Summary(rec, req)
			case "contacts":
				handler.Contacts(rec, req)
			default:
				handler.Tasks(rec, req)
			}
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestSidebarWorkbenchContactsValidatesAndCapsPagination(t *testing.T) {
	store := &fakeSidebarWorkbenchStore{
		employee: SidebarEmployee{ID: 7, CorpID: 9},
		contacts: SidebarContactPage{Page: 2, PerPage: 50, Total: 0, TotalPage: 0, Items: []SidebarContactListItem{}},
	}
	handler := NewSidebarWorkbenchHandler(store, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})
	req := httptest.NewRequest(http.MethodGet, "/sidebar/workContact/index?keyword=%E5%AE%A2%E6%88%B7&page=2&perPage=999", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "7")
	rec := httptest.NewRecorder()

	handler.Contacts(rec, req)

	if rec.Code != http.StatusOK || store.contactFilter.Page != 2 || store.contactFilter.PerPage != 50 || store.contactFilter.Keyword != "客户" {
		t.Fatalf("code=%d filter=%+v body=%s", rec.Code, store.contactFilter, rec.Body.String())
	}
}

func TestSidebarWorkbenchTasksRejectsUnknownKindAndState(t *testing.T) {
	store := &fakeSidebarWorkbenchStore{employee: SidebarEmployee{ID: 7, CorpID: 9}}
	handler := NewSidebarWorkbenchHandler(store, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})
	for _, path := range []string{
		"/sidebar/workbench/tasks?kind=archive&state=pending",
		"/sidebar/workbench/tasks?kind=contactSop&state=unknown",
		"/sidebar/workbench/tasks?kind=contactSop&state=pending",
		"/sidebar/workbench/tasks?kind=roomSop&state=recorded",
		"/sidebar/workbench/tasks?kind=contactSop&state=recorded&page=bad",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Mochat-Go-Employee-ID", "7")
		rec := httptest.NewRecorder()
		handler.Tasks(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("path=%s code=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestSidebarWorkbenchRejectsNonGET(t *testing.T) {
	store := &fakeSidebarWorkbenchStore{employee: SidebarEmployee{ID: 7, CorpID: 9}}
	handler := NewSidebarWorkbenchHandler(store, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})
	req := httptest.NewRequest(http.MethodPost, "/sidebar/workbench/summary", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "7")
	rec := httptest.NewRecorder()
	handler.Summary(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

type fakeSidebarWorkbenchStore struct {
	employee SidebarEmployee
	summary  SidebarWorkbenchSummary
	contacts SidebarContactPage
	tasks    SidebarTaskPage

	summaryEmployee SidebarEmployee
	contactEmployee SidebarEmployee
	contactFilter   SidebarContactFilter
	taskEmployee    SidebarEmployee
	taskFilter      SidebarTaskFilter
}

func (s *fakeSidebarWorkbenchStore) SidebarEmployeeByID(_ context.Context, employeeID int) (SidebarEmployee, bool, error) {
	if employeeID != s.employee.ID {
		return SidebarEmployee{}, false, nil
	}
	return s.employee, true, nil
}

func (s *fakeSidebarWorkbenchStore) SidebarWorkbenchSummary(_ context.Context, employee SidebarEmployee) (SidebarWorkbenchSummary, error) {
	s.summaryEmployee = employee
	return s.summary, nil
}

func (s *fakeSidebarWorkbenchStore) SidebarContacts(_ context.Context, employee SidebarEmployee, filter SidebarContactFilter) (SidebarContactPage, error) {
	s.contactEmployee = employee
	s.contactFilter = filter
	return s.contacts, nil
}

func (s *fakeSidebarWorkbenchStore) SidebarTasks(_ context.Context, employee SidebarEmployee, filter SidebarTaskFilter) (SidebarTaskPage, error) {
	s.taskEmployee = employee
	s.taskFilter = filter
	return s.tasks, nil
}
