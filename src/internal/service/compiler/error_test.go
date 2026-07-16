package compiler

import (
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"strings"
	"testing"
)

func TestRunPrettySyntaxErrorContainsHintAndCaret(t *testing.T) {
	originalLang := localize.Language
	originalColor := localize.UseColor
	localize.SetLanguage(localize.LangEN)
	localize.UseColor = false
	defer func() {
		localize.SetLanguage(originalLang)
		localize.UseColor = originalColor
	}()

	service := New()
	_, err := service.Run("model User {\n    Name string =\n}\n")
	if err == nil {
		t.Fatal("expected syntax error")
	}

	rendered := rplerr.Format(nil, err)
	if !strings.Contains(rendered, "unexpected expression token") {
		t.Fatalf("expected syntax message, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "^") {
		t.Fatalf("expected caret in rendered error, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Expressions here usually use") {
		t.Fatalf("expected helpful hint in rendered error, got:\n%s", rendered)
	}
}
