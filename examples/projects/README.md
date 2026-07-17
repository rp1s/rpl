# Full RPL Projects

These directories are complete, isolated Go modules. Unlike the small cookbook
schemas one level above, every project contains an RPL configuration, schema,
handwritten application layer, executable entry point, and tests that compile
against freshly generated code.

## Project Matrix

| Project | RPL attrs | What it demonstrates | Verification |
| --- | --- | --- | --- |
| [account-service](account-service/README.md) | `sql`, `std`, `validate` | SQLite-oriented repository API, typed SQL filters, validation-first service | generation + `go test ./...` |
| [session-cache](session-cache/README.md) | `redis`, `std`, `validate` | Redis keys, hash serialization, ignored fields, TTL boundary | generation + round-trip test |
| [process-service](process-service/README.md) | `transport`, `validate` | one service exposed through os.bin, HTTP, Unix, NATS, Kafka, and WebSocket | clean generation + multi-transport integration tests |
| [grpc-service](grpc-service/README.md) | `grpc`, `validate` | protobuf, typed gRPC adapters, validation-aware service, TCP server | generation + adapter tests |

## Run All Projects

From the repository root:

```bash
make build-host

for project in account-service session-cache process-service grpc-service; do
  ./build/$(go env GOOS)-$(go env GOARCH)/rpl \
    run "examples/projects/$project/src/main.rpl" \
    out "examples/projects/$project/generated"
  go -C "examples/projects/$project" test ./...
done
```

The repository test suite performs the same lifecycle in temporary directories:

```bash
make test-projects
```

This target does not trust generated files left in the working tree. It copies
each project without `generated/`, builds the required attrs, regenerates the
module, checks important API fragments, and runs `go test ./...`.

## Common Project Layout

```text
project/
├── .rpl/
│   └── config.xml          project-local RPL settings
├── src/
│   └── main.rpl            source schema
├── generated/              reproducible output; intentionally ignored
├── internal/               handwritten application code
├── cmd/                    runnable entry points
├── go.mod                  isolated Go module
└── README.md               project-specific tutorial
```

The import boundary is intentional: `internal/` and `cmd/` may depend on
`generated/`, while generated code never imports the handwritten application.
Deleting `generated/` and rerunning RPL must always restore a buildable project.

## Local, Global, and Bundled Attrs

Every example has a project `.rpl/config.xml`. Attr discovery uses this order:

1. project attrs configured by `.rpl/config.xml`;
2. user-wide attrs configured by the global RPL config;
3. attrs bundled next to the `rpl` executable.

Create and inspect the global setup with:

```bash
rpl config init --global
rpl config path --global
rpl config show --global
rpl attr init --global acme:audit
rpl attr list
```

The generated attr directory contains a `go.mod` connected to the SDK sidecar
shipped with RPL. The first `attr list`, `attr info`, or generation request
builds its executable automatically. Set `RPL_ATTR_DEV=1` to rebuild an existing
attr binary after editing its Go sources.

Set `RPL_CONFIG_HOME` to use an isolated or portable config directory:

```bash
RPL_CONFIG_HOME="$PWD/.tooling/rpl" rpl config init --global
RPL_CONFIG_HOME="$PWD/.tooling/rpl" rpl attr init --global acme:audit
```

A project attr with the same `author:name` shadows its global and bundled
counterpart. This lets a repository pin a generator without changing other
projects on the machine.

## Working on an Example

Use the same short loop in every project:

```bash
rpl fmt src/main.rpl
rpl auto set import src/main.rpl
rpl run src/main.rpl out generated
go test ./...
```

Generated files are build inputs, not the source of truth. Change the `.rpl`
schema or attr implementation instead of editing `*.gen.go`, `.proto`, or
`.rpl-generated.json` by hand.
