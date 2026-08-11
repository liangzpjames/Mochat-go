package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type insightResolver struct {
	principal Principal
	err       error
}

func (r insightResolver) Resolve(_ *http.Request) (Principal, error) {
	return r.principal, r.err
}

type insightAuthorizer struct {
	err error
}

func (a insightAuthorizer) Authorize(_ context.Context, _ Principal, _ int64, _ string) error {
	return a.err
}

func insightRequest(handler http.Handler, page string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/"+page+"?corpId=2", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestAllPagesReturnStructuredLimitations(t *testing.T) {
	handler := NewInsightHandler(insightResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil)
	for _, page := range []string{"session-analysis", "smart-analysis", "emotion", "employee-score", "communication-keyword"} {
		rec := insightRequest(handler, page)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s code = %d, want 200", page, rec.Code)
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("%s invalid json: %v", page, err)
		}
		data := payload["data"].(map[string]any)
		if data["capability"] != "limited" {
			t.Fatalf("%s capability = %v, want limited", page, data["capability"])
		}
		limitations, ok := data["limitations"].([]any)
		if !ok || len(limitations) == 0 {
			t.Fatalf("%s limitations = %#v, want non-empty", page, data["limitations"])
		}
		if data["data"].([]any) == nil {
			t.Fatalf("%s data must be a real empty array", page)
		}
	}
}

func TestInsightUsesPrincipalCorpWhenClientOmitsCorpID(t *testing.T) {
	handler := NewInsightHandler(insightResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/emotion", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
}

func TestInsightUnauthorizedForbiddenAndUnknownPage(t *testing.T) {
	unauth := NewInsightHandler(insightResolver{err: errors.New("no principal")}, nil)
	if rec := insightRequest(unauth, "emotion"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized code = %d, want 401", rec.Code)
	}
	denied := NewInsightHandler(insightResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, insightAuthorizer{err: errors.New("denied")})
	if rec := insightRequest(denied, "emotion"); rec.Code != http.StatusForbidden {
		t.Fatalf("forbidden code = %d, want 403", rec.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/nope?corpId=2", nil)
	rec := httptest.NewRecorder()
	NewInsightHandler(insightResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown page code = %d, want 404", rec.Code)
	}
}
