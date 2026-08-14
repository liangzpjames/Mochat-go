package providers

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
)

var ErrInvalidRegistration = errors.New("invalid provider registration")

// StatusProvider is the smallest runtime status contract. Implementations
// must derive the result from their actual runtime configuration and state.
type StatusProvider interface {
	Status() Status
}

// Registration classifies a runtime Provider before it can appear in a
// registry snapshot. Classification is deliberately explicit so a new
// Provider cannot silently become an unreviewed capability.
type Registration struct {
	Kind         string
	Source       Source
	Capabilities []string
	Provider     StatusProvider
}

type Registry struct {
	mu      sync.RWMutex
	entries map[string]Registration
}

func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]Registration)}
}

func (r *Registry) Register(registration Registration) error {
	if r == nil || strings.TrimSpace(registration.Kind) == "" || !validSource(registration.Source) || registration.Provider == nil {
		return ErrInvalidRegistration
	}
	kind := strings.TrimSpace(registration.Kind)
	capabilities := normalizeCapabilities(registration.Capabilities)
	if len(capabilities) == 0 || len(capabilities) != len(registration.Capabilities) {
		return ErrInvalidRegistration
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entries[kind]; exists {
		return ErrInvalidRegistration
	}
	registration.Kind = kind
	registration.Capabilities = capabilities
	r.entries[kind] = registration
	return nil
}

// Snapshot returns a deterministic, defensive copy. The context parameter is
// reserved for future scoped status providers; current StatusProvider values
// are runtime objects with their own configured scope.
func (r *Registry) Snapshot(_ context.Context) []Status {
	if r == nil {
		return []Status{}
	}
	r.mu.RLock()
	entries := make([]Registration, 0, len(r.entries))
	for _, registration := range r.entries {
		entries = append(entries, registration)
	}
	r.mu.RUnlock()
	sort.Slice(entries, func(i, j int) bool { return entries[i].Kind < entries[j].Kind })

	statuses := make([]Status, 0, len(entries))
	for _, registration := range entries {
		status := cloneStatus(registration.Provider.Status())
		status.Kind = registration.Kind
		status.Source = registration.Source
		status.Capabilities = append([]string(nil), registration.Capabilities...)
		if status.Code == "" {
			status.Code = "provider.status_unclassified"
		}
		if !validState(status.State) {
			status.State = StateUnavailable
			status.Code = "provider.invalid_state"
			status.Reason = "provider returned an invalid state"
			status.Action = "inspect provider runtime"
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func validSource(source Source) bool {
	switch source {
	case SourceExternal, SourceSimulated, SourceLocal, SourceCodeOnly:
		return true
	default:
		return false
	}
}

func validState(state State) bool {
	switch state {
	case StateReady, StateLimited, StateUnavailable:
		return true
	default:
		return false
	}
}

func normalizeCapabilities(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil
		}
		if _, exists := seen[value]; exists {
			return nil
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func cloneStatus(status Status) Status {
	status.Capabilities = append([]string(nil), status.Capabilities...)
	status.Missing = append([]string(nil), status.Missing...)
	return status
}
