package domain

import (
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
