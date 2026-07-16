package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	goast "go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"os"
	"os/exec"
	pkgpath "path"
	"path/filepath"
	"rpl/pkg/sdk"
	"sort"
	"strings"
)

func buildGRPCPlan(req sdk.GenerateRequest) (*grpcPlan, error) {
	plan := &grpcPlan{
		RootModel:            req.Model,
		RootFields:           req.Model.ActiveFields("grpc"),
		ModelImportPath:      strings.TrimSpace(req.File.GoPackagePath),
		ModelsBaseImportPath: pkgpath.Dir(strings.TrimSpace(req.File.GoPackagePath)),
		ProtoFileName:        grpcProtoFileName(req),
		PackageName:          grpcPackageName(req),
		GoPackagePath:        grpcGeneratedPackagePath(req),
		ServiceName:          req.Model.Name + "Service",
	}

	root, err := buildGRPCNode(plan, req.File, req.Model, []string{req.Model.Name}, []string{req.Model.Name})
	if err != nil {
		return nil, err
	}

	plan.RootNode = root
	idSubject, err := buildGRPCIDSubject(plan, req.Model)
	if err != nil {
		return nil, err
	}
	plan.IDSubject = idSubject

	serviceSubject, err := buildGRPCServiceSubject(plan, req.Model)
	if err != nil {
		return nil, err
	}
	plan.ServiceSubject = serviceSubject
	plan.AutoPut = true
	plan.AutoList = true
	plan.AutoGetByID = serviceSubject.Mode == grpcSubjectID
	plan.AutoDelete = serviceSubject.Mode == grpcSubjectID

	serviceMethods, err := buildInsideMethods(plan, req)
	if err != nil {
		return nil, err
	}
	plan.ServiceMethods = serviceMethods
	plan.InsideMethods = serviceMethods
	plan.HasService = true
	return plan, nil
}

func grpcPackageName(req sdk.GenerateRequest) string {
	return "grpc"
}

func grpcGeneratedPackagePath(req sdk.GenerateRequest) string {
	base := strings.TrimSpace(req.File.GoPackagePath)
	if base == "" {
		return ""
	}
	return pkgpath.Join(base, "grpc")
}

func grpcProtoFileName(req sdk.GenerateRequest) string {
	stem := sdk.SnakeCase(req.Model.Name)
	if stem == "" {
		stem = strings.TrimSpace(req.File.ModelFileStem)
	}
	return stem + ".proto"
}

func grpcModelImportAlias(modelName string) string {
	alias := strings.ReplaceAll(sdk.SnakeCase(modelName), "_", "")
	if strings.TrimSpace(alias) == "" {
		return "modelpkg"
	}
	return alias + "pkg"
}

func grpcModelImportPath(plan *grpcPlan, modelName string) string {
	if plan == nil {
		return ""
	}
	if modelName == plan.RootModel.Name {
		if strings.TrimSpace(plan.ModelImportPath) != "" {
			return plan.ModelImportPath
		}
		return ".."
	}
	if strings.TrimSpace(plan.ModelsBaseImportPath) != "" {
		return pkgpath.Join(plan.ModelsBaseImportPath, sdk.SnakeCase(modelName))
	}
	return "../" + sdk.SnakeCase(modelName)
}

func grpcModelGoType(plan *grpcPlan, modelName string) string {
	if plan == nil {
		return modelName
	}
	if modelName == plan.RootModel.Name {
		return "modelpkg." + modelName
	}
	return grpcModelImportAlias(modelName) + "." + modelName
}

// buildGRPCNode expands one RPL model into a protobuf message description.
// The path is based on field names, so generated nested message names stay
// stable and never collide inside the shared `models` package.
func buildGRPCNode(plan *grpcPlan, ctx sdk.FileContext, model sdk.Model, path []string, stack []string) (*grpcMessageNode, error) {
	node := &grpcMessageNode{
		Model:       model,
		Path:        append([]string(nil), path...),
		MessageName: grpcMessageName(path),
	}
	if len(path) > 1 {
		node.ToHelperName = grpcToHelperName(path)
		node.FromHelperName = grpcFromHelperName(path)
	}

	plan.Nodes = append(plan.Nodes, node)

	number := 1
	for _, field := range model.ActiveFields("grpc") {
		binding, err := buildGRPCField(plan, ctx, field, path, stack)
		if err != nil {
			return nil, err
		}
		if binding == nil {
			continue
		}

		binding.Number = number
		number++
		node.Fields = append(node.Fields, *binding)
	}

	return node, nil
}

