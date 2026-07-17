package cli

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"rpl/internal/config"
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"strings"
)

func (app *App) configCommand(args []string) error {
	if len(args) == 0 {
		return app.showConfig(nil)
	}

	switch strings.TrimSpace(args[0]) {
	case "show":
		return app.showConfig(args[1:])
	case "path":
		return app.printConfigPath(args[1:])
	case "init":
		return app.initConfig(args[1:])
	default:
		return rplerr.Newf(localize.Text("неизвестная config-команда %q", "unknown config command %q"), args[0]).
			WithHint(localize.Text("Доступные варианты: `show`, `path`, `init`.", "Available options are `show`, `path`, and `init`."))
	}
}

func (app *App) showConfig(args []string) error {
	global, remaining, err := extractGlobalFlag(args)
	if err != nil {
		return err
	}
	if len(remaining) != 0 {
		return rplerr.New(localize.Text("config show не принимает путь", "config show does not accept a path"))
	}

	var cfg *config.Config
	var path string
	if global {
		path, err = config.GlobalPath()
		if err == nil {
			cfg, err = config.LoadGlobalOrDefault()
		}
	} else {
		cfg, path, _, err = config.LoadForBase("")
	}
	if err != nil {
		return err
	}

	body, err := xml.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf(localize.Text("кодирование конфигурации: %w", "encode config: %w"), err)
	}
	_, _ = fmt.Fprintf(os.Stdout, "%s\n%s%s\n", path, xml.Header, body)
	return nil
}

func (app *App) printConfigPath(args []string) error {
	global, remaining, err := extractGlobalFlag(args)
	if err != nil {
		return err
	}
	if len(remaining) != 0 {
		return rplerr.New(localize.Text("config path не принимает путь", "config path does not accept a path"))
	}

	var path string
	if global {
		path, err = config.GlobalPath()
	} else {
		path, _, err = config.PathForBase("")
	}
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(os.Stdout, path)
	return nil
}

func (app *App) initConfig(args []string) error {
	global, remaining, err := extractGlobalFlag(args)
	if err != nil {
		return err
	}
	if len(remaining) != 0 {
		return rplerr.New(localize.Text("config init не принимает путь", "config init does not accept a path"))
	}

	var path string
	if global {
		path, err = config.GlobalPath()
		if err == nil {
			_, err = config.LoadOrCreateGlobal()
		}
	} else {
		path = filepath.FromSlash(config.DefaultPath)
		_, err = config.LoadOrCreate(path)
	}
	if err != nil {
		return err
	}

	absolute, absErr := filepath.Abs(path)
	if absErr == nil {
		path = absolute
	}
	_, _ = fmt.Fprintln(os.Stdout, fmt.Sprintf(localize.Text("конфигурация готова: %s", "configuration ready: %s"), path))
	return nil
}
