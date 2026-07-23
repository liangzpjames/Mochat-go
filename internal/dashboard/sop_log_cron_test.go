package dashboard

import (
	"context"
	"io"
	"log"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSOPLogCronGeneratesDueContactAndRoomLogsIdempotently(t *testing.T) {
	store := &fakeSOPLogCronStore{
		contactSources: []ContactSOPLogSource{{
			ID:             66,
			CorpID:         7,
			SettingRaw:     `[{"content":[{"type":"text","value":"上午提醒"}],"time":"10:00"},{"content":[{"type":"text","value":"未来提醒"}],"time":"11:00"}]`,
			EmployeeIDsRaw: `[{"id":31}]`,
			ContactIDsRaw:  `88,88`,
		}},
		roomSources: []RoomSOPLogSource{{
			ID:         77,
			CorpID:     7,
			SettingRaw: `{"content":[{"type":"text","value":"群提醒"}]}`,
			RoomIDsRaw: `[{"roomId":99}]`,
		}},
		contactTargets: []ContactSOPLogTarget{{
			EmployeeWXUserID:      "wx-user-31",
			ContactWXExternalUser: "external-88",
		}},
		roomTargets: []RoomSOPLogTarget{{
			RoomID:           99,
			EmployeeWXUserID: "wx-owner-31",
		}},
	}
	cron := NewSOPLogCron(store, log.New(io.Discard, "", 0))
	cron.now = func() time.Time {
		return time.Date(2026, 7, 6, 10, 30, 0, 0, time.Local)
	}

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.contactLogs) != 1 {
		t.Fatalf("contact logs = %#v", store.contactLogs)
	}
	if len(store.roomLogs) != 1 {
		t.Fatalf("room logs = %#v", store.roomLogs)
	}
	if !strings.Contains(store.contactLogs[0].TaskRaw, "上午提醒") || strings.Contains(store.contactLogs[0].TaskRaw, "未来提醒") {
		t.Fatalf("contact task raw = %s", store.contactLogs[0].TaskRaw)
	}
	if store.contactEmployeeIDs[0] != 31 || store.contactContactIDs[0] != 88 {
		t.Fatalf("contact target ids = employees %#v contacts %#v", store.contactEmployeeIDs, store.contactContactIDs)
	}
	if store.roomRoomIDs[0] != 99 {
		t.Fatalf("room ids = %#v", store.roomRoomIDs)
	}
}

func TestSOPLogCronRequiresStore(t *testing.T) {
	err := NewSOPLogCron(nil, nil).RunOnce(context.Background())
	if err == nil || err.Error() != "SOP log cron dependencies are not configured" {
		t.Fatalf("err = %v", err)
	}
}

func TestSOPLogCronUsesTargetAnchorForRelativeDelay(t *testing.T) {
	now := time.Date(2026, 7, 6, 10, 30, 0, 0, time.Local)
	store := &fakeSOPLogCronStore{
		contactSources: []ContactSOPLogSource{{
			ID:             66,
			CorpID:         7,
			SettingRaw:     `{"content":[{"type":"text","value":"添加后提醒"}],"delayMinutes":30}`,
			EmployeeIDsRaw: `[{"id":31}]`,
			ContactIDsRaw:  `[{"id":88},{"id":89}]`,
		}},
		roomSources: []RoomSOPLogSource{{
			ID:         77,
			CorpID:     7,
			SettingRaw: `{"content":[{"type":"text","value":"建群后提醒"}],"after":"1h"}`,
			RoomIDsRaw: `[{"roomId":99}]`,
		}},
		contactTargets: []ContactSOPLogTarget{{
			EmployeeWXUserID:      "wx-user-31",
			ContactWXExternalUser: "external-88",
			AnchorTime:            time.Date(2026, 7, 6, 10, 0, 0, 0, time.Local),
		}, {
			EmployeeWXUserID:      "wx-user-31",
			ContactWXExternalUser: "external-89",
			AnchorTime:            time.Date(2026, 7, 6, 10, 10, 0, 0, time.Local),
		}},
		roomTargets: []RoomSOPLogTarget{{
			RoomID:           99,
			EmployeeWXUserID: "wx-owner-31",
			AnchorTime:       time.Date(2026, 7, 6, 9, 30, 0, 0, time.Local),
		}},
	}
	cron := NewSOPLogCron(store, log.New(io.Discard, "", 0))
	cron.now = func() time.Time { return now }

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.contactLogs) != 1 {
		t.Fatalf("contact logs = %#v", store.contactLogs)
	}
	if store.contactLogs[0].ContactWXExternalUser != "external-88" || !store.contactLogs[0].CreatedAt.Equal(now) {
		t.Fatalf("contact log = %#v", store.contactLogs[0])
	}
	if len(store.roomLogs) != 1 {
		t.Fatalf("room logs = %#v", store.roomLogs)
	}
	if !store.roomLogs[0].CreatedAt.Equal(now) {
		t.Fatalf("room log = %#v", store.roomLogs[0])
	}
}

