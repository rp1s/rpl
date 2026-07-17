# Multi-Transport User Service

This standalone module generates six delivery adapters around one handwritten
user service. `internal/users.Service` implements `UserTransportService` once;
the generated package exposes that implementation through child-process IPC,
HTTP, a Unix socket, NATS, Kafka, and WebSocket.

## Schema

`src/main.rpl` applies several modes to the same model:

```rpl
@transport(os.bin)
@transport(http)
@transport(unix)
@transport(nats)
@transport(kafka)
@transport(websocket)
model User {
    Id int @transport.id()
    Name string @validate(min: 2, max: 64)

    func Health return (string)
    func Label return (string) @transport.Model()
}
```

`Health` is available on every adapter. `Label` is model-bound and, because the
model has an ID, becomes:

```go
Label(context.Context, int) (string, error)
```

The same service also receives generated CRUD operations: `Put`, `List`,
`GetByID`, and `Delete`.

## Generated Tree

```text
generated/user/
├── model.gen.go
├── validation/validation.gen.go
└── transport/
    ├── transport.gen.go
    ├── http.gen.go
    ├── unix.gen.go
    ├── nats.gen.go
    ├── kafka.gen.go
    └── websocket.gen.go
```

The common file contains the service contract, request/response payloads, JSON
dispatcher, and backward-compatible `os.bin` adapter. Each other file depends
on that common protocol but imports only what its own transport requires.

## Generate and Test

```bash
rpl run src/main.rpl out generated
go test ./...
```

The project tests execute real calls through:

- the newline JSON `os.bin` server;
- an `httptest` HTTP server and generated typed client;
- a real Unix domain socket and generated typed client.

The compiler suite additionally tests generated NATS, Kafka, and WebSocket
adapters with in-memory implementations of their public interfaces.

## Run os.bin

```bash
printf '%s\n' \
  '{"method":"Put","payload":{"user":{"Id":7,"Name":"Ada"}}}' \
  '{"method":"GetByID","payload":{"id":7}}' \
  '{"method":"Label","payload":{"id":7}}' \
  | go run ./cmd/server
```

Or build a child-process client:

```go
client, err := transport.NewUserTransportClient("./process-service")
defer client.Close()
saved, err := client.Put(ctx, user.User{Id: 7, Name: "Ada"})
```

Stdout is reserved for response envelopes. Application logs must go to stderr.

## Run HTTP

```bash
go run ./cmd/http
```

The generated endpoints begin at `/rpl/user/`:

```bash
curl -sS http://127.0.0.1:8080/rpl/user/Put \
  -H 'Content-Type: application/json' \
  -d '{"user":{"Id":7,"Name":"Ada"}}'
```

Typed client:

```go
client, err := transport.NewUserHTTPClient("http://127.0.0.1:8080", nil)
saved, err := client.Put(ctx, user.User{Id: 7, Name: "Ada"})
```

Custom mounts use `NewUserHTTPHandlerAt` and `NewUserHTTPClientAt`; the default
is also available as `UserHTTPDefaultBasePath`.

Add authentication, tracing, CORS, request IDs, and error/status mapping with
ordinary `net/http` middleware.

## Run Unix Socket

```bash
go run ./cmd/unix -socket /tmp/rpl-users.sock
```

Typed client:

```go
client, err := transport.DialUserUnix(ctx, "/tmp/rpl-users.sock")
defer client.Close()
found, err := client.GetByID(ctx, 7)
```

The server accepts concurrent connections and removes its socket on shutdown.
Calls on one client stay ordered. Context deadlines are applied to socket I/O.
Startup refuses to unlink an existing socket automatically; clean up a verified
stale path explicitly rather than risking another running process.

## Connect NATS

Implement the generated `UserNATSBroker` using the NATS client already selected
by the application. The bridge has only `Request` and `Subscribe` methods.

```go
server, err := transport.StartUserNATSServer(broker, "example.users", service)
client, err := transport.NewUserNATSClient(broker, "example.users")
```

The bridge owns reconnect behavior, draining, subscriptions, and translating a
NATS reply into the generated callback response.

## Connect Kafka

The generated `UserKafkaBroker` makes the extra RPC machinery explicit. Its
implementation owns correlation IDs, reply topics, producer acknowledgements,
consumer commits, retries, and dead-letter behavior.

```go
server, err := transport.StartUserKafkaServer(
    broker,
    "example.users",
    "user-workers",
    service,
)
client, err := transport.NewUserKafkaClient(broker, "example.users", keyFunc)
```

The optional key function derives Kafka partition keys from typed request
values.

## Connect WebSocket

Wrap the selected WebSocket library with `UserWebSocketConn` and pass an
upgrader function:

```go
handler := transport.NewUserWebSocketHandler(service, upgrade)
http.Handle("/users/ws", handler)
```

For an already-upgraded or in-memory connection, call `ServeConn`. The generated
typed client takes the same connection interface:

```go
client, err := transport.NewUserWebSocketClient(connection)
label, err := client.Label(ctx, 7)
```

The application remains responsible for origin validation, authentication,
ping/pong, read limits, deadlines, and reconnect policy.

## Method-Level Routing

This project exposes all methods everywhere, but the language also supports
adapter-specific methods:

```rpl
func HTTPDebug return (string) @transport(http)
func Changed return (string) @transport(kafka)
func Watch return (string) @transport(websocket)
```

Those methods remain in the one business interface, but only their selected
client/adapter can dispatch them. See
`examples/09-transport/07-multi-adapter-service.rpl`.

## Production Checklist

- Keep business logic transport-independent.
- Put auth and authorization at each delivery boundary.
- Configure deadlines for every remote call.
- Map domain errors into stable public error codes.
- Add idempotency for broker retries and Kafka delivery.
- Drain broker subscriptions and close clients during shutdown.
- Reserve protocol output streams for protocol data only.
- Integration-test the concrete NATS, Kafka, and WebSocket bridges you choose.
