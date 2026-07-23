package dashboard

import (
	"strings"
	"testing"
	"time"
)

func TestQueuePayloadRegistryContainsWorkerQueues(t *testing.T) {
	registry := QueuePayloadRegistry()
	if len(registry) != 11 {
		t.Fatalf("registry length = %d", len(registry))
	}
	for _, name := range []string{QueueNameWeWorkCallback, QueueNameEmployeeApply, QueueNameContactWelcome, QueueNameAsyncFileUpload, QueueNameMarkTags, QueueNameMessageRemind, QueueNameWorkRoomSync, QueueNameWorkContactSync, QueueNameWorkDepartmentList, QueueNameMediumMediaIDUpdate, QueueNameEmployeeStatisticApply} {
		descriptor, ok := QueuePayloadDescriptorByName(name)
		if !ok {
			t.Fatalf("descriptor %s not found", name)
		}
		if descriptor.SourceKey == "" || descriptor.ProcessingKey == "" || descriptor.DeadLetterKey == "" || descriptor.PayloadType == "" {
			t.Fatalf("descriptor %s is incomplete: %+v", name, descriptor)
		}
		if descriptor.IdempotencyTTL != 10*time.Minute {
			t.Fatalf("descriptor %s ttl = %s", name, descriptor.IdempotencyTTL)
		}
	}
}

func TestEmployeeStatisticApplyIdempotencyKeyNormalizesSource(t *testing.T) {
	left := EmployeeStatisticApplyIdempotencyKey(EmployeeStatisticApplyEvent{Source: " employeeStatistic "})
	right := EmployeeStatisticApplyIdempotencyKey(EmployeeStatisticApplyEvent{Source: "employeeStatistic"})
	if left == "" || left != right {
		t.Fatalf("keys differ: %q %q", left, right)
	}
	if !strings.HasPrefix(left, "mochat-go:queue-idempotency:employee-statistic-apply:") {
		t.Fatalf("key prefix = %q", left)
	}

	changed := EmployeeStatisticApplyIdempotencyKey(EmployeeStatisticApplyEvent{Source: "manual"})
	if changed == left {
		t.Fatalf("different source should produce a different key")
	}
	tenantChanged := EmployeeStatisticApplyIdempotencyKey(EmployeeStatisticApplyEvent{TenantID: 8, Source: "employeeStatistic"})
	if tenantChanged == left {
		t.Fatalf("different tenant should produce a different key")
	}
	corpChanged := EmployeeStatisticApplyIdempotencyKey(EmployeeStatisticApplyEvent{CorpID: 7, Source: "employeeStatistic"})
	if corpChanged == left || corpChanged == tenantChanged {
		t.Fatalf("different corp should produce a distinct key: base=%q tenant=%q corp=%q", left, tenantChanged, corpChanged)
	}
}

func TestMediumMediaIDUpdateIdempotencyKeyNormalizesMediumIDs(t *testing.T) {
	left := MediumMediaIDUpdateIdempotencyKey(MediumMediaIDUpdateEvent{CorpID: 7, MediumIDs: []int{21, 0, 9, 21}, Source: " media_id_update "})
	right := MediumMediaIDUpdateIdempotencyKey(MediumMediaIDUpdateEvent{CorpID: 7, MediumIDs: []int{9, 21}, Source: "media_id_update"})
	if left == "" || left != right {
		t.Fatalf("keys differ: %q %q", left, right)
	}
	if !strings.HasPrefix(left, "mochat-go:queue-idempotency:medium-media-id-update:") {
		t.Fatalf("key prefix = %q", left)
	}

	changed := MediumMediaIDUpdateIdempotencyKey(MediumMediaIDUpdateEvent{CorpID: 8, MediumIDs: []int{9, 21}, Source: "media_id_update"})
	if changed == left {
		t.Fatalf("different corp should produce a different key")
	}
}

func TestWorkDepartmentListIdempotencyKeyNormalizesCorpIDs(t *testing.T) {
	left := WorkDepartmentListIdempotencyKey(WorkDepartmentListEvent{CorpIDs: []int{7, 0, 2, 7}, UserID: 5, TenantID: 11, Source: " dashboard.department.list "})
	right := WorkDepartmentListIdempotencyKey(WorkDepartmentListEvent{CorpIDs: []int{2, 7}, UserID: 5, TenantID: 11, Source: "dashboard.department.list"})
	if left == "" || left != right {
		t.Fatalf("keys differ: %q %q", left, right)
	}
	if !strings.HasPrefix(left, "mochat-go:queue-idempotency:work-department-list:") {
		t.Fatalf("key prefix = %q", left)
	}

	changed := WorkDepartmentListIdempotencyKey(WorkDepartmentListEvent{CorpIDs: []int{2, 7}, UserID: 6, TenantID: 11, Source: "dashboard.department.list"})
	if changed == left {
		t.Fatalf("different user should produce a different key")
	}
}

