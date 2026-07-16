# RPL

RPL is a declarative language for describing models and metadata and generating
production-ready Go code around them.

One schema can define domain models, validation rules, SQL and MongoDB storage,
Redis helpers, gRPC services, and local transports. Generator behavior is
extended through attrs discovered by the RPL runtime.

## Example

```rpl
package account

target(lang: golang)

attrs (
	"rpl:sql"
	"rpl:validate"
)

@sql(table: "users", db: "postgres")
model User {
	@sql(primaryKey: true)
	Id int64

	@validate(email)
	@sql(unique: true)
	Email string
}
```

Generate a Go package:

```bash
rpl run src/main.rpl out models
```

## Built-in attrs

- `rpl:std` — comments, groups, and ignore metadata;
- `rpl:validate` — validation and sensitive-field helpers;
- `rpl:sql` — PostgreSQL/SQLite DDL, typed columns, stores, and queries;
- `rpl:mongodb` — BSON adapters, indexes, and CRUD/query helpers;
- `rpl:redis` — Redis key and hash helpers;
- `rpl:grpc` — protobuf definitions, Go stubs, and conversion helpers;
- `rpl:transport` — local stdin/stdout service transports.

## CLI

```text
rpl init [dir]                    create a project
rpl run <file.rpl> [out <dir>]   generate code
rpl fmt <file.rpl>               format a schema
rpl auto set import <file.rpl>   add missing attrs and imports
rpl docs [file.rpl]              generate schema documentation
rpl attr list                    list discovered attrs
rpl attr info author:name        inspect an attr
rpl runtime                      start the JSON API runtime
```

## Development

Requirements: Go and Node.js (only for the VS Code extension).

```bash
make test
make build-host
make build-all
make plugin
```

The Go module lives in `src/`. Built binaries are written to `build/` and are
not committed.

## VS Code

The extension in `editors/vscode/rpl` provides diagnostics, formatting,
completion, hover help, CodeLens actions, an RPL workspace explorer, toolchain
status, and generation tasks.

```bash
make plugin
```

Install the resulting VSIX with `Extensions: Install from VSIX...`.

## Repository layout

```text
src/                  compiler, CLI, runtime, SDK, and built-in attrs
editors/vscode/rpl/   VS Code extension
examples/             schema examples
test/app/             generated-code integration fixture
```

Repository: <https://github.com/rp1s/rpl>

## License

Apache-2.0. See the repository `LICENSE` file.
