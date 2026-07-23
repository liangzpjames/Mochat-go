package dashboard

import (
	"context"
	"testing"
	"time"
)

func TestContactBatchSendResultCronSyncsResults(t *testing.T) {
	now := time.Date(2026, 7, 4, 15, 30, 0, 0, time.Local)
	store := &fakeContactBatchSendResultCronStore{
		ids: []int{501},
		target: ContactBatchSendSyncTarget{
			ID:                501,
			BatchID:           7001,
			Status:            0,
			MsgID:             "msg-contact",
			SendEmployeeTotal: 2,
			SendContactTotal:  3,
			Credential:        RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		},
	}
	client := &fakeBatchSendResultCronClient{
		taskPage: BatchSendGroupTaskPage{
			ErrCode: 0,
			ErrMsg:  "ok",
			TaskList: []BatchSendGroupTask{
				{UserID: "employee-wx", Status: 0, SendTime: 1783155600},
				{UserID: "employee-wx", Status: 1, SendTime: 1783159200},
			},
		},
		resultPages: []BatchSendGroupResultPage{
			{
				SendList: []BatchSendGroupResult{{
					UserID:         "employee-wx",
					ExternalUserID: "external-1",
					Status:         1,
					SendTime:       1783159300,
				}},
				NextCursor: "cursor-2",
			},
			{
				SendList: []BatchSendGroupResult{{
					UserID:         "employee-wx",
					ExternalUserID: "external-2",
					Status:         2,
					SendTime:       1783159400,
				}},
			},
		},
	}
	cron := NewContactBatchSendResultCron(store, client, nil)
	cron.now = func() time.Time { return now }

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	expectedSince := time.Date(2026, 6, 27, 0, 0, 0, 0, time.Local)
	if !store.since.Equal(expectedSince) {
		t.Fatalf("since = %s", store.since)
	}
	if client.taskMsgID != "msg-contact" || client.taskLimit != 500 {
		t.Fatalf("task request = msg %q limit %d", client.taskMsgID, client.taskLimit)
	}
	if store.employeeUpdateID != 501 || store.employeeErrCode != 0 || store.employeeErrMsg != "ok" || store.employeeSendTime != 1783159200 {
		t.Fatalf("employee update = id %d code %d msg %q send %d", store.employeeUpdateID, store.employeeErrCode, store.employeeErrMsg, store.employeeSendTime)
	}
	if len(store.resultUpdates) != 2 {
		t.Fatalf("result updates = %#v", store.resultUpdates)
	}
	if store.resultUpdates[0].externalUserID != "external-1" || store.resultUpdates[1].externalUserID != "external-2" {
		t.Fatalf("result updates = %#v", store.resultUpdates)
	}
	if got := client.resultCursors; len(got) != 2 || got[0] != "" || got[1] != "cursor-2" {
		t.Fatalf("result cursors = %#v", got)
	}
	if store.refreshBatchID != 7001 || store.refreshEmployeeTotal != 2 || store.refreshContactTotal != 3 {
		t.Fatalf("refresh = batch %d employees %d contacts %d", store.refreshBatchID, store.refreshEmployeeTotal, store.refreshContactTotal)
	}
}

func TestRoomBatchSendResultCronSyncsResults(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.Local)
	store := &fakeRoomBatchSendResultCronStore{
		ids: []int{601},
		target: RoomBatchSendSyncTarget{
			ID:                601,
			BatchID:           8001,
			Status:            0,
			MsgID:             "msg-room",
			SendEmployeeTotal: 1,
			SendRoomTotal:     2,
			Credential:        RoomWelcomeCorpCredential{CorpID: 8, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		},
	}
	client := &fakeBatchSendResultCronClient{
		taskPage: BatchSendGroupTaskPage{
			ErrCode:  0,
			ErrMsg:   "ok",
			TaskList: []BatchSendGroupTask{{UserID: "room-owner", Status: 1, SendTime: 1783159200}},
		},
		resultPages: []BatchSendGroupResultPage{{
			SendList: []BatchSendGroupResult{{
				UserID:   "room-owner",
				ChatID:   "chat-1",
				Status:   1,
				SendTime: 1783159300,
			}},
		}},
	}
	cron := NewRoomBatchSendResultCron(store, client, nil)
	cron.now = func() time.Time { return now }

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	expectedSince := time.Date(2026, 6, 27, 0, 0, 0, 0, time.Local)
	if !store.since.Equal(expectedSince) {
		t.Fatalf("since = %s", store.since)
	}
	if store.employeeUpdateID != 601 || store.employeeSendTime != 1783159200 {
		t.Fatalf("employee update = id %d send %d", store.employeeUpdateID, store.employeeSendTime)
	}
	if len(store.resultUpdates) != 1 || store.resultUpdates[0].chatID != "chat-1" || store.resultUpdates[0].status != 1 {
		t.Fatalf("result updates = %#v", store.resultUpdates)
	}
	if store.refreshBatchID != 8001 || store.refreshEmployeeTotal != 1 || store.refreshRoomTotal != 2 {
		t.Fatalf("refresh = batch %d employees %d rooms %d", store.refreshBatchID, store.refreshEmployeeTotal, store.refreshRoomTotal)
	}
}

