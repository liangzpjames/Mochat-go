package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

// Room batch (阶段 B) real-MariaDB integration. These reuse the contact batch
// harness schema/fixture (which now also seeds mc_work_room + room batch
// tables) and drive the production MySQLStore durable room create, due, read,
// cancel and ledger paths. Never contacts a real WeCom endpoint.

func roomBatchHarnessInput(h *contactBatchIntegrationHarness, idempotencyKey string, sendWay int, definiteTime string, employeeIDs []int, targets []dashboard.RoomBatchTarget) dashboard.RoomBatchDispatchInput {
	if len(employeeIDs) == 0 {
		employeeIDs = []int{101}
	}
	if len(targets) == 0 {
		targets = []dashboard.RoomBatchTarget{{OwnerEmployeeID: 101, RoomID: 601}, {OwnerEmployeeID: 101, RoomID: 602}}
	}
	return dashboard.RoomBatchDispatchInput{
		Batch: dashboard.RoomMessageBatchSendWrite{
			CorpID: h.principal.CorpID, UserID: h.principal.UserID, UserName: "admin",
			EmployeeIDs: employeeIDs, BatchTitle: "群聊触达",
			Content:     []dashboard.ContactMessageBatchSendContent{{MsgType: "text", Content: "hello"}},
			ContentJSON: `[{"msgType":"text","content":"hello"}]`,
			SendWay:     sendWay, DefiniteTime: definiteTime,
		},
		RoomTargets:           targets,
		SenderOwnerEmployeeID: targets[0].OwnerEmployeeID,
		IdempotencyKey:        idempotencyKey,
		RequestID:             idempotencyKey,
	}
}

func roomBatchDispatchIDs(t *testing.T, h *contactBatchIntegrationHarness, operationID int64) []int64 {
	t.Helper()
	rows, err := h.db.Query(`SELECT id FROM mochat_go_wecom_capability_dispatches WHERE tenant_id=? AND corp_id=? AND operation_id=? ORDER BY id ASC`, h.principal.TenantID, h.principal.CorpID, operationID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return ids
}

func driveRoomBatchDispatchToTerminal(t *testing.T, h *contactBatchIntegrationHarness, dispatchID int64, terminal, targetID, errorCode string) {
	t.Helper()
	principal := contactBatchDispatchPrincipal(h)
	claimed, err := h.store.ClaimDispatch(context.Background(), wecomcapability.DispatchClaimRequest{Principal: principal, DispatchID: dispatchID, ExpectedCredentialVersion: 1, LeaseDuration: 5 * time.Minute})
	if err != nil {
		t.Fatalf("claim dispatch %d: %v", dispatchID, err)
	}
	transition := func(status string) {
		_, err := h.store.TransitionDispatch(context.Background(), wecomcapability.DispatchTransitionRequest{
			Principal: principal, DispatchID: dispatchID, Status: status, LeaseToken: claimed.LeaseToken, Attempt: claimed.Attempt,
			ProviderMessageID: fmt.Sprintf("room-msg-%d", dispatchID), LastErrorCode: errorCode,
		})
		if err != nil {
			t.Fatalf("transition %s dispatch %d: %v", status, dispatchID, err)
		}
	}
	transition(wecomcapability.DispatchSubmitting)
	transition(wecomcapability.DispatchSubmitted)
	transition(wecomcapability.DispatchPolling)
	if _, err := h.store.RecordDispatchResult(context.Background(), wecomcapability.DispatchResultRequest{
		Principal: principal, DispatchID: dispatchID, LeaseToken: claimed.LeaseToken, Attempt: claimed.Attempt,
		TargetKind: "employee_room_chat_id", TargetID: targetID, Status: terminal, ProviderTargetID: "room-1", ErrorCode: errorCode,
	}); err != nil {
		t.Fatalf("record result dispatch %d: %v", dispatchID, err)
	}
	transition(terminal)
}

// TestRoomBatchCreateCommitsBusinessOperationDispatchAuditEventAtomically
// locks the durable room create: business row + room_batch_send operation +
// dispatch chunks + create audit/event commit in one transaction, with room
// targets resolved to chat ids and ownership verified inside the tx.
func TestRoomBatchCreateCommitsBusinessOperationDispatchAuditEventAtomically(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	result, err := h.store.CreateRoomBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), roomBatchHarnessInput(h, "room-atomic-1", 1, "", []int{101, 102}, []dashboard.RoomBatchTarget{{OwnerEmployeeID: 101, RoomID: 601}, {OwnerEmployeeID: 101, RoomID: 602}, {OwnerEmployeeID: 102, RoomID: 603}}))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.BatchID <= 0 || result.OperationID <= 0 || result.Duplicate {
		t.Fatalf("result=%#v", result)
	}
	var requestID, capability string
	if err := h.db.QueryRow(`SELECT request_id, capability FROM mochat_go_wecom_capability_operations WHERE id=?`, result.OperationID).Scan(&requestID, &capability); err != nil {
		t.Fatal(err)
	}
	if capability != string(wecomcapability.RoomBatchSend) || requestID != fmt.Sprintf("room-batch:%d", result.BatchID) {
		t.Fatalf("operation request_id=%q capability=%q", requestID, capability)
	}
	ids := roomBatchDispatchIDs(t, h, result.OperationID)
	if len(ids) != 2 {
		t.Fatalf("dispatch count=%d, want 2 (one per owner)", len(ids))
	}
	for _, id := range ids {
		var kind string
		if err := h.db.QueryRow(`SELECT dispatch_kind FROM mochat_go_wecom_capability_dispatches WHERE id=?`, id).Scan(&kind); err != nil {
			t.Fatal(err)
		}
		if kind != string(wecomcapability.DispatchKindRoomBatch) {
			t.Fatalf("dispatch %d kind=%q, want room_batch", id, kind)
		}
	}
	var roomResults, employeeRows int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM mc_room_message_batch_send_result WHERE batch_id=?`, result.BatchID).Scan(&roomResults); err != nil {
		t.Fatal(err)
	}
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM mc_room_message_batch_send_employee WHERE batch_id=?`, result.BatchID).Scan(&employeeRows); err != nil {
		t.Fatal(err)
	}
	if roomResults != 3 || employeeRows != 2 {
		t.Fatalf("room results=%d employee rows=%d, want 3 and 2", roomResults, employeeRows)
	}
	var batchTitle string
	if err := h.db.QueryRow(`SELECT batch_title FROM mc_room_message_batch_send WHERE id=?`, result.BatchID).Scan(&batchTitle); err != nil {
		t.Fatal(err)
	}
	if batchTitle != "群聊触达" {
		t.Fatalf("batch_title=%q", batchTitle)
	}
}

