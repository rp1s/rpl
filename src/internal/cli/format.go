package cli

import (
	"fmt"
	"os"
	"rpl/internal/fsutil"
	"rpl/pkg/ansi"
	"rpl/pkg/error/localize"
)

func (app *App) formatFile(args []string) error {
	path, err := onePathArg("fmt", args)
	if err != nil {
		return err
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	formatted, err := app.compiler.Format(string(body), path)
	if err != nil {
		return err
	}

	if err := fsutil.WriteFile(path, []byte(formatted), 0o644); err != nil {
		return err
	}

	ansi.Println(os.Stdout, ansi.Success, fmt.Sprintf(localize.Text("файл отформатирован: %s", "file formatted: %s"), path))
	return nil
}
