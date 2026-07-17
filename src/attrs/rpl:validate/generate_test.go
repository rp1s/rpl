package main

import (
	"rpl/pkg/sdk"
	"strings"
	"testing"
)

func TestValidationCodeSupportsRequiredUUIDAndPattern(t *testing.T) {
	field := sdk.Field{
		Name: "Slug",
		Type: sdk.TypeRef{Name: "string"},
		RuntimeAttrs: []sdk.Attr{{
			Identifier: "validate",
			NamedArgs: []sdk.NamedValue{
				{Name: "required", Value: sdk.Value{Kind: "bool", Bool: true}},
				{Name: "uuid", Value: sdk.Value{Kind: "bool", Bool: true}},
				{Name: "pattern", Value: sdk.Value{Kind: "string", Text: `^[a-z-]+$`}},
			},
		}},
	}

	lines, flags := validationCodeForField(field, "phonePattern")
	generated := strings.Join(lines, "\n")
	for _, want := range []string{"strings.TrimSpace(model.Slug)", "regexp.MatchString", "must be uuid", "has invalid format"} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated validation does not contain %q:\n%s", want, generated)
		}
	}
	if !flags["strings"] || !flags["regexp"] {
		t.Fatalf("missing generated import flags: %#v", flags)
	}
}
