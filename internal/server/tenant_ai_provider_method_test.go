package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jiyi/mochat-go/internal/config"
)

func TestTenantAIProviderExactPathUnsupportedMethodReturns405(t *testing.T) {
	srv, err := New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second}, WithSaaSAdminTenantAIProviderHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusMethodNotAllowed) })))
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantAIProvider", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