func buildGRPCField(plan *grpcPlan, ctx sdk.FileContext, field sdk.Field, path []string, stack []string) (*grpcFieldBinding, error) {
	if field.Mode("grpc") == "inside" {
		return nil, nil
	}

	if model, ok := field.Type.RefModel(ctx); ok {
		if containsString(stack, model.Name) {
			return nil, rplerr.Newf(
				localize.Text("grpc-плагин пока не поддерживает рекурсивную модель %q", "grpc plugin does not support recursive model %q yet"),
				model.Name,
			).WithHint(localize.Text(
				"Разорвите цикл между моделями или пометьте внутреннее поле как `@grpc(mode: \"inside\")`, если оно не должно попадать в protobuf-схему.",
				"Break the model cycle or mark the internal field with `@grpc(mode: \"inside\")` if it should stay out of the protobuf schema.",
			))
		}

		child, err := buildGRPCNode(plan, ctx, *model, append(path, field.Name), append(stack, model.Name))
		if err != nil {
			return nil, err
		}

		return &grpcFieldBinding{
			Field:         field,
			Kind:          "model",
			ProtoType:     child.MessageName,
			GoMessageType: "*" + child.MessageName,
			Child:         child,
		}, nil
	}

	if field.Type.IsTime() {
		plan.UsesTimestamp = true
		if field.Type.IsList {
			plan.UsesTimeImport = true
		}

		return &grpcFieldBinding{
			Field:         field,
			Kind:          "time",
			ProtoType:     "google.protobuf.Timestamp",
			GoMessageType: "*timestamppb.Timestamp",
		}, nil
	}

	switch {
	case field.Type.IsString():
		if field.Type.Optional && !field.Type.IsList {
			plan.UsesWrappers = true
			return &grpcFieldBinding{Field: field, Kind: "wrapper_string", ProtoType: "google.protobuf.StringValue", GoMessageType: "*wrapperspb.StringValue"}, nil
		}
		return &grpcFieldBinding{Field: field, Kind: "string", ProtoType: "string", GoMessageType: "string"}, nil
	case field.Type.IsBool():
		if field.Type.Optional && !field.Type.IsList {
			plan.UsesWrappers = true
			return &grpcFieldBinding{Field: field, Kind: "wrapper_bool", ProtoType: "google.protobuf.BoolValue", GoMessageType: "*wrapperspb.BoolValue"}, nil
		}
		return &grpcFieldBinding{Field: field, Kind: "bool", ProtoType: "bool", GoMessageType: "bool"}, nil
	case field.Type.IsInteger():
		if field.Type.Optional && !field.Type.IsList {
			plan.UsesWrappers = true
			if strings.HasPrefix(field.Type.BaseName(), "u") {
				return &grpcFieldBinding{Field: field, Kind: "wrapper_uint", ProtoType: "google.protobuf.UInt64Value", GoMessageType: "*wrapperspb.UInt64Value"}, nil
			}
			return &grpcFieldBinding{Field: field, Kind: "wrapper_int", ProtoType: "google.protobuf.Int64Value", GoMessageType: "*wrapperspb.Int64Value"}, nil
		}
		if strings.HasPrefix(field.Type.BaseName(), "u") {
			return &grpcFieldBinding{Field: field, Kind: "uint", ProtoType: "uint64", GoMessageType: "uint64"}, nil
		}
		return &grpcFieldBinding{Field: field, Kind: "int", ProtoType: "int64", GoMessageType: "int64"}, nil
	case field.Type.IsFloat():
		if field.Type.Optional && !field.Type.IsList {
			plan.UsesWrappers = true
			return &grpcFieldBinding{Field: field, Kind: "wrapper_float", ProtoType: "google.protobuf.DoubleValue", GoMessageType: "*wrapperspb.DoubleValue"}, nil
		}
		return &grpcFieldBinding{Field: field, Kind: "float", ProtoType: "double", GoMessageType: "float64"}, nil
	default:
		return nil, rplerr.Newf(
			localize.Text("grpc не умеет сериализовать поле %q типа %q", "grpc cannot serialize field %q of type %q"),
			field.Name,
			field.Type.Name,
		).WithHint(localize.Text(
			"Используйте scalar-тип, `time.Time`, другую модель RPL или пометьте поле как `@grpc(mode: \"inside\")`, если его не нужно включать в protobuf.",
			"Use a scalar type, `time.Time`, another RPL model, or mark the field as `@grpc(mode: \"inside\")` if it should stay out of protobuf.",
		))
	}
}

