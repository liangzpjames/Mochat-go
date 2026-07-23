package dashboard

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"
)

func TestSaaSAdminOperationQueueAssignmentReminderCronEnqueuesLatestDueAssignments(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	assignmentLog := func(id int64, objectID string, owner string, nextFollowUpAt string, status string) SaaSAdminOperationLog {
		return SaaSAdminOperationLog{
			ID:            id,
			TenantID:      12,
			ActorUserID:   1,
			ActorTenantID: 1,
			Action:        SaaSAdminOperationActionOperationQueueAssign,
			TargetType:    SaaSAdminOperationTargetAdminTask,
			TargetID:      objectID,
			TargetName:    "运营任务 SLA #" + objectID,
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"tenantId":       12,
				"source":         SaaSAdminOperationQueueSourceTaskSLA,
				"objectType":     SaaSAdminOperationTargetAdminTask,
				"objectId":       objectID,
				"targetName":     "运营任务 SLA #" + objectID,
				"owner":          owner,
				"status":         status,
				"nextFollowUpAt": nextFollowUpAt,
				"remark":         "cron assignment " + objectID,
			}),
			CreatedAt: now.Add(time.Duration(id) * time.Minute).Format("2006-01-02 15:04:05"),
		}
	}
	overdue := assignmentLog(1, "7001", "Ops-Overdue", now.AddDate(0, 0, -2).Format("2006-01-02 09:00:00"), SaaSAdminRiskFollowUpStatusPending)
	dueSoon := assignmentLog(2, "7002", "Ops-DueSoon", now.AddDate(0, 0, 2).Format("2006-01-02 09:00:00"), SaaSAdminRiskFollowUpStatusContacted)
	oldOverdue := assignmentLog(3, "7003", "Ops-Old", now.AddDate(0, 0, -3).Format("2006-01-02 09:00:00"), SaaSAdminRiskFollowUpStatusPending)
	latestFuture := assignmentLog(4, "7003", "Ops-New", now.AddDate(0, 0, 30).Format("2006-01-02 09:00:00"), SaaSAdminRiskFollowUpStatusContacted)
	beforeClose := assignmentLog(5, "7004", "Ops-Closed", now.AddDate(0, 0, -4).Format("2006-01-02 09:00:00"), SaaSAdminRiskFollowUpStatusPending)
	closed := assignmentLog(6, "7004", "Ops-Closed", "", SaaSAdminRiskFollowUpStatusResolved)
	closed.Action = SaaSAdminOperationActionOperationQueueAssignmentClose

	dueSoonAssignment, ok := saasAdminOperationQueueAssignmentFromLog(dueSoon)
	if !ok {
		t.Fatal("due soon assignment should parse")
	}
	dueSoonAssignment.DueState = SaaSAdminRiskFollowUpDueStateDueSoon
	dueSoonNotify := SaaSAdminOperationQueueAssignmentNotifications{
		Options:     SaaSAdminOperationQueueAssignmentOptions{DueState: SaaSAdminRiskFollowUpDueStateDueSoon, CurrentOnly: true},
		Channel:     SaaSAlertNotificationChannelWebhook,
		MaxAttempts: 7,
		Remark:      "existing due soon reminder",
	}
	dueSoonAlert, dueSoonKey, err := saasAdminOperationQueueAssignmentNotificationAlert(dueSoonAssignment, dueSoonNotify, 1)
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeSaaSAdminStore{
		operationLogs: []SaaSAdminOperationLog{overdue, dueSoon, oldOverdue, latestFuture, beforeClose, closed},
		notifications: []SaaSAlertNotification{{
			ID:              9001,
			NotificationKey: dueSoonKey,
			AlertKey:        dueSoonAlert.AlertType,
			TenantID:        12,
			Channel:         SaaSAlertNotificationChannelWebhook,
			Status:          SaaSAlertNotificationStatusPending,
			Alert:           dueSoonAlert,
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	var output bytes.Buffer
	cron := NewSaaSAdminOperationQueueAssignmentReminderCron(handler, 100, 7, log.New(&output, "", 0))

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.notificationEnqueueCalls != 1 || store.recordOperationLogCalls != 1 {
		t.Fatalf("writes notification=%d operation=%d", store.notificationEnqueueCalls, store.recordOperationLogCalls)
	}
	if store.lastEnqueuedAlert.Status.Metric != SaaSEventMetricOperationQueueAssignment ||
		store.lastEnqueuedAlert.AlertType != SaaSAlertTypeOperationQueueAssign ||
		store.lastEnqueuedAlert.Context["operationId"] != int64(1) ||
		store.lastEnqueuedAlert.Context["owner"] != "Ops-Overdue" ||
		store.lastEnqueuedMaxAttempts != 7 {
		t.Fatalf("enqueued alert = %+v maxAttempts=%d", store.lastEnqueuedAlert, store.lastEnqueuedMaxAttempts)
	}
	operation := store.lastRecordedOperationLog
	if operation.Action != SaaSAdminOperationActionOperationQueueAssignmentNotify ||
		operation.ActorUserID != 0 ||
		operation.ActorTenantID != 1 ||
		operation.Remark != "自动运营待办认领到期提醒" ||
		!strings.Contains(operation.AfterJSON, `"currentOnly":true`) {
		t.Fatalf("operation = %+v", operation)
	}
	if !strings.Contains(output.String(), "due_states=2") ||
		!strings.Contains(output.String(), "matched=2") ||
		!strings.Contains(output.String(), "enqueued=1") ||
		!strings.Contains(output.String(), "skipped_existing=1") {
		t.Fatalf("log output = %q", output.String())
	}

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.notificationEnqueueCalls != 1 || store.recordOperationLogCalls != 1 {
		t.Fatalf("second run should be idempotent: notification=%d operation=%d", store.notificationEnqueueCalls, store.recordOperationLogCalls)
	}
}

func TestSaaSAdminOperationQueueAssignmentReminderCronRequiresDependencies(t *testing.T) {
	err := NewSaaSAdminOperationQueueAssignmentReminderCron(nil, 0, 0, nil).RunOnce(context.Background())
	if err == nil || err.Error() != "SaaS operation queue assignment reminder cron dependencies are not configured" {
		t.Fatalf("RunOnce err = %v", err)
	}
}

func TestOperationQueueAssignmentReminderUsesExactNotificationKeyFallback(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	logItem := SaaSAdminOperationLog{
		ID:            11,
		TenantID:      12,
		ActorUserID:   1,
		ActorTenantID: 1,
		Action:        SaaSAdminOperationActionOperationQueueAssign,
		TargetType:    SaaSAdminOperationTargetAdminTask,
		TargetID:      "7011",
		TargetName:    "精确通知 key 兜底",
		AfterJSON: saasAdminPayloadJSON(map[string]any{
			"tenantId":       12,
			"source":         SaaSAdminOperationQueueSourceTaskSLA,
			"objectType":     SaaSAdminOperationTargetAdminTask,
			"objectId":       "7011",
			"targetName":     "精确通知 key 兜底",
			"owner":          "Ops-Exact",
			"status":         SaaSAdminRiskFollowUpStatusPending,
			"nextFollowUpAt": now.AddDate(0, 0, -2).Format("2006-01-02 09:00:00"),
			"remark":         "exact-key-fallback",
		}),
		CreatedAt: now.Add(-time.Hour).Format("2006-01-02 15:04:05"),
	}
	assignment, ok := saasAdminOperationQueueAssignmentFromLog(logItem)
	if !ok {
		t.Fatal("assignment should parse")
	}
	assignment.DueState = SaaSAdminRiskFollowUpDueStateOverdue
	notify := SaaSAdminOperationQueueAssignmentNotifications{
		Options: SaaSAdminOperationQueueAssignmentOptions{
			DueState:    SaaSAdminRiskFollowUpDueStateOverdue,
			Limit:       20,
			CurrentOnly: true,
		},
		Channel:       SaaSAlertNotificationChannelWebhook,
		MaxAttempts:   3,
		Remark:        "exact key fallback",
		ActorTenantID: 1,
	}
	alert, notificationKey, err := saasAdminOperationQueueAssignmentNotificationAlert(assignment, notify, 1)
	if err != nil {
		t.Fatal(err)
	}
	base := &fakeSaaSAdminStore{operationLogs: []SaaSAdminOperationLog{logItem}}
	store := &truncatedSaaSAdminNotificationStore{
		fakeSaaSAdminStore: base,
		hidden: SaaSAlertNotification{
			ID:              9901,
			NotificationKey: notificationKey,
			TenantID:        12,
			Channel:         SaaSAlertNotificationChannelWebhook,
			Status:          SaaSAlertNotificationStatusDelivered,
			Alert:           alert,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	result, err := handler.createOperationQueueAssignmentNotifications(context.Background(), notify)
	if err != nil {
		t.Fatal(err)
	}
	if result.MatchedCount != 1 || result.EligibleCount != 1 || result.SkippedExistingCount != 1 || result.EnqueuedCount != 0 {
		t.Fatalf("result = %+v", result)
	}
	if store.listCalls != 1 || store.keyCalls != 1 {
		t.Fatalf("reads list=%d key=%d", store.listCalls, store.keyCalls)
	}
	if base.notificationEnqueueCalls != 0 || base.recordOperationLogCalls != 0 {
		t.Fatalf("writes notification=%d operation=%d", base.notificationEnqueueCalls, base.recordOperationLogCalls)
	}
}

type truncatedSaaSAdminNotificationStore struct {
	*fakeSaaSAdminStore
	hidden    SaaSAlertNotification
	listCalls int
	keyCalls  int
}

func (s *truncatedSaaSAdminNotificationStore) SaaSAdminAlertNotifications(context.Context, SaaSAdminAlertNotificationOptions) ([]SaaSAlertNotification, error) {
	s.listCalls++
	return nil, nil
}

func (s *truncatedSaaSAdminNotificationStore) SaaSAlertNotificationByKey(_ context.Context, notificationKey string) (SaaSAlertNotification, error) {
	s.keyCalls++
	if s.hidden.NotificationKey == notificationKey {
		return s.hidden, nil
	}
	return SaaSAlertNotification{}, nil
}
