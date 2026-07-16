package localize

import "strings"

const (
	LangRU   = "ru"
	LangEN   = "en"
	LangTEST = "ts"
)

var Language = LangEN
var UseColor = true

func IsRussian() bool {
	return Language != LangEN
}

func Text(ru, en string) string {
	if IsRussian() {
		return ru
	} else if Language == LangTEST {
		return en
	}
	return en
}

func SText(test, en string) string {
	if Language == LangTEST {
		return test
	}
	return en
}

func SetLanguage(lang string) {
	switch NormalizeLanguage(lang) {
	case LangEN:
		Language = LangEN
	case LangTEST:
		Language = LangTEST
	default:
		Language = LangRU
	}
}

func NormalizeLanguage(lang string) string {
	normalized := strings.ToLower(strings.TrimSpace(lang))

	switch {
	case strings.HasPrefix(normalized, LangEN):
		return LangEN
	case strings.HasPrefix(normalized, LangRU):
		return LangRU
	case strings.HasPrefix(normalized, LangTEST):
		return LangTEST
	default:
		return LangEN
	}
}