func buildInsideMethods(plan *grpcPlan, req sdk.GenerateRequest) ([]grpcInsideMethod, error) {
	methods := make([]grpcInsideMethod, 0)
	usedNames := grpcReservedServiceMethodNames(plan)

	for _, method := range req.Model.Methods {
		if !grpcMethodSelected(method) {
			continue
		}
		item, err := buildInsideMethod(plan, req.Model, sdk.Field{}, method)
		if err != nil {
			return nil, err
		}
		if err := grpcEnsureUniqueServiceMethod(usedNames, item.BridgeName); err != nil {
			return nil, err
		}
		methods = append(methods, item)
	}

	for _, field := range req.Model.Fields {
		explicitMethods := false
		for _, method := range field.Methods {
			if !grpcMethodSelected(method) {
				continue
			}
			explicitMethods = true

			item, err := buildInsideMethod(plan, req.Model, field, method)
			if err != nil {
				return nil, err
			}
			if err := grpcEnsureUniqueServiceMethod(usedNames, item.BridgeName); err != nil {
				return nil, err
			}
			methods = append(methods, item)
		}

		if explicitMethods || field.Mode("grpc") != "inside" {
			continue
		}

		discovered, err := discoverInsideMethods(req.File, field)
		if err != nil {
			return nil, err
		}
		for _, method := range discovered {
			item, err := buildInsideMethod(plan, req.Model, field, method)
			if err != nil {
				return nil, err
			}
			if err := grpcEnsureUniqueServiceMethod(usedNames, item.BridgeName); err != nil {
				return nil, err
			}
			methods = append(methods, item)
		}
	}

	return methods, nil
}

func grpcMethodSelected(method sdk.Method) bool {
	if method.Mode("grpc") == "inside" {
		return true
	}
	if len(method.RuntimeAttrs) > 0 {
		return true
	}
	return len(method.Attrs) == 0
}

func buildInsideMethod(plan *grpcPlan, model sdk.Model, field sdk.Field, method sdk.Method) (grpcInsideMethod, error) {
	subject, err := buildGRPCMethodSubject(plan, model, field, method)
	if err != nil {
		return grpcInsideMethod{}, err
	}

	item := grpcInsideMethod{
		Field:   field,
		Method:  method,
		Subject: subject,
		Results: make([]grpcInsideValue, 0),
		Params:  make([]grpcInsideValue, 0),
	}

	item.BridgeName = grpcInsideBridgeName(field, method)
	item.HandlerName = "Handle" + item.BridgeName + "GRPC"
	item.RequestMessageName = plan.RootModel.Name + item.BridgeName + "Request"
	item.ResponseMessageName = plan.RootModel.Name + item.BridgeName + "Response"

	paramNumber := 1
	if item.Subject.Mode != grpcSubjectNone {
		paramNumber = 2
	}
	for index, param := range method.Params {
		value, err := buildInsideValue(plan, param.Name, param.Type, paramNumber+index, false)
		if err != nil {
			return grpcInsideMethod{}, err
		}
		item.Params = append(item.Params, value)
	}

	errorSeen := false
	resultIndex := 1
	for i, result := range method.Returns {
		if result.IsError() {
			if errorSeen || i != len(method.Returns)-1 {
				return grpcInsideMethod{}, rplerr.Newf(
					localize.Text("inside-метод %q должен иметь `error` только последним результатом", "inside method %q must use `error` only as the last return value"),
					method.Name,
				).WithHint(localize.Text(
					"Используйте сигнатуру вроде `return (string, error)` или `return (error)`.",
					"Use a signature like `return (string, error)` or `return (error)`.",
				))
			}
			errorSeen = true
			item.HasErrorReturn = true
			continue
		}

		value, err := buildInsideValue(plan, grpcInsideResultName(field, method, len(item.Results)), result, resultIndex, true)
		if err != nil {
			return grpcInsideMethod{}, err
		}
		item.Results = append(item.Results, value)
		resultIndex++
	}

	return item, nil
}

