package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// searchPaths returns the ordered list of config file locations to try.
func searchPaths() []string {
	paths := []string{
		"/etc/herald/herald.yaml",
	}

	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "herald", "herald.yaml"))
	}

	paths = append(paths, "herald.yaml")

	if envPath := os.Getenv("HERALD_CONFIG"); envPath != "" {
		paths = append(paths, envPath)
	}

	return paths
}

// Load reads configuration from YAML files and environment variables.
// Files are loaded in order (each overrides the previous):
// /etc/herald/herald.yaml < ~/.config/herald/herald.yaml < ./herald.yaml < $HERALD_CONFIG
func Load() (*Config, error) {
	cfg := Defaults()

	for _, path := range searchPaths() {
		if err := loadFile(cfg, path); err != nil {
			return nil, fmt.Errorf("loading config %s: %w", path, err)
		}
	}

	applyEnvOverrides(cfg)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// LoadFromFile reads configuration from a specific file path.
func LoadFromFile(path string) (*Config, error) {
	cfg := Defaults()

	if err := loadFile(cfg, path); err != nil {
		return nil, fmt.Errorf("loading config %s: %w", path, err)
	}

	applyEnvOverrides(cfg)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the configuration.
// Environment variables have higher priority than YAML config values.
func applyEnvOverrides(cfg *Config) {
	if token := os.Getenv("HERALD_NGROK_AUTHTOKEN"); token != "" {
		cfg.Tunnel.AuthToken = token
	}
}

func loadFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from trusted config search paths
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	slog.Debug("loading config file", "path", path)

	expanded := os.ExpandEnv(string(data))

	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}

	return nil
}

// ExpandHome replaces a leading ~ with the user's home directory.
func ExpandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

func validate(cfg *Config) error {
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", cfg.Server.Port)
	}

	if cfg.Server.Host == "0.0.0.0" {
		return fmt.Errorf("server.host must not be 0.0.0.0 — Herald must listen on localhost only (use Traefik for external access)")
	}

	if cfg.Execution.MaxConcurrent < 1 {
		return fmt.Errorf("execution.max_concurrent must be at least 1")
	}

	cfg.Database.Path = ExpandHome(cfg.Database.Path)
	cfg.Execution.WorkDir = ExpandHome(cfg.Execution.WorkDir)

	return nil
}
