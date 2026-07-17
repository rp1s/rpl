package config

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"rpl/internal/fsutil"
	"rpl/pkg/error/localize"
	"strings"
)

const (
	DefaultPath         = ".rpl/config.xml"
	defaultRuntimesPath = ".rpl/attrs"
	globalRuntimesPath  = "attrs"
	GlobalHomeEnv       = "RPL_CONFIG_HOME"
)

// BundledRuntimesPathFromExecutable resolves the sidecar attrs folder near the
// installed binary so packaged distributions can ship built-in runtimes.
func BundledRuntimesPathFromExecutable() string {
	executable, err := os.Executable()
	if err != nil || strings.TrimSpace(executable) == "" {
		return ""
	}

	binDir := filepath.Dir(executable)
	if strings.TrimSpace(binDir) == "" {
		return ""
	}

	return filepath.Join(binDir, ".rpl", "attrs")
}

// BundledSDKPathFromExecutable resolves the Go SDK module shipped with release
// builds. Attr scaffolds use it through a local replace directive, so a plugin
// can be built without cloning the RPL repository.
func BundledSDKPathFromExecutable() string {
	executable, err := os.Executable()
	if err != nil || strings.TrimSpace(executable) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(executable), ".rpl", "sdk")
}

type Config struct {
	XMLName      xml.Name           `xml:"config"`
	Runtimes     RuntimesConfig     `xml:"runtimes"`
	Localization LocalizationConfig `xml:"localization"`
	AuthorData   *AuthorData        `xml:"author_data"`
}

type RuntimesConfig struct {
	Directory string `xml:"directory"`
}

type LocalizationConfig struct {
	Language string `xml:"language"`
	UseColor *bool  `xml:"use_color"`
}

type AuthorData struct {
	AuthorName *string `xml:"author_name"`
}

func Default() *Config {
	useColor := true

	return &Config{
		Runtimes: RuntimesConfig{
			Directory: defaultRuntimesPath,
		},
		Localization: LocalizationConfig{
			Language: localize.LangEN,
			UseColor: &useColor,
		},
	}
}

// GlobalDefault returns defaults for the user-wide configuration. Its attrs
// directory is resolved relative to the global RPL config directory rather
// than to an individual project.
func GlobalDefault() *Config {
	cfg := Default()
	cfg.Runtimes.Directory = globalRuntimesPath
	return cfg
}

// GlobalDir returns the per-user RPL configuration directory. RPL_CONFIG_HOME
// makes the location explicit for portable installations and test runners.
func GlobalDir() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(GlobalHomeEnv)); configured != "" {
		absolute, err := filepath.Abs(configured)
		if err != nil {
			return "", fmt.Errorf(localize.Text("определение глобальной папки RPL %q: %w", "resolve global RPL directory %q: %w"), configured, err)
		}
		return absolute, nil
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf(localize.Text("определение пользовательской папки конфигурации: %w", "resolve user config directory: %w"), err)
	}
	return filepath.Join(base, "rpl"), nil
}

func GlobalPath() (string, error) {
	dir, err := GlobalDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.xml"), nil
}

func GlobalAttrsPath() (string, error) {
	dir, err := GlobalDir()
	if err != nil {
		return "", err
	}
	cfg, err := LoadGlobalOrDefault()
	if err != nil {
		return "", err
	}

	path := filepath.FromSlash(strings.TrimSpace(cfg.Runtimes.Directory))
	if path == "" {
		path = globalRuntimesPath
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	return filepath.Clean(path), nil
}

func LoadOrCreateDefault() (*Config, error) {
	return LoadOrCreate(DefaultPath)
}

func LoadDefaultOrDefault() (*Config, error) {
	cfg, _, _, err := LoadForBase("")
	return cfg, err
}

func LoadGlobalOrDefault() (*Config, error) {
	path, err := GlobalPath()
	if err != nil {
		return nil, err
	}
	return loadOrDefault(path, GlobalDefault())
}

func LoadOrCreateGlobal() (*Config, error) {
	path, err := GlobalPath()
	if err != nil {
		return nil, err
	}
	return loadOrCreate(path, GlobalDefault())
}

// LoadForBase searches for the nearest project config relative to a file or
// directory path. When no project config exists it falls back to in-memory defaults.
func LoadForBase(basePath string) (*Config, string, bool, error) {
	configPath, exists, err := PathForBase(basePath)
	if err != nil {
		return nil, "", false, err
	}
	base, err := LoadGlobalOrDefault()
	if err != nil {
		return nil, "", false, err
	}
	if !exists {
		return base, configPath, false, nil
	}

	cfg, err := loadOrDefault(configPath, base)
	if err != nil {
		return nil, "", false, err
	}

	return cfg, configPath, true, nil
}

// LoadOrDefault overlays the on-disk XML onto the in-memory defaults so new
// config fields keep sensible values even in older config files.
func LoadOrDefault(path string) (*Config, error) {
	return loadOrDefault(path, Default())
}

func loadOrDefault(path string, defaults *Config) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return clone(defaults), nil
		}

		return nil, fmt.Errorf(localize.Text("чтение конфигурации %q: %w", "read config %q: %w"), path, err)
	}

	cfg := clone(defaults)
	if err := xml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf(localize.Text("разбор конфигурации %q: %w", "decode config %q: %w"), path, err)
	}

	cfg.Normalize()
	return cfg, nil
}

