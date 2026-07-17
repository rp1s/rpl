package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strings"
)

var localize = struct {
	Text func(string, string) string
}{
	Text: sdk.Text,
}

func generateTransport(req sdk.GenerateRequest) sdk.GenerateResponse {
	plan, err := buildTransportPlan(req)
	if err != nil || plan == nil {
		return sdk.GenerateResponse{}
	}
	return generateTransportResponse(plan)
}

func generateTransportResponse(plan *transportPlan) sdk.GenerateResponse {
	builder := sdk.NewCodeBuilder()
	addTransportCommonImports(builder, plan)
	builder.AddOrderedBlock("transport.protocol", renderTransportProtocol(plan), 10)
	builder.AddOrderedBlock("transport.service", renderTransportService(plan), 20)
	builder.AddOrderedBlock("transport.dispatch", renderTransportDispatch(plan), 30)
	if mode, ok := plan.mode(transportModeOSBin); ok {
		addTransportOSBinImports(builder)
		builder.AddOrderedBlock("transport.osbin.server", renderTransportServer(plan, mode), 40)
		builder.AddOrderedBlock("transport.osbin.client", renderTransportClient(plan, mode), 50)
	}

	body, err := sdk.RenderGoFile("transport", builder.Response())
	if err != nil || strings.TrimSpace(string(body)) == "" {
		return sdk.GenerateResponse{}
	}

	response := sdk.GenerateResponse{
		Files: []sdk.GeneratedFile{{
			Path:    "transport/transport.gen.go",
			Content: string(body),
		}},
	}
	for _, mode := range plan.Modes {
		var file sdk.GeneratedFile
		switch mode.Name {
		case transportModeHTTP:
			file = generateHTTPTransportFile(plan, mode)
		case transportModeUnix:
			file = generateUnixTransportFile(plan, mode)
		case transportModeNATS:
			file = generateNATSTransportFile(plan, mode)
		case transportModeKafka:
			file = generateKafkaTransportFile(plan, mode)
		case transportModeWebSocket:
			file = generateWebSocketTransportFile(plan, mode)
		}
		if strings.TrimSpace(file.Path) != "" && strings.TrimSpace(file.Content) != "" {
			response.Files = append(response.Files, file)
		}
	}
	return response
}

func addTransportCommonImports(builder *sdk.CodeBuilder, plan *transportPlan) {
	if builder == nil || plan == nil {
		return
	}
	builder.AddImport("context")
	builder.AddImport("encoding/json")
	builder.AddImport("fmt")
	builder.AddImport(plan.ModelImportPath, "modelpkg")
}

func addTransportOSBinImports(builder *sdk.CodeBuilder) {
	builder.AddImport("io")
	builder.AddImport("os")
	builder.AddImport("os/exec")
	builder.AddImport("strings")
	builder.AddImport("sync")
}

func renderTransportProtocol(plan *transportPlan) string {
	var builder strings.Builder
	builder.WriteString(sdk.WithDocComment(
		fmt.Sprintf("type %s struct {\n\tMethod string `json:\"method\"`\n\tPayload json.RawMessage `json:\"payload,omitempty\"`\n}", plan.EnvelopeName),
		"%s описывает transport-запрос для модели %s.",
		"%s describes the transport request envelope for model %s.",
		plan.EnvelopeName,
		plan.Model.Name,
	))
	builder.WriteString("\n\n")
	builder.WriteString(sdk.WithDocComment(
		fmt.Sprintf("type %s struct {\n\tResult json.RawMessage `json:\"result,omitempty\"`\n\tError string `json:\"error,omitempty\"`\n}", plan.ResponseName),
		"%s описывает transport-ответ для модели %s.",
		"%s describes the transport response envelope for model %s.",
		plan.ResponseName,
		plan.Model.Name,
	))

	for _, method := range plan.Methods {
		builder.WriteString("\n\n")
		builder.WriteString(renderTransportPayloadType(method.RequestTypeName, method.RequestFields, "request", method.Name))
		builder.WriteString("\n\n")
		builder.WriteString(renderTransportPayloadType(method.ResponseTypeName, method.ResultFields, "response", method.Name))
	}

	return builder.String()
}

