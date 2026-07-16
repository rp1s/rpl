package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"rpl/internal/config"
	"rpl/internal/fsutil"
	"rpl/pkg/ansi"
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"strings"
)

func (app *App) initProject(args []string) error {
	if len(args) > 1 {
		return rplerr.New(localize.Text("команда init принимает не более одного пути", "init command accepts at most one path"))
	}

	root := "."
	if len(args) == 1 {
		root = strings.TrimSpace(args[0])
		if root == "" {
			return rplerr.New(localize.Text("путь проекта пуст", "project path is empty"))
		}
	}

	projectRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf(localize.Text("определение пути проекта %q: %w", "resolve project path %q: %w"), root, err)
	}

	createdItems := make([]string, 0)
	keptItems := make([]string, 0)

	if _, err := ensureDirExists(projectRoot); err != nil {
		return err
	}
	if created, err := ensureDirExists(filepath.Join(projectRoot, "src")); err != nil {
		return err
	} else if created {
		createdItems = append(createdItems, filepath.Join(projectRoot, "src"))
	}

	cfg := config.Default()
	configPath := filepath.Join(projectRoot, config.DefaultPath)
	if created, err := writeConfigIfMissing(configPath, cfg); err != nil {
		return err
	} else if created {
		createdItems = append(createdItems, configPath)
	} else {
		keptItems = append(keptItems, configPath)
	}

	pluginsDir := filepath.Join(projectRoot, cfg.Runtimes.Directory)
	if created, err := ensureDirExists(pluginsDir); err != nil {
		return err
	} else if created {
		createdItems = append(createdItems, pluginsDir)
	}

	readmePath := filepath.Join(projectRoot, "README.md")
	if created, err := writeFileIfMissing(readmePath, projectReadmeTemplate(filepath.Base(projectRoot), cfg.Runtimes.Directory)); err != nil {
		return err
	} else if created {
		createdItems = append(createdItems, readmePath)
	} else {
		keptItems = append(keptItems, readmePath)
	}

	mainFilePath := filepath.Join(projectRoot, "src", "main.rpl")
	if created, err := writeFileIfMissing(mainFilePath, defaultProjectMainFile); err != nil {
		return err
	} else if created {
		createdItems = append(createdItems, mainFilePath)
	} else {
		keptItems = append(keptItems, mainFilePath)
	}

	ansi.Println(os.Stdout, ansi.Success, fmt.Sprintf(localize.Text("проект инициализирован: %s", "project initialized: %s"), projectRoot))
	if len(createdItems) > 0 {
		ansi.Println(os.Stdout, ansi.Info, fmt.Sprintf(localize.Text("создано: %s", "created: %s"), formatPaths(projectRoot, createdItems)))
	}
	if len(keptItems) > 0 {
		ansi.Println(os.Stdout, ansi.Warn, fmt.Sprintf(localize.Text("уже существует, оставлено как есть: %s", "already exists, kept as is: %s"), formatPaths(projectRoot, keptItems)))
	}

	return nil
}

func ensureDirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	switch {
	case err == nil && !info.IsDir():
		return false, fmt.Errorf(localize.Text("путь %q существует и не является папкой", "path %q exists and is not a directory"), path)
	case err == nil:
		return false, nil
	case !os.IsNotExist(err):
		return false, fmt.Errorf(localize.Text("чтение пути %q: %w", "stat path %q: %w"), path, err)
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return false, fmt.Errorf(localize.Text("создание папки %q: %w", "create directory %q: %w"), path, err)
	}

	return true, nil
}

func writeFileIfMissing(path string, body string) (bool, error) {
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return false, nil
		}
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf(localize.Text("чтение пути %q: %w", "stat path %q: %w"), path, err)
	}

	if err := fsutil.WriteFile(path, []byte(body), 0o644); err != nil {
		return false, fmt.Errorf(localize.Text("запись файла %q: %w", "write file %q: %w"), path, err)
	}

	return true, nil
}

func writeConfigIfMissing(path string, cfg *config.Config) (bool, error) {
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return false, nil
		}
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf(localize.Text("чтение пути %q: %w", "stat path %q: %w"), path, err)
	}

	if err := config.Save(path, cfg); err != nil {
		return false, err
	}

	return true, nil
}

func formatPaths(root string, paths []string) string {
	items := make([]string, 0, len(paths))
	seen := make(map[string]struct{})

	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		if rel == "." {
			rel = root
		}

		rel = filepath.ToSlash(rel)
		if _, exists := seen[rel]; exists {
			continue
		}

		seen[rel] = struct{}{}
		items = append(items, rel)
	}

	return strings.Join(items, ", ")
}

func projectReadmeTemplate(projectName string, pluginDir string) string {
	if strings.TrimSpace(projectName) == "" {
		projectName = "RPL Project"
	}
	if strings.TrimSpace(pluginDir) == "" {
		pluginDir = ".rpl/attrs"
	}

	return fmt.Sprintf("# %s\n\nProject initialized with RPL.\n\n## Structure\n\n- src/ - your .rpl source files\n- %s - local generator attrs\n\n## Quick Start\n\nRun the sample schema:\n\n```bash\nrpl run src/main.rpl\n```\n\nCreate a new attr scaffold:\n\n```bash\nrpl attr init rpl:custom\n```\n", projectName, pluginDir)
}

const defaultProjectMainFile = `target(lang: golang)

attrs (
    "rpl:validate"
)

model Example {
    Name string @validate(min: 1, max: 32)
}
`
