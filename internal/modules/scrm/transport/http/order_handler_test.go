package http

import (
	"context"
	"encoding/json"
	"jiyi/mochat-go/internal/modules/scrm/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOrderHandlerRoutesListPathToListContext(t *testing.T) {
	repo := &routingOrderRepository{
		listItems: []domain.Order{{ID: "o-list", TenantID: 7, CorpID: 1536612155}},
	}
	h := NewOrderHandler(repo, routingPrincipalResolver{})
	req := httptest.NewRequest(http.MethodGet, "/dashboard/scrm/orders?corpId=1536612155&status=paid", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	assertOrderEnvelope(t, rec, http.StatusOK)
	if repo.listCalls != 1 {
		t.Fatalf("ListContext calls = %d, want 1", repo.listCalls)
	}
	if repo.getCalls != 0 || repo.auditCalls != 0 {
		t.Fatalf("detail repository calls = GetContext %d, AuditContext %d, want 0, 0", repo.getCalls, repo.auditCalls)
	}
}

func TestOrderHandlerRoutesDetailPathToGetAndAuditContext(t *testing.T) {
	repo := &routingOrderRepository{
		detail: domain.Order{ID: "o-detail", TenantID: 7, CorpID: 1536612155},
		audit:  []map[string]any{{"action": "created"}},
	}
	h := NewOrderHandler(repo, routingPrincipalResolver{})
	req := httptest.NewRequest(http.MethodGet, "/dashboard/scrm/orders/o-detail?corpId=1536612155&view=audit", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	assertOrderEnvelope(t, rec, http.StatusOK)
	var payload struct {
		Order domain.Order     `json:"order"`
		Audit []map[string]any `json:"audit"`
	}
	response := assertOrderEnvelope(t, rec, http.StatusOK)
	if err := json.Unmarshal(response.Data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Order.ID != "o-detail" || len(payload.Audit) != 1 {
		t.Fatalf("detail payload = %#v", payload)
	}
	if repo.getCalls != 1 || repo.auditCalls != 1 {
		t.Fatalf("detail repository calls = GetContext %d, AuditContext %d, want 1, 1", repo.getCalls, repo.auditCalls)
	}
	if repo.gotDetailID != "o-detail" {
		t.Fatalf("detail id = %q, want %q", repo.gotDetailID, "o-detail")
	}
	if repo.listCalls != 0 {
		t.Fatalf("ListContext calls = %d, want 0", repo.listCalls)
	}
}

func TestOrderHandlerCreatesAndListsScopedOrder(t *testing.T) {
	r := NewMemoryOrderRepository()
	h := NewOrderHandler(r)
	req := httptest.NewRequest("POST", "/scrm/orders", strings.NewReader(`{"id":"o1","tenantId":1,"corpId":1,"contactId":"c1","opportunityId":"opp1","title":"年度续费","note":"客户确认","amountCents":100,"status":"pending"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	response := assertOrderEnvelope(t, rec, http.StatusOK)
	var created domain.Order
	if err := json.Unmarshal(response.Data, &created); err != nil {
		t.Fatalf("decode created order: %v", err)
	}
	if created.ID != "o1" || created.Title != "年度续费" || created.Note != "客户确认" || created.OpportunityID != "opp1" {
		t.Fatalf("created order = %#v", created)
	}
	if len(r.List(1, 1)) != 1 {
		t.Fatal("not persisted")
	}
	if _, e := domain.NewOrder(domain.NewOrderInput{ID: "", TenantID: 1, CorpID: 1, ContactID: "c", Status: domain.OrderPending}); e == nil {
		t.Fatal("expected invalid")
	}
}

func TestOrderHandlerTransitionUsesDashboardEnvelope(t *testing.T) {
	repo := NewMemoryOrderRepository()
	_, err := repo.Create(domain.Order{ID: "o-transition", TenantID: 7, CorpID: 1536612155, ContactID: "c1", Status: domain.OrderPending, Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOrderHandler(repo, routingPrincipalResolver{})
	req := httptest.NewRequest(http.MethodPatch, "/dashboard/scrm/orders/o-transition/transition?corpId=1536612155", strings.NewReader(`{"status":"paid","version":1}`))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	assertOrderEnvelope(t, rec, http.StatusOK)
}

type orderEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func assertOrderEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, status int) orderEnvelope {
	t.Helper()
	var response orderEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode order envelope: %v; body = %q", err, recorder.Body.String())
	}
	if response.Code != status || response.Msg != "success" || len(response.Data) == 0 || string(response.Data) == "null" {
		t.Fatalf("response = %#v, want code=%d msg=success non-null data; body = %q", response, status, recorder.Body.String())
	}
	return response
}

type routingPrincipalResolver struct{}

func (routingPrincipalResolver) Resolve(*http.Request) (Principal, error) {
	return Principal{TenantID: 7, UserID: 11}, nil
}

type routingOrderRepository struct {
	listItems   []domain.Order
	detail      domain.Order
	audit       []map[string]any
	listCalls   int
	getCalls    int
	auditCalls  int
	gotDetailID string
}

func (r *routingOrderRepository) Create(order domain.Order) (domain.Order, error) {
	return order, nil
}

func (r *routingOrderRepository) List(int64, int64) []domain.Order {
	panic("legacy List must not be used when ListContext is available")
}

func (r *routingOrderRepository) Transition(string, int64, domain.OrderStatus, int64) (domain.Order, error) {
	return domain.Order{}, nil
}

func (r *routingOrderRepository) CreateContext(context.Context, domain.Order, int64) (domain.Order, error) {
	return domain.Order{}, nil
}

func (r *routingOrderRepository) ListContext(context.Context, int64, int64) ([]domain.Order, error) {
	r.listCalls++
	return r.listItems, nil
}

func (r *routingOrderRepository) TransitionContext(context.Context, string, int64, int64, domain.OrderStatus, int64, int64) (domain.Order, error) {
	return domain.Order{}, nil
}

func (r *routingOrderRepository) GetContext(_ context.Context, id string, _, _ int64) (domain.Order, error) {
	r.getCalls++
	r.gotDetailID = id
	return r.detail, nil
}

func (r *routingOrderRepository) AuditContext(context.Context, string, int64, int64) ([]map[string]any, error) {
	r.auditCalls++
	return r.audit, nil
}
