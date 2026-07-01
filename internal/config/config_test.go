package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoad_HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "config.json", `{
		"listen_addr": "127.0.0.1:9999",
		"db_path": "`+dir+`/spectre.db",
		"llm": {
			"base_url": "http://llm.example.internal",
			"model": "test-model"
		}
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.ListenAddr != "127.0.0.1:9999" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, "127.0.0.1:9999")
	}
	if cfg.LLM.BaseURL != "http://llm.example.internal" {
		t.Errorf("LLM.BaseURL = %q", cfg.LLM.BaseURL)
	}
	if cfg.LLM.Model != "test-model" {
		t.Errorf("LLM.Model = %q", cfg.LLM.Model)
	}
	// defaults
	if cfg.LogLevel != defaultLogLevel {
		t.Errorf("LogLevel = %q, want default %q", cfg.LogLevel, defaultLogLevel)
	}
	if cfg.LLM.TimeoutSeconds != defaultTimeoutSeconds {
		t.Errorf("LLM.TimeoutSeconds = %d, want default %d", cfg.LLM.TimeoutSeconds, defaultTimeoutSeconds)
	}
	if cfg.LLM.MaxHistoryMessages != defaultMaxHistoryMessages {
		t.Errorf("LLM.MaxHistoryMessages = %d, want default %d", cfg.LLM.MaxHistoryMessages, defaultMaxHistoryMessages)
	}
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "config.json", `{"listen_addr": "127.0.0.1:8787"}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load returned nil error, want error for missing required fields")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("Load returned nil error, want error for missing file")
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	got, err := expandHome("~/.local/share/spectre/spectre.db")
	if err != nil {
		t.Fatalf("expandHome returned error: %v", err)
	}
	want := filepath.Join(home, ".local/share/spectre/spectre.db")
	if got != want {
		t.Errorf("expandHome = %q, want %q", got, want)
	}

	got, err = expandHome("/absolute/path/spectre.db")
	if err != nil {
		t.Fatalf("expandHome returned error: %v", err)
	}
	if got != "/absolute/path/spectre.db" {
		t.Errorf("expandHome should leave absolute paths unchanged, got %q", got)
	}
}
