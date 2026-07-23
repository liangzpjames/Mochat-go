package dashboard

import (
	"context"
	"testing"
)

func TestRoomTagPullCronSyncsTaskAndContactResults(t *testing.T) {
	store := &fakeRoomTagPullCronStore{
		corps: []RoomTagPullCronCorp{{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"}},
		activities: []RoomTagPullCronActivity{{
			ID: 9001,
			WXTIDs: []RoomTagPullWXTID{
				{WXUserID: "employee-wx", TID: "msg-room-tag", Status: 0},
			},
		}},
		pending: true,
	}
	client := &fakeBatchSendResultCronClient{
		taskPage: BatchSendGroupTaskPage{
			TaskList: []BatchSendGroupTask{{UserID: "employee-wx", Status: 1, SendTime: 1783159200}},
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
					Status:         3,
					SendTime:       1783159400,
				}},
			},
		},
	}
	cron := NewRoomTagPullCron(store, client, nil)

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.taskMsgID != "msg-room-tag" || client.taskLimit != 500 {
		t.Fatalf("task request = msg %q limit %d", client.taskMsgID, client.taskLimit)
	}
	if len(store.savedTIDs) != 1 || store.savedTIDs[0].Status != 1 || store.savedTIDs[0].WXUserID != "employee-wx" {
		t.Fatalf("saved tids = %#v", store.savedTIDs)
	}
	if len(store.contactUpdates) != 2 {
		t.Fatalf("contact updates = %#v", store.contactUpdates)
	}
	if store.contactUpdates[0].externalUserID != "external-1" || store.contactUpdates[0].status != 1 {
		t.Fatalf("contact update 0 = %#v", store.contactUpdates[0])
	}
	if store.contactUpdates[1].externalUserID != "external-2" || store.contactUpdates[1].status != 3 {
		t.Fatalf("contact update 1 = %#v", store.contactUpdates[1])
	}
	if got := client.resultCursors; len(got) != 2 || got[0] != "" || got[1] != "cursor-2" {
		t.Fatalf("result cursors = %#v", got)
	}
}

func TestRoomTagPullCronRequiresDependencies(t *testing.T) {
	err := NewRoomTagPullCron(nil, nil, nil).RunOnce(context.Background())
	if err == nil || err.Error() != "RoomTagPull cron dependencies are not configured" {
		t.Fatalf("err = %v", err)
	}
}

type fakeRoomTagPullCronStore struct {
	corps          []RoomTagPullCronCorp
	activities     []RoomTagPullCronActivity
	pending        bool
	savedTIDs      []RoomTagPullWXTID
	contactUpdates []fakeBatchSendResultUpdate
}

func (s *fakeRoomTagPullCronStore) RoomTagPullCronCorps(context.Context) ([]RoomTagPullCronCorp, error) {
	return append([]RoomTagPullCronCorp{}, s.corps...), nil
}

func (s *fakeRoomTagPullCronStore) RoomTagPullCronActivitiesByCorp(context.Context, int) ([]RoomTagPullCronActivity, error) {
	return append([]RoomTagPullCronActivity{}, s.activities...), nil
}

func (s *fakeRoomTagPullCronStore) UpdateRoomTagPullWXTIDs(_ context.Context, _ int, tids []RoomTagPullWXTID) error {
	s.savedTIDs = append([]RoomTagPullWXTID{}, tids...)
	return nil
}

func (s *fakeRoomTagPullCronStore) RoomTagPullHasPendingContacts(context.Context, int) (bool, error) {
	return s.pending, nil
}

func (s *fakeRoomTagPullCronStore) UpdateRoomTagPullContactSendStatus(_ context.Context, activityID int, wxUserID string, externalUserID string, status int) error {
	s.contactUpdates = append(s.contactUpdates, fakeBatchSendResultUpdate{batchID: activityID, userID: wxUserID, externalUserID: externalUserID, status: status})
	return nil
}
