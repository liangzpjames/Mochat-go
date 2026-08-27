package dashboardadmin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type recordingStore struct {
	provisionCalls   int
	provision        ProvisionResult
	provisionErr     error
	resend           ResendActivationResult
	resendErr        error
	replace          GovernanceResult
	replaceErr       error
	status           GovernanceResult
	statusErr        error
	governance       DashboardAdminGovernanceView
	governanceErr    error
	seenActor        Actor
	seenInput        ProvisionDashboardTenant
	seenResendInput  ResendActivationInput
	seenReplaceInput ReplaceSuperAdminInput
	seenStatusInput  SuperAdminStatusInput
}

func (s *recordingStore) ProvisionDashboardTenant(_ context.Context, actor Actor, input ProvisionDashboardTenant) (ProvisionResult, error) {
	s.provisionCalls++
	s.seenActor = actor
	s.seenInput = input
	return s.provision, s.provisionErr
}

func (s *recordingStore) DashboardAdminGovernance(_ context.Context, actor Actor, tenantID int) (DashboardAdminGovernanceView, error) {
	s.seenActor = actor
	if tenantID <= 0 {
		return DashboardAdminGovernanceView{}, ErrInvalidRequest
	}
	return s.governance, s.governanceErr
}

func (s *recordingStore) ResendDashboardActivation(_ context.Context, actor Actor, input ResendActivationInput) (ResendActivationResult, error) {
	s.seenActor = actor
	s.seenResendInput = input
	return s.resend, s.resendErr
}

func (s *recordingStore) ReplaceDashboardSuperAdmin(_ context.Context, actor Actor, input ReplaceSuperAdminInput) (GovernanceResult, error) {
	s.seenActor = actor
	s.seenReplaceInput = input
	return s.replace, s.replaceErr
}

func (s *recordingStore) SetDashboardSuperAdminStatus(_ context.Context, actor Actor, input SuperAdminStatusInput) (GovernanceResult, error) {
	s.seenActor = actor
	s.seenStatusInput = input
	return s.status, s.statusErr
}

func TestProvisionRejectsInactiveOrExplicitlyDeniedSaaSActorBeforeStore(t *testing.T) {
	store := &recordingStore{}
	service := NewService(store)
	input := validProvisionInput()

	for _, actor := range []Actor{
		{UserID: 7, Active: false, Permissions: []string{PermissionTenantsManage}},
		{UserID: 7, Active: true, Permissions: []string{"platform.audit.read"}},
	} {
		_, err := service.ProvisionDashboardTenant(context.Background(), actor, input)
		if !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("actor rejected with error %v, want ErrPermissionDenied", err)
		}
	}
	if store.provisionCalls != 0 {
		t.Fatalf("store calls=%d, authorization must fail before target work", store.provisionCalls)
	}
}

func TestDashboardAdminGovernanceRequiresSaaSActorAndDelegatesTenantPath(t *testing.T) {
	store := &recordingStore{governance: DashboardAdminGovernanceView{TenantID: 41, BindingVersion: 7, Identities: []DashboardIdentityRecord{{ID: 52, IsSuperAdmin: true}}}}
	service := NewService(store)
	view, err := service.DashboardAdminGovernance(context.Background(), validActor(), 41)
	if err != nil || view.TenantID != 41 || view.BindingVersion != 7 || len(view.Identities) != 1 {
		t.Fatalf("view=%+v error=%v, want authoritative governance list", view, err)
	}
	if _, err := service.DashboardAdminGovernance(context.Background(), Actor{UserID: 7, Active: false, Permissions: []string{"*"}}, 41); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("inactive actor error=%v, want ErrPermissionDenied", err)
	}
}

