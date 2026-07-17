package main

import (
	"fmt"
	"regexp"
	"rpl/pkg/sdk"
	"sort"
	"strconv"
	"strings"
)

const (
	wasmtimeVersion   = "46.0.1"
	witBindgenVersion = "0.59.0"
)

var (
	wasmNamePattern    = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	wasmVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
	wasmWITKeywords    = map[string]bool{
		"as": true, "bool": true, "borrow": true, "char": true, "enum": true,
		"export": true, "f32": true, "f64": true, "flags": true, "func": true,
		"future": true, "import": true, "include": true, "interface": true,
		"list": true, "option": true, "own": true, "package": true, "record": true,
		"resource": true, "result": true, "s8": true, "s16": true, "s32": true,
		"s64": true, "static": true, "stream": true, "string": true, "tuple": true,
		"type": true, "u8": true, "u16": true, "u32": true, "u64": true,
		"use": true, "variant": true, "with": true, "world": true,
	}
	wasmRustKeywords = map[string]bool{
		"as": true, "break": true, "const": true, "continue": true, "crate": true,
		"else": true, "enum": true, "extern": true, "false": true, "fn": true,
		"for": true, "if": true, "impl": true, "in": true, "let": true,
		"loop": true, "match": true, "mod": true, "move": true, "mut": true,
		"pub": true, "ref": true, "return": true, "self": true, "static": true,
		"struct": true, "super": true, "trait": true, "true": true, "type": true,
		"unsafe": true, "use": true, "where": true, "while": true,
	}
)

type wasmPlan struct {
	Model       sdk.Model
	Package     string
	Namespace   string
	PackageName string
	Version     string
	World       string
	Interface   string
	Guest       string
	Hosts       map[string]bool
	Root        string
	MemoryBytes int64
	TimeoutMS   int64
	Fuel        int64
	Models      []wasmModel
	Methods     []wasmMethod
}

type wasmModel struct {
	Name    string      `json:"name"`
	WITName string      `json:"wit_name"`
	Fields  []wasmField `json:"fields"`
}

type wasmField struct {
	Name    string      `json:"name"`
	WITName string      `json:"wit_name"`
	Type    sdk.TypeRef `json:"type"`
}

type wasmMethod struct {
	Name     string        `json:"name"`
	WITName  string        `json:"wit_name"`
	Params   []wasmValue   `json:"params,omitempty"`
	Returns  []sdk.TypeRef `json:"returns,omitempty"`
	HasError bool          `json:"has_error,omitempty"`
}

type wasmValue struct {
	Name    string      `json:"name"`
	WITName string      `json:"wit_name"`
	Type    sdk.TypeRef `json:"type"`
}

