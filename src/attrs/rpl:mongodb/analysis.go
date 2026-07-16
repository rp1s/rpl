package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strings"
)

var mongodbModelSpec = sdk.AttrSpec{
	Namespace: "mongodb",
	Help: sdk.Text(
		"На уровне модели mongodb понимает db и collection.",
		"At model level mongodb understands db and collection.",
	),
	Args: []sdk.AttrArgSpec{
		{Name: "db", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "collection", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@mongodb", Insert: "@mongodb", Help: sdk.Text("Базовый MongoDB-атрибут.", "Base MongoDB attr.")},
	},
}

var mongodbFieldSpec = sdk.AttrSpec{
	Namespace: "mongodb",
	Help: sdk.Text(
		"На уровне поля mongodb понимает name, index, unique, sparse, search, sort, objectId, omitempty, default, updatedAt и ignore.",
		"At field level mongodb understands name, index, unique, sparse, search, sort, objectId, omitempty, default, updatedAt, and ignore.",
	),
	Args: []sdk.AttrArgSpec{
		{Name: "name", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "index", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "unique", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "sparse", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "search", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "sort", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "objectId", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "omitempty", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "default", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "updatedAt", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "ignore", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool, sdk.AttrValueTypeStringLike}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@mongodb", Insert: "@mongodb", Help: sdk.Text("Базовый MongoDB-атрибут поля.", "Base MongoDB field attr.")},
	},
}

func analyzeMongoDB(req sdk.GenerateRequest) (sdk.AnalyzeResponse, error) {
	builder := sdk.NewAnalyzeBuilder()

	modelResolved := builder.ValidateAttrSpec(req.Model.RuntimeAttrs, mongodbModelSpec)
	validateMongoDBDatabase(builder, modelRuntimeAttr(req.Model, "mongodb"), modelResolved.ValueMap())

	for _, field := range req.Model.Fields {
		analyzeMongoDBField(builder, field)
		for _, method := range field.Methods {
			for _, attr := range method.RuntimeAttrs {
				builder.AddDiagnostic(sdk.DiagnosticAt(
					attr,
					fmt.Sprintf(sdk.Text("attr %q нельзя использовать на методе поля %q", "attr %q cannot be used on field method %q"), attr.Identifier, method.Name),
					sdk.Text("MongoDB attrs описывают модель и поля хранения, а не методы поля.", "MongoDB attrs describe storage metadata on models and fields, not field methods."),
				))
			}
		}
	}

	for _, field := range req.Model.ActiveFields("mongodb") {
		builder.AddClaim("field.domain", "storage", req.Model.Name+"."+field.Name)
	}

	generated, err := generateMongoDB(req)
	if err != nil {
		return sdk.AnalyzeResponse{}, err
	}
	sdk.AddGeneratedClaimsInScope(builder, generated, packageScope(req.File, "mongodb"))
	return builder.Response(), nil
}

func validateMongoDBDatabase(builder *sdk.AnalyzeBuilder, attr sdk.Attr, values map[string]sdk.Value) {
	if builder == nil {
		return
	}

	raw := strings.TrimSpace(values["db"].String())
	if raw == "" {
		return
	}

	switch strings.ToLower(raw) {
	case "mongo", "mongodb":
		return
	default:
		builder.AddDiagnostic(sdk.DiagnosticAt(
			attr,
			fmt.Sprintf(sdk.Text("неподдерживаемый mongodb db %q", "unsupported mongodb db %q"), raw),
			sdk.Text("Используйте `mongodb` или `mongo`, если хотите явно указать движок.", "Use `mongodb` or `mongo` when you want to explicitly name the engine."),
		))
	}
}

func analyzeMongoDBField(builder *sdk.AnalyzeBuilder, field sdk.Field) {
	resolved := builder.ValidateAttrSpec(field.RuntimeAttrs, mongodbFieldSpec)
	values := resolved.ValueMap()

	if values["objectId"].BoolValue() && !field.Type.IsString() {
		builder.AddDiagnostic(sdk.DiagnosticAt(
			fieldRuntimeAttr(field, "mongodb"),
			fmt.Sprintf(sdk.Text("mongodb(objectId: true) можно ставить только на string, а не на %q", "mongodb(objectId: true) can only be used on string, not %q"), field.Type.Name),
			sdk.Text("Обычно это поле выглядит как `ID string @mongodb(objectId: true)`.", "A typical field looks like `ID string @mongodb(objectId: true)`."),
		))
	}

	if values["updatedAt"].BoolValue() && !field.Type.IsTime() {
		builder.AddDiagnostic(sdk.DiagnosticAt(
			fieldRuntimeAttr(field, "mongodb"),
			fmt.Sprintf(sdk.Text("mongodb(updatedAt: true) можно ставить только на time.Time, а не на %q", "mongodb(updatedAt: true) can only be used on time.Time, not %q"), field.Type.Name),
			sdk.Text("Обычно это поле выглядит как `UpdatedAt time.Time @mongodb(default: \"now\", updatedAt: true)`.", "A typical field looks like `UpdatedAt time.Time @mongodb(default: \"now\", updatedAt: true)`."),
		))
	}

	if field.IgnoredBy("mongodb") && hasMeaningfulMongoDBConfig(values) {
		builder.AddDiagnostic(sdk.DiagnosticAt(
			fieldRuntimeAttr(field, "mongodb"),
			fmt.Sprintf(sdk.Text("поле %q одновременно игнорирует и настраивает mongodb", "field %q both ignores and configures mongodb"), field.Name),
			sdk.Text("Если поле нужно исключить из mongodb, уберите остальные mongodb-аргументы.", "If the field should be excluded from mongodb, remove the rest of the mongodb arguments."),
		))
	}
}

func hasMeaningfulMongoDBConfig(values map[string]sdk.Value) bool {
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

func modelRuntimeAttr(model sdk.Model, name string) sdk.Attr {
	attr, _ := model.ResolvedAttr(name)
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