func renderTransportPayloadType(name string, fields []transportValue, kind string, methodName string) string {
	var builder strings.Builder
	builder.WriteString(sdk.DocComment(
		"%s хранит %s payload для transport метода %s.",
		"%s stores the %s payload for transport method %s.",
		name,
		kind,
		methodName,
	))
	builder.WriteString("\n")
	builder.WriteString("type ")
	builder.WriteString(name)
	builder.WriteString(" struct {\n")
	for _, field := range fields {
		builder.WriteString("\t")
		builder.WriteString(exportTransportFieldName(field.Name))
		builder.WriteString(" ")
		builder.WriteString(field.Type.GoString())
		builder.WriteString(fmt.Sprintf(" `json:%q`\n", field.JSONName+",omitempty"))
	}
	builder.WriteString("}")
	return builder.String()
}

func renderTransportService(plan *transportPlan) string {
	lines := make([]string, 0, len(plan.Methods))
	for _, method := range plan.Methods {
		lines = append(lines, transportServiceSignature(method))
	}

	return sdk.WithDocComment(
		fmt.Sprintf("type %s interface {\n\t%s\n}", plan.ServiceName, strings.Join(lines, "\n\t")),
		"%s перечисляет transport-операции для модели %s.",
		"%s lists the transport operations for model %s.",
		plan.ServiceName,
		plan.Model.Name,
	)
}

func renderTransportServer(plan *transportPlan, mode transportModePlan) string {
	var builder strings.Builder

	builder.WriteString(sdk.WithDocComment(
		fmt.Sprintf("type %s struct {\n\tService %s\n}", plan.ServerName, plan.ServiceName),
		"%s оборачивает реализацию %s и обслуживает stdin/stdout transport.",
		"%s wraps a %s implementation and serves the stdin/stdout transport.",
		plan.ServerName,
		plan.ServiceName,
	))
	builder.WriteString("\n\n")
	builder.WriteString(sdk.WithDocComment(
		fmt.Sprintf("func New%s(service %s) *%s {\n\treturn &%s{Service: service}\n}", plan.ServerName, plan.ServiceName, plan.ServerName, plan.ServerName),
		"New%s создает transport server для модели %s.",
		"New%s creates the transport server for model %s.",
		plan.ServerName,
		plan.Model.Name,
	))
	builder.WriteString("\n\n")
	builder.WriteString(sdk.WithDocComment(
		fmt.Sprintf("func Run%s(service %s) error {\n\treturn New%s(service).Serve(os.Stdin, os.Stdout)\n}", plan.ServerName, plan.ServiceName, plan.ServerName),
		"Run%s запускает shell transport server модели %s на stdin/stdout.",
		"Run%s starts the shell transport server for model %s on stdin/stdout.",
		plan.ServerName,
		plan.Model.Name,
	))
	builder.WriteString("\n\n")
	builder.WriteString(sdk.WithDocComment(
		fmt.Sprintf("func (server *%s) Serve(reader io.Reader, writer io.Writer) error {\n\tif server == nil || server.Service == nil {\n\t\treturn fmt.Errorf(%q)\n\t}\n\n\tdecoder := json.NewDecoder(reader)\n\tencoder := json.NewEncoder(writer)\n\tfor {\n\t\tvar envelope %s\n\t\tif err := decoder.Decode(&envelope); err != nil {\n\t\t\tif err == io.EOF {\n\t\t\t\treturn nil\n\t\t\t}\n\t\t\treturn err\n\t\t}\n\n\t\tresponse, err := %s(server.Service, context.Background(), %q, envelope)\n\t\tif err != nil {\n\t\t\tresponse = %s{Error: err.Error()}\n\t\t}\n\t\tif err := encoder.Encode(response); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n}", plan.ServerName, "transport server requires a configured service", plan.EnvelopeName, transportDispatchName(plan), mode.Name, plan.ResponseName),
		"Serve читает transport-запросы для модели %s из reader и пишет ответы в writer.",
		"Serve reads transport requests for model %s from reader and writes responses to writer.",
		plan.Model.Name,
	))
	builder.WriteString("\n\n")
	return builder.String()
}

func renderTransportDispatch(plan *transportPlan) string {
	var builder strings.Builder
	builder.WriteString(renderTransportModeAllowed(plan))
	builder.WriteString("\n\n")
	builder.WriteString(fmt.Sprintf("func %s(service %s, ctx context.Context, mode string, envelope %s) (%s, error) {\n\tif service == nil {\n\t\treturn %s{}, fmt.Errorf(\"transport service is nil\")\n\t}\n\tif !%s(mode, envelope.Method) {\n\t\treturn %s{}, fmt.Errorf(\"transport method %%q is not enabled for mode %%q\", envelope.Method, mode)\n\t}\n\tswitch envelope.Method {\n", transportDispatchName(plan), plan.ServiceName, plan.EnvelopeName, plan.ResponseName, plan.ResponseName, transportAllowedName(plan), plan.ResponseName))
	for _, method := range plan.Methods {
		builder.WriteString(fmt.Sprintf("\tcase %q:\n", method.Name))
		builder.WriteString(indentTransport(renderTransportDispatchCase(plan, method), "\t\t"))
		builder.WriteString("\n")
	}
	builder.WriteString("\tdefault:\n")
	builder.WriteString(fmt.Sprintf("\t\treturn %s{}, fmt.Errorf(\"unknown transport method %%q\", envelope.Method)\n", plan.ResponseName))
	builder.WriteString("\t}\n}")
	return builder.String()
}