func buildWASMPlan(req sdk.GenerateRequest) (*wasmPlan, error) {
	values := req.Model.ResolvedValues("wasm")
	packageID := strings.TrimSpace(values["wit"].String())
	if packageID == "" {
		packageID = "rpl:" + wasmKebab(req.Model.Name) + "@0.1.0"
	}
	namespace, packageName, version, err := parseWASMPackage(packageID)
	if err != nil {
		return nil, err
	}
	world := wasmConfiguredName(values["world"].String(), wasmKebab(req.Model.Name)+"-plugin")
	interfaceName := wasmConfiguredName(values["interface"].String(), wasmKebab(req.Model.Name)+"-service")
	guest := strings.ToLower(strings.TrimSpace(values["guest"].String()))
	if guest == "" {
		guest = "rust"
	}
	if guest != "rust" && guest != "none" {
		return nil, fmt.Errorf("wasm guest %q is not supported yet; use rust or none", guest)
	}
	hosts, err := parseWASMHosts(values["hosts"].String())
	if err != nil {
		return nil, err
	}
	memoryBytes, err := wasmPositiveInt(values["memory"].String(), 64*1024*1024, "memory")
	if err != nil {
		return nil, err
	}
	timeoutMS, err := wasmPositiveInt(values["timeout"].String(), 1000, "timeout")
	if err != nil {
		return nil, err
	}
	fuel, err := wasmPositiveInt(values["fuel"].String(), 10_000_000, "fuel")
	if err != nil {
		return nil, err
	}

	plan := &wasmPlan{
		Model: req.Model, Package: packageID, Namespace: namespace, PackageName: packageName,
		Version: version, World: world, Interface: interfaceName, Guest: guest, Hosts: hosts,
		Root: wasmKebab(req.Model.Name), MemoryBytes: memoryBytes, TimeoutMS: timeoutMS, Fuel: fuel,
	}
	models, err := collectWASMModels(req.File, req.Model)
	if err != nil {
		return nil, err
	}
	plan.Models = models
	for _, method := range req.Model.Methods {
		values := method.ResolvedValues("wasm")
		if values["ignore"].BoolValue() {
			continue
		}
		item := wasmMethod{Name: method.Name, WITName: wasmConfiguredName(values["name"].String(), wasmKebab(method.Name))}
		for _, param := range method.Params {
			item.Params = append(item.Params, wasmValue{Name: param.Name, WITName: wasmKebab(param.Name), Type: param.Type})
		}
		item.Returns = append(item.Returns, method.Returns...)
		for index, result := range item.Returns {
			if !result.IsError() {
				continue
			}
			if index != len(item.Returns)-1 {
				return nil, fmt.Errorf("wasm method %q must place error last in its return list", method.Name)
			}
			item.HasError = true
			item.Returns = item.Returns[:len(item.Returns)-1]
			break
		}
		plan.Methods = append(plan.Methods, item)
	}
	if len(plan.Methods) == 0 {
		return nil, fmt.Errorf("wasm model %q has no exported methods", req.Model.Name)
	}
	if err := validateWASMPlan(plan, req.File); err != nil {
		return nil, err
	}
	return plan, nil
}

func parseWASMPackage(raw string) (string, string, string, error) {
	identity, version, ok := strings.Cut(strings.TrimSpace(raw), "@")
	if !ok || !wasmVersionPattern.MatchString(version) {
		return "", "", "", fmt.Errorf("wasm package must look like namespace:name@1.2.3")
	}
	namespace, name, ok := strings.Cut(identity, ":")
	if !ok || !wasmNamePattern.MatchString(namespace) || !wasmNamePattern.MatchString(name) {
		return "", "", "", fmt.Errorf("wasm package namespace and name must be lowercase WIT identifiers")
	}
	return namespace, name, version, nil
}

func parseWASMHosts(raw string) (map[string]bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = "rust"
	}
	result := make(map[string]bool)
	for _, item := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	}) {
		switch item {
		case "", "none":
		case "rust":
			result[item] = true
		default:
			return nil, fmt.Errorf("wasm host %q is not verified for the Component Model; use rust or none", item)
		}
	}
	return result, nil
}

func wasmPositiveInt(raw string, fallback int64, name string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("wasm %s must be a positive integer", name)
	}
	return parsed, nil
}

