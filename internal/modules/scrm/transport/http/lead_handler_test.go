package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/application"
	"jiyi/mochat-go/internal/modules/scrm/domain"
)

func TestCreateLeadReturnsCreatedAndUsesPrincipalTenant(t *testing.T) {
	now := time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)
	service := &fakeLeadService{createResult: application.CreateLeadResult{
		Created: true,
		Lead: application.LeadView{
			ID: "generated-id", TenantID: 41, BusinessKey: "manual:2026-0001",
			Name: "Example", Source: domain.LeadSourceManual, Status: domain.LeadStatusNew,
			Version: 1, CreatedAt: now, UpdatedAt: now,
		},
	}}
	handler := NewLeadHandler(service, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41, CorpID: 41}})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(nethttp.MethodPost, LeadsPath, strings.NewReader(
		`{"businessKey":"manual:2026-0001","name":"Example","source":"manual"}`,
	))
	handler.Create(response, request)

	if response.Code != nethttp.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	if service.createCommand.TenantID != 41 {
		t.Fatalf("service tenant ID = %d, want principal tenant 41", service.createCommand.TenantID)
	}
	assertResponseDoesNotExposeTenant(t, response.Body.Bytes())
	var envelope struct {
		Data leadJSON `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.ID != "generated-id" || envelope.Data.CreatedAt != "2026-07-30T00:00:00Z" {
		t.Fatalf("data = %#v", envelope.Data)
	}
}

func TestCreateLeadReturnsOKWithSameObjectForIdempotentRetry(t *testing.T) {
	service := &fakeLeadService{createResult: application.CreateLeadResult{
		Created: false,
		Lead: application.LeadView{
			ID: "existing-id", BusinessKey: "manual:2026-0001", Name: "Existing",
			Source: domain.LeadSourceImport, Status: domain.LeadStatusNew, Version: 1,
		},
	}}
	handler := NewLeadHandler(service, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41, CorpID: 41}})

	response := httptest.NewRecorder()
	handler.Create(response, httptest.NewRequest(nethttp.MethodPost, LeadsPath, strings.NewReader(
		`{"businessKey":"manual:2026-0001","name":"Ignored on retry","source":"manual"}`,
	)))

	if response.Code != nethttp.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	var envelope struct {
		Data leadJSON `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.ID != "existing-id" || envelope.Data.Name != "Existing" || envelope.Data.Source != "import" {
		t.Fatalf("data = %#v", envelope.Data)
	}
}

func TestCreateLeadRejectsClientTenantAndStrictJSONViolations(t *testing.T) {
	tooLarge := `{"businessKey":"key","name":"` + strings.Repeat("x", int(MaxRequestBodyBytes)) + `","source":"manual"}`
	for _, tc := range []struct {
		name       string
		body       string
		wantStatus int
		wantCalls  int
	}{
		{name: "tenant ID is ignored", body: `{"tenantId":99,"businessKey":"key","name":"Ada","source":"manual"}`, wantStatus: nethttp.StatusOK, wantCalls: 1},
		{name: "unknown field", body: `{"businessKey":"key","name":"Ada","source":"manual","extra":true}`, wantStatus: nethttp.StatusBadRequest},
		{name: "trailing JSON", body: `{"businessKey":"key","name":"Ada","source":"manual"} {}`, wantStatus: nethttp.StatusBadRequest},
		{name: "too large", body: tooLarge, wantStatus: nethttp.StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeLeadService{}
			handler := NewLeadHandler(service, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41, CorpID: 41}})
			response := httptest.NewRecorder()

			handler.Create(response, httptest.NewRequest(nethttp.MethodPost, LeadsPath, strings.NewReader(tc.body)))

			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, tc.wantStatus, response.Body)
			}
			if service.createCalls != tc.wantCalls {
				t.Fatalf("service create calls = %d, want %d", service.createCalls, tc.wantCalls)
			}
		})
	}
}

