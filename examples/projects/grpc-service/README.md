# gRPC User Service

This project generates protobuf definitions and typed Go adapters from an RPL
model, then implements the generated service interface with a small in-memory
repository. The TCP server in `cmd/server` is runnable application code, not a
placeholder.

## Output

Generation creates:

```text
generated/user/
├── model.gen.go
├── validation/validation.gen.go
└── grpc/
    ├── user.proto
    ├── user.pb.go
    ├── user_grpc.pb.go
    ├── proto.gen.go
    ├── server.gen.go
    └── client.gen.go
```

RPL owns the semantic mapping and adapter layer. `protoc`, `protoc-gen-go`, and
`protoc-gen-go-grpc` produce the standard protobuf runtime files.

## Prerequisites

The three protobuf tools must be available on `PATH` during generation:

```bash
protoc --version
protoc-gen-go --version
protoc-gen-go-grpc --version
```

The checked-in `go.mod` pins the Go gRPC and protobuf runtime dependencies.

## Generate, Test, and Run

```bash
rpl run src/main.rpl out generated
go mod download
go test ./...
go run ./cmd/server -listen :50051
```

The generated service interface is intentionally application-shaped:

```go
type UserService interface {
    Put(context.Context, user.User) (user.User, error)
    GetByID(context.Context, int) (user.User, error)
    Delete(context.Context, int) error
    List(context.Context) ([]user.User, error)
}
```

Handwritten code does not need to manipulate protobuf messages. The generated
server converts messages into domain models before calling the service, and the
generated client exposes the same domain-oriented interface.

## Validation Boundary

`internal/users.Service.Put` calls the generated validator before storing a
model. The adapter test invokes `UserGRPCServer.Put` with a protobuf message and
proves that an invalid name/email returns an error through the same boundary.

## Register in an Existing Server

The executable uses the convenience function:

```go
server := grpc.NewServer()
generatedgrpc.RegisterUserGRPC(server, users.NewService())
```

For production, add interceptors for authentication, request IDs, metrics,
tracing, rate limits, panic recovery, and error-to-status mapping when creating
the `grpc.Server`.

## Typed Client

Given a `grpc.ClientConnInterface`:

```go
client := generatedgrpc.NewUserGRPCClient(conn)
saved, err := client.Put(ctx, user.User{Id: 7, Name: "Ada", Email: "ada@example.com"})
found, err := client.GetByID(ctx, saved.Id)
```

The returned value implements the same generated `UserService` interface as the
server-side handwritten service, which makes in-process fakes and remote clients
interchangeable at the application boundary.

## Exercises

- Add an RPL method and compare classic versus `@grpc.Model()` binding.
- Add a second model and register both generated services in one gRPC server.
- Use `bufconn` for a full network-stack test without a TCP port.
- Map domain errors to canonical gRPC status codes in an interceptor.
