package config

import (
	"os"
	"path/filepath"
	"rpl/pkg/error/localize"
	"testing"
)

func TestLoadDefaultOrDefaultDoesNotCreateConfig(t *testing.T) {
	tempDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg, err := LoadDefaultOrDefault()
	if err != nil {
		t.Fatalf("load default or default: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected config")
	}

	if _, err := os.Stat(filepath.Join(tempDir, ".rpl", "config.xml")); !os.IsNotExist(err) {
		t.Fatalf("expected config file to be absent, got err=%v", err)
	}
}

func TestLoadForBasePrefersNearestProjectConfig(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "project")
	configPath := filepath.Join(projectDir, ".rpl", "config.xml")
	sourcePath := filepath.Join(projectDir, "src", "main.rpl")

	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}

	cfg := Default()
	cfg.Localization.Language = localize.LangRU
	if err := Save(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, gotPath, exists, err := LoadForBase(sourcePath)
	if err != nil {
		t.Fatalf("load for base: %v", err)
	}
	if !exists {
		t.Fatal("expected project config to exist")
	}
	if gotPath != configPath {
		t.Fatalf("config path = %q, want %q", gotPath, configPath)
	}
	if loaded == nil {
		t.Fatal("expected config")
	}
	if loaded.Localization.Language != localize.LangRU {
		t.Fatalf("language = %q, want %q", loaded.Localization.Language, localize.LangRU)
	}
}
