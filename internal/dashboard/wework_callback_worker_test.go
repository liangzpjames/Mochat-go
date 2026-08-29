package dashboard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

func TestWeWorkCallbackWorkerTriggersArchiveSyncForMessageAuditNotification(t *testing.T) {
	trigger := &fakeWorkMessageArchiveSyncTrigger{}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, &fakeWeWorkCallbackWorkerStore{}, &fakeWeWorkCallbackWorkerClient{}, "", log.Default()).
		WithArchiveSyncTrigger(trigger)

	if err := worker.Process(context.Background(), WeWorkCallbackEvent{CorpID: 4, EventPath: "event.msgaudit_notify"}); err != nil {
		t.Fatal(err)
	}
	if len(trigger.corpIDs) != 1 || trigger.corpIDs[0] != 4 {
		t.Fatalf("triggered corps=%v, want [4]", trigger.corpIDs)
	}

	trigger.err = errors.New("archive bridge unavailable")
	if err := worker.Process(context.Background(), WeWorkCallbackEvent{CorpID: 4, EventPath: "event.msgaudit_notify"}); !errors.Is(err, trigger.err) {
		t.Fatalf("Process error=%v, want trigger error", err)
	}

	withoutTrigger := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, &fakeWeWorkCallbackWorkerStore{}, &fakeWeWorkCallbackWorkerClient{}, "", log.Default())
	if err := withoutTrigger.Process(context.Background(), WeWorkCallbackEvent{CorpID: 4, EventPath: "event.msgaudit_notify"}); err != nil {
		t.Fatalf("disabled archive trigger error=%v", err)
	}
}

func TestWeWorkCallbackWorkerRunsFromDurableInboxWithoutRedis(t *testing.T) {
	completed := make(chan WeWorkCallbackClaim, 1)
	store := &fakeDurableWeWorkCallbackWorkerStore{
		fakeWeWorkCallbackWorkerStore: &fakeWeWorkCallbackWorkerStore{},
		claims: []WeWorkCallbackClaim{{
			ID: 9, EventKey: strings.Repeat("a", 64), PayloadFingerprint: strings.Repeat("b", 64),
			LeaseToken: "lease-token", LeaseFence: 4, Attempt: 1,
			Event: WeWorkCallbackEvent{TenantID: 3, CorpID: 7, EventPath: "event.ignored", Message: map[string]string{}},
		}},
		completed: completed,
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, &fakeWeWorkCallbackWorkerClient{}, "", log.Default())
	worker.pollTimeout = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()

	select {
	case claim := <-completed:
		if claim.EventKey != strings.Repeat("a", 64) || claim.LeaseFence != 4 {
			t.Fatalf("completed claim=%+v", claim)
		}
		cancel()
	case <-time.After(time.Second):
		cancel()
		t.Fatal("durable callback claim was not completed")
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("run error=%v", err)
	}
	if len(store.failedClaims) != 0 {
		t.Fatalf("unexpected failures=%d", len(store.failedClaims))
	}
	if len(store.claimLeaseDurations) == 0 || store.claimLeaseDurations[0] <= worker.processingTimeout {
		t.Fatalf("claim lease duration=%v processing timeout=%s", store.claimLeaseDurations, worker.processingTimeout)
	}
}

func TestWeWorkCallbackWorkerImportsLegacyBacklogBeforeAcknowledgingRedis(t *testing.T) {
	legacy := &fakeLegacyWeWorkCallbackBacklog{
		stats: LegacyWeWorkCallbackBacklogStats{Pending: 1},
		deliveries: []LegacyWeWorkCallbackDelivery{{
			Raw:   "legacy-raw-1",
			Event: WeWorkCallbackEvent{CorpID: 7, WxCorpID: "wx-corp", EventPath: "event.change_contact.delete_user", Message: map[string]string{"UserID": "go-user"}, RawXML: "<xml>secret</xml>"},
		}},
	}
	store := &fakeDurableWeWorkCallbackWorkerStore{fakeWeWorkCallbackWorkerStore: &fakeWeWorkCallbackWorkerStore{tenantIDs: map[int]int{7: 21}}}
	if _, err := ImportLegacyWeWorkCallbackBacklog(context.Background(), legacy, store, log.Default()); err != nil {
		t.Fatal(err)
	}
	if len(store.acceptedLegacyEvents) != 1 || store.acceptedLegacyEvents[0].TenantID != 21 || store.acceptedLegacyEvents[0].RawXML != "" {
		t.Fatalf("accepted legacy events=%+v", store.acceptedLegacyEvents)
	}
	if len(legacy.acked) != 1 || legacy.acked[0].Raw != "legacy-raw-1" {
		t.Fatalf("legacy acked=%+v", legacy.acked)
	}
}

func TestWeWorkCallbackWorkerKeepsLegacyDeliveryWhenDurableAcceptanceFails(t *testing.T) {
	legacy := &fakeLegacyWeWorkCallbackBacklog{
		stats:      LegacyWeWorkCallbackBacklogStats{Processing: 1},
		deliveries: []LegacyWeWorkCallbackDelivery{{Raw: "legacy-raw-db-failure", Event: WeWorkCallbackEvent{TenantID: 21, CorpID: 7, EventPath: "event.ignored", Message: map[string]string{}}}},
	}
	store := &fakeDurableWeWorkCallbackWorkerStore{fakeWeWorkCallbackWorkerStore: &fakeWeWorkCallbackWorkerStore{}, acceptLegacyErr: errors.New("mysql unavailable")}
	if _, err := ImportLegacyWeWorkCallbackBacklog(context.Background(), legacy, store, log.Default()); err == nil {
		t.Fatal("expected durable acceptance failure")
	}
	if len(legacy.acked) != 0 {
		t.Fatalf("legacy delivery was removed after DB failure: %+v", legacy.acked)
	}
	if len(legacy.deliveries) != 1 {
		t.Fatalf("legacy delivery was not retained for retry: %+v", legacy.deliveries)
	}
	store.acceptLegacyErr = nil
	legacy.preflights = 0
	if _, err := ImportLegacyWeWorkCallbackBacklog(context.Background(), legacy, store, log.Default()); err != nil {
		t.Fatalf("retry legacy import: %v", err)
	}
	if len(legacy.acked) != 1 || len(legacy.deliveries) != 0 {
		t.Fatalf("retry acked=%+v deliveries=%+v", legacy.acked, legacy.deliveries)
	}
}

func TestWeWorkCallbackWorkerLegacyDuplicateImportKeepsOneDurableEvent(t *testing.T) {
	event := WeWorkCallbackEvent{TenantID: 21, CorpID: 7, WxCorpID: "wx-corp", EventPath: "event.ignored", Message: map[string]string{"MsgId": "legacy-provider-id"}}
	legacy := &fakeLegacyWeWorkCallbackBacklog{
		stats:      LegacyWeWorkCallbackBacklogStats{Processing: 2},
		deliveries: []LegacyWeWorkCallbackDelivery{{Raw: "legacy-a", Event: event}, {Raw: "legacy-b", Event: event}},
	}
	store := &fakeDurableWeWorkCallbackWorkerStore{fakeWeWorkCallbackWorkerStore: &fakeWeWorkCallbackWorkerStore{}}
	if _, err := ImportLegacyWeWorkCallbackBacklog(context.Background(), legacy, store, log.Default()); err != nil {
		t.Fatal(err)
	}
	if len(store.acceptedLegacyKeys) != 2 || len(store.legacyUniqueKeys) != 1 || len(legacy.acked) != 2 {
		t.Fatalf("accept calls=%d unique=%d acked=%d", len(store.acceptedLegacyKeys), len(store.legacyUniqueKeys), len(legacy.acked))
	}
}

func TestWeWorkCallbackWorkerFailsClosedWhenLegacyDeadBacklogExists(t *testing.T) {
	legacy := &fakeLegacyWeWorkCallbackBacklog{stats: LegacyWeWorkCallbackBacklogStats{Pending: 1, Dead: 2}}
	store := &fakeDurableWeWorkCallbackWorkerStore{fakeWeWorkCallbackWorkerStore: &fakeWeWorkCallbackWorkerStore{}}
	if _, err := ImportLegacyWeWorkCallbackBacklog(context.Background(), legacy, store, log.Default()); !errors.Is(err, ErrLegacyWeWorkCallbackDeadBacklog) {
		t.Fatalf("error=%v", err)
	}
	if legacy.nextCalls != 0 || len(legacy.acked) != 0 {
		t.Fatalf("dead preflight mutated backlog: next=%d acked=%d", legacy.nextCalls, len(legacy.acked))
	}
}

func TestWeWorkCallbackWorkerFailsDurableClaimWithOriginalFence(t *testing.T) {
	store := &fakeDurableWeWorkCallbackWorkerStore{fakeWeWorkCallbackWorkerStore: &fakeWeWorkCallbackWorkerStore{}}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, &fakeWeWorkCallbackWorkerClient{}, "", log.Default())
	claim := WeWorkCallbackClaim{ID: 10, EventKey: strings.Repeat("c", 64), LeaseToken: "lease-token", LeaseFence: 7, Attempt: 2, Event: WeWorkCallbackEvent{EventPath: "event.change_contact.create_user"}}

	worker.handleClaim(context.Background(), claim)
	if len(store.failedClaims) != 1 || store.failedClaims[0].EventKey != claim.EventKey || store.failedClaims[0].LeaseFence != claim.LeaseFence || len(store.completedClaims) != 0 {
		t.Fatalf("failed=%+v completed=%+v", store.failedClaims, store.completedClaims)
	}
}

type fakeWorkMessageArchiveSyncTrigger struct {
	corpIDs []int
	err     error
}

func (f *fakeWorkMessageArchiveSyncTrigger) RunCorp(_ context.Context, corpID int) error {
	f.corpIDs = append(f.corpIDs, corpID)
	return f.err
}

func TestWeWorkCallbackWorkerSyncsEmployeesForContactChange(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		employeeCredentials: []WorkEmployeeSyncCredential{{CorpID: 7, TenantID: 1, WXCorpID: "ww-go", EmployeeSecret: "employee-secret", ContactSecret: "contact-secret"}},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		departments: []WorkEmployeeSyncDepartment{{WXDepartmentID: 1, Name: "总部"}},
		userDetails: map[string]WorkEmployeeSyncEmployee{
			"go-user": {WXUserID: "go-user", Name: "Go员工", DepartmentIDs: []int{1}},
		},
		followUsers: []string{"go-user"},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{CorpID: 7, EventPath: "event.change_contact.create_user", Message: map[string]string{"UserID": "go-user"}})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if len(store.syncedEmployees) != 1 || store.syncedEmployees[0].WXUserID != "go-user" {
		t.Fatalf("synced employees = %#v", store.syncedEmployees)
	}
	if len(store.syncedDepartments) != 1 || store.syncedDepartments[0].WXDepartmentID != 1 {
		t.Fatalf("synced departments = %#v", store.syncedDepartments)
	}
	if len(client.userDetailCalls) != 1 || client.userDetailCalls[0] != "go-user" {
		t.Fatalf("user detail calls = %#v", client.userDetailCalls)
	}
}

func TestWeWorkCallbackWorkerSyncsTagsAndRooms(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		tags:       []WorkContactTagSyncGroup{{WXGroupID: "tag-group", GroupName: "标签组", Tags: []WorkContactTagSyncTag{{WXContactTagID: "tag-id", Name: "标签"}}}},
		groupChats: []WorkRoomSyncGroupChat{{WXChatID: "room-1", Status: 1}},
		rooms: map[string]WorkRoomSyncRoom{
			"room-1": {WXChatID: "room-1", Name: "客户群"},
		},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default())

	if err := worker.Process(context.Background(), WeWorkCallbackEvent{CorpID: 7, EventPath: "event.change_external_tag.create", Message: map[string]string{"TagType": "tag", "Id": "tag-id"}}); err != nil {
		t.Fatalf("tag process error = %v", err)
	}
	if store.syncedTagGroupCorpID != 7 || store.syncedTagGroup.WXGroupID != "tag-group" {
		t.Fatalf("synced tag group = corp %d group %#v", store.syncedTagGroupCorpID, store.syncedTagGroup)
	}

	if err := worker.Process(context.Background(), WeWorkCallbackEvent{CorpID: 7, EventPath: "event.change_external_chat.update", Message: map[string]string{"ChatId": "room-1"}}); err != nil {
		t.Fatalf("room process error = %v", err)
	}
	if len(store.syncedRooms) != 1 || store.syncedRooms[0].WXChatID != "room-1" {
		t.Fatalf("synced rooms = %#v", store.syncedRooms)
	}
}

