package main

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"rpl/pkg/sdk"
	"strconv"
	"strings"
	"time"
)

var redisHashNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

var redisModelSpec = sdk.AttrSpec{
	Namespace: "redis",
	Help:      sdk.Text("На уровне модели redis понимает db, table и ttl.", "At model level redis understands db, table, and ttl."),
	Args: []sdk.AttrArgSpec{
		{Name: "db", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "table", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "ttl", Types: []sdk.AttrValueType{sdk.AttrValueTypeNumber}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@redis", Insert: "@redis", Help: sdk.Text("Базовый Redis-атрибут.", "Base Redis attr.")},
	},
}

var redisFieldSpec = sdk.AttrSpec{
	Namespace: "redis",
	Help:      sdk.Text("На уровне поля redis понимает name, unique, default и ignore.", "At field level redis understands name, unique, default, and ignore."),
	Args: []sdk.AttrArgSpec{
		{Name: "name", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "unique", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "default", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "ignore", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool, sdk.AttrValueTypeStringLike}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@redis", Insert: "@redis", Help: sdk.Text("Базовый Redis-атрибут поля.", "Base Redis field attr.")},
	},
}

func analyzeRedis(req sdk.GenerateRequest) (sdk.AnalyzeResponse, error) {
	builder := sdk.NewAnalyzeBuilder()

	builder.ValidateAttrSpec(req.Model.RuntimeAttrs, redisModelSpec)

	hashNames := make(map[string]string)
	for _, field := range req.Model.Fields {
		analyzeRedisField(builder, field)
		if !field.IgnoredBy("redis") {
			name := redisHashName(field)
			if previous, exists := hashNames[name]; exists {
				builder.AddDiagnostic(sdk.DiagnosticAt(
					fieldRuntimeAttr(field, "redis"),
					fmt.Sprintf(sdk.Text("redis hash name %q используется полями %q и %q", "Redis hash name %q is used by fields %q and %q"), name, previous, field.Name),
					sdk.Text("Задайте уникальное имя через `@redis(name: \"...\")`.", "Choose a unique name with `@redis(name: \"...\")`."),
				))
			} else {
				hashNames[name] = field.Name
			}
		}
		for _, method := range field.Methods {
			for _, attr := range method.RuntimeAttrs {
				builder.AddDiagnostic(sdk.DiagnosticAt(
					attr,
					fmt.Sprintf(sdk.Text("attr %q нельзя использовать на методе поля %q", "attr %q cannot be used on field method %q"), attr.Identifier, method.Name),
					sdk.Text("Redis attrs описывают модель и поля хранения, а не методы поля.", "Redis attrs describe storage metadata on models and fields, not field methods."),
				))
			}
		}
	}

	for _, field := range req.Model.ActiveFields("redis") {
		builder.AddClaim("field.domain", "storage", req.Model.Name+"."+field.Name)
	}

	sdk.AddGeneratedClaimsInScope(builder, generateRedis(req), packageScope(req.File))
	return builder.Response(), nil
}

func analyzeRedisField(builder *sdk.AnalyzeBuilder, field sdk.Field) {
	resolved := builder.ValidateAttrSpec(field.RuntimeAttrs, redisFieldSpec)
	values := resolved.ValueMap()
	if name := strings.TrimSpace(values["name"].String()); name != "" && !redisHashNamePattern.MatchString(name) {
		builder.AddDiagnostic(sdk.DiagnosticAt(
			fieldRuntimeAttr(field, "redis"),
			fmt.Sprintf(sdk.Text("некорректное Redis hash name %q", "invalid Redis hash name %q"), name),
			sdk.Text("Используйте буквы, цифры, `_`, `-` или `.`; первый символ должен быть буквой или `_`.", "Use letters, digits, `_`, `-`, or `.`; the first character must be a letter or `_`."),
		))
	}
	validateRedisDefault(builder, field, values["default"])

	if field.IgnoredBy("redis") && hasMeaningfulRedisConfig(values) {
		builder.AddDiagnostic(sdk.DiagnosticAt(
			fieldRuntimeAttr(field, "redis"),
			fmt.Sprintf(sdk.Text("поле %q одновременно игнорирует и настраивает redis", "field %q both ignores and configures redis"), field.Name),
			sdk.Text("Если поле нужно исключить из redis, уберите остальные redis-аргументы.", "If the field should be ignored by redis, remove the rest of the redis arguments."),
		))
	}
}

func validateRedisDefault(builder *sdk.AnalyzeBuilder, field sdk.Field, value sdk.Value) {
	raw := strings.TrimSpace(value.String())
	if raw == "" {
		return
	}
	valid := true
	switch {
	case field.Type.IsList:
		valid = json.Valid([]byte(raw))
	case field.Type.IsString():
		return
	case field.Type.IsBool():
		_, err := strconv.ParseBool(raw)
		valid = err == nil
	case field.Type.IsInteger():
		bits := redisIntegerBits(field.Type.BaseName())
		if strings.HasPrefix(field.Type.BaseName(), "u") || field.Type.BaseName() == "byte" {
			_, err := strconv.ParseUint(raw, 10, bits)
			valid = err == nil
		} else {
			_, err := strconv.ParseInt(raw, 10, bits)
			valid = err == nil
		}
	case field.Type.IsFloat():
		bits := 64
		if field.Type.BaseName() == "float32" {
			bits = 32
		}
		parsed, err := strconv.ParseFloat(raw, bits)
		valid = err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
	case field.Type.IsTime():
		if !strings.EqualFold(raw, "now") {
			_, err := time.Parse(time.RFC3339, raw)
			valid = err == nil
		}
	default:
		valid = json.Valid([]byte(raw))
	}
	if !valid {
		builder.AddDiagnostic(sdk.DiagnosticAt(
			fieldRuntimeAttr(field, "redis"),
			fmt.Sprintf(sdk.Text("default %q несовместим с Redis-полем %q", "default %q is incompatible with Redis field %q"), raw, field.Name),
			sdk.Text("Используйте scalar, RFC3339/now для времени или JSON для списков и моделей.", "Use a scalar, RFC3339/now for time, or JSON for lists and models."),
		))
	}
}

func redisIntegerBits(name string) int {
	switch name {
	case "int8", "uint8", "byte":
		return 8
	case "int16", "uint16":
		return 16
	case "int32", "uint32":
		return 32
	default:
		return 64
	}
}

func redisHashName(field sdk.Field) string {
	if name := strings.TrimSpace(field.ResolvedValues("redis")["name"].String()); name != "" {
		return name
	}
	return sdk.SnakeCase(field.Name)
}

func hasMeaningfulRedisConfig(values map[string]sdk.Value) bool {
	for name := range values {
		if name != "ignore" {
			return true
		}
	}
	return false
}

func fieldRuntimeAttr(field sdk.Field, name string) sdk.Attr {
	attr, _ := field.ResolvedAttr(name)
	if len(attr.Attrs) > 0 {
		return attr.Attrs[0]
	}
	return sdk.Attr{}
}

func packageScope(file sdk.FileContext, parts ...string) string {
	base := strings.TrimSpace(file.GoPackagePath)
	if base == "" {
		base = strings.TrimSpace(file.PackageName)
	}
	items := make([]string, 0, len(parts)+1)
	if base != "" {
		items = append(items, base)
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return strings.Join(items, "/")
}
