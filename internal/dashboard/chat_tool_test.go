package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatToolConfigReturnsAgentsToolsAndDomains(t *testing.T) {
	store := &fakeChatToolStore{
		users: map[int]User{1: {ID: 1}},
		agentsByCorp: map[int][]WorkAgent{
			7: {{ID: 11, CorpID: 7, Name: "侧边栏应用", SquareLogoURL: "https://example.com/logo.png"}},
		},
		tools: []ChatTool{
			{ID: 1, PageName: "客户画像", PageFlag: "customer"},
			{ID: 2, PageName: "素材库", PageFlag: "mediumGroup"},
		},
	}
	handler := NewChatToolConfigHandler(store, staticChatToolCache("7-99"), HeaderUserIDResolver{}, "http://sidebar.example.com/", "http://api.example.com/")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/chatTool/config", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastCorpID != 7 {
		t.Fatalf("lastCorpID = %d", store.lastCorpID)
	}

	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	domains := data["whiteDomains"].([]any)
	if domains[0] != "http://sidebar.example.com" || domains[1] != "http://api.example.com" {
		t.Fatalf("whiteDomains = %#v", domains)
	}
	agents := data["agents"].([]any)
	agent := agents[0].(map[string]any)
	if int(agent["id"].(float64)) != 11 || agent["name"] != "侧边栏应用" || agent["squareLogoUrl"] != "https://example.com/logo.png" {
		t.Fatalf("agent = %#v", agent)
	}
	tools := agent["chatTools"].([]any)
	firstTool := tools[0].(map[string]any)
	secondTool := tools[1].(map[string]any)
	if firstTool["pageFlag"] != "contact" || firstTool["pageUrl"] != "http://sidebar.example.com/contact?agentId=11" {
		t.Fatalf("first tool = %#v", firstTool)
	}
	if secondTool["pageFlag"] != "medium" || secondTool["pageUrl"] != "http://sidebar.example.com/medium?agentId=11" {
		t.Fatalf("second tool = %#v", secondTool)
	}
}

func TestChatToolConfigReturnsEmptyArrayWhenNoAgents(t *testing.T) {
	store := &fakeChatToolStore{users: map[int]User{1: {ID: 1}}}
	handler := NewChatToolConfigHandler(store, staticChatToolCache("7-99"), HeaderUserIDResolver{}, "http://sidebar.example.com", "http://api.example.com")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/chatTool/config", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if data := body["data"].([]any); len(data) != 0 {
		t.Fatalf("data = %#v", data)
	}
}

func TestChatToolConfigUsesPrincipalCorpWithoutLoginSelection(t *testing.T) {
	store := &fakeChatToolStore{users: map[int]User{1: {ID: 1}}}
	handler := NewChatToolConfigHandler(store, staticChatToolCache(""), HeaderUserIDResolver{}, "http://sidebar.example.com", "http://api.example.com")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/chatTool/config", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

type fakeChatToolStore struct {
	users        map[int]User
	agentsByCorp map[int][]WorkAgent
	tools        []ChatTool
	firstCorpID  int
	firstEmpID   int
	firstOK      bool
	lastCorpID   int
}

func (s *fakeChatToolStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeChatToolStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 0, nil
}

func (s *fakeChatToolStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return s.firstCorpID, s.firstEmpID, s.firstOK, nil
}

func (s *fakeChatToolStore) WorkAgentsByCorpID(_ context.Context, corpID int) ([]WorkAgent, error) {
	s.lastCorpID = corpID
	return s.agentsByCorp[corpID], nil
}

func (s *fakeChatToolStore) EnabledChatTools(_ context.Context) ([]ChatTool, error) {
	return s.tools, nil
}

type staticChatToolCache string

func (c staticChatToolCache) UserCorpCache(context.Context, int) (string, error) {
	return string(c), nil
}
