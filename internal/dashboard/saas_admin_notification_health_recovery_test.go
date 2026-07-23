package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestSaaSAdminBuildOperationQueueIncludesNotificationHealth(t *testing.T) {
	critical := SaaSAdminNotificationHealthTenant{
		SaaSAdminNotificationHealthSnapshot: SaaSAdminNotificationHealthSnapshot{
			TenantID: 12, TenantName: "严重租户", NotificationCount: 2, DeadCount: 1, LastFailureAt: "2026-07-10 12:00:00",
		},
		HealthState:     SaaSAdminNotificationHealthStateCritical,
		Reasons:         []string{"1 条通知已耗尽"},
		SuggestedAction: "检查 Webhook",
	}
	warning := SaaSAdminNotificationHealthTenant{
		SaaSAdminNotificationHealthSnapshot: SaaSAdminNotificationHealthSnapshot{
			TenantID: 13, TenantName: "预警租户", NotificationCount: 1, FailedCount: 1, LastFailureAt: "2026-07-10 11:00:00",
		},
		HealthState:     SaaSAdminNotificationHealthStateWarning,
		Reasons:         []string{"1 条通知等待重试"},
		SuggestedAction: "处理待重试通知",
	}
	healthy := SaaSAdminNotificationHealthTenant{
		SaaSAdminNotificationHealthSnapshot: SaaSAdminNotificationHealthSnapshot{TenantID: 14, TenantName: "健康租户", NotificationCount: 2, DeliveredCount: 2},
		HealthState:                         SaaSAdminNotificationHealthStateHealthy,
	}
	assignment := SaaSAdminOperationQueueAssignment{
		TenantID: 12, Source: SaaSAdminOperationQueueSourceNotificationHealth, ObjectType: SaaSAdminOperationTargetNotificationHealth,
		ObjectID: "12", Owner: "Ops-Health", Status: SaaSAdminRiskFollowUpStatusPending, OperationID: 9,
	}
	items, summary := saasAdminBuildOperationQueue(saasAdminOperationQueueBuildInput{
		NotificationHealth: []SaaSAdminNotificationHealthTenant{healthy, warning, critical},
		Assignments: map[string]SaaSAdminOperationQueueAssignment{
			saasAdminOperationQueueAssignmentKey(assignment.Source, assignment.ObjectID): assignment,
		},
		Options: SaaSAdminOperationQueueOptions{Limit: 10},
	})

	if len(items) != 2 || summary.QueueCount != 2 || summary.NotificationHealthCount != 2 || summary.CriticalCount != 1 || summary.HighCount != 1 {
		t.Fatalf("items=%+v summary=%+v", items, summary)
	}
	if items[0].TenantID != 12 || items[0].Priority != SaaSAdminCustomerSuccessPriorityCritical || items[0].Owner != "Ops-Health" || items[0].Assignment == nil {
		t.Fatalf("critical item = %+v", items[0])
	}
	if items[1].TenantID != 13 || items[1].Priority != SaaSAdminCustomerSuccessPriorityHigh || items[1].ObjectType != SaaSAdminOperationTargetNotificationHealth {
		t.Fatalf("warning item = %+v", items[1])
	}
}