func TestDashboardAdminGovernanceReturnsActionableIdentityCapabilities(t *testing.T) {
	store := &recordingStore{governance: DashboardAdminGovernanceView{TenantID: 41, BindingVersion: 7, Identities: []DashboardIdentityRecord{
		{ID: 51, IsSuperAdmin: true, UserStatus: 1, IdentityStatus: 1, ActivatedAt: "2026-08-27T00:00:00Z"},
		{ID: 52, IsSuperAdmin: false, UserStatus: 1, IdentityStatus: 1, ActivatedAt: "2026-08-27T00:00:00Z"},
		{ID: 53, IsSuperAdmin: true, UserStatus: 1, IdentityStatus: 1},
		{ID: 54, IsSuperAdmin: true, UserStatus: 2, IdentityStatus: 2, ActivatedAt: "2026-08-26T00:00:00Z"},
	}}}
	view, err := NewService(store).DashboardAdminGovernance(context.Background(), validActor(), 41)
	if err != nil {
		t.Fatal(err)
	}
	assertAction := func(id int, action string, want bool) {
		t.Helper()
		for _, identity := range view.Identities {
			if identity.ID != id {
				continue
			}
			got := false
			for _, item := range identity.AvailableActions {
				got = got || item == action
			}
			if got != want {
				t.Fatalf("identity=%d action=%s got=%v want=%v capabilities=%+v", id, action, got, want, identity)
			}
			return
		}
		t.Fatalf("identity=%d not found", id)
	}
	assertAction(51, DashboardGovernanceActionDisable, false)
	assertAction(51, DashboardGovernanceActionReplaceCurrent, true)
	assertAction(52, DashboardGovernanceActionReplacementCandidate, true)
	assertAction(53, DashboardGovernanceActionResendActivation, true)
	assertAction(54, DashboardGovernanceActionRestore, true)
	if view.Identities[0].BlockedReasons[DashboardGovernanceActionDisable] != DashboardGovernanceBlockLastSuperAdmin {
		t.Fatalf("disable block=%q", view.Identities[0].BlockedReasons[DashboardGovernanceActionDisable])
	}
}

func TestPlatformSuperAdminWildcardPermissionAuthorizesTenantGovernance(t *testing.T) {
	if !(Actor{UserID: 7, Active: true, Permissions: []string{"*"}}).HasPermission(PermissionTenantsManage) {
		t.Fatal("platform wildcard permission was not recognized")
	}
}

func TestProvisionRejectsBodyTenantAndActorFields(t *testing.T) {
	service := NewService(&recordingStore{})
	input := validProvisionInput()
	input.BodyTenantID = 99
	input.BodyActorID = 88

	_, err := service.ProvisionDashboardTenant(context.Background(), validActor(), input)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error=%v, want ErrInvalidRequest", err)
	}
}

func TestProvisionIdempotencyDoesNotReplayActivationToken(t *testing.T) {
	store := &recordingStore{
		provision: ProvisionResult{
			TenantID:        41,
			DashboardUserID: 52,
			ActivationToken: "opaque-token-only-for-test",
		},
	}
	service := NewService(store)

	first, err := service.ProvisionDashboardTenant(context.Background(), validActor(), validProvisionInput())
	if err != nil {
		t.Fatalf("first provision: %v", err)
	}
	if first.ActivationToken == "" {
		t.Fatal("first successful provisioning must return the one-time activation token")
	}

	store.provision = ProvisionResult{
		TenantID:        first.TenantID,
		DashboardUserID: first.DashboardUserID,
		Idempotent:      true,
	}
	second, err := service.ProvisionDashboardTenant(context.Background(), validActor(), validProvisionInput())
	if err != nil {
		t.Fatalf("idempotent provision: %v", err)
	}
	if second.ActivationToken != "" {
		t.Fatal("idempotent provisioning replayed a raw activation token")
	}
	if store.provisionCalls != 2 {
		t.Fatalf("store calls=%d, want two durable idempotency lookups", store.provisionCalls)
	}
}

