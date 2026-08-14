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
	Archive       providers.StatusProvider
	AudioStorage  providers.StatusProvider
	WeComStandard providers.StatusProvider
}

// NewRegistry registers every built-in Provider with an explicit source and
// capability classification. Runtime components are injected by the app.
func NewRegistry(dependencies Dependencies) (*providers.Registry, error) {
	registry := providers.NewRegistry()
	registrations := []providers.Registration{
		providers.Registration{Kind: "ai", Source: providers.SourceExternal, Capabilities: []string{"chat"}, Provider: dependencyOrUnavailable(dependencies.AI, "ai")},
		providers.Registration{Kind: "audio_storage", Source: providers.SourceLocal, Capabilities: []string{"audio_object_storage"}, Provider: dependencyOrUnavailable(dependencies.AudioStorage, "audio_storage")},
		providers.Registration{Kind: "wecom_archive", Source: providers.SourceExternal, Capabilities: []string{"archive_sync"}, Provider: dependencyOrUnavailable(dependencies.Archive, "wecom_archive")},
		providers.Registration{Kind: "wecom_standard", Source: providers.SourceExternal, Capabilities: []string{"employee_sync"}, Provider: dependencyOrUnavailable(dependencies.WeComStandard, "wecom_standard")},
	}
	for _, registration := range registrations {
		if err := registry.Register(registration); err != nil {
			return nil, fmt.Errorf("register provider %s: %w", registration.Kind, err)
		}
	}
	return registry, nil
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
