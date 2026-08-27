package dashboardadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/saasauth"
	"jiyi/mochat-go/internal/wecomcredentials"
)

type fakeWeComIntegrationStore struct {
	view             WeComIntegrationView
	candidate        WeComVerificationCandidate
	verification     WeComVerificationResult
	verificationCode string
	currentBefore    WeComIntegration
	currentAfter     WeComIntegration
	seenTenant       int
	seenInput        WeComIntegrationCandidateInput
}

func (s *fakeWeComIntegrationStore) WeComIntegration(_ context.Context, _ Actor, tenantID int) (WeComIntegrationView, error) {
	s.seenTenant = tenantID
	return s.view, nil
}
func (s *fakeWeComIntegrationStore) SaveWeComIntegrationCandidate(_ context.Context, _ Actor, tenantID int, input WeComIntegrationCandidateInput) (WeComIntegration, error) {
	s.seenTenant, s.seenInput = tenantID, input
	if err := ValidateWeComIntegrationCandidate(input, s.view.Candidate != nil && s.view.Candidate.CredentialConfigured); err != nil {
		return WeComIntegration{}, err
	}
	return WeComIntegration{ID: "candidate", Mode: input.Mode, Slot: "candidate", Status: "pending_verification", Version: input.Version + 1, CredentialConfigured: true}, nil
}
func (s *fakeWeComIntegrationStore) SaveWeComIntegrationCurrent(_ context.Context, _ Actor, tenantID int, input WeComIntegrationCandidateInput) (WeComIntegration, error) {
	s.seenTenant, s.seenInput = tenantID, input
	return WeComIntegration{ID: "current", Mode: input.Mode, Slot: "current", Status: "pending_verification", Version: input.Version + 1, CredentialConfigured: true}, nil
}

func TestWeComIntegrationHTTPUsesPathTenantRejectsBodyScopeAndDoesNotEchoSecrets(t *testing.T) {
	store := &fakeWeComIntegrationStore{view: WeComIntegrationView{TenantID: 41, CorpID: 63, Current: &WeComIntegration{ID: "current", Slot: "current", Status: "active", CredentialConfigured: true, CredentialHint: "configured", CredentialCiphertext: "cipher-secret", CredentialKeyID: "key-secret"}}}
	handler := NewHTTPHandler(NewService(&dashboardAdminHTTPStore{})).WithWeComIntegration(NewWeComIntegrationService(store, nil))
	auth := func(request *http.Request) *http.Request {
		return request.WithContext(saasauth.WithPrincipal(request.Context(), saasauth.Principal{UserID: 700, AuthVersion: 9}))
	}
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, auth(httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenants/41/wecom-integration", nil)))
	if get.Code != http.StatusOK || store.seenTenant != 41 {
		t.Fatalf("status=%d tenant=%d", get.Code, store.seenTenant)
	}
	lower := strings.ToLower(get.Body.String())
	for _, forbidden := range []string{"cipher-secret", "key-secret", "ciphertext", "permanentcode"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("GET leaked %q: %s", forbidden, get.Body.String())
		}
	}
	retired := httptest.NewRecorder()
	handler.ServeHTTP(retired, auth(httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/tenants/41/wecom-integration/candidate", bytes.NewBufferString(`{"mode":"third_party_delegated","providerAppId":"provider","permanentCode":"secret-value","tenantId":99,"version":1}`))))
	if retired.Code != http.StatusNotFound {
		t.Fatalf("retired candidate route status=%d body=%s", retired.Code, retired.Body.String())
	}
}
func (s *fakeWeComIntegrationStore) WeComIntegrationVerificationCandidate(context.Context, Actor, int, uint64) (WeComVerificationCandidate, error) {
	return s.candidate, nil
}
func (s *fakeWeComIntegrationStore) CompleteWeComIntegrationVerification(_ context.Context, _ Actor, _ int, _ uint64, result WeComVerificationResult, code string) (WeComIntegration, error) {
	s.verification, s.verificationCode = result, code
	return WeComIntegration{ID: "candidate", Slot: "candidate", Status: map[bool]string{true: "verified", false: "failed"}[code == ""]}, nil
}
func (s *fakeWeComIntegrationStore) SwitchWeComIntegration(context.Context, Actor, int, uint64) (WeComIntegrationView, error) {
	return WeComIntegrationView{}, nil
}
func (s *fakeWeComIntegrationStore) RollbackWeComIntegration(context.Context, Actor, int, uint64) (WeComIntegrationView, error) {
	return WeComIntegrationView{}, nil
}
func (s *fakeWeComIntegrationStore) WeComIntegrationAudits(context.Context, Actor, int) ([]WeComIntegrationAudit, error) {
	return []WeComIntegrationAudit{}, nil
}

func TestWeComIntegrationCandidateModeContractsAndSecretRetention(t *testing.T) {
	tests := []struct {
		name     string
		input    WeComIntegrationCandidateInput
		existing bool
		ok       bool
	}{
		{"self built create", WeComIntegrationCandidateInput{Mode: "self_built", EmployeeSecret: "employee", Version: 1}, false, true},
		{"self built retain", WeComIntegrationCandidateInput{Mode: "self_built", Version: 2}, true, true},
		{"self built first requires secret", WeComIntegrationCandidateInput{Mode: "self_built", Version: 1}, false, false},
		{"self built rejects delegated", WeComIntegrationCandidateInput{Mode: "self_built", EmployeeSecret: "employee", ProviderAppID: "provider"}, false, false},
		{"delegated create", WeComIntegrationCandidateInput{Mode: "third_party_delegated", ProviderAppID: "provider", PermanentCode: "permanent", Version: 1}, false, true},
		{"delegated retain", WeComIntegrationCandidateInput{Mode: "third_party_delegated", ProviderAppID: "provider", Version: 2}, true, true},
		{"delegated first requires permanent", WeComIntegrationCandidateInput{Mode: "third_party_delegated", ProviderAppID: "provider"}, false, false},
		{"delegated rejects self built", WeComIntegrationCandidateInput{Mode: "third_party_delegated", ProviderAppID: "provider", PermanentCode: "permanent", ChatSecret: "chat"}, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateWeComIntegrationCandidate(tc.input, tc.existing)
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v ok=%v", err, tc.ok)
			}
		})
	}
}

