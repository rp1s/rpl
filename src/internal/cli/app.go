package cli

import (
	"rpl/internal/config"
	"rpl/internal/jsonapi"
	compilersvc "rpl/internal/service/compiler"
)

type App struct {
	cfg      *config.Config
	compiler *compilersvc.Service
	api      *jsonapi.Server
}

func New(cfg *config.Config) *App {
	return &App{
		cfg:      cfg,
		compiler: compilersvc.New(),
		api:      jsonapi.New(cfg),
	}
}

func (app *App) RunJSONAPI() error {
	return app.api.Run()
}