func TestHandlersRejectMissingOrInvalidPrincipal(t *testing.T) {
	for _, tc := range []struct {
		name     string
		resolver fakePrincipalResolver
	}{
		{name: "missing", resolver: fakePrincipalResolver{err: errors.New("missing principal")}},
		{name: "missing user", resolver: fakePrincipalResolver{principal: Principal{TenantID: 41}}},
		{name: "missing tenant", resolver: fakePrincipalResolver{principal: Principal{UserID: 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeLeadService{}
			handler := NewLeadHandler(service, tc.resolver)
			for _, request := range []*nethttp.Request{
				httptest.NewRequest(nethttp.MethodPost, LeadsPath, strings.NewReader(`{}`)),
				httptest.NewRequest(nethttp.MethodGet, LeadsPath, nil),
			} {
				response := httptest.NewRecorder()
				if request.Method == nethttp.MethodPost {
					handler.Create(response, request)
				} else {
					handler.List(response, request)
				}
				if response.Code != nethttp.StatusUnauthorized {
					t.Fatalf("%s status = %d, body = %s", request.Method, response.Code, response.Body)
				}
			}
			if service.createCalls != 0 || service.listCalls != 0 {
				t.Fatalf("service calls = create %d, list %d", service.createCalls, service.listCalls)
			}
		})
	}
}

func TestHandlersMapPrincipalBackendUnavailableToServiceUnavailable(t *testing.T) {
	service := &fakeLeadService{}
	handler := NewLeadHandler(service, fakePrincipalResolver{
		err: fmt.Errorf("%w: redis password and mysql DSN", ErrPrincipalUnavailable),
	})
	for _, request := range []*nethttp.Request{
		httptest.NewRequest(nethttp.MethodPost, LeadsPath, strings.NewReader(`{}`)),
		httptest.NewRequest(nethttp.MethodGet, LeadsPath, nil),
	} {
		response := httptest.NewRecorder()
		if request.Method == nethttp.MethodPost {
			handler.Create(response, request)
		} else {
			handler.List(response, request)
		}
		if response.Code != nethttp.StatusServiceUnavailable {
			t.Fatalf("%s status = %d, body = %s", request.Method, response.Code, response.Body)
		}
		body := strings.ToLower(response.Body.String())
		if strings.Contains(body, "redis") || strings.Contains(body, "mysql") || strings.Contains(body, "dsn") {
			t.Fatalf("%s response leaked backend detail: %s", request.Method, response.Body)
		}
	}
	if service.createCalls != 0 || service.listCalls != 0 {
		t.Fatalf("service calls = create %d, list %d", service.createCalls, service.listCalls)
	}
}

func TestCreateLeadMapsValidationAndUnavailableErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "validation", err: application.ErrInvalidArgument, wantStatus: nethttp.StatusUnprocessableEntity},
		{name: "unavailable", err: application.ErrUnavailable, wantStatus: nethttp.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeLeadService{createErr: tc.err}
			handler := NewLeadHandler(service, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41, CorpID: 41}})
			response := httptest.NewRecorder()
			handler.Create(response, httptest.NewRequest(nethttp.MethodPost, LeadsPath, strings.NewReader(
				`{"businessKey":"key","name":"Ada","source":"manual"}`,
			)))
			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, tc.wantStatus, response.Body)
			}
		})
	}
}

