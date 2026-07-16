package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"sort"
	"strconv"
	"strings"
)

func generateStd(req sdk.GenerateRequest) (sdk.GenerateResponse, error) {
	builder := sdk.NewCodeBuilder()

	fieldMetaTypeName := req.Model.Name + "StdFieldMeta"
	methodMetaTypeName := req.Model.Name + "StdMethodMeta"

	builder.AddOrderedBlock("std.types.method", generateStdMethodMetaType(methodMetaTypeName), 0)
	builder.AddOrderedBlock("std.types.field", generateStdFieldMetaType(fieldMetaTypeName, methodMetaTypeName), 1)
	builder.AddOrderedBlock("std.comment", generateStdCommentMethod(req.Model), 10)
	builder.AddOrderedBlock("std.group", generateStdGroupMethod(req.Model), 20)
	builder.AddOrderedBlock("std.fields", generateStdFieldsMethod(req.Model, fieldMetaTypeName, methodMetaTypeName, req.File), 30)
	builder.AddOrderedBlock("std.field", generateStdFieldLookupMethod(req.Model, fieldMetaTypeName), 40)

	body, err := sdk.RenderGoFile(req.File.PackageName, builder.Response())
	if err != nil {
		return sdk.GenerateResponse{}, err
	}

	return sdk.GenerateResponse{
		Files: []sdk.GeneratedFile{{
			Path:    "meta.gen.go",
			Content: string(body),
		}},
	}, nil
}

func generateStdMethodMetaType(typeName string) string {
	return sdk.WithDocComment(
		fmt.Sprintf("type %s struct {\n\tName string\n\tParams []string\n\tReturns []string\n\tInside bool\n}", typeName),
		"%s описывает метод поля в std-метаданных.",
		"%s describes a field method in std metadata.",
		typeName,
	)
}

func generateStdFieldMetaType(typeName string, methodMetaTypeName string) string {
	return sdk.WithDocComment(
		fmt.Sprintf("type %s struct {\n\tName string\n\tType string\n\tKind string\n\tOptional bool\n\tRepeated bool\n\tGroup string\n\tModel string\n\tExternal bool\n\tIgnoredBy []string\n\tMethods []%s\n}", typeName, methodMetaTypeName),
		"%s описывает поле модели и его стандартные метаданные.",
		"%s describes a model field and its standard metadata.",
		typeName,
	)
}

func generateStdCommentMethod(model sdk.Model) string {
	return sdk.WithDocComment(
		fmt.Sprintf("func (model %s) StdComment() string {\n\treturn %s\n}", model.Name, strconv.Quote(strings.TrimSpace(model.Comment()))),
		"StdComment возвращает комментарий модели %s из attr `@comment`.",
		"StdComment returns model %s comment from the `@comment` attr.",
		model.Name,
	)
}

func generateStdGroupMethod(model sdk.Model) string {
	return sdk.WithDocComment(
		fmt.Sprintf("func (model %s) StdGroup() string {\n\treturn %s\n}", model.Name, strconv.Quote(strings.TrimSpace(model.Group()))),
		"StdGroup возвращает группу модели %s, если она задана.",
		"StdGroup returns the group of model %s when it is set.",
		model.Name,
	)
}

func generateStdFieldsMethod(model sdk.Model, fieldMetaTypeName string, methodMetaTypeName string, file sdk.FileContext) string {
	items := make([]string, 0, len(model.Fields))
	for _, field := range model.Fields {
		items = append(items, renderStdFieldMeta(field, fieldMetaTypeName, methodMetaTypeName, file))
	}

	body := ""
	if len(items) > 0 {
		body = strings.Join(items, ",\n\t\t") + ","
	}

	return sdk.WithDocComment(
		fmt.Sprintf("func (model %s) StdFields() []%s {\n\treturn []%s{\n\t\t%s\n\t}\n}", model.Name, fieldMetaTypeName, fieldMetaTypeName, body),
		"StdFields возвращает полное std-описание полей модели %s.",
		"StdFields returns the full std field description for model %s.",
		model.Name,
	)
}