func TestWeWorkCallbackWorkerNoopsPHPEventsWithoutListeners(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		tags: []WorkContactTagSyncGroup{{WXGroupID: "tag-group", GroupName: "标签组", Tags: []WorkContactTagSyncTag{{WXContactTagID: "tag-id", Name: "标签"}}}},
		contacts: map[string]WorkContactSyncContact{
			"external-user": {WXExternalUserID: "external-user", FollowUsers: []WorkContactSyncFollowUser{{UserID: "go-user"}}},
		},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default())
	events := []WeWorkCallbackEvent{
		{CorpID: 7, EventPath: "event.change_external_tag.shuffle"},
		{CorpID: 7, EventPath: "event.change_contact.update_tag"},
		{CorpID: 7, EventPath: "event.change_external_contact.add_half_external_contact", Message: map[string]string{"UserID": "go-user", "ExternalUserID": "external-user"}},
		{CorpID: 7, EventPath: "event.change_external_contact.transfer_fail", Message: map[string]string{"UserID": "go-user", "ExternalUserID": "external-user"}},
	}

	for _, event := range events {
		if err := worker.Process(context.Background(), event); err != nil {
			t.Fatalf("Process(%s) error = %v", event.EventPath, err)
		}
	}
	if len(store.syncedTags) != 0 || store.syncedTagGroupCorpID != 0 {
		t.Fatalf("unexpected tag sync tags=%#v group=%#v", store.syncedTags, store.syncedTagGroup)
	}
	if len(store.syncedContactBundles) != 0 || store.syncedSingleContactCorpID != 0 || store.removedContactCorpID != 0 {
		t.Fatalf("unexpected contact sync bundles=%#v single=%#v removed=%d", store.syncedContactBundles, store.syncedSingleContact, store.removedContactCorpID)
	}
	if len(client.externalDetailCalls) != 0 || len(client.groupIDCalls) != 0 || len(client.tagIDCalls) != 0 {
		t.Fatalf("unexpected client calls external=%#v groups=%#v tags=%#v", client.externalDetailCalls, client.groupIDCalls, client.tagIDCalls)
	}
}

func TestWeWorkCallbackWorkerSyncsSingleContactFromEvent(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts: map[string]WorkContactSyncContact{
			"external-user": {
				WXExternalUserID: "external-user",
				Name:             "客户",
				BusinessNo:       "BIZ-1",
				FollowUsers:      []WorkContactSyncFollowUser{{UserID: "go-user", Remark: "备注"}},
			},
		},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.add_external_contact",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if store.syncedSingleContactCorpID != 7 || store.syncedSingleContactEmployee.ID != 3 || store.syncedSingleContact.WXExternalUserID != "external-user" || store.syncedSingleContact.BusinessNo != "BIZ-1" || !store.syncedSingleContactCreateMissing {
		t.Fatalf("synced single contact = corp %d employee %#v contact %#v create=%v", store.syncedSingleContactCorpID, store.syncedSingleContactEmployee, store.syncedSingleContact, store.syncedSingleContactCreateMissing)
	}
	if len(store.syncedContactBundles) != 0 {
		t.Fatalf("unexpected full contact sync = %#v", store.syncedContactBundles)
	}
	if len(client.externalDetailCalls) != 1 || client.externalDetailCalls[0] != "external-user" {
		t.Fatalf("external detail calls = %#v", client.externalDetailCalls)
	}
}

func TestWeWorkCallbackWorkerEnqueuesGenericWelcomeForNewContact(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
		syncResult:           WorkContactSyncResult{ContactID: 101},
		greetings: []GreetingItem{
			{ID: 1, CorpID: 7, Words: "通用##客户名称##", RangeType: 1},
			{ID: 2, CorpID: 7, Words: "指定##客户名称##", MediumID: 21, RangeType: 2, EmployeeIDs: []int{3}},
		},
		greetingMedia: map[int]GreetingMedium{
			21: {
				ID:   21,
				Type: 3,
				Content: map[string]any{
					"title":       "资料",
					"imageLink":   "https://example.com/doc",
					"imagePath":   "image/a.jpg",
					"description": "说明",
				},
			},
		},
	}
	queue := &fakeWeWorkCallbackWorkerQueue{welcomeStatuses: map[int]int{}}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts: map[string]WorkContactSyncContact{
			"external-user": {
				WXExternalUserID: "external-user",
				Name:             "客户A",
				FollowUsers:      []WorkContactSyncFollowUser{{UserID: "go-user"}},
			},
		},
	}
	worker := NewWeWorkCallbackWorker(fakeWeWorkCallbackWorkerCapabilities(queue), store, client, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.add_external_contact",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user", "WelcomeCode": "welcome-code"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if queue.contactWelcomeEvent.ContactID != 101 || queue.contactWelcomeEvent.EmployeeID != 3 || queue.contactWelcomeEvent.WelcomeCode != "welcome-code" {
		t.Fatalf("welcome event = %#v", queue.contactWelcomeEvent)
	}
	if queue.contactWelcomeEvent.Content.Text != "指定##客户名称##" || queue.contactWelcomeEvent.Content.Medium == nil || queue.contactWelcomeEvent.Content.Medium.MediumType != 3 {
		t.Fatalf("welcome content = %#v", queue.contactWelcomeEvent.Content)
	}
	if queue.welcomeStatuses[101] != 1 || queue.welcomeStatusTTL != contactWelcomeStatusTTL {
		t.Fatalf("welcome status = %#v ttl=%s", queue.welcomeStatuses, queue.welcomeStatusTTL)
	}
}

func TestWeWorkCallbackWorkerEnqueuesContactTimeAutoTagForNewContact(t *testing.T) {
	queue := &fakeWeWorkCallbackWorkerQueue{}
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
		syncResult:           WorkContactSyncResult{ContactID: 101},
		contactTimeTaskResult: AutoTagContactTimeTaskResult{
			MarkTagsEvents: []MarkTagsEvent{{
				CorpID:          7,
				ContactID:       101,
				EmployeeID:      3,
				TagIDs:          []int{18},
				Source:          "auto-tag-contact-time:88",
				AutoTagID:       66,
				AutoTagRecordID: 88,
			}},
		},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts: map[string]WorkContactSyncContact{
			"external-user": {
				WXExternalUserID: "external-user",
				Name:             "客户A",
				FollowUsers:      []WorkContactSyncFollowUser{{UserID: "go-user"}},
			},
		},
	}
	worker := NewWeWorkCallbackWorker(fakeWeWorkCallbackWorkerCapabilities(queue), store, client, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.add_external_contact",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user", "WelcomeCode": "welcome-code"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if store.contactTimeTaskCorpID != 7 || store.contactTimeTaskEmployeeID != 3 || store.contactTimeTaskContactID != 101 || store.contactTimeTaskCalls != 1 {
		t.Fatalf("contact time auto tag task = corp %d employee %d contact %d calls %d", store.contactTimeTaskCorpID, store.contactTimeTaskEmployeeID, store.contactTimeTaskContactID, store.contactTimeTaskCalls)
	}
	if len(queue.markTagsEvents) != 1 {
		t.Fatalf("mark tag events = %#v", queue.markTagsEvents)
	}
	event := queue.markTagsEvents[0]
	if event.ContactID != 101 || event.EmployeeID != 3 || event.AutoTagRecordID != 88 || len(event.TagIDs) != 1 || event.TagIDs[0] != 18 {
		t.Fatalf("mark tag event = %#v", event)
	}
}

func TestWeWorkCallbackWorkerGeneratesContactSOPLogForNewContact(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 30, 0, 0, time.Local)
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
		syncResult:           WorkContactSyncResult{ContactID: 101},
		contactSOPSources: []ContactSOPLogSource{{
			ID:             66,
			CorpID:         7,
			SettingRaw:     `{"content":[{"type":"text","value":"回调个人SOP"}],"delayMinutes":0}`,
			EmployeeIDsRaw: `[3]`,
			ContactIDsRaw:  `[101]`,
		}},
		contactSOPTargets: []ContactSOPLogTarget{{
			EmployeeWXUserID:      "go-user",
			ContactWXExternalUser: "external-user",
			AnchorTime:            now,
		}},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts: map[string]WorkContactSyncContact{
			"external-user": {
				WXExternalUserID: "external-user",
				Name:             "客户A",
				FollowUsers:      []WorkContactSyncFollowUser{{UserID: "go-user"}},
			},
		},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default()).WithNow(func() time.Time { return now })

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.add_external_contact",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if len(store.contactSOPLogs) != 1 {
		t.Fatalf("contact SOP logs = %#v", store.contactSOPLogs)
	}
	logItem := store.contactSOPLogs[0]
	if logItem.ContactSOPID != 66 || logItem.EmployeeWXUserID != "go-user" || logItem.ContactWXExternalUser != "external-user" || !strings.Contains(logItem.TaskRaw, "回调个人SOP") || !logItem.CreatedAt.Equal(now) {
		t.Fatalf("contact SOP log = %#v", logItem)
	}
}

func TestWeWorkCallbackWorkerEnqueuesChannelCodeWelcomeBeforeGeneric(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
		syncResult:           WorkContactSyncResult{ContactID: 101},
		greetings:            []GreetingItem{{ID: 1, CorpID: 7, Words: "通用欢迎语", RangeType: 1}},
		channelCodeWelcomes: map[int]ChannelCodeWelcome{
			88: {
				ID: 88,
				WelcomeMessage: map[string]any{
					"scanCodePush": 1,
					"messageDetail": []any{
						map[string]any{"type": 1, "welcomeContent": "渠道欢迎##客户名称##", "mediumId": 22},
					},
				},
			},
		},
		greetingMedia: map[int]GreetingMedium{
			22: {
				ID:   22,
				Type: 3,
				Content: map[string]any{
					"title":     "渠道资料",
					"imageLink": "https://example.com/channel",
				},
			},
		},
	}
	queue := &fakeWeWorkCallbackWorkerQueue{welcomeStatuses: map[int]int{}}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts: map[string]WorkContactSyncContact{
			"external-user": {
				WXExternalUserID: "external-user",
				Name:             "客户A",
				FollowUsers:      []WorkContactSyncFollowUser{{UserID: "go-user"}},
			},
		},
	}
	worker := NewWeWorkCallbackWorker(fakeWeWorkCallbackWorkerCapabilities(queue), store, client, "worker-secret", log.Default()).
		WithNow(func() time.Time { return time.Date(2026, 7, 4, 9, 30, 0, 0, time.Local) })

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.add_external_contact",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user", "WelcomeCode": "welcome-code", "State": "channelCode-88"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if queue.contactWelcomeEvent.ContactID != 101 || queue.contactWelcomeEvent.EmployeeID != 3 || queue.contactWelcomeEvent.WelcomeCode != "welcome-code" {
		t.Fatalf("welcome event = %#v", queue.contactWelcomeEvent)
	}
	if queue.contactWelcomeEvent.Content.Text != "渠道欢迎##客户名称##" || queue.contactWelcomeEvent.Content.Medium == nil || queue.contactWelcomeEvent.Content.Medium.MediumType != 3 {
		t.Fatalf("welcome content = %#v", queue.contactWelcomeEvent.Content)
	}
	if queue.contactWelcomeEvent.Content.Medium.MediumContent["title"] != "渠道资料" {
		t.Fatalf("welcome medium = %#v", queue.contactWelcomeEvent.Content.Medium)
	}
}

