package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

type fakeArchiveComponentStore struct {
	object ArchiveComponentObject
	found  bool
}

func (store *fakeArchiveComponentStore) ArchiveComponentByID(context.Context, ArchiveComponentFilter) (ArchiveComponentObject, bool, error) {
	return store.object, store.found, nil
}

type fakeArchiveComponentBridge struct {
	content ArchiveComponentContent
	calls   int
}

func (bridge *fakeArchiveComponentBridge) FetchArchiveComponent(context.Context, ArchiveComponentObject) (ArchiveComponentContent, error) {
	bridge.calls++
	return bridge.content, nil
}

func TestArchiveComponentCreatesBoundOneTimeSessionAndServesContent(t *testing.T) {
	const id = "8ff7bf2d-5604-43bc-a600-3ec91d575085"
	store := &fakeArchiveComponentStore{found: true, object: ArchiveComponentObject{
		ID: id, TenantID: 11, CorpID: 27, WXCorpID: "ww-local", MessageID: "dz-1", PublicKeyVersion: 1, EncryptedSecretKey: "wrapped-key",
	}}
	bridge := &fakeArchiveComponentBridge{content: ArchiveComponentContent{Type: "voice", MIMEType: "audio/wav", FileName: "message.wav", Body: []byte("fixture-audio")}}
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	handler := NewArchiveComponentHandler(store, bridge).WithClock(func() time.Time { return now })
	request := archiveComponentRequest(http.MethodPost, "/dashboard/archive/components/"+id+"/session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		SessionURL string `json:"sessionUrl"`
		ExpiresIn  int    `json:"expiresIn"`
	}
	if json.Unmarshal(response.Body.Bytes(), &payload) != nil || payload.ExpiresIn != 60 || !strings.HasPrefix(payload.SessionURL, "/dashboard/archive/components/session/") || strings.Contains(payload.SessionURL, "wrapped-key") {
		t.Fatalf("session payload=%s", response.Body.String())
	}
	view := archiveComponentRequest(http.MethodGet, payload.SessionURL)
	viewResponse := httptest.NewRecorder()
	handler.ServeHTTP(viewResponse, view)
	if viewResponse.Code != http.StatusOK || viewResponse.Body.String() != "fixture-audio" || viewResponse.Header().Get("Content-Type") != "audio/wav" || bridge.calls != 1 {
		t.Fatalf("view status/body/type/calls=%d/%q/%q/%d", viewResponse.Code, viewResponse.Body.String(), viewResponse.Header().Get("Content-Type"), bridge.calls)
	}
	replay := httptest.NewRecorder()
	handler.ServeHTTP(replay, archiveComponentRequest(http.MethodGet, payload.SessionURL))
	if replay.Code != http.StatusNotFound || bridge.calls != 1 {
		t.Fatalf("replay status/calls=%d/%d", replay.Code, bridge.calls)
	}
}

func TestArchiveComponentSessionRejectsChangedAuthVersion(t *testing.T) {
	const id = "8ff7bf2d-5604-43bc-a600-3ec91d575085"
	store := &fakeArchiveComponentStore{found: true, object: ArchiveComponentObject{ID: id, TenantID: 11, CorpID: 27, WXCorpID: "ww-local", MessageID: "dz-1", PublicKeyVersion: 1, EncryptedSecretKey: "wrapped-key"}}
	bridge := &fakeArchiveComponentBridge{content: ArchiveComponentContent{Type: "text", MIMEType: "text/plain", Body: []byte("hello")}}
	handler := NewArchiveComponentHandler(store, bridge)
	create := httptest.NewRecorder()
	handler.ServeHTTP(create, archiveComponentRequest(http.MethodPost, "/dashboard/archive/components/"+id+"/session"))
	var payload struct {
		SessionURL string `json:"sessionUrl"`
	}
	_ = json.Unmarshal(create.Body.Bytes(), &payload)
	request := archiveComponentRequest(http.MethodGet, payload.SessionURL)
	principal, _ := DashboardPrincipalFromContext(request.Context())
	principal.AuthVersion++
	request = request.WithContext(dashboardprincipal.WithPrincipal(request.Context(), principal))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || bridge.calls != 0 {
		t.Fatalf("status/calls=%d/%d", response.Code, bridge.calls)
	}
}

func TestArchiveComponentRejectsMissingScopeCrossUserAndExpiry(t *testing.T) {
	const id = "8ff7bf2d-5604-43bc-a600-3ec91d575085"
	store := &fakeArchiveComponentStore{found: true, object: ArchiveComponentObject{ID: id, TenantID: 11, CorpID: 27, WXCorpID: "ww-local", MessageID: "dz-1", PublicKeyVersion: 1, EncryptedSecretKey: "wrapped-key"}}
	handler := NewArchiveComponentHandler(store, &fakeArchiveComponentBridge{content: ArchiveComponentContent{Type: "text", MIMEType: "text/plain", Body: []byte("hello")}})
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodPost, "/dashboard/archive/components/"+id+"/session", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing principal status=%d", missing.Code)
	}
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	handler.WithClock(func() time.Time { return now })
	create := httptest.NewRecorder()
	handler.ServeHTTP(create, archiveComponentRequest(http.MethodPost, "/dashboard/archive/components/"+id+"/session"))
	var payload struct {
		SessionURL string `json:"sessionUrl"`
	}
	_ = json.Unmarshal(create.Body.Bytes(), &payload)
	cross := archiveComponentRequest(http.MethodGet, payload.SessionURL)
	principal, _ := DashboardPrincipalFromContext(cross.Context())
	principal.UserID = 6
	cross = cross.WithContext(dashboardprincipal.WithPrincipal(cross.Context(), principal))
	crossResponse := httptest.NewRecorder()
	handler.ServeHTTP(crossResponse, cross)
	if crossResponse.Code != http.StatusNotFound {
		t.Fatalf("cross-user status=%d", crossResponse.Code)
	}
	now = now.Add(61 * time.Second)
	expired := httptest.NewRecorder()
	handler.ServeHTTP(expired, archiveComponentRequest(http.MethodGet, payload.SessionURL))
	if expired.Code != http.StatusNotFound {
		t.Fatalf("expired status=%d", expired.Code)
	}
}

func archiveComponentRequest(method, path string) *http.Request {
	request := httptest.NewRequest(method, path, nil)
	ctx := dashboardprincipal.WithPrincipal(request.Context(), dashboardprincipal.DashboardPrincipal{UserID: 5, TenantID: 11, CorpID: 27, CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 1})
	ctx = WithDashboardAccessContext(ctx, DashboardAccessContext{UserID: 5, TenantID: 11, CorpID: 27, ScopeRequired: true, Scope: DataScopeTenant, PermissionCodes: []string{"dashboard.chat.v2_all"}, PermissionScopes: map[string]DataScope{"dashboard.chat.v2_all": DataScopeTenant}})
	return request.WithContext(ctx)
}
