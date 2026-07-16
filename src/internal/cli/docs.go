package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"rpl/internal/docs"
	"rpl/internal/fsutil"
	"rpl/pkg/ansi"
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"strings"
)

func (app *App) writeDocs(args []string) error {
	sourcePath, err := resolveDocsSource(args)
	if err != nil {
		return err
	}

	file, absolutePath, err := app.compiler.LoadFile(sourcePath)
	if err != nil {
		return err
	}

	readmePath := docs.OutputPath(absolutePath)
	body := docs.BuildReadme(file, absolutePath)
	if err := fsutil.WriteFile(readmePath, []byte(body), 0o644); err != nil {
		return fmt.Errorf(localize.Text("запись README %q: %w", "write README %q: %w"), readmePath, err)
	}

	ansi.Println(os.Stdout, ansi.Success, fmt.Sprintf(localize.Text("README создан: %s", "README generated: %s"), relativeOrAbsolute(readmePath)))
	return nil
}

func resolveDocsSource(args []string) (string, error) {
	switch len(args) {
	case 0:
		for _, candidate := range []string{"src/main.rpl", "main.rpl"} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
		return "", rplerr.New(localize.Text("не найден файл схемы для docs", "schema file for docs was not found")).
			WithHint(localize.Text("Передайте путь явно: `rpl docs src/main.rpl`.", "Pass the path explicitly: `rpl docs src/main.rpl`."))
	case 1:
		return onePathArg("docs", args)
	default:
		return "", rplerr.New(localize.Text("команда docs принимает не более одного пути", "docs command accepts at most one path"))
	}
}

func relativeOrAbsolute(path string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return filepath.ToSlash(path)
	}

	rel, err := filepath.Rel(cwd, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
