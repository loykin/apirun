package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/loykin/apirun/internal/util"
	"github.com/loykin/apirun/pkg/env"
)

type Auth struct {
	Type    string       `mapstructure:"type"`
	Name    string       `mapstructure:"name"`
	Methods MethodConfig `mapstructure:"methods"`
}

type MethodConfig interface {
	ToMap() map[string]interface{}
}

type mapConfig struct{ m map[string]interface{} }

func (s mapConfig) ToMap() map[string]interface{} { return s.m }

// NewAuthSpecFromMap provides a simple way to build a MethodConfig from a plain map.
// It is a transitional helper replacing previous AuthSpec.
func NewAuthSpecFromMap(m map[string]interface{}) MethodConfig { return mapConfig{m: m} }

// Acquire resolves and acquires authentication according to this Auth configuration.
// Behavior:
// - Uses Type as the provider key (e.g., "basic", "oauth2", "pocketbase").
// - Uses the single Methods configuration (since Type is already selected globally).
// - Renders any Go templates in the method config using only flat key/value pairs from the environment:
//   - First from process environment variables if present (so CLI can inject secrets),
//   - Then leaves unchanged any placeholders if not resolvable (RenderAnyTemplate keeps originals when missing).
//
// - Calls the provider registry to acquire the token value.
// - If Name is set, storage by name is handled by the migration layer using the returned value.
func (a *Auth) Acquire(ctx context.Context, e *env.Env) (string, error) {
	if a == nil {
		return "", nil
	}
	pt := strings.TrimSpace(a.Type)
	if pt == "" {
		return "", fmt.Errorf("auth: missing type")
	}
	if a.Methods == nil {
		return "", fmt.Errorf("auth: methods not provided")
	}
	cfg := a.Methods.ToMap()
	// Render templates in cfg using the provided env (global/local/auth)
	renderedAny := util.RenderAnyTemplate(cfg, e)
	rendered, _ := renderedAny.(map[string]interface{})
	if rendered == nil {
		rendered = cfg
	}
	val, err := AcquireAndStoreWithName(ctx, pt, rendered)
	if err != nil {
		return "", err
	}
	return val, nil
}