func generateStdFieldLookupMethod(model sdk.Model, fieldMetaTypeName string) string {
	return sdk.WithDocComment(
		fmt.Sprintf("func (model %s) StdField(name string) (%s, bool) {\n\tfor _, field := range model.StdFields() {\n\t\tif field.Name == name {\n\t\t\treturn field, true\n\t\t}\n\t}\n\n\treturn %s{}, false\n}", model.Name, fieldMetaTypeName, fieldMetaTypeName),
		"StdField ищет std-метаданные конкретного поля модели %s по имени.",
		"StdField looks up std metadata for a named field on model %s.",
		model.Name,
	)
}

func renderStdFieldMeta(field sdk.Field, fieldMetaTypeName string, methodMetaTypeName string, file sdk.FileContext) string {
	modelName := ""
	if ref, ok := field.RefModel(file); ok && ref != nil {
		modelName = ref.Name
	}

	return fmt.Sprintf("%s{\n\t\t\tName: %s,\n\t\t\tType: %s,\n\t\t\tKind: %s,\n\t\t\tOptional: %t,\n\t\t\tRepeated: %t,\n\t\t\tGroup: %s,\n\t\t\tModel: %s,\n\t\t\tExternal: %t,\n\t\t\tIgnoredBy: %s,\n\t\t\tMethods: %s,\n\t\t}",
		fieldMetaTypeName,
		strconv.Quote(field.Name),
		strconv.Quote(field.GoType()),
		strconv.Quote(string(field.Type.Kind())),
		field.Type.Optional,
		field.Type.IsList,
		strconv.Quote(strings.TrimSpace(field.Group())),
		strconv.Quote(modelName),
		field.Type.IsExternal(file),
		renderStringSlice(field.IgnoredTargets()),
		renderMethodMetaSlice(field.Methods, methodMetaTypeName),
	)
}

func renderMethodMetaSlice(methods []sdk.Method, methodMetaTypeName string) string {
	if len(methods) == 0 {
		return "nil"
	}

	items := make([]string, 0, len(methods))
	for _, method := range methods {
		items = append(items, fmt.Sprintf("%s{Name: %s, Params: %s, Returns: %s, Inside: %t}",
			methodMetaTypeName,
			strconv.Quote(method.Name),
			renderParamTypeSlice(method.Params),
			renderTypeRefSlice(method.Returns),
			strings.EqualFold(strings.TrimSpace(method.Mode("grpc")), "inside"),
		))
	}

	return "[]" + methodMetaTypeName + "{" + strings.Join(items, ", ") + "}"
}

func renderParamTypeSlice(params []sdk.MethodParam) string {
	if len(params) == 0 {
		return "nil"
	}

	items := make([]string, 0, len(params))
	for _, param := range params {
		if strings.TrimSpace(param.Name) != "" {
			items = append(items, param.Name+" "+param.Type.GoString())
			continue
		}
		items = append(items, param.Type.GoString())
	}

	return renderStringSlicePreserveOrder(items)
}

func renderTypeRefSlice(types []sdk.TypeRef) string {
	if len(types) == 0 {
		return "nil"
	}

	items := make([]string, 0, len(types))
	for _, item := range types {
		items = append(items, item.GoString())
	}

	return renderStringSlice(items)
}

func renderStringSlice(items []string) string {
	if len(items) == 0 {
		return "nil"
	}

	normalized := append([]string(nil), items...)
	sort.Strings(normalized)

	parts := make([]string, 0, len(normalized))
	for _, item := range normalized {
		parts = append(parts, strconv.Quote(item))
	}

	return "[]string{" + strings.Join(parts, ", ") + "}"
}

func renderStringSlicePreserveOrder(items []string) string {
	if len(items) == 0 {
		return "nil"
	}

	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, strconv.Quote(item))
	}

	return "[]string{" + strings.Join(parts, ", ") + "}"
}
