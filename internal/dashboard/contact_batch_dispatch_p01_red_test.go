package dashboard

import "testing"

// TestContactBatchDispatchInputPreservesBatchTitleAndMediumID is the RED
// contract for P0-1: the durable create body carries batchTitle and mediumId,
// and the store input must not silently drop them (the legacy business row,
// list and show projections must round-trip them).
func TestContactBatchDispatchInputPreservesBatchTitleAndMediumID(t *testing.T) {
	h := &ContactMessageBatchSendHandler{}
	body := contactBatchDispatchBody{
		BatchTitle:     "夏日客户触达",
		EmployeeIDs:    []int{11},
		ContactTargets: []ContactBatchTarget{{EmployeeID: 11, ContactID: 21}},
		Content:        []ContactMessageBatchSendContent{{MsgType: "text", Content: "hello"}},
		IdempotencyKey: "contact-701",
		MediumID:       45,
		SendWay:        1,
	}
	input, err := h.contactBatchDispatchInputFromBody(body, User{ID: 1, Name: "actor"}, 7, DashboardAccessContext{Scope: DataScopeTenant})
	if err != nil {
		t.Fatalf("valid durable body rejected: %v", err)
	}
	if input.Batch.BatchTitle != "夏日客户触达" {
		t.Fatalf("batch title lost in durable input: %q", input.Batch.BatchTitle)
	}
	if input.Batch.MediumID != 45 {
		t.Fatalf("medium id lost in durable input: %d", input.Batch.MediumID)
	}
}
