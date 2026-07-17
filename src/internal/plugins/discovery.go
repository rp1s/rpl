package plugins

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"rpl/internal/config"
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"strings"
)

type configuredDiscoveryContext struct {
	cfg         *config.Config
	projectRoot string
}

func EnsureAvailable(name string, author string) error {
	return EnsureAvailableAt("", name, author)
}

func EnsureAvailableAt(basePath string, name string, author string) error {
	_, err := FindConfiguredAt(basePath, name, author)
	return err
}

func ListConfigured() ([]Binary, error) {
	return ListConfiguredAt("")
}

func ListConfiguredAt(basePath string) ([]Binary, error) {
	dirs, err := configuredSearchPathsForBase(basePath)
	if err != nil {
		return nil, err
	}

	return ListFromSearchPaths(dirs...)
}

func FindConfigured(name string, author string) (*Binary, error) {
	return FindConfiguredAt("", name, author)
}

func FindConfiguredAt(basePath string, name string, author string) (*Binary, error) {
	dirs, err := configuredSearchPathsForBase(basePath)
	if err != nil {
		return nil, err
	}

	return FindInSearchPaths(name, author, dirs...)
}

// configuredSearchPathsForBase combines the primary project's attr locations
// with a guarded working-directory fallback, deduplicating everything on the way.
func configuredSearchPathsForBase(basePath string) ([]string, error) {
	ctx, err := loadConfiguredDiscoveryContext(basePath)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	items := make([]string, 0, 6)
	appendPaths := func(dirs []string) {
		for _, dir := range dirs {
			dir = strings.TrimSpace(dir)
			if dir == "" {
				continue
			}
			if _, ok := seen[dir]; ok {
				continue
			}
			seen[dir] = struct{}{}
			items = append(items, dir)
		}
	}

	appendPaths(configuredSearchPaths(ctx.cfg.Runtimes.Directory, ctx.projectRoot))

	if shouldUseWorkingDirFallback(basePath, ctx.projectRoot) {
		fallbackCtx, err := loadConfiguredDiscoveryContext("")
		if err == nil && allowWorkingDirFallback(ctx.projectRoot, fallbackCtx.projectRoot) {
			appendPaths(configuredSearchPaths(fallbackCtx.cfg.Runtimes.Directory, fallbackCtx.projectRoot))
		}
	}

	// User-wide attrs are shared by every project. They come after project
	// locations so a repository can pin or override an attr locally, but before
	// bundled attrs so users can upgrade an installed generator independently.
	globalRoot, globalErr := config.GlobalDir()
	globalCfg, globalCfgErr := config.LoadGlobalOrDefault()
	if globalErr != nil {
		return nil, globalErr
	}
	if globalCfgErr != nil {
		return nil, globalCfgErr
	}
	appendPaths(configuredGlobalSearchPaths(globalCfg.Runtimes.Directory, globalRoot))

	// Installed releases keep built-in attrs next to the executable. Keep this
	// fallback last so project-local and source-tree attrs can intentionally
	// shadow the bundled version during development.
	appendPaths([]string{config.BundledRuntimesPathFromExecutable()})

	return items, nil
}

// loadConfiguredDiscoveryContext resolves both the project root and the config
// visible from a given source path so discovery follows project-local settings.
func loadConfiguredDiscoveryContext(basePath string) (configuredDiscoveryContext, error) {
	projectRoot, err := discoverProjectRoot(basePath)
	if err != nil {
		return configuredDiscoveryContext{}, fmt.Errorf(localize.Text("определение пути проекта attrs: %w", "resolve attrs project path: %w"), err)
	}

	cfgPath := filepath.Join(projectRoot, filepath.FromSlash(config.DefaultPath))
	cfg, err := config.LoadOrDefault(cfgPath)
	if err != nil {
		return configuredDiscoveryContext{}, fmt.Errorf(localize.Text("загрузка конфигурации attrs: %w", "load attrs config: %w"), err)
	}

	return configuredDiscoveryContext{
		cfg:         cfg,
		projectRoot: projectRoot,
	}, nil
}

// configuredSearchPaths expands the configured runtime directory into all local
// aliases currently recognized by the discovery layer.
func configuredSearchPaths(primary string, projectRoot string) []string {
	items := make([]string, 0, 3)
	seen := make(map[string]struct{})

	add := func(path string) {
		path = resolveConfiguredPath(projectRoot, path)
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		items = append(items, path)
	}

	add(primary)
	add(alternateLocalAttrsPath(primary))
	for _, path := range sourceTreeAttrsPaths(projectRoot) {
		add(path)
	}

	return items
}

