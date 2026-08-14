package catalog

import (
	"fmt"

	"jiyi/mochat-go/internal/modules/providers"
)

// Dependencies are the real runtime components bound by the application. A
// nil dependency is represented as unavailable instead of being omitted, so
// the status API remains truthful about a registered but unwired capability.
type Dependencies struct {
	AI            providers.StatusProvider
	AIEnabled     bool
	Archive       providers.StatusProvider
	AudioStorage  providers.StatusProvider
	WeComStandard providers.StatusProvider
}

// NewRegistry registers every built-in Provider with an explicit source and
// capability classification. Runtime components are injected by the app.
func NewRegistry(dependencies Dependencies) (*providers.Registry, error) {
	registry := providers.NewRegistry()
	err := registry.Register(providers.Registration{Kind: "ai", Source: aiSource(dependencies.AIEnabled), Capabilities: []string{"chat"}, Provider: dependencyOrUnavailable(dependencies.AI, "ai")})
	if err != nil {
		return nil, fmt.Errorf("register provider ai: %w", err)
	}
	err = registry.Register(providers.Registration{Kind: "audio_storage", Source: providers.SourceLocal, Capabilities: []string{"audio_object_storage"}, Provider: dependencyOrUnavailable(dependencies.AudioStorage, "audio_storage")})
	if err != nil {
		return nil, fmt.Errorf("register provider audio_storage: %w", err)
	}
	err = registry.Register(providers.Registration{Kind: "wecom_archive", Source: providers.SourceExternal, Capabilities: []string{"archive_sync"}, Provider: dependencyOrUnavailable(dependencies.Archive, "wecom_archive")})
	if err != nil {
		return nil, fmt.Errorf("register provider wecom_archive: %w", err)
	}
	err = registry.Register(providers.Registration{Kind: "wecom_standard", Source: providers.SourceExternal, Capabilities: []string{"employee_sync"}, Provider: dependencyOrUnavailable(dependencies.WeComStandard, "wecom_standard")})
	if err != nil {
		return nil, fmt.Errorf("register provider wecom_standard: %w", err)
	}
	return registry, nil
}

func aiSource(enabled bool) providers.Source {
	if enabled {
		return providers.SourceExternal
	}
	return providers.SourceCodeOnly
}

// DisabledAIProvider is an explicit deployment-disabled runtime component. It
// never makes an outbound request and is distinct from a configured external
// AI client whose key is missing.
type DisabledAIProvider struct{}

func (DisabledAIProvider) Status() providers.Status {
	return providers.Status{
		Kind: "ai", State: providers.StateLimited, Source: providers.SourceCodeOnly,
		Code: "ai.disabled", Action: "enable AI insight in deployment configuration",
		Capabilities: []string{"chat"},
	}
}

type unavailableProvider struct {
	kind string
}

func (p unavailableProvider) Status() providers.Status {
	return providers.Status{
		Kind:   p.kind,
		State:  providers.StateUnavailable,
		Code:   "provider.runtime_component_missing",
		Action: "enable the runtime component",
	}
}

func dependencyOrUnavailable(provider providers.StatusProvider, kind string) providers.StatusProvider {
	if provider != nil {
		return provider
	}
	return unavailableProvider{kind: kind}
}
