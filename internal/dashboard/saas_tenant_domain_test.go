package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeSaaSAdminTenantDomainStore struct {
	*fakeSaaSAdminStore
	domains            []SaaSTenantDomain
	domain             SaaSTenantDomain
	createResult       SaaSAdminTenantDomainResult
	verificationResult SaaSAdminTenantDomainResult
	commandResult      SaaSAdminTenantDomainResult
	lastOptions        SaaSAdminTenantDomainOptions
	lastCreate         SaaSAdminTenantDomainCreate
	lastVerification   SaaSAdminTenantDomainVerification
	lastCommand        SaaSAdminTenantDomainCommand
	listCalls          int
	createCalls        int
	verificationCalls  int
	commandCalls       int
}

func (store *fakeSaaSAdminTenantDomainStore) SaaSTenantDomainByHostname(_ context.Context, hostname string) (SaaSTenantDomain, bool, error) {
	if store.domain.Hostname == hostname {
		return store.domain, true, nil
	}
	return SaaSTenantDomain{}, false, nil
}

func (store *fakeSaaSAdminTenantDomainStore) SaaSAdminTenantDomains(_ context.Context, options SaaSAdminTenantDomainOptions) ([]SaaSTenantDomain, error) {
	store.listCalls++
	store.lastOptions = options
	return store.domains, nil
}

func (store *fakeSaaSAdminTenantDomainStore) SaaSAdminTenantDomain(_ context.Context, _ int64) (SaaSTenantDomain, error) {
	return store.domain, nil
}

func (store *fakeSaaSAdminTenantDomainStore) CreateSaaSAdminTenantDomain(_ context.Context, input SaaSAdminTenantDomainCreate) (SaaSAdminTenantDomainResult, error) {
	store.createCalls++
	store.lastCreate = input
	return store.createResult, nil
}

func (store *fakeSaaSAdminTenantDomainStore) CompleteSaaSAdminTenantDomainVerification(_ context.Context, input SaaSAdminTenantDomainVerification) (SaaSAdminTenantDomainResult, error) {
	store.verificationCalls++
	store.lastVerification = input
	return store.verificationResult, nil
}

func (store *fakeSaaSAdminTenantDomainStore) ApplySaaSAdminTenantDomainCommand(_ context.Context, input SaaSAdminTenantDomainCommand) (SaaSAdminTenantDomainResult, error) {
	store.commandCalls++
	store.lastCommand = input
	return store.commandResult, nil
}

type fakeTenantDomainTXTLookup struct {
	values []string
	err    error
	name   string
}

func (lookup *fakeTenantDomainTXTLookup) LookupTXT(_ context.Context, name string) ([]string, error) {
	lookup.name = name
	return lookup.values, lookup.err
}

