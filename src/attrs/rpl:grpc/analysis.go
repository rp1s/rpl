package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strings"
)

var grpcModelSpec = sdk.AttrSpec{
	Namespace: "grpc",
	Help:      localize.Text("На уровне модели grpc понимает `subject` и `model`: `subject` управляет auto-операциями Put/GetByID/Delete, а `model` включает instance-style subject для custom methods по умолчанию.", "At model level grpc understands `subject` and `model`: `subject` controls the auto Put/GetByID/Delete operations, while `model` enables instance-style subjects for custom methods by default."),
	Args: []sdk.AttrArgSpec{
		{Name: "subject", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "model", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}, Aliases: []string{"Model"}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@grpc", Insert: "@grpc", Help: localize.Text("Включает gRPC-генерацию для модели.", "Enables gRPC generation for the model.")},
		{Label: "@grpc.Model()", Insert: "@grpc.Model()", Help: localize.Text("Делает custom grpc methods instance-style по умолчанию и позволяет им наследовать subject model/id.", "Makes custom grpc methods instance-style by default and lets them inherit the model/id subject.")},
		{Label: "@grpc(subject: \"id\")", Insert: "@grpc(subject: \"id\")", Help: localize.Text("Переключает instance-операции на идентификатор модели.", "Switches instance operations to the model identifier.")},
	},
}

var grpcFieldSpec = sdk.AttrSpec{
	Namespace: "grpc",
	Help:      localize.Text("На уровне поля и метода grpc понимает mode, inside, ignore, subject, model и id.", "At field and method level grpc understands mode, inside, ignore, subject, model, and id."),
	Args: []sdk.AttrArgSpec{
		{Name: "inside", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}, Aliases: []string{"Inside"}},
		{Name: "id", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}},
		{Name: "model", Types: []sdk.AttrValueType{sdk.AttrValueTypeBool}, Aliases: []string{"Model"}},
		{Name: "mode", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
		{Name: "ignore", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike, sdk.AttrValueTypeBool}},
		{Name: "subject", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}},
	},
	Snippets: []sdk.AttrSnippetSpec{
		{Label: "@grpc", Insert: "@grpc", Help: localize.Text("Базовый grpc-атрибут.", "Base grpc attr.")},
		{Label: "@grpc.Model()", Insert: "@grpc.Model()", Help: localize.Text("Явно делает method или field-method instance-style и включает subject model/id.", "Explicitly makes a method or field-method instance-style and enables the model/id subject.")},
		{Label: "@grpc.id()", Insert: "@grpc.id()", Help: localize.Text("Отмечает поле как идентификатор модели и автоматически включает id-based grpc операции.", "Marks the field as the model identifier and automatically enables id-based grpc operations.")},
		{Label: "@grpc(id: true)", Insert: "@grpc(id: true)", Help: localize.Text("Отмечает поле как идентификатор модели для grpc subject=id.", "Marks the field as the model identifier for grpc subject=id.")},
		{Label: "@grpc.Inside()", Insert: "@grpc.Inside()", Help: localize.Text("Включает inside-режим для непереносимого поля.", "Enables inside mode for a non-transferable field.")},
	},
}

func analyzeGRPC(req sdk.GenerateRequest) (sdk.AnalyzeResponse, error) {
	builder := sdk.NewAnalyzeBuilder()

	modelResolved := builder.ValidateAttrSpec(req.Model.RuntimeAttrs, grpcModelSpec)
	validateGRPCSubjectValue(builder, modelRuntimeAttr(req.Model, "grpc"), modelResolved.ValueMap())
	validateGRPCModelSubjectConfig(builder, req.Model, modelResolved.ValueMap())
	for _, method := range req.Model.Methods {
		analyzeGRPCMethod(builder, req.Model, sdk.Field{}, method)
	}
	for _, field := range req.Model.Fields {
		analyzeGRPCField(builder, req.Model, field)
		for _, method := range field.Methods {
			analyzeGRPCMethod(builder, req.Model, field, method)
		}
	}

	plan, err := buildGRPCPlan(req)
	if err != nil {
		builder.AddDiagnostic(grpcDiagnostic(err))
		return builder.Response(), nil
	}

	scope := packageScope(req.File, "grpc")
	sdk.AddGeneratedClaimsInScope(builder, grpcCodeResponse(plan), scope)
	grpcClaimProtoArtifacts(builder, plan, scope)
	return builder.Response(), nil
}

