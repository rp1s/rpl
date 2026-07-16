package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strings"
)

func renderServiceAdapter(plan *grpcPlan) string {
	if !plan.HasService {
		return ""
	}

	serverName := plan.RootModel.Name + "GRPCServer"
	serviceName := plan.ServiceName

	parts := []string{
		renderServiceInterface(plan, serviceName),
		renderServiceServerStruct(plan, serverName, serviceName),
		renderServiceConstructor(plan, serverName, serviceName),
		renderServiceRegister(plan, serverName, serviceName),
		renderServiceInterfaceAssertion(plan, serverName),
	}

	if plan.AutoPut {
		parts = append(parts, renderPutServerMethod(plan, serverName, serviceName))
	}
	if plan.AutoGetByID {
		parts = append(parts, renderGetByIDServerMethod(plan, serverName, serviceName))
	}
	if plan.AutoDelete {
		parts = append(parts, renderDeleteServerMethod(plan, serverName, serviceName))
	}
	if plan.AutoList {
		parts = append(parts, renderListServerMethod(plan, serverName, serviceName))
	}
	for _, method := range plan.ServiceMethods {
		parts = append(parts, renderServiceServerMethod(plan, serverName, serviceName, method))
	}

	return strings.Join(parts, "\n\n")
}

func renderServiceInterface(plan *grpcPlan, serviceName string) string {
	lines := make([]string, 0, len(plan.ServiceMethods)+4)
	rootType := grpcModelGoType(plan, plan.RootModel.Name)

	if plan.AutoPut {
		lines = append(lines, fmt.Sprintf("\tPut(ctx context.Context, %s %s) (%s, error)", grpcServiceModelParamName(plan), rootType, rootType))
	}
	if plan.AutoGetByID && plan.IDSubject != nil {
		lines = append(lines, fmt.Sprintf("\tGetByID(ctx context.Context, %s %s) (%s, error)", grpcServiceIDParamName(plan.IDSubject.IDField), plan.IDSubject.IDField.Type.GoString(), rootType))
	}
	if plan.AutoDelete && plan.IDSubject != nil {
		lines = append(lines, fmt.Sprintf("\tDelete(ctx context.Context, %s %s) error", grpcServiceIDParamName(plan.IDSubject.IDField), plan.IDSubject.IDField.Type.GoString()))
	}
	if plan.AutoList {
		lines = append(lines, fmt.Sprintf("\tList(ctx context.Context) ([]%s, error)", rootType))
	}
	for _, method := range plan.ServiceMethods {
		lines = append(lines, "\t"+renderServiceMethodSignature(plan, method))
	}

	return fmt.Sprintf("type %s interface {\n%s\n}", serviceName, strings.Join(lines, "\n"))
}

func renderServiceMethodSignature(plan *grpcPlan, method grpcInsideMethod) string {
	params := []string{"ctx context.Context"}
	if subject := grpcServiceSubjectSignature(plan, method.Subject); strings.TrimSpace(subject) != "" {
		params = append(params, subject)
	}
	for _, param := range method.Params {
		params = append(params, param.Name+" "+param.Type.GoString())
	}

	returns := grpcClientReturnTypeNames(method)
	if len(returns) == 1 {
		return fmt.Sprintf("%s(%s) %s", method.BridgeName, strings.Join(params, ", "), returns[0])
	}
	return fmt.Sprintf("%s(%s) (%s)", method.BridgeName, strings.Join(params, ", "), strings.Join(returns, ", "))
}

func renderServiceServerStruct(plan *grpcPlan, serverName string, serviceName string) string {
	return fmt.Sprintf("type %s struct {\n\tUnimplemented%sServer\n\tService %s\n}", serverName, plan.ServiceName, serviceName)
}

func renderServiceConstructor(plan *grpcPlan, serverName string, serviceName string) string {
	return sdk.WithDocComment(
		fmt.Sprintf("func New%s(service %s) *%s {\n\treturn &%s{Service: service}\n}", serverName, serviceName, serverName, serverName),
		"New%s создает gRPC server adapter для сервиса модели %s.",
		"New%s creates the gRPC server adapter for the %s model service.",
		serverName,
		plan.RootModel.Name,
	)
}