func configuredGlobalSearchPaths(primary string, globalRoot string) []string {
	items := make([]string, 0, 2)
	seen := make(map[string]struct{})
	for _, configuredPath := range []string{primary, alternateLocalAttrsPath(primary)} {
		path := resolveConfiguredPath(globalRoot, configuredPath)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		items = append(items, path)
	}
	return items
}

func resolveConfiguredPath(projectRoot string, configuredPath string) string {
	configuredPath = strings.TrimSpace(configuredPath)
	if configuredPath == "" {
		return ""
	}

	path := filepath.FromSlash(configuredPath)
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	if strings.TrimSpace(projectRoot) == "" {
		return filepath.Clean(path)
	}

	return filepath.Join(projectRoot, path)
}

func alternateLocalAttrsPath(primary string) string {
	primary = filepath.ToSlash(strings.TrimSpace(primary))
	if primary == "" {
		return ""
	}

	switch {
	case strings.HasSuffix(primary, "/runtimes"):
		return strings.TrimSuffix(primary, "/runtimes") + "/attrs"
	case strings.HasSuffix(primary, "/attrs"):
		return strings.TrimSuffix(primary, "/attrs") + "/runtimes"
	default:
		return ""
	}
}

// discoverProjectRoot climbs upward until it finds a project layout marker and
// otherwise falls back to the original start directory.
func discoverProjectRoot(basePath string) (string, error) {
	startDir, err := discoveryStartDir(basePath)
	if err != nil {
		return "", err
	}

	current := startDir
	for {
		if hasProjectAttrsLayout(current) {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return startDir, nil
		}
		current = parent
	}
}

// discoveryStartDir accepts either file paths or directories and converts them
// into the directory where discovery should begin scanning upward.
func discoveryStartDir(basePath string) (string, error) {
	trimmed := strings.TrimSpace(basePath)
	if trimmed == "" {
		return os.Getwd()
	}

	absolute, err := filepath.Abs(trimmed)
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

func hasProjectAttrsLayout(dir string) bool {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return false
	}

	if fileExists(filepath.Join(dir, filepath.FromSlash(config.DefaultPath))) {
		return true
	}

	if dirExists(filepath.Join(dir, ".rpl", "attrs")) || dirExists(filepath.Join(dir, ".rpl", "runtimes")) {
		return true
	}

	for _, path := range sourceTreeAttrsPaths(dir) {
		if dirExists(path) {
			return true
		}
	}

	return false
}

// sourceTreeAttrsPaths covers the repository layout used while developing RPL
// itself, where built-in attrs live under src/attrs instead of a packaged
// .rpl/attrs folder near the executable or project root.
func sourceTreeAttrsPaths(projectRoot string) []string {
	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot == "" {
		return nil
	}

	return []string{
		filepath.Join(projectRoot, "attrs"),
		filepath.Join(projectRoot, ".rpl", "attrs"),
		filepath.Join(projectRoot, ".rpl", "runtimes"),
		filepath.Join(projectRoot, "src", "attrs"),
		filepath.Join(projectRoot, "src", ".rpl", "attrs"),
		filepath.Join(projectRoot, "src", ".rpl", "runtimes"),
	}
}

func shouldUseWorkingDirFallback(basePath string, projectRoot string) bool {
	if strings.TrimSpace(basePath) == "" {
		return false
	}

	return !hasProjectAttrsLayout(projectRoot)
}

func allowWorkingDirFallback(primaryRoot string, fallbackRoot string) bool {
	primaryRoot = strings.TrimSpace(primaryRoot)
	fallbackRoot = strings.TrimSpace(fallbackRoot)
	if fallbackRoot == "" || fallbackRoot == primaryRoot {
		return false
	}
	if isExecutableInstallRoot(fallbackRoot) {
		return false
	}

	return true
}

