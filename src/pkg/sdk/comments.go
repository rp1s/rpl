package sdk

import (
	"fmt"
	"strings"
)

func DocComment(primary string, fallback string, args ...any) string {
	text := strings.TrimSpace(Text(primary, fallback))
	if text == "" {
		return ""
	}
	if len(args) > 0 {
		text = fmt.Sprintf(text, args...)
	}

	lines := make([]string, 0)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, "// "+line)
	}

	return strings.Join(lines, "\n")
}

func WithDocComment(code string, primary string, fallback string, args ...any) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}

	comment := DocComment(primary, fallback, args...)
	if comment == "" {
		return code
	}

	return comment + "\n" + code
}