func TestSOPLogCronUsesRoomJoinAnchorForRelativeRoomDelay(t *testing.T) {
	now := time.Date(2026, 7, 6, 10, 30, 0, 0, time.Local)
	store := &fakeSOPLogCronStore{
		roomSources: []RoomSOPLogSource{{
			ID:         77,
			CorpID:     7,
			SettingRaw: `{"content":[{"type":"text","value":"入群后提醒"}],"targetAnchor":"customer_join_room","delayMinutes":30}`,
			RoomIDsRaw: `[{"roomId":99}]`,
		}},
		roomTargetsByAnchor: map[string][]RoomSOPLogTarget{
			SOPLogTargetAnchorRoomJoin: {{
				RoomID:                99,
				EmployeeWXUserID:      "wx-owner-31",
				ContactWXExternalUser: "external-88",
				AnchorTime:            time.Date(2026, 7, 6, 10, 0, 0, 0, time.Local),
			}, {
				RoomID:                99,
				EmployeeWXUserID:      "wx-owner-31",
				ContactWXExternalUser: "external-89",
				AnchorTime:            time.Date(2026, 7, 6, 10, 10, 0, 0, time.Local),
			}},
		},
	}
	cron := NewSOPLogCron(store, log.New(io.Discard, "", 0))
	cron.now = func() time.Time { return now }

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.roomTargetAnchors) != 1 || store.roomTargetAnchors[0] != SOPLogTargetAnchorRoomJoin {
		t.Fatalf("room target anchors = %#v", store.roomTargetAnchors)
	}
	if len(store.roomLogs) != 1 {
		t.Fatalf("room logs = %#v", store.roomLogs)
	}
	if store.roomLogs[0].ContactWXExternalUser != "external-88" || !store.roomLogs[0].CreatedAt.Equal(now) {
		t.Fatalf("room log = %#v", store.roomLogs[0])
	}
}

func TestSOPLogTasksSupportAnchoredRelativeDelay(t *testing.T) {
	now := time.Date(2026, 7, 6, 10, 30, 0, 0, time.Local)
	raw := `[
		{"content":[{"type":"text","value":"已到期分钟提醒"}],"baseTime":"2026-07-06 09:00:00","delayMinutes":90},
		{"content":[{"type":"text","value":"未来提醒"}],"startTime":"2026-07-06 10:00:00","delay":"45m"},
		{"content":[{"type":"text","value":"已到期中文提醒"}],"anchorTime":"2026-07-05 10:30:00","after":"1天"}
	]`

	tasks, err := sopLogTasks(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("tasks = %#v", tasks)
	}
	if !strings.Contains(tasks[0].Raw, "已到期分钟提醒") || !tasks[0].CreatedAt.Equal(now) {
		t.Fatalf("first task = %#v", tasks[0])
	}
	if !strings.Contains(tasks[1].Raw, "已到期中文提醒") || !tasks[1].CreatedAt.Equal(now) {
		t.Fatalf("second task = %#v", tasks[1])
	}
	for _, task := range tasks {
		if strings.Contains(task.Raw, "未来提醒") {
			t.Fatalf("future task should be skipped: %#v", tasks)
		}
	}
}

func TestSOPLogTasksPreferAbsoluteTimeOverRelativeDelay(t *testing.T) {
	now := time.Date(2026, 7, 6, 10, 30, 0, 0, time.Local)
	raw := `{"content":[{"type":"text","value":"绝对时间优先"}],"time":"11:00","baseTime":"2026-07-06 09:00:00","delayMinutes":30}`

	tasks, err := sopLogTasks(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("tasks = %#v", tasks)
	}
}

func TestSOPLogTasksSupportRecurringRules(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 30, 0, 0, time.Local)
	raw := `[
		{"content":[{"type":"text","value":"周二已到期"}],"cycle":"weekly","weekdays":[2],"time":"09:00"},
		{"content":[{"type":"text","value":"周三跳过"}],"cycle":"weekly","weekdays":["周三"],"time":"09:00"},
		{"content":[{"type":"text","value":"周二未来"}],"cycle":"weekly","weekdays":"周二","time":"11:00"},
		{"content":[{"type":"text","value":"每月已到期"}],"cycle":"monthly","monthDays":[7],"time":"10:00"},
		{"content":[{"type":"text","value":"指定日期已到期"}],"repeatDates":["2026-07-07"],"time":"08:00"},
		{"content":[{"type":"text","value":"缺少周几跳过"}],"cycle":"weekly","time":"09:00"}
	]`

	tasks, err := sopLogTasks(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 {
		t.Fatalf("tasks = %#v", tasks)
	}
	joined := tasks[0].Raw + tasks[1].Raw + tasks[2].Raw
	for _, expected := range []string{"周二已到期", "每月已到期", "指定日期已到期", "_mochatGoOccurrence"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in %s", expected, joined)
		}
	}
	for _, unexpected := range []string{"周三跳过", "周二未来", "缺少周几跳过"} {
		if strings.Contains(joined, unexpected) {
			t.Fatalf("unexpected %q in %s", unexpected, joined)
		}
	}
}