func TestWorkContactSyncIdempotencyKeyNormalizesEmployeesAndContacts(t *testing.T) {
	left := WorkContactSyncIdempotencyKey(WorkContactSyncEvent{
		CorpID: 7,
		Employees: []WorkContactSyncEmployee{
			{ID: 12, WXUserID: " lisi "},
			{ID: 11, WXUserID: "zhangsan"},
			{ID: 12, WXUserID: "lisi"},
		},
		ExternalUserIDs: []string{" external-2 ", "external-1", "external-1"},
		Source:          " queue ",
	})
	right := WorkContactSyncIdempotencyKey(WorkContactSyncEvent{
		CorpID: 7,
		Employees: []WorkContactSyncEmployee{
			{ID: 11, WXUserID: "zhangsan"},
			{ID: 12, WXUserID: "lisi"},
		},
		ExternalUserIDs: []string{"external-1", "external-2"},
		Source:          "queue",
	})
	if left == "" || left != right {
		t.Fatalf("keys differ: %q %q", left, right)
	}
	if !strings.HasPrefix(left, "mochat-go:queue-idempotency:work-contact-sync:") {
		t.Fatalf("key prefix = %q", left)
	}
	changed := WorkContactSyncIdempotencyKey(WorkContactSyncEvent{
		CorpID:    7,
		Employees: []WorkContactSyncEmployee{{ID: 11, WXUserID: "zhangsan"}},
		Source:    "queue",
	})
	if changed == left {
		t.Fatalf("different employee set should produce a different key")
	}
}

func TestWorkRoomSyncIdempotencyKeyUsesCorpAndChat(t *testing.T) {
	left := WorkRoomSyncIdempotencyKey(WorkRoomSyncEvent{CorpID: 7, ChatID: " chat-1 ", Source: " room "})
	right := WorkRoomSyncIdempotencyKey(WorkRoomSyncEvent{CorpID: 7, ChatID: "chat-1", Source: "room"})
	if left == "" || left != right {
		t.Fatalf("keys differ: %q %q", left, right)
	}
	if !strings.HasPrefix(left, "mochat-go:queue-idempotency:work-room-sync:") {
		t.Fatalf("key prefix = %q", left)
	}
	fullSync := WorkRoomSyncIdempotencyKey(WorkRoomSyncEvent{CorpID: 7, Source: "room"})
	if fullSync == left {
		t.Fatalf("full sync and single room sync should produce different keys")
	}
	wxCorp := WorkRoomSyncIdempotencyKey(WorkRoomSyncEvent{WXCorpID: "ww-go", ChatID: "chat-1", Source: "room"})
	if wxCorp == left {
		t.Fatalf("wx corpid payload should keep a distinct identity")
	}
}

func TestMessageRemindIdempotencyKeyNormalizesRecipients(t *testing.T) {
	left := MessageRemindIdempotencyKey(MessageRemindEvent{
		CorpID:  7,
		ToUser:  MessageRemindRecipients{" go-b ", "go-a", "go-a"},
		MsgType: "text",
		Content: "提醒",
		Source:  " remind ",
	})
	right := MessageRemindIdempotencyKey(MessageRemindEvent{
		CorpID:  7,
		ToUser:  MessageRemindRecipients{"go-a", "go-b"},
		MsgType: "text",
		Content: "提醒",
		Source:  "remind",
	})
	if left == "" || left != right {
		t.Fatalf("keys differ: %q %q", left, right)
	}
	if !strings.HasPrefix(left, "mochat-go:queue-idempotency:message-remind:") {
		t.Fatalf("key prefix = %q", left)
	}
	changed := MessageRemindIdempotencyKey(MessageRemindEvent{
		CorpID:  7,
		ToParty: MessageRemindRecipients{"go-a", "go-b"},
		MsgType: "text",
		Content: "提醒",
		Source:  "remind",
	})
	if changed == left {
		t.Fatalf("different recipient type should produce a different key")
	}
}

func TestMarkTagsIdempotencyKeyNormalizesTagIDs(t *testing.T) {
	left := MarkTagsIdempotencyKey(MarkTagsEvent{CorpID: 7, ContactID: 101, EmployeeID: 3, TagIDs: []int{9, 0, 2, 9}, Source: " queue "})
	right := MarkTagsIdempotencyKey(MarkTagsEvent{CorpID: 7, ContactID: 101, EmployeeID: 3, TagIDs: []int{2, 9}, Source: "queue"})
	if left == "" || left != right {
		t.Fatalf("keys differ: %q %q", left, right)
	}
	if !strings.HasPrefix(left, "mochat-go:queue-idempotency:mark-tags:") {
		t.Fatalf("key prefix = %q", left)
	}
	changed := MarkTagsIdempotencyKey(MarkTagsEvent{CorpID: 7, ContactID: 102, EmployeeID: 3, TagIDs: []int{2, 9}, Source: "queue"})
	if changed == left {
		t.Fatalf("different contact should produce a different key")
	}
}

