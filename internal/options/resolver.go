package options

import "github.com/nchapman/lleme/internal/config"

// Resolver resolves option values with priority: session > persona > config.
type Resolver struct {
	Persona *config.Persona
	Config  *config.Config
}

// NewResolver creates a new option resolver.
func NewResolver(persona *config.Persona, cfg *config.Config) *Resolver {
	return &Resolver{
		Persona: persona,
		Config:  cfg,
	}
}

// ResolveFloat returns the first non-zero value from: sessionVal, persona, config.
// Note: Zero is treated as "not set", not as an explicit value.
func (r *Resolver) ResolveFloat(sessionVal float64, key string) float64 {
	if sessionVal != 0 {
		return sessionVal
	}
	if r.Persona != nil {
		if v := r.Persona.GetFloatOption(key, 0); v != 0 {
			return v
		}
	}
	return r.Config.LlamaCpp.GetFloatOption(key, 0)
}

// ResolveInt returns the first non-zero value from: sessionVal, persona, config.
func (r *Resolver) ResolveInt(sessionVal int, key string) int {
	if sessionVal != 0 {
		return sessionVal
	}
	return r.GetConfigInt(key)
}

// GetConfigInt returns the first non-zero value from: persona, config.
func (r *Resolver) GetConfigInt(key string) int {
	if r.Persona != nil {
		if val, ok := r.Persona.Options[key]; ok {
			switch v := val.(type) {
			case int:
				return v
			case float64:
				return int(v)
			}
		}
	}
	return r.Config.LlamaCpp.GetIntOption(key, 0)
}

// GetConfigFloat returns the first non-zero value from: persona, config.
func (r *Resolver) GetConfigFloat(key string) float64 {
	if r.Persona != nil {
		if v := r.Persona.GetFloatOption(key, 0); v != 0 {
			return v
		}
	}
	return r.Config.LlamaCpp.GetFloatOption(key, 0)
}