func renderTransportModeAllowed(plan *transportPlan) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("func %s(mode string, method string) bool {\n\tswitch mode {\n", transportAllowedName(plan)))
	for _, mode := range plan.Modes {
		builder.WriteString(fmt.Sprintf("\tcase %q:\n\t\tswitch method {\n", mode.Name))
		for _, method := range mode.Methods {
			builder.WriteString(fmt.Sprintf("\t\tcase %q:\n\t\t\treturn true\n", method.Name))
		}
		builder.WriteString("\t\t}\n")
	}
	builder.WriteString("\t}\n\treturn false\n}")
	return builder.String()
}

func transportDispatchName(plan *transportPlan) string {
	return sdk.LowerCamel(plan.Model.Name) + "TransportDispatch"
}

func transportAllowedName(plan *transportPlan) string {
	return sdk.LowerCamel(plan.Model.Name) + "TransportMethodAllowed"
}

func renderTransportDispatchCase(plan *transportPlan, method transportMethodPlan) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("var payload %s\n", method.RequestTypeName))
	builder.WriteString("if len(envelope.Payload) > 0 {\n")
	builder.WriteString(fmt.Sprintf("\tif err := json.Unmarshal(envelope.Payload, &payload); err != nil {\n\t\treturn %s{}, err\n\t}\n", plan.ResponseName))
	builder.WriteString("}\n")

	call := "service." + method.Name + "(ctx"
	for _, arg := range method.CallArgs {
		call += ", " + arg
	}
	call += ")"

	switch len(method.SignatureReturns) {
	case 0:
		builder.WriteString(fmt.Sprintf("if err := %s; err != nil {\n\treturn %s{}, err\n}\n", call, plan.ResponseName))
		builder.WriteString(fmt.Sprintf("body, err := json.Marshal(%s{})\n", method.ResponseTypeName))
		builder.WriteString("if err != nil {\n\treturn " + plan.ResponseName + "{}, err\n}\n")
		builder.WriteString(fmt.Sprintf("return %s{Result: body}, nil", plan.ResponseName))
	case 1:
		builder.WriteString(fmt.Sprintf("result, err := %s\n", call))
		builder.WriteString(fmt.Sprintf("if err != nil {\n\treturn %s{}, err\n}\n", plan.ResponseName))
		builder.WriteString(fmt.Sprintf("body, err := json.Marshal(%s{%s: result})\n", method.ResponseTypeName, exportTransportFieldName(method.ResultFields[0].Name)))
		builder.WriteString("if err != nil {\n\treturn " + plan.ResponseName + "{}, err\n}\n")
		builder.WriteString(fmt.Sprintf("return %s{Result: body}, nil", plan.ResponseName))
	default:
		results := make([]string, 0, len(method.SignatureReturns))
		for index := range method.SignatureReturns {
			results = append(results, fmt.Sprintf("result%d", index+1))
		}
		builder.WriteString(fmt.Sprintf("%s, err := %s\n", strings.Join(results, ", "), call))
		builder.WriteString(fmt.Sprintf("if err != nil {\n\treturn %s{}, err\n}\n", plan.ResponseName))
		assignments := make([]string, 0, len(results))
		for index, result := range results {
			assignments = append(assignments, fmt.Sprintf("%s: %s", exportTransportFieldName(method.ResultFields[index].Name), result))
		}
		builder.WriteString(fmt.Sprintf("body, err := json.Marshal(%s{%s})\n", method.ResponseTypeName, strings.Join(assignments, ", ")))
		builder.WriteString("if err != nil {\n\treturn " + plan.ResponseName + "{}, err\n}\n")
		builder.WriteString(fmt.Sprintf("return %s{Result: body}, nil", plan.ResponseName))
	}
	return builder.String()
}

