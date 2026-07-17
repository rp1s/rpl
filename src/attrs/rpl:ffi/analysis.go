package main

import (
	"fmt"
	"path/filepath"
	"rpl/pkg/sdk"
	"strings"
)

var ffiModelSpec = sdk.AttrSpec{
	Namespace: "ffi",
	Help:      sdk.Text("FFI генерирует C ABI, серверы C/Rust и клиенты Go/Python/C/Rust.", "FFI generates a C ABI, C/Rust servers, and Go/Python/C/Rust clients."),
	Args: []sdk.AttrArgSpec{
		{Name: "server", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}, Help: sdk.Text("c, rust, c,rust или none", "c, rust, c,rust, or none")},
		{Name: "clients", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}, Help: sdk.Text("Список: go,python,c,rust", "List: go,python,c,rust")},
		{Name: "library", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "prefix", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "abiVersion", Types: []sdk.AttrValueType{sdk.AttrValueTypeNumber}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@ffi(rust + all clients)", Insert: "@ffi(server: \"rust\", clients: \"go,python,c,rust\")", Help: sdk.Text("Rust-сервер и все клиенты.", "Rust server and every client binding.")},
		{Label: "@ffi(c)", Insert: "@ffi(server: \"c\", clients: \"go,python,c,rust\")", Help: sdk.Text("C-сервер и все клиенты.", "C server and every client binding.")},
	},
}

