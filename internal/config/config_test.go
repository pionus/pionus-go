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