func grpcCodeResponse(plan *grpcPlan) sdk.GenerateResponse {
	builder := sdk.NewCodeBuilder()
	builder.AddOrderedBlock("grpc.to.message", renderRootToMethod(plan), 10)
	builder.AddOrderedBlock("grpc.from.message", renderRootFromMethod(plan), 20)

	if helpers := renderNestedHelpers(plan); helpers != "" {
		builder.AddOrderedBlock("grpc.nested.helpers", helpers, 30)
	}
	if adapter := renderServiceAdapter(plan); adapter != "" {
		builder.AddOrderedBlock("grpc.service.adapter", adapter, 40)
	}

	return builder.Response()
}

func grpcClaimProtoArtifacts(builder *sdk.AnalyzeBuilder, plan *grpcPlan, scope string) {
	if builder == nil || plan == nil {
		return
	}

	stem := strings.TrimSuffix(plan.ProtoFileName, ".proto")
	builder.AddClaim("file", plan.ProtoFileName, scope)
	builder.AddClaim("file", stem+".pb.go", scope)
	if plan.HasService {
		builder.AddClaim("file", stem+"_grpc.pb.go", scope)
	}

	for _, node := range plan.Nodes {
		builder.AddClaim("identifier", node.MessageName, scope)
	}
	for _, method := range plan.InsideMethods {
		builder.AddClaim("identifier", method.RequestMessageName, scope)
		builder.AddClaim("identifier", method.ResponseMessageName, scope)
	}
	if plan.HasService {
		serviceName := strings.TrimSpace(plan.ServiceName)
		builder.AddClaim("identifier", serviceName+"Client", scope)
		builder.AddClaim("identifier", serviceName+"Server", scope)
		builder.AddClaim("identifier", "Unimplemented"+serviceName+"Server", scope)
		builder.AddClaim("identifier", "Unsafe"+serviceName+"Server", scope)
		builder.AddClaim("identifier", "Register"+serviceName+"Server", scope)
		builder.AddClaim("identifier", "New"+serviceName+"Client", scope)
	}
}

func analyzeGRPCField(builder *sdk.AnalyzeBuilder, model sdk.Model, field sdk.Field) {
	resolved := builder.ValidateAttrSpec(field.RuntimeAttrs, grpcFieldSpec)
	values := resolved.ValueMap()
	validateGRPCSubjectValue(builder, fieldRuntimeAttr(field, "grpc"), values)

	if field.IgnoredBy("grpc") && hasMeaningfulGRPCConfig(values) {
		builder.AddDiagnostic(sdk.DiagnosticAt(
			fieldRuntimeAttr(field, "grpc"),
			fmt.Sprintf(localize.Text("поле %q одновременно игнорирует и настраивает grpc", "field %q both ignores and configures grpc"), field.Name),
			localize.Text("Если поле нужно исключить из grpc, уберите остальные grpc-аргументы.", "If the field should be ignored by grpc, remove the rest of the grpc arguments."),
		))
	}

	validateGRPCFieldSubjectConfig(builder, model, field, values)
}

func analyzeGRPCMethod(builder *sdk.AnalyzeBuilder, model sdk.Model, field sdk.Field, method sdk.Method) {
	resolved := builder.ValidateAttrSpec(method.RuntimeAttrs, grpcFieldSpec)
	values := resolved.ValueMap()
	validateGRPCSubjectValue(builder, methodRuntimeAttr(method, "grpc"), values)
	validateGRPCMethodSubjectConfig(builder, model, field, method, values)
}

