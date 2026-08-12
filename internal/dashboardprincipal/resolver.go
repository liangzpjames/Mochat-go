package dashboardprincipal

import (
	"context"
	"errors"
	"time"
)

var (
	ErrTenantAccessDenied = errors.New("dashboard tenant access denied")
	ErrBindingUnavailable = errors.New("dashboard tenant corp binding unavailable")
)

// AuthenticatedIdentity is the identity-domain fact produced after the
// Dashboard session has validated the user and auth_version. It deliberately
// does not accept tenant or corp values from an HTTP request.
type AuthenticatedIdentity struct {
	UserID       int
	TenantID     int
	AuthVersion  uint64
	IsSuperAdmin bool
	Active       bool
}

// Binding is the single server-side tenant-to-corp fact used to build a
// DashboardPrincipal.
type Binding struct {
	TenantID int
	CorpID   int
	Status   CorpBindingStatus
	Version  uint64
	Count    int
}

type BindingStore interface {
	ResolveBinding(context.Context, int) (Binding, error)
}

type IdentityStore interface {
	ResolveIdentity(context.Context, int) (AuthenticatedIdentity, error)
}

type TenantGate interface {
	Authorize(context.Context, int, time.Time) (bool, error)
}

type TenantGateFunc func(context.Context, int, time.Time) (bool, error)

func (gate TenantGateFunc) Authorize(ctx context.Context, tenantID int, now time.Time) (bool, error) {
	return gate(ctx, tenantID, now)
}

type PrincipalResolver interface {
	ResolveUser(context.Context, int, time.Time) (DashboardPrincipal, error)
}

type Resolver struct {
	identity IdentityStore
	gate     TenantGate
	bindings BindingStore
}

func NewResolver(gate TenantGate, bindings BindingStore) (*Resolver, error) {
	if gate == nil || bindings == nil {
		return nil, ErrPrincipalUnavailable
	}
	return &Resolver{gate: gate, bindings: bindings}, nil
}

func NewResolverWithIdentity(identity IdentityStore, gate TenantGate, bindings BindingStore) (*Resolver, error) {
	if identity == nil {
		return nil, ErrPrincipalUnavailable
	}
	resolver, err := NewResolver(gate, bindings)
	if err != nil {
		return nil, err
	}
	resolver.identity = identity
	return resolver, nil
}

func (resolver *Resolver) ResolveUser(ctx context.Context, userID int, now time.Time) (DashboardPrincipal, error) {
	if resolver == nil || resolver.identity == nil || userID <= 0 {
		return DashboardPrincipal{}, ErrPrincipalUnavailable
	}
	identity, err := resolver.identity.ResolveIdentity(ctx, userID)
	if err != nil || identity.UserID != userID {
		return DashboardPrincipal{}, ErrPrincipalUnavailable
	}
	return resolver.Resolve(ctx, identity, now)
}

// Resolve enforces the order tenant SaaS gate -> unique binding -> principal.
// Identity validation has already happened in the Dashboard auth/session
// layer; the resolver still rechecks the facts it consumes before producing a
// principal.
func (resolver *Resolver) Resolve(ctx context.Context, identity AuthenticatedIdentity, now time.Time) (DashboardPrincipal, error) {
	if resolver == nil || resolver.gate == nil || resolver.bindings == nil || !validIdentity(identity) {
		return DashboardPrincipal{}, ErrPrincipalUnavailable
	}
	allowed, err := resolver.gate.Authorize(ctx, identity.TenantID, now)
	if err != nil {
		return DashboardPrincipal{}, ErrPrincipalUnavailable
	}
	if !allowed {
		return DashboardPrincipal{}, ErrTenantAccessDenied
	}

	binding, err := resolver.bindings.ResolveBinding(ctx, identity.TenantID)
	if err != nil || !validBindingForTenant(binding, identity.TenantID) {
		return DashboardPrincipal{}, ErrPrincipalUnavailable
	}

	return DashboardPrincipal{
		UserID:       identity.UserID,
		TenantID:     identity.TenantID,
		CorpID:       binding.CorpID,
		CorpStatus:   binding.Status,
		IsSuperAdmin: identity.IsSuperAdmin,
		AuthVersion:  identity.AuthVersion,
	}, nil
}

func validIdentity(identity AuthenticatedIdentity) bool {
	return identity.UserID > 0 && identity.TenantID > 0 && identity.AuthVersion > 0 && identity.Active
}

func validBindingForTenant(binding Binding, tenantID int) bool {
	if binding.Count > 1 || binding.TenantID != tenantID || binding.CorpID <= 0 || binding.Version == 0 {
		return false
	}
	switch binding.Status {
	case CorpBindingStatusPending, CorpBindingStatusActive, CorpBindingStatusSuspended:
		return true
	default:
		return false
	}
}