func renderTransportClient(plan *transportPlan, mode transportModePlan) string {
	var builder strings.Builder
	builder.WriteString(sdk.WithDocComment(
		fmt.Sprintf("type %s struct {\n\tcmd *exec.Cmd\n\tstdin io.WriteCloser\n\tstdout io.ReadCloser\n\tencoder *json.Encoder\n\tdecoder *json.Decoder\n\tmu sync.Mutex\n}", plan.ClientName),
		"%s вызывает transport-методы модели %s через дочерний процесс.",
		"%s calls the transport methods of model %s through a child process.",
		plan.ClientName,
		plan.Model.Name,
	))
	builder.WriteString("\n\n")
	builder.WriteString(sdk.WithDocComment(
		fmt.Sprintf("func New%s(command string, args ...string) (*%s, error) {\n\tif strings.TrimSpace(command) == \"\" {\n\t\treturn nil, fmt.Errorf(%q)\n\t}\n\tcmd := exec.Command(command, args...)\n\tcmd.Stderr = os.Stderr\n\tstdin, err := cmd.StdinPipe()\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tstdout, err := cmd.StdoutPipe()\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tif err := cmd.Start(); err != nil {\n\t\treturn nil, err\n\t}\n\treturn &%s{cmd: cmd, stdin: stdin, stdout: stdout, encoder: json.NewEncoder(stdin), decoder: json.NewDecoder(stdout)}, nil\n}", plan.ClientName, plan.ClientName, "transport command is required", plan.ClientName),
		"New%s запускает transport server модели %s как дочерний процесс.",
		"New%s starts the transport server for model %s as a child process.",
		plan.ClientName,
		plan.Model.Name,
	))
	builder.WriteString("\n\n")
	builder.WriteString(sdk.WithDocComment(
		fmt.Sprintf("func (client *%s) Close() error {\n\tif client == nil {\n\t\treturn nil\n\t}\n\tif client.stdin != nil {\n\t\t_ = client.stdin.Close()\n\t}\n\tif client.cmd == nil {\n\t\treturn nil\n\t}\n\treturn client.cmd.Wait()\n}", plan.ClientName),
		"Close завершает transport client модели %s и дожидается дочернего процесса.",
		"Close stops the transport client for model %s and waits for the child process.",
		plan.Model.Name,
	))
	builder.WriteString("\n\n")
	builder.WriteString(renderTransportClientRoundTrip(plan))

	for _, method := range mode.Methods {
		builder.WriteString("\n\n")
		builder.WriteString(renderTransportClientMethod(plan, method))
	}

	return builder.String()
}

func renderTransportClientRoundTrip(plan *transportPlan) string {
	return sdk.WithDocComment(
		fmt.Sprintf("func (client *%s) roundTrip(ctx context.Context, method string, request any, response any) error {\n\tif client == nil {\n\t\treturn fmt.Errorf(%q)\n\t}\n\tif err := ctx.Err(); err != nil {\n\t\treturn err\n\t}\n\n\tpayload, err := json.Marshal(request)\n\tif err != nil {\n\t\treturn err\n\t}\n\n\tclient.mu.Lock()\n\tdefer client.mu.Unlock()\n\tif err := client.encoder.Encode(%s{Method: method, Payload: payload}); err != nil {\n\t\treturn err\n\t}\n\n\tvar envelope %s\n\tif err := client.decoder.Decode(&envelope); err != nil {\n\t\treturn err\n\t}\n\tif strings.TrimSpace(envelope.Error) != \"\" {\n\t\treturn fmt.Errorf(\"transport %%s: %%s\", method, envelope.Error)\n\t}\n\tif response == nil || len(envelope.Result) == 0 {\n\t\treturn nil\n\t}\n\treturn json.Unmarshal(envelope.Result, response)\n}", plan.ClientName, "transport client is nil", plan.EnvelopeName, plan.ResponseName),
		"roundTrip отправляет transport-запрос модели %s и читает ответ из shell-процесса.",
		"roundTrip sends a transport request for model %s and reads the response from the shell process.",
		plan.Model.Name,
	)
}

func renderTransportClientMethod(plan *transportPlan, method transportMethodPlan) string {
	return renderTransportTypedClientMethod(plan, method, plan.ClientName, "roundTrip")
}

func renderTransportTypedClientMethods(plan *transportPlan, methods []transportMethodPlan, receiverName string, roundTripMethod string) string {
	items := make([]string, 0, len(methods))
	for _, method := range methods {
		items = append(items, renderTransportTypedClientMethod(plan, method, receiverName, roundTripMethod))
	}
	return strings.Join(items, "\n\n")
}

