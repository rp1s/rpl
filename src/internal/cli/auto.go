package cli

import (
	"fmt"
	"os"
	compilersvc "rpl/internal/service/compiler"
	"rpl/pkg/ansi"
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"strings"
)

func (app *App) autoCommand(args []string) error {
	if len(args) < 3 {
		return rplerr.New(localize.Text("ожидалась команда `auto set import <file.rpl>`", "expected `auto set import <file.rpl>`")).
			WithHint(localize.Text("Пример: `rpl auto set import src/main.rpl`.", "Example: `rpl auto set import src/main.rpl`."))
	}

	if strings.TrimSpace(args[0]) != "set" || strings.TrimSpace(args[1]) != "import" {
		return rplerr.Newf(localize.Text("неизвестная auto-команда %q", "unknown auto command %q"), strings.Join(args, " ")).
			WithHint(localize.Text("Используйте `rpl auto set import <file.rpl>`.", "Use `rpl auto set import <file.rpl>`."))
	}

	path, err := onePathArg("auto set import", args[2:])
	if err != nil {
		return err
	}

	service := compilersvc.New()
	if err := service.AutoSetImportsInPlace(path); err != nil {
		return err
	}

	ansi.Println(os.Stdout, ansi.Success, fmt.Sprintf(localize.Text("импорты и attrs обновлены: %s", "imports and attrs updated: %s"), path))
	return nil
}
