// Package config loads the collector configuration from YAML and environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const EnvPrefix = "RFC6035_2OTEL_"

// Config contains all runtime configuration. Values are loaded in this order:
// defaults, YAML, then RFC6035_2OTEL_ environment variables. A double underscore
// in an environment variable represents YAML nesting; for example,
// RFC6035_2OTEL_OTLP__ENDPOINT overrides otlp.endpoint.
type Config struct {
	Listen       ListenConfig  `yaml:"listen"`
	OTLP         OTLPConfig    `yaml:"otlp"`
	DedupeWindow time.Duration `yaml:"dedupe_window"`
	Log          LogConfig     `yaml:"log"`
	Service      ServiceConfig `yaml:"service"`
}

type ListenConfig struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

type OTLPConfig struct {
	Endpoint string            `yaml:"endpoint"`
	Headers  map[string]string `yaml:"headers"`
	Protocol string            `yaml:"protocol"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

type ServiceConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

func Default() Config {
	return Config{
		Listen:       ListenConfig{Address: "0.0.0.0", Port: 5060},
		OTLP:         OTLPConfig{Headers: map[string]string{}, Protocol: "http"},
		DedupeWindow: 32 * time.Second,
		Log:          LogConfig{Level: "info"},
		Service:      ServiceConfig{Name: "rfc6035-2otel", Version: "dev"},
	}
}

// Load reads path when non-empty, applies environment overrides, and validates the
// resulting configuration.
func Load(path string) (*Config, error) {
	cfg := Default()
	if path != "" {
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("config file %q: %w", path, err)
		}
		decoder := yaml.NewDecoder(strings.NewReader(string(contents)))
		decoder.KnownFields(true)
		if err := decoder.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("config file %q: %w", path, err)
		}
		if cfg.OTLP.Headers == nil {
			cfg.OTLP.Headers = map[string]string{}
		}
	}
	if err := applyEnvironment(&cfg, os.Environ()); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyEnvironment(cfg *Config, environ []string) error {
	for _, entry := range environ {
		key, value, found := strings.Cut(entry, "=")
		if !found || !strings.HasPrefix(key, EnvPrefix) {
			continue
		}
		name := strings.ToLower(strings.TrimPrefix(key, EnvPrefix))
		name = strings.ReplaceAll(name, "__", ".")
		if err := applyEnvironmentValue(cfg, name, value); err != nil {
			return err
		}
	}
	return nil
}

func applyEnvironmentValue(cfg *Config, key, value string) error {
	parseInt := func() (int, error) {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return 0, fmt.Errorf("%s must be an integer: %w", key, err)
		}
		return parsed, nil
	}
	switch key {
	case "listen.address":
		cfg.Listen.Address = value
	case "listen.port":
		parsed, err := parseInt()
		if err != nil {
			return err
		}
		cfg.Listen.Port = parsed
	case "otlp.endpoint":
		cfg.OTLP.Endpoint = value
	case "otlp.protocol":
		cfg.OTLP.Protocol = value
	case "dedupe_window":
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("dedupe_window must be a duration: %w", err)
		}
		cfg.DedupeWindow = parsed
	case "log.level":
		cfg.Log.Level = value
	case "service.name":
		cfg.Service.Name = value
	case "service.version":
		cfg.Service.Version = value
	default:
		if strings.HasPrefix(key, "otlp.headers.") {
			header := strings.TrimPrefix(key, "otlp.headers.")
			if header == "" {
				return fmt.Errorf("otlp.headers key is empty")
			}
			if cfg.OTLP.Headers == nil {
				cfg.OTLP.Headers = map[string]string{}
			}
			cfg.OTLP.Headers[header] = value
			return nil
		}
		return fmt.Errorf("unknown environment configuration key %q", key)
	}
	return nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Listen.Address) == "" {
		return fmt.Errorf("listen.address is required")
	}
	if c.Listen.Port < 1 || c.Listen.Port > 65535 {
		return fmt.Errorf("listen.port must be between 1 and 65535")
	}
	if strings.TrimSpace(c.OTLP.Endpoint) == "" {
		return fmt.Errorf("otlp.endpoint is required")
	}
	if c.OTLP.Protocol != "http" && c.OTLP.Protocol != "grpc" {
		return fmt.Errorf("otlp.protocol must be http or grpc")
	}
	if c.DedupeWindow <= 0 {
		return fmt.Errorf("dedupe_window must be positive")
	}
	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("log.level must be debug, info, warn, or error")
	}
	if strings.TrimSpace(c.Service.Name) == "" {
		return fmt.Errorf("service.name is required")
	}
	if strings.TrimSpace(c.Service.Version) == "" {
		return fmt.Errorf("service.version is required")
	}
	return nil
}