func renderServiceRegister(plan *grpcPlan, serverName string, serviceName string) string {
	return sdk.WithDocComment(
		fmt.Sprintf("func Register%s(registrar grpc.ServiceRegistrar, service %s) {\n\tRegister%sServer(registrar, New%s(service))\n}", plan.RootModel.Name+"GRPC", serviceName, plan.ServiceName, serverName),
		"Register%s регистрирует gRPC сервис модели %s в переданном registrar.",
		"Register%s registers the gRPC service for model %s on the provided registrar.",
		plan.RootModel.Name+"GRPC",
		plan.RootModel.Name,
	)
}

func renderServiceInterfaceAssertion(plan *grpcPlan, serverName string) string {
	return fmt.Sprintf("var _ %sServer = (*%s)(nil)", plan.ServiceName, serverName)
}

func renderPutServerMethod(plan *grpcPlan, serverName string, serviceName string) string {
	body := []string{
		renderServiceNilGuard("server == nil || server.Service == nil"),
		fmt.Sprintf("model, err := FromMessage(request)"),
		"if err != nil {\n\t\treturn nil, err\n\t}",
		fmt.Sprintf("result, err := server.Service.Put(ctx, %s)", grpcServiceModelParamName(plan)),
		"if err != nil {\n\t\treturn nil, err\n\t}",
		"return ToMessage(result), nil",
	}
	body[1] = fmt.Sprintf("%s, err := FromMessage(request)", grpcServiceModelParamName(plan))

	return sdk.WithDocComment(
		fmt.Sprintf("func (server *%s) Put(ctx context.Context, request *%s) (*%s, error) {\n\t%s\n}", serverName, plan.RootNode.MessageName, plan.RootNode.MessageName, strings.Join(body, "\n\t")),
		"Put проксирует gRPC вызов Put к сервису модели %s.",
		"Put forwards the Put gRPC call to the %s model service.",
		plan.RootModel.Name,
	)
}

func renderGetByIDServerMethod(plan *grpcPlan, serverName string, serviceName string) string {
	requestName := plan.RootModel.Name + "GetByIDRequest"
	subjectName := grpcServiceSubjectLocalName(*plan.IDSubject, plan)
	body := []string{
		renderServiceNilGuard("server == nil || server.Service == nil"),
		"if request == nil {\n\t\trequest = &" + requestName + "{}\n\t}",
	}
	body = append(body, renderDecodeValue(plan.IDSubject.IDValue, "request."+grpcGeneratedFieldName(plan.IDSubject.IDField.Name), subjectName)...)
	body = append(body,
		fmt.Sprintf("result, err := server.Service.GetByID(ctx, %s)", subjectName),
		"if err != nil {\n\t\treturn nil, err\n\t}",
		"return ToMessage(result), nil",
	)

	return sdk.WithDocComment(
		fmt.Sprintf("func (server *%s) GetByID(ctx context.Context, request *%s) (*%s, error) {\n\t%s\n}", serverName, requestName, plan.RootNode.MessageName, strings.Join(body, "\n\t")),
		"GetByID проксирует gRPC вызов GetByID к сервису модели %s.",
		"GetByID forwards the GetByID gRPC call to the %s model service.",
		plan.RootModel.Name,
	)
}

func renderDeleteServerMethod(plan *grpcPlan, serverName string, serviceName string) string {
	requestName := plan.RootModel.Name + "DeleteRequest"
	responseName := plan.RootModel.Name + "DeleteResponse"
	subjectName := grpcServiceSubjectLocalName(*plan.IDSubject, plan)
	body := []string{
		renderServiceNilGuard("server == nil || server.Service == nil"),
		"if request == nil {\n\t\trequest = &" + requestName + "{}\n\t}",
	}
	body = append(body, renderDecodeValue(plan.IDSubject.IDValue, "request."+grpcGeneratedFieldName(plan.IDSubject.IDField.Name), subjectName)...)
	body = append(body,
		fmt.Sprintf("if err := server.Service.Delete(ctx, %s); err != nil {\n\t\treturn nil, err\n\t}", subjectName),
		"return &"+responseName+"{}, nil",
	)

	return sdk.WithDocComment(
		fmt.Sprintf("func (server *%s) Delete(ctx context.Context, request *%s) (*%s, error) {\n\t%s\n}", serverName, requestName, responseName, strings.Join(body, "\n\t")),
		"Delete проксирует gRPC вызов Delete к сервису модели %s.",
		"Delete forwards the Delete gRPC call to the %s model service.",
		plan.RootModel.Name,
	)
}

