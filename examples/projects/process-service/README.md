# Process Service

This project exposes a typed user service through a child process. The generated
transport uses newline-delimited JSON over stdin/stdout, so it works without an
HTTP port, service discovery, TLS certificates, or a separate daemon.

It is useful for editor helpers, build tools, local automation, sandboxed
workers, and language bridges where a parent process owns lifecycle and I/O.

## Generated API

`@transport(os.bin)` generates:

- `UserTransportService`, the interface implemented by handwritten code;
- `NewUserTransportServer` and `RunUserTransportServer`;
- request and response payload types for CRUD and schema methods;
- `NewUserTransportClient`, which starts a child process;
- typed client methods for `Put`, `List`, `GetByID`, `Delete`, `Health`, and `Label`.

Because `Id` has `@transport.id()`, `Label @transport.Model()` is bound by ID:

```go
Label(ctx context.Context, id int) (string, error)
```

The classic `Health` method has no implicit model argument.

## Generate and Test

```bash
rpl run src/main.rpl out generated
go test ./...
go build -o process-service ./cmd/server
```

The integration test sends three real protocol envelopes through the generated
server and decodes three responses. It covers dispatch, model decoding, shared
service state, ID binding, and error envelope handling without starting an OS
process.

## Run the Protocol by Hand

Start the server and pipe requests into it:

```bash
printf '%s\n' \
  '{"method":"Put","payload":{"user":{"Id":7,"Name":"Ada"}}}' \
  '{"method":"GetByID","payload":{"id":7}}' \
  '{"method":"Label","payload":{"id":7}}' \
  | go run ./cmd/server
```

Each input line produces exactly one JSON response line. Protocol errors are
returned in the response `error` field; malformed JSON or broken I/O terminates
the stream.

## Use the Generated Child-Process Client

After building the server:

```go
client, err := transport.NewUserTransportClient("./process-service")
if err != nil {
    return err
}
defer client.Close()

saved, err := client.Put(ctx, user.User{Id: 7, Name: "Ada"})
label, err := client.Label(ctx, saved.Id)
```

The client serializes calls with a mutex because stdin/stdout is one ordered
request stream. Cancellation is checked before a round trip. Process shutdown
is owned by `Close`.

## Production Notes

- Keep stdout reserved for protocol responses; send logs to stderr.
- Treat input as untrusted even when the parent is local.
- Put timeouts around client calls in the parent process.
- Decide whether one failed request should return an error envelope or end the process.
- Use gRPC when you need remote networking, streaming, load balancing, or standard observability middleware.
