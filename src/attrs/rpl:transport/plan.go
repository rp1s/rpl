package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strings"
)

const (
	transportModeOSBin     = "os.bin"
	transportModeHTTP      = "http"
	transportModeUnix      = "unix"
	transportModeNATS      = "nats"
	transportModeKafka     = "kafka"
	transportModeWebSocket = "websocket"
)

var transportSupportedModes = []string{
	transportModeOSBin,
	transportModeHTTP,
	transportModeUnix,
	transportModeNATS,
	transportModeKafka,
	transportModeWebSocket,
}

type transportSubjectMode string

const (
	transportSubjectNone  transportSubjectMode = "none"
	transportSubjectModel transportSubjectMode = "model"
	transportSubjectID    transportSubjectMode = "id"
)

type transportPlan struct {
	Model           sdk.Model
	ModelImportPath string
	ClientName      string
	ServerName      string
	ServiceName     string
	EnvelopeName    string
	ResponseName    string
	Methods         []transportMethodPlan
	Modes           []transportModePlan
	HTTPBasePath    string
	BrokerPrefix    string
	KafkaGroup      string
}

type transportModePlan struct {
	Name    string
	Methods []transportMethodPlan
}

type transportMethodPlan struct {
	Name             string
	RequestTypeName  string
	ResponseTypeName string
	RequestFields    []transportValue
	ResultFields     []transportValue
	CallArgs         []string
	SignatureArgs    []transportValue
	SignatureReturns []sdk.TypeRef
}

type transportValue struct {
	Name     string
	Type     sdk.TypeRef
	JSONName string
}

func buildTransportPlan(req sdk.GenerateRequest) (*transportPlan, error) {
	modelModes, err := transportModelModes(req.Model)
	if err != nil {
		return nil, err
	}
	modelEnabled := transportModelEnabled(req.Model)
	activeMethods := transportActiveModelMethods(req.Model)
	if !modelEnabled && len(activeMethods) == 0 {
		return nil, nil
	}

	idField, hasID, err := transportFindIDField(req.Model)
	if err != nil {
		return nil, err
	}

	plan := &transportPlan{
		Model:           req.Model,
		ModelImportPath: transportModelImportPath(req.File),
		ClientName:      req.Model.Name + "TransportClient",
		ServerName:      req.Model.Name + "TransportServer",
		ServiceName:     req.Model.Name + "TransportService",
		EnvelopeName:    sdk.LowerCamel(req.Model.Name) + "TransportEnvelope",
		ResponseName:    sdk.LowerCamel(req.Model.Name) + "TransportResponse",
		Methods:         make([]transportMethodPlan, 0),
		Modes:           make([]transportModePlan, 0),
		HTTPBasePath:    transportModelSetting(req.Model, "httpPath", "/rpl/"+sdk.SnakeCase(req.Model.Name)),
		BrokerPrefix:    transportModelSetting(req.Model, "brokerPrefix", "rpl."+strings.ToLower(req.Model.Name)),
		KafkaGroup:      transportModelSetting(req.Model, "kafkaGroup", "rpl-"+strings.ToLower(req.Model.Name)),
	}
	modeIndexes := make(map[string]int)
	methodIndexes := make(map[string]int)
	ensureMode := func(mode string) int {
		if index, ok := modeIndexes[mode]; ok {
			return index
		}
		index := len(plan.Modes)
		modeIndexes[mode] = index
		plan.Modes = append(plan.Modes, transportModePlan{Name: mode, Methods: make([]transportMethodPlan, 0)})
		return index
	}
	addMethod := func(mode string, method transportMethodPlan) {
		modeIndex := ensureMode(mode)
		for _, existing := range plan.Modes[modeIndex].Methods {
			if existing.Name == method.Name {
				return
			}
		}
		plan.Modes[modeIndex].Methods = append(plan.Modes[modeIndex].Methods, method)
		if _, ok := methodIndexes[method.Name]; !ok {
			methodIndexes[method.Name] = len(plan.Methods)
			plan.Methods = append(plan.Methods, method)
		}
	}

	if modelEnabled {
		for _, mode := range modelModes {
			ensureMode(mode)
			for _, method := range transportAutoMethods(req.Model, hasID, idField) {
				addMethod(mode, method)
			}
		}
	}

	for _, method := range activeMethods {
		modes, err := transportModesForMethod(method, modelModes, modelEnabled)
		if err != nil {
			return nil, err
		}
		item, err := buildTransportCustomMethodPlan(req.Model, method, hasID, idField)
		if err != nil {
			return nil, err
		}
		for _, mode := range modes {
			addMethod(mode, item)
		}
	}

	return plan, nil
}