func renderListServerMethod(plan *grpcPlan, serverName string, serviceName string) string {
	requestName := plan.RootModel.Name + "ListRequest"
	responseName := plan.RootModel.Name + "ListResponse"
	body := []string{
		renderServiceNilGuard("server == nil || server.Service == nil"),
		"items, err := server.Service.List(ctx)",
		"if err != nil {\n\t\treturn nil, err\n\t}",
		"response := &" + responseName + "{}",
		"if len(items) > 0 {\n\t\tresponse.Items = make([]*" + plan.RootNode.MessageName + ", 0, len(items))\n\t\tfor _, item := range items {\n\t\t\tresponse.Items = append(response.Items, ToMessage(item))\n\t\t}\n\t}",
		"return response, nil",
	}

	return sdk.WithDocComment(
		fmt.Sprintf("func (server *%s) List(ctx context.Context, request *%s) (*%s, error) {\n\t_ = request\n\t%s\n}", serverName, requestName, responseName, strings.Join(body, "\n\t")),
		"List проксирует gRPC вызов List к сервису модели %s.",
		"List forwards the List gRPC call to the %s model service.",
		plan.RootModel.Name,
	)
}

func renderServiceServerMethod(plan *grpcPlan, serverName string, serviceName string, method grpcInsideMethod) string {
	body := []string{
		renderServiceNilGuard("server == nil || server.Service == nil"),
		"if request == nil {\n\t\trequest = &" + method.RequestMessageName + "{}\n\t}",
	}
	body = append(body, renderServerSubjectDecode(plan, method)...)
	body = append(body, renderServerParamDecode(method)...)

	callArgs := []string{"ctx"}
	if subject := grpcServiceSubjectLocalName(method.Subject, plan); strings.TrimSpace(subject) != "" {
		callArgs = append(callArgs, subject)
	}
	for _, param := range method.Params {
		callArgs = append(callArgs, grpcInsideLocalName(param.Name))
	}
	call := "server.Service." + method.BridgeName + "(" + strings.Join(callArgs, ", ") + ")"

	if len(method.Results) == 0 {
		body = append(body,
			"if err := "+call+"; err != nil {\n\t\treturn nil, err\n\t}",
			"return &"+method.ResponseMessageName+"{}, nil",
		)
	} else {
		resultVars := make([]string, 0, len(method.Results))
		for _, result := range method.Results {
			resultVars = append(resultVars, grpcInsideLocalName(result.Name))
		}

		body = append(body, strings.Join(append(resultVars, "err"), ", ")+" := "+call)
		body = append(body, "if err != nil {\n\t\treturn nil, err\n\t}")
		body = append(body, "response := &"+method.ResponseMessageName+"{}")
		for _, result := range method.Results {
			body = append(body, renderEncodeValue(result, grpcInsideLocalName(result.Name), "response."+grpcGeneratedFieldName(result.Name))...)
		}
		body = append(body, "return response, nil")
	}

	return sdk.WithDocComment(
		fmt.Sprintf("func (server *%s) %s(ctx context.Context, request *%s) (*%s, error) {\n\t%s\n}", serverName, method.BridgeName, method.RequestMessageName, method.ResponseMessageName, strings.Join(body, "\n\t")),
		"%s проксирует gRPC вызов %s к сервису модели %s.",
		"%s forwards the %s gRPC call to the %s model service.",
		method.BridgeName,
		method.BridgeName,
		plan.RootModel.Name,
	)
}

