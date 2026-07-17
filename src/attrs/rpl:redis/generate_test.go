package main

import (
	"rpl/pkg/sdk"
	"strings"
	"testing"
)

func TestRedisCustomHashNameAndDefault(t *testing.T) {
	field := sdk.Field{
		Name: "Enabled",
		Type: sdk.TypeRef{Name: "bool"},
		RuntimeAttrs: []sdk.Attr{{
			Identifier: "redis",
			NamedArgs: []sdk.NamedValue{
				{Name: "name", Value: sdk.Value{Kind: "string", Text: "is_enabled"}},
				{Name: "default", Value: sdk.Value{Kind: "string", Text: "true"}},
			},
		}},
	}

	if got := redisHashName(field); got != "is_enabled" {
		t.Fatalf("redisHashName() = %q", got)
	}
	code, _ := redisDefaultCode(field)
	for _, want := range []string{`values["is_enabled"]`, "model.Enabled = true"} {
		if !strings.Contains(code, want) {
			t.Fatalf("default code does not contain %q:\n%s", want, code)
		}
	}
}

func TestRedisListDefaultUsesJSON(t *testing.T) {
	field := sdk.Field{
		Name: "Scopes",
		Type: sdk.TypeRef{Name: "string", IsList: true},
		RuntimeAttrs: []sdk.Attr{{
			Name:      "redis",
			NamedArgs: []sdk.NamedValue{{Name: "default", Value: sdk.Value{Kind: "string", Text: `["read","write"]`}}},
		}},
	}

	code, imports := redisDefaultCode(field)
	if !imports["encoding/json"] || !strings.Contains(code, "json.Unmarshal") {
		t.Fatalf("list default must use JSON decoding: %s", code)
	}
	if strings.Contains(code, `model.Scopes = \"`) {
		t.Fatalf("list default was emitted as a scalar string: %s", code)
	}
}
