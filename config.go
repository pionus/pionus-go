package main

import (
	"fmt"
	"os"

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

type Config struct {
	Server    ServerConfig   `yaml:"server"`
	Theme     string         `yaml:"theme"`
	Database  DatabaseConfig `yaml:"database"`
	TLS       TLSConfig      `yaml:"tls"`
	AuthToken string         `yaml:"-"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{
		Server:   ServerConfig{Addr: ":8087"},
		Theme:    "default",
		Database: DatabaseConfig{Path: "./data/pionus.db"},
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
	if v := os.Getenv("PIONUS_DB_PATH"); v != "" {
		cfg.Database.Path = v
	}

	return cfg, nil
}
