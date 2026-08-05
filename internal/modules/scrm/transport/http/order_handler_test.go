package http

import (
	"jiyi/mochat-go/internal/modules/scrm/domain"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOrderHandlerCreatesAndListsScopedOrder(t *testing.T) {
	r := NewMemoryOrderRepository()
	h := NewOrderHandler(r)
	req := httptest.NewRequest("POST", "/scrm/orders", strings.NewReader(`{"id":"o1","tenantId":1,"corpId":1,"contactId":"c1","amountCents":100,"status":"pending"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	if len(r.List(1, 1)) != 1 {
		t.Fatal("not persisted")
	}
	if _, e := domain.NewOrder(domain.NewOrderInput{ID: "", TenantID: 1, CorpID: 1, ContactID: "c", Status: domain.OrderPending}); e == nil {
		t.Fatal("expected invalid")
	}
}
