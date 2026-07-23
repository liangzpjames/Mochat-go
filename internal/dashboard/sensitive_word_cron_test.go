package dashboard

import (
	"context"
	"io"
	"log"
	"strings"
	"testing"
	"time"
)

func TestSensitiveWordMonitorCronScansMessagesAndAdvancesCursors(t *testing.T) {
	store := &fakeSensitiveWordMonitorCronStore{
		words: []SensitiveWordCronWord{
			{ID: 10, CorpID: 1, Name: "违规"},
			{ID: 11, CorpID: 1, Name: "Refund"},
			{ID: 12, CorpID: 2, Name: "其它企业"},
		},
		cursors: map[[2]int]int{},
		messages: map[[2]int][]SensitiveWordArchivedMessage{
			{1, 1}: {
				{
					TableIndex:     1,
					ID:             1,
					CorpID:         1,
					WorkEmployeeID: 101,
					ToUserType:     1,
					ToUserID:       201,
					SenderType:     0,
					MsgType:        1,
					ContentText:    "普通问候",
					MsgDataTime:    time.Date(2026, 7, 6, 9, 0, 0, 0, time.Local),
					SenderName:     "员工A",
					TargetName:     "客户B",
				},
				{
					TableIndex:     1,
					ID:             2,
					CorpID:         1,
					WorkEmployeeID: 101,
					ToUserType:     1,
					ToUserID:       201,
					SenderType:     0,
					MsgType:        1,
					ContentRaw:     `{"content":"这是一条违规消息"}`,
					MsgDataTime:    time.Date(2026, 7, 6, 9, 1, 0, 0, time.Local),
					SenderName:     "员工A",
					TargetName:     "客户B",
				},
				{
					TableIndex:     1,
					ID:             3,
					CorpID:         1,
					WorkEmployeeID: 101,
					ToUserType:     1,
					ToUserID:       201,
					SenderType:     1,
					MsgType:        1,
					ContentRaw:     `{"text":"customer wants a refund"}`,
					MsgDataTime:    time.Date(2026, 7, 6, 9, 2, 0, 0, time.Local),
					SenderName:     "员工A",
					TargetName:     "客户B",
				},
			},
		},
	}
	cron := NewSensitiveWordMonitorCron(store, log.New(io.Discard, "", 0))
	cron.now = func() time.Time { return time.Date(2026, 7, 6, 10, 0, 0, 0, time.Local) }

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := len(store.inserts); got != 2 {
		t.Fatalf("inserted monitor count = %d, want 2: %#v", got, store.inserts)
	}
	first := store.inserts[0]
	if first.SensitiveWordID != 10 || first.Source != 2 || first.TriggerUserID != 101 || first.TriggerName != "员工A" || first.Sender != "员工A" || first.TriggerScenario != "客户会话" {
		t.Fatalf("employee trigger monitor = %#v", first)
	}
	if !strings.Contains(first.ConversationJSON, `"isTrigger":1`) || !strings.Contains(first.ConversationJSON, "违规消息") {
		t.Fatalf("employee trigger conversation json = %s", first.ConversationJSON)
	}
	second := store.inserts[1]
	if second.SensitiveWordID != 11 || second.Source != 1 || second.TriggerUserID != 201 || second.TriggerName != "客户B" || second.Sender != "客户B" {
		t.Fatalf("contact trigger monitor = %#v", second)
	}
	if got := store.cursors[[2]int{1, 1}]; got != 3 {
		t.Fatalf("cursor corp 1 table 1 = %d, want 3", got)
	}
}

func TestSensitiveWordMonitorCronRequiresStore(t *testing.T) {
	err := NewSensitiveWordMonitorCron(nil, nil).RunOnce(context.Background())
	if err == nil || err.Error() != "sensitiveWordsMonitor cron dependencies are not configured" {
		t.Fatalf("RunOnce err = %v", err)
	}
}

type fakeSensitiveWordMonitorCronStore struct {
	words    []SensitiveWordCronWord
	cursors  map[[2]int]int
	messages map[[2]int][]SensitiveWordArchivedMessage
	inserts  []SensitiveWordMonitorCreate
}

func (s *fakeSensitiveWordMonitorCronStore) ActiveSensitiveWords(context.Context) ([]SensitiveWordCronWord, error) {
	return s.words, nil
}

func (s *fakeSensitiveWordMonitorCronStore) SensitiveWordMessageCursor(_ context.Context, corpID int, tableIndex int) (int, error) {
	return s.cursors[[2]int{corpID, tableIndex}], nil
}

func (s *fakeSensitiveWordMonitorCronStore) PendingSensitiveWordMessages(_ context.Context, corpID int, tableIndex int, afterID int, limit int) ([]SensitiveWordArchivedMessage, error) {
	messages := s.messages[[2]int{corpID, tableIndex}]
	filtered := make([]SensitiveWordArchivedMessage, 0, len(messages))
	for _, message := range messages {
		if message.ID <= afterID {
			continue
		}
		filtered = append(filtered, message)
		if len(filtered) == limit {
			break
		}
	}
	return filtered, nil
}

func (s *fakeSensitiveWordMonitorCronStore) InsertSensitiveWordMonitor(_ context.Context, item SensitiveWordMonitorCreate) (bool, error) {
	for _, existing := range s.inserts {
		if existing.CorpID == item.CorpID && existing.SensitiveWordID == item.SensitiveWordID && existing.TriggerUserID == item.TriggerUserID && existing.SendTime.Equal(item.SendTime) && existing.ContentRaw == item.ContentRaw {
			return false, nil
		}
	}
	s.inserts = append(s.inserts, item)
	return true, nil
}

func (s *fakeSensitiveWordMonitorCronStore) UpdateSensitiveWordMessageCursor(_ context.Context, corpID int, tableIndex int, lastID int) error {
	s.cursors[[2]int{corpID, tableIndex}] = lastID
	return nil
}