// TestRoomBatchDurableIdempotencyReplaysWithoutDuplicateWrites locks room
// idempotency replay semantics.
func TestRoomBatchDurableIdempotencyReplaysWithoutDuplicateWrites(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	first, err := h.store.CreateRoomBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), roomBatchHarnessInput(h, "room-idem-1", 1, "", nil, nil))
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	op1, dispatch1, audit1, event1 := 0, 0, 0, 0
	_ = h.db.QueryRow(`SELECT (SELECT COUNT(*) FROM mochat_go_wecom_capability_operations), (SELECT COUNT(*) FROM mochat_go_wecom_capability_dispatches), (SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_audits), (SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_events)`).Scan(&op1, &dispatch1, &audit1, &event1)
	second, err := h.store.CreateRoomBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), roomBatchHarnessInput(h, "room-idem-1", 1, "", nil, nil))
	if err != nil {
		t.Fatalf("duplicate create: %v", err)
	}
	if !second.Duplicate || second.BatchID != first.BatchID || second.OperationID != first.OperationID {
		t.Fatalf("duplicate replay first=%#v second=%#v", first, second)
	}
	op2, dispatch2, audit2, event2 := 0, 0, 0, 0
	_ = h.db.QueryRow(`SELECT (SELECT COUNT(*) FROM mochat_go_wecom_capability_operations), (SELECT COUNT(*) FROM mochat_go_wecom_capability_dispatches), (SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_audits), (SELECT COUNT(*) FROM mochat_go_wecom_capability_operation_events)`).Scan(&op2, &dispatch2, &audit2, &event2)
	if op1 != op2 || dispatch1 != dispatch2 || audit1 != audit2 || event1 != event2 {
		t.Fatalf("duplicate wrote rows before=(%d,%d,%d,%d) after=(%d,%d,%d,%d)", op1, dispatch1, audit1, event1, op2, dispatch2, audit2, event2)
	}
}