func TestWeWorkCallbackWorkerEnqueuesWorkRoomAutoPullWelcomeBeforeGeneric(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
		syncResult:           WorkContactSyncResult{ContactID: 101},
		greetings:            []GreetingItem{{ID: 1, CorpID: 7, Words: "通用欢迎语", RangeType: 1}},
		workRoomAutoPullWelcomes: map[int]WorkRoomAutoPullWelcome{
			77: {
				ID:           77,
				LeadingWords: "入群引导##客户名称##",
				Rooms: []WorkRoomAutoPullWelcomeRoom{{
					RoomID:        9001,
					MaxNum:        50,
					RoomMax:       200,
					MemberNum:     12,
					RoomQRCodeURL: "room/qrcode-9001.png",
				}},
			},
		},
	}
	queue := &fakeWeWorkCallbackWorkerQueue{welcomeStatuses: map[int]int{}}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts: map[string]WorkContactSyncContact{
			"external-user": {
				WXExternalUserID: "external-user",
				Name:             "客户A",
				FollowUsers:      []WorkContactSyncFollowUser{{UserID: "go-user"}},
			},
		},
	}
	worker := NewWeWorkCallbackWorker(fakeWeWorkCallbackWorkerCapabilities(queue), store, client, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.add_external_contact",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user", "WelcomeCode": "welcome-code", "State": "workRoomAutoPullId-77"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	content := queue.contactWelcomeEvent.Content
	if content.Text != "入群引导##客户名称##" || content.Medium == nil || content.Medium.MediumType != 2 {
		t.Fatalf("welcome content = %#v", content)
	}
	if content.Medium.MediumContent["imagePath"] != "room/qrcode-9001.png" {
		t.Fatalf("welcome medium = %#v", content.Medium)
	}
}

func TestWeWorkCallbackWorkerEnqueuesFissionWelcomeBeforeGeneric(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
		syncResult:           WorkContactSyncResult{ContactID: 101},
		greetings:            []GreetingItem{{ID: 1, CorpID: 7, Words: "通用欢迎语", RangeType: 1}},
		workFissionWelcomes: map[int]WorkFissionContactWelcome{
			77: {
				ParentContactID: 77,
				FissionID:       903,
				MsgText:         "裂变欢迎##客户名称##",
				LinkTitle:       "裂变任务",
				LinkDesc:        "查看助力进度",
				LinkCoverURL:    "fission/cover.png",
			},
		},
	}
	queue := &fakeWeWorkCallbackWorkerQueue{welcomeStatuses: map[int]int{}}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts: map[string]WorkContactSyncContact{
			"external-user": {
				WXExternalUserID: "external-user",
				Name:             "客户A",
				FollowUsers:      []WorkContactSyncFollowUser{{UserID: "go-user"}},
			},
		},
	}
	worker := NewWeWorkCallbackWorker(fakeWeWorkCallbackWorkerCapabilities(queue), store, client, "worker-secret", log.Default()).
		WithWorkFissionBaseURLs("http://api.example.com", "http://op.example.com")

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.add_external_contact",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user", "WelcomeCode": "welcome-code", "State": "fission-77"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	content := queue.contactWelcomeEvent.Content
	if content.Text != "裂变欢迎##客户名称##" || content.Medium == nil || content.Medium.MediumType != 3 {
		t.Fatalf("welcome content = %#v", content)
	}
	medium := content.Medium.MediumContent
	if medium["title"] != "裂变任务" || medium["description"] != "查看助力进度" || medium["imagePath"] != "fission/cover.png" {
		t.Fatalf("welcome medium = %#v", content.Medium)
	}
	if medium["imageLink"] != "http://op.example.com/auth/workFission?id=903&target=%2FworkFission%3Fid%3D903" {
		t.Fatalf("welcome imageLink = %#v", medium["imageLink"])
	}
}

func TestWeWorkCallbackWorkerMarksWorkRoomAutoPullTagsForNewContact(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
		syncResult:           WorkContactSyncResult{ContactID: 101},
		workRoomAutoPullWelcomes: map[int]WorkRoomAutoPullWelcome{
			77: {
				ID:     77,
				TagIDs: []int{31, 32},
			},
		},
		updateProfileResult: WorkContactUpdateResult{
			WXUserID:         "go-user",
			WXExternalUserID: "external-user",
			AddedWXTagIDs:    []string{"wx-tag-auto"},
			AddedTagNames:    []string{"自动拉群标签"},
		},
		updateProfileFound: true,
	}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts: map[string]WorkContactSyncContact{
			"external-user": {
				WXExternalUserID: "external-user",
				Name:             "客户A",
				FollowUsers:      []WorkContactSyncFollowUser{{UserID: "go-user"}},
			},
		},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.add_external_contact",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user", "State": "workRoomAutoPullId-77"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if store.updatedProfileCalls != 1 || store.updatedProfileValues.CorpID != 7 || store.updatedProfileValues.ContactID != 101 || store.updatedProfileValues.EmployeeID != 3 {
		t.Fatalf("updated profile values = calls:%d values:%#v", store.updatedProfileCalls, store.updatedProfileValues)
	}
	if !store.updatedProfileValues.HasTag || len(store.updatedProfileValues.TagIDs) != 2 || store.updatedProfileValues.TagIDs[0] != 31 || store.updatedProfileValues.TagIDs[1] != 32 {
		t.Fatalf("updated tag ids = %#v", store.updatedProfileValues)
	}
	if client.markTagsPayload.UserID != "go-user" || client.markTagsPayload.ExternalUserID != "external-user" || len(client.markTagsPayload.AddTag) != 1 || client.markTagsPayload.AddTag[0] != "wx-tag-auto" {
		t.Fatalf("mark tags payload = %#v", client.markTagsPayload)
	}
}

func TestWeWorkCallbackWorkerRejectsUnmappedContactWelcomeTags(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		workRoomAutoPullWelcomes: map[int]WorkRoomAutoPullWelcome{77: {ID: 77, TagIDs: []int{31}}},
		updateProfileFound:       true,
		updateProfileResult: WorkContactUpdateResult{
			WXUserID: "go-user", WXExternalUserID: "external-user", TagSyncRequested: true, UnsyncableTagIDs: []int{31},
		},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, &fakeWeWorkCallbackWorkerClient{}, "worker-secret", log.Default())

	err := worker.markContactTagsFromState(context.Background(), 7, RoomWelcomeCorpCredential{CorpID: 7}, 3, 101, WeWorkCallbackEvent{
		Message: map[string]string{"State": "workRoomAutoPullId-77"},
	})
	if err == nil {
		t.Fatal("unmapped callback tag was reported as fully synchronized")
	}
}

func TestWeWorkCallbackWorkerMarksFissionTagsForNewContact(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
		syncResult:           WorkContactSyncResult{ContactID: 101},
		workFissionRules: map[int]WorkFissionContactRule{
			88: {
				ID:     88,
				TagIDs: []int{41},
			},
		},
		updateProfileResult: WorkContactUpdateResult{
			WXUserID:         "go-user",
			WXExternalUserID: "external-user",
			AddedWXTagIDs:    []string{"wx-fission-tag"},
			AddedTagNames:    []string{"裂变标签"},
		},
		updateProfileFound: true,
	}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts: map[string]WorkContactSyncContact{
			"external-user": {
				WXExternalUserID: "external-user",
				Name:             "客户A",
				FollowUsers:      []WorkContactSyncFollowUser{{UserID: "go-user"}},
			},
		},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.add_external_contact",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user", "State": "fission-88"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if store.updatedProfileCalls != 1 || store.updatedProfileValues.ContactID != 101 || len(store.updatedProfileValues.TagIDs) != 1 || store.updatedProfileValues.TagIDs[0] != 41 {
		t.Fatalf("updated profile values = calls:%d values:%#v", store.updatedProfileCalls, store.updatedProfileValues)
	}
	if client.markTagsPayload.UserID != "go-user" || client.markTagsPayload.ExternalUserID != "external-user" || len(client.markTagsPayload.AddTag) != 1 || client.markTagsPayload.AddTag[0] != "wx-fission-tag" {
		t.Fatalf("mark tags payload = %#v", client.markTagsPayload)
	}
}

func TestWeWorkCallbackWorkerHandlesFissionAddContact(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
		syncResult:           WorkContactSyncResult{ContactID: 101, ContactWasNew: true},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts: map[string]WorkContactSyncContact{
			"external-user": {
				WXExternalUserID: "external-user",
				Name:             "裂变客户",
				Avatar:           "https://wecom.example/avatar.png",
				UnionID:          "union-fission",
				FollowUsers:      []WorkContactSyncFollowUser{{UserID: "go-user"}},
			},
		},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.add_external_contact",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user", "State": "fission-77"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	event := store.workFissionAddContactEvent
	if store.workFissionAddContactCalls != 1 || event.CorpID != 7 || event.ParentContactID != 77 || event.EmployeeID != 3 || event.EmployeeWXUserID != "go-user" {
		t.Fatalf("fission add contact event = calls:%d event:%#v", store.workFissionAddContactCalls, event)
	}
	if event.WXExternalUserID != "external-user" || event.UnionID != "union-fission" || event.Name != "裂变客户" || event.Avatar != "https://wecom.example/avatar.png" || !event.IsNew {
		t.Fatalf("fission add contact payload = %#v", event)
	}
}

func TestWeWorkCallbackWorkerSendsFissionEmployeeReminder(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
		syncResult:           WorkContactSyncResult{ContactID: 101, ContactWasNew: true},
		remindAgent:          RoomTagPullAgentCredential{CorpID: 7, WXCorpID: "ww-go", WXAgentID: "100001", WXSecret: "agent-secret"},
		workFissionAddContactResult: WorkFissionAddContactResult{
			ParentContactID: 77,
			FissionID:       903,
			InviteCount:     1,
			TotalCount:      1,
			Completed:       true,
			EmployeeReminder: &WorkFissionEmployeeReminder{
				ToUser:  "service-a|service-b",
				Content: "客户【裂变客户】已完成裂变任务：Go裂变活动",
			},
		},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts: map[string]WorkContactSyncContact{
			"external-user": {
				WXExternalUserID: "external-user",
				Name:             "裂变客户",
				UnionID:          "union-fission",
				FollowUsers:      []WorkContactSyncFollowUser{{UserID: "go-user"}},
			},
		},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.add_external_contact",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user", "State": "fission-77"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if client.agentTextCredential.WXAgentID != "100001" || client.agentTextToUser != "service-a|service-b" || client.agentTextContent != "客户【裂变客户】已完成裂变任务：Go裂变活动" {
		t.Fatalf("agent text = credential:%#v to:%q content:%q", client.agentTextCredential, client.agentTextToUser, client.agentTextContent)
	}
	if client.agentTextDuplicateChecks != 1 {
		t.Fatalf("provider duplicate checks=%d, want 1", client.agentTextDuplicateChecks)
	}
}

func TestWeWorkCallbackWorkerSendsFissionCustomerPush(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
		syncResult:           WorkContactSyncResult{ContactID: 101, ContactWasNew: true},
		workFissionAddContactResult: WorkFissionAddContactResult{
			ParentContactID: 77,
			FissionID:       903,
			InviteCount:     1,
			TotalCount:      1,
			Completed:       true,
			CustomerPush: &WorkFissionCustomerPush{
				Sender:         "go-user",
				ExternalUserID: "external-parent",
				Content: []ContactMessageBatchSendContent{
					{MsgType: "text", Content: "恭喜%NICKNAME%完成任务"},
					{MsgType: "image", PicURL: "push/image.png"},
				},
			},
		},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts: map[string]WorkContactSyncContact{
			"external-user": {
				WXExternalUserID: "external-user",
				Name:             "裂变客户",
				UnionID:          "union-fission",
				FollowUsers:      []WorkContactSyncFollowUser{{UserID: "go-user"}},
			},
		},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default()).
		WithFileStorageRoot("/tmp/mochat-fission-push")

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.add_external_contact",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user", "State": "fission-77"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if client.uploadedImagePath != filepath.Join("/tmp/mochat-fission-push", "push/image.png") {
		t.Fatalf("uploaded image path = %q", client.uploadedImagePath)
	}
	payload := client.contactBatchSendPayload
	if payload.Sender != "go-user" || len(payload.ExternalUserID) != 1 || payload.ExternalUserID[0] != "external-parent" {
		t.Fatalf("batch payload routing = %#v", payload)
	}
	if len(payload.Content) != 2 || payload.Content[0].Content != "恭喜%NICKNAME%完成任务" || payload.Content[1].MediaID != "media-fission-push" {
		t.Fatalf("batch payload content = %#v", payload.Content)
	}
}

