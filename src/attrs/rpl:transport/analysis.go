package main

import (
	"fmt"
	"path/filepath"
	"rpl/pkg/sdk"
	"strings"
)

var transportModelSpec = sdk.AttrSpec{
	Namespace: "transport",
	Help:      localize.Text("Transport генерирует локальный process transport через stdin/stdout и shell client/server для модели.", "Transport generates a local process transport over stdin/stdout together with shell client/server helpers for the model."),
	Args: []sdk.AttrArgSpec{
		{Name: "mode", Positional: true, Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}, Help: localize.Text("Сейчас поддерживается только `os.bin`.", "Currently only `os.bin` is supported.")},
		{Name: "subject", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "model", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}, Aliases: []string{"Model"}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@transport(os.bin)", Insert: "@transport(os.bin)", Help: localize.Text("Включает stdin/stdout transport для модели.", "Enables stdin/stdout transport for the model.")},
		{Label: "@transport.Model()", Insert: "@transport.Model()", Help: localize.Text("Делает custom transport methods instance-style.", "Makes custom transport methods instance-style.")},
		{Label: "@transport(subject: \"id\")", Insert: "@transport(subject: \"id\")", Help: localize.Text("Переключает instance-style методы на идентификатор модели.", "Switches instance-style methods to the model identifier.")},
	},
}

var transportFieldSpec = sdk.AttrSpec{
	Namespace: "transport",
	Help:      localize.Text("На уровне поля и метода transport понимает `id`, `model`, `subject`, `ignore` и positional mode `os.bin`.", "At field and method level transport understands `id`, `model`, `subject`, `ignore`, and the positional `os.bin` mode."),
	Args: []sdk.AttrArgSpec{
		{Name: "mode", Positional: true, Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}, Help: localize.Text("Сейчас поддерживается только `os.bin`.", "Currently only `os.bin` is supported.")},
		{Name: "id", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}, Aliases: []string{"ID"}},
		{Name: "model", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}, Aliases: []string{"Model"}},
		{Name: "subject", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "ignore", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike, sdk.AttrValueTypeBool}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@transport.id()", Insert: "@transport.id()", Help: localize.Text("Отмечает поле как идентификатор transport-модели.", "Marks the field as the transport model identifier.")},
		{Label: "@transport.Model()", Insert: "@transport.Model()", Help: localize.Text("Делает method instance-style и позволяет принимать model/id subject.", "Makes the method instance-style and lets it accept the model/id subject.")},
		{Label: "@transport(os.bin)", Insert: "@transport(os.bin)", Help: localize.Text("Включает transport только для конкретного метода.", "Enables transport only for a specific method.")},
	},
}

func analyzeTransport(req sdk.GenerateRequest) (sdk.AnalyzeResponse, error) {
	builder := sdk.NewAnalyzeBuilder()

	modelResolved := builder.ValidateAttrSpec(req.Model.RuntimeAttrs, transportModelSpec)
	validateTransportMode(builder, modelRuntimeAttr(req.Model, "transport"), modeValue(modelResolved))
	validateTransportSubject(builder, modelRuntimeAttr(req.Model, "transport"), modelResolved.ValueMap())

	for _, field := range req.Model.Fields {
		resolved := builder.ValidateAttrSpec(field.RuntimeAttrs, transportFieldSpec)
		validateTransportMode(builder, fieldRuntimeAttr(field, "transport"), modeValue(resolved))
		validateTransportSubject(builder, fieldRuntimeAttr(field, "transport"), resolved.ValueMap())
	}

	for _, method := range req.Model.Methods {
		resolved := builder.ValidateAttrSpec(method.RuntimeAttrs, transportFieldSpec)
		validateTransportMode(builder, methodRuntimeAttr(method, "transport"), modeValue(resolved))
		validateTransportSubject(builder, methodRuntimeAttr(method, "transport"), resolved.ValueMap())
	}

	plan, err := buildTransportPlan(req)
	if err != nil {
		builder.AddDiagnostic(transportDiagnostic(err))
		return builder.Response(), nil
	}
	if plan == nil {
		return builder.Response(), nil
	}

	scope := packageScope(req.File, "transport")
	sdk.AddGeneratedClaimsInScope(builder, generateTransportResponse(plan), scope)
	return builder.Response(), nil
}

func packageScope(file sdk.FileContext, parts ...string) string {
	base := strings.TrimSpace(file.PackageName)
	if base == "" {
		base = filepath.Base(strings.TrimSpace(file.OutputDir))
	}
	items := make([]string, 0, len(parts)+1)
	if base != "" {
		items = append(items, base)
	}
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return strings.Join(items, "/")
}

func validateTransportMode(builder *sdk.AnalyzeBuilder, attr sdk.Attr, mode string) {
	if builder == nil || strings.TrimSpace(mode) == "" {
		return
	}
	if strings.EqualFold(strings.TrimSpace(mode), transportModeOSBin) {
		return
	}

	builder.AddDiagnostic(sdk.DiagnosticAt(
		attr,
		fmt.Sprintf(localize.Text("transport mode %q пока не поддерживается", "transport mode %q is not supported yet"), mode),
		localize.Text("Сейчас используйте `@transport(os.bin)` для stdin/stdout shell transport.", "For now use `@transport(os.bin)` for the stdin/stdout shell transport."),
	))
}

func validateTransportSubject(builder *sdk.AnalyzeBuilder, attr sdk.Attr, values map[string]sdk.Value) {
	if builder == nil {
		return
	}
	value, ok := values["subject"]
	if !ok {
		return
	}
	switch strings.ToLower(strings.TrimSpace(value.String())) {
	case "", string(transportSubjectModel), string(transportSubjectID):
		return
	default:
		builder.AddDiagnostic(sdk.DiagnosticAt(
			attr,
			fmt.Sprintf(localize.Text("transport subject должен быть `model` или `id`, а не %q", "transport subject must be `model` or `id`, not %q"), value.String()),
			localize.Text("Используйте `@transport(subject: \"model\")` или `@transport(subject: \"id\")`.", "Use `@transport(subject: \"model\")` or `@transport(subject: \"id\")`."),
		))
	}
}

func modeValue(resolved sdk.ResolvedAttr) string {
	for _, attr := range resolved.Attrs {
		if len(attr.Args) > 0 {
			return strings.TrimSpace(attr.Args[0].String())
		}
		if value, ok := attr.Named("mode"); ok {
			return strings.TrimSpace(value.String())
		}
	}
	return ""
}

func transportDiagnostic(err error) sdk.Diagnostic {
	if err == nil {
		return sdk.Diagnostic{}
	}
	if typed, ok := err.(*sdk.DiagnosticError); ok {
		return sdk.Diagnostic{
			Message: typed.Message,
			Hint:    typed.Hint,
			Detail:  typed.Detail,
		}
	}
	return sdk.Diagnostic{Message: err.Error()}
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

func methodRuntimeAttr(method sdk.Method, name string) sdk.Attr {
	attr, _ := method.ResolvedAttr(name)
	if len(attr.Attrs) > 0 {
		return attr.Attrs[0]
	}
	return sdk.Attr{}
}
