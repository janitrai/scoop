package translation

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	// ProviderEnvVar selects the default translation provider.
	ProviderEnvVar = "TRANSLATION_PROVIDER"
	// DefaultProviderName is used when TRANSLATION_PROVIDER is unset.
	DefaultProviderName = "local"
)

// Registry stores translation providers and resolves a default provider.
type Registry struct {
	providers       map[string]Provider
	defaultProvider string
}

func NewRegistry(defaultProvider string) *Registry {
	normalizedDefault := normalizeProviderName(defaultProvider)
	if normalizedDefault == "" {
		normalizedDefault = DefaultProviderName
	}

	return &Registry{
		providers:       make(map[string]Provider),
		defaultProvider: normalizedDefault,
	}
}

// NewRegistryFromEnv creates a provider registry from environment configuration.
func NewRegistryFromEnv() *Registry {
	registry := NewRegistry(os.Getenv(ProviderEnvVar))
	_ = registry.Register(NewLocalProviderFromEnv())
	_ = registry.Register(NewGoogleProvider())

	if _, exists := registry.providers[registry.defaultProvider]; !exists {
		registry.defaultProvider = DefaultProviderName
	}
	if _, exists := registry.providers[registry.defaultProvider]; !exists {
		for name := range registry.providers {
			registry.defaultProvider = name
			break
		}
	}

	return registry
}

// Register adds one provider.
func (r *Registry) Register(provider Provider) error {
	if r == nil {
		return fmt.Errorf("registry is nil")
	}
	if provider == nil {
		return fmt.Errorf("provider is nil")
	}
	name := normalizeProviderName(provider.Name())
	if name == "" {
		return fmt.Errorf("provider name is required")
	}
	r.providers[name] = provider
	return nil
}

// Provider resolves a provider by name. Empty names use the configured default provider.
func (r *Registry) Provider(name string) (Provider, error) {
	if r == nil {
		return nil, fmt.Errorf("registry is nil")
	}
	if len(r.providers) == 0 {
		return nil, fmt.Errorf("no translation providers are registered")
	}

	resolvedName := normalizeProviderName(name)
	if resolvedName == "" {
		resolvedName = r.defaultProvider
	}
	provider, ok := r.providers[resolvedName]
	if ok {
		return provider, nil
	}

	return nil, fmt.Errorf("translation provider %q is not registered (available: %s)", resolvedName, strings.Join(r.ProviderNames(), ", "))
}

func (r *Registry) DefaultProvider() string {
	if r == nil {
		return ""
	}
	return r.defaultProvider
}

func (r *Registry) ProviderNames() []string {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func normalizeProviderName(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}