func TestWeWorkCallbackWorkerFissionPushRemainsBestEffortButLogsSanitizedFailure(t *testing.T) {
	const secret = "callback-secret-value"
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
		syncResult:           WorkContactSyncResult{ContactID: 101, ContactWasNew: true},
		workFissionAddContactResult: WorkFissionAddContactResult{FissionID: 903, Completed: true, CustomerPush: &WorkFissionCustomerPush{
			Sender: "go-user", ExternalUserID: "external-parent",
			Content: []ContactMessageBatchSendContent{{MsgType: "text", Content: "完成任务"}},
		}},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts: map[string]WorkContactSyncContact{"external-user": {
			WXExternalUserID: "external-user", FollowUsers: []WorkContactSyncFollowUser{{UserID: "go-user"}},
		}},
		contactBatchSendErr: errors.New(`Authorization: Basic ` + secret + `; {"password":"` + secret + `"}`),
	}
	var logs bytes.Buffer
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.New(&logs, "", 0))

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID: 7, EventPath: "event.change_external_contact.add_external_contact",
		Message: map[string]string{"UserID": "go-user", "ExternalUserID": "external-user", "State": "fission-77"},
	})
	if err != nil {
		t.Fatalf("best-effort fission push changed callback outcome: %v", err)
	}
	if got := logs.String(); !strings.Contains(got, "fission add contact skipped") || strings.Contains(got, secret) || strings.Contains(strings.ToLower(got), "access_token=") {
		t.Fatalf("unsafe or missing best-effort failure log: %s", got)
	}
}

func TestSelectWorkRoomAutoPullContactWelcomeContentSkipsFullRooms(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		workRoomAutoPullWelcomes: map[int]WorkRoomAutoPullWelcome{
			77: {
				ID:           77,
				LeadingWords: "入群引导",
				Rooms: []WorkRoomAutoPullWelcomeRoom{
					{RoomID: 9001, MaxNum: 50, RoomMax: 200, MemberNum: 50, RoomQRCodeURL: "room/full-by-config.png"},
					{RoomID: 9002, MaxNum: 80, RoomMax: 80, MemberNum: 80, RoomQRCodeURL: "room/full-by-room.png"},
					{RoomID: 9003, MaxNum: 90, RoomMax: 200, MemberNum: 12, RoomQRCodeURL: "room/available.png"},
				},
			},
		},
	}

	content, found, err := selectWorkRoomAutoPullContactWelcomeContent(context.Background(), store, 77)
	if err != nil {
		t.Fatalf("select error = %v", err)
	}
	if !found || content.Text != "入群引导" || content.Medium == nil || content.Medium.MediumContent["imagePath"] != "room/available.png" {
		t.Fatalf("content = %#v found=%v", content, found)
	}
}

func TestSelectChannelCodeContactWelcomeContentUsesSpecialAndPeriodicRules(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		channelCodeWelcomes: map[int]ChannelCodeWelcome{
			88: {
				ID: 88,
				WelcomeMessage: map[string]any{
					"scanCodePush": 1,
					"messageDetail": []any{
						map[string]any{"type": 1, "welcomeContent": "通用渠道欢迎", "mediumId": 21},
						map[string]any{
							"type":   2,
							"status": 1,
							"detail": []any{
								map[string]any{
									"chooseCycle": []any{6},
									"timeSlot": []any{
										map[string]any{"startTime": "00:00", "endTime": "00:00", "welcomeContent": "周期兜底欢迎"},
									},
								},
							},
						},
						map[string]any{
							"type":   3,
							"status": 1,
							"detail": []any{
								map[string]any{
									"startDate": "2026-07-04",
									"endDate":   "2026-07-04",
									"timeSlot": []any{
										map[string]any{"startTime": "09:00", "endTime": "10:00", "welcomeContent": "特殊时期欢迎", "mediumId": 22},
										map[string]any{"startTime": "00:00", "endTime": "00:00", "welcomeContent": "特殊兜底欢迎"},
									},
								},
							},
						},
					},
				},
			},
			89: {
				ID: 89,
				WelcomeMessage: map[string]any{
					"scanCodePush": 1,
					"messageDetail": []any{
						map[string]any{"type": 1, "welcomeContent": "通用渠道欢迎"},
						map[string]any{
							"type":   2,
							"status": 1,
							"detail": []any{
								map[string]any{
									"chooseCycle": []any{6},
									"timeSlot": []any{
										map[string]any{"startTime": "00:00", "endTime": "00:00", "welcomeContent": "周期兜底欢迎"},
									},
								},
							},
						},
					},
				},
			},
		},
		greetingMedia: map[int]GreetingMedium{
			22: {ID: 22, Type: 2, Content: map[string]any{"imagePath": "image/special.jpg"}},
		},
	}

	special, found, err := selectChannelCodeContactWelcomeContent(context.Background(), store, 88, time.Date(2026, 7, 4, 9, 30, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("special select error = %v", err)
	}
	if !found || special.Text != "特殊时期欢迎" || special.Medium == nil || special.Medium.MediumType != 2 {
		t.Fatalf("special content = %#v found=%v", special, found)
	}

	periodic, found, err := selectChannelCodeContactWelcomeContent(context.Background(), store, 89, time.Date(2026, 7, 4, 11, 30, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("periodic select error = %v", err)
	}
	if !found || periodic.Text != "周期兜底欢迎" || periodic.Medium != nil {
		t.Fatalf("periodic content = %#v found=%v", periodic, found)
	}
}

func TestWeWorkCallbackWorkerSkipsGenericWelcomeWhenAlreadySent(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
		syncResult:           WorkContactSyncResult{ContactID: 101},
		greetings:            []GreetingItem{{ID: 1, CorpID: 7, Words: "通用", RangeType: 1}},
	}
	queue := &fakeWeWorkCallbackWorkerQueue{welcomeStatuses: map[int]int{101: 1}}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts: map[string]WorkContactSyncContact{
			"external-user": {WXExternalUserID: "external-user", Name: "客户A", FollowUsers: []WorkContactSyncFollowUser{{UserID: "go-user"}}},
		},
	}
	worker := NewWeWorkCallbackWorker(fakeWeWorkCallbackWorkerCapabilities(queue), store, client, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.add_external_contact",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user", "WelcomeCode": "welcome-code"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if queue.contactWelcomeEvent.WelcomeCode != "" {
		t.Fatalf("unexpected welcome event = %#v", queue.contactWelcomeEvent)
	}
}

func TestWeWorkCallbackWorkerUpdatesExistingSingleContactFromEvent(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential:           RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees: []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts: map[string]WorkContactSyncContact{
			"external-user": {WXExternalUserID: "external-user", Name: "客户更新", FollowUsers: []WorkContactSyncFollowUser{{UserID: "go-user"}}},
		},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.edit_external_contact",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if store.syncedSingleContactCorpID != 7 || store.syncedSingleContactEmployee.ID != 3 || store.syncedSingleContact.WXExternalUserID != "external-user" || store.syncedSingleContactCreateMissing {
		t.Fatalf("synced single contact = corp %d employee %#v contact %#v create=%v", store.syncedSingleContactCorpID, store.syncedSingleContactEmployee, store.syncedSingleContact, store.syncedSingleContactCreateMissing)
	}
	if len(store.syncedContactBundles) != 0 {
		t.Fatalf("unexpected full contact sync = %#v", store.syncedContactBundles)
	}
}

func TestWeWorkCallbackWorkerRemovesContactRelationFromEvent(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		remindAgent: RoomTagPullAgentCredential{CorpID: 7, WXCorpID: "ww-go", WXAgentID: "100001", WXSecret: "agent-secret"},
		removalResult: WorkContactRemovalResult{
			ContactID:   101,
			ContactName: "删除客户",
			WXUserID:    "go-user",
			Status:      workContactEmployeeStatusRemoved,
		},
	}
	client := &fakeWeWorkCallbackWorkerClient{}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default()).
		WithSidebarBaseURL("https://sidebar.example.com")

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.del_external_contact",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if store.removedContactCorpID != 7 || store.removedContactWXUserID != "go-user" || store.removedContactExternalID != "external-user" || store.removedContactStatus != 2 {
		t.Fatalf("removed contact = corp %d user %q external %q status %d", store.removedContactCorpID, store.removedContactWXUserID, store.removedContactExternalID, store.removedContactStatus)
	}
	if client.agentTextCredential.WXAgentID != "100001" || client.agentTextToUser != "go-user" || !strings.Contains(client.agentTextContent, "【删除提醒】您已将客户删除哦！") || !strings.Contains(client.agentTextContent, "https://sidebar.example.com/contact?agentId={{agentId}}&contactId=101") {
		t.Fatalf("delete reminder = credential:%#v to:%q content:%q", client.agentTextCredential, client.agentTextToUser, client.agentTextContent)
	}
}

func TestWeWorkCallbackWorkerPassivelyRemovesContactRelationFromEvent(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		remindAgent: RoomTagPullAgentCredential{CorpID: 7, WXCorpID: "ww-go", WXAgentID: "100001", WXSecret: "agent-secret"},
		removalResult: WorkContactRemovalResult{
			ContactID:   102,
			ContactName: "流失客户",
			WXUserID:    "go-user",
			Status:      workContactEmployeeStatusPassiveRemoved,
		},
	}
	client := &fakeWeWorkCallbackWorkerClient{}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default()).
		WithSidebarBaseURL("https://sidebar.example.com")

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_contact.del_follow_user",
		Message:   map[string]string{"UserID": "go-user", "ExternalUserID": "external-user"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if store.removedContactCorpID != 7 || store.removedContactWXUserID != "go-user" || store.removedContactExternalID != "external-user" || store.removedContactStatus != 3 {
		t.Fatalf("removed contact = corp %d user %q external %q status %d", store.removedContactCorpID, store.removedContactWXUserID, store.removedContactExternalID, store.removedContactStatus)
	}
	if client.agentTextCredential.WXAgentID != "100001" || client.agentTextToUser != "go-user" || !strings.Contains(client.agentTextContent, "【流失提醒】有客户将您删除哦！") || !strings.Contains(client.agentTextContent, "客户昵称：流失客户") {
		t.Fatalf("passive reminder = credential:%#v to:%q content:%q", client.agentTextCredential, client.agentTextToUser, client.agentTextContent)
	}
}

func TestWeWorkCallbackWorkerSyncsSingleContactTagFromEvent(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		tags: []WorkContactTagSyncGroup{{WXGroupID: "tag-group", GroupName: "标签组", Tags: []WorkContactTagSyncTag{{WXContactTagID: "tag-id", Name: "标签"}}}},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_tag.update",
		Message:   map[string]string{"TagType": "tag", "Id": "tag-id"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if store.syncedTagGroupCorpID != 7 || store.syncedTagGroup.WXGroupID != "tag-group" || len(store.syncedTagGroup.Tags) != 1 || store.syncedTagGroup.Tags[0].WXContactTagID != "tag-id" {
		t.Fatalf("synced tag group = corp %d group %#v", store.syncedTagGroupCorpID, store.syncedTagGroup)
	}
	if len(store.syncedTags) != 0 {
		t.Fatalf("unexpected full tag sync = %#v", store.syncedTags)
	}
	if len(client.tagIDCalls) != 1 || client.tagIDCalls[0] != "tag-id" {
		t.Fatalf("tag id calls = %#v", client.tagIDCalls)
	}
}

func TestWeWorkCallbackWorkerDeletesContactTagGroupFromEvent(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, &fakeWeWorkCallbackWorkerClient{}, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_tag.delete",
		Message:   map[string]string{"TagType": "tag_group", "Id": "tag-group"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if store.deletedTagGroupCorpID != 7 || store.deletedTagGroupWXID != "tag-group" {
		t.Fatalf("deleted tag group = corp %d id %q", store.deletedTagGroupCorpID, store.deletedTagGroupWXID)
	}
	if len(store.syncedTags) != 0 {
		t.Fatalf("unexpected full tag sync = %#v", store.syncedTags)
	}
}

func TestWeWorkCallbackWorkerSyncsSingleRoomFromEvent(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		groupChats: []WorkRoomSyncGroupChat{{WXChatID: "room-1", Status: 1}, {WXChatID: "room-2", Status: 0}},
		rooms: map[string]WorkRoomSyncRoom{
			"room-1": {WXChatID: "room-1", Name: "客户群1"},
			"room-2": {WXChatID: "room-2", Name: "客户群2"},
		},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default())

	if err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_chat.update",
		Message:   map[string]string{"ChatId": "room-1"},
	}); err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if len(store.syncedRooms) != 1 || store.syncedRooms[0].WXChatID != "room-1" || store.syncedRooms[0].Status != 1 {
		t.Fatalf("synced rooms = %#v", store.syncedRooms)
	}
	if client.detailCalls != 1 || client.detailChatIDs[0] != "room-1" {
		t.Fatalf("detail calls = %d ids=%#v", client.detailCalls, client.detailChatIDs)
	}
	if store.roomJoinTaskCorpID != 7 || store.roomJoinTaskWXChatID != "room-1" {
		t.Fatalf("room join auto tag task = corp %d chat %q", store.roomJoinTaskCorpID, store.roomJoinTaskWXChatID)
	}
}

