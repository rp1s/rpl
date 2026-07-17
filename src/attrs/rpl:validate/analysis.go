package main

import (
	"fmt"
	"regexp"
	"rpl/pkg/sdk"
	"strings"
)

var validateFieldSpec = sdk.AttrSpec{
	Namespace: "validate",
	Help:      sdk.Text("Разрешены: required, min, max, minLen, maxLen, email, phone, url, uuid, pattern, past, hash.", "Allowed args are: required, min, max, minLen, maxLen, email, phone, url, uuid, pattern, past, hash."),
	Args: []sdk.AttrArgSpec{
		{Name: "required", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "min", Types: []sdk.AttrValueType{sdk.AttrValueTypeNumber}},
		{Name: "max", Types: []sdk.AttrValueType{sdk.AttrValueTypeNumber}},
		{Name: "minLen", Types: []sdk.AttrValueType{sdk.AttrValueTypeNumber}},
		{Name: "maxLen", Types: []sdk.AttrValueType{sdk.AttrValueTypeNumber}},
		{Name: "email", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "phone", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "url", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "uuid", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "pattern", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "past", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "hash", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@validate", Insert: "@validate", Help: sdk.Text("Базовый validate-атрибут.", "Base validate attr.")},
	},
}

func analyzeValidate(req sdk.GenerateRequest) (sdk.AnalyzeResponse, error) {
	builder := sdk.NewAnalyzeBuilder()

	for _, attr := range req.Model.RuntimeAttrs {
		builder.AddDiagnostic(sdk.DiagnosticAt(
			attr,
			fmt.Sprintf(sdk.Text("attr %q нельзя использовать на модели", "attr %q cannot be used on a model"), attr.Identifier),
			sdk.Text("Используйте validate-атрибуты только на полях модели.", "Use validate attrs only on model fields."),
		))
	}

	for _, field := range req.Model.Fields {
		analyzeValidateField(builder, field)
		for _, method := range field.Methods {
			for _, attr := range method.RuntimeAttrs {
				builder.AddDiagnostic(sdk.DiagnosticAt(
					attr,
					fmt.Sprintf(sdk.Text("attr %q нельзя использовать на методе поля %q", "attr %q cannot be used on field method %q"), attr.Identifier, method.Name),
					sdk.Text("Validate attrs описывают только данные поля, а не методы inside/gRPC.", "Validate attrs describe field data only, not inside/gRPC methods."),
				))
			}
		}
	}

	sdk.AddGeneratedClaimsInScope(builder, generateValidate(req), packageScope(req.File, "validation"))
	return builder.Response(), nil
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

func analyzeValidateField(builder *sdk.AnalyzeBuilder, field sdk.Field) {
	if builder == nil {
		return
	}

	resolved := builder.ValidateAttrSpec(field.RuntimeAttrs, validateFieldSpec)
	values := resolved.ValueMap()
	if values["required"].BoolValue() && !validateSupportsRequired(field.Type) {
		builder.AddDiagnostic(sdk.IncompatibleAttrType(fieldRuntimeAttr(field, "validate"), "validate", "required", field.Type, sdk.Text("Используйте optional-тип, строку, список или time.Time.", "Use an optional type, string, list, or time.Time.")))
	}

	if min, ok := values["min"]; ok {
		if !field.Type.IsString() && !field.Type.IsNumeric() {
			builder.AddDiagnostic(sdk.IncompatibleAttrType(
				fieldRuntimeAttr(field, "validate"),
				"validate",
				"min",
				field.Type,
				"",
			))
		}
		if max, ok := values["max"]; ok {
			minValue, minErr := min.Float64()
			maxValue, maxErr := max.Float64()
			if minErr == nil && maxErr == nil && minValue > maxValue {
				builder.AddDiagnostic(sdk.Diagnostic{
					Message: fmt.Sprintf(sdk.Text("validate(min: ...) больше validate(max: ...) у поля %q", "validate(min: ...) is greater than validate(max: ...) on field %q"), field.Name),
				})
			}
		}
	}
	if _, ok := values["max"]; ok && !field.Type.IsString() && !field.Type.IsNumeric() {
		builder.AddDiagnostic(sdk.IncompatibleAttrType(fieldRuntimeAttr(field, "validate"), "validate", "max", field.Type, ""))
	}
	if minLen, ok := values["minLen"]; ok {
		if !validateSupportsLen(field.Type) {
			builder.AddDiagnostic(sdk.IncompatibleAttrType(fieldRuntimeAttr(field, "validate"), "validate", "minLen", field.Type, ""))
		}
		if maxLen, ok := values["maxLen"]; ok {
			minValue, minErr := minLen.Float64()
			maxValue, maxErr := maxLen.Float64()
			if minErr == nil && maxErr == nil && minValue > maxValue {
				builder.AddDiagnostic(sdk.Diagnostic{
					Message: fmt.Sprintf(sdk.Text("validate(minLen: ...) больше validate(maxLen: ...) у поля %q", "validate(minLen: ...) is greater than validate(maxLen: ...) on field %q"), field.Name),
				})
			}
		}
	}
	if _, ok := values["maxLen"]; ok && !validateSupportsLen(field.Type) {
		builder.AddDiagnostic(sdk.IncompatibleAttrType(fieldRuntimeAttr(field, "validate"), "validate", "maxLen", field.Type, ""))
	}
	if values["email"].BoolValue() && !validateSupportsString(field.Type) {
		builder.AddDiagnostic(sdk.IncompatibleAttrType(fieldRuntimeAttr(field, "validate"), "validate", "email", field.Type, ""))
	}
	if values["phone"].BoolValue() && !validateSupportsString(field.Type) {
		builder.AddDiagnostic(sdk.IncompatibleAttrType(fieldRuntimeAttr(field, "validate"), "validate", "phone", field.Type, ""))
	}
	if values["url"].BoolValue() && !validateSupportsString(field.Type) {
		builder.AddDiagnostic(sdk.IncompatibleAttrType(fieldRuntimeAttr(field, "validate"), "validate", "url", field.Type, ""))
	}
	if values["uuid"].BoolValue() && !validateSupportsString(field.Type) {
		builder.AddDiagnostic(sdk.IncompatibleAttrType(fieldRuntimeAttr(field, "validate"), "validate", "uuid", field.Type, ""))
	}
	if pattern, ok := values["pattern"]; ok {
		if !validateSupportsString(field.Type) {
			builder.AddDiagnostic(sdk.IncompatibleAttrType(fieldRuntimeAttr(field, "validate"), "validate", "pattern", field.Type, ""))
		} else if _, err := regexp.Compile(pattern.String()); err != nil {
			builder.AddDiagnostic(sdk.DiagnosticAt(
				fieldRuntimeAttr(field, "validate"),
				fmt.Sprintf(sdk.Text("validate pattern у поля %q не компилируется: %v", "validate pattern on field %q does not compile: %v"), field.Name, err),
				sdk.Text("Исправьте регулярное выражение в `pattern`.", "Fix the regular expression in `pattern`."),
			))
		}
	}
	if values["past"].BoolValue() && !field.Type.IsTime() {
		builder.AddDiagnostic(sdk.DiagnosticAt(
			fieldRuntimeAttr(field, "validate"),
			fmt.Sprintf(sdk.Text("validate(past: true) требует time.Time, а не %q", "validate(past: true) requires time.Time, not %q"), field.Type.Name),
			"",
		))
	}
	if _, ok := values["hash"]; ok && (!field.Type.IsString() || field.Type.IsList) {
		builder.AddDiagnostic(sdk.DiagnosticAt(
			fieldRuntimeAttr(field, "validate"),
			fmt.Sprintf(sdk.Text("validate(hash: ...) требует string-поле, а не %q", "validate(hash: ...) requires a string field, not %q"), field.Type.Name),
			"",
		))
	}
}

func fieldRuntimeAttr(field sdk.Field, name string) sdk.Attr {
	attr, _ := field.ResolvedAttr(name)
	if len(attr.Attrs) > 0 {
		return attr.Attrs[0]
	}
	return sdk.Attr{}
}

func validateSupportsLen(typeRef sdk.TypeRef) bool {
	return typeRef.IsList || typeRef.IsString()
}

func validateSupportsString(typeRef sdk.TypeRef) bool {
	return typeRef.IsString() || (typeRef.IsList && typeRef.BaseName() == "string")
}

func validateSupportsRequired(typeRef sdk.TypeRef) bool {
	return typeRef.Optional || typeRef.IsString() || typeRef.IsList || typeRef.IsTime()
}