func transportModelSetting(model sdk.Model, name string, fallback string) string {
	attrs := append(append([]sdk.Attr(nil), model.RuntimeAttrs...), model.Attrs...)
	for _, attr := range attrs {
		if !attr.Matches("transport") && attr.Namespace() != "transport" {
			continue
		}
		if value, ok := attr.Named(name); ok && strings.TrimSpace(value.String()) != "" {
			return strings.TrimSpace(value.String())
		}
	}
	return fallback
}

func transportModelImportPath(file sdk.FileContext) string {
	if strings.TrimSpace(file.GoPackagePath) != "" {
		return strings.TrimSpace(file.GoPackagePath)
	}
	return ".."
}

func transportModelEnabled(model sdk.Model) bool {
	_, ok := model.ResolvedAttr("transport")
	return ok
}

func transportActiveModelMethods(model sdk.Model) []sdk.Method {
	methods := make([]sdk.Method, 0)
	modelEnabled := transportModelEnabled(model)
	for _, method := range model.Methods {
		if transportMethodIgnored(method) {
			continue
		}
		if modelEnabled || method.HasRuntimeAffinity("transport") {
			methods = append(methods, method)
		}
	}
	return methods
}

func transportModelModes(model sdk.Model) ([]string, error) {
	if !transportModelEnabled(model) {
		return nil, nil
	}
	modes, err := transportExplicitModes(append(append([]sdk.Attr(nil), model.RuntimeAttrs...), model.Attrs...))
	if err != nil {
		return nil, err
	}
	if len(modes) == 0 {
		return []string{transportModeOSBin}, nil
	}
	return modes, nil
}

func transportModesForMethod(method sdk.Method, modelModes []string, modelEnabled bool) ([]string, error) {
	explicit, err := transportExplicitModes(append(append([]sdk.Attr(nil), method.RuntimeAttrs...), method.Attrs...))
	if err != nil {
		return nil, err
	}
	if len(explicit) > 0 {
		return explicit, nil
	}
	if modelEnabled && len(modelModes) > 0 {
		return append([]string(nil), modelModes...), nil
	}
	return []string{transportModeOSBin}, nil
}

func transportExplicitModes(attrs []sdk.Attr) ([]string, error) {
	items := make([]string, 0)
	seen := make(map[string]struct{})
	for _, attr := range attrs {
		if !attr.Matches("transport") && attr.Namespace() != "transport" {
			continue
		}
		mode := ""
		if len(attr.Args) > 0 && strings.TrimSpace(attr.SubName()) == "" {
			mode = attr.Args[0].String()
		}
		if named, ok := attr.Named("mode"); ok {
			mode = named.String()
		}
		if strings.TrimSpace(mode) == "" {
			continue
		}
		normalized, err := normalizeTransportMode(mode)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		items = append(items, normalized)
	}
	return items, nil
}

func normalizeTransportMode(mode string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch normalized {
	case "ws", "web-socket":
		normalized = transportModeWebSocket
	}
	for _, supported := range transportSupportedModes {
		if normalized == supported {
			return normalized, nil
		}
	}
	return "", sdk.NewErrorf(localize.Text("transport mode %q не поддерживается", "transport mode %q is not supported"), mode).
		WithHint(localize.Text("Используйте `os.bin`, `http`, `unix`, `nats`, `kafka` или `websocket`.", "Use `os.bin`, `http`, `unix`, `nats`, `kafka`, or `websocket`."))
}

func (plan *transportPlan) mode(name string) (transportModePlan, bool) {
	if plan == nil {
		return transportModePlan{}, false
	}
	for _, mode := range plan.Modes {
		if mode.Name == name {
			return mode, true
		}
	}
	return transportModePlan{}, false
}

