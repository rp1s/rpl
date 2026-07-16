package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatPreservesTopLevelFieldMethodExtension(t *testing.T) {
	service := New()

	formatted, err := service.Format(`target(lang: golang)
model User {
Name string = "x" {
@validate(min: 1, max: 32)
}
}
func User.Name {
func Ping() return (User.Name)
}`, "schema.rpl")
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	for _, want := range []string{
		"target(lang: golang)",
		"model User {",
		"\tName string = \"x\"",
		"func User.Name {",
		"\tfunc Ping return (User.Name)",
	} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatted output does not contain %q\n%s", want, formatted)
		}
	}
}

func TestFormatPreservesTopLevelModelMethodExtension(t *testing.T) {
	service := New()

	formatted, err := service.Format(`target(lang: golang)
model User {
Name string = "x"
}
func User {
func String return (string)
}`, "schema.rpl")
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	for _, want := range []string{
		"target(lang: golang)",
		"model User {",
		"\tName string = \"x\"",
		"func User {",
		"\tfunc String return (string)",
	} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatted output does not contain %q\n%s", want, formatted)
		}
	}
}

func TestFormatUsesExplicitEmptyParensForMarkerAttrs(t *testing.T) {
	service := New()

	formatted, err := service.Format(`target(lang: golang)

attrs (
    "rpl:grpc",
    "rpl:validate"
)

@grpc
model User {
    Phone string
    {
        @validate(phone)
    }
}`, "schema.rpl")
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	if !strings.Contains(formatted, "@grpc()\nmodel User") {
		t.Fatalf("expected formatter to render @grpc() explicitly, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "@validate(phone)") {
		t.Fatalf("expected shorthand attr to survive formatting, got:\n%s", formatted)
	}
}

func TestFormatPreservesPackageDirective(t *testing.T) {
	service := New()

	formatted, err := service.Format(`package user
target(lang: golang)
model User {
Name string
}`, "schema.rpl")
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	if !strings.HasPrefix(formatted, "package user\n\n") {
		t.Fatalf("expected formatted output to start with package directive, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "target(lang: golang)") {
		t.Fatalf("expected target to survive formatting, got:\n%s", formatted)
	}
}

func TestFormatPreservesRawGoDefaultExpr(t *testing.T) {
	service := New()

	formatted, err := service.Format(`target(lang: golang)
import (
    "math/bits"
)
model User {
Size int = bits.UintSize
Tags []string = []string{"a", "b"}
}`, "schema.rpl")
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	if !strings.Contains(formatted, "Size int = bits.UintSize") {
		t.Fatalf("expected formatter to preserve imported Go default expression, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, `Tags []string = []string{"a", "b"}`) {
		t.Fatalf("expected formatter to preserve composite literal default expression, got:\n%s", formatted)
	}
}

func TestFormatRemovesDuplicatePackageTargetFromNonOwnerFile(t *testing.T) {
	service := New()
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "a.rpl")
	secondPath := filepath.Join(dir, "b.rpl")

	if err := os.WriteFile(firstPath, []byte(`package user

target(lang: golang)

model User {
	Id int
}
`), 0o644); err != nil {
		t.Fatalf("write first file: %v", err)
	}

	formatted, err := service.Format(`package user

target(lang: golang)

model User2 {
	Id User.Id
}
`, secondPath)
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	if strings.Contains(formatted, "target(lang: golang)") {
		t.Fatalf("expected formatter to remove duplicate package target from the non-owner file, got:\n%s", formatted)
	}
	if !strings.HasPrefix(formatted, "package user\n\nmodel User2") {
		t.Fatalf("expected package directive to stay at the top after target removal, got:\n%s", formatted)
	}
}

func TestFormatRemovesDuplicatePackageTargetAndKeepsComments(t *testing.T) {
	service := New()
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "a.rpl")
	secondPath := filepath.Join(dir, "b.rpl")

	if err := os.WriteFile(firstPath, []byte(`package user

target(lang: golang)

model User {
	Id int
}
`), 0o644); err != nil {
		t.Fatalf("write first file: %v", err)
	}

	formatted, err := service.Format(`package user

target(lang: golang)

// комментарий должен остаться
model User2 {
	Id int
}
`, secondPath)
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	if strings.Contains(formatted, "target(lang: golang)") {
		t.Fatalf("expected formatter to remove duplicate package target even in commented files, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "// комментарий должен остаться") {
		t.Fatalf("expected formatter to preserve comments, got:\n%s", formatted)
	}
}
