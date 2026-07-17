package main

import (
	"fmt"
	"path/filepath"
	"rpl/pkg/sdk"
	"strings"
)

var transportModelSpec = sdk.AttrSpec{
	Namespace: "transport",
	Help:      localize.Text("Transport генерирует общий service contract и адаптеры os.bin, HTTP, Unix socket, NATS, Kafka и WebSocket.", "Transport generates one service contract with os.bin, HTTP, Unix socket, NATS, Kafka, and WebSocket adapters."),
	Args: []sdk.AttrArgSpec{
		{Name: "mode", Positional: true, Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}, Help: localize.Text("Режим: os.bin, http, unix, nats, kafka или websocket.", "Mode: os.bin, http, unix, nats, kafka, or websocket.")},
		{Name: "subject", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "model", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}, Aliases: []string{"Model"}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@transport(os.bin)", Insert: "@transport(os.bin)", Help: localize.Text("Включает stdin/stdout transport для модели.", "Enables stdin/stdout transport for the model.")},
		{Label: "@transport(http)", Insert: "@transport(http)", Help: localize.Text("Включает HTTP JSON adapter.", "Enables the HTTP JSON adapter.")},
		{Label: "@transport(unix)", Insert: "@transport(unix)", Help: localize.Text("Включает Unix socket adapter.", "Enables the Unix socket adapter.")},
		{Label: "@transport(nats)", Insert: "@transport(nats)", Help: localize.Text("Включает NATS request/reply adapter.", "Enables the NATS request/reply adapter.")},
		{Label: "@transport(kafka)", Insert: "@transport(kafka)", Help: localize.Text("Включает Kafka RPC adapter.", "Enables the Kafka RPC adapter.")},
		{Label: "@transport(websocket)", Insert: "@transport(websocket)", Help: localize.Text("Включает WebSocket adapter.", "Enables the WebSocket adapter.")},
		{Label: "@transport.Model()", Insert: "@transport.Model()", Help: localize.Text("Делает custom transport methods instance-style.", "Makes custom transport methods instance-style.")},
		{Label: "@transport(subject: \"id\")", Insert: "@transport(subject: \"id\")", Help: localize.Text("Переключает instance-style методы на идентификатор модели.", "Switches instance-style methods to the model identifier.")},
	},
}

var transportFieldSpec = sdk.AttrSpec{
	Namespace: "transport",
	Help:      localize.Text("На уровне поля и метода transport понимает `id`, `model`, `subject`, `ignore` и отдельный transport mode.", "At field and method level transport understands `id`, `model`, `subject`, `ignore`, and a dedicated transport mode."),
	Args: []sdk.AttrArgSpec{
		{Name: "mode", Positional: true, Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}, Help: localize.Text("Режим: os.bin, http, unix, nats, kafka или websocket.", "Mode: os.bin, http, unix, nats, kafka, or websocket.")},
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

	validateTransportAttrs(builder, req.Model.RuntimeAttrs, transportModelSpec, false)

	for _, field := range req.Model.Fields {
		validateTransportAttrs(builder, field.RuntimeAttrs, transportFieldSpec, true)
	}

	for _, method := range req.Model.Methods {
		validateTransportAttrs(builder, method.RuntimeAttrs, transportFieldSpec, false)
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

// Multiple @transport(mode) attrs are intentional: they select independent
// adapters rather than conflicting values of one scalar setting. Validate each
// occurrence separately so the generic attr resolver does not report a false
// mode conflict while retaining normal name/type diagnostics.
func validateTransportAttrs(builder *sdk.AnalyzeBuilder, attrs []sdk.Attr, spec sdk.AttrSpec, validateMode bool) {
	for _, attr := range attrs {
		if !attr.Matches("transport") && attr.Namespace() != "transport" {
			continue
		}
		resolved := builder.ValidateAttrSpec([]sdk.Attr{attr}, spec)
		if validateMode {
			validateTransportMode(builder, attr, modeValue(resolved))
		}
		validateTransportSubject(builder, attr, resolved.ValueMap())
	}
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
	if _, err := normalizeTransportMode(mode); err == nil {
		return
	}

	builder.AddDiagnostic(sdk.DiagnosticAt(
		attr,
		fmt.Sprintf(localize.Text("transport mode %q не поддерживается", "transport mode %q is not supported"), mode),
		localize.Text("Используйте `os.bin`, `http`, `unix`, `nats`, `kafka` или `websocket`.", "Use `os.bin`, `http`, `unix`, `nats`, `kafka`, or `websocket`."),
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