// TestRoomBatchDurableZeroWriteOnIsolationViolations locks cross-tenant and
// room-ownership violations fail closed with zero writes.
func TestRoomBatchDurableZeroWriteOnIsolationViolations(t *testing.T) {
	t.Run("cross tenant", func(t *testing.T) {
		h := newContactBatchIntegrationHarness(t)
		other := dashboardprincipal.DashboardPrincipal{TenantID: 22, CorpID: 2201, UserID: 22001, AuthVersion: 1, IsSuperAdmin: true}
		access := dashboard.DashboardAccessContext{UserID: 22001, TenantID: 22, CorpID: 2201, IsSuperAdmin: true, Scope: dashboard.DataScopeTenant}
		input := roomBatchHarnessInput(h, "room-iso-cross", 1, "", nil, nil)
		input.Batch.CorpID, input.Batch.UserID = 2201, 22001
		_, err := h.store.CreateRoomBatchDispatch(context.Background(), other, access, input)
		if err == nil || !errors.Is(err, dashboard.ErrContactBatchTenantDenied) {
			t.Fatalf("cross tenant err=%v", err)
		}
		var batch, operation, dispatch int
		_ = h.db.QueryRow(`SELECT (SELECT COUNT(*) FROM mc_room_message_batch_send), (SELECT COUNT(*) FROM mochat_go_wecom_capability_operations), (SELECT COUNT(*) FROM mochat_go_wecom_capability_dispatches)`).Scan(&batch, &operation, &dispatch)
		if batch != 0 || operation != 0 || dispatch != 0 {
			t.Fatalf("cross tenant wrote rows batch=%d operation=%d dispatch=%d", batch, operation, dispatch)
		}
	})
	t.Run("room not owned by owner", func(t *testing.T) {
		h := newContactBatchIntegrationHarness(t)
		if _, err := h.db.Exec(`UPDATE mc_work_room SET owner_id=102 WHERE id=601`); err != nil {
			t.Fatal(err)
		}
		_, err := h.store.CreateRoomBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), roomBatchHarnessInput(h, "room-iso-owner", 1, "", nil, nil))
		if err == nil || !errors.Is(err, dashboard.ErrContactBatchTargetNotOwned) {
			t.Fatalf("ownership change err=%v", err)
		}
		var batch, operation, dispatch int
		_ = h.db.QueryRow(`SELECT (SELECT COUNT(*) FROM mc_room_message_batch_send), (SELECT COUNT(*) FROM mochat_go_wecom_capability_operations), (SELECT COUNT(*) FROM mochat_go_wecom_capability_dispatches)`).Scan(&batch, &operation, &dispatch)
		if batch != 0 || operation != 0 || dispatch != 0 {
			t.Fatalf("ownership change wrote rows batch=%d operation=%d dispatch=%d", batch, operation, dispatch)
		}
	})
	t.Run("revoked permission", func(t *testing.T) {
		h := newContactBatchIntegrationHarness(t)
		principal := dashboardprincipal.DashboardPrincipal{TenantID: 11, CorpID: 1101, UserID: 11002, AuthVersion: 1}
		if _, err := h.db.Exec(`DELETE FROM mochat_go_dashboard_user_permissions WHERE tenant_id=11 AND user_id=11002`); err != nil {
			t.Fatal(err)
		}
		input := roomBatchHarnessInput(h, "room-iso-revoked", 1, "", nil, nil)
		input.Batch.UserID = 11002
		_, err := h.store.CreateRoomBatchDispatch(context.Background(), principal, contactBatchHarnessAccess(11002, false), input)
		if err == nil || !errors.Is(err, dashboard.ErrContactBatchPermissionDenied) {
			t.Fatalf("revoked permission err=%v", err)
		}
		var batch int
		_ = h.db.QueryRow(`SELECT COUNT(*) FROM mc_room_message_batch_send`).Scan(&batch)
		if batch != 0 {
			t.Fatalf("revoked permission wrote rows batch=%d", batch)
		}
	})
}

// TestRoomBatchLegacyCronExcludesDurableRoomBatches locks the legacy room cron
// never claims a durable room batch.
func TestRoomBatchLegacyCronExcludesDurableRoomBatches(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	if _, err := h.store.CreateRoomBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), roomBatchHarnessInput(h, "room-legacy-excl", 2, "2000-01-01 00:00:00", nil, nil)); err != nil {
		t.Fatalf("create durable: %v", err)
	}
	legacySeed := `INSERT INTO mc_room_message_batch_send (tenant_id,corp_id,user_id,user_name,employee_ids,content,send_way,send_status,definite_time,batch_title,created_at,updated_at) VALUES (11,1101,11001,'legacy','[101]','[{"msgType":"text","content":"legacy"}]',2,0,NOW() - INTERVAL 1 HOUR,'legacy room batch',NOW(),NOW())`
	if _, err := h.db.Exec(legacySeed); err != nil {
		t.Fatal(err)
	}
	ids, err := h.store.DueRoomMessageBatchSendIDs(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatalf("legacy due ids=%v, want only the legacy room batch", ids)
	}
	for _, id := range ids {
		var title string
		if err := h.db.QueryRow(`SELECT batch_title FROM mc_room_message_batch_send WHERE id=?`, id).Scan(&title); err != nil {
			t.Fatal(err)
		}
		if title != "legacy room batch" {
			t.Fatalf("durable room batch %d leaked into legacy cron (title=%q)", id, title)
		}
	}
}

