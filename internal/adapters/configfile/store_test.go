package configfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOrCreateWritesDefaultConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := New(path)

	config, err := store.LoadOrCreate()
	if err != nil {
		t.Fatalf("load or create config: %v", err)
	}
	if config != Default() {
		t.Fatalf("expected default config, got %#v", config)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(strings.ToLower(string(contents)), "api_key") {
		t.Fatal("config must not contain an API key field")
	}
}

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := New(path)
	expected := Config{
		Version:         CurrentVersion,
		DefaultLanguage: "en",
	}

	if err := store.Save(expected); err != nil {
		t.Fatalf("save config: %v", err)
	}
	actual, err := store.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if actual != expected {
		t.Fatalf("expected %#v, got %#v", expected, actual)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(
		path,
		[]byte(`{"version":1,"defaultLanguage":"zh-CN","apiKey":"secret"}`),
		0o600,
	); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	_, err := New(path).Load()
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestLoadRejectsUnsupportedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(
		path,
		[]byte(`{"version":2,"defaultLanguage":"zh-CN"}`),
		0o600,
	); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	_, err := New(path).Load()
	if err == nil || !strings.Contains(err.Error(), "unsupported config version") {
		t.Fatalf("expected version error, got %v", err)
	}
}

func TestLoadReturnsNotExist(t *testing.T) {
	_, err := New(filepath.Join(t.TempDir(), "missing.json")).Load()
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected not exist error, got %v", err)
	}
}
