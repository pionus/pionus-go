package plugin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/pionus/pionus-go/internal/config"
	"github.com/pionus/pionus-go/internal/events"
)

const hookTimeout = 3 * time.Second

var pluginIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Manager owns the enabled plugin instances and dispatches events.
type Manager struct {
	plugins []Plugin
	envs    []Env
	log     *slog.Logger
}

// NewManager validates the configured plugins and constructs the enabled
// instances. Disabled plugin factories are not initialized or started.
func NewManager(factories []Factory, cfg config.PluginsConfig, env Env) (*Manager, error) {
	if env.Log == nil {
		env.Log = slog.Default()
	}
	m := &Manager{log: env.Log}
	if len(cfg.Enabled) == 0 {
		return m, nil
	}

	instances := make(map[string]Plugin, len(factories))
	for _, factory := range factories {
		if factory == nil {
			continue
		}
		p := factory()
		if p == nil {
			return nil, errors.New("plugin factory returned nil")
		}
		manifest := p.Manifest()
		if !pluginIDRe.MatchString(manifest.ID) {
			return nil, fmt.Errorf("invalid plugin id %q", manifest.ID)
		}
		if _, exists := instances[manifest.ID]; exists {
			return nil, fmt.Errorf("duplicate plugin id %q", manifest.ID)
		}
		instances[manifest.ID] = p
	}

	seen := make(map[string]bool, len(cfg.Enabled))
	for _, rawID := range cfg.Enabled {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if seen[id] {
			return nil, fmt.Errorf("plugin %q enabled more than once", id)
		}
		seen[id] = true

		p, ok := instances[id]
		if !ok {
			return nil, fmt.Errorf("plugin %q is enabled but not registered", id)
		}
		manifest := p.Manifest()
		if manifest.APIVersion != APIVersion {
			return nil, fmt.Errorf(
				"plugin %q uses API version %d, host supports %d",
				id, manifest.APIVersion, APIVersion,
			)
		}

		pluginEnv := env
		pluginEnv.Settings = copySettings(cfg.Config[id])
		m.plugins = append(m.plugins, p)
		m.envs = append(m.envs, pluginEnv)
	}
	return m, nil
}

func copySettings(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// Manifests returns the enabled plugin manifests in startup order.
func (m *Manager) Manifests() []Manifest {
	if m == nil {
		return nil
	}
	out := make([]Manifest, 0, len(m.plugins))
	for _, p := range m.plugins {
		out = append(out, p.Manifest())
	}
	return out
}

// Init initializes plugins in order. If one fails, plugins already initialized
// are stopped in reverse order.
func (m *Manager) Init(ctx context.Context) error {
	if m == nil {
		return nil
	}
	for i, p := range m.plugins {
		initializer, ok := p.(Initializer)
		if !ok {
			continue
		}
		if err := initializer.Init(ctx, &m.envs[i]); err != nil {
			_ = m.stopRange(ctx, i)
			return fmt.Errorf("plugin %s init: %w", p.Manifest().ID, err)
		}
	}
	return nil
}

// Start starts plugins in order. If one fails, all plugins are stopped.
func (m *Manager) Start(ctx context.Context) error {
	if m == nil {
		return nil
	}
	for _, p := range m.plugins {
		starter, ok := p.(Starter)
		if !ok {
			continue
		}
		if err := starter.Start(ctx); err != nil {
			_ = m.Stop(ctx)
			return fmt.Errorf("plugin %s start: %w", p.Manifest().ID, err)
		}
	}
	return nil
}

// Stop stops plugins in reverse startup order.
func (m *Manager) Stop(ctx context.Context) error {
	if m == nil {
		return nil
	}
	return m.stopRange(ctx, len(m.plugins)-1)
}

func (m *Manager) stopRange(ctx context.Context, last int) error {
	var errs []error
	for i := last; i >= 0; i-- {
		stopper, ok := m.plugins[i].(Stopper)
		if !ok {
			continue
		}
		if err := stopper.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("plugin %s stop: %w", m.plugins[i].Manifest().ID, err))
		}
	}
	return errors.Join(errs...)
}

// Emit dispatches an event to every enabled plugin that implements
// events.Handler. Handler failures are logged and do not fail the request.
func (m *Manager) Emit(ctx context.Context, event events.Event) {
	if m == nil || len(m.plugins) == 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for _, p := range m.plugins {
		handler, ok := p.(events.Handler)
		if !ok {
			continue
		}
		m.dispatch(ctx, p.Manifest().ID, event, handler)
	}
}

func (m *Manager) dispatch(ctx context.Context, id string, event events.Event, handler events.Handler) {
	ctx, cancel := context.WithTimeout(ctx, hookTimeout)
	defer cancel()

	defer func() {
		if r := recover(); r != nil {
			m.log.Error("plugin event panic",
				"plugin", id, "event", event.Type, "event_id", event.ID, "panic", r)
		}
	}()

	if err := handler.HandleEvent(ctx, event); err != nil {
		m.log.Error("plugin event failed",
			"plugin", id, "event", event.Type, "event_id", event.ID, "err", err)
		return
	}
	m.log.Debug("plugin event handled",
		"plugin", id, "event", event.Type, "event_id", event.ID)
}