func grpcDiagnostic(err error) sdk.Diagnostic {
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

func hasMeaningfulGRPCConfig(values map[string]sdk.Value) bool {
	for name := range values {
		if name != "ignore" {
			return true
		}
	}
	return false
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

func validateGRPCSubjectValue(builder *sdk.AnalyzeBuilder, attr sdk.Attr, values map[string]sdk.Value) {
	if builder == nil {
		return
	}

	value, ok := values["subject"]
	if !ok {
		return
	}

	switch strings.ToLower(strings.TrimSpace(value.String())) {
	case "", "model", "id":
		return
	default:
		builder.AddDiagnostic(sdk.DiagnosticAt(
			attr,
			fmt.Sprintf(localize.Text("grpc subject должен быть `model` или `id`, а не %q", "grpc subject must be `model` or `id`, not %q"), value.String()),
			localize.Text("Используйте `@grpc(subject: \"model\")` или `@grpc(subject: \"id\")`.", "Use `@grpc(subject: \"model\")` or `@grpc(subject: \"id\")`."),
		))
	}
}

func validateGRPCModelSubjectConfig(builder *sdk.AnalyzeBuilder, model sdk.Model, values map[string]sdk.Value) {
	if builder == nil {
		return
	}

	if !grpcSubjectValuePresent(values) {
		return
	}

	if strings.TrimSpace(model.Name) == "" {
		return
	}

	if values["subject"].String() == string(grpcSubjectID) {
		if _, ok, _, err := grpcFindIDField(model); err != nil {
			builder.AddDiagnostic(grpcDiagnostic(err))
		} else if !ok {
			builder.AddDiagnostic(sdk.DiagnosticAt(
				modelRuntimeAttr(model, "grpc"),
				fmt.Sprintf(localize.Text("grpc subject=id требует идентификатор у модели %q", "grpc subject=id requires an identifier on model %q"), model.Name),
				localize.Text("Добавьте поле `Id` или пометьте нужное поле как `@grpc.id()`.", "Add an `Id` field or mark the correct field with `@grpc.id()`."),
			))
		}
	}
}

func validateGRPCFieldSubjectConfig(builder *sdk.AnalyzeBuilder, model sdk.Model, field sdk.Field, values map[string]sdk.Value) {
	if builder == nil {
		return
	}
	if !grpcSubjectValuePresent(values) {
		return
	}
	if grpcFieldUsesSubject(model, field) {
		return
	}

	builder.AddDiagnostic(sdk.DiagnosticAt(
		fieldRuntimeAttr(field, "grpc"),
		fmt.Sprintf(localize.Text("grpc subject у поля %q работает только вместе с `@grpc.Model()` или inside-режимом", "grpc subject on field %q only works together with `@grpc.Model()` or inside mode"), field.Name),
		localize.Text("Добавьте `@grpc.Model()` на поле, метод поля или на модель, если field-methods должны принимать model/id subject.", "Add `@grpc.Model()` on the field, on a field method, or on the model if field methods should receive the model/id subject."),
	))
}

func validateGRPCMethodSubjectConfig(builder *sdk.AnalyzeBuilder, model sdk.Model, field sdk.Field, method sdk.Method, values map[string]sdk.Value) {
	if builder == nil {
		return
	}
	if !grpcSubjectValuePresent(values) {
		return
	}
	if grpcMethodUsesSubject(model, field, method) {
		return
	}

	builder.AddDiagnostic(sdk.DiagnosticAt(
		methodRuntimeAttr(method, "grpc"),
		fmt.Sprintf(localize.Text("grpc subject у метода %q работает только вместе с `@grpc.Model()` или inside-режимом", "grpc subject on method %q only works together with `@grpc.Model()` or inside mode"), method.Name),
		localize.Text("Добавьте `@grpc.Model()` на метод, поле или модель, если метод должен принимать model/id subject.", "Add `@grpc.Model()` on the method, field, or model if the method should receive the model/id subject."),
	))
}

func grpcSubjectValuePresent(values map[string]sdk.Value) bool {
	if len(values) == 0 {
		return false
	}
	_, ok := values["subject"]
	return ok
}

func packageScope(file sdk.FileContext, parts ...string) string {
	base := strings.TrimSpace(file.GoPackagePath)
	if base == "" {
		base = strings.TrimSpace(file.PackageName)
	}
	items := make([]string, 0, len(parts)+1)
	if base != "" {
		items = append(items, base)
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return strings.Join(items, "/")
}