func isExecutableInstallRoot(projectRoot string) bool {
	bundledPath := strings.TrimSpace(config.BundledRuntimesPathFromExecutable())
	if bundledPath == "" {
		return false
	}

	rootAttrs := filepath.Join(projectRoot, ".rpl", "attrs")
	return filepath.Clean(rootAttrs) == filepath.Clean(bundledPath)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// List accepts both "one folder per attr" layouts and loose executable files
// so old and new local plugin layouts remain discoverable.
func List(dir string) ([]Binary, error) {
	trimmedDir := strings.TrimSpace(dir)
	if trimmedDir == "" {
		return nil, errors.New(localize.Text("папка attrs обязательна", "attrs directory is required"))
	}

	info, err := os.Stat(trimmedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Binary{}, nil
		}

		return nil, fmt.Errorf(localize.Text("чтение папки attrs %q: %w", "read attrs directory %q: %w"), trimmedDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf(localize.Text("путь attrs %q не является папкой", "attrs path %q is not a directory"), trimmedDir)
	}

	entries, err := os.ReadDir(trimmedDir)
	if err != nil {
		return nil, fmt.Errorf(localize.Text("чтение папки attrs %q: %w", "read attrs directory %q: %w"), trimmedDir, err)
	}

	items := make([]Binary, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			item, err := loadBinaryFromDir(filepath.Join(trimmedDir, entry.Name()), entry.Name())
			if err != nil {
				return nil, err
			}

			items = append(items, *item)
			continue
		}

		item, err := loadBinaryFromFile(filepath.Join(trimmedDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if item != nil {
			items = append(items, *item)
		}
	}

	return items, nil
}

// ListFromSearchPaths preserves the first hit for each author:name pair, which
// lets project-local attrs shadow fallback directories predictably.
func ListFromSearchPaths(dirs ...string) ([]Binary, error) {
	seen := make(map[string]struct{})
	items := make([]Binary, 0)

	for _, dir := range dirs {
		found, err := List(dir)
		if err != nil {
			return nil, err
		}

		for _, item := range found {
			key := strings.TrimSpace(item.Manifest.Author) + ":" + strings.TrimSpace(item.Manifest.Name)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			items = append(items, item)
		}
	}

	return items, nil
}

func Find(dir string, name string, author string) (*Binary, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, errors.New(localize.Text("имя attr обязательно", "attr name is required"))
	}

	trimmedAuthor := strings.TrimSpace(author)
	if trimmedAuthor == "" {
		return nil, errors.New(localize.Text("автор attr обязателен", "attr author is required"))
	}

	items, err := List(dir)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if item.Manifest.Name == trimmedName && item.Manifest.Author == trimmedAuthor {
			found := item
			return &found, nil
		}
	}

	return nil, rplerr.Newf(
		localize.Text("attr %q автора %q не найден в папке %q", "attr %q by author %q was not found in directory %q"),
		trimmedName,
		trimmedAuthor,
		dir,
	).WithHint(localize.Text("Посмотрите `rpl attr list` или создайте новый каркас через `rpl attr init author:name`.", "Try `rpl attr list` or create a scaffold with `rpl attr init author:name`."))
}

// FindInSearchPaths keeps track of every scanned directory so the final error
// can explain where discovery looked before giving up.
func FindInSearchPaths(name string, author string, dirs ...string) (*Binary, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, errors.New(localize.Text("имя attr обязательно", "attr name is required"))
	}

	trimmedAuthor := strings.TrimSpace(author)
	if trimmedAuthor == "" {
		return nil, errors.New(localize.Text("автор attr обязателен", "attr author is required"))
	}

	searched := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		searched = append(searched, dir)

		items, err := List(dir)
		if err != nil {
			return nil, err
		}

		for _, item := range items {
			if item.Manifest.Name == trimmedName && item.Manifest.Author == trimmedAuthor {
				found := item
				return &found, nil
			}
		}
	}

	return nil, rplerr.Newf(
		localize.Text("attr %q автора %q не найден", "attr %q by author %q was not found"),
		trimmedName,
		trimmedAuthor,
	).WithDetail(fmt.Sprintf(localize.Text(
		"Искали в папках: %s",
		"Searched in directories: %s",
	), strings.Join(searched, ", "))).WithHint(localize.Text(
		"Запустите `rpl attr list` и положите локальный attr в `.rpl/attrs` или `.rpl/runtimes` проекта.",
		"Run `rpl attr list` and place a local attr inside the project's `.rpl/attrs` or `.rpl/runtimes`.",
	))
}

func LoadManifest(path string) (*Manifest, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(localize.Text("чтение манифеста attr %q: %w", "read attr manifest %q: %w"), path, err)
	}

	var manifest Manifest
	if err := xml.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf(localize.Text("разбор манифеста attr %q: %w", "parse attr manifest %q: %w"), path, err)
	}

	return &manifest, nil
}

// loadBinaryFromDir merges manifest data with the folder name.
// Folder-based identity keeps local plugins easy to reason about:
// `.rpl/attrs/rpl:sql` always resolves to author `rpl`, name `sql`
// even when the manifest omits one of those fields.
func loadBinaryFromDir(dir string, folderName string) (*Binary, error) {
	manifestPath := filepath.Join(dir, DefaultManifestName)
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, err
	}

	applyFolderIdentity(manifest, folderName)
	entryName := manifestEntryName(manifest)

	if err := ensureLocalBinaryUpToDate(dir, entryName); err != nil {
		return nil, err
	}

	execPath, err := resolveExecutablePath(dir, entryName)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(execPath)
	if err != nil {
		return nil, fmt.Errorf(localize.Text("чтение исполняемого файла attr %q: %w", "read attr executable %q: %w"), execPath, err)
	}
	if !isExecutableFile(info) {
		return nil, fmt.Errorf(localize.Text("файл attr %q не является исполняемым", "attr file %q is not executable"), execPath)
	}

	if err := normalizeManifest(manifest, execPath); err != nil {
		return nil, err
	}

	return &Binary{
		Manifest:     *manifest,
		ManifestPath: manifestPath,
		ExecPath:     execPath,
	}, nil
}