func renderServiceClient(plan *grpcPlan) string {
	if !plan.HasService {
		return ""
	}

	serviceName := plan.ServiceName
	implName := sdk.LowerCamel(plan.RootModel.Name) + "GRPCClient"
	serviceClientName := plan.ServiceName + "Client"

	parts := []string{
		fmt.Sprintf("type %s struct {\n\tclient %s\n}", implName, serviceClientName),
		sdk.WithDocComment(
			fmt.Sprintf("func New%s(conn grpc.ClientConnInterface) %s {\n\treturn Wrap%s(New%sClient(conn))\n}", plan.RootModel.Name+"GRPCClient", serviceName, plan.RootModel.Name+"GRPCClient", plan.ServiceName),
			"New%s создает typed gRPC клиент для сервиса модели %s.",
			"New%s creates the typed gRPC client for the %s model service.",
			plan.RootModel.Name+"GRPCClient",
			plan.RootModel.Name,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func Wrap%s(client %s) %s {\n\treturn &%s{client: client}\n}", plan.RootModel.Name+"GRPCClient", serviceClientName, serviceName, implName),
			"Wrap%s оборачивает protobuf-клиент в сервисный API модели %s.",
			"Wrap%s wraps the protobuf client into the service API for model %s.",
			plan.RootModel.Name+"GRPCClient",
			plan.RootModel.Name,
		),
		fmt.Sprintf("var _ %s = (*%s)(nil)", serviceName, implName),
	}

	if plan.AutoPut {
		parts = append(parts, renderPutClientMethod(plan, implName))
	}
	if plan.AutoGetByID {
		parts = append(parts, renderGetByIDClientMethod(plan, implName))
	}
	if plan.AutoDelete {
		parts = append(parts, renderDeleteClientMethod(plan, implName))
	}
	if plan.AutoList {
		parts = append(parts, renderListClientMethod(plan, implName))
	}
	for _, method := range plan.ServiceMethods {
		parts = append(parts, renderServiceClientMethod(plan, implName, method))
	}

	return strings.Join(parts, "\n\n")
}

func renderPutClientMethod(plan *grpcPlan, clientName string) string {
	rootType := grpcModelGoType(plan, plan.RootModel.Name)
	paramName := grpcServiceModelParamName(plan)
	body := []string{
		renderClientNilGuard(fmt.Sprintf("return %s{}, fmt.Errorf(%q)", rootType, "grpc client is nil")),
		fmt.Sprintf("response, err := client.client.Put(ctx, ToMessage(%s))", paramName),
		"if err != nil {\n\t\treturn " + rootType + "{}, err\n\t}",
		"return FromMessage(response)",
	}

	return sdk.WithDocComment(
		fmt.Sprintf("func (client *%s) Put(ctx context.Context, %s %s) (%s, error) {\n\t%s\n}", clientName, paramName, rootType, rootType, strings.Join(body, "\n\t")),
		"Put вызывает удаленный gRPC метод Put для модели %s.",
		"Put calls the remote Put gRPC method for model %s.",
		plan.RootModel.Name,
	)
}

func renderGetByIDClientMethod(plan *grpcPlan, clientName string) string {
	rootType := grpcModelGoType(plan, plan.RootModel.Name)
	requestName := plan.RootModel.Name + "GetByIDRequest"
	paramName := grpcServiceIDParamName(plan.IDSubject.IDField)
	body := []string{
		renderClientNilGuard(fmt.Sprintf("return %s{}, fmt.Errorf(%q)", rootType, "grpc client is nil")),
		"request := &" + requestName + "{}",
	}
	body = append(body, renderEncodeValue(plan.IDSubject.IDValue, paramName, "request."+grpcGeneratedFieldName(plan.IDSubject.IDField.Name))...)
	body = append(body,
		"response, err := client.client.GetByID(ctx, request)",
		"if err != nil {\n\t\treturn "+rootType+"{}, err\n\t}",
		"return FromMessage(response)",
	)

	return sdk.WithDocComment(
		fmt.Sprintf("func (client *%s) GetByID(ctx context.Context, %s %s) (%s, error) {\n\t%s\n}", clientName, paramName, plan.IDSubject.IDField.Type.GoString(), rootType, strings.Join(body, "\n\t")),
		"GetByID вызывает удаленный gRPC метод GetByID для модели %s.",
		"GetByID calls the remote GetByID gRPC method for model %s.",
		plan.RootModel.Name,
	)
}

