package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strings"
)

func generateUnixTransportFile(plan *transportPlan, mode transportModePlan) sdk.GeneratedFile {
	builder := sdk.NewCodeBuilder()
	for _, path := range []string{"context", "encoding/json", "errors", "fmt", "io", "net", "os", "strings", "sync", "time"} {
		builder.AddImport(path)
	}
	builder.AddImport(plan.ModelImportPath, "modelpkg")
	builder.AddOrderedBlock("transport.unix.server", renderUnixServer(plan), 10)
	builder.AddOrderedBlock("transport.unix.client", renderUnixClient(plan, mode), 20)
	body, err := sdk.RenderGoFile("transport", builder.Response())
	if err != nil {
		return sdk.GeneratedFile{}
	}
	return sdk.GeneratedFile{Path: "transport/unix.gen.go", Content: string(body)}
}

func renderUnixServer(plan *transportPlan) string {
	serverName := plan.Model.Name + "UnixServer"
	endpointName := plan.Model.Name + "Unix"
	return fmt.Sprintf(`// %s serves %s over a Unix domain socket.
type %s struct {
	Service %s
	listener net.Listener
	path string
	closeOnce sync.Once
	wg sync.WaitGroup
	connMu sync.Mutex
	connections map[net.Conn]struct{}
}

func Listen%s(path string, service %s) (*%s, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("transport Unix socket path is required")
	}
	if service == nil {
		return nil, fmt.Errorf("transport service is nil")
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	server := &%s{Service: service, listener: listener, path: path, connections: make(map[net.Conn]struct{})}
	go func() { _ = server.Serve() }()
	return server, nil
}

func (server *%s) Serve() error {
	if server == nil || server.listener == nil || server.Service == nil {
		return fmt.Errorf("transport Unix server is not configured")
	}
	for {
		connection, err := server.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		server.connMu.Lock()
		server.connections[connection] = struct{}{}
		server.connMu.Unlock()
		server.wg.Add(1)
		go func() {
			defer server.wg.Done()
			defer func() {
				server.connMu.Lock()
				delete(server.connections, connection)
				server.connMu.Unlock()
			}()
			defer connection.Close()
			_ = server.serveConnection(connection)
		}()
	}
}

func (server *%s) serveConnection(connection net.Conn) error {
	decoder := json.NewDecoder(connection)
	encoder := json.NewEncoder(connection)
	for {
		var envelope %s
		if err := decoder.Decode(&envelope); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		response, dispatchErr := %s(server.Service, context.Background(), %q, envelope)
		if dispatchErr != nil {
			response = %s{Error: dispatchErr.Error()}
		}
		if err := encoder.Encode(response); err != nil {
			return err
		}
	}
}

func (server *%s) Close() error {
	if server == nil {
		return nil
	}
	var closeErr error
	server.closeOnce.Do(func() {
		if server.listener != nil {
			closeErr = server.listener.Close()
		}
		server.connMu.Lock()
		for connection := range server.connections {
			_ = connection.Close()
		}
		server.connMu.Unlock()
		server.wg.Wait()
		if server.path != "" {
			if info, err := os.Lstat(server.path); err == nil && info.Mode()&os.ModeSocket != 0 {
				if err := os.Remove(server.path); err != nil && !os.IsNotExist(err) && closeErr == nil {
					closeErr = err
				}
			}
		}
	})
	return closeErr
}`,
		serverName, plan.Model.Name, serverName, plan.ServiceName,
		endpointName, plan.ServiceName, serverName, serverName,
		serverName, serverName, plan.EnvelopeName, transportDispatchName(plan), transportModeUnix, plan.ResponseName,
		serverName,
	)
}

func renderUnixClient(plan *transportPlan, mode transportModePlan) string {
	clientName := plan.Model.Name + "UnixClient"
	endpointName := plan.Model.Name + "Unix"
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf(`// %s calls %s through a Unix domain socket.
type %s struct {
	connection net.Conn
	encoder *json.Encoder
	decoder *json.Decoder
	mu sync.Mutex
}

func Dial%s(ctx context.Context, path string) (*%s, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("transport Unix socket path is required")
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	return &%s{connection: connection, encoder: json.NewEncoder(connection), decoder: json.NewDecoder(connection)}, nil
}

func (client *%s) Close() error {
	if client == nil || client.connection == nil {
		return nil
	}
	return client.connection.Close()
}

func (client *%s) roundTrip(ctx context.Context, method string, requestValue any, responseValue any) error {
	if client == nil || client.connection == nil {
		return fmt.Errorf("transport Unix client is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	payload, err := json.Marshal(requestValue)
	if err != nil {
		return err
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if deadline, ok := ctx.Deadline(); ok {
		_ = client.connection.SetDeadline(deadline)
		defer client.connection.SetDeadline(time.Time{})
	}
	if err := client.encoder.Encode(%s{Method: method, Payload: payload}); err != nil {
		return err
	}
	var envelope %s
	if err := client.decoder.Decode(&envelope); err != nil {
		return err
	}
	if strings.TrimSpace(envelope.Error) != "" {
		return fmt.Errorf("transport Unix %%s: %%s", method, envelope.Error)
	}
	if responseValue == nil || len(envelope.Result) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Result, responseValue)
}`,
		clientName, plan.Model.Name, clientName,
		endpointName, clientName, clientName,
		clientName, clientName, plan.EnvelopeName, plan.ResponseName,
	))
	if len(mode.Methods) > 0 {
		builder.WriteString("\n\n")
		builder.WriteString(renderTransportTypedClientMethods(plan, mode.Methods, clientName, "roundTrip"))
	}
	return builder.String()
}
