package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"rpl/pkg/sdk"
	"strings"
)

func generateProtoArtifacts(plan *grpcPlan) ([]sdk.GeneratedFile, error) {
	tempDir, err := os.MkdirTemp("", "rpl-grpc-*")
	if err != nil {
		return nil, fmt.Errorf(localize.Text("создание временной папки для grpc: %w", "create grpc temp directory: %w"), err)
	}
	defer os.RemoveAll(tempDir)

	protoContent := renderProto(plan)
	protoPath := filepath.Join(tempDir, plan.ProtoFileName)
	if err := os.WriteFile(protoPath, []byte(protoContent), 0o644); err != nil {
		return nil, fmt.Errorf(localize.Text("запись proto-файла %q: %w", "write proto file %q: %w"), protoPath, err)
	}

	cmd := exec.Command(
		"protoc",
		"--go_out=paths=source_relative:.",
		"--go-grpc_out=paths=source_relative,require_unimplemented_servers=false:.",
		plan.ProtoFileName,
	)
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		problem := rplerr.Newf(
			localize.Text("grpc-плагин не смог сгенерировать код для модели %q", "grpc plugin could not generate code for model %q"),
			plan.RootModel.Name,
		).WithHint(localize.Text(
			"Проверьте, что `protoc` и Go gRPC plugins установлены и доступны в PATH.",
			"Make sure `protoc` and the Go gRPC plugins are installed and available in PATH.",
		))
		if detail := strings.TrimSpace(string(output)); detail != "" {
			problem.WithDetail(detail)
		}
		return nil, problem
	}

	files := make([]sdk.GeneratedFile, 0, 3)
	names := []string{
		plan.ProtoFileName,
		strings.TrimSuffix(plan.ProtoFileName, ".proto") + ".pb.go",
	}
	if plan.HasService {
		names = append(names, strings.TrimSuffix(plan.ProtoFileName, ".proto")+"_grpc.pb.go")
	} else {
		files = append(files, sdk.GeneratedFile{
			Path:   filepath.ToSlash(filepath.Join("grpc", strings.TrimSuffix(plan.ProtoFileName, ".proto")+"_grpc.pb.go")),
			Delete: true,
		})
	}
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join(tempDir, name))
		if err != nil {
			return nil, fmt.Errorf(localize.Text("чтение grpc-артефакта %q: %w", "read grpc artifact %q: %w"), name, err)
		}

		files = append(files, sdk.GeneratedFile{
			Path:    filepath.ToSlash(filepath.Join("grpc", name)),
			Content: string(body),
		})
	}

	return files, nil
}

func renderProto(plan *grpcPlan) string {
	var builder strings.Builder

	builder.WriteString("syntax = \"proto3\";\n\n")
	builder.WriteString("package ")
	builder.WriteString(plan.PackageName)
	builder.WriteString(";\n\n")
	builder.WriteString("option go_package = ")
	builder.WriteString(fmt.Sprintf("%q", grpcGoPackageOption(plan)))
	builder.WriteString(";\n\n")

	imports := make([]string, 0, 2)
	if plan.UsesTimestamp {
		imports = append(imports, "google/protobuf/timestamp.proto")
	}
	if plan.UsesWrappers {
		imports = append(imports, "google/protobuf/wrappers.proto")
	}
	for _, item := range imports {
		builder.WriteString("import ")
		builder.WriteString(fmt.Sprintf("%q", item))
		builder.WriteString(";\n")
	}
	if len(imports) > 0 {
		builder.WriteString("\n")
	}

	for i, node := range plan.Nodes {
		builder.WriteString(renderProtoMessage(node))
		builder.WriteString("\n")
		if i != len(plan.Nodes)-1 || len(protoServiceMessages(plan)) > 0 || plan.HasService {
			builder.WriteString("\n")
		}
	}

	serviceMessages := protoServiceMessages(plan)
	for i, block := range serviceMessages {
		builder.WriteString(block)
		builder.WriteString("\n")
		if i != len(serviceMessages)-1 {
			builder.WriteString("\n")
		}
	}

	if service := renderProtoService(plan); service != "" {
		builder.WriteString("\n")
		builder.WriteString(service)
		builder.WriteString("\n")
	}

	return builder.String()
}

func renderProtoMessage(node *grpcMessageNode) string {
	if node == nil {
		return ""
	}

	if len(node.Fields) == 0 {
		return fmt.Sprintf("message %s {}", node.MessageName)
	}

	lines := make([]string, 0, len(node.Fields))
	for _, field := range node.Fields {
		label := ""
		if field.Field.Type.IsList {
			label = "repeated "
		}

		lines = append(lines, fmt.Sprintf("\t%s%s %s = %d;", label, field.ProtoType, sdk.SnakeCase(field.Field.Name), field.Number))
	}

	return fmt.Sprintf("message %s {\n%s\n}", node.MessageName, strings.Join(lines, "\n"))
}