func renderDeleteClientMethod(plan *grpcPlan, clientName string) string {
	requestName := plan.RootModel.Name + "DeleteRequest"
	paramName := grpcServiceIDParamName(plan.IDSubject.IDField)
	body := []string{
		renderClientNilGuard(fmt.Sprintf("return fmt.Errorf(%q)", "grpc client is nil")),
		"request := &" + requestName + "{}",
	}
	body = append(body, renderEncodeValue(plan.IDSubject.IDValue, paramName, "request."+grpcGeneratedFieldName(plan.IDSubject.IDField.Name))...)
	body = append(body,
		"_, err := client.client.Delete(ctx, request)",
		"if err != nil {\n\t\treturn err\n\t}",
		"return nil",
	)

	return sdk.WithDocComment(
		fmt.Sprintf("func (client *%s) Delete(ctx context.Context, %s %s) error {\n\t%s\n}", clientName, paramName, plan.IDSubject.IDField.Type.GoString(), strings.Join(body, "\n\t")),
		"Delete вызывает удаленный gRPC метод Delete для модели %s.",
		"Delete calls the remote Delete gRPC method for model %s.",
		plan.RootModel.Name,
	)
}

func renderListClientMethod(plan *grpcPlan, clientName string) string {
	responseName := plan.RootModel.Name + "ListResponse"
	rootType := grpcModelGoType(plan, plan.RootModel.Name)
	body := []string{
		renderClientNilGuard(fmt.Sprintf("return nil, fmt.Errorf(%q)", "grpc client is nil")),
		fmt.Sprintf("response, err := client.client.List(ctx, &%sListRequest{})", plan.RootModel.Name),
		"if err != nil {\n\t\treturn nil, err\n\t}",
		"if response == nil {\n\t\tresponse = &" + responseName + "{}\n\t}",
		fmt.Sprintf("items := make([]%s, 0, len(response.Items))", rootType),
		"for _, item := range response.Items {\n\t\tvalue, err := FromMessage(item)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\titems = append(items, value)\n\t}",
		"return items, nil",
	}

	return sdk.WithDocComment(
		fmt.Sprintf("func (client *%s) List(ctx context.Context) ([]%s, error) {\n\t%s\n}", clientName, rootType, strings.Join(body, "\n\t")),
		"List вызывает удаленный gRPC метод List для модели %s.",
		"List calls the remote List gRPC method for model %s.",
		plan.RootModel.Name,
	)
}

func renderServiceClientMethod(plan *grpcPlan, clientName string, method grpcInsideMethod) string {
	body := []string{
		renderClientNilGuard(fmt.Sprintf("return %s", grpcClientZeroReturnValues(method))),
		"request := &" + method.RequestMessageName + "{}",
	}
	body = append(body, renderClientSubjectAssign(plan, method)...)
	body = append(body, renderClientRequestAssign(method)...)

	responseVar := "response"
	if len(method.Results) == 0 {
		responseVar = "_"
	}
	body = append(body, fmt.Sprintf("%s, err := client.client.%s(ctx, request)", responseVar, method.BridgeName))
	body = append(body, "if err != nil {\n\t\treturn "+grpcClientErrorReturn(method)+"\n\t}")

	if len(method.Results) == 0 {
		body = append(body, "return nil")
	} else {
		body = append(body, "if response == nil {\n\t\tresponse = &"+method.ResponseMessageName+"{}\n\t}")
		body = append(body, renderClientResponseAssign(method)...)
		body = append(body, "return "+grpcClientSuccessReturn(method))
	}

	return sdk.WithDocComment(
		fmt.Sprintf("func (client *%s) %s(ctx context.Context%s) %s {\n\t%s\n}", clientName, method.BridgeName, renderServiceMethodParamSignature(plan, method), renderClientReturnSignature(method), strings.Join(body, "\n\t")),
		"%s вызывает удаленный gRPC метод %s и возвращает обычные Go-значения.",
		"%s calls the remote %s gRPC method and returns regular Go values.",
		method.BridgeName,
		method.BridgeName,
	)
}

