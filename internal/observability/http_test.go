package observability

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPMiddlewareLogsOnlyCriticalOutcomes(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(Config{Level: "debug", Format: "json", Output: &output, Service: "test"})
	if err != nil {
		t.Fatal(err)
	}
	handler := HTTPMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestID(r.Context()) == "" {
			t.Error("request id missing from context")
		}
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
		case "/readyz":
			w.WriteHeader(http.StatusServiceUnavailable)
		case "/dashboard/company/employee-sync":
			w.WriteHeader(http.StatusAccepted)
		case "/dashboard/denied":
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))

	health := callHTTPMiddleware(t, handler, http.MethodGet, "/healthz", "request-health")
	if health.Header().Get("X-Request-ID") != "request-health" {
		t.Fatalf("health request id = %q", health.Header().Get("X-Request-ID"))
	}
	if output.Len() != 0 {
		t.Fatalf("successful health poll logged: %s", output.String())
	}
	callHTTPMiddleware(t, handler, http.MethodPost, "/dashboard/company/employee-sync?token=never-query", "request-sync")
	callHTTPMiddleware(t, handler, http.MethodPut, "/dashboard/denied", "request-denied")
	callHTTPMiddleware(t, handler, http.MethodPost, "/dashboard/fail", "request-failed")
	callHTTPMiddleware(t, handler, http.MethodGet, "/readyz", "request-ready")

	logs := output.String()
	for _, required := range []string{
		`"event":"critical_request_completed"`,
		`"event":"request_rejected"`,
		`"event":"critical_request_failed"`,
		`"event":"readiness_failed"`,
		`"level":"INFO"`,
		`"level":"WARN"`,
		`"level":"ERROR"`,
		`"request_id":"request-sync"`,
		`"component":"wecom_sync"`,
		`"duration_ms":`,
		`"error_code":"HTTP_403"`,
		`"error_code":"HTTP_500"`,
	} {
		if !strings.Contains(logs, required) {
			t.Fatalf("missing %q: %s", required, logs)
		}
	}
	for _, forbidden := range []string{"never-query", "Authorization", "Bearer"} {
		if strings.Contains(logs, forbidden) {
			t.Fatalf("HTTP log leaked %q: %s", forbidden, logs)
		}
	}
}

func TestHTTPMiddlewareRejectsUnsafeRequestIDWithoutLoggingIt(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(Config{Level: "info", Format: "json", Output: &output, Service: "test"})
	if err != nil {
		t.Fatal(err)
	}
	handler := HTTPMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "/dashboard/company/archive-sync", nil)
	request.Header.Set("X-Request-ID", "unsafe request id token=secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	requestID := response.Header().Get("X-Request-ID")
	if requestID == "" || requestID == request.Header.Get("X-Request-ID") {
		t.Fatalf("generated request id = %q", requestID)
	}
	if strings.Contains(output.String(), "unsafe request id") || strings.Contains(output.String(), "secret") {
		t.Fatalf("unsafe request id leaked: %s", output.String())
	}
}

func TestHTTPMiddlewareDoesNotDuplicateSuiteCallbackDomainLogs(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(Config{Level: "debug", Format: "json", Output: &output, Service: "test"})
	if err != nil {
		t.Fatal(err)
	}
	handler := HTTPMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	callHTTPMiddleware(t, handler, http.MethodPost, "/wecom/suite/callback", "request-callback")
	if output.Len() != 0 {
		t.Fatalf("suite callback outcome logged twice at HTTP boundary: %s", output.String())
	}
}

func TestHTTPMiddlewareRecoversPanicAndLogsControlledFailure(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(Config{Level: "debug", Format: "json", Output: &output, Service: "test"})
	if err != nil {
		t.Fatal(err)
	}
	handler := HTTPMiddleware(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("token=panic-secret")
	}))
	response := callHTTPMiddleware(t, handler, http.MethodPost, "/dashboard/panic", "request-panic")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	logs := output.String()
	for _, required := range []string{`"event":"critical_request_panicked"`, `"request_id":"request-panic"`, `"error_code":"HTTP_HANDLER_PANIC"`, `"level":"ERROR"`} {
		if !strings.Contains(logs, required) {
			t.Fatalf("missing %q: %s", required, logs)
		}
	}
	if strings.Contains(logs, "panic-secret") || strings.Count(logs, `"request_id":"request-panic"`) != 1 {
		t.Fatalf("panic log leaked or duplicated: %s", logs)
	}
}

func callHTTPMiddleware(t *testing.T, handler http.Handler, method string, target string, requestID string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", "Bearer never-log")
	request.Header.Set("Cookie", "session=never-log")
	request.Header.Set("X-Request-ID", requestID)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