func TestAsyncFileUploadIdempotencyKeyNormalizesFileOrder(t *testing.T) {
	left := AsyncFileUploadIdempotencyKey(AsyncFileUploadEvent{
		Files: []AsyncFileUploadFile{
			{SourcePath: " /tmp/b.txt ", TargetPath: " b.txt "},
			{SourcePath: "/tmp/a.txt", TargetPath: "a.txt", DeleteSource: true},
		},
		Source: " file_upload_queue ",
	})
	right := AsyncFileUploadIdempotencyKey(AsyncFileUploadEvent{
		Files: []AsyncFileUploadFile{
			{SourcePath: "/tmp/a.txt", TargetPath: "a.txt", DeleteSource: true},
			{SourcePath: "/tmp/b.txt", TargetPath: "b.txt"},
		},
		Source: "file_upload_queue",
	})
	if left == "" || left != right {
		t.Fatalf("keys differ: %q %q", left, right)
	}
	if !strings.HasPrefix(left, "mochat-go:queue-idempotency:async-file-upload:") {
		t.Fatalf("key prefix = %q", left)
	}
	changed := AsyncFileUploadIdempotencyKey(AsyncFileUploadEvent{
		Files:  []AsyncFileUploadFile{{SourcePath: "/tmp/a.txt", TargetPath: "a.txt"}},
		Source: "file_upload_queue",
	})
	if changed == left {
		t.Fatalf("different delete flag should produce a different key")
	}
}

func TestContactWelcomeIdempotencyKeyUsesContactAndWelcomeCode(t *testing.T) {
	left := ContactWelcomeIdempotencyKey(ContactWelcomeEvent{CorpID: 7, ContactID: 101, EmployeeID: 3, WelcomeCode: " welcome-code "})
	right := ContactWelcomeIdempotencyKey(ContactWelcomeEvent{CorpID: 7, ContactID: 101, EmployeeID: 3, WelcomeCode: "welcome-code"})
	if left == "" || left != right {
		t.Fatalf("keys differ: %q %q", left, right)
	}
	if !strings.HasPrefix(left, "mochat-go:queue-idempotency:contact-welcome:") {
		t.Fatalf("key prefix = %q", left)
	}
	changed := ContactWelcomeIdempotencyKey(ContactWelcomeEvent{CorpID: 7, ContactID: 102, EmployeeID: 3, WelcomeCode: "welcome-code"})
	if changed == left {
		t.Fatalf("different contact should produce a different key")
	}
}

func TestEmployeeApplyIdempotencyKeyNormalizesCorpIDs(t *testing.T) {
	left := EmployeeApplyIdempotencyKey(EmployeeApplyEvent{CorpIDs: []int{7, 0, 2, 7}, UserID: 5, Source: " dashboard.corp.store "})
	right := EmployeeApplyIdempotencyKey(EmployeeApplyEvent{CorpIDs: []int{2, 7}, UserID: 5, Source: "dashboard.corp.store"})
	if left == "" || left != right {
		t.Fatalf("keys differ: %q %q", left, right)
	}
	if !strings.HasPrefix(left, "mochat-go:queue-idempotency:employee-apply:") {
		t.Fatalf("key prefix = %q", left)
	}

	manual := EmployeeApplyIdempotencyKey(EmployeeApplyEvent{CorpIDs: []int{2, 7}, UserID: 5, Source: "manual"})
	if manual == left {
		t.Fatalf("different source should produce a different key")
	}
}

func TestWeWorkCallbackIdempotencyKeyUsesBusinessIdentity(t *testing.T) {
	base := WeWorkCallbackEvent{
		CorpID:    7,
		WxCorpID:  " ww-go ",
		EventPath: " event.change_contact.create_user ",
		Message:   map[string]string{"ToUserName": "ww-go", "CreateTime": "1710000000", "UserID": "go-user", "Event": "change_contact"},
		RawXML:    " <xml/> ",
	}
	left := base
	left.ReceivedAt = "2026-07-04 12:00:00"
	left.RawXML = "<xml><UserID>go-user</UserID><CreateTime>1710000000</CreateTime></xml>"
	right := base
	right.ReceivedAt = "2026-07-04 12:01:00"
	right.RawXML = "<xml><CreateTime>1710000000</CreateTime><UserID>go-user</UserID></xml>"

	leftKey := WeWorkCallbackIdempotencyKey(left)
	rightKey := WeWorkCallbackIdempotencyKey(right)
	if leftKey == "" || leftKey != rightKey {
		t.Fatalf("keys differ: %q %q", leftKey, rightKey)
	}
	if !strings.HasPrefix(leftKey, "mochat-go:queue-idempotency:wework-callback:") {
		t.Fatalf("key prefix = %q", leftKey)
	}

	changed := base
	changed.Message = map[string]string{"ToUserName": "ww-go", "CreateTime": "1710000001", "UserID": "go-user", "Event": "change_contact"}
	if WeWorkCallbackIdempotencyKey(changed) == leftKey {
		t.Fatalf("different business event should produce a different key")
	}
}

func TestWeWorkCallbackIdempotencyKeyFallsBackToRawXML(t *testing.T) {
	left := WeWorkCallbackEvent{CorpID: 7, EventPath: "unknown", RawXML: "<xml><A>1</A></xml>"}
	right := left
	right.RawXML = "<xml><A>2</A></xml>"
	if WeWorkCallbackIdempotencyKey(left) == WeWorkCallbackIdempotencyKey(right) {
		t.Fatalf("different raw xml fallback should produce a different key")
	}
}