func TestWeWorkCallbackWorkerSyncsCreatedRoomFromEvent(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		groupChats: []WorkRoomSyncGroupChat{{WXChatID: "room-1", Status: 1}},
		rooms: map[string]WorkRoomSyncRoom{
			"room-1": {WXChatID: "room-1", Name: "新客户群"},
		},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default())

	if err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_chat.create",
		Message:   map[string]string{"ChatId": "room-1"},
	}); err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if len(store.syncedRooms) != 1 || store.syncedRooms[0].WXChatID != "room-1" || store.syncedRooms[0].Name != "新客户群" || store.syncedRooms[0].Status != 1 {
		t.Fatalf("synced rooms = %#v", store.syncedRooms)
	}
	if client.detailCalls != 1 || client.detailChatIDs[0] != "room-1" {
		t.Fatalf("detail calls = %d ids=%#v", client.detailCalls, client.detailChatIDs)
	}
	if store.roomJoinTaskCorpID != 7 || store.roomJoinTaskWXChatID != "room-1" {
		t.Fatalf("room join auto tag task = corp %d chat %q", store.roomJoinTaskCorpID, store.roomJoinTaskWXChatID)
	}
}

func TestWeWorkCallbackWorkerEnqueuesRoomJoinAutoTagFromRoomEvent(t *testing.T) {
	queue := &fakeWeWorkCallbackWorkerQueue{}
	store := &fakeWeWorkCallbackWorkerStore{
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		roomJoinTaskResult: AutoTagRoomJoinTaskResult{
			MarkTagsEvents: []MarkTagsEvent{{
				CorpID:          7,
				ContactID:       101,
				EmployeeID:      3,
				TagIDs:          []int{8, 9},
				Source:          "auto-tag-join-room:77",
				AutoTagID:       66,
				AutoTagRecordID: 77,
			}},
		},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		groupChats: []WorkRoomSyncGroupChat{{WXChatID: "room-1", Status: 1}},
		rooms: map[string]WorkRoomSyncRoom{
			"room-1": {WXChatID: "room-1", Name: "客户群1"},
		},
	}
	worker := NewWeWorkCallbackWorker(fakeWeWorkCallbackWorkerCapabilities(queue), store, client, "worker-secret", log.Default())

	if err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_chat.update",
		Message:   map[string]string{"ChatId": "room-1"},
	}); err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if store.roomJoinTaskCorpID != 7 || store.roomJoinTaskWXChatID != "room-1" || store.roomJoinTaskCalls != 1 {
		t.Fatalf("room join auto tag task = corp %d chat %q calls %d", store.roomJoinTaskCorpID, store.roomJoinTaskWXChatID, store.roomJoinTaskCalls)
	}
	if len(queue.markTagsEvents) != 1 {
		t.Fatalf("mark tag events = %#v", queue.markTagsEvents)
	}
	event := queue.markTagsEvents[0]
	if event.ContactID != 101 || event.EmployeeID != 3 || event.AutoTagRecordID != 77 || len(event.TagIDs) != 2 || event.TagIDs[0] != 8 || event.TagIDs[1] != 9 {
		t.Fatalf("mark tag event = %#v", event)
	}
}

func TestWeWorkCallbackWorkerGeneratesRoomJoinSOPLogFromRoomEvent(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 30, 0, 0, time.Local)
	store := &fakeWeWorkCallbackWorkerStore{
		credential:        RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		roomIDsByWXChatID: map[string]int{"room-1": 99},
		roomSOPSources: []RoomSOPLogSource{{
			ID:         77,
			CorpID:     7,
			SettingRaw: `{"content":[{"type":"text","value":"回调入群后SOP"}],"targetAnchor":"room_join","delayMinutes":0}`,
			RoomIDsRaw: `[99]`,
		}},
		roomSOPTargetsByAnchor: map[string][]RoomSOPLogTarget{
			SOPLogTargetAnchorRoomJoin: {{
				RoomID:                99,
				EmployeeWXUserID:      "go-user",
				ContactWXExternalUser: "external-room",
				AnchorTime:            now,
			}},
		},
	}
	client := &fakeWeWorkCallbackWorkerClient{
		groupChats: []WorkRoomSyncGroupChat{{WXChatID: "room-1", Status: 1}},
		rooms: map[string]WorkRoomSyncRoom{
			"room-1": {WXChatID: "room-1", Name: "客户群1"},
		},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, client, "worker-secret", log.Default()).WithNow(func() time.Time { return now })

	if err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_chat.update",
		Message:   map[string]string{"ChatId": "room-1"},
	}); err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if len(store.roomSOPLogs) != 1 {
		t.Fatalf("room SOP logs = %#v", store.roomSOPLogs)
	}
	logItem := store.roomSOPLogs[0]
	if logItem.RoomSOPID != 77 || logItem.RoomID != 99 || logItem.EmployeeWXUserID != "go-user" || logItem.ContactWXExternalUser != "external-room" || !strings.Contains(logItem.TaskRaw, "回调入群后SOP") || !logItem.CreatedAt.Equal(now) {
		t.Fatalf("room SOP log = %#v", logItem)
	}
}