// LoadOrCreate behaves like LoadOrDefault but also persists a normalized file
// back to disk, which is useful for bootstrapping new projects.
func LoadOrCreate(path string) (*Config, error) {
	return loadOrCreate(path, Default())
}

func loadOrCreate(path string, defaults *Config) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath
	}

	if err := ensureDir(path); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := clone(defaults)
			if err := Save(path, cfg); err != nil {
				return nil, err
			}

			return cfg, nil
		}

		return nil, fmt.Errorf(localize.Text("чтение конфигурации %q: %w", "read config %q: %w"), path, err)
	}

	cfg := clone(defaults)
	if err := xml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf(localize.Text("разбор конфигурации %q: %w", "decode config %q: %w"), path, err)
	}

	cfg.Normalize()
	if err := Save(path, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func clone(cfg *Config) *Config {
	if cfg == nil {
		return Default()
	}

	copy := *cfg
	if cfg.Localization.UseColor != nil {
		value := *cfg.Localization.UseColor
		copy.Localization.UseColor = &value
	}
	if cfg.AuthorData != nil {
		authorData := *cfg.AuthorData
		if cfg.AuthorData.AuthorName != nil {
			value := *cfg.AuthorData.AuthorName
			authorData.AuthorName = &value
		}
		copy.AuthorData = &authorData
	}
	return &copy
}

func SaveDefault(cfg *Config) error {
	return Save(DefaultPath, cfg)
}

// PathForBase returns the nearest config path for a file or directory and
// reports whether that config already exists on disk.
func PathForBase(basePath string) (string, bool, error) {
	trimmed := strings.TrimSpace(basePath)
	if trimmed == "" {
		return DefaultPath, fileExists(DefaultPath), nil
	}

	startDir, err := baseStartDir(trimmed)
	if err != nil {
		return "", false, err
	}

	current := startDir
	for {
		candidate := filepath.Join(current, filepath.FromSlash(DefaultPath))
		if fileExists(candidate) {
			return candidate, true, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Join(startDir, filepath.FromSlash(DefaultPath)), false, nil
		}
		current = parent
	}
}

func DefaultExists() (bool, error) {
	_, err := os.Stat(DefaultPath)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, fmt.Errorf(localize.Text("чтение конфигурации %q: %w", "read config %q: %w"), DefaultPath, err)
}

// Save normalizes the config before marshaling so the written XML stays
// complete and stable even when callers pass partially filled structs.
func Save(path string, cfg *Config) error {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath
	}

	if cfg == nil {
		cfg = Default()
	}

	cfg.Normalize()

	if err := ensureDir(path); err != nil {
		return err
	}

	body, err := xml.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf(localize.Text("кодирование конфигурации %q: %w", "encode config %q: %w"), path, err)
	}

	body = append([]byte(xml.Header), body...)
	body = append(body, '\n')

	if err := fsutil.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf(localize.Text("запись конфигурации %q: %w", "write config %q: %w"), path, err)
	}

	return nil
}

// Normalize fills zero-value config sections with runtime defaults after load
// or before save, which keeps the rest of the code free from nil checks.
func (cfg *Config) Normalize() {
	if cfg == nil {
		return
	}

	if strings.TrimSpace(cfg.Runtimes.Directory) == "" {
		cfg.Runtimes.Directory = defaultRuntimesPath
	}

	if strings.TrimSpace(cfg.Localization.Language) == "" {
		cfg.Localization.Language = localize.LangEN
	} else {
		cfg.Localization.Language = localize.NormalizeLanguage(cfg.Localization.Language)
	}

	if cfg.Localization.UseColor == nil {
		useColor := true
		cfg.Localization.UseColor = &useColor
	}
}

func (cfg *Config) UseColor() bool {
	if cfg == nil || cfg.Localization.UseColor == nil {
		return true
	}

	return *cfg.Localization.UseColor
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}

	return os.MkdirAll(dir, 0o755)
}

func baseStartDir(basePath string) (string, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(basePath))
	if err != nil {
		return "", err
	}

	info, statErr := os.Stat(absolute)
	switch {
	case statErr == nil:
		if info.IsDir() {
			return absolute, nil
		}
		return filepath.Dir(absolute), nil
	case os.IsNotExist(statErr):
		if filepath.Ext(absolute) != "" {
			return filepath.Dir(absolute), nil
		}
		return absolute, nil
	default:
		return "", statErr
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
