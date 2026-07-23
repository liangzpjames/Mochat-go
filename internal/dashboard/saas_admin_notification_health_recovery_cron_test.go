package dashboard

import (
	"bytes"
	"context"
	"log"
	"strconv"
	"strings"
	"testing"
)

func TestSaaSAdminNotificationHealthRecoveryCronClosesRecoveredAssignments(t *testing.T) {
	store := &fakeSaaSAdminStore{
		notificationHealthSource: SaaSAdminNotificationHealthSource{Tenants: []SaaSAdminNotificationHealthSnapshot{
			{TenantID: 12, TenantName: "已恢复租户", NotificationCount: 5, DeliveredCount: 5, TotalAttempts: 5},
			{TenantID: 13, TenantName: "仍异常租户", NotificationCount: 1, FailedCount: 1, TotalAttempts: 1},
			{TenantID: 14, TenantName: "无数据租户"},
			{TenantID: 15, TenantName: "只有抑制租户", NotificationCount: 1, SuppressedCount: 1},
		}},
	}
	for i, tenantID := range []int{12, 13, 14, 15} {
		assignment := SaaSAdminOperationQueueAssignment{
			TenantID: tenantID, Source: SaaSAdminOperationQueueSourceNotificationHealth, ObjectType: SaaSAdminOperationTargetNotificationHealth,
			ObjectID: strconv.Itoa(tenantID), TargetName: "通知健康异常", Owner: "Ops-Health", Status: SaaSAdminRiskFollowUpStatusContacted,
			NextFollowUpAt: "2026-07-20 09:00:00", Remark: "通知健康异常认领", OperationID: int64(i + 1), AssignedAt: "2026-07-10 12:00:00",
		}
		store.operationLogs = append(store.operationLogs, SaaSAdminOperationLog{
			ID: int64(i + 1), TenantID: tenantID, ActorUserID: 1, ActorTenantID: 1,
			Action: SaaSAdminOperationActionOperationQueueAssign, TargetType: SaaSAdminOperationTargetNotificationHealth,
			TargetID: assignment.ObjectID, TargetName: assignment.TargetName, AfterJSON: saasAdminPayloadJSON(saasAdminOperationQueueAssignmentPayload(assignment)), Remark: assignment.Remark,
		})
	}

	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	var output bytes.Buffer
	cron := NewSaaSAdminNotificationHealthRecoveryCron(handler, 48, 30, log.New(&output, "", 0))

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.lastNotificationHealthOptions.WindowHours != 48 || store.lastNotificationHealthOptions.StaleMinutes != 30 || store.lastNotificationHealthOptions.Limit != saasAdminExportMaxLimit {
		t.Fatalf("health options = %+v", store.lastNotificationHealthOptions)
	}
	if store.recordOperationLogCalls != 1 {
		t.Fatalf("operation calls = %d", store.recordOperationLogCalls)
	}
	operation := store.lastRecordedOperationLog
	if operation.Action != SaaSAdminOperationActionOperationQueueAssignmentClose ||
		operation.ActorUserID != 0 ||
		operation.ActorTenantID != 1 ||
		operation.TenantID != 12 ||
		operation.Remark != "自动通知健康恢复结案" ||
		!strings.Contains(operation.AfterJSON, `"reason":"notification_health_recovered"`) ||
		!strings.Contains(operation.AfterJSON, `"healthState":"healthy"`) {
		t.Fatalf("operation = %+v", operation)
	}
	if !strings.Contains(output.String(), "matched=4") ||
		!strings.Contains(output.String(), "recovered=1") ||
		!strings.Contains(output.String(), "closed=1") ||
		!strings.Contains(output.String(), "unhealthy=1") ||
		!strings.Contains(output.String(), "no_data=1") ||
		!strings.Contains(output.String(), "no_delivery_evidence=1") {
		t.Fatalf("log output = %q", output.String())
	}

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.recordOperationLogCalls != 1 {
		t.Fatalf("second run should be idempotent: operation calls = %d", store.recordOperationLogCalls)
	}
	if !strings.Contains(output.String(), "already_closed=1") {
		t.Fatalf("second run log output = %q", output.String())
	}
}

func TestSaaSAdminNotificationHealthRecoveryCronRequiresDependencies(t *testing.T) {
	err := NewSaaSAdminNotificationHealthRecoveryCron(nil, 0, 0, nil).RunOnce(context.Background())
	if err == nil || err.Error() != "SaaS notification health recovery cron dependencies are not configured" {
		t.Fatalf("RunOnce err = %v", err)
	}
}
