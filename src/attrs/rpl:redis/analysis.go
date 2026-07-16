package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strings"
)

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
	Help:      sdk.Text("На уровне поля redis понимает unique, default и ignore.", "At field level redis understands unique, default, and ignore."),
	Args: []sdk.AttrArgSpec{
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

	for _, field := range req.Model.Fields {
		analyzeRedisField(builder, field)
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

	if field.IgnoredBy("redis") && hasMeaningfulRedisConfig(values) {
		builder.AddDiagnostic(sdk.DiagnosticAt(
			fieldRuntimeAttr(field, "redis"),
			fmt.Sprintf(sdk.Text("поле %q одновременно игнорирует и настраивает redis", "field %q both ignores and configures redis"), field.Name),
			sdk.Text("Если поле нужно исключить из redis, уберите остальные redis-аргументы.", "If the field should be ignored by redis, remove the rest of the redis arguments."),
		))
	}
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