func buildGRPCServiceSubject(plan *grpcPlan, model sdk.Model) (grpcMethodSubject, error) {
	mode, err := grpcResolveSubjectMode(model.ResolvedValues("grpc"), "")
	if err != nil {
		return grpcMethodSubject{}, err
	}
	if mode == "" {
		if plan != nil && plan.IDSubject != nil && plan.IDSubject.Explicit {
			mode = grpcSubjectID
		} else {
			mode = grpcSubjectModel
		}
	}

	if mode == grpcSubjectID {
		if plan == nil || plan.IDSubject == nil {
			return grpcMethodSubject{}, rplerr.Newf(
				localize.Text("grpc subject=id требует идентификатор у модели %q", "grpc subject=id requires an identifier on model %q"),
				model.Name,
			).WithHint(localize.Text(
				"Добавьте поле `Id`, либо пометьте нужное поле как `@grpc.id()`.",
				"Add an `Id` field or mark the correct field with `@grpc.id()`.",
			))
		}
		return *plan.IDSubject, nil
	}

	return grpcMethodSubject{Mode: grpcSubjectModel}, nil
}

func buildGRPCMethodSubject(plan *grpcPlan, model sdk.Model, field sdk.Field, method sdk.Method) (grpcMethodSubject, error) {
	methodValues := method.ResolvedValues("grpc")
	mode, err := grpcResolveSubjectMode(methodValues, "")
	if err != nil {
		return grpcMethodSubject{}, err
	}
	if mode == "" && strings.TrimSpace(field.Name) != "" {
		mode, err = grpcResolveSubjectMode(field.ResolvedValues("grpc"), "")
		if err != nil {
			return grpcMethodSubject{}, err
		}
	}

	if !grpcMethodUsesSubject(model, field, method) {
		if mode != "" {
			return grpcMethodSubject{}, rplerr.Newf(
				localize.Text("grpc method %q использует subject без `@grpc.Model()`", "grpc method %q uses subject without `@grpc.Model()`"),
				grpcInsideBridgeName(field, method),
			).WithHint(localize.Text(
				"Добавьте `@grpc.Model()` на метод, поле или модель, либо уберите `subject` и оставьте метод в classic grpc режиме.",
				"Add `@grpc.Model()` on the method, field, or model, or remove `subject` and keep the method in classic grpc mode.",
			))
		}
		return grpcMethodSubject{Mode: grpcSubjectNone}, nil
	}

	if mode == "" {
		mode = plan.ServiceSubject.Mode
	}

	if mode == grpcSubjectID {
		if plan == nil || plan.IDSubject == nil {
			return grpcMethodSubject{}, rplerr.Newf(
				localize.Text("grpc method %q требует идентификатор у модели %q", "grpc method %q requires an identifier on model %q"),
				grpcInsideBridgeName(field, method),
				model.Name,
			).WithHint(localize.Text(
				"Добавьте поле `Id`, либо пометьте нужное поле как `@grpc.id()`.",
				"Add an `Id` field or mark the correct field with `@grpc.id()`.",
			))
		}
		return *plan.IDSubject, nil
	}

	return grpcMethodSubject{Mode: grpcSubjectModel}, nil
}

func buildGRPCIDSubject(plan *grpcPlan, model sdk.Model) (*grpcMethodSubject, error) {
	field, ok, explicit, err := grpcFindIDField(model)
	if err != nil || !ok {
		return nil, err
	}
	if field.IgnoredBy("grpc") {
		return nil, rplerr.Newf(
			localize.Text("grpc id-поле %q не может быть скрыто от grpc", "grpc id field %q cannot be hidden from grpc"),
			field.Name,
		).WithHint(localize.Text(
			"Уберите `@ignore(\"grpc\")` или выберите другое поле как `@grpc.id()`.",
			"Remove `@ignore(\"grpc\")` or choose a different field via `@grpc.id()`.",
		))
	}

	value, err := buildInsideValue(plan, field.Name, field.Type, 1, false)
	if err != nil {
		return nil, err
	}

	subject := grpcMethodSubject{
		Mode:     grpcSubjectID,
		IDField:  *field,
		IDValue:  value,
		Explicit: explicit,
	}
	return &subject, nil
}

