package language

import (
	"fmt"
	"rpl/internal/config"
	"rpl/pkg/error/localize"
)

type State struct {
	Lang    string
	Message string
}

type Service struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

func (service *Service) Current() string {
	return localize.Language
}

// CurrentAt switches localization to the nearest project config for the given
// path before reporting the active language.
func (service *Service) CurrentAt(basePath string) string {
	if err := service.ApplyAt(basePath); err != nil {
		return localize.Language
	}

	return localize.Language
}

// ApplyAt loads the nearest project config for the given path and applies its
// localization settings to the current process.
func (service *Service) ApplyAt(basePath string) error {
	cfg, _, _, err := config.LoadForBase(basePath)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = service.cfg
	}
	if cfg == nil {
		return nil
	}

	localize.SetLanguage(cfg.Localization.Language)
	localize.UseColor = cfg.UseColor()
	return nil
}

func (service *Service) Set(lang string) (State, error) {
	normalized := localize.NormalizeLanguage(lang)
	if normalized == localize.Language {
		return State{
			Lang:    localize.Language,
			Message: fmt.Sprintf(localize.Text("Язык уже был сменён на %s :)", "The language has already been changed to %s :)"), localize.Language),
		}, nil
	}

	if service.cfg != nil {
		service.cfg.Localization.Language = normalized
		exists, err := config.DefaultExists()
		if err != nil {
			return State{}, err
		}
		if exists {
			if err := config.SaveDefault(service.cfg); err != nil {
				return State{}, fmt.Errorf(localize.Text("ошибка сохранения конфигурации: %s", "failed to save config: %s"), err.Error())
			}
		}
	}

	localize.SetLanguage(normalized)
	return State{
		Lang:    localize.Language,
		Message: fmt.Sprintf(localize.Text("Язык сменён на %s", "Language changed to %s"), localize.Language),
	}, nil
}

// SetAt updates the nearest project config for the given path when it exists
// and always applies the requested language to the current process.
func (service *Service) SetAt(basePath string, lang string) (State, error) {
	cfg, configPath, exists, err := config.LoadForBase(basePath)
	if err != nil {
		return State{}, err
	}
	if cfg == nil {
		cfg = config.Default()
	}

	normalized := localize.NormalizeLanguage(lang)
	if normalized == localize.NormalizeLanguage(cfg.Localization.Language) {
		localize.SetLanguage(normalized)
		localize.UseColor = cfg.UseColor()
		return State{
			Lang:    localize.Language,
			Message: fmt.Sprintf(localize.Text("Язык уже был сменён на %s :)", "The language has already been changed to %s :)"), localize.Language),
		}, nil
	}

	cfg.Localization.Language = normalized
	if exists {
		if err := config.Save(configPath, cfg); err != nil {
			return State{}, fmt.Errorf(localize.Text("ошибка сохранения конфигурации: %s", "failed to save config: %s"), err.Error())
		}
	}

	localize.SetLanguage(normalized)
	localize.UseColor = cfg.UseColor()
	return State{
		Lang:    localize.Language,
		Message: fmt.Sprintf(localize.Text("Язык сменён на %s", "Language changed to %s"), localize.Language),
	}, nil
}
