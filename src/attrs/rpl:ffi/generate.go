package main

import (
	"encoding/json"
	"fmt"
	"rpl/pkg/sdk"
	"sort"
)

type ffiSchema struct {
	ABI     int64             `json:"abi_version"`
	Model   string            `json:"model"`
	Prefix  string            `json:"prefix"`
	Library string            `json:"library"`
	Servers []string          `json:"servers"`
	Clients []string          `json:"clients"`
	Fields  []ffiSchemaField  `json:"fields"`
	Methods []ffiSchemaMethod `json:"methods"`
}

type ffiSchemaField struct {
	Name     string      `json:"name"`
	WireName string      `json:"wire_name"`
	Type     sdk.TypeRef `json:"type"`
}

type ffiSchemaMethod struct {
	Name     string           `json:"name"`
	WireName string           `json:"wire_name"`
	Params   []ffiSchemaField `json:"params"`
	Returns  []ffiSchemaField `json:"returns"`
}

func generateFFI(req sdk.GenerateRequest) (sdk.GenerateResponse, error) {
	plan, err := buildFFIPlan(req)
	if err != nil {
		return sdk.GenerateResponse{}, err
	}
	return generateFFIResponse(plan), nil
}

func generateFFIResponse(plan *ffiPlan) sdk.GenerateResponse {
	if plan == nil {
		return sdk.GenerateResponse{}
	}
	files := []sdk.GeneratedFile{
		{Path: "ffi/" + ffiCName(plan.Model.Name) + ".h", Content: generateFFIHeader(plan)},
		{Path: "ffi/schema.json", Content: generateFFISchema(plan)},
	}
	if plan.Servers["c"] {
		files = append(files, sdk.GeneratedFile{Path: "ffi/c/" + ffiCName(plan.Model.Name) + "_server.c", Content: generateFFICServer(plan)})
	}
	if plan.Clients["c"] {
		files = append(files, sdk.GeneratedFile{Path: "ffi/c/" + ffiCName(plan.Model.Name) + "_client.c", Content: generateFFICClient(plan)})
	}
	if plan.Clients["go"] {
		files = append(files, sdk.GeneratedFile{Path: "ffi/go/client.gen.go", Content: generateFFIGoClient(plan)})
		if plan.GoClientModes["cgo"] {
			files = append(files, sdk.GeneratedFile{Path: "ffi/go/native_cgo.gen.go", Content: generateFFIGoCGO(plan)})
		}
		if plan.GoClientModes["purego"] {
			files = append(files,
				sdk.GeneratedFile{Path: "ffi/go/native_purego.gen.go", Content: generateFFIGoPureGo(plan)},
				sdk.GeneratedFile{Path: "ffi/go/native_purego_unix.gen.go", Content: generateFFIGoPureGoUnix()},
				sdk.GeneratedFile{Path: "ffi/go/native_purego_windows.gen.go", Content: generateFFIGoPureGoWindows()},
			)
		}
	}
	if plan.Clients["python"] {
		files = append(files, sdk.GeneratedFile{Path: "ffi/python/" + ffiCName(plan.Model.Name) + "_ffi.py", Content: generateFFIPythonClient(plan)})
	}
	if plan.Servers["rust"] || plan.Clients["rust"] {
		files = append(files,
			sdk.GeneratedFile{Path: "ffi/rust/Cargo.toml", Content: generateFFIRustCargo(plan)},
			sdk.GeneratedFile{Path: "ffi/rust/src/lib.rs", Content: generateFFIRust(plan)},
		)
	}
	return sdk.GenerateResponse{Files: files}
}

func generateFFISchema(plan *ffiPlan) string {
	schema := ffiSchema{
		ABI: plan.ABIVersion, Model: plan.Model.Name, Prefix: plan.Prefix, Library: plan.Library,
		Servers: ffiSelectedLanguages(plan.Servers), Clients: ffiSelectedClients(plan),
	}
	for _, field := range plan.Fields {
		schema.Fields = append(schema.Fields, ffiSchemaField{Name: field.Name, WireName: field.WireName, Type: field.Type})
	}
	for _, method := range plan.Methods {
		item := ffiSchemaMethod{Name: method.Name, WireName: method.WireName}
		for _, param := range method.Params {
			item.Params = append(item.Params, ffiSchemaField{Name: param.Name, WireName: param.WireName, Type: param.Type})
		}
		for _, result := range method.Returns {
			item.Returns = append(item.Returns, ffiSchemaField{Name: result.Name, WireName: result.WireName, Type: result.Type})
		}
		schema.Methods = append(schema.Methods, item)
	}
	body, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Sprintf("{\"error\":%q}\n", err.Error())
	}
	return string(body) + "\n"
}

func ffiSelectedLanguages(values map[string]bool) []string {
	items := make([]string, 0, len(values))
	for name, enabled := range values {
		if enabled {
			items = append(items, name)
		}
	}
	sort.Strings(items)
	return items
}

func ffiSelectedClients(plan *ffiPlan) []string {
	if plan == nil {
		return nil
	}
	items := make([]string, 0, len(plan.Clients)+len(plan.GoClientModes))
	for name, enabled := range plan.Clients {
		if !enabled || name == "go" {
			continue
		}
		items = append(items, name)
	}
	if plan.Clients["go"] {
		if plan.GoClientModes["cgo"] {
			items = append(items, "go")
		}
		if plan.GoClientModes["purego"] {
			items = append(items, "go:purego")
		}
	}
	sort.Strings(items)
	return items
}
