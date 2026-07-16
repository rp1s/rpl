package main

import (
	"os/exec"
	"rpl/pkg/sdk"
	"strings"
)

type grpcSubjectMode string

const (
	grpcSubjectNone  grpcSubjectMode = "none"
	grpcSubjectModel grpcSubjectMode = "model"
	grpcSubjectID    grpcSubjectMode = "id"
)

var localize = struct {
	Text func(string, string) string
}{
	Text: sdk.Text,
}

var rplerr = struct {
	Newf func(string, ...any) *sdk.DiagnosticError
}{
	Newf: sdk.NewErrorf,
}

type grpcPlan struct {
	RootModel            sdk.Model
	RootFields           []sdk.Field
	RootNode             *grpcMessageNode
	Nodes                []*grpcMessageNode
	ServiceMethods       []grpcInsideMethod
	ServiceSubject       grpcMethodSubject
	IDSubject            *grpcMethodSubject
	InsideMethods        []grpcInsideMethod
	ModelImportPath      string
	ModelsBaseImportPath string
	ProtoFileName        string
	PackageName          string
	GoPackagePath        string
	ServiceName          string
	HasService           bool
	AutoPut              bool
	AutoGetByID          bool
	AutoDelete           bool
	AutoList             bool
	UsesTimestamp        bool
	UsesWrappers         bool
	UsesTimeImport       bool
}

type goListPackage struct {
	Dir      string   `json:"Dir"`
	GoFiles  []string `json:"GoFiles"`
	CgoFiles []string `json:"CgoFiles"`
}

type grpcMessageNode struct {
	Model          sdk.Model
	Path           []string
	MessageName    string
	ToHelperName   string
	FromHelperName string
	Fields         []grpcFieldBinding
}

type grpcFieldBinding struct {
	Field         sdk.Field
	Number        int
	Kind          string
	ProtoType     string
	GoMessageType string
	Child         *grpcMessageNode
}

type grpcInsideMethod struct {
	Field               sdk.Field
	Method              sdk.Method
	Subject             grpcMethodSubject
	BridgeName          string
	HandlerName         string
	RequestMessageName  string
	ResponseMessageName string
	Params              []grpcInsideValue
	Results             []grpcInsideValue
	HasErrorReturn      bool
}

type grpcInsideValue struct {
	Name      string
	Type      sdk.TypeRef
	ProtoType string
	Kind      string
	Number    int
}

type grpcMethodSubject struct {
	Mode     grpcSubjectMode
	IDField  sdk.Field
	IDValue  grpcInsideValue
	Explicit bool
}

func generateGRPC(req sdk.GenerateRequest) (sdk.GenerateResponse, error) {
	if err := ensureGRPCTools(); err != nil {
		return sdk.GenerateResponse{}, err
	}

	plan, err := buildGRPCPlan(req)
	if err != nil {
		return sdk.GenerateResponse{}, err
	}

	mappingBuilder := sdk.NewCodeBuilder()
	serverBuilder := sdk.NewCodeBuilder()
	clientBuilder := sdk.NewCodeBuilder()

	addGRPCMappingImports(mappingBuilder, plan)
	addGRPCServerImports(serverBuilder, plan)
	addGRPCClientImports(clientBuilder, plan)

	mappingBuilder.AddOrderedBlock("grpc.to.message", renderRootToMethod(plan), 10)
	mappingBuilder.AddOrderedBlock("grpc.from.message", renderRootFromMethod(plan), 20)
	if helpers := renderNestedHelpers(plan); helpers != "" {
		mappingBuilder.AddOrderedBlock("grpc.nested.helpers", helpers, 30)
	}

	if adapter := renderServiceAdapter(plan); adapter != "" {
		serverBuilder.AddOrderedBlock("grpc.service.adapter", adapter, 10)
	}
	if client := renderServiceClient(plan); client != "" {
		clientBuilder.AddOrderedBlock("grpc.service.client", client, 10)
	}

	files, err := generateProtoArtifacts(plan)
	if err != nil {
		return sdk.GenerateResponse{}, err
	}

	response := sdk.GenerateResponse{Files: files}
	body, err := sdk.RenderGoFile("grpc", mappingBuilder.Response())
	if err != nil {
		return sdk.GenerateResponse{}, err
	}
	if strings.TrimSpace(string(body)) != "" {
		response.Files = append(response.Files, sdk.GeneratedFile{
			Path:    "grpc/proto.gen.go",
			Content: string(body),
		})
	}
	body, err = sdk.RenderGoFile("grpc", serverBuilder.Response())
	if err != nil {
		return sdk.GenerateResponse{}, err
	}
	if strings.TrimSpace(string(body)) != "" {
		response.Files = append(response.Files, sdk.GeneratedFile{
			Path:    "grpc/server.gen.go",
			Content: string(body),
		})
	}
	body, err = sdk.RenderGoFile("grpc", clientBuilder.Response())
	if err != nil {
		return sdk.GenerateResponse{}, err
	}
	if strings.TrimSpace(string(body)) != "" {
		response.Files = append(response.Files, sdk.GeneratedFile{
			Path:    "grpc/client.gen.go",
			Content: string(body),
		})
	}

	return response, nil
}