func TestSaaSAdminOperationQueueAssignRecordsNotificationHealthOwner(t *testing.T) {
	store := newFakeSaaSAdminOperationQueueStore(t)
	store.notificationHealthSource = SaaSAdminNotificationHealthSource{Tenants: []SaaSAdminNotificationHealthSnapshot{{
		TenantID: 12, TenantName: "运营待办租户", NotificationCount: 1, DeadCount: 1, LastFailureAt: "2026-07-10 12:00:00",
	}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/operationQueueAssign?source=notification_health&healthWindowHours=48&healthStaleMinutes=30&limit=10", strings.NewReader(`{"owner":"Ops-Health","status":"contacted","remark":"通知健康异常认领"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.OperationQueueAssign(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["matchedCount"].(float64) != 1 || data["assignedCount"].(float64) != 1 || data["notificationHealthAssignedCount"].(float64) != 1 {
		t.Fatalf("data = %+v", data)
	}
	if store.lastNotificationHealthOptions.WindowHours != 48 || store.lastNotificationHealthOptions.StaleMinutes != 30 {
		t.Fatalf("health options = %+v", store.lastNotificationHealthOptions)
	}
	log := store.lastRecordedOperationLog
	if log.Action != SaaSAdminOperationActionOperationQueueAssign || log.TargetType != SaaSAdminOperationTargetNotificationHealth || log.TargetID != "12" || !strings.Contains(log.AfterJSON, `"source":"notification_health"`) || !strings.Contains(log.AfterJSON, `"owner":"Ops-Health"`) {
		t.Fatalf("operation log = %+v", log)
	}
}

func TestSaaSAdminNotificationHealthRecoveryClosesOnlyHealthyAssignments(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			2: {ID: 2, TenantID: 12, IsSuperAdmin: 1},
		},
		notificationHealthSource: SaaSAdminNotificationHealthSource{Tenants: []SaaSAdminNotificationHealthSnapshot{
			{TenantID: 12, TenantName: "已恢复租户", NotificationCount: 5, DeliveredCount: 5},
			{TenantID: 13, TenantName: "无数据租户"},
			{TenantID: 14, TenantName: "仍异常租户", NotificationCount: 1, FailedCount: 1},
			{TenantID: 15, TenantName: "只有抑制租户", NotificationCount: 1, SuppressedCount: 1},
		}},
	}
	for i, tenantID := range []int{12, 13, 14, 15} {
		assignment := SaaSAdminOperationQueueAssignment{
			TenantID: tenantID, Source: SaaSAdminOperationQueueSourceNotificationHealth, ObjectType: SaaSAdminOperationTargetNotificationHealth,
			ObjectID:   strconv.Itoa(tenantID),
			TargetName: "通知健康异常", Owner: "Ops-Health", Status: SaaSAdminRiskFollowUpStatusContacted,
			NextFollowUpAt: "2026-07-20 09:00:00", Remark: "通知健康异常认领", OperationID: int64(i + 1), AssignedAt: "2026-07-10 12:00:00",
		}
		store.operationLogs = append(store.operationLogs, SaaSAdminOperationLog{
			ID: int64(i + 1), TenantID: tenantID, ActorUserID: 1, ActorTenantID: 1,
			Action: SaaSAdminOperationActionOperationQueueAssign, TargetType: SaaSAdminOperationTargetNotificationHealth,
			TargetID: assignment.ObjectID, TargetName: assignment.TargetName, AfterJSON: saasAdminPayloadJSON(saasAdminOperationQueueAssignmentPayload(assignment)), Remark: assignment.Remark,
		})
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationHealthRecovery?windowHours=24&staleMinutes=15", strings.NewReader(`{"remark":"健康恢复自动结案"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.NotificationHealthRecovery(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["matchedAssignmentCount"].(float64) != 4 || data["activeAssignmentCount"].(float64) != 4 || data["recoveredTenantCount"].(float64) != 1 || data["closedCount"].(float64) != 1 || data["unhealthyCount"].(float64) != 1 || data["noDataCount"].(float64) != 1 || data["noDeliveryEvidenceCount"].(float64) != 1 {
		t.Fatalf("data = %+v", data)
	}
	if store.recordOperationLogCalls != 1 {
		t.Fatalf("operation calls = %d", store.recordOperationLogCalls)
	}
	log := store.lastRecordedOperationLog
	if log.Action != SaaSAdminOperationActionOperationQueueAssignmentClose || log.TenantID != 12 || log.TargetType != SaaSAdminOperationTargetNotificationHealth || !strings.Contains(log.AfterJSON, `"reason":"notification_health_recovered"`) || !strings.Contains(log.AfterJSON, `"healthState":"healthy"`) {
		t.Fatalf("recovery log = %+v", log)
	}

	repeatReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationHealthRecovery", nil)
	repeatReq.Header.Set("X-Mochat-Go-User-ID", "1")
	repeatRec := httptest.NewRecorder()
	handler.NotificationHealthRecovery(repeatRec, repeatReq)
	if repeatRec.Code != http.StatusOK {
		t.Fatalf("repeat status=%d body=%s", repeatRec.Code, repeatRec.Body.String())
	}
	repeat := decodeSaaSAdminResponse(t, repeatRec)
	if repeat["closedCount"].(float64) != 0 || repeat["alreadyClosedCount"].(float64) != 1 || store.recordOperationLogCalls != 1 {
		t.Fatalf("repeat=%+v calls=%d", repeat, store.recordOperationLogCalls)
	}

	forbiddenReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationHealthRecovery", nil)
	forbiddenReq.Header.Set("X-Mochat-Go-User-ID", "2")
	forbiddenRec := httptest.NewRecorder()
	handler.NotificationHealthRecovery(forbiddenRec, forbiddenReq)
	if forbiddenRec.Code != http.StatusForbidden {
		t.Fatalf("forbidden status=%d body=%s", forbiddenRec.Code, forbiddenRec.Body.String())
	}
}
