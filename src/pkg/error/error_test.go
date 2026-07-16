package error

import (
	"errors"
	"rpl/pkg/error/localize"
	"strings"
	"testing"
)

func TestFormatShowsSnippetAndHint(t *testing.T) {
	originalLang := localize.Language
	originalColor := localize.UseColor
	localize.SetLanguage(localize.LangEN)
	localize.UseColor = false
	defer func() {
		localize.SetLanguage(originalLang)
		localize.UseColor = originalColor
	}()

	err := New("unexpected character '='").
		WithLocation("schema.rpl", 2, 15).
		WithSource("model User {\n    Name string =\n}\n").
		WithHint("Use a colon for named arguments: `name: value`.")

	rendered := Format(nil, err)
	if !strings.Contains(rendered, "unexpected character '='") {
		t.Fatalf("expected message in rendered error, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "schema.rpl") {
		t.Fatalf("expected file path in rendered error, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "^") {
		t.Fatalf("expected caret indicator in rendered error, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Use a colon for named arguments") {
		t.Fatalf("expected hint in rendered error, got:\n%s", rendered)
	}
}

func TestFormatAddsHintForGenericCommandError(t *testing.T) {
	originalLang := localize.Language
	originalColor := localize.UseColor
	localize.SetLanguage(localize.LangEN)
	localize.UseColor = false
	defer func() {
		localize.SetLanguage(originalLang)
		localize.UseColor = originalColor
	}()

	rendered := Format(nil, errors.New(`unknown command "wat"`))
	if !strings.Contains(rendered, "rpl help") {
		t.Fatalf("expected generic command hint, got:\n%s", rendered)
	}
}
