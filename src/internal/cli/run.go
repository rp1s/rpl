package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"rpl/pkg/ansi"
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"strings"
)

func (app *App) runFile(args []string) error {
	path, outputDir, err := parseRunArgs(args)
	if err != nil {
		return err
	}

	if _, err := app.compiler.RunFileTo(path, outputDir); err != nil {
		return err
	}

	if outputDir == "" {
		ansi.Println(os.Stdout, ansi.Success, fmt.Sprintf(localize.Text("генерация завершена: %s", "generation completed: %s"), path))
		return nil
	}

	ansi.Println(os.Stdout, ansi.Success, fmt.Sprintf(localize.Text("генерация завершена: %s -> %s", "generation completed: %s -> %s"), path, outputDir))
	return nil
}

func parseRunArgs(args []string) (string, string, error) {
	if len(args) == 0 {
		return "", "", rplerr.New(localize.Text("не указан путь", "path is required")).
			WithHint(localize.Text("Например: `rpl run src/main.rpl` или `rpl run src/main.rpl out user`.", "For example: `rpl run src/main.rpl` or `rpl run src/main.rpl out user`."))
	}

	path := strings.TrimSpace(args[0])
	if path == "" {
		return "", "", rplerr.New(localize.Text("путь пуст", "path is empty"))
	}

	if len(args) == 1 {
		return path, "", nil
	}

	if len(args) != 3 || !strings.EqualFold(strings.TrimSpace(args[1]), "out") {
		return "", "", rplerr.New(localize.Text("неверный синтаксис команды run", "invalid run command syntax")).
			WithHint(localize.Text("Используйте `rpl run <file.rpl> out <dir>`.", "Use `rpl run <file.rpl> out <dir>`."))
	}

	outputDir := strings.TrimSpace(args[2])
	if outputDir == "" {
		return "", "", rplerr.New(localize.Text("папка вывода пуста", "output directory is empty"))
	}

	absoluteOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return "", "", rplerr.Newf(localize.Text("не удалось определить папку вывода %q", "failed to resolve output directory %q"), outputDir)
	}

	return path, absoluteOutputDir, nil
}
