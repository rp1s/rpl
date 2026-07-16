package ansi

import (
	"fmt"
	"io"
	"os"
	"rpl/pkg/error/localize"
	"strings"
)

type Level struct {
	Label string
	FG    string
	BG    string
}

var (
	Error   = Level{Label: " ERROR ", FG: "97", BG: "41"}
	Warn    = Level{Label: " WARN  ", FG: "30", BG: "43"}
	Info    = Level{Label: " INFO  ", FG: "97", BG: "44"}
	Hint    = Level{Label: " HINT  ", FG: "30", BG: "46"}
	Note    = Level{Label: " NOTE  ", FG: "97", BG: "100"}
	Success = Level{Label: "  OK   ", FG: "30", BG: "42"}
)

const (
	reset = "\033[0m"
	bold  = "\033[1m"
	dim   = "\033[2m"
	cyan  = "\033[36m"
)

func Enabled(writer io.Writer) bool {
	if !localize.UseColor {
		return false
	}
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return false
	}

	file, ok := writer.(*os.File)
	if !ok {
		return localize.UseColor
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

func Sprintln(writer io.Writer, level Level, text string) string {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return label(writer, level)
	}

	lines := strings.Split(text, "\n")
	prefix := label(writer, level)
	padding := strings.Repeat(" ", visibleWidth(level.Label)+1)

	rendered := make([]string, 0, len(lines))
	rendered = append(rendered, prefix+" "+lines[0])
	for _, line := range lines[1:] {
		rendered = append(rendered, padding+line)
	}

	return strings.Join(rendered, "\n")
}

func Println(writer io.Writer, level Level, text string) {
	if writer == nil {
		writer = os.Stdout
	}

	_, _ = fmt.Fprintln(writer, Sprintln(writer, level, text))
}

func Heading(writer io.Writer, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if !Enabled(writer) {
		return text
	}

	return bold + cyan + text + reset
}

func Accent(writer io.Writer, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if !Enabled(writer) {
		return text
	}

	return bold + text + reset
}

func Muted(writer io.Writer, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if !Enabled(writer) {
		return text
	}

	return dim + text + reset
}

func label(writer io.Writer, level Level) string {
	if !Enabled(writer) {
		return "[" + strings.TrimSpace(level.Label) + "]"
	}

	return fmt.Sprintf("\033[1;%s;%sm%s%s", level.FG, level.BG, level.Label, reset)
}

func visibleWidth(text string) int {
	return len(strings.TrimSpace(text)) + 2
}