func addGRPCMappingImports(builder *sdk.CodeBuilder, plan *grpcPlan) {
	if builder == nil || plan == nil {
		return
	}

	builder.AddImport(grpcModelImportPath(plan, plan.RootModel.Name), "modelpkg")
	for _, item := range grpcNestedModelImports(plan) {
		builder.AddImport(item.Path, item.Alias)
	}
	if grpcMessageUsesTimestamp(plan) {
		builder.AddImport("google.golang.org/protobuf/types/known/timestamppb")
	}
	if plan.UsesWrappers {
		builder.AddImport("google.golang.org/protobuf/types/known/wrapperspb")
	}
}

func addGRPCServerImports(builder *sdk.CodeBuilder, plan *grpcPlan) {
	if builder == nil || plan == nil {
		return
	}
	builder.AddImport(grpcModelImportPath(plan, plan.RootModel.Name), "modelpkg")
	if plan.HasService {
		builder.AddImport("context")
		builder.AddImport("google.golang.org/grpc")
		builder.AddImport("fmt")
	}
	if grpcInsideUsesResponseTimestamp(plan) {
		builder.AddImport("google.golang.org/protobuf/types/known/timestamppb")
	}
	if grpcInsideUsesTimeImport(plan) {
		builder.AddImport("time")
	}
}

func addGRPCClientImports(builder *sdk.CodeBuilder, plan *grpcPlan) {
	if builder == nil || plan == nil {
		return
	}
	builder.AddImport(grpcModelImportPath(plan, plan.RootModel.Name), "modelpkg")
	if plan.HasService {
		builder.AddImport("context")
		builder.AddImport("google.golang.org/grpc")
		builder.AddImport("fmt")
	}
	if grpcInsideUsesRequestTimestamp(plan) {
		builder.AddImport("google.golang.org/protobuf/types/known/timestamppb")
	}
	if grpcInsideUsesTimeImport(plan) {
		builder.AddImport("time")
	}
}

func grpcMessageUsesTimestamp(plan *grpcPlan) bool {
	if plan == nil {
		return false
	}
	for _, node := range plan.Nodes {
		if node == nil {
			continue
		}
		for _, field := range node.Fields {
			if field.Kind == "time" {
				return true
			}
		}
	}
	return false
}

func grpcInsideUsesRequestTimestamp(plan *grpcPlan) bool {
	if plan == nil {
		return false
	}
	for _, method := range plan.ServiceMethods {
		if method.Subject.Mode == grpcSubjectID && method.Subject.IDValue.Kind == "time" {
			return true
		}
		for _, param := range method.Params {
			if param.Kind == "time" {
				return true
			}
		}
	}
	return false
}

func grpcInsideUsesResponseTimestamp(plan *grpcPlan) bool {
	if plan == nil {
		return false
	}
	for _, method := range plan.ServiceMethods {
		for _, result := range method.Results {
			if result.Kind == "time" {
				return true
			}
		}
	}
	return false
}

func grpcInsideUsesTimeImport(plan *grpcPlan) bool {
	return grpcInsideUsesRequestTimestamp(plan) || grpcInsideUsesResponseTimestamp(plan)
}

func grpcNestedModelImports(plan *grpcPlan) []sdk.ImportRef {
	if plan == nil {
		return nil
	}

	seen := make(map[string]struct{})
	imports := make([]sdk.ImportRef, 0)
	for _, node := range plan.Nodes {
		if node == nil || node.Model.Name == "" || node.Model.Name == plan.RootModel.Name {
			continue
		}

		path := grpcModelImportPath(plan, node.Model.Name)
		alias := grpcModelImportAlias(node.Model.Name)
		key := alias + "|" + path
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		imports = append(imports, sdk.ImportRef{Alias: alias, Path: path})
	}

	return imports
}

func ensureGRPCTools() error {
	for _, name := range []string{"protoc", "protoc-gen-go", "protoc-gen-go-grpc"} {
		if _, err := exec.LookPath(name); err != nil {
			return rplerr.Newf(
				localize.Text("утилита %q не найдена для grpc-плагина", "tool %q is required for the grpc plugin"),
				name,
			).WithHint(localize.Text(
				"Установите `protoc`, `protoc-gen-go` и `protoc-gen-go-grpc`, чтобы RPL мог генерировать `.proto` и Go gRPC-код.",
				"Install `protoc`, `protoc-gen-go`, and `protoc-gen-go-grpc` so RPL can generate `.proto` and Go gRPC code.",
			))
		}
	}

	return nil
}
