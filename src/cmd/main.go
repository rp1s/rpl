package main

import (
	"os"
	"rpl/internal/cli"
	"rpl/internal/config"
	"rpl/internal/version"
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"rpl/pkg/fingerprint"
	"strings"
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

	// :)
	if err := checkfingerprint(); err != nil {
		fail(err)
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

// checkfingerprint blocks startup only for explicitly device-locked builds.
func checkfingerprint() error {
	if strings.TrimSpace(version.Fingerprint) == "" {
		return nil
	}
	key, err := fingerprint.Fingerprint()
	return validateFingerprint(key, err)
}

func validateFingerprint(key string, lookupErr error) error {
	if strings.TrimSpace(version.Fingerprint) == "" {
		return nil
	}
	if lookupErr != nil {
		return rplerr.Wrap(lookupErr, localize.Text(
			"не удалось определить отпечаток устройства",
			"failed to determine the device fingerprint",
		)).WithHint(localize.Text(
			"Проверьте доступ к системным идентификаторам и попробуйте снова.",
			"Check access to system identifiers and try again.",
		))
	}
	if version.Fingerprint == key {
		return nil
	}

	return rplerr.New(localize.Text(
		"эта сборка RPL предназначена для другого устройства",
		"this RPL build is intended for a different device",
	)).WithDetail(localize.Text(
		"Отпечаток текущего устройства не совпадает с отпечатком, с которым была собрана эта версия.",
		"The current device fingerprint does not match the fingerprint this build was compiled for.",
	)).WithHint(localize.Text(
		"Пересоберите RPL на этом устройстве или обновите fingerprint в internal/version/version.go.",
		"Rebuild RPL on this device or update the fingerprint in internal/version/version.go.",
	))
}
