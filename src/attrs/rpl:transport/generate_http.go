package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strings"
)

func generateHTTPTransportFile(plan *transportPlan, mode transportModePlan) sdk.GeneratedFile {
	builder := sdk.NewCodeBuilder()
	for _, path := range []string{"bytes", "context", "encoding/json", "fmt", "io", "net/http", "net/url", "strings"} {
		builder.AddImport(path)
	}
	builder.AddImport(plan.ModelImportPath, "modelpkg")
	builder.AddOrderedBlock("transport.http.handler", renderHTTPHandler(plan), 10)
	builder.AddOrderedBlock("transport.http.client", renderHTTPClient(plan, mode), 20)
	body, err := sdk.RenderGoFile("transport", builder.Response())
	if err != nil {
		return sdk.GeneratedFile{}
	}
	return sdk.GeneratedFile{Path: "transport/http.gen.go", Content: string(body)}
}

func renderHTTPHandler(plan *transportPlan) string {
	handlerName := plan.Model.Name + "HTTPHandler"
	basePathName := plan.Model.Name + "HTTPDefaultBasePath"
	normalizeName := sdk.LowerCamel(plan.Model.Name) + "HTTPNormalizeBasePath"
	return fmt.Sprintf(`const %s = %q

func %s(basePath string) string {
	basePath = strings.TrimRight(strings.TrimSpace(basePath), "/")
	if basePath == "" {
		return %s
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	return basePath
}

// %s exposes %s through POST JSON endpoints.
type %s struct {
	Service %s
	BasePath string
}

func New%s(service %s) *%s {
	return New%sAt(%s, service)
}

func New%sAt(basePath string, service %s) *%s {
	return &%s{Service: service, BasePath: %s(basePath)}
}

func Register%s(mux *http.ServeMux, service %s) {
	if mux == nil {
		return
	}
	handler := New%s(service)
	mux.Handle(handler.BasePath+"/", handler)
}

func (handler *%s) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, %q, http.StatusMethodNotAllowed)
		return
	}
	method := strings.Trim(strings.TrimPrefix(request.URL.Path, handler.BasePath), "/")
	if method == "" {
		http.Error(writer, %q, http.StatusNotFound)
		return
	}
	payload, err := io.ReadAll(io.LimitReader(request.Body, 8<<20))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	response, dispatchErr := %s(handler.Service, request.Context(), %q, %s{Method: method, Payload: payload})
	if dispatchErr != nil {
		response = %s{Error: dispatchErr.Error()}
		writer.WriteHeader(http.StatusInternalServerError)
	}
	_ = json.NewEncoder(writer).Encode(response)
}`,
		basePathName, plan.HTTPBasePath,
		normalizeName, basePathName,
		handlerName, plan.ServiceName, handlerName, plan.ServiceName,
		handlerName, plan.ServiceName, handlerName, handlerName, basePathName,
		handlerName, plan.ServiceName, handlerName, handlerName, normalizeName,
		plan.Model.Name+"HTTP", plan.ServiceName, handlerName,
		handlerName, "method not allowed", "transport method is required",
		transportDispatchName(plan), transportModeHTTP, plan.EnvelopeName, plan.ResponseName,
	)
}

func renderHTTPClient(plan *transportPlan, mode transportModePlan) string {
	clientName := plan.Model.Name + "HTTPClient"
	basePathName := plan.Model.Name + "HTTPDefaultBasePath"
	normalizeName := sdk.LowerCamel(plan.Model.Name) + "HTTPNormalizeBasePath"
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf(`// %s calls the generated HTTP JSON endpoints for %s.
type %s struct {
	baseURL string
	basePath string
	client *http.Client
}

func New%s(baseURL string, client *http.Client) (*%s, error) {
	return New%sAt(baseURL, %s, client)
}

func New%sAt(baseURL string, basePath string, client *http.Client) (*%s, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("transport HTTP base URL is required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &%s{baseURL: baseURL, basePath: %s(basePath), client: client}, nil
}

func (client *%s) roundTrip(ctx context.Context, method string, requestValue any, responseValue any) error {
	if client == nil || client.client == nil {
		return fmt.Errorf("transport HTTP client is nil")
	}
	payload, err := json.Marshal(requestValue)
	if err != nil {
		return err
	}
	endpoint := client.baseURL + client.basePath + "/" + url.PathEscape(method)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var envelope %s
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&envelope); err != nil {
		return err
	}
	if strings.TrimSpace(envelope.Error) != "" {
		return fmt.Errorf("transport HTTP %%s: %%s", method, envelope.Error)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("transport HTTP %%s returned %%s", method, response.Status)
	}
	if responseValue == nil || len(envelope.Result) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Result, responseValue)
}`,
		clientName, plan.Model.Name, clientName,
		clientName, clientName, clientName, basePathName,
		clientName, clientName, clientName, normalizeName,
		clientName, plan.ResponseName,
	))
	if len(mode.Methods) > 0 {
		builder.WriteString("\n\n")
		builder.WriteString(renderTransportTypedClientMethods(plan, mode.Methods, clientName, "roundTrip"))
	}
	return builder.String()
}
