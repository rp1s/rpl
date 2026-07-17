package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"rpl/internal/config"
	"rpl/internal/plugins"
	"rpl/pkg/ansi"
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"sort"
	"strings"
)

func (app *App) attrCommand(args []string) error {
	if len(args) == 0 {
		return app.listAttrs()
	}

	switch strings.TrimSpace(args[0]) {
	case "init":
		return app.initAttr(args[1:])
	case "list":
		return app.listAttrs()
	case "info":
		return app.attrInfo(args[1:])
	default:
		return rplerr.Newf(localize.Text("неизвестная attr-команда %q", "unknown attr command %q"), args[0]).
			WithHint(localize.Text("Доступные варианты: `init`, `list`, `info`.", "Available options are `init`, `list`, and `info`."))
	}
}

func (app *App) initAttr(args []string) error {
	global, remaining, err := extractGlobalFlag(args)
	if err != nil {
		return err
	}

	identifier, err := onePathArg("attr init", remaining)
	if err != nil {
		return err
	}

	attrsRoot := ""
	if global {
		if _, err := config.LoadOrCreateGlobal(); err != nil {
			return err
		}
		attrsRoot, err = config.GlobalAttrsPath()
		if err != nil {
			return err
		}
	}

	result, err := plugins.CreateScaffold(plugins.ScaffoldInput{
		ProjectRoot: ".",
		AttrsRoot:   attrsRoot,
		Identifier:  identifier,
	})
	if err != nil {
		return err
	}

	projectRoot, _ := filepath.Abs(".")
	ansi.Println(os.Stdout, ansi.Success, fmt.Sprintf(localize.Text("каркас attr создан: %s", "attr scaffold created: %s"), result.AttrDir))
	if len(result.CreatedPaths) > 0 {
		ansi.Println(os.Stdout, ansi.Info, fmt.Sprintf(localize.Text("создано: %s", "created: %s"), formatPaths(projectRoot, result.CreatedPaths)))
	}
	if len(result.ExistingPaths) > 0 {
		ansi.Println(os.Stdout, ansi.Warn, fmt.Sprintf(localize.Text("уже существует, оставлено как есть: %s", "already exists, kept as is: %s"), formatPaths(projectRoot, result.ExistingPaths)))
	}

	return nil
}

func extractGlobalFlag(args []string) (bool, []string, error) {
	global := false
	remaining := make([]string, 0, len(args))
	for _, arg := range args {
		switch strings.TrimSpace(arg) {
		case "--global", "-g":
			global = true
		default:
			if strings.HasPrefix(strings.TrimSpace(arg), "-") {
				return false, nil, rplerr.Newf(localize.Text("неизвестный флаг %q", "unknown flag %q"), arg)
			}
			remaining = append(remaining, arg)
		}
	}
	return global, remaining, nil
}

func (app *App) listAttrs() error {
	items, err := plugins.ListConfigured()
	if err != nil {
		return err
	}

	sort.Slice(items, func(i int, j int) bool {
		if items[i].Manifest.Author == items[j].Manifest.Author {
			return items[i].Manifest.Name < items[j].Manifest.Name
		}
		return items[i].Manifest.Author < items[j].Manifest.Author
	})

	if len(items) == 0 {
		ansi.Println(os.Stdout, ansi.Warn, localize.Text("attrs не найдены", "attrs were not found"))
		return nil
	}

	ansi.Println(os.Stdout, ansi.Info, localize.Text("Доступные attrs", "Available attrs"))
	for _, item := range items {
		label := fmt.Sprintf("%s:%s", item.Manifest.Author, item.Manifest.Name)
		if strings.TrimSpace(item.Manifest.Version) != "" {
			label += " v" + item.Manifest.Version
		}
		if strings.TrimSpace(item.Manifest.Description) != "" {
			label += " - " + item.Manifest.Description
		}
		_, _ = fmt.Fprintln(os.Stdout, "  "+label)
	}

	return nil
}

func (app *App) attrInfo(args []string) error {
	identifier, err := onePathArg("attr info", args)
	if err != nil {
		return err
	}

	author, name, ok := strings.Cut(strings.TrimSpace(identifier), ":")
	if !ok || strings.TrimSpace(author) == "" || strings.TrimSpace(name) == "" {
		return rplerr.New(localize.Text("идентификатор attr должен быть в формате author:name", "attr identifier must use author:name format")).
			WithHint(localize.Text("Пример: `rpl attr info rpl:sql`.", "Example: `rpl attr info rpl:sql`."))
	}

	item, err := plugins.FindConfigured(strings.TrimSpace(name), strings.TrimSpace(author))
	if err != nil {
		return err
	}

	ansi.Println(os.Stdout, ansi.Info, fmt.Sprintf("%s:%s", item.Manifest.Author, item.Manifest.Name))
	_, _ = fmt.Fprintf(os.Stdout, "  version: %s\n", item.Manifest.Version)
	_, _ = fmt.Fprintf(os.Stdout, "  entry:   %s\n", item.Manifest.Entry)
	_, _ = fmt.Fprintf(os.Stdout, "  path:    %s\n", item.ExecPath)
	if strings.TrimSpace(item.Manifest.Description) != "" {
		_, _ = fmt.Fprintf(os.Stdout, "  about:   %s\n", item.Manifest.Description)
	}

	return nil
}
