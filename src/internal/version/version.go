package version

import (
	"fmt"
	"rpl/pkg/error/localize"
)

var Version = "0.7.2"

var Author *string

func GeneratedAuthor() string {
	if Author == nil {
		return localize.Text("<Имя автора>", "<Author name>")
	}
	return fmt.Sprintf("<%s>", *Author)
}
