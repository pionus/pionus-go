package plugin

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/pionus/pionus-go/internal/config"
	"github.com/pionus/pionus-go/internal/events"
	"github.com/pionus/pionus-go/internal/model"
)

type testPlugin struct {
	manifest Manifest
	order    *[]string
	settings map[string]any

	initErr   error
	startErr  error
	stopErr   error
	eventErr  error
	panic     bool
	events    []events.Event
	initCalls int
	starts    int
	stopCalls int
}

func (p *testPlugin) Manifest() Manifest { return p.manifest }

func (p *testPlugin) Init(_ context.Context, env *Env) error {
	p.initCalls++
	p.settings = env.Settings
	if p.order != nil {
		*p.order = append(*p.order, "init:"+p.manifest.ID)
	}
	return p.initErr
}

func (p *testPlugin) Start(context.Context) error {
	p.starts++
	if p.order != nil {
		*p.order = append(*p.order, "start:"+p.manifest.ID)
	}
	return p.startErr
}

func (p *testPlugin) Stop(context.Context) error {
	p.stopCalls++
	if p.order != nil {
		*p.order = append(*p.order, "stop:"+p.manifest.ID)
	}
	return p.stopErr
}

func (p *testPlugin) HandleEvent(_ context.Context, event events.Event) error {
	if p.panic {
		panic("boom")
	}
	p.events = append(p.events, event)
	return p.eventErr
}

func testLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func factory(p *testPlugin) Factory {
	return func() Plugin { return p }
}

func enabled(ids ...string) config.PluginsConfig {
	return config.PluginsConfig{Enabled: ids}
}

func TestManagerLifecycleAndOrder(t *testing.T) {
	var order []string
	a := &testPlugin{manifest: Manifest{ID: "a", APIVersion: APIVersion}, order: &order}
	b := &testPlugin{manifest: Manifest{ID: "b", APIVersion: APIVersion}, order: &order}

	cfg := enabled("b", "a")
	cfg.Config = map[string]map[string]any{
		"a": {"url": "https://a.example"},
	}
	m, err := NewManager([]Factory{factory(a), factory(b)}, cfg, Env{Log: testLog()})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []string{"init:b", "init:a", "start:b", "start:a", "stop:a", "stop:b"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
	if a.settings["url"] != "https://a.example" {
		t.Fatalf("plugin settings = %+v", a.settings)
	}
	if len(m.Manifests()) != 2 || m.Manifests()[0].ID != "b" {
		t.Fatalf("manifests = %+v", m.Manifests())
	}
}

func TestManagerValidation(t *testing.T) {
	ok := &testPlugin{manifest: Manifest{ID: "ok", APIVersion: APIVersion}}

	tests := []struct {
		name      string
		factories []Factory
		cfg       config.PluginsConfig
	}{
		{"unknown", []Factory{factory(ok)}, enabled("missing")},
		{"duplicate", []Factory{factory(ok), factory(ok)}, enabled("ok")},
		{"api version", []Factory{factory(&testPlugin{manifest: Manifest{ID: "bad", APIVersion: 99}})}, enabled("bad")},
		{"invalid id", []Factory{factory(&testPlugin{manifest: Manifest{ID: "Bad_ID", APIVersion: APIVersion}})}, enabled("Bad_ID")},
		{"enabled twice", []Factory{factory(ok)}, enabled("ok", "ok")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewManager(tt.factories, tt.cfg, Env{Log: testLog()}); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestManagerInitAndStartFailuresRollBack(t *testing.T) {
	first := &testPlugin{manifest: Manifest{ID: "first", APIVersion: APIVersion}}
	second := &testPlugin{
		manifest: Manifest{ID: "second", APIVersion: APIVersion},
		initErr:  errors.New("init failed"),
	}
	m, err := NewManager(
		[]Factory{factory(first), factory(second)},
		enabled("first", "second"),
		Env{Log: testLog()},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Init(context.Background()); err == nil {
		t.Fatal("expected init error")
	}
	if first.stopCalls != 1 || second.stopCalls != 1 {
		t.Fatalf("stop calls = %d/%d", first.stopCalls, second.stopCalls)
	}

	third := &testPlugin{
		manifest: Manifest{ID: "third", APIVersion: APIVersion},
		startErr: errors.New("start failed"),
	}
	m, err = NewManager(
		[]Factory{factory(first), factory(third)},
		enabled("first", "third"),
		Env{Log: testLog()},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(context.Background()); err == nil {
		t.Fatal("expected start error")
	}
	if first.stopCalls != 2 || third.stopCalls != 1 {
		t.Fatalf("stop calls after start failure = %d/%d", first.stopCalls, third.stopCalls)
	}
}

func TestManagerEventDispatch(t *testing.T) {
	bad := &testPlugin{
		manifest: Manifest{ID: "bad", APIVersion: APIVersion},
		eventErr: errors.New("handler error"),
	}
	good := &testPlugin{manifest: Manifest{ID: "good", APIVersion: APIVersion}}
	m, err := NewManager(
		[]Factory{factory(bad), factory(good)},
		enabled("bad", "good"),
		Env{Log: testLog()},
	)
	if err != nil {
		t.Fatal(err)
	}

	event := events.New(events.PostCreated, events.SourceAPI, events.PostData{Post: &model.Post{Slug: "post"}})
	m.Emit(context.Background(), event)
	if len(good.events) != 1 || good.events[0].Type != events.PostCreated {
		t.Fatalf("good events = %+v", good.events)
	}

	panicPlugin := &testPlugin{
		manifest: Manifest{ID: "panic", APIVersion: APIVersion},
		panic:    true,
	}
	m, err = NewManager(
		[]Factory{factory(panicPlugin), factory(good)},
		enabled("panic", "good"),
		Env{Log: testLog()},
	)
	if err != nil {
		t.Fatal(err)
	}
	good.events = nil
	m.Emit(context.Background(), events.New(events.NoteCreated, events.SourceAdmin, events.NoteData{}))
	if len(good.events) != 1 {
		t.Fatal("panic in one plugin stopped dispatch to later plugins")
	}
}

func TestEnvDecodeSettings(t *testing.T) {
	env := Env{Settings: map[string]any{
		"url":    "https://example.com",
		"events": []any{"post.created", "comment.created"},
	}}
	var out struct {
		URL    string   `yaml:"url"`
		Events []string `yaml:"events"`
	}
	if err := env.DecodeSettings(&out); err != nil {
		t.Fatal(err)
	}
	if out.URL != "https://example.com" || len(out.Events) != 2 {
		t.Fatalf("decoded = %+v", out)
	}
}
