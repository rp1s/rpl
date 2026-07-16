package language

import (
	"os"
	"path/filepath"
	"rpl/internal/config"
	"rpl/pkg/error/localize"
	"testing"
)

func TestSetDoesNotCreateConfigOutsideProject(t *testing.T) {
	tempDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	originalLang := localize.Language
	defer func() {
		localize.SetLanguage(originalLang)
		_ = os.Chdir(originalWD)
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	service := New(config.Default())
	if _, err := service.Set(localize.LangRU); err != nil {
		t.Fatalf("set language: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tempDir, ".rpl", "config.xml")); !os.IsNotExist(err) {
		t.Fatalf("expected config file to be absent, got err=%v", err)
	}
}

func TestSetAtUpdatesNearestProjectConfig(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "project")
	configPath := filepath.Join(projectDir, ".rpl", "config.xml")
	sourcePath := filepath.Join(projectDir, "src", "main.rpl")

	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}

	cfg := config.Default()
	cfg.Localization.Language = localize.LangEN
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	originalLang := localize.Language
	originalUseColor := localize.UseColor
	defer func() {
		localize.SetLanguage(originalLang)
		localize.UseColor = originalUseColor
	}()

	service := New(config.Default())
	if _, err := service.SetAt(sourcePath, localize.LangRU); err != nil {
		t.Fatalf("set language at path: %v", err)
	}

	loaded, err := config.LoadOrDefault(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if loaded.Localization.Language != localize.LangRU {
		t.Fatalf("language = %q, want %q", loaded.Localization.Language, localize.LangRU)
	}
}