func TestBatchSendResultCronsRequireDependencies(t *testing.T) {
	if err := NewContactBatchSendResultCron(nil, nil, nil).RunOnce(context.Background()); err == nil || err.Error() != "ContactSyncSendResultTask cron dependencies are not configured" {
		t.Fatalf("contact err = %v", err)
	}
	if err := NewRoomBatchSendResultCron(nil, nil, nil).RunOnce(context.Background()); err == nil || err.Error() != "RoomSyncSendResultTask cron dependencies are not configured" {
		t.Fatalf("room err = %v", err)
	}
}

type fakeContactBatchSendResultCronStore struct {
	ids                  []int
	since                time.Time
	target               ContactBatchSendSyncTarget
	employeeUpdateID     int
	employeeErrCode      int
	employeeErrMsg       string
	employeeSendTime     int64
	resultUpdates        []fakeBatchSendResultUpdate
	refreshBatchID       int
	refreshEmployeeTotal int
	refreshContactTotal  int
}

func (s *fakeContactBatchSendResultCronStore) ContactBatchSendEmployeeIDsForSync(_ context.Context, since time.Time) ([]int, error) {
	s.since = since
	return append([]int{}, s.ids...), nil
}

func (s *fakeContactBatchSendResultCronStore) ContactBatchSendSyncTarget(context.Context, int) (ContactBatchSendSyncTarget, bool, error) {
	return s.target, true, nil
}

func (s *fakeContactBatchSendResultCronStore) UpdateContactBatchSendEmployeeSent(_ context.Context, batchEmployeeID int, errCode int, errMsg string, sendTime int64) error {
	s.employeeUpdateID = batchEmployeeID
	s.employeeErrCode = errCode
	s.employeeErrMsg = errMsg
	s.employeeSendTime = sendTime
	return nil
}

func (s *fakeContactBatchSendResultCronStore) UpdateContactBatchSendResult(_ context.Context, batchID int, externalUserID string, userID string, status int, sendTime int64) error {
	s.resultUpdates = append(s.resultUpdates, fakeBatchSendResultUpdate{batchID: batchID, externalUserID: externalUserID, userID: userID, status: status, sendTime: sendTime})
	return nil
}

func (s *fakeContactBatchSendResultCronStore) RefreshContactBatchSendTotals(_ context.Context, batchID int, sendEmployeeTotal int, sendContactTotal int) error {
	s.refreshBatchID = batchID
	s.refreshEmployeeTotal = sendEmployeeTotal
	s.refreshContactTotal = sendContactTotal
	return nil
}

type fakeRoomBatchSendResultCronStore struct {
	ids                  []int
	since                time.Time
	target               RoomBatchSendSyncTarget
	employeeUpdateID     int
	employeeSendTime     int64
	resultUpdates        []fakeBatchSendResultUpdate
	refreshBatchID       int
	refreshEmployeeTotal int
	refreshRoomTotal     int
}

func (s *fakeRoomBatchSendResultCronStore) RoomBatchSendEmployeeIDsForSync(_ context.Context, since time.Time) ([]int, error) {
	s.since = since
	return append([]int{}, s.ids...), nil
}

func (s *fakeRoomBatchSendResultCronStore) RoomBatchSendSyncTarget(context.Context, int) (RoomBatchSendSyncTarget, bool, error) {
	return s.target, true, nil
}

func (s *fakeRoomBatchSendResultCronStore) UpdateRoomBatchSendEmployeeSent(_ context.Context, batchEmployeeID int, _ int, _ string, sendTime int64) error {
	s.employeeUpdateID = batchEmployeeID
	s.employeeSendTime = sendTime
	return nil
}

func (s *fakeRoomBatchSendResultCronStore) UpdateRoomBatchSendResult(_ context.Context, batchID int, chatID string, userID string, status int, sendTime int64) error {
	s.resultUpdates = append(s.resultUpdates, fakeBatchSendResultUpdate{batchID: batchID, chatID: chatID, userID: userID, status: status, sendTime: sendTime})
	return nil
}

func (s *fakeRoomBatchSendResultCronStore) RefreshRoomBatchSendTotals(_ context.Context, batchID int, sendEmployeeTotal int, sendRoomTotal int) error {
	s.refreshBatchID = batchID
	s.refreshEmployeeTotal = sendEmployeeTotal
	s.refreshRoomTotal = sendRoomTotal
	return nil
}

type fakeBatchSendResultCronClient struct {
	taskPage      BatchSendGroupTaskPage
	taskMsgID     string
	taskLimit     int
	resultPages   []BatchSendGroupResultPage
	resultCursors []string
}

func (c *fakeBatchSendResultCronClient) GroupMessageTasks(_ context.Context, _ RoomWelcomeCorpCredential, msgID string, limit int, _ string) (BatchSendGroupTaskPage, error) {
	c.taskMsgID = msgID
	c.taskLimit = limit
	return c.taskPage, nil
}

func (c *fakeBatchSendResultCronClient) GroupMessageSendResults(_ context.Context, _ RoomWelcomeCorpCredential, _ string, _ string, _ int, cursor string) (BatchSendGroupResultPage, error) {
	c.resultCursors = append(c.resultCursors, cursor)
	if len(c.resultPages) == 0 {
		return BatchSendGroupResultPage{}, nil
	}
	page := c.resultPages[0]
	c.resultPages = c.resultPages[1:]
	return page, nil
}

type fakeBatchSendResultUpdate struct {
	batchID        int
	externalUserID string
	chatID         string
	userID         string
	status         int
	sendTime       int64
}