func transportAutoMethods(model sdk.Model, hasID bool, idField sdk.Field) []transportMethodPlan {
	modelType := sdk.TypeRef{Name: "modelpkg." + model.Name}
	methods := []transportMethodPlan{
		{
			Name:             "Put",
			RequestTypeName:  model.Name + "TransportPutRequest",
			ResponseTypeName: model.Name + "TransportPutResponse",
			RequestFields: []transportValue{{
				Name:     sdk.LowerCamel(model.Name),
				Type:     modelType,
				JSONName: sdk.SnakeCase(model.Name),
			}},
			ResultFields: []transportValue{{
				Name:     "Result",
				Type:     modelType,
				JSONName: "result",
			}},
			SignatureArgs: []transportValue{{
				Name: sdk.LowerCamel(model.Name),
				Type: modelType,
			}},
			SignatureReturns: []sdk.TypeRef{modelType},
			CallArgs: []string{
				"payload." + exportTransportFieldName(sdk.LowerCamel(model.Name)),
			},
		},
		{
			Name:             "List",
			RequestTypeName:  model.Name + "TransportListRequest",
			ResponseTypeName: model.Name + "TransportListResponse",
			ResultFields: []transportValue{{
				Name:     "Items",
				Type:     sdk.TypeRef{Name: "modelpkg." + model.Name, IsList: true},
				JSONName: "items",
			}},
			SignatureReturns: []sdk.TypeRef{{Name: "modelpkg." + model.Name, IsList: true}},
		},
	}

	if hasID {
		idType := idField.Type
		methods = append(methods,
			transportMethodPlan{
				Name:             "GetByID",
				RequestTypeName:  model.Name + "TransportGetByIDRequest",
				ResponseTypeName: model.Name + "TransportGetByIDResponse",
				RequestFields: []transportValue{{
					Name:     sdk.LowerCamel(idField.Name),
					Type:     idType,
					JSONName: sdk.SnakeCase(idField.Name),
				}},
				ResultFields: []transportValue{{
					Name:     "Result",
					Type:     modelType,
					JSONName: "result",
				}},
				SignatureArgs: []transportValue{{
					Name: sdk.LowerCamel(idField.Name),
					Type: idType,
				}},
				SignatureReturns: []sdk.TypeRef{modelType},
				CallArgs: []string{
					"payload." + exportTransportFieldName(sdk.LowerCamel(idField.Name)),
				},
			},
			transportMethodPlan{
				Name:             "Delete",
				RequestTypeName:  model.Name + "TransportDeleteRequest",
				ResponseTypeName: model.Name + "TransportDeleteResponse",
				RequestFields: []transportValue{{
					Name:     sdk.LowerCamel(idField.Name),
					Type:     idType,
					JSONName: sdk.SnakeCase(idField.Name),
				}},
				SignatureArgs: []transportValue{{
					Name: sdk.LowerCamel(idField.Name),
					Type: idType,
				}},
				CallArgs: []string{
					"payload." + exportTransportFieldName(sdk.LowerCamel(idField.Name)),
				},
			},
		)
	}

	return methods
}

func buildTransportCustomMethodPlan(model sdk.Model, method sdk.Method, hasID bool, idField sdk.Field) (transportMethodPlan, error) {
	subject, err := transportMethodSubject(model, method, hasID, idField)
	if err != nil {
		return transportMethodPlan{}, err
	}

	requestFields := make([]transportValue, 0)
	signatureArgs := make([]transportValue, 0)
	callArgs := make([]string, 0)

	switch subject {
	case transportSubjectModel:
		value := transportValue{
			Name:     sdk.LowerCamel(model.Name),
			Type:     sdk.TypeRef{Name: "modelpkg." + model.Name},
			JSONName: sdk.SnakeCase(model.Name),
		}
		requestFields = append(requestFields, value)
		signatureArgs = append(signatureArgs, value)
		callArgs = append(callArgs, "payload."+exportTransportFieldName(value.Name))
	case transportSubjectID:
		value := transportValue{
			Name:     sdk.LowerCamel(idField.Name),
			Type:     idField.Type,
			JSONName: sdk.SnakeCase(idField.Name),
		}
		requestFields = append(requestFields, value)
		signatureArgs = append(signatureArgs, value)
		callArgs = append(callArgs, "payload."+exportTransportFieldName(value.Name))
	}

	for _, param := range method.Params {
		value := transportValue{
			Name:     param.Name,
			Type:     param.Type,
			JSONName: sdk.SnakeCase(param.Name),
		}
		requestFields = append(requestFields, value)
		signatureArgs = append(signatureArgs, value)
		callArgs = append(callArgs, "payload."+exportTransportFieldName(param.Name))
	}

	resultFields := make([]transportValue, 0)
	signatureReturns := make([]sdk.TypeRef, 0)
	nonErrorReturns := transportNonErrorReturns(method)
	for index, result := range nonErrorReturns {
		name := "Result"
		jsonName := "result"
		if len(nonErrorReturns) > 1 {
			name = fmt.Sprintf("Result%d", index+1)
			jsonName = sdk.SnakeCase(name)
		}
		resultFields = append(resultFields, transportValue{
			Name:     name,
			Type:     result,
			JSONName: jsonName,
		})
		signatureReturns = append(signatureReturns, result)
	}

	return transportMethodPlan{
		Name:             method.Name,
		RequestTypeName:  model.Name + "Transport" + method.Name + "Request",
		ResponseTypeName: model.Name + "Transport" + method.Name + "Response",
		RequestFields:    requestFields,
		ResultFields:     resultFields,
		CallArgs:         callArgs,
		SignatureArgs:    signatureArgs,
		SignatureReturns: signatureReturns,
	}, nil
}

