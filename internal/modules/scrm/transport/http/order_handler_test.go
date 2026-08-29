package http

import (
	"context"
	"encoding/json"
	"errors"
	"jiyi/mochat-go/internal/modules/scrm/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestOrderHandlerRoutesListPathToListContext(t *testing.T) {
	repo := &routingOrderRepository{
		listItems: []domain.Order{{ID: "o-list", TenantID: 7, CorpID: 1536612155}},
		listTotal: 12,
	}
	h := NewOrderHandler(repo, routingPrincipalResolver{})
	req := httptest.NewRequest(http.MethodGet, "/dashboard/scrm/orders?corpId=1536612155&status=paid&page=2&pageSize=5", nil)
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
	if repo.listPage != 2 || repo.listPageSize != 5 {
		t.Fatalf("list page/pageSize = %d/%d, want 2/5", repo.listPage, repo.listPageSize)
	}
	var payload struct {
		Items    []domain.Order `json:"items"`
		Total    int            `json:"total"`
		Page     int            `json:"page"`
		PageSize int            `json:"pageSize"`
	}
	response := assertOrderEnvelope(t, rec, http.StatusOK)
	if err := json.Unmarshal(response.Data, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 1 || payload.Total != 12 || payload.Page != 2 || payload.PageSize != 5 {
		t.Fatalf("list payload = %#v", payload)
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

func TestOrderHandlerReturnsInternalServerErrorWhenAuditQueryFails(t *testing.T) {
	repo := &routingOrderRepository{
		detail:   domain.Order{ID: "o-detail", TenantID: 7, CorpID: 1536612155},
		auditErr: errors.New("audit unavailable"),
	}
	h := NewOrderHandler(repo, routingPrincipalResolver{})
	req := httptest.NewRequest(http.MethodGet, "/dashboard/scrm/orders/o-detail?corpId=1536612155", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %q, want 500", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "audit unavailable") {
		t.Fatalf("body = %q, must not expose repository error", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "internal server error") {
		t.Fatalf("body = %q, want stable audit error", rec.Body.String())
	}
}

func TestOrderHandlerCreatesAndListsScopedOrder(t *testing.T) {
	r := NewMemoryOrderRepository()
	h := NewOrderHandler(r, routingPrincipalResolver{corpID: 1})
	req := httptest.NewRequest("POST", "/scrm/orders", strings.NewReader(`{"id":"o1","tenantId":1,"corpId":1,"contactId":"c1","opportunityId":"opp1","title":"年度续费","note":"客户确认","amountCents":100,"status":"pending"}`))
	req.Header.Set("Idempotency-Key", "explicit-order")
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
	if len(r.List(7, 1)) != 1 {
		t.Fatal("not persisted")
	}
	autoReq := httptest.NewRequest("POST", "/scrm/orders", strings.NewReader(`{"tenantId":1,"corpId":1,"contactId":"c1","title":"auto id","amountCents":50,"status":"pending"}`))
	autoReq.Header.Set("Idempotency-Key", "auto-order")
	autoRec := httptest.NewRecorder()
	h.ServeHTTP(autoRec, autoReq)
	if autoRec.Code != 200 {
		t.Fatalf("auto id status %d body %q", autoRec.Code, autoRec.Body.String())
	}
	autoResponse := assertOrderEnvelope(t, autoRec, http.StatusOK)
	var autoCreated domain.Order
	if err := json.Unmarshal(autoResponse.Data, &autoCreated); err != nil {
		t.Fatalf("decode auto order: %v", err)
	}
	if autoCreated.ID == "" || strings.HasPrefix(autoCreated.ID, "P35-ORDER-") {
		t.Fatalf("auto id = %q, want server-side non-acceptance id", autoCreated.ID)
	}
	if len(r.List(7, 1)) != 2 {
		t.Fatal("auto order not persisted")
	}
	generated, err := domain.NewOrder(domain.NewOrderInput{TenantID: 1, CorpID: 1, ContactID: "c", Title: "auto id", AmountCents: 1, Status: domain.OrderPending})
	if err != nil {
		t.Fatalf("NewOrder without id failed: %v", err)
	}
	if generated.ID == "" || strings.HasPrefix(generated.ID, "P35-ORDER-") {
		t.Fatalf("generated id = %q, want server-side non-acceptance id", generated.ID)
	}
}

func TestOrderHandlerRequiresIdempotencyKeyForCreate(t *testing.T) {
	h := NewOrderHandler(NewMemoryOrderRepository(), routingPrincipalResolver{corpID: 1})
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/scrm/orders", strings.NewReader(`{"contactId":"c1","title":"renewal","amountCents":100,"status":"pending"}`)))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %q, want 422", rec.Code, rec.Body.String())
	}
}

func TestOrderHandlerReplaysExactFirstResponseAfterClientLosesIt(t *testing.T) {
	repo := NewMemoryOrderRepository()
	h := NewOrderHandler(repo, routingPrincipalResolver{corpID: 1})
	first := createOrderRequest("retry-key", `{"contactId":"c1","title":"renewal","amountCents":100,"status":"pending"}`)
	firstResponse := httptest.NewRecorder()
	h.ServeHTTP(firstResponse, first)

	retryResponse := httptest.NewRecorder()
	h.ServeHTTP(retryResponse, createOrderRequest("retry-key", `{"status":"pending","amountCents":100,"title":"renewal","contactId":"c1"}`))

	if firstResponse.Code != http.StatusOK || retryResponse.Code != firstResponse.Code {
		t.Fatalf("statuses = first:%d retry:%d", firstResponse.Code, retryResponse.Code)
	}
	if retryResponse.Body.String() != firstResponse.Body.String() {
		t.Fatalf("retry body = %q, want exact first body %q", retryResponse.Body.String(), firstResponse.Body.String())
	}
	if got := len(repo.List(7, 1)); got != 1 {
		t.Fatalf("orders = %d, want 1", got)
	}
}

func TestOrderHandlerRejectsSameKeyWithDifferentPayload(t *testing.T) {
	h := NewOrderHandler(NewMemoryOrderRepository(), routingPrincipalResolver{corpID: 1})
	first := httptest.NewRecorder()
	h.ServeHTTP(first, createOrderRequest("conflict-key", `{"contactId":"c1","title":"renewal","amountCents":100,"status":"pending"}`))
	conflict := httptest.NewRecorder()
	h.ServeHTTP(conflict, createOrderRequest("conflict-key", `{"contactId":"c1","title":"renewal","amountCents":200,"status":"pending"}`))

	if first.Code != http.StatusOK || conflict.Code != http.StatusConflict {
		t.Fatalf("statuses = first:%d conflict:%d, conflict body = %q", first.Code, conflict.Code, conflict.Body.String())
	}
}

func TestOrderHandlerTreatsClientOrderIDAsPartOfTheIdempotencyPayload(t *testing.T) {
	h := NewOrderHandler(NewMemoryOrderRepository(), routingPrincipalResolver{corpID: 1})
	first := httptest.NewRecorder()
	h.ServeHTTP(first, createOrderRequest("client-id-key", `{"id":"client-order-1","contactId":"c1","title":"renewal","amountCents":100,"status":"pending"}`))
	conflict := httptest.NewRecorder()
	h.ServeHTTP(conflict, createOrderRequest("client-id-key", `{"id":"client-order-2","contactId":"c1","title":"renewal","amountCents":100,"status":"pending"}`))

	if first.Code != http.StatusOK || conflict.Code != http.StatusConflict {
		t.Fatalf("statuses = first:%d conflict:%d, conflict body = %q", first.Code, conflict.Code, conflict.Body.String())
	}
}

func TestOrderHandlerSerializesThirtyTwoConcurrentCreatesWithSameKey(t *testing.T) {
	repo := NewMemoryOrderRepository()
	h := NewOrderHandler(repo, routingPrincipalResolver{corpID: 1})
	responses := make([]*httptest.ResponseRecorder, 32)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range responses {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			responses[index] = httptest.NewRecorder()
			h.ServeHTTP(responses[index], createOrderRequest("concurrent-key", `{"contactId":"c1","title":"renewal","amountCents":100,"status":"pending"}`))
		}(index)
	}
	close(start)
	wait.Wait()

	wantBody := responses[0].Body.String()
	for index, response := range responses {
		if response.Code != http.StatusOK || response.Body.String() != wantBody {
			t.Fatalf("response[%d] = status:%d body:%q, want status 200 body %q", index, response.Code, response.Body.String(), wantBody)
		}
	}
	if got := len(repo.List(7, 1)); got != 1 {
		t.Fatalf("orders = %d, want 1", got)
	}
}

func TestOrderHandlerScopesSameIdempotencyKeyByTenantAndCorp(t *testing.T) {
	repo := NewMemoryOrderRepository()
	for _, principal := range []Principal{
		{TenantID: 7, CorpID: 1, UserID: 11},
		{TenantID: 7, CorpID: 2, UserID: 11},
		{TenantID: 8, CorpID: 1, UserID: 11},
	} {
		h := NewOrderHandler(repo, fixedOrderPrincipalResolver{principal: principal})
		response := httptest.NewRecorder()
		h.ServeHTTP(response, createOrderRequest("shared-key", `{"contactId":"c1","title":"renewal","amountCents":100,"status":"pending"}`))
		if response.Code != http.StatusOK {
			t.Fatalf("scope %d/%d status = %d, body = %q", principal.TenantID, principal.CorpID, response.Code, response.Body.String())
		}
	}
	if len(repo.List(7, 1)) != 1 || len(repo.List(7, 2)) != 1 || len(repo.List(8, 1)) != 1 {
		t.Fatalf("scope counts = 7/1:%d 7/2:%d 8/1:%d", len(repo.List(7, 1)), len(repo.List(7, 2)), len(repo.List(8, 1)))
	}
}

func createOrderRequest(key, body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/scrm/orders", strings.NewReader(body))
	request.Header.Set("Idempotency-Key", key)
	return request
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

type routingPrincipalResolver struct{ corpID int64 }

func (r routingPrincipalResolver) Resolve(*http.Request) (Principal, error) {
	corpID := r.corpID
	if corpID == 0 {
		corpID = 1536612155
	}
	return Principal{TenantID: 7, UserID: 11, CorpID: corpID}, nil
}

type fixedOrderPrincipalResolver struct{ principal Principal }

func (r fixedOrderPrincipalResolver) Resolve(*http.Request) (Principal, error) {
	return r.principal, nil
}

type routingOrderRepository struct {
	listItems    []domain.Order
	listTotal    int
	detail       domain.Order
	audit        []map[string]any
	auditErr     error
	listCalls    int
	listPage     int
	listPageSize int
	getCalls     int
	auditCalls   int
	gotDetailID  string
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

func (r *routingOrderRepository) ListContext(_ context.Context, _ int64, _ int64, page, pageSize int) ([]domain.Order, int, error) {
	r.listCalls++
	r.listPage = page
	r.listPageSize = pageSize
	return r.listItems, r.listTotal, nil
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
	return r.audit, r.auditErr
}
