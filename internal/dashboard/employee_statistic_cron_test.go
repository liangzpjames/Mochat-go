package dashboard

import (
	"context"
	"testing"
	"time"
)

func TestEmployeeStatisticCronInsertsYesterdayBehavior(t *testing.T) {
	store := &fakeEmployeeStatisticStore{
		targets: []EmployeeStatisticTarget{
			{ID: 31, CorpID: 7, WXUserID: "go-user", WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		},
	}
	cache := &fakeEmployeeStatisticCache{}
	client := &fakeEmployeeStatisticClient{
		behavior: []StatisticBehaviorData{
			{ChatCnt: 9, MessageCnt: 11, ReplyPercentage: 0.83, AvgReplyTime: 25, NegativeFeedbackCnt: 2, NewApplyCnt: 4},
		},
	}
	cron := NewEmployeeStatisticCron(store, cache, client, nil)
	cron.now = func() time.Time { return time.Date(2026, 7, 4, 12, 0, 0, 0, time.Local) }

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !cache.set || cache.corpID != 7 || cache.employeeID != 31 {
		t.Fatalf("cache set = %v corp=%d employee=%d", cache.set, cache.corpID, cache.employeeID)
	}
	if client.wxCorpID != "ww-go" || client.contactSecret != "contact-secret" || len(client.userIDs) != 1 || client.userIDs[0] != "go-user" {
		t.Fatalf("client call = corp %q secret %q users %#v", client.wxCorpID, client.contactSecret, client.userIDs)
	}
	if client.start.Format("2006-01-02 15:04:05") != "2026-07-03 00:00:00" || client.end.Format("2006-01-02 15:04:05") != "2026-07-03 23:59:59" {
		t.Fatalf("range = %s - %s", client.start, client.end)
	}
	if len(store.records) != 1 {
		t.Fatalf("records = %#v", store.records)
	}
	record := store.records[0]
	if record.CorpID != 7 || record.EmployeeID != 31 || record.ChatCnt != 9 || record.MessageCnt != 11 || record.ReplyPercentage != 83 || record.NewApplyCnt != 4 || record.NewContactCnt != 4 || record.AvgReplyTime != 25 || record.NegativeFeedbackCnt != 2 {
		t.Fatalf("record = %+v", record)
	}
	if record.SynTime.Format("2006-01-02 15:04:05") != "2026-07-03 00:00:00" {
		t.Fatalf("synTime = %s", record.SynTime)
	}
}

func TestEmployeeStatisticCronSkipsCachedEmployee(t *testing.T) {
	store := &fakeEmployeeStatisticStore{
		targets: []EmployeeStatisticTarget{
			{ID: 31, CorpID: 7, WXUserID: "go-user", WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		},
	}
	cache := &fakeEmployeeStatisticCache{applied: true}
	client := &fakeEmployeeStatisticClient{}
	cron := NewEmployeeStatisticCron(store, cache, client, nil)

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.called {
		t.Fatalf("client should not be called")
	}
	if len(store.records) != 0 {
		t.Fatalf("records = %#v", store.records)
	}
}

func TestEmployeeStatisticCronRequiresDependencies(t *testing.T) {
	cron := NewEmployeeStatisticCron(nil, nil, nil, nil)
	err := cron.RunOnce(context.Background())
	if err == nil || err.Error() != "employeeStatistic cron dependencies are not configured" {
		t.Fatalf("err = %v", err)
	}
}

type fakeEmployeeStatisticStore struct {
	targets          []EmployeeStatisticTarget
	records          []EmployeeStatisticRecord
	tenantByCorp     map[int]int
	corpIDsByTenant  map[int][]int
	status           SaaSQuotaStatus
	refreshTenantID  int
	refreshMetric    string
	statusTenantID   int
	statusMetric     string
	statusAdditional int64
}

func (s *fakeEmployeeStatisticStore) EmployeeStatisticTargets(context.Context) ([]EmployeeStatisticTarget, error) {
	return s.targets, nil
}

func (s *fakeEmployeeStatisticStore) InsertEmployeeStatistic(_ context.Context, record EmployeeStatisticRecord) error {
	s.records = append(s.records, record)
	return nil
}

func (s *fakeEmployeeStatisticStore) TenantIDByCorpID(_ context.Context, corpID int) (int, error) {
	return s.tenantByCorp[corpID], nil
}

func (s *fakeEmployeeStatisticStore) CorpIDsByTenant(_ context.Context, tenantID int) ([]int, error) {
	return s.corpIDsByTenant[tenantID], nil
}

func (s *fakeEmployeeStatisticStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}

func (s *fakeEmployeeStatisticStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.statusTenantID = tenantID
	s.statusMetric = metric
	s.statusAdditional = additional
	return s.status, nil
}

type fakeEmployeeStatisticCache struct {
	applied    bool
	set        bool
	corpID     int
	employeeID int
	startUnix  int64
	ttl        time.Duration
}

func (c *fakeEmployeeStatisticCache) EmployeeStatisticApplied(_ context.Context, corpID int, employeeID int, startUnix int64) (bool, error) {
	c.corpID = corpID
	c.employeeID = employeeID
	c.startUnix = startUnix
	return c.applied, nil
}

func (c *fakeEmployeeStatisticCache) SetEmployeeStatisticApplied(_ context.Context, corpID int, employeeID int, startUnix int64, ttl time.Duration) error {
	c.set = true
	c.corpID = corpID
	c.employeeID = employeeID
	c.startUnix = startUnix
	c.ttl = ttl
	return nil
}

type fakeEmployeeStatisticClient struct {
	behavior      []StatisticBehaviorData
	called        bool
	wxCorpID      string
	contactSecret string
	userIDs       []string
	start         time.Time
	end           time.Time
}

func (c *fakeEmployeeStatisticClient) UserBehavior(_ context.Context, credential RoomWelcomeCorpCredential, userIDs []string, start time.Time, end time.Time) ([]StatisticBehaviorData, error) {
	c.called = true
	c.wxCorpID = credential.WXCorpID
	c.contactSecret = credential.ContactSecret
	c.userIDs = append([]string{}, userIDs...)
	c.start = start
	c.end = end
	return c.behavior, nil
}
