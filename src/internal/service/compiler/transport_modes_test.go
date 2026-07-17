package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFileGeneratesAndCompilesAllTransportModes(t *testing.T) {
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:transport")

	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/transportmodes\n\ngo 1.25.6\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	sourcePath := filepath.Join(projectDir, "main.rpl")
	schema := `target(lang: golang)

attrs (
    "rpl:transport"
)

@transport(os.bin)
@transport(http, httpPath: "/api/users", brokerPrefix: "acme.users", kafkaGroup: "acme-users-rpc")
@transport(unix)
@transport(nats)
@transport(kafka)
@transport(websocket)
model User {
    Id int @transport.id()
    Name string

    func Shared return (string)
    func HTTPOnly return (string) @transport(http)
    func Notify return (string) @transport(nats)
    func Event return (string) @transport(kafka)
    func Watch return (string) @transport(websocket)
}
`
	if err := os.WriteFile(sourcePath, []byte(schema), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	outputDir := filepath.Join(projectDir, "generated")
	if _, err := New().RunFileTo(sourcePath, outputDir); err != nil {
		t.Fatalf("generate all transport modes: %v", err)
	}

	transportDir := filepath.Join(outputDir, "user", "transport")
	for _, expected := range []string{
		"transport.gen.go",
		"http.gen.go",
		"unix.gen.go",
		"nats.gen.go",
		"kafka.gen.go",
		"websocket.gen.go",
	} {
		if _, err := os.Stat(filepath.Join(transportDir, expected)); err != nil {
			t.Fatalf("expected generated %s: %v", expected, err)
		}
	}

	assertFileContains(t, filepath.Join(transportDir, "transport.gen.go"), "type UserTransportService interface")
	assertFileContains(t, filepath.Join(transportDir, "http.gen.go"), "type UserHTTPHandler struct")
	assertFileContains(t, filepath.Join(transportDir, "http.gen.go"), `const UserHTTPDefaultBasePath = "/api/users"`)
	assertFileContains(t, filepath.Join(transportDir, "unix.gen.go"), "func ListenUserUnix")
	assertFileContains(t, filepath.Join(transportDir, "nats.gen.go"), "type UserNATSBroker interface")
	assertFileContains(t, filepath.Join(transportDir, "nats.gen.go"), `prefix = "acme.users"`)
	assertFileContains(t, filepath.Join(transportDir, "kafka.gen.go"), "type UserKafkaBroker interface")
	assertFileContains(t, filepath.Join(transportDir, "kafka.gen.go"), `group = "acme-users-rpc"`)
	assertFileContains(t, filepath.Join(transportDir, "websocket.gen.go"), "type UserWebSocketConn interface")
	assertFileContains(t, filepath.Join(transportDir, "http.gen.go"), "func (client *UserHTTPClient) HTTPOnly")
	assertFileNotContains(t, filepath.Join(transportDir, "http.gen.go"), "func (client *UserHTTPClient) Event")
	assertFileContains(t, filepath.Join(transportDir, "kafka.gen.go"), "func (client *UserKafkaClient) Event")
	assertFileNotContains(t, filepath.Join(transportDir, "kafka.gen.go"), "func (client *UserKafkaClient) HTTPOnly")

	applicationTest := `package transportmodes_test

import (
    "context"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "os"
    "runtime"
    "strings"
    "sync"
    "testing"

    model "example.com/transportmodes/generated/user"
    transport "example.com/transportmodes/generated/user/transport"
)

type service struct{}

func (service) Put(_ context.Context, user model.User) (model.User, error) { return user, nil }
func (service) List(context.Context) ([]model.User, error) { return []model.User{{Id: 1, Name: "Ada"}}, nil }
func (service) GetByID(_ context.Context, id int) (model.User, error) { return model.User{Id: id, Name: "Ada"}, nil }
func (service) Delete(context.Context, int) error { return nil }
func (service) Shared(context.Context) (string, error) { return "shared", nil }
func (service) HTTPOnly(context.Context) (string, error) { return "http", nil }
func (service) Notify(context.Context) (string, error) { return "nats", nil }
func (service) Event(context.Context) (string, error) { return "kafka", nil }
func (service) Watch(context.Context) (string, error) { return "websocket", nil }

var _ transport.UserTransportService = service{}

func TestHTTPRoundTrip(t *testing.T) {
    server := httptest.NewServer(transport.NewUserHTTPHandler(service{}))
    defer server.Close()
    client, err := transport.NewUserHTTPClient(server.URL, server.Client())
    if err != nil { t.Fatal(err) }
    got, err := client.Put(context.Background(), model.User{Id: 7, Name: "Ada"})
    if err != nil { t.Fatal(err) }
    if got.Id != 7 || got.Name != "Ada" { t.Fatalf("unexpected user: %#v", got) }
    label, err := client.HTTPOnly(context.Background())
    if err != nil || label != "http" { t.Fatalf("HTTPOnly = %q, %v", label, err) }
    blocked, err := http.Post(server.URL+"/api/users/Event", "application/json", strings.NewReader(` + "`{}`" + `))
    if err != nil { t.Fatal(err) }
    defer blocked.Body.Close()
    if blocked.StatusCode != http.StatusInternalServerError {
        t.Fatalf("method restricted to Kafka returned HTTP status %d", blocked.StatusCode)
    }
}

func TestUnixRoundTrip(t *testing.T) {
    if runtime.GOOS == "windows" { t.Skip("Unix socket runtime is platform-specific") }
    placeholder, err := os.CreateTemp("", "rpl-modes-*.sock")
    if err != nil { t.Fatal(err) }
    socket := placeholder.Name()
    _ = placeholder.Close()
    _ = os.Remove(socket)
    t.Cleanup(func() { _ = os.Remove(socket) })
    server, err := transport.ListenUserUnix(socket, service{})
    if err != nil { t.Fatal(err) }
    defer server.Close()
    client, err := transport.DialUserUnix(context.Background(), socket)
    if err != nil { t.Fatal(err) }
    defer client.Close()
    got, err := client.GetByID(context.Background(), 9)
    if err != nil { t.Fatal(err) }
    if got.Id != 9 { t.Fatalf("id = %d", got.Id) }
    shared, err := client.Shared(context.Background())
    if err != nil || shared != "shared" { t.Fatalf("Shared = %q, %v", shared, err) }
}

type closer func() error
func (close closer) Close() error { return close() }

type natsBroker struct {
    mu sync.RWMutex
    handlers map[string]func(context.Context, []byte) ([]byte, error)
}

func newNATSBroker() *natsBroker { return &natsBroker{handlers: make(map[string]func(context.Context, []byte) ([]byte, error))} }
func (broker *natsBroker) Request(ctx context.Context, subject string, payload []byte) ([]byte, error) {
    broker.mu.RLock(); handler := broker.handlers[subject]; broker.mu.RUnlock()
    return handler(ctx, payload)
}
func (broker *natsBroker) Subscribe(subject string, handler func(context.Context, []byte) ([]byte, error)) (io.Closer, error) {
    broker.mu.Lock(); broker.handlers[subject] = handler; broker.mu.Unlock()
    return closer(func() error { broker.mu.Lock(); delete(broker.handlers, subject); broker.mu.Unlock(); return nil }), nil
}

func TestNATSRoundTrip(t *testing.T) {
    broker := newNATSBroker()
    server, err := transport.StartUserNATSServer(broker, "demo.users", service{})
    if err != nil { t.Fatal(err) }
    defer server.Close()
    client, err := transport.NewUserNATSClient(broker, "demo.users")
    if err != nil { t.Fatal(err) }
    got, err := client.Notify(context.Background())
    if err != nil || got != "nats" { t.Fatalf("Notify = %q, %v", got, err) }
}

type kafkaHandler func(context.Context, []byte, []byte) ([]byte, error)
type kafkaBroker struct {
    mu sync.RWMutex
    handlers map[string]kafkaHandler
}

func newKafkaBroker() *kafkaBroker { return &kafkaBroker{handlers: make(map[string]kafkaHandler)} }
func (broker *kafkaBroker) Request(ctx context.Context, topic string, key []byte, payload []byte) ([]byte, error) {
    broker.mu.RLock(); handler := broker.handlers[topic]; broker.mu.RUnlock()
    return handler(ctx, key, payload)
}
func (broker *kafkaBroker) Consume(topic string, group string, handler func(context.Context, []byte, []byte) ([]byte, error)) (io.Closer, error) {
    _ = group
    broker.mu.Lock(); broker.handlers[topic] = handler; broker.mu.Unlock()
    return closer(func() error { broker.mu.Lock(); delete(broker.handlers, topic); broker.mu.Unlock(); return nil }), nil
}

func TestKafkaRoundTrip(t *testing.T) {
    broker := newKafkaBroker()
    server, err := transport.StartUserKafkaServer(broker, "demo.users", "tests", service{})
    if err != nil { t.Fatal(err) }
    defer server.Close()
    client, err := transport.NewUserKafkaClient(broker, "demo.users", func(_ string, request any) []byte {
        body, _ := json.Marshal(request)
        return body
    })
    if err != nil { t.Fatal(err) }
    got, err := client.Event(context.Background())
    if err != nil || got != "kafka" { t.Fatalf("Event = %q, %v", got, err) }
}

type memoryWebSocket struct {
    reads <-chan []byte
    writes chan<- []byte
    done <-chan struct{}
    close func()
}

func newWebSocketPair() (*memoryWebSocket, *memoryWebSocket) {
    leftToRight := make(chan []byte, 1)
    rightToLeft := make(chan []byte, 1)
    done := make(chan struct{})
    once := &sync.Once{}
    closePair := func() { once.Do(func() { close(done) }) }
    left := &memoryWebSocket{reads: rightToLeft, writes: leftToRight, done: done, close: closePair}
    right := &memoryWebSocket{reads: leftToRight, writes: rightToLeft, done: done, close: closePair}
    return left, right
}

func (connection *memoryWebSocket) ReadJSON(value any) error {
    select {
    case <-connection.done:
        return io.EOF
    case body := <-connection.reads:
        return json.Unmarshal(body, value)
    }
}

func (connection *memoryWebSocket) WriteJSON(value any) error {
    body, err := json.Marshal(value)
    if err != nil { return err }
    select {
    case <-connection.done:
        return io.EOF
    case connection.writes <- body:
        return nil
    }
}

func (connection *memoryWebSocket) Close() error { connection.close(); return nil }

func TestWebSocketRoundTrip(t *testing.T) {
    serverConnection, clientConnection := newWebSocketPair()
    handler := transport.NewUserWebSocketHandler(service{}, nil)
    finished := make(chan error, 1)
    go func() { finished <- handler.ServeConn(context.Background(), serverConnection) }()
    client, err := transport.NewUserWebSocketClient(clientConnection)
    if err != nil { t.Fatal(err) }
    got, err := client.Watch(context.Background())
    if err != nil || got != "websocket" { t.Fatalf("Watch = %q, %v", got, err) }
    if err := client.Close(); err != nil { t.Fatal(err) }
    if err := <-finished; err != nil { t.Fatal(err) }
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "transport_test.go"), []byte(applicationTest), 0o644); err != nil {
		t.Fatalf("write generated module test: %v", err)
	}

	runGoTest(t, projectDir)
}

func TestRunFileRejectsUnknownTransportMode(t *testing.T) {
	useRepoRootAsWorkingDir(t)
	buildBundledRuntime(t, "rpl:transport")

	projectDir := t.TempDir()
	sourcePath := filepath.Join(projectDir, "main.rpl")
	schema := `target(lang: golang)

attrs (
    "rpl:transport"
)

@transport(smtp)
model Message {
    Id string @transport.id()
}
`
	if err := os.WriteFile(sourcePath, []byte(schema), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	_, err := New().RunFile(sourcePath)
	if err == nil {
		t.Fatal("expected unsupported transport mode error")
	}
	if !strings.Contains(err.Error(), `transport mode "smtp"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