func grpcFindIDField(model sdk.Model) (*sdk.Field, bool, bool, error) {
	explicit := make([]*sdk.Field, 0, 1)
	for i := range model.Fields {
		values := model.Fields[i].ResolvedValues("grpc")
		if value, ok := values["id"]; ok && value.BoolValue() {
			explicit = append(explicit, &model.Fields[i])
		}
	}
	if len(explicit) > 1 {
		names := make([]string, 0, len(explicit))
		for _, field := range explicit {
			names = append(names, field.Name)
		}
		return nil, false, false, rplerr.Newf(
			localize.Text("grpc id указан сразу у нескольких полей модели %q: %s", "grpc id is declared on multiple fields of model %q: %s"),
			model.Name,
			strings.Join(names, ", "),
		).WithHint(localize.Text(
			"Оставьте `@grpc.id()` только у одного поля.",
			"Keep `@grpc.id()` on only one field.",
		))
	}
	if len(explicit) == 1 {
		return explicit[0], true, true, nil
	}

	for i := range model.Fields {
		if strings.EqualFold(strings.TrimSpace(model.Fields[i].Name), "id") {
			return &model.Fields[i], true, false, nil
		}
	}

	return nil, false, false, nil
}

func grpcResolveSubjectMode(values map[string]sdk.Value, fallback grpcSubjectMode) (grpcSubjectMode, error) {
	value, ok := values["subject"]
	if !ok {
		return fallback, nil
	}

	switch strings.ToLower(strings.TrimSpace(value.String())) {
	case "":
		return fallback, nil
	case string(grpcSubjectModel):
		return grpcSubjectModel, nil
	case string(grpcSubjectID):
		return grpcSubjectID, nil
	default:
		return "", rplerr.Newf(
			localize.Text("grpc subject должен быть `model` или `id`, а не %q", "grpc subject must be `model` or `id`, not %q"),
			value.String(),
		).WithHint(localize.Text(
			"Используйте `@grpc(subject: \"model\")` или `@grpc(subject: \"id\")`.",
			"Use `@grpc(subject: \"model\")` or `@grpc(subject: \"id\")`.",
		))
	}
}

func grpcMethodUsesSubject(model sdk.Model, field sdk.Field, method sdk.Method) bool {
	if method.Mode("grpc") == "inside" {
		return true
	}
	if strings.TrimSpace(field.Name) != "" && field.Mode("grpc") == "inside" {
		return true
	}

	if enabled, ok := grpcResolveModelFlag(method.ResolvedValues("grpc")); ok {
		return enabled
	}
	if strings.TrimSpace(field.Name) != "" {
		if enabled, ok := grpcResolveModelFlag(field.ResolvedValues("grpc")); ok {
			return enabled
		}
	}
	if enabled, ok := grpcResolveModelFlag(model.ResolvedValues("grpc")); ok {
		return enabled
	}

	return false
}

func grpcFieldUsesSubject(model sdk.Model, field sdk.Field) bool {
	if strings.TrimSpace(field.Name) == "" {
		return false
	}
	if field.Mode("grpc") == "inside" {
		return true
	}
	if enabled, ok := grpcResolveModelFlag(field.ResolvedValues("grpc")); ok {
		return enabled
	}
	if enabled, ok := grpcResolveModelFlag(model.ResolvedValues("grpc")); ok {
		return enabled
	}
	return false
}

func grpcResolveModelFlag(values map[string]sdk.Value) (bool, bool) {
	for name, value := range values {
		if strings.EqualFold(strings.TrimSpace(name), "model") {
			return value.BoolValue(), true
		}
	}
	return false, false
}

func grpcReservedServiceMethodNames(plan *grpcPlan) map[string]struct{} {
	items := map[string]struct{}{
		"Put":  {},
		"List": {},
	}
	if plan != nil && plan.ServiceSubject.Mode == grpcSubjectID {
		items["GetByID"] = struct{}{}
		items["Delete"] = struct{}{}
	}
	return items
}

func grpcEnsureUniqueServiceMethod(used map[string]struct{}, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if _, exists := used[name]; exists {
		return rplerr.Newf(
			localize.Text("grpc service method %q сгенерирован больше одного раза", "grpc service method %q is generated more than once"),
			name,
		).WithHint(localize.Text(
			"Переименуйте конфликтующий field-method или смените grpc subject/авто-операции.",
			"Rename the conflicting field method or change the grpc subject/auto operations.",
		))
	}
	used[name] = struct{}{}
	return nil
}

