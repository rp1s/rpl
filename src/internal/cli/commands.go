package cli

import (
	"fmt"
	"io"
	"os"
	"rpl/internal/version"
	"rpl/pkg/ansi"
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"strings"
)

func (app *App) Execute(args []string) error {
	if len(args) == 0 {
		app.printHelp()
		return nil
	}

	command := strings.TrimSpace(args[0])
	switch command {
	case "run", "generate", "gen", "g":
		return app.runFile(args[1:])
	case "fmt", "format":
		return app.formatFile(args[1:])
	case "auto":
		return app.autoCommand(args[1:])
	case "docs", "doc":
		return app.writeDocs(args[1:])
	case "init", "install", "new":
		return app.initProject(args[1:])
	case "attr":
		return app.attrCommand(args[1:])
	case "config":
		return app.configCommand(args[1:])
	case "runtime":
		return app.RunJSONAPI()
	case "help", "-h", "--help":
		app.printHelp()
		return nil
	default:
		return rplerr.Newf(localize.Text("неизвестная команда %q", "unknown command %q"), command).
			WithHint(localize.Text("Посмотрите `rpl help`, чтобы увидеть доступные команды.", "Run `rpl help` to see the available commands."))
	}
}

func (app *App) printHelp() {
	renderHelp(os.Stdout)
}

type helpSection struct {
	title   string
	entries []helpEntry
}

type helpEntry struct {
	command     string
	description string
}

func renderHelp(out io.Writer) {
	sections := []helpSection{
		{
			title: localize.Text("Проект", "Project"),
			entries: []helpEntry{
				{command: "init [dir]", description: localize.Text("создать новый проект", "create a new project")},
				{command: "new [dir]", description: localize.Text("алиас для init", "alias for init")},
			},
		},
		{
			title: localize.Text("Схема", "Schema"),
			entries: []helpEntry{
				{command: "run <file.rpl> [out dir]", description: localize.Text("запустить генерацию", "run generation")},
				{command: "gen <file.rpl> [out dir]", description: localize.Text("короткий алиас для run", "short alias for run")},
				{command: "fmt <file.rpl>", description: localize.Text("отформатировать файл схемы", "format a schema file")},
				{command: "auto set import <file.rpl>", description: localize.Text("добавить недостающие attrs и импорты", "add missing attrs and imports")},
				{command: "docs [file.rpl]", description: localize.Text("сгенерировать README по схеме", "generate README from schema")},
			},
		},
		{
			title: localize.Text("Атрибуты", "Attrs"),
			entries: []helpEntry{
				{command: "attr init [--global] author:name", description: localize.Text("создать локальный или глобальный attr", "create a local or global attr")},
				{command: "attr list", description: localize.Text("показать локальные attrs", "list local attrs")},
				{command: "attr info author:name", description: localize.Text("показать манифест attr", "show attr manifest")},
			},
		},
		{
			title: localize.Text("Инструменты", "Tools"),
			entries: []helpEntry{
				{command: "config show [--global]", description: localize.Text("показать эффективную конфигурацию", "show effective configuration")},
				{command: "config init [--global]", description: localize.Text("создать конфигурацию", "create configuration")},
				{command: "runtime", description: localize.Text("запустить JSON API сервер", "start the JSON API server")},
				{command: "help", description: localize.Text("показать эту справку", "show this help")},
			},
		},
	}

	examples := []string{
		"rpl init",
		"rpl run src/main.rpl out build",
		"rpl attr init rpl:custom",
	}

	_, _ = fmt.Fprintln(out, ansi.Heading(out, "RPL "+version.Version))
	_, _ = fmt.Fprintln(out, ansi.Muted(out, localize.Text("Компилятор схем, генератор кода и runtime-сервер.", "Schema compiler, code generator, and runtime server.")))
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, ansi.Heading(out, localize.Text("Использование", "Usage")))
	_, _ = fmt.Fprintln(out, "  "+ansi.Accent(out, "rpl <command> [arguments]"))

	for _, section := range sections {
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprintln(out, ansi.Heading(out, section.title))
		printHelpSection(out, section.entries)
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, ansi.Heading(out, localize.Text("Примеры", "Examples")))
	for _, example := range examples {
		_, _ = fmt.Fprintln(out, "  "+ansi.Accent(out, example))
	}
}

func printHelpSection(out io.Writer, entries []helpEntry) {
	width := 0
	for _, entry := range entries {
		if len(entry.command) > width {
			width = len(entry.command)
		}
	}

	for _, entry := range entries {
		padding := width - len(entry.command) + 3
		if padding < 3 {
			padding = 3
		}
		_, _ = fmt.Fprintln(out, "  "+ansi.Accent(out, entry.command)+strings.Repeat(" ", padding)+ansi.Muted(out, entry.description))
	}
}

func onePathArg(command string, args []string) (string, error) {
	if len(args) == 0 {
		return "", rplerr.New(localize.Text("не указан путь", "path is required")).
			WithHint(localize.Text("Например: `rpl run src/main.rpl`.", "For example: `rpl run src/main.rpl`."))
	}
	if len(args) > 1 {
		return "", rplerr.Newf(localize.Text("команда %q принимает только один путь", "command %q accepts only one path"), command)
	}

	path := strings.TrimSpace(args[0])
	if path == "" {
		return "", rplerr.New(localize.Text("путь пуст", "path is empty"))
	}

	return path, nil
}
