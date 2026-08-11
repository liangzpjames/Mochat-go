package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/modules/reporting"
)

type resolverStub struct{}

func (resolverStub) Resolve(*http.Request) (Principal, error) {
	return Principal{UserID: 7, TenantID: 7, CorpID: 9}, nil
}

type serviceStub struct{}

func (serviceStub) Query(context.Context, reporting.ReportKind, reporting.ReportQuery) (reporting.ReportResult, error) {
	return reporting.ReportResult{}, nil
}

func TestUnknownReportReturns422(t *testing.T) {
	handler := NewHandler(serviceStub{}, resolverStub{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/reports/unknown?corpId=9&timezone=Asia%2FShanghai&startAt=2026-08-01T00:00:00Z&endAt=2026-08-02T00:00:00Z", nil)
	req.SetPathValue("kind", "unknown")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestReportResponseUsesSharedMsgEnvelope(t *testing.T) {
	handler := NewHandler(serviceStub{}, resolverStub{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/reports/customer?corpId=9&timezone=Asia%2FShanghai&startAt=2026-08-01T00:00:00Z&endAt=2026-08-02T00:00:00Z", nil)
	req.SetPathValue("kind", "customer")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	var envelope map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["msg"] != "success" {
		t.Fatalf("msg=%v body=%s", envelope["msg"], recorder.Body.String())
	}
	if _, ok := envelope["message"]; ok {
		t.Fatalf("legacy message field present: %s", recorder.Body.String())
	}
}

func TestParseQueryEmployeeDepartmentFilters(t *testing.T) {
	for _, tc := range []struct {
		name            string
		query           string
		wantEmployees   []int64
		wantDepartments []int64
	}{
		{name: "plural comma separated", query: "corpId=9&timezone=Asia%2FShanghai&startAt=2026-08-01T00:00:00Z&endAt=2026-08-02T00:00:00Z&employeeIds=4,5&departmentIds=1,2", wantEmployees: []int64{4, 5}, wantDepartments: []int64{1, 2}},
		{name: "singular fallback", query: "corpId=9&timezone=Asia%2FShanghai&startAt=2026-08-01T00:00:00Z&endAt=2026-08-02T00:00:00Z&employeeId=4&departmentId=1", wantEmployees: []int64{4}, wantDepartments: []int64{1}},
		{name: "absent filters stay empty", query: "corpId=9&timezone=Asia%2FShanghai&startAt=2026-08-01T00:00:00Z&endAt=2026-08-02T00:00:00Z", wantEmployees: []int64{}, wantDepartments: []int64{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/dashboard/reports/customer?"+tc.query, nil)
			query, err := parseQuery(req, Principal{TenantID: 7, CorpID: 9})
			if err != nil {
				t.Fatal(err)
			}
			if len(query.EmployeeIDs) != len(tc.wantEmployees) {
				t.Fatalf("employees=%v want=%v", query.EmployeeIDs, tc.wantEmployees)
			}
			for i := range tc.wantEmployees {
				if query.EmployeeIDs[i] != tc.wantEmployees[i] {
					t.Fatalf("employees=%v want=%v", query.EmployeeIDs, tc.wantEmployees)
				}
			}
			if len(query.DepartmentIDs) != len(tc.wantDepartments) {
				t.Fatalf("departments=%v want=%v", query.DepartmentIDs, tc.wantDepartments)
			}
			for i := range tc.wantDepartments {
				if query.DepartmentIDs[i] != tc.wantDepartments[i] {
					t.Fatalf("departments=%v want=%v", query.DepartmentIDs, tc.wantDepartments)
				}
			}
		})
	}
}

func TestParseQueryConversionStage(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/dashboard/reports/conversion?corpId=9&timezone=Asia%2FShanghai&startAt=2026-08-01T00:00:00Z&endAt=2026-08-02T00:00:00Z&stage=won&page=2&pageSize=10", nil)
	query, err := parseQuery(req, Principal{TenantID: 7, CorpID: 9})
	if err != nil {
		t.Fatal(err)
	}
	if query.Stage != "won" {
		t.Fatalf("stage=%q want won", query.Stage)
	}
	if query.Page != 2 || query.PageSize != 10 {
		t.Fatalf("page=%d pageSize=%d want 2/10", query.Page, query.PageSize)
	}
}

type recordingAuthorizer struct {
	permission string
}

func (a *recordingAuthorizer) Authorize(_ context.Context, _ Principal, _ int64, permission string) error {
	a.permission = permission
	return nil
}

func TestOverviewReportUsesOverviewMenuPermission(t *testing.T) {
	authorizer := &recordingAuthorizer{}
	handler := NewHandler(serviceStub{}, resolverStub{}, authorizer)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/reports/overview?corpId=9&timezone=Asia%2FShanghai&startAt=2026-08-01T00:00:00Z&endAt=2026-08-02T00:00:00Z", nil)
	req.SetPathValue("kind", "overview")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if authorizer.permission != "/dashboard/corpData/index#get" {
		t.Fatalf("permission=%q want /dashboard/corpData/index#get", authorizer.permission)
	}
}

func TestReportKindUsesDataReportMenuPermission(t *testing.T) {
	authorizer := &recordingAuthorizer{}
	handler := NewHandler(serviceStub{}, resolverStub{}, authorizer)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/reports/report?corpId=9&timezone=Asia%2FShanghai&startAt=2026-08-01T00:00:00Z&endAt=2026-08-02T00:00:00Z", nil)
	req.SetPathValue("kind", "report")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if authorizer.permission != "/data/report#get" {
		t.Fatalf("permission=%q want /data/report#get", authorizer.permission)
	}
}
