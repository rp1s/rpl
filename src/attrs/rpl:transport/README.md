# rpl:transport

`rpl:transport` generates several delivery adapters around one domain-oriented
Go service interface. Business code implements that interface once; RPL owns
request payloads, response payloads, dispatch, typed clients, and adapter
boilerplate.

Version 2 supports:

- `os.bin` — newline JSON over child-process stdin/stdout;
- `http` — POST JSON endpoints using `net/http`;
- `unix` — concurrent request/reply over a Unix domain socket;
- `nats` — subject-based request/reply behind a small broker interface;
- `kafka` — topic/group RPC behind an interface that owns correlation replies;
- `websocket` — bidirectional JSON calls behind connection/upgrader interfaces.

All adapters use the same generated request/response structures and the same
`context.Context`-based service contract.

## One Model, Several Adapters

```rpl
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

    func Health return (string)
    func HTTPDebug return (string) @transport(http)
    func Notify return (string) @transport(nats)
    func Changed return (string) @transport(kafka)
    func Watch return (string) @transport(websocket)
}
```

Model-level modes enable generated CRUD and every method without an explicit
mode. A method-level mode restricts that method to the selected adapter. The
generated `UserTransportService` contains the union of all operations, so one
business implementation remains the source of truth.

Repeated modes are deduplicated. `websocket` also accepts `ws` as an alias.
Omitting a mode keeps backward-compatible `os.bin` behavior.

Model-level settings can declare generated routing defaults:

- `httpPath`: HTTP base path, beginning with `/`;
- `brokerPrefix`: NATS subject and Kafka topic prefix;
- `kafkaGroup`: Kafka server consumer group.

They may be placed on any model-level `@transport(...)` declaration. Repeated
transport modes still share one resolved configuration.

## Generated Files

Depending on selected modes, output contains:

```text
user/transport/
├── transport.gen.go    common protocol, service, dispatcher, optional os.bin
├── http.gen.go
├── unix.gen.go
├── nats.gen.go
├── kafka.gen.go
└── websocket.gen.go
```

`transport.gen.go` always owns:

- request and response types for every operation;
- the model service interface;
- mode-aware method routing;
- JSON dispatch and domain error envelopes.

An adapter cannot invoke a method that was not assigned to its mode, even if a
crafted envelope names that method directly.

## Automatic Operations

A model-level transport generates:

```go
type UserTransportService interface {
    Put(context.Context, user.User) (user.User, error)
    List(context.Context) ([]user.User, error)
    GetByID(context.Context, int) (user.User, error)
    Delete(context.Context, int) error
}
```

`GetByID` and `Delete` require an ID. RPL first looks for
`@transport.id()`, then falls back to a field named `Id` or `ID`.

## Model-Bound Methods

Classic methods receive only declared parameters:

```rpl
func Search(query string) return ([]string)
```

`@transport.Model()` adds an implicit model subject. With an ID field it uses
the ID by default:

```rpl
func Label return (string) @transport.Model()
```

Generated signature:

```go
Label(context.Context, int) (string, error)
```

Select the whole model explicitly with `subject: "model"`, or the identifier
with `subject: "id"`. A subject without model binding is rejected during
analysis.

## HTTP

`@transport(http)` generates a standard `http.Handler`, ServeMux registration,
and typed client:

```go
mux := http.NewServeMux()
transport.RegisterUserHTTP(mux, service)
http.ListenAndServe(":8080", mux)
```

Client:

```go
client, err := transport.NewUserHTTPClient("http://127.0.0.1:8080", nil)
saved, err := client.Put(ctx, user.User{Id: 7, Name: "Ada"})
```

The default path is `<httpPath>/<Method>`, where `httpPath` defaults to
`/rpl/<snake_model>`. Requests and responses are
limited to 8 MiB by the generated handler/client. Applications should add auth,
timeouts, CORS policy, rate limits, tracing, and domain error/status mapping in
normal HTTP middleware.

