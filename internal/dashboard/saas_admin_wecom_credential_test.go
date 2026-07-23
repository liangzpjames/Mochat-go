package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSaaSWeComCredentialProtectionStore struct {
	*fakeSaaSAdminStore
	statuses       []SaaSWeComCredentialProtectionStatus
	statusCalls    int
	rotationResult SaaSWeComCredentialRotationResult
	rotationCalls  int
	rotationTenant int
	rotationLimit  int
}

func (s *fakeSaaSWeComCredentialProtectionStore) SaaSWeComCredentialProtection(context.Context) (SaaSWeComCredentialProtectionStatus, error) {
	index := s.statusCalls
	s.statusCalls++
	if len(s.statuses) == 0 {
		return SaaSWeComCredentialProtectionStatus{}, nil
	}
	if index >= len(s.statuses) {
		index = len(s.statuses) - 1
	}
	return s.statuses[index], nil
}

func (s *fakeSaaSWeComCredentialProtectionStore) RotateSaaSWeComCredentials(_ context.Context, tenantID int, limit int) (SaaSWeComCredentialRotationResult, error) {
	s.rotationCalls++
	s.rotationTenant = tenantID
	s.rotationLimit = limit
	return s.rotationResult, nil
}

func TestSaaSAdminWeComCredentialProtectionReturnsCountsOnly(t *testing.T) {
	store := &fakeSaaSWeComCredentialProtectionStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}},
		statuses: []SaaSWeComCredentialProtectionStatus{{
			EncryptionConfigured: true, RequireEncryption: true, DedicatedConfigured: true,
			ActiveKeyID: "wecom-q3", KeyCount: 2, ConfiguredCredentialCount: 5,
			CorpCredentialCount: 2, AgentCredentialCount: 3, EncryptedCredentialCount: 4,
			LegacyPlaintextCount: 1, RotationRequiredCount: 2, UnavailableKeyCount: 1,
			UnavailableKeyIDs: []string{"retired"}, Healthy: false,
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/wecomCredentialProtection", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.WeComCredentialProtection(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	protection := decodeSaaSAdminResponse(t, rec)["credentialProtection"].(map[string]any)
	if protection["supported"] != true || protection["activeKeyId"] != "wecom-q3" ||
		protection["corpCredentialCount"].(float64) != 2 || protection["agentCredentialCount"].(float64) != 3 ||
		protection["legacyPlaintextCount"].(float64) != 1 || protection["healthy"] != false {
		t.Fatalf("credential protection=%+v", protection)
	}
	if strings.Contains(rec.Body.String(), "employee-secret-sentinel") || strings.Contains(rec.Body.String(), "agent-secret-sentinel") {
		t.Fatalf("credential response leaked secret material: %s", rec.Body.String())
	}
}

func TestSaaSAdminWeComCredentialRotationAuditsCountsOnly(t *testing.T) {
	base := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}
	store := &fakeSaaSWeComCredentialProtectionStore{
		fakeSaaSAdminStore: base,
		statuses: []SaaSWeComCredentialProtectionStatus{
			{EncryptionConfigured: true, ActiveKeyID: "wecom-q3", KeyCount: 2, ConfiguredCredentialCount: 3, LegacyPlaintextCount: 1, RotationRequiredCount: 2},
			{EncryptionConfigured: true, ActiveKeyID: "wecom-q3", KeyCount: 2, ConfiguredCredentialCount: 3, EncryptedCredentialCount: 3, ActiveKeyCredentialCount: 3, Healthy: true},
		},
		rotationResult: SaaSWeComCredentialRotationResult{
			TenantID: 10, Limit: 25, ScannedCount: 2, RotatedCount: 2,
			CorpRotatedCount: 1, AgentRotatedCount: 1, LegacyCount: 1,
			ReencryptedCount: 1, ActiveKeyID: "wecom-q3",
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/wecomCredentialRotation", strings.NewReader(`{"tenantId":10,"limit":25,"remark":"季度轮换"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.WeComCredentialRotation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.rotationCalls != 1 || store.rotationTenant != 10 || store.rotationLimit != 25 || store.statusCalls != 2 {
		t.Fatalf("rotation calls=%d tenant=%d limit=%d status_calls=%d", store.rotationCalls, store.rotationTenant, store.rotationLimit, store.statusCalls)
	}
	logItem := base.lastRecordedOperationLog
	if logItem.Action != SaaSAdminOperationActionWeComCredentialRotate || logItem.TargetType != SaaSAdminOperationTargetWeComCredential ||
		logItem.TargetID != "10" || logItem.TenantID != 10 || logItem.ActorUserID != 1 {
		t.Fatalf("operation log=%+v", logItem)
	}
	if strings.Contains(logItem.BeforeJSON+logItem.AfterJSON+rec.Body.String(), "employee-secret-sentinel") ||
		strings.Contains(logItem.BeforeJSON+logItem.AfterJSON+rec.Body.String(), "agent-secret-sentinel") {
		t.Fatalf("rotation leaked credential material: before=%s after=%s body=%s", logItem.BeforeJSON, logItem.AfterJSON, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	rotation := data["rotation"].(map[string]any)
	if rotation["rotatedCount"].(float64) != 2 || rotation["corpRotatedCount"].(float64) != 1 || data["operationId"].(float64) <= 0 {
		t.Fatalf("rotation response=%+v", data)
	}
}

func TestSaaSAdminWeComCredentialRotationValidatesAndRequiresPlatformAdmin(t *testing.T) {
	t.Run("invalid limit", func(t *testing.T) {
		store := &fakeSaaSWeComCredentialProtectionStore{fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}}
		handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
		req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/wecomCredentialRotation", strings.NewReader(`{"limit":1001}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Mochat-Go-User-ID", "1")
		rec := httptest.NewRecorder()
		handler.WeComCredentialRotation(rec, req)
		if rec.Code != http.StatusBadRequest || store.rotationCalls != 0 {
			t.Fatalf("status=%d body=%s calls=%d", rec.Code, rec.Body.String(), store.rotationCalls)
		}
	})

	t.Run("tenant admin", func(t *testing.T) {
		store := &fakeSaaSWeComCredentialProtectionStore{fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, TenantID: 10, IsSuperAdmin: 1}}}}
		handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
		req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/wecomCredentialRotation", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Mochat-Go-User-ID", "7")
		rec := httptest.NewRecorder()
		handler.WeComCredentialRotation(rec, req)
		if rec.Code != http.StatusForbidden || store.rotationCalls != 0 {
			t.Fatalf("status=%d body=%s calls=%d", rec.Code, rec.Body.String(), store.rotationCalls)
		}
	})
}
