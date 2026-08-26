package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseOptionsKeepsSecretsOutOfPrintableSummary(t *testing.T) {
	const dsn = "acceptance:secret@tcp(mysql:3306)/mochat"
	const token = "MOCHAT-LOCAL-ACCEPTANCE-BEARER-01234567890123456789"
	options, err := parseOptions([]string{"verify", "-dsn", dsn, "-bridge-token", token, "-dashboard-password-file", "local-password-file"}, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	summary, err := json.Marshal(options.publicSummary())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(summary), "secret") || strings.Contains(string(summary), token) || strings.Contains(string(summary), dsn) {
		t.Fatalf("public summary leaked secret: %s", summary)
	}
	if options.Action != "verify" || options.DSN != dsn || options.BridgeToken != token {
		t.Fatalf("options=%+v", options)
	}
}

func TestRedactErrorHidesDSNAndBearerDetails(t *testing.T) {
	for _, message := range []string{
		"dial acceptance:secret@tcp(mysql:3306): connection refused",
		"authorization failed for Bearer super-secret-token",
		"invalid sdkFileId=MOCHAT-LOCAL-SECRET",
	} {
		redacted := redactError(errors.New(message))
		if redacted == message || strings.Contains(redacted, "secret") || strings.Contains(redacted, "super-secret-token") {
			t.Fatalf("error was not redacted: %q", redacted)
		}
	}
}

func TestFixtureBridgeHealthAndBearerBoundary(t *testing.T) {
	const token = "MOCHAT-LOCAL-ACCEPTANCE-BEARER-01234567890123456789"
	handler, closeFixture, err := newFixtureBridge(t.TempDir(), token)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFixture()

	server := httptest.NewServer(handler)
	defer server.Close()
	response, err := server.Client().Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("health status=%d", response.StatusCode)
	}
	response.Body.Close()

	body := bytes.NewBufferString(`{"corp_id":820827,"wx_corpid":"ww-MOCHAT-LOCAL-ACCEPTANCE-20260827","seq":0,"limit":9}`)
	request, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/work-message/archive/messages", body)
	response, err = server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated bridge status=%d", response.StatusCode)
	}
	response.Body.Close()

	body = bytes.NewBufferString(`{"corp_id":820827,"wx_corpid":"ww-MOCHAT-LOCAL-ACCEPTANCE-20260827","seq":0,"limit":9}`)
	request, _ = http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/work-message/archive/messages", body)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err = server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("authenticated bridge status=%d", response.StatusCode)
	}
	response.Body.Close()
}

func TestCleanupStatementsAreDatasetScoped(t *testing.T) {
	if len(acceptanceIntegrationID) != 36 {
		t.Fatalf("integration id length=%d", len(acceptanceIntegrationID))
	}
	statements := cleanupStatements()
	if len(statements) < 10 {
		t.Fatalf("cleanup statement count=%d", len(statements))
	}
	for _, statement := range statements {
		normalized := strings.ToLower(statement)
		if strings.Contains(normalized, "truncate") || strings.Contains(normalized, "drop table") {
			t.Fatalf("destructive broad cleanup: %s", statement)
		}
		if !strings.Contains(normalized, "?") {
			t.Fatalf("cleanup is not parameter scoped: %s", statement)
		}
	}
}

func TestDatasetObjectPathAcceptsStoredAbsolutePathWithinArchiveMediaRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "upload", "static")
	wanted := filepath.Join(root, "archive-media", "object-id")
	resolved, ok := datasetObjectPath(root, wanted)
	if !ok || resolved != wanted {
		t.Fatalf("resolved=%q ok=%t", resolved, ok)
	}
	if _, ok := datasetObjectPath(root, filepath.Join(filepath.Dir(root), "outside")); ok {
		t.Fatal("path outside storage root was accepted")
	}
}

func TestVerifyDashboardMediaHTTPUsesRealLoginAndRejectsUnauthenticatedRead(t *testing.T) {
	payload := []byte("fixture-media")
	digest := sha256.Sum256(payload)
	var unauthenticated, authenticated bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/dashboard/user/auth":
			if request.Method != http.MethodPost {
				t.Fatalf("login method=%s", request.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "data": map[string]any{"token": "opaque-dashboard-token"}})
		case "/dashboard/archive/media/8ff7bf2d-5604-43bc-a600-3ec91d575085/content":
			if request.Header.Get("Authorization") == "" {
				unauthenticated = true
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			authenticated = true
			_, _ = w.Write(payload)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	if err := verifyDashboardMediaHTTP(context.Background(), server.Client(), server.URL, "19008208270", "local-password", "8ff7bf2d-5604-43bc-a600-3ec91d575085", fmt.Sprintf("%x", digest)); err != nil {
		t.Fatal(err)
	}
	if !unauthenticated || !authenticated {
		t.Fatalf("unauthenticated=%t authenticated=%t", unauthenticated, authenticated)
	}
}

func TestReadAcceptancePasswordRequiresNonEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dashboard-password")
	if err := os.WriteFile(path, []byte("local-password\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	password, err := readAcceptancePassword(path)
	if err != nil || password != "local-password" {
		t.Fatalf("password=%q err=%v", password, err)
	}
	if err := os.WriteFile(path, []byte(" \r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readAcceptancePassword(path); err == nil {
		t.Fatal("empty password file was accepted")
	}
}

func TestTerminalFixtureMediaErrorsDoNotAbortRemainingObjects(t *testing.T) {
	for _, value := range []string{"archive.media_missing", "archive.media_corrupt: archive media operation failed", "archive.media_integrity_mismatch: archive media operation failed"} {
		if !terminalFixtureMediaError(errors.New(value)) {
			t.Fatalf("terminal error %q was not recognized", value)
		}
	}
	if terminalFixtureMediaError(errors.New("archive.media_fetch_failed")) {
		t.Fatal("retryable media fetch error was treated as terminal")
	}
}