var ffiMemberSpec = sdk.AttrSpec{
	Namespace: "ffi",
	Help:      sdk.Text("На поле или методе FFI настраивает wire-name или исключение.", "On a field or method FFI configures the wire name or exclusion."),
	Args: []sdk.AttrArgSpec{
		{Name: "name", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "ignore", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@ffi(name)", Insert: "@ffi(name: \"wire_name\")", Help: sdk.Text("Задаёт стабильное ABI/wire-имя.", "Sets a stable ABI/wire name.")},
		{Label: "@ffi(ignore)", Insert: "@ffi(ignore: true)", Help: sdk.Text("Исключает поле или метод из FFI.", "Excludes a field or method from FFI.")},
	},
}

func analyzeFFI(req sdk.GenerateRequest) (sdk.AnalyzeResponse, error) {
	builder := sdk.NewAnalyzeBuilder()
	resolved := builder.ValidateAttrSpec(req.Model.RuntimeAttrs, ffiModelSpec)
	values := resolved.ValueMap()
	validateFFIModelConfig(builder, ffiModelAttr(req.Model), req.Model, values)

	fieldNames := make(map[string]string)
	for _, field := range req.Model.Fields {
		member := builder.ValidateAttrSpec(field.RuntimeAttrs, ffiMemberSpec).ValueMap()
		validateFFIMemberName(builder, ffiFieldAttr(field), field.Name, member["name"].String())
		if field.IgnoredBy("ffi") {
			continue
		}
		wireName := ffiConfiguredName(member, sdk.SnakeCase(field.Name))
		if previous, exists := fieldNames[wireName]; exists {
			builder.AddDiagnostic(sdk.DiagnosticAt(ffiFieldAttr(field), fmt.Sprintf("FFI field name %q is used by %q and %q", wireName, previous, field.Name), "Choose a unique @ffi(name: \"...\")."))
		} else {
			fieldNames[wireName] = field.Name
		}
		for _, method := range field.Methods {
			for _, attr := range method.RuntimeAttrs {
				builder.AddDiagnostic(sdk.DiagnosticAt(attr, fmt.Sprintf("ffi cannot be attached to field method %q", method.Name), "Move the operation to the model interface."))
			}
		}
	}

	methodNames := make(map[string]string)
	activeMethods := 0
	for _, method := range req.Model.Methods {
		member := builder.ValidateAttrSpec(method.RuntimeAttrs, ffiMemberSpec).ValueMap()
		validateFFIMemberName(builder, ffiMethodAttr(method), method.Name, member["name"].String())
		if member["ignore"].BoolValue() {
			continue
		}
		activeMethods++
		wireName := ffiConfiguredName(member, sdk.SnakeCase(method.Name))
		if previous, exists := methodNames[wireName]; exists {
			builder.AddDiagnostic(sdk.DiagnosticAt(ffiMethodAttr(method), fmt.Sprintf("FFI method name %q is used by %q and %q", wireName, previous, method.Name), "Choose a unique @ffi(name: \"...\")."))
		} else {
			methodNames[wireName] = method.Name
		}
	}
	if activeMethods == 0 {
		builder.AddDiagnostic(sdk.DiagnosticAt(ffiModelAttr(req.Model), fmt.Sprintf("ffi model %q has no exported methods", req.Model.Name), "Add a model method or remove @ffi."))
	}

	plan, err := buildFFIPlan(req)
	if err != nil {
		builder.AddDiagnostic(sdk.DiagnosticAt(ffiModelAttr(req.Model), err.Error(), "Check @ffi server, clients, prefix, library, and abiVersion."))
		return builder.Response(), nil
	}
	sdk.AddGeneratedClaimsInScope(builder, generateFFIResponse(plan), ffiPackageScope(req.File))
	return builder.Response(), nil
}

func validateFFIModelConfig(builder *sdk.AnalyzeBuilder, attr sdk.Attr, model sdk.Model, values map[string]sdk.Value) {
	prefix := strings.TrimSpace(values["prefix"].String())
	if prefix == "" {
		prefix = sdk.SnakeCase(model.Name)
	}
	if !ffiIdentifierPattern.MatchString(prefix) {
		builder.AddDiagnostic(sdk.DiagnosticAt(attr, fmt.Sprintf("invalid FFI prefix %q", prefix), "Use a C identifier: letters, numbers, and underscores."))
	}
	if library := strings.TrimSpace(values["library"].String()); library != "" && !ffiLibraryPattern.MatchString(library) {
		builder.AddDiagnostic(sdk.DiagnosticAt(attr, fmt.Sprintf("invalid FFI library name %q", library), "Use letters, numbers, dot, dash, or underscore."))
	}
	if raw := strings.TrimSpace(values["server"].String()); raw != "" {
		if _, err := parseFFILanguages(raw, "rust", ffiServerLanguages); err != nil {
			builder.AddDiagnostic(sdk.DiagnosticAt(attr, err.Error(), "Supported FFI servers: c and rust."))
		}
	}
	if raw := strings.TrimSpace(values["clients"].String()); raw != "" {
		if _, err := parseFFILanguages(raw, "all", ffiClientLanguages); err != nil {
			builder.AddDiagnostic(sdk.DiagnosticAt(attr, err.Error(), "Supported FFI clients: go, python, c, and rust."))
		}
	}
}

func validateFFIMemberName(builder *sdk.AnalyzeBuilder, attr sdk.Attr, owner string, raw string) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return
	}
	if !ffiIdentifierPattern.MatchString(name) {
		builder.AddDiagnostic(sdk.DiagnosticAt(attr, fmt.Sprintf("invalid FFI name %q on %q", name, owner), "Use letters, numbers, and underscores."))
	}
}

func ffiPackageScope(file sdk.FileContext) string {
	base := strings.TrimSpace(file.GoPackagePath)
	if base == "" {
		base = filepath.Base(strings.TrimSpace(file.OutputDir))
	}
	return strings.Trim(strings.Join([]string{base, "ffi"}, "/"), "/")
}

func ffiModelAttr(model sdk.Model) sdk.Attr {
	resolved, _ := model.ResolvedAttr("ffi")
	if len(resolved.Attrs) > 0 {
		return resolved.Attrs[0]
	}
	return sdk.Attr{}
}

func ffiFieldAttr(field sdk.Field) sdk.Attr {
	resolved, _ := field.ResolvedAttr("ffi")
	if len(resolved.Attrs) > 0 {
		return resolved.Attrs[0]
	}
	return sdk.Attr{}
}

func ffiMethodAttr(method sdk.Method) sdk.Attr {
	resolved, _ := method.ResolvedAttr("ffi")
	if len(resolved.Attrs) > 0 {
		return resolved.Attrs[0]
	}
	return sdk.Attr{}
}
