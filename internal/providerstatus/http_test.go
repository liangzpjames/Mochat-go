package providerstatus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/modules/providers"
)

func TestHTTPHandlerRequiresDashboardPrincipal(t *testing.T) {
	handler := NewHTTPHandler(NewService(&statusTestSource{}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/dashboard/providers/status", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	var envelope struct {
		ErrorCode string `json:"errorCode"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.ErrorCode != CodeSessionInvalid {
		t.Fatalf("errorCode = %q, want %q", envelope.ErrorCode, CodeSessionInvalid)
	}
}

func TestHTTPHandlerFailsClosedWhenServiceIsNotWired(t *testing.T) {
	handler := NewHTTPHandler(nil)
	request := httptest.NewRequest(http.MethodGet, "/dashboard/providers/status", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
}

func TestHTTPHandlerUsesContextScopeAndIgnoresRequestScopeInjection(t *testing.T) {
	source := &statusTestSource{statuses: []providers.Status{{Kind: "wecom_standard", State: providers.StateReady, Code: "wecom.runtime_verified", Source: providers.SourceExternal}}}
	handler := NewHTTPHandler(NewService(source))
	principal := statusTestPrincipal(false)
	request := httptest.NewRequest(http.MethodGet, "/dashboard/providers/status?tenantId=999&corpId=998&actorId=997", nil)
	request = request.WithContext(dashboardprincipal.WithPrincipal(context.Background(), principal))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || source.seen.TenantID != 7 || source.seen.CorpID != 11 || source.seen.UserID != 1 {
		t.Fatalf("status=%d sourcePrincipal=%#v", recorder.Code, source.seen)
	}
	if string(recorder.Body.Bytes()) == "" {
		t.Fatal("empty provider status response")
	}
}

func TestHTTPHandlerReturnsStableSourceFailure(t *testing.T) {
	handler := NewHTTPHandler(NewService(&statusTestSource{err: ErrSourceUnavailable}))
	request := httptest.NewRequest(http.MethodGet, "/dashboard/providers/status", nil)
	request = request.WithContext(dashboardprincipal.WithPrincipal(context.Background(), statusTestPrincipal(true)))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
	var envelope struct {
		ErrorCode string `json:"errorCode"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.ErrorCode != CodeSourceUnavailable {
		t.Fatalf("errorCode = %q, want %q", envelope.ErrorCode, CodeSourceUnavailable)
	}
}
