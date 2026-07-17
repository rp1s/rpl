package main

import (
	"os"
	"rpl/internal/cli"
	"rpl/internal/config"
	"rpl/internal/version"
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
)

// main wires config-dependent globals first, then hands control to the CLI.
func main() {
	cfg, err := loadConfig()
	if err != nil {
		fail(err)
	}

	localize.SetLanguage(cfg.Localization.Language)
	localize.UseColor = cfg.UseColor()
	if cfg.AuthorData != nil {
		version.Author = cfg.AuthorData.AuthorName
	}

	runtimeApp := cli.New(cfg)

	if err := runtimeApp.Execute(os.Args[1:]); err != nil {
		fail(err)
	}
}

func loadConfig() (*config.Config, error) {
	return config.LoadDefaultOrDefault()
}

func fail(err error) {
	if err == nil {
		return
	}

	rplerr.Print(os.Stderr, err)
	os.Exit(1)
}
