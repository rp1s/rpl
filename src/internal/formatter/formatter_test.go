package formatter

import "testing"

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