func TestSOPLogCronCreatesRecurringLogsPerOccurrence(t *testing.T) {
	store := &fakeSOPLogCronStore{
		contactSources: []ContactSOPLogSource{{
			ID:             66,
			CorpID:         7,
			SettingRaw:     `{"content":[{"type":"text","value":"每日提醒"}],"cycle":"daily","time":"09:00"}`,
			EmployeeIDsRaw: `[31]`,
			ContactIDsRaw:  `[88]`,
		}},
		contactTargets: []ContactSOPLogTarget{{
			EmployeeWXUserID:      "wx-user-31",
			ContactWXExternalUser: "external-88",
		}},
	}
	cron := NewSOPLogCron(store, log.New(io.Discard, "", 0))
	now := time.Date(2026, 7, 7, 10, 30, 0, 0, time.Local)
	cron.now = func() time.Time { return now }

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	now = now.AddDate(0, 0, 1)
	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.contactLogs) != 2 {
		t.Fatalf("contact logs = %#v", store.contactLogs)
	}
	if store.contactLogs[0].TaskRaw == store.contactLogs[1].TaskRaw {
		t.Fatalf("recurring task raw should differ per occurrence: %#v", store.contactLogs)
	}
}

type fakeSOPLogCronStore struct {
	contactSources      []ContactSOPLogSource
	roomSources         []RoomSOPLogSource
	contactTargets      []ContactSOPLogTarget
	roomTargets         []RoomSOPLogTarget
	roomTargetsByAnchor map[string][]RoomSOPLogTarget

	contactEmployeeIDs []int
	contactContactIDs  []int
	roomRoomIDs        []int
	roomTargetAnchors  []string
	roomIDsByWXChatID  map[string]int
	contactLogs        []ContactSOPLogCreate
	roomLogs           []RoomSOPLogCreate
	contactKeys        map[string]struct{}
	roomKeys           map[string]struct{}
}

func (s *fakeSOPLogCronStore) ActiveContactSOPLogSources(context.Context) ([]ContactSOPLogSource, error) {
	return append([]ContactSOPLogSource{}, s.contactSources...), nil
}

func (s *fakeSOPLogCronStore) ActiveRoomSOPLogSources(context.Context) ([]RoomSOPLogSource, error) {
	return append([]RoomSOPLogSource{}, s.roomSources...), nil
}

func (s *fakeSOPLogCronStore) ContactSOPLogTargets(_ context.Context, _ int, employeeIDs []int, contactIDs []int) ([]ContactSOPLogTarget, error) {
	s.contactEmployeeIDs = append([]int{}, employeeIDs...)
	s.contactContactIDs = append([]int{}, contactIDs...)
	return append([]ContactSOPLogTarget{}, s.contactTargets...), nil
}

func (s *fakeSOPLogCronStore) RoomSOPLogTargets(_ context.Context, _ int, roomIDs []int, targetAnchor string) ([]RoomSOPLogTarget, error) {
	s.roomRoomIDs = append([]int{}, roomIDs...)
	s.roomTargetAnchors = append(s.roomTargetAnchors, targetAnchor)
	if s.roomTargetsByAnchor != nil {
		if targets, ok := s.roomTargetsByAnchor[targetAnchor]; ok {
			return append([]RoomSOPLogTarget{}, targets...), nil
		}
	}
	return append([]RoomSOPLogTarget{}, s.roomTargets...), nil
}

func (s *fakeSOPLogCronStore) RoomIDByWXChatID(_ context.Context, _ int, wxChatID string) (int, bool, error) {
	if s.roomIDsByWXChatID == nil {
		return 0, false, nil
	}
	id, ok := s.roomIDsByWXChatID[wxChatID]
	return id, ok && id > 0, nil
}

func (s *fakeSOPLogCronStore) InsertContactSOPLog(_ context.Context, item ContactSOPLogCreate) (bool, error) {
	if s.contactKeys == nil {
		s.contactKeys = map[string]struct{}{}
	}
	key := strings.Join([]string{item.EmployeeWXUserID, item.ContactWXExternalUser, item.TaskRaw}, "\x00")
	if _, ok := s.contactKeys[key]; ok {
		return false, nil
	}
	s.contactKeys[key] = struct{}{}
	s.contactLogs = append(s.contactLogs, item)
	return true, nil
}

func (s *fakeSOPLogCronStore) InsertRoomSOPLog(_ context.Context, item RoomSOPLogCreate) (bool, error) {
	if s.roomKeys == nil {
		s.roomKeys = map[string]struct{}{}
	}
	key := strings.Join([]string{strconv.Itoa(item.RoomID), item.EmployeeWXUserID, item.ContactWXExternalUser, item.TaskRaw}, "\x00")
	if _, ok := s.roomKeys[key]; ok {
		return false, nil
	}
	s.roomKeys[key] = struct{}{}
	s.roomLogs = append(s.roomLogs, item)
	return true, nil
}
