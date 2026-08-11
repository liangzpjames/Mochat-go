package dashboardprincipal

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type resolverBindingStore struct {
		binding Binding
		err     error
		calls   int
}

func (store *resolverBindingStore) ResolveBinding(context.Context, int) (Binding, error) {
	store.calls++
	return store.binding, store.err
}

type resolverTenantGate struct {
		allowed bool
		err     error
		calls   []int
}

type resolverIdentityStore struct {
	identity AuthenticatedIdentity
	err      error
	calls    int
}

func (store *resolverIdentityStore) ResolveIdentity(context.Context, int) (AuthenticatedIdentity, error) {
	store.calls++
	return store.identity, store.err
}

func (gate *resolverTenantGate) Authorize(_ context.Context, tenantID int, _ time.Time) (bool, error) {
	gate.calls = append(gate.calls, tenantID)
	return gate.allowed, gate.err
}

func TestResolverOrdersSaaSGateBeforeUniqueBindingAndBuildsPrincipal(t *testing.T) {
	gate := &resolverTenantGate{allowed: true}
	bindings := &resolverBindingStore{binding: Binding{
		TenantID: 902,
		CorpID:   77,
		Status:   CorpBindingStatusActive,
		Version:  3,
	}}
	resolver, err := NewResolver(gate, bindings)
	if err != nil {
		t.Fatal(err)
	}

	got, err := resolver.Resolve(context.Background(), AuthenticatedIdentity{
		UserID:       7,
		TenantID:     902,
		AuthVersion:  4,
		Active:       true,
		IsSuperAdmin: true,
	}, time.Unix(123, 0))
	if err != nil {
		t.Fatal(err)
	}
	want := DashboardPrincipal{
		UserID:       7,
		TenantID:     902,
		CorpID:       77,
		CorpStatus:   CorpBindingStatusActive,
		IsSuperAdmin: true,
		AuthVersion:  4,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("principal = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(gate.calls, []int{902}) || bindings.calls != 1 {
		t.Fatalf("gate calls=%v binding calls=%d, want one gate then one binding lookup", gate.calls, bindings.calls)
	}
}

func TestResolverFailsClosedBeforeBindingWhenSaaSGateDenies(t *testing.T) {
	gate := &resolverTenantGate{allowed: false}
	bindings := &resolverBindingStore{binding: Binding{TenantID: 902, CorpID: 77, Status: CorpBindingStatusActive, Version: 1}}
	resolver, err := NewResolver(gate, bindings)
	if err != nil {
		t.Fatal(err)
	}

	_, err = resolver.Resolve(context.Background(), AuthenticatedIdentity{UserID: 7, TenantID: 902, AuthVersion: 1, Active: true}, time.Unix(123, 0))
	if !errors.Is(err, ErrTenantAccessDenied) {
		t.Fatalf("err = %v, want ErrTenantAccessDenied", err)
	}
	if bindings.calls != 0 {
		t.Fatalf("binding lookup count = %d, want 0 when SaaS gate denies", bindings.calls)
	}
}

func TestResolverRejectsMissingDuplicateSuspendedAndMismatchedBindings(t *testing.T) {
	cases := []struct {
		name    string
		binding Binding
		err     error
	}{
		{name: "missing", err: ErrBindingUnavailable},
		{name: "duplicate", binding: Binding{TenantID: 902, CorpID: 77, Status: CorpBindingStatusActive, Version: 2, Count: 2}},
		{name: "suspended", binding: Binding{TenantID: 902, CorpID: 77, Status: CorpBindingStatusSuspended, Version: 2}},
		{name: "tenant mismatch", binding: Binding{TenantID: 903, CorpID: 77, Status: CorpBindingStatusActive, Version: 2}},
		{name: "corp missing", binding: Binding{TenantID: 902, CorpID: 0, Status: CorpBindingStatusActive, Version: 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gate := &resolverTenantGate{allowed: true}
			bindings := &resolverBindingStore{binding: tc.binding, err: tc.err}
			resolver, err := NewResolver(gate, bindings)
			if err != nil {
				t.Fatal(err)
			}
			_, err = resolver.Resolve(context.Background(), AuthenticatedIdentity{UserID: 7, TenantID: 902, AuthVersion: 1, Active: true}, time.Unix(123, 0))
			if !errors.Is(err, ErrPrincipalUnavailable) {
				t.Fatalf("err = %v, want ErrPrincipalUnavailable", err)
			}
		})
	}
}

func TestResolverRejectsInactiveOrUnversionedIdentityBeforeSaaSGate(t *testing.T) {
	cases := []AuthenticatedIdentity{
		{UserID: 7, TenantID: 902, AuthVersion: 1, Active: false},
		{UserID: 7, TenantID: 902, AuthVersion: 0, Active: true},
		{UserID: 0, TenantID: 902, AuthVersion: 1, Active: true},
	}
	for _, identity := range cases {
		gate := &resolverTenantGate{allowed: true}
		bindings := &resolverBindingStore{}
		resolver, err := NewResolver(gate, bindings)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := resolver.Resolve(context.Background(), identity, time.Unix(123, 0)); !errors.Is(err, ErrPrincipalUnavailable) {
			t.Fatalf("identity %+v: err = %v, want ErrPrincipalUnavailable", identity, err)
		}
		if len(gate.calls) != 0 || bindings.calls != 0 {
			t.Fatalf("identity %+v reached downstream checks: gate=%v binding=%d", identity, gate.calls, bindings.calls)
		}
	}
}

func TestResolverResolveUserOrdersIdentityThenSaaSGateThenBinding(t *testing.T) {
	identity := &resolverIdentityStore{identity: AuthenticatedIdentity{UserID: 7, TenantID: 902, AuthVersion: 4, Active: true}}
	gate := &resolverTenantGate{allowed: true}
	bindings := &resolverBindingStore{binding: Binding{TenantID: 902, CorpID: 77, Status: CorpBindingStatusActive, Version: 3}}
	resolver, err := NewResolverWithIdentity(identity, gate, bindings)
	if err != nil {
		t.Fatal(err)
	}

	principal, err := resolver.ResolveUser(context.Background(), 7, time.Unix(123, 0))
	if err != nil {
		t.Fatal(err)
	}
	if principal.UserID != 7 || principal.TenantID != 902 || principal.CorpID != 77 {
		t.Fatalf("principal=%+v", principal)
	}
	if identity.calls != 1 || len(gate.calls) != 1 || bindings.calls != 1 {
		t.Fatalf("identity=%d gate=%v bindings=%d, want identity -> gate -> binding", identity.calls, gate.calls, bindings.calls)
	}
}