func TestListLeadsUsesPrincipalTenantAndReturnsOnlyServiceResults(t *testing.T) {
	now := time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)
	included, err := domain.NewLead("lead-41", 41, "key-41", "Included", domain.LeadSourceManual, now)
	if err != nil {
		t.Fatal(err)
	}
	service := &fakeLeadService{listPage: application.LeadPage{
		Items:      []domain.Lead{included},
		NextCursor: "next-41",
	}}
	handler := NewLeadHandler(service, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41, CorpID: 41}})
	response := httptest.NewRecorder()
	cursor := base64.RawURLEncoding.EncodeToString([]byte(`{"created_at":"2026-07-30T00:00:00Z","id":"previous"}`))

	handler.List(response, httptest.NewRequest(nethttp.MethodGet, LeadsPath+"?cursor="+cursor+"&pageSize=3", nil))

	if response.Code != nethttp.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	if service.listQuery.TenantID != 41 || service.listQuery.Cursor != cursor || service.listQuery.PageSize != 3 {
		t.Fatalf("query = %#v", service.listQuery)
	}
	assertResponseDoesNotExposeTenant(t, response.Body.Bytes())
	var envelope struct {
		Data struct {
			Items      []leadJSON `json:"items"`
			NextCursor string     `json:"nextCursor"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Items) != 1 || envelope.Data.Items[0].ID != "lead-41" || envelope.Data.NextCursor != "next-41" {
		t.Fatalf("data = %#v", envelope.Data)
	}
}

func TestListLeadsRejectsMalformedCursorAndPageSize(t *testing.T) {
	for _, rawQuery := range []string{
		"cursor=-1",
		"pageSize=nope",
		"pageSize=-1",
		"pageSize=1&pageSize=2",
		"cursor=one&cursor=two",
	} {
		t.Run(rawQuery, func(t *testing.T) {
			service := &fakeLeadService{}
			handler := NewLeadHandler(service, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41, CorpID: 41}})
			response := httptest.NewRecorder()
			handler.List(response, httptest.NewRequest(nethttp.MethodGet, LeadsPath+"?"+rawQuery, nil))

			if response.Code != nethttp.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body)
			}
			if service.listCalls != 0 {
				t.Fatalf("service list calls = %d, want 0", service.listCalls)
			}
		})
	}
}

func TestListLeadsRejectsMalformedRawQueryBeforeCallingService(t *testing.T) {
	for _, rawQuery := range []string{
		"cursor=%ZZ",
		"cursor=valid;pageSize=3",
		"pageSize=%ZZ",
	} {
		t.Run(rawQuery, func(t *testing.T) {
			service := &fakeLeadService{}
			handler := NewLeadHandler(service, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41, CorpID: 41}})
			request := httptest.NewRequest(nethttp.MethodGet, LeadsPath, nil)
			request.URL.RawQuery = rawQuery
			response := httptest.NewRecorder()

			handler.List(response, request)

			if response.Code != nethttp.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body)
			}
			if service.listCalls != 0 {
				t.Fatalf("service list calls = %d, want 0", service.listCalls)
			}
		})
	}
}

func TestListLeadsMapsRepositoryUnavailable(t *testing.T) {
	service := &fakeLeadService{listErr: application.ErrUnavailable}
	handler := NewLeadHandler(service, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41, CorpID: 41}})
	response := httptest.NewRecorder()

	handler.List(response, httptest.NewRequest(nethttp.MethodGet, LeadsPath, nil))

	if response.Code != nethttp.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
}

func TestListLeadsMapsMalformedRepositoryCursorToBadRequest(t *testing.T) {
	service := &fakeLeadService{listErr: application.ErrInvalidArgument}
	handler := NewLeadHandler(service, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41, CorpID: 41}})
	response := httptest.NewRecorder()

	handler.List(response, httptest.NewRequest(nethttp.MethodGet, LeadsPath+"?cursor=bogus", nil))

	if response.Code != nethttp.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
}

func TestRegisterRoutesRegistersOnlyPostAndGetForLeads(t *testing.T) {
	router := &recordingRegistrar{}
	handler := NewLeadHandler(&fakeLeadService{}, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41, CorpID: 41}})
	if err := RegisterRoutes(router, handler); err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{nethttp.MethodPost, nethttp.MethodGet} {
		if _, ok := router.routes[method+" "+LeadsPath]; !ok {
			t.Fatalf("%s route was not registered", method)
		}
	}
	for _, method := range []string{nethttp.MethodPut, nethttp.MethodPatch, nethttp.MethodDelete} {
		if _, ok := router.routes[method+" "+LeadsPath]; ok {
			t.Fatalf("%s route must not be registered", method)
		}
	}
}

type recordingRegistrar struct {
	routes map[string]nethttp.Handler
}

func (r *recordingRegistrar) Handle(method, pattern string, handler nethttp.Handler) error {
	if r.routes == nil {
		r.routes = make(map[string]nethttp.Handler)
	}
	r.routes[method+" "+pattern] = handler
	return nil
}

type leadJSON struct {
	ID          string `json:"id"`
	BusinessKey string `json:"businessKey"`
	Name        string `json:"name"`
	Source      string `json:"source"`
	Status      string `json:"status"`
	Version     int64  `json:"version"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func assertResponseDoesNotExposeTenant(t *testing.T, body []byte) {
	t.Helper()
	if bytes.Contains(bytes.ToLower(body), []byte("tenant")) {
		t.Fatalf("response exposes tenant: %s", body)
	}
}

type fakePrincipalResolver struct {
	principal Principal
	err       error
}

func (r fakePrincipalResolver) Resolve(*nethttp.Request) (Principal, error) {
	return r.principal, r.err
}

type fakeLeadService struct {
	createResult  application.CreateLeadResult
	createErr     error
	createCommand application.CreateLeadCommand
	createCalls   int
	listPage      application.LeadPage
	listErr       error
	listQuery     application.ListLeadsQuery
	listCalls     int
}

func (s *fakeLeadService) CreateLead(_ context.Context, command application.CreateLeadCommand) (application.CreateLeadResult, error) {
	s.createCalls++
	s.createCommand = command
	return s.createResult, s.createErr
}

func (s *fakeLeadService) ListLeads(_ context.Context, query application.ListLeadsQuery) (application.LeadPage, error) {
	s.listCalls++
	s.listQuery = query
	return s.listPage, s.listErr
}

func (s *fakeLeadService) AssignLeads(context.Context, application.AssignLeadsCommand) ([]application.LeadMutationResult, error) {
	return nil, nil
}
func (s *fakeLeadService) TransitionLead(context.Context, application.TransitionLeadCommand) (application.LeadView, error) {
	return application.LeadView{}, nil
}
func (s *fakeLeadService) FindDuplicateLeads(context.Context, int64, int64, string, string) ([]application.LeadView, error) {
	return nil, nil
}