// loadBinaryFromFile supports legacy "single executable + sibling xml" attrs
// outside the newer folder-based layout used in `.rpl/attrs`.
func loadBinaryFromFile(execPath string) (*Binary, error) {
	info, err := os.Stat(execPath)
	if err != nil {
		return nil, fmt.Errorf(localize.Text("чтение файла attr %q: %w", "read attr file %q: %w"), execPath, err)
	}
	if !isExecutableFile(info) {
		return nil, nil
	}

	manifestPath := manifestPathFor(execPath)
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	if err := normalizeManifest(manifest, execPath); err != nil {
		return nil, err
	}

	return &Binary{
		Manifest:     *manifest,
		ManifestPath: manifestPath,
		ExecPath:     execPath,
	}, nil
}

func manifestPathFor(execPath string) string {
	dir := filepath.Dir(execPath)
	base := strings.TrimSuffix(filepath.Base(execPath), filepath.Ext(execPath))
	return filepath.Join(dir, base+".xml")
}

// normalizeManifest fills backward-compatible defaults and validates that the
// manifest still points at the executable we actually found on disk.
func normalizeManifest(manifest *Manifest, execPath string) error {
	if manifest == nil {
		return errors.New(localize.Text("манифест attr отсутствует", "attr manifest is nil"))
	}

	base := strings.TrimSuffix(filepath.Base(execPath), filepath.Ext(execPath))
	if strings.TrimSpace(manifest.Name) == "" {
		manifest.Name = base
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return fmt.Errorf(localize.Text("у attr %q не указана версия", "attr %q version is missing"), manifest.Name)
	}

	entryName := manifestEntryName(manifest)
	execBase := filepath.Base(execPath)
	if execBase != entryName && !(entryName == DefaultExecutableName && execBase == LegacyExecutableName) {
		return fmt.Errorf(
			localize.Text("манифест attr %q ссылается на другой исполняемый файл %q", "attr manifest %q points to a different executable %q"),
			manifest.Name,
			entryName,
		)
	}

	manifest.Entry = entryName
	manifest.Executable = entryName
	if strings.TrimSpace(manifest.Path) == "" {
		manifest.Path = execPath
	}
	if strings.TrimSpace(manifest.SDKVersion) == "" {
		manifest.SDKVersion = "1"
	}

	return nil
}

func applyFolderIdentity(manifest *Manifest, folderName string) {
	if manifest == nil {
		return
	}

	author, name, ok := strings.Cut(folderName, ":")
	if !ok {
		return
	}

	if strings.TrimSpace(manifest.Author) == "" {
		manifest.Author = strings.TrimSpace(author)
	}
	if strings.TrimSpace(manifest.Name) == "" {
		manifest.Name = strings.TrimSpace(name)
	}
}

func manifestEntryName(manifest *Manifest) string {
	if manifest == nil {
		return DefaultExecutableName
	}

	if entry := strings.TrimSpace(manifest.Entry); entry != "" {
		return entry
	}
	if executable := strings.TrimSpace(manifest.Executable); executable != "" {
		return executable
	}

	return DefaultExecutableName
}

// resolveExecutablePath prefers the manifest entry, but still accepts the old
// `runtime` binary name for compatibility with existing local attrs.
func resolveExecutablePath(dir string, entryName string) (string, error) {
	primary := filepath.Join(dir, entryName)
	if _, err := os.Stat(primary); err == nil {
		return primary, nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf(localize.Text("чтение исполняемого файла attr %q: %w", "read attr executable %q: %w"), primary, err)
	}

	if entryName == DefaultExecutableName {
		legacy := filepath.Join(dir, LegacyExecutableName)
		if _, err := os.Stat(legacy); err == nil {
			return legacy, nil
		} else if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf(localize.Text("чтение исполняемого файла attr %q: %w", "read attr executable %q: %w"), legacy, err)
		}
	}

	return primary, nil
}

func isExecutableFile(info os.FileInfo) bool {
	if info == nil || info.IsDir() {
		return false
	}

	mode := info.Mode()
	if mode&0o111 != 0 {
		return true
	}

	switch strings.ToLower(filepath.Ext(info.Name())) {
	case ".exe", ".bat", ".cmd":
		return true
	default:
		return false
	}
}