func transportFindIDField(model sdk.Model) (sdk.Field, bool, error) {
	var explicit *sdk.Field
	var fallback *sdk.Field
	for i := range model.Fields {
		field := model.Fields[i]
		attr, ok := field.ResolvedAttr("transport")
		if ok {
			for _, item := range attr.Attrs {
				if item.SubName() == "id" || item.NamedBool("id") {
					if explicit != nil {
						return sdk.Field{}, false, sdk.NewErrorf(localize.Text("transport.id() можно оставить только у одного поля модели %q", "transport.id() can only be used on one field of model %q"), model.Name).
							WithHint(localize.Text("Оставьте `@transport.id()` только у одного поля.", "Keep `@transport.id()` on only one field."))
					}
					value := field
					explicit = &value
				}
			}
		}
		if fallback == nil && (field.Name == "Id" || field.Name == "ID") {
			value := field
			fallback = &value
		}
	}

	if explicit != nil {
		return *explicit, true, nil
	}
	if fallback != nil {
		return *fallback, true, nil
	}
	return sdk.Field{}, false, nil
}

func transportMethodSubject(model sdk.Model, method sdk.Method, hasID bool, idField sdk.Field) (transportSubjectMode, error) {
	attr, _ := method.ResolvedAttr("transport")
	modelAttr, _ := model.ResolvedAttr("transport")

	subject := transportSubjectNone
	if subjectValue, ok := attr.Value("subject"); ok {
		subject = transportSubjectMode(strings.ToLower(strings.TrimSpace(subjectValue.String())))
	}
	if subject == transportSubjectNone {
		if subjectValue, ok := modelAttr.Value("subject"); ok {
			subject = transportSubjectMode(strings.ToLower(strings.TrimSpace(subjectValue.String())))
		}
	}

	modelBinding := attrEnabled(attr, "model") || attrEnabled(modelAttr, "model")
	if !modelBinding && subject != transportSubjectNone {
		return transportSubjectNone, sdk.NewErrorf(localize.Text("transport method %q использует subject без `@transport.Model()`", "transport method %q uses a subject without `@transport.Model()`"), method.Name).
			WithHint(localize.Text("Добавьте `@transport.Model()` на метод или на модель, либо уберите `subject`.", "Add `@transport.Model()` on the method or model, or remove `subject`."))
	}
	if !modelBinding {
		return transportSubjectNone, nil
	}

	if subject == transportSubjectNone {
		if hasID {
			subject = transportSubjectID
		} else {
			subject = transportSubjectModel
		}
	}
	if subject == transportSubjectID && !hasID {
		return transportSubjectNone, sdk.NewErrorf(localize.Text("transport method %q требует поле идентификатора для subject=id", "transport method %q requires an identifier field for subject=id"), method.Name).
			WithHint(localize.Text("Добавьте поле `Id` или пометьте нужное поле как `@transport.id()`.", "Add an `Id` field or mark the correct field with `@transport.id()`."))
	}
	_ = idField
	return subject, nil
}

func transportNonErrorReturns(method sdk.Method) []sdk.TypeRef {
	if len(method.Returns) == 0 {
		return nil
	}
	items := make([]sdk.TypeRef, 0, len(method.Returns))
	for _, result := range method.Returns {
		if result.IsError() {
			continue
		}
		items = append(items, result)
	}
	return items
}

func attrNamedBool(attr sdk.ResolvedAttr, name string) bool {
	value, ok := attr.Value(name)
	return ok && value.BoolValue()
}

func attrEnabled(attr sdk.ResolvedAttr, name string) bool {
	if attrNamedBool(attr, name) {
		return true
	}
	for _, item := range attr.Attrs {
		if strings.EqualFold(strings.TrimSpace(item.SubName()), name) {
			return true
		}
	}
	return false
}

func transportMethodIgnored(method sdk.Method) bool {
	for _, attr := range append(append([]sdk.Attr(nil), method.RuntimeAttrs...), method.Attrs...) {
		if attr.Matches("transport.ignore") || attr.SubName() == "ignore" {
			return true
		}
		value, ok := attr.Named("ignore")
		if !ok {
			continue
		}
		if value.Kind == "bool" {
			if value.BoolValue() {
				return true
			}
			continue
		}
		if strings.TrimSpace(value.String()) != "" {
			return true
		}
	}
	return false
}
