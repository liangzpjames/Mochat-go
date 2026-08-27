package archivebridge

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/testfixtures/archivesource"
)

const testFixtureAdminBearer = "local-fixture-admin-012345678901234567890123456789"

func TestFixtureAdminSeedsSendsPersistsAndCleansBothModes(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "fixture-state.json")
	store := NewStore()
	fixtures, err := NewFixtureManager(statePath, store)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(Config{BearerToken: testBridgeBearer, FixtureEnabled: true, FixtureAdminToken: testFixtureAdminBearer, FixtureManager: fixtures}, store)
	if err != nil {
		t.Fatal(err)
	}
	bindings := []Binding{
		{TenantID: 11, CorpID: 27, WXCorpID: "ww-self", IntegrationMode: ModeSelfBuilt},
		{TenantID: 21, CorpID: 37, WXCorpID: "ww-delegated", IntegrationMode: ModeThirdPartyDelegated},
	}
	for _, binding := range bindings {
		response := postFixture(t, handler, "/v1/fixture/seed", map[string]any{"binding": binding, "dataset": "MOCHAT-LOCAL-SIM-contract"})
		if response.Code != http.StatusOK {
			t.Fatalf("seed mode=%s status=%d body=%s", binding.IntegrationMode, response.Code, response.Body.String())
		}
		response = postFixture(t, handler, "/v1/fixture/send", map[string]any{"binding": binding, "dataset": "MOCHAT-LOCAL-SIM-contract", "type": "text", "text": "双模式模拟消息"})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "messageId") {
			t.Fatalf("send mode=%s status=%d body=%s", binding.IntegrationMode, response.Code, response.Body.String())
		}
	}

	restoredStore := NewStore()
	restored, err := NewFixtureManager(statePath, restoredStore)
	if err != nil {
		t.Fatal(err)
	}
	if status := restored.Status(); status.DatasetCount != 2 || status.MessageCount != 17 || status.DelegatedAuthorizedCount != 1 {
		t.Fatalf("restored status=%+v", status)
	}
	restoredHandler, err := NewHandler(Config{BearerToken: testBridgeBearer, FixtureEnabled: true, FixtureAdminToken: testFixtureAdminBearer, FixtureManager: restored}, restoredStore)
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range bindings {
		raw, _ := json.Marshal(map[string]any{"tenant_id": binding.TenantID, "corp_id": binding.CorpID, "wx_corpid": binding.WXCorpID, "integration_mode": binding.IntegrationMode, "seq": 0, "limit": 100})
		request := httptest.NewRequest(http.MethodPost, "/v1/archive/messages", bytes.NewReader(raw))
		request.Header.Set("Authorization", "Bearer "+testBridgeBearer)
		response := httptest.NewRecorder()
		restoredHandler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "MOCHAT-LOCAL-SIM-contract") {
			t.Fatalf("restored mode=%s response=%d %s", binding.IntegrationMode, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), "MOCHAT-LOCAL-SIM-contract-STAFF-01") || !strings.Contains(response.Body.String(), "MOCHAT-LOCAL-SIM-contract-EXTERNAL-01") {
			t.Fatalf("restored mode=%s did not preserve dataset-scoped participants: %s", binding.IntegrationMode, response.Body.String())
		}
		if binding.IntegrationMode == ModeSelfBuilt && strings.Contains(response.Body.String(), archivesource.DatasetMarker) {
			t.Fatalf("self-built fixture escaped its dataset scope: %s", response.Body.String())
		}
	}
	for _, binding := range bindings {
		response := postFixture(t, restoredHandler, "/v1/fixture/cleanup", map[string]any{"binding": binding, "dataset": "MOCHAT-LOCAL-SIM-contract", "confirmDataset": "MOCHAT-LOCAL-SIM-contract"})
		if response.Code != http.StatusOK {
			t.Fatalf("cleanup=%d %s", response.Code, response.Body.String())
		}
	}
	if restored.Status().DatasetCount != 0 {
		t.Fatalf("cleanup status=%+v", restored.Status())
	}
}

func postFixture(t *testing.T, handler http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	request.Header.Set("Authorization", "Bearer "+testFixtureAdminBearer)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