func buildInsideValue(plan *grpcPlan, name string, typeRef sdk.TypeRef, number int, isReturn bool) (grpcInsideValue, error) {
	value := grpcInsideValue{
		Name:   grpcInsideValueName(name, number),
		Type:   typeRef,
		Number: number,
	}

	if !typeRef.IsBytes() && (typeRef.Optional || typeRef.IsList) {
		kind := localize.Text("аргумент", "argument")
		if isReturn {
			kind = localize.Text("результат", "return value")
		}
		return grpcInsideValue{}, rplerr.Newf(
			localize.Text("grpc inside не поддерживает %s %q типа %q", "grpc inside does not support %s %q of type %q"),
			kind,
			name,
			typeRef.Name,
		).WithHint(localize.Text(
			"Пока supported only plain scalar значения, []byte, time.Time и trailing `error` без optional/list-обёрток.",
			"Right now only plain scalar values, []byte, time.Time, and a trailing `error` are supported without optional/list wrappers.",
		))
	}

	switch {
	case typeRef.IsBytes():
		value.Kind = "bytes"
		value.ProtoType = "bytes"
	case typeRef.IsString():
		value.Kind = "string"
		value.ProtoType = "string"
	case typeRef.IsBool():
		value.Kind = "bool"
		value.ProtoType = "bool"
	case typeRef.IsInteger():
		if strings.HasPrefix(typeRef.BaseName(), "u") || typeRef.BaseName() == "byte" {
			value.Kind = "uint"
			value.ProtoType = "uint64"
		} else {
			value.Kind = "int"
			value.ProtoType = "int64"
		}
	case typeRef.IsFloat():
		value.Kind = "float"
		value.ProtoType = "double"
	case typeRef.IsTime():
		value.Kind = "time"
		value.ProtoType = "google.protobuf.Timestamp"
		plan.UsesTimestamp = true
		plan.UsesTimeImport = true
	default:
		kind := localize.Text("аргумент", "argument")
		if isReturn {
			kind = localize.Text("результат", "return value")
		}
		return grpcInsideValue{}, rplerr.Newf(
			localize.Text("grpc inside не поддерживает %s %q типа %q", "grpc inside does not support %s %q of type %q"),
			kind,
			name,
			typeRef.Name,
		).WithHint(localize.Text(
			"Пока поддерживаются string, bool, числа, []byte, time.Time и trailing `error`.",
			"Right now supported types are string, bool, numbers, []byte, time.Time, and a trailing `error`.",
		))
	}

	return value, nil
}

func discoverInsideMethods(ctx sdk.FileContext, field sdk.Field) ([]sdk.Method, error) {
	importPath, typeName, ok := resolveGoTypeImport(ctx, field.Type)
	if !ok {
		return nil, nil
	}

	pkg, err := inspectGoPackage(ctx, importPath)
	if err != nil {
		return nil, err
	}

	methods, err := discoverExportedMethods(pkg, typeName)
	if err != nil {
		return nil, err
	}

	sort.Slice(methods, func(i int, j int) bool {
		return methods[i].Name < methods[j].Name
	})
	return methods, nil
}

func resolveGoTypeImport(ctx sdk.FileContext, typeRef sdk.TypeRef) (string, string, bool) {
	typeName := strings.TrimSpace(typeRef.Name)
	if typeName == "" {
		return "", "", false
	}

	prefix, name, ok := strings.Cut(typeName, ".")
	if !ok || strings.TrimSpace(prefix) == "" || strings.TrimSpace(name) == "" {
		return "", "", false
	}

	for _, item := range ctx.Imports {
		alias := strings.TrimSpace(item.Alias)
		if alias == "" {
			alias = pkgpath.Base(strings.TrimSpace(item.Path))
		}
		if alias == prefix {
			return strings.TrimSpace(item.Path), strings.TrimSpace(name), true
		}
	}

	return "", "", false
}

