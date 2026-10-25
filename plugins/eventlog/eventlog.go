// Package eventlog is a small reference plugin that logs application events.
// It is disabled by default; enable it with plugins.enabled: [eventlog].
package eventlog

import (
	"context"
	"log/slog"

	"github.com/pionus/pionus-go/internal/events"
	"github.com/pionus/pionus-go/internal/plugin"
)

// Plugin logs event metadata through the host logger.
type Plugin struct {
	log *slog.Logger
}

// New constructs the plugin.
func New() plugin.Plugin { return &Plugin{} }

// Manifest implements plugin.Plugin.
func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          "eventlog",
		Name:        "Event log",
		Version:     "1.0.0",
		APIVersion:  plugin.APIVersion,
		Description: "Logs post, comment and note events for diagnostics.",
	}
}

// Init implements plugin.Initializer.
func (p *Plugin) Init(_ context.Context, env *plugin.Env) error {
	p.log = env.Log.With("plugin", "eventlog")
	return nil
}

// HandleEvent implements events.Handler.
func (p *Plugin) HandleEvent(_ context.Context, event events.Event) error {
	p.log.Info("event",
		"type", event.Type,
		"source", event.Source,
		"event_id", event.ID,
	)
	return nil
}
