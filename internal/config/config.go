// Package config loads spectre's single JSON configuration file.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the top-level shape of spectre's JSON config file.
type Config struct {
	ListenAddr string    `json:"listen_addr"`
	DBPath     string    `json:"db_path"`
	LLM        LLMConfig `json:"llm"`
	LogLevel   string    `json:"log_level"`
}

// LLMConfig configures the OpenAI-compatible chat completions backend.
type LLMConfig struct {
	BaseURL            string `json:"base_url"`
	Model              string `json:"model"`
	APIKey             string `json:"api_key"`
	SystemPrompt       string `json:"system_prompt"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
	MaxHistoryMessages int    `json:"max_history_messages"`
}

const (
	defaultListenAddr         = "127.0.0.1:8787"
	defaultLogLevel           = "info"
	defaultTimeoutSeconds     = 120
	defaultMaxHistoryMessages = 40
)

// DefaultPath returns the default config file location:
// $XDG_CONFIG_HOME/spectre/config.json, falling back to
// ~/.config/spectre/config.json.
func DefaultPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "spectre", "config.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "spectre", "config.json"), nil
}

// Load reads and validates the config file at path, applying defaults for
// optional fields. If path is empty, DefaultPath is used.
func Load(path string) (*Config, error) {
	if path == "" {
		p, err := DefaultPath()
		if err != nil {
			return nil, err
		}
		path = p
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	applyDefaults(&cfg)

	dbPath, err := expandHome(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("expand db_path: %w", err)
	}
	cfg.DBPath = dbPath

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}

	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = defaultLogLevel
	}
	if cfg.LLM.TimeoutSeconds == 0 {
		cfg.LLM.TimeoutSeconds = defaultTimeoutSeconds
	}
	if cfg.LLM.MaxHistoryMessages == 0 {
		cfg.LLM.MaxHistoryMessages = defaultMaxHistoryMessages
	}
}

func validate(cfg *Config) error {
	var missing []string
	if cfg.DBPath == "" {
		missing = append(missing, "db_path")
	}
	if cfg.LLM.BaseURL == "" {
		missing = append(missing, "llm.base_url")
	}
	if cfg.LLM.Model == "" {
		missing = append(missing, "llm.model")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required field(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

// expandHome expands a leading "~" in path to the current user's home
// directory. Paths without a leading "~" are returned unchanged.
func expandHome(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:]), nil
	}
	// "~otheruser/..." is not supported; leave as-is.
	return path, nil
}