func inspectGoPackage(ctx sdk.FileContext, importPath string) (goListPackage, error) {
	workDir := grpcGoListWorkDir(ctx)
	item, detail, err := runGoListPackage(importPath, workDir, false)
	if err == nil {
		return item, nil
	}
	if !shouldFallbackGoList(detail) {
		return goListPackage{}, grpcGoListProblem(importPath, detail)
	}

	tempDir, cleanup, tempErr := createGoListModule()
	if tempErr != nil {
		return goListPackage{}, grpcGoListProblem(importPath, detail)
	}
	defer cleanup()

	item, fallbackDetail, err := runGoListPackage(importPath, tempDir, true)
	if err == nil {
		return item, nil
	}

	return goListPackage{}, grpcGoListProblem(importPath, mergeGoListDetails(detail, fallbackDetail))
}

func grpcGoListWorkDir(ctx sdk.FileContext) string {
	if root := strings.TrimSpace(ctx.ProjectRoot); root != "" {
		return filepath.FromSlash(root)
	}
	if sourcePath := strings.TrimSpace(ctx.SourcePath); sourcePath != "" {
		return filepath.Dir(filepath.FromSlash(sourcePath))
	}
	return ""
}

func runGoListPackage(importPath string, workDir string, allowModuleDownload bool) (goListPackage, string, error) {
	args := []string{"list", "-json"}
	if allowModuleDownload {
		args = append(args, "-mod=mod")
	}
	args = append(args, importPath)

	cmd := exec.Command("go", args...)
	if trimmed := strings.TrimSpace(workDir); trimmed != "" {
		cmd.Dir = trimmed
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		return goListPackage{}, detail, err
	}

	var item goListPackage
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &item); err != nil {
		return goListPackage{}, strings.TrimSpace(stdout.String()), fmt.Errorf(localize.Text("разбор go list для пакета %q: %w", "decode go list for package %q: %w"), importPath, err)
	}
	if strings.TrimSpace(item.Dir) == "" {
		return goListPackage{}, "", rplerr.Newf(
			localize.Text("grpc не смог определить директорию Go-пакета %q", "grpc could not determine directory for Go package %q"),
			importPath,
		)
	}

	return item, "", nil
}

func createGoListModule() (string, func(), error) {
	dir, err := os.MkdirTemp("", "rpl-grpc-go-list-*")
	if err != nil {
		return "", nil, err
	}

	goMod := "module rpl.local/grpcinspect\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}

	return dir, func() {
		_ = os.RemoveAll(dir)
	}, nil
}

func shouldFallbackGoList(detail string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(detail))
	if trimmed == "" {
		return false
	}

	return strings.Contains(trimmed, "go.mod file not found") ||
		strings.Contains(trimmed, "no required module provides package")
}

func mergeGoListDetails(primary string, fallback string) string {
	primary = strings.TrimSpace(primary)
	fallback = strings.TrimSpace(fallback)

	switch {
	case primary == "":
		return fallback
	case fallback == "":
		return primary
	case primary == fallback:
		return primary
	default:
		return primary + "\n" + fallback
	}
}

func grpcGoListProblem(importPath string, detail string) error {
	problem := rplerr.Newf(
		localize.Text("grpc не смог проанализировать Go-пакет %q", "grpc could not inspect Go package %q"),
		importPath,
	).WithHint(localize.Text(
		"Убедитесь, что пакет доступен в Go toolchain; если проект без `go.mod`, RPL попробует временный модуль для `go list`.",
		"Make sure the package is available to the Go toolchain; if the project has no `go.mod`, RPL will try a temporary module for `go list`.",
	))
	if trimmed := strings.TrimSpace(detail); trimmed != "" {
		problem.WithDetail(trimmed)
	}
	return problem
}

