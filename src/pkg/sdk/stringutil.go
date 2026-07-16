package sdk

import (
	"strconv"
	"strings"
	"unicode"
)

func Quote(text string) string {
	return strconv.Quote(text)
}

func Indent(text string, prefix string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	for i := range lines {
		if lines[i] == "" {
			continue
		}
		lines[i] = prefix + lines[i]
	}

	return strings.Join(lines, "\n")
}

func JoinNonEmpty(parts ...string) string {
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		items = append(items, strings.TrimSpace(part))
	}

	return strings.Join(items, "\n\n")
}

func SnakeCase(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	var builder strings.Builder
	var prevLower bool
	var prevUnderscore bool
	for i, char := range name {
		if char == '.' {
			if builder.Len() > 0 && !prevUnderscore {
				builder.WriteByte('_')
				prevUnderscore = true
			}
			prevLower = false
			continue
		}

		if unicode.IsUpper(char) {
			if i > 0 && prevLower && !prevUnderscore {
				builder.WriteByte('_')
				prevUnderscore = true
			}
			builder.WriteRune(unicode.ToLower(char))
			prevLower = false
			prevUnderscore = false
			continue
		}

		if char == '-' || char == ' ' {
			if !prevUnderscore {
				builder.WriteByte('_')
			}
			prevLower = false
			prevUnderscore = true
			continue
		}

		builder.WriteRune(unicode.ToLower(char))
		prevLower = unicode.IsLetter(char) || unicode.IsDigit(char)
		prevUnderscore = false
	}

	return strings.Trim(builder.String(), "_")
}

func LowerCamel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	runes := []rune(name)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}
