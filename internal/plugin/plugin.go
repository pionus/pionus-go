// Package plugin defines the compile-time extension contract used by Pionus.
//
// A plugin is a Go package whose factory is listed in plugins/plugins.go. It is
// built into the binary; there is intentionally no runtime .so loading.
package plugin

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/config"
	"github.com/pionus/pionus-go/internal/store"
	"gopkg.in/yaml.v3"
)

// APIVersion is the plugin contract version. A plugin with a different version
// is rejected at startup.
const APIVersion = 1

// Manifest describes a plugin. ID must be stable and unique.
type Manifest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	APIVersion  int    `json:"api_version"`
	Description string `json:"description,omitempty"`
}

// Plugin is the required part of every extension.
type Plugin interface {
	Manifest() Manifest
}

// Initializer is called once after the plugin is constructed. It may register
// routes on Env.App and must return before the server starts listening.
type Initializer interface {
	Init(ctx context.Context, env *Env) error
}

// Starter is called after all plugins have been initialized. Background
// workers belong here.
type Starter interface {
	Start(ctx context.Context) error
}

// Stopper is called in reverse plugin order during shutdown.
type Stopper interface {
	Stop(ctx context.Context) error
}

// Factory constructs a plugin instance.
type Factory func() Plugin

// Env is the capability surface exposed to a trusted compile-time plugin.
type Env struct {
	Config   *config.Config
	Store    store.Store
	App      *arry.Arry
	Log      *slog.Logger
	Settings map[string]any
}

// DecodeSettings decodes the plugin's section of plugins.config into out.
func (e *Env) DecodeSettings(out any) error {
	data, err := yaml.Marshal(e.Settings)
	if err != nil {
		return fmt.Errorf("encode plugin settings: %w", err)
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode plugin settings: %w", err)
	}
	return nil
}