func TestProvisionActivationBuildsFragmentPathOnlyForFirstDelivery(t *testing.T) {
	expiresAt := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	store := &recordingStore{provision: ProvisionResult{TenantID: 41, DashboardUserID: 52, ActivationToken: "opaque +/token", ActivationExpiresAt: expiresAt}}
	service := NewService(store)
	result, err := service.ProvisionDashboardTenant(context.Background(), validActor(), validProvisionInput())
	if err != nil {
		t.Fatal(err)
	}
	if result.ActivationPath != "/activate#token=opaque+%2B%2Ftoken" || !result.ActivationExpiresAt.Equal(expiresAt) {
		t.Fatalf("result=%+v", result)
	}
	if strings.Contains(result.ActivationPath, "?token=") {
		t.Fatal("activation path put token in query")
	}
	store.provision.Idempotent = true
	replay, err := service.ProvisionDashboardTenant(context.Background(), validActor(), validProvisionInput())
	if err != nil {
		t.Fatal(err)
	}
	if replay.ActivationToken != "" || replay.ActivationPath != "" || !replay.ActivationExpiresAt.IsZero() {
		t.Fatalf("replay leaked delivery: %+v", replay)
	}
}

func TestResendActivationBuildsFragmentPathAndClearsReplay(t *testing.T) {
	expiresAt := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	store := &recordingStore{resend: ResendActivationResult{TenantID: 41, DashboardUserID: 52, Version: 4, ActivationToken: "resend-token", ActivationExpiresAt: expiresAt}}
	service := NewService(store)
	input := ResendActivationInput{TenantID: 41, TargetUserID: 52, ExpectedVersion: 3, RequestID: "resend-request"}
	result, err := service.ResendActivation(context.Background(), validActor(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.ActivationPath != "/activate#token=resend-token" || !result.ActivationExpiresAt.Equal(expiresAt) {
		t.Fatalf("result=%+v", result)
	}
	store.resend.Idempotent = true
	replay, err := service.ResendActivation(context.Background(), validActor(), input)
	if err != nil {
		t.Fatal(err)
	}
	if replay.ActivationToken != "" || replay.ActivationPath != "" || !replay.ActivationExpiresAt.IsZero() {
		t.Fatalf("replay leaked delivery: %+v", replay)
	}
}

func TestProvisionDuplicatePhoneIsConflictWithoutResult(t *testing.T) {
	store := &recordingStore{provisionErr: ErrLoginIdentifierConflict}
	service := NewService(store)

	_, err := service.ProvisionDashboardTenant(context.Background(), validActor(), validProvisionInput())
	if !errors.Is(err, ErrLoginIdentifierConflict) {
		t.Fatalf("error=%v, want ErrLoginIdentifierConflict", err)
	}
}

func TestReplaceDelegatesActivationCheckToStore(t *testing.T) {
	store := &recordingStore{replaceErr: ErrReplacementRequiresActivation}
	service := NewService(store)

	_, err := service.ReplaceDashboardSuperAdmin(context.Background(), validActor(), ReplaceSuperAdminInput{
		TenantID:        41,
		CurrentAdminID:  52,
		NewAdminID:      63,
		ExpectedVersion: 3,
		RequestID:       "replace-service-test",
	})
	if !errors.Is(err, ErrReplacementRequiresActivation) {
		t.Fatalf("error=%v, want ErrReplacementRequiresActivation", err)
	}
}

func TestDeactivateLastValidSuperAdminIsConflict(t *testing.T) {
	store := &recordingStore{statusErr: ErrLastSuperAdmin}
	service := NewService(store)

	_, err := service.SetDashboardSuperAdminStatus(context.Background(), validActor(), SuperAdminStatusInput{
		TenantID:        41,
		TargetUserID:    52,
		Enabled:         false,
		ExpectedVersion: 4,
		RequestID:       "status-service-test",
	})
	if !errors.Is(err, ErrLastSuperAdmin) {
		t.Fatalf("error=%v, want ErrLastSuperAdmin", err)
	}
}

func TestGovernanceRequiresDurableRequestKey(t *testing.T) {
	service := NewService(&recordingStore{})
	for _, input := range []any{
		ReplaceSuperAdminInput{TenantID: 41, CurrentAdminID: 52, NewAdminID: 63, ExpectedVersion: 3},
		SuperAdminStatusInput{TenantID: 41, TargetUserID: 52, Enabled: false, ExpectedVersion: 3},
	} {
		var err error
		switch value := input.(type) {
		case ReplaceSuperAdminInput:
			_, err = service.ReplaceDashboardSuperAdmin(context.Background(), validActor(), value)
		case SuperAdminStatusInput:
			_, err = service.SetDashboardSuperAdminStatus(context.Background(), validActor(), value)
		}
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("input=%T error=%v, want missing request key to fail closed", input, err)
		}
	}
}

func TestDashboardUserMutationCannotSetSuperAdmin(t *testing.T) {
	if err := ValidateDashboardUserMutation(DashboardUserMutation{IsSuperAdmin: true}); !errors.Is(err, ErrSuperAdminGovernanceOnly) {
		t.Fatalf("error=%v, want ErrSuperAdminGovernanceOnly", err)
	}
	if err := ValidateDashboardUserMutation(DashboardUserMutation{IsSuperAdmin: false}); err != nil {
		t.Fatalf("ordinary Dashboard user mutation rejected: %v", err)
	}
}

func TestPackageLimitsJSONRequiresEveryQuotaKey(t *testing.T) {
	fields := make(map[string]int64, len(packageLimitJSONKeys))
	for _, key := range packageLimitJSONKeys {
		fields[key] = 0
	}
	delete(fields, "roomSops")
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	var limits SaaSAdminPackageLimits
	if err := json.Unmarshal(data, &limits); err == nil {
		t.Fatal("quota JSON missing a required key was accepted")
	}
}

func TestPackageLimitsJSONAcceptsExplicitZeroAndRejectsUnknown(t *testing.T) {
	fields := make(map[string]int64, len(packageLimitJSONKeys)+1)
	for _, key := range packageLimitJSONKeys {
		fields[key] = 0
	}
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	var limits SaaSAdminPackageLimits
	if err := json.Unmarshal(data, &limits); err != nil {
		t.Fatalf("explicit zero quota snapshot rejected: %v", err)
	}
	fields["unexpected"] = 0
	data, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &limits); err == nil {
		t.Fatal("quota JSON unknown key was accepted")
	}
}

