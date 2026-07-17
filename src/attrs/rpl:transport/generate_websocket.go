package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strings"
)

func generateWebSocketTransportFile(plan *transportPlan, mode transportModePlan) sdk.GeneratedFile {
	builder := sdk.NewCodeBuilder()
	for _, path := range []string{"context", "encoding/json", "fmt", "io", "net/http", "strings", "sync"} {
		builder.AddImport(path)
	}
	builder.AddImport(plan.ModelImportPath, "modelpkg")
	builder.AddOrderedBlock("transport.websocket", renderWebSocketTransport(plan, mode), 10)
	body, err := sdk.RenderGoFile("transport", builder.Response())
	if err != nil {
		return sdk.GeneratedFile{}
	}
	return sdk.GeneratedFile{Path: "transport/websocket.gen.go", Content: string(body)}
}

func renderWebSocketTransport(plan *transportPlan, mode transportModePlan) string {
	connName := plan.Model.Name + "WebSocketConn"
	upgraderName := plan.Model.Name + "WebSocketUpgrader"
	handlerName := plan.Model.Name + "WebSocketHandler"
	clientName := plan.Model.Name + "WebSocketClient"
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf(`// %s is implemented by WebSocket libraries such as gorilla/websocket.
type %s interface {
	ReadJSON(any) error
	WriteJSON(any) error
	Close() error
}

type %s func(http.ResponseWriter, *http.Request) (%s, error)

type %s struct {
	Service %s
	Upgrade %s
}

func New%s(service %s, upgrade %s) *%s {
	return &%s{Service: service, Upgrade: upgrade}
}

func (handler *%s) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if handler == nil || handler.Service == nil || handler.Upgrade == nil {
		http.Error(writer, "transport WebSocket handler is not configured", http.StatusInternalServerError)
		return
	}
	connection, err := handler.Upgrade(writer, request)
	if err != nil {
		return
	}
	defer connection.Close()
	_ = handler.ServeConn(request.Context(), connection)
}

func (handler *%s) ServeConn(ctx context.Context, connection %s) error {
	if handler == nil || handler.Service == nil || connection == nil {
		return fmt.Errorf("transport WebSocket handler is not configured")
	}
	for {
		var envelope %s
		if err := connection.ReadJSON(&envelope); err != nil {
			if err == io.EOF { return nil }
			return err
		}
		response, dispatchErr := %s(handler.Service, ctx, %q, envelope)
		if dispatchErr != nil { response = %s{Error: dispatchErr.Error()} }
		if err := connection.WriteJSON(response); err != nil { return err }
	}
}

type %s struct {
	connection %s
	mu sync.Mutex
}

func New%s(connection %s) (*%s, error) {
	if connection == nil { return nil, fmt.Errorf("transport WebSocket connection is required") }
	return &%s{connection: connection}, nil
}

func (client *%s) Close() error {
	if client == nil || client.connection == nil { return nil }
	return client.connection.Close()
}

func (client *%s) roundTrip(ctx context.Context, method string, requestValue any, responseValue any) error {
	if client == nil || client.connection == nil { return fmt.Errorf("transport WebSocket client is nil") }
	if err := ctx.Err(); err != nil { return err }
	payload, err := json.Marshal(requestValue)
	if err != nil { return err }
	client.mu.Lock()
	defer client.mu.Unlock()
	if err := client.connection.WriteJSON(%s{Method: method, Payload: payload}); err != nil { return err }
	var envelope %s
	if err := client.connection.ReadJSON(&envelope); err != nil { return err }
	if strings.TrimSpace(envelope.Error) != "" { return fmt.Errorf("transport WebSocket %%s: %%s", method, envelope.Error) }
	if responseValue == nil || len(envelope.Result) == 0 { return nil }
	return json.Unmarshal(envelope.Result, responseValue)
}`,
		connName, connName, upgraderName, connName,
		handlerName, plan.ServiceName, upgraderName,
		handlerName, plan.ServiceName, upgraderName, handlerName, handlerName,
		handlerName, handlerName, connName, plan.EnvelopeName,
		transportDispatchName(plan), transportModeWebSocket, plan.ResponseName,
		clientName, connName, clientName, connName, clientName, clientName,
		clientName, clientName, plan.EnvelopeName, plan.ResponseName,
	))
	if len(mode.Methods) > 0 {
		builder.WriteString("\n\n")
		builder.WriteString(renderTransportTypedClientMethods(plan, mode.Methods, clientName, "roundTrip"))
	}
	return builder.String()
}
