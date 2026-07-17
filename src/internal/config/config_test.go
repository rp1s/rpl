package config

import (
	"os"
	"path/filepath"
	"rpl/pkg/error/localize"
	"testing"
)

func TestLoadDefaultOrDefaultDoesNotCreateConfig(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(GlobalHomeEnv, filepath.Join(tempDir, "global"))
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
	t.Setenv(GlobalHomeEnv, filepath.Join(t.TempDir(), "global"))
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

func TestGlobalConfigProvidesDefaultsForProjects(t *testing.T) {
	globalDir := filepath.Join(t.TempDir(), "portable-rpl")
	t.Setenv(GlobalHomeEnv, globalDir)

	global := GlobalDefault()
	global.Localization.Language = localize.LangRU
	useColor := false
	global.Localization.UseColor = &useColor
	globalPath, err := GlobalPath()
	if err != nil {
		t.Fatalf("global path: %v", err)
	}
	if err := Save(globalPath, global); err != nil {
		t.Fatalf("save global config: %v", err)
	}

	projectDir := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	loaded, _, exists, err := LoadForBase(projectDir)
	if err != nil {
		t.Fatalf("load effective config: %v", err)
	}
	if exists {
		t.Fatal("did not expect a project config")
	}
	if loaded.Localization.Language != localize.LangRU {
		t.Fatalf("language = %q, want %q", loaded.Localization.Language, localize.LangRU)
	}
	if loaded.UseColor() {
		t.Fatal("expected global color=false")
	}
	if got, want := loaded.Runtimes.Directory, "attrs"; got != want {
		t.Fatalf("runtime directory = %q, want %q", got, want)
	}
}

func TestGlobalAttrsPathResolvesRelativeToGlobalDir(t *testing.T) {
	globalDir := filepath.Join(t.TempDir(), "config")
	t.Setenv(GlobalHomeEnv, globalDir)

	cfg := GlobalDefault()
	cfg.Runtimes.Directory = "shared/plugins"
	path, err := GlobalPath()
	if err != nil {
		t.Fatalf("global path: %v", err)
	}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("save global config: %v", err)
	}

	got, err := GlobalAttrsPath()
	if err != nil {
		t.Fatalf("global attrs path: %v", err)
	}
	want := filepath.Join(globalDir, "shared", "plugins")
	if got != want {
		t.Fatalf("attrs path = %q, want %q", got, want)
	}
}
