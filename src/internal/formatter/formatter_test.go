package formatter

import (
	"strings"
	"testing"
)

func TestFormatPreservesLineComments(t *testing.T) {
	source := `target(lang: golang)

// модель пользователя
model User {
	// имя
	Name string
}
`

	formatted, err := Format(source, "")
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	if formatted != source {
		t.Fatalf("expected formatter to preserve commented source\nwant:\n%s\ngot:\n%s", source, formatted)
	}
}

func TestFormatPreservesFieldAttributeLayout(t *testing.T) {
	source := `target(lang: golang)

attrs (
	"rpl:sql"
)

model CachedCliText {
	Lang string {
		@sql(index: true)
		@sql(primaryKey: true)
	}
	CreatedAt time.Time @sql(default: "now")
}
`

	formatted, err := Format(source, "")
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	for _, expected := range []string{
		"\tLang string\n\t{\n\t\t@sql(index: true)\n\t\t@sql(primaryKey: true)\n\t}",
		"\tCreatedAt time.Time @sql(default: \"now\")",
	} {
		if !strings.Contains(formatted, expected) {
			t.Fatalf("formatted output does not preserve %q:\n%s", expected, formatted)
		}
	}
}
