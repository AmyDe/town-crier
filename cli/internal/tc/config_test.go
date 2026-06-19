package tc

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func strptr(s string) *string { return &s }

func TestLoadConfig_LoadsFromFile(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `{"url":"https://api.example.com","apiKey":"sk-test"}`)

	cfg, err := LoadConfig(path, nil, nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.URL != "https://api.example.com" || cfg.APIKey != "sk-test" {
		t.Fatalf("cfg = %+v, want url=https://api.example.com apiKey=sk-test", cfg)
	}
}

func TestLoadConfig_CliArgsOverrideFile(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `{"url":"https://api.example.com","apiKey":"sk-file"}`)

	cfg, err := LoadConfig(path, strptr("http://localhost:8080"), strptr("sk-override"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.URL != "http://localhost:8080" || cfg.APIKey != "sk-override" {
		t.Fatalf("cfg = %+v, want overridden values", cfg)
	}
}

func TestLoadConfig_CliArgsOnlyWhenNoFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "missing", "config.json")

	cfg, err := LoadConfig(path, strptr("http://localhost:8080"), strptr("sk-arg"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.URL != "http://localhost:8080" || cfg.APIKey != "sk-arg" {
		t.Fatalf("cfg = %+v, want CLI-only values", cfg)
	}
}

func TestLoadConfig_ErrorsWhenURLMissing(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "missing", "config.json")

	_, err := LoadConfig(path, nil, strptr("sk-test"))
	if !errors.Is(err, ErrURLNotConfigured) {
		t.Fatalf("err = %v, want ErrURLNotConfigured", err)
	}
}

func TestLoadConfig_ErrorsWhenAPIKeyMissing(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "missing", "config.json")

	_, err := LoadConfig(path, strptr("http://localhost:8080"), nil)
	if !errors.Is(err, ErrAPIKeyNotConfigured) {
		t.Fatalf("err = %v, want ErrAPIKeyNotConfigured", err)
	}
}

func TestLoadConfig_PartialOverrideOnlyURL(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `{"url":"https://api.example.com","apiKey":"sk-file"}`)

	cfg, err := LoadConfig(path, strptr("http://localhost:8080"), nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.URL != "http://localhost:8080" || cfg.APIKey != "sk-file" {
		t.Fatalf("cfg = %+v, want url overridden, apiKey from file", cfg)
	}
}
