package main

import (
	"fmt"
	"path/filepath"
	"rpl/pkg/sdk"
	"strings"
)

var wasmModelSpec = sdk.AttrSpec{
	Namespace: "wasm",
	Help: sdk.Text(
		"Генерирует WIT-контракт, Rust guest и типизированные Wasmtime host bindings.",
		"Generates a WIT contract, a Rust guest, and typed Wasmtime host bindings.",
	),
	Args: []sdk.AttrArgSpec{
		{Name: "wit", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}, Aliases: []string{"witPackage"}, Help: sdk.Text("WIT package: namespace:name@1.2.3", "WIT package: namespace:name@1.2.3")},
		{Name: "world", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "interface", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "guest", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}, Help: sdk.Text("rust или none", "rust or none")},
		{Name: "hosts", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}, Help: sdk.Text("rust или none", "rust or none")},
		{Name: "memory", Types: []sdk.AttrValueType{sdk.AttrValueTypeNumber}, Help: sdk.Text("Лимит памяти в байтах", "Memory limit in bytes")},
		{Name: "timeout", Types: []sdk.AttrValueType{sdk.AttrValueTypeNumber}, Help: sdk.Text("Лимит вызова в миллисекундах", "Call timeout in milliseconds")},
		{Name: "fuel", Types: []sdk.AttrValueType{sdk.AttrValueTypeNumber}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@wasm(rust)", Insert: "@wasm(wit: \"example:service@0.1.0\", guest: \"rust\", hosts: \"rust\")", Help: sdk.Text("Rust Component guest и Wasmtime host.", "Rust Component guest and Wasmtime host.")},
		{Label: "@wasm(contract)", Insert: "@wasm(wit: \"example:service@0.1.0\", guest: \"none\", hosts: \"none\")", Help: sdk.Text("Генерирует только WIT и manifest.", "Generates only WIT and the manifest.")},
	},
}

var wasmMemberSpec = sdk.AttrSpec{
	Namespace: "wasm",
	Help:      sdk.Text("Настраивает WIT-имя или исключает поле/метод.", "Configures a WIT name or excludes a field/method."),
	Args: []sdk.AttrArgSpec{
		{Name: "name", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "ignore", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@wasm(name)", Insert: "@wasm(name: \"stable-name\")"},
		{Label: "@wasm(ignore)", Insert: "@wasm(ignore: true)"},
	},
}

func analyzeWASM(req sdk.GenerateRequest) (sdk.AnalyzeResponse, error) {
	builder := sdk.NewAnalyzeBuilder()
	builder.ValidateAttrSpec(req.Model.RuntimeAttrs, wasmModelSpec)
	for _, field := range req.Model.Fields {
		resolved := builder.ValidateAttrSpec(field.RuntimeAttrs, wasmMemberSpec).ValueMap()
		validateWASMMemberName(builder, wasmFieldAttr(field), field.Name, resolved["name"].String())
	}
	for _, method := range req.Model.Methods {
		resolved := builder.ValidateAttrSpec(method.RuntimeAttrs, wasmMemberSpec).ValueMap()
		validateWASMMemberName(builder, wasmMethodAttr(method), method.Name, resolved["name"].String())
	}
	plan, err := buildWASMPlan(req)
	if err != nil {
		builder.AddDiagnostic(sdk.DiagnosticAt(wasmModelAttr(req.Model), err.Error(), "Check @wasm wit, names, languages, limits, and exported types."))
		return builder.Response(), nil
	}
	sdk.AddGeneratedClaimsInScope(builder, generateWASMResponse(plan), wasmPackageScope(req.File, plan))
	return builder.Response(), nil
}

func validateWASMMemberName(builder *sdk.AnalyzeBuilder, attr sdk.Attr, owner string, raw string) {
	name := strings.TrimSpace(raw)
	if name != "" && !wasmNamePattern.MatchString(name) {
		builder.AddDiagnostic(sdk.DiagnosticAt(attr, fmt.Sprintf("invalid WIT name %q on %q", name, owner), "Use lowercase kebab-case."))
	}
}

func wasmPackageScope(file sdk.FileContext, plan *wasmPlan) string {
	base := strings.TrimSpace(file.GoPackagePath)
	if base == "" {
		base = filepath.Base(strings.TrimSpace(file.OutputDir))
	}
	return strings.Trim(strings.Join([]string{base, "wasm", plan.Root}, "/"), "/")
}

func wasmModelAttr(model sdk.Model) sdk.Attr {
	resolved, _ := model.ResolvedAttr("wasm")
	if len(resolved.Attrs) > 0 {
		return resolved.Attrs[0]
	}
	return sdk.Attr{}
}

func wasmFieldAttr(field sdk.Field) sdk.Attr {
	resolved, _ := field.ResolvedAttr("wasm")
	if len(resolved.Attrs) > 0 {
		return resolved.Attrs[0]
	}
	return sdk.Attr{}
}

func wasmMethodAttr(method sdk.Method) sdk.Attr {
	resolved, _ := method.ResolvedAttr("wasm")
	if len(resolved.Attrs) > 0 {
		return resolved.Attrs[0]
	}
	return sdk.Attr{}
}
