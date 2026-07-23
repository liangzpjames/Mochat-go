package dashboard

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestStatisticIndexBuildsModeTable(t *testing.T) {
	store := &fakeStatisticStore{
		users: map[int]User{1: {ID: 1}},
		today: StatisticContactSummary{
			Total: 20,
			Add:   3,
			Loss:  1,
			Net:   2,
		},
		contactTrend: []StatisticContactDay{
			{Date: "2026/07/01", Total: 10, Add: 2, Loss: 1, Net: 1},
			{Date: "2026/07/02", Total: 12, Add: 4, Loss: 1, Net: 3},
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewStatisticHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "http://api.example.com", nil)

	rec := performRequest(handler.Index, "GET", "/dashboard/statistic/index?startTime=2026-07-01&endTime=2026-07-02&mode=4&employeeId=5,6", nil, map[string]string{"X-Mochat-Go-User-ID": "1"})
	if rec.Code != 200 {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/statistic/index#get" || authorizer.corpID != 7 {
		t.Fatalf("authorizer = %+v", authorizer)
	}
	if !reflect.DeepEqual(store.contactTrendEmployeeIDs, []int{5, 6}) {
		t.Fatalf("employee ids = %+v", store.contactTrendEmployeeIDs)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	table := data["table"].(map[string]any)
	if table["2026/07/01"].(float64) != 1 || table["2026/07/02"].(float64) != 3 {
		t.Fatalf("table = %#v", table)
	}
	today := data["today"].(map[string]any)
	if today["total"].(float64) != 20 || today["net"].(float64) != 2 {
		t.Fatalf("today = %#v", today)
	}
}

func TestStatisticEmployeesUsesWeComBehavior(t *testing.T) {
	store := &fakeStatisticStore{
		users:      map[int]User{1: {ID: 1}},
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wwid", ContactSecret: "secret"},
		employees: []StatisticEmployee{
			{ID: 2, WXUserID: "u2", Name: "张三"},
			{ID: 3, WXUserID: "u3", Name: "李四"},
		},
		localSummary: StatisticEmployeeMetric{ChatCnt: 1},
	}
	client := &fakeStatisticBehaviorClient{behaviorByUser: map[string][]StatisticBehaviorData{
		"u2": {{ChatCnt: 10, MessageCnt: 20, ReplyPercentage: 80, AvgReplyTime: 6}},
		"u3": {{ChatCnt: 4, MessageCnt: 8, ReplyPercentage: 100, AvgReplyTime: 2}},
	}}
	handler := NewStatisticHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, nil, "http://api.example.com", client)

	rec := performRequest(handler.Employees, "GET", "/dashboard/statistic/employees?startTime=2026-07-01&endTime=2026-07-02", nil, map[string]string{"X-Mochat-Go-User-ID": "1"})
	if rec.Code != 200 {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if data["chat_cnt"].(float64) != 14 || data["message_cnt"].(float64) != 28 {
		t.Fatalf("data = %#v", data)
	}
	if data["reply_percentage"].(float64) != 90 || data["avg_reply_time"].(float64) != 4 {
		t.Fatalf("averages = %#v", data)
	}
	if len(client.calls) != 1 || !reflect.DeepEqual(client.calls[0], []string{"u2", "u3"}) {
		t.Fatalf("client calls = %+v", client.calls)
	}
}

func TestStatisticEmployeeCountsPaginatesAndFormatsAvatar(t *testing.T) {
	store := &fakeStatisticStore{
		users:      map[int]User{1: {ID: 1}},
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wwid", ContactSecret: "secret"},
		employees: []StatisticEmployee{
			{ID: 2, WXUserID: "u2", Name: "张三", Avatar: "avatar/a.png"},
			{ID: 3, WXUserID: "u3", Name: "李四"},
			{ID: 4, WXUserID: "u4", Name: "王五"},
			{ID: 5, WXUserID: "u5", Name: "赵六"},
			{ID: 6, WXUserID: "u6", Name: "钱七"},
			{ID: 7, WXUserID: "u7", Name: "孙八"},
		},
	}
	client := &fakeStatisticBehaviorClient{behaviorByUser: map[string][]StatisticBehaviorData{
		"u2": {{ChatCnt: 9, MessageCnt: 3, ReplyPercentage: 50, AvgReplyTime: 1}},
	}}
	handler := NewStatisticHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, nil, "http://api.example.com", client)

	rec := performRequest(handler.EmployeeCounts, "GET", "/dashboard/statistic/employeeCounts?startTime=2026-07-01&endTime=2026-07-02&page=1", nil, map[string]string{"X-Mochat-Go-User-ID": "1"})
	if rec.Code != 200 {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if data["total"].(float64) != 6 {
		t.Fatalf("total = %#v", data["total"])
	}
	table := data["table"].([]any)
	if len(table) != 5 {
		t.Fatalf("table len = %d", len(table))
	}
	first := table[0].(map[string]any)
	if first["avatar"] != "http://api.example.com/static/avatar/a.png" || first["chat_cnt"].(float64) != 9 {
		t.Fatalf("first = %#v", first)
	}
}

func TestStatisticEmployeesTrendFiltersEmployeesAndMode(t *testing.T) {
	store := &fakeStatisticStore{
		users:      map[int]User{1: {ID: 1}},
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wwid", ContactSecret: "secret"},
		employeesByID: []StatisticEmployee{
			{ID: 2, WXUserID: "u2", Name: "张三"},
		},
	}
	client := &fakeStatisticBehaviorClient{behaviorByUser: map[string][]StatisticBehaviorData{
		"u2": {{StatTime: time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local).Unix(), ChatCnt: 2, MessageCnt: 7, ReplyPercentage: 66.666, AvgReplyTime: 5}},
	}}
	handler := NewStatisticHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, nil, "http://api.example.com", client)

	rec := performRequest(handler.EmployeesTrend, "GET", "/dashboard/statistic/employeesTrend?startTime=2026-07-01&endTime=2026-07-02&mode=2&employees=%5B2%5D", nil, map[string]string{"X-Mochat-Go-User-ID": "1"})
	if rec.Code != 200 {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !reflect.DeepEqual(store.employeesByIDsArg, []int{2}) {
		t.Fatalf("employees arg = %+v", store.employeesByIDsArg)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	table := data["table"].(map[string]any)
	if table["2026-07-01"].(float64) != 7 {
		t.Fatalf("table = %#v", table)
	}
	list := data["list"].([]any)
	row := list[0].(map[string]any)
	if row["reply_percentage"].(float64) != 66.67 {
		t.Fatalf("row = %#v", row)
	}
}

type fakeStatisticStore struct {
	users                   map[int]User
	today                   StatisticContactSummary
	contactTrend            []StatisticContactDay
	contactTrendEmployeeIDs []int
	topTotal                int
	topList                 []StatisticTopEmployee
	employees               []StatisticEmployee
	employeesByID           []StatisticEmployee
	employeesByIDsArg       []int
	localSummary            StatisticEmployeeMetric
	localByEmployee         map[int]StatisticEmployeeMetric
	localTrend              []StatisticBehaviorData
	credential              RoomWelcomeCorpCredential
}

func (s *fakeStatisticStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeStatisticStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 99, nil
}

func (s *fakeStatisticStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeStatisticStore) StatisticToday(_ context.Context, _ int, _ time.Time) (StatisticContactSummary, error) {
	return s.today, nil
}

func (s *fakeStatisticStore) StatisticContactTrend(_ context.Context, _ int, employeeIDs []int, _ time.Time, _ time.Time) ([]StatisticContactDay, error) {
	s.contactTrendEmployeeIDs = append([]int{}, employeeIDs...)
	return s.contactTrend, nil
}

func (s *fakeStatisticStore) StatisticTopEmployees(_ context.Context, _ int, _ int) (int, []StatisticTopEmployee, error) {
	return s.topTotal, s.topList, nil
}

func (s *fakeStatisticStore) StatisticEmployeesByCorp(_ context.Context, _ int) ([]StatisticEmployee, error) {
	return s.employees, nil
}

func (s *fakeStatisticStore) StatisticEmployeesByIDs(_ context.Context, _ int, employeeIDs []int) ([]StatisticEmployee, error) {
	s.employeesByIDsArg = append([]int{}, employeeIDs...)
	return s.employeesByID, nil
}

func (s *fakeStatisticStore) StatisticEmployeeLocalSummary(_ context.Context, _ int, _ []int, _ time.Time, _ time.Time) (StatisticEmployeeMetric, error) {
	return s.localSummary, nil
}

func (s *fakeStatisticStore) StatisticEmployeeLocalMetricsByEmployee(_ context.Context, _ int, employeeIDs []int, _ time.Time, _ time.Time) (map[int]StatisticEmployeeMetric, error) {
	if s.localByEmployee != nil {
		return s.localByEmployee, nil
	}
	out := map[int]StatisticEmployeeMetric{}
	for _, employeeID := range employeeIDs {
		out[employeeID] = StatisticEmployeeMetric{}
	}
	return out, nil
}

func (s *fakeStatisticStore) StatisticEmployeeLocalTrend(_ context.Context, _ int, _ []int, _ time.Time, _ time.Time) ([]StatisticBehaviorData, error) {
	return s.localTrend, nil
}

func (s *fakeStatisticStore) RoomWelcomeCorpCredentialByID(_ context.Context, _ int) (RoomWelcomeCorpCredential, bool, error) {
	if s.credential.WXCorpID == "" || s.credential.ContactSecret == "" {
		return RoomWelcomeCorpCredential{}, false, nil
	}
	return s.credential, true, nil
}

type fakeStatisticBehaviorClient struct {
	behaviorByUser map[string][]StatisticBehaviorData
	calls          [][]string
}

func (c *fakeStatisticBehaviorClient) UserBehavior(_ context.Context, _ RoomWelcomeCorpCredential, userIDs []string, _ time.Time, _ time.Time) ([]StatisticBehaviorData, error) {
	c.calls = append(c.calls, append([]string{}, userIDs...))
	out := []StatisticBehaviorData{}
	for _, userID := range userIDs {
		out = append(out, c.behaviorByUser[userID]...)
	}
	return out, nil
}