func TestNormalizeSaaSTenantDomainHostname(t *testing.T) {
	hostname, err := NormalizeSaaSTenantDomainHostname(" Login.Customer-1.Example.COM. ")
	if err != nil || hostname != "login.customer-1.example.com" {
		t.Fatalf("hostname=%q err=%v", hostname, err)
	}
	for _, value := range []string{"localhost", "127.0.0.1", "example", "https://example.com", "example.com:443", "_bad.example.com", "-bad.example.com", "bad-.example.com"} {
		if _, err := NormalizeSaaSTenantDomainHostname(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
	if hostname, ok := SaaSTenantDomainRequestHostname("Login.Customer.Example.com:8443"); !ok || hostname != "login.customer.example.com" {
		t.Fatalf("request hostname=%q ok=%v", hostname, ok)
	}
	if _, ok := SaaSTenantDomainRequestHostname("127.0.0.1:18090"); ok {
		t.Fatal("IP host must not be treated as a custom domain")
	}
}

func TestSaaSTenantDomainDNSVerifierRequiresExactTXTValue(t *testing.T) {
	domain := SaaSTenantDomain{Hostname: "login.customer.example.com", VerificationToken: "token-123"}
	lookup := &fakeTenantDomainTXTLookup{values: []string{"unrelated", domain.VerificationRecordValue()}}
	verifier := NewSaaSTenantDomainDNSVerifierWithLookup(lookup, time.Second)
	if err := verifier.Verify(context.Background(), domain); err != nil {
		t.Fatal(err)
	}
	if lookup.name != "_mochat.login.customer.example.com" {
		t.Fatalf("lookup name=%q", lookup.name)
	}
	lookup.values = []string{"mochat-domain-verification=other"}
	if err := verifier.Verify(context.Background(), domain); err == nil {
		t.Fatal("expected mismatched TXT value to fail")
	}
	lookup.err = errors.New("resolver unavailable")
	if err := verifier.Verify(context.Background(), domain); err == nil || err.Error() != "DNS TXT 记录暂不可用" {
		t.Fatalf("resolver error=%v", err)
	}
}

func TestSaaSAdminTenantDomainHandlersCreateVerifyAndPreserveActor(t *testing.T) {
	domain := SaaSTenantDomain{
		ID: 18, TenantID: 9, TenantName: "客户租户", Hostname: "login.customer.example.com",
		Status: SaaSTenantDomainStatusPending, VerificationMethod: SaaSTenantDomainVerificationDNS,
		VerificationToken: "token-123", Version: 1,
	}
	store := &fakeSaaSAdminTenantDomainStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}},
		domains:            []SaaSTenantDomain{domain},
		domain:             domain,
		createResult:       SaaSAdminTenantDomainResult{Domain: domain, OperationID: 81},
		verificationResult: SaaSAdminTenantDomainResult{Domain: SaaSTenantDomain{
			ID: 18, TenantID: 9, Hostname: domain.Hostname, Status: SaaSTenantDomainStatusActive,
			VerificationMethod: SaaSTenantDomainVerificationDNS, VerificationToken: "token-123",
			VerifiedAt: "2026-07-11 20:00:00", IsPrimary: true, Version: 2,
		}, OperationID: 82},
	}
	lookup := &fakeTenantDomainTXTLookup{values: []string{domain.VerificationRecordValue()}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).
		WithTenantDomainVerifier(NewSaaSTenantDomainDNSVerifierWithLookup(lookup, time.Second))

	listRequest := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenantDomains?tenantId=9&status=pending&keyword=customer&limit=20", nil)
	listRequest.Header.Set("X-Mochat-Go-User-ID", "1")
	listResponse := httptest.NewRecorder()
	handler.TenantDomains(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || store.listCalls != 1 || store.lastOptions.TenantID != 9 || store.lastOptions.Status != SaaSTenantDomainStatusPending {
		t.Fatalf("list status=%d calls=%d options=%+v body=%s", listResponse.Code, store.listCalls, store.lastOptions, listResponse.Body.String())
	}

	createPayload := []byte(`{"action":"create","tenantId":9,"hostname":"Login.Customer.Example.com"}`)
	createRequest := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantDomain", bytes.NewReader(createPayload))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.Header.Set("X-Mochat-Go-User-ID", "1")
	createResponse := httptest.NewRecorder()
	handler.TenantDomain(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated || store.createCalls != 1 || store.lastCreate.Hostname != "login.customer.example.com" || len(store.lastCreate.Token) != 43 {
		t.Fatalf("create status=%d input=%+v body=%s", createResponse.Code, store.lastCreate, createResponse.Body.String())
	}
	if store.lastCreate.ActorUserID != 1 || store.lastCreate.ActorTenantID != 1 {
		t.Fatalf("create actor=%+v", store.lastCreate)
	}

	verifyPayload, _ := json.Marshal(SaaSAdminTenantDomainRequest{Action: SaaSTenantDomainActionVerify, ID: 18, ExpectedVersion: 1})
	verifyRequest := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/tenantDomain", bytes.NewReader(verifyPayload))
	verifyRequest.Header.Set("Content-Type", "application/json")
	verifyRequest.Header.Set("X-Mochat-Go-User-ID", "1")
	verifyResponse := httptest.NewRecorder()
	handler.TenantDomain(verifyResponse, verifyRequest)
	if verifyResponse.Code != http.StatusOK || store.verificationCalls != 1 || !store.lastVerification.Success {
		t.Fatalf("verify status=%d input=%+v body=%s", verifyResponse.Code, store.lastVerification, verifyResponse.Body.String())
	}
	if store.lastVerification.ActorUserID != 1 || store.lastVerification.ActorTenantID != 1 {
		t.Fatalf("verification actor=%+v", store.lastVerification)
	}
}

func TestSaaSAdminTenantDomainVerificationFailureIsPersisted(t *testing.T) {
	domain := SaaSTenantDomain{ID: 18, TenantID: 9, Hostname: "login.customer.example.com", Status: SaaSTenantDomainStatusPending, VerificationToken: "token-123", Version: 4}
	store := &fakeSaaSAdminTenantDomainStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}},
		domain:             domain,
		verificationResult: SaaSAdminTenantDomainResult{Domain: domain, OperationID: 90},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).
		WithTenantDomainVerifier(NewSaaSTenantDomainDNSVerifierWithLookup(&fakeTenantDomainTXTLookup{values: []string{"wrong"}}, time.Second))
	payload, _ := json.Marshal(SaaSAdminTenantDomainRequest{Action: SaaSTenantDomainActionVerify, ID: 18, ExpectedVersion: 4})
	request := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/tenantDomain", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Mochat-Go-User-ID", "1")
	response := httptest.NewRecorder()
	handler.TenantDomain(response, request)
	if response.Code != http.StatusUnprocessableEntity || store.verificationCalls != 1 || store.lastVerification.Success || store.lastVerification.ErrorMessage == "" {
		t.Fatalf("status=%d verification=%+v body=%s", response.Code, store.lastVerification, response.Body.String())
	}
}