func renderProtoInsideMessage(name string, values []grpcInsideValue) string {
	if len(values) == 0 {
		return fmt.Sprintf("message %s {}", name)
	}

	lines := make([]string, 0, len(values))
	for _, value := range values {
		lines = append(lines, fmt.Sprintf("\t%s %s = %d;", value.ProtoType, sdk.SnakeCase(value.Name), value.Number))
	}

	return fmt.Sprintf("message %s {\n%s\n}", name, strings.Join(lines, "\n"))
}

func protoServiceMessages(plan *grpcPlan) []string {
	if plan == nil || !plan.HasService {
		return nil
	}

	items := make([]string, 0, len(plan.ServiceMethods)*2+4)
	if plan.AutoGetByID {
		items = append(items, renderProtoIDRequest(plan, plan.RootModel.Name+"GetByIDRequest"))
	}
	if plan.AutoDelete {
		items = append(items, renderProtoIDRequest(plan, plan.RootModel.Name+"DeleteRequest"))
		items = append(items, fmt.Sprintf("message %sDeleteResponse {}", plan.RootModel.Name))
	}
	if plan.AutoList {
		items = append(items, fmt.Sprintf("message %sListRequest {}", plan.RootModel.Name))
		items = append(items, renderProtoListResponse(plan))
	}
	for _, method := range plan.ServiceMethods {
		items = append(items, renderProtoServiceMethodRequest(plan, method))
		items = append(items, renderProtoInsideMessage(method.ResponseMessageName, method.Results))
	}
	return items
}

func renderProtoListResponse(plan *grpcPlan) string {
	return fmt.Sprintf("message %sListResponse {\n\trepeated %s items = 1;\n}", plan.RootModel.Name, plan.RootNode.MessageName)
}

func renderProtoIDRequest(plan *grpcPlan, name string) string {
	if plan == nil || plan.IDSubject == nil {
		return fmt.Sprintf("message %s {}", name)
	}
	return fmt.Sprintf(
		"message %s {\n\t%s %s = 1;\n}",
		name,
		plan.IDSubject.IDValue.ProtoType,
		sdk.SnakeCase(plan.IDSubject.IDField.Name),
	)
}

func renderProtoServiceMethodRequest(plan *grpcPlan, method grpcInsideMethod) string {
	lines := make([]string, 0, len(method.Params)+1)
	switch method.Subject.Mode {
	case grpcSubjectNone:
		// Classic methods do not carry an implicit model/id subject.
	case grpcSubjectID:
		lines = append(lines, fmt.Sprintf("\t%s %s = 1;", method.Subject.IDValue.ProtoType, sdk.SnakeCase(method.Subject.IDField.Name)))
	default:
		lines = append(lines, fmt.Sprintf("\t%s %s = 1;", plan.RootNode.MessageName, sdk.SnakeCase(plan.RootModel.Name)))
	}
	for _, value := range method.Params {
		lines = append(lines, fmt.Sprintf("\t%s %s = %d;", value.ProtoType, sdk.SnakeCase(value.Name), value.Number))
	}

	if len(lines) == 0 {
		return fmt.Sprintf("message %s {}", method.RequestMessageName)
	}

	return fmt.Sprintf("message %s {\n%s\n}", method.RequestMessageName, strings.Join(lines, "\n"))
}

func renderProtoService(plan *grpcPlan) string {
	if plan == nil || !plan.HasService || plan.RootNode == nil {
		return ""
	}

	lines := make([]string, 0, len(plan.ServiceMethods)+4)
	if plan.AutoPut {
		lines = append(lines, fmt.Sprintf("\trpc Put (%s) returns (%s);", plan.RootNode.MessageName, plan.RootNode.MessageName))
	}
	if plan.AutoGetByID {
		lines = append(lines, fmt.Sprintf("\trpc GetByID (%s) returns (%s);", plan.RootModel.Name+"GetByIDRequest", plan.RootNode.MessageName))
	}
	if plan.AutoDelete {
		lines = append(lines, fmt.Sprintf("\trpc Delete (%s) returns (%sDeleteResponse);", plan.RootModel.Name+"DeleteRequest", plan.RootModel.Name))
	}
	if plan.AutoList {
		lines = append(lines, fmt.Sprintf("\trpc List (%sListRequest) returns (%sListResponse);", plan.RootModel.Name, plan.RootModel.Name))
	}
	for _, method := range plan.ServiceMethods {
		lines = append(lines, fmt.Sprintf("\trpc %s (%s) returns (%s);", method.BridgeName, method.RequestMessageName, method.ResponseMessageName))
	}

	return fmt.Sprintf("service %s {\n%s\n}", plan.ServiceName, strings.Join(lines, "\n"))
}

func grpcGoPackageOption(plan *grpcPlan) string {
	if plan == nil {
		return ""
	}

	importPath := strings.TrimSpace(plan.GoPackagePath)
	packageName := strings.TrimSpace(plan.PackageName)
	switch {
	case importPath == "":
		if packageName == "" {
			return "./"
		}
		return "./;" + packageName
	case packageName == "":
		return importPath
	default:
		return importPath + ";" + packageName
	}
}
