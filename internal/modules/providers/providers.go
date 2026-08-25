package providers

import (
	"context"
	"errors"
	"io"
	"time"
)

// State describes whether a provider is usable for the current deployment.
type State string

const (
	StateReady       State = "ready"
	StateLimited     State = "limited"
	StateUnavailable State = "unavailable"
)

// Source distinguishes a real external integration from local or simulated
// implementations. It is part of the runtime contract and is never inferred
// from a page manifest.
type Source string

const (
	SourceExternal  Source = "external"
	SourceSimulated Source = "simulated"
	SourceLocal     Source = "local"
	SourceCodeOnly  Source = "code_only"
)

// Status is the structured provider health used by pages and acceptance tooling.
type Status struct {
	Kind               string             `json:"kind"`
	State              State              `json:"state"`
	Code               string             `json:"code,omitempty"`
	Source             Source             `json:"source,omitempty"`
	Reason             string             `json:"reason,omitempty"`
	Action             string             `json:"action,omitempty"`
	Capabilities       []string           `json:"capabilities,omitempty"`
	Missing            []string           `json:"missing,omitempty"`
	LastSyncAt         *time.Time         `json:"lastSyncAt,omitempty"`
	LastSuccessAt      *time.Time         `json:"lastSuccessAt,omitempty"`
	LastFailureAt      *time.Time         `json:"lastFailureAt,omitempty"`
	LastErrorCode      string             `json:"lastErrorCode,omitempty"`
	CapabilityStatuses []CapabilityStatus `json:"capabilityStatuses,omitempty"`
	// Callback route/worker configuration is runtime-only prerequisite input.
	// Neither flag is a successful receive/verification evidence.
	CallbackRouteConfigured  bool `json:"-"`
	CallbackWorkerConfigured bool `json:"-"`
}

// CapabilityStatus is a tenant-scoped status for one classified capability.
// It is separate from the provider-level status so one successful operation
// cannot make unrelated capabilities appear ready.
type CapabilityStatus struct {
	Capability    string     `json:"capability"`
	State         State      `json:"state"`
	Code          string     `json:"code,omitempty"`
	Source        Source     `json:"source,omitempty"`
	Reason        string     `json:"reason,omitempty"`
	Action        string     `json:"action,omitempty"`
	Missing       []string   `json:"missing,omitempty"`
	LastSyncAt    *time.Time `json:"lastSyncAt,omitempty"`
	LastSuccessAt *time.Time `json:"lastSuccessAt,omitempty"`
	LastFailureAt *time.Time `json:"lastFailureAt,omitempty"`
	LastErrorCode string     `json:"lastErrorCode,omitempty"`
}

// ErrNotConfigured is returned when a provider is registered but its required
// configuration is absent or activation has not happened yet.
var ErrNotConfigured = errors.New("provider not configured")

// ErrCapabilityUnavailable is returned when an adapter is intentionally
// registered but the real external capability has not been implemented or
// activated. Callers must not convert it into a successful sync result.
var ErrCapabilityUnavailable = errors.New("provider capability unavailable")

// PutOptions carries metadata for a stored object.
type PutOptions struct {
	ContentType string
	SizeBytes   int64
}

// AudioProvider stores and retrieves opaque audio bytes under relative keys.
type AudioProvider interface {
	Put(ctx context.Context, key string, reader io.Reader, opts PutOptions) error
	Open(ctx context.Context, key string) (io.ReadCloser, int64, error)
	Delete(ctx context.Context, key string) error
	Status() Status
}

// ChatRequest is a single-turn completion request.
type ChatRequest struct {
	Model    string
	System   string
	Prompt   string
	JSONMode bool
}

// AIProviderMetadata is optional, non-sensitive runtime identification.
type AIProviderMetadata struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type AIProviderMetadataReader interface {
	Metadata() AIProviderMetadata
}

// AIProvider produces text completions through a model service.
type AIProvider interface {
	Chat(ctx context.Context, req ChatRequest) (string, error)
	Status() Status
}

// AIProviderResolver returns the provider authorized for one tenant/corp
// execution scope. Production implementations must not fall back to a
// process-wide provider.
type AIProviderResolver interface {
	Resolve(ctx context.Context, tenantID, corpID int64) (AIProvider, error)
}

// AIProviderResolveError exposes diagnostics that are explicitly safe for a
// tenant-facing run record. Resolver implementations must never return raw
// transport, URL, ciphertext, or credential errors through these methods.
type AIProviderResolveError interface {
	error
	SafeCode() string
	SafeReason() string
}

// StaticAIProviderResolver is intentionally small and is for explicit test
// construction only. Production composition uses a tenant-backed resolver.
type StaticAIProviderResolver struct{ Provider AIProvider }

func (r StaticAIProviderResolver) Resolve(context.Context, int64, int64) (AIProvider, error) {
	if r.Provider == nil {
		return nil, ErrNotConfigured
	}
	return r.Provider, nil
}

// SyncOptions controls an archive sync run.
type SyncOptions struct {
	CorpID int64
	Limit  int
}

// SyncResult reports how many archive messages were fetched and inserted.
type SyncResult struct {
	Fetched  int
	Inserted int
}

// ArchiveProvider synchronizes enterprise WeChat conversation archives.
type ArchiveProvider interface {
	Status() Status
	Sync(ctx context.Context, opts SyncOptions) (SyncResult, error)
}
