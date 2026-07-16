package version

import (
	"fmt"
	"rpl/pkg/error/localize"
)

var Version = "0.6.0"

var Author *string

// Fingerprint optionally locks a private build to one device. Public release
// builds leave it empty and therefore run on every supported machine. It can
// be injected with: -ldflags "-X rpl/internal/version.Fingerprint=<value>".
var Fingerprint string

func GeneratedAuthor() string {
	if Author == nil {
		return localize.Text("<Имя автора>", "<Author name>")
	}
	return fmt.Sprintf("<%s>", *Author)
}
