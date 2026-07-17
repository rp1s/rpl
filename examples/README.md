# RPL Examples

This directory is a working cookbook for RPL.

There are two levels of examples:

- numbered folders contain focused schemas for learning one language or attr feature;
- [`projects`](projects) contains standalone Go modules with configuration,
  generated boundaries, handwritten application code, commands, and tests.

If you want one big entry point first, start with:

- `99-showcase/main.rpl` for the largest package-level showcase
- `99-showcase/realtime.rpl` for `grpc.Model()`, `subject`, and `inside`

The closest "single rich model" example is `99-showcase/main.rpl: User`.
The full showcase package uses almost every built-in runtime across several models,
because some attrs are model-only, some are field-only, and some only make
sense on specific transports.

## Folders

- `00-syntax` - core language syntax without much runtime logic
- `01-std` - `@comment`, `@group`, `@ignore`
- `02-validate` - numeric, string, time, URL, phone, email, hash validation
- `03-sql` - SQL storage examples, including SQLite
- `04-redis` - Redis cache/storage examples
- `05-grpc` - classic and model-bound gRPC styles
- `06-multifile` - one package split into multiple `.rpl` files
- `07-imports` - importing another `.rpl` file
- `08-mongodb` - MongoDB collections, indexes, search, and CRUD helpers
- `09-transport` - local stdin/stdout process transport with generated shell client/server
- `99-showcase` - large end-to-end examples
- `projects` - four complete applications that are regenerated and compiled in CI

## Choose the Right Starting Point

| Goal | Start here |
| --- | --- |
| Learn model and field syntax | `00-syntax/01-basic-model.rpl` |
| Generate validation | `02-validate/01-string-and-number-validation.rpl` |
| Build a database repository | `03-sql/03-sqlite-storage.rpl` then `projects/account-service` |
| Serialize cache values | `04-redis/01-session-cache.rpl` then `projects/session-cache` |
| Expose gRPC | `05-grpc/01-basic-service.rpl` then `projects/grpc-service` |
| Split one package across files | `06-multifile/main.rpl` |
| Build local process IPC | `09-transport/01-os-bin-service.rpl` then `projects/process-service` |
| Inspect most features together | `99-showcase/main.rpl` |

## Run An Example

From the repository root:

```bash
go -C src run ./cmd run ../examples/05-grpc/01-basic-service.rpl out /tmp/rpl-out
go -C src run ./cmd run ../examples/09-transport/01-os-bin-service.rpl out /tmp/rpl-transport
```

For package-based examples, run the main file in that folder:

```bash
go -C src run ./cmd run ../examples/06-multifile/main.rpl out /tmp/rpl-multifile
go -C src run ./cmd run ../examples/99-showcase/main.rpl out /tmp/rpl-showcase
```

## Run a Full Project

Build RPL once, generate one module, and run its tests:

```bash
make build-host
./build/$(go env GOOS)-$(go env GOARCH)/rpl \
  run examples/projects/account-service/src/main.rpl \
  out examples/projects/account-service/generated
go -C examples/projects/account-service test ./...
```

Run the clean-room lifecycle for all full projects:

```bash
make test-projects
```

The test copies each project to a temporary directory without `generated/`.
That distinction matters: a checked-in generated fixture can hide a broken
generator, while a clean-room project test proves schema, config, attrs, module
imports, generated code, and handwritten consumers still agree.

## Global Attr Setup for Examples

The projects work with bundled attrs from a normal RPL release. To develop one
shared custom attr for all example projects:

```bash
rpl config init --global
rpl attr init --global acme:example
rpl attr list
```

`RPL_CONFIG_HOME=/path/to/portable-rpl` isolates the global config for CI or a
throwaway experiment. A project-local attr with the same identifier has higher
priority than the global one.

## Notes

- The examples prefer explicit attrs like `@grpc()` and `@sql(index: true)` so
  the syntax is easier to copy.
- SQL examples use `@sql(primaryKey: true)` for stable updates and upserts; the
  generated package also exposes typed columns, `Where`/`And`, and `NewStore`.
- Some examples use standard library imports such as `time` or `net/http`.
- The `grpc` examples show both classic custom methods and the explicit
  instance-style mode enabled by `@grpc.Model()`.
- The `transport` examples use `@transport(os.bin)` to generate a local shell
  transport over stdin/stdout without HTTP.
- Do not edit generated `*.gen.go` or `.proto` files. Change the schema, rerun
  RPL, and keep handwritten policy inside `internal/` or `cmd/`.
- Every full project README explains the generated API, testing boundary,
  execution command, production considerations, and suggested exercises.