func discoverExportedMethods(pkg goListPackage, typeName string) ([]sdk.Method, error) {
	fset := gotoken.NewFileSet()
	files := append([]string(nil), pkg.GoFiles...)
	files = append(files, pkg.CgoFiles...)

	methods := make([]sdk.Method, 0)
	seen := make(map[string]struct{})
	for _, name := range files {
		filePath := filepath.Join(pkg.Dir, name)
		file, err := goparser.ParseFile(fset, filePath, nil, 0)
		if err != nil {
			return nil, fmt.Errorf(localize.Text("разбор Go-файла %q: %w", "parse Go file %q: %w"), filePath, err)
		}

		imports := fileImportMap(file)
		for _, decl := range file.Decls {
			fn, ok := decl.(*goast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name == nil || !fn.Name.IsExported() {
				continue
			}
			if !receiverMatches(fn.Recv, typeName) {
				continue
			}

			method, ok := goFuncToSDKMethod(fn, imports)
			if !ok {
				continue
			}
			if _, exists := seen[method.Name]; exists {
				continue
			}
			seen[method.Name] = struct{}{}
			methods = append(methods, method)
		}
	}

	return methods, nil
}

func fileImportMap(file *goast.File) map[string]string {
	items := make(map[string]string)
	if file == nil {
		return items
	}

	for _, item := range file.Imports {
		if item == nil || item.Path == nil {
			continue
		}

		pathValue := strings.Trim(item.Path.Value, `"`)
		if pathValue == "" {
			continue
		}

		alias := ""
		if item.Name != nil {
			alias = strings.TrimSpace(item.Name.Name)
		}
		if alias == "" {
			alias = pkgpath.Base(pathValue)
		}
		items[alias] = pathValue
	}

	return items
}

func receiverMatches(list *goast.FieldList, typeName string) bool {
	if list == nil || len(list.List) == 0 {
		return false
	}

	switch recv := list.List[0].Type.(type) {
	case *goast.Ident:
		return recv.Name == typeName
	case *goast.StarExpr:
		ident, ok := recv.X.(*goast.Ident)
		return ok && ident.Name == typeName
	default:
		return false
	}
}

func goFuncToSDKMethod(fn *goast.FuncDecl, imports map[string]string) (sdk.Method, bool) {
	method := sdk.Method{
		Name:    fn.Name.Name,
		Params:  make([]sdk.MethodParam, 0),
		Returns: make([]sdk.TypeRef, 0),
	}

	paramIndex := 1
	if fn.Type != nil && fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			typeRef, ok := goExprToTypeRef(field.Type, imports)
			if !ok {
				return sdk.Method{}, false
			}
			names := field.Names
			if len(names) == 0 {
				names = []*goast.Ident{{Name: fmt.Sprintf("arg%d", paramIndex)}}
			}
			for _, name := range names {
				paramName := fmt.Sprintf("arg%d", paramIndex)
				if name != nil && strings.TrimSpace(name.Name) != "" {
					paramName = name.Name
				}
				method.Params = append(method.Params, sdk.MethodParam{
					Name: paramName,
					Type: typeRef,
				})
				paramIndex++
			}
		}
	}

	if fn.Type != nil && fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			typeRef, ok := goExprToTypeRef(field.Type, imports)
			if !ok {
				return sdk.Method{}, false
			}

			count := 1
			if len(field.Names) > 0 {
				count = len(field.Names)
			}
			for i := 0; i < count; i++ {
				method.Returns = append(method.Returns, typeRef)
			}
		}
	}

	return method, true
}

func goExprToTypeRef(expr goast.Expr, imports map[string]string) (sdk.TypeRef, bool) {
	switch value := expr.(type) {
	case *goast.Ident:
		switch value.Name {
		case "string", "bool", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "byte", "float32", "float64", "error":
			return sdk.TypeRef{Name: value.Name}, true
		default:
			return sdk.TypeRef{}, false
		}
	case *goast.ArrayType:
		if value.Len != nil {
			return sdk.TypeRef{}, false
		}

		inner, ok := goExprToTypeRef(value.Elt, imports)
		if !ok {
			return sdk.TypeRef{}, false
		}
		inner.IsList = true
		inner.Optional = false
		return inner, inner.IsBytes()
	case *goast.SelectorExpr:
		pkgIdent, ok := value.X.(*goast.Ident)
		if !ok {
			return sdk.TypeRef{}, false
		}
		if imports[pkgIdent.Name] == "time" && value.Sel.Name == "Time" {
			return sdk.TypeRef{Name: "time.Time"}, true
		}
		return sdk.TypeRef{}, false
	default:
		return sdk.TypeRef{}, false
	}
}

func grpcMessageName(path []string) string {
	return strings.Join(path, "") + "Message"
}

func grpcToHelperName(path []string) string {
	return sdk.LowerCamel(path[0]) + "To" + strings.Join(path[1:], "") + "Message"
}

func grpcFromHelperName(path []string) string {
	return sdk.LowerCamel(path[0]) + "From" + strings.Join(path[1:], "") + "Message"
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == strings.TrimSpace(want) {
			return true
		}
	}

	return false
}
