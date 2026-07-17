# RPL Test Fixtures

RPL uses several test layers. They answer different questions and should remain
separate so a failure points to the correct boundary.

## Core Go Tests

```bash
go -C src test ./cmd/... ./internal/... ./pkg/...
```

These tests cover parsing, semantic analysis, formatting, generated file
planning, config loading, global/project attr discovery, CLI behavior, JSON API,
SDK helpers, and compiler integration.

## Built-in Attr Tests

```bash
make test-attrs
```

Every attr lives in a directory whose name contains `:`, so the Makefile enters
each directory and runs its Go tests directly. These tests focus on generator
plans, emitted code fragments, validation diagnostics, and dialect-specific
behavior.

## Full Project Tests

```bash
make test-projects
```

The test table is in
`src/internal/service/compiler/project_examples_test.go`. For every project it:

1. copies the fixture into a new temporary directory;
2. deliberately skips any existing `generated/` directory;
3. builds the attrs required by the schema;
4. configures those attrs through an isolated global RPL config;
5. runs the compiler into `generated/`;
6. checks important generated files and API signatures;
7. runs `go test ./...` inside the resulting standalone module.

This catches errors that unit tests alone cannot: wrong module import paths,
missing generated files, stale API assumptions in handwritten code, invalid
protobuf output, and examples that only work inside the RPL repository.

`transport_modes_test.go` adds a focused generated-module fixture containing
all transport modes at once. It performs round trips through HTTP, Unix socket,
in-memory NATS/Kafka broker bridges, and an in-memory WebSocket pair while also
preserving the independent `os.bin` compatibility test.

## Legacy Integration Application

`test/app` is a larger generated fixture retained for focused SQL, validation,
and gRPC compatibility checks. It includes generated artifacts and third-party
dependencies. New tutorial-style scenarios should normally be added under
`examples/projects`, while narrow regression fixtures belong next to the Go
package that owns the behavior.

## Adding a Project Fixture

Add a self-contained module under `examples/projects/<name>` with:

- `.rpl/config.xml`;
- `src/main.rpl`;
- `go.mod` and, when needed, `go.sum`;
- at least one handwritten consumer of generated code;
- at least one meaningful test;
- a README with generation and execution commands.

Then add the project, required attrs, and expected generated fragments to
`TestProjectExamplesGenerateAndCompile`. Do not commit its `generated/` folder:
the test must prove that generation from source is sufficient.
