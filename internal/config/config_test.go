package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadYAMLAndEnvironmentPrecedence(t *testing.T) {
	t.Setenv("RFC6035_2OTEL_OTLP__ENDPOINT", "https://environment.example")
	t.Setenv("RFC6035_2OTEL_OTLP__HEADERS__AUTHORIZATION", "Bearer token")
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := []byte("listen:\n  address: 127.0.0.1\n  port: 5514\notlp:\n  endpoint: https://file.example\n  headers:\n    X-Tenant: tenant\n  protocol: grpc\ndedupe_window: 45s\nlog:\n  level: debug\nservice:\n  name: test-collector\n  version: 1.2.3\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OTLP.Endpoint != "https://environment.example" {
		t.Fatalf("endpoint = %q", cfg.OTLP.Endpoint)
	}
	if cfg.OTLP.Headers["authorization"] != "Bearer token" {
		t.Fatalf("environment header = %q", cfg.OTLP.Headers["authorization"])
	}
	if cfg.OTLP.Headers["X-Tenant"] != "tenant" {
		t.Fatalf("YAML header = %q", cfg.OTLP.Headers["X-Tenant"])
	}
	if cfg.Listen.Port != 5514 || cfg.DedupeWindow != 45*time.Second || cfg.Service.Name != "test-collector" {
		t.Fatalf("loaded config = %#v", cfg)
	}
}

func TestLoadInvalidConfigurationNamesKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("otlp:\n  endpoint: https://example.invalid\nlisten:\n  port: 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "listen.port") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadRejectsUnknownYAMLKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("otlp:\n  endpoint: https://example.invalid\n  unexpected: value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("error = %v", err)
	}
}