func renderServiceMethodParamSignature(plan *grpcPlan, method grpcInsideMethod) string {
	items := make([]string, 0, len(method.Params)+1)
	if subject := grpcServiceSubjectSignature(plan, method.Subject); strings.TrimSpace(subject) != "" {
		items = append(items, subject)
	}
	for _, param := range method.Params {
		items = append(items, param.Name+" "+param.Type.GoString())
	}
	if len(items) == 0 {
		return ""
	}
	return ", " + strings.Join(items, ", ")
}

func renderServerSubjectDecode(plan *grpcPlan, method grpcInsideMethod) []string {
	switch method.Subject.Mode {
	case grpcSubjectNone:
		return nil
	case grpcSubjectID:
		return renderDecodeValue(method.Subject.IDValue, "request."+grpcGeneratedFieldName(method.Subject.IDField.Name), grpcServiceSubjectLocalName(method.Subject, plan))
	default:
		return []string{
			fmt.Sprintf("%s, err := FromMessage(request.%s)", grpcServiceSubjectLocalName(method.Subject, plan), grpcGeneratedFieldName(plan.RootModel.Name)),
			"if err != nil {\n\t\treturn nil, err\n\t}",
		}
	}
}

func renderClientSubjectAssign(plan *grpcPlan, method grpcInsideMethod) []string {
	switch method.Subject.Mode {
	case grpcSubjectNone:
		return nil
	case grpcSubjectID:
		return renderEncodeValue(method.Subject.IDValue, grpcServiceSubjectLocalName(method.Subject, plan), "request."+grpcGeneratedFieldName(method.Subject.IDField.Name))
	default:
		return []string{
			fmt.Sprintf("request.%s = ToMessage(%s)", grpcGeneratedFieldName(plan.RootModel.Name), grpcServiceSubjectLocalName(method.Subject, plan)),
		}
	}
}

func renderServerParamDecode(method grpcInsideMethod) []string {
	lines := make([]string, 0, len(method.Params))
	for _, param := range method.Params {
		lines = append(lines, renderDecodeValue(param, "request."+grpcGeneratedFieldName(param.Name), grpcInsideLocalName(param.Name))...)
	}
	return lines
}

func renderClientRequestAssign(method grpcInsideMethod) []string {
	lines := make([]string, 0, len(method.Params))
	for _, param := range method.Params {
		lines = append(lines, renderEncodeValue(param, param.Name, "request."+grpcGeneratedFieldName(param.Name))...)
	}
	return lines
}

func renderClientResponseAssign(method grpcInsideMethod) []string {
	lines := make([]string, 0, len(method.Results))
	for _, result := range method.Results {
		lines = append(lines, renderDecodeValue(result, "response."+grpcGeneratedFieldName(result.Name), grpcInsideLocalName(result.Name))...)
	}
	return lines
}

func renderEncodeValue(value grpcInsideValue, source string, target string) []string {
	switch value.Kind {
	case "string", "bool", "bytes":
		return []string{target + " = " + source}
	case "int":
		return []string{target + " = int64(" + source + ")"}
	case "uint":
		return []string{target + " = uint64(" + source + ")"}
	case "float":
		return []string{target + " = float64(" + source + ")"}
	case "time":
		return []string{target + " = timestamppb.New(" + source + ")"}
	default:
		return nil
	}
}

func renderDecodeValue(value grpcInsideValue, source string, target string) []string {
	switch value.Kind {
	case "string", "bool", "bytes":
		return []string{target + " := " + source}
	case "int", "uint", "float":
		return []string{target + " := " + value.Type.BaseName() + "(" + source + ")"}
	case "time":
		return []string{
			"var " + target + " " + value.Type.GoString(),
			"if " + source + " != nil {\n\t\t" + target + " = " + source + ".AsTime()\n\t}",
		}
	default:
		return nil
	}
}

func renderClientNilGuard(line string) string {
	return "if client == nil || client.client == nil {\n\t\t" + line + "\n\t}"
}

func renderServiceNilGuard(condition string) string {
	return "if " + condition + " {\n\t\treturn nil, fmt.Errorf(\"grpc service is nil\")\n\t}"
}

func renderClientReturnSignature(method grpcInsideMethod) string {
	items := grpcClientReturnTypeNames(method)
	if len(items) == 1 {
		return items[0]
	}
	return "(" + strings.Join(items, ", ") + ")"
}

