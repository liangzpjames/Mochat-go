package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

type fakeKBRepo struct {
	items []ports.KnowledgeBase
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

func (f *fakeKBRepo) Create(_ context.Context, v ports.KnowledgeBase) (ports.KnowledgeBase, error) {
	f.items = append(f.items, v)
	return v, nil
}

func (f *fakeKBRepo) Update(_ context.Context, v ports.KnowledgeBase) (ports.KnowledgeBase, error) {
	for i := range f.items {
		if f.items[i].ID == v.ID && f.items[i].TenantID == v.TenantID && f.items[i].CorpID == v.CorpID {
			f.items[i] = v
			return v, nil
		}
	}
	return ports.KnowledgeBase{}, errors.New("not found")
}

func (f *fakeKBRepo) Delete(_ context.Context, tenantID, corpID int64, id string) error {
	for i := range f.items {
		if f.items[i].ID == id && f.items[i].TenantID == tenantID && f.items[i].CorpID == corpID {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}

type fakeAgentRepo struct {
	items []ports.Agent
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

func (f *fakeAgentRepo) Create(_ context.Context, v ports.Agent) (ports.Agent, error) {
	f.items = append(f.items, v)
	return v, nil
}

func (f *fakeAgentRepo) Update(_ context.Context, v ports.Agent) (ports.Agent, error) {
	for i := range f.items {
		if f.items[i].ID == v.ID && f.items[i].TenantID == v.TenantID && f.items[i].CorpID == v.CorpID {
			f.items[i] = v
			return v, nil
		}
	}
	return ports.Agent{}, errors.New("not found")
}

func (f *fakeAgentRepo) Delete(_ context.Context, tenantID, corpID int64, id string) error {
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
	return NewKnowledgeBaseHandler(&fakeKBRepo{}, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, authorizer, func() string { return "kb-1" })
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
	handler := NewKnowledgeBaseHandler(repo, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "x" })
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
	rec := perform(handler, http.MethodPost, "/dashboard/ai-settings/knowledge-bases?corpId=2", `{"name":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
}

func TestKnowledgeBaseCreatePersistsScopedRecord(t *testing.T) {
	repo := &fakeKBRepo{}
	handler := NewKnowledgeBaseHandler(repo, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "kb-1" })
	rec := perform(handler, http.MethodPost, "/dashboard/ai-settings/knowledge-bases?corpId=2", `{"name":"售后话术库","description":"test"}`)
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
	handler := NewKnowledgeBaseHandler(repo, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "x" })
	rec := perform(handler, http.MethodPut, "/dashboard/ai-settings/knowledge-bases/kb-1?corpId=2", `{"name":"改名"}`)
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
	unauth := NewKnowledgeBaseHandler(&fakeKBRepo{}, fakeResolver{err: errors.New("no principal")}, nil, func() string { return "x" })
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
	handler := NewAgentHandler(repo, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "agent-1" })
	rec := perform(handler, http.MethodPost, "/dashboard/ai-settings/agents?corpId=2", `{"name":"", "knowledgeBaseIds":["kb-1"]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty name code = %d, want 400", rec.Code)
	}
	rec = perform(handler, http.MethodPost, "/dashboard/ai-settings/agents?corpId=2", `{"name":"智能客服","knowledgeBaseIds":["kb-1"]}`)
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
