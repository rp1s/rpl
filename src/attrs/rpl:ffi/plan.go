package main

import (
	"fmt"
	"regexp"
	"rpl/pkg/sdk"
	"sort"
	"strconv"
	"strings"
)

var ffiIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var ffiLibraryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

var ffiServerLanguages = map[string]bool{"c": true, "rust": true}
var ffiClientLanguages = map[string]bool{"go": true, "python": true, "c": true, "rust": true}

var ffiRustKeywords = map[string]bool{
	"as": true, "break": true, "const": true, "continue": true, "crate": true, "else": true,
	"enum": true, "extern": true, "false": true, "fn": true, "for": true, "if": true,
	"impl": true, "in": true, "let": true, "loop": true, "match": true, "mod": true,
	"move": true, "mut": true, "pub": true, "ref": true, "return": true, "self": true,
	"Self": true, "static": true, "struct": true, "super": true, "trait": true, "true": true,
	"type": true, "unsafe": true, "use": true, "where": true, "while": true,
}

var ffiPythonKeywords = map[string]bool{
	"and": true, "as": true, "assert": true, "async": true, "await": true, "break": true,
	"class": true, "continue": true, "def": true, "del": true, "elif": true, "else": true,
	"except": true, "False": true, "finally": true, "for": true, "from": true, "global": true,
	"if": true, "import": true, "in": true, "is": true, "lambda": true, "None": true,
	"nonlocal": true, "not": true, "or": true, "pass": true, "raise": true, "return": true,
	"True": true, "try": true, "while": true, "with": true, "yield": true,
}

type ffiPlan struct {
	Model         sdk.Model
	Prefix        string
	Library       string
	ABIVersion    int64
	Servers       map[string]bool
	Clients       map[string]bool
	GoClientModes map[string]bool
	Fields        []ffiField
	Methods       []ffiMethod
}

type ffiField struct {
	Name     string
	WireName string
	Type     sdk.TypeRef
}

type ffiValue struct {
	Name     string
	WireName string
	Type     sdk.TypeRef
}

type ffiMethod struct {
	Name       string
	ExportName string
	WireName   string
	Params     []ffiValue
	Returns    []ffiValue
}

func buildFFIPlan(req sdk.GenerateRequest) (*ffiPlan, error) {
	values := req.Model.ResolvedValues("ffi")
	prefix := strings.TrimSpace(values["prefix"].String())
	if prefix == "" {
		prefix = sdk.SnakeCase(req.Model.Name)
	}
	library := strings.TrimSpace(values["library"].String())
	if library == "" {
		library = prefix
	}
	abiVersion := int64(1)
	if raw := strings.TrimSpace(values["abiVersion"].String()); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("ffi abiVersion must be a positive integer")
		}
		abiVersion = parsed
	}
	servers, err := parseFFILanguages(values["server"].String(), "rust", ffiServerLanguages)
	if err != nil {
		return nil, fmt.Errorf("ffi server: %w", err)
	}
	clients, goClientModes, err := parseFFIClients(values["clients"].String(), "go,python,c,rust")
	if err != nil {
		return nil, fmt.Errorf("ffi clients: %w", err)
	}

	plan := &ffiPlan{
		Model:         req.Model,
		Prefix:        prefix,
		Library:       library,
		ABIVersion:    abiVersion,
		Servers:       servers,
		Clients:       clients,
		GoClientModes: goClientModes,
	}
	for _, field := range req.Model.ActiveFields("ffi") {
		wireName := ffiConfiguredName(field.ResolvedValues("ffi"), sdk.SnakeCase(field.Name))
		plan.Fields = append(plan.Fields, ffiField{Name: field.Name, WireName: wireName, Type: field.Type})
	}
	for _, method := range req.Model.Methods {
		memberValues := method.ResolvedValues("ffi")
		if memberValues["ignore"].BoolValue() {
			continue
		}
		wireName := ffiConfiguredName(memberValues, sdk.SnakeCase(method.Name))
		item := ffiMethod{Name: method.Name, ExportName: ffiExportedName(method.Name), WireName: wireName}
		for _, param := range method.Params {
			item.Params = append(item.Params, ffiValue{Name: param.Name, WireName: sdk.SnakeCase(param.Name), Type: param.Type})
		}
		for index, result := range method.Returns {
			name := "value"
			if len(method.Returns) > 1 {
				name = fmt.Sprintf("value%d", index+1)
			}
			item.Returns = append(item.Returns, ffiValue{Name: name, WireName: name, Type: result})
		}
		plan.Methods = append(plan.Methods, item)
	}
	return plan, nil
}

func ffiConfiguredName(values map[string]sdk.Value, fallback string) string {
	if value := strings.TrimSpace(values["name"].String()); value != "" {
		return value
	}
	return fallback
}

func parseFFILanguages(raw string, fallback string, allowed map[string]bool) (map[string]bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = fallback
	}
	selected := make(map[string]bool)
	for _, item := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	}) {
		item = strings.TrimSpace(item)
		switch item {
		case "":
			continue
		case "none":
			selected = make(map[string]bool)
			continue
		case "all":
			for name := range allowed {
				selected[name] = true
			}
			continue
		}
		if !allowed[item] {
			keys := make([]string, 0, len(allowed))
			for name := range allowed {
				keys = append(keys, name)
			}
			sort.Strings(keys)
			return nil, fmt.Errorf("unsupported language %q; use %s", item, strings.Join(keys, ", "))
		}
		selected[item] = true
	}
	return selected, nil
}

func parseFFIClients(raw string, fallback string) (map[string]bool, map[string]bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = fallback
	}
	clients := make(map[string]bool)
	goModes := make(map[string]bool)
	for _, item := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	}) {
		item = strings.TrimSpace(item)
		switch item {
		case "":
			continue
		case "none":
			clients = make(map[string]bool)
			goModes = make(map[string]bool)
			continue
		case "all":
			for name := range ffiClientLanguages {
				clients[name] = true
			}
			goModes["cgo"] = true
			continue
		case "go", "go:cgo":
			clients["go"] = true
			goModes["cgo"] = true
			continue
		case "purego", "go:purego":
			clients["go"] = true
			goModes["purego"] = true
			continue
		case "go:all", "go:both":
			clients["go"] = true
			goModes["cgo"] = true
			goModes["purego"] = true
			continue
		}
		if !ffiClientLanguages[item] || item == "go" {
			return nil, nil, fmt.Errorf("unsupported client %q; use c, rust, python, go, go:purego, or go:all", item)
		}
		clients[item] = true
	}
	return clients, goModes, nil
}

func ffiExportedName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == ' '
	})
	if len(parts) == 0 {
		return "Call"
	}
	for index, part := range parts {
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}

func ffiCName(value string) string {
	name := sdk.SnakeCase(value)
	if name == "" {
		return "value"
	}
	return name
}

func ffiRustName(value string) string {
	if ffiRustKeywords[value] {
		return "r#" + value
	}
	return value
}

func ffiPythonName(value string) string {
	if ffiPythonKeywords[value] {
		return value + "_"
	}
	return value
}