func grpcClientReturnTypeNames(method grpcInsideMethod) []string {
	items := make([]string, 0, len(method.Method.Returns)+1)
	for _, item := range method.Method.Returns {
		if item.IsError() {
			continue
		}
		items = append(items, item.GoString())
	}
	items = append(items, "error")
	return items
}

func grpcClientZeroReturnValues(method grpcInsideMethod) string {
	items := make([]string, 0, len(method.Method.Returns)+1)
	for _, item := range method.Method.Returns {
		if item.IsError() {
			continue
		}
		items = append(items, grpcZeroValue(item))
	}
	items = append(items, `fmt.Errorf("grpc client is nil")`)
	return strings.Join(items, ", ")
}

func grpcClientErrorReturn(method grpcInsideMethod) string {
	items := make([]string, 0, len(method.Method.Returns)+1)
	for _, item := range method.Method.Returns {
		if item.IsError() {
			continue
		}
		items = append(items, grpcZeroValue(item))
	}
	items = append(items, "err")
	return strings.Join(items, ", ")
}

func grpcClientSuccessReturn(method grpcInsideMethod) string {
	items := make([]string, 0, len(method.Results)+1)
	for _, result := range method.Results {
		items = append(items, grpcInsideLocalName(result.Name))
	}
	items = append(items, "nil")
	return strings.Join(items, ", ")
}

func grpcZeroValue(typeRef sdk.TypeRef) string {
	switch {
	case typeRef.Optional, typeRef.IsList, typeRef.IsBytes():
		return "nil"
	case typeRef.IsString():
		return `""`
	case typeRef.IsBool():
		return "false"
	case typeRef.IsInteger(), typeRef.IsFloat():
		return "0"
	case typeRef.IsTime():
		return "time.Time{}"
	case strings.TrimSpace(typeRef.Name) != "":
		return typeRef.BaseName() + "{}"
	default:
		return "nil"
	}
}

func grpcGeneratedFieldName(name string) string {
	parts := strings.Split(sdk.SnakeCase(name), "_")
	var builder strings.Builder
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		builder.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			builder.WriteString(part[1:])
		}
	}
	return builder.String()
}

func grpcInsideBridgeName(field sdk.Field, method sdk.Method) string {
	if strings.TrimSpace(field.Name) == "" {
		return method.Name
	}
	if method.Name == field.Name && len(method.Params) == 0 {
		return "Get" + field.Name
	}
	return field.Name + method.Name
}

func grpcInsideValueName(name string, index int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Sprintf("value%d", index)
	}
	return name
}

func grpcInsideResultName(field sdk.Field, method sdk.Method, index int) string {
	if strings.TrimSpace(field.Name) == "" {
		if index == 0 {
			return "result"
		}
		return fmt.Sprintf("result%d", index+1)
	}
	if method.Name == field.Name && len(method.Params) == 0 && index == 0 {
		return "value"
	}
	if index == 0 {
		return "result"
	}
	return fmt.Sprintf("result%d", index+1)
}

func grpcInsideLocalName(name string) string {
	return sdk.LowerCamel(name)
}

func grpcServiceSubjectSignature(plan *grpcPlan, subject grpcMethodSubject) string {
	switch subject.Mode {
	case grpcSubjectNone:
		return ""
	case grpcSubjectID:
		return grpcServiceIDParamName(subject.IDField) + " " + subject.IDField.Type.GoString()
	default:
		return grpcServiceModelParamName(plan) + " " + grpcModelGoType(plan, plan.RootModel.Name)
	}
}

func grpcServiceSubjectLocalName(subject grpcMethodSubject, plan *grpcPlan) string {
	switch subject.Mode {
	case grpcSubjectNone:
		return ""
	case grpcSubjectID:
		return grpcServiceIDParamName(subject.IDField)
	default:
		return grpcServiceModelParamName(plan)
	}
}

func grpcServiceModelParamName(plan *grpcPlan) string {
	if plan == nil {
		return "model"
	}
	return sdk.LowerCamel(plan.RootModel.Name)
}

func grpcServiceIDParamName(field sdk.Field) string {
	name := sdk.LowerCamel(field.Name)
	if strings.TrimSpace(name) == "" {
		return "id"
	}
	return name
}
