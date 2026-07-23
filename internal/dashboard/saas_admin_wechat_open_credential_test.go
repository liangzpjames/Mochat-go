package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSaaSWeChatOpenCredentialProtectionStore struct {
	*fakeSaaSAdminStore
	statuses       []SaaSWeChatOpenCredentialProtectionStatus
	statusCalls    int
	rotationResult SaaSWeChatOpenCredentialRotationResult
	rotationCalls  int
	rotationTenant int
	rotationLimit  int
}

func (s *fakeSaaSWeChatOpenCredentialProtectionStore) SaaSWeChatOpenCredentialProtection(context.Context) (SaaSWeChatOpenCredentialProtectionStatus, error) {
	index := s.statusCalls
	s.statusCalls++
	if len(s.statuses) == 0 {
		return SaaSWeChatOpenCredentialProtectionStatus{}, nil
	}
	if index >= len(s.statuses) {
		index = len(s.statuses) - 1
	}
	return s.statuses[index], nil
}

func (s *fakeSaaSWeChatOpenCredentialProtectionStore) RotateSaaSWeChatOpenCredentials(_ context.Context, tenantID int, limit int) (SaaSWeChatOpenCredentialRotationResult, error) {
	s.rotationCalls++
	s.rotationTenant = tenantID
	s.rotationLimit = limit
	return s.rotationResult, nil
}

func TestSaaSAdminWeChatOpenCredentialProtectionReturnsCountsOnly(t *testing.T) {
	store := &fakeSaaSWeChatOpenCredentialProtectionStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}},
		statuses: []SaaSWeChatOpenCredentialProtectionStatus{{
			EncryptionConfigured: true, RequireEncryption: true, DedicatedConfigured: true, ActiveKeyID: "wechat-open-q3", KeyCount: 2,
			ConfiguredCredentialCount: 4, ComponentTicketCount: 1, OfficialAccountCount: 3, EncryptedCredentialCount: 3,
			LegacyPlaintextCount: 1, RotationRequiredCount: 1, Healthy: false,
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/wechatOpenCredentialProtection", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WeChatOpenCredentialProtection(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	protection := decodeSaaSAdminResponse(t, rec)["credentialProtection"].(map[string]any)
	if protection["activeKeyId"] != "wechat-open-q3" || protection["componentTicketCount"].(float64) != 1 || protection["officialAccountCount"].(float64) != 3 {
		t.Fatalf("protection=%+v", protection)
	}
	if strings.Contains(rec.Body.String(), "ticket-secret-sentinel") || strings.Contains(rec.Body.String(), "refresh-token-sentinel") {
		t.Fatalf("response leaked credential material: %s", rec.Body.String())
	}
}

func TestSaaSAdminWeChatOpenCredentialRotationAuditsCountsOnly(t *testing.T) {
	base := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}
	store := &fakeSaaSWeChatOpenCredentialProtectionStore{
		fakeSaaSAdminStore: base,
		statuses: []SaaSWeChatOpenCredentialProtectionStatus{
			{EncryptionConfigured: true, ActiveKeyID: "wechat-open-q3", KeyCount: 2, ConfiguredCredentialCount: 2, RotationRequiredCount: 2},
			{EncryptionConfigured: true, ActiveKeyID: "wechat-open-q3", KeyCount: 2, ConfiguredCredentialCount: 2, EncryptedCredentialCount: 2, ActiveKeyCredentialCount: 2, Healthy: true},
		},
		rotationResult: SaaSWeChatOpenCredentialRotationResult{TenantID: 10, Limit: 25, ScannedCount: 2, RotatedCount: 2, ComponentTicketRotatedCount: 0, OfficialAccountRotatedCount: 2, LegacyCount: 1, ReencryptedCount: 1, ActiveKeyID: "wechat-open-q3"},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/wechatOpenCredentialRotation", strings.NewReader(`{"tenantId":10,"limit":25,"remark":"季度轮换"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WeChatOpenCredentialRotation(rec, req)
	if rec.Code != http.StatusOK || store.rotationCalls != 1 || store.rotationTenant != 10 || store.rotationLimit != 25 {
		t.Fatalf("status=%d body=%s calls=%d tenant=%d limit=%d", rec.Code, rec.Body.String(), store.rotationCalls, store.rotationTenant, store.rotationLimit)
	}
	logItem := base.lastRecordedOperationLog
	if logItem.Action != SaaSAdminOperationActionWeChatOpenCredentialRotate || logItem.TargetType != SaaSAdminOperationTargetWeChatOpenCredential || logItem.TargetID != "10" {
		t.Fatalf("operation log=%+v", logItem)
	}
	if strings.Contains(logItem.BeforeJSON+logItem.AfterJSON+rec.Body.String(), "ticket-secret-sentinel") {
		t.Fatal("rotation leaked credential material")
	}
}