Use `NewUserHTTPHandlerAt("/api/users", service)` and
`NewUserHTTPClientAt(baseURL, "/api/users", client)` when the default route does
not fit the application. `UserHTTPDefaultBasePath` exposes the generated
default without duplicating a string.

## Unix Socket

```go
server, err := transport.ListenUserUnix("/tmp/users.sock", service)
defer server.Close()

client, err := transport.DialUserUnix(ctx, "/tmp/users.sock")
defer client.Close()
```

The server accepts concurrent connections; calls on one client are serialized
to preserve request/reply ordering. Context deadlines become connection I/O
deadlines. `Close` stops the listener, closes active connections, waits for
handlers, and removes the socket.

For safety, startup does not unlink an existing socket path: `ListenUserUnix`
returns the `net.Listen` error instead of potentially disconnecting another
process. Remove a verified stale socket explicitly during application startup.

Unix sockets are a runtime capability. The generated code compiles on all Go
targets, but `net.Listen("unix", ...)` is only usable where the OS supports it.

## NATS

Generated code deliberately does not force a version of `nats.go`. Implement:

```go
type UserNATSBroker interface {
    Request(context.Context, string, []byte) ([]byte, error)
    Subscribe(string, func(context.Context, []byte) ([]byte, error)) (io.Closer, error)
}
```

Then start both sides:

```go
server, err := transport.StartUserNATSServer(broker, "company.users", service)
client, err := transport.NewUserNATSClient(broker, "company.users")
```

Subjects have the form `<prefix>.<lower_method>`. The default prefix is
`brokerPrefix`, which defaults to `rpl.<lower_model>`. A `nats.go` bridge is responsible for subscription
draining, request deadlines, reconnect behavior, and mapping message replies to
the callback result.

## Kafka

Kafka is not request/reply by itself, so the generated adapter makes that
requirement explicit:

```go
type UserKafkaBroker interface {
    Request(ctx context.Context, topic string, key, payload []byte) ([]byte, error)
    Consume(
        topic string,
        group string,
        handler func(context.Context, []byte, []byte) ([]byte, error),
    ) (io.Closer, error)
}
```

The application bridge owns correlation IDs, reply topics, producer delivery,
consumer commits, retry/dead-letter policy, and exactly-once/idempotency rules.

```go
server, err := transport.StartUserKafkaServer(broker, "company.users", "user-workers", service)
client, err := transport.NewUserKafkaClient(broker, "company.users", keyFunc)
```

Topics use `<prefix>.<lower_method>`. `keyFunc` can derive a partition key from
the typed generated request; it may be nil.

## WebSocket

The generated WebSocket adapter is library-neutral:

```go
type UserWebSocketConn interface {
    ReadJSON(any) error
    WriteJSON(any) error
    Close() error
}

type UserWebSocketUpgrader func(
    http.ResponseWriter,
    *http.Request,
) (UserWebSocketConn, error)
```

Wrap the WebSocket library chosen by the application and pass the upgrader to:

```go
handler := transport.NewUserWebSocketHandler(service, upgrade)
http.Handle("/users/ws", handler)
```

Tests and already-upgraded connections can call `handler.ServeConn` directly.
The typed client accepts any compatible connection:

```go
client, err := transport.NewUserWebSocketClient(connection)
result, err := client.Watch(ctx)
```

One generated client serializes calls because JSON request/reply frames share
one ordered connection. Applications own origin checks, ping/pong, read limits,
write deadlines, reconnection, and authentication during upgrade.

## NATS/Kafka Interfaces Are Intentional

The abstraction is not meant to hide broker semantics. It keeps domain code and
generated schemas independent of a dependency version while making transport
responsibilities visible at the integration boundary. A production bridge
should live in the infrastructure layer and have its own integration tests
against the actual broker.

## Testing

The compiler integration suite generates one module with all six modes and
executes round trips for HTTP, Unix, NATS, Kafka, and WebSocket. The original
`os.bin` compatibility test remains separate.

```bash
make test-projects
go -C src test ./internal/service/compiler -run Transport
```
