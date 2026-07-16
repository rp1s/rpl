# RPL

[![Release](https://img.shields.io/github/v/release/rp1s/rpl?display_name=tag&sort=semver)](https://github.com/rp1s/rpl/releases/latest)
[![License](https://img.shields.io/github/license/rp1s/rpl)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white)](src/go.mod)
[![VS Code](https://img.shields.io/badge/VS%20Code-extension-007ACC?logo=visualstudiocode&logoColor=white)](editors/vscode/rpl)

RPL is a declarative schema language, compiler, extensible code-generation
runtime, and developer toolchain. It turns compact `.rpl` files into typed Go
models and the infrastructure around them: validation, SQL, MongoDB, Redis,
protobuf/gRPC, local process transports, documentation, and editor tooling.

The project includes:

- the RPL language parser, semantic analyzer, formatter, and Go target;
- a CLI for projects, generation, formatting, imports, docs, and attrs;
- a newline-oriented JSON runtime used by editor integrations;
- a public Go SDK for writing generator attrs;
- seven bundled `rpl:*` attrs;
- a VS Code extension with diagnostics, completion, CodeLens, navigation,
  tasks, and toolchain management;
- examples, integration fixtures, and release builds for six OS/architecture
  combinations.

> Current release: **RPL 0.6.0**. The production target included in this
> repository is Go (`target(lang: golang)`).

## Table of contents

- [Why RPL](#why-rpl)
- [How it works](#how-it-works)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Language guide](#language-guide)
- [Built-in attrs](#built-in-attrs)
- [CLI reference](#cli-reference)
- [Project configuration](#project-configuration)
- [Runtime API and attr SDK](#runtime-api-and-attr-sdk)
- [VS Code extension](#vs-code-extension)
- [Examples](#examples)
- [Repository architecture](#repository-architecture)
- [Development and testing](#development-and-testing)
- [Release builds](#release-builds)
- [Troubleshooting](#troubleshooting)
- [License](#license)

## Why RPL

Application models are usually repeated across structs, validators, database
queries, protobuf messages, clients, servers, and documentation. Those copies
drift. RPL keeps the model and its intent in one schema while leaving generated
Go code explicit, typed, and inspectable.

RPL is designed around a few principles:

1. **The schema stays small.** Models, fields, methods, defaults, imports, and
   generator metadata use a compact language.
2. **Generators are explicit.** A schema lists the attrs it depends on and
   applies them where they matter.
3. **Attrs are independent processes.** Built-in and local generators use a
   versioned SDK protocol instead of being hard-coded into the parser.
4. **Generated code is ordinary Go.** The output can be read, tested, and used
   without a special runtime embedded in the application.
5. **Diagnostics come from the real compiler.** The CLI and VS Code extension
   share parser, semantic analysis, formatting, attr discovery, and type
   catalog behavior.
6. **Generation is deterministic.** RPL tracks generated files and removes
   stale sidecars when schema capabilities change.

## How it works

```text
                         ┌──────────────────────┐
 .rpl files ───────────► │ parser + semantic    │
 imports / package       │ analysis + formatter │
                         └──────────┬───────────┘
                                    │ normalized schema + context
                                    ▼
                         ┌──────────────────────┐
 .rpl/config.xml ──────► │ attr discovery and   │ ◄──── bundled / local attrs
                         │ capability planning   │       rpl:sql, rpl:grpc, ...
                         └──────────┬───────────┘
                                    │ generated file claims
                                    ▼
                         ┌──────────────────────┐
                         │ Go target + attr     │
                         │ generators           │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         models, validators, stores,
                         protobuf, clients, servers, docs
```

The core compiler owns parsing, cross-file resolution, model groups, target
types, diagnostics, and generated-file coordination. Attrs contribute analysis,
documentation, and generated files at model or file scope. Conflicting Go file
claims are detected before output is written.

## Installation

### Download a release

Open [GitHub Releases](https://github.com/rp1s/rpl/releases/latest) and choose
the archive for your platform:

| Platform | Asset |
| --- | --- |
| macOS Apple Silicon | `rpl-v0.6.0-darwin-arm64.tar.gz` |
| macOS Intel | `rpl-v0.6.0-darwin-amd64.tar.gz` |
| Linux x86-64 | `rpl-v0.6.0-linux-amd64.tar.gz` |
| Linux ARM64 | `rpl-v0.6.0-linux-arm64.tar.gz` |
| Windows x86-64 | `rpl-v0.6.0-windows-amd64.zip` |
| Windows ARM64 | `rpl-v0.6.0-windows-arm64.zip` |

Every CLI archive contains:

```text
rpl-v0.6.0-<os>-<arch>/
├── rpl or rpl.exe
├── .rpl/
│   └── attrs/
│       ├── rpl_std/
│       ├── rpl_validate/
│       ├── rpl_sql/
│       ├── rpl_mongodb/
│       ├── rpl_redis/
│       ├── rpl_grpc/
│       └── rpl_transport/
├── README.md
└── LICENSE
```

Keep the executable and its `.rpl/attrs` directory together. RPL discovers
these bundled attrs relative to the executable and also searches the active
project configuration.

Verify the download against `checksums.txt`, then run:

```bash
./rpl help
./rpl attr list
```

On Unix, place the extracted directory somewhere stable and add or symlink the
executable into a directory on `PATH`. On Windows, add the extracted directory
to `PATH` without separating `rpl.exe` from `.rpl`.

### Build from source

Requirements:

- Go 1.25 or newer;
- `make` for the repository shortcuts;
- Node.js only when building the VS Code extension;
- `protoc` and Go protobuf tools when working on protobuf generation internals.

```bash
git clone https://github.com/rp1s/rpl.git
cd rpl
make build-host
```

The host build is created under:

```text
build/<goos>-<goarch>/rpl
build/<goos>-<goarch>/.rpl/attrs/
```

To build and install into `~/.local/bin` together with bundled attrs:

```bash
make install
```

The installer adds `~/.local/bin` to the detected shell profile if necessary.

## Quick start

### 1. Create a project

```bash
rpl init hello-rpl
cd hello-rpl
```

`rpl init` creates missing files but keeps existing files unchanged:

```text
hello-rpl/
├── .rpl/
│   ├── config.xml
│   └── attrs/
├── src/
│   └── main.rpl
└── README.md
```

### 2. Define a schema

```rpl
package account

target(lang: golang)

import (
    "time"
)

attrs (
    "rpl:sql"
    "rpl:validate"
)

@sql(db: "postgres", table: "users")
model User {
    Id int64 @sql(column: "id", primaryKey: true)
    Email string @validate(email) @sql(unique: true, index: true)
    Name string = "anonymous" @validate(min: 2, max: 64)
    CreatedAt time.Time @sql(default: "now") @validate(past)
    UpdatedAt time.Time? @sql(default: "now", updatedAt: true)
}
```

### 3. Format and complete imports

```bash
rpl auto set import src/main.rpl
rpl fmt src/main.rpl
```

### 4. Generate code

```bash
rpl run src/main.rpl out models
```

The exact output depends on enabled attrs. For the schema above it includes the
base Go model, validation helpers, SQL DDL, scanners, typed columns, CRUD/query
helpers, and a `Store` abstraction.

### 5. Generate schema documentation

```bash
rpl docs src/main.rpl
```

## Language guide

### Package and target

`package` groups multiple `.rpl` files into one logical package. It is optional
for a single-file schema. `target` selects the output language:

```rpl
package billing

target(lang: golang)
```

When a package contains several files, the compiler resolves them together and
keeps package-level target ownership consistent.

### Imports

The same `import` block can reference Go packages and other RPL files:

```rpl
import (
    "time"
    http "net/http"
    "shared.rpl"
)
```

Go imports make external types such as `time.Time` and `http.Request`
available to the schema and generated package. RPL imports make models from
another schema file available for resolution.

### Attr declarations

Before using a generator namespace, declare its runtime identifier:

```rpl
attrs (
    "rpl:std"
    "rpl:validate"
    "rpl:sql"
)
```

The compiler discovers each manifest and executable, asks the attr for its
specification and capabilities, validates attr arguments, runs analysis, and
collects generated files.

### Models and fields

```rpl
model User {
    Id int64
    Name string
    Active bool = true
    Nickname string?
    Tags []string
}
```

- field names use exported Go-style identifiers;
- `?` marks an optional value;
- `[]T` represents a repeated value;
- `= expression` declares a default expression;
- model names can be used as field types;
- imported Go types can be referenced by package alias.

RPL understands common Go scalar types, `[]byte`, `time.Time`, model types, and
external imported types. The target type catalog is also exposed to editor
completion through the runtime API.

### Attr syntax

Attrs can be written inline:

```rpl
Email string @validate(email) @sql(unique: true)
```

or in a block when several annotations need to stay readable:

```rpl
Email string
{
    @comment("Public login address.")
    @validate(email: true, minLen: 6, maxLen: 160)
    @sql(unique: true, index: true)
}
```

Attrs can apply to models, fields, and methods. Their allowed arguments and
value types come from the runtime `AttrSpec`, so CLI and editor validation stay
aligned with the actual generator.

### Model methods

```rpl
model User {
    FirstName string
    LastName string
}

func User {
    func DisplayName return (string)
    func IsNamed return (bool)
}
```

Methods may also be declared inside a model and can accept parameters:

```rpl
model User {
    Name string

    func Greeting (prefix string) return (string)
}
```

### Field methods

```rpl
model User {
    Email string
}

func User.Email {
    func Domain return (string)
    func Normalized return (string)
}
```

Field-bound and model-bound methods can be consumed by transport attrs. gRPC
also supports inspecting exported methods of imported Go types for explicit
inside-field exposure.

### Groups and derived models

`@group` creates reusable projections such as request, public, or private
models:

```rpl
attrs (
    "rpl:std"
)

model User {
    Name string @group("req")
    Age int @group("req")
    Bio string @group("public")
}

model AuditEntry {
    Actor UserReq
    Snapshot UserPublic
}
```

### Ignore rules

Use a generator-specific flag or the standard `@ignore` attr:

```rpl
Secret string @ignore("grpc", "sql", "redis")
DebugState string @redis(ignore: true)
InternalFlags string @grpc(ignore: true)
```

### Multi-file packages

Models may be split across several `.rpl` files sharing the same package. Run
generation from the package entry file; the compiler finds related package
files, resolves cross-file types, and generates one coordinated output tree.

See [`examples/06-multifile`](examples/06-multifile) and
[`examples/07-imports`](examples/07-imports).

## Built-in attrs

| Runtime | Version | Main responsibility |
| --- | ---: | --- |
| `rpl:std` | 1.0.0 | comments, groups, and cross-generator ignore metadata |
| `rpl:validate` | 1.0.0 | field validation and sensitive/hash helpers |
| `rpl:sql` | 1.1.0 | PostgreSQL/SQLite DDL, scans, CRUD, typed queries, stores |
| `rpl:mongodb` | 1.0.0 | BSON adapters, indexes, search/sort metadata, CRUD helpers |
| `rpl:redis` | 1.0.0 | Redis keys, hash storage, uniqueness, defaults, TTL |
| `rpl:grpc` | 1.0.0 | `.proto`, protobuf Go, gRPC services, clients, conversions |
| `rpl:transport` | 1.0.0 | local stdin/stdout process client/server shells |

### `rpl:std`

Standard metadata understood across the toolchain:

- `@comment("text")` attaches documentation to a model or field;
- `@group("name")` includes a field in a derived projection;
- `@ignore("attr", ...)` excludes a field from selected generators.

### `rpl:validate`

Field validation arguments:

| Argument | Purpose |
| --- | --- |
| `min`, `max` | numeric or comparable range constraints |
| `minLen`, `maxLen` | string/collection length constraints |
| `email` | email format validation |
| `phone` | phone format validation |
| `url` | URL validation |
| `past` | require a timestamp in the past |
| `hash` | mark a sensitive value and generate hash-oriented helpers |

Example:

```rpl
model Signup {
    Name string @validate(min: 2, max: 32)
    Email string @validate(email)
    Password string @validate(minLen: 8, maxLen: 128, hash: "password")
}
```

### `rpl:sql`

Model arguments:

- `db`: `postgres` (default) or `sqlite`;
- `table`: explicit table name.

Field arguments:

- `column`, `primaryKey`, `unique`, `index`;
- `default`, `updatedAt`, `ignore`.

The generator validates and quotes identifiers, supports composite primary
keys, excludes keys from `UPDATE SET`, chooses a valid upsert conflict target,
and handles SQLite pagination. Generated APIs include DDL initialization,
scanners, CRUD/query functions, typed `Column` values, `Where`/`And`, an
`Executor` interface compatible with `*sql.DB` and `*sql.Tx`, and `NewStore`.

```go
store := usersql.NewStore(db)
if err := store.Init(ctx); err != nil {
    return err
}

user, err := store.Get(ctx,
    usersql.Where(usersql.ColumnEmail, email),
)
```

`Init` is a bootstrap helper using `CREATE TABLE/INDEX IF NOT EXISTS`; it is not
a destructive migration engine. More details are in
[`src/attrs/rpl:sql/README.md`](src/attrs/rpl:sql/README.md).

### `rpl:mongodb`

Model arguments: `db`, `collection`.

Field arguments: `name`, `index`, `unique`, `sparse`, `search`, `sort`,
`objectId`, `omitempty`, `default`, `updatedAt`, and `ignore`.

The output includes collection constants, index definitions, BSON conversion,
metadata, CRUD/query helpers, search filters, sort fields, ObjectID handling,
and update timestamp behavior.

### `rpl:redis`

Model arguments: `db`, `table`, `ttl`.

Field arguments: `unique`, `default`, `ignore`.

The generator produces model key conventions, hash serialization helpers,
unique-key metadata, defaults, and expiration-aware storage helpers.

### `rpl:grpc`

`@grpc()` generates protobuf definitions, protobuf Go code, typed gRPC
services, clients, servers, and conversion helpers. It supports:

- ordinary model services;
- custom model methods;
- model-bound calls with `@grpc.Model()` or `model: true`;
- identity fields with `@grpc.id()`;
- subject binding;
- ignored fields;
- explicit inside fields/methods through `@grpc.Inside()` or `mode: "inside"`;
- inspection of exported methods on imported Go types.

Generated sidecar files are tracked and removed if gRPC is later removed from
the schema.

### `rpl:transport`

`@transport(os.bin)` generates a local process transport over stdin/stdout,
without HTTP. The generated package includes a service interface, server loop,
client, request/response envelopes, model CRUD-style operations, and custom
method bindings. Identity and model-bound semantics use `@transport.id()` and
`@transport.Model()`.

## CLI reference

Run `rpl help` for the installed version.

### Project commands

```text
rpl init [dir]
rpl new [dir]
```

Creates a project scaffold. Existing files are preserved.

### Generation

```text
rpl run <file.rpl>
rpl run <file.rpl> out <directory>
rpl generate <file.rpl> out <directory>
rpl gen <file.rpl> out <directory>
rpl g <file.rpl> out <directory>
```

Without `out`, the generator uses its normal schema-relative output behavior.
With `out`, the destination is resolved explicitly.

### Formatting

```text
rpl fmt <file.rpl>
rpl format <file.rpl>
```

Formats the schema in place using the compiler formatter.

### Automatic imports and attrs

```text
rpl auto set import <file.rpl>
```

Analyzes used external Go types and attr namespaces and inserts missing
`import (...)` and `attrs (...)` declarations.

### Documentation

```text
rpl docs [file.rpl]
rpl doc [file.rpl]
```

Generates README documentation from a schema. If no path is supplied, RPL
looks for `src/main.rpl` and then `main.rpl`.

### Attr management

```text
rpl attr list
rpl attr info author:name
rpl attr init author:name
```

- `list` prints discovered manifests and versions;
- `info` prints one attr manifest;
- `init` creates a local SDK v2 attr scaffold with analysis, generation,
  documentation, manifest, and README files.

### Runtime

```text
rpl runtime
```

Starts the JSON API over stdin/stdout for editors and other tools.

## Project configuration

RPL searches upward from the active schema for `.rpl/config.xml`. If no file is
found, in-memory defaults are used.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<config>
  <runtimes>
    <directory>.rpl/attrs</directory>
  </runtimes>
  <localization>
    <language>en</language>
    <use_color>true</use_color>
  </localization>
  <author_data>
    <author_name>Example Team</author_name>
  </author_data>
</config>
```

| Setting | Meaning |
| --- | --- |
| `runtimes/directory` | project-local attr directory |
| `localization/language` | diagnostic language (`en` or `ru`) |
| `localization/use_color` | ANSI output for CLI diagnostics |
| `author_data/author_name` | author inserted into generated metadata where supported |

Attr discovery considers the configured project directory and bundled attrs
next to the RPL executable. Project-local attrs can override or extend the
installed toolchain.

## Runtime API and attr SDK

### JSON runtime actions

The stdin/stdout runtime exposes these actions:

| Action | Purpose |
| --- | --- |
| `run` | parse and compile code into an AST response |
| `check` | return compiler diagnostics without writing output |
| `format` | return formatted schema code |
| `auto.set.import` | return code with missing imports and attrs inserted |
| `lang` | set the active diagnostic language |
| `lang.current` | read the active diagnostic language |
| `attrs.get` | inspect one configured attr |
| `attrs.search` | list/search discovered attrs and their specs |
| `types.catalog` | return target types and structural snippets |

The VS Code extension keeps one runtime process alive and sends current
document text to these actions. Runtime failures are surfaced as diagnostics
instead of being hidden only in logs.

### Attr SDK

The SDK under [`src/pkg/sdk`](src/pkg/sdk) defines:

- attr manifests, capabilities, and versioned requests;
- schema/model/type snapshots;
- `AttrSpec` and typed attr arguments;
- analysis diagnostics and storage/file claims;
- model/file generation requests and responses;
- docs requests;
- Go file formatting and code-builder helpers;
- catalog, schema, declarations, runtime, and target convenience packages.

Start a local attr with:

```bash
rpl attr init acme:audit
```

The generated executable communicates with the compiler over stdin/stdout. An
attr advertises whether it can analyze/generate/docs at model or file scope,
and the compiler invokes only supported capabilities.

## VS Code extension

Download `rpl-vscode-0.6.0.vsix` from the release and run:

```text
Extensions: Install from VSIX...
```

The extension requires the RPL CLI in `PATH`, or an explicit binary selected
with `RPL: Select CLI Binary`.

### Editing features

- TextMate and semantic highlighting for `.rpl`;
- completion for keywords, models, target types, structures, attrs, and attr
  arguments discovered from the active runtime;
- hover documentation sourced from `AttrSpec`;
- live compiler diagnostics;
- quick fixes for missing attrs and Go imports;
- document formatting and optional format-on-save;
- automatic import/attr insertion on save;
- Outline and `Go to Symbol` for package, target, models, and fields;
- CodeLens actions above models: Generate, Check, Docs.

### Workspace UX

- an RPL Explorer view listing schemas, attrs, and toolchain state;
- editor-title Generate and Check actions;
- status bar and Language Status integration;
- a central RPL action menu;
- generated VS Code Tasks for Generate, Docs, and Format;
- output-channel logs and runtime restart controls;
- remembered generation output directory per schema.

### Extension settings

| Setting | Default | Purpose |
| --- | ---: | --- |
| `rpl.binaryPath` | `rpl` | CLI binary path |
| `rpl.enableDiagnostics` | `true` | live compiler diagnostics |
| `rpl.enableCompletions` | `true` | completion provider |
| `rpl.autoSetImportsOnSave` | `true` | update attrs/imports on save |
| `rpl.formatOnSave` | `true` | format through RPL runtime on save |
| `rpl.enableCodeLens` | `true` | model actions in the editor |
| `rpl.showStatusBar` | `true` | runtime and diagnostic state |
| `rpl.enableTaskProvider` | `true` | automatic workspace tasks |

Extension source and additional documentation live in
[`editors/vscode/rpl`](editors/vscode/rpl).

## Examples

[`examples`](examples) is a working cookbook:

| Directory | Topic |
| --- | --- |
| `00-syntax` | models, defaults, imports, model methods, field methods |
| `01-std` | comments, groups, derived models, ignore rules |
| `02-validate` | ranges, lengths, email, phone, URL, past time, hashes |
| `03-sql` | PostgreSQL, SQLite, keys, indexes, defaults, stores |
| `04-redis` | cache models, TTL, uniqueness, ignored fields |
| `05-grpc` | classic, model-bound, ID-bound, and inside methods |
| `06-multifile` | one package split across RPL files |
| `07-imports` | importing another RPL schema |
| `08-mongodb` | collections, BSON, indexes, search, sort, CRUD |
| `09-transport` | stdin/stdout process transport |
| `99-showcase` | end-to-end combinations of built-in attrs |

Run an example from the repository root:

```bash
go -C src run ./cmd run ../examples/03-sql/01-user-storage.rpl out /tmp/rpl-sql
go -C src run ./cmd run ../examples/05-grpc/01-basic-service.rpl out /tmp/rpl-grpc
go -C src run ./cmd run ../examples/99-showcase/main.rpl out /tmp/rpl-showcase
```

## Repository architecture

```text
.
├── Makefile
├── README.md
├── editors/
│   └── vscode/rpl/              VS Code extension
├── examples/                    executable schema cookbook
├── src/                         Go module
│   ├── attrs/                   bundled attr source modules
│   │   ├── rpl:std/
│   │   ├── rpl:validate/
│   │   ├── rpl:sql/
│   │   ├── rpl:mongodb/
│   │   ├── rpl:redis/
│   │   ├── rpl:grpc/
│   │   └── rpl:transport/
│   ├── cmd/                     CLI and fingerprint tools
│   ├── internal/
│   │   ├── cli/                 command implementations
│   │   ├── config/              XML config discovery and defaults
│   │   ├── formatter/           RPL source formatter
│   │   ├── generator/           parser, AST, analyzer, targets, runtime
│   │   ├── jsonapi/             editor/runtime request handlers
│   │   ├── plugins/             attr discovery, processes, scaffolding
│   │   └── service/             compiler/language/type services
│   └── pkg/
│       ├── error/               structured localized diagnostics
│       ├── fingerprint/         optional private-build device identity
│       └── sdk/                 public attr SDK
└── test/app/                    generated-code integration fixture
```

### Generation lifecycle

1. Locate project configuration relative to the requested schema.
2. Load the file and related package/import files.
3. Parse source into the AST and resolve types, groups, methods, and defaults.
4. Discover declared attrs and query their capabilities/specs.
5. Run compiler and attr analysis; merge structured diagnostics.
6. Plan base target and attr-generated files and reject conflicting claims.
7. Format generated Go source and write the output atomically.
8. Update `.rpl-generated.json` and remove obsolete generated sidecars.

## Development and testing

### Common commands

```bash
make help
make test
make test-attrs
make build-host
make build-all
make plugin
make clean
```

### Go tests

```bash
go -C src test ./...
go -C src vet ./...
```

`make test` runs the core package suite and then every built-in attr. Attr tests
are invoked from their directories because runtime directory names contain
`author:name` identifiers.

### Integration fixture

```bash
cd test/app
go test ./...
```

The fixture compiles generated models, validation, SQL, protobuf, gRPC client,
and gRPC server packages together.

### VS Code tests and package

```bash
cd editors/vscode/rpl
npm test
vsce package
```

### Building all release targets

```bash
make build-all
```

Default matrix:

- `darwin/arm64`, `darwin/amd64`;
- `linux/arm64`, `linux/amd64`;
- `windows/arm64`, `windows/amd64`.

Each target contains the CLI and compiled sidecar attrs. The build also creates
the standalone fingerprint inspection tool under `build/fingerprint/`.

## Release builds

Public builds are portable and do not require a device fingerprint. Private
device-locked builds remain available explicitly:

```bash
make build-host FINGERPRINT=<sha256-fingerprint>
```

Internally this injects:

```text
-ldflags "-X rpl/internal/version.Fingerprint=<value>"
```

When no value is supplied, RPL skips device inspection entirely. This is the
correct mode for public GitHub releases and cross-platform packages.

Release assets include SHA-256 checksums. Verify them before installing:

```bash
shasum -a 256 -c checksums.txt
```

## Troubleshooting

### `attr ... was not found`

Run:

```bash
rpl attr list
```

Confirm that `.rpl/attrs` is still beside the installed executable or that the
project's `.rpl/config.xml` points to a valid local attr directory.

### Runtime or VS Code diagnostics are unavailable

Check:

```bash
rpl help
rpl runtime
```

In VS Code, use `RPL: Show Toolchain Status`, inspect the RPL Output channel,
and set `rpl.binaryPath` if the CLI is not on `PATH`.

### An external Go type cannot be resolved

Declare its Go import in the schema or run:

```bash
rpl auto set import path/to/schema.rpl
```

For gRPC inside-method inspection, the imported Go package must be available to
the active Go toolchain/module cache.

### Generated protobuf files are missing

Confirm that the schema declares `rpl:grpc`, applies `@grpc()`, and that the
protobuf tools required by the development environment are installed. Inspect
the compiler diagnostic detail; subprocess output is preserved.

### Output contains stale files

Generate through RPL rather than copying individual generated files. RPL uses
`.rpl-generated.json` to track ownership and remove obsolete sidecars safely.

### ANSI output or diagnostic language is wrong

Update `.rpl/config.xml`:

```xml
<localization>
  <language>en</language>
  <use_color>false</use_color>
</localization>
```

## License

RPL is released under the [Apache License 2.0](LICENSE).

- Repository: <https://github.com/rp1s/rpl>
- Releases: <https://github.com/rp1s/rpl/releases>
- Issues: <https://github.com/rp1s/rpl/issues>
