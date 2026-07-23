package dashboard

import (
	"context"
	"fmt"
	"testing"
)

func TestWorkAgentSyncCronRefreshesAgents(t *testing.T) {
	store := &fakeWorkAgentSyncStore{
		agents: []WorkAgentSyncItem{
			{ID: 7, CorpID: 3, WXCorpID: "ww-corp", WXAgentID: "100001", WXSecret: "agent-secret"},
		},
	}
	client := &fakeWorkAgentSyncClient{
		detail: WorkAgentDetail{
			Name:               "客户运营",
			SquareLogoURL:      "https://example.com/logo.png",
			Description:        "更新后的应用",
			Close:              0,
			RedirectDomain:     "go.example.com",
			ReportLocationFlag: 1,
			IsReportEnter:      1,
			HomeURL:            "https://example.com/home",
		},
	}
	cron := NewWorkAgentSyncCron(store, client, nil)

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.wxCorpID != "ww-corp" || client.wxSecret != "agent-secret" || client.wxAgentID != "100001" {
		t.Fatalf("client call = corp %q secret %q agent %q", client.wxCorpID, client.wxSecret, client.wxAgentID)
	}
	if store.updatedAgentID != 7 || store.updatedDetail.Name != "客户运营" || store.updatedDetail.HomeURL != "https://example.com/home" {
		t.Fatalf("updated agent=%d detail=%+v", store.updatedAgentID, store.updatedDetail)
	}
}

func TestWorkAgentSyncCronSkipsMissingCredential(t *testing.T) {
	store := &fakeWorkAgentSyncStore{
		agents: []WorkAgentSyncItem{
			{ID: 7, CorpID: 3, WXCorpID: "ww-corp", WXAgentID: "", WXSecret: "agent-secret"},
		},
	}
	client := &fakeWorkAgentSyncClient{}
	cron := NewWorkAgentSyncCron(store, client, nil)

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.called {
		t.Fatalf("client should not be called")
	}
	if store.updatedAgentID != 0 {
		t.Fatalf("updatedAgentID = %d", store.updatedAgentID)
	}
}

func TestWorkAgentSyncCronRequiresDependencies(t *testing.T) {
	cron := NewWorkAgentSyncCron(nil, nil, nil)
	err := cron.RunOnce(context.Background())
	if err == nil || err.Error() != "pullAgent cron dependencies are not configured" {
		t.Fatalf("err = %v", err)
	}
}

type fakeWorkAgentSyncStore struct {
	agents         []WorkAgentSyncItem
	err            error
	updateErr      error
	updateOK       bool
	updatedAgentID int
	updatedDetail  WorkAgentDetail
}

func (s *fakeWorkAgentSyncStore) WorkAgentsForSync(context.Context) ([]WorkAgentSyncItem, error) {
	return s.agents, s.err
}

func (s *fakeWorkAgentSyncStore) UpdateWorkAgentDetail(_ context.Context, agentID int, detail WorkAgentDetail) (bool, error) {
	if s.updateErr != nil {
		return false, s.updateErr
	}
	s.updatedAgentID = agentID
	s.updatedDetail = detail
	if s.updateOK {
		return true, nil
	}
	return true, nil
}

type fakeWorkAgentSyncClient struct {
	detail    WorkAgentDetail
	err       error
	called    bool
	wxCorpID  string
	wxSecret  string
	wxAgentID string
}

func (c *fakeWorkAgentSyncClient) AgentDetail(_ context.Context, credential RoomWelcomeCorpCredential, wxSecret string, wxAgentID string) (WorkAgentDetail, error) {
	c.called = true
	c.wxCorpID = credential.WXCorpID
	c.wxSecret = wxSecret
	c.wxAgentID = wxAgentID
	if c.err != nil {
		return WorkAgentDetail{}, c.err
	}
	if c.detail.Name == "" {
		return WorkAgentDetail{}, fmt.Errorf("missing detail")
	}
	return c.detail, nil
}
