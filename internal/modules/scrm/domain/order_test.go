package domain

import "testing"

func TestNewOrderRejectsInvalidMoneyAndState(t *testing.T) {
	for _, input := range []NewOrderInput{{AmountCents: -1, Status: OrderPending}, {AmountCents: 1, Status: "unknown"}} {
		if _, err := NewOrder(input); err == nil {
			t.Fatalf("expected rejection for %#v", input)
		}
	}
}

func TestOrderTransitionUsesOptimisticVersion(t *testing.T) {
	order, err := NewOrder(NewOrderInput{ID: "o1", TenantID: 1, CorpID: 2, ContactID: "c1", AmountCents: 100, Currency: "CNY", Status: OrderPending})
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