func validActor() Actor {
	return Actor{UserID: 7, Active: true, Permissions: []string{PermissionTenantsManage}}
}

func validProvisionInput() ProvisionDashboardTenant {
	return ProvisionDashboardTenant{
		TenantName:           "客户租户",
		WeComIntegrationMode: WeComIntegrationModeSelfBuilt,
		PackageID:            11,
		AdminLoginIdentifier: "13800000000",
		AdminName:            "租户管理员",
		IdempotencyKey:       "provision-key-1",
		ExpectedVersion:      1,
		Subscription:         SubscriptionInput{PackageCode: "pro", Status: "trialing", BillingCycle: "custom", StartsAt: "2026-08-11T00:00:00Z", ExpiresAt: "2026-09-11T00:00:00Z"},
	}
}

func TestProvisionRequiresImmutableWeComIntegrationMode(t *testing.T) {
	service := NewService(&recordingStore{})
	for _, mode := range []string{"", "self-built", "third_party", "unknown"} {
		input := validProvisionInput()
		input.WeComIntegrationMode = mode
		if _, err := service.ProvisionDashboardTenant(context.Background(), validActor(), input); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("mode=%q error=%v, want ErrInvalidRequest", mode, err)
		}
	}
	for _, mode := range []string{WeComIntegrationModeSelfBuilt, WeComIntegrationModeThirdPartyDelegated} {
		store := &recordingStore{}
		service := NewService(store)
		input := validProvisionInput()
		input.WeComIntegrationMode = mode
		if _, err := service.ProvisionDashboardTenant(context.Background(), validActor(), input); err != nil {
			t.Fatalf("mode=%q error=%v", mode, err)
		}
		if store.seenInput.WeComIntegrationMode != mode {
			t.Fatalf("stored mode=%q, want %q", store.seenInput.WeComIntegrationMode, mode)
		}
	}
}
