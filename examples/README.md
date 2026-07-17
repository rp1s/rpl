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
- `01-std` - comments, reusable projections, command DTOs, selective ignores
- `02-validate` - scalars, optional values, collections, URL/phone/email, hash policy
- `03-sql` - PostgreSQL/SQLite, composite keys, JSON fields, defaults, custom columns
- `04-redis` - simple/composite/fallback keys and scalar, optional, list, nested hash values
- `05-grpc` - CRUD, nested messages, inside fields, classic/model/ID-bound methods
- `06-multifile` - one package split into multiple `.rpl` files
- `07-imports` - importing another `.rpl` file
- `08-mongodb` - ObjectID, sparse/unique indexes, BSON names, search/sort, CRUD/watch helpers
- `09-transport` - six adapters, multi-adapter, subject binding, and method-only services
- `10-ffi` - stable C ABI, C/Rust servers, and Go/Python/C/Rust clients
- `11-wasm` - WIT contracts, Rust Component guests, and typed Wasmtime hosts
- `99-showcase` - large end-to-end examples
- `projects` - five complete applications that are regenerated and compiled in CI

## Choose the Right Starting Point

| Goal | Start here |
| --- | --- |
| Learn model and field syntax | `00-syntax/01-basic-model.rpl` |
| Generate validation | `02-validate/01-string-and-number-validation.rpl` |
| Validate lists and optional fields | `02-validate/03-collections-and-optional-values.rpl` |
| Build a database repository | `03-sql/03-sqlite-storage.rpl` then `projects/account-service` |
| Use a composite SQL key | `03-sql/04-composite-primary-key.rpl` |
| Serialize cache values | `04-redis/01-session-cache.rpl` then `projects/session-cache` |
| Build a composite Redis key | `04-redis/03-composite-cache-key.rpl` |
| Expose gRPC | `05-grpc/01-basic-service.rpl` then `projects/grpc-service` |
| Mix gRPC method subjects | `05-grpc/07-method-subject-overrides.rpl` |
| Generate a MongoDB repository | `08-mongodb/02-sparse-profile-store.rpl` |
| Split one package across files | `06-multifile/main.rpl` |
| Build local process IPC | `09-transport/01-os-bin-service.rpl` then `projects/process-service` |
| Expose only selected transport methods | `09-transport/09-method-only-adapters.rpl` |
| Build a native cross-language library | `10-ffi/01-rust-server-all-clients.rpl` |
| Build a sandboxed application plugin | `11-wasm/01-rust-component.rpl` |
| Inspect most features together | `99-showcase/main.rpl` |

## Run An Example

From the repository root:

```bash
go -C src run ./cmd run ../examples/05-grpc/01-basic-service.rpl out /tmp/rpl-out
go -C src run ./cmd run ../examples/09-transport/01-os-bin-service.rpl out /tmp/rpl-transport
go -C src run ./cmd run ../examples/10-ffi/01-rust-server-all-clients.rpl out /tmp/rpl-ffi
go -C src run ./cmd run ../examples/11-wasm/01-rust-component.rpl out /tmp/rpl-wasm
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

Generate every focused plugin schema and compile all full projects:

```bash
make test-examples
```

The test copies each project to a temporary directory without `generated/`.
That distinction matters: a checked-in generated fixture can hide a broken
generator, while a clean-room project test proves schema, config, attrs, module
imports, generated code, and handwritten consumers still agree.

`TestFocusedAttrExamplesGenerate` separately copies and generates every `.rpl`
file from the nine plugin cookbook folders. This keeps even the small examples
executable instead of treating them as unchecked documentation snippets.

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
- Every plugin folder has its own README with a feature matrix and notes about
  the generated API.
- SQL examples use `@sql(primaryKey: true)` for stable updates and upserts; the
  generated package also exposes typed columns, `Where`/`And`, and `NewStore`.
- Some examples use standard library imports such as `time` or `net/http`.
- The `grpc` examples show both classic custom methods and the explicit
  instance-style mode enabled by `@grpc.Model()`.
- The `transport` examples cover six adapters. Repeated model-level attrs
  generate several clients/servers around one service interface; method-level
  modes restrict individual operations to one adapter.
- The `ffi` examples generate a canonical header plus selected server/client
  languages. Rust, C, Python, and Go syntax are verified by plugin tests.
- The `wasm` examples use WIT and the Component Model rather than the native
  FFI ABI. Rust guest/host scaffolds are generated without exposing linear memory.
- Do not edit generated `*.gen.go` or `.proto` files. Change the schema, rerun
  RPL, and keep handwritten policy inside `internal/` or `cmd/`.
- Every full project README explains the generated API, testing boundary,
  execution command, production considerations, and suggested exercises.