func TestWeComIntegrationLegacyCandidateLifecycleFailsClosed(t *testing.T) {
	store := &fakeWeComIntegrationStore{candidate: WeComVerificationCandidate{Integration: WeComIntegration{ID: "candidate", Version: 3}, TenantID: 41, CorpID: 63, AuthoritativeWXCorpID: "ww-authoritative", Credentials: wecomcredentials.AuthorizationCredential{Mode: "self_built", EmployeeSecret: "secret"}}}
	service := NewWeComIntegrationService(store, WeComIntegrationVerifierFunc(func(_ context.Context, request WeComVerificationRequest) (WeComVerificationResult, error) {
		t.Fatal("retired verifier must not be called")
		return WeComVerificationResult{}, nil
	}))
	for name, call := range map[string]func() error{
		"save": func() error {
			_, err := service.SaveCandidate(context.Background(), Actor{UserID: 7, Active: true}, 41, WeComIntegrationCandidateInput{Mode: WeComIntegrationModeThirdPartyDelegated})
			return err
		},
		"verify": func() error {
			_, err := service.VerifyCandidate(context.Background(), Actor{UserID: 7, Active: true}, 41, 3)
			return err
		},
		"switch": func() error {
			_, err := service.Switch(context.Background(), Actor{UserID: 7, Active: true}, 41, 3)
			return err
		},
		"rollback": func() error {
			_, err := service.Rollback(context.Background(), Actor{UserID: 7, Active: true}, 41, 3)
			return err
		},
	} {
		if err := call(); !errors.Is(err, ErrWeComModeImmutable) {
			t.Fatalf("%s err=%v, want immutable", name, err)
		}
	}
	if store.seenTenant != 0 || store.verificationCode != "" {
		t.Fatal("retired lifecycle reached persistence")
	}
}

func TestWeComIntegrationPublicJSONContainsNoCredentialMaterial(t *testing.T) {
	value := WeComIntegrationView{TenantID: 41, CorpID: 63, Current: &WeComIntegration{ID: "current", Mode: "self_built", Slot: "current", Status: "active", CredentialConfigured: true, CredentialHint: "configured", CredentialCiphertext: "cipher-secret", CredentialKeyID: "key-secret"}}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"cipher-secret", "key-secret", "employeesecret", "permanentcode", "ciphertext"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, raw)
		}
	}
}
