package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

type fakeKBRepo struct {
	items       []ports.KnowledgeBase
	createErr   error
	getByIDsErr error
	updateErr   error
	deleteErr   error
	deleteActor int64
}

func (f *fakeKBRepo) List(_ context.Context, tenantID, corpID int64) ([]ports.KnowledgeBase, error) {
	result := make([]ports.KnowledgeBase, 0)
	for _, item := range f.items {
		if item.TenantID == tenantID && item.CorpID == corpID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (f *fakeKBRepo) GetByIDs(_ context.Context, tenantID, corpID int64, ids []string) ([]ports.KnowledgeBase, error) {
	if f.getByIDsErr != nil {
		return nil, f.getByIDsErr
	}
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	result := make([]ports.KnowledgeBase, 0, len(ids))
	for _, item := range f.items {
		if item.TenantID == tenantID && item.CorpID == corpID {
			if _, ok := wanted[item.ID]; ok {
				result = append(result, item)
			}
		}
	}
	return result, nil
}

func (f *fakeKBRepo) Create(_ context.Context, v ports.KnowledgeBase) (ports.KnowledgeBase, error) {
	if f.createErr != nil {
		return ports.KnowledgeBase{}, f.createErr
	}
	f.items = append(f.items, v)
	return v, nil
}

func (f *fakeKBRepo) Update(_ context.Context, v ports.KnowledgeBase) (ports.KnowledgeBase, error) {
	if f.updateErr != nil {
		return ports.KnowledgeBase{}, f.updateErr
	}
	for i := range f.items {
		if f.items[i].ID == v.ID && f.items[i].TenantID == v.TenantID && f.items[i].CorpID == v.CorpID {
			f.items[i] = v
			return v, nil
		}
	}
	return ports.KnowledgeBase{}, errors.New("not found")
}

func (f *fakeKBRepo) Delete(_ context.Context, tenantID, corpID, actorUserID int64, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleteActor = actorUserID
	for i := range f.items {
		if f.items[i].ID == id && f.items[i].TenantID == tenantID && f.items[i].CorpID == corpID {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}

type fakeAgentRepo struct {
	items       []ports.Agent
	createErr   error
	updateErr   error
	deleteErr   error
	deleteActor int64
}

func (f *fakeAgentRepo) List(_ context.Context, tenantID, corpID int64) ([]ports.Agent, error) {
	result := make([]ports.Agent, 0)
	for _, item := range f.items {
		if item.TenantID == tenantID && item.CorpID == corpID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (f *fakeAgentRepo) ListReferencingKnowledgeBase(_ context.Context, tenantID, corpID int64, knowledgeBaseID string) ([]ports.Agent, error) {
	result := make([]ports.Agent, 0)
	for _, item := range f.items {
		if item.TenantID != tenantID || item.CorpID != corpID {
			continue
		}
		for _, id := range item.KnowledgeBaseIDs {
			if id == knowledgeBaseID {
				result = append(result, item)
				break
			}
		}
	}
	return result, nil
}

func (f *fakeAgentRepo) Create(_ context.Context, v ports.Agent) (ports.Agent, error) {
	if f.createErr != nil {
		return ports.Agent{}, f.createErr
	}
	f.items = append(f.items, v)
	return v, nil
}

func (f *fakeAgentRepo) Update(_ context.Context, v ports.Agent) (ports.Agent, error) {
	if f.updateErr != nil {
		return ports.Agent{}, f.updateErr
	}
	for i := range f.items {
		if f.items[i].ID == v.ID && f.items[i].TenantID == v.TenantID && f.items[i].CorpID == v.CorpID {
			f.items[i] = v
			return v, nil
		}
	}
	return ports.Agent{}, errors.New("not found")
}

func (f *fakeAgentRepo) Delete(_ context.Context, tenantID, corpID, actorUserID int64, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleteActor = actorUserID
	for i := range f.items {
		if f.items[i].ID == id && f.items[i].TenantID == tenantID && f.items[i].CorpID == corpID {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}

type fakeResolver struct {
	principal Principal
	err       error
}

func (f fakeResolver) Resolve(_ *http.Request) (Principal, error) {
	return f.principal, f.err
}

type fakeAuthorizer struct {
	err error
}

func (f fakeAuthorizer) Authorize(_ context.Context, _ Principal, _ int64, _ string) error {
	return f.err
}

func newTestKBs(authorizer Authorizer) *KnowledgeBaseHandler {
	return NewKnowledgeBaseHandler(&fakeKBRepo{}, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, authorizer, func() string { return "kb-1" })
}

func perform(handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func envelopeData(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid envelope json: %v", err)
	}
	return payload
}

func TestKnowledgeBaseListScopesTenantAndCorp(t *testing.T) {
	repo := &fakeKBRepo{}
	repo.items = append(repo.items,
		ports.KnowledgeBase{ID: "a", TenantID: 1, CorpID: 2},
		ports.KnowledgeBase{ID: "b", TenantID: 9, CorpID: 2},
	)
	handler := NewKnowledgeBaseHandler(repo, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "x" })
	rec := perform(handler, http.MethodGet, "/dashboard/ai-settings/knowledge-bases?corpId=2", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	payload := envelopeData(t, rec)
	if payload["code"].(float64) != http.StatusOK {
		t.Fatalf("envelope code = %v", payload["code"])
	}
	items, ok := payload["data"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("data = %#v, want exactly 1 scoped item", payload["data"])
	}
}

func TestKnowledgeBaseCreateRequiresName(t *testing.T) {
	handler := newTestKBs(nil)
	rec := perform(handler, http.MethodPost, "/dashboard/ai-settings/knowledge-bases?corpId=2", `{"name":"","status":1}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
}

func TestKnowledgeBaseCreatePersistsScopedRecord(t *testing.T) {
	repo := &fakeKBRepo{}
	handler := NewKnowledgeBaseHandler(repo, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "kb-1" })
	rec := perform(handler, http.MethodPost, "/dashboard/ai-settings/knowledge-bases?corpId=2", `{"name":"售后话术库","description":"test","status":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.items) != 1 || repo.items[0].TenantID != 1 || repo.items[0].CorpID != 2 {
		t.Fatalf("stored item = %#v, want tenant=1 corp=2", repo.items)
	}
}

func TestKnowledgeBaseUpdateAndDelete(t *testing.T) {
	repo := &fakeKBRepo{}
	repo.items = append(repo.items, ports.KnowledgeBase{ID: "kb-1", TenantID: 1, CorpID: 2})
	handler := NewKnowledgeBaseHandler(repo, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "x" })
	rec := perform(handler, http.MethodPut, "/dashboard/ai-settings/knowledge-bases/kb-1?corpId=2", `{"name":"改名","status":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update code = %d, want 200", rec.Code)
	}
	rec = perform(handler, http.MethodDelete, "/dashboard/ai-settings/knowledge-bases/kb-1?corpId=2", "")
	if rec.Code != http.StatusOK || len(repo.items) != 0 {
		t.Fatalf("delete code = %d items=%d, want 200 and empty", rec.Code, len(repo.items))
	}
}

func TestKnowledgeBaseUsesPrincipalCorpWhenClientOmitsCorpID(t *testing.T) {
	handler := newTestKBs(nil)
	rec := perform(handler, http.MethodGet, "/dashboard/ai-settings/knowledge-bases", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
}

func TestKnowledgeBaseUnauthorizedAndForbidden(t *testing.T) {
	unauth := NewKnowledgeBaseHandler(&fakeKBRepo{}, &fakeAgentRepo{}, fakeResolver{err: errors.New("no principal")}, nil, func() string { return "x" })
	rec := perform(unauth, http.MethodGet, "/dashboard/ai-settings/knowledge-bases?corpId=2", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized code = %d, want 401", rec.Code)
	}
	denied := newTestKBs(fakeAuthorizer{err: errors.New("denied")})
	rec = perform(denied, http.MethodGet, "/dashboard/ai-settings/knowledge-bases?corpId=2", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("forbidden code = %d, want 403", rec.Code)
	}
}

func TestAgentCRUDScopesAndValidates(t *testing.T) {
	repo := &fakeAgentRepo{}
	handler := NewAgentHandler(repo, &fakeKBRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" })
	rec := perform(handler, http.MethodPost, "/dashboard/ai-settings/agents?corpId=2", `{"name":"", "knowledgeBaseIds":["kb-1"],"status":1}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty name code = %d, want 400", rec.Code)
	}
	rec = perform(handler, http.MethodPost, "/dashboard/ai-settings/agents?corpId=2", `{"name":"智能客服","knowledgeBaseIds":[],"status":1}`)
	if rec.Code != http.StatusOK || len(repo.items) != 1 || repo.items[0].TenantID != 1 {
		t.Fatalf("create code=%d items=%#v", rec.Code, repo.items)
	}
	rec = perform(handler, http.MethodGet, "/dashboard/ai-settings/agents?corpId=2", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list code = %d, want 200", rec.Code)
	}
	payload := envelopeData(t, rec)
	if len(payload["data"].([]any)) != 1 {
		t.Fatalf("list data = %#v", payload["data"])
	}
}

func TestKnowledgeBasePersistsDisabledStatus(t *testing.T) {
	repo := &fakeKBRepo{}
	handler := NewKnowledgeBaseHandler(repo, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "kb-1" })

	response := perform(handler, http.MethodPost, "/dashboard/ai-settings/knowledge-bases", `{"name":"售后库","description":"","documentCount":0,"status":0}`)
	if response.Code != http.StatusOK || len(repo.items) != 1 || repo.items[0].Status != 0 {
		t.Fatalf("response = %s, stored = %#v, want persisted status 0", response.Body.String(), repo.items)
	}
}

func TestAISettingsRejectsInvalidStatus(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
		target  string
		body    string
	}{
		{
			name:    "knowledge base",
			handler: NewKnowledgeBaseHandler(&fakeKBRepo{}, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "kb-1" }),
			target:  "/dashboard/ai-settings/knowledge-bases",
			body:    `{"name":"售后库","status":2}`,
		},
		{
			name:    "agent",
			handler: NewAgentHandler(&fakeAgentRepo{}, &fakeKBRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" }),
			target:  "/dashboard/ai-settings/agents",
			body:    `{"name":"客服助手","knowledgeBaseIds":[],"status":-1}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := perform(test.handler, http.MethodPost, test.target, test.body)
			payload := envelopeData(t, response)
			if response.Code != http.StatusBadRequest || payload["msg"] != machineCodeInvalidStatus {
				t.Fatalf("code = %d, want 400; body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestAISettingsRejectsMissingStatusWithMachineCode(t *testing.T) {
	handler := NewAgentHandler(&fakeAgentRepo{}, &fakeKBRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" })

	response := perform(handler, http.MethodPost, "/dashboard/ai-settings/agents", `{"name":"客服助手","knowledgeBaseIds":[]}`)
	payload := envelopeData(t, response)
	if response.Code != http.StatusBadRequest || payload["msg"] != machineCodeInvalidStatus {
		t.Fatalf("response = %s", response.Body.String())
	}
}

func TestAISettingsValidatesNameRuneLengthAndDocumentCount(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
		target  string
		body    string
	}{
		{
			name:    "knowledge base name too short",
			handler: NewKnowledgeBaseHandler(&fakeKBRepo{}, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "kb-1" }),
			target:  "/dashboard/ai-settings/knowledge-bases",
			body:    `{"name":"你","documentCount":0,"status":1}`,
		},
		{
			name:    "knowledge base name too long in runes",
			handler: NewKnowledgeBaseHandler(&fakeKBRepo{}, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "kb-1" }),
			target:  "/dashboard/ai-settings/knowledge-bases",
			body:    `{"name":"` + strings.Repeat("你", 129) + `","documentCount":0,"status":1}`,
		},
		{
			name:    "agent name too short",
			handler: NewAgentHandler(&fakeAgentRepo{}, &fakeKBRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" }),
			target:  "/dashboard/ai-settings/agents",
			body:    `{"name":"你","knowledgeBaseIds":[],"status":1}`,
		},
		{
			name:    "agent name too long in runes",
			handler: NewAgentHandler(&fakeAgentRepo{}, &fakeKBRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" }),
			target:  "/dashboard/ai-settings/agents",
			body:    `{"name":"` + strings.Repeat("你", 129) + `","knowledgeBaseIds":[],"status":1}`,
		},
		{
			name:    "negative document count",
			handler: NewKnowledgeBaseHandler(&fakeKBRepo{}, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "kb-1" }),
			target:  "/dashboard/ai-settings/knowledge-bases",
			body:    `{"name":"售后库","documentCount":-1,"status":1}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := perform(test.handler, http.MethodPost, test.target, test.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("code = %d, want 400; body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestAgentPersistsStableDeduplicatedKnowledgeBaseIDs(t *testing.T) {
	knowledgeBases := &fakeKBRepo{items: []ports.KnowledgeBase{
		{ID: "kb-1", TenantID: 1, CorpID: 2},
		{ID: "kb-2", TenantID: 1, CorpID: 2},
	}}
	agents := &fakeAgentRepo{}
	handler := NewAgentHandler(agents, knowledgeBases, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" })

	response := perform(handler, http.MethodPost, "/dashboard/ai-settings/agents", `{"name":"客服助手","knowledgeBaseIds":["kb-2","kb-1","kb-2"],"status":1}`)
	if response.Code != http.StatusOK || len(agents.items) != 1 || strings.Join(agents.items[0].KnowledgeBaseIDs, ",") != "kb-2,kb-1" {
		t.Fatalf("response = %s, stored = %#v", response.Body.String(), agents.items)
	}
}

func TestAgentRejectsBlankKnowledgeBaseID(t *testing.T) {
	handler := NewAgentHandler(&fakeAgentRepo{}, &fakeKBRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" })

	response := perform(handler, http.MethodPost, "/dashboard/ai-settings/agents", `{"name":"客服助手","knowledgeBaseIds":[" "],"status":1}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400; body = %s", response.Code, response.Body.String())
	}
}

func TestAISettingsHandlesMissingCrossRepositoryWithoutPanic(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
		method  string
		target  string
		body    string
	}{
		{
			name:    "agent knowledge base validation",
			handler: NewAgentHandler(&fakeAgentRepo{}, nil, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" }),
			method:  http.MethodPost,
			target:  "/dashboard/ai-settings/agents",
			body:    `{"name":"客服助手","knowledgeBaseIds":["kb-1"],"status":1}`,
		},
		{
			name:    "knowledge base reference lookup",
			handler: NewKnowledgeBaseHandler(&fakeKBRepo{}, nil, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "kb-1" }),
			method:  http.MethodDelete,
			target:  "/dashboard/ai-settings/knowledge-bases/kb-1",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := perform(test.handler, test.method, test.target, test.body)
			payload := envelopeData(t, response)
			if response.Code != http.StatusInternalServerError || payload["msg"] != machineCodeStorageFailure {
				t.Fatalf("response = %s", response.Body.String())
			}
		})
	}
}

func TestAgentPersistsDisabledStatusAndKnowledgeBaseUpdatePersistsDisabledStatus(t *testing.T) {
	knowledgeBases := &fakeKBRepo{items: []ports.KnowledgeBase{{ID: "kb-1", TenantID: 1, CorpID: 2, Status: 1}}}
	agents := &fakeAgentRepo{}
	agentHandler := NewAgentHandler(agents, knowledgeBases, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" })
	knowledgeBaseHandler := NewKnowledgeBaseHandler(knowledgeBases, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "kb-1" })

	response := perform(agentHandler, http.MethodPost, "/dashboard/ai-settings/agents", `{"name":"客服助手","knowledgeBaseIds":[],"status":0}`)
	if response.Code != http.StatusOK || len(agents.items) != 1 || agents.items[0].Status != 0 {
		t.Fatalf("agent response = %s, stored = %#v", response.Body.String(), agents.items)
	}
	response = perform(agentHandler, http.MethodPut, "/dashboard/ai-settings/agents/agent-1", `{"name":"客服助手","knowledgeBaseIds":[],"status":0}`)
	if response.Code != http.StatusOK || agents.items[0].Status != 0 {
		t.Fatalf("agent update response = %s, stored = %#v", response.Body.String(), agents.items)
	}
	response = perform(knowledgeBaseHandler, http.MethodPut, "/dashboard/ai-settings/knowledge-bases/kb-1", `{"name":"售后库","documentCount":0,"status":0}`)
	if response.Code != http.StatusOK || knowledgeBases.items[0].Status != 0 {
		t.Fatalf("knowledge base response = %s, stored = %#v", response.Body.String(), knowledgeBases.items)
	}
}

func TestAISettingsMapsMissingRecordsAndStorageFailures(t *testing.T) {
	tests := []struct {
		name        string
		handler     http.Handler
		method      string
		target      string
		body        string
		statusCode  int
		machineCode string
	}{
		{
			name:        "knowledge base update missing",
			handler:     NewKnowledgeBaseHandler(&fakeKBRepo{updateErr: ports.ErrNotFound}, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "kb-1" }),
			method:      http.MethodPut,
			target:      "/dashboard/ai-settings/knowledge-bases/kb-1",
			body:        `{"name":"售后库","documentCount":0,"status":1}`,
			statusCode:  http.StatusNotFound,
			machineCode: machineCodeNotFound,
		},
		{
			name:        "knowledge base delete storage failure",
			handler:     NewKnowledgeBaseHandler(&fakeKBRepo{deleteErr: errors.New("database unavailable")}, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "kb-1" }),
			method:      http.MethodDelete,
			target:      "/dashboard/ai-settings/knowledge-bases/kb-1",
			statusCode:  http.StatusInternalServerError,
			machineCode: machineCodeStorageFailure,
		},
		{
			name:        "agent update missing",
			handler:     NewAgentHandler(&fakeAgentRepo{updateErr: ports.ErrNotFound}, &fakeKBRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" }),
			method:      http.MethodPut,
			target:      "/dashboard/ai-settings/agents/agent-1",
			body:        `{"name":"客服助手","knowledgeBaseIds":[],"status":1}`,
			statusCode:  http.StatusNotFound,
			machineCode: machineCodeNotFound,
		},
		{
			name:        "agent delete storage failure",
			handler:     NewAgentHandler(&fakeAgentRepo{deleteErr: errors.New("database unavailable")}, &fakeKBRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" }),
			method:      http.MethodDelete,
			target:      "/dashboard/ai-settings/agents/agent-1",
			statusCode:  http.StatusInternalServerError,
			machineCode: machineCodeStorageFailure,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := perform(test.handler, test.method, test.target, test.body)
			payload := envelopeData(t, response)
			if response.Code != test.statusCode || payload["msg"] != test.machineCode {
				t.Fatalf("response = %s", response.Body.String())
			}
		})
	}
}

func TestAISettingsMapsCreateStorageFailures(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
		target  string
		body    string
	}{
		{
			name:    "knowledge base",
			handler: NewKnowledgeBaseHandler(&fakeKBRepo{createErr: errors.New("database unavailable")}, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "kb-1" }),
			target:  "/dashboard/ai-settings/knowledge-bases",
			body:    `{"name":"售后库","documentCount":0,"status":1}`,
		},
		{
			name:    "agent",
			handler: NewAgentHandler(&fakeAgentRepo{createErr: errors.New("database unavailable")}, &fakeKBRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" }),
			target:  "/dashboard/ai-settings/agents",
			body:    `{"name":"客服助手","knowledgeBaseIds":[],"status":1}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := perform(test.handler, http.MethodPost, test.target, test.body)
			payload := envelopeData(t, response)
			if response.Code != http.StatusInternalServerError || payload["msg"] != machineCodeStorageFailure {
				t.Fatalf("response = %s", response.Body.String())
			}
		})
	}
}

func TestAgentDistinguishesKnowledgeBaseLookupFailureFromInvalidIDs(t *testing.T) {
	tests := []struct {
		name           string
		knowledgeBases *fakeKBRepo
		statusCode     int
		machineCode    string
	}{
		{
			name:           "storage failure",
			knowledgeBases: &fakeKBRepo{getByIDsErr: errors.New("database unavailable")},
			statusCode:     http.StatusInternalServerError,
			machineCode:    machineCodeStorageFailure,
		},
		{
			name:           "missing knowledge base",
			knowledgeBases: &fakeKBRepo{},
			statusCode:     http.StatusBadRequest,
			machineCode:    machineCodeKnowledgeBaseInvalid,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewAgentHandler(&fakeAgentRepo{}, test.knowledgeBases, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" })
			response := perform(handler, http.MethodPost, "/dashboard/ai-settings/agents", `{"name":"客服助手","knowledgeBaseIds":["kb-1"],"status":1}`)
			payload := envelopeData(t, response)
			if response.Code != test.statusCode || payload["msg"] != test.machineCode {
				t.Fatalf("response = %s", response.Body.String())
			}
		})
	}
}

func TestAISettingsValidatesDescriptionRuneLength(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.Handler
		target     string
		body       string
		statusCode int
	}{
		{
			name:       "knowledge base accepts 512 runes",
			handler:    NewKnowledgeBaseHandler(&fakeKBRepo{}, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "kb-1" }),
			target:     "/dashboard/ai-settings/knowledge-bases",
			body:       `{"name":"售后库","description":"` + strings.Repeat("你", 512) + `","documentCount":0,"status":1}`,
			statusCode: http.StatusOK,
		},
		{
			name:       "knowledge base rejects 513 runes",
			handler:    NewKnowledgeBaseHandler(&fakeKBRepo{}, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "kb-1" }),
			target:     "/dashboard/ai-settings/knowledge-bases",
			body:       `{"name":"售后库","description":"` + strings.Repeat("你", 513) + `","documentCount":0,"status":1}`,
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "agent accepts 512 runes",
			handler:    NewAgentHandler(&fakeAgentRepo{}, &fakeKBRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" }),
			target:     "/dashboard/ai-settings/agents",
			body:       `{"name":"客服助手","description":"` + strings.Repeat("你", 512) + `","knowledgeBaseIds":[],"status":1}`,
			statusCode: http.StatusOK,
		},
		{
			name:       "agent rejects 513 runes",
			handler:    NewAgentHandler(&fakeAgentRepo{}, &fakeKBRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" }),
			target:     "/dashboard/ai-settings/agents",
			body:       `{"name":"客服助手","description":"` + strings.Repeat("你", 513) + `","knowledgeBaseIds":[],"status":1}`,
			statusCode: http.StatusBadRequest,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := perform(test.handler, http.MethodPost, test.target, test.body)
			if response.Code != test.statusCode {
				t.Fatalf("code = %d, want %d; body = %s", response.Code, test.statusCode, response.Body.String())
			}
			if test.statusCode == http.StatusBadRequest && envelopeData(t, response)["msg"] != machineCodeDescriptionInvalid {
				t.Fatalf("response = %s", response.Body.String())
			}
		})
	}
}

func TestAgentRejectsKnowledgeBaseOutsidePrincipalCorp(t *testing.T) {
	knowledgeBases := &fakeKBRepo{items: []ports.KnowledgeBase{{ID: "other-corp", TenantID: 1, CorpID: 99}}}
	handler := NewAgentHandler(&fakeAgentRepo{}, knowledgeBases, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" })

	response := perform(handler, http.MethodPost, "/dashboard/ai-settings/agents", `{"name":"客服助手","knowledgeBaseIds":["other-corp"],"status":1}`)
	payload := envelopeData(t, response)
	if response.Code != http.StatusBadRequest || payload["msg"] != "AI_SETTINGS_KNOWLEDGE_BASE_INVALID" {
		t.Fatalf("response = %s", response.Body.String())
	}
}

func TestKnowledgeBaseDeleteRejectsReferencedRecord(t *testing.T) {
	knowledgeBases := &fakeKBRepo{items: []ports.KnowledgeBase{{ID: "kb-1", TenantID: 1, CorpID: 2}}}
	agents := &fakeAgentRepo{items: []ports.Agent{{ID: "agent-1", TenantID: 1, CorpID: 2, KnowledgeBaseIDs: []string{"kb-1"}}}}
	handler := NewKnowledgeBaseHandler(knowledgeBases, agents, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "x" })

	response := perform(handler, http.MethodDelete, "/dashboard/ai-settings/knowledge-bases/kb-1", "")
	payload := envelopeData(t, response)
	if response.Code != http.StatusConflict || payload["msg"] != machineCodeKnowledgeBaseReferenced || len(knowledgeBases.items) != 1 {
		t.Fatalf("code = %d items = %#v, want 409 and unchanged knowledge base", response.Code, knowledgeBases.items)
	}
}

func TestAISettingsDeleteUsesPrincipalAsAuditActor(t *testing.T) {
	principal := fakeResolver{principal: Principal{UserID: 17, TenantID: 1, CorpID: 2}}
	t.Run("knowledge base", func(t *testing.T) {
		knowledgeBases := &fakeKBRepo{items: []ports.KnowledgeBase{{ID: "kb-1", TenantID: 1, CorpID: 2}}}
		handler := NewKnowledgeBaseHandler(knowledgeBases, &fakeAgentRepo{}, principal, nil, func() string { return "x" })
		response := perform(handler, http.MethodDelete, "/dashboard/ai-settings/knowledge-bases/kb-1", "")
		if response.Code != http.StatusOK || knowledgeBases.deleteActor != 17 {
			t.Fatalf("response = %s, actor = %d", response.Body.String(), knowledgeBases.deleteActor)
		}
	})
	t.Run("agent", func(t *testing.T) {
		agents := &fakeAgentRepo{items: []ports.Agent{{ID: "agent-1", TenantID: 1, CorpID: 2}}}
		handler := NewAgentHandler(agents, &fakeKBRepo{}, principal, nil, func() string { return "x" })
		response := perform(handler, http.MethodDelete, "/dashboard/ai-settings/agents/agent-1", "")
		if response.Code != http.StatusOK || agents.deleteActor != 17 {
			t.Fatalf("response = %s, actor = %d", response.Body.String(), agents.deleteActor)
		}
	})
}

func TestAISettingsRejectsUnknownOrTrailingJSON(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
		target  string
		body    string
	}{
		{
			name:    "unknown field",
			handler: NewKnowledgeBaseHandler(&fakeKBRepo{}, &fakeAgentRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "kb-1" }),
			target:  "/dashboard/ai-settings/knowledge-bases",
			body:    `{"name":"售后库","status":1,"unexpected":true}`,
		},
		{
			name:    "trailing document",
			handler: NewAgentHandler(&fakeAgentRepo{}, &fakeKBRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" }),
			target:  "/dashboard/ai-settings/agents",
			body:    `{"name":"客服助手","knowledgeBaseIds":[],"status":1}{}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := perform(test.handler, http.MethodPost, test.target, test.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("code = %d, want 400; body = %s", response.Code, response.Body.String())
			}
		})
	}
}