func collectWASMModels(ctx sdk.FileContext, root sdk.Model) ([]wasmModel, error) {
	models := make(map[string]sdk.Model)
	for _, model := range ctx.AllModels {
		models[model.Name] = model
	}
	models[root.Name] = root
	state := make(map[string]int)
	result := make([]wasmModel, 0)
	var visit func(sdk.Model) error
	visit = func(model sdk.Model) error {
		if state[model.Name] == 1 {
			return fmt.Errorf("wasm does not support recursive model graph through %q", model.Name)
		}
		if state[model.Name] == 2 {
			return nil
		}
		state[model.Name] = 1
		item := wasmModel{Name: model.Name, WITName: wasmKebab(model.Name)}
		for _, field := range model.ActiveFields("wasm") {
			values := field.ResolvedValues("wasm")
			item.Fields = append(item.Fields, wasmField{
				Name: field.Name, WITName: wasmConfiguredName(values["name"].String(), wasmKebab(field.Name)), Type: field.Type,
			})
			if child, ok := models[field.Type.BaseName()]; ok {
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		state[model.Name] = 2
		result = append(result, item)
		return nil
	}
	if err := visit(root); err != nil {
		return nil, err
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Name == root.Name {
			return false
		}
		if result[j].Name == root.Name {
			return true
		}
		return result[i].WITName < result[j].WITName
	})
	return result, nil
}

func validateWASMPlan(plan *wasmPlan, ctx sdk.FileContext) error {
	for label, value := range map[string]string{"world": plan.World, "interface": plan.Interface} {
		if !wasmNamePattern.MatchString(value) {
			return fmt.Errorf("wasm %s %q must be a lowercase kebab-case WIT identifier", label, value)
		}
	}
	known := make(map[string]bool)
	modelNames := make(map[string]string)
	for _, model := range plan.Models {
		known[model.Name] = true
		if previous, exists := modelNames[model.WITName]; exists {
			return fmt.Errorf("wasm models %q and %q use the same WIT name %q", previous, model.Name, model.WITName)
		}
		modelNames[model.WITName] = model.Name
	}
	for _, model := range plan.Models {
		fieldNames := make(map[string]string)
		for _, field := range model.Fields {
			if previous, exists := fieldNames[field.WITName]; exists {
				return fmt.Errorf("wasm fields %s.%s and %s.%s use the same WIT name %q", model.Name, previous, model.Name, field.Name, field.WITName)
			}
			fieldNames[field.WITName] = field.Name
			if err := validateWASMType(field.Type, known, ctx); err != nil {
				return fmt.Errorf("wasm field %s.%s: %w", model.Name, field.Name, err)
			}
		}
	}
	methodNames := make(map[string]string)
	for _, method := range plan.Methods {
		if previous, exists := methodNames[method.WITName]; exists {
			return fmt.Errorf("wasm methods %q and %q use the same WIT name %q", previous, method.Name, method.WITName)
		}
		methodNames[method.WITName] = method.Name
		paramNames := make(map[string]string)
		for _, param := range method.Params {
			if previous, exists := paramNames[param.WITName]; exists {
				return fmt.Errorf("wasm method %s parameters %q and %q use the same WIT name %q", method.Name, previous, param.Name, param.WITName)
			}
			paramNames[param.WITName] = param.Name
			if err := validateWASMType(param.Type, known, ctx); err != nil {
				return fmt.Errorf("wasm method %s parameter %s: %w", method.Name, param.Name, err)
			}
		}
		for _, result := range method.Returns {
			if err := validateWASMType(result, known, ctx); err != nil {
				return fmt.Errorf("wasm method %s result: %w", method.Name, err)
			}
		}
	}
	return nil
}

func validateWASMType(typeRef sdk.TypeRef, known map[string]bool, ctx sdk.FileContext) error {
	if typeRef.IsString() || typeRef.IsBool() || typeRef.IsInteger() || typeRef.IsFloat() || known[typeRef.BaseName()] {
		return nil
	}
	if typeRef.IsTime() {
		return fmt.Errorf("time.Time has no implicit WIT representation; expose int64 or string explicitly")
	}
	if typeRef.IsError() {
		return fmt.Errorf("error is only allowed as the final method result")
	}
	if typeRef.IsModel(ctx) {
		return nil
	}
	return fmt.Errorf("type %q cannot be represented by the WASM MVP", typeRef.Name)
}

func wasmConfiguredName(raw string, fallback string) string {
	if value := strings.TrimSpace(raw); value != "" {
		return value
	}
	return fallback
}

func wasmKebab(value string) string {
	snake := sdk.SnakeCase(value)
	return strings.ReplaceAll(strings.Trim(snake, "_"), "_", "-")
}

func wasmRustIdent(value string) string {
	identifier := strings.ReplaceAll(value, "-", "_")
	if wasmRustKeywords[identifier] {
		return identifier + "_"
	}
	return identifier
}

func wasmWITIdent(value string) string {
	if wasmWITKeywords[value] {
		return "%" + value
	}
	return value
}

func wasmRustTypeName(value string) string {
	parts := strings.Split(wasmKebab(value), "-")
	for index, part := range parts {
		if part != "" {
			parts[index] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}
