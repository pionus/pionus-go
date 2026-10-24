// Package config loads pionus configuration from YAML and environment.
package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type TLSConfig struct {
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

type UploadConfig struct {
	Dir       string `yaml:"dir"`
	MaxSizeMB int    `yaml:"max_size_mb"`
}

type SessionConfig struct {
	CookieName string `yaml:"cookie_name"`
	TTLHours   int    `yaml:"ttl_hours"`
	Secure     bool   `yaml:"secure"`
}

type SecurityConfig struct {
	Secret string `yaml:"secret"`
}

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Security SecurityConfig `yaml:"security"`
	Theme    string         `yaml:"theme"`
	// Timezone is the IANA zone used for timestamps and display.
	Timezone  string         `yaml:"timezone"`
	Database  DatabaseConfig `yaml:"database"`
	TLS       TLSConfig      `yaml:"tls"`
	Upload    UploadConfig   `yaml:"upload"`
	Session   SessionConfig  `yaml:"session"`
	AuthToken string         `yaml:"-"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{
		Server:   ServerConfig{Addr: ":8087"},
		Timezone: "Asia/Shanghai",
		Database: DatabaseConfig{Path: "./data/pionus.db"},
		Upload:   UploadConfig{Dir: "./uploads", MaxSizeMB: 10},
		Session:  SessionConfig{CookieName: "pionus_session", TTLHours: 24 * 7},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if v := os.Getenv("PIONUS_AUTH_TOKEN"); v != "" {
		cfg.AuthToken = v
	}
	if v := os.Getenv("PIONUS_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := os.Getenv("PIONUS_SECRET"); v != "" {
		cfg.Security.Secret = v
	}
	if v := os.Getenv("PIONUS_TIMEZONE"); v != "" {
		cfg.Timezone = v
	}
	if v := os.Getenv("PIONUS_DB_PATH"); v != "" {
		cfg.Database.Path = v
	}
	if v := os.Getenv("PIONUS_UPLOAD_DIR"); v != "" {
		cfg.Upload.Dir = v
	}
	if v := os.Getenv("PIONUS_UPLOAD_MAX_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Upload.MaxSizeMB = n
		}
	}
	if v := os.Getenv("PIONUS_SESSION_COOKIE"); v != "" {
		cfg.Session.CookieName = v
	}
	if v := os.Getenv("PIONUS_SESSION_TTL_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Session.TTLHours = n
		}
	}
	if v := os.Getenv("PIONUS_SESSION_SECURE"); v != "" {
		cfg.Session.Secure = v == "1" || v == "true" || v == "yes"
	}

	// Serving TLS from the app itself implies HTTPS, so mark cookies secure
	// unless the operator explicitly disabled it.
	if cfg.TLS.Cert != "" {
		cfg.Session.Secure = true
	}
	if cfg.Session.TTLHours <= 0 {
		cfg.Session.TTLHours = 24 * 7
	}
	if cfg.Upload.MaxSizeMB <= 0 {
		cfg.Upload.MaxSizeMB = 10
	}

	return cfg, nil
}
