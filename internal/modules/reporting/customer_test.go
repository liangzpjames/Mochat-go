package reporting

import "testing"

func TestAggregateCustomersDeduplicatesStableIDs(t *testing.T) {
	r := AggregateCustomers([]CustomerEvent{{ID: "a", Stage: "contact"}, {ID: "a", Stage: "contact"}, {ID: "b", Stage: "lead"}})
	if *r.Summary["contact"] != 1 || *r.Summary["lead"] != 1 {
		t.Fatalf("%v", r.Summary)
	}
}