func TestWeWorkCallbackWorkerDeletesDismissedRoom(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, &fakeWeWorkCallbackWorkerClient{}, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_external_chat.dismiss",
		Message:   map[string]string{"ChatId": "room-dismissed"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if store.deletedRoomCorpID != 7 || store.deletedRoomWXChatID != "room-dismissed" {
		t.Fatalf("deleted room = corp %d chat %q", store.deletedRoomCorpID, store.deletedRoomWXChatID)
	}
}

func TestWeWorkCallbackWorkerDeletesEmployeeFromEvent(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, &fakeWeWorkCallbackWorkerClient{}, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_contact.delete_user",
		Message:   map[string]string{"UserID": "go-user"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if store.deletedEmployeeCorpID != 7 || store.deletedEmployeeWXUserID != "go-user" {
		t.Fatalf("deleted employee = corp %d user %q", store.deletedEmployeeCorpID, store.deletedEmployeeWXUserID)
	}
	if len(store.syncedEmployees) != 0 || len(store.syncedDepartments) != 0 {
		t.Fatalf("unexpected full employee sync departments=%#v employees=%#v", store.syncedDepartments, store.syncedEmployees)
	}
}

func TestWeWorkCallbackWorkerSyncsDepartmentFromEvent(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, &fakeWeWorkCallbackWorkerClient{}, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_contact.create_party",
		Message:   map[string]string{"Id": "8", "Name": "销售二部", "ParentId": "2", "Order": "80"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if store.syncedDepartmentCorpID != 7 || store.syncedDepartment.WXDepartmentID != 8 || store.syncedDepartment.Name != "销售二部" || store.syncedDepartment.WXParentID != 2 || store.syncedDepartment.Order != 80 {
		t.Fatalf("synced department = corp %d department %#v", store.syncedDepartmentCorpID, store.syncedDepartment)
	}
	if !store.syncedDepartment.HasName || !store.syncedDepartment.HasParent || !store.syncedDepartment.HasOrder {
		t.Fatalf("department field flags = %#v", store.syncedDepartment)
	}
	if len(store.syncedEmployees) != 0 || len(store.syncedDepartments) != 0 {
		t.Fatalf("unexpected full employee sync departments=%#v employees=%#v", store.syncedDepartments, store.syncedEmployees)
	}
}

func TestWeWorkCallbackWorkerDeletesDepartmentFromEvent(t *testing.T) {
	store := &fakeWeWorkCallbackWorkerStore{}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, &fakeWeWorkCallbackWorkerClient{}, "worker-secret", log.Default())

	err := worker.Process(context.Background(), WeWorkCallbackEvent{
		CorpID:    7,
		EventPath: "event.change_contact.delete_party",
		Message:   map[string]string{"Id": "8"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if store.deletedDepartmentCorpID != 7 || store.deletedDepartmentWXDepartmentID != 8 {
		t.Fatalf("deleted department = corp %d department %d", store.deletedDepartmentCorpID, store.deletedDepartmentWXDepartmentID)
	}
	if len(store.syncedEmployees) != 0 || len(store.syncedDepartments) != 0 {
		t.Fatalf("unexpected full employee sync departments=%#v employees=%#v", store.syncedDepartments, store.syncedEmployees)
	}
}

func TestWeWorkCallbackWorkerCompletesSuccessfulDurableClaim(t *testing.T) {
	store := &fakeDurableWeWorkCallbackWorkerStore{fakeWeWorkCallbackWorkerStore: &fakeWeWorkCallbackWorkerStore{}}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, &fakeWeWorkCallbackWorkerClient{}, "worker-secret", log.Default())

	worker.handleClaim(context.Background(), WeWorkCallbackClaim{
		ID: 1, EventKey: strings.Repeat("d", 64), LeaseToken: "lease", LeaseFence: 1, Attempt: 1,
		Event: WeWorkCallbackEvent{
			CorpID:    7,
			EventPath: "event.change_external_chat.dismiss",
			Message:   map[string]string{"ChatId": "room-dismissed"},
		},
	})

	if len(store.completedClaims) != 1 || len(store.failedClaims) != 0 {
		t.Fatalf("completed=%+v failed=%+v", store.completedClaims, store.failedClaims)
	}
	if !store.fakeWeWorkCallbackWorkerStore.callbackExecutionFound || store.fakeWeWorkCallbackWorkerStore.callbackExecution.EventKey != strings.Repeat("d", 64) || store.fakeWeWorkCallbackWorkerStore.callbackExecution.LeaseFence != 1 {
		t.Fatalf("side effect callback execution=%+v found=%t", store.fakeWeWorkCallbackWorkerStore.callbackExecution, store.fakeWeWorkCallbackWorkerStore.callbackExecutionFound)
	}
}

func TestWeWorkCallbackWorkerRejectsStaleFenceBeforeSideEffects(t *testing.T) {
	eventKey := strings.Repeat("9", 64)
	workerStore := &fakeWeWorkCallbackWorkerStore{}
	store := &fakeDurableWeWorkCallbackWorkerStore{
		fakeWeWorkCallbackWorkerStore: workerStore,
		currentFences:                 map[string]uint64{eventKey: 2},
	}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, &fakeWeWorkCallbackWorkerClient{}, "worker-secret", log.Default())
	event := WeWorkCallbackEvent{TenantID: 3, CorpID: 7, EventPath: "event.change_external_chat.dismiss", Message: map[string]string{"ChatId": "room-dismissed"}}

	worker.handleClaim(context.Background(), WeWorkCallbackClaim{ID: 6, EventKey: eventKey, LeaseToken: "old", LeaseFence: 1, Attempt: 1, Event: event})
	worker.handleClaim(context.Background(), WeWorkCallbackClaim{ID: 6, EventKey: eventKey, LeaseToken: "new", LeaseFence: 2, Attempt: 2, Event: event})

	if workerStore.deletedRoomCalls != 1 {
		t.Fatalf("side effects=%d, want one current-fence execution", workerStore.deletedRoomCalls)
	}
}

func TestWeWorkCallbackWorkerRecordsQueueItemExecution(t *testing.T) {
	store := &fakeDurableWeWorkCallbackWorkerStore{fakeWeWorkCallbackWorkerStore: &fakeWeWorkCallbackWorkerStore{tenantIDs: map[int]int{7: 21}}}
	recorder := &fakeWorkerExecutionRecorder{}
	ctx := taskrunner.WithTaskRuntime(context.Background(), "wework-callback", "run-wework-1", recorder)
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, &fakeWeWorkCallbackWorkerClient{}, "worker-secret", log.Default())

	worker.handleClaim(ctx, WeWorkCallbackClaim{
		ID: 2, EventKey: strings.Repeat("e", 64), LeaseToken: "lease", LeaseFence: 1, Attempt: 1,
		Event: WeWorkCallbackEvent{
			CorpID:    7,
			EventPath: "event.change_external_chat.dismiss",
			Message:   map[string]string{"ChatId": "room-dismissed"},
		},
	})

	running := recordedExecutionByStatus(t, recorder, "wework-callback", taskrunner.StatusRunning)
	succeeded := recordedExecutionByStatus(t, recorder, "wework-callback", taskrunner.StatusSucceeded)
	if running.ExecutionID == "" || running.TenantID != 21 || succeeded.TenantID != 21 || succeeded.ExecutionID != running.ExecutionID || succeeded.RunID != "run-wework-1" || succeeded.StoppedAt == "" || succeeded.Error != "" {
		t.Fatalf("executions = %+v", recorder.executions)
	}
}

func TestWeWorkCallbackWorkerRetriesFailedDurableClaim(t *testing.T) {
	store := &fakeDurableWeWorkCallbackWorkerStore{fakeWeWorkCallbackWorkerStore: &fakeWeWorkCallbackWorkerStore{}}
	worker := NewWeWorkCallbackWorker(WeWorkCallbackWorkerCapabilities{}, store, &fakeWeWorkCallbackWorkerClient{}, "worker-secret", log.Default())

	worker.handleClaim(context.Background(), WeWorkCallbackClaim{ID: 3, EventKey: strings.Repeat("f", 64), LeaseToken: "lease", LeaseFence: 2, Attempt: 2, Event: WeWorkCallbackEvent{EventPath: "event.change_contact.create_user"}})

	if len(store.failedClaims) != 1 || len(store.completedClaims) != 0 {
		t.Fatalf("failed=%+v completed=%+v", store.failedClaims, store.completedClaims)
	}
}

func TestWeWorkCallbackWorkerRedactsCredentialBearingProviderErrorsEverywhere(t *testing.T) {
	const secret = "callback-secret-value"
	queue := &fakeWeWorkCallbackWorkerQueue{}
	workerStore := &fakeWeWorkCallbackWorkerStore{
		tenantIDs:                map[int]int{7: 21},
		credential:               RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		contactSyncEmployees:     []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
		syncResult:               WorkContactSyncResult{ContactID: 101},
		workRoomAutoPullWelcomes: map[int]WorkRoomAutoPullWelcome{77: {ID: 77, TagIDs: []int{31}}},
		updateProfileFound:       true,
		updateProfileResult:      WorkContactUpdateResult{WXUserID: "go-user", WXExternalUserID: "external-user", AddedWXTagIDs: []string{"wx-tag-31"}},
	}
	providerErr := &url.Error{
		Op:  "POST",
		URL: "https://qyapi.weixin.qq.com/cgi-bin/externalcontact/mark_tag?access_token=" + secret,
		Err: errors.New(`timeout Authorization: Bearer ` + secret + `; {"access_token":"` + secret + `"}; secret=multi word ` + secret),
	}
	client := &fakeWeWorkCallbackWorkerClient{
		contacts:    map[string]WorkContactSyncContact{"external-user": {WXExternalUserID: "external-user", FollowUsers: []WorkContactSyncFollowUser{{UserID: "go-user"}}}},
		markTagsErr: providerErr,
	}
	store := &fakeDurableWeWorkCallbackWorkerStore{fakeWeWorkCallbackWorkerStore: workerStore}
	recorder := &fakeWorkerExecutionRecorder{}
	ctx := taskrunner.WithTaskRuntime(context.Background(), "wework-callback", "run-safe-error", recorder)
	var logs bytes.Buffer
	worker := NewWeWorkCallbackWorker(fakeWeWorkCallbackWorkerCapabilities(queue), store, client, "worker-secret", log.New(&logs, "", 0))

	worker.handleClaim(ctx, WeWorkCallbackClaim{
		ID: 5, EventKey: strings.Repeat("2", 64), LeaseToken: "lease", LeaseFence: 3, Attempt: 1,
		Event: WeWorkCallbackEvent{
			TenantID: 21, CorpID: 7, EventPath: "event.change_external_contact.add_external_contact",
			Message: map[string]string{"UserID": "go-user", "ExternalUserID": "external-user", "State": "workRoomAutoPullId-77"},
		},
	})

	combined := strings.Join(store.failedReasons, "\n") + "\n" + logs.String()
	for _, execution := range recorder.executions {
		combined += "\n" + execution.Error
	}
	if strings.Contains(combined, secret) || strings.Contains(strings.ToLower(combined), "access_token=") {
		t.Fatalf("credential-bearing provider error escaped redaction: %s", combined)
	}
	if len(store.failedReasons) != 1 || !strings.Contains(store.failedReasons[0], "timeout") {
		t.Fatalf("failure reason=%q", store.failedReasons)
	}
}

func TestWeWorkCallbackWorkerRetriesContactTagSyncFailuresThroughDeliveryChain(t *testing.T) {
	for _, tc := range []struct {
		name      string
		result    WorkContactUpdateResult
		clientErr error
	}{
		{
			name:   "unmapped",
			result: WorkContactUpdateResult{WXUserID: "go-user", WXExternalUserID: "external-user", TagSyncRequested: true, UnsyncableTagIDs: []int{31}},
		},
		{
			name:   "mixed mapping",
			result: WorkContactUpdateResult{WXUserID: "go-user", WXExternalUserID: "external-user", TagSyncRequested: true, AddedWXTagIDs: []string{"wx-tag-31"}, UnsyncableTagIDs: []int{32}},
		},
		{
			name:      "remote failure",
			result:    WorkContactUpdateResult{WXUserID: "go-user", WXExternalUserID: "external-user", TagSyncRequested: true, AddedWXTagIDs: []string{"wx-tag-31"}},
			clientErr: fmt.Errorf("wecom unavailable"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queue := &fakeWeWorkCallbackWorkerQueue{}
			workerStore := &fakeWeWorkCallbackWorkerStore{
				credential:               RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
				contactSyncEmployees:     []WorkContactSyncEmployee{{ID: 3, WXUserID: "go-user"}},
				syncResult:               WorkContactSyncResult{ContactID: 101},
				workRoomAutoPullWelcomes: map[int]WorkRoomAutoPullWelcome{77: {ID: 77, TagIDs: []int{31, 32}}},
				updateProfileFound:       true,
				updateProfileResult:      tc.result,
			}
			client := &fakeWeWorkCallbackWorkerClient{
				contacts:    map[string]WorkContactSyncContact{"external-user": {WXExternalUserID: "external-user", FollowUsers: []WorkContactSyncFollowUser{{UserID: "go-user"}}}},
				markTagsErr: tc.clientErr,
			}
			store := &fakeDurableWeWorkCallbackWorkerStore{fakeWeWorkCallbackWorkerStore: workerStore}
			worker := NewWeWorkCallbackWorker(fakeWeWorkCallbackWorkerCapabilities(queue), store, client, "worker-secret", log.Default())

			worker.handleClaim(context.Background(), WeWorkCallbackClaim{
				ID: 4, EventKey: strings.Repeat("1", 64), LeaseToken: "lease", LeaseFence: 1, Attempt: 1,
				Event: WeWorkCallbackEvent{
					CorpID: 7, EventPath: "event.change_external_contact.add_external_contact",
					Message: map[string]string{"UserID": "go-user", "ExternalUserID": "external-user", "State": "workRoomAutoPullId-77"},
				},
			})

			if len(store.failedClaims) != 1 || len(store.completedClaims) != 0 {
				t.Fatalf("failed=%+v completed=%+v", store.failedClaims, store.completedClaims)
			}
		})
	}
}

type fakeWeWorkCallbackWorkerStore struct {
	tenantIDs                map[int]int
	employeeCredentials      []WorkEmployeeSyncCredential
	credential               RoomWelcomeCorpCredential
	contactSyncEmployees     []WorkContactSyncEmployee
	syncResult               WorkContactSyncResult
	greetings                []GreetingItem
	greetingMedia            map[int]GreetingMedium
	channelCodeWelcomes      map[int]ChannelCodeWelcome
	workRoomAutoPullWelcomes map[int]WorkRoomAutoPullWelcome
	workFissionRules         map[int]WorkFissionContactRule
	workFissionWelcomes      map[int]WorkFissionContactWelcome
	remindAgent              RoomTagPullAgentCredential
	updateProfileResult      WorkContactUpdateResult
	updateProfileFound       bool

	syncedDepartments                []WorkEmployeeSyncDepartment
	syncedEmployees                  []WorkEmployeeSyncEmployee
	syncedTags                       []WorkContactTagSyncGroup
	syncedTagGroupCorpID             int
	syncedTagGroup                   WorkContactTagSyncGroup
	syncedContactBundles             []WorkContactSyncEmployeeContacts
	syncedSingleContactCorpID        int
	syncedSingleContactEmployee      WorkContactSyncEmployee
	syncedSingleContact              WorkContactSyncContact
	syncedSingleContactCreateMissing bool
	removedContactCorpID             int
	removedContactWXUserID           string
	removedContactExternalID         string
	removedContactStatus             int
	removalResult                    WorkContactRemovalResult
	syncedRooms                      []WorkRoomSyncRoom
	deletedRoomCorpID                int
	deletedRoomWXChatID              string
	deletedRoomCalls                 int
	deletedEmployeeCorpID            int
	deletedEmployeeWXUserID          string
	syncedDepartmentCorpID           int
	syncedDepartment                 WorkDepartmentEventDepartment
	deletedDepartmentCorpID          int
	deletedDepartmentWXDepartmentID  int
	deletedTagCorpID                 int
	deletedTagWXID                   string
	deletedTagGroupCorpID            int
	deletedTagGroupWXID              string
	updatedProfileValues             WorkContactUpdateValues
	updatedProfileCalls              int
	workFissionAddContactCalls       int
	workFissionAddContactEvent       WorkFissionAddContactEvent
	workFissionAddContactResult      WorkFissionAddContactResult
	roomJoinTaskResult               AutoTagRoomJoinTaskResult
	roomJoinTaskCorpID               int
	roomJoinTaskWXChatID             string
	roomJoinTaskCalls                int
	contactTimeTaskResult            AutoTagContactTimeTaskResult
	contactTimeTaskCorpID            int
	contactTimeTaskEmployeeID        int
	contactTimeTaskContactID         int
	contactTimeTaskCalls             int
	contactSOPSources                []ContactSOPLogSource
	roomSOPSources                   []RoomSOPLogSource
	contactSOPTargets                []ContactSOPLogTarget
	roomSOPTargets                   []RoomSOPLogTarget
	roomSOPTargetsByAnchor           map[string][]RoomSOPLogTarget
	roomIDsByWXChatID                map[string]int
	contactSOPLogs                   []ContactSOPLogCreate
	roomSOPLogs                      []RoomSOPLogCreate
	contactSOPLogKeys                map[string]struct{}
	roomSOPLogKeys                   map[string]struct{}
	callbackExecution                WeWorkCallbackExecution
	callbackExecutionFound           bool
}

func (s *fakeWeWorkCallbackWorkerStore) TenantIDByCorpID(_ context.Context, corpID int) (int, error) {
	return s.tenantIDs[corpID], nil
}

func (s *fakeWeWorkCallbackWorkerStore) ActiveContactSOPLogSources(context.Context) ([]ContactSOPLogSource, error) {
	return append([]ContactSOPLogSource{}, s.contactSOPSources...), nil
}

func (s *fakeWeWorkCallbackWorkerStore) ActiveRoomSOPLogSources(context.Context) ([]RoomSOPLogSource, error) {
	return append([]RoomSOPLogSource{}, s.roomSOPSources...), nil
}

func (s *fakeWeWorkCallbackWorkerStore) ContactSOPLogTargets(_ context.Context, _ int, _ []int, _ []int) ([]ContactSOPLogTarget, error) {
	return append([]ContactSOPLogTarget{}, s.contactSOPTargets...), nil
}

func (s *fakeWeWorkCallbackWorkerStore) RoomSOPLogTargets(_ context.Context, _ int, _ []int, targetAnchor string) ([]RoomSOPLogTarget, error) {
	if s.roomSOPTargetsByAnchor != nil {
		if targets, ok := s.roomSOPTargetsByAnchor[targetAnchor]; ok {
			return append([]RoomSOPLogTarget{}, targets...), nil
		}
	}
	return append([]RoomSOPLogTarget{}, s.roomSOPTargets...), nil
}

func (s *fakeWeWorkCallbackWorkerStore) RoomIDByWXChatID(_ context.Context, _ int, wxChatID string) (int, bool, error) {
	if s.roomIDsByWXChatID == nil {
		return 0, false, nil
	}
	id, ok := s.roomIDsByWXChatID[wxChatID]
	return id, ok && id > 0, nil
}

func (s *fakeWeWorkCallbackWorkerStore) InsertContactSOPLog(_ context.Context, item ContactSOPLogCreate) (bool, error) {
	if s.contactSOPLogKeys == nil {
		s.contactSOPLogKeys = map[string]struct{}{}
	}
	key := strings.Join([]string{item.EmployeeWXUserID, item.ContactWXExternalUser, item.TaskRaw}, "\x00")
	if _, ok := s.contactSOPLogKeys[key]; ok {
		return false, nil
	}
	s.contactSOPLogKeys[key] = struct{}{}
	s.contactSOPLogs = append(s.contactSOPLogs, item)
	return true, nil
}

func (s *fakeWeWorkCallbackWorkerStore) InsertRoomSOPLog(_ context.Context, item RoomSOPLogCreate) (bool, error) {
	if s.roomSOPLogKeys == nil {
		s.roomSOPLogKeys = map[string]struct{}{}
	}
	key := strings.Join([]string{item.EmployeeWXUserID, item.ContactWXExternalUser, item.TaskRaw}, "\x00")
	if _, ok := s.roomSOPLogKeys[key]; ok {
		return false, nil
	}
	s.roomSOPLogKeys[key] = struct{}{}
	s.roomSOPLogs = append(s.roomSOPLogs, item)
	return true, nil
}

func (s *fakeWeWorkCallbackWorkerStore) WorkEmployeeSyncCredentials(_ context.Context, _ []int) ([]WorkEmployeeSyncCredential, error) {
	return append([]WorkEmployeeSyncCredential{}, s.employeeCredentials...), nil
}

func (s *fakeWeWorkCallbackWorkerStore) SyncWorkEmployees(_ context.Context, _ WorkEmployeeSyncCredential, departments []WorkEmployeeSyncDepartment, employees []WorkEmployeeSyncEmployee, _ []string, _ string) (WorkEmployeeSyncResult, error) {
	s.syncedDepartments = append([]WorkEmployeeSyncDepartment{}, departments...)
	s.syncedEmployees = append([]WorkEmployeeSyncEmployee{}, employees...)
	return WorkEmployeeSyncResult{}, nil
}

func (s *fakeWeWorkCallbackWorkerStore) RoomWelcomeCorpCredentialByID(_ context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error) {
	if s.credential.CorpID == 0 {
		s.credential.CorpID = corpID
	}
	return s.credential, s.credential.WXCorpID != "", nil
}

func (s *fakeWeWorkCallbackWorkerStore) WorkContactSyncEmployees(_ context.Context, _ int) ([]WorkContactSyncEmployee, error) {
	return append([]WorkContactSyncEmployee{}, s.contactSyncEmployees...), nil
}

func (s *fakeWeWorkCallbackWorkerStore) WorkContactSyncEmployeeByWXUserID(_ context.Context, _ int, wxUserID string) (WorkContactSyncEmployee, bool, error) {
	for _, employee := range s.contactSyncEmployees {
		if employee.WXUserID == wxUserID {
			return employee, true, nil
		}
	}
	return WorkContactSyncEmployee{}, false, nil
}

func (s *fakeWeWorkCallbackWorkerStore) SyncWorkContacts(_ context.Context, _ int, bundles []WorkContactSyncEmployeeContacts) (WorkContactSyncResult, error) {
	s.syncedContactBundles = append([]WorkContactSyncEmployeeContacts{}, bundles...)
	return WorkContactSyncResult{}, nil
}

func (s *fakeWeWorkCallbackWorkerStore) SyncWorkContactForEmployee(_ context.Context, corpID int, employee WorkContactSyncEmployee, contact WorkContactSyncContact, createMissing bool) (WorkContactSyncResult, error) {
	s.syncedSingleContactCorpID = corpID
	s.syncedSingleContactEmployee = employee
	s.syncedSingleContact = contact
	s.syncedSingleContactCreateMissing = createMissing
	if s.syncResult.ContactID == 0 {
		s.syncResult.ContactID = 101
	}
	return s.syncResult, nil
}

func (s *fakeWeWorkCallbackWorkerStore) GreetingsByCorp(_ context.Context, _ int) ([]GreetingItem, error) {
	return append([]GreetingItem{}, s.greetings...), nil
}

func (s *fakeWeWorkCallbackWorkerStore) GreetingMediaByIDs(_ context.Context, mediumIDs []int) (map[int]GreetingMedium, error) {
	result := make(map[int]GreetingMedium, len(mediumIDs))
	for _, id := range mediumIDs {
		if medium, ok := s.greetingMedia[id]; ok {
			result[id] = medium
		}
	}
	return result, nil
}

func (s *fakeWeWorkCallbackWorkerStore) ChannelCodeWelcomeByID(_ context.Context, channelCodeID int) (ChannelCodeWelcome, bool, error) {
	welcome, ok := s.channelCodeWelcomes[channelCodeID]
	return welcome, ok, nil
}

func (s *fakeWeWorkCallbackWorkerStore) WorkRoomAutoPullWelcomeByID(_ context.Context, id int) (WorkRoomAutoPullWelcome, bool, error) {
	welcome, ok := s.workRoomAutoPullWelcomes[id]
	return welcome, ok, nil
}

func (s *fakeWeWorkCallbackWorkerStore) WorkFissionContactRuleByID(_ context.Context, id int) (WorkFissionContactRule, bool, error) {
	rule, ok := s.workFissionRules[id]
	return rule, ok, nil
}

func (s *fakeWeWorkCallbackWorkerStore) WorkFissionContactWelcomeByParentID(_ context.Context, parentContactID int) (WorkFissionContactWelcome, bool, error) {
	welcome, ok := s.workFissionWelcomes[parentContactID]
	return welcome, ok, nil
}

func (s *fakeWeWorkCallbackWorkerStore) HandleWorkFissionAddContact(_ context.Context, event WorkFissionAddContactEvent) (WorkFissionAddContactResult, bool, error) {
	s.workFissionAddContactCalls++
	s.workFissionAddContactEvent = event
	if s.workFissionAddContactResult.FissionID > 0 || s.workFissionAddContactResult.EmployeeReminder != nil {
		return s.workFissionAddContactResult, true, nil
	}
	return WorkFissionAddContactResult{ParentContactID: event.ParentContactID, FissionID: 903, InviteCount: 1, TotalCount: 1, Completed: true}, true, nil
}

func (s *fakeWeWorkCallbackWorkerStore) RoomTagPullRemindAgentByCorpID(_ context.Context, corpID int) (RoomTagPullAgentCredential, bool, error) {
	if s.remindAgent.CorpID == 0 {
		s.remindAgent.CorpID = corpID
	}
	return s.remindAgent, s.remindAgent.WXCorpID != "", nil
}

func (s *fakeWeWorkCallbackWorkerStore) UpdateWorkContactProfile(_ context.Context, values WorkContactUpdateValues) (WorkContactUpdateResult, bool, error) {
	s.updatedProfileCalls++
	s.updatedProfileValues = values
	found := s.updateProfileFound
	if !found && s.updateProfileResult.WXUserID == "" && s.updateProfileResult.WXExternalUserID == "" && len(s.updateProfileResult.AddedWXTagIDs) == 0 {
		found = true
	}
	return s.updateProfileResult, found, nil
}

func (s *fakeWeWorkCallbackWorkerStore) RemoveWorkContactEmployeeRelation(_ context.Context, corpID int, wxUserID string, wxExternalUserID string, status int) (WorkContactRemovalResult, bool, error) {
	s.removedContactCorpID = corpID
	s.removedContactWXUserID = wxUserID
	s.removedContactExternalID = wxExternalUserID
	s.removedContactStatus = status
	result := s.removalResult
	if result.ContactID == 0 {
		result = WorkContactRemovalResult{ContactID: 101, ContactName: "删除客户", WXUserID: wxUserID, Status: status}
	}
	return result, true, nil
}

func (s *fakeWeWorkCallbackWorkerStore) SyncWorkContactTags(_ context.Context, _ int, groups []WorkContactTagSyncGroup) (WorkContactTagSyncResult, error) {
	s.syncedTags = append([]WorkContactTagSyncGroup{}, groups...)
	return WorkContactTagSyncResult{}, nil
}

func (s *fakeWeWorkCallbackWorkerStore) SyncWorkContactTagGroup(_ context.Context, corpID int, group WorkContactTagSyncGroup) (WorkContactTagSyncResult, error) {
	s.syncedTagGroupCorpID = corpID
	s.syncedTagGroup = group
	return WorkContactTagSyncResult{}, nil
}

func (s *fakeWeWorkCallbackWorkerStore) DeleteWorkContactTagByWXContactTagID(_ context.Context, corpID int, wxContactTagID string) (bool, error) {
	s.deletedTagCorpID = corpID
	s.deletedTagWXID = wxContactTagID
	return true, nil
}

func (s *fakeWeWorkCallbackWorkerStore) DeleteWorkContactTagGroupByWXGroupID(_ context.Context, corpID int, wxGroupID string) (bool, error) {
	s.deletedTagGroupCorpID = corpID
	s.deletedTagGroupWXID = wxGroupID
	return true, nil
}

func (s *fakeWeWorkCallbackWorkerStore) SyncWorkRooms(_ context.Context, _ int, rooms []WorkRoomSyncRoom) (WorkRoomSyncResult, error) {
	s.syncedRooms = append([]WorkRoomSyncRoom{}, rooms...)
	return WorkRoomSyncResult{}, nil
}

func (s *fakeWeWorkCallbackWorkerStore) SyncWorkRoom(_ context.Context, _ int, room WorkRoomSyncRoom) (WorkRoomSyncResult, error) {
	s.syncedRooms = []WorkRoomSyncRoom{room}
	return WorkRoomSyncResult{}, nil
}

func (s *fakeWeWorkCallbackWorkerStore) AutoTagRoomJoinTask(_ context.Context, corpID int, wxChatID string) (AutoTagRoomJoinTaskResult, error) {
	s.roomJoinTaskCorpID = corpID
	s.roomJoinTaskWXChatID = wxChatID
	s.roomJoinTaskCalls++
	return s.roomJoinTaskResult, nil
}

func (s *fakeWeWorkCallbackWorkerStore) AutoTagContactTimeTask(_ context.Context, corpID int, employeeID int, contactID int) (AutoTagContactTimeTaskResult, error) {
	s.contactTimeTaskCorpID = corpID
	s.contactTimeTaskEmployeeID = employeeID
	s.contactTimeTaskContactID = contactID
	s.contactTimeTaskCalls++
	return s.contactTimeTaskResult, nil
}

func (s *fakeWeWorkCallbackWorkerStore) DeleteWorkRoomByWXChatID(ctx context.Context, corpID int, wxChatID string) (bool, error) {
	s.callbackExecution, s.callbackExecutionFound = WeWorkCallbackExecutionFromContext(ctx)
	s.deletedRoomCalls++
	s.deletedRoomCorpID = corpID
	s.deletedRoomWXChatID = wxChatID
	return true, nil
}

func (s *fakeWeWorkCallbackWorkerStore) DeleteWorkEmployeeByWXUserID(_ context.Context, corpID int, wxUserID string) (bool, error) {
	s.deletedEmployeeCorpID = corpID
	s.deletedEmployeeWXUserID = wxUserID
	return true, nil
}

func (s *fakeWeWorkCallbackWorkerStore) SyncWorkDepartment(_ context.Context, corpID int, department WorkDepartmentEventDepartment) (WorkEmployeeSyncResult, error) {
	s.syncedDepartmentCorpID = corpID
	s.syncedDepartment = department
	return WorkEmployeeSyncResult{}, nil
}

func (s *fakeWeWorkCallbackWorkerStore) DeleteWorkDepartmentByWXDepartmentID(_ context.Context, corpID int, wxDepartmentID int) (bool, error) {
	s.deletedDepartmentCorpID = corpID
	s.deletedDepartmentWXDepartmentID = wxDepartmentID
	return true, nil
}

type fakeWeWorkCallbackWorkerClient struct {
	departments                []WorkEmployeeSyncDepartment
	departmentUsers            map[int][]WorkEmployeeSyncEmployee
	userDetails                map[string]WorkEmployeeSyncEmployee
	followUsers                []string
	tags                       []WorkContactTagSyncGroup
	externalIDs                map[string][]string
	contacts                   map[string]WorkContactSyncContact
	groupChats                 []WorkRoomSyncGroupChat
	rooms                      map[string]WorkRoomSyncRoom
	detailCalls                int
	detailChatIDs              []string
	userDetailCalls            []string
	externalDetailCalls        []string
	groupIDCalls               []string
	tagIDCalls                 []string
	markTagsPayload            WorkContactMarkTagsPayload
	markTagsErr                error
	uploadedImagePath          string
	contactBatchSendCredential RoomWelcomeCorpCredential
	contactBatchSendPayload    ContactMessageBatchSendMessagePayload
	contactBatchSendErr        error
	agentTextCredential        RoomTagPullAgentCredential
	agentTextToUser            string
	agentTextContent           string
	agentTextDuplicateChecks   int
}

func (c *fakeWeWorkCallbackWorkerClient) Departments(_ context.Context, _ WorkEmployeeSyncCredential) ([]WorkEmployeeSyncDepartment, error) {
	return append([]WorkEmployeeSyncDepartment{}, c.departments...), nil
}

func (c *fakeWeWorkCallbackWorkerClient) DepartmentUsers(_ context.Context, _ WorkEmployeeSyncCredential, wxDepartmentID int) ([]WorkEmployeeSyncEmployee, error) {
	return append([]WorkEmployeeSyncEmployee{}, c.departmentUsers[wxDepartmentID]...), nil
}

func (c *fakeWeWorkCallbackWorkerClient) User(_ context.Context, _ WorkEmployeeSyncCredential, wxUserID string) (WorkEmployeeSyncEmployee, error) {
	c.userDetailCalls = append(c.userDetailCalls, wxUserID)
	return c.userDetails[wxUserID], nil
}

func (c *fakeWeWorkCallbackWorkerClient) FollowUsers(_ context.Context, _ WorkEmployeeSyncCredential) ([]string, error) {
	return append([]string{}, c.followUsers...), nil
}

func (c *fakeWeWorkCallbackWorkerClient) CorpTags(_ context.Context, _ RoomWelcomeCorpCredential) ([]WorkContactTagSyncGroup, error) {
	return append([]WorkContactTagSyncGroup{}, c.tags...), nil
}

func (c *fakeWeWorkCallbackWorkerClient) CorpTagsByIDs(_ context.Context, _ RoomWelcomeCorpCredential, groupIDs []string, tagIDs []string) ([]WorkContactTagSyncGroup, error) {
	c.groupIDCalls = append(c.groupIDCalls, groupIDs...)
	c.tagIDCalls = append(c.tagIDCalls, tagIDs...)
	return append([]WorkContactTagSyncGroup{}, c.tags...), nil
}

func (c *fakeWeWorkCallbackWorkerClient) ExternalContactList(_ context.Context, _ RoomWelcomeCorpCredential, wxUserID string) ([]string, bool, error) {
	return append([]string{}, c.externalIDs[wxUserID]...), false, nil
}

func (c *fakeWeWorkCallbackWorkerClient) ExternalContactDetail(_ context.Context, _ RoomWelcomeCorpCredential, wxExternalUserID string) (WorkContactSyncContact, error) {
	c.externalDetailCalls = append(c.externalDetailCalls, wxExternalUserID)
	return c.contacts[wxExternalUserID], nil
}

func (c *fakeWeWorkCallbackWorkerClient) MarkExternalContactTags(_ context.Context, _ RoomWelcomeCorpCredential, payload WorkContactMarkTagsPayload) error {
	c.markTagsPayload = payload
	return c.markTagsErr
}

func (c *fakeWeWorkCallbackWorkerClient) UploadTemporaryImage(_ context.Context, _ RoomWelcomeCorpCredential, filePath string) (string, error) {
	c.uploadedImagePath = filePath
	return "media-fission-push", nil
}

func (c *fakeWeWorkCallbackWorkerClient) SubmitContactMessageBatchSend(_ context.Context, credential RoomWelcomeCorpCredential, payload ContactMessageBatchSendMessagePayload) (ContactMessageBatchSendMessageResult, error) {
	c.contactBatchSendCredential = credential
	c.contactBatchSendPayload = payload
	if c.contactBatchSendErr != nil {
		return ContactMessageBatchSendMessageResult{}, c.contactBatchSendErr
	}
	return ContactMessageBatchSendMessageResult{ErrCode: 0, ErrMsg: "ok", MsgID: "msg-fission-push"}, nil
}

func (c *fakeWeWorkCallbackWorkerClient) SendAgentTextMessage(_ context.Context, credential RoomTagPullAgentCredential, toUser string, content string) error {
	c.agentTextCredential = credential
	c.agentTextToUser = toUser
	c.agentTextContent = content
	return nil
}

func (c *fakeWeWorkCallbackWorkerClient) SendAgentTextMessageWithDuplicateCheck(_ context.Context, credential RoomTagPullAgentCredential, toUser string, content string) error {
	c.agentTextDuplicateChecks++
	c.agentTextCredential = credential
	c.agentTextToUser = toUser
	c.agentTextContent = content
	return nil
}

func (c *fakeWeWorkCallbackWorkerClient) GroupChats(_ context.Context, _ RoomWelcomeCorpCredential) ([]WorkRoomSyncGroupChat, error) {
	return append([]WorkRoomSyncGroupChat{}, c.groupChats...), nil
}

func (c *fakeWeWorkCallbackWorkerClient) GroupChatDetail(_ context.Context, _ RoomWelcomeCorpCredential, wxChatID string) (WorkRoomSyncRoom, error) {
	c.detailCalls++
	c.detailChatIDs = append(c.detailChatIDs, wxChatID)
	return c.rooms[wxChatID], nil
}

type fakeWeWorkCallbackWorkerQueue struct {
	contactWelcomeEvent ContactWelcomeEvent
	welcomeStatuses     map[int]int
	welcomeStatusTTL    time.Duration
	markTagsEvents      []MarkTagsEvent
}

func fakeWeWorkCallbackWorkerCapabilities(queue *fakeWeWorkCallbackWorkerQueue) WeWorkCallbackWorkerCapabilities {
	return WeWorkCallbackWorkerCapabilities{
		ContactWelcomeQueue: queue,
		ContactWelcomeCache: queue,
		MarkTagsQueue:       queue,
	}
}

func (q *fakeWeWorkCallbackWorkerQueue) EnqueueContactWelcome(_ context.Context, event ContactWelcomeEvent) error {
	q.contactWelcomeEvent = event
	return nil
}

func (q *fakeWeWorkCallbackWorkerQueue) EnqueueMarkTags(_ context.Context, event MarkTagsEvent) error {
	q.markTagsEvents = append(q.markTagsEvents, event)
	return nil
}

func (q *fakeWeWorkCallbackWorkerQueue) WorkContactWelcomeStatus(_ context.Context, contactID int) (int, error) {
	return q.welcomeStatuses[contactID], nil
}

func (q *fakeWeWorkCallbackWorkerQueue) SetWorkContactWelcomeStatus(_ context.Context, contactID int, status int, ttl time.Duration) error {
	if q.welcomeStatuses == nil {
		q.welcomeStatuses = map[int]int{}
	}
	q.welcomeStatuses[contactID] = status
	q.welcomeStatusTTL = ttl
	return nil
}

type fakeDurableWeWorkCallbackWorkerStore struct {
	*fakeWeWorkCallbackWorkerStore
	mu                   sync.Mutex
	claims               []WeWorkCallbackClaim
	completedClaims      []WeWorkCallbackClaim
	failedClaims         []WeWorkCallbackClaim
	failedReasons        []string
	currentFences        map[string]uint64
	claimLeaseDurations  []time.Duration
	completed            chan WeWorkCallbackClaim
	acceptLegacyErr      error
	acceptedLegacyEvents []WeWorkCallbackEvent
	acceptedLegacyKeys   []string
	legacyUniqueKeys     map[string]struct{}
}

func (s *fakeDurableWeWorkCallbackWorkerStore) AcceptWeWorkCallback(_ context.Context, event WeWorkCallbackEvent, eventKey string, _ string) (bool, error) {
	if s.acceptLegacyErr != nil {
		return false, s.acceptLegacyErr
	}
	s.acceptedLegacyEvents = append(s.acceptedLegacyEvents, event)
	s.acceptedLegacyKeys = append(s.acceptedLegacyKeys, eventKey)
	if s.legacyUniqueKeys == nil {
		s.legacyUniqueKeys = map[string]struct{}{}
	}
	_, replayed := s.legacyUniqueKeys[eventKey]
	s.legacyUniqueKeys[eventKey] = struct{}{}
	return replayed, nil
}

func (s *fakeDurableWeWorkCallbackWorkerStore) ClaimWeWorkCallback(_ context.Context, leaseDuration time.Duration, _ int) (WeWorkCallbackClaim, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimLeaseDurations = append(s.claimLeaseDurations, leaseDuration)
	if len(s.claims) == 0 {
		return WeWorkCallbackClaim{}, false, nil
	}
	claim := s.claims[0]
	s.claims = s.claims[1:]
	return claim, true, nil
}

func (s *fakeDurableWeWorkCallbackWorkerStore) ValidateWeWorkCallbackClaim(_ context.Context, claim WeWorkCallbackClaim) error {
	if current, ok := s.currentFences[claim.EventKey]; ok && current != claim.LeaseFence {
		return ErrWeWorkCallbackLeaseLost
	}
	return nil
}

func (s *fakeDurableWeWorkCallbackWorkerStore) CompleteWeWorkCallback(_ context.Context, claim WeWorkCallbackClaim) error {
	s.mu.Lock()
	s.completedClaims = append(s.completedClaims, claim)
	s.mu.Unlock()
	if s.completed != nil {
		s.completed <- claim
	}
	return nil
}

func (s *fakeDurableWeWorkCallbackWorkerStore) FailWeWorkCallback(_ context.Context, claim WeWorkCallbackClaim, reason string, _ int, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedClaims = append(s.failedClaims, claim)
	s.failedReasons = append(s.failedReasons, reason)
	return false, nil
}

type fakeLegacyWeWorkCallbackBacklog struct {
	stats      LegacyWeWorkCallbackBacklogStats
	deliveries []LegacyWeWorkCallbackDelivery
	acked      []LegacyWeWorkCallbackDelivery
	nextCalls  int
	preflights int
}

func (b *fakeLegacyWeWorkCallbackBacklog) PreflightLegacyWeWorkCallbackBacklog(context.Context) (LegacyWeWorkCallbackBacklogStats, error) {
	b.preflights++
	if b.preflights > 1 {
		return LegacyWeWorkCallbackBacklogStats{}, nil
	}
	return b.stats, nil
}

func (b *fakeLegacyWeWorkCallbackBacklog) NextLegacyWeWorkCallback(context.Context) (LegacyWeWorkCallbackDelivery, bool, error) {
	b.nextCalls++
	if len(b.deliveries) == 0 {
		return LegacyWeWorkCallbackDelivery{}, false, nil
	}
	delivery := b.deliveries[0]
	return delivery, true, nil
}

func (b *fakeLegacyWeWorkCallbackBacklog) AckLegacyWeWorkCallback(_ context.Context, delivery LegacyWeWorkCallbackDelivery) error {
	if len(b.deliveries) == 0 || b.deliveries[0].Raw != delivery.Raw {
		return errors.New("delivery is not pending acknowledgement")
	}
	b.deliveries = b.deliveries[1:]
	b.acked = append(b.acked, delivery)
	return nil
}