// TestRoomBatchDurableCancelFencedByWorkerClaim locks cancel vs worker claim
// fencing for room dispatches.
func TestRoomBatchDurableCancelFencedByWorkerClaim(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	result, err := h.store.CreateRoomBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), roomBatchHarnessInput(h, "room-cancel-fence", 1, "", nil, nil))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ids := roomBatchDispatchIDs(t, h, result.OperationID)
	if len(ids) != 1 {
		t.Fatalf("dispatch count=%d", len(ids))
	}
	if _, err := h.store.ClaimDispatch(context.Background(), wecomcapability.DispatchClaimRequest{Principal: contactBatchDispatchPrincipal(h), DispatchID: ids[0], ExpectedCredentialVersion: 1, LeaseDuration: 5 * time.Minute}); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := h.store.CancelRoomBatchDurable(context.Background(), h.principal, int(result.BatchID)); err == nil || !errors.Is(err, dashboard.ErrContactBatchConflict) {
		t.Fatalf("cancel after claim err=%v", err)
	}
	var status string
	if err := h.db.QueryRow(`SELECT status FROM mochat_go_wecom_capability_dispatches WHERE id=?`, ids[0]).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != wecomcapability.DispatchClaimed {
		t.Fatalf("dispatch status=%s, want claimed", status)
	}
}

// TestRoomBatchDurableAggregationPartialSuccessFailure locks the room durable
// view aggregate across success and failure dispatches.
func TestRoomBatchDurableAggregationPartialSuccessFailure(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	result, err := h.store.CreateRoomBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), roomBatchHarnessInput(h, "room-agg-1", 1, "", []int{101, 102}, []dashboard.RoomBatchTarget{{OwnerEmployeeID: 101, RoomID: 601}, {OwnerEmployeeID: 102, RoomID: 603}}))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ids := roomBatchDispatchIDs(t, h, result.OperationID)
	if len(ids) != 2 {
		t.Fatalf("dispatch count=%d, want 2", len(ids))
	}
	driveRoomBatchDispatchToTerminal(t, h, ids[0], wecomcapability.DispatchSucceeded, "101:chat-601", "")
	driveRoomBatchDispatchToTerminal(t, h, ids[1], wecomcapability.DispatchFailed, "102:chat-603", "wecom.http_401")
	view, found, err := h.store.RoomBatchDurableView(context.Background(), h.principal, int(result.BatchID))
	if err != nil || !found {
		t.Fatalf("durable view found=%v err=%v", found, err)
	}
	if view.Operation.Status != wecomcapability.OperationPartialFailed {
		t.Fatalf("aggregate status=%s, want partial_failed", view.Operation.Status)
	}
	got := map[string]string{}
	for _, item := range view.Results {
		got[item.TargetID] = item.Status
	}
	if got["101:chat-601"] != wecomcapability.DispatchSucceeded || got["102:chat-603"] != wecomcapability.DispatchFailed {
		t.Fatalf("result map=%#v", got)
	}
}

// TestRoomBatchDurableOwnerAndReceivePagesProject locks the durable owner and
// room-receive projections used by the dashboard pages.
func TestRoomBatchDurableOwnerAndReceivePagesProject(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	result, err := h.store.CreateRoomBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), roomBatchHarnessInput(h, "room-pages-1", 1, "", []int{101}, []dashboard.RoomBatchTarget{{OwnerEmployeeID: 101, RoomID: 601}, {OwnerEmployeeID: 101, RoomID: 602}}))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ownerPage, err := h.store.RoomBatchDurableOwnerPage(context.Background(), h.principal, int(result.BatchID), dashboard.RoomMessageBatchSendOwnerFilter{BatchID: int(result.BatchID), Page: 1, PerPage: 15})
	if err != nil {
		t.Fatalf("owner page: %v", err)
	}
	if len(ownerPage.Items) != 1 || ownerPage.Items[0].EmployeeID != 101 || ownerPage.Items[0].SendRoomTotal != 2 {
		t.Fatalf("owner page=%#v", ownerPage.Items)
	}
	roomPage, err := h.store.RoomBatchDurableReceivePage(context.Background(), h.principal, int(result.BatchID), dashboard.RoomMessageBatchSendRoomFilter{BatchID: int(result.BatchID), Page: 1, PerPage: 15})
	if err != nil {
		t.Fatalf("receive page: %v", err)
	}
	if len(roomPage.Items) != 2 {
		t.Fatalf("receive page items=%#v, want 2", roomPage.Items)
	}
	roomIDs := map[int]bool{}
	for _, item := range roomPage.Items {
		roomIDs[item.RoomID] = true
	}
	if !roomIDs[601] || !roomIDs[602] {
		t.Fatalf("receive page room ids=%#v", roomPage.Items)
	}
}