func renderTransportTypedClientMethod(plan *transportPlan, method transportMethodPlan, receiverName string, roundTripMethod string) string {
	argsSig := transportClientArgsSignature(method)
	returnsSig := transportClientReturnsSignature(method)
	zeroReturn := transportClientErrorReturn(method)

	var builder strings.Builder
	builder.WriteString(sdk.DocComment(
		"%s вызывает transport-метод %s модели %s.",
		"%s calls the transport method %s of model %s.",
		method.Name,
		method.Name,
		plan.Model.Name,
	))
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("func (client *%s) %s(ctx context.Context%s)%s {\n", receiverName, method.Name, argsSig, returnsSig))
	builder.WriteString(fmt.Sprintf("\trequest := %s{%s}\n", method.RequestTypeName, transportClientRequestAssignments(method)))

	if len(method.ResultFields) > 0 {
		builder.WriteString(fmt.Sprintf("\tvar response %s\n", method.ResponseTypeName))
		builder.WriteString(fmt.Sprintf("\tif err := client.%s(ctx, %q, request, &response); err != nil {\n\t\t%s\n\t}\n", roundTripMethod, method.Name, zeroReturn))
		builder.WriteString("\treturn ")
		builder.WriteString(transportClientResponseReturns(method))
		builder.WriteString("\n}")
		return builder.String()
	}

	builder.WriteString(fmt.Sprintf("\tif err := client.%s(ctx, %q, request, nil); err != nil {\n\t\t%s\n\t}\n", roundTripMethod, method.Name, zeroReturn))
	if len(method.SignatureReturns) == 0 {
		builder.WriteString("\treturn nil\n}")
		return builder.String()
	}
	builder.WriteString("\treturn nil\n}")
	return builder.String()
}

func transportServiceSignature(method transportMethodPlan) string {
	return fmt.Sprintf("%s(ctx context.Context%s)%s", method.Name, transportClientArgsSignature(method), transportClientReturnsSignature(method))
}

func transportClientArgsSignature(method transportMethodPlan) string {
	if len(method.SignatureArgs) == 0 {
		return ""
	}
	items := make([]string, 0, len(method.SignatureArgs))
	for _, arg := range method.SignatureArgs {
		items = append(items, arg.Name+" "+arg.Type.GoString())
	}
	return ", " + strings.Join(items, ", ")
}

func transportClientReturnsSignature(method transportMethodPlan) string {
	items := make([]string, 0, len(method.SignatureReturns)+1)
	for _, item := range method.SignatureReturns {
		items = append(items, item.GoString())
	}
	items = append(items, "error")
	if len(items) == 1 {
		return " error"
	}
	if len(items) == 2 {
		return " (" + items[0] + ", error)"
	}
	return " (" + strings.Join(items, ", ") + ")"
}

func transportClientErrorReturn(method transportMethodPlan) string {
	if len(method.SignatureReturns) == 0 {
		return "return err"
	}
	items := make([]string, 0, len(method.SignatureReturns)+1)
	for _, item := range method.SignatureReturns {
		items = append(items, goZeroValue(item))
	}
	items = append(items, "err")
	return "return " + strings.Join(items, ", ")
}

func transportClientResponseReturns(method transportMethodPlan) string {
	items := make([]string, 0, len(method.ResultFields)+1)
	for _, item := range method.ResultFields {
		items = append(items, "response."+exportTransportFieldName(item.Name))
	}
	items = append(items, "nil")
	return strings.Join(items, ", ")
}

func transportClientRequestAssignments(method transportMethodPlan) string {
	if len(method.RequestFields) == 0 {
		return ""
	}
	items := make([]string, 0, len(method.RequestFields))
	for _, field := range method.RequestFields {
		items = append(items, fmt.Sprintf("%s: %s", exportTransportFieldName(field.Name), field.Name))
	}
	return strings.Join(items, ", ")
}

func exportTransportFieldName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "Value"
	}
	return strings.ToUpper(trimmed[:1]) + trimmed[1:]
}

func indentTransport(text string, prefix string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func goZeroValue(typeRef sdk.TypeRef) string {
	switch {
	case typeRef.IsList:
		return "nil"
	case typeRef.Optional:
		return "nil"
	case typeRef.IsString():
		return `""`
	case typeRef.IsBool():
		return "false"
	case typeRef.IsNumeric():
		return "0"
	case typeRef.IsError():
		return "nil"
	default:
		return typeRef.GoString() + "{}"
	}
}
