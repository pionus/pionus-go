package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndEnvOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(`
server:
  addr: ":9000"
  access_log: "logs/access.log"
timezone: "UTC"
database:
  path: "./test.db"
upload:
  dir: "./uploads"
  max_size_mb: 3
session:
  cookie_name: "sid"
  ttl_hours: 2
  secure: false
`), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PIONUS_ADDR", "127.0.0.1:9999")
	t.Setenv("PIONUS_SECRET", "secret")
	t.Setenv("PIONUS_ACCESS_LOG", "-")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Addr != "127.0.0.1:9999" || cfg.Security.Secret != "secret" || cfg.Server.AccessLog != "-" {
		t.Fatalf("config = %+v", cfg)
	}
	if cfg.Upload.MaxSizeMB != 3 || cfg.Session.TTLHours != 2 || cfg.Timezone != "UTC" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadPlugins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(`
plugins:
  enabled:
    - webhook
  config:
    webhook:
      url: "https://example.com/hook"
      events: ["post.created"]
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Plugins.Enabled) != 1 || cfg.Plugins.Enabled[0] != "webhook" {
		t.Fatalf("enabled = %+v", cfg.Plugins.Enabled)
	}
	if got := cfg.Plugins.Config["webhook"]["url"]; got != "https://example.com/hook" {
		t.Fatalf("url = %v", got)
	}

	t.Setenv("PIONUS_PLUGINS_ENABLED", "telegram, webhook")
	cfg, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"telegram", "webhook"}
	if len(cfg.Plugins.Enabled) != len(want) {
		t.Fatalf("enabled = %+v", cfg.Plugins.Enabled)
	}
	for i := range want {
		if cfg.Plugins.Enabled[i] != want[i] {
			t.Fatalf("enabled = %+v", cfg.Plugins.Enabled)
		}
	}
}
