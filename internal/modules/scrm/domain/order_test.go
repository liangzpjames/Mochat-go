package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestNewOrderRejectsInvalidMoneyAndState(t *testing.T) {
	for _, input := range []NewOrderInput{{AmountCents: -1, Status: OrderPending}, {AmountCents: 1, Status: "unknown"}} {
		if _, err := NewOrder(input); err == nil {
			t.Fatalf("expected rejection for %#v", input)
		}
	}
}

func TestNewOrderRequiresInjectedID(t *testing.T) {
	_, err := NewOrder(NewOrderInput{TenantID: 1, CorpID: 2, ContactID: "c", Title: "order", AmountCents: 1, Status: OrderPending})
	if err == nil {
		t.Fatal("blank ID must be rejected by domain")
	}
}

func TestOrderCreateRequestHashRequiresExplicitClientOrderID(t *testing.T) {
	hashType := reflect.TypeOf(OrderCreateRequestHash)
	if hashType.IsVariadic() || hashType.NumIn() != 2 || hashType.In(1).Kind() != reflect.String {
		t.Fatalf("OrderCreateRequestHash type = %s, want non-variadic func(Order, string)", hashType)
	}
	order := Order{ContactID: "contact", Title: "renewal", AmountCents: 100, Currency: "CNY", Status: OrderPending}
	first, err := OrderCreateRequestHash(order, "client-order-1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := OrderCreateRequestHash(order, "client-order-2")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("explicit client order ID must affect the canonical payload hash")
	}
}

func TestOrderTransitionUsesOptimisticVersion(t *testing.T) {
	order, err := NewOrder(NewOrderInput{ID: "o1", TenantID: 1, CorpID: 2, ContactID: "c1", Title: "测试订单", AmountCents: 100, Currency: "CNY", Status: OrderPending})
	if err != nil {
		t.Fatal(err)
	}
	if err := order.Transition(OrderPaid, 2); err == nil {
		t.Fatal("expected version conflict")
	}
	if err := order.Transition(OrderPaid, 1); err != nil {
		t.Fatal(err)
	}
	if order.Version != 2 || order.Status != OrderPaid {
		t.Fatalf("order=%#v", order)
	}
}

func TestNewOrderRequiresTitleAndPreservesBusinessFields(t *testing.T) {
	order, err := NewOrder(NewOrderInput{ID: "o1", TenantID: 1, CorpID: 2, ContactID: "c1", OpportunityID: "opp1", Title: "年度续费", Note: "客户确认本周付款", AmountCents: 100, Currency: "cny", Status: OrderPending})
	if err != nil {
		t.Fatal(err)
	}
	if order.Title != "年度续费" || order.Note != "客户确认本周付款" || order.OpportunityID != "opp1" {
		t.Fatalf("order = %#v", order)
	}
	for _, input := range []NewOrderInput{
		{ID: "o2", TenantID: 1, CorpID: 2, ContactID: "c1", AmountCents: 100, Status: OrderPending},
		{ID: "o3", TenantID: 1, CorpID: 2, ContactID: "c1", Title: "title", Note: strings.Repeat("n", 2001), AmountCents: 100, Status: OrderPending},
	} {
		if _, err := NewOrder(input); err == nil {
			t.Fatalf("expected invalid order for %#v", input)
		}
	}
}
